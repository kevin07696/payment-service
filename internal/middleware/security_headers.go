package middleware

import (
	"net/http"
)

// SecurityHeaders adds security-related HTTP headers to responses
// These headers help protect against common web vulnerabilities
type SecurityHeaders struct {
	// Allow customization for development vs production
	isDevelopment bool
}

// NewSecurityHeaders creates a new security headers middleware
func NewSecurityHeaders(isDevelopment bool) *SecurityHeaders {
	return &SecurityHeaders{
		isDevelopment: isDevelopment,
	}
}

// Middleware wraps an HTTP handler with security headers
func (sh *SecurityHeaders) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// X-Frame-Options: Prevents clickjacking attacks
		// DENY prevents any domain from framing this site
		w.Header().Set("X-Frame-Options", "DENY")

		// X-Content-Type-Options: Prevents MIME type sniffing
		// nosniff prevents browsers from MIME-sniffing a response away from the declared content-type
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// X-XSS-Protection: Enables XSS filter in older browsers
		// 1; mode=block enables the filter and blocks the page if XSS is detected
		// Note: Modern browsers use CSP instead, but this helps legacy browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Strict-Transport-Security (HSTS): Forces HTTPS connections
		// Only set in production to avoid issues with local development
		if !sh.isDevelopment {
			// max-age=31536000 (1 year), includeSubDomains applies to all subdomains
			// preload allows inclusion in browser HSTS preload lists
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Content-Security-Policy: Controls what resources can be loaded
		// This is a restrictive policy suitable for an API service
		csp := "default-src 'none'; " + // Block all content by default
			"frame-ancestors 'none'; " + // Prevent framing (redundant with X-Frame-Options but CSP is modern standard)
			"base-uri 'none'; " + // Prevent <base> tag injection
			"form-action 'none'" // Prevent form submissions (API service shouldn't have forms)

		// In development, allow slightly more permissive CSP for debugging
		if sh.isDevelopment {
			// Allow inline scripts/styles for development tools
			csp = "default-src 'self'; " +
				"script-src 'self' 'unsafe-inline'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"frame-ancestors 'none'; " +
				"base-uri 'self'; " +
				"form-action 'self'"
		}
		w.Header().Set("Content-Security-Policy", csp)

		// Referrer-Policy: Controls referrer information sent with requests
		// no-referrer prevents leaking sensitive URLs to third parties
		w.Header().Set("Referrer-Policy", "no-referrer")

		// Permissions-Policy: Controls browser features
		// Disables features not needed by a payment API
		w.Header().Set("Permissions-Policy",
			"geolocation=(), "+
				"microphone=(), "+
				"camera=(), "+
				"payment=(), "+ // Ironically, disable browser payment API (we handle payments ourselves)
				"usb=(), "+
				"magnetometer=(), "+
				"gyroscope=(), "+
				"accelerometer=()")

		// X-Permitted-Cross-Domain-Policies: Controls Adobe Flash/PDF cross-domain policies
		// none prevents all cross-domain access
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// MiddlewareFunc returns a function that wraps an http.HandlerFunc
// This is useful for simpler handler chains
func (sh *SecurityHeaders) MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sh.Middleware(next).ServeHTTP(w, r)
	}
}
