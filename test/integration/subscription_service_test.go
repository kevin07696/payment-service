package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/adapters/postgres"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	"github.com/kevin07696/payment-service/internal/services/payment"
	"github.com/kevin07696/payment-service/internal/services/subscription"
	"github.com/kevin07696/payment-service/test/integration/testdb"
	"github.com/kevin07696/payment-service/test/mocks"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionService_Integration_Lifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool := testdb.SetupTestDB(t)
	defer testdb.TeardownTestDB(t, pool)

	t.Run("CreateSubscription_WithGateway", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		subRepo := postgres.NewSubscriptionRepository(dbPort)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockCCGateway := mocks.NewMockCreditCardGateway()
		mockRecurringGateway := mocks.NewMockRecurringGateway()
		mockLogger := mocks.NewMockLogger()

		paymentSvc := payment.NewService(dbPort, txRepo, mockCCGateway, mockLogger)

		// Configure mock gateway
		mockRecurringGateway.SetCreateResponse(&ports.SubscriptionResult{
			SubscriptionID:        "gw-sub-001",
			GatewaySubscriptionID: "gw-sub-001",
			Status:                models.SubStatusActive,
			NextBillingDate:       time.Now().AddDate(0, 1, 0),
			Message:               "Subscription created",
		}, nil)

		service := subscription.NewService(dbPort, subRepo, paymentSvc, mockRecurringGateway, mockLogger)

		// Execute
		ctx := context.Background()
		req := ports.ServiceCreateSubscriptionRequest{
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-001",
			Amount:             decimal.NewFromFloat(29.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			PaymentMethodToken: "test-token",
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
		}

		result, err := service.CreateSubscription(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, models.SubStatusActive, result.Status)
		assert.NotEmpty(t, result.SubscriptionID)

		// Verify database persistence
		subUUID, err := uuid.Parse(result.SubscriptionID)
		require.NoError(t, err)

		sub, err := subRepo.GetByID(ctx, pool, subUUID)
		require.NoError(t, err)
		assert.Equal(t, "MERCH-001", sub.MerchantID)
		assert.Equal(t, "CUST-001", sub.CustomerID)
		assert.True(t, decimal.NewFromFloat(29.99).Equal(sub.Amount))
		assert.Equal(t, models.FrequencyMonthly, sub.Frequency)
		assert.Equal(t, models.SubStatusActive, sub.Status)
		assert.Equal(t, "gw-sub-001", sub.GatewaySubscriptionID)
	})

	t.Run("CreateSubscription_WithoutGateway", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		subRepo := postgres.NewSubscriptionRepository(dbPort)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockCCGateway := mocks.NewMockCreditCardGateway()
		mockLogger := mocks.NewMockLogger()

		paymentSvc := payment.NewService(dbPort, txRepo, mockCCGateway, mockLogger)

		// Service without recurring gateway
		service := subscription.NewService(dbPort, subRepo, paymentSvc, nil, mockLogger)

		// Execute
		ctx := context.Background()
		req := ports.ServiceCreateSubscriptionRequest{
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-002",
			Amount:             decimal.NewFromFloat(19.99),
			Currency:           "USD",
			Frequency:          models.FrequencyWeekly,
			PaymentMethodToken: "test-token",
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
		}

		result, err := service.CreateSubscription(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, models.SubStatusActive, result.Status)

		// Verify database
		subUUID, err := uuid.Parse(result.SubscriptionID)
		require.NoError(t, err)

		sub, err := subRepo.GetByID(ctx, pool, subUUID)
		require.NoError(t, err)
		assert.Equal(t, "", sub.GatewaySubscriptionID) // No gateway
	})

	t.Run("UpdateSubscription", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		subRepo := postgres.NewSubscriptionRepository(dbPort)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockCCGateway := mocks.NewMockCreditCardGateway()
		mockRecurringGateway := mocks.NewMockRecurringGateway()
		mockLogger := mocks.NewMockLogger()

		paymentSvc := payment.NewService(dbPort, txRepo, mockCCGateway, mockLogger)

		// Create initial subscription
		mockRecurringGateway.SetCreateResponse(&ports.SubscriptionResult{
			SubscriptionID:        "gw-sub-002",
			GatewaySubscriptionID: "gw-sub-002",
			Status:                models.SubStatusActive,
			NextBillingDate:       time.Now().AddDate(0, 1, 0),
		}, nil)

		service := subscription.NewService(dbPort, subRepo, paymentSvc, mockRecurringGateway, mockLogger)
		ctx := context.Background()

		createReq := ports.ServiceCreateSubscriptionRequest{
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-003",
			Amount:             decimal.NewFromFloat(39.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			PaymentMethodToken: "test-token",
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
		}

		createResult, err := service.CreateSubscription(ctx, createReq)
		require.NoError(t, err)

		// Configure update response
		mockRecurringGateway.SetUpdateResponse(&ports.SubscriptionResult{
			SubscriptionID:        createResult.SubscriptionID,
			GatewaySubscriptionID: "gw-sub-002",
			Status:                models.SubStatusActive,
			NextBillingDate:       time.Now().AddDate(0, 1, 0),
		}, nil)

		// Execute update
		newAmount := decimal.NewFromFloat(49.99)
		newFreq := models.FrequencyYearly

		updateReq := ports.ServiceUpdateSubscriptionRequest{
			SubscriptionID: createResult.SubscriptionID,
			Amount:         &newAmount,
			Frequency:      &newFreq,
		}

		updateResult, err := service.UpdateSubscription(ctx, updateReq)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, updateResult)

		// Verify database update
		subUUID, err := uuid.Parse(createResult.SubscriptionID)
		require.NoError(t, err)

		sub, err := subRepo.GetByID(ctx, pool, subUUID)
		require.NoError(t, err)
		assert.True(t, decimal.NewFromFloat(49.99).Equal(sub.Amount))
		assert.Equal(t, models.FrequencyYearly, sub.Frequency)
	})

	t.Run("CancelSubscription", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		subRepo := postgres.NewSubscriptionRepository(dbPort)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockCCGateway := mocks.NewMockCreditCardGateway()
		mockRecurringGateway := mocks.NewMockRecurringGateway()
		mockLogger := mocks.NewMockLogger()

		paymentSvc := payment.NewService(dbPort, txRepo, mockCCGateway, mockLogger)

		// Create subscription
		mockRecurringGateway.SetCreateResponse(&ports.SubscriptionResult{
			SubscriptionID:        "gw-sub-003",
			GatewaySubscriptionID: "gw-sub-003",
			Status:                models.SubStatusActive,
			NextBillingDate:       time.Now().AddDate(0, 1, 0),
		}, nil)

		service := subscription.NewService(dbPort, subRepo, paymentSvc, mockRecurringGateway, mockLogger)
		ctx := context.Background()

		createResult, err := service.CreateSubscription(ctx, ports.ServiceCreateSubscriptionRequest{
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-004",
			Amount:             decimal.NewFromFloat(9.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			PaymentMethodToken: "test-token",
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
		})
		require.NoError(t, err)

		// Configure cancel response
		mockRecurringGateway.SetCancelResponse(&ports.SubscriptionResult{
			SubscriptionID:        createResult.SubscriptionID,
			GatewaySubscriptionID: "gw-sub-003",
			Status:                models.SubStatusCancelled,
		}, nil)

		// Execute cancel
		cancelReq := ports.ServiceCancelSubscriptionRequest{
			SubscriptionID: createResult.SubscriptionID,
		}

		cancelResult, err := service.CancelSubscription(ctx, cancelReq)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, cancelResult)
		assert.Equal(t, models.SubStatusCancelled, cancelResult.Status)

		// Verify database
		subUUID, err := uuid.Parse(createResult.SubscriptionID)
		require.NoError(t, err)

		sub, err := subRepo.GetByID(ctx, pool, subUUID)
		require.NoError(t, err)
		assert.Equal(t, models.SubStatusCancelled, sub.Status)
		assert.NotNil(t, sub.CancelledAt)
	})
}

func TestSubscriptionService_Integration_ProcessBilling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool := testdb.SetupTestDB(t)
	defer testdb.TeardownTestDB(t, pool)

	t.Run("ProcessDueBilling_Success", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		subRepo := postgres.NewSubscriptionRepository(dbPort)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockCCGateway := mocks.NewMockCreditCardGateway()
		mockLogger := mocks.NewMockLogger()

		// Configure payment gateway to succeed
		mockCCGateway.SetSaleResponse(&ports.PaymentResult{
			GatewayTransactionID: "billing-tx-001",
			Status:               models.StatusCaptured,
			AuthCode:             "BILL001",
			ResponseCode:         "00",
			Message:              "Approved",
		}, nil)

		paymentSvc := payment.NewService(dbPort, txRepo, mockCCGateway, mockLogger)

		// Service without recurring gateway (app-managed billing)
		service := subscription.NewService(dbPort, subRepo, paymentSvc, nil, mockLogger)

		ctx := context.Background()

		// Create subscription due for billing
		createResult, err := service.CreateSubscription(ctx, ports.ServiceCreateSubscriptionRequest{
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-BILLING-001",
			Amount:             decimal.NewFromFloat(15.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			PaymentMethodToken: "test-billing-token",
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
		})
		require.NoError(t, err)

		// Update next billing date to yesterday (due)
		subUUID, err := uuid.Parse(createResult.SubscriptionID)
		require.NoError(t, err)

		sub, err := subRepo.GetByID(ctx, pool, subUUID)
		require.NoError(t, err)
		sub.NextBillingDate = time.Now().AddDate(0, 0, -1).UTC()
		err = subRepo.Update(ctx, pool, sub)
		require.NoError(t, err)

		// Execute batch billing
		batchResult, err := service.ProcessDueBilling(ctx, time.Now(), 100)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, batchResult)
		assert.Equal(t, 1, batchResult.ProcessedCount)
		assert.Equal(t, 1, batchResult.SuccessCount)
		assert.Equal(t, 0, batchResult.FailedCount)

		// Verify subscription updated
		updatedSub, err := subRepo.GetByID(ctx, pool, subUUID)
		require.NoError(t, err)
		// Next billing date should be updated to future
		assert.True(t, updatedSub.NextBillingDate.After(time.Now()))
		assert.Equal(t, 0, updatedSub.FailureRetryCount)

		// Verify transaction created
		transactions, err := txRepo.ListByCustomer(ctx, pool, "MERCH-001", "CUST-BILLING-001", 10, 0)
		require.NoError(t, err)
		assert.Len(t, transactions, 1)
		assert.Equal(t, models.TypeSale, transactions[0].Type)
		assert.Equal(t, models.StatusCaptured, transactions[0].Status)
	})
}
