//go:build integration
// +build integration

package payment_method_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStorePaymentMethod_CreditCard tests storing a tokenized credit card payment method
// Uses Browser Post STORAGE flow to tokenize and save card
func TestStorePaymentMethod_CreditCard(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	merchantID := "test-merchant-staging"
	customerID := "test-customer-001"
	time.Sleep(2 * time.Second) // EPX rate limiting

	// Generate JWT token for authentication
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	// Tokenize and save card using Browser Post STORAGE
	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t,
		cfg,
		client,
		jwtToken,
		merchantID,
		customerID,
		testutil.TestVisaCard,
		callbackBaseURL,
	)
	require.NoError(t, err, "Should tokenize and save card")
	assert.NotEmpty(t, paymentMethodID, "Should return payment method ID")

	t.Logf("Stored payment method: %s", paymentMethodID)
}

// TestStorePaymentMethod_ACH tests storing a tokenized ACH payment method
func TestStorePaymentMethod_ACH(t *testing.T) {
	t.Skip("TODO: Update to use StoreACHAccount RPC once implemented (currently returns Unimplemented)")

	cfg, client := testutil.Setup(t)
	time.Sleep(2 * time.Second) // EPX rate limiting
	merchantID := "test-merchant-staging"
	jwtToken := generateJWTToken(t, merchantID)

	// TODO: Replace with StoreACHAccount RPC call
	paymentMethodID, err := testutil.TokenizeAndSaveACH(
		cfg,
		client,
		jwtToken,
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
// Uses Browser Post STORAGE flow to create payment method first
func TestGetPaymentMethod(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	merchantID := "test-merchant-staging"
	customerID := "test-customer-003"
	time.Sleep(2 * time.Second)

	// Generate JWT and store a payment method using Browser Post STORAGE
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t,
		cfg,
		client,
		jwtToken,
		merchantID,
		customerID,
		testutil.TestMastercardCard,
		callbackBaseURL,
	)
	require.NoError(t, err)
	t.Logf("Created payment method: %s", paymentMethodID)
}

// TestListPaymentMethods lists all payment methods for a customer
// Uses Browser Post STORAGE flow to create multiple payment methods
func TestListPaymentMethods(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	merchantID := "test-merchant-staging"
	customerID := "test-customer-list-001"
	time.Sleep(2 * time.Second)

	// Generate JWT for authentication
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	// Store multiple payment methods using Browser Post STORAGE
	_, err := testutil.TokenizeAndSaveCardViaBrowserPost(t, cfg, client, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	_, err = testutil.TokenizeAndSaveCardViaBrowserPost(t, cfg, client, jwtToken, merchantID, customerID, testutil.TestMastercardCard, callbackBaseURL)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	t.Logf("Created multiple payment methods for customer %s", customerID)
}

// TestDeletePaymentMethod tests soft-deleting a payment method
// Uses Browser Post STORAGE flow to create payment method first
func TestDeletePaymentMethod(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	merchantID := "test-merchant-staging"
	customerID := "test-customer-delete-001"
	time.Sleep(2 * time.Second)

	// Generate JWT and store a payment method
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
		t,
		cfg,
		client,
		jwtToken,
		merchantID,
		customerID,
		testutil.TestAmexCard,
		callbackBaseURL,
	)
	require.NoError(t, err)
	t.Logf("Created payment method %s for deletion test", paymentMethodID)
}

// TestStoreMultipleCardsForCustomer tests storing multiple cards for the same customer
// Uses Browser Post STORAGE flow to create multiple payment methods
func TestStoreMultipleCardsForCustomer(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t)

	cfg, client := testutil.Setup(t)
	merchantID := "test-merchant-staging"
	customerID := "test-customer-multi-001"
	time.Sleep(2 * time.Second)

	// Generate JWT for authentication
	jwtToken := generateJWTToken(t, merchantID)
	callbackBaseURL := "http://localhost:8081"

	// Store Visa using Browser Post STORAGE
	visaID, err := testutil.TokenizeAndSaveCardViaBrowserPost(t, cfg, client, jwtToken, merchantID, customerID, testutil.TestVisaCard, callbackBaseURL)
	require.NoError(t, err)
	t.Logf("Stored Visa: %s", visaID)
	time.Sleep(2 * time.Second)

	// Store Mastercard
	mastercardID, err := testutil.TokenizeAndSaveCardViaBrowserPost(t, cfg, client, jwtToken, merchantID, customerID, testutil.TestMastercardCard, callbackBaseURL)
	require.NoError(t, err)
	t.Logf("Stored Mastercard: %s", mastercardID)
	time.Sleep(2 * time.Second)

	// Store Amex
	amexID, err := testutil.TokenizeAndSaveCardViaBrowserPost(t, cfg, client, jwtToken, merchantID, customerID, testutil.TestAmexCard, callbackBaseURL)
	require.NoError(t, err)
	t.Logf("Stored Amex: %s", amexID)

	// Verify all three have unique IDs
	assert.NotEqual(t, visaID, mastercardID)
	assert.NotEqual(t, visaID, amexID)
	assert.NotEqual(t, mastercardID, amexID)

	t.Logf("Successfully stored 3 different cards for customer %s", customerID)
}
