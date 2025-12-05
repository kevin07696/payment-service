package payment_method

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/handlers"
	"github.com/kevin07696/payment-service/internal/ports"
	paymentmethodv1 "github.com/kevin07696/payment-service/proto/payment_method/v1"
)

// ConnectHandler implements the Connect RPC PaymentMethodServiceHandler interface
type ConnectHandler struct {
	service ports.PaymentMethodService
	logger  *zap.Logger
}

// NewConnectHandler creates a new Connect RPC payment method handler
func NewConnectHandler(service ports.PaymentMethodService, logger *zap.Logger) *ConnectHandler {
	return &ConnectHandler{
		service: service,
		logger:  logger,
	}
}

// GetPaymentMethod retrieves a specific payment method
func (h *ConnectHandler) GetPaymentMethod(
	ctx context.Context,
	req *connect.Request[paymentmethodv1.GetPaymentMethodRequest],
) (*connect.Response[paymentmethodv1.PaymentMethod], error) {
	msg := req.Msg

	if msg.PaymentMethodId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payment_method_id is required"))
	}

	pm, err := h.service.GetPaymentMethod(ctx, msg.PaymentMethodId)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	return connect.NewResponse(paymentMethodToProto(pm)), nil
}

// ListPaymentMethods lists all payment methods for a customer
func (h *ConnectHandler) ListPaymentMethods(
	ctx context.Context,
	req *connect.Request[paymentmethodv1.ListPaymentMethodsRequest],
) (*connect.Response[paymentmethodv1.ListPaymentMethodsResponse], error) {
	msg := req.Msg

	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}
	if msg.CustomerId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("customer_id is required"))
	}

	pms, err := h.service.ListPaymentMethods(ctx, msg.MerchantId, msg.CustomerId)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	// Filter by payment type if provided
	if msg.PaymentType != nil {
		desiredType := paymentMethodTypeFromProto(*msg.PaymentType)
		filtered := make([]*domain.PaymentMethod, 0)
		for _, pm := range pms {
			if pm.PaymentType == desiredType {
				filtered = append(filtered, pm)
			}
		}
		pms = filtered
	}

	// Filter by is_active if provided
	if msg.IsActive != nil {
		filtered := make([]*domain.PaymentMethod, 0)
		for _, pm := range pms {
			if pm.IsActive() == *msg.IsActive {
				filtered = append(filtered, pm)
			}
		}
		pms = filtered
	}

	// Get total count before pagination
	totalCount := len(pms)

	// Apply pagination
	limit := int(msg.Limit)
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := int(msg.Offset)
	if offset < 0 {
		offset = 0
	}

	// Apply offset and limit
	if offset >= len(pms) {
		pms = []*domain.PaymentMethod{}
	} else {
		end := offset + limit
		if end > len(pms) {
			end = len(pms)
		}
		pms = pms[offset:end]
	}

	protoPMs := make([]*paymentmethodv1.PaymentMethod, len(pms))
	for i, pm := range pms {
		protoPMs[i] = paymentMethodToProto(pm)
	}

	response := &paymentmethodv1.ListPaymentMethodsResponse{
		PaymentMethods: protoPMs,
		TotalCount:     int32(totalCount),
	}

	return connect.NewResponse(response), nil
}

// UpdatePaymentMethodStatus updates the active status of a payment method
func (h *ConnectHandler) UpdatePaymentMethodStatus(
	ctx context.Context,
	req *connect.Request[paymentmethodv1.UpdatePaymentMethodStatusRequest],
) (*connect.Response[paymentmethodv1.PaymentMethodResponse], error) {
	msg := req.Msg

	h.logger.Info("UpdatePaymentMethodStatus request received",
		zap.String("payment_method_id", msg.PaymentMethodId),
		zap.String("customer_id", msg.CustomerId),
		zap.Bool("is_active", msg.IsActive),
	)

	if msg.PaymentMethodId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payment_method_id is required"))
	}
	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}
	if msg.CustomerId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("customer_id is required"))
	}

	pm, err := h.service.UpdatePaymentMethodStatus(ctx, msg.PaymentMethodId, msg.MerchantId, msg.CustomerId, msg.IsActive)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	return connect.NewResponse(paymentMethodToResponse(pm)), nil
}

// DeletePaymentMethod soft deletes a payment method (sets deleted_at)
func (h *ConnectHandler) DeletePaymentMethod(
	ctx context.Context,
	req *connect.Request[paymentmethodv1.DeletePaymentMethodRequest],
) (*connect.Response[paymentmethodv1.DeletePaymentMethodResponse], error) {
	msg := req.Msg

	h.logger.Info("DeletePaymentMethod request received",
		zap.String("payment_method_id", msg.PaymentMethodId),
	)

	if msg.PaymentMethodId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payment_method_id is required"))
	}

	err := h.service.DeletePaymentMethod(ctx, msg.PaymentMethodId)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	response := &paymentmethodv1.DeletePaymentMethodResponse{
		Success: true,
		Message: "Payment method soft deleted successfully (90-day retention)",
	}

	return connect.NewResponse(response), nil
}

// SetDefaultPaymentMethod marks a payment method as default
func (h *ConnectHandler) SetDefaultPaymentMethod(
	ctx context.Context,
	req *connect.Request[paymentmethodv1.SetDefaultPaymentMethodRequest],
) (*connect.Response[paymentmethodv1.PaymentMethodResponse], error) {
	msg := req.Msg

	h.logger.Info("SetDefaultPaymentMethod request received",
		zap.String("payment_method_id", msg.PaymentMethodId),
		zap.String("customer_id", msg.CustomerId),
	)

	if msg.PaymentMethodId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payment_method_id is required"))
	}
	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}
	if msg.CustomerId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("customer_id is required"))
	}

	pm, err := h.service.SetDefaultPaymentMethod(ctx, msg.PaymentMethodId, msg.MerchantId, msg.CustomerId)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	return connect.NewResponse(paymentMethodToResponse(pm)), nil
}


// UpdatePaymentMethod updates payment method metadata (billing info, nickname)
//
// NOTE: Not yet implemented - requires schema migration to add billing fields:
//   - billing_name, billing_address, billing_city, billing_state, billing_zip
//   - nickname (optional user-provided label)
//
// Current schema only stores: last_four, card_brand, card_exp_*, bank_name, account_type
//
// Implementation plan:
//  1. Add migration to add billing_* columns to customer_payment_methods table
//  2. Update domain.PaymentMethod to include billing fields
//  3. Update sqlc queries to support UPDATE of billing fields
//  4. Implement this handler to call payment method service
//
// Use case: Allow customers to update billing address without re-tokenizing card
func (h *ConnectHandler) UpdatePaymentMethod(
	ctx context.Context,
	req *connect.Request[paymentmethodv1.UpdatePaymentMethodRequest],
) (*connect.Response[paymentmethodv1.PaymentMethodResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("UpdatePaymentMethod requires billing fields schema migration"))
}

