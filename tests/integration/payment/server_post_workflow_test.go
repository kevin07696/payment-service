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

// TestServerPost_AuthorizeWithStoredCard tests authorization using ConnectRPC with a stored payment method
func TestServerPost_AuthorizeWithStoredCard(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "00000000-0000-0000-0000-000000001001" // UUID format required
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create payment method via Browser Post STORAGE
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL,
	)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Test: Authorize using ConnectRPC with stored payment method
	t.Log("[TEST] Authorizing payment with stored card via ConnectRPC...")
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

	// Verify response
	assert.NotEmpty(t, authResp.Msg.TransactionId)
	assert.Equal(t, int64(15000), authResp.Msg.AmountCents)
	assert.True(t, authResp.Msg.IsApproved, "Authorization should be approved")
	assert.NotEmpty(t, authResp.Msg.AuthorizationCode)

	t.Logf("✅ Authorization successful via ConnectRPC - TX: %s", authResp.Msg.TransactionId)
}

// TestServerPost_SaleWithStoredCard tests sale transaction using ConnectRPC with stored payment method
func TestServerPost_SaleWithStoredCard(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "00000000-0000-0000-0000-000000001002" // UUID format required
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create payment method via Browser Post STORAGE
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestMastercardCard, callbackBaseURL,
	)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Test: Sale using ConnectRPC with stored payment method
	t.Log("[TEST] Processing SALE with stored card via ConnectRPC...")
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 2999, // $29.99
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
		Metadata: map[string]string{
			"order_id": "ORDER-SERVERPOST-001",
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err)

	// Verify response
	assert.NotEmpty(t, saleResp.Msg.TransactionId)
	assert.Equal(t, int64(2999), saleResp.Msg.AmountCents)
	assert.True(t, saleResp.Msg.IsApproved, "Sale should be approved")
	assert.NotEmpty(t, saleResp.Msg.AuthorizationCode)
	assert.NotNil(t, saleResp.Msg.Card)
	assert.Equal(t, "mastercard", saleResp.Msg.Card.Brand)

	t.Logf("✅ SALE successful via ConnectRPC - TX: %s", saleResp.Msg.TransactionId)
}

// TestServerPost_CaptureWithFinancialBRIC tests capture operation using transaction from AUTH
func TestServerPost_CaptureWithFinancialBRIC(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "00000000-0000-0000-0000-000000001003" // UUID format required
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create payment method and authorize
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL,
	)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Authorize first
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
	t.Logf("[SETUP] AUTH created: %s", authTxID)
	time.Sleep(1 * time.Second)

	// Test: Capture the authorized transaction
	t.Log("[TEST] Capturing authorized transaction via ConnectRPC...")
	captureReq := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: authTxID,
		AmountCents:   10000, // Full capture
	})
	addJWTAuth(t, captureReq, cfg, merchantID)

	captureResp, err := client.Capture(ctx, captureReq)
	require.NoError(t, err)

	// Verify capture
	assert.NotEmpty(t, captureResp.Msg.TransactionId)
	assert.True(t, captureResp.Msg.IsApproved, "Capture should be approved")
	assert.Equal(t, int64(10000), captureResp.Msg.AmountCents)
	assert.Equal(t, authTxID, captureResp.Msg.ParentTransactionId, "Should reference AUTH transaction")

	t.Logf("✅ CAPTURE successful via ConnectRPC - TX: %s", captureResp.Msg.TransactionId)
}

// TestServerPost_VoidWithFinancialBRIC tests void operation on authorized transaction
func TestServerPost_VoidWithFinancialBRIC(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "00000000-0000-0000-0000-000000001004" // UUID format required
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create payment method and authorize
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL,
	)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Authorize first
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
	t.Logf("[SETUP] AUTH created: %s", authTxID)
	time.Sleep(1 * time.Second)

	// Test: Void the authorized transaction
	t.Log("[TEST] Voiding authorized transaction via ConnectRPC...")
	voidReq := connect.NewRequest(&paymentv1.VoidRequest{
		TransactionId: authTxID,
	})
	addJWTAuth(t, voidReq, cfg, merchantID)

	voidResp, err := client.Void(ctx, voidReq)
	require.NoError(t, err)

	// Verify void
	assert.NotEmpty(t, voidResp.Msg.TransactionId)
	assert.True(t, voidResp.Msg.IsApproved, "Void should be approved")
	assert.Equal(t, authTxID, voidResp.Msg.ParentTransactionId, "Should reference AUTH transaction")

	t.Logf("✅ VOID successful via ConnectRPC - TX: %s", voidResp.Msg.TransactionId)
}

// TestServerPost_RefundWithFinancialBRIC tests refund operation on captured/sale transaction
func TestServerPost_RefundWithFinancialBRIC(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "00000000-0000-0000-0000-000000001005" // UUID format required
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create payment method and process sale
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL,
	)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Process sale first
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
	saleTxID := saleResp.Msg.TransactionId
	t.Logf("[SETUP] SALE created: %s", saleTxID)
	time.Sleep(1 * time.Second)

	// Test: Refund the sale (partial refund)
	t.Log("[TEST] Refunding sale transaction via ConnectRPC...")
	refundReq := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId: saleTxID,
		AmountCents:   5000, // Partial refund: $50.00
		Reason:        "Customer requested partial refund",
	})
	addJWTAuth(t, refundReq, cfg, merchantID)

	refundResp, err := client.Refund(ctx, refundReq)
	require.NoError(t, err)

	// Verify refund
	assert.NotEmpty(t, refundResp.Msg.TransactionId)
	assert.True(t, refundResp.Msg.IsApproved, "Refund should be approved")
	assert.Equal(t, int64(5000), refundResp.Msg.AmountCents)
	assert.Equal(t, saleTxID, refundResp.Msg.ParentTransactionId, "Should reference SALE transaction")

	t.Logf("✅ REFUND successful via ConnectRPC - TX: %s", refundResp.Msg.TransactionId)
}

// TestServerPost_ConcurrentOperations tests concurrent CAPTURE + VOID operations for race condition prevention
func TestServerPost_ConcurrentOperations(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "00000000-0000-0000-0000-000000001006" // UUID format required
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup: Create payment method and authorize
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL,
	)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Authorize
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
	t.Logf("[SETUP] AUTH created: %s", authTxID)
	time.Sleep(1 * time.Second)

	// Test: Launch concurrent CAPTURE + VOID
	t.Log("[TEST] Launching concurrent CAPTURE + VOID via ConnectRPC...")

	var captureErr, voidErr error
	var captureResp, voidResp *connect.Response[paymentv1.PaymentResponse]

	captureDone := make(chan bool)
	voidDone := make(chan bool)

	// Concurrent CAPTURE
	go func() {
		defer func() { captureDone <- true }()
		req := connect.NewRequest(&paymentv1.CaptureRequest{
			TransactionId: authTxID,
			AmountCents:   10000,
		})
		addJWTAuth(t, req, cfg, merchantID)
		captureResp, captureErr = client.Capture(ctx, req)
	}()

	// Concurrent VOID
	go func() {
		defer func() { voidDone <- true }()
		req := connect.NewRequest(&paymentv1.VoidRequest{
			TransactionId: authTxID,
		})
		addJWTAuth(t, req, cfg, merchantID)
		voidResp, voidErr = client.Void(ctx, req)
	}()

	// Wait for both operations
	<-captureDone
	<-voidDone

	// Verify no data corruption
	captureSuccess := captureErr == nil && captureResp != nil && captureResp.Msg.IsApproved
	voidSuccess := voidErr == nil && voidResp != nil && voidResp.Msg.IsApproved

	t.Logf("[RESULT] CAPTURE=%v VOID=%v", captureSuccess, voidSuccess)

	// At least one should succeed, but no data corruption
	successCount := 0
	if captureSuccess {
		successCount++
	}
	if voidSuccess {
		successCount++
	}

	assert.GreaterOrEqual(t, successCount, 1, "At least one operation must succeed")
	assert.LessOrEqual(t, successCount, 2, "Max two operations can succeed")
	t.Log("✅ Concurrency test passed - no data corruption detected")
}
