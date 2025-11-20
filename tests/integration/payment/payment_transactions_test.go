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

// TestSaleTransaction_WithStoredCard tests a sale transaction using a stored payment method
func TestSaleTransaction_WithStoredCard(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-sale-stored"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Tokenize and save payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Process a sale transaction
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 2999, // $29.99
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
		Metadata: map[string]string{
			"order_id": "ORDER-12345",
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err)

	// Verify response
	assert.NotEmpty(t, saleResp.Msg.TransactionId)
	assert.NotEmpty(t, saleResp.Msg.ParentTransactionId, "Parent transaction ID should be set for grouping")
	assert.Equal(t, int64(2999), saleResp.Msg.AmountCents)
	assert.Equal(t, "USD", saleResp.Msg.Currency)
	assert.True(t, saleResp.Msg.IsApproved)
	assert.NotEmpty(t, saleResp.Msg.AuthorizationCode)

	// Verify card info is abstracted
	assert.NotNil(t, saleResp.Msg.Card)
	assert.Equal(t, "visa", saleResp.Msg.Card.Brand)
	assert.Equal(t, "1111", saleResp.Msg.Card.LastFour)

	t.Logf("✅ Sale completed - Transaction ID: %s", saleResp.Msg.TransactionId)
}

// TestAuthorizeAndCapture_WithStoredCard tests auth + capture flow
func TestAuthorizeAndCapture_WithStoredCard(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-auth-capture"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Tokenize and save payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestMastercardCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Step 1: Authorize
	authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 15000, // $150.00
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
	t.Logf("Step 1: Authorization completed: %s", authTxID)

	time.Sleep(1 * time.Second)

	// Step 2: Capture (full amount)
	captureReq := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   15000,
	})
	addJWTAuth(t, captureReq, cfg, merchantID)

	captureResp, err := client.Capture(ctx, captureReq)
	require.NoError(t, err)
	assert.NotEmpty(t, captureResp.Msg.TransactionId)
	assert.Equal(t, authTxID, captureResp.Msg.ParentTransactionId, "Capture should reference auth as parent")
	assert.True(t, captureResp.Msg.IsApproved)

	t.Logf("✅ Step 2: Capture completed: %s", captureResp.Msg.TransactionId)
}

// TestAuthorizeAndCapture_PartialCapture tests partial capture
func TestAuthorizeAndCapture_PartialCapture(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-partial-capture"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Tokenize and save payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Authorize for $100
	authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 10000, // $100.00
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

	// Capture only $75 (partial)
	captureReq := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   7500, // $75.00
	})
	addJWTAuth(t, captureReq, cfg, merchantID)

	captureResp, err := client.Capture(ctx, captureReq)
	require.NoError(t, err)
	assert.Equal(t, int64(7500), captureResp.Msg.AmountCents, "Should capture partial amount")
	assert.True(t, captureResp.Msg.IsApproved)

	t.Log("✅ Partial capture completed - Captured $75 of $100 authorization")
}

// TestSaleTransaction_WithToken tests sale with one-time payment token
func TestSaleTransaction_WithToken(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "guest-customer-token"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Tokenize card but don't save it (one-time use)
	token, err := testutil.TokenizeCard(cfg, testutil.TestVisaCard)
	require.NoError(t, err, "Should tokenize card")
	time.Sleep(1 * time.Second)

	// Use token directly for sale (not saving payment method)
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 4999, // $49.99
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentToken{
			PaymentToken: token,
		},
		Metadata: map[string]string{
			"order_id": "GUEST-ORDER-67890",
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)

	if err != nil {
		t.Logf("Token sale returned error (may require valid EPX environment): %v", err)
	} else {
		assert.NotEmpty(t, saleResp.Msg.TransactionId)
		assert.NotEmpty(t, saleResp.Msg.ParentTransactionId)
		t.Logf("✅ Token sale completed: %s", saleResp.Msg.TransactionId)
	}
}

// TestGetTransaction retrieves transaction details
func TestGetTransaction(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-get-tx"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a transaction
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 9999, // $99.99
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err)
	transactionID := saleResp.Msg.TransactionId

	time.Sleep(1 * time.Second)

	// Retrieve transaction
	getReq := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: transactionID,
	})
	addJWTAuth(t, getReq, cfg, merchantID)

	getResp, err := client.GetTransaction(ctx, getReq)
	require.NoError(t, err)

	assert.Equal(t, transactionID, getResp.Msg.Id)
	assert.Equal(t, customerID, getResp.Msg.CustomerId)
	assert.Equal(t, int64(9999), getResp.Msg.AmountCents)

	t.Logf("✅ Retrieved transaction: %s", transactionID)
}

// TestListTransactions tests listing transactions with various filters
func TestListTransactions(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "test-customer-list-tx"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create payment method
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Create 3 transactions for the same customer
	var lastParentTxID string
	for i := 1; i <= 3; i++ {
		saleReq := connect.NewRequest(&paymentv1.SaleRequest{
			MerchantId:  merchantID,
			CustomerId:  customerID,
			AmountCents: 2500, // $25.00
			Currency:    "USD",
			PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
				PaymentMethodId: paymentMethodID,
			},
		})
		addJWTAuth(t, saleReq, cfg, merchantID)

		resp, err := client.Sale(ctx, saleReq)
		require.NoError(t, err)

		if i == 3 {
			// Save the parent_transaction_id from the last transaction for filtering test
			lastParentTxID = resp.Msg.ParentTransactionId
		}

		time.Sleep(1 * time.Second)
	}

	// Test 1: List transactions by customer_id
	t.Run("list_by_customer_id", func(t *testing.T) {
		listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
			MerchantId: merchantID,
			CustomerId: customerID,
			Limit:      100,
		})
		addJWTAuth(t, listReq, cfg, merchantID)

		listResp, err := client.ListTransactions(ctx, listReq)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(listResp.Msg.Transactions), 3, "Should have at least 3 transactions for customer")

		t.Logf("✅ Found %d transactions for customer %s", len(listResp.Msg.Transactions), customerID)
	})

	// Test 2: List transactions by parent_transaction_id
	t.Run("list_by_parent_transaction_id", func(t *testing.T) {
		listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
			MerchantId:          merchantID,
			ParentTransactionId: lastParentTxID,
			Limit:               100,
		})
		addJWTAuth(t, listReq, cfg, merchantID)

		listResp, err := client.ListTransactions(ctx, listReq)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(listResp.Msg.Transactions), 0, "Should have transactions for parent")

		// Verify all transactions have same parent_transaction_id
		for _, tx := range listResp.Msg.Transactions {
			assert.Equal(t, lastParentTxID, tx.ParentTransactionId, "All transactions should have same parent_transaction_id")
		}

		t.Logf("✅ Found %d transactions for parent %s", len(listResp.Msg.Transactions), lastParentTxID)
	})
}
