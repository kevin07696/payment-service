package payment_method

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/adapters/database"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/kevin07696/payment-service/internal/testutil/mocks"
)

// MockTransactionManager implements database.TransactionManager for testing
type MockTransactionManager struct {
	mock.Mock
	querier *mocks.MockQuerier
}

var _ database.TransactionManager = (*MockTransactionManager)(nil)

func (m *MockTransactionManager) WithTx(ctx context.Context, fn func(sqlc.Querier) error) error {
	args := m.Called(ctx, fn)

	// Execute the transaction function with the mock querier
	if err := fn(m.querier); err != nil {
		return err
	}

	return args.Error(0)
}

// Test setup helper
func setupPaymentMethodService(t *testing.T) (*paymentMethodService, *mocks.MockQuerier, *MockTransactionManager) {
	mockQuerier := new(mocks.MockQuerier)
	mockTxManager := new(MockTransactionManager)
	mockTxManager.querier = mockQuerier
	logger := zap.NewNop()

	service := &paymentMethodService{
		queries:       mockQuerier,
		txManager:     mockTxManager,
		browserPost:   nil, // Not used in these unit tests
		serverPost:    nil, // Not used in these unit tests
		bricStorage:   nil, // Not used in these unit tests
		secretManager: nil, // Not used in these unit tests
		logger:        logger,
	}

	return service, mockQuerier, mockTxManager
}

// TestSavePaymentMethod_Success tests successful payment method save
func TestSavePaymentMethod_Success(t *testing.T) {
	service, mockQuerier, mockTxManager := setupPaymentMethodService(t)
	ctx := context.Background()

	merchantID := uuid.New()
	customerID := uuid.New()
	cardBrand := "visa"
	expMonth := 12
	expYear := 2025

	req := &ports.SavePaymentMethodRequest{
		MerchantID:   merchantID.String(),
		CustomerID:   customerID.String(),
		PaymentType:  domain.PaymentMethodTypeCreditCard,
		PaymentToken: "test-bric-token",
		LastFour:     "4242",
		CardBrand:    &cardBrand,
		CardExpMonth: &expMonth,
		CardExpYear:  &expYear,
		IsDefault:    false,
	}

	expectedPM := sqlc.CustomerPaymentMethod{
		ID:           uuid.New(),
		MerchantID:   merchantID,
		CustomerID:   customerID,
		PaymentType:  string(domain.PaymentMethodTypeCreditCard),
		Bric:         req.PaymentToken,
		LastFour:     req.LastFour,
		CardBrand:    pgtype.Text{String: cardBrand, Valid: true},
		CardExpMonth: pgtype.Int4{Int32: int32(expMonth), Valid: true},
		CardExpYear:  pgtype.Int4{Int32: int32(expYear), Valid: true},
		IsDefault:    pgtype.Bool{Bool: false, Valid: true},
		IsActive:     pgtype.Bool{Bool: true, Valid: true},
		IsVerified:   pgtype.Bool{Bool: true, Valid: true},
	}

	mockQuerier.On("CreatePaymentMethod", ctx, mock.AnythingOfType("sqlc.CreatePaymentMethodParams")).
		Return(expectedPM, nil)

	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.SavePaymentMethod(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, req.PaymentType, result.PaymentType)
	assert.Equal(t, req.LastFour, result.LastFour)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestSavePaymentMethod_ValidationErrors tests input validation
func TestSavePaymentMethod_ValidationErrors(t *testing.T) {
	service, _, _ := setupPaymentMethodService(t)
	ctx := context.Background()

	merchantID := uuid.New().String()
	customerID := uuid.New().String()

	testCases := []struct {
		name          string
		req           *ports.SavePaymentMethodRequest
		expectedError string
	}{
		{
			name: "Missing payment token",
			req: &ports.SavePaymentMethodRequest{
				MerchantID:   merchantID,
				CustomerID:   customerID,
				PaymentType:  domain.PaymentMethodTypeCreditCard,
				PaymentToken: "", // Missing
				LastFour:     "4242",
			},
			expectedError: "payment_token is required",
		},
		{
			name: "Invalid last four",
			req: &ports.SavePaymentMethodRequest{
				MerchantID:   merchantID,
				CustomerID:   customerID,
				PaymentType:  domain.PaymentMethodTypeCreditCard,
				PaymentToken: "test-token",
				LastFour:     "42", // Too short
			},
			expectedError: "last_four must be exactly 4 digits",
		},
		{
			name: "Credit card missing card details",
			req: &ports.SavePaymentMethodRequest{
				MerchantID:   merchantID,
				CustomerID:   customerID,
				PaymentType:  domain.PaymentMethodTypeCreditCard,
				PaymentToken: "test-token",
				LastFour:     "4242",
				// Missing CardBrand, CardExpMonth, CardExpYear
			},
			expectedError: "card details (brand, exp_month, exp_year) are required for credit cards",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := service.SavePaymentMethod(ctx, tc.req)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

// TestGetPaymentMethod_Success tests successful payment method retrieval
func TestGetPaymentMethod_Success(t *testing.T) {
	service, mockQuerier, _ := setupPaymentMethodService(t)
	ctx := context.Background()

	pmID := uuid.New()
	dbPM := sqlc.CustomerPaymentMethod{
		ID:          pmID,
		MerchantID:  uuid.New(),
		CustomerID:  uuid.New(),
		PaymentType: "credit_card",
		Bric:        "test-bric",
		LastFour:    "4242",
		IsActive:    pgtype.Bool{Bool: true, Valid: true},
	}

	mockQuerier.On("GetPaymentMethodByID", ctx, pmID).
		Return(dbPM, nil)

	result, err := service.GetPaymentMethod(ctx, pmID.String())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, pmID.String(), result.ID)
	assert.Equal(t, "4242", result.LastFour)

	mockQuerier.AssertExpectations(t)
}

// TestGetPaymentMethod_NotFound tests payment method not found
func TestGetPaymentMethod_NotFound(t *testing.T) {
	service, mockQuerier, _ := setupPaymentMethodService(t)
	ctx := context.Background()

	pmID := uuid.New()

	mockQuerier.On("GetPaymentMethodByID", ctx, pmID).
		Return(sqlc.CustomerPaymentMethod{}, fmt.Errorf("not found"))

	result, err := service.GetPaymentMethod(ctx, pmID.String())

	assert.Error(t, err)
	assert.Nil(t, result)

	mockQuerier.AssertExpectations(t)
}

// TestListPaymentMethods_Success tests successful listing
func TestListPaymentMethods_Success(t *testing.T) {
	service, mockQuerier, _ := setupPaymentMethodService(t)
	ctx := context.Background()

	merchantID := uuid.New()
	customerID := uuid.New()

	dbPMs := []sqlc.CustomerPaymentMethod{
		{
			ID:          uuid.New(),
			MerchantID:  merchantID,
			CustomerID:  customerID,
			PaymentType: "credit_card",
			LastFour:    "4242",
			IsActive:    pgtype.Bool{Bool: true, Valid: true},
		},
		{
			ID:          uuid.New(),
			MerchantID:  merchantID,
			CustomerID:  customerID,
			PaymentType: "ach",
			LastFour:    "1234",
			IsActive:    pgtype.Bool{Bool: true, Valid: true},
		},
	}

	mockQuerier.On("ListPaymentMethodsByCustomer", ctx, mock.MatchedBy(func(params sqlc.ListPaymentMethodsByCustomerParams) bool {
		return params.MerchantID == merchantID && params.CustomerID == customerID
	})).Return(dbPMs, nil)

	result, err := service.ListPaymentMethods(ctx, merchantID.String(), customerID.String())

	require.NoError(t, err)
	assert.Len(t, result, 2)

	mockQuerier.AssertExpectations(t)
}

// TestUpdatePaymentMethodStatus_Deactivate tests deactivation
func TestUpdatePaymentMethodStatus_Deactivate(t *testing.T) {
	service, mockQuerier, _ := setupPaymentMethodService(t)
	ctx := context.Background()

	pmID := uuid.New()
	merchantID := uuid.New()
	customerID := uuid.New()

	dbPM := sqlc.CustomerPaymentMethod{
		ID:          pmID,
		MerchantID:  merchantID,
		CustomerID:  customerID,
		PaymentType: "credit_card",
		IsActive:    pgtype.Bool{Bool: true, Valid: true},
	}

	deactivatedPM := dbPM
	deactivatedPM.IsActive = pgtype.Bool{Bool: false, Valid: true}

	mockQuerier.On("GetPaymentMethodByID", ctx, pmID).
		Return(dbPM, nil).Once()

	mockQuerier.On("DeactivatePaymentMethod", ctx, pmID).
		Return(nil)

	mockQuerier.On("GetPaymentMethodByID", ctx, pmID).
		Return(deactivatedPM, nil).Once()

	result, err := service.UpdatePaymentMethodStatus(ctx, pmID.String(), merchantID.String(), customerID.String(), false)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsActive)

	mockQuerier.AssertExpectations(t)
}

// TestDeletePaymentMethod_Success tests successful deletion
func TestDeletePaymentMethod_Success(t *testing.T) {
	service, mockQuerier, _ := setupPaymentMethodService(t)
	ctx := context.Background()

	pmID := uuid.New()

	mockQuerier.On("DeletePaymentMethod", ctx, pmID).
		Return(nil)

	err := service.DeletePaymentMethod(ctx, pmID.String())

	require.NoError(t, err)

	mockQuerier.AssertExpectations(t)
}

// TestSetDefaultPaymentMethod_Success tests setting default payment method
func TestSetDefaultPaymentMethod_Success(t *testing.T) {
	service, mockQuerier, mockTxManager := setupPaymentMethodService(t)
	ctx := context.Background()

	pmID := uuid.New()
	merchantID := uuid.New()
	customerID := uuid.New()

	dbPM := sqlc.CustomerPaymentMethod{
		ID:          pmID,
		MerchantID:  merchantID,
		CustomerID:  customerID,
		PaymentType: "credit_card",
		IsDefault:   pgtype.Bool{Bool: false, Valid: true},
		IsActive:    pgtype.Bool{Bool: true, Valid: true},
	}

	updatedPM := dbPM
	updatedPM.IsDefault = pgtype.Bool{Bool: true, Valid: true}

	// First call to verify payment method exists
	mockQuerier.On("GetPaymentMethodByID", ctx, pmID).
		Return(dbPM, nil).Once()

	// Inside transaction: unset all defaults
	mockQuerier.On("SetPaymentMethodAsDefault", ctx, mock.MatchedBy(func(params sqlc.SetPaymentMethodAsDefaultParams) bool {
		return params.MerchantID == merchantID && params.CustomerID == customerID
	})).Return(nil)

	// Inside transaction: mark this one as default
	mockQuerier.On("MarkPaymentMethodAsDefault", ctx, pmID).
		Return(nil)

	// Inside transaction: fetch updated payment method
	mockQuerier.On("GetPaymentMethodByID", ctx, pmID).
		Return(updatedPM, nil).Once()

	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.SetDefaultPaymentMethod(ctx, pmID.String(), merchantID.String(), customerID.String())

	require.NoError(t, err)
	assert.NotNil(t, result)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}
