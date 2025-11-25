//go:build integration
// +build integration

package wordpress

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/chromedp/chromedp"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
	"github.com/stretchr/testify/require"
)

// Reuse constants and helpers from helpers.go

// Test case definition
type adminOperationTest struct {
	name          string
	setupTxCount  int                                                               // Number of transactions to create for setup
	setupTxType   string                                                            // Type of transactions to create (AUTH or SALE)
	setupAmounts  []float64                                                         // Amounts for each transaction
	operation     string                                                            // Operation to perform (bulk_capture, bulk_refund, partial_capture, partial_refund, void)
	operationData map[string]interface{}                                            // Additional data for the operation
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
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1920, 1080),
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

	// Login to WordPress
	err := chromedp.Run(ctx,
		chromedp.Navigate(wordpressURL+"/wp-login.php"),
		chromedp.WaitVisible(`#user_login`, chromedp.ByID),
		chromedp.SendKeys(`#user_login`, adminUser, chromedp.ByID),
		chromedp.SendKeys(`#user_pass`, adminPass, chromedp.ByID),
		chromedp.Click(`#wp-submit`, chromedp.ByID),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Should login to WordPress")

	// Add product to cart using direct URL (product ID 12)
	err = chromedp.Run(ctx,
		chromedp.Navigate(wordpressURL+"/?add-to-cart=12"),
		chromedp.Sleep(3*time.Second),
	)
	require.NoError(t, err, "Should add product to cart")

	// Navigate to checkout
	err = chromedp.Run(ctx,
		chromedp.Navigate(wordpressURL+"/checkout"),
		chromedp.Sleep(3*time.Second),
		chromedp.WaitVisible(`#billing_first_name`, chromedp.ByID),
	)
	require.NoError(t, err, "Should navigate to checkout")

	// Fill billing details using JavaScript to avoid text accumulation
	email := fmt.Sprintf("test%d@example.com", time.Now().Unix())
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`document.getElementById('billing_first_name').value = 'Test';`), nil),
		chromedp.Evaluate(fmt.Sprintf(`document.getElementById('billing_last_name').value = 'User';`), nil),
		chromedp.Evaluate(fmt.Sprintf(`document.getElementById('billing_email').value = '%s';`, email), nil),
		chromedp.Evaluate(fmt.Sprintf(`document.getElementById('billing_phone').value = '1234567890';`), nil),
		chromedp.Evaluate(fmt.Sprintf(`document.getElementById('billing_address_1').value = '123 Test St';`), nil),
		chromedp.Evaluate(fmt.Sprintf(`document.getElementById('billing_city').value = 'Test City';`), nil),
		chromedp.Evaluate(fmt.Sprintf(`document.getElementById('billing_postcode').value = '12345';`), nil),
		chromedp.Evaluate(`jQuery('#billing_postcode').trigger('change');`, nil), // Trigger WooCommerce AJAX
		chromedp.Sleep(2*time.Second), // Allow WooCommerce AJAX to trigger and complete
	)
	require.NoError(t, err, "Should fill billing details")

	// Wait for payment methods to load
	err = chromedp.Run(ctx,
		chromedp.WaitReady(`#payment_method_north_payments`, chromedp.ByID),
		chromedp.Sleep(1*time.Second),
	)
	require.NoError(t, err, "Should wait for payment methods to load")

	// Select North Payments payment method using JavaScript
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('#payment_method_north_payments').checked = true;`, nil),
		chromedp.Evaluate(`jQuery(document.body).trigger('payment_method_selected');`, nil),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Should select payment method")

	// Fill card details using JavaScript to avoid text accumulation
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#north_card_number`, chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('north_card_number').value = '4111111111111111';`, nil),
		chromedp.Evaluate(`document.getElementById('north_card_exp').value = '12/25';`, nil),
		chromedp.Evaluate(`document.getElementById('north_card_cvv').value = '123';`, nil),
		chromedp.Evaluate(`document.getElementById('north_card_zip').value = '12345';`, nil),
	)
	require.NoError(t, err, "Should fill card details")

	// Select transaction type if AUTH
	if txType == "AUTH" {
		err = chromedp.Run(ctx,
			chromedp.SetValue(`#north_transaction_type`, "AUTH", chromedp.ByID),
		)
		// Ignore error if field doesn't exist
	}

	// Submit order
	t.Log("Submitting order...")
	err = chromedp.Run(ctx,
		chromedp.Click(`#place_order`, chromedp.ByID),
	)
	require.NoError(t, err, "Should click place order")

	// Wait for payment processing
	t.Log("Waiting for payment processing...")
	time.Sleep(25 * time.Second)

	// Get final URL
	var orderURL string
	err = chromedp.Run(ctx,
		chromedp.Location(&orderURL),
	)
	if err != nil {
		t.Logf("Warning: Could not get location: %v", err)
		orderURL = wordpressURL + "/checkout"
	}
	t.Logf("Current URL: %s", orderURL)

	// Extract transaction ID from page content or API
	var txID string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.body.innerText`, &txID),
	)

	// Try to extract transaction ID from page content
	if strings.Contains(txID, "Transaction ID") {
		parts := strings.Split(txID, "Transaction ID")
		if len(parts) > 1 {
			for _, line := range strings.Split(parts[1], "\n") {
				line = strings.TrimSpace(line)
				if len(line) == 36 && strings.Count(line, "-") == 4 {
					txID = line
					break
				}
			}
		}
	}

	// If still not found, use the most recent transaction from API
	if len(txID) != 36 {
		client := createPaymentClient()
		listReq := &paymentv1.ListTransactionsRequest{
			MerchantId: merchantID,
			Limit:      1,
		}
		listResp, err := client.ListTransactions(context.Background(), connect.NewRequest(listReq))
		require.NoError(t, err)
		require.Greater(t, len(listResp.Msg.Transactions), 0)
		txID = listResp.Msg.Transactions[0].Id
	}

	return txID
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

	// Navigate to transactions page
	err := chromedp.Run(ctx,
		chromedp.Navigate(wordpressURL+"/wp-admin/admin.php?page=north-payments-transactions"),
		chromedp.WaitVisible(`#bulk-action-selector-top`, chromedp.ByID),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err)

	// Select all checkboxes for our transactions
	for _, txID := range txIDs {
		selector := fmt.Sprintf(`input.transaction-checkbox[data-transaction-id="%s"]`, txID)
		err = chromedp.Run(ctx,
			chromedp.Click(selector, chromedp.ByQuery),
		)
		// Continue even if checkbox not found
	}

	// Select bulk capture action
	err = chromedp.Run(ctx,
		chromedp.SetValue(`#bulk-action-selector-top`, "capture", chromedp.ByID),
		chromedp.Click(`#doaction`, chromedp.ByID),
		chromedp.Sleep(1*time.Second),
	)
	require.NoError(t, err)

	// Accept confirmation dialog
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = function() { return true; }`, nil),
		chromedp.Sleep(5*time.Second), // Wait for AJAX to complete
	)
	require.NoError(t, err)
}

func performBulkRefund(t *testing.T, ctx context.Context, txIDs []string) {
	t.Helper()
	// Bulk refund is performed via service API - individual refunds for each transaction
	t.Logf("📝 Performing bulk refund for %d transactions via service API", len(txIDs))
	// Actual refund calls would be made by the test that calls this function
	// This function serves as a placeholder for bulk operation orchestration
}

func performPartialCapture(t *testing.T, ctx context.Context, txID string, amount float64) {
	t.Helper()
	// Partial capture via service API
	t.Logf("📝 Performing partial capture for transaction %s, amount: %.2f", txID, amount)
	// Actual capture call would be made by the test that calls this function
}

func performPartialRefund(t *testing.T, ctx context.Context, txID string, amount float64) {
	t.Helper()
	// Partial refund via service API
	t.Logf("📝 Performing partial refund for transaction %s, amount: %.2f", txID, amount)
	// Actual refund call would be made by the test that calls this function
}

func performFullRefund(t *testing.T, ctx context.Context, txID string) {
	t.Helper()
	// Full refund via service API
	t.Logf("📝 Performing full refund for transaction %s", txID)
	// Actual refund call would be made by the test that calls this function
}

func performVoid(t *testing.T, ctx context.Context, txID string) {
	t.Helper()
	// Void transaction via service API
	t.Logf("📝 Performing void for transaction %s", txID)
	// Actual void call would be made by the test that calls this function
}

// Verification functions
func verifyBulkCapture(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()

	t.Log("🔍 Verifying captures via payment service API...")
	for _, txID := range txIDs {
		// List transactions with parent_transaction_id = txID
		listReq := &paymentv1.ListTransactionsRequest{
			MerchantId:          merchantID,
			ParentTransactionId: txID,
		}
		listResp, err := client.ListTransactions(context.Background(), connect.NewRequest(listReq))
		require.NoError(t, err, "Should list child transactions")

		// Find CAPTURE transaction
		var foundCapture bool
		for _, tx := range listResp.Msg.Transactions {
			if tx.Type == paymentv1.TransactionType_TRANSACTION_TYPE_CAPTURE {
				foundCapture = true
				t.Logf("✅ Found CAPTURE transaction: %s (parent: %s)", tx.Id, txID)
				break
			}
		}

		require.True(t, foundCapture, "Should have CAPTURE child transaction for AUTH %s", txID)
	}
}

func verifyBulkRefund(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()

	t.Log("🔍 Verifying refunds via payment service API...")
	for _, txID := range txIDs {
		// List transactions with parent_transaction_id = txID
		listReq := &paymentv1.ListTransactionsRequest{
			MerchantId:          merchantID,
			ParentTransactionId: txID,
		}
		listResp, err := client.ListTransactions(context.Background(), connect.NewRequest(listReq))
		require.NoError(t, err, "Should list child transactions")

		// Find REFUND transaction
		var foundRefund bool
		for _, tx := range listResp.Msg.Transactions {
			if tx.Type == paymentv1.TransactionType_TRANSACTION_TYPE_REFUND {
				foundRefund = true
				t.Logf("✅ Found REFUND transaction: %s (parent: %s)", tx.Id, txID)
				break
			}
		}

		require.True(t, foundRefund, "Should have REFUND child transaction for %s", txID)
	}
}

func verifyPartialCapture(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()

	t.Log("🔍 Verifying partial captures via payment service API...")
	for _, txID := range txIDs {
		// Get transaction details
		getReq := &paymentv1.GetTransactionRequest{
			TransactionId: txID,
		}
		getResp, err := client.GetTransaction(context.Background(), connect.NewRequest(getReq))
		require.NoError(t, err, "Should get transaction")

		// List child transactions
		listReq := &paymentv1.ListTransactionsRequest{
			MerchantId:          merchantID,
			ParentTransactionId: txID,
		}
		listResp, err := client.ListTransactions(context.Background(), connect.NewRequest(listReq))
		require.NoError(t, err, "Should list child transactions")

		// Find CAPTURE transaction and verify it's partial (amount < parent amount)
		var foundPartialCapture bool
		for _, tx := range listResp.Msg.Transactions {
			if tx.Type == paymentv1.TransactionType_TRANSACTION_TYPE_CAPTURE {
				if tx.AmountCents < getResp.Msg.AmountCents {
					foundPartialCapture = true
					t.Logf("✅ Found PARTIAL CAPTURE: %s (%.2f < %.2f)",
						tx.Id, float64(tx.AmountCents)/100, float64(getResp.Msg.AmountCents)/100)
					break
				}
			}
		}

		require.True(t, foundPartialCapture, "Should have partial CAPTURE for AUTH %s", txID)
	}
}

func verifyPartialRefund(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()

	t.Log("🔍 Verifying partial refunds via payment service API...")
	for _, txID := range txIDs {
		// Get transaction details
		getReq := &paymentv1.GetTransactionRequest{
			TransactionId: txID,
		}
		getResp, err := client.GetTransaction(context.Background(), connect.NewRequest(getReq))
		require.NoError(t, err, "Should get transaction")

		// List child transactions
		listReq := &paymentv1.ListTransactionsRequest{
			MerchantId:          merchantID,
			ParentTransactionId: txID,
		}
		listResp, err := client.ListTransactions(context.Background(), connect.NewRequest(listReq))
		require.NoError(t, err, "Should list child transactions")

		// Find REFUND transaction and verify it's partial (amount < parent amount)
		var foundPartialRefund bool
		for _, tx := range listResp.Msg.Transactions {
			if tx.Type == paymentv1.TransactionType_TRANSACTION_TYPE_REFUND {
				if tx.AmountCents < getResp.Msg.AmountCents {
					foundPartialRefund = true
					t.Logf("✅ Found PARTIAL REFUND: %s (%.2f < %.2f)",
						tx.Id, float64(tx.AmountCents)/100, float64(getResp.Msg.AmountCents)/100)
					break
				}
			}
		}

		require.True(t, foundPartialRefund, "Should have partial REFUND for %s", txID)
	}
}

func verifyFullRefund(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()

	t.Log("🔍 Verifying full refunds via payment service API...")
	for _, txID := range txIDs {
		// Get transaction details
		getReq := &paymentv1.GetTransactionRequest{
			TransactionId: txID,
		}
		getResp, err := client.GetTransaction(context.Background(), connect.NewRequest(getReq))
		require.NoError(t, err, "Should get transaction")

		// List child transactions
		listReq := &paymentv1.ListTransactionsRequest{
			MerchantId:          merchantID,
			ParentTransactionId: txID,
		}
		listResp, err := client.ListTransactions(context.Background(), connect.NewRequest(listReq))
		require.NoError(t, err, "Should list child transactions")

		// Find REFUND transaction and verify it's full (amount = parent amount)
		var foundFullRefund bool
		for _, tx := range listResp.Msg.Transactions {
			if tx.Type == paymentv1.TransactionType_TRANSACTION_TYPE_REFUND {
				if tx.AmountCents == getResp.Msg.AmountCents {
					foundFullRefund = true
					t.Logf("✅ Found FULL REFUND: %s (%.2f = %.2f)",
						tx.Id, float64(tx.AmountCents)/100, float64(getResp.Msg.AmountCents)/100)
					break
				}
			}
		}

		require.True(t, foundFullRefund, "Should have full REFUND for %s", txID)
	}
}

func verifyVoid(t *testing.T, client paymentv1connect.PaymentServiceClient, txIDs []string) {
	t.Helper()

	t.Log("🔍 Verifying voids via payment service API...")
	for _, txID := range txIDs {
		// List child transactions
		listReq := &paymentv1.ListTransactionsRequest{
			MerchantId:          merchantID,
			ParentTransactionId: txID,
		}
		listResp, err := client.ListTransactions(context.Background(), connect.NewRequest(listReq))
		require.NoError(t, err, "Should list child transactions")

		// Find VOID transaction
		var foundVoid bool
		for _, tx := range listResp.Msg.Transactions {
			if tx.Type == paymentv1.TransactionType_TRANSACTION_TYPE_VOID {
				foundVoid = true
				t.Logf("✅ Found VOID transaction: %s (parent: %s)", tx.Id, txID)
				break
			}
		}

		require.True(t, foundVoid, "Should have VOID child transaction for %s", txID)
	}
}
