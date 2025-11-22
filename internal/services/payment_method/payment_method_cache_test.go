package payment_method

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Helper function to create test payment method
func createTestPaymentMethod(id uuid.UUID, customerID string) sqlc.CustomerPaymentMethod {
	return sqlc.CustomerPaymentMethod{
		ID:          id,
		MerchantID:  uuid.New(),
		CustomerID:  customerID,
		Bric:        "bric_test_123",
		PaymentType: "card",
		LastFour:    "4242",
		IsDefault:   pgtype.Bool{Bool: true, Valid: true},
		IsActive:    pgtype.Bool{Bool: true, Valid: true},
		IsVerified:  pgtype.Bool{Bool: true, Valid: true},
		CardBrand:   pgtype.Text{String: "visa", Valid: true},
		CardExpMonth: pgtype.Int4{Int32: 12, Valid: true},
		CardExpYear:  pgtype.Int4{Int32: 2025, Valid: true},
		ReturnCount:  0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// Test_PaymentMethodCache_Get_CacheHit verifies cache returns cached entry without DB calls
func Test_PaymentMethodCache_Get_CacheHit(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pmID := uuid.New()
	expectedPM := createTestPaymentMethod(pmID, "cust_123")

	mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
		Return(expectedPM, nil).Once()

	// Pre-populate cache
	_, err := cache.Get(context.Background(), pmID)
	require.NoError(t, err)

	// Act - Second call should hit cache
	start := time.Now()
	cached, err := cache.Get(context.Background(), pmID)
	duration := time.Since(start)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cached)
	assert.Equal(t, pmID.String(), cached.ID)
	assert.Equal(t, expectedPM.CustomerID, cached.CustomerID)

	// Verify cache was used (no additional DB calls)
	mockQueries.AssertNumberOfCalls(t, "GetPaymentMethodByID", 1)

	// Cache hit should be very fast (< 10ms)
	assert.Less(t, duration, 10*time.Millisecond)
}

// Test_PaymentMethodCache_Get_CacheMiss verifies DB fetch on cache miss
func Test_PaymentMethodCache_Get_CacheMiss(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pmID := uuid.New()
	expectedPM := createTestPaymentMethod(pmID, "cust_123")

	mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
		Return(expectedPM, nil).Once()

	// Act
	cached, err := cache.Get(context.Background(), pmID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cached)
	assert.Equal(t, pmID.String(), cached.ID)

	// Verify DB was called
	mockQueries.AssertExpectations(t)
}

// Test_PaymentMethodCache_Get_TTLExpiration verifies expired entries trigger refetch
func Test_PaymentMethodCache_Get_TTLExpiration(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	// Use very short TTL for testing
	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		100*time.Millisecond, // 100ms TTL
		100,
	)

	pmID := uuid.New()
	expectedPM := createTestPaymentMethod(pmID, "cust_123")

	// Setup mock to be called twice (initial + after expiration)
	mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
		Return(expectedPM, nil).Twice()

	// First call to populate cache
	_, err := cache.Get(context.Background(), pmID)
	require.NoError(t, err)

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Act - Second call should trigger refetch due to expiration
	cached, err := cache.Get(context.Background(), pmID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cached)

	// Verify DB was called twice (initial + refetch)
	mockQueries.AssertNumberOfCalls(t, "GetPaymentMethodByID", 2)
}

// Test_PaymentMethodCache_Get_DBError verifies error handling for DB failures
func Test_PaymentMethodCache_Get_DBError(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pmID := uuid.New()
	expectedErr := errors.New("database connection failed")

	mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
		Return(sqlc.CustomerPaymentMethod{}, expectedErr).Once()

	// Act
	cached, err := cache.Get(context.Background(), pmID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, cached)
	assert.ErrorContains(t, err, "failed to fetch payment method")
}

// Test_PaymentMethodCache_Get_ContextCancellation verifies context cancellation handling
func Test_PaymentMethodCache_Get_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pmID := uuid.New()
	expectedPM := createTestPaymentMethod(pmID, "cust_123")

	ctx, cancel := context.WithCancel(context.Background())

	// Setup mock to cancel context after DB call
	mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
		Run(func(args mock.Arguments) {
			cancel() // Cancel context after DB fetch
		}).
		Return(expectedPM, nil).Once()

	// Act
	cached, err := cache.Get(ctx, pmID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, cached)
	assert.ErrorIs(t, err, context.Canceled)
}

// Test_PaymentMethodCache_Invalidate verifies cache invalidation
func Test_PaymentMethodCache_Invalidate(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pmID := uuid.New()
	expectedPM := createTestPaymentMethod(pmID, "cust_123")

	// Setup mock to be called twice (initial + after invalidation)
	mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
		Return(expectedPM, nil).Twice()

	// Populate cache
	_, err := cache.Get(context.Background(), pmID)
	require.NoError(t, err)

	// Act - Invalidate cache
	cache.Invalidate(pmID)

	// Get again - should refetch
	cached, err := cache.Get(context.Background(), pmID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cached)

	// Verify DB was called twice (initial + after invalidation)
	mockQueries.AssertNumberOfCalls(t, "GetPaymentMethodByID", 2)
}

// Test_PaymentMethodCache_InvalidateByCustomer verifies bulk invalidation by customer
func Test_PaymentMethodCache_InvalidateByCustomer(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	customerID := "cust_123"

	// Create 3 payment methods for the same customer and 2 for another
	pms := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}

	for i, pmID := range pms {
		var custID string
		if i < 3 {
			custID = customerID // First 3 for cust_123
		} else {
			custID = "cust_456" // Last 2 for cust_456
		}
		pm := createTestPaymentMethod(pmID, custID)

		// Setup mock to be called for initial population and refetch after invalidation
		mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
			Return(pm, nil).Maybe()

		// Populate cache
		_, err := cache.Get(context.Background(), pmID)
		require.NoError(t, err)
	}

	// Act - Invalidate by customer (should invalidate first 3)
	cache.InvalidateByCustomer(customerID)

	// Get all payment methods again
	for _, pmID := range pms {
		_, err := cache.Get(context.Background(), pmID)
		require.NoError(t, err)
	}

	// Assert - Verify the first 3 were refetched (2 calls each: initial + refetch = 6 total)
	// and the last 2 only have initial calls (1 call each = 2 total)
	// Total = 6 + 2 = 8 calls
	mockQueries.AssertNumberOfCalls(t, "GetPaymentMethodByID", 8)
}

// Test_PaymentMethodCache_InvalidateAll verifies clearing entire cache
func Test_PaymentMethodCache_InvalidateAll(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	// Create multiple payment methods
	pms := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for i, pmID := range pms {
		pm := createTestPaymentMethod(pmID, "cust_"+string(rune('A'+i)))

		// Setup mock to be called twice (initial + after invalidation)
		mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
			Return(pm, nil).Twice()

		// Populate cache
		_, err := cache.Get(context.Background(), pmID)
		require.NoError(t, err)
	}

	// Act - Invalidate all
	cache.InvalidateAll()

	// Get all payment methods again - should refetch
	for _, pmID := range pms {
		cached, err := cache.Get(context.Background(), pmID)
		require.NoError(t, err)
		assert.NotNil(t, cached)
	}

	// Assert - Verify each payment method was fetched twice (3 pms * 2 calls = 6 total)
	mockQueries.AssertNumberOfCalls(t, "GetPaymentMethodByID", 6)
}

// Test_PaymentMethodCache_LRUEviction verifies oldest entries evicted when max size exceeded
func Test_PaymentMethodCache_LRUEviction(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	// Small cache size for testing
	maxSize := 5
	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		maxSize,
	)

	// Create more payment methods than cache size
	pms := make([]uuid.UUID, 10)
	for i := 0; i < 10; i++ {
		pms[i] = uuid.New()
		pm := createTestPaymentMethod(pms[i], "cust_"+string(rune('A'+i)))

		mockQueries.On("GetPaymentMethodByID", mock.Anything, pms[i]).
			Return(pm, nil).Maybe()
	}

	// Act - Add payment methods to cache
	for _, pmID := range pms {
		_, err := cache.Get(context.Background(), pmID)
		require.NoError(t, err)

		// Small delay to ensure different access times
		time.Sleep(10 * time.Millisecond)
	}

	// Assert - Cache size should not exceed maxSize
	size := 0
	cache.cache.Range(func(key, value interface{}) bool {
		size++
		return true
	})

	// Cache should have evicted entries to stay under maxSize
	assert.LessOrEqual(t, size, maxSize)
}

// Test_PaymentMethodCache_ConcurrentGet verifies thread safety
func Test_PaymentMethodCache_ConcurrentGet(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pmID := uuid.New()
	expectedPM := createTestPaymentMethod(pmID, "cust_123")

	// Setup mock with thread-safe expectations
	mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
		Return(expectedPM, nil).Once()

	// Pre-populate cache to avoid race on first fetch
	_, err := cache.Get(context.Background(), pmID)
	require.NoError(t, err)

	// Act - 1000 concurrent Get() calls for same payment method (all should hit cache)
	const concurrency = 1000
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errChan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			cached, err := cache.Get(context.Background(), pmID)
			if err != nil {
				errChan <- err
				return
			}
			if cached == nil {
				errChan <- errors.New("cached payment method is nil")
				return
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Assert - No errors should occur
	for err := range errChan {
		t.Errorf("Concurrent Get() error: %v", err)
	}

	// Verify DB called only once (pre-population)
	mockQueries.AssertNumberOfCalls(t, "GetPaymentMethodByID", 1)
}

// Test_PaymentMethodCache_ConcurrentMixedOperations verifies thread safety with mixed operations
func Test_PaymentMethodCache_ConcurrentMixedOperations(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pms := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for i, pmID := range pms {
		pm := createTestPaymentMethod(pmID, "cust_"+string(rune('A'+i)))

		mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
			Return(pm, nil).Maybe()
	}

	// Act - Mixed concurrent operations (Get, Invalidate, InvalidateAll, InvalidateByCustomer)
	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			switch idx % 5 {
			case 0: // Get
				pmID := pms[idx%len(pms)]
				_, _ = cache.Get(context.Background(), pmID)
			case 1: // Invalidate
				pmID := pms[idx%len(pms)]
				cache.Invalidate(pmID)
			case 2: // InvalidateAll
				cache.InvalidateAll()
			case 3: // InvalidateByCustomer
				customerID := "cust_" + string(rune('A'+(idx%len(pms))))
				cache.InvalidateByCustomer(customerID)
			case 4: // Get (another payment method)
				pmID := pms[(idx+1)%len(pms)]
				_, _ = cache.Get(context.Background(), pmID)
			}
		}(i)
	}

	// Wait for all operations to complete (should not deadlock)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("Concurrent operations deadlocked")
	}
}

// Test_PaymentMethodCache_AccessTimeUpdate verifies LRU access time updates
func Test_PaymentMethodCache_AccessTimeUpdate(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(t)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pmID := uuid.New()
	expectedPM := createTestPaymentMethod(pmID, "cust_123")

	mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
		Return(expectedPM, nil).Once()

	// Populate cache
	_, err := cache.Get(context.Background(), pmID)
	require.NoError(t, err)

	// Get initial access time
	initialAccessTimeVal, ok := cache.accessTimes.Load(pmID)
	require.True(t, ok)
	initialAccessTime := initialAccessTimeVal.(time.Time)

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Act - Access again (cache hit)
	_, err = cache.Get(context.Background(), pmID)
	require.NoError(t, err)

	// Assert - Access time should be updated
	updatedAccessTimeVal, ok := cache.accessTimes.Load(pmID)
	require.True(t, ok)
	updatedAccessTime := updatedAccessTimeVal.(time.Time)

	assert.True(t, updatedAccessTime.After(initialAccessTime),
		"Access time should be updated on cache hit")
}

// Test_PaymentMethodCache_ConvertSqlcToPaymentMethod verifies model conversion
func Test_PaymentMethodCache_ConvertSqlcToPaymentMethod(t *testing.T) {
	t.Parallel()

	// Arrange
	pmID := uuid.New()
	merchantID := uuid.New()
	dbPM := createTestPaymentMethod(pmID, "cust_123")
	dbPM.MerchantID = merchantID

	// Act
	domainPM := convertSqlcToPaymentMethod(&dbPM)

	// Assert
	assert.Equal(t, pmID.String(), domainPM.ID)
	assert.Equal(t, merchantID.String(), domainPM.MerchantID)
	assert.Equal(t, "cust_123", domainPM.CustomerID)
	assert.Equal(t, "bric_test_123", domainPM.PaymentToken)
	assert.Equal(t, domain.PaymentMethodType("card"), domainPM.PaymentType)
	assert.Equal(t, "4242", domainPM.LastFour)
	assert.True(t, domainPM.IsDefault)
	assert.True(t, domainPM.IsActive)
	assert.True(t, domainPM.IsVerified)
	assert.NotNil(t, domainPM.CardBrand)
	assert.Equal(t, "visa", *domainPM.CardBrand)
	assert.NotNil(t, domainPM.CardExpMonth)
	assert.Equal(t, 12, *domainPM.CardExpMonth)
	assert.NotNil(t, domainPM.CardExpYear)
	assert.Equal(t, 2025, *domainPM.CardExpYear)
}

// Benchmark_PaymentMethodCache_CacheHit measures cache hit performance
func Benchmark_PaymentMethodCache_CacheHit(b *testing.B) {
	// Setup
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(b)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pmID := uuid.New()
	expectedPM := createTestPaymentMethod(pmID, "cust_123")

	mockQueries.On("GetPaymentMethodByID", mock.Anything, pmID).
		Return(expectedPM, nil).Once()

	// Pre-populate cache
	_, _ = cache.Get(context.Background(), pmID)

	// Reset timer to exclude setup
	b.ResetTimer()

	// Benchmark cache hits
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(context.Background(), pmID)
	}
}

// Benchmark_PaymentMethodCache_CacheMiss measures cache miss performance
func Benchmark_PaymentMethodCache_CacheMiss(b *testing.B) {
	// Setup
	mockQueries := new(mocks.MockQuerier)
	logger := zaptest.NewLogger(b)

	cache := NewPaymentMethodCache(
		mockQueries,
		logger,
		5*time.Minute,
		100,
	)

	pmIDs := make([]uuid.UUID, b.N)
	for i := 0; i < b.N; i++ {
		pmIDs[i] = uuid.New()
		pm := createTestPaymentMethod(pmIDs[i], "cust_"+string(rune('A'+(i%26))))

		mockQueries.On("GetPaymentMethodByID", mock.Anything, pmIDs[i]).
			Return(pm, nil).Once()
	}

	// Reset timer to exclude setup
	b.ResetTimer()

	// Benchmark cache misses
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(context.Background(), pmIDs[i])
	}
}
