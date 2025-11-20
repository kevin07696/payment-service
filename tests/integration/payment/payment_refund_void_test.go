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

// TestRefund_MultiplePartialRefunds tests multiple refunds on same transaction
// Verifies that multiple partial refunds can be processed against a single sale
func TestRefund_MultiplePartialRefunds(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-multiple-refunds"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create tokenized payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient}
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Process sale for $200
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 20000, // $200.00
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err)
	parentTxID := saleResp.Msg.TransactionId
	t.Logf("Created $200 sale: %s", parentTxID)

	time.Sleep(1 * time.Second)

	// First refund: $50
	refund1Req := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId: parentTxID,
		AmountCents:   5000, // $50.00
		Reason:        "First partial refund",
	})
	addJWTAuth(t, refund1Req, cfg, merchantID)

	refund1Resp, err := client.Refund(ctx, refund1Req)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), refund1Resp.Msg.AmountCents)
	t.Logf("First refund of $50 succeeded: %s", refund1Resp.Msg.TransactionId)

	time.Sleep(1 * time.Second)

	// Second refund: $75
	refund2Req := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId: parentTxID,
		AmountCents:   7500, // $75.00
		Reason:        "Second partial refund",
	})
	addJWTAuth(t, refund2Req, cfg, merchantID)

	refund2Resp, err := client.Refund(ctx, refund2Req)
	require.NoError(t, err)
	assert.Equal(t, int64(7500), refund2Resp.Msg.AmountCents)
	t.Logf("Second refund of $75 succeeded: %s", refund2Resp.Msg.TransactionId)

	// Verify 2 refunds exist for this parent transaction
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId:          merchantID,
		ParentTransactionId: parentTxID,
		Limit:               100,
	})
	addJWTAuth(t, listReq, cfg, merchantID)

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(listResp.Msg.Transactions), 2, "Should have at least 2 refunds")

	t.Log("✅ Multiple partial refunds completed - Total $125 refunded from $200 sale")
}

// TestVoid_CancelAuthorization tests void operation on an authorization (before capture)
// Verifies that an uncaptured authorization can be voided
func TestVoid_CancelAuthorization(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-void-auth"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create tokenized payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient}
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, merchantID, customerID, testutil.TestAmexCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Authorize (not captured yet, so can be voided)
	authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 7500, // $75.00
		Currency:    "USD",
		PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, authReq, cfg, merchantID)

	authResp, err := client.Authorize(ctx, authReq)
	require.NoError(t, err)
	authTxID := authResp.Msg.TransactionId
	t.Logf("Authorization completed: %s", authTxID)

	time.Sleep(1 * time.Second)

	// Void the authorization
	voidReq := connect.NewRequest(&paymentv1.VoidRequest{
		TransactionId: authTxID,
	})
	addJWTAuth(t, voidReq, cfg, merchantID)

	voidResp, err := client.Void(ctx, voidReq)
	require.NoError(t, err)
	assert.True(t, voidResp.Msg.IsApproved, "Void should be approved")
	assert.Equal(t, authTxID, voidResp.Msg.ParentTransactionId, "Void should reference auth as parent")

	t.Logf("✅ Void completed - Authorization %s canceled", authTxID)
}

// TestVoid_CapturedTransactionValidation tests void behavior on a captured transaction
// Some gateways may reject void on captured transactions, or treat it as refund
func TestVoid_CapturedTransactionValidation(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-void-captured"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create tokenized payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient}
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Process sale (auth + capture in one step)
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 5000, // $50.00
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err)
	saleTxID := saleResp.Msg.TransactionId
	t.Logf("Sale completed: %s", saleTxID)

	time.Sleep(1 * time.Second)

	// Try to void captured transaction
	// EPX may reject this or treat it as refund depending on timing
	voidReq := connect.NewRequest(&paymentv1.VoidRequest{
		TransactionId: saleTxID,
	})
	addJWTAuth(t, voidReq, cfg, merchantID)

	voidResp, err := client.Void(ctx, voidReq)

	// Document the behavior - gateway may handle this differently
	if err != nil {
		t.Logf("Void after capture rejected (expected): %v", err)
	} else {
		t.Logf("Void after capture accepted (may be treated as refund): %s", voidResp.Msg.TransactionId)
	}

	t.Log("✅ Void validation test completed - behavior documented")
}
