package payment

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	"github.com/kevin07696/payment-service/internal/converters"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/epxutil"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/internal/services/authorization"
	merchantsvc "github.com/kevin07696/payment-service/internal/services/merchant"
	"go.uber.org/zap"
)

// PaymentServiceConfig holds configuration for payment service
type PaymentServiceConfig struct {
	DefaultACHClass string // Default STD_ENTRY_CLASS for ACH transactions (default: "WEB")
}

// DefaultPaymentServiceConfig returns sensible defaults for payment service
// Reads from environment variables:
// - DEFAULT_ACH_CLASS: Default STD_ENTRY_CLASS for ACH (WEB, TEL, PPD, CCD)
func DefaultPaymentServiceConfig() PaymentServiceConfig {
	defaultACHClass := "WEB" // Internet-initiated (default for e-commerce)
	if v := os.Getenv("DEFAULT_ACH_CLASS"); v != "" {
		defaultACHClass = v
	}
	return PaymentServiceConfig{
		DefaultACHClass: defaultACHClass,
	}
}

// paymentService implements the PaymentService port
type paymentService struct {
	queries                    sqlc.Querier
	txManager                  database.TransactionManager
	serverPost                 ports.ServerPostAdapter
	secretManager              ports.SecretManagerAdapter
	merchantCredentialResolver *authorization.MerchantCredentialResolver
	merchantAuthService        ports.MerchantAuthorizationService
	logger                     *zap.Logger
	config                     PaymentServiceConfig
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	queries sqlc.Querier,
	txManager database.TransactionManager,
	serverPost ports.ServerPostAdapter,
	secretManager ports.SecretManagerAdapter,
	merchantCache *merchantsvc.MerchantCredentialCache,
	logger *zap.Logger,
	config *PaymentServiceConfig,
) ports.PaymentService {
	// Create merchant credential resolver with cache (70% DB load reduction)
	merchantCredentialResolver := authorization.NewMerchantCredentialResolver(
		merchantCache,
		queries,
		secretManager,
		logger,
	)

	// Create service-merchant access checker for authorization
	accessChecker := authorization.NewSQLCServiceMerchantAccessChecker(queries)

	// Create merchant authorization service with access checker
	merchantAuthService := authorization.NewMerchantAuthorizationService(logger, accessChecker)

	// Use default config if not provided
	cfg := DefaultPaymentServiceConfig()
	if config != nil {
		if config.DefaultACHClass != "" {
			cfg.DefaultACHClass = config.DefaultACHClass
		}
	}

	return &paymentService{
		queries:                    queries,
		txManager:                  txManager,
		serverPost:                 serverPost,
		secretManager:              secretManager,
		merchantCredentialResolver: merchantCredentialResolver,
		merchantAuthService:        merchantAuthService,
		logger:                     logger,
		config:                     cfg,
	}
}

// Sale combines authorize and capture in one operation
func (s *paymentService) Sale(ctx context.Context, req *ports.SaleRequest) (*domain.Transaction, error) {
	// Resolve merchant_id from token context
	resolvedMerchantID, err := s.merchantAuthService.ResolveMerchantID(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Processing sale transaction",
		zap.String("merchant_id", resolvedMerchantID),
		zap.Int64("amount_cents", req.AmountCents),
	)

	// Parse transaction ID from idempotency key (required)
	if req.IdempotencyKey == nil {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "idempotency_key")
	}

	txID, err := uuid.Parse(*req.IdempotencyKey)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "idempotency_key")
	}

	// Get merchant credentials using sqlc
	// Try parsing as UUID first, otherwise treat as slug
	var merchant sqlc.Merchant
	merchantID, err := uuid.Parse(resolvedMerchantID)
	if err == nil {
		// Valid UUID - lookup by ID
		merchant, err = s.queries.GetMerchantByID(ctx, merchantID)
		if err != nil {
			return nil, domain.ErrMerchantNotFoundTyped
		}
	} else {
		// Not a UUID - lookup by slug
		merchant, err = s.queries.GetMerchantBySlug(ctx, resolvedMerchantID)
		if err != nil {
			return nil, domain.ErrMerchantNotFoundTyped
		}
		merchantID = merchant.ID // Use the merchant's UUID for subsequent operations
	}

	// Check if merchant is active (Valid must be true and Bool must be true)
	if !merchant.IsActive {
		return nil, domain.ErrMerchantInactiveTyped
	}

	// Get MAC secret from secret manager (will be used for EPX request signing)
	_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		return nil, domain.ErrMerchantCredentialFailed
	}

	// Resolve payment token (validates payment method if needed)
	tokenInfo, err := s.resolvePaymentToken(ctx, req.PaymentMethodID, req.PaymentToken, &req.AmountCents)
	if err != nil {
		return nil, err
	}

	authGUID := tokenInfo.Token
	paymentMethodUUID := tokenInfo.PaymentMethodID
	paymentMethodType := tokenInfo.PaymentMethodType

	// Generate deterministic numeric TRAN_NBR from transaction UUID (parsed from idempotency key above)
	// This ensures idempotency - same UUID always produces same TRAN_NBR
	epxTranNbr := epxutil.UUIDToEPXTranNbr(txID)

	// For BRIC-based transactions, set Card Entry Method to "Z"
	cardEntryMethod := "Z" // BRIC/token
	industryType := "E"    // Ecommerce (required for EPX certification)

	// Default STD_ENTRY_CLASS for ACH transactions if not provided
	// NACHA requires STD_ENTRY_CLASS for all ACH transactions:
	// - WEB: Internet-initiated (default for e-commerce)
	// - TEL: Telephone-initiated (operator enters phone order)
	// - PPD: Prearranged (recurring billing - handled by subscription service)
	// - CCD: Corporate (B2B payments)
	stdEntryClass := req.StdEntryClass
	if paymentMethodType == domain.PaymentMethodTypeACH && stdEntryClass == nil {
		defaultClass := s.config.DefaultACHClass
		stdEntryClass = &defaultClass
		s.logger.Debug("Defaulting STD_ENTRY_CLASS for ACH transaction",
			zap.String("transaction_id", txID.String()),
			zap.String("default_class", defaultClass),
		)
	}

	epxReq := &ports.ServerPostRequest{
		CustNbr:     merchant.CustNbr,
		MerchNbr:    merchant.MerchNbr,
		DBAnbr:      merchant.DbaNbr,
		TerminalNbr: merchant.TerminalNbr,
		// Use semantic operation - adapter determines EPX transaction type
		Operation:       ports.OperationSale,
		Amount:          centsToDecimalString(req.AmountCents),
		PaymentType:     ports.PaymentMethodType(paymentMethodType),
		TranNbr:         epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:       "SALE",     // Transaction class: SALE = auth + capture combined
		CustomerID:      stringOrEmpty(req.CustomerID),
		CardEntryMethod: &cardEntryMethod, // "Z" for BRIC-based transactions
		IndustryType:    &industryType,    // "E" for Ecommerce
		StdEntryClass:   stdEntryClass,    // ACH SEC code: WEB (default), TEL, PPD, CCD
	}

	// EPX uses different fields for ACH vs credit card BRIC transactions
	// ACH: ORIG_AUTH_GUID (reference to previous ACH transaction)
	// Credit Card: AUTH_GUID (storage token)
	if paymentMethodType == domain.PaymentMethodTypeACH {
		epxReq.OriginalAuthGUID = authGUID
	} else {
		epxReq.AuthGUID = authGUID
	}

	// Wrap EPX call with explicit timeout for external service reliability
	epxCtx, epxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer epxCancel()
	epxResp, err := s.serverPost.ProcessTransaction(epxCtx, epxReq)
	if err != nil {
		s.logger.Error("EPX transaction failed", zap.Error(err))
		return nil, domain.ErrGatewayError.WithDetail("operation", "process_sale")
	}

	// Save transaction to database using WithTx for transaction safety
	var transaction *domain.Transaction
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Amount is already in cents - use directly
		amountCents := req.AmountCents

		// Convert customer ID to pgtype.Text if provided
		var customerIDText pgtype.Text
		if req.CustomerID != nil && *req.CustomerID != "" {
			customerIDText = pgtype.Text{String: *req.CustomerID, Valid: true}
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
			CustomerID:          customerIDText,
			OrderID:             converters.ToNullableText(req.OrderID), // Merchant's external order/invoice ID
			AmountCents:         amountCents,
			Currency:            req.Currency,
			Type:                string(domain.TransactionTypeSale), // SALE for all purchases (credit, ACH, PIN-less debit)
			PaymentMethodType:   string(paymentMethodType),          // Use actual type: credit_card, ach, or pinless_debit
			PaymentMethodID:     converters.ToNullableUUID(req.PaymentMethodID),
			TranNbr:             pgtype.Text{String: epxTranNbr, Valid: true},
			AuthGuid:            converters.ToNullableText(&epxResp.AuthGUID), // Store transaction's BRIC
			AuthResp:            pgtype.Text{String: epxResp.AuthResp, Valid: true},
			AuthCode:            converters.ToNullableText(&epxResp.AuthCode),
			AuthCardType:        converters.ToNullableText(&epxResp.AuthCardType),
			Metadata:            metadataJSON,
			ParentTransactionID: pgtype.UUID{}, // NULL for first transaction
			ProcessedAt:         pgtype.Timestamptz{},
		}

		dbTx, err := q.CreateTransaction(ctx, params)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "create_transaction")
		}

		// Mark payment method as used if provided
		if paymentMethodUUID != nil {
			if err := q.MarkPaymentMethodUsed(ctx, *paymentMethodUUID); err != nil {
				s.logger.Warn("Failed to mark payment method as used", zap.Error(err))
			}
		}

		// Convert sqlc transaction to domain transaction
		transaction = sqlcTransactionToDomain(&dbTx)
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
	resolvedMerchantID, err := s.merchantAuthService.ResolveMerchantID(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Processing authorization",
		zap.String("merchant_id", resolvedMerchantID),
		zap.Int64("amount_cents", req.AmountCents),
	)

	// Parse transaction ID from idempotency key (required)
	if req.IdempotencyKey == nil {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "idempotency_key")
	}

	txID, err := uuid.Parse(*req.IdempotencyKey)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "idempotency_key")
	}

	// Check idempotency
	existing, err := s.getTransactionByIdempotencyKey(ctx, *req.IdempotencyKey)
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
			return nil, domain.ErrMerchantNotFoundTyped
		}
	} else {
		// Not a UUID - lookup by slug
		merchant, err = s.queries.GetMerchantBySlug(ctx, resolvedMerchantID)
		if err != nil {
			return nil, domain.ErrMerchantNotFoundTyped
		}
		merchantID = merchant.ID // Use the merchant's UUID for subsequent operations
	}

	if !merchant.IsActive {
		return nil, domain.ErrMerchantInactiveTyped
	}

	// Get MAC secret
	_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		return nil, domain.ErrMerchantCredentialFailed
	}

	// Resolve payment token (no validation needed for authorize)
	tokenInfo, err := s.resolvePaymentToken(ctx, req.PaymentMethodID, req.PaymentToken, nil)
	if err != nil {
		return nil, err
	}

	authGUID := tokenInfo.Token
	paymentMethodUUID := tokenInfo.PaymentMethodID

	// Generate deterministic numeric TRAN_NBR from transaction UUID (parsed from idempotency key above)
	// This ensures idempotency - same UUID always produces same TRAN_NBR
	epxTranNbr := epxutil.UUIDToEPXTranNbr(txID)

	s.logger.Debug("Generated EPX TRAN_NBR",
		zap.String("transaction_id", txID.String()),
		zap.String("tran_nbr", epxTranNbr),
	)

	industryType := "E" // Ecommerce (required for EPX certification)

	// Call EPX Server Post API for authorization only
	epxReq := &ports.ServerPostRequest{
		CustNbr:     merchant.CustNbr,
		MerchNbr:    merchant.MerchNbr,
		DBAnbr:      merchant.DbaNbr,
		TerminalNbr: merchant.TerminalNbr,
		// Use semantic operation - adapter determines EPX transaction type
		Operation:    ports.OperationAuthorize,
		Amount:       centsToDecimalString(req.AmountCents),
		PaymentType:  ports.PaymentMethodTypeCreditCard,
		AuthGUID:     authGUID,
		TranNbr:      epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:    "AUTH",     // Transaction class: AUTH = authorization-only, requires capture
		CustomerID:   stringOrEmpty(req.CustomerID),
		IndustryType: &industryType, // "E" for Ecommerce
		// Don't send CardEntryMethod for BRIC-based AUTH transactions
	}

	s.logger.Debug("Calling EPX ServerPost",
		zap.String("tran_nbr", epxTranNbr),
		zap.String("auth_guid", authGUID),
		zap.String("amount", centsToDecimalString(req.AmountCents)),
		zap.String("transaction_type", string(ports.TransactionTypeAuthOnly)),
		zap.String("tran_group", "AUTH"),
	)

	// Wrap EPX call with explicit timeout for external service reliability
	epxCtx, epxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer epxCancel()
	epxResp, err := s.serverPost.ProcessTransaction(epxCtx, epxReq)
	if err != nil {
		s.logger.Error("EPX authorization failed", zap.Error(err))
		return nil, domain.ErrGatewayError.WithDetail("operation", "process_transaction")
	}

	s.logger.Debug("EPX ServerPost Response",
		zap.String("auth_resp", epxResp.AuthResp),
		zap.String("auth_code", epxResp.AuthCode),
		zap.String("auth_resp_text", epxResp.AuthRespText),
		zap.String("auth_guid", epxResp.AuthGUID),
	)

	// Save transaction to database
	var transaction *domain.Transaction
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Amount is already in cents - use directly
		amountCents := req.AmountCents

		// Convert customer ID to pgtype.Text if provided
		var customerIDText pgtype.Text
		if req.CustomerID != nil && *req.CustomerID != "" {
			customerIDText = pgtype.Text{String: *req.CustomerID, Valid: true}
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
			CustomerID:          customerIDText,
			OrderID:             converters.ToNullableText(req.OrderID), // Merchant's external order/invoice ID
			AmountCents:         amountCents,
			Currency:            "USD",
			Type:                string(domain.TransactionTypeAuth),
			PaymentMethodType:   string(domain.PaymentMethodTypeCreditCard),
			PaymentMethodID:     converters.ToNullableUUID(req.PaymentMethodID),
			TranNbr:             pgtype.Text{String: epxTranNbr, Valid: true},
			AuthGuid:            converters.ToNullableText(&epxResp.AuthGUID), // Store AUTH's BRIC
			AuthResp:            pgtype.Text{String: epxResp.AuthResp, Valid: true},
			AuthCode:            converters.ToNullableText(&epxResp.AuthCode),
			AuthCardType:        converters.ToNullableText(&epxResp.AuthCardType),
			Metadata:            metadataJSON,
			ParentTransactionID: pgtype.UUID{}, // NULL for first transaction
			ProcessedAt:         pgtype.Timestamptz{},
		}

		dbTx, err := q.CreateTransaction(ctx, params)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "create_transaction")
		}

		if paymentMethodUUID != nil {
			if err := q.MarkPaymentMethodUsed(ctx, *paymentMethodUUID); err != nil {
				s.logger.Warn("Failed to mark payment method as used", zap.Error(err))
			}
		}

		transaction = sqlcTransactionToDomain(&dbTx)
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
	// Validate inputs first (fail-fast) - before any DB or external calls
	if s.serverPost == nil {
		return nil, domain.ErrInternalError.WithDetail("reason", "server_post_adapter_not_initialized")
	}

	if req.TransactionID == "" {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "transaction_id")
	}

	originalTxID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "transaction_id")
	}

	// Parse CAPTURE transaction ID from idempotency key (required)
	if req.IdempotencyKey == nil {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "idempotency_key")
	}

	txID, err := uuid.Parse(*req.IdempotencyKey)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "idempotency_key")
	}

	// Validate amount if provided
	if req.AmountCents != nil {
		if *req.AmountCents <= 0 {
			return nil, domain.ErrValidationAmountInvalid.WithDetail("reason", "amount_must_be_positive")
		}
	}

	// Get original AUTH transaction to validate access
	originalTx, err := s.queries.GetTransactionByID(ctx, originalTxID)
	if err != nil {
		return nil, domain.ErrTxnNotFound
	}

	// Validate transaction access
	domainTx := sqlcTransactionToDomain(&originalTx)
	if err := s.merchantAuthService.ValidateTransactionAccess(ctx, domainTx); err != nil {
		return nil, err
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
			return sqlcTransactionToDomain(existingTx), nil
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
			return domain.ErrDatabaseError.WithDetail("operation", "get_transaction_tree")
		}

		if len(groupTxs) == 0 {
			return domain.ErrTxnNotFound.WithDetail("parent_id", originalTxID.String())
		}

		// Convert to domain transactions
		domainTxs := make([]*domain.Transaction, len(groupTxs))
		for i, tx := range groupTxs {
			// Convert GetTransactionTreeRow to Transaction for sqlcToDomain
			sqlcTx := sqlc.Transaction(tx)
			domainTxs[i] = sqlcTransactionToDomain(&sqlcTx)
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
			return domain.ErrTxnCannotBeCaptured
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
			return domain.ErrMerchantNotFoundTyped
		}

		if !merchant.IsActive {
			return domain.ErrMerchantInactiveTyped
		}

		// Get MAC secret
		_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
		if err != nil {
			return domain.ErrMerchantCredentialFailed
		}

		return nil // Continue outside transaction for EPX call
	})

	if err != nil {
		return nil, err
	}

	// Re-fetch state outside transaction for EPX call
	groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, originalTxID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_transaction_tree")
	}

	domainTxsRefetch := make([]*domain.Transaction, len(groupTxsRefetch))
	for i, tx := range groupTxsRefetch {
		// Convert GetTransactionTreeRow to Transaction for sqlcToDomain
		sqlcTx := sqlc.Transaction(tx)
		domainTxsRefetch[i] = sqlcTransactionToDomain(&sqlcTx)
	}
	state := ComputeGroupState(domainTxsRefetch)

	merchantID := uuid.MustParse(domainTxsRefetch[0].MerchantID)
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, domain.ErrMerchantNotFoundTyped
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
			return nil, domain.ErrDatabaseError.WithDetail("operation", "create_pending_transaction")
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
	epxTranNbr := epxutil.UUIDToEPXTranNbr(txID)

	industryType := "E" // Ecommerce (required for EPX certification)

	epxReq := &ports.ServerPostRequest{
		CustNbr:     merchant.CustNbr,
		MerchNbr:    merchant.MerchNbr,
		DBAnbr:      merchant.DbaNbr,
		TerminalNbr: merchant.TerminalNbr,
		// Use semantic operation - adapter determines EPX transaction type
		Operation:        ports.OperationCapture,
		Amount:           centsToDecimalString(finalCaptureAmountCents),
		PaymentType:      ports.PaymentMethodTypeCreditCard,
		OriginalAuthGUID: authBRIC,   // Reference to AUTH transaction
		TranNbr:          epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:        "",         // No BATCH_ID for capture
		CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
		IndustryType:     &industryType, // "E" for Ecommerce
	}

	// Wrap EPX call with explicit timeout for external service reliability
	epxCtx, epxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer epxCancel()
	epxResp, err := s.serverPost.ProcessTransaction(epxCtx, epxReq)
	if err != nil {
		s.logger.Error("EPX capture failed", zap.Error(err))
		return nil, domain.ErrGatewayError.WithDetail("operation", "process_transaction")
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
		return nil, domain.ErrDatabaseError.WithDetail("operation", "update_transaction")
	}

	// Fetch the updated transaction
	updatedTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_transaction")
	}
	transaction = sqlcTransactionToDomain(&updatedTx)

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
	// Validate inputs first (fail-fast) - before any DB or external calls
	if s.serverPost == nil {
		return nil, domain.ErrInternalError.WithDetail("reason", "server_post_adapter_not_initialized")
	}

	if req.ParentTransactionID == "" {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "parent_transaction_id")
	}

	parentTxID, err := uuid.Parse(req.ParentTransactionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "parent_transaction_id")
	}

	// Parse VOID transaction ID from idempotency key (required)
	if req.IdempotencyKey == nil {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "idempotency_key")
	}

	txID, err := uuid.Parse(*req.IdempotencyKey)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "idempotency_key")
	}

	// Get parent transaction to validate access
	parentTx, err := s.queries.GetTransactionByID(ctx, parentTxID)
	if err != nil {
		return nil, domain.ErrTxnNotFound
	}

	// Get transaction tree (includes parent + all descendants)
	groupTxs, err := s.queries.GetTransactionTree(ctx, parentTxID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_transaction_tree")
	}

	// Validate access using the parent transaction
	firstTx := sqlcTransactionToDomain(&parentTx)
	if err := s.merchantAuthService.ValidateTransactionAccess(ctx, firstTx); err != nil {
		return nil, err
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
			return sqlcTransactionToDomain(existingTx), nil
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
			domainTxs[i] = sqlcTransactionToDomain(&sqlcTx)
		}

		// Compute current state using WAL
		state := ComputeGroupState(domainTxs)

		// Validate void is allowed
		canVoid, reason := state.CanVoid()
		if !canVoid {
			return domain.ErrTxnCannotBeVoided.WithDetail("reason", reason)
		}

		// Get the active AUTH transaction
		if state.ActiveAuthID == nil {
			return domain.ErrTxnCannotBeVoided.WithDetail("reason", "no_active_authorization")
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
			return domain.ErrTxnNotFound.WithDetail("reason", "active_authorization_not_found")
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
			return domain.ErrMerchantNotFoundTyped
		}

		if !merchant.IsActive {
			return domain.ErrMerchantInactiveTyped
		}

		// Get MAC secret
		_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
		if err != nil {
			return domain.ErrMerchantCredentialFailed
		}

		return nil // Continue outside transaction for EPX call
	})

	if err != nil {
		return nil, err
	}

	// Re-fetch state outside transaction for EPX call
	groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, parentTxID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_transaction_tree")
	}

	domainTxsRefetch := make([]*domain.Transaction, len(groupTxsRefetch))
	for i, tx := range groupTxsRefetch {
		sqlcTx := sqlc.Transaction(tx)
		domainTxsRefetch[i] = sqlcTransactionToDomain(&sqlcTx)
	}
	state := ComputeGroupState(domainTxsRefetch)

	merchantID := uuid.MustParse(domainTxsRefetch[0].MerchantID)
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, domain.ErrMerchantNotFoundTyped
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
			return nil, domain.ErrDatabaseError.WithDetail("operation", "create_pending_transaction")
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
	epxTranNbr := epxutil.UUIDToEPXTranNbr(txID)

	industryType := "E" // Ecommerce (required for EPX certification)

	epxReq := &ports.ServerPostRequest{
		CustNbr:     merchant.CustNbr,
		MerchNbr:    merchant.MerchNbr,
		DBAnbr:      merchant.DbaNbr,
		TerminalNbr: merchant.TerminalNbr,
		// Use semantic operation - adapter determines EPX transaction type (CCEX for CC, CKCX for ACH)
		Operation:        ports.OperationVoid,
		Amount:           centsToDecimalString(voidAmountCents),
		PaymentType:      ports.PaymentMethodType(domainTxsRefetch[0].PaymentMethodType),
		OriginalAuthGUID: authBRIC,   // Reference to AUTH transaction
		TranNbr:          epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:        "VOID",     // EPX TRAN_GROUP classification
		CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
		IndustryType:     &industryType, // "E" for Ecommerce
	}

	// Wrap EPX call with explicit timeout for external service reliability
	epxCtx, epxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer epxCancel()
	epxResp, err := s.serverPost.ProcessTransaction(epxCtx, epxReq)
	if err != nil {
		s.logger.Error("EPX void failed", zap.Error(err))
		return nil, domain.ErrGatewayError.WithDetail("operation", "process_transaction")
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
		return nil, domain.ErrDatabaseError.WithDetail("operation", "update_transaction")
	}

	// Fetch the updated transaction
	updatedTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_transaction")
	}
	transaction = sqlcTransactionToDomain(&updatedTx)

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
	// Validate inputs first (fail-fast) - before any DB or external calls
	if s.serverPost == nil {
		return nil, domain.ErrInternalError.WithDetail("reason", "server_post_adapter_not_initialized")
	}

	if req.ParentTransactionID == "" {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "parent_transaction_id")
	}

	parentTxID, err := uuid.Parse(req.ParentTransactionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "parent_transaction_id")
	}

	// Parse REFUND transaction ID from idempotency key (required)
	if req.IdempotencyKey == nil {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "idempotency_key")
	}

	txID, err := uuid.Parse(*req.IdempotencyKey)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "idempotency_key")
	}

	// Validate amount if provided
	var refundAmountCents int64
	if req.AmountCents != nil {
		refundAmountCents = *req.AmountCents
		if refundAmountCents <= 0 {
			return nil, domain.ErrValidationAmountInvalid.WithDetail("reason", "amount_must_be_positive")
		}
	}

	// Get parent transaction to validate access
	parentTx, err := s.queries.GetTransactionByID(ctx, parentTxID)
	if err != nil {
		return nil, domain.ErrTxnNotFound
	}

	// Get transaction tree (includes parent + all descendants)
	groupTxs, err := s.queries.GetTransactionTree(ctx, parentTxID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_transaction_tree")
	}

	// Validate access using the parent transaction
	firstTx := sqlcTransactionToDomain(&parentTx)
	if err := s.merchantAuthService.ValidateTransactionAccess(ctx, firstTx); err != nil {
		return nil, err
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
			return sqlcTransactionToDomain(existingTx), nil
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
			domainTxs[i] = sqlcTransactionToDomain(&sqlcTx)
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
			return domain.ErrTxnCannotBeRefunded
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
			return domain.ErrMerchantNotFoundTyped
		}

		if !merchant.IsActive {
			return domain.ErrMerchantInactiveTyped
		}

		// Get MAC secret
		_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
		if err != nil {
			return domain.ErrMerchantCredentialFailed
		}

		return nil // Continue outside transaction for EPX call
	})

	if err != nil {
		return nil, err
	}

	// Re-fetch state outside transaction for EPX call
	groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, parentTxID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_transaction_tree")
	}

	domainTxsRefetch := make([]*domain.Transaction, len(groupTxsRefetch))
	for i, tx := range groupTxsRefetch {
		sqlcTx := sqlc.Transaction(tx)
		domainTxsRefetch[i] = sqlcTransactionToDomain(&sqlcTx)
	}
	state := ComputeGroupState(domainTxsRefetch)

	merchantID := uuid.MustParse(domainTxsRefetch[0].MerchantID)
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, domain.ErrMerchantNotFoundTyped
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
			return nil, domain.ErrDatabaseError.WithDetail("operation", "create_pending_transaction")
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
	epxTranNbr := epxutil.UUIDToEPXTranNbr(txID)

	industryType := "E" // Ecommerce (required for EPX certification)

	epxReq := &ports.ServerPostRequest{
		CustNbr:     merchant.CustNbr,
		MerchNbr:    merchant.MerchNbr,
		DBAnbr:      merchant.DbaNbr,
		TerminalNbr: merchant.TerminalNbr,
		// Use semantic operation - adapter determines EPX transaction type
		// CRITICAL FIX: Now ACH refunds will use CKC3 (ACH Credit) instead of CCE9 (CC Refund)
		Operation:        ports.OperationRefund,
		Amount:           centsToDecimalString(finalRefundAmountCents),
		PaymentType:      ports.PaymentMethodType(domainTxsRefetch[0].PaymentMethodType),
		OriginalAuthGUID: authBRIC,   // Reference to CAPTURE (or AUTH if SALE)
		TranNbr:          epxTranNbr, // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:        "REFUND",   // EPX TRAN_GROUP classification
		CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
		IndustryType:     &industryType, // "E" for Ecommerce
	}

	// Wrap EPX call with explicit timeout for external service reliability
	epxCtx, epxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer epxCancel()
	epxResp, err := s.serverPost.ProcessTransaction(epxCtx, epxReq)
	if err != nil {
		s.logger.Error("EPX refund failed", zap.Error(err))
		return nil, domain.ErrGatewayError.WithDetail("operation", "process_transaction")
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
		return nil, domain.ErrDatabaseError.WithDetail("operation", "update_transaction")
	}

	// Fetch the updated transaction
	updatedTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_transaction")
	}
	transaction = sqlcTransactionToDomain(&updatedTx)

	s.logger.Info("Refund completed",
		zap.String("transaction_id", transaction.ID),
		zap.String("parent_transaction_id", parentTxID.String()),
		zap.String("amount", formatCentsForLog(finalRefundAmountCents)),
		zap.String("status", string(transaction.Status)),
	)

	return transaction, nil
}

// PaymentTokenInfo contains resolved payment token information
type PaymentTokenInfo struct {
	Token             string                      // BRIC token (auth_guid)
	PaymentMethodID   *uuid.UUID                  // Parsed payment method UUID (if using saved method)
	PaymentMethodType domain.PaymentMethodType    // Type of payment method
	PaymentMethod     *sqlc.CustomerPaymentMethod // Full payment method record (if needed)
}

// resolvePaymentToken resolves payment_method_id or payment_token from request
// Returns the BRIC token and payment method information
func (s *paymentService) resolvePaymentToken(ctx context.Context, paymentMethodID *string, paymentToken *string, validateForAmount *int64) (*PaymentTokenInfo, error) {
	if paymentMethodID != nil {
		// Using saved payment method
		pmID, err := uuid.Parse(*paymentMethodID)
		if err != nil {
			return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "payment_method_id")
		}

		dbPM, err := s.queries.GetPaymentMethodByID(ctx, pmID)
		if err != nil {
			return nil, domain.ErrPMNotFound
		}

		info := &PaymentTokenInfo{
			Token:             dbPM.Bric,
			PaymentMethodID:   &pmID,
			PaymentMethodType: domain.PaymentMethodType(dbPM.PaymentType),
			PaymentMethod:     &dbPM,
		}

		// Optionally validate payment method can be used for amount
		if validateForAmount != nil {
			domainPM := sqlcPaymentMethodToDomain(&dbPM)
			canUse, reason := domainPM.CanUseForAmount(*validateForAmount)
			if !canUse {
				// Map reason strings to domain errors
				switch reason {
				case "payment method is not active":
					return nil, domain.ErrPMInactive
				case "credit card is expired":
					return nil, domain.ErrPMExpired
				case "ACH account must be verified before use":
					return nil, domain.ErrPMNotVerified
				default:
					return nil, domain.ErrPMInactive.WithDetail("reason", reason)
				}
			}
		}

		return info, nil
	}

	if paymentToken != nil {
		// Using one-time token
		return &PaymentTokenInfo{
			Token:             *paymentToken,
			PaymentMethodID:   nil,
			PaymentMethodType: domain.PaymentMethodTypeCreditCard, // Default for one-time tokens
			PaymentMethod:     nil,
		}, nil
	}

	return nil, domain.ErrValidationMissingField.WithDetail("field", "payment_method_id or payment_token")
}

// getTransactionByIdempotencyKey is a private helper for idempotency checking.
// It retrieves a transaction by its ID (idempotency_key IS the transaction ID).
// This is an internal implementation detail, not exposed via ports.
func (s *paymentService) getTransactionByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	txID, err := uuid.Parse(key)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "idempotency_key")
	}

	dbTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err != nil {
		s.logger.Debug("Transaction not found by idempotency key",
			zap.String("idempotency_key", key),
			zap.Error(err),
		)
		return nil, domain.ErrTxnNotFound
	}

	return sqlcTransactionToDomain(&dbTx), nil
}
