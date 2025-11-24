package testutil

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

// Setup initializes test environment and returns config and client
func Setup(t *testing.T) (*Config, *Client) {
	t.Helper()

	// Load config from environment
	cfg, err := LoadConfig()
	require.NoError(t, err, "Failed to load test configuration")

	// Seed test merchant with EPX credentials from environment
	seedTestMerchant(t, cfg)

	// Seed test chargebacks for chargeback integration tests
	SeedChargebacks(t)

	// Create API client
	client := NewClient(cfg.ServiceURL)

	t.Logf("Integration test setup complete - service: %s", cfg.ServiceURL)

	return cfg, client
}

// GetDB returns a database connection for direct SQL operations in tests
func GetDB(t *testing.T) *sql.DB {
	t.Helper()

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "payment_service"
	}

	connStr := "host=" + dbHost + " port=" + dbPort + " user=" + dbUser +
		" password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err, "Failed to connect to database")

	// Test connection
	err = db.Ping()
	require.NoError(t, err, "Failed to ping database")

	return db
}

// seedTestMerchant ensures the test merchant exists with correct EPX credentials
func seedTestMerchant(t *testing.T, cfg *Config) {
	t.Helper()

	// Connect to database
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "payment_service"
	}

	connStr := "host=" + dbHost + " port=" + dbPort + " user=" + dbUser +
		" password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Insert or update test merchant with EPX credentials from environment
	_, err = db.Exec(`
		INSERT INTO merchants (
			id,
			slug,
			mac_secret_path,
			cust_nbr,
			merch_nbr,
			dba_nbr,
			terminal_nbr,
			environment,
			name,
			is_active,
			created_at,
			updated_at
		) VALUES (
			'00000000-0000-0000-0000-000000000001'::uuid,
			'test-merchant-staging',
			'epx/staging/mac_secret',
			$1, $2, $3, $4,
			'staging',
			'Test Merchant (Staging)',
			true,
			NOW(),
			NOW()
		) ON CONFLICT (id) DO UPDATE SET
			slug = 'test-merchant-staging',
			mac_secret_path = EXCLUDED.mac_secret_path,
			cust_nbr = EXCLUDED.cust_nbr,
			merch_nbr = EXCLUDED.merch_nbr,
			dba_nbr = EXCLUDED.dba_nbr,
			terminal_nbr = EXCLUDED.terminal_nbr,
			environment = 'staging',
			name = 'Test Merchant (Staging)',
			updated_at = NOW()
	`, cfg.EPXCustNbr, cfg.EPXMerchNbr, cfg.EPXDBANbr, cfg.EPXTerminalNbr)

	require.NoError(t, err, "Failed to seed test merchant")

	t.Logf("✅ Test merchant seeded with EPX credentials: CUST_NBR=%s, MERCH_NBR=%s, DBA_NBR=%s, TERMINAL_NBR=%s",
		cfg.EPXCustNbr, cfg.EPXMerchNbr, cfg.EPXDBANbr, cfg.EPXTerminalNbr)
}

// SeedChargebacks seeds test chargeback data for integration tests
// This function is idempotent - it can be called multiple times safely
func SeedChargebacks(t *testing.T) {
	t.Helper()

	db := GetDB(t)
	defer db.Close()

	testMerchantUUID := "00000000-0000-0000-0000-000000000001"
	testMerchantSlug := "test-merchant-staging" // Merchant slug for chargebacks.merchant_id

	// Step 1: Get or create a test transaction
	var testTxnID string
	err := db.QueryRow(`
		SELECT id FROM transactions
		WHERE merchant_id = $1::uuid
		AND status = 'approved'
		LIMIT 1
	`, testMerchantUUID).Scan(&testTxnID)

	if err == sql.ErrNoRows {
		// Create a test transaction
		err = db.QueryRow(`
			INSERT INTO transactions (
				id, merchant_id, customer_id,
				amount_cents, currency, type, payment_method_type,
				tran_nbr, auth_guid, auth_resp, auth_code,
				processed_at, created_at, updated_at
			) VALUES (
				gen_random_uuid(), $1::uuid, gen_random_uuid(),
				10000, 'USD', 'SALE', 'credit_card',
				'TXN-' || substr(gen_random_uuid()::text, 1, 10),
				'BRIC-' || gen_random_uuid()::text,
				'00', 'AUTH123',
				NOW(), NOW(), NOW()
			) RETURNING id
		`, testMerchantUUID).Scan(&testTxnID)
		require.NoError(t, err, "Failed to create test transaction for chargebacks")
		t.Logf("✅ Created test transaction for chargebacks: %s", testTxnID)
	} else {
		require.NoError(t, err, "Failed to query test transaction")
		t.Logf("✅ Using existing test transaction for chargebacks: %s", testTxnID)
	}

	// Step 2: Seed test chargebacks with various statuses
	// Use INSERT ... ON CONFLICT to make it idempotent
	result, err := db.Exec(`
		INSERT INTO chargebacks (
			transaction_id, merchant_id, customer_id,
			case_number, dispute_date, chargeback_date,
			chargeback_amount, currency, reason_code, reason_description,
			status, raw_data
		) VALUES
		-- NEW chargeback
		(
			$1::uuid, $2, 'cust_test_001',
			'CB-NEW-TEST', NOW() - INTERVAL '5 days', NOW() - INTERVAL '3 days',
			'50.00', 'USD', 'P22', 'Cardholder disputes quality of goods or services',
			'new', '{"status": "NEW", "source": "test_seed", "test": true}'::jsonb
		),
		-- PENDING chargeback
		(
			$1::uuid, $2, 'cust_test_001',
			'CB-PENDING-TEST', NOW() - INTERVAL '10 days', NOW() - INTERVAL '7 days',
			'75.50', 'USD', 'F10', 'Fraudulent transaction - card absent environment',
			'pending', '{"status": "PENDING", "source": "test_seed", "test": true}'::jsonb
		),
		-- RESPONDED chargeback
		(
			$1::uuid, $2, 'cust_test_001',
			'CB-RESPONDED-TEST', NOW() - INTERVAL '20 days', NOW() - INTERVAL '15 days',
			'100.00', 'USD', 'C08', 'Goods/Services not received',
			'responded', '{"status": "RESPONDED", "source": "test_seed", "test": true}'::jsonb
		),
		-- WON chargeback
		(
			$1::uuid, $2, 'cust_test_001',
			'CB-WON-TEST', NOW() - INTERVAL '30 days', NOW() - INTERVAL '25 days',
			'125.00', 'USD', 'P08', 'Credit not processed',
			'won', '{"status": "WON", "source": "test_seed", "test": true}'::jsonb
		),
		-- LOST chargeback
		(
			$1::uuid, $2, 'cust_test_001',
			'CB-LOST-TEST', NOW() - INTERVAL '35 days', NOW() - INTERVAL '30 days',
			'200.00', 'USD', 'F29', 'Card not present fraud',
			'lost', '{"status": "LOST", "source": "test_seed", "test": true}'::jsonb
		)
		ON CONFLICT (case_number) DO NOTHING
	`, testTxnID, testMerchantSlug)

	require.NoError(t, err, "Failed to seed test chargebacks")

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		t.Logf("✅ Seeded %d test chargebacks (statuses: new, pending, responded, won, lost)", rowsAffected)
	} else {
		t.Logf("✅ Test chargebacks already exist (skipped seeding)")
	}
}
