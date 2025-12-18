package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasScope(t *testing.T) {
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
			result := HasScope(tt.scopes, tt.checkScope)
			assert.Equal(t, tt.expectedHas, result)
		})
	}
}

func TestAllPaymentScopes(t *testing.T) {
	scopes := AllPaymentScopes()

	// Should include all standard scopes
	assert.Contains(t, scopes, ScopePaymentsCreate)
	assert.Contains(t, scopes, ScopePaymentsRead)
	assert.Contains(t, scopes, ScopePaymentsVoid)
	assert.Contains(t, scopes, ScopePaymentsRefund)
	assert.Contains(t, scopes, ScopePaymentMethodsRead)
	assert.Contains(t, scopes, ScopePaymentMethodsCreate)
	assert.Contains(t, scopes, ScopeSubscriptionsManage)
	assert.Contains(t, scopes, ScopeSubscriptionsRead)

	// Should NOT include wildcard
	assert.NotContains(t, scopes, ScopeAll)
}
