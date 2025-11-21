# CI/CD Test Strategy - Comprehensive Guide

## Executive Summary

This document defines what tests run where in the CI/CD pipeline, why they're positioned at each stage, and the rationale behind test categorization. The strategy balances **fast feedback**, **cost efficiency**, and **comprehensive validation**.

---

## Test Categorization

### Test Pyramid for Payment Service

```
                    ┌─────────────────┐
                    │  Manual/Explore │  <-- Manual testing in staging
                    │   (Ad-hoc)      │
                    └─────────────────┘
                 ┌───────────────────────┐
                 │   E2E Tests (Future)  │  <-- Browser automation, full flows
                 │     (Expensive)       │
                 └───────────────────────┘
            ┌────────────────────────────────┐
            │    Integration Tests           │  <-- Tests against deployed service
            │  (tests/integration/*)         │
            │      ~80 test cases            │
            │    Execution: 5-15 min         │
            └────────────────────────────────┘
       ┌──────────────────────────────────────────┐
       │           Unit Tests                     │  <-- Isolated component tests
       │      (internal/*/\*_test.go)              │
       │         ~150 test cases                  │
       │       Execution: <1 min                  │
       └──────────────────────────────────────────┘
```

**Test Distribution:**
- Unit Tests: 60% of tests, <1% of execution time
- Integration Tests: 35% of tests, 80% of execution time
- E2E Tests: 5% of tests (future), 15% of execution time
- Manual Tests: Ad-hoc validation

---

## Test Categories Defined

### 1. Unit Tests

**Definition:** Tests that verify individual components in isolation without external dependencies.

**Characteristics:**
- No database connections
- No network calls
- No file system access (except test fixtures)
- Use mocks/stubs for dependencies
- Fast execution (<1ms per test)
- Deterministic (same input = same output)

**Examples in Codebase:**

```go
// internal/domain/payment_method_test.go
func TestValidateCardNumber(t *testing.T) {
    // Tests credit card number validation logic
    // No external dependencies
}

// internal/services/payment/payment_service_test.go
type MockServerPostAdapter struct {
    mock.Mock
}
// Tests payment service with mocked EPX adapter
```

**Test Files:**
```
internal/domain/
├── payment_method_test.go       # Domain model validation
├── transaction_test.go          # Transaction state machine
├── merchant_test.go             # Merchant entity logic
├── errors_test.go               # Error handling
├── subscription_test.go         # Subscription business rules
└── chargeback_test.go           # Chargeback domain logic

internal/services/
├── payment/payment_service_test.go           # Payment service with mocks
├── payment/validation_test.go                # Payment validation logic
├── payment_method/payment_method_service_test.go
├── subscription/subscription_service_test.go
└── merchant/merchant_service_test.go

internal/adapters/
├── epx/server_post_adapter_test.go    # EPX adapter logic
├── epx/server_post_error_test.go      # Error handling
└── database/postgres_test.go          # Database adapter (with mocks)

internal/handlers/
└── payment/browser_post_callback_handler_test.go
```

**Why Unit Tests:**
- Catch logic errors early
- Document expected behavior
- Enable refactoring with confidence
- Fast feedback during development

---

### 2. Integration Tests

**Definition:** Tests that verify interactions between components and external systems.

**Characteristics:**
- Require running application
- Connect to real database
- Make HTTP/gRPC calls to service
- May interact with external APIs (EPX sandbox)
- Slower execution (seconds to minutes per test)
- Environment-dependent

**Examples in Codebase:**

```go
// tests/integration/payment/payment_service_critical_test.go
//go:build integration
// +build integration

func TestEPXDeclineCodeHandling(t *testing.T) {
    cfg, client := testutil.Setup(t)  // Requires running service
    // Makes real gRPC call to deployed service
    // Service interacts with EPX sandbox
}
```

**Test Files:**
```
tests/integration/
├── auth/
│   ├── jwt_auth_test.go            # JWT authentication flow
│   ├── epx_callback_auth_test.go   # EPX callback signature validation
│   └── cron_auth_test.go           # Cron endpoint authentication
├── payment/
│   ├── payment_service_critical_test.go     # Critical payment flows
│   ├── browser_post_workflow_test.go        # Browser Post workflow
│   ├── server_post_workflow_test.go         # Server Post workflow
│   ├── browser_post_idempotency_test.go     # Idempotency handling
│   ├── server_post_idempotency_test.go
│   └── payment_ach_verification_test.go     # ACH verification flow
├── payment_method/
│   └── payment_method_test.go      # Tokenization flow
├── subscription/
│   └── recurring_billing_test.go   # Recurring billing flow
├── cron/
│   └── ach_verification_cron_test.go  # Cron job integration
├── connect/
│   └── connect_protocol_test.go    # ConnectRPC protocol validation
└── merchant/
    └── merchant_test.go            # Merchant management
```

**Why Integration Tests:**
- Verify component interactions
- Catch configuration issues
- Validate API contracts
- Test error handling with real systems
- Ensure database schema matches code

---

### 3. Smoke Tests

**Definition:** Minimal tests that verify critical functionality in production.

**Characteristics:**
- Run after production deployment
- Test only critical paths
- Fast execution (<2 minutes total)
- No destructive operations
- Read-only operations preferred

**Recommended Smoke Tests:**

```go
// tests/smoke/health_check_test.go
func TestProductionHealthCheck(t *testing.T) {
    // Verify /health endpoint responds
    // Check database connectivity
    // Verify secrets are loaded
}

// tests/smoke/payment_retrieval_test.go
func TestGetPayment(t *testing.T) {
    // Retrieve a known test payment
    // Verify response structure
    // No side effects
}

// tests/smoke/metrics_test.go
func TestMetricsEndpoint(t *testing.T) {
    // Verify Prometheus metrics are exposed
    // Check key metrics exist
}
```

**Why Smoke Tests:**
- Fast production validation
- Catch deployment issues immediately
- Minimal infrastructure impact
- Enable automated rollback decisions

---

### 4. End-to-End (E2E) Tests (Future)

**Definition:** Tests that simulate complete user journeys through the system.

**Characteristics:**
- Browser automation (Selenium, Playwright)
- Multi-service interactions
- Test from user perspective
- Very slow execution (minutes per test)
- Fragile (UI changes break tests)

**Future E2E Test Examples:**

```javascript
// tests/e2e/browser_post_flow.spec.js
test('Complete payment with Browser Post', async ({ page }) => {
  // 1. Service initiates Browser Post
  // 2. Redirect to EPX payment page
  // 3. User fills credit card form
  // 4. Submit payment
  // 5. Redirect back to callback
  // 6. Verify payment success
});
```

**When to Add E2E Tests:**
- After production launch
- When UI becomes more complex
- For critical user journeys only
- Run nightly, not on every commit

---

## Test Execution Strategy by Pipeline Stage

### Stage 1: Pull Request (Pre-Merge)

**Objective:** Fast feedback to developers

**Tests Run:**
- Unit tests only
- Code linting
- Build verification

**Duration:** 3-5 minutes

**Rationale:**
- Developers need immediate feedback
- Integration tests too slow for PR iterations
- Most bugs caught by unit tests
- Infrastructure costs too high for every PR

**Configuration:**

```yaml
# .github/workflows/pr-validation.yml
on:
  pull_request:
    branches: [main, develop]

jobs:
  test-unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Run unit tests
        run: |
          # Explicitly exclude integration tests
          go test -v -race -coverprofile=coverage.out \
            $(go list ./... | grep -v /tests/integration)

      - name: Generate coverage report
        run: |
          go tool cover -func=coverage.out
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
          echo "COVERAGE=$COVERAGE" >> $GITHUB_ENV

      - name: Enforce coverage threshold
        run: |
          # Fail if coverage drops below 70%
          COVERAGE_NUM=$(echo $COVERAGE | sed 's/%//')
          if (( $(echo "$COVERAGE_NUM < 70" | bc -l) )); then
            echo "Coverage $COVERAGE is below 70% threshold"
            exit 1
          fi
```

**Why This Strategy:**
- Fast iteration cycle encourages frequent commits
- Catches most bugs before merge
- Low cost (no infrastructure provisioning)
- Clear pass/fail criteria

---

### Stage 2: Develop Branch (Staging Deployment)

**Objective:** Comprehensive validation before production

**Tests Run:**
1. Unit tests (re-run to ensure no merge conflicts introduced bugs)
2. Build Docker image
3. Deploy to staging infrastructure
4. Integration tests against deployed service

**Duration:** 20-25 minutes

**Rationale:**
- Integration tests require real infrastructure
- Validate database migrations
- Test against real EPX sandbox
- Catch configuration issues
- Verify end-to-end workflows
- Production-like environment

**Configuration:**

```yaml
# .github/workflows/ci-cd.yml (develop branch)
on:
  push:
    branches: [develop]

jobs:
  # ... unit tests, build ...

  integration-tests:
    needs: [deploy-staging]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Run integration tests
        env:
          SERVICE_URL: http://${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}
          EPX_MAC_STAGING: ${{ secrets.EPX_MAC_STAGING }}
          # ... other EPX credentials ...
        run: |
          # Run ONLY integration tests (via build tag)
          go test ./tests/integration/... \
            -v \
            -tags=integration \
            -timeout=15m \
            -json | tee test-results.json

      - name: Parse test results
        if: always()
        run: |
          # Extract pass/fail counts
          PASS=$(grep '"Action":"pass"' test-results.json | wc -l)
          FAIL=$(grep '"Action":"fail"' test-results.json | wc -l)

          echo "Integration Tests: $PASS passed, $FAIL failed"

          if [ $FAIL -gt 0 ]; then
            echo "❌ Integration tests failed"
            exit 1
          fi
```

**Integration Test Execution Order:**

```
1. Authentication Tests (fast, foundational)
   ├── tests/integration/auth/jwt_auth_test.go
   ├── tests/integration/auth/epx_callback_auth_test.go
   └── tests/integration/auth/cron_auth_test.go

2. Core Payment Workflows (critical path)
   ├── tests/integration/payment/payment_service_critical_test.go
   ├── tests/integration/payment/server_post_workflow_test.go
   └── tests/integration/payment/browser_post_workflow_test.go

3. Advanced Payment Features
   ├── tests/integration/payment/browser_post_idempotency_test.go
   ├── tests/integration/payment/server_post_idempotency_test.go
   └── tests/integration/payment/payment_ach_verification_test.go

4. Supporting Services
   ├── tests/integration/payment_method/payment_method_test.go
   ├── tests/integration/subscription/recurring_billing_test.go
   ├── tests/integration/merchant/merchant_test.go
   └── tests/integration/cron/ach_verification_cron_test.go

5. Protocol Validation
   └── tests/integration/connect/connect_protocol_test.go
```

**Why This Strategy:**
- Integration tests only run after unit tests pass (fail fast)
- Staging infrastructure only provisioned when needed
- Tests run against production-like environment
- Catches issues before production deployment
- Comprehensive validation without blocking PRs

---

### Stage 3: Main Branch (Production Deployment)

**Objective:** Verify production deployment success

**Tests Run:**
1. Unit tests (final validation)
2. Build Docker image (same as staging)
3. Deploy to production
4. Smoke tests (minimal validation)

**Duration:** 15-20 minutes

**Rationale:**
- Integration tests already passed in staging
- Re-running full integration suite wastes time
- Smoke tests catch deployment-specific issues
- Fast rollback if smoke tests fail
- Production traffic is the ultimate test

**Configuration:**

```yaml
# .github/workflows/ci-cd.yml (main branch)
on:
  push:
    branches: [main]

jobs:
  # ... unit tests, build ...

  production-smoke-tests:
    needs: [deploy-production]
    runs-on: ubuntu-latest
    timeout-minutes: 5  # Fast timeout
    steps:
      - uses: actions/checkout@v4

      - name: Health check
        run: |
          PROD_URL="${{ secrets.PRODUCTION_URL }}"

          # Retry logic for health check
          for i in {1..10}; do
            if curl -f -s "$PROD_URL/cron/health" > /dev/null 2>&1; then
              echo "✅ Production health check passed"
              exit 0
            fi
            echo "Attempt $i/10 failed, retrying..."
            sleep 5
          done

          echo "❌ Production health check failed"
          exit 1

      - name: Verify database connectivity
        run: |
          # Simple read-only query to verify DB connection
          # Could be a dedicated /readiness endpoint

      - name: Check metrics endpoint
        run: |
          # Verify Prometheus metrics are being exported
          curl -f "$PROD_URL/metrics" | grep payment_service_info

      - name: Test critical read operation
        run: |
          # Retrieve a known test payment (read-only)
          # Verifies database schema is correct
          # No side effects

      - name: Trigger auto-rollback on failure
        if: failure()
        uses: ./.github/actions/rollback
        with:
          environment: production
```

**Why This Strategy:**
- Smoke tests are fast (2-3 minutes)
- Catch deployment configuration errors
- Verify critical paths work
- Enable automated rollback
- Don't duplicate integration tests

---

## Test Execution Matrix

| Test Category | PR | Develop (Staging) | Main (Production) | Nightly |
|--------------|-----|-------------------|-------------------|---------|
| **Unit Tests** | ✅ All | ✅ All | ✅ All | ✅ All |
| **Linting** | ✅ | ✅ | ✅ | ✅ |
| **Build Check** | ✅ | ✅ | ✅ | ✅ |
| **Integration - Auth** | ❌ | ✅ | ❌ | ✅ |
| **Integration - Payment** | ❌ | ✅ | ❌ | ✅ |
| **Integration - Subscription** | ❌ | ✅ | ❌ | ✅ |
| **Integration - Cron** | ❌ | ✅ | ❌ | ✅ |
| **Smoke Tests** | ❌ | ❌ | ✅ | ❌ |
| **E2E Tests (Future)** | ❌ | ❌ | ❌ | ✅ |
| **Performance Tests** | ❌ | ❌ | ❌ | ✅ |
| **Security Scans** | ❌ | ✅ | ✅ | ✅ |

**Legend:**
- ✅ = Always run
- ❌ = Not run
- 🔄 = Run on schedule

---

## Detailed Test Breakdown

### Unit Tests - What They Cover

**Domain Layer Tests** (`internal/domain/*_test.go`):
```go
// What: Business rule validation
// Why: Ensure domain logic is correct independent of infrastructure
// Examples:
- Payment state transitions (pending → processing → approved)
- Credit card number validation (Luhn algorithm)
- Amount validation (positive, max limits)
- Subscription billing cycle calculations
- Chargeback reason code handling
```

**Service Layer Tests** (`internal/services/*_test.go`):
```go
// What: Business logic orchestration
// Why: Test service layer with mocked dependencies
// Examples:
- Payment creation workflow (with mocked EPX adapter)
- Payment method tokenization (with mocked database)
- Subscription renewal logic (with mocked payment service)
- Idempotency key handling (with mocked cache)
```

**Adapter Tests** (`internal/adapters/*_test.go`):
```go
// What: External system interaction logic
// Why: Test adapter logic without calling real external systems
// Examples:
- EPX request/response parsing
- Database query building
- Error code mapping
- Retry logic
```

**Handler Tests** (`internal/handlers/*_test.go`):
```go
// What: HTTP/gRPC request handling
// Why: Test request validation and response formatting
// Examples:
- Request parameter validation
- Authentication middleware
- Error response formatting
- Browser Post callback parsing
```

**Execution:**
```bash
# Run all unit tests (excludes integration tests)
go test -v -race -coverprofile=coverage.out \
  $(go list ./... | grep -v /tests/integration)

# Run with coverage threshold
go test -v -race -coverprofile=coverage.out \
  $(go list ./... | grep -v /tests/integration) && \
  go tool cover -func=coverage.out | grep total | awk '{print $3}' | \
  awk '{if ($1+0 < 70) exit 1}'
```

---

### Integration Tests - What They Cover

**Authentication Tests** (`tests/integration/auth/*_test.go`):
```go
// What: End-to-end authentication flows
// Why: Verify JWT, EPX callback signatures, cron authentication
// Coverage:
- JWT token generation and validation
- Public key rotation
- EPX MAC signature validation
- Cron endpoint IP allowlist
- Service-to-service authentication
```

**Payment Workflow Tests** (`tests/integration/payment/*_test.go`):
```go
// What: Complete payment processing flows
// Why: Verify payment lifecycle with real EPX sandbox
// Coverage:
- Server Post payment flow (API-initiated)
- Browser Post payment flow (customer-initiated)
- EPX approval/decline code handling
- Payment state persistence in database
- Transaction record creation
- Idempotency key handling
- ACH verification workflow
- Refund/void operations
```

**Payment Method Tests** (`tests/integration/payment_method/*_test.go`):
```go
// What: Tokenization and vault operations
// Why: Verify EPX tokenization integration
// Coverage:
- Credit card tokenization
- ACH tokenization
- Token retrieval
- Token deletion (vault operations)
- PCI compliance (no card data stored)
```

**Subscription Tests** (`tests/integration/subscription/*_test.go`):
```go
// What: Recurring billing workflows
// Why: Verify subscription lifecycle
// Coverage:
- Subscription creation
- Initial payment processing
- Recurring charge scheduling
- Subscription cancellation
- Failed payment handling
```

**Cron Job Tests** (`tests/integration/cron/*_test.go`):
```go
// What: Scheduled job execution
// Why: Verify cron endpoints work correctly
// Coverage:
- ACH verification status sync
- Dispute sync from EPX
- Authentication on cron endpoints
```

**Protocol Tests** (`tests/integration/connect/*_test.go`):
```go
// What: ConnectRPC protocol compliance
// Why: Ensure gRPC and Connect clients can interoperate
// Coverage:
- ConnectRPC request/response handling
- Error code mapping
- Streaming support (future)
```

**Execution:**
```bash
# Run all integration tests (requires running service)
SERVICE_URL=http://localhost:8080 \
EPX_MAC_STAGING=$EPX_MAC \
go test -v -tags=integration -timeout=15m ./tests/integration/...

# Run specific integration test suite
go test -v -tags=integration ./tests/integration/payment/...

# Run single integration test
go test -v -tags=integration -run TestEPXDeclineCodeHandling \
  ./tests/integration/payment/
```

---

### Smoke Tests - What They Should Cover

**Recommended Smoke Test Suite** (to be implemented):

```go
// tests/smoke/health_test.go
func TestProductionHealth(t *testing.T) {
    // GET /cron/health
    // Expected: 200 OK, {"status": "healthy"}
}

// tests/smoke/database_test.go
func TestDatabaseConnectivity(t *testing.T) {
    // Query a lightweight table (e.g., SELECT 1)
    // Verifies database credentials and network connectivity
}

// tests/smoke/secrets_test.go
func TestSecretsLoaded(t *testing.T) {
    // Verify EPX credentials are loaded
    // Check non-sensitive configuration
    // Ensure no default/placeholder values
}

// tests/smoke/metrics_test.go
func TestMetricsExposed(t *testing.T) {
    // GET /metrics
    // Verify Prometheus metrics endpoint responds
    // Check key metrics exist (e.g., payment_service_info)
}

// tests/smoke/payment_retrieval_test.go
func TestRetrieveKnownPayment(t *testing.T) {
    // Retrieve a known test payment by ID
    // Verifies API, database, and response serialization
    // Read-only operation (no side effects)
}
```

**Execution:**
```bash
# Run smoke tests against production
PRODUCTION_URL=https://payment-service.example.com \
go test -v -tags=smoke -timeout=5m ./tests/smoke/...
```

---

## Test Parallelization Strategy

### Parallel Unit Tests

**Current:** Go runs unit tests in parallel by default

```bash
# Run with custom parallelism
go test -v -parallel=4 ./...

# Run with race detector (slower but catches concurrency bugs)
go test -v -race ./...
```

**Recommended:** Let Go auto-detect parallelism (uses GOMAXPROCS)

---

### Sequential Integration Tests

**Problem:** Integration tests often conflict when run in parallel:
- Database state conflicts
- EPX sandbox rate limits
- Port conflicts if using test containers

**Solution:** Run integration test packages sequentially

```bash
# Current approach (sequential packages)
go test -v -tags=integration -p=1 ./tests/integration/...

# Better approach: Isolate test data
go test -v -tags=integration ./tests/integration/... \
  -parallel=1  # Tests within each package run sequentially
```

**Future Improvement:** Test data isolation

```go
// Use unique identifiers per test
func TestPaymentCreation(t *testing.T) {
    testID := uuid.New().String()
    payment := createPayment(t, PaymentRequest{
        IdempotencyKey: fmt.Sprintf("test-%s", testID),
        Amount: "10.00",
    })
    // Cleanup
    defer deletePayment(t, payment.ID)
}
```

---

## Test Data Management

### Unit Test Data

**Strategy:** Embedded test fixtures

```go
// tests/fixtures/cards.go
package fixtures

func VisaApprovalCard() *CardDetails {
    return &CardDetails{
        Number: "4111111111111111",
        CVV: "999",
        Expiry: "12/25",
    }
}
```

**Why:**
- Fast (no I/O)
- Deterministic
- Version controlled
- Easy to understand

---

### Integration Test Data

**Strategy:** Ephemeral test services + API-created data

**Current Approach:**
```go
// tests/integration/testutil/setup.go
func Setup(t *testing.T) (*Config, *Client) {
    // Load test service credentials from tests/fixtures/auth/test_services.json
    services, err := LoadTestServices()
    require.NoError(t, err)

    // Create authenticated client
    client := NewAuthenticatedClient(services[0])

    return &Config{ServiceURL: os.Getenv("SERVICE_URL")}, client
}
```

**Test Services:**
```json
// tests/fixtures/auth/test_services.json
[
  {
    "id": "test-service-001",
    "name": "Integration Test Service",
    "public_key_pem": "...",
    "private_key_pem": "..."
  }
]
```

**Why:**
- Each test creates its own data (isolated)
- Cleanup happens via database migrations reset (in CI)
- Matches production data creation flow
- Tests the actual API, not test-only endpoints

**Improvement Needed:**
```go
// Add cleanup to prevent data accumulation
func TestPaymentFlow(t *testing.T) {
    cfg, client := testutil.Setup(t)

    // Create payment
    payment := createPayment(t, client)

    // Test logic...

    // Cleanup (should be added)
    t.Cleanup(func() {
        deletePayment(t, client, payment.ID)
    })
}
```

---

## Test Tagging Strategy

### Build Tags for Test Isolation

**Current Tags:**

```go
//go:build integration
// +build integration

// This test ONLY runs when -tags=integration is specified
```

**Recommended Tag Expansion:**

```go
// Unit tests (default, no tag needed)
// tests/unit/...

//go:build integration
// tests/integration/...

//go:build smoke
// tests/smoke/...

//go:build e2e
// tests/e2e/...

//go:build performance
// tests/performance/...

//go:build slow
// Long-running tests (>1 minute)
```

**Execution:**

```bash
# Unit tests only (default)
go test ./...

# Integration tests only
go test -tags=integration ./tests/integration/...

# Smoke tests only
go test -tags=smoke ./tests/smoke/...

# E2E tests only
go test -tags=e2e ./tests/e2e/...

# All tests
go test -tags="integration smoke e2e" ./tests/...
```

---

## Test Timeout Strategy

### Appropriate Timeouts by Test Type

| Test Type | Timeout | Rationale |
|-----------|---------|-----------|
| Unit tests | 5s per test | Should be nearly instant |
| Integration tests | 30s per test | Allows for network/DB latency |
| Full integration suite | 15m total | Accounts for EPX sandbox delays |
| Smoke tests | 10s per test | Production should be fast |
| Full smoke suite | 5m total | Should not delay deployments |
| E2E tests | 2m per test | Browser automation is slow |
| Performance tests | 30m total | Load testing takes time |

**Configuration:**

```bash
# Per-test timeout
go test -timeout=30s ./tests/integration/payment/...

# Suite timeout
go test -timeout=15m ./tests/integration/...
```

**Why Timeouts Matter:**
- Prevent hanging tests from blocking CI
- Catch infinite loops or deadlocks
- Force proper error handling
- Ensure fast feedback

---

## Test Coverage Strategy

### Coverage Targets

| Component | Coverage Target | Rationale |
|-----------|----------------|-----------|
| Domain layer | 90%+ | Critical business logic |
| Service layer | 80%+ | Complex orchestration |
| Handlers | 70%+ | Mostly boilerplate |
| Adapters | 70%+ | Error handling paths |
| Overall | 75%+ | Balanced coverage |

**Enforcement:**

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out

# Fail if below threshold
go tool cover -func=coverage.out | grep total | awk '{print $3}' | \
  awk '{if ($1+0 < 75) exit 1}'
```

**Exclude from Coverage:**
- Generated code (`*.pb.go`, `sqlc.go`)
- Test helpers (`testutil/*`)
- Main function (`cmd/server/main.go`)

---

## Test Maintenance Guidelines

### Flaky Test Handling

**Definition:** Test that sometimes passes, sometimes fails without code changes

**Common Causes:**
- Timing issues (race conditions)
- External dependency failures (EPX sandbox down)
- Test data conflicts (parallel execution)
- Hard-coded timestamps

**Solutions:**

```go
// Bad: Hard-coded timestamp
func TestPaymentExpiry(t *testing.T) {
    payment := Payment{
        CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
    }
    assert.False(t, payment.IsExpired())  // Fails after 2025-01-01
}

// Good: Relative timestamps
func TestPaymentExpiry(t *testing.T) {
    payment := Payment{
        CreatedAt: time.Now().Add(-48 * time.Hour),
        ExpiresAt: time.Now().Add(-1 * time.Hour),
    }
    assert.True(t, payment.IsExpired())
}

// Bad: Race condition
func TestConcurrentPayments(t *testing.T) {
    go createPayment()
    go createPayment()
    time.Sleep(100 * time.Millisecond)  // Hope they finish
    assert.Equal(t, 2, countPayments())
}

// Good: Synchronization
func TestConcurrentPayments(t *testing.T) {
    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        createPayment()
    }()
    go func() {
        defer wg.Done()
        createPayment()
    }()

    wg.Wait()
    assert.Equal(t, 2, countPayments())
}
```

---

### Test Naming Conventions

**Current:** Inconsistent naming

**Recommended Convention:**

```go
// Pattern: Test[FunctionName]_[Scenario]_[ExpectedBehavior]

// Good examples:
func TestCreatePayment_WithValidCard_ReturnsApproved(t *testing.T)
func TestCreatePayment_WithDeclinedCard_ReturnsDeclined(t *testing.T)
func TestCreatePayment_WithInvalidAmount_ReturnsError(t *testing.T)
func TestCreatePayment_WithDuplicateIdempotencyKey_ReturnsCachedResponse(t *testing.T)

// Table-driven test:
func TestCreatePayment_VariousScenarios(t *testing.T) {
    tests := []struct{
        name     string
        input    PaymentRequest
        expected PaymentStatus
    }{
        {name: "valid_card", input: validRequest(), expected: Approved},
        {name: "declined_card", input: declinedRequest(), expected: Declined},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test logic
        })
    }
}
```

---

## Test Strategy Summary

### Quick Reference

**When to Run What:**

```
┌─────────────────┬──────────────┬──────────────┬──────────────┐
│ Stage           │ Tests        │ Duration     │ Cost         │
├─────────────────┼──────────────┼──────────────┼──────────────┤
│ Local Dev       │ Unit         │ <1 min       │ Free         │
│ Pull Request    │ Unit         │ 3-5 min      │ Low          │
│ Develop Branch  │ Unit + Integ │ 20-25 min    │ Medium       │
│ Main Branch     │ Unit + Smoke │ 15-20 min    │ Low          │
│ Nightly         │ All + E2E    │ 60-90 min    │ High         │
└─────────────────┴──────────────┴──────────────┴──────────────┘
```

**Test Commands:**

```bash
# Local development
go test -v -race ./...

# Pre-commit
go test -v -race -coverprofile=coverage.out \
  $(go list ./... | grep -v /tests/integration)

# Integration (requires running service)
SERVICE_URL=http://localhost:8080 \
go test -v -tags=integration -timeout=15m ./tests/integration/...

# Smoke (production)
PRODUCTION_URL=https://payment-service.example.com \
go test -v -tags=smoke -timeout=5m ./tests/smoke/...
```

---

## Implementation Checklist

- [x] Categorize existing tests (unit vs integration)
- [x] Document test execution strategy
- [ ] Add smoke test suite (`tests/smoke/`)
- [ ] Implement test data cleanup in integration tests
- [ ] Add coverage enforcement to CI pipeline
- [ ] Create nightly test workflow
- [ ] Add performance test suite (future)
- [ ] Implement E2E tests (future)
- [ ] Set up test result dashboards
- [ ] Document flaky test procedures

---

## Future Enhancements

### Performance Testing

```bash
# Load testing with k6 or vegeta
go test -tags=performance -timeout=30m ./tests/performance/...

# Benchmark tests
go test -bench=. -benchmem ./internal/services/payment/
```

### Mutation Testing

```bash
# Install go-mutesting
go install github.com/zimmski/go-mutesting/cmd/go-mutesting@latest

# Run mutation tests to verify test quality
go-mutesting ./internal/services/payment/...
```

### Test Result Dashboards

- Integrate with GitHub Actions test reporting
- Track test duration trends
- Monitor flaky test rates
- Coverage trends over time
