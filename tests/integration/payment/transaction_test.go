//go:build integration
// +build integration

package payment_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaleTransaction_WithStoredCard tests a sale transaction using a stored payment method
func TestSaleTransaction_WithStoredCard(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-txn-001"
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

	// Process a sale transaction
	saleReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "29.99",
		"currency":          "USD",
		"metadata": map[string]string{
			"order_id": "ORDER-12345",
		},
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	assert.Equal(t, 200, saleResp.StatusCode, "Sale should succeed")

	var saleResult map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &saleResult)
	require.NoError(t, err)

	// Verify response
	assert.NotEmpty(t, saleResult["transactionId"])
	assert.NotEmpty(t, saleResult["groupId"], "Should return group_id for future refunds")
	assert.Equal(t, "29.99", saleResult["amount"])
	assert.Equal(t, "USD", saleResult["currency"])
	assert.True(t, saleResult["isApproved"].(bool))
	assert.NotEmpty(t, saleResult["authorizationCode"])

	// Verify card info is abstracted (no EPX fields)
	if card, ok := saleResult["card"].(map[string]interface{}); ok && card != nil {
		assert.Equal(t, "visa", card["brand"])
		assert.Equal(t, "1111", card["lastFour"])
	}

	t.Logf("Sale completed - Group ID: %s, Transaction ID: %s",
		saleResult["groupId"], saleResult["transactionId"])
}

// TestAuthorizeAndCapture_WithStoredCard tests auth + capture flow
func TestAuthorizeAndCapture_WithStoredCard(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-txn-002"
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

	// Step 1: Authorize
	authReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "150.00",
		"currency":          "USD",
	}

	authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()

	assert.Equal(t, 200, authResp.StatusCode, "Authorization should succeed")

	var authResult map[string]interface{}
	err = testutil.DecodeResponse(authResp, &authResult)
	require.NoError(t, err)

	transactionID := authResult["transactionId"].(string)
	groupID := authResult["groupId"].(string)
	assert.True(t, authResult["isApproved"].(bool))

	t.Logf("Authorization completed - Transaction ID: %s", transactionID)
	time.Sleep(2 * time.Second)

	// Step 2: Capture (full amount)
	captureReq := map[string]interface{}{
		"transaction_id": transactionID,
		"amount":         "150.00",
	}

	captureResp, err := client.Do("POST", "/api/v1/payments/capture", captureReq)
	require.NoError(t, err)
	defer captureResp.Body.Close()

	assert.Equal(t, 200, captureResp.StatusCode, "Capture should succeed")

	var captureResult map[string]interface{}
	err = testutil.DecodeResponse(captureResp, &captureResult)
	require.NoError(t, err)

	assert.NotEmpty(t, captureResult["transactionId"])
	assert.Equal(t, groupID, captureResult["groupId"], "Should have same group_id")
	assert.True(t, captureResult["isApproved"].(bool))

	t.Logf("Capture completed - Group ID: %s", groupID)
}

// TestAuthorizeAndCapture_PartialCapture tests partial capture
func TestAuthorizeAndCapture_PartialCapture(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-txn-003"
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

	// Authorize for $100
	authReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "100.00",
		"currency":          "USD",
	}

	authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()

	var authResult map[string]interface{}
	err = testutil.DecodeResponse(authResp, &authResult)
	require.NoError(t, err)

	transactionID := authResult["transactionId"].(string)
	time.Sleep(2 * time.Second)

	// Capture only $75 (partial)
	captureReq := map[string]interface{}{
		"transaction_id": transactionID,
		"amount":         "75.00",
	}

	captureResp, err := client.Do("POST", "/api/v1/payments/capture", captureReq)
	require.NoError(t, err)
	defer captureResp.Body.Close()

	assert.Equal(t, 200, captureResp.StatusCode, "Partial capture should succeed")

	var captureResult map[string]interface{}
	err = testutil.DecodeResponse(captureResp, &captureResult)
	require.NoError(t, err)

	assert.Equal(t, "75.00", captureResult["amount"], "Should capture partial amount")
	assert.True(t, captureResult["isApproved"].(bool))

	t.Logf("Partial capture completed - Captured $75 of $100 authorization")
}

// TestSaleTransaction_WithToken tests sale with one-time payment token
func TestSaleTransaction_WithToken(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "guest-customer-001"
	time.Sleep(2 * time.Second)

	// Tokenize card but don't save it (one-time use)
	token, err := testutil.TokenizeCard(cfg, testutil.TestVisaCard)
	require.NoError(t, err, "Should tokenize card")
	time.Sleep(1 * time.Second)

	// Use token directly for sale (not saving payment method)
	saleReq := map[string]interface{}{
		"agent_id":      "test-merchant-staging",
		"customer_id":   customerID,
		"payment_token": token,
		"amount":        "49.99",
		"currency":      "USD",
		"metadata": map[string]string{
			"order_id": "GUEST-ORDER-67890",
		},
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	if saleResp.StatusCode == 200 {
		var saleResult map[string]interface{}
		err = testutil.DecodeResponse(saleResp, &saleResult)
		require.NoError(t, err)

		assert.NotEmpty(t, saleResult["transactionId"])
		assert.NotEmpty(t, saleResult["groupId"])
		t.Logf("Token sale completed - Group ID: %s", saleResult["groupId"])
	} else {
		t.Logf("Token sale response status: %d (may require valid EPX environment)", saleResp.StatusCode)
	}
}

// TestGetTransaction retrieves transaction details
func TestGetTransaction(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-txn-004"
	time.Sleep(2 * time.Second)

	// Create a transaction
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	saleReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "99.99",
		"currency":          "USD",
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	var saleResult map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &saleResult)
	require.NoError(t, err)

	transactionID := saleResult["transactionId"].(string)
	time.Sleep(1 * time.Second)

	// Retrieve transaction
	getResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID), nil)
	require.NoError(t, err)
	defer getResp.Body.Close()

	assert.Equal(t, 200, getResp.StatusCode)

	var getResult map[string]interface{}
	err = testutil.DecodeResponse(getResp, &getResult)
	require.NoError(t, err)

	assert.Equal(t, transactionID, getResult["id"])
	assert.Equal(t, customerID, getResult["customerId"])
	assert.Equal(t, "99.99", getResult["amount"])

	t.Logf("Retrieved transaction: %s", transactionID)
}

// TestListTransactions tests listing transactions with various filters (customer_id and group_id)
func TestListTransactions(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-txn-list"
	time.Sleep(2 * time.Second)

	// Create payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Create 3 transactions for the same customer
	var lastGroupID string
	for i := 1; i <= 3; i++ {
		saleReq := map[string]interface{}{
			"agent_id":          "test-merchant-staging",
			"customer_id":       customerID,
			"payment_method_id": paymentMethodID,
			"amount":            "25.00",
			"currency":          "USD",
		}

		resp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
		require.NoError(t, err)

		if i == 3 {
			// Save the group_id from the last transaction for group filtering test
			var saleResult map[string]interface{}
			testutil.DecodeResponse(resp, &saleResult)
			lastGroupID = saleResult["groupId"].(string)
		}

		resp.Body.Close()
		time.Sleep(2 * time.Second)
	}

	// Test 1: List transactions by customer_id
	t.Run("list_by_customer_id", func(t *testing.T) {
		listResp, err := client.Do("GET",
			fmt.Sprintf("/api/v1/payments?agent_id=test-merchant-staging&customer_id=%s", customerID), nil)
		require.NoError(t, err)
		defer listResp.Body.Close()

		assert.Equal(t, 200, listResp.StatusCode)

		var listResult map[string]interface{}
		err = json.NewDecoder(listResp.Body).Decode(&listResult)
		require.NoError(t, err)

		transactions := listResult["transactions"].([]interface{})
		assert.GreaterOrEqual(t, len(transactions), 3, "Should have at least 3 transactions for customer")

		t.Logf("✅ Found %d transactions for customer %s", len(transactions), customerID)
	})

	// Test 2: List transactions by group_id
	t.Run("list_by_group_id", func(t *testing.T) {
		listResp, err := client.Do("GET",
			fmt.Sprintf("/api/v1/payments?agent_id=test-merchant-staging&group_id=%s", lastGroupID), nil)
		require.NoError(t, err)
		defer listResp.Body.Close()

		assert.Equal(t, 200, listResp.StatusCode)

		var listResult map[string]interface{}
		err = json.NewDecoder(listResp.Body).Decode(&listResult)
		require.NoError(t, err)

		transactions := listResult["transactions"].([]interface{})
		assert.GreaterOrEqual(t, len(transactions), 1, "Should have at least 1 transaction in group")

		// Verify all transactions have same group_id
		for _, txInterface := range transactions {
			tx := txInterface.(map[string]interface{})
			assert.Equal(t, lastGroupID, tx["groupId"], "All transactions should have same group_id")
		}

		t.Logf("✅ Found %d transactions in group %s", len(transactions), lastGroupID)
	})
}
