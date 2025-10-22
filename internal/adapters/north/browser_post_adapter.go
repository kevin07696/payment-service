package north

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	pkgerrors "github.com/kevin07696/payment-service/pkg/errors"
	"github.com/shopspring/decimal"
)

// BrowserPostAdapter implements CreditCardGateway using North's Browser Post API
// This adapter handles backend operations with BRIC tokens obtained from frontend tokenization
type BrowserPostAdapter struct {
	config     AuthConfig
	baseURL    string
	httpClient ports.HTTPClient
	logger     ports.Logger
}

// NewBrowserPostAdapter creates a new Browser Post adapter with dependency injection
func NewBrowserPostAdapter(config AuthConfig, baseURL string, httpClient ports.HTTPClient, logger ports.Logger) *BrowserPostAdapter {
	return &BrowserPostAdapter{
		config:     config,
		baseURL:    baseURL,
		httpClient: httpClient,
		logger:     logger,
	}
}

// BrowserPostResponse represents the XML response structure from Browser Post API
type BrowserPostResponse struct {
	XMLName xml.Name `xml:"RESPONSE"`
	Fields  struct {
		Fields []BrowserPostField `xml:"FIELD"`
	} `xml:"FIELDS"`
}

// BrowserPostField represents a single field in the XML response
type BrowserPostField struct {
	Key   string `xml:"KEY,attr"`
	Value string `xml:",chardata"`
}

// Authorize authorizes a payment using a BRIC token
func (a *BrowserPostAdapter) Authorize(ctx context.Context, req *ports.AuthorizeRequest) (*ports.PaymentResult, error) {
	if req.Token == "" {
		return nil, pkgerrors.NewValidationError("token", "BRIC token is required")
	}

	// Parse EPI-Id into 4-part key
	parts := strings.Split(a.config.EPIId, "-")
	if len(parts) != 4 {
		return nil, pkgerrors.NewPaymentError("CONFIG_ERROR", "Invalid EPI-Id format", pkgerrors.CategoryInvalidRequest, false)
	}

	// Build form data for authorization
	formData := url.Values{}
	formData.Set("CUST_NBR", parts[0])
	formData.Set("MERCH_NBR", parts[1])
	formData.Set("DBA_NBR", parts[2])
	formData.Set("TERMINAL_NBR", parts[3])
	formData.Set("BRIC", req.Token)
	formData.Set("AMOUNT", fmt.Sprintf("%.2f", req.Amount.InexactFloat64()))
	formData.Set("CURRENCY", req.Currency)

	// Determine transaction type
	tranType := "A" // Authorization only
	if req.Capture {
		tranType = "S" // Sale (auth + capture)
	}
	formData.Set("TRAN_TYPE", tranType)

	// Add billing info if provided (for AVS verification)
	if req.BillingInfo.ZipCode != "" {
		formData.Set("ZIP_CODE", req.BillingInfo.ZipCode)
	}
	if req.BillingInfo.Address != "" {
		formData.Set("ADDRESS", req.BillingInfo.Address)
	}
	if req.BillingInfo.City != "" {
		formData.Set("CITY", req.BillingInfo.City)
	}
	if req.BillingInfo.State != "" {
		formData.Set("STATE", req.BillingInfo.State)
	}
	if req.BillingInfo.FirstName != "" {
		formData.Set("FIRST_NAME", req.BillingInfo.FirstName)
	}
	if req.BillingInfo.LastName != "" {
		formData.Set("LAST_NAME", req.BillingInfo.LastName)
	}

	endpoint := "/sale"
	resp, err := a.makeRequest(ctx, endpoint, formData)
	if err != nil {
		return nil, err
	}

	// Parse response fields
	responseCode := a.getFieldValue(resp, "RESP_CODE")
	responseText := a.getFieldValue(resp, "RESP_TEXT")
	authCode := a.getFieldValue(resp, "AUTH_CODE")
	transactionID := a.getFieldValue(resp, "TRANSACTION_ID")
	avsResponse := a.getFieldValue(resp, "AUTH_CARD_K")  // AVS response code
	cvvResponse := a.getFieldValue(resp, "AUTH_CARD_L")  // CVV response code

	// Check response code
	codeInfo := GetCreditCardResponseCode(responseCode)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(responseText)
	}

	status := models.StatusAuthorized
	if req.Capture {
		status = models.StatusCaptured
	}

	return &ports.PaymentResult{
		TransactionID:        transactionID,
		GatewayTransactionID: transactionID,
		Amount:               req.Amount,
		Status:               status,
		ResponseCode:         responseCode,
		Message:              responseText,
		AuthCode:             authCode,
		AVSResponse:          avsResponse,
		CVVResponse:          cvvResponse,
		Timestamp:            time.Now(),
	}, nil
}

// Capture captures a previously authorized payment using BRIC token
func (a *BrowserPostAdapter) Capture(ctx context.Context, req *ports.CaptureRequest) (*ports.PaymentResult, error) {
	if req.TransactionID == "" {
		return nil, pkgerrors.NewValidationError("transaction_id", "transaction ID is required")
	}

	parts := strings.Split(a.config.EPIId, "-")
	if len(parts) != 4 {
		return nil, pkgerrors.NewPaymentError("CONFIG_ERROR", "Invalid EPI-Id format", pkgerrors.CategoryInvalidRequest, false)
	}

	formData := url.Values{}
	formData.Set("CUST_NBR", parts[0])
	formData.Set("MERCH_NBR", parts[1])
	formData.Set("DBA_NBR", parts[2])
	formData.Set("TERMINAL_NBR", parts[3])
	formData.Set("TRANSACTION_ID", req.TransactionID)
	formData.Set("AMOUNT", fmt.Sprintf("%.2f", req.Amount.InexactFloat64()))

	endpoint := fmt.Sprintf("/sale/%s/capture", req.TransactionID)
	resp, err := a.makeRequest(ctx, endpoint, formData)
	if err != nil {
		return nil, err
	}

	responseCode := a.getFieldValue(resp, "RESP_CODE")
	responseText := a.getFieldValue(resp, "RESP_TEXT")
	transactionID := a.getFieldValue(resp, "TRANSACTION_ID")
	avsResponse := a.getFieldValue(resp, "AUTH_CARD_K")  // AVS response code
	cvvResponse := a.getFieldValue(resp, "AUTH_CARD_L")  // CVV response code

	codeInfo := GetCreditCardResponseCode(responseCode)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(responseText)
	}

	return &ports.PaymentResult{
		TransactionID:        transactionID,
		GatewayTransactionID: transactionID,
		Amount:               req.Amount,
		Status:               models.StatusCaptured,
		ResponseCode:         responseCode,
		Message:              responseText,
		AVSResponse:          avsResponse,
		CVVResponse:          cvvResponse,
		Timestamp:            time.Now(),
	}, nil
}

// Void voids a transaction using BRIC token
func (a *BrowserPostAdapter) Void(ctx context.Context, transactionID string) (*ports.PaymentResult, error) {
	if transactionID == "" {
		return nil, pkgerrors.NewValidationError("transaction_id", "transaction ID is required")
	}

	parts := strings.Split(a.config.EPIId, "-")
	if len(parts) != 4 {
		return nil, pkgerrors.NewPaymentError("CONFIG_ERROR", "Invalid EPI-Id format", pkgerrors.CategoryInvalidRequest, false)
	}

	formData := url.Values{}
	formData.Set("CUST_NBR", parts[0])
	formData.Set("MERCH_NBR", parts[1])
	formData.Set("DBA_NBR", parts[2])
	formData.Set("TERMINAL_NBR", parts[3])
	formData.Set("TRANSACTION_ID", transactionID)

	endpoint := fmt.Sprintf("/void/%s", transactionID)
	resp, err := a.makeRequest(ctx, endpoint, formData)
	if err != nil {
		return nil, err
	}

	responseCode := a.getFieldValue(resp, "RESP_CODE")
	responseText := a.getFieldValue(resp, "RESP_TEXT")
	txnID := a.getFieldValue(resp, "TRANSACTION_ID")
	avsResponse := a.getFieldValue(resp, "AUTH_CARD_K")  // AVS response code
	cvvResponse := a.getFieldValue(resp, "AUTH_CARD_L")  // CVV response code

	codeInfo := GetCreditCardResponseCode(responseCode)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(responseText)
	}

	return &ports.PaymentResult{
		TransactionID:        txnID,
		GatewayTransactionID: txnID,
		Amount:               decimal.Zero,
		Status:               models.StatusVoided,
		ResponseCode:         responseCode,
		Message:              responseText,
		AVSResponse:          avsResponse,
		CVVResponse:          cvvResponse,
		Timestamp:            time.Now(),
	}, nil
}

// Refund refunds a transaction using BRIC token
func (a *BrowserPostAdapter) Refund(ctx context.Context, req *ports.RefundRequest) (*ports.PaymentResult, error) {
	if req.TransactionID == "" {
		return nil, pkgerrors.NewValidationError("transaction_id", "transaction ID is required")
	}

	parts := strings.Split(a.config.EPIId, "-")
	if len(parts) != 4 {
		return nil, pkgerrors.NewPaymentError("CONFIG_ERROR", "Invalid EPI-Id format", pkgerrors.CategoryInvalidRequest, false)
	}

	formData := url.Values{}
	formData.Set("CUST_NBR", parts[0])
	formData.Set("MERCH_NBR", parts[1])
	formData.Set("DBA_NBR", parts[2])
	formData.Set("TERMINAL_NBR", parts[3])
	formData.Set("TRANSACTION_ID", req.TransactionID)
	formData.Set("AMOUNT", fmt.Sprintf("%.2f", req.Amount.InexactFloat64()))

	if req.Reason != "" {
		formData.Set("REASON", req.Reason)
	}

	endpoint := fmt.Sprintf("/refund/%s", req.TransactionID)
	resp, err := a.makeRequest(ctx, endpoint, formData)
	if err != nil {
		return nil, err
	}

	responseCode := a.getFieldValue(resp, "RESP_CODE")
	responseText := a.getFieldValue(resp, "RESP_TEXT")
	transactionID := a.getFieldValue(resp, "TRANSACTION_ID")
	avsResponse := a.getFieldValue(resp, "AUTH_CARD_K")  // AVS response code
	cvvResponse := a.getFieldValue(resp, "AUTH_CARD_L")  // CVV response code

	codeInfo := GetCreditCardResponseCode(responseCode)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(responseText)
	}

	return &ports.PaymentResult{
		TransactionID:        transactionID,
		GatewayTransactionID: transactionID,
		Amount:               req.Amount,
		Status:               models.StatusRefunded,
		ResponseCode:         responseCode,
		Message:              responseText,
		AVSResponse:          avsResponse,
		CVVResponse:          cvvResponse,
		Timestamp:            time.Now(),
	}, nil
}

// VerifyAccount verifies a BRIC token is valid
func (a *BrowserPostAdapter) VerifyAccount(ctx context.Context, req *ports.VerifyAccountRequest) (*ports.VerificationResult, error) {
	if req.Token == "" {
		return nil, pkgerrors.NewValidationError("token", "BRIC token is required")
	}

	parts := strings.Split(a.config.EPIId, "-")
	if len(parts) != 4 {
		return nil, pkgerrors.NewPaymentError("CONFIG_ERROR", "Invalid EPI-Id format", pkgerrors.CategoryInvalidRequest, false)
	}

	// Perform a $0.00 authorization to verify the token
	formData := url.Values{}
	formData.Set("CUST_NBR", parts[0])
	formData.Set("MERCH_NBR", parts[1])
	formData.Set("DBA_NBR", parts[2])
	formData.Set("TERMINAL_NBR", parts[3])
	formData.Set("BRIC", req.Token)
	formData.Set("AMOUNT", "0.00")
	formData.Set("TRAN_TYPE", "V") // Verification

	if req.BillingInfo.ZipCode != "" {
		formData.Set("ZIP_CODE", req.BillingInfo.ZipCode)
	}

	endpoint := "/verify"
	resp, err := a.makeRequest(ctx, endpoint, formData)
	if err != nil {
		// Network errors shouldn't fail verification - return unverified
		return &ports.VerificationResult{
			Verified:     false,
			ResponseCode: "96",
			Message:      "Unable to verify",
		}, nil
	}

	responseCode := a.getFieldValue(resp, "RESP_CODE")
	responseText := a.getFieldValue(resp, "RESP_TEXT")

	verified := responseCode == "00" || responseCode == "85" // 00 = approved, 85 = verified

	return &ports.VerificationResult{
		Verified:     verified,
		ResponseCode: responseCode,
		Message:      responseText,
	}, nil
}

// makeRequest makes a form-encoded HTTP request to the Browser Post API
func (a *BrowserPostAdapter) makeRequest(ctx context.Context, endpoint string, formData url.Values) (*BrowserPostResponse, error) {
	if a.logger != nil {
		a.logger.Info("making request to North Browser Post API",
			ports.String("endpoint", endpoint),
			ports.String("amount", formData.Get("AMOUNT")),
		)
	}

	// Build full URL
	fullURL := a.baseURL + endpoint

	// Calculate HMAC signature
	payload := formData.Encode()
	signature := CalculateSignature(a.config.EPIKey, endpoint, []byte(payload))

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", fullURL, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("EPI-Id", a.config.EPIId)
	httpReq.Header.Set("EPI-Signature", signature)

	// Make request
	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, pkgerrors.NewPaymentError("NETWORK_ERROR", "Failed to connect to payment gateway", pkgerrors.CategoryNetworkError, true)
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode >= 500 {
		return nil, pkgerrors.NewPaymentError("GATEWAY_ERROR", "Payment gateway error", pkgerrors.CategorySystemError, true)
	}

	if httpResp.StatusCode >= 400 {
		return nil, pkgerrors.NewPaymentError("REQUEST_ERROR", "Invalid request to payment gateway", pkgerrors.CategoryInvalidRequest, false)
	}

	// Parse XML response
	var resp BrowserPostResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XML response: %w", err)
	}

	return &resp, nil
}

// getFieldValue extracts a field value from the XML response
func (a *BrowserPostAdapter) getFieldValue(resp *BrowserPostResponse, key string) string {
	for _, field := range resp.Fields.Fields {
		if field.Key == key {
			return field.Value
		}
	}
	return ""
}
