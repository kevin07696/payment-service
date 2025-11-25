package testutil

import (
	"context"
	"database/sql"
	"testing"
	"time"

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

// GetDB returns a shared database connection pool for direct SQL operations in tests
// PERFORMANCE: Uses singleton pattern to reuse connections across tests
// DO NOT call Close() on the returned *sql.DB - it's a shared pool
func GetDB(t *testing.T) *sql.DB {
	t.Helper()

	// Use singleton pool instead of creating new connections
	// This prevents connection exhaustion and improves test performance
	return GetDBPool(t)
}

// seedTestMerchant ensures the test merchant exists with correct EPX credentials
func seedTestMerchant(t *testing.T, cfg *Config) {
	t.Helper()

	// Use shared database pool (no need to close)
	db := GetDB(t)

	// Insert or update test merchant with EPX credentials from environment
	// Use constants for merchant ID, slug, and name for consistency across tests
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DBInsertTimeout)*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
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
			$1::uuid,
			$2,
			'epx/staging/mac_secret',
			$3, $4, $5, $6,
			'staging',
			$7,
			true,
			NOW(),
			NOW()
		) ON CONFLICT (id) DO UPDATE SET
			slug = $2,
			mac_secret_path = EXCLUDED.mac_secret_path,
			cust_nbr = EXCLUDED.cust_nbr,
			merch_nbr = EXCLUDED.merch_nbr,
			dba_nbr = EXCLUDED.dba_nbr,
			terminal_nbr = EXCLUDED.terminal_nbr,
			environment = 'staging',
			name = $7,
			updated_at = NOW()
	`, TestMerchantUUID, TestMerchantSlug, cfg.EPXCustNbr, cfg.EPXMerchNbr, cfg.EPXDBANbr, cfg.EPXTerminalNbr, TestMerchantName)

	require.NoError(t, err, "Failed to seed test merchant")

	t.Logf("✅ Test merchant seeded: %s (%s) - EPX: CUST_NBR=%s, MERCH_NBR=%s, DBA_NBR=%s, TERMINAL_NBR=%s",
		TestMerchantSlug, TestMerchantName, cfg.EPXCustNbr, cfg.EPXMerchNbr, cfg.EPXDBANbr, cfg.EPXTerminalNbr)
}

// SeedChargebacks seeds test chargeback data for integration tests
// This function is idempotent - it can be called multiple times safely
// Uses constants from constants.go for merchant IDs, customer IDs, and case numbers
func SeedChargebacks(t *testing.T) {
	t.Helper()

	// Use shared database pool (no need to close)
	db := GetDB(t)

	// Step 1: Get or create a test transaction
	// Add context timeout for database query (10 seconds)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DBQueryTimeout)*time.Second)
	defer cancel()

	var testTxnID string
	err := db.QueryRowContext(ctx, `
		SELECT id FROM transactions
		WHERE merchant_id = $1::uuid
		AND status = 'approved'
		LIMIT 1
	`, TestMerchantUUID).Scan(&testTxnID)

	if err == sql.ErrNoRows {
		// Create a test transaction with context timeout (15 seconds for insert)
		insertCtx, insertCancel := context.WithTimeout(context.Background(), time.Duration(DBInsertTimeout)*time.Second)
		defer insertCancel()

		err = db.QueryRowContext(insertCtx, `
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
		`, TestMerchantUUID).Scan(&testTxnID)
		require.NoError(t, err, "Failed to create test transaction for chargebacks")
		t.Logf("✅ Created test transaction for chargebacks: %s", testTxnID)
	} else {
		require.NoError(t, err, "Failed to query test transaction")
		t.Logf("✅ Using existing test transaction for chargebacks: %s", testTxnID)
	}

	// Step 2: Seed test chargebacks with various statuses
	// Use INSERT ... ON CONFLICT to make it idempotent
	// Use constants for merchant slug, customer ID, and case numbers for consistency
	seedCtx, seedCancel := context.WithTimeout(context.Background(), time.Duration(DBInsertTimeout)*time.Second)
	defer seedCancel()

	var result sql.Result
	result, err = db.ExecContext(seedCtx, `
		INSERT INTO chargebacks (
			transaction_id, merchant_id, customer_id,
			case_number, dispute_date, chargeback_date,
			chargeback_amount, currency, reason_code, reason_description,
			status, raw_data
		) VALUES
		-- NEW chargeback
		(
			$1::uuid, $2, $3,
			$4, NOW() - INTERVAL '5 days', NOW() - INTERVAL '3 days',
			'50.00', 'USD', 'P22', 'Cardholder disputes quality of goods or services',
			'new', '{"status": "NEW", "source": "test_seed", "test": true}'::jsonb
		),
		-- PENDING chargeback
		(
			$1::uuid, $2, $3,
			$5, NOW() - INTERVAL '10 days', NOW() - INTERVAL '7 days',
			'75.50', 'USD', 'F10', 'Fraudulent transaction - card absent environment',
			'pending', '{"status": "PENDING", "source": "test_seed", "test": true}'::jsonb
		),
		-- RESPONDED chargeback
		(
			$1::uuid, $2, $3,
			$6, NOW() - INTERVAL '20 days', NOW() - INTERVAL '15 days',
			'100.00', 'USD', 'C08', 'Goods/Services not received',
			'responded', '{"status": "RESPONDED", "source": "test_seed", "test": true}'::jsonb
		),
		-- WON chargeback
		(
			$1::uuid, $2, $3,
			$7, NOW() - INTERVAL '30 days', NOW() - INTERVAL '25 days',
			'125.00', 'USD', 'P08', 'Credit not processed',
			'won', '{"status": "WON", "source": "test_seed", "test": true}'::jsonb
		),
		-- LOST chargeback
		(
			$1::uuid, $2, $3,
			$8, NOW() - INTERVAL '35 days', NOW() - INTERVAL '30 days',
			'200.00', 'USD', 'F29', 'Card not present fraud',
			'lost', '{"status": "LOST", "source": "test_seed", "test": true}'::jsonb
		)
		ON CONFLICT (case_number) DO NOTHING
	`, testTxnID, TestMerchantSlug, TestCustomerID,
		ChargebackCaseNumberNew,
		ChargebackCaseNumberPending,
		ChargebackCaseNumberResponded,
		ChargebackCaseNumberWon,
		ChargebackCaseNumberLost)

	require.NoError(t, err, "Failed to seed test chargebacks")

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		t.Logf("✅ Seeded %d test chargebacks (statuses: new, pending, responded, won, lost)", rowsAffected)
	} else {
		t.Logf("✅ Test chargebacks already exist (skipped seeding)")
	}
}

// CleanupChargebacks removes all test chargeback data from the database
// Uses t.Cleanup() pattern for automatic cleanup after test execution
// SECURITY NOTE: Only deletes chargebacks marked with raw_data->>'test' = 'true'
func CleanupChargebacks(t *testing.T) {
	t.Helper()

	// Register cleanup function to run after test completion
	t.Cleanup(func() {
		// Use shared database pool (no need to close)
		db := GetDB(t)

		// Use context timeout for cleanup operations
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DBInsertTimeout)*time.Second)
		defer cancel()

		// Delete only test chargebacks (marked with test: true in raw_data)
		// This prevents accidental deletion of real data
		result, err := db.ExecContext(ctx, `
			DELETE FROM chargebacks
			WHERE raw_data->>'test' = 'true'
		`)

		if err != nil {
			t.Logf("⚠️  Warning: Failed to cleanup test chargebacks: %v", err)
			return
		}

		rowsDeleted, _ := result.RowsAffected()
		if rowsDeleted > 0 {
			t.Logf("🧹 Cleaned up %d test chargebacks", rowsDeleted)
		}
	})
}

// CleanupTestTransactions removes test transactions for the test merchant
// Uses t.Cleanup() pattern for automatic cleanup after test execution
// SECURITY NOTE: Only deletes transactions for the test merchant UUID
func CleanupTestTransactions(t *testing.T) {
	t.Helper()

	// Register cleanup function to run after test completion
	t.Cleanup(func() {
		// Use shared database pool (no need to close)
		db := GetDB(t)

		// Use context timeout for cleanup operations
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DBInsertTimeout)*time.Second)
		defer cancel()

		// Delete only test merchant transactions
		// This prevents accidental deletion of real data
		result, err := db.ExecContext(ctx, `
			DELETE FROM transactions
			WHERE merchant_id = $1::uuid
		`, TestMerchantUUID)

		if err != nil {
			t.Logf("⚠️  Warning: Failed to cleanup test transactions: %v", err)
			return
		}

		rowsDeleted, _ := result.RowsAffected()
		if rowsDeleted > 0 {
			t.Logf("🧹 Cleaned up %d test transactions", rowsDeleted)
		}
	})
}
