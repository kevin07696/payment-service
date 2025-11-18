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
	// Command line flags
	bricToken := flag.String("bric", "", "BRIC token from EPX (required)")
	amount := flag.String("amount", "10.00", "Amount to charge")
	operation := flag.String("op", "sale", "Operation: sale, auth, list")
	flag.Parse()

	if *bricToken == "" && *operation != "list" {
		fmt.Println("❌ BRIC token is required!")
		fmt.Println("\nUsage:")
		fmt.Println("  go run test_real_bric.go -bric=<TOKEN> [-amount=10.00] [-op=sale]")
		fmt.Println("\nExample:")
		fmt.Println("  go run test_real_bric.go -bric=ABC123DEF456 -amount=25.00")
		fmt.Println("\nTo get a BRIC token:")
		fmt.Println("  1. Open examples/epx_tokenize.html in a browser")
		fmt.Println("  2. Submit the form with a test card")
		fmt.Println("  3. Copy the BRIC from the response")
		os.Exit(1)
	}

	fmt.Println("=====================================")
	fmt.Println("Testing with Real EPX BRIC Token")
	fmt.Println("=====================================")
	fmt.Printf("BRIC: %s\n", *bricToken)
	fmt.Printf("Amount: $%s\n", *amount)
	fmt.Printf("Operation: %s\n\n", *operation)

	// Load WordPress credentials
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

	// Create client
	client := paymentv1connect.NewPaymentServiceClient(
		&http.Client{Timeout: 30 * time.Second},
		"http://localhost:8080",
	)

	// Generate JWT
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(creds.PrivateKey))
	if err != nil {
		fmt.Printf("❌ Failed to parse private key: %v\n", err)
		return
	}

	merchantID := "1a20fff8-2cec-48e5-af49-87e501652913" // ACME Corp

	// Execute operation
	ctx := context.Background()

	switch *operation {
	case "sale":
		processSale(ctx, client, privateKey, creds.ServiceID, merchantID, *bricToken, *amount)
	case "auth":
		processAuth(ctx, client, privateKey, creds.ServiceID, merchantID, *bricToken, *amount)
	case "list":
		listTransactions(ctx, client, privateKey, creds.ServiceID, merchantID)
	default:
		fmt.Printf("❌ Unknown operation: %s\n", *operation)
	}
}

func generateToken(privateKey interface{}, serviceID, merchantID string, scopes []string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":         serviceID,
		"sub":         merchantID,
		"merchant_id": merchantID,
		"iat":         now.Unix(),
		"exp":         now.Add(5 * time.Minute).Unix(),
		"jti":         uuid.New().String(),
		"scopes":      scopes,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

func processSale(ctx context.Context, client paymentv1connect.PaymentServiceClient, privateKey interface{}, serviceID, merchantID, bric, amount string) {
	fmt.Println("Processing SALE transaction...")
	fmt.Println("------------------------------")

	// Generate JWT
	token, err := generateToken(privateKey, serviceID, merchantID, []string{"payment:create"})
	if err != nil {
		fmt.Printf("❌ Failed to generate JWT: %v\n", err)
		return
	}

	// Create request
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:     merchantID,
		Amount:         amount,
		Currency:       "USD",
		PaymentMethod:  &paymentv1.SaleRequest_PaymentToken{PaymentToken: bric},
		IdempotencyKey: uuid.New().String(),
		Metadata: map[string]string{
			"test":   "real_bric",
			"source": "test_real_bric.go",
		},
	})

	req.Header().Set("Authorization", "Bearer "+token)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Execute
	resp, err := client.Sale(ctx, req)
	if err != nil {
		fmt.Printf("❌ Sale failed: %v\n", err)
		return
	}

	// Display results
	fmt.Println("✅ SALE SUCCESSFUL!")
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

	// Save group ID for refund
	fmt.Printf("\n📝 To refund this transaction, use:\n")
	fmt.Printf("   go run test_refund.go -group=%s -amount=%s\n", resp.Msg.GroupId, amount)
}

func processAuth(ctx context.Context, client paymentv1connect.PaymentServiceClient, privateKey interface{}, serviceID, merchantID, bric, amount string) {
	fmt.Println("Processing AUTH transaction...")
	fmt.Println("------------------------------")

	// Generate JWT
	token, err := generateToken(privateKey, serviceID, merchantID, []string{"payment:create"})
	if err != nil {
		fmt.Printf("❌ Failed to generate JWT: %v\n", err)
		return
	}

	// Create request
	req := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:     merchantID,
		Amount:         amount,
		Currency:       "USD",
		PaymentMethod:  &paymentv1.AuthorizeRequest_PaymentToken{PaymentToken: bric},
		IdempotencyKey: uuid.New().String(),
		Metadata: map[string]string{
			"test":   "real_bric_auth",
			"source": "test_real_bric.go",
		},
	})

	req.Header().Set("Authorization", "Bearer "+token)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Execute
	resp, err := client.Authorize(ctx, req)
	if err != nil {
		fmt.Printf("❌ Auth failed: %v\n", err)
		return
	}

	// Display results
	fmt.Println("✅ AUTH SUCCESSFUL!")
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

	fmt.Printf("\n📝 Next steps:\n")
	fmt.Printf("   To capture: go run test_capture.go -tx=%s -amount=%s\n", resp.Msg.TransactionId, amount)
	fmt.Printf("   To void: go run test_void.go -group=%s\n", resp.Msg.GroupId)
}

func listTransactions(ctx context.Context, client paymentv1connect.PaymentServiceClient, privateKey interface{}, serviceID, merchantID string) {
	fmt.Println("Listing recent transactions...")
	fmt.Println("------------------------------")

	// Generate JWT
	token, err := generateToken(privateKey, serviceID, merchantID, []string{"payment:read"})
	if err != nil {
		fmt.Printf("❌ Failed to generate JWT: %v\n", err)
		return
	}

	// Create request
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: merchantID,
		Limit:      10,
	})

	req.Header().Set("Authorization", "Bearer "+token)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Execute
	resp, err := client.ListTransactions(ctx, req)
	if err != nil {
		fmt.Printf("❌ List failed: %v\n", err)
		return
	}

	// Display results
	fmt.Printf("Found %d transactions (showing last %d)\n\n", resp.Msg.TotalCount, len(resp.Msg.Transactions))

	for i, tx := range resp.Msg.Transactions {
		fmt.Printf("%d. Transaction %s\n", i+1, tx.Id[:8])
		fmt.Printf("   Type: %s | Status: %s | Amount: %s %s\n",
			tx.Type, tx.Status, tx.Amount, tx.Currency)
		fmt.Printf("   Group: %s\n", tx.GroupId[:8])
		if tx.Card != nil {
			fmt.Printf("   Card: %s ****%s\n", tx.Card.Brand, tx.Card.LastFour)
		}
		fmt.Printf("   Created: %s\n", tx.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
		fmt.Println()
	}
}