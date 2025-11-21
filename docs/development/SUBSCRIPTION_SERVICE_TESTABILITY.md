# SubscriptionService Testability Analysis

## Executive Summary

During Phase 1 of TDD refactor, comprehensive unit tests were written for `SubscriptionService`. The tests **compile successfully** but **cannot run** due to tight coupling with the database adapter.

## Files Created

- `/home/kevinlam/Documents/projects/payments/internal/services/subscription/subscription_service_test.go`
  - 700+ lines of comprehensive unit tests
  - Tests for all public methods: Create, Update, Cancel, Pause, Resume, Get, List
  - Edge cases: validation errors, business logic violations, state transitions
  - Helper functions: domain conversion, date calculations
  - Mock adapters for ServerPost and SecretManager interfaces

## Testability Issue

### Root Cause

The `subscriptionService` is tightly coupled to `*database.PostgreSQLAdapter`:

```go
type subscriptionService struct {
    db            *database.PostgreSQLAdapter  // Concrete type, not interface
    serverPost    adapterports.ServerPostAdapter
    secretManager adapterports.SecretManagerAdapter
    logger        *zap.Logger
}
```

### Problem Pattern

All database operations use methods on the concrete adapter:

```go
// Cannot be mocked - requires real database connection
pm, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)

// Cannot be mocked - requires real transaction management
err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
    dbSub, err := q.CreateSubscription(ctx, params)
    // ...
})
```

### Impact

- **Unit tests cannot run** without a real PostgreSQL database
- **Integration tests required** for basic functionality verification
- **Slower test execution** (database setup/teardown overhead)
- **Test flakiness** (database state pollution between tests)
- **Difficult to test error paths** (hard to simulate specific DB errors)

## Recommended Refactoring

### Option 1: Inject Querier Interface (Recommended)

Refactor the service to accept `sqlc.Querier` interface instead of concrete adapter:

```go
type subscriptionService struct {
    queries       sqlc.Querier  // Interface instead of concrete type
    txManager     TransactionManager  // New interface for transaction management
    serverPost    adapterports.ServerPostAdapter
    secretManager adapterports.SecretManagerAdapter
    logger        *zap.Logger
}

// New interface for transaction management
type TransactionManager interface {
    WithTx(ctx context.Context, fn func(*sqlc.Queries) error) error
}
```

**Benefits:**
- Full unit test coverage with mocks
- No database required for tests
- Easy to test error scenarios
- Follows dependency inversion principle

**Effort:** Medium (requires refactoring all methods)

### Option 2: Repository Pattern

Create a `SubscriptionRepository` interface:

```go
type SubscriptionRepository interface {
    CreateSubscription(ctx context.Context, params CreateSubscriptionParams) (*sqlc.Subscription, error)
    GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*sqlc.Subscription, error)
    // ... other methods
}

type subscriptionService struct {
    repo          SubscriptionRepository  // Inject repository instead of DB
    serverPost    adapterports.ServerPostAdapter
    secretManager adapterports.SecretManagerAdapter
    logger        *zap.Logger
}
```

**Benefits:**
- Clean separation of concerns
- Repository can handle transaction management internally
- Service focuses on business logic

**Effort:** High (requires new repository layer)

### Option 3: Keep Integration Tests

Accept that SubscriptionService requires integration tests:

- Use `testcontainers` for PostgreSQL
- Run tests against real database
- Focus on business logic testing at handler level

**Benefits:**
- No refactoring required
- Tests verify actual database behavior

**Drawbacks:**
- Slower test execution
- Requires Docker/PostgreSQL
- Harder to test error paths

## Current Test Coverage

Despite inability to run, tests provide valuable documentation:

### Test Cases Implemented

**CreateSubscription:**
- ✅ Success case with all fields
- ✅ Invalid payment method ID format
- ✅ Payment method not found
- ✅ Payment method belongs to wrong customer
- ✅ Invalid amount (zero, negative, malformed)
- ✅ Payment method inactive

**UpdateSubscription:**
- ✅ Success case (amount, interval, payment method)
- ✅ Cannot update cancelled subscription

**CancelSubscription:**
- ✅ Immediate cancellation
- ✅ Cancel at period end

**PauseSubscription:**
- ✅ Success case
- ✅ Cannot pause cancelled subscription

**ResumeSubscription:**
- ✅ Success case
- ✅ Cannot resume active subscription

**GetSubscription:**
- ✅ Success case
- ✅ Not found error

**ListCustomerSubscriptions:**
- ✅ Success case with multiple subscriptions

**Helper Functions:**
- ✅ `calculateNextBillingDate` for all interval units
- ✅ `sqlcSubscriptionToDomain` conversion

## Next Steps

1. **Immediate:** Document this limitation in TDD_REFACTOR_PLAN.md
2. **Short-term:** Decide on refactoring approach (Option 1 recommended)
3. **Long-term:** Apply same refactoring to PaymentService, PaymentMethodService

## Integration Test Strategy (Temporary)

Until refactoring is complete, use integration tests:

```go
// +build integration

func TestSubscriptionService_Integration(t *testing.T) {
    // Use testcontainers to spin up PostgreSQL
    container := setupPostgresContainer(t)
    defer container.Terminate(context.Background())

    // Create real service with real DB
    service := setupRealSubscriptionService(t, container)

    // Run tests against real database
}
```

## Lessons Learned

1. **TDD reveals coupling issues early** - Writing tests first would have caught this design issue
2. **Dependency injection is critical** - Services should depend on interfaces, not concrete types
3. **Transaction management needs abstraction** - `WithTx` pattern requires interface wrapper for testing

## References

- Original service: `internal/services/subscription/subscription_service.go`
- Test file: `internal/services/subscription/subscription_service_test.go`
- Similar pattern in: `internal/services/payment/payment_service.go`
