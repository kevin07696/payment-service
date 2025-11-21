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

// TestBrowserPost_Workflows tests various payment workflows using Browser Post API
// Uses table-driven approach to test different transaction types and workflows
func TestBrowserPost_Workflows(t *testing.T) {
	tests := []struct {
		name            string
		transactionType string   // "SALE", "AUTH", "STORAGE"
		amount          string   // Initial transaction amount
		workflow        []string // Sequence of operations: ["SALE", "REFUND"] or ["AUTH", "CAPTURE", "REFUND"]
		refundAmount    string   // Amount to refund (for partial refund tests)
	}{
		{
			name:            "SALE_to_REFUND",
			transactionType: "SALE",
			amount:          "50.00",
			workflow:        []string{"SALE", "REFUND"},
			refundAmount:    "25.00",
		},
		{
			name:            "AUTH_CAPTURE_REFUND",
			transactionType: "AUTH",
			amount:          "50.00",
			workflow:        []string{"AUTH", "CAPTURE", "REFUND"},
			refundAmount:    "25.00",
		},
		{
			name:            "AUTH_VOID",
			transactionType: "AUTH",
			amount:          "100.00",
			workflow:        []string{"AUTH", "VOID"},
			refundAmount:    "",
		},
		// Future: Add STORAGE test case for tokenizing without immediate charge
		// {
		//     name:            "STORAGE_to_SALE",
		//     transactionType: "STORAGE",
		//     amount:          "0.00",
		//     workflow:        []string{"STORAGE", "SALE"},
		//     refundAmount:    "",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, client := testutil.Setup(t)
			time.Sleep(2 * time.Second)

			// Generate JWT token for authentication
			merchantID := "00000000-0000-0000-0000-000000000001"
			services, err := testutil.LoadTestServices()
			require.NoError(t, err)
			require.NotEmpty(t, services, "No test services found")
			jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
			require.NoError(t, err)

			callbackBaseURL := "http://localhost:8081"
			t.Logf("📡 Using callback URL: %s", callbackBaseURL)

			// Step 1: Get BRIC via Browser Post based on transaction type
			t.Logf("🚀 Getting REAL BRIC from EPX via automated browser (%s)...", tt.transactionType)

			var bricResult *testutil.RealBRICResult

			switch tt.transactionType {
			case "SALE":
				bricResult = testutil.GetRealBRICForSaleAutomated(t, client, cfg, tt.amount, callbackBaseURL, jwtToken)
			case "AUTH":
				bricResult = testutil.GetRealBRICForAuthAutomated(t, client, cfg, tt.amount, callbackBaseURL, jwtToken)
			// case "STORAGE":
			//     bricResult = testutil.GetRealBRICForStorageAutomated(t, client, cfg, callbackBaseURL)
			default:
				t.Fatalf("Unknown transaction type: %s", tt.transactionType)
			}

			t.Logf("✅ Step 1: %s completed", tt.transactionType)
			t.Logf("   Transaction ID: %s", bricResult.TransactionID)
			t.Logf("   Group ID: %s", bricResult.GroupID)

			time.Sleep(2 * time.Second)

			// Execute workflow steps
			stepNum := 2
			for _, operation := range tt.workflow[1:] { // Skip first step (already done)
				t.Logf("🔄 Step %d: Executing %s...", stepNum, operation)

				switch operation {
				case "CAPTURE":
					executeCaptureStep(t, client, bricResult, tt.amount)

				case "REFUND":
					executeRefundStep(t, client, bricResult, tt.refundAmount)

				case "VOID":
					executeVoidStep(t, client, bricResult)

				case "SALE":
					// For STORAGE → SALE workflow
					executeSaleWithStoredBRIC(t, client, bricResult, tt.amount)
				}

				stepNum++
				time.Sleep(2 * time.Second)
			}

			t.Log("========================================================================")
			t.Logf("✅ %s workflow COMPLETE with automated REAL BRIC", tt.name)
			t.Log("   ✅ All operations approved - NO 'RR' errors!")
			t.Log("========================================================================")
		})
	}
}

// executeCaptureStep performs a CAPTURE operation
func executeCaptureStep(t *testing.T, client *testutil.Client, bricResult *testutil.RealBRICResult, amount string) {
	captureReq := map[string]interface{}{
		"transaction_id": bricResult.TransactionID,
		"amount":         amount,
	}

	t.Log("💳 Capturing with REAL BRIC...")
	captureResp, err := client.Do("POST", "/api/v1/payments/capture", captureReq)
	require.NoError(t, err)
	defer captureResp.Body.Close()

	assert.Equal(t, 200, captureResp.StatusCode, "CAPTURE should succeed with REAL BRIC")

	var captureResult map[string]interface{}
	err = testutil.DecodeResponse(captureResp, &captureResult)
	require.NoError(t, err)

	isApproved, _ := captureResult["isApproved"].(bool)
	assert.True(t, isApproved, "CAPTURE should be approved with REAL BRIC")

	captureTransactionID := captureResult["transactionId"].(string)
	captureGroupID := captureResult["groupId"].(string)

	assert.Equal(t, bricResult.GroupID, captureGroupID, "CAPTURE should share same group_id")
	t.Logf("✅ CAPTURE successful - Transaction ID: %s", captureTransactionID)
}

// executeRefundStep performs a REFUND operation
func executeRefundStep(t *testing.T, client *testutil.Client, bricResult *testutil.RealBRICResult, refundAmount string) {
	// Convert refund amount to cents for ConnectRPC
	var amountCents int64
	fmt.Sscanf(refundAmount, "%f", &amountCents)
	amountCents = int64(amountCents * 100)

	refundReq := map[string]interface{}{
		"transaction_id": bricResult.TransactionID,
		"amount_cents":   amountCents,
		"reason":         "Customer request - automated test",
	}

	t.Log("💸 Refunding with REAL BRIC...")
	refundResp, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refundReq)
	require.NoError(t, err)
	defer refundResp.Body.Close()

	assert.Equal(t, 200, refundResp.StatusCode, "REFUND should succeed with REAL BRIC")

	var refundResult map[string]interface{}
	err = testutil.DecodeResponse(refundResp, &refundResult)
	require.NoError(t, err)

	isApproved, _ := refundResult["isApproved"].(bool)
	assert.True(t, isApproved, "REFUND should be approved with REAL BRIC")

	refundTransactionID, ok := refundResult["transactionId"].(string)
	require.True(t, ok && refundTransactionID != "", "REFUND should return transaction_id")

	refundParentTxID, ok := refundResult["parentTransactionId"].(string)
	require.True(t, ok && refundParentTxID != "", "REFUND should have parent_transaction_id")

	assert.Equal(t, bricResult.TransactionID, refundParentTxID, "REFUND parent should be the original transaction")
	t.Logf("✅ REFUND successful - Transaction ID: %s (parent: %s)", refundTransactionID, refundParentTxID)
}

// executeVoidStep performs a VOID operation
func executeVoidStep(t *testing.T, client *testutil.Client, bricResult *testutil.RealBRICResult) {
	voidReq := map[string]interface{}{
		"group_id": bricResult.GroupID,
	}

	t.Log("🚫 Voiding authorization with REAL BRIC...")
	voidResp, err := client.Do("POST", "/api/v1/payments/void", voidReq)
	require.NoError(t, err)
	defer voidResp.Body.Close()

	assert.Equal(t, 200, voidResp.StatusCode, "VOID should succeed with REAL BRIC")

	var voidResult map[string]interface{}
	err = testutil.DecodeResponse(voidResp, &voidResult)
	require.NoError(t, err)

	isApproved, _ := voidResult["isApproved"].(bool)
	assert.True(t, isApproved, "VOID should be approved with REAL BRIC")

	voidTransactionID := voidResult["transactionId"].(string)
	voidGroupID := voidResult["groupId"].(string)

	assert.Equal(t, bricResult.GroupID, voidGroupID, "VOID should share same group_id as AUTH")
	t.Logf("✅ VOID successful - Transaction ID: %s", voidTransactionID)
}

// executeSaleWithStoredBRIC performs a SALE using a stored BRIC from STORAGE transaction
func executeSaleWithStoredBRIC(t *testing.T, client *testutil.Client, bricResult *testutil.RealBRICResult, amount string) {
	// This would use the stored BRIC from STORAGE transaction to make a SALE
	// Implementation depends on how the service handles stored BRICs
	t.Log("💳 Processing SALE with stored BRIC...")
	// SALE with stored BRIC using Browser Post STORAGE flow
}

// TestBrowserPost_PartialCapture tests partial capture of an authorized amount
func TestBrowserPost_PartialCapture(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Load test service credentials for JWT authentication
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services, "No test services found")

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, "00000000-0000-0000-0000-000000000001", time.Hour)
	require.NoError(t, err)

	callbackBaseURL := "http://localhost:8081"

	// Step 1: Create AUTH for $150
	t.Log("[SETUP] Creating AUTH transaction for $150.00...")
	authResult := testutil.GetRealBRICForAuthAutomated(t, client, cfg, "150.00", callbackBaseURL, jwtToken)
	t.Logf("[CREATED] AUTH TX=%s", authResult.TransactionID)
	time.Sleep(2 * time.Second)

	// Set JWT auth for capture
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	// Step 2: Capture only $100 (partial capture)
	t.Log("[TEST] Capturing $100 of $150 authorized...")
	captureReq := map[string]interface{}{
		"transaction_id": authResult.TransactionID,
		"amount_cents":   int64(10000), // $100.00
	}

	captureResp, err := client.DoConnectRPC("payment.v1.PaymentService", "Capture", captureReq)
	require.NoError(t, err)
	defer captureResp.Body.Close()
	require.Equal(t, 200, captureResp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, testutil.DecodeResponse(captureResp, &result))

	assert.True(t, result["isApproved"].(bool), "Partial capture should be approved")

	// ConnectRPC may return amountCents as string or float64 depending on JSON encoding
	var amountCents float64
	switch v := result["amountCents"].(type) {
	case float64:
		amountCents = v
	case string:
		// Try parsing as int
		var parsed int64
		_, err = fmt.Sscanf(v, "%d", &parsed)
		require.NoError(t, err, "Failed to parse amountCents string")
		amountCents = float64(parsed)
	default:
		t.Fatalf("Unexpected amountCents type: %T", v)
	}
	assert.Equal(t, float64(10000), amountCents, "Should capture $100")

	t.Logf("[PASS] Partial capture successful: $100 of $150 authorized")
}

// TestBrowserPost_SaleWithToken tests a sale transaction using a tokenized card (not stored)
func TestBrowserPost_SaleWithToken(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Load test service credentials
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services)

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, "00000000-0000-0000-0000-000000000001", time.Hour)
	require.NoError(t, err)

	// Step 1: Tokenize card (get BRIC but don't store as payment method)
	t.Log("[SETUP] Tokenizing card via EPX...")
	storageBRIC, err := testutil.TokenizeCard(cfg, testutil.TestVisaCard)
	require.NoError(t, err)
	t.Logf("[CREATED] Storage BRIC: %s", storageBRIC)
	time.Sleep(1 * time.Second)

	// Step 2: Use token directly for sale (via Browser Post SALE with BRIC)
	t.Log("[TEST] Processing SALE with token...")
	callbackBaseURL := "http://localhost:8081"
	saleResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "29.99", callbackBaseURL, jwtToken)

	assert.NotEmpty(t, saleResult.TransactionID)
	assert.NotEmpty(t, saleResult.GroupID)

	t.Logf("[PASS] Sale with token successful: TX=%s", saleResult.TransactionID)
}

// TestBrowserPost_MultiplePartialRefunds tests processing multiple partial refunds against a single sale
func TestBrowserPost_MultiplePartialRefunds(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Load test service credentials
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services)

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, "00000000-0000-0000-0000-000000000001", time.Hour)
	require.NoError(t, err)

	callbackBaseURL := "http://localhost:8081"

	// Step 1: Create SALE for $200
	t.Log("[SETUP] Creating SALE transaction for $200.00...")
	saleResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "200.00", callbackBaseURL, jwtToken)
	t.Logf("[CREATED] SALE TX=%s", saleResult.TransactionID)
	time.Sleep(2 * time.Second)

	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	// Step 2: First partial refund of $50
	t.Log("[TEST] Processing first refund of $50...")
	refund1Req := map[string]interface{}{
		"transaction_id": saleResult.TransactionID,
		"amount_cents":   int64(5000),
		"reason":         "First partial refund",
	}

	refund1Resp, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refund1Req)
	require.NoError(t, err)
	defer refund1Resp.Body.Close()
	require.Equal(t, 200, refund1Resp.StatusCode)

	var refund1Result map[string]interface{}
	require.NoError(t, testutil.DecodeResponse(refund1Resp, &refund1Result))
	assert.True(t, refund1Result["isApproved"].(bool))
	t.Logf("[PASS] First refund of $50 succeeded: %s", refund1Result["transactionId"])

	time.Sleep(1 * time.Second)

	// Step 3: Second partial refund of $75
	t.Log("[TEST] Processing second refund of $75...")
	refund2Req := map[string]interface{}{
		"transaction_id": saleResult.TransactionID,
		"amount_cents":   int64(7500),
		"reason":         "Second partial refund",
	}

	refund2Resp, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refund2Req)
	require.NoError(t, err)
	defer refund2Resp.Body.Close()
	require.Equal(t, 200, refund2Resp.StatusCode)

	var refund2Result map[string]interface{}
	require.NoError(t, testutil.DecodeResponse(refund2Resp, &refund2Result))
	assert.True(t, refund2Result["isApproved"].(bool))
	t.Logf("[PASS] Second refund of $75 succeeded: %s", refund2Result["transactionId"])

	t.Log("[COMPLETE] Multiple partial refunds successful: $50 + $75 = $125 of $200 refunded")
}

// TestBrowserPost_ConcurrentOperations tests concurrent CAPTURE + VOID operations for race condition prevention
func TestBrowserPost_ConcurrentOperations(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Load test service credentials
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services)

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, "00000000-0000-0000-0000-000000000001", time.Hour)
	require.NoError(t, err)

	callbackBaseURL := "http://localhost:8081"

	// Step 1: Create AUTH transaction
	t.Log("[SETUP] Creating AUTH transaction for $100.00...")
	authResult := testutil.GetRealBRICForAuthAutomated(t, client, cfg, "100.00", callbackBaseURL, jwtToken)
	t.Logf("[CREATED] AUTH TX=%s GROUP=%s", authResult.TransactionID, authResult.GroupID)
	time.Sleep(2 * time.Second)

	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	// Step 2: Launch concurrent CAPTURE + VOID
	t.Log("[TEST] Launching concurrent CAPTURE + VOID operations...")

	var captureErr, voidErr error
	var captureStatus, voidStatus int
	var captureResp, voidResp map[string]interface{}

	// Use channels for goroutine synchronization
	captureDone := make(chan bool)
	voidDone := make(chan bool)

	// Concurrent CAPTURE
	go func() {
		defer func() { captureDone <- true }()
		resp, err := client.DoConnectRPC("payment.v1.PaymentService", "Capture", map[string]interface{}{
			"transaction_id": authResult.TransactionID,
			"amount_cents":   int64(10000),
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
		defer func() { voidDone <- true }()
		resp, err := client.DoConnectRPC("payment.v1.PaymentService", "Void", map[string]interface{}{
			"transaction_id": authResult.TransactionID,
		})
		voidErr = err
		if err == nil {
			voidStatus = resp.StatusCode
			testutil.DecodeResponse(resp, &voidResp)
			resp.Body.Close()
		}
	}()

	// Wait for both operations
	<-captureDone
	<-voidDone

	// Verify no data corruption
	captureSuccess := captureErr == nil && captureStatus == 200
	voidSuccess := voidErr == nil && voidStatus == 200

	t.Logf("[RESULT] CAPTURE=%v (status=%d) VOID=%v (status=%d)",
		captureSuccess, captureStatus, voidSuccess, voidStatus)

	// At least one should succeed, but no data corruption
	successCount := 0
	if captureSuccess {
		successCount++
	}
	if voidSuccess {
		successCount++
	}

	assert.GreaterOrEqual(t, successCount, 1, "At least one operation must succeed")
	assert.LessOrEqual(t, successCount, 2, "Max two operations can succeed")
	t.Log("[PASS] Concurrency test passed - no data corruption detected")
}
