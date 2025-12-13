package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/internal/testutil/mocks"
)

// ============================================================================
// TransactionQueryService Tests
// ============================================================================

// TestListTransactions_Success tests successful transaction listing with all filters
func TestListTransactions_Success(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &transactionQueryService{
		queries: mockQuerier,
		logger:  logger,
	}

	merchantID := uuid.New()
	customerID := uuid.New()
	subscriptionID := uuid.New()

	merchantIDStr := merchantID.String()
	customerIDStr := customerID.String()
	subscriptionIDStr := subscriptionID.String()

	filters := &ports.ListTransactionsFilters{
		MerchantID:     &merchantIDStr,
		CustomerID:     &customerIDStr,
		SubscriptionID: &subscriptionIDStr,
		Limit:          10,
		Offset:         0,
	}

	// Mock transactions
	now := time.Now()
	dbTxs := []sqlc.Transaction{
		{
			ID:                uuid.New(),
			MerchantID:        merchantID,
			CustomerID:        pgtype.Text{String: customerIDStr, Valid: true},
			SubscriptionID:    pgtype.UUID{Bytes: subscriptionID, Valid: true},
			AmountCents:       10000,
			Currency:          "USD",
			Type:              "SALE",
			PaymentMethodType: "credit_card",
			Status:            pgtype.Text{String: "approved", Valid: true},
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}

	// Expect ListTransactions call
	mockQuerier.On("ListTransactions", context.Background(), mock.MatchedBy(func(params sqlc.ListTransactionsParams) bool {
		return params.MerchantID == merchantID &&
			params.CustomerID.Valid && params.CustomerID.String == customerIDStr &&
			params.SubscriptionID.Valid && params.SubscriptionID.Bytes == subscriptionID &&
			params.LimitVal == 10 &&
			params.OffsetVal == 0
	})).Return(dbTxs, nil)

	// Expect CountTransactions call
	mockQuerier.On("CountTransactions", context.Background(), mock.MatchedBy(func(params sqlc.CountTransactionsParams) bool {
		return params.MerchantID == merchantID &&
			params.CustomerID.Valid && params.CustomerID.String == customerIDStr &&
			params.SubscriptionID.Valid && params.SubscriptionID.Bytes == subscriptionID
	})).Return(int64(1), nil)

	// Execute
	result, count, err := service.ListTransactions(context.Background(), filters)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, count)
	assert.Equal(t, merchantID.String(), result[0].MerchantID)

	mockQuerier.AssertExpectations(t)
}

// TestListTransactions_WithSubscriptionIDOnly tests filtering by subscription_id
func TestListTransactions_WithSubscriptionIDOnly(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &transactionQueryService{
		queries: mockQuerier,
		logger:  logger,
	}

	merchantID := uuid.New()
	subscriptionID := uuid.New()

	merchantIDStr := merchantID.String()
	subscriptionIDStr := subscriptionID.String()

	filters := &ports.ListTransactionsFilters{
		MerchantID:     &merchantIDStr,
		SubscriptionID: &subscriptionIDStr, // Only subscription filter
		Limit:          50,
		Offset:         0,
	}

	// Mock transactions
	now := time.Now()
	dbTxs := []sqlc.Transaction{
		{
			ID:                uuid.New(),
			MerchantID:        merchantID,
			SubscriptionID:    pgtype.UUID{Bytes: subscriptionID, Valid: true},
			AmountCents:       5000,
			Currency:          "USD",
			Type:              "SALE",
			PaymentMethodType: "credit_card",
			Status:            pgtype.Text{String: "approved", Valid: true},
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                uuid.New(),
			MerchantID:        merchantID,
			SubscriptionID:    pgtype.UUID{Bytes: subscriptionID, Valid: true},
			AmountCents:       5000,
			Currency:          "USD",
			Type:              "SALE",
			PaymentMethodType: "credit_card",
			Status:            pgtype.Text{String: "approved", Valid: true},
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}

	// Expect ListTransactions call with subscription_id filter
	mockQuerier.On("ListTransactions", context.Background(), mock.MatchedBy(func(params sqlc.ListTransactionsParams) bool {
		return params.MerchantID == merchantID &&
			!params.CustomerID.Valid && // No customer filter
			params.SubscriptionID.Valid && params.SubscriptionID.Bytes == subscriptionID &&
			params.LimitVal == 50
	})).Return(dbTxs, nil)

	// Expect CountTransactions call
	mockQuerier.On("CountTransactions", context.Background(), mock.MatchedBy(func(params sqlc.CountTransactionsParams) bool {
		return params.MerchantID == merchantID &&
			params.SubscriptionID.Valid && params.SubscriptionID.Bytes == subscriptionID
	})).Return(int64(2), nil)

	// Execute
	result, count, err := service.ListTransactions(context.Background(), filters)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, count)
	// Verify all transactions belong to the subscription
	for _, tx := range result {
		assert.NotNil(t, tx.SubscriptionID)
		assert.Equal(t, subscriptionID.String(), *tx.SubscriptionID)
	}

	mockQuerier.AssertExpectations(t)
}

// TestListTransactions_MissingMerchantID tests that merchant_id is required
func TestListTransactions_MissingMerchantID(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &transactionQueryService{
		queries: mockQuerier,
		logger:  logger,
	}

	filters := &ports.ListTransactionsFilters{
		MerchantID: nil, // Missing required field
		Limit:      10,
		Offset:     0,
	}

	// Execute
	result, count, err := service.ListTransactions(context.Background(), filters)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "merchant_id is required")

	// No database calls should have been made
	mockQuerier.AssertNotCalled(t, "ListTransactions")
	mockQuerier.AssertNotCalled(t, "CountTransactions")
}

// TestListTransactions_InvalidMerchantIDFormat tests invalid merchant_id format
func TestListTransactions_InvalidMerchantIDFormat(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &transactionQueryService{
		queries: mockQuerier,
		logger:  logger,
	}

	invalidID := "not-a-uuid"
	filters := &ports.ListTransactionsFilters{
		MerchantID: &invalidID,
		Limit:      10,
		Offset:     0,
	}

	// Execute
	result, count, err := service.ListTransactions(context.Background(), filters)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, count)
	assert.True(t, errors.Is(err, domain.ErrValidationInvalidUUID), "expected ErrValidationInvalidUUID")

	// No database calls should have been made
	mockQuerier.AssertNotCalled(t, "ListTransactions")
	mockQuerier.AssertNotCalled(t, "CountTransactions")
}

// TestListTransactions_DefaultLimit tests that default limit is applied
func TestListTransactions_DefaultLimit(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &transactionQueryService{
		queries: mockQuerier,
		logger:  logger,
	}

	merchantID := uuid.New()
	merchantIDStr := merchantID.String()

	filters := &ports.ListTransactionsFilters{
		MerchantID: &merchantIDStr,
		Limit:      0, // Should default to 100
		Offset:     0,
	}

	// Expect ListTransactions call with default limit
	mockQuerier.On("ListTransactions", context.Background(), mock.MatchedBy(func(params sqlc.ListTransactionsParams) bool {
		return params.MerchantID == merchantID &&
			params.LimitVal == 100 // Default limit
	})).Return([]sqlc.Transaction{}, nil)

	mockQuerier.On("CountTransactions", context.Background(), mock.Anything).Return(int64(0), nil)

	// Execute
	result, count, err := service.ListTransactions(context.Background(), filters)

	// Assert
	require.NoError(t, err)
	assert.Len(t, result, 0)
	assert.Equal(t, 0, count)

	mockQuerier.AssertExpectations(t)
}

// TestGetTransaction_Success tests successful transaction retrieval
func TestGetTransaction_Success(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &transactionQueryService{
		queries: mockQuerier,
		logger:  logger,
	}

	txID := uuid.New()
	merchantID := uuid.New()
	now := time.Now()

	dbTx := sqlc.Transaction{
		ID:                txID,
		MerchantID:        merchantID,
		AmountCents:       10000,
		Currency:          "USD",
		Type:              "SALE",
		PaymentMethodType: "credit_card",
		Status:            pgtype.Text{String: "approved", Valid: true},
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	mockQuerier.On("GetTransactionByID", context.Background(), txID).Return(dbTx, nil)

	// Execute
	result, err := service.GetTransaction(context.Background(), txID.String())

	// Assert
	require.NoError(t, err)
	assert.Equal(t, txID.String(), result.ID)
	assert.Equal(t, merchantID.String(), result.MerchantID)

	mockQuerier.AssertExpectations(t)
}

// TestGetTransaction_NotFound tests transaction not found error
func TestGetTransaction_NotFound(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &transactionQueryService{
		queries: mockQuerier,
		logger:  logger,
	}

	txID := uuid.New()

	mockQuerier.On("GetTransactionByID", context.Background(), txID).Return(sqlc.Transaction{}, errors.New("no rows"))

	// Execute
	result, err := service.GetTransaction(context.Background(), txID.String())

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domain.ErrTxnNotFound))

	mockQuerier.AssertExpectations(t)
}

// TestGetTransaction_InvalidUUID tests invalid UUID format
func TestGetTransaction_InvalidUUID(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &transactionQueryService{
		queries: mockQuerier,
		logger:  logger,
	}

	// Execute
	result, err := service.GetTransaction(context.Background(), "not-a-uuid")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, domain.ErrValidationInvalidUUID))

	// No database calls should have been made
	mockQuerier.AssertNotCalled(t, "GetTransactionByID")
}
