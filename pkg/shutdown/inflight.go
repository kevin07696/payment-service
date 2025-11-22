package shutdown

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// InFlightTracker tracks in-flight work (requests, jobs, etc.) to ensure
// graceful shutdown waits for work to complete
type InFlightTracker struct {
	wg         sync.WaitGroup
	shutdownCh chan struct{}
	logger     *zap.Logger
	name       string
}

// NewInFlightTracker creates a new in-flight work tracker
func NewInFlightTracker(name string, logger *zap.Logger) *InFlightTracker {
	return &InFlightTracker{
		shutdownCh: make(chan struct{}),
		logger:     logger,
		name:       name,
	}
}

// Add increments the in-flight work counter
// Returns false if shutdown has been initiated (don't start new work)
func (ift *InFlightTracker) Add() bool {
	select {
	case <-ift.shutdownCh:
		// Shutdown initiated - don't accept new work
		return false
	default:
		ift.wg.Add(1)
		return true
	}
}

// Done decrements the in-flight work counter
// Call this when work is complete (typically via defer)
func (ift *InFlightTracker) Done() {
	ift.wg.Done()
}

// Shutdown initiates shutdown and waits for all in-flight work to complete
// Returns error if context times out before all work completes
func (ift *InFlightTracker) Shutdown(ctx context.Context) error {
	// Signal that we're shutting down (reject new work)
	close(ift.shutdownCh)

	ift.logger.Info("Waiting for in-flight work to complete",
		zap.String("tracker", ift.name),
	)

	// Wait for all in-flight work with timeout
	done := make(chan struct{})
	go func() {
		ift.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		ift.logger.Info("All in-flight work completed",
			zap.String("tracker", ift.name),
		)
		return nil
	case <-ctx.Done():
		ift.logger.Warn("Shutdown timeout - some work may be incomplete",
			zap.String("tracker", ift.name),
		)
		return ctx.Err()
	}
}

// IsShuttingDown returns true if shutdown has been initiated
func (ift *InFlightTracker) IsShuttingDown() bool {
	select {
	case <-ift.shutdownCh:
		return true
	default:
		return false
	}
}

// Run executes a function as in-flight work
// Automatically handles Add/Done and respects shutdown
func (ift *InFlightTracker) Run(fn func()) bool {
	if !ift.Add() {
		return false // Shutdown in progress, work not started
	}
	defer ift.Done()

	fn()
	return true
}

// RunWithContext executes a function with context as in-flight work
// Returns false if shutdown is in progress
func (ift *InFlightTracker) RunWithContext(ctx context.Context, fn func(context.Context)) bool {
	if !ift.Add() {
		return false // Shutdown in progress, work not started
	}
	defer ift.Done()

	fn(ctx)
	return true
}

// BackgroundWorker manages a background worker with graceful shutdown
type BackgroundWorker struct {
	name       string
	logger     *zap.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan struct{}
}

// NewBackgroundWorker creates a new background worker
func NewBackgroundWorker(name string, logger *zap.Logger) *BackgroundWorker {
	ctx, cancel := context.WithCancel(context.Background())

	return &BackgroundWorker{
		name:       name,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		shutdownCh: make(chan struct{}),
	}
}

// Start begins the background worker
// The work function should respect ctx.Done() for cancellation
func (bw *BackgroundWorker) Start(work func(ctx context.Context)) {
	bw.wg.Add(1)

	go func() {
		defer bw.wg.Done()

		bw.logger.Info("Background worker started",
			zap.String("worker", bw.name),
		)

		work(bw.ctx)

		bw.logger.Info("Background worker stopped",
			zap.String("worker", bw.name),
		)
	}()
}

// Stop gracefully stops the background worker
func (bw *BackgroundWorker) Stop() {
	select {
	case <-bw.shutdownCh:
		// Already stopped
		return
	default:
		close(bw.shutdownCh)
	}

	bw.logger.Info("Stopping background worker",
		zap.String("worker", bw.name),
	)

	// Cancel context to signal worker to stop
	bw.cancel()

	// Wait for worker to finish
	bw.wg.Wait()

	bw.logger.Info("Background worker stopped successfully",
		zap.String("worker", bw.name),
	)
}

// Shutdown waits for the worker to stop with timeout
func (bw *BackgroundWorker) Shutdown(ctx context.Context) error {
	bw.Stop()

	// Wait for worker with timeout
	done := make(chan struct{})
	go func() {
		bw.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		bw.logger.Warn("Background worker shutdown timeout",
			zap.String("worker", bw.name),
		)
		return ctx.Err()
	}
}

// Context returns the worker's context
func (bw *BackgroundWorker) Context() context.Context {
	return bw.ctx
}

// PeriodicWorker runs a function periodically with graceful shutdown support
type PeriodicWorker struct {
	*BackgroundWorker
	interval time.Duration
}

// NewPeriodicWorker creates a new periodic worker
func NewPeriodicWorker(name string, interval time.Duration, logger *zap.Logger) *PeriodicWorker {
	return &PeriodicWorker{
		BackgroundWorker: NewBackgroundWorker(name, logger),
		interval:         interval,
	}
}

// Start begins the periodic worker
func (pw *PeriodicWorker) Start(work func(ctx context.Context)) {
	pw.BackgroundWorker.Start(func(ctx context.Context) {
		ticker := time.NewTicker(pw.interval)
		defer ticker.Stop()

		// Run immediately on start
		work(ctx)

		for {
			select {
			case <-ctx.Done():
				pw.logger.Info("Periodic worker context cancelled",
					zap.String("worker", pw.name),
				)
				return
			case <-ticker.C:
				work(ctx)
			}
		}
	})
}
