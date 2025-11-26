package shutdown

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	// HTTP in-flight request metrics
	httpInflightRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_inflight_requests",
		Help: "Number of HTTP requests currently being processed",
	}, []string{"server"})

	httpRequestsRejectedDraining = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_rejected_draining_total",
		Help: "Total number of requests rejected during draining period",
	}, []string{"server"})

	httpDrainingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_draining_duration_seconds",
		Help:    "Time taken to drain in-flight HTTP requests",
		Buckets: []float64{0.5, 1, 2, 5, 10, 15, 20, 25, 30},
	}, []string{"server"})
)

// HTTPInFlightTracker tracks in-flight HTTP requests for graceful shutdown
// Provides middleware for request tracking and draining functionality
type HTTPInFlightTracker struct {
	name     string
	logger   *zap.Logger
	count    atomic.Int64
	draining atomic.Bool
	done     chan struct{}
}

// NewHTTPInFlightTracker creates a new HTTP in-flight request tracker
func NewHTTPInFlightTracker(name string, logger *zap.Logger) *HTTPInFlightTracker {
	return &HTTPInFlightTracker{
		name:   name,
		logger: logger,
		done:   make(chan struct{}),
	}
}

// Middleware returns HTTP middleware that tracks in-flight requests
// During draining, new requests are rejected with 503 Service Unavailable
func (t *HTTPInFlightTracker) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If draining, reject new requests with 503 Service Unavailable
		if t.draining.Load() {
			httpRequestsRejectedDraining.WithLabelValues(t.name).Inc()
			t.logger.Debug("Rejecting request during draining",
				zap.String("server", t.name),
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
			)
			http.Error(w, "Service is shutting down", http.StatusServiceUnavailable)
			return
		}

		// Increment in-flight counter
		count := t.count.Add(1)
		httpInflightRequests.WithLabelValues(t.name).Set(float64(count))

		t.logger.Debug("HTTP request started",
			zap.String("server", t.name),
			zap.Int64("inflight", count),
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
		)

		// Decrement when done
		defer func() {
			count := t.count.Add(-1)
			httpInflightRequests.WithLabelValues(t.name).Set(float64(count))

			t.logger.Debug("HTTP request completed",
				zap.String("server", t.name),
				zap.Int64("inflight", count),
				zap.String("path", r.URL.Path),
			)

			// If draining and this was the last request, signal completion
			if t.draining.Load() && count == 0 {
				select {
				case <-t.done:
					// Already closed
				default:
					close(t.done)
				}
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// Drain initiates draining phase - stops accepting new requests and waits for in-flight to complete
// Returns when all in-flight requests are complete or context is cancelled
func (t *HTTPInFlightTracker) Drain(ctx context.Context) error {
	drainingStart := time.Now()

	// Set draining flag to reject new requests
	t.draining.Store(true)

	currentCount := t.count.Load()
	t.logger.Info("Starting HTTP request draining",
		zap.String("server", t.name),
		zap.Int64("inflight_requests", currentCount),
	)

	// If no in-flight requests, we're done immediately
	if currentCount == 0 {
		httpDrainingDuration.WithLabelValues(t.name).Observe(time.Since(drainingStart).Seconds())
		t.logger.Info("No in-flight HTTP requests to drain",
			zap.String("server", t.name),
		)
		return nil
	}

	// Wait for all in-flight requests to complete or timeout
	select {
	case <-t.done:
		elapsed := time.Since(drainingStart)
		httpDrainingDuration.WithLabelValues(t.name).Observe(elapsed.Seconds())
		t.logger.Info("All in-flight HTTP requests drained successfully",
			zap.String("server", t.name),
			zap.Duration("elapsed", elapsed),
		)
		return nil

	case <-ctx.Done():
		elapsed := time.Since(drainingStart)
		httpDrainingDuration.WithLabelValues(t.name).Observe(elapsed.Seconds())
		remainingCount := t.count.Load()
		t.logger.Warn("HTTP draining timeout - forcing shutdown with remaining requests",
			zap.String("server", t.name),
			zap.Int64("remaining_requests", remainingCount),
			zap.Duration("elapsed", elapsed),
		)
		return ctx.Err()
	}
}

// IsDraining returns whether the tracker is in draining mode
func (t *HTTPInFlightTracker) IsDraining() bool {
	return t.draining.Load()
}

// Count returns the current number of in-flight requests
func (t *HTTPInFlightTracker) Count() int64 {
	return t.count.Load()
}

// DrainingHealthCheck returns a health check handler that returns unhealthy during draining
// This allows load balancers to remove the instance from rotation
func (t *HTTPInFlightTracker) DrainingHealthCheck(healthyHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if t.IsDraining() {
			// Return 503 during draining to signal load balancers
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"draining","message":"Service is shutting down gracefully"}`))
			return
		}

		// Otherwise use the normal health check
		healthyHandler.ServeHTTP(w, r)
	})
}
