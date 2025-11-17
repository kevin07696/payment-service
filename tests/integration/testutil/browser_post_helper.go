package testutil

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// RealBRICResult contains the result of obtaining a real BRIC from EPX
type RealBRICResult struct {
	TransactionID string
	GroupID       string
	BRIC          string
	Amount        string
	MerchantID    string
}

// GetRealBRICFromEPX obtains a real BRIC token by POSTing test card data directly to EPX
// This enables testing of CAPTURE, VOID, and REFUND operations with real EPX integration
//
// Flow:
// 1. GET /browser-post/form to get TAC and EPX URL
// 2. POST test card data directly to EPX (no browser needed!)
// 3. EPX calls our callback with real BRIC
// 4. Return BRIC for use in subsequent operations
func GetRealBRICFromEPX(t *testing.T, client *Client, cfg *Config, amount string, transactionType string) *RealBRICResult {
	t.Helper()

	// Step 1: Get Browser Post form configuration
	transactionID := uuid.New().String()
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID
	returnURL := cfg.ServiceURL + "/api/v1/payments/browser-post/callback"

	formReq := fmt.Sprintf("/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=%s&transaction_type=%s&return_url=%s",
		transactionID, merchantID, amount, transactionType, url.QueryEscape(returnURL))

	t.Logf("🔧 Getting Browser Post form config for %s transaction...", transactionType)
	formResp, err := client.Do("GET", formReq, nil)
	require.NoError(t, err, "Failed to get Browser Post form")
	defer formResp.Body.Close()

	require.Equal(t, 200, formResp.StatusCode, "Form generation should succeed")

	var formConfig map[string]interface{}
	err = DecodeResponse(formResp, &formConfig)
	require.NoError(t, err, "Failed to decode form configuration")

	// Extract EPX details from form config
	tac, ok := formConfig["tac"].(string)
	require.True(t, ok && tac != "", "Form config should contain TAC")

	postURL, ok := formConfig["postURL"].(string)
	require.True(t, ok && postURL != "", "Form config should contain postURL")

	custNbr, ok := formConfig["custNbr"].(string)
	require.True(t, ok && custNbr != "", "Form config should contain custNbr")

	merchNbr, ok := formConfig["merchNbr"].(string)
	require.True(t, ok && merchNbr != "", "Form config should contain merchNbr")

	dbaName, ok := formConfig["dbaName"].(string)
	require.True(t, ok && dbaName != "", "Form config should contain dbaName")

	terminalNbr, ok := formConfig["terminalNbr"].(string)
	require.True(t, ok && terminalNbr != "", "Form config should contain terminalNbr")

	t.Logf("✅ Got TAC and EPX URL: %s", postURL)

	// Step 2: POST test card data directly to EPX
	// Map transaction type to EPX TRAN_GROUP code
	var epxTranGroup string
	switch transactionType {
	case "AUTH":
		epxTranGroup = "A" // A = Authorization only
	case "SALE":
		epxTranGroup = "U" // U = Sale (auth + capture)
	default:
		epxTranGroup = "U" // Default to sale
	}

	epxFormData := url.Values{
		// TAC from Key Exchange
		"TAC": {tac},

		// Merchant credentials
		"CUST_NBR":     {custNbr},
		"MERCH_NBR":    {merchNbr},
		"DBA_NBR":      {dbaName},
		"TERMINAL_NBR": {terminalNbr},

		// Transaction details (EPX echoes these back)
		"TRAN_NBR":   {transactionID},
		"TRAN_GROUP": {epxTranGroup},
		"AMOUNT":     {amount},

		// Test card data (EPX Sandbox test card)
		"CARD_NBR": {"4111111111111111"}, // Visa test card
		"EXP_DATE": {"1225"},             // Dec 2025 (MMYY format)
		"CVV":      {"123"},

		// Return URL (EPX will redirect here after processing)
		"REDIRECT_URL": {returnURL},

		// Pass-through data (EPX echoes back in callback)
		"USER_DATA_1": {returnURL},                 // Return URL
		"USER_DATA_2": {"test-customer-real-bric"}, // Customer ID
		"USER_DATA_3": {merchantID},                // Merchant ID

		// Additional fields
		"INDUSTRY_TYPE": {"E"}, // E-commerce
	}

	t.Logf("🚀 POSTing test card to EPX at %s...", postURL)
	epxResp, err := http.PostForm(postURL, epxFormData)
	require.NoError(t, err, "Failed to POST to EPX")
	defer epxResp.Body.Close()

	t.Logf("✅ EPX responded with status: %d", epxResp.StatusCode)

	// Step 3: Parse EPX HTML response and extract callback form data
	// EPX returns an HTML page with a self-posting form containing the transaction results
	// We need to parse this HTML and POST it to our callback endpoint (simulating what browser does)
	t.Logf("📄 Parsing EPX HTML response to extract callback data...")

	// Read the full response body
	bodyBytes, err := io.ReadAll(epxResp.Body)
	require.NoError(t, err, "Failed to read EPX response")

	responseHTML := string(bodyBytes)

	// Parse callback form data from HTML using simple string extraction
	// EPX returns form fields like: <input type="hidden" name="AUTH_RESP" value="000">
	callbackData := extractCallbackFormData(responseHTML)

	if len(callbackData) == 0 {
		t.Logf("⚠️  No callback form data found in EPX response (might be an error page)")
		t.Logf("Response preview: %s", truncateString(responseHTML, 500))
	} else {
		t.Logf("✅ Extracted %d callback fields from EPX response", len(callbackData))

		// Step 4: Simulate browser POST to our callback endpoint
		t.Logf("📮 POSTing callback data to our server (simulating browser auto-submit)...")

		callbackResp, err := http.PostForm(returnURL, callbackData)
		require.NoError(t, err, "Failed to POST callback to our server")
		defer callbackResp.Body.Close()

		t.Logf("✅ Callback completed with status: %d", callbackResp.StatusCode)

		// Give server a moment to process callback
		time.Sleep(500 * time.Millisecond)
	}

	// Step 4: Verify transaction was created with real BRIC
	getTxResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID), nil)
	require.NoError(t, err, "Failed to get transaction")
	defer getTxResp.Body.Close()

	require.Equal(t, 200, getTxResp.StatusCode, "Transaction should exist in database")

	var transaction map[string]interface{}
	err = DecodeResponse(getTxResp, &transaction)
	require.NoError(t, err, "Failed to decode transaction")

	// Extract group_id from transaction
	groupID, ok := transaction["groupId"].(string)
	require.True(t, ok && groupID != "", "Transaction should have group_id")

	status, ok := transaction["status"].(string)
	require.True(t, ok, "Transaction should have status")

	t.Logf("✅ Transaction created with REAL BRIC:")
	t.Logf("   Transaction ID: %s", transactionID)
	t.Logf("   Group ID: %s", groupID)
	t.Logf("   Status: %s", status)
	t.Logf("   Type: %s", transactionType)

	// Return the result for use in subsequent operations
	return &RealBRICResult{
		TransactionID: transactionID,
		GroupID:       groupID,
		BRIC:          groupID, // BRIC is stored in transaction_groups table, referenced by group_id
		Amount:        amount,
		MerchantID:    merchantID,
	}
}

// GetRealBRICForAuth is a convenience wrapper for AUTH transactions
func GetRealBRICForAuth(t *testing.T, client *Client, cfg *Config, amount string) *RealBRICResult {
	return GetRealBRICFromEPX(t, client, cfg, amount, "AUTH")
}

// GetRealBRICForSale is a convenience wrapper for SALE transactions
func GetRealBRICForSale(t *testing.T, client *Client, cfg *Config, amount string) *RealBRICResult {
	return GetRealBRICFromEPX(t, client, cfg, amount, "SALE")
}

// extractCallbackFormData parses EPX's HTML response and extracts form field values
// EPX returns HTML like: <input type="hidden" name="AUTH_RESP" value="000">
// We extract all name/value pairs to POST back to our callback
func extractCallbackFormData(html string) url.Values {
	data := url.Values{}

	// Regex to match: <input type="hidden" name="FIELD_NAME" value="FIELD_VALUE">
	// This captures both name and value attributes
	inputRegex := regexp.MustCompile(`<input[^>]+name="([^"]+)"[^>]+value="([^"]*)"`)
	matches := inputRegex.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			fieldName := match[1]
			fieldValue := match[2]
			data.Set(fieldName, fieldValue)
		}
	}

	// Also try reverse order: value before name
	inputRegex2 := regexp.MustCompile(`<input[^>]+value="([^"]*)"[^>]+name="([^"]+)"`)
	matches2 := inputRegex2.FindAllStringSubmatch(html, -1)

	for _, match := range matches2 {
		if len(match) >= 3 {
			fieldValue := match[1]
			fieldName := match[2]
			// Only set if not already present
			if data.Get(fieldName) == "" {
				data.Set(fieldName, fieldValue)
			}
		}
	}

	return data
}

// truncateString truncates a string to maxLength and adds "..." if truncated
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}
