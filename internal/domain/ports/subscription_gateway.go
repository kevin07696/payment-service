package ports

import (
	"context"
	"time"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/shopspring/decimal"
)

// SubscriptionRequest represents a request to create a subscription
type SubscriptionRequest struct {
	CustomerID      string
	Amount          decimal.Decimal
	Currency        string
	Frequency       models.BillingFrequency
	PaymentToken    string // BRIC token
	BillingInfo     models.BillingInfo
	StartDate       time.Time
	NumberOfPayments int // 0 = infinite
	MaxRetries      int
	FailureOption   models.FailureOption
	Metadata        map[string]string
}

// UpdateSubscriptionRequest represents a request to update a subscription
type UpdateSubscriptionRequest struct {
	Amount          *decimal.Decimal
	Frequency       *models.BillingFrequency
	NextBillingDate *time.Time
	PaymentToken    *string
}

// SubscriptionResult represents the result of a subscription operation
type SubscriptionResult struct {
	SubscriptionID        string
	GatewaySubscriptionID string
	Status                models.SubscriptionStatus
	Amount                decimal.Decimal
	Frequency             models.BillingFrequency
	NextBillingDate       time.Time
	Message               string
}

// RecurringBillingGateway defines the interface for recurring billing operations
type RecurringBillingGateway interface {
	// CreateSubscription creates a new recurring subscription
	CreateSubscription(ctx context.Context, req *SubscriptionRequest) (*SubscriptionResult, error)

	// UpdateSubscription updates an existing subscription
	UpdateSubscription(ctx context.Context, subscriptionID string, req *UpdateSubscriptionRequest) (*SubscriptionResult, error)

	// CancelSubscription cancels a subscription
	CancelSubscription(ctx context.Context, subscriptionID string, immediate bool) (*SubscriptionResult, error)

	// PauseSubscription pauses a subscription
	PauseSubscription(ctx context.Context, subscriptionID string) (*SubscriptionResult, error)

	// ResumeSubscription resumes a paused subscription
	ResumeSubscription(ctx context.Context, subscriptionID string) (*SubscriptionResult, error)

	// GetSubscription retrieves subscription details
	GetSubscription(ctx context.Context, subscriptionID string) (*SubscriptionResult, error)

	// ListSubscriptions lists all subscriptions for a customer
	ListSubscriptions(ctx context.Context, customerID string) ([]*SubscriptionResult, error)

	// ChargePaymentMethod charges a stored payment method one-time
	// This is independent from subscriptions and does not count toward subscription payments
	ChargePaymentMethod(ctx context.Context, paymentMethodID string, amount decimal.Decimal) (*PaymentResult, error)
}
