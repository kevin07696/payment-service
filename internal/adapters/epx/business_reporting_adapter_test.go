package epx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBusinessReportingAdapter_GetTransaction(t *testing.T) {
	tests := []struct {
		name           string
		authGUID       string
		mockResponse   *apiTransactionResponse
		mockStatusCode int
		wantErr        bool
		errContains    string
		validate       func(t *testing.T, txn *ports.TransactionDetails)
	}{
		{
			name:     "successful transaction query - approved ACH",
			authGUID: "TEST_GUID_001",
			mockResponse: &apiTransactionResponse{
				AuthGUID:         "TEST_GUID_001",
				TranNbr:          "12345",
				TranType:         "CKC0",
				Status:           "approved",
				AuthResp:         "00",
				AuthRespText:     "APPROVAL",
				Amount:           "0.00",
				CurrencyCode:     "USD",
				TransactionDate:  time.Now().Format(time.RFC3339),
				PaymentMethod:    "ach_checking",
				MaskedAccountNbr: "****1234",
				CustNbr:          "1234",
				MerchNbr:         "123456",
				DBAnbr:           "1",
				TerminalNbr:      "1",
				BatchID:          "20250124",
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validate: func(t *testing.T, txn *ports.TransactionDetails) {
				assert.Equal(t, "TEST_GUID_001", txn.AuthGUID)
				assert.Equal(t, "CKC0", txn.TranType)
				assert.Equal(t, ports.TransactionStatusApproved, txn.Status)
				assert.Equal(t, "00", txn.AuthResp)
				assert.False(t, txn.IsACHReturn)
				assert.Equal(t, "ach_checking", txn.PaymentMethod)
			},
		},
		{
			name:     "successful query - ACH return R02",
			authGUID: "TEST_GUID_002",
			mockResponse: &apiTransactionResponse{
				AuthGUID:         "TEST_GUID_002",
				TranNbr:          "12346",
				TranType:         "CKC0",
				Status:           "returned",
				AuthResp:         "05",
				AuthRespText:     "ACH RETURN",
				Amount:           "0.00",
				CurrencyCode:     "USD",
				TransactionDate:  time.Now().Add(-3 * 24 * time.Hour).Format(time.RFC3339),
				PaymentMethod:    "ach_checking",
				MaskedAccountNbr: "****5678",
				ACHReturn: &apiACHReturn{
					ReturnCode:       "R02",
					ReturnReason:     "Account Closed",
					ReturnDate:       time.Now().Format(time.RFC3339),
					OriginalAuthGUID: "TEST_GUID_002",
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validate: func(t *testing.T, txn *ports.TransactionDetails) {
				assert.Equal(t, "TEST_GUID_002", txn.AuthGUID)
				assert.Equal(t, ports.TransactionStatusReturned, txn.Status)
				assert.True(t, txn.IsACHReturn)
				assert.Equal(t, "R02", txn.ACHReturnCode)
				assert.Equal(t, "Account Closed", txn.ACHReturnReason)
				assert.NotNil(t, txn.ACHReturnDate)
			},
		},
		{
			name:           "transaction not found",
			authGUID:       "NONEXISTENT",
			mockStatusCode: http.StatusNotFound,
			wantErr:        true,
			errContains:    "transaction not found",
		},
		{
			name:        "empty auth_guid",
			authGUID:    "",
			wantErr:     true,
			errContains: "authGUID is required",
		},
		{
			name:           "API error 500",
			authGUID:       "TEST_GUID_003",
			mockStatusCode: http.StatusInternalServerError,
			wantErr:        true,
			errContains:    "API error: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify HTTP method
				assert.Equal(t, "GET", r.Method)

				// Verify authentication headers
				assert.NotEmpty(t, r.Header.Get("X-API-Key"))
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				// Return mock response
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Create adapter with mock server URL
			config := &BusinessReportingConfig{
				BaseURL:   server.URL,
				APIKey:    "test_key",
				APISecret: "test_secret",
				Timeout:   5 * time.Second,
			}

			adapter := NewBusinessReportingAdapter(config, zap.NewNop())

			// Execute test
			ctx := context.Background()
			txn, err := adapter.GetTransaction(ctx, tt.authGUID)

			// Verify error expectations
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			// Verify success expectations
			require.NoError(t, err)
			require.NotNil(t, txn)

			if tt.validate != nil {
				tt.validate(t, txn)
			}
		})
	}
}

func TestBusinessReportingAdapter_CheckACHReturns(t *testing.T) {
	tests := []struct {
		name             string
		authGUID         string
		mockResponse     *apiTransactionResponse
		wantHasReturn    bool
		wantReturnCode   string
		wantReturnReason string
		wantErr          bool
	}{
		{
			name:     "no ACH return - approved transaction",
			authGUID: "APPROVED_001",
			mockResponse: &apiTransactionResponse{
				AuthGUID:      "APPROVED_001",
				Status:        "approved",
				AuthResp:      "00",
				PaymentMethod: "ach_checking",
			},
			wantHasReturn:  false,
			wantReturnCode: "",
		},
		{
			name:     "has ACH return - R02 account closed",
			authGUID: "RETURNED_001",
			mockResponse: &apiTransactionResponse{
				AuthGUID:      "RETURNED_001",
				Status:        "returned",
				AuthResp:      "05",
				PaymentMethod: "ach_checking",
				ACHReturn: &apiACHReturn{
					ReturnCode:   "R02",
					ReturnReason: "Account Closed",
					ReturnDate:   time.Now().Format(time.RFC3339),
				},
			},
			wantHasReturn:    true,
			wantReturnCode:   "R02",
			wantReturnReason: "Account Closed",
		},
		{
			name:     "has ACH return - R03 no account",
			authGUID: "RETURNED_002",
			mockResponse: &apiTransactionResponse{
				AuthGUID:      "RETURNED_002",
				Status:        "returned",
				AuthResp:      "05",
				PaymentMethod: "ach_checking",
				ACHReturn: &apiACHReturn{
					ReturnCode:   "R03",
					ReturnReason: "No Account/Unable to Locate Account",
					ReturnDate:   time.Now().Format(time.RFC3339),
				},
			},
			wantHasReturn:    true,
			wantReturnCode:   "R03",
			wantReturnReason: "No Account/Unable to Locate Account",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create adapter
			config := &BusinessReportingConfig{
				BaseURL: server.URL,
				APIKey:  "test_key",
				Timeout: 5 * time.Second,
			}

			adapter := NewBusinessReportingAdapter(config, zap.NewNop())

			// Execute test
			ctx := context.Background()
			hasReturn, returnCode, returnReason, err := adapter.CheckACHReturns(ctx, tt.authGUID)

			// Verify error expectations
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			// Verify success expectations
			require.NoError(t, err)
			assert.Equal(t, tt.wantHasReturn, hasReturn)
			assert.Equal(t, tt.wantReturnCode, returnCode)
			assert.Equal(t, tt.wantReturnReason, returnReason)
		})
	}
}

func TestBusinessReportingAdapter_QueryTransactions(t *testing.T) {
	now := time.Now()
	startDate := now.Add(-7 * 24 * time.Hour)
	endDate := now

	tests := []struct {
		name         string
		params       *ports.TransactionQueryParams
		mockResponse *apiQueryResponse
		wantErr      bool
		validate     func(t *testing.T, result *ports.TransactionQueryResult)
	}{
		{
			name: "query ACH returns only",
			params: &ports.TransactionQueryParams{
				StartDate:      &startDate,
				EndDate:        &endDate,
				ACHReturnsOnly: true,
				Limit:          10,
			},
			mockResponse: &apiQueryResponse{
				Transactions: []apiTransactionResponse{
					{
						AuthGUID:      "RETURNED_001",
						Status:        "returned",
						PaymentMethod: "ach_checking",
						ACHReturn: &apiACHReturn{
							ReturnCode:   "R02",
							ReturnReason: "Account Closed",
						},
					},
					{
						AuthGUID:      "RETURNED_002",
						Status:        "returned",
						PaymentMethod: "ach_checking",
						ACHReturn: &apiACHReturn{
							ReturnCode:   "R03",
							ReturnReason: "No Account",
						},
					},
				},
				TotalCount: 2,
				HasMore:    false,
			},
			wantErr: false,
			validate: func(t *testing.T, result *ports.TransactionQueryResult) {
				assert.Len(t, result.Transactions, 2)
				assert.Equal(t, 2, result.TotalCount)
				assert.False(t, result.HasMore)

				// Verify all are ACH returns
				for _, txn := range result.Transactions {
					assert.True(t, txn.IsACHReturn)
					assert.NotEmpty(t, txn.ACHReturnCode)
				}
			},
		},
		{
			name: "query with pagination",
			params: &ports.TransactionQueryParams{
				Limit:  10,
				Offset: 20,
			},
			mockResponse: &apiQueryResponse{
				Transactions: []apiTransactionResponse{
					{AuthGUID: "TXN_021"},
					{AuthGUID: "TXN_022"},
				},
				TotalCount: 50,
				HasMore:    true,
			},
			wantErr: false,
			validate: func(t *testing.T, result *ports.TransactionQueryResult) {
				assert.Len(t, result.Transactions, 2)
				assert.Equal(t, 50, result.TotalCount)
				assert.True(t, result.HasMore)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify query parameters
				query := r.URL.Query()

				if tt.params.ACHReturnsOnly {
					assert.Equal(t, "true", query.Get("ach_returns_only"))
				}

				if tt.params.Limit > 0 {
					assert.Equal(t, "10", query.Get("limit"))
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			// Create adapter
			config := &BusinessReportingConfig{
				BaseURL: server.URL,
				APIKey:  "test_key",
				Timeout: 5 * time.Second,
			}

			adapter := NewBusinessReportingAdapter(config, zap.NewNop())

			// Execute test
			ctx := context.Background()
			result, err := adapter.QueryTransactions(ctx, tt.params)

			// Verify error expectations
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			// Verify success expectations
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestBusinessReportingAdapter_MapStatus(t *testing.T) {
	adapter := &businessReportingAdapter{}

	tests := []struct {
		apiStatus string
		want      ports.TransactionStatus
	}{
		{"approved", ports.TransactionStatusApproved},
		{"success", ports.TransactionStatusApproved},
		{"declined", ports.TransactionStatusDeclined},
		{"failed", ports.TransactionStatusDeclined},
		{"pending", ports.TransactionStatusPending},
		{"returned", ports.TransactionStatusReturned},
		{"voided", ports.TransactionStatusVoided},
		{"refunded", ports.TransactionStatusRefunded},
		{"settled", ports.TransactionStatusSettled},
		{"unknown", ports.TransactionStatusPending}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.apiStatus, func(t *testing.T) {
			got := adapter.mapStatus(tt.apiStatus)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBusinessReportingAdapter_MapPaymentMethod(t *testing.T) {
	adapter := &businessReportingAdapter{}

	tests := []struct {
		apiMethod string
		want      string
	}{
		{"credit_card", "credit_card"},
		{"card", "credit_card"},
		{"cc", "credit_card"},
		{"ach_checking", "ach_checking"},
		{"checking", "ach_checking"},
		{"ach", "ach_checking"},
		{"ach_savings", "ach_savings"},
		{"savings", "ach_savings"},
		{"unknown", "unknown"}, // Pass through
	}

	for _, tt := range tests {
		t.Run(tt.apiMethod, func(t *testing.T) {
			got := adapter.mapPaymentMethod(tt.apiMethod)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBusinessReportingAdapter_GetACHReturnsForDateRange(t *testing.T) {
	startDate := time.Now().Add(-7 * 24 * time.Hour)
	endDate := time.Now()

	mockResponse := &apiQueryResponse{
		Transactions: []apiTransactionResponse{
			{
				AuthGUID:      "RETURNED_001",
				Status:        "returned",
				PaymentMethod: "ach_checking",
				ACHReturn: &apiACHReturn{
					ReturnCode:   "R02",
					ReturnReason: "Account Closed",
				},
			},
		},
		TotalCount: 1,
		HasMore:    false,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Verify ACH returns only filter is set
		assert.Equal(t, "true", query.Get("ach_returns_only"))

		// Verify date range
		assert.NotEmpty(t, query.Get("start_date"))
		assert.NotEmpty(t, query.Get("end_date"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	config := &BusinessReportingConfig{
		BaseURL:  server.URL,
		APIKey:   "test_key",
		CustNbr:  "1234",
		MerchNbr: "123456",
		DBAnbr:   "1",
		Timeout:  5 * time.Second,
	}

	adapter := NewBusinessReportingAdapter(config, zap.NewNop())

	ctx := context.Background()
	returns, err := adapter.GetACHReturnsForDateRange(ctx, startDate, endDate)

	require.NoError(t, err)
	require.Len(t, returns, 1)
	assert.Equal(t, "RETURNED_001", returns[0].AuthGUID)
	assert.True(t, returns[0].IsACHReturn)
	assert.Equal(t, "R02", returns[0].ACHReturnCode)
}
