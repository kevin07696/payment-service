//go:build integration
// +build integration

package payment_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addJWTAuth adds JWT authentication to a Connect request
func addJWTAuth[T any](t *testing.T, req *connect.Request[T], cfg *testutil.Config, merchantID string) {
	t.Helper()

	services, err := testutil.LoadTestServices()
	require.NoError(t, err, "Failed to load test services")
	require.NotEmpty(t, services, "No test services available")

	service := services[0]

	token, err := testutil.GenerateJWT(
		service.PrivateKeyPEM,
		service.ServiceID,
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err, "Failed to generate JWT")

	req.Header().Set("Authorization", "Bearer "+token)
}

// TestRefund_IdempotencyWithClientUUID tests refund idempotency using client-generated idempotency keys
// Pattern: client provides idempotency_key, which becomes the transaction ID (database enforces PRIMARY KEY uniqueness)
func TestRefund_IdempotencyWithClientUUID(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-refund-idempotency"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create tokenized payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient}
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Process sale
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 10000, // $100.00
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err)
	parentTxID := saleResp.Msg.TransactionId
	t.Logf("Created sale transaction: %s", parentTxID)

	time.Sleep(1 * time.Second)

	// CLIENT GENERATES REFUND IDEMPOTENCY KEY (which becomes transaction ID)
	refundIdempotencyKey := uuid.New().String()
	t.Logf("Client generated refund idempotency key: %s", refundIdempotencyKey)

	// First refund request with client-generated idempotency_key
	refundReq := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId:  parentTxID,           // The SALE transaction to refund
		AmountCents:    5000,                 // $50.00 partial refund
		Reason:         "Partial refund",
		IdempotencyKey: refundIdempotencyKey, // Client-provided UUID for idempotency
	})
	addJWTAuth(t, refundReq, cfg, merchantID)

	refund1Resp, err := client.Refund(ctx, refundReq)
	require.NoError(t, err)
	firstRefundTxID := refund1Resp.Msg.TransactionId
	assert.Equal(t, refundIdempotencyKey, firstRefundTxID, "Transaction ID should match client-provided idempotency key")
	t.Logf("First refund succeeded: %s", firstRefundTxID)

	time.Sleep(500 * time.Millisecond)

	// RETRY: Same refund request with SAME idempotency_key (simulating network retry)
	// Should be IDEMPOTENT - returns existing transaction, doesn't create duplicate
	refund2Resp, err := client.Refund(ctx, refundReq)
	require.NoError(t, err)
	secondRefundTxID := refund2Resp.Msg.TransactionId

	// VERIFY IDEMPOTENCY: Same transaction_id returned
	assert.Equal(t, firstRefundTxID, secondRefundTxID,
		"Retry with same idempotency_key should return existing transaction (idempotent)")

	// Verify only 1 refund exists (retry didn't create duplicate)
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId:          merchantID,
		ParentTransactionId: parentTxID,
		Limit:               100,
	})
	addJWTAuth(t, listReq, cfg, merchantID)

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)
	assert.Equal(t, 1, len(listResp.Msg.Transactions), "Should have 1 refund (retry didn't create duplicate)")

	t.Log("✅ Refund idempotency verified using client-generated idempotency key pattern")
}

// TestRefund_MultipleDifferentUUIDs tests that multiple refunds on same transaction work correctly
// Verifies that idempotency doesn't prevent legitimate multiple refunds
func TestRefund_MultipleDifferentUUIDs(t *testing.T) {
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

	// Process sale for $100
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 10000,
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err)
	parentTxID := saleResp.Msg.TransactionId
	t.Logf("Created $100 sale: %s", parentTxID)

	time.Sleep(1 * time.Second)

	// First refund: $30 with idempotency key 1
	refund1IdempotencyKey := uuid.New().String()
	refund1Req := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId:  parentTxID,
		AmountCents:    3000,
		Reason:         "First partial refund",
		IdempotencyKey: refund1IdempotencyKey,
	})
	addJWTAuth(t, refund1Req, cfg, merchantID)

	refund1Resp, err := client.Refund(ctx, refund1Req)
	require.NoError(t, err)
	assert.Equal(t, refund1IdempotencyKey, refund1Resp.Msg.TransactionId)
	t.Logf("First refund of $30 succeeded: %s", refund1Resp.Msg.TransactionId)

	time.Sleep(1 * time.Second)

	// Second refund: $40 with DIFFERENT idempotency key (new refund, not a retry)
	refund2IdempotencyKey := uuid.New().String()
	refund2Req := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId:  parentTxID,
		AmountCents:    4000,
		Reason:         "Second partial refund",
		IdempotencyKey: refund2IdempotencyKey,
	})
	addJWTAuth(t, refund2Req, cfg, merchantID)

	refund2Resp, err := client.Refund(ctx, refund2Req)
	require.NoError(t, err)
	assert.Equal(t, refund2IdempotencyKey, refund2Resp.Msg.TransactionId)
	assert.NotEqual(t, refund1IdempotencyKey, refund2Resp.Msg.TransactionId, "Different idempotency keys create different transactions")
	t.Logf("Second refund of $40 succeeded: %s", refund2Resp.Msg.TransactionId)

	// Verify 2 refunds exist
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId:          merchantID,
		ParentTransactionId: parentTxID,
		Limit:               100,
	})
	addJWTAuth(t, listReq, cfg, merchantID)

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)
	assert.Equal(t, 2, len(listResp.Msg.Transactions), "Should have 2 refunds")

	t.Log("✅ Multiple refunds with different idempotency keys work correctly (total: $70 of $100)")
}

// TestRefund_ConcurrentSameUUID tests handling of concurrent retry requests with same idempotency_key
// Idempotency key ensures concurrent retries are safe (only one transaction created)
func TestRefund_ConcurrentSameUUID(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-concurrent-refunds"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create tokenized payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient}
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Process sale for $100
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 10000,
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err)
	parentTxID := saleResp.Msg.TransactionId
	t.Logf("Created $100 sale: %s", parentTxID)

	time.Sleep(1 * time.Second)

	// Generate SINGLE refund idempotency key for concurrent requests
	refundIdempotencyKey := uuid.New().String()
	t.Logf("Refund idempotency key: %s", refundIdempotencyKey)

	// Launch 3 concurrent requests with SAME idempotency_key (simulating network retries)
	type refundResult struct {
		transactionID string
		err           error
	}

	results := make(chan refundResult, 3)

	// Launch 3 concurrent refund requests with SAME idempotency key
	for i := 0; i < 3; i++ {
		go func() {
			refundReq := connect.NewRequest(&paymentv1.RefundRequest{
				TransactionId:  parentTxID,
				AmountCents:    4000,
				Reason:         "Concurrent retry test",
				IdempotencyKey: refundIdempotencyKey, // SAME idempotency key for all 3 requests
			})
			addJWTAuth(t, refundReq, cfg, merchantID)

			resp, err := client.Refund(ctx, refundReq)
			if err != nil {
				results <- refundResult{err: err}
			} else {
				results <- refundResult{transactionID: resp.Msg.TransactionId, err: nil}
			}
		}()
	}

	// Collect results
	successCount := 0
	var transactionIDs []string
	for i := 0; i < 3; i++ {
		result := <-results
		if result.err == nil {
			successCount++
			transactionIDs = append(transactionIDs, result.transactionID)
		}
	}

	t.Logf("Concurrent retries: %d out of 3 succeeded", successCount)

	// All 3 requests should succeed (idempotent)
	assert.Equal(t, 3, successCount, "All retries should succeed (idempotent)")

	// All should return the SAME transaction_id
	for _, txID := range transactionIDs {
		assert.Equal(t, refundIdempotencyKey, txID, "All concurrent retries should return same transaction_id")
	}

	// Verify final state: only 1 refund exists
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId:          merchantID,
		ParentTransactionId: parentTxID,
		Limit:               100,
	})
	addJWTAuth(t, listReq, cfg, merchantID)

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)
	assert.Equal(t, 1, len(listResp.Msg.Transactions), "Should have 1 refund (concurrent retries didn't create duplicates)")

	t.Log("✅ Idempotency key prevents duplicate refunds from concurrent retries")
}
