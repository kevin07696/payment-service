// Package cron provides handlers for scheduled cron jobs.
package cron

import (
	"crypto/subtle"
	"net/http"
)

// AuthenticateCronRequest verifies the cron request is authorized using timing-safe comparison.
// This is the centralized authentication function for all cron handlers.
//
// Security: Uses crypto/subtle.ConstantTimeCompare to prevent timing attacks
// where an attacker could determine the secret character-by-character by
// measuring response times.
func AuthenticateCronRequest(r *http.Request, expectedSecret string) bool {
	if expectedSecret == "" {
		return false
	}

	// Check X-Cron-Secret header (primary method)
	cronSecret := r.Header.Get("X-Cron-Secret")
	if cronSecret != "" && constantTimeEqual(cronSecret, expectedSecret) {
		return true
	}

	// Check Authorization header (Bearer token)
	authHeader := r.Header.Get("Authorization")
	expectedBearer := "Bearer " + expectedSecret
	if authHeader != "" && constantTimeEqual(authHeader, expectedBearer) {
		return true
	}

	// Note: Google Cloud Scheduler OIDC token validation can be added here
	// for production environments that require additional security.
	// The X-Cron-Secret header is the primary authentication method.

	return false
}

// constantTimeEqual performs a timing-safe string comparison.
// Returns true if the strings are equal, false otherwise.
// The time taken is proportional to the length of the strings and is independent
// of the contents, preventing timing attacks.
func constantTimeEqual(a, b string) bool {
	// If lengths differ, still do a comparison to maintain constant time
	// but the result will always be false
	if len(a) != len(b) {
		// Compare against a dummy value of same length to maintain constant time
		dummy := make([]byte, len(a))
		subtle.ConstantTimeCompare([]byte(a), dummy)
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
