package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// BillingFrequency represents how often a subscription is billed
type BillingFrequency string

const (
	FrequencyWeekly   BillingFrequency = "weekly"
	FrequencyBiWeekly BillingFrequency = "biweekly"
	FrequencyMonthly  BillingFrequency = "monthly"
	FrequencyYearly   BillingFrequency = "yearly"
)

// SubscriptionStatus represents the current state of a subscription
type SubscriptionStatus string

const (
	SubStatusActive    SubscriptionStatus = "active"
	SubStatusPaused    SubscriptionStatus = "paused"
	SubStatusCancelled SubscriptionStatus = "cancelled"
	SubStatusExpired   SubscriptionStatus = "expired"
)

// FailureOption determines what happens when a subscription payment fails
type FailureOption string

const (
	FailureForward FailureOption = "forward" // Move billing date forward
	FailureSkip    FailureOption = "skip"    // Skip this billing cycle
	FailurePause   FailureOption = "pause"   // Pause subscription
)

// Subscription represents a recurring billing subscription
type Subscription struct {
	ID                    string
	MerchantID            string
	CustomerID            string
	Amount                decimal.Decimal
	Currency              string
	Frequency             BillingFrequency
	Status                SubscriptionStatus
	PaymentMethodToken    string
	NextBillingDate       time.Time
	FailureRetryCount     int
	MaxRetries            int
	FailureOption         FailureOption
	GatewaySubscriptionID string
	Metadata              map[string]string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	CancelledAt           *time.Time
}
