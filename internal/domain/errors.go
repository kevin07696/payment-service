package domain

import (
	"errors"
	"fmt"
)

// ErrorCode represents a machine-readable error code
type ErrorCode string

const (
	// Authentication & Authorization Errors (AUTH_*)
	ErrorCodeAuthMissing           ErrorCode = "AUTH_MISSING"
	ErrorCodeAuthInvalid           ErrorCode = "AUTH_INVALID"
	ErrorCodeAuthMerchantMismatch  ErrorCode = "AUTH_MERCHANT_MISMATCH"
	ErrorCodeAuthAccessDenied      ErrorCode = "AUTH_ACCESS_DENIED"
	ErrorCodeAuthInsufficientPerms ErrorCode = "AUTH_INSUFFICIENT_PERMISSIONS"

	// Merchant Errors (MERCHANT_*)
	ErrorCodeMerchantNotFound ErrorCode = "MERCHANT_NOT_FOUND"
	ErrorCodeMerchantInactive ErrorCode = "MERCHANT_INACTIVE"
	ErrorCodeMerchantRequired ErrorCode = "MERCHANT_REQUIRED"

	// Transaction Errors (TXN_*)
	ErrorCodeTxnNotFound         ErrorCode = "TXN_NOT_FOUND"
	ErrorCodeTxnInvalidState     ErrorCode = "TXN_INVALID_STATE"
	ErrorCodeTxnAlreadyProcessed ErrorCode = "TXN_ALREADY_PROCESSED"
	ErrorCodeTxnAmountMismatch   ErrorCode = "TXN_AMOUNT_MISMATCH"
	ErrorCodeTxnProcessingFailed ErrorCode = "TXN_PROCESSING_FAILED"

	// Payment Method Errors (PM_*)
	ErrorCodePMNotFound        ErrorCode = "PM_NOT_FOUND"
	ErrorCodePMRequired        ErrorCode = "PM_REQUIRED"
	ErrorCodePMInvalid         ErrorCode = "PM_INVALID"
	ErrorCodePMExpired         ErrorCode = "PM_EXPIRED"
	ErrorCodePMNotVerified     ErrorCode = "PM_NOT_VERIFIED"
	ErrorCodePMInsufficientACH ErrorCode = "PM_INSUFFICIENT_ACH_VERIFICATIONS"

	// Customer Errors (CUSTOMER_*)
	ErrorCodeCustomerNotFound ErrorCode = "CUSTOMER_NOT_FOUND"

	// Validation Errors (VALIDATION_*)
	ErrorCodeValidationFailed        ErrorCode = "VALIDATION_FAILED"
	ErrorCodeValidationAmountInvalid ErrorCode = "VALIDATION_AMOUNT_INVALID"
	ErrorCodeValidationMissingField  ErrorCode = "VALIDATION_MISSING_FIELD"

	// Payment Gateway Errors (GATEWAY_*)
	ErrorCodeGatewayError    ErrorCode = "GATEWAY_ERROR"
	ErrorCodeGatewayTimeout  ErrorCode = "GATEWAY_TIMEOUT"
	ErrorCodeGatewayDeclined ErrorCode = "GATEWAY_DECLINED"

	// Idempotency Errors (IDEMPOTENCY_*)
	ErrorCodeIdempotencyConflict ErrorCode = "IDEMPOTENCY_CONFLICT"

	// Internal Errors (INTERNAL_*)
	ErrorCodeInternalError ErrorCode = "INTERNAL_ERROR"
	ErrorCodeDatabaseError ErrorCode = "INTERNAL_DATABASE_ERROR"
)

// DomainError represents a structured domain error with error code and context
type DomainError struct {
	Err     error
	Details map[string]interface{}
	Code    ErrorCode
	Message string
}

// Error implements the error interface
func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Is/As support
func (e *DomainError) Unwrap() error {
	return e.Err
}

// WithDetail adds a detail field to the error
func (e *DomainError) WithDetail(key string, value interface{}) *DomainError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// NewDomainError creates a new domain error
func NewDomainError(code ErrorCode, message string) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WrapError wraps an existing error with a domain error code
func WrapError(code ErrorCode, message string, err error) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
		Err:     err,
	}
}

// IsDomainError checks if an error is a DomainError with the given code
func IsDomainError(err error, code ErrorCode) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code == code
	}
	return false
}

// GetErrorCode extracts the error code from an error, returns empty string if not a DomainError
func GetErrorCode(err error) ErrorCode {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code
	}
	return ""
}

// IsNotFoundError checks if an error represents a "not found" condition
func IsNotFoundError(err error) bool {
	code := GetErrorCode(err)
	return code == ErrorCodeMerchantNotFound ||
		code == ErrorCodeTxnNotFound ||
		code == ErrorCodePMNotFound ||
		code == ErrorCodeCustomerNotFound
}

// IsAuthError checks if an error is authentication/authorization related
func IsAuthError(err error) bool {
	code := GetErrorCode(err)
	return code == ErrorCodeAuthMissing ||
		code == ErrorCodeAuthInvalid ||
		code == ErrorCodeAuthMerchantMismatch ||
		code == ErrorCodeAuthAccessDenied ||
		code == ErrorCodeAuthInsufficientPerms
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	code := GetErrorCode(err)
	return code == ErrorCodeValidationFailed ||
		code == ErrorCodeValidationAmountInvalid ||
		code == ErrorCodeValidationMissingField
}

// IsGatewayError checks if an error is a payment gateway error
func IsGatewayError(err error) bool {
	code := GetErrorCode(err)
	return code == ErrorCodeGatewayError ||
		code == ErrorCodeGatewayTimeout ||
		code == ErrorCodeGatewayDeclined
}

// Structured error instances (new style)
var (
	ErrAuthMissing          = NewDomainError(ErrorCodeAuthMissing, "authentication required")
	ErrAuthInvalid          = NewDomainError(ErrorCodeAuthInvalid, "invalid authentication")
	ErrAuthMerchantMismatch = NewDomainError(ErrorCodeAuthMerchantMismatch, "merchant ID mismatch")
	ErrAuthAccessDenied     = NewDomainError(ErrorCodeAuthAccessDenied, "access denied")

	ErrMerchantNotFoundTyped = NewDomainError(ErrorCodeMerchantNotFound, "merchant not found")
	ErrMerchantInactiveTyped = NewDomainError(ErrorCodeMerchantInactive, "merchant is not active")
	ErrMerchantRequired      = NewDomainError(ErrorCodeMerchantRequired, "merchant_id is required")

	ErrTxnNotFound         = NewDomainError(ErrorCodeTxnNotFound, "transaction not found")
	ErrTxnInvalidState     = NewDomainError(ErrorCodeTxnInvalidState, "transaction is in invalid state for this operation")
	ErrTxnAlreadyProcessed = NewDomainError(ErrorCodeTxnAlreadyProcessed, "transaction already processed")

	ErrPMNotFound    = NewDomainError(ErrorCodePMNotFound, "payment method not found")
	ErrPMRequired    = NewDomainError(ErrorCodePMRequired, "payment method required")
	ErrPMInvalid     = NewDomainError(ErrorCodePMInvalid, "invalid payment method")
	ErrPMExpired     = NewDomainError(ErrorCodePMExpired, "payment method has expired")
	ErrPMNotVerified = NewDomainError(ErrorCodePMNotVerified, "ACH payment method not verified")

	ErrCustomerNotFound = NewDomainError(ErrorCodeCustomerNotFound, "customer not found")

	ErrValidationFailed        = NewDomainError(ErrorCodeValidationFailed, "validation failed")
	ErrValidationAmountInvalid = NewDomainError(ErrorCodeValidationAmountInvalid, "invalid amount")
	ErrValidationMissingField  = NewDomainError(ErrorCodeValidationMissingField, "required field missing")

	ErrGatewayError    = NewDomainError(ErrorCodeGatewayError, "payment gateway error")
	ErrGatewayTimedOut = NewDomainError(ErrorCodeGatewayTimeout, "payment gateway timeout")
	ErrGatewayDeclined = NewDomainError(ErrorCodeGatewayDeclined, "payment declined by gateway")

	ErrIdempotencyConflict = NewDomainError(ErrorCodeIdempotencyConflict, "idempotency key conflict")

	ErrInternalError = NewDomainError(ErrorCodeInternalError, "internal server error")
	ErrDatabaseError = NewDomainError(ErrorCodeDatabaseError, "database error")
)

// Common domain errors (legacy - kept for backward compatibility)
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
