package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestServiceCredentials represents a test service with RSA keys
type TestServiceCredentials struct {
	ServiceID            string `json:"service_id"`
	ServiceName          string `json:"service_name"`
	Environment          string `json:"environment"`
	PrivateKeyPEM        string `json:"private_key_pem"`
	PublicKeyPEM         string `json:"public_key_pem"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
}

func main() {
	inputPath := flag.String("input", "", "Path to test_services.json")
	dbURL := flag.String("db", "", "Database connection string")
	flag.Parse()

	if *inputPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -input flag is required\n")
		os.Exit(1)
	}

	if *dbURL == "" {
		fmt.Fprintf(os.Stderr, "Error: -db flag is required\n")
		os.Exit(1)
	}

	// Read test services JSON
	jsonData, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		os.Exit(1)
	}

	var services []TestServiceCredentials
	if err := json.Unmarshal(jsonData, &services); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// Connect to database
	db, err := sql.Open("pgx", *dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error pinging database: %v\n", err)
		os.Exit(1)
	}

	// Insert or update services
	for _, svc := range services {
		// Use IIFE to ensure context is canceled after each iteration
		func() {
			insertCtx, insertCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer insertCancel()

			_, err := db.ExecContext(insertCtx, `
				INSERT INTO services (
					service_id, service_name, environment,
					public_key, public_key_fingerprint,
					is_active, created_at, updated_at
				) VALUES (
					$1, $2, $3, $4, $5,
					true, NOW(), NOW()
				) ON CONFLICT (service_id) DO UPDATE SET
					service_name = EXCLUDED.service_name,
					environment = EXCLUDED.environment,
					public_key = EXCLUDED.public_key,
					public_key_fingerprint = EXCLUDED.public_key_fingerprint,
					updated_at = NOW()
			`, svc.ServiceID, svc.ServiceName, svc.Environment, svc.PublicKeyPEM, svc.PublicKeyFingerprint)

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error upserting service %s: %v\n", svc.ServiceID, err)
				os.Exit(1)
			}

			fmt.Printf("✅ Seeded service: %s\n", svc.ServiceID)
		}()
	}

	// Grant test services access to test merchant
	testMerchantID := "00000000-0000-0000-0000-000000000001"

	for _, svc := range services {
		// Use IIFE to ensure context is canceled after each iteration
		func() {
			accessCtx, accessCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer accessCancel()

			// First, get the service's internal ID
			var serviceInternalID string
			err := db.QueryRowContext(accessCtx, `
				SELECT id FROM services WHERE service_id = $1
			`, svc.ServiceID).Scan(&serviceInternalID)

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting service ID for %s: %v\n", svc.ServiceID, err)
				os.Exit(1)
			}

			// Then insert into service_merchants using the internal ID
			_, err = db.ExecContext(accessCtx, `
				INSERT INTO service_merchants (
					service_id, merchant_id
				) VALUES (
					$1::uuid, $2::uuid
				) ON CONFLICT (service_id, merchant_id) DO NOTHING
			`, serviceInternalID, testMerchantID)

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error granting access for service %s: %v\n", svc.ServiceID, err)
				os.Exit(1)
			}

			fmt.Printf("✅ Granted %s access to test merchant\n", svc.ServiceID)
		}()
	}

	fmt.Printf("\n✅ Successfully seeded %d test services\n", len(services))
}
