package merchant

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/handlers"
	"github.com/kevin07696/payment-service/internal/ports"
	merchantv1 "github.com/kevin07696/payment-service/proto/merchant/v1"
)

// ConnectHandler implements the Connect RPC MerchantServiceHandler interface
type ConnectHandler struct {
	service ports.MerchantService
	logger  *zap.Logger
}

// NewConnectHandler creates a new Connect RPC merchant handler
func NewConnectHandler(service ports.MerchantService, logger *zap.Logger) *ConnectHandler {
	return &ConnectHandler{
		service: service,
		logger:  logger,
	}
}

// RegisterMerchant adds a new merchant to the system
func (h *ConnectHandler) RegisterMerchant(
	ctx context.Context,
	req *connect.Request[merchantv1.RegisterMerchantRequest],
) (*connect.Response[merchantv1.MerchantResponse], error) {
	msg := req.Msg

	h.logger.Info("RegisterMerchant request received",
		zap.String("merchant_id", msg.MerchantId),
		zap.String("environment", msg.Environment.String()),
	)

	// Validate request
	if err := validateRegisterMerchantRequest(msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Convert to service request
	serviceReq := &ports.RegisterMerchantRequest{
		AgentID:      msg.MerchantId,
		MACSecret:    msg.MacSecret,
		CustNbr:      msg.CustNbr,
		MerchNbr:     msg.MerchNbr,
		DBAnbr:       msg.DbaNbr,
		TerminalNbr:  msg.TerminalNbr,
		Environment:  environmentFromProto(msg.Environment),
		MerchantName: msg.MerchantId, // Default to merchant_id if not provided
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	// Call service
	merchant, err := h.service.RegisterMerchant(ctx, serviceReq)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	// Convert to proto response and wrap in Connect response
	return connect.NewResponse(merchantToResponse(merchant)), nil
}

// GetMerchant retrieves merchant credentials (internal use only)
func (h *ConnectHandler) GetMerchant(
	ctx context.Context,
	req *connect.Request[merchantv1.GetMerchantRequest],
) (*connect.Response[merchantv1.Merchant], error) {
	msg := req.Msg

	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}

	merchant, err := h.service.GetMerchant(ctx, msg.MerchantId)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	return connect.NewResponse(merchantToProto(merchant)), nil
}

// ListMerchants lists all registered merchants
func (h *ConnectHandler) ListMerchants(
	ctx context.Context,
	req *connect.Request[merchantv1.ListMerchantsRequest],
) (*connect.Response[merchantv1.ListMerchantsResponse], error) {
	msg := req.Msg

	// Default pagination
	limit := int(msg.Limit)
	if limit <= 0 {
		limit = 100
	}
	offset := int(msg.Offset)

	var environment *domain.Environment
	if msg.Environment != nil {
		env := environmentFromProto(*msg.Environment)
		environment = &env
	}

	var isActive *bool
	if msg.IsActive != nil {
		isActive = msg.IsActive
	}

	merchants, totalCount, err := h.service.ListMerchants(ctx, environment, isActive, limit, offset)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	protoMerchants := make([]*merchantv1.MerchantSummary, len(merchants))
	for i, merchant := range merchants {
		protoMerchants[i] = merchantToSummary(merchant)
	}

	response := &merchantv1.ListMerchantsResponse{
		Merchants:  protoMerchants,
		TotalCount: int32(totalCount),
	}

	return connect.NewResponse(response), nil
}

// UpdateMerchant updates merchant credentials
func (h *ConnectHandler) UpdateMerchant(
	ctx context.Context,
	req *connect.Request[merchantv1.UpdateMerchantRequest],
) (*connect.Response[merchantv1.MerchantResponse], error) {
	msg := req.Msg

	h.logger.Info("UpdateMerchant request received",
		zap.String("merchant_id", msg.MerchantId),
	)

	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}

	serviceReq := &ports.UpdateMerchantRequest{
		AgentID: msg.MerchantId,
	}

	if msg.MacSecret != nil {
		serviceReq.MACSecret = msg.MacSecret
	}
	if msg.CustNbr != nil {
		serviceReq.CustNbr = msg.CustNbr
	}
	if msg.MerchNbr != nil {
		serviceReq.MerchNbr = msg.MerchNbr
	}
	if msg.DbaNbr != nil {
		serviceReq.DBAnbr = msg.DbaNbr
	}
	if msg.TerminalNbr != nil {
		serviceReq.TerminalNbr = msg.TerminalNbr
	}
	if msg.Environment != nil {
		env := environmentFromProto(*msg.Environment)
		serviceReq.Environment = &env
	}
	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	merchant, err := h.service.UpdateMerchant(ctx, serviceReq)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	return connect.NewResponse(merchantToResponse(merchant)), nil
}

// DeactivateMerchant deactivates a merchant
func (h *ConnectHandler) DeactivateMerchant(
	ctx context.Context,
	req *connect.Request[merchantv1.DeactivateMerchantRequest],
) (*connect.Response[merchantv1.MerchantResponse], error) {
	msg := req.Msg

	h.logger.Info("DeactivateMerchant request received",
		zap.String("merchant_id", msg.MerchantId),
		zap.String("reason", msg.Reason),
	)

	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}

	// Get merchant before deactivation to return in response
	merchant, err := h.service.GetMerchant(ctx, msg.MerchantId)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	err = h.service.DeactivateMerchant(ctx, msg.MerchantId, msg.Reason)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	// Mark as inactive in response
	merchant.IsActive = false

	return connect.NewResponse(merchantToResponse(merchant)), nil
}

// RotateMAC rotates MAC secret in secret manager
func (h *ConnectHandler) RotateMAC(
	ctx context.Context,
	req *connect.Request[merchantv1.RotateMACRequest],
) (*connect.Response[merchantv1.RotateMACResponse], error) {
	msg := req.Msg

	h.logger.Info("RotateMerchantMAC request received",
		zap.String("merchant_id", msg.MerchantId),
	)

	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}
	if msg.NewMacSecret == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("new_mac_secret is required"))
	}

	serviceReq := &ports.RotateMerchantMACRequest{
		AgentID:      msg.MerchantId,
		NewMACSecret: msg.NewMacSecret,
	}

	err := h.service.RotateMerchantMAC(ctx, serviceReq)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	// Get updated merchant to return mac_secret_path
	merchant, err := h.service.GetMerchant(ctx, msg.MerchantId)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	response := &merchantv1.RotateMACResponse{
		MerchantId:    merchant.AgentID,
		MacSecretPath: merchant.MACSecretPath,
		RotatedAt:     timestamppb.New(time.Now()),
	}

	return connect.NewResponse(response), nil
}

