package subscription

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/domain"
	subscriptionv1 "github.com/kevin07696/payment-service/proto/subscription/v1"
)

// Validation helpers

func validateCreateSubscriptionRequest(req *subscriptionv1.CreateSubscriptionRequest) error {
	if req.MerchantId == "" {
		return fmt.Errorf("merchant_id is required")
	}
	if req.CustomerId == "" {
		return fmt.Errorf("customer_id is required")
	}
	if req.AmountCents <= 0 {
		return fmt.Errorf("amount_cents must be greater than 0")
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
		MerchantId:      sub.MerchantID,
		CustomerId:      sub.CustomerID,
		AmountCents:     sub.AmountCents,
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
		Id:                sub.ID,
		MerchantId:        sub.MerchantID,
		CustomerId:        sub.CustomerID,
		AmountCents:       sub.AmountCents,
		Currency:          string(sub.Currency),
		IntervalValue:     int32(sub.IntervalValue),
		IntervalUnit:      intervalUnitToProto(sub.IntervalUnit),
		Status:            subscriptionStatusToProto(sub.Status),
		PaymentMethodId:   sub.PaymentMethodID,
		NextBillingDate:   timestamppb.New(sub.NextBillingDate),
		FailureRetryCount: int32(sub.FailureRetryCount),
		MaxRetries:        int32(sub.MaxRetries),
		CreatedAt:         timestamppb.New(sub.CreatedAt),
		UpdatedAt:         timestamppb.New(sub.UpdatedAt),
		Metadata:          convertMetadataToProto(sub.Metadata),
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
