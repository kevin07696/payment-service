package testutil

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts int           // Maximum number of attempts (default: 10)
	InitialWait time.Duration // Initial wait between attempts (default: 100ms)
	MaxWait     time.Duration // Maximum wait between attempts (default: 5s)
	Multiplier  float64       // Wait time multiplier (default: 2.0)
}

// DefaultRetryConfig returns sensible defaults for integration tests
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 10,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     5 * time.Second,
		Multiplier:  2.0,
	}
}

// QuickRetryConfig returns config for fast-responding operations
func QuickRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 5,
		InitialWait: 50 * time.Millisecond,
		MaxWait:     500 * time.Millisecond,
		Multiplier:  1.5,
	}
}

// SlowRetryConfig returns config for slow operations (external APIs, DB heavy)
func SlowRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 15,
		InitialWait: 500 * time.Millisecond,
		MaxWait:     10 * time.Second,
		Multiplier:  2.0,
	}
}

// ErrMaxAttemptsReached is returned when all retry attempts are exhausted
var ErrMaxAttemptsReached = errors.New("max retry attempts reached")

// RetryUntil retries the given function until it returns true or max attempts reached
// Use this instead of time.Sleep() for polling operations
//
// Example:
//
//	err := testutil.RetryUntil(t, "wait for transaction", testutil.DefaultRetryConfig(), func() (bool, error) {
//	    tx, err := client.GetTransaction(txID)
//	    if err != nil {
//	        return false, nil // Keep retrying
//	    }
//	    return tx.Status == "approved", nil
//	})
func RetryUntil(t *testing.T, description string, cfg RetryConfig, fn func() (done bool, err error)) error {
	t.Helper()

	if cfg.MaxAttempts == 0 {
		cfg = DefaultRetryConfig()
	}

	wait := cfg.InitialWait
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		done, err := fn()
		if err != nil {
			lastErr = err
			t.Logf("%s: attempt %d/%d failed: %v", description, attempt, cfg.MaxAttempts, err)
		} else if done {
			if attempt > 1 {
				t.Logf("%s: succeeded on attempt %d", description, attempt)
			}
			return nil
		}

		if attempt < cfg.MaxAttempts {
			time.Sleep(wait)
			wait = time.Duration(float64(wait) * cfg.Multiplier)
			if wait > cfg.MaxWait {
				wait = cfg.MaxWait
			}
		}
	}

	if lastErr != nil {
		return fmt.Errorf("%s: %w (last error: %v)", description, ErrMaxAttemptsReached, lastErr)
	}
	return fmt.Errorf("%s: %w", description, ErrMaxAttemptsReached)
}

// RetryUntilWithContext is like RetryUntil but respects context cancellation
func RetryUntilWithContext(ctx context.Context, t *testing.T, description string, cfg RetryConfig, fn func() (done bool, err error)) error {
	t.Helper()

	if cfg.MaxAttempts == 0 {
		cfg = DefaultRetryConfig()
	}

	wait := cfg.InitialWait
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", description, ctx.Err())
		default:
		}

		done, err := fn()
		if err != nil {
			lastErr = err
			t.Logf("%s: attempt %d/%d failed: %v", description, attempt, cfg.MaxAttempts, err)
		} else if done {
			if attempt > 1 {
				t.Logf("%s: succeeded on attempt %d", description, attempt)
			}
			return nil
		}

		if attempt < cfg.MaxAttempts {
			select {
			case <-ctx.Done():
				return fmt.Errorf("%s: %w", description, ctx.Err())
			case <-time.After(wait):
			}
			wait = time.Duration(float64(wait) * cfg.Multiplier)
			if wait > cfg.MaxWait {
				wait = cfg.MaxWait
			}
		}
	}

	if lastErr != nil {
		return fmt.Errorf("%s: %w (last error: %v)", description, ErrMaxAttemptsReached, lastErr)
	}
	return fmt.Errorf("%s: %w", description, ErrMaxAttemptsReached)
}

// WaitForCondition is a simpler version of RetryUntil that just waits for a condition
// Returns error if condition is never met within timeout
//
// Example:
//
//	err := testutil.WaitForCondition(t, "server ready", 10*time.Second, func() bool {
//	    resp, err := http.Get(serverURL + "/health")
//	    return err == nil && resp.StatusCode == 200
//	})
func WaitForCondition(t *testing.T, description string, timeout time.Duration, condition func() bool) error {
	t.Helper()

	deadline := time.Now().Add(timeout)
	interval := 100 * time.Millisecond

	for time.Now().Before(deadline) {
		if condition() {
			return nil
		}
		time.Sleep(interval)
		// Exponential backoff up to 1 second
		if interval < time.Second {
			interval = time.Duration(float64(interval) * 1.5)
		}
	}

	return fmt.Errorf("%s: condition not met within %v", description, timeout)
}

// WaitForServer waits for an HTTP server to be ready (returns 200 on health endpoint)
func WaitForServer(t *testing.T, healthURL string, timeout time.Duration) error {
	t.Helper()

	return WaitForCondition(t, "server ready at "+healthURL, timeout, func() bool {
		client := NewClient("")
		resp, err := client.HTTPClient.Get(healthURL)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == 200
	})
}

// WaitForDatabase waits for a database to be ready
func WaitForDatabase(t *testing.T, timeout time.Duration) error {
	t.Helper()

	return WaitForCondition(t, "database ready", timeout, func() bool {
		db := GetDB(t)
		if db == nil {
			return false
		}
		return db.Ping() == nil
	})
}

// MustWait is a helper that fails the test if waiting fails
// Use this when the wait is critical and failure should stop the test
//
// Example:
//
//	testutil.MustWait(t, testutil.WaitForServer(t, cfg.ServiceURL+"/health", 30*time.Second))
func MustWait(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
}
