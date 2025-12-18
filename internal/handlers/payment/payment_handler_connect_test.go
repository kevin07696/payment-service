package payment

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
)

// =============================================================================
// Mock Payment Service
// =============================================================================

type MockPaymentService struct {
	mock.Mock
}

func (m *MockPaymentService) Authorize(ctx context.Context, req *ports.AuthorizeRequest) (*domain.Transaction, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockPaymentService) Capture(ctx context.Context, req *ports.CaptureRequest) (*domain.Transaction, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockPaymentService) Sale(ctx context.Context, req *ports.SaleRequest) (*domain.Transaction, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockPaymentService) Void(ctx context.Context, req *ports.VoidRequest) (*domain.Transaction, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockPaymentService) Refund(ctx context.Context, req *ports.RefundRequest) (*domain.Transaction, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

// Ensure MockPaymentService implements ports.PaymentService
var _ ports.PaymentService = (*MockPaymentService)(nil)

// =============================================================================
// Mock Transaction Query Service
// =============================================================================

type MockQueryService struct {
	mock.Mock
}

func (m *MockQueryService) GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error) {
	args := m.Called(ctx, transactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockQueryService) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Transaction), args.Error(1)
}

func (m *MockQueryService) ListTransactions(ctx context.Context, filters *ports.ListTransactionsFilters) ([]*domain.Transaction, int, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*domain.Transaction), args.Int(1), args.Error(2)
}

func (m *MockQueryService) GetTransactionsByGroup(ctx context.Context, parentTransactionID string) ([]*domain.Transaction, error) {
	args := m.Called(ctx, parentTransactionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Transaction), args.Error(1)
}

// Ensure MockQueryService implements ports.TransactionQueryService
var _ ports.TransactionQueryService = (*MockQueryService)(nil)

// =============================================================================
// Test Helper Functions
// =============================================================================

func createTestHandler(t *testing.T) (*ConnectHandler, *MockPaymentService, *MockQueryService) {
	t.Helper()
	mockPayment := new(MockPaymentService)
	mockQuery := new(MockQueryService)
	logger := zap.NewNop()

	handler := NewConnectHandler(mockPayment, mockQuery, logger)
	return handler, mockPayment, mockQuery
}

func createTestTransaction() *domain.Transaction {
	customerID := uuid.New().String()
	authCode := "AUTH123"
	authResp := "00"
	return &domain.Transaction{
		ID:          uuid.New().String(),
		MerchantID:  uuid.New().String(),
		CustomerID:  &customerID,
		AmountCents: 1000,
		Currency:    "USD",
		Status:      domain.TransactionStatusApproved,
		Type:        domain.TransactionTypeSale,
		AuthCode:    &authCode,
		AuthResp:    &authResp,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// =============================================================================
// Sale Tests - Validation
// =============================================================================

func TestSale_WithValidCard_ReturnsApproved(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	expectedTx := createTestTransaction()
	mockPayment.On("Sale", mock.Anything, mock.AnythingOfType("*ports.SaleRequest")).
		Return(expectedTx, nil)

	paymentMethodID := uuid.New().String()
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:    uuid.New().String(),
		AmountCents:   1000,
		Currency:      "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
	})

	resp, err := handler.Sale(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Payment processed successfully", resp.Msg.Message)
	assert.Equal(t, expectedTx.ID, resp.Msg.TransactionId)
	mockPayment.AssertExpectations(t)
}

func TestSale_MissingMerchantID_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	paymentMethodID := uuid.New().String()
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:    "", // Missing
		AmountCents:   1000,
		Currency:      "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
	})

	resp, err := handler.Sale(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "merchant_id")
}

func TestSale_MissingPaymentMethod_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:  uuid.New().String(),
		AmountCents: 1000,
		Currency:    "USD",
		// PaymentMethod is missing
	})

	resp, err := handler.Sale(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "payment_method")
}

func TestSale_ZeroAmount_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	paymentMethodID := uuid.New().String()
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:    uuid.New().String(),
		AmountCents:   0, // Invalid
		Currency:      "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
	})

	resp, err := handler.Sale(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSale_NegativeAmount_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	paymentMethodID := uuid.New().String()
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:    uuid.New().String(),
		AmountCents:   -100, // Negative
		Currency:      "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
	})

	resp, err := handler.Sale(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSale_ServiceError_ReturnsAppropriateError(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	// Simulate a domain error from the service
	domainErr := domain.NewDomainError(domain.ErrorCodePMExpired, "Payment method expired")
	mockPayment.On("Sale", mock.Anything, mock.AnythingOfType("*ports.SaleRequest")).
		Return(nil, domainErr)

	paymentMethodID := uuid.New().String()
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:    uuid.New().String(),
		AmountCents:   1000,
		Currency:      "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
	})

	resp, err := handler.Sale(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	mockPayment.AssertExpectations(t)
}

// =============================================================================
// Idempotency Tests (per testing/idempotency.md)
// =============================================================================

func TestSale_DuplicateIdempotencyKey_ReturnsSameTransaction(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	idempotencyKey := uuid.New().String()
	expectedTx := createTestTransaction()

	// First call creates the transaction
	mockPayment.On("Sale", mock.Anything, mock.MatchedBy(func(req *ports.SaleRequest) bool {
		return req.IdempotencyKey != nil && *req.IdempotencyKey == idempotencyKey
	})).Return(expectedTx, nil).Times(2) // Called twice, returns same

	merchantID := uuid.New().String()
	paymentMethodID := uuid.New().String()

	// First request
	req1 := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:     merchantID,
		AmountCents:    1000,
		Currency:       "USD",
		PaymentMethod:  &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
		IdempotencyKey: idempotencyKey,
	})
	resp1, err1 := handler.Sale(context.Background(), req1)
	require.NoError(t, err1)

	// Second request with same idempotency key
	req2 := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:     merchantID,
		AmountCents:    1000,
		Currency:       "USD",
		PaymentMethod:  &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
		IdempotencyKey: idempotencyKey,
	})
	resp2, err2 := handler.Sale(context.Background(), req2)
	require.NoError(t, err2)

	// Should return the same transaction ID
	assert.Equal(t, resp1.Msg.TransactionId, resp2.Msg.TransactionId)
	mockPayment.AssertExpectations(t)
}

func TestAuthorize_DuplicateIdempotencyKey_ReturnsSameTransaction(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	idempotencyKey := uuid.New().String()
	expectedTx := createTestTransaction()
	expectedTx.Type = domain.TransactionTypeAuth

	mockPayment.On("Authorize", mock.Anything, mock.MatchedBy(func(req *ports.AuthorizeRequest) bool {
		return req.IdempotencyKey != nil && *req.IdempotencyKey == idempotencyKey
	})).Return(expectedTx, nil).Times(2)

	merchantID := uuid.New().String()
	paymentMethodID := uuid.New().String()

	// First request
	req1 := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:     merchantID,
		AmountCents:    1000,
		Currency:       "USD",
		PaymentMethod:  &paymentv1.AuthorizeRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
		IdempotencyKey: idempotencyKey,
	})
	resp1, err1 := handler.Authorize(context.Background(), req1)
	require.NoError(t, err1)

	// Second request with same idempotency key
	req2 := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:     merchantID,
		AmountCents:    1000,
		Currency:       "USD",
		PaymentMethod:  &paymentv1.AuthorizeRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
		IdempotencyKey: idempotencyKey,
	})
	resp2, err2 := handler.Authorize(context.Background(), req2)
	require.NoError(t, err2)

	assert.Equal(t, resp1.Msg.TransactionId, resp2.Msg.TransactionId)
	mockPayment.AssertExpectations(t)
}

func TestCapture_DuplicateIdempotencyKey_ReturnsSameTransaction(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	idempotencyKey := uuid.New().String()
	expectedTx := createTestTransaction()
	expectedTx.Type = domain.TransactionTypeCapture

	mockPayment.On("Capture", mock.Anything, mock.MatchedBy(func(req *ports.CaptureRequest) bool {
		return req.IdempotencyKey != nil && *req.IdempotencyKey == idempotencyKey
	})).Return(expectedTx, nil).Times(2)

	transactionID := uuid.New().String()

	// First request
	req1 := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId:  transactionID,
		IdempotencyKey: idempotencyKey,
	})
	resp1, err1 := handler.Capture(context.Background(), req1)
	require.NoError(t, err1)

	// Second request with same idempotency key
	req2 := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId:  transactionID,
		IdempotencyKey: idempotencyKey,
	})
	resp2, err2 := handler.Capture(context.Background(), req2)
	require.NoError(t, err2)

	assert.Equal(t, resp1.Msg.TransactionId, resp2.Msg.TransactionId)
	mockPayment.AssertExpectations(t)
}

// =============================================================================
// Concurrency Tests (per testing/concurrency.md - run with -race)
// =============================================================================

func TestSale_ConcurrentRequests_NoRaceConditions(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	// Allow concurrent calls - each returns a new transaction
	mockPayment.On("Sale", mock.Anything, mock.AnythingOfType("*ports.SaleRequest")).
		Return(createTestTransaction(), nil).
		Maybe() // Allow any number of calls

	const numRequests = 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()

			req := connect.NewRequest(&paymentv1.SaleRequest{
				MerchantId:    uuid.New().String(),
				AmountCents:   1000,
				Currency:      "USD",
				PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: uuid.New().String()},
			})

			_, err := handler.Sale(context.Background(), req)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// All requests should succeed
	for err := range results {
		assert.NoError(t, err)
	}
}

func TestSale_ConcurrentDuplicateKeys_CreatesOnlyOne(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	idempotencyKey := uuid.New().String()
	expectedTx := createTestTransaction()

	// Mock returns the same transaction for all calls with same idempotency key
	mockPayment.On("Sale", mock.Anything, mock.MatchedBy(func(req *ports.SaleRequest) bool {
		return req.IdempotencyKey != nil && *req.IdempotencyKey == idempotencyKey
	})).Return(expectedTx, nil)

	const numRequests = 5
	var wg sync.WaitGroup
	wg.Add(numRequests)

	merchantID := uuid.New().String()
	paymentMethodID := uuid.New().String()

	results := make(chan string, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()

			req := connect.NewRequest(&paymentv1.SaleRequest{
				MerchantId:     merchantID,
				AmountCents:    1000,
				Currency:       "USD",
				PaymentMethod:  &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: paymentMethodID},
				IdempotencyKey: idempotencyKey,
			})

			resp, err := handler.Sale(context.Background(), req)
			if err == nil {
				results <- resp.Msg.TransactionId
			} else {
				results <- ""
			}
		}()
	}

	wg.Wait()
	close(results)

	// All responses should have the same transaction ID
	var txIDs []string
	for txID := range results {
		if txID != "" {
			txIDs = append(txIDs, txID)
		}
	}

	require.NotEmpty(t, txIDs, "At least one request should succeed")
	firstID := txIDs[0]
	for _, id := range txIDs {
		assert.Equal(t, firstID, id, "All concurrent requests with same idempotency key should return same transaction")
	}
}

// =============================================================================
// Authorize Tests
// =============================================================================

func TestAuthorize_WithValidCard_ReturnsAuthorized(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	expectedTx := createTestTransaction()
	expectedTx.Type = domain.TransactionTypeAuth
	mockPayment.On("Authorize", mock.Anything, mock.AnythingOfType("*ports.AuthorizeRequest")).
		Return(expectedTx, nil)

	req := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:    uuid.New().String(),
		AmountCents:   1000,
		Currency:      "USD",
		PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{PaymentMethodId: uuid.New().String()},
	})

	resp, err := handler.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Payment authorized", resp.Msg.Message)
	mockPayment.AssertExpectations(t)
}

func TestAuthorize_MissingMerchantID_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	req := connect.NewRequest(&paymentv1.AuthorizeRequest{
		MerchantId:    "", // Missing
		AmountCents:   1000,
		Currency:      "USD",
		PaymentMethod: &paymentv1.AuthorizeRequest_PaymentMethodId{PaymentMethodId: uuid.New().String()},
	})

	resp, err := handler.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// =============================================================================
// Capture Tests
// =============================================================================

func TestCapture_ValidTransaction_ReturnsCaptured(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	expectedTx := createTestTransaction()
	expectedTx.Type = domain.TransactionTypeCapture
	mockPayment.On("Capture", mock.Anything, mock.AnythingOfType("*ports.CaptureRequest")).
		Return(expectedTx, nil)

	req := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: uuid.New().String(),
	})

	resp, err := handler.Capture(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Payment captured", resp.Msg.Message)
	mockPayment.AssertExpectations(t)
}

func TestCapture_MissingTransactionID_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	req := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: "", // Missing
	})

	resp, err := handler.Capture(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "transaction_id")
}

func TestCapture_WithPartialAmount_PassesAmountToService(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	transactionID := uuid.New().String()
	partialAmount := int64(500)

	expectedTx := createTestTransaction()
	expectedTx.AmountCents = partialAmount

	mockPayment.On("Capture", mock.Anything, mock.MatchedBy(func(req *ports.CaptureRequest) bool {
		return req.TransactionID == transactionID &&
			req.AmountCents != nil && *req.AmountCents == partialAmount
	})).Return(expectedTx, nil)

	req := connect.NewRequest(&paymentv1.CaptureRequest{
		TransactionId: transactionID,
		AmountCents:   partialAmount,
	})

	resp, err := handler.Capture(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	mockPayment.AssertExpectations(t)
}

// =============================================================================
// Void Tests
// =============================================================================

func TestVoid_ValidTransaction_ReturnsVoided(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	expectedTx := createTestTransaction()
	expectedTx.Type = domain.TransactionTypeVoid
	mockPayment.On("Void", mock.Anything, mock.AnythingOfType("*ports.VoidRequest")).
		Return(expectedTx, nil)

	req := connect.NewRequest(&paymentv1.VoidRequest{
		TransactionId: uuid.New().String(),
	})

	resp, err := handler.Void(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Payment cancelled", resp.Msg.Message)
	mockPayment.AssertExpectations(t)
}

func TestVoid_MissingTransactionID_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	req := connect.NewRequest(&paymentv1.VoidRequest{
		TransactionId: "", // Missing
	})

	resp, err := handler.Void(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// =============================================================================
// Refund Tests
// =============================================================================

func TestRefund_ValidTransaction_ReturnsRefunded(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	expectedTx := createTestTransaction()
	expectedTx.Type = domain.TransactionTypeRefund
	mockPayment.On("Refund", mock.Anything, mock.AnythingOfType("*ports.RefundRequest")).
		Return(expectedTx, nil)

	req := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId: uuid.New().String(),
	})

	resp, err := handler.Refund(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Refund processed", resp.Msg.Message)
	mockPayment.AssertExpectations(t)
}

func TestRefund_MissingTransactionID_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	req := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId: "", // Missing
	})

	resp, err := handler.Refund(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRefund_WithPartialAmount_PassesAmountToService(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	transactionID := uuid.New().String()
	refundAmount := int64(300)

	expectedTx := createTestTransaction()
	expectedTx.AmountCents = refundAmount
	expectedTx.Type = domain.TransactionTypeRefund

	mockPayment.On("Refund", mock.Anything, mock.MatchedBy(func(req *ports.RefundRequest) bool {
		return req.ParentTransactionID == transactionID &&
			req.AmountCents != nil && *req.AmountCents == refundAmount
	})).Return(expectedTx, nil)

	req := connect.NewRequest(&paymentv1.RefundRequest{
		TransactionId: transactionID,
		AmountCents:   refundAmount,
		Reason:        "Customer request",
	})

	resp, err := handler.Refund(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	mockPayment.AssertExpectations(t)
}

// =============================================================================
// GetTransaction Tests
// =============================================================================

func TestGetTransaction_ValidID_ReturnsTransaction(t *testing.T) {
	handler, _, mockQuery := createTestHandler(t)

	transactionID := uuid.New().String()
	expectedTx := createTestTransaction()
	expectedTx.ID = transactionID

	mockQuery.On("GetTransaction", mock.Anything, transactionID).
		Return(expectedTx, nil)

	req := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: transactionID,
	})

	resp, err := handler.GetTransaction(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, transactionID, resp.Msg.Id)
	mockQuery.AssertExpectations(t)
}

func TestGetTransaction_MissingID_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	req := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: "", // Missing
	})

	resp, err := handler.GetTransaction(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGetTransaction_NotFound_ReturnsNotFoundError(t *testing.T) {
	handler, _, mockQuery := createTestHandler(t)

	transactionID := uuid.New().String()
	mockQuery.On("GetTransaction", mock.Anything, transactionID).
		Return(nil, domain.NewDomainError(domain.ErrorCodeTxnNotFound, "Transaction not found"))

	req := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: transactionID,
	})

	resp, err := handler.GetTransaction(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	mockQuery.AssertExpectations(t)
}

// =============================================================================
// ListTransactions Tests
// =============================================================================

func TestListTransactions_ValidMerchantID_ReturnsTransactions(t *testing.T) {
	handler, _, mockQuery := createTestHandler(t)

	merchantID := uuid.New().String()
	txs := []*domain.Transaction{
		createTestTransaction(),
		createTestTransaction(),
	}

	mockQuery.On("ListTransactions", mock.Anything, mock.MatchedBy(func(f *ports.ListTransactionsFilters) bool {
		return f.MerchantID != nil && *f.MerchantID == merchantID
	})).Return(txs, 2, nil)

	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: merchantID,
		Limit:      10,
	})

	resp, err := handler.ListTransactions(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Msg.Transactions, 2)
	assert.Equal(t, int32(2), resp.Msg.TotalCount)
	mockQuery.AssertExpectations(t)
}

func TestListTransactions_MissingMerchantID_ReturnsInvalidArgument(t *testing.T) {
	handler, _, _ := createTestHandler(t)

	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: "", // Missing
	})

	resp, err := handler.ListTransactions(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestListTransactions_WithFilters_PassesFiltersToService(t *testing.T) {
	handler, _, mockQuery := createTestHandler(t)

	merchantID := uuid.New().String()
	customerID := uuid.New().String()
	orderID := "ORDER-123"

	mockQuery.On("ListTransactions", mock.Anything, mock.MatchedBy(func(f *ports.ListTransactionsFilters) bool {
		return f.MerchantID != nil && *f.MerchantID == merchantID &&
			f.CustomerID != nil && *f.CustomerID == customerID &&
			f.OrderID != nil && *f.OrderID == orderID
	})).Return([]*domain.Transaction{}, 0, nil)

	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: merchantID,
		CustomerId: customerID,
		OrderId:    orderID,
		Limit:      10,
	})

	resp, err := handler.ListTransactions(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	mockQuery.AssertExpectations(t)
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestSale_InternalError_ReturnsMaskedError(t *testing.T) {
	handler, mockPayment, _ := createTestHandler(t)

	// Simulate an internal error (not a domain error)
	mockPayment.On("Sale", mock.Anything, mock.AnythingOfType("*ports.SaleRequest")).
		Return(nil, errors.New("database connection failed"))

	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:    uuid.New().String(),
		AmountCents:   1000,
		Currency:      "USD",
		PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: uuid.New().String()},
	})

	resp, err := handler.Sale(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	// Internal errors should NOT leak database details
	assert.NotContains(t, err.Error(), "database connection")
	mockPayment.AssertExpectations(t)
}

// =============================================================================
// Table-Driven Validation Tests
// =============================================================================

func TestSale_ValidationCases(t *testing.T) {
	testCases := []struct {
		name        string
		request     *paymentv1.SaleRequest
		expectError bool
		errorCode   connect.Code
	}{
		{
			name: "Valid request with payment method ID",
			request: &paymentv1.SaleRequest{
				MerchantId:    uuid.New().String(),
				AmountCents:   1000,
				Currency:      "USD",
				PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: uuid.New().String()},
			},
			expectError: false,
		},
		{
			name: "Valid request with payment token",
			request: &paymentv1.SaleRequest{
				MerchantId:    uuid.New().String(),
				AmountCents:   1000,
				Currency:      "USD",
				PaymentMethod: &paymentv1.SaleRequest_PaymentToken{PaymentToken: "tok_test123"},
			},
			expectError: false,
		},
		{
			name: "Missing merchant ID",
			request: &paymentv1.SaleRequest{
				AmountCents:   1000,
				Currency:      "USD",
				PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: uuid.New().String()},
			},
			expectError: true,
			errorCode:   connect.CodeInvalidArgument,
		},
		{
			name: "Zero amount",
			request: &paymentv1.SaleRequest{
				MerchantId:    uuid.New().String(),
				AmountCents:   0,
				Currency:      "USD",
				PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: uuid.New().String()},
			},
			expectError: true,
			errorCode:   connect.CodeInvalidArgument,
		},
		{
			name: "Missing currency",
			request: &paymentv1.SaleRequest{
				MerchantId:    uuid.New().String(),
				AmountCents:   1000,
				Currency:      "",
				PaymentMethod: &paymentv1.SaleRequest_PaymentMethodId{PaymentMethodId: uuid.New().String()},
			},
			expectError: true,
			errorCode:   connect.CodeInvalidArgument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, mockPayment, _ := createTestHandler(t)

			if !tc.expectError {
				expectedTx := createTestTransaction()
				mockPayment.On("Sale", mock.Anything, mock.AnythingOfType("*ports.SaleRequest")).
					Return(expectedTx, nil)
			}

			req := connect.NewRequest(tc.request)
			resp, err := handler.Sale(context.Background(), req)

			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, resp)
				assert.Equal(t, tc.errorCode, connect.CodeOf(err))
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}
