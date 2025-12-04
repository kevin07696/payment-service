//go:build integration
// +build integration

package business_reporting

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/epx"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestBusinessReporting_GetTransaction tests querying a real transaction from EPX
func TestBusinessReporting_GetTransaction(t *testing.T) {
	// Skip if no EPX credentials configured
	apiKey := os.Getenv("EPX_API_KEY")
	if apiKey == "" {
		t.Skip("EPX_API_KEY not set - skipping integration test")
	}

	// Create adapter with sandbox credentials
	config := &epx.BusinessReportingConfig{
		BaseURL:   os.Getenv("EPX_REPORTING_BASE_URL"),
		APIKey:    apiKey,
		APISecret: os.Getenv("EPX_API_SECRET"),
		CustNbr:   os.Getenv("EPX_CUST_NBR"),
		MerchNbr:  os.Getenv("EPX_MERCH_NBR"),
		DBAnbr:    os.Getenv("EPX_DBA_NBR"),
		Timeout:   30 * time.Second,
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api-sandbox.north.com/reporting/v1"
	}

	adapter := epx.NewBusinessReportingAdapter(config, zap.NewNop())

	// Test: Query a known transaction
	// NOTE: Replace with actual AUTH_GUID from a test transaction
	testAuthGUID := os.Getenv("TEST_AUTH_GUID")
	if testAuthGUID == "" {
		t.Skip("TEST_AUTH_GUID not set - skipping transaction query test")
	}

	ctx := context.Background()
	txn, err := adapter.GetTransaction(ctx, testAuthGUID)

	require.NoError(t, err)
	require.NotNil(t, txn)

	// Verify transaction details
	assert.Equal(t, testAuthGUID, txn.AuthGUID)
	assert.NotEmpty(t, txn.Status)
	assert.NotEmpty(t, txn.AuthResp)

	t.Logf("Transaction retrieved: %s, Status: %s, Type: %s",
		txn.AuthGUID, txn.Status, txn.TranType)
}

// TestBusinessReporting_CheckACHReturns tests checking for ACH returns
func TestBusinessReporting_CheckACHReturns(t *testing.T) {
	apiKey := os.Getenv("EPX_API_KEY")
	if apiKey == "" {
		t.Skip("EPX_API_KEY not set - skipping integration test")
	}

	config := &epx.BusinessReportingConfig{
		BaseURL:   os.Getenv("EPX_REPORTING_BASE_URL"),
		APIKey:    apiKey,
		APISecret: os.Getenv("EPX_API_SECRET"),
		Timeout:   30 * time.Second,
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api-sandbox.north.com/reporting/v1"
	}

	adapter := epx.NewBusinessReportingAdapter(config, zap.NewNop())

	tests := []struct {
		name       string
		authGUID   string
		wantReturn bool
	}{
		{
			name:       "approved transaction - no return",
			authGUID:   os.Getenv("TEST_APPROVED_GUID"),
			wantReturn: false,
		},
		{
			name:       "returned transaction - has return",
			authGUID:   os.Getenv("TEST_RETURNED_GUID"),
			wantReturn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.authGUID == "" {
				t.Skipf("Required ENV var not set for test: %s", tt.name)
			}

			ctx := context.Background()
			hasReturn, returnCode, returnReason, err := adapter.CheckACHReturns(ctx, tt.authGUID)

			require.NoError(t, err)
			assert.Equal(t, tt.wantReturn, hasReturn)

			if hasReturn {
				assert.NotEmpty(t, returnCode, "Return code should not be empty")
				assert.NotEmpty(t, returnReason, "Return reason should not be empty")
				t.Logf("ACH Return found: Code=%s, Reason=%s", returnCode, returnReason)
			} else {
				assert.Empty(t, returnCode, "Return code should be empty for approved transactions")
			}
		})
	}
}

// TestBusinessReporting_QueryTransactions tests querying multiple transactions
func TestBusinessReporting_QueryTransactions(t *testing.T) {
	apiKey := os.Getenv("EPX_API_KEY")
	if apiKey == "" {
		t.Skip("EPX_API_KEY not set - skipping integration test")
	}

	config := &epx.BusinessReportingConfig{
		BaseURL:   os.Getenv("EPX_REPORTING_BASE_URL"),
		APIKey:    apiKey,
		APISecret: os.Getenv("EPX_API_SECRET"),
		CustNbr:   os.Getenv("EPX_CUST_NBR"),
		MerchNbr:  os.Getenv("EPX_MERCH_NBR"),
		DBAnbr:    os.Getenv("EPX_DBA_NBR"),
		Timeout:   30 * time.Second,
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api-sandbox.north.com/reporting/v1"
	}

	adapter := epx.NewBusinessReportingAdapter(config, zap.NewNop())

	// Test: Query all ACH returns from last 30 days
	startDate := time.Now().Add(-30 * 24 * time.Hour)
	endDate := time.Now()

	params := &ports.TransactionQueryParams{
		StartDate:      &startDate,
		EndDate:        &endDate,
		ACHReturnsOnly: true,
		CustNbr:        config.CustNbr,
		MerchNbr:       config.MerchNbr,
		DBAnbr:         config.DBAnbr,
		Limit:          100,
	}

	ctx := context.Background()
	result, err := adapter.QueryTransactions(ctx, params)

	require.NoError(t, err)
	require.NotNil(t, result)

	t.Logf("Found %d ACH returns out of %d total transactions",
		len(result.Transactions), result.TotalCount)

	// Verify all returned transactions are ACH returns
	for _, txn := range result.Transactions {
		assert.True(t, txn.IsACHReturn, "All transactions should be ACH returns")
		assert.NotEmpty(t, txn.ACHReturnCode, "ACH return code should not be empty")
		t.Logf("Return: %s - Code: %s, Reason: %s",
			txn.AuthGUID, txn.ACHReturnCode, txn.ACHReturnReason)
	}
}

// TestBusinessReporting_GetACHReturnsForDateRange tests getting ACH returns in a date range
func TestBusinessReporting_GetACHReturnsForDateRange(t *testing.T) {
	apiKey := os.Getenv("EPX_API_KEY")
	if apiKey == "" {
		t.Skip("EPX_API_KEY not set - skipping integration test")
	}

	config := &epx.BusinessReportingConfig{
		BaseURL:   os.Getenv("EPX_REPORTING_BASE_URL"),
		APIKey:    apiKey,
		APISecret: os.Getenv("EPX_API_SECRET"),
		CustNbr:   os.Getenv("EPX_CUST_NBR"),
		MerchNbr:  os.Getenv("EPX_MERCH_NBR"),
		DBAnbr:    os.Getenv("EPX_DBA_NBR"),
		Timeout:   30 * time.Second,
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api-sandbox.north.com/reporting/v1"
	}

	adapter := epx.NewBusinessReportingAdapter(config, zap.NewNop())

	// Get returns from last 7 days
	startDate := time.Now().Add(-7 * 24 * time.Hour)
	endDate := time.Now()

	ctx := context.Background()
	returns, err := adapter.GetACHReturnsForDateRange(ctx, startDate, endDate)

	require.NoError(t, err)
	require.NotNil(t, returns)

	t.Logf("Found %d ACH returns in the last 7 days", len(returns))

	// Group returns by return code
	returnCodes := make(map[string]int)
	for _, txn := range returns {
		assert.True(t, txn.IsACHReturn)
		returnCodes[txn.ACHReturnCode]++
	}

	for code, count := range returnCodes {
		t.Logf("Return Code %s: %d occurrences", code, count)
	}
}

// TestBusinessReporting_ErrorHandling tests error scenarios
func TestBusinessReporting_ErrorHandling(t *testing.T) {
	apiKey := os.Getenv("EPX_API_KEY")
	if apiKey == "" {
		t.Skip("EPX_API_KEY not set - skipping integration test")
	}

	config := &epx.BusinessReportingConfig{
		BaseURL:   os.Getenv("EPX_REPORTING_BASE_URL"),
		APIKey:    apiKey,
		APISecret: os.Getenv("EPX_API_SECRET"),
		Timeout:   30 * time.Second,
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api-sandbox.north.com/reporting/v1"
	}

	adapter := epx.NewBusinessReportingAdapter(config, zap.NewNop())

	t.Run("nonexistent transaction", func(t *testing.T) {
		ctx := context.Background()
		_, err := adapter.GetTransaction(ctx, "NONEXISTENT_GUID_12345")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("invalid date range", func(t *testing.T) {
		// End date before start date
		startDate := time.Now()
		endDate := time.Now().Add(-7 * 24 * time.Hour)

		ctx := context.Background()
		_, err := adapter.GetACHReturnsForDateRange(ctx, startDate, endDate)

		// May or may not error depending on API implementation
		// Just verify it doesn't panic
		t.Logf("Invalid date range error (if any): %v", err)
	})

	t.Run("timeout handling", func(t *testing.T) {
		// Create adapter with very short timeout
		shortTimeoutConfig := *config
		shortTimeoutConfig.Timeout = 1 * time.Millisecond

		shortAdapter := epx.NewBusinessReportingAdapter(&shortTimeoutConfig, zap.NewNop())

		ctx := context.Background()
		_, err := shortAdapter.GetTransaction(ctx, "ANY_GUID")

		// Should timeout
		require.Error(t, err)
		t.Logf("Timeout error: %v", err)
	})
}
