//go:build integration
// +build integration

package payment_method_test

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStorePaymentMethod_CreditCard tests storing a tokenized credit card payment method
func TestStorePaymentMethod_CreditCard(t *testing.T) {
	t.Skip("TODO: Update to use Browser Post STORAGE flow with TokenizeAndSaveCardViaBrowserPost after removing deprecated SavePaymentMethod RPC")

	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second) // EPX rate limiting

	// TODO: Replace with TokenizeAndSaveCardViaBrowserPost
	// Requires: JWT token, callbackBaseURL, headless Chrome setup
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg,
		client,
		"test-merchant-staging",
		"test-customer-001",
		testutil.TestVisaCard,
	)
	require.NoError(t, err, "Should tokenize and save card")
	assert.NotEmpty(t, paymentMethodID, "Should return payment method ID")

	t.Logf("Stored payment method: %s", paymentMethodID)

	// Verify we can retrieve it
	resp, err := client.Do("GET", fmt.Sprintf("/api/v1/payment-methods/%s", paymentMethodID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Should retrieve payment method")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "visa", result["cardBrand"])
	assert.Equal(t, "1111", result["lastFour"])
}

// TestStorePaymentMethod_ACH tests storing a tokenized ACH payment method
func TestStorePaymentMethod_ACH(t *testing.T) {
	t.Skip("TODO: Update to use StoreACHAccount RPC once implemented (currently returns Unimplemented)")

	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second) // EPX rate limiting

	// TODO: Replace with StoreACHAccount RPC call
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg,
		client,
		"test-merchant-staging",
		"test-customer-002",
		testutil.TestACHChecking,
	)
	require.NoError(t, err, "Should tokenize and save ACH")
	assert.NotEmpty(t, paymentMethodID, "Should return payment method ID")

	t.Logf("Stored ACH payment method: %s", paymentMethodID)

	// Verify we can retrieve it
	resp, err := client.Do("GET", fmt.Sprintf("/api/v1/payment-methods/%s", paymentMethodID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Should retrieve payment method")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "7890", result["lastFour"])
	assert.Equal(t, "checking", result["accountType"])
}

// TestGetPaymentMethod retrieves a stored payment method
func TestGetPaymentMethod(t *testing.T) {
	t.Skip("TODO: Update to use Browser Post STORAGE flow - depends on deprecated TokenizeAndSaveCard")

	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// First, store a payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg,
		client,
		"test-merchant-staging",
		"test-customer-003",
		testutil.TestMastercardCard,
	)
	require.NoError(t, err)

	time.Sleep(1 * time.Second) // Allow propagation

	// Retrieve the payment method
	resp, err := client.Do("GET", fmt.Sprintf("/api/v1/payment-methods/%s", paymentMethodID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Should retrieve payment method")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, paymentMethodID, result["id"])
	assert.Equal(t, "mastercard", result["cardBrand"])
	assert.Equal(t, "4444", result["lastFour"])
	assert.NotEmpty(t, result["createdAt"])

	t.Logf("Retrieved payment method: %+v", result)
}

// TestListPaymentMethods lists all payment methods for a customer
func TestListPaymentMethods(t *testing.T) {
	t.Skip("TODO: Update to use Browser Post STORAGE flow - depends on deprecated TokenizeAndSaveCard")

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-list-001"
	time.Sleep(2 * time.Second)

	// Store multiple payment methods
	_, err := testutil.TokenizeAndSaveCard(cfg, client, "test-merchant-staging", customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	_, err = testutil.TokenizeAndSaveCard(cfg, client, "test-merchant-staging", customerID, testutil.TestMastercardCard)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	_, err = testutil.TokenizeAndSaveACH(cfg, client, "test-merchant-staging", customerID, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// List all payment methods for customer
	resp, err := client.Do("GET", fmt.Sprintf("/api/v1/payment-methods?agent_id=test-merchant-staging&customer_id=%s", customerID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	paymentMethods, ok := result["paymentMethods"].([]interface{})
	require.True(t, ok, "Should have paymentMethods array")
	assert.GreaterOrEqual(t, len(paymentMethods), 3, "Should have at least 3 payment methods")

	t.Logf("Found %d payment methods for customer %s", len(paymentMethods), customerID)
}

// TestDeletePaymentMethod tests soft-deleting a payment method
func TestDeletePaymentMethod(t *testing.T) {
	t.Skip("TODO: Update to use Browser Post STORAGE flow - depends on deprecated TokenizeAndSaveCard")

	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Store a payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg,
		client,
		"test-merchant-staging",
		"test-customer-delete-001",
		testutil.TestAmexCard,
	)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	// Delete the payment method
	resp, err := client.Do("DELETE", fmt.Sprintf("/api/v1/payment-methods/%s", paymentMethodID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Should delete payment method")

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.True(t, result["success"].(bool), "Delete should succeed")
	t.Logf("Deleted payment method: %s", paymentMethodID)

	// Verify it's deleted (should not be in list anymore or marked as deleted)
	time.Sleep(1 * time.Second)
	resp2, err := client.Do("GET", fmt.Sprintf("/api/v1/payment-methods/%s", paymentMethodID), nil)
	require.NoError(t, err)
	defer resp2.Body.Close()

	// Either 404 or returns with deletedAt set
	if resp2.StatusCode == 200 {
		var pm map[string]interface{}
		json.NewDecoder(resp2.Body).Decode(&pm)
		// If soft delete, should have deletedAt
		assert.NotNil(t, pm["deletedAt"], "Should be soft deleted")
	}
}

// TestStorePaymentMethod_ValidationErrors tests validation error handling
func TestStorePaymentMethod_ValidationErrors(t *testing.T) {
	t.Skip("TODO: Update to use ConnectRPC StorePaymentMethod endpoint (deprecated HTTP REST /api/v1/payment-methods removed)")

	_, client := testutil.Setup(t)

	testCases := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
	}{
		{
			name: "missing payment_token",
			request: map[string]interface{}{
				"agent_id":    "test-merchant-staging",
				"customer_id": "test-customer",
			},
			expectedStatus: 400,
		},
		{
			name: "missing agent_id",
			request: map[string]interface{}{
				"customer_id":   "test-customer",
				"payment_token": "fake-token",
			},
			expectedStatus: 400,
		},
		{
			name: "missing customer_id",
			request: map[string]interface{}{
				"agent_id":      "test-merchant-staging",
				"payment_token": "fake-token",
			},
			expectedStatus: 400,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			time.Sleep(1 * time.Second) // Rate limiting

			resp, err := client.Do("POST", "/api/v1/payment-methods", tc.request)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode, tc.name)

			body, _ := io.ReadAll(resp.Body)
			t.Logf("Validation error response: %s", string(body))
		})
	}
}

// TestStoreMultipleCardsForCustomer tests storing multiple cards for the same customer
func TestStoreMultipleCardsForCustomer(t *testing.T) {
	t.Skip("TODO: Update to use Browser Post STORAGE flow with TokenizeAndSaveCardViaBrowserPost (deprecated TokenizeAndSaveCard removed)")

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-multi-001"
	time.Sleep(2 * time.Second)

	// Store Visa
	visaID, err := testutil.TokenizeAndSaveCard(cfg, client, "test-merchant-staging", customerID, testutil.TestVisaCard)
	require.NoError(t, err)
	t.Logf("Stored Visa: %s", visaID)

	time.Sleep(2 * time.Second)

	// Store Mastercard
	mastercardID, err := testutil.TokenizeAndSaveCard(cfg, client, "test-merchant-staging", customerID, testutil.TestMastercardCard)
	require.NoError(t, err)
	t.Logf("Stored Mastercard: %s", mastercardID)

	time.Sleep(2 * time.Second)

	// Store Amex
	amexID, err := testutil.TokenizeAndSaveCard(cfg, client, "test-merchant-staging", customerID, testutil.TestAmexCard)
	require.NoError(t, err)
	t.Logf("Stored Amex: %s", amexID)

	// Verify all three are saved
	assert.NotEqual(t, visaID, mastercardID)
	assert.NotEqual(t, visaID, amexID)
	assert.NotEqual(t, mastercardID, amexID)

	// List and verify
	time.Sleep(1 * time.Second)
	resp, err := client.Do("GET", fmt.Sprintf("/api/v1/payment-methods?agent_id=test-merchant-staging&customer_id=%s", customerID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	paymentMethods, ok := result["paymentMethods"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(paymentMethods), 3, "Should have at least 3 payment methods")
}
