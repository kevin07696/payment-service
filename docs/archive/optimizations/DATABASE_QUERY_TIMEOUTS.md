# Database Query Timeouts - Implementation Guide

**Status**: Infrastructure Ready
**Date**: 2025-11-20
**Priority**: P0 - CRITICAL

---

## Overview

Database query timeouts prevent slow queries from blocking the application and consuming connection pool resources. This document describes the timeout infrastructure and how to use it.

## Configuration

### Timeout Tiers

The system implements a three-tier timeout strategy based on query complexity:

| Tier | Timeout | Use Cases |
|------|---------|-----------|
| **Simple** | 2s | ID lookups, single-row operations, existence checks |
| **Complex** | 5s | JOINs, WHERE clauses, aggregations, GROUP BY |
| **Report** | 30s | Analytics, large scans, complex aggregations |

### Configuration Structure

```go
type PostgreSQLConfig struct {
    // ... connection settings ...

    // Query timeout settings
    SimpleQueryTimeout  time.Duration // Default: 2s
    ComplexQueryTimeout time.Duration // Default: 5s
    ReportQueryTimeout  time.Duration // Default: 30s
}
```

## Usage

### 1. Access Database Adapter

Services need access to the `*database.PostgreSQLAdapter` to use timeout helpers:

```go
type myService struct {
    dbAdapter *database.PostgreSQLAdapter  // Store adapter reference
    queries   sqlc.Querier                 // For backward compatibility
    logger    *zap.Logger
}
```

### 2. Simple Queries (2s timeout)

Use `SimpleQueryContext()` for lookups and single-row operations:

```go
// Get by ID - simple query
ctx, cancel := s.dbAdapter.SimpleQueryContext(ctx)
defer cancel()

transaction, err := s.queries.GetTransactionByID(ctx, txID)
if err != nil {
    // Handle timeout or other errors
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        s.logger.Error("Query timeout",
            zap.String("query", "GetTransactionByID"),
            zap.Duration("timeout", 2*time.Second))
    }
    return nil, err
}
```

**Examples of simple queries**:
- `GetTransactionByID`
- `GetMerchantByID`
- `GetPaymentMethodByID`
- `GetSubscriptionByID`
- Existence checks (e.g., `CustomerExists`)

### 3. Complex Queries (5s timeout)

Use `ComplexQueryContext()` for queries with JOINs, filters, or aggregations:

```go
// Get transaction tree - complex query with recursive CTE
ctx, cancel := s.dbAdapter.ComplexQueryContext(ctx)
defer cancel()

tree, err := s.queries.GetTransactionTree(ctx, parentTxID)
if err != nil {
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        s.logger.Error("Complex query timeout",
            zap.String("query", "GetTransactionTree"),
            zap.String("parent_tx_id", parentTxID))
    }
    return nil, err
}
```

**Examples of complex queries**:
- `GetTransactionTree` (recursive CTE)
- `ListPaymentMethodsByCustomer` (JOIN + filter + sort)
- `GetPendingACHVerifications` (filter + date range)
- `GetChargebacksByMerchant` (JOIN + aggregation)

### 4. Report Queries (30s timeout)

Use `ReportQueryContext()` for analytics and reports:

```go
// Generate transaction report - analytics query
ctx, cancel := s.dbAdapter.ReportQueryContext(ctx)
defer cancel()

report, err := s.queries.GetTransactionReport(ctx, sqlc.GetTransactionReportParams{
    MerchantID: merchantID,
    StartDate:  startDate,
    EndDate:    endDate,
})
if err != nil {
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        s.logger.Warn("Report query timeout - consider date range reduction",
            zap.String("merchant_id", merchantID))
    }
    return nil, err
}
```

**Examples of report queries**:
- Daily/monthly transaction summaries
- Revenue analytics
- Chargeback reports
- Settlement reports

## Implementation Pattern

### Before (No Timeout)

```go
func (s *paymentService) GetTransaction(ctx context.Context, txID string) (*Transaction, error) {
    // Query has NO timeout - could block indefinitely
    tx, err := s.queries.GetTransactionByID(ctx, txID)
    if err != nil {
        return nil, err
    }
    return convertTransaction(tx), nil
}
```

**Problem**: If database is slow or query is inefficient, request blocks forever, consuming connection pool resources.

### After (With Timeout)

```go
func (s *paymentService) GetTransaction(ctx context.Context, txID string) (*Transaction, error) {
    // Create timeout context for simple query
    queryCtx, cancel := s.dbAdapter.SimpleQueryContext(ctx)
    defer cancel()

    // Query will timeout after 2 seconds
    tx, err := s.queries.GetTransactionByID(queryCtx, txID)
    if err != nil {
        if errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
            s.logger.Error("GetTransactionByID timeout",
                zap.String("tx_id", txID),
                zap.Duration("timeout", 2*time.Second))
            return nil, fmt.Errorf("query timeout: %w", err)
        }
        return nil, err
    }
    return convertTransaction(tx), nil
}
```

**Benefits**:
- Query fails fast after 2 seconds
- Connection returned to pool quickly
- Prevents connection pool exhaustion
- Clear timeout error messages in logs

## Context Propagation

**IMPORTANT**: Always use the timeout context for the query, not the original parent context:

```go
// ✅ CORRECT - query uses timeout context
queryCtx, cancel := s.dbAdapter.SimpleQueryContext(ctx)
defer cancel()
result, err := s.queries.GetSomething(queryCtx, id)

// ❌ WRONG - query uses parent context (no timeout)
queryCtx, cancel := s.dbAdapter.SimpleQueryContext(ctx)
defer cancel()
result, err := s.queries.GetSomething(ctx, id)  // BUG: using parent ctx
```

## Error Handling

### Detecting Timeouts

```go
if err != nil {
    // Check if error was due to timeout
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        s.logger.Error("Query timeout occurred",
            zap.String("query", "GetTransactionByID"),
            zap.String("id", txID),
            zap.Duration("timeout", s.dbAdapter.Config().SimpleQueryTimeout))

        // Return user-friendly error
        return nil, fmt.Errorf("database query timeout - please try again")
    }

    // Handle other database errors
    return nil, fmt.Errorf("database error: %w", err)
}
```

### Logging Best Practices

```go
if errors.Is(ctx.Err(), context.DeadlineExceeded) {
    s.logger.Error("Query timeout",
        zap.String("query", "GetTransactionTree"),        // Query name
        zap.String("parent_tx_id", parentTxID),          // Query parameters
        zap.Duration("timeout", 5*time.Second),           // Timeout value
        zap.String("recommendation", "Consider query optimization or index creation"))
}
```

## Migration Strategy

### Phase 1: Infrastructure (COMPLETED ✅)

- [x] Add timeout configuration to `PostgreSQLConfig`
- [x] Create timeout helper methods in `PostgreSQLAdapter`
- [x] Set default timeouts (2s / 5s / 30s)
- [x] Build and test infrastructure

### Phase 2: Service Updates (IN PROGRESS)

Update services to use timeout contexts:

1. **Payment Service** - High priority
   - `GetTransactionByID` - Simple (2s)
   - `GetTransactionTree` - Complex (5s)
   - `ProcessPayment` - Simple (2s) for inserts
   - `CreateTransaction` - Simple (2s)

2. **Payment Method Service** - High priority
   - `GetPaymentMethodByID` - Simple (2s)
   - `ListPaymentMethodsByCustomer` - Complex (5s)
   - `GetPendingACHVerifications` - Complex (5s)

3. **Subscription Service** - Medium priority
   - `GetSubscriptionByID` - Simple (2s)
   - `ListActiveSubscriptions` - Complex (5s)

4. **Merchant Service** - Low priority
   - `GetMerchantByID` - Simple (2s)
   - `GetMerchantBySlug` - Simple (2s)

### Phase 3: Monitoring (TODO)

Add metrics to track:
- Query timeout frequency
- Queries approaching timeout threshold
- Slow query patterns

## Testing

### Unit Test Example

```go
func TestQueryTimeout(t *testing.T) {
    cfg := database.DefaultPostgreSQLConfig(testDatabaseURL)
    cfg.SimpleQueryTimeout = 100 * time.Millisecond  // Short timeout for test

    adapter, err := database.NewPostgreSQLAdapter(context.Background(), cfg, logger)
    require.NoError(t, err)
    defer adapter.Close()

    ctx := context.Background()
    queryCtx, cancel := adapter.SimpleQueryContext(ctx)
    defer cancel()

    // Simulate slow query with pg_sleep
    _, err = adapter.Pool().Exec(queryCtx, "SELECT pg_sleep(1)")

    // Should timeout after 100ms
    assert.Error(t, err)
    assert.True(t, errors.Is(queryCtx.Err(), context.DeadlineExceeded))
}
```

## FAQ

### Q: What if a query legitimately takes longer than the timeout?

**A**: Adjust the timeout tier or use a custom timeout:

```go
// Use longer timeout for specific query
ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()
result, err := s.queries.SlowButLegitimateQuery(ctx, params)
```

### Q: Should I add timeouts to transaction blocks?

**A**: Yes, but use the longest timeout needed by any query in the transaction:

```go
// Transaction with multiple queries
txCtx, cancel := s.dbAdapter.ComplexQueryContext(ctx)
defer cancel()

err := s.txManager.WithTx(txCtx, func(q sqlc.Querier) error {
    // All queries in transaction use same timeout context
    _, err := q.CreateTransaction(txCtx, params1)
    if err != nil {
        return err
    }

    _, err = q.UpdatePaymentMethod(txCtx, params2)
    if err != nil {
        return err
    }

    return nil
})
```

### Q: What about background cron jobs?

**A**: Cron jobs should still use timeouts to prevent runaway queries:

```go
// ACH verification cron - runs every 5 minutes
func (s *cronService) ProcessACHVerifications(ctx context.Context) error {
    // Use complex timeout for filtered list query
    queryCtx, cancel := s.dbAdapter.ComplexQueryContext(ctx)
    defer cancel()

    pending, err := s.queries.GetPendingACHVerifications(queryCtx, limit)
    // ... process results ...
}
```

## Benefits

### ✅ Prevents Connection Pool Exhaustion

Without timeouts, slow queries hold connections indefinitely:

```
Pool: [BUSY] [BUSY] [BUSY] [BUSY] [BUSY]
New request: ❌ No connections available! Service DOWN
```

With timeouts, connections are released after timeout:

```
Pool: [BUSY] [BUSY] [FREE] [FREE] [FREE]
Slow queries timeout → connections returned → service stays UP
```

### ✅ Fail Fast

- Query times out after 2-30 seconds (depending on tier)
- Clear error message to user
- No indefinite waiting

### ✅ Improved Observability

Timeout errors in logs help identify:
- Slow queries needing optimization
- Missing indexes
- Inefficient query patterns
- Need for caching

### ✅ Predictable Performance

Users get consistent response times instead of unpredictable delays.

---

## Status Summary

| Component | Status | Notes |
|-----------|--------|-------|
| Infrastructure | ✅ COMPLETE | Config, helpers, defaults |
| Documentation | ✅ COMPLETE | This document |
| Payment Service | 🔄 PENDING | High priority |
| Payment Method Service | 🔄 PENDING | High priority |
| Subscription Service | 🔄 PENDING | Medium priority |
| Merchant Service | 🔄 PENDING | Low priority |
| Monitoring | 📋 TODO | Metrics needed |

**Next Steps**: Update payment service to use timeout contexts for all database queries.
