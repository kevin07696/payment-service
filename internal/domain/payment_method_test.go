package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestPaymentMethod_CanUseForAmount_ACHUnverified tests that unverified ACH accounts are blocked
func TestPaymentMethod_CanUseForAmount_ACHUnverified(t *testing.T) {
	pm := &PaymentMethod{
		PaymentType: PaymentMethodTypeACH,
		IsActive:    true,
		IsVerified:  false, // Unverified
		CreatedAt:   time.Now(),
	}

	// Try with low amount
	canUse, reason := pm.CanUseForAmount(10000) // $100.00
	assert.False(t, canUse, "Unverified ACH should be blocked")
	assert.Contains(t, reason, "must be verified", "Error should mention verification requirement")

	// Try with high amount
	canUse, reason = pm.CanUseForAmount(250000) // $2,500.00
	assert.False(t, canUse, "Unverified ACH should be blocked")
	assert.Contains(t, reason, "must be verified")
}

// TestPaymentMethod_CanUseForAmount_ACHVerified tests that verified ACH accounts are allowed
func TestPaymentMethod_CanUseForAmount_ACHVerified(t *testing.T) {
	pm := &PaymentMethod{
		PaymentType: PaymentMethodTypeACH,
		IsActive:    true,
		IsVerified:  true, // Verified
		CreatedAt:   time.Now().Add(-4 * 24 * time.Hour), // 4 days old
	}

	// Try with low amount
	canUse, reason := pm.CanUseForAmount(10000) // $100.00
	assert.True(t, canUse, "Verified ACH should be allowed")
	assert.Empty(t, reason, "No error for verified ACH")

	// Try with high amount
	canUse, reason = pm.CanUseForAmount(500000) // $5,000.00
	assert.True(t, canUse, "Verified ACH should be allowed for any amount")
	assert.Empty(t, reason)
}

// TestPaymentMethod_CanUseForAmount_ACHInactive tests that inactive accounts are blocked
func TestPaymentMethod_CanUseForAmount_ACHInactive(t *testing.T) {
	pm := &PaymentMethod{
		PaymentType: PaymentMethodTypeACH,
		IsActive:    false, // Inactive (e.g., failed verification)
		IsVerified:  true,
		CreatedAt:   time.Now(),
	}

	canUse, reason := pm.CanUseForAmount(10000)
	assert.False(t, canUse, "Inactive ACH should be blocked")
	assert.Contains(t, reason, "not active", "Error should mention inactive status")
}

// TestPaymentMethod_CanUseForAmount_CreditCard tests credit card validation
func TestPaymentMethod_CanUseForAmount_CreditCard(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		isActive    bool
		expMonth    int
		expYear     int
		amountCents int64
		expectedOK  bool
		expectedMsg string
	}{
		{
			name:        "Active_NotExpired",
			isActive:    true,
			expMonth:    12,
			expYear:     now.Year() + 1,
			amountCents: 10000,
			expectedOK:  true,
			expectedMsg: "",
		},
		{
			name:        "Active_Expired",
			isActive:    true,
			expMonth:    1,
			expYear:     now.Year() - 1,
			amountCents: 10000,
			expectedOK:  false,
			expectedMsg: "expired",
		},
		{
			name:        "Inactive",
			isActive:    false,
			expMonth:    12,
			expYear:     now.Year() + 1,
			amountCents: 10000,
			expectedOK:  false,
			expectedMsg: "not active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				PaymentType:  PaymentMethodTypeCreditCard,
				IsActive:     tt.isActive,
				CardExpMonth: &tt.expMonth,
				CardExpYear:  &tt.expYear,
			}

			canUse, reason := pm.CanUseForAmount(tt.amountCents)
			assert.Equal(t, tt.expectedOK, canUse)
			if !tt.expectedOK {
				assert.Contains(t, reason, tt.expectedMsg)
			}
		})
	}
}

// TestPaymentMethod_IsACH tests the IsACH helper method
func TestPaymentMethod_IsACH(t *testing.T) {
	tests := []struct {
		name        string
		paymentType PaymentMethodType
		expected    bool
	}{
		{
			name:        "ACH_ReturnsTrue",
			paymentType: PaymentMethodTypeACH,
			expected:    true,
		},
		{
			name:        "CreditCard_ReturnsFalse",
			paymentType: PaymentMethodTypeCreditCard,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				PaymentType: tt.paymentType,
			}
			assert.Equal(t, tt.expected, pm.IsACH())
		})
	}
}

// TestPaymentMethod_IsCreditCard tests the IsCreditCard helper method
func TestPaymentMethod_IsCreditCard(t *testing.T) {
	tests := []struct {
		name        string
		paymentType PaymentMethodType
		expected    bool
	}{
		{
			name:        "CreditCard_ReturnsTrue",
			paymentType: PaymentMethodTypeCreditCard,
			expected:    true,
		},
		{
			name:        "ACH_ReturnsFalse",
			paymentType: PaymentMethodTypeACH,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				PaymentType: tt.paymentType,
			}
			assert.Equal(t, tt.expected, pm.IsCreditCard())
		})
	}
}

// TestPaymentMethod_CanBeUsed tests the general usability check
func TestPaymentMethod_CanBeUsed(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		paymentType PaymentMethodType
		isActive    bool
		isVerified  bool
		expMonth    *int
		expYear     *int
		expected    bool
	}{
		{
			name:        "ACH_Verified_Active",
			paymentType: PaymentMethodTypeACH,
			isActive:    true,
			isVerified:  true,
			expected:    true,
		},
		{
			name:        "ACH_Unverified_Active",
			paymentType: PaymentMethodTypeACH,
			isActive:    true,
			isVerified:  false,
			expected:    false, // Must be verified
		},
		{
			name:        "ACH_Verified_Inactive",
			paymentType: PaymentMethodTypeACH,
			isActive:    false,
			isVerified:  true,
			expected:    false,
		},
		{
			name:        "CreditCard_Active_NotExpired",
			paymentType: PaymentMethodTypeCreditCard,
			isActive:    true,
			isVerified:  true,
			expMonth:    intPtr(12),
			expYear:     intPtr(now.Year() + 1),
			expected:    true,
		},
		{
			name:        "CreditCard_Active_Expired",
			paymentType: PaymentMethodTypeCreditCard,
			isActive:    true,
			isVerified:  true,
			expMonth:    intPtr(1),
			expYear:     intPtr(now.Year() - 1),
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				PaymentType:  tt.paymentType,
				IsActive:     tt.isActive,
				IsVerified:   tt.isVerified,
				CardExpMonth: tt.expMonth,
				CardExpYear:  tt.expYear,
			}
			assert.Equal(t, tt.expected, pm.CanBeUsed())
		})
	}
}

// Helper function for tests
func intPtr(i int) *int {
	return &i
}
