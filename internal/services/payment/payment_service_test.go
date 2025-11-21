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

	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/converters"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/kevin07696/payment-service/internal/testutil/mocks"
	"github.com/shopspring/decimal"
)

// ============================================================================
// Mock Implementations
// ============================================================================

// MockServerPostAdapter mocks the EPX Server Post adapter
type MockServerPostAdapter struct {
	mock.Mock
}

func (m *MockServerPostAdapter) ProcessTransaction(ctx context.Context, req *adapterports.ServerPostRequest) (*adapterports.ServerPostResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*adapterports.ServerPostResponse), args.Error(1)
}

func (m *MockServerPostAdapter) ProcessTransactionViaSocket(ctx context.Context, req *adapterports.ServerPostRequest) (*adapterports.ServerPostResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*adapterports.ServerPostResponse), args.Error(1)
}

func (m *MockServerPostAdapter) ValidateToken(ctx context.Context, authGUID string) error {
	args := m.Called(ctx, authGUID)
	return args.Error(0)
}

// MockSecretManagerAdapter mocks the secret manager adapter
type MockSecretManagerAdapter struct {
	mock.Mock
}

func (m *MockSecretManagerAdapter) GetSecret(ctx context.Context, path string) (*adapterports.Secret, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*adapterports.Secret), args.Error(1)
}

func (m *MockSecretManagerAdapter) GetSecretVersion(ctx context.Context, path string, version string) (*adapterports.Secret, error) {
	args := m.Called(ctx, path, version)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*adapterports.Secret), args.Error(1)
}

func (m *MockSecretManagerAdapter) PutSecret(ctx context.Context, path string, value string, metadata map[string]string) (string, error) {
	args := m.Called(ctx, path, value, metadata)
	return args.String(0), args.Error(1)
}

func (m *MockSecretManagerAdapter) RotateSecret(ctx context.Context, path string, newValue string) (*adapterports.SecretRotationInfo, error) {
	args := m.Called(ctx, path, newValue)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*adapterports.SecretRotationInfo), args.Error(1)
}

func (m *MockSecretManagerAdapter) DeleteSecret(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

// NOTE: Complete mock implementation of sqlc.Querier would require ~70 methods.
// For unit tests that require database operations, we should use integration tests
// with a real PostgreSQL database instead of complex mocks.
//
// The critical business logic (idempotency, amount validation, state transitions)
// is thoroughly tested in group_state_test.go and validation_test.go using pure
// functions that don't require database mocking.

// ============================================================================
// Test Helpers
// ============================================================================
//
// Critical Business Logic Tests - Require Integration Tests
// ============================================================================
//
// The following 5 critical business logic tests from Phase 1 of the risk-based
// testing strategy require integration tests with a real PostgreSQL database
// rather than unit tests with mocks. This is because:
//
// 1. Idempotency testing requires actual database constraints (PRIMARY KEY)
// 2. State validation requires WAL-based state computation across multiple rows
// 3. Concurrent request testing requires real database transaction locking
// 4. EPX decline handling requires both database writes and EPX adapter calls
//
// The pure business logic (state computation, validation rules) is already
// thoroughly tested in:
// - group_state_test.go: WAL-based state computation logic
// - validation_test.go: Table-driven validation rules
//
// These integration tests should be implemented in:
// tests/integration/payment/payment_service_integration_test.go
//
// Recommended tests:
// 1. TestSale_DuplicateIdempotencyKey_ReturnsSameTransaction (p99, catastrophic)
// 2. TestRefund_ExceedsOriginalAmount_ReturnsValidationError (p95, catastrophic)
// 3. TestCapture_NonAuthorizedTransaction_ReturnsValidationError (p95, high)
// 4. TestCaptureAndVoid_ConcurrentRequests_ExactlyOneSucceeds (p99.9, high)
// 5. TestSale_InsufficientFunds_ReturnsDeclinedStatus (p90, medium)

// ============================================================================
// Helper Function Tests
// ============================================================================

// TestSqlcToDomain_ValidTransaction tests conversion from sqlc to domain model
func TestSqlcToDomain_ValidTransaction(t *testing.T) {
	txID := uuid.New()
	parentTxID := uuid.New()
	merchantID := uuid.New()
	customerID := "cust_test_12345"
	authGUID := "bric-abc123"
	authResp := "00"
	authCode := "999999"

	sqlcTx := &sqlc.Transaction{
		ID:                  txID,
		ParentTransactionID: pgtype.UUID{Bytes: parentTxID, Valid: true},
		MerchantID:          merchantID,
		CustomerID:          pgtype.Text{String: customerID, Valid: true},
		AmountCents:         10050, // $100.50
		Currency:            "USD",
		Type:                "sale",
		PaymentMethodType:   "credit_card",
		AuthGuid:            pgtype.Text{String: authGUID, Valid: true},
		AuthResp:            pgtype.Text{String: authResp, Valid: true},
		AuthCode:            pgtype.Text{String: authCode, Valid: true},
		Status:              pgtype.Text{String: "approved", Valid: true},
	}

	domainTx := sqlcToDomain(sqlcTx)

	// Assert: Conversion is accurate
	assert.Equal(t, txID.String(), domainTx.ID)
	require.NotNil(t, domainTx.ParentTransactionID)
	assert.Equal(t, parentTxID.String(), *domainTx.ParentTransactionID)
	assert.Equal(t, merchantID.String(), domainTx.MerchantID)
	require.NotNil(t, domainTx.CustomerID)
	assert.Equal(t, customerID, *domainTx.CustomerID)
	assert.Equal(t, int64(10050), domainTx.AmountCents) // $100.50 = 10050 cents
	assert.Equal(t, "USD", domainTx.Currency)
	assert.Equal(t, domain.TransactionType("sale"), domainTx.Type)
	assert.Equal(t, authGUID, domainTx.AuthGUID)
	require.NotNil(t, domainTx.AuthResp)
	assert.Equal(t, authResp, *domainTx.AuthResp)
	require.NotNil(t, domainTx.AuthCode)
	assert.Equal(t, authCode, *domainTx.AuthCode)
	assert.Equal(t, domain.TransactionStatusApproved, domainTx.Status)
}

// TestToNullableText_NilValue tests nullable text conversion
func TestToNullableText_NilValue(t *testing.T) {
	result := converters.ToNullableText(nil)
	assert.False(t, result.Valid)
}

// TestToNullableText_ValidValue tests nullable text conversion with value
func TestToNullableText_ValidValue(t *testing.T) {
	str := "test-value"
	result := converters.ToNullableText(&str)
	assert.True(t, result.Valid)
	assert.Equal(t, "test-value", result.String)
}

// TestToNullableUUID_NilValue tests nullable UUID conversion
func TestToNullableUUID_NilValue(t *testing.T) {
	result := converters.ToNullableUUID(nil)
	assert.False(t, result.Valid)
}

// TestToNullableUUID_ValidValue tests nullable UUID conversion with value
func TestToNullableUUID_ValidValue(t *testing.T) {
	id := uuid.New()
	str := id.String()
	result := converters.ToNullableUUID(&str)
	assert.True(t, result.Valid)
	assert.Equal(t, id, uuid.UUID(result.Bytes))
}

// TestToNullableUUID_InvalidFormat tests nullable UUID conversion with invalid format
func TestToNullableUUID_InvalidFormat(t *testing.T) {
	str := "not-a-uuid"
	result := converters.ToNullableUUID(&str)
	assert.False(t, result.Valid)
}

// TestToNumeric_ValidDecimal tests decimal to numeric conversion
func TestToNumeric_ValidDecimal(t *testing.T) {
	d := decimal.RequireFromString("123.45")
	result := toNumeric(d)
	assert.True(t, result.Valid)
	assert.Equal(t, d.Coefficient(), result.Int)
	assert.Equal(t, d.Exponent(), result.Exp)
}

// TestStringOrEmpty_NilValue tests string or empty with nil
func TestStringOrEmpty_NilValue(t *testing.T) {
	result := stringOrEmpty(nil)
	assert.Equal(t, "", result)
}

// TestStringOrEmpty_ValidValue tests string or empty with value
func TestStringOrEmpty_ValidValue(t *testing.T) {
	str := "test-value"
	result := stringOrEmpty(&str)
	assert.Equal(t, "test-value", result)
}

// TestStringToUUIDPtr_NilValue tests UUID pointer conversion with nil
func TestStringToUUIDPtr_NilValue(t *testing.T) {
	result := stringToUUIDPtr(nil)
	assert.Nil(t, result)
}

// TestStringToUUIDPtr_EmptyValue tests UUID pointer conversion with empty string
func TestStringToUUIDPtr_EmptyValue(t *testing.T) {
	str := ""
	result := stringToUUIDPtr(&str)
	assert.Nil(t, result)
}

// TestStringToUUIDPtr_ValidValue tests UUID pointer conversion with valid UUID
func TestStringToUUIDPtr_ValidValue(t *testing.T) {
	id := uuid.New()
	str := id.String()
	result := stringToUUIDPtr(&str)
	require.NotNil(t, result)
	assert.Equal(t, id, *result)
}

// TestStringToUUIDPtr_InvalidFormat tests UUID pointer conversion with invalid format
func TestStringToUUIDPtr_InvalidFormat(t *testing.T) {
	str := "not-a-uuid"
	result := stringToUUIDPtr(&str)
	assert.Nil(t, result)
}

// TestIsUniqueViolation tests unique constraint error detection
func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"unique constraint error", errors.New("unique constraint violation"), true},
		{"duplicate key error", errors.New("duplicate key value"), true},
		{"postgres 23505 error", errors.New("ERROR: 23505"), true},
		{"other error", errors.New("something else"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUniqueViolation(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Service Method Tests (with mocking)
// ============================================================================

// TestListTransactions_Success tests successful transaction listing with all filters
func TestListTransactions_Success(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &paymentService{
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

	service := &paymentService{
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

	service := &paymentService{
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

	service := &paymentService{
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
	assert.Contains(t, err.Error(), "invalid merchant_id format")

	// No database calls should have been made
	mockQuerier.AssertNotCalled(t, "ListTransactions")
	mockQuerier.AssertNotCalled(t, "CountTransactions")
}

// TestListTransactions_DefaultLimit tests that default limit is applied
func TestListTransactions_DefaultLimit(t *testing.T) {
	// Setup
	mockQuerier := new(mocks.MockQuerier)
	logger := zap.NewNop()

	service := &paymentService{
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
