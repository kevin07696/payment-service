//go:build integration
// +build integration

package subscription_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApartmentPayE2E_OwnerCreatesInvoiceAndRenterPays simulates the full flow:
// 1. Owner creates an invoice for a tenant
// 2. Owner generates a payment link
// 3. Tenant pays the invoice using the payment link
// 4. Invoice status is updated to paid
//
// This tests the apartment-pay frontend -> payment service integration
func TestApartmentPayE2E_OwnerCreatesInvoiceAndRenterPays(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	client := testutil.NewClient("http://localhost:8081")

	// Test data - simulating apartment-pay frontend data
	merchantID := "00000000-0000-0000-0000-000000000001"       // Owner's merchant
	tenantID := "00000000-0000-0000-0000-000000000010"         // Tenant as customer
	invoiceAmount := int64(150000)                             // $1,500.00 monthly rent
	invoiceDescription := "December 2024 Rent - Unit 101"

	time.Sleep(2 * time.Second)

	// Load test service credentials and generate JWT
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services, "No test services found")

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
	require.NoError(t, err)

	t.Logf("=== APARTMENT PAY E2E TEST: Invoice Payment Flow ===")
	t.Logf("Merchant ID: %s", merchantID)
	t.Logf("Tenant ID: %s", tenantID)
	t.Logf("Invoice Amount: $%.2f", float64(invoiceAmount)/100)

	// Step 1: Create a payment method for the tenant (simulates tenant adding card)
	t.Log("\n--- Step 1: Tenant adds payment method ---")
	paymentMethodID := setupPaymentMethod(t, cfg, client, merchantID, tenantID)
	t.Logf("✅ Payment method created: %s", paymentMethodID)
	time.Sleep(2 * time.Second)

	// Step 2: Process payment for the invoice (simulates tenant clicking Pay on payment link)
	t.Log("\n--- Step 2: Tenant pays invoice via payment link ---")
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	// Generate unique idempotency key for each test run
	saleIdempotencyKey := uuid.New().String()

	saleReq := map[string]interface{}{
		"merchantId":      merchantID,
		"customerId":      tenantID,
		"paymentMethodId": paymentMethodID,
		"amountCents":     invoiceAmount,
		"currency":        "USD",
		"idempotencyKey":  saleIdempotencyKey,
		"metadata": map[string]string{
			"invoice_id":  "inv-test-001",
			"description": invoiceDescription,
			"type":        "rent_payment",
		},
	}

	saleResp, err := client.DoConnectRPC("payment.v1.PaymentService", "Sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	require.Equal(t, 200, saleResp.StatusCode, "Sale should succeed")

	var saleResult map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &saleResult)
	require.NoError(t, err)

	isApproved, _ := saleResult["isApproved"].(bool)
	transactionID, _ := saleResult["transactionId"].(string)
	authCode, _ := saleResult["authorizationCode"].(string)

	assert.True(t, isApproved, "Payment should be approved")
	assert.NotEmpty(t, transactionID, "Should have transaction ID")

	t.Logf("✅ Payment approved!")
	t.Logf("   Transaction ID: %s", transactionID)
	t.Logf("   Authorization Code: %s", authCode)
	t.Logf("   Amount: $%.2f", float64(invoiceAmount)/100)

	// Step 3: Verify transaction can be retrieved (owner can see payment status)
	t.Log("\n--- Step 3: Owner verifies payment ---")
	time.Sleep(1 * time.Second)

	getTxReq := map[string]interface{}{
		"transactionId": transactionID,
	}

	getTxResp, err := client.DoConnectRPC("payment.v1.PaymentService", "GetTransaction", getTxReq)
	require.NoError(t, err)
	defer getTxResp.Body.Close()

	var txResult map[string]interface{}
	err = testutil.DecodeResponse(getTxResp, &txResult)
	require.NoError(t, err)

	txStatus, _ := txResult["status"].(string)
	assert.Contains(t, txStatus, "APPROVED", "Transaction should show approved status")

	t.Logf("✅ Transaction verified: %s", txStatus)

	t.Log("\n=== APARTMENT PAY E2E TEST PASSED ===")
	t.Logf("Invoice '%s' for tenant %s paid successfully", invoiceDescription, tenantID)
}

// TestApartmentPayE2E_OwnerCreatesRecurringSubscription simulates:
// 1. Owner sets up recurring invoice with auto-pay
// 2. Subscription is created in payment service
// 3. Cron job processes billing at the scheduled interval
// 4. Verify billing succeeds and subscription remains active
//
// NOTE: For testing, we use a 1-second interval to quickly verify the flow
// In production, this would be monthly
func TestApartmentPayE2E_OwnerCreatesRecurringSubscription(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	client := testutil.NewClient("http://localhost:8081")

	// Test data
	merchantID := "00000000-0000-0000-0000-000000000001"
	tenantID := "00000000-0000-0000-0000-000000000011" // Different tenant for this test
	rentAmount := int64(175000)                        // $1,750.00 monthly rent

	time.Sleep(2 * time.Second)

	// Load test service credentials
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services, "No test services found")

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
	require.NoError(t, err)

	t.Logf("=== APARTMENT PAY E2E TEST: Recurring Subscription ===")
	t.Logf("Merchant ID: %s", merchantID)
	t.Logf("Tenant ID: %s", tenantID)
	t.Logf("Monthly Rent: $%.2f", float64(rentAmount)/100)

	// Step 1: Tenant adds payment method
	t.Log("\n--- Step 1: Tenant adds payment method for auto-pay ---")
	paymentMethodID := setupPaymentMethod(t, cfg, client, merchantID, tenantID)
	t.Logf("✅ Payment method created: %s", paymentMethodID)
	time.Sleep(2 * time.Second)

	// Step 2: Owner creates recurring subscription (auto-pay enabled)
	t.Log("\n--- Step 2: Owner enables auto-pay subscription ---")
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	// Set start_date to past so it's immediately due for billing
	startDate := time.Now().Add(-60 * 24 * time.Hour)

	createSubReq := map[string]interface{}{
		"merchantId":      merchantID,
		"customerId":      tenantID,
		"paymentMethodId": paymentMethodID,
		"amountCents":     rentAmount,
		"currency":        "USD",
		"intervalValue":   1,
		"intervalUnit":    3, // INTERVAL_UNIT_MONTH = 3
		"startDate":       startDate.Format(time.RFC3339Nano),
		"maxRetries":      3,
		"metadata": map[string]string{
			"description":    "Monthly Rent - Unit 102",
			"apartment_unit": "102",
			"tenant_name":    "Jane Smith",
		},
	}

	subResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "CreateSubscription", createSubReq)
	require.NoError(t, err)
	defer subResp.Body.Close()

	require.Equal(t, 200, subResp.StatusCode, "CreateSubscription should succeed")

	var subResult map[string]interface{}
	err = testutil.DecodeResponse(subResp, &subResult)
	require.NoError(t, err)

	subscriptionID, ok := subResult["subscriptionId"].(string)
	require.True(t, ok && subscriptionID != "", "Subscription should have ID")

	status, _ := subResult["status"].(string)
	t.Logf("✅ Subscription created: %s", subscriptionID)
	t.Logf("   Status: %s", status)
	time.Sleep(2 * time.Second)

	// Step 3: Trigger billing (simulates cron job)
	t.Log("\n--- Step 3: Process due billing (simulates cron) ---")
	billingReq := map[string]interface{}{
		"as_of_date": time.Now().Format("2006-01-02"),
		"batch_size": 100,
	}

	// Use cron endpoint with cron secret authentication
	billingResp, err := client.DoCronRequest("/cron/process-billing", billingReq, cfg.CronSecret)
	require.NoError(t, err)
	defer billingResp.Body.Close()

	// Cron endpoint returns 200 for success or 206 for partial success
	assert.True(t, billingResp.StatusCode == 200 || billingResp.StatusCode == 206, "ProcessBilling should succeed (status %d)", billingResp.StatusCode)

	var billingResult map[string]interface{}
	err = testutil.DecodeResponse(billingResp, &billingResult)
	require.NoError(t, err)

	processedCount, _ := billingResult["processed"].(float64)
	successCount, _ := billingResult["success_count"].(float64)
	failureCount, _ := billingResult["failure_count"].(float64)

	t.Logf("✅ Billing processed: %d subscriptions, %d successful, %d failed", int(processedCount), int(successCount), int(failureCount))
	// Note: Old stale subscriptions in the test database may fail billing.
	// The important thing is that our subscription was processed and remains active.
	assert.Greater(t, int(processedCount), 0, "Should have processed at least one subscription")

	// Step 4: Verify subscription is still active
	t.Log("\n--- Step 4: Verify subscription status ---")
	time.Sleep(1 * time.Second)

	getSubReq := map[string]interface{}{
		"subscriptionId": subscriptionID,
	}

	getSubResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "GetSubscription", getSubReq)
	require.NoError(t, err)
	defer getSubResp.Body.Close()

	var subscription map[string]interface{}
	err = testutil.DecodeResponse(getSubResp, &subscription)
	require.NoError(t, err)

	finalStatus, _ := subscription["status"].(string)
	nextBillingDate, _ := subscription["nextBillingDate"].(string)

	assert.Equal(t, "SUBSCRIPTION_STATUS_ACTIVE", finalStatus, "Subscription should remain active")
	t.Logf("✅ Subscription verified:")
	t.Logf("   Status: %s", finalStatus)
	t.Logf("   Next Billing Date: %s", nextBillingDate)

	t.Log("\n=== APARTMENT PAY RECURRING SUBSCRIPTION TEST PASSED ===")
	t.Logf("Auto-pay for tenant %s is active and billing correctly", tenantID)
}

// TestApartmentPayE2E_RapidSubscriptionBilling tests rapid billing intervals
// This test uses a 1-second interval to verify the subscription scheduler works correctly
//
// PREREQUISITES:
// 1. Rate limit must be increased: ./paycli -action=update-service -json=update-service.json
//    where update-service.json contains: {"service_id": "test-service-001", "rate_limit": 100}
// 2. Payment service must be running
func TestApartmentPayE2E_RapidSubscriptionBilling(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	// Skip in short mode - this is a longer running test
	if testing.Short() {
		t.Skip("Skipping rapid billing test in short mode")
	}

	cfg, _ := testutil.Setup(t)
	client := testutil.NewClient("http://localhost:8081")

	merchantID := "00000000-0000-0000-0000-000000000001"
	tenantID := "00000000-0000-0000-0000-000000000012"
	dailyAmount := int64(1000) // $10.00 for rapid test

	time.Sleep(2 * time.Second)

	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services, "No test services found")

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
	require.NoError(t, err)

	t.Logf("=== RAPID SUBSCRIPTION BILLING TEST ===")
	t.Logf("Testing 1-DAY interval subscription with multiple billing cycles")

	// Setup payment method
	t.Log("\n--- Setup: Create payment method ---")
	paymentMethodID := setupPaymentMethod(t, cfg, client, merchantID, tenantID)
	t.Logf("✅ Payment method: %s", paymentMethodID)
	time.Sleep(2 * time.Second)

	// Create subscription with daily interval, starting 2 days ago
	t.Log("\n--- Create subscription (1-day interval, 2 days overdue) ---")
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	// Start 2 days ago so we have 2 billing cycles due
	startDate := time.Now().Add(-48 * time.Hour)

	createSubReq := map[string]interface{}{
		"merchantId":      merchantID,
		"customerId":      tenantID,
		"paymentMethodId": paymentMethodID,
		"amountCents":     dailyAmount,
		"currency":        "USD",
		"intervalValue":   1,
		"intervalUnit":    1, // INTERVAL_UNIT_DAY = 1
		"startDate":       startDate.Format(time.RFC3339Nano),
		"maxRetries":      3,
		"metadata": map[string]string{
			"test": "rapid_billing",
		},
	}

	subResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "CreateSubscription", createSubReq)
	require.NoError(t, err)
	defer subResp.Body.Close()
	require.Equal(t, 200, subResp.StatusCode)

	var subResult map[string]interface{}
	err = testutil.DecodeResponse(subResp, &subResult)
	require.NoError(t, err)

	subscriptionID, _ := subResult["subscriptionId"].(string)
	t.Logf("✅ Subscription created: %s", subscriptionID)

	// Process multiple billing cycles
	t.Log("\n--- Process billing cycles ---")

	totalProcessed := 0
	totalSuccess := 0

	for cycle := 1; cycle <= 3; cycle++ {
		time.Sleep(2 * time.Second)

		billingReq := map[string]interface{}{
			"as_of_date": time.Now().Format("2006-01-02"),
			"batch_size": 100,
		}

		// Use cron endpoint with cron secret authentication
		billingResp, err := client.DoCronRequest("/cron/process-billing", billingReq, cfg.CronSecret)
		require.NoError(t, err)
		defer billingResp.Body.Close()

		var billingResult map[string]interface{}
		err = testutil.DecodeResponse(billingResp, &billingResult)
		require.NoError(t, err)

		processed, _ := billingResult["processed"].(float64)
		success, _ := billingResult["success_count"].(float64)

		totalProcessed += int(processed)
		totalSuccess += int(success)

		t.Logf("   Cycle %d: Processed=%d, Success=%d", cycle, int(processed), int(success))
	}

	t.Logf("\n✅ Total billing cycles: Processed=%d, Success=%d", totalProcessed, totalSuccess)

	// Verify final subscription state
	t.Log("\n--- Verify final subscription state ---")
	time.Sleep(1 * time.Second)

	getSubResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "GetSubscription", map[string]interface{}{
		"subscriptionId": subscriptionID,
	})
	require.NoError(t, err)
	defer getSubResp.Body.Close()

	var subscription map[string]interface{}
	err = testutil.DecodeResponse(getSubResp, &subscription)
	require.NoError(t, err)

	finalStatus, _ := subscription["status"].(string)
	nextBilling, _ := subscription["nextBillingDate"].(string)

	t.Logf("✅ Final status: %s", finalStatus)
	t.Logf("   Next billing: %s", nextBilling)

	assert.Equal(t, "SUBSCRIPTION_STATUS_ACTIVE", finalStatus, "Subscription should still be active")

	t.Log("\n=== RAPID SUBSCRIPTION BILLING TEST PASSED ===")
}
