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

			callbackBaseURL := "http://localhost:8081"
			t.Logf("📡 Using callback URL: %s", callbackBaseURL)

			// Step 1: Get BRIC via Browser Post based on transaction type
			t.Logf("🚀 Getting REAL BRIC from EPX via automated browser (%s)...", tt.transactionType)

			var bricResult *testutil.RealBRICResult

			switch tt.transactionType {
			case "SALE":
				bricResult = testutil.GetRealBRICForSaleAutomated(t, client, cfg, tt.amount, callbackBaseURL)
			case "AUTH":
				bricResult = testutil.GetRealBRICForAuthAutomated(t, client, cfg, tt.amount, callbackBaseURL)
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
	// TODO: Implement SALE with stored BRIC when STORAGE endpoint is available
}
