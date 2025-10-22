package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBExecutor implements the DBPort interface for PostgreSQL
type DBExecutor struct {
	pool *pgxpool.Pool
}

// NewDBExecutor creates a new PostgreSQL database executor
func NewDBExecutor(pool *pgxpool.Pool) *DBExecutor {
	return &DBExecutor{pool: pool}
}

// GetDB returns the underlying database connection pool
func (db *DBExecutor) GetDB() *pgxpool.Pool {
	return db.pool
}

// WithTransaction executes a function within a database transaction
// Transaction is explicitly passed to the callback function
func (db *DBExecutor) WithTransaction(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Ensure rollback on panic or error
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // Re-throw panic after rollback
		}
	}()

	// Execute function with explicit transaction
	if err := fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// WithReadOnlyTransaction executes a function within a read-only transaction
// Provides consistent reads across multiple queries
func (db *DBExecutor) WithReadOnlyTransaction(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return fmt.Errorf("begin read-only transaction: %w", err)
	}

	// Ensure rollback on panic or error
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // Re-throw panic after rollback
		}
	}()

	// Execute function with explicit transaction
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	// Commit read-only transaction (releases locks)
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit read-only transaction: %w", err)
	}

	return nil
}
