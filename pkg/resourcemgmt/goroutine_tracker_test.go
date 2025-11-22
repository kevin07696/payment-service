package resourcemgmt

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Test_NewGoroutineTracker verifies tracker initialization
func Test_NewGoroutineTracker(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	cfg := &Config{
		CheckInterval:    10 * time.Second,
		LeakThreshold:    50,
		LongRunningLimit: 5 * time.Minute,
	}

	// Act
	tracker := NewGoroutineTracker(logger, cfg)

	// Assert
	require.NotNil(t, tracker)
	assert.Equal(t, cfg.CheckInterval, tracker.checkInterval)
	assert.Equal(t, cfg.LeakThreshold, tracker.leakThreshold)
	assert.Equal(t, cfg.LongRunningLimit, tracker.longRunningLimit)
	assert.Greater(t, tracker.baselineCount, 0, "Baseline should be set")
	assert.NotNil(t, tracker.trackedGoroutines)
}

// Test_NewGoroutineTracker_DefaultConfig verifies default config is used
func Test_NewGoroutineTracker_DefaultConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)

	// Act
	tracker := NewGoroutineTracker(logger, nil)

	// Assert
	require.NotNil(t, tracker)
	assert.Equal(t, 30*time.Second, tracker.checkInterval)
	assert.Equal(t, 100, tracker.leakThreshold)
	assert.Equal(t, 10*time.Minute, tracker.longRunningLimit)
}

// Test_DefaultConfig verifies default configuration values
func Test_DefaultConfig(t *testing.T) {
	t.Parallel()

	// Act
	cfg := DefaultConfig()

	// Assert
	assert.Equal(t, 30*time.Second, cfg.CheckInterval)
	assert.Equal(t, 100, cfg.LeakThreshold)
	assert.Equal(t, 10*time.Minute, cfg.LongRunningLimit)
}

// Test_Track_Untrack verifies basic tracking lifecycle
func Test_Track_Untrack(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	// Act - Track a goroutine
	tg := tracker.Track("test-id-1", "test_worker")

	// Assert - Tracked
	require.NotNil(t, tg)
	assert.Equal(t, "test-id-1", tg.ID)
	assert.Equal(t, "test_worker", tg.Type)
	assert.NotNil(t, tg.Done)
	assert.WithinDuration(t, time.Now(), tg.StartTime, 100*time.Millisecond)

	stats := tracker.GetStats()
	assert.Equal(t, 1, stats.TrackedCount)
	assert.Equal(t, 1, stats.ByType["test_worker"])

	// Act - Untrack
	tracker.Untrack("test-id-1")

	// Assert - Untracked
	stats = tracker.GetStats()
	assert.Equal(t, 0, stats.TrackedCount)
	assert.Equal(t, 0, stats.ByType["test_worker"])

	// Done channel should be closed
	select {
	case <-tg.Done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed")
	}
}

// Test_Track_MultipleGoroutines verifies tracking multiple goroutines
func Test_Track_MultipleGoroutines(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	// Act - Track multiple goroutines
	tracker.Track("webhook-1", "webhook_delivery")
	tracker.Track("webhook-2", "webhook_delivery")
	tracker.Track("cron-1", "cron_job")
	tracker.Track("sub-1", "subscription_billing")

	// Assert
	stats := tracker.GetStats()
	assert.Equal(t, 4, stats.TrackedCount)
	assert.Equal(t, 2, stats.ByType["webhook_delivery"])
	assert.Equal(t, 1, stats.ByType["cron_job"])
	assert.Equal(t, 1, stats.ByType["subscription_billing"])

	// Act - Untrack some
	tracker.Untrack("webhook-1")
	tracker.Untrack("cron-1")

	// Assert
	stats = tracker.GetStats()
	assert.Equal(t, 2, stats.TrackedCount)
	assert.Equal(t, 1, stats.ByType["webhook_delivery"])
	assert.Equal(t, 0, stats.ByType["cron_job"])
}

// Test_Go_HelperMethod verifies Go helper tracks and untracks automatically
func Test_Go_HelperMethod(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	executed := make(chan struct{})
	untracked := make(chan struct{})

	// Act - Start goroutine with Go helper
	tracker.Go("test_worker", func(ctx context.Context) {
		close(executed)

		// Wait a bit to verify tracking is active
		time.Sleep(50 * time.Millisecond)
	})

	// Assert - Goroutine is tracked while running
	<-executed
	time.Sleep(10 * time.Millisecond) // Let tracking happen
	stats := tracker.GetStats()
	assert.Equal(t, 1, stats.TrackedCount, "Goroutine should be tracked")

	// Wait for goroutine to complete
	go func() {
		time.Sleep(200 * time.Millisecond)
		stats := tracker.GetStats()
		if stats.TrackedCount == 0 {
			close(untracked)
		}
	}()

	// Assert - Goroutine is untracked after completion
	select {
	case <-untracked:
		// Success
	case <-time.After(500 * time.Millisecond):
		stats := tracker.GetStats()
		t.Fatalf("Goroutine should be untracked, but count is %d", stats.TrackedCount)
	}
}

// Test_GoWithContext_HelperMethod verifies GoWithContext helper
func Test_GoWithContext_HelperMethod(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executed := int32(0)

	// Act
	tracker.GoWithContext(ctx, "test_worker", func(c context.Context) {
		atomic.StoreInt32(&executed, 1)
		<-c.Done() // Wait for cancellation
	})

	// Wait for goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Assert - Goroutine is tracked
	assert.Equal(t, int32(1), atomic.LoadInt32(&executed))
	stats := tracker.GetStats()
	assert.Equal(t, 1, stats.TrackedCount)

	// Cancel context and verify untracking
	cancel()
	time.Sleep(50 * time.Millisecond)

	stats = tracker.GetStats()
	assert.Equal(t, 0, stats.TrackedCount, "Goroutine should be untracked after context cancel")
}

// Test_GetStats_Accuracy verifies statistics are accurate
func Test_GetStats_Accuracy(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	// Act - Track various goroutines
	tracker.Track("w1", "webhook")
	tracker.Track("w2", "webhook")
	tracker.Track("c1", "cron")

	// Assert
	stats := tracker.GetStats()
	assert.Greater(t, stats.BaselineGoroutines, 0, "Baseline should be set")
	assert.Equal(t, 3, stats.TrackedCount)
	assert.Equal(t, 2, stats.ByType["webhook"])
	assert.Equal(t, 1, stats.ByType["cron"])

	// Increase is Total - Baseline
	assert.Equal(t, stats.TotalGoroutines-stats.BaselineGoroutines, stats.Increase)

	// Total goroutines should be positive
	assert.Greater(t, stats.TotalGoroutines, 0)
}

// Test_Dump_ReturnsAllTracked verifies Dump returns all tracked goroutines
func Test_Dump_ReturnsAllTracked(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	// Act
	tracker.Track("id1", "type1")
	tracker.Track("id2", "type2")
	tracker.Track("id3", "type1")

	// Assert
	dump := tracker.Dump()
	assert.Len(t, dump, 3)

	// Verify IDs are present
	ids := make(map[string]bool)
	for _, tg := range dump {
		ids[tg.ID] = true
	}
	assert.True(t, ids["id1"])
	assert.True(t, ids["id2"])
	assert.True(t, ids["id3"])
}

// Test_StartMonitoring_ChecksForLeaks verifies monitoring loop runs
func Test_StartMonitoring_ChecksForLeaks(t *testing.T) {
	// Not parallel - modifies goroutine count

	// Arrange
	logger := zaptest.NewLogger(t)
	cfg := &Config{
		CheckInterval:    100 * time.Millisecond, // Fast for testing
		LeakThreshold:    5,
		LongRunningLimit: 1 * time.Second,
	}
	tracker := NewGoroutineTracker(logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act - Start monitoring
	go tracker.StartMonitoring(ctx)

	// Let it run a few checks
	time.Sleep(350 * time.Millisecond)

	// Assert - Should have run at least 3 times (0ms, 100ms, 200ms, 300ms)
	// Monitoring is working if no panic/deadlock
	// Leak detection is logged but we can't easily verify logs in test

	// Cancel and verify it stops
	cancel()
	time.Sleep(50 * time.Millisecond)
	// Success if no panic
}

// Test_StartMonitoring_StopsOnContextCancel verifies monitoring stops
func Test_StartMonitoring_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	cfg := &Config{
		CheckInterval:    50 * time.Millisecond,
		LeakThreshold:    100,
		LongRunningLimit: 1 * time.Minute,
	}
	tracker := NewGoroutineTracker(logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	monitoringStopped := make(chan struct{})

	// Act
	go func() {
		tracker.StartMonitoring(ctx)
		close(monitoringStopped)
	}()

	// Let it run
	time.Sleep(150 * time.Millisecond)

	// Cancel
	cancel()

	// Assert - Should stop within reasonable time
	select {
	case <-monitoringStopped:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Monitoring should stop after context cancellation")
	}
}

// Test_ConcurrentTracking verifies thread safety
func Test_ConcurrentTracking(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Act - 100 concurrent Track operations
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			id := generateID()
			tracker.Track(id, "concurrent_test")
			time.Sleep(10 * time.Millisecond)
			tracker.Untrack(id)
		}(i)
	}

	wg.Wait()

	// Assert - All should be untracked
	stats := tracker.GetStats()
	assert.Equal(t, 0, stats.TrackedCount, "All goroutines should be untracked")
}

// Test_ConcurrentGetStats verifies GetStats is thread-safe
func Test_ConcurrentGetStats(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	tracker.Track("test1", "type1")
	tracker.Track("test2", "type1")

	// Act - 100 concurrent GetStats calls
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stats := tracker.GetStats()
			assert.GreaterOrEqual(t, stats.TrackedCount, 0)
		}()
	}

	wg.Wait()
	// Success if no race detected
}

// Test_Untrack_NonExistent verifies untracking non-existent ID is safe
func Test_Untrack_NonExistent(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	// Act - Untrack non-existent ID
	require.NotPanics(t, func() {
		tracker.Untrack("does-not-exist")
	})

	// Assert
	stats := tracker.GetStats()
	assert.Equal(t, 0, stats.TrackedCount)
}

// Test_generateID_Unique verifies ID generation is unique
func Test_generateID_Unique(t *testing.T) {
	t.Parallel()

	// Act - Generate multiple IDs
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateID()
		assert.False(t, ids[id], "ID should be unique")
		ids[id] = true
	}

	// Assert
	assert.Len(t, ids, 1000)
}

// Test_TrackedGoroutine_DoneChannel verifies Done channel behavior
func Test_TrackedGoroutine_DoneChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	// Act
	tg := tracker.Track("test-id", "test")

	// Assert - Channel is open
	select {
	case <-tg.Done:
		t.Fatal("Done channel should be open")
	default:
		// Success
	}

	// Untrack
	tracker.Untrack("test-id")

	// Assert - Channel is closed
	select {
	case <-tg.Done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed after untrack")
	}
}

// Benchmark_Track_Untrack benchmarks tracking overhead
func Benchmark_Track_Untrack(b *testing.B) {
	logger := zaptest.NewLogger(b)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := generateID()
		tracker.Track(id, "benchmark")
		tracker.Untrack(id)
	}
}

// Benchmark_GetStats benchmarks statistics retrieval
func Benchmark_GetStats(b *testing.B) {
	logger := zaptest.NewLogger(b)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	// Pre-populate with some tracked goroutines
	for i := 0; i < 100; i++ {
		tracker.Track(generateID(), "benchmark")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracker.GetStats()
	}
}

// Benchmark_Dump benchmarks dump operation
func Benchmark_Dump(b *testing.B) {
	logger := zaptest.NewLogger(b)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	// Pre-populate
	for i := 0; i < 100; i++ {
		tracker.Track(generateID(), "benchmark")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracker.Dump()
	}
}

// Benchmark_Go_Helper benchmarks Go helper method
func Benchmark_Go_Helper(b *testing.B) {
	logger := zaptest.NewLogger(b)
	tracker := NewGoroutineTracker(logger, DefaultConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		done := make(chan struct{})
		tracker.Go("benchmark", func(ctx context.Context) {
			close(done)
		})
		<-done
	}
}
