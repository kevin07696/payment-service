package resilience

import (
	"context"
	"testing"
	"time"
)

func TestDefaultTimeoutConfig(t *testing.T) {
	config := DefaultTimeoutConfig()

	// Verify timeout hierarchy is correctly ordered
	if config.HTTPHandler <= config.Service {
		t.Errorf("HTTPHandler (%v) must be > Service (%v)", config.HTTPHandler, config.Service)
	}

	if config.Service <= config.ServiceCritical {
		t.Errorf("Service (%v) must be > ServiceCritical (%v)", config.Service, config.ServiceCritical)
	}

	if config.ServiceCritical <= config.ExternalAPI {
		t.Errorf("ServiceCritical (%v) must be > ExternalAPI (%v)", config.ServiceCritical, config.ExternalAPI)
	}

	if config.ExternalAPI <= config.SingleRetry {
		t.Errorf("ExternalAPI (%v) must be > SingleRetry (%v)", config.ExternalAPI, config.SingleRetry)
	}

	// Verify production values
	if config.HTTPHandler != 60*time.Second {
		t.Errorf("Expected HTTPHandler = 60s, got %v", config.HTTPHandler)
	}

	if config.Service != 50*time.Second {
		t.Errorf("Expected Service = 50s, got %v", config.Service)
	}

	if config.ExternalAPI != 30*time.Second {
		t.Errorf("Expected ExternalAPI = 30s, got %v", config.ExternalAPI)
	}
}

func TestTestTimeoutConfig(t *testing.T) {
	config := TestTimeoutConfig()

	// Verify test timeouts are shorter
	if config.HTTPHandler >= 10*time.Second {
		t.Errorf("Test timeouts should be < 10s, got %v", config.HTTPHandler)
	}

	// Verify hierarchy is still preserved in test config
	if config.HTTPHandler <= config.Service {
		t.Errorf("HTTPHandler (%v) must be > Service (%v)", config.HTTPHandler, config.Service)
	}

	if config.Service <= config.ExternalAPI {
		t.Errorf("Service (%v) must be > ExternalAPI (%v)", config.Service, config.ExternalAPI)
	}
}

func TestHandlerContext(t *testing.T) {
	config := DefaultTimeoutConfig()
	parent := context.Background()

	ctx, cancel := config.HandlerContext(parent)
	defer cancel()

	// Verify context has deadline
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("HandlerContext should have deadline")
	}

	// Verify deadline is approximately HTTPHandler duration from now
	expectedDeadline := time.Now().Add(config.HTTPHandler)
	diff := deadline.Sub(expectedDeadline).Abs()
	if diff > 100*time.Millisecond {
		t.Errorf("Deadline diff too large: %v", diff)
	}
}

func TestServiceContext(t *testing.T) {
	config := DefaultTimeoutConfig()
	parent := context.Background()

	ctx, cancel := config.ServiceContext(parent)
	defer cancel()

	// Verify context has deadline
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("ServiceContext should have deadline")
	}

	// Verify deadline is approximately Service duration from now
	expectedDeadline := time.Now().Add(config.Service)
	diff := deadline.Sub(expectedDeadline).Abs()
	if diff > 100*time.Millisecond {
		t.Errorf("Deadline diff too large: %v", diff)
	}
}

func TestTimeoutHierarchyPreservation(t *testing.T) {
	// Verify that child contexts respect parent deadlines
	config := DefaultTimeoutConfig()

	// Create parent context with 5 second timeout
	parent, parentCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer parentCancel()

	// Try to create child with longer timeout
	child, childCancel := config.HandlerContext(parent)
	defer childCancel()

	// Child should inherit parent's shorter deadline
	parentDeadline, _ := parent.Deadline()
	childDeadline, _ := child.Deadline()

	// Child deadline should be same or earlier than parent
	if childDeadline.After(parentDeadline) {
		t.Errorf("Child deadline (%v) should not be after parent deadline (%v)",
			childDeadline, parentDeadline)
	}
}

func TestContextCancellationPropagation(t *testing.T) {
	config := DefaultTimeoutConfig()
	parent := context.Background()

	ctx, cancel := config.ServiceContext(parent)

	// Cancel context
	cancel()

	// Verify context is cancelled
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled immediately")
	}

	// Verify error is context.Canceled
	if ctx.Err() != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", ctx.Err())
	}
}

func TestContextTimeout(t *testing.T) {
	// Use test config for faster tests
	config := TestTimeoutConfig()
	parent := context.Background()

	// Create context with 100ms timeout
	config.Service = 100 * time.Millisecond
	ctx, cancel := config.ServiceContext(parent)
	defer cancel()

	// Wait for timeout
	select {
	case <-ctx.Done():
		// Verify error is DeadlineExceeded
		if ctx.Err() != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got %v", ctx.Err())
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should timeout after 100ms")
	}
}

func TestAllContextCreators(t *testing.T) {
	config := DefaultTimeoutConfig()
	parent := context.Background()

	tests := []struct {
		name    string
		creator func(context.Context) (context.Context, context.CancelFunc)
		timeout time.Duration
	}{
		{"HandlerContext", config.HandlerContext, config.HTTPHandler},
		{"CronContext", config.CronContext, config.CronJob},
		{"ServiceContext", config.ServiceContext, config.Service},
		{"CriticalPathContext", config.CriticalPathContext, config.ServiceCritical},
		{"NonCriticalContext", config.NonCriticalContext, config.ServiceNonCritial},
		{"ExternalAPIContext", config.ExternalAPIContext, config.ExternalAPI},
		{"RetryAttemptContext", config.RetryAttemptContext, config.SingleRetry},
		{"WebhookContext", config.WebhookContext, config.WebhookDelivery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.creator(parent)
			defer cancel()

			// Verify deadline exists
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatalf("%s should have deadline", tt.name)
			}

			// Verify deadline is approximately correct
			expectedDeadline := time.Now().Add(tt.timeout)
			diff := deadline.Sub(expectedDeadline).Abs()
			if diff > 100*time.Millisecond {
				t.Errorf("%s: deadline diff too large: %v (expected ~%v)",
					tt.name, diff, tt.timeout)
			}
		})
	}
}

func TestCriticalVsNonCriticalPaths(t *testing.T) {
	config := DefaultTimeoutConfig()

	// Critical path should have longer timeout than non-critical
	if config.ServiceCritical <= config.ServiceNonCritial {
		t.Errorf("Critical path (%v) should have longer timeout than non-critical (%v)",
			config.ServiceCritical, config.ServiceNonCritial)
	}
}

func TestCronJobTimeout(t *testing.T) {
	config := DefaultTimeoutConfig()

	// Cron jobs should have significantly longer timeout than HTTP handlers
	if config.CronJob <= config.HTTPHandler {
		t.Errorf("CronJob (%v) should have longer timeout than HTTPHandler (%v)",
			config.CronJob, config.HTTPHandler)
	}

	// Verify cron job timeout is at least 5 minutes
	if config.CronJob < 5*time.Minute {
		t.Errorf("CronJob timeout should be >= 5 minutes, got %v", config.CronJob)
	}
}

func TestTimeoutBudget(t *testing.T) {
	config := DefaultTimeoutConfig()

	// Verify timeout hierarchy relationships
	//
	// Note: ExternalAPI timeout (30s) is used as HTTP client timeout PER request,
	// not for the total retry loop. The retry loop is bounded by the Service timeout (50s).
	//
	// Example operation flow:
	//   Service timeout: 50s (total budget)
	//     ├─ DB query: 5s
	//     ├─ EPX call with retries: ~30-40s
	//     │  ├─ Attempt 1: up to 30s (HTTP timeout)
	//     │  ├─ Backoff: ~100ms
	//     │  ├─ Attempt 2: up to 30s (HTTP timeout)
	//     │  └─ Backoff: ~200ms
	//     └─ Response processing: ~5s
	//
	// The Service timeout must accommodate:
	// - Database operations (~5s)
	// - External API calls with retries (~30-35s for 1-2 attempts typically)
	// - Logic and serialization overhead (~10s buffer)

	// Verify SingleRetry timeout is reasonable for individual EPX attempts
	if config.SingleRetry < 5*time.Second {
		t.Errorf("SingleRetry (%v) should be >= 5s for reliable EPX calls", config.SingleRetry)
	}

	// Verify ExternalAPI timeout allows for at least one full attempt
	if config.ExternalAPI < config.SingleRetry {
		t.Errorf("ExternalAPI (%v) must be >= SingleRetry (%v)",
			config.ExternalAPI, config.SingleRetry)
	}

	// Verify Service timeout has buffer for DB + EPX + overhead
	// Minimum: 5s (DB) + 30s (EPX) + 10s (buffer) = 45s
	minServiceBudget := 5*time.Second + config.ExternalAPI + 10*time.Second
	if config.Service < minServiceBudget {
		t.Errorf("Service timeout (%v) insufficient for typical operations (need >= %v)",
			config.Service, minServiceBudget)
	}

	// Verify HTTPHandler has buffer beyond Service timeout
	if config.HTTPHandler <= config.Service {
		t.Errorf("HTTPHandler (%v) must be > Service (%v)",
			config.HTTPHandler, config.Service)
	}
}
