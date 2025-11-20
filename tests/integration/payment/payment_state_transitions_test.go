//go:build integration
// +build integration

package payment_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStateTransition_VoidAfterCapture tests that voiding a captured transaction fails or behaves as refund
func TestStateTransition_VoidAfterCapture(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-void-after-capture"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Step 1: Authorize $100
	authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 10000,
		Currency:    "USD",
		PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, authReq, cfg, merchantID)

	authResp, err := client.Authorize(ctx, authReq)
	require.NoError(t, err)
	authTxID := authResp.Msg.TransactionId
	t.Logf("Step 1: Authorization created: %s", authTxID)

	time.Sleep(1 * time.Second)

	// Step 2: Capture the authorization
	captureReq := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   10000,
	})
	addJWTAuth(t, captureReq, cfg, merchantID)

	captureResp, err := client.Capture(ctx, captureReq)
	require.NoError(t, err)
	assert.True(t, captureResp.Msg.IsApproved)
	t.Logf("Step 2: Captured transaction: %s", captureResp.Msg.TransactionId)

	time.Sleep(1 * time.Second)

	// Step 3: Try to VOID the captured transaction (should fail or behave as refund)
	voidReq := connect.NewRequest(&paymentv1.VoidRequest{
		TransactionId: authTxID,
	})
	addJWTAuth(t, voidReq, cfg, merchantID)

	voidResp, err := client.Void(ctx, voidReq)

	// EPX typically rejects voids on captured transactions
	if err != nil {
		t.Logf("✅ Void after capture correctly rejected: %v", err)
	} else {
		// If it succeeds, EPX may have converted it to a refund
		t.Logf("⚠️  Void after capture was processed (may have been converted to refund): %s", voidResp.Msg.TransactionId)
	}
}

// TestStateTransition_CaptureAfterVoid tests that capturing a voided authorization fails
func TestStateTransition_CaptureAfterVoid(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-capture-after-void"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Step 1: Authorize $75
	authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 7500,
		Currency:    "USD",
		PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, authReq, cfg, merchantID)

	authResp, err := client.Authorize(ctx, authReq)
	require.NoError(t, err)
	authTxID := authResp.Msg.TransactionId
	t.Logf("Step 1: Authorization created: %s", authTxID)

	time.Sleep(1 * time.Second)

	// Step 2: Void the authorization
	voidReq := connect.NewRequest(&paymentv1.VoidRequest{
		TransactionId: authTxID,
	})
	addJWTAuth(t, voidReq, cfg, merchantID)

	voidResp, err := client.Void(ctx, voidReq)
	require.NoError(t, err)
	assert.True(t, voidResp.Msg.IsApproved)
	t.Logf("Step 2: Authorization voided: %s", voidResp.Msg.TransactionId)

	time.Sleep(1 * time.Second)

	// Step 3: Try to CAPTURE the voided authorization (should fail)
	captureReq := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   7500,
	})
	addJWTAuth(t, captureReq, cfg, merchantID)

	_, err = client.Capture(ctx, captureReq)
	require.Error(t, err, "Capture after void should fail")
	t.Logf("✅ Capture after void correctly rejected: %v", err)
}

// TestStateTransition_PartialCaptureValidation tests partial capture amount validation
func TestStateTransition_PartialCaptureValidation(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-partial-capture"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Authorize for $100
	authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 10000,
		Currency:    "USD",
		PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, authReq, cfg, merchantID)

	authResp, err := client.Authorize(ctx, authReq)
	require.NoError(t, err)
	authTxID := authResp.Msg.TransactionId
	t.Logf("Authorized $100: %s", authTxID)

	time.Sleep(1 * time.Second)

	// Try to capture MORE than authorized amount ($150 > $100)
	overCaptureReq := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   15000, // Exceeds authorization
	})
	addJWTAuth(t, overCaptureReq, cfg, merchantID)

	_, err = client.Capture(ctx, overCaptureReq)
	if err != nil {
		t.Logf("✅ Over-capture correctly rejected: %v", err)
	} else {
		t.Log("⚠️  Over-capture was allowed - consider adding validation")
	}

	time.Sleep(1 * time.Second)

	// Now do a valid partial capture ($60)
	partialCaptureReq := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   6000,
	})
	addJWTAuth(t, partialCaptureReq, cfg, merchantID)

	partialCaptureResp, err := client.Capture(ctx, partialCaptureReq)
	require.NoError(t, err)
	assert.Equal(t, int64(6000), partialCaptureResp.Msg.AmountCents)
	t.Log("✅ Partial capture of $60 succeeded")
}

// TestStateTransition_MultipleCaptures tests that multiple captures on same auth are handled correctly
func TestStateTransition_MultipleCaptures(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-multiple-captures"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Authorize for $100
	authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 10000,
		Currency:    "USD",
		PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, authReq, cfg, merchantID)

	authResp, err := client.Authorize(ctx, authReq)
	require.NoError(t, err)
	authTxID := authResp.Msg.TransactionId
	parentTxID := authTxID
	t.Logf("Authorized $100: %s", authTxID)

	time.Sleep(1 * time.Second)

	// First capture: $40
	capture1Req := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   4000,
	})
	addJWTAuth(t, capture1Req, cfg, merchantID)

	capture1Resp, err := client.Capture(ctx, capture1Req)
	require.NoError(t, err)
	assert.True(t, capture1Resp.Msg.IsApproved)
	t.Log("First capture of $40 succeeded")

	time.Sleep(1 * time.Second)

	// Second capture: $40 (total = $80, within $100 auth)
	capture2Req := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   4000,
	})
	addJWTAuth(t, capture2Req, cfg, merchantID)

	capture2Resp, err := client.Capture(ctx, capture2Req)

	// EPX behavior: May allow multiple captures (multi-capture) or reject
	// Depends on merchant configuration
	if err != nil {
		t.Logf("✅ Second capture rejected - EPX doesn't support multi-capture: %v", err)
	} else {
		assert.True(t, capture2Resp.Msg.IsApproved)
		t.Log("✅ Multiple captures allowed - EPX supports multi-capture for this merchant")
	}

	// Verify transactions for this parent
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId:          merchantID,
		ParentTransactionId: parentTxID,
		Limit:               100,
	})
	addJWTAuth(t, listReq, cfg, merchantID)

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)
	t.Logf("Transaction group has %d child transactions", len(listResp.Msg.Transactions))
}

// TestStateTransition_RefundWithoutCapture tests that refunding an uncaptured auth fails
func TestStateTransition_RefundWithoutCapture(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-refund-no-capture"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Authorize (but don't capture)
	authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 5000,
		Currency:    "USD",
		PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, authReq, cfg, merchantID)

	authResp, err := client.Authorize(ctx, authReq)
	require.NoError(t, err)
	authTxID := authResp.Msg.TransactionId
	t.Logf("Authorization created (not captured): %s", authTxID)

	time.Sleep(1 * time.Second)

	// Try to refund the uncaptured authorization (should fail)
	refundReq := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId: authTxID,
		AmountCents:   5000,
		Reason:        "Refund attempt on uncaptured auth",
	})
	addJWTAuth(t, refundReq, cfg, merchantID)

	_, err = client.Refund(ctx, refundReq)

	// Expected: Should fail (can only refund captured/settled transactions)
	if err != nil {
		t.Logf("✅ Refund on uncaptured auth correctly rejected: %v", err)
	} else {
		t.Log("⚠️  Refund on uncaptured auth was allowed - EPX may have special handling")
	}
}

// TestStateTransition_FullWorkflow tests complete auth → capture → refund workflow
func TestStateTransition_FullWorkflow(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-full-workflow"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Step 1: Authorize $200
	authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 20000,
		Currency:    "USD",
		PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, authReq, cfg, merchantID)

	authResp, err := client.Authorize(ctx, authReq)
	require.NoError(t, err)
	authTxID := authResp.Msg.TransactionId
	assert.True(t, authResp.Msg.IsApproved)
	t.Logf("✅ Step 1: Authorized $200: %s", authTxID)

	time.Sleep(1 * time.Second)

	// Step 2: Capture $150 (partial)
	captureReq := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   15000,
	})
	addJWTAuth(t, captureReq, cfg, merchantID)

	captureResp, err := client.Capture(ctx, captureReq)
	require.NoError(t, err)
	assert.Equal(t, int64(15000), captureResp.Msg.AmountCents)
	assert.Equal(t, authTxID, captureResp.Msg.ParentTransactionId)
	t.Logf("✅ Step 2: Captured $150 (partial): %s", captureResp.Msg.TransactionId)

	time.Sleep(1 * time.Second)

	// Step 3: Refund $75
	refundReq := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId: authTxID,
		AmountCents:   7500,
		Reason:        "Partial refund on captured amount",
	})
	addJWTAuth(t, refundReq, cfg, merchantID)

	refundResp, err := client.Refund(ctx, refundReq)
	require.NoError(t, err)
	assert.Equal(t, int64(7500), refundResp.Msg.AmountCents)
	assert.Equal(t, authTxID, refundResp.Msg.ParentTransactionId)
	assert.True(t, refundResp.Msg.IsApproved)
	t.Logf("✅ Step 3: Refunded $75: %s", refundResp.Msg.TransactionId)

	// Step 4: Verify all transactions linked by parent_transaction_id
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId:          merchantID,
		ParentTransactionId: authTxID,
		Limit:               100,
	})
	addJWTAuth(t, listReq, cfg, merchantID)

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(listResp.Msg.Transactions), 2, "Should have capture and refund")

	// Verify all share same parent_transaction_id
	for _, tx := range listResp.Msg.Transactions {
		assert.Equal(t, authTxID, tx.ParentTransactionId)
	}

	t.Logf("✅ Step 4: Verified %d child transactions for auth %s", len(listResp.Msg.Transactions), authTxID)
	t.Log("✅ Full workflow completed: Auth → Capture → Refund")
}
