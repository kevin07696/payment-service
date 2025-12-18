package payment

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/converters"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
)

// ============================================================================
// Mock Implementations
// ============================================================================

// MockServerPostAdapter mocks the EPX Server Post adapter
type MockServerPostAdapter struct {
	mock.Mock
}

func (m *MockServerPostAdapter) ProcessTransaction(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ServerPostResponse), args.Error(1)
}

func (m *MockServerPostAdapter) ProcessTransactionViaSocket(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ServerPostResponse), args.Error(1)
}

func (m *MockServerPostAdapter) ValidateToken(ctx context.Context, authGUID string) error {
	args := m.Called(ctx, authGUID)
	return args.Error(0)
}

// MockSecretManagerAdapter mocks the secret manager adapter
type MockSecretManagerAdapter struct {
	mock.Mock
}

func (m *MockSecretManagerAdapter) GetSecret(ctx context.Context, path string) (*ports.Secret, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.Secret), args.Error(1)
}

func (m *MockSecretManagerAdapter) GetSecretVersion(ctx context.Context, path string, version string) (*ports.Secret, error) {
	args := m.Called(ctx, path, version)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.Secret), args.Error(1)
}

func (m *MockSecretManagerAdapter) PutSecret(ctx context.Context, path string, value string, metadata map[string]string) (string, error) {
	args := m.Called(ctx, path, value, metadata)
	return args.String(0), args.Error(1)
}

func (m *MockSecretManagerAdapter) RotateSecret(ctx context.Context, path string, newValue string) (*ports.SecretRotationInfo, error) {
	args := m.Called(ctx, path, newValue)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.SecretRotationInfo), args.Error(1)
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

	domainTx := sqlcTransactionToDomain(sqlcTx)

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

// NOTE: ListTransactions tests have been moved to transaction_query_service_test.go
// because ListTransactions is now implemented by TransactionQueryService.

// ============================================================================
// Idempotency Key Validation Tests
// ============================================================================
//
// These tests verify that idempotency keys are required for all payment operations.
// Idempotency is critical for preventing duplicate transactions (double charges,
// double refunds, etc.). The idempotency key is used to generate:
// - EPX TRAN_NBR (deterministic)
// - Transaction ID (parsed from UUID)

// createMinimalPaymentService creates a service with minimal dependencies for validation tests.
// Only serverPost is needed since validation happens before any DB or external calls.
func createMinimalPaymentService() *paymentService {
	return &paymentService{
		serverPost: &MockServerPostAdapter{},
	}
}

// TestCapture_MissingIdempotencyKey_ReturnsValidationError tests that Capture requires idempotency key
func TestCapture_MissingIdempotencyKey_ReturnsValidationError(t *testing.T) {
	svc := createMinimalPaymentService()
	ctx := context.Background()

	req := &ports.CaptureRequest{
		TransactionID:  uuid.New().String(),
		IdempotencyKey: nil, // Missing idempotency key
	}

	_, err := svc.Capture(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationMissingField.Is(err), "expected ErrValidationMissingField, got: %v", err)

	// Verify error details
	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}

// TestCapture_InvalidIdempotencyKeyFormat_ReturnsValidationError tests that Capture validates UUID format
func TestCapture_InvalidIdempotencyKeyFormat_ReturnsValidationError(t *testing.T) {
	svc := createMinimalPaymentService()
	ctx := context.Background()

	invalidKey := "not-a-valid-uuid"
	req := &ports.CaptureRequest{
		TransactionID:  uuid.New().String(),
		IdempotencyKey: &invalidKey,
	}

	_, err := svc.Capture(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationInvalidUUID.Is(err), "expected ErrValidationInvalidUUID, got: %v", err)

	// Verify error details
	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}

// TestVoid_MissingIdempotencyKey_ReturnsValidationError tests that Void requires idempotency key
func TestVoid_MissingIdempotencyKey_ReturnsValidationError(t *testing.T) {
	svc := createMinimalPaymentService()
	ctx := context.Background()

	req := &ports.VoidRequest{
		ParentTransactionID: uuid.New().String(),
		IdempotencyKey:      nil, // Missing idempotency key
	}

	_, err := svc.Void(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationMissingField.Is(err), "expected ErrValidationMissingField, got: %v", err)

	// Verify error details
	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}

// TestVoid_InvalidIdempotencyKeyFormat_ReturnsValidationError tests that Void validates UUID format
func TestVoid_InvalidIdempotencyKeyFormat_ReturnsValidationError(t *testing.T) {
	svc := createMinimalPaymentService()
	ctx := context.Background()

	invalidKey := "not-a-valid-uuid"
	req := &ports.VoidRequest{
		ParentTransactionID: uuid.New().String(),
		IdempotencyKey:      &invalidKey,
	}

	_, err := svc.Void(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationInvalidUUID.Is(err), "expected ErrValidationInvalidUUID, got: %v", err)

	// Verify error details
	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}

// TestRefund_MissingIdempotencyKey_ReturnsValidationError tests that Refund requires idempotency key
func TestRefund_MissingIdempotencyKey_ReturnsValidationError(t *testing.T) {
	svc := createMinimalPaymentService()
	ctx := context.Background()

	req := &ports.RefundRequest{
		ParentTransactionID: uuid.New().String(),
		IdempotencyKey:      nil, // Missing idempotency key
	}

	_, err := svc.Refund(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationMissingField.Is(err), "expected ErrValidationMissingField, got: %v", err)

	// Verify error details
	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}

// TestRefund_InvalidIdempotencyKeyFormat_ReturnsValidationError tests that Refund validates UUID format
func TestRefund_InvalidIdempotencyKeyFormat_ReturnsValidationError(t *testing.T) {
	svc := createMinimalPaymentService()
	ctx := context.Background()

	invalidKey := "not-a-valid-uuid"
	req := &ports.RefundRequest{
		ParentTransactionID: uuid.New().String(),
		IdempotencyKey:      &invalidKey,
	}

	_, err := svc.Refund(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationInvalidUUID.Is(err), "expected ErrValidationInvalidUUID, got: %v", err)

	// Verify error details
	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}

// TestIdempotencyKeyValidation_TableDriven tests all operations with various invalid inputs
func TestIdempotencyKeyValidation_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		idempotencyKey *string
		expectedError  *domain.DomainError
		description    string
	}{
		{
			name:           "nil idempotency key",
			idempotencyKey: nil,
			expectedError:  domain.ErrValidationMissingField,
			description:    "Missing idempotency key should return ErrValidationMissingField",
		},
		{
			name:           "empty string idempotency key",
			idempotencyKey: strPtr(""),
			expectedError:  domain.ErrValidationInvalidUUID,
			description:    "Empty string is invalid UUID format",
		},
		{
			name:           "invalid UUID format - plain text",
			idempotencyKey: strPtr("not-a-uuid"),
			expectedError:  domain.ErrValidationInvalidUUID,
			description:    "Plain text is invalid UUID format",
		},
		{
			name:           "invalid UUID format - partial UUID",
			idempotencyKey: strPtr("550e8400-e29b-41d4"),
			expectedError:  domain.ErrValidationInvalidUUID,
			description:    "Partial UUID is invalid",
		},
		{
			name:           "invalid UUID format - extra characters",
			idempotencyKey: strPtr("550e8400-e29b-41d4-a716-446655440000-extra"),
			expectedError:  domain.ErrValidationInvalidUUID,
			description:    "UUID with extra characters is invalid",
		},
	}

	for _, tt := range tests {
		t.Run("Capture_"+tt.name, func(t *testing.T) {
			svc := createMinimalPaymentService()
			req := &ports.CaptureRequest{
				TransactionID:  uuid.New().String(),
				IdempotencyKey: tt.idempotencyKey,
			}

			_, err := svc.Capture(context.Background(), req)

			require.Error(t, err, tt.description)
			assert.True(t, tt.expectedError.Is(err), "expected %v, got: %v", tt.expectedError, err)
		})

		t.Run("Void_"+tt.name, func(t *testing.T) {
			svc := createMinimalPaymentService()
			req := &ports.VoidRequest{
				ParentTransactionID: uuid.New().String(),
				IdempotencyKey:      tt.idempotencyKey,
			}

			_, err := svc.Void(context.Background(), req)

			require.Error(t, err, tt.description)
			assert.True(t, tt.expectedError.Is(err), "expected %v, got: %v", tt.expectedError, err)
		})

		t.Run("Refund_"+tt.name, func(t *testing.T) {
			svc := createMinimalPaymentService()
			req := &ports.RefundRequest{
				ParentTransactionID: uuid.New().String(),
				IdempotencyKey:      tt.idempotencyKey,
			}

			_, err := svc.Refund(context.Background(), req)

			require.Error(t, err, tt.description)
			assert.True(t, tt.expectedError.Is(err), "expected %v, got: %v", tt.expectedError, err)
		})
	}
}

// strPtr is a helper to create string pointers for tests
func strPtr(s string) *string {
	return &s
}

// ============================================================================
// Mock for MerchantAuthorizationService
// ============================================================================

// MockMerchantAuthorizationService mocks the MerchantAuthorizationService port
type MockMerchantAuthorizationService struct {
	mock.Mock
}

func (m *MockMerchantAuthorizationService) ResolveMerchantID(ctx context.Context, requestedMerchantID string) (string, error) {
	args := m.Called(ctx, requestedMerchantID)
	return args.String(0), args.Error(1)
}

func (m *MockMerchantAuthorizationService) ValidateTransactionAccess(ctx context.Context, tx *domain.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

func (m *MockMerchantAuthorizationService) ValidateCustomerAccess(ctx context.Context, merchantID, customerID string) error {
	args := m.Called(ctx, merchantID, customerID)
	return args.Error(0)
}

func (m *MockMerchantAuthorizationService) ValidatePaymentMethodAccess(ctx context.Context, merchantID, paymentMethodID string) error {
	args := m.Called(ctx, merchantID, paymentMethodID)
	return args.Error(0)
}

// ============================================================================
// NOTE: resolvePaymentToken Validation
// ============================================================================
//
// The resolvePaymentToken function validates payment method status before use.
// This validation is implemented in domain.PaymentMethod.CanUseForAmount() and is
// already thoroughly tested in:
//   - internal/domain/payment_method_test.go
//
// Service-level integration tests for resolvePaymentToken should verify:
//   - Payment method not found → ErrPMNotFound
//   - Invalid payment method UUID → ErrValidationInvalidUUID
//   - Pending ACH → ErrPMInactive (not verified)
//   - Expired credit card → ErrPMInactive
//   - Revoked payment method → ErrPMInactive
//   - Active payment method → Success
//
// These are better suited for integration tests due to the 158-method Querier interface.
// See: tests/integration/payment/payment_method_validation_test.go

// ============================================================================
// Sale/Authorize Idempotency Validation Tests
// ============================================================================

// createPaymentServiceWithMerchantAuth creates a service with merchant auth mock for Sale/Authorize tests
func createPaymentServiceWithMerchantAuth() (*paymentService, *MockMerchantAuthorizationService) {
	mockMerchantAuth := new(MockMerchantAuthorizationService)
	svc := &paymentService{
		serverPost:          &MockServerPostAdapter{},
		merchantAuthService: mockMerchantAuth,
		logger:              zap.NewNop(), // No-op logger for tests
	}
	return svc, mockMerchantAuth
}

// TestSale_MissingIdempotencyKey_ReturnsValidationError tests that Sale requires idempotency key
func TestSale_MissingIdempotencyKey_ReturnsValidationError(t *testing.T) {
	svc, mockMerchantAuth := createPaymentServiceWithMerchantAuth()
	ctx := context.Background()

	// Setup: ResolveMerchantID succeeds
	merchantID := uuid.New().String()
	mockMerchantAuth.On("ResolveMerchantID", ctx, merchantID).Return(merchantID, nil)

	req := &ports.SaleRequest{
		MerchantID:     merchantID,
		AmountCents:    1000,
		IdempotencyKey: nil, // Missing idempotency key
	}

	_, err := svc.Sale(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationMissingField.Is(err), "expected ErrValidationMissingField, got: %v", err)

	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}

// TestSale_InvalidIdempotencyKeyFormat_ReturnsValidationError tests that Sale validates UUID format
func TestSale_InvalidIdempotencyKeyFormat_ReturnsValidationError(t *testing.T) {
	svc, mockMerchantAuth := createPaymentServiceWithMerchantAuth()
	ctx := context.Background()

	merchantID := uuid.New().String()
	mockMerchantAuth.On("ResolveMerchantID", ctx, merchantID).Return(merchantID, nil)

	invalidKey := "not-a-valid-uuid"
	req := &ports.SaleRequest{
		MerchantID:     merchantID,
		AmountCents:    1000,
		IdempotencyKey: &invalidKey,
	}

	_, err := svc.Sale(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationInvalidUUID.Is(err), "expected ErrValidationInvalidUUID, got: %v", err)

	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}

// TestAuthorize_MissingIdempotencyKey_ReturnsValidationError tests that Authorize requires idempotency key
func TestAuthorize_MissingIdempotencyKey_ReturnsValidationError(t *testing.T) {
	svc, mockMerchantAuth := createPaymentServiceWithMerchantAuth()
	ctx := context.Background()

	merchantID := uuid.New().String()
	mockMerchantAuth.On("ResolveMerchantID", ctx, merchantID).Return(merchantID, nil)

	req := &ports.AuthorizeRequest{
		MerchantID:     merchantID,
		AmountCents:    1000,
		IdempotencyKey: nil, // Missing idempotency key
	}

	_, err := svc.Authorize(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationMissingField.Is(err), "expected ErrValidationMissingField, got: %v", err)

	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}

// TestAuthorize_InvalidIdempotencyKeyFormat_ReturnsValidationError tests that Authorize validates UUID format
func TestAuthorize_InvalidIdempotencyKeyFormat_ReturnsValidationError(t *testing.T) {
	svc, mockMerchantAuth := createPaymentServiceWithMerchantAuth()
	ctx := context.Background()

	merchantID := uuid.New().String()
	mockMerchantAuth.On("ResolveMerchantID", ctx, merchantID).Return(merchantID, nil)

	invalidKey := "not-a-valid-uuid"
	req := &ports.AuthorizeRequest{
		MerchantID:     merchantID,
		AmountCents:    1000,
		IdempotencyKey: &invalidKey,
	}

	_, err := svc.Authorize(ctx, req)

	require.Error(t, err)
	assert.True(t, domain.ErrValidationInvalidUUID.Is(err), "expected ErrValidationInvalidUUID, got: %v", err)

	var domainErr *domain.DomainError
	if assert.ErrorAs(t, err, &domainErr) {
		assert.Equal(t, "idempotency_key", domainErr.Details["field"])
	}
}
