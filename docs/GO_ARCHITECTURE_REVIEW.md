# Go Architecture Review: Payment Service

**Review Date:** 2025-11-22
**Reviewer Focus:** Go idioms, performance, interface design, memory optimization, testing patterns

## Executive Summary

This payment service demonstrates strong architectural foundations with excellent use of Go patterns. The codebase shows a mature understanding of hexagonal architecture, dependency injection, and idiomatic Go. However, there are several opportunities for performance optimization, interface refinement, and enhanced testing strategies.

**Overall Grade: B+ (Very Good)**

**Strengths:**
- Clean hexagonal architecture with well-defined ports/adapters
- Excellent use of sqlc for type-safe database operations
- Strong idempotency and state management patterns
- Good error handling with custom domain errors
- Proper use of context propagation

**Areas for Improvement:**
- Memory allocations in hot paths
- Interface granularity could be improved
- Missing performance benchmarks for critical paths
- Connection pool monitoring could be more sophisticated
- Some repeated code patterns could be abstracted

---

## 1. Interface Design & Dependency Injection

### Current State: Good

**Strengths:**
```go
// Excellent port/adapter separation
type PaymentService interface {
    Authorize(ctx context.Context, req *AuthorizeRequest) (*domain.Transaction, error)
    Capture(ctx context.Context, req *CaptureRequest) (*domain.Transaction, error)
    // ... clean, focused methods
}

// Proper dependency injection via constructor
func NewPaymentService(
    queries sqlc.Querier,
    txManager database.TransactionManager,
    serverPost adapterports.ServerPostAdapter,
    // ...
) ports.PaymentService
```

**Issues Identified:**

1. **Request Structs in Interface Definitions**
   - Location: `/internal/services/ports/payment_service.go`
   - Issue: Request/response structs live in the same package as interfaces
   - Impact: Creates circular dependency risk; violates single responsibility

2. **Interface Segregation Violations**
   - Location: `sqlc.Querier` interface
   - Issue: Massive interface with 70+ methods
   - Impact: Difficult to mock, violates Interface Segregation Principle

3. **Service Dependencies Not Minimal**
   ```go
   type paymentService struct {
       queries                   sqlc.Querier // 70+ methods
       txManager                 database.TransactionManager
       serverPost                adapterports.ServerPostAdapter
       secretManager             adapterports.SecretManagerAdapter
       merchantResolver          *authorization.MerchantResolver
       merchantCredentialResolver *authorization.MerchantCredentialResolver
       merchantAuthService       *authorization.MerchantAuthorizationService
       logger                    *zap.Logger
   }
   ```
   - 8 dependencies is manageable but could indicate responsibility sprawl

### Recommendations

#### 1.1 Split Request/Response Models
```go
// BEFORE (anti-pattern)
// internal/services/ports/payment_service.go
type AuthorizeRequest struct { ... }
type PaymentService interface { ... }

// AFTER (idiomatic)
// internal/services/payment/models.go or internal/domain/requests.go
package domain

type AuthorizeRequest struct { ... }

// internal/services/ports/payment_service.go
package ports

import "github.com/.../internal/domain"

type PaymentService interface {
    Authorize(ctx context.Context, req *domain.AuthorizeRequest) (*domain.Transaction, error)
}
```

**Rationale:**
- Separates data models from behavior contracts
- Allows DTOs to be versioned independently
- Reduces coupling between package boundaries

#### 1.2 Create Domain-Specific Query Interfaces
```go
// internal/services/ports/payment_repository.go
package ports

// Small, focused interfaces for payment service needs
type PaymentRepository interface {
    GetTransactionByID(ctx context.Context, id uuid.UUID) (sqlc.Transaction, error)
    CreateTransaction(ctx context.Context, params sqlc.CreateTransactionParams) (sqlc.Transaction, error)
    GetTransactionTree(ctx context.Context, parentID uuid.UUID) ([]sqlc.GetTransactionTreeRow, error)
    MarkPaymentMethodUsed(ctx context.Context, id uuid.UUID) error
}

type MerchantRepository interface {
    GetMerchantByID(ctx context.Context, id uuid.UUID) (sqlc.Merchant, error)
    GetMerchantBySlug(ctx context.Context, slug string) (sqlc.Merchant, error)
}

// internal/adapters/database/repository.go
package database

// Adapter implements small interfaces by delegating to sqlc.Queries
type paymentRepository struct {
    queries *sqlc.Queries
}

func (r *paymentRepository) GetTransactionByID(ctx context.Context, id uuid.UUID) (sqlc.Transaction, error) {
    return r.queries.GetTransactionByID(ctx, id)
}
```

**Benefits:**
- Easier to mock (3-4 methods vs 70+)
- Clear documentation of what each service actually needs
- Enables easier testing and refactoring
- Follows Interface Segregation Principle

**Implementation Priority:** Medium (2-3 week effort)

#### 1.3 Consider Functional Options Pattern for Services
```go
// Current constructor is already long
func NewPaymentService(
    queries sqlc.Querier,
    txManager database.TransactionManager,
    serverPost adapterports.ServerPostAdapter,
    secretManager adapterports.SecretManagerAdapter,
    merchantResolver *authorization.MerchantResolver,
    logger *zap.Logger,
) ports.PaymentService

// Refactor to functional options for flexibility
type PaymentServiceOption func(*paymentService)

func WithCustomAuth(authService *authorization.MerchantAuthorizationService) PaymentServiceOption {
    return func(s *paymentService) {
        s.merchantAuthService = authService
    }
}

func NewPaymentService(
    repo ports.PaymentRepository,
    epx adapterports.ServerPostAdapter,
    opts ...PaymentServiceOption,
) ports.PaymentService {
    s := &paymentService{
        repo: repo,
        epx:  epx,
        merchantAuthService: authorization.NewMerchantAuthorizationService(zap.NewNop()),
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}
```

**Benefits:**
- Makes optional dependencies explicit
- Easier to add new dependencies without breaking existing code
- Cleaner test setup

**Implementation Priority:** Low (nice-to-have)

---

## 2. Error Handling Patterns

### Current State: Very Good

**Strengths:**
```go
// Excellent domain error definitions
var (
    ErrMerchantInactive           = errors.New("merchant is inactive")
    ErrPaymentMethodNotVerified   = errors.New("payment method not verified")
    ErrTransactionCannotBeCaptured = errors.New("transaction cannot be captured")
)

// Good error wrapping
if err != nil {
    return nil, fmt.Errorf("failed to get merchant: %w", err)
}

// Proper error mapping in handlers
func (h *ConnectHandler) handleServiceErrorConnect(err error) error {
    switch {
    case errors.Is(err, domain.ErrMerchantInactive):
        return connect.NewError(connect.CodeFailedPrecondition, errors.New("agent is inactive"))
    // ...
    }
}
```

**Issues Identified:**

1. **Error Context Loss**
   ```go
   // payment_service.go:311
   _, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
   if err != nil {
       return nil, fmt.Errorf("failed to get MAC secret: %w", err)
   }
   ```
   - Missing contextual information (merchant ID, which operation)
   - Stack traces not captured

2. **Inconsistent Error Messages**
   - Some errors return technical details: "invalid idempotency_key format"
   - Others sanitized: "internal server error"
   - No consistent error code system

3. **Silent Error Handling**
   ```go
   // payment_service.go:233
   if err := q.MarkPaymentMethodUsed(ctx, *paymentMethodUUID); err != nil {
       s.logger.Warn("Failed to mark payment method as used", zap.Error(err))
       // Error is logged but swallowed - is this intentional?
   }
   ```

### Recommendations

#### 2.1 Create Structured Error Types
```go
// internal/domain/errors.go
package domain

import "fmt"

// DomainError provides structured error information
type DomainError struct {
    Code      string                 // Machine-readable error code
    Message   string                 // Human-readable message
    Operation string                 // Which operation failed
    Details   map[string]interface{} // Additional context
    Err       error                  // Wrapped error
}

func (e *DomainError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %s: %v", e.Operation, e.Message, e.Err)
    }
    return fmt.Sprintf("%s: %s", e.Operation, e.Message)
}

func (e *DomainError) Unwrap() error {
    return e.Err
}

// Error constructors
func ErrInvalidAmount(amount int64) *DomainError {
    return &DomainError{
        Code:    "INVALID_AMOUNT",
        Message: "amount must be positive",
        Details: map[string]interface{}{"amount": amount},
    }
}

func ErrMerchantNotFound(merchantID string) *DomainError {
    return &DomainError{
        Code:    "MERCHANT_NOT_FOUND",
        Message: "merchant not found",
        Details: map[string]interface{}{"merchant_id": merchantID},
    }
}
```

**Usage:**
```go
// In service layer
if merchant, err := s.queries.GetMerchantByID(ctx, merchantID); err != nil {
    return nil, &domain.DomainError{
        Code:      "MERCHANT_FETCH_FAILED",
        Message:   "failed to retrieve merchant",
        Operation: "Sale",
        Details:   map[string]interface{}{"merchant_id": merchantID},
        Err:       err,
    }
}
```

**Benefits:**
- Consistent error structure across services
- Rich error context for debugging
- Easier to convert to API error responses
- Can be logged with structured fields

**Implementation Priority:** High (1 week effort)

#### 2.2 Add Error Budgets and Circuit Breaking Context
```go
// Track error rates per operation
type ErrorBudget struct {
    operation     string
    errorCount    atomic.Int64
    requestCount  atomic.Int64
    windowStart   atomic.Int64
}

func (b *ErrorBudget) RecordError(err error) {
    b.errorCount.Add(1)
    b.requestCount.Add(1)
}

func (b *ErrorBudget) ErrorRate() float64 {
    errors := b.errorCount.Load()
    requests := b.requestCount.Load()
    if requests == 0 {
        return 0
    }
    return float64(errors) / float64(requests)
}
```

**Implementation Priority:** Medium

---

## 3. Memory Allocations & Performance

### Current State: Good with Hot Spots

**Issues Identified:**

1. **String Concatenation in Hot Paths**
   ```go
   // server_post_adapter.go:483
   batchID := now.Format("20060102") // Allocates new string every request
   localDate := now.Format("010206")
   localTime := now.Format("150405")
   ```
   - Each `Format()` allocates a new string
   - Called on every transaction (hot path)

2. **Repeated UUID Parsing**
   ```go
   // payment_service.go:92-106
   merchantID, err := uuid.Parse(resolvedMerchantID)
   if err == nil {
       merchant, err = s.queries.GetMerchantByID(ctx, merchantID)
   } else {
       merchant, err = s.queries.GetMerchantBySlug(ctx, resolvedMerchantID)
       merchantID = merchant.ID // Parse again implicitly
   }
   ```
   - UUID validation happens multiple times per transaction

3. **Map Allocations in Metadata Handling**
   ```go
   // payment_service.go:185-199
   metadata := req.Metadata
   if metadata == nil {
       metadata = make(map[string]interface{}) // Allocates on every nil case
   }
   metadata["auth_resp_text"] = epxResp.AuthRespText
   metadata["auth_avs"] = epxResp.AuthAVS
   metadata["auth_cvv2"] = epxResp.AuthCVV2

   metadataJSON, err := json.Marshal(metadata) // Allocates buffer
   ```

4. **Slice Growth in Conversions**
   ```go
   // payment_service.go:1333-1336
   transactions := make([]*domain.Transaction, len(dbTxs))
   for i, dbTx := range dbTxs {
       transactions[i] = sqlcToDomain(&dbTx)
   }
   ```
   - Pre-allocation is good, but `sqlcToDomain` has hidden allocations

5. **URL Values Construction**
   ```go
   // server_post_adapter.go:465
   func (a *serverPostAdapter) buildFormData(req *ports.ServerPostRequest) url.Values {
       data := url.Values{} // Allocates empty map
       data.Set("CUST_NBR", req.CustNbr) // Multiple allocations for map growth
       // ... 20+ Set() calls
   }
   ```

### Recommendations

#### 3.1 Pre-allocate and Reuse Buffers
```go
// Add sync.Pool for common allocations
var (
    metadataPool = sync.Pool{
        New: func() interface{} {
            return make(map[string]interface{}, 8) // Pre-sized for typical use
        },
    }

    urlValuesPool = sync.Pool{
        New: func() interface{} {
            return make(url.Values, 25) // Pre-sized for EPX request
        },
    }
)

// In buildFormData
func (a *serverPostAdapter) buildFormData(req *ports.ServerPostRequest) url.Values {
    data := urlValuesPool.Get().(url.Values)
    // Clear existing values
    for k := range data {
        delete(data, k)
    }

    data.Set("CUST_NBR", req.CustNbr)
    // ... populate

    return data
}

// Caller must return to pool
func (a *serverPostAdapter) ProcessTransaction(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
    formData := a.buildFormData(req)
    defer func() {
        urlValuesPool.Put(formData)
    }()

    // ... use formData
}
```

**Expected Impact:** 15-20% reduction in allocations for transaction processing

#### 3.2 Optimize String Formatting
```go
// BEFORE
batchID := now.Format("20060102")
localDate := now.Format("010206")
localTime := now.Format("150405")

// AFTER - reuse buffer
var dateBuf [8]byte  // Stack allocated
var timeBuf [6]byte  // Stack allocated

func formatDate(t time.Time, buf *[8]byte) string {
    year, month, day := t.Date()
    buf[0] = byte('0' + year/1000)
    buf[1] = byte('0' + (year/100)%10)
    buf[2] = byte('0' + (year/10)%10)
    buf[3] = byte('0' + year%10)
    buf[4] = byte('0' + month/10)
    buf[5] = byte('0' + month%10)
    buf[6] = byte('0' + day/10)
    buf[7] = byte('0' + day%10)
    return string(buf[:])
}
```

**Alternative:** Pre-compute and cache date strings
```go
type dateCache struct {
    mu        sync.RWMutex
    date      time.Time
    batchID   string
    localDate string
}

var epxDateCache dateCache

func (a *serverPostAdapter) getBatchID() string {
    now := time.Now()

    epxDateCache.mu.RLock()
    if epxDateCache.date.Day() == now.Day() {
        batchID := epxDateCache.batchID
        epxDateCache.mu.RUnlock()
        return batchID
    }
    epxDateCache.mu.RUnlock()

    epxDateCache.mu.Lock()
    defer epxDateCache.mu.Unlock()

    // Double-check after acquiring write lock
    if epxDateCache.date.Day() == now.Day() {
        return epxDateCache.batchID
    }

    epxDateCache.date = now
    epxDateCache.batchID = now.Format("20060102")
    epxDateCache.localDate = now.Format("010206")
    return epxDateCache.batchID
}
```

**Expected Impact:** 5-10% reduction in CPU time for EPX requests

#### 3.3 Optimize sqlcToDomain Conversions
```go
// BEFORE - creates intermediate pointers
func sqlcToDomain(dbTx *sqlc.Transaction) *domain.Transaction {
    var parentTxID *string
    if dbTx.ParentTransactionID.Valid {
        id := uuid.UUID(dbTx.ParentTransactionID.Bytes).String()
        parentTxID = &id  // Heap allocation
    }
    // ... 5 more similar allocations
}

// AFTER - use string inline, only allocate if needed
func sqlcToDomain(dbTx *sqlc.Transaction) *domain.Transaction {
    tx := &domain.Transaction{
        ID:          dbTx.ID.String(),
        MerchantID:  dbTx.MerchantID.String(),
        AmountCents: dbTx.AmountCents,
        Currency:    dbTx.Currency,
        Type:        domain.TransactionType(dbTx.Type),
        // ... non-pointer fields first
    }

    // Batch pointer allocations
    if dbTx.ParentTransactionID.Valid {
        id := uuid.UUID(dbTx.ParentTransactionID.Bytes).String()
        tx.ParentTransactionID = &id
    }

    if dbTx.CustomerID.Valid {
        tx.CustomerID = &dbTx.CustomerID.String
    }

    // ... continue

    return tx
}
```

**Benchmark Target:**
```go
func BenchmarkSqlcToDomain(b *testing.B) {
    dbTx := createTestTransaction()

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        _ = sqlcToDomain(&dbTx)
    }
}
// Target: < 15 allocs/op (currently likely 20+)
```

**Implementation Priority:** High (significant throughput impact)

#### 3.4 Add Benchmarks for Hot Paths
```go
// internal/services/payment/payment_service_bench_test.go
package payment

import (
    "context"
    "testing"
)

func BenchmarkSale(b *testing.B) {
    // Setup service with mocks
    service := setupBenchmarkService(b)
    req := &ports.SaleRequest{
        MerchantID:  "merchant-123",
        AmountCents: 10000,
        Currency:    "USD",
        PaymentToken: stringPtr("test-token"),
        IdempotencyKey: stringPtr(uuid.New().String()),
    }

    ctx := context.Background()

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        _, err := service.Sale(ctx, req)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkGroupStateComputation(b *testing.B) {
    txs := createTestTransactionTree(10) // 10 transactions

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        _ = ComputeGroupState(txs)
    }
}
// Target: < 1000 ns/op, < 5 allocs/op
```

**Implementation Priority:** High (needed for baseline metrics)

---

## 4. Database Patterns & Connection Pooling

### Current State: Good with Room for Optimization

**Strengths:**
```go
// Excellent transaction management
func (a *PostgreSQLAdapter) WithTx(ctx context.Context, fn func(sqlc.Querier) error) error {
    tx, err := a.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }

    qtx := a.queries.WithTx(tx)

    defer func() {
        if p := recover(); p != nil {
            tx.Rollback(ctx)
            panic(p)
        }
    }()

    if err := fn(qtx); err != nil {
        tx.Rollback(ctx)
        return err
    }

    return tx.Commit(ctx)
}
```

**Issues Identified:**

1. **N+1 Query Pattern in Capture/Void/Refund**
   ```go
   // payment_service.go:529
   groupTxs, err := q.GetTransactionTree(ctx, originalTxID)
   // ... validation

   // Then re-fetch OUTSIDE transaction (line 595)
   groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, originalTxID)
   ```
   - Fetches same data twice
   - Second fetch might see different data (race condition)

2. **Missing Query Timeouts**
   ```go
   // Queries use parent context without specific timeouts
   dbTx, err := s.queries.GetTransactionByID(ctx, txID)
   ```
   - Should use different timeouts for simple vs complex queries
   - Adapter has timeout helpers but they're not used

3. **Connection Pool Monitoring is Passive**
   ```go
   // StartPoolMonitoring logs warnings at 80% utilization
   if utilization > 80 {
       a.logger.Warn("Database connection pool highly utilized", ...)
   }
   ```
   - No alerting or auto-scaling
   - No metrics exposure (Prometheus)

4. **Transaction Scope Too Large**
   ```go
   // payment_service.go:526-588
   err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
       // Get transaction tree
       // Compute state
       // Validate capture
       // Get merchant
       // Validate merchant
       // Get secret
       return nil // Transaction held during EPX call? NO - good!
   })

   // EPX call happens outside transaction - GOOD
   epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
   ```
   - Actually good! Transaction doesn't span EPX call
   - But validation could be done before transaction

### Recommendations

#### 4.1 Eliminate Redundant GetTransactionTree Call
```go
// BEFORE
err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
    groupTxs, err := q.GetTransactionTree(ctx, originalTxID)
    // ... compute state, validate
    return nil
})

// Re-fetch outside transaction
groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, originalTxID)
state := ComputeGroupState(domainTxsRefetch)

// AFTER - fetch once, validate before transaction
groupTxs, err := s.queries.GetTransactionTree(ctx, originalTxID)
if err != nil {
    return nil, fmt.Errorf("failed to get transaction tree: %w", err)
}

domainTxs := convertTodomainTransactions(groupTxs)
state := ComputeGroupState(domainTxs)

// Validate BEFORE opening transaction
canCapture, reason := state.CanCapture(captureAmountCents)
if !canCapture {
    return nil, domain.ErrTransactionCannotBeCaptured
}

// Only use transaction for writes
err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
    // Create pending transaction
    // Update records
    return nil
})
```

**Benefits:**
- Reduces database load (1 query instead of 2)
- Shortens transaction duration
- Eliminates race condition window

**Implementation Priority:** High (low effort, high impact)

#### 4.2 Use Tiered Query Timeouts
```go
// In payment service methods
func (s *paymentService) GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error) {
    // Simple query - use short timeout
    queryCtx, cancel := s.txManager.SimpleQueryContext(ctx)
    defer cancel()

    dbTx, err := s.queries.GetTransactionByID(queryCtx, txID)
    // ...
}

func (s *paymentService) ListTransactions(ctx context.Context, filters *ports.ListTransactionsFilters) ([]*domain.Transaction, int, error) {
    // Complex query with filters - use longer timeout
    queryCtx, cancel := s.txManager.ComplexQueryContext(ctx)
    defer cancel()

    dbTxs, err := s.queries.ListTransactions(queryCtx, params)
    // ...
}
```

**Configuration:**
```go
// database/postgres.go
type PostgreSQLConfig struct {
    SimpleQueryTimeout  time.Duration // 2 seconds (ID lookups)
    ComplexQueryTimeout time.Duration // 5 seconds (JOINs, filters)
    ReportQueryTimeout  time.Duration // 30 seconds (analytics)
}
```

**Implementation Priority:** Medium (defensive programming)

#### 4.3 Add Prepared Statement Caching
```go
// SQLC already generates efficient queries, but can optimize further
// In high-throughput scenarios, use pgx prepared statements

type preparedStatementCache struct {
    mu         sync.RWMutex
    statements map[string]*pgconn.StatementDescription
}

// This is handled by pgx automatically, but can be monitored
func (a *PostgreSQLAdapter) monitorPreparedStatements() {
    rows, err := a.pool.Query(context.Background(),
        "SELECT name, statement FROM pg_prepared_statements")
    // Log and alert on cache misses
}
```

**Implementation Priority:** Low (pgx handles this well already)

#### 4.4 Add Connection Pool Metrics
```go
// internal/adapters/database/metrics.go
package database

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    dbPoolSize = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "payment_service_db_pool_size",
            Help: "Current size of database connection pool",
        },
        []string{"state"}, // acquired, idle, max
    )

    dbQueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "payment_service_db_query_duration_seconds",
            Help:    "Database query duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"query", "status"},
    )
)

// In StartPoolMonitoring
func (a *PostgreSQLAdapter) StartPoolMonitoring(ctx context.Context, interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                stat := a.pool.Stat()
                dbPoolSize.WithLabelValues("acquired").Set(float64(stat.AcquiredConns()))
                dbPoolSize.WithLabelValues("idle").Set(float64(stat.IdleConns()))
                dbPoolSize.WithLabelValues("max").Set(float64(stat.MaxConns()))
            }
        }
    }()
}
```

**Implementation Priority:** Medium (production observability)

---

## 5. HTTP Handler Patterns

### Current State: Very Good

**Strengths:**
```go
// Clean Connect RPC handlers
func (h *ConnectHandler) Sale(
    ctx context.Context,
    req *connect.Request[paymentv1.SaleRequest],
) (*connect.Response[paymentv1.PaymentResponse], error) {
    // Validate
    if err := validateSaleRequest(req.Msg); err != nil {
        return nil, connect.NewError(connect.CodeInvalidArgument, err)
    }

    // Convert to service request
    serviceReq := &ports.SaleRequest{ ... }

    // Call service
    tx, err := h.service.Sale(ctx, serviceReq)
    if err != nil {
        return nil, h.handleServiceErrorConnect(err)
    }

    // Convert to proto response
    return connect.NewResponse(transactionToPaymentResponse(tx)), nil
}
```

**Issues Identified:**

1. **Request Validation Happens After Parsing**
   ```go
   // Already parsed heavy proto message before validation
   msg := req.Msg
   if err := validateSaleRequest(msg); err != nil {
       return nil, connect.NewError(connect.CodeInvalidArgument, err)
   }
   ```
   - Should validate required fields early
   - Could use protobuf validation rules

2. **Error Mapping is Repetitive**
   ```go
   // 60+ lines of error mapping in handleServiceErrorConnect
   func (h *ConnectHandler) handleServiceErrorConnect(err error) error {
       switch {
       case errors.Is(err, domain.ErrMerchantInactive):
           return connect.NewError(connect.CodeFailedPrecondition, ...)
       case errors.Is(err, domain.ErrMerchantRequired):
           return connect.NewError(connect.CodeInvalidArgument, ...)
       // ... 20 more cases
       }
   }
   ```

3. **Missing Request/Response Logging**
   ```go
   // Logs request received, but not response details
   h.logger.Info("Sale request received",
       zap.String("merchant_id", msg.MerchantId),
       zap.Int64("amount_cents", msg.AmountCents),
   )
   // Missing: response status, duration, error details
   ```

### Recommendations

#### 5.1 Use Protobuf Validation
```go
// Install buf validate plugin
// buf.gen.yaml
version: v1
plugins:
  - plugin: buf.build/bufbuild/validate-go
    out: proto
    opt: paths=source_relative

// proto/payment/v1/payment.proto
message SaleRequest {
    string merchant_id = 1 [(validate.rules).string = {
        min_len: 1,
        max_len: 255,
        pattern: "^[a-zA-Z0-9-_]+$",
    }];

    int64 amount_cents = 2 [(validate.rules).int64 = {
        gt: 0,
        lt: 100000000, // $1M limit
    }];

    string currency = 3 [(validate.rules).string = {
        len: 3,
        in: ["USD", "EUR", "GBP"],
    }];

    oneof payment_method = 4 [(validate.rules).oneof.required = true];
}

// Generated validation
func (h *ConnectHandler) Sale(ctx context.Context, req *connect.Request[paymentv1.SaleRequest]) (*connect.Response[paymentv1.PaymentResponse], error) {
    // Validate happens automatically with buf-validate
    if err := req.Msg.Validate(); err != nil {
        return nil, connect.NewError(connect.CodeInvalidArgument, err)
    }
    // ...
}
```

**Benefits:**
- Validation rules live with schema
- Generated code is optimized
- Consistent across all handlers

**Implementation Priority:** Medium (1-2 days)

#### 5.2 Create Declarative Error Mapping
```go
// internal/handlers/errors.go
package handlers

var domainToConnectErrors = map[error]connect.Code{
    domain.ErrMerchantInactive:           connect.CodeFailedPrecondition,
    domain.ErrMerchantRequired:           connect.CodeInvalidArgument,
    domain.ErrAuthMerchantMismatch:       connect.CodePermissionDenied,
    domain.ErrPaymentMethodNotFound:      connect.CodeNotFound,
    domain.ErrPaymentMethodNotVerified:   connect.CodeFailedPrecondition,
    domain.ErrTransactionCannotBeCaptured: connect.CodeFailedPrecondition,
    // ... all mappings
}

func MapDomainError(err error) error {
    for domainErr, code := range domainToConnectErrors {
        if errors.Is(err, domainErr) {
            return connect.NewError(code, errors.New(domainErr.Error()))
        }
    }

    // Handle DomainError type
    var domainErr *domain.DomainError
    if errors.As(err, &domainErr) {
        return connect.NewError(
            errorCodeToConnect(domainErr.Code),
            errors.New(domainErr.Message),
        )
    }

    return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
}
```

**Implementation Priority:** Low (code clarity improvement)

#### 5.3 Add Request/Response Interceptor
```go
// internal/handlers/interceptors/logging.go
package interceptors

import (
    "connectrpc.com/connect"
    "go.uber.org/zap"
    "time"
)

func NewLoggingInterceptor(logger *zap.Logger) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            start := time.Now()

            logger.Info("Request started",
                zap.String("procedure", req.Spec().Procedure),
                zap.String("protocol", req.Peer().Protocol),
            )

            resp, err := next(ctx, req)

            duration := time.Since(start)

            if err != nil {
                logger.Error("Request failed",
                    zap.String("procedure", req.Spec().Procedure),
                    zap.Duration("duration", duration),
                    zap.Error(err),
                )
            } else {
                logger.Info("Request completed",
                    zap.String("procedure", req.Spec().Procedure),
                    zap.Duration("duration", duration),
                )
            }

            return resp, err
        }
    }
}

// Usage in server setup
interceptors := connect.WithInterceptors(
    interceptors.NewLoggingInterceptor(logger),
    interceptors.NewMetricsInterceptor(),
    interceptors.NewAuthInterceptor(authService),
)
```

**Implementation Priority:** High (production observability)

---

## 6. EPX Adapter Patterns

### Current State: Good with Resilience Patterns

**Strengths:**
```go
// Excellent circuit breaker implementation
err := a.circuitBreaker.Call(func() error {
    // Retry logic with exponential backoff
    for attempt := 0; attempt <= a.config.MaxRetries; attempt++ {
        if attempt > 0 {
            delay := a.backoff.NextDelay(attempt - 1)
            // ... backoff
        }
        // ... send request
    }
})

// Good connection pooling
transport := &http.Transport{
    TLSClientConfig: &tls.Config{
        InsecureSkipVerify: config.InsecureSkipVerify,
    },
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 100,
    IdleConnTimeout:     90 * time.Second,
}
```

**Issues Identified:**

1. **XML Parsing Without Limits**
   ```go
   // server_post_adapter.go:671
   var epxResp EPXResponse
   if err := xml.Unmarshal(body, &epxResp); err != nil {
       return nil, fmt.Errorf("failed to unmarshal XML: %w", err)
   }
   ```
   - No size limits on response body
   - Vulnerable to XML bomb attacks

2. **Buffer Allocation in Socket Read**
   ```go
   // server_post_adapter.go:276
   buffer := make([]byte, 4096) // Fixed size, heap allocated every call
   n, err := conn.Read(buffer)
   ```
   - Should use sync.Pool
   - 4096 might be too small for some responses

3. **Form Data String Building**
   ```go
   // Creates url.Values then calls Encode()
   formData := a.buildFormData(req)
   httpReq, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL,
       strings.NewReader(formData.Encode()))
   ```
   - `Encode()` allocates new string
   - `strings.NewReader()` allocates again

4. **Time.Now() Called Multiple Times**
   ```go
   // server_post_adapter.go:481-489
   now := time.Now()
   batchID := now.Format("20060102")
   // ...
   localDate := now.Format("010206")
   localTime := now.Format("150405")
   ```

### Recommendations

#### 6.1 Add Response Size Limits
```go
// server_post_adapter.go
const maxResponseSize = 10 * 1024 * 1024 // 10MB

func (a *serverPostAdapter) ProcessTransaction(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
    // ...

    httpResp, err := a.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer httpResp.Body.Close()

    // Limit response size
    limitedBody := io.LimitReader(httpResp.Body, maxResponseSize)
    body, err := io.ReadAll(limitedBody)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    if len(body) >= maxResponseSize {
        return nil, fmt.Errorf("response exceeds maximum size of %d bytes", maxResponseSize)
    }

    // Parse response
    parsedResp, err := a.parseResponse(body, req)
    // ...
}
```

**Implementation Priority:** High (security)

#### 6.2 Use Buffer Pool for Socket Operations
```go
var socketBufferPool = sync.Pool{
    New: func() interface{} {
        buf := make([]byte, 8192) // Larger default
        return &buf
    },
}

func (a *serverPostAdapter) ProcessTransactionViaSocket(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
    // ...

    bufPtr := socketBufferPool.Get().(*[]byte)
    buffer := *bufPtr
    defer socketBufferPool.Put(bufPtr)

    n, err := conn.Read(buffer)
    // ...
}
```

**Implementation Priority:** Medium (performance)

#### 6.3 Optimize Form Encoding
```go
// Use bytes.Buffer directly instead of url.Values
func (a *serverPostAdapter) buildFormData(req *ports.ServerPostRequest) *bytes.Buffer {
    // Pre-allocate buffer
    buf := bytes.NewBuffer(make([]byte, 0, 2048))

    // Write key-value pairs directly
    buf.WriteString("CUST_NBR=")
    buf.WriteString(url.QueryEscape(req.CustNbr))
    buf.WriteString("&MERCH_NBR=")
    buf.WriteString(url.QueryEscape(req.MerchNbr))
    // ...

    return buf
}

// Usage
func (a *serverPostAdapter) ProcessTransaction(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
    formData := a.buildFormData(req)

    httpReq, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL, formData)
    // ...
}
```

**Expected Impact:** 20-30% reduction in allocations for EPX requests

**Implementation Priority:** High

#### 6.4 Add EPX Response Caching for Idempotency
```go
// Cache EPX responses for idempotency
type epxResponseCache struct {
    mu    sync.RWMutex
    cache map[string]*ports.ServerPostResponse
    ttl   time.Duration
}

func (c *epxResponseCache) Get(tranNbr string) (*ports.ServerPostResponse, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    resp, ok := c.cache[tranNbr]
    return resp, ok
}

func (c *epxResponseCache) Set(tranNbr string, resp *ports.ServerPostResponse) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.cache[tranNbr] = resp
}

// In serverPostAdapter
func (a *serverPostAdapter) ProcessTransaction(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
    // Check cache first (EPX is idempotent by TRAN_NBR)
    if cached, ok := a.responseCache.Get(req.TranNbr); ok {
        a.logger.Info("Returning cached EPX response", zap.String("tran_nbr", req.TranNbr))
        return cached, nil
    }

    // ... make request

    // Cache successful responses
    if response.IsApproved {
        a.responseCache.Set(req.TranNbr, response)
    }

    return response, nil
}
```

**Benefits:**
- Protects against double-charging on retries
- Reduces EPX load
- Faster response times for retries

**Implementation Priority:** High (correctness)

---

## 7. Testing Patterns & Coverage

### Current State: Good Foundation, Missing Coverage

**Strengths:**
```go
// Excellent table-driven tests
func TestComputeGroupState(t *testing.T) {
    tests := []struct {
        name     string
        transactions []*domain.Transaction
        want     *GroupState
    }{
        {
            name: "single auth transaction",
            transactions: []*domain.Transaction{
                {Type: domain.TransactionTypeAuth, ...},
            },
            want: &GroupState{
                ActiveAuthAmount: 10000,
                ...
            },
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ComputeGroupState(tt.transactions)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

**Issues Identified:**

1. **Missing Benchmarks**
   - No benchmarks found in service layer
   - Critical path performance not measured
   - Memory allocation profiles unknown

2. **Integration Tests Not Isolated**
   ```go
   // tests/integration/payment/payment_service_critical_test.go
   // Uses shared database, could have test interference
   ```

3. **Mock Explosion Problem**
   ```go
   // payment_service_test.go:94
   // NOTE: Complete mock implementation of sqlc.Querier would require ~70 methods.
   ```
   - Acknowledged but not solved
   - Makes unit testing services very difficult

4. **No Property-Based Tests**
   - Transaction state machine would benefit from property testing
   - Invariant: total captured <= total authorized
   - Invariant: total refunded <= total captured

5. **Missing Fuzz Tests**
   - Amount calculations vulnerable to overflow
   - UUID parsing vulnerable to crashes
   - No fuzz coverage

### Recommendations

#### 7.1 Add Comprehensive Benchmarks
```go
// internal/services/payment/payment_service_bench_test.go
package payment

import (
    "context"
    "testing"
    "github.com/google/uuid"
)

func BenchmarkSale_CreditCard(b *testing.B) {
    service := setupBenchmarkService(b)
    ctx := context.Background()

    b.Run("with_saved_payment_method", func(b *testing.B) {
        pmID := uuid.New().String()

        b.ResetTimer()
        b.ReportAllocs()

        for i := 0; i < b.N; i++ {
            req := &ports.SaleRequest{
                MerchantID:      "merchant-123",
                AmountCents:     10000,
                Currency:        "USD",
                PaymentMethodID: &pmID,
                IdempotencyKey:  stringPtr(uuid.New().String()),
            }

            _, err := service.Sale(ctx, req)
            if err != nil {
                b.Fatal(err)
            }
        }
    })

    b.Run("with_one_time_token", func(b *testing.B) {
        b.ResetTimer()
        b.ReportAllocs()

        for i := 0; i < b.N; i++ {
            token := "tok_" + uuid.New().String()
            req := &ports.SaleRequest{
                MerchantID:     "merchant-123",
                AmountCents:    10000,
                Currency:       "USD",
                PaymentToken:   &token,
                IdempotencyKey: stringPtr(uuid.New().String()),
            }

            _, err := service.Sale(ctx, req)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}

func BenchmarkGroupStateComputation(b *testing.B) {
    scenarios := map[string]int{
        "simple_auth":            1,  // Single AUTH
        "auth_capture":           2,  // AUTH + CAPTURE
        "auth_partial_captures":  5,  // AUTH + 4 partial CAPTUREs
        "complex_tree":          10,  // Mixed operations
    }

    for name, txCount := range scenarios {
        b.Run(name, func(b *testing.B) {
            txs := generateTransactionTree(txCount)

            b.ResetTimer()
            b.ReportAllocs()

            for i := 0; i < b.N; i++ {
                _ = ComputeGroupState(txs)
            }
        })
    }
}

func BenchmarkSqlcToDomain(b *testing.B) {
    dbTx := createTestSqlcTransaction()

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        _ = sqlcToDomain(&dbTx)
    }
}

// Benchmark targets:
// - Sale: < 5ms/op, < 100 allocs/op (with mocked EPX)
// - GroupStateComputation (10 tx): < 10µs/op, < 10 allocs/op
// - SqlcToDomain: < 1µs/op, < 15 allocs/op
```

**Implementation Priority:** High (baseline for optimization)

#### 7.2 Add Property-Based Tests
```go
// Install: go get github.com/leanovate/gopter

// internal/services/payment/payment_properties_test.go
package payment

import (
    "testing"
    "github.com/leanovate/gopter"
    "github.com/leanovate/gopter/gen"
    "github.com/leanovate/gopter/prop"
)

func TestGroupState_Properties(t *testing.T) {
    parameters := gopter.DefaultTestParameters()
    parameters.MinSuccessfulTests = 100

    properties := gopter.NewProperties(parameters)

    // Property: Captured amount never exceeds authorized amount
    properties.Property("captured <= authorized", prop.ForAll(
        func(txs []*domain.Transaction) bool {
            state := ComputeGroupState(txs)
            return state.CapturedAmount <= state.ActiveAuthAmount
        },
        genTransactionTree(1, 10), // Generate 1-10 transactions
    ))

    // Property: Refunded amount never exceeds captured amount
    properties.Property("refunded <= captured", prop.ForAll(
        func(txs []*domain.Transaction) bool {
            state := ComputeGroupState(txs)
            return state.RefundedAmount <= state.CapturedAmount
        },
        genTransactionTree(1, 10),
    ))

    // Property: After void, no active auth
    properties.Property("void clears active auth", prop.ForAll(
        func(authTx *domain.Transaction, voidTx *domain.Transaction) bool {
            authTx.Type = domain.TransactionTypeAuth
            authTx.Status = domain.TransactionStatusApproved

            voidTx.Type = domain.TransactionTypeVoid
            voidTx.Status = domain.TransactionStatusApproved
            voidTx.Metadata = map[string]interface{}{
                "original_transaction_type": "auth",
            }

            state := ComputeGroupState([]*domain.Transaction{authTx, voidTx})
            return state.ActiveAuthID == nil && state.IsAuthVoided
        },
        genTransaction(),
        genTransaction(),
    ))

    properties.TestingRun(t)
}

func genTransactionTree(minSize, maxSize int) gopter.Gen {
    return gen.SliceOfN(
        gen.IntRange(minSize, maxSize),
        genTransaction(),
    )
}

func genTransaction() gopter.Gen {
    return gopter.CombineGens(
        gen.Identifier(),
        gen.Int64Range(100, 1000000),
        gen.OneConstOf(
            domain.TransactionTypeAuth,
            domain.TransactionTypeCapture,
            domain.TransactionTypeSale,
            domain.TransactionTypeRefund,
            domain.TransactionTypeVoid,
        ),
        gen.OneConstOf(
            domain.TransactionStatusApproved,
            domain.TransactionStatusDeclined,
        ),
    ).Map(func(values interface{}) *domain.Transaction {
        v := values.([]interface{})
        return &domain.Transaction{
            ID:          v[0].(string),
            AmountCents: v[1].(int64),
            Type:        v[2].(domain.TransactionType),
            Status:      v[3].(domain.TransactionStatus),
        }
    })
}
```

**Implementation Priority:** Medium (catch edge cases)

#### 7.3 Add Fuzz Tests
```go
// internal/domain/transaction_fuzz_test.go
package domain_test

import (
    "testing"
    "github.com/kevin07696/payment-service/internal/services/payment"
)

func FuzzComputeGroupState(f *testing.F) {
    // Seed corpus
    f.Add(int64(1000), int64(500), int64(250))

    f.Fuzz(func(t *testing.T, authAmount, captureAmount, refundAmount int64) {
        // Skip invalid inputs
        if authAmount <= 0 || captureAmount < 0 || refundAmount < 0 {
            return
        }

        txs := []*domain.Transaction{
            {
                Type:        domain.TransactionTypeAuth,
                Status:      domain.TransactionStatusApproved,
                AmountCents: authAmount,
                AuthGUID:    "auth-guid",
            },
        }

        if captureAmount > 0 {
            txs = append(txs, &domain.Transaction{
                Type:        domain.TransactionTypeCapture,
                Status:      domain.TransactionStatusApproved,
                AmountCents: captureAmount,
                AuthGUID:    "capture-guid",
            })
        }

        if refundAmount > 0 {
            txs = append(txs, &domain.Transaction{
                Type:        domain.TransactionTypeRefund,
                Status:      domain.TransactionStatusApproved,
                AmountCents: refundAmount,
                AuthGUID:    "refund-guid",
            })
        }

        // Should not panic
        state := payment.ComputeGroupState(txs)

        // Invariants
        if state.CapturedAmount > state.ActiveAuthAmount {
            t.Errorf("captured (%d) > authorized (%d)", state.CapturedAmount, state.ActiveAuthAmount)
        }

        if state.RefundedAmount > state.CapturedAmount {
            t.Errorf("refunded (%d) > captured (%d)", state.RefundedAmount, state.CapturedAmount)
        }
    })
}
```

**Run:**
```bash
go test -fuzz=FuzzComputeGroupState -fuzztime=30s
```

**Implementation Priority:** High (critical path validation)

#### 7.4 Create Repository Interface Mocks
```go
// Instead of mocking sqlc.Querier (70+ methods), create focused repository interfaces

// internal/services/ports/repositories.go
package ports

type TransactionRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error)
    GetTree(ctx context.Context, rootID uuid.UUID) ([]*domain.Transaction, error)
    Create(ctx context.Context, tx *domain.Transaction) error
    UpdateWithEPXResponse(ctx context.Context, id uuid.UUID, resp *EPXResponse) error
}

type MerchantRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Merchant, error)
    GetBySlug(ctx context.Context, slug string) (*domain.Merchant, error)
}

// internal/testutil/mocks/repositories.go
package mocks

type MockTransactionRepository struct {
    mock.Mock
}

func (m *MockTransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*domain.Transaction), args.Error(1)
}

// ... 3 more methods (much easier than 70!)

// Usage in tests
func TestPaymentService_Sale(t *testing.T) {
    mockTxRepo := new(mocks.MockTransactionRepository)
    mockMerchantRepo := new(mocks.MockMerchantRepository)
    mockEPX := new(mocks.MockServerPostAdapter)

    mockMerchantRepo.On("GetByID", mock.Anything, mock.Anything).
        Return(&domain.Merchant{IsActive: true}, nil)

    mockEPX.On("ProcessTransaction", mock.Anything, mock.Anything).
        Return(&ports.ServerPostResponse{
            AuthGUID:   "bric-123",
            AuthResp:   "00",
            IsApproved: true,
        }, nil)

    service := NewPaymentService(mockTxRepo, mockMerchantRepo, mockEPX, logger)

    // ... test
}
```

**Implementation Priority:** High (enables proper unit testing)

---

## 8. Struct Layout & Memory Efficiency

### Current Analysis

**Current Struct Layouts:**
```go
// domain.Transaction - 296 bytes (with padding)
type Transaction struct {
    ID                  string                 // 16 bytes (pointer to string header)
    ParentTransactionID *string                // 8 bytes (pointer)
    MerchantID          string                 // 16 bytes
    CustomerID          *string                // 8 bytes
    SubscriptionID      *string                // 8 bytes
    AmountCents         int64                  // 8 bytes
    Currency            string                 // 16 bytes
    Status              TransactionStatus      // 16 bytes (string)
    Type                TransactionType        // 16 bytes (string)
    PaymentMethodType   PaymentMethodType      // 16 bytes (string)
    PaymentMethodID     *string                // 8 bytes
    AuthGUID            string                 // 16 bytes
    AuthResp            *string                // 8 bytes
    AuthCode            *string                // 8 bytes
    AuthRespText        *string                // 8 bytes
    AuthCardType        *string                // 8 bytes
    AuthAVS             *string                // 8 bytes
    AuthCVV2            *string                // 8 bytes
    IdempotencyKey      *string                // 8 bytes
    Metadata            map[string]interface{} // 8 bytes (pointer)
    CreatedAt           time.Time              // 24 bytes
    UpdatedAt           time.Time              // 24 bytes
}
```

**Issues:**
1. String-based enums (Status, Type, PaymentMethodType) waste memory
2. Many optional string pointers could use nullable value types
3. Field ordering not optimized for alignment

### Recommendations

#### 8.1 Use Integer Enums Instead of Strings
```go
// BEFORE
type TransactionStatus string
const (
    TransactionStatusApproved TransactionStatus = "approved"
    TransactionStatusDeclined TransactionStatus = "declined"
)

// AFTER
type TransactionStatus uint8
const (
    TransactionStatusApproved TransactionStatus = 1
    TransactionStatusDeclined TransactionStatus = 2
)

func (s TransactionStatus) String() string {
    switch s {
    case TransactionStatusApproved:
        return "approved"
    case TransactionStatusDeclined:
        return "declined"
    default:
        return "unknown"
    }
}

func (s TransactionStatus) MarshalJSON() ([]byte, error) {
    return []byte(`"` + s.String() + `"`), nil
}

func (s *TransactionStatus) UnmarshalJSON(data []byte) error {
    str := string(data[1 : len(data)-1]) // Remove quotes
    switch str {
    case "approved":
        *s = TransactionStatusApproved
    case "declined":
        *s = TransactionStatusDeclined
    default:
        return fmt.Errorf("invalid status: %s", str)
    }
    return nil
}
```

**Savings:** 16 bytes → 1 byte per enum field (15 bytes × 3 enums = 45 bytes saved)

**Implementation Priority:** Low (breaks existing JSON contracts)

#### 8.2 Optimize Struct Field Ordering
```go
// BEFORE (suboptimal alignment)
type Transaction struct {
    ID          string  // 16 bytes
    AmountCents int64   // 8 bytes
    Status      string  // 16 bytes
    // ... padding waste
}

// AFTER (optimal alignment)
type Transaction struct {
    // 8-byte aligned fields first
    AmountCents int64   // 8 bytes
    CreatedAt   time.Time  // 24 bytes (3×8)
    UpdatedAt   time.Time  // 24 bytes

    // Pointers (8 bytes)
    ParentTransactionID *string
    CustomerID          *string
    SubscriptionID      *string
    PaymentMethodID     *string
    AuthResp            *string
    AuthCode            *string
    AuthRespText        *string
    AuthCardType        *string
    AuthAVS             *string
    AuthCVV2            *string
    IdempotencyKey      *string
    Metadata            map[string]interface{}

    // Strings (16 bytes each)
    ID                string
    MerchantID        string
    Currency          string
    AuthGUID          string

    // 1-byte enums (if implemented)
    Status            TransactionStatus
    Type              TransactionType
    PaymentMethodType PaymentMethodType
    // 5 bytes padding
}
```

**Tools to verify:**
```bash
go install honnef.co/go/tools/cmd/structlayout@latest
go install honnef.co/go/tools/cmd/structlayout-optimize@latest

structlayout -json github.com/kevin07696/payment-service/internal/domain Transaction | structlayout-optimize
```

**Implementation Priority:** Low (marginal gains)

#### 8.3 Use sql.NullString Instead of *string for Optional Fields
```go
// BEFORE
type Transaction struct {
    CustomerID *string // 8 bytes pointer + 16 bytes string = 24 bytes when set
}

// AFTER
type Transaction struct {
    CustomerID sql.NullString // 17 bytes (16 string + 1 bool)
}
```

**Benefits:**
- Slightly more cache-friendly (data locality)
- Avoids double indirection
- Standard library type

**Drawbacks:**
- Changes JSON serialization behavior
- Requires custom MarshalJSON

**Implementation Priority:** Low (requires API changes)

---

## 9. Concurrency Patterns

### Current State: Good Basics, Room for Advanced Patterns

**Strengths:**
```go
// Proper context propagation
func (s *paymentService) Sale(ctx context.Context, req *ports.SaleRequest) (*domain.Transaction, error) {
    // Context passed to all downstream calls
    merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
    epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
}

// Good use of sync.Pool (in resilience package)
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}
```

**Issues Identified:**

1. **No Worker Pool for Background Tasks**
   - Cron jobs run sequentially
   - ACH verification could process batches concurrently

2. **No Rate Limiting**
   - EPX adapter has retry/backoff but no rate limiting
   - Could overwhelm EPX API under load

3. **Database Connection Pool Not Tuned for Concurrency**
   ```go
   // postgres.go:35
   MaxConns: 25, // Is this enough for production?
   MinConns: 5,
   ```

### Recommendations

#### 9.1 Add Worker Pool for Batch Processing
```go
// internal/util/workerpool.go
package util

type WorkerPool struct {
    workers   int
    taskQueue chan func()
    wg        sync.WaitGroup
}

func NewWorkerPool(workers int, queueSize int) *WorkerPool {
    pool := &WorkerPool{
        workers:   workers,
        taskQueue: make(chan func(), queueSize),
    }

    for i := 0; i < workers; i++ {
        pool.wg.Add(1)
        go pool.worker()
    }

    return pool
}

func (p *WorkerPool) worker() {
    defer p.wg.Done()
    for task := range p.taskQueue {
        task()
    }
}

func (p *WorkerPool) Submit(task func()) {
    p.taskQueue <- task
}

func (p *WorkerPool) Shutdown() {
    close(p.taskQueue)
    p.wg.Wait()
}

// Usage in ACH verification cron
func (h *ACHVerificationHandler) ProcessBatch(ctx context.Context, paymentMethods []*domain.PaymentMethod) error {
    pool := util.NewWorkerPool(10, 100) // 10 workers
    defer pool.Shutdown()

    results := make(chan error, len(paymentMethods))

    for _, pm := range paymentMethods {
        pm := pm // Capture for closure
        pool.Submit(func() {
            err := h.service.VerifyACHAccount(ctx, &ports.VerifyACHAccountRequest{
                PaymentMethodID: pm.ID,
                MerchantID:      pm.MerchantID,
                CustomerID:      pm.CustomerID,
            })
            results <- err
        })
    }

    // Collect results
    var errors []error
    for i := 0; i < len(paymentMethods); i++ {
        if err := <-results; err != nil {
            errors = append(errors, err)
        }
    }

    if len(errors) > 0 {
        return fmt.Errorf("verification failed for %d payment methods", len(errors))
    }

    return nil
}
```

**Implementation Priority:** Medium (scalability)

#### 9.2 Add Rate Limiting to EPX Adapter
```go
// Install: go get golang.org/x/time/rate

import "golang.org/x/time/rate"

type serverPostAdapter struct {
    // ... existing fields
    rateLimiter *rate.Limiter
}

func NewServerPostAdapter(config *ServerPostConfig, logger *zap.Logger) ports.ServerPostAdapter {
    // EPX allows 100 requests/second (hypothetical limit)
    limiter := rate.NewLimiter(rate.Limit(100), 100) // 100 req/s, burst of 100

    return &serverPostAdapter{
        // ...
        rateLimiter: limiter,
    }
}

func (a *serverPostAdapter) ProcessTransaction(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
    // Wait for rate limiter
    if err := a.rateLimiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limit wait failed: %w", err)
    }

    // ... existing implementation
}
```

**Implementation Priority:** High (protect EPX API)

#### 9.3 Tune Connection Pool for Production
```go
// Guidance for connection pool sizing:
// - Max connections = (CPU cores * 2) + effective_spindle_count
// - For cloud databases, start with 25-50 and monitor
// - Monitor pool exhaustion and adjust

type PostgreSQLConfig struct {
    // Add dynamic sizing based on environment
    MaxConns        int32  // Default: 25 (dev), 100 (staging), 200 (prod)
    MinConns        int32  // Default: 5 (dev), 25 (staging), 50 (prod)
    // ...
}

func DefaultPostgreSQLConfig(databaseURL string, environment string) *PostgreSQLConfig {
    var maxConns, minConns int32

    switch environment {
    case "production":
        maxConns = 200
        minConns = 50
    case "staging":
        maxConns = 100
        minConns = 25
    default: // development
        maxConns = 25
        minConns = 5
    }

    return &PostgreSQLConfig{
        DatabaseURL: databaseURL,
        MaxConns:    maxConns,
        MinConns:    minConns,
        // ...
    }
}
```

**Implementation Priority:** Medium (production tuning)

---

## 10. Code Organization & Patterns

### Current State: Excellent Hexagonal Architecture

**Strengths:**
- Clear separation of concerns (domain, ports, adapters)
- Dependency inversion principle followed
- No business logic in handlers

**Minor Improvements:**

#### 10.1 Extract Shared Converters
```go
// Multiple places have sqlcToDomain conversion
// payment_service.go:1378
// payment_method_service.go:516

// Consolidate to:
// internal/converters/transaction.go
package converters

func SqlcTransactionToDomain(dbTx *sqlc.Transaction) *domain.Transaction {
    // ... single source of truth
}

// internal/converters/payment_method.go
func SqlcPaymentMethodToDomain(dbPM *sqlc.CustomerPaymentMethod) *domain.PaymentMethod {
    // ...
}
```

**Implementation Priority:** Low (code duplication cleanup)

#### 10.2 Add Service Middleware Pattern
```go
// internal/services/middleware/logging.go
package middleware

type ServiceMiddleware func(service ports.PaymentService) ports.PaymentService

type loggingMiddleware struct {
    next   ports.PaymentService
    logger *zap.Logger
}

func LoggingMiddleware(logger *zap.Logger) ServiceMiddleware {
    return func(next ports.PaymentService) ports.PaymentService {
        return &loggingMiddleware{next: next, logger: logger}
    }
}

func (m *loggingMiddleware) Sale(ctx context.Context, req *ports.SaleRequest) (*domain.Transaction, error) {
    start := time.Now()

    m.logger.Info("Sale started",
        zap.String("merchant_id", req.MerchantID),
        zap.Int64("amount", req.AmountCents),
    )

    tx, err := m.next.Sale(ctx, req)

    duration := time.Since(start)

    if err != nil {
        m.logger.Error("Sale failed",
            zap.Duration("duration", duration),
            zap.Error(err),
        )
    } else {
        m.logger.Info("Sale completed",
            zap.Duration("duration", duration),
            zap.String("transaction_id", tx.ID),
        )
    }

    return tx, err
}

// Apply middleware
func NewPaymentService(...) ports.PaymentService {
    base := &paymentService{...}

    // Wrap with middleware
    withLogging := middleware.LoggingMiddleware(logger)(base)
    withMetrics := middleware.MetricsMiddleware()(withLogging)
    withTracing := middleware.TracingMiddleware()(withMetrics)

    return withTracing
}
```

**Implementation Priority:** Low (nice-to-have)

---

## Summary of Recommendations

### High Priority (Immediate Action)

1. **Add Comprehensive Benchmarks** - Establish performance baseline
2. **Create Repository Interfaces** - Enable proper unit testing
3. **Optimize EPX Form Building** - 20-30% allocation reduction
4. **Add Structured Errors** - Improve debugging and monitoring
5. **Eliminate Redundant GetTransactionTree** - Reduce DB load
6. **Add EPX Response Size Limits** - Security fix
7. **Add EPX Rate Limiting** - Protect external API
8. **Add Request/Response Logging Interceptor** - Production observability
9. **Add Fuzz Tests** - Critical path validation

**Estimated Effort:** 2-3 weeks
**Expected Impact:** 20-30% performance improvement, better testability, production-ready observability

### Medium Priority (Next Quarter)

1. **Split Request/Response Models** - Reduce coupling
2. **Add Error Budgets** - Track reliability
3. **Use Tiered Query Timeouts** - Defensive programming
4. **Add Connection Pool Metrics** - Production observability
5. **Use Protobuf Validation** - Cleaner validation code
6. **Add Worker Pool** - Batch processing scalability
7. **Add Property-Based Tests** - Catch edge cases

**Estimated Effort:** 3-4 weeks
**Expected Impact:** Improved architecture, better reliability tracking

### Low Priority (Future Enhancements)

1. **Functional Options Pattern** - Constructor flexibility
2. **Integer Enums** - Memory savings (breaks API)
3. **Optimize Struct Layout** - Marginal memory gains
4. **Extract Shared Converters** - Code cleanup
5. **Service Middleware Pattern** - Cross-cutting concerns

**Estimated Effort:** 1-2 weeks
**Expected Impact:** Code quality improvements, marginal performance gains

---

## Performance Targets

Based on this review, here are recommended performance targets:

**Latency (p95):**
- Sale (with EPX call): < 500ms
- Authorize (with EPX call): < 500ms
- Capture (with EPX call): < 400ms
- GetTransaction (DB only): < 50ms
- ListTransactions (DB only): < 100ms

**Throughput:**
- Concurrent transactions: 100+ req/s (limited by EPX)
- Read operations: 1000+ req/s

**Memory:**
- Heap allocations per Sale: < 100 allocs/op
- Heap allocations per GroupStateComputation: < 10 allocs/op
- Connection pool utilization: < 70% under normal load

**Reliability:**
- Error rate: < 0.1% (excluding EPX declines)
- Database connection pool exhaustion: 0
- Circuit breaker trips: < 1/hour under normal load

---

## Conclusion

This payment service demonstrates strong Go engineering practices with a clean hexagonal architecture, excellent use of sqlc for type-safety, and solid error handling patterns. The main opportunities for improvement lie in:

1. **Performance optimization** - Memory allocations in hot paths can be reduced significantly
2. **Testing infrastructure** - Missing benchmarks and property-based tests
3. **Production observability** - Need better metrics, logging, and monitoring
4. **Interface granularity** - Large interfaces make testing difficult

The recommendations in this document focus on high-impact, low-effort improvements that will make the service more performant, testable, and production-ready while maintaining the excellent architectural foundation already in place.

**Next Steps:**
1. Implement high-priority recommendations
2. Run benchmarks to establish baseline
3. Set up continuous performance monitoring
4. Create tracking issues for medium/low priority items

**Files Referenced:**
- `/home/kevinlam/Documents/projects/payments/internal/services/payment/payment_service.go`
- `/home/kevinlam/Documents/projects/payments/internal/services/payment_method/payment_method_service.go`
- `/home/kevinlam/Documents/projects/payments/internal/adapters/epx/server_post_adapter.go`
- `/home/kevinlam/Documents/projects/payments/internal/adapters/database/postgres.go`
- `/home/kevinlam/Documents/projects/payments/internal/handlers/payment/payment_handler_connect.go`
- `/home/kevinlam/Documents/projects/payments/internal/domain/transaction.go`
- `/home/kevinlam/Documents/projects/payments/internal/domain/payment_method.go`
- `/home/kevinlam/Documents/projects/payments/internal/services/payment/group_state.go`
