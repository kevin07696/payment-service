package resilience

import (
	"context"
	"time"
)

// TimeoutConfig defines timeout values for the application's timeout hierarchy
//
// Timeout Hierarchy (from outermost to innermost):
//   HTTP Handler (60s)
//     ↓
//   Service Layer (50s)
//     ↓
//   External API (30s - EPX Gateway)
//     ↓
//   Database Query (2s/5s/30s - based on complexity)
//
// This hierarchy ensures each layer completes before its parent times out,
// preventing cascading timeout failures and providing predictable behavior.
type TimeoutConfig struct {
	// Handler layer timeouts
	HTTPHandler time.Duration // Overall request timeout (default: 60s)
	CronJob     time.Duration // Cron job execution timeout (default: 5 minutes)

	// Service layer timeouts
	Service          time.Duration // Service operation timeout (default: 50s)
	ServiceCritical  time.Duration // Critical path operations (default: 45s)
	ServiceNonCritial time.Duration // Non-critical operations like webhooks (default: 30s)

	// External API timeouts (adapters)
	ExternalAPI    time.Duration // EPX gateway calls (default: 30s)
	SingleRetry    time.Duration // Individual retry attempt (default: 10s)
	WebhookDelivery time.Duration // Webhook delivery per attempt (default: 10s)

	// Database timeouts (already implemented in postgres adapter)
	// Listed here for documentation only
	// SimpleQuery:  2s  - ID lookups, single row operations
	// ComplexQuery: 5s  - JOINs, filters, aggregations
	// ReportQuery:  30s - Analytics, large scans
}

// DefaultTimeoutConfig returns production timeout values
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		// Handler layer
		HTTPHandler: 60 * time.Second,
		CronJob:     5 * time.Minute,

		// Service layer (must be < HTTPHandler)
		Service:           50 * time.Second,
		ServiceCritical:   45 * time.Second,
		ServiceNonCritial: 30 * time.Second,

		// External APIs
		ExternalAPI:     30 * time.Second,
		SingleRetry:     10 * time.Second,
		WebhookDelivery: 10 * time.Second,
	}
}

// TestTimeoutConfig returns shorter timeouts for testing
func TestTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		HTTPHandler:       5 * time.Second,
		CronJob:           30 * time.Second,
		Service:           4 * time.Second,
		ServiceCritical:   3 * time.Second,
		ServiceNonCritial: 2 * time.Second,
		ExternalAPI:       2 * time.Second,
		SingleRetry:       1 * time.Second,
		WebhookDelivery:   1 * time.Second,
	}
}

// HandlerContext creates a context with timeout for HTTP handlers
func (tc *TimeoutConfig) HandlerContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, tc.HTTPHandler)
}

// CronContext creates a context with timeout for cron jobs
func (tc *TimeoutConfig) CronContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, tc.CronJob)
}

// ServiceContext creates a context with timeout for service layer operations
func (tc *TimeoutConfig) ServiceContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, tc.Service)
}

// CriticalPathContext creates a context for critical business operations
func (tc *TimeoutConfig) CriticalPathContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, tc.ServiceCritical)
}

// NonCriticalContext creates a context for non-critical operations
func (tc *TimeoutConfig) NonCriticalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, tc.ServiceNonCritial)
}

// ExternalAPIContext creates a context for external API calls
func (tc *TimeoutConfig) ExternalAPIContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, tc.ExternalAPI)
}

// RetryAttemptContext creates a context for a single retry attempt
func (tc *TimeoutConfig) RetryAttemptContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, tc.SingleRetry)
}

// WebhookContext creates a context for webhook delivery
func (tc *TimeoutConfig) WebhookContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, tc.WebhookDelivery)
}
