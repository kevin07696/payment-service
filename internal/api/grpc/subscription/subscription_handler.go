package subscription

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	subscriptionv1 "github.com/kevin07696/payment-service/api/proto/subscription/v1"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	"github.com/shopspring/decimal"
)

// Handler implements the gRPC Subscription Service
type Handler struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	subscriptionService ports.SubscriptionService
	logger              ports.Logger
}

// NewHandler creates a new subscription gRPC handler
func NewHandler(subscriptionService ports.SubscriptionService, logger ports.Logger) *Handler {
	return &Handler{
		subscriptionService: subscriptionService,
		logger:              logger,
	}
}

// CreateSubscription creates a new recurring billing subscription
func (h *Handler) CreateSubscription(ctx context.Context, req *subscriptionv1.CreateSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("gRPC CreateSubscription request received",
		ports.String("merchant_id", req.MerchantId),
		ports.String("customer_id", req.CustomerId))

	if err := validateCreateSubscriptionRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid amount: %v", err))
	}

	serviceReq := ports.ServiceCreateSubscriptionRequest{
		MerchantID:         req.MerchantId,
		CustomerID:         req.CustomerId,
		Amount:             amount,
		Currency:           req.Currency,
		Frequency:          toModelBillingFrequency(req.Frequency),
		PaymentMethodToken: req.PaymentMethodToken,
		StartDate:          req.StartDate.AsTime(),
		MaxRetries:         int(req.MaxRetries),
		FailureOption:      toModelFailureOption(req.FailureOption),
		Metadata:           req.Metadata,
		IdempotencyKey:     req.IdempotencyKey,
	}

	// Set default max retries if not specified
	if serviceReq.MaxRetries == 0 {
		serviceReq.MaxRetries = 3
	}

	resp, err := h.subscriptionService.CreateSubscription(ctx, serviceReq)
	if err != nil {
		h.logger.Error("CreateSubscription failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("create subscription failed: %v", err))
	}

	return toProtoSubscriptionResponse(resp), nil
}

// UpdateSubscription updates subscription properties
func (h *Handler) UpdateSubscription(ctx context.Context, req *subscriptionv1.UpdateSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("gRPC UpdateSubscription request received",
		ports.String("subscription_id", req.SubscriptionId))

	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	serviceReq := ports.ServiceUpdateSubscriptionRequest{
		SubscriptionID: req.SubscriptionId,
		IdempotencyKey: req.IdempotencyKey,
	}

	if req.Amount != nil {
		amount, err := decimal.NewFromString(*req.Amount)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid amount: %v", err))
		}
		serviceReq.Amount = &amount
	}

	if req.Frequency != nil {
		freq := toModelBillingFrequency(*req.Frequency)
		serviceReq.Frequency = &freq
	}

	if req.PaymentMethodToken != nil {
		serviceReq.PaymentMethodToken = req.PaymentMethodToken
	}

	resp, err := h.subscriptionService.UpdateSubscription(ctx, serviceReq)
	if err != nil {
		h.logger.Error("UpdateSubscription failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("update subscription failed: %v", err))
	}

	return toProtoSubscriptionResponse(resp), nil
}

// CancelSubscription cancels an active subscription
func (h *Handler) CancelSubscription(ctx context.Context, req *subscriptionv1.CancelSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("gRPC CancelSubscription request received",
		ports.String("subscription_id", req.SubscriptionId))

	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	serviceReq := ports.ServiceCancelSubscriptionRequest{
		SubscriptionID:    req.SubscriptionId,
		CancelAtPeriodEnd: req.CancelAtPeriodEnd,
		Reason:            req.Reason,
		IdempotencyKey:    req.IdempotencyKey,
	}

	resp, err := h.subscriptionService.CancelSubscription(ctx, serviceReq)
	if err != nil {
		h.logger.Error("CancelSubscription failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("cancel subscription failed: %v", err))
	}

	return toProtoSubscriptionResponse(resp), nil
}

// PauseSubscription pauses an active subscription
func (h *Handler) PauseSubscription(ctx context.Context, req *subscriptionv1.PauseSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("gRPC PauseSubscription request received",
		ports.String("subscription_id", req.SubscriptionId))

	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	resp, err := h.subscriptionService.PauseSubscription(ctx, req.SubscriptionId)
	if err != nil {
		h.logger.Error("PauseSubscription failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("pause subscription failed: %v", err))
	}

	return toProtoSubscriptionResponse(resp), nil
}

// ResumeSubscription resumes a paused subscription
func (h *Handler) ResumeSubscription(ctx context.Context, req *subscriptionv1.ResumeSubscriptionRequest) (*subscriptionv1.SubscriptionResponse, error) {
	h.logger.Info("gRPC ResumeSubscription request received",
		ports.String("subscription_id", req.SubscriptionId))

	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	resp, err := h.subscriptionService.ResumeSubscription(ctx, req.SubscriptionId)
	if err != nil {
		h.logger.Error("ResumeSubscription failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("resume subscription failed: %v", err))
	}

	return toProtoSubscriptionResponse(resp), nil
}

// GetSubscription retrieves subscription details
func (h *Handler) GetSubscription(ctx context.Context, req *subscriptionv1.GetSubscriptionRequest) (*subscriptionv1.Subscription, error) {
	if req.SubscriptionId == "" {
		return nil, status.Error(codes.InvalidArgument, "subscription_id is required")
	}

	sub, err := h.subscriptionService.GetSubscription(ctx, req.SubscriptionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("subscription not found: %v", err))
	}

	return toProtoSubscription(sub), nil
}

// ListCustomerSubscriptions lists all subscriptions for a customer
func (h *Handler) ListCustomerSubscriptions(ctx context.Context, req *subscriptionv1.ListCustomerSubscriptionsRequest) (*subscriptionv1.ListCustomerSubscriptionsResponse, error) {
	if req.MerchantId == "" {
		return nil, status.Error(codes.InvalidArgument, "merchant_id is required")
	}
	if req.CustomerId == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}

	subs, err := h.subscriptionService.ListCustomerSubscriptions(ctx, req.MerchantId, req.CustomerId)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("list subscriptions failed: %v", err))
	}

	protoSubs := make([]*subscriptionv1.Subscription, len(subs))
	for i, sub := range subs {
		protoSubs[i] = toProtoSubscription(sub)
	}

	return &subscriptionv1.ListCustomerSubscriptionsResponse{
		Subscriptions: protoSubs,
	}, nil
}

// ProcessDueBilling processes subscriptions due for billing
func (h *Handler) ProcessDueBilling(ctx context.Context, req *subscriptionv1.ProcessDueBillingRequest) (*subscriptionv1.ProcessDueBillingResponse, error) {
	h.logger.Info("gRPC ProcessDueBilling request received")

	batchSize := int(req.BatchSize)
	if batchSize == 0 {
		batchSize = 100
	}

	asOfDate := req.AsOfDate.AsTime()

	result, err := h.subscriptionService.ProcessDueBilling(ctx, asOfDate, batchSize)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("process due billing failed: %v", err))
	}

	protoErrors := make([]*subscriptionv1.BillingError, len(result.Errors))
	for i, billingErr := range result.Errors {
		protoErrors[i] = &subscriptionv1.BillingError{
			SubscriptionId: billingErr.SubscriptionID,
			CustomerId:     billingErr.CustomerID,
			Error:          billingErr.Error,
			Retriable:      billingErr.Retriable,
		}
	}

	return &subscriptionv1.ProcessDueBillingResponse{
		ProcessedCount: int32(result.ProcessedCount),
		SuccessCount:   int32(result.SuccessCount),
		FailedCount:    int32(result.FailedCount),
		SkippedCount:   int32(result.SkippedCount),
		Errors:         protoErrors,
	}, nil
}

// Validation helpers

func validateCreateSubscriptionRequest(req *subscriptionv1.CreateSubscriptionRequest) error {
	if req.MerchantId == "" {
		return fmt.Errorf("merchant_id is required")
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
	if req.Frequency == subscriptionv1.BillingFrequency_BILLING_FREQUENCY_UNSPECIFIED {
		return fmt.Errorf("frequency is required")
	}
	if req.PaymentMethodToken == "" {
		return fmt.Errorf("payment_method_token is required")
	}
	if req.StartDate == nil {
		return fmt.Errorf("start_date is required")
	}
	return nil
}

// Conversion helpers

func toProtoSubscriptionResponse(resp *ports.ServiceSubscriptionResponse) *subscriptionv1.SubscriptionResponse {
	protoResp := &subscriptionv1.SubscriptionResponse{
		SubscriptionId:        resp.SubscriptionID,
		MerchantId:            resp.MerchantID,
		CustomerId:            resp.CustomerID,
		Amount:                resp.Amount.String(),
		Currency:              resp.Currency,
		Frequency:             toProtoBillingFrequency(resp.Frequency),
		Status:                toProtoSubscriptionStatus(resp.Status),
		PaymentMethodToken:    resp.PaymentMethodToken,
		NextBillingDate:       timestamppb.New(resp.NextBillingDate),
		GatewaySubscriptionId: resp.GatewaySubscriptionID,
		CreatedAt:             timestamppb.New(resp.CreatedAt),
		UpdatedAt:             timestamppb.New(resp.UpdatedAt),
	}

	if resp.CancelledAt != nil {
		protoResp.CancelledAt = timestamppb.New(*resp.CancelledAt)
	}

	return protoResp
}

func toProtoSubscription(sub *models.Subscription) *subscriptionv1.Subscription {
	protoSub := &subscriptionv1.Subscription{
		Id:                    sub.ID,
		MerchantId:            sub.MerchantID,
		CustomerId:            sub.CustomerID,
		Amount:                sub.Amount.String(),
		Currency:              sub.Currency,
		Frequency:             toProtoBillingFrequency(sub.Frequency),
		Status:                toProtoSubscriptionStatus(sub.Status),
		PaymentMethodToken:    sub.PaymentMethodToken,
		NextBillingDate:       timestamppb.New(sub.NextBillingDate),
		GatewaySubscriptionId: sub.GatewaySubscriptionID,
		FailureRetryCount:     int32(sub.FailureRetryCount),
		MaxRetries:            int32(sub.MaxRetries),
		FailureOption:         toProtoFailureOption(sub.FailureOption),
		CreatedAt:             timestamppb.New(sub.CreatedAt),
		UpdatedAt:             timestamppb.New(sub.UpdatedAt),
		Metadata:              sub.Metadata,
	}

	if sub.CancelledAt != nil {
		protoSub.CancelledAt = timestamppb.New(*sub.CancelledAt)
	}

	return protoSub
}

func toProtoBillingFrequency(freq models.BillingFrequency) subscriptionv1.BillingFrequency {
	switch freq {
	case models.FrequencyWeekly:
		return subscriptionv1.BillingFrequency_BILLING_FREQUENCY_WEEKLY
	case models.FrequencyBiWeekly:
		return subscriptionv1.BillingFrequency_BILLING_FREQUENCY_BIWEEKLY
	case models.FrequencyMonthly:
		return subscriptionv1.BillingFrequency_BILLING_FREQUENCY_MONTHLY
	case models.FrequencyYearly:
		return subscriptionv1.BillingFrequency_BILLING_FREQUENCY_YEARLY
	default:
		return subscriptionv1.BillingFrequency_BILLING_FREQUENCY_UNSPECIFIED
	}
}

func toModelBillingFrequency(freq subscriptionv1.BillingFrequency) models.BillingFrequency {
	switch freq {
	case subscriptionv1.BillingFrequency_BILLING_FREQUENCY_WEEKLY:
		return models.FrequencyWeekly
	case subscriptionv1.BillingFrequency_BILLING_FREQUENCY_BIWEEKLY:
		return models.FrequencyBiWeekly
	case subscriptionv1.BillingFrequency_BILLING_FREQUENCY_MONTHLY:
		return models.FrequencyMonthly
	case subscriptionv1.BillingFrequency_BILLING_FREQUENCY_YEARLY:
		return models.FrequencyYearly
	default:
		return models.FrequencyMonthly
	}
}

func toProtoSubscriptionStatus(status models.SubscriptionStatus) subscriptionv1.SubscriptionStatus {
	switch status {
	case models.SubStatusActive:
		return subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_ACTIVE
	case models.SubStatusPaused:
		return subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAUSED
	case models.SubStatusCancelled:
		return subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_CANCELLED
	default:
		return subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_UNSPECIFIED
	}
}

func toProtoFailureOption(opt models.FailureOption) subscriptionv1.FailureOption {
	switch opt {
	case models.FailureForward:
		return subscriptionv1.FailureOption_FAILURE_OPTION_FORWARD
	case models.FailureSkip:
		return subscriptionv1.FailureOption_FAILURE_OPTION_SKIP
	case models.FailurePause:
		return subscriptionv1.FailureOption_FAILURE_OPTION_PAUSE
	default:
		return subscriptionv1.FailureOption_FAILURE_OPTION_UNSPECIFIED
	}
}

func toModelFailureOption(opt subscriptionv1.FailureOption) models.FailureOption {
	switch opt {
	case subscriptionv1.FailureOption_FAILURE_OPTION_FORWARD:
		return models.FailureForward
	case subscriptionv1.FailureOption_FAILURE_OPTION_SKIP:
		return models.FailureSkip
	case subscriptionv1.FailureOption_FAILURE_OPTION_PAUSE:
		return models.FailurePause
	default:
		return models.FailureForward
	}
}
