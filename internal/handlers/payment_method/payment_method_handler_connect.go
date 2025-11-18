package payment_method

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
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

// SavePaymentMethod tokenizes and saves a payment method
func (h *ConnectHandler) SavePaymentMethod(
	ctx context.Context,
	req *connect.Request[paymentmethodv1.SavePaymentMethodRequest],
) (*connect.Response[paymentmethodv1.PaymentMethodResponse], error) {
	msg := req.Msg

	h.logger.Info("SavePaymentMethod request received",
		zap.String("merchant_id", msg.MerchantId),
		zap.String("customer_id", msg.CustomerId),
		zap.String("payment_type", msg.PaymentType.String()),
	)

	// Validate request
	if err := validateSavePaymentMethodRequest(msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Convert to service request
	serviceReq := &ports.SavePaymentMethodRequest{
		MerchantID:   msg.MerchantId,
		CustomerID:   msg.CustomerId,
		PaymentToken: msg.PaymentToken,
		PaymentType:  paymentMethodTypeFromProto(msg.PaymentType),
		LastFour:     msg.LastFour,
		IsDefault:    msg.IsDefault,
	}

	// Credit card fields
	if msg.CardBrand != nil {
		serviceReq.CardBrand = msg.CardBrand
	}
	if msg.CardExpMonth != nil {
		month := int(*msg.CardExpMonth)
		serviceReq.CardExpMonth = &month
	}
	if msg.CardExpYear != nil {
		year := int(*msg.CardExpYear)
		serviceReq.CardExpYear = &year
	}

	// ACH fields
	if msg.BankName != nil {
		serviceReq.BankName = msg.BankName
	}
	if msg.AccountType != nil {
		serviceReq.AccountType = msg.AccountType
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	// Call service
	pm, err := h.service.SavePaymentMethod(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	// Convert to proto response and wrap in Connect response
	return connect.NewResponse(paymentMethodToResponse(pm)), nil
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
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, domain.ErrPaymentMethodNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("payment method not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get payment method"))
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
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list payment methods"))
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
			if pm.IsActive == *msg.IsActive {
				filtered = append(filtered, pm)
			}
		}
		pms = filtered
	}

	protoPMs := make([]*paymentmethodv1.PaymentMethod, len(pms))
	for i, pm := range pms {
		protoPMs[i] = paymentMethodToProto(pm)
	}

	response := &paymentmethodv1.ListPaymentMethodsResponse{
		PaymentMethods: protoPMs,
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
		return nil, handleServiceErrorConnect(err)
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
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, domain.ErrPaymentMethodNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("payment method not found"))
		}
		response := &paymentmethodv1.DeletePaymentMethodResponse{
			Success: false,
			Message: err.Error(),
		}
		return connect.NewResponse(response), nil
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
		return nil, handleServiceErrorConnect(err)
	}

	return connect.NewResponse(paymentMethodToResponse(pm)), nil
}

// VerifyACHAccount sends pre-note for ACH verification
func (h *ConnectHandler) VerifyACHAccount(
	ctx context.Context,
	req *connect.Request[paymentmethodv1.VerifyACHAccountRequest],
) (*connect.Response[paymentmethodv1.VerifyACHAccountResponse], error) {
	msg := req.Msg

	h.logger.Info("VerifyACHAccount request received",
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

	serviceReq := &ports.VerifyACHAccountRequest{
		PaymentMethodID: msg.PaymentMethodId,
		MerchantID:      msg.MerchantId,
		CustomerID:      msg.CustomerId,
	}

	err := h.service.VerifyACHAccount(ctx, serviceReq)
	if err != nil {
		response := &paymentmethodv1.VerifyACHAccountResponse{
			PaymentMethodId: msg.PaymentMethodId,
			Status:          "failed",
			Message:         err.Error(),
		}
		return connect.NewResponse(response), nil
	}

	response := &paymentmethodv1.VerifyACHAccountResponse{
		PaymentMethodId: msg.PaymentMethodId,
		Status:          "verified",
		Message:         "ACH account verified successfully",
	}

	return connect.NewResponse(response), nil
}

// ConvertFinancialBRICToStorageBRIC converts a Financial BRIC to Storage BRIC and saves payment method
func (h *ConnectHandler) ConvertFinancialBRICToStorageBRIC(
	ctx context.Context,
	req *connect.Request[paymentmethodv1.ConvertFinancialBRICRequest],
) (*connect.Response[paymentmethodv1.PaymentMethodResponse], error) {
	msg := req.Msg

	h.logger.Info("ConvertFinancialBRICToStorageBRIC request received",
		zap.String("merchant_id", msg.MerchantId),
		zap.String("customer_id", msg.CustomerId),
		zap.String("transaction_id", msg.TransactionId),
		zap.String("payment_type", msg.PaymentType.String()),
	)

	// Validate request
	if err := validateConvertFinancialBRICRequest(msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Convert to service request
	serviceReq := &ports.ConvertFinancialBRICRequest{
		MerchantID:    msg.MerchantId,
		CustomerID:    msg.CustomerId,
		FinancialBRIC: msg.FinancialBric,
		PaymentType:   paymentMethodTypeFromProto(msg.PaymentType),
		TransactionID: msg.TransactionId,
		LastFour:      msg.LastFour,
		IsDefault:     msg.IsDefault,
	}

	// Credit card fields
	if msg.CardBrand != nil {
		serviceReq.CardBrand = msg.CardBrand
	}
	if msg.CardExpMonth != nil {
		month := int(*msg.CardExpMonth)
		serviceReq.CardExpMonth = &month
	}
	if msg.CardExpYear != nil {
		year := int(*msg.CardExpYear)
		serviceReq.CardExpYear = &year
	}

	// ACH fields
	if msg.BankName != nil {
		serviceReq.BankName = msg.BankName
	}
	if msg.AccountType != nil {
		serviceReq.AccountType = msg.AccountType
	}

	// Billing information
	if msg.FirstName != nil {
		serviceReq.FirstName = msg.FirstName
	}
	if msg.LastName != nil {
		serviceReq.LastName = msg.LastName
	}
	if msg.Address != nil {
		serviceReq.Address = msg.Address
	}
	if msg.City != nil {
		serviceReq.City = msg.City
	}
	if msg.State != nil {
		serviceReq.State = msg.State
	}
	if msg.ZipCode != nil {
		serviceReq.ZipCode = msg.ZipCode
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	// Call service
	pm, err := h.service.ConvertFinancialBRICToStorageBRIC(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	h.logger.Info("Financial BRIC converted to Storage BRIC successfully",
		zap.String("payment_method_id", pm.ID),
		zap.String("customer_id", pm.CustomerID),
	)

	// Convert to proto response and wrap in Connect response
	return connect.NewResponse(paymentMethodToResponse(pm)), nil
}

// handleServiceErrorConnect maps domain errors to Connect error codes
func handleServiceErrorConnect(err error) error {
	// Map domain errors to Connect status codes
	switch {
	case errors.Is(err, domain.ErrPaymentMethodNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("payment method not found"))
	case errors.Is(err, domain.ErrPaymentMethodExpired):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("payment method is expired"))
	case errors.Is(err, domain.ErrPaymentMethodNotVerified):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("ACH payment method is not verified"))
	case errors.Is(err, domain.ErrPaymentMethodInactive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("payment method is inactive"))
	case errors.Is(err, domain.ErrInvalidPaymentMethodType):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid payment method type"))
	case errors.Is(err, domain.ErrMerchantInactive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("agent is inactive"))
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("duplicate idempotency key"))
	case errors.Is(err, sql.ErrNoRows):
		return connect.NewError(connect.CodeNotFound, errors.New("resource not found"))
	case err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)):
		return connect.NewError(connect.CodeCanceled, errors.New("request canceled"))
	default:
		// Log internal errors but don't expose details to client
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
}
