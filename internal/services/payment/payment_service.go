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
	queries          sqlc.Querier
	txManager        database.TransactionManager
	serverPost       adapterports.ServerPostAdapter
	secretManager    adapterports.SecretManagerAdapter
	merchantResolver *authorization.MerchantResolver
	logger           *zap.Logger
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	queries sqlc.Querier,
	txManager database.TransactionManager,
	serverPost adapterports.ServerPostAdapter,
	secretManager adapterports.SecretManagerAdapter,
	merchantResolver *authorization.MerchantResolver,
	logger *zap.Logger,
) ports.PaymentService {
	return &paymentService{
		queries:          queries,
		txManager:        txManager,
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
func (s *paymentService) validateTransactionAccess(ctx context.Context, tx *domain.Transaction) error {
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
		zap.Int64("amount_cents", req.AmountCents),
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
	// Try parsing as UUID first, otherwise treat as slug
	var merchant sqlc.Merchant
	merchantID, err := uuid.Parse(resolvedMerchantID)
	if err == nil {
		// Valid UUID - lookup by ID
		merchant, err = s.queries.GetMerchantByID(ctx, merchantID)
		if err != nil {
			return nil, fmt.Errorf("failed to get merchant by ID: %w", err)
		}
	} else {
		// Not a UUID - lookup by slug
		merchant, err = s.queries.GetMerchantBySlug(ctx, resolvedMerchantID)
		if err != nil {
			return nil, fmt.Errorf("failed to get merchant by slug: %w", err)
		}
		merchantID = merchant.ID // Use the merchant's UUID for subsequent operations
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

	// Determine payment method type and auth credentials
	var authGUID string
	var paymentMethodUUID *uuid.UUID                                                    // Reuse parsed UUID
	var paymentMethodType domain.PaymentMethodType = domain.PaymentMethodTypeCreditCard // Default

	if req.PaymentMethodID != nil {
		// Using saved payment method - parse UUID once
		pmID, err := uuid.Parse(*req.PaymentMethodID)
		if err != nil {
			return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
		}
		paymentMethodUUID = &pmID

		dbPM, err := s.queries.GetPaymentMethodByID(ctx, pmID)
		if err != nil {
			return nil, fmt.Errorf("failed to get payment method: %w", err)
		}

		// Convert to domain model to use grace period logic
		domainPM := sqlcPaymentMethodToDomain(&dbPM)
		paymentMethodType = domainPM.PaymentType

		// Check if payment method can be used for this amount (ACH must be verified)
		canUse, reason := domainPM.CanUseForAmount(req.AmountCents)
		if !canUse {
			// Map reason strings to domain errors
			switch reason {
			case "payment method is not active":
				return nil, domain.ErrPaymentMethodInactive
			case "credit card is expired":
				return nil, domain.ErrPaymentMethodExpired
			case "ACH account must be verified before use":
				return nil, domain.ErrPaymentMethodNotVerified
			default:
				return nil, fmt.Errorf("payment method cannot be used: %s", reason)
			}
		}

		authGUID = dbPM.Bric

	} else if req.PaymentToken != nil {
		// Using one-time token
		authGUID = *req.PaymentToken

	} else {
		return nil, fmt.Errorf("payment method required: provide payment_method_id or payment_token")
	}

	// Generate deterministic numeric TRAN_NBR from transaction UUID (parsed from idempotency key above)
	// This ensures idempotency - same UUID always produces same TRAN_NBR
	epxTranNbr := util.UUIDToEPXTranNbr(txID)

	// Determine transaction type based on payment method (credit card vs ACH)
	var transactionType adapterports.TransactionType
	if paymentMethodType == domain.PaymentMethodTypeACH {
		transactionType = adapterports.TransactionTypeACHDebit // CKC2 - ACH Debit/Sale
	} else {
		transactionType = adapterports.TransactionTypeSale // CCE1 - CC Sale (auth + capture)
	}

	// For BRIC-based transactions, set Card Entry Method to "Z"
	cardEntryMethod := "Z" // BRIC/token

	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         merchant.CustNbr,
		MerchNbr:        merchant.MerchNbr,
		DBAnbr:          merchant.DbaNbr,
		TerminalNbr:     merchant.TerminalNbr,
		TransactionType: transactionType,
		Amount:          centsToDecimalString(req.AmountCents),
		PaymentType:     adapterports.PaymentMethodType(paymentMethodType),
		TranNbr:         epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:       "SALE",     // Transaction class: SALE = auth + capture combined
		CustomerID:      stringOrEmpty(req.CustomerID),
		CardEntryMethod: &cardEntryMethod, // "Z" for BRIC-based transactions
	}

	// EPX uses different fields for ACH vs credit card BRIC transactions
	// ACH: ORIG_AUTH_GUID (reference to previous ACH transaction)
	// Credit Card: AUTH_GUID (storage token)
	if paymentMethodType == domain.PaymentMethodTypeACH {
		epxReq.OriginalAuthGUID = authGUID
	} else {
		epxReq.AuthGUID = authGUID
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX transaction failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Save transaction to database using WithTx for transaction safety
	var transaction *domain.Transaction
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Amount is already in cents - use directly
		amountCents := req.AmountCents

		// Parse customer ID to UUID if provided
		var customerIDUUID pgtype.UUID
		if req.CustomerID != nil && *req.CustomerID != "" {
			cid, err := uuid.Parse(*req.CustomerID)
			if err != nil {
				return fmt.Errorf("invalid customer_id format: %w", err)
			}
			customerIDUUID = pgtype.UUID{Bytes: cid, Valid: true}
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
		// parent_transaction_id is NULL for first transaction (SALE)
		params := sqlc.CreateTransactionParams{
			ID:                  txID,
			MerchantID:          merchantID,
			CustomerID:          customerIDUUID,
			AmountCents:         amountCents,
			Currency:            req.Currency,
			Type:                string(domain.TransactionTypeSale), // SALE for all purchases (credit, ACH, PIN-less debit)
			PaymentMethodType:   string(paymentMethodType),          // Use actual type: credit_card, ach, or pinless_debit
			PaymentMethodID:     toNullableUUID(req.PaymentMethodID),
			TranNbr:             pgtype.Text{String: epxTranNbr, Valid: true},
			AuthGuid:            toNullableText(&epxResp.AuthGUID), // Store transaction's BRIC
			AuthResp:            pgtype.Text{String: epxResp.AuthResp, Valid: true},
			AuthCode:            toNullableText(&epxResp.AuthCode),
			AuthCardType:        toNullableText(&epxResp.AuthCardType),
			Metadata:            metadataJSON,
			ParentTransactionID: pgtype.UUID{}, // NULL for first transaction
			ProcessedAt:         pgtype.Timestamptz{},
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
		zap.Int64("amount_cents", req.AmountCents),
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
	// Try parsing as UUID first, otherwise treat as slug
	var merchant sqlc.Merchant
	merchantID, err := uuid.Parse(resolvedMerchantID)
	if err == nil {
		// Valid UUID - lookup by ID
		merchant, err = s.queries.GetMerchantByID(ctx, merchantID)
		if err != nil {
			return nil, fmt.Errorf("failed to get merchant by ID: %w", err)
		}
	} else {
		// Not a UUID - lookup by slug
		merchant, err = s.queries.GetMerchantBySlug(ctx, resolvedMerchantID)
		if err != nil {
			return nil, fmt.Errorf("failed to get merchant by slug: %w", err)
		}
		merchantID = merchant.ID // Use the merchant's UUID for subsequent operations
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

		pm, err := s.queries.GetPaymentMethodByID(ctx, pmID)
		if err != nil {
			return nil, fmt.Errorf("failed to get payment method: %w", err)
		}
		authGUID = pm.Bric
	} else if req.PaymentToken != nil {
		authGUID = *req.PaymentToken
	} else {
		return nil, fmt.Errorf("either payment_method_id or payment_token is required")
	}

	// Generate deterministic numeric TRAN_NBR from transaction UUID (parsed from idempotency key above)
	// This ensures idempotency - same UUID always produces same TRAN_NBR
	epxTranNbr := util.UUIDToEPXTranNbr(txID)

	// Call EPX Server Post API for authorization only
	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         merchant.CustNbr,
		MerchNbr:        merchant.MerchNbr,
		DBAnbr:          merchant.DbaNbr,
		TerminalNbr:     merchant.TerminalNbr,
		TransactionType: adapterports.TransactionTypeAuthOnly,
		Amount:          centsToDecimalString(req.AmountCents),
		PaymentType:     adapterports.PaymentMethodTypeCreditCard,
		AuthGUID:        authGUID,
		TranNbr:         epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:       "AUTH",     // Transaction class: AUTH = authorization-only, requires capture
		CustomerID:      stringOrEmpty(req.CustomerID),
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX authorization failed", zap.Error(err))
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	// Save transaction to database
	var transaction *domain.Transaction
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Amount is already in cents - use directly
		amountCents := req.AmountCents

		// Parse customer ID to UUID if provided
		var customerIDUUID pgtype.UUID
		if req.CustomerID != nil && *req.CustomerID != "" {
			cid, err := uuid.Parse(*req.CustomerID)
			if err != nil {
				return fmt.Errorf("invalid customer_id format: %w", err)
			}
			customerIDUUID = pgtype.UUID{Bytes: cid, Valid: true}
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
		// parent_transaction_id is NULL for first transaction (AUTH)
		params := sqlc.CreateTransactionParams{
			ID:                  txID,
			MerchantID:          merchantID,
			CustomerID:          customerIDUUID,
			AmountCents:         amountCents,
			Currency:            "USD",
			Type:                string(domain.TransactionTypeAuth),
			PaymentMethodType:   string(domain.PaymentMethodTypeCreditCard),
			PaymentMethodID:     toNullableUUID(req.PaymentMethodID),
			TranNbr:             pgtype.Text{String: epxTranNbr, Valid: true},
			AuthGuid:            toNullableText(&epxResp.AuthGUID), // Store AUTH's BRIC
			AuthResp:            pgtype.Text{String: epxResp.AuthResp, Valid: true},
			AuthCode:            toNullableText(&epxResp.AuthCode),
			AuthCardType:        toNullableText(&epxResp.AuthCardType),
			Metadata:            metadataJSON,
			ParentTransactionID: pgtype.UUID{}, // NULL for first transaction
			ProcessedAt:         pgtype.Timestamptz{},
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
	originalTx, err := s.queries.GetTransactionByID(ctx, originalTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get original transaction: %w", err)
	}

	// Validate transaction access
	domainTx := sqlcToDomain(&originalTx)
	if err := s.validateTransactionAccess(ctx, domainTx); err != nil {
		return nil, err
	}

	// Validate amount if provided
	if req.AmountCents != nil {
		if *req.AmountCents <= 0 {
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
	existingTxDB, existErr := s.queries.GetTransactionByID(ctx, txID)
	if existErr == nil {
		existingTx = &existingTxDB
		// Transaction exists - check if it's complete (has auth_resp)
		if existingTx.AuthResp.Valid && existingTx.AuthResp.String != "" {
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

	s.logger.Info("Processing capture",
		zap.String("capture_transaction_id", txID.String()),
		zap.String("original_transaction_id", req.TransactionID),
	)

	var transaction *domain.Transaction
	var groupTxs []sqlc.GetTransactionTreeRow

	// Use database transaction for consistency
	// Note: We rely on idempotency (transaction.id as PK) rather than row-level locks on parent_transaction_id
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Get transaction tree (includes root + all descendants)
		var err error
		groupTxs, err = q.GetTransactionTree(ctx, originalTxID)
		if err != nil {
			return fmt.Errorf("failed to get transaction tree: %w", err)
		}

		if len(groupTxs) == 0 {
			return fmt.Errorf("no transactions found for parent %s", originalTxID.String())
		}

		// Convert to domain transactions
		domainTxs := make([]*domain.Transaction, len(groupTxs))
		for i, tx := range groupTxs {
			// Convert GetTransactionTreeRow to Transaction for sqlcToDomain
			sqlcTx := sqlc.Transaction(tx)
			domainTxs[i] = sqlcToDomain(&sqlcTx)
		}

		// Compute current state using WAL
		state := ComputeGroupState(domainTxs)

		// Validate capture is allowed
		captureAmountCents := state.ActiveAuthAmount
		if req.AmountCents != nil {
			captureAmountCents = *req.AmountCents
		}

		canCapture, reason := state.CanCapture(captureAmountCents)
		if !canCapture {
			s.logger.Warn("Capture validation failed",
				zap.String("capture_transaction_id", txID.String()),
				zap.String("reason", reason),
			)
			return domain.ErrTransactionCannotBeCaptured
		}

		s.logger.Info("Capture validation passed",
			zap.String("auth_bric", state.ActiveAuthBRIC),
			zap.String("capture_amount", formatCentsForLog(captureAmountCents)),
			zap.String("remaining", formatCentsForLog(state.ActiveAuthAmount-state.CapturedAmount)),
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
	groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, originalTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction tree: %w", err)
	}

	domainTxsRefetch := make([]*domain.Transaction, len(groupTxsRefetch))
	for i, tx := range groupTxsRefetch {
		// Convert GetTransactionTreeRow to Transaction for sqlcToDomain
		sqlcTx := sqlc.Transaction(tx)
		domainTxsRefetch[i] = sqlcToDomain(&sqlcTx)
	}
	state := ComputeGroupState(domainTxsRefetch)

	merchantID := uuid.MustParse(domainTxsRefetch[0].MerchantID)
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merchant: %w", err)
	}

	// Determine capture amount (use full auth amount if not specified)
	finalCaptureAmountCents := state.ActiveAuthAmount
	if req.AmountCents != nil {
		finalCaptureAmountCents = *req.AmountCents
	}

	// Get BRIC for CAPTURE operation (uses AUTH's BRIC)
	authBRIC := state.GetBRICForOperation(domain.TransactionTypeCapture)

	// Create pending transaction BEFORE calling EPX
	// Only create if transaction doesn't exist yet
	if existingTx == nil {
		captureMetadata := map[string]interface{}{}
		_, _, err := s.CreatePendingTransaction(ctx, CreatePendingTransactionParams{
			ID:                  txID,
			ParentTransactionID: &originalTxID, // Parent transaction ID (the AUTH)
			MerchantID:          merchantID,
			CustomerID:          domainTxsRefetch[0].CustomerID,
			Amount:              finalCaptureAmountCents,
			Currency:            domainTxsRefetch[0].Currency,
			Type:                domain.TransactionTypeCapture,
			PaymentMethodType:   domain.PaymentMethodType(domainTxsRefetch[0].PaymentMethodType),
			PaymentMethodID:     stringToUUIDPtr(domainTxsRefetch[0].PaymentMethodID),
			Metadata:            captureMetadata,
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
		zap.String("amount", formatCentsForLog(finalCaptureAmountCents)),
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
		Amount:           centsToDecimalString(finalCaptureAmountCents),
		PaymentType:      adapterports.PaymentMethodTypeCreditCard,
		OriginalAuthGUID: authBRIC,   // Reference to AUTH transaction
		TranNbr:          epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:        "",         // No BATCH_ID for capture
		CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
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
		domainTxsRefetch[0].CustomerID,
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
	updatedTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated transaction: %w", err)
	}
	transaction = sqlcToDomain(&updatedTx)

	s.logger.Info("Capture completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("parent_transaction_id", originalTxID.String()),
		zap.String("status", string(transaction.Status)),
	)

	return transaction, nil
}

// Void cancels an authorized or captured payment
// Uses WAL-based state computation for consistency
func (s *paymentService) Void(ctx context.Context, req *ports.VoidRequest) (*domain.Transaction, error) {
	// Validate inputs first (fail-fast)
	if s.serverPost == nil {
		return nil, fmt.Errorf("server post adapter not initialized")
	}

	if req.ParentTransactionID == "" {
		return nil, fmt.Errorf("parent_transaction_id is required")
	}

	parentTxID, err := uuid.Parse(req.ParentTransactionID)
	if err != nil {
		return nil, fmt.Errorf("invalid parent_transaction_id format: %w", err)
	}

	// Get parent transaction to validate access
	parentTx, err := s.queries.GetTransactionByID(ctx, parentTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent transaction: %w", err)
	}

	// Get transaction tree (includes parent + all descendants)
	groupTxs, err := s.queries.GetTransactionTree(ctx, parentTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction tree: %w", err)
	}

	// Validate access using the parent transaction
	firstTx := sqlcToDomain(&parentTx)
	if err := s.validateTransactionAccess(ctx, firstTx); err != nil {
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
	existingTxDB, existErr := s.queries.GetTransactionByID(ctx, txID)
	if existErr == nil {
		existingTx = &existingTxDB
		// Transaction exists - check if it's complete (has auth_resp)
		if existingTx.AuthResp.Valid && existingTx.AuthResp.String != "" {
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
		zap.String("parent_transaction_id", req.ParentTransactionID),
	)

	var transaction *domain.Transaction
	var voidAmountCents int64
	var originalTxType domain.TransactionType

	// Use database transaction for consistency
	// Note: We rely on idempotency (transaction.id as PK) rather than row-level locks on parent_transaction_id
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Convert to domain transactions (reuse groupTxs from earlier GetTransactionTree call)
		domainTxs := make([]*domain.Transaction, len(groupTxs))
		for i, tx := range groupTxs {
			// Convert GetTransactionTreeRow to Transaction for sqlcToDomain
			sqlcTx := sqlc.Transaction(tx)
			domainTxs[i] = sqlcToDomain(&sqlcTx)
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

		voidAmountCents = originalAuth.AmountCents
		originalTxType = originalAuth.Type

		s.logger.Info("Void validation passed",
			zap.String("auth_bric", state.ActiveAuthBRIC),
			zap.String("void_amount", formatCentsForLog(voidAmountCents)),
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
	groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, parentTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction tree: %w", err)
	}

	domainTxsRefetch := make([]*domain.Transaction, len(groupTxsRefetch))
	for i, tx := range groupTxsRefetch {
		sqlcTx := sqlc.Transaction(tx)
		domainTxsRefetch[i] = sqlcToDomain(&sqlcTx)
	}
	state := ComputeGroupState(domainTxsRefetch)

	merchantID := uuid.MustParse(domainTxsRefetch[0].MerchantID)
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
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
			ID:                  txID,
			ParentTransactionID: &parentTxID,
			MerchantID:          merchantID,
			CustomerID:          domainTxsRefetch[0].CustomerID,
			Amount:              voidAmountCents,
			Currency:            domainTxsRefetch[0].Currency,
			Type:                domain.TransactionTypeVoid,
			PaymentMethodType:   domain.PaymentMethodType(domainTxsRefetch[0].PaymentMethodType),
			PaymentMethodID:     stringToUUIDPtr(domainTxsRefetch[0].PaymentMethodID),
			Metadata:            voidMetadata,
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
		zap.String("amount", formatCentsForLog(voidAmountCents)),
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
		Amount:           centsToDecimalString(voidAmountCents),
		PaymentType:      adapterports.PaymentMethodType(domainTxsRefetch[0].PaymentMethodType),
		OriginalAuthGUID: authBRIC,   // Reference to AUTH transaction
		TranNbr:          epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:        "",
		CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
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
		domainTxsRefetch[0].CustomerID,
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
	updatedTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated transaction: %w", err)
	}
	transaction = sqlcToDomain(&updatedTx)

	s.logger.Info("Void completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("parent_transaction_id", parentTxID.String()),
		zap.String("status", string(transaction.Status)),
	)

	return transaction, nil
}

// Refund returns funds to the customer
// Uses WAL-based state computation for consistency
func (s *paymentService) Refund(ctx context.Context, req *ports.RefundRequest) (*domain.Transaction, error) {
	// Validate inputs first (fail-fast)
	if s.serverPost == nil {
		return nil, fmt.Errorf("server post adapter not initialized")
	}

	if req.ParentTransactionID == "" {
		return nil, fmt.Errorf("parent_transaction_id is required")
	}

	parentTxID, err := uuid.Parse(req.ParentTransactionID)
	if err != nil {
		return nil, fmt.Errorf("invalid parent_transaction_id format: %w", err)
	}

	// Get parent transaction to validate access
	parentTx, err := s.queries.GetTransactionByID(ctx, parentTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent transaction: %w", err)
	}

	// Get transaction tree (includes parent + all descendants)
	groupTxs, err := s.queries.GetTransactionTree(ctx, parentTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction tree: %w", err)
	}

	// Validate access using the parent transaction
	firstTx := sqlcToDomain(&parentTx)
	if err := s.validateTransactionAccess(ctx, firstTx); err != nil {
		return nil, err
	}

	// Validate amount if provided
	var refundAmountCents int64
	if req.AmountCents != nil {
		refundAmountCents = *req.AmountCents
		if refundAmountCents <= 0 {
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
	existingTxDB, existErr := s.queries.GetTransactionByID(ctx, txID)
	if existErr == nil {
		existingTx = &existingTxDB
		// Transaction exists - check if it's complete (has auth_resp)
		if existingTx.AuthResp.Valid && existingTx.AuthResp.String != "" {
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
		zap.String("parent_transaction_id", req.ParentTransactionID),
		zap.String("reason", req.Reason),
	)

	var transaction *domain.Transaction
	var finalRefundAmountCents int64

	// Use database transaction for consistency
	// Note: We rely on idempotency (transaction.id as PK) rather than row-level locks on parent_transaction_id
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Convert to domain transactions (reuse groupTxs from earlier GetTransactionTree call)
		domainTxs := make([]*domain.Transaction, len(groupTxs))
		for i, tx := range groupTxs {
			// Convert GetTransactionTreeRow to Transaction for sqlcToDomain
			sqlcTx := sqlc.Transaction(tx)
			domainTxs[i] = sqlcToDomain(&sqlcTx)
		}

		// Compute current state using WAL
		state := ComputeGroupState(domainTxs)

		// Determine refund amount (use full captured amount if not specified)
		finalRefundAmountCents = state.CapturedAmount
		if req.AmountCents != nil {
			finalRefundAmountCents = refundAmountCents // Use pre-validated amount
		}

		// Validate refund is allowed
		canRefund, reason := state.CanRefund(finalRefundAmountCents)
		if !canRefund {
			s.logger.Warn("Refund validation failed",
				zap.String("parent_transaction_id", req.ParentTransactionID),
				zap.String("reason", reason),
			)
			return domain.ErrTransactionCannotBeRefunded
		}

		s.logger.Info("Refund validation passed",
			zap.String("captured_amount", formatCentsForLog(state.CapturedAmount)),
			zap.String("refunded_amount", formatCentsForLog(state.RefundedAmount)),
			zap.String("refund_amount", formatCentsForLog(finalRefundAmountCents)),
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
	groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, parentTxID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction tree: %w", err)
	}

	domainTxsRefetch := make([]*domain.Transaction, len(groupTxsRefetch))
	for i, tx := range groupTxsRefetch {
		sqlcTx := sqlc.Transaction(tx)
		domainTxsRefetch[i] = sqlcToDomain(&sqlcTx)
	}
	state := ComputeGroupState(domainTxsRefetch)

	merchantID := uuid.MustParse(domainTxsRefetch[0].MerchantID)
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
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
			ID:                  txID,
			ParentTransactionID: &parentTxID,
			MerchantID:          merchantID,
			CustomerID:          domainTxsRefetch[0].CustomerID,
			Amount:              finalRefundAmountCents,
			Currency:            domainTxsRefetch[0].Currency,
			Type:                domain.TransactionTypeRefund,
			PaymentMethodType:   domain.PaymentMethodType(domainTxsRefetch[0].PaymentMethodType),
			PaymentMethodID:     stringToUUIDPtr(domainTxsRefetch[0].PaymentMethodID),
			Metadata:            refundMetadata,
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
		zap.String("amount", formatCentsForLog(finalRefundAmountCents)),
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
		Amount:           centsToDecimalString(finalRefundAmountCents),
		PaymentType:      adapterports.PaymentMethodType(domainTxsRefetch[0].PaymentMethodType),
		OriginalAuthGUID: authBRIC,   // Reference to CAPTURE (or AUTH if SALE)
		TranNbr:          epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:        "",
		CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
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
		domainTxsRefetch[0].CustomerID,
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
	updatedTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated transaction: %w", err)
	}
	transaction = sqlcToDomain(&updatedTx)

	s.logger.Info("Refund completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("parent_transaction_id", parentTxID.String()),
		zap.String("amount", formatCentsForLog(finalRefundAmountCents)),
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

	dbTx, err := s.queries.GetTransactionByID(ctx, txID)
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

	dbTx, err := s.queries.GetTransactionByID(ctx, txID)
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
	// MerchantID is required
	if filters.MerchantID == nil {
		return nil, 0, fmt.Errorf("merchant_id is required")
	}
	merchantID, err := uuid.Parse(*filters.MerchantID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid merchant_id format: %w", err)
	}

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
		MerchantID:          merchantID,
		CustomerID:          toNullableUUID(filters.CustomerID),
		SubscriptionID:      toNullableUUID(filters.SubscriptionID),
		ParentTransactionID: toNullableUUID(filters.ParentTransactionID),
		Status:              toNullableText(filters.Status),
		Type:                toNullableText(filters.Type),
		PaymentMethodID:     toNullableUUID(filters.PaymentMethodID),
		LimitVal:            int32(limit),
		OffsetVal:           int32(offset),
	}

	dbTxs, err := s.queries.ListTransactions(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list transactions: %w", err)
	}

	countParams := sqlc.CountTransactionsParams{
		MerchantID:          merchantID,
		CustomerID:          toNullableUUID(filters.CustomerID),
		SubscriptionID:      toNullableUUID(filters.SubscriptionID),
		ParentTransactionID: toNullableUUID(filters.ParentTransactionID),
		Status:              toNullableText(filters.Status),
		Type:                toNullableText(filters.Type),
		PaymentMethodID:     toNullableUUID(filters.PaymentMethodID),
	}

	count, err := s.queries.CountTransactions(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	transactions := make([]*domain.Transaction, len(dbTxs))
	for i, dbTx := range dbTxs {
		transactions[i] = sqlcToDomain(&dbTx)
	}

	return transactions, int(count), nil
}

// GetTransactionsByGroup retrieves all transactions in a group (parent + children) using parent_transaction_id
func (s *paymentService) GetTransactionsByGroup(ctx context.Context, parentTransactionID string) ([]*domain.Transaction, error) {
	parentID, err := uuid.Parse(parentTransactionID)
	if err != nil {
		return nil, fmt.Errorf("invalid parent transaction ID: %w", err)
	}

	// Get transaction tree (includes parent + all descendants)
	groupTxs, err := s.queries.GetTransactionTree(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction tree: %w", err)
	}

	transactions := make([]*domain.Transaction, len(groupTxs))
	for i, tx := range groupTxs {
		sqlcTx := sqlc.Transaction(tx)
		transactions[i] = sqlcToDomain(&sqlcTx)
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

// Helper functions to convert between sqlc and domain models

func sqlcToDomain(dbTx *sqlc.Transaction) *domain.Transaction {
	var parentTxID *string
	if dbTx.ParentTransactionID.Valid {
		id := uuid.UUID(dbTx.ParentTransactionID.Bytes).String()
		parentTxID = &id
	}

	var customerID *string
	if dbTx.CustomerID.Valid {
		id := uuid.UUID(dbTx.CustomerID.Bytes).String()
		customerID = &id
	}

	var pmID *string
	if dbTx.PaymentMethodID.Valid {
		id := uuid.UUID(dbTx.PaymentMethodID.Bytes).String()
		pmID = &id
	}

	var subscriptionID *string
	if dbTx.SubscriptionID.Valid {
		id := uuid.UUID(dbTx.SubscriptionID.Bytes).String()
		subscriptionID = &id
	}

	tx := &domain.Transaction{
		ID:                  dbTx.ID.String(),
		ParentTransactionID: parentTxID,
		MerchantID:          dbTx.MerchantID.String(),
		CustomerID:          customerID,
		SubscriptionID:      subscriptionID,
		AmountCents:         dbTx.AmountCents,
		Currency:            dbTx.Currency,
		Type:                domain.TransactionType(dbTx.Type),
		PaymentMethodType:   domain.PaymentMethodType(dbTx.PaymentMethodType),
		PaymentMethodID:     pmID,
		CreatedAt:           dbTx.CreatedAt,
		UpdatedAt:           dbTx.UpdatedAt,
	}

	// Status is a GENERATED column in database (pgtype.Text)
	if dbTx.Status.Valid {
		tx.Status = domain.TransactionStatus(dbTx.Status.String)
	}

	// Note: auth_guid (BRIC) is stored in each transaction record
	if dbTx.AuthGuid.Valid {
		tx.AuthGUID = dbTx.AuthGuid.String
	}

	// AuthResp is pgtype.Text
	if dbTx.AuthResp.Valid {
		tx.AuthResp = &dbTx.AuthResp.String
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

// centsToDecimalString converts cents (int64) to a decimal string for EPX API
// Example: 1050 -> "10.50"
func centsToDecimalString(cents int64) string {
	d := decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))
	return d.StringFixed(2)
}

// formatCentsForLog formats cents (int64) as a dollar amount for logging
// Example: 1050 -> "$10.50"
func formatCentsForLog(cents int64) string {
	return "$" + centsToDecimalString(cents)
}

// sqlcPaymentMethodToDomain converts sqlc model to domain model
func sqlcPaymentMethodToDomain(dbPM *sqlc.CustomerPaymentMethod) *domain.PaymentMethod {
	pm := &domain.PaymentMethod{
		ID:           dbPM.ID.String(),
		MerchantID:   dbPM.MerchantID.String(),
		CustomerID:   dbPM.CustomerID.String(),
		PaymentType:  domain.PaymentMethodType(dbPM.PaymentType),
		PaymentToken: dbPM.Bric,
		LastFour:     dbPM.LastFour,
		IsDefault:    dbPM.IsDefault.Bool,
		IsActive:     dbPM.IsActive.Bool,
		IsVerified:   dbPM.IsVerified.Bool,
		CreatedAt:    dbPM.CreatedAt,
		UpdatedAt:    dbPM.UpdatedAt,
	}

	if dbPM.CardBrand.Valid {
		pm.CardBrand = &dbPM.CardBrand.String
	}

	if dbPM.CardExpMonth.Valid {
		expMonth := int(dbPM.CardExpMonth.Int32)
		pm.CardExpMonth = &expMonth
	}

	if dbPM.CardExpYear.Valid {
		expYear := int(dbPM.CardExpYear.Int32)
		pm.CardExpYear = &expYear
	}

	if dbPM.BankName.Valid {
		pm.BankName = &dbPM.BankName.String
	}

	if dbPM.AccountType.Valid {
		pm.AccountType = &dbPM.AccountType.String
	}

	if dbPM.LastUsedAt.Valid {
		pm.LastUsedAt = &dbPM.LastUsedAt.Time
	}

	// ACH Verification fields (from migration 009)
	if dbPM.VerificationStatus.Valid {
		pm.VerificationStatus = &dbPM.VerificationStatus.String
	}

	if dbPM.PrenoteTransactionID.Valid {
		prenoteID := uuid.UUID(dbPM.PrenoteTransactionID.Bytes).String()
		pm.PreNoteTransactionID = &prenoteID
	}

	if dbPM.VerifiedAt.Valid {
		pm.VerifiedAt = &dbPM.VerifiedAt.Time
	}

	if dbPM.VerificationFailureReason.Valid {
		pm.VerificationFailureReason = &dbPM.VerificationFailureReason.String
	}

	// ReturnCount is NOT NULL DEFAULT 0, so always present
	returnCount := int(dbPM.ReturnCount)
	pm.ReturnCount = &returnCount

	if dbPM.DeactivationReason.Valid {
		pm.DeactivationReason = &dbPM.DeactivationReason.String
	}

	if dbPM.DeactivatedAt.Valid {
		pm.DeactivatedAt = &dbPM.DeactivatedAt.Time
	}

	return pm
}
