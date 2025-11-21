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
			err:      ErrTransactionNotFound,
			contains: "transaction not found",
		},
		{
			name:     "transaction_cannot_be_voided",
			err:      ErrTransactionCannotBeVoided,
			contains: "transaction cannot be voided",
		},
		{
			name:     "transaction_cannot_be_captured",
			err:      ErrTransactionCannotBeCaptured,
			contains: "transaction cannot be captured",
		},
		{
			name:     "transaction_cannot_be_refunded",
			err:      ErrTransactionCannotBeRefunded,
			contains: "transaction cannot be refunded",
		},
		{
			name:     "invalid_transaction_status",
			err:      ErrInvalidTransactionStatus,
			contains: "invalid transaction status",
		},
		{
			name:     "invalid_transaction_amount",
			err:      ErrInvalidTransactionAmount,
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
			err:      ErrInvalidBillingInterval,
			contains: "invalid billing interval",
		},
		{
			name:     "max_retries_exceeded",
			err:      ErrMaxRetriesExceeded,
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
			err:      ErrPaymentMethodNotFound,
			contains: "payment method not found",
		},
		{
			name:     "payment_method_expired",
			err:      ErrPaymentMethodExpired,
			contains: "payment method is expired",
		},
		{
			name:     "payment_method_not_verified",
			err:      ErrPaymentMethodNotVerified,
			contains: "ach payment method is not verified",
		},
		{
			name:     "payment_method_inactive",
			err:      ErrPaymentMethodInactive,
			contains: "payment method is inactive",
		},
		{
			name:     "invalid_payment_method_type",
			err:      ErrInvalidPaymentMethodType,
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
			err:      ErrInvalidChargebackStatus,
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
			err:      ErrMerchantNotFound,
			contains: "merchant not found",
		},
		{
			name:     "merchant_inactive",
			err:      ErrMerchantInactive,
			contains: "merchant is inactive",
		},
		{
			name:     "merchant_already_exists",
			err:      ErrMerchantAlreadyExists,
			contains: "merchant already exists",
		},
		{
			name:     "invalid_environment",
			err:      ErrInvalidEnvironment,
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
			err:      ErrGatewayTimeout,
			contains: "gateway request timed out",
		},
		{
			name:     "gateway_unavailable",
			err:      ErrGatewayUnavailable,
			contains: "gateway is unavailable",
		},
		{
			name:     "invalid_gateway_response",
			err:      ErrInvalidGatewayResponse,
			contains: "invalid gateway response",
		},
		{
			name:     "transaction_declined",
			err:      ErrTransactionDeclined,
			contains: "transaction was declined by gateway",
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
			err:      ErrMissingRequiredField,
			contains: "missing required field",
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
			baseErr:     ErrTransactionNotFound,
			wrapMessage: "failed to process payment",
		},
		{
			name:        "wrap_payment_method_expired",
			baseErr:     ErrPaymentMethodExpired,
			wrapMessage: "cannot charge card",
		},
		{
			name:        "wrap_gateway_timeout",
			baseErr:     ErrGatewayTimeout,
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
			err:       ErrTransactionNotFound,
			target:    ErrTransactionNotFound,
			shouldBe:  true,
			shouldNot: ErrSubscriptionNotFound,
		},
		{
			name:      "wrapped_transaction_not_found_matches",
			err:       fmt.Errorf("context: %w", ErrTransactionNotFound),
			target:    ErrTransactionNotFound,
			shouldBe:  true,
			shouldNot: ErrPaymentMethodNotFound,
		},
		{
			name:      "gateway_timeout_matches_itself",
			err:       ErrGatewayTimeout,
			target:    ErrGatewayTimeout,
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

// TestDomainErrors_UniqueMessages tests that each error has a unique message
func TestDomainErrors_UniqueMessages(t *testing.T) {
	allErrors := []error{
		// Transaction errors
		ErrTransactionNotFound,
		ErrTransactionCannotBeVoided,
		ErrTransactionCannotBeCaptured,
		ErrTransactionCannotBeRefunded,
		ErrInvalidTransactionStatus,
		ErrInvalidTransactionAmount,
		// Subscription errors
		ErrSubscriptionNotFound,
		ErrSubscriptionNotActive,
		ErrSubscriptionAlreadyCancelled,
		ErrInvalidBillingInterval,
		ErrMaxRetriesExceeded,
		// Payment method errors
		ErrPaymentMethodNotFound,
		ErrPaymentMethodExpired,
		ErrPaymentMethodNotVerified,
		ErrPaymentMethodInactive,
		ErrInvalidPaymentMethodType,
		// Chargeback errors
		ErrChargebackNotFound,
		ErrChargebackCannotRespond,
		ErrChargebackAlreadyResolved,
		ErrInvalidChargebackStatus,
		// Merchant errors
		ErrMerchantNotFound,
		ErrMerchantInactive,
		ErrMerchantAlreadyExists,
		ErrInvalidEnvironment,
		// Gateway errors
		ErrGatewayTimeout,
		ErrGatewayUnavailable,
		ErrInvalidGatewayResponse,
		ErrTransactionDeclined,
		// Idempotency errors
		ErrDuplicateIdempotencyKey,
		// Validation errors
		ErrInvalidAmount,
		ErrInvalidCurrency,
		ErrMissingRequiredField,
	}

	messages := make(map[string]error)
	for _, err := range allErrors {
		msg := err.Error()
		if existing, found := messages[msg]; found {
			t.Errorf("duplicate error message %q found in both %v and %v", msg, existing, err)
		}
		messages[msg] = err
	}

	// Verify we have the expected number of unique errors
	expectedCount := 32
	if len(messages) != expectedCount {
		t.Errorf("expected %d unique error messages, got %d", expectedCount, len(messages))
	}
}

// TestDomainErrors_NotNil tests that all domain errors are defined and not nil
func TestDomainErrors_NotNil(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		// Transaction errors
		{"ErrTransactionNotFound", ErrTransactionNotFound},
		{"ErrTransactionCannotBeVoided", ErrTransactionCannotBeVoided},
		{"ErrTransactionCannotBeCaptured", ErrTransactionCannotBeCaptured},
		{"ErrTransactionCannotBeRefunded", ErrTransactionCannotBeRefunded},
		{"ErrInvalidTransactionStatus", ErrInvalidTransactionStatus},
		{"ErrInvalidTransactionAmount", ErrInvalidTransactionAmount},
		// Subscription errors
		{"ErrSubscriptionNotFound", ErrSubscriptionNotFound},
		{"ErrSubscriptionNotActive", ErrSubscriptionNotActive},
		{"ErrSubscriptionAlreadyCancelled", ErrSubscriptionAlreadyCancelled},
		{"ErrInvalidBillingInterval", ErrInvalidBillingInterval},
		{"ErrMaxRetriesExceeded", ErrMaxRetriesExceeded},
		// Payment method errors
		{"ErrPaymentMethodNotFound", ErrPaymentMethodNotFound},
		{"ErrPaymentMethodExpired", ErrPaymentMethodExpired},
		{"ErrPaymentMethodNotVerified", ErrPaymentMethodNotVerified},
		{"ErrPaymentMethodInactive", ErrPaymentMethodInactive},
		{"ErrInvalidPaymentMethodType", ErrInvalidPaymentMethodType},
		// Chargeback errors
		{"ErrChargebackNotFound", ErrChargebackNotFound},
		{"ErrChargebackCannotRespond", ErrChargebackCannotRespond},
		{"ErrChargebackAlreadyResolved", ErrChargebackAlreadyResolved},
		{"ErrInvalidChargebackStatus", ErrInvalidChargebackStatus},
		// Merchant errors
		{"ErrMerchantNotFound", ErrMerchantNotFound},
		{"ErrMerchantInactive", ErrMerchantInactive},
		{"ErrMerchantAlreadyExists", ErrMerchantAlreadyExists},
		{"ErrInvalidEnvironment", ErrInvalidEnvironment},
		// Gateway errors
		{"ErrGatewayTimeout", ErrGatewayTimeout},
		{"ErrGatewayUnavailable", ErrGatewayUnavailable},
		{"ErrInvalidGatewayResponse", ErrInvalidGatewayResponse},
		{"ErrTransactionDeclined", ErrTransactionDeclined},
		// Idempotency errors
		{"ErrDuplicateIdempotencyKey", ErrDuplicateIdempotencyKey},
		// Validation errors
		{"ErrInvalidAmount", ErrInvalidAmount},
		{"ErrInvalidCurrency", ErrInvalidCurrency},
		{"ErrMissingRequiredField", ErrMissingRequiredField},
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
			err:          ErrTransactionNotFound,
			expectedType: "transaction",
		},
		{
			name:         "subscription_error_in_switch",
			err:          ErrSubscriptionNotActive,
			expectedType: "subscription",
		},
		{
			name:         "payment_method_error_in_switch",
			err:          ErrPaymentMethodExpired,
			expectedType: "payment_method",
		},
		{
			name:         "gateway_error_in_switch",
			err:          ErrGatewayTimeout,
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
			case errors.Is(tt.err, ErrTransactionNotFound),
				errors.Is(tt.err, ErrTransactionCannotBeVoided),
				errors.Is(tt.err, ErrTransactionCannotBeCaptured),
				errors.Is(tt.err, ErrTransactionCannotBeRefunded):
				errorType = "transaction"
			case errors.Is(tt.err, ErrSubscriptionNotFound),
				errors.Is(tt.err, ErrSubscriptionNotActive),
				errors.Is(tt.err, ErrSubscriptionAlreadyCancelled):
				errorType = "subscription"
			case errors.Is(tt.err, ErrPaymentMethodNotFound),
				errors.Is(tt.err, ErrPaymentMethodExpired),
				errors.Is(tt.err, ErrPaymentMethodNotVerified):
				errorType = "payment_method"
			case errors.Is(tt.err, ErrGatewayTimeout),
				errors.Is(tt.err, ErrGatewayUnavailable),
				errors.Is(tt.err, ErrTransactionDeclined):
				errorType = "gateway"
			case errors.Is(tt.err, ErrInvalidAmount),
				errors.Is(tt.err, ErrInvalidCurrency),
				errors.Is(tt.err, ErrMissingRequiredField):
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
			err:         ErrTransactionNotFound,
			minLength:   10,
			mustContain: []string{"transaction", "not found"},
		},
		{
			name:        "payment_method_not_verified_is_descriptive",
			err:         ErrPaymentMethodNotVerified,
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
			err:         ErrGatewayTimeout,
			minLength:   15,
			mustContain: []string{"gateway", "timed out"},
		},
		{
			name:           "errors_not_generic",
			err:            ErrTransactionNotFound,
			mustNotContain: []string{"error", "failed", "oops"},
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
