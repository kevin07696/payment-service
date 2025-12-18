package browserpost

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseAmountToCents tests parsing of decimal amount strings to cents
// This function is critical for Browser POST where amounts come as strings from query params
func TestParseAmountToCents(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int64
		expectError bool
	}{
		// Standard formats
		{
			name:     "standard_two_decimal_places",
			input:    "50.00",
			expected: 5000,
		},
		{
			name:     "standard_with_cents",
			input:    "19.99",
			expected: 1999,
		},
		{
			name:     "repeating_decimal",
			input:    "33.33",
			expected: 3333,
		},
		{
			name:     "whole_dollar_no_decimal",
			input:    "100",
			expected: 10000,
		},
		{
			name:     "sub_dollar_amount",
			input:    "0.99",
			expected: 99,
		},
		{
			name:     "single_decimal_digit",
			input:    "50.0",
			expected: 5000,
		},
		{
			name:     "zero_amount",
			input:    "0.00",
			expected: 0,
		},
		{
			name:     "zero_no_decimal",
			input:    "0",
			expected: 0,
		},

		// Edge cases for precision
		{
			name:     "three_decimal_places_truncated",
			input:    "50.999",
			expected: 5099, // Truncates to 2 decimal places
		},
		{
			name:     "many_decimal_places_truncated",
			input:    "10.123456",
			expected: 1012, // Truncates to 2 decimal places
		},

		// Large amounts
		{
			name:     "thousand_dollars",
			input:    "1000.00",
			expected: 100000,
		},
		{
			name:     "ten_thousand_dollars",
			input:    "10000.00",
			expected: 1000000,
		},

		// Error cases
		{
			name:        "invalid_non_numeric",
			input:       "abc",
			expectError: true,
		},
		{
			name:        "invalid_empty_string",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid_partial_numeric",
			input:       "50.ab",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAmountToCents(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result, "parseAmountToCents(%q) should return %d", tt.input, tt.expected)
			}
		})
	}
}

// TestParseAmountToCents_FloatingPointPrecision verifies that the function
// correctly handles values that would cause floating-point precision issues
// if implemented with float64 multiplication
func TestParseAmountToCents_FloatingPointPrecision(t *testing.T) {
	// These specific values are known to cause issues with float64 * 100
	// e.g., 19.99 * 100 = 1998.9999999999998 in float64
	problemCases := []struct {
		input    string
		expected int64
	}{
		{"19.99", 1999},
		{"33.33", 3333},
		{"66.66", 6666},
		{"0.01", 1},
		{"0.10", 10},
		{"1.01", 101},
		{"2.99", 299},
	}

	for _, tc := range problemCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := parseAmountToCents(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result,
				"parseAmountToCents(%q) = %d, want %d (potential float precision issue)",
				tc.input, result, tc.expected)
		})
	}
}

// ============================================================================
// buildRedirectURL Tests
// ============================================================================
//
// Tests for URL construction ensuring proper encoding and security

// TestBuildRedirectURL tests the redirect URL construction
func TestBuildRedirectURL(t *testing.T) {
	svc := &BrowserPostService{
		callbackBaseURL: "https://api.example.com",
	}

	tests := []struct {
		name           string
		transactionID  string
		merchantID     string
		txType         string
		customerID     string
		expectedParams map[string]string
		description    string
	}{
		{
			name:          "all_parameters_present",
			transactionID: "550e8400-e29b-41d4-a716-446655440000",
			merchantID:    "660e8400-e29b-41d4-a716-446655440001",
			txType:        "SALE",
			customerID:    "customer_123",
			expectedParams: map[string]string{
				"transaction_id":   "550e8400-e29b-41d4-a716-446655440000",
				"merchant_id":      "660e8400-e29b-41d4-a716-446655440001",
				"transaction_type": "SALE",
				"customer_id":      "customer_123",
			},
			description: "All parameters should be included in URL",
		},
		{
			name:          "empty_customer_id",
			transactionID: "550e8400-e29b-41d4-a716-446655440000",
			merchantID:    "660e8400-e29b-41d4-a716-446655440001",
			txType:        "AUTH",
			customerID:    "",
			expectedParams: map[string]string{
				"transaction_id":   "550e8400-e29b-41d4-a716-446655440000",
				"merchant_id":      "660e8400-e29b-41d4-a716-446655440001",
				"transaction_type": "AUTH",
			},
			description: "Empty customer_id should not be included",
		},
		{
			name:          "special_characters_in_customer_id",
			transactionID: "550e8400-e29b-41d4-a716-446655440000",
			merchantID:    "660e8400-e29b-41d4-a716-446655440001",
			txType:        "SALE",
			customerID:    "user+test@example.com",
			expectedParams: map[string]string{
				"transaction_id":   "550e8400-e29b-41d4-a716-446655440000",
				"merchant_id":      "660e8400-e29b-41d4-a716-446655440001",
				"transaction_type": "SALE",
				"customer_id":      "user+test@example.com",
			},
			description: "Special characters should be properly URL encoded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.buildRedirectURL(tt.transactionID, tt.merchantID, tt.txType, tt.customerID)

			// Verify base URL
			assert.Contains(t, result, "https://api.example.com/api/v1/payments/browser-post/callback?")

			// Verify each expected parameter
			for key, value := range tt.expectedParams {
				assert.Contains(t, result, key+"=", tt.description)
				// For URL-encoded values, check the encoded form
				if key == "customer_id" && value == "user+test@example.com" {
					// URL encoding should convert + to %2B and @ to %40
					assert.Contains(t, result, "customer_id=user%2Btest%40example.com")
				}
			}

			// Verify customer_id is NOT in URL when empty
			if tt.customerID == "" {
				assert.NotContains(t, result, "customer_id=")
			}
		})
	}
}

// TestBuildRedirectURL_InjectionPrevention tests that URL construction prevents injection attacks
func TestBuildRedirectURL_InjectionPrevention(t *testing.T) {
	svc := &BrowserPostService{
		callbackBaseURL: "https://api.example.com",
	}

	tests := []struct {
		name          string
		customerID    string
		shouldEncode  string
		description   string
	}{
		{
			name:         "ampersand_injection",
			customerID:   "customer&admin=true",
			shouldEncode: "%26",
			description:  "Ampersand should be encoded to prevent parameter injection",
		},
		{
			name:         "equals_injection",
			customerID:   "customer=admin",
			shouldEncode: "%3D",
			description:  "Equals should be encoded to prevent value injection",
		},
		{
			name:         "question_mark_injection",
			customerID:   "customer?callback=evil.com",
			shouldEncode: "%3F",
			description:  "Question mark should be encoded",
		},
		{
			name:         "newline_injection",
			customerID:   "customer\nX-Injected: header",
			shouldEncode: "%0A",
			description:  "Newlines should be encoded to prevent header injection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.buildRedirectURL(
				"550e8400-e29b-41d4-a716-446655440000",
				"660e8400-e29b-41d4-a716-446655440001",
				"SALE",
				tt.customerID,
			)

			// Verify the dangerous character is encoded
			assert.Contains(t, result, tt.shouldEncode, tt.description)
			// Verify the raw dangerous character is NOT present
			assert.NotContains(t, result, tt.customerID, "Raw input should be encoded, not present as-is")
		})
	}
}

// ============================================================================
// GenerateFormConfig Validation Tests
// ============================================================================
//
// NOTE: GenerateFormConfig requires mocking:
// - sqlc.Querier (158 methods)
// - ports.KeyExchangeAdapter
// - ports.SecretManagerAdapter
//
// Input validation tests would require significant mock setup.
// The validation logic is straightforward UUID parsing and is better tested via integration tests.
// See: tests/integration/browser_post/generate_form_config_test.go
//
// Key validation scenarios:
// - Invalid transaction_id format → ErrValidationInvalidUUID
// - Invalid merchant_id format → ErrValidationInvalidUUID
// - Invalid transaction_type → ErrTxnInvalidType
// - Inactive merchant → ErrMerchantInactiveTyped
// - MAC secret fetch failure → ErrMerchantCredentialFailed
// - Key Exchange failure → ErrGatewayError

// ============================================================================
// validateMACSignature Validation Tests
// ============================================================================
//
// NOTE: validateMACSignature requires mocking:
// - sqlc.Querier.GetMerchantByID
// - ports.SecretManagerAdapter.GetSecret
// - ports.BrowserPostAdapter.ValidateResponseMAC
//
// These tests are better suited for integration tests.
// See: tests/integration/browser_post/mac_validation_test.go
//
// Key validation scenarios:
// - Empty merchant_id → ErrMerchantRequired
// - Invalid merchant_id format → ErrValidationInvalidUUID
// - Merchant not found → ErrMerchantNotFoundTyped
// - Secret fetch failure → ErrMerchantCredentialFailed
// - Invalid MAC signature → ErrSignatureValidationFailed (via adapter)
