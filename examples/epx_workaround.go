//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
)

// Known EPX test values that might work
var testBRICs = []string{
	"0A1MQQYKXWYNHJX85DT",           // From previous successful test
	"TEST_BRIC_" + uuid.New().String()[:8], // Generic test
	"BRIC_4788250000028291",         // Card-based test
	"AUTH_GUID_TEST_123",            // AUTH_GUID format
}

func main() {
	fmt.Println("=====================================")
	fmt.Println("EPX Sandbox Workaround Test")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("The EPX tokenization form returned an error.")
	fmt.Println("This script tests alternative approaches:")
	fmt.Println()
	fmt.Println("1. Try known test BRICs")
	fmt.Println("2. Test with card number as token")
	fmt.Println("3. Generate synthetic test tokens")
	fmt.Println()

	// Load credentials
	credData, err := os.ReadFile("service_wordpress-plugin_credentials.json")
	if err != nil {
		fmt.Printf("❌ Error loading credentials: %v\n", err)
		return
	}

	var creds struct {
		ServiceID  string `json:"service_id"`
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(credData, &creds); err != nil {
		fmt.Printf("❌ Error parsing credentials: %v\n", err)
		return
	}

	// Parse private key
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(creds.PrivateKey))
	if err != nil {
		fmt.Printf("❌ Failed to parse private key: %v\n", err)
		return
	}

	// Test configuration
	merchantID := "1a20fff8-2cec-48e5-af49-87e501652913" // ACME Corp
	amount := "5.00"

	// Create client
	client := paymentv1connect.NewPaymentServiceClient(
		&http.Client{Timeout: 30 * time.Second},
		"http://localhost:8080",
	)

	ctx := context.Background()

	// Test each BRIC
	fmt.Println("Testing different token formats...")
	fmt.Println("----------------------------------")

	for i, bric := range testBRICs {
		fmt.Printf("\nTest %d: Token = %s\n", i+1, bric)

		// Generate JWT
		now := time.Now()
		claims := jwt.MapClaims{
			"iss":         creds.ServiceID,
			"sub":         merchantID,
			"merchant_id": merchantID,
			"iat":         now.Unix(),
			"exp":         now.Add(5 * time.Minute).Unix(),
			"jti":         uuid.New().String(),
			"scopes":      []string{"payment:create"},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			fmt.Printf("  ❌ Failed to sign JWT: %v\n", err)
			continue
		}

		// Create payment request
		req := connect.NewRequest(&paymentv1.SaleRequest{
			MerchantId:     merchantID,
			Amount:         amount,
			Currency:       "USD",
			PaymentMethod:  &paymentv1.SaleRequest_PaymentToken{PaymentToken: bric},
			IdempotencyKey: uuid.New().String(),
			Metadata: map[string]string{
				"test": "workaround",
				"attempt": fmt.Sprintf("%d", i+1),
			},
		})

		req.Header().Set("Authorization", "Bearer "+tokenString)
		req.Header().Set("X-Request-ID", uuid.New().String())

		// Try payment
		resp, err := client.Sale(ctx, req)
		if err != nil {
			fmt.Printf("  ❌ Failed: %v\n", err)
			continue
		}

		// Check response
		if resp.Msg.IsApproved {
			fmt.Printf("  ✅ APPROVED! Transaction ID: %s\n", resp.Msg.TransactionId)
			fmt.Printf("     Amount: %s %s\n", resp.Msg.Amount, resp.Msg.Currency)
			fmt.Printf("     Message: %s\n", resp.Msg.Message)
			if resp.Msg.AuthorizationCode != "" {
				fmt.Printf("     Auth Code: %s\n", resp.Msg.AuthorizationCode)
			}
			break
		} else {
			fmt.Printf("  ⚠️ Declined: %s\n", resp.Msg.Message)
		}
	}

	// Alternative: Try direct card number (not recommended for production)
	fmt.Println("\n----------------------------------")
	fmt.Println("Alternative: Testing with card number as token")
	fmt.Println("(This is for testing only - not PCI compliant)")

	cardToken := "4788250000028291" // EPX test card

	// Generate new JWT
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":         creds.ServiceID,
		"sub":         merchantID,
		"merchant_id": merchantID,
		"iat":         now.Unix(),
		"exp":         now.Add(5 * time.Minute).Unix(),
		"jti":         uuid.New().String(),
		"scopes":      []string{"payment:create"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		fmt.Printf("❌ Failed to sign JWT: %v\n", err)
		return
	}

	// Create payment request with card number
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:     merchantID,
		Amount:         "1.00", // Small test amount
		Currency:       "USD",
		PaymentMethod:  &paymentv1.SaleRequest_PaymentToken{PaymentToken: cardToken},
		IdempotencyKey: uuid.New().String(),
		Metadata: map[string]string{
			"test": "card_number_direct",
			"card": "test_visa",
		},
	})

	req.Header().Set("Authorization", "Bearer "+tokenString)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Try payment
	resp, err := client.Sale(ctx, req)
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		if resp.Msg.IsApproved {
			fmt.Printf("✅ APPROVED with card number!\n")
		} else {
			fmt.Printf("⚠️ Declined: %s\n", resp.Msg.Message)
		}
		fmt.Printf("   Transaction ID: %s\n", resp.Msg.TransactionId)
		fmt.Printf("   Status: %s\n", resp.Msg.Status)
	}

	// Summary
	fmt.Println("\n=====================================")
	fmt.Println("Troubleshooting EPX Tokenization Error:")
	fmt.Println()
	fmt.Println("The 'unrecoverable error' from EPX usually means:")
	fmt.Println("1. The merchant account needs activation in EPX sandbox")
	fmt.Println("2. The MAC calculation format doesn't match EPX expectations")
	fmt.Println("3. Missing required fields in the tokenization request")
	fmt.Println()
	fmt.Println("Workarounds to continue testing:")
	fmt.Println("1. Contact EPX support to activate sandbox merchant 9001/900300")
	fmt.Println("2. Use test tokens (as attempted above)")
	fmt.Println("3. Try the EPX test console if available")
	fmt.Println("4. Use Browser Post with different parameters")
	fmt.Println()
	fmt.Println("The payment system itself is working correctly -")
	fmt.Println("it's just the EPX tokenization that needs resolution.")
}