package subscription

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	subscriptionv1 "github.com/kevin07696/payment-service/api/proto/subscription/v1"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	"github.com/shopspring/decimal"
)

// MockSubscriptionService mocks the subscription service
type MockSubscriptionService struct {
	mock.Mock
}

func (m *MockSubscriptionService) CreateSubscription(ctx context.Context, req ports.ServiceCreateSubscriptionRequest) (*ports.ServiceSubscriptionResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ServiceSubscriptionResponse), args.Error(1)
}

func (m *MockSubscriptionService) UpdateSubscription(ctx context.Context, req ports.ServiceUpdateSubscriptionRequest) (*ports.ServiceSubscriptionResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ServiceSubscriptionResponse), args.Error(1)
}

func (m *MockSubscriptionService) CancelSubscription(ctx context.Context, req ports.ServiceCancelSubscriptionRequest) (*ports.ServiceSubscriptionResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ServiceSubscriptionResponse), args.Error(1)
}

func (m *MockSubscriptionService) PauseSubscription(ctx context.Context, subscriptionID string) (*ports.ServiceSubscriptionResponse, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ServiceSubscriptionResponse), args.Error(1)
}

func (m *MockSubscriptionService) ResumeSubscription(ctx context.Context, subscriptionID string) (*ports.ServiceSubscriptionResponse, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ServiceSubscriptionResponse), args.Error(1)
}

func (m *MockSubscriptionService) GetSubscription(ctx context.Context, subscriptionID string) (*models.Subscription, error) {
	args := m.Called(ctx, subscriptionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Subscription), args.Error(1)
}

func (m *MockSubscriptionService) ListCustomerSubscriptions(ctx context.Context, merchantID, customerID string) ([]*models.Subscription, error) {
	args := m.Called(ctx, merchantID, customerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Subscription), args.Error(1)
}

func (m *MockSubscriptionService) ProcessDueBilling(ctx context.Context, asOfDate time.Time, batchSize int) (*ports.BillingBatchResult, error) {
	args := m.Called(ctx, asOfDate, batchSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BillingBatchResult), args.Error(1)
}

// MockLogger mocks the logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
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

func (m *MockLogger) Fatal(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
}

func TestHandler_CreateSubscription_Success(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	now := time.Now()
	nextBilling := now.AddDate(0, 1, 0)

	req := &subscriptionv1.CreateSubscriptionRequest{
		MerchantId:         "MERCH-001",
		CustomerId:         "CUST-001",
		Amount:             "29.99",
		Currency:           "USD",
		Frequency:          subscriptionv1.BillingFrequency_BILLING_FREQUENCY_MONTHLY,
		PaymentMethodToken: "tok_pm_123",
		StartDate:          timestamppb.New(now),
		MaxRetries:         3,
		FailureOption:      subscriptionv1.FailureOption_FAILURE_OPTION_FORWARD,
		IdempotencyKey:     "idem-sub-001",
		Metadata:           map[string]string{"plan": "premium"},
	}

	mockLogger.On("Info", "gRPC CreateSubscription request received", mock.Anything).Return()
	mockService.On("CreateSubscription", ctx, mock.MatchedBy(func(req ports.ServiceCreateSubscriptionRequest) bool {
		return req.MerchantID == "MERCH-001" &&
			req.CustomerID == "CUST-001" &&
			req.Amount.Equal(decimal.NewFromFloat(29.99)) &&
			req.Currency == "USD" &&
			req.Frequency == models.FrequencyMonthly &&
			req.PaymentMethodToken == "tok_pm_123"
	})).Return(&ports.ServiceSubscriptionResponse{
		SubscriptionID:        "sub-001",
		MerchantID:            "MERCH-001",
		CustomerID:            "CUST-001",
		Amount:                decimal.NewFromFloat(29.99),
		Currency:              "USD",
		Frequency:             models.FrequencyMonthly,
		Status:                models.SubStatusActive,
		PaymentMethodToken:    "tok_pm_123",
		NextBillingDate:       nextBilling,
		GatewaySubscriptionID: "gw-sub-001",
		CreatedAt:             now,
		UpdatedAt:             now,
	}, nil)

	resp, err := handler.CreateSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "sub-001", resp.SubscriptionId)
	assert.Equal(t, "29.99", resp.Amount)
	assert.Equal(t, subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_ACTIVE, resp.Status)
	assert.Equal(t, subscriptionv1.BillingFrequency_BILLING_FREQUENCY_MONTHLY, resp.Frequency)
	mockService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestHandler_CreateSubscription_MissingMerchantID(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &subscriptionv1.CreateSubscriptionRequest{
		MerchantId:         "", // Missing
		CustomerId:         "CUST-001",
		Amount:             "29.99",
		Currency:           "USD",
		Frequency:          subscriptionv1.BillingFrequency_BILLING_FREQUENCY_MONTHLY,
		PaymentMethodToken: "tok_test",
		StartDate:          timestamppb.Now(),
	}

	mockLogger.On("Info", "gRPC CreateSubscription request received", mock.Anything).Return()

	resp, err := handler.CreateSubscription(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "merchant_id is required")
}

func TestHandler_CreateSubscription_InvalidAmount(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &subscriptionv1.CreateSubscriptionRequest{
		MerchantId:         "MERCH-001",
		CustomerId:         "CUST-001",
		Amount:             "invalid", // Invalid decimal
		Currency:           "USD",
		Frequency:          subscriptionv1.BillingFrequency_BILLING_FREQUENCY_MONTHLY,
		PaymentMethodToken: "tok_test",
		StartDate:          timestamppb.Now(),
	}

	mockLogger.On("Info", "gRPC CreateSubscription request received", mock.Anything).Return()

	resp, err := handler.CreateSubscription(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "invalid amount")
}

func TestHandler_CreateSubscription_ServiceError(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &subscriptionv1.CreateSubscriptionRequest{
		MerchantId:         "MERCH-001",
		CustomerId:         "CUST-001",
		Amount:             "29.99",
		Currency:           "USD",
		Frequency:          subscriptionv1.BillingFrequency_BILLING_FREQUENCY_MONTHLY,
		PaymentMethodToken: "tok_test",
		StartDate:          timestamppb.Now(),
	}

	mockLogger.On("Info", "gRPC CreateSubscription request received", mock.Anything).Return()
	mockLogger.On("Error", "CreateSubscription failed", mock.Anything).Return()
	mockService.On("CreateSubscription", ctx, mock.AnythingOfType("ports.ServiceCreateSubscriptionRequest")).
		Return(nil, errors.New("database error"))

	resp, err := handler.CreateSubscription(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "create subscription failed")
	mockLogger.AssertExpectations(t)
}

func TestHandler_UpdateSubscription_Success(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	now := time.Now()
	newAmount := "39.99"
	newFrequency := subscriptionv1.BillingFrequency_BILLING_FREQUENCY_YEARLY

	req := &subscriptionv1.UpdateSubscriptionRequest{
		SubscriptionId: "sub-update-001",
		Amount:         &newAmount,
		Frequency:      &newFrequency,
		IdempotencyKey: "idem-update-001",
	}

	mockLogger.On("Info", "gRPC UpdateSubscription request received", mock.Anything).Return()
	mockService.On("UpdateSubscription", ctx, mock.MatchedBy(func(req ports.ServiceUpdateSubscriptionRequest) bool {
		return req.SubscriptionID == "sub-update-001" &&
			req.Amount != nil &&
			req.Amount.Equal(decimal.NewFromFloat(39.99)) &&
			req.Frequency != nil &&
			*req.Frequency == models.FrequencyYearly
	})).Return(&ports.ServiceSubscriptionResponse{
		SubscriptionID:     "sub-update-001",
		MerchantID:         "MERCH-001",
		CustomerID:         "CUST-001",
		Amount:             decimal.NewFromFloat(39.99),
		Currency:           "USD",
		Frequency:          models.FrequencyYearly,
		Status:             models.SubStatusActive,
		PaymentMethodToken: "tok_pm_123",
		NextBillingDate:    now.AddDate(1, 0, 0),
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil)

	resp, err := handler.UpdateSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "sub-update-001", resp.SubscriptionId)
	assert.Equal(t, "39.99", resp.Amount)
	assert.Equal(t, subscriptionv1.BillingFrequency_BILLING_FREQUENCY_YEARLY, resp.Frequency)
	mockService.AssertExpectations(t)
}

func TestHandler_UpdateSubscription_MissingSubscriptionID(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	newAmount := "39.99"
	req := &subscriptionv1.UpdateSubscriptionRequest{
		SubscriptionId: "", // Missing
		Amount:         &newAmount,
	}

	mockLogger.On("Info", "gRPC UpdateSubscription request received", mock.Anything).Return()

	resp, err := handler.UpdateSubscription(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "subscription_id is required")
}

func TestHandler_CancelSubscription_Success(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	now := time.Now()
	cancelledTime := now

	req := &subscriptionv1.CancelSubscriptionRequest{
		SubscriptionId:    "sub-cancel-001",
		CancelAtPeriodEnd: false,
		Reason:            "User requested",
		IdempotencyKey:    "idem-cancel-001",
	}

	mockLogger.On("Info", "gRPC CancelSubscription request received", mock.Anything).Return()
	mockService.On("CancelSubscription", ctx, mock.MatchedBy(func(req ports.ServiceCancelSubscriptionRequest) bool {
		return req.SubscriptionID == "sub-cancel-001" &&
			req.CancelAtPeriodEnd == false &&
			req.Reason == "User requested" &&
			req.IdempotencyKey == "idem-cancel-001"
	})).Return(&ports.ServiceSubscriptionResponse{
		SubscriptionID:     "sub-cancel-001",
		MerchantID:         "MERCH-001",
		CustomerID:         "CUST-001",
		Amount:             decimal.NewFromFloat(29.99),
		Currency:           "USD",
		Frequency:          models.FrequencyMonthly,
		Status:             models.SubStatusCancelled,
		PaymentMethodToken: "tok_pm_123",
		NextBillingDate:    now,
		CreatedAt:          now,
		UpdatedAt:          now,
		CancelledAt:        &cancelledTime,
	}, nil)

	resp, err := handler.CancelSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "sub-cancel-001", resp.SubscriptionId)
	assert.Equal(t, subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_CANCELLED, resp.Status)
	assert.NotNil(t, resp.CancelledAt)
	mockService.AssertExpectations(t)
}

func TestHandler_PauseSubscription_Success(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	now := time.Now()

	req := &subscriptionv1.PauseSubscriptionRequest{
		SubscriptionId: "sub-pause-001",
	}

	mockLogger.On("Info", "gRPC PauseSubscription request received", mock.Anything).Return()
	mockService.On("PauseSubscription", ctx, "sub-pause-001").Return(&ports.ServiceSubscriptionResponse{
		SubscriptionID:     "sub-pause-001",
		MerchantID:         "MERCH-001",
		CustomerID:         "CUST-001",
		Amount:             decimal.NewFromFloat(29.99),
		Currency:           "USD",
		Frequency:          models.FrequencyMonthly,
		Status:             models.SubStatusPaused,
		PaymentMethodToken: "tok_pm_123",
		NextBillingDate:    now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil)

	resp, err := handler.PauseSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "sub-pause-001", resp.SubscriptionId)
	assert.Equal(t, subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAUSED, resp.Status)
	mockService.AssertExpectations(t)
}

func TestHandler_ResumeSubscription_Success(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	now := time.Now()

	req := &subscriptionv1.ResumeSubscriptionRequest{
		SubscriptionId: "sub-resume-001",
	}

	mockLogger.On("Info", "gRPC ResumeSubscription request received", mock.Anything).Return()
	mockService.On("ResumeSubscription", ctx, "sub-resume-001").Return(&ports.ServiceSubscriptionResponse{
		SubscriptionID:     "sub-resume-001",
		MerchantID:         "MERCH-001",
		CustomerID:         "CUST-001",
		Amount:             decimal.NewFromFloat(29.99),
		Currency:           "USD",
		Frequency:          models.FrequencyMonthly,
		Status:             models.SubStatusActive,
		PaymentMethodToken: "tok_pm_123",
		NextBillingDate:    now.AddDate(0, 1, 0),
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil)

	resp, err := handler.ResumeSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "sub-resume-001", resp.SubscriptionId)
	assert.Equal(t, subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_ACTIVE, resp.Status)
	mockService.AssertExpectations(t)
}

func TestHandler_GetSubscription_Success(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	now := time.Now()

	req := &subscriptionv1.GetSubscriptionRequest{
		SubscriptionId: "sub-get-001",
	}

	mockService.On("GetSubscription", ctx, "sub-get-001").Return(&models.Subscription{
		ID:                    "sub-get-001",
		MerchantID:            "MERCH-001",
		CustomerID:            "CUST-001",
		Amount:                decimal.NewFromFloat(29.99),
		Currency:              "USD",
		Frequency:             models.FrequencyMonthly,
		Status:                models.SubStatusActive,
		PaymentMethodToken:    "tok_pm_123",
		NextBillingDate:       now.AddDate(0, 1, 0),
		FailureRetryCount:     0,
		MaxRetries:            3,
		FailureOption:         models.FailureForward,
		GatewaySubscriptionID: "gw-sub-001",
		CreatedAt:             now,
		UpdatedAt:             now,
		Metadata:              map[string]string{"plan": "basic"},
	}, nil)

	resp, err := handler.GetSubscription(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "sub-get-001", resp.Id)
	assert.Equal(t, "MERCH-001", resp.MerchantId)
	assert.Equal(t, "29.99", resp.Amount)
	assert.Equal(t, subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_ACTIVE, resp.Status)
	assert.Equal(t, "basic", resp.Metadata["plan"])
	mockService.AssertExpectations(t)
}

func TestHandler_GetSubscription_NotFound(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &subscriptionv1.GetSubscriptionRequest{
		SubscriptionId: "sub-not-found",
	}

	mockService.On("GetSubscription", ctx, "sub-not-found").
		Return(nil, errors.New("not found"))

	resp, err := handler.GetSubscription(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "subscription not found")
}

func TestHandler_ListCustomerSubscriptions_Success(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	now := time.Now()

	req := &subscriptionv1.ListCustomerSubscriptionsRequest{
		MerchantId: "MERCH-001",
		CustomerId: "CUST-001",
	}

	subs := []*models.Subscription{
		{
			ID:                 "sub-001",
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-001",
			Amount:             decimal.NewFromFloat(29.99),
			Currency:           "USD",
			Frequency:          models.FrequencyMonthly,
			Status:             models.SubStatusActive,
			PaymentMethodToken: "tok_001",
			NextBillingDate:    now.AddDate(0, 1, 0),
			FailureRetryCount:  0,
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
		{
			ID:                 "sub-002",
			MerchantID:         "MERCH-001",
			CustomerID:         "CUST-001",
			Amount:             decimal.NewFromFloat(99.99),
			Currency:           "USD",
			Frequency:          models.FrequencyYearly,
			Status:             models.SubStatusCancelled,
			PaymentMethodToken: "tok_002",
			NextBillingDate:    now.AddDate(1, 0, 0),
			FailureRetryCount:  0,
			MaxRetries:         3,
			FailureOption:      models.FailureForward,
			CreatedAt:          now,
			UpdatedAt:          now,
		},
	}

	mockService.On("ListCustomerSubscriptions", ctx, "MERCH-001", "CUST-001").
		Return(subs, nil)

	resp, err := handler.ListCustomerSubscriptions(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Subscriptions, 2)
	assert.Equal(t, "sub-001", resp.Subscriptions[0].Id)
	assert.Equal(t, "sub-002", resp.Subscriptions[1].Id)
	mockService.AssertExpectations(t)
}

func TestHandler_ListCustomerSubscriptions_MissingMerchantID(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &subscriptionv1.ListCustomerSubscriptionsRequest{
		MerchantId: "", // Missing
		CustomerId: "CUST-001",
	}

	resp, err := handler.ListCustomerSubscriptions(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "merchant_id is required")
}

func TestHandler_ProcessDueBilling_Success(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	asOfDate := time.Now()

	req := &subscriptionv1.ProcessDueBillingRequest{
		AsOfDate:  timestamppb.New(asOfDate),
		BatchSize: 50,
	}

	mockLogger.On("Info", "gRPC ProcessDueBilling request received", mock.Anything).Return()
	mockService.On("ProcessDueBilling", ctx, mock.MatchedBy(func(t time.Time) bool {
		return t.Equal(asOfDate)
	}), 50).
		Return(&ports.BillingBatchResult{
			ProcessedCount: 10,
			SuccessCount:   8,
			FailedCount:    2,
			SkippedCount:   0,
			Errors: []ports.BillingError{
				{
					SubscriptionID: "sub-fail-001",
					CustomerID:     "CUST-001",
					Error:          "insufficient funds",
					Retriable:      true,
				},
				{
					SubscriptionID: "sub-fail-002",
					CustomerID:     "CUST-002",
					Error:          "card expired",
					Retriable:      false,
				},
			},
		}, nil)

	resp, err := handler.ProcessDueBilling(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(10), resp.ProcessedCount)
	assert.Equal(t, int32(8), resp.SuccessCount)
	assert.Equal(t, int32(2), resp.FailedCount)
	assert.Len(t, resp.Errors, 2)
	assert.Equal(t, "sub-fail-001", resp.Errors[0].SubscriptionId)
	assert.True(t, resp.Errors[0].Retriable)
	mockService.AssertExpectations(t)
}

func TestHandler_ProcessDueBilling_DefaultBatchSize(t *testing.T) {
	mockService := new(MockSubscriptionService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	asOfDate := time.Now()

	req := &subscriptionv1.ProcessDueBillingRequest{
		AsOfDate:  timestamppb.New(asOfDate),
		BatchSize: 0, // Should default to 100
	}

	mockLogger.On("Info", "gRPC ProcessDueBilling request received", mock.Anything).Return()
	mockService.On("ProcessDueBilling", ctx, mock.MatchedBy(func(t time.Time) bool {
		return t.Equal(asOfDate)
	}), 100).
		Return(&ports.BillingBatchResult{
			ProcessedCount: 5,
			SuccessCount:   5,
			FailedCount:    0,
			SkippedCount:   0,
			Errors:         []ports.BillingError{},
		}, nil)

	resp, err := handler.ProcessDueBilling(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(5), resp.ProcessedCount)
	mockService.AssertExpectations(t)
}

func TestConversions_BillingFrequency(t *testing.T) {
	tests := []struct {
		name      string
		proto     subscriptionv1.BillingFrequency
		model     models.BillingFrequency
		modelBack models.BillingFrequency
	}{
		{"weekly", subscriptionv1.BillingFrequency_BILLING_FREQUENCY_WEEKLY, models.FrequencyWeekly, models.FrequencyWeekly},
		{"biweekly", subscriptionv1.BillingFrequency_BILLING_FREQUENCY_BIWEEKLY, models.FrequencyBiWeekly, models.FrequencyBiWeekly},
		{"monthly", subscriptionv1.BillingFrequency_BILLING_FREQUENCY_MONTHLY, models.FrequencyMonthly, models.FrequencyMonthly},
		{"yearly", subscriptionv1.BillingFrequency_BILLING_FREQUENCY_YEARLY, models.FrequencyYearly, models.FrequencyYearly},
		{"unspecified", subscriptionv1.BillingFrequency_BILLING_FREQUENCY_UNSPECIFIED, models.FrequencyMonthly, models.FrequencyMonthly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Proto to model
			result := toModelBillingFrequency(tt.proto)
			assert.Equal(t, tt.model, result)

			// Model back to proto
			if tt.proto != subscriptionv1.BillingFrequency_BILLING_FREQUENCY_UNSPECIFIED {
				protoResult := toProtoBillingFrequency(tt.modelBack)
				assert.Equal(t, tt.proto, protoResult)
			}
		})
	}
}

func TestConversions_SubscriptionStatus(t *testing.T) {
	tests := []struct {
		name     string
		model    models.SubscriptionStatus
		expected subscriptionv1.SubscriptionStatus
	}{
		{"active", models.SubStatusActive, subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_ACTIVE},
		{"paused", models.SubStatusPaused, subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAUSED},
		{"cancelled", models.SubStatusCancelled, subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_CANCELLED},
		{"unknown", models.SubscriptionStatus("unknown"), subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toProtoSubscriptionStatus(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConversions_FailureOption(t *testing.T) {
	tests := []struct {
		name      string
		proto     subscriptionv1.FailureOption
		model     models.FailureOption
		modelBack models.FailureOption
	}{
		{"forward", subscriptionv1.FailureOption_FAILURE_OPTION_FORWARD, models.FailureForward, models.FailureForward},
		{"skip", subscriptionv1.FailureOption_FAILURE_OPTION_SKIP, models.FailureSkip, models.FailureSkip},
		{"pause", subscriptionv1.FailureOption_FAILURE_OPTION_PAUSE, models.FailurePause, models.FailurePause},
		{"unspecified", subscriptionv1.FailureOption_FAILURE_OPTION_UNSPECIFIED, models.FailureForward, models.FailureForward},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Proto to model
			result := toModelFailureOption(tt.proto)
			assert.Equal(t, tt.model, result)

			// Model back to proto
			if tt.proto != subscriptionv1.FailureOption_FAILURE_OPTION_UNSPECIFIED {
				protoResult := toProtoFailureOption(tt.modelBack)
				assert.Equal(t, tt.proto, protoResult)
			}
		})
	}
}
