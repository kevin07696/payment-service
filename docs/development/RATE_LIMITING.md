# Distributed Rate Limiting with PostgreSQL UNLOGGED Tables

**Last Updated:** 2025-11-24

## Overview

The payment service implements distributed rate limiting using PostgreSQL **UNLOGGED tables** as a high-speed L2 cache. This provides:

- **Distributed coordination** - All service instances share the same rate limit state
- **Near-memory performance** - UNLOGGED tables skip WAL (Write-Ahead Logging) for 2-3x faster writes
- **No additional dependencies** - Uses existing PostgreSQL infrastructure (no Redis/Memcached)
- **Automatic cleanup** - Cron job removes old entries every 5 minutes

---

## Architecture

### L2 Cache Strategy

```
┌─────────────────────────────────────────────────┐
│  Service Instance 1  │  Service Instance 2  │ ... │
│  ┌──────────────┐   │  ┌──────────────┐    │
│  │ Auth         │   │  │ Auth         │    │
│  │ Middleware   │   │  │ Middleware   │    │
│  └──────┬───────┘   │  └──────┬───────┘    │
│         │           │         │             │
│         └───────────┼─────────┴─────────────┤
│                     ▼                        │
│         ┌──────────────────────────┐         │
│         │  PostgreSQL UNLOGGED     │         │
│         │  rate_limit_cache        │         │
│         │  (2-3x faster writes)    │         │
│         └──────────────────────────┘         │
└─────────────────────────────────────────────────┘
```

### Why UNLOGGED Tables?

**UNLOGGED** = No Write-Ahead Logging (WAL)

- **Performance:** 2-3x faster writes than regular tables
- **Trade-off:** Data lost on server crash (acceptable for rate limiting)
- **Use case:** Perfect for ephemeral data like rate limit buckets

---

## Database Schema

### Migration: `024_unlogged_rate_limits.sql`

```sql
CREATE UNLOGGED TABLE IF NOT EXISTS rate_limit_cache (
    bucket_key VARCHAR(255) PRIMARY KEY,
    tokens INTEGER NOT NULL,
    last_refill TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rate_limit_cache_last_refill
ON rate_limit_cache(last_refill);
```

### Bucket Key Format

```
service:{service_id}:{timestamp}
```

**Example:**
```
service:acme-corp-001:2024-11-24-14:30
```

- **service_id**: From JWT claims
- **timestamp**: Minute-level precision (buckets reset every minute)

---

## Rate Limiting Flow

### 1. Token Consumption (Atomic Operation)

```sql
INSERT INTO rate_limit_cache (bucket_key, tokens, last_refill)
VALUES ('service:acme-001:2024-11-24-14:30', 100, NOW())
ON CONFLICT (bucket_key) DO UPDATE
SET tokens = GREATEST(rate_limit_cache.tokens - 1, 0),
    last_refill = NOW()
RETURNING tokens;
```

**Atomicity guarantees:**
- `INSERT ... ON CONFLICT` is atomic
- `GREATEST(tokens - 1, 0)` prevents negative tokens
- Works correctly across multiple concurrent requests

### 2. Rate Limit Check Logic

**Location:** `internal/middleware/connect_auth.go:393-438`

```go
// 1. Get service rate limit from database
rateLimit, err := ai.queries.GetServiceRateLimit(ctx, entityID)
limit := 100 // Default limit
if err == nil && rateLimit.Valid {
    limit = int(rateLimit.Int32)
}

// 2. Build bucket key with minute-level precision
bucketKey := fmt.Sprintf("service:%s:%s",
    entityID,
    timeutil.Now().Format("2006-01-02-15:04"))

// 3. Try database rate limiting through circuit breaker
result, err := ai.rateLimitCB.Execute(func() (interface{}, error) {
    tokens, err := ai.queries.ConsumeRateLimitToken(ctx, sqlc.ConsumeRateLimitTokenParams{
        BucketKey:     bucketKey,
        InitialTokens: int32(limit),
    })
    return tokens, err
})

// 4. Check if circuit breaker succeeded
if err == nil {
    tokens := result.(int32)
    if tokens <= 0 {
        return fmt.Errorf("rate limit exceeded for %s %s", entityType, entityID)
    }
    return nil
}

// 5. Circuit is open or database failed - fall back to in-memory rate limiting
ai.logger.Warn("Using in-memory rate limiting fallback",
    zap.String("bucket_key", bucketKey),
    zap.Error(err))

return ai.checkMemoryRateLimit(bucketKey, int32(limit))
```

---

## Failover Strategy

### Circuit Breaker Pattern

The rate limiting system uses a **circuit breaker** to handle database failures gracefully:

#### States

1. **Closed** (Normal operation)
   - All requests go to PostgreSQL UNLOGGED table
   - Fast distributed rate limiting

2. **Open** (Database failure)
   - After 5 consecutive failures, circuit opens
   - Requests fall back to in-memory rate limiting
   - **WARNING:** In-memory limits are NOT distributed across instances

3. **Half-Open** (Recovery)
   - After 30 seconds, circuit attempts one test request
   - If successful, returns to Closed state
   - If failed, stays Open for another 30 seconds

### In-Memory Fallback

**Location:** `internal/middleware/connect_auth.go:440-467`

```go
func (ai *AuthInterceptor) checkMemoryRateLimit(bucketKey string, limit int32) error {
    now := timeutil.Now()

    // Get or create bucket
    value, loaded := ai.memoryBuckets.LoadOrStore(bucketKey, &tokenBucket{
        tokens:    limit,
        timestamp: now,
    })

    bucket := value.(*tokenBucket)
    bucket.mu.Lock()
    defer bucket.mu.Unlock()

    // Reset tokens every minute
    if !loaded || bucket.timestamp.Format("2006-01-02-15:04") != now.Format("2006-01-02-15:04") {
        bucket.tokens = limit
        bucket.timestamp = now
    }

    // Try to consume a token
    if bucket.tokens <= 0 {
        return fmt.Errorf("rate limit exceeded (in-memory)")
    }

    bucket.tokens--
    return nil
}
```

**Trade-off:**
- **Pro:** High availability during DB outages
- **Con:** Each instance has independent limits (100 req/min × 10 instances = 1000 req/min)

---

## Cleanup Cron Job

### Handler: `rate_limit_cleanup_handler.go`

**Endpoint:** `POST /cron/cleanup-rate-limits`

**Schedule:** Every 5 minutes

**Retention:** Keeps last 1 hour of data (for analytics), deletes older entries

```go
func (h *RateLimitCleanupHandler) CleanupRateLimitBuckets(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Deletes entries > 1 hour old
    err := h.queries.CleanupOldRateLimitBuckets(ctx)
    if err != nil {
        h.logger.Error("Failed to cleanup old rate limit buckets", zap.Error(err))
        h.respondError(w, http.StatusInternalServerError, "cleanup failed")
        return
    }

    h.logger.Info("Rate limit bucket cleanup completed successfully")
    h.respondSuccess(w, RateLimitCleanupResponse{
        Success:     true,
        ProcessedAt: timeutil.Now().Format(time.RFC3339),
        Message:     "Deleted rate limit buckets older than 1 hour",
    })
}
```

### Cloud Scheduler Configuration

**GCP Cloud Scheduler job configuration:**

```bash
gcloud scheduler jobs create http rate-limit-cleanup \
  --schedule="*/5 * * * *" \
  --uri="https://api.example.com/cron/cleanup-rate-limits" \
  --http-method=POST \
  --headers="X-Cron-Secret=YOUR_CRON_SECRET" \
  --attempt-deadline=30s \
  --location=us-central1
```

### Monitoring Endpoints

#### Health Check
```bash
GET /cron/rate-limit/health
```

Response:
```json
{
  "status": "healthy",
  "service": "rate-limit-cleanup-cron",
  "time": "2024-11-24T15:30:00Z"
}
```

#### Stats
```bash
GET /cron/rate-limit/stats
X-Cron-Secret: YOUR_SECRET
```

Response:
```json
{
  "table_type": "UNLOGGED (2-3x faster writes, no WAL)",
  "cleanup_interval": "5 minutes",
  "retention_period": "1 hour",
  "last_check": "2024-11-24T15:30:00Z",
  "recommended_schedule": "every 5 minutes",
  "distributed": true,
  "cache_level": "L2 (PostgreSQL UNLOGGED)"
}
```

---

## Performance Characteristics

### Benchmarks (Expected)

| Metric | Regular Table | UNLOGGED Table | Redis |
|--------|--------------|----------------|-------|
| Write Latency (p50) | 2-3ms | 0.5-1ms | 0.3-0.5ms |
| Write Latency (p99) | 10-15ms | 3-5ms | 1-2ms |
| Throughput | 3,000 TPS | 8,000 TPS | 15,000 TPS |
| Network Hops | 1 | 1 | 1 |
| Durability | Full | None (lost on crash) | Depends on config |

### Trade-offs Analysis

#### ✅ Advantages over Redis

1. **No additional infrastructure** - Uses existing PostgreSQL
2. **Simpler deployment** - One less service to manage
3. **Lower operational cost** - No separate Redis cluster
4. **ACID transactions** - Can join with other tables if needed

#### ⚠️ Disadvantages vs Redis

1. **Slower writes** - 2-3x slower than Redis (but still fast enough)
2. **Less flexible** - Redis has more data structures (sorted sets, bitmaps, etc.)
3. **Not horizontally scalable** - PostgreSQL is vertically scaled

#### Decision Rationale

For **payment service rate limiting**, UNLOGGED PostgreSQL is sufficient:
- Current load: ~100 req/second peak
- Expected capacity: 8,000 TPS (80x headroom)
- Simpler operations (no Redis deployment/monitoring)

---

## Configuration

### Environment Variables

```bash
# Rate limit defaults (per service per minute)
DEFAULT_RATE_LIMIT=100

# Circuit breaker settings
RATE_LIMIT_CIRCUIT_FAILURE_THRESHOLD=5
RATE_LIMIT_CIRCUIT_TIMEOUT=30s

# Cleanup cron settings
CRON_SECRET=your-secret-here  # Shared with all cron jobs
```

### Per-Service Rate Limits

Rate limits are stored in the `services` table:

```sql
UPDATE services
SET rate_limit_per_minute = 1000
WHERE id = 'acme-corp-001';
```

**Note:** Changes take effect on next authentication (5-minute cache TTL)

---

## Monitoring and Alerts

### Key Metrics

1. **Rate Limit Hits**
   - Metric: `rate_limit_exceeded_total`
   - Alert: > 100/minute indicates legitimate traffic spike or attack

2. **Circuit Breaker Opens**
   - Metric: `rate_limit_circuit_open_total`
   - Alert: > 1/hour indicates database issues

3. **Cleanup Job Failures**
   - Metric: `rate_limit_cleanup_failures_total`
   - Alert: > 3 consecutive failures

4. **UNLOGGED Table Size**
   - Query: `SELECT pg_total_relation_size('rate_limit_cache');`
   - Alert: > 100 MB indicates cleanup not running

### Example Prometheus Queries

```promql
# Rate limit hit rate (per second)
rate(rate_limit_exceeded_total[1m])

# Circuit breaker open percentage
rate(rate_limit_circuit_open_total[5m]) / rate(rate_limit_requests_total[5m]) * 100

# Average tokens remaining
avg(rate_limit_tokens_remaining)
```

---

## Troubleshooting

### Issue: "Rate limit exceeded" for legitimate traffic

**Possible causes:**
1. Service rate limit too low
2. Multiple instances consuming from same bucket
3. Attack or traffic spike

**Diagnosis:**
```sql
-- Check current service limit
SELECT id, rate_limit_per_minute
FROM services
WHERE id = 'acme-corp-001';

-- Check recent rate limit activity
SELECT bucket_key, tokens, last_refill
FROM rate_limit_cache
WHERE bucket_key LIKE 'service:acme-corp-001:%'
ORDER BY last_refill DESC
LIMIT 10;
```

**Resolution:**
```sql
-- Increase service limit
UPDATE services
SET rate_limit_per_minute = 500
WHERE id = 'acme-corp-001';
```

### Issue: Circuit breaker keeps opening

**Possible causes:**
1. Database connection pool exhausted
2. Slow queries causing timeouts
3. Network issues

**Diagnosis:**
```bash
# Check database connections
psql -c "SELECT count(*) FROM pg_stat_activity WHERE datname = 'payment_service';"

# Check for long-running queries
psql -c "SELECT pid, now() - query_start as duration, query FROM pg_stat_activity WHERE state = 'active' ORDER BY duration DESC;"
```

**Resolution:**
1. Increase connection pool size (`DB_MAX_CONNS`)
2. Add database indexes if queries are slow
3. Scale database vertically

### Issue: Cleanup job not running

**Check cron job status:**
```bash
# Manual trigger
curl -X POST https://api.example.com/cron/cleanup-rate-limits \
  -H "X-Cron-Secret: YOUR_SECRET"

# Check stats
curl https://api.example.com/cron/rate-limit/stats \
  -H "X-Cron-Secret: YOUR_SECRET"
```

**Check table size:**
```sql
SELECT
    pg_size_pretty(pg_total_relation_size('rate_limit_cache')) as total_size,
    count(*) as row_count,
    max(last_refill) as newest_entry,
    min(last_refill) as oldest_entry
FROM rate_limit_cache;
```

---

## Future Optimizations

### Potential Improvements

1. **Redis Migration** (if needed at scale)
   - When: > 10,000 TPS sustained
   - Benefit: 3-5x faster writes, horizontal scaling

2. **Partitioned UNLOGGED Tables**
   - Partition by hour: `rate_limit_cache_YYYYMMDDHH`
   - Drop old partitions instead of DELETE (instant)

3. **Adaptive Rate Limiting**
   - Increase limits during verified traffic spikes
   - Decrease limits during attack patterns

4. **Distributed Rate Limiting Algorithms**
   - Sliding window log
   - Leaky bucket with token bucket hybrid

---

## Related Documentation

- **[Authentication](AUTH.md)** - JWT token authentication
- **[Database Schema](DATABASE.md)** - Complete schema reference
- **[Setup Guide](SETUP.md)** - Service configuration

---

## References

- PostgreSQL UNLOGGED tables: https://www.postgresql.org/docs/current/sql-createtable.html#SQL-CREATETABLE-UNLOGGED
- Token bucket algorithm: https://en.wikipedia.org/wiki/Token_bucket
- Circuit breaker pattern: https://martinfowler.com/bliki/CircuitBreaker.html
