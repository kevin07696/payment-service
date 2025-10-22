package north

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	pkgerrors "github.com/kevin07696/payment-service/pkg/errors"
	"github.com/kevin07696/payment-service/test/mocks"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRecurringBillingTest(t *testing.T, handler http.HandlerFunc) (*RecurringBillingAdapter, *httptest.Server) {
	server := httptest.NewServer(handler)

	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}

	adapter := NewRecurringBillingAdapter(config, server.URL, httpClient, logger)

	return adapter, server
}

func TestRecurringBillingAdapter_CreateSubscription_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/subscription", r.URL.Path)

		// Verify headers
		assert.NotEmpty(t, r.Header.Get("EPI-Id"))
		assert.NotEmpty(t, r.Header.Get("EPI-Signature"))

		// Parse request
		var req CreateSubscriptionRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "John", req.CustomerData.FirstName)
		assert.Equal(t, "Doe", req.CustomerData.LastName)
		assert.Equal(t, "john@example.com", req.CustomerData.Email)
		assert.Equal(t, 99.99, req.SubscriptionData.Amount)
		assert.Equal(t, "Monthly", req.SubscriptionData.Frequency)
		assert.Equal(t, "Forward", req.SubscriptionData.FailureOption)
		assert.Equal(t, 3, req.SubscriptionData.Retries)

		// Send success response
		resp := SubscriptionResponse{
			ID:              12345,
			Amount:          99.99,
			Frequency:       "Monthly",
			Status:          "Active",
			NextBillingDate: "2025-11-20",
			Response:        "00",
			ResponseText:    "Subscription created successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupRecurringBillingTest(t, handler)
	defer server.Close()

	startDate := time.Date(2025, 10, 20, 0, 0, 0, 0, time.UTC)
	req := &ports.SubscriptionRequest{
		CustomerID:   "CUST-001",
		Amount:       decimal.NewFromFloat(99.99),
		Currency:     "USD",
		Frequency:    models.FrequencyMonthly,
		PaymentToken: "test-bric-token",
		BillingInfo: models.BillingInfo{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john@example.com",
		},
		StartDate:     startDate,
		MaxRetries:    3,
		FailureOption: models.FailureForward,
	}

	result, err := adapter.CreateSubscription(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "12345", result.SubscriptionID)
	assert.Equal(t, models.SubStatusActive, result.Status)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(99.99)))
	assert.Equal(t, models.FrequencyMonthly, result.Frequency)
	assert.Equal(t, "2025-11-20", result.NextBillingDate.Format("2006-01-02"))
}

func TestRecurringBillingAdapter_CreateSubscription_MissingToken(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not make request with missing token")
	}

	adapter, server := setupRecurringBillingTest(t, handler)
	defer server.Close()

	req := &ports.SubscriptionRequest{
		Amount:        decimal.NewFromFloat(99.99),
		PaymentToken:  "", // Missing
		FailureOption: models.FailureForward,
	}

	result, err := adapter.CreateSubscription(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	validationErr, ok := err.(*pkgerrors.ValidationError)
	require.True(t, ok)
	assert.Equal(t, "payment_token", validationErr.Field)
}

func TestRecurringBillingAdapter_CreateSubscription_DeclinedCard(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := SubscriptionResponse{
			Response:     "51",
			ResponseText: "DECLINED - Insufficient Funds",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupRecurringBillingTest(t, handler)
	defer server.Close()

	req := &ports.SubscriptionRequest{
		Amount:        decimal.NewFromFloat(99.99),
		PaymentToken:  "test-bric-token",
		Frequency:     models.FrequencyMonthly,
		StartDate:     time.Now(),
		FailureOption: models.FailureForward,
	}

	result, err := adapter.CreateSubscription(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok)
	assert.Equal(t, "51", paymentErr.Code)
	assert.Equal(t, pkgerrors.CategoryInsufficientFunds, paymentErr.Category)
}

func TestRecurringBillingAdapter_UpdateSubscription_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/subscription/12345", r.URL.Path)

		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		assert.Equal(t, 149.99, req["amount"])
		assert.Equal(t, "BiWeekly", req["frequency"])

		resp := SubscriptionResponse{
			ID:              12345,
			Amount:          149.99,
			Frequency:       "BiWeekly",
			Status:          "Active",
			NextBillingDate: "2025-11-03",
			Response:        "00",
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupRecurringBillingTest(t, handler)
	defer server.Close()

	newAmount := decimal.NewFromFloat(149.99)
	newFreq := models.FrequencyBiWeekly

	req := &ports.UpdateSubscriptionRequest{
		Amount:    &newAmount,
		Frequency: &newFreq,
	}

	result, err := adapter.UpdateSubscription(context.Background(), "12345", req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "12345", result.SubscriptionID)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(149.99)))
}

func TestRecurringBillingAdapter_CancelSubscription_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/subscription/cancel", r.URL.Path)

		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		assert.Equal(t, "12345", req["subscriptionId"])
		assert.Equal(t, true, req["immediate"])

		resp := SubscriptionResponse{
			ID:           12345,
			Status:       "Cancelled",
			ResponseText: "Subscription cancelled",
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupRecurringBillingTest(t, handler)
	defer server.Close()

	result, err := adapter.CancelSubscription(context.Background(), "12345", true)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "12345", result.SubscriptionID)
	assert.Equal(t, models.SubStatusCancelled, result.Status)
}

func TestRecurringBillingAdapter_PauseSubscription_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/subscription/pause", r.URL.Path)

		resp := SubscriptionResponse{
			ID:           12345,
			Status:       "Paused",
			ResponseText: "Subscription paused",
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupRecurringBillingTest(t, handler)
	defer server.Close()

	result, err := adapter.PauseSubscription(context.Background(), "12345")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.SubStatusPaused, result.Status)
}

func TestRecurringBillingAdapter_ResumeSubscription_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/subscription/resume", r.URL.Path)

		resp := SubscriptionResponse{
			ID:              12345,
			Status:          "Active",
			NextBillingDate: "2025-11-20",
			ResponseText:    "Subscription resumed",
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupRecurringBillingTest(t, handler)
	defer server.Close()

	result, err := adapter.ResumeSubscription(context.Background(), "12345")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.SubStatusActive, result.Status)
	assert.Equal(t, "2025-11-20", result.NextBillingDate.Format("2006-01-02"))
}

func TestRecurringBillingAdapter_GetSubscription_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/subscription/12345", r.URL.Path)

		resp := SubscriptionResponse{
			ID:              12345,
			Amount:          99.99,
			Frequency:       "Monthly",
			Status:          "Active",
			NextBillingDate: "2025-11-20",
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupRecurringBillingTest(t, handler)
	defer server.Close()

	result, err := adapter.GetSubscription(context.Background(), "12345")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "12345", result.SubscriptionID)
	assert.Equal(t, models.SubStatusActive, result.Status)
}

func TestRecurringBillingAdapter_ListSubscriptions_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.String(), "/subscription/list")
		assert.Contains(t, r.URL.Query().Get("customerId"), "CUST-001")

		resp := []SubscriptionResponse{
			{
				ID:              12345,
				Status:          "Active",
				NextBillingDate: "2025-11-20",
			},
			{
				ID:              12346,
				Status:          "Paused",
				NextBillingDate: "2025-12-01",
			},
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupRecurringBillingTest(t, handler)
	defer server.Close()

	results, err := adapter.ListSubscriptions(context.Background(), "CUST-001")

	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "12345", results[0].SubscriptionID)
	assert.Equal(t, models.SubStatusActive, results[0].Status)
	assert.Equal(t, "12346", results[1].SubscriptionID)
	assert.Equal(t, models.SubStatusPaused, results[1].Status)
}

func TestRecurringBillingAdapter_NetworkError(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}
	adapter := NewRecurringBillingAdapter(config, "http://invalid-url:9999", httpClient, logger)

	req := &ports.SubscriptionRequest{
		Amount:        decimal.NewFromFloat(99.99),
		PaymentToken:  "test-token",
		Frequency:     models.FrequencyMonthly,
		StartDate:     time.Now(),
		FailureOption: models.FailureForward,
	}

	result, err := adapter.CreateSubscription(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok)
	assert.Equal(t, "NETWORK_ERROR", paymentErr.Code)
	assert.True(t, paymentErr.IsRetriable)
}

func TestRecurringBillingAdapter_FrequencyMapping(t *testing.T) {
	tests := []struct {
		domain   models.BillingFrequency
		expected string
	}{
		{models.FrequencyWeekly, "Weekly"},
		{models.FrequencyBiWeekly, "BiWeekly"},
		{models.FrequencyMonthly, "Monthly"},
		{models.FrequencyYearly, "Yearly"},
	}

	for _, tt := range tests {
		t.Run(string(tt.domain), func(t *testing.T) {
			result := mapFrequencyToAPI(tt.domain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRecurringBillingAdapter_FailureOptionMapping(t *testing.T) {
	tests := []struct {
		domain   models.FailureOption
		expected string
	}{
		{models.FailureForward, "Forward"},
		{models.FailureSkip, "Skip"},
		{models.FailurePause, "Pause"},
	}

	for _, tt := range tests {
		t.Run(string(tt.domain), func(t *testing.T) {
			result := mapFailureOptionToAPI(tt.domain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRecurringBillingAdapter_StatusMapping(t *testing.T) {
	tests := []struct {
		api      string
		expected models.SubscriptionStatus
	}{
		{"Active", models.SubStatusActive},
		{"Paused", models.SubStatusPaused},
		{"Cancelled", models.SubStatusCancelled},
		{"Expired", models.SubStatusExpired},
	}

	for _, tt := range tests {
		t.Run(tt.api, func(t *testing.T) {
			result := mapSubscriptionStatusFromAPI(tt.api)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRecurringBillingAdapter_DependencyInjection(t *testing.T) {
	config := AuthConfig{
		EPIId:  "test-id",
		EPIKey: "test-key",
	}

	mockLogger := mocks.NewMockLogger()
	mockHTTP := mocks.NewMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		resp := SubscriptionResponse{
			ID:     123,
			Status: "Active",
		}
		body, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})

	adapter := NewRecurringBillingAdapter(config, "http://test.com", mockHTTP, mockLogger)

	req := &ports.SubscriptionRequest{
		Amount:        decimal.NewFromFloat(99.99),
		PaymentToken:  "token",
		Frequency:     models.FrequencyMonthly,
		StartDate:     time.Now(),
		FailureOption: models.FailureForward,
	}

	_, err := adapter.CreateSubscription(context.Background(), req)

	require.NoError(t, err)
	assert.Len(t, mockHTTP.Calls, 1)
	assert.Len(t, mockLogger.InfoCalls, 1)
	assert.Equal(t, "making request to North Recurring Billing", mockLogger.InfoCalls[0].Message)
}
