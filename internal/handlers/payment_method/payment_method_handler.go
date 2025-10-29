package payment_method

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	paymentmethodv1 "github.com/kevin07696/payment-service/proto/payment_method/v1"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"go.uber.org/zap"
)

// Handler implements the gRPC PaymentMethodServiceServer
type Handler struct {
	paymentmethodv1.UnimplementedPaymentMethodServiceServer
	service ports.PaymentMethodService
	logger  *zap.Logger
}

// NewHandler creates a new payment method handler
func NewHandler(service ports.PaymentMethodService, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// SavePaymentMethod tokenizes and saves a payment method
func (h *Handler) SavePaymentMethod(ctx context.Context, req *paymentmethodv1.SavePaymentMethodRequest) (*paymentmethodv1.PaymentMethodResponse, error) {
	h.logger.Info("SavePaymentMethod request received",
		zap.String("agent_id", req.AgentId),
		zap.String("customer_id", req.CustomerId),
		zap.String("payment_type", req.PaymentType.String()),
	)

	// Validate request
	if err := validateSavePaymentMethodRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Convert to service request
	serviceReq := &ports.SavePaymentMethodRequest{
		AgentID:      req.AgentId,
		CustomerID:   req.CustomerId,
		PaymentToken: req.PaymentToken,
		PaymentType:  paymentMethodTypeFromProto(req.PaymentType),
		LastFour:     req.LastFour,
		IsDefault:    req.IsDefault,
	}

	// Credit card fields
	if req.CardBrand != nil {
		serviceReq.CardBrand = req.CardBrand
	}
	if req.CardExpMonth != nil {
		month := int(*req.CardExpMonth)
		serviceReq.CardExpMonth = &month
	}
	if req.CardExpYear != nil {
		year := int(*req.CardExpYear)
		serviceReq.CardExpYear = &year
	}

	// ACH fields
	if req.BankName != nil {
		serviceReq.BankName = req.BankName
	}
	if req.AccountType != nil {
		serviceReq.AccountType = req.AccountType
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	// Call service
	pm, err := h.service.SavePaymentMethod(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Convert to proto response
	return paymentMethodToResponse(pm), nil
}

// GetPaymentMethod retrieves a specific payment method
func (h *Handler) GetPaymentMethod(ctx context.Context, req *paymentmethodv1.GetPaymentMethodRequest) (*paymentmethodv1.PaymentMethod, error) {
	if req.PaymentMethodId == "" {
		return nil, status.Error(codes.InvalidArgument, "payment_method_id is required")
	}

	pm, err := h.service.GetPaymentMethod(ctx, req.PaymentMethodId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, domain.ErrPaymentMethodNotFound) {
			return nil, status.Error(codes.NotFound, "payment method not found")
		}
		return nil, status.Error(codes.Internal, "failed to get payment method")
	}

	return paymentMethodToProto(pm), nil
}

// ListPaymentMethods lists all payment methods for a customer
func (h *Handler) ListPaymentMethods(ctx context.Context, req *paymentmethodv1.ListPaymentMethodsRequest) (*paymentmethodv1.ListPaymentMethodsResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	pms, err := h.service.ListPaymentMethods(ctx, req.AgentId, req.CustomerId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list payment methods")
	}

	// Filter by payment type if provided
	if req.PaymentType != nil {
		desiredType := paymentMethodTypeFromProto(*req.PaymentType)
		filtered := make([]*domain.PaymentMethod, 0)
		for _, pm := range pms {
			if pm.PaymentType == desiredType {
				filtered = append(filtered, pm)
			}
		}
		pms = filtered
	}

	// Filter by is_active if provided
	if req.IsActive != nil {
		filtered := make([]*domain.PaymentMethod, 0)
		for _, pm := range pms {
			if pm.IsActive == *req.IsActive {
				filtered = append(filtered, pm)
			}
		}
		pms = filtered
	}

	protoPMs := make([]*paymentmethodv1.PaymentMethod, len(pms))
	for i, pm := range pms {
		protoPMs[i] = paymentMethodToProto(pm)
	}

	return &paymentmethodv1.ListPaymentMethodsResponse{
		PaymentMethods: protoPMs,
	}, nil
}

// UpdatePaymentMethodStatus updates the active status of a payment method
func (h *Handler) UpdatePaymentMethodStatus(ctx context.Context, req *paymentmethodv1.UpdatePaymentMethodStatusRequest) (*paymentmethodv1.PaymentMethodResponse, error) {
	h.logger.Info("UpdatePaymentMethodStatus request received",
		zap.String("payment_method_id", req.PaymentMethodId),
		zap.String("customer_id", req.CustomerId),
		zap.Bool("is_active", req.IsActive),
	)

	if req.PaymentMethodId == "" {
		return nil, status.Error(codes.InvalidArgument, "payment_method_id is required")
	}
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	pm, err := h.service.UpdatePaymentMethodStatus(ctx, req.PaymentMethodId, req.AgentId, req.CustomerId, req.IsActive)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return paymentMethodToResponse(pm), nil
}

// DeletePaymentMethod soft deletes a payment method (sets deleted_at)
func (h *Handler) DeletePaymentMethod(ctx context.Context, req *paymentmethodv1.DeletePaymentMethodRequest) (*paymentmethodv1.DeletePaymentMethodResponse, error) {
	h.logger.Info("DeletePaymentMethod request received",
		zap.String("payment_method_id", req.PaymentMethodId),
	)

	if req.PaymentMethodId == "" {
		return nil, status.Error(codes.InvalidArgument, "payment_method_id is required")
	}

	err := h.service.DeletePaymentMethod(ctx, req.PaymentMethodId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, domain.ErrPaymentMethodNotFound) {
			return nil, status.Error(codes.NotFound, "payment method not found")
		}
		return &paymentmethodv1.DeletePaymentMethodResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &paymentmethodv1.DeletePaymentMethodResponse{
		Success: true,
		Message: "Payment method soft deleted successfully (90-day retention)",
	}, nil
}

// SetDefaultPaymentMethod marks a payment method as default
func (h *Handler) SetDefaultPaymentMethod(ctx context.Context, req *paymentmethodv1.SetDefaultPaymentMethodRequest) (*paymentmethodv1.PaymentMethodResponse, error) {
	h.logger.Info("SetDefaultPaymentMethod request received",
		zap.String("payment_method_id", req.PaymentMethodId),
		zap.String("customer_id", req.CustomerId),
	)

	if req.PaymentMethodId == "" {
		return nil, status.Error(codes.InvalidArgument, "payment_method_id is required")
	}
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	pm, err := h.service.SetDefaultPaymentMethod(ctx, req.PaymentMethodId, req.AgentId, req.CustomerId)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return paymentMethodToResponse(pm), nil
}

// VerifyACHAccount sends pre-note for ACH verification
func (h *Handler) VerifyACHAccount(ctx context.Context, req *paymentmethodv1.VerifyACHAccountRequest) (*paymentmethodv1.VerifyACHAccountResponse, error) {
	h.logger.Info("VerifyACHAccount request received",
		zap.String("payment_method_id", req.PaymentMethodId),
		zap.String("customer_id", req.CustomerId),
	)

	if req.PaymentMethodId == "" {
		return nil, status.Error(codes.InvalidArgument, "payment_method_id is required")
	}
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	serviceReq := &ports.VerifyACHAccountRequest{
		PaymentMethodID: req.PaymentMethodId,
		AgentID:         req.AgentId,
		CustomerID:      req.CustomerId,
	}

	err := h.service.VerifyACHAccount(ctx, serviceReq)
	if err != nil {
		return &paymentmethodv1.VerifyACHAccountResponse{
			PaymentMethodId: req.PaymentMethodId,
			Status:          "failed",
			Message:         err.Error(),
		}, nil
	}

	return &paymentmethodv1.VerifyACHAccountResponse{
		PaymentMethodId: req.PaymentMethodId,
		Status:          "verified",
		Message:         "ACH account verified successfully",
	}, nil
}

// Validation helpers

func validateSavePaymentMethodRequest(req *paymentmethodv1.SavePaymentMethodRequest) error {
	if req.AgentId == "" {
		return fmt.Errorf("agent_id is required")
	}
	if req.CustomerId == "" {
		return fmt.Errorf("customer_id is required")
	}
	if req.PaymentToken == "" {
		return fmt.Errorf("payment_token is required")
	}
	if req.PaymentType == paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED {
		return fmt.Errorf("payment_type is required")
	}
	if len(req.LastFour) != 4 {
		return fmt.Errorf("last_four must be exactly 4 digits")
	}

	// Validate credit card specific fields
	if req.PaymentType == paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD {
		if req.CardBrand == nil || *req.CardBrand == "" {
			return fmt.Errorf("card_brand is required for credit cards")
		}
		if req.CardExpMonth == nil {
			return fmt.Errorf("card_exp_month is required for credit cards")
		}
		if req.CardExpYear == nil {
			return fmt.Errorf("card_exp_year is required for credit cards")
		}
		if *req.CardExpMonth < 1 || *req.CardExpMonth > 12 {
			return fmt.Errorf("card_exp_month must be between 1 and 12")
		}
	}

	// Validate ACH specific fields
	if req.PaymentType == paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_ACH {
		if req.BankName == nil || *req.BankName == "" {
			return fmt.Errorf("bank_name is required for ACH")
		}
		if req.AccountType == nil || *req.AccountType == "" {
			return fmt.Errorf("account_type is required for ACH")
		}
	}

	return nil
}

// Conversion helpers

func paymentMethodToResponse(pm *domain.PaymentMethod) *paymentmethodv1.PaymentMethodResponse {
	resp := &paymentmethodv1.PaymentMethodResponse{
		PaymentMethodId: pm.ID,
		AgentId:         pm.AgentID,
		CustomerId:      pm.CustomerID,
		PaymentType:     paymentMethodTypeToProto(pm.PaymentType),
		LastFour:        pm.LastFour,
		IsDefault:       pm.IsDefault,
		IsActive:        pm.IsActive,
		IsVerified:      pm.IsVerified,
		CreatedAt:       timestamppb.New(pm.CreatedAt),
	}

	if pm.CardBrand != nil {
		resp.CardBrand = pm.CardBrand
	}
	if pm.CardExpMonth != nil {
		month := int32(*pm.CardExpMonth)
		resp.CardExpMonth = &month
	}
	if pm.CardExpYear != nil {
		year := int32(*pm.CardExpYear)
		resp.CardExpYear = &year
	}
	if pm.BankName != nil {
		resp.BankName = pm.BankName
	}
	if pm.AccountType != nil {
		resp.AccountType = pm.AccountType
	}
	if pm.LastUsedAt != nil {
		resp.LastUsedAt = timestamppb.New(*pm.LastUsedAt)
	}

	return resp
}

func paymentMethodToProto(pm *domain.PaymentMethod) *paymentmethodv1.PaymentMethod {
	proto := &paymentmethodv1.PaymentMethod{
		Id:          pm.ID,
		AgentId:     pm.AgentID,
		CustomerId:  pm.CustomerID,
		PaymentType: paymentMethodTypeToProto(pm.PaymentType),
		LastFour:    pm.LastFour,
		IsDefault:   pm.IsDefault,
		IsActive:    pm.IsActive,
		IsVerified:  pm.IsVerified,
		CreatedAt:   timestamppb.New(pm.CreatedAt),
		UpdatedAt:   timestamppb.New(pm.UpdatedAt),
	}

	if pm.CardBrand != nil {
		proto.CardBrand = pm.CardBrand
	}
	if pm.CardExpMonth != nil {
		month := int32(*pm.CardExpMonth)
		proto.CardExpMonth = &month
	}
	if pm.CardExpYear != nil {
		year := int32(*pm.CardExpYear)
		proto.CardExpYear = &year
	}
	if pm.BankName != nil {
		proto.BankName = pm.BankName
	}
	if pm.AccountType != nil {
		proto.AccountType = pm.AccountType
	}
	if pm.LastUsedAt != nil {
		proto.LastUsedAt = timestamppb.New(*pm.LastUsedAt)
	}

	return proto
}

func paymentMethodTypeToProto(pmType domain.PaymentMethodType) paymentmethodv1.PaymentMethodType {
	switch pmType {
	case domain.PaymentMethodTypeCreditCard:
		return paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD
	case domain.PaymentMethodTypeACH:
		return paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_ACH
	default:
		return paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED
	}
}

func paymentMethodTypeFromProto(pmType paymentmethodv1.PaymentMethodType) domain.PaymentMethodType {
	switch pmType {
	case paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD:
		return domain.PaymentMethodTypeCreditCard
	case paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_ACH:
		return domain.PaymentMethodTypeACH
	default:
		return domain.PaymentMethodTypeCreditCard // Default
	}
}

// Error handling

func handleServiceError(err error) error {
	// Map domain errors to gRPC status codes
	switch {
	case errors.Is(err, domain.ErrPaymentMethodNotFound):
		return status.Error(codes.NotFound, "payment method not found")
	case errors.Is(err, domain.ErrPaymentMethodExpired):
		return status.Error(codes.FailedPrecondition, "payment method is expired")
	case errors.Is(err, domain.ErrPaymentMethodNotVerified):
		return status.Error(codes.FailedPrecondition, "ACH payment method is not verified")
	case errors.Is(err, domain.ErrPaymentMethodInactive):
		return status.Error(codes.FailedPrecondition, "payment method is inactive")
	case errors.Is(err, domain.ErrInvalidPaymentMethodType):
		return status.Error(codes.InvalidArgument, "invalid payment method type")
	case errors.Is(err, domain.ErrAgentInactive):
		return status.Error(codes.FailedPrecondition, "agent is inactive")
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
