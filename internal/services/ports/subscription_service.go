package ports

import (
	"context"
	"time"

	"github.com/kevin07696/payment-service/internal/domain"
)

// CreateSubscriptionRequest contains parameters for creating a subscription
type CreateSubscriptionRequest struct {
	MerchantID      string
	CustomerID      string
	Amount          string
	Currency        string
	IntervalValue   int
	IntervalUnit    domain.IntervalUnit
	PaymentMethodID string
	StartDate       time.Time
	MaxRetries      int
	Metadata        map[string]interface{}
	IdempotencyKey  *string
}

// UpdateSubscriptionRequest contains parameters for updating a subscription
type UpdateSubscriptionRequest struct {
	SubscriptionID  string
	Amount          *string
	IntervalValue   *int
	IntervalUnit    *domain.IntervalUnit
	PaymentMethodID *string
	IdempotencyKey  *string
}

// CancelSubscriptionRequest contains parameters for canceling a subscription
type CancelSubscriptionRequest struct {
	SubscriptionID    string
	CancelAtPeriodEnd bool
	Reason            string
	IdempotencyKey    *string
}

// SubscriptionService defines the port for subscription operations
type SubscriptionService interface {
	// CreateSubscription creates a new recurring billing subscription
	CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*domain.Subscription, error)

	// UpdateSubscription updates subscription properties
	UpdateSubscription(ctx context.Context, req *UpdateSubscriptionRequest) (*domain.Subscription, error)

	// CancelSubscription cancels an active subscription
	CancelSubscription(ctx context.Context, req *CancelSubscriptionRequest) (*domain.Subscription, error)

	// PauseSubscription pauses an active subscription
	PauseSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error)

	// ResumeSubscription resumes a paused subscription
	ResumeSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error)

	// GetSubscription retrieves subscription details
	GetSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error)

	// ListCustomerSubscriptions lists all subscriptions for a customer
	ListCustomerSubscriptions(ctx context.Context, agentID, customerID string) ([]*domain.Subscription, error)

	// ProcessDueBilling processes subscriptions due for billing (cron/admin)
	ProcessDueBilling(ctx context.Context, asOfDate time.Time, batchSize int) (processed, success, failed int, errors []error)
}
