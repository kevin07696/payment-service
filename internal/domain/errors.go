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
	ErrorCodeTxnCannotBeVoided   ErrorCode = "TXN_CANNOT_BE_VOIDED"
	ErrorCodeTxnCannotBeCaptured ErrorCode = "TXN_CANNOT_BE_CAPTURED"
	ErrorCodeTxnCannotBeRefunded ErrorCode = "TXN_CANNOT_BE_REFUNDED"
	ErrorCodeTxnInvalidAmount    ErrorCode = "TXN_INVALID_AMOUNT"

	// Payment Method Errors (PM_*)
	ErrorCodePMNotFound        ErrorCode = "PM_NOT_FOUND"
	ErrorCodePMRequired        ErrorCode = "PM_REQUIRED"
	ErrorCodePMInvalid         ErrorCode = "PM_INVALID"
	ErrorCodePMExpired         ErrorCode = "PM_EXPIRED"
	ErrorCodePMNotVerified     ErrorCode = "PM_NOT_VERIFIED"
	ErrorCodePMInactive        ErrorCode = "PM_INACTIVE"
	ErrorCodePMInvalidType     ErrorCode = "PM_INVALID_TYPE"
	ErrorCodePMInsufficientACH ErrorCode = "PM_INSUFFICIENT_ACH_VERIFICATIONS"

	// Customer Errors (CUSTOMER_*)
	ErrorCodeCustomerNotFound ErrorCode = "CUSTOMER_NOT_FOUND"

	// Subscription Errors (SUBSCRIPTION_*)
	ErrorCodeSubscriptionNotFound         ErrorCode = "SUBSCRIPTION_NOT_FOUND"
	ErrorCodeSubscriptionNotActive        ErrorCode = "SUBSCRIPTION_NOT_ACTIVE"
	ErrorCodeSubscriptionAlreadyCancelled ErrorCode = "SUBSCRIPTION_ALREADY_CANCELLED"
	ErrorCodeSubscriptionInvalidInterval  ErrorCode = "SUBSCRIPTION_INVALID_INTERVAL"
	ErrorCodeSubscriptionMaxRetries       ErrorCode = "SUBSCRIPTION_MAX_RETRIES_EXCEEDED"

	// Chargeback Errors (CHARGEBACK_*)
	ErrorCodeChargebackNotFound        ErrorCode = "CHARGEBACK_NOT_FOUND"
	ErrorCodeChargebackCannotRespond   ErrorCode = "CHARGEBACK_CANNOT_RESPOND"
	ErrorCodeChargebackAlreadyResolved ErrorCode = "CHARGEBACK_ALREADY_RESOLVED"
	ErrorCodeChargebackInvalidStatus   ErrorCode = "CHARGEBACK_INVALID_STATUS"

	// Merchant Errors (additional)
	ErrorCodeMerchantAlreadyExists ErrorCode = "MERCHANT_ALREADY_EXISTS"
	ErrorCodeMerchantInvalidEnv    ErrorCode = "MERCHANT_INVALID_ENVIRONMENT"

	// Validation Errors (VALIDATION_*)
	ErrorCodeValidationFailed          ErrorCode = "VALIDATION_FAILED"
	ErrorCodeValidationAmountInvalid   ErrorCode = "VALIDATION_AMOUNT_INVALID"
	ErrorCodeValidationMissingField    ErrorCode = "VALIDATION_MISSING_FIELD"
	ErrorCodeValidationInvalidAmount   ErrorCode = "VALIDATION_INVALID_AMOUNT"
	ErrorCodeValidationInvalidCurrency ErrorCode = "VALIDATION_INVALID_CURRENCY"

	// Payment Gateway Errors (GATEWAY_*)
	ErrorCodeGatewayError       ErrorCode = "GATEWAY_ERROR"
	ErrorCodeGatewayTimeout     ErrorCode = "GATEWAY_TIMEOUT"
	ErrorCodeGatewayDeclined    ErrorCode = "GATEWAY_DECLINED"
	ErrorCodeGatewayUnavailable ErrorCode = "GATEWAY_UNAVAILABLE"
	ErrorCodeGatewayInvalidResp ErrorCode = "GATEWAY_INVALID_RESPONSE"

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
	ErrTxnCannotBeVoided   = NewDomainError(ErrorCodeTxnCannotBeVoided, "transaction cannot be voided")
	ErrTxnCannotBeCaptured = NewDomainError(ErrorCodeTxnCannotBeCaptured, "transaction cannot be captured")
	ErrTxnCannotBeRefunded = NewDomainError(ErrorCodeTxnCannotBeRefunded, "transaction cannot be refunded")
	ErrTxnInvalidAmount    = NewDomainError(ErrorCodeTxnInvalidAmount, "invalid transaction amount")

	ErrPMNotFound    = NewDomainError(ErrorCodePMNotFound, "payment method not found")
	ErrPMRequired    = NewDomainError(ErrorCodePMRequired, "payment method required")
	ErrPMInvalid     = NewDomainError(ErrorCodePMInvalid, "invalid payment method")
	ErrPMExpired     = NewDomainError(ErrorCodePMExpired, "payment method has expired")
	ErrPMNotVerified = NewDomainError(ErrorCodePMNotVerified, "ACH payment method not verified")
	ErrPMInactive    = NewDomainError(ErrorCodePMInactive, "payment method is inactive")
	ErrPMInvalidType = NewDomainError(ErrorCodePMInvalidType, "invalid payment method type")

	ErrSubscriptionNotFound         = NewDomainError(ErrorCodeSubscriptionNotFound, "subscription not found")
	ErrSubscriptionNotActive        = NewDomainError(ErrorCodeSubscriptionNotActive, "subscription is not active")
	ErrSubscriptionAlreadyCancelled = NewDomainError(ErrorCodeSubscriptionAlreadyCancelled, "subscription is already cancelled")
	ErrSubscriptionInvalidInterval  = NewDomainError(ErrorCodeSubscriptionInvalidInterval, "invalid billing interval")
	ErrSubscriptionMaxRetries       = NewDomainError(ErrorCodeSubscriptionMaxRetries, "max billing retries exceeded")

	ErrChargebackNotFound        = NewDomainError(ErrorCodeChargebackNotFound, "chargeback not found")
	ErrChargebackCannotRespond   = NewDomainError(ErrorCodeChargebackCannotRespond, "cannot respond to chargeback (deadline passed or already responded)")
	ErrChargebackAlreadyResolved = NewDomainError(ErrorCodeChargebackAlreadyResolved, "chargeback is already resolved")
	ErrChargebackInvalidStatus   = NewDomainError(ErrorCodeChargebackInvalidStatus, "invalid chargeback status")

	ErrMerchantAlreadyExists = NewDomainError(ErrorCodeMerchantAlreadyExists, "merchant already exists")
	ErrMerchantInvalidEnv    = NewDomainError(ErrorCodeMerchantInvalidEnv, "invalid environment")

	ErrGatewayUnavailable = NewDomainError(ErrorCodeGatewayUnavailable, "gateway is unavailable")
	ErrGatewayInvalidResp = NewDomainError(ErrorCodeGatewayInvalidResp, "invalid gateway response")
	ErrTxnDeclined        = NewDomainError(ErrorCodeGatewayDeclined, "transaction was declined by gateway")

	ErrDuplicateIdempotencyKey = NewDomainError(ErrorCodeIdempotencyConflict, "duplicate idempotency key")

	ErrInvalidAmount   = NewDomainError(ErrorCodeValidationInvalidAmount, "invalid amount")
	ErrInvalidCurrency = NewDomainError(ErrorCodeValidationInvalidCurrency, "invalid currency")

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

// Legacy sentinel errors - DEPRECATED: Use DomainError instances above instead
// These are kept for backward compatibility but should not be used in new code
//
// Migration guide:
//
//	ErrTransactionNotFound         -> ErrTxnNotFound
//	ErrTransactionCannotBeVoided   -> ErrTxnCannotBeVoided
//	ErrTransactionCannotBeCaptured -> ErrTxnCannotBeCaptured
//	ErrTransactionCannotBeRefunded -> ErrTxnCannotBeRefunded
//	ErrInvalidTransactionStatus    -> ErrTxnInvalidState
//	ErrInvalidTransactionAmount    -> ErrTxnInvalidAmount
//	ErrSubscriptionNotFound         -> (use DomainError above)
//	ErrSubscriptionNotActive        -> (use DomainError above)
//	ErrSubscriptionAlreadyCancelled -> (use DomainError above)
//	ErrInvalidBillingInterval       -> ErrSubscriptionInvalidInterval
//	ErrMaxRetriesExceeded           -> ErrSubscriptionMaxRetries
//	ErrPaymentMethodNotFound        -> ErrPMNotFound
//	ErrPaymentMethodExpired         -> ErrPMExpired
//	ErrPaymentMethodNotVerified     -> ErrPMNotVerified
//	ErrPaymentMethodInactive        -> ErrPMInactive
//	ErrInvalidPaymentMethodType     -> ErrPMInvalidType
//	ErrChargebackNotFound           -> (use DomainError above)
//	ErrChargebackCannotRespond      -> (use DomainError above)
//	ErrChargebackAlreadyResolved    -> (use DomainError above)
//	ErrInvalidChargebackStatus      -> (use DomainError above)
//	ErrMerchantNotFound             -> ErrMerchantNotFoundTyped
//	ErrMerchantInactive             -> ErrMerchantInactiveTyped
//	ErrMerchantAlreadyExists        -> (use DomainError above)
//	ErrInvalidEnvironment           -> ErrMerchantInvalidEnv
//	ErrGatewayTimeout               -> ErrGatewayTimedOut
//	ErrGatewayUnavailable           -> (use DomainError above)
//	ErrInvalidGatewayResponse       -> ErrGatewayInvalidResp
//	ErrTransactionDeclined          -> ErrTxnDeclined
//	ErrDuplicateIdempotencyKey      -> (use DomainError above)
//	ErrInvalidAmount                -> (use DomainError above)
//	ErrInvalidCurrency              -> (use DomainError above)
//	ErrMissingRequiredField         -> ErrValidationMissingField
//
// TODO: Remove these after migrating all usages to DomainError
var (
	// Transaction errors - DEPRECATED
	ErrTransactionNotFound         = errors.New("transaction not found")
	ErrTransactionCannotBeVoided   = errors.New("transaction cannot be voided")
	ErrTransactionCannotBeCaptured = errors.New("transaction cannot be captured")
	ErrTransactionCannotBeRefunded = errors.New("transaction cannot be refunded")
	ErrInvalidTransactionStatus    = errors.New("invalid transaction status")
	ErrInvalidTransactionAmount    = errors.New("invalid transaction amount")

	// Subscription errors - DEPRECATED
	// Use ErrSubscriptionNotFound, ErrSubscriptionNotActive, etc. instead
	ErrInvalidBillingInterval = errors.New("invalid billing interval")
	ErrMaxRetriesExceeded     = errors.New("max billing retries exceeded")

	// Payment method errors - DEPRECATED
	ErrPaymentMethodNotFound    = errors.New("payment method not found")
	ErrPaymentMethodExpired     = errors.New("payment method is expired")
	ErrPaymentMethodNotVerified = errors.New("ACH payment method is not verified")
	ErrPaymentMethodInactive    = errors.New("payment method is inactive")
	ErrInvalidPaymentMethodType = errors.New("invalid payment method type")

	// Chargeback errors - DEPRECATED
	// Chargebacks now use DomainError instances (see lines 222-225)
	ErrInvalidChargebackStatus = errors.New("invalid chargeback status")

	// Merchant errors - DEPRECATED
	ErrMerchantNotFound   = errors.New("merchant not found")
	ErrMerchantInactive   = errors.New("merchant is inactive")
	ErrInvalidEnvironment = errors.New("invalid environment")

	// Gateway errors - DEPRECATED
	ErrGatewayTimeout         = errors.New("gateway request timed out")
	ErrInvalidGatewayResponse = errors.New("invalid gateway response")
	ErrTransactionDeclined    = errors.New("transaction was declined by gateway")

	// Validation errors - DEPRECATED
	ErrMissingRequiredField = errors.New("missing required field")
)
