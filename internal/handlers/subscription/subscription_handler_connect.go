package subscription

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/handlers"
	"github.com/kevin07696/payment-service/internal/ports"
	subscriptionv1 "github.com/kevin07696/payment-service/proto/subscription/v1"
)

// ConnectHandlerConfig holds configuration for the subscription handler
type ConnectHandlerConfig struct {
	DefaultMaxRetries int // Default max retries before past_due (default: 3)
}

// ConnectHandler implements the Connect RPC SubscriptionServiceHandler interface
type ConnectHandler struct {
	service           ports.SubscriptionService
	logger            *zap.Logger
	defaultMaxRetries int
}

// NewConnectHandler creates a new Connect RPC subscription handler
func NewConnectHandler(service ports.SubscriptionService, logger *zap.Logger, cfg ConnectHandlerConfig) *ConnectHandler {
	maxRetries := 3
	if cfg.DefaultMaxRetries > 0 {
		maxRetries = cfg.DefaultMaxRetries
	}

	return &ConnectHandler{
		service:           service,
		logger:            logger,
		defaultMaxRetries: maxRetries,
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
		serviceReq.MaxRetries = h.defaultMaxRetries
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	// Call service
	sub, err := h.service.CreateSubscription(ctx, serviceReq)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
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
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
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
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
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
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
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
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
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
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	return connect.NewResponse(subscriptionToProto(sub)), nil
}

// ListSubscriptions lists subscriptions with optional filters
func (h *ConnectHandler) ListSubscriptions(
	ctx context.Context,
	req *connect.Request[subscriptionv1.ListSubscriptionsRequest],
) (*connect.Response[subscriptionv1.ListSubscriptionsResponse], error) {
	msg := req.Msg

	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}

	subs, err := h.service.ListSubscriptions(ctx, msg.MerchantId, msg.CustomerId)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
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

	// Get total count before pagination
	totalCount := len(subs)

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
	if offset >= len(subs) {
		subs = []*domain.Subscription{}
	} else {
		end := offset + limit
		if end > len(subs) {
			end = len(subs)
		}
		subs = subs[offset:end]
	}

	protoSubs := make([]*subscriptionv1.Subscription, len(subs))
	for i, sub := range subs {
		protoSubs[i] = subscriptionToProto(sub)
	}

	response := &subscriptionv1.ListSubscriptionsResponse{
		Subscriptions: protoSubs,
		TotalCount:    int32(totalCount),
	}

	return connect.NewResponse(response), nil
}
