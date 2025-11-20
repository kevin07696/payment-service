package testutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
var (
	TestVisaCard = TestCard{
		Number:   "4111111111111111",
		ExpMonth: "12",
		ExpYear:  "2025",
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "visa",
		LastFour: "1111",
	}

	TestMastercardCard = TestCard{
		Number:   "5555555555554444",
		ExpMonth: "12",
		ExpYear:  "2025",
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "mastercard",
		LastFour: "4444",
	}

	TestAmexCard = TestCard{
		Number:   "378282246310005",
		ExpMonth: "12",
		ExpYear:  "2025",
		CVV:      "1234",
		ZipCode:  "12345",
		CardType: "amex",
		LastFour: "0005",
	}

	TestDiscoverCard = TestCard{
		Number:   "6011111111111117",
		ExpMonth: "12",
		ExpYear:  "2025",
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
		ExpYear:  "2025",
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "visa",
		LastFour: "1111",
	}

	TestMastercardDebitCard = TestCard{
		Number:   "5555555555554444",
		ExpMonth: "12",
		ExpYear:  "2025",
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "mastercard",
		LastFour: "4444",
	}
)

// SkipIfBRICStorageUnavailable skips tests that require EPX BRIC Storage
// BRIC Storage (CCE8/CKC8) is now available in sandbox
// This function is kept for backward compatibility but no longer skips
func SkipIfBRICStorageUnavailable(t *testing.T) {
	// BRIC Storage is now available - no longer skip
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

// SavePaymentMethod saves a tokenized payment method via the API
func SavePaymentMethod(client *Client, merchantID, customerID, token string, card *TestCard, ach *TestACH) (string, error) {
	req := map[string]interface{}{
		"merchant_id":   merchantID,
		"customer_id":   customerID,
		"payment_token": token,
		"is_default":    true,
	}

	if card != nil {
		req["payment_type"] = "PAYMENT_METHOD_TYPE_CREDIT_CARD"
		req["last_four"] = card.LastFour
		req["card_brand"] = card.CardType
		req["card_exp_month"] = parseMonth(card.ExpMonth)
		req["card_exp_year"] = parseYear(card.ExpYear)
	} else if ach != nil {
		req["payment_type"] = "PAYMENT_METHOD_TYPE_ACH"
		req["last_four"] = ach.LastFour
		req["account_type"] = ach.AccountType
		req["bank_name"] = "Test Bank"
	}

	// ConnectRPC endpoint for SavePaymentMethod
	resp, err := client.Do("POST", "/payment_method.v1.PaymentMethodService/SavePaymentMethod", req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to save payment method: status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// ConnectRPC response structure uses camelCase: {"paymentMethodId": "...", "merchantId": "...", ...}
	paymentMethodID, ok := result["paymentMethodId"].(string)
	if !ok || paymentMethodID == "" {
		return "", fmt.Errorf("paymentMethodId not found in response: %+v", result)
	}

	return paymentMethodID, nil
}

// TokenizeAndSaveCard is a convenience function that tokenizes a card and saves it
func TokenizeAndSaveCard(cfg *Config, client *Client, merchantID, customerID string, card TestCard) (string, error) {
	// Tokenize card
	token, err := TokenizeCard(cfg, card)
	if err != nil {
		return "", fmt.Errorf("tokenization failed: %w", err)
	}

	// Small delay to respect rate limits
	time.Sleep(100 * time.Millisecond)

	// Save payment method
	paymentMethodID, err := SavePaymentMethod(client, merchantID, customerID, token, &card, nil)
	if err != nil {
		return "", fmt.Errorf("save payment method failed: %w", err)
	}

	return paymentMethodID, nil
}

// TokenizeAndSaveACH is a convenience function that tokenizes an ACH account and saves it
func TokenizeAndSaveACH(cfg *Config, client *Client, merchantID, customerID string, ach TestACH) (string, error) {
	// Tokenize ACH
	token, err := TokenizeACH(cfg, ach)
	if err != nil {
		return "", fmt.Errorf("tokenization failed: %w", err)
	}

	// Small delay to respect rate limits
	time.Sleep(100 * time.Millisecond)

	// Save payment method
	paymentMethodID, err := SavePaymentMethod(client, merchantID, customerID, token, nil, &ach)
	if err != nil {
		return "", fmt.Errorf("save payment method failed: %w", err)
	}

	return paymentMethodID, nil
}

func parseMonth(month string) int {
	var m int
	fmt.Sscanf(month, "%d", &m)
	return m
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
