package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/adapters/postgres"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/test/integration/testdb"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool := testdb.SetupTestDB(t)
	defer testdb.TeardownTestDB(t, pool)

	dbPort := postgres.NewDBExecutor(pool)
	repo := postgres.NewTransactionRepository(dbPort)

	t.Run("CreateAndGet", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)
		ctx := context.Background()

		// Create transaction
		tx := &models.Transaction{
			ID:                uuid.New().String(),
			MerchantID:        "MERCH-001",
			CustomerID:        "CUST-001",
			Amount:            decimal.NewFromFloat(99.99),
			Currency:          "USD",
			Status:            models.StatusPending,
			Type:              models.TypeAuthorization,
			PaymentMethodType: models.PaymentMethodCreditCard,
			PaymentMethodToken: "token-123",
			IdempotencyKey:    "idempotency-" + uuid.New().String(),
			Metadata:          map[string]string{"order_id": "ORD-001"},
		}

		err := repo.Create(ctx, pool, tx)
		require.NoError(t, err)

		// Retrieve transaction
		txID, err := uuid.Parse(tx.ID)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, pool, txID)
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, tx.ID, retrieved.ID)
		assert.Equal(t, tx.MerchantID, retrieved.MerchantID)
		assert.Equal(t, tx.CustomerID, retrieved.CustomerID)
		assert.True(t, tx.Amount.Equal(retrieved.Amount))
		assert.Equal(t, tx.Currency, retrieved.Currency)
		assert.Equal(t, tx.Status, retrieved.Status)
		assert.Equal(t, tx.Type, retrieved.Type)
		assert.Equal(t, tx.IdempotencyKey, retrieved.IdempotencyKey)
		assert.NotZero(t, retrieved.CreatedAt)
		assert.NotZero(t, retrieved.UpdatedAt)
	})

	t.Run("GetByIdempotencyKey", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)
		ctx := context.Background()

		idempotencyKey := "unique-key-" + uuid.New().String()

		tx := &models.Transaction{
			ID:                uuid.New().String(),
			MerchantID:        "MERCH-001",
			Amount:            decimal.NewFromFloat(50.00),
			Currency:          "USD",
			Status:            models.StatusCaptured,
			Type:              models.TypeSale,
			PaymentMethodType: models.PaymentMethodCreditCard,
			IdempotencyKey:    idempotencyKey,
			Metadata:          map[string]string{},
		}

		err := repo.Create(ctx, pool, tx)
		require.NoError(t, err)

		// Retrieve by idempotency key
		retrieved, err := repo.GetByIdempotencyKey(ctx, pool, idempotencyKey)
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, tx.ID, retrieved.ID)
		assert.Equal(t, idempotencyKey, retrieved.IdempotencyKey)
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)
		ctx := context.Background()

		tx := &models.Transaction{
			ID:                uuid.New().String(),
			MerchantID:        "MERCH-001",
			Amount:            decimal.NewFromFloat(100.00),
			Currency:          "USD",
			Status:            models.StatusPending,
			Type:              models.TypeAuthorization,
			PaymentMethodType: models.PaymentMethodCreditCard,
			Metadata:          map[string]string{},
		}

		err := repo.Create(ctx, pool, tx)
		require.NoError(t, err)

		// Update status
		txID, err := uuid.Parse(tx.ID)
		require.NoError(t, err)

		err = repo.UpdateStatus(ctx, pool, txID, models.StatusCaptured, nil, nil, nil)
		require.NoError(t, err)

		// Verify update
		retrieved, err := repo.GetByID(ctx, pool, txID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusCaptured, retrieved.Status)
	})

	t.Run("ListByMerchant", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)
		ctx := context.Background()

		merchantID := "MERCH-002"

		// Create multiple transactions
		for i := 0; i < 5; i++ {
			tx := &models.Transaction{
				ID:                uuid.New().String(),
				MerchantID:        merchantID,
				Amount:            decimal.NewFromFloat(float64(i + 1)),
				Currency:          "USD",
				Status:            models.StatusCaptured,
				Type:              models.TypeSale,
				PaymentMethodType: models.PaymentMethodCreditCard,
				Metadata:          map[string]string{},
			}
			err := repo.Create(ctx, pool, tx)
			require.NoError(t, err)

			// Small delay to ensure different timestamps
			time.Sleep(10 * time.Millisecond)
		}

		// List transactions
		transactions, err := repo.ListByMerchant(ctx, pool, merchantID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, transactions, 5)
		assert.Equal(t, merchantID, transactions[0].MerchantID)
	})

	t.Run("ListByCustomer", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)
		ctx := context.Background()

		customerID := "CUST-003"

		// Create multiple transactions for customer
		for i := 0; i < 3; i++ {
			tx := &models.Transaction{
				ID:                uuid.New().String(),
				MerchantID:        "MERCH-001",
				CustomerID:        customerID,
				Amount:            decimal.NewFromFloat(float64((i + 1) * 10)),
				Currency:          "USD",
				Status:            models.StatusCaptured,
				Type:              models.TypeSale,
				PaymentMethodType: models.PaymentMethodCreditCard,
				Metadata:          map[string]string{},
			}
			err := repo.Create(ctx, pool, tx)
			require.NoError(t, err)
		}

		// List transactions
		transactions, err := repo.ListByCustomer(ctx, pool, "MERCH-001", customerID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, transactions, 3)
		assert.Equal(t, customerID, transactions[0].CustomerID)
	})
}

func TestSubscriptionRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool := testdb.SetupTestDB(t)
	defer testdb.TeardownTestDB(t, pool)

	dbPort := postgres.NewDBExecutor(pool)
	repo := postgres.NewSubscriptionRepository(dbPort)

	t.Run("CreateAndGet", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)
		ctx := context.Background()

		nextBilling := time.Now().AddDate(0, 1, 0).UTC()
		sub := &models.Subscription{
			ID:                 uuid.New().String(),
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-001",
			Amount:             decimal.NewFromFloat(29.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			Status:             models.SubStatusActive,
			PaymentMethodToken: "token-123",
			NextBillingDate:    nextBilling,
			FailureRetryCount:  0,
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
			Metadata:           map[string]string{"plan": "premium"},
		}

		err := repo.Create(ctx, pool, sub)
		require.NoError(t, err)

		// Retrieve subscription
		subID, err := uuid.Parse(sub.ID)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, pool, subID)
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, sub.ID, retrieved.ID)
		assert.Equal(t, sub.MerchantID, retrieved.MerchantID)
		assert.Equal(t, sub.CustomerID, retrieved.CustomerID)
		assert.True(t, sub.Amount.Equal(retrieved.Amount))
		assert.Equal(t, sub.Frequency, retrieved.Frequency)
		assert.Equal(t, sub.Status, retrieved.Status)
		assert.Equal(t, sub.FailureOption, retrieved.FailureOption)
		assert.NotZero(t, retrieved.CreatedAt)
	})

	t.Run("Update", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)
		ctx := context.Background()

		sub := &models.Subscription{
			ID:                 uuid.New().String(),
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-001",
			Amount:             decimal.NewFromFloat(19.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			Status:             models.SubStatusActive,
			PaymentMethodToken: "token-123",
			NextBillingDate:    time.Now().AddDate(0, 1, 0).UTC(),
			FailureRetryCount:  0,
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
			Metadata:           map[string]string{},
		}

		err := repo.Create(ctx, pool, sub)
		require.NoError(t, err)

		// Update subscription
		sub.Amount = decimal.NewFromFloat(39.99)
		sub.Frequency = models.FrequencyYearly
		sub.Status = models.SubStatusPaused

		err = repo.Update(ctx, pool, sub)
		require.NoError(t, err)

		// Verify update
		subID, err := uuid.Parse(sub.ID)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, pool, subID)
		require.NoError(t, err)
		assert.True(t, decimal.NewFromFloat(39.99).Equal(retrieved.Amount))
		assert.Equal(t, models.FrequencyYearly, retrieved.Frequency)
		assert.Equal(t, models.SubStatusPaused, retrieved.Status)
	})

	t.Run("ListByCustomer", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)
		ctx := context.Background()

		customerID := "CUST-004"

		// Create multiple subscriptions
		for i := 0; i < 3; i++ {
			sub := &models.Subscription{
				ID:                 uuid.New().String(),
				MerchantID:         "MERCH-001",
				CustomerID:         customerID,
				Amount:             decimal.NewFromFloat(float64((i + 1) * 10)),
				Currency:           "USD",
				Frequency:          models.FrequencyMonthly,
				Status:             models.SubStatusActive,
				PaymentMethodToken: "token-123",
				NextBillingDate:    time.Now().AddDate(0, 1, 0).UTC(),
				FailureRetryCount:  0,
				MaxRetries:         3,
				FailureOption:      models.FailureForward,
				Metadata:           map[string]string{},
			}
			err := repo.Create(ctx, pool, sub)
			require.NoError(t, err)
		}

		// List subscriptions
		subscriptions, err := repo.ListByCustomer(ctx, pool, "MERCH-001", customerID)
		require.NoError(t, err)
		assert.Len(t, subscriptions, 3)
		assert.Equal(t, customerID, subscriptions[0].CustomerID)
	})

	t.Run("ListActiveSubscriptionsDueForBilling", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)
		ctx := context.Background()

		now := time.Now().UTC()
		yesterday := now.AddDate(0, 0, -1)
		tomorrow := now.AddDate(0, 0, 1)

		// Create subscription due for billing (yesterday)
		subDue := &models.Subscription{
			ID:                 uuid.New().String(),
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-001",
			Amount:             decimal.NewFromFloat(29.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			Status:             models.SubStatusActive,
			PaymentMethodToken: "token-123",
			NextBillingDate:    yesterday,
			FailureRetryCount:  0,
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
			Metadata:           map[string]string{},
		}
		err := repo.Create(ctx, pool, subDue)
		require.NoError(t, err)

		// Create subscription not due yet (tomorrow)
		subNotDue := &models.Subscription{
			ID:                 uuid.New().String(),
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-002",
			Amount:             decimal.NewFromFloat(19.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			Status:             models.SubStatusActive,
			PaymentMethodToken: "token-456",
			NextBillingDate:    tomorrow,
			FailureRetryCount:  0,
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
			Metadata:           map[string]string{},
		}
		err = repo.Create(ctx, pool, subNotDue)
		require.NoError(t, err)

		// List due subscriptions
		dueSubscriptions, err := repo.ListActiveSubscriptionsDueForBilling(ctx, pool, now, 100)
		require.NoError(t, err)
		assert.Len(t, dueSubscriptions, 1)
		assert.Equal(t, subDue.ID, dueSubscriptions[0].ID)
	})
}
