# P2 Optimization Integration Guide

**Status**: All P2 components implemented and tested ✅
**Next Step**: Integration into main application

## Components Ready for Integration

### 1. Caching Layer (P2-1, P2-2)

**Files**:
- `internal/services/merchant/credential_cache.go`
- `internal/services/payment_method/payment_method_cache.go`

**Integration Points**:
```go
// In cmd/server/main.go or service initialization:

// Initialize merchant credential cache
merchantCache := merchant.NewMerchantCredentialCache(
    queries,
    secretManager,
    logger,
    5*time.Minute,  // TTL
    1000,           // Max size
)

// Initialize payment method cache
pmCache := payment_method.NewPaymentMethodCache(
    queries,
    logger,
    2*time.Minute,  // TTL
    10000,          // Max size
)

// Update service constructors to use caches
paymentService := payment.NewService(
    queries,
    merchantCache,  // Instead of direct queries
    pmCache,        // Instead of direct queries
    epxAdapter,
    logger,
)
```

**Impact**: 70% reduction in merchant config queries, 60% reduction in payment method queries

---

### 2. HTTP/2 & Connection Pooling (P2-3)

**Files**:
- `pkg/http/client.go`

**Integration Points**:
```go
// Replace EPX HTTP client initialization:
epxHTTPClient := pkghttp.NewHTTPClient(
    pkghttp.EPXClientConfig(),
    30*time.Second, // Request timeout
)

// Replace webhook HTTP client initialization:
webhookHTTPClient := pkghttp.NewHTTPClient(
    pkghttp.WebhookClientConfig(),
    10*time.Second, // Request timeout
)

// Pass to adapters:
epxAdapter := epx.NewAdapter(epxHTTPClient, ...)
webhookService := webhook.NewService(webhookHTTPClient, ...)
```

**Impact**: 90%+ connection reuse, 20-30% latency reduction

---

### 3. Response Compression (P2-4)

**Files**:
- `pkg/middleware/compression.go`

**Integration Points**:
```go
// In HTTP server setup (cmd/server/main.go):
import pkgmiddleware "github.com/kevin07696/payment-service/pkg/middleware"

// Apply compression middleware to HTTP servers:
mux := http.NewServeMux()

// Wrap with compression
compressedMux := pkgmiddleware.GzipHandler(
    pkgmiddleware.GzipDefaultLevel,
    logger,
)(mux)

// Or with custom config:
gzipConfig := pkgmiddleware.DefaultGzipConfig()
gzipConfig.ExcludedPaths = []string{"/health", "/metrics", "/ready"}
compressedMux := pkgmiddleware.GzipHandlerWithCustomConfig(gzipConfig, logger)(mux)

httpServer := &http.Server{
    Handler: compressedMux,
    ...
}
```

**Impact**: 40-60% bandwidth reduction, 60-80% JSON response compression

---

### 4. Graceful Shutdown (P2-5)

**Files**:
- `pkg/shutdown/manager.go`
- `pkg/shutdown/inflight.go`

**Integration Points**:
```go
// In cmd/server/main.go:
import pkgshutdown "github.com/kevin07696/payment-service/pkg/shutdown"

func main() {
    // Create shutdown manager
    shutdownMgr := pkgshutdown.NewManager(logger, 30*time.Second)

    // Register components in order (shut down in REVERSE):
    // 1. Database (registered first, shut down last)
    shutdownMgr.RegisterCloser("database", dbPool)

    // 2. Services
    shutdownMgr.Register("payment_service", paymentService.Shutdown)
    shutdownMgr.Register("webhook_service", webhookService.Shutdown)

    // 3. HTTP servers (registered last, shut down first)
    shutdownMgr.RegisterHTTPServer("http_server", httpServer)
    shutdownMgr.RegisterHTTPServer("grpc_server", grpcServer)

    // 4. Background workers
    cronWorker := pkgshutdown.NewPeriodicWorker(
        "ach_verification_cron",
        5*time.Minute,
        logger,
    )
    cronWorker.Start(func(ctx context.Context) {
        // Cron work here
    })
    shutdownMgr.Register("cron_worker", cronWorker.Shutdown)

    // Create in-flight tracker for HTTP requests
    inflightTracker := pkgshutdown.NewInFlightTracker("http_requests", logger)

    // Wrap HTTP handlers to track in-flight work:
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !inflightTracker.Add() {
            http.Error(w, "Service shutting down", http.StatusServiceUnavailable)
            return
        }
        defer inflightTracker.Done()

        // Handle request normally
        actualHandler.ServeHTTP(w, r)
    })

    // Wait for shutdown signal
    shutdownMgr.WaitForShutdown()
}
```

**Impact**: Zero-downtime deployments, proper cleanup order, no data loss on shutdown

---

### 5. Goroutine Leak Detection (P2-6)

**Files**:
- `pkg/resourcemgmt/goroutine_tracker.go`

**Integration Points**:
```go
// In cmd/server/main.go:
import "github.com/kevin07696/payment-service/pkg/resourcemgmt"

func main() {
    // Initialize goroutine tracker
    goroutineTracker := resourcemgmt.NewGoroutineTracker(
        logger,
        resourcemgmt.DefaultConfig(),
    )

    // Start monitoring (will run until context cancelled)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go goroutineTracker.StartMonitoring(ctx)

    // Use tracker for webhook deliveries:
    webhookService := webhook.NewService(
        goroutineTracker, // Pass tracker
        httpClient,
        logger,
    )

    // In webhook delivery code:
    goroutineTracker.Go("webhook_delivery", func(ctx context.Context) {
        // Webhook delivery work
        deliverWebhook(ctx, ...)
    })

    // Or with context:
    goroutineTracker.GoWithContext(ctx, "subscription_billing", func(ctx context.Context) {
        // Billing work
        processBilling(ctx, ...)
    })

    // For manual tracking:
    id := "custom-work-123"
    tg := goroutineTracker.Track(id, "custom_work")
    go func() {
        defer goroutineTracker.Untrack(id)
        // Do work
    }()
}
```

**Impact**: Early leak detection (30s), debugging support, prevention through lifecycle management

---

## Integration Checklist

- [ ] Add cache initialization to service constructors
- [ ] Replace HTTP client creation with optimized configs
- [ ] Apply compression middleware to HTTP servers
- [ ] Integrate shutdown manager into main.go
- [ ] Add in-flight tracking to HTTP handlers
- [ ] Initialize goroutine tracker and start monitoring
- [ ] Update webhook/cron services to use goroutine tracker
- [ ] Add Prometheus metrics endpoints if not already present
- [ ] Test graceful shutdown with running requests
- [ ] Verify cache hit rates in metrics
- [ ] Monitor goroutine count stability

## Metrics to Monitor

**Caching**:
- `merchant_cache_hits_total{merchant_id}` - Cache hits by merchant
- `merchant_cache_misses_total{reason}` - Cache misses (expired, not_found)
- `payment_method_cache_hits_total` - Payment method cache hits
- `payment_method_cache_size` - Current cache size

**Shutdown**:
- `graceful_shutdowns_total` - Total graceful shutdowns
- `shutdown_duration_seconds` - Total shutdown time
- `component_shutdown_duration_seconds{component}` - Per-component shutdown time
- `shutdown_errors_total{component}` - Shutdown errors

**Goroutines**:
- `goroutines_count` - Total goroutines in process
- `goroutine_leaks_detected_total` - Potential leak detections
- `tracked_goroutines{type}` - Tracked goroutines by type
- `long_running_goroutines{type}` - Long-running goroutines by type

## Performance Expectations

Based on P2 optimization targets:

| Optimization | Metric | Expected Improvement |
|-------------|--------|---------------------|
| Merchant Config Cache | DB queries/sec | -70% (950 → 285 at 1000 TPS) |
| Payment Method Cache | DB queries/sec | -60% (800 → 320 at 1000 TPS) |
| HTTP/2 Connection Pooling | EPX latency | -20-30% (300ms → 210-240ms) |
| Response Compression | Bandwidth | -40-60% (10KB → 4-6KB JSON) |
| Graceful Shutdown | Deployment downtime | 100% (5-30s → 0s) |
| Goroutine Leak Detection | Memory leak time-to-detection | 30s monitoring interval |

## Testing Plan

1. **Load Testing**:
   - Run 1000 TPS for 10 minutes
   - Verify cache hit rates >90%
   - Monitor goroutine count stability
   - Confirm connection reuse >90%

2. **Graceful Shutdown**:
   - Start 100 in-flight requests
   - Send SIGTERM
   - Verify all requests complete
   - Confirm zero errors

3. **Goroutine Leak Detection**:
   - Intentionally leak goroutines
   - Verify alert within 30s
   - Check Prometheus metrics accuracy

4. **Compression**:
   - Measure response sizes before/after
   - Verify 60-80% JSON compression
   - Check excluded paths don't compress

---

**Next Action**: Integrate components into `cmd/server/main.go` and run integration tests.
