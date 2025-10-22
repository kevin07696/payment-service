package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/shopspring/decimal"
)

// ACHPaymentRequest represents a request to process an ACH payment
type ACHPaymentRequest struct {
	Amount        decimal.Decimal
	Currency      string
	AccountType   models.ACHAccountType
	RoutingNumber string
	AccountNumber string
	SECCode       models.SECCode
	BillingInfo   models.BillingInfo
	ReceiverName  string // Required for CCD transactions
	IdempotencyKey string
	Metadata      map[string]string
}

// BankAccountVerificationRequest represents a request to verify a bank account
type BankAccountVerificationRequest struct {
	RoutingNumber string
	AccountNumber string
	AccountType   models.ACHAccountType
}

// ACHGateway defines the interface for ACH payment operations
type ACHGateway interface {
	// ProcessPayment processes an ACH payment
	ProcessPayment(ctx context.Context, req *ACHPaymentRequest) (*PaymentResult, error)

	// RefundPayment refunds an ACH payment
	RefundPayment(ctx context.Context, transactionID string, amount decimal.Decimal) (*PaymentResult, error)

	// VerifyBankAccount verifies bank account details
	VerifyBankAccount(ctx context.Context, req *BankAccountVerificationRequest) (*VerificationResult, error)
}
