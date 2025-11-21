package resilience

import (
	"testing"
	"time"
)

func TestDefaultExponentialBackoff(t *testing.T) {
	backoff := DefaultExponentialBackoff()

	// Verify configuration
	if backoff.BaseDelay != 100*time.Millisecond {
		t.Errorf("Expected BaseDelay = 100ms, got %v", backoff.BaseDelay)
	}

	if backoff.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay = 30s, got %v", backoff.MaxDelay)
	}

	if backoff.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier = 2.0, got %f", backoff.Multiplier)
	}

	if backoff.Jitter != 0.1 {
		t.Errorf("Expected Jitter = 0.1, got %f", backoff.Jitter)
	}
}

func TestExponentialBackoff_NextDelay(t *testing.T) {
	backoff := &ExponentialBackoff{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.0, // No jitter for predictable testing
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},  // 100ms * 2^0 = 100ms
		{1, 200 * time.Millisecond},  // 100ms * 2^1 = 200ms
		{2, 400 * time.Millisecond},  // 100ms * 2^2 = 400ms
		{3, 800 * time.Millisecond},  // 100ms * 2^3 = 800ms
		{4, 1600 * time.Millisecond}, // 100ms * 2^4 = 1600ms
		{5, 3200 * time.Millisecond}, // 100ms * 2^5 = 3200ms
		{6, 6400 * time.Millisecond}, // 100ms * 2^6 = 6400ms
		{7, 10 * time.Second},        // 100ms * 2^7 = 12800ms, capped at 10s
		{10, 10 * time.Second},       // Capped at MaxDelay
	}

	for _, tt := range tests {
		delay := backoff.NextDelay(tt.attempt)
		if delay != tt.expected {
			t.Errorf("NextDelay(%d) = %v, want %v", tt.attempt, delay, tt.expected)
		}
	}
}

func TestExponentialBackoff_WithJitter(t *testing.T) {
	backoff := &ExponentialBackoff{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1, // ±10% jitter
	}

	// Test that jitter creates variance
	attempt := 3
	delays := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		delays[i] = backoff.NextDelay(attempt)
	}

	// Expected delay for attempt 3: 800ms
	// With ±10% jitter: 720ms - 880ms
	expectedDelay := 800 * time.Millisecond
	minExpected := time.Duration(float64(expectedDelay) * 0.9)  // 720ms
	maxExpected := time.Duration(float64(expectedDelay) * 1.1)  // 880ms

	// Check all delays are within jitter range
	for i, delay := range delays {
		if delay < minExpected || delay > maxExpected {
			t.Errorf("Delay[%d] = %v, expected range [%v, %v]", i, delay, minExpected, maxExpected)
		}
	}

	// Check that delays have variance (not all the same)
	allSame := true
	firstDelay := delays[0]
	for _, delay := range delays[1:] {
		if delay != firstDelay {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("All delays are identical - jitter is not working")
	}
}

func TestExponentialBackoff_NegativeAttempt(t *testing.T) {
	backoff := DefaultExponentialBackoff()

	delay := backoff.NextDelay(-1)
	if delay != backoff.BaseDelay {
		t.Errorf("NextDelay(-1) = %v, want %v", delay, backoff.BaseDelay)
	}
}

func TestExponentialBackoff_MaxDelayCap(t *testing.T) {
	backoff := &ExponentialBackoff{
		BaseDelay:  1 * time.Second,
		MaxDelay:   5 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.0,
	}

	// Attempt 3: 1s * 2^3 = 8s, should be capped at 5s
	delay := backoff.NextDelay(3)
	if delay != 5*time.Second {
		t.Errorf("NextDelay(3) = %v, want %v (capped at MaxDelay)", delay, 5*time.Second)
	}

	// Attempt 10: Should still be capped at 5s
	delay = backoff.NextDelay(10)
	if delay != 5*time.Second {
		t.Errorf("NextDelay(10) = %v, want %v (capped at MaxDelay)", delay, 5*time.Second)
	}
}

func TestWebhookBackoff(t *testing.T) {
	backoff := WebhookBackoff()

	// Verify configuration optimized for webhooks
	if backoff.BaseDelay != 1*time.Second {
		t.Errorf("Expected BaseDelay = 1s, got %v", backoff.BaseDelay)
	}

	if backoff.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay = 30s, got %v", backoff.MaxDelay)
	}

	// Test webhook retry sequence (no jitter)
	backoff.Jitter = 0.0
	expected := []time.Duration{
		1 * time.Second,  // Attempt 0
		2 * time.Second,  // Attempt 1
		4 * time.Second,  // Attempt 2
		8 * time.Second,  // Attempt 3
		16 * time.Second, // Attempt 4
		30 * time.Second, // Attempt 5 (capped)
	}

	for attempt, expectedDelay := range expected {
		delay := backoff.NextDelay(attempt)
		if delay != expectedDelay {
			t.Errorf("Webhook NextDelay(%d) = %v, want %v", attempt, delay, expectedDelay)
		}
	}
}

func TestFixedBackoff(t *testing.T) {
	backoff := &FixedBackoff{
		Delay: 1 * time.Second,
	}

	// All attempts should return the same delay
	for attempt := 0; attempt < 10; attempt++ {
		delay := backoff.NextDelay(attempt)
		if delay != 1*time.Second {
			t.Errorf("FixedBackoff.NextDelay(%d) = %v, want 1s", attempt, delay)
		}
	}
}

func TestExponentialBackoff_RealWorldScenario(t *testing.T) {
	// Simulate EPX gateway retry scenario
	backoff := DefaultExponentialBackoff()

	t.Log("Simulating EPX gateway retry scenario with exponential backoff:")

	totalDelay := time.Duration(0)
	for attempt := 0; attempt < 6; attempt++ {
		delay := backoff.NextDelay(attempt)
		totalDelay += delay

		t.Logf("  Attempt %d: delay = %v, cumulative = %v",
			attempt, delay, totalDelay)
	}

	// With exponential backoff, total delay for 6 attempts should be manageable
	// Approximate: 100ms + 200ms + 400ms + 800ms + 1600ms + 3200ms ≈ 6.3s
	// With jitter, could vary ±10%
	if totalDelay > 10*time.Second {
		t.Errorf("Total delay %v exceeds reasonable threshold", totalDelay)
	}
}

// Benchmark different backoff strategies
func BenchmarkExponentialBackoff(b *testing.B) {
	backoff := DefaultExponentialBackoff()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backoff.NextDelay(i % 10)
	}
}

func BenchmarkFixedBackoff(b *testing.B) {
	backoff := &FixedBackoff{Delay: 1 * time.Second}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backoff.NextDelay(i % 10)
	}
}
