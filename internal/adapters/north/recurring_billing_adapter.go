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
	"github.com/shopspring/decimal"
)

// RecurringBillingAdapter implements RecurringBillingGateway for North Recurring Billing API
type RecurringBillingAdapter struct {
	config     AuthConfig
	baseURL    string
	httpClient ports.HTTPClient
	logger     ports.Logger
}

// NewRecurringBillingAdapter creates a new Recurring Billing adapter with dependency injection
func NewRecurringBillingAdapter(config AuthConfig, baseURL string, httpClient ports.HTTPClient, logger ports.Logger) *RecurringBillingAdapter {
	return &RecurringBillingAdapter{
		config:     config,
		baseURL:    baseURL,
		httpClient: httpClient,
		logger:     logger,
	}
}

// CreateSubscriptionRequest represents the API request structure
type CreateSubscriptionRequest struct {
	CustomerData struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Email     string `json:"email"`
		Phone     string `json:"phone,omitempty"`
	} `json:"customerData"`
	PaymentMethod struct {
		PreviousPayment *struct {
			BRIC        string `json:"bric"`
			PaymentType string `json:"paymentType"`
		} `json:"previousPayment,omitempty"`
	} `json:"paymentMethod"`
	SubscriptionData struct {
		Amount           float64 `json:"amount"`
		Frequency        string  `json:"frequency"`
		BillingDate      string  `json:"billingDate"`
		FailureOption    string  `json:"failureOption"`
		Retries          int     `json:"retries"`
		NumberOfPayments int     `json:"numberOfPayments,omitempty"`
	} `json:"subscriptionData"`
}

// SubscriptionResponse represents the API response structure
type SubscriptionResponse struct {
	ID              int     `json:"id"`
	Amount          float64 `json:"amount"`
	Frequency       string  `json:"frequency"`
	Status          string  `json:"status"`
	NextBillingDate string  `json:"nextBillingDate"`
	Response        string  `json:"response,omitempty"`
	ResponseText    string  `json:"responseText,omitempty"`
}

// CreateSubscription implements RecurringBillingGateway.CreateSubscription
func (a *RecurringBillingAdapter) CreateSubscription(ctx context.Context, req *ports.SubscriptionRequest) (*ports.SubscriptionResult, error) {
	if req.PaymentToken == "" {
		return nil, pkgerrors.NewValidationError("payment_token", "payment token is required")
	}

	endpoint := "/subscription"

	apiReq := CreateSubscriptionRequest{}
	apiReq.CustomerData.FirstName = req.BillingInfo.FirstName
	apiReq.CustomerData.LastName = req.BillingInfo.LastName
	apiReq.CustomerData.Email = req.BillingInfo.Email
	apiReq.CustomerData.Phone = req.BillingInfo.Phone

	apiReq.PaymentMethod.PreviousPayment = &struct {
		BRIC        string `json:"bric"`
		PaymentType string `json:"paymentType"`
	}{
		BRIC:        req.PaymentToken,
		PaymentType: "CreditCard",
	}

	apiReq.SubscriptionData.Amount = req.Amount.InexactFloat64()
	apiReq.SubscriptionData.Frequency = mapFrequencyToAPI(req.Frequency)
	apiReq.SubscriptionData.BillingDate = req.StartDate.Format("2006-01-02")
	apiReq.SubscriptionData.FailureOption = mapFailureOptionToAPI(req.FailureOption)
	apiReq.SubscriptionData.Retries = req.MaxRetries
	apiReq.SubscriptionData.NumberOfPayments = req.NumberOfPayments

	var resp SubscriptionResponse
	if err := a.makeRequest(ctx, "POST", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	// Check for error response
	if resp.Response != "" && resp.Response != "00" {
		codeInfo := GetCreditCardResponseCode(resp.Response)
		return nil, codeInfo.ToPaymentError(resp.ResponseText)
	}

	nextBilling, _ := time.Parse("2006-01-02", resp.NextBillingDate)

	return &ports.SubscriptionResult{
		SubscriptionID:        fmt.Sprintf("%d", resp.ID),
		GatewaySubscriptionID: fmt.Sprintf("%d", resp.ID),
		Status:                mapSubscriptionStatusFromAPI(resp.Status),
		Amount:                req.Amount,
		Frequency:             req.Frequency,
		NextBillingDate:       nextBilling,
		Message:               resp.ResponseText,
	}, nil
}

// UpdateSubscription implements RecurringBillingGateway.UpdateSubscription
func (a *RecurringBillingAdapter) UpdateSubscription(ctx context.Context, subscriptionID string, req *ports.UpdateSubscriptionRequest) (*ports.SubscriptionResult, error) {
	endpoint := fmt.Sprintf("/subscription/%s", subscriptionID)

	apiReq := make(map[string]interface{})

	if req.Amount != nil {
		apiReq["amount"] = req.Amount.InexactFloat64()
	}
	if req.Frequency != nil {
		apiReq["frequency"] = mapFrequencyToAPI(*req.Frequency)
	}
	if req.NextBillingDate != nil {
		apiReq["nextBillingDate"] = req.NextBillingDate.Format("2006-01-02")
	}
	if req.PaymentToken != nil {
		apiReq["paymentMethod"] = map[string]interface{}{
			"previousPayment": map[string]string{
				"bric":        *req.PaymentToken,
				"paymentType": "CreditCard",
			},
		}
	}

	var resp SubscriptionResponse
	if err := a.makeRequest(ctx, "PUT", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	if resp.Response != "" && resp.Response != "00" {
		codeInfo := GetCreditCardResponseCode(resp.Response)
		return nil, codeInfo.ToPaymentError(resp.ResponseText)
	}

	nextBilling, _ := time.Parse("2006-01-02", resp.NextBillingDate)

	amount := decimal.NewFromFloat(resp.Amount)
	if req.Amount != nil {
		amount = *req.Amount
	}

	return &ports.SubscriptionResult{
		SubscriptionID:        subscriptionID,
		GatewaySubscriptionID: subscriptionID,
		Status:                mapSubscriptionStatusFromAPI(resp.Status),
		Amount:                amount,
		NextBillingDate:       nextBilling,
		Message:               resp.ResponseText,
	}, nil
}

// CancelSubscription implements RecurringBillingGateway.CancelSubscription
func (a *RecurringBillingAdapter) CancelSubscription(ctx context.Context, subscriptionID string, immediate bool) (*ports.SubscriptionResult, error) {
	endpoint := "/subscription/cancel"

	apiReq := map[string]interface{}{
		"subscriptionId": subscriptionID,
		"immediate":      immediate,
	}

	var resp SubscriptionResponse
	if err := a.makeRequest(ctx, "POST", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	return &ports.SubscriptionResult{
		SubscriptionID:        subscriptionID,
		GatewaySubscriptionID: subscriptionID,
		Status:                models.SubStatusCancelled,
		Message:               resp.ResponseText,
	}, nil
}

// PauseSubscription implements RecurringBillingGateway.PauseSubscription
func (a *RecurringBillingAdapter) PauseSubscription(ctx context.Context, subscriptionID string) (*ports.SubscriptionResult, error) {
	endpoint := "/subscription/pause"

	apiReq := map[string]interface{}{
		"subscriptionId": subscriptionID,
	}

	var resp SubscriptionResponse
	if err := a.makeRequest(ctx, "POST", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	return &ports.SubscriptionResult{
		SubscriptionID:        subscriptionID,
		GatewaySubscriptionID: subscriptionID,
		Status:                models.SubStatusPaused,
		Message:               resp.ResponseText,
	}, nil
}

// ResumeSubscription implements RecurringBillingGateway.ResumeSubscription
func (a *RecurringBillingAdapter) ResumeSubscription(ctx context.Context, subscriptionID string) (*ports.SubscriptionResult, error) {
	endpoint := "/subscription/resume"

	apiReq := map[string]interface{}{
		"subscriptionId": subscriptionID,
	}

	var resp SubscriptionResponse
	if err := a.makeRequest(ctx, "POST", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	nextBilling, _ := time.Parse("2006-01-02", resp.NextBillingDate)

	return &ports.SubscriptionResult{
		SubscriptionID:        subscriptionID,
		GatewaySubscriptionID: subscriptionID,
		Status:                models.SubStatusActive,
		NextBillingDate:       nextBilling,
		Message:               resp.ResponseText,
	}, nil
}

// GetSubscription implements RecurringBillingGateway.GetSubscription
func (a *RecurringBillingAdapter) GetSubscription(ctx context.Context, subscriptionID string) (*ports.SubscriptionResult, error) {
	endpoint := fmt.Sprintf("/subscription/%s", subscriptionID)

	var resp SubscriptionResponse
	if err := a.makeRequest(ctx, "GET", endpoint, nil, &resp); err != nil {
		return nil, err
	}

	nextBilling, _ := time.Parse("2006-01-02", resp.NextBillingDate)

	return &ports.SubscriptionResult{
		SubscriptionID:        subscriptionID,
		GatewaySubscriptionID: subscriptionID,
		Status:                mapSubscriptionStatusFromAPI(resp.Status),
		NextBillingDate:       nextBilling,
		Message:               resp.ResponseText,
	}, nil
}

// ListSubscriptions implements RecurringBillingGateway.ListSubscriptions
func (a *RecurringBillingAdapter) ListSubscriptions(ctx context.Context, customerID string) ([]*ports.SubscriptionResult, error) {
	endpoint := fmt.Sprintf("/subscription/list?customerId=%s", customerID)

	var respList []SubscriptionResponse
	if err := a.makeRequest(ctx, "GET", endpoint, nil, &respList); err != nil {
		return nil, err
	}

	results := make([]*ports.SubscriptionResult, len(respList))
	for i, resp := range respList {
		nextBilling, _ := time.Parse("2006-01-02", resp.NextBillingDate)
		results[i] = &ports.SubscriptionResult{
			SubscriptionID:        fmt.Sprintf("%d", resp.ID),
			GatewaySubscriptionID: fmt.Sprintf("%d", resp.ID),
			Status:                mapSubscriptionStatusFromAPI(resp.Status),
			NextBillingDate:       nextBilling,
		}
	}

	return results, nil
}

// ChargePaymentMethodRequest represents the one-time payment request
type ChargePaymentMethodRequest struct {
	PaymentMethodID int     `json:"PaymentMethodID"`
	Amount          float64 `json:"Amount"`
}

// ChargePaymentMethodResponse represents the one-time payment response
type ChargePaymentMethodResponse struct {
	Date       string  `json:"Date"`
	GUID       string  `json:"GUID"`
	Amount     float64 `json:"Amount"`
	Code       string  `json:"Code"`
	Text       string  `json:"Text"`
	Approval   string  `json:"Approval"`
	Successful bool    `json:"Successful"`
}

// ChargePaymentMethod processes a one-time payment using a stored payment method
// This is independent from subscriptions and does not count toward subscription payments
func (a *RecurringBillingAdapter) ChargePaymentMethod(
	ctx context.Context,
	paymentMethodID string,
	amount decimal.Decimal,
) (*ports.PaymentResult, error) {
	if paymentMethodID == "" {
		return nil, pkgerrors.NewValidationError("payment_method_id", "payment method ID is required")
	}

	endpoint := "/chargepaymentmethod"

	// Convert paymentMethodID string to int
	pmID := 0
	if _, err := fmt.Sscanf(paymentMethodID, "%d", &pmID); err != nil {
		return nil, pkgerrors.NewValidationError("payment_method_id", "invalid payment method ID format")
	}

	apiReq := ChargePaymentMethodRequest{
		PaymentMethodID: pmID,
		Amount:          amount.InexactFloat64(),
	}

	var resp ChargePaymentMethodResponse
	if err := a.makeRequest(ctx, "POST", endpoint, apiReq, &resp); err != nil {
		return nil, err
	}

	// Check for error response
	if !resp.Successful || (resp.Code != "" && resp.Code != "00") {
		codeInfo := GetCreditCardResponseCode(resp.Code)
		return nil, codeInfo.ToPaymentError(resp.Text)
	}

	timestamp, _ := time.Parse(time.RFC3339, resp.Date)
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	return &ports.PaymentResult{
		TransactionID:        resp.GUID,
		GatewayTransactionID: resp.GUID,
		Amount:               amount,
		Status:               models.StatusCaptured,
		ResponseCode:         resp.Code,
		Message:              resp.Text,
		AuthCode:             resp.Approval,
		Timestamp:            timestamp,
	}, nil
}

// makeRequest makes an HTTP request to the Recurring Billing API with HMAC authentication
func (a *RecurringBillingAdapter) makeRequest(ctx context.Context, method, endpoint string, request interface{}, response interface{}) error {
	var payloadBytes []byte
	var err error

	if request != nil {
		payloadBytes, err = json.Marshal(request)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
	} else {
		payloadBytes = []byte{}
	}

	signature := CalculateSignature(a.config.EPIKey, endpoint, payloadBytes)

	url := a.baseURL + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("EPI-Id", a.config.EPIId)
	httpReq.Header.Set("EPI-Signature", signature)

	if a.logger != nil {
		a.logger.Info("making request to North Recurring Billing",
			ports.String("method", method),
			ports.String("endpoint", endpoint),
		)
	}

	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return pkgerrors.NewPaymentError("NETWORK_ERROR", "Failed to connect to payment gateway", pkgerrors.CategoryNetworkError, true)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode >= 500 {
		return pkgerrors.NewPaymentError("GATEWAY_ERROR", "Payment gateway error", pkgerrors.CategorySystemError, true)
	}

	if httpResp.StatusCode >= 400 {
		return pkgerrors.NewPaymentError("REQUEST_ERROR", "Invalid request to payment gateway", pkgerrors.CategoryInvalidRequest, false)
	}

	if err := json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// Helper functions to map between domain and API formats

func mapFrequencyToAPI(freq models.BillingFrequency) string {
	switch freq {
	case models.FrequencyWeekly:
		return "Weekly"
	case models.FrequencyBiWeekly:
		return "BiWeekly"
	case models.FrequencyMonthly:
		return "Monthly"
	case models.FrequencyYearly:
		return "Yearly"
	default:
		return "Monthly"
	}
}

func mapFailureOptionToAPI(opt models.FailureOption) string {
	switch opt {
	case models.FailureForward:
		return "Forward"
	case models.FailureSkip:
		return "Skip"
	case models.FailurePause:
		return "Pause"
	default:
		return "Forward"
	}
}

func mapSubscriptionStatusFromAPI(status string) models.SubscriptionStatus {
	switch status {
	case "Active":
		return models.SubStatusActive
	case "Paused":
		return models.SubStatusPaused
	case "Cancelled":
		return models.SubStatusCancelled
	case "Expired":
		return models.SubStatusExpired
	default:
		return models.SubStatusActive
	}
}
