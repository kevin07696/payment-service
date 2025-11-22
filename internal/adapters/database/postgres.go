package database

import (
	"context"
	"fmt"
	"time"

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

	// Query timeout settings
	SimpleQueryTimeout  time.Duration // Timeout for simple queries (ID lookups, single row operations)
	ComplexQueryTimeout time.Duration // Timeout for complex queries (JOINs, aggregations, filters)
	ReportQueryTimeout  time.Duration // Timeout for report/analytics queries
}

// DefaultPostgreSQLConfig returns default configuration
// Pool sizing rationale:
// - MaxConns: 50 allows handling burst traffic (25-50 req/s at ~50ms avg query time)
// - MinConns: 10 (20% of max) keeps warm connections ready, reduces cold-start latency
// - Lifetime/IdleTime: Prevents stale connections, allows graceful rotation
// - MaxConnLifetime: 30m follows PostgreSQL best practices (was 1h)
func DefaultPostgreSQLConfig(databaseURL string) *PostgreSQLConfig {
	return &PostgreSQLConfig{
		DatabaseURL:     databaseURL,
		MaxConns:        50, // Increased for production workload (was 25)
		MinConns:        10, // Increased to 20% of max (was 5)
		MaxConnLifetime: "30m", // Reduced from 1h - PostgreSQL best practice
		MaxConnIdleTime: "15m", // Reduced to release idle connections faster (was 30m)
		// Query timeouts - tiered based on complexity
		SimpleQueryTimeout:  2 * time.Second,  // ID lookups, single row operations
		ComplexQueryTimeout: 5 * time.Second,  // JOINs, filters, aggregations
		ReportQueryTimeout:  30 * time.Second, // Analytics, reports
	}
}

// PostgreSQLAdapter provides database access using pgx pool and sqlc-generated queries
type PostgreSQLAdapter struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
	logger  *zap.Logger
	config  *PostgreSQLConfig
	stopCh  chan struct{} // Channel to signal pool monitoring goroutine shutdown
}

// NewPostgreSQLAdapter creates a new PostgreSQL adapter with connection pooling
func NewPostgreSQLAdapter(ctx context.Context, cfg *PostgreSQLConfig, logger *zap.Logger) (*PostgreSQLAdapter, error) {
	// Parse connection string and create pool config
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		// SECURITY: Do not wrap error as it may contain the database URL with password
		// Return a sanitized error message instead
		return nil, fmt.Errorf("failed to parse database configuration: invalid connection parameters")
	}

	// Configure pool settings
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns

	// Parse and set connection lifetime
	if cfg.MaxConnLifetime != "" {
		maxLifetime, err := time.ParseDuration(cfg.MaxConnLifetime)
		if err != nil {
			return nil, fmt.Errorf("invalid MaxConnLifetime: %w", err)
		}
		poolConfig.MaxConnLifetime = maxLifetime
	}

	// Parse and set idle time
	if cfg.MaxConnIdleTime != "" {
		maxIdleTime, err := time.ParseDuration(cfg.MaxConnIdleTime)
		if err != nil {
			return nil, fmt.Errorf("invalid MaxConnIdleTime: %w", err)
		}
		poolConfig.MaxConnIdleTime = maxIdleTime
	}

	// Set health check period to detect and replace broken connections
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		// SECURITY: Do not wrap error as it may contain connection details
		// Provide generic error message for security
		return nil, fmt.Errorf("failed to establish database connection pool")
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		// SECURITY: Ping errors are safe to wrap (no connection string exposure)
		return nil, fmt.Errorf("database connection test failed: %w", err)
	}

	logger.Info("PostgreSQL adapter initialized",
		zap.String("database", poolConfig.ConnConfig.Database),
		zap.String("host", poolConfig.ConnConfig.Host),
		zap.Uint16("port", poolConfig.ConnConfig.Port),
		zap.Int32("max_conns", poolConfig.MaxConns),
		zap.Int32("min_conns", poolConfig.MinConns),
		zap.Duration("max_conn_lifetime", poolConfig.MaxConnLifetime),
		zap.Duration("max_conn_idle_time", poolConfig.MaxConnIdleTime),
		zap.Duration("health_check_period", poolConfig.HealthCheckPeriod),
	)

	// Create sqlc queries instance
	queries := sqlc.New(pool)

	return &PostgreSQLAdapter{
		pool:    pool,
		queries: queries,
		logger:  logger,
		config:  cfg,
		stopCh:  make(chan struct{}),
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

// Shutdown gracefully stops the pool monitoring goroutine
func (a *PostgreSQLAdapter) Shutdown() {
	select {
	case <-a.stopCh:
		// Already closed, do nothing
	default:
		close(a.stopCh)
	}
}

// Close closes the database connection pool and stops monitoring
func (a *PostgreSQLAdapter) Close() {
	a.logger.Info("Closing PostgreSQL connection pool")
	a.Shutdown() // Stop monitoring goroutine first
	a.pool.Close()
}

// WithTx executes a function within a database transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func (a *PostgreSQLAdapter) WithTx(ctx context.Context, fn func(sqlc.Querier) error) error {
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

// StartPoolMonitoring starts a background goroutine that monitors connection pool health
// It logs warnings when pool utilization is high and errors when near exhaustion
func (a *PostgreSQLAdapter) StartPoolMonitoring(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				a.logger.Info("Stopping connection pool monitoring (context cancelled)")
				return
			case <-a.stopCh:
				a.logger.Info("Stopping connection pool monitoring (shutdown requested)")
				return
			case <-ticker.C:
				stat := a.pool.Stat()
				total := stat.MaxConns()
				acquired := stat.AcquiredConns()
				idle := stat.IdleConns()
				utilization := float64(acquired) / float64(total) * 100

				a.logger.Debug("Database connection pool status",
					zap.Int32("total_connections", total),
					zap.Int32("acquired_connections", acquired),
					zap.Int32("idle_connections", idle),
					zap.Float64("utilization_percent", utilization),
				)

				// Warn at 80% utilization
				if utilization > 80 {
					a.logger.Warn("Database connection pool highly utilized",
						zap.Float64("utilization_percent", utilization),
						zap.Int32("acquired", acquired),
						zap.Int32("total", total),
						zap.String("recommendation", "Consider increasing MaxConns or investigating connection leaks"),
					)
				}

				// Error at 95% utilization (near exhaustion)
				if utilization > 95 {
					a.logger.Error("Database connection pool near exhaustion",
						zap.Float64("utilization_percent", utilization),
						zap.Int32("acquired", acquired),
						zap.Int32("total", total),
						zap.String("action_required", "CRITICAL: Scale up connections or fix leaks immediately"),
					)
				}
			}
		}
	}()

	a.logger.Info("Database connection pool monitoring started",
		zap.Duration("check_interval", interval),
	)
}

// SimpleQueryContext creates a context with timeout for simple queries
// Use for: ID lookups, single row operations, existence checks
func (a *PostgreSQLAdapter) SimpleQueryContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, a.config.SimpleQueryTimeout)
}

// ComplexQueryContext creates a context with timeout for complex queries
// Use for: JOINs, WHERE clauses with multiple conditions, aggregations, GROUP BY
func (a *PostgreSQLAdapter) ComplexQueryContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, a.config.ComplexQueryTimeout)
}

// ReportQueryContext creates a context with timeout for report/analytics queries
// Use for: Large scans, complex aggregations, analytics, reports
func (a *PostgreSQLAdapter) ReportQueryContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, a.config.ReportQueryTimeout)
}
