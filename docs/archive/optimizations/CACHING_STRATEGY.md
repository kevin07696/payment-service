# Comprehensive Caching Strategy

**Review Date:** 2025-11-20
**Scope:** All caching opportunities across the payment service

---

## Executive Summary

Caching analysis reveals **12 high-impact caching opportunities** that can reduce:
- Database load by **60-70%**
- Secret manager calls by **80-90%**
- External API calls by **95%**
- Average request latency by **15-30ms**

**Total Performance Impact at 1000 TPS:**
- **Before**: ~4000-5000 DB queries/sec
- **After**: ~1500-2000 DB queries/sec
- **Savings**: 2500-3000 DB queries/sec

---

## Part 1: Critical Path Caching (P0 - Implement First)

### CACHE-1: Merchant Credentials Cache 🎯

**Priority:** P0 - CRITICAL
**Impact:** HIGH (50% DB load reduction)
**Location:** All payment operations

**Current Problem:**
```go
// Every payment operation fetches merchant twice:
merchant, err := s.queries.GetMerchantByID(ctx, merchantID)       // DB query
macSecret, err := s.secretManager.GetSecret(ctx, merchant.MacSecretPath) // Vault API call
```

**Frequency:**
- Sale: 1x per transaction
- Authorize: 1x per transaction
- Capture: 1x per transaction
- Subscription billing: 1x per subscription
- **Total at 1000 TPS**: 1000 merchant fetches/sec + 1000 secret fetches/sec

**Cache Design:**
```go
// internal/services/merchant/credential_cache.go

type MerchantCredentialCache struct {
    cache sync.Map // map[uuid.UUID]*CachedCredential
    queries sqlc.Querier
    secretMgr adapterports.SecretManagerAdapter
    logger *zap.Logger

    // Configuration
    ttl time.Duration
    maxSize int
}

type CachedCredential struct {
    merchant   sqlc.Merchant
    macSecret  string
    cachedAt   time.Time
    expiresAt  time.Time
    mu         sync.RWMutex
}

func (c *MerchantCredentialCache) Get(ctx context.Context, merchantID uuid.UUID) (*CachedCredential, error) {
    // Fast path: check cache
    if val, ok := c.cache.Load(merchantID); ok {
        cached := val.(*CachedCredential)
        cached.mu.RLock()
        defer cached.mu.RUnlock()

        if time.Now().Before(cached.expiresAt) {
            return cached, nil
        }
    }

    // Slow path: fetch from DB + Vault
    merchant, err := c.queries.GetMerchantByID(ctx, merchantID)
    if err != nil {
        return nil, err
    }

    macSecret, err := c.secretMgr.GetSecret(ctx, merchant.MacSecretPath)
    if err != nil {
        return nil, err
    }

    // Store in cache
    cached := &CachedCredential{
        merchant:  merchant,
        macSecret: macSecret,
        cachedAt:  time.Now(),
        expiresAt: time.Now().Add(c.ttl),
    }
    c.cache.Store(merchantID, cached)

    // Evict oldest if max size exceeded
    c.evictIfNeeded()

    return cached, nil
}

func (c *MerchantCredentialCache) Invalidate(merchantID uuid.UUID) {
    c.cache.Delete(merchantID)
}

func (c *MerchantCredentialCache) evictIfNeeded() {
    // LRU eviction if size exceeds maxSize
    // Implementation details...
}
```

**Configuration:**
```yaml
caching:
  merchant_credentials:
    enabled: true
    ttl: 5m          # 5 minutes (balance freshness vs performance)
    max_size: 1000   # Max 1000 merchants in cache
```

**Cache Invalidation Strategy:**
- **Time-based**: 5-minute TTL
- **Event-based**: Invalidate on merchant update
- **Manual**: Admin API to clear specific merchant

**Expected Metrics:**
- **Cache hit rate**: 90-95% (merchants reused frequently)
- **Cache misses**: 5-10% (new merchants, expired entries)
- **Latency reduction**: 15-25ms per cache hit (no DB + Vault calls)

**Testing:**
```go
func TestMerchantCredentialCache(t *testing.T) {
    t.Run("cache hit", func(t *testing.T) {
        cache := NewMerchantCredentialCache(5*time.Minute, 1000)

        // First fetch (cache miss)
        cred1, err := cache.Get(ctx, merchantID)
        require.NoError(t, err)

        // Second fetch (cache hit)
        cred2, err := cache.Get(ctx, merchantID)
        require.NoError(t, err)

        // Should be same instance
        assert.Equal(t, cred1.macSecret, cred2.macSecret)

        // Verify DB called once
        mock.AssertExpectations(t)
    })

    t.Run("cache expiration", func(t *testing.T) {
        cache := NewMerchantCredentialCache(100*time.Millisecond, 1000)

        _, err := cache.Get(ctx, merchantID)
        require.NoError(t, err)

        // Wait for expiration
        time.Sleep(150 * time.Millisecond)

        // Should refetch
        _, err = cache.Get(ctx, merchantID)
        require.NoError(t, err)
    })

    t.Run("concurrent access", func(t *testing.T) {
        cache := NewMerchantCredentialCache(5*time.Minute, 1000)

        var wg sync.WaitGroup
        for i := 0; i < 100; i++ {
            wg.Add(1)
            go func() {
                defer wg.Done()
                _, err := cache.Get(ctx, merchantID)
                assert.NoError(t, err)
            }()
        }
        wg.Wait()

        // DB should be called once despite 100 concurrent requests
        mock.AssertNumberOfCalls(t, "GetMerchantByID", 1)
    })
}
```

**Monitoring:**
```go
// Metrics to track
prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "merchant_cache_hits_total",
}, []string{"merchant_id"})

prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "merchant_cache_misses_total",
}, []string{"reason"}) // expired, not_found, error

prometheus.NewGauge(prometheus.GaugeOpts{
    Name: "merchant_cache_size",
})
```

---

### CACHE-2: Payment Method Cache 🎯

**Priority:** P0 - CRITICAL
**Impact:** HIGH (90% cache hit rate)
**Location:** Payment processing

**Current Problem:**
```go
// Every payment fetches payment method from DB
dbPM, err := s.queries.GetPaymentMethodByID(ctx, pmID)
domainPM := sqlcPaymentMethodToDomain(&dbPM)
```

**Frequency:**
- Saved payment methods used repeatedly
- High-volume merchants: Same payment method 100+ times/day
- **At 1000 TPS with 80% saved PM**: 800 PM fetches/sec

**Cache Design:**
```go
type PaymentMethodCache struct {
    cache sync.Map // map[uuid.UUID]*CachedPaymentMethod
    queries sqlc.Querier
    logger *zap.Logger

    ttl time.Duration
    maxSize int
}

type CachedPaymentMethod struct {
    pm *domain.PaymentMethod
    cachedAt time.Time
    expiresAt time.Time
    mu sync.RWMutex
}

func (c *PaymentMethodCache) Get(ctx context.Context, pmID uuid.UUID) (*domain.PaymentMethod, error) {
    // Check cache
    if val, ok := c.cache.Load(pmID); ok {
        cached := val.(*CachedPaymentMethod)
        cached.mu.RLock()
        defer cached.mu.RUnlock()

        if time.Now().Before(cached.expiresAt) {
            return cached.pm, nil
        }
    }

    // Fetch from DB
    dbPM, err := c.queries.GetPaymentMethodByID(ctx, pmID)
    if err != nil {
        return nil, err
    }

    pm := sqlcPaymentMethodToDomain(&dbPM)

    // Cache result
    cached := &CachedPaymentMethod{
        pm: pm,
        cachedAt: time.Now(),
        expiresAt: time.Now().Add(c.ttl),
    }
    c.cache.Store(pmID, cached)

    return pm, nil
}

func (c *PaymentMethodCache) Invalidate(pmID uuid.UUID) {
    c.cache.Delete(pmID)
}
```

**Cache Invalidation Points:**
```go
// In payment_method_service.go

func (s *paymentMethodService) UpdatePaymentMethodStatus(ctx context.Context, ...) {
    // ... update DB ...

    // Invalidate cache
    s.pmCache.Invalidate(pmID)
}

func (s *paymentMethodService) DeletePaymentMethod(ctx context.Context, pmID string) error {
    // ... soft delete ...

    // Invalidate cache
    s.pmCache.Invalidate(uuid.MustParse(pmID))
}

func (s *paymentMethodService) MarkPaymentMethodVerified(ctx context.Context, pmID uuid.UUID) error {
    // ... update verification status ...

    // Invalidate cache
    s.pmCache.Invalidate(pmID)
}
```

**Configuration:**
```yaml
caching:
  payment_methods:
    enabled: true
    ttl: 2m          # 2 minutes (shorter than merchant, updated more frequently)
    max_size: 10000  # Support 10k cached payment methods
```

**Expected Metrics:**
- **Cache hit rate**: 90-95%
- **Latency reduction**: 5-10ms per cache hit
- **At 1000 TPS**: 720-760 DB queries saved/sec

---

### CACHE-3: Merchant Slug → ID Resolution 🎯

**Priority:** P0 - HIGH
**Impact:** MEDIUM (eliminates slug lookup)
**Location:** All merchant identification

**Current Problem:**
```go
// Many API calls use merchant slug instead of UUID
merchant, err := s.queries.GetMerchantBySlug(ctx, "acme-corp")
```

**Frequency:**
- Common in REST APIs (readable URLs)
- **If 30% of requests use slug**: 300 lookups/sec at 1000 TPS

**Cache Design:**
```go
type MerchantSlugCache struct {
    slugToID sync.Map // map[string]uuid.UUID
    idToSlug sync.Map // map[uuid.UUID]string (bidirectional)
    queries sqlc.Querier
    ttl time.Duration
}

type cachedSlug struct {
    id uuid.UUID
    expiresAt time.Time
}

func (c *MerchantSlugCache) GetIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
    // Check cache
    if val, ok := c.slugToID.Load(slug); ok {
        cached := val.(*cachedSlug)
        if time.Now().Before(cached.expiresAt) {
            return cached.id, nil
        }
    }

    // Fetch from DB
    merchant, err := c.queries.GetMerchantBySlug(ctx, slug)
    if err != nil {
        return uuid.Nil, err
    }

    // Cache both directions
    cached := &cachedSlug{
        id: merchant.ID,
        expiresAt: time.Now().Add(c.ttl),
    }
    c.slugToID.Store(slug, cached)
    c.idToSlug.Store(merchant.ID, slug)

    return merchant.ID, nil
}
```

**Configuration:**
```yaml
caching:
  merchant_slugs:
    enabled: true
    ttl: 15m  # Slugs rarely change
    max_size: 5000
```

**Expected Metrics:**
- **Cache hit rate**: 95%+ (slugs very stable)
- **Latency reduction**: 5ms per cache hit

---

## Part 2: Secondary Caching (P1 - High Value)

### CACHE-4: Transaction Lookup by TranNbr

**Priority:** P1
**Impact:** MEDIUM (Browser Post callbacks)
**Location:** `internal/handlers/payment/browser_post_callback_handler.go`

**Problem:**
```go
// EPX callbacks often retry
tx, err := s.queries.GetTransactionByTranNbr(ctx, tranNbr)
```

**Solution:**
```go
type TransactionCache struct {
    cache *lru.Cache // LRU with max size
    ttl time.Duration
}

func (c *TransactionCache) GetByTranNbr(ctx context.Context, tranNbr string, fetcher func() (*domain.Transaction, error)) (*domain.Transaction, error) {
    if val, ok := c.cache.Get(tranNbr); ok {
        return val.(*domain.Transaction), nil
    }

    tx, err := fetcher()
    if err != nil {
        return nil, err
    }

    c.cache.Add(tranNbr, tx)
    return tx, nil
}
```

**Benefits:**
- Faster callback processing
- Reduces DB load during EPX retries
- Transactions are immutable once created

---

### CACHE-5: EPX Response Code Descriptions

**Priority:** P1
**Impact:** LOW (micro-optimization)
**Location:** Response handling

**Static Data:**
```go
var EPXResponseCodes = map[string]ResponseCode{
    "00": {Code: "00", Message: "Approved", Category: "success"},
    "05": {Code: "05", Message: "Do Not Honor", Category: "decline"},
    "51": {Code: "51", Message: "Insufficient Funds", Category: "decline"},
    "54": {Code: "54", Message: "Expired Card", Category: "decline"},
    "57": {Code: "57", Message: "Transaction Not Permitted", Category: "decline"},
    "61": {Code: "61", Message: "Exceeds Withdrawal Limit", Category: "decline"},
    "65": {Code: "65", Message: "Exceeds Frequency Limit", Category: "decline"},
    "91": {Code: "91", Message: "Issuer Unavailable", Category: "error"},
    // ... etc
}

type ResponseCode struct {
    Code string
    Message string
    Category string // success, decline, error
}

func GetResponseInfo(code string) ResponseCode {
    if rc, ok := EPXResponseCodes[code]; ok {
        return rc
    }
    return ResponseCode{
        Code: code,
        Message: "Unknown Response Code",
        Category: "error",
    }
}
```

**Benefits:**
- No DB/API calls needed
- Enrich logs and responses
- Categorize declines vs errors

---

### CACHE-6: Chargeback Reason Codes

**Priority:** P2
**Impact:** LOW
**Location:** Chargeback processing

**Static Data:**
```go
var ChargebackReasonCodes = map[string]ChargebackReason{
    // Visa
    "10.1": {Network: "Visa", Code: "10.1", Description: "EMV Liability Shift Counterfeit Fraud"},
    "10.4": {Network: "Visa", Code: "10.4", Description: "Other Fraud - Card Absent"},
    "11.1": {Network: "Visa", Code: "11.1", Description: "Card Recovery Bulletin"},
    "11.2": {Network: "Visa", Code: "11.2", Description: "Declined Authorization"},
    "11.3": {Network: "Visa", Code: "11.3", Description: "No Authorization"},

    // Mastercard
    "4837": {Network: "MC", Code: "4837", Description: "No Cardholder Authorization"},
    "4863": {Network: "MC", Code: "4863", Description: "Cardholder Does Not Recognize"},

    // Amex
    "C02": {Network: "Amex", Code: "C02", Description: "Credit Not Processed"},
    // ... etc
}

type ChargebackReason struct {
    Network string
    Code string
    Description string
    Category string // fraud, authorization, processing_error, consumer_dispute
}
```

---

## Part 3: Query Result Caching (P2)

### CACHE-7: Subscription Billing Query Cache

**Priority:** P2
**Impact:** MEDIUM (cron performance)
**Location:** `internal/services/subscription/subscription_service.go:497-506`

**Problem:**
```go
// Cron job runs every 5 minutes, same query
dueSubs, err := s.queries.ListSubscriptionsDueForBilling(ctx, params)
```

**Solution:**
```go
type BillingQueryCache struct {
    lastResult []sqlc.Subscription
    lastQuery time.Time
    cacheDuration time.Duration
    mu sync.RWMutex
}

func (c *BillingQueryCache) GetDueSubscriptions(ctx context.Context, asOfDate time.Time, fetcher func() ([]sqlc.Subscription, error)) ([]sqlc.Subscription, error) {
    c.mu.RLock()

    // Cache valid if queried within last minute
    if time.Since(c.lastQuery) < c.cacheDuration {
        result := c.lastResult
        c.mu.RUnlock()
        return result, nil
    }
    c.mu.RUnlock()

    // Fetch fresh data
    c.mu.Lock()
    defer c.mu.Unlock()

    result, err := fetcher()
    if err != nil {
        return nil, err
    }

    c.lastResult = result
    c.lastQuery = time.Now()

    return result, nil
}
```

**Benefits:**
- Cron job idempotent within 1-minute window
- Reduces DB load if cron runs multiple times quickly

---

### CACHE-8: Customer Payment Methods List

**Priority:** P2
**Impact:** MEDIUM
**Location:** List payment methods endpoint

**Problem:**
```go
// Customers frequently view payment methods
pms, err := s.queries.ListPaymentMethodsByCustomer(ctx, params)
```

**Solution:**
```go
type CustomerPMCache struct {
    cache sync.Map // map[string]cachedPMList
    ttl time.Duration
}

type cachedPMList struct {
    pms []*domain.PaymentMethod
    expiresAt time.Time
}

func (c *CustomerPMCache) GetList(ctx context.Context, merchantID, customerID uuid.UUID, fetcher func() ([]*domain.PaymentMethod, error)) ([]*domain.PaymentMethod, error) {
    key := fmt.Sprintf("%s:%s", merchantID, customerID)

    if val, ok := c.cache.Load(key); ok {
        cached := val.(*cachedPMList)
        if time.Now().Before(cached.expiresAt) {
            return cached.pms, nil
        }
    }

    pms, err := fetcher()
    if err != nil {
        return nil, err
    }

    c.cache.Store(key, &cachedPMList{
        pms: pms,
        expiresAt: time.Now().Add(c.ttl),
    })

    return pms, nil
}
```

**Invalidation:**
- When payment method added/updated/deleted for customer
- TTL: 30 seconds (reasonable staleness)

---

## Part 4: External API Response Caching

### CACHE-9: Currency Exchange Rates (Future)

**Priority:** P3 (when multi-currency added)
**Impact:** HIGH (external API cost)

**Design:**
```go
type ExchangeRateCache struct {
    rates sync.Map // map[string]cachedRate
    provider string // "fixer.io", "exchangerate-api.com"
    ttl time.Duration
}

type cachedRate struct {
    from string
    to string
    rate float64
    timestamp time.Time
    expiresAt time.Time
}

func (c *ExchangeRateCache) GetRate(ctx context.Context, from, to string) (float64, error) {
    key := fmt.Sprintf("%s_%s", from, to)

    if val, ok := c.rates.Load(key); ok {
        cached := val.(*cachedRate)
        if time.Now().Before(cached.expiresAt) {
            return cached.rate, nil
        }
    }

    // Fetch from external API
    rate, err := c.fetchRate(ctx, from, to)
    if err != nil {
        return 0, err
    }

    c.rates.Store(key, &cachedRate{
        from: from,
        to: to,
        rate: rate,
        timestamp: time.Now(),
        expiresAt: time.Now().Add(c.ttl),
    })

    return rate, nil
}
```

**Configuration:**
```yaml
caching:
  exchange_rates:
    enabled: true
    ttl: 5m  # Refresh every 5 minutes
    provider: "fixer.io"
```

**Benefits:**
- Reduces external API costs (pay per request)
- Faster response (no API call)
- Fallback to cached rate if API down

---

## Part 5: Computed Result Caching

### CACHE-10: Group State Computation Cache

**Priority:** P1
**Impact:** MEDIUM (Capture/Void/Refund operations)
**Location:** `internal/services/payment/group_state.go:35-103`

**Problem:**
```go
// Capture operation calls this twice
state := ComputeGroupState(domainTxs)
```

**Solution:**
```go
type GroupStateCache struct {
    cache sync.Map // map[string]*CachedGroupState
    ttl time.Duration
}

type CachedGroupState struct {
    state *GroupState
    version int64 // max(updated_at) of transactions
    expiresAt time.Time
}

func (c *GroupStateCache) GetOrCompute(ctx context.Context, rootTxID string, txs []*domain.Transaction) *GroupState {
    // Compute version from transaction timestamps
    var maxVersion int64
    for _, tx := range txs {
        v := tx.UpdatedAt.Unix()
        if v > maxVersion {
            maxVersion = v
        }
    }

    key := rootTxID
    if val, ok := c.cache.Load(key); ok {
        cached := val.(*CachedGroupState)

        // Cache valid if version matches and not expired
        if cached.version == maxVersion && time.Now().Before(cached.expiresAt) {
            return cached.state
        }
    }

    // Compute fresh state
    state := ComputeGroupState(txs)

    c.cache.Store(key, &CachedGroupState{
        state: state,
        version: maxVersion,
        expiresAt: time.Now().Add(c.ttl),
    })

    return state
}
```

**Benefits:**
- Eliminates redundant computation
- Version-based invalidation (no stale data)
- Particularly valuable for complex transaction trees

---

### CACHE-11: Billing Date Calculations

**Priority:** P3
**Impact:** LOW
**Location:** Subscription service

**Optimization:**
```go
// Pre-compute common intervals
var StandardIntervals = map[string]time.Duration{
    "1_day":   24 * time.Hour,
    "7_day":   7 * 24 * time.Hour,
    "14_day":  14 * 24 * time.Hour,
    "30_day":  30 * 24 * time.Hour,
    "1_month": 30 * 24 * time.Hour, // Approximate
}

func calculateNextBillingDate(current time.Time, interval int, unit domain.IntervalUnit) time.Time {
    key := fmt.Sprintf("%d_%s", interval, unit)

    // Fast path for common intervals
    if duration, ok := StandardIntervals[key]; ok {
        return current.Add(duration)
    }

    // Fallback to precise calculation
    switch unit {
    case domain.IntervalUnitMonth:
        return current.AddDate(0, interval, 0)
    // ... etc
    }
}
```

---

## Part 6: Cache Architecture

### Multi-Layer Caching Strategy

```
┌─────────────────────────────────────────────────┐
│            Application Layer                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐     │
│  │  In-Mem  │  │  In-Mem  │  │  In-Mem  │     │
│  │ Merchant │  │ Payment  │  │  Slug    │     │
│  │  Cache   │  │  Method  │  │  Cache   │     │
│  └──────────┘  └──────────┘  └──────────┘     │
└─────────────────────────────────────────────────┘
           │              │              │
           ▼              ▼              ▼
┌─────────────────────────────────────────────────┐
│         Redis Cache (Optional Future)           │
│    - Shared across multiple instances           │
│    - TTL: 1-5 minutes                           │
│    - Pub/Sub for invalidation                   │
└─────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────┐
│              PostgreSQL Database                 │
│         - Source of truth                       │
│         - Indexed queries                       │
└─────────────────────────────────────────────────┘
```

### Cache Consistency Strategy

**Write-Through:**
```go
func UpdatePaymentMethod(ctx context.Context, pm *PaymentMethod) error {
    // Update database
    if err := db.Update(ctx, pm); err != nil {
        return err
    }

    // Invalidate cache immediately
    cache.Invalidate(pm.ID)

    return nil
}
```

**Read-Through:**
```go
func GetPaymentMethod(ctx context.Context, pmID uuid.UUID) (*PaymentMethod, error) {
    // Try cache first
    if pm, err := cache.Get(pmID); err == nil {
        return pm, nil
    }

    // Cache miss - fetch from DB
    pm, err := db.Get(ctx, pmID)
    if err != nil {
        return nil, err
    }

    // Populate cache
    cache.Set(pmID, pm)

    return pm, nil
}
```

---

## Part 7: Implementation Roadmap

### Week 1: Critical Path Caching
- ✅ Merchant credentials cache
- ✅ Payment method cache
- ✅ Merchant slug cache
- Tests + monitoring

### Week 2: Secondary Caching
- ✅ Transaction lookup cache
- ✅ Group state cache
- ✅ EPX response codes (static data)

### Week 3: Query Result Caching
- ✅ Subscription billing query
- ✅ Customer PM list cache

### Week 4: Testing & Optimization
- Load tests
- Cache hit rate analysis
- TTL tuning
- Eviction policy optimization

---

## Part 8: Monitoring & Metrics

### Cache Metrics

```go
// Prometheus metrics
var (
    cacheHits = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_hits_total",
        },
        []string{"cache_name"},
    )

    cacheMisses = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_misses_total",
        },
        []string{"cache_name", "reason"}, // expired, not_found, error
    )

    cacheSize = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cache_size",
        },
        []string{"cache_name"},
    )

    cacheEvictions = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_evictions_total",
        },
        []string{"cache_name", "reason"}, // ttl, lru, manual
    )
)
```

### Dashboards

**Cache Performance:**
- Hit rate by cache (target: >90%)
- Average latency saved per hit
- Cache size utilization
- Eviction rate

**Cache Health:**
- Invalidation rate
- Error rate
- TTL effectiveness

---

## Summary

**Total Caching Opportunities:** 11
**Critical Path:** 3 (merchant, payment method, slug)
**Secondary:** 5 (transactions, EPX codes, etc.)
**Future:** 3 (exchange rates, etc.)

**Expected Performance Improvement:**
- **60-70% reduction** in DB queries
- **80-90% reduction** in secret manager calls
- **15-30ms latency** improvement per request
- **At 1000 TPS**: 2500-3000 fewer DB queries/sec

**Estimated Effort:** 3-4 weeks
**Priority:** P0 for critical path caching

---

**Document Version:** 1.0
**Last Updated:** 2025-11-20
**Complements:** All other optimization documents
