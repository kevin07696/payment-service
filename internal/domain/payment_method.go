package domain

import (
	"time"
)

// PaymentMethod represents a saved payment method (tokenized)
type PaymentMethod struct {
	// Identity
	ID string `json:"id"` // UUID

	// Multi-tenant
	AgentID string `json:"agent_id"`

	// Customer
	CustomerID string `json:"customer_id"`

	// Payment type
	PaymentType PaymentMethodType `json:"payment_type"` // credit_card or ach

	// Tokenization
	PaymentToken string `json:"payment_token"` // EPX token (AUTH_GUID from tokenization)

	// Display metadata (NEVER store full card/account numbers)
	LastFour string `json:"last_four"` // Last 4 digits

	// Credit card specific (optional)
	CardBrand    *string `json:"card_brand"`     // "visa", "mastercard", "amex", "discover"
	CardExpMonth *int    `json:"card_exp_month"` // 1-12
	CardExpYear  *int    `json:"card_exp_year"`  // 2025, 2026, etc.

	// ACH specific (optional)
	BankName    *string `json:"bank_name"`    // "Chase", "Bank of America", etc.
	AccountType *string `json:"account_type"` // "checking" or "savings"

	// Status
	IsDefault  bool `json:"is_default"`
	IsActive   bool `json:"is_active"`
	IsVerified bool `json:"is_verified"` // For ACH pre-note verification

	// Timestamps
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// IsCreditCard returns true if this is a credit card payment method
func (pm *PaymentMethod) IsCreditCard() bool {
	return pm.PaymentType == PaymentMethodTypeCreditCard
}

// IsACH returns true if this is an ACH payment method
func (pm *PaymentMethod) IsACH() bool {
	return pm.PaymentType == PaymentMethodTypeACH
}

// IsExpired returns true if the credit card is expired
func (pm *PaymentMethod) IsExpired() bool {
	if !pm.IsCreditCard() || pm.CardExpMonth == nil || pm.CardExpYear == nil {
		return false
	}

	now := time.Now()
	expYear := *pm.CardExpYear
	expMonth := *pm.CardExpMonth

	// Check if expired
	if expYear < now.Year() {
		return true
	}
	if expYear == now.Year() && expMonth < int(now.Month()) {
		return true
	}

	return false
}

// CanBeUsed returns true if the payment method can be used for transactions
func (pm *PaymentMethod) CanBeUsed() bool {
	if !pm.IsActive {
		return false
	}

	// ACH must be verified
	if pm.IsACH() && !pm.IsVerified {
		return false
	}

	// Credit card must not be expired
	if pm.IsCreditCard() && pm.IsExpired() {
		return false
	}

	return true
}

// GetDisplayName returns a human-readable display name for the payment method
func (pm *PaymentMethod) GetDisplayName() string {
	if pm.IsCreditCard() {
		brand := "Card"
		if pm.CardBrand != nil {
			brand = *pm.CardBrand
		}
		return brand + " •••• " + pm.LastFour
	}

	// ACH
	accountType := "Account"
	if pm.AccountType != nil {
		accountType = *pm.AccountType
	}
	bankName := ""
	if pm.BankName != nil {
		bankName = *pm.BankName + " "
	}
	return bankName + accountType + " •••• " + pm.LastFour
}

// MarkUsed updates the last used timestamp
func (pm *PaymentMethod) MarkUsed() {
	now := time.Now()
	pm.LastUsedAt = &now
}