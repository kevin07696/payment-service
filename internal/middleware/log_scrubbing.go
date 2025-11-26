package middleware

import (
	"regexp"
)

// SensitiveFieldPatterns defines regex patterns for detecting sensitive data
// NOTE: These patterns are conservative to avoid false positives.
// Most scrubbing should be done by checking field names with ScrubField()
var SensitiveFieldPatterns = map[string]*regexp.Regexp{
	// Credit card numbers (PAN) - 13-19 consecutive digits only (no spaces/dashes)
	// Spaces/dashes patterns too aggressive (match order IDs, etc.)
	"credit_card": regexp.MustCompile(`\b\d{13,19}\b`),

	// Social Security Numbers - matches XXX-XX-XXXX format only
	"ssn": regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),

	// BRIC tokens from EPX (starts with "bric_")
	"bric_token": regexp.MustCompile(`bric_[A-Za-z0-9_]+`),

	// JWT tokens (starts with "eyJ" - full JWT format)
	"jwt_token": regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}\.eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}`),

	// API keys (common patterns - only if clearly an API key)
	"api_key_sk": regexp.MustCompile(`sk_[a-zA-Z0-9_]{20,}`), // Stripe-style secret keys
	"api_key_pk": regexp.MustCompile(`pk_[a-zA-Z0-9_]{20,}`), // Stripe-style publishable keys
}

// SensitiveFields is a list of field names that should be scrubbed from logs
var SensitiveFields = []string{
	// Payment card fields
	"card_number",
	"cardnumber",
	"pan",
	"cvv",
	"cvv2",
	"cvc",
	"expiry",
	"expiration_date",
	"exp_month",
	"exp_year",

	// ACH/Bank fields
	"account_number",
	"accountnumber",
	"routing_number",
	"routingnumber",
	"bank_account",

	// Authentication/Security
	"password",
	"passwd",
	"pwd",
	"secret",
	"mac_secret",
	"private_key",
	"api_key",
	"apikey",
	"auth_token",
	"access_token",
	"refresh_token",
	"bearer_token",

	// EPX-specific
	"bric",
	"bric_token",
	"epx_mac",

	// Personal information
	"ssn",
	"social_security",
	"tax_id",
	"drivers_license",

	// Sensitive response fields
	"auth_resp_text",      // EPX auth response text may contain card details
	"processor_resp_text", // Processor response text
}

// Note: Log scrubbing functions (ScrubString, ScrubField, ScrubZapField, etc.) were removed
// as the codebase uses manual field selection instead - logging safe fields like "last_4"
// rather than scrubbing sensitive data after the fact.
//
// The patterns above (SensitiveFieldPatterns, SensitiveFields) are retained as reference
// for what constitutes sensitive data in this payment system.
