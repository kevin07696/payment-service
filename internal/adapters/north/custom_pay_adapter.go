package north

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	pkgerrors "github.com/kevin07696/payment-service/pkg/errors"
)

// CustomPayAdapter implements the CreditCardGateway interface for North Custom Pay API
type CustomPayAdapter struct {
	config     AuthConfig
	baseURL    string
	httpClient ports.HTTPClient
	logger     ports.Logger
}

// NewCustomPayAdapter creates a new Custom Pay adapter with dependency injection
func NewCustomPayAdapter(config AuthConfig, baseURL string, httpClient ports.HTTPClient, logger ports.Logger) *CustomPayAdapter {
	return &CustomPayAdapter{
		config:     config,
		baseURL:    baseURL,
		httpClient: httpClient,
		logger:     logger,
	}
}

// NewCustomPayAdapterWithDefaults creates a new Custom Pay adapter with default HTTP client
func NewCustomPayAdapterWithDefaults(config AuthConfig, baseURL string, logger ports.Logger) *CustomPayAdapter {
	return &CustomPayAdapter{
		config:  config,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// SaleRequest represents a sale request to Custom Pay API
type SaleRequest struct {
	Amount          float64 `json:"amount"`
	Capture         bool    `json:"capture"`
	Transaction     int64   `json:"transaction"`
	BatchID         string  `json:"batchID"`
	IndustryType    string  `json:"industryType"`    // E=Ecommerce
	CardEntryMethod string  `json:"cardEntryMethod"` // Z=Token
}

// SaleResponse represents the response from Custom Pay API
type SaleResponse struct {
	Data struct {
		Response string `json:"response"` // Response code (00, 51, etc.)
		Text     string `json:"text"`     // Response message
		AuthCode string `json:"authCode"` // Authorization code
	} `json:"data"`
	Reference struct {
		BRIC string `json:"bric"` // Token for future transactions
	} `json:"reference"`
	Status int `json:"status"` // HTTP status
}

// Authorize implements CreditCardGateway.Authorize
func (a *CustomPayAdapter) Authorize(ctx context.Context, req *ports.AuthorizeRequest) (*ports.PaymentResult, error) {
	if req.Token == "" {
		return nil, pkgerrors.NewValidationError("token", "payment token is required")
	}

	endpoint := fmt.Sprintf("/sale/%s", req.Token)

	// Generate unique transaction ID (in production, use a proper ID generator)
	transactionID := time.Now().Unix()

	apiReq := SaleRequest{
		Amount:          req.Amount.InexactFloat64(),
		Capture:         req.Capture,
		Transaction:     transactionID,
		BatchID:         time.Now().Format("20060102"),
		IndustryType:    "E", // Ecommerce
		CardEntryMethod: "Z", // Token-based
	}

	var resp SaleResponse
	if err := a.makeRequest(ctx, "POST", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	// Check response code
	codeInfo := GetCreditCardResponseCode(resp.Data.Response)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(resp.Data.Text)
	}

	status := models.StatusAuthorized
	if req.Capture {
		status = models.StatusCaptured
	}

	return &ports.PaymentResult{
		TransactionID:        fmt.Sprintf("%d", transactionID),
		GatewayTransactionID: resp.Reference.BRIC,
		Amount:               req.Amount,
		Status:               status,
		ResponseCode:         resp.Data.Response,
		Message:              resp.Data.Text,
		AuthCode:             resp.Data.AuthCode,
		Timestamp:            time.Now(),
	}, nil
}

// Capture implements CreditCardGateway.Capture
func (a *CustomPayAdapter) Capture(ctx context.Context, req *ports.CaptureRequest) (*ports.PaymentResult, error) {
	endpoint := fmt.Sprintf("/sale/%s/capture", req.TransactionID)

	apiReq := map[string]interface{}{
		"amount":          req.Amount.InexactFloat64(),
		"batchID":         time.Now().Format("20060102"),
		"transaction":     time.Now().Unix(),
		"cardEntryMethod": "Z",
	}

	var resp SaleResponse
	if err := a.makeRequest(ctx, "PUT", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	codeInfo := GetCreditCardResponseCode(resp.Data.Response)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(resp.Data.Text)
	}

	return &ports.PaymentResult{
		TransactionID:        req.TransactionID,
		GatewayTransactionID: resp.Reference.BRIC,
		Amount:               req.Amount,
		Status:               models.StatusCaptured,
		ResponseCode:         resp.Data.Response,
		Message:              resp.Data.Text,
		AuthCode:             resp.Data.AuthCode,
		Timestamp:            time.Now(),
	}, nil
}

// Void implements CreditCardGateway.Void
func (a *CustomPayAdapter) Void(ctx context.Context, transactionID string) (*ports.PaymentResult, error) {
	endpoint := fmt.Sprintf("/void/%s", transactionID)

	apiReq := map[string]interface{}{
		"batchID":         time.Now().Format("20060102"),
		"transaction":     time.Now().Unix(),
		"cardEntryMethod": "Z",
	}

	var resp SaleResponse
	if err := a.makeRequest(ctx, "PUT", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	codeInfo := GetCreditCardResponseCode(resp.Data.Response)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(resp.Data.Text)
	}

	return &ports.PaymentResult{
		TransactionID:        transactionID,
		GatewayTransactionID: resp.Reference.BRIC,
		Status:               models.StatusVoided,
		ResponseCode:         resp.Data.Response,
		Message:              resp.Data.Text,
		Timestamp:            time.Now(),
	}, nil
}

// Refund implements CreditCardGateway.Refund
func (a *CustomPayAdapter) Refund(ctx context.Context, req *ports.RefundRequest) (*ports.PaymentResult, error) {
	endpoint := fmt.Sprintf("/refund/%s", req.TransactionID)

	apiReq := map[string]interface{}{
		"amount":          req.Amount.InexactFloat64(),
		"batchID":         time.Now().Format("20060102"),
		"transaction":     time.Now().Unix(),
		"industryType":    "E",
		"cardEntryMethod": "Z",
	}

	var resp SaleResponse
	if err := a.makeRequest(ctx, "POST", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	codeInfo := GetCreditCardResponseCode(resp.Data.Response)
	if codeInfo.IsDeclined {
		return nil, codeInfo.ToPaymentError(resp.Data.Text)
	}

	return &ports.PaymentResult{
		TransactionID:        req.TransactionID,
		GatewayTransactionID: resp.Reference.BRIC,
		Amount:               req.Amount,
		Status:               models.StatusRefunded,
		ResponseCode:         resp.Data.Response,
		Message:              resp.Data.Text,
		Timestamp:            time.Now(),
	}, nil
}

// VerifyAccount implements CreditCardGateway.VerifyAccount
func (a *CustomPayAdapter) VerifyAccount(ctx context.Context, req *ports.VerifyAccountRequest) (*ports.VerificationResult, error) {
	endpoint := "/avs"

	apiReq := map[string]interface{}{
		"transaction":     time.Now().Unix(),
		"batchID":         time.Now().Format("20060102"),
		"cardEntryMethod": "Z",
	}

	var resp SaleResponse
	if err := a.makeRequest(ctx, "POST", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	codeInfo := GetCreditCardResponseCode(resp.Data.Response)

	return &ports.VerificationResult{
		Verified:     codeInfo.IsApproved,
		ResponseCode: resp.Data.Response,
		Message:      resp.Data.Text,
	}, nil
}

// makeRequest makes an HTTP request to the Custom Pay API with HMAC authentication
func (a *CustomPayAdapter) makeRequest(ctx context.Context, method, endpoint string, request interface{}, response interface{}) error {
	// Marshal request body
	payloadBytes, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Calculate HMAC signature
	signature := CalculateSignature(a.config.EPIKey, endpoint, payloadBytes)

	// Create HTTP request
	url := a.baseURL + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("EPI-Id", a.config.EPIId)
	httpReq.Header.Set("EPI-Signature", signature)

	// Log request (excluding sensitive data)
	if a.logger != nil {
		a.logger.Info("making request to North Custom Pay",
			ports.String("method", method),
			ports.String("endpoint", endpoint),
		)
	}

	// Make request
	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return pkgerrors.NewPaymentError("NETWORK_ERROR", "Failed to connect to payment gateway", pkgerrors.CategoryNetworkError, true)
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check HTTP status
	if httpResp.StatusCode >= 500 {
		return pkgerrors.NewPaymentError("GATEWAY_ERROR", "Payment gateway error", pkgerrors.CategorySystemError, true)
	}

	if httpResp.StatusCode >= 400 {
		return pkgerrors.NewPaymentError("REQUEST_ERROR", "Invalid request to payment gateway", pkgerrors.CategoryInvalidRequest, false)
	}

	// Parse response
	if err := json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}
