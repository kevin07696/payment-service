package merchant

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockSecretManager mocks the ports.SecretManagerAdapter interface
type MockSecretManager struct {
	mock.Mock
}

func (m *MockSecretManager) GetSecret(ctx context.Context, path string) (*ports.Secret, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.Secret), args.Error(1)
}

func (m *MockSecretManager) GetSecretVersion(ctx context.Context, path string, version string) (*ports.Secret, error) {
	args := m.Called(ctx, path, version)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.Secret), args.Error(1)
}

func (m *MockSecretManager) PutSecret(ctx context.Context, path, value string, metadata map[string]string) (string, error) {
	args := m.Called(ctx, path, value, metadata)
	return args.String(0), args.Error(1)
}

func (m *MockSecretManager) RotateSecret(ctx context.Context, path string, newValue string) (*ports.SecretRotationInfo, error) {
	args := m.Called(ctx, path, newValue)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.SecretRotationInfo), args.Error(1)
}

func (m *MockSecretManager) DeleteSecret(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

// Test_MerchantCredentialCache_Get_CacheHit verifies cache returns cached entry without DB/Vault calls
func Test_MerchantCredentialCache_Get_CacheHit(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
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
		ID:            merchantID,
		CustNbr:       "9001",
		MacSecretPath: "secrets/merchant/9001/mac",
	}
	expectedSecret := &ports.Secret{Value: "test-mac-secret"}

	// Pre-populate cache
	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Return(expectedMerchant, nil).Once()
	mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/9001/mac").
		Return(expectedSecret, nil).Once()

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
	assert.Equal(t, expectedSecret.Value, cached.macSecret)

	// Verify cache was used (no additional DB/Vault calls)
	mockQueries.AssertNumberOfCalls(t, "GetMerchantByID", 1)
	mockSecretMgr.AssertNumberOfCalls(t, "GetSecret", 1)

	// Cache hit should be very fast (< 10ms)
	assert.Less(t, duration, 10*time.Millisecond)
}

// Test_MerchantCredentialCache_Get_CacheMiss verifies DB/Vault fetch on cache miss
func Test_MerchantCredentialCache_Get_CacheMiss(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
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
		ID:            merchantID,
		CustNbr:       "9001",
		MacSecretPath: "secrets/merchant/9001/mac",
	}
	expectedSecret := &ports.Secret{Value: "test-mac-secret"}

	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Return(expectedMerchant, nil).Once()
	mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/9001/mac").
		Return(expectedSecret, nil).Once()

	// Act
	cached, err := cache.Get(context.Background(), merchantID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cached)
	assert.Equal(t, expectedMerchant.CustNbr, cached.merchant.CustNbr)
	assert.Equal(t, expectedSecret.Value, cached.macSecret)

	// Verify DB and Vault were called
	mockQueries.AssertExpectations(t)
	mockSecretMgr.AssertExpectations(t)
}

// Test_MerchantCredentialCache_Get_TTLExpiration verifies expired entries trigger refetch
func Test_MerchantCredentialCache_Get_TTLExpiration(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
	logger := zaptest.NewLogger(t)

	// Use very short TTL for testing
	cache := NewMerchantCredentialCache(
		mockQueries,
		mockSecretMgr,
		logger,
		100*time.Millisecond, // 100ms TTL
		100,
	)

	merchantID := uuid.New()
	expectedMerchant := sqlc.Merchant{
		ID:            merchantID,
		CustNbr:       "9001",
		MacSecretPath: "secrets/merchant/9001/mac",
	}
	expectedSecret := &ports.Secret{Value: "test-mac-secret"}

	// Setup mock to be called twice (initial + after expiration)
	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Return(expectedMerchant, nil).Twice()
	mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/9001/mac").
		Return(expectedSecret, nil).Twice()

	// First call to populate cache
	_, err := cache.Get(context.Background(), merchantID)
	require.NoError(t, err)

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Act - Second call should trigger refetch due to expiration
	cached, err := cache.Get(context.Background(), merchantID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cached)

	// Verify DB and Vault were called twice (initial + refetch)
	mockQueries.AssertNumberOfCalls(t, "GetMerchantByID", 2)
	mockSecretMgr.AssertNumberOfCalls(t, "GetSecret", 2)
}

// Test_MerchantCredentialCache_Get_DBError verifies error handling for DB failures
func Test_MerchantCredentialCache_Get_DBError(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
	logger := zaptest.NewLogger(t)

	cache := NewMerchantCredentialCache(
		mockQueries,
		mockSecretMgr,
		logger,
		5*time.Minute,
		100,
	)

	merchantID := uuid.New()
	expectedErr := errors.New("database connection failed")

	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Return(sqlc.Merchant{}, expectedErr).Once()

	// Act
	cached, err := cache.Get(context.Background(), merchantID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, cached)
	assert.True(t, errors.Is(err, domain.ErrMerchantNotFoundTyped), "expected ErrMerchantNotFoundTyped")

	// Vault should not be called if DB fails
	mockSecretMgr.AssertNotCalled(t, "GetSecret")
}

// Test_MerchantCredentialCache_Get_VaultError verifies error handling for Vault failures
func Test_MerchantCredentialCache_Get_VaultError(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
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
		ID:            merchantID,
		CustNbr:       "9001",
		MacSecretPath: "secrets/merchant/9001/mac",
	}
	expectedErr := errors.New("vault connection failed")

	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Return(expectedMerchant, nil).Once()
	mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/9001/mac").
		Return(nil, expectedErr).Once()

	// Act
	cached, err := cache.Get(context.Background(), merchantID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, cached)
	assert.True(t, errors.Is(err, domain.ErrMerchantCredentialFailed), "expected ErrMerchantCredentialFailed")
}

// Test_MerchantCredentialCache_Get_ContextCancellation verifies context cancellation between DB and Vault
func Test_MerchantCredentialCache_Get_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
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
		ID:            merchantID,
		CustNbr:       "9001",
		MacSecretPath: "secrets/merchant/9001/mac",
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Setup mock to cancel context after DB call
	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Run(func(args mock.Arguments) {
			cancel() // Cancel context after DB fetch
		}).
		Return(expectedMerchant, nil).Once()

	// Act
	cached, err := cache.Get(ctx, merchantID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, cached)
	assert.ErrorIs(t, err, context.Canceled)

	// Vault should not be called due to context cancellation
	mockSecretMgr.AssertNotCalled(t, "GetSecret")
}

// Test_MerchantCredentialCache_Invalidate verifies cache invalidation
func Test_MerchantCredentialCache_Invalidate(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
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
		ID:            merchantID,
		CustNbr:       "9001",
		MacSecretPath: "secrets/merchant/9001/mac",
	}
	expectedSecret := &ports.Secret{Value: "test-mac-secret"}

	// Setup mock to be called twice (initial + after invalidation)
	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Return(expectedMerchant, nil).Twice()
	mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/9001/mac").
		Return(expectedSecret, nil).Twice()

	// Populate cache
	_, err := cache.Get(context.Background(), merchantID)
	require.NoError(t, err)

	// Act - Invalidate cache
	cache.Invalidate(merchantID)

	// Get again - should refetch
	cached, err := cache.Get(context.Background(), merchantID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cached)

	// Verify DB and Vault were called twice (initial + after invalidation)
	mockQueries.AssertNumberOfCalls(t, "GetMerchantByID", 2)
	mockSecretMgr.AssertNumberOfCalls(t, "GetSecret", 2)
}

// Test_MerchantCredentialCache_InvalidateAll verifies clearing entire cache
func Test_MerchantCredentialCache_InvalidateAll(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
	logger := zaptest.NewLogger(t)

	cache := NewMerchantCredentialCache(
		mockQueries,
		mockSecretMgr,
		logger,
		5*time.Minute,
		100,
	)

	// Create multiple merchants
	merchants := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for i, merchantID := range merchants {
		merchant := sqlc.Merchant{
			ID:            merchantID,
			CustNbr:       string(rune('A' + i)),
			MacSecretPath: "secrets/merchant/test/mac",
		}
		secret := &ports.Secret{Value: "test-mac-secret"}

		// Setup mock to be called twice (initial + after invalidation)
		mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
			Return(merchant, nil).Twice()
		mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/test/mac").
			Return(secret, nil).Twice()

		// Populate cache
		_, err := cache.Get(context.Background(), merchantID)
		require.NoError(t, err)
	}

	// Act - Invalidate all
	cache.InvalidateAll()

	// Get all merchants again - should refetch
	for _, merchantID := range merchants {
		cached, err := cache.Get(context.Background(), merchantID)
		require.NoError(t, err)
		assert.NotNil(t, cached)
	}

	// Assert - Verify each merchant was fetched twice (3 merchants * 2 calls = 6 total)
	mockQueries.AssertNumberOfCalls(t, "GetMerchantByID", 6)
}

// Test_MerchantCredentialCache_LRUEviction verifies oldest entries evicted when max size exceeded
func Test_MerchantCredentialCache_LRUEviction(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
	logger := zaptest.NewLogger(t)

	// Small cache size for testing
	maxSize := 5
	cache := NewMerchantCredentialCache(
		mockQueries,
		mockSecretMgr,
		logger,
		5*time.Minute,
		maxSize,
	)

	// Create more merchants than cache size
	merchants := make([]uuid.UUID, 10)
	for i := 0; i < 10; i++ {
		merchants[i] = uuid.New()
		merchant := sqlc.Merchant{
			ID:            merchants[i],
			CustNbr:       string(rune('A' + i)),
			MacSecretPath: "secrets/merchant/test/mac",
		}
		secret := &ports.Secret{Value: "test-mac-secret"}

		mockQueries.On("GetMerchantByID", mock.Anything, merchants[i]).
			Return(merchant, nil).Maybe()
		mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/test/mac").
			Return(secret, nil).Maybe()
	}

	// Act - Add merchants to cache
	for _, merchantID := range merchants {
		_, err := cache.Get(context.Background(), merchantID)
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
	// (eviction removes extra 10% to reduce churn)
	assert.LessOrEqual(t, size, maxSize)
}

// Test_MerchantCredentialCache_ConcurrentGet verifies thread safety
func Test_MerchantCredentialCache_ConcurrentGet(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
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
		ID:            merchantID,
		CustNbr:       "9001",
		MacSecretPath: "secrets/merchant/9001/mac",
	}
	expectedSecret := &ports.Secret{Value: "test-mac-secret"}

	// Setup mock with thread-safe expectations
	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Return(expectedMerchant, nil).Once()
	mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/9001/mac").
		Return(expectedSecret, nil).Once()

	// Pre-populate cache to avoid race on first fetch
	_, err := cache.Get(context.Background(), merchantID)
	require.NoError(t, err)

	// Act - 1000 concurrent Get() calls for same merchant (all should hit cache)
	const concurrency = 1000
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errChan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			cached, err := cache.Get(context.Background(), merchantID)
			if err != nil {
				errChan <- err
				return
			}
			if cached == nil {
				errChan <- errors.New("cached credential is nil")
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

	// Verify DB/Vault called only once (first concurrent request)
	mockQueries.AssertNumberOfCalls(t, "GetMerchantByID", 1)
	mockSecretMgr.AssertNumberOfCalls(t, "GetSecret", 1)
}

// Test_MerchantCredentialCache_ConcurrentMixedOperations verifies thread safety with mixed operations
func Test_MerchantCredentialCache_ConcurrentMixedOperations(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
	logger := zaptest.NewLogger(t)

	cache := NewMerchantCredentialCache(
		mockQueries,
		mockSecretMgr,
		logger,
		5*time.Minute,
		100,
	)

	merchants := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for i, merchantID := range merchants {
		merchant := sqlc.Merchant{
			ID:            merchantID,
			CustNbr:       string(rune('A' + i)),
			MacSecretPath: "secrets/merchant/test/mac",
		}
		secret := &ports.Secret{Value: "test-mac-secret"}

		mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
			Return(merchant, nil).Maybe()
		mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/test/mac").
			Return(secret, nil).Maybe()
	}

	// Act - Mixed concurrent operations (Get, Invalidate, InvalidateAll)
	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			switch idx % 4 {
			case 0: // Get
				merchantID := merchants[idx%len(merchants)]
				_, _ = cache.Get(context.Background(), merchantID)
			case 1: // Invalidate
				merchantID := merchants[idx%len(merchants)]
				cache.Invalidate(merchantID)
			case 2: // InvalidateAll
				cache.InvalidateAll()
			case 3: // Get (another merchant)
				merchantID := merchants[(idx+1)%len(merchants)]
				_, _ = cache.Get(context.Background(), merchantID)
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

// Test_CachedCredential_GetMethods verifies thread-safe access to cached credential fields
func Test_CachedCredential_GetMethods(t *testing.T) {
	t.Parallel()

	// Arrange
	merchantID := uuid.New()
	merchant := sqlc.Merchant{
		ID:      merchantID,
		CustNbr: "9001",
	}
	macSecret := "test-mac-secret"

	cached := &CachedCredential{
		merchant:  merchant,
		macSecret: macSecret,
		cachedAt:  time.Now(),
		expiresAt: time.Now().Add(5 * time.Minute),
	}

	// Act & Assert - GetMerchant
	retrievedMerchant := cached.GetMerchant()
	assert.Equal(t, merchant.CustNbr, retrievedMerchant.CustNbr)

	// Act & Assert - GetMACSecret
	retrievedSecret := cached.GetMACSecret()
	assert.Equal(t, macSecret, retrievedSecret)

	// Act & Assert - GetBoth
	retrievedMerchant2, retrievedSecret2 := cached.GetBoth()
	assert.Equal(t, merchant.CustNbr, retrievedMerchant2.CustNbr)
	assert.Equal(t, macSecret, retrievedSecret2)
}

// Test_CachedCredential_ConcurrentAccess verifies thread safety of CachedCredential methods
func Test_CachedCredential_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	// Arrange
	merchantID := uuid.New()
	merchant := sqlc.Merchant{
		ID:      merchantID,
		CustNbr: "9001",
	}
	macSecret := "test-mac-secret"

	cached := &CachedCredential{
		merchant:  merchant,
		macSecret: macSecret,
		cachedAt:  time.Now(),
		expiresAt: time.Now().Add(5 * time.Minute),
	}

	// Act - 1000 concurrent reads
	const concurrency = 1000
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			switch idx % 3 {
			case 0:
				_ = cached.GetMerchant()
			case 1:
				_ = cached.GetMACSecret()
			case 2:
				_, _ = cached.GetBoth()
			}
		}(i)
	}

	// Wait for all operations to complete (should not panic or deadlock)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no panic or deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("Concurrent access deadlocked")
	}
}

// Test_MerchantCredentialCache_AccessTimeUpdate verifies LRU access time updates
func Test_MerchantCredentialCache_AccessTimeUpdate(t *testing.T) {
	t.Parallel()

	// Arrange
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
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
		ID:            merchantID,
		CustNbr:       "9001",
		MacSecretPath: "secrets/merchant/9001/mac",
	}
	expectedSecret := &ports.Secret{Value: "test-mac-secret"}

	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Return(expectedMerchant, nil).Once()
	mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/9001/mac").
		Return(expectedSecret, nil).Once()

	// Populate cache
	_, err := cache.Get(context.Background(), merchantID)
	require.NoError(t, err)

	// Get initial access time
	initialAccessTimeVal, ok := cache.accessTimes.Load(merchantID)
	require.True(t, ok)
	initialAccessTime := initialAccessTimeVal.(time.Time)

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Act - Access again (cache hit)
	_, err = cache.Get(context.Background(), merchantID)
	require.NoError(t, err)

	// Assert - Access time should be updated
	updatedAccessTimeVal, ok := cache.accessTimes.Load(merchantID)
	require.True(t, ok)
	updatedAccessTime := updatedAccessTimeVal.(time.Time)

	assert.True(t, updatedAccessTime.After(initialAccessTime),
		"Access time should be updated on cache hit")
}

// Benchmark_MerchantCredentialCache_CacheHit measures cache hit performance
func Benchmark_MerchantCredentialCache_CacheHit(b *testing.B) {
	// Setup
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
	logger := zaptest.NewLogger(b)

	cache := NewMerchantCredentialCache(
		mockQueries,
		mockSecretMgr,
		logger,
		5*time.Minute,
		100,
	)

	merchantID := uuid.New()
	expectedMerchant := sqlc.Merchant{
		ID:            merchantID,
		CustNbr:       "9001",
		MacSecretPath: "secrets/merchant/9001/mac",
	}
	expectedSecret := &ports.Secret{Value: "test-mac-secret"}

	mockQueries.On("GetMerchantByID", mock.Anything, merchantID).
		Return(expectedMerchant, nil).Once()
	mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/9001/mac").
		Return(expectedSecret, nil).Once()

	// Pre-populate cache
	_, _ = cache.Get(context.Background(), merchantID)

	// Reset timer to exclude setup
	b.ResetTimer()

	// Benchmark cache hits
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(context.Background(), merchantID)
	}
}

// Benchmark_MerchantCredentialCache_CacheMiss measures cache miss performance
func Benchmark_MerchantCredentialCache_CacheMiss(b *testing.B) {
	// Setup
	mockQueries := new(mocks.MockQuerier)
	mockSecretMgr := new(MockSecretManager)
	logger := zaptest.NewLogger(b)

	cache := NewMerchantCredentialCache(
		mockQueries,
		mockSecretMgr,
		logger,
		5*time.Minute,
		100,
	)

	merchantIDs := make([]uuid.UUID, b.N)
	for i := 0; i < b.N; i++ {
		merchantIDs[i] = uuid.New()
		merchant := sqlc.Merchant{
			ID:            merchantIDs[i],
			CustNbr:       string(rune('A' + (i % 26))),
			MacSecretPath: "secrets/merchant/test/mac",
		}
		secret := &ports.Secret{Value: "test-mac-secret"}

		mockQueries.On("GetMerchantByID", mock.Anything, merchantIDs[i]).
			Return(merchant, nil).Once()
		mockSecretMgr.On("GetSecret", mock.Anything, "secrets/merchant/test/mac").
			Return(secret, nil).Once()
	}

	// Reset timer to exclude setup
	b.ResetTimer()

	// Benchmark cache misses
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(context.Background(), merchantIDs[i])
	}
}
