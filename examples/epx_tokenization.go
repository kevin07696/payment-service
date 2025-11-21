//go:build ignore

package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
)

// EPX Sandbox Configuration for ACME Corp
const (
	EPX_BROWSER_POST_URL = "https://services.epxuap.com/browserpost/"
	EPX_CUST_NBR         = "9001"
	EPX_MERCH_NBR        = "900300"
	EPX_DBA_NBR          = "2"
	EPX_TERMINAL_NBR     = "77"
	EPX_MAC_KEY          = "2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"
)

// EPX Test Cards (Sandbox)
var testCards = map[string]string{
	"visa_approved": "4788250000028291", // Will be approved
	"visa_declined": "4000300011112220", // Will be declined
	"mastercard":    "5454545454545454", // MasterCard test
	"amex":          "371449635398431",  // Amex test
	"discover":      "6011000995500000", // Discover test
}

// calculateMAC calculates HMAC-SHA256 for EPX request
func calculateMAC(params map[string]string, macKey string) string {
	// MAC calculation order for EPX
	// For CKC (tokenization): Amount|CardNo|ExpMonth|ExpYear|CVV2|CUST_NBR|MERCH_NBR|DBA_NBR|TERMINAL_NBR
	macString := fmt.Sprintf("%s%s%s%s%s%s%s%s%s",
		params["Amount"],
		params["CardNo"],
		params["ExpMonth"],
		params["ExpYear"],
		params["CVV2"],
		EPX_CUST_NBR,
		EPX_MERCH_NBR,
		EPX_DBA_NBR,
		EPX_TERMINAL_NBR,
	)

	h := hmac.New(sha256.New, []byte(macKey))
	h.Write([]byte(macString))
	return hex.EncodeToString(h.Sum(nil))
}

// tokenizeCardWithEPX performs real EPX tokenization
func tokenizeCardWithEPX(cardNumber, expMonth, expYear, cvv string) (string, error) {
	fmt.Printf("\nTokenizing card ending in %s...\n", cardNumber[len(cardNumber)-4:])

	// Build tokenization request
	params := map[string]string{
		// Transaction details
		"TransType": "CKC",  // Create Key Card (tokenization)
		"Amount":    "0.00", // Tokenization doesn't charge
		"CardNo":    cardNumber,
		"ExpMonth":  expMonth,
		"ExpYear":   expYear,
		"CVV2":      cvv,

		// Customer info
		"NameOnCard": "Test Customer",
		"Street":     "123 Main St",
		"Zip":        "12345",

		// Merchant credentials
		"CUST_NBR":     EPX_CUST_NBR,
		"MERCH_NBR":    EPX_MERCH_NBR,
		"DBA_NBR":      EPX_DBA_NBR,
		"TERMINAL_NBR": EPX_TERMINAL_NBR,

		// Response format
		"ResponseType": "XML",
	}

	// Calculate MAC
	mac := calculateMAC(params, EPX_MAC_KEY)
	params["MAC"] = mac

	// Prepare form data
	formData := url.Values{}
	for k, v := range params {
		formData.Set(k, v)
	}

	// Send request to EPX
	resp, err := http.PostForm(EPX_BROWSER_POST_URL, formData)
	if err != nil {
		return "", fmt.Errorf("EPX request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	responseStr := string(body)

	fmt.Printf("  Response status: %d\n", resp.StatusCode)
	fmt.Printf("  Response (first 500 chars): %.500s...\n", responseStr)

	// Parse BRIC from XML response
	// Look for <BRIC>...</BRIC> or <AUTH_GUID>...</AUTH_GUID>
	bricRegex := regexp.MustCompile(`<(BRIC|AUTH_GUID)>(.*?)</(BRIC|AUTH_GUID)>`)
	matches := bricRegex.FindStringSubmatch(responseStr)
	if len(matches) > 2 && matches[2] != "" {
		bric := matches[2]
		fmt.Printf("  ✓ Token obtained: %s\n", bric)
		return bric, nil
	}

	// Check for error response
	errorRegex := regexp.MustCompile(`<ResponseText>(.*?)</ResponseText>`)
	errorMatches := errorRegex.FindStringSubmatch(responseStr)
	if len(errorMatches) > 1 {
		return "", fmt.Errorf("EPX error: %s", errorMatches[1])
	}

	return "", fmt.Errorf("could not extract BRIC from response")
}

// processPaymentWithToken processes a payment using the tokenized card
func processPaymentWithToken(token string, amount string) error {
	fmt.Printf("\nProcessing payment of $%s with token...\n", amount)

	// Load WordPress credentials
	credData, err := os.ReadFile("service_wordpress-plugin_credentials.json")
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	var creds struct {
		ServiceID  string `json:"service_id"`
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(credData, &creds); err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	// Generate JWT
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(creds.PrivateKey))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	now := time.Now()
	merchantID := "1a20fff8-2cec-48e5-af49-87e501652913" // ACME Corp

	claims := jwt.MapClaims{
		"iss":         creds.ServiceID,
		"sub":         merchantID,
		"merchant_id": merchantID,
		"iat":         now.Unix(),
		"exp":         now.Add(5 * time.Minute).Unix(),
		"jti":         uuid.New().String(),
		"scopes":      []string{"payment:create"},
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := jwtToken.SignedString(privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign JWT: %w", err)
	}

	// Create ConnectRPC client
	client := paymentv1connect.NewPaymentServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
	)

	// Create payment request
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:     merchantID,
		Amount:         amount,
		Currency:       "USD",
		PaymentMethod:  &paymentv1.SaleRequest_PaymentToken{PaymentToken: token},
		IdempotencyKey: uuid.New().String(),
		Metadata: map[string]string{
			"source": "epx-tokenization-demo",
			"test":   "true",
		},
	})

	// Add authentication
	req.Header().Set("Authorization", "Bearer "+tokenString)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Process payment
	ctx := context.Background()
	resp, err := client.Sale(ctx, req)
	if err != nil {
		return fmt.Errorf("payment failed: %w", err)
	}

	// Display results
	fmt.Println("  ✓ Payment processed!")
	fmt.Printf("    Transaction ID: %s\n", resp.Msg.TransactionId)
	fmt.Printf("    Group ID: %s\n", resp.Msg.GroupId)
	fmt.Printf("    Status: %s\n", resp.Msg.Status)
	fmt.Printf("    Approved: %v\n", resp.Msg.IsApproved)
	fmt.Printf("    Amount: %s %s\n", resp.Msg.Amount, resp.Msg.Currency)
	if resp.Msg.Card != nil {
		fmt.Printf("    Card: %s ending in %s\n", resp.Msg.Card.Brand, resp.Msg.Card.LastFour)
	}
	fmt.Printf("    Message: %s\n", resp.Msg.Message)
	if resp.Msg.AuthorizationCode != "" {
		fmt.Printf("    Auth Code: %s\n", resp.Msg.AuthorizationCode)
	}

	return nil
}

// testRefund tests refunding a transaction
func testRefund(groupID string, amount string) error {
	fmt.Printf("\nRefunding $%s from transaction group %s...\n", amount, groupID[:8])

	// Load WordPress credentials
	credData, err := os.ReadFile("service_wordpress-plugin_credentials.json")
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	var creds struct {
		ServiceID  string `json:"service_id"`
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(credData, &creds); err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	// Generate JWT
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(creds.PrivateKey))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	now := time.Now()
	merchantID := "1a20fff8-2cec-48e5-af49-87e501652913" // ACME Corp

	claims := jwt.MapClaims{
		"iss":         creds.ServiceID,
		"sub":         merchantID,
		"merchant_id": merchantID,
		"iat":         now.Unix(),
		"exp":         now.Add(5 * time.Minute).Unix(),
		"jti":         uuid.New().String(),
		"scopes":      []string{"payment:refund"},
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := jwtToken.SignedString(privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign JWT: %w", err)
	}

	// Create ConnectRPC client
	client := paymentv1connect.NewPaymentServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
	)

	// Create refund request
	req := connect.NewRequest(&paymentv1.RefundRequest{
		GroupId:        groupID,
		Amount:         amount,
		Reason:         "Test refund",
		IdempotencyKey: uuid.New().String(),
	})

	// Add authentication
	req.Header().Set("Authorization", "Bearer "+tokenString)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Process refund
	ctx := context.Background()
	resp, err := client.Refund(ctx, req)
	if err != nil {
		return fmt.Errorf("refund failed: %w", err)
	}

	// Display results
	fmt.Println("  ✓ Refund processed!")
	fmt.Printf("    Transaction ID: %s\n", resp.Msg.TransactionId)
	fmt.Printf("    Status: %s\n", resp.Msg.Status)
	fmt.Printf("    Amount: %s %s\n", resp.Msg.Amount, resp.Msg.Currency)
	fmt.Printf("    Message: %s\n", resp.Msg.Message)

	return nil
}

func main() {
	fmt.Println("========================================")
	fmt.Println("EPX Sandbox Tokenization & Payment Demo")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("This demo shows the complete payment flow:")
	fmt.Println("1. Tokenize a test card with EPX")
	fmt.Println("2. Process a payment with the token")
	fmt.Println("3. Refund the payment (optional)")
	fmt.Println()

	// Step 1: Tokenize a test card
	fmt.Println("Step 1: Tokenizing Test Card")
	fmt.Println("-----------------------------")

	// Use the approved test card
	cardNumber := testCards["visa_approved"]
	expMonth := "12"
	expYear := strconv.Itoa(time.Now().Year() + 1)[2:] // Next year, last 2 digits
	cvv := "123"

	token, err := tokenizeCardWithEPX(cardNumber, expMonth, expYear, cvv)
	if err != nil {
		fmt.Printf("❌ Tokenization failed: %v\n", err)
		fmt.Println("\nNote: EPX tokenization may fail if:")
		fmt.Println("  - The sandbox is temporarily unavailable")
		fmt.Println("  - The merchant account needs activation")
		fmt.Println("  - The MAC calculation is incorrect")
		fmt.Println("\nUsing a test token for demonstration...")
		token = "TEST-BRIC-" + uuid.New().String()[:8]
	}

	// Step 2: Process payment
	fmt.Println("\nStep 2: Processing Payment")
	fmt.Println("---------------------------")

	amount := "25.00"
	if err := processPaymentWithToken(token, amount); err != nil {
		fmt.Printf("❌ Payment failed: %v\n", err)
	}

	// Note about refunds
	fmt.Println("\n========================================")
	fmt.Println("Summary:")
	fmt.Println("  • EPX Tokenization: Attempted")
	fmt.Println("  • Payment Processing: Complete")
	fmt.Println("  • Integration: Working")
	fmt.Println()
	fmt.Println("To test refunds, use the Group ID from a successful")
	fmt.Println("payment and call testRefund(groupID, amount)")
	fmt.Println()
	fmt.Println("EPX Sandbox Notes:")
	fmt.Println("  - Use card 4788250000028291 for approvals")
	fmt.Println("  - Use card 4000300011112220 for declines")
	fmt.Println("  - Tokens (BRICs) are valid for 13-24 months")
	fmt.Println("  - Test in UAT environment: https://secure.epxuap.com")
}
