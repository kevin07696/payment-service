package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// VerifyACHAccountRequest contains parameters for ACH verification
type VerifyACHAccountRequest struct {
	PaymentMethodID string
	MerchantID      string
	CustomerID      string
}

// StoreACHAccountRequest contains parameters for storing an ACH account
type StoreACHAccountRequest struct {
	MerchantID     string
	CustomerID     string
	RoutingNumber  string
	AccountNumber  string
	AccountType    string // "CHECKING" or "SAVINGS"
	NameOnAccount  string
	FirstName      string
	LastName       string
	Address        string
	City           string
	State          string
	ZipCode        string
	IdempotencyKey string // UUID for transaction idempotency
}

// PaymentMethodService defines the port for payment method operations
type PaymentMethodService interface {
	// GetPaymentMethod retrieves a specific payment method
	GetPaymentMethod(ctx context.Context, paymentMethodID string) (*domain.PaymentMethod, error)

	// ListPaymentMethods lists all payment methods for a customer
	ListPaymentMethods(ctx context.Context, merchantID, customerID string) ([]*domain.PaymentMethod, error)

	// UpdatePaymentMethodStatus updates the active status of a payment method
	UpdatePaymentMethodStatus(ctx context.Context, paymentMethodID, merchantID, customerID string, isActive bool) (*domain.PaymentMethod, error)

	// DeletePaymentMethod soft deletes a payment method (sets deleted_at)
	DeletePaymentMethod(ctx context.Context, paymentMethodID string) error

	// SetDefaultPaymentMethod marks a payment method as default
	SetDefaultPaymentMethod(ctx context.Context, paymentMethodID, merchantID, customerID string) (*domain.PaymentMethod, error)

	// StoreACHAccount stores ACH account with pre-note verification
	// Sends Pre-Note Debit (CKC0/CKS0) to EPX, stores GUID/BRIC with status=pending_verification
	StoreACHAccount(ctx context.Context, req *StoreACHAccountRequest) (*domain.PaymentMethod, error)

	// VerifyACHAccount sends pre-note for ACH verification
	VerifyACHAccount(ctx context.Context, req *VerifyACHAccountRequest) error
}
