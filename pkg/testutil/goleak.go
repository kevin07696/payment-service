package testutil

import (
	"testing"

	"go.uber.org/goleak"
)

// VerifyNoGoroutineLeaks verifies that no goroutines were leaked during the test.
// This should be called with defer at the beginning of tests that create goroutines.
//
// Usage:
//
//	func TestMyFunction(t *testing.T) {
//		defer testutil.VerifyNoGoroutineLeaks(t)()
//		// Test code that may spawn goroutines...
//	}
//
// P2-6: Goroutine Leak Detection
// This prevents resource leaks by ensuring all goroutines are properly cleaned up.
func VerifyNoGoroutineLeaks(t *testing.T) func() {
	// Ignore known persistent goroutines from dependencies
	opts := []goleak.Option{
		// Ignore goleak's own goroutines
		goleak.IgnoreCurrent(),

		// Ignore test runner goroutines
		goleak.IgnoreTopFunction("testing.(*T).Run"),
		goleak.IgnoreTopFunction("testing.tRunner"),

		// Ignore HTTP server goroutines (for integration tests)
		goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
		goleak.IgnoreTopFunction("net/http.(*connReader).backgroundRead"),

		// Ignore database connection pool goroutines
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("github.com/jackc/pgx/v5/pgxpool.(*Pool).backgroundHealthCheck"),

		// Ignore gRPC goroutines (for ConnectRPC tests)
		goleak.IgnoreTopFunction("google.golang.org/grpc.(*addrConn).resetTransport"),
		goleak.IgnoreTopFunction("google.golang.org/grpc.(*ccBalancerWrapper).watcher"),
	}

	return func() {
		goleak.VerifyNone(t, opts...)
	}
}

// VerifyNoGoroutineLeaksMain should be called in TestMain to verify
// that the entire test suite doesn't leak goroutines.
//
// Usage:
//
//	func TestMain(m *testing.M) {
//		goleak.VerifyTestMain(m)
//	}
func VerifyNoGoroutineLeaksMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("testing.(*T).Run"),
		goleak.IgnoreTopFunction("testing.tRunner"),
	)
}
