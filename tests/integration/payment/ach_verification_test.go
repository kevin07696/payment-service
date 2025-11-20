//go:build integration
// +build integration

package payment_test

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestACH_SaveAccount tests that saving an ACH account creates a pre-note
// and sets verification_status='pending'
func TestACH_SaveAccount(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "10000000-0000-0000-0000-000000000001" // Customer UUID
	time.Sleep(2 * time.Second)

	// Save ACH account (triggers pre-note CKC0)
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"00000000-0000-0000-0000-000000000001", // Merchant UUID
		customerID,
		testutil.TestACHChecking,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Verify payment method was created with pending status
	db := testutil.GetDB(t)
	status, isVerified, err := testutil.GetACHVerificationStatus(db, paymentMethodID)
	require.NoError(t, err)

	assert.Equal(t, "pending", status, "New ACH account should have verification_status='pending'")
	assert.False(t, isVerified, "New ACH account should have is_verified=false")

	t.Logf("✅ ACH account saved - payment_method_id: %s, status: %s, is_verified: %v",
		paymentMethodID, status, isVerified)
}

// TestACH_BlockUnverifiedPayments tests that unverified ACH accounts cannot be used for payments
func TestACH_BlockUnverifiedPayments(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "10000000-0000-0000-0000-000000000002" // Customer UUID
	time.Sleep(2 * time.Second)

	// Save ACH account (unverified)
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"00000000-0000-0000-0000-000000000001", // Merchant UUID
		customerID,
		testutil.TestACHChecking,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Try to make payment with unverified ACH account
	idempotencyKey := uuid.New().String()
	saleReq := map[string]interface{}{
		"merchantId":      "00000000-0000-0000-0000-000000000001", // Merchant UUID
		"customerId":      customerID,
		"paymentMethodId": paymentMethodID,
		"amountCents":     10000, // $100.00 in cents
		"currency":        "USD",
		"idempotencyKey":  idempotencyKey,
		"metadata": map[string]string{
			"test_case": "unverified_ach_blocked",
		},
	}

	saleResp, err := client.Do("POST", "/payment.v1.PaymentService/Sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	// Should be rejected
	assert.NotEqual(t, 200, saleResp.StatusCode,
		"Payment should be rejected for unverified ACH account")

	// Manually decode error response (DecodeResponse returns error for non-2xx)
	body, err := io.ReadAll(saleResp.Body)
	require.NoError(t, err)

	var errorResult map[string]interface{}
	err = json.Unmarshal(body, &errorResult)
	require.NoError(t, err)

	// Verify error message mentions verification requirement (ConnectRPC uses "message" field)
	errorMsg, ok := errorResult["message"].(string)
	require.True(t, ok, "Error response should have 'message' field")
	assert.Contains(t, errorMsg, "verified",
		"Error should mention verification requirement")

	t.Logf("✅ Unverified ACH payment correctly blocked - Error: %v", errorMsg)
}

// TestACH_AllowVerifiedPayments tests that verified ACH accounts can be used for payments
func TestACH_AllowVerifiedPayments(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "10000000-0000-0000-0000-000000000003" // Customer UUID
	time.Sleep(2 * time.Second)

	// Save ACH account
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"00000000-0000-0000-0000-000000000001", // Merchant UUID
		customerID,
		testutil.TestACHChecking,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Simulate verification (simulate 3 days passing + cron job running)
	db := testutil.GetDB(t)
	err = testutil.MarkACHAsVerified(db, paymentMethodID)
	require.NoError(t, err)

	// Verify status changed
	status, isVerified, err := testutil.GetACHVerificationStatus(db, paymentMethodID)
	require.NoError(t, err)
	assert.Equal(t, "verified", status)
	assert.True(t, isVerified)

	// Now try to make payment
	idempotencyKey := uuid.New().String()
	saleReq := map[string]interface{}{
		"merchantId":      "00000000-0000-0000-0000-000000000001", // Merchant UUID
		"customerId":      customerID,
		"paymentMethodId": paymentMethodID,
		"amountCents":     25000, // $250.00 in cents
		"currency":        "USD",
		"idempotencyKey":  idempotencyKey,
		"metadata": map[string]string{
			"test_case": "verified_ach_allowed",
		},
	}

	saleResp, err := client.Do("POST", "/payment.v1.PaymentService/Sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	// Log the response for debugging
	if saleResp.StatusCode != 200 {
		body, _ := io.ReadAll(saleResp.Body)
		t.Fatalf("❌ Payment failed with status %d: %s", saleResp.StatusCode, string(body))
	}

	// Should succeed
	assert.Equal(t, 200, saleResp.StatusCode,
		"Payment should succeed for verified ACH account")

	var result map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &result)
	require.NoError(t, err)

	t.Logf("Response body: %+v", result)

	isApproved, ok := result["isApproved"].(bool)
	if !ok {
		t.Fatalf("isApproved field not found or wrong type in response: %+v", result)
	}
	assert.True(t, isApproved, "Transaction should be approved")

	// amountCents might be string or number depending on serialization
	amountStr, ok := result["amountCents"].(string)
	if ok {
		assert.Equal(t, "25000", amountStr, "Amount should be 25000 cents ($250.00)")
	} else {
		assert.Equal(t, float64(25000), result["amountCents"], "Amount should be 25000 cents ($250.00)")
	}

	t.Logf("✅ Verified ACH payment approved - Transaction ID: %s", result["transactionId"])
}

// TestACH_FailedAccountBlocked tests that failed ACH accounts cannot be used
func TestACH_FailedAccountBlocked(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "10000000-0000-0000-0000-000000000004" // Customer UUID
	time.Sleep(2 * time.Second)

	// Save ACH account
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"00000000-0000-0000-0000-000000000001", // Merchant UUID
		customerID,
		testutil.TestACHChecking,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Simulate ACH return code (failed verification)
	db := testutil.GetDB(t)
	err = testutil.MarkACHAsFailed(db, paymentMethodID, "R03: No Account/Unable to Locate")
	require.NoError(t, err)

	// Verify status changed
	status, isVerified, err := testutil.GetACHVerificationStatus(db, paymentMethodID)
	require.NoError(t, err)
	assert.Equal(t, "failed", status)
	assert.False(t, isVerified)

	// Try to make payment
	idempotencyKey := uuid.New().String()
	saleReq := map[string]interface{}{
		"merchantId":      "00000000-0000-0000-0000-000000000001", // Merchant UUID
		"customerId":      customerID,
		"paymentMethodId": paymentMethodID,
		"amountCents":     5000, // $50.00 in cents
		"currency":        "USD",
		"idempotencyKey":  idempotencyKey,
	}

	saleResp, err := client.Do("POST", "/payment.v1.PaymentService/Sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	// Should be rejected
	assert.NotEqual(t, 200, saleResp.StatusCode,
		"Payment should be rejected for failed ACH account")

	// Manually decode error response (DecodeResponse returns error for non-2xx)
	body, err := io.ReadAll(saleResp.Body)
	require.NoError(t, err)

	var errorResult map[string]interface{}
	err = json.Unmarshal(body, &errorResult)
	require.NoError(t, err)

	// Verify error message mentions inactive status (ConnectRPC uses "message" field)
	errorMsg, ok := errorResult["message"].(string)
	require.True(t, ok, "Error response should have 'message' field")
	assert.Contains(t, errorMsg, "not active",
		"Error should mention inactive status")

	t.Logf("✅ Failed ACH account correctly blocked - Error: %v", errorMsg)
}

// TestACH_HighValuePayments tests that even verified ACH can handle high-value transactions
func TestACH_HighValuePayments(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "10000000-0000-0000-0000-000000000005" // Customer UUID
	time.Sleep(2 * time.Second)

	// Save and verify ACH account
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"00000000-0000-0000-0000-000000000001", // Merchant UUID
		customerID,
		testutil.TestACHChecking,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Mark as verified
	db := testutil.GetDB(t)
	err = testutil.MarkACHAsVerified(db, paymentMethodID)
	require.NoError(t, err)

	// Try high-value payment ($2,500)
	idempotencyKey := uuid.New().String()
	saleReq := map[string]interface{}{
		"merchantId":      "00000000-0000-0000-0000-000000000001", // Merchant UUID
		"customerId":      customerID,
		"paymentMethodId": paymentMethodID,
		"amountCents":     250000, // $2,500.00 in cents
		"currency":        "USD",
		"idempotencyKey":  idempotencyKey,
		"metadata": map[string]string{
			"test_case": "high_value_verified_ach",
		},
	}

	saleResp, err := client.Do("POST", "/payment.v1.PaymentService/Sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	// Should succeed
	assert.Equal(t, 200, saleResp.StatusCode,
		"High-value payment should succeed for verified ACH")

	var result map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &result)
	require.NoError(t, err)

	assert.True(t, result["isApproved"].(bool))

	// amountCents might be string or number depending on serialization
	amountStr, ok := result["amountCents"].(string)
	if ok {
		assert.Equal(t, "250000", amountStr, "Amount should be 250000 cents ($2,500.00)")
	} else {
		assert.Equal(t, float64(250000), result["amountCents"], "Amount should be 250000 cents ($2,500.00)")
	}

	t.Logf("✅ High-value ACH payment approved - $2,500 transaction")
}
