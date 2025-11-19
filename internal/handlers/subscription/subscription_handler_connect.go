package subscription

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	subscriptionv1 "github.com/kevin07696/payment-service/proto/subscription/v1"
)

// ConnectHandler implements the Connect RPC SubscriptionServiceHandler interface
type ConnectHandler struct {
	service ports.SubscriptionService
	logger  *zap.Logger
}

// NewConnectHandler creates a new Connect RPC subscription handler
func NewConnectHandler(service ports.SubscriptionService, logger *zap.Logger) *ConnectHandler {
	return &ConnectHandler{
		service: service,
		logger:  logger,
	}
}

// CreateSubscription creates a new recurring billing subscription
func (h *ConnectHandler) CreateSubscription(
	ctx context.Context,
	req *connect.Request[subscriptionv1.CreateSubscriptionRequest],
) (*connect.Response[subscriptionv1.SubscriptionResponse], error) {
	msg := req.Msg

	h.logger.Info("CreateSubscription request received",
		zap.String("merchant_id", msg.MerchantId),
		zap.String("customer_id", msg.CustomerId),
		zap.Int64("amount_cents", msg.AmountCents),
	)

	// Validate request
	if err := validateCreateSubscriptionRequest(msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Convert to service request
	serviceReq := &ports.CreateSubscriptionRequest{
		MerchantID:      msg.MerchantId,
		CustomerID:      msg.CustomerId,
		AmountCents:     msg.AmountCents,
		Currency:        msg.Currency,
		IntervalValue:   int(msg.IntervalValue),
		IntervalUnit:    intervalUnitFromProto(msg.IntervalUnit),
		PaymentMethodID: msg.PaymentMethodId,
		StartDate:       msg.StartDate.AsTime(),
		MaxRetries:      int(msg.MaxRetries),
		Metadata:        convertMetadata(msg.Metadata),
	}

	if serviceReq.MaxRetries == 0 {
		serviceReq.MaxRetries = 3 // Default
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	// Call service
	sub, err := h.service.CreateSubscription(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	// Convert to proto response and wrap in Connect response
	return connect.NewResponse(subscriptionToResponse(sub)), nil
}

// UpdateSubscription updates subscription properties
func (h *ConnectHandler) UpdateSubscription(
	ctx context.Context,
	req *connect.Request[subscriptionv1.UpdateSubscriptionRequest],
) (*connect.Response[subscriptionv1.SubscriptionResponse], error) {
	msg := req.Msg

	h.logger.Info("UpdateSubscription request received",
		zap.String("subscription_id", msg.SubscriptionId),
	)

	if msg.SubscriptionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("subscription_id is required"))
	}

	serviceReq := &ports.UpdateSubscriptionRequest{
		SubscriptionID: msg.SubscriptionId,
	}

	if msg.AmountCents != nil {
		serviceReq.AmountCents = msg.AmountCents
	}

	if msg.IntervalValue != nil {
		val := int(*msg.IntervalValue)
		serviceReq.IntervalValue = &val
	}

	if msg.IntervalUnit != nil {
		unit := intervalUnitFromProto(*msg.IntervalUnit)
		serviceReq.IntervalUnit = &unit
	}

	if msg.PaymentMethodId != nil {
		serviceReq.PaymentMethodID = msg.PaymentMethodId
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	sub, err := h.service.UpdateSubscription(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	return connect.NewResponse(subscriptionToResponse(sub)), nil
}

// CancelSubscription cancels an active subscription
func (h *ConnectHandler) CancelSubscription(
	ctx context.Context,
	req *connect.Request[subscriptionv1.CancelSubscriptionRequest],
) (*connect.Response[subscriptionv1.SubscriptionResponse], error) {
	msg := req.Msg

	h.logger.Info("CancelSubscription request received",
		zap.String("subscription_id", msg.SubscriptionId),
		zap.Bool("cancel_at_period_end", msg.CancelAtPeriodEnd),
	)

	if msg.SubscriptionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("subscription_id is required"))
	}

	serviceReq := &ports.CancelSubscriptionRequest{
		SubscriptionID:    msg.SubscriptionId,
		CancelAtPeriodEnd: msg.CancelAtPeriodEnd,
		Reason:            msg.Reason,
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	sub, err := h.service.CancelSubscription(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	return connect.NewResponse(subscriptionToResponse(sub)), nil
}

// PauseSubscription pauses an active subscription
func (h *ConnectHandler) PauseSubscription(
	ctx context.Context,
	req *connect.Request[subscriptionv1.PauseSubscriptionRequest],
) (*connect.Response[subscriptionv1.SubscriptionResponse], error) {
	msg := req.Msg

	h.logger.Info("PauseSubscription request received",
		zap.String("subscription_id", msg.SubscriptionId),
	)

	if msg.SubscriptionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("subscription_id is required"))
	}

	sub, err := h.service.PauseSubscription(ctx, msg.SubscriptionId)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	return connect.NewResponse(subscriptionToResponse(sub)), nil
}

// ResumeSubscription resumes a paused subscription
func (h *ConnectHandler) ResumeSubscription(
	ctx context.Context,
	req *connect.Request[subscriptionv1.ResumeSubscriptionRequest],
) (*connect.Response[subscriptionv1.SubscriptionResponse], error) {
	msg := req.Msg

	h.logger.Info("ResumeSubscription request received",
		zap.String("subscription_id", msg.SubscriptionId),
	)

	if msg.SubscriptionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("subscription_id is required"))
	}

	sub, err := h.service.ResumeSubscription(ctx, msg.SubscriptionId)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	return connect.NewResponse(subscriptionToResponse(sub)), nil
}

// GetSubscription retrieves subscription details
func (h *ConnectHandler) GetSubscription(
	ctx context.Context,
	req *connect.Request[subscriptionv1.GetSubscriptionRequest],
) (*connect.Response[subscriptionv1.Subscription], error) {
	msg := req.Msg

	if msg.SubscriptionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("subscription_id is required"))
	}

	sub, err := h.service.GetSubscription(ctx, msg.SubscriptionId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, domain.ErrSubscriptionNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("subscription not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to get subscription"))
	}

	return connect.NewResponse(subscriptionToProto(sub)), nil
}

// ListCustomerSubscriptions lists all subscriptions for a customer
func (h *ConnectHandler) ListCustomerSubscriptions(
	ctx context.Context,
	req *connect.Request[subscriptionv1.ListCustomerSubscriptionsRequest],
) (*connect.Response[subscriptionv1.ListCustomerSubscriptionsResponse], error) {
	msg := req.Msg

	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}
	if msg.CustomerId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("customer_id is required"))
	}

	subs, err := h.service.ListCustomerSubscriptions(ctx, msg.MerchantId, msg.CustomerId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list subscriptions"))
	}

	// Filter by status if provided
	if msg.Status != nil {
		desiredStatus := subscriptionStatusFromProto(*msg.Status)
		filtered := make([]*domain.Subscription, 0)
		for _, sub := range subs {
			if sub.Status == desiredStatus {
				filtered = append(filtered, sub)
			}
		}
		subs = filtered
	}

	protoSubs := make([]*subscriptionv1.Subscription, len(subs))
	for i, sub := range subs {
		protoSubs[i] = subscriptionToProto(sub)
	}

	response := &subscriptionv1.ListCustomerSubscriptionsResponse{
		Subscriptions: protoSubs,
	}

	return connect.NewResponse(response), nil
}

// ProcessDueBilling processes subscriptions due for billing (internal/admin use)
func (h *ConnectHandler) ProcessDueBilling(
	ctx context.Context,
	req *connect.Request[subscriptionv1.ProcessDueBillingRequest],
) (*connect.Response[subscriptionv1.ProcessDueBillingResponse], error) {
	msg := req.Msg

	h.logger.Info("ProcessDueBilling request received",
		zap.Time("as_of_date", msg.AsOfDate.AsTime()),
		zap.Int32("batch_size", msg.BatchSize),
	)

	batchSize := int(msg.BatchSize)
	if batchSize <= 0 {
		batchSize = 100 // Default
	}

	processed, success, failed, errors := h.service.ProcessDueBilling(ctx, msg.AsOfDate.AsTime(), batchSize)

	// Convert errors to billing errors
	billingErrors := make([]*subscriptionv1.BillingError, len(errors))
	for i, err := range errors {
		billingErrors[i] = &subscriptionv1.BillingError{
			Error:     err.Error(),
			Retriable: isRetriableError(err),
		}
	}

	response := &subscriptionv1.ProcessDueBillingResponse{
		ProcessedCount: int32(processed),
		SuccessCount:   int32(success),
		FailedCount:    int32(failed),
		SkippedCount:   int32(0), // Not tracking skipped yet
		Errors:         billingErrors,
	}

	return connect.NewResponse(response), nil
}

// handleServiceErrorConnect maps domain errors to Connect error codes
func handleServiceErrorConnect(err error) error {
	// Map domain errors to Connect status codes
	switch {
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("subscription not found"))
	case errors.Is(err, domain.ErrSubscriptionNotActive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("subscription is not active"))
	case errors.Is(err, domain.ErrSubscriptionAlreadyCancelled):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("subscription is already cancelled"))
	case errors.Is(err, domain.ErrPaymentMethodNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("payment method not found"))
	case errors.Is(err, domain.ErrPaymentMethodExpired):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("payment method is expired"))
	case errors.Is(err, domain.ErrPaymentMethodNotVerified):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("ACH payment method is not verified"))
	case errors.Is(err, domain.ErrPaymentMethodInactive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("payment method is inactive"))
	case errors.Is(err, domain.ErrInvalidBillingInterval):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid billing interval"))
	case errors.Is(err, domain.ErrInvalidAmount):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid amount"))
	case errors.Is(err, domain.ErrInvalidCurrency):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid currency"))
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
