package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/adapters/postgres"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	"github.com/kevin07696/payment-service/internal/services/payment"
	pkgerrors "github.com/kevin07696/payment-service/pkg/errors"
	"github.com/kevin07696/payment-service/test/integration/testdb"
	"github.com/kevin07696/payment-service/test/mocks"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentService_Integration_AuthorizeSale(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool := testdb.SetupTestDB(t)
	defer testdb.TeardownTestDB(t, pool)

	t.Run("Authorize_Success", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockGateway := mocks.NewMockCreditCardGateway()
		mockLogger := mocks.NewMockLogger()

		// Configure mock gateway to return success
		mockGateway.SetAuthorizeResponse(&ports.PaymentResult{
			GatewayTransactionID: "gw-tx-001",
			Status:               models.StatusAuthorized,
			AuthCode:             "AUTH123",
			ResponseCode:         "00",
			Message:              "Approved",
		}, nil)

		service := payment.NewService(dbPort, txRepo, mockGateway, mockLogger)

		// Execute
		ctx := context.Background()
		req := ports.ServiceAuthorizeRequest{
			MerchantID:     "MERCH-001",
			CustomerID:     "CUST-001",
			Amount:         decimal.NewFromFloat(100.00),
			Currency:       "USD",
			Token:          "test-bric-token",
			IdempotencyKey: "test-idempotency-001",
		}

		result, err := service.Authorize(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, models.StatusAuthorized, result.Status)
		assert.NotEmpty(t, result.TransactionID)

		// Verify database persistence
		txUUID, err := uuid.Parse(result.TransactionID)
		require.NoError(t, err)

		tx, err := txRepo.GetByID(ctx, pool, txUUID)
		require.NoError(t, err)
		assert.Equal(t, "MERCH-001", tx.MerchantID)
		assert.Equal(t, "CUST-001", tx.CustomerID)
		assert.True(t, decimal.NewFromFloat(100.00).Equal(tx.Amount))
		assert.Equal(t, models.StatusAuthorized, tx.Status)
		assert.Equal(t, models.TypeAuthorization, tx.Type)
		assert.Equal(t, "test-idempotency-001", tx.IdempotencyKey)
	})

	t.Run("Authorize_IdempotencyCheck", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockGateway := mocks.NewMockCreditCardGateway()
		mockLogger := mocks.NewMockLogger()

		mockGateway.SetAuthorizeResponse(&ports.PaymentResult{
			GatewayTransactionID: "gw-tx-002",
			Status:               models.StatusCaptured,
			AuthCode:             "AUTH456",
			ResponseCode:         "00",
			Message:              "Approved",
		}, nil)

		service := payment.NewService(dbPort, txRepo, mockGateway, mockLogger)

		// Execute first request
		ctx := context.Background()
		req := ports.ServiceAuthorizeRequest{
			MerchantID:     "MERCH-001",
			CustomerID:     "CUST-001",
			Amount:         decimal.NewFromFloat(50.00),
			Currency:       "USD",
			Token:          "test-token",
			IdempotencyKey: "idempotency-duplicate-test",
		}

		result1, err := service.Authorize(ctx, req)
		require.NoError(t, err)

		// Execute duplicate request with same idempotency key
		result2, err := service.Authorize(ctx, req)
		require.NoError(t, err)

		// Assert - should return same transaction
		assert.Equal(t, result1.TransactionID, result2.TransactionID)
		assert.Equal(t, 1, mockGateway.AuthorizeCalls) // Gateway called only once
	})

	t.Run("Sale_Success", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockGateway := mocks.NewMockCreditCardGateway()
		mockLogger := mocks.NewMockLogger()

		mockGateway.SetSaleResponse(&ports.PaymentResult{
			GatewayTransactionID: "gw-tx-003",
			Status:               models.StatusCaptured,
			AuthCode:             "SALE789",
			ResponseCode:         "00",
			Message:              "Approved",
		}, nil)

		service := payment.NewService(dbPort, txRepo, mockGateway, mockLogger)

		// Execute
		ctx := context.Background()
		req := ports.ServiceSaleRequest{
			MerchantID:     "MERCH-001",
			CustomerID:     "CUST-002",
			Amount:         decimal.NewFromFloat(75.50),
			Currency:       "USD",
			Token:          "test-sale-token",
			IdempotencyKey: "sale-idempotency-001",
		}

		result, err := service.Sale(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, models.StatusCaptured, result.Status)

		// Verify database
		txUUID, err := uuid.Parse(result.TransactionID)
		require.NoError(t, err)

		tx, err := txRepo.GetByID(ctx, pool, txUUID)
		require.NoError(t, err)
		assert.Equal(t, models.TypeSale, tx.Type)
		assert.Equal(t, models.StatusCaptured, tx.Status)
	})

	t.Run("Authorize_GatewayError", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockGateway := mocks.NewMockCreditCardGateway()
		mockLogger := mocks.NewMockLogger()

		// Configure gateway to return error
		mockGateway.SetAuthorizeResponse(nil, pkgerrors.NewPaymentError(
			"51",
			"Insufficient funds",
			pkgerrors.CategoryInsufficientFunds,
			true,
		))

		service := payment.NewService(dbPort, txRepo, mockGateway, mockLogger)

		// Execute
		ctx := context.Background()
		req := ports.ServiceAuthorizeRequest{
			MerchantID:     "MERCH-001",
			Amount:         decimal.NewFromFloat(100.00),
			Currency:       "USD",
			Token:          "test-token",
			IdempotencyKey: "error-test-001",
		}

		result, err := service.Authorize(ctx, req)

		// Assert
		require.Error(t, err)
		assert.Nil(t, result)

		var paymentErr *pkgerrors.PaymentError
		require.ErrorAs(t, err, &paymentErr)
		assert.Equal(t, "51", paymentErr.Code)
		assert.Equal(t, pkgerrors.CategoryInsufficientFunds, paymentErr.Category)
	})
}

func TestPaymentService_Integration_CaptureVoidRefund(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	pool := testdb.SetupTestDB(t)
	defer testdb.TeardownTestDB(t, pool)

	t.Run("Capture_Success", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockGateway := mocks.NewMockCreditCardGateway()
		mockLogger := mocks.NewMockLogger()

		// Create initial authorization
		mockGateway.SetAuthorizeResponse(&ports.PaymentResult{
			GatewayTransactionID: "gw-auth-001",
			Status:               models.StatusAuthorized,
			AuthCode:             "AUTH001",
			ResponseCode:         "00",
		}, nil)

		service := payment.NewService(dbPort, txRepo, mockGateway, mockLogger)
		ctx := context.Background()

		authReq := ports.ServiceAuthorizeRequest{
			MerchantID:     "MERCH-001",
			Amount:         decimal.NewFromFloat(150.00),
			Currency:       "USD",
			Token:          "test-token",
			IdempotencyKey: "auth-for-capture-001",
		}

		authResult, err := service.Authorize(ctx, authReq)
		require.NoError(t, err)

		// Configure capture response
		mockGateway.SetCaptureResponse(&ports.PaymentResult{
			GatewayTransactionID: "gw-capture-001",
			Status:               models.StatusCaptured,
			ResponseCode:         "00",
			Message:              "Captured",
		}, nil)

		// Execute capture
		amt := decimal.NewFromFloat(150.00)
		captureReq := ports.ServiceCaptureRequest{
			TransactionID: authResult.TransactionID,
			Amount:        &amt,
		}

		captureResult, err := service.Capture(ctx, captureReq)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, captureResult)
		assert.Equal(t, models.StatusCaptured, captureResult.Status)

		// Verify database - original transaction should be updated
		txUUID, err := uuid.Parse(authResult.TransactionID)
		require.NoError(t, err)

		originalTx, err := txRepo.GetByID(ctx, pool, txUUID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusCaptured, originalTx.Status)
	})

	t.Run("Void_Success", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockGateway := mocks.NewMockCreditCardGateway()
		mockLogger := mocks.NewMockLogger()

		// Create initial transaction
		mockGateway.SetAuthorizeResponse(&ports.PaymentResult{
			GatewayTransactionID: "gw-auth-002",
			Status:               models.StatusAuthorized,
			ResponseCode:         "00",
		}, nil)

		service := payment.NewService(dbPort, txRepo, mockGateway, mockLogger)
		ctx := context.Background()

		authResult, err := service.Authorize(ctx, ports.ServiceAuthorizeRequest{
			MerchantID:     "MERCH-001",
			Amount:         decimal.NewFromFloat(200.00),
			Currency:       "USD",
			Token:          "test-token",
			IdempotencyKey: "auth-for-void-001",
		})
		require.NoError(t, err)

		// Configure void response
		mockGateway.SetVoidResponse(&ports.PaymentResult{
			GatewayTransactionID: "gw-void-001",
			Status:               models.StatusVoided,
			ResponseCode:         "00",
			Message:              "Voided",
		}, nil)

		// Execute void
		voidReq := ports.ServiceVoidRequest{
			TransactionID: authResult.TransactionID,
		}

		voidResult, err := service.Void(ctx, voidReq)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, voidResult)
		assert.Equal(t, models.StatusVoided, voidResult.Status)

		// Verify original transaction status
		txUUID, err := uuid.Parse(authResult.TransactionID)
		require.NoError(t, err)

		originalTx, err := txRepo.GetByID(ctx, pool, txUUID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusVoided, originalTx.Status)
	})

	t.Run("Refund_Success", func(t *testing.T) {
		testdb.CleanDatabase(t, pool)

		// Setup
		dbPort := postgres.NewDBExecutor(pool)
		txRepo := postgres.NewTransactionRepository(dbPort)
		mockGateway := mocks.NewMockCreditCardGateway()
		mockLogger := mocks.NewMockLogger()

		// Create initial sale
		mockGateway.SetSaleResponse(&ports.PaymentResult{
			GatewayTransactionID: "gw-sale-refund",
			Status:               models.StatusCaptured,
			ResponseCode:         "00",
		}, nil)

		service := payment.NewService(dbPort, txRepo, mockGateway, mockLogger)
		ctx := context.Background()

		saleResult, err := service.Sale(ctx, ports.ServiceSaleRequest{
			MerchantID:     "MERCH-001",
			Amount:         decimal.NewFromFloat(99.99),
			Currency:       "USD",
			Token:          "test-token",
			IdempotencyKey: "sale-for-refund-001",
		})
		require.NoError(t, err)

		// Configure refund response
		mockGateway.SetRefundResponse(&ports.PaymentResult{
			GatewayTransactionID: "gw-refund-001",
			Status:               models.StatusRefunded,
			ResponseCode:         "00",
			Message:              "Refunded",
		}, nil)

		// Execute refund
		amt := decimal.NewFromFloat(99.99)
		refundReq := ports.ServiceRefundRequest{
			TransactionID: saleResult.TransactionID,
			Amount:        &amt,
		}

		refundResult, err := service.Refund(ctx, refundReq)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, refundResult)
		assert.Equal(t, models.StatusRefunded, refundResult.Status)

		// Verify original transaction status
		txUUID, err := uuid.Parse(saleResult.TransactionID)
		require.NoError(t, err)

		originalTx, err := txRepo.GetByID(ctx, pool, txUUID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusRefunded, originalTx.Status)
	})
}
