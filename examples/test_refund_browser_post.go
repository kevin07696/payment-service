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

func main() {
	fmt.Println("=====================================")
	fmt.Println("Testing REFUND with Browser Post BRIC")
	fmt.Println("=====================================")
	fmt.Println()

	// The approved Browser Post transaction
	groupID := "874a2cf3-651c-4e0d-8207-20015feab5eb" // From first successful Browser Post
	bric := "0A1MR7P3KB67B3WM80E"
	amount := "10.00" // Partial refund

	fmt.Printf("Original Transaction Group: %s\n", groupID[:8])
	fmt.Printf("BRIC: %s\n", bric)
	fmt.Printf("Refund Amount: $%s\n\n", amount)

	// Load WordPress credentials
	credData, err := os.ReadFile("service_wordpress-plugin_credentials.json")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	var creds struct {
		ServiceID  string `json:"service_id"`
		PrivateKey string `json:"private_key"`
	}
	json.Unmarshal(credData, &creds)

	// Parse private key
	privateKey, _ := jwt.ParseRSAPrivateKeyFromPEM([]byte(creds.PrivateKey))

	// Test merchant ID
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
		"scopes":      []string{"payment:refund"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)

	// Create client
	client := paymentv1connect.NewPaymentServiceClient(
		&http.Client{Timeout: 30 * time.Second},
		"http://localhost:8080",
	)

	// Create refund request
	req := connect.NewRequest(&paymentv1.RefundRequest{
		GroupId:        groupID,
		Amount:         amount,
		Reason:         "Test refund with real Browser Post BRIC",
		IdempotencyKey: uuid.New().String(),
	})

	req.Header().Set("Authorization", "Bearer "+tokenString)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Execute refund
	ctx := context.Background()
	resp, err := client.Refund(ctx, req)
	if err != nil {
		fmt.Printf("❌ Refund failed: %v\n", err)
		return
	}

	// Display results
	if resp.Msg.IsApproved {
		fmt.Println("✅ REFUND APPROVED!")
	} else {
		fmt.Println("⚠️  REFUND DECLINED")
	}

	fmt.Printf("  Transaction ID: %s\n", resp.Msg.TransactionId)
	fmt.Printf("  Group ID: %s\n", resp.Msg.GroupId)
	fmt.Printf("  Status: %s\n", resp.Msg.Status)
	fmt.Printf("  Approved: %v\n", resp.Msg.IsApproved)
	fmt.Printf("  Amount: %s %s\n", resp.Msg.Amount, resp.Msg.Currency)
	fmt.Printf("  Message: %s\n", resp.Msg.Message)

	fmt.Println("\n=====================================")
	fmt.Println("This demonstrates that BRICs from Browser Post")
	fmt.Println("work correctly for follow-up operations (REFUND,")
	fmt.Println("VOID, CAPTURE) on the original transaction!")
}
