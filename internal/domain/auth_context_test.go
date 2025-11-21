package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTokenClaims_HasScope tests scope checking logic
func TestTokenClaims_HasScope(t *testing.T) {
	tests := []struct {
		name        string
		scopes      []string
		checkScope  string
		expectedHas bool
	}{
		{
			name:        "has specific scope",
			scopes:      []string{ScopePaymentsCreate, ScopePaymentsRead},
			checkScope:  ScopePaymentsCreate,
			expectedHas: true,
		},
		{
			name:        "does not have scope",
			scopes:      []string{ScopePaymentsRead},
			checkScope:  ScopePaymentsCreate,
			expectedHas: false,
		},
		{
			name:        "has wildcard scope",
			scopes:      []string{ScopeAll},
			checkScope:  ScopePaymentsCreate,
			expectedHas: true,
		},
		{
			name:        "wildcard grants any scope",
			scopes:      []string{ScopeAll},
			checkScope:  ScopePaymentsVoid,
			expectedHas: true,
		},
		{
			name:        "wildcard among other scopes",
			scopes:      []string{ScopePaymentsRead, ScopeAll, ScopePaymentsCreate},
			checkScope:  ScopePaymentsRefund,
			expectedHas: true,
		},
		{
			name:        "empty scopes returns false",
			scopes:      []string{},
			checkScope:  ScopePaymentsCreate,
			expectedHas: false,
		},
		{
			name:        "nil scopes returns false",
			scopes:      nil,
			checkScope:  ScopePaymentsCreate,
			expectedHas: false,
		},
		{
			name:        "multiple scopes with match",
			scopes:      []string{ScopePaymentsCreate, ScopePaymentsRead, ScopePaymentsVoid},
			checkScope:  ScopePaymentsVoid,
			expectedHas: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &TokenClaims{
				Scopes: tt.scopes,
			}
			result := claims.HasScope(tt.checkScope)
			assert.Equal(t, tt.expectedHas, result)
		})
	}
}

// TestTokenClaims_IsSingleMerchant tests single merchant detection
func TestTokenClaims_IsSingleMerchant(t *testing.T) {
	tests := []struct {
		name        string
		merchantIDs []string
		expected    bool
	}{
		{
			name:        "single merchant returns true",
			merchantIDs: []string{"merchant_123"},
			expected:    true,
		},
		{
			name:        "multiple merchants returns false",
			merchantIDs: []string{"merchant_123", "merchant_456"},
			expected:    false,
		},
		{
			name:        "no merchants returns false",
			merchantIDs: []string{},
			expected:    false,
		},
		{
			name:        "nil merchants returns false",
			merchantIDs: nil,
			expected:    false,
		},
		{
			name:        "three merchants returns false",
			merchantIDs: []string{"merchant_1", "merchant_2", "merchant_3"},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &TokenClaims{
				MerchantIDs: tt.merchantIDs,
			}
			result := claims.IsSingleMerchant()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTokenClaims_IsMultiMerchant tests multi-merchant detection
func TestTokenClaims_IsMultiMerchant(t *testing.T) {
	tests := []struct {
		name        string
		merchantIDs []string
		expected    bool
	}{
		{
			name:        "multiple merchants returns true",
			merchantIDs: []string{"merchant_123", "merchant_456"},
			expected:    true,
		},
		{
			name:        "single merchant returns false",
			merchantIDs: []string{"merchant_123"},
			expected:    false,
		},
		{
			name:        "no merchants returns false",
			merchantIDs: []string{},
			expected:    false,
		},
		{
			name:        "nil merchants returns false",
			merchantIDs: nil,
			expected:    false,
		},
		{
			name:        "three merchants returns true",
			merchantIDs: []string{"merchant_1", "merchant_2", "merchant_3"},
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &TokenClaims{
				MerchantIDs: tt.merchantIDs,
			}
			result := claims.IsMultiMerchant()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTokenClaims_GetSingleMerchantID tests merchant ID extraction
func TestTokenClaims_GetSingleMerchantID(t *testing.T) {
	tests := []struct {
		name           string
		merchantIDs    []string
		expectedID     string
		expectedExists bool
	}{
		{
			name:           "single merchant returns ID",
			merchantIDs:    []string{"merchant_abc123"},
			expectedID:     "merchant_abc123",
			expectedExists: true,
		},
		{
			name:           "multiple merchants returns empty and false",
			merchantIDs:    []string{"merchant_123", "merchant_456"},
			expectedID:     "",
			expectedExists: false,
		},
		{
			name:           "no merchants returns empty and false",
			merchantIDs:    []string{},
			expectedID:     "",
			expectedExists: false,
		},
		{
			name:           "nil merchants returns empty and false",
			merchantIDs:    nil,
			expectedID:     "",
			expectedExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &TokenClaims{
				MerchantIDs: tt.merchantIDs,
			}
			id, exists := claims.GetSingleMerchantID()
			assert.Equal(t, tt.expectedID, id)
			assert.Equal(t, tt.expectedExists, exists)
		})
	}
}

// TestTokenClaims_HasMerchantAccess tests merchant access checking
func TestTokenClaims_HasMerchantAccess(t *testing.T) {
	tests := []struct {
		name        string
		merchantIDs []string
		checkID     string
		expectedHas bool
	}{
		{
			name:        "has access to single merchant",
			merchantIDs: []string{"merchant_123"},
			checkID:     "merchant_123",
			expectedHas: true,
		},
		{
			name:        "has access to one of multiple merchants",
			merchantIDs: []string{"merchant_123", "merchant_456", "merchant_789"},
			checkID:     "merchant_456",
			expectedHas: true,
		},
		{
			name:        "does not have access",
			merchantIDs: []string{"merchant_123", "merchant_456"},
			checkID:     "merchant_999",
			expectedHas: false,
		},
		{
			name:        "empty merchant list denies access",
			merchantIDs: []string{},
			checkID:     "merchant_123",
			expectedHas: false,
		},
		{
			name:        "nil merchant list denies access",
			merchantIDs: nil,
			checkID:     "merchant_123",
			expectedHas: false,
		},
		{
			name:        "case sensitive check",
			merchantIDs: []string{"merchant_ABC"},
			checkID:     "merchant_abc",
			expectedHas: false,
		},
		{
			name:        "exact match required",
			merchantIDs: []string{"merchant_123456"},
			checkID:     "merchant_123",
			expectedHas: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &TokenClaims{
				MerchantIDs: tt.merchantIDs,
			}
			result := claims.HasMerchantAccess(tt.checkID)
			assert.Equal(t, tt.expectedHas, result)
		})
	}
}

// TestTokenClaims_EdgeCases tests boundary conditions and edge cases
func TestTokenClaims_EdgeCases(t *testing.T) {
	t.Run("empty string merchant ID", func(t *testing.T) {
		claims := &TokenClaims{
			MerchantIDs: []string{""},
		}

		assert.True(t, claims.IsSingleMerchant())
		assert.False(t, claims.IsMultiMerchant())

		id, exists := claims.GetSingleMerchantID()
		assert.True(t, exists)
		assert.Equal(t, "", id)

		assert.True(t, claims.HasMerchantAccess(""))
		assert.False(t, claims.HasMerchantAccess("merchant_123"))
	})

	t.Run("empty string scope", func(t *testing.T) {
		claims := &TokenClaims{
			Scopes: []string{""},
		}

		assert.True(t, claims.HasScope(""))
		assert.False(t, claims.HasScope(ScopePaymentsCreate))
	})

	t.Run("nil pointers for optional fields", func(t *testing.T) {
		claims := &TokenClaims{
			TokenType:   TokenTypeMerchant,
			MerchantIDs: []string{"merchant_123"},
			CustomerID:  nil,
			SessionID:   nil,
			Scopes:      []string{ScopePaymentsCreate},
		}

		assert.Nil(t, claims.CustomerID)
		assert.Nil(t, claims.SessionID)
		assert.True(t, claims.HasScope(ScopePaymentsCreate))
	})

	t.Run("duplicate merchant IDs in list", func(t *testing.T) {
		claims := &TokenClaims{
			MerchantIDs: []string{"merchant_123", "merchant_123", "merchant_456"},
		}

		assert.True(t, claims.IsMultiMerchant())
		assert.True(t, claims.HasMerchantAccess("merchant_123"))
		assert.True(t, claims.HasMerchantAccess("merchant_456"))
	})

	t.Run("duplicate scopes in list", func(t *testing.T) {
		claims := &TokenClaims{
			Scopes: []string{ScopePaymentsCreate, ScopePaymentsCreate, ScopePaymentsRead},
		}

		assert.True(t, claims.HasScope(ScopePaymentsCreate))
		assert.True(t, claims.HasScope(ScopePaymentsRead))
	})
}
