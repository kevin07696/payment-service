package authorization

import (
	"context"

	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	"go.uber.org/zap"
)

// ServiceMerchantAccessChecker checks if a service has access to a merchant
type ServiceMerchantAccessChecker interface {
	CheckServiceMerchantAccess(ctx context.Context, serviceID, merchantID string) (bool, error)
}

// MerchantAuthorizationService handles merchant-level authorization logic
type MerchantAuthorizationService struct {
	logger        *zap.Logger
	accessChecker ServiceMerchantAccessChecker
}

// NewMerchantAuthorizationService creates a new merchant authorization service
func NewMerchantAuthorizationService(logger *zap.Logger, accessChecker ServiceMerchantAccessChecker) *MerchantAuthorizationService {
	return &MerchantAuthorizationService{
		logger:        logger,
		accessChecker: accessChecker,
	}
}

// ResolveMerchantID resolves the merchant_id from request and validates access
// Authentication Model:
// - Token identifies the SERVICE (no merchant_id in token)
// - Request specifies the MERCHANT (merchant_id in request body)
// - Database validates service has access to the merchant
//
// This method:
// - In no-auth mode (development/testing), uses the requested merchant_id directly
// - In JWT auth mode, validates service has access to the requested merchant via database
// - API key auth (future) would have merchant_id bound to the key
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

	// For API key auth (future): merchant_id would be bound to the key
	// This branch handles that case if we add API key auth later
	if authInfo.MerchantID != "" {
		// API key is bound to a specific merchant - verify request matches
		if requestedMerchantID != "" && requestedMerchantID != authInfo.MerchantID {
			s.logger.Warn("Merchant ID mismatch with API key",
				zap.String("requested", requestedMerchantID),
				zap.String("api_key_merchant", authInfo.MerchantID))
			return "", domain.ErrAuthMerchantMismatch.
				WithDetail("requested", requestedMerchantID).
				WithDetail("authenticated", authInfo.MerchantID)
		}
		s.logger.Debug("Resolved merchant ID from API key",
			zap.String("merchant_id", authInfo.MerchantID))
		return authInfo.MerchantID, nil
	}

	// JWT auth: Service token, merchant comes from request
	// Validate service has access to the requested merchant
	if authInfo.Type == auth.AuthTypeJWT && requestedMerchantID != "" {
		// Validate service has access to the requested merchant
		if s.accessChecker != nil {
			hasAccess, err := s.accessChecker.CheckServiceMerchantAccess(ctx, authInfo.ServiceID, requestedMerchantID)
			if err != nil {
				s.logger.Error("Failed to check service-merchant access",
					zap.String("service_id", authInfo.ServiceID),
					zap.String("merchant_id", requestedMerchantID),
					zap.Error(err))
				return "", domain.ErrAuthAccessDenied.WithDetail("reason", "access check failed")
			}
			if !hasAccess {
				s.logger.Warn("Service does not have access to merchant",
					zap.String("service_id", authInfo.ServiceID),
					zap.String("merchant_id", requestedMerchantID))
				return "", domain.ErrAuthAccessDenied.
					WithDetail("service_id", authInfo.ServiceID).
					WithDetail("merchant_id", requestedMerchantID)
			}
		}
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

	// For service auth, verify the service has access to the transaction's merchant
	if authInfo.Type == auth.AuthTypeJWT {
		if s.accessChecker != nil {
			hasAccess, err := s.accessChecker.CheckServiceMerchantAccess(ctx, authInfo.ServiceID, tx.MerchantID)
			if err != nil {
				s.logger.Error("Failed to check service-merchant access for transaction",
					zap.String("service_id", authInfo.ServiceID),
					zap.String("merchant_id", tx.MerchantID),
					zap.String("transaction_id", tx.ID),
					zap.Error(err))
				return domain.ErrAuthAccessDenied.WithDetail("reason", "access check failed")
			}
			if !hasAccess {
				s.logger.Warn("Service does not have access to transaction's merchant",
					zap.String("service_id", authInfo.ServiceID),
					zap.String("merchant_id", tx.MerchantID),
					zap.String("transaction_id", tx.ID))
				return domain.ErrAuthAccessDenied.
					WithDetail("resource", "transaction").
					WithDetail("transaction_id", tx.ID).
					WithDetail("service_id", authInfo.ServiceID).
					WithDetail("merchant_id", tx.MerchantID)
			}
		}
		s.logger.Debug("Service access to transaction validated",
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

	// For service auth, verify the service has access to the customer's merchant
	if authInfo.Type == auth.AuthTypeJWT {
		if s.accessChecker != nil {
			hasAccess, err := s.accessChecker.CheckServiceMerchantAccess(ctx, authInfo.ServiceID, merchantID)
			if err != nil {
				s.logger.Error("Failed to check service-merchant access for customer",
					zap.String("service_id", authInfo.ServiceID),
					zap.String("merchant_id", merchantID),
					zap.String("customer_id", customerID),
					zap.Error(err))
				return domain.ErrAuthAccessDenied.WithDetail("reason", "access check failed")
			}
			if !hasAccess {
				s.logger.Warn("Service does not have access to customer's merchant",
					zap.String("service_id", authInfo.ServiceID),
					zap.String("merchant_id", merchantID),
					zap.String("customer_id", customerID))
				return domain.ErrAuthAccessDenied.
					WithDetail("resource", "customer").
					WithDetail("customer_id", customerID).
					WithDetail("service_id", authInfo.ServiceID).
					WithDetail("merchant_id", merchantID)
			}
		}
		s.logger.Debug("Service access to customer validated",
			zap.String("service_id", authInfo.ServiceID),
			zap.String("customer_id", customerID))
		return nil
	}

	return domain.ErrAuthInvalid.WithDetail("reason", "unknown auth type")
}

// Ensure MerchantAuthorizationService implements ports.MerchantAuthorizationService
var _ ports.MerchantAuthorizationService = (*MerchantAuthorizationService)(nil)

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

	// For service auth, verify the service has access to the payment method's merchant
	if authInfo.Type == auth.AuthTypeJWT {
		if s.accessChecker != nil {
			hasAccess, err := s.accessChecker.CheckServiceMerchantAccess(ctx, authInfo.ServiceID, merchantID)
			if err != nil {
				s.logger.Error("Failed to check service-merchant access for payment method",
					zap.String("service_id", authInfo.ServiceID),
					zap.String("merchant_id", merchantID),
					zap.String("payment_method_id", paymentMethodID),
					zap.Error(err))
				return domain.ErrAuthAccessDenied.WithDetail("reason", "access check failed")
			}
			if !hasAccess {
				s.logger.Warn("Service does not have access to payment method's merchant",
					zap.String("service_id", authInfo.ServiceID),
					zap.String("merchant_id", merchantID),
					zap.String("payment_method_id", paymentMethodID))
				return domain.ErrAuthAccessDenied.
					WithDetail("resource", "payment_method").
					WithDetail("payment_method_id", paymentMethodID).
					WithDetail("service_id", authInfo.ServiceID).
					WithDetail("merchant_id", merchantID)
			}
		}
		s.logger.Debug("Service access to payment method validated",
			zap.String("service_id", authInfo.ServiceID),
			zap.String("payment_method_id", paymentMethodID))
		return nil
	}

	return domain.ErrAuthInvalid.WithDetail("reason", "unknown auth type")
}
