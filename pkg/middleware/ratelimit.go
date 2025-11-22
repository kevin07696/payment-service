package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipLimiter tracks a rate limiter and its last access time
type ipLimiter struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// RateLimiter provides rate limiting functionality with automatic cleanup
type RateLimiter struct {
	limiters  map[string]*ipLimiter
	mu        sync.RWMutex
	rate      rate.Limit
	burst     int
	maxSize   int           // Maximum number of IP limiters to cache
	cleanupInterval time.Duration // How often to cleanup stale entries
	stopCh    chan struct{} // Channel to signal cleanup goroutine shutdown
}

// NewRateLimiter creates a new rate limiter
// requestsPerSecond: max requests per second per IP
// burst: max burst size
func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters:        make(map[string]*ipLimiter),
		rate:            rate.Limit(requestsPerSecond),
		burst:           burst,
		maxSize:         10000,              // Max 10k unique IPs in cache
		cleanupInterval: 5 * time.Minute,    // Cleanup every 5 minutes
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// cleanupLoop periodically removes stale entries from the rate limiter cache
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

// cleanup removes entries that haven't been accessed in the last cleanup interval
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.cleanupInterval)
	removed := 0

	for ip, limiter := range rl.limiters {
		if limiter.lastAccess.Before(cutoff) {
			delete(rl.limiters, ip)
			removed++
		}
	}

	// Log cleanup if significant (more than 100 entries removed)
	if removed > 100 {
		// Note: Would log here if logger was available
		// For now, this is a silent cleanup
	}
}

// Shutdown stops the cleanup goroutine
func (rl *RateLimiter) Shutdown() {
	close(rl.stopCh)
}

// getLimiter returns the rate limiter for the given IP
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Check if limiter exists
	limiter, exists := rl.limiters[ip]
	if exists {
		// Update last access time
		limiter.lastAccess = time.Now()
		return limiter.limiter
	}

	// Check if map is at capacity
	if len(rl.limiters) >= rl.maxSize {
		// Evict oldest entry (LRU)
		var oldestIP string
		var oldestTime time.Time
		first := true

		for ip, lim := range rl.limiters {
			if first || lim.lastAccess.Before(oldestTime) {
				oldestIP = ip
				oldestTime = lim.lastAccess
				first = false
			}
		}

		if oldestIP != "" {
			delete(rl.limiters, oldestIP)
		}
	}

	// Create new limiter
	newLimiter := &ipLimiter{
		limiter:    rate.NewLimiter(rl.rate, rl.burst),
		lastAccess: time.Now(),
	}
	rl.limiters[ip] = newLimiter

	return newLimiter.limiter
}

// Middleware returns HTTP middleware that applies rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		ip := r.RemoteAddr

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
		ip := r.RemoteAddr

		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		handler(w, r)
	}
}
