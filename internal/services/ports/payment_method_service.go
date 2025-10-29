package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// SavePaymentMethodRequest contains parameters for saving a payment method
type SavePaymentMethodRequest struct {
	AgentID         string
	CustomerID      string
	PaymentToken    string // EPX token (AUTH_GUID)
	PaymentType     domain.PaymentMethodType
	LastFour        string
	CardBrand       *string
	CardExpMonth    *int
	CardExpYear     *int
	BankName        *string
	AccountType     *string
	IsDefault       bool
	IdempotencyKey  *string
}

// VerifyACHAccountRequest contains parameters for ACH verification
type VerifyACHAccountRequest struct {
	PaymentMethodID string
	AgentID         string
	CustomerID      string
}

// PaymentMethodService defines the port for payment method operations
type PaymentMethodService interface {
	// SavePaymentMethod tokenizes and saves a payment method
	SavePaymentMethod(ctx context.Context, req *SavePaymentMethodRequest) (*domain.PaymentMethod, error)

	// GetPaymentMethod retrieves a specific payment method
	GetPaymentMethod(ctx context.Context, paymentMethodID string) (*domain.PaymentMethod, error)

	// ListPaymentMethods lists all payment methods for a customer
	ListPaymentMethods(ctx context.Context, agentID, customerID string) ([]*domain.PaymentMethod, error)

	// UpdatePaymentMethodStatus updates the active status of a payment method
	UpdatePaymentMethodStatus(ctx context.Context, paymentMethodID, agentID, customerID string, isActive bool) (*domain.PaymentMethod, error)

	// DeletePaymentMethod soft deletes a payment method (sets deleted_at)
	DeletePaymentMethod(ctx context.Context, paymentMethodID string) error

	// SetDefaultPaymentMethod marks a payment method as default
	SetDefaultPaymentMethod(ctx context.Context, paymentMethodID, agentID, customerID string) (*domain.PaymentMethod, error)

	// VerifyACHAccount sends pre-note for ACH verification
	VerifyACHAccount(ctx context.Context, req *VerifyACHAccountRequest) error
}
