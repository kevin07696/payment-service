package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	serviceports "github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/kevin07696/payment-service/internal/util"
	"go.uber.org/zap"
)

// DatabaseAdapter defines the interface for database operations
type DatabaseAdapter interface {
	Queries() sqlc.Querier
}

// PaymentMethodService defines the interface for payment method operations
type PaymentMethodService interface {
	ConvertFinancialBRICToStorageBRIC(ctx context.Context, req *serviceports.ConvertFinancialBRICRequest) (*domain.PaymentMethod, error)
}

// mapEPXTranGroupToType maps EPX TRAN_GROUP values to our transaction type
// EPX can return either single-letter codes (A, U) or full words (AUTH, SALE)
// Mapping:
//   - "A" or "AUTH" → "auth" (authorization only, requires capture)
//   - "U" or "SALE" → "sale" (combined auth+capture in single step)
//   - Default → "sale" (EPX default is SALE)
func mapEPXTranGroupToType(tranGroup string) string {
	switch strings.ToUpper(tranGroup) {
	case "A", "AUTH":
		return "auth"
	case "U", "SALE":
		return "sale"
	default:
		// Default to sale (EPX's default transaction type)
		return "sale"
	}
}

// mapRequestTypeToTransactionType maps requested transaction type to our internal type
// Mapping:
//   - "AUTH" → "auth" (authorization only, requires capture)
//   - "SALE" → "sale" (combined auth+capture in single step)
//   - Default → "sale"
func mapRequestTypeToTransactionType(requestType string) string {
	switch strings.ToUpper(requestType) {
	case "AUTH":
		return "auth"
	case "SALE":
		return "sale"
	default:
		return "sale"
	}
}

// BrowserPostCallbackHandler handles the redirect callback from EPX Browser Post API
// This endpoint receives the transaction results after EPX processes the payment
// Multi-tenant: fetches merchant-specific credentials from database per request
type BrowserPostCallbackHandler struct {
	dbAdapter          DatabaseAdapter
	browserPost        ports.BrowserPostAdapter
	keyExchangeAdapter ports.KeyExchangeAdapter
	secretManager      ports.SecretManagerAdapter
	paymentMethodSvc   PaymentMethodService
	logger             *zap.Logger
	epxPostURL         string // EPX Browser Post endpoint URL (e.g., "https://secure.epxuap.com/browserpost")
	callbackBaseURL    string // Base URL for callback (e.g., "http://localhost:8081")
}

// NewBrowserPostCallbackHandler creates a new Browser Post callback handler
func NewBrowserPostCallbackHandler(
	dbAdapter DatabaseAdapter,
	browserPost ports.BrowserPostAdapter,
	keyExchangeAdapter ports.KeyExchangeAdapter,
	secretManager ports.SecretManagerAdapter,
	paymentMethodSvc PaymentMethodService,
	logger *zap.Logger,
	epxPostURL string,
	callbackBaseURL string,
) *BrowserPostCallbackHandler {
	return &BrowserPostCallbackHandler{
		dbAdapter:          dbAdapter,
		browserPost:        browserPost,
		keyExchangeAdapter: keyExchangeAdapter,
		secretManager:      secretManager,
		paymentMethodSvc:   paymentMethodSvc,
		logger:             logger,
		epxPostURL:         epxPostURL,
		callbackBaseURL:    callbackBaseURL,
	}
}

// GetPaymentForm generates form configuration for Browser Post payment
// Fetches merchant credentials, calls Key Exchange for TAC, and returns form config
// NO database write - transaction only created on callback
// Endpoint: GET /api/v1/payments/browser-post/form?transaction_id={uuid}&merchant_id={id}&amount={amount}&transaction_type={type}&return_url={url}
// transaction_type: "SALE" (auth+capture) or "AUTH" (auth-only, capture later via Server Post)
func (h *BrowserPostCallbackHandler) GetPaymentForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.logger.Warn("Browser Post form generator received non-GET request",
			zap.String("method", r.Method),
		)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract frontend-generated transaction ID
	transactionIDStr := r.URL.Query().Get("transaction_id")
	if transactionIDStr == "" {
		h.logger.Warn("Browser Post form request missing transaction_id parameter")
		http.Error(w, "transaction_id parameter is required", http.StatusBadRequest)
		return
	}

	transactionID, err := uuid.Parse(transactionIDStr)
	if err != nil {
		h.logger.Warn("Invalid transaction_id format",
			zap.String("transaction_id", transactionIDStr),
			zap.Error(err),
		)
		http.Error(w, "invalid transaction_id format", http.StatusBadRequest)
		return
	}

	// Extract merchant ID
	merchantIDStr := r.URL.Query().Get("merchant_id")
	if merchantIDStr == "" {
		h.logger.Warn("Browser Post form request missing merchant_id parameter")
		http.Error(w, "merchant_id parameter is required", http.StatusBadRequest)
		return
	}

	merchantID, err := uuid.Parse(merchantIDStr)
	if err != nil {
		h.logger.Warn("Invalid merchant_id format",
			zap.String("merchant_id", merchantIDStr),
			zap.Error(err),
		)
		http.Error(w, "invalid merchant_id format", http.StatusBadRequest)
		return
	}

	// Extract amount
	amountStr := r.URL.Query().Get("amount")
	if amountStr == "" {
		h.logger.Warn("Browser Post form request missing amount parameter")
		http.Error(w, "amount parameter is required", http.StatusBadRequest)
		return
	}

	// Validate amount format
	if _, err := fmt.Sscanf(amountStr, "%f", new(float64)); err != nil {
		h.logger.Warn("Invalid amount format",
			zap.String("amount", amountStr),
			zap.Error(err),
		)
		http.Error(w, "amount must be a valid number", http.StatusBadRequest)
		return
	}

	// Extract transaction type (SALE or AUTH)
	transactionType := r.URL.Query().Get("transaction_type")
	if transactionType == "" {
		transactionType = "SALE" // Default to SALE (auth+capture)
	}

	// Validate transaction type
	if transactionType != "SALE" && transactionType != "AUTH" {
		h.logger.Warn("Invalid transaction_type",
			zap.String("transaction_type", transactionType),
		)
		http.Error(w, "transaction_type must be SALE or AUTH", http.StatusBadRequest)
		return
	}

	// Extract return URL
	returnURL := r.URL.Query().Get("return_url")
	if returnURL == "" {
		h.logger.Warn("Browser Post form request missing return_url parameter")
		http.Error(w, "return_url parameter is required", http.StatusBadRequest)
		return
	}

	// Parse return URL to extract base URL (scheme + host)
	// For example: https://abc123.ngrok.io/api/v1/payments/browser-post/callback -> https://abc123.ngrok.io
	parsedURL, err := url.Parse(returnURL)
	if err != nil {
		h.logger.Warn("Invalid return_url format",
			zap.String("return_url", returnURL),
			zap.Error(err),
		)
		http.Error(w, "invalid return_url format", http.StatusBadRequest)
		return
	}
	callbackBaseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// Fetch merchant credentials from database
	merchant, err := h.dbAdapter.Queries().GetMerchantByID(r.Context(), merchantID)
	if err != nil {
		h.logger.Error("Failed to fetch merchant credentials",
			zap.Error(err),
			zap.String("merchant_id", merchantID.String()),
		)
		http.Error(w, "merchant not found", http.StatusNotFound)
		return
	}

	if !merchant.IsActive {
		h.logger.Warn("Merchant is not active",
			zap.String("merchant_id", merchantID.String()),
		)
		http.Error(w, "merchant is not active", http.StatusForbidden)
		return
	}

	// Fetch merchant-specific MAC from secret manager
	macSecret, err := h.secretManager.GetSecret(r.Context(), merchant.MacSecretPath)
	if err != nil {
		h.logger.Error("Failed to fetch MAC secret for merchant",
			zap.Error(err),
			zap.String("merchant_id", merchantID.String()),
			zap.String("mac_secret_path", merchant.MacSecretPath),
		)
		http.Error(w, "failed to retrieve merchant credentials", http.StatusInternalServerError)
		return
	}

	// Generate deterministic numeric TRAN_NBR from transaction UUID
	// This ensures idempotency - same UUID always produces same TRAN_NBR
	// Do this before GetTAC since we need tran_nbr for Key Exchange
	epxTranNbr := util.UUIDToEPXTranNbr(transactionID)

	// Build redirect URL with transaction_id and transaction_type as query parameters
	// EPX will redirect to this URL with all query parameters preserved
	// Use the callbackBaseURL extracted from return_url parameter (supports ngrok, staging, etc.)
	redirectURL := fmt.Sprintf("%s/api/v1/payments/browser-post/callback?transaction_id=%s&merchant_id=%s&transaction_type=%s",
		callbackBaseURL, transactionID.String(), merchantID.String(), transactionType)

	// Call EPX Key Exchange to get TAC (do this before idempotency check - we need fresh TAC regardless)
	keyExchangeReq := &ports.KeyExchangeRequest{
		MerchantID:  merchantID.String(),
		CustNbr:     merchant.CustNbr,
		MerchNbr:    merchant.MerchNbr,
		DBAnbr:      merchant.DbaNbr,
		TerminalNbr: merchant.TerminalNbr,
		MAC:         macSecret.Value, // Merchant-specific MAC from secret manager
		Amount:      amountStr,
		TranNbr:     epxTranNbr,      // EPX numeric TRAN_NBR (max 10 digits)
		TranGroup:   transactionType, // SALE or AUTH
		RedirectURL: redirectURL,     // Include transaction_id in redirect URL
	}

	keyExchangeResp, err := h.keyExchangeAdapter.GetTAC(r.Context(), keyExchangeReq)
	if err != nil {
		h.logger.Error("Failed to get TAC from Key Exchange",
			zap.Error(err),
			zap.String("transaction_id", transactionID.String()),
			zap.String("merchant_id", merchantID.String()),
		)
		http.Error(w, "failed to get TAC from payment gateway", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Successfully obtained TAC for Browser Post",
		zap.String("transaction_id", transactionID.String()),
		zap.String("merchant_id", merchantID.String()),
		zap.String("transaction_type", transactionType),
		zap.Time("tac_expires_at", keyExchangeResp.ExpiresAt),
	)

	// Check if transaction already exists (idempotency check)
	// If client retries with same transaction_id, return existing TRAN_NBR
	existingTx, err := h.dbAdapter.Queries().GetTransactionByID(r.Context(), transactionID)
	if err == nil {
		// Transaction exists - extract tran_nbr and return cached response
		var epxTranNbr string
		if existingTx.TranNbr.Valid {
			epxTranNbr = existingTx.TranNbr.String
		} else {
			// Fallback: generate tran_nbr if not set (shouldn't happen)
			epxTranNbr = util.UUIDToEPXTranNbr(transactionID)
		}

		h.logger.Info("Transaction already exists, returning cached form config",
			zap.String("transaction_id", transactionID.String()),
			zap.String("tran_nbr", epxTranNbr),
		)

		// Return form config for existing transaction (idempotent response)
		formConfig := map[string]interface{}{
			"transactionId": transactionID.String(),
			"epxTranNbr":    epxTranNbr,
			"tac":           keyExchangeResp.TAC, // Still need fresh TAC for form submission
			"expiresAt":     keyExchangeResp.ExpiresAt.Unix(),
			"postURL":       h.epxPostURL,
			"custNbr":       merchant.CustNbr,
			"merchNbr":      merchant.MerchNbr,
			"dbaName":       merchant.DbaNbr,
			"terminalNbr":   merchant.TerminalNbr,
			"industryType":  "E",
			"tranType":      "S",
			"redirectURL":   fmt.Sprintf("%s/api/v1/payments/browser-post/callback?transaction_id=%s&merchant_id=%s&transaction_type=%s", callbackBaseURL, transactionID.String(), merchantID.String(), transactionType),
			"returnUrl":     returnURL,
			"merchantId":    merchant.ID.String(),
			"merchantName":  merchant.Name,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(formConfig); err != nil {
			h.logger.Error("Failed to encode form configuration", zap.Error(err))
		}
		return
	}

	// Transaction doesn't exist - create pending transaction
	// Parse amount for pending transaction (epxTranNbr already generated above)
	var amountFloat float64
	if _, err := fmt.Sscanf(amountStr, "%f", &amountFloat); err != nil {
		h.logger.Error("Failed to parse amount for pending transaction",
			zap.String("amount", amountStr),
			zap.Error(err),
		)
		http.Error(w, "invalid amount format", http.StatusBadRequest)
		return
	}

	// Create pending transaction record
	// This establishes the transaction UUID and TRAN_NBR for idempotency
	// Status will be empty initially (updated after EPX response in callback)
	var amountNumeric pgtype.Numeric
	if err := amountNumeric.Scan(amountStr); err != nil {
		h.logger.Error("Failed to convert amount to numeric",
			zap.String("amount", amountStr),
			zap.Error(err),
		)
		http.Error(w, "invalid amount format", http.StatusBadRequest)
		return
	}

	// Determine transaction type for pending transaction
	internalTxType := mapRequestTypeToTransactionType(transactionType)

	// Create pending transaction with empty auth_resp (will be updated in callback)
	_, err = h.dbAdapter.Queries().CreateTransaction(r.Context(), sqlc.CreateTransactionParams{
		ID:                transactionID,
		MerchantID:        merchantID,
		CustomerID:        pgtype.Text{}, // Unknown until callback (from USER_DATA_2)
		Amount:            amountNumeric,
		Currency:          "USD",
		Type:              internalTxType,
		PaymentMethodType: "credit_card", // Browser Post is credit card
		PaymentMethodID:   pgtype.UUID{}, // Unknown until callback
		TranNbr: pgtype.Text{
			String: epxTranNbr,
			Valid:  true,
		},
		AuthGuid: pgtype.Text{}, // Will be set in callback
		AuthResp: "",            // Empty initially (callback will update)
		AuthCode: pgtype.Text{},
		AuthCardType: pgtype.Text{},
		Metadata:     []byte("{}"),
		GroupID:      nil, // DB auto-generates group_id
	})

	if err != nil {
		h.logger.Error("Failed to create pending transaction",
			zap.Error(err),
			zap.String("transaction_id", transactionID.String()),
		)
		http.Error(w, "failed to create transaction", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Created pending transaction for Browser Post",
		zap.String("transaction_id", transactionID.String()),
		zap.String("tran_nbr", epxTranNbr),
		zap.String("merchant_id", merchantID.String()),
	)

	// Build form configuration response with TAC and credentials (TAC already obtained above)
	// Frontend will map these clear field names to EPX's required field names when submitting to EPX:
	//   - returnUrl → USER_DATA_1 (where to redirect user after payment)
	//   - customer_id (if provided by frontend) → USER_DATA_2 (triggers payment method save)
	//   - merchantId → USER_DATA_3 (merchant UUID for callback validation)
	formConfig := map[string]interface{}{
		// Frontend UUID echoed back
		"transactionId": transactionID.String(),

		// EPX numeric TRAN_NBR (max 10 digits)
		"epxTranNbr": epxTranNbr,

		// TAC from Key Exchange (expires in 15 minutes)
		"tac":       keyExchangeResp.TAC,
		"expiresAt": keyExchangeResp.ExpiresAt.Unix(),

		// EPX Browser Post endpoint URL
		"postURL": h.epxPostURL,

		// Merchant credentials (for form hidden fields)
		"custNbr":     merchant.CustNbr,
		"merchNbr":    merchant.MerchNbr,
		"dbaName":     merchant.DbaNbr,
		"terminalNbr": merchant.TerminalNbr,

		// Static config
		"industryType": "E", // E-commerce
		"tranType":     "S", // Sale (auth + capture)

		// Pass-through data (EPX will echo these back in callback via USER_DATA_* fields)
		// Note: Frontend should map returnUrl → USER_DATA_1, merchantId → USER_DATA_3
		// If user wants to save payment method, frontend should include customer_id in USER_DATA_2
		"redirectURL": redirectURL,          // Full callback URL with query params (MUST match Key Exchange)
		"returnUrl":   returnURL,            // Where to redirect user after payment (maps to USER_DATA_1)
		"merchantId":  merchant.ID.String(), // Merchant UUID for callback validation (maps to USER_DATA_3)

		// Merchant display name (for UI)
		"merchantName": merchant.Name,
	}

	// Return JSON response (NO database write)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(formConfig); err != nil {
		h.logger.Error("Failed to encode form configuration",
			zap.Error(err),
		)
	}
}

// HandleCallback processes the Browser Post redirect callback from EPX
// According to EPX docs (page 7-8): EPX redirects browser with transaction results as self-posting form
// Endpoint: POST /api/v1/payments/browser-post/callback
func (h *BrowserPostCallbackHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Warn("Browser Post callback received non-POST request",
			zap.String("method", r.Method),
		)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.logger.Error("Failed to parse form data",
			zap.Error(err),
		)
		h.renderErrorPage(w, "Invalid request data", "")
		return
	}

	// Log all received form parameters for debugging
	formData := make(map[string]interface{})
	for key, values := range r.Form {
		if len(values) == 1 {
			formData[key] = values[0]
		} else {
			formData[key] = values
		}
	}

	h.logger.Info("Received Browser Post callback",
		zap.Int("form_values", len(r.Form)),
		zap.Any("all_form_data", formData),
	)

	// Convert r.Form (url.Values) to map[string][]string for ParseRedirectResponse
	params := make(map[string][]string)
	for key, values := range r.Form {
		params[key] = values
	}

	// Parse the response using BrowserPostAdapter
	response, err := h.browserPost.ParseRedirectResponse(params)
	if err != nil {
		h.logger.Error("Failed to parse Browser Post response",
			zap.Error(err),
		)
		h.renderErrorPage(w, "Failed to process payment response", err.Error())
		return
	}

	// Extract transaction_id from form data
	// EPX takes REDIRECT_URL query parameters and merges them into form POST data
	// So transaction_id=... from REDIRECT_URL becomes available as "transaction_id" in form data
	transactionIDStr := response.RawParams["transaction_id"]
	if transactionIDStr == "" {
		h.logger.Error("Missing transaction_id in form data",
			zap.String("url", r.URL.String()),
		)
		h.renderErrorPage(w, "Missing transaction reference", "")
		return
	}

	transactionID, err := uuid.Parse(transactionIDStr)
	if err != nil {
		h.logger.Error("Invalid transaction_id format",
			zap.Error(err),
			zap.String("transaction_id", transactionIDStr),
		)
		h.renderErrorPage(w, "Invalid transaction reference", "")
		return
	}

	// Extract merchant_id from form data (from REDIRECT_URL query parameter)
	merchantIDStr := response.RawParams["merchant_id"]
	merchantID, err := uuid.Parse(merchantIDStr)
	if err != nil {
		h.logger.Error("Invalid merchant ID in callback",
			zap.Error(err),
			zap.String("merchant_id", merchantIDStr),
		)
		h.renderErrorPage(w, "Invalid merchant ID", "")
		return
	}

	// Extract transaction_type from form data (from REDIRECT_URL query parameter)
	transactionType := response.RawParams["transaction_type"]
	if transactionType == "" {
		transactionType = "SALE" // Default to SALE if not specified
	}

	// Parse amount
	var amountNumeric pgtype.Numeric
	if err := amountNumeric.Scan(response.Amount); err != nil {
		h.logger.Error("Failed to parse amount",
			zap.String("amount", response.Amount),
			zap.Error(err),
		)
		h.renderErrorPage(w, "Invalid amount", "")
		return
	}

	// Build metadata with display-only fields
	metadata := map[string]interface{}{
		"auth_resp_text": response.AuthRespText,
		"auth_avs":       response.AuthAVS,
		"auth_cvv2":      response.AuthCVV2,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		h.logger.Error("Failed to marshal metadata",
			zap.Error(err),
			zap.String("transaction_id", transactionID.String()),
		)
		metadataJSON = []byte("{}")
	}

	// Map transaction_type to internal transaction type
	// We already extracted transaction_type from REDIRECT_URL query parameters
	internalTransactionType := mapRequestTypeToTransactionType(transactionType)
	h.logger.Info("Transaction type from REDIRECT_URL",
		zap.String("request_type", transactionType),
		zap.String("internal_type", internalTransactionType),
		zap.String("transaction_id", transactionID.String()),
	)

	// Update transaction with EPX response data
	// Transaction was created as pending in GetPaymentForm, now update with EPX results
	// Uses tran_nbr from EPX response to find the transaction record
	tx, err := h.dbAdapter.Queries().UpdateTransactionFromEPXResponse(r.Context(), sqlc.UpdateTransactionFromEPXResponseParams{
		TranNbr: pgtype.Text{
			String: response.TranNbr,
			Valid:  response.TranNbr != "",
		},
		AuthGuid: pgtype.Text{
			String: response.AuthGUID,
			Valid:  response.AuthGUID != "",
		},
		AuthResp: response.AuthResp, // Required - updates status GENERATED column
		AuthCode: pgtype.Text{
			String: response.AuthCode,
			Valid:  response.AuthCode != "",
		},
		AuthCardType: pgtype.Text{
			String: response.AuthCardType,
			Valid:  response.AuthCardType != "",
		},
		Metadata: metadataJSON,
	})

	if err != nil {
		h.logger.Error("Failed to update transaction from EPX response",
			zap.Error(err),
			zap.String("transaction_id", transactionID.String()),
			zap.String("tran_nbr", response.TranNbr),
			zap.String("merchant_id", merchantID.String()),
		)
		h.renderErrorPage(w, "Failed to process transaction", "")
		return
	}

	h.logger.Info("Successfully processed transaction from Browser Post callback",
		zap.String("transaction_id", tx.ID.String()),
		zap.String("group_id", tx.GroupID.String()),
		zap.String("merchant_id", merchantID.String()),
		zap.String("status", tx.Status.String),         // Generated from auth_resp
		zap.String("auth_resp", response.AuthResp),
		zap.String("auth_guid", response.AuthGUID),
	)

	// Check if user wants to save payment method (from USER_DATA fields)
	// If yes and transaction approved, convert Financial BRIC to Storage BRIC
	if response.IsApproved && h.shouldSavePaymentMethod(response.RawParams) {
		if err := h.savePaymentMethod(r.Context(), merchantID.String(), response, tx.ID.String()); err != nil {
			h.logger.Error("Failed to save payment method",
				zap.Error(err),
				zap.String("transaction_id", tx.ID.String()),
			)
			// Don't fail the transaction - user can save it later
		}
	}

	// Extract return_url from USER_DATA_1 (state parameter pattern)
	returnURL := h.extractReturnURL(response.RawParams)

	if returnURL != "" {
		// Redirect to calling service (POS/e-commerce/etc.) with transaction data
		h.redirectToService(w, returnURL, tx.ID.String(), tx.GroupID.String(), tx.Status.String, response)
	} else {
		// Fallback: render simple receipt if no return_url provided
		h.renderReceiptPage(w, response, tx.ID.String(), tx.GroupID.String())
	}
}

// extractReturnURL extracts the return_url from EPX USER_DATA_1 field
// USER_DATA_1 contains just the return URL (transaction_id comes from TRAN_NBR)
func (h *BrowserPostCallbackHandler) extractReturnURL(rawParams map[string]string) string {
	if userData1, ok := rawParams["USER_DATA_1"]; ok {
		return userData1
	}
	return ""
}

// redirectToService redirects browser to calling service (POS/e-commerce/etc.) with transaction data
func (h *BrowserPostCallbackHandler) redirectToService(w http.ResponseWriter, returnURL, txID, groupID, status string, response *ports.BrowserPostResponse) {
	// Build redirect URL with transaction data as query parameters
	redirectURL := fmt.Sprintf("%s?groupId=%s&transactionId=%s&status=%s&amount=%s&cardType=%s&authCode=%s",
		returnURL,
		groupID,
		txID,
		status,
		response.Amount,
		response.AuthCardType,
		response.AuthCode,
	)

	h.logger.Info("Redirecting to calling service",
		zap.String("return_url", returnURL),
		zap.String("group_id", groupID),
		zap.String("status", status),
	)

	// Render HTML that auto-redirects to POS/service
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta http-equiv="refresh" content="0; url=%s">
	<title>Payment %s</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			display: flex;
			align-items: center;
			justify-content: center;
			height: 100vh;
			margin: 0;
			background: #f5f5f5;
		}
		.container {
			text-align: center;
			padding: 2rem;
			background: white;
			border-radius: 8px;
			box-shadow: 0 2px 8px rgba(0,0,0,0.1);
		}
		.spinner {
			border: 4px solid #f3f3f3;
			border-top: 4px solid #3498db;
			border-radius: 50%%;
			width: 40px;
			height: 40px;
			animation: spin 1s linear infinite;
			margin: 0 auto 1rem;
		}
		@keyframes spin {
			0%% { transform: rotate(0deg); }
			100%% { transform: rotate(360deg); }
		}
	</style>
</head>
<body>
	<div class="container">
		<div class="spinner"></div>
		<h2>Payment %s</h2>
		<p>Returning to application...</p>
		<p><a href="%s">Click here if not redirected automatically</a></p>
	</div>
	<script>
		setTimeout(function() {
			window.location.href = "%s";
		}, 100);
	</script>
</body>
</html>`, redirectURL, status, status, redirectURL, redirectURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// renderReceiptPage renders an HTML receipt page to the user
func (h *BrowserPostCallbackHandler) renderReceiptPage(w http.ResponseWriter, response *ports.BrowserPostResponse, txID string, groupID string) {
	approved := response.IsApproved

	// Mask card number (show last 4 digits) - get from raw params if available
	maskedCard := ""
	if cardNbr, ok := response.RawParams["CARD_NBR"]; ok && len(cardNbr) >= 4 {
		maskedCard = "****-****-****-" + cardNbr[len(cardNbr)-4:]
	}

	// Get invoice number from raw params if available
	invoiceNbr := ""
	if inv, ok := response.RawParams["INVOICE_NBR"]; ok {
		invoiceNbr = inv
	}

	tmpl := template.Must(template.New("receipt").Parse(receiptTemplate))

	data := map[string]interface{}{
		"Approved":      approved,
		"Amount":        response.Amount,
		"Currency":      "USD",
		"CardType":      getCardTypeName(response.AuthCardType),
		"MaskedCard":    maskedCard,
		"AuthCode":      response.AuthCode,
		"AuthRespText":  response.AuthRespText,
		"TransactionID": txID,
		"GroupID":       groupID,
		"TranNbr":       response.TranNbr,
		"BRIC":          response.AuthGUID,
		"InvoiceNbr":    invoiceNbr,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := tmpl.Execute(w, data); err != nil {
		h.logger.Error("Failed to render receipt template",
			zap.Error(err),
		)
	}
}

// renderErrorPage renders an HTML error page
func (h *BrowserPostCallbackHandler) renderErrorPage(w http.ResponseWriter, message, details string) {
	tmpl := template.Must(template.New("error").Parse(errorTemplate))

	data := map[string]interface{}{
		"Message": message,
		"Details": details,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK) // Still 200 because this is a redirect from EPX

	if err := tmpl.Execute(w, data); err != nil {
		h.logger.Error("Failed to render error template",
			zap.Error(err),
		)
	}
}

// shouldSavePaymentMethod checks if the user requested to save their payment method
// This is indicated by USER_DATA_2 field containing customer_id (if present, save payment method)
func (h *BrowserPostCallbackHandler) shouldSavePaymentMethod(rawParams map[string]string) bool {
	// Check USER_DATA_2 for customer_id - if present, save payment method
	if customerID, ok := rawParams["USER_DATA_2"]; ok && customerID != "" {
		return true
	}
	return false
}

// savePaymentMethod converts the Financial BRIC to a Storage BRIC and saves it
func (h *BrowserPostCallbackHandler) savePaymentMethod(ctx context.Context, merchantID string, response *ports.BrowserPostResponse, txID string) error {
	// Extract customer_id from USER_DATA_2
	customerID, ok := response.RawParams["USER_DATA_2"]
	if !ok || customerID == "" {
		return fmt.Errorf("customer_id not provided in USER_DATA_2")
	}

	// Determine payment type
	paymentType := "credit_card" // Browser Post is typically credit card
	// Note: If we support ACH through Browser Post, we'd need to detect it here

	// Extract last four digits from card number
	lastFour := ""
	if cardNbr, ok := response.RawParams["CARD_NBR"]; ok && len(cardNbr) >= 4 {
		lastFour = cardNbr[len(cardNbr)-4:]
	}

	if lastFour == "" {
		return fmt.Errorf("unable to extract last four digits from card number")
	}

	// Extract card expiration (YYMM format)
	var cardExpMonth, cardExpYear *int
	if expDate, ok := response.RawParams["EXP_DATE"]; ok && len(expDate) == 4 {
		// Parse YYMM format
		year := expDate[0:2]
		month := expDate[2:4]

		var yy, mm int
		fmt.Sscanf(year, "%d", &yy)
		fmt.Sscanf(month, "%d", &mm)

		// Convert YY to full year (20YY)
		fullYear := 2000 + yy
		cardExpYear = &fullYear
		cardExpMonth = &mm
	}

	// Extract card brand from AUTH_CARD_TYPE
	var cardBrand *string
	if response.AuthCardType != "" {
		brand := getCardTypeName(response.AuthCardType)
		cardBrand = &brand
	}

	// Extract billing information
	firstName := getStringPtr(response.RawParams, "FIRST_NAME")
	lastName := getStringPtr(response.RawParams, "LAST_NAME")
	address := getStringPtr(response.RawParams, "ADDRESS")
	city := getStringPtr(response.RawParams, "CITY")
	state := getStringPtr(response.RawParams, "STATE")
	zipCode := getStringPtr(response.RawParams, "ZIP_CODE")

	// Build ConvertFinancialBRICRequest using merchant_id (UUID from merchants table)
	req := &serviceports.ConvertFinancialBRICRequest{
		MerchantID:    merchantID,
		CustomerID:    customerID,
		FinancialBRIC: response.AuthGUID,
		PaymentType:   domain.PaymentMethodType(paymentType),
		TransactionID: txID,
		LastFour:      lastFour,
		CardBrand:     cardBrand,
		CardExpMonth:  cardExpMonth,
		CardExpYear:   cardExpYear,
		IsDefault:     false, // Don't auto-set as default
		FirstName:     firstName,
		LastName:      lastName,
		Address:       address,
		City:          city,
		State:         state,
		ZipCode:       zipCode,
	}

	// Call payment method service to convert Financial BRIC to Storage BRIC
	_, err := h.paymentMethodSvc.ConvertFinancialBRICToStorageBRIC(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to convert Financial BRIC to Storage BRIC: %w", err)
	}

	h.logger.Info("Payment method saved successfully",
		zap.String("customer_id", customerID),
		zap.String("merchant_id", merchantID),
		zap.String("last_four", lastFour),
	)

	return nil
}

// getStringPtr returns a pointer to the string value if it exists in the map
func getStringPtr(m map[string]string, key string) *string {
	if val, ok := m[key]; ok && val != "" {
		return &val
	}
	return nil
}

// getCardTypeName converts EPX card type code to human-readable name
func getCardTypeName(code string) string {
	cardTypes := map[string]string{
		"V": "Visa",
		"M": "Mastercard",
		"A": "American Express",
		"D": "Discover",
		"J": "JCB",
	}

	if name, ok := cardTypes[strings.ToUpper(code)]; ok {
		return name
	}
	return code
}

// HTML template for receipt page
const receiptTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Payment {{if .Approved}}Successful{{else}}Failed{{end}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .receipt {
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .status {
            font-size: 24px;
            font-weight: bold;
            margin: 20px 0;
        }
        .status.success {
            color: #10b981;
        }
        .status.failed {
            color: #ef4444;
        }
        .icon {
            font-size: 64px;
            margin-bottom: 20px;
        }
        .details {
            margin: 30px 0;
            border-top: 2px solid #e5e7eb;
            padding-top: 20px;
        }
        .detail-row {
            display: flex;
            justify-content: space-between;
            margin: 15px 0;
            padding: 10px 0;
        }
        .detail-label {
            font-weight: 600;
            color: #6b7280;
        }
        .detail-value {
            color: #111827;
            font-weight: 500;
        }
        .amount {
            font-size: 32px;
            font-weight: bold;
            text-align: center;
            margin: 20px 0;
            color: #111827;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e5e7eb;
            color: #6b7280;
            font-size: 14px;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #3b82f6;
            color: white;
            text-decoration: none;
            border-radius: 6px;
            margin-top: 20px;
            font-weight: 500;
        }
        .button:hover {
            background-color: #2563eb;
        }
    </style>
</head>
<body>
    <div class="receipt">
        <div class="header">
            <div class="icon">{{if .Approved}}✓{{else}}✗{{end}}</div>
            <div class="status {{if .Approved}}success{{else}}failed{{end}}">
                Payment {{if .Approved}}Successful{{else}}Failed{{end}}
            </div>
        </div>

        {{if .Approved}}
            <div class="amount">${{.Amount}} {{.Currency}}</div>

            <div class="details">
                {{if .CardType}}
                <div class="detail-row">
                    <span class="detail-label">Card Type</span>
                    <span class="detail-value">{{.CardType}}</span>
                </div>
                {{end}}

                {{if .MaskedCard}}
                <div class="detail-row">
                    <span class="detail-label">Card Number</span>
                    <span class="detail-value">{{.MaskedCard}}</span>
                </div>
                {{end}}

                {{if .AuthCode}}
                <div class="detail-row">
                    <span class="detail-label">Authorization Code</span>
                    <span class="detail-value">{{.AuthCode}}</span>
                </div>
                {{end}}

                {{if .TransactionID}}
                <div class="detail-row">
                    <span class="detail-label">Transaction ID</span>
                    <span class="detail-value">{{.TransactionID}}</span>
                </div>
                {{end}}

                {{if .GroupID}}
                <div class="detail-row">
                    <span class="detail-label">Group ID</span>
                    <span class="detail-value">{{.GroupID}}</span>
                </div>
                {{end}}

                {{if .TranNbr}}
                <div class="detail-row">
                    <span class="detail-label">Reference Number</span>
                    <span class="detail-value">{{.TranNbr}}</span>
                </div>
                {{end}}

                {{if .InvoiceNbr}}
                <div class="detail-row">
                    <span class="detail-label">Invoice Number</span>
                    <span class="detail-value">{{.InvoiceNbr}}</span>
                </div>
                {{end}}
            </div>

            <div class="footer">
                <p>Thank you for your payment!</p>
                <p>A confirmation email has been sent to your email address.</p>
            </div>
        {{else}}
            <div class="details">
                <div class="detail-row">
                    <span class="detail-label">Status</span>
                    <span class="detail-value">{{.AuthRespText}}</span>
                </div>
                <div class="detail-row">
                    <span class="detail-label">Amount</span>
                    <span class="detail-value">${{.Amount}} {{.Currency}}</span>
                </div>
            </div>

            <div class="footer">
                <p>Your payment could not be processed.</p>
                <p>Please check your payment information and try again.</p>
                <a href="/" class="button">Try Again</a>
            </div>
        {{end}}
    </div>
</body>
</html>
`

// HTML template for error page
const errorTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Payment Error</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .error-page {
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            text-align: center;
        }
        .icon {
            font-size: 64px;
            color: #ef4444;
            margin-bottom: 20px;
        }
        h1 {
            color: #ef4444;
            margin-bottom: 20px;
        }
        .message {
            color: #6b7280;
            margin: 20px 0;
        }
        .details {
            background-color: #fef2f2;
            border: 1px solid #fecaca;
            padding: 15px;
            border-radius: 6px;
            margin: 20px 0;
            color: #991b1b;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background-color: #3b82f6;
            color: white;
            text-decoration: none;
            border-radius: 6px;
            margin-top: 20px;
            font-weight: 500;
        }
        .button:hover {
            background-color: #2563eb;
        }
    </style>
</head>
<body>
    <div class="error-page">
        <div class="icon">⚠</div>
        <h1>Payment Error</h1>
        <p class="message">{{.Message}}</p>
        {{if .Details}}
        <div class="details">{{.Details}}</div>
        {{end}}
        <a href="/" class="button">Return to Home</a>
    </div>
</body>
</html>
`
