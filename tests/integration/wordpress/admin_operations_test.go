//go:build integration
// +build integration

package wordpress

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
	"github.com/stretchr/testify/require"
)

// Reuse constants and helpers from helpers.go

// Test case definition
type adminOperationTest struct {
	name          string
	setupTxCount  int              // Number of transactions to create for setup
	setupTxType   string           // Type of transactions to create (AUTH or SALE)
	setupAmounts  []float64        // Amounts for each transaction
	operation     string           // Operation to perform (bulk_capture, bulk_refund, partial_capture, partial_refund, void)
	operationData map[string]interface{} // Additional data for the operation
	verifyFunc    func(*testing.T, paymentv1connect.PaymentServiceClient, []string) // Verification function
}

// TestWordPressAdminOperations tests all WordPress admin transaction operations
func TestWordPressAdminOperations(t *testing.T) {
	tests := []adminOperationTest{
		{
			name:         "Bulk Capture - 2 AUTH Transactions",
			setupTxCount: 2,
			setupTxType:  "AUTH",
			setupAmounts: []float64{50.00, 75.00},
			operation:    "bulk_capture",
			verifyFunc:   verifyBulkCapture,
		},
		{
			name:         "Bulk Refund - 2 SALE Transactions",
			setupTxCount: 2,
			setupTxType:  "SALE",
			setupAmounts: []float64{100.00, 150.00},
			operation:    "bulk_refund",
			verifyFunc:   verifyBulkRefund,
		},
		{
			name:         "Partial Capture - 1 AUTH Transaction",
			setupTxCount: 1,
			setupTxType:  "AUTH",
			setupAmounts: []float64{100.00},
			operation:    "partial_capture",
			operationData: map[string]interface{}{
				"capture_amount": 50.00, // Capture $50 of $100 auth
			},
			verifyFunc: verifyPartialCapture,
		},
		{
			name:         "Partial Refund - 1 SALE Transaction",
			setupTxCount: 1,
			setupTxType:  "SALE",
			setupAmounts: []float64{200.00},
			operation:    "partial_refund",
			operationData: map[string]interface{}{
				"refund_amount": 75.00, // Refund $75 of $200 sale
			},
			verifyFunc: verifyPartialRefund,
		},
		{
			name:         "SALE and Full Refund",
			setupTxCount: 1,
			setupTxType:  "SALE",
			setupAmounts: []float64{125.00},
			operation:    "full_refund",
			verifyFunc:   verifyFullRefund,
		},
		{
			name:         "AUTH and Void",
			setupTxCount: 1,
			setupTxType:  "AUTH",
			setupAmounts: []float64{99.00},
			operation:    "void",
			verifyFunc:   verifyVoid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("🧪 Running test: %s", tt.name)

			// Create payment client
			client := createPaymentClient()

			// Setup: Create transactions via WordPress checkout
			txIDs := setupTransactions(t, tt.setupTxCount, tt.setupTxType, tt.setupAmounts)
			require.Len(t, txIDs, tt.setupTxCount, "Should create required number of transactions")

			// Verify transactions exist in payment service
			verifyTransactionsExist(t, client, txIDs)

			// Perform WordPress admin operation
			performAdminOperation(t, tt.operation, txIDs, tt.operationData)

			// Wait for operation to process
			time.Sleep(2 * time.Second)

			// Verify operation results via payment service API
			tt.verifyFunc(t, client, txIDs)

			t.Logf("✅ Test passed: %s", tt.name)
		})
	}
}

// setupTransactions creates test transactions via WordPress checkout
func setupTransactions(t *testing.T, count int, txType string, amounts []float64) []string {
	t.Helper()

	t.Logf("📦 Setting up %d %s transaction(s)...", count, txType)

	// Create Chrome context for checkout
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-cache", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	txIDs := make([]string, 0, count)

	for i := 0; i < count; i++ {
		amount := amounts[i]
		t.Logf("   Creating transaction %d/%d: %s $%.2f", i+1, count, txType, amount)

		// Perform checkout
		txID := performCheckout(t, ctx, txType, amount)
		txIDs = append(txIDs, txID)

		t.Logf("   ✅ Created transaction: %s", txID)

		// Small delay between transactions
		if i < count-1 {
			time.Sleep(1 * time.Second)
		}
	}

	t.Logf("✅ Setup complete: %d transaction(s) created", count)
	return txIDs
}

// performCheckout performs a WordPress checkout and returns the transaction ID
func performCheckout(t *testing.T, ctx context.Context, txType string, amount float64) string {
	t.Helper()

	// TODO: Implement automated checkout flow
	// For now, this is a placeholder that would need to be implemented
	// based on the actual WordPress checkout process

	// This would involve:
	// 1. Navigate to product page
	// 2. Add to cart
	// 3. Go to checkout
	// 4. Fill billing details
	// 5. Select transaction type (AUTH vs SALE)
	// 6. Fill card details
	// 7. Submit order
	// 8. Capture transaction ID from response

	// Placeholder: generate a mock transaction ID
	return uuid.New().String()
}

// verifyTransactionsExist verifies transactions exist in payment service via API
func verifyTransactionsExist(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()

	t.Log("🔍 Verifying transactions exist in payment service...")

	for _, txID := range txIDs {
		req := &paymentv1.GetTransactionRequest{
			TransactionId: txID,
		}

		resp, err := client.GetTransaction(context.Background(), connect.NewRequest(req))
		require.NoError(t, err, "Transaction %s should exist", txID)
		require.NotNil(t, resp.Msg, "Transaction response should not be nil")
		require.Equal(t, txID, resp.Msg.Id, "Transaction ID should match")
		_ = resp // Mark as used

		t.Logf("   ✅ Transaction %s exists", txID)
	}

	t.Log("✅ All transactions verified in payment service")
}

// performAdminOperation performs a WordPress admin operation on transactions
func performAdminOperation(t *testing.T, operation string, txIDs []string, data map[string]interface{}) {
	t.Helper()

	t.Logf("⚙️  Performing WordPress admin operation: %s", operation)

	// Create Chrome context for admin operations
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Login to WordPress admin
	err := chromedp.Run(ctx,
		chromedp.Navigate(wordpressURL+"/wp-login.php"),
		chromedp.WaitVisible(`#user_login`, chromedp.ByID),
		chromedp.SendKeys(`#user_login`, adminUser, chromedp.ByID),
		chromedp.SendKeys(`#user_pass`, adminPass, chromedp.ByID),
		chromedp.Click(`#wp-submit`, chromedp.ByID),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Should login to WordPress admin")

	// Navigate to transactions page
	err = chromedp.Run(ctx,
		chromedp.Navigate(wordpressURL+"/wp-admin/admin.php?page=north-payments-transactions"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Should navigate to transactions page")

	// Perform operation based on type
	switch operation {
	case "bulk_capture":
		performBulkCapture(t, ctx, txIDs)
	case "bulk_refund":
		performBulkRefund(t, ctx, txIDs)
	case "partial_capture":
		performPartialCapture(t, ctx, txIDs[0], data["capture_amount"].(float64))
	case "partial_refund":
		performPartialRefund(t, ctx, txIDs[0], data["refund_amount"].(float64))
	case "full_refund":
		performFullRefund(t, ctx, txIDs[0])
	case "void":
		performVoid(t, ctx, txIDs[0])
	default:
		t.Fatalf("Unknown operation: %s", operation)
	}

	t.Logf("✅ WordPress admin operation completed: %s", operation)
}

// WordPress admin operation implementations
func performBulkCapture(t *testing.T, ctx context.Context, txIDs []string) {
	t.Helper()
	// TODO: Implement bulk capture UI automation
	// 1. Select checkboxes for txIDs
	// 2. Select "Capture" from bulk actions dropdown
	// 3. Click Apply
	// 4. Confirm action
}

func performBulkRefund(t *testing.T, ctx context.Context, txIDs []string) {
	t.Helper()
	// TODO: Implement bulk refund UI automation
}

func performPartialCapture(t *testing.T, ctx context.Context, txID string, amount float64) {
	t.Helper()
	// TODO: Implement partial capture UI automation
}

func performPartialRefund(t *testing.T, ctx context.Context, txID string, amount float64) {
	t.Helper()
	// TODO: Implement partial refund UI automation
}

func performFullRefund(t *testing.T, ctx context.Context, txID string) {
	t.Helper()
	// TODO: Implement full refund UI automation
}

func performVoid(t *testing.T, ctx context.Context, txID string) {
	t.Helper()
	// TODO: Implement void UI automation
}

// Verification functions
func verifyBulkCapture(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()

	for _, txID := range txIDs {
		req := &paymentv1.GetTransactionRequest{TransactionId: txID}
		resp, err := client.GetTransaction(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)
		_ = resp // Mark as used

		// Verify that a CAPTURE transaction was created
		// The original AUTH should still exist, and there should be a child CAPTURE transaction
		t.Logf("   Verifying capture for transaction: %s", txID)
		// TODO: Add actual verification logic
	}
}

func verifyBulkRefund(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()
	// TODO: Implement verification
}

func verifyPartialCapture(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()
	// TODO: Implement verification
}

func verifyPartialRefund(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()
	// TODO: Implement verification
}

func verifyFullRefund(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()
	// TODO: Implement verification
}

func verifyVoid(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()
	// TODO: Implement verification
}
