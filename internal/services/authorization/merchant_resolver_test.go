package authorization

import (
	"testing"

	"connectrpc.com/connect"

	"github.com/kevin07696/payment-service/internal/domain"
)

func TestMerchantResolver_Resolve_MerchantToken_SingleMerchant(t *testing.T) {
	resolver := NewMerchantResolver()

	tests := []struct {
		name              string
		token             *domain.TokenClaims
		requestedMerchant string
		expectMerchant    string
		expectError       bool
		errorCode         connect.Code
	}{
		{
			name: "single merchant - no merchant_id in request",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeMerchant,
				MerchantIDs: []string{"merchant_123"},
			},
			requestedMerchant: "",
			expectMerchant:    "merchant_123",
			expectError:       false,
		},
		{
			name: "single merchant - merchant_id in request is ignored",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeMerchant,
				MerchantIDs: []string{"merchant_123"},
			},
			requestedMerchant: "different_merchant",
			expectMerchant:    "merchant_123",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merchantID, err := resolver.Resolve(tt.token, tt.requestedMerchant)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if connectErr, ok := err.(*connect.Error); ok {
					if connectErr.Code() != tt.errorCode {
						t.Errorf("expected error code %v, got %v", tt.errorCode, connectErr.Code())
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if merchantID != tt.expectMerchant {
					t.Errorf("expected merchant_id %s, got %s", tt.expectMerchant, merchantID)
				}
			}
		})
	}
}

func TestMerchantResolver_Resolve_MerchantToken_MultiMerchant(t *testing.T) {
	resolver := NewMerchantResolver()

	tests := []struct {
		name              string
		token             *domain.TokenClaims
		requestedMerchant string
		expectMerchant    string
		expectError       bool
		errorCode         connect.Code
	}{
		{
			name: "multi merchant - missing merchant_id in request",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeMerchant,
				MerchantIDs: []string{"merchant_1", "merchant_2", "merchant_3"},
			},
			requestedMerchant: "",
			expectError:       true,
			errorCode:         connect.CodeInvalidArgument,
		},
		{
			name: "multi merchant - valid merchant_id in request",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeMerchant,
				MerchantIDs: []string{"merchant_1", "merchant_2", "merchant_3"},
			},
			requestedMerchant: "merchant_2",
			expectMerchant:    "merchant_2",
			expectError:       false,
		},
		{
			name: "multi merchant - unauthorized merchant_id in request",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeMerchant,
				MerchantIDs: []string{"merchant_1", "merchant_2", "merchant_3"},
			},
			requestedMerchant: "merchant_999",
			expectError:       true,
			errorCode:         connect.CodePermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merchantID, err := resolver.Resolve(tt.token, tt.requestedMerchant)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if connectErr, ok := err.(*connect.Error); ok {
					if connectErr.Code() != tt.errorCode {
						t.Errorf("expected error code %v, got %v", tt.errorCode, connectErr.Code())
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if merchantID != tt.expectMerchant {
					t.Errorf("expected merchant_id %s, got %s", tt.expectMerchant, merchantID)
				}
			}
		})
	}
}

func TestMerchantResolver_Resolve_MerchantToken_NoMerchants(t *testing.T) {
	resolver := NewMerchantResolver()

	token := &domain.TokenClaims{
		TokenType:   domain.TokenTypeMerchant,
		MerchantIDs: []string{},
	}

	_, err := resolver.Resolve(token, "merchant_123")
	if err == nil {
		t.Error("expected error for token with no merchants")
		return
	}

	if connectErr, ok := err.(*connect.Error); ok {
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Errorf("expected CodeUnauthenticated, got %v", connectErr.Code())
		}
	}
}

func TestMerchantResolver_Resolve_CustomerToken(t *testing.T) {
	resolver := NewMerchantResolver()

	customerID := "customer_123"
	token := &domain.TokenClaims{
		TokenType:   domain.TokenTypeCustomer,
		MerchantIDs: []string{},
		CustomerID:  &customerID,
	}

	_, err := resolver.Resolve(token, "merchant_123")
	if err == nil {
		t.Error("expected error for customer token")
		return
	}

	if connectErr, ok := err.(*connect.Error); ok {
		if connectErr.Code() != connect.CodePermissionDenied {
			t.Errorf("expected CodePermissionDenied, got %v", connectErr.Code())
		}
	}
}

func TestMerchantResolver_Resolve_GuestToken(t *testing.T) {
	resolver := NewMerchantResolver()

	tests := []struct {
		name              string
		token             *domain.TokenClaims
		requestedMerchant string
		expectMerchant    string
		expectError       bool
		errorCode         connect.Code
	}{
		{
			name: "guest token - valid single merchant",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeGuest,
				MerchantIDs: []string{"merchant_123"},
			},
			requestedMerchant: "",
			expectMerchant:    "merchant_123",
			expectError:       false,
		},
		{
			name: "guest token - no merchants",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeGuest,
				MerchantIDs: []string{},
			},
			requestedMerchant: "",
			expectError:       true,
			errorCode:         connect.CodeUnauthenticated,
		},
		{
			name: "guest token - multiple merchants",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeGuest,
				MerchantIDs: []string{"merchant_1", "merchant_2"},
			},
			requestedMerchant: "",
			expectError:       true,
			errorCode:         connect.CodeUnauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merchantID, err := resolver.Resolve(tt.token, tt.requestedMerchant)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if connectErr, ok := err.(*connect.Error); ok {
					if connectErr.Code() != tt.errorCode {
						t.Errorf("expected error code %v, got %v", tt.errorCode, connectErr.Code())
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if merchantID != tt.expectMerchant {
					t.Errorf("expected merchant_id %s, got %s", tt.expectMerchant, merchantID)
				}
			}
		})
	}
}

func TestMerchantResolver_Resolve_AdminToken(t *testing.T) {
	resolver := NewMerchantResolver()

	tests := []struct {
		name              string
		token             *domain.TokenClaims
		requestedMerchant string
		expectMerchant    string
		expectError       bool
		errorCode         connect.Code
	}{
		{
			name: "admin token - missing merchant_id in request",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeAdmin,
				MerchantIDs: []string{},
			},
			requestedMerchant: "",
			expectError:       true,
			errorCode:         connect.CodeInvalidArgument,
		},
		{
			name: "admin token - valid merchant_id in request",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeAdmin,
				MerchantIDs: []string{},
			},
			requestedMerchant: "any_merchant_999",
			expectMerchant:    "any_merchant_999",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merchantID, err := resolver.Resolve(tt.token, tt.requestedMerchant)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if connectErr, ok := err.(*connect.Error); ok {
					if connectErr.Code() != tt.errorCode {
						t.Errorf("expected error code %v, got %v", tt.errorCode, connectErr.Code())
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if merchantID != tt.expectMerchant {
					t.Errorf("expected merchant_id %s, got %s", tt.expectMerchant, merchantID)
				}
			}
		})
	}
}

func TestMerchantResolver_ValidateScope(t *testing.T) {
	resolver := NewMerchantResolver()

	tests := []struct {
		name          string
		token         *domain.TokenClaims
		requiredScope string
		expectError   bool
	}{
		{
			name: "has required scope",
			token: &domain.TokenClaims{
				Scopes: []string{domain.ScopePaymentsCreate, domain.ScopePaymentsRead},
			},
			requiredScope: domain.ScopePaymentsCreate,
			expectError:   false,
		},
		{
			name: "missing required scope",
			token: &domain.TokenClaims{
				Scopes: []string{domain.ScopePaymentsRead},
			},
			requiredScope: domain.ScopePaymentsCreate,
			expectError:   true,
		},
		{
			name: "has wildcard scope",
			token: &domain.TokenClaims{
				Scopes: []string{domain.ScopeAll},
			},
			requiredScope: domain.ScopePaymentsCreate,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.ValidateScope(tt.token, tt.requiredScope)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestMerchantResolver_ValidateCustomerAccess(t *testing.T) {
	resolver := NewMerchantResolver()

	customerID := "customer_123"

	tests := []struct {
		name        string
		token       *domain.TokenClaims
		customerID  string
		expectError bool
	}{
		{
			name: "customer accessing own resources",
			token: &domain.TokenClaims{
				TokenType:  domain.TokenTypeCustomer,
				CustomerID: &customerID,
			},
			customerID:  "customer_123",
			expectError: false,
		},
		{
			name: "customer accessing other customer resources",
			token: &domain.TokenClaims{
				TokenType:  domain.TokenTypeCustomer,
				CustomerID: &customerID,
			},
			customerID:  "customer_456",
			expectError: true,
		},
		{
			name: "merchant token - skip validation",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeMerchant,
				MerchantIDs: []string{"merchant_123"},
			},
			customerID:  "any_customer",
			expectError: false,
		},
		{
			name: "admin token - skip validation",
			token: &domain.TokenClaims{
				TokenType:   domain.TokenTypeAdmin,
				MerchantIDs: []string{},
			},
			customerID:  "any_customer",
			expectError: false,
		},
		{
			name: "customer token - missing customer_id in token",
			token: &domain.TokenClaims{
				TokenType:  domain.TokenTypeCustomer,
				CustomerID: nil,
			},
			customerID:  "customer_123",
			expectError: true,
		},
		{
			name: "customer token - empty customer_id validation",
			token: &domain.TokenClaims{
				TokenType:  domain.TokenTypeCustomer,
				CustomerID: &customerID,
			},
			customerID:  "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.ValidateCustomerAccess(tt.token, tt.customerID)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
