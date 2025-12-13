package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// TransactionQueryService provides read-only access to transactions.
// Separated from PaymentService for:
// 1. Interface Segregation - handlers only depend on what they use
// 2. Testability - no EPX mocking needed for query tests
// 3. Scalability - can add caching layer independently
type TransactionQueryService interface {
	// GetTransaction retrieves a single transaction by ID.
	GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error)

	// GetTransactionByIdempotencyKey retrieves a transaction by its idempotency key.
	GetTransactionByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error)

	// ListTransactions retrieves transactions matching the given filters.
	// Returns the transactions, total count, and any error.
	ListTransactions(ctx context.Context, filters *ListTransactionsFilters) ([]*domain.Transaction, int, error)

	// GetTransactionsByGroup retrieves all transactions in a group (parent + children).
	GetTransactionsByGroup(ctx context.Context, parentTransactionID string) ([]*domain.Transaction, error)
}
