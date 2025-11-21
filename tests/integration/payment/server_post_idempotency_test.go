//go:build integration
// +build integration

package payment_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_ServerPost_Refund_IdempotencySameUUID tests that calling Refund with the same UUID returns the same transaction
// This uses a regular Browser Post BRIC (not BRIC Storage) with real EPX
func TestIntegration_ServerPost_Refund_IdempotencySameUUID(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Generate JWT for authentication
	merchantID := "00000000-0000-0000-0000-000000000001"
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	// Step 1: Create a SALE via Browser Post to get a BRIC
	t.Log("Step 1: Creating SALE via Browser Post...")
	bricResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "50.00", callbackBaseURL, jwtToken)
	t.Logf("SALE approved: %s (Group: %s)", bricResult.TransactionID, bricResult.GroupID)

	time.Sleep(2 * time.Second)

	// Step 2: Perform first Refund with a specific UUID (idempotency key)
	t.Log("Step 2: Performing first Refund with idempotency key...")
	refundID := uuid.New()
	refundReq := map[string]interface{}{
		"merchant_id":     merchantID,
		"transaction_id":  bricResult.TransactionID,
		"amount_cents":    int64(2500), // 25.00 in cents
		"reason":          "Customer request",
		"idempotency_key": refundID.String(),
	}

	// Set JWT authentication
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	refundResp1, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refundReq)
	require.NoError(t, err)
	defer refundResp1.Body.Close()

	var refund1 map[string]interface{}
	err = testutil.DecodeResponse(refundResp1, &refund1)
	require.NoError(t, err)

	isApproved1, _ := refund1["isApproved"].(bool)
	require.True(t, isApproved1, "First Refund should be approved")

	transactionID1, ok := refund1["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")
	authCode1, ok := refund1["authorizationCode"].(string)
	require.True(t, ok, "authorizationCode should be a string")

	t.Logf("First Refund approved: %s (Auth Code: %s)", transactionID1, authCode1)

	// Step 3: Retry the Refund with the SAME UUID (idempotency test)
	t.Log("Step 3: Retrying Refund with same idempotency key...")

	// Set JWT authentication for retry
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	refundResp2, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refundReq)
	require.NoError(t, err)
	defer refundResp2.Body.Close()

	var refund2 map[string]interface{}
	err = testutil.DecodeResponse(refundResp2, &refund2)
	require.NoError(t, err)

	isApproved2, _ := refund2["isApproved"].(bool)
	require.True(t, isApproved2, "Second Refund should be approved (idempotency)")

	transactionID2, ok := refund2["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")
	authCode2, ok := refund2["authorizationCode"].(string)
	require.True(t, ok, "authorizationCode should be a string")

	// Assertions for idempotency
	assert.Equal(t, transactionID1, transactionID2, "Should return same transaction ID")
	assert.Equal(t, authCode1, authCode2, "Should return same auth code")

	t.Logf("✅ Idempotency verified: Both requests returned identical transaction %s", transactionID2)
	t.Log("✅ Test passed: Refund idempotency working correctly")
}

// TestIntegration_ServerPost_Void_IdempotencySameUUID tests that calling Void with the same UUID returns the same transaction with real EPX
func TestIntegration_ServerPost_Void_IdempotencySameUUID(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Generate JWT for authentication
	merchantID := "00000000-0000-0000-0000-000000000001"
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	// Step 1: Create an AUTH via Browser Post
	t.Log("Step 1: Creating AUTH via Browser Post...")
	bricResult := testutil.GetRealBRICForAuthAutomated(t, client, cfg, "50.00", callbackBaseURL, jwtToken)
	t.Logf("AUTH approved: %s", bricResult.TransactionID)

	time.Sleep(2 * time.Second)

	// Step 2: Perform first Void with a specific UUID
	t.Log("Step 2: Performing first Void with idempotency key...")
	voidID := uuid.New()
	voidReq := map[string]interface{}{
		"merchant_id":     merchantID,
		"transaction_id":  bricResult.TransactionID,
		"idempotency_key": voidID.String(),
	}

	// Set JWT authentication
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	voidResp1, err := client.DoConnectRPC("payment.v1.PaymentService", "Void", voidReq)
	require.NoError(t, err)
	defer voidResp1.Body.Close()

	var void1 map[string]interface{}
	err = testutil.DecodeResponse(voidResp1, &void1)
	require.NoError(t, err)

	isApproved1, _ := void1["isApproved"].(bool)
	require.True(t, isApproved1, "First Void should be approved")

	transactionID1, ok := void1["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")

	t.Logf("First Void approved: %s", transactionID1)

	// Step 3: Retry the Void with the SAME UUID
	t.Log("Step 3: Retrying Void with same idempotency key...")

	// Set JWT authentication for retry
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	voidResp2, err := client.DoConnectRPC("payment.v1.PaymentService", "Void", voidReq)
	require.NoError(t, err)
	defer voidResp2.Body.Close()

	var void2 map[string]interface{}
	err = testutil.DecodeResponse(voidResp2, &void2)
	require.NoError(t, err)

	transactionID2, ok := void2["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")

	// Assertions for idempotency
	assert.Equal(t, transactionID1, transactionID2, "Should return same transaction ID")

	t.Logf("✅ Idempotency verified: Both requests returned identical transaction %s", transactionID2)
	t.Log("✅ Test passed: Void idempotency working correctly")
}

// TestIntegration_ServerPost_Capture_IdempotencySameUUID tests that calling Capture with the same UUID returns the same transaction with real EPX
func TestIntegration_ServerPost_Capture_IdempotencySameUUID(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Generate JWT for authentication
	merchantID := "00000000-0000-0000-0000-000000000001"
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	// Step 1: Create an AUTH via Browser Post
	t.Log("Step 1: Creating AUTH via Browser Post...")
	bricResult := testutil.GetRealBRICForAuthAutomated(t, client, cfg, "50.00", callbackBaseURL, jwtToken)
	t.Logf("AUTH approved: %s", bricResult.TransactionID)

	time.Sleep(2 * time.Second)

	// Step 2: Perform first Capture with a specific UUID
	t.Log("Step 2: Performing first Capture with idempotency key...")
	captureID := uuid.New()
	captureReq := map[string]interface{}{
		"merchant_id":     merchantID,
		"transaction_id":  bricResult.TransactionID,
		"amount_cents":    int64(3000), // 30.00 in cents
		"idempotency_key": captureID.String(),
	}

	// Set JWT authentication
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	captureResp1, err := client.DoConnectRPC("payment.v1.PaymentService", "Capture", captureReq)
	require.NoError(t, err)
	defer captureResp1.Body.Close()

	var capture1 map[string]interface{}
	err = testutil.DecodeResponse(captureResp1, &capture1)
	require.NoError(t, err)

	isApproved1, _ := capture1["isApproved"].(bool)
	require.True(t, isApproved1, "First Capture should be approved")

	transactionID1, ok := capture1["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")

	t.Logf("First Capture approved: %s", transactionID1)

	// Step 3: Retry the Capture with the SAME UUID
	t.Log("Step 3: Retrying Capture with same idempotency key...")

	// Set JWT authentication for retry
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	captureResp2, err := client.DoConnectRPC("payment.v1.PaymentService", "Capture", captureReq)
	require.NoError(t, err)
	defer captureResp2.Body.Close()

	var capture2 map[string]interface{}
	err = testutil.DecodeResponse(captureResp2, &capture2)
	require.NoError(t, err)

	transactionID2, ok := capture2["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")

	// Assertions for idempotency
	assert.Equal(t, transactionID1, transactionID2, "Should return same transaction ID")

	t.Logf("✅ Idempotency verified: Both requests returned identical transaction %s", transactionID2)
	t.Log("✅ Test passed: Capture idempotency working correctly")
}

// TestIntegration_ServerPost_Refund_IdempotencyConcurrent tests that concurrent Refund requests with the same UUID don't create duplicates
// This is the critical race condition test using real EPX
func TestIntegration_ServerPost_Refund_IdempotencyConcurrent(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Generate JWT for authentication
	merchantID := "00000000-0000-0000-0000-000000000001"
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	// Step 1: Create a SALE via Browser Post
	t.Log("Step 1: Creating SALE via Browser Post...")
	bricResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "100.00", callbackBaseURL, jwtToken)
	t.Logf("SALE approved: %s", bricResult.TransactionID)

	time.Sleep(2 * time.Second)

	// Step 2: Create the FIRST refund and verify it completes successfully
	t.Log("Step 2: Creating first Refund to establish the transaction...")
	refundID := uuid.New()
	refundReq := map[string]interface{}{
		"merchant_id":     merchantID,
		"transaction_id":  bricResult.TransactionID,
		"amount_cents":    int64(5000), // 50.00 in cents
		"reason":          "Concurrent test",
		"idempotency_key": refundID.String(),
	}

	// Set JWT authentication
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	// Create the first refund and wait for it to complete
	refundResp1, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refundReq)
	require.NoError(t, err, "First refund should succeed")
	defer refundResp1.Body.Close()

	var refund1 map[string]interface{}
	err = testutil.DecodeResponse(refundResp1, &refund1)
	require.NoError(t, err, "Should decode first refund response")

	isApproved1, _ := refund1["isApproved"].(bool)
	require.True(t, isApproved1, "First Refund should be approved")

	firstTxID, ok := refund1["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")
	firstAuthCode, ok := refund1["authorizationCode"].(string)
	require.True(t, ok, "authorizationCode should be a string")

	t.Logf("✅ First Refund completed: %s (Auth Code: %s)", firstTxID, firstAuthCode)

	// Step 3: Now launch 10 concurrent requests with the SAME idempotency_key
	// These should all return the SAME completed transaction
	t.Log("Step 3: Launching 10 concurrent Refund requests with same idempotency_key...")

	concurrentCount := 10
	var wg sync.WaitGroup
	results := make([]map[string]interface{}, concurrentCount)
	errors := make([]error, concurrentCount)

	// Launch concurrent requests
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Set JWT authentication for each concurrent request
			client.SetHeader("Authorization", "Bearer "+jwtToken)
			defer client.ClearHeaders()

			resp, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refundReq)
			if err != nil {
				errors[index] = err
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			err = testutil.DecodeResponse(resp, &result)
			errors[index] = err
			results[index] = result
		}(i)
	}

	wg.Wait()
	t.Log("All concurrent requests completed")

	// Step 4: Verify all requests succeeded
	successCount := 0
	for i, err := range errors {
		if err == nil && results[i] != nil {
			successCount++
		} else if err != nil {
			t.Logf("Request %d error: %v", i, err)
		}
	}

	require.Equal(t, concurrentCount, successCount, "All concurrent requests should succeed")

	// Step 5: Verify all results are IDENTICAL to the first refund
	t.Log("Step 5: Verifying all concurrent requests returned the same transaction...")
	for i, result := range results {
		if result != nil {
			txID := result["transactionId"].(string)
			authCode := result["authorizationCode"].(string)

			assert.Equal(t, firstTxID, txID, fmt.Sprintf("Result %d: Transaction ID should match first refund", i))
			assert.Equal(t, firstAuthCode, authCode, fmt.Sprintf("Result %d: Auth code should match first refund", i))
		}
	}

	t.Logf("✅ All %d concurrent requests returned identical transaction %s", concurrentCount, firstTxID)
	t.Log("✅ Test passed: Concurrent refunds correctly returned same transaction via idempotency")
}

// TestIntegration_ServerPost_Refund_DifferentUUIDs tests that different UUIDs create different transactions with real EPX
func TestIntegration_ServerPost_Refund_DifferentUUIDs(t *testing.T) {
	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Generate JWT for authentication
	merchantID := "00000000-0000-0000-0000-000000000001"
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	// Step 1: Create a SALE via Browser Post
	t.Log("Step 1: Creating SALE via Browser Post...")
	bricResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "100.00", callbackBaseURL, jwtToken)
	t.Logf("SALE approved: %s", bricResult.TransactionID)

	time.Sleep(2 * time.Second)

	// Step 2: Perform first Refund with UUID 1
	t.Log("Step 2: Performing first Refund with UUID 1...")
	refundID1 := uuid.New()
	refund1Req := map[string]interface{}{
		"merchant_id":     merchantID,
		"transaction_id":  bricResult.TransactionID,
		"amount_cents":    int64(2500), // 25.00 in cents
		"reason":          "First refund",
		"idempotency_key": refundID1.String(),
	}

	// Set JWT authentication
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	refundResp1, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refund1Req)
	require.NoError(t, err)
	defer refundResp1.Body.Close()

	var refund1 map[string]interface{}
	err = testutil.DecodeResponse(refundResp1, &refund1)
	require.NoError(t, err)

	isApproved1, _ := refund1["isApproved"].(bool)
	require.True(t, isApproved1, "First Refund should be approved")

	txID1, ok := refund1["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")

	t.Logf("First Refund approved: %s", txID1)

	// Step 3: Perform second Refund with different UUID 2
	t.Log("Step 3: Performing second Refund with UUID 2...")
	refundID2 := uuid.New()
	refund2Req := map[string]interface{}{
		"merchant_id":     merchantID,
		"transaction_id":  bricResult.TransactionID,
		"amount_cents":    int64(3000), // 30.00 in cents
		"reason":          "Second refund",
		"idempotency_key": refundID2.String(),
	}

	// Set JWT authentication
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	refundResp2, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refund2Req)
	require.NoError(t, err)
	defer refundResp2.Body.Close()

	var refund2 map[string]interface{}
	err = testutil.DecodeResponse(refundResp2, &refund2)
	require.NoError(t, err)

	isApproved2, _ := refund2["isApproved"].(bool)
	require.True(t, isApproved2, "Second Refund should be approved")

	txID2, ok := refund2["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")

	t.Logf("Second Refund approved: %s", txID2)

	// Step 4: Verify they are DIFFERENT transactions
	assert.NotEqual(t, txID1, txID2, "Different UUIDs should create different transactions")

	t.Logf("✅ Test passed: Different UUIDs correctly created different transactions")
}
