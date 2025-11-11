package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	serviceports "github.com/kevin07696/payment-service/internal/services/ports"
	"go.uber.org/zap"
)

// DatabaseAdapter defines the interface for database operations
type DatabaseAdapter interface {
	Queries() *sqlc.Queries
}

// PaymentMethodService defines the interface for payment method operations
type PaymentMethodService interface {
	ConvertFinancialBRICToStorageBRIC(ctx context.Context, req *serviceports.ConvertFinancialBRICRequest) (*domain.PaymentMethod, error)
}

// BrowserPostCallbackHandler handles the redirect callback from EPX Browser Post API
// This endpoint receives the transaction results after EPX processes the payment
type BrowserPostCallbackHandler struct {
	dbAdapter        DatabaseAdapter
	browserPost      ports.BrowserPostAdapter
	paymentMethodSvc PaymentMethodService
	logger           *zap.Logger
	epxPostURL       string // EPX Browser Post endpoint URL
	epxCustNbr       string // EPX Customer Number
	epxMerchNbr      string // EPX Merchant Number
	epxDBAnbr        string // EPX DBA Number
	epxTerminalNbr   string // EPX Terminal Number
	callbackBaseURL  string // Base URL for callback (e.g., "http://localhost:8081")
}

// NewBrowserPostCallbackHandler creates a new Browser Post callback handler
func NewBrowserPostCallbackHandler(
	dbAdapter DatabaseAdapter,
	browserPost ports.BrowserPostAdapter,
	paymentMethodSvc PaymentMethodService,
	logger *zap.Logger,
	epxPostURL string,
	epxCustNbr string,
	epxMerchNbr string,
	epxDBAnbr string,
	epxTerminalNbr string,
	callbackBaseURL string,
) *BrowserPostCallbackHandler {
	return &BrowserPostCallbackHandler{
		dbAdapter:        dbAdapter,
		browserPost:      browserPost,
		paymentMethodSvc: paymentMethodSvc,
		logger:           logger,
		epxPostURL:       epxPostURL,
		epxCustNbr:       epxCustNbr,
		epxMerchNbr:      epxMerchNbr,
		epxDBAnbr:        epxDBAnbr,
		epxTerminalNbr:   epxTerminalNbr,
		callbackBaseURL:  callbackBaseURL,
	}
}

// GetPaymentForm generates form configuration for Browser Post payment
// This endpoint creates a PENDING transaction immediately for audit trail
// Endpoint: GET /api/v1/payments/browser-post/form?amount=99.99&return_url=https://pos.example.com/complete&agent_id=merchant-123
func (h *BrowserPostCallbackHandler) GetPaymentForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.logger.Warn("Browser Post form generator received non-GET request",
			zap.String("method", r.Method),
		)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract parameters
	amount := r.URL.Query().Get("amount")
	if amount == "" {
		h.logger.Warn("Browser Post form request missing amount parameter")
		http.Error(w, "amount parameter is required", http.StatusBadRequest)
		return
	}

	returnURL := r.URL.Query().Get("return_url")
	if returnURL == "" {
		h.logger.Warn("Browser Post form request missing return_url parameter")
		http.Error(w, "return_url parameter is required", http.StatusBadRequest)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		agentID = h.epxCustNbr // Default to EPX customer number
	}

	// Validate amount format
	if _, err := fmt.Sscanf(amount, "%f", new(float64)); err != nil {
		h.logger.Warn("Invalid amount format",
			zap.String("amount", amount),
			zap.Error(err),
		)
		http.Error(w, "amount must be a valid number", http.StatusBadRequest)
		return
	}

	// Generate unique transaction number using Unix timestamp with microseconds
	// This ensures uniqueness even for rapid requests within the same second
	now := time.Now()
	tranNbr := fmt.Sprintf("%d%06d", now.Unix()%100000, now.Nanosecond()/1000)

	// Create PENDING transaction immediately for audit trail
	txID := uuid.New()
	groupID := uuid.New()

	// Parse amount to numeric
	var amountNumeric pgtype.Numeric
	if err := amountNumeric.Scan(amount); err != nil {
		h.logger.Error("Failed to parse amount",
			zap.String("amount", amount),
			zap.Error(err),
		)
		http.Error(w, "invalid amount format", http.StatusBadRequest)
		return
	}

	// Create PENDING transaction
	_, err := h.dbAdapter.Queries().CreateTransaction(r.Context(), sqlc.CreateTransactionParams{
		ID:                txID,
		GroupID:           groupID,
		AgentID:           agentID,
		CustomerID:        pgtype.Text{}, // No customer ID for Browser Post (guest checkout)
		Amount:            amountNumeric,
		Currency:          "USD",
		Status:            "pending", // PENDING status - will be updated by callback
		Type:              "charge",
		PaymentMethodType: "credit_card",
		PaymentMethodID:   pgtype.UUID{},
		IdempotencyKey: pgtype.Text{
			String: tranNbr,
			Valid:  true,
		},
		Metadata: []byte("{}"),
	})

	if err != nil {
		h.logger.Error("Failed to create PENDING transaction",
			zap.Error(err),
			zap.String("transaction_id", txID.String()),
		)
		http.Error(w, "failed to create transaction", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Created PENDING transaction for Browser Post",
		zap.String("transaction_id", txID.String()),
		zap.String("group_id", groupID.String()),
		zap.String("amount", amount),
		zap.String("tran_nbr", tranNbr),
	)

	// Build form configuration
	formConfig := map[string]interface{}{
		// Transaction IDs (for POS to track)
		"transactionId": txID.String(),
		"groupId":       groupID.String(),

		// EPX endpoint URL (where the form will POST to)
		"postURL": h.epxPostURL,

		// EPX credentials (hidden fields)
		"custNbr":     h.epxCustNbr,
		"merchNbr":    h.epxMerchNbr,
		"dBAnbr":      h.epxDBAnbr,
		"terminalNbr": h.epxTerminalNbr,

		// Transaction details
		"amount":       amount,
		"tranNbr":      tranNbr,
		"tranGroup":    "SALE",
		"tranCode":     "SALE",
		"industryType": "E", // E-commerce
		"cardEntMeth":  "E", // E-commerce card entry

		// Callback URL (where EPX will redirect after payment - whitelisted)
		"redirectURL": h.callbackBaseURL + "/api/v1/payments/browser-post/callback",

		// Pass return_url through EPX USER_DATA field (state parameter pattern)
		"userData1": "return_url=" + returnURL,

		// Additional fields for display/tracking
		"merchantName": "Payment Service",
	}

	// Return JSON response
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

	h.logger.Info("Received Browser Post callback",
		zap.Int("form_values", len(r.Form)),
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

	// Look up PENDING transaction by TRAN_NBR idempotency key
	// Transaction should exist from CreateBrowserPostForm call
	existingTx, err := h.dbAdapter.Queries().GetTransactionByIdempotencyKey(r.Context(), pgtype.Text{
		String: response.TranNbr,
		Valid:  true,
	})

	if err != nil {
		h.logger.Error("PENDING transaction not found for callback",
			zap.Error(err),
			zap.String("tran_nbr", response.TranNbr),
		)
		h.renderErrorPage(w, "Transaction not found", "Please contact support")
		return
	}

	// Check if transaction was already updated (duplicate callback)
	if existingTx.Status != "pending" {
		h.logger.Info("Duplicate callback - transaction already processed",
			zap.String("tran_nbr", response.TranNbr),
			zap.String("existing_tx_id", existingTx.ID.String()),
			zap.String("status", existingTx.Status),
		)
		// Extract return_url from USER_DATA_1 and redirect
		returnURL := h.extractReturnURL(response.RawParams)
		if returnURL != "" {
			h.redirectToService(w, returnURL, existingTx.ID.String(), existingTx.GroupID.String(), existingTx.Status, response)
		} else {
			h.renderReceiptPage(w, response, existingTx.ID.String(), existingTx.GroupID.String())
		}
		return
	}

	// UPDATE the PENDING transaction with EPX response
	err = h.updateTransaction(r.Context(), existingTx.ID, response)
	if err != nil {
		h.logger.Error("Failed to update transaction",
			zap.Error(err),
			zap.String("transaction_id", existingTx.ID.String()),
			zap.String("auth_guid", response.AuthGUID),
		)
		h.renderErrorPage(w, "Failed to update transaction", "")
		return
	}

	// Determine final status
	status := "failed"
	if response.IsApproved {
		status = "completed"
	}

	h.logger.Info("Successfully updated transaction from Browser Post callback",
		zap.String("transaction_id", existingTx.ID.String()),
		zap.String("group_id", existingTx.GroupID.String()),
		zap.String("status", status),
		zap.String("auth_resp", response.AuthResp),
		zap.String("auth_guid", response.AuthGUID),
	)

	// Check if user wants to save payment method (from USER_DATA fields)
	// If yes and transaction approved, convert Financial BRIC to Storage BRIC
	if response.IsApproved && h.shouldSavePaymentMethod(response.RawParams) {
		if err := h.savePaymentMethod(r.Context(), response, existingTx.ID.String()); err != nil {
			h.logger.Error("Failed to save payment method",
				zap.Error(err),
				zap.String("transaction_id", existingTx.ID.String()),
			)
			// Don't fail the transaction - user can save it later
		}
	}

	// Extract return_url from USER_DATA_1 (state parameter pattern)
	returnURL := h.extractReturnURL(response.RawParams)

	if returnURL != "" {
		// Redirect to calling service (POS/e-commerce/etc.) with transaction data
		h.redirectToService(w, returnURL, existingTx.ID.String(), existingTx.GroupID.String(), status, response)
	} else {
		// Fallback: render simple receipt if no return_url provided
		h.renderReceiptPage(w, response, existingTx.ID.String(), existingTx.GroupID.String())
	}
}

// storeTransaction saves the transaction to the database
// AUTH_GUID (BRIC) is stored for refunds, voids, disputes, and reconciliation
// Returns transaction ID and group ID for linking to external systems
func (h *BrowserPostCallbackHandler) storeTransaction(ctx context.Context, response *ports.BrowserPostResponse) (string, string, error) {
	// Determine status from AUTH_RESP
	// "00" = approved, others = failed/declined
	status := "failed"
	if response.IsApproved {
		status = "completed"
	}

	// Determine transaction type (Browser Post is always charge, refunds go through Server Post)
	txType := "charge"

	// Create transaction
	txID := uuid.New()
	groupID := uuid.New()

	// Parse amount to numeric
	var amountNumeric pgtype.Numeric
	if err := amountNumeric.Scan(response.Amount); err != nil {
		return "", "", fmt.Errorf("invalid amount: %w", err)
	}

	// Get agent ID from raw params (CUST_NBR)
	agentID := "unknown"
	if custNbr, ok := response.RawParams["CUST_NBR"]; ok && custNbr != "" {
		agentID = custNbr
	}

	_, err := h.dbAdapter.Queries().CreateTransaction(ctx, sqlc.CreateTransactionParams{
		ID:                txID,
		GroupID:           groupID,
		AgentID:           agentID,
		CustomerID:        pgtype.Text{}, // No customer ID in Browser Post (guest checkout)
		Amount:            amountNumeric,
		Currency:          "USD",
		Status:            status,
		Type:              txType,
		PaymentMethodType: "credit_card",
		PaymentMethodID:   pgtype.UUID{}, // No saved payment method for Browser Post
		AuthGuid: pgtype.Text{
			String: response.AuthGUID,
			Valid:  response.AuthGUID != "",
		},
		AuthResp: pgtype.Text{
			String: response.AuthResp,
			Valid:  response.AuthResp != "",
		},
		AuthCode: pgtype.Text{
			String: response.AuthCode,
			Valid:  response.AuthCode != "",
		},
		AuthRespText: pgtype.Text{
			String: response.AuthRespText,
			Valid:  response.AuthRespText != "",
		},
		AuthCardType: pgtype.Text{
			String: response.AuthCardType,
			Valid:  response.AuthCardType != "",
		},
		AuthAvs: pgtype.Text{
			String: response.AuthAVS,
			Valid:  response.AuthAVS != "",
		},
		AuthCvv2: pgtype.Text{
			String: response.AuthCVV2,
			Valid:  response.AuthCVV2 != "",
		},
		IdempotencyKey: pgtype.Text{
			String: response.TranNbr,
			Valid:  response.TranNbr != "",
		},
		Metadata: []byte("{}"),
	})

	if err != nil {
		return "", "", fmt.Errorf("failed to create transaction: %w", err)
	}

	return txID.String(), groupID.String(), nil
}

// updateTransaction updates an existing PENDING transaction with EPX callback data
func (h *BrowserPostCallbackHandler) updateTransaction(ctx context.Context, txID uuid.UUID, response *ports.BrowserPostResponse) error {
	// Determine status from AUTH_RESP
	status := "failed"
	if response.IsApproved {
		status = "completed"
	}

	// Update transaction with all EPX response fields
	_, err := h.dbAdapter.Queries().UpdateTransaction(ctx, sqlc.UpdateTransactionParams{
		ID:     txID,
		Status: status,
		AuthGuid: pgtype.Text{
			String: response.AuthGUID,
			Valid:  response.AuthGUID != "",
		},
		AuthResp: pgtype.Text{
			String: response.AuthResp,
			Valid:  response.AuthResp != "",
		},
		AuthCode: pgtype.Text{
			String: response.AuthCode,
			Valid:  response.AuthCode != "",
		},
		AuthRespText: pgtype.Text{
			String: response.AuthRespText,
			Valid:  response.AuthRespText != "",
		},
		AuthCardType: pgtype.Text{
			String: response.AuthCardType,
			Valid:  response.AuthCardType != "",
		},
		AuthAvs: pgtype.Text{
			String: response.AuthAVS,
			Valid:  response.AuthAVS != "",
		},
		AuthCvv2: pgtype.Text{
			String: response.AuthCVV2,
			Valid:  response.AuthCVV2 != "",
		},
	})

	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return nil
}

// extractReturnURL extracts the return_url from EPX USER_DATA_1 field
// Uses state parameter pattern to relay POS/service URL through EPX
func (h *BrowserPostCallbackHandler) extractReturnURL(rawParams map[string]string) string {
	// USER_DATA_1 format: "return_url=https://pos.example.com/complete"
	if userData1, ok := rawParams["USER_DATA_1"]; ok {
		if len(userData1) > 11 && userData1[:11] == "return_url=" {
			return userData1[11:] // Extract URL after "return_url="
		}
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
// This is indicated by USER_DATA_1 field containing "save_payment_method=true"
func (h *BrowserPostCallbackHandler) shouldSavePaymentMethod(rawParams map[string]string) bool {
	// Check USER_DATA_1 for save_payment_method flag
	if userData1, ok := rawParams["USER_DATA_1"]; ok {
		return strings.Contains(strings.ToLower(userData1), "save_payment_method=true") ||
			strings.Contains(strings.ToLower(userData1), "save_payment_method=1")
	}
	return false
}

// savePaymentMethod converts the Financial BRIC to a Storage BRIC and saves it
func (h *BrowserPostCallbackHandler) savePaymentMethod(ctx context.Context, response *ports.BrowserPostResponse, txID string) error {
	// Extract customer_id from USER_DATA_2
	customerID, ok := response.RawParams["USER_DATA_2"]
	if !ok || customerID == "" {
		return fmt.Errorf("customer_id not provided in USER_DATA_2")
	}

	// Extract agent_id from CUST_NBR
	agentID, ok := response.RawParams["CUST_NBR"]
	if !ok || agentID == "" {
		return fmt.Errorf("agent_id not found in CUST_NBR")
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

	// Build ConvertFinancialBRICRequest
	req := &serviceports.ConvertFinancialBRICRequest{
		AgentID:       agentID,
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
		zap.String("agent_id", agentID),
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
