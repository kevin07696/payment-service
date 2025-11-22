# Comprehensive Test Plan - P0/P1/P2 Optimizations

**Status**: Ready for Implementation
**Date**: 2025-11-22
**Follows**: Go Testing Best Practices & Project Conventions

---

## Testing Philosophy & Standards

### Go Testing Best Practices Applied:

1. **Table-Driven Tests**: Use `t.Run()` with subtests for multiple scenarios
2. **Clear Test Names**: `Test<Type>_<Method>_<Scenario>` format
3. **AAA Pattern**: Arrange-Act-Assert structure
4. **Test Helpers**: `testify/assert` and `testify/require` for assertions
5. **Parallel Execution**: Mark independent tests with `t.Parallel()`
6. **Benchmarks**: Performance-critical code has `Benchmark*` tests
7. **Race Detection**: All tests must pass with `-race` flag
8. **Coverage Target**: 80%+ for critical paths, 60%+ overall

### Project-Specific Conventions:

- ✅ Mock external dependencies (DB, Vault, HTTP)
- ✅ Use `testify/mock` for interfaces
- ✅ Context-aware tests (test context cancellation)
- ✅ Goroutine leak detection in concurrent tests
- ✅ Cleanup with `t.Cleanup()` instead of `defer`

---

## Phase 1: Critical Unit Tests (P0 Priority)

### 1. Merchant Credential Cache Tests

**File**: `internal/services/merchant/credential_cache_test.go`

**Test Coverage**:

```go
// Table of Contents:
// - Basic Operations
func TestMerchantCredentialCache_Get_CacheHit(t *testing.T)
func TestMerchantCredentialCache_Get_CacheMiss(t *testing.T)
func TestMerchantCredentialCache_Get_Expiration(t *testing.T)

// - Invalidation
func TestMerchantCredentialCache_Invalidate(t *testing.T)
func TestMerchantCredentialCache_InvalidateAll(t *testing.T)

// - LRU Eviction
func TestMerchantCredentialCache_EvictIfNeeded_MaxSizeExceeded(t *testing.T)
func TestMerchantCredentialCache_EvictIfNeeded_LRUOrdering(t *testing.T)

// - Thread Safety
func TestMerchantCredentialCache_Get_ConcurrentAccess(t *testing.T)
func TestMerchantCredentialCache_Get_RaceCondition(t *testing.T)

// - Error Handling
func TestMerchantCredentialCache_Get_DatabaseError(t *testing.T)
func TestMerchantCredentialCache_Get_VaultError(t *testing.T)
func TestMerchantCredentialCache_Get_ContextCancellation(t *testing.T)

// - Metrics
func TestMerchantCredentialCache_Metrics_CacheHits(t *testing.T)
func TestMerchantCredentialCache_Metrics_CacheMisses(t *testing.T)
func TestMerchantCredentialCache_Metrics_CacheSize(t *testing.T)
func TestMerchantCredentialCache_Metrics_Evictions(t *testing.T)

// - Benchmarks
func BenchmarkMerchantCredentialCache_Get_CacheHit(b *testing.B)
func BenchmarkMerchantCredentialCache_Get_CacheMiss(b *testing.B)
```

**Example Test Structure**:

```go
func TestMerchantCredentialCache_Get_CacheHit(t *testing.T) {
    t.Parallel()

    // Arrange
    mockQueries := &mockQuerier{}
    mockSecretMgr := &mockSecretManager{}
    logger := zaptest.NewLogger(t)

    cache := NewMerchantCredentialCache(
        mockQueries,
        mockSecretMgr,
        logger,
        5*time.Minute,
        100,
    )

    merchantID := uuid.New()
    expectedMerchant := sqlc.Merchant{
        ID:      merchantID,
        CustNbr: "9001",
        // ... other fields
    }

    // Pre-populate cache
    mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
        Return(expectedMerchant, nil).Once()
    mockSecretMgr.On("GetSecret", mock.Anything, mock.Anything).
        Return(&ports.Secret{Value: "test-secret"}, nil).Once()

    // First call to populate cache
    _, err := cache.Get(context.Background(), merchantID)
    require.NoError(t, err)

    // Act - Second call should hit cache
    start := time.Now()
    cached, err := cache.Get(context.Background(), merchantID)
    duration := time.Since(start)

    // Assert
    require.NoError(t, err)
    assert.NotNil(t, cached)
    assert.Equal(t, expectedMerchant.CustNbr, cached.merchant.CustNbr)

    // Verify cache was used (no additional DB/Vault calls)
    mockQueries.AssertNumberOfCalls(t, "GetMerchantByID", 1)
    mockSecretMgr.AssertNumberOfCalls(t, "GetSecret", 1)

    // Cache hit should be very fast (< 1ms)
    assert.Less(t, duration, time.Millisecond)
}

func TestMerchantCredentialCache_Get_ConcurrentAccess(t *testing.T) {
    t.Parallel()

    // Arrange
    cache := setupTestCache(t)
    merchantID := uuid.New()

    // Seed cache with data
    seedCache(t, cache, merchantID)

    // Act - 1000 concurrent reads
    const goroutines = 1000
    var wg sync.WaitGroup
    errChan := make(chan error, goroutines)

    for i := 0; i < goroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            _, err := cache.Get(context.Background(), merchantID)
            if err != nil {
                errChan <- err
            }
        }()
    }

    wg.Wait()
    close(errChan)

    // Assert - No errors, no panics
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }
    assert.Empty(t, errors, "Concurrent access should not produce errors")
}
```

**Test Data Helpers**:

```go
// internal/services/merchant/testdata/fixtures.go
package merchant_test

func TestMerchant(merchantID uuid.UUID) sqlc.Merchant {
    return sqlc.Merchant{
        ID:            merchantID,
        Slug:          "test-merchant",
        CustNbr:       "9001",
        MerchNbr:      "900300",
        DbaNbr:        "2",
        TerminalNbr:   "77",
        MacSecretPath: "payment-service/merchants/test-merchant/mac",
        Environment:   "sandbox",
        IsActive:      true,
    }
}

func TestCachedCredential(merchantID uuid.UUID, ttl time.Duration) *CachedCredential {
    now := time.Now()
    return &CachedCredential{
        merchant:  TestMerchant(merchantID),
        macSecret: "test-mac-secret-12345",
        cachedAt:  now,
        expiresAt: now.Add(ttl),
    }
}
```

---

### 2. Payment Method Cache Tests

**File**: `internal/services/payment_method/payment_method_cache_test.go`

**Test Coverage**:

```go
// Basic Operations
func TestPaymentMethodCache_Get_CacheHit(t *testing.T)
func TestPaymentMethodCache_Get_CacheMiss(t *testing.T)
func TestPaymentMethodCache_Get_Expiration(t *testing.T)

// Invalidation
func TestPaymentMethodCache_Invalidate(t *testing.T)
func TestPaymentMethodCache_InvalidateByCustomer(t *testing.T)
func TestPaymentMethodCache_InvalidateAll(t *testing.T)

// LRU Eviction
func TestPaymentMethodCache_EvictIfNeeded_MaxSize10000(t *testing.T)
func TestPaymentMethodCache_EvictIfNeeded_LRUOrdering(t *testing.T)
func TestPaymentMethodCache_EvictIfNeeded_Evict10Percent(t *testing.T)

// Thread Safety
func TestPaymentMethodCache_Get_ConcurrentReads(t *testing.T)
func TestPaymentMethodCache_Get_ConcurrentWrites(t *testing.T)
func TestPaymentMethodCache_Get_NoDoubleLocking(t *testing.T)

// Error Handling
func TestPaymentMethodCache_Get_DatabaseError(t *testing.T)
func TestPaymentMethodCache_Get_ContextCancellation(t *testing.T)

// Domain Conversion
func TestConvertSqlcToPaymentMethod_CreditCard(t *testing.T)
func TestConvertSqlcToPaymentMethod_ACH(t *testing.T)
func TestConvertSqlcToPaymentMethod_OptionalFields(t *testing.T)

// Metrics
func TestPaymentMethodCache_Metrics_HitRate(t *testing.T)
func TestPaymentMethodCache_Metrics_CacheSize(t *testing.T)

// Benchmarks
func BenchmarkPaymentMethodCache_Get_CacheHit(b *testing.B)
func BenchmarkPaymentMethodCache_Get_10000Concurrent(b *testing.B)
```

**Critical Test Cases**:

```go
func TestPaymentMethodCache_Get_NoDoubleLocking(t *testing.T) {
    // Test the fix for double-locking bug
    t.Parallel()

    cache := setupTestCache(t)
    pmID := uuid.New()

    // Seed cache
    seedPaymentMethodCache(t, cache, pmID)

    // Act - Concurrent reads should not deadlock
    done := make(chan struct{})
    go func() {
        defer close(done)

        for i := 0; i < 10000; i++ {
            _, err := cache.Get(context.Background(), pmID)
            require.NoError(t, err)
        }
    }()

    // Assert - Should complete within reasonable time (no deadlock)
    select {
    case <-done:
        // Success
    case <-time.After(5 * time.Second):
        t.Fatal("Test timed out - possible deadlock from double locking")
    }
}

func TestPaymentMethodCache_InvalidateByCustomer(t *testing.T) {
    t.Parallel()

    // Arrange
    cache := setupTestCache(t)
    customerID := "customer-123"

    // Create 5 payment methods for same customer
    var pmIDs []uuid.UUID
    for i := 0; i < 5; i++ {
        pmID := uuid.New()
        pmIDs = append(pmIDs, pmID)
        seedPaymentMethodWithCustomer(t, cache, pmID, customerID)
    }

    // Create 3 payment methods for different customer
    for i := 0; i < 3; i++ {
        pmID := uuid.New()
        seedPaymentMethodWithCustomer(t, cache, pmID, "other-customer")
    }

    // Act
    cache.InvalidateByCustomer(customerID)

    // Assert - Customer's PMs should be gone
    for _, pmID := range pmIDs {
        // This should trigger DB fetch (cache miss)
        mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
            Return(testPaymentMethod(pmID), nil).Once()

        _, err := cache.Get(context.Background(), pmID)
        require.NoError(t, err)
    }

    // Verify DB was called (cache was invalidated)
    mockQueries.AssertExpectations(t)
}
```

---

### 3. Graceful Shutdown Tests

**File**: `pkg/shutdown/manager_test.go`

**Test Coverage**:

```go
// Registration
func TestManager_Register_SingleComponent(t *testing.T)
func TestManager_Register_MultipleComponents(t *testing.T)
func TestManager_RegisterHTTPServer(t *testing.T)

// Shutdown Ordering
func TestManager_Shutdown_LIFOOrdering(t *testing.T)
func TestManager_Shutdown_ReverseRegistrationOrder(t *testing.T)

// Timeout Handling
func TestManager_Shutdown_ComponentTimeout(t *testing.T)
func TestManager_Shutdown_OverallTimeout(t *testing.T)

// Error Handling
func TestManager_Shutdown_ComponentError(t *testing.T)
func TestManager_Shutdown_ContinuesOnError(t *testing.T)

// HTTP Server Shutdown
func TestManager_Shutdown_HTTPServerGraceful(t *testing.T)
func TestManager_Shutdown_HTTPServerWithActiveConnections(t *testing.T)

// Signal Handling
func TestManager_WaitForShutdown_SIGTERM(t *testing.T)
func TestManager_WaitForShutdown_SIGINT(t *testing.T)

// Metrics
func TestManager_Metrics_ShutdownDuration(t *testing.T)
func TestManager_Metrics_ComponentDuration(t *testing.T)
func TestManager_Metrics_ErrorCount(t *testing.T)
```

**Example LIFO Test**:

```go
func TestManager_Shutdown_LIFOOrdering(t *testing.T) {
    t.Parallel()

    // Arrange
    logger := zaptest.NewLogger(t)
    mgr := NewManager(logger, 10*time.Second)

    var shutdownOrder []string
    var mu sync.Mutex

    recordShutdown := func(name string) func(context.Context) error {
        return func(ctx context.Context) error {
            mu.Lock()
            defer mu.Unlock()
            shutdownOrder = append(shutdownOrder, name)
            return nil
        }
    }

    // Register components in order: A, B, C
    mgr.Register("component_A", recordShutdown("A"))
    mgr.Register("component_B", recordShutdown("B"))
    mgr.Register("component_C", recordShutdown("C"))

    // Act - Trigger shutdown
    mgr.Shutdown()

    // Assert - Should shut down in reverse order: C, B, A
    expectedOrder := []string{"C", "B", "A"}
    assert.Equal(t, expectedOrder, shutdownOrder,
        "Components should shut down in LIFO (reverse registration) order")
}
```

**File**: `pkg/shutdown/inflight_test.go`

**Test Coverage**:

```go
// Basic Operations
func TestInFlightTracker_Add_BeforeShutdown(t *testing.T)
func TestInFlightTracker_Add_AfterShutdown(t *testing.T)
func TestInFlightTracker_Done_DecrementsCount(t *testing.T)

// Shutdown Behavior
func TestInFlightTracker_Shutdown_WaitsForInFlight(t *testing.T)
func TestInFlightTracker_Shutdown_RejectsNewWork(t *testing.T)
func TestInFlightTracker_Shutdown_Timeout(t *testing.T)

// Background Workers
func TestBackgroundWorker_Start_StopsOnContextCancel(t *testing.T)
func TestBackgroundWorker_Shutdown_WaitsForCompletion(t *testing.T)

// Periodic Workers
func TestPeriodicWorker_Start_RunsImmediately(t *testing.T)
func TestPeriodicWorker_Start_RunsPeriodically(t *testing.T)
func TestPeriodicWorker_Shutdown_StopsCleanly(t *testing.T)
```

**Example In-Flight Test**:

```go
func TestInFlightTracker_Shutdown_WaitsForInFlight(t *testing.T) {
    t.Parallel()

    // Arrange
    logger := zaptest.NewLogger(t)
    tracker := NewInFlightTracker("test_tracker", logger)

    const workers = 100
    workDuration := 100 * time.Millisecond

    // Start 100 workers
    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        if !tracker.Add() {
            t.Fatal("Add() should succeed before shutdown")
        }

        wg.Add(1)
        go func() {
            defer wg.Done()
            defer tracker.Done()

            // Simulate work
            time.Sleep(workDuration)
        }()
    }

    // Wait for all workers to start
    time.Sleep(10 * time.Millisecond)

    // Act - Initiate shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    shutdownStart := time.Now()
    err := tracker.Shutdown(ctx)
    shutdownDuration := time.Since(shutdownStart)

    // Assert
    require.NoError(t, err, "Shutdown should wait for all workers")
    assert.GreaterOrEqual(t, shutdownDuration, workDuration,
        "Shutdown should wait for workers to complete")

    // Verify all workers completed
    wg.Wait()

    // Verify new work is rejected
    assert.False(t, tracker.Add(), "Add() should fail after shutdown")
}
```

---

### 4. Compression Middleware Tests

**File**: `pkg/middleware/compression_test.go`

**Test Coverage**:

```go
// Basic Compression
func TestGzipHandler_CompressesJSON(t *testing.T)
func TestGzipHandler_CompressesHTML(t *testing.T)
func TestGzipHandler_SkipsImages(t *testing.T)

// Accept-Encoding Header
func TestGzipHandler_RequiresAcceptEncoding(t *testing.T)
func TestGzipHandler_SkipsWithoutAcceptEncoding(t *testing.T)

// Excluded Paths
func TestGzipHandler_ExcludesHealthEndpoints(t *testing.T)
func TestGzipHandler_ExcludesMetrics(t *testing.T)

// Content-Type Handling (BUG FIX)
func TestGzipHandler_ChecksContentTypeAfterHandler(t *testing.T)
func TestGzipHandler_SkipsNonCompressible(t *testing.T)

// Headers
func TestGzipHandler_SetsContentEncoding(t *testing.T)
func TestGzipHandler_RemovesContentLength(t *testing.T)
func TestGzipHandler_SetsVaryHeader(t *testing.T)

// Writer Pool
func TestGzipHandler_ReusesWriters(t *testing.T)
func TestGzipHandler_ClosesWriters(t *testing.T)

// Status Codes
func TestGzipHandler_Compresses200(t *testing.T)
func TestGzipHandler_Compresses500(t *testing.T)

// Benchmarks
func BenchmarkGzipHandler_Compression(b *testing.B)
func BenchmarkGzipHandler_NoCompression(b *testing.B)
func BenchmarkGzipHandler_WriterPoolReuse(b *testing.B)
```

**Example Compression Test**:

```go
func TestGzipHandler_CompressesJSON(t *testing.T) {
    t.Parallel()

    // Arrange
    logger := zaptest.NewLogger(t)
    handler := GzipHandler(GzipDefaultLevel, logger)

    jsonData := `{"key":"value","array":[1,2,3]}`
    testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(jsonData))
    })

    wrappedHandler := handler(testHandler)

    // Act
    req := httptest.NewRequest("GET", "/api/test", nil)
    req.Header.Set("Accept-Encoding", "gzip")

    rec := httptest.NewRecorder()
    wrappedHandler.ServeHTTP(rec, req)

    // Assert
    assert.Equal(t, http.StatusOK, rec.Code)
    assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
    assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))
    assert.Empty(t, rec.Header().Get("Content-Length"),
        "Content-Length should be removed")

    // Decompress and verify
    reader, err := gzip.NewReader(rec.Body)
    require.NoError(t, err)
    defer reader.Close()

    decompressed, err := io.ReadAll(reader)
    require.NoError(t, err)

    assert.Equal(t, jsonData, string(decompressed))

    // Verify compression ratio
    originalSize := len(jsonData)
    compressedSize := rec.Body.Len()
    ratio := float64(compressedSize) / float64(originalSize)

    assert.Less(t, ratio, 0.8, "Compression should achieve >20% reduction")
}

func TestGzipHandler_ExcludesHealthEndpoints(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name string
        path string
        shouldCompress bool
    }{
        {"Regular API", "/api/payment", true},
        {"Cron Health", "/cron/health", false},
        {"ACH Health", "/cron/ach/health", false},
        {"Metrics", "/metrics", true}, // Not excluded
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            logger := zaptest.NewLogger(t)
            config := DefaultGzipConfig()
            config.ExcludedPaths = []string{"/cron/health", "/cron/ach/health"}

            handler := GzipHandlerWithCustomConfig(config, logger)

            testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "application/json")
                w.Write([]byte(`{"status":"ok"}`))
            })

            wrappedHandler := handler(testHandler)

            // Act
            req := httptest.NewRequest("GET", tt.path, nil)
            req.Header.Set("Accept-Encoding", "gzip")

            rec := httptest.NewRecorder()
            wrappedHandler.ServeHTTP(rec, req)

            // Assert
            if tt.shouldCompress {
                assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
            } else {
                assert.Empty(t, rec.Header().Get("Content-Encoding"))
            }
        })
    }
}
```

---

## Phase 2: Supporting Unit Tests (P1 Priority)

### 5. HTTP Client Configuration Tests

**File**: `pkg/http/client_test.go`

**Test Coverage**:

```go
// Configuration Validation
func TestEPXClientConfig_Settings(t *testing.T)
func TestWebhookClientConfig_Settings(t *testing.T)
func TestDefaultClientConfig_Settings(t *testing.T)

// Client Creation
func TestNewHTTPClient_EPXConfig(t *testing.T)
func TestNewHTTPClient_WebhookConfig(t *testing.T)
func TestNewHTTPClient_HTTP2Enabled(t *testing.T)

// TLS Configuration
func TestNewHTTPClient_TLSVersion(t *testing.T)
func TestNewHTTPClient_CipherSuites(t *testing.T)

// Timeout Settings
func TestNewHTTPClient_Timeouts(t *testing.T)
func TestNewHTTPClient_KeepAlive(t *testing.T)
```

---

### 6. Goroutine Tracker Tests

**File**: `pkg/resourcemgmt/goroutine_tracker_test.go`

**Test Coverage**:

```go
// Tracking
func TestGoroutineTracker_Track_Untrack(t *testing.T)
func TestGoroutineTracker_Go_AutoTrack(t *testing.T)
func TestGoroutineTracker_GoWithContext_Cancellation(t *testing.T)

// Leak Detection
func TestGoroutineTracker_CheckForLeaks_ThresholdExceeded(t *testing.T)
func TestGoroutineTracker_CheckForLeaks_BelowThreshold(t *testing.T)

// Long-Running Detection
func TestGoroutineTracker_CheckLongRunning_Alert(t *testing.T)
func TestGoroutineTracker_CheckLongRunning_NoAlert(t *testing.T)

// Monitoring
func TestGoroutineTracker_StartMonitoring_Interval(t *testing.T)
func TestGoroutineTracker_StartMonitoring_ContextCancel(t *testing.T)

// Metrics
func TestGoroutineTracker_Metrics_Count(t *testing.T)
func TestGoroutineTracker_Metrics_LeaksDetected(t *testing.T)
```

---

## Phase 3: Integration Tests (P2 Priority)

### 7. Cache Integration Tests

**File**: `tests/integration/cache/merchant_cache_integration_test.go`

**Test Coverage**:

```go
// +build integration

// End-to-End Cache Flow
func TestMerchantCache_Integration_PaymentFlow(t *testing.T)
func TestMerchantCache_Integration_HitRate(t *testing.T)
func TestMerchantCache_Integration_1000TPS(t *testing.T)

// Database Integration
func TestMerchantCache_Integration_DatabaseRoundtrip(t *testing.T)
func TestMerchantCache_Integration_VaultIntegration(t *testing.T)

// Performance
func TestMerchantCache_Integration_Latency(t *testing.T)
```

**File**: `tests/integration/cache/payment_method_cache_integration_test.go`

**Test Coverage**:

```go
// +build integration

// Service Integration
func TestPaymentMethodCache_Integration_GetPaymentMethod(t *testing.T)
func TestPaymentMethodCache_Integration_UpdateInvalidation(t *testing.T)
func TestPaymentMethodCache_Integration_BulkInvalidation(t *testing.T)

// Load Testing
func TestPaymentMethodCache_Integration_1000Concurrent(t *testing.T)
func TestPaymentMethodCache_Integration_HitRate90Percent(t *testing.T)
```

---

### 8. Shutdown Integration Tests

**File**: `tests/integration/shutdown/graceful_shutdown_test.go`

**Test Coverage**:

```go
// +build integration

// Zero-Downtime Deployment
func TestShutdown_Integration_ActiveRequests(t *testing.T)
func TestShutdown_Integration_NoDataLoss(t *testing.T)

// Real Server Shutdown
func TestShutdown_Integration_HTTPServerGraceful(t *testing.T)
func TestShutdown_Integration_100InFlightRequests(t *testing.T)
```

---

## Test Execution Strategy

### 1. Local Development

```bash
# Run all unit tests
go test ./... -v

# Run with race detection
go test ./... -race

# Run with coverage
go test ./... -cover -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out

# Run only cache tests
go test ./internal/services/merchant -run TestMerchantCredentialCache

# Run benchmarks
go test ./pkg/middleware -bench=. -benchmem
```

### 2. CI/CD Pipeline

```yaml
# .github/workflows/tests.yml
name: Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run unit tests
        run: go test ./... -v -race -coverprofile=coverage.out

      - name: Check coverage
        run: |
          total_coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          if (( $(echo "$total_coverage < 60" | bc -l) )); then
            echo "Coverage $total_coverage% is below 60% threshold"
            exit 1
          fi

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run integration tests
        run: go test ./tests/integration/... -tags=integration -v
        env:
          DATABASE_URL: postgres://postgres:postgres@localhost:5432/test?sslmode=disable
```

---

## Success Criteria

### Unit Tests:
- ✅ All tests pass with `-race` flag
- ✅ Coverage ≥ 80% for cache components
- ✅ Coverage ≥ 80% for shutdown components
- ✅ Coverage ≥ 60% for compression middleware
- ✅ All benchmarks complete without panics
- ✅ No goroutine leaks detected

### Integration Tests:
- ✅ Cache hit rate ≥ 90% at 1000 TPS
- ✅ Zero data loss during shutdown with 100 in-flight requests
- ✅ Graceful shutdown completes within 30 seconds
- ✅ Compression achieves 40-60% bandwidth reduction

### Performance:
- ✅ Cache Get() < 1ms (cache hit)
- ✅ Shutdown completes all in-flight work
- ✅ Compression adds < 5ms latency

---

## Test Implementation Order

**Week 1** (Critical Path):
1. **Day 1**: Merchant credential cache tests
2. **Day 2**: Payment method cache tests
3. **Day 3**: Shutdown manager tests
4. **Day 4**: Shutdown in-flight tests
5. **Day 5**: Compression middleware tests

**Week 2** (Supporting Tests):
6. **Day 1**: HTTP client config tests
7. **Day 2**: Goroutine tracker tests
8. **Day 3-4**: Integration tests (caches)
9. **Day 5**: Integration tests (shutdown)

**Week 3** (Validation):
10. **Day 1-2**: Load testing at 1000 TPS
11. **Day 3-4**: Performance validation
12. **Day 5**: Documentation and reporting

---

## Next Steps

1. **Review this test plan** ✅ (You selected this)
2. **Create test templates** (scaffolding for each test file)
3. **Implement Phase 1 tests** (critical unit tests)
4. **Implement Phase 2 tests** (supporting unit tests)
5. **Implement Phase 3 tests** (integration tests)

**Ready to proceed with implementation?**
