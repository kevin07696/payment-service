package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// dbPoolSingleton holds the singleton database connection pool
// PERFORMANCE: Reuses a single connection pool across all tests to avoid
// connection overhead and resource exhaustion
var (
	dbPool     *sql.DB
	dbPoolOnce sync.Once
	dbPoolErr  error
)

// GetDBPool returns a singleton database connection pool for tests
// This function is thread-safe and guarantees only one pool is created
// even when called concurrently from multiple test goroutines.
//
// CRITICAL PERFORMANCE FIX: Creating a new connection pool for every test
// leads to connection exhaustion and test failures. This singleton pattern
// ensures connection reuse across all tests.
//
// The pool is automatically configured with:
// - Connection pooling enabled
// - Idle connections maintained
// - No connection limits (suitable for testing)
//
// Usage:
//
//	db := testutil.GetDBPool(t)
//	// Use db for queries - DO NOT call db.Close()
//	// The pool is shared and will be cleaned up automatically
func GetDBPool(t interface {
	Helper()
	Fatalf(format string, args ...interface{})
}) *sql.DB {
	t.Helper()

	// Initialize pool exactly once using sync.Once
	// This ensures thread-safety and prevents multiple initializations
	dbPoolOnce.Do(func() {
		dbPool, dbPoolErr = initDBPool()
	})

	if dbPoolErr != nil {
		t.Fatalf("Failed to initialize database pool: %v", dbPoolErr)
	}

	return dbPool
}

// initDBPool creates and configures the database connection pool
// This is called exactly once by GetDBPool via sync.Once
func initDBPool() (*sql.DB, error) {
	// Read database configuration from environment variables
	dbHost := getEnvOrDefault("DB_HOST", "localhost")
	dbPort := getEnvOrDefault("DB_PORT", "5432")
	dbUser := getEnvOrDefault("DB_USER", "postgres")
	dbPassword := getEnvOrDefault("DB_PASSWORD", "postgres")
	dbName := getEnvOrDefault("DB_NAME", "payment_service")

	// Build PostgreSQL connection string
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName,
	)

	// Open connection pool
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for test environment
	// These settings balance performance with resource usage
	db.SetMaxOpenConns(25)   // Maximum connections (prevents overwhelming DB)
	db.SetMaxIdleConns(10)   // Keep idle connections for reuse
	db.SetConnMaxLifetime(0) // No connection lifetime limit
	db.SetConnMaxIdleTime(0) // No idle timeout

	// Verify connection is working
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ResetDBPool resets the singleton pool (for testing purposes only)
// WARNING: This should only be called in test cleanup or test utilities
// Do NOT call this in production code
func ResetDBPool() {
	if dbPool != nil {
		dbPool.Close()
		dbPool = nil
		dbPoolErr = nil
		dbPoolOnce = sync.Once{}
	}
}
