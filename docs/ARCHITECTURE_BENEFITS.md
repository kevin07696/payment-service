# Ports & Adapters Architecture - Design Benefits

## Overview

This payment microservice implements the **Hexagonal Architecture (Ports & Adapters)** pattern with strict dependency injection through interfaces. This document explains the benefits and demonstrates the interchangeability achieved.

## Core Principle: Dependency Inversion

```
┌─────────────────────────────────────────┐
│         Domain Layer (Core)             │
│  ┌────────────────────────────────┐    │
│  │  Ports (Interfaces)            │    │
│  │  - Logger                      │    │
│  │  - HTTPClient                  │    │
│  │  - CreditCardGateway          │    │
│  │  - RecurringBillingGateway    │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
              ↑
              │ Depends on abstractions
              │
┌─────────────────────────────────────────┐
│    Infrastructure Layer (Adapters)      │
│  ┌────────────────────────────────┐    │
│  │  Implementations               │    │
│  │  - ZapLoggerAdapter           │    │
│  │  - http.Client                │    │
│  │  - CustomPayAdapter           │    │
│  │  - RecurringBillingAdapter    │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

**Key:** The domain layer defines interfaces (ports), and infrastructure provides implementations (adapters). Dependencies point inward - infrastructure depends on domain, never the reverse.

## Benefits Demonstrated

### 1. Easy Testing with Mocks

**Before (Tightly Coupled):**
```go
// Hard to test - requires real HTTP server
adapter := NewAdapter(config, url, zap.NewProduction())
```

**After (Loosely Coupled):**
```go
// Easy to test - inject mocks
mockHTTP := mocks.NewMockHTTPClient(...)
mockLogger := mocks.NewMockLogger()
adapter := NewAdapter(config, url, mockHTTP, mockLogger)

// Verify behavior
assert.Len(t, mockLogger.InfoCalls, 1)
assert.Equal(t, "expected log message", mockLogger.InfoCalls[0].Message)
```

### 2. Swappable Implementations

**Logger Interchangeability:**
```go
// Development: verbose logging
devLogger, _ := security.NewZapLoggerDevelopment()
adapter := NewCustomPayAdapter(config, url, httpClient, devLogger)

// Production: structured JSON logging
prodLogger, _ := security.NewZapLoggerProduction()
adapter := NewCustomPayAdapter(config, url, httpClient, prodLogger)

// Testing: mock logger
mockLogger := mocks.NewMockLogger()
adapter := NewCustomPayAdapter(config, url, httpClient, mockLogger)

// Custom: implement your own Logger interface
customLogger := MyCustomLogger{}
adapter := NewCustomPayAdapter(config, url, httpClient, customLogger)
```

**HTTP Client Interchangeability:**
```go
// Standard HTTP client
stdClient := &http.Client{Timeout: 30 * time.Second}
adapter := NewCustomPayAdapter(config, url, stdClient, logger)

// HTTP client with retry logic
retryClient := &RetryHTTPClient{MaxRetries: 3}
adapter := NewCustomPayAdapter(config, url, retryClient, logger)

// HTTP client with circuit breaker
cbClient := &CircuitBreakerHTTPClient{...}
adapter := NewCustomPayAdapter(config, url, cbClient, logger)

// Mock HTTP client for testing
mockClient := mocks.NewMockHTTPClient(...)
adapter := NewCustomPayAdapter(config, url, mockClient, logger)
```

### 3. No Code Changes to Add Features

Want to add request tracing? Just wrap the HTTP client:

```go
// Create a tracing HTTP client wrapper
type TracingHTTPClient struct {
	underlying ports.HTTPClient
	tracer     Tracer
}

func (t *TracingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	span := t.tracer.StartSpan("http.request")
	defer span.Finish()

	return t.underlying.Do(req)
}

// Use it without changing CustomPayAdapter code
tracingClient := &TracingHTTPClient{
	underlying: &http.Client{},
	tracer:     openTelemetryTracer,
}
adapter := NewCustomPayAdapter(config, url, tracingClient, logger)
```

Want to add metrics? Wrap the logger:

```go
type MetricsLogger struct {
	underlying ports.Logger
	metrics    MetricsCollector
}

func (m *MetricsLogger) Info(msg string, fields ...ports.Field) {
	m.metrics.IncrementCounter("log.info")
	m.underlying.Info(msg, fields...)
}

// Use it
metricsLogger := &MetricsLogger{
	underlying: zapLogger,
	metrics:    prometheus,
}
adapter := NewCustomPayAdapter(config, url, httpClient, metricsLogger)
```

### 4. Parallel Development

Different teams can work simultaneously:

```go
// Team A: Works on domain logic (defines ports)
type CreditCardGateway interface {
    Authorize(ctx context.Context, req *AuthorizeRequest) (*PaymentResult, error)
}

// Team B: Works on North adapter implementation
type CustomPayAdapter struct { ... }

// Team C: Works on Stripe adapter implementation
type StripeAdapter struct { ... }

// All teams implement the same interface - guaranteed compatibility
```

### 5. Easy Migration Between Gateways

Switch from North to Stripe without changing business logic:

```go
// Current: North Custom Pay
northAdapter := north.NewCustomPayAdapter(northConfig, url, httpClient, logger)
paymentService := services.NewPaymentService(northAdapter, repo, logger)

// Future: Migrate to Stripe
stripeAdapter := stripe.NewStripeAdapter(stripeConfig, httpClient, logger)
paymentService := services.NewPaymentService(stripeAdapter, repo, logger)

// Business logic unchanged - both implement CreditCardGateway interface
```

### 6. Testing Without External Dependencies

**Unit Test Isolation:**
```go
func TestPaymentService_ProcessPayment(t *testing.T) {
    // Mock all external dependencies
    mockGateway := mocks.NewMockGateway()
    mockRepo := mocks.NewMockRepository()
    mockLogger := mocks.NewMockLogger()

    // Test business logic in complete isolation
    service := NewPaymentService(mockGateway, mockRepo, mockLogger)

    result, err := service.ProcessPayment(ctx, request)

    // Verify interactions
    assert.NoError(t, err)
    assert.Len(t, mockGateway.AuthorizeCalls, 1)
    assert.Len(t, mockRepo.SaveCalls, 1)
}
```

### 7. Configuration Flexibility

Different configurations for different environments:

```go
// Development
func NewDevelopmentAdapter() *CustomPayAdapter {
    logger, _ := security.NewZapLoggerDevelopment() // Verbose
    httpClient := &http.Client{Timeout: 60 * time.Second} // Long timeout for debugging
    return NewCustomPayAdapter(devConfig, devURL, httpClient, logger)
}

// Production
func NewProductionAdapter() *CustomPayAdapter {
    logger, _ := security.NewZapLoggerProduction() // JSON structured
    httpClient := &http.Client{Timeout: 10 * time.Second} // Fast fail
    return NewCustomPayAdapter(prodConfig, prodURL, httpClient, logger)
}

// Testing
func NewTestAdapter() *CustomPayAdapter {
    logger := mocks.NewMockLogger() // Capture logs
    httpClient := mocks.NewMockHTTPClient(...) // Mock responses
    return NewCustomPayAdapter(testConfig, testURL, httpClient, logger)
}
```

## Implementation Guidelines

### Creating a New Port (Interface)

```go
// internal/domain/ports/my_service.go
package ports

type MyService interface {
    DoSomething(ctx context.Context, input Input) (*Output, error)
}
```

### Creating an Adapter (Implementation)

```go
// internal/adapters/myvendor/my_adapter.go
package myvendor

type MyAdapter struct {
    httpClient ports.HTTPClient
    logger     ports.Logger
    config     Config
}

// Always use dependency injection
func NewMyAdapter(config Config, httpClient ports.HTTPClient, logger ports.Logger) *MyAdapter {
    return &MyAdapter{
        config:     config,
        httpClient: httpClient,
        logger:     logger,
    }
}

// Implement the interface
func (a *MyAdapter) DoSomething(ctx context.Context, input ports.Input) (*ports.Output, error) {
    a.logger.Info("processing request", ports.String("action", "DoSomething"))

    // Use injected HTTP client
    resp, err := a.httpClient.Do(req)
    // ...
}
```

### Creating a Mock for Testing

```go
// test/mocks/mock_my_service.go
package mocks

type MockMyService struct {
    DoSomethingFunc func(ctx context.Context, input Input) (*Output, error)
    Calls           []Input
}

func (m *MockMyService) DoSomething(ctx context.Context, input Input) (*Output, error) {
    m.Calls = append(m.Calls, input)
    if m.DoSomethingFunc != nil {
        return m.DoSomethingFunc(ctx, input)
    }
    return &Output{}, nil
}
```

## Testing Strategy

### Layer 1: Port/Interface Tests
- Verify interface contracts
- Test default implementations

### Layer 2: Adapter Unit Tests
- Mock all dependencies (HTTP, Logger, etc.)
- Test adapter logic in isolation
- **Current Status:** Custom Pay Adapter has **85.7% coverage**

### Layer 3: Integration Tests
- Use real implementations
- Test actual API calls (sandbox environment)
- Verify end-to-end flows

### Layer 4: Service Tests
- Mock adapters
- Test business logic
- Verify orchestration

## Benefits Summary

| Benefit | Traditional Approach | Ports & Adapters |
|---------|---------------------|------------------|
| **Testability** | Requires real dependencies | Easy mocking |
| **Flexibility** | Code changes to swap implementations | Constructor injection only |
| **Maintainability** | Tightly coupled | Loose coupling |
| **Team Collaboration** | Sequential development | Parallel development |
| **Refactoring Safety** | Risky - affects many files | Safe - isolated changes |
| **Migration** | Rewrite business logic | Swap adapter |
| **Testing Speed** | Slow (real HTTP calls) | Fast (in-memory mocks) |
| **Code Reusability** | Low | High |

## Real-World Example: Adding Circuit Breaker

**Traditional Approach (Requires modifying adapter code):**
```go
// Modify every adapter method
func (a *Adapter) Authorize(...) {
    if a.circuitBreaker.IsOpen() {
        return ErrCircuitOpen
    }
    // ... actual logic
}
```

**Ports & Adapters (Zero changes to adapter):**
```go
// Create circuit breaker HTTP client wrapper
type CircuitBreakerHTTPClient struct {
    underlying ports.HTTPClient
    breaker    CircuitBreaker
}

func (c *CircuitBreakerHTTPClient) Do(req *http.Request) (*http.Response, error) {
    if c.breaker.IsOpen() {
        return nil, ErrCircuitOpen
    }

    resp, err := c.underlying.Do(req)
    if err != nil {
        c.breaker.RecordFailure()
    } else {
        c.breaker.RecordSuccess()
    }
    return resp, err
}

// Use it - NO CHANGES to CustomPayAdapter
cbClient := &CircuitBreakerHTTPClient{
    underlying: &http.Client{},
    breaker:    NewCircuitBreaker(),
}
adapter := NewCustomPayAdapter(config, url, cbClient, logger)
```

## Conclusion

The ports & adapters architecture provides:
- ✅ **Testability**: Easy unit testing with mocks
- ✅ **Flexibility**: Swap implementations without code changes
- ✅ **Maintainability**: Clear boundaries and responsibilities
- ✅ **Scalability**: Add features by wrapping, not modifying
- ✅ **Team Velocity**: Parallel development on interfaces
- ✅ **Future-Proofing**: Easy migration between vendors

This architecture enables rapid development while maintaining high code quality and test coverage (currently **85.7%** on adapters).
