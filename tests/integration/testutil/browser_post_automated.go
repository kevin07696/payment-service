//go:build integration
// +build integration

package testutil

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// GetRealBRICAutomated uses headless Chrome to get a real BRIC from EPX
// This fully automates the Browser Post flow by controlling a real browser
//
// Flow:
// 1. GET /browser-post/form to get TAC and EPX URL
// 2. Use headless Chrome to fill form and submit to EPX
// 3. EPX processes in browser and redirects to callback
// 4. Query database for BRIC
// 5. Return BRIC for use in subsequent operations
func GetRealBRICAutomated(t *testing.T, client *Client, cfg *Config, amount string, transactionType string, callbackBaseURL string) *RealBRICResult {
	t.Helper()

	// Step 1: Get Browser Post form configuration
	transactionID := uuid.New().String()
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID

	// Use provided callback URL or default to cfg.ServiceURL
	if callbackBaseURL == "" {
		callbackBaseURL = cfg.ServiceURL
	}
	returnURL := callbackBaseURL + "/api/v1/payments/browser-post/callback"

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

	// Extract numeric TRAN_NBR from backend (EPX requires numeric, max 10 digits)
	epxTranNbr, ok := formConfig["epxTranNbr"].(string)
	require.True(t, ok && epxTranNbr != "", "Form config should contain epxTranNbr")

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

	// Extract redirectURL with query params (must match what backend sent to Key Exchange)
	redirectURL, ok := formConfig["redirectURL"].(string)
	require.True(t, ok && redirectURL != "", "Form config should contain redirectURL")

	t.Logf("✅ Got TAC and EPX URL: %s", postURL)

	// Step 2: Create headless Chrome context
	t.Logf("🌐 Starting headless Chrome...")
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set timeout for the whole browser operation
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// EPX Browser Post form uses TRAN_CODE (not TRAN_GROUP)
	// Per EPX Data Dictionary:
	// - TRAN_CODE="SALE" for sale transactions (auth + capture)
	// - TRAN_CODE="AUTH" for authorization-only transactions
	var epxTranCode string
	switch transactionType {
	case "AUTH":
		epxTranCode = "AUTH"
	case "SALE":
		epxTranCode = "SALE"
	default:
		epxTranCode = "SALE" // Default to sale
	}

	// Step 3: Build form URL with parameters (EPX accepts GET parameters)
	// We'll use chromedp to navigate to EPX with form data as POST body
	t.Logf("🚀 Submitting test card to EPX via headless Chrome...")

	// Create HTML form that auto-submits to EPX
	// Per EPX Browser Post API documentation (page 5):
	// - Form must POST to Browser Post API
	// - Form must include TAC, TRAN_CODE, INDUSTRY_TYPE
	// - Form must include merchant credentials (CUST_NBR, MERCH_NBR, DBA_NBR, TERMINAL_NBR)
	// NOTE: REDIRECT_URL is NOT sent in form POST - it's already in the TAC from Key Exchange
	formHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>EPX Test Form</title></head>
<body>
<form id="epxForm" method="POST" action="%s">
    <input type="hidden" name="TAC" value="%s">
    <input type="hidden" name="CUST_NBR" value="%s">
    <input type="hidden" name="MERCH_NBR" value="%s">
    <input type="hidden" name="DBA_NBR" value="%s">
    <input type="hidden" name="TERMINAL_NBR" value="%s">
    <input type="hidden" name="TRAN_NBR" value="%s">
    <input type="hidden" name="TRAN_CODE" value="%s">
    <input type="hidden" name="AMOUNT" value="%s">
    <input type="hidden" name="ACCOUNT_NBR" value="4111111111111111">
    <input type="hidden" name="EXP_DATE" value="2512">
    <input type="hidden" name="CVV" value="123">
    <input type="hidden" name="USER_DATA_1" value="%s">
    <input type="hidden" name="USER_DATA_2" value="test-customer">
    <input type="hidden" name="USER_DATA_3" value="%s">
    <input type="hidden" name="INDUSTRY_TYPE" value="E">
</form>
<script>document.getElementById('epxForm').submit();</script>
</body>
</html>`,
		postURL, tac, custNbr, merchNbr, dbaName, terminalNbr,
		epxTranNbr, epxTranCode, amount, transactionID, merchantID)

	// Use data URL to load the form
	dataURL := "data:text/html;base64," + base64Encode(formHTML)

	var finalURL string
	err = chromedp.Run(ctx,
		// Navigate to our form page
		chromedp.Navigate(dataURL),
		// Wait for navigation to EPX
		chromedp.Sleep(2*time.Second),
		// Wait for either success redirect or error
		chromedp.Sleep(5*time.Second),
		// Get final URL after redirects
		chromedp.Location(&finalURL),
	)

	if err != nil {
		t.Logf("⚠️  Browser automation encountered an issue: %v", err)
		t.Logf("Final URL: %s", finalURL)
	} else {
		t.Logf("✅ Browser completed EPX flow, final URL: %s", finalURL)
	}

	// Give server time to process callback
	time.Sleep(2 * time.Second)

	// Step 4: Verify transaction was created with real BRIC
	t.Logf("🔍 Querying database for transaction...")
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

	t.Logf("✅ Transaction created with REAL BRIC (via automated browser):")
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

// GetRealBRICForAuthAutomated is a convenience wrapper for AUTH transactions using automated browser
// callbackBaseURL: optional public URL for EPX to call back (e.g., ngrok URL). If empty, uses cfg.ServiceURL
func GetRealBRICForAuthAutomated(t *testing.T, client *Client, cfg *Config, amount string, callbackBaseURL string) *RealBRICResult {
	return GetRealBRICAutomated(t, client, cfg, amount, "AUTH", callbackBaseURL)
}

// GetRealBRICForSaleAutomated is a convenience wrapper for SALE transactions using automated browser
// callbackBaseURL: optional public URL for EPX to call back (e.g., ngrok URL). If empty, uses cfg.ServiceURL
func GetRealBRICForSaleAutomated(t *testing.T, client *Client, cfg *Config, amount string, callbackBaseURL string) *RealBRICResult {
	return GetRealBRICAutomated(t, client, cfg, amount, "SALE", callbackBaseURL)
}

// base64Encode encodes a string to base64
func base64Encode(s string) string {
	// Simple base64 encoding for data URLs
	const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	data := []byte(s)
	result := make([]byte, ((len(data)+2)/3)*4)

	for i, j := 0, 0; i < len(data); i, j = i+3, j+4 {
		b := uint32(data[i]) << 16
		if i+1 < len(data) {
			b |= uint32(data[i+1]) << 8
		}
		if i+2 < len(data) {
			b |= uint32(data[i+2])
		}

		result[j] = base64Table[(b>>18)&0x3F]
		result[j+1] = base64Table[(b>>12)&0x3F]
		if i+1 < len(data) {
			result[j+2] = base64Table[(b>>6)&0x3F]
		} else {
			result[j+2] = '='
		}
		if i+2 < len(data) {
			result[j+3] = base64Table[b&0x3F]
		} else {
			result[j+3] = '='
		}
	}

	return string(result)
}
