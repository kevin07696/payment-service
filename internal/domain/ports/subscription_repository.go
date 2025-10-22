package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/domain/models"
)

// SubscriptionRepository defines the interface for subscription persistence
type SubscriptionRepository interface {
	// Create creates a new subscription
	Create(ctx context.Context, tx DBTX, subscription *models.Subscription) error

	// GetByID retrieves a subscription by its ID
	GetByID(ctx context.Context, db DBTX, id uuid.UUID) (*models.Subscription, error)

	// Update updates subscription fields
	Update(ctx context.Context, tx DBTX, subscription *models.Subscription) error

	// ListByCustomer lists subscriptions for a customer
	ListByCustomer(ctx context.Context, db DBTX, merchantID, customerID string) ([]*models.Subscription, error)

	// ListActiveSubscriptionsDueForBilling lists active subscriptions that need billing
	ListActiveSubscriptionsDueForBilling(ctx context.Context, db DBTX, dueDate time.Time, limit int32) ([]*models.Subscription, error)
}
