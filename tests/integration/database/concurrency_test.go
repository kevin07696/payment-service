//go:build integration
// +build integration

package database_test

import (
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentTransactions tests database behavior under concurrent access
// Following style guide: test atomic SQL operations, verify final state
func TestConcurrentTransactions(t *testing.T) {
	db := testutil.GetDB(t)
	require.NotNil(t, db, "Database connection required")

	t.Run("concurrent_inserts_no_duplicates", func(t *testing.T) {
		testConcurrentInserts(t, db)
	})

	t.Run("concurrent_updates_to_same_record", func(t *testing.T) {
		testConcurrentUpdates(t, db)
	})

	t.Run("transaction_isolation", func(t *testing.T) {
		testTransactionIsolation(t, db)
	})
}

// testConcurrentInserts verifies multiple goroutines can insert simultaneously
// Uses factory-created merchant for proper test isolation
func testConcurrentInserts(t *testing.T, db *sql.DB) {
	t.Helper()

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	const numGoroutines = 10
	const insertsPerGoroutine = 5

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	errors := make(chan error, numGoroutines*insertsPerGoroutine)

	// Use order_id to group test transactions for cleanup
	orderID := "test-order-" + uuid.New().String()[:8]

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < insertsPerGoroutine; j++ {
				txID := uuid.New().String()
				tranNbr := "TXN-CONC-" + uuid.New().String()[:8]

				// Insert transaction with proper schema:
				// - auth_resp='00' and processed_at generates status='approved'
				// - SALE type requires parent_transaction_id = NULL (standalone transaction)
				// - Do NOT include status column (it's GENERATED)
				_, err := db.Exec(`
					INSERT INTO transactions (
						id, merchant_id, amount_cents, currency, type,
						payment_method_type, tran_nbr, order_id,
						auth_resp, processed_at, created_at, updated_at
					) VALUES (
						$1::uuid, $2::uuid, $3, 'USD', 'SALE',
						'credit_card', $4, $5,
						'00', NOW(), NOW(), NOW()
					)
				`, txID, ctx.Merchant.ID.String(), 1000+(workerID*100)+j, tranNbr, orderID)

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					errors <- err
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Cleanup using order_id
	t.Cleanup(func() {
		db.Exec("DELETE FROM transactions WHERE order_id = $1", orderID)
	})

	// All inserts should succeed (unique IDs)
	assert.Equal(t, int64(numGoroutines*insertsPerGoroutine), successCount, "All inserts should succeed")
	assert.Equal(t, int64(0), errorCount, "No errors expected")

	// Verify count in database
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM transactions WHERE order_id = $1", orderID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines*insertsPerGoroutine, count, "All transactions should be in database")

	// Verify all have status='approved' (generated from auth_resp='00')
	var approvedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM transactions WHERE order_id = $1 AND status = 'approved'", orderID).Scan(&approvedCount)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines*insertsPerGoroutine, approvedCount, "All transactions should have approved status")
}

// testConcurrentUpdates verifies concurrent updates to the same record are handled correctly
// Tests atomic SQL UPDATE operations per style guide
func testConcurrentUpdates(t *testing.T, db *sql.DB) {
	t.Helper()

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	// Create a test transaction with auth_resp='00' to get status='approved'
	txID := uuid.New().String()
	_, err := db.Exec(`
		INSERT INTO transactions (
			id, merchant_id, amount_cents, currency, type,
			payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
		) VALUES (
			$1::uuid, $2::uuid, 1000, 'USD', 'SALE',
			'credit_card', 'TXN-UPDATE-TEST', '00', NOW(), NOW(), NOW()
		)
	`, txID, ctx.Merchant.ID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DELETE FROM transactions WHERE id = $1::uuid", txID)
	})

	// Verify initial status is 'approved'
	var initialStatus string
	err = db.QueryRow("SELECT status FROM transactions WHERE id = $1::uuid", txID).Scan(&initialStatus)
	require.NoError(t, err)
	assert.Equal(t, "approved", initialStatus, "Initial status should be approved")

	const numGoroutines = 20
	var wg sync.WaitGroup
	var successCount int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Atomic update: amount_cents = amount_cents + 1
			// This is the correct pattern per style guide - single atomic operation
			result, err := db.Exec(`
				UPDATE transactions
				SET amount_cents = amount_cents + 1,
				    updated_at = NOW()
				WHERE id = $1::uuid
			`, txID)

			if err == nil {
				rows, _ := result.RowsAffected()
				if rows > 0 {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// All updates should succeed
	assert.Equal(t, int64(numGoroutines), successCount, "All updates should succeed")

	// Verify final amount - this checks final state, not just responses
	var finalAmount int64
	err = db.QueryRow("SELECT amount_cents FROM transactions WHERE id = $1::uuid", txID).Scan(&finalAmount)
	require.NoError(t, err)

	expectedAmount := int64(1000 + numGoroutines)
	assert.Equal(t, expectedAmount, finalAmount, "Final amount should reflect all increments")
}

// testTransactionIsolation tests that database transactions are properly isolated
// Tests READ COMMITTED isolation (PostgreSQL default)
func testTransactionIsolation(t *testing.T, db *sql.DB) {
	t.Helper()

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	// Create initial test data with auth_resp='00' for status='approved'
	txID := uuid.New().String()
	_, err := db.Exec(`
		INSERT INTO transactions (
			id, merchant_id, amount_cents, currency, type,
			payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
		) VALUES (
			$1::uuid, $2::uuid, 1000, 'USD', 'SALE',
			'credit_card', 'TXN-ISOLATION-TEST', '00', NOW(), NOW(), NOW()
		)
	`, txID, ctx.Merchant.ID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DELETE FROM transactions WHERE id = $1::uuid", txID)
	})

	// Start first transaction, update but don't commit
	tx1, err := db.Begin()
	require.NoError(t, err)
	defer tx1.Rollback()

	_, err = tx1.Exec("UPDATE transactions SET amount_cents = 2000 WHERE id = $1::uuid", txID)
	require.NoError(t, err)

	// Start second transaction, should see original value (isolation)
	tx2, err := db.Begin()
	require.NoError(t, err)
	defer tx2.Rollback()

	var amountInTx2 int64
	err = tx2.QueryRow("SELECT amount_cents FROM transactions WHERE id = $1::uuid", txID).Scan(&amountInTx2)
	require.NoError(t, err)

	// In READ COMMITTED isolation (PostgreSQL default), tx2 should see committed value (1000)
	assert.Equal(t, int64(1000), amountInTx2, "Transaction 2 should see committed value, not uncommitted update")

	// Now commit tx1
	err = tx1.Commit()
	require.NoError(t, err)

	// After tx1 commits, a new query in tx2 should see the new value
	var amountAfterCommit int64
	err = tx2.QueryRow("SELECT amount_cents FROM transactions WHERE id = $1::uuid", txID).Scan(&amountAfterCommit)
	require.NoError(t, err)

	assert.Equal(t, int64(2000), amountAfterCommit, "After commit, transaction 2 should see updated value")
}

// TestOptimisticLocking tests optimistic concurrency control patterns
// Per style guide: test TOCTOU scenarios with atomic check-and-update
func TestOptimisticLocking(t *testing.T) {
	db := testutil.GetDB(t)
	require.NotNil(t, db, "Database connection required")

	t.Run("atomic_check_and_update_prevents_over_refund", func(t *testing.T) {
		factory := testutil.NewFactory(t)
		ctx := factory.CreateTestContext(t)

		// Create test transaction with 10000 cents
		txID := uuid.New().String()
		_, err := db.Exec(`
			INSERT INTO transactions (
				id, merchant_id, amount_cents, currency, type,
				payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
			) VALUES (
				$1::uuid, $2::uuid, 10000, 'USD', 'SALE',
				'credit_card', 'TXN-OPT-LOCK', '00', NOW(), NOW(), NOW()
			)
		`, txID, ctx.Merchant.ID.String())
		require.NoError(t, err)

		t.Cleanup(func() {
			db.Exec("DELETE FROM transactions WHERE id = $1::uuid", txID)
		})

		// Simulate concurrent refunds using atomic check-and-update pattern
		// This pattern prevents over-refunding: UPDATE ... WHERE amount >= refund_amount
		const numRefunds = 5
		const refundAmount int64 = 3000

		var wg sync.WaitGroup
		var successCount int64

		for i := 0; i < numRefunds; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Atomic check-and-update: only update if amount >= refund
				// This is the TOCTOU-safe pattern from style guide
				result, err := db.Exec(`
					UPDATE transactions
					SET amount_cents = amount_cents - $1
					WHERE id = $2::uuid
					  AND amount_cents >= $1
				`, refundAmount, txID)

				if err == nil {
					rows, _ := result.RowsAffected()
					if rows > 0 {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}()
		}

		wg.Wait()

		// Only 3 refunds should succeed (10000 / 3000 = 3 full refunds)
		// The remaining amount would be 1000, not enough for another 3000 refund
		assert.Equal(t, int64(3), successCount, "Only 3 refunds of 3000 should fit in 10000")

		// Verify final amount - check final state, not just responses
		var finalAmount int64
		err = db.QueryRow("SELECT amount_cents FROM transactions WHERE id = $1::uuid", txID).Scan(&finalAmount)
		require.NoError(t, err)
		assert.Equal(t, int64(1000), finalAmount, "Final amount should be 1000 (10000 - 3*3000)")
	})
}

// TestDeadlockPrevention tests that the system handles potential deadlocks gracefully
// Per style guide: test with timeout-based detection
func TestDeadlockPrevention(t *testing.T) {
	db := testutil.GetDB(t)
	require.NotNil(t, db, "Database connection required")

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	// Create two transactions
	txID1 := uuid.New().String()
	txID2 := uuid.New().String()

	_, err := db.Exec(`
		INSERT INTO transactions (
			id, merchant_id, amount_cents, currency, type,
			payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
		) VALUES (
			$1::uuid, $2::uuid, 1000, 'USD', 'SALE',
			'credit_card', 'TXN-DEAD-1', '00', NOW(), NOW(), NOW()
		)
	`, txID1, ctx.Merchant.ID.String())
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO transactions (
			id, merchant_id, amount_cents, currency, type,
			payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
		) VALUES (
			$1::uuid, $2::uuid, 2000, 'USD', 'SALE',
			'credit_card', 'TXN-DEAD-2', '00', NOW(), NOW(), NOW()
		)
	`, txID2, ctx.Merchant.ID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DELETE FROM transactions WHERE id IN ($1::uuid, $2::uuid)", txID1, txID2)
	})

	// Simulate potential deadlock scenario:
	// Goroutine 1: Lock txID1, then txID2
	// Goroutine 2: Lock txID2, then txID1
	// Without proper handling, this causes a deadlock

	var wg sync.WaitGroup
	errChan := make(chan error, 2)
	done := make(chan struct{})

	wg.Add(2)

	// Goroutine 1: Lock tx1 -> tx2
	go func() {
		defer wg.Done()
		tx, err := db.Begin()
		if err != nil {
			errChan <- err
			return
		}
		defer tx.Rollback()

		// Lock txID1
		_, err = tx.Exec("UPDATE transactions SET amount_cents = amount_cents + 1 WHERE id = $1::uuid", txID1)
		if err != nil {
			errChan <- err
			return
		}

		time.Sleep(50 * time.Millisecond) // Small delay to encourage interleaving

		// Lock txID2
		_, err = tx.Exec("UPDATE transactions SET amount_cents = amount_cents + 1 WHERE id = $1::uuid", txID2)
		if err != nil {
			errChan <- err
			return
		}

		tx.Commit()
	}()

	// Goroutine 2: Lock tx2 -> tx1 (opposite order)
	go func() {
		defer wg.Done()
		tx, err := db.Begin()
		if err != nil {
			errChan <- err
			return
		}
		defer tx.Rollback()

		// Lock txID2 (opposite order)
		_, err = tx.Exec("UPDATE transactions SET amount_cents = amount_cents + 1 WHERE id = $1::uuid", txID2)
		if err != nil {
			errChan <- err
			return
		}

		time.Sleep(50 * time.Millisecond)

		// Lock txID1
		_, err = tx.Exec("UPDATE transactions SET amount_cents = amount_cents + 1 WHERE id = $1::uuid", txID1)
		if err != nil {
			errChan <- err
			return
		}

		tx.Commit()
	}()

	// Wait with timeout to detect if deadlock hangs
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Completed (with or without deadlock detection)
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock detected - operations didn't complete within timeout")
	}

	close(errChan)

	// PostgreSQL will detect and abort one of the transactions with deadlock error
	// This is expected behavior - the important thing is it doesn't hang
	deadlockDetected := false
	for err := range errChan {
		if err != nil {
			t.Logf("Transaction error (expected if deadlock): %v", err)
			deadlockDetected = true
		}
	}

	// Either no deadlock occurred (due to timing) or PostgreSQL detected and handled it
	t.Logf("Deadlock detected: %v (both outcomes are acceptable)", deadlockDetected)
}

// TestTransactionParentRelationship tests parent_transaction_id constraints
// Per schema: SALE/AUTH require NULL parent, CAPTURE/REFUND/VOID require parent
func TestTransactionParentRelationship(t *testing.T) {
	db := testutil.GetDB(t)
	require.NotNil(t, db, "Database connection required")

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	t.Run("auth_requires_null_parent", func(t *testing.T) {
		// AUTH transaction must have parent_transaction_id = NULL
		txID := uuid.New().String()
		_, err := db.Exec(`
			INSERT INTO transactions (
				id, merchant_id, amount_cents, currency, type,
				payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
			) VALUES (
				$1::uuid, $2::uuid, 1000, 'USD', 'AUTH',
				'credit_card', 'TXN-AUTH-1', '00', NOW(), NOW(), NOW()
			)
		`, txID, ctx.Merchant.ID.String())
		require.NoError(t, err, "AUTH with NULL parent should succeed")

		t.Cleanup(func() {
			db.Exec("DELETE FROM transactions WHERE id = $1::uuid", txID)
		})

		// Verify status is 'approved'
		var status string
		err = db.QueryRow("SELECT status FROM transactions WHERE id = $1::uuid", txID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "approved", status)
	})

	t.Run("capture_requires_parent", func(t *testing.T) {
		// First create an AUTH transaction
		authID := uuid.New().String()
		_, err := db.Exec(`
			INSERT INTO transactions (
				id, merchant_id, amount_cents, currency, type,
				payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
			) VALUES (
				$1::uuid, $2::uuid, 1000, 'USD', 'AUTH',
				'credit_card', 'TXN-AUTH-PARENT', '00', NOW(), NOW(), NOW()
			)
		`, authID, ctx.Merchant.ID.String())
		require.NoError(t, err)

		t.Cleanup(func() {
			db.Exec("DELETE FROM transactions WHERE parent_transaction_id = $1::uuid", authID)
			db.Exec("DELETE FROM transactions WHERE id = $1::uuid", authID)
		})

		// CAPTURE must have parent_transaction_id
		captureID := uuid.New().String()
		_, err = db.Exec(`
			INSERT INTO transactions (
				id, merchant_id, amount_cents, currency, type, parent_transaction_id,
				payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
			) VALUES (
				$1::uuid, $2::uuid, 1000, 'USD', 'CAPTURE', $3::uuid,
				'credit_card', 'TXN-CAPTURE-1', '00', NOW(), NOW(), NOW()
			)
		`, captureID, ctx.Merchant.ID.String(), authID)
		require.NoError(t, err, "CAPTURE with parent should succeed")

		// Verify the parent relationship
		var parentID string
		err = db.QueryRow("SELECT parent_transaction_id FROM transactions WHERE id = $1::uuid", captureID).Scan(&parentID)
		require.NoError(t, err)
		assert.Equal(t, authID, parentID)
	})

	t.Run("capture_without_parent_fails", func(t *testing.T) {
		// CAPTURE without parent should fail due to CHECK constraint
		captureID := uuid.New().String()
		_, err := db.Exec(`
			INSERT INTO transactions (
				id, merchant_id, amount_cents, currency, type,
				payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
			) VALUES (
				$1::uuid, $2::uuid, 1000, 'USD', 'CAPTURE',
				'credit_card', 'TXN-CAPTURE-FAIL', '00', NOW(), NOW(), NOW()
			)
		`, captureID, ctx.Merchant.ID.String())

		assert.Error(t, err, "CAPTURE without parent should fail")
		if err != nil {
			t.Logf("Expected error for CAPTURE without parent: %v", err)
		}
	})
}

// TestStatusGeneration tests the GENERATED status column
func TestStatusGeneration(t *testing.T) {
	db := testutil.GetDB(t)
	require.NotNil(t, db, "Database connection required")

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	testCases := []struct {
		name        string
		authResp    *string
		processedAt bool // true = NOW(), false = NULL
		expected    string
	}{
		{"pending", nil, false, "pending"},         // auth_resp=NULL, processed_at=NULL
		{"failed", nil, true, "failed"},            // auth_resp=NULL, processed_at=NOW()
		{"approved", strPtr("00"), true, "approved"}, // auth_resp='00', processed_at=NOW()
		{"declined", strPtr("05"), true, "declined"}, // auth_resp='05', processed_at=NOW()
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			txID := uuid.New().String()
			tranNbr := "TXN-STATUS-" + tc.name

			var err error
			if tc.authResp == nil && !tc.processedAt {
				// pending: both NULL
				_, err = db.Exec(`
					INSERT INTO transactions (
						id, merchant_id, amount_cents, currency, type,
						payment_method_type, tran_nbr, created_at, updated_at
					) VALUES (
						$1::uuid, $2::uuid, 1000, 'USD', 'SALE',
						'credit_card', $3, NOW(), NOW()
					)
				`, txID, ctx.Merchant.ID.String(), tranNbr)
			} else if tc.authResp == nil && tc.processedAt {
				// failed: auth_resp=NULL, processed_at=NOW()
				_, err = db.Exec(`
					INSERT INTO transactions (
						id, merchant_id, amount_cents, currency, type,
						payment_method_type, tran_nbr, processed_at, created_at, updated_at
					) VALUES (
						$1::uuid, $2::uuid, 1000, 'USD', 'SALE',
						'credit_card', $3, NOW(), NOW(), NOW()
					)
				`, txID, ctx.Merchant.ID.String(), tranNbr)
			} else {
				// approved/declined: auth_resp set, processed_at=NOW()
				_, err = db.Exec(`
					INSERT INTO transactions (
						id, merchant_id, amount_cents, currency, type,
						payment_method_type, tran_nbr, auth_resp, processed_at, created_at, updated_at
					) VALUES (
						$1::uuid, $2::uuid, 1000, 'USD', 'SALE',
						'credit_card', $3, $4, NOW(), NOW(), NOW()
					)
				`, txID, ctx.Merchant.ID.String(), tranNbr, *tc.authResp)
			}
			require.NoError(t, err)

			t.Cleanup(func() {
				db.Exec("DELETE FROM transactions WHERE id = $1::uuid", txID)
			})

			// Verify generated status
			var status string
			err = db.QueryRow("SELECT status FROM transactions WHERE id = $1::uuid", txID).Scan(&status)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, status, "Status should be generated correctly")
		})
	}
}

// TestConcurrentStatusTransitions tests concurrent updates that affect generated status
func TestConcurrentStatusTransitions(t *testing.T) {
	db := testutil.GetDB(t)
	require.NotNil(t, db, "Database connection required")

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	// Create pending transaction (auth_resp=NULL, processed_at=NULL)
	txID := uuid.New().String()
	_, err := db.Exec(`
		INSERT INTO transactions (
			id, merchant_id, amount_cents, currency, type,
			payment_method_type, tran_nbr, created_at, updated_at
		) VALUES (
			$1::uuid, $2::uuid, 1000, 'USD', 'SALE',
			'credit_card', 'TXN-STATUS-TRANS', NOW(), NOW()
		)
	`, txID, ctx.Merchant.ID.String())
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Exec("DELETE FROM transactions WHERE id = $1::uuid", txID)
	})

	// Verify initial status is 'pending'
	var status string
	err = db.QueryRow("SELECT status FROM transactions WHERE id = $1::uuid", txID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "pending", status)

	// Concurrent updates: one approves, one declines - only one should win
	var wg sync.WaitGroup
	var approveWon, declineWon atomic.Bool

	wg.Add(2)

	go func() {
		defer wg.Done()
		result, err := db.Exec(`
			UPDATE transactions
			SET auth_resp = '00', processed_at = NOW()
			WHERE id = $1::uuid AND auth_resp IS NULL
		`, txID)
		if err == nil {
			rows, _ := result.RowsAffected()
			if rows > 0 {
				approveWon.Store(true)
			}
		}
	}()

	go func() {
		defer wg.Done()
		result, err := db.Exec(`
			UPDATE transactions
			SET auth_resp = '05', processed_at = NOW()
			WHERE id = $1::uuid AND auth_resp IS NULL
		`, txID)
		if err == nil {
			rows, _ := result.RowsAffected()
			if rows > 0 {
				declineWon.Store(true)
			}
		}
	}()

	wg.Wait()

	// Exactly one should succeed (WHERE auth_resp IS NULL ensures only first wins)
	assert.True(t, approveWon.Load() != declineWon.Load(),
		"Exactly one update should win (approve=%v, decline=%v)", approveWon.Load(), declineWon.Load())

	// Verify final status is consistent with auth_resp
	var finalStatus, authResp string
	err = db.QueryRow("SELECT status, auth_resp FROM transactions WHERE id = $1::uuid", txID).Scan(&finalStatus, &authResp)
	require.NoError(t, err)

	if authResp == "00" {
		assert.Equal(t, "approved", finalStatus)
	} else {
		assert.Equal(t, "declined", finalStatus)
	}
}

// Helper function for string pointer
func strPtr(s string) *string {
	return &s
}

// TestHighConcurrency runs stress test with many concurrent operations
// Per style guide: "Test with high concurrency - 10 concurrent requests isn't enough, try 100+"
func TestHighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high concurrency test in short mode")
	}

	db := testutil.GetDB(t)
	require.NotNil(t, db, "Database connection required")

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	const numGoroutines = 100
	orderID := "stress-test-" + uuid.New().String()[:8]

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			txID := uuid.New().String()
			tranNbr := "TXN-STRESS-" + uuid.New().String()[:8]

			_, err := db.Exec(`
				INSERT INTO transactions (
					id, merchant_id, amount_cents, currency, type,
					payment_method_type, tran_nbr, order_id,
					auth_resp, processed_at, created_at, updated_at
				) VALUES (
					$1::uuid, $2::uuid, $3, 'USD', 'SALE',
					'credit_card', $4, $5,
					'00', NOW(), NOW(), NOW()
				)
			`, txID, ctx.Merchant.ID.String(), 1000+workerID, tranNbr, orderID)

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Cleanup(func() {
		db.Exec("DELETE FROM transactions WHERE order_id = $1", orderID)
	})

	t.Logf("High concurrency test: %d successful, %d errors in %v", successCount, errorCount, elapsed)

	assert.Equal(t, int64(numGoroutines), successCount, "All high-concurrency inserts should succeed")
	assert.Equal(t, int64(0), errorCount, "No errors expected in high-concurrency test")

	// Verify all transactions exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM transactions WHERE order_id = $1", orderID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines, count, "All transactions should be in database")
}
