package payment

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

	paymentv1 "github.com/kevin07696/payment-service/api/proto/payment/v1"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	"github.com/shopspring/decimal"
)

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

func TestHandler_Authorize_Success(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.AuthorizeRequest{
		MerchantId:     "MERCH-001",
		CustomerId:     "CUST-001",
		Amount:         "100.00",
		Currency:       "USD",
		Token:          "tok_test_123",
		IdempotencyKey: "idem-001",
		BillingInfo: &paymentv1.BillingInfo{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john@example.com",
			Address: &paymentv1.Address{
				Street1:    "123 Main St",
				City:       "Wilmington",
				State:      "DE",
				PostalCode: "19801",
				Country:    "US",
			},
		},
		Metadata: map[string]string{"order_id": "12345"},
	}

	mockLogger.On("Info", "gRPC Authorize request received", mock.Anything).Return()
	mockService.On("Authorize", ctx, mock.MatchedBy(func(req ports.ServiceAuthorizeRequest) bool {
		return req.MerchantID == "MERCH-001" &&
			req.CustomerID == "CUST-001" &&
			req.Amount.Equal(decimal.NewFromFloat(100.00)) &&
			req.Currency == "USD" &&
			req.Token == "tok_test_123"
	})).Return(&ports.PaymentResponse{
		TransactionID:        "txn-001",
		Amount:               decimal.NewFromFloat(100.00),
		Currency:             "USD",
		Status:               models.StatusAuthorized,
		GatewayTransactionID: "gw-txn-001",
		GatewayResponseCode:  "00",
		GatewayResponseMsg:   "Approved",
		IsApproved:           true,
	}, nil)

	resp, err := handler.Authorize(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "txn-001", resp.TransactionId)
	amt, _ := decimal.NewFromString(resp.Amount)
	assert.True(t, amt.Equal(decimal.NewFromFloat(100.00)))
	assert.Equal(t, paymentv1.TransactionStatus_TRANSACTION_STATUS_AUTHORIZED, resp.Status)
	assert.True(t, resp.IsApproved)
	mockService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestHandler_Authorize_MissingMerchantID(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.AuthorizeRequest{
		MerchantId: "", // Missing
		CustomerId: "CUST-001",
		Amount:     "100.00",
		Currency:   "USD",
		Token:      "tok_test",
	}

	mockLogger.On("Info", "gRPC Authorize request received", mock.Anything).Return()

	resp, err := handler.Authorize(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "merchant_id is required")
}

func TestHandler_Authorize_InvalidAmount(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.AuthorizeRequest{
		MerchantId: "MERCH-001",
		CustomerId: "CUST-001",
		Amount:     "invalid", // Invalid decimal
		Currency:   "USD",
		Token:      "tok_test",
	}

	mockLogger.On("Info", "gRPC Authorize request received", mock.Anything).Return()

	resp, err := handler.Authorize(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "invalid amount")
}

func TestHandler_Authorize_ServiceError(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.AuthorizeRequest{
		MerchantId: "MERCH-001",
		CustomerId: "CUST-001",
		Amount:     "100.00",
		Currency:   "USD",
		Token:      "tok_test",
		BillingInfo: &paymentv1.BillingInfo{
			FirstName: "Test",
			LastName:  "User",
			Address: &paymentv1.Address{
				Street1:    "123 Test St",
				City:       "Test City",
				State:      "TS",
				PostalCode: "12345",
				Country:    "US",
			},
		},
	}

	mockLogger.On("Info", "gRPC Authorize request received", mock.Anything).Return()
	mockLogger.On("Error", "Authorize failed", mock.Anything).Return()
	mockService.On("Authorize", ctx, mock.AnythingOfType("ports.ServiceAuthorizeRequest")).
		Return(nil, errors.New("insufficient funds"))

	resp, err := handler.Authorize(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "authorization failed")
	mockLogger.AssertExpectations(t)
}

func TestHandler_Capture_Success(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.CaptureRequest{
		TransactionId:  "txn-auth-001",
		Amount:         "75.50",
		IdempotencyKey: "idem-capture-001",
	}

	mockLogger.On("Info", "gRPC Capture request received", mock.Anything).Return()
	mockService.On("Capture", ctx, mock.MatchedBy(func(req ports.ServiceCaptureRequest) bool {
		return req.TransactionID == "txn-auth-001" &&
			req.Amount != nil &&
			req.Amount.Equal(decimal.NewFromFloat(75.50)) &&
			req.IdempotencyKey == "idem-capture-001"
	})).Return(&ports.PaymentResponse{
		TransactionID:  "txn-auth-001",
		Amount:         decimal.NewFromFloat(75.50),
		Currency:       "USD",
		Status:         models.StatusCaptured,
		IsApproved:     true,
	}, nil)

	resp, err := handler.Capture(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "txn-auth-001", resp.TransactionId)
	assert.Equal(t, paymentv1.TransactionStatus_TRANSACTION_STATUS_CAPTURED, resp.Status)
	mockService.AssertExpectations(t)
}

func TestHandler_Capture_MissingTransactionID(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.CaptureRequest{
		TransactionId: "", // Missing
		Amount:        "100.00",
	}

	mockLogger.On("Info", "gRPC Capture request received", mock.Anything).Return()

	resp, err := handler.Capture(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "transaction_id is required")
}

func TestHandler_Sale_Success(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.SaleRequest{
		MerchantId: "MERCH-001",
		CustomerId: "CUST-001",
		Amount:     "50.00",
		Currency:   "USD",
		Token:      "tok_sale_123",
		BillingInfo: &paymentv1.BillingInfo{
			FirstName: "Jane",
			LastName:  "Smith",
			Address: &paymentv1.Address{
				Street1:    "456 Main St",
				City:       "New York",
				State:      "NY",
				PostalCode: "10001",
				Country:    "US",
			},
		},
	}

	mockLogger.On("Info", "gRPC Sale request received", mock.Anything).Return()
	mockService.On("Sale", ctx, mock.AnythingOfType("ports.ServiceSaleRequest")).
		Return(&ports.PaymentResponse{
			TransactionID:  "txn-sale-001",
			Amount:         decimal.NewFromFloat(50.00),
			Currency:       "USD",
			Status:         models.StatusCaptured,
			IsApproved:     true,
		}, nil)

	resp, err := handler.Sale(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "txn-sale-001", resp.TransactionId)
	assert.Equal(t, paymentv1.TransactionStatus_TRANSACTION_STATUS_CAPTURED, resp.Status)
	mockService.AssertExpectations(t)
}

func TestHandler_Void_Success(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.VoidRequest{
		TransactionId:  "txn-void-001",
		IdempotencyKey: "idem-void-001",
	}

	mockLogger.On("Info", "gRPC Void request received", mock.Anything).Return()
	mockService.On("Void", ctx, mock.MatchedBy(func(req ports.ServiceVoidRequest) bool {
		return req.TransactionID == "txn-void-001" &&
			req.IdempotencyKey == "idem-void-001"
	})).Return(&ports.PaymentResponse{
		TransactionID:  "txn-void-001",
		Status:         models.StatusVoided,
		IsApproved:     true,
	}, nil)

	resp, err := handler.Void(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "txn-void-001", resp.TransactionId)
	assert.Equal(t, paymentv1.TransactionStatus_TRANSACTION_STATUS_VOIDED, resp.Status)
	mockService.AssertExpectations(t)
}

func TestHandler_Refund_Success(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.RefundRequest{
		TransactionId:  "txn-refund-001",
		Amount:         "25.00",
		Reason:         "Customer requested",
		IdempotencyKey: "idem-refund-001",
	}

	mockLogger.On("Info", "gRPC Refund request received", mock.Anything).Return()
	mockService.On("Refund", ctx, mock.MatchedBy(func(req ports.ServiceRefundRequest) bool {
		return req.TransactionID == "txn-refund-001" &&
			req.Amount != nil &&
			req.Amount.Equal(decimal.NewFromFloat(25.00)) &&
			req.Reason == "Customer requested" &&
			req.IdempotencyKey == "idem-refund-001"
	})).Return(&ports.PaymentResponse{
		TransactionID:  "txn-refund-001",
		Amount:         decimal.NewFromFloat(25.00),
		Status:         models.StatusRefunded,
		IsApproved:     true,
	}, nil)

	resp, err := handler.Refund(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "txn-refund-001", resp.TransactionId)
	assert.Equal(t, paymentv1.TransactionStatus_TRANSACTION_STATUS_REFUNDED, resp.Status)
	mockService.AssertExpectations(t)
}

func TestHandler_GetTransaction_Success(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.GetTransactionRequest{
		TransactionId: "txn-get-001",
	}

	now := time.Now()
	mockService.On("GetTransaction", ctx, "txn-get-001").Return(&models.Transaction{
		ID:                   "txn-get-001",
		MerchantID:           "MERCH-001",
		CustomerID:           "CUST-001",
		Amount:               decimal.NewFromFloat(100.00),
		Currency:             "USD",
		Status:               models.StatusAuthorized,
		Type:                 models.TypeAuthorization,
		PaymentMethodType:    models.PaymentMethodCreditCard,
		GatewayTransactionID: "gw-001",
		PaymentMethodToken:   "tok_001",
		GatewayResponseCode:  "00",
		GatewayResponseMsg:   "Approved",
		IdempotencyKey:       "idem-001",
		CreatedAt:            now,
		UpdatedAt:            now,
		Metadata:             map[string]string{"test": "value"},
	}, nil)

	resp, err := handler.GetTransaction(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "txn-get-001", resp.Id)
	assert.Equal(t, "MERCH-001", resp.MerchantId)

	// Compare decimal amounts properly
	amt, _ := decimal.NewFromString(resp.Amount)
	assert.True(t, amt.Equal(decimal.NewFromFloat(100.00)))

	assert.Equal(t, paymentv1.TransactionStatus_TRANSACTION_STATUS_AUTHORIZED, resp.Status)
	assert.Equal(t, paymentv1.TransactionType_TRANSACTION_TYPE_AUTHORIZATION, resp.Type)
	mockService.AssertExpectations(t)
}

func TestHandler_GetTransaction_NotFound(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.GetTransactionRequest{
		TransactionId: "txn-not-found",
	}

	mockService.On("GetTransaction", ctx, "txn-not-found").
		Return(nil, errors.New("not found"))

	resp, err := handler.GetTransaction(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "transaction not found")
}

func TestHandler_ListTransactions_NotImplemented(t *testing.T) {
	mockService := new(MockPaymentService)
	mockLogger := new(MockLogger)
	handler := NewHandler(mockService, mockLogger)

	ctx := context.Background()
	req := &paymentv1.ListTransactionsRequest{
		MerchantId: "MERCH-001",
	}

	resp, err := handler.ListTransactions(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unimplemented, st.Code())
}

func TestConversions_TransactionStatus(t *testing.T) {
	tests := []struct {
		name     string
		model    models.TransactionStatus
		expected paymentv1.TransactionStatus
	}{
		{"pending", models.StatusPending, paymentv1.TransactionStatus_TRANSACTION_STATUS_PENDING},
		{"authorized", models.StatusAuthorized, paymentv1.TransactionStatus_TRANSACTION_STATUS_AUTHORIZED},
		{"captured", models.StatusCaptured, paymentv1.TransactionStatus_TRANSACTION_STATUS_CAPTURED},
		{"voided", models.StatusVoided, paymentv1.TransactionStatus_TRANSACTION_STATUS_VOIDED},
		{"refunded", models.StatusRefunded, paymentv1.TransactionStatus_TRANSACTION_STATUS_REFUNDED},
		{"failed", models.StatusFailed, paymentv1.TransactionStatus_TRANSACTION_STATUS_FAILED},
		{"unknown", models.TransactionStatus("unknown"), paymentv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toProtoTransactionStatus(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConversions_TransactionType(t *testing.T) {
	tests := []struct {
		name     string
		model    models.TransactionType
		expected paymentv1.TransactionType
	}{
		{"authorization", models.TypeAuthorization, paymentv1.TransactionType_TRANSACTION_TYPE_AUTHORIZATION},
		{"capture", models.TypeCapture, paymentv1.TransactionType_TRANSACTION_TYPE_CAPTURE},
		{"sale", models.TypeSale, paymentv1.TransactionType_TRANSACTION_TYPE_SALE},
		{"void", models.TypeVoid, paymentv1.TransactionType_TRANSACTION_TYPE_VOID},
		{"refund", models.TypeRefund, paymentv1.TransactionType_TRANSACTION_TYPE_REFUND},
		{"unknown", models.TransactionType("unknown"), paymentv1.TransactionType_TRANSACTION_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toProtoTransactionType(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConversions_PaymentMethodType(t *testing.T) {
	tests := []struct {
		name     string
		model    models.PaymentMethodType
		expected paymentv1.PaymentMethodType
	}{
		{"credit_card", models.PaymentMethodCreditCard, paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD},
		{"ach", models.PaymentMethodACH, paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_ACH},
		{"unknown", models.PaymentMethodType("unknown"), paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toProtoPaymentMethodType(tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}
