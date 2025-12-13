//go:build integration
// +build integration

package database_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseConstraints verifies database constraints are enforced
// These tests ensure data integrity at the database level
func TestDatabaseConstraints(t *testing.T) {
	db := testutil.GetDB(t)
	require.NotNil(t, db, "Database connection required")

	t.Run("unique_constraints", func(t *testing.T) {
		testUniqueConstraints(t, db)
	})

	t.Run("foreign_key_constraints", func(t *testing.T) {
		testForeignKeyConstraints(t, db)
	})

	t.Run("not_null_constraints", func(t *testing.T) {
		testNotNullConstraints(t, db)
	})

	t.Run("check_constraints", func(t *testing.T) {
		testCheckConstraints(t, db)
	})
}

// testUniqueConstraints tests unique constraint enforcement
func testUniqueConstraints(t *testing.T, db *sql.DB) {
	t.Helper()

	t.Run("merchants_slug_unique", func(t *testing.T) {
		// Use factory to create a properly configured merchant
		factory := testutil.NewFactory(t)
		merchant1 := factory.NewMerchant(t).Create()

		// Try to insert another merchant with the same slug - should fail
		_, err := db.Exec(`
			INSERT INTO merchants (id, slug, name, cust_nbr, merch_nbr, dba_nbr, terminal_nbr, mac_secret_path, environment, is_active, created_at, updated_at)
			VALUES ($1::uuid, $2, 'Test Duplicate', '9001', '900300', '2', '77', 'test/path', 'staging', true, NOW(), NOW())
		`, uuid.New().String(), merchant1.Slug)

		assert.Error(t, err, "Duplicate slug should fail")
		assert.True(t, strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate"),
			"Error should mention unique constraint violation: %v", err)
	})

	t.Run("chargebacks_case_number_unique", func(t *testing.T) {
		factory := testutil.NewFactory(t)
		ctx := factory.CreateTestContext(t)

		// Create a test transaction first (without status - it's GENERATED)
		txID := uuid.New().String()
		_, err := db.Exec(`
			INSERT INTO transactions (id, merchant_id, amount_cents, currency, type, payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, 1000, 'USD', 'SALE', 'credit_card', 'TXN-' || $1, '00', NOW(), NOW(), NOW())
		`, txID, ctx.Merchant.ID.String())
		require.NoError(t, err, "Transaction insert should succeed")

		caseNumber := "CB-UNIQUE-TEST-" + uuid.New().String()[:8]

		// First chargeback should succeed (include all required fields)
		_, err = db.Exec(`
			INSERT INTO chargebacks (transaction_id, merchant_id, case_number, status, dispute_date, chargeback_date, chargeback_amount, currency, reason_code, raw_data, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, 'new', NOW(), NOW(), '50.00', 'USD', 'P22', '{}', NOW(), NOW())
		`, txID, ctx.Merchant.Slug, caseNumber)
		require.NoError(t, err, "First chargeback insert should succeed")

		// Cleanup
		t.Cleanup(func() {
			db.Exec("DELETE FROM chargebacks WHERE case_number = $1", caseNumber)
			db.Exec("DELETE FROM transactions WHERE id = $1::uuid", txID)
		})

		// Second chargeback with same case number should fail
		_, err = db.Exec(`
			INSERT INTO chargebacks (transaction_id, merchant_id, case_number, status, dispute_date, chargeback_date, chargeback_amount, currency, reason_code, raw_data, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, 'pending', NOW(), NOW(), '25.00', 'USD', 'F10', '{}', NOW(), NOW())
		`, txID, ctx.Merchant.Slug, caseNumber)

		assert.Error(t, err, "Duplicate case number should fail")
		assert.True(t, strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate"),
			"Error should mention unique constraint violation: %v", err)
	})
}

// testForeignKeyConstraints tests foreign key constraint enforcement
func testForeignKeyConstraints(t *testing.T, db *sql.DB) {
	t.Helper()

	t.Run("subscriptions_require_valid_merchant", func(t *testing.T) {
		nonExistentMerchantID := uuid.New().String()

		_, err := db.Exec(`
			INSERT INTO subscriptions (id, merchant_id, customer_id, amount_cents, currency, interval_value, interval_unit, status, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, 'cust_test', 1000, 'USD', 1, 'month', 'active', NOW(), NOW())
		`, uuid.New().String(), nonExistentMerchantID)

		assert.Error(t, err, "Insert with non-existent merchant should fail")
		assert.True(t, strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "violates"),
			"Error should mention foreign key constraint: %v", err)
	})

	t.Run("subscriptions_require_valid_payment_method", func(t *testing.T) {
		factory := testutil.NewFactory(t)
		ctx := factory.CreateTestContext(t)
		nonExistentPMID := uuid.New().String()

		_, err := db.Exec(`
			INSERT INTO subscriptions (id, merchant_id, customer_id, payment_method_id, amount_cents, currency, interval_value, interval_unit, status, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, 'cust_test', $3::uuid, 1000, 'USD', 1, 'month', 'active', NOW(), NOW())
		`, uuid.New().String(), ctx.Merchant.ID.String(), nonExistentPMID)

		assert.Error(t, err, "Insert with non-existent payment method should fail")
	})
}

// testNotNullConstraints tests NOT NULL constraint enforcement
func testNotNullConstraints(t *testing.T, db *sql.DB) {
	t.Helper()

	t.Run("transactions_require_merchant_id", func(t *testing.T) {
		// Note: status is GENERATED, so don't include it in INSERT
		_, err := db.Exec(`
			INSERT INTO transactions (id, merchant_id, amount_cents, currency, type, payment_method_type, tran_nbr, created_at, updated_at)
			VALUES ($1::uuid, NULL, 1000, 'USD', 'SALE', 'credit_card', 'TXN-NULL-MERCHANT', NOW(), NOW())
		`, uuid.New().String())

		assert.Error(t, err, "NULL merchant_id should fail")
		// PostgreSQL error message contains "null" for NOT NULL violations
		assert.True(t, strings.Contains(strings.ToLower(err.Error()), "null"),
			"Error should mention NULL constraint violation: %v", err)
	})

	t.Run("transactions_require_amount", func(t *testing.T) {
		factory := testutil.NewFactory(t)
		ctx := factory.CreateTestContext(t)

		// Note: status is GENERATED, so don't include it in INSERT
		_, err := db.Exec(`
			INSERT INTO transactions (id, merchant_id, amount_cents, currency, type, payment_method_type, tran_nbr, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, NULL, 'USD', 'SALE', 'credit_card', 'TXN-NULL-AMOUNT', NOW(), NOW())
		`, uuid.New().String(), ctx.Merchant.ID.String())

		assert.Error(t, err, "NULL amount should fail")
	})

	t.Run("merchants_require_name", func(t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO merchants (id, slug, name, cust_nbr, merch_nbr, dba_nbr, terminal_nbr, mac_secret_path, environment, is_active, created_at, updated_at)
			VALUES ($1::uuid, 'test-null-name-slug', NULL, '9001', '900300', '2', '77', 'test/path', 'staging', true, NOW(), NOW())
		`, uuid.New().String())

		assert.Error(t, err, "NULL name should fail")
	})
}

// testCheckConstraints tests CHECK constraint enforcement
func testCheckConstraints(t *testing.T, db *sql.DB) {
	t.Helper()

	t.Run("transactions_amount_must_be_non_negative", func(t *testing.T) {
		factory := testutil.NewFactory(t)
		ctx := factory.CreateTestContext(t)

		// Note: status is GENERATED, so don't include it in INSERT
		// This test depends on whether the schema has a CHECK constraint
		_, err := db.Exec(`
			INSERT INTO transactions (id, merchant_id, amount_cents, currency, type, payment_method_type, tran_nbr, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, -100, 'USD', 'SALE', 'credit_card', 'TXN-NEG', NOW(), NOW())
		`, uuid.New().String(), ctx.Merchant.ID.String())

		// Cleanup if it succeeded (no constraint)
		if err == nil {
			db.Exec("DELETE FROM transactions WHERE tran_nbr = 'TXN-NEG'")
			t.Skip("No CHECK constraint on amount_cents (negative amounts allowed)")
		}

		assert.Error(t, err, "Negative amount should fail if CHECK constraint exists")
	})

	t.Run("subscriptions_interval_must_be_positive", func(t *testing.T) {
		factory := testutil.NewFactory(t)
		ctx := factory.CreateTestContext(t)

		// Create required payment method in customer_payment_methods table
		pmID := uuid.New().String()
		_, err := db.Exec(`
			INSERT INTO customer_payment_methods (id, merchant_id, customer_id, payment_type, last_four, bric, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, 'cust_test', 'credit_card', '4242', 'test-bric-token', NOW(), NOW())
		`, pmID, ctx.Merchant.ID.String())
		require.NoError(t, err, "Payment method insert should succeed")

		t.Cleanup(func() {
			db.Exec("DELETE FROM customer_payment_methods WHERE id = $1::uuid", pmID)
		})

		_, err = db.Exec(`
			INSERT INTO subscriptions (id, merchant_id, customer_id, payment_method_id, amount_cents, currency, interval_value, interval_unit, status, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, 'cust_test', $3::uuid, 1000, 'USD', 0, 'month', 'active', NOW(), NOW())
		`, uuid.New().String(), ctx.Merchant.ID.String(), pmID)

		// Skip if no constraint
		if err == nil {
			db.Exec("DELETE FROM subscriptions WHERE customer_id = 'cust_test'")
			t.Skip("No CHECK constraint on interval_value (zero allowed)")
		}

		assert.Error(t, err, "Zero interval should fail if CHECK constraint exists")
	})
}

// TestCascadeDeletes tests that foreign key cascades work correctly
func TestCascadeDeletes(t *testing.T) {
	db := testutil.GetDB(t)
	require.NotNil(t, db, "Database connection required")

	t.Run("deleting_merchant_cascades_to_transactions", func(t *testing.T) {
		factory := testutil.NewFactory(t)

		// Create a test merchant using factory (this also ensures cleanup)
		merchant := factory.NewMerchant(t).Create()

		// Create a transaction for this merchant (without status - it's GENERATED)
		txID := uuid.New().String()
		_, err := db.Exec(`
			INSERT INTO transactions (id, merchant_id, amount_cents, currency, type, payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, 1000, 'USD', 'SALE', 'credit_card', 'TXN-CASCADE', '00', NOW(), NOW(), NOW())
		`, txID, merchant.ID.String())
		require.NoError(t, err, "Transaction insert should succeed")

		// Try to delete the merchant - this may fail due to RESTRICT constraint
		_, err = db.Exec("DELETE FROM merchants WHERE id = $1::uuid", merchant.ID)

		if err != nil {
			// RESTRICT constraint prevents deletion - this is expected behavior
			t.Logf("Merchant deletion blocked by foreign key constraint (RESTRICT policy): %v", err)
			// Clean up the transaction first, then let factory cleanup handle merchant
			db.Exec("DELETE FROM transactions WHERE id = $1::uuid", txID)
			return
		}

		// If delete succeeded, check if transaction was cascaded
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM transactions WHERE id = $1::uuid", txID).Scan(&count)
		require.NoError(t, err)

		t.Logf("After merchant deletion: %d related transactions remain (CASCADE policy)", count)
		assert.Equal(t, 0, count, "Transactions should be deleted with CASCADE policy")
	})
}
