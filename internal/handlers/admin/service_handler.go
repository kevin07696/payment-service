package admin

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	adminv1 "github.com/kevin07696/payment-service/proto/admin/v1"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/pkg/crypto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ServiceHandler handles admin service management operations.
type ServiceHandler struct {
	queries *sqlc.Queries
}

// NewServiceHandler creates a new service handler.
func NewServiceHandler(queries *sqlc.Queries) *ServiceHandler {
	return &ServiceHandler{
		queries: queries,
	}
}

// CreateService creates a new service with auto-generated RSA keypair.
func (h *ServiceHandler) CreateService(
	ctx context.Context,
	req *connect.Request[adminv1.CreateServiceRequest],
) (*connect.Response[adminv1.CreateServiceResponse], error) {
	// Validate request
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}
	if req.Msg.ServiceName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_name is required"))
	}
	if req.Msg.Environment == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("environment is required"))
	}

	// Auto-generate RSA keypair
	keypair, err := crypto.GenerateRSAKeyPair()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate keypair: %w", err))
	}

	// Set default rate limits if not provided
	requestsPerSecond := req.Msg.RequestsPerSecond
	if requestsPerSecond == 0 {
		requestsPerSecond = 100
	}
	burstLimit := req.Msg.BurstLimit
	if burstLimit == 0 {
		burstLimit = 200
	}

	// Create service in database (store only public key)
	service, err := h.queries.CreateService(ctx, sqlc.CreateServiceParams{
		ID:                   uuid.New(),
		ServiceID:            req.Msg.ServiceId,
		ServiceName:          req.Msg.ServiceName,
		PublicKey:            keypair.PublicKeyPEM,
		PublicKeyFingerprint: keypair.Fingerprint,
		Environment:          req.Msg.Environment,
		RequestsPerSecond: pgtype.Int4{
			Int32: requestsPerSecond,
			Valid: true,
		},
		BurstLimit: pgtype.Int4{
			Int32: burstLimit,
			Valid: true,
		},
		IsActive: pgtype.Bool{Bool: true, Valid: true},
		// TODO: Get admin ID from auth context
		CreatedBy: pgtype.UUID{Valid: false},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create service: %w", err))
	}

	// TODO: Audit log the service creation

	// Return service + private key (ONE-TIME ONLY)
	return connect.NewResponse(&adminv1.CreateServiceResponse{
		Service: &adminv1.Service{
			Id:                   service.ID.String(),
			ServiceId:            service.ServiceID,
			ServiceName:          service.ServiceName,
			PublicKeyFingerprint: service.PublicKeyFingerprint,
			Environment:          service.Environment,
			RequestsPerSecond:    service.RequestsPerSecond.Int32,
			BurstLimit:           service.BurstLimit.Int32,
			IsActive:             service.IsActive.Bool,
			CreatedAt:            timestamppb.New(service.CreatedAt.Time),
			UpdatedAt:            timestamppb.New(service.UpdatedAt.Time),
		},
		PrivateKey: keypair.PrivateKeyPEM,
		Message:    "⚠️  SAVE THIS PRIVATE KEY - IT WILL NOT BE SHOWN AGAIN!",
	}), nil
}

// RotateServiceKey generates a new RSA keypair for an existing service.
func (h *ServiceHandler) RotateServiceKey(
	ctx context.Context,
	req *connect.Request[adminv1.RotateServiceKeyRequest],
) (*connect.Response[adminv1.RotateServiceKeyResponse], error) {
	// Validate request
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}

	// Generate new keypair
	keypair, err := crypto.GenerateRSAKeyPair()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate keypair: %w", err))
	}

	// Get service UUID from service_id
	oldService, err := h.queries.GetServiceByServiceID(ctx, req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("service not found: %w", err))
	}

	// Update service with new public key
	service, err := h.queries.RotateServiceKey(ctx, sqlc.RotateServiceKeyParams{
		ID:                   oldService.ID,
		PublicKey:            keypair.PublicKeyPEM,
		PublicKeyFingerprint: keypair.Fingerprint,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to rotate key: %w", err))
	}

	// TODO: Audit log the key rotation with reason

	return connect.NewResponse(&adminv1.RotateServiceKeyResponse{
		Service: &adminv1.Service{
			Id:                   service.ID.String(),
			ServiceId:            service.ServiceID,
			ServiceName:          service.ServiceName,
			PublicKeyFingerprint: service.PublicKeyFingerprint,
			Environment:          service.Environment,
			RequestsPerSecond:    service.RequestsPerSecond.Int32,
			BurstLimit:           service.BurstLimit.Int32,
			IsActive:             service.IsActive.Bool,
			CreatedAt:            timestamppb.New(service.CreatedAt.Time),
			UpdatedAt:            timestamppb.New(service.UpdatedAt.Time),
		},
		PrivateKey: keypair.PrivateKeyPEM,
		Message:    "⚠️  KEY ROTATED - SAVE NEW PRIVATE KEY AND UPDATE SERVICE CONFIG!",
	}), nil
}

// GetService retrieves service details by ID.
func (h *ServiceHandler) GetService(
	ctx context.Context,
	req *connect.Request[adminv1.GetServiceRequest],
) (*connect.Response[adminv1.GetServiceResponse], error) {
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}

	service, err := h.queries.GetServiceByServiceID(ctx, req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("service not found: %w", err))
	}

	return connect.NewResponse(&adminv1.GetServiceResponse{
		Service: &adminv1.Service{
			Id:                   service.ID.String(),
			ServiceId:            service.ServiceID,
			ServiceName:          service.ServiceName,
			PublicKeyFingerprint: service.PublicKeyFingerprint,
			Environment:          service.Environment,
			RequestsPerSecond:    service.RequestsPerSecond.Int32,
			BurstLimit:           service.BurstLimit.Int32,
			IsActive:             service.IsActive.Bool,
			CreatedAt:            timestamppb.New(service.CreatedAt.Time),
			UpdatedAt:            timestamppb.New(service.UpdatedAt.Time),
		},
	}), nil
}

// ListServices lists all services with optional filtering.
func (h *ServiceHandler) ListServices(
	ctx context.Context,
	req *connect.Request[adminv1.ListServicesRequest],
) (*connect.Response[adminv1.ListServicesResponse], error) {
	// Set default limit
	limit := req.Msg.Limit
	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	// Build query parameters
	var environment pgtype.Text
	if req.Msg.Environment != nil {
		environment = pgtype.Text{String: *req.Msg.Environment, Valid: true}
	}

	var isActive pgtype.Bool
	if req.Msg.IsActive != nil {
		isActive = pgtype.Bool{Bool: *req.Msg.IsActive, Valid: true}
	}

	services, err := h.queries.ListServices(ctx, sqlc.ListServicesParams{
		Environment: environment,
		IsActive:    isActive,
		LimitVal:    limit,
		OffsetVal:   req.Msg.Offset,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list services: %w", err))
	}

	// Convert to proto messages
	protoServices := make([]*adminv1.Service, len(services))
	for i, svc := range services {
		protoServices[i] = &adminv1.Service{
			Id:                   svc.ID.String(),
			ServiceId:            svc.ServiceID,
			ServiceName:          svc.ServiceName,
			PublicKeyFingerprint: svc.PublicKeyFingerprint,
			Environment:          svc.Environment,
			RequestsPerSecond:    svc.RequestsPerSecond.Int32,
			BurstLimit:           svc.BurstLimit.Int32,
			IsActive:             svc.IsActive.Bool,
			CreatedAt:            timestamppb.New(svc.CreatedAt.Time),
			UpdatedAt:            timestamppb.New(svc.UpdatedAt.Time),
		}
	}

	return connect.NewResponse(&adminv1.ListServicesResponse{
		Services: protoServices,
		Total:    int64(len(services)), // TODO: Get actual count from DB
	}), nil
}

// DeactivateService deactivates a service.
func (h *ServiceHandler) DeactivateService(
	ctx context.Context,
	req *connect.Request[adminv1.DeactivateServiceRequest],
) (*connect.Response[adminv1.DeactivateServiceResponse], error) {
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}

	// Get service UUID
	service, err := h.queries.GetServiceByServiceID(ctx, req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("service not found: %w", err))
	}

	// Deactivate service
	if err := h.queries.DeactivateService(ctx, service.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to deactivate service: %w", err))
	}

	// TODO: Audit log deactivation with reason

	// Refetch to get updated service
	updatedService, err := h.queries.GetServiceByServiceID(ctx, req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch updated service: %w", err))
	}

	return connect.NewResponse(&adminv1.DeactivateServiceResponse{
		Service: &adminv1.Service{
			Id:                   updatedService.ID.String(),
			ServiceId:            updatedService.ServiceID,
			ServiceName:          updatedService.ServiceName,
			PublicKeyFingerprint: updatedService.PublicKeyFingerprint,
			Environment:          updatedService.Environment,
			RequestsPerSecond:    updatedService.RequestsPerSecond.Int32,
			BurstLimit:           updatedService.BurstLimit.Int32,
			IsActive:             updatedService.IsActive.Bool,
			CreatedAt:            timestamppb.New(updatedService.CreatedAt.Time),
			UpdatedAt:            timestamppb.New(updatedService.UpdatedAt.Time),
		},
	}), nil
}

// ActivateService reactivates a service.
func (h *ServiceHandler) ActivateService(
	ctx context.Context,
	req *connect.Request[adminv1.ActivateServiceRequest],
) (*connect.Response[adminv1.ActivateServiceResponse], error) {
	if req.Msg.ServiceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
	}

	// Get service UUID
	service, err := h.queries.GetServiceByServiceID(ctx, req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("service not found: %w", err))
	}

	// Activate service
	if err := h.queries.ActivateService(ctx, service.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to activate service: %w", err))
	}

	// Refetch to get updated service
	updatedService, err := h.queries.GetServiceByServiceID(ctx, req.Msg.ServiceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to fetch updated service: %w", err))
	}

	return connect.NewResponse(&adminv1.ActivateServiceResponse{
		Service: &adminv1.Service{
			Id:                   updatedService.ID.String(),
			ServiceId:            updatedService.ServiceID,
			ServiceName:          updatedService.ServiceName,
			PublicKeyFingerprint: updatedService.PublicKeyFingerprint,
			Environment:          updatedService.Environment,
			RequestsPerSecond:    updatedService.RequestsPerSecond.Int32,
			BurstLimit:           updatedService.BurstLimit.Int32,
			IsActive:             updatedService.IsActive.Bool,
			CreatedAt:            timestamppb.New(updatedService.CreatedAt.Time),
			UpdatedAt:            timestamppb.New(updatedService.UpdatedAt.Time),
		},
	}), nil
}
