package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kevin07696/payment-service/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Get database URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"
	}

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	// Generate admin credentials
	adminEmail := "admin@payment-service.local"
	adminPassword := generateSecurePassword()
	adminPasswordHash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), 10)
	if err != nil {
		log.Fatal("Failed to hash admin password:", err)
	}

	adminID := uuid.New().String()

	// Insert or get admin account
	err = sqlDB.QueryRow(`
		INSERT INTO admins (id, email, password_hash, role, is_active)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			updated_at = NOW()
		RETURNING id
	`, adminID, adminEmail, string(adminPasswordHash), "super_admin", true).Scan(&adminID)

	if err != nil {
		log.Fatal("Failed to create admin account:", err)
	}

	// Generate RSA keypair for test service
	privateKey, publicKey, err := auth.GenerateRSAKeyPair(2048)
	if err != nil {
		log.Fatal("Failed to generate RSA keypair:", err)
	}

	publicKeyPEM, err := auth.PublicKeyToPEM(publicKey)
	if err != nil {
		log.Fatal("Failed to convert public key to PEM:", err)
	}

	serviceID := uuid.New().String()

	// Insert test service
	_, err = sqlDB.Exec(`
		INSERT INTO services (
			id, service_id, service_name, public_key,
			public_key_fingerprint, environment,
			requests_per_second, burst_limit, is_active, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (service_id) DO UPDATE SET
			public_key = EXCLUDED.public_key,
			updated_at = NOW()
	`, serviceID, "test-pos-system", "Test POS System (Development)",
		string(publicKeyPEM), generateFingerprint(publicKeyPEM), "staging",
		1000, 2000, true, adminID)

	if err != nil {
		log.Fatal("Failed to create test service:", err)
	}

	// Create test merchant
	merchantID := uuid.New().String()

	// Check if merchant already exists
	var existingMerchantID string
	err = sqlDB.QueryRow(`SELECT id FROM merchants WHERE slug = 'test-merchant-dev'`).Scan(&existingMerchantID)
	if err == nil {
		merchantID = existingMerchantID
		fmt.Println("Using existing test merchant:", merchantID)
	} else {
		// Create new merchant
		err = sqlDB.QueryRow(`
			INSERT INTO merchants (
				id, slug, name, cust_nbr, merch_nbr, dba_nbr,
				terminal_nbr, mac_secret_path, environment,
				is_active, status, tier, requests_per_second,
				burst_limit, created_by
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			RETURNING id
		`, merchantID, "test-merchant-dev", "Test Merchant (Development)",
			"9001", "900300", "2", "77", "/secrets/test-merchant", "staging",
			true, "active", "standard", 100, 200, adminID).Scan(&merchantID)

		if err != nil {
			log.Fatal("Failed to create test merchant:", err)
		}
	}

	// Grant service access to merchant (use the services.id, not service_id)
	var registeredServiceID string
	err = sqlDB.QueryRow(`
		SELECT id FROM services WHERE service_id = 'test-pos-system'
	`).Scan(&registeredServiceID)

	if err != nil {
		log.Fatal("Failed to find registered service:", err)
	}

	_, err = sqlDB.Exec(`
		INSERT INTO service_merchants (
			service_id, merchant_id, scopes, granted_by
		) VALUES ($1, $2, $3, $4)
		ON CONFLICT (service_id, merchant_id) DO UPDATE SET
			scopes = EXCLUDED.scopes,
			granted_at = NOW()
	`, registeredServiceID, merchantID,
		"{payment:create,payment:read,payment:update,payment:refund,subscription:manage,payment_method:manage}",
		adminID)

	if err != nil {
		log.Fatal("Failed to grant service access:", err)
	}

	// NOTE: API key/secret authentication has been removed.
	// Services authenticate using RSA keypairs (JWT tokens).
	// The private key was output when creating the service above.

	// Add EPX IPs for development
	epxIPs := []struct {
		ip   string
		desc string
	}{
		{"10.0.0.1", "Development EPX Gateway 1"},
		{"10.0.0.2", "Development EPX Gateway 2"},
		{"192.168.1.100", "Test EPX Gateway"},
	}

	for _, epx := range epxIPs {
		_, err = sqlDB.Exec(`
			INSERT INTO epx_ip_whitelist (ip_address, description, added_by)
			VALUES ($1, $2, $3)
			ON CONFLICT (ip_address) DO NOTHING
		`, epx.ip, epx.desc, adminID)
		if err != nil {
			log.Printf("Warning: Failed to add EPX IP %s: %v\n", epx.ip, err)
		}
	}

	// Save credentials to file for reference
	credentialsFile := "seed_credentials.txt"
	file, err := os.Create(credentialsFile)
	if err != nil {
		log.Printf("Warning: Could not create credentials file: %v\n", err)
	} else {
		defer file.Close()
		fmt.Fprintf(file, "===========================================\n")
		fmt.Fprintf(file, "SEED CREDENTIALS - GENERATED AUTOMATICALLY\n")
		fmt.Fprintf(file, "===========================================\n\n")
		fmt.Fprintf(file, "Admin Account:\n")
		fmt.Fprintf(file, "  Email: %s\n", adminEmail)
		fmt.Fprintf(file, "  Password: %s\n", adminPassword)
		fmt.Fprintf(file, "  ID: %s\n\n", adminID)
		fmt.Fprintf(file, "Test Merchant:\n")
		fmt.Fprintf(file, "  ID: %s\n", merchantID)
		fmt.Fprintf(file, "  Slug: test-merchant-dev\n\n")
		fmt.Fprintf(file, "Test Service:\n")
		fmt.Fprintf(file, "  ID: %s\n", serviceID)
		fmt.Fprintf(file, "  Service ID: test-pos-system\n")
		fmt.Fprintf(file, "  Private Key (for JWT signing):\n%s\n", string(auth.PrivateKeyToPEM(privateKey)))
		fmt.Fprintf(file, "===========================================\n")
	}

	// Print to console
	fmt.Println("========================================")
	fmt.Println("SEED DATA CREATED SUCCESSFULLY")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Admin Account:")
	fmt.Printf("  Email: %s\n", adminEmail)
	fmt.Printf("  Password: %s\n", adminPassword)
	fmt.Printf("  ⚠️  SAVE THIS PASSWORD - IT CANNOT BE RECOVERED!\n")
	fmt.Println()
	fmt.Println("Test Merchant:")
	fmt.Printf("  Merchant ID: %s\n", merchantID)
	fmt.Printf("  Slug: test-merchant-dev\n")
	fmt.Println()
	fmt.Println("Service Authentication:")
	fmt.Println("  Services use RSA keypairs for JWT-based authentication")
	fmt.Println("  The private key for 'test-pos-system' is saved above")
	fmt.Println("Test Service:")
	fmt.Printf("  Service ID: test-pos-system\n")
	fmt.Printf("  Has access to test merchant\n")
	fmt.Println()
	fmt.Printf("Credentials saved to: %s\n", credentialsFile)
	fmt.Println("========================================")
}

func generateSecurePassword() string {
	// Generate a secure random password
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, 16)
	rand.Read(b)
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

func generateFingerprint(publicKeyPEM []byte) string {
	// Simple fingerprint generation
	return fmt.Sprintf("SHA256:%s", base64.StdEncoding.EncodeToString(publicKeyPEM)[:40])
}
