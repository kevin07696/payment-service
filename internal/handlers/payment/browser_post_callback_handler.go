package payment

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"

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
	dbAdapter         DatabaseAdapter
	browserPost       ports.BrowserPostAdapter
	paymentMethodSvc  PaymentMethodService
	logger            *zap.Logger
}

// NewBrowserPostCallbackHandler creates a new Browser Post callback handler
func NewBrowserPostCallbackHandler(
	dbAdapter DatabaseAdapter,
	browserPost ports.BrowserPostAdapter,
	paymentMethodSvc PaymentMethodService,
	logger *zap.Logger,
) *BrowserPostCallbackHandler {
	return &BrowserPostCallbackHandler{
		dbAdapter:        dbAdapter,
		browserPost:      browserPost,
		paymentMethodSvc: paymentMethodSvc,
		logger:           logger,
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

	// Check for duplicate transaction using TRAN_NBR (as recommended in EPX docs page 8)
	// This handles the PRG (POST-REDIRECT-GET) pattern where same response can be received multiple times
	if response.TranNbr != "" {
		existingTx, err := h.dbAdapter.Queries().GetTransactionByIdempotencyKey(r.Context(), pgtype.Text{
			String: response.TranNbr,
			Valid:  true,
		})

		if err == nil {
			// Transaction already exists - this is a duplicate callback
			h.logger.Info("Duplicate Browser Post callback detected",
				zap.String("tran_nbr", response.TranNbr),
				zap.String("existing_tx_id", existingTx.ID.String()),
			)
			// Still render success page with existing transaction
			h.renderReceiptPage(w, response, existingTx.ID.String())
			return
		}
	}

	// Store the transaction in database
	// We store AUTH_GUID (BRIC) even for guest checkouts because it's needed for:
	// - Refunds (most common reason)
	// - Voids/cancellations
	// - Chargeback defense
	// - Reconciliation with EPX settlement reports
	txID, err := h.storeTransaction(r.Context(), response)
	if err != nil {
		h.logger.Error("Failed to store transaction",
			zap.Error(err),
			zap.String("auth_guid", response.AuthGUID),
		)
		// Still show success to user if payment was approved, but log the error
		if response.AuthResp == "00" {
			h.renderReceiptPage(w, response, "")
			return
		}
		h.renderErrorPage(w, "Failed to record transaction", "")
		return
	}

	h.logger.Info("Successfully processed Browser Post callback",
		zap.String("transaction_id", txID),
		zap.String("auth_resp", response.AuthResp),
		zap.String("auth_guid", response.AuthGUID),
	)

	// Check if user wants to save payment method (from USER_DATA fields)
	// If yes and transaction approved, convert Financial BRIC to Storage BRIC
	if response.IsApproved && h.shouldSavePaymentMethod(response.RawParams) {
		if err := h.savePaymentMethod(r.Context(), response, txID); err != nil {
			h.logger.Error("Failed to save payment method",
				zap.Error(err),
				zap.String("transaction_id", txID),
			)
			// Don't fail the transaction - user can save it later
		}
	}

	// Render receipt page based on transaction status
	h.renderReceiptPage(w, response, txID)
}

// storeTransaction saves the transaction to the database
// AUTH_GUID (BRIC) is stored for refunds, voids, disputes, and reconciliation
func (h *BrowserPostCallbackHandler) storeTransaction(ctx context.Context, response *ports.BrowserPostResponse) (string, error) {
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
		return "", fmt.Errorf("invalid amount: %w", err)
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
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	return txID.String(), nil
}

// renderReceiptPage renders an HTML receipt page to the user
func (h *BrowserPostCallbackHandler) renderReceiptPage(w http.ResponseWriter, response *ports.BrowserPostResponse, txID string) {
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
		AgentID:        agentID,
		CustomerID:     customerID,
		FinancialBRIC:  response.AuthGUID,
		PaymentType:    domain.PaymentMethodType(paymentType),
		TransactionID:  txID,
		LastFour:       lastFour,
		CardBrand:      cardBrand,
		CardExpMonth:   cardExpMonth,
		CardExpYear:    cardExpYear,
		IsDefault:      false, // Don't auto-set as default
		FirstName:      firstName,
		LastName:       lastName,
		Address:        address,
		City:           city,
		State:          state,
		ZipCode:        zipCode,
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
