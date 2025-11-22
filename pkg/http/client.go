package http

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// HTTPClientConfig holds HTTP client configuration
// Optimized for different service patterns (EPX, webhooks, etc.)
type HTTPClientConfig struct {
	// Connection pooling
	MaxIdleConns        int           // Total idle connections across all hosts
	MaxIdleConnsPerHost int           // Idle connections per host
	MaxConnsPerHost     int           // Maximum connections per host (including active)
	IdleConnTimeout     time.Duration // How long idle connections stay alive

	// Timeouts
	DialTimeout           time.Duration // TCP connection timeout
	TLSHandshakeTimeout   time.Duration // TLS handshake timeout
	ResponseHeaderTimeout time.Duration // Waiting for response headers
	ExpectContinueTimeout time.Duration // 100-continue timeout

	// Keep-alive
	DisableKeepAlives bool
	KeepAlive         time.Duration

	// Compression
	DisableCompression bool

	// TLS
	InsecureSkipVerify bool
	MinTLSVersion      uint16
}

// EPXClientConfig returns optimized config for EPX gateway
// EPX is a single host - tune pool for high concurrency to one endpoint
func EPXClientConfig() *HTTPClientConfig {
	return &HTTPClientConfig{
		// EPX is single host - tune for it
		MaxIdleConns:        50,  // Total pool size
		MaxIdleConnsPerHost: 50,  // All for EPX host
		MaxConnsPerHost:     100, // Allow 100 concurrent to EPX
		IdleConnTimeout:     90 * time.Second,

		// Timeouts tuned for payment gateway
		DialTimeout:           10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second, // EPX can be slow
		ExpectContinueTimeout: 1 * time.Second,

		// Keep-alive
		DisableKeepAlives: false,
		KeepAlive:         60 * time.Second,

		// Compression (EPX responses are form-encoded, not JSON)
		DisableCompression: true, // Not useful for form data

		// TLS
		InsecureSkipVerify: false, // Production should verify
		MinTLSVersion:      tls.VersionTLS12,
	}
}

// WebhookClientConfig returns optimized config for webhook delivery
// Webhooks go to many different hosts - tune for broad distribution
func WebhookClientConfig() *HTTPClientConfig {
	return &HTTPClientConfig{
		// Webhooks go to many different hosts
		MaxIdleConns:        200, // Large pool for many hosts
		MaxIdleConnsPerHost: 2,   // Only 2 per host (don't overwhelm endpoints)
		MaxConnsPerHost:     5,   // Limit concurrent per endpoint
		IdleConnTimeout:     30 * time.Second, // Short timeout (many hosts)

		// Timeouts tuned for webhooks
		DialTimeout:           5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second, // Webhooks should be fast
		ExpectContinueTimeout: 1 * time.Second,

		// Keep-alive
		DisableKeepAlives: false,
		KeepAlive:         30 * time.Second, // Shorter for webhooks

		// Compression
		DisableCompression: false, // Webhooks send JSON

		// TLS
		InsecureSkipVerify: false,
		MinTLSVersion:      tls.VersionTLS12,
	}
}

// DefaultClientConfig returns a balanced configuration for general use
func DefaultClientConfig() *HTTPClientConfig {
	return &HTTPClientConfig{
		// Balanced settings
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,

		// Standard timeouts
		DialTimeout:           10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,

		// Keep-alive
		DisableKeepAlives: false,
		KeepAlive:         60 * time.Second,

		// Compression
		DisableCompression: false,

		// TLS
		InsecureSkipVerify: false,
		MinTLSVersion:      tls.VersionTLS12,
	}
}

// NewHTTPClient creates an HTTP client with the given configuration
// Optimized for HTTP/2 with connection pooling and keep-alive
func NewHTTPClient(cfg *HTTPClientConfig, timeout time.Duration) *http.Client {
	// Create dialer with keep-alive
	dialer := &net.Dialer{
		Timeout:   cfg.DialTimeout,
		KeepAlive: cfg.KeepAlive,
		// Enable TCP keep-alive probes
		// Detects broken connections faster
	}

	// Create transport with optimized settings
	transport := &http.Transport{
		Proxy:       http.ProxyFromEnvironment,
		DialContext: dialer.DialContext,

		// Connection pooling - critical for performance
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,

		// Timeouts
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		ExpectContinueTimeout: cfg.ExpectContinueTimeout,

		// Keep-alive - reuse connections
		DisableKeepAlives: cfg.DisableKeepAlives,

		// Compression
		DisableCompression: cfg.DisableCompression,

		// TLS configuration
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			MinVersion:         cfg.MinTLSVersion,
			// Prefer modern cipher suites
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			},
		},

		// Force HTTP/2 for better performance
		ForceAttemptHTTP2: true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		// CheckRedirect can be configured if needed
	}
}
