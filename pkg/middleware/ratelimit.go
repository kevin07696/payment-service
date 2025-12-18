package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting functionality with O(1) LRU eviction
type RateLimiter struct {
	cache          *lru.Cache[string, *rate.Limiter]
	trustedProxies map[string]bool // Trusted proxy IPs that can set X-Forwarded-For
	proxyMu        sync.RWMutex    // Protects trustedProxies map
	rate           rate.Limit
	burst          int
}

// NewRateLimiter creates a new rate limiter
// requestsPerSecond: max requests per second per IP
// burst: max burst size
// trustedProxies: list of IP addresses allowed to set X-Forwarded-For header
func NewRateLimiter(requestsPerSecond float64, burst int, trustedProxies []string) *RateLimiter {
	// Build trusted proxies map for O(1) lookup
	proxyMap := make(map[string]bool)
	for _, proxy := range trustedProxies {
		proxyMap[strings.TrimSpace(proxy)] = true
	}

	// Create LRU cache with max 10k entries
	// The cache handles eviction automatically in O(1) time
	cache, _ := lru.New[string, *rate.Limiter](10000)

	return &RateLimiter{
		cache:          cache,
		trustedProxies: proxyMap,
		rate:           rate.Limit(requestsPerSecond),
		burst:          burst,
	}
}

// Shutdown is a no-op since LRU cache doesn't need cleanup goroutines.
// Kept for backward compatibility with existing code.
func (rl *RateLimiter) Shutdown() {
	// No-op: LRU cache handles cleanup automatically
}

// getLimiter returns the rate limiter for the given IP
// Uses O(1) LRU cache operations instead of O(n) eviction
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	// Try to get existing limiter (also updates LRU order)
	if limiter, ok := rl.cache.Get(ip); ok {
		return limiter
	}

	// Create new limiter and add to cache
	// LRU cache handles eviction automatically when capacity is reached
	limiter := rate.NewLimiter(rl.rate, rl.burst)
	rl.cache.Add(ip, limiter)

	return limiter
}

// getClientIP extracts the real client IP from the request
// Security: Only trusts X-Forwarded-For header when request comes from a trusted proxy.
// This prevents IP spoofing attacks where attackers set X-Forwarded-For to bypass rate limiting.
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	// Get the direct connection IP first
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr doesn't have port, use as-is
		remoteIP = r.RemoteAddr
	}

	// Only trust proxy headers if the request comes from a trusted proxy
	rl.proxyMu.RLock()
	trusted := len(rl.trustedProxies) > 0 && rl.trustedProxies[remoteIP]
	rl.proxyMu.RUnlock()

	if trusted {
		// Check X-Forwarded-For header (set by proxies/load balancers)
		// Format: "client, proxy1, proxy2" - we want the first (original client) IP
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}

		// Check X-Real-IP header (alternative proxy header)
		xri := r.Header.Get("X-Real-IP")
		if xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	// Return direct connection IP (either no proxy or untrusted proxy)
	return remoteIP
}

// Middleware returns HTTP middleware that applies rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP (only trusts proxy headers from trusted proxies)
		ip := rl.getClientIP(r)

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HTTPHandlerFunc wraps a handler function with rate limiting
func (rl *RateLimiter) HTTPHandlerFunc(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get client IP (only trusts proxy headers from trusted proxies)
		ip := rl.getClientIP(r)

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		handler(w, r)
	}
}

// HTTPHandler wraps an http.Handler with rate limiting
func (rl *RateLimiter) HTTPHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP (only trusts proxy headers from trusted proxies)
		ip := rl.getClientIP(r)

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		handler.ServeHTTP(w, r)
	})
}
