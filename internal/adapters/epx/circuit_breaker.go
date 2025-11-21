package epx

import (
	"errors"
	"sync"
	"time"
)

// CircuitState represents the current state of the circuit breaker
type CircuitState int

const (
	// StateClosed - Circuit is closed, requests flow normally
	StateClosed CircuitState = iota
	// StateOpen - Circuit is open, requests fail immediately
	StateOpen
	// StateHalfOpen - Circuit is testing if service recovered
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	// ErrCircuitOpen is returned when circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrTooManyRequests is returned when too many requests in half-open state
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	// MaxFailures is the number of failures before opening circuit
	MaxFailures uint32
	// Timeout is how long to wait before transitioning from open to half-open
	Timeout time.Duration
	// MaxRequestsHalfOpen is max concurrent requests allowed in half-open state
	MaxRequestsHalfOpen uint32
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		MaxFailures:         5,               // Open after 5 consecutive failures
		Timeout:             30 * time.Second, // Try again after 30 seconds
		MaxRequestsHalfOpen: 1,               // Allow 1 test request in half-open
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu                  sync.RWMutex
	state               CircuitState
	failures            uint32
	successes           uint32
	requestsHalfOpen    uint32
	lastStateChangeTime time.Time
	config              CircuitBreakerConfig
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:               StateClosed,
		lastStateChangeTime: time.Now(),
		config:              config,
	}
}

// Call executes the given function if circuit breaker allows it
func (cb *CircuitBreaker) Call(fn func() error) error {
	// Check if we can make the call
	if err := cb.beforeCall(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.afterCall(err)

	return err
}

// beforeCall checks if circuit allows the request
func (cb *CircuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Allow request
		return nil

	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.lastStateChangeTime) > cb.config.Timeout {
			// Transition to half-open
			cb.setState(StateHalfOpen)
			cb.requestsHalfOpen++
			return nil
		}
		// Circuit still open
		return ErrCircuitOpen

	case StateHalfOpen:
		// Check if we've exceeded max requests in half-open
		if cb.requestsHalfOpen >= cb.config.MaxRequestsHalfOpen {
			return ErrTooManyRequests
		}
		cb.requestsHalfOpen++
		return nil

	default:
		return ErrCircuitOpen
	}
}

// afterCall records the result and updates circuit state
func (cb *CircuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		// Request failed
		cb.onFailure()
	} else {
		// Request succeeded
		cb.onSuccess()
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.failures++

	switch cb.state {
	case StateClosed:
		// Check if we've reached failure threshold
		if cb.failures >= cb.config.MaxFailures {
			cb.setState(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in half-open goes back to open
		cb.setState(StateOpen)
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	cb.successes++

	switch cb.state {
	case StateHalfOpen:
		// Success in half-open state closes the circuit
		cb.setState(StateClosed)

	case StateClosed:
		// Reset failure counter on success
		cb.failures = 0
	}
}

// setState transitions to a new state
func (cb *CircuitBreaker) setState(newState CircuitState) {
	if cb.state == newState {
		return
	}

	cb.state = newState
	cb.lastStateChangeTime = time.Now()

	// Reset counters on state change
	switch newState {
	case StateClosed:
		cb.failures = 0
		cb.successes = 0
		cb.requestsHalfOpen = 0

	case StateOpen:
		cb.requestsHalfOpen = 0

	case StateHalfOpen:
		cb.failures = 0
		cb.successes = 0
		cb.requestsHalfOpen = 0
	}
}

// State returns the current circuit state (thread-safe)
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Failures returns the current failure count (thread-safe)
func (cb *CircuitBreaker) Failures() uint32 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Successes returns the current success count (thread-safe)
func (cb *CircuitBreaker) Successes() uint32 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.successes
}

// Reset resets the circuit breaker to closed state (useful for testing)
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.requestsHalfOpen = 0
	cb.lastStateChangeTime = time.Now()
}
