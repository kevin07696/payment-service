package north

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/kevin07696/payment-service/internal/domain/ports"
	pkgerrors "github.com/kevin07696/payment-service/pkg/errors"
	"github.com/kevin07696/payment-service/test/mocks"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDependencyInjection_HTTPClientMock demonstrates that we can inject a mock HTTP client
// This allows us to test without any real HTTP server and control responses precisely
func TestDependencyInjection_HTTPClientMock(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()

	// Create a mock HTTP client that returns a specific response
	mockHTTPClient := mocks.NewMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		// Verify HMAC signature was added
		assert.NotEmpty(t, req.Header.Get("EPI-Id"))
		assert.NotEmpty(t, req.Header.Get("EPI-Signature"))

		// Return success response
		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{
				Response: "00",
				Text:     "MOCKED APPROVAL",
				AuthCode: "999999",
			},
			Reference: struct {
				BRIC string `json:"bric"`
			}{
				BRIC: "mocked-bric-token",
			},
		}

		body, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	// Inject the mock HTTP client
	adapter := NewCustomPayAdapter(config, "http://fake-url.com", mockHTTPClient, logger)

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "test-token",
		Capture:  true,
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "MOCKED APPROVAL", result.Message)
	assert.Equal(t, "999999", result.AuthCode)

	// Verify HTTP client was called exactly once
	assert.Len(t, mockHTTPClient.Calls, 1)

	// Verify logger captured the request
	assert.Len(t, logger.InfoCalls, 1)
	assert.Equal(t, "making request to North Custom Pay", logger.InfoCalls[0].Message)
}

// TestDependencyInjection_LoggerMock demonstrates logger interchangeability
func TestDependencyInjection_LoggerMock(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	mockLogger := mocks.NewMockLogger()

	mockHTTPClient := mocks.NewMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{Response: "00", Text: "OK", AuthCode: "123"},
			Reference: struct {
				BRIC string `json:"bric"`
			}{BRIC: "token"},
		}
		body, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	adapter := NewCustomPayAdapter(config, "http://test.com", mockHTTPClient, mockLogger)

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(50.00),
		Token:    "token",
		Capture:  true,
	}

	_, err := adapter.Authorize(context.Background(), req)
	require.NoError(t, err)

	// Assert logger was called with expected message
	require.Len(t, mockLogger.InfoCalls, 1)
	call := mockLogger.InfoCalls[0]
	assert.Equal(t, "making request to North Custom Pay", call.Message)

	// Assert fields were logged
	assert.Len(t, call.Fields, 2)
	assert.Equal(t, "method", call.Fields[0].Key)
	assert.Equal(t, "POST", call.Fields[0].Value)
	assert.Equal(t, "endpoint", call.Fields[1].Key)
	assert.Equal(t, "/sale/token", call.Fields[1].Value)
}

// TestDependencyInjection_HTTPClientNetworkError demonstrates error handling with mocked client
func TestDependencyInjection_HTTPClientNetworkError(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()

	// Create a mock HTTP client that simulates a network error
	mockHTTPClient := mocks.NewMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		return nil, &mockNetworkError{msg: "connection timeout"}
	})

	adapter := NewCustomPayAdapter(config, "http://test.com", mockHTTPClient, logger)

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "test-token",
		Capture:  true,
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok, "Error should be PaymentError")
	assert.Equal(t, "NETWORK_ERROR", paymentErr.Code)
	assert.Equal(t, pkgerrors.CategoryNetworkError, paymentErr.Category)
	assert.True(t, paymentErr.IsRetriable, "Network errors should be retriable")
}

// TestDependencyInjection_MultipleAdaptersWithDifferentLoggers demonstrates
// that we can create multiple adapter instances with different logger implementations
func TestDependencyInjection_MultipleAdaptersWithDifferentLoggers(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	// Create two different logger instances
	logger1 := mocks.NewMockLogger()
	logger2 := mocks.NewMockLogger()

	mockHTTP := mocks.NewMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		resp := SaleResponse{
			Data: struct {
				Response string `json:"response"`
				Text     string `json:"text"`
				AuthCode string `json:"authCode"`
			}{Response: "00"},
			Reference: struct {
				BRIC string `json:"bric"`
			}{BRIC: "token"},
		}
		body, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	})

	// Create two adapters with different loggers
	adapter1 := NewCustomPayAdapter(config, "http://test1.com", mockHTTP, logger1)
	adapter2 := NewCustomPayAdapter(config, "http://test2.com", mockHTTP, logger2)

	req := &ports.AuthorizeRequest{
		Amount:  decimal.NewFromFloat(100.00),
		Token:   "token",
		Capture: true,
	}

	// Call both adapters
	adapter1.Authorize(context.Background(), req)
	adapter2.Authorize(context.Background(), req)

	// Each logger should only have its own call
	assert.Len(t, logger1.InfoCalls, 1, "Logger 1 should have 1 call")
	assert.Len(t, logger2.InfoCalls, 1, "Logger 2 should have 1 call")
}

// mockNetworkError is a simple error type for testing
type mockNetworkError struct {
	msg string
}

func (e *mockNetworkError) Error() string {
	return e.msg
}
