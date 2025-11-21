//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"flag"
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

func main() {
	bric := flag.String("bric", "0A1MR7P3KB67B3WM80E", "BRIC token from successful Browser Post")
	amount := flag.String("amount", "10.00", "Amount to charge")
	flag.Parse()

	fmt.Println("=====================================")
	fmt.Println("Testing Real BRIC with Test Merchant")
	fmt.Println("=====================================")
	fmt.Printf("BRIC: %s\n", *bric)
	fmt.Printf("Amount: $%s\n", *amount)
	fmt.Printf("Merchant: Test Merchant (Browser Post generated)\n\n")

	// Load WordPress credentials (for JWT)
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

	// Use TEST merchant ID (same one that generated the BRIC)
	merchantID := "00000000-0000-0000-0000-000000000001"

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
		fmt.Printf("❌ Failed to sign JWT: %v\n", err)
		return
	}

	// Create client
	client := paymentv1connect.NewPaymentServiceClient(
		&http.Client{Timeout: 30 * time.Second},
		"http://localhost:8080",
	)

	// Create payment request
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:     merchantID,
		Amount:         *amount,
		Currency:       "USD",
		PaymentMethod:  &paymentv1.SaleRequest_PaymentToken{PaymentToken: *bric},
		IdempotencyKey: uuid.New().String(),
		Metadata: map[string]string{
			"test":   "real_bric_test_merchant",
			"source": "browser_post_generated",
		},
	})

	req.Header().Set("Authorization", "Bearer "+tokenString)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Execute
	ctx := context.Background()
	resp, err := client.Sale(ctx, req)
	if err != nil {
		fmt.Printf("❌ Sale failed: %v\n", err)
		return
	}

	// Display results
	if resp.Msg.IsApproved {
		fmt.Println("✅ SALE APPROVED!")
	} else {
		fmt.Println("⚠️  SALE DECLINED")
	}

	fmt.Printf("  Transaction ID: %s\n", resp.Msg.TransactionId)
	fmt.Printf("  Group ID: %s\n", resp.Msg.GroupId)
	fmt.Printf("  Status: %s\n", resp.Msg.Status)
	fmt.Printf("  Approved: %v\n", resp.Msg.IsApproved)
	fmt.Printf("  Amount: %s %s\n", resp.Msg.Amount, resp.Msg.Currency)
	if resp.Msg.Card != nil {
		fmt.Printf("  Card: %s ending in %s\n", resp.Msg.Card.Brand, resp.Msg.Card.LastFour)
	}
	fmt.Printf("  Message: %s\n", resp.Msg.Message)
	if resp.Msg.AuthorizationCode != "" {
		fmt.Printf("  Auth Code: %s\n", resp.Msg.AuthorizationCode)
	}

	fmt.Println("\n=====================================")
	fmt.Println("Note: BRICs from Browser Post are tied to")
	fmt.Println("the merchant that generated them.")
	fmt.Println("To use with ACME merchant, generate a new")
	fmt.Println("BRIC specifically for ACME.")
}
