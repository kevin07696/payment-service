package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
)

// TestService represents a test service from JSON
type TestService struct {
	ServiceID            string `json:"service_id"`
	ServiceName          string `json:"service_name"`
	Environment          string `json:"environment"`
	PrivateKeyPEM        string `json:"private_key_pem"`
	PublicKeyPEM         string `json:"public_key_pem"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
}

func main() {
	// Get database URL from environment or use default
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"
	}

	// Connect to database
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ping database: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Connected to database")

	// Read test services from JSON
	// Path is relative to script directory (scripts/seed_test_services/)
	// Go up 2 levels to project root, then into tests/fixtures/auth/
	jsonPath := "../../tests/fixtures/auth/test_services.json"
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read %s: %v\n", jsonPath, err)
		os.Exit(1)
	}

	var testServices []TestService
	if err := json.Unmarshal(jsonData, &testServices); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Loaded %d test services from JSON\n\n", len(testServices))

	ctx := context.Background()

	// Ensure test merchant exists
	testMerchantID := ensureTestMerchant(ctx, db)
	fmt.Printf("✓ Test merchant ready: %s\n\n", testMerchantID)

	// Insert each test service
	for _, svc := range testServices {
		fmt.Printf("Seeding %s...\n", svc.ServiceName)

		// Check if service already exists
		var existingID uuid.UUID
		err := db.QueryRowContext(ctx, `
			SELECT id FROM services WHERE service_id = $1
		`, svc.ServiceID).Scan(&existingID)

		if err == sql.ErrNoRows {
			// Insert new service
			id := uuid.New()
			_, err = db.ExecContext(ctx, `
				INSERT INTO services (
					id, service_id, service_name, public_key,
					public_key_fingerprint, environment, is_active,
					requests_per_second, created_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
			`, id, svc.ServiceID, svc.ServiceName, svc.PublicKeyPEM,
				svc.PublicKeyFingerprint, svc.Environment, true, 100)

			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed to insert service: %v\n", err)
				continue
			}

			fmt.Printf("  ✓ Inserted service (ID: %s)\n", id)

			// Link service to test merchant
			if err := linkServiceToMerchant(ctx, db, id, testMerchantID); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed to link to merchant: %v\n", err)
				continue
			}

			fmt.Printf("  ✓ Linked to test merchant\n")
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Database error: %v\n", err)
			continue
		} else {
			fmt.Printf("  ⊙ Service already exists (ID: %s)\n", existingID)

			// Ensure it's linked to test merchant
			if err := linkServiceToMerchant(ctx, db, existingID, testMerchantID); err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed to link to merchant: %v\n", err)
			} else {
				fmt.Printf("  ✓ Ensured merchant link\n")
			}
		}

		fmt.Println()
	}

	fmt.Println("✅ Database seeding complete!")
	fmt.Println("\nTest Services:")
	for _, svc := range testServices {
		fmt.Printf("  - %s (%s)\n", svc.ServiceName, svc.ServiceID)
		fmt.Printf("    Fingerprint: %s\n", svc.PublicKeyFingerprint)
	}
}

func ensureTestMerchant(ctx context.Context, db *sql.DB) uuid.UUID {
	// Use a fixed UUID for test merchant
	testMerchantID := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	// Check if test merchant exists
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM merchants WHERE id = $1)
	`, testMerchantID).Scan(&exists)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check merchant existence: %v\n", err)
		os.Exit(1)
	}

	if !exists {
		// Insert test merchant
		_, err = db.ExecContext(ctx, `
			INSERT INTO merchants (
				id, slug, cust_nbr, merch_nbr, dba_nbr, terminal_nbr,
				mac_secret_path, environment, is_active, name,
				status, tier, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
		`, testMerchantID, "test-merchant-001", "000000", "000000",
			"00", "00000001", "secrets/test/mac", "test", true,
			"Test Merchant", "active", "standard")

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to insert test merchant: %v\n", err)
			os.Exit(1)
		}
	}

	return testMerchantID
}

func linkServiceToMerchant(ctx context.Context, db *sql.DB, serviceID, merchantID uuid.UUID) error {
	// Check if link already exists
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM service_merchants
			WHERE service_id = $1 AND merchant_id = $2
		)
	`, serviceID, merchantID).Scan(&exists)

	if err != nil {
		return err
	}

	if exists {
		return nil // Already linked
	}

	// Create link with all necessary scopes
	scopes := []string{
		"payment:create",
		"payment:read",
		"payment:refund",
		"payment:void",
		"payment_method:create",
		"payment_method:read",
		"transaction:read",
	}

	// Insert with proper PostgreSQL array type handling
	_, err = db.ExecContext(ctx, `
		INSERT INTO service_merchants (service_id, merchant_id, scopes, granted_at)
		VALUES ($1, $2, $3, NOW())
	`, serviceID, merchantID, pq.Array(scopes))

	return err
}
