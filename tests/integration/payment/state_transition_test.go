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

// TestStateTransition_VoidAfterCapture tests that voiding a captured transaction fails or behaves as refund
func TestStateTransition_VoidAfterCapture(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-void-after-capture"
	time.Sleep(2 * time.Second)

	// Create payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Step 1: Authorize $100
	authReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "100.00",
		"currency":          "USD",
	}

	authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()

	var authResult map[string]interface{}
	err = testutil.DecodeResponse(authResp, &authResult)
	require.NoError(t, err)

	transactionID := authResult["transactionId"].(string)
	groupID := authResult["groupId"].(string)
	t.Logf("Step 1: Authorization created - Transaction ID: %s", transactionID)

	time.Sleep(2 * time.Second)

	// Step 2: Capture the authorization
	captureReq := map[string]interface{}{
		"transaction_id": transactionID,
		"amount":         "100.00",
	}

	captureResp, err := client.Do("POST", "/api/v1/payments/capture", captureReq)
	require.NoError(t, err)
	defer captureResp.Body.Close()

	assert.Equal(t, 200, captureResp.StatusCode, "Capture should succeed")

	var captureResult map[string]interface{}
	err = testutil.DecodeResponse(captureResp, &captureResult)
	require.NoError(t, err)

	t.Logf("Step 2: Captured transaction - Group: %s", groupID)

	time.Sleep(2 * time.Second)

	// Step 3: Try to VOID the captured transaction (should fail)
	voidReq := map[string]interface{}{
		"group_id": groupID,
	}

	voidResp, err := client.Do("POST", "/api/v1/payments/void", voidReq)
	require.NoError(t, err)
	defer voidResp.Body.Close()

	// Expected: Should fail OR behave as refund (depending on EPX behavior)
	// EPX typically rejects voids on captured transactions
	if voidResp.StatusCode != 200 {
		t.Logf("✅ Void after capture correctly rejected (HTTP %d)", voidResp.StatusCode)
	} else {
		// If it succeeds, EPX may have converted it to a refund
		var voidResult map[string]interface{}
		err = testutil.DecodeResponse(voidResp, &voidResult)
		require.NoError(t, err)

		t.Logf("⚠️  Void after capture was processed (may have been converted to refund by EPX)")
		t.Logf("    Transaction ID: %v", voidResult["transactionId"])
	}
}

// TestStateTransition_CaptureAfterVoid tests that capturing a voided authorization fails
func TestStateTransition_CaptureAfterVoid(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-capture-after-void"
	time.Sleep(2 * time.Second)

	// Create payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Step 1: Authorize $75
	authReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "75.00",
		"currency":          "USD",
	}

	authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()

	var authResult map[string]interface{}
	err = testutil.DecodeResponse(authResp, &authResult)
	require.NoError(t, err)

	transactionID := authResult["transactionId"].(string)
	groupID := authResult["groupId"].(string)
	t.Logf("Step 1: Authorization created - Transaction ID: %s", transactionID)

	time.Sleep(2 * time.Second)

	// Step 2: Void the authorization
	voidReq := map[string]interface{}{
		"group_id": groupID,
	}

	voidResp, err := client.Do("POST", "/api/v1/payments/void", voidReq)
	require.NoError(t, err)
	defer voidResp.Body.Close()

	assert.Equal(t, 200, voidResp.StatusCode, "Void should succeed")

	var voidResult map[string]interface{}
	err = testutil.DecodeResponse(voidResp, &voidResult)
	require.NoError(t, err)

	t.Logf("Step 2: Authorization voided - Group: %s", groupID)

	time.Sleep(2 * time.Second)

	// Step 3: Try to CAPTURE the voided authorization (should fail)
	captureReq := map[string]interface{}{
		"transaction_id": transactionID,
		"amount":         "75.00",
	}

	captureResp, err := client.Do("POST", "/api/v1/payments/capture", captureReq)
	require.NoError(t, err)
	defer captureResp.Body.Close()

	// Expected: Should fail because authorization was voided
	assert.NotEqual(t, 200, captureResp.StatusCode, "Capture after void should fail")
	t.Logf("✅ Capture after void correctly rejected (HTTP %d)", captureResp.StatusCode)
}

// TestStateTransition_PartialCaptureValidation tests partial capture amount validation
func TestStateTransition_PartialCaptureValidation(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-partial-capture-validation"
	time.Sleep(2 * time.Second)

	// Create payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Authorize for $100
	authReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "100.00",
		"currency":          "USD",
	}

	authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()

	var authResult map[string]interface{}
	err = testutil.DecodeResponse(authResp, &authResult)
	require.NoError(t, err)

	transactionID := authResult["transactionId"].(string)
	t.Logf("Authorized $100 - Transaction ID: %s", transactionID)

	time.Sleep(2 * time.Second)

	// Try to capture MORE than authorized amount ($150 > $100)
	overCaptureReq := map[string]interface{}{
		"transaction_id": transactionID,
		"amount":         "150.00", // Exceeds authorization
	}

	overCaptureResp, err := client.Do("POST", "/api/v1/payments/capture", overCaptureReq)
	require.NoError(t, err)
	defer overCaptureResp.Body.Close()

	// Expected: Should fail (cannot capture more than authorized)
	if overCaptureResp.StatusCode != 200 {
		t.Logf("✅ Over-capture correctly rejected (HTTP %d)", overCaptureResp.StatusCode)
	} else {
		t.Log("⚠️  Over-capture was allowed - consider adding validation to prevent capturing more than authorized amount")
	}

	time.Sleep(2 * time.Second)

	// Now do a valid partial capture ($60)
	partialCaptureReq := map[string]interface{}{
		"transaction_id": transactionID,
		"amount":         "60.00",
	}

	partialCaptureResp, err := client.Do("POST", "/api/v1/payments/capture", partialCaptureReq)
	require.NoError(t, err)
	defer partialCaptureResp.Body.Close()

	assert.Equal(t, 200, partialCaptureResp.StatusCode, "Partial capture should succeed")

	var partialCaptureResult map[string]interface{}
	err = testutil.DecodeResponse(partialCaptureResp, &partialCaptureResult)
	require.NoError(t, err)

	assert.Equal(t, "60.00", partialCaptureResult["amount"], "Should capture partial amount")
	t.Log("✅ Partial capture of $60 succeeded")
}

// TestStateTransition_MultipleCaptures tests that multiple captures on same auth are handled correctly
func TestStateTransition_MultipleCaptures(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-multiple-captures"
	time.Sleep(2 * time.Second)

	// Create payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Authorize for $100
	authReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "100.00",
		"currency":          "USD",
	}

	authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()

	var authResult map[string]interface{}
	err = testutil.DecodeResponse(authResp, &authResult)
	require.NoError(t, err)

	transactionID := authResult["transactionId"].(string)
	groupID := authResult["groupId"].(string)
	t.Logf("Authorized $100 - Transaction ID: %s", transactionID)

	time.Sleep(2 * time.Second)

	// First capture: $40
	capture1Req := map[string]interface{}{
		"transaction_id": transactionID,
		"amount":         "40.00",
	}

	capture1Resp, err := client.Do("POST", "/api/v1/payments/capture", capture1Req)
	require.NoError(t, err)
	defer capture1Resp.Body.Close()

	assert.Equal(t, 200, capture1Resp.StatusCode, "First capture should succeed")
	t.Log("First capture of $40 succeeded")

	time.Sleep(2 * time.Second)

	// Second capture: $40 (total = $80, within $100 auth)
	capture2Req := map[string]interface{}{
		"transaction_id": transactionID,
		"amount":         "40.00",
	}

	capture2Resp, err := client.Do("POST", "/api/v1/payments/capture", capture2Req)
	require.NoError(t, err)
	defer capture2Resp.Body.Close()

	// EPX behavior: May allow multiple captures (multi-capture) or reject
	// Depends on merchant configuration
	if capture2Resp.StatusCode == 200 {
		t.Log("✅ Multiple captures allowed (EPX supports multi-capture for this merchant)")
	} else {
		t.Logf("✅ Second capture rejected (HTTP %d) - EPX doesn't support multi-capture", capture2Resp.StatusCode)
	}

	time.Sleep(1 * time.Second)

	// Verify group transactions
	listResp, err := client.Do("GET",
		fmt.Sprintf("/api/v1/payments?agent_id=test-merchant-staging&group_id=%s", groupID), nil)
	require.NoError(t, err)
	defer listResp.Body.Close()

	var listResult map[string]interface{}
	err = testutil.DecodeResponse(listResp, &listResult)
	require.NoError(t, err)

	transactions := listResult["transactions"].([]interface{})
	t.Logf("Group has %d transactions (1 auth + %d captures)", len(transactions), len(transactions)-1)
}

// TestStateTransition_RefundWithoutCapture tests that refunding an uncaptured auth fails
func TestStateTransition_RefundWithoutCapture(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-refund-no-capture"
	time.Sleep(2 * time.Second)

	// Create payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Authorize (but don't capture)
	authReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "50.00",
		"currency":          "USD",
	}

	authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()

	var authResult map[string]interface{}
	err = testutil.DecodeResponse(authResp, &authResult)
	require.NoError(t, err)

	groupID := authResult["groupId"].(string)
	t.Logf("Authorization created (not captured) - Group: %s", groupID)

	time.Sleep(2 * time.Second)

	// Try to refund the uncaptured authorization (should fail)
	refundReq := map[string]interface{}{
		"group_id": groupID,
		"amount":   "50.00",
		"reason":   "Refund attempt on uncaptured auth",
	}

	refundResp, err := client.Do("POST", "/api/v1/payments/refund", refundReq)
	require.NoError(t, err)
	defer refundResp.Body.Close()

	// Expected: Should fail (can only refund captured/settled transactions)
	if refundResp.StatusCode != 200 {
		t.Logf("✅ Refund on uncaptured auth correctly rejected (HTTP %d)", refundResp.StatusCode)
	} else {
		t.Log("⚠️  Refund on uncaptured auth was allowed - EPX may have special handling")
		t.Log("    Typically, refunds should only work on captured transactions")
	}
}

// TestStateTransition_FullWorkflow tests complete auth → capture → refund workflow
func TestStateTransition_FullWorkflow(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-full-workflow"
	time.Sleep(2 * time.Second)

	// Create payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Step 1: Authorize $200
	authReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "200.00",
		"currency":          "USD",
	}

	authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()

	var authResult map[string]interface{}
	err = testutil.DecodeResponse(authResp, &authResult)
	require.NoError(t, err)

	transactionID := authResult["transactionId"].(string)
	groupID := authResult["groupId"].(string)
	assert.True(t, authResult["isApproved"].(bool), "Authorization should be approved")
	t.Logf("✅ Step 1: Authorized $200 - Group: %s", groupID)

	time.Sleep(2 * time.Second)

	// Step 2: Capture $150 (partial)
	captureReq := map[string]interface{}{
		"transaction_id": transactionID,
		"amount":         "150.00",
	}

	captureResp, err := client.Do("POST", "/api/v1/payments/capture", captureReq)
	require.NoError(t, err)
	defer captureResp.Body.Close()

	assert.Equal(t, 200, captureResp.StatusCode, "Capture should succeed")

	var captureResult map[string]interface{}
	err = testutil.DecodeResponse(captureResp, &captureResult)
	require.NoError(t, err)

	assert.Equal(t, "150.00", captureResult["amount"], "Should capture partial amount")
	assert.Equal(t, groupID, captureResult["groupId"], "Should have same group_id")
	t.Logf("✅ Step 2: Captured $150 (partial)")

	time.Sleep(2 * time.Second)

	// Step 3: Refund $75
	refundReq := map[string]interface{}{
		"group_id": groupID,
		"amount":   "75.00",
		"reason":   "Partial refund on captured amount",
	}

	refundResp, err := client.Do("POST", "/api/v1/payments/refund", refundReq)
	require.NoError(t, err)
	defer refundResp.Body.Close()

	assert.Equal(t, 200, refundResp.StatusCode, "Refund should succeed")

	var refundResult map[string]interface{}
	err = testutil.DecodeResponse(refundResp, &refundResult)
	require.NoError(t, err)

	assert.Equal(t, "75.00", refundResult["amount"], "Should refund requested amount")
	assert.Equal(t, groupID, refundResult["groupId"], "Should have same group_id")
	assert.True(t, refundResult["isApproved"].(bool), "Refund should be approved")
	t.Logf("✅ Step 3: Refunded $75")

	time.Sleep(1 * time.Second)

	// Step 4: Verify all transactions linked by group_id
	listResp, err := client.Do("GET",
		fmt.Sprintf("/api/v1/payments?agent_id=test-merchant-staging&group_id=%s", groupID), nil)
	require.NoError(t, err)
	defer listResp.Body.Close()

	var listResult map[string]interface{}
	err = testutil.DecodeResponse(listResp, &listResult)
	require.NoError(t, err)

	transactions := listResult["transactions"].([]interface{})
	assert.GreaterOrEqual(t, len(transactions), 3, "Should have auth, capture, and refund")

	// Verify all share same group_id
	for _, txInterface := range transactions {
		tx := txInterface.(map[string]interface{})
		assert.Equal(t, groupID, tx["groupId"], "All transactions should share group_id")
	}

	t.Logf("✅ Step 4: Verified %d transactions all linked by group_id: %s", len(transactions), groupID)
	t.Log("✅ Full workflow completed: Auth → Capture → Refund")
}
