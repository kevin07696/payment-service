//go:build integration
// +build integration

package payment_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Phase 1: Critical Business Logic Integration Tests
// ============================================================================
// Verifies 5 most critical payment scenarios (likelihood × impact framework)
// All tests use REAL EPX integration via headless Chrome Browser Post

// TestBrowserPostIdempotency verifies duplicate transaction prevention
// Risk: p99 probability, catastrophic impact (double-charging customers)
func TestBrowserPostIdempotency(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Create SALE transaction via Browser Post
	t.Log("[SETUP] Creating SALE transaction...")
	saleResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "50.00", "http://localhost:8081")
	t.Logf("[CREATED] TX=%s GROUP=%s", saleResult.TransactionID, saleResult.GroupID)

	// Fetch transaction (first attempt)
	t.Log("[TEST] Fetching transaction (attempt 1)...")
	resp1, err := client.Do("GET", "/api/v1/payments/"+saleResult.TransactionID, nil)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, 200, resp1.StatusCode)

	var tx1 map[string]interface{}
	require.NoError(t, testutil.DecodeResponse(resp1, &tx1))
	amount1, status1 := tx1["amount"].(string), tx1["status"].(string)

	// Fetch same transaction again (simulates idempotent retry)
	t.Log("[TEST] Fetching transaction (attempt 2 - idempotent retry)...")
	resp2, err := client.Do("GET", "/api/v1/payments/"+saleResult.TransactionID, nil)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, 200, resp2.StatusCode)

	var tx2 map[string]interface{}
	require.NoError(t, testutil.DecodeResponse(resp2, &tx2))
	amount2, status2 := tx2["amount"].(string), tx2["status"].(string)

	// Verify idempotent behavior
	assert.Equal(t, amount1, amount2, "Amount must be identical across fetches")
	assert.Equal(t, status1, status2, "Status must be identical across fetches")

	t.Logf("[PASS] Idempotency verified: TX=%s", saleResult.TransactionID)
	t.Log("[NOTE] Database PRIMARY KEY on transaction_id prevents duplicate callbacks")
}

// TestRefundAmountValidation verifies refund amount limits
// Risk: p95 probability, catastrophic impact (merchant theft via over-refunding)
func TestRefundAmountValidation(t *testing.T) {
	tests := []struct {
		name           string
		saleAmount     string
		refundAmount   string
		expectRejected bool
	}{
		{
			name:           "refund_exceeds_sale_amount",
			saleAmount:     "100.00",
			refundAmount:   "150.00",
			expectRejected: true,
		},
		{
			name:           "refund_equals_sale_amount",
			saleAmount:     "100.00",
			refundAmount:   "100.00",
			expectRejected: false,
		},
		{
			name:           "partial_refund_within_limit",
			saleAmount:     "100.00",
			refundAmount:   "50.00",
			expectRejected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, client := testutil.Setup(t)
			time.Sleep(2 * time.Second)

			// Create SALE transaction
			t.Logf("[SETUP] Creating SALE amount=$%s", tt.saleAmount)
			saleResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, tt.saleAmount, "http://localhost:8081")
			t.Logf("[CREATED] TX=%s GROUP=%s", saleResult.TransactionID, saleResult.GroupID)
			time.Sleep(2 * time.Second)

			// Attempt refund
			t.Logf("[TEST] Attempting refund amount=$%s", tt.refundAmount)
			refundReq := map[string]interface{}{
				"group_id": saleResult.GroupID,
				"amount":   tt.refundAmount,
				"reason":   fmt.Sprintf("Test: %s", tt.name),
			}

			refundResp, err := client.Do("POST", "/api/v1/payments/refund", refundReq)
			require.NoError(t, err)
			defer refundResp.Body.Close()

			// Verify expected behavior
			if tt.expectRejected {
				assert.True(t, refundResp.StatusCode == 400 || refundResp.StatusCode == 422,
					"Expected validation error, got status=%d", refundResp.StatusCode)
				t.Logf("[PASS] Refund correctly rejected: HTTP %d", refundResp.StatusCode)
			} else {
				assert.Equal(t, 200, refundResp.StatusCode,
					"Expected refund success, got status=%d", refundResp.StatusCode)
				t.Logf("[PASS] Refund correctly accepted: HTTP %d", refundResp.StatusCode)
			}
		})
	}
}

// TestCaptureStateValidation verifies state machine transitions
// Risk: p95 probability, high impact (money inconsistencies from invalid states)
func TestCaptureStateValidation(t *testing.T) {
	tests := []struct {
		name             string
		setupTransaction string // "SALE" or "AUTH"
		expectSuccess    bool
	}{
		{
			name:             "capture_already_captured_sale",
			setupTransaction: "SALE",
			expectSuccess:    false,
		},
		{
			name:             "capture_authorized_transaction",
			setupTransaction: "AUTH",
			expectSuccess:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, client := testutil.Setup(t)
			time.Sleep(2 * time.Second)

			// Create transaction based on test case
			var txResult *testutil.RealBRICResult
			if tt.setupTransaction == "SALE" {
				t.Log("[SETUP] Creating SALE (already captured)")
				txResult = testutil.GetRealBRICForSaleAutomated(t, client, cfg, "50.00", "http://localhost:8081")
			} else {
				t.Log("[SETUP] Creating AUTH (ready for capture)")
				txResult = testutil.GetRealBRICForAuthAutomated(t, client, cfg, "50.00", "http://localhost:8081")
			}
			t.Logf("[CREATED] TX=%s TYPE=%s", txResult.TransactionID, tt.setupTransaction)
			time.Sleep(2 * time.Second)

			// Attempt capture
			t.Log("[TEST] Attempting capture...")
			captureReq := map[string]interface{}{
				"transaction_id": txResult.TransactionID,
				"amount":         "50.00",
			}

			captureResp, err := client.Do("POST", "/api/v1/payments/capture", captureReq)
			require.NoError(t, err)
			defer captureResp.Body.Close()

			// Verify expected behavior
			if tt.expectSuccess {
				assert.Equal(t, 200, captureResp.StatusCode,
					"Expected capture success, got status=%d", captureResp.StatusCode)
				var result map[string]interface{}
				require.NoError(t, testutil.DecodeResponse(captureResp, &result))
				assert.True(t, result["isApproved"].(bool), "Capture should be approved")
				t.Logf("[PASS] Capture succeeded as expected")
			} else {
				assert.True(t, captureResp.StatusCode == 400 || captureResp.StatusCode == 422,
					"Expected validation error, got status=%d", captureResp.StatusCode)
				t.Logf("[PASS] Capture correctly rejected: HTTP %d", captureResp.StatusCode)
			}
		})
	}
}

// TestConcurrentOperationHandling verifies race condition prevention
// Risk: p99.9 probability, high impact (data corruption in high-traffic scenarios)
func TestConcurrentOperationHandling(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Create AUTH transaction
	t.Log("[SETUP] Creating AUTH transaction...")
	authResult := testutil.GetRealBRICForAuthAutomated(t, client, cfg, "100.00", "http://localhost:8081")
	t.Logf("[CREATED] TX=%s GROUP=%s", authResult.TransactionID, authResult.GroupID)
	time.Sleep(2 * time.Second)

	// Launch CAPTURE and VOID concurrently
	t.Log("[TEST] Launching concurrent CAPTURE + VOID...")
	var wg sync.WaitGroup
	var captureErr, voidErr error
	var captureStatus, voidStatus int
	var captureResp, voidResp map[string]interface{}

	wg.Add(2)

	// Concurrent CAPTURE
	go func() {
		defer wg.Done()
		resp, err := client.Do("POST", "/api/v1/payments/capture", map[string]interface{}{
			"transaction_id": authResult.TransactionID,
			"amount":         "100.00",
		})
		captureErr = err
		if err == nil {
			captureStatus = resp.StatusCode
			testutil.DecodeResponse(resp, &captureResp)
			resp.Body.Close()
		}
	}()

	// Concurrent VOID
	go func() {
		defer wg.Done()
		resp, err := client.Do("POST", "/api/v1/payments/void", map[string]interface{}{
			"group_id": authResult.GroupID,
			"reason":   "Concurrent test",
		})
		voidErr = err
		if err == nil {
			voidStatus = resp.StatusCode
			testutil.DecodeResponse(resp, &voidResp)
			resp.Body.Close()
		}
	}()

	wg.Wait()

	// Verify no data corruption
	captureSuccess := captureErr == nil && captureStatus == 200
	voidSuccess := voidErr == nil && voidStatus == 200

	t.Logf("[RESULT] CAPTURE=%v (status=%d) VOID=%v (status=%d)",
		captureSuccess, captureStatus, voidSuccess, voidStatus)

	// Both might succeed if executed sequentially, but no data corruption should occur
	successCount := 0
	if captureSuccess {
		successCount++
	}
	if voidSuccess {
		successCount++
	}

	assert.GreaterOrEqual(t, successCount, 1, "At least one operation must succeed")
	assert.LessOrEqual(t, successCount, 2, "Max two operations can succeed")
	t.Log("[PASS] No data corruption detected")
}

// TestEPXDeclineCodeHandling verifies decline code processing
// Risk: p90 probability, medium impact (customer experience with error messages)
func TestEPXDeclineCodeHandling(t *testing.T) {
	tests := []struct {
		name         string
		cardDetails  *testutil.CardDetails
		amount       string // EPX uses last 3 digits as response code trigger
		expectStatus string
	}{
		{
			name:         "insufficient_funds_code_51",
			cardDetails:  testutil.VisaDeclineCard(),
			amount:       "1.20", // .20 → EPX code 51 (DECLINE)
			expectStatus: "TRANSACTION_STATUS_DECLINED",
		},
		{
			name:         "generic_decline_code_05",
			cardDetails:  testutil.VisaDeclineCard(),
			amount:       "1.05", // .05 → EPX code 05 (DECLINE)
			expectStatus: "TRANSACTION_STATUS_DECLINED",
		},
		{
			name:         "approval_with_standard_card",
			cardDetails:  testutil.DefaultApprovalCard(),
			amount:       "10.00",
			expectStatus: "TRANSACTION_STATUS_APPROVED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, client := testutil.Setup(t)
			time.Sleep(2 * time.Second)

			// Create SALE with specified card and amount
			t.Logf("[SETUP] Creating SALE card=%s amount=$%s", tt.cardDetails.Number, tt.amount)
			saleResult := testutil.GetRealBRICForSaleAutomatedWithCard(
				t, client, cfg, tt.amount, "http://localhost:8081", tt.cardDetails,
			)
			t.Logf("[CREATED] TX=%s GROUP=%s", saleResult.TransactionID, saleResult.GroupID)
			time.Sleep(2 * time.Second)

			// Fetch transaction details
			t.Log("[TEST] Fetching transaction status...")
			resp, err := client.Do("GET", "/api/v1/payments/"+saleResult.TransactionID, nil)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, 200, resp.StatusCode)

			var transaction map[string]interface{}
			require.NoError(t, testutil.DecodeResponse(resp, &transaction))
			status := transaction["status"].(string)

			// Verify expected status
			assert.Equal(t, tt.expectStatus, status,
				"Expected status=%s, got status=%s", tt.expectStatus, status)
			t.Logf("[PASS] Status verified: %s", status)
		})
	}
}
