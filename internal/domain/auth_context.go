package domain

// Scope constants define the available permissions for JWT tokens.
// These scopes are included in the JWT "scopes" claim and validated
// at the handler level to control access to specific operations.
const (
	ScopePaymentsCreate       = "payments:create"
	ScopePaymentsRead         = "payments:read"
	ScopePaymentsVoid         = "payments:void"
	ScopePaymentsRefund       = "payments:refund"
	ScopePaymentMethodsRead   = "payment_methods:read"
	ScopePaymentMethodsCreate = "payment_methods:create"
	ScopeStorageTokenize      = "storage:tokenize"
	ScopeStorageDetokenize    = "storage:detokenize"
	ScopeSubscriptionsManage  = "subscriptions:manage"
	ScopeSubscriptionsRead    = "subscriptions:read"
	ScopeAll                  = "*"
)

// AllPaymentScopes returns all standard payment-related scopes.
// Used as the default when generating tokens via jwtgen CLI.
func AllPaymentScopes() []string {
	return []string{
		ScopePaymentsCreate,
		ScopePaymentsRead,
		ScopePaymentsVoid,
		ScopePaymentsRefund,
		ScopePaymentMethodsRead,
		ScopePaymentMethodsCreate,
		ScopeSubscriptionsManage,
		ScopeSubscriptionsRead,
	}
}

// HasScope checks if the provided scopes include a specific scope.
// Returns true if the scope is present or if ScopeAll ("*") is present.
func HasScope(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope || s == ScopeAll {
			return true
		}
	}
	return false
}
