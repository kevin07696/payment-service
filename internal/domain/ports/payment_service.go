package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/shopspring/decimal"
)

// PaymentService defines the business logic for payment operations
type PaymentService interface {
	// Authorize authorizes a payment without capturing funds
	Authorize(ctx context.Context, req ServiceAuthorizeRequest) (*PaymentResponse, error)

	// Capture captures a previously authorized payment
	Capture(ctx context.Context, req ServiceCaptureRequest) (*PaymentResponse, error)

	// Sale performs authorization and capture in one step
	Sale(ctx context.Context, req ServiceSaleRequest) (*PaymentResponse, error)

	// Void voids a previously authorized or captured transaction
	Void(ctx context.Context, req ServiceVoidRequest) (*PaymentResponse, error)

	// Refund refunds a captured transaction
	Refund(ctx context.Context, req ServiceRefundRequest) (*PaymentResponse, error)

	// GetTransaction retrieves a transaction by ID or idempotency key
	GetTransaction(ctx context.Context, transactionID string) (*models.Transaction, error)

	// GetTransactionByIdempotencyKey retrieves a transaction by idempotency key
	GetTransactionByIdempotencyKey(ctx context.Context, key string) (*models.Transaction, error)

	// ListTransactions lists transactions for a merchant or customer with pagination
	ListTransactions(ctx context.Context, req ServiceListTransactionsRequest) (*ServiceListTransactionsResponse, error)
}

// ServiceAuthorizeRequest represents a payment authorization request
type ServiceAuthorizeRequest struct {
	MerchantID      string
	CustomerID      string
	Amount          decimal.Decimal
	Currency        string
	Token           string // BRIC token from tokenization
	BillingInfo     models.BillingInfo
	IdempotencyKey  string
	Metadata        map[string]string
}

// ServiceCaptureRequest represents a capture request for an authorized transaction
type ServiceCaptureRequest struct {
	TransactionID   string
	Amount          *decimal.Decimal // Optional: partial capture
	IdempotencyKey  string
}

// ServiceSaleRequest represents a sale (authorize + capture) request
type ServiceSaleRequest struct {
	MerchantID      string
	CustomerID      string
	Amount          decimal.Decimal
	Currency        string
	Token           string // BRIC token from tokenization
	BillingInfo     models.BillingInfo
	IdempotencyKey  string
	Metadata        map[string]string
}

// ServiceVoidRequest represents a void request
type ServiceVoidRequest struct {
	TransactionID   string
	IdempotencyKey  string
}

// ServiceRefundRequest represents a refund request
type ServiceRefundRequest struct {
	TransactionID   string
	Amount          *decimal.Decimal // Optional: partial refund
	IdempotencyKey  string
	Reason          string
}

// PaymentResponse represents the response from a payment operation
type PaymentResponse struct {
	TransactionID        string
	Status               models.TransactionStatus
	Amount               decimal.Decimal
	Currency             string
	GatewayTransactionID string
	GatewayResponseCode  string
	GatewayResponseMsg   string
	AVSResponse          string // AVS verification result
	CVVResponse          string // CVV verification result
	IsApproved           bool
	IsDeclined           bool
	IsRetriable          bool
	ErrorCategory        string
	CreatedAt            string
	UpdatedAt            string
}

// ServiceListTransactionsRequest represents a request to list transactions
type ServiceListTransactionsRequest struct {
	MerchantID string
	CustomerID string // Optional: filter by customer
	Limit      int32  // Maximum number of transactions to return
	Offset     int32  // Number of transactions to skip
}

// ServiceListTransactionsResponse represents the response from listing transactions
type ServiceListTransactionsResponse struct {
	Transactions []*models.Transaction
	TotalCount   int32
}
