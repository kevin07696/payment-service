package cron

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAuthenticateCronRequest validates the timing-safe cron authentication
func TestAuthenticateCronRequest(t *testing.T) {
	const validSecret = "test-cron-secret-at-least-32-characters-long"

	tests := []struct {
		name           string
		expectedSecret string
		headerName     string
		headerValue    string
		want           bool
	}{
		// Valid X-Cron-Secret cases
		{
			name:           "Valid X-Cron-Secret header",
			expectedSecret: validSecret,
			headerName:     "X-Cron-Secret",
			headerValue:    validSecret,
			want:           true,
		},
		{
			name:           "Invalid X-Cron-Secret header",
			expectedSecret: validSecret,
			headerName:     "X-Cron-Secret",
			headerValue:    "wrong-secret",
			want:           false,
		},
		{
			name:           "Empty X-Cron-Secret header",
			expectedSecret: validSecret,
			headerName:     "X-Cron-Secret",
			headerValue:    "",
			want:           false,
		},
		{
			name:           "Missing X-Cron-Secret header",
			expectedSecret: validSecret,
			headerName:     "",
			headerValue:    "",
			want:           false,
		},

		// Valid Bearer token cases
		{
			name:           "Valid Bearer token",
			expectedSecret: validSecret,
			headerName:     "Authorization",
			headerValue:    "Bearer " + validSecret,
			want:           true,
		},
		{
			name:           "Invalid Bearer token",
			expectedSecret: validSecret,
			headerName:     "Authorization",
			headerValue:    "Bearer wrong-secret",
			want:           false,
		},
		{
			name:           "Bearer token without Bearer prefix",
			expectedSecret: validSecret,
			headerName:     "Authorization",
			headerValue:    validSecret,
			want:           false,
		},
		{
			name:           "Empty Authorization header",
			expectedSecret: validSecret,
			headerName:     "Authorization",
			headerValue:    "",
			want:           false,
		},

		// Edge cases
		{
			name:           "Empty expected secret rejects all",
			expectedSecret: "",
			headerName:     "X-Cron-Secret",
			headerValue:    validSecret,
			want:           false,
		},
		{
			name:           "Both headers empty",
			expectedSecret: validSecret,
			headerName:     "",
			headerValue:    "",
			want:           false,
		},
		{
			name:           "Partial match (prefix)",
			expectedSecret: validSecret,
			headerName:     "X-Cron-Secret",
			headerValue:    "test-cron-secret",
			want:           false,
		},
		{
			name:           "Case sensitivity - uppercase",
			expectedSecret: validSecret,
			headerName:     "X-Cron-Secret",
			headerValue:    "TEST-CRON-SECRET-AT-LEAST-32-CHARACTERS-LONG",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/cron/test", nil)
			if tt.headerName != "" && tt.headerValue != "" {
				req.Header.Set(tt.headerName, tt.headerValue)
			}

			got := AuthenticateCronRequest(req, tt.expectedSecret)
			assert.Equal(t, tt.want, got, "AuthenticateCronRequest() = %v, want %v", got, tt.want)
		})
	}
}

// TestConstantTimeEqual validates the timing-safe comparison function
func TestConstantTimeEqual(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "Equal strings",
			a:    "secret123",
			b:    "secret123",
			want: true,
		},
		{
			name: "Different strings same length",
			a:    "secret123",
			b:    "secret456",
			want: false,
		},
		{
			name: "Different lengths - a shorter",
			a:    "short",
			b:    "longer-string",
			want: false,
		},
		{
			name: "Different lengths - b shorter",
			a:    "longer-string",
			b:    "short",
			want: false,
		},
		{
			name: "Empty strings equal",
			a:    "",
			b:    "",
			want: true,
		},
		{
			name: "Empty vs non-empty",
			a:    "",
			b:    "nonempty",
			want: false,
		},
		{
			name: "Single character equal",
			a:    "x",
			b:    "x",
			want: true,
		},
		{
			name: "Single character different",
			a:    "x",
			b:    "y",
			want: false,
		},
		{
			name: "Unicode strings equal",
			a:    "hello-世界",
			b:    "hello-世界",
			want: true,
		},
		{
			name: "Unicode strings different",
			a:    "hello-世界",
			b:    "hello-世界!",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constantTimeEqual(tt.a, tt.b)
			assert.Equal(t, tt.want, got, "constantTimeEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		})
	}
}

// TestAuthenticateCronRequest_MultipleHeaders tests that X-Cron-Secret takes precedence
func TestAuthenticateCronRequest_MultipleHeaders(t *testing.T) {
	const validSecret = "test-cron-secret-at-least-32-characters-long"

	t.Run("X-Cron-Secret valid, Authorization invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cron/test", nil)
		req.Header.Set("X-Cron-Secret", validSecret)
		req.Header.Set("Authorization", "Bearer wrong-secret")

		got := AuthenticateCronRequest(req, validSecret)
		assert.True(t, got, "Should authenticate when X-Cron-Secret is valid")
	})

	t.Run("X-Cron-Secret invalid, Authorization valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cron/test", nil)
		req.Header.Set("X-Cron-Secret", "wrong-secret")
		req.Header.Set("Authorization", "Bearer "+validSecret)

		got := AuthenticateCronRequest(req, validSecret)
		assert.True(t, got, "Should authenticate when Authorization is valid (fallback)")
	})

	t.Run("Both invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cron/test", nil)
		req.Header.Set("X-Cron-Secret", "wrong-secret")
		req.Header.Set("Authorization", "Bearer also-wrong")

		got := AuthenticateCronRequest(req, validSecret)
		assert.False(t, got, "Should not authenticate when both are invalid")
	})
}
