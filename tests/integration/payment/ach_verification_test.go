//go:build integration
// +build integration

package payment_test

import (
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestACH_SaveAccount tests that saving an ACH account creates a pre-note
// and sets verification_status='pending'
func TestACH_SaveAccount(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-ach-save-001"
	time.Sleep(2 * time.Second)

	// Save ACH account (triggers pre-note CKC0)
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"test-merchant-staging",
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
	customerID := "test-customer-ach-block-001"
	time.Sleep(2 * time.Second)

	// Save ACH account (unverified)
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestACHChecking,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Try to make payment with unverified ACH account
	saleReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "100.00",
		"currency":          "USD",
		"metadata": map[string]string{
			"test_case": "unverified_ach_blocked",
		},
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	// Should be rejected
	assert.NotEqual(t, 200, saleResp.StatusCode,
		"Payment should be rejected for unverified ACH account")

	var errorResult map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &errorResult)
	require.NoError(t, err)

	// Verify error message mentions verification requirement
	if errorMsg, ok := errorResult["error"].(string); ok {
		assert.Contains(t, errorMsg, "verified",
			"Error should mention verification requirement")
	}

	t.Logf("✅ Unverified ACH payment correctly blocked - Error: %v", errorResult["error"])
}

// TestACH_AllowVerifiedPayments tests that verified ACH accounts can be used for payments
func TestACH_AllowVerifiedPayments(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-ach-allow-001"
	time.Sleep(2 * time.Second)

	// Save ACH account
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"test-merchant-staging",
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
	saleReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "250.00",
		"currency":          "USD",
		"metadata": map[string]string{
			"test_case": "verified_ach_allowed",
		},
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	// Should succeed
	assert.Equal(t, 200, saleResp.StatusCode,
		"Payment should succeed for verified ACH account")

	var result map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &result)
	require.NoError(t, err)

	assert.True(t, result["isApproved"].(bool), "Transaction should be approved")
	assert.Equal(t, "ach", result["paymentMethodType"])
	assert.Equal(t, "250.00", result["amount"])

	t.Logf("✅ Verified ACH payment approved - Transaction ID: %s", result["transactionId"])
}

// TestACH_FailedAccountBlocked tests that failed ACH accounts cannot be used
func TestACH_FailedAccountBlocked(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-ach-failed-001"
	time.Sleep(2 * time.Second)

	// Save ACH account
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"test-merchant-staging",
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
	saleReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "50.00",
		"currency":          "USD",
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	// Should be rejected
	assert.NotEqual(t, 200, saleResp.StatusCode,
		"Payment should be rejected for failed ACH account")

	var errorResult map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &errorResult)
	require.NoError(t, err)

	// Verify error message mentions inactive status
	if errorMsg, ok := errorResult["error"].(string); ok {
		assert.Contains(t, errorMsg, "not active",
			"Error should mention inactive status")
	}

	t.Logf("✅ Failed ACH account correctly blocked - Error: %v", errorResult["error"])
}

// TestACH_HighValuePayments tests that even verified ACH can handle high-value transactions
func TestACH_HighValuePayments(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-ach-highvalue-001"
	time.Sleep(2 * time.Second)

	// Save and verify ACH account
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg, client,
		"test-merchant-staging",
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
	saleReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "2500.00",
		"currency":          "USD",
		"metadata": map[string]string{
			"test_case": "high_value_verified_ach",
		},
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	// Should succeed
	assert.Equal(t, 200, saleResp.StatusCode,
		"High-value payment should succeed for verified ACH")

	var result map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &result)
	require.NoError(t, err)

	assert.True(t, result["isApproved"].(bool))
	assert.Equal(t, "2500.00", result["amount"])

	t.Logf("✅ High-value ACH payment approved - $2,500 transaction")
}
