package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestParseExpirationDateMMYY tests parsing of MMYY expiration date strings
func TestParseExpirationDateMMYY(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *ExpirationDate
	}{
		{
			name:     "valid_december_2025",
			input:    "1225",
			expected: &ExpirationDate{Month: 12, Year: 2025},
		},
		{
			name:     "valid_january_2030",
			input:    "0130",
			expected: &ExpirationDate{Month: 1, Year: 2030},
		},
		{
			name:     "valid_june_2024",
			input:    "0624",
			expected: &ExpirationDate{Month: 6, Year: 2024},
		},
		{
			name:     "invalid_month_00",
			input:    "0025",
			expected: nil,
		},
		{
			name:     "invalid_month_13",
			input:    "1325",
			expected: nil,
		},
		{
			name:     "too_short",
			input:    "122",
			expected: nil,
		},
		{
			name:     "too_long",
			input:    "12255",
			expected: nil,
		},
		{
			name:     "empty_string",
			input:    "",
			expected: nil,
		},
		{
			name:     "non_numeric",
			input:    "abcd",
			expected: nil,
		},
		{
			name:     "partial_numeric",
			input:    "12ab",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseExpirationDateMMYY(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Month, result.Month)
				assert.Equal(t, tt.expected.Year, result.Year)
			}
		})
	}
}

// TestFormatExpirationDateMMYY tests formatting of MMYY to MM/YY display format
func TestFormatExpirationDateMMYY(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid_december_2025",
			input:    "1225",
			expected: "12/25",
		},
		{
			name:     "valid_january_2030",
			input:    "0130",
			expected: "01/30",
		},
		{
			name:     "too_short_returned_unchanged",
			input:    "122",
			expected: "122",
		},
		{
			name:     "too_long_returned_unchanged",
			input:    "12255",
			expected: "12255",
		},
		{
			name:     "empty_returned_unchanged",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatExpirationDateMMYY(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFormatMaskedCard tests formatting of last four digits as masked card number
func TestFormatMaskedCard(t *testing.T) {
	tests := []struct {
		name     string
		lastFour string
		expected string
	}{
		{
			name:     "valid_last_four",
			lastFour: "4242",
			expected: "****-****-****-4242",
		},
		{
			name:     "different_last_four",
			lastFour: "1234",
			expected: "****-****-****-1234",
		},
		{
			name:     "empty_returns_empty",
			lastFour: "",
			expected: "",
		},
		{
			name:     "three_digits",
			lastFour: "123",
			expected: "****-****-****-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMaskedCard(tt.lastFour)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPaymentMethod_CanUseForAmount_ACHUnverified tests that unverified ACH accounts are blocked
func TestPaymentMethod_CanUseForAmount_ACHUnverified(t *testing.T) {
	pm := &PaymentMethod{
		PaymentType: PaymentMethodTypeACH,
		Status:      PaymentMethodStatusPending, // Unverified (pending)
		CreatedAt:   time.Now(),
	}

	// Try with low amount
	canUse, reason := pm.CanUseForAmount(10000) // $100.00
	assert.False(t, canUse, "Unverified ACH should be blocked")
	assert.Contains(t, reason, "pending verification", "Error should mention pending status")

	// Try with high amount
	canUse, reason = pm.CanUseForAmount(250000) // $2,500.00
	assert.False(t, canUse, "Unverified ACH should be blocked")
	assert.Contains(t, reason, "pending")
}

// TestPaymentMethod_CanUseForAmount_ACHVerified tests that verified ACH accounts are allowed
func TestPaymentMethod_CanUseForAmount_ACHVerified(t *testing.T) {
	pm := &PaymentMethod{
		PaymentType: PaymentMethodTypeACH,
		Status:      PaymentMethodStatusActive,           // Verified (active)
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
		Status:      PaymentMethodStatusRevoked, // Inactive (revoked)
		CreatedAt:   time.Now(),
	}

	canUse, reason := pm.CanUseForAmount(10000)
	assert.False(t, canUse, "Inactive ACH should be blocked")
	assert.Contains(t, reason, "revoked", "Error should mention revoked status")
}

// TestPaymentMethod_CanUseForAmount_CreditCard tests credit card validation
func TestPaymentMethod_CanUseForAmount_CreditCard(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		status      PaymentMethodStatus
		expMonth    int
		expYear     int
		amountCents int64
		expectedOK  bool
		expectedMsg string
	}{
		{
			name:        "Active_NotExpired",
			status:      PaymentMethodStatusActive,
			expMonth:    12,
			expYear:     now.Year() + 1,
			amountCents: 10000,
			expectedOK:  true,
			expectedMsg: "",
		},
		{
			name:        "Expired_Status",
			status:      PaymentMethodStatusExpired,
			expMonth:    1,
			expYear:     now.Year() - 1,
			amountCents: 10000,
			expectedOK:  false,
			expectedMsg: "expired",
		},
		{
			name:        "Revoked",
			status:      PaymentMethodStatusRevoked,
			expMonth:    12,
			expYear:     now.Year() + 1,
			amountCents: 10000,
			expectedOK:  false,
			expectedMsg: "revoked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				PaymentType:  PaymentMethodTypeCreditCard,
				Status:       tt.status,
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
		status      PaymentMethodStatus
		expMonth    *int
		expYear     *int
		expected    bool
	}{
		{
			name:        "ACH_Active",
			paymentType: PaymentMethodTypeACH,
			status:      PaymentMethodStatusActive,
			expected:    true,
		},
		{
			name:        "ACH_Pending",
			paymentType: PaymentMethodTypeACH,
			status:      PaymentMethodStatusPending,
			expected:    false, // Pending = not usable
		},
		{
			name:        "ACH_Revoked",
			paymentType: PaymentMethodTypeACH,
			status:      PaymentMethodStatusRevoked,
			expected:    false,
		},
		{
			name:        "CreditCard_Active",
			paymentType: PaymentMethodTypeCreditCard,
			status:      PaymentMethodStatusActive,
			expMonth:    intPtr(12),
			expYear:     intPtr(now.Year() + 1),
			expected:    true,
		},
		{
			name:        "CreditCard_Expired",
			paymentType: PaymentMethodTypeCreditCard,
			status:      PaymentMethodStatusExpired,
			expMonth:    intPtr(1),
			expYear:     intPtr(now.Year() - 1),
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				PaymentType:  tt.paymentType,
				Status:       tt.status,
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

func strPtr(s string) *string {
	return &s
}

// ============================================================================
// GetDisplayName Tests
// ============================================================================

// TestPaymentMethod_GetDisplayName_CreditCard tests credit card display names
func TestPaymentMethod_GetDisplayName_CreditCard(t *testing.T) {
	tests := []struct {
		name      string
		cardBrand *string
		lastFour  string
		expected  string
	}{
		{
			name:      "Visa card with brand",
			cardBrand: strPtr("Visa"),
			lastFour:  "4242",
			expected:  "Visa •••• 4242",
		},
		{
			name:      "Mastercard with brand",
			cardBrand: strPtr("Mastercard"),
			lastFour:  "5555",
			expected:  "Mastercard •••• 5555",
		},
		{
			name:      "Amex with brand",
			cardBrand: strPtr("American Express"),
			lastFour:  "1234",
			expected:  "American Express •••• 1234",
		},
		{
			name:      "Card without brand (nil)",
			cardBrand: nil,
			lastFour:  "9999",
			expected:  "Card •••• 9999",
		},
		{
			name:      "Card with empty brand",
			cardBrand: strPtr(""),
			lastFour:  "0000",
			expected:  " •••• 0000",
		},
		{
			name:      "Discover card",
			cardBrand: strPtr("Discover"),
			lastFour:  "6011",
			expected:  "Discover •••• 6011",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				PaymentType: PaymentMethodTypeCreditCard,
				CardBrand:   tt.cardBrand,
				LastFour:    tt.lastFour,
			}
			result := pm.GetDisplayName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPaymentMethod_GetDisplayName_ACH tests ACH account display names
func TestPaymentMethod_GetDisplayName_ACH(t *testing.T) {
	tests := []struct {
		name        string
		bankName    *string
		accountType *string
		lastFour    string
		expected    string
	}{
		{
			name:        "Checking account with bank name",
			bankName:    strPtr("Chase"),
			accountType: strPtr("Checking"),
			lastFour:    "1234",
			expected:    "Chase Checking •••• 1234",
		},
		{
			name:        "Savings account with bank name",
			bankName:    strPtr("Bank of America"),
			accountType: strPtr("Savings"),
			lastFour:    "5678",
			expected:    "Bank of America Savings •••• 5678",
		},
		{
			name:        "Account without bank name",
			bankName:    nil,
			accountType: strPtr("Checking"),
			lastFour:    "9999",
			expected:    "Checking •••• 9999",
		},
		{
			name:        "Account without account type",
			bankName:    strPtr("Wells Fargo"),
			accountType: nil,
			lastFour:    "4321",
			expected:    "Wells Fargo Account •••• 4321",
		},
		{
			name:        "Account with neither bank name nor type",
			bankName:    nil,
			accountType: nil,
			lastFour:    "0000",
			expected:    "Account •••• 0000",
		},
		{
			name:        "Account with empty bank name",
			bankName:    strPtr(""),
			accountType: strPtr("Checking"),
			lastFour:    "7777",
			expected:    " Checking •••• 7777",
		},
		{
			name:        "Account with empty account type",
			bankName:    strPtr("Citibank"),
			accountType: strPtr(""),
			lastFour:    "8888",
			expected:    "Citibank  •••• 8888",
		},
		{
			name:        "Business checking account",
			bankName:    strPtr("US Bank"),
			accountType: strPtr("Business Checking"),
			lastFour:    "3333",
			expected:    "US Bank Business Checking •••• 3333",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				PaymentType: PaymentMethodTypeACH,
				BankName:    tt.bankName,
				AccountType: tt.accountType,
				LastFour:    tt.lastFour,
			}
			result := pm.GetDisplayName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPaymentMethod_GetDisplayName_EdgeCases tests edge cases and boundary conditions
func TestPaymentMethod_GetDisplayName_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		paymentType PaymentMethodType
		cardBrand   *string
		bankName    *string
		accountType *string
		lastFour    string
		expected    string
	}{
		{
			name:        "Empty last four (credit card)",
			paymentType: PaymentMethodTypeCreditCard,
			cardBrand:   strPtr("Visa"),
			lastFour:    "",
			expected:    "Visa •••• ",
		},
		{
			name:        "Empty last four (ACH)",
			paymentType: PaymentMethodTypeACH,
			bankName:    strPtr("Chase"),
			accountType: strPtr("Checking"),
			lastFour:    "",
			expected:    "Chase Checking •••• ",
		},
		{
			name:        "Very long brand name",
			paymentType: PaymentMethodTypeCreditCard,
			cardBrand:   strPtr("Super Premium Platinum Rewards Card"),
			lastFour:    "4242",
			expected:    "Super Premium Platinum Rewards Card •••• 4242",
		},
		{
			name:        "Very long bank name",
			paymentType: PaymentMethodTypeACH,
			bankName:    strPtr("The First National Bank of the United States"),
			accountType: strPtr("Checking"),
			lastFour:    "1234",
			expected:    "The First National Bank of the United States Checking •••• 1234",
		},
		{
			name:        "Special characters in brand",
			paymentType: PaymentMethodTypeCreditCard,
			cardBrand:   strPtr("Visa®"),
			lastFour:    "4242",
			expected:    "Visa® •••• 4242",
		},
		{
			name:        "Unicode characters in bank name",
			paymentType: PaymentMethodTypeACH,
			bankName:    strPtr("José's Bank 💰"),
			accountType: strPtr("Checking"),
			lastFour:    "1234",
			expected:    "José's Bank 💰 Checking •••• 1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				PaymentType: tt.paymentType,
				CardBrand:   tt.cardBrand,
				BankName:    tt.bankName,
				AccountType: tt.accountType,
				LastFour:    tt.lastFour,
			}
			result := pm.GetDisplayName()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// MarkUsed Tests
// ============================================================================

// TestPaymentMethod_MarkUsed_InitialUse tests marking a payment method as used for the first time
func TestPaymentMethod_MarkUsed_InitialUse(t *testing.T) {
	pm := &PaymentMethod{
		ID:         "pm_123",
		LastUsedAt: nil, // Never used before
	}

	// Mark as used
	beforeMark := time.Now()
	pm.MarkUsed()
	afterMark := time.Now()

	// Assert: LastUsedAt is set
	assert.NotNil(t, pm.LastUsedAt, "LastUsedAt should be set")

	// Assert: Timestamp is recent (within last second)
	assert.True(t, pm.LastUsedAt.After(beforeMark.Add(-1*time.Second)),
		"LastUsedAt should be after test start")
	assert.True(t, pm.LastUsedAt.Before(afterMark.Add(1*time.Second)),
		"LastUsedAt should be before test end")
}

// TestPaymentMethod_MarkUsed_SubsequentUse tests updating an already-used payment method
func TestPaymentMethod_MarkUsed_SubsequentUse(t *testing.T) {
	yesterday := time.Now().Add(-24 * time.Hour)
	pm := &PaymentMethod{
		ID:         "pm_456",
		LastUsedAt: &yesterday,
	}

	// Mark as used again
	beforeMark := time.Now()
	pm.MarkUsed()
	afterMark := time.Now()

	// Assert: LastUsedAt is updated
	assert.NotNil(t, pm.LastUsedAt, "LastUsedAt should still be set")
	assert.True(t, pm.LastUsedAt.After(yesterday),
		"LastUsedAt should be updated to a newer time")

	// Assert: New timestamp is recent
	assert.True(t, pm.LastUsedAt.After(beforeMark.Add(-1*time.Second)),
		"LastUsedAt should be after test start")
	assert.True(t, pm.LastUsedAt.Before(afterMark.Add(1*time.Second)),
		"LastUsedAt should be before test end")
}

// TestPaymentMethod_MarkUsed_MultipleSequentialCalls tests idempotency of multiple calls
func TestPaymentMethod_MarkUsed_MultipleSequentialCalls(t *testing.T) {
	pm := &PaymentMethod{
		ID:         "pm_789",
		LastUsedAt: nil,
	}

	// First call
	pm.MarkUsed()
	assert.NotNil(t, pm.LastUsedAt)

	// Second call - verify timestamp is at or after current time
	beforeSecondCall := time.Now()
	pm.MarkUsed()
	assert.NotNil(t, pm.LastUsedAt)

	// Assert: Second timestamp is at or after the time we captured
	assert.True(t, !pm.LastUsedAt.Before(beforeSecondCall),
		"Second MarkUsed should update timestamp to at or after operation time")
}

// TestPaymentMethod_MarkUsed_PreservesOtherFields tests that MarkUsed doesn't affect other fields
func TestPaymentMethod_MarkUsed_PreservesOtherFields(t *testing.T) {
	pm := &PaymentMethod{
		ID:           "pm_preserve",
		MerchantID:   "merchant_123",
		CustomerID:   "customer_456",
		PaymentType:  PaymentMethodTypeCreditCard,
		PaymentToken: "token_abc",
		LastFour:     "4242",
		CardBrand:    strPtr("Visa"),
		Status:       PaymentMethodStatusActive,
		LastUsedAt:   nil,
	}

	// Store original values
	originalID := pm.ID
	originalMerchantID := pm.MerchantID
	originalCustomerID := pm.CustomerID
	originalPaymentType := pm.PaymentType
	originalToken := pm.PaymentToken
	originalLastFour := pm.LastFour
	originalStatus := pm.Status

	// Mark as used
	pm.MarkUsed()

	// Assert: All other fields remain unchanged
	assert.Equal(t, originalID, pm.ID)
	assert.Equal(t, originalMerchantID, pm.MerchantID)
	assert.Equal(t, originalCustomerID, pm.CustomerID)
	assert.Equal(t, originalPaymentType, pm.PaymentType)
	assert.Equal(t, originalToken, pm.PaymentToken)
	assert.Equal(t, originalLastFour, pm.LastFour)
	assert.Equal(t, originalStatus, pm.Status)

	// Only LastUsedAt should be updated
	assert.NotNil(t, pm.LastUsedAt)
}
