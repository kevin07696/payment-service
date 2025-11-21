package admin

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/testutil/mocks"
	adminv1 "github.com/kevin07696/payment-service/proto/admin/v1"
)

// =============================================================================
// Test Setup
// =============================================================================

func setupServiceHandler(t *testing.T) (*ServiceHandler, *mocks.MockQuerier) {
	mockQuerier := new(mocks.MockQuerier)
	handler := &ServiceHandler{
		queries: mockQuerier,
	}
	return handler, mockQuerier
}

// =============================================================================
// CreateService Tests
// =============================================================================

// TestCreateService_Success tests successful service creation with auto-generated keypair
func TestCreateService_Success(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.CreateServiceRequest{
		ServiceId:         "test-service",
		ServiceName:       "Test Service",
		Environment:       "sandbox",
		RequestsPerSecond: 50,
		BurstLimit:        100,
	})

	// Mock database service creation
	serviceID := uuid.New()
	createdService := sqlc.Service{
		ID:                   serviceID,
		ServiceID:            "test-service",
		ServiceName:          "Test Service",
		PublicKey:            "-----BEGIN PUBLIC KEY-----\nMIIB...\n-----END PUBLIC KEY-----\n",
		PublicKeyFingerprint: "abc123def456",
		Environment:          "sandbox",
		RequestsPerSecond:    pgtype.Int4{Int32: 50, Valid: true},
		BurstLimit:           pgtype.Int4{Int32: 100, Valid: true},
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
		CreatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
		UpdatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
	}

	mockQuerier.On("CreateService", ctx, mock.MatchedBy(func(params sqlc.CreateServiceParams) bool {
		return params.ServiceID == "test-service" &&
			params.ServiceName == "Test Service" &&
			params.Environment == "sandbox" &&
			params.RequestsPerSecond.Int32 == 50 &&
			params.BurstLimit.Int32 == 100 &&
			params.IsActive.Bool == true
	})).Return(createdService, nil)

	// Mock audit log creation
	mockQuerier.On("CreateAuditLog", ctx, mock.AnythingOfType("sqlc.CreateAuditLogParams")).Return(nil)

	// Execute
	resp, err := handler.CreateService(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
	assert.Equal(t, "test-service", resp.Msg.Service.ServiceId)
	assert.Equal(t, "Test Service", resp.Msg.Service.ServiceName)
	assert.NotEmpty(t, resp.Msg.PrivateKey, "Private key should be returned")
	assert.Contains(t, resp.Msg.PrivateKey, "BEGIN RSA PRIVATE KEY")
	assert.Contains(t, resp.Msg.Message, "SAVE THIS PRIVATE KEY")

	mockQuerier.AssertExpectations(t)
}

// TestCreateService_DefaultRateLimits tests default values when not provided
func TestCreateService_DefaultRateLimits(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.CreateServiceRequest{
		ServiceId:   "test-service",
		ServiceName: "Test Service",
		Environment: "sandbox",
		// RequestsPerSecond and BurstLimit not provided
	})

	serviceID := uuid.New()
	createdService := sqlc.Service{
		ID:                   serviceID,
		ServiceID:            "test-service",
		ServiceName:          "Test Service",
		PublicKey:            "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----\n",
		PublicKeyFingerprint: "fingerprint",
		Environment:          "sandbox",
		RequestsPerSecond:    pgtype.Int4{Int32: 100, Valid: true},
		BurstLimit:           pgtype.Int4{Int32: 200, Valid: true},
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
		CreatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
		UpdatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
	}

	mockQuerier.On("CreateService", ctx, mock.MatchedBy(func(params sqlc.CreateServiceParams) bool {
		// Verify defaults are applied
		return params.RequestsPerSecond.Int32 == 100 && params.BurstLimit.Int32 == 200
	})).Return(createdService, nil)

	// Mock audit log creation
	mockQuerier.On("CreateAuditLog", ctx, mock.AnythingOfType("sqlc.CreateAuditLogParams")).Return(nil)

	// Execute
	resp, err := handler.CreateService(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int32(100), resp.Msg.Service.RequestsPerSecond)
	assert.Equal(t, int32(200), resp.Msg.Service.BurstLimit)

	mockQuerier.AssertExpectations(t)
}

// TestCreateService_ValidationErrors tests validation failures
func TestCreateService_ValidationErrors(t *testing.T) {
	handler, _ := setupServiceHandler(t)
	ctx := context.Background()

	tests := []struct {
		name          string
		request       *adminv1.CreateServiceRequest
		expectedError string
		expectedCode  connect.Code
	}{
		{
			name: "missing service_id",
			request: &adminv1.CreateServiceRequest{
				ServiceId:   "", // Missing
				ServiceName: "Test",
				Environment: "sandbox",
			},
			expectedError: "service_id is required",
			expectedCode:  connect.CodeInvalidArgument,
		},
		{
			name: "missing service_name",
			request: &adminv1.CreateServiceRequest{
				ServiceId:   "test",
				ServiceName: "", // Missing
				Environment: "sandbox",
			},
			expectedError: "service_name is required",
			expectedCode:  connect.CodeInvalidArgument,
		},
		{
			name: "missing environment",
			request: &adminv1.CreateServiceRequest{
				ServiceId:   "test",
				ServiceName: "Test",
				Environment: "", // Missing
			},
			expectedError: "environment is required",
			expectedCode:  connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(tt.request)

			resp, err := handler.CreateService(ctx, req)

			assert.Error(t, err)
			assert.Nil(t, resp)
			assert.Contains(t, err.Error(), tt.expectedError)
			assert.Equal(t, tt.expectedCode, connect.CodeOf(err))
		})
	}
}

// TestCreateService_DatabaseError tests database failure
func TestCreateService_DatabaseError(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.CreateServiceRequest{
		ServiceId:   "test-service",
		ServiceName: "Test Service",
		Environment: "sandbox",
	})

	// Mock database error
	mockQuerier.On("CreateService", ctx, mock.AnythingOfType("sqlc.CreateServiceParams")).
		Return(sqlc.Service{}, fmt.Errorf("database error"))

	// Execute
	resp, err := handler.CreateService(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create service")
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))

	mockQuerier.AssertExpectations(t)
}

// =============================================================================
// RotateServiceKey Tests
// =============================================================================

// TestRotateServiceKey_Success tests successful key rotation
func TestRotateServiceKey_Success(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.RotateServiceKeyRequest{
		ServiceId: "test-service",
	})

	serviceID := uuid.New()
	oldService := sqlc.Service{
		ID:                   serviceID,
		ServiceID:            "test-service",
		ServiceName:          "Test Service",
		PublicKey:            "old-public-key",
		PublicKeyFingerprint: "old-fingerprint",
		Environment:          "sandbox",
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
	}

	rotatedService := sqlc.Service{
		ID:                   serviceID,
		ServiceID:            "test-service",
		ServiceName:          "Test Service",
		PublicKey:            "new-public-key",
		PublicKeyFingerprint: "new-fingerprint",
		Environment:          "sandbox",
		RequestsPerSecond:    pgtype.Int4{Int32: 100, Valid: true},
		BurstLimit:           pgtype.Int4{Int32: 200, Valid: true},
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
		CreatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
		UpdatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
	}

	mockQuerier.On("GetServiceByServiceID", ctx, "test-service").
		Return(oldService, nil)

	mockQuerier.On("RotateServiceKey", ctx, mock.MatchedBy(func(params sqlc.RotateServiceKeyParams) bool {
		return params.ID == serviceID &&
			params.PublicKey != "" &&
			params.PublicKeyFingerprint != ""
	})).Return(rotatedService, nil)

	// Mock audit log creation
	mockQuerier.On("CreateAuditLog", ctx, mock.AnythingOfType("sqlc.CreateAuditLogParams")).Return(nil)

	// Execute
	resp, err := handler.RotateServiceKey(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
	assert.Equal(t, "test-service", resp.Msg.Service.ServiceId)
	assert.NotEmpty(t, resp.Msg.PrivateKey)
	assert.Contains(t, resp.Msg.PrivateKey, "BEGIN RSA PRIVATE KEY")
	assert.Contains(t, resp.Msg.Message, "KEY ROTATED")

	mockQuerier.AssertExpectations(t)
}

// TestRotateServiceKey_MissingServiceID tests validation
func TestRotateServiceKey_MissingServiceID(t *testing.T) {
	handler, _ := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.RotateServiceKeyRequest{
		ServiceId: "", // Missing
	})

	resp, err := handler.RotateServiceKey(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "service_id is required")
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestRotateServiceKey_ServiceNotFound tests service not found error
func TestRotateServiceKey_ServiceNotFound(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.RotateServiceKeyRequest{
		ServiceId: "nonexistent",
	})

	mockQuerier.On("GetServiceByServiceID", ctx, "nonexistent").
		Return(sqlc.Service{}, fmt.Errorf("not found"))

	resp, err := handler.RotateServiceKey(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "service not found")
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	mockQuerier.AssertExpectations(t)
}

// TestRotateServiceKey_DatabaseError tests database update failure
func TestRotateServiceKey_DatabaseError(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.RotateServiceKeyRequest{
		ServiceId: "test-service",
	})

	oldService := sqlc.Service{
		ID:        uuid.New(),
		ServiceID: "test-service",
	}

	mockQuerier.On("GetServiceByServiceID", ctx, "test-service").
		Return(oldService, nil)

	mockQuerier.On("RotateServiceKey", ctx, mock.AnythingOfType("sqlc.RotateServiceKeyParams")).
		Return(sqlc.Service{}, fmt.Errorf("database error"))

	resp, err := handler.RotateServiceKey(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to rotate key")
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))

	mockQuerier.AssertExpectations(t)
}

// =============================================================================
// GetService Tests
// =============================================================================

// TestGetService_Success tests successful service retrieval
func TestGetService_Success(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.GetServiceRequest{
		ServiceId: "test-service",
	})

	service := sqlc.Service{
		ID:                   uuid.New(),
		ServiceID:            "test-service",
		ServiceName:          "Test Service",
		PublicKeyFingerprint: "fingerprint123",
		Environment:          "production",
		RequestsPerSecond:    pgtype.Int4{Int32: 100, Valid: true},
		BurstLimit:           pgtype.Int4{Int32: 200, Valid: true},
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
		CreatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
		UpdatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
	}

	mockQuerier.On("GetServiceByServiceID", ctx, "test-service").
		Return(service, nil)

	resp, err := handler.GetService(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
	assert.Equal(t, "test-service", resp.Msg.Service.ServiceId)
	assert.Equal(t, "Test Service", resp.Msg.Service.ServiceName)
	assert.Equal(t, "production", resp.Msg.Service.Environment)

	mockQuerier.AssertExpectations(t)
}

// TestGetService_MissingServiceID tests validation
func TestGetService_MissingServiceID(t *testing.T) {
	handler, _ := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.GetServiceRequest{
		ServiceId: "", // Missing
	})

	resp, err := handler.GetService(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "service_id is required")
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestGetService_NotFound tests service not found error
func TestGetService_NotFound(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.GetServiceRequest{
		ServiceId: "nonexistent",
	})

	mockQuerier.On("GetServiceByServiceID", ctx, "nonexistent").
		Return(sqlc.Service{}, fmt.Errorf("not found"))

	resp, err := handler.GetService(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "service not found")
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	mockQuerier.AssertExpectations(t)
}

// =============================================================================
// ListServices Tests
// =============================================================================

// TestListServices_Success tests successful service listing
func TestListServices_Success(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.ListServicesRequest{
		Limit:  10,
		Offset: 0,
	})

	services := []sqlc.Service{
		{
			ID:                   uuid.New(),
			ServiceID:            "service-1",
			ServiceName:          "Service 1",
			PublicKeyFingerprint: "fp1",
			Environment:          "sandbox",
			RequestsPerSecond:    pgtype.Int4{Int32: 100, Valid: true},
			BurstLimit:           pgtype.Int4{Int32: 200, Valid: true},
			IsActive:             pgtype.Bool{Bool: true, Valid: true},
			CreatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
			UpdatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
		},
		{
			ID:                   uuid.New(),
			ServiceID:            "service-2",
			ServiceName:          "Service 2",
			PublicKeyFingerprint: "fp2",
			Environment:          "production",
			RequestsPerSecond:    pgtype.Int4{Int32: 50, Valid: true},
			BurstLimit:           pgtype.Int4{Int32: 100, Valid: true},
			IsActive:             pgtype.Bool{Bool: true, Valid: true},
			CreatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
			UpdatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
		},
	}

	mockQuerier.On("ListServices", ctx, mock.MatchedBy(func(params sqlc.ListServicesParams) bool {
		return params.LimitVal == 10 && params.OffsetVal == 0
	})).Return(services, nil)

	resp, err := handler.ListServices(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
	assert.Len(t, resp.Msg.Services, 2)
	assert.Equal(t, "service-1", resp.Msg.Services[0].ServiceId)
	assert.Equal(t, "service-2", resp.Msg.Services[1].ServiceId)

	mockQuerier.AssertExpectations(t)
}

// TestListServices_WithFilters tests filtering by environment and status
func TestListServices_WithFilters(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	env := "production"
	active := true
	req := connect.NewRequest(&adminv1.ListServicesRequest{
		Environment: &env,
		IsActive:    &active,
		Limit:       10,
	})

	mockQuerier.On("ListServices", ctx, mock.MatchedBy(func(params sqlc.ListServicesParams) bool {
		return params.Environment.Valid &&
			params.Environment.String == "production" &&
			params.IsActive.Valid &&
			params.IsActive.Bool == true
	})).Return([]sqlc.Service{}, nil)

	resp, err := handler.ListServices(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)

	mockQuerier.AssertExpectations(t)
}

// TestListServices_DefaultLimit tests default limit is applied
func TestListServices_DefaultLimit(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.ListServicesRequest{
		// Limit not provided
	})

	mockQuerier.On("ListServices", ctx, mock.MatchedBy(func(params sqlc.ListServicesParams) bool {
		return params.LimitVal == 50 // Default limit
	})).Return([]sqlc.Service{}, nil)

	resp, err := handler.ListServices(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)

	mockQuerier.AssertExpectations(t)
}

// TestListServices_MaxLimitEnforced tests max limit of 100 is enforced
func TestListServices_MaxLimitEnforced(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.ListServicesRequest{
		Limit: 500, // Exceeds max
	})

	mockQuerier.On("ListServices", ctx, mock.MatchedBy(func(params sqlc.ListServicesParams) bool {
		return params.LimitVal == 100 // Max limit enforced
	})).Return([]sqlc.Service{}, nil)

	resp, err := handler.ListServices(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)

	mockQuerier.AssertExpectations(t)
}

// =============================================================================
// DeactivateService Tests
// =============================================================================

// TestDeactivateService_Success tests successful service deactivation
func TestDeactivateService_Success(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.DeactivateServiceRequest{
		ServiceId: "test-service",
	})

	serviceID := uuid.New()
	service := sqlc.Service{
		ID:        serviceID,
		ServiceID: "test-service",
		IsActive:  pgtype.Bool{Bool: true, Valid: true},
	}

	deactivatedService := service
	deactivatedService.IsActive = pgtype.Bool{Bool: false, Valid: true}
	deactivatedService.CreatedAt = pgtype.Timestamp{Time: time.Now(), Valid: true}
	deactivatedService.UpdatedAt = pgtype.Timestamp{Time: time.Now(), Valid: true}
	deactivatedService.RequestsPerSecond = pgtype.Int4{Int32: 100, Valid: true}
	deactivatedService.BurstLimit = pgtype.Int4{Int32: 200, Valid: true}

	mockQuerier.On("GetServiceByServiceID", ctx, "test-service").
		Return(service, nil).Once()

	mockQuerier.On("DeactivateService", ctx, serviceID).
		Return(nil)

	// Mock audit log creation
	mockQuerier.On("CreateAuditLog", ctx, mock.AnythingOfType("sqlc.CreateAuditLogParams")).Return(nil)

	mockQuerier.On("GetServiceByServiceID", ctx, "test-service").
		Return(deactivatedService, nil).Once()

	resp, err := handler.DeactivateService(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
	assert.False(t, resp.Msg.Service.IsActive)

	mockQuerier.AssertExpectations(t)
}

// TestDeactivateService_MissingServiceID tests validation
func TestDeactivateService_MissingServiceID(t *testing.T) {
	handler, _ := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.DeactivateServiceRequest{
		ServiceId: "", // Missing
	})

	resp, err := handler.DeactivateService(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "service_id is required")
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestDeactivateService_NotFound tests service not found error
func TestDeactivateService_NotFound(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.DeactivateServiceRequest{
		ServiceId: "nonexistent",
	})

	mockQuerier.On("GetServiceByServiceID", ctx, "nonexistent").
		Return(sqlc.Service{}, fmt.Errorf("not found"))

	resp, err := handler.DeactivateService(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "service not found")
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	mockQuerier.AssertExpectations(t)
}

// =============================================================================
// ActivateService Tests
// =============================================================================

// TestActivateService_Success tests successful service activation
func TestActivateService_Success(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.ActivateServiceRequest{
		ServiceId: "test-service",
	})

	serviceID := uuid.New()
	service := sqlc.Service{
		ID:        serviceID,
		ServiceID: "test-service",
		IsActive:  pgtype.Bool{Bool: false, Valid: true},
	}

	activatedService := service
	activatedService.IsActive = pgtype.Bool{Bool: true, Valid: true}
	activatedService.CreatedAt = pgtype.Timestamp{Time: time.Now(), Valid: true}
	activatedService.UpdatedAt = pgtype.Timestamp{Time: time.Now(), Valid: true}
	activatedService.RequestsPerSecond = pgtype.Int4{Int32: 100, Valid: true}
	activatedService.BurstLimit = pgtype.Int4{Int32: 200, Valid: true}

	mockQuerier.On("GetServiceByServiceID", ctx, "test-service").
		Return(service, nil).Once()

	mockQuerier.On("ActivateService", ctx, serviceID).
		Return(nil)

	mockQuerier.On("GetServiceByServiceID", ctx, "test-service").
		Return(activatedService, nil).Once()

	resp, err := handler.ActivateService(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
	assert.True(t, resp.Msg.Service.IsActive)

	mockQuerier.AssertExpectations(t)
}

// TestActivateService_MissingServiceID tests validation
func TestActivateService_MissingServiceID(t *testing.T) {
	handler, _ := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.ActivateServiceRequest{
		ServiceId: "", // Missing
	})

	resp, err := handler.ActivateService(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "service_id is required")
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestActivateService_NotFound tests service not found error
func TestActivateService_NotFound(t *testing.T) {
	handler, mockQuerier := setupServiceHandler(t)
	ctx := context.Background()

	req := connect.NewRequest(&adminv1.ActivateServiceRequest{
		ServiceId: "nonexistent",
	})

	mockQuerier.On("GetServiceByServiceID", ctx, "nonexistent").
		Return(sqlc.Service{}, fmt.Errorf("not found"))

	resp, err := handler.ActivateService(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "service not found")
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	mockQuerier.AssertExpectations(t)
}
