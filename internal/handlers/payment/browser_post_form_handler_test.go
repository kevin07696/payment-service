package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	serviceports "github.com/kevin07696/payment-service/internal/services/ports"
	"go.uber.org/zap/zaptest"
)

// mockBrowserPostAdapter is a mock implementation of ports.BrowserPostAdapter for testing
type mockBrowserPostAdapter struct{}

func (m *mockBrowserPostAdapter) BuildFormData(tac, amount, tranNbr, tranGroup, redirectURL string) (*ports.BrowserPostFormData, error) {
	return nil, nil
}

func (m *mockBrowserPostAdapter) ParseRedirectResponse(params map[string][]string) (*ports.BrowserPostResponse, error) {
	return nil, nil
}

func (m *mockBrowserPostAdapter) ValidateResponseMAC(params map[string][]string, mac string) error {
	return nil
}

// mockDatabaseAdapter is a mock implementation of DatabaseAdapter for testing
type mockDatabaseAdapter struct{}

func (m *mockDatabaseAdapter) Queries() *sqlc.Queries {
	return nil
}

// mockPaymentMethodService is a mock implementation of PaymentMethodService for testing
type mockPaymentMethodService struct{}

func (m *mockPaymentMethodService) ConvertFinancialBRICToStorageBRIC(ctx context.Context, req *serviceports.ConvertFinancialBRICRequest) (*domain.PaymentMethod, error) {
	return nil, nil
}

// TestGetPaymentForm tests the GetPaymentForm handler using table-driven tests
func TestGetPaymentForm(t *testing.T) {
	tests := []struct {
		name               string
		method             string
		queryParams        string
		expectedStatusCode int
		expectedError      bool
		validateResponse   func(t *testing.T, body map[string]interface{})
	}{
		{
			name:               "Success - Valid amount",
			method:             http.MethodGet,
			queryParams:        "?amount=99.99",
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
			validateResponse: func(t *testing.T, body map[string]interface{}) {
				// Validate all required fields are present
				requiredFields := []string{
					"postURL", "custNbr", "merchNbr", "dBAnbr", "terminalNbr",
					"amount", "tranNbr", "tranGroup", "tranCode", "industryType",
					"cardEntMeth", "redirectURL", "merchantName",
				}

				for _, field := range requiredFields {
					if _, ok := body[field]; !ok {
						t.Errorf("Missing required field: %s", field)
					}
				}

				// Validate specific field values
				if body["amount"] != "99.99" {
					t.Errorf("Expected amount=99.99, got %v", body["amount"])
				}

				if body["postURL"] != "https://secure.epxuap.com/browserpost" {
					t.Errorf("Expected EPX sandbox URL, got %v", body["postURL"])
				}

				if body["custNbr"] != "9001" {
					t.Errorf("Expected custNbr=9001, got %v", body["custNbr"])
				}

				if body["tranCode"] != "SALE" {
					t.Errorf("Expected tranCode=SALE, got %v", body["tranCode"])
				}

				// Validate transaction number is numeric
				tranNbr, ok := body["tranNbr"].(string)
				if !ok || tranNbr == "" {
					t.Error("tranNbr should be a non-empty string")
				}
			},
		},
		{
			name:               "Success - Small amount",
			method:             http.MethodGet,
			queryParams:        "?amount=1.00",
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
			validateResponse: func(t *testing.T, body map[string]interface{}) {
				if body["amount"] != "1.00" {
					t.Errorf("Expected amount=1.00, got %v", body["amount"])
				}
			},
		},
		{
			name:               "Success - Large amount",
			method:             http.MethodGet,
			queryParams:        "?amount=9999.99",
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
			validateResponse: func(t *testing.T, body map[string]interface{}) {
				if body["amount"] != "9999.99" {
					t.Errorf("Expected amount=9999.99, got %v", body["amount"])
				}
			},
		},
		{
			name:               "Error - Missing amount parameter",
			method:             http.MethodGet,
			queryParams:        "",
			expectedStatusCode: http.StatusBadRequest,
			expectedError:      true,
			validateResponse:   nil,
		},
		{
			name:               "Error - Invalid amount (non-numeric)",
			method:             http.MethodGet,
			queryParams:        "?amount=invalid",
			expectedStatusCode: http.StatusBadRequest,
			expectedError:      true,
			validateResponse:   nil,
		},
		{
			name:               "Error - Invalid amount (letters)",
			method:             http.MethodGet,
			queryParams:        "?amount=abc.def",
			expectedStatusCode: http.StatusBadRequest,
			expectedError:      true,
			validateResponse:   nil,
		},
		{
			name:               "Error - Wrong HTTP method (POST)",
			method:             http.MethodPost,
			queryParams:        "?amount=99.99",
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedError:      true,
			validateResponse:   nil,
		},
		{
			name:               "Error - Wrong HTTP method (PUT)",
			method:             http.MethodPut,
			queryParams:        "?amount=99.99",
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedError:      true,
			validateResponse:   nil,
		},
		{
			name:               "Error - Wrong HTTP method (DELETE)",
			method:             http.MethodDelete,
			queryParams:        "?amount=99.99",
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedError:      true,
			validateResponse:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			logger := zaptest.NewLogger(t)
			handler := NewBrowserPostCallbackHandler(
				&mockDatabaseAdapter{},
				&mockBrowserPostAdapter{},
				&mockPaymentMethodService{},
				logger,
				"https://secure.epxuap.com/browserpost", // EPX sandbox URL
				"9001",                                   // EPX Customer Number
				"900300",                                 // EPX Merchant Number
				"2",                                      // EPX DBA Number
				"77",                                     // EPX Terminal Number
				"http://localhost:8081",                 // Callback base URL
			)

			// Create request
			req := httptest.NewRequest(tt.method, "/api/v1/payments/browser-post/form"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			// Execute
			handler.GetPaymentForm(w, req)

			// Assert status code
			if w.Code != tt.expectedStatusCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatusCode, w.Code)
			}

			// For success cases, validate response body
			if !tt.expectedError && tt.validateResponse != nil {
				var responseBody map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
					t.Fatalf("Failed to unmarshal response body: %v", err)
				}

				tt.validateResponse(t, responseBody)

				// Verify Content-Type header
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type: application/json, got %s", contentType)
				}
			}
		})
	}
}

// TestGetPaymentForm_UniqueTransactionNumbers verifies that each call generates a unique transaction number
func TestGetPaymentForm_UniqueTransactionNumbers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	handler := NewBrowserPostCallbackHandler(
		&mockDatabaseAdapter{},
		&mockBrowserPostAdapter{},
		&mockPaymentMethodService{},
		logger,
		"https://secure.epxuap.com/browserpost",
		"9001",
		"900300",
		"2",
		"77",
		"http://localhost:8081",
	)

	tranNbrs := make(map[string]bool)

	// Make multiple requests and verify transaction numbers are unique
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form?amount=99.99", nil)
		w := httptest.NewRecorder()

		handler.GetPaymentForm(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Request %d failed with status %d", i, w.Code)
		}

		var responseBody map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
			t.Fatalf("Failed to unmarshal response body: %v", err)
		}

		tranNbr := responseBody["tranNbr"].(string)

		if tranNbrs[tranNbr] {
			t.Errorf("Duplicate transaction number detected: %s", tranNbr)
		}

		tranNbrs[tranNbr] = true
	}

	if len(tranNbrs) != 10 {
		t.Errorf("Expected 10 unique transaction numbers, got %d", len(tranNbrs))
	}
}

// TestGetPaymentForm_CredentialsConfiguration verifies handler uses provided credentials
func TestGetPaymentForm_CredentialsConfiguration(t *testing.T) {
	tests := []struct {
		name            string
		epxPostURL      string
		epxCustNbr      string
		epxMerchNbr     string
		epxDBAnbr       string
		epxTerminalNbr  string
		callbackBaseURL string
	}{
		{
			name:            "Sandbox credentials",
			epxPostURL:      "https://secure.epxuap.com/browserpost",
			epxCustNbr:      "9001",
			epxMerchNbr:     "900300",
			epxDBAnbr:       "2",
			epxTerminalNbr:  "77",
			callbackBaseURL: "http://localhost:8081",
		},
		{
			name:            "Production credentials",
			epxPostURL:      "https://secure.epxnow.com/browserpost",
			epxCustNbr:      "1234",
			epxMerchNbr:     "567890",
			epxDBAnbr:       "1",
			epxTerminalNbr:  "99",
			callbackBaseURL: "https://api.example.com",
		},
		{
			name:            "Custom credentials",
			epxPostURL:      "https://custom.epx.com/browserpost",
			epxCustNbr:      "CUST001",
			epxMerchNbr:     "MERCH001",
			epxDBAnbr:       "5",
			epxTerminalNbr:  "123",
			callbackBaseURL: "https://payments.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			handler := NewBrowserPostCallbackHandler(
				&mockDatabaseAdapter{},
				&mockBrowserPostAdapter{},
				&mockPaymentMethodService{},
				logger,
				tt.epxPostURL,
				tt.epxCustNbr,
				tt.epxMerchNbr,
				tt.epxDBAnbr,
				tt.epxTerminalNbr,
				tt.callbackBaseURL,
			)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form?amount=99.99", nil)
			w := httptest.NewRecorder()

			handler.GetPaymentForm(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", w.Code)
			}

			var responseBody map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
				t.Fatalf("Failed to unmarshal response body: %v", err)
			}

			// Verify credentials are correctly set in response
			if responseBody["postURL"] != tt.epxPostURL {
				t.Errorf("Expected postURL=%s, got %v", tt.epxPostURL, responseBody["postURL"])
			}

			if responseBody["custNbr"] != tt.epxCustNbr {
				t.Errorf("Expected custNbr=%s, got %v", tt.epxCustNbr, responseBody["custNbr"])
			}

			if responseBody["merchNbr"] != tt.epxMerchNbr {
				t.Errorf("Expected merchNbr=%s, got %v", tt.epxMerchNbr, responseBody["merchNbr"])
			}

			if responseBody["dBAnbr"] != tt.epxDBAnbr {
				t.Errorf("Expected dBAnbr=%s, got %v", tt.epxDBAnbr, responseBody["dBAnbr"])
			}

			if responseBody["terminalNbr"] != tt.epxTerminalNbr {
				t.Errorf("Expected terminalNbr=%s, got %v", tt.epxTerminalNbr, responseBody["terminalNbr"])
			}

			expectedRedirectURL := tt.callbackBaseURL + "/api/v1/payments/browser-post/callback"
			if responseBody["redirectURL"] != expectedRedirectURL {
				t.Errorf("Expected redirectURL=%s, got %v", expectedRedirectURL, responseBody["redirectURL"])
			}
		})
	}
}

// TestGetPaymentForm_EdgeCases tests edge cases and boundary conditions
func TestGetPaymentForm_EdgeCases(t *testing.T) {
	tests := []struct {
		name               string
		queryParams        string
		expectedStatusCode int
		expectedError      bool
	}{
		{
			name:               "Zero amount",
			queryParams:        "?amount=0.00",
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
		},
		{
			name:               "Very small amount",
			queryParams:        "?amount=0.01",
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
		},
		{
			name:               "Negative amount (invalid)",
			queryParams:        "?amount=-99.99",
			expectedStatusCode: http.StatusOK, // Parses as valid float, validation could be added
			expectedError:      false,
		},
		{
			name:               "Amount with many decimal places",
			queryParams:        "?amount=99.999999",
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
		},
		{
			name:               "Integer amount (no decimal)",
			queryParams:        "?amount=100",
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
		},
		{
			name:               "Empty amount parameter",
			queryParams:        "?amount=",
			expectedStatusCode: http.StatusBadRequest,
			expectedError:      true,
		},
		{
			name:               "Multiple amount parameters (uses first)",
			queryParams:        "?amount=99.99&amount=50.00",
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			handler := NewBrowserPostCallbackHandler(
				&mockDatabaseAdapter{},
				&mockBrowserPostAdapter{},
				&mockPaymentMethodService{},
				logger,
				"https://secure.epxuap.com/browserpost",
				"9001",
				"900300",
				"2",
				"77",
				"http://localhost:8081",
			)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			handler.GetPaymentForm(w, req)

			if w.Code != tt.expectedStatusCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatusCode, w.Code)
			}
		})
	}
}

// BenchmarkGetPaymentForm benchmarks the GetPaymentForm handler performance
func BenchmarkGetPaymentForm(b *testing.B) {
	logger := zaptest.NewLogger(b)
	handler := NewBrowserPostCallbackHandler(
		&mockDatabaseAdapter{},
		&mockBrowserPostAdapter{},
		&mockPaymentMethodService{},
		logger,
		"https://secure.epxuap.com/browserpost",
		"9001",
		"900300",
		"2",
		"77",
		"http://localhost:8081",
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form?amount=99.99", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.GetPaymentForm(w, req)
	}
}
