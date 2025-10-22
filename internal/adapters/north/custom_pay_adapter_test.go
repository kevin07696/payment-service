package north

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	pkgerrors "github.com/kevin07696/payment-service/pkg/errors"
	"github.com/kevin07696/payment-service/test/mocks"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCustomPayTest(t *testing.T, handler http.HandlerFunc) (*CustomPayAdapter, *httptest.Server) {
	server := httptest.NewServer(handler)

	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	// Use mock logger for tests
	logger := mocks.NewMockLogger()

	// Use default HTTP client
	httpClient := &http.Client{}

	adapter := NewCustomPayAdapter(config, server.URL, httpClient, logger)

	return adapter, server
}

func TestCustomPayAdapter_Authorize_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/sale/test-bric-token", r.URL.Path)

		// Verify headers
		assert.NotEmpty(t, r.Header.Get("EPI-Id"))
		assert.NotEmpty(t, r.Header.Get("EPI-Signature"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse request body
		var req SaleRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, 100.00, req.Amount)
		assert.True(t, req.Capture)
		assert.Equal(t, "E", req.IndustryType)
		assert.Equal(t, "Z", req.CardEntryMethod)

		// Send success response
		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{
				Response: "00",
				Text:     "APPROVAL",
				AuthCode: "123456",
			},
			Reference: struct {
				BRIC string `json:"bric"`
			}{
				BRIC: "test-bric-token",
			},
			Status: 200,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "test-bric-token",
		Capture:  true,
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-bric-token", result.GatewayTransactionID)
	assert.Equal(t, models.StatusCaptured, result.Status)
	assert.Equal(t, "00", result.ResponseCode)
	assert.Equal(t, "APPROVAL", result.Message)
	assert.Equal(t, "123456", result.AuthCode)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(100.00)))
}

func TestCustomPayAdapter_Authorize_AuthOnly(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		var req SaleRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Verify capture is false for auth-only
		assert.False(t, req.Capture)

		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{
				Response: "00",
				Text:     "APPROVAL",
				AuthCode: "123456",
			},
			Reference: struct {
				BRIC string `json:"bric"`
			}{
				BRIC: "auth-only-bric",
			},
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(50.00),
		Currency: "USD",
		Token:    "test-bric-token",
		Capture:  false, // Auth only
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, models.StatusAuthorized, result.Status, "Should be authorized, not captured")
}

func TestCustomPayAdapter_Authorize_InsufficientFunds(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{
				Response: "51",
				Text:     "DECLINED - Insufficient Funds",
				AuthCode: "",
			},
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "test-bric-token",
		Capture:  true,
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok, "Error should be a PaymentError")
	assert.Equal(t, "51", paymentErr.Code)
	assert.Equal(t, pkgerrors.CategoryInsufficientFunds, paymentErr.Category)
	assert.True(t, paymentErr.IsRetriable)
}

func TestCustomPayAdapter_Authorize_MissingToken(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not make request with missing token")
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "", // Missing token
		Capture:  true,
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	validationErr, ok := err.(*pkgerrors.ValidationError)
	require.True(t, ok, "Error should be a ValidationError")
	assert.Equal(t, "token", validationErr.Field)
}

func TestCustomPayAdapter_Capture_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/sale/test-transaction-id/capture", r.URL.Path)

		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{
				Response: "00",
				Text:     "CAPTURED",
				AuthCode: "789012",
			},
			Reference: struct {
				BRIC string `json:"bric"`
			}{
				BRIC: "captured-bric",
			},
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	req := &ports.CaptureRequest{
		TransactionID: "test-transaction-id",
		Amount:        decimal.NewFromFloat(75.00),
	}

	result, err := adapter.Capture(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-transaction-id", result.TransactionID)
	assert.Equal(t, models.StatusCaptured, result.Status)
	assert.Equal(t, "00", result.ResponseCode)
}

func TestCustomPayAdapter_Void_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/void/test-transaction-id", r.URL.Path)

		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{
				Response: "00",
				Text:     "VOIDED",
			},
			Reference: struct {
				BRIC string `json:"bric"`
			}{
				BRIC: "voided-bric",
			},
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	result, err := adapter.Void(context.Background(), "test-transaction-id")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-transaction-id", result.TransactionID)
	assert.Equal(t, models.StatusVoided, result.Status)
	assert.Equal(t, "00", result.ResponseCode)
}

func TestCustomPayAdapter_Refund_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/refund/test-transaction-id", r.URL.Path)

		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{
				Response: "00",
				Text:     "REFUNDED",
			},
			Reference: struct {
				BRIC string `json:"bric"`
			}{
				BRIC: "refund-bric",
			},
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	req := &ports.RefundRequest{
		TransactionID: "test-transaction-id",
		Amount:        decimal.NewFromFloat(50.00),
		Reason:        "Customer request",
	}

	result, err := adapter.Refund(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-transaction-id", result.TransactionID)
	assert.Equal(t, models.StatusRefunded, result.Status)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(50.00)))
}

func TestCustomPayAdapter_VerifyAccount_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/avs", r.URL.Path)

		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{
				Response: "00",
				Text:     "VERIFIED",
			},
		}

		json.NewEncoder(w).Encode(resp)
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	req := &ports.VerifyAccountRequest{
		Token: "test-bric-token",
	}

	result, err := adapter.VerifyAccount(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Verified)
	assert.Equal(t, "00", result.ResponseCode)
}

func TestCustomPayAdapter_NetworkError(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}
	// Use invalid URL to trigger network error
	adapter := NewCustomPayAdapter(config, "http://invalid-url-that-does-not-exist:9999", httpClient, logger)

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "test-bric-token",
		Capture:  true,
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok)
	assert.Equal(t, pkgerrors.CategoryNetworkError, paymentErr.Category)
	assert.True(t, paymentErr.IsRetriable)
}

func TestCustomPayAdapter_GatewayError_5xx(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "test-bric-token",
		Capture:  true,
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok)
	assert.Equal(t, "GATEWAY_ERROR", paymentErr.Code)
	assert.Equal(t, pkgerrors.CategorySystemError, paymentErr.Category)
	assert.True(t, paymentErr.IsRetriable)
}

func TestCustomPayAdapter_BadRequest_4xx(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}

	adapter, server := setupCustomPayTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "test-bric-token",
		Capture:  true,
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok)
	assert.Equal(t, "REQUEST_ERROR", paymentErr.Code)
	assert.Equal(t, pkgerrors.CategoryInvalidRequest, paymentErr.Category)
	assert.False(t, paymentErr.IsRetriable)
}
