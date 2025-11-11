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

// TestCreateSubscription_WithStoredCard tests creating a subscription with stored payment method
func TestCreateSubscription_WithStoredCard(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-sub-001"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Create subscription
	subReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"plan_id":           "monthly-premium",
		"amount":            "29.99",
		"currency":          "USD",
		"billing_cycle":     "monthly",
		"start_date":        time.Now().Format(time.RFC3339),
		"metadata": map[string]string{
			"plan_name": "Premium Monthly",
		},
	}

	subResp, err := client.Do("POST", "/api/v1/subscriptions", subReq)
	require.NoError(t, err)
	defer subResp.Body.Close()

	assert.Equal(t, 201, subResp.StatusCode, "Should create subscription")

	var subResult map[string]interface{}
	err = testutil.DecodeResponse(subResp, &subResult)
	require.NoError(t, err)

	// Verify subscription response
	assert.NotEmpty(t, subResult["subscription_id"])
	assert.Equal(t, customerID, subResult["customer_id"])
	assert.Equal(t, paymentMethodID, subResult["payment_method_id"])
	assert.Equal(t, "active", subResult["status"])
	assert.Equal(t, "29.99", subResult["amount"])
	assert.Equal(t, "monthly", subResult["billing_cycle"])

	t.Logf("Created subscription: %s", subResult["subscription_id"])
}

// TestGetSubscription retrieves subscription details
func TestGetSubscription(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-sub-002"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestMastercardCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Create subscription
	subReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"plan_id":           "yearly-premium",
		"amount":            "299.99",
		"currency":          "USD",
		"billing_cycle":     "yearly",
		"start_date":        time.Now().Format(time.RFC3339),
	}

	subResp, err := client.Do("POST", "/api/v1/subscriptions", subReq)
	require.NoError(t, err)
	defer subResp.Body.Close()

	var subResult map[string]interface{}
	err = testutil.DecodeResponse(subResp, &subResult)
	require.NoError(t, err)

	subscriptionID := subResult["subscription_id"].(string)
	time.Sleep(2 * time.Second)

	// Retrieve subscription
	getResp, err := client.Do("GET", "/api/v1/subscriptions/"+subscriptionID, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()

	assert.Equal(t, 200, getResp.StatusCode)

	var getResult map[string]interface{}
	err = testutil.DecodeResponse(getResp, &getResult)
	require.NoError(t, err)

	assert.Equal(t, subscriptionID, getResult["subscription_id"])
	assert.Equal(t, customerID, getResult["customer_id"])
	assert.Equal(t, "299.99", getResult["amount"])
	assert.Equal(t, "yearly", getResult["billing_cycle"])
	assert.Equal(t, "active", getResult["status"])

	t.Logf("Retrieved subscription: %s", subscriptionID)
}

// TestListSubscriptions_ByCustomer lists subscriptions for a customer
func TestListSubscriptions_ByCustomer(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-sub-003"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Create multiple subscriptions
	plans := []map[string]interface{}{
		{
			"plan_id":       "basic-monthly",
			"amount":        "9.99",
			"billing_cycle": "monthly",
		},
		{
			"plan_id":       "pro-monthly",
			"amount":        "49.99",
			"billing_cycle": "monthly",
		},
	}

	for _, plan := range plans {
		subReq := map[string]interface{}{
			"agent_id":          "test-merchant-staging",
			"customer_id":       customerID,
			"payment_method_id": paymentMethodID,
			"plan_id":           plan["plan_id"],
			"amount":            plan["amount"],
			"currency":          "USD",
			"billing_cycle":     plan["billing_cycle"],
			"start_date":        time.Now().Format(time.RFC3339),
		}

		resp, err := client.Do("POST", "/api/v1/subscriptions", subReq)
		require.NoError(t, err)
		resp.Body.Close()
		time.Sleep(2 * time.Second)
	}

	// List subscriptions
	listResp, err := client.Do("GET", "/api/v1/subscriptions?customer_id="+customerID, nil)
	require.NoError(t, err)
	defer listResp.Body.Close()

	assert.Equal(t, 200, listResp.StatusCode)

	var listResult map[string]interface{}
	err = testutil.DecodeResponse(listResp, &listResult)
	require.NoError(t, err)

	subscriptions := listResult["subscriptions"].([]interface{})
	assert.GreaterOrEqual(t, len(subscriptions), 2, "Should have at least 2 subscriptions")

	t.Logf("Found %d subscriptions for customer %s", len(subscriptions), customerID)
}

// TestRecurringBilling tests processing recurring billing for a subscription
func TestRecurringBilling(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-sub-004"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Create subscription
	subReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"plan_id":           "monthly-test",
		"amount":            "19.99",
		"currency":          "USD",
		"billing_cycle":     "monthly",
		"start_date":        time.Now().Format(time.RFC3339),
	}

	subResp, err := client.Do("POST", "/api/v1/subscriptions", subReq)
	require.NoError(t, err)
	defer subResp.Body.Close()

	var subResult map[string]interface{}
	err = testutil.DecodeResponse(subResp, &subResult)
	require.NoError(t, err)

	subscriptionID := subResult["subscription_id"].(string)
	time.Sleep(2 * time.Second)

	// Process recurring billing
	billingReq := map[string]interface{}{
		"subscription_id": subscriptionID,
	}

	billingResp, err := client.Do("POST", "/api/v1/subscriptions/"+subscriptionID+"/bill", billingReq)
	require.NoError(t, err)
	defer billingResp.Body.Close()

	assert.Equal(t, 200, billingResp.StatusCode, "Recurring billing should succeed")

	var billingResult map[string]interface{}
	err = testutil.DecodeResponse(billingResp, &billingResult)
	require.NoError(t, err)

	// Verify transaction was created
	assert.NotEmpty(t, billingResult["transaction_id"])
	assert.NotEmpty(t, billingResult["group_id"])
	assert.Equal(t, "19.99", billingResult["amount"])
	assert.Equal(t, "completed", billingResult["status"])
	assert.True(t, billingResult["is_approved"].(bool))

	t.Logf("Recurring billing processed - Transaction ID: %s", billingResult["transaction_id"])
}

// TestCancelSubscription tests canceling a subscription
func TestCancelSubscription(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-sub-005"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Create subscription
	subReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"plan_id":           "monthly-cancel-test",
		"amount":            "39.99",
		"currency":          "USD",
		"billing_cycle":     "monthly",
		"start_date":        time.Now().Format(time.RFC3339),
	}

	subResp, err := client.Do("POST", "/api/v1/subscriptions", subReq)
	require.NoError(t, err)
	defer subResp.Body.Close()

	var subResult map[string]interface{}
	err = testutil.DecodeResponse(subResp, &subResult)
	require.NoError(t, err)

	subscriptionID := subResult["subscription_id"].(string)
	time.Sleep(2 * time.Second)

	// Cancel subscription
	cancelReq := map[string]interface{}{
		"reason": "Customer requested cancellation",
	}

	cancelResp, err := client.Do("POST", "/api/v1/subscriptions/"+subscriptionID+"/cancel", cancelReq)
	require.NoError(t, err)
	defer cancelResp.Body.Close()

	assert.Equal(t, 200, cancelResp.StatusCode, "Cancel should succeed")

	var cancelResult map[string]interface{}
	err = testutil.DecodeResponse(cancelResp, &cancelResult)
	require.NoError(t, err)

	assert.Equal(t, "canceled", cancelResult["status"])
	assert.NotEmpty(t, cancelResult["canceled_at"])

	t.Logf("Subscription canceled: %s", subscriptionID)

	time.Sleep(2 * time.Second)

	// Verify subscription is canceled
	getResp, err := client.Do("GET", "/api/v1/subscriptions/"+subscriptionID, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()

	var getResult map[string]interface{}
	err = testutil.DecodeResponse(getResp, &getResult)
	require.NoError(t, err)

	assert.Equal(t, "canceled", getResult["status"])
}

// TestPauseAndResumeSubscription tests pausing and resuming a subscription
func TestPauseAndResumeSubscription(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-sub-006"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestMastercardCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Create subscription
	subReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"plan_id":           "monthly-pause-test",
		"amount":            "24.99",
		"currency":          "USD",
		"billing_cycle":     "monthly",
		"start_date":        time.Now().Format(time.RFC3339),
	}

	subResp, err := client.Do("POST", "/api/v1/subscriptions", subReq)
	require.NoError(t, err)
	defer subResp.Body.Close()

	var subResult map[string]interface{}
	err = testutil.DecodeResponse(subResp, &subResult)
	require.NoError(t, err)

	subscriptionID := subResult["subscription_id"].(string)
	time.Sleep(2 * time.Second)

	// Pause subscription
	pauseResp, err := client.Do("POST", "/api/v1/subscriptions/"+subscriptionID+"/pause", nil)
	require.NoError(t, err)
	defer pauseResp.Body.Close()

	assert.Equal(t, 200, pauseResp.StatusCode, "Pause should succeed")

	var pauseResult map[string]interface{}
	err = testutil.DecodeResponse(pauseResp, &pauseResult)
	require.NoError(t, err)

	assert.Equal(t, "paused", pauseResult["status"])
	t.Logf("Subscription paused: %s", subscriptionID)

	time.Sleep(2 * time.Second)

	// Resume subscription
	resumeResp, err := client.Do("POST", "/api/v1/subscriptions/"+subscriptionID+"/resume", nil)
	require.NoError(t, err)
	defer resumeResp.Body.Close()

	assert.Equal(t, 200, resumeResp.StatusCode, "Resume should succeed")

	var resumeResult map[string]interface{}
	err = testutil.DecodeResponse(resumeResp, &resumeResult)
	require.NoError(t, err)

	assert.Equal(t, "active", resumeResult["status"])
	t.Logf("Subscription resumed: %s", subscriptionID)
}

// TestUpdateSubscriptionPaymentMethod tests updating payment method on subscription
func TestUpdateSubscriptionPaymentMethod(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-sub-007"
	time.Sleep(2 * time.Second)

	// Tokenize and save first payment method
	paymentMethodID1, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Create subscription with first payment method
	subReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID1,
		"plan_id":           "monthly-update-test",
		"amount":            "34.99",
		"currency":          "USD",
		"billing_cycle":     "monthly",
		"start_date":        time.Now().Format(time.RFC3339),
	}

	subResp, err := client.Do("POST", "/api/v1/subscriptions", subReq)
	require.NoError(t, err)
	defer subResp.Body.Close()

	var subResult map[string]interface{}
	err = testutil.DecodeResponse(subResp, &subResult)
	require.NoError(t, err)

	subscriptionID := subResult["subscription_id"].(string)
	time.Sleep(2 * time.Second)

	// Tokenize and save second payment method
	paymentMethodID2, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestMastercardCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Update subscription payment method
	updateReq := map[string]interface{}{
		"payment_method_id": paymentMethodID2,
	}

	updateResp, err := client.Do("PUT", "/api/v1/subscriptions/"+subscriptionID+"/payment-method", updateReq)
	require.NoError(t, err)
	defer updateResp.Body.Close()

	assert.Equal(t, 200, updateResp.StatusCode, "Update should succeed")

	var updateResult map[string]interface{}
	err = testutil.DecodeResponse(updateResp, &updateResult)
	require.NoError(t, err)

	assert.Equal(t, paymentMethodID2, updateResult["payment_method_id"], "Should use new payment method")
	t.Logf("Updated subscription %s to use new payment method %s", subscriptionID, paymentMethodID2)
}

// TestSubscription_FailedRecurringBilling tests handling of failed recurring billing
func TestSubscription_FailedRecurringBilling(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-sub-008"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method with decline test card
	declineCard := testutil.TestCard{
		Number:   "4000000000000002", // Test card that will decline
		ExpMonth: "12",
		ExpYear:  "2025",
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "visa",
		LastFour: "0002",
	}

	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		declineCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Create subscription
	subReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"plan_id":           "monthly-fail-test",
		"amount":            "14.99",
		"currency":          "USD",
		"billing_cycle":     "monthly",
		"start_date":        time.Now().Format(time.RFC3339),
	}

	subResp, err := client.Do("POST", "/api/v1/subscriptions", subReq)
	require.NoError(t, err)
	defer subResp.Body.Close()

	var subResult map[string]interface{}
	err = testutil.DecodeResponse(subResp, &subResult)
	require.NoError(t, err)

	subscriptionID := subResult["subscription_id"].(string)
	time.Sleep(2 * time.Second)

	// Attempt recurring billing (should fail)
	billingReq := map[string]interface{}{
		"subscription_id": subscriptionID,
	}

	billingResp, err := client.Do("POST", "/api/v1/subscriptions/"+subscriptionID+"/bill", billingReq)
	require.NoError(t, err)
	defer billingResp.Body.Close()

	// Should still return transaction, but with failed status
	if billingResp.StatusCode == 200 {
		var billingResult map[string]interface{}
		err = testutil.DecodeResponse(billingResp, &billingResult)
		require.NoError(t, err)

		// Verify transaction failed
		assert.Equal(t, "failed", billingResult["status"])
		assert.False(t, billingResult["is_approved"].(bool))

		t.Logf("Failed recurring billing handled correctly - Transaction ID: %s",
			billingResult["transaction_id"])
	} else {
		t.Logf("Recurring billing returned error status (acceptable): %d", billingResp.StatusCode)
	}

	time.Sleep(2 * time.Second)

	// Verify subscription status (might be marked as past_due)
	getResp, err := client.Do("GET", "/api/v1/subscriptions/"+subscriptionID, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()

	var getResult map[string]interface{}
	err = testutil.DecodeResponse(getResp, &getResult)
	require.NoError(t, err)

	// Status might be past_due or remain active depending on retry logic
	status := getResult["status"].(string)
	assert.Contains(t, []string{"active", "past_due"}, status,
		"Subscription should be active or past_due after failed billing")

	t.Logf("Subscription status after failed billing: %s", status)
}
