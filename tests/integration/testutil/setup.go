package testutil

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

// Setup initializes test environment and returns config and client
// NOTE: This does NOT seed any test data. Each test should create its own
// isolated data using the TestFactory. See factories.go for details.
//
// Example usage:
//
//	func TestSomething(t *testing.T) {
//	    cfg, client := testutil.Setup(t)
//	    factory := testutil.NewFactory(t)
//	    ctx := factory.CreateTestContext(t) // Creates merchant + service + access
//	    token, _ := testutil.GenerateJWT(ctx.Service.PrivateKeyPEM, ctx.Service.ServiceID, ctx.Merchant.ID.String(), time.Hour)
//	    // ... use client with token
//	}
func Setup(t *testing.T) (*Config, *Client) {
	t.Helper()

	// Load config from environment
	cfg, err := LoadConfig()
	require.NoError(t, err, "Failed to load test configuration")

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
