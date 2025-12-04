package domain

import (
	"strconv"
	"time"

	"github.com/kevin07696/payment-service/pkg/timeutil"
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

// CancellationReason represents why a subscription was cancelled
type CancellationReason string

const (
	CancellationReasonUserRequested     CancellationReason = "user_requested"
	CancellationReasonPaymentFailed     CancellationReason = "payment_failed"
	CancellationReasonGracePeriodExpired CancellationReason = "grace_period_expired"
	CancellationReasonAdmin             CancellationReason = "admin"
)

// Subscription represents a recurring billing subscription
type Subscription struct {
	NextBillingDate       time.Time              `json:"next_billing_date"`
	UpdatedAt             time.Time              `json:"updated_at"`
	CreatedAt             time.Time              `json:"created_at"`
	CancelledAt           *time.Time             `json:"cancelled_at"`
	PastDueSince          *time.Time             `json:"past_due_since,omitempty"`
	Metadata              map[string]interface{} `json:"metadata"`
	GatewaySubscriptionID *string                `json:"gateway_subscription_id"`
	CancellationReason    *string                `json:"cancellation_reason,omitempty"`
	Currency              string                 `json:"currency"`
	Status                SubscriptionStatus     `json:"status"`
	IntervalUnit          IntervalUnit           `json:"interval_unit"`
	PaymentMethodID       string                 `json:"payment_method_id"`
	ID                    string                 `json:"id"`
	CustomerID            string                 `json:"customer_id"`
	MerchantID            string                 `json:"merchant_id"`
	IntervalValue         int                    `json:"interval_value"`
	FailureRetryCount     int                    `json:"failure_retry_count"`
	MaxRetries            int                    `json:"max_retries"`
	GracePeriodDays       int                    `json:"grace_period_days"`
	AmountCents           int64                  `json:"amount_cents"`
}

// IsActive returns true if the subscription is currently active
func (s *Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive
}

// IsCancelled returns true if the subscription has been cancelled
func (s *Subscription) IsCancelled() bool {
	return s.Status == SubscriptionStatusCancelled || s.CancelledAt != nil
}

// IsPastDue returns true if the subscription is past due
func (s *Subscription) IsPastDue() bool {
	return s.Status == SubscriptionStatusPastDue
}

// IsGracePeriodExpired returns true if the subscription is past due and grace period has expired
func (s *Subscription) IsGracePeriodExpired() bool {
	if s.Status != SubscriptionStatusPastDue || s.PastDueSince == nil {
		return false
	}
	gracePeriodEnd := s.PastDueSince.AddDate(0, 0, s.GracePeriodDays)
	return timeutil.Now().After(gracePeriodEnd)
}

// CanBeBilled returns true if the subscription is due for billing
func (s *Subscription) CanBeBilled() bool {
	return s.IsActive() && timeutil.Now().After(s.NextBillingDate)
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

	var next time.Time
	switch s.IntervalUnit {
	case IntervalUnitDay:
		next = current.AddDate(0, 0, s.IntervalValue)
	case IntervalUnitWeek:
		next = current.AddDate(0, 0, s.IntervalValue*7)
	case IntervalUnitMonth:
		next = current.AddDate(0, s.IntervalValue, 0)
	case IntervalUnitYear:
		next = current.AddDate(s.IntervalValue, 0, 0)
	default:
		next = current.AddDate(0, s.IntervalValue, 0) // Default to months
	}

	// Ensure the result is in UTC
	return timeutil.ToUTC(next)
}

// GetIntervalDescription returns a human-readable interval description
func (s *Subscription) GetIntervalDescription() string {
	if s.IntervalValue == 1 {
		return string(s.IntervalUnit)
	}
	return strconv.Itoa(s.IntervalValue) + " " + string(s.IntervalUnit) + "s"
}
