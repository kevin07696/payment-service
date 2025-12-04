package payment

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	"go.uber.org/zap"
)

// BrowserPostCallbackHandler handles Browser Post HTTP requests
// Responsible for HTTP concerns only - parsing requests and formatting responses
// Business logic is delegated to BrowserPostService and PaymentMethodService
type BrowserPostCallbackHandler struct {
	browserPostSvc   ports.BrowserPostService
	paymentMethodSvc ports.PaymentMethodService
	renderer         *TemplateRenderer
	logger           *zap.Logger
}

// NewBrowserPostCallbackHandler creates a new Browser Post callback handler
func NewBrowserPostCallbackHandler(
	browserPostSvc ports.BrowserPostService,
	paymentMethodSvc ports.PaymentMethodService,
	renderer *TemplateRenderer,
	logger *zap.Logger,
) *BrowserPostCallbackHandler {
	return &BrowserPostCallbackHandler{
		browserPostSvc:   browserPostSvc,
		paymentMethodSvc: paymentMethodSvc,
		renderer:         renderer,
		logger:           logger,
	}
}

// GetPaymentForm handles GET /api/v1/payments/browser-post/form
func (h *BrowserPostCallbackHandler) GetPaymentForm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse required parameters
	transactionID := r.URL.Query().Get("transaction_id")
	if transactionID == "" {
		http.Error(w, "transaction_id parameter is required", http.StatusBadRequest)
		return
	}

	merchantID := r.URL.Query().Get("merchant_id")
	if merchantID == "" {
		http.Error(w, "merchant_id parameter is required", http.StatusBadRequest)
		return
	}

	amount := r.URL.Query().Get("amount")
	if amount == "" {
		http.Error(w, "amount parameter is required", http.StatusBadRequest)
		return
	}

	returnURL := r.URL.Query().Get("return_url")
	if returnURL == "" {
		http.Error(w, "return_url parameter is required", http.StatusBadRequest)
		return
	}

	if _, err := url.Parse(returnURL); err != nil {
		http.Error(w, "invalid return_url format", http.StatusBadRequest)
		return
	}

	// Parse optional parameters
	transactionType := r.URL.Query().Get("transaction_type")
	if transactionType == "" {
		transactionType = "SALE"
	}
	customerID := r.URL.Query().Get("customer_id")

	// Call service
	resp, err := h.browserPostSvc.GenerateFormConfig(r.Context(), &ports.GenerateFormConfigRequest{
		TransactionID:   transactionID,
		MerchantID:      merchantID,
		Amount:          amount,
		TransactionType: transactionType,
		CustomerID:      customerID,
		ReturnURL:       returnURL,
	})
	if err != nil {
		h.logger.Error("Failed to generate form config", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Build JSON response per North Developer Browser Post API Integration Guide
	formConfig := map[string]interface{}{
		"transactionId": resp.TransactionID,
		"epxTranNbr":    resp.EPXTranNbr,
		"tac":           resp.TAC,
		"expiresAt":     resp.ExpiresAt.Unix(),
		"postURL":       resp.PostURL,
		"custNbr":       resp.CustNbr,
		"merchNbr":      resp.MerchNbr,
		"dbaName":       resp.DBAName,
		"terminalNbr":   resp.TerminalNbr,
		"industryType":  resp.IndustryType,
		"tranCode":      resp.TranCode, // EPX TRAN_CODE for Browser POST form (SALE, AUTH, STORAGE, ACH_STORAGE_C, ACH_STORAGE_S)
		"redirectURL":   resp.RedirectURL,
		"returnUrl":     resp.ReturnURL,
		"merchantId":    resp.MerchantID,
		"merchantName":  resp.MerchantName,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(formConfig); err != nil {
		h.logger.Error("Failed to encode form configuration", zap.Error(err))
	}
}

// HandleCallback handles GET/POST /api/v1/payments/browser-post/callback
// EPX redirects via 302 which browsers follow as GET with query parameters
func (h *BrowserPostCallbackHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderError(w, "Invalid request data", "", "")
		return
	}

	// Parse EPX response from query params (GET) or form body (POST)
	// r.Form contains both URL query values and POST body values
	params := make(map[string][]string)
	for key, values := range r.Form {
		params[key] = values
	}

	epxResponse, err := h.browserPostSvc.ParseRedirectResponse(params)
	if err != nil {
		h.logger.Error("Failed to parse Browser Post response", zap.Error(err))
		h.renderError(w, "Failed to process payment response", err.Error(), "")
		return
	}

	// Extract transaction type
	transactionTypeStr := epxResponse.RawParams["transaction_type"]
	if transactionTypeStr == "" {
		transactionTypeStr = "SALE"
	}
	txType := domain.ParseRequestTransactionType(transactionTypeStr)

	// Extract return URL for error pages
	returnURL := epxResponse.RawParams["USER_DATA_1"]

	// Process callback
	callbackReq := &ports.ProcessCallbackRequest{
		TranNbr:         epxResponse.TranNbr,
		AuthGUID:        epxResponse.AuthGUID,
		AuthResp:        epxResponse.AuthResp,
		AuthCode:        epxResponse.AuthCode,
		AuthCardType:    epxResponse.AuthCardType,
		AuthRespText:    epxResponse.AuthRespText,
		AuthAVS:         epxResponse.AuthAVS,
		AuthCVV2:        epxResponse.AuthCVV2,
		Amount:          epxResponse.Amount,
		IsApproved:      epxResponse.IsApproved,
		TransactionID:   epxResponse.RawParams["transaction_id"],
		MerchantID:      epxResponse.RawParams["merchant_id"],
		TransactionType: txType,
		CustomerID:      epxResponse.RawParams["USER_DATA_2"],
		RawParams:       epxResponse.RawParams,
	}

	callbackResp, err := h.browserPostSvc.ProcessCallback(r.Context(), callbackReq)
	if err != nil {
		h.logger.Error("Failed to process callback", zap.Error(err))
		h.renderError(w, err.Error(), "", returnURL)
		return
	}

	// Route to appropriate response based on transaction type
	switch {
	case txType == domain.RequestTransactionTypeStorage && epxResponse.IsApproved:
		h.handleCreditCardStorage(w, r, epxResponse, returnURL)

	case txType.IsACHStorage() && epxResponse.IsApproved:
		h.handleBankAccountStorage(w, r, txType, epxResponse, returnURL)

	default:
		h.renderReceipt(w, epxResponse, callbackResp, returnURL)
	}
}

// handleCreditCardStorage saves credit card and renders confirmation
func (h *BrowserPostCallbackHandler) handleCreditCardStorage(w http.ResponseWriter, r *http.Request, epxResponse *ports.BrowserPostResponse, returnURL string) {
	merchantID := epxResponse.RawParams["merchant_id"]
	customerID := epxResponse.RawParams["customer_id"]

	pm, err := h.paymentMethodSvc.SaveCreditCardFromCallback(r.Context(), &ports.SaveCreditCardFromCallbackRequest{
		MerchantID:       merchantID,
		CustomerID:       customerID,
		BRIC:             epxResponse.AuthGUID,
		MaskedAccountNbr: epxResponse.RawParams["AUTH_MASKED_ACCOUNT_NBR"],
		ExpirationDate:   epxResponse.RawParams["EXP_DATE"],
		CardTypeCode:     epxResponse.AuthCardType,
	})
	if err != nil {
		h.logger.Error("Failed to save credit card", zap.Error(err))
		h.renderError(w, "Failed to save payment method", err.Error(), returnURL)
		return
	}

	h.logger.Info("Credit card saved", zap.String("payment_method_id", pm.ID))

	// Extract last four and expiration for display
	lastFour := domain.ExtractLastFour(epxResponse.RawParams["AUTH_MASKED_ACCOUNT_NBR"])
	expDate := domain.FormatExpirationDateMMYY(epxResponse.RawParams["EXP_DATE"])
	cardBrand := string(domain.CardBrandFromEPXCode(epxResponse.AuthCardType))

	h.renderTemplate(w, TemplatePaymentMethodCreditCard, &CreditCardData{
		CardBrand:       cardBrand,
		LastFour:        lastFour,
		ExpirationDate:  expDate,
		PaymentMethodID: pm.ID,
		ReturnURL:       returnURL,
	})
}

// handleBankAccountStorage saves bank account, sends prenote, and renders confirmation
func (h *BrowserPostCallbackHandler) handleBankAccountStorage(w http.ResponseWriter, r *http.Request, txType domain.RequestTransactionType, epxResponse *ports.BrowserPostResponse, returnURL string) {
	merchantID := epxResponse.RawParams["merchant_id"]
	customerID := epxResponse.RawParams["customer_id"]

	// Step 1: Save ACH payment method
	pm, err := h.paymentMethodSvc.SaveACHFromCallback(r.Context(), &ports.SaveACHFromCallbackRequest{
		MerchantID:       merchantID,
		CustomerID:       customerID,
		BRIC:             epxResponse.AuthGUID,
		MaskedAccountNbr: epxResponse.RawParams["AUTH_MASKED_ACCOUNT_NBR"],
		TransactionType:  txType,
	})
	if err != nil {
		h.logger.Error("Failed to save bank account", zap.Error(err))
		h.renderError(w, "Failed to save payment method", err.Error(), returnURL)
		return
	}

	h.logger.Info("Bank account saved (unverified)", zap.String("payment_method_id", pm.ID))

	// Step 2: Send prenote
	accountType := "Checking"
	if !txType.IsCheckingAccount() {
		accountType = "Savings"
	}

	if err := h.paymentMethodSvc.SendPrenote(r.Context(), &ports.SendPrenoteRequest{
		MerchantID:      merchantID,
		PaymentMethodID: pm.ID,
		CustomerID:      customerID,
		BRIC:            epxResponse.AuthGUID,
		AccountType:     accountType,
	}); err != nil {
		h.logger.Error("Failed to send prenote", zap.Error(err), zap.String("payment_method_id", pm.ID))
		// Don't fail - payment method was saved, prenote can be retried
	} else {
		h.logger.Info("Prenote sent", zap.String("payment_method_id", pm.ID))
	}

	lastFour := domain.ExtractLastFour(epxResponse.RawParams["AUTH_MASKED_ACCOUNT_NBR"])

	h.renderTemplate(w, TemplatePaymentMethodBankAccount, &BankAccountData{
		AccountType:     accountType,
		LastFour:        lastFour,
		PaymentMethodID: pm.ID,
		ReturnURL:       returnURL,
	})
}

// renderReceipt renders the receipt template for SALE/AUTH transactions
func (h *BrowserPostCallbackHandler) renderReceipt(w http.ResponseWriter, epxResponse *ports.BrowserPostResponse, callbackResp *ports.ProcessCallbackResponse, returnURL string) {
	// Per North Developer Browser Post API Guide: EPX returns AUTH_ACCOUNT_NBR in response
	lastFour := domain.ExtractLastFour(epxResponse.RawParams["AUTH_ACCOUNT_NBR"])
	maskedCard := domain.FormatMaskedCard(lastFour)

	h.renderTemplate(w, TemplateReceipt, &ReceiptData{
		Approved:      epxResponse.IsApproved,
		Amount:        epxResponse.Amount,
		Currency:      "USD",
		CardType:      string(domain.CardBrandFromEPXCode(epxResponse.AuthCardType)),
		MaskedCard:    maskedCard,
		AuthCode:      epxResponse.AuthCode,
		AuthRespText:  epxResponse.AuthRespText,
		TransactionID: callbackResp.TransactionID,
		TranNbr:       epxResponse.TranNbr,
		ReturnURL:     returnURL,
	})
}

// renderError renders the error template
func (h *BrowserPostCallbackHandler) renderError(w http.ResponseWriter, message, details, returnURL string) {
	h.renderTemplate(w, TemplateError, &ErrorData{
		Message:   message,
		Details:   details,
		ReturnURL: returnURL,
	})
}

// renderTemplate renders a template with proper headers
func (h *BrowserPostCallbackHandler) renderTemplate(w http.ResponseWriter, name TemplateName, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.renderer.Render(w, name, data); err != nil {
		h.logger.Error("Failed to render template", zap.Error(err), zap.String("template", string(name)))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

