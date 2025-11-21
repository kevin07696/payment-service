package authorization

import (
	"context"

	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/domain"
	"go.uber.org/zap"
)

// MerchantAuthorizationService handles merchant-level authorization logic
type MerchantAuthorizationService struct {
	logger *zap.Logger
}

// NewMerchantAuthorizationService creates a new merchant authorization service
func NewMerchantAuthorizationService(logger *zap.Logger) *MerchantAuthorizationService {
	return &MerchantAuthorizationService{
		logger: logger,
	}
}

// ResolveMerchantID resolves the merchant_id from auth context and request
// This method centralizes the logic for determining which merchant a request is for:
// - In no-auth mode (development/testing), uses the requested merchant_id
// - In JWT/API key auth mode, uses the merchant_id from auth context
// - Validates that requested merchant_id matches authenticated merchant_id if both present
func (s *MerchantAuthorizationService) ResolveMerchantID(ctx context.Context, requestedMerchantID string) (string, error) {
	// Get auth info from context
	authInfo := auth.GetAuthInfo(ctx)

	// If no auth (development/testing mode)
	if authInfo.Type == auth.AuthTypeNone {
		if requestedMerchantID == "" {
			return "", domain.ErrMerchantRequired
		}
		s.logger.Debug("Resolved merchant ID in no-auth mode",
			zap.String("merchant_id", requestedMerchantID))
		return requestedMerchantID, nil
	}

	// If merchant ID is in context (API key auth or JWT with merchant_id)
	if authInfo.MerchantID != "" {
		// If a specific merchant was requested, verify it matches
		if requestedMerchantID != "" && requestedMerchantID != authInfo.MerchantID {
			s.logger.Warn("Merchant ID mismatch",
				zap.String("requested", requestedMerchantID),
				zap.String("authenticated", authInfo.MerchantID))
			return "", domain.ErrAuthMerchantMismatch.
				WithDetail("requested", requestedMerchantID).
				WithDetail("authenticated", authInfo.MerchantID)
		}
		s.logger.Debug("Resolved merchant ID from auth context",
			zap.String("merchant_id", authInfo.MerchantID),
			zap.String("auth_type", string(authInfo.Type)))
		return authInfo.MerchantID, nil
	}

	// For service auth (JWT without merchant_id claim), use the requested merchant ID
	// This allows services to act on behalf of multiple merchants
	if authInfo.Type == auth.AuthTypeJWT && requestedMerchantID != "" {
		s.logger.Debug("Resolved merchant ID for service auth",
			zap.String("merchant_id", requestedMerchantID),
			zap.String("service_id", authInfo.ServiceID))
		return requestedMerchantID, nil
	}

	return "", domain.ErrMerchantRequired.WithDetail("reason", "no merchant in auth context and no merchant requested")
}

// ValidateTransactionAccess validates that the auth context has access to a transaction
// This ensures that:
// - In no-auth mode, all access is allowed (for development/testing)
// - In merchant auth mode, only the owning merchant can access the transaction
// - In service auth mode, the service must be authorized for the transaction's merchant
func (s *MerchantAuthorizationService) ValidateTransactionAccess(ctx context.Context, tx *domain.Transaction) error {
	// Get auth info from context
	authInfo := auth.GetAuthInfo(ctx)

	// If no auth (development/testing mode), allow access
	if authInfo.Type == auth.AuthTypeNone {
		return nil
	}

	// If merchant auth, verify it's their transaction
	if authInfo.MerchantID != "" {
		if tx.MerchantID != authInfo.MerchantID {
			s.logger.Warn("Transaction access denied - merchant mismatch",
				zap.String("transaction_merchant", tx.MerchantID),
				zap.String("auth_merchant", authInfo.MerchantID),
				zap.String("transaction_id", tx.ID))
			return domain.ErrAuthAccessDenied.
				WithDetail("resource", "transaction").
				WithDetail("transaction_id", tx.ID).
				WithDetail("transaction_merchant", tx.MerchantID).
				WithDetail("auth_merchant", authInfo.MerchantID)
		}
		return nil
	}

	// For service auth, we'd need to verify the service has access to the merchant
	// This would require a database lookup, which we'll add if needed
	if authInfo.Type == auth.AuthTypeJWT {
		// For now, allow service access (actual authorization happens at interceptor level)
		s.logger.Debug("Allowing service access to transaction",
			zap.String("service_id", authInfo.ServiceID),
			zap.String("transaction_id", tx.ID))
		return nil
	}

	return domain.ErrAuthInvalid.WithDetail("reason", "unknown auth type")
}

// ValidateCustomerAccess validates that the auth context has access to a customer's data
// This ensures that only the owning merchant (or authorized services) can access customer data
func (s *MerchantAuthorizationService) ValidateCustomerAccess(ctx context.Context, merchantID, customerID string) error {
	// Get auth info from context
	authInfo := auth.GetAuthInfo(ctx)

	// If no auth (development/testing mode), allow access
	if authInfo.Type == auth.AuthTypeNone {
		return nil
	}

	// If merchant auth, verify it's their customer
	if authInfo.MerchantID != "" {
		if merchantID != authInfo.MerchantID {
			s.logger.Warn("Customer access denied - merchant mismatch",
				zap.String("customer_merchant", merchantID),
				zap.String("auth_merchant", authInfo.MerchantID),
				zap.String("customer_id", customerID))
			return domain.ErrAuthAccessDenied.
				WithDetail("resource", "customer").
				WithDetail("customer_id", customerID).
				WithDetail("customer_merchant", merchantID).
				WithDetail("auth_merchant", authInfo.MerchantID)
		}
		return nil
	}

	// For service auth, allow access (authorization happens at interceptor level)
	if authInfo.Type == auth.AuthTypeJWT {
		s.logger.Debug("Allowing service access to customer",
			zap.String("service_id", authInfo.ServiceID),
			zap.String("customer_id", customerID))
		return nil
	}

	return domain.ErrAuthInvalid.WithDetail("reason", "unknown auth type")
}

// ValidatePaymentMethodAccess validates that the auth context has access to a payment method
// This ensures that only the owning merchant (or authorized services) can access payment methods
func (s *MerchantAuthorizationService) ValidatePaymentMethodAccess(ctx context.Context, merchantID, paymentMethodID string) error {
	// Get auth info from context
	authInfo := auth.GetAuthInfo(ctx)

	// If no auth (development/testing mode), allow access
	if authInfo.Type == auth.AuthTypeNone {
		return nil
	}

	// If merchant auth, verify it's their payment method
	if authInfo.MerchantID != "" {
		if merchantID != authInfo.MerchantID {
			s.logger.Warn("Payment method access denied - merchant mismatch",
				zap.String("payment_method_merchant", merchantID),
				zap.String("auth_merchant", authInfo.MerchantID),
				zap.String("payment_method_id", paymentMethodID))
			return domain.ErrAuthAccessDenied.
				WithDetail("resource", "payment_method").
				WithDetail("payment_method_id", paymentMethodID).
				WithDetail("payment_method_merchant", merchantID).
				WithDetail("auth_merchant", authInfo.MerchantID)
		}
		return nil
	}

	// For service auth, allow access (authorization happens at interceptor level)
	if authInfo.Type == auth.AuthTypeJWT {
		s.logger.Debug("Allowing service access to payment method",
			zap.String("service_id", authInfo.ServiceID),
			zap.String("payment_method_id", paymentMethodID))
		return nil
	}

	return domain.ErrAuthInvalid.WithDetail("reason", "unknown auth type")
}
