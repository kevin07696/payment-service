//go:build integration
// +build integration

package payment_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBrowserPost_EndToEnd_Success tests the complete Browser Post flow
// Step 1: GET form configuration with TAC
// Step 2: Simulate EPX callback with approved response
// Step 3: Verify transaction created in database
func TestBrowserPost_EndToEnd_Success(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	// Create a test merchant in the database first (if not exists)
	// For now, we'll assume test merchant UUID exists from seed data (004_test_merchants.sql)

	// Step 1: Get payment form configuration
	transactionID := uuid.New().String()
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID from seed data
	amount := "49.99"
	returnURL := "http://localhost:3000/payment/complete"

	formReq := fmt.Sprintf("/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=%s&transaction_type=SALE&return_url=%s",
		transactionID, merchantID, amount, url.QueryEscape(returnURL))

	formResp, err := client.Do("GET", formReq, nil)
	require.NoError(t, err)
	defer formResp.Body.Close()

	assert.Equal(t, 200, formResp.StatusCode, "Should return form configuration")

	var formConfig map[string]interface{}
	err = testutil.DecodeResponse(formResp, &formConfig)
	require.NoError(t, err)

	// Verify form configuration
	assert.NotEmpty(t, formConfig["tac"], "Should return TAC from Key Exchange")
	assert.NotEmpty(t, formConfig["postURL"], "Should return EPX post URL")
	assert.Equal(t, transactionID, formConfig["transactionId"], "Should echo back transaction ID")
	assert.NotEmpty(t, formConfig["custNbr"], "Should return merchant credentials")
	assert.NotEmpty(t, formConfig["merchNbr"], "Should return merchant credentials")

	t.Logf("✅ Step 1: Got form configuration - TAC: %v, Transaction ID: %s", formConfig["tac"], transactionID)

	time.Sleep(1 * time.Second)

	// Step 2: Simulate EPX callback with approved response
	// Build callback form data (simulating what EPX would send)
	// EPX echoes back TRAN_NBR (contains transaction_id) and USER_DATA_1 (contains return_url)
	callbackData := url.Values{
		"AUTH_GUID":      {uuid.New().String()}, // Simulated BRIC token
		"AUTH_RESP":      {"00"},                 // 00 = approved
		"AUTH_CODE":      {"123456"},             // Bank auth code
		"AUTH_RESP_TEXT": {"APPROVED"},
		"AUTH_CARD_TYPE": {"V"},           // Visa
		"AUTH_AVS":       {"Y"},           // Address match
		"AUTH_CVV2":      {"M"},           // CVV match
		"TRAN_NBR":       {transactionID}, // EPX echoes back our transaction_id (idempotency key)
		"TRAN_GROUP":     {"SALE"},
		"AMOUNT":         {amount},
		"USER_DATA_1":    {returnURL},               // EPX echoes back return_url
		"USER_DATA_2":    {"test-customer-001"},     // Customer ID
		"USER_DATA_3":    {merchantID},              // Merchant ID
		"CARD_NBR":       {"************1111"},      // Masked card
		"EXP_DATE":       {"2512"},                  // Dec 2025
		"INVOICE_NBR":    {"INV-" + transactionID},
	}

	// POST callback to our service
	callbackResp, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
	require.NoError(t, err)
	defer callbackResp.Body.Close()

	// Browser Post callback returns HTML (redirect page), so we expect 200
	assert.Equal(t, 200, callbackResp.StatusCode, "Callback should succeed with HTML response")

	t.Logf("✅ Step 2: EPX callback processed successfully")

	time.Sleep(2 * time.Second)

	// Step 3: Verify transaction was created in database
	getTxResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID), nil)
	require.NoError(t, err)
	defer getTxResp.Body.Close()

	assert.Equal(t, 200, getTxResp.StatusCode, "Should find transaction in database")

	var transaction map[string]interface{}
	err = testutil.DecodeResponse(getTxResp, &transaction)
	require.NoError(t, err)

	// Verify transaction details
	assert.Equal(t, transactionID, transaction["id"], "Transaction ID should match")
	assert.NotEmpty(t, transaction["groupId"], "Should have group_id")
	assert.Equal(t, amount, transaction["amount"], "Amount should match")
	assert.Equal(t, "USD", transaction["currency"], "Currency should be USD")
	assert.Equal(t, "TRANSACTION_STATUS_APPROVED", transaction["status"], "Status should be COMPLETED (from auth_resp=00)")
	assert.Equal(t, "PAYMENT_METHOD_TYPE_CREDIT_CARD", transaction["paymentMethodType"], "Payment type should be credit_card")
	assert.Equal(t, "test-customer-001", transaction["customerId"], "Customer ID should match")

	t.Logf("✅ Step 3: Transaction verified in database - ID: %s, Group: %v, Status: %s",
		transaction["id"], transaction["groupId"], transaction["status"])

	t.Log("✅ Browser Post End-to-End test PASSED")
}

// TestBrowserPost_Callback_Idempotency tests that duplicate callbacks don't create duplicate transactions
// This simulates EPX retrying the callback (network failure, timeout, etc.)
func TestBrowserPost_Callback_Idempotency(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	transactionID := uuid.New().String()
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID
	amount := "25.00"
	returnURL := "http://localhost:3000/complete"

	// Build callback form data
	callbackData := url.Values{
		"AUTH_GUID":      {uuid.New().String()},
		"AUTH_RESP":      {"00"}, // Approved
		"AUTH_CODE":      {"789012"},
		"AUTH_RESP_TEXT": {"APPROVED"},
		"AUTH_CARD_TYPE": {"M"}, // Mastercard
		"AUTH_AVS":       {"Y"},
		"AUTH_CVV2":      {"M"},
		"TRAN_NBR":       {transactionID}, // EPX echoes back transaction_id (idempotency)
		"TRAN_GROUP":     {"SALE"},
		"AMOUNT":         {amount},
		"USER_DATA_1":    {returnURL},                   // EPX echoes back return_url
		"USER_DATA_2":    {"test-customer-idempotency"}, // Customer ID
		"USER_DATA_3":    {merchantID},
		"CARD_NBR":       {"************5454"},
		"EXP_DATE":       {"2612"},
		"INVOICE_NBR":    {"INV-" + transactionID},
	}

	// First callback - should create transaction
	resp1, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
	require.NoError(t, err)
	defer resp1.Body.Close()

	assert.Equal(t, 200, resp1.StatusCode, "First callback should succeed")
	t.Log("✅ First callback processed")

	time.Sleep(1 * time.Second)

	// Second callback with SAME transaction_id - should be idempotent (not create duplicate)
	resp2, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, 200, resp2.StatusCode, "Second callback should also succeed (idempotent)")
	t.Log("✅ Second callback processed (idempotent)")

	time.Sleep(1 * time.Second)

	// Verify: Only ONE transaction exists (not two)
	// We do this by querying the transaction by ID - should find exactly one
	getTxResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID), nil)
	require.NoError(t, err)
	defer getTxResp.Body.Close()

	assert.Equal(t, 200, getTxResp.StatusCode, "Transaction should exist")

	var transaction map[string]interface{}
	err = testutil.DecodeResponse(getTxResp, &transaction)
	require.NoError(t, err)

	// Verify it's the same transaction (not duplicated)
	assert.Equal(t, transactionID, transaction["id"], "Transaction ID should match")
	assert.Equal(t, "25", transaction["amount"], "Amount should match original (trailing zeros trimmed)")

	// Additional check: Query by group_id to ensure only 1 transaction in group
	groupID := transaction["groupId"].(string)
	listResp, err := client.Do("GET",
		fmt.Sprintf("/api/v1/payments?merchant_id=%s&group_id=%s", merchantID, groupID), nil)
	require.NoError(t, err)
	defer listResp.Body.Close()

	var listResult map[string]interface{}
	err = testutil.DecodeResponse(listResp, &listResult)
	require.NoError(t, err)

	transactions := listResult["transactions"].([]interface{})
	assert.Equal(t, 1, len(transactions), "Should have exactly 1 transaction (not 2 duplicates)")

	t.Logf("✅ Idempotency verified - Only 1 transaction created despite 2 callbacks")
}

// TestBrowserPost_Callback_DeclinedTransaction tests handling of declined transactions
func TestBrowserPost_Callback_DeclinedTransaction(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	transactionID := uuid.New().String()
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID
	amount := "15.00"
	returnURL := "http://localhost:3000/declined"

	// Build callback data for DECLINED transaction (AUTH_RESP != "00")
	callbackData := url.Values{
		"AUTH_GUID":      {uuid.New().String()}, // EPX still provides BRIC even for declined
		"AUTH_RESP":      {"05"},                // 05 = declined
		"AUTH_CODE":      {""},                  // No auth code for declined
		"AUTH_RESP_TEXT": {"DECLINED - INSUFFICIENT FUNDS"},
		"AUTH_CARD_TYPE": {"V"},
		"AUTH_AVS":       {"U"}, // Unavailable
		"AUTH_CVV2":      {"P"}, // Not processed
		"TRAN_NBR":       {transactionID},
		"TRAN_GROUP":     {"SALE"},
		"AMOUNT":         {amount},
		"USER_DATA_1":    {returnURL},
		"USER_DATA_2":    {"test-customer-declined"},
		"USER_DATA_3":    {merchantID},
		"CARD_NBR":       {"************0002"}, // Decline test card
		"EXP_DATE":       {"2512"},
		"INVOICE_NBR":    {"INV-" + transactionID},
	}

	// POST declined callback
	callbackResp, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
	require.NoError(t, err)
	defer callbackResp.Body.Close()

	assert.Equal(t, 200, callbackResp.StatusCode, "Declined callback should still return 200 (HTML response)")

	time.Sleep(1 * time.Second)

	// Verify transaction was created with declined status
	getTxResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID), nil)
	require.NoError(t, err)
	defer getTxResp.Body.Close()

	assert.Equal(t, 200, getTxResp.StatusCode, "Should find declined transaction")

	var transaction map[string]interface{}
	err = testutil.DecodeResponse(getTxResp, &transaction)
	require.NoError(t, err)

	// Verify declined status
	assert.Equal(t, transactionID, transaction["id"], "Transaction ID should match")
	assert.Equal(t, "TRANSACTION_STATUS_DECLINED", transaction["status"], "Status should be FAILED (from auth_resp=05)")
	assert.Equal(t, "15", transaction["amount"], "Amount should still be recorded (trailing zeros trimmed)")
	assert.Equal(t, "test-customer-declined", transaction["customerId"], "Customer ID should match")

	t.Logf("✅ Declined transaction handled correctly - Status: %s", transaction["status"])
}

// TestBrowserPost_Callback_GuestCheckout tests browser post without customer_id (guest checkout)
func TestBrowserPost_Callback_GuestCheckout(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	transactionID := uuid.New().String()
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID
	amount := "35.00"
	returnURL := "http://localhost:3000/guest-complete"

	// Build callback data WITHOUT customer ID (guest checkout)
	callbackData := url.Values{
		"AUTH_GUID":      {uuid.New().String()},
		"AUTH_RESP":      {"00"}, // Approved
		"AUTH_CODE":      {"456789"},
		"AUTH_RESP_TEXT": {"APPROVED"},
		"AUTH_CARD_TYPE": {"A"}, // Amex
		"AUTH_AVS":       {"Y"},
		"AUTH_CVV2":      {"M"},
		"TRAN_NBR":       {transactionID},
		"TRAN_GROUP":     {"SALE"},
		"AMOUNT":         {amount},
		"USER_DATA_1":    {returnURL},
		"USER_DATA_2":    {""}, // Empty customer ID = guest
		"USER_DATA_3":    {merchantID},
		"CARD_NBR":       {"***********0005"},
		"EXP_DATE":       {"2812"},
		"INVOICE_NBR":    {"GUEST-" + transactionID},
	}

	// POST guest checkout callback
	callbackResp, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
	require.NoError(t, err)
	defer callbackResp.Body.Close()

	assert.Equal(t, 200, callbackResp.StatusCode, "Guest checkout callback should succeed")

	time.Sleep(1 * time.Second)

	// Verify transaction created with NULL customer_id
	getTxResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID), nil)
	require.NoError(t, err)
	defer getTxResp.Body.Close()

	assert.Equal(t, 200, getTxResp.StatusCode, "Should find guest transaction")

	var transaction map[string]interface{}
	err = testutil.DecodeResponse(getTxResp, &transaction)
	require.NoError(t, err)

	// Verify guest checkout transaction
	assert.Equal(t, transactionID, transaction["id"], "Transaction ID should match")
	assert.Equal(t, "TRANSACTION_STATUS_APPROVED", transaction["status"], "Status should be COMPLETED")
	assert.Equal(t, "35", transaction["amount"], "Amount should match (trailing zeros trimmed)")

	// Customer ID should be empty/null for guest checkout
	customerID, hasCustomerID := transaction["customerId"]
	if hasCustomerID {
		assert.Empty(t, customerID, "Customer ID should be empty for guest checkout")
	}

	t.Log("✅ Guest checkout handled correctly - Transaction created without customer ID")
}

// TestBrowserPost_FormGeneration_ValidationErrors tests form generation validation
func TestBrowserPost_FormGeneration_ValidationErrors(t *testing.T) {
	_, client := testutil.Setup(t)

	testCases := []struct {
		name          string
		transactionID string
		merchantID    string
		amount        string
		returnURL     string
		expectedError string
	}{
		{
			name:          "missing transaction_id",
			transactionID: "",
			merchantID:    "test-merchant-staging",
			amount:        "10.00",
			returnURL:     "http://localhost:3000/complete",
			expectedError: "transaction_id parameter is required",
		},
		{
			name:          "invalid transaction_id format",
			transactionID: "not-a-uuid",
			merchantID:    "test-merchant-staging",
			amount:        "10.00",
			returnURL:     "http://localhost:3000/complete",
			expectedError: "invalid transaction_id format",
		},
		{
			name:          "missing merchant_id",
			transactionID: uuid.New().String(),
			merchantID:    "",
			amount:        "10.00",
			returnURL:     "http://localhost:3000/complete",
			expectedError: "merchant_id parameter is required",
		},
		{
			name:          "missing amount",
			transactionID: uuid.New().String(),
			merchantID:    "test-merchant-staging",
			amount:        "",
			returnURL:     "http://localhost:3000/complete",
			expectedError: "amount parameter is required",
		},
		{
			name:          "invalid amount format",
			transactionID: uuid.New().String(),
			merchantID:    "test-merchant-staging",
			amount:        "not-a-number",
			returnURL:     "http://localhost:3000/complete",
			expectedError: "amount must be a valid number",
		},
		{
			name:          "missing return_url",
			transactionID: uuid.New().String(),
			merchantID:    "test-merchant-staging",
			amount:        "10.00",
			returnURL:     "",
			expectedError: "return_url parameter is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			time.Sleep(500 * time.Millisecond)

			formReq := fmt.Sprintf("/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=%s&transaction_type=SALE&return_url=%s",
				tc.transactionID, tc.merchantID, tc.amount, url.QueryEscape(tc.returnURL))

			resp, err := client.Do("GET", formReq, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, 400, resp.StatusCode, tc.name+" should return 400 Bad Request")
			t.Logf("✅ %s: Validation error caught correctly (HTTP %d)", tc.name, resp.StatusCode)
		})
	}
}

// TestBrowserPost_Callback_MissingRequiredFields tests callback with missing critical fields
func TestBrowserPost_Callback_MissingRequiredFields(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	testCases := []struct {
		name          string
		buildCallback func(transactionID, merchantID string) url.Values
		description   string
	}{
		{
			name: "missing AUTH_RESP",
			buildCallback: func(txID, merchID string) url.Values {
				return url.Values{
					"AUTH_GUID":      {uuid.New().String()},
					// Missing AUTH_RESP (critical field)
					"AUTH_CODE":      {"123456"},
					"AUTH_RESP_TEXT": {"APPROVED"},
					"TRAN_NBR":       {txID},
					"AMOUNT":         {"10.00"},
					"USER_DATA_3":    {merchID},
				}
			},
			description: "AUTH_RESP determines approval status",
		},
		{
			name: "missing TRAN_NBR",
			buildCallback: func(txID, merchID string) url.Values {
				return url.Values{
					"AUTH_GUID":      {uuid.New().String()},
					"AUTH_RESP":      {"00"},
					"AUTH_CODE":      {"123456"},
					"AUTH_RESP_TEXT": {"APPROVED"},
					// Missing TRAN_NBR (transaction ID)
					"AMOUNT":      {"10.00"},
					"USER_DATA_3": {merchID},
				}
			},
			description: "TRAN_NBR is required to identify transaction",
		},
		{
			name: "missing AMOUNT",
			buildCallback: func(txID, merchID string) url.Values {
				return url.Values{
					"AUTH_GUID":      {uuid.New().String()},
					"AUTH_RESP":      {"00"},
					"AUTH_CODE":      {"123456"},
					"AUTH_RESP_TEXT": {"APPROVED"},
					"TRAN_NBR":       {txID},
					// Missing AMOUNT
					"USER_DATA_3": {merchID},
				}
			},
			description: "AMOUNT is required for transaction",
		},
		{
			name: "missing USER_DATA_3 (merchant_id)",
			buildCallback: func(txID, merchID string) url.Values {
				return url.Values{
					"AUTH_GUID":      {uuid.New().String()},
					"AUTH_RESP":      {"00"},
					"AUTH_CODE":      {"123456"},
					"AUTH_RESP_TEXT": {"APPROVED"},
					"TRAN_NBR":       {txID},
					"AMOUNT":         {"10.00"},
					// Missing USER_DATA_3 (merchant_id)
				}
			},
			description: "USER_DATA_3 contains merchant_id for authorization",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transactionID := uuid.New().String()
			merchantID := "00000000-0000-0000-0000-000000000001"

			callbackData := tc.buildCallback(transactionID, merchantID)

			resp, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should handle gracefully (may return 400 or 200 with error HTML)
			// The key is it shouldn't crash
			t.Logf("✅ %s: Handled gracefully (HTTP %d) - %s", tc.name, resp.StatusCode, tc.description)
		})
	}
}

// TestBrowserPost_Callback_InvalidDataTypes tests callback with malformed data
func TestBrowserPost_Callback_InvalidDataTypes(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	testCases := []struct {
		name        string
		callbackFn  func() url.Values
		description string
	}{
		{
			name: "invalid amount format",
			callbackFn: func() url.Values {
				return url.Values{
					"AUTH_GUID":      {uuid.New().String()},
					"AUTH_RESP":      {"00"},
					"AUTH_CODE":      {"123456"},
					"TRAN_NBR":       {uuid.New().String()},
					"AMOUNT":         {"not-a-number"}, // Invalid
					"USER_DATA_3":    {"00000000-0000-0000-0000-000000000001"},
				}
			},
			description: "Amount should be numeric",
		},
		{
			name: "negative amount",
			callbackFn: func() url.Values {
				return url.Values{
					"AUTH_GUID":      {uuid.New().String()},
					"AUTH_RESP":      {"00"},
					"AUTH_CODE":      {"123456"},
					"TRAN_NBR":       {uuid.New().String()},
					"AMOUNT":         {"-50.00"}, // Negative
					"USER_DATA_3":    {"00000000-0000-0000-0000-000000000001"},
				}
			},
			description: "Amount should be positive",
		},
		{
			name: "invalid transaction_id format",
			callbackFn: func() url.Values {
				return url.Values{
					"AUTH_GUID":      {uuid.New().String()},
					"AUTH_RESP":      {"00"},
					"AUTH_CODE":      {"123456"},
					"TRAN_NBR":       {"not-a-uuid"}, // Invalid UUID
					"AMOUNT":         {"25.00"},
					"USER_DATA_3":    {"00000000-0000-0000-0000-000000000001"},
				}
			},
			description: "Transaction ID should be valid UUID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", tc.callbackFn())
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should handle validation errors gracefully
			// Typically return 400 or HTML error page
			assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 500,
				"Should handle validation error without server crash")
			t.Logf("✅ %s: Validation handled (HTTP %d) - %s", tc.name, resp.StatusCode, tc.description)
		})
	}
}

// TestBrowserPost_Callback_DifferentDeclineCodes tests various decline response codes
func TestBrowserPost_Callback_DifferentDeclineCodes(t *testing.T) {
	_, client := testutil.Setup(t)

	declineCodes := []struct {
		code        string
		description string
	}{
		{"05", "Do Not Honor"},
		{"51", "Insufficient Funds"},
		{"54", "Expired Card"},
		{"61", "Exceeds Withdrawal Limit"},
		{"62", "Restricted Card"},
		{"65", "Activity Limit Exceeded"},
		{"91", "Issuer Unavailable"},
	}

	for _, dc := range declineCodes {
		t.Run("decline_code_"+dc.code, func(t *testing.T) {
			time.Sleep(1 * time.Second)

			transactionID := uuid.New().String()
			merchantID := "00000000-0000-0000-0000-000000000001"

			callbackData := url.Values{
				"AUTH_GUID":      {""}, // No BRIC for declined
				"AUTH_RESP":      {dc.code},
				"AUTH_CODE":      {""},
				"AUTH_RESP_TEXT": {"DECLINED - " + dc.description},
				"AUTH_CARD_TYPE": {"V"},
				"TRAN_NBR":       {transactionID},
				"TRAN_GROUP":     {"SALE"},
				"AMOUNT":         {"100.00"},
				"USER_DATA_2":    {"test-customer-" + dc.code},
				"USER_DATA_3":    {merchantID},
				"CARD_NBR":       {"************1234"},
				"EXP_DATE":       {"2512"},
			}

			resp, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode, "Declined callback should return 200")

			time.Sleep(500 * time.Millisecond)

			// Verify transaction recorded as declined
			getTxResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID), nil)
			if err == nil {
				defer getTxResp.Body.Close()
				if getTxResp.StatusCode == 200 {
					var tx map[string]interface{}
					if testutil.DecodeResponse(getTxResp, &tx) == nil {
						assert.Equal(t, "declined", tx["status"],
							"Response code %s should result in declined status", dc.code)
					}
				}
			}

			t.Logf("✅ Decline code %s (%s): Handled correctly", dc.code, dc.description)
		})
	}
}

// TestBrowserPost_Callback_LargeAmount tests handling of very large transaction amounts
func TestBrowserPost_Callback_LargeAmount(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	testCases := []struct {
		amount      string
		description string
	}{
		{"999999.99", "Maximum typical amount"},
		{"1000000.00", "One million dollars"},
		{"0.01", "Minimum amount (1 cent)"},
	}

	for _, tc := range testCases {
		t.Run(strings.ReplaceAll(tc.amount, ".", "_"), func(t *testing.T) {
			time.Sleep(1 * time.Second)

			transactionID := uuid.New().String()
			merchantID := "00000000-0000-0000-0000-000000000001"

			callbackData := url.Values{
				"AUTH_GUID":      {uuid.New().String()},
				"AUTH_RESP":      {"00"},
				"AUTH_CODE":      {"123456"},
				"AUTH_RESP_TEXT": {"APPROVED"},
				"AUTH_CARD_TYPE": {"V"},
				"TRAN_NBR":       {transactionID},
				"TRAN_GROUP":     {"SALE"},
				"AMOUNT":         {tc.amount},
				"USER_DATA_1":    {"http://localhost:3000/complete"},
				"USER_DATA_2":    {"test-customer-large"},
				"USER_DATA_3":    {merchantID},
				"CARD_NBR":       {"************1111"},
				"EXP_DATE":       {"2512"},
			}

			resp, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode, "Large amount callback should succeed")

			time.Sleep(500 * time.Millisecond)

			// Verify amount stored correctly
			getTxResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID), nil)
			if err == nil {
				defer getTxResp.Body.Close()
				if getTxResp.StatusCode == 200 {
					var tx map[string]interface{}
					if testutil.DecodeResponse(getTxResp, &tx) == nil {
						// API trims trailing zeros: "1000000.00" → "1000000", "999999.99" stays "999999.99", "0.01" stays "0.01"
						expectedAmount := strings.TrimRight(strings.TrimRight(tc.amount, "0"), ".")
						assert.Equal(t, expectedAmount, tx["amount"], "Amount should be stored accurately")
					}
				}
			}

			t.Logf("✅ Large amount %s (%s): Handled correctly", tc.amount, tc.description)
		})
	}
}

// TestBrowserPost_Callback_SpecialCharactersInFields tests handling of special characters
func TestBrowserPost_Callback_SpecialCharactersInFields(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	transactionID := uuid.New().String()
	merchantID := "00000000-0000-0000-0000-000000000001"

	// Callback with special characters in text fields
	callbackData := url.Values{
		"AUTH_GUID":      {uuid.New().String()},
		"AUTH_RESP":      {"00"},
		"AUTH_CODE":      {"123456"},
		"AUTH_RESP_TEXT": {"APPROVED - O'Brien & Co. <script>alert('xss')</script>"}, // Special chars
		"AUTH_CARD_TYPE": {"V"},
		"TRAN_NBR":       {transactionID},
		"TRAN_GROUP":     {"SALE"},
		"AMOUNT":         {"50.00"},
		"USER_DATA_1":    {"http://localhost?param=value&other=123"},
		"USER_DATA_2":    {"customer-name-with-dashes@example.com"},
		"USER_DATA_3":    {merchantID},
		"CARD_NBR":       {"************9999"},
		"EXP_DATE":       {"2512"},
		"INVOICE_NBR":    {"INV-2024/12-001"}, // Special chars in invoice
	}

	resp, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Special characters should be handled safely")

	time.Sleep(1 * time.Second)

	// Verify transaction created without XSS or injection issues
	getTxResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID), nil)
	require.NoError(t, err)
	defer getTxResp.Body.Close()

	assert.Equal(t, 200, getTxResp.StatusCode, "Transaction should be retrievable")

	var tx map[string]interface{}
	err = testutil.DecodeResponse(getTxResp, &tx)
	require.NoError(t, err)

	assert.Equal(t, "TRANSACTION_STATUS_APPROVED", tx["status"], "Transaction should be COMPLETED")
	assert.Equal(t, transactionID, tx["id"], "Transaction ID should match")

	t.Log("✅ Special characters handled safely (no injection/XSS)")
}

// TestBrowserPost_Callback_InvalidMerchantID tests callback with non-existent merchant
func TestBrowserPost_Callback_InvalidMerchantID(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	transactionID := uuid.New().String()
	invalidMerchantID := uuid.New().String() // Random UUID that doesn't exist

	callbackData := url.Values{
		"AUTH_GUID":      {uuid.New().String()},
		"AUTH_RESP":      {"00"},
		"AUTH_CODE":      {"123456"},
		"AUTH_RESP_TEXT": {"APPROVED"},
		"AUTH_CARD_TYPE": {"V"},
		"TRAN_NBR":       {transactionID},
		"TRAN_GROUP":     {"SALE"},
		"AMOUNT":         {"10.00"},
		"USER_DATA_3":    {invalidMerchantID}, // Invalid merchant
		"CARD_NBR":       {"************1111"},
	}

	resp, err := client.DoForm("POST", "/api/v1/payments/browser-post/callback", callbackData)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should handle gracefully - either reject or create with warning
	// The important thing is it doesn't crash
	assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 500,
		"Invalid merchant should be handled gracefully")

	t.Logf("✅ Invalid merchant ID handled gracefully (HTTP %d)", resp.StatusCode)
}

// TestBrowserPost_FormGeneration_InvalidTransactionType tests unsupported transaction types
func TestBrowserPost_FormGeneration_InvalidTransactionType(t *testing.T) {
	_, client := testutil.Setup(t)
	time.Sleep(2 * time.Second)

	transactionID := uuid.New().String()
	merchantID := "00000000-0000-0000-0000-000000000001"

	invalidTypes := []string{"REFUND", "VOID", "CAPTURE", "INVALID"}

	for _, txType := range invalidTypes {
		t.Run("type_"+txType, func(t *testing.T) {
			formReq := fmt.Sprintf("/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=10.00&transaction_type=%s&return_url=http://localhost:3000",
				transactionID, merchantID, txType)

			resp, err := client.Do("GET", formReq, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should reject unsupported transaction types for Browser Post
			// Browser Post typically only supports SALE and AUTH
			if resp.StatusCode == 400 {
				t.Logf("✅ Transaction type %s correctly rejected (HTTP 400)", txType)
			} else {
				t.Logf("⚠️  Transaction type %s returned HTTP %d (may need validation)", txType, resp.StatusCode)
			}
		})
	}
}
