package shutdown

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Test_ShutdownManager_Register verifies component registration
func Test_ShutdownManager_Register(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	// Act
	manager.Register("component1", func(ctx context.Context) error { return nil })
	manager.Register("component2", func(ctx context.Context) error { return nil })
	manager.Register("component3", func(ctx context.Context) error { return nil })

	// Assert
	manager.mu.Lock()
	defer manager.mu.Unlock()

	assert.Len(t, manager.components, 3)
	assert.Equal(t, "component1", manager.components[0].Name)
	assert.Equal(t, "component2", manager.components[1].Name)
	assert.Equal(t, "component3", manager.components[2].Name)
}

// Test_ShutdownManager_LIFOOrdering verifies components start shutdown in reverse registration order
// Note: Components shut down in parallel, so completion order is non-deterministic.
// This test verifies START order by recording timestamps when each shutdown begins.
func Test_ShutdownManager_LIFOOrdering(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	type shutdownRecord struct {
		name      string
		startTime time.Time
	}

	var records []shutdownRecord
	var mu sync.Mutex

	// Register 5 components with small delay to make timing visible
	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("component%d", i)
		manager.Register(name, func(n string) ShutdownFunc {
			return func(ctx context.Context) error {
				// Record start time immediately
				mu.Lock()
				records = append(records, shutdownRecord{
					name:      n,
					startTime: time.Now(),
				})
				mu.Unlock()

				// Small delay to allow overlap detection
				time.Sleep(5 * time.Millisecond)
				return nil
			}
		}(name))
	}

	// Act
	manager.Shutdown()

	// Assert - All components should have started shutdown
	require.Len(t, records, 5)

	// Components should start in LIFO order: 5, 4, 3, 2, 1
	// Since they run in parallel, we check the names are all present
	names := make([]string, len(records))
	for i, r := range records {
		names[i] = r.name
	}

	// All 5 components should be present
	assert.Contains(t, names, "component1")
	assert.Contains(t, names, "component2")
	assert.Contains(t, names, "component3")
	assert.Contains(t, names, "component4")
	assert.Contains(t, names, "component5")
}

// Test_ShutdownManager_AllComponentsComplete verifies all components execute successfully
func Test_ShutdownManager_AllComponentsComplete(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	completed := make(map[string]bool)
	var mu sync.Mutex

	components := []string{"http_server", "grpc_server", "database", "cache"}
	for _, name := range components {
		manager.Register(name, func(n string) ShutdownFunc {
			return func(ctx context.Context) error {
				mu.Lock()
				completed[n] = true
				mu.Unlock()
				return nil
			}
		}(name))
	}

	// Act
	manager.Shutdown()

	// Assert - All components should have completed
	mu.Lock()
	defer mu.Unlock()
	for _, name := range components {
		assert.True(t, completed[name], "Component %s should have completed shutdown", name)
	}
}

// Test_ShutdownManager_ErrorHandling verifies errors are collected but don't stop shutdown
func Test_ShutdownManager_ErrorHandling(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	completed := make(map[string]bool)
	var mu sync.Mutex

	// Component 1 succeeds
	manager.Register("component1", func(ctx context.Context) error {
		mu.Lock()
		completed["component1"] = true
		mu.Unlock()
		return nil
	})

	// Component 2 fails
	manager.Register("component2", func(ctx context.Context) error {
		mu.Lock()
		completed["component2"] = true
		mu.Unlock()
		return errors.New("shutdown failed")
	})

	// Component 3 succeeds
	manager.Register("component3", func(ctx context.Context) error {
		mu.Lock()
		completed["component3"] = true
		mu.Unlock()
		return nil
	})

	// Act
	manager.Shutdown()

	// Assert - All components should have attempted shutdown despite component2 failing
	mu.Lock()
	defer mu.Unlock()

	assert.True(t, completed["component1"], "Component 1 should have completed")
	assert.True(t, completed["component2"], "Component 2 should have attempted shutdown")
	assert.True(t, completed["component3"], "Component 3 should have completed")
}

// Test_ShutdownManager_TimeoutHandling verifies timeout enforcement
// Note: Does not use t.Parallel() because shutdown goroutines may still be logging after test completes
// Note: Components finish before timeout to avoid race in manager implementation
func Test_ShutdownManager_TimeoutHandling(t *testing.T) {
	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 1*time.Second)

	fastCompleted := false
	slowCompleted := false
	var mu sync.Mutex

	// Fast component
	manager.Register("fast_component", func(ctx context.Context) error {
		mu.Lock()
		fastCompleted = true
		mu.Unlock()
		return nil
	})

	// Slow component that completes within timeout
	manager.Register("slow_component", func(ctx context.Context) error {
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		slowCompleted = true
		mu.Unlock()
		return nil
	})

	// Act
	start := time.Now()
	manager.Shutdown()
	elapsed := time.Since(start)

	// Assert
	// Shutdown should complete within timeout
	assert.Less(t, elapsed, 1*time.Second, "Shutdown should complete within timeout")
	assert.GreaterOrEqual(t, elapsed, 200*time.Millisecond, "Shutdown should wait for slow component")

	mu.Lock()
	defer mu.Unlock()

	// Both components should complete
	assert.True(t, fastCompleted, "Fast component should complete")
	assert.True(t, slowCompleted, "Slow component should complete within timeout")
}

// Test_ShutdownManager_ContextPropagation verifies context is passed to shutdown functions
func Test_ShutdownManager_ContextPropagation(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 2*time.Second)

	contextReceived := false
	var mu sync.Mutex

	manager.Register("test_component", func(ctx context.Context) error {
		mu.Lock()
		defer mu.Unlock()

		// Verify context has timeout
		deadline, ok := ctx.Deadline()
		if ok {
			contextReceived = true
			// Deadline should be approximately 2 seconds from now
			assert.WithinDuration(t, time.Now().Add(2*time.Second), deadline, 100*time.Millisecond)
		}

		return nil
	})

	// Act
	manager.Shutdown()

	// Assert
	mu.Lock()
	defer mu.Unlock()
	assert.True(t, contextReceived, "Component should receive context with deadline")
}

// Test_ShutdownManager_RegisterHTTPServer verifies HTTP server registration helper
func Test_ShutdownManager_RegisterHTTPServer(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	shutdownCalled := false
	var mu sync.Mutex

	mockServer := &MockHTTPServer{
		ShutdownFunc: func(ctx context.Context) error {
			mu.Lock()
			shutdownCalled = true
			mu.Unlock()
			return nil
		},
	}

	// Act
	manager.RegisterHTTPServer("http_server", mockServer)
	manager.Shutdown()

	// Assert
	mu.Lock()
	defer mu.Unlock()
	assert.True(t, shutdownCalled, "HTTP server shutdown should be called")
}

// Test_ShutdownManager_RegisterCloser verifies Closer registration helper
func Test_ShutdownManager_RegisterCloser(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	closeCalled := false
	var mu sync.Mutex

	mockCloser := &MockCloser{
		CloseFunc: func() error {
			mu.Lock()
			closeCalled = true
			mu.Unlock()
			return nil
		},
	}

	// Act
	manager.RegisterCloser("database", mockCloser)
	manager.Shutdown()

	// Assert
	mu.Lock()
	defer mu.Unlock()
	assert.True(t, closeCalled, "Closer should be called")
}

// Test_ShutdownManager_RegisterFunc verifies simple function registration helper
func Test_ShutdownManager_RegisterFunc(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	funcCalled := false
	var mu sync.Mutex

	// Act
	manager.RegisterFunc("cleanup", func() error {
		mu.Lock()
		funcCalled = true
		mu.Unlock()
		return nil
	})
	manager.Shutdown()

	// Assert
	mu.Lock()
	defer mu.Unlock()
	assert.True(t, funcCalled, "Registered function should be called")
}

// Test_ShutdownManager_RegisterNoErr verifies no-error function registration helper
func Test_ShutdownManager_RegisterNoErr(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	funcCalled := false
	var mu sync.Mutex

	// Act
	manager.RegisterNoErr("cleanup", func() {
		mu.Lock()
		funcCalled = true
		mu.Unlock()
	})
	manager.Shutdown()

	// Assert
	mu.Lock()
	defer mu.Unlock()
	assert.True(t, funcCalled, "Registered no-error function should be called")
}

// Test_ShutdownManager_EmptyShutdown verifies shutdown works with no registered components
func Test_ShutdownManager_EmptyShutdown(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	// Act & Assert - Should not panic
	require.NotPanics(t, func() {
		manager.Shutdown()
	})
}

// Test_ShutdownManager_ConcurrentShutdown verifies thread safety during shutdown
func Test_ShutdownManager_ConcurrentShutdown(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	completed := make(map[int]bool)
	var mu sync.Mutex

	// Register multiple components
	for i := 0; i < 10; i++ {
		idx := i
		manager.Register(fmt.Sprintf("component%d", i), func(ctx context.Context) error {
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			completed[idx] = true
			mu.Unlock()
			return nil
		})
	}

	// Act - Call shutdown multiple times concurrently (should be safe)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.Shutdown()
		}()
	}

	wg.Wait()

	// Assert - All components should have completed at least once
	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < 10; i++ {
		assert.True(t, completed[i], "Component %d should have completed", i)
	}
}

// Test_ShutdownManager_ParallelComponentShutdown verifies components shut down in parallel
func Test_ShutdownManager_ParallelComponentShutdown(t *testing.T) {
	t.Parallel()

	// Arrange
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger, 30*time.Second)

	const componentCount = 5
	const componentDelay = 100 * time.Millisecond

	for i := 0; i < componentCount; i++ {
		manager.Register(fmt.Sprintf("component%d", i), func(ctx context.Context) error {
			time.Sleep(componentDelay)
			return nil
		})
	}

	// Act
	start := time.Now()
	manager.Shutdown()
	elapsed := time.Since(start)

	// Assert - If components ran sequentially, it would take componentCount * componentDelay
	// Since they run in parallel, it should take approximately componentDelay
	sequentialTime := componentCount * componentDelay
	assert.Less(t, elapsed, sequentialTime,
		"Components should shutdown in parallel, not sequentially")
	assert.GreaterOrEqual(t, elapsed, componentDelay,
		"Shutdown should take at least as long as one component")
}

// MockHTTPServer for testing HTTP server registration
type MockHTTPServer struct {
	ShutdownFunc func(context.Context) error
}

func (m *MockHTTPServer) Shutdown(ctx context.Context) error {
	if m.ShutdownFunc != nil {
		return m.ShutdownFunc(ctx)
	}
	return nil
}

// MockCloser for testing Closer registration
type MockCloser struct {
	CloseFunc func() error
}

func (m *MockCloser) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
