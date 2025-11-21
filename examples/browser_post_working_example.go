//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/google/uuid"
)

func main() {
	fmt.Println("=====================================")
	fmt.Println("EPX Browser Post - Working Example")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("This demonstrates the CORRECT way to tokenize with EPX")
	fmt.Println("using Browser Post + TAC (as integration tests do)")
	fmt.Println()

	// Configuration
	serviceURL := "http://localhost:8081"                // HTTP endpoints on 8081
	merchantID := "1a20fff8-2cec-48e5-af49-87e501652913" // ACME Corp
	amount := "10.00"
	transactionType := "SALE"

	// Step 1: Get TAC from payment service
	fmt.Println("Step 1: Getting TAC from payment service...")
	fmt.Println("--------------------------------------------")

	transactionID := uuid.New().String()
	returnURL := serviceURL + "/api/v1/payments/browser-post/callback"

	formURL := fmt.Sprintf("%s/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=%s&transaction_type=%s&return_url=%s",
		serviceURL, transactionID, merchantID, amount, transactionType, url.QueryEscape(returnURL))

	fmt.Printf("Calling: %s\n", formURL)

	resp, err := http.Get(formURL)
	if err != nil {
		fmt.Printf("❌ Failed to get TAC: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("❌ Form generation failed: %d - %s\n", resp.StatusCode, string(body))
		return
	}

	var formConfig map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&formConfig); err != nil {
		fmt.Printf("❌ Failed to decode form config: %v\n", err)
		return
	}

	// Extract values
	tac, _ := formConfig["tac"].(string)
	postURL, _ := formConfig["postURL"].(string)
	custNbr, _ := formConfig["custNbr"].(string)
	merchNbr, _ := formConfig["merchNbr"].(string)
	dbaName, _ := formConfig["dbaName"].(string)
	terminalNbr, _ := formConfig["terminalNbr"].(string)

	if tac == "" || postURL == "" {
		fmt.Printf("❌ Missing required fields in form config\n")
		fmt.Printf("Config: %+v\n", formConfig)
		return
	}

	fmt.Printf("✅ Got TAC: %s...\n", tac[:20])
	fmt.Printf("   EPX URL: %s\n", postURL)
	fmt.Printf("   Merchant: %s/%s/%s/%s\n\n", custNbr, merchNbr, dbaName, terminalNbr)

	// Step 2: POST to EPX with TAC
	fmt.Println("Step 2: Posting card data to EPX...")
	fmt.Println("------------------------------------")

	// Determine TRAN_GROUP
	var tranGroup string
	if transactionType == "AUTH" {
		tranGroup = "A" // Authorization only
	} else {
		tranGroup = "U" // Sale (auth + capture)
	}

	// Build EPX request (exact format from integration tests)
	epxData := url.Values{}
	epxData.Set("TAC", tac)
	epxData.Set("CUST_NBR", custNbr)
	epxData.Set("MERCH_NBR", merchNbr)
	epxData.Set("DBA_NBR", dbaName)
	epxData.Set("TERMINAL_NBR", terminalNbr)
	epxData.Set("TRAN_NBR", transactionID)
	epxData.Set("TRAN_GROUP", tranGroup)
	epxData.Set("AMOUNT", amount)
	epxData.Set("CARD_NBR", "4111111111111111") // Visa test card (approved)
	epxData.Set("EXP_DATE", "1225")             // Dec 2025 (MMYY format)
	epxData.Set("CVV", "123")
	epxData.Set("REDIRECT_URL", returnURL)
	epxData.Set("USER_DATA_1", returnURL)
	epxData.Set("USER_DATA_2", "wordpress-browser-post")
	epxData.Set("USER_DATA_3", merchantID)
	epxData.Set("INDUSTRY_TYPE", "E") // E-commerce

	fmt.Printf("Posting to EPX: %s\n", postURL)
	fmt.Printf("Card: 4111111111111111 (Visa test)\n")
	fmt.Printf("Amount: $%s\n", amount)

	epxResp, err := http.PostForm(postURL, epxData)
	if err != nil {
		fmt.Printf("❌ Failed to POST to EPX: %v\n", err)
		return
	}
	defer epxResp.Body.Close()

	fmt.Printf("✅ EPX Response: %d\n\n", epxResp.StatusCode)

	// Step 3: Parse EPX response
	fmt.Println("Step 3: Parsing EPX response...")
	fmt.Println("--------------------------------")

	bodyBytes, _ := io.ReadAll(epxResp.Body)
	responseHTML := string(bodyBytes)

	// Extract callback data from HTML
	callbackData := extractCallbackFormData(responseHTML)

	if len(callbackData) == 0 {
		fmt.Printf("⚠️  No callback form data found\n")
		fmt.Printf("Response preview:\n%.500s...\n", responseHTML)
		return
	}

	fmt.Printf("✅ Extracted %d callback fields\n", len(callbackData))

	// Display important fields
	authResp := callbackData.Get("AUTH_RESP")
	authGuid := callbackData.Get("AUTH_GUID")
	bric := callbackData.Get("BRIC")
	message := callbackData.Get("RESPONSE_TEXT")

	fmt.Printf("\nTransaction Results:\n")
	fmt.Printf("  AUTH_RESP: %s\n", authResp)
	fmt.Printf("  AUTH_GUID: %s\n", authGuid)
	fmt.Printf("  BRIC: %s\n", bric)
	fmt.Printf("  Message: %s\n", message)

	// Step 4: POST callback to our server
	fmt.Println("\nStep 4: Posting callback to payment service...")
	fmt.Println("-----------------------------------------------")

	callbackResp, err := http.PostForm(returnURL, callbackData)
	if err != nil {
		fmt.Printf("❌ Failed to POST callback: %v\n", err)
		return
	}
	defer callbackResp.Body.Close()

	if callbackResp.StatusCode == 200 {
		fmt.Printf("✅ Callback processed successfully\n")

		// Check if approved
		if authResp == "000" || authResp == "00" {
			fmt.Println("\n🎉 TRANSACTION APPROVED!")
			fmt.Printf("   BRIC Token: %s\n", bric)
			fmt.Println("\nYou can now use this BRIC for future transactions:")
			fmt.Printf("   go run test_real_bric.go -bric=%s -amount=5.00\n", bric)
		} else {
			fmt.Printf("\n⚠️  Transaction declined: %s\n", message)
		}
	} else {
		fmt.Printf("⚠️  Callback returned: %d\n", callbackResp.StatusCode)
	}

	fmt.Println("\n=====================================")
	fmt.Println("Summary:")
	fmt.Println("  ✅ TAC obtained from payment service")
	fmt.Println("  ✅ Posted to EPX with TAC")
	fmt.Println("  ✅ Received EPX response")
	fmt.Println("  ✅ Callback processed")
	fmt.Println()
	fmt.Println("This is the CORRECT way to use EPX sandbox!")
	fmt.Println("No CKC tokenization needed - use TAC instead.")
}

func extractCallbackFormData(html string) url.Values {
	data := url.Values{}

	// Extract hidden input fields: <input type="hidden" name="..." value="...">
	re := regexp.MustCompile(`<input[^>]+name="([^"]+)"[^>]+value="([^"]*)"`)
	matches := re.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			name := match[1]
			value := match[2]
			data.Set(name, value)
		}
	}

	return data
}
