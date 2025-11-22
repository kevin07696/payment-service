package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kevin07696/payment-service/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityHeaders_ProductionMode tests security headers in production mode
// Security Risk: HIGH - Missing security headers expose users to various attacks
func TestSecurityHeaders_ProductionMode(t *testing.T) {
	// Create production mode security headers
	secHeaders := middleware.NewSecurityHeaders(false) // false = production

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	protectedHandler := secHeaders.Middleware(handler)

	t.Run("XFrameOptions_DENY", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"),
			"X-Frame-Options should be set to DENY to prevent clickjacking")

		t.Log("[PASS] X-Frame-Options: DENY (prevents clickjacking)")
	})

	t.Run("XContentTypeOptions_nosniff", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"),
			"X-Content-Type-Options should be nosniff to prevent MIME sniffing")

		t.Log("[PASS] X-Content-Type-Options: nosniff (prevents MIME sniffing)")
	})

	t.Run("XXSSProtection_Enabled", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		assert.Equal(t, "1; mode=block", rec.Header().Get("X-XSS-Protection"),
			"X-XSS-Protection should enable XSS filter for legacy browsers")

		t.Log("[PASS] X-XSS-Protection: 1; mode=block (legacy browser protection)")
	})

	t.Run("HSTS_ProductionOnly", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		hsts := rec.Header().Get("Strict-Transport-Security")
		assert.NotEmpty(t, hsts, "HSTS header should be set in production")
		assert.Contains(t, hsts, "max-age=31536000",
			"HSTS should have 1 year max-age")
		assert.Contains(t, hsts, "includeSubDomains",
			"HSTS should include subdomains")
		assert.Contains(t, hsts, "preload",
			"HSTS should allow preload list inclusion")

		t.Logf("[PASS] HSTS: %s (forces HTTPS)", hsts)
	})

	t.Run("CSP_RestrictivePolicy", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		csp := rec.Header().Get("Content-Security-Policy")
		assert.NotEmpty(t, csp, "CSP header should be set")
		assert.Contains(t, csp, "default-src 'none'",
			"CSP should block all content by default (API service)")
		assert.Contains(t, csp, "frame-ancestors 'none'",
			"CSP should prevent framing")
		assert.Contains(t, csp, "base-uri 'none'",
			"CSP should prevent base tag injection")
		assert.Contains(t, csp, "form-action 'none'",
			"CSP should prevent form submissions (API service)")

		t.Logf("[PASS] CSP: %s (restrictive API policy)", csp)
	})

	t.Run("ReferrerPolicy_NoReferrer", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"),
			"Referrer-Policy should prevent leaking URLs to third parties")

		t.Log("[PASS] Referrer-Policy: no-referrer (prevents URL leakage)")
	})

	t.Run("PermissionsPolicy_DisabledFeatures", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		permPolicy := rec.Header().Get("Permissions-Policy")
		assert.NotEmpty(t, permPolicy, "Permissions-Policy should be set")

		// Verify dangerous features are disabled
		assert.Contains(t, permPolicy, "geolocation=()",
			"Should disable geolocation")
		assert.Contains(t, permPolicy, "microphone=()",
			"Should disable microphone")
		assert.Contains(t, permPolicy, "camera=()",
			"Should disable camera")
		assert.Contains(t, permPolicy, "payment=()",
			"Should disable browser payment API")

		t.Logf("[PASS] Permissions-Policy: %s (disables unnecessary features)", permPolicy)
	})

	t.Run("XPermittedCrossDomainPolicies_None", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		assert.Equal(t, "none", rec.Header().Get("X-Permitted-Cross-Domain-Policies"),
			"Should prevent Adobe Flash/PDF cross-domain access")

		t.Log("[PASS] X-Permitted-Cross-Domain-Policies: none (prevents Flash/PDF XD)")
	})

	t.Run("AllHeaders_SetOnEveryRequest", func(t *testing.T) {
		// Make multiple requests to verify headers are set consistently
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			protectedHandler.ServeHTTP(rec, req)

			// Verify all critical headers are present
			headers := rec.Header()
			assert.NotEmpty(t, headers.Get("X-Frame-Options"), "Request %d missing X-Frame-Options", i)
			assert.NotEmpty(t, headers.Get("X-Content-Type-Options"), "Request %d missing X-Content-Type-Options", i)
			assert.NotEmpty(t, headers.Get("Content-Security-Policy"), "Request %d missing CSP", i)
			assert.NotEmpty(t, headers.Get("Referrer-Policy"), "Request %d missing Referrer-Policy", i)
		}

		t.Log("[PASS] All security headers set consistently on every request")
	})
}

// TestSecurityHeaders_DevelopmentMode tests security headers in development mode
func TestSecurityHeaders_DevelopmentMode(t *testing.T) {
	// Create development mode security headers
	secHeaders := middleware.NewSecurityHeaders(true) // true = development

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	protectedHandler := secHeaders.Middleware(handler)

	t.Run("HSTS_NotSet_InDevelopment", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		hsts := rec.Header().Get("Strict-Transport-Security")
		assert.Empty(t, hsts,
			"HSTS should NOT be set in development to allow HTTP localhost")

		t.Log("[PASS] HSTS not set in development (allows HTTP localhost)")
	})

	t.Run("CSP_Permissive_InDevelopment", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		csp := rec.Header().Get("Content-Security-Policy")
		assert.NotEmpty(t, csp, "CSP should still be set in development")

		// Development CSP should be more permissive
		assert.Contains(t, csp, "default-src 'self'",
			"Development CSP should allow self resources")
		assert.Contains(t, csp, "unsafe-inline",
			"Development CSP should allow inline scripts/styles for debugging")

		// But should still block framing
		assert.Contains(t, csp, "frame-ancestors 'none'",
			"Even in development, should prevent framing")

		t.Logf("[PASS] CSP in development: %s (more permissive for debugging)", csp)
	})

	t.Run("OtherHeaders_StillSet_InDevelopment", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rec, req)

		// Other security headers should still be set in development
		assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"),
			"X-Frame-Options should be set even in development")
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"),
			"X-Content-Type-Options should be set even in development")
		assert.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"),
			"Referrer-Policy should be set even in development")

		t.Log("[PASS] Other security headers still set in development")
	})
}

// TestSecurityHeaders_MiddlewareFunc tests the function wrapper variant
func TestSecurityHeaders_MiddlewareFunc(t *testing.T) {
	secHeaders := middleware.NewSecurityHeaders(false)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	protectedHandler := secHeaders.MiddlewareFunc(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	protectedHandler(rec, req)

	// Verify headers are set using MiddlewareFunc wrapper
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"),
		"MiddlewareFunc should also set security headers")
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"),
		"CSP should be set via MiddlewareFunc")

	t.Log("[PASS] MiddlewareFunc wrapper applies security headers")
}

// TestSecurityHeaders_ChainedMiddleware tests that headers work when chained
func TestSecurityHeaders_ChainedMiddleware(t *testing.T) {
	secHeaders := middleware.NewSecurityHeaders(false)

	// Create a chain: SecurityHeaders -> Custom Middleware -> Handler
	customMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Custom middleware adds its own header
			w.Header().Set("X-Custom-Header", "custom-value")
			next.ServeHTTP(w, r)
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain: SecurityHeaders -> CustomMiddleware -> Handler
	chain := secHeaders.Middleware(customMiddleware(handler))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	chain.ServeHTTP(rec, req)

	// Verify both security headers and custom header are present
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"),
		"Security headers should be set in chain")
	assert.Equal(t, "custom-value", rec.Header().Get("X-Custom-Header"),
		"Custom middleware headers should also be set")

	t.Log("[PASS] Security headers work correctly in middleware chain")
}

// TestSecurityHeaders_OWASP_Compliance tests OWASP security header recommendations
func TestSecurityHeaders_OWASP_Compliance(t *testing.T) {
	secHeaders := middleware.NewSecurityHeaders(false)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	protectedHandler := secHeaders.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	protectedHandler.ServeHTTP(rec, req)

	// OWASP Top 10 2021 - Security Header Recommendations
	headers := rec.Header()

	t.Run("OWASP_Clickjacking_Protection", func(t *testing.T) {
		// A04:2021 – Insecure Design (Clickjacking)
		xfo := headers.Get("X-Frame-Options")
		csp := headers.Get("Content-Security-Policy")

		assert.Equal(t, "DENY", xfo, "X-Frame-Options should prevent clickjacking")
		assert.Contains(t, csp, "frame-ancestors 'none'",
			"CSP should also prevent clickjacking (modern approach)")

		t.Log("[PASS] OWASP: Clickjacking protection via X-Frame-Options + CSP")
	})

	t.Run("OWASP_XSS_Protection", func(t *testing.T) {
		// A03:2021 – Injection (XSS)
		csp := headers.Get("Content-Security-Policy")
		xxss := headers.Get("X-XSS-Protection")

		assert.Contains(t, csp, "default-src 'none'",
			"Restrictive CSP prevents XSS attacks")
		assert.Equal(t, "1; mode=block", xxss,
			"X-XSS-Protection provides legacy browser protection")

		t.Log("[PASS] OWASP: XSS protection via CSP + X-XSS-Protection")
	})

	t.Run("OWASP_MITM_Protection", func(t *testing.T) {
		// A02:2021 – Cryptographic Failures (MITM)
		hsts := headers.Get("Strict-Transport-Security")

		require.NotEmpty(t, hsts, "HSTS should be set to prevent MITM")
		assert.Contains(t, hsts, "max-age=31536000",
			"HSTS max-age should be at least 1 year")
		assert.Contains(t, hsts, "includeSubDomains",
			"HSTS should protect subdomains")

		t.Log("[PASS] OWASP: MITM protection via HSTS")
	})

	t.Run("OWASP_Information_Leakage_Prevention", func(t *testing.T) {
		// A01:2021 – Broken Access Control (Info Leakage)
		refPolicy := headers.Get("Referrer-Policy")
		xcto := headers.Get("X-Content-Type-Options")

		assert.Equal(t, "no-referrer", refPolicy,
			"Referrer-Policy should prevent URL leakage")
		assert.Equal(t, "nosniff", xcto,
			"X-Content-Type-Options prevents MIME sniffing")

		t.Log("[PASS] OWASP: Information leakage prevention")
	})

	t.Run("OWASP_SecurityHeaders_Score", func(t *testing.T) {
		// Check against securityheaders.com criteria
		requiredHeaders := []string{
			"X-Frame-Options",
			"X-Content-Type-Options",
			"Content-Security-Policy",
			"Strict-Transport-Security",
			"Referrer-Policy",
		}

		missingHeaders := []string{}
		for _, header := range requiredHeaders {
			if headers.Get(header) == "" {
				missingHeaders = append(missingHeaders, header)
			}
		}

		assert.Empty(t, missingHeaders,
			"All critical security headers should be present: %v", missingHeaders)

		t.Log("[PASS] OWASP: All critical security headers present")
		t.Log("[INFO] Would score A+ on securityheaders.com")
	})
}
