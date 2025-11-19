package fixtures

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
)

// SubscriptionBuilder provides fluent API for building test subscriptions.
type SubscriptionBuilder struct {
	subscription *sqlc.Subscription
}

// NewSubscription creates a new subscription builder with sensible defaults.
func NewSubscription() *SubscriptionBuilder {
	now := time.Now()
	return &SubscriptionBuilder{
		subscription: &sqlc.Subscription{
			ID:                uuid.New(),
			MerchantID:        uuid.New(),
			CustomerID:        uuid.New(),
			AmountCents:       2999, // $29.99
			Currency:          "USD",
			IntervalValue:     1,
			IntervalUnit:      "month",
			Status:            "active",
			PaymentMethodID:   uuid.New(),
			NextBillingDate:   pgtype.Date{Time: now.AddDate(0, 1, 0), Valid: true},
			FailureRetryCount: 0,
			MaxRetries:        3,
			Metadata:          []byte("{}"),
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}
}

func (b *SubscriptionBuilder) WithID(id uuid.UUID) *SubscriptionBuilder {
	b.subscription.ID = id
	return b
}

func (b *SubscriptionBuilder) WithMerchantID(merchantID uuid.UUID) *SubscriptionBuilder {
	b.subscription.MerchantID = merchantID
	return b
}

func (b *SubscriptionBuilder) WithCustomerID(customerID uuid.UUID) *SubscriptionBuilder {
	b.subscription.CustomerID = customerID
	return b
}

func (b *SubscriptionBuilder) WithAmountCents(amountCents int64) *SubscriptionBuilder {
	b.subscription.AmountCents = amountCents
	return b
}

func (b *SubscriptionBuilder) WithCurrency(currency string) *SubscriptionBuilder {
	b.subscription.Currency = currency
	return b
}

func (b *SubscriptionBuilder) WithInterval(value int32, unit string) *SubscriptionBuilder {
	b.subscription.IntervalValue = value
	b.subscription.IntervalUnit = unit
	return b
}

func (b *SubscriptionBuilder) WithStatus(status string) *SubscriptionBuilder {
	b.subscription.Status = status
	return b
}

func (b *SubscriptionBuilder) Active() *SubscriptionBuilder {
	b.subscription.Status = "active"
	return b
}

func (b *SubscriptionBuilder) Paused() *SubscriptionBuilder {
	b.subscription.Status = "paused"
	return b
}

func (b *SubscriptionBuilder) Cancelled() *SubscriptionBuilder {
	b.subscription.Status = "cancelled"
	b.subscription.CancelledAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return b
}

func (b *SubscriptionBuilder) PastDue() *SubscriptionBuilder {
	b.subscription.Status = "past_due"
	return b
}

func (b *SubscriptionBuilder) WithPaymentMethodID(paymentMethodID uuid.UUID) *SubscriptionBuilder {
	b.subscription.PaymentMethodID = paymentMethodID
	return b
}

func (b *SubscriptionBuilder) WithNextBillingDate(date time.Time) *SubscriptionBuilder {
	b.subscription.NextBillingDate = pgtype.Date{Time: date, Valid: true}
	return b
}

func (b *SubscriptionBuilder) WithFailureRetryCount(count int32) *SubscriptionBuilder {
	b.subscription.FailureRetryCount = count
	return b
}

func (b *SubscriptionBuilder) WithMaxRetries(maxRetries int32) *SubscriptionBuilder {
	b.subscription.MaxRetries = maxRetries
	return b
}

func (b *SubscriptionBuilder) Build() sqlc.Subscription {
	return *b.subscription
}

// Convenience functions for common subscription scenarios

// ActiveSubscription creates an active subscription.
func ActiveSubscription(merchantID, customerID, paymentMethodID uuid.UUID) sqlc.Subscription {
	return NewSubscription().
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		WithPaymentMethodID(paymentMethodID).
		Active().
		Build()
}

// CancelledSubscription creates a cancelled subscription.
func CancelledSubscription(merchantID, customerID, paymentMethodID uuid.UUID) sqlc.Subscription {
	return NewSubscription().
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		WithPaymentMethodID(paymentMethodID).
		Cancelled().
		Build()
}

// PastDueSubscription creates a past due subscription with failure count.
func PastDueSubscription(merchantID, customerID, paymentMethodID uuid.UUID, failureCount int32) sqlc.Subscription {
	return NewSubscription().
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		WithPaymentMethodID(paymentMethodID).
		PastDue().
		WithFailureRetryCount(failureCount).
		Build()
}
