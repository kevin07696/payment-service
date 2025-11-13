package domain

import "errors"

// Common domain errors
var (
	// Transaction errors
	ErrTransactionNotFound         = errors.New("transaction not found")
	ErrTransactionCannotBeVoided   = errors.New("transaction cannot be voided")
	ErrTransactionCannotBeCaptured = errors.New("transaction cannot be captured")
	ErrTransactionCannotBeRefunded = errors.New("transaction cannot be refunded")
	ErrInvalidTransactionStatus    = errors.New("invalid transaction status")
	ErrInvalidTransactionAmount    = errors.New("invalid transaction amount")

	// Subscription errors
	ErrSubscriptionNotFound         = errors.New("subscription not found")
	ErrSubscriptionNotActive        = errors.New("subscription is not active")
	ErrSubscriptionAlreadyCancelled = errors.New("subscription is already cancelled")
	ErrInvalidBillingInterval       = errors.New("invalid billing interval")
	ErrMaxRetriesExceeded           = errors.New("max billing retries exceeded")

	// Payment method errors
	ErrPaymentMethodNotFound    = errors.New("payment method not found")
	ErrPaymentMethodExpired     = errors.New("payment method is expired")
	ErrPaymentMethodNotVerified = errors.New("ACH payment method is not verified")
	ErrPaymentMethodInactive    = errors.New("payment method is inactive")
	ErrInvalidPaymentMethodType = errors.New("invalid payment method type")

	// Chargeback errors
	ErrChargebackNotFound        = errors.New("chargeback not found")
	ErrChargebackCannotRespond   = errors.New("cannot respond to chargeback (deadline passed or already responded)")
	ErrChargebackAlreadyResolved = errors.New("chargeback is already resolved")
	ErrInvalidChargebackStatus   = errors.New("invalid chargeback status")

	// Merchant errors
	ErrMerchantNotFound      = errors.New("merchant not found")
	ErrMerchantInactive      = errors.New("merchant is inactive")
	ErrMerchantAlreadyExists = errors.New("merchant already exists")
	ErrInvalidEnvironment    = errors.New("invalid environment")

	// Gateway errors
	ErrGatewayTimeout         = errors.New("gateway request timed out")
	ErrGatewayUnavailable     = errors.New("gateway is unavailable")
	ErrInvalidGatewayResponse = errors.New("invalid gateway response")
	ErrTransactionDeclined    = errors.New("transaction was declined by gateway")

	// Idempotency errors
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")

	// Validation errors
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrInvalidCurrency      = errors.New("invalid currency")
	ErrMissingRequiredField = errors.New("missing required field")
)
