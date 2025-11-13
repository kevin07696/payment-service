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

// TestBrowserPost_AuthCapture_Workflow tests the full SALE → REFUND workflow with real BRIC
// Uses headless Chrome automation to get real BRIC from EPX (fully automated!)
// Requires Chrome/Chromium installed on the system
// Requires ngrok or CALLBACK_BASE_URL for EPX to reach callback endpoint
// Note: EPX test merchant doesn't support AUTH-only ("A"), so we use SALE ("U") instead
func TestBrowserPost_AuthCapture_Workflow(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Try using localhost:8081 directly (EPX developer verified whitelist)
	callbackBaseURL := "http://localhost:8081"
	t.Logf("📡 Using callback URL: %s", callbackBaseURL)

	// Step 1: Get REAL BRIC from EPX using automated browser (SALE transaction)
	t.Log("🚀 Getting REAL BRIC from EPX via automated browser (SALE)...")
	bricResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "50.00", callbackBaseURL)

	t.Logf("✅ Step 1: SALE with REAL BRIC (automated):")
	t.Logf("   Transaction ID: %s", bricResult.TransactionID)
	t.Logf("   Group ID: %s", bricResult.GroupID)

	// Give EPX time to process the SALE
	time.Sleep(2 * time.Second)

	// Step 2: REFUND using group_id (uses real BRIC)
	refundReq := map[string]interface{}{
		"group_id": bricResult.GroupID, // Use group_id for refund
		"amount":   "25.00",             // Partial refund
		"reason":   "Customer request - automated test",
	}

	t.Log("💸 Refunding with REAL BRIC...")
	refundResp, err := client.Do("POST", "/api/v1/payments/refund", refundReq)
	require.NoError(t, err)
	defer refundResp.Body.Close()

	assert.Equal(t, 200, refundResp.StatusCode, "REFUND should succeed with REAL BRIC")

	var refundResult map[string]interface{}
	err = testutil.DecodeResponse(refundResp, &refundResult)
	require.NoError(t, err)

	isApproved, _ := refundResult["isApproved"].(bool)
	assert.True(t, isApproved, "REFUND should be approved with REAL BRIC")

	refundTransactionID := refundResult["transactionId"].(string)
	refundGroupID := refundResult["groupId"].(string)

	assert.Equal(t, bricResult.GroupID, refundGroupID, "REFUND should share same group_id")
	t.Logf("✅ Step 2: REFUND successful - Transaction ID: %s", refundTransactionID)

	t.Log("========================================================================")
	t.Logf("✅ SALE → REFUND workflow COMPLETE with automated REAL BRIC:")
	t.Logf("   SALE: %s (group: %s)", bricResult.TransactionID, bricResult.GroupID)
	t.Logf("   REFUND: %s (group: %s)", refundTransactionID, refundGroupID)
	t.Log("   ✅ All operations approved - NO 'RR' errors!")
	t.Log("========================================================================")
}

// TestBrowserPost_AuthCaptureRefund_Workflow tests the full AUTH → CAPTURE → REFUND workflow with real BRIC
// Verifies all three transactions share the same group_id and EPX accepts real BRIC for all operations
// Uses headless Chrome automation to get real BRIC from EPX (fully automated!)
// Requires ngrok or CALLBACK_BASE_URL for EPX to reach callback endpoint
func TestBrowserPost_AuthCaptureRefund_Workflow(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Try using localhost:8081 directly (EPX developer verified whitelist)
	callbackBaseURL := "http://localhost:8081"
	t.Logf("📡 Using callback URL: %s", callbackBaseURL)

	// Step 1: Get REAL BRIC from EPX using automated browser
	t.Log("🚀 Getting REAL BRIC from EPX via automated browser (AUTH)...")
	bricResult := testutil.GetRealBRICForAuthAutomated(t, client, cfg, "50.00", callbackBaseURL)

	t.Logf("✅ Step 1: AUTH with REAL BRIC:")
	t.Logf("   Transaction ID: %s", bricResult.TransactionID)
	t.Logf("   Group ID: %s", bricResult.GroupID)

	// Step 3: CAPTURE using transaction_id (uses real BRIC)
	captureReq := map[string]interface{}{
		"transaction_id": bricResult.TransactionID, // Use AUTH transaction ID
		"amount":         bricResult.Amount,        // Full capture
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
	t.Logf("✅ Step 2: CAPTURE successful - Transaction ID: %s", captureTransactionID)

	time.Sleep(2 * time.Second)

	// Step 4: REFUND using group_id (uses real BRIC)
	refundReq := map[string]interface{}{
		"group_id": bricResult.GroupID, // Use group_id for refund
		"amount":   "25.00",             // Partial refund
		"reason":   "Customer request",
	}

	t.Log("💸 Refunding with REAL BRIC...")
	refundResp, err := client.Do("POST", "/api/v1/payments/refund", refundReq)
	require.NoError(t, err)
	defer refundResp.Body.Close()

	assert.Equal(t, 200, refundResp.StatusCode, "REFUND should succeed with REAL BRIC")

	var refundResult map[string]interface{}
	err = testutil.DecodeResponse(refundResp, &refundResult)
	require.NoError(t, err)

	isRefundApproved, _ := refundResult["isApproved"].(bool)
	assert.True(t, isRefundApproved, "REFUND should be approved with REAL BRIC")

	refundTransactionID := refundResult["transactionId"].(string)
	refundGroupID := refundResult["groupId"].(string)

	// Verify all three transactions share the same group_id
	assert.Equal(t, bricResult.GroupID, refundGroupID, "REFUND should share same group_id")

	t.Logf("✅ Step 3: REFUND successful - Transaction ID: %s", refundTransactionID)

	t.Log("========================================================================")
	t.Logf("✅ AUTH → CAPTURE → REFUND workflow COMPLETE with REAL BRIC:")
	t.Logf("   AUTH: %s (group: %s)", bricResult.TransactionID, bricResult.GroupID)
	t.Logf("   CAPTURE: %s (group: %s)", captureTransactionID, captureGroupID)
	t.Logf("   REFUND: %s (group: %s)", refundTransactionID, refundGroupID)
	t.Log("   ✅ All operations approved - NO 'RR' errors!")
	t.Log("========================================================================")
}

// TestBrowserPost_AuthVoid_Workflow tests AUTH → VOID workflow with real BRIC
// VOID cancels an authorization before capture
// Uses headless Chrome automation to get real BRIC from EPX (fully automated!)
// Requires ngrok or CALLBACK_BASE_URL for EPX to reach callback endpoint
func TestBrowserPost_AuthVoid_Workflow(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Try using localhost:8081 directly (EPX developer verified whitelist)
	callbackBaseURL := "http://localhost:8081"
	t.Logf("📡 Using callback URL: %s", callbackBaseURL)

	// Step 1: Get REAL BRIC from EPX using automated browser
	t.Log("🚀 Getting REAL BRIC from EPX via automated browser (AUTH)...")
	bricResult := testutil.GetRealBRICForAuthAutomated(t, client, cfg, "100.00", callbackBaseURL)

	t.Logf("✅ Step 1: AUTH with REAL BRIC:")
	t.Logf("   Transaction ID: %s", bricResult.TransactionID)
	t.Logf("   Group ID: %s", bricResult.GroupID)

	// Step 3: VOID the authorization using group_id (uses real BRIC)
	voidReq := map[string]interface{}{
		"group_id": bricResult.GroupID, // Use group_id for void
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

	// Verify VOID shares same group_id
	assert.Equal(t, bricResult.GroupID, voidGroupID, "VOID should share same group_id as AUTH")

	t.Logf("✅ Step 2: VOID successful - Transaction ID: %s", voidTransactionID)

	// Verify VOID transaction
	time.Sleep(1 * time.Second)
	getVoidTxResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", voidTransactionID), nil)
	require.NoError(t, err)
	defer getVoidTxResp.Body.Close()

	var voidTx map[string]interface{}
	err = testutil.DecodeResponse(getVoidTxResp, &voidTx)
	require.NoError(t, err)

	assert.Equal(t, "TRANSACTION_STATUS_APPROVED", voidTx["status"], "VOID status (gateway outcome)")

	t.Log("========================================================================")
	t.Logf("✅ AUTH → VOID workflow COMPLETE with REAL BRIC:")
	t.Logf("   AUTH: %s (group: %s)", bricResult.TransactionID, bricResult.GroupID)
	t.Logf("   VOID: %s (group: %s)", voidTransactionID, voidGroupID)
	t.Log("   ✅ VOID successfully cancelled authorization - NO 'RR' errors!")
	t.Log("========================================================================")
}
