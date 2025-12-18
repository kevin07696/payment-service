package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// WebhookEventType represents the type of webhook event
type WebhookEventType string

const (
	// Subscription events
	WebhookEventSubscriptionCreated   WebhookEventType = "subscription.created"
	WebhookEventSubscriptionUpdated   WebhookEventType = "subscription.updated"
	WebhookEventSubscriptionCancelled WebhookEventType = "subscription.cancelled"
	WebhookEventSubscriptionPastDue   WebhookEventType = "subscription.past_due"

	// Payment events
	WebhookEventPaymentSucceeded WebhookEventType = "payment.succeeded"
	WebhookEventPaymentFailed    WebhookEventType = "payment.failed"
	WebhookEventPaymentRefunded  WebhookEventType = "payment.refunded"
)

// WebhookEvent represents an event to be delivered via webhook
type WebhookEvent struct {
	EventType  WebhookEventType       `json:"event_type"`
	MerchantID string                 `json:"merchant_id"`
	Data       map[string]interface{} `json:"data"`
	Timestamp  time.Time              `json:"timestamp"`
}

// WebhookService defines the interface for webhook delivery operations
type WebhookService interface {
	// DeliverEvent delivers a webhook event to all subscribed endpoints
	DeliverEvent(ctx context.Context, event *WebhookEvent) error

	// SendSubscriptionCancelled sends a subscription.cancelled webhook event
	SendSubscriptionCancelled(ctx context.Context, subscriptionID uuid.UUID, merchantID string, reason string) error

	// SendSubscriptionPastDue sends a subscription.past_due webhook event
	SendSubscriptionPastDue(ctx context.Context, subscriptionID uuid.UUID, merchantID string) error

	// SendPaymentSucceeded sends a payment.succeeded webhook event
	SendPaymentSucceeded(ctx context.Context, transactionID uuid.UUID, merchantID string, amountCents int64) error

	// SendPaymentFailed sends a payment.failed webhook event
	SendPaymentFailed(ctx context.Context, transactionID uuid.UUID, merchantID string, reason string) error
}
