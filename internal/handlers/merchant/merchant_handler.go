package merchant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	merchantv1 "github.com/kevin07696/payment-service/proto/merchant/v1"
	"go.uber.org/zap"
)

// Handler implements the gRPC MerchantServiceServer
type Handler struct {
	merchantv1.UnimplementedMerchantServiceServer
	service ports.MerchantService
	logger  *zap.Logger
}

// NewHandler creates a new merchant handler
func NewHandler(service ports.MerchantService, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterMerchant adds a new merchant to the system
func (h *Handler) RegisterMerchant(ctx context.Context, req *merchantv1.RegisterMerchantRequest) (*merchantv1.MerchantResponse, error) {
	h.logger.Info("RegisterMerchant request received",
		zap.String("merchant_id", req.MerchantId),
		zap.String("environment", req.Environment.String()),
	)

	// Validate request
	if err := validateRegisterMerchantRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Convert to service request
	serviceReq := &ports.RegisterMerchantRequest{
		AgentID:      req.MerchantId,
		MACSecret:    req.MacSecret,
		CustNbr:      req.CustNbr,
		MerchNbr:     req.MerchNbr,
		DBAnbr:       req.DbaNbr,
		TerminalNbr:  req.TerminalNbr,
		Environment:  environmentFromProto(req.Environment),
		MerchantName: req.MerchantId, // Default to merchant_id if not provided
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	// Call service
	merchant, err := h.service.RegisterMerchant(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Convert to proto response
	return merchantToResponse(merchant), nil
}

// GetMerchant retrieves merchant credentials (internal use only)
func (h *Handler) GetMerchant(ctx context.Context, req *merchantv1.GetMerchantRequest) (*merchantv1.Merchant, error) {
	if req.MerchantId == "" {
		return nil, status.Error(codes.InvalidArgument, "merchant_id is required")
	}

	merchant, err := h.service.GetMerchant(ctx, req.MerchantId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, domain.ErrMerchantNotFound) {
			return nil, status.Error(codes.NotFound, "merchant not found")
		}
		return nil, status.Error(codes.Internal, "failed to get merchant")
	}

	return merchantToProto(merchant), nil
}

// ListMerchants lists all registered merchants
func (h *Handler) ListMerchants(ctx context.Context, req *merchantv1.ListMerchantsRequest) (*merchantv1.ListMerchantsResponse, error) {
	// Default pagination
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}
	offset := int(req.Offset)

	var environment *domain.Environment
	if req.Environment != nil {
		env := environmentFromProto(*req.Environment)
		environment = &env
	}

	var isActive *bool
	if req.IsActive != nil {
		isActive = req.IsActive
	}

	merchants, totalCount, err := h.service.ListMerchants(ctx, environment, isActive, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list merchants")
	}

	protoMerchants := make([]*merchantv1.MerchantSummary, len(merchants))
	for i, merchant := range merchants {
		protoMerchants[i] = merchantToSummary(merchant)
	}

	return &merchantv1.ListMerchantsResponse{
		Merchants:  protoMerchants,
		TotalCount: int32(totalCount),
	}, nil
}

// UpdateMerchant updates merchant credentials
func (h *Handler) UpdateMerchant(ctx context.Context, req *merchantv1.UpdateMerchantRequest) (*merchantv1.MerchantResponse, error) {
	h.logger.Info("UpdateMerchant request received",
		zap.String("merchant_id", req.MerchantId),
	)

	if req.MerchantId == "" {
		return nil, status.Error(codes.InvalidArgument, "merchant_id is required")
	}

	serviceReq := &ports.UpdateMerchantRequest{
		AgentID: req.MerchantId,
	}

	if req.MacSecret != nil {
		serviceReq.MACSecret = req.MacSecret
	}
	if req.CustNbr != nil {
		serviceReq.CustNbr = req.CustNbr
	}
	if req.MerchNbr != nil {
		serviceReq.MerchNbr = req.MerchNbr
	}
	if req.DbaNbr != nil {
		serviceReq.DBAnbr = req.DbaNbr
	}
	if req.TerminalNbr != nil {
		serviceReq.TerminalNbr = req.TerminalNbr
	}
	if req.Environment != nil {
		env := environmentFromProto(*req.Environment)
		serviceReq.Environment = &env
	}
	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	merchant, err := h.service.UpdateMerchant(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return merchantToResponse(merchant), nil
}

// DeactivateMerchant deactivates a merchant
func (h *Handler) DeactivateMerchant(ctx context.Context, req *merchantv1.DeactivateMerchantRequest) (*merchantv1.MerchantResponse, error) {
	h.logger.Info("DeactivateMerchant request received",
		zap.String("merchant_id", req.MerchantId),
		zap.String("reason", req.Reason),
	)

	if req.MerchantId == "" {
		return nil, status.Error(codes.InvalidArgument, "merchant_id is required")
	}

	// Get merchant before deactivation to return in response
	merchant, err := h.service.GetMerchant(ctx, req.MerchantId)
	if err != nil {
		return nil, handleServiceError(err)
	}

	err = h.service.DeactivateMerchant(ctx, req.MerchantId, req.Reason)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Mark as inactive in response
	merchant.IsActive = false

	return merchantToResponse(merchant), nil
}

// RotateMAC rotates MAC secret in secret manager
func (h *Handler) RotateMAC(ctx context.Context, req *merchantv1.RotateMACRequest) (*merchantv1.RotateMACResponse, error) {
	h.logger.Info("RotateMerchantMAC request received",
		zap.String("merchant_id", req.MerchantId),
	)

	if req.MerchantId == "" {
		return nil, status.Error(codes.InvalidArgument, "merchant_id is required")
	}
	if req.NewMacSecret == "" {
		return nil, status.Error(codes.InvalidArgument, "new_mac_secret is required")
	}

	serviceReq := &ports.RotateMerchantMACRequest{
		AgentID:      req.MerchantId,
		NewMACSecret: req.NewMacSecret,
	}

	err := h.service.RotateMerchantMAC(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Get updated merchant to return mac_secret_path
	merchant, err := h.service.GetMerchant(ctx, req.MerchantId)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return &merchantv1.RotateMACResponse{
		MerchantId:    merchant.AgentID,
		MacSecretPath: merchant.MACSecretPath,
		RotatedAt:     timestamppb.New(time.Now()),
	}, nil
}

// Validation helpers

func validateRegisterMerchantRequest(req *merchantv1.RegisterMerchantRequest) error {
	if req.MerchantId == "" {
		return fmt.Errorf("merchant_id is required")
	}
	if req.MacSecret == "" {
		return fmt.Errorf("mac_secret is required")
	}
	if req.CustNbr == "" {
		return fmt.Errorf("cust_nbr is required")
	}
	if req.MerchNbr == "" {
		return fmt.Errorf("merch_nbr is required")
	}
	if req.DbaNbr == "" {
		return fmt.Errorf("dba_nbr is required")
	}
	if req.TerminalNbr == "" {
		return fmt.Errorf("terminal_nbr is required")
	}
	if req.Environment == merchantv1.Environment_ENVIRONMENT_UNSPECIFIED {
		return fmt.Errorf("environment is required")
	}
	return nil
}

// Conversion helpers

func merchantToResponse(merchant *domain.Merchant) *merchantv1.MerchantResponse {
	return &merchantv1.MerchantResponse{
		MerchantId:    merchant.AgentID,
		MacSecretPath: merchant.MACSecretPath,
		CustNbr:       merchant.CustNbr,
		MerchNbr:      merchant.MerchNbr,
		DbaNbr:        merchant.DBAnbr,
		TerminalNbr:   merchant.TerminalNbr,
		Environment:   environmentToProto(merchant.Environment),
		IsActive:      merchant.IsActive,
		CreatedAt:     timestamppb.New(merchant.CreatedAt),
		UpdatedAt:     timestamppb.New(merchant.UpdatedAt),
	}
}

func merchantToProto(merchant *domain.Merchant) *merchantv1.Merchant {
	return &merchantv1.Merchant{
		Id:            merchant.ID,
		MerchantId:    merchant.AgentID,
		MacSecretPath: merchant.MACSecretPath,
		CustNbr:       merchant.CustNbr,
		MerchNbr:      merchant.MerchNbr,
		DbaNbr:        merchant.DBAnbr,
		TerminalNbr:   merchant.TerminalNbr,
		Environment:   environmentToProto(merchant.Environment),
		IsActive:      merchant.IsActive,
		CreatedAt:     timestamppb.New(merchant.CreatedAt),
		UpdatedAt:     timestamppb.New(merchant.UpdatedAt),
		Metadata:      nil, // Not storing metadata yet
	}
}

func merchantToSummary(merchant *domain.Merchant) *merchantv1.MerchantSummary {
	return &merchantv1.MerchantSummary{
		MerchantId:  merchant.AgentID,
		MerchNbr:    merchant.MerchNbr,
		Environment: environmentToProto(merchant.Environment),
		IsActive:    merchant.IsActive,
		CreatedAt:   timestamppb.New(merchant.CreatedAt),
	}
}

func environmentToProto(env domain.Environment) merchantv1.Environment {
	switch env {
	case domain.EnvironmentSandbox:
		return merchantv1.Environment_ENVIRONMENT_SANDBOX
	case domain.EnvironmentProduction:
		return merchantv1.Environment_ENVIRONMENT_PRODUCTION
	default:
		return merchantv1.Environment_ENVIRONMENT_UNSPECIFIED
	}
}

func environmentFromProto(env merchantv1.Environment) domain.Environment {
	switch env {
	case merchantv1.Environment_ENVIRONMENT_SANDBOX:
		return domain.EnvironmentSandbox
	case merchantv1.Environment_ENVIRONMENT_PRODUCTION:
		return domain.EnvironmentProduction
	default:
		return domain.EnvironmentSandbox // Default
	}
}

// Error handling

func handleServiceError(err error) error {
	// Map domain errors to gRPC status codes
	switch {
	case errors.Is(err, domain.ErrMerchantNotFound):
		return status.Error(codes.NotFound, "merchant not found")
	case errors.Is(err, domain.ErrMerchantInactive):
		return status.Error(codes.FailedPrecondition, "merchant is inactive")
	case errors.Is(err, domain.ErrMerchantAlreadyExists):
		return status.Error(codes.AlreadyExists, "merchant already exists")
	case errors.Is(err, domain.ErrInvalidEnvironment):
		return status.Error(codes.InvalidArgument, "invalid environment")
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey):
		return status.Error(codes.AlreadyExists, "duplicate idempotency key")
	case errors.Is(err, sql.ErrNoRows):
		return status.Error(codes.NotFound, "resource not found")
	case err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)):
		return status.Error(codes.Canceled, "request canceled")
	default:
		// Log internal errors but don't expose details to client
		return status.Error(codes.Internal, "internal server error")
	}
}
