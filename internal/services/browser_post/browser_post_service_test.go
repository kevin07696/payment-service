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
