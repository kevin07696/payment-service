package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	subscriptionv1 "github.com/kevin07696/payment-service/proto/subscription/v1"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"go.uber.org/zap"
)

// Handler implements the gRPC SubscriptionServiceServer
type Handler struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	service ports.SubscriptionService
	logger  *zap.Logger
}

// NewHandler creates a new subscription handler
func NewHandler(service ports.SubscriptionService, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// CreateSubscription creates a new recurring billing subscription
func (h *Handler) CreateSubscription(ctx context.Context, req *subscriptionv1.CreateSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("CreateSubscription request received",
		zap.String("agent_id", req.AgentId),
		zap.String("customer_id", req.CustomerId),
		zap.String("amount", req.Amount),
	)

	// Validate request
	if err := validateCreateSubscriptionRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Convert to service request
	serviceReq := &ports.CreateSubscriptionRequest{
		AgentID:         req.AgentId,
		CustomerID:      req.CustomerId,
		Amount:          req.Amount,
		Currency:        req.Currency,
		IntervalValue:   int(req.IntervalValue),
		IntervalUnit:    intervalUnitFromProto(req.IntervalUnit),
		PaymentMethodID: req.PaymentMethodId,
		StartDate:       req.StartDate.AsTime(),
		MaxRetries:      int(req.MaxRetries),
		Metadata:        convertMetadata(req.Metadata),
	}

	if serviceReq.MaxRetries == 0 {
		serviceReq.MaxRetries = 3 // Default
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	// Call service
	sub, err := h.service.CreateSubscription(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Convert to proto response
	return subscriptionToResponse(sub), nil
}

// UpdateSubscription updates subscription properties
func (h *Handler) UpdateSubscription(ctx context.Context, req *subscriptionv1.UpdateSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("UpdateSubscription request received",
		zap.String("subscription_id", req.SubscriptionId),
	)

	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	serviceReq := &ports.UpdateSubscriptionRequest{
		SubscriptionID: req.SubscriptionId,
	}

	if req.Amount != nil {
		serviceReq.Amount = req.Amount
	}

	if req.IntervalValue != nil {
		val := int(*req.IntervalValue)
		serviceReq.IntervalValue = &val
	}

	if req.IntervalUnit != nil {
		unit := intervalUnitFromProto(*req.IntervalUnit)
		serviceReq.IntervalUnit = &unit
	}

	if req.PaymentMethodId != nil {
		serviceReq.PaymentMethodID = req.PaymentMethodId
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	sub, err := h.service.UpdateSubscription(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return subscriptionToResponse(sub), nil
}

// CancelSubscription cancels an active subscription
func (h *Handler) CancelSubscription(ctx context.Context, req *subscriptionv1.CancelSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("CancelSubscription request received",
		zap.String("subscription_id", req.SubscriptionId),
		zap.Bool("cancel_at_period_end", req.CancelAtPeriodEnd),
	)

	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	serviceReq := &ports.CancelSubscriptionRequest{
		SubscriptionID:    req.SubscriptionId,
		CancelAtPeriodEnd: req.CancelAtPeriodEnd,
		Reason:            req.Reason,
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	sub, err := h.service.CancelSubscription(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return subscriptionToResponse(sub), nil
}

// PauseSubscription pauses an active subscription
func (h *Handler) PauseSubscription(ctx context.Context, req *subscriptionv1.PauseSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("PauseSubscription request received",
		zap.String("subscription_id", req.SubscriptionId),
	)

	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	sub, err := h.service.PauseSubscription(ctx, req.SubscriptionId)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return subscriptionToResponse(sub), nil
}

// ResumeSubscription resumes a paused subscription
func (h *Handler) ResumeSubscription(ctx context.Context, req *subscriptionv1.ResumeSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("ResumeSubscription request received",
		zap.String("subscription_id", req.SubscriptionId),
	)

	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	sub, err := h.service.ResumeSubscription(ctx, req.SubscriptionId)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return subscriptionToResponse(sub), nil
}

// GetSubscription retrieves subscription details
func (h *Handler) GetSubscription(ctx context.Context, req *subscriptionv1.GetSubscriptionRequest) (*subscriptionv1.Subscription, error) {
	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	sub, err := h.service.GetSubscription(ctx, req.SubscriptionId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, domain.ErrSubscriptionNotFound) {
			return nil, status.Error(codes.NotFound, "subscription not found")
		}
		return nil, status.Error(codes.Internal, "failed to get subscription")
	}

	return subscriptionToProto(sub), nil
}

// ListCustomerSubscriptions lists all subscriptions for a customer
func (h *Handler) ListCustomerSubscriptions(ctx context.Context, req *subscriptionv1.ListCustomerSubscriptionsRequest) (*subscriptionv1.ListCustomerSubscriptionsResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	subs, err := h.service.ListCustomerSubscriptions(ctx, req.AgentId, req.CustomerId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list subscriptions")
	}

	// Filter by status if provided
	if req.Status != nil {
		desiredStatus := subscriptionStatusFromProto(*req.Status)
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

	return &subscriptionv1.ListCustomerSubscriptionsResponse{
		Subscriptions: protoSubs,
	}, nil
}

// ProcessDueBilling processes subscriptions due for billing (internal/admin use)
func (h *Handler) ProcessDueBilling(ctx context.Context, req *subscriptionv1.ProcessDueBillingRequest) (*subscriptionv1.ProcessDueBillingResponse, error) {
	h.logger.Info("ProcessDueBilling request received",
		zap.Time("as_of_date", req.AsOfDate.AsTime()),
		zap.Int32("batch_size", req.BatchSize),
	)

	batchSize := int(req.BatchSize)
	if batchSize <= 0 {
		batchSize = 100 // Default
	}

	processed, success, failed, errors := h.service.ProcessDueBilling(ctx, req.AsOfDate.AsTime(), batchSize)

	// Convert errors to billing errors
	billingErrors := make([]*subscriptionv1.BillingError, len(errors))
	for i, err := range errors {
		billingErrors[i] = &subscriptionv1.BillingError{
			Error:     err.Error(),
			Retriable: isRetriableError(err),
		}
	}

	return &subscriptionv1.ProcessDueBillingResponse{
		ProcessedCount: int32(processed),
		SuccessCount:   int32(success),
		FailedCount:    int32(failed),
		SkippedCount:   int32(0), // Not tracking skipped yet
		Errors:         billingErrors,
	}, nil
}

// Validation helpers

func validateCreateSubscriptionRequest(req *subscriptionv1.CreateSubscriptionRequest) error {
	if req.AgentId == "" {
		return fmt.Errorf("agent_id is required")
	}
	if req.CustomerId == "" {
		return fmt.Errorf("customer_id is required")
	}
	if req.Amount == "" {
		return fmt.Errorf("amount is required")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.IntervalValue <= 0 {
		return fmt.Errorf("interval_value must be positive")
	}
	if req.IntervalUnit == subscriptionv1.IntervalUnit_INTERVAL_UNIT_UNSPECIFIED {
		return fmt.Errorf("interval_unit is required")
	}
	if req.PaymentMethodId == "" {
		return fmt.Errorf("payment_method_id is required")
	}
	return nil
}

// Conversion helpers

func convertMetadata(meta map[string]string) map[string]interface{} {
	if meta == nil {
		return nil
	}
	result := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		result[k] = v
	}
	return result
}

func subscriptionToResponse(sub *domain.Subscription) *subscriptionv1.SubscriptionResponse {
	resp := &subscriptionv1.SubscriptionResponse{
		SubscriptionId:  sub.ID,
		AgentId:         sub.AgentID,
		CustomerId:      sub.CustomerID,
		Amount:          sub.Amount.String(),
		Currency:        string(sub.Currency),
		IntervalValue:   int32(sub.IntervalValue),
		IntervalUnit:    intervalUnitToProto(sub.IntervalUnit),
		Status:          subscriptionStatusToProto(sub.Status),
		PaymentMethodId: sub.PaymentMethodID,
		NextBillingDate: timestamppb.New(sub.NextBillingDate),
		CreatedAt:       timestamppb.New(sub.CreatedAt),
		UpdatedAt:       timestamppb.New(sub.UpdatedAt),
	}

	if sub.GatewaySubscriptionID != nil {
		resp.GatewaySubscriptionId = *sub.GatewaySubscriptionID
	}

	if sub.CancelledAt != nil {
		resp.CancelledAt = timestamppb.New(*sub.CancelledAt)
	}

	return resp
}

func subscriptionToProto(sub *domain.Subscription) *subscriptionv1.Subscription {
	proto := &subscriptionv1.Subscription{
		Id:                 sub.ID,
		AgentId:            sub.AgentID,
		CustomerId:         sub.CustomerID,
		Amount:             sub.Amount.String(),
		Currency:           string(sub.Currency),
		IntervalValue:      int32(sub.IntervalValue),
		IntervalUnit:       intervalUnitToProto(sub.IntervalUnit),
		Status:             subscriptionStatusToProto(sub.Status),
		PaymentMethodId:    sub.PaymentMethodID,
		NextBillingDate:    timestamppb.New(sub.NextBillingDate),
		FailureRetryCount:  int32(sub.FailureRetryCount),
		MaxRetries:         int32(sub.MaxRetries),
		CreatedAt:          timestamppb.New(sub.CreatedAt),
		UpdatedAt:          timestamppb.New(sub.UpdatedAt),
		Metadata:           convertMetadataToProto(sub.Metadata),
	}

	if sub.GatewaySubscriptionID != nil {
		proto.GatewaySubscriptionId = *sub.GatewaySubscriptionID
	}

	if sub.CancelledAt != nil {
		proto.CancelledAt = timestamppb.New(*sub.CancelledAt)
	}

	return proto
}

func intervalUnitToProto(unit domain.IntervalUnit) subscriptionv1.IntervalUnit {
	switch unit {
	case domain.IntervalUnitDay:
		return subscriptionv1.IntervalUnit_INTERVAL_UNIT_DAY
	case domain.IntervalUnitWeek:
		return subscriptionv1.IntervalUnit_INTERVAL_UNIT_WEEK
	case domain.IntervalUnitMonth:
		return subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH
	case domain.IntervalUnitYear:
		return subscriptionv1.IntervalUnit_INTERVAL_UNIT_YEAR
	default:
		return subscriptionv1.IntervalUnit_INTERVAL_UNIT_UNSPECIFIED
	}
}

func intervalUnitFromProto(unit subscriptionv1.IntervalUnit) domain.IntervalUnit {
	switch unit {
	case subscriptionv1.IntervalUnit_INTERVAL_UNIT_DAY:
		return domain.IntervalUnitDay
	case subscriptionv1.IntervalUnit_INTERVAL_UNIT_WEEK:
		return domain.IntervalUnitWeek
	case subscriptionv1.IntervalUnit_INTERVAL_UNIT_MONTH:
		return domain.IntervalUnitMonth
	case subscriptionv1.IntervalUnit_INTERVAL_UNIT_YEAR:
		return domain.IntervalUnitYear
	default:
		return domain.IntervalUnitMonth // Default
	}
}

func subscriptionStatusToProto(status domain.SubscriptionStatus) subscriptionv1.SubscriptionStatus {
	switch status {
	case domain.SubscriptionStatusActive:
		return subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_ACTIVE
	case domain.SubscriptionStatusPaused:
		return subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAUSED
	case domain.SubscriptionStatusCancelled:
		return subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_CANCELLED
	case domain.SubscriptionStatusPastDue:
		return subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAST_DUE
	default:
		return subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_UNSPECIFIED
	}
}

func subscriptionStatusFromProto(status subscriptionv1.SubscriptionStatus) domain.SubscriptionStatus {
	switch status {
	case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_ACTIVE:
		return domain.SubscriptionStatusActive
	case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAUSED:
		return domain.SubscriptionStatusPaused
	case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_CANCELLED:
		return domain.SubscriptionStatusCancelled
	case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAST_DUE:
		return domain.SubscriptionStatusPastDue
	default:
		return domain.SubscriptionStatusActive
	}
}

func convertMetadataToProto(meta map[string]interface{}) map[string]string {
	if meta == nil {
		return nil
	}
	result := make(map[string]string, len(meta))
	for k, v := range meta {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

func isRetriableError(err error) bool {
	// Determine if error is retriable
	switch {
	case errors.Is(err, domain.ErrGatewayTimeout):
		return true
	case errors.Is(err, domain.ErrGatewayUnavailable):
		return true
	case errors.Is(err, domain.ErrPaymentMethodNotVerified):
		return false // Need user action
	case errors.Is(err, domain.ErrPaymentMethodExpired):
		return false // Need user action
	case errors.Is(err, domain.ErrPaymentMethodInactive):
		return false // Need user action
	default:
		return false
	}
}

// Error handling

func handleServiceError(err error) error {
	// Map domain errors to gRPC status codes
	switch {
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		return status.Error(codes.NotFound, "subscription not found")
	case errors.Is(err, domain.ErrSubscriptionNotActive):
		return status.Error(codes.FailedPrecondition, "subscription is not active")
	case errors.Is(err, domain.ErrSubscriptionAlreadyCancelled):
		return status.Error(codes.FailedPrecondition, "subscription is already cancelled")
	case errors.Is(err, domain.ErrPaymentMethodNotFound):
		return status.Error(codes.NotFound, "payment method not found")
	case errors.Is(err, domain.ErrPaymentMethodExpired):
		return status.Error(codes.FailedPrecondition, "payment method is expired")
	case errors.Is(err, domain.ErrPaymentMethodNotVerified):
		return status.Error(codes.FailedPrecondition, "ACH payment method is not verified")
	case errors.Is(err, domain.ErrPaymentMethodInactive):
		return status.Error(codes.FailedPrecondition, "payment method is inactive")
	case errors.Is(err, domain.ErrInvalidBillingInterval):
		return status.Error(codes.InvalidArgument, "invalid billing interval")
	case errors.Is(err, domain.ErrInvalidAmount):
		return status.Error(codes.InvalidArgument, "invalid amount")
	case errors.Is(err, domain.ErrInvalidCurrency):
		return status.Error(codes.InvalidArgument, "invalid currency")
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
