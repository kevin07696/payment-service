//go:build ignore

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	EPX_BROWSER_POST_URL = "https://services.epxuap.com/browserpost/"
	EPX_CUST_NBR         = "9001"
	EPX_MERCH_NBR        = "900300"
	EPX_DBA_NBR          = "2"
	EPX_TERMINAL_NBR     = "77"
	EPX_MAC_KEY          = "2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"
)

func main() {
	fmt.Println("=====================================")
	fmt.Println("EPX MAC Calculation Diagnostic")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("Testing different MAC calculation methods...")
	fmt.Println()

	// Test card data
	cardNo := "4788250000028291"
	expMonth := "12"
	expYear := "25"
	cvv := "123"
	amount := "0.00"

	// Method 1: Simple concatenation (what we're currently using)
	fmt.Println("Method 1: Simple Concatenation")
	fmt.Println("-------------------------------")
	macString1 := amount + cardNo + expMonth + expYear + cvv +
		EPX_CUST_NBR + EPX_MERCH_NBR + EPX_DBA_NBR + EPX_TERMINAL_NBR
	mac1 := calculateHMAC(macString1, EPX_MAC_KEY)
	fmt.Printf("Order: Amount|Card|ExpM|ExpY|CVV|Cust|Merch|DBA|Term\n")
	fmt.Printf("String: %.50s...\n", macString1)
	fmt.Printf("MAC: %s\n\n", mac1)

	// Method 2: With separators
	fmt.Println("Method 2: Pipe Separators")
	fmt.Println("-------------------------")
	macString2 := strings.Join([]string{
		amount, cardNo, expMonth, expYear, cvv,
		EPX_CUST_NBR, EPX_MERCH_NBR, EPX_DBA_NBR, EPX_TERMINAL_NBR,
	}, "|")
	mac2 := calculateHMAC(macString2, EPX_MAC_KEY)
	fmt.Printf("String: %.50s...\n", macString2)
	fmt.Printf("MAC: %s\n\n", mac2)

	// Method 3: Different field order (merchant first)
	fmt.Println("Method 3: Merchant Fields First")
	fmt.Println("-------------------------------")
	macString3 := EPX_CUST_NBR + EPX_MERCH_NBR + EPX_DBA_NBR + EPX_TERMINAL_NBR +
		amount + cardNo + expMonth + expYear + cvv
	mac3 := calculateHMAC(macString3, EPX_MAC_KEY)
	fmt.Printf("Order: Cust|Merch|DBA|Term|Amount|Card|ExpM|ExpY|CVV\n")
	fmt.Printf("String: %.50s...\n", macString3)
	fmt.Printf("MAC: %s\n\n", mac3)

	// Method 4: TransType included
	fmt.Println("Method 4: Including TransType")
	fmt.Println("-----------------------------")
	transType := "CKC"
	macString4 := transType + amount + cardNo + expMonth + expYear + cvv +
		EPX_CUST_NBR + EPX_MERCH_NBR + EPX_DBA_NBR + EPX_TERMINAL_NBR
	mac4 := calculateHMAC(macString4, EPX_MAC_KEY)
	fmt.Printf("Order: TransType|Amount|Card|ExpM|ExpY|CVV|Cust|Merch|DBA|Term\n")
	fmt.Printf("String: %.50s...\n", macString4)
	fmt.Printf("MAC: %s\n\n", mac4)

	// Method 5: All form fields
	fmt.Println("Method 5: All Form Fields")
	fmt.Println("-------------------------")
	nameOnCard := "Test Customer"
	street := "123 Main St"
	zip := "12345"
	macString5 := amount + cardNo + expMonth + expYear + cvv +
		nameOnCard + street + zip +
		EPX_CUST_NBR + EPX_MERCH_NBR + EPX_DBA_NBR + EPX_TERMINAL_NBR
	mac5 := calculateHMAC(macString5, EPX_MAC_KEY)
	fmt.Printf("Order: Amount|Card|ExpM|ExpY|CVV|Name|Street|Zip|Cust|Merch|DBA|Term\n")
	fmt.Printf("String: %.50s...\n", macString5)
	fmt.Printf("MAC: %s\n\n", mac5)

	// Test with actual EPX request
	fmt.Println("=====================================")
	fmt.Println("Testing MAC with EPX Request")
	fmt.Println("=====================================")
	fmt.Println()

	// Try Method 1 (our current approach)
	fmt.Println("Sending test request with Method 1 MAC...")
	testEPXRequest(mac1, cardNo, expMonth, expYear, cvv)

	fmt.Println("\n=====================================")
	fmt.Println("Possible Issues:")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("1. MAC Calculation:")
	fmt.Println("   - Field order might be different than expected")
	fmt.Println("   - Some fields might need to be excluded/included")
	fmt.Println("   - Separators might be required between fields")
	fmt.Println()
	fmt.Println("2. Merchant Configuration:")
	fmt.Println("   - Merchant 9001/900300 might not be activated")
	fmt.Println("   - CKC transaction type might not be enabled")
	fmt.Println("   - Browser Post might be disabled for this merchant")
	fmt.Println()
	fmt.Println("3. Request Format:")
	fmt.Println("   - Missing required fields")
	fmt.Println("   - Incorrect field names or values")
	fmt.Println("   - Wrong content type or encoding")
}

func calculateHMAC(message, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func testEPXRequest(mac, cardNo, expMonth, expYear, cvv string) {
	// Build form data
	data := url.Values{}
	data.Set("TransType", "CKC")
	data.Set("Amount", "0.00")
	data.Set("CardNo", cardNo)
	data.Set("ExpMonth", expMonth)
	data.Set("ExpYear", expYear)
	data.Set("CVV2", cvv)
	data.Set("NameOnCard", "Test Customer")
	data.Set("Street", "123 Main St")
	data.Set("Zip", "12345")
	data.Set("CUST_NBR", EPX_CUST_NBR)
	data.Set("MERCH_NBR", EPX_MERCH_NBR)
	data.Set("DBA_NBR", EPX_DBA_NBR)
	data.Set("TERMINAL_NBR", EPX_TERMINAL_NBR)
	data.Set("MAC", mac)
	data.Set("ResponseType", "XML")

	// Send request
	resp, err := http.PostForm(EPX_BROWSER_POST_URL, data)
	if err != nil {
		fmt.Printf("❌ Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Read response
	body, _ := io.ReadAll(resp.Body)
	responseStr := string(body)

	fmt.Printf("Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Response (first 300 chars):\n%.300s...\n", responseStr)

	// Check for specific error messages
	if strings.Contains(responseStr, "unrecoverable error") {
		fmt.Println("❌ Got 'unrecoverable error' - MAC or merchant config issue")
	} else if strings.Contains(responseStr, "BRIC") || strings.Contains(responseStr, "AUTH_GUID") {
		fmt.Println("✅ Success! Token found in response")
	} else if strings.Contains(responseStr, "MAC") {
		fmt.Println("❌ MAC validation error explicitly mentioned")
	}
}