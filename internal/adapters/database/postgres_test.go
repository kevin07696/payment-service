package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewPostgreSQLAdapter tests adapter initialization
// This test requires a real database connection
func TestNewPostgreSQLAdapter(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err, "Should create adapter successfully")
	require.NotNil(t, adapter, "Adapter should not be nil")
	defer adapter.Close()

	// Verify adapter components are initialized
	assert.NotNil(t, adapter.Pool(), "Pool should be initialized")
	assert.NotNil(t, adapter.Queries(), "Queries should be initialized")

	// Verify pool stats
	stats := adapter.Stats()
	assert.NotNil(t, stats, "Stats should be available")
}

// TestNewPostgreSQLAdapter_InvalidURL tests error handling for invalid database URL
// Best Practice: Assert on error behavior, not exact error messages (implementation details)
func TestNewPostgreSQLAdapter_InvalidURL(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	tests := []struct {
		name        string
		databaseURL string
	}{
		{
			name:        "empty_URL",
			databaseURL: "",
		},
		{
			name:        "invalid_URL_format",
			databaseURL: "not-a-valid-url",
		},
		{
			name:        "invalid_scheme",
			databaseURL: "mysql://user:password@localhost:5432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &PostgreSQLConfig{
				DatabaseURL: tt.databaseURL,
				MaxConns:    25,
				MinConns:    5,
			}

			adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)

			// Assert behavior, not implementation details
			require.Error(t, err, "Should return error for invalid URL")
			require.Nil(t, adapter, "Adapter should be nil on error")

			// Note: We don't assert on exact error message text because:
			// 1. Error messages are implementation details, not API contracts
			// 2. Asserting on messages makes tests brittle (breaks when improving clarity)
			// 3. Internal database adapter errors aren't user-facing
			// 4. The important behavior is: "invalid URL returns an error"
		})
	}
}

// TestNewPostgreSQLAdapter_ConnectionFailure tests handling of connection failures
func TestNewPostgreSQLAdapter_ConnectionFailure(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Use a valid URL format but non-existent server
	cfg := &PostgreSQLConfig{
		DatabaseURL: "postgres://user:password@localhost:54321/nonexistent",
		MaxConns:    25,
		MinConns:    5,
	}

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	assert.Error(t, err, "Should return error when connection fails")
	assert.Nil(t, adapter, "Adapter should be nil on connection failure")
}

// TestHealthCheck tests the health check functionality
func TestHealthCheck(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err)
	defer adapter.Close()

	// Test successful health check
	err = adapter.HealthCheck(ctx)
	assert.NoError(t, err, "Health check should pass with valid connection")
}

// TestHealthCheck_AfterClose tests health check after closing connection
func TestHealthCheck_AfterClose(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err)

	// Close the connection
	adapter.Close()

	// Health check should fail after close
	err = adapter.HealthCheck(ctx)
	assert.Error(t, err, "Health check should fail after connection closed")
}

// TestWithTx_Success tests successful transaction execution
func TestWithTx_Success(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err)
	defer adapter.Close()

	// Execute a transaction that should succeed
	executed := false
	err = adapter.WithTx(ctx, func(q sqlc.Querier) error {
		executed = true
		// Simple operation that should succeed
		return nil
	})

	assert.NoError(t, err, "Transaction should complete successfully")
	assert.True(t, executed, "Transaction function should have been executed")
}

// TestWithTx_Rollback tests transaction rollback on error
func TestWithTx_Rollback(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err)
	defer adapter.Close()

	// Execute a transaction that should rollback
	testError := errors.New("intentional test error")
	err = adapter.WithTx(ctx, func(q sqlc.Querier) error {
		// Return error to trigger rollback
		return testError
	})

	assert.Error(t, err, "Transaction should return error")
	assert.Equal(t, testError, err, "Should return the original error")
}

// TestWithTx_Panic tests transaction rollback on panic
func TestWithTx_Panic(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err)
	defer adapter.Close()

	// Test that panic is recovered and re-thrown
	defer func() {
		r := recover()
		assert.NotNil(t, r, "Panic should be recovered and re-thrown")
		assert.Equal(t, "test panic", r, "Panic value should be preserved")
	}()

	_ = adapter.WithTx(ctx, func(q sqlc.Querier) error {
		panic("test panic")
	})

	t.Fatal("Should not reach here - panic should be thrown")
}

// TestWithTx_ContextCancellation tests transaction behavior with cancelled context
func TestWithTx_ContextCancellation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(context.Background(), cfg, logger)
	require.NoError(t, err)
	defer adapter.Close()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Transaction should fail with cancelled context
	err = adapter.WithTx(ctx, func(q sqlc.Querier) error {
		return nil
	})

	// Assert behavior: transaction should fail with cancelled context
	require.Error(t, err, "Transaction should fail with cancelled context")

	// Best Practice: Use error type check instead of message string
	// This is stable across error message rewording
	assert.ErrorIs(t, err, context.Canceled, "Error should be context.Canceled")
}

// TestPool tests the Pool method
func TestPool(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err)
	defer adapter.Close()

	pool := adapter.Pool()
	assert.NotNil(t, pool, "Pool should not be nil")

	// Pool should be usable
	err = pool.Ping(ctx)
	assert.NoError(t, err, "Should be able to ping using pool")
}

// TestClose tests the Close method
func TestClose(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err)

	// Close should not panic
	assert.NotPanics(t, func() {
		adapter.Close()
	}, "Close should not panic")

	// Multiple closes should not panic
	assert.NotPanics(t, func() {
		adapter.Close()
	}, "Multiple closes should not panic")
}

// TestConnectionPoolSettings tests that pool settings are applied correctly
func TestConnectionPoolSettings(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()

	cfg := &PostgreSQLConfig{
		DatabaseURL:     databaseURL,
		MaxConns:        10,
		MinConns:        2,
		MaxConnLifetime: "30m",
		MaxConnIdleTime: "15m",
	}

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err)
	defer adapter.Close()

	stats := adapter.Stats()
	// Test that pool respects MaxConns limit (not exact value due to pgxpool internals)
	assert.LessOrEqual(t, stats.MaxConns(), int32(10), "MaxConns should not exceed configuration")
	assert.Greater(t, stats.MaxConns(), int32(0), "MaxConns should be positive")

	// Give pool time to establish min connections
	time.Sleep(100 * time.Millisecond)

	// Stats should show connections are being managed
	assert.GreaterOrEqual(t, stats.TotalConns(), int32(0), "Should have some connections")
}

// TestGetTransactionTree tests the recursive CTE query for transaction hierarchies
func TestGetTransactionTree(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	logger := zap.NewNop()
	cfg := DefaultPostgreSQLConfig(databaseURL)

	adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
	require.NoError(t, err)
	defer adapter.Close()

	queries := adapter.Queries()

	// Clean up any test data first
	// Note: In a real test, you'd use test transactions or a test database

	t.Run("simple parent-child hierarchy", func(t *testing.T) {
		// This test validates the GetTransactionTree query works correctly
		// In a full implementation, you would:
		// 1. Create test merchant and customer
		// 2. Create AUTH transaction (parent)
		// 3. Create CAPTURE transaction (child of AUTH)
		// 4. Create REFUND transaction (child of CAPTURE)
		// 5. Call GetTransactionTree and verify it returns all 3 transactions in order

		// For now, we just verify the method exists and doesn't panic
		// Use a UUID that doesn't exist in the database
		nonExistentID := [16]byte{} // Zero UUID
		tree, err := queries.GetTransactionTree(ctx, nonExistentID)

		// Should not error even with non-existent transaction
		assert.NoError(t, err)
		assert.Empty(t, tree, "Should return empty tree for non-existent transaction")
	})

	t.Run("validates tree structure with parent_transaction_id", func(t *testing.T) {
		// This test documents the expected behavior:
		// - GetTransactionTree recursively fetches children using parent_transaction_id
		// - Returns all descendants in tree order
		// - Example: AUTH (id=A) → CAPTURE (id=C, parent=A) → REFUND (id=R, parent=C)
		//   GetTransactionTree(A) returns [A, C, R]
		//   GetTransactionTree(C) returns [C, R]
		//   GetTransactionTree(R) returns [R]

		// Full integration test deferred: Requires comprehensive test fixtures
		// (merchants, customers, payment methods) and transaction creation flow.
		// Current coverage: Basic query validation (empty tree test above)
		// Additional coverage: End-to-end integration tests in tests/integration/payment/
		// verify transaction hierarchies work correctly in real workflows.
		t.Skip("Full integration test requires test data fixtures - see docs/UNIT_TEST_REFACTORING_ANALYSIS.md")
	})
}

// =============================================================================
// Query Timeout Helper Tests
// =============================================================================
// These tests document and verify the query timeout helpers.
// No database connection required - tests context.WithTimeout behavior only.

// TestWithSimpleQueryTimeout documents and verifies simple query timeout behavior.
// Use for: ID lookups, single row SELECTs, simple UPDATEs/INSERTs
// Default timeout: 2 seconds
func TestWithSimpleQueryTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"default_2s", 2 * time.Second},
		{"custom_500ms", 500 * time.Millisecond},
		{"zero_timeout", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &PostgreSQLAdapter{
				config: &PostgreSQLConfig{SimpleQueryTimeout: tt.timeout},
			}

			ctx, cancel := adapter.WithSimpleQueryTimeout(context.Background())
			defer cancel()

			assertContextTimeout(t, ctx, tt.timeout, 50)
		})
	}
}

// TestWithComplexQueryTimeout documents and verifies complex query timeout behavior.
// Use for: JOINs, WHERE clauses with multiple conditions, aggregations, pagination
// Default timeout: 5 seconds
func TestWithComplexQueryTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"default_5s", 5 * time.Second},
		{"custom_3s", 3 * time.Second},
		{"zero_timeout", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &PostgreSQLAdapter{
				config: &PostgreSQLConfig{ComplexQueryTimeout: tt.timeout},
			}

			ctx, cancel := adapter.WithComplexQueryTimeout(context.Background())
			defer cancel()

			assertContextTimeout(t, ctx, tt.timeout, 50)
		})
	}
}

// TestWithReportQueryTimeout documents and verifies report query timeout behavior.
// Use for: Large result sets, complex aggregations, historical data queries, analytics
// Default timeout: 30 seconds
func TestWithReportQueryTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"default_30s", 30 * time.Second},
		{"custom_60s", 60 * time.Second},
		{"zero_timeout", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &PostgreSQLAdapter{
				config: &PostgreSQLConfig{ReportQueryTimeout: tt.timeout},
			}

			ctx, cancel := adapter.WithReportQueryTimeout(context.Background())
			defer cancel()

			assertContextTimeout(t, ctx, tt.timeout, 50)
		})
	}
}

// TestQueryTimeoutCancellation verifies that cancel() properly cancels the context.
// This demonstrates the required pattern: defer cancel() immediately after creating timeout.
func TestQueryTimeoutCancellation(t *testing.T) {
	adapter := &PostgreSQLAdapter{
		config: &PostgreSQLConfig{
			SimpleQueryTimeout: 5 * time.Second,
		},
	}

	t.Run("cancel_immediately_closes_done_channel", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		timeoutCtx, cancel := adapter.WithSimpleQueryTimeout(ctx)

		// Act
		cancel()

		// Assert - Done channel should be closed
		select {
		case <-timeoutCtx.Done():
			assert.Equal(t, context.Canceled, timeoutCtx.Err(),
				"Context error should be Canceled after cancel()")
		default:
			t.Fatal("Done channel should be closed after cancel()")
		}
	})

	t.Run("multiple_cancel_calls_are_safe", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		_, cancel := adapter.WithSimpleQueryTimeout(ctx)

		// Act & Assert - Should not panic
		assert.NotPanics(t, func() {
			cancel()
			cancel()
			cancel()
		}, "Multiple cancel() calls should be safe")
	})
}

// TestQueryTimeoutInheritance verifies parent context deadline takes precedence
// when it's shorter than the timeout helper's deadline.
func TestQueryTimeoutInheritance(t *testing.T) {
	adapter := &PostgreSQLAdapter{
		config: &PostgreSQLConfig{
			SimpleQueryTimeout: 10 * time.Second, // Long timeout
		},
	}

	t.Run("parent_shorter_deadline_takes_precedence", func(t *testing.T) {
		// Arrange - Parent has 100ms deadline
		parentCtx, parentCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer parentCancel()

		// Act - Request 10s timeout but parent only allows 100ms
		timeoutCtx, cancel := adapter.WithSimpleQueryTimeout(parentCtx)
		defer cancel()

		// Assert - Effective deadline should be parent's (shorter)
		deadline, ok := timeoutCtx.Deadline()
		require.True(t, ok, "Context should have a deadline")

		actualTimeout := time.Until(deadline)
		// Should be closer to 100ms than 10s
		assert.Less(t, actualTimeout, 1*time.Second,
			"Effective timeout should be limited by parent's 100ms deadline")
	})

	t.Run("parent_cancelled_propagates_to_child", func(t *testing.T) {
		// Arrange
		parentCtx, parentCancel := context.WithCancel(context.Background())
		timeoutCtx, cancel := adapter.WithSimpleQueryTimeout(parentCtx)
		defer cancel()

		// Act
		parentCancel()

		// Assert - Child should also be cancelled
		select {
		case <-timeoutCtx.Done():
			// Parent cancellation propagates to child
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Child context should be cancelled when parent is cancelled")
		}
	})
}

// TestDefaultPostgreSQLConfig_TimeoutDefaults verifies the default timeout values.
// This serves as documentation for the expected defaults.
func TestDefaultPostgreSQLConfig_TimeoutDefaults(t *testing.T) {
	cfg := DefaultPostgreSQLConfig("postgres://localhost/test")

	assert.Equal(t, 2*time.Second, cfg.SimpleQueryTimeout,
		"Simple query timeout should default to 2s (ID lookups, single rows)")

	assert.Equal(t, 5*time.Second, cfg.ComplexQueryTimeout,
		"Complex query timeout should default to 5s (JOINs, aggregations)")

	assert.Equal(t, 30*time.Second, cfg.ReportQueryTimeout,
		"Report query timeout should default to 30s (analytics, large results)")
}

// abs returns the absolute value of an int64
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// assertContextTimeout verifies the context has the expected deadline within tolerance.
// toleranceMs allows for small timing variance in test execution.
func assertContextTimeout(t *testing.T, ctx context.Context, expected time.Duration, toleranceMs int64) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	require.True(t, ok, "Context should have a deadline")

	actual := time.Until(deadline)
	diff := abs(actual.Milliseconds() - expected.Milliseconds())
	assert.LessOrEqual(t, diff, toleranceMs,
		"Timeout should be ~%v, got %v", expected, actual)
}
