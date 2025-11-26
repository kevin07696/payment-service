package testutil_test

import (
	"context"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/pkg/testutil"
)

// TestVerifyNoGoroutineLeaks_NoLeak verifies that goleak detection works
// when no goroutines are leaked
func TestVerifyNoGoroutineLeaks_NoLeak(t *testing.T) {
	defer testutil.VerifyNoGoroutineLeaks(t)()

	// Test code that doesn't leak goroutines
	done := make(chan bool)
	go func() {
		time.Sleep(10 * time.Millisecond)
		done <- true
	}()
	<-done // Wait for goroutine to complete
}

// TestVerifyNoGoroutineLeaks_WithLeak is commented out because it intentionally fails
// Uncomment to verify that goleak catches goroutine leaks
/*
func TestVerifyNoGoroutineLeaks_WithLeak(t *testing.T) {
	defer testutil.VerifyNoGoroutineLeaks(t)()

	// This goroutine is never cleaned up - goleak should catch it
	go func() {
		for {
			time.Sleep(time.Second)
		}
	}()
}
*/

// TestGoleakBasicFunctionality demonstrates goleak working
func TestGoleakBasicFunctionality(t *testing.T) {
	defer testutil.VerifyNoGoroutineLeaks(t)()

	// Spawn goroutine and properly clean it up
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan bool)
	go func() {
		<-ctx.Done()
		done <- true
	}()

	// Cancel context to stop goroutine
	cancel()
	<-done // Wait for cleanup
}
