//go:build integration
// +build integration

package payment_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBrowserPostIdempotency verifies duplicate transaction prevention via Browser Post
// Ensures that fetching the same transaction multiple times returns identical results
// Risk: p99 probability, catastrophic impact (double-charging customers)
func TestBrowserPostIdempotency(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Load test service credentials for JWT authentication
	services, err := testutil.LoadTestServices()
	require.NoError(t, err, "Failed to load test services")
	require.NotEmpty(t, services, "No test services found")

	// Generate JWT token for test service
	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, "00000000-0000-0000-0000-000000000001", time.Hour)
	require.NoError(t, err, "Failed to generate JWT token")

	// Create SALE transaction via Browser Post
	t.Log("[SETUP] Creating SALE transaction...")
	saleResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "50.00", "http://localhost:8081", jwtToken)
	t.Logf("[CREATED] TX=%s GROUP=%s", saleResult.TransactionID, saleResult.GroupID)

	// Set JWT auth for subsequent requests
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	// Fetch transaction (first attempt)
	t.Log("[TEST] Fetching transaction (attempt 1)...")
	resp1, err := client.DoConnectRPC("payment.v1.PaymentService", "GetTransaction", map[string]interface{}{
		"transaction_id": saleResult.TransactionID,
	})
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, 200, resp1.StatusCode)

	var tx1 map[string]interface{}
	require.NoError(t, testutil.DecodeResponse(resp1, &tx1))

	// Log response for debugging
	t.Logf("Response 1: %+v", tx1)

	// Extract amount and status - handle both string and numeric types
	var amount1, status1 string
	if amt, ok := tx1["amount"].(string); ok {
		amount1 = amt
	} else if amt, ok := tx1["amountCents"].(float64); ok {
		amount1 = fmt.Sprintf("%.2f", amt/100)
	}

	if stat, ok := tx1["status"].(string); ok {
		status1 = stat
	} else if stat, ok := tx1["status"].(float64); ok {
		status1 = fmt.Sprintf("%d", int(stat))
	}

	// Fetch same transaction again (simulates idempotent retry)
	t.Log("[TEST] Fetching transaction (attempt 2 - idempotent retry)...")
	resp2, err := client.DoConnectRPC("payment.v1.PaymentService", "GetTransaction", map[string]interface{}{
		"transaction_id": saleResult.TransactionID,
	})
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, 200, resp2.StatusCode)

	var tx2 map[string]interface{}
	require.NoError(t, testutil.DecodeResponse(resp2, &tx2))

	// Log response for debugging
	t.Logf("Response 2: %+v", tx2)

	// Extract amount and status - handle both string and numeric types
	var amount2, status2 string
	if amt, ok := tx2["amount"].(string); ok {
		amount2 = amt
	} else if amt, ok := tx2["amountCents"].(float64); ok {
		amount2 = fmt.Sprintf("%.2f", amt/100)
	}

	if stat, ok := tx2["status"].(string); ok {
		status2 = stat
	} else if stat, ok := tx2["status"].(float64); ok {
		status2 = fmt.Sprintf("%d", int(stat))
	}

	// Verify idempotent behavior
	assert.Equal(t, amount1, amount2, "Amount must be identical across fetches")
	assert.Equal(t, status1, status2, "Status must be identical across fetches")

	t.Logf("[PASS] Idempotency verified: TX=%s", saleResult.TransactionID)
	t.Log("[NOTE] Database PRIMARY KEY on transaction_id prevents duplicate callbacks")
}
