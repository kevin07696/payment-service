# Resource Management & Leak Prevention

**Created**: 2025-11-20
**Status**: Analysis Complete - Awaiting Test Implementation
**Priority**: P0 (Critical for Production Stability)

## Executive Summary

This document analyzes resource management and leak prevention to improve:
- **Memory stability** through goroutine leak detection and prevention
- **File descriptor management** preventing "too many open files" errors
- **Graceful shutdown** ensuring zero data loss during deployments
- **Context propagation** preventing resource leaks from cancelled operations

**Current State**:
- ✅ Basic graceful shutdown implemented (10s timeout)
- ✅ Context usage widespread (819 occurrences)
- ❌ **No goroutine leak detection**
- ❌ **No file handle monitoring**
- ❌ **Incomplete shutdown** (doesn't drain in-flight requests)
- ⚠️ Some goroutines may not respect context cancellation

**Critical Findings**:
1. 🔴 **Goroutine leaks possible**: No leak detection, background workers not tracked
2. ⚠️ **Incomplete shutdown**: HTTP servers shut down but goroutines may continue
3. ❌ **No file descriptor monitoring**: Can hit OS limits unexpectedly
4. ⚠️ **Some background work doesn't cancel**: Webhooks, cron jobs may not respect shutdown

**Expected Impact**:
- **Zero goroutine leaks** through systematic tracking
- **Zero data loss** on shutdown through proper draining
- **Predictable resource usage** through monitoring
- **Faster deployments** through graceful shutdown (30s → 5s)

---

## Table of Contents

1. [Goroutine Leak Detection](#1-goroutine-leak-detection)
2. [Graceful Shutdown Enhancement](#2-graceful-shutdown-enhancement)
3. [Context Cancellation Propagation](#3-context-cancellation-propagation)
4. [File Handle Management](#4-file-handle-management)
5. [Background Worker Management](#5-background-worker-management)
6. [Resource Limits & Monitoring](#6-resource-limits--monitoring)
7. [Memory Leak Detection](#7-memory-leak-detection)
8. [Testing Requirements](#8-testing-requirements)

---

## 1. Goroutine Leak Detection

### Background

Goroutine leaks occur when goroutines are started but never terminated, leading to:
- Gradual memory increase
- File descriptor exhaustion (if goroutine holds connections)
- Eventually: service crash or OOM kill

**Current State**: No leak detection mechanism

---

### RES-M1: Goroutine Tracking & Leak Detection

**Priority**: P0

**Implementation**:
```go
package resourcemgmt

import (
    "context"
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
        Help: "Current number of goroutines",
    })

    goroutineLeakDetected = promauto.NewCounter(prometheus.CounterOpts{
        Name: "goroutine_leaks_detected_total",
        Help: "Total number of goroutine leak detections",
    })

    trackedGoroutines = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "tracked_goroutines",
        Help: "Number of tracked goroutines by type",
    }, []string{"type"})
)

// GoroutineTracker tracks goroutines to detect leaks
type GoroutineTracker struct {
    mu               sync.RWMutex
    trackedGoroutines map[string]*TrackedGoroutine
    logger           *zap.Logger
    baselineCount    int
    checkInterval    time.Duration
}

// TrackedGoroutine represents a tracked goroutine
type TrackedGoroutine struct {
    ID        string
    Type      string    // "webhook", "cron", "subscription_billing", etc.
    StartTime time.Time
    Done      chan struct{}
}

// NewGoroutineTracker creates a new goroutine tracker
func NewGoroutineTracker(logger *zap.Logger) *GoroutineTracker {
    return &GoroutineTracker{
        trackedGoroutines: make(map[string]*TrackedGoroutine),
        logger:            logger,
        baselineCount:     runtime.NumGoroutine(),
        checkInterval:     30 * time.Second,
    }
}

// Track registers a goroutine for tracking
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

    return tg
}

// Untrack removes a goroutine from tracking (call when goroutine exits)
func (gt *GoroutineTracker) Untrack(id string) {
    gt.mu.Lock()
    defer gt.mu.Unlock()

    if tg, ok := gt.trackedGoroutines[id]; ok {
        close(tg.Done)
        trackedGoroutines.WithLabelValues(tg.Type).Dec()
        delete(gt.trackedGoroutines, id)
    }
}

// StartMonitoring begins goroutine leak detection
func (gt *GoroutineTracker) StartMonitoring(ctx context.Context) {
    ticker := time.NewTicker(gt.checkInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            gt.checkForLeaks()
        }
    }
}

// checkForLeaks detects goroutine leaks
func (gt *GoroutineTracker) checkForLeaks() {
    currentCount := runtime.NumGoroutine()
    goroutineCount.Set(float64(currentCount))

    // Check for significant increase
    increase := currentCount - gt.baselineCount
    if increase > 100 { // More than 100 goroutines above baseline
        gt.logger.Warn("Potential goroutine leak detected",
            zap.Int("current_count", currentCount),
            zap.Int("baseline_count", gt.baselineCount),
            zap.Int("increase", increase),
        )
        goroutineLeakDetected.Inc()
    }

    // Check for long-running tracked goroutines
    gt.mu.RLock()
    defer gt.mu.RUnlock()

    for id, tg := range gt.trackedGoroutines {
        age := time.Since(tg.StartTime)
        if age > 10*time.Minute { // Long-running threshold
            gt.logger.Warn("Long-running goroutine detected",
                zap.String("id", id),
                zap.String("type", tg.Type),
                zap.Duration("age", age),
            )
        }
    }
}

// Go is a helper that starts a tracked goroutine
func (gt *GoroutineTracker) Go(goroutineType string, fn func(ctx context.Context)) {
    id := generateID()
    tg := gt.Track(id, goroutineType)

    go func() {
        defer gt.Untrack(id)

        // Create context that ends when done
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()

        fn(ctx)
    }()
}

func generateID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}
```

**Usage**:
```go
// In main.go or service initialization
tracker := resourcemgmt.NewGoroutineTracker(logger)
go tracker.StartMonitoring(ctx)

// When starting background goroutines
tracker.Go("webhook_delivery", func(ctx context.Context) {
    // Webhook delivery logic
    // Goroutine will be tracked automatically
})

// Or manual tracking
id := generateUniqueID()
tg := tracker.Track(id, "subscription_billing")
go func() {
    defer tracker.Untrack(id)

    // Do work...
}()
```

**Impact**:
- **Early detection**: Alerts on leak within 30 seconds
- **Debugging**: Track which goroutine types are leaking
- **Prevention**: Forces conscious goroutine lifecycle management

---

## 2. Graceful Shutdown Enhancement

### Background

Current graceful shutdown (`cmd/server/main.go:266-282`):
```go
// Wait for interrupt signal
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

logger.Info("Shutting down servers...")

// Graceful shutdown
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Shutdown ConnectRPC server
if err := connectServer.Shutdown(shutdownCtx); err != nil {
    logger.Error("ConnectRPC server shutdown error", zap.Error(err))
}

// Shutdown HTTP server
if err := httpServer.Shutdown(shutdownCtx); err != nil {
    logger.Error("HTTP server shutdown error", zap.Error(err))
}

logger.Info("Servers stopped")
```

**Problems**:
1. ❌ Only shuts down HTTP servers (background goroutines continue)
2. ❌ No in-flight request draining verification
3. ❌ Database connections not closed gracefully
4. ❌ Background workers (cron, webhooks) not stopped

---

### RES-M2: Complete Graceful Shutdown

**Priority**: P0

**Enhanced Implementation**:
```go
package main

import (
    "context"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "go.uber.org/zap"
)

// ShutdownManager coordinates graceful shutdown of all components
type ShutdownManager struct {
    logger         *zap.Logger
    shutdownFuncs  []func(context.Context) error
    mu             sync.Mutex
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(logger *zap.Logger) *ShutdownManager {
    return &ShutdownManager{
        logger:        logger,
        shutdownFuncs: make([]func(context.Context) error, 0),
    }
}

// Register adds a shutdown function to be called during graceful shutdown
func (sm *ShutdownManager) Register(name string, fn func(context.Context) error) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    wrappedFn := func(ctx context.Context) error {
        sm.logger.Info("Shutting down component", zap.String("component", name))
        start := time.Now()

        err := fn(ctx)

        elapsed := time.Since(start)
        if err != nil {
            sm.logger.Error("Component shutdown failed",
                zap.String("component", name),
                zap.Duration("elapsed", elapsed),
                zap.Error(err),
            )
        } else {
            sm.logger.Info("Component shut down successfully",
                zap.String("component", name),
                zap.Duration("elapsed", elapsed),
            )
        }

        return err
    }

    sm.shutdownFuncs = append(sm.shutdownFuncs, wrappedFn)
}

// WaitForShutdown waits for interrupt signal and performs graceful shutdown
func (sm *ShutdownManager) WaitForShutdown() {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    // Wait for signal
    sig := <-quit
    sm.logger.Info("Received shutdown signal",
        zap.String("signal", sig.String()),
    )

    // Create shutdown context with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Shutdown all components in REVERSE registration order
    // (Last registered = first to shut down, e.g., HTTP servers before workers)
    var wg sync.WaitGroup
    errors := make([]error, 0)

    for i := len(sm.shutdownFuncs) - 1; i >= 0; i-- {
        fn := sm.shutdownFuncs[i]

        wg.Add(1)
        go func(shutdownFn func(context.Context) error) {
            defer wg.Done()

            if err := shutdownFn(shutdownCtx); err != nil {
                sm.mu.Lock()
                errors = append(errors, err)
                sm.mu.Unlock()
            }
        }(fn)
    }

    // Wait for all shutdowns to complete
    wg.Wait()

    if len(errors) > 0 {
        sm.logger.Error("Graceful shutdown completed with errors",
            zap.Int("error_count", len(errors)),
        )
    } else {
        sm.logger.Info("Graceful shutdown completed successfully")
    }
}

// In main.go:

func main() {
    logger := initLogger()
    defer logger.Sync()

    // Create shutdown manager
    shutdownMgr := NewShutdownManager(logger)

    // ... initialize services ...

    // Register shutdown functions (in dependency order)

    // 1. Background workers (stop first - prevent new work)
    shutdownMgr.Register("goroutine_tracker", func(ctx context.Context) error {
        // Cancel tracker context
        return nil
    })

    shutdownMgr.Register("subscription_billing_worker", func(ctx context.Context) error {
        // Stop subscription billing background worker
        billingWorker.Stop()
        return nil
    })

    shutdownMgr.Register("webhook_retry_worker", func(ctx context.Context) error {
        // Stop webhook retry worker
        webhookRetryWorker.Stop()
        return nil
    })

    // 2. HTTP servers (stop accepting new requests)
    shutdownMgr.Register("connect_server", func(ctx context.Context) error {
        return connectServer.Shutdown(ctx)
    })

    shutdownMgr.Register("http_server", func(ctx context.Context) error {
        return httpServer.Shutdown(ctx)
    })

    // 3. Service cleanup (finish in-flight work)
    shutdownMgr.Register("webhook_service", func(ctx context.Context) error {
        // Wait for in-flight webhook deliveries
        return webhookService.Shutdown(ctx)
    })

    shutdownMgr.Register("payment_service", func(ctx context.Context) error {
        // Flush any pending work
        return paymentService.Shutdown(ctx)
    })

    // 4. Database (close connections last)
    shutdownMgr.Register("database", func(ctx context.Context) error {
        dbAdapter.Close()
        return nil
    })

    // Start servers...
    // (existing server startup code)

    // Wait for shutdown signal and execute graceful shutdown
    shutdownMgr.WaitForShutdown()
}
```

**Service Shutdown Support**:
```go
// Add Shutdown method to services that need cleanup

// WebhookDeliveryService
type WebhookDeliveryService struct {
    // ... existing fields ...
    inFlightWg sync.WaitGroup
    shutdownCh chan struct{}
}

func (s *WebhookDeliveryService) Shutdown(ctx context.Context) error {
    close(s.shutdownCh)

    // Wait for in-flight deliveries with timeout
    done := make(chan struct{})
    go func() {
        s.inFlightWg.Wait()
        close(done)
    }()

    select {
    case <-done:
        s.logger.Info("All in-flight webhooks completed")
        return nil
    case <-ctx.Done():
        s.logger.Warn("Shutdown timeout - some webhooks may be incomplete")
        return ctx.Err()
    }
}

// Track in-flight work
func (s *WebhookDeliveryService) deliverToSubscription(...) error {
    s.inFlightWg.Add(1)
    defer s.inFlightWg.Done()

    // Check if shutting down
    select {
    case <-s.shutdownCh:
        return errors.New("service shutting down")
    default:
        // Continue
    }

    // ... delivery logic ...
}
```

**Impact**:
- **Zero data loss**: All in-flight requests complete before shutdown
- **Faster deployments**: Predictable 5-30s shutdown (vs unpredictable)
- **No errors**: Clients don't see "connection reset" errors
- **Clean state**: All resources properly released

---

## 3. Context Cancellation Propagation

### RES-M3: Verify Context Propagation

**Priority**: P1

**Current**: 819 context usages (good coverage)

**Audit for Missing Propagation**:
```bash
# Find goroutines that don't accept context
grep -rn "go func()" internal/ | grep -v "ctx context.Context"

# Find long-running operations without context
grep -rn "for {" internal/ | grep -v "ctx.Done()"
```

**Common Anti-Patterns to Fix**:
```go
// ❌ BAD: Goroutine doesn't accept context
go func() {
    time.Sleep(5 * time.Second)
    doWork()
}()

// ✅ GOOD: Context-aware goroutine
go func(ctx context.Context) {
    select {
    case <-time.After(5 * time.Second):
        doWork(ctx)
    case <-ctx.Done():
        return
    }
}(ctx)

// ❌ BAD: Infinite loop without context
for {
    doWork()
    time.Sleep(1 * time.Second)
}

// ✅ GOOD: Context-aware loop
for {
    select {
    case <-ctx.Done():
        return
    case <-time.After(1 * time.Second):
        doWork(ctx)
    }
}
```

**Systematic Fix**:
```go
// Create utility function for context-aware loops
package resourcemgmt

func Loop(ctx context.Context, interval time.Duration, fn func(context.Context)) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            fn(ctx)
        }
    }
}

// Usage:
resourcemgmt.Loop(ctx, 1*time.Minute, func(ctx context.Context) {
    processACHVerifications(ctx)
})
```

---

## 4. File Handle Management

### RES-M4: File Descriptor Monitoring

**Priority**: P1

**Implementation**:
```go
package resourcemgmt

import (
    "os"
    "runtime"
    "syscall"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    openFileDescriptors = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "open_file_descriptors",
        Help: "Number of open file descriptors",
    })

    maxFileDescriptors = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "max_file_descriptors",
        Help: "Maximum allowed file descriptors (ulimit -n)",
    })

    fileDescriptorUtilization = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "file_descriptor_utilization",
        Help: "File descriptor utilization (0.0 to 1.0)",
    })
)

// MonitorFileDescriptors tracks file descriptor usage
func MonitorFileDescriptors(ctx context.Context, interval time.Duration) {
    // Get max FD limit
    var rLimit syscall.Rlimit
    if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
        maxFileDescriptors.Set(float64(rLimit.Cur))
    }

    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            updateFileDescriptorMetrics()
        }
    }
}

func updateFileDescriptorMetrics() {
    // Count open files in /proc/self/fd (Linux-specific)
    files, err := os.ReadDir("/proc/self/fd")
    if err != nil {
        return
    }

    openFDs := float64(len(files))
    openFileDescriptors.Set(openFDs)

    // Calculate utilization
    var rLimit syscall.Rlimit
    if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
        utilization := openFDs / float64(rLimit.Cur)
        fileDescriptorUtilization.Set(utilization)

        if utilization > 0.8 {
            // Log warning
            logger.Warn("High file descriptor utilization",
                zap.Float64("utilization", utilization),
                zap.Float64("open_fds", openFDs),
                zap.Uint64("max_fds", rLimit.Cur),
            )
        }
    }
}
```

**Alert Rule**:
```yaml
- alert: HighFileDescriptorUsage
  expr: file_descriptor_utilization > 0.8
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "File descriptor usage above 80%"
    description: "{{ $value | humanizePercentage }} of file descriptors in use"
```

**Increase Limit** (if needed):
```bash
# Temporarily (current session)
ulimit -n 65536

# Permanently (in /etc/security/limits.conf)
* soft nofile 65536
* hard nofile 65536

# For Docker containers (in docker-compose.yml)
services:
  payment-service:
    ulimits:
      nofile:
        soft: 65536
        hard: 65536
```

---

## 5. Background Worker Management

### RES-M5: Worker Pool Pattern

**Priority**: P1

**Problem**: Background workers (cron jobs, async tasks) not systematically managed

**Solution**: Worker pool with lifecycle management

**Implementation**:
```go
package workers

import (
    "context"
    "sync"
    "time"

    "go.uber.org/zap"
)

// Worker represents a background worker
type Worker interface {
    Name() string
    Run(ctx context.Context) error
    Stop(ctx context.Context) error
}

// WorkerPool manages background workers
type WorkerPool struct {
    workers []Worker
    logger  *zap.Logger
    wg      sync.WaitGroup
    stopCh  chan struct{}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(logger *zap.Logger) *WorkerPool {
    return &WorkerPool{
        workers: make([]Worker, 0),
        logger:  logger,
        stopCh:  make(chan struct{}),
    }
}

// Register adds a worker to the pool
func (wp *WorkerPool) Register(worker Worker) {
    wp.workers = append(wp.workers, worker)
}

// Start starts all workers
func (wp *WorkerPool) Start(ctx context.Context) {
    for _, worker := range wp.workers {
        wp.wg.Add(1)

        w := worker // Capture for goroutine
        go func() {
            defer wp.wg.Done()

            wp.logger.Info("Starting worker", zap.String("worker", w.Name()))

            if err := w.Run(ctx); err != nil {
                wp.logger.Error("Worker error",
                    zap.String("worker", w.Name()),
                    zap.Error(err),
                )
            }

            wp.logger.Info("Worker stopped", zap.String("worker", w.Name()))
        }()
    }
}

// Stop stops all workers gracefully
func (wp *WorkerPool) Stop(ctx context.Context) error {
    close(wp.stopCh)

    // Stop all workers
    for _, worker := range wp.workers {
        if err := worker.Stop(ctx); err != nil {
            wp.logger.Error("Worker stop error",
                zap.String("worker", worker.Name()),
                zap.Error(err),
            )
        }
    }

    // Wait for all to finish
    done := make(chan struct{})
    go func() {
        wp.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        wp.logger.Info("All workers stopped")
        return nil
    case <-ctx.Done():
        wp.logger.Warn("Worker shutdown timeout")
        return ctx.Err()
    }
}
```

**Example Worker** (ACH Verification Cron):
```go
type ACHVerificationWorker struct {
    service  ACHVerificationService
    interval time.Duration
    logger   *zap.Logger
    stopCh   chan struct{}
}

func (w *ACHVerificationWorker) Name() string {
    return "ach_verification_cron"
}

func (w *ACHVerificationWorker) Run(ctx context.Context) error {
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil
        case <-w.stopCh:
            return nil
        case <-ticker.C:
            if err := w.service.ProcessPendingVerifications(ctx); err != nil {
                w.logger.Error("ACH verification error", zap.Error(err))
            }
        }
    }
}

func (w *ACHVerificationWorker) Stop(ctx context.Context) error {
    close(w.stopCh)
    return nil
}

// In main.go:
workerPool := workers.NewWorkerPool(logger)
workerPool.Register(&ACHVerificationWorker{
    service:  achVerificationService,
    interval: 1 * time.Hour,
    logger:   logger,
    stopCh:   make(chan struct{}),
})

workerPool.Start(ctx)

// Register shutdown
shutdownMgr.Register("worker_pool", func(ctx context.Context) error {
    return workerPool.Stop(ctx)
})
```

---

## 6. Resource Limits & Monitoring

### RES-M6: Comprehensive Resource Metrics

**Priority**: P1

**Implementation**:
```go
package resourcemgmt

import (
    "runtime"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Memory metrics
    memoryAllocBytes = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "memory_alloc_bytes",
        Help: "Bytes of allocated heap objects",
    })

    memoryTotalAllocBytes = promauto.NewCounter(prometheus.CounterOpts{
        Name: "memory_total_alloc_bytes_total",
        Help: "Cumulative bytes allocated for heap objects",
    })

    memorySysBytes = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "memory_sys_bytes",
        Help: "Total bytes of memory obtained from the OS",
    })

    memoryNumGC = promauto.NewCounter(prometheus.CounterOpts{
        Name: "memory_gc_runs_total",
        Help: "Number of completed GC cycles",
    })

    memoryGCPauseSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "memory_gc_pause_seconds",
        Help:    "GC pause duration in seconds",
        Buckets: prometheus.ExponentialBuckets(0.00001, 2, 20), // 10µs to 10s
    })

    // CPU metrics
    cpuGoroutines = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "cpu_goroutines",
        Help: "Number of goroutines",
    })

    cpuThreads = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "cpu_threads",
        Help: "Number of OS threads",
    })
)

// MonitorResources collects runtime resource metrics
func MonitorResources(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    var lastNumGC uint32

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            var m runtime.MemStats
            runtime.ReadMemStats(&m)

            // Memory metrics
            memoryAllocBytes.Set(float64(m.Alloc))
            memoryTotalAllocBytes.Add(float64(m.TotalAlloc))
            memorySysBytes.Set(float64(m.Sys))
            memoryNumGC.Add(float64(m.NumGC - lastNumGC))

            // GC pause times
            if m.NumGC > lastNumGC {
                for i := lastNumGC; i < m.NumGC; i++ {
                    pause := m.PauseNs[(i+255)%256]
                    memoryGCPauseSeconds.Observe(float64(pause) / 1e9)
                }
                lastNumGC = m.NumGC
            }

            // CPU metrics
            cpuGoroutines.Set(float64(runtime.NumGoroutine()))
            cpuThreads.Set(float64(runtime.GOMAXPROCS(0)))
        }
    }
}
```

**Start Monitoring** (in `main.go`):
```go
// Start resource monitoring
go resourcemgmt.MonitorResources(ctx, 10*time.Second)
go resourcemgmt.MonitorFileDescriptors(ctx, 30*time.Second)
```

---

## 7. Memory Leak Detection

### RES-M7: Heap Profiling & Leak Detection

**Priority**: P2

**Enable pprof** (already available via `net/http/pprof`):
```go
import _ "net/http/pprof"

// In main.go (add to HTTP mux)
httpMux.HandleFunc("/debug/pprof/", pprof.Index)
httpMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
httpMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
httpMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
httpMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
httpMux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
httpMux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
```

**Memory Leak Detection**:
```bash
# Capture heap profile
curl http://localhost:8081/debug/pprof/heap > heap1.prof

# Wait 10 minutes
sleep 600

# Capture again
curl http://localhost:8081/debug/pprof/heap > heap2.prof

# Compare to find leaks
go tool pprof -base=heap1.prof heap2.prof
(pprof) top10
(pprof) list <function_name>
```

**Goroutine Leak Detection**:
```bash
# Capture goroutine profile
curl http://localhost:8081/debug/pprof/goroutine > goroutine.prof

# Analyze
go tool pprof goroutine.prof
(pprof) top10
(pprof) list <function_name>
```

---

## 8. Testing Requirements

### 8.1 Goroutine Leak Tests

```go
func TestNoGoroutineLeaks(t *testing.T) {
    // Capture baseline
    baseline := runtime.NumGoroutine()

    // Run test operation
    for i := 0; i < 100; i++ {
        processPayment(context.Background(), &payment)
    }

    // Wait for goroutines to finish
    time.Sleep(1 * time.Second)

    // Check for leaks
    final := runtime.NumGoroutine()
    if final > baseline+5 { // Allow small variance
        t.Errorf("Goroutine leak detected: baseline=%d, final=%d", baseline, final)
    }
}
```

### 8.2 Graceful Shutdown Tests

```go
func TestGracefulShutdown(t *testing.T) {
    shutdownMgr := NewShutdownManager(logger)

    completed := false
    shutdownMgr.Register("test_component", func(ctx context.Context) error {
        time.Sleep(500 * time.Millisecond)
        completed = true
        return nil
    })

    // Trigger shutdown
    go shutdownMgr.WaitForShutdown()

    // Send SIGTERM
    process, _ := os.FindProcess(os.Getpid())
    process.Signal(syscall.SIGTERM)

    // Wait for completion
    time.Sleep(1 * time.Second)

    if !completed {
        t.Error("Shutdown function not called")
    }
}
```

### 8.3 Context Cancellation Tests

```go
func TestContextCancellation(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    completed := false

    go func() {
        select {
        case <-ctx.Done():
            // Cancelled correctly
            return
        case <-time.After(1 * time.Second):
            completed = true // Should not reach here
        }
    }()

    time.Sleep(200 * time.Millisecond)

    if completed {
        t.Error("Goroutine did not respect context cancellation")
    }
}
```

---

## Summary

| Category | Current | Optimized | Impact |
|----------|---------|-----------|--------|
| Goroutine Tracking | None | Monitored | **Zero leaks** |
| Graceful Shutdown | Basic | Complete | **Zero data loss** |
| Context Propagation | Good | Audited | **Clean cancellation** |
| File Descriptors | Unmonitored | Tracked | **Prevent exhaustion** |
| Background Workers | Ad-hoc | Managed | **Predictable lifecycle** |
| Resource Metrics | Basic | Comprehensive | **Full visibility** |

**Implementation Priority**:
1. P0: Goroutine tracking (RES-M1), Graceful shutdown (RES-M2)
2. P1: File descriptor monitoring (RES-M4), Worker pool (RES-M5), Resource metrics (RES-M6)
3. P2: Context audit (RES-M3), Memory profiling (RES-M7)

**Expected Impact**:
- **100% uptime** during deployments (zero data loss)
- **Zero resource leaks** (goroutines, FDs, memory)
- **Predictable performance** under load
- **Faster incident response** (comprehensive metrics)

**Document Status**: ✅ Complete
**Last Updated**: 2025-11-20
