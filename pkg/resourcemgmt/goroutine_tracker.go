package resourcemgmt

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	// Goroutine metrics
	goroutineCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "goroutines_count",
		Help: "Current number of goroutines in the process",
	})

	goroutineLeakDetected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "goroutine_leaks_detected_total",
		Help: "Total number of potential goroutine leak detections",
	})

	trackedGoroutines = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "tracked_goroutines",
		Help: "Number of tracked goroutines by type",
	}, []string{"type"})

	longRunningGoroutines = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "long_running_goroutines",
		Help: "Number of goroutines running longer than threshold",
	}, []string{"type"})
)

// TrackedGoroutine represents a tracked goroutine
type TrackedGoroutine struct {
	ID        string
	Type      string    // "webhook_delivery", "cron_job", "subscription_billing", etc.
	StartTime time.Time
	Done      chan struct{}
}

// GoroutineTracker tracks goroutines to detect and prevent leaks
// Provides visibility into goroutine lifecycle and alerts on anomalies
type GoroutineTracker struct {
	mu                sync.RWMutex
	trackedGoroutines map[string]*TrackedGoroutine
	logger            *zap.Logger
	baselineCount     int
	checkInterval     time.Duration
	leakThreshold     int           // Goroutines above baseline to trigger alert
	longRunningLimit  time.Duration // Duration after which goroutine is considered long-running
}

// Config holds configuration for goroutine tracker
type Config struct {
	CheckInterval    time.Duration // How often to check for leaks
	LeakThreshold    int           // Goroutines above baseline to alert
	LongRunningLimit time.Duration // Duration to flag long-running goroutines
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		CheckInterval:    30 * time.Second,
		LeakThreshold:    100, // Alert if 100+ goroutines above baseline
		LongRunningLimit: 10 * time.Minute,
	}
}

// NewGoroutineTracker creates a new goroutine tracker
func NewGoroutineTracker(logger *zap.Logger, cfg *Config) *GoroutineTracker {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	baselineCount := runtime.NumGoroutine()

	logger.Info("Goroutine tracker initialized",
		zap.Int("baseline_goroutines", baselineCount),
		zap.Duration("check_interval", cfg.CheckInterval),
		zap.Int("leak_threshold", cfg.LeakThreshold),
		zap.Duration("long_running_limit", cfg.LongRunningLimit),
	)

	return &GoroutineTracker{
		trackedGoroutines: make(map[string]*TrackedGoroutine),
		logger:            logger,
		baselineCount:     baselineCount,
		checkInterval:     cfg.CheckInterval,
		leakThreshold:     cfg.LeakThreshold,
		longRunningLimit:  cfg.LongRunningLimit,
	}
}

// Track registers a goroutine for tracking
// Returns TrackedGoroutine that should be used to untrack when done
func (gt *GoroutineTracker) Track(id, goroutineType string) *TrackedGoroutine {
	tg := &TrackedGoroutine{
		ID:        id,
		Type:      goroutineType,
		StartTime: time.Now(),
		Done:      make(chan struct{}),
	}

	gt.mu.Lock()
	gt.trackedGoroutines[id] = tg
	gt.mu.Unlock()

	trackedGoroutines.WithLabelValues(goroutineType).Inc()

	gt.logger.Debug("Goroutine tracked",
		zap.String("id", id),
		zap.String("type", goroutineType),
	)

	return tg
}

// Untrack removes a goroutine from tracking
// Call this when goroutine exits (typically via defer)
func (gt *GoroutineTracker) Untrack(id string) {
	gt.mu.Lock()
	defer gt.mu.Unlock()

	if tg, ok := gt.trackedGoroutines[id]; ok {
		close(tg.Done)
		trackedGoroutines.WithLabelValues(tg.Type).Dec()
		delete(gt.trackedGoroutines, id)

		duration := time.Since(tg.StartTime)
		gt.logger.Debug("Goroutine untracked",
			zap.String("id", id),
			zap.String("type", tg.Type),
			zap.Duration("lifetime", duration),
		)
	}
}

// StartMonitoring begins periodic goroutine leak detection
// Runs until context is cancelled
func (gt *GoroutineTracker) StartMonitoring(ctx context.Context) {
	gt.logger.Info("Starting goroutine leak monitoring")

	ticker := time.NewTicker(gt.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			gt.logger.Info("Stopping goroutine leak monitoring")
			return
		case <-ticker.C:
			gt.checkForLeaks()
		}
	}
}

// checkForLeaks detects potential goroutine leaks
func (gt *GoroutineTracker) checkForLeaks() {
	currentCount := runtime.NumGoroutine()
	goroutineCount.Set(float64(currentCount))

	increase := currentCount - gt.baselineCount

	// Check for significant increase above baseline
	if increase > gt.leakThreshold {
		gt.logger.Warn("Potential goroutine leak detected",
			zap.Int("current_count", currentCount),
			zap.Int("baseline_count", gt.baselineCount),
			zap.Int("increase", increase),
			zap.Int("threshold", gt.leakThreshold),
		)
		goroutineLeakDetected.Inc()
	}

	// Check for long-running tracked goroutines
	gt.checkLongRunning()

	// Log summary
	gt.logSummary(currentCount, increase)
}

// checkLongRunning checks for goroutines running longer than threshold
func (gt *GoroutineTracker) checkLongRunning() {
	gt.mu.RLock()
	defer gt.mu.RUnlock()

	longRunningByType := make(map[string]int)

	for id, tg := range gt.trackedGoroutines {
		age := time.Since(tg.StartTime)
		if age > gt.longRunningLimit {
			longRunningByType[tg.Type]++

			gt.logger.Warn("Long-running goroutine detected",
				zap.String("id", id),
				zap.String("type", tg.Type),
				zap.Duration("age", age),
				zap.Duration("limit", gt.longRunningLimit),
			)
		}
	}

	// Update metrics
	for goroutineType, count := range longRunningByType {
		longRunningGoroutines.WithLabelValues(goroutineType).Set(float64(count))
	}
}

// logSummary logs current goroutine status
func (gt *GoroutineTracker) logSummary(currentCount, increase int) {
	gt.mu.RLock()
	defer gt.mu.RUnlock()

	// Count by type
	countByType := make(map[string]int)
	for _, tg := range gt.trackedGoroutines {
		countByType[tg.Type]++
	}

	gt.logger.Debug("Goroutine status",
		zap.Int("total_goroutines", currentCount),
		zap.Int("baseline", gt.baselineCount),
		zap.Int("increase", increase),
		zap.Int("tracked_count", len(gt.trackedGoroutines)),
		zap.Any("by_type", countByType),
	)
}

// Go is a helper that starts a tracked goroutine
// Automatically handles tracking and untracking
func (gt *GoroutineTracker) Go(goroutineType string, fn func(ctx context.Context)) {
	id := generateID()
	_ = gt.Track(id, goroutineType)

	go func() {
		defer gt.Untrack(id)

		// Create context that can be cancelled
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		fn(ctx)
	}()
}

// GoWithContext starts a tracked goroutine with provided context
func (gt *GoroutineTracker) GoWithContext(ctx context.Context, goroutineType string, fn func(ctx context.Context)) {
	id := generateID()
	_ = gt.Track(id, goroutineType)

	go func() {
		defer gt.Untrack(id)
		fn(ctx)
	}()
}

// Stats returns current tracking statistics
type Stats struct {
	TotalGoroutines   int
	BaselineGoroutines int
	Increase          int
	TrackedCount      int
	ByType            map[string]int
}

// GetStats returns current goroutine statistics
func (gt *GoroutineTracker) GetStats() Stats {
	gt.mu.RLock()
	defer gt.mu.RUnlock()

	currentCount := runtime.NumGoroutine()
	increase := currentCount - gt.baselineCount

	countByType := make(map[string]int)
	for _, tg := range gt.trackedGoroutines {
		countByType[tg.Type]++
	}

	return Stats{
		TotalGoroutines:    currentCount,
		BaselineGoroutines: gt.baselineCount,
		Increase:           increase,
		TrackedCount:       len(gt.trackedGoroutines),
		ByType:             countByType,
	}
}

// generateID generates a unique ID for tracking
func generateID() string {
	return fmt.Sprintf("gr-%d", time.Now().UnixNano())
}

// Dump returns a list of all currently tracked goroutines (for debugging)
func (gt *GoroutineTracker) Dump() []TrackedGoroutine {
	gt.mu.RLock()
	defer gt.mu.RUnlock()

	result := make([]TrackedGoroutine, 0, len(gt.trackedGoroutines))
	for _, tg := range gt.trackedGoroutines {
		result = append(result, *tg)
	}

	return result
}
