//go:build ignore

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
)

func main() {
	ctx := context.Background()

	// Connect to database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	// Generate RSA keypair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate RSA key: %v\n", err)
		os.Exit(1)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal public key: %v\n", err)
		os.Exit(1)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Create merchant
	merchantID := uuid.New()
	merchant, err := queries.CreateMerchant(ctx, sqlc.CreateMerchantParams{
		ID:            merchantID,
		Slug:          "e2e-test-merchant",
		Name:          "E2E Test Merchant",
		CustNbr:       os.Getenv("EPX_CUST_NBR"),
		MerchNbr:      os.Getenv("EPX_MERCH_NBR"),
		DbaNbr:        os.Getenv("EPX_DBA_NBR"),
		TerminalNbr:   os.Getenv("EPX_TERMINAL_NBR"),
		MacSecretPath: "epx/staging/mac_secret",
		Environment:   "staging",
		IsActive:      true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create merchant: %v\n", err)
		os.Exit(1)
	}

	// Create service
	serviceID := uuid.New()
	serviceIdentifier := "e2e-test-service"
	_, err = queries.CreateService(ctx, sqlc.CreateServiceParams{
		ID:        serviceID,
		ServiceID: serviceIdentifier,
		Name:      "E2E Test Service",
		PublicKey: string(publicKeyPEM),
		IsActive:  true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create service: %v\n", err)
		os.Exit(1)
	}

	// Grant access
	err = queries.GrantServiceAccess(ctx, sqlc.GrantServiceAccessParams{
		ServiceID:  serviceID,
		MerchantID: merchantID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to grant access: %v\n", err)
		os.Exit(1)
	}

	// Generate JWT token
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":         serviceIdentifier,
		"sub":         serviceIdentifier,
		"iat":         now.Unix(),
		"nbf":         now.Unix(),
		"exp":         now.Add(24 * time.Hour).Unix(),
		"jti":         uuid.New().String(),
		"merchant_id": merchant.ID.String(),
		"scopes": []string{
			"payments:create",
			"payments:read",
			"payments:void",
			"payments:refund",
			"payment_methods:read",
			"payment_methods:create",
			"subscriptions:manage",
			"subscriptions:read",
		},
	})

	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sign JWT: %v\n", err)
		os.Exit(1)
	}

	// Output
	fmt.Println("=== E2E Test Data Created ===")
	fmt.Println()
	fmt.Printf("Merchant ID: %s\n", merchant.ID)
	fmt.Printf("Service ID:  %s\n", serviceIdentifier)
	fmt.Println()
	fmt.Println("Set these environment variables:")
	fmt.Println()
	fmt.Printf("export TEST_MERCHANT_ID='%s'\n", merchant.ID)
	fmt.Printf("export TEST_JWT_TOKEN='%s'\n", tokenString)
	fmt.Println()
	fmt.Println("Private key (save to file if needed):")
	fmt.Println(string(privateKeyPEM))
}
