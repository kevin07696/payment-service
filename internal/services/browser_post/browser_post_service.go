package browserpost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/internal/util"
	"go.uber.org/zap"
)

// BrowserPostService implements the Browser Post business logic
type BrowserPostService struct {
	queries            sqlc.Querier
	keyExchangeAdapter ports.KeyExchangeAdapter
	browserPostAdapter ports.BrowserPostAdapter
	secretManager      ports.SecretManagerAdapter
	logger             *zap.Logger
	epxPostURL         string
	callbackBaseURL    string
}

// NewBrowserPostService creates a new Browser Post service
func NewBrowserPostService(
	queries sqlc.Querier,
	keyExchangeAdapter ports.KeyExchangeAdapter,
	browserPostAdapter ports.BrowserPostAdapter,
	secretManager ports.SecretManagerAdapter,
	logger *zap.Logger,
	epxPostURL string,
	callbackBaseURL string,
) *BrowserPostService {
	return &BrowserPostService{
		queries:            queries,
		keyExchangeAdapter: keyExchangeAdapter,
		browserPostAdapter: browserPostAdapter,
		secretManager:      secretManager,
		logger:             logger,
		epxPostURL:         epxPostURL,
		callbackBaseURL:    callbackBaseURL,
	}
}

// GenerateFormConfig validates merchant, gets TAC, creates pending transaction, and returns form config
func (s *BrowserPostService) GenerateFormConfig(ctx context.Context, req *ports.GenerateFormConfigRequest) (*ports.GenerateFormConfigResponse, error) {
	transactionID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "transaction_id")
	}

	merchantID, err := uuid.Parse(req.MerchantID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	txType := domain.ParseRequestTransactionType(req.TransactionType)
	if !txType.IsValid() {
		return nil, domain.ErrTxnInvalidType.WithDetail("transaction_type", req.TransactionType)
	}

	// Fetch and validate merchant
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		s.logger.Error("Failed to fetch merchant",
			zap.Error(err),
			zap.String("merchant_id", merchantID.String()),
		)
		return nil, domain.ErrMerchantNotFoundTyped
	}

	if !merchant.IsActive {
		s.logger.Warn("Merchant is not active",
			zap.String("merchant_id", merchantID.String()),
		)
		return nil, domain.ErrMerchantInactiveTyped
	}

	// Fetch merchant-specific MAC from secret manager
	macSecret, err := s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		s.logger.Error("Failed to fetch MAC secret for merchant",
			zap.Error(err),
			zap.String("merchant_id", merchantID.String()),
		)
		return nil, domain.ErrMerchantCredentialFailed
	}

	// Generate deterministic numeric TRAN_NBR from transaction UUID
	epxTranNbr := util.UUIDToEPXTranNbr(transactionID)

	// Build redirect URL with properly encoded query parameters
	redirectURL := s.buildRedirectURL(transactionID.String(), merchantID.String(), string(txType), req.CustomerID)

	// Call EPX Key Exchange to get TAC
	// Key Exchange uses TRAN_GROUP (SALE, AUTH, STORAGE)
	keyExchangeReq := &ports.KeyExchangeRequest{
		MerchantID:   merchantID.String(),
		CustNbr:      merchant.CustNbr,
		MerchNbr:     merchant.MerchNbr,
		DBAnbr:       merchant.DbaNbr,
		TerminalNbr:  merchant.TerminalNbr,
		MAC:          macSecret.Value,
		Amount:       req.Amount,
		TranNbr:      epxTranNbr,
		TranGroup:    txType.ToEPXTranGroup(), // Use TRAN_GROUP for Key Exchange
		RedirectURL:  redirectURL,
		IndustryType: "E",
	}

	keyExchangeResp, err := s.keyExchangeAdapter.GetTAC(ctx, keyExchangeReq)
	if err != nil {
		s.logger.Error("Failed to get TAC from Key Exchange",
			zap.Error(err),
			zap.String("transaction_id", transactionID.String()),
		)
		return nil, domain.ErrGatewayError.WithDetail("operation", "get_tac")
	}

	s.logger.Info("Successfully obtained TAC for Browser Post",
		zap.String("transaction_id", transactionID.String()),
		zap.String("merchant_id", merchantID.String()),
		zap.String("transaction_type", string(txType)),
	)

	// Check if transaction already exists (idempotency)
	existingTx, err := s.queries.GetTransactionByID(ctx, transactionID)
	if err == nil {
		var existingTranNbr string
		if existingTx.TranNbr.Valid {
			existingTranNbr = existingTx.TranNbr.String
		} else {
			existingTranNbr = epxTranNbr
		}

		s.logger.Info("Transaction already exists, returning cached form config",
			zap.String("transaction_id", transactionID.String()),
		)

		return &ports.GenerateFormConfigResponse{
			TransactionID: transactionID.String(),
			EPXTranNbr:    existingTranNbr,
			TAC:           keyExchangeResp.TAC,
			ExpiresAt:     keyExchangeResp.ExpiresAt,
			PostURL:       s.epxPostURL,
			CustNbr:       merchant.CustNbr,
			MerchNbr:      merchant.MerchNbr,
			DBAName:       merchant.DbaNbr,
			TerminalNbr:   merchant.TerminalNbr,
			IndustryType:  "E",
			TranCode:      txType.ToEPXTranCode(), // EPX TRAN_CODE for Browser POST form
			RedirectURL:   redirectURL,
			ReturnURL:     req.ReturnURL,
			MerchantID:    merchant.ID.String(),
			MerchantName:  merchant.Name,
		}, nil
	}

	// Create pending transaction
	amountCents, err := parseAmountToCents(req.Amount)
	if err != nil {
		return nil, domain.ErrValidationAmountInvalid.WithDetail("amount", req.Amount)
	}

	internalTxType := string(txType.ToTransactionType())
	paymentMethodType := string(txType.ToPaymentMethodType())

	_, err = s.queries.CreateTransaction(ctx, sqlc.CreateTransactionParams{
		ID:                  transactionID,
		MerchantID:          merchantID,
		CustomerID:          pgtype.Text{},
		AmountCents:         amountCents,
		Currency:            "USD",
		Type:                internalTxType,
		PaymentMethodType:   paymentMethodType,
		PaymentMethodID:     pgtype.UUID{},
		TranNbr:             pgtype.Text{String: epxTranNbr, Valid: true},
		AuthGuid:            pgtype.Text{},
		AuthResp:            pgtype.Text{},
		AuthCode:            pgtype.Text{},
		AuthCardType:        pgtype.Text{},
		Metadata:            []byte("{}"),
		ParentTransactionID: pgtype.UUID{Valid: false},
		ProcessedAt:         pgtype.Timestamptz{},
	})
	if err != nil {
		s.logger.Error("Failed to create pending transaction",
			zap.Error(err),
			zap.String("transaction_id", transactionID.String()),
		)
		return nil, domain.ErrDatabaseError.WithDetail("operation", "create_transaction")
	}

	s.logger.Info("Created pending transaction for Browser Post",
		zap.String("transaction_id", transactionID.String()),
		zap.String("tran_nbr", epxTranNbr),
		zap.String("merchant_id", merchantID.String()),
	)

	return &ports.GenerateFormConfigResponse{
		TransactionID: transactionID.String(),
		EPXTranNbr:    epxTranNbr,
		TAC:           keyExchangeResp.TAC,
		ExpiresAt:     keyExchangeResp.ExpiresAt,
		PostURL:       s.epxPostURL,
		CustNbr:       merchant.CustNbr,
		MerchNbr:      merchant.MerchNbr,
		DBAName:       merchant.DbaNbr,
		TerminalNbr:   merchant.TerminalNbr,
		IndustryType:  "E",
		TranCode:      txType.ToEPXTranCode(), // EPX TRAN_CODE for Browser POST form
		RedirectURL:   redirectURL,
		ReturnURL:     req.ReturnURL,
		MerchantID:    merchant.ID.String(),
		MerchantName:  merchant.Name,
	}, nil
}

// ParseRedirectResponse delegates to the adapter to parse EPX redirect parameters
func (s *BrowserPostService) ParseRedirectResponse(params map[string][]string) (*ports.BrowserPostResponse, error) {
	return s.browserPostAdapter.ParseRedirectResponse(params)
}

// ProcessCallback updates transaction with EPX response and returns the result
// Payment method storage is orchestrated by the handler
func (s *BrowserPostService) ProcessCallback(ctx context.Context, req *ports.ProcessCallbackRequest) (*ports.ProcessCallbackResponse, error) {
	// Step 1: Validate MAC signature before processing
	if err := s.validateMACSignature(ctx, req); err != nil {
		s.logger.Error("MAC validation failed",
			zap.Error(err),
			zap.String("transaction_id", req.TransactionID),
			zap.String("merchant_id", req.MerchantID),
		)
		return nil, domain.ErrSignatureValidationFailed
	}

	metadata := map[string]interface{}{
		"auth_resp_text": req.AuthRespText,
		"auth_avs":       req.AuthAVS,
		"auth_cvv2":      req.AuthCVV2,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		s.logger.Error("Failed to marshal metadata", zap.Error(err))
		metadataJSON = []byte("{}")
	}

	s.logger.Info("Processing callback",
		zap.String("transaction_id", req.TransactionID),
		zap.String("transaction_type", string(req.TransactionType)),
		zap.String("tran_nbr", req.TranNbr),
	)

	tx, err := s.queries.UpdateTransactionFromEPXResponse(ctx, sqlc.UpdateTransactionFromEPXResponseParams{
		CustomerID: func() pgtype.Text {
			if req.CustomerID != "" {
				return pgtype.Text{String: req.CustomerID, Valid: true}
			}
			return pgtype.Text{}
		}(),
		TranNbr:      pgtype.Text{String: req.TranNbr, Valid: req.TranNbr != ""},
		AuthGuid:     pgtype.Text{String: req.AuthGUID, Valid: req.AuthGUID != ""},
		AuthResp:     pgtype.Text{String: req.AuthResp, Valid: true},
		AuthCode:     pgtype.Text{String: req.AuthCode, Valid: req.AuthCode != ""},
		AuthCardType: pgtype.Text{String: req.AuthCardType, Valid: req.AuthCardType != ""},
		Metadata:     metadataJSON,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			s.logger.Warn("TAC replay attack detected - transaction not in PENDING status",
				zap.String("transaction_id", req.TransactionID),
				zap.String("tran_nbr", req.TranNbr),
			)
			return nil, domain.ErrTxnAlreadyProcessed
		}
		s.logger.Error("Failed to update transaction from EPX response",
			zap.Error(err),
			zap.String("transaction_id", req.TransactionID),
		)
		return nil, domain.ErrDatabaseError.WithDetail("operation", "update_transaction")
	}

	s.logger.Info("Successfully processed transaction",
		zap.String("transaction_id", tx.ID.String()),
		zap.String("status", tx.Status.String),
	)

	response := &ports.ProcessCallbackResponse{
		TransactionID: tx.ID.String(),
		Status:        tx.Status.String,
		ReturnURL:     req.RawParams["USER_DATA_1"],
	}

	if tx.ParentTransactionID.Valid {
		response.ParentTransactionID = uuid.UUID(tx.ParentTransactionID.Bytes).String()
	}

	return response, nil
}

func parseAmountToCents(amount string) (int64, error) {
	var amountFloat float64
	if _, err := fmt.Sscanf(amount, "%f", &amountFloat); err != nil {
		return 0, err
	}
	return int64(amountFloat * 100), nil
}

// buildRedirectURL constructs the callback URL with properly encoded query parameters.
// This prevents injection issues and ensures special characters are handled correctly.
func (s *BrowserPostService) buildRedirectURL(transactionID, merchantID, txType, customerID string) string {
	params := url.Values{}
	params.Set("transaction_id", transactionID)
	params.Set("merchant_id", merchantID)
	params.Set("transaction_type", txType)
	if customerID != "" {
		params.Set("customer_id", customerID)
	}
	return fmt.Sprintf("%s/api/v1/payments/browser-post/callback?%s", s.callbackBaseURL, params.Encode())
}

// validateMACSignature validates the MAC signature in the EPX callback response.
// This ensures the callback is genuinely from EPX and data hasn't been tampered with.
func (s *BrowserPostService) validateMACSignature(ctx context.Context, req *ports.ProcessCallbackRequest) error {
	if req.MerchantID == "" {
		return domain.ErrMerchantRequired
	}

	merchantID, err := uuid.Parse(req.MerchantID)
	if err != nil {
		return domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	// Fetch merchant to get MAC secret path
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		s.logger.Error("Failed to fetch merchant for MAC validation",
			zap.Error(err),
			zap.String("merchant_id", req.MerchantID),
		)
		return domain.ErrMerchantNotFoundTyped
	}

	// Fetch MAC secret from secret manager
	macSecret, err := s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		s.logger.Error("Failed to fetch MAC secret",
			zap.Error(err),
			zap.String("merchant_id", req.MerchantID),
		)
		return domain.ErrMerchantCredentialFailed
	}

	// Convert RawParams map[string]string to map[string][]string for adapter
	params := make(map[string][]string)
	for key, value := range req.RawParams {
		params[key] = []string{value}
	}

	// Validate MAC using the adapter
	return s.browserPostAdapter.ValidateResponseMAC(params, macSecret.Value)
}
