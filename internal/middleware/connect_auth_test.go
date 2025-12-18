package middleware

import (
	"net/http"
	"testing"

	"go.uber.org/zap"
)

func TestExtractClientIP_TrustedProxies(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name           string
		trustedProxies []string
		remoteAddr     string
		xForwardedFor  string
		xRealIP        string
		expectedIP     string
	}{
		{
			name:           "Trusted proxy with X-Forwarded-For",
			trustedProxies: []string{"10.0.0.1"},
			remoteAddr:     "10.0.0.1:12345",
			xForwardedFor:  "192.168.1.100",
			xRealIP:        "",
			expectedIP:     "192.168.1.100",
		},
		{
			name:           "Trusted proxy with X-Forwarded-For chain",
			trustedProxies: []string{"10.0.0.1"},
			remoteAddr:     "10.0.0.1:12345",
			xForwardedFor:  "203.0.113.50, 10.0.0.5, 10.0.0.1",
			xRealIP:        "",
			expectedIP:     "203.0.113.50", // First IP in chain is original client
		},
		{
			name:           "Trusted proxy with X-Real-IP",
			trustedProxies: []string{"10.0.0.1"},
			remoteAddr:     "10.0.0.1:12345",
			xForwardedFor:  "",
			xRealIP:        "192.168.1.100",
			expectedIP:     "192.168.1.100",
		},
		{
			name:           "Untrusted proxy - header ignored",
			trustedProxies: []string{"10.0.0.1"},
			remoteAddr:     "172.16.0.50:12345",
			xForwardedFor:  "192.168.1.100",
			xRealIP:        "",
			expectedIP:     "172.16.0.50", // Remote addr used, header ignored
		},
		{
			name:           "No trusted proxies configured",
			trustedProxies: nil,
			remoteAddr:     "10.0.0.1:12345",
			xForwardedFor:  "192.168.1.100",
			xRealIP:        "",
			expectedIP:     "10.0.0.1", // Remote addr used, header ignored
		},
		{
			name:           "Empty trusted proxies list",
			trustedProxies: []string{},
			remoteAddr:     "10.0.0.1:12345",
			xForwardedFor:  "192.168.1.100",
			xRealIP:        "",
			expectedIP:     "10.0.0.1",
		},
		{
			name:           "Remote address without port",
			trustedProxies: []string{"10.0.0.1"},
			remoteAddr:     "10.0.0.1",
			xForwardedFor:  "192.168.1.100",
			xRealIP:        "",
			expectedIP:     "192.168.1.100",
		},
		{
			name:           "IPv6 trusted proxy",
			trustedProxies: []string{"::1"},
			remoteAddr:     "[::1]:12345",
			xForwardedFor:  "2001:db8::1",
			xRealIP:        "",
			expectedIP:     "2001:db8::1",
		},
		{
			name:           "IPv6 untrusted proxy",
			trustedProxies: []string{"::1"},
			remoteAddr:     "[2001:db8::5]:12345",
			xForwardedFor:  "2001:db8::1",
			xRealIP:        "",
			expectedIP:     "2001:db8::5", // Header ignored
		},
		{
			name:           "X-Forwarded-For takes precedence over X-Real-IP",
			trustedProxies: []string{"10.0.0.1"},
			remoteAddr:     "10.0.0.1:12345",
			xForwardedFor:  "192.168.1.100",
			xRealIP:        "192.168.1.200",
			expectedIP:     "192.168.1.100", // XFF preferred
		},
		{
			name:           "Malicious client spoofing - should be blocked",
			trustedProxies: []string{"10.0.0.1"}, // Load balancer IP
			remoteAddr:     "192.168.100.50:12345",
			xForwardedFor:  "8.8.8.8", // Attacker trying to spoof IP
			xRealIP:        "",
			expectedIP:     "192.168.100.50", // Attacker's real IP returned
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Build trusted proxy map
			proxyMap := make(map[string]bool, len(tc.trustedProxies))
			for _, proxy := range tc.trustedProxies {
				proxyMap[proxy] = true
			}

			ai := &AuthInterceptor{
				trustedProxies: proxyMap,
				logger:         logger,
			}

			headers := http.Header{}
			if tc.xForwardedFor != "" {
				headers.Set("X-Forwarded-For", tc.xForwardedFor)
			}
			if tc.xRealIP != "" {
				headers.Set("X-Real-IP", tc.xRealIP)
			}

			result := ai.extractClientIP(headers, tc.remoteAddr)

			if result != tc.expectedIP {
				t.Errorf("extractClientIP() = %q, want %q", result, tc.expectedIP)
			}
		})
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name         string
		addr         string
		expectedHost string
		expectedPort string
		expectErr    bool
	}{
		{
			name:         "IPv4 with port",
			addr:         "192.168.1.1:8080",
			expectedHost: "192.168.1.1",
			expectedPort: "8080",
		},
		{
			name:         "IPv4 without port",
			addr:         "192.168.1.1",
			expectedHost: "192.168.1.1",
			expectedPort: "",
		},
		{
			name:         "IPv6 with port",
			addr:         "[::1]:8080",
			expectedHost: "::1",
			expectedPort: "8080",
		},
		{
			name:         "IPv6 without port (bracketed)",
			addr:         "[::1]",
			expectedHost: "::1",
			expectedPort: "",
		},
		{
			name:         "IPv6 full address with port",
			addr:         "[2001:db8:85a3::8a2e:370:7334]:443",
			expectedHost: "2001:db8:85a3::8a2e:370:7334",
			expectedPort: "443",
		},
		{
			name:         "Localhost with port",
			addr:         "localhost:3000",
			expectedHost: "localhost",
			expectedPort: "3000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, port, err := splitHostPort(tc.addr)

			if tc.expectErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if host != tc.expectedHost {
				t.Errorf("host = %q, want %q", host, tc.expectedHost)
			}
			if port != tc.expectedPort {
				t.Errorf("port = %q, want %q", port, tc.expectedPort)
			}
		})
	}
}

func TestExtractClientIP_SecurityScenarios(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("Attacker cannot spoof IP when not behind trusted proxy", func(t *testing.T) {
		ai := &AuthInterceptor{
			trustedProxies: map[string]bool{"10.0.0.1": true},
			logger:         logger,
		}

		// Attacker connects directly and tries to spoof X-Forwarded-For
		headers := http.Header{}
		headers.Set("X-Forwarded-For", "8.8.8.8") // Trying to look like Google

		result := ai.extractClientIP(headers, "192.168.100.50:12345")

		// Should return attacker's real IP, not the spoofed one
		if result != "192.168.100.50" {
			t.Errorf("IP spoofing protection failed: got %q, want %q", result, "192.168.100.50")
		}
	})

	t.Run("Load balancer can set real client IP", func(t *testing.T) {
		ai := &AuthInterceptor{
			trustedProxies: map[string]bool{"10.0.0.1": true},
			logger:         logger,
		}

		// Request comes from trusted load balancer with client IP
		headers := http.Header{}
		headers.Set("X-Forwarded-For", "203.0.113.195")

		result := ai.extractClientIP(headers, "10.0.0.1:12345")

		// Should return the client IP set by load balancer
		if result != "203.0.113.195" {
			t.Errorf("Load balancer IP passthrough failed: got %q, want %q", result, "203.0.113.195")
		}
	})

	t.Run("Multiple proxies in X-Forwarded-For chain", func(t *testing.T) {
		ai := &AuthInterceptor{
			trustedProxies: map[string]bool{"10.0.0.1": true},
			logger:         logger,
		}

		// Request passed through multiple proxies
		headers := http.Header{}
		headers.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")

		result := ai.extractClientIP(headers, "10.0.0.1:12345")

		// Should return the first IP (original client)
		if result != "203.0.113.195" {
			t.Errorf("Multi-proxy chain handling failed: got %q, want %q", result, "203.0.113.195")
		}
	})
}
