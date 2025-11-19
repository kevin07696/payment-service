package database

import (
	"context"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
)

// TransactionManager provides database transaction management
// This interface abstracts the transaction handling to enable testing
type TransactionManager interface {
	// WithTx executes a function within a database transaction
	// If the function returns an error, the transaction is rolled back
	// Otherwise, the transaction is committed
	// Uses sqlc.Querier interface instead of concrete *sqlc.Queries for testability
	WithTx(ctx context.Context, fn func(sqlc.Querier) error) error
}

// Ensure PostgreSQLAdapter implements TransactionManager
var _ TransactionManager = (*PostgreSQLAdapter)(nil)
