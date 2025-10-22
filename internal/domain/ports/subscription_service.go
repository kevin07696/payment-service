package ports

import (
	"context"
	"time"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/shopspring/decimal"
)

// SubscriptionService defines the business logic for subscription operations
type SubscriptionService interface {
	// CreateSubscription creates a new recurring billing subscription
	CreateSubscription(ctx context.Context, req ServiceCreateSubscriptionRequest) (*ServiceSubscriptionResponse, error)

	// UpdateSubscription updates subscription properties
	UpdateSubscription(ctx context.Context, req ServiceUpdateSubscriptionRequest) (*ServiceSubscriptionResponse, error)

	// CancelSubscription cancels an active subscription
	CancelSubscription(ctx context.Context, req ServiceCancelSubscriptionRequest) (*ServiceSubscriptionResponse, error)

	// PauseSubscription pauses an active subscription
	PauseSubscription(ctx context.Context, subscriptionID string) (*ServiceSubscriptionResponse, error)

	// ResumeSubscription resumes a paused subscription
	ResumeSubscription(ctx context.Context, subscriptionID string) (*ServiceSubscriptionResponse, error)

	// GetSubscription retrieves a subscription by ID
	GetSubscription(ctx context.Context, subscriptionID string) (*models.Subscription, error)

	// ListCustomerSubscriptions lists all subscriptions for a customer
	ListCustomerSubscriptions(ctx context.Context, merchantID, customerID string) ([]*models.Subscription, error)

	// ProcessDueBilling processes all subscriptions due for billing
	ProcessDueBilling(ctx context.Context, asOfDate time.Time, batchSize int) (*BillingBatchResult, error)
}

// ServiceCreateSubscriptionRequest represents a request to create a subscription
type ServiceCreateSubscriptionRequest struct {
	MerchantID         string
	CustomerID         string
	Amount             decimal.Decimal
	Currency           string
	Frequency          models.BillingFrequency
	PaymentMethodToken string // BRIC token for recurring charges
	StartDate          time.Time
	MaxRetries         int
	FailureOption      models.FailureOption
	Metadata           map[string]string
	IdempotencyKey     string
}

// ServiceUpdateSubscriptionRequest represents a request to update a subscription
type ServiceUpdateSubscriptionRequest struct {
	SubscriptionID     string
	Amount             *decimal.Decimal // Optional: update amount
	Frequency          *models.BillingFrequency
	PaymentMethodToken *string
	IdempotencyKey     string
}

// ServiceCancelSubscriptionRequest represents a request to cancel a subscription
type ServiceCancelSubscriptionRequest struct {
	SubscriptionID string
	CancelAtPeriodEnd bool // If true, cancel after current billing period
	Reason         string
	IdempotencyKey string
}

// ServiceSubscriptionResponse represents the response from a subscription operation
type ServiceSubscriptionResponse struct {
	SubscriptionID        string
	MerchantID            string
	CustomerID            string
	Amount                decimal.Decimal
	Currency              string
	Frequency             models.BillingFrequency
	Status                models.SubscriptionStatus
	PaymentMethodToken    string
	NextBillingDate       time.Time
	GatewaySubscriptionID string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	CancelledAt           *time.Time
}

// BillingBatchResult represents the result of processing a batch of subscriptions
type BillingBatchResult struct {
	ProcessedCount int
	SuccessCount   int
	FailedCount    int
	SkippedCount   int
	Errors         []BillingError
}

// BillingError represents an error during billing processing
type BillingError struct {
	SubscriptionID string
	CustomerID     string
	Error          string
	Retriable      bool
}
