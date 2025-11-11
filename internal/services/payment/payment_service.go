package payment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// paymentService implements the PaymentService port
type paymentService struct {
	db            *database.PostgreSQLAdapter
	serverPost    adapterports.ServerPostAdapter
	secretManager adapterports.SecretManagerAdapter
	logger        *zap.Logger
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	db *database.PostgreSQLAdapter,
	serverPost adapterports.ServerPostAdapter,
	secretManager adapterports.SecretManagerAdapter,
	logger *zap.Logger,
) ports.PaymentService {
	return &paymentService{
		db:            db,
		serverPost:    serverPost,
		secretManager: secretManager,
		logger:        logger,
	}
}

// Sale combines authorize and capture in one operation
func (s *paymentService) Sale(ctx context.Context, req *ports.SaleRequest) (*domain.Transaction, error) {
	s.logger.Info("Processing sale transaction",
		zap.String("agent_id", req.AgentID),
		zap.String("amount", req.Amount),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.GetTransactionByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			s.logger.Info("Idempotent request, returning existing transaction",
				zap.String("transaction_id", existing.ID),
			)
			return existing, nil
		}
	}

	// Get agent credentials using sqlc
	agent, err := s.db.Queries().GetAgentByAgentID(ctx, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// Check if agent is active (Valid must be true and Bool must be true)
	if !agent.IsActive.Valid || !agent.IsActive.Bool {
		return nil, domain.ErrAgentInactive
	}

	// Get MAC secret from secret manager (will be used for EPX request signing)
	_, err = s.secretManager.GetSecret(ctx, agent.MacSecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC secret: %w", err)
	}

	// Determine auth_guid (payment token)
	var authGUID string
	var paymentMethodUUID *uuid.UUID // Reuse parsed UUID
	if req.PaymentMethodID != nil {
		// Using saved payment method - parse UUID once
		pmID, err := uuid.Parse(*req.PaymentMethodID)
		if err != nil {
			return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
		}
		paymentMethodUUID = &pmID

		pm, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)
		if err != nil {
			return nil, fmt.Errorf("failed to get payment method: %w", err)
		}
		authGUID = pm.PaymentToken
	} else if req.PaymentToken != nil {
		// Using one-time token
		authGUID = *req.PaymentToken
	} else {
		return nil, fmt.Errorf("either payment_method_id or payment_token is required")
	}

	// Call EPX Server Post API for sale
	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         agent.CustNbr,
		MerchNbr:        agent.MerchNbr,
		DBAnbr:          agent.DbaNbr,
		TerminalNbr:     agent.TerminalNbr,
		TransactionType: adapterports.TransactionTypeSale,
		Amount:          req.Amount,
		PaymentType:     adapterports.PaymentMethodTypeCreditCard,
		AuthGUID:        authGUID,
		TranNbr:         uuid.New().String(),
		TranGroup:       uuid.New().String(),
		CustomerID:      stringOrEmpty(req.CustomerID),
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX transaction failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Save transaction to database using WithTx for transaction safety
	var transaction *domain.Transaction
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// Parse amount
		amount, err := decimal.NewFromString(req.Amount)
		if err != nil {
			return fmt.Errorf("invalid amount: %w", err)
		}

		// Marshal metadata
		metadataJSON, err := json.Marshal(req.Metadata)
		if err != nil {
			s.logger.Warn("Failed to marshal metadata", zap.Error(err))
			metadataJSON = []byte("{}")
		}

		// Determine status
		status := domain.TransactionStatusFailed
		if epxResp.IsApproved {
			status = domain.TransactionStatusCompleted
		}

		// Create transaction using sqlc-generated function
		params := sqlc.CreateTransactionParams{
			ID:                uuid.New(),
			GroupID:           uuid.MustParse(epxResp.TranGroup),
			AgentID:           req.AgentID,
			CustomerID:        toNullableText(req.CustomerID),
			Amount:            toNumeric(amount),
			Currency:          req.Currency,
			Status:            string(status),
			Type:              string(domain.TransactionTypeCharge),
			PaymentMethodType: string(domain.PaymentMethodTypeCreditCard),
			PaymentMethodID:   toNullableUUID(req.PaymentMethodID),
			AuthGuid:          toNullableText(&epxResp.AuthGUID),
			AuthResp:          toNullableText(&epxResp.AuthResp),
			AuthCode:          toNullableText(&epxResp.AuthCode),
			AuthRespText:      toNullableText(&epxResp.AuthRespText),
			AuthCardType:      toNullableText(&epxResp.AuthCardType),
			AuthAvs:           toNullableText(&epxResp.AuthAVS),
			AuthCvv2:          toNullableText(&epxResp.AuthCVV2),
			IdempotencyKey:    toNullableText(req.IdempotencyKey),
			Metadata:          metadataJSON,
		}

		dbTx, err := q.CreateTransaction(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		// Mark payment method as used if provided
		if paymentMethodUUID != nil {
			if err := q.MarkPaymentMethodUsed(ctx, *paymentMethodUUID); err != nil {
				s.logger.Warn("Failed to mark payment method as used", zap.Error(err))
			}
		}

		// Convert sqlc transaction to domain transaction
		transaction = sqlcToDomain(&dbTx)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Sale transaction completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("status", string(transaction.Status)),
		zap.Bool("approved", transaction.IsApproved()),
	)

	return transaction, nil
}

// Authorize holds funds on a payment method without capturing
func (s *paymentService) Authorize(ctx context.Context, req *ports.AuthorizeRequest) (*domain.Transaction, error) {
	s.logger.Info("Processing authorization",
		zap.String("agent_id", req.AgentID),
		zap.String("amount", req.Amount),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.GetTransactionByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			s.logger.Info("Idempotent request, returning existing transaction",
				zap.String("transaction_id", existing.ID),
			)
			return existing, nil
		}
	}

	// Get agent credentials
	agent, err := s.db.Queries().GetAgentByAgentID(ctx, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	if !agent.IsActive.Valid || !agent.IsActive.Bool {
		return nil, domain.ErrAgentInactive
	}

	// Get MAC secret
	_, err = s.secretManager.GetSecret(ctx, agent.MacSecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC secret: %w", err)
	}

	// Determine auth_guid (payment token)
	var authGUID string
	var paymentMethodUUID *uuid.UUID
	if req.PaymentMethodID != nil {
		pmID, err := uuid.Parse(*req.PaymentMethodID)
		if err != nil {
			return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
		}
		paymentMethodUUID = &pmID

		pm, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)
		if err != nil {
			return nil, fmt.Errorf("failed to get payment method: %w", err)
		}
		authGUID = pm.PaymentToken
	} else if req.PaymentToken != nil {
		authGUID = *req.PaymentToken
	} else {
		return nil, fmt.Errorf("either payment_method_id or payment_token is required")
	}

	// Call EPX Server Post API for authorization only
	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         agent.CustNbr,
		MerchNbr:        agent.MerchNbr,
		DBAnbr:          agent.DbaNbr,
		TerminalNbr:     agent.TerminalNbr,
		TransactionType: adapterports.TransactionTypeAuthOnly,
		Amount:          req.Amount,
		PaymentType:     adapterports.PaymentMethodTypeCreditCard,
		AuthGUID:        authGUID,
		TranNbr:         uuid.New().String(),
		TranGroup:       uuid.New().String(),
		CustomerID:      stringOrEmpty(req.CustomerID),
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX authorization failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Save transaction to database
	var transaction *domain.Transaction
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		amount, err := decimal.NewFromString(req.Amount)
		if err != nil {
			return fmt.Errorf("invalid amount: %w", err)
		}

		metadataJSON, err := json.Marshal(req.Metadata)
		if err != nil {
			s.logger.Warn("Failed to marshal metadata", zap.Error(err))
			metadataJSON = []byte("{}")
		}

		status := domain.TransactionStatusFailed
		if epxResp.IsApproved {
			status = domain.TransactionStatusCompleted
		}

		params := sqlc.CreateTransactionParams{
			ID:                uuid.New(),
			GroupID:           uuid.MustParse(epxResp.TranGroup),
			AgentID:           req.AgentID,
			CustomerID:        toNullableText(req.CustomerID),
			Amount:            toNumeric(amount),
			Currency:          "USD",
			Status:            string(status),
			Type:              string(domain.TransactionTypeAuth),
			PaymentMethodType: string(domain.PaymentMethodTypeCreditCard),
			PaymentMethodID:   toNullableUUID(req.PaymentMethodID),
			AuthGuid:          toNullableText(&epxResp.AuthGUID),
			AuthResp:          toNullableText(&epxResp.AuthResp),
			AuthCode:          toNullableText(&epxResp.AuthCode),
			AuthRespText:      toNullableText(&epxResp.AuthRespText),
			AuthCardType:      toNullableText(&epxResp.AuthCardType),
			AuthAvs:           toNullableText(&epxResp.AuthAVS),
			AuthCvv2:          toNullableText(&epxResp.AuthCVV2),
			IdempotencyKey:    toNullableText(req.IdempotencyKey),
			Metadata:          metadataJSON,
		}

		dbTx, err := q.CreateTransaction(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		if paymentMethodUUID != nil {
			if err := q.MarkPaymentMethodUsed(ctx, *paymentMethodUUID); err != nil {
				s.logger.Warn("Failed to mark payment method as used", zap.Error(err))
			}
		}

		transaction = sqlcToDomain(&dbTx)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Authorization completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("status", string(transaction.Status)),
		zap.Bool("approved", transaction.IsApproved()),
	)

	return transaction, nil
}

// Capture completes a previously authorized payment
func (s *paymentService) Capture(ctx context.Context, req *ports.CaptureRequest) (*domain.Transaction, error) {
	s.logger.Info("Processing capture",
		zap.String("transaction_id", req.TransactionID),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.GetTransactionByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			s.logger.Info("Idempotent request, returning existing transaction",
				zap.String("transaction_id", existing.ID),
			)
			return existing, nil
		}
	}

	// Get original authorization transaction
	originalTx, err := s.GetTransaction(ctx, req.TransactionID)
	if err != nil {
		return nil, err
	}

	if !originalTx.CanBeCaptured() {
		return nil, domain.ErrTransactionCannotBeCaptured
	}

	// Get agent credentials
	agent, err := s.db.Queries().GetAgentByAgentID(ctx, originalTx.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	if !agent.IsActive.Valid || !agent.IsActive.Bool {
		return nil, domain.ErrAgentInactive
	}

	// Get MAC secret
	_, err = s.secretManager.GetSecret(ctx, agent.MacSecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC secret: %w", err)
	}

	// Determine capture amount (partial or full)
	captureAmount := originalTx.Amount
	if req.Amount != nil {
		amt, err := decimal.NewFromString(*req.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount format: %w", err)
		}
		if amt.GreaterThan(originalTx.Amount) {
			return nil, fmt.Errorf("capture amount cannot exceed authorized amount")
		}
		captureAmount = amt
	}

	// Call EPX Server Post API for capture
	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         agent.CustNbr,
		MerchNbr:        agent.MerchNbr,
		DBAnbr:          agent.DbaNbr,
		TerminalNbr:     agent.TerminalNbr,
		TransactionType: adapterports.TransactionTypeCapture,
		Amount:          captureAmount.String(),
		PaymentType:     adapterports.PaymentMethodTypeCreditCard,
		AuthGUID:        *originalTx.AuthGUID, // Use original AUTH_GUID
		TranNbr:         uuid.New().String(),
		TranGroup:       originalTx.GroupID, // Same group as original
		CustomerID:      stringOrEmpty(originalTx.CustomerID),
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX capture failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Save transaction to database
	var transaction *domain.Transaction
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		status := domain.TransactionStatusFailed
		if epxResp.IsApproved {
			status = domain.TransactionStatusCompleted
		}

		params := sqlc.CreateTransactionParams{
			ID:                uuid.New(),
			GroupID:           uuid.MustParse(originalTx.GroupID),
			AgentID:           originalTx.AgentID,
			CustomerID:        toNullableText(originalTx.CustomerID),
			Amount:            toNumeric(captureAmount),
			Currency:          originalTx.Currency,
			Status:            string(status),
			Type:              string(domain.TransactionTypeCapture),
			PaymentMethodType: string(originalTx.PaymentMethodType),
			PaymentMethodID:   toNullableUUID(originalTx.PaymentMethodID),
			AuthGuid:          toNullableText(&epxResp.AuthGUID),
			AuthResp:          toNullableText(&epxResp.AuthResp),
			AuthCode:          toNullableText(&epxResp.AuthCode),
			AuthRespText:      toNullableText(&epxResp.AuthRespText),
			AuthCardType:      toNullableText(&epxResp.AuthCardType),
			AuthAvs:           toNullableText(&epxResp.AuthAVS),
			AuthCvv2:          toNullableText(&epxResp.AuthCVV2),
			IdempotencyKey:    toNullableText(req.IdempotencyKey),
			Metadata:          []byte(fmt.Sprintf(`{"original_transaction_id":"%s"}`, originalTx.ID)),
		}

		dbTx, err := q.CreateTransaction(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		transaction = sqlcToDomain(&dbTx)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Capture completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("original_transaction_id", originalTx.ID),
		zap.String("status", string(transaction.Status)),
	)

	return transaction, nil
}

// Void cancels an authorized or captured payment
func (s *paymentService) Void(ctx context.Context, req *ports.VoidRequest) (*domain.Transaction, error) {
	s.logger.Info("Processing void",
		zap.String("group_id", req.GroupID),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.GetTransactionByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			s.logger.Info("Idempotent request, returning existing transaction",
				zap.String("transaction_id", existing.ID),
			)
			return existing, nil
		}
	}

	// Get all transactions in the group using ListTransactions
	groupTxs, _, err := s.ListTransactions(ctx, &ports.ListTransactionsFilters{
		GroupID: &req.GroupID,
		Limit:   100, // Reasonable limit for a single transaction group
		Offset:  0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions by group: %w", err)
	}

	if len(groupTxs) == 0 {
		return nil, fmt.Errorf("no transactions found for group %s", req.GroupID)
	}

	// Find the original transaction (with auth_guid for EPX operation)
	originalTx, err := findOriginalTransaction(groupTxs)
	if err != nil {
		return nil, err
	}

	if !originalTx.CanBeVoided() {
		return nil, domain.ErrTransactionCannotBeVoided
	}

	// Get agent credentials
	agent, err := s.db.Queries().GetAgentByAgentID(ctx, originalTx.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	if !agent.IsActive.Valid || !agent.IsActive.Bool {
		return nil, domain.ErrAgentInactive
	}

	// Get MAC secret
	_, err = s.secretManager.GetSecret(ctx, agent.MacSecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC secret: %w", err)
	}

	// Call EPX Server Post API for void
	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         agent.CustNbr,
		MerchNbr:        agent.MerchNbr,
		DBAnbr:          agent.DbaNbr,
		TerminalNbr:     agent.TerminalNbr,
		TransactionType: adapterports.TransactionTypeVoid,
		Amount:          originalTx.Amount.String(),
		PaymentType:     adapterports.PaymentMethodType(originalTx.PaymentMethodType),
		AuthGUID:        *originalTx.AuthGUID, // Use original AUTH_GUID
		TranNbr:         uuid.New().String(),
		TranGroup:       originalTx.GroupID, // Same group as original
		CustomerID:      stringOrEmpty(originalTx.CustomerID),
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX void failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Save transaction to database
	var transaction *domain.Transaction
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		status := domain.TransactionStatusFailed
		if epxResp.IsApproved {
			status = domain.TransactionStatusCompleted
		}

		params := sqlc.CreateTransactionParams{
			ID:                uuid.New(),
			GroupID:           uuid.MustParse(originalTx.GroupID),
			AgentID:           originalTx.AgentID,
			CustomerID:        toNullableText(originalTx.CustomerID),
			Amount:            toNumeric(originalTx.Amount),
			Currency:          originalTx.Currency,
			Status:            string(status),
			Type:              string(domain.TransactionTypeCharge), // Void is still a charge type
			PaymentMethodType: string(originalTx.PaymentMethodType),
			PaymentMethodID:   toNullableUUID(originalTx.PaymentMethodID),
			AuthGuid:          toNullableText(&epxResp.AuthGUID),
			AuthResp:          toNullableText(&epxResp.AuthResp),
			AuthCode:          toNullableText(&epxResp.AuthCode),
			AuthRespText:      toNullableText(&epxResp.AuthRespText),
			AuthCardType:      toNullableText(&epxResp.AuthCardType),
			AuthAvs:           toNullableText(&epxResp.AuthAVS),
			AuthCvv2:          toNullableText(&epxResp.AuthCVV2),
			IdempotencyKey:    toNullableText(req.IdempotencyKey),
			Metadata:          []byte(fmt.Sprintf(`{"original_transaction_id":"%s"}`, originalTx.ID)),
		}

		dbTx, err := q.CreateTransaction(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		transaction = sqlcToDomain(&dbTx)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Void completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("original_transaction_id", originalTx.ID),
		zap.String("status", string(transaction.Status)),
	)

	return transaction, nil
}

// Refund returns funds to the customer
func (s *paymentService) Refund(ctx context.Context, req *ports.RefundRequest) (*domain.Transaction, error) {
	s.logger.Info("Processing refund",
		zap.String("group_id", req.GroupID),
		zap.String("reason", req.Reason),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.GetTransactionByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			s.logger.Info("Idempotent request, returning existing transaction",
				zap.String("transaction_id", existing.ID),
			)
			return existing, nil
		}
	}

	// Get all transactions in the group using ListTransactions
	groupTxs, _, err := s.ListTransactions(ctx, &ports.ListTransactionsFilters{
		GroupID: &req.GroupID,
		Limit:   100, // Reasonable limit for a single transaction group
		Offset:  0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions by group: %w", err)
	}

	if len(groupTxs) == 0 {
		return nil, fmt.Errorf("no transactions found for group %s", req.GroupID)
	}

	// Find the original transaction (with auth_guid for EPX operation)
	originalTx, err := findOriginalTransaction(groupTxs)
	if err != nil {
		return nil, err
	}

	if !originalTx.CanBeRefunded() {
		return nil, domain.ErrTransactionCannotBeRefunded
	}

	// Get agent credentials
	agent, err := s.db.Queries().GetAgentByAgentID(ctx, originalTx.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	if !agent.IsActive.Valid || !agent.IsActive.Bool {
		return nil, domain.ErrAgentInactive
	}

	// Get MAC secret
	_, err = s.secretManager.GetSecret(ctx, agent.MacSecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC secret: %w", err)
	}

	// Determine refund amount (partial or full)
	refundAmount := originalTx.Amount
	if req.Amount != nil {
		amt, err := decimal.NewFromString(*req.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount format: %w", err)
		}
		if amt.GreaterThan(originalTx.Amount) {
			return nil, fmt.Errorf("refund amount cannot exceed original transaction amount")
		}
		refundAmount = amt
	}

	// Call EPX Server Post API for refund
	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         agent.CustNbr,
		MerchNbr:        agent.MerchNbr,
		DBAnbr:          agent.DbaNbr,
		TerminalNbr:     agent.TerminalNbr,
		TransactionType: adapterports.TransactionTypeRefund,
		Amount:          refundAmount.String(),
		PaymentType:     adapterports.PaymentMethodType(originalTx.PaymentMethodType),
		AuthGUID:        *originalTx.AuthGUID, // Use original AUTH_GUID
		TranNbr:         uuid.New().String(),
		TranGroup:       originalTx.GroupID, // Same group as original
		CustomerID:      stringOrEmpty(originalTx.CustomerID),
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX refund failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Save transaction to database
	var transaction *domain.Transaction
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		status := domain.TransactionStatusFailed
		if epxResp.IsApproved {
			status = domain.TransactionStatusCompleted
		}

		metadata := map[string]interface{}{
			"original_transaction_id": originalTx.ID,
			"refund_reason":           req.Reason,
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			s.logger.Warn("Failed to marshal metadata", zap.Error(err))
			metadataJSON = []byte(fmt.Sprintf(`{"original_transaction_id":"%s"}`, originalTx.ID))
		}

		params := sqlc.CreateTransactionParams{
			ID:                uuid.New(),
			GroupID:           uuid.MustParse(originalTx.GroupID),
			AgentID:           originalTx.AgentID,
			CustomerID:        toNullableText(originalTx.CustomerID),
			Amount:            toNumeric(refundAmount),
			Currency:          originalTx.Currency,
			Status:            string(status),
			Type:              string(domain.TransactionTypeRefund),
			PaymentMethodType: string(originalTx.PaymentMethodType),
			PaymentMethodID:   toNullableUUID(originalTx.PaymentMethodID),
			AuthGuid:          toNullableText(&epxResp.AuthGUID),
			AuthResp:          toNullableText(&epxResp.AuthResp),
			AuthCode:          toNullableText(&epxResp.AuthCode),
			AuthRespText:      toNullableText(&epxResp.AuthRespText),
			AuthCardType:      toNullableText(&epxResp.AuthCardType),
			AuthAvs:           toNullableText(&epxResp.AuthAVS),
			AuthCvv2:          toNullableText(&epxResp.AuthCVV2),
			IdempotencyKey:    toNullableText(req.IdempotencyKey),
			Metadata:          metadataJSON,
		}

		dbTx, err := q.CreateTransaction(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create transaction: %w", err)
		}

		transaction = sqlcToDomain(&dbTx)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Refund completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("original_transaction_id", originalTx.ID),
		zap.String("amount", refundAmount.String()),
		zap.String("status", string(transaction.Status)),
	)

	return transaction, nil
}

// GetTransaction retrieves transaction details using sqlc
func (s *paymentService) GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error) {
	txID, err := uuid.Parse(transactionID)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction ID: %w", err)
	}

	dbTx, err := s.db.Queries().GetTransactionByID(ctx, txID)
	if err != nil {
		s.logger.Debug("Transaction not found",
			zap.String("transaction_id", transactionID),
			zap.Error(err),
		)
		return nil, domain.ErrTransactionNotFound
	}

	return sqlcToDomain(&dbTx), nil
}

// GetTransactionByIdempotencyKey retrieves a transaction by idempotency key using sqlc
func (s *paymentService) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	dbTx, err := s.db.Queries().GetTransactionByIdempotencyKey(ctx, pgtype.Text{String: key, Valid: true})
	if err != nil {
		s.logger.Debug("Transaction not found by idempotency key",
			zap.String("idempotency_key", key),
			zap.Error(err),
		)
		return nil, domain.ErrTransactionNotFound
	}

	return sqlcToDomain(&dbTx), nil
}

// ListTransactions lists transactions with filters using sqlc
func (s *paymentService) ListTransactions(ctx context.Context, filters *ports.ListTransactionsFilters) ([]*domain.Transaction, int, error) {
	// Set defaults
	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	params := sqlc.ListTransactionsParams{
		AgentID:         toNullableText(filters.AgentID),
		CustomerID:      toNullableText(filters.CustomerID),
		GroupID:         toNullableUUID(filters.GroupID),
		Status:          toNullableText(filters.Status),
		Type:            toNullableText(filters.Type),
		PaymentMethodID: toNullableUUID(filters.PaymentMethodID),
		LimitVal:        int32(limit),
		OffsetVal:       int32(offset),
	}

	dbTxs, err := s.db.Queries().ListTransactions(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list transactions: %w", err)
	}

	countParams := sqlc.CountTransactionsParams{
		AgentID:         toNullableText(filters.AgentID),
		CustomerID:      toNullableText(filters.CustomerID),
		GroupID:         toNullableUUID(filters.GroupID),
		Status:          toNullableText(filters.Status),
		Type:            toNullableText(filters.Type),
		PaymentMethodID: toNullableUUID(filters.PaymentMethodID),
	}

	count, err := s.db.Queries().CountTransactions(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	transactions := make([]*domain.Transaction, len(dbTxs))
	for i, dbTx := range dbTxs {
		transactions[i] = sqlcToDomain(&dbTx)
	}

	return transactions, int(count), nil
}

// GetTransactionsByGroup retrieves all transactions in a group using sqlc
func (s *paymentService) GetTransactionsByGroup(ctx context.Context, groupID string) ([]*domain.Transaction, error) {
	gID, err := uuid.Parse(groupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID: %w", err)
	}

	dbTxs, err := s.db.Queries().GetTransactionsByGroupID(ctx, gID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions by group: %w", err)
	}

	transactions := make([]*domain.Transaction, len(dbTxs))
	for i, dbTx := range dbTxs {
		transactions[i] = sqlcToDomain(&dbTx)
	}

	return transactions, nil
}

// findOriginalTransaction finds the original chargeable transaction in a group
// that has the auth_guid needed for EPX refund/void operations
func findOriginalTransaction(transactions []*domain.Transaction) (*domain.Transaction, error) {
	var originalTx *domain.Transaction

	for _, tx := range transactions {
		// Look for completed transactions with auth_guid that can be voided/refunded
		if tx.Status == domain.TransactionStatusCompleted &&
			tx.AuthGUID != nil &&
			(tx.Type == domain.TransactionTypeCharge ||
			 tx.Type == domain.TransactionTypeAuth ||
			 tx.Type == domain.TransactionTypeCapture) {
			// Prefer the earliest transaction
			if originalTx == nil || tx.CreatedAt.Before(originalTx.CreatedAt) {
				originalTx = tx
			}
		}
	}

	if originalTx == nil {
		return nil, fmt.Errorf("no original chargeable transaction found in group")
	}

	return originalTx, nil
}

// Helper functions to convert between sqlc and domain models

func sqlcToDomain(dbTx *sqlc.Transaction) *domain.Transaction {
	tx := &domain.Transaction{
		ID:                dbTx.ID.String(),
		GroupID:           dbTx.GroupID.String(),
		AgentID:           dbTx.AgentID,
		Amount:            decimal.NewFromBigInt(dbTx.Amount.Int, dbTx.Amount.Exp),
		Currency:          dbTx.Currency,
		Status:            domain.TransactionStatus(dbTx.Status),
		Type:              domain.TransactionType(dbTx.Type),
		PaymentMethodType: domain.PaymentMethodType(dbTx.PaymentMethodType),
		CreatedAt:         dbTx.CreatedAt,
		UpdatedAt:         dbTx.UpdatedAt,
	}

	if dbTx.CustomerID.Valid {
		customerID := dbTx.CustomerID.String
		tx.CustomerID = &customerID
	}

	if dbTx.PaymentMethodID.Valid {
		pmID := dbTx.PaymentMethodID.String()
		tx.PaymentMethodID = &pmID
	}

	if dbTx.AuthGuid.Valid {
		tx.AuthGUID = &dbTx.AuthGuid.String
	}
	if dbTx.AuthResp.Valid {
		tx.AuthResp = &dbTx.AuthResp.String
	}
	if dbTx.AuthCode.Valid {
		tx.AuthCode = &dbTx.AuthCode.String
	}
	if dbTx.AuthRespText.Valid {
		tx.AuthRespText = &dbTx.AuthRespText.String
	}
	if dbTx.AuthCardType.Valid {
		tx.AuthCardType = &dbTx.AuthCardType.String
	}
	if dbTx.AuthAvs.Valid {
		tx.AuthAVS = &dbTx.AuthAvs.String
	}
	if dbTx.AuthCvv2.Valid {
		tx.AuthCVV2 = &dbTx.AuthCvv2.String
	}
	if dbTx.IdempotencyKey.Valid {
		tx.IdempotencyKey = &dbTx.IdempotencyKey.String
	}

	if len(dbTx.Metadata) > 0 {
		if err := json.Unmarshal(dbTx.Metadata, &tx.Metadata); err != nil {
			// Log error but don't fail the entire operation
			// Metadata is supplementary information
			tx.Metadata = nil
		}
	}

	return tx
}

func toNullableText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func toNullableUUID(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{Valid: false}
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		// Invalid UUID format - return invalid pgtype.UUID
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func toNumeric(d decimal.Decimal) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   d.Coefficient(),
		Exp:   d.Exponent(),
		Valid: true,
	}
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
