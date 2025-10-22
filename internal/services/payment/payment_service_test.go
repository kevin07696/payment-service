package payment_test

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
	"github.com/kevin07696/payment-service/internal/services/payment"
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
	// Execute the function with a nil transaction for testing
	return fn(ctx, nil)
}

func (m *MockDBPort) WithReadOnlyTransaction(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return fn(ctx, nil)
}

// MockTransactionRepository mocks the transaction repository
type MockTransactionRepository struct {
	mock.Mock
}

func (m *MockTransactionRepository) Create(ctx context.Context, tx ports.DBTX, transaction *models.Transaction) error {
	args := m.Called(ctx, tx, transaction)
	return args.Error(0)
}

func (m *MockTransactionRepository) GetByID(ctx context.Context, db ports.DBTX, id uuid.UUID) (*models.Transaction, error) {
	args := m.Called(ctx, db, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) GetByIdempotencyKey(ctx context.Context, db ports.DBTX, key string) (*models.Transaction, error) {
	args := m.Called(ctx, db, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) UpdateStatus(ctx context.Context, tx ports.DBTX, id uuid.UUID, status models.TransactionStatus, gatewayTxnID, responseCode, responseMessage *string) error {
	args := m.Called(ctx, tx, id, status, gatewayTxnID, responseCode, responseMessage)
	return args.Error(0)
}

func (m *MockTransactionRepository) ListByMerchant(ctx context.Context, db ports.DBTX, merchantID string, limit, offset int32) ([]*models.Transaction, error) {
	args := m.Called(ctx, db, merchantID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Transaction), args.Error(1)
}

func (m *MockTransactionRepository) ListByCustomer(ctx context.Context, db ports.DBTX, merchantID, customerID string, limit, offset int32) ([]*models.Transaction, error) {
	args := m.Called(ctx, db, merchantID, customerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Transaction), args.Error(1)
}

// MockCreditCardGateway mocks the credit card gateway
type MockCreditCardGateway struct {
	mock.Mock
}

func (m *MockCreditCardGateway) Authorize(ctx context.Context, req *ports.AuthorizeRequest) (*ports.PaymentResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResult), args.Error(1)
}

func (m *MockCreditCardGateway) Capture(ctx context.Context, req *ports.CaptureRequest) (*ports.PaymentResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResult), args.Error(1)
}

func (m *MockCreditCardGateway) Void(ctx context.Context, transactionID string) (*ports.PaymentResult, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResult), args.Error(1)
}

func (m *MockCreditCardGateway) Refund(ctx context.Context, req *ports.RefundRequest) (*ports.PaymentResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PaymentResult), args.Error(1)
}

func (m *MockCreditCardGateway) VerifyAccount(ctx context.Context, req *ports.VerifyAccountRequest) (*ports.VerificationResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.VerificationResult), args.Error(1)
}

// MockLogger mocks the logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Debug(msg string, fields ...ports.Field) {
	m.Called(msg, fields)
}

// Test Authorize - Successful authorization
func TestService_Authorize_Success(t *testing.T) {
	// Setup mocks
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	req := ports.ServiceAuthorizeRequest{
		MerchantID: "MERCH123",
		CustomerID: "CUST456",
		Amount:     decimal.NewFromFloat(100.00),
		Currency:   "USD",
		Token:      "tok_test123",
		BillingInfo: models.BillingInfo{
			FirstName: "John",
			LastName:  "Doe",
		},
		IdempotencyKey: "idem-key-123",
		Metadata:       map[string]string{"order_id": "ORDER123"},
	}

	// Mock expectations
	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "idem-key-123").
		Return((*models.Transaction)(nil), errors.New("not found"))

	mockTxRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Transaction")).
		Return(nil)

	mockGateway.On("Authorize", ctx, mock.AnythingOfType("*ports.AuthorizeRequest")).
		Return(&ports.PaymentResult{
			TransactionID:        "txn-123",
			GatewayTransactionID: "gw-txn-456",
			Amount:               decimal.NewFromFloat(100.00),
			Status:               models.StatusAuthorized,
			ResponseCode:         "00",
			Message:              "Approved",
			Timestamp:            time.Now(),
		}, nil)

	mockTxRepo.On("UpdateStatus", ctx, mock.Anything, mock.AnythingOfType("uuid.UUID"),
		models.StatusAuthorized, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.Authorize(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.StatusAuthorized, resp.Status)
	assert.True(t, resp.IsApproved)
	assert.False(t, resp.IsDeclined)

	mockTxRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

// Test Authorize - Idempotency check returns existing transaction
func TestService_Authorize_IdempotencyReturnsExisting(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	req := ports.ServiceAuthorizeRequest{
		MerchantID:     "MERCH123",
		Amount:         decimal.NewFromFloat(100.00),
		Currency:       "USD",
		Token:          "tok_test123",
		IdempotencyKey: "idem-key-existing",
	}

	existingTxn := &models.Transaction{
		ID:                   uuid.New().String(),
		MerchantID:           "MERCH123",
		Amount:               decimal.NewFromFloat(100.00),
		Currency:             "USD",
		Status:               models.StatusAuthorized,
		GatewayTransactionID: "gw-existing",
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "idem-key-existing").
		Return(existingTxn, nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.Authorize(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, existingTxn.ID, resp.TransactionID)
	assert.Equal(t, models.StatusAuthorized, resp.Status)

	// Gateway should NOT be called
	mockGateway.AssertNotCalled(t, "Authorize")
	mockTxRepo.AssertExpectations(t)
}

// Test Authorize - Gateway error
func TestService_Authorize_GatewayError(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	req := ports.ServiceAuthorizeRequest{
		MerchantID:     "MERCH123",
		Amount:         decimal.NewFromFloat(100.00),
		Currency:       "USD",
		Token:          "tok_test123",
		IdempotencyKey: "idem-gateway-error", // Add idempotency key
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "idem-gateway-error").
		Return((*models.Transaction)(nil), errors.New("not found"))

	mockTxRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Transaction")).
		Return(nil)

	mockGateway.On("Authorize", ctx, mock.AnythingOfType("*ports.AuthorizeRequest")).
		Return((*ports.PaymentResult)(nil), errors.New("gateway timeout"))

	mockTxRepo.On("UpdateStatus", ctx, mock.Anything, mock.AnythingOfType("uuid.UUID"),
		models.StatusFailed, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.Authorize(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "gateway authorize")

	mockTxRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

// Test Capture - Successful capture
func TestService_Capture_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	originalTxnID := uuid.New()
	req := ports.ServiceCaptureRequest{
		TransactionID:  originalTxnID.String(),
		IdempotencyKey: "capture-idem-123",
	}

	originalTxn := &models.Transaction{
		ID:                   originalTxnID.String(),
		MerchantID:           "MERCH123",
		CustomerID:           "CUST456",
		Amount:               decimal.NewFromFloat(100.00),
		Currency:             "USD",
		Status:               models.StatusAuthorized,
		Type:                 models.TypeAuthorization,
		GatewayTransactionID: "gw-auth-123",
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "capture-idem-123").
		Return((*models.Transaction)(nil), errors.New("not found"))

	mockTxRepo.On("GetByID", ctx, mock.Anything, originalTxnID).
		Return(originalTxn, nil)

	mockTxRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Transaction")).
		Return(nil)

	mockGateway.On("Capture", ctx, mock.AnythingOfType("*ports.CaptureRequest")).
		Return(&ports.PaymentResult{
			TransactionID:        "capture-txn-123",
			GatewayTransactionID: "gw-capture-456",
			Amount:               decimal.NewFromFloat(100.00),
			Status:               models.StatusCaptured,
			ResponseCode:         "00",
			Message:              "Captured",
			Timestamp:            time.Now(),
		}, nil)

	mockTxRepo.On("UpdateStatus", ctx, mock.Anything, mock.AnythingOfType("uuid.UUID"),
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.Capture(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.StatusCaptured, resp.Status)
	assert.True(t, resp.IsApproved)

	mockTxRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

// Test Capture - Cannot capture non-authorized transaction
func TestService_Capture_InvalidStatus(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	originalTxnID := uuid.New()
	req := ports.ServiceCaptureRequest{
		TransactionID: originalTxnID.String(),
	}

	originalTxn := &models.Transaction{
		ID:         originalTxnID.String(),
		MerchantID: "MERCH123",
		Amount:     decimal.NewFromFloat(100.00),
		Status:     models.StatusFailed, // Not authorized
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "").
		Return((*models.Transaction)(nil), errors.New("not found"))

	mockTxRepo.On("GetByID", ctx, mock.Anything, originalTxnID).
		Return(originalTxn, nil)

	// Execute
	resp, err := service.Capture(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not in authorized state")

	mockGateway.AssertNotCalled(t, "Capture")
}

// Test Sale - Successful sale (authorize + capture)
func TestService_Sale_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	req := ports.ServiceSaleRequest{
		MerchantID: "MERCH123",
		CustomerID: "CUST456",
		Amount:     decimal.NewFromFloat(150.00),
		Currency:   "USD",
		Token:      "tok_sale123",
		BillingInfo: models.BillingInfo{
			FirstName: "Jane",
			LastName:  "Smith",
		},
		IdempotencyKey: "sale-idem-123",
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "sale-idem-123").
		Return((*models.Transaction)(nil), errors.New("not found"))

	mockTxRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Transaction")).
		Return(nil)

	// Sale uses Authorize with Capture=true
	mockGateway.On("Authorize", ctx, mock.MatchedBy(func(req *ports.AuthorizeRequest) bool {
		return req.Capture == true // Verify capture flag is set
	})).Return(&ports.PaymentResult{
		TransactionID:        "sale-txn-123",
		GatewayTransactionID: "gw-sale-456",
		Amount:               decimal.NewFromFloat(150.00),
		Status:               models.StatusCaptured,
		ResponseCode:         "00",
		Message:              "Approved and Captured",
		Timestamp:            time.Now(),
	}, nil)

	mockTxRepo.On("UpdateStatus", ctx, mock.Anything, mock.AnythingOfType("uuid.UUID"),
		models.StatusCaptured, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.Sale(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.StatusCaptured, resp.Status)
	assert.True(t, resp.IsApproved)

	mockTxRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

// Test Void - Successful void
func TestService_Void_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	originalTxnID := uuid.New()
	req := ports.ServiceVoidRequest{
		TransactionID:  originalTxnID.String(),
		IdempotencyKey: "void-idem-123",
	}

	originalTxn := &models.Transaction{
		ID:                   originalTxnID.String(),
		MerchantID:           "MERCH123",
		Amount:               decimal.NewFromFloat(100.00),
		Status:               models.StatusAuthorized,
		GatewayTransactionID: "gw-auth-123",
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "void-idem-123").
		Return((*models.Transaction)(nil), errors.New("not found"))

	mockTxRepo.On("GetByID", ctx, mock.Anything, originalTxnID).
		Return(originalTxn, nil)

	mockTxRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Transaction")).
		Return(nil)

	mockGateway.On("Void", ctx, "gw-auth-123").
		Return(&ports.PaymentResult{
			TransactionID:        "void-txn-123",
			GatewayTransactionID: "gw-void-456",
			Status:               models.StatusVoided,
			ResponseCode:         "00",
			Message:              "Voided",
			Timestamp:            time.Now(),
		}, nil)

	mockTxRepo.On("UpdateStatus", ctx, mock.Anything, mock.AnythingOfType("uuid.UUID"),
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.Void(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.StatusVoided, resp.Status)
	assert.True(t, resp.IsApproved)

	mockTxRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

// Test Refund - Successful refund
func TestService_Refund_Success(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	originalTxnID := uuid.New()
	refundAmount := decimal.NewFromFloat(50.00)
	req := ports.ServiceRefundRequest{
		TransactionID:  originalTxnID.String(),
		Amount:         &refundAmount,
		IdempotencyKey: "refund-idem-123",
		Reason:         "Customer request",
	}

	originalTxn := &models.Transaction{
		ID:                   originalTxnID.String(),
		MerchantID:           "MERCH123",
		Amount:               decimal.NewFromFloat(100.00),
		Status:               models.StatusCaptured,
		GatewayTransactionID: "gw-capture-123",
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "refund-idem-123").
		Return((*models.Transaction)(nil), errors.New("not found"))

	mockTxRepo.On("GetByID", ctx, mock.Anything, originalTxnID).
		Return(originalTxn, nil)

	mockTxRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Transaction")).
		Return(nil)

	mockGateway.On("Refund", ctx, mock.AnythingOfType("*ports.RefundRequest")).
		Return(&ports.PaymentResult{
			TransactionID:        "refund-txn-123",
			GatewayTransactionID: "gw-refund-456",
			Amount:               decimal.NewFromFloat(50.00),
			Status:               models.StatusRefunded,
			ResponseCode:         "00",
			Message:              "Refunded",
			Timestamp:            time.Now(),
		}, nil)

	mockTxRepo.On("UpdateStatus", ctx, mock.Anything, mock.AnythingOfType("uuid.UUID"),
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.Refund(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.StatusRefunded, resp.Status)
	assert.True(t, resp.IsApproved)

	mockTxRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

// Test Refund - Cannot refund non-captured transaction
func TestService_Refund_InvalidStatus(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	originalTxnID := uuid.New()
	req := ports.ServiceRefundRequest{
		TransactionID: originalTxnID.String(),
	}

	originalTxn := &models.Transaction{
		ID:     originalTxnID.String(),
		Amount: decimal.NewFromFloat(100.00),
		Status: models.StatusAuthorized, // Not captured
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "").
		Return((*models.Transaction)(nil), errors.New("not found"))

	mockTxRepo.On("GetByID", ctx, mock.Anything, originalTxnID).
		Return(originalTxn, nil)

	// Execute
	resp, err := service.Refund(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not captured")

	mockGateway.AssertNotCalled(t, "Refund")
}

// Test GetTransaction
func TestService_GetTransaction(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	txnID := uuid.New()

	expectedTxn := &models.Transaction{
		ID:         txnID.String(),
		MerchantID: "MERCH123",
		Amount:     decimal.NewFromFloat(100.00),
		Status:     models.StatusCaptured,
	}

	mockTxRepo.On("GetByID", ctx, mock.Anything, txnID).
		Return(expectedTxn, nil)

	// Execute
	txn, err := service.GetTransaction(ctx, txnID.String())

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedTxn.ID, txn.ID)
	assert.Equal(t, expectedTxn.Amount, txn.Amount)

	mockTxRepo.AssertExpectations(t)
}

// Test AVS/CVV Integration - Authorize with AVS/CVV values
func TestService_Authorize_WithAVSCVV(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	req := ports.ServiceAuthorizeRequest{
		MerchantID: "MERCH123",
		CustomerID: "CUST456",
		Amount:     decimal.NewFromFloat(100.00),
		Currency:   "USD",
		Token:      "tok_test123",
		BillingInfo: models.BillingInfo{
			FirstName: "John",
			LastName:  "Doe",
			Address:   "123 Main St",
			City:      "Wilmington",
			State:     "DE",
			ZipCode:   "19801",
		},
		IdempotencyKey: "idem-avs-123",
	}

	// Mock expectations
	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "idem-avs-123").
		Return((*models.Transaction)(nil), errors.New("not found"))

	var capturedTransaction *models.Transaction
	mockTxRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Transaction")).
		Run(func(args mock.Arguments) {
			capturedTransaction = args.Get(2).(*models.Transaction)
		}).
		Return(nil)

	// Gateway returns AVS/CVV values
	mockGateway.On("Authorize", ctx, mock.AnythingOfType("*ports.AuthorizeRequest")).
		Return(&ports.PaymentResult{
			TransactionID:        "txn-avs-123",
			GatewayTransactionID: "gw-txn-avs-456",
			Amount:               decimal.NewFromFloat(100.00),
			Status:               models.StatusAuthorized,
			ResponseCode:         "00",
			Message:              "Approved",
			AVSResponse:          "Y", // Full AVS match
			CVVResponse:          "M", // CVV match
			Timestamp:            time.Now(),
		}, nil)

	mockTxRepo.On("UpdateStatus", ctx, mock.Anything, mock.AnythingOfType("uuid.UUID"),
		models.StatusAuthorized, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.Authorize(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, models.StatusAuthorized, resp.Status)
	assert.Equal(t, "Y", resp.AVSResponse) // Verify AVS flows through
	assert.Equal(t, "M", resp.CVVResponse) // Verify CVV flows through
	assert.True(t, resp.IsApproved)

	// Verify Transaction was created with AVS/CVV values
	require.NotNil(t, capturedTransaction)
	assert.Equal(t, "Y", capturedTransaction.AVSResponse)
	assert.Equal(t, "M", capturedTransaction.CVVResponse)

	mockTxRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}

// Test AVS/CVV Integration - Sale with different AVS codes
func TestService_Sale_WithAVSCVV_Variations(t *testing.T) {
	testCases := []struct {
		name        string
		avsCode     string
		cvvCode     string
		description string
	}{
		{
			name:        "Full Match",
			avsCode:     "Y",
			cvvCode:     "M",
			description: "Both AVS and CVV match",
		},
		{
			name:        "ZIP Only Match",
			avsCode:     "Z",
			cvvCode:     "M",
			description: "ZIP matches, address doesn't",
		},
		{
			name:        "Address Only Match",
			avsCode:     "A",
			cvvCode:     "M",
			description: "Address matches, ZIP doesn't",
		},
		{
			name:        "No AVS Match",
			avsCode:     "N",
			cvvCode:     "M",
			description: "Neither AVS field matches",
		},
		{
			name:        "CVV No Match",
			avsCode:     "Y",
			cvvCode:     "N",
			description: "AVS matches but CVV doesn't",
		},
		{
			name:        "AVS Unavailable",
			avsCode:     "U",
			cvvCode:     "M",
			description: "AVS service unavailable",
		},
		{
			name:        "CVV Not Processed",
			avsCode:     "Y",
			cvvCode:     "P",
			description: "CVV not processed by issuer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := new(MockDBPort)
			mockTxRepo := new(MockTransactionRepository)
			mockGateway := new(MockCreditCardGateway)
			mockLogger := new(MockLogger)

			service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

			ctx := context.Background()
			req := ports.ServiceSaleRequest{
				MerchantID:     "MERCH123",
				CustomerID:     "CUST456",
				Amount:         decimal.NewFromFloat(100.00),
				Currency:       "USD",
				Token:          "tok_test123",
				IdempotencyKey: "idem-sale-" + tc.name,
			}

			mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "idem-sale-"+tc.name).
				Return((*models.Transaction)(nil), errors.New("not found"))

			var capturedTransaction *models.Transaction
			mockTxRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Transaction")).
				Run(func(args mock.Arguments) {
					capturedTransaction = args.Get(2).(*models.Transaction)
				}).
				Return(nil)

			mockGateway.On("Authorize", ctx, mock.AnythingOfType("*ports.AuthorizeRequest")).
				Return(&ports.PaymentResult{
					TransactionID:        "txn-sale-" + tc.name,
					GatewayTransactionID: "gw-txn-sale-" + tc.name,
					Amount:               decimal.NewFromFloat(100.00),
					Status:               models.StatusCaptured,
					ResponseCode:         "00",
					Message:              "Approved",
					AVSResponse:          tc.avsCode,
					CVVResponse:          tc.cvvCode,
					Timestamp:            time.Now(),
				}, nil)

			mockTxRepo.On("UpdateStatus", ctx, mock.Anything, mock.AnythingOfType("uuid.UUID"),
				models.StatusCaptured, mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			mockLogger.On("Info", mock.Anything, mock.Anything).Return()

			// Execute
			resp, err := service.Sale(ctx, req)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tc.avsCode, resp.AVSResponse, tc.description)
			assert.Equal(t, tc.cvvCode, resp.CVVResponse, tc.description)

			// Verify Transaction has correct AVS/CVV
			require.NotNil(t, capturedTransaction)
			assert.Equal(t, tc.avsCode, capturedTransaction.AVSResponse)
			assert.Equal(t, tc.cvvCode, capturedTransaction.CVVResponse)

			mockTxRepo.AssertExpectations(t)
			mockGateway.AssertExpectations(t)
		})
	}
}

// Test AVS/CVV Integration - GetTransaction returns AVS/CVV
func TestService_GetTransaction_ReturnsAVSCVV(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	txnID := uuid.New()

	expectedTxn := &models.Transaction{
		ID:                   txnID.String(),
		MerchantID:           "MERCH123",
		Amount:               decimal.NewFromFloat(100.00),
		Status:               models.StatusAuthorized,
		GatewayTransactionID: "gw-123",
		GatewayResponseCode:  "00",
		GatewayResponseMsg:   "Approved",
		AVSResponse:          "Y",
		CVVResponse:          "M",
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	mockTxRepo.On("GetByID", ctx, mock.Anything, txnID).
		Return(expectedTxn, nil)

	// Execute
	txn, err := service.GetTransaction(ctx, txnID.String())

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedTxn.ID, txn.ID)
	assert.Equal(t, expectedTxn.AVSResponse, txn.AVSResponse)
	assert.Equal(t, expectedTxn.CVVResponse, txn.CVVResponse)

	mockTxRepo.AssertExpectations(t)
}

// Test AVS/CVV Integration - Capture with AVS/CVV
func TestService_Capture_WithAVSCVV(t *testing.T) {
	mockDB := new(MockDBPort)
	mockTxRepo := new(MockTransactionRepository)
	mockGateway := new(MockCreditCardGateway)
	mockLogger := new(MockLogger)

	service := payment.NewService(mockDB, mockTxRepo, mockGateway, mockLogger)

	ctx := context.Background()
	authTxnID := uuid.New()

	authTxn := &models.Transaction{
		ID:                   authTxnID.String(),
		MerchantID:           "MERCH123",
		Amount:               decimal.NewFromFloat(100.00),
		Status:               models.StatusAuthorized,
		GatewayTransactionID: "gw-auth-123",
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	req := ports.ServiceCaptureRequest{
		TransactionID:  authTxnID.String(),
		Amount:         &authTxn.Amount,
		IdempotencyKey: "idem-cap-avs",
	}

	mockTxRepo.On("GetByIdempotencyKey", ctx, mock.Anything, "idem-cap-avs").
		Return((*models.Transaction)(nil), errors.New("not found"))

	mockTxRepo.On("GetByID", ctx, mock.Anything, authTxnID).
		Return(authTxn, nil)

	var capturedTransaction *models.Transaction
	mockTxRepo.On("Create", ctx, mock.Anything, mock.AnythingOfType("*models.Transaction")).
		Run(func(args mock.Arguments) {
			capturedTransaction = args.Get(2).(*models.Transaction)
		}).
		Return(nil)

	mockGateway.On("Capture", ctx, mock.AnythingOfType("*ports.CaptureRequest")).
		Return(&ports.PaymentResult{
			TransactionID:        authTxnID.String(),
			GatewayTransactionID: "gw-cap-123",
			Amount:               decimal.NewFromFloat(100.00),
			Status:               models.StatusCaptured,
			ResponseCode:         "00",
			Message:              "Captured",
			AVSResponse:          "Y",
			CVVResponse:          "M",
			Timestamp:            time.Now(),
		}, nil)

	// UpdateStatus is called twice: once for capture txn, once for original txn
	mockTxRepo.On("UpdateStatus", ctx, mock.Anything, mock.AnythingOfType("uuid.UUID"),
		models.StatusCaptured, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	mockTxRepo.On("UpdateStatus", ctx, mock.Anything, authTxnID,
		models.StatusCaptured, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Execute
	resp, err := service.Capture(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Y", resp.AVSResponse)
	assert.Equal(t, "M", resp.CVVResponse)

	// Verify capture transaction was created with AVS/CVV
	require.NotNil(t, capturedTransaction)
	assert.Equal(t, "Y", capturedTransaction.AVSResponse)
	assert.Equal(t, "M", capturedTransaction.CVVResponse)

	mockTxRepo.AssertExpectations(t)
	mockGateway.AssertExpectations(t)
}
