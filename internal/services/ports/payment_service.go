package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// AuthorizeRequest contains parameters for authorization
type AuthorizeRequest struct {
	AgentID         string
	CustomerID      *string // Nullable for guest transactions
	Amount          string
	Currency        string
	PaymentMethodID *string // Saved payment method
	PaymentToken    *string // One-time token from EPX
	IdempotencyKey  *string
	Metadata        map[string]interface{}
}

// CaptureRequest contains parameters for capturing authorized funds
type CaptureRequest struct {
	TransactionID  string
	Amount         *string // Optional: partial capture
	IdempotencyKey *string
}

// SaleRequest contains parameters for sale (auth + capture)
type SaleRequest struct {
	AgentID         string
	CustomerID      *string
	Amount          string
	Currency        string
	PaymentMethodID *string
	PaymentToken    *string
	IdempotencyKey  *string
	Metadata        map[string]interface{}
}

// VoidRequest contains parameters for voiding a transaction
type VoidRequest struct {
	GroupID        string  // Transaction group to void
	IdempotencyKey *string
}

// RefundRequest contains parameters for refunding a transaction
type RefundRequest struct {
	GroupID        string  // Transaction group to refund
	Amount         *string // Optional: partial refund
	Reason         string
	IdempotencyKey *string
}

// ListTransactionsFilters contains filter parameters for listing transactions
type ListTransactionsFilters struct {
	AgentID         *string
	CustomerID      *string
	GroupID         *string
	Status          *string
	Type            *string
	PaymentMethodID *string
	Limit           int
	Offset          int
}

// PaymentService defines the port for payment operations
type PaymentService interface {
	// Authorize holds funds on a payment method without capturing
	Authorize(ctx context.Context, req *AuthorizeRequest) (*domain.Transaction, error)

	// Capture completes a previously authorized payment
	Capture(ctx context.Context, req *CaptureRequest) (*domain.Transaction, error)

	// Sale combines authorize and capture in one operation
	Sale(ctx context.Context, req *SaleRequest) (*domain.Transaction, error)

	// Void cancels an authorized or captured payment
	Void(ctx context.Context, req *VoidRequest) (*domain.Transaction, error)

	// Refund returns funds to the customer
	Refund(ctx context.Context, req *RefundRequest) (*domain.Transaction, error)

	// GetTransaction retrieves transaction details
	GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error)

	// GetTransactionByIdempotencyKey retrieves a transaction by idempotency key
	GetTransactionByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error)

	// ListTransactions lists transactions with filters
	ListTransactions(ctx context.Context, filters *ListTransactionsFilters) ([]*domain.Transaction, int, error)
}
