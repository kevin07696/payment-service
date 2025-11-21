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

// TestACH_SaveAccount tests that saving an ACH account creates a pre-note
// and sets verification_status='pending'
func TestACH_SaveAccount(t *testing.T) {
	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "10000000-0000-0000-0000-000000000001"

	time.Sleep(1 * time.Second)

	// Generate JWT token for authentication
	jwtToken := generateJWTToken(t, merchantID)

	// Save ACH account (triggers pre-note CKC0)
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	paymentMethodID, err := testutil.TokenizeAndSaveACH(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

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
	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "10000000-0000-0000-0000-000000000002"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	time.Sleep(1 * time.Second)

	// Generate JWT token for authentication
	jwtToken := generateJWTToken(t, merchantID)

	// Save ACH account (unverified)
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	paymentMethodID, err := testutil.TokenizeAndSaveACH(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Try to make payment with unverified ACH account
	idempotencyKey := uuid.New().String()
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 10000, // $100.00
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
		IdempotencyKey: idempotencyKey,
		Metadata: map[string]string{
			"test_case": "unverified_ach_blocked",
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	_, err = client.Sale(ctx, saleReq)

	// Should be rejected
	require.Error(t, err, "Payment should be rejected for unverified ACH account")

	// Verify error message mentions verification requirement
	assert.Contains(t, err.Error(), "verified",
		"Error should mention verification requirement")

	t.Logf("✅ Unverified ACH payment correctly blocked - Error: %v", err)
}

// TestACH_AllowVerifiedPayments tests that verified ACH accounts can be used for payments
func TestACH_AllowVerifiedPayments(t *testing.T) {
	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "10000000-0000-0000-0000-000000000003"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	time.Sleep(1 * time.Second)

	// Generate JWT token for authentication
	jwtToken := generateJWTToken(t, merchantID)

	// Save ACH account
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	paymentMethodID, err := testutil.TokenizeAndSaveACH(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

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
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 25000, // $250.00
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
		IdempotencyKey: idempotencyKey,
		Metadata: map[string]string{
			"test_case": "verified_ach_allowed",
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err, "Payment should succeed for verified ACH account")

	assert.True(t, saleResp.Msg.IsApproved, "Transaction should be approved")
	assert.Equal(t, int64(25000), saleResp.Msg.AmountCents, "Amount should be 25000 cents ($250.00)")

	t.Logf("✅ Verified ACH payment approved - Transaction ID: %s", saleResp.Msg.TransactionId)
}

// TestACH_HighValuePayments tests that even verified ACH can handle high-value transactions
func TestACH_HighValuePayments(t *testing.T) {
	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	customerID := "10000000-0000-0000-0000-000000000005"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	time.Sleep(1 * time.Second)

	// Generate JWT token for authentication
	jwtToken := generateJWTToken(t, merchantID)

	// Save and verify ACH account
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	paymentMethodID, err := testutil.TokenizeAndSaveACH(cfg, testClient, jwtToken, merchantID, customerID, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Mark as verified
	db := testutil.GetDB(t)
	err = testutil.MarkACHAsVerified(db, paymentMethodID)
	require.NoError(t, err)

	// Try high-value payment ($2,500)
	idempotencyKey := uuid.New().String()
	saleReq := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  merchantID,
		CustomerId:  customerID,
		AmountCents: 250000, // $2,500.00
		Currency:    "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
			PaymentMethodId: paymentMethodID,
		},
		IdempotencyKey: idempotencyKey,
		Metadata: map[string]string{
			"test_case": "high_value_verified_ach",
		},
	})
	addJWTAuth(t, saleReq, cfg, merchantID)

	saleResp, err := client.Sale(ctx, saleReq)
	require.NoError(t, err, "High-value payment should succeed for verified ACH")

	assert.True(t, saleResp.Msg.IsApproved)
	assert.Equal(t, int64(250000), saleResp.Msg.AmountCents, "Amount should be 250000 cents ($2,500.00)")

	t.Log("✅ High-value ACH payment approved - $2,500 transaction")
}
