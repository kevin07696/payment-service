package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/authorization"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/kevin07696/payment-service/internal/util"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// paymentService implements the PaymentService port
type paymentService struct {
	db               *database.PostgreSQLAdapter
	serverPost       adapterports.ServerPostAdapter
	secretManager    adapterports.SecretManagerAdapter
	merchantResolver *authorization.MerchantResolver
	logger           *zap.Logger
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	db *database.PostgreSQLAdapter,
	serverPost adapterports.ServerPostAdapter,
	secretManager adapterports.SecretManagerAdapter,
	merchantResolver *authorization.MerchantResolver,
	logger *zap.Logger,
) ports.PaymentService {
	return &paymentService{
		db:               db,
		serverPost:       serverPost,
		secretManager:    secretManager,
		merchantResolver: merchantResolver,
		logger:           logger,
	}
}

// resolveMerchantID resolves the merchant_id from auth context and request
func (s *paymentService) resolveMerchantID(ctx context.Context, requestedMerchantID string) (string, error) {
	// Get auth info from context
	authInfo := auth.GetAuthInfo(ctx)

	// If no auth (development/testing mode)
	if authInfo.Type == auth.AuthTypeNone {
		if requestedMerchantID == "" {
			return "", fmt.Errorf("merchant_id is required when auth is disabled")
		}
		return requestedMerchantID, nil
	}

	// If merchant ID is in context (API key auth or JWT with merchant_id)
	if authInfo.MerchantID != "" {
		// If a specific merchant was requested, verify it matches
		if requestedMerchantID != "" && requestedMerchantID != authInfo.MerchantID {
			return "", fmt.Errorf("merchant_id mismatch: requested %s but authenticated as %s",
				requestedMerchantID, authInfo.MerchantID)
		}
		return authInfo.MerchantID, nil
	}

	// For service auth, use the requested merchant ID if provided
	if authInfo.Type == auth.AuthTypeJWT && requestedMerchantID != "" {
		return requestedMerchantID, nil
	}

	return "", fmt.Errorf("unable to determine merchant_id")
}

// validateTransactionAccess validates that the auth context has access to a transaction
func (s *paymentService) validateTransactionAccess(ctx context.Context, tx *domain.Transaction, requiredScope string) error {
	// Get auth info from context
	authInfo := auth.GetAuthInfo(ctx)

	// If no auth (development/testing mode), allow access
	if authInfo.Type == auth.AuthTypeNone {
		s.logger.Debug("No authentication in context - allowing access (test mode or internal call)")
		return nil
	}

	// Check if the authenticated entity has access to this transaction's merchant
	if authInfo.MerchantID != "" {
		// For direct merchant authentication (API key)
		if authInfo.MerchantID != tx.MerchantID {
			return fmt.Errorf("no access to transaction for merchant %s", tx.MerchantID)
		}
	} else if authInfo.Type == auth.AuthTypeJWT && authInfo.ServiceID != "" {
		// For service authentication, we trust that the service verified merchant access
		// The auth interceptor already validated service-to-merchant access
		s.logger.Debug("Service authenticated access",
			zap.String("service_id", authInfo.ServiceID),
			zap.String("transaction_merchant", tx.MerchantID))
	} else {
		return fmt.Errorf("insufficient permissions to access transaction")
	}

	// Access granted
	return nil
}

// Sale combines authorize and capture in one operation
func (s *paymentService) Sale(ctx context.Context, req *ports.SaleRequest) (*domain.Transaction, error) {
	// Resolve merchant_id from token context
	resolvedMerchantID, err := s.resolveMerchantID(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Processing sale transaction",
		zap.String("merchant_id", resolvedMerchantID),
		zap.String("amount", req.Amount),
	)

	// Parse transaction ID from idempotency key (required)
	if req.IdempotencyKey == nil {
		return nil, fmt.Errorf("idempotency_key (transaction_id) is required")
	}

	txID, err := uuid.Parse(*req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("invalid idempotency_key format: %w", err)
	}

	// Get merchant credentials using sqlc
	merchantID, err := uuid.Parse(resolvedMerchantID)
	if err != nil {
		return nil, fmt.Errorf("invalid merchant_id format: %w", err)
	}

	merchant, err := s.db.Queries().GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merchant: %w", err)
	}

	// Check if merchant is active (Valid must be true and Bool must be true)
	if !merchant.IsActive {
		return nil, domain.ErrMerchantInactive
	}

	// Get MAC secret from secret manager (will be used for EPX request signing)
	_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
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

	// Generate deterministic numeric TRAN_NBR from transaction UUID (parsed from idempotency key above)
	// This ensures idempotency - same UUID always produces same TRAN_NBR
	epxTranNbr := util.UUIDToEPXTranNbr(txID)

	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         merchant.CustNbr,
		MerchNbr:        merchant.MerchNbr,
		DBAnbr:          merchant.DbaNbr,
		TerminalNbr:     merchant.TerminalNbr,
		TransactionType: adapterports.TransactionTypeSale,
		Amount:          req.Amount,
		PaymentType:     adapterports.PaymentMethodTypeCreditCard,
		AuthGUID:        authGUID,
		TranNbr:         epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
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

		// Merge request metadata with EPX response fields
		metadata := req.Metadata
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		// Add EPX display-only fields to metadata
		metadata["auth_resp_text"] = epxResp.AuthRespText
		metadata["auth_avs"] = epxResp.AuthAVS
		metadata["auth_cvv2"] = epxResp.AuthCVV2

		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			s.logger.Warn("Failed to marshal metadata", zap.Error(err))
			metadataJSON = []byte("{}")
		}

		// Create transaction using sqlc-generated function
		// Note: Status is auto-generated by database based on auth_resp
		// auth_guid (BRIC) is stored directly in the transaction
		// group_id auto-generates in DB (first transaction establishes the group)
		params := sqlc.CreateTransactionParams{
			ID:                txID,
			MerchantID:        merchantID,
			CustomerID:        toNullableText(req.CustomerID),
			Amount:            toNumeric(amount),
			Currency:          req.Currency,
			Type:              string(domain.TransactionTypeSale),
			PaymentMethodType: string(domain.PaymentMethodTypeCreditCard),
			PaymentMethodID:   toNullableUUID(req.PaymentMethodID),
			AuthGuid:          toNullableText(&epxResp.AuthGUID), // Store SALE's BRIC
			AuthResp:          epxResp.AuthResp,
			AuthCode:          toNullableText(&epxResp.AuthCode),
			AuthCardType:      toNullableText(&epxResp.AuthCardType),
			Metadata:          metadataJSON,
			GroupID:           nil, // DB auto-generates group_id for first transaction
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
	// Resolve merchant_id from token context
	resolvedMerchantID, err := s.resolveMerchantID(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Processing authorization",
		zap.String("merchant_id", resolvedMerchantID),
		zap.String("amount", req.Amount),
	)

	// Parse transaction ID from idempotency key (required)
	if req.IdempotencyKey == nil {
		return nil, fmt.Errorf("idempotency_key (transaction_id) is required")
	}

	txID, err := uuid.Parse(*req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("invalid idempotency_key format: %w", err)
	}

	// Check idempotency
	existing, err := s.GetTransactionByIdempotencyKey(ctx, *req.IdempotencyKey)
	if err == nil {
		s.logger.Info("Idempotent request, returning existing transaction",
			zap.String("transaction_id", existing.ID),
		)
		return existing, nil
	}

	// Get merchant credentials
	merchantID, err := uuid.Parse(resolvedMerchantID)
	if err != nil {
		return nil, fmt.Errorf("invalid merchant_id format: %w", err)
	}

	merchant, err := s.db.Queries().GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merchant: %w", err)
	}

	if !merchant.IsActive {
		return nil, domain.ErrMerchantInactive
	}

	// Get MAC secret
	_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
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
		CustNbr:         merchant.CustNbr,
		MerchNbr:        merchant.MerchNbr,
		DBAnbr:          merchant.DbaNbr,
		TerminalNbr:     merchant.TerminalNbr,
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

		// Merge request metadata with EPX response fields
		metadata := req.Metadata
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		// Add EPX display-only fields to metadata
		metadata["auth_resp_text"] = epxResp.AuthRespText
		metadata["auth_avs"] = epxResp.AuthAVS
		metadata["auth_cvv2"] = epxResp.AuthCVV2

		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			s.logger.Warn("Failed to marshal metadata", zap.Error(err))
			metadataJSON = []byte("{}")
		}

		// Note: Status is auto-generated by database based on auth_resp
		// auth_guid (BRIC) is stored directly in the transaction
		// group_id auto-generates in DB (first transaction establishes the group)
		params := sqlc.CreateTransactionParams{
			ID:                txID,
			MerchantID:        merchantID,
			CustomerID:        toNullableText(req.CustomerID),
			Amount:            toNumeric(amount),
			Currency:          "USD",
			Type:              string(domain.TransactionTypeAuth),
			PaymentMethodType: string(domain.PaymentMethodTypeCreditCard),
			PaymentMethodID:   toNullableUUID(req.PaymentMethodID),
			AuthGuid:          toNullableText(&epxResp.AuthGUID), // Store AUTH's BRIC
			AuthResp:          epxResp.AuthResp,
			AuthCode:          toNullableText(&epxResp.AuthCode),
			AuthCardType:      toNullableText(&epxResp.AuthCardType),
			Metadata:          metadataJSON,
			GroupID:           nil, // DB auto-generates group_id for first transaction
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
// Uses WAL-based state computation and row-level locking for consistency
func (s *paymentService) Capture(ctx context.Context, req *ports.CaptureRequest) (*domain.Transaction, error) {
	// Validate inputs first (fail-fast)
	if s.serverPost == nil {
		return nil, fmt.Errorf("server post adapter not initialized")
	}

	if req.TransactionID == "" {
		return nil, fmt.Errorf("transaction_id (original AUTH) is required")
	}

	originalTxID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction_id format: %w", err)
	}

	// Get original AUTH transaction first to validate access
	originalTx, err := s.db.Queries().GetTransactionByID(ctx, originalTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get original transaction: %w", err)
	}

	// Validate transaction access with required scope
	domainTx := sqlcToDomain(&originalTx)
	if err := s.validateTransactionAccess(ctx, domainTx, domain.ScopePaymentsCreate); err != nil {
		return nil, err
	}

	// Validate amount if provided
	var captureAmount decimal.Decimal
	if req.Amount != nil {
		captureAmount, err = decimal.NewFromString(*req.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount format: %w", err)
		}
		if captureAmount.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("amount must be greater than zero")
		}
	}

	// Generate or parse CAPTURE transaction ID for idempotency
	var txID uuid.UUID
	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		txID, err = uuid.Parse(*req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("invalid idempotency_key format: %w", err)
		}
	} else {
		txID = uuid.New()
	}

	// Check idempotency - CAPTURE transaction already exists?
	var existingTx *sqlc.Transaction
	existingTxDB, existErr := s.db.Queries().GetTransactionByID(ctx, txID)
	if existErr == nil {
		existingTx = &existingTxDB
		// Transaction exists - check if it's complete (has auth_resp)
		if existingTx.AuthResp != "" {
			// Transaction is complete - return existing (idempotent)
			s.logger.Info("CAPTURE transaction already complete (idempotency)",
				zap.String("transaction_id", txID.String()),
				zap.String("status", existingTx.Status.String),
			)
			return sqlcToDomain(existingTx), nil
		}
		// Transaction exists but auth_resp is empty - it's still pending
		s.logger.Warn("CAPTURE transaction is pending - possible retry",
			zap.String("transaction_id", txID.String()),
		)
		// Continue to process (will update the pending transaction)
	}

	groupID := originalTx.GroupID

	s.logger.Info("Processing capture",
		zap.String("capture_transaction_id", txID.String()),
		zap.String("original_transaction_id", req.TransactionID),
		zap.String("group_id", groupID.String()),
		zap.String("amount", stringOrEmpty(req.Amount)),
	)

	var transaction *domain.Transaction
	var groupTxs []sqlc.Transaction

	// Use database transaction for consistency
	// Note: We rely on idempotency (transaction.id as PK) rather than row-level locks on group_id
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// Get all transactions in chronological order
		groupTxs, err := q.GetTransactionsByGroupID(ctx, groupID)
		if err != nil {
			return fmt.Errorf("failed to get group transactions: %w", err)
		}

		if len(groupTxs) == 0 {
			return fmt.Errorf("no transactions found for group %s", groupID.String())
		}

		// Convert to domain transactions
		domainTxs := make([]*domain.Transaction, len(groupTxs))
		for i, tx := range groupTxs {
			domainTxs[i] = sqlcToDomain(&tx)
		}

		// Compute current state using WAL
		state := ComputeGroupState(domainTxs)

		// Validate capture is allowed
		captureAmount := state.ActiveAuthAmount
		if req.Amount != nil {
			captureAmount, err = decimal.NewFromString(*req.Amount)
			if err != nil {
				return fmt.Errorf("invalid amount format: %w", err)
			}
		}

		canCapture, reason := state.CanCapture(captureAmount)
		if !canCapture {
			s.logger.Warn("Capture validation failed",
				zap.String("capture_transaction_id", txID.String()),
				zap.String("reason", reason),
			)
			return domain.ErrTransactionCannotBeCaptured
		}

		s.logger.Info("Capture validation passed",
			zap.String("auth_bric", state.ActiveAuthBRIC),
			zap.String("capture_amount", captureAmount.String()),
			zap.String("remaining", state.ActiveAuthAmount.Sub(state.CapturedAmount).String()),
		)

		// Get merchant from first transaction
		merchantID := uuid.MustParse(domainTxs[0].MerchantID)
		merchant, err := q.GetMerchantByID(ctx, merchantID)
		if err != nil {
			return fmt.Errorf("failed to get merchant: %w", err)
		}

		if !merchant.IsActive {
			return domain.ErrMerchantInactive
		}

		// Get MAC secret
		_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
		if err != nil {
			return fmt.Errorf("failed to get MAC secret: %w", err)
		}

		return nil // Continue outside transaction for EPX call
	})

	if err != nil {
		return nil, err
	}

	// Re-fetch state outside transaction for EPX call
	groupTxs, err = s.db.Queries().GetTransactionsByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group transactions: %w", err)
	}

	domainTxs := make([]*domain.Transaction, len(groupTxs))
	for i, tx := range groupTxs {
		domainTxs[i] = sqlcToDomain(&tx)
	}
	state := ComputeGroupState(domainTxs)

	merchantID := uuid.MustParse(domainTxs[0].MerchantID)
	merchant, err := s.db.Queries().GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merchant: %w", err)
	}

	// Determine capture amount (use validated amount from earlier if provided)
	finalCaptureAmount := state.ActiveAuthAmount
	if req.Amount != nil {
		finalCaptureAmount = captureAmount // Use pre-validated amount
	}

	// Get BRIC for CAPTURE operation (uses AUTH's BRIC)
	authBRIC := state.GetBRICForOperation(domain.TransactionTypeCapture)

	// Create pending transaction BEFORE calling EPX
	// Only create if transaction doesn't exist yet
	if existingTx == nil {
		captureMetadata := map[string]interface{}{}
		_, _, err := s.CreatePendingTransaction(ctx, CreatePendingTransactionParams{
			ID:                txID,
			GroupID:           &groupID,
			MerchantID:        merchantID,
			CustomerID:        domainTxs[0].CustomerID,
			Amount:            finalCaptureAmount,
			Currency:          domainTxs[0].Currency,
			Type:              domain.TransactionTypeCapture,
			PaymentMethodType: domain.PaymentMethodType(domainTxs[0].PaymentMethodType),
			PaymentMethodID:   stringToUUIDPtr(domainTxs[0].PaymentMethodID),
			Metadata:          captureMetadata,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create pending transaction: %w", err)
		}

		s.logger.Info("Created pending CAPTURE transaction",
			zap.String("transaction_id", txID.String()),
		)
	}

	// Call EPX Server Post API for capture
	s.logger.Info("Calling EPX for capture",
		zap.String("auth_bric", authBRIC),
		zap.String("amount", finalCaptureAmount.String()),
	)

	// Generate deterministic numeric TRAN_NBR from transaction UUID
	// This ensures idempotency - same UUID always produces same TRAN_NBR
	epxTranNbr := util.UUIDToEPXTranNbr(txID)

	epxReq := &adapterports.ServerPostRequest{
		CustNbr:          merchant.CustNbr,
		MerchNbr:         merchant.MerchNbr,
		DBAnbr:           merchant.DbaNbr,
		TerminalNbr:      merchant.TerminalNbr,
		TransactionType:  adapterports.TransactionTypeCapture,
		Amount:           finalCaptureAmount.StringFixed(2),
		PaymentType:      adapterports.PaymentMethodTypeCreditCard,
		OriginalAuthGUID: authBRIC,   // Reference to AUTH transaction
		TranNbr:          epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:        "",         // No BATCH_ID for capture
		CustomerID:       stringOrEmpty(domainTxs[0].CustomerID),
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX capture failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Update pending transaction with EPX response
	metadata := map[string]interface{}{
		"auth_resp_text": epxResp.AuthRespText,
		"auth_avs":       epxResp.AuthAVS,
		"auth_cvv2":      epxResp.AuthCVV2,
	}
	err = s.UpdateTransactionWithEPXResponse(
		ctx,
		epxTranNbr,
		domainTxs[0].CustomerID,
		&epxResp.AuthGUID,
		&epxResp.AuthResp,
		&epxResp.AuthCode,
		&epxResp.AuthCardType,
		metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update transaction with EPX response: %w", err)
	}

	// Fetch the updated transaction
	updatedTx, err := s.db.Queries().GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated transaction: %w", err)
	}
	transaction = sqlcToDomain(&updatedTx)

	s.logger.Info("Capture completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("group_id", groupID.String()),
		zap.String("status", string(transaction.Status)),
	)

	return transaction, nil
}

// Void cancels an authorized or captured payment
// Uses WAL-based state computation and row-level locking for consistency
func (s *paymentService) Void(ctx context.Context, req *ports.VoidRequest) (*domain.Transaction, error) {
	// Validate inputs first (fail-fast)
	if s.serverPost == nil {
		return nil, fmt.Errorf("server post adapter not initialized")
	}

	if req.GroupID == "" {
		return nil, fmt.Errorf("group_id is required")
	}

	groupID, err := uuid.Parse(req.GroupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group_id format: %w", err)
	}

	// Get transactions in group to validate access
	groupTxs, err := s.db.Queries().GetTransactionsByGroupID(ctx, groupID)
	if err != nil || len(groupTxs) == 0 {
		return nil, fmt.Errorf("failed to get group transactions: %w", err)
	}

	// Validate access using the first transaction in the group
	firstTx := sqlcToDomain(&groupTxs[0])
	if err := s.validateTransactionAccess(ctx, firstTx, domain.ScopePaymentsVoid); err != nil {
		return nil, err
	}

	// Generate or parse transaction ID for idempotency
	var txID uuid.UUID
	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		txID, err = uuid.Parse(*req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("invalid idempotency_key format: %w", err)
		}
	} else {
		txID = uuid.New()
	}

	// Check idempotency - VOID transaction already exists?
	var existingTx *sqlc.Transaction
	existingTxDB, existErr := s.db.Queries().GetTransactionByID(ctx, txID)
	if existErr == nil {
		existingTx = &existingTxDB
		// Transaction exists - check if it's complete (has auth_resp)
		if existingTx.AuthResp != "" {
			// Transaction is complete - return existing (idempotent)
			s.logger.Info("VOID transaction already complete (idempotency)",
				zap.String("transaction_id", txID.String()),
				zap.String("status", existingTx.Status.String),
			)
			return sqlcToDomain(existingTx), nil
		}
		// Transaction exists but auth_resp is empty - it's still pending
		s.logger.Warn("VOID transaction is pending - possible retry",
			zap.String("transaction_id", txID.String()),
		)
		// Continue to process (will update the pending transaction)
	}

	s.logger.Info("Processing void",
		zap.String("void_transaction_id", txID.String()),
		zap.String("group_id", req.GroupID),
	)

	var transaction *domain.Transaction
	var voidAmount decimal.Decimal
	var originalTxType domain.TransactionType

	// Use database transaction for consistency
	// Note: We rely on idempotency (transaction.id as PK) rather than row-level locks on group_id
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// Get all transactions in chronological order (already fetched above for auth check)
		groupTxs, err := q.GetTransactionsByGroupID(ctx, groupID)
		if err != nil {
			return fmt.Errorf("failed to get group transactions: %w", err)
		}

		if len(groupTxs) == 0 {
			return fmt.Errorf("no transactions found for group %s", req.GroupID)
		}

		// Convert to domain transactions
		domainTxs := make([]*domain.Transaction, len(groupTxs))
		for i, tx := range groupTxs {
			domainTxs[i] = sqlcToDomain(&tx)
		}

		// Compute current state using WAL
		state := ComputeGroupState(domainTxs)

		// Validate void is allowed
		canVoid, reason := state.CanVoid()
		if !canVoid {
			return fmt.Errorf("void not allowed: %s", reason)
		}

		// Get the active AUTH transaction
		if state.ActiveAuthID == nil {
			return fmt.Errorf("no active authorization to void")
		}

		// Find original AUTH transaction for amount
		var originalAuth *domain.Transaction
		for _, tx := range domainTxs {
			if tx.ID == *state.ActiveAuthID {
				originalAuth = tx
				break
			}
		}
		if originalAuth == nil {
			return fmt.Errorf("active authorization transaction not found")
		}

		voidAmount = originalAuth.Amount
		originalTxType = originalAuth.Type

		s.logger.Info("Void validation passed",
			zap.String("auth_bric", state.ActiveAuthBRIC),
			zap.String("void_amount", voidAmount.String()),
		)

		// Get merchant from first transaction
		merchantID := uuid.MustParse(domainTxs[0].MerchantID)
		merchant, err := q.GetMerchantByID(ctx, merchantID)
		if err != nil {
			return fmt.Errorf("failed to get merchant: %w", err)
		}

		if !merchant.IsActive {
			return domain.ErrMerchantInactive
		}

		// Get MAC secret
		_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
		if err != nil {
			return fmt.Errorf("failed to get MAC secret: %w", err)
		}

		return nil // Continue outside transaction for EPX call
	})

	if err != nil {
		return nil, err
	}

	// Re-fetch state outside transaction for EPX call
	groupTxs, err = s.db.Queries().GetTransactionsByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group transactions: %w", err)
	}

	domainTxs := make([]*domain.Transaction, len(groupTxs))
	for i, tx := range groupTxs {
		domainTxs[i] = sqlcToDomain(&tx)
	}
	state := ComputeGroupState(domainTxs)

	merchantID := uuid.MustParse(domainTxs[0].MerchantID)
	merchant, err := s.db.Queries().GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merchant: %w", err)
	}

	// Get BRIC for VOID operation (uses AUTH's BRIC)
	authBRIC := state.GetBRICForOperation(domain.TransactionTypeVoid)

	// Create pending transaction BEFORE calling EPX
	// Only create if transaction doesn't exist yet
	if existingTx == nil {
		voidMetadata := map[string]interface{}{
			"original_transaction_type": string(originalTxType),
		}
		_, _, err := s.CreatePendingTransaction(ctx, CreatePendingTransactionParams{
			ID:                txID,
			GroupID:           &groupID,
			MerchantID:        merchantID,
			CustomerID:        domainTxs[0].CustomerID,
			Amount:            voidAmount,
			Currency:          domainTxs[0].Currency,
			Type:              domain.TransactionTypeVoid,
			PaymentMethodType: domain.PaymentMethodType(domainTxs[0].PaymentMethodType),
			PaymentMethodID:   stringToUUIDPtr(domainTxs[0].PaymentMethodID),
			Metadata:          voidMetadata,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create pending transaction: %w", err)
		}

		s.logger.Info("Created pending VOID transaction",
			zap.String("transaction_id", txID.String()),
		)
	}

	// Call EPX Server Post API for void
	s.logger.Info("Calling EPX for void",
		zap.String("auth_bric", authBRIC),
		zap.String("amount", voidAmount.String()),
	)

	// Generate deterministic numeric TRAN_NBR from transaction UUID
	// This ensures idempotency - same UUID always produces same TRAN_NBR
	epxTranNbr := util.UUIDToEPXTranNbr(txID)

	epxReq := &adapterports.ServerPostRequest{
		CustNbr:          merchant.CustNbr,
		MerchNbr:         merchant.MerchNbr,
		DBAnbr:           merchant.DbaNbr,
		TerminalNbr:      merchant.TerminalNbr,
		TransactionType:  adapterports.TransactionTypeVoid,
		Amount:           voidAmount.StringFixed(2),
		PaymentType:      adapterports.PaymentMethodType(domainTxs[0].PaymentMethodType),
		OriginalAuthGUID: authBRIC,   // Reference to AUTH transaction
		TranNbr:          epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:        "",
		CustomerID:       stringOrEmpty(domainTxs[0].CustomerID),
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX void failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Update pending transaction with EPX response
	metadata := map[string]interface{}{
		"original_transaction_type": string(originalTxType),
		"auth_resp_text":            epxResp.AuthRespText,
		"auth_avs":                  epxResp.AuthAVS,
		"auth_cvv2":                 epxResp.AuthCVV2,
	}
	err = s.UpdateTransactionWithEPXResponse(
		ctx,
		epxTranNbr,
		domainTxs[0].CustomerID,
		&epxResp.AuthGUID,
		&epxResp.AuthResp,
		&epxResp.AuthCode,
		&epxResp.AuthCardType,
		metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update transaction with EPX response: %w", err)
	}

	// Fetch the updated transaction
	updatedTx, err := s.db.Queries().GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated transaction: %w", err)
	}
	transaction = sqlcToDomain(&updatedTx)

	s.logger.Info("Void completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("group_id", groupID.String()),
		zap.String("status", string(transaction.Status)),
	)

	return transaction, nil
}

// Refund returns funds to the customer
// Uses WAL-based state computation and row-level locking for consistency
func (s *paymentService) Refund(ctx context.Context, req *ports.RefundRequest) (*domain.Transaction, error) {
	// Validate inputs first (fail-fast)
	if s.serverPost == nil {
		return nil, fmt.Errorf("server post adapter not initialized")
	}

	if req.GroupID == "" {
		return nil, fmt.Errorf("group_id is required")
	}

	groupID, err := uuid.Parse(req.GroupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group_id format: %w", err)
	}

	// Get transactions in group to validate access
	groupTxs, err := s.db.Queries().GetTransactionsByGroupID(ctx, groupID)
	if err != nil || len(groupTxs) == 0 {
		return nil, fmt.Errorf("failed to get group transactions: %w", err)
	}

	// Validate access using the first transaction in the group
	firstTx := sqlcToDomain(&groupTxs[0])
	if err := s.validateTransactionAccess(ctx, firstTx, domain.ScopePaymentsRefund); err != nil {
		return nil, err
	}

	// Validate amount if provided
	var refundAmount decimal.Decimal
	if req.Amount != nil {
		refundAmount, err = decimal.NewFromString(*req.Amount)
		if err != nil {
			return nil, fmt.Errorf("invalid amount format: %w", err)
		}
		if refundAmount.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("amount must be greater than zero")
		}
	}

	// Generate or parse transaction ID for idempotency
	var txID uuid.UUID
	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		txID, err = uuid.Parse(*req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("invalid idempotency_key format: %w", err)
		}
	} else {
		txID = uuid.New()
	}

	// Check idempotency - REFUND transaction already exists?
	var existingTx *sqlc.Transaction
	existingTxDB, existErr := s.db.Queries().GetTransactionByID(ctx, txID)
	if existErr == nil {
		existingTx = &existingTxDB
		// Transaction exists - check if it's complete (has auth_resp)
		if existingTx.AuthResp != "" {
			// Transaction is complete - return existing (idempotent)
			s.logger.Info("REFUND transaction already complete (idempotency)",
				zap.String("transaction_id", txID.String()),
				zap.String("status", existingTx.Status.String),
			)
			return sqlcToDomain(existingTx), nil
		}
		// Transaction exists but auth_resp is empty - it's still pending
		// This means another request is processing it, or it failed mid-way
		s.logger.Warn("REFUND transaction is pending - possible retry",
			zap.String("transaction_id", txID.String()),
		)
		// Continue to process (will update the pending transaction)
	}

	s.logger.Info("Processing refund",
		zap.String("refund_transaction_id", txID.String()),
		zap.String("group_id", req.GroupID),
		zap.String("reason", req.Reason),
	)

	var transaction *domain.Transaction
	var finalRefundAmount decimal.Decimal

	// Use database transaction for consistency
	// Note: We rely on idempotency (transaction.id as PK) rather than row-level locks on group_id
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// Get all transactions in chronological order
		groupTxs, err := q.GetTransactionsByGroupID(ctx, groupID)
		if err != nil {
			return fmt.Errorf("failed to get group transactions: %w", err)
		}

		if len(groupTxs) == 0 {
			return fmt.Errorf("no transactions found for group %s", req.GroupID)
		}

		// Convert to domain transactions
		domainTxs := make([]*domain.Transaction, len(groupTxs))
		for i, tx := range groupTxs {
			domainTxs[i] = sqlcToDomain(&tx)
		}

		// Compute current state using WAL
		state := ComputeGroupState(domainTxs)

		// Determine refund amount (use full captured amount if not specified)
		finalRefundAmount = state.CapturedAmount
		if req.Amount != nil {
			finalRefundAmount = refundAmount // Use pre-validated amount
		}

		// Validate refund is allowed
		canRefund, reason := state.CanRefund(finalRefundAmount)
		if !canRefund {
			s.logger.Warn("Refund validation failed",
				zap.String("group_id", req.GroupID),
				zap.String("reason", reason),
			)
			return domain.ErrTransactionCannotBeRefunded
		}

		s.logger.Info("Refund validation passed",
			zap.String("captured_amount", state.CapturedAmount.String()),
			zap.String("refunded_amount", state.RefundedAmount.String()),
			zap.String("refund_amount", finalRefundAmount.String()),
		)

		// Get merchant from first transaction
		merchantID := uuid.MustParse(domainTxs[0].MerchantID)
		merchant, err := q.GetMerchantByID(ctx, merchantID)
		if err != nil {
			return fmt.Errorf("failed to get merchant: %w", err)
		}

		if !merchant.IsActive {
			return domain.ErrMerchantInactive
		}

		// Get MAC secret
		_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
		if err != nil {
			return fmt.Errorf("failed to get MAC secret: %w", err)
		}

		return nil // Continue outside transaction for EPX call
	})

	if err != nil {
		return nil, err
	}

	// Re-fetch state outside transaction for EPX call
	groupTxs, err = s.db.Queries().GetTransactionsByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group transactions: %w", err)
	}

	domainTxs := make([]*domain.Transaction, len(groupTxs))
	for i, tx := range groupTxs {
		domainTxs[i] = sqlcToDomain(&tx)
	}
	state := ComputeGroupState(domainTxs)

	merchantID := uuid.MustParse(domainTxs[0].MerchantID)
	merchant, err := s.db.Queries().GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merchant: %w", err)
	}

	// Get BRIC for REFUND operation (uses CAPTURE's BRIC if available, otherwise AUTH's BRIC)
	authBRIC := state.GetBRICForOperation(domain.TransactionTypeRefund)

	// Create pending transaction BEFORE calling EPX
	// This ensures idempotency - if this call is retried, the transaction already exists
	// Only create if transaction doesn't exist yet
	if existingTx == nil {
		refundMetadata := map[string]interface{}{
			"refund_reason": req.Reason,
		}
		_, _, err := s.CreatePendingTransaction(ctx, CreatePendingTransactionParams{
			ID:                txID,
			GroupID:           &groupID,
			MerchantID:        merchantID,
			CustomerID:        domainTxs[0].CustomerID,
			Amount:            finalRefundAmount,
			Currency:          domainTxs[0].Currency,
			Type:              domain.TransactionTypeRefund,
			PaymentMethodType: domain.PaymentMethodType(domainTxs[0].PaymentMethodType),
			PaymentMethodID:   stringToUUIDPtr(domainTxs[0].PaymentMethodID),
			Metadata:          refundMetadata,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create pending transaction: %w", err)
		}

		s.logger.Info("Created pending REFUND transaction",
			zap.String("transaction_id", txID.String()),
		)
	}

	// Call EPX Server Post API for refund
	s.logger.Info("Calling EPX for refund",
		zap.String("auth_bric", authBRIC),
		zap.String("amount", finalRefundAmount.String()),
	)

	// Generate deterministic numeric TRAN_NBR from transaction UUID
	// This ensures idempotency - same UUID always produces same TRAN_NBR
	epxTranNbr := util.UUIDToEPXTranNbr(txID)

	epxReq := &adapterports.ServerPostRequest{
		CustNbr:          merchant.CustNbr,
		MerchNbr:         merchant.MerchNbr,
		DBAnbr:           merchant.DbaNbr,
		TerminalNbr:      merchant.TerminalNbr,
		TransactionType:  adapterports.TransactionTypeRefund,
		Amount:           finalRefundAmount.StringFixed(2),
		PaymentType:      adapterports.PaymentMethodType(domainTxs[0].PaymentMethodType),
		OriginalAuthGUID: authBRIC,   // Reference to CAPTURE (or AUTH if SALE)
		TranNbr:          epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:        "",
		CustomerID:       stringOrEmpty(domainTxs[0].CustomerID),
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX refund failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Update pending transaction with EPX response
	metadata := map[string]interface{}{
		"refund_reason":  req.Reason,
		"auth_resp_text": epxResp.AuthRespText,
		"auth_avs":       epxResp.AuthAVS,
		"auth_cvv2":      epxResp.AuthCVV2,
	}
	err = s.UpdateTransactionWithEPXResponse(
		ctx,
		epxTranNbr,
		domainTxs[0].CustomerID,
		&epxResp.AuthGUID,
		&epxResp.AuthResp,
		&epxResp.AuthCode,
		&epxResp.AuthCardType,
		metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update transaction with EPX response: %w", err)
	}

	// Fetch the updated transaction
	updatedTx, err := s.db.Queries().GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated transaction: %w", err)
	}
	transaction = sqlcToDomain(&updatedTx)

	s.logger.Info("Refund completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("group_id", groupID.String()),
		zap.String("amount", finalRefundAmount.String()),
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

// GetTransactionByIdempotencyKey retrieves a transaction by idempotency key
// Note: idempotency_key IS the transaction ID (no separate column)
func (s *paymentService) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	// Idempotency key is the transaction ID (UUID)
	txID, err := uuid.Parse(key)
	if err != nil {
		return nil, fmt.Errorf("invalid idempotency_key format: %w", err)
	}

	dbTx, err := s.db.Queries().GetTransactionByID(ctx, txID)
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
		MerchantID:      toNullableUUID(filters.MerchantID),
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
		MerchantID:      toNullableUUID(filters.MerchantID),
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
// that can be voided/refunded. Note: auth_guid (BRIC) is stored in each transaction record
// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation
func isUniqueViolation(err error) bool {
	// Check for Postgres unique_violation error code (23505)
	// This occurs when trying to insert duplicate (group_id, type)
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "unique constraint") ||
		strings.Contains(err.Error(), "duplicate key") ||
		strings.Contains(err.Error(), "23505")
}

func findOriginalTransaction(transactions []*domain.Transaction) (*domain.Transaction, error) {
	var originalTx *domain.Transaction

	for _, tx := range transactions {
		// Look for approved transactions that can be voided/refunded
		// Note: auth_guid is stored in each transaction record
		if tx.Status == domain.TransactionStatusApproved &&
			(tx.Type == domain.TransactionTypeSale ||
				tx.Type == domain.TransactionTypeAuth ||
				tx.Type == domain.TransactionTypeCapture) {
			// Prefer the most recent transaction (latest created_at)
			// For AUTH → CAPTURE workflow, we want the CAPTURE, not the AUTH
			// For SALE, we want the SALE itself
			if originalTx == nil || tx.CreatedAt.After(originalTx.CreatedAt) {
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
		MerchantID:        dbTx.MerchantID.String(),
		Amount:            decimal.NewFromBigInt(dbTx.Amount.Int, dbTx.Amount.Exp),
		Currency:          dbTx.Currency,
		Type:              domain.TransactionType(dbTx.Type),
		PaymentMethodType: domain.PaymentMethodType(dbTx.PaymentMethodType),
		CreatedAt:         dbTx.CreatedAt,
		UpdatedAt:         dbTx.UpdatedAt,
	}

	// Status is a GENERATED column in database (pgtype.Text)
	if dbTx.Status.Valid {
		tx.Status = domain.TransactionStatus(dbTx.Status.String)
	}

	if dbTx.CustomerID.Valid {
		customerID := dbTx.CustomerID.String
		tx.CustomerID = &customerID
	}

	if dbTx.PaymentMethodID.Valid {
		pmID := dbTx.PaymentMethodID.String()
		tx.PaymentMethodID = &pmID
	}

	if dbTx.SubscriptionID.Valid {
		subID := uuid.UUID(dbTx.SubscriptionID.Bytes).String()
		tx.SubscriptionID = &subID
	}

	// Note: auth_guid (BRIC) is stored in each transaction record
	if dbTx.AuthGuid.Valid {
		tx.AuthGUID = dbTx.AuthGuid.String
	}

	// AuthResp is a string (not pgtype.Text)
	if dbTx.AuthResp != "" {
		tx.AuthResp = &dbTx.AuthResp
	}
	if dbTx.AuthCode.Valid {
		tx.AuthCode = &dbTx.AuthCode.String
	}
	if dbTx.AuthCardType.Valid {
		tx.AuthCardType = &dbTx.AuthCardType.String
	}

	// Parse metadata JSONB and extract display-only fields
	if len(dbTx.Metadata) > 0 {
		if err := json.Unmarshal(dbTx.Metadata, &tx.Metadata); err != nil {
			// Log error but don't fail the entire operation
			// Metadata is supplementary information
			tx.Metadata = nil
		} else {
			// Extract display-only fields from metadata for API compatibility
			if authRespText, ok := tx.Metadata["auth_resp_text"].(string); ok {
				tx.AuthRespText = &authRespText
			}
			if authAvs, ok := tx.Metadata["auth_avs"].(string); ok {
				tx.AuthAVS = &authAvs
			}
			if authCvv2, ok := tx.Metadata["auth_cvv2"].(string); ok {
				tx.AuthCVV2 = &authCvv2
			}
		}
	}

	// Transaction ID is the idempotency key
	txID := dbTx.ID.String()
	tx.IdempotencyKey = &txID

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

// toNullableUUIDFromUUID converts *uuid.UUID to pgtype.UUID
func toNullableUUIDFromUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
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

// stringToUUIDPtr converts an optional string to a UUID pointer
// Returns nil if string is empty or nil
func stringToUUIDPtr(s *string) *uuid.UUID {
	if s == nil || *s == "" {
		return nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil
	}
	return &id
}
