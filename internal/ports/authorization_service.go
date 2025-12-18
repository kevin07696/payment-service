package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// MerchantAuthorizationService handles merchant-level authorization logic
// This interface abstracts authorization operations for dependency injection
type MerchantAuthorizationService interface {
	// ResolveMerchantID resolves the merchant_id from request and validates access
	// - In no-auth mode (development/testing), uses the requested merchant_id directly
	// - In JWT auth mode, validates service has access to the requested merchant via database
	ResolveMerchantID(ctx context.Context, requestedMerchantID string) (string, error)

	// ValidateTransactionAccess validates that the auth context has access to a transaction
	ValidateTransactionAccess(ctx context.Context, tx *domain.Transaction) error

	// ValidateCustomerAccess validates that the auth context has access to a customer's data
	ValidateCustomerAccess(ctx context.Context, merchantID, customerID string) error

	// ValidatePaymentMethodAccess validates that the auth context has access to a payment method
	ValidatePaymentMethodAccess(ctx context.Context, merchantID, paymentMethodID string) error
}
