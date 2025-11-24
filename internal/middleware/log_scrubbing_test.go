package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScrubString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Credit card number without separators",
			input:    "PAN: 4111111111111111",
			expected: "PAN: [REDACTED]",
		},
		{
			name:     "SSN with dashes",
			input:    "SSN: 123-45-6789",
			expected: "SSN: [REDACTED]",
		},
		{
			name:     "BRIC token",
			input:    "Token: bric_epx_abc123def456",
			expected: "Token: [REDACTED]",
		},
		{
			name:     "JWT token",
			input:    "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected: "Bearer [REDACTED]",
		},
		{
			name:     "Stripe secret key",
			input:    "Key: sk_test_51H5N0zK6I7lZP3U5Z",
			expected: "Key: [REDACTED]",
		},
		{
			name:     "Multiple sensitive values",
			input:    "Card 4111111111111111 for customer cust_123",
			expected: "Card [REDACTED] for customer cust_123",
		},
		{
			name:     "No sensitive data",
			input:    "Processing payment for merchant ABC-123",
			expected: "Processing payment for merchant ABC-123",
		},
		{
			name:     "Last 4 digits only (should not scrub)",
			input:    "Card ending in 1111",
			expected: "Card ending in 1111",
		},
		{
			name:     "Order ID should not be scrubbed",
			input:    "Order ID: ORD-4111-1111-1111-1111",
			expected: "Order ID: ORD-4111-1111-1111-1111",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ScrubString(tt.input)
			assert.Equal(t, tt.expected, result, "Scrubbing failed for: %s", tt.name)
		})
	}
}

func TestScrubField(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		shouldScrub bool
	}{
		// Payment card fields
		{"card_number", "card_number", true},
		{"CardNumber", "CardNumber", true},
		{"pan", "pan", true},
		{"cvv", "cvv", true},
		{"cvv2", "cvv2", true},
		{"cvc", "cvc", true},

		// ACH/Bank fields
		{"account_number", "account_number", true},
		{"routing_number", "routing_number", true},
		{"bank_account", "bank_account", true},

		// Authentication
		{"password", "password", true},
		{"secret", "secret", true},
		{"mac_secret", "mac_secret", true},
		{"api_key", "api_key", true},
		{"access_token", "access_token", true},

		// EPX-specific
		{"bric_token", "bric_token", true},
		{"epx_mac", "epx_mac", true},

		// Personal information
		{"ssn", "ssn", true},
		{"social_security", "social_security", true},

		// Safe fields
		{"merchant_id", "merchant_id", false},
		{"customer_id", "customer_id", false},
		{"transaction_id", "transaction_id", false},
		{"amount_cents", "amount_cents", false},
		{"currency", "currency", false},
		{"last_4", "last_4", false},
		{"card_brand", "card_brand", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ScrubField(tt.fieldName)
			assert.Equal(t, tt.shouldScrub, result, "Field scrubbing check failed for: %s", tt.fieldName)
		})
	}
}

func BenchmarkScrubString(b *testing.B) {
	input := "Processing payment for card 4111-1111-1111-1111 with CVV 123, amount $100.00"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScrubString(input)
	}
}

func BenchmarkScrubField(b *testing.B) {
	fieldNames := []string{
		"card_number",
		"merchant_id",
		"cvv",
		"customer_id",
		"mac_secret",
		"transaction_id",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, field := range fieldNames {
			ScrubField(field)
		}
	}
}
