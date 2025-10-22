package ports

import (
	"context"
	"time"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/shopspring/decimal"
)

// AuthorizeRequest represents a request to authorize a payment
type AuthorizeRequest struct {
	Amount      decimal.Decimal
	Currency    string
	Token       string // BRIC token from Browser Post
	BillingInfo models.BillingInfo
	Capture     bool   // If true, perform auth+capture (sale)
	IdempotencyKey string
	Metadata    map[string]string
}

// CaptureRequest represents a request to capture an authorized payment
type CaptureRequest struct {
	TransactionID string
	Amount        decimal.Decimal // Can be partial capture
}

// RefundRequest represents a request to refund a payment
type RefundRequest struct {
	TransactionID string
	Amount        decimal.Decimal // Can be partial refund
	Reason        string
}

// PaymentResult represents the result of a payment operation
type PaymentResult struct {
	TransactionID        string
	GatewayTransactionID string
	Amount               decimal.Decimal
	Status               models.TransactionStatus
	ResponseCode         string
	Message              string
	AuthCode             string
	AVSResponse          string // AVS verification result (Y=match, N=no match, etc.)
	CVVResponse          string // CVV verification result
	Timestamp            time.Time
}

// VerifyAccountRequest represents a request to verify account details
type VerifyAccountRequest struct {
	Token       string
	BillingInfo models.BillingInfo
}

// VerificationResult represents the result of account verification
type VerificationResult struct {
	Verified     bool
	ResponseCode string
	Message      string
}

// CreditCardGateway defines the interface for credit card payment operations
type CreditCardGateway interface {
	// Authorize authorizes a payment without capturing it
	Authorize(ctx context.Context, req *AuthorizeRequest) (*PaymentResult, error)

	// Capture captures a previously authorized payment
	Capture(ctx context.Context, req *CaptureRequest) (*PaymentResult, error)

	// Void voids an authorized but not yet captured payment
	Void(ctx context.Context, transactionID string) (*PaymentResult, error)

	// Refund refunds a captured payment
	Refund(ctx context.Context, req *RefundRequest) (*PaymentResult, error)

	// VerifyAccount verifies card details without charging
	VerifyAccount(ctx context.Context, req *VerifyAccountRequest) (*VerificationResult, error)
}
