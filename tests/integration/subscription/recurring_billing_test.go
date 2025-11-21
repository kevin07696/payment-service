//go:build integration
// +build integration

package subscription_test

import (
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPaymentMethod is a helper to create a payment method via Browser Post STORAGE for recurring billing tests
func setupPaymentMethod(t *testing.T, cfg *testutil.Config, client *testutil.Client, merchantID, customerID string) string {
	t.Helper()
	callbackBaseURL := "http://localhost:8081"

	// Load test service credentials and generate JWT
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services, "No test services found")

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
	require.NoError(t, err)

	// Tokenize and save payment method via Browser Post STORAGE flow
	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t, cfg, client, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL,
	)
	require.NoError(t, err)
	return paymentMethodID
}

// TestRecurringBilling tests the subscription payment cycle integration
// This is a true integration test - tests cron trigger + EPX payment + database state
func TestRecurringBilling(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	// ConnectRPC client on port 8080 for subscription RPCs
	client := testutil.NewClient("http://localhost:8080")
	customerID := "00000000-0000-0000-0000-000000000004" // Valid UUID for test customer
	merchantID := "00000000-0000-0000-0000-000000000001"
	time.Sleep(2 * time.Second)

	// Load test service credentials and generate JWT for subscription calls
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services, "No test services found")

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
	require.NoError(t, err)

	// Setup payment method via Browser Post STORAGE
	paymentMethodID := setupPaymentMethod(t, cfg, client, merchantID, customerID)
	time.Sleep(2 * time.Second)

	t.Logf("✅ Payment method created: %s", paymentMethodID)

	// Create subscription using ConnectRPC
	// Set start_date to 2 months ago so next_billing_date will be in the past (due for billing)
	startDate := time.Now().Add(-60 * 24 * time.Hour)

	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	createSubReq := map[string]interface{}{
		"merchantId":      merchantID,
		"customerId":      customerID,
		"paymentMethodId": paymentMethodID,
		"amountCents":     1999, // $19.99
		"currency":        "USD",
		"intervalValue":   1,
		"intervalUnit":    3, // INTERVAL_UNIT_MONTH = 3
		"startDate":       startDate.Format(time.RFC3339Nano),
		"maxRetries":      3,
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

	t.Logf("✅ Subscription created: %s", subscriptionID)
	time.Sleep(2 * time.Second)

	// Process recurring billing using ProcessDueBilling (simulates cron job trigger)
	// This processes all subscriptions due as of now
	billingReq := map[string]interface{}{
		"asOfDate":  time.Now().Format(time.RFC3339Nano),
		"batchSize": 100,
	}

	billingResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "ProcessDueBilling", billingReq)
	require.NoError(t, err)
	defer billingResp.Body.Close()

	assert.Equal(t, 200, billingResp.StatusCode, "ProcessDueBilling should succeed")

	var billingResult map[string]interface{}
	err = testutil.DecodeResponse(billingResp, &billingResult)
	require.NoError(t, err)

	// Verify billing was processed
	processedCount, _ := billingResult["processedCount"].(float64)
	successCount, _ := billingResult["successCount"].(float64)

	assert.Greater(t, int(processedCount), 0, "Should have processed at least one subscription")
	assert.Greater(t, int(successCount), 0, "Should have successfully billed at least one subscription")

	t.Logf("✅ Recurring billing integration test passed - Processed: %d, Success: %d",
		int(processedCount), int(successCount))

	// Verify subscription status
	time.Sleep(1 * time.Second)
	getResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "GetSubscription", map[string]interface{}{
		"subscriptionId": subscriptionID,
	})
	require.NoError(t, err)
	defer getResp.Body.Close()

	var subscription map[string]interface{}
	err = testutil.DecodeResponse(getResp, &subscription)
	require.NoError(t, err)

	// ConnectRPC serializes proto enums as strings (e.g., "SUBSCRIPTION_STATUS_ACTIVE"), not numbers
	status, ok := subscription["status"].(string)
	require.True(t, ok, "status should be a string")
	assert.Equal(t, "SUBSCRIPTION_STATUS_ACTIVE", status, "Subscription should remain active after successful billing")

	t.Logf("✅ Subscription status after billing: %s", status)
}

// TestSubscription_FailedRecurringBilling tests handling of failed recurring billing
// Integration test: cron + EPX decline + database state handling
func TestSubscription_FailedRecurringBilling(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, _ := testutil.Setup(t)
	// ConnectRPC client on port 8080 for subscription RPCs
	client := testutil.NewClient("http://localhost:8080")
	customerID := "00000000-0000-0000-0000-000000000008" // Valid UUID for test customer
	merchantID := "00000000-0000-0000-0000-000000000001"
	time.Sleep(2 * time.Second)

	// Load test service credentials and generate JWT
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, services, "No test services found")

	jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
	require.NoError(t, err)

	// Setup payment method with standard test card via Browser Post STORAGE
	// EPX triggers declines based on amount, not card number
	// We'll use a standard test card and trigger decline with a specific amount in recurring billing
	declineCard := testutil.TestVisaCard
	callbackBaseURL := "http://localhost:8081"
	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t, cfg, client, jwtToken, merchantID, customerID, declineCard, callbackBaseURL,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	t.Logf("✅ Payment method created: %s", paymentMethodID)

	// Create subscription using ConnectRPC
	// Set start_date to 2 months ago so next_billing_date will be in the past (due for billing)
	startDate := time.Now().Add(-60 * 24 * time.Hour)

	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	createSubReq := map[string]interface{}{
		"merchantId":      merchantID,
		"customerId":      customerID,
		"paymentMethodId": paymentMethodID,
		"amountCents":     105, // $1.05 - EPX test amount that triggers decline (response code 05)
		"currency":        "USD",
		"intervalValue":   1,
		"intervalUnit":    3, // INTERVAL_UNIT_MONTH = 3
		"startDate":       startDate.Format(time.RFC3339Nano),
		"maxRetries":      3,
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

	t.Logf("✅ Subscription created: %s", subscriptionID)
	time.Sleep(2 * time.Second)

	// Attempt recurring billing (should fail due to decline card)
	billingReq := map[string]interface{}{
		"asOfDate":  time.Now().Format(time.RFC3339Nano),
		"batchSize": 100,
	}

	billingResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "ProcessDueBilling", billingReq)
	require.NoError(t, err)
	defer billingResp.Body.Close()

	// ProcessDueBilling should succeed even if individual billings fail
	assert.Equal(t, 200, billingResp.StatusCode, "ProcessDueBilling should succeed")

	var billingResult map[string]interface{}
	err = testutil.DecodeResponse(billingResp, &billingResult)
	require.NoError(t, err)

	// Verify that billing was attempted but failed
	processedCount, _ := billingResult["processedCount"].(float64)
	failedCount, _ := billingResult["failedCount"].(float64)

	assert.Greater(t, int(processedCount), 0, "Should have processed at least one subscription")
	assert.Greater(t, int(failedCount), 0, "Should have failed at least one billing")

	t.Logf("✅ Failed recurring billing handled correctly - Processed: %d, Failed: %d",
		int(processedCount), int(failedCount))

	time.Sleep(2 * time.Second)

	// Verify subscription status updated correctly (should be marked as past_due)
	getResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "GetSubscription", map[string]interface{}{
		"subscriptionId": subscriptionID,
	})
	require.NoError(t, err)
	defer getResp.Body.Close()

	var subscription map[string]interface{}
	err = testutil.DecodeResponse(getResp, &subscription)
	require.NoError(t, err)

	// ConnectRPC serializes proto enums as strings (e.g., "SUBSCRIPTION_STATUS_ACTIVE"), not numbers
	status, ok := subscription["status"].(string)
	require.True(t, ok, "status should be a string")
	assert.Contains(t, []string{"SUBSCRIPTION_STATUS_ACTIVE", "SUBSCRIPTION_STATUS_PAST_DUE"}, status,
		"Subscription should be active or past_due after failed billing")

	t.Logf("✅ Subscription status after failed billing: %s", status)
}
