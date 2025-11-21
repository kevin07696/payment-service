package epx

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	if config.MaxFailures != 5 {
		t.Errorf("Expected MaxFailures = 5, got %d", config.MaxFailures)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout = 30s, got %v", config.Timeout)
	}

	if config.MaxRequestsHalfOpen != 1 {
		t.Errorf("Expected MaxRequestsHalfOpen = 1, got %d", config.MaxRequestsHalfOpen)
	}
}

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	if cb.State() != StateClosed {
		t.Errorf("Expected initial state = closed, got %v", cb.State())
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected failures = 0, got %d", cb.Failures())
	}

	if cb.Successes() != 0 {
		t.Errorf("Expected successes = 0, got %d", cb.Successes())
	}
}

func TestCircuitBreaker_SuccessfulCalls(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	// Make 10 successful calls
	for i := 0; i < 10; i++ {
		err := cb.Call(func() error {
			return nil
		})

		if err != nil {
			t.Fatalf("Expected success, got error: %v", err)
		}
	}

	// Circuit should remain closed
	if cb.State() != StateClosed {
		t.Errorf("Expected state = closed after successes, got %v", cb.State())
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected failures = 0, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_TransitionToOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             1 * time.Second,
		MaxRequestsHalfOpen: 1,
	}
	cb := NewCircuitBreaker(config)

	// Make 3 failed calls (should open circuit)
	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		err := cb.Call(func() error {
			return testErr
		})

		if err != testErr {
			t.Fatalf("Expected test error, got: %v", err)
		}
	}

	// Circuit should be open now
	if cb.State() != StateOpen {
		t.Errorf("Expected state = open after %d failures, got %v", config.MaxFailures, cb.State())
	}

	// Next call should fail immediately without executing function
	executed := false
	err := cb.Call(func() error {
		executed = true
		return nil
	})

	if err != ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}

	if executed {
		t.Error("Function should not execute when circuit is open")
	}
}

func TestCircuitBreaker_TransitionToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             100 * time.Millisecond,
		MaxRequestsHalfOpen: 1,
	}
	cb := NewCircuitBreaker(config)

	// Trigger circuit open
	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		_ = cb.Call(func() error { return testErr })
	}

	if cb.State() != StateOpen {
		t.Fatalf("Circuit should be open, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next call should transition to half-open
	err := cb.Call(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected success in half-open, got %v", err)
	}

	// Circuit should be closed after successful half-open call
	if cb.State() != StateClosed {
		t.Errorf("Expected state = closed after half-open success, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             100 * time.Millisecond,
		MaxRequestsHalfOpen: 1,
	}
	cb := NewCircuitBreaker(config)

	// Trigger circuit open
	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		_ = cb.Call(func() error { return testErr })
	}

	// Wait for timeout to reach half-open
	time.Sleep(150 * time.Millisecond)

	// Fail in half-open state (should go back to open)
	err := cb.Call(func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected test error, got %v", err)
	}

	// Circuit should be open again
	if cb.State() != StateOpen {
		t.Errorf("Expected state = open after half-open failure, got %v", cb.State())
	}
}

func TestCircuitBreaker_MaxRequestsHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             100 * time.Millisecond,
		MaxRequestsHalfOpen: 2,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error { return testErr })
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// First two calls should be allowed in half-open
	for i := 0; i < 2; i++ {
		err := cb.beforeCall()
		if err != nil {
			t.Errorf("Call %d should be allowed in half-open, got error: %v", i+1, err)
		}
	}

	// Third call should be rejected
	err := cb.beforeCall()
	if err != ErrTooManyRequests {
		t.Errorf("Expected ErrTooManyRequests for 3rd call in half-open, got %v", err)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             1 * time.Second,
		MaxRequestsHalfOpen: 1,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error { return testErr })
	}

	if cb.State() != StateOpen {
		t.Fatalf("Circuit should be open, got %v", cb.State())
	}

	// Reset circuit
	cb.Reset()

	// Circuit should be closed with zero counters
	if cb.State() != StateClosed {
		t.Errorf("Expected state = closed after reset, got %v", cb.State())
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected failures = 0 after reset, got %d", cb.Failures())
	}

	if cb.Successes() != 0 {
		t.Errorf("Expected successes = 0 after reset, got %d", cb.Successes())
	}
}

func TestCircuitBreaker_StateString(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(999), "unknown"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("State %d: expected %q, got %q", tt.state, tt.expected, tt.state.String())
		}
	}
}

func TestCircuitBreaker_ConcurrentCalls(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         10,
		Timeout:             1 * time.Second,
		MaxRequestsHalfOpen: 5,
	}
	cb := NewCircuitBreaker(config)

	// Make 100 concurrent successful calls
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			err := cb.Call(func() error {
				time.Sleep(1 * time.Millisecond)
				return nil
			})
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Circuit should still be closed
	if cb.State() != StateClosed {
		t.Errorf("Expected state = closed after concurrent calls, got %v", cb.State())
	}
}

func TestCircuitBreaker_FailureCounterReset(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             1 * time.Second,
		MaxRequestsHalfOpen: 1,
	}
	cb := NewCircuitBreaker(config)

	// Make 2 failed calls
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error { return testErr })
	}

	if cb.Failures() != 2 {
		t.Fatalf("Expected failures = 2, got %d", cb.Failures())
	}

	// Make a successful call (should reset failure counter)
	err := cb.Call(func() error { return nil })
	if err != nil {
		t.Fatalf("Expected success, got %v", err)
	}

	if cb.Failures() != 0 {
		t.Errorf("Expected failures = 0 after success, got %d", cb.Failures())
	}

	// Circuit should still be closed
	if cb.State() != StateClosed {
		t.Errorf("Expected state = closed, got %v", cb.State())
	}
}
