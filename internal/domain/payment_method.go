package domain

import (
	"time"

	"github.com/kevin07696/payment-service/pkg/timeutil"
)

// VerificationStatus represents the verification status of a payment method
type VerificationStatus string

const (
	VerificationStatusPending  VerificationStatus = "pending"
	VerificationStatusVerified VerificationStatus = "verified"
	VerificationStatusFailed   VerificationStatus = "failed"
)

// No grace period - ACH accounts must be fully verified before use

// PaymentMethod represents a saved payment method (tokenized)
type PaymentMethod struct {
	CreatedAt                 time.Time         `json:"created_at"`
	UpdatedAt                 time.Time         `json:"updated_at"`
	ReturnCount               *int              `json:"return_count"`
	PreNoteTransactionID      *string           `json:"prenote_transaction_id"`
	LastUsedAt                *time.Time        `json:"last_used_at"`
	DeactivatedAt             *time.Time        `json:"deactivated_at"`
	CardBrand                 *string           `json:"card_brand"`
	CardExpMonth              *int              `json:"card_exp_month"`
	CardExpYear               *int              `json:"card_exp_year"`
	BankName                  *string           `json:"bank_name"`
	AccountType               *string           `json:"account_type"`
	VerificationStatus        *string           `json:"verification_status"`
	DeactivationReason        *string           `json:"deactivation_reason"`
	VerificationFailureReason *string           `json:"verification_failure_reason"`
	VerifiedAt                *time.Time        `json:"verified_at"`
	ID                        string            `json:"id"`
	PaymentType               PaymentMethodType `json:"payment_type"`
	LastFour                  string            `json:"last_four"`
	CustomerID                string            `json:"customer_id"`
	MerchantID                string            `json:"merchant_id"`
	PaymentToken              string            `json:"payment_token"`
	IsDefault                 bool              `json:"is_default"`
	IsVerified                bool              `json:"is_verified"`
	IsActive                  bool              `json:"is_active"`
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

	now := timeutil.Now()
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

// CanUseForAmount returns true if the payment method can be used for the specified amount
// For ACH: requires full verification (no grace period)
func (pm *PaymentMethod) CanUseForAmount(amountCents int64) (bool, string) {
	// Check active status FIRST (applies to all payment types)
	// Inactive payment methods should always return "not active" regardless of other states
	if !pm.IsActive {
		return false, "payment method is not active"
	}

	// Credit card expiration check
	if pm.IsCreditCard() && pm.IsExpired() {
		return false, "credit card is expired"
	}

	// ACH verification check (only if active)
	if pm.IsACH() && !pm.IsVerified {
		return false, "ACH account must be verified before use"
	}

	return true, ""
}

// CanBeUsed returns true if the payment method can be used for transactions
// NOTE: This does NOT check amount-specific limits. Use CanUseForAmount() for that.
func (pm *PaymentMethod) CanBeUsed() bool {
	if !pm.IsActive {
		return false
	}

	// Credit card must not be expired
	if pm.IsCreditCard() && pm.IsExpired() {
		return false
	}

	// ACH must be verified
	if pm.IsACH() && !pm.IsVerified {
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
	now := timeutil.Now()
	pm.LastUsedAt = &now
}
