//go:build integration
// +build integration

package wordpress

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// WordPressConfig contains configuration for WordPress testing
type WordPressConfig struct {
	BaseURL      string // WordPress base URL (e.g., http://localhost:8082)
	AdminUser    string // WordPress admin username
	AdminPass    string // WordPress admin password
	DatabaseURL  string // PostgreSQL connection string for payment service
	CacheDisable bool   // Disable browser cache
}

// DefaultWordPressConfig returns default configuration for WordPress testing
func DefaultWordPressConfig() *WordPressConfig {
	return &WordPressConfig{
		BaseURL:      "http://localhost:8082",
		AdminUser:    "admin",
		AdminPass:    "admin",
		DatabaseURL:  "postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable",
		CacheDisable: true,
	}
}

// TestWordPressCheckoutFlow tests the complete WordPress WooCommerce checkout with North Payments
func TestWordPressCheckoutFlow(t *testing.T) {
	cfg := DefaultWordPressConfig()

	// Create headless Chrome context with cache disabled
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-cache", cfg.CacheDisable),
		chromedp.Flag("disable-application-cache", cfg.CacheDisable),
		chromedp.Flag("disable-offline-load-stale-cache", cfg.CacheDisable),
		chromedp.Flag("disk-cache-size", "0"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	t.Log("🔍 Starting WordPress Checkout Test")
	t.Log("═══════════════════════════════════════════════════════════════")

	// Step 1: Login to WordPress
	t.Log("📍 Logging in to WordPress...")
	err := chromedp.Run(ctx,
		chromedp.Navigate(cfg.BaseURL+"/wp-login.php"),
		chromedp.WaitVisible(`#user_login`, chromedp.ByID),
		chromedp.SendKeys(`#user_login`, cfg.AdminUser, chromedp.ByID),
		chromedp.SendKeys(`#user_pass`, cfg.AdminPass, chromedp.ByID),
		chromedp.Click(`#wp-submit`, chromedp.ByID),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Should login to WordPress successfully")
	t.Log("✅ Logged in to WordPress")

	// Step 2: Navigate to shop and add product to cart
	t.Log("🛍️  Adding product to cart...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(cfg.BaseURL+"/shop"),
		chromedp.WaitVisible(`.products`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		// Click first "Add to cart" button
		chromedp.Click(`.add_to_cart_button`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Should add product to cart")
	t.Log("✅ Product added to cart")

	// Step 3: Navigate to checkout
	t.Log("🛒 Going to checkout...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(cfg.BaseURL+"/checkout"),
		chromedp.WaitVisible(`#billing_first_name`, chromedp.ByID),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Should navigate to checkout page")
	t.Log("✅ At checkout page")

	// Step 4: Fill billing details
	t.Log("📝 Filling billing details...")
	err = chromedp.Run(ctx,
		chromedp.SendKeys(`#billing_first_name`, "Test", chromedp.ByID),
		chromedp.SendKeys(`#billing_last_name`, "User", chromedp.ByID),
		chromedp.SendKeys(`#billing_email`, "test@example.com", chromedp.ByID),
		chromedp.SendKeys(`#billing_phone`, "1234567890", chromedp.ByID),
		chromedp.SendKeys(`#billing_address_1`, "123 Test St", chromedp.ByID),
		chromedp.SendKeys(`#billing_city`, "Test City", chromedp.ByID),
		chromedp.SendKeys(`#billing_postcode`, "12345", chromedp.ByID),
		chromedp.Sleep(1*time.Second),
	)
	require.NoError(t, err, "Should fill billing details")
	t.Log("✅ Billing details filled")

	// Step 5: Select North Payments gateway
	t.Log("💳 Selecting North Payments gateway...")
	err = chromedp.Run(ctx,
		chromedp.Click(`#payment_method_north_payments`, chromedp.ByID),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Should select North Payments")
	t.Log("✅ North Payments selected")

	// Step 6: Fill card details
	t.Log("🔢 Filling card details...")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#north_card_number`, chromedp.ByID),
		chromedp.SendKeys(`#north_card_number`, "4111111111111111", chromedp.ByID),
		chromedp.SendKeys(`#north_card_exp`, "12/25", chromedp.ByID),
		chromedp.SendKeys(`#north_card_cvv`, "123", chromedp.ByID),
		chromedp.SendKeys(`#north_card_zip`, "12345", chromedp.ByID),
		chromedp.Sleep(1*time.Second),
	)
	require.NoError(t, err, "Should fill card details")
	t.Log("✅ Card details filled")

	// Step 7: Submit order
	t.Log("🚀 Submitting order...")
	var finalURL string
	err = chromedp.Run(ctx,
		chromedp.Click(`#place_order`, chromedp.ByID),
		chromedp.Sleep(15*time.Second), // Wait for EPX redirect and callback
		chromedp.Location(&finalURL),
	)

	if err != nil {
		t.Logf("⚠️  Order submission encountered an issue: %v", err)
	}

	t.Logf("📍 Final URL: %s", finalURL)

	// Step 8: Verify transaction in database
	t.Log("🔍 Verifying transaction in database...")
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	require.NoError(t, err, "Should connect to database")
	defer db.Close()

	// Query for most recent transaction (should be from this test)
	var txID, txType, status, amount, authResp, authCode, authGUID string
	var createdAt time.Time
	err = db.QueryRow(`
		SELECT id, type, status, amount, auth_resp, auth_code, auth_guid, created_at
		FROM transactions
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&txID, &txType, &status, &amount, &authResp, &authCode, &authGUID, &createdAt)

	require.NoError(t, err, "Should find transaction in database")

	t.Log("✅ Transaction Found!")
	t.Logf("   ID:              %s", txID)
	t.Logf("   Type:            %s", txType)
	t.Logf("   Status:          %s", status)
	t.Logf("   Amount:          $%s", amount)
	t.Logf("   Auth Response:   %s", authResp)
	t.Logf("   Auth Code:       %s", authCode)
	t.Logf("   Auth GUID:       %s", authGUID)
	t.Logf("   Created:         %s", createdAt.Format(time.RFC3339))

	// Verify transaction was approved
	require.Equal(t, "approved", status, "Transaction should be approved")
	require.Equal(t, "SALE", txType, "Transaction should be SALE type")
	require.Equal(t, "00", authResp, "Auth response should be 00 (APPROVED)")

	t.Log("═══════════════════════════════════════════════════════════════")
	t.Log("🎉 WordPress checkout test PASSED!")
}

// TestWordPressCheckoutWithConsoleLogging tests checkout with browser console logging
func TestWordPressCheckoutWithConsoleLogging(t *testing.T) {
	cfg := DefaultWordPressConfig()

	// Create Chrome context with console logging enabled
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false), // Run with UI for debugging
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-cache", cfg.CacheDisable),
		chromedp.Flag("disable-application-cache", cfg.CacheDisable),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(t.Logf),
	)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	t.Log("🔍 WordPress Checkout Test with Console Logging")
	t.Log("═══════════════════════════════════════════════════════════════")

	// Enable console logging
	consoleLogs := make([]string, 0)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		// Capture console logs here if needed
		// This is a placeholder for console event handling
	})

	// Follow same steps as TestWordPressCheckoutFlow
	// (Login, add to cart, checkout, fill details, submit)

	t.Log("📋 Instructions:")
	t.Log("   1. Login to WordPress")
	t.Log("   2. Add a product to cart")
	t.Log("   3. Go to checkout")
	t.Log("   4. Fill billing details")
	t.Log("   5. Select North Payments")
	t.Log("   6. Fill card details (4111111111111111, 12/25, 123, 12345)")
	t.Log("   7. Click 'Place Order'")
	t.Log("")
	t.Log("Browser will open for manual testing...")
	t.Log("Console logs will be displayed here.")

	// Open WordPress login page
	err := chromedp.Run(ctx,
		chromedp.Navigate(cfg.BaseURL+"/wp-login.php"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	require.NoError(t, err)

	// Keep browser open for manual testing
	time.Sleep(3 * time.Minute)

	if len(consoleLogs) > 0 {
		t.Log("📜 Browser Console Logs:")
		t.Log("═══════════════════════════════════════════════════════════════")
		for i, log := range consoleLogs {
			t.Logf("[%d] %s", i+1, log)
		}
	}
}
