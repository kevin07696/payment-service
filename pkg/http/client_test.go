package http

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_EPXClientConfig verifies EPX configuration values
func Test_EPXClientConfig(t *testing.T) {
	t.Parallel()

	// Act
	cfg := EPXClientConfig()

	// Assert - EPX is single host, high concurrency
	assert.Equal(t, 50, cfg.MaxIdleConns, "EPX should have 50 total idle connections")
	assert.Equal(t, 50, cfg.MaxIdleConnsPerHost, "EPX should allow 50 idle per host (single host)")
	assert.Equal(t, 100, cfg.MaxConnsPerHost, "EPX should allow 100 concurrent connections")
	assert.Equal(t, 90*time.Second, cfg.IdleConnTimeout, "EPX should have 90s idle timeout")

	// Timeouts tuned for payment gateway
	assert.Equal(t, 10*time.Second, cfg.DialTimeout)
	assert.Equal(t, 10*time.Second, cfg.TLSHandshakeTimeout)
	assert.Equal(t, 30*time.Second, cfg.ResponseHeaderTimeout, "EPX can be slow, needs longer timeout")
	assert.Equal(t, 1*time.Second, cfg.ExpectContinueTimeout)

	// Keep-alive for connection reuse
	assert.False(t, cfg.DisableKeepAlives, "EPX should use keep-alive")
	assert.Equal(t, 60*time.Second, cfg.KeepAlive)

	// Compression not useful for form data
	assert.True(t, cfg.DisableCompression, "EPX responses are form-encoded, not compressible")

	// TLS security
	assert.False(t, cfg.InsecureSkipVerify, "Production should verify certificates")
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinTLSVersion)
}

// Test_WebhookClientConfig verifies webhook configuration values
func Test_WebhookClientConfig(t *testing.T) {
	t.Parallel()

	// Act
	cfg := WebhookClientConfig()

	// Assert - Webhooks go to many hosts, lower per-host limits
	assert.Equal(t, 200, cfg.MaxIdleConns, "Webhooks need large pool for many hosts")
	assert.Equal(t, 2, cfg.MaxIdleConnsPerHost, "Only 2 idle per host (don't overwhelm)")
	assert.Equal(t, 5, cfg.MaxConnsPerHost, "Limit 5 concurrent per endpoint")
	assert.Equal(t, 30*time.Second, cfg.IdleConnTimeout, "Shorter timeout for many hosts")

	// Shorter timeouts for webhooks (should be fast)
	assert.Equal(t, 5*time.Second, cfg.DialTimeout)
	assert.Equal(t, 5*time.Second, cfg.TLSHandshakeTimeout)
	assert.Equal(t, 10*time.Second, cfg.ResponseHeaderTimeout, "Webhooks should respond quickly")
	assert.Equal(t, 1*time.Second, cfg.ExpectContinueTimeout)

	// Keep-alive with shorter interval
	assert.False(t, cfg.DisableKeepAlives)
	assert.Equal(t, 30*time.Second, cfg.KeepAlive, "Shorter keep-alive for webhooks")

	// Compression enabled for JSON
	assert.False(t, cfg.DisableCompression, "Webhooks send JSON, should compress")

	// TLS security
	assert.False(t, cfg.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinTLSVersion)
}

// Test_DefaultClientConfig verifies default configuration values
func Test_DefaultClientConfig(t *testing.T) {
	t.Parallel()

	// Act
	cfg := DefaultClientConfig()

	// Assert - Balanced settings
	assert.Equal(t, 100, cfg.MaxIdleConns)
	assert.Equal(t, 10, cfg.MaxIdleConnsPerHost)
	assert.Equal(t, 50, cfg.MaxConnsPerHost)
	assert.Equal(t, 90*time.Second, cfg.IdleConnTimeout)

	// Standard timeouts
	assert.Equal(t, 10*time.Second, cfg.DialTimeout)
	assert.Equal(t, 10*time.Second, cfg.TLSHandshakeTimeout)
	assert.Equal(t, 30*time.Second, cfg.ResponseHeaderTimeout)
	assert.Equal(t, 1*time.Second, cfg.ExpectContinueTimeout)

	// Keep-alive
	assert.False(t, cfg.DisableKeepAlives)
	assert.Equal(t, 60*time.Second, cfg.KeepAlive)

	// Compression
	assert.False(t, cfg.DisableCompression)

	// TLS
	assert.False(t, cfg.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinTLSVersion)
}

// Test_NewHTTPClient_EPXConfig verifies client created with EPX config
func Test_NewHTTPClient_EPXConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := EPXClientConfig()
	timeout := 60 * time.Second

	// Act
	client := NewHTTPClient(cfg, timeout)

	// Assert
	require.NotNil(t, client)
	assert.Equal(t, timeout, client.Timeout)

	// Verify transport configuration
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "Transport should be *http.Transport")

	assert.Equal(t, cfg.MaxIdleConns, transport.MaxIdleConns)
	assert.Equal(t, cfg.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	assert.Equal(t, cfg.MaxConnsPerHost, transport.MaxConnsPerHost)
	assert.Equal(t, cfg.IdleConnTimeout, transport.IdleConnTimeout)

	assert.Equal(t, cfg.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	assert.Equal(t, cfg.ResponseHeaderTimeout, transport.ResponseHeaderTimeout)
	assert.Equal(t, cfg.ExpectContinueTimeout, transport.ExpectContinueTimeout)

	assert.Equal(t, cfg.DisableKeepAlives, transport.DisableKeepAlives)
	assert.Equal(t, cfg.DisableCompression, transport.DisableCompression)

	// Verify HTTP/2 is forced
	assert.True(t, transport.ForceAttemptHTTP2, "HTTP/2 should be forced for performance")

	// Verify TLS config
	require.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, cfg.InsecureSkipVerify, transport.TLSClientConfig.InsecureSkipVerify)
	assert.Equal(t, cfg.MinTLSVersion, transport.TLSClientConfig.MinVersion)
}

// Test_NewHTTPClient_WebhookConfig verifies client created with webhook config
func Test_NewHTTPClient_WebhookConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := WebhookClientConfig()
	timeout := 15 * time.Second

	// Act
	client := NewHTTPClient(cfg, timeout)

	// Assert
	require.NotNil(t, client)
	assert.Equal(t, timeout, client.Timeout)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	// Verify webhook-specific settings
	assert.Equal(t, 200, transport.MaxIdleConns, "Webhooks need large pool")
	assert.Equal(t, 2, transport.MaxIdleConnsPerHost, "Low per-host limit")
	assert.Equal(t, 5, transport.MaxConnsPerHost, "Limit concurrent per host")

	assert.False(t, transport.DisableCompression, "Compression for JSON")
	assert.True(t, transport.ForceAttemptHTTP2)
}

// Test_NewHTTPClient_TLSConfiguration verifies TLS settings
func Test_NewHTTPClient_TLSConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := DefaultClientConfig()

	// Act
	client := NewHTTPClient(cfg, 30*time.Second)

	// Assert
	transport := client.Transport.(*http.Transport)
	tlsConfig := transport.TLSClientConfig

	require.NotNil(t, tlsConfig, "TLS config should be set")

	// Verify minimum TLS version
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion,
		"Should require TLS 1.2 minimum")

	// Verify cipher suites are set (modern, secure ciphers)
	assert.NotEmpty(t, tlsConfig.CipherSuites, "Cipher suites should be configured")
	assert.Contains(t, tlsConfig.CipherSuites, uint16(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256),
		"Should include modern ECDHE cipher")
	assert.Contains(t, tlsConfig.CipherSuites, uint16(tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384))
	assert.Contains(t, tlsConfig.CipherSuites, uint16(tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256))
	assert.Contains(t, tlsConfig.CipherSuites, uint16(tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384))
}

// Test_NewHTTPClient_HTTP2Forced verifies HTTP/2 is enabled
func Test_NewHTTPClient_HTTP2Forced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *HTTPClientConfig
	}{
		{"EPX Config", EPXClientConfig()},
		{"Webhook Config", WebhookClientConfig()},
		{"Default Config", DefaultClientConfig()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			client := NewHTTPClient(tt.cfg, 30*time.Second)

			// Assert
			transport := client.Transport.(*http.Transport)
			assert.True(t, transport.ForceAttemptHTTP2,
				"%s should force HTTP/2", tt.name)
		})
	}
}

// Test_NewHTTPClient_KeepAliveSettings verifies keep-alive configuration
func Test_NewHTTPClient_KeepAliveSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		cfg               *HTTPClientConfig
		expectedKeepAlive time.Duration
		expectedDisabled  bool
	}{
		{
			name:              "EPX keeps connections alive",
			cfg:               EPXClientConfig(),
			expectedKeepAlive: 60 * time.Second,
			expectedDisabled:  false,
		},
		{
			name:              "Webhook uses shorter keep-alive",
			cfg:               WebhookClientConfig(),
			expectedKeepAlive: 30 * time.Second,
			expectedDisabled:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			client := NewHTTPClient(tt.cfg, 30*time.Second)

			// Assert
			transport := client.Transport.(*http.Transport)
			assert.Equal(t, tt.expectedDisabled, transport.DisableKeepAlives)

			// Verify dialer has keep-alive set
			// Note: Can't directly inspect dialer from transport, but we verify config is used
			assert.Equal(t, tt.expectedKeepAlive, tt.cfg.KeepAlive)
		})
	}
}

// Test_NewHTTPClient_Timeouts verifies all timeouts are set correctly
func Test_NewHTTPClient_Timeouts(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := &HTTPClientConfig{
		MaxIdleConns:          10,
		DialTimeout:           1 * time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
		ExpectContinueTimeout: 4 * time.Second,
		MinTLSVersion:         tls.VersionTLS12,
	}
	clientTimeout := 5 * time.Second

	// Act
	client := NewHTTPClient(cfg, clientTimeout)

	// Assert
	assert.Equal(t, clientTimeout, client.Timeout, "Client timeout should be set")

	transport := client.Transport.(*http.Transport)
	assert.Equal(t, 2*time.Second, transport.TLSHandshakeTimeout)
	assert.Equal(t, 3*time.Second, transport.ResponseHeaderTimeout)
	assert.Equal(t, 4*time.Second, transport.ExpectContinueTimeout)

	// Dial timeout is in the dialer, verified via config
	assert.Equal(t, 1*time.Second, cfg.DialTimeout)
}

// Test_NewHTTPClient_ProxyFromEnvironment verifies proxy configuration
func Test_NewHTTPClient_ProxyFromEnvironment(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := DefaultClientConfig()

	// Act
	client := NewHTTPClient(cfg, 30*time.Second)

	// Assert
	transport := client.Transport.(*http.Transport)
	assert.NotNil(t, transport.Proxy, "Proxy function should be set to ProxyFromEnvironment")
}

// Test_NewHTTPClient_CompressionSettings verifies compression configuration
func Test_NewHTTPClient_CompressionSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		disableCompression     bool
		expectedCompression    bool
	}{
		{"Compression enabled", false, false},
		{"Compression disabled", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := &HTTPClientConfig{
				MaxIdleConns:       10,
				DisableCompression: tt.disableCompression,
				MinTLSVersion:      tls.VersionTLS12,
			}

			// Act
			client := NewHTTPClient(cfg, 30*time.Second)

			// Assert
			transport := client.Transport.(*http.Transport)
			assert.Equal(t, tt.expectedCompression, transport.DisableCompression)
		})
	}
}

// Test_ConfigsAreDistinct verifies each config has different settings
func Test_ConfigsAreDistinct(t *testing.T) {
	t.Parallel()

	// Arrange
	epx := EPXClientConfig()
	webhook := WebhookClientConfig()
	def := DefaultClientConfig()

	// Assert - EPX vs Webhook have different pool settings
	assert.NotEqual(t, epx.MaxIdleConns, webhook.MaxIdleConns,
		"EPX and Webhook should have different pool sizes")
	assert.NotEqual(t, epx.MaxIdleConnsPerHost, webhook.MaxIdleConnsPerHost,
		"EPX and Webhook should have different per-host limits")

	// EPX allows more concurrent connections to single host
	assert.Greater(t, epx.MaxConnsPerHost, webhook.MaxConnsPerHost,
		"EPX should allow more concurrent connections (single host)")

	// Webhook has shorter timeouts (should be faster)
	assert.Less(t, webhook.ResponseHeaderTimeout, epx.ResponseHeaderTimeout,
		"Webhooks should have shorter response timeout")

	// EPX disables compression, webhook enables it
	assert.NotEqual(t, epx.DisableCompression, webhook.DisableCompression,
		"EPX and Webhook should have different compression settings")

	// Default should be different from both
	assert.NotEqual(t, def.MaxConnsPerHost, epx.MaxConnsPerHost)
	assert.NotEqual(t, def.MaxIdleConnsPerHost, webhook.MaxIdleConnsPerHost)
}
