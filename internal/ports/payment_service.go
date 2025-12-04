package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// AuthorizeRequest contains parameters for authorization.
type AuthorizeRequest struct {
	MerchantID      string
	CustomerID      *string
	OrderID         *string // Merchant's external order/invoice ID
	AmountCents     int64
	Currency        string
	PaymentMethodID *string
	PaymentToken    *string
	IdempotencyKey  *string
	Metadata        map[string]interface{}
}

// CaptureRequest contains parameters for capturing authorized funds.
type CaptureRequest struct {
	TransactionID  string
	AmountCents    *int64
	IdempotencyKey *string
}

// SaleRequest contains parameters for sale (auth + capture).
type SaleRequest struct {
	MerchantID      string
	CustomerID      *string
	OrderID         *string // Merchant's external order/invoice ID
	AmountCents     int64
	Currency        string
	PaymentMethodID *string
	PaymentToken    *string
	IdempotencyKey  *string
	Metadata        map[string]interface{}
	StdEntryClass   *string // ACH only: WEB, TEL, PPD, CCD (ignored for credit cards)
}

// VoidRequest contains parameters for voiding a transaction.
type VoidRequest struct {
	ParentTransactionID string
	IdempotencyKey      *string
}

// RefundRequest contains parameters for refunding a transaction.
type RefundRequest struct {
	ParentTransactionID string
	AmountCents         *int64
	Reason              string
	IdempotencyKey      *string
}

// ListTransactionsFilters contains filter parameters for listing transactions.
type ListTransactionsFilters struct {
	MerchantID          *string
	CustomerID          *string
	OrderID             *string // Filter by merchant's external order/invoice ID
	SubscriptionID      *string
	ParentTransactionID *string
	Status              *string
	Type                *string
	PaymentMethodID     *string
	Limit               int
	Offset              int
}

// PaymentService defines the port for payment operations.
type PaymentService interface {
	Authorize(ctx context.Context, req *AuthorizeRequest) (*domain.Transaction, error)
	Capture(ctx context.Context, req *CaptureRequest) (*domain.Transaction, error)
	Sale(ctx context.Context, req *SaleRequest) (*domain.Transaction, error)
	Void(ctx context.Context, req *VoidRequest) (*domain.Transaction, error)
	Refund(ctx context.Context, req *RefundRequest) (*domain.Transaction, error)
	GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error)
	GetTransactionByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error)
	ListTransactions(ctx context.Context, filters *ListTransactionsFilters) ([]*domain.Transaction, int, error)
}
