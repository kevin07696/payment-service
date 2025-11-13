package authorization

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/kevin07696/payment-service/internal/domain"
)

// MerchantResolver resolves merchant_id based on token context and request parameters
type MerchantResolver struct{}

// NewMerchantResolver creates a new merchant resolver
func NewMerchantResolver() *MerchantResolver {
	return &MerchantResolver{}
}

// Resolve determines the merchant_id to use for an operation based on token and request
func (r *MerchantResolver) Resolve(token *domain.TokenClaims, requestedMerchantID string) (string, error) {
	switch token.TokenType {
	case domain.TokenTypeMerchant:
		return r.resolveMerchantToken(token, requestedMerchantID)
	case domain.TokenTypeCustomer:
		return "", connect.NewError(connect.CodePermissionDenied,
			errors.New("customers cannot create payments"))
	case domain.TokenTypeGuest:
		return r.resolveGuestToken(token)
	case domain.TokenTypeAdmin:
		return r.resolveAdminToken(requestedMerchantID)
	default:
		return "", connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("invalid token type: %s", token.TokenType))
	}
}

// resolveMerchantToken handles merchant token resolution
func (r *MerchantResolver) resolveMerchantToken(token *domain.TokenClaims, requested string) (string, error) {
	// No merchants in token
	if len(token.MerchantIDs) == 0 {
		return "", connect.NewError(connect.CodeUnauthenticated,
			errors.New("token has no merchant access"))
	}

	// Single merchant (POS cashier)
	if len(token.MerchantIDs) == 1 {
		// Use the only merchant in token (ignore request)
		return token.MerchantIDs[0], nil
	}

	// Multiple merchants (operator)
	if requested == "" {
		return "", connect.NewError(connect.CodeInvalidArgument,
			errors.New("merchant_id required: token has multiple merchants"))
	}

	// Validate requested merchant is in token's list
	if !token.HasMerchantAccess(requested) {
		return "", connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("merchant_id '%s' not in allowed list", requested))
	}

	return requested, nil
}

// resolveGuestToken handles guest token resolution
func (r *MerchantResolver) resolveGuestToken(token *domain.TokenClaims) (string, error) {
	// Guest tokens must have exactly one merchant
	if len(token.MerchantIDs) != 1 {
		return "", connect.NewError(connect.CodeUnauthenticated,
			errors.New("guest token must have exactly one merchant_id"))
	}

	return token.MerchantIDs[0], nil
}

// resolveAdminToken handles admin token resolution
func (r *MerchantResolver) resolveAdminToken(requested string) (string, error) {
	if requested == "" {
		return "", connect.NewError(connect.CodeInvalidArgument,
			errors.New("merchant_id required for admin"))
	}
	return requested, nil
}

// ValidateScope checks if token has required scope for an operation
func (r *MerchantResolver) ValidateScope(token *domain.TokenClaims, requiredScope string) error {
	if !token.HasScope(requiredScope) {
		return connect.NewError(connect.CodePermissionDenied,
			fmt.Errorf("missing required scope: %s", requiredScope))
	}
	return nil
}

// ValidateCustomerAccess validates that customer can only access their own resources
func (r *MerchantResolver) ValidateCustomerAccess(token *domain.TokenClaims, customerID string) error {
	if token.TokenType != domain.TokenTypeCustomer {
		return nil // Not a customer token, skip validation
	}

	if token.CustomerID == nil || *token.CustomerID == "" {
		return connect.NewError(connect.CodeUnauthenticated,
			errors.New("customer token must have customer_id"))
	}

	if customerID != "" && customerID != *token.CustomerID {
		return connect.NewError(connect.CodePermissionDenied,
			errors.New("customers can only access their own resources"))
	}

	return nil
}
