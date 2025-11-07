package chargeback

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	chargebackv1 "github.com/kevin07696/payment-service/proto/chargeback/v1"
	"go.uber.org/zap"
)

// MockQueryExecutor is a mock implementation of QueryExecutor interface
type MockQueryExecutor struct {
	mock.Mock
}

func (m *MockQueryExecutor) GetChargebackByID(ctx context.Context, id uuid.UUID) (sqlc.Chargeback, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Chargeback), args.Error(1)
}

func (m *MockQueryExecutor) ListChargebacks(ctx context.Context, params sqlc.ListChargebacksParams) ([]sqlc.Chargeback, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]sqlc.Chargeback), args.Error(1)
}

func (m *MockQueryExecutor) CountChargebacks(ctx context.Context, params sqlc.CountChargebacksParams) (int64, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(int64), args.Error(1)
}

// GetChargeback Tests

func TestGetChargeback_Success(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	chargebackID := uuid.New()
	groupID := uuid.New()

	// Mock database response
	chargeback := sqlc.Chargeback{
		ID:                chargebackID,
		AgentID:           "test-agent-123",
		GroupID:           pgtype.UUID{Bytes: groupID, Valid: true},
		CustomerID:        pgtype.Text{String: "customer-456", Valid: true},
		CaseNumber:        "CASE-001",
		DisputeDate:       time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC),
		ChargebackDate:    time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
		Status:            "new",
		ReasonCode:        "10.4",
		ReasonDescription: pgtype.Text{String: "Fraudulent Transaction", Valid: true},
		ChargebackAmount:  "99.99",
		Currency:          "USD",
		EvidenceFiles:     []string{},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	mockQueries.On("GetChargebackByID", mock.Anything, chargebackID).Return(chargeback, nil)

	// Test request
	req := &chargebackv1.GetChargebackRequest{
		ChargebackId: chargebackID.String(),
		AgentId:      "test-agent-123",
	}

	// Execute
	resp, err := handler.GetChargeback(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, chargebackID.String(), resp.Id)
	assert.Equal(t, "test-agent-123", resp.AgentId)
	assert.Equal(t, groupID.String(), resp.GroupId)
	assert.Equal(t, "customer-456", resp.CustomerId)
	assert.Equal(t, "CASE-001", resp.CaseNumber)
	assert.Equal(t, chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_NEW, resp.Status)
	assert.Equal(t, "10.4", resp.ReasonCode)
	assert.Equal(t, "99.99", resp.ChargebackAmount)

	mockQueries.AssertExpectations(t)
}

func TestGetChargeback_MissingChargebackID(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	// Test request with missing chargeback_id
	req := &chargebackv1.GetChargebackRequest{
		ChargebackId: "",
		AgentId:      "test-agent-123",
	}

	// Execute
	resp, err := handler.GetChargeback(context.Background(), req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "chargeback_id is required")
}

func TestGetChargeback_MissingAgentID(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	// Test request with missing agent_id
	req := &chargebackv1.GetChargebackRequest{
		ChargebackId: uuid.New().String(),
		AgentId:      "",
	}

	// Execute
	resp, err := handler.GetChargeback(context.Background(), req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "agent_id is required")
}

func TestGetChargeback_InvalidUUID(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	// Test request with invalid UUID
	req := &chargebackv1.GetChargebackRequest{
		ChargebackId: "invalid-uuid",
		AgentId:      "test-agent-123",
	}

	// Execute
	resp, err := handler.GetChargeback(context.Background(), req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "invalid chargeback_id format")
}

func TestGetChargeback_NotFound(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	chargebackID := uuid.New()

	// Mock database error (not found)
	mockQueries.On("GetChargebackByID", mock.Anything, chargebackID).
		Return(sqlc.Chargeback{}, assert.AnError)

	// Test request
	req := &chargebackv1.GetChargebackRequest{
		ChargebackId: chargebackID.String(),
		AgentId:      "test-agent-123",
	}

	// Execute
	resp, err := handler.GetChargeback(context.Background(), req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "chargeback not found")

	mockQueries.AssertExpectations(t)
}

func TestGetChargeback_Unauthorized(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	chargebackID := uuid.New()

	// Mock database response with different agent_id
	chargeback := sqlc.Chargeback{
		ID:             chargebackID,
		AgentID:        "different-agent",
		CaseNumber:     "CASE-001",
		DisputeDate:    time.Now(),
		ChargebackDate: time.Now(),
		Status:         "new",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	mockQueries.On("GetChargebackByID", mock.Anything, chargebackID).Return(chargeback, nil)

	// Test request with mismatched agent_id
	req := &chargebackv1.GetChargebackRequest{
		ChargebackId: chargebackID.String(),
		AgentId:      "test-agent-123",
	}

	// Execute
	resp, err := handler.GetChargeback(context.Background(), req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "not authorized to access this chargeback")

	mockQueries.AssertExpectations(t)
}

// ListChargebacks Tests

func TestListChargebacks_Success(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	fromDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2025, 10, 29, 0, 0, 0, 0, time.UTC)

	// Mock database response
	chargebacks := []sqlc.Chargeback{
		{
			ID:                uuid.New(),
			AgentID:           "test-agent-123",
			CaseNumber:        "CASE-001",
			DisputeDate:       time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC),
			ChargebackDate:    time.Date(2025, 10, 25, 0, 0, 0, 0, time.UTC),
			Status:            "new",
			ReasonCode:        "10.4",
			ReasonDescription: pgtype.Text{String: "Fraudulent Transaction", Valid: true},
			ChargebackAmount:  "99.99",
			Currency:          "USD",
			EvidenceFiles:     []string{},
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		},
	}

	mockQueries.On("ListChargebacks", mock.Anything, mock.MatchedBy(func(params sqlc.ListChargebacksParams) bool {
		return params.AgentID.String == "test-agent-123" &&
			params.DisputeDateFrom.Valid && params.DisputeDateFrom.Time.Equal(fromDate) &&
			params.DisputeDateTo.Valid && params.DisputeDateTo.Time.Equal(toDate)
	})).Return(chargebacks, nil)

	mockQueries.On("CountChargebacks", mock.Anything, mock.Anything).Return(int64(1), nil)

	// Test request
	req := &chargebackv1.ListChargebacksRequest{
		AgentId:         "test-agent-123",
		DisputeDateFrom: timestamppb.New(fromDate),
		DisputeDateTo:   timestamppb.New(toDate),
	}

	// Execute
	resp, err := handler.ListChargebacks(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(1), resp.TotalCount)
	assert.Len(t, resp.Chargebacks, 1)

	chargeback := resp.Chargebacks[0]
	assert.Equal(t, "CASE-001", chargeback.CaseNumber)
	assert.Equal(t, chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_NEW, chargeback.Status)
	assert.Equal(t, "10.4", chargeback.ReasonCode)
	assert.Equal(t, "99.99", chargeback.ChargebackAmount)

	mockQueries.AssertExpectations(t)
}

func TestListChargebacks_WithGroupIDFilter(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	groupID := uuid.New()
	groupIDStr := groupID.String()

	// Mock database response
	mockQueries.On("ListChargebacks", mock.Anything, mock.MatchedBy(func(params sqlc.ListChargebacksParams) bool {
		return params.AgentID.String == "test-agent-123" &&
			params.GroupID.Valid &&
			params.GroupID.Bytes == groupID
	})).Return([]sqlc.Chargeback{}, nil)

	mockQueries.On("CountChargebacks", mock.Anything, mock.Anything).Return(int64(0), nil)

	// Test request with group_id filter
	req := &chargebackv1.ListChargebacksRequest{
		AgentId: "test-agent-123",
		GroupId: &groupIDStr,
	}

	// Execute
	resp, err := handler.ListChargebacks(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(0), resp.TotalCount)
	assert.Len(t, resp.Chargebacks, 0)

	mockQueries.AssertExpectations(t)
}

func TestListChargebacks_MissingAgentID(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	// Test request with missing agent_id
	req := &chargebackv1.ListChargebacksRequest{
		AgentId: "",
	}

	// Execute
	resp, err := handler.ListChargebacks(context.Background(), req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "agent_id is required")
}

func TestListChargebacks_DatabaseError(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	// Mock database error
	mockQueries.On("ListChargebacks", mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	// Test request
	req := &chargebackv1.ListChargebacksRequest{
		AgentId: "test-agent-123",
	}

	// Execute
	resp, err := handler.ListChargebacks(context.Background(), req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "failed to list chargebacks")

	mockQueries.AssertExpectations(t)
}

func TestListChargebacks_WithPagination(t *testing.T) {
	// Setup
	mockQueries := new(MockQueryExecutor)
	logger := zap.NewNop()
	handler := NewHandlerWithQueries(mockQueries, logger)

	// Mock response
	mockQueries.On("ListChargebacks", mock.Anything, mock.MatchedBy(func(params sqlc.ListChargebacksParams) bool {
		return params.AgentID.String == "test-agent-123" &&
			params.LimitVal == 10 &&
			params.OffsetVal == 20
	})).Return([]sqlc.Chargeback{}, nil)

	mockQueries.On("CountChargebacks", mock.Anything, mock.Anything).Return(int64(0), nil)

	// Test request with pagination
	req := &chargebackv1.ListChargebacksRequest{
		AgentId: "test-agent-123",
		Limit:   10,
		Offset:  20,
	}

	// Execute
	resp, err := handler.ListChargebacks(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(0), resp.TotalCount)
	assert.Len(t, resp.Chargebacks, 0)

	mockQueries.AssertExpectations(t)
}
