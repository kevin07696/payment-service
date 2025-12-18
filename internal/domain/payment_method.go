package domain

import (
	"fmt"
	"time"

	"github.com/kevin07696/payment-service/pkg/timeutil"
)

// ExpirationDate represents a credit card expiration date
type ExpirationDate struct {
	Month int // 1-12
	Year  int // Full year (e.g., 2025)
}

// ParseExpirationDateMMYY parses an expiration date in MMYY format (e.g., "1225" = December 2025)
// Returns nil if the input is invalid or empty
func ParseExpirationDateMMYY(mmyy string) *ExpirationDate {
	if len(mmyy) != 4 {
		return nil
	}

	var month, year int
	if _, err := fmt.Sscanf(mmyy[0:2], "%d", &month); err != nil {
		return nil
	}
	if _, err := fmt.Sscanf(mmyy[2:4], "%d", &year); err != nil {
		return nil
	}

	// Validate month
	if month < 1 || month > 12 {
		return nil
	}

	// Convert 2-digit year to full year
	fullYear := 2000 + year

	return &ExpirationDate{
		Month: month,
		Year:  fullYear,
	}
}

// FormatExpirationDateMMYY formats a raw MMYY string to MM/YY display format
// Returns the input unchanged if it's not exactly 4 characters
func FormatExpirationDateMMYY(mmyy string) string {
	if len(mmyy) != 4 {
		return mmyy
	}
	return mmyy[0:2] + "/" + mmyy[2:4]
}

// VerificationStatus represents the verification status of a payment method
type VerificationStatus string

const (
	VerificationStatusVerified   VerificationStatus = "verified"   // Verified (credit cards always, ACH after 3 days)
	VerificationStatusUnverified VerificationStatus = "unverified" // ACH pending verification
)

// PrenoteStatus represents the prenote send status for ACH payment methods
type PrenoteStatus string

const (
	PrenoteStatusNotRequired PrenoteStatus = "not_required" // Credit cards
	PrenoteStatusPending     PrenoteStatus = "pending"      // Needs to be sent
	PrenoteStatusSent        PrenoteStatus = "sent"         // Successfully sent
	PrenoteStatusFailed      PrenoteStatus = "failed"       // Transient error, needs retry
	PrenoteStatusMaxRetries  PrenoteStatus = "max_retries"  // Gave up after max attempts
)

// ACHAccountType represents the type of ACH bank account
type ACHAccountType string

const (
	ACHAccountTypeChecking ACHAccountType = "checking"
	ACHAccountTypeSavings  ACHAccountType = "savings"
)

// IsValid returns true if the account type is a known valid value
func (t ACHAccountType) IsValid() bool {
	switch t {
	case ACHAccountTypeChecking, ACHAccountTypeSavings:
		return true
	}
	return false
}

// PaymentMethodStatus represents the lifecycle status of a payment method
// This is the primary status field that determines if a payment method can be used
type PaymentMethodStatus string

const (
	PaymentMethodStatusPending PaymentMethodStatus = "pending" // ACH awaiting verification (prenote period)
	PaymentMethodStatusActive  PaymentMethodStatus = "active"  // Verified and usable
	PaymentMethodStatusFailed  PaymentMethodStatus = "failed"  // Verification failed (ACH returns, max retries)
	PaymentMethodStatusExpired PaymentMethodStatus = "expired" // Credit card past expiration date
	PaymentMethodStatusRevoked PaymentMethodStatus = "revoked" // Manually deactivated by merchant/customer
)

// IsValid returns true if the status is a known valid value
func (s PaymentMethodStatus) IsValid() bool {
	switch s {
	case PaymentMethodStatusPending, PaymentMethodStatusActive,
		PaymentMethodStatusFailed, PaymentMethodStatusExpired,
		PaymentMethodStatusRevoked:
		return true
	}
	return false
}

// IsUsable returns true if the payment method can be used for transactions
func (s PaymentMethodStatus) IsUsable() bool {
	return s == PaymentMethodStatusActive
}

// IsTerminal returns true if this is a terminal status (no further transitions expected)
func (s PaymentMethodStatus) IsTerminal() bool {
	switch s {
	case PaymentMethodStatusFailed, PaymentMethodStatusExpired, PaymentMethodStatusRevoked:
		return true
	}
	return false
}

// Status change reasons (constants for audit trail)
const (
	StatusReasonACHVerified       = "ach_verified"        // 3-day period passed, no returns
	StatusReasonACHReturnDetected = "ach_return_detected" // ACH return file detected
	StatusReasonPrenoteMaxRetries = "prenote_max_retries" // Prenote failed after max attempts
	StatusReasonCardExpired       = "card_expired"        // Credit card past expiration
	StatusReasonManualRevoke      = "manual_revoke"       // Merchant/customer requested deactivation
	StatusReasonFraudSuspected    = "fraud_suspected"     // Fraud detection triggered
	StatusReasonAccountClosed     = "account_closed"      // Bank account closed
	StatusReasonInsufficientFunds = "insufficient_funds"  // Repeated NSF returns
	StatusReasonInvalidAccount    = "invalid_account"     // Invalid account number
)

// No grace period - ACH accounts must be fully verified before use

// PaymentMethod represents a saved payment method (tokenized)
// Field order optimized for memory alignment (largest to smallest):
// - time.Time (24 bytes)
// - strings (16 bytes)
// - pointers to time.Time (8 bytes)
// - pointers to string (8 bytes)
// - pointers to int (8 bytes)
// - bools (1 byte each, grouped at end)
type PaymentMethod struct {
	// Timestamps (24 bytes each) - largest fields first
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Strings (16 bytes each on 64-bit)
	ID           string              `json:"id"`
	MerchantID   string              `json:"merchant_id"`
	CustomerID   string              `json:"customer_id"`
	PaymentToken string              `json:"payment_token"`
	LastFour     string              `json:"last_four"`
	PaymentType  PaymentMethodType   `json:"payment_type"` // card, ach
	Status       PaymentMethodStatus `json:"status"`       // pending, active, failed, expired, revoked

	// Pointers to time.Time (8 bytes each)
	LastUsedAt      *time.Time `json:"last_used_at"`
	VerifiedAt      *time.Time `json:"verified_at"`       // When ACH was verified (3-day period passed)
	StatusChangedAt *time.Time `json:"status_changed_at"` // When status last changed

	// Pointers to strings (8 bytes each)
	CardBrand     *string `json:"card_brand"`
	BankName      *string `json:"bank_name"`
	AccountType   *string `json:"account_type"`
	PrenoteStatus *string `json:"prenote_status"` // Internal: not_required, pending, sent, failed, max_retries
	StatusReason  *string `json:"status_reason"`  // Why status changed (for audit trail)

	// Pointers to int (8 bytes each)
	ReturnCount     *int `json:"return_count"`
	PrenoteAttempts *int `json:"prenote_attempts"`
	CardExpMonth    *int `json:"card_exp_month"`
	CardExpYear     *int `json:"card_exp_year"`

	// Booleans (1 byte each) - smallest fields last
	IsDefault bool `json:"is_default"`
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
// Only payment methods with status 'active' can be used
func (pm *PaymentMethod) CanUseForAmount(amountCents int64) (bool, string) {
	// Check status - only 'active' payment methods can be used
	if !pm.Status.IsUsable() {
		switch pm.Status {
		case PaymentMethodStatusPending:
			return false, "payment method is pending verification"
		case PaymentMethodStatusFailed:
			return false, "payment method verification failed"
		case PaymentMethodStatusExpired:
			return false, "payment method is expired"
		case PaymentMethodStatusRevoked:
			return false, "payment method has been revoked"
		default:
			return false, "payment method is not active"
		}
	}

	return true, ""
}

// CanBeUsed returns true if the payment method can be used for transactions
// NOTE: This does NOT check amount-specific limits. Use CanUseForAmount() for that.
func (pm *PaymentMethod) CanBeUsed() bool {
	return pm.Status.IsUsable()
}

// IsActive returns true if the payment method status is active
// Convenience method for backward compatibility
func (pm *PaymentMethod) IsActive() bool {
	return pm.Status == PaymentMethodStatusActive
}

// IsVerified returns true if the payment method has been verified
// For credit cards: always true (status is 'active' immediately)
// For ACH: true when status transitions from 'pending' to 'active'
func (pm *PaymentMethod) IsVerified() bool {
	return pm.Status == PaymentMethodStatusActive
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

// ExtractLastFour extracts the last 4 characters from a masked account number
// Returns empty string if the input has fewer than 4 characters
func ExtractLastFour(masked string) string {
	if len(masked) >= 4 {
		return masked[len(masked)-4:]
	}
	return ""
}

// FormatMaskedCard formats a last four digits string as a masked card number
// Returns "****-****-****-XXXX" format for display
func FormatMaskedCard(lastFour string) string {
	if lastFour == "" {
		return ""
	}
	return "****-****-****-" + lastFour
}
