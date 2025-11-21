package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/adapters/database" // For database.TransactionManager type
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/kevin07696/payment-service/internal/testutil/fixtures"
	"github.com/kevin07696/payment-service/internal/testutil/mocks"
)

// NOTE: These tests currently require integration testing due to the WithTx callback pattern
// The callback receives *sqlc.Queries (concrete type), making pure unit testing impossible
// Future refactoring should move away from WithTx callbacks to enable full unit testing
// For now, tests that use transactions are marked with t.Skip() and documented

// MockServerPostAdapter mocks the server post adapter
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

// MockTransactionManager mocks the transaction manager
type MockTransactionManager struct {
	mock.Mock
	querier *mocks.MockQuerier
}

// Ensure MockTransactionManager implements database.TransactionManager
var _ database.TransactionManager = (*MockTransactionManager)(nil)

func (m *MockTransactionManager) WithTx(ctx context.Context, fn func(sqlc.Querier) error) error {
	args := m.Called(ctx, fn)

	// Execute the transaction function with the mock querier
	// This works now because the callback accepts sqlc.Querier interface, not concrete type
	if err := fn(m.querier); err != nil {
		return err
	}

	return args.Error(0)
}

// setupSubscriptionService creates a service with mocked dependencies
func setupSubscriptionService(t *testing.T) (*subscriptionService, *mocks.MockQuerier, *MockTransactionManager, *MockServerPostAdapter, *MockSecretManagerAdapter) {
	mockQuerier := new(mocks.MockQuerier)
	mockTxManager := new(MockTransactionManager)
	mockTxManager.querier = mockQuerier // Set the querier so WithTx can use it
	mockServerPost := new(MockServerPostAdapter)
	mockSecretManager := new(MockSecretManagerAdapter)
	logger := zap.NewNop()

	service := &subscriptionService{
		queries:       mockQuerier,
		txManager:     mockTxManager,
		serverPost:    mockServerPost,
		secretManager: mockSecretManager,
		logger:        logger,
	}

	return service, mockQuerier, mockTxManager, mockServerPost, mockSecretManager
}

// TestCreateSubscription_Success tests successful subscription creation
func TestCreateSubscription_Success(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	merchantID := uuid.New()
	customerID := uuid.New()
	paymentMethodID := uuid.New()

	req := &ports.CreateSubscriptionRequest{
		MerchantID:      merchantID.String(),
		CustomerID:      customerID.String(),
		PaymentMethodID: paymentMethodID.String(),
		AmountCents:     9999, // $99.99
		Currency:        "USD",
		IntervalValue:   1,
		IntervalUnit:    domain.IntervalUnitMonth,
		StartDate:       time.Now(),
		MaxRetries:      3,
		Metadata:        map[string]interface{}{"plan": "premium"},
	}

	// Mock payment method lookup
	dbPaymentMethod := fixtures.NewPaymentMethod().
		WithID(paymentMethodID).
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		Active().
		Build()

	mockQuerier.On("GetPaymentMethodByID", ctx, paymentMethodID).
		Return(dbPaymentMethod, nil)

	// Mock subscription creation inside transaction
	expectedSub := fixtures.NewSubscription().
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		WithPaymentMethodID(paymentMethodID).
		WithAmountCents(9999).
		WithCurrency("USD").
		WithInterval(1, string(domain.IntervalUnitMonth)).
		Active().
		Build()

	mockQuerier.On("CreateSubscription", ctx, mock.AnythingOfType("sqlc.CreateSubscriptionParams")).
		Return(expectedSub, nil)

	// Mock transaction execution - returns nil to simulate successful transaction
	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	// Execute
	result, err := service.CreateSubscription(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, merchantID.String(), result.MerchantID)
	assert.Equal(t, customerID.String(), result.CustomerID)
	assert.Equal(t, int64(9999), result.AmountCents)
	assert.Equal(t, "USD", result.Currency)
	assert.Equal(t, 1, result.IntervalValue)
	assert.Equal(t, domain.IntervalUnitMonth, result.IntervalUnit)
	assert.Equal(t, domain.SubscriptionStatusActive, result.Status)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestCreateSubscription_InvalidPaymentMethodID tests validation
func TestCreateSubscription_InvalidPaymentMethodID(t *testing.T) {
	service, _, _, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	req := &ports.CreateSubscriptionRequest{
		MerchantID:      uuid.New().String(),
		CustomerID:      uuid.New().String(),
		PaymentMethodID: "invalid-uuid",
		AmountCents:     9999, // $99.99
		Currency:        "USD",
		IntervalValue:   1,
		IntervalUnit:    domain.IntervalUnitMonth,
	}

	result, err := service.CreateSubscription(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid payment_method_id format")
}

// TestCreateSubscription_PaymentMethodNotFound tests not found error
func TestCreateSubscription_PaymentMethodNotFound(t *testing.T) {
	service, mockQuerier, _, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	paymentMethodID := uuid.New()
	req := &ports.CreateSubscriptionRequest{
		MerchantID:      uuid.New().String(),
		CustomerID:      uuid.New().String(),
		PaymentMethodID: paymentMethodID.String(),
		AmountCents:     9999, // $99.99
		Currency:        "USD",
		IntervalValue:   1,
		IntervalUnit:    domain.IntervalUnitMonth,
	}

	mockQuerier.On("GetPaymentMethodByID", ctx, paymentMethodID).
		Return(sqlc.CustomerPaymentMethod{}, fmt.Errorf("not found"))

	result, err := service.CreateSubscription(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "payment method not found")

	mockQuerier.AssertExpectations(t)
}

// TestCreateSubscription_PaymentMethodBelongsToWrongCustomer tests ownership validation
func TestCreateSubscription_PaymentMethodBelongsToWrongCustomer(t *testing.T) {
	service, mockQuerier, _, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	merchantID := uuid.New()
	customerID := uuid.New()
	wrongCustomerID := uuid.New()
	paymentMethodID := uuid.New()

	req := &ports.CreateSubscriptionRequest{
		MerchantID:      merchantID.String(),
		CustomerID:      customerID.String(),
		PaymentMethodID: paymentMethodID.String(),
		AmountCents:     9999, // $99.99
		Currency:        "USD",
		IntervalValue:   1,
		IntervalUnit:    domain.IntervalUnitMonth,
	}

	// Payment method belongs to different customer
	dbPaymentMethod := fixtures.NewPaymentMethod().
		WithID(paymentMethodID).
		WithMerchantID(merchantID).
		WithCustomerID(wrongCustomerID).
		Active().
		Build()

	mockQuerier.On("GetPaymentMethodByID", ctx, paymentMethodID).
		Return(dbPaymentMethod, nil)

	result, err := service.CreateSubscription(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not belong to customer")

	mockQuerier.AssertExpectations(t)
}

// TestCreateSubscription_InvalidAmount tests amount validation
func TestCreateSubscription_InvalidAmount(t *testing.T) {
	service, mockQuerier, _, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	merchantID := uuid.New()
	customerID := uuid.New()
	paymentMethodID := uuid.New()

	dbPaymentMethod := fixtures.NewPaymentMethod().
		WithID(paymentMethodID).
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		Active().
		Build()

	mockQuerier.On("GetPaymentMethodByID", ctx, paymentMethodID).
		Return(dbPaymentMethod, nil)

	testCases := []struct {
		name        string
		amountCents int64
		errMsg      string
	}{
		{"Zero amount", 0, "amount must be greater than zero"},
		{"Negative amount", -1000, "amount must be greater than zero"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &ports.CreateSubscriptionRequest{
				MerchantID:      merchantID.String(),
				CustomerID:      customerID.String(),
				PaymentMethodID: paymentMethodID.String(),
				AmountCents:     tc.amountCents,
				Currency:        "USD",
				IntervalValue:   1,
				IntervalUnit:    domain.IntervalUnitMonth,
			}

			result, err := service.CreateSubscription(ctx, req)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

// TestUpdateSubscription_Success tests successful update
func TestUpdateSubscription_Success(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()
	merchantID := uuid.New()
	customerID := uuid.New()

	newAmount := int64(14999) // $149.99
	req := &ports.UpdateSubscriptionRequest{
		SubscriptionID: subscriptionID.String(),
		AmountCents:    &newAmount,
	}

	// Mock existing subscription lookup
	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		WithAmountCents(9999).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	// Mock update
	updatedSub := existingSub
	updatedSub.AmountCents = 14999

	mockQuerier.On("UpdateSubscription", ctx, mock.AnythingOfType("sqlc.UpdateSubscriptionParams")).
		Return(updatedSub, nil)

	// Mock transaction
	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.UpdateSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(14999), result.AmountCents)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestUpdateSubscription_ChangePaymentMethod tests payment method update
func TestUpdateSubscription_ChangePaymentMethod(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()
	merchantID := uuid.New()
	customerID := uuid.New()
	oldPaymentMethodID := uuid.New()
	newPaymentMethodID := uuid.New()

	// Existing subscription with old payment method
	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		WithPaymentMethodID(oldPaymentMethodID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	// New payment method belongs to same customer
	newPM := fixtures.NewPaymentMethod().
		WithID(newPaymentMethodID).
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		Active().
		Build()

	mockQuerier.On("GetPaymentMethodByID", ctx, newPaymentMethodID).
		Return(newPM, nil)

	// Updated subscription with new payment method
	updatedSub := existingSub
	updatedSub.PaymentMethodID = newPaymentMethodID

	mockQuerier.On("UpdateSubscription", ctx, mock.MatchedBy(func(params sqlc.UpdateSubscriptionParams) bool {
		return params.ID == subscriptionID &&
			params.PaymentMethodID == newPaymentMethodID
	})).Return(updatedSub, nil)

	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	// Execute
	newPMStr := newPaymentMethodID.String()
	req := &ports.UpdateSubscriptionRequest{
		SubscriptionID:  subscriptionID.String(),
		PaymentMethodID: &newPMStr,
	}

	result, err := service.UpdateSubscription(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, newPaymentMethodID.String(), result.PaymentMethodID)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestUpdateSubscription_PaymentMethodBelongsToWrongCustomer tests ownership validation
func TestUpdateSubscription_PaymentMethodBelongsToWrongCustomer(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()
	merchantID := uuid.New()
	customerID := uuid.New()
	wrongCustomerID := uuid.New()
	newPaymentMethodID := uuid.New()

	// Existing subscription
	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	// Payment method belongs to different customer
	wrongPM := fixtures.NewPaymentMethod().
		WithID(newPaymentMethodID).
		WithMerchantID(merchantID).
		WithCustomerID(wrongCustomerID). // Different customer!
		Active().
		Build()

	mockQuerier.On("GetPaymentMethodByID", ctx, newPaymentMethodID).
		Return(wrongPM, nil)

	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	newPMStr := newPaymentMethodID.String()
	req := &ports.UpdateSubscriptionRequest{
		SubscriptionID:  subscriptionID.String(),
		PaymentMethodID: &newPMStr,
	}

	result, err := service.UpdateSubscription(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not belong to customer")

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestUpdateSubscription_PaymentMethodNotActive tests active status validation
func TestUpdateSubscription_PaymentMethodNotActive(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()
	merchantID := uuid.New()
	customerID := uuid.New()
	newPaymentMethodID := uuid.New()

	// Existing subscription
	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	// Payment method is inactive
	inactivePM := fixtures.NewPaymentMethod().
		WithID(newPaymentMethodID).
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		Inactive(). // Not active!
		Build()

	mockQuerier.On("GetPaymentMethodByID", ctx, newPaymentMethodID).
		Return(inactivePM, nil)

	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	newPMStr := newPaymentMethodID.String()
	req := &ports.UpdateSubscriptionRequest{
		SubscriptionID:  subscriptionID.String(),
		PaymentMethodID: &newPMStr,
	}

	result, err := service.UpdateSubscription(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not active")

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestUpdateSubscription_PaymentMethodNotFound tests not found error
func TestUpdateSubscription_PaymentMethodNotFound(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()
	newPaymentMethodID := uuid.New()

	// Existing subscription
	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	// Payment method not found
	mockQuerier.On("GetPaymentMethodByID", ctx, newPaymentMethodID).
		Return(sqlc.CustomerPaymentMethod{}, fmt.Errorf("not found"))

	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	newPMStr := newPaymentMethodID.String()
	req := &ports.UpdateSubscriptionRequest{
		SubscriptionID:  subscriptionID.String(),
		PaymentMethodID: &newPMStr,
	}

	result, err := service.UpdateSubscription(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "payment method not found")

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestUpdateSubscription_InvalidPaymentMethodIDFormat tests format validation
func TestUpdateSubscription_InvalidPaymentMethodIDFormat(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	invalidPMID := "not-a-valid-uuid"
	req := &ports.UpdateSubscriptionRequest{
		SubscriptionID:  subscriptionID.String(),
		PaymentMethodID: &invalidPMID,
	}

	result, err := service.UpdateSubscription(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid payment_method_id format")

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestUpdateSubscription_CannotUpdateCancelled tests status validation
func TestUpdateSubscription_CannotUpdateCancelled(t *testing.T) {
	service, mockQuerier, _, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	newAmount := int64(14999) // $149.99
	req := &ports.UpdateSubscriptionRequest{
		SubscriptionID: subscriptionID.String(),
		AmountCents:    &newAmount,
	}

	// Subscription is cancelled
	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Cancelled().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	result, err := service.UpdateSubscription(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot update subscription in cancelled status")

	mockQuerier.AssertExpectations(t)
}

// TestCancelSubscription_ImmediateCancel tests immediate cancellation
func TestCancelSubscription_ImmediateCancel(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	req := &ports.CancelSubscriptionRequest{
		SubscriptionID:    subscriptionID.String(),
		CancelAtPeriodEnd: false,
	}

	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	cancelledSub := existingSub
	cancelledSub.Status = string(domain.SubscriptionStatusCancelled)
	now := time.Now()
	cancelledSub.CancelledAt = pgtype.Timestamptz{Time: now, Valid: true}

	mockQuerier.On("CancelSubscription", ctx, mock.MatchedBy(func(params sqlc.CancelSubscriptionParams) bool {
		return params.ID == subscriptionID &&
			params.Status == string(domain.SubscriptionStatusCancelled) &&
			params.CanceledAt.Valid
	})).Return(cancelledSub, nil)

	// Mock transaction
	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.CancelSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.SubscriptionStatusCancelled, result.Status)
	assert.NotNil(t, result.CancelledAt)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestCancelSubscription_CancelAtPeriodEnd tests deferred cancellation
func TestCancelSubscription_CancelAtPeriodEnd(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	req := &ports.CancelSubscriptionRequest{
		SubscriptionID:    subscriptionID.String(),
		CancelAtPeriodEnd: true,
	}

	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	// Should remain active when cancel_at_period_end is true
	mockQuerier.On("CancelSubscription", ctx, mock.MatchedBy(func(params sqlc.CancelSubscriptionParams) bool {
		return params.ID == subscriptionID &&
			params.Status == string(domain.SubscriptionStatusActive) &&
			!params.CanceledAt.Valid
	})).Return(existingSub, nil)

	// Mock transaction
	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.CancelSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.SubscriptionStatusActive, result.Status)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestPauseSubscription_Success tests pausing active subscription
func TestPauseSubscription_Success(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	existingSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(existingSub, nil)

	pausedSub := existingSub
	pausedSub.Status = string(domain.SubscriptionStatusPaused)

	mockQuerier.On("UpdateSubscriptionStatus", ctx, mock.MatchedBy(func(params sqlc.UpdateSubscriptionStatusParams) bool {
		return params.ID == subscriptionID &&
			params.Status == string(domain.SubscriptionStatusPaused)
	})).Return(pausedSub, nil)

	// Mock transaction
	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.PauseSubscription(ctx, subscriptionID.String())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.SubscriptionStatusPaused, result.Status)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestPauseSubscription_CannotPauseCancelled tests status validation
func TestPauseSubscription_CannotPauseCancelled(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	cancelledSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Cancelled().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(cancelledSub, nil)

	// Mock transaction - the callback will return validation error
	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.PauseSubscription(ctx, subscriptionID.String())

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot pause subscription in cancelled status")

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestResumeSubscription_Success tests resuming paused subscription
func TestResumeSubscription_Success(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	pausedSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Paused().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(pausedSub, nil)

	activeSub := pausedSub
	activeSub.Status = string(domain.SubscriptionStatusActive)

	mockQuerier.On("UpdateSubscriptionStatus", ctx, mock.MatchedBy(func(params sqlc.UpdateSubscriptionStatusParams) bool {
		return params.ID == subscriptionID &&
			params.Status == string(domain.SubscriptionStatusActive)
	})).Return(activeSub, nil)

	// Mock transaction
	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.ResumeSubscription(ctx, subscriptionID.String())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.SubscriptionStatusActive, result.Status)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestResumeSubscription_CannotResumeActive tests status validation
func TestResumeSubscription_CannotResumeActive(t *testing.T) {
	service, mockQuerier, mockTxManager, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	activeSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(activeSub, nil)

	// Mock transaction - the callback will return validation error
	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.ResumeSubscription(ctx, subscriptionID.String())

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot resume subscription in active status")

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestGetSubscription_Success tests successful retrieval
func TestGetSubscription_Success(t *testing.T) {
	service, mockQuerier, _, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	dbSub := fixtures.NewSubscription().
		WithID(subscriptionID).
		Active().
		Build()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(dbSub, nil)

	result, err := service.GetSubscription(ctx, subscriptionID.String())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, subscriptionID.String(), result.ID)

	mockQuerier.AssertExpectations(t)
}

// TestGetSubscription_NotFound tests not found error
func TestGetSubscription_NotFound(t *testing.T) {
	service, mockQuerier, _, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	subscriptionID := uuid.New()

	mockQuerier.On("GetSubscriptionByID", ctx, subscriptionID).
		Return(sqlc.Subscription{}, fmt.Errorf("not found"))

	result, err := service.GetSubscription(ctx, subscriptionID.String())

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "subscription not found")

	mockQuerier.AssertExpectations(t)
}

// TestListCustomerSubscriptions_Success tests listing subscriptions
func TestListCustomerSubscriptions_Success(t *testing.T) {
	service, mockQuerier, _, _, _ := setupSubscriptionService(t)
	ctx := context.Background()

	merchantID := uuid.New()
	customerID := uuid.New()

	dbSubs := []sqlc.Subscription{
		fixtures.NewSubscription().WithMerchantID(merchantID).WithCustomerID(customerID).Active().Build(),
		fixtures.NewSubscription().WithMerchantID(merchantID).WithCustomerID(customerID).Paused().Build(),
	}

	mockQuerier.On("ListSubscriptionsByCustomer", ctx, mock.MatchedBy(func(params sqlc.ListSubscriptionsByCustomerParams) bool {
		return params.MerchantID == merchantID && params.CustomerID == customerID
	})).Return(dbSubs, nil)

	result, err := service.ListCustomerSubscriptions(ctx, merchantID.String(), customerID.String())

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, domain.SubscriptionStatusActive, result[0].Status)
	assert.Equal(t, domain.SubscriptionStatusPaused, result[1].Status)

	mockQuerier.AssertExpectations(t)
}

// TestCalculateNextBillingDate tests billing date calculation
func TestCalculateNextBillingDate(t *testing.T) {
	baseDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		name          string
		intervalValue int
		intervalUnit  domain.IntervalUnit
		expected      time.Time
	}{
		{
			name:          "Daily",
			intervalValue: 7,
			intervalUnit:  domain.IntervalUnitDay,
			expected:      time.Date(2025, 1, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "Weekly",
			intervalValue: 2,
			intervalUnit:  domain.IntervalUnitWeek,
			expected:      time.Date(2025, 1, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "Monthly",
			intervalValue: 1,
			intervalUnit:  domain.IntervalUnitMonth,
			expected:      time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "Yearly",
			intervalValue: 1,
			intervalUnit:  domain.IntervalUnitYear,
			expected:      time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateNextBillingDate(baseDate, tc.intervalValue, tc.intervalUnit)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestSqlcSubscriptionToDomain tests domain conversion
func TestSqlcSubscriptionToDomain(t *testing.T) {
	subscriptionID := uuid.New()
	merchantID := uuid.New()
	customerID := uuid.New()
	paymentMethodID := uuid.New()

	metadata := map[string]interface{}{
		"plan":     "premium",
		"discount": "10percent",
	}
	metadataJSON, _ := json.Marshal(metadata)

	dbSub := &sqlc.Subscription{
		ID:                    subscriptionID,
		MerchantID:            merchantID,
		CustomerID:            customerID,
		AmountCents:           9999,
		Currency:              "USD",
		IntervalValue:         1,
		IntervalUnit:          string(domain.IntervalUnitMonth),
		Status:                string(domain.SubscriptionStatusActive),
		PaymentMethodID:       paymentMethodID,
		NextBillingDate:       pgtype.Date{Time: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		FailureRetryCount:     0,
		MaxRetries:            3,
		Metadata:              metadataJSON,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
		CancelledAt:           pgtype.Timestamptz{Valid: false},
		GatewaySubscriptionID: pgtype.Text{Valid: false},
	}

	result := sqlcSubscriptionToDomain(dbSub)

	assert.Equal(t, subscriptionID.String(), result.ID)
	assert.Equal(t, merchantID.String(), result.MerchantID)
	assert.Equal(t, customerID.String(), result.CustomerID)
	assert.Equal(t, int64(9999), result.AmountCents)
	assert.Equal(t, "USD", result.Currency)
	assert.Equal(t, 1, result.IntervalValue)
	assert.Equal(t, domain.IntervalUnitMonth, result.IntervalUnit)
	assert.Equal(t, domain.SubscriptionStatusActive, result.Status)
	assert.Equal(t, paymentMethodID.String(), result.PaymentMethodID)
	assert.Equal(t, 0, result.FailureRetryCount)
	assert.Equal(t, 3, result.MaxRetries)
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, "premium", result.Metadata["plan"])
}
