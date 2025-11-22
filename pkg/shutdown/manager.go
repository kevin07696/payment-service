package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	// Shutdown metrics
	shutdownDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "shutdown_duration_seconds",
		Help:    "Total time taken to shutdown gracefully",
		Buckets: []float64{1, 5, 10, 15, 20, 25, 30},
	})

	componentShutdownDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "component_shutdown_duration_seconds",
		Help:    "Time taken to shutdown individual components",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 15, 20, 25, 30},
	}, []string{"component"})

	shutdownErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "shutdown_errors_total",
		Help: "Total number of shutdown errors by component",
	}, []string{"component"})

	gracefulShutdownsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "graceful_shutdowns_total",
		Help: "Total number of graceful shutdowns",
	})
)

// ShutdownFunc represents a function that shuts down a component
type ShutdownFunc func(context.Context) error

// Component represents a registered shutdown component
type Component struct {
	Name         string
	ShutdownFunc ShutdownFunc
}

// Manager coordinates graceful shutdown of all service components
// Components shut down in REVERSE registration order (LIFO)
// This ensures dependencies shut down properly (e.g., HTTP servers before database)
type Manager struct {
	logger     *zap.Logger
	components []Component
	mu         sync.Mutex
	timeout    time.Duration
}

// NewManager creates a new shutdown manager
func NewManager(logger *zap.Logger, timeout time.Duration) *Manager {
	return &Manager{
		logger:     logger,
		components: make([]Component, 0),
		timeout:    timeout,
	}
}

// Register adds a shutdown function to be called during graceful shutdown
// Components are shut down in REVERSE order of registration (LIFO)
// Example registration order:
//   1. Background workers (shutdown first - stop generating new work)
//   2. HTTP servers (stop accepting new requests)
//   3. Services (finish in-flight requests)
//   4. Database (close connections last)
func (sm *Manager) Register(name string, fn ShutdownFunc) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	component := Component{
		Name:         name,
		ShutdownFunc: fn,
	}

	sm.components = append(sm.components, component)

	sm.logger.Debug("Registered shutdown component",
		zap.String("component", name),
		zap.Int("registration_order", len(sm.components)),
	)
}

// WaitForShutdown blocks until a shutdown signal is received (SIGINT or SIGTERM)
// Then executes graceful shutdown of all registered components
func (sm *Manager) WaitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-quit
	sm.logger.Info("Received shutdown signal - initiating graceful shutdown",
		zap.String("signal", sig.String()),
		zap.Duration("timeout", sm.timeout),
	)

	// Execute shutdown
	sm.Shutdown()
}

// Shutdown performs graceful shutdown of all registered components
// Can be called manually or via WaitForShutdown
func (sm *Manager) Shutdown() {
	gracefulShutdownsTotal.Inc()
	shutdownStart := time.Now()

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), sm.timeout)
	defer cancel()

	sm.logger.Info("Starting graceful shutdown",
		zap.Int("component_count", len(sm.components)),
		zap.Duration("timeout", sm.timeout),
	)

	// Shutdown all components in REVERSE registration order (LIFO)
	// This ensures proper dependency ordering
	errors := sm.shutdownComponents(ctx)

	// Record total shutdown duration
	shutdownElapsed := time.Since(shutdownStart)
	shutdownDuration.Observe(shutdownElapsed.Seconds())

	if len(errors) > 0 {
		sm.logger.Error("Graceful shutdown completed with errors",
			zap.Int("error_count", len(errors)),
			zap.Duration("elapsed", shutdownElapsed),
		)
		for component, err := range errors {
			sm.logger.Error("Component shutdown error",
				zap.String("component", component),
				zap.Error(err),
			)
		}
	} else {
		sm.logger.Info("Graceful shutdown completed successfully",
			zap.Duration("elapsed", shutdownElapsed),
		)
	}
}

// shutdownComponents executes shutdown for all components in reverse order
func (sm *Manager) shutdownComponents(ctx context.Context) map[string]error {
	sm.mu.Lock()
	components := make([]Component, len(sm.components))
	copy(components, sm.components)
	sm.mu.Unlock()

	errors := make(map[string]error)
	var errorsMu sync.Mutex

	// Shutdown components in REVERSE order (LIFO)
	// Use goroutines for parallel shutdown, but wait for each "phase"
	var wg sync.WaitGroup

	for i := len(components) - 1; i >= 0; i-- {
		component := components[i]

		wg.Add(1)
		go func(comp Component) {
			defer wg.Done()

			start := time.Now()
			sm.logger.Info("Shutting down component",
				zap.String("component", comp.Name),
			)

			// Execute shutdown function
			if err := comp.ShutdownFunc(ctx); err != nil {
				errorsMu.Lock()
				errors[comp.Name] = err
				errorsMu.Unlock()

				shutdownErrors.WithLabelValues(comp.Name).Inc()
				sm.logger.Error("Component shutdown failed",
					zap.String("component", comp.Name),
					zap.Error(err),
					zap.Duration("elapsed", time.Since(start)),
				)
			} else {
				sm.logger.Info("Component shut down successfully",
					zap.String("component", comp.Name),
					zap.Duration("elapsed", time.Since(start)),
				)
			}

			// Record component shutdown duration
			componentShutdownDuration.WithLabelValues(comp.Name).Observe(time.Since(start).Seconds())
		}(component)
	}

	// Wait for all shutdowns to complete or context timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		sm.logger.Info("All components shut down")
	case <-ctx.Done():
		sm.logger.Warn("Shutdown timeout exceeded - some components may not have completed",
			zap.Duration("timeout", sm.timeout),
		)
	}

	return errors
}

// RegisterHTTPServer is a convenience method for registering HTTP servers
func (sm *Manager) RegisterHTTPServer(name string, server interface{ Shutdown(context.Context) error }) {
	sm.Register(name, server.Shutdown)
}

// RegisterCloser is a convenience method for registering components with Close() method
func (sm *Manager) RegisterCloser(name string, closer interface{ Close() error }) {
	sm.Register(name, func(ctx context.Context) error {
		return closer.Close()
	})
}

// RegisterFunc is a convenience method for registering simple shutdown functions
func (sm *Manager) RegisterFunc(name string, fn func() error) {
	sm.Register(name, func(ctx context.Context) error {
		return fn()
	})
}

// RegisterNoErr is a convenience method for shutdown functions that don't return errors
func (sm *Manager) RegisterNoErr(name string, fn func()) {
	sm.Register(name, func(ctx context.Context) error {
		fn()
		return nil
	})
}
