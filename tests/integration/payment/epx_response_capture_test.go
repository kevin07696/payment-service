//go:build integration
// +build integration

package payment_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/require"
)

// TestEPX_CaptureAuthRespTextValues runs all Server Post operations and captures
// the actual AUTH_RESP_TEXT values returned by EPX for certification documentation
func TestEPX_CaptureAuthRespTextValues(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, cfg.ServiceURL)
	merchantID := "00000000-0000-0000-0000-000000000001"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Setup: Create payment method via Browser Post STORAGE for all tests
	testClient := &testutil.Client{BaseURL: cfg.ServiceURL, HTTPClient: httpClient, Headers: make(map[string]string)}
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := cfg.CallbackBaseURL

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("EPX SERVER POST API - AUTH_RESP_TEXT VALUES FOR CERTIFICATION")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	// Test 1: AUTH operation
	t.Run("Capture_AUTH_Response", func(t *testing.T) {
		customerID := uuid.NewString()

		paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
			t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL,
		)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
			MerchantId:     merchantID,
			CustomerId:     customerID,
			AmountCents:    10000, // $100.00
			Currency:       "USD",
			IdempotencyKey: uuid.NewString(),
			PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
				PaymentMethodId: paymentMethodID,
			},
		})
		addJWTAuth(t, authReq, cfg, merchantID)

		authResp, err := client.Authorize(ctx, authReq)
		require.NoError(t, err)

		fmt.Printf("AUTH Operation:\n")
		fmt.Printf("  Transaction ID: %s\n", authResp.Msg.TransactionId)
		fmt.Printf("  Is Approved: %v\n", authResp.Msg.IsApproved)
		fmt.Printf("  Authorization Code: %s\n", authResp.Msg.AuthorizationCode)
		fmt.Printf("  Message: %s\n", authResp.Msg.Message)
		fmt.Printf("  AUTH_RESP_TEXT (EPX): %s\n", authResp.Msg.Message)
		fmt.Println()
	})

	// Test 2: SALE operation
	t.Run("Capture_SALE_Response", func(t *testing.T) {
		customerID := uuid.NewString()

		paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
			t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestMastercardCard, callbackBaseURL,
		)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		saleReq := connect.NewRequest(&paymentv1.SaleRequest{
			MerchantId:     merchantID,
			CustomerId:     customerID,
			AmountCents:    5000, // $50.00
			Currency:       "USD",
			IdempotencyKey: uuid.NewString(),
			PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
				PaymentMethodId: paymentMethodID,
			},
		})
		addJWTAuth(t, saleReq, cfg, merchantID)

		saleResp, err := client.Sale(ctx, saleReq)
		require.NoError(t, err)

		fmt.Printf("SALE Operation:\n")
		fmt.Printf("  Transaction ID: %s\n", saleResp.Msg.TransactionId)
		fmt.Printf("  Is Approved: %v\n", saleResp.Msg.IsApproved)
		fmt.Printf("  Authorization Code: %s\n", saleResp.Msg.AuthorizationCode)
		fmt.Printf("  Message: %s\n", saleResp.Msg.Message)
		fmt.Printf("  AUTH_RESP_TEXT (EPX): %s\n", saleResp.Msg.Message)
		fmt.Println()
	})

	// Test 3: CAPTURE operation
	t.Run("Capture_CAPTURE_Response", func(t *testing.T) {
		customerID := uuid.NewString()

		paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
			t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL,
		)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// First do AUTH
		authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
			MerchantId:     merchantID,
			CustomerId:     customerID,
			AmountCents:    7500, // $75.00
			Currency:       "USD",
			IdempotencyKey: uuid.NewString(),
			PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
				PaymentMethodId: paymentMethodID,
			},
		})
		addJWTAuth(t, authReq, cfg, merchantID)

		authResp, err := client.Authorize(ctx, authReq)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// Then CAPTURE
		captureReq := connect.NewRequest(&paymentv1.CaptureRequest{
			TransactionId: authResp.Msg.TransactionId,
			AmountCents:   7500, // Full capture
		})
		addJWTAuth(t, captureReq, cfg, merchantID)

		captureResp, err := client.Capture(ctx, captureReq)
		require.NoError(t, err)

		fmt.Printf("CAPTURE Operation:\n")
		fmt.Printf("  Transaction ID: %s\n", captureResp.Msg.TransactionId)
		fmt.Printf("  Parent Transaction ID: %s\n", captureResp.Msg.ParentTransactionId)
		fmt.Printf("  Is Approved: %v\n", captureResp.Msg.IsApproved)
		fmt.Printf("  Authorization Code: %s\n", captureResp.Msg.AuthorizationCode)
		fmt.Printf("  Message: %s\n", captureResp.Msg.Message)
		fmt.Printf("  AUTH_RESP_TEXT (EPX): %s\n", captureResp.Msg.Message)
		fmt.Println()
	})

	// Test 4: VOID operation
	t.Run("Capture_VOID_Response", func(t *testing.T) {
		customerID := uuid.NewString()

		paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
			t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL,
		)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// First do AUTH
		authReq := connect.NewRequest(&paymentv1.AuthorizeRequest{
			MerchantId:     merchantID,
			CustomerId:     customerID,
			AmountCents:    6000, // $60.00
			Currency:       "USD",
			IdempotencyKey: uuid.NewString(),
			PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{
				PaymentMethodId: paymentMethodID,
			},
		})
		addJWTAuth(t, authReq, cfg, merchantID)

		authResp, err := client.Authorize(ctx, authReq)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// Then VOID
		voidReq := connect.NewRequest(&paymentv1.VoidRequest{
			TransactionId: authResp.Msg.TransactionId,
		})
		addJWTAuth(t, voidReq, cfg, merchantID)

		voidResp, err := client.Void(ctx, voidReq)
		require.NoError(t, err)

		fmt.Printf("VOID Operation:\n")
		fmt.Printf("  Transaction ID: %s\n", voidResp.Msg.TransactionId)
		fmt.Printf("  Parent Transaction ID: %s\n", voidResp.Msg.ParentTransactionId)
		fmt.Printf("  Is Approved: %v\n", voidResp.Msg.IsApproved)
		fmt.Printf("  Authorization Code: %s\n", voidResp.Msg.AuthorizationCode)
		fmt.Printf("  Message: %s\n", voidResp.Msg.Message)
		fmt.Printf("  AUTH_RESP_TEXT (EPX): %s\n", voidResp.Msg.Message)
		fmt.Println()
	})

	// Test 5: REFUND operation
	t.Run("Capture_REFUND_Response", func(t *testing.T) {
		customerID := uuid.NewString()

		paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
			t, cfg, testClient, jwtToken, merchantID, customerID, testutil.TestMastercardCard, callbackBaseURL,
		)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// First do SALE
		saleReq := connect.NewRequest(&paymentv1.SaleRequest{
			MerchantId:     merchantID,
			CustomerId:     customerID,
			AmountCents:    12000, // $120.00
			Currency:       "USD",
			IdempotencyKey: uuid.NewString(),
			PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{
				PaymentMethodId: paymentMethodID,
			},
		})
		addJWTAuth(t, saleReq, cfg, merchantID)

		saleResp, err := client.Sale(ctx, saleReq)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// Then REFUND
		refundReq := connect.NewRequest(&paymentv1.RefundRequest{
			TransactionId: saleResp.Msg.TransactionId,
			AmountCents:   3000, // Partial refund: $30.00
			Reason:        "Customer requested refund",
		})
		addJWTAuth(t, refundReq, cfg, merchantID)

		refundResp, err := client.Refund(ctx, refundReq)
		require.NoError(t, err)

		fmt.Printf("REFUND Operation:\n")
		fmt.Printf("  Transaction ID: %s\n", refundResp.Msg.TransactionId)
		fmt.Printf("  Parent Transaction ID: %s\n", refundResp.Msg.ParentTransactionId)
		fmt.Printf("  Is Approved: %v\n", refundResp.Msg.IsApproved)
		fmt.Printf("  Authorization Code: %s\n", refundResp.Msg.AuthorizationCode)
		fmt.Printf("  Message: %s\n", refundResp.Msg.Message)
		fmt.Printf("  AUTH_RESP_TEXT (EPX): %s\n", refundResp.Msg.Message)
		fmt.Println()
	})

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("END OF EPX AUTH_RESP_TEXT CAPTURE")
	fmt.Println(strings.Repeat("=", 80) + "\n")
}
