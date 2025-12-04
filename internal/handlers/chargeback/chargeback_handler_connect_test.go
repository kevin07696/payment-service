package chargeback

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	chargebackv1 "github.com/kevin07696/payment-service/proto/chargeback/v1"
)

// MockQueryExecutor is a mock implementation of QueryExecutor
type MockQueryExecutor struct {
	mock.Mock
}

func (m *MockQueryExecutor) GetChargebackByID(ctx context.Context, id uuid.UUID) (sqlc.Chargeback, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Chargeback), args.Error(1)
}

func (m *MockQueryExecutor) ListChargebacks(ctx context.Context, params sqlc.ListChargebacksParams) ([]sqlc.Chargeback, error) {
	args := m.Called(ctx, params)
	return args.Get(0).([]sqlc.Chargeback), args.Error(1)
}

func (m *MockQueryExecutor) CountChargebacks(ctx context.Context, params sqlc.CountChargebacksParams) (int64, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(int64), args.Error(1)
}

// TestGetChargeback_Validation tests input validation for GetChargeback
func TestGetChargeback_Validation(t *testing.T) {
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := newConnectHandlerWithQueries(mockQueries, logger)

	tests := []struct {
		name          string
		chargebackID  string
		merchantID    string
		expectedCode  connect.Code
		expectedError string
	}{
		{
			name:          "Missing chargeback_id",
			chargebackID:  "",
			merchantID:    "test-merchant",
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "chargeback_id is required",
		},
		{
			name:          "Missing merchant_id",
			chargebackID:  uuid.New().String(),
			merchantID:    "",
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "merchant_id is required",
		},
		{
			name:          "Invalid chargeback_id format",
			chargebackID:  "not-a-uuid",
			merchantID:    "test-merchant",
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "invalid chargeback_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(&chargebackv1.GetChargebackRequest{
				ChargebackId: tt.chargebackID,
				MerchantId:   tt.merchantID,
			})

			_, err := handler.GetChargeback(context.Background(), req)

			require.Error(t, err)
			var connectErr *connect.Error
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, tt.expectedCode, connectErr.Code())
			assert.Contains(t, connectErr.Message(), tt.expectedError)
		})
	}

	// Verify no database calls were made for validation errors
	mockQueries.AssertExpectations(t)
}

// TestListChargebacks_Validation tests input validation for ListChargebacks
func TestListChargebacks_Validation(t *testing.T) {
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := newConnectHandlerWithQueries(mockQueries, logger)

	tests := []struct {
		name          string
		merchantID    string
		expectedCode  connect.Code
		expectedError string
	}{
		{
			name:          "Missing merchant_id",
			merchantID:   "",
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "merchant_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(&chargebackv1.ListChargebacksRequest{
				MerchantId: tt.merchantID,
				Limit:      10,
				Offset:     0,
			})

			_, err := handler.ListChargebacks(context.Background(), req)

			require.Error(t, err)
			var connectErr *connect.Error
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, tt.expectedCode, connectErr.Code())
			assert.Contains(t, connectErr.Message(), tt.expectedError)
		})
	}

	// Verify no database calls were made for validation errors
	mockQueries.AssertExpectations(t)
}

// TestGetChargeback_NotFound tests NotFound error handling
func TestGetChargeback_NotFound(t *testing.T) {
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := newConnectHandlerWithQueries(mockQueries, logger)

	chargebackID := uuid.New()
	mockQueries.On("GetChargebackByID", mock.Anything, chargebackID).
		Return(sqlc.Chargeback{}, errors.New("not found"))

	req := connect.NewRequest(&chargebackv1.GetChargebackRequest{
		ChargebackId: chargebackID.String(),
		MerchantId:   "test-merchant",
	})

	_, err := handler.GetChargeback(context.Background(), req)

	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())

	mockQueries.AssertExpectations(t)
}

// TestListChargebacks_LimitDefaults tests default limit handling
func TestListChargebacks_LimitDefaults(t *testing.T) {
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := newConnectHandlerWithQueries(mockQueries, logger)

	tests := []struct {
		name          string
		inputLimit    int32
		expectedLimit int32
	}{
		{
			name:          "Zero limit defaults to 100",
			inputLimit:    0,
			expectedLimit: 100,
		},
		{
			name:          "Limit over 1000 capped at 1000",
			inputLimit:    2000,
			expectedLimit: 1000,
		},
		{
			name:          "Valid limit unchanged",
			inputLimit:    50,
			expectedLimit: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueries.On("ListChargebacks", mock.Anything, mock.MatchedBy(func(params sqlc.ListChargebacksParams) bool {
				return params.LimitVal == tt.expectedLimit
			})).Return([]sqlc.Chargeback{}, nil).Once()

			mockQueries.On("CountChargebacks", mock.Anything, mock.Anything).
				Return(int64(0), nil).Once()

			req := connect.NewRequest(&chargebackv1.ListChargebacksRequest{
				MerchantId: "test-merchant",
				Limit:      tt.inputLimit,
				Offset:     0,
			})

			_, err := handler.ListChargebacks(context.Background(), req)
			require.NoError(t, err)
		})
	}

	mockQueries.AssertExpectations(t)
}
