package ports

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX represents a database executor that can be either a pool or transaction
// This matches the SQLC generated interface
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...interface{}) pgx.Row
}

// TransactionManager manages database transactions
type TransactionManager interface {
	// WithTransaction executes fn within a write transaction
	// The transaction is explicitly passed to the callback function
	WithTransaction(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error

	// WithReadOnlyTransaction executes fn within a read-only transaction
	// Ensures consistent reads across multiple queries
	WithReadOnlyTransaction(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error
}

// DBPort provides access to the database and transaction management
type DBPort interface {
	GetDB() *pgxpool.Pool
	TransactionManager
}
