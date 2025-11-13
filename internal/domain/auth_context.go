package domain

import (
	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims represents the JWT token claims for authorization
type TokenClaims struct {
	jwt.RegisteredClaims

	// Authorization Context
	TokenType   string   `json:"token_type"`   // "merchant" | "customer" | "admin"
	MerchantIDs []string `json:"merchant_ids"` // Array of merchant UUIDs (empty for customers/admin)
	CustomerID  *string  `json:"customer_id"`  // Customer UUID (only for customer tokens)
	SessionID   *string  `json:"session_id"`   // Session ID (only for guest tokens)

	// Permissions
	Scopes []string `json:"scopes"` // ["payments:create", "payments:read", etc.]
}

// TokenType constants
const (
	TokenTypeMerchant = "merchant"
	TokenTypeCustomer = "customer"
	TokenTypeGuest    = "guest"
	TokenTypeAdmin    = "admin"
)

// Scope constants
const (
	ScopePaymentsCreate        = "payments:create"
	ScopePaymentsRead          = "payments:read"
	ScopePaymentsVoid          = "payments:void"
	ScopePaymentsRefund        = "payments:refund"
	ScopePaymentMethodsRead    = "payment_methods:read"
	ScopePaymentMethodsCreate  = "payment_methods:create"
	ScopeStorageTokenize       = "storage:tokenize"
	ScopeStorageDetokenize     = "storage:detokenize"
	ScopeAll                   = "*"
)

// HasScope checks if the token has a specific scope
func (c *TokenClaims) HasScope(scope string) bool {
	for _, s := range c.Scopes {
		if s == scope || s == ScopeAll {
			return true
		}
	}
	return false
}

// IsSingleMerchant returns true if token has exactly one merchant
func (c *TokenClaims) IsSingleMerchant() bool {
	return len(c.MerchantIDs) == 1
}

// IsMultiMerchant returns true if token has multiple merchants
func (c *TokenClaims) IsMultiMerchant() bool {
	return len(c.MerchantIDs) > 1
}

// GetSingleMerchantID returns the merchant ID if token has exactly one merchant
func (c *TokenClaims) GetSingleMerchantID() (string, bool) {
	if c.IsSingleMerchant() {
		return c.MerchantIDs[0], true
	}
	return "", false
}

// HasMerchantAccess checks if token has access to a specific merchant
func (c *TokenClaims) HasMerchantAccess(merchantID string) bool {
	for _, id := range c.MerchantIDs {
		if id == merchantID {
			return true
		}
	}
	return false
}
