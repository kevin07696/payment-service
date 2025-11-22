package payment_method

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	// Cache metrics
	// Note: paymentMethodCacheHits uses no labels to avoid allocation overhead
	// At 1000 TPS with 95% hit rate and 80% saved cards, labels would cause 760 allocations/sec
	paymentMethodCacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "payment_method_cache_hits_total",
		Help: "Total number of payment method cache hits",
	})

	paymentMethodCacheMisses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payment_method_cache_misses_total",
		Help: "Total number of payment method cache misses",
	}, []string{"reason"}) // expired, not_found, error

	paymentMethodCacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "payment_method_cache_size",
		Help: "Current number of payment methods in cache",
	})

	paymentMethodCacheEvictions = promauto.NewCounter(prometheus.CounterOpts{
		Name: "payment_method_cache_evictions_total",
		Help: "Total number of cache evictions due to size limit",
	})
)

// PaymentMethodCache caches payment method data to reduce database queries
// Expected cache hit rate: 90-95% (saved payment methods reused frequently)
// At 1000 TPS with 80% saved PM: saves 720-760 DB queries/sec
//
// LRU Eviction: This cache uses "approximate LRU" eviction.
// The eviction algorithm iterates through sync.Map to find the oldest entry,
// but sync.Map.Range() does not guarantee consistent iteration order.
// This is acceptable for caching use cases - we still evict old entries,
// just not necessarily THE oldest. The performance benefit of sync.Map's
// lock-free reads outweighs the imprecision in eviction order.
type PaymentMethodCache struct {
	cache   sync.Map // map[uuid.UUID]*CachedPaymentMethod
	queries sqlc.Querier
	logger  *zap.Logger

	// Configuration
	ttl     time.Duration
	maxSize int

	// LRU tracking for eviction
	accessTimes sync.Map // map[uuid.UUID]time.Time
	mu          sync.RWMutex
}

// CachedPaymentMethod stores a payment method with expiration
type CachedPaymentMethod struct {
	pm        *domain.PaymentMethod
	cachedAt  time.Time
	expiresAt time.Time
	mu        sync.RWMutex
}

// NewPaymentMethodCache creates a new payment method cache
func NewPaymentMethodCache(
	queries sqlc.Querier,
	logger *zap.Logger,
	ttl time.Duration,
	maxSize int,
) *PaymentMethodCache {
	return &PaymentMethodCache{
		queries: queries,
		logger:  logger,
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// Get retrieves a payment method from cache or fetches from DB
// Thread-safe for concurrent access
func (c *PaymentMethodCache) Get(ctx context.Context, pmID uuid.UUID) (*domain.PaymentMethod, error) {
	// Fast path: check cache
	if val, ok := c.cache.Load(pmID); ok {
		cached := val.(*CachedPaymentMethod)

		// Check validity and get payment method while holding lock
		var pm *domain.PaymentMethod
		var isValid bool
		cached.mu.RLock()
		isValid = time.Now().Before(cached.expiresAt)
		if isValid {
			pm = cached.pm
		}
		cached.mu.RUnlock()

		// If valid, update access time and return (no lock held)
		if isValid {
			// Update access time for LRU (after releasing lock to avoid contention)
			c.accessTimes.Store(pmID, time.Now())

			// Record cache hit metric (no labels to avoid allocations)
			paymentMethodCacheHits.Inc()

			c.logger.Debug("Payment method cache hit",
				zap.String("payment_method_id", pmID.String()),
			)

			return pm, nil
		}

		// Cache entry expired
		paymentMethodCacheMisses.WithLabelValues("expired").Inc()
		c.logger.Debug("Payment method cache expired",
			zap.String("payment_method_id", pmID.String()),
		)
	} else {
		// Not in cache
		paymentMethodCacheMisses.WithLabelValues("not_found").Inc()
	}

	// Slow path: fetch from DB
	return c.fetchAndCache(ctx, pmID)
}

// fetchAndCache fetches payment method from DB and caches it
func (c *PaymentMethodCache) fetchAndCache(ctx context.Context, pmID uuid.UUID) (*domain.PaymentMethod, error) {
	c.logger.Debug("Fetching payment method from DB",
		zap.String("payment_method_id", pmID.String()),
	)

	// Fetch from database
	dbPM, err := c.queries.GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		paymentMethodCacheMisses.WithLabelValues("error").Inc()
		return nil, fmt.Errorf("failed to fetch payment method: %w", err)
	}

	// Check if context was cancelled after DB operation
	select {
	case <-ctx.Done():
		paymentMethodCacheMisses.WithLabelValues("cancelled").Inc()
		return nil, ctx.Err()
	default:
	}

	// Convert sqlc model to domain model
	pm := convertSqlcToPaymentMethod(&dbPM)

	// Create cached entry
	now := time.Now()
	cached := &CachedPaymentMethod{
		pm:        pm,
		cachedAt:  now,
		expiresAt: now.Add(c.ttl),
	}

	// Store in cache
	c.cache.Store(pmID, cached)
	c.accessTimes.Store(pmID, now)

	// Update cache size metric
	c.updateCacheSize()

	// Evict oldest entries if max size exceeded
	c.evictIfNeeded()

	c.logger.Debug("Cached payment method",
		zap.String("payment_method_id", pmID.String()),
		zap.Duration("ttl", c.ttl),
	)

	return pm, nil
}

// Invalidate removes a payment method from the cache
// Call this when payment method is updated, deleted, or verification status changes
func (c *PaymentMethodCache) Invalidate(pmID uuid.UUID) {
	c.cache.Delete(pmID)
	c.accessTimes.Delete(pmID)

	c.updateCacheSize()

	c.logger.Info("Invalidated payment method cache entry",
		zap.String("payment_method_id", pmID.String()),
	)
}

// InvalidateByCustomer removes all payment methods for a customer
// Useful when customer's payment methods are bulk updated
func (c *PaymentMethodCache) InvalidateByCustomer(customerID string) {
	count := 0
	c.cache.Range(func(key, value interface{}) bool {
		cached := value.(*CachedPaymentMethod)
		cached.mu.RLock()
		if cached.pm.CustomerID == customerID {
			cached.mu.RUnlock()
			pmID := key.(uuid.UUID)
			c.cache.Delete(pmID)
			c.accessTimes.Delete(pmID)
			count++
		} else {
			cached.mu.RUnlock()
		}
		return true
	})

	if count > 0 {
		c.updateCacheSize()
		c.logger.Info("Invalidated payment methods by customer",
			zap.String("customer_id", customerID),
			zap.Int("count", count),
		)
	}
}

// InvalidateAll clears the entire cache
func (c *PaymentMethodCache) InvalidateAll() {
	c.cache.Range(func(key, value interface{}) bool {
		c.cache.Delete(key)
		c.accessTimes.Delete(key)
		return true
	})

	c.updateCacheSize()

	c.logger.Info("Invalidated entire payment method cache")
}

// evictIfNeeded implements LRU eviction when cache exceeds maxSize
func (c *PaymentMethodCache) evictIfNeeded() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Count current size
	size := 0
	c.cache.Range(func(key, value interface{}) bool {
		size++
		return true
	})

	// If under limit, no eviction needed
	if size <= c.maxSize {
		return
	}

	// Find oldest entries to evict
	type entry struct {
		id         uuid.UUID
		accessTime time.Time
	}

	var entries []entry
	c.accessTimes.Range(func(key, value interface{}) bool {
		entries = append(entries, entry{
			id:         key.(uuid.UUID),
			accessTime: value.(time.Time),
		})
		return true
	})

	// Sort by access time (oldest first)
	for i := 0; i < len(entries)-1; i++ {
		for j := 0; j < len(entries)-i-1; j++ {
			if entries[j].accessTime.After(entries[j+1].accessTime) {
				entries[j], entries[j+1] = entries[j+1], entries[j]
			}
		}
	}

	// Evict oldest 10% or until under maxSize
	evictCount := (size - c.maxSize) + (c.maxSize / 10)
	for i := 0; i < evictCount && i < len(entries); i++ {
		c.cache.Delete(entries[i].id)
		c.accessTimes.Delete(entries[i].id)
		paymentMethodCacheEvictions.Inc()

		c.logger.Debug("Evicted payment method from cache (LRU)",
			zap.String("payment_method_id", entries[i].id.String()),
			zap.Time("last_access", entries[i].accessTime),
		)
	}

	c.updateCacheSize()
}

// updateCacheSize updates the Prometheus cache size gauge
func (c *PaymentMethodCache) updateCacheSize() {
	size := 0
	c.cache.Range(func(key, value interface{}) bool {
		size++
		return true
	})
	paymentMethodCacheSize.Set(float64(size))
}

// convertSqlcToPaymentMethod converts sqlc model to domain model for caching
// Internal helper for cache - duplicated from service to avoid import cycles
func convertSqlcToPaymentMethod(dbPM *sqlc.CustomerPaymentMethod) *domain.PaymentMethod {
	pm := &domain.PaymentMethod{
		ID:           dbPM.ID.String(),
		MerchantID:   dbPM.MerchantID.String(),
		CustomerID:   dbPM.CustomerID,
		PaymentToken: dbPM.Bric,
		PaymentType:  domain.PaymentMethodType(dbPM.PaymentType),
		LastFour:     dbPM.LastFour,
		IsDefault:    dbPM.IsDefault.Bool,
		IsActive:     dbPM.IsActive.Bool,
		IsVerified:   dbPM.IsVerified.Bool,
		CreatedAt:    dbPM.CreatedAt,
		UpdatedAt:    dbPM.UpdatedAt,
	}

	// Optional credit card fields
	if dbPM.CardBrand.Valid {
		pm.CardBrand = &dbPM.CardBrand.String
	}
	if dbPM.CardExpMonth.Valid {
		month := int(dbPM.CardExpMonth.Int32)
		pm.CardExpMonth = &month
	}
	if dbPM.CardExpYear.Valid {
		year := int(dbPM.CardExpYear.Int32)
		pm.CardExpYear = &year
	}

	// Optional ACH fields
	if dbPM.BankName.Valid {
		pm.BankName = &dbPM.BankName.String
	}
	if dbPM.AccountType.Valid {
		pm.AccountType = &dbPM.AccountType.String
	}

	// Optional metadata fields
	if dbPM.LastUsedAt.Valid {
		pm.LastUsedAt = &dbPM.LastUsedAt.Time
	}
	if dbPM.VerificationStatus.Valid {
		pm.VerificationStatus = &dbPM.VerificationStatus.String
	}
	if dbPM.VerifiedAt.Valid {
		pm.VerifiedAt = &dbPM.VerifiedAt.Time
	}
	if dbPM.DeactivatedAt.Valid {
		pm.DeactivatedAt = &dbPM.DeactivatedAt.Time
	}
	if dbPM.DeactivationReason.Valid {
		pm.DeactivationReason = &dbPM.DeactivationReason.String
	}
	if dbPM.VerificationFailureReason.Valid {
		pm.VerificationFailureReason = &dbPM.VerificationFailureReason.String
	}
	if dbPM.PrenoteTransactionID.Valid {
		prenoteID := uuid.UUID(dbPM.PrenoteTransactionID.Bytes).String()
		pm.PreNoteTransactionID = &prenoteID
	}

	returnCount := int(dbPM.ReturnCount)
	pm.ReturnCount = &returnCount

	return pm
}
