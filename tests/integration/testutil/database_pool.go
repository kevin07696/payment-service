package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// dbPoolSingleton holds the singleton database connection pool
// PERFORMANCE: Reuses a single connection pool across all tests to avoid
// connection overhead and resource exhaustion
var (
	dbPool     *sql.DB
	dbPoolOnce sync.Once
	dbPoolErr  error

	// pgxPoolSingleton for SQLC queries
	pgxPool     *pgxpool.Pool
	pgxPoolOnce sync.Once
	pgxPoolErr  error
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
//
// Configuration follows 12-factor app methodology - all config from environment:
// - DATABASE_URL: Full connection string (preferred for CI/production)
// - DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME: Individual vars (same as server)
//
// IMPORTANT: Tests use the SAME database config as the server to ensure
// test data (services, merchants) is visible to the running server.
func initDBPool() (*sql.DB, error) {
	connStr, err := buildDatabaseConnString()
	if err != nil {
		return nil, err
	}

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

// buildDatabaseConnString builds the database connection string from environment variables
// Uses the same DB_* variables as the server to ensure tests connect to the same database.
// This is critical for integration tests where test data must be visible to the running server.
func buildDatabaseConnString() (string, error) {
	// Check for DATABASE_URL first (full connection string - preferred for CI/production)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL, nil
	}

	// Fall back to individual environment variables (same as server uses)
	// NO hardcoded defaults - fail fast if config is missing
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Validate required configuration
	var missing []string
	if dbHost == "" {
		missing = append(missing, "DB_HOST")
	}
	if dbPort == "" {
		missing = append(missing, "DB_PORT")
	}
	if dbUser == "" {
		missing = append(missing, "DB_USER")
	}
	if dbPassword == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if dbName == "" {
		missing = append(missing, "DB_NAME")
	}

	if len(missing) > 0 {
		return "", fmt.Errorf("missing required database configuration: %v (set DATABASE_URL or individual DB_* variables)", missing)
	}

	// Build PostgreSQL connection string (host= format for database/sql)
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName,
	)
	return connStr, nil
}

// buildPgxConnString builds the pgx connection string from environment variables
// Uses postgres:// URL format required by pgxpool
func buildPgxConnString() (string, error) {
	// Check for DATABASE_URL first (full connection string - preferred for CI/production)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL, nil
	}

	// Fall back to individual environment variables (same as server uses)
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// Validate required configuration
	var missing []string
	if dbHost == "" {
		missing = append(missing, "DB_HOST")
	}
	if dbPort == "" {
		missing = append(missing, "DB_PORT")
	}
	if dbUser == "" {
		missing = append(missing, "DB_USER")
	}
	if dbPassword == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if dbName == "" {
		missing = append(missing, "DB_NAME")
	}

	if len(missing) > 0 {
		return "", fmt.Errorf("missing required database configuration: %v (set DATABASE_URL or individual DB_* variables)", missing)
	}

	// Build PostgreSQL URL format for pgxpool
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName,
	)
	return connStr, nil
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

// GetPgxPool returns a singleton pgxpool.Pool for SQLC queries in tests
// This function is thread-safe and guarantees only one pool is created
// even when called concurrently from multiple test goroutines.
//
// Usage:
//
//	pool := testutil.GetPgxPool(t)
//	queries := sqlc.New(pool)
//	// Use queries for SQLC operations
func GetPgxPool(t interface {
	Helper()
	Fatalf(format string, args ...interface{})
}) *pgxpool.Pool {
	t.Helper()

	pgxPoolOnce.Do(func() {
		pgxPool, pgxPoolErr = initPgxPool()
	})

	if pgxPoolErr != nil {
		t.Fatalf("Failed to initialize pgx pool: %v", pgxPoolErr)
	}

	return pgxPool
}

// initPgxPool creates and configures the pgx connection pool
// This is called exactly once by GetPgxPool via sync.Once
//
// Uses the same database configuration as initDBPool to ensure consistency.
func initPgxPool() (*pgxpool.Pool, error) {
	connStr, err := buildPgxConnString()
	if err != nil {
		return nil, err
	}

	// Parse config and create pool
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pgx config: %w", err)
	}

	// Configure pool for test environment
	config.MaxConns = 25
	config.MinConns = 5

	// Create pool with timeout to prevent hanging on connection issues
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}

	// Verify connection with timeout
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// ResetPgxPool resets the singleton pgx pool (for testing purposes only)
func ResetPgxPool() {
	if pgxPool != nil {
		pgxPool.Close()
		pgxPool = nil
		pgxPoolErr = nil
		pgxPoolOnce = sync.Once{}
	}
}
