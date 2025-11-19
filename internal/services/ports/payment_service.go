package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// AuthorizeRequest contains parameters for authorization
type AuthorizeRequest struct {
	MerchantID      string
	CustomerID      *string // Nullable for guest transactions
	AmountCents     int64   // Amount in cents
	Currency        string
	PaymentMethodID *string // Saved payment method
	PaymentToken    *string // One-time token from EPX
	IdempotencyKey  *string
	Metadata        map[string]interface{}
}

// CaptureRequest contains parameters for capturing authorized funds
type CaptureRequest struct {
	TransactionID  string
	AmountCents    *int64 // Optional: partial capture in cents
	IdempotencyKey *string
}

// SaleRequest contains parameters for sale (auth + capture)
type SaleRequest struct {
	MerchantID      string
	CustomerID      *string
	AmountCents     int64 // Amount in cents
	Currency        string
	PaymentMethodID *string
	PaymentToken    *string
	IdempotencyKey  *string
	Metadata        map[string]interface{}
}

// VoidRequest contains parameters for voiding a transaction
type VoidRequest struct {
	ParentTransactionID string // Parent transaction ID to void (AUTH or SALE transaction)
	IdempotencyKey      *string
}

// RefundRequest contains parameters for refunding a transaction
type RefundRequest struct {
	ParentTransactionID string // Parent transaction ID to refund (AUTH or SALE transaction)
	AmountCents         *int64 // Optional: partial refund in cents
	Reason              string
	IdempotencyKey      *string
}

// ListTransactionsFilters contains filter parameters for listing transactions
type ListTransactionsFilters struct {
	MerchantID          *string
	CustomerID          *string
	SubscriptionID      *string // Filter by subscription ID
	ParentTransactionID *string // Filter by parent transaction ID
	Status              *string
	Type                *string
	PaymentMethodID     *string
	Limit               int
	Offset              int
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
