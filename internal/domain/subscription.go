package domain

import (
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// SubscriptionStatus represents the subscription state
type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusPaused    SubscriptionStatus = "paused"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
)

// IntervalUnit defines the time unit for billing intervals
type IntervalUnit string

const (
	IntervalUnitDay   IntervalUnit = "day"
	IntervalUnitWeek  IntervalUnit = "week"
	IntervalUnitMonth IntervalUnit = "month"
	IntervalUnitYear  IntervalUnit = "year"
)

// Subscription represents a recurring billing subscription
type Subscription struct {
	// Identity
	ID string `json:"id"` // UUID

	// Multi-tenant
	AgentID string `json:"agent_id"`

	// Customer
	CustomerID string `json:"customer_id"`

	// Billing details
	Amount   decimal.Decimal `json:"amount"`
	Currency string          `json:"currency"` // ISO 4217 code

	// Billing interval (e.g., 1 month, 2 weeks, 3 months)
	IntervalValue int          `json:"interval_value"` // 1, 2, 3, etc.
	IntervalUnit  IntervalUnit `json:"interval_unit"`  // day, week, month, year

	// Status and dates
	Status          SubscriptionStatus `json:"status"`
	NextBillingDate time.Time          `json:"next_billing_date"`

	// Payment method (must be a saved payment method)
	PaymentMethodID string `json:"payment_method_id"` // UUID reference

	// Gateway reference
	GatewaySubscriptionID *string `json:"gateway_subscription_id"` // EPX subscription ID (if applicable)

	// Failure handling
	FailureRetryCount int `json:"failure_retry_count"`
	MaxRetries        int `json:"max_retries"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CancelledAt *time.Time `json:"cancelled_at"`
}

// IsActive returns true if the subscription is currently active
func (s *Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive
}

// IsCancelled returns true if the subscription has been cancelled
func (s *Subscription) IsCancelled() bool {
	return s.Status == SubscriptionStatusCancelled || s.CancelledAt != nil
}

// CanBeBilled returns true if the subscription is due for billing
func (s *Subscription) CanBeBilled() bool {
	return s.IsActive() && time.Now().After(s.NextBillingDate)
}

// ShouldRetry returns true if the subscription should retry after a failed billing
func (s *Subscription) ShouldRetry() bool {
	return s.FailureRetryCount < s.MaxRetries
}

// IncrementRetryCount increments the failure retry counter
func (s *Subscription) IncrementRetryCount() {
	s.FailureRetryCount++
	if s.FailureRetryCount >= s.MaxRetries {
		s.Status = SubscriptionStatusPastDue
	}
}

// ResetRetryCount resets the failure retry counter after successful billing
func (s *Subscription) ResetRetryCount() {
	s.FailureRetryCount = 0
}

// CalculateNextBillingDate calculates the next billing date based on interval
func (s *Subscription) CalculateNextBillingDate() time.Time {
	current := s.NextBillingDate

	switch s.IntervalUnit {
	case IntervalUnitDay:
		return current.AddDate(0, 0, s.IntervalValue)
	case IntervalUnitWeek:
		return current.AddDate(0, 0, s.IntervalValue*7)
	case IntervalUnitMonth:
		return current.AddDate(0, s.IntervalValue, 0)
	case IntervalUnitYear:
		return current.AddDate(s.IntervalValue, 0, 0)
	default:
		return current.AddDate(0, s.IntervalValue, 0) // Default to months
	}
}

// GetIntervalDescription returns a human-readable interval description
func (s *Subscription) GetIntervalDescription() string {
	if s.IntervalValue == 1 {
		return string(s.IntervalUnit)
	}
	return strconv.Itoa(s.IntervalValue) + " " + string(s.IntervalUnit) + "s"
}
