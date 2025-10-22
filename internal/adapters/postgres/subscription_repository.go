package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
)

// SubscriptionRepository implements ports.SubscriptionRepository using SQLC
type SubscriptionRepository struct {
	queries *sqlc.Queries
}

// NewSubscriptionRepository creates a new subscription repository
func NewSubscriptionRepository(db ports.DBPort) *SubscriptionRepository {
	return &SubscriptionRepository{
		queries: sqlc.New(db.GetDB()),
	}
}

// Create creates a new subscription
func (r *SubscriptionRepository) Create(ctx context.Context, tx ports.DBTX, subscription *models.Subscription) error {
	var q *sqlc.Queries
	if tx != nil {
		q = sqlc.New(tx)
	} else {
		q = r.queries
	}

	// Parse UUID from string
	subID, err := uuid.Parse(subscription.ID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Convert metadata map to JSON bytes
	var metadataBytes []byte
	if subscription.Metadata != nil {
		metadataBytes, err = json.Marshal(subscription.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	} else {
		metadataBytes = []byte("{}")
	}

	// Convert amount to pgtype.Numeric
	amount := pgtype.Numeric{}
	if err := amount.Scan(subscription.Amount.String()); err != nil {
		return fmt.Errorf("convert amount: %w", err)
	}

	err = q.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
		ID:                    subID,
		MerchantID:            subscription.MerchantID,
		CustomerID:            subscription.CustomerID,
		Amount:                amount,
		Currency:              subscription.Currency,
		Frequency:             string(subscription.Frequency),
		Status:                string(subscription.Status),
		PaymentMethodToken:    subscription.PaymentMethodToken,
		NextBillingDate:       pgtype.Date{Time: subscription.NextBillingDate, Valid: true},
		MaxRetries:            int32(subscription.MaxRetries),
		FailureOption:         string(subscription.FailureOption),
		GatewaySubscriptionID: nullText(subscription.GatewaySubscriptionID),
		Metadata:              metadataBytes,
	})

	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}

	return nil
}

// GetByID retrieves a subscription by its ID
func (r *SubscriptionRepository) GetByID(ctx context.Context, db ports.DBTX, id uuid.UUID) (*models.Subscription, error) {
	var q *sqlc.Queries
	if db != nil {
		q = sqlc.New(db)
	} else {
		q = r.queries
	}

	row, err := q.GetSubscriptionByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get subscription by id: %w", err)
	}

	return r.toDomainModel(row)
}

// Update updates subscription fields
func (r *SubscriptionRepository) Update(ctx context.Context, tx ports.DBTX, subscription *models.Subscription) error {
	var q *sqlc.Queries
	if tx != nil {
		q = sqlc.New(tx)
	} else {
		q = r.queries
	}

	// Parse UUID from string
	subID, err := uuid.Parse(subscription.ID)
	if err != nil {
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Convert amount to pgtype.Numeric
	amount := pgtype.Numeric{}
	if err := amount.Scan(subscription.Amount.String()); err != nil {
		return fmt.Errorf("convert amount: %w", err)
	}

	var cancelledAt pgtype.Timestamptz
	if subscription.CancelledAt != nil && !subscription.CancelledAt.IsZero() {
		cancelledAt = pgtype.Timestamptz{Time: *subscription.CancelledAt, Valid: true}
	}

	err = q.UpdateSubscription(ctx, sqlc.UpdateSubscriptionParams{
		ID:                    subID,
		Amount:                amount,
		Frequency:             pgtype.Text{String: string(subscription.Frequency), Valid: true},
		Status:                pgtype.Text{String: string(subscription.Status), Valid: true},
		PaymentMethodToken:    pgtype.Text{String: subscription.PaymentMethodToken, Valid: true},
		NextBillingDate:       pgtype.Date{Time: subscription.NextBillingDate, Valid: true},
		FailureRetryCount:     pgtype.Int4{Int32: int32(subscription.FailureRetryCount), Valid: true},
		GatewaySubscriptionID: nullText(subscription.GatewaySubscriptionID),
		CancelledAt:           cancelledAt,
	})

	if err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}

	return nil
}

// ListByCustomer lists subscriptions for a customer
func (r *SubscriptionRepository) ListByCustomer(ctx context.Context, db ports.DBTX, merchantID, customerID string) ([]*models.Subscription, error) {
	var q *sqlc.Queries
	if db != nil {
		q = sqlc.New(db)
	} else {
		q = r.queries
	}

	rows, err := q.ListSubscriptionsByCustomer(ctx, sqlc.ListSubscriptionsByCustomerParams{
		MerchantID: merchantID,
		CustomerID: customerID,
	})
	if err != nil {
		return nil, fmt.Errorf("list subscriptions by customer: %w", err)
	}

	subscriptions := make([]*models.Subscription, len(rows))
	for i, row := range rows {
		sub, err := r.toDomainModel(row)
		if err != nil {
			return nil, fmt.Errorf("convert subscription %d: %w", i, err)
		}
		subscriptions[i] = sub
	}

	return subscriptions, nil
}

// ListActiveSubscriptionsDueForBilling lists active subscriptions that need billing
func (r *SubscriptionRepository) ListActiveSubscriptionsDueForBilling(ctx context.Context, db ports.DBTX, dueDate time.Time, limit int32) ([]*models.Subscription, error) {
	var q *sqlc.Queries
	if db != nil {
		q = sqlc.New(db)
	} else {
		q = r.queries
	}

	rows, err := q.ListActiveSubscriptionsDueForBilling(ctx, sqlc.ListActiveSubscriptionsDueForBillingParams{
		NextBillingDate: pgtype.Date{Time: dueDate, Valid: true},
		Limit:           limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list active subscriptions due for billing: %w", err)
	}

	subscriptions := make([]*models.Subscription, len(rows))
	for i, row := range rows {
		sub, err := r.toDomainModel(row)
		if err != nil {
			return nil, fmt.Errorf("convert subscription %d: %w", i, err)
		}
		subscriptions[i] = sub
	}

	return subscriptions, nil
}

// toDomainModel converts a SQLC subscription to a domain model
func (r *SubscriptionRepository) toDomainModel(row sqlc.Subscription) (*models.Subscription, error) {
	// Convert metadata bytes to map
	var metadata map[string]string
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	// Convert amount from pgtype.Numeric to decimal.Decimal
	amount, err := pgNumericToDecimal(row.Amount)
	if err != nil {
		return nil, fmt.Errorf("convert amount: %w", err)
	}

	var cancelledAt *time.Time
	if row.CancelledAt.Valid {
		cancelledAt = &row.CancelledAt.Time
	}

	return &models.Subscription{
		ID:                    row.ID.String(),
		MerchantID:            row.MerchantID,
		CustomerID:            row.CustomerID,
		Amount:                amount,
		Currency:              row.Currency,
		Frequency:             models.BillingFrequency(row.Frequency),
		Status:                models.SubscriptionStatus(row.Status),
		PaymentMethodToken:    row.PaymentMethodToken,
		NextBillingDate:       row.NextBillingDate.Time,
		FailureRetryCount:     int(row.FailureRetryCount),
		MaxRetries:            int(row.MaxRetries),
		FailureOption:         models.FailureOption(row.FailureOption),
		GatewaySubscriptionID: row.GatewaySubscriptionID.String,
		Metadata:              metadata,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
		CancelledAt:           cancelledAt,
	}, nil
}

