package subscription

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDBPort mocks the database port
type MockDBPort struct {
	mock.Mock
}

func (m *MockDBPort) GetDB() *pgxpool.Pool {
	return nil
}

func (m *MockDBPort) WithTransaction(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	args := m.Called(ctx, fn)
	if args.Error(0) != nil {
		return args.Error(0)
	}
	// Execute the function with nil transaction for testing
	return fn(ctx, nil)
}

func (m *MockDBPort) WithReadOnlyTransaction(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return fn(ctx, nil)
}

// MockSubscriptionRepository mocks the subscription repository
type MockSubscriptionRepository struct {
	mock.Mock
}

func (m *MockSubscriptionRepository) Create(ctx context.Context, tx ports.DBTX, sub *models.Subscription) error {
	args := m.Called(ctx, tx, sub)
	return args.Error(0)
}

func (m *MockSubscriptionRepository) GetByID(ctx context.Context, tx ports.DBTX, id uuid.UUID) (*models.Subscription, error) {
	args := m.Called(ctx, tx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) Update(ctx context.Context, tx ports.DBTX, sub *models.Subscription) error {
	args := m.Called(ctx, tx, sub)
	return args.Error(0)
}

func (m *MockSubscriptionRepository) ListByCustomer(ctx context.Context, tx ports.DBTX, merchantID, customerID string) ([]*models.Subscription, error) {
	args := m.Called(ctx, tx, merchantID, customerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepository) ListActiveSubscriptionsDueForBilling(ctx context.Context, tx ports.DBTX, asOfDate time.Time, limit int32) ([]*models.Subscription, error) {
	args := m.Called(ctx, tx, asOfDate, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Subscription), args.Error(1)
}

// MockPaymentService mocks the payment service
type MockPaymentService struct {
	mock.Mock
}

func (m *MockPaymentService) Authorize(ctx context.Context, req ports.ServiceAuthorizeRequest) (*ports.PaymentResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResponse), args.Error(1)
}

func (m *MockPaymentService) Capture(ctx context.Context, req ports.ServiceCaptureRequest) (*ports.PaymentResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResponse), args.Error(1)
}

func (m *MockPaymentService) Sale(ctx context.Context, req ports.ServiceSaleRequest) (*ports.PaymentResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResponse), args.Error(1)
}

func (m *MockPaymentService) Void(ctx context.Context, req ports.ServiceVoidRequest) (*ports.PaymentResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResponse), args.Error(1)
}

func (m *MockPaymentService) Refund(ctx context.Context, req ports.ServiceRefundRequest) (*ports.PaymentResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResponse), args.Error(1)
}

func (m *MockPaymentService) GetTransaction(ctx context.Context, transactionID string) (*models.Transaction, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Transaction), args.Error(1)
}

func (m *MockPaymentService) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*models.Transaction, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Transaction), args.Error(1)
}

// MockRecurringBillingGateway mocks the recurring billing gateway
type MockRecurringBillingGateway struct {
	mock.Mock
}

func (m *MockRecurringBillingGateway) CreateSubscription(ctx context.Context, req *ports.SubscriptionRequest) (*ports.SubscriptionResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.SubscriptionResult), args.Error(1)
}

func (m *MockRecurringBillingGateway) UpdateSubscription(ctx context.Context, subscriptionID string, req *ports.UpdateSubscriptionRequest) (*ports.SubscriptionResult, error) {
	args := m.Called(ctx, subscriptionID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.SubscriptionResult), args.Error(1)
}

func (m *MockRecurringBillingGateway) CancelSubscription(ctx context.Context, subscriptionID string, immediate bool) (*ports.SubscriptionResult, error) {
	args := m.Called(ctx, subscriptionID, immediate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.SubscriptionResult), args.Error(1)
}

func (m *MockRecurringBillingGateway) PauseSubscription(ctx context.Context, subscriptionID string) (*ports.SubscriptionResult, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.SubscriptionResult), args.Error(1)
}

func (m *MockRecurringBillingGateway) ResumeSubscription(ctx context.Context, subscriptionID string) (*ports.SubscriptionResult, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.SubscriptionResult), args.Error(1)
}

func (m *MockRecurringBillingGateway) GetSubscription(ctx context.Context, subscriptionID string) (*ports.SubscriptionResult, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.SubscriptionResult), args.Error(1)
}

func (m *MockRecurringBillingGateway) ListSubscriptions(ctx context.Context, customerID string) ([]*ports.SubscriptionResult, error) {
	args := m.Called(ctx, customerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ports.SubscriptionResult), args.Error(1)
}

func (m *MockRecurringBillingGateway) ChargePaymentMethod(ctx context.Context, paymentMethodID string, amount decimal.Decimal) (*ports.PaymentResult, error) {
	args := m.Called(ctx, paymentMethodID, amount)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResult), args.Error(1)
}

// MockLogger mocks the logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Debug(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
}

func TestService_CreateSubscription_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockGateway := new(MockRecurringBillingGateway)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, mockGateway, mockLogger)

	ctx := context.Background()
	startDate := time.Now()
	req := ports.ServiceCreateSubscriptionRequest{
		MerchantID:         "MERCH123",
		CustomerID:         "CUST456",
		Amount:             decimal.NewFromFloat(29.99),
		Currency:           "USD",
		Frequency:          models.FrequencyMonthly,
		PaymentMethodToken: "tok_test123",
		StartDate:          startDate,
		MaxRetries:         3,
		FailureOption:      models.FailureForward,
		Metadata:           map[string]string{"plan": "premium"},
		IdempotencyKey:     "idem-create-123",
	}

	// Mock expectations
	mockDB.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context, pgx.Tx) error")).
		Return(nil)
	mockSubRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Subscription")).
		Return(nil)
	mockGateway.On("CreateSubscription", ctx, mock.AnythingOfType("*ports.SubscriptionRequest")).
		Return(&ports.SubscriptionResult{
			GatewaySubscriptionID: "gw_sub_123",
			Status:                models.SubStatusActive,
		}, nil)
	// Expect Update call to persist gateway subscription ID
	mockSubRepo.On("Update", ctx, mock.Anything, mock.AnythingOfType("*models.Subscription")).
		Return(nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.CreateSubscription(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, req.MerchantID, resp.MerchantID)
	assert.Equal(t, req.CustomerID, resp.CustomerID)
	assert.Equal(t, req.Amount.String(), resp.Amount.String())
	assert.Equal(t, models.SubStatusActive, resp.Status)
	assert.True(t, resp.NextBillingDate.After(startDate))
	mockDB.AssertExpectations(t)
	mockSubRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

func TestService_CreateSubscription_WithoutGateway(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockLogger := new(MockLogger)

	// Create service without gateway
	service := NewService(mockDB, mockSubRepo, mockPaymentService, nil, mockLogger)

	ctx := context.Background()
	req := ports.ServiceCreateSubscriptionRequest{
		MerchantID:         "MERCH123",
		CustomerID:         "CUST456",
		Amount:             decimal.NewFromFloat(29.99),
		Currency:           "USD",
		Frequency:          models.FrequencyMonthly,
		PaymentMethodToken: "tok_test123",
		StartDate:          time.Now(),
		MaxRetries:         3,
		FailureOption:      models.FailureForward,
	}

	mockDB.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context, pgx.Tx) error")).
		Return(nil)
	mockSubRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Subscription")).
		Return(nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	resp, err := service.CreateSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.SubStatusActive, resp.Status)
	mockDB.AssertExpectations(t)
	mockSubRepo.AssertExpectations(t)
}

func TestService_UpdateSubscription_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockGateway := new(MockRecurringBillingGateway)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, mockGateway, mockLogger)

	ctx := context.Background()
	subID := uuid.New()
	existingSub := &models.Subscription{
		ID:                    subID.String(),
		MerchantID:            "MERCH123",
		CustomerID:            "CUST456",
		Amount:                decimal.NewFromFloat(29.99),
		Frequency:             models.FrequencyMonthly,
		Status:                models.SubStatusActive,
		GatewaySubscriptionID: "gw_sub_123",
	}

	newAmount := decimal.NewFromFloat(39.99)
	req := ports.ServiceUpdateSubscriptionRequest{
		SubscriptionID: subID.String(),
		Amount:         &newAmount,
	}

	mockSubRepo.On("GetByID", ctx, mock.Anything, subID).
		Return(existingSub, nil)
	mockDB.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context, pgx.Tx) error")).
		Return(nil)
	mockSubRepo.On("Update", ctx, mock.Anything, mock.AnythingOfType("*models.Subscription")).
		Return(nil)
	mockGateway.On("UpdateSubscription", ctx, "gw_sub_123", mock.AnythingOfType("*ports.UpdateSubscriptionRequest")).
		Return(&ports.SubscriptionResult{}, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	resp, err := service.UpdateSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, newAmount.String(), resp.Amount.String())
	mockDB.AssertExpectations(t)
	mockSubRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

func TestService_UpdateSubscription_CancelledSubscription(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, nil, mockLogger)

	ctx := context.Background()
	subID := uuid.New()
	cancelledSub := &models.Subscription{
		ID:     subID.String(),
		Status: models.SubStatusCancelled,
	}

	req := ports.ServiceUpdateSubscriptionRequest{
		SubscriptionID: subID.String(),
	}

	mockSubRepo.On("GetByID", ctx, mock.Anything, subID).
		Return(cancelledSub, nil)

	resp, err := service.UpdateSubscription(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "cannot update cancelled subscription")
}

func TestService_CancelSubscription_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockGateway := new(MockRecurringBillingGateway)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, mockGateway, mockLogger)

	ctx := context.Background()
	subID := uuid.New()
	activeSub := &models.Subscription{
		ID:                    subID.String(),
		Status:                models.SubStatusActive,
		GatewaySubscriptionID: "gw_sub_123",
	}

	req := ports.ServiceCancelSubscriptionRequest{
		SubscriptionID: subID.String(),
		Reason:         "Customer request",
	}

	mockSubRepo.On("GetByID", ctx, mock.Anything, subID).
		Return(activeSub, nil)
	mockDB.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context, pgx.Tx) error")).
		Return(nil)
	mockSubRepo.On("Update", ctx, mock.Anything, mock.AnythingOfType("*models.Subscription")).
		Return(nil)
	mockGateway.On("CancelSubscription", ctx, "gw_sub_123", true).
		Return(&ports.SubscriptionResult{}, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	resp, err := service.CancelSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.SubStatusCancelled, resp.Status)
	assert.NotNil(t, resp.CancelledAt)
	mockDB.AssertExpectations(t)
	mockSubRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

func TestService_CancelSubscription_AlreadyCancelled(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, nil, mockLogger)

	ctx := context.Background()
	subID := uuid.New()
	cancelledSub := &models.Subscription{
		ID:     subID.String(),
		Status: models.SubStatusCancelled,
	}

	req := ports.ServiceCancelSubscriptionRequest{
		SubscriptionID: subID.String(),
	}

	mockSubRepo.On("GetByID", ctx, mock.Anything, subID).
		Return(cancelledSub, nil)

	resp, err := service.CancelSubscription(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "already cancelled")
}

func TestService_PauseSubscription_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockGateway := new(MockRecurringBillingGateway)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, mockGateway, mockLogger)

	ctx := context.Background()
	subID := uuid.New()
	activeSub := &models.Subscription{
		ID:                    subID.String(),
		Status:                models.SubStatusActive,
		GatewaySubscriptionID: "gw_sub_123",
	}

	mockSubRepo.On("GetByID", ctx, mock.Anything, subID).
		Return(activeSub, nil)
	mockDB.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context, pgx.Tx) error")).
		Return(nil)
	mockSubRepo.On("Update", ctx, mock.Anything, mock.AnythingOfType("*models.Subscription")).
		Return(nil)
	mockGateway.On("PauseSubscription", ctx, "gw_sub_123").
		Return(&ports.SubscriptionResult{}, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	resp, err := service.PauseSubscription(ctx, subID.String())

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.SubStatusPaused, resp.Status)
	mockDB.AssertExpectations(t)
	mockSubRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

func TestService_PauseSubscription_NotActive(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, nil, mockLogger)

	ctx := context.Background()
	subID := uuid.New()
	pausedSub := &models.Subscription{
		ID:     subID.String(),
		Status: models.SubStatusPaused,
	}

	mockSubRepo.On("GetByID", ctx, mock.Anything, subID).
		Return(pausedSub, nil)

	resp, err := service.PauseSubscription(ctx, subID.String())

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "can only pause active subscriptions")
}

func TestService_ResumeSubscription_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockGateway := new(MockRecurringBillingGateway)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, mockGateway, mockLogger)

	ctx := context.Background()
	subID := uuid.New()
	pausedSub := &models.Subscription{
		ID:                    subID.String(),
		Status:                models.SubStatusPaused,
		Frequency:             models.FrequencyMonthly,
		GatewaySubscriptionID: "gw_sub_123",
	}

	mockSubRepo.On("GetByID", ctx, mock.Anything, subID).
		Return(pausedSub, nil)
	mockDB.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context, pgx.Tx) error")).
		Return(nil)
	mockSubRepo.On("Update", ctx, mock.Anything, mock.AnythingOfType("*models.Subscription")).
		Return(nil)
	mockGateway.On("ResumeSubscription", ctx, "gw_sub_123").
		Return(&ports.SubscriptionResult{}, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	resp, err := service.ResumeSubscription(ctx, subID.String())

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.SubStatusActive, resp.Status)
	assert.True(t, resp.NextBillingDate.After(time.Now().Add(-1*time.Minute)))
	mockDB.AssertExpectations(t)
	mockSubRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

func TestService_ResumeSubscription_NotPaused(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, nil, mockLogger)

	ctx := context.Background()
	subID := uuid.New()
	activeSub := &models.Subscription{
		ID:     subID.String(),
		Status: models.SubStatusActive,
	}

	mockSubRepo.On("GetByID", ctx, mock.Anything, subID).
		Return(activeSub, nil)

	resp, err := service.ResumeSubscription(ctx, subID.String())

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "can only resume paused subscriptions")
}

func TestService_ProcessDueBilling_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, nil, mockLogger)

	ctx := context.Background()
	asOfDate := time.Now()

	sub1 := &models.Subscription{
		ID:                 uuid.New().String(),
		MerchantID:         "MERCH123",
		CustomerID:         "CUST456",
		Amount:             decimal.NewFromFloat(29.99),
		Currency:           "USD",
		Frequency:          models.FrequencyMonthly,
		Status:             models.SubStatusActive,
		PaymentMethodToken: "tok_test123",
		NextBillingDate:    asOfDate.Add(-1 * time.Hour),
		MaxRetries:         3,
		FailureRetryCount:  0,
		FailureOption:      models.FailureForward,
	}

	mockSubRepo.On("ListActiveSubscriptionsDueForBilling", ctx, mock.Anything, asOfDate, int32(100)).
		Return([]*models.Subscription{sub1}, nil)
	mockPaymentService.On("Sale", ctx, mock.AnythingOfType("ports.ServiceSaleRequest")).
		Return(&ports.PaymentResponse{
			TransactionID: uuid.New().String(),
			Status:        models.StatusCaptured,
			IsApproved:    true,
		}, nil)
	mockDB.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context, pgx.Tx) error")).
		Return(nil)
	mockSubRepo.On("Update", ctx, mock.Anything, mock.AnythingOfType("*models.Subscription")).
		Return(nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	result, err := service.ProcessDueBilling(ctx, asOfDate, 100)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.ProcessedCount)
	assert.Equal(t, 1, result.SuccessCount)
	assert.Equal(t, 0, result.FailedCount)
	mockSubRepo.AssertExpectations(t)
	mockPaymentService.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}

func TestService_ProcessDueBilling_WithFailures(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, nil, mockLogger)

	ctx := context.Background()
	asOfDate := time.Now()

	sub1 := &models.Subscription{
		ID:                 uuid.New().String(),
		MerchantID:         "MERCH123",
		CustomerID:         "CUST456",
		Amount:             decimal.NewFromFloat(29.99),
		Currency:           "USD",
		Frequency:          models.FrequencyMonthly,
		Status:             models.SubStatusActive,
		PaymentMethodToken: "tok_test123",
		NextBillingDate:    asOfDate.Add(-1 * time.Hour),
		MaxRetries:         3,
		FailureRetryCount:  0,
		FailureOption:      models.FailureForward,
	}

	mockSubRepo.On("ListActiveSubscriptionsDueForBilling", ctx, mock.Anything, asOfDate, int32(100)).
		Return([]*models.Subscription{sub1}, nil)
	mockPaymentService.On("Sale", ctx, mock.AnythingOfType("ports.ServiceSaleRequest")).
		Return(nil, errors.New("insufficient funds"))
	mockDB.On("WithTransaction", ctx, mock.AnythingOfType("func(context.Context, pgx.Tx) error")).
		Return(nil)
	mockSubRepo.On("Update", ctx, mock.Anything, mock.AnythingOfType("*models.Subscription")).
		Return(nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	result, err := service.ProcessDueBilling(ctx, asOfDate, 100)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.ProcessedCount)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 1, result.FailedCount)
	assert.Len(t, result.Errors, 1)
	mockSubRepo.AssertExpectations(t)
	mockPaymentService.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}

func TestService_GetSubscription(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, nil, mockLogger)

	ctx := context.Background()
	subID := uuid.New()
	expectedSub := &models.Subscription{
		ID:         subID.String(),
		MerchantID: "MERCH123",
		CustomerID: "CUST456",
		Status:     models.SubStatusActive,
	}

	mockSubRepo.On("GetByID", ctx, mock.Anything, subID).
		Return(expectedSub, nil)

	sub, err := service.GetSubscription(ctx, subID.String())

	require.NoError(t, err)
	assert.NotNil(t, sub)
	assert.Equal(t, expectedSub.ID, sub.ID)
	mockSubRepo.AssertExpectations(t)
}

func TestService_ListCustomerSubscriptions(t *testing.T) {
	mockDB := new(MockDBPort)
	mockSubRepo := new(MockSubscriptionRepository)
	mockPaymentService := new(MockPaymentService)
	mockLogger := new(MockLogger)

	service := NewService(mockDB, mockSubRepo, mockPaymentService, nil, mockLogger)

	ctx := context.Background()
	merchantID := "MERCH123"
	customerID := "CUST456"

	expectedSubs := []*models.Subscription{
		{ID: uuid.New().String(), MerchantID: merchantID, CustomerID: customerID},
		{ID: uuid.New().String(), MerchantID: merchantID, CustomerID: customerID},
	}

	mockSubRepo.On("ListByCustomer", ctx, mock.Anything, merchantID, customerID).
		Return(expectedSubs, nil)

	subs, err := service.ListCustomerSubscriptions(ctx, merchantID, customerID)

	require.NoError(t, err)
	assert.NotNil(t, subs)
	assert.Len(t, subs, 2)
	mockSubRepo.AssertExpectations(t)
}

func TestService_CalculateNextBillingDate(t *testing.T) {
	service := &Service{}
	baseDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		frequency models.BillingFrequency
		expected  time.Time
	}{
		{
			name:      "Weekly",
			frequency: models.FrequencyWeekly,
			expected:  baseDate.AddDate(0, 0, 7),
		},
		{
			name:      "BiWeekly",
			frequency: models.FrequencyBiWeekly,
			expected:  baseDate.AddDate(0, 0, 14),
		},
		{
			name:      "Monthly",
			frequency: models.FrequencyMonthly,
			expected:  baseDate.AddDate(0, 1, 0),
		},
		{
			name:      "Yearly",
			frequency: models.FrequencyYearly,
			expected:  baseDate.AddDate(1, 0, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.calculateNextBillingDate(baseDate, tt.frequency)
			assert.Equal(t, tt.expected, result)
		})
	}
}
