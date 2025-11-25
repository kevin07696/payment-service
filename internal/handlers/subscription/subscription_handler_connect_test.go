package subscription

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	subscriptionv1 "github.com/kevin07696/payment-service/proto/subscription/v1"
)

// MockSubscriptionService is a mock implementation of SubscriptionService
type MockSubscriptionService struct {
	mock.Mock
}

func (m *MockSubscriptionService) CreateSubscription(ctx context.Context, req *ports.CreateSubscriptionRequest) (*domain.Subscription, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionService) UpdateSubscription(ctx context.Context, req *ports.UpdateSubscriptionRequest) (*domain.Subscription, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionService) CancelSubscription(ctx context.Context, req *ports.CancelSubscriptionRequest) (*domain.Subscription, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionService) PauseSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionService) ResumeSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionService) GetSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionService) ListCustomerSubscriptions(ctx context.Context, merchantID, customerID string) ([]*domain.Subscription, error) {
	args := m.Called(ctx, merchantID, customerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Subscription), args.Error(1)
}

func (m *MockSubscriptionService) ProcessDueBilling(ctx context.Context, asOfDate time.Time, batchSize int) (processed, success, failed int, errors []error) {
	args := m.Called(ctx, asOfDate, batchSize)
	return args.Int(0), args.Int(1), args.Int(2), args.Get(3).([]error)
}

// TestCreateSubscription_Validation tests input validation for CreateSubscription
func TestCreateSubscription_Validation(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	startDate := timestamppb.New(time.Now())

	tests := []struct {
		name          string
		request       *subscriptionv1.CreateSubscriptionRequest
		expectedCode  connect.Code
		expectedError string
	}{
		{
			name: "Missing merchant_id",
			request: &subscriptionv1.CreateSubscriptionRequest{
				MerchantId:      "",
				CustomerId:      "cust_123",
				AmountCents:     1000,
				Currency:        "USD",
				IntervalValue:   1,
				IntervalUnit:    subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH,
				PaymentMethodId: "pm_123",
				StartDate:       startDate,
			},
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "merchant_id is required",
		},
		{
			name: "Missing customer_id",
			request: &subscriptionv1.CreateSubscriptionRequest{
				MerchantId:      "merchant_123",
				CustomerId:      "",
				AmountCents:     1000,
				Currency:        "USD",
				IntervalValue:   1,
				IntervalUnit:    subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH,
				PaymentMethodId: "pm_123",
				StartDate:       startDate,
			},
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "customer_id is required",
		},
		{
			name: "Invalid amount_cents (zero)",
			request: &subscriptionv1.CreateSubscriptionRequest{
				MerchantId:      "merchant_123",
				CustomerId:      "cust_123",
				AmountCents:     0,
				Currency:        "USD",
				IntervalValue:   1,
				IntervalUnit:    subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH,
				PaymentMethodId: "pm_123",
				StartDate:       startDate,
			},
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "amount_cents must be greater than 0",
		},
		{
			name: "Invalid amount_cents (negative)",
			request: &subscriptionv1.CreateSubscriptionRequest{
				MerchantId:      "merchant_123",
				CustomerId:      "cust_123",
				AmountCents:     -100,
				Currency:        "USD",
				IntervalValue:   1,
				IntervalUnit:    subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH,
				PaymentMethodId: "pm_123",
				StartDate:       startDate,
			},
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "amount_cents must be greater than 0",
		},
		{
			name: "Missing currency",
			request: &subscriptionv1.CreateSubscriptionRequest{
				MerchantId:      "merchant_123",
				CustomerId:      "cust_123",
				AmountCents:     1000,
				Currency:        "",
				IntervalValue:   1,
				IntervalUnit:    subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH,
				PaymentMethodId: "pm_123",
				StartDate:       startDate,
			},
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "currency is required",
		},
		{
			name: "Invalid interval_value (zero)",
			request: &subscriptionv1.CreateSubscriptionRequest{
				MerchantId:      "merchant_123",
				CustomerId:      "cust_123",
				AmountCents:     1000,
				Currency:        "USD",
				IntervalValue:   0,
				IntervalUnit:    subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH,
				PaymentMethodId: "pm_123",
				StartDate:       startDate,
			},
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "interval_value must be positive",
		},
		{
			name: "Invalid interval_unit (unspecified)",
			request: &subscriptionv1.CreateSubscriptionRequest{
				MerchantId:      "merchant_123",
				CustomerId:      "cust_123",
				AmountCents:     1000,
				Currency:        "USD",
				IntervalValue:   1,
				IntervalUnit:    subscriptionv1.IntervalUnit_INTERVAL_UNIT_UNSPECIFIED,
				PaymentMethodId: "pm_123",
				StartDate:       startDate,
			},
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "interval_unit is required",
		},
		{
			name: "Missing payment_method_id",
			request: &subscriptionv1.CreateSubscriptionRequest{
				MerchantId:      "merchant_123",
				CustomerId:      "cust_123",
				AmountCents:     1000,
				Currency:        "USD",
				IntervalValue:   1,
				IntervalUnit:    subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH,
				PaymentMethodId: "",
				StartDate:       startDate,
			},
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "payment_method_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(tt.request)

			_, err := handler.CreateSubscription(context.Background(), req)

			require.Error(t, err)
			var connectErr *connect.Error
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, tt.expectedCode, connectErr.Code())
			assert.Contains(t, connectErr.Message(), tt.expectedError)
		})
	}

	// Verify no service calls were made for validation errors
	mockService.AssertExpectations(t)
}

// TestUpdateSubscription_Validation tests input validation for UpdateSubscription
func TestUpdateSubscription_Validation(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	tests := []struct {
		name           string
		subscriptionID string
		expectedCode   connect.Code
		expectedError  string
	}{
		{
			name:           "Missing subscription_id",
			subscriptionID: "",
			expectedCode:   connect.CodeInvalidArgument,
			expectedError:  "subscription_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(&subscriptionv1.UpdateSubscriptionRequest{
				SubscriptionId: tt.subscriptionID,
			})

			_, err := handler.UpdateSubscription(context.Background(), req)

			require.Error(t, err)
			var connectErr *connect.Error
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, tt.expectedCode, connectErr.Code())
			assert.Contains(t, connectErr.Message(), tt.expectedError)
		})
	}

	mockService.AssertExpectations(t)
}

// TestCancelSubscription_Validation tests input validation for CancelSubscription
func TestCancelSubscription_Validation(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	req := connect.NewRequest(&subscriptionv1.CancelSubscriptionRequest{
		SubscriptionId: "",
	})

	_, err := handler.CancelSubscription(context.Background(), req)

	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	assert.Contains(t, connectErr.Message(), "subscription_id is required")

	mockService.AssertExpectations(t)
}

// TestPauseSubscription_Validation tests input validation for PauseSubscription
func TestPauseSubscription_Validation(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	req := connect.NewRequest(&subscriptionv1.PauseSubscriptionRequest{
		SubscriptionId: "",
	})

	_, err := handler.PauseSubscription(context.Background(), req)

	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	assert.Contains(t, connectErr.Message(), "subscription_id is required")

	mockService.AssertExpectations(t)
}

// TestResumeSubscription_Validation tests input validation for ResumeSubscription
func TestResumeSubscription_Validation(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	req := connect.NewRequest(&subscriptionv1.ResumeSubscriptionRequest{
		SubscriptionId: "",
	})

	_, err := handler.ResumeSubscription(context.Background(), req)

	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	assert.Contains(t, connectErr.Message(), "subscription_id is required")

	mockService.AssertExpectations(t)
}

// TestGetSubscription_Validation tests input validation for GetSubscription
func TestGetSubscription_Validation(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	req := connect.NewRequest(&subscriptionv1.GetSubscriptionRequest{
		SubscriptionId: "",
	})

	_, err := handler.GetSubscription(context.Background(), req)

	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	assert.Contains(t, connectErr.Message(), "subscription_id is required")

	mockService.AssertExpectations(t)
}

// TestListCustomerSubscriptions_Validation tests input validation for ListCustomerSubscriptions
func TestListCustomerSubscriptions_Validation(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	tests := []struct {
		name          string
		merchantID    string
		customerID    string
		expectedCode  connect.Code
		expectedError string
	}{
		{
			name:          "Missing merchant_id",
			merchantID:    "",
			customerID:    "cust_123",
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "merchant_id is required",
		},
		{
			name:          "Missing customer_id",
			merchantID:    "merchant_123",
			customerID:    "",
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "customer_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(&subscriptionv1.ListCustomerSubscriptionsRequest{
				MerchantId: tt.merchantID,
				CustomerId: tt.customerID,
			})

			_, err := handler.ListCustomerSubscriptions(context.Background(), req)

			require.Error(t, err)
			var connectErr *connect.Error
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, tt.expectedCode, connectErr.Code())
			assert.Contains(t, connectErr.Message(), tt.expectedError)
		})
	}

	mockService.AssertExpectations(t)
}

// TestGetSubscription_NotFound tests NotFound error handling
func TestGetSubscription_NotFound(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	subscriptionID := "sub_123"
	mockService.On("GetSubscription", mock.Anything, subscriptionID).
		Return(nil, domain.ErrSubscriptionNotFound)

	req := connect.NewRequest(&subscriptionv1.GetSubscriptionRequest{
		SubscriptionId: subscriptionID,
	})

	_, err := handler.GetSubscription(context.Background(), req)

	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	assert.Contains(t, connectErr.Message(), "subscription not found")

	mockService.AssertExpectations(t)
}

// TestProcessDueBilling_BatchSizeDefault tests default batch size handling
func TestProcessDueBilling_BatchSizeDefault(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	tests := []struct {
		name              string
		inputBatchSize    int32
		expectedBatchSize int
	}{
		{
			name:              "Zero batch_size defaults to 100",
			inputBatchSize:    0,
			expectedBatchSize: 100,
		},
		{
			name:              "Negative batch_size defaults to 100",
			inputBatchSize:    -10,
			expectedBatchSize: 100,
		},
		{
			name:              "Valid batch_size unchanged",
			inputBatchSize:    50,
			expectedBatchSize: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asOfDate := time.Now()
			mockService.On("ProcessDueBilling", mock.Anything, mock.AnythingOfType("time.Time"), tt.expectedBatchSize).
				Return(0, 0, 0, []error{}).Once()

			req := connect.NewRequest(&subscriptionv1.ProcessDueBillingRequest{
				AsOfDate:  timestamppb.New(asOfDate),
				BatchSize: tt.inputBatchSize,
			})

			_, err := handler.ProcessDueBilling(context.Background(), req)
			require.NoError(t, err)
		})
	}

	mockService.AssertExpectations(t)
}

// TestCreateSubscription_MaxRetriesDefault tests default maxRetries handling
func TestCreateSubscription_MaxRetriesDefault(t *testing.T) {
	mockService := new(MockSubscriptionService)
	logger := zap.NewNop()
	handler := NewConnectHandler(mockService, logger)

	startDate := time.Now()

	// Mock expects maxRetries to be 3 (default)
	mockService.On("CreateSubscription", mock.Anything, mock.MatchedBy(func(req *ports.CreateSubscriptionRequest) bool {
		return req.MaxRetries == 3
	})).Return(&domain.Subscription{
		ID:         "sub_123",
		MerchantID: "merchant_123",
		CustomerID: "cust_123",
		Status:     domain.SubscriptionStatusActive,
	}, nil)

	req := connect.NewRequest(&subscriptionv1.CreateSubscriptionRequest{
		MerchantId:      "merchant_123",
		CustomerId:      "cust_123",
		AmountCents:     1000,
		Currency:        "USD",
		IntervalValue:   1,
		IntervalUnit:    subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH,
		PaymentMethodId: "pm_123",
		StartDate:       timestamppb.New(startDate),
		MaxRetries:      0, // Should default to 3
	})

	resp, err := handler.CreateSubscription(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	mockService.AssertExpectations(t)
}
