package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// SaveCreditCardFromCallbackRequest contains parameters for saving a credit card from Browser Post callback.
type SaveCreditCardFromCallbackRequest struct {
	MerchantID       string
	CustomerID       string
	BRIC             string
	MaskedAccountNbr string
	ExpirationDate   string
	CardTypeCode     string
}

// SaveACHFromCallbackRequest contains parameters for saving an ACH account from Browser Post callback.
type SaveACHFromCallbackRequest struct {
	MerchantID       string
	CustomerID       string
	BRIC             string
	MaskedAccountNbr string
	TransactionType  domain.RequestTransactionType
}

// SendPrenoteRequest contains parameters for sending an ACH prenote.
type SendPrenoteRequest struct {
	MerchantID      string
	PaymentMethodID string
	CustomerID      string
	BRIC            string
	AccountType     string
}

// PaymentMethodService defines the port for payment method operations.
type PaymentMethodService interface {
	GetPaymentMethod(ctx context.Context, paymentMethodID string) (*domain.PaymentMethod, error)
	ListPaymentMethods(ctx context.Context, merchantID, customerID string) ([]*domain.PaymentMethod, error)
	UpdatePaymentMethodStatus(ctx context.Context, paymentMethodID, merchantID, customerID string, isActive bool) (*domain.PaymentMethod, error)
	DeletePaymentMethod(ctx context.Context, paymentMethodID string) error
	SetDefaultPaymentMethod(ctx context.Context, paymentMethodID, merchantID, customerID string) (*domain.PaymentMethod, error)
	SaveCreditCardFromCallback(ctx context.Context, req *SaveCreditCardFromCallbackRequest) (*domain.PaymentMethod, error)
	SaveACHFromCallback(ctx context.Context, req *SaveACHFromCallbackRequest) (*domain.PaymentMethod, error)
	SendPrenote(ctx context.Context, req *SendPrenoteRequest) error
}
