// +build integration

package testutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestCard represents a test credit card
type TestCard struct {
	Number   string
	ExpMonth string
	ExpYear  string
	CVV      string
	ZipCode  string
	CardType string // "visa", "mastercard", "amex", "discover"
	LastFour string
}

// TestACH represents a test ACH account
type TestACH struct {
	RoutingNumber string
	AccountNumber string
	AccountType   string // "checking" or "savings"
	LastFour      string
}

// Test cards (EPX sandbox)
// Note: Expiration dates are set dynamically to current year + 1 to ensure they're always valid
var (
	TestVisaCard = TestCard{
		Number:   "4111111111111111",
		ExpMonth: "06",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "visa",
		LastFour: "1111",
	}

	TestMastercardCard = TestCard{
		Number:   "5555555555554444",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "mastercard",
		LastFour: "4444",
	}

	TestAmexCard = TestCard{
		Number:   "378282246310005",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "1234",
		ZipCode:  "12345",
		CardType: "amex",
		LastFour: "0005",
	}

	TestDiscoverCard = TestCard{
		Number:   "6011111111111117",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "discover",
		LastFour: "1117",
	}

	TestACHChecking = TestACH{
		RoutingNumber: "021000021",
		AccountNumber: "1234567890",
		AccountType:   "checking",
		LastFour:      "7890",
	}

	TestACHSavings = TestACH{
		RoutingNumber: "021000021",
		AccountNumber: "9876543210",
		AccountType:   "savings",
		LastFour:      "3210",
	}

	// Test debit cards (for PIN-less debit DB0P transactions)
	// PIN-less debit uses same card numbers as credit cards but different transaction type
	TestVisaDebitCard = TestCard{
		Number:   "4111111111111111",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "visa",
		LastFour: "1111",
	}

	TestMastercardDebitCard = TestCard{
		Number:   "5555555555554444",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "mastercard",
		LastFour: "4444",
	}
)

// SkipIfBRICStorageUnavailable skips tests that require EPX BRIC Storage
// BRIC Storage (CCE8/CKC8) is now available and working for ACH and credit cards
// This function is kept for backward compatibility but no longer skips
func SkipIfBRICStorageUnavailable(t *testing.T) {
	// BRIC Storage is working - no longer skip
	// Tests will fail if credentials are missing, which is the desired behavior
}

// TokenizeCard tokenizes a credit card using EPX BRIC Storage (CCE8) and returns the Storage BRIC
// Based on EPX Transaction Specs - BRIC Storage.pdf (page 4)
// NOTE: Requires BRIC Storage to be enabled by EPX in sandbox merchant account
func TokenizeCard(cfg *Config, card TestCard) (string, error) {
	// Validate EPX credentials are configured
	if cfg.EPXMac == "" {
		return "", fmt.Errorf("EPX_MAC_STAGING environment variable is required for tokenization")
	}

	// Build XML request for BRIC Storage (CCE8)
	batchID := time.Now().Format("20060102")
	tranNbr := fmt.Sprintf("%d", time.Now().Unix())
	expDate := fmt.Sprintf("%s%s", card.ExpMonth, card.ExpYear[2:]) // Format: MMYY

	xmlRequest := fmt.Sprintf(`<DETAIL cust_nbr="%s" merch_nbr="%s" dba_nbr="%s" terminal_nbr="%s">
<TRAN_TYPE>CCE8</TRAN_TYPE>
<BATCH_ID>%s</BATCH_ID>
<TRAN_NBR>%s</TRAN_NBR>
<ACCOUNT_NBR>%s</ACCOUNT_NBR>
<EXP_DATE>%s</EXP_DATE>
<CARD_ENT_METH>E</CARD_ENT_METH>
<INDUSTRY_TYPE>E</INDUSTRY_TYPE>
<ADDRESS>123 Test St</ADDRESS>
<ZIP_CODE>%s</ZIP_CODE>
<CVV2>%s</CVV2>
<FIRST_NAME>Test</FIRST_NAME>
<LAST_NAME>Customer</LAST_NAME>
</DETAIL>`,
		cfg.EPXCustNbr, cfg.EPXMerchNbr, cfg.EPXDBANbr, cfg.EPXTerminalNbr,
		batchID, tranNbr,
		card.Number, expDate, card.ZipCode, card.CVV)

	// Send request to EPX
	epxURL := "https://secure.epxuap.com"
	resp, err := http.Post(epxURL, "application/xml", strings.NewReader(xmlRequest))
	if err != nil {
		return "", fmt.Errorf("EPX BRIC Storage request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read EPX response: %w", err)
	}

	// Parse XML response
	var xmlResp struct {
		Fields []struct {
			Key   string `xml:"KEY,attr"`
			Value string `xml:",chardata"`
		} `xml:"FIELDS>FIELD"`
	}

	// EPX returns nested XML: <RESPONSE><FIELDS><FIELD KEY="...">value</FIELD></FIELDS></RESPONSE>
	if err := xml.Unmarshal(body, &xmlResp); err != nil {
		return "", fmt.Errorf("failed to parse XML response: %w\nRaw: %s", err, string(body))
	}

	// Extract fields from response
	fields := make(map[string]string)
	for _, field := range xmlResp.Fields {
		fields[field.Key] = field.Value
	}

	authResp := fields["AUTH_RESP"]
	authGuid := fields["AUTH_GUID"]
	authRespText := fields["AUTH_RESP_TEXT"]

	// Check for approval (00 = approved, 85 = not declined)
	if authResp != "00" && authResp != "85" {
		return "", fmt.Errorf("EPX BRIC Storage failed: %s (code: %s)", authRespText, authResp)
	}

	if authGuid == "" {
		return "", fmt.Errorf("EPX did not return AUTH_GUID (Storage BRIC)")
	}

	return authGuid, nil
}

// TokenizeACH tokenizes an ACH account using EPX Server Post API with CKC0 (pre-note)
// Returns the AUTH_GUID (Storage BRIC) that can be used for future ACH transactions
// Based on EPX ACH Transaction Specs - Server Post API (HTTPS POST method)
func TokenizeACH(cfg *Config, ach TestACH) (string, error) {
	// Validate EPX credentials are configured
	if cfg.EPXMac == "" {
		return "", fmt.Errorf("EPX_MAC_STAGING environment variable is required for tokenization")
	}

	// Build form data for Server Post API with CKC0 (ACH Pre-Note Debit)
	now := time.Now()
	batchID := now.Format("20060102")                         // YYYYMMDD
	tranNbr := fmt.Sprintf("%d", now.Unix())
	localDate := now.Format("010206")                        // MMDDYY
	localTime := now.Format("150405")                        // HHMMSS

	formData := url.Values{}
	formData.Set("CUST_NBR", cfg.EPXCustNbr)
	formData.Set("MERCH_NBR", cfg.EPXMerchNbr)
	formData.Set("DBA_NBR", cfg.EPXDBANbr)
	formData.Set("TERMINAL_NBR", cfg.EPXTerminalNbr)
	formData.Set("TRAN_TYPE", "CKC0")                         // ACH Checking Pre-Note Debit
	formData.Set("AMOUNT", "0.00")                            // Pre-note is $0.00
	formData.Set("TRAN_NBR", tranNbr)
	formData.Set("BATCH_ID", batchID)
	formData.Set("LOCAL_DATE", localDate)
	formData.Set("LOCAL_TIME", localTime)
	formData.Set("ACCOUNT_NBR", ach.AccountNumber)
	formData.Set("ROUTING_NBR", ach.RoutingNumber)
	formData.Set("CARD_ENT_METH", "X")                        // Manually entered
	formData.Set("INDUSTRY_TYPE", "E")                        // E-commerce
	formData.Set("FIRST_NAME", "Test")
	formData.Set("LAST_NAME", "Customer")

	// Calculate MAC signature (concatenate all values excluding MAC itself)
	macPayload := ""
	for key := range formData {
		if key != "MAC" {
			macPayload += formData.Get(key)
		}
	}
	formData.Set("MAC", calculateMAC(macPayload, cfg.EPXMac))

	// Send request to EPX Server Post API (sandbox)
	epxURL := "https://secure.epxuap.com"
	resp, err := http.PostForm(epxURL, formData)
	if err != nil {
		return "", fmt.Errorf("EPX Server Post request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read EPX response: %w", err)
	}

	// Parse XML response (EPX returns <RESPONSE><FIELDS><FIELD KEY="xxx">value</FIELD></FIELDS></RESPONSE>)
	var xmlResp struct {
		Fields []struct {
			Key   string `xml:"KEY,attr"`
			Value string `xml:",chardata"`
		} `xml:"FIELDS>FIELD"`
	}

	if err := xml.Unmarshal(body, &xmlResp); err != nil {
		return "", fmt.Errorf("failed to parse XML response: %w\nRaw: %s", err, string(body))
	}

	// Extract fields from XML
	fields := make(map[string]string)
	for _, field := range xmlResp.Fields {
		fields[field.Key] = field.Value
	}

	authResp := fields["AUTH_RESP"]
	authGuid := fields["AUTH_GUID"]
	authRespText := fields["AUTH_RESP_TEXT"]

	// Check for approval (00 = approved)
	if authResp != "00" {
		return "", fmt.Errorf("EPX ACH pre-note failed: %s (code: %s)", authRespText, authResp)
	}

	if authGuid == "" {
		return "", fmt.Errorf("EPX did not return AUTH_GUID (Storage BRIC)")
	}

	return authGuid, nil
}

// TokenizeAndSaveCardViaBrowserPost uses Browser Post API to tokenize and save a credit card
// This is the preferred method for merchants using Browser Post API
// Flow:
//  1. Uses Browser Post STORAGE transaction to tokenize the card
//  2. Browser Post callback handler automatically saves the storage BRIC to payment_methods table
//  3. Queries for the payment method that was created
//  4. Returns the payment method ID
//
// jwtToken is required for authentication when querying payment methods
// t is required for Browser Post automation (uses headless Chrome)
// callbackBaseURL is the public URL where EPX can callback (e.g., ngrok URL or localhost:8081)
func TokenizeAndSaveCardViaBrowserPost(t *testing.T, cfg *Config, client *Client, jwtToken, merchantID, customerID string, card TestCard, callbackBaseURL string) (string, error) {
	t.Helper()

	// Step 1: Use Browser Post STORAGE transaction to tokenize the card
	// STORAGE transaction creates a storage BRIC without charging
	// The callback handler will automatically save it to payment_methods table
	cardDetails := &CardDetails{
		Number:  card.Number,
		CVV:     card.CVV,
		ExpDate: card.ExpYear[2:] + card.ExpMonth, // Format: YYMM (EPX Browser Post format)
		Zip:     card.ZipCode,
	}

	bricResult := GetRealBRICAutomated(t, client, cfg, "0.00", "STORAGE", callbackBaseURL, cardDetails, customerID, jwtToken)
	if bricResult == nil {
		return "", fmt.Errorf("Browser Post STORAGE transaction failed: no BRIC returned")
	}

	// Step 2: The Browser Post callback handler automatically saves STORAGE transactions to payment_methods
	// We just need to query for the payment method that was created
	// Wait a moment for the payment method to be saved
	time.Sleep(500 * time.Millisecond)

	// Step 3: Query for the payment method that was auto-saved by the callback handler
	// The callback handler automatically saves STORAGE transactions to payment_methods table
	transactionID := bricResult.TransactionID
	if transactionID == "" {
		return "", fmt.Errorf("Browser Post returned empty transaction ID")
	}

	// List payment methods for this customer to find the one we just created
	client.SetHeader("Authorization", "Bearer "+jwtToken)
	defer client.ClearHeaders()

	resp, err := client.DoConnectRPC("payment_method.v1.PaymentMethodService", "ListPaymentMethods", map[string]interface{}{
		"merchant_id": merchantID,
		"customer_id": customerID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list payment methods: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("list payment methods failed: status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := DecodeResponse(resp, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	paymentMethods, ok := result["paymentMethods"].([]interface{})
	if !ok || len(paymentMethods) == 0 {
		return "", fmt.Errorf("no payment methods found for customer after STORAGE transaction")
	}

	// Return the most recently created payment method (last in list)
	// The callback handler creates the payment method with the storage BRIC
	lastPM := paymentMethods[len(paymentMethods)-1].(map[string]interface{})
	paymentMethodID, ok := lastPM["id"].(string)
	if !ok || paymentMethodID == "" {
		return "", fmt.Errorf("payment method ID not found in response")
	}

	return paymentMethodID, nil
}

func parseMonth(month string) int {
	var m int
	fmt.Sscanf(month, "%d", &m)
	return m
}

// TokenizeAndSaveACH is a stub for ACH account tokenization
// TODO: Implement when StoreACHAccount RPC is available
func TokenizeAndSaveACH(cfg *Config, client *Client, merchantID, customerID string, achAccount TestACH) (string, error) {
	return "", fmt.Errorf("TokenizeAndSaveACH not yet implemented - waiting for StoreACHAccount RPC")
}

// TokenizeAndSaveCard is a stub for saving tokenized cards
// TODO: Update to use Browser Post STORAGE flow
func TokenizeAndSaveCard(cfg *Config, client *Client, merchantID, customerID string, card TestCard) (string, error) {
	return "", fmt.Errorf("TokenizeAndSaveCard deprecated - use TokenizeAndSaveCardViaBrowserPost instead")
}

func parseYear(year string) int {
	var y int
	fmt.Sscanf(year, "%d", &y)
	return y
}

// calculateMAC calculates HMAC-SHA256 MAC for EPX Server Post requests
func calculateMAC(payload, macKey string) string {
	h := hmac.New(sha256.New, []byte(macKey))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}
