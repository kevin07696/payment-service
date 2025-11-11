package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"go.uber.org/zap"
)

// PostgreSQLConfig contains configuration for PostgreSQL connection
type PostgreSQLConfig struct {
	// Connection string
	// Example: "postgres://user:password@localhost:5432/dbname?sslmode=disable"
	DatabaseURL string

	// Pool settings
	MaxConns        int32  // Maximum number of connections in pool
	MinConns        int32  // Minimum number of connections in pool
	MaxConnLifetime string // Max connection lifetime (e.g., "1h")
	MaxConnIdleTime string // Max connection idle time (e.g., "30m")
}

// DefaultPostgreSQLConfig returns default configuration
func DefaultPostgreSQLConfig(databaseURL string) *PostgreSQLConfig {
	return &PostgreSQLConfig{
		DatabaseURL:     databaseURL,
		MaxConns:        25,
		MinConns:        5,
		MaxConnLifetime: "1h",
		MaxConnIdleTime: "30m",
	}
}

// PostgreSQLAdapter provides database access using pgx pool and sqlc-generated queries
type PostgreSQLAdapter struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
	logger  *zap.Logger
}

// NewPostgreSQLAdapter creates a new PostgreSQL adapter with connection pooling
func NewPostgreSQLAdapter(ctx context.Context, cfg *PostgreSQLConfig, logger *zap.Logger) (*PostgreSQLAdapter, error) {
	// Parse connection string and create pool config
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("PostgreSQL adapter initialized",
		zap.String("database", poolConfig.ConnConfig.Database),
		zap.String("host", poolConfig.ConnConfig.Host),
		zap.Uint16("port", poolConfig.ConnConfig.Port),
		zap.Int32("max_conns", cfg.MaxConns),
		zap.Int32("min_conns", cfg.MinConns),
	)

	// Create sqlc queries instance
	queries := sqlc.New(pool)

	return &PostgreSQLAdapter{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}, nil
}

// Queries returns the sqlc queries instance for database operations
func (a *PostgreSQLAdapter) Queries() sqlc.Querier {
	return a.queries
}

// Pool returns the underlying connection pool for advanced operations
func (a *PostgreSQLAdapter) Pool() *pgxpool.Pool {
	return a.pool
}

// Close closes the database connection pool
func (a *PostgreSQLAdapter) Close() {
	a.logger.Info("Closing PostgreSQL connection pool")
	a.pool.Close()
}

// WithTx executes a function within a database transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func (a *PostgreSQLAdapter) WithTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	// Begin transaction
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create queries instance for this transaction
	qtx := a.queries.WithTx(tx)

	// Defer rollback in case of panic
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback(ctx)
			panic(p) // Re-throw panic after rollback
		}
	}()

	// Execute function
	if err := fn(qtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			a.logger.Error("Failed to rollback transaction",
				zap.Error(rbErr),
				zap.NamedError("original_error", err),
			)
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// HealthCheck performs a health check on the database connection
func (a *PostgreSQLAdapter) HealthCheck(ctx context.Context) error {
	return a.pool.Ping(ctx)
}

// Stats returns connection pool statistics
func (a *PostgreSQLAdapter) Stats() *pgxpool.Stat {
	return a.pool.Stat()
}
