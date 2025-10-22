package errors

import (
	"fmt"
)

// ErrorCategory represents the category of error for handling
type ErrorCategory string

const (
	CategoryApproved          ErrorCategory = "approved"
	CategoryDeclined          ErrorCategory = "declined"
	CategoryInsufficientFunds ErrorCategory = "insufficient_funds"
	CategoryInvalidCard       ErrorCategory = "invalid_card"
	CategoryExpiredCard       ErrorCategory = "expired_card"
	CategoryFraud             ErrorCategory = "fraud"
	CategorySystemError       ErrorCategory = "system_error"
	CategoryNetworkError      ErrorCategory = "network_error"
	CategoryInvalidRequest    ErrorCategory = "invalid_request"
)

// PaymentError represents a payment processing error with detailed context
type PaymentError struct {
	Code           string
	Message        string
	GatewayMessage string
	IsRetriable    bool
	Category       ErrorCategory
	Details        map[string]interface{}
}

func (e *PaymentError) Error() string {
	if e.GatewayMessage != "" {
		return fmt.Sprintf("%s: %s (gateway: %s)", e.Code, e.Message, e.GatewayMessage)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewPaymentError creates a new payment error
func NewPaymentError(code, message string, category ErrorCategory, retriable bool) *PaymentError {
	return &PaymentError{
		Code:        code,
		Message:     message,
		Category:    category,
		IsRetriable: retriable,
		Details:     make(map[string]interface{}),
	}
}

// ValidationError represents input validation errors
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
