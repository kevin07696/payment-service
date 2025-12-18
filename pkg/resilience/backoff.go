package resilience

import (
	"math"
	"math/rand"
	"time"
)

// BackoffStrategy defines retry backoff behavior
type BackoffStrategy interface {
	NextDelay(attempt int) time.Duration
}

// ExponentialBackoff implements exponential backoff with jitter
// This prevents thundering herd by spreading retry attempts over time
type ExponentialBackoff struct {
	BaseDelay  time.Duration // Initial delay (e.g., 100ms)
	MaxDelay   time.Duration // Maximum delay (e.g., 30s)
	Multiplier float64       // Exponential multiplier (typically 2.0)
	Jitter     float64       // Jitter factor (0.0-1.0, typically 0.1 for ±10%)
}

// DefaultExponentialBackoff returns sensible defaults for EPX gateway retries
//
// Retry sequence with defaults (±10% jitter):
//   - Attempt 0: ~100ms (90-110ms)
//   - Attempt 1: ~200ms (180-220ms)
//   - Attempt 2: ~400ms (360-440ms)
//   - Attempt 3: ~800ms (720-880ms)
//   - Attempt 4: ~1.6s (1.4-1.8s)
//   - Attempt 5: ~3.2s (2.9-3.5s)
func DefaultExponentialBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1, // ±10% jitter
	}
}

// WebhookBackoff returns backoff configuration optimized for webhook retries
//
// Retry sequence (±10% jitter):
//   - Attempt 0: ~1s
//   - Attempt 1: ~2s
//   - Attempt 2: ~4s
//   - Attempt 3: ~8s
//   - Attempt 4: ~16s
//   - Attempt 5+: ~30s (capped)
func WebhookBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1,
	}
}

// NextDelay calculates the delay for the given attempt number (0-indexed)
//
// The delay is calculated as: BaseDelay * (Multiplier ^ attempt) ± jitter
// The result is capped at MaxDelay to prevent excessive delays
func (eb *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	if attempt < 0 {
		return eb.BaseDelay
	}

	// Calculate exponential delay: BaseDelay * (Multiplier ^ attempt)
	delay := float64(eb.BaseDelay) * math.Pow(eb.Multiplier, float64(attempt))

	// Cap at MaxDelay
	if delay > float64(eb.MaxDelay) {
		delay = float64(eb.MaxDelay)
	}

	// Add jitter: delay ± (delay * jitter)
	// This spreads retry attempts over time to prevent thundering herd
	jitterAmount := delay * eb.Jitter
	jitter := (rand.Float64()*2 - 1) * jitterAmount // Random value in [-jitterAmount, +jitterAmount]

	finalDelay := time.Duration(delay + jitter)

	// Ensure non-negative
	if finalDelay < 0 {
		finalDelay = eb.BaseDelay
	}

	return finalDelay
}

// FixedBackoff implements a simple fixed delay backoff (for backward compatibility)
type FixedBackoff struct {
	Delay time.Duration
}

// NextDelay returns the fixed delay regardless of attempt number
func (fb *FixedBackoff) NextDelay(attempt int) time.Duration {
	return fb.Delay
}
