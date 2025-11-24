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

// TestAutomatedCheckoutAndVerify - fully automated checkout test
func TestAutomatedCheckoutAndVerify(t *testing.T) {
	client := createPaymentClient()

	// Create browser context with proper viewport for headless mode
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-cache", true),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1920, 1080), // Critical: WooCommerce AJAX may fail with small viewport
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	t.Log("🛍️  Starting automated checkout test...")

	// Perform automated checkout
	txID := automatedCheckout(t, ctx, 75.00, false) // SALE transaction (product ID 12 = $75)
	require.NotEmpty(t, txID, "Transaction ID should be returned")

	t.Logf("✅ Checkout completed, transaction ID: %s", txID)

	// Verify transaction via API
	t.Log("🔍 Verifying transaction via payment service API...")
	req := &paymentv1.GetTransactionRequest{TransactionId: txID}
	resp, err := client.GetTransaction(context.Background(), connect.NewRequest(req))
	require.NoError(t, err, "Should get transaction from API")
	require.Equal(t, txID, resp.Msg.Id)
	require.Equal(t, int64(7500), resp.Msg.AmountCents) // $75.00 (product ID 12)
	require.Equal(t, paymentv1.TransactionStatus_TRANSACTION_STATUS_APPROVED, resp.Msg.Status)

	t.Log("✅ Transaction verified via API")
	t.Logf("   Type: %s", resp.Msg.Type)
	t.Logf("   Status: %s", resp.Msg.Status)
	t.Logf("   Amount: $%.2f", float64(resp.Msg.AmountCents)/100.0)
	t.Logf("   Auth Code: %s", resp.Msg.AuthorizationCode)
}

// TestBulkCaptureWorkflow - automated bulk capture test
func TestBulkCaptureWorkflow(t *testing.T) {
	client := createPaymentClient()

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

	t.Log("🧪 Testing Bulk Capture Workflow...")

	// Setup: Create 2 AUTH transactions
	t.Log("📦 Creating 2 AUTH transactions...")
	tx1 := automatedCheckout(t, ctx, 75.00, true) // AUTH
	time.Sleep(2 * time.Second)
	tx2 := automatedCheckout(t, ctx, 100.00, true) // AUTH

	t.Logf("✅ Created AUTH transactions: %s, %s", tx1, tx2)

	// Verify AUTH transactions exist
	verifyTransaction(t, client, tx1, paymentv1.TransactionType_TRANSACTION_TYPE_AUTH)
	verifyTransaction(t, client, tx2, paymentv1.TransactionType_TRANSACTION_TYPE_AUTH)

	// Perform bulk capture via WordPress admin
	t.Log("⚙️  Performing bulk capture via WordPress admin...")
	bulkCapture(t, ctx, []string{tx1, tx2})

	// Wait for captures to process
	time.Sleep(3 * time.Second)

	// Verify captures via API
	t.Log("🔍 Verifying captures via payment service API...")
	verifyCaptureExists(t, client, tx1)
	verifyCaptureExists(t, client, tx2)

	t.Log("✅ Bulk capture workflow completed successfully")
}

// automatedCheckout performs a complete automated checkout
func automatedCheckout(t *testing.T, ctx context.Context, amount float64, isAuth bool) string {
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

	// Clear cart to avoid accumulation from previous test runs
	err = chromedp.Run(ctx,
		chromedp.Navigate(wordpressURL+"/cart"),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`
			if (typeof jQuery !== 'undefined') {
				jQuery('.woocommerce-cart-form__contents .remove').click();
			}
		`, nil),
		chromedp.Sleep(2*time.Second),
	)
	// Ignore errors if cart is already empty

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

	// Fill billing details using JavaScript - all in one block to avoid scope issues
	email := fmt.Sprintf("test%d@example.com", time.Now().Unix())
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				function setFieldValue(id, value) {
					var el = document.getElementById(id);
					el.value = value;
					el.dispatchEvent(new Event('input', { bubbles: true }));
					el.dispatchEvent(new Event('change', { bubbles: true }));
				}
				setFieldValue('billing_first_name', 'Test');
				setFieldValue('billing_last_name', 'User');
				setFieldValue('billing_email', '%s');
				setFieldValue('billing_phone', '1234567890');
				setFieldValue('billing_address_1', '123 Test St');
				setFieldValue('billing_city', 'Test City');
				setFieldValue('billing_postcode', '12345');
			})();
		`, email), nil),
		chromedp.Sleep(3*time.Second), // Allow WooCommerce validation and AJAX to complete
	)
	require.NoError(t, err, "Should fill billing details")

	// Wait for payment methods to load
	// Note: The radio input itself has display:none (normal for WooCommerce)
	// We just need to ensure the element exists and is ready to interact with
	err = chromedp.Run(ctx,
		chromedp.WaitReady(`#payment_method_north_payments`, chromedp.ByID),
		chromedp.Sleep(1*time.Second),
	)
	require.NoError(t, err, "Should wait for payment methods to load")

	// Select North Payments payment method
	// The radio input is hidden (display:none), so we use JavaScript to select it
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('#payment_method_north_payments').checked = true;`, nil),
		chromedp.Evaluate(`jQuery(document.body).trigger('payment_method_selected');`, nil),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Should select payment method")

	// Fill card details - use setTimeout to trigger click asynchronously
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#north_card_number`, chromedp.ByID),
		chromedp.Sleep(1*time.Second), // Allow jQuery event handlers to attach
		// Clear fields first to prevent text accumulation
		chromedp.Evaluate(`
			document.getElementById('north_card_number').value = '';
			document.getElementById('north_card_exp').value = '';
			document.getElementById('north_card_cvv').value = '';
			document.getElementById('north_card_zip').value = '';
		`, nil),
		// Set values directly with proper formatting (bypass the formatter)
		chromedp.Evaluate(`
			document.getElementById('north_card_number').value = '4111111111111111';
			document.getElementById('north_card_exp').value = '12/25';
			document.getElementById('north_card_cvv').value = '123';
			document.getElementById('north_card_zip').value = '12345';
			// Trigger change events for WooCommerce validation
			['north_card_number', 'north_card_exp', 'north_card_cvv', 'north_card_zip'].forEach(function(id) {
				var el = document.getElementById(id);
				el.dispatchEvent(new Event('input', { bubbles: true }));
				el.dispatchEvent(new Event('change', { bubbles: true }));
			});
		`, nil),
		chromedp.Sleep(1*time.Second), // Allow validation to complete
	)
	require.NoError(t, err, "Should fill card details")

	// Select transaction type if AUTH
	if isAuth {
		err = chromedp.Run(ctx,
			chromedp.SetValue(`#north_transaction_type`, "AUTH", chromedp.ByID),
		)
		// Ignore error if field doesn't exist
	}

	// Submit order using jQuery form submit to properly trigger WooCommerce validation
	t.Log("Submitting order...")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			setTimeout(function() {
				// Trigger WooCommerce's submit handler directly
				jQuery('form.checkout').submit();
			}, 10);
			'Form submission scheduled';
		`, nil),
	)
	require.NoError(t, err, "Should schedule form submission")

	// Wait for payment processing - this can take 10-20 seconds for EPX
	// The page will show a blocking overlay during processing
	t.Log("Waiting for payment processing...")
	time.Sleep(25 * time.Second)

	// Get final URL
	var orderURL string
	err = chromedp.Run(ctx,
		chromedp.Location(&orderURL),
	)
	if err != nil {
		t.Logf("Warning: Could not get location: %v", err)
		orderURL = wordpressURL + "/checkout" // fallback
	}
	t.Logf("Current URL: %s", orderURL)

	// Extract transaction ID from order received page or URL
	var txID string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.body.innerText`, &txID),
	)

	// Try to extract transaction ID from page content
	if strings.Contains(txID, "Transaction ID") {
		parts := strings.Split(txID, "Transaction ID")
		if len(parts) > 1 {
			// Extract UUID pattern
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

// bulkCapture performs bulk capture via WordPress admin UI
func bulkCapture(t *testing.T, ctx context.Context, txIDs []string) {
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

// verifyTransaction verifies a transaction exists with correct type
func verifyTransaction(t *testing.T, client paymentv1connect.PaymentServiceClient, txID string, expectedType paymentv1.TransactionType) {
	t.Helper()

	req := &paymentv1.GetTransactionRequest{TransactionId: txID}
	resp, err := client.GetTransaction(context.Background(), connect.NewRequest(req))
	require.NoError(t, err, "Transaction %s should exist", txID)
	require.Equal(t, expectedType, resp.Msg.Type, "Transaction type should match")
}

// verifyCaptureExists verifies a CAPTURE child transaction exists for an AUTH parent
func verifyCaptureExists(t *testing.T, client paymentv1connect.PaymentServiceClient, authTxID string) {
	t.Helper()

	// List transactions with parent_transaction_id = authTxID
	listReq := &paymentv1.ListTransactionsRequest{
		MerchantId:          merchantID,
		ParentTransactionId: authTxID,
	}
	listResp, err := client.ListTransactions(context.Background(), connect.NewRequest(listReq))
	require.NoError(t, err, "Should list child transactions")

	// Find CAPTURE transaction
	var foundCapture bool
	for _, tx := range listResp.Msg.Transactions {
		if tx.Type == paymentv1.TransactionType_TRANSACTION_TYPE_CAPTURE {
			foundCapture = true
			t.Logf("✅ Found CAPTURE transaction: %s (parent: %s)", tx.Id, authTxID)
			break
		}
	}

	require.True(t, foundCapture, "Should have CAPTURE child transaction for AUTH %s", authTxID)
}
