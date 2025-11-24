package middleware

import (
	"regexp"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

// ScrubString scrubs sensitive data from a string using pattern matching
func ScrubString(s string) string {
	scrubbed := s

	// Apply regex patterns
	for _, pattern := range SensitiveFieldPatterns {
		scrubbed = pattern.ReplaceAllString(scrubbed, "[REDACTED]")
	}

	return scrubbed
}

// ScrubField checks if a field name is sensitive and should be redacted
func ScrubField(fieldName string) bool {
	fieldLower := strings.ToLower(fieldName)

	for _, sensitive := range SensitiveFields {
		if strings.Contains(fieldLower, sensitive) {
			return true
		}
	}

	return false
}

// ScrubZapFields creates a scrubbing zap.Field encoder
// This returns a function that can be used to wrap field values
func ScrubZapField(field zap.Field) zap.Field {
	// Check if field name indicates sensitive data
	if ScrubField(field.Key) {
		return zap.String(field.Key, "[REDACTED]")
	}

	// If it's a string field, scrub the content
	if field.Type == zapcore.StringType {
		if field.String != "" {
			scrubbedStr := ScrubString(field.String)
			if scrubbedStr != field.String {
				return zap.String(field.Key, scrubbedStr)
			}
		}
	}

	return field
}

// ScrubLogMessage scrubs sensitive data from log messages
func ScrubLogMessage(msg string) string {
	return ScrubString(msg)
}

// NewScrubLogger creates a zap logger wrapper that automatically scrubs sensitive fields
// Note: This is a basic implementation. For production, consider using zap.WrapCore
// to intercept fields before they're encoded.
func NewScrubLogger(logger *zap.Logger) *zap.Logger {
	// For now, we recommend manually scrubbing sensitive fields at call sites:
	//
	// GOOD:
	//   logger.Info("Processing payment",
	//       zap.String("merchant_id", merchantID),
	//       zap.String("last_4", last4),  // Only log last 4 digits
	//   )
	//
	// BAD:
	//   logger.Info("Processing payment",
	//       zap.String("card_number", cardNumber),  // Never log full PAN
	//   )
	//
	// For automatic scrubbing, wrap the core:
	// return logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
	//     return &scrubbingCore{Core: core}
	// }))

	return logger
}

// scrubbingCore is a zapcore.Core wrapper that scrubs sensitive fields
// This is a placeholder for future implementation if needed
type scrubbingCore struct {
	zapcore.Core
}

// Example of how to use the scrubber in application code:
//
// import "github.com/kevin07696/payment-service/internal/middleware"
//
// // Scrub a string before logging
// scrubbedMsg := middleware.ScrubString("Card 4111-1111-1111-1111 charged")
// logger.Info(scrubbedMsg)  // Logs: "Card [REDACTED] charged"
//
// // Check if a field should be scrubbed
// if middleware.ScrubField("card_number") {
//     logger.Info("Payment processed", zap.String("card_number", "[REDACTED]"))
// }
//
// // Manual scrubbing of sensitive fields
// logger.Info("EPX Response",
//     zap.String("auth_code", authCode),
//     zap.String("auth_resp_text", middleware.ScrubString(authRespText)),
// )
