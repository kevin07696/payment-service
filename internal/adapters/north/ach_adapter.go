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

// ACHAdapter implements ACHGateway for North Pay-by-Bank (ACH) API
type ACHAdapter struct {
	config     AuthConfig
	baseURL    string
	httpClient ports.HTTPClient
	logger     ports.Logger
}

// NewACHAdapter creates a new ACH adapter with dependency injection
func NewACHAdapter(config AuthConfig, baseURL string, httpClient ports.HTTPClient, logger ports.Logger) *ACHAdapter {
	return &ACHAdapter{
		config:     config,
		baseURL:    baseURL,
		httpClient: httpClient,
		logger:     logger,
	}
}

// ACHResponse represents the XML response structure
type ACHResponse struct {
	XMLName xml.Name `xml:"RESPONSE"`
	Fields  struct {
		Fields []ACHField `xml:"FIELD"`
	} `xml:"FIELDS"`
}

// ACHField represents a single field in the XML response
type ACHField struct {
	Key   string `xml:"KEY,attr"`
	Value string `xml:",chardata"`
}

// ProcessPayment implements ACHGateway.ProcessPayment
func (a *ACHAdapter) ProcessPayment(ctx context.Context, req *ports.ACHPaymentRequest) (*ports.PaymentResult, error) {
	if req.RoutingNumber == "" || req.AccountNumber == "" {
		return nil, pkgerrors.NewValidationError("bank_account", "routing and account numbers are required")
	}

	// Parse EPI-Id into 4-part key
	parts := strings.Split(a.config.EPIId, "-")
	if len(parts) != 4 {
		return nil, pkgerrors.NewPaymentError("CONFIG_ERROR", "Invalid EPI-Id format", pkgerrors.CategoryInvalidRequest, false)
	}

	// Determine transaction type based on account type
	tranType := "CKC2" // Checking account debit
	if req.AccountType == models.AccountTypeSavings {
		tranType = "CKS2" // Savings account debit
	}

	// Build form data
	formData := url.Values{}
	formData.Set("CUST_NBR", parts[0])
	formData.Set("MERCH_NBR", parts[1])
	formData.Set("DBA_NBR", parts[2])
	formData.Set("TERMINAL_NBR", parts[3])
	formData.Set("TRAN_TYPE", tranType)
	formData.Set("CARD_ENT_METH", "X") // Manual entry
	formData.Set("AMOUNT", fmt.Sprintf("%.2f", req.Amount.InexactFloat64()))
	formData.Set("BATCH_ID", time.Now().Format("20060102"))
	formData.Set("TRAN_NBR", fmt.Sprintf("%d", time.Now().Unix()))
	formData.Set("ACCOUNT_NBR", req.AccountNumber)
	formData.Set("ROUTING_NBR", req.RoutingNumber)

	// Add customer info
	if req.BillingInfo.FirstName != "" {
		formData.Set("FIRST_NAME", req.BillingInfo.FirstName)
	}
	if req.BillingInfo.LastName != "" {
		formData.Set("LAST_NAME", req.BillingInfo.LastName)
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
	if req.BillingInfo.ZipCode != "" {
		formData.Set("ZIP_CODE", req.BillingInfo.ZipCode)
	}

	// Add SEC code if provided
	if req.SECCode != "" {
		formData.Set("STD_ENTRY_CLASS", string(req.SECCode))
	}

	// Add receiver name for CCD transactions
	if req.ReceiverName != "" {
		formData.Set("RECV_NAME", req.ReceiverName)
	}

	resp, err := a.makeRequest(ctx, formData)
	if err != nil {
		return nil, err
	}

	// Parse response fields
	responseCode := a.getFieldValue(resp, "AUTH_RESP")
	responseText := a.getFieldValue(resp, "AUTH_RESP_TEXT")
	authGUID := a.getFieldValue(resp, "AUTH_GUID")
	_ = a.getFieldValue(resp, "AUTH_MASKED_ACCOUNT_NBR") // For logging/audit if needed

	// Check response code
	codeInfo := GetACHResponseCode(responseCode)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(responseText)
	}

	return &ports.PaymentResult{
		TransactionID:        formData.Get("TRAN_NBR"),
		GatewayTransactionID: authGUID,
		Amount:               req.Amount,
		Status:               models.StatusCaptured,
		ResponseCode:         responseCode,
		Message:              responseText,
		Timestamp:            time.Now(),
	}, nil
}

// RefundPayment implements ACHGateway.RefundPayment
func (a *ACHAdapter) RefundPayment(ctx context.Context, transactionID string, amount decimal.Decimal) (*ports.PaymentResult, error) {
	parts := strings.Split(a.config.EPIId, "-")
	if len(parts) != 4 {
		return nil, pkgerrors.NewPaymentError("CONFIG_ERROR", "Invalid EPI-Id format", pkgerrors.CategoryInvalidRequest, false)
	}

	// ACH refund - assuming checking account (would need original transaction details in production)
	tranType := "CKC3" // Checking account credit (refund)

	formData := url.Values{}
	formData.Set("CUST_NBR", parts[0])
	formData.Set("MERCH_NBR", parts[1])
	formData.Set("DBA_NBR", parts[2])
	formData.Set("TERMINAL_NBR", parts[3])
	formData.Set("TRAN_TYPE", tranType)
	formData.Set("CARD_ENT_METH", "X")
	formData.Set("AMOUNT", fmt.Sprintf("%.2f", amount.InexactFloat64()))
	formData.Set("BATCH_ID", time.Now().Format("20060102"))
	formData.Set("TRAN_NBR", fmt.Sprintf("%d", time.Now().Unix()))
	formData.Set("ORIG_AUTH_GUID", transactionID)

	resp, err := a.makeRequest(ctx, formData)
	if err != nil {
		return nil, err
	}

	responseCode := a.getFieldValue(resp, "AUTH_RESP")
	responseText := a.getFieldValue(resp, "AUTH_RESP_TEXT")
	authGUID := a.getFieldValue(resp, "AUTH_GUID")

	codeInfo := GetACHResponseCode(responseCode)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(responseText)
	}

	return &ports.PaymentResult{
		TransactionID:        transactionID,
		GatewayTransactionID: authGUID,
		Amount:               amount,
		Status:               models.StatusRefunded,
		ResponseCode:         responseCode,
		Message:              responseText,
		Timestamp:            time.Now(),
	}, nil
}

// VerifyBankAccount implements ACHGateway.VerifyBankAccount
func (a *ACHAdapter) VerifyBankAccount(ctx context.Context, req *ports.BankAccountVerificationRequest) (*ports.VerificationResult, error) {
	// North validates routing/account numbers on payment requests
	// For explicit verification, we can attempt a $0.00 transaction or use their validation endpoint
	// This implementation does basic format validation

	if req.RoutingNumber == "" || len(req.RoutingNumber) != 9 {
		return &ports.VerificationResult{
			Verified:     false,
			ResponseCode: "78",
			Message:      "Invalid routing number format",
		}, nil
	}

	if req.AccountNumber == "" {
		return &ports.VerificationResult{
			Verified:     false,
			ResponseCode: "14",
			Message:      "Invalid account number",
		}, nil
	}

	// In production, you would make an actual API call to verify
	// For now, return success if format is valid
	return &ports.VerificationResult{
		Verified:     true,
		ResponseCode: "00",
		Message:      "Account format valid",
	}, nil
}

// makeRequest makes a form-encoded HTTP request to the ACH API
func (a *ACHAdapter) makeRequest(ctx context.Context, formData url.Values) (*ACHResponse, error) {
	if a.logger != nil {
		a.logger.Info("making request to North ACH API",
			ports.String("tran_type", formData.Get("TRAN_TYPE")),
			ports.String("amount", formData.Get("AMOUNT")),
		)
	}

	// Create HTTP request with form data
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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
	var resp ACHResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XML response: %w", err)
	}

	return &resp, nil
}

// getFieldValue extracts a field value from the XML response
func (a *ACHAdapter) getFieldValue(resp *ACHResponse, key string) string {
	for _, field := range resp.Fields.Fields {
		if field.Key == key {
			return field.Value
		}
	}
	return ""
}
