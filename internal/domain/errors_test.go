package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestDomainErrors_TransactionErrors tests all transaction-related domain errors
func TestDomainErrors_TransactionErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "transaction_not_found",
			err:      ErrTxnNotFound,
			contains: "transaction not found",
		},
		{
			name:     "transaction_cannot_be_voided",
			err:      ErrTxnCannotBeVoided,
			contains: "transaction cannot be voided",
		},
		{
			name:     "transaction_cannot_be_captured",
			err:      ErrTxnCannotBeCaptured,
			contains: "transaction cannot be captured",
		},
		{
			name:     "transaction_cannot_be_refunded",
			err:      ErrTxnCannotBeRefunded,
			contains: "transaction cannot be refunded",
		},
		{
			name:     "invalid_transaction_state",
			err:      ErrTxnInvalidState,
			contains: "transaction is in invalid state",
		},
		{
			name:     "invalid_transaction_amount",
			err:      ErrTxnInvalidAmount,
			contains: "invalid transaction amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error to be defined, got nil")
			}
			if !strings.Contains(strings.ToLower(tt.err.Error()), tt.contains) {
				t.Errorf("error message %q does not contain %q", tt.err.Error(), tt.contains)
			}
		})
	}
}

// TestDomainErrors_SubscriptionErrors tests all subscription-related domain errors
func TestDomainErrors_SubscriptionErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "subscription_not_found",
			err:      ErrSubscriptionNotFound,
			contains: "subscription not found",
		},
		{
			name:     "subscription_not_active",
			err:      ErrSubscriptionNotActive,
			contains: "subscription is not active",
		},
		{
			name:     "subscription_already_cancelled",
			err:      ErrSubscriptionAlreadyCancelled,
			contains: "subscription is already cancelled",
		},
		{
			name:     "invalid_billing_interval",
			err:      ErrSubscriptionInvalidInterval,
			contains: "invalid billing interval",
		},
		{
			name:     "max_retries_exceeded",
			err:      ErrSubscriptionMaxRetries,
			contains: "max billing retries exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error to be defined, got nil")
			}
			if !strings.Contains(strings.ToLower(tt.err.Error()), tt.contains) {
				t.Errorf("error message %q does not contain %q", tt.err.Error(), tt.contains)
			}
		})
	}
}

// TestDomainErrors_PaymentMethodErrors tests all payment method-related domain errors
func TestDomainErrors_PaymentMethodErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "payment_method_not_found",
			err:      ErrPMNotFound,
			contains: "payment method not found",
		},
		{
			name:     "payment_method_expired",
			err:      ErrPMExpired,
			contains: "payment method has expired",
		},
		{
			name:     "payment_method_not_verified",
			err:      ErrPMNotVerified,
			contains: "ach payment method not verified",
		},
		{
			name:     "payment_method_inactive",
			err:      ErrPMInactive,
			contains: "payment method is inactive",
		},
		{
			name:     "invalid_payment_method_type",
			err:      ErrPMInvalidType,
			contains: "invalid payment method type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error to be defined, got nil")
			}
			if !strings.Contains(strings.ToLower(tt.err.Error()), tt.contains) {
				t.Errorf("error message %q does not contain %q", tt.err.Error(), tt.contains)
			}
		})
	}
}

// TestDomainErrors_ChargebackErrors tests all chargeback-related domain errors
func TestDomainErrors_ChargebackErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "chargeback_not_found",
			err:      ErrChargebackNotFound,
			contains: "chargeback not found",
		},
		{
			name:     "chargeback_cannot_respond",
			err:      ErrChargebackCannotRespond,
			contains: "cannot respond to chargeback",
		},
		{
			name:     "chargeback_already_resolved",
			err:      ErrChargebackAlreadyResolved,
			contains: "chargeback is already resolved",
		},
		{
			name:     "invalid_chargeback_status",
			err:      ErrChargebackInvalidStatus,
			contains: "invalid chargeback status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error to be defined, got nil")
			}
			if !strings.Contains(strings.ToLower(tt.err.Error()), tt.contains) {
				t.Errorf("error message %q does not contain %q", tt.err.Error(), tt.contains)
			}
		})
	}
}

// TestDomainErrors_MerchantErrors tests all merchant-related domain errors
func TestDomainErrors_MerchantErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "merchant_not_found",
			err:      ErrMerchantNotFoundTyped,
			contains: "merchant not found",
		},
		{
			name:     "merchant_inactive",
			err:      ErrMerchantInactiveTyped,
			contains: "merchant is not active",
		},
		{
			name:     "merchant_already_exists",
			err:      ErrMerchantAlreadyExists,
			contains: "merchant already exists",
		},
		{
			name:     "invalid_environment",
			err:      ErrMerchantInvalidEnv,
			contains: "invalid environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error to be defined, got nil")
			}
			if !strings.Contains(strings.ToLower(tt.err.Error()), tt.contains) {
				t.Errorf("error message %q does not contain %q", tt.err.Error(), tt.contains)
			}
		})
	}
}

// TestDomainErrors_GatewayErrors tests all gateway-related domain errors
func TestDomainErrors_GatewayErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "gateway_timeout",
			err:      ErrGatewayTimedOut,
			contains: "gateway timeout",
		},
		{
			name:     "gateway_unavailable",
			err:      ErrGatewayUnavailable,
			contains: "gateway is unavailable",
		},
		{
			name:     "invalid_gateway_response",
			err:      ErrGatewayInvalidResp,
			contains: "invalid gateway response",
		},
		{
			name:     "transaction_declined",
			err:      ErrTxnDeclined,
			contains: "declined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error to be defined, got nil")
			}
			if !strings.Contains(strings.ToLower(tt.err.Error()), tt.contains) {
				t.Errorf("error message %q does not contain %q", tt.err.Error(), tt.contains)
			}
		})
	}
}

// TestDomainErrors_IdempotencyErrors tests idempotency-related domain errors
func TestDomainErrors_IdempotencyErrors(t *testing.T) {
	if ErrDuplicateIdempotencyKey == nil {
		t.Error("expected ErrDuplicateIdempotencyKey to be defined, got nil")
	}

	expected := "duplicate idempotency key"
	if !strings.Contains(strings.ToLower(ErrDuplicateIdempotencyKey.Error()), expected) {
		t.Errorf("error message %q does not contain %q", ErrDuplicateIdempotencyKey.Error(), expected)
	}
}

// TestDomainErrors_ValidationErrors tests all validation-related domain errors
func TestDomainErrors_ValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "invalid_amount",
			err:      ErrInvalidAmount,
			contains: "invalid amount",
		},
		{
			name:     "invalid_currency",
			err:      ErrInvalidCurrency,
			contains: "invalid currency",
		},
		{
			name:     "missing_required_field",
			err:      ErrValidationMissingField,
			contains: "required field missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("expected error to be defined, got nil")
			}
			if !strings.Contains(strings.ToLower(tt.err.Error()), tt.contains) {
				t.Errorf("error message %q does not contain %q", tt.err.Error(), tt.contains)
			}
		})
	}
}

// TestDomainErrors_Wrapping tests that domain errors can be wrapped and unwrapped correctly
func TestDomainErrors_Wrapping(t *testing.T) {
	tests := []struct {
		name        string
		baseErr     error
		wrapMessage string
	}{
		{
			name:        "wrap_transaction_not_found",
			baseErr:     ErrTxnNotFound,
			wrapMessage: "failed to process payment",
		},
		{
			name:        "wrap_payment_method_expired",
			baseErr:     ErrPMExpired,
			wrapMessage: "cannot charge card",
		},
		{
			name:        "wrap_gateway_timeout",
			baseErr:     ErrGatewayTimedOut,
			wrapMessage: "payment processing failed",
		},
		{
			name:        "wrap_duplicate_idempotency_key",
			baseErr:     ErrDuplicateIdempotencyKey,
			wrapMessage: "request already processed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap the error
			wrapped := fmt.Errorf("%s: %w", tt.wrapMessage, tt.baseErr)

			// Verify the wrapped error contains the wrap message
			if !strings.Contains(wrapped.Error(), tt.wrapMessage) {
				t.Errorf("wrapped error %q does not contain wrap message %q", wrapped.Error(), tt.wrapMessage)
			}

			// Verify the wrapped error can be unwrapped to the original
			if !errors.Is(wrapped, tt.baseErr) {
				t.Errorf("errors.Is failed: wrapped error does not match base error %v", tt.baseErr)
			}
		})
	}
}

// TestDomainErrors_IsComparison tests that errors.Is() works correctly for all domain errors
func TestDomainErrors_IsComparison(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		target    error
		shouldBe  bool
		shouldNot error
	}{
		{
			name:      "transaction_not_found_matches_itself",
			err:       ErrTxnNotFound,
			target:    ErrTxnNotFound,
			shouldBe:  true,
			shouldNot: ErrSubscriptionNotFound,
		},
		{
			name:      "wrapped_transaction_not_found_matches",
			err:       fmt.Errorf("context: %w", ErrTxnNotFound),
			target:    ErrTxnNotFound,
			shouldBe:  true,
			shouldNot: ErrPMNotFound,
		},
		{
			name:      "gateway_timeout_matches_itself",
			err:       ErrGatewayTimedOut,
			target:    ErrGatewayTimedOut,
			shouldBe:  true,
			shouldNot: ErrGatewayUnavailable,
		},
		{
			name:      "duplicate_idempotency_key_matches_itself",
			err:       ErrDuplicateIdempotencyKey,
			target:    ErrDuplicateIdempotencyKey,
			shouldBe:  true,
			shouldNot: ErrInvalidAmount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test positive match
			if tt.shouldBe && !errors.Is(tt.err, tt.target) {
				t.Errorf("errors.Is(%v, %v) = false, want true", tt.err, tt.target)
			}

			// Test negative match
			if errors.Is(tt.err, tt.shouldNot) {
				t.Errorf("errors.Is(%v, %v) = true, want false", tt.err, tt.shouldNot)
			}
		})
	}
}

// TestDomainErrors_UniqueErrorCodes tests that each error has a unique error code
func TestDomainErrors_UniqueErrorCodes(t *testing.T) {
	allErrors := []*DomainError{
		// Transaction errors
		ErrTxnNotFound,
		ErrTxnCannotBeVoided,
		ErrTxnCannotBeCaptured,
		ErrTxnCannotBeRefunded,
		ErrTxnInvalidState,
		ErrTxnInvalidAmount,
		ErrTxnDeclined, // Note: ErrGatewayDeclined uses same code
		// Subscription errors
		ErrSubscriptionNotFound,
		ErrSubscriptionNotActive,
		ErrSubscriptionAlreadyCancelled,
		ErrSubscriptionInvalidInterval,
		ErrSubscriptionMaxRetries,
		// Payment method errors
		ErrPMNotFound,
		ErrPMExpired,
		ErrPMNotVerified,
		ErrPMInactive,
		ErrPMInvalidType,
		ErrPMNotBelongToCustomer,
		// Chargeback errors
		ErrChargebackNotFound,
		ErrChargebackCannotRespond,
		ErrChargebackAlreadyResolved,
		ErrChargebackInvalidStatus,
		// Merchant errors
		ErrMerchantNotFoundTyped,
		ErrMerchantInactiveTyped,
		ErrMerchantAlreadyExists,
		ErrMerchantInvalidEnv,
		// Gateway errors
		ErrGatewayTimedOut,
		ErrGatewayUnavailable,
		ErrGatewayInvalidResp,
		ErrGatewayError,
		// Note: ErrGatewayDeclined excluded - same code as ErrTxnDeclined
		// Idempotency errors
		ErrDuplicateIdempotencyKey,
		// Validation errors
		ErrInvalidAmount,
		ErrInvalidCurrency,
		ErrValidationMissingField,
		ErrValidationFailed,
		ErrValidationInvalidUUID,
		// Internal errors
		ErrInternalError,
		ErrDatabaseError,
	}

	codes := make(map[ErrorCode]*DomainError)
	for _, err := range allErrors {
		if existing, found := codes[err.Code]; found {
			t.Errorf("duplicate error code %q found in both %v and %v", err.Code, existing.Message, err.Message)
		}
		codes[err.Code] = err
	}
}

// TestDomainErrors_NotNil tests that all domain errors are defined and not nil
func TestDomainErrors_NotNil(t *testing.T) {
	tests := []struct {
		name string
		err  *DomainError
	}{
		// Transaction errors
		{"ErrTxnNotFound", ErrTxnNotFound},
		{"ErrTxnCannotBeVoided", ErrTxnCannotBeVoided},
		{"ErrTxnCannotBeCaptured", ErrTxnCannotBeCaptured},
		{"ErrTxnCannotBeRefunded", ErrTxnCannotBeRefunded},
		{"ErrTxnInvalidState", ErrTxnInvalidState},
		{"ErrTxnInvalidAmount", ErrTxnInvalidAmount},
		// Subscription errors
		{"ErrSubscriptionNotFound", ErrSubscriptionNotFound},
		{"ErrSubscriptionNotActive", ErrSubscriptionNotActive},
		{"ErrSubscriptionAlreadyCancelled", ErrSubscriptionAlreadyCancelled},
		{"ErrSubscriptionInvalidInterval", ErrSubscriptionInvalidInterval},
		{"ErrSubscriptionMaxRetries", ErrSubscriptionMaxRetries},
		// Payment method errors
		{"ErrPMNotFound", ErrPMNotFound},
		{"ErrPMExpired", ErrPMExpired},
		{"ErrPMNotVerified", ErrPMNotVerified},
		{"ErrPMInactive", ErrPMInactive},
		{"ErrPMInvalidType", ErrPMInvalidType},
		// Chargeback errors
		{"ErrChargebackNotFound", ErrChargebackNotFound},
		{"ErrChargebackCannotRespond", ErrChargebackCannotRespond},
		{"ErrChargebackAlreadyResolved", ErrChargebackAlreadyResolved},
		{"ErrChargebackInvalidStatus", ErrChargebackInvalidStatus},
		// Merchant errors
		{"ErrMerchantNotFoundTyped", ErrMerchantNotFoundTyped},
		{"ErrMerchantInactiveTyped", ErrMerchantInactiveTyped},
		{"ErrMerchantAlreadyExists", ErrMerchantAlreadyExists},
		{"ErrMerchantInvalidEnv", ErrMerchantInvalidEnv},
		// Gateway errors
		{"ErrGatewayTimedOut", ErrGatewayTimedOut},
		{"ErrGatewayUnavailable", ErrGatewayUnavailable},
		{"ErrGatewayInvalidResp", ErrGatewayInvalidResp},
		{"ErrTxnDeclined", ErrTxnDeclined},
		// Idempotency errors
		{"ErrDuplicateIdempotencyKey", ErrDuplicateIdempotencyKey},
		// Validation errors
		{"ErrInvalidAmount", ErrInvalidAmount},
		{"ErrInvalidCurrency", ErrInvalidCurrency},
		{"ErrValidationMissingField", ErrValidationMissingField},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s is nil, expected to be defined", tt.name)
			}
		})
	}
}

// TestDomainErrors_SwitchCase tests that domain errors can be used in switch/case statements
func TestDomainErrors_SwitchCase(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedType string
	}{
		{
			name:         "transaction_error_in_switch",
			err:          ErrTxnNotFound,
			expectedType: "transaction",
		},
		{
			name:         "subscription_error_in_switch",
			err:          ErrSubscriptionNotActive,
			expectedType: "subscription",
		},
		{
			name:         "payment_method_error_in_switch",
			err:          ErrPMExpired,
			expectedType: "payment_method",
		},
		{
			name:         "gateway_error_in_switch",
			err:          ErrGatewayTimedOut,
			expectedType: "gateway",
		},
		{
			name:         "validation_error_in_switch",
			err:          ErrInvalidAmount,
			expectedType: "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errorType string

			// Use the error in a switch/case statement
			switch {
			case errors.Is(tt.err, ErrTxnNotFound),
				errors.Is(tt.err, ErrTxnCannotBeVoided),
				errors.Is(tt.err, ErrTxnCannotBeCaptured),
				errors.Is(tt.err, ErrTxnCannotBeRefunded):
				errorType = "transaction"
			case errors.Is(tt.err, ErrSubscriptionNotFound),
				errors.Is(tt.err, ErrSubscriptionNotActive),
				errors.Is(tt.err, ErrSubscriptionAlreadyCancelled):
				errorType = "subscription"
			case errors.Is(tt.err, ErrPMNotFound),
				errors.Is(tt.err, ErrPMExpired),
				errors.Is(tt.err, ErrPMNotVerified):
				errorType = "payment_method"
			case errors.Is(tt.err, ErrGatewayTimedOut),
				errors.Is(tt.err, ErrGatewayUnavailable),
				errors.Is(tt.err, ErrTxnDeclined):
				errorType = "gateway"
			case errors.Is(tt.err, ErrInvalidAmount),
				errors.Is(tt.err, ErrInvalidCurrency),
				errors.Is(tt.err, ErrValidationMissingField):
				errorType = "validation"
			default:
				errorType = "unknown"
			}

			if errorType != tt.expectedType {
				t.Errorf("switch/case returned %q, want %q", errorType, tt.expectedType)
			}
		})
	}
}

// TestDomainErrors_MessageDescriptiveness tests that error messages are descriptive
func TestDomainErrors_MessageDescriptiveness(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		minLength      int
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:        "transaction_not_found_is_descriptive",
			err:         ErrTxnNotFound,
			minLength:   10,
			mustContain: []string{"transaction", "not found"},
		},
		{
			name:        "payment_method_not_verified_is_descriptive",
			err:         ErrPMNotVerified,
			minLength:   15,
			mustContain: []string{"ach", "payment method", "not verified"},
		},
		{
			name:        "chargeback_cannot_respond_is_descriptive",
			err:         ErrChargebackCannotRespond,
			minLength:   20,
			mustContain: []string{"cannot respond", "chargeback"},
		},
		{
			name:        "gateway_timeout_is_descriptive",
			err:         ErrGatewayTimedOut,
			minLength:   10,
			mustContain: []string{"gateway", "timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			msgLower := strings.ToLower(msg)

			// Check minimum length
			if tt.minLength > 0 && len(msg) < tt.minLength {
				t.Errorf("error message %q is too short (length %d), expected at least %d characters",
					msg, len(msg), tt.minLength)
			}

			// Check must contain
			for _, required := range tt.mustContain {
				if !strings.Contains(msgLower, strings.ToLower(required)) {
					t.Errorf("error message %q does not contain required text %q", msg, required)
				}
			}

			// Check must not contain
			for _, forbidden := range tt.mustNotContain {
				if strings.Contains(msgLower, strings.ToLower(forbidden)) {
					t.Errorf("error message %q contains forbidden text %q", msg, forbidden)
				}
			}
		})
	}
}

// TestDomainError_WithDetail tests that WithDetail adds details correctly
func TestDomainError_WithDetail(t *testing.T) {
	err := ErrTxnNotFound.WithDetail("transaction_id", "abc123")

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Details == nil {
		t.Fatal("expected details map to be initialized")
	}

	if err.Details["transaction_id"] != "abc123" {
		t.Errorf("expected detail transaction_id to be 'abc123', got %v", err.Details["transaction_id"])
	}
}

// TestDomainError_ErrorFormat tests the Error() output format
func TestDomainError_ErrorFormat(t *testing.T) {
	err := NewDomainError(ErrorCodeTxnNotFound, "test message")

	// Should include code and message
	expected := "TXN_NOT_FOUND: test message"
	if err.Error() != expected {
		t.Errorf("expected error string %q, got %q", expected, err.Error())
	}
}
