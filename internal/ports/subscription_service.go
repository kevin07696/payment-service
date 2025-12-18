package ports

import (
	"context"
	"time"

	"github.com/kevin07696/payment-service/internal/domain"
)

// CreateSubscriptionRequest contains parameters for creating a subscription.
type CreateSubscriptionRequest struct {
	MerchantID      string
	CustomerID      string
	AmountCents     int64
	Currency        string
	IntervalValue   int
	IntervalUnit    domain.IntervalUnit
	PaymentMethodID string
	StartDate       time.Time
	MaxRetries      int
	Metadata        map[string]interface{}
	IdempotencyKey  *string
}

// UpdateSubscriptionRequest contains parameters for updating a subscription.
type UpdateSubscriptionRequest struct {
	SubscriptionID  string
	AmountCents     *int64
	IntervalValue   *int
	IntervalUnit    *domain.IntervalUnit
	PaymentMethodID *string
	IdempotencyKey  *string
}

// CancelSubscriptionRequest contains parameters for canceling a subscription.
type CancelSubscriptionRequest struct {
	SubscriptionID    string
	CancelAtPeriodEnd bool
	Reason            string
	IdempotencyKey    *string
}

// ExpiredPastDueResult contains the result of processing expired past_due subscriptions.
type ExpiredPastDueResult struct {
	Processed int     // Total subscriptions processed
	Cancelled int     // Successfully cancelled
	Failed    int     // Failed to cancel
	Errors    []error // Errors encountered
}

// SubscriptionService defines the port for subscription operations.
type SubscriptionService interface {
	CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*domain.Subscription, error)
	UpdateSubscription(ctx context.Context, req *UpdateSubscriptionRequest) (*domain.Subscription, error)
	CancelSubscription(ctx context.Context, req *CancelSubscriptionRequest) (*domain.Subscription, error)
	PauseSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error)
	ResumeSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error)
	GetSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error)
	ListSubscriptions(ctx context.Context, merchantID, customerID string) ([]*domain.Subscription, error)
	ProcessDueBilling(ctx context.Context, asOfDate time.Time, batchSize int) (processed, success, failed int, errors []error)
	// ProcessExpiredPastDue auto-cancels subscriptions whose grace period has expired
	ProcessExpiredPastDue(ctx context.Context, batchSize int) *ExpiredPastDueResult
}
