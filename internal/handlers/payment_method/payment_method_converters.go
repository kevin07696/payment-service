package payment_method

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/domain"
	paymentmethodv1 "github.com/kevin07696/payment-service/proto/payment_method/v1"
)

// Conversion helpers

func paymentMethodToResponse(pm *domain.PaymentMethod) *paymentmethodv1.PaymentMethodResponse {
	resp := &paymentmethodv1.PaymentMethodResponse{
		PaymentMethodId: pm.ID,
		MerchantId:      pm.MerchantID,
		CustomerId:      pm.CustomerID,
		PaymentType:     paymentMethodTypeToProto(pm.PaymentType),
		LastFour:        pm.LastFour,
		IsDefault:       pm.IsDefault,
		IsActive:        pm.IsActive(),   // Uses Status field via method
		IsVerified:      pm.IsVerified(), // Uses Status field via method
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
		MerchantId:  pm.MerchantID,
		CustomerId:  pm.CustomerID,
		PaymentType: paymentMethodTypeToProto(pm.PaymentType),
		LastFour:    pm.LastFour,
		IsDefault:   pm.IsDefault,
		IsActive:    pm.IsActive(),   // Uses Status field via method
		IsVerified:  pm.IsVerified(), // Uses Status field via method
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
