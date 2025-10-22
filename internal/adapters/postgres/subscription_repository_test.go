package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kevin07696/payment-service/internal/adapters/postgres"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: These are integration tests that require a running PostgreSQL database.
// To run these tests, set up a test database and set the DATABASE_URL environment variable:
// export DATABASE_URL="postgres://user:pass@localhost:5432/payment_service_test?sslmode=disable"
// go test -tags=integration ./internal/adapters/postgres/...

func TestSubscriptionRepository_Create(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewSubscriptionRepository(dbExecutor)

	t.Run("creates subscription successfully", func(t *testing.T) {
		sub := &models.Subscription{
			ID:                 uuid.New().String(),
			MerchantID:         "MERCH123",
			CustomerID:         "CUST456",
			Amount:             decimal.NewFromFloat(29.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			Status:             models.SubStatusActive,
			PaymentMethodToken: "tok_abc123",
			NextBillingDate:    time.Now().Add(30 * 24 * time.Hour),
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
			Metadata:           map[string]string{"plan": "premium", "tier": "gold"},
		}

		err := repo.Create(ctx, nil, sub)
		require.NoError(t, err)

		// Verify it was created
		retrieved, err := repo.GetByID(ctx, nil, uuid.MustParse(sub.ID))
		require.NoError(t, err)
		assert.Equal(t, sub.MerchantID, retrieved.MerchantID)
		assert.Equal(t, sub.Amount.String(), retrieved.Amount.String())
		assert.Equal(t, sub.Frequency, retrieved.Frequency)
		assert.Equal(t, sub.Status, retrieved.Status)
	})

	t.Run("creates subscription within transaction", func(t *testing.T) {
		var createdSub *models.Subscription

		err := dbExecutor.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
			subscription := &models.Subscription{
				ID:                 uuid.New().String(),
				MerchantID:         "MERCH789",
				CustomerID:         "CUST789",
				Amount:             decimal.NewFromFloat(49.99),
				Currency:           "USD",
				Frequency:          models.FrequencyMonthly,
				Status:             models.SubStatusActive,
				PaymentMethodToken: "tok_xyz789",
				NextBillingDate:    time.Now().Add(30 * 24 * time.Hour),
				MaxRetries:         3,
				FailureOption:      models.FailureForward,
				Metadata:           map[string]string{},
			}

			if err := repo.Create(ctx, tx, subscription); err != nil {
				return err
			}

			createdSub = subscription
			return nil
		})

		require.NoError(t, err)

		// Verify transaction was committed
		retrieved, err := repo.GetByID(ctx, nil, uuid.MustParse(createdSub.ID))
		require.NoError(t, err)
		assert.Equal(t, createdSub.MerchantID, retrieved.MerchantID)
	})
}

func TestSubscriptionRepository_Update(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewSubscriptionRepository(dbExecutor)

	// Create initial subscription
	sub := &models.Subscription{
		ID:                 uuid.New().String(),
		MerchantID:         "MERCH123",
		CustomerID:         "CUST456",
		Amount:             decimal.NewFromFloat(29.99),
		Currency:           "USD",
		Frequency:          models.FrequencyMonthly,
		Status:             models.SubStatusActive,
		PaymentMethodToken: "tok_abc123",
		NextBillingDate:    time.Now().Add(30 * 24 * time.Hour),
		MaxRetries:         3,
		FailureOption:      models.FailureForward,
		Metadata:           map[string]string{},
	}

	err := repo.Create(ctx, nil, sub)
	require.NoError(t, err)

	// Update subscription
	sub.Amount = decimal.NewFromFloat(39.99)
	sub.Status = models.SubStatusPaused
	cancelTime := time.Now()
	sub.CancelledAt = &cancelTime

	err = repo.Update(ctx, nil, sub)
	require.NoError(t, err)

	// Verify update
	updated, err := repo.GetByID(ctx, nil, uuid.MustParse(sub.ID))
	require.NoError(t, err)
	assert.Equal(t, "39.99", updated.Amount.String())
	assert.Equal(t, models.SubStatusPaused, updated.Status)
	assert.NotNil(t, updated.CancelledAt)
}

func TestSubscriptionRepository_ListByCustomer(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewSubscriptionRepository(dbExecutor)

	merchantID := "MERCH999"
	customerID := "CUST888"

	// Create multiple subscriptions for the customer
	for i := 0; i < 4; i++ {
		sub := &models.Subscription{
			ID:                 uuid.New().String(),
			MerchantID:         merchantID,
			CustomerID:         customerID,
			Amount:             decimal.NewFromInt(int64(10 + i*5)),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			Status:             models.SubStatusActive,
			PaymentMethodToken: "tok_test" + string(rune(i)),
			NextBillingDate:    time.Now().Add(time.Duration(i) * 24 * time.Hour),
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
			Metadata:           map[string]string{},
		}
		err := repo.Create(ctx, nil, sub)
		require.NoError(t, err)
	}

	// List customer subscriptions
	subscriptions, err := repo.ListByCustomer(ctx, nil, merchantID, customerID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(subscriptions), 4)

	// All should belong to the customer
	for _, sub := range subscriptions {
		assert.Equal(t, customerID, sub.CustomerID)
		assert.Equal(t, merchantID, sub.MerchantID)
	}
}

func TestSubscriptionRepository_ListActiveSubscriptionsDueForBilling(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewSubscriptionRepository(dbExecutor)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	// Create subscriptions with different billing dates
	testCases := []struct {
		name            string
		billingDate     time.Time
		status          models.SubscriptionStatus
		shouldBeIncluded bool
	}{
		{"due yesterday", yesterday, models.SubStatusActive, true},
		{"due today", now, models.SubStatusActive, true},
		{"due tomorrow", tomorrow, models.SubStatusActive, false},
		{"paused subscription", yesterday, models.SubStatusPaused, false},
		{"cancelled subscription", yesterday, models.SubStatusCancelled, false},
	}

	for _, tc := range testCases {
		sub := &models.Subscription{
			ID:                 uuid.New().String(),
			MerchantID:         "MERCH777",
			CustomerID:         "CUST" + tc.name,
			Amount:             decimal.NewFromFloat(19.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			Status:             tc.status,
			PaymentMethodToken: "tok_test",
			NextBillingDate:    tc.billingDate,
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
			Metadata:           map[string]string{"test": tc.name},
		}
		err := repo.Create(ctx, nil, sub)
		require.NoError(t, err)
	}

	// Query for subscriptions due for billing
	due, err := repo.ListActiveSubscriptionsDueForBilling(ctx, nil, now, 100)
	require.NoError(t, err)

	// Count how many should be included
	includedCount := 0
	for _, tc := range testCases {
		if tc.shouldBeIncluded {
			includedCount++
		}
	}

	assert.GreaterOrEqual(t, len(due), includedCount)

	// Verify all returned subscriptions are active and due
	for _, sub := range due {
		assert.Equal(t, models.SubStatusActive, sub.Status)
		assert.True(t, sub.NextBillingDate.Before(now) || sub.NextBillingDate.Equal(now))
	}
}

func TestSubscriptionRepository_CancelSubscription(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewSubscriptionRepository(dbExecutor)

	// Create subscription
	sub := &models.Subscription{
		ID:                 uuid.New().String(),
		MerchantID:         "MERCH123",
		CustomerID:         "CUST456",
		Amount:             decimal.NewFromFloat(29.99),
		Currency:           "USD",
		Frequency:          models.FrequencyMonthly,
		Status:             models.SubStatusActive,
		PaymentMethodToken: "tok_abc123",
		NextBillingDate:    time.Now().Add(30 * 24 * time.Hour),
		MaxRetries:         3,
		FailureOption:      models.FailureForward,
		Metadata:           map[string]string{},
	}

	err := repo.Create(ctx, nil, sub)
	require.NoError(t, err)

	// Cancel subscription
	cancelTime := time.Now()
	sub.Status = models.SubStatusCancelled
	sub.CancelledAt = &cancelTime

	err = repo.Update(ctx, nil, sub)
	require.NoError(t, err)

	// Verify cancellation
	cancelled, err := repo.GetByID(ctx, nil, uuid.MustParse(sub.ID))
	require.NoError(t, err)
	assert.Equal(t, models.SubStatusCancelled, cancelled.Status)
	assert.NotNil(t, cancelled.CancelledAt)
	assert.WithinDuration(t, cancelTime, *cancelled.CancelledAt, time.Second)
}
