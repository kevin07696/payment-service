package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	// gRPC request metrics
	grpcRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"},
	)

	grpcRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "grpc_request_duration_seconds",
			Help: "Duration of gRPC requests in seconds",
			// Custom buckets optimized for payment API latencies (10ms to 5s)
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"method"},
	)

	grpcRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "grpc_requests_in_flight",
			Help: "Number of gRPC requests currently being processed",
		},
	)

	// SLO Tracking Metrics
	// SLI: Service Level Indicator - actual measurement of service behavior
	// SLO: Service Level Objective - target for SLI (e.g., 99.9% success rate)

	// Availability SLO - tracks success/failure rate
	sloAvailabilityTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "slo_availability_total",
			Help: "Total requests for availability SLO tracking (success vs failure)",
		},
		[]string{"service", "outcome"}, // outcome: "success" or "failure"
	)

	// Latency SLO - tracks requests meeting latency targets
	sloLatencyCompliance = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "slo_latency_compliance_total",
			Help: "Requests meeting latency SLO targets",
		},
		[]string{"service", "slo_target", "compliant"}, // slo_target: "p95_100ms", compliant: "true"/"false"
	)

	// Error Budget - tracks remaining error budget (1 - SLO target)
	// For 99.9% SLO, error budget is 0.1% (1000 failed requests per 1M total)
	sloErrorBudgetRemaining = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "slo_error_budget_remaining_ratio",
			Help: "Remaining error budget as a ratio (1.0 = full budget, 0.0 = exhausted)",
		},
		[]string{"service", "slo_window"}, // slo_window: "30d", "7d", etc.
	)

	// Business SLOs - payment-specific success metrics
	paymentTransactionSLO = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_transaction_slo_total",
			Help: "Payment transaction outcomes for SLO tracking",
		},
		[]string{"transaction_type", "outcome"}, // transaction_type: "sale", "auth", "capture", outcome: "approved", "declined", "error"
	)

	// Database Connection Pool Metrics
	// Critical for detecting connection exhaustion before failure
	dbPoolTotalConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_total_connections",
			Help: "Current total number of connections in the pool (idle + active)",
		},
	)

	dbPoolIdleConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_idle_connections",
			Help: "Current number of idle connections in the pool",
		},
	)

	dbPoolActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_active_connections",
			Help: "Current number of active connections in use",
		},
	)

	dbPoolWaitCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "db_pool_wait_count_total",
			Help: "Total number of times a connection request had to wait",
		},
	)

	dbPoolWaitDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name: "db_pool_wait_duration_seconds",
			Help: "Time spent waiting for a connection from the pool",
			// Buckets for connection wait time (1ms to 5s)
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0},
		},
	)

	dbPoolMaxConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_max_connections",
			Help: "Maximum number of connections allowed in the pool",
		},
	)

	dbPoolConnectionUtilization = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_connection_utilization_ratio",
			Help: "Connection pool utilization ratio (active / max), critical alert threshold at 0.8",
		},
	)
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that records Prometheus metrics
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		grpcRequestsInFlight.Inc()
		defer grpcRequestsInFlight.Dec()

		// Call the handler
		resp, err := handler(ctx, req)

		// Record metrics
		duration := time.Since(start).Seconds()
		grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(duration)

		statusCode := "OK"
		if err != nil {
			st, _ := status.FromError(err)
			statusCode = st.Code().String()
		}
		grpcRequestsTotal.WithLabelValues(info.FullMethod, statusCode).Inc()

		// Track SLO metrics
		RecordAvailabilitySLO(info.FullMethod, err == nil)
		RecordLatencySLO(info.FullMethod, duration)

		return resp, err
	}
}

// RecordAvailabilitySLO tracks request success/failure for availability SLO
// service: service name (e.g., "payment", "subscription")
// success: true if request succeeded, false if failed
func RecordAvailabilitySLO(service string, success bool) {
	outcome := "failure"
	if success {
		outcome = "success"
	}
	sloAvailabilityTotal.WithLabelValues(service, outcome).Inc()
}

// RecordLatencySLO tracks whether request met latency SLO target
// service: service name
// durationSeconds: actual request duration
func RecordLatencySLO(service string, durationSeconds float64) {
	// Payment service SLO targets:
	// P95 latency < 100ms for critical payment operations
	// P99 latency < 500ms
	targets := map[string]float64{
		"p95_100ms": 0.100,
		"p99_500ms": 0.500,
	}

	for targetName, targetValue := range targets {
		compliant := "true"
		if durationSeconds > targetValue {
			compliant = "false"
		}
		sloLatencyCompliance.WithLabelValues(service, targetName, compliant).Inc()
	}
}

// UpdateErrorBudget calculates and updates error budget remaining
// service: service name
// window: SLO window (e.g., "30d", "7d")
// sloTarget: target availability (e.g., 0.999 for 99.9%)
// successCount: number of successful requests in window
// totalCount: total requests in window
func UpdateErrorBudget(service, window string, sloTarget float64, successCount, totalCount int64) {
	if totalCount == 0 {
		sloErrorBudgetRemaining.WithLabelValues(service, window).Set(1.0)
		return
	}

	errorBudget := 1.0 - sloTarget // e.g., 0.001 for 99.9% SLO

	// Calculate remaining budget
	// If actual availability = 99.95%, and SLO = 99.9%, we've used 5% of our 0.1% budget
	errorsAllowed := int64(float64(totalCount) * errorBudget)
	actualErrors := totalCount - successCount
	budgetRemaining := 1.0 - (float64(actualErrors) / float64(errorsAllowed))

	// Clamp between 0 and 1
	if budgetRemaining < 0 {
		budgetRemaining = 0
	}
	if budgetRemaining > 1 {
		budgetRemaining = 1
	}

	sloErrorBudgetRemaining.WithLabelValues(service, window).Set(budgetRemaining)
}

// RecordPaymentTransactionSLO tracks payment transaction outcomes for business SLOs
// transactionType: "sale", "auth", "capture", "refund", "void"
// outcome: "approved", "declined", "error"
func RecordPaymentTransactionSLO(transactionType, outcome string) {
	paymentTransactionSLO.WithLabelValues(transactionType, outcome).Inc()
}

// UpdateDatabasePoolMetrics updates Prometheus metrics for database connection pool
// This function should be called periodically to track pool health
//
// Parameters:
//   - totalConns: Current total connections (idle + active)
//   - idleConns: Current idle connections
//   - maxConns: Maximum allowed connections
//   - waitCount: Total number of connection waits
//   - waitDuration: Time spent waiting for connections (in seconds)
//
// Critical Alerts:
//   - Connection utilization > 80%: Pool approaching exhaustion
//   - Wait count increasing rapidly: Need to increase pool size
//   - Active connections = max: Pool exhausted, requests being queued
func UpdateDatabasePoolMetrics(totalConns, idleConns, maxConns int32, waitCount int64, waitDuration float64) {
	// Update basic pool metrics
	dbPoolTotalConnections.Set(float64(totalConns))
	dbPoolIdleConnections.Set(float64(idleConns))
	dbPoolMaxConnections.Set(float64(maxConns))

	// Calculate and update active connections
	activeConns := totalConns - idleConns
	dbPoolActiveConnections.Set(float64(activeConns))

	// Update wait metrics if provided
	if waitCount > 0 {
		// Only increment the counter by the delta since last update
		// This requires tracking previous waitCount, but for simplicity
		// we'll just set the wait duration histogram
		dbPoolWaitDuration.Observe(waitDuration)
	}

	// Calculate connection utilization ratio (critical metric for alerting)
	// Alert when this exceeds 0.8 (80% utilization)
	if maxConns > 0 {
		utilization := float64(activeConns) / float64(maxConns)
		dbPoolConnectionUtilization.Set(utilization)
	}
}

// IncrementDatabasePoolWaitCount increments the connection wait counter
// Call this each time a request has to wait for a connection
func IncrementDatabasePoolWaitCount() {
	dbPoolWaitCount.Inc()
}

// RecordDatabasePoolWaitDuration records how long a request waited for a connection
// waitSeconds: duration in seconds that the request waited
func RecordDatabasePoolWaitDuration(waitSeconds float64) {
	dbPoolWaitDuration.Observe(waitSeconds)
}
