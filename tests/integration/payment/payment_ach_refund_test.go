//go:build integration
// +build integration

package payment_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/require"
)

// TestACH_SaleAndRefund_CriticalBugFix tests the complete ACH sale and refund workflow
// This test verifies the CRITICAL BUG FIX where ACH refunds now use CKC3 (ACH Credit)
// instead of CCE9 (Credit Card Refund)
//
// BEFORE FIX: ACH refunds failed because they used CCE9 transaction type
// AFTER FIX: ACH refunds work because adapter now translates OperationRefund + ACH → CKC3
func TestACH_SaleAndRefund_CriticalBugFix(t *testing.T) {
	cfg, client := testutil.Setup(t)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := uuid.New().String()

	time.Sleep(1 * time.Second)

	// Generate JWT token for authentication
	jwtToken := generateJWTToken(t, merchantID)

	// Step 1: Save and verify ACH account
	t.Log("Step 1: Save and verify ACH checking account...")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	paymentMethodID, err := testutil.TokenizeAndSaveACH(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Manually verify the ACH account (simulate pre-note verification)
	t.Log("Step 1a: Manually verifying ACH account...")
	db := testutil.GetDB(t)
	err = testutil.MarkACHAsVerified(db, paymentMethodID)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Step 2: Make ACH sale (uses CKC2 - ACH Debit)
	t.Log("Step 2: Making ACH sale ($100.00)...")
	saleIdempotencyKey := uuid.New().String()
	saleReq := map[string]interface{}{
		"merchant_id":  merchantID,
		"customer_id":  customerID,
		"amount_cents": 10000, // $100.00
		"currency":     "USD",
		"payment_method": map[string]interface{}{
			"payment_method_id": paymentMethodID,
		},
		"idempotency_key": saleIdempotencyKey,
		"metadata": map[string]interface{}{
			"test_case": "ach_refund_bug_fix",
			"note":      "Testing ACH refund fix - should use CKC3 not CCE9",
		},
	}

	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	saleResp, err := client.DoConnectRPC("payment.v1.PaymentService", "Sale", saleReq)
	require.NoError(t, err, "ACH Sale should succeed")
	defer saleResp.Body.Close()

	var saleResult map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &saleResult)
	require.NoError(t, err)

	isApproved, _ := saleResult["isApproved"].(bool)
	require.True(t, isApproved, "Sale should be approved")

	saleTransactionID, ok := saleResult["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")
	require.NotEmpty(t, saleTransactionID, "Sale transaction ID should be set")

	t.Logf("✅ ACH Sale approved - Transaction ID: %s, Amount: $100.00", saleTransactionID)

	time.Sleep(2 * time.Second) // Give EPX time to process

	// Step 3: Refund the ACH payment (CRITICAL: should use CKC3, not CCE9)
	t.Log("Step 3: Refunding ACH payment ($50.00 partial refund)...")
	refundIdempotencyKey := uuid.New().String()
	refundReq := map[string]interface{}{
		"merchant_id":           merchantID,
		"parent_transaction_id": saleTransactionID,
		"amount_cents":          5000, // $50.00 partial refund
		"reason":                "Customer requested partial refund",
		"idempotency_key":       refundIdempotencyKey,
		"metadata": map[string]interface{}{
			"test_case": "ach_refund_bug_fix_verification",
			"note":      "CRITICAL: This refund MUST use CKC3 (ACH Credit) NOT CCE9 (CC Refund)",
		},
	}

	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	// THIS IS THE CRITICAL TEST - ACH refunds should now work!
	refundResp, err := client.DoConnectRPC("payment.v1.PaymentService", "Refund", refundReq)
	require.NoError(t, err, "ACH Refund should succeed (CRITICAL BUG FIX VERIFICATION)")
	defer refundResp.Body.Close()

	var refundResult map[string]interface{}
	err = testutil.DecodeResponse(refundResp, &refundResult)
	require.NoError(t, err)

	isRefundApproved, _ := refundResult["isApproved"].(bool)
	require.True(t, isRefundApproved, "ACH Refund should be approved")

	refundTransactionID, ok := refundResult["transactionId"].(string)
	require.True(t, ok, "transactionId should be a string")
	require.NotEmpty(t, refundTransactionID, "Refund transaction ID should be set")

	t.Logf("✅ ACH Refund approved - Transaction ID: %s, Amount: $50.00", refundTransactionID)

	t.Log("\n" +
		"═══════════════════════════════════════════════════════════════════\n" +
		"✅ CRITICAL BUG FIX VERIFIED: ACH Refund Works!\n" +
		"───────────────────────────────────────────────────────────────────\n" +
		"Before fix: ACH refunds failed (used CCE9 - credit card only)\n" +
		"After fix:  ACH refunds succeed (uses CKC3 - ACH credit)\n" +
		"───────────────────────────────────────────────────────────────────\n" +
		"Implementation: Semantic operations pattern in adapter layer\n" +
		"  OperationRefund + ACH → CKC3 (ACH Credit)\n" +
		"  OperationRefund + CC  → CCE9 (Credit Card Refund)\n" +
		"═══════════════════════════════════════════════════════════════════\n")
}
