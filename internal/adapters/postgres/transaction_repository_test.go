package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Use test database URL from environment or default
	dbURL := "postgres://postgres:postgres@localhost:5432/payment_service_test?sslmode=disable"

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("Could not connect to test database: %v", err)
		return nil, nil
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Could not ping test database: %v", err)
		return nil, nil
	}

	cleanup := func() {
		// Clean up test data
		_, _ = pool.Exec(ctx, "TRUNCATE transactions, subscriptions, audit_logs CASCADE")
		pool.Close()
	}

	return pool, cleanup
}

func TestTransactionRepository_Create(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewTransactionRepository(dbExecutor)

	t.Run("creates transaction successfully", func(t *testing.T) {
		tx := &models.Transaction{
			ID:                uuid.New().String(),
			MerchantID:        "MERCH123",
			CustomerID:        "CUST456",
			Amount:            decimal.NewFromFloat(99.99),
			Currency:          "USD",
			Status:            models.StatusPending,
			Type:              models.TypeSale,
			PaymentMethodType: models.PaymentMethodCreditCard,
			IdempotencyKey:    uuid.New().String(),
			Metadata:          map[string]string{"source": "api", "version": "v1"},
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		err := repo.Create(ctx, nil, tx)
		require.NoError(t, err)

		// Verify it was created
		retrieved, err := repo.GetByID(ctx, nil, uuid.MustParse(tx.ID))
		require.NoError(t, err)
		assert.Equal(t, tx.MerchantID, retrieved.MerchantID)
		assert.Equal(t, tx.Amount.String(), retrieved.Amount.String())
		assert.Equal(t, tx.Status, retrieved.Status)
	})

	t.Run("creates transaction within explicit transaction", func(t *testing.T) {
		var createdTx *models.Transaction

		err := dbExecutor.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
			transaction := &models.Transaction{
				ID:                uuid.New().String(),
				MerchantID:        "MERCH789",
				Amount:            decimal.NewFromFloat(150.00),
				Currency:          "USD",
				Status:            models.StatusAuthorized,
				Type:              models.TypeAuthorization,
				PaymentMethodType: models.PaymentMethodCreditCard,
				Metadata:          map[string]string{},
			}

			if err := repo.Create(ctx, tx, transaction); err != nil {
				return err
			}

			createdTx = transaction
			return nil
		})

		require.NoError(t, err)

		// Verify transaction was committed
		retrieved, err := repo.GetByID(ctx, nil, uuid.MustParse(createdTx.ID))
		require.NoError(t, err)
		assert.Equal(t, createdTx.MerchantID, retrieved.MerchantID)
	})
}

func TestTransactionRepository_GetByIdempotencyKey(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewTransactionRepository(dbExecutor)

	idempotencyKey := uuid.New().String()
	tx := &models.Transaction{
		ID:             uuid.New().String(),
		MerchantID:     "MERCH123",
		Amount:         decimal.NewFromFloat(75.50),
		Currency:       "USD",
		Status:         models.StatusPending,
		Type:           models.TypeSale,
		PaymentMethodType: models.PaymentMethodCreditCard,
		IdempotencyKey: idempotencyKey,
		Metadata:       map[string]string{},
	}

	err := repo.Create(ctx, nil, tx)
	require.NoError(t, err)

	// Retrieve by idempotency key
	retrieved, err := repo.GetByIdempotencyKey(ctx, nil, idempotencyKey)
	require.NoError(t, err)
	assert.Equal(t, tx.ID, retrieved.ID)
	assert.Equal(t, tx.Amount.String(), retrieved.Amount.String())
}

func TestTransactionRepository_UpdateStatus(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewTransactionRepository(dbExecutor)

	// Create initial transaction
	tx := &models.Transaction{
		ID:                uuid.New().String(),
		MerchantID:        "MERCH123",
		Amount:            decimal.NewFromFloat(200.00),
		Currency:          "USD",
		Status:            models.StatusPending,
		Type:              models.TypeSale,
		PaymentMethodType: models.PaymentMethodCreditCard,
		Metadata:          map[string]string{},
	}

	err := repo.Create(ctx, nil, tx)
	require.NoError(t, err)

	// Update status
	gatewayTxnID := "GW-12345"
	responseCode := "00"
	responseMsg := "Approved"

	err = repo.UpdateStatus(ctx, nil, uuid.MustParse(tx.ID), models.StatusCaptured,
		&gatewayTxnID, &responseCode, &responseMsg)
	require.NoError(t, err)

	// Verify update
	updated, err := repo.GetByID(ctx, nil, uuid.MustParse(tx.ID))
	require.NoError(t, err)
	assert.Equal(t, models.StatusCaptured, updated.Status)
	assert.Equal(t, gatewayTxnID, updated.GatewayTransactionID)
	assert.Equal(t, responseCode, updated.GatewayResponseCode)
	assert.Equal(t, responseMsg, updated.GatewayResponseMsg)
}

func TestTransactionRepository_ListByMerchant(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewTransactionRepository(dbExecutor)

	merchantID := "MERCH999"

	// Create multiple transactions
	for i := 0; i < 5; i++ {
		tx := &models.Transaction{
			ID:                uuid.New().String(),
			MerchantID:        merchantID,
			Amount:            decimal.NewFromInt(int64(100 + i*10)),
			Currency:          "USD",
			Status:            models.StatusPending,
			Type:              models.TypeSale,
			PaymentMethodType: models.PaymentMethodCreditCard,
			Metadata:          map[string]string{},
		}
		err := repo.Create(ctx, nil, tx)
		require.NoError(t, err)
	}

	// List transactions
	transactions, err := repo.ListByMerchant(ctx, nil, merchantID, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(transactions), 5)

	// Test pagination
	page1, err := repo.ListByMerchant(ctx, nil, merchantID, 2, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 2)

	page2, err := repo.ListByMerchant(ctx, nil, merchantID, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 2)

	// Pages should be different
	assert.NotEqual(t, page1[0].ID, page2[0].ID)
}

func TestTransactionRepository_ListByCustomer(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	if pool == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	dbExecutor := postgres.NewDBExecutor(pool)
	repo := postgres.NewTransactionRepository(dbExecutor)

	merchantID := "MERCH888"
	customerID := "CUST777"

	// Create transactions for specific customer
	for i := 0; i < 3; i++ {
		tx := &models.Transaction{
			ID:                uuid.New().String(),
			MerchantID:        merchantID,
			CustomerID:        customerID,
			Amount:            decimal.NewFromInt(int64(50 + i*5)),
			Currency:          "USD",
			Status:            models.StatusCaptured,
			Type:              models.TypeSale,
			PaymentMethodType: models.PaymentMethodCreditCard,
			Metadata:          map[string]string{},
		}
		err := repo.Create(ctx, nil, tx)
		require.NoError(t, err)
	}

	// List customer transactions
	transactions, err := repo.ListByCustomer(ctx, nil, merchantID, customerID, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(transactions), 3)

	// All should belong to the customer
	for _, tx := range transactions {
		assert.Equal(t, customerID, tx.CustomerID)
		assert.Equal(t, merchantID, tx.MerchantID)
	}
}
