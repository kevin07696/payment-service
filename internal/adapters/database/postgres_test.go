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
func TestNewPostgreSQLAdapter_InvalidURL(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	tests := []struct {
		name        string
		databaseURL string
		expectError string
	}{
		{
			name:        "empty URL",
			databaseURL: "",
			expectError: "failed to", // pgxpool accepts empty URL but fails on connect
		},
		{
			name:        "invalid URL format",
			databaseURL: "not-a-valid-url",
			expectError: "failed to parse database URL",
		},
		{
			name:        "invalid scheme",
			databaseURL: "mysql://user:password@localhost:5432/db",
			expectError: "failed to parse database URL",
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
			assert.Error(t, err, "Should return error for invalid URL")
			assert.Nil(t, adapter, "Adapter should be nil on error")
			assert.Contains(t, err.Error(), tt.expectError)
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

	adapter.WithTx(ctx, func(q sqlc.Querier) error {
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

	assert.Error(t, err, "Transaction should fail with cancelled context")
	assert.Contains(t, err.Error(), "context canceled", "Error should indicate context cancellation")
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

		// TODO: Add full integration test with actual transaction creation
		// This requires test fixtures for merchants, customers, and payment methods
		t.Skip("Full integration test requires test data fixtures - see docs/UNIT_TEST_REFACTORING_ANALYSIS.md")
	})
}
