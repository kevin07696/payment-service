# Testing Guide

**Target Audience:** Developers writing tests for the payment service codebase
**Topic:** Testing strategy, patterns, and best practices
**Goal:** Write reliable, maintainable tests that catch bugs without being brittle

**Last Updated:** 2025-11-23

---

## Table of Contents

1. [Testing Philosophy](#1-testing-philosophy)
2. [Test Types](#2-test-types)
3. [Test Organization](#3-test-organization)
4. [Test Quality Principles](#4-test-quality-principles)
5. [Unit Testing](#5-unit-testing)
6. [Integration Testing](#6-integration-testing)
7. [Test Utilities](#7-test-utilities)
8. [Assertions](#8-assertions)
9. [Coverage & Quality](#9-coverage--quality)
10. [Running Tests](#10-running-tests)
11. [Common Patterns](#11-common-patterns)
12. [Anti-Patterns](#12-anti-patterns)

---

## 1. Testing Philosophy

### Why We Test

**Primary Goals:**
1. **Catch bugs early** - Before they reach production
2. **Enable refactoring** - Change code confidently
3. **Document behavior** - Tests show how code should work
4. **Prevent regressions** - Ensure bugs don't return

### What We Test

**Test the behavior, not the implementation:**

✅ **DO test:**
- Public API contracts
- Business logic outcomes
- Error handling behavior
- State transitions
- Integration points

❌ **DON'T test:**
- Private function internals
- Implementation details
- Framework/library behavior
- Exact log messages (unless user-facing)

### The Golden Rule

> **Test behavior, not implementation details.**

Tests should survive refactoring. If you can change the implementation without changing behavior, tests shouldn't break.

---

## 2. Test Types

### Unit Tests

**What:** Test individual functions/methods in isolation
**Where:** `internal/*/`*`_test.go`
**Dependencies:** Mocked via interfaces
**Speed:** Fast (<1ms per test)

```go
// internal/domain/transaction_test.go
func TestTransaction_IsApproved(t *testing.T) {
    tx := &Transaction{Status: TransactionStatusApproved}
    assert.True(t, tx.IsApproved())
}
```

**When to write:**
- Domain logic
- Pure functions
- Business rules
- Type methods

---

### Integration Tests

**What:** Test multiple components together with real dependencies
**Where:** `tests/integration/`*`/`*`_test.go`
**Dependencies:** Real database, real HTTP calls
**Speed:** Slow (seconds per test)
**Build Tag:** `//go:build integration`

```go
//go:build integration
// +build integration

package payment_test

func TestPayment_Sale_Integration(t *testing.T) {
    // Uses real database, real EPX staging environment
}
```

**When to write:**
- Service layer workflows
- Database operations
- External API integration
- End-to-end scenarios

---

### E2E Tests

**What:** Test complete user workflows through the system
**Where:** `tests/integration/*/workflow_test.go`
**Dependencies:** Full system (database, APIs, browser automation)
**Speed:** Very slow (10s+ per test)

```go
func TestBrowserPost_CompleteWorkflow(t *testing.T) {
    // 1. Get TAC token
    // 2. Submit payment via browser
    // 3. Verify callback received
    // 4. Check database state
}
```

**When to write:**
- Critical payment flows
- Multi-step workflows
- Browser-based payments
- Production-like scenarios

---

## 3. Test Organization

### File Structure

```
payment-service/
├── internal/
│   ├── domain/
│   │   ├── transaction.go
│   │   └── transaction_test.go          # Unit tests (same package)
│   ├── services/
│   │   └── payment/
│   │       ├── payment_service.go
│   │       └── payment_service_test.go  # Unit tests
│   └── adapters/
│       └── epx/
│           ├── server_post_adapter.go
│           └── server_post_adapter_test.go
│
└── tests/
    └── integration/
        ├── testutil/                     # Shared test utilities
        │   ├── client.go                # HTTP client helpers
        │   ├── auth_helpers.go          # JWT generation
        │   └── browser_post_automated.go
        ├── payment/
        │   ├── browser_post_workflow_test.go    # E2E workflows
        │   ├── server_post_workflow_test.go
        │   └── payment_service_critical_test.go # Critical paths
        ├── subscription/
        │   └── subscription_test.go
        └── merchant/
            └── merchant_test.go
```

### Naming Conventions

**Test Files:**
```
<package>_test.go           # Same package unit tests
<feature>_test.go           # Integration tests
<workflow>_workflow_test.go # E2E workflow tests
```

**Test Functions:**
```go
// Format: TestFunctionName_Scenario
func TestTransaction_IsApproved(t *testing.T)
func TestPaymentService_Sale_WithInvalidAmount(t *testing.T)
func TestBrowserPost_SALE_to_REFUND(t *testing.T)

// Table-driven: Descriptive test case names
tests := []struct {
    name     string
    // ...
}{
    {
        name: "approved_with_00_response_code",
    },
    {
        name: "declined_with_05_response_code",
    },
}
```

**Build Tags:**

All integration tests MUST have build tag:
```go
//go:build integration
// +build integration

package payment_test
```

This allows running unit tests separately from integration tests.

---

## 4. Test Quality Principles

### Prefer Non-Brittle Tests

**Brittle test:** Breaks when implementation changes, even though behavior is unchanged
**Robust test:** Only breaks when actual behavior changes

### When Brittle Tests Are ACCEPTABLE

#### 1. Testing Public API Contracts ✅

External-facing APIs where exact format matters:

```go
// ✅ ACCEPTABLE: Testing exact API response structure
func TestSaleAPI_ResponseFormat(t *testing.T) {
    resp := callSaleAPI()

    // Exact field names matter (ConnectRPC contract)
    assert.Equal(t, "transaction_id", getFieldName(resp))
    assert.Equal(t, "approved", resp.Status) // Exact status string
}
```

**Why acceptable:** External clients depend on exact format

---

#### 2. Testing Gateway Integration ✅

EPX expects exact field names and formats:

```go
// ✅ ACCEPTABLE: Testing exact EPX request format
func TestEPX_RequestFormat(t *testing.T) {
    req := buildEPXRequest()

    // EPX requires exact field names
    assert.Equal(t, "TRAN_TYPE", getFieldName(req, 0))
    assert.Equal(t, "CUST_NBR", getFieldName(req, 1))
    assert.Equal(t, "CCE1", req.TranType) // Exact EPX code
}
```

**Why acceptable:** External system contract must be exact

---

#### 3. Security/Compliance Requirements ✅

Authentication, encryption, regulatory compliance:

```go
// ✅ ACCEPTABLE: Testing exact JWT structure
func TestJWT_Structure(t *testing.T) {
    token := generateJWT()

    // JWT spec requires exact structure
    parts := strings.Split(token, ".")
    assert.Len(t, parts, 3, "JWT must have header.payload.signature")

    // PCI compliance: must not log card numbers
    logOutput := captureLog()
    assert.NotContains(t, logOutput, "4111111111111111")
}
```

**Why acceptable:** Security/compliance requirements are strict

---

#### 4. User-Facing Error Messages ✅

Messages shown directly to end users:

```go
// ✅ ACCEPTABLE: Testing exact user-facing error
func TestPayment_InsufficientFunds_UserMessage(t *testing.T) {
    _, err := service.Sale(ctx, insufficientFundsReq)

    // User sees this exact message - UX consistency matters
    assert.Contains(t, err.Error(), "Insufficient funds. Please use a different payment method.")
}
```

**Why acceptable:** UX consistency and clarity for users

---

#### 5. Regression Prevention ✅

Known bugs that caused production incidents:

```go
// ✅ ACCEPTABLE: Regression test for specific bug
func TestPayment_Issue123_DoubleChargeOnRetry(t *testing.T) {
    // Bug #123: Retry without idempotency key caused double charge

    req := &SaleRequest{
        IdempotencyKey: nil, // Intentionally nil
        // ...
    }

    _, err := service.Sale(ctx, req)

    // Must get exact error about missing idempotency key
    assert.ErrorIs(t, err, ErrIdempotencyKeyRequired)
}
```

**Why acceptable:** Specific bug must never return

---

### When Brittle Tests Are NOT ACCEPTABLE

#### 1. Testing Internal Implementation ❌

Internal error messages, private functions:

```go
// ❌ BAD: Testing internal error message
func TestDatabase_Connect_ErrorMessage(t *testing.T) {
    _, err := db.Connect("invalid")

    // DON'T test exact internal error text
    assert.Equal(t, err.Error(), "failed to connect to database: connection refused")

    // ✅ GOOD: Test that error occurred
    assert.Error(t, err)
    assert.Nil(t, db)
}
```

**Why bad:** Error message may improve without changing behavior

---

#### 2. Testing Framework/Library Details ❌

Database driver errors, HTTP client internals:

```go
// ❌ BAD: Testing pgx driver error message
func TestQuery_DatabaseError(t *testing.T) {
    _, err := queries.GetTransaction(ctx, "invalid-uuid")

    // DON'T assert exact pgx error text
    assert.Equal(t, err.Error(), "pq: invalid input syntax for type uuid")

    // ✅ GOOD: Test error category
    assert.Error(t, err)
    // Or if you need specificity:
    assert.ErrorContains(t, err, "invalid")
}
```

**Why bad:** Library error messages change between versions

---

#### 3. Over-Specifying Order ❌

When order doesn't matter:

```go
// ❌ BAD: Testing exact order of unordered operations
func TestGetTransactions_Order(t *testing.T) {
    txs := service.GetRecentTransactions()

    // DON'T assert exact order unless order matters
    assert.Equal(t, txs[0].ID, "tx-1")
    assert.Equal(t, txs[1].ID, "tx-2")

    // ✅ GOOD: Test that all expected items exist
    ids := extractIDs(txs)
    assert.ElementsMatch(t, ids, []string{"tx-1", "tx-2", "tx-3"})
}
```

**Why bad:** Implementation may use different data structure

---

#### 4. Testing Log Messages ❌

Unless logs are part of audit trail:

```go
// ❌ BAD: Testing exact log output
func TestPayment_Sale_Logging(t *testing.T) {
    logOutput := captureLog()
    service.Sale(ctx, req)

    // DON'T test exact log format (internal diagnostic)
    assert.Contains(t, logOutput, "Processing sale transaction for merchant_id=abc amount_cents=1000")

    // ✅ GOOD: Don't test logs unless they're compliance/audit logs
    // Just ensure behavior is correct
}
```

**Why bad:** Log format changes don't affect behavior

---

#### 5. Testing Private Method Behavior ❌

Private helpers, internal functions:

```go
// ❌ BAD: Testing private helper directly
func TestPaymentService_calculateFee(t *testing.T) {
    fee := service.calculateFee(100)
    assert.Equal(t, fee, 3)
}

// ✅ GOOD: Test through public interface
func TestPaymentService_Sale_IncludesFee(t *testing.T) {
    tx, _ := service.Sale(ctx, &SaleRequest{AmountCents: 100})

    // Fee is included in transaction metadata
    assert.Greater(t, tx.AmountCents, 100)
}
```

**Why bad:** Private method may be refactored away

---

### The Trade-Off Decision Matrix

| Scenario | Brittle OK? | Reason |
|----------|-------------|--------|
| External API contract | ✅ Yes | Clients depend on exact format |
| User-facing error message | ✅ Yes | UX consistency matters |
| Internal error message | ❌ No | May improve over time |
| EPX field names | ✅ Yes | Gateway requires exact format |
| Database driver error | ❌ No | Library implementation detail |
| JWT token structure | ✅ Yes | Security/compliance spec |
| Log message format | ❌ No | Internal diagnostic |
| PCI compliance check | ✅ Yes | Regulatory requirement |
| Private function behavior | ❌ No | Implementation detail |
| Regression test for bug | ✅ Yes | Must prevent specific failure |

**Guiding Question:**
*"If this exact detail changes, is it a breaking change for users/external systems/compliance?"*

- **Yes** → Brittle test acceptable
- **No** → Test behavior instead

---

## 5. Unit Testing

### Table-Driven Tests (Preferred)

Use table-driven tests for multiple scenarios:

```go
func TestTransaction_IsApproved(t *testing.T) {
    tests := []struct {
        name     string
        authResp *string
        expected bool
    }{
        {
            name:     "approved_with_00_response_code",
            authResp: stringPtr("00"),
            expected: true,
        },
        {
            name:     "declined_with_05_response_code",
            authResp: stringPtr("05"),
            expected: false,
        },
        {
            name:     "nil_auth_resp_not_approved",
            authResp: nil,
            expected: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tx := &Transaction{AuthResp: tt.authResp}
            assert.Equal(t, tt.expected, tx.IsApproved())
        })
    }
}
```

**Benefits:**
- Easy to add new test cases
- Clear documentation of all scenarios
- DRY (Don't Repeat Yourself)

---

### Mocking via Interfaces

Use interfaces for dependencies:

```go
// Good: Service depends on interface
type paymentService struct {
    serverPost ports.ServerPostAdapter  // Interface
}

// Test: Mock the interface
type mockServerPost struct {
    mock.Mock
}

func (m *mockServerPost) ProcessTransaction(ctx context.Context, req *ServerPostRequest) (*ServerPostResponse, error) {
    args := m.Called(ctx, req)
    return args.Get(0).(*ServerPostResponse), args.Error(1)
}

func TestPaymentService_Sale(t *testing.T) {
    mockAdapter := new(mockServerPost)
    mockAdapter.On("ProcessTransaction", mock.Anything, mock.Anything).
        Return(&ServerPostResponse{AuthResp: "00"}, nil)

    service := NewPaymentService(queries, mockAdapter, logger)

    tx, err := service.Sale(ctx, req)

    assert.NoError(t, err)
    assert.True(t, tx.IsApproved())
    mockAdapter.AssertExpectations(t)
}
```

---

### Helper Functions

Extract common setup to helpers:

```go
// Helper: Creates test transaction
func newTestTransaction(txType TransactionType, status TransactionStatus) *Transaction {
    return &Transaction{
        ID:     uuid.New().String(),
        Type:   txType,
        Status: status,
    }
}

// Helper: String pointer
func stringPtr(s string) *string {
    return &s
}

// Usage in tests
func TestTransaction_CanBeVoided(t *testing.T) {
    tx := newTestTransaction(TransactionTypeAuth, TransactionStatusApproved)
    assert.True(t, tx.CanBeVoided())
}
```

---

## 6. Integration Testing

### Build Tags

**All integration tests MUST use build tag:**

```go
//go:build integration
// +build integration

package payment_test
```

**Why:** Allows running unit tests separately (fast feedback loop)

---

### Database Integration Tests

Integration tests use real PostgreSQL:

```go
//go:build integration

func TestPaymentService_Sale_Integration(t *testing.T) {
    // Setup: Real database connection
    cfg, client := testutil.Setup(t)

    // Test: Real database operations
    tx, err := service.Sale(ctx, req)

    // Verify: Check database state
    dbTx, err := queries.GetTransaction(ctx, tx.ID)
    assert.NoError(t, err)
    assert.Equal(t, tx.ID, dbTx.ID)
}
```

**Database best practices:**
- Use transactions (rollback after test)
- Clean up test data
- Use unique identifiers (UUIDs)
- Don't depend on test execution order

---

### External API Integration Tests

Tests that call EPX staging environment:

```go
//go:build integration

func TestEPX_Sale_RealAPI(t *testing.T) {
    // Uses EPX staging environment
    adapter := epx.NewServerPostAdapter(stagingConfig, logger)

    resp, err := adapter.ProcessTransaction(ctx, &ServerPostRequest{
        TranType: "CCE1",
        Amount:   "10.00",
        // ... EPX test card
    })

    assert.NoError(t, err)
    assert.Equal(t, "00", resp.AuthResp) // EPX approved
}
```

**External API best practices:**
- Use staging/sandbox environments only
- Use test credentials
- Handle rate limits
- Use test payment methods (test cards)
- Tests may be flaky (network issues) - implement retry logic

---

### Workflow Tests

Multi-step scenarios:

```go
//go:build integration

func TestBrowserPost_SALE_to_REFUND(t *testing.T) {
    cfg, client := testutil.Setup(t)

    // Step 1: Execute SALE via Browser Post
    bricResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "50.00", callbackURL, jwtToken)
    t.Logf("✅ SALE completed: %s", bricResult.TransactionID)

    // Step 2: Execute REFUND
    refundResp := executeRefund(t, client, bricResult.TransactionID, "25.00", jwtToken)
    t.Logf("✅ REFUND completed: %s", refundResp.TransactionID)

    // Step 3: Verify final state
    tx := getTransaction(t, client, refundResp.TransactionID)
    assert.Equal(t, "approved", tx.Status)
    assert.Equal(t, int64(2500), tx.AmountCents)
}
```

---

## 7. Test Utilities

### Shared Helpers (`tests/integration/testutil/`)

**`client.go`** - HTTP client wrapper:
```go
func NewClient(baseURL string) *Client
func (c *Client) Do(method, path string, body interface{}) (*http.Response, error)
func (c *Client) SetHeader(key, value string)
```

**`auth_helpers.go`** - JWT generation:
```go
func GenerateJWT(privateKey, serviceID, merchantID string, expiry time.Duration) (string, error)
func LoadTestServices() ([]TestService, error)
```

**`browser_post_automated.go`** - Browser automation:
```go
func GetRealBRICForSaleAutomated(t *testing.T, client *Client, cfg *Config, amount, callbackURL, jwtToken string) *RealBRICResult
func GetRealBRICForAuthAutomated(t *testing.T, client *Client, cfg *Config, amount, callbackURL, jwtToken string) *RealBRICResult
```

---

### Test Fixtures

**Services fixtures** (`tests/fixtures/auth/test_services.json`):
```json
[
  {
    "service_id": "test-service-1",
    "service_name": "Test Service",
    "private_key_pem": "-----BEGIN RSA PRIVATE KEY-----\n...",
    "merchant_ids": ["00000000-0000-0000-0000-000000000001"]
  }
]
```

**Usage:**
```go
services, err := testutil.LoadTestServices()
require.NoError(t, err)
jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
```

---

## 8. Assertions

### Use testify (assert/require)

**`assert`** - Test continues on failure (multiple assertions)
**`require`** - Test stops on failure (prerequisite checks)

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestPayment_Sale(t *testing.T) {
    // Require: Stop if setup fails
    services, err := testutil.LoadTestServices()
    require.NoError(t, err, "Failed to load test services")
    require.NotEmpty(t, services, "No test services found")

    // Assert: Continue even if these fail
    tx, err := service.Sale(ctx, req)
    assert.NoError(t, err)
    assert.NotNil(t, tx)
    assert.Equal(t, "approved", tx.Status)
}
```

---

### Common Assertions

```go
// Equality
assert.Equal(t, expected, actual)
assert.NotEqual(t, unexpected, actual)

// Nil/Empty checks
assert.Nil(t, obj)
assert.NotNil(t, obj)
assert.Empty(t, slice)
assert.NotEmpty(t, slice)

// Errors
assert.NoError(t, err)
assert.Error(t, err)
assert.ErrorIs(t, err, domain.ErrInvalidAmount)
assert.ErrorContains(t, err, "invalid")

// Strings
assert.Contains(t, haystack, needle)
assert.NotContains(t, haystack, needle)

// Collections
assert.Len(t, slice, 3)
assert.ElementsMatch(t, expected, actual) // Ignore order

// Comparisons
assert.Greater(t, actual, minimum)
assert.GreaterOrEqual(t, actual, minimum)
assert.Less(t, actual, maximum)

// Booleans
assert.True(t, condition)
assert.False(t, condition)
```

---

## 9. Coverage & Quality

### Coverage Goals

**Target Coverage:**
- **Domain layer:** 90%+ (pure business logic)
- **Service layer:** 80%+ (orchestration logic)
- **Adapter layer:** 70%+ (external integration)
- **Handlers:** 70%+ (HTTP/gRPC)

**Check coverage:**
```bash
# Unit tests only
go test ./internal/... -cover

# With coverage report
go test ./internal/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

### Avoiding Flaky Tests

**Flaky test:** Sometimes passes, sometimes fails (no code changes)

**Common causes:**
1. **Time dependencies** - Use fixed time in tests
2. **Race conditions** - Use proper synchronization
3. **External services** - Mock or use stable test endpoints
4. **Test order dependency** - Tests must be independent
5. **Insufficient waits** - Async operations need proper polling

**Solutions:**

```go
// ❌ BAD: Flaky time-based test
func TestExpiry(t *testing.T) {
    token := generateToken(time.Now().Add(1 * time.Second))
    time.Sleep(2 * time.Second)
    assert.True(t, token.IsExpired())
}

// ✅ GOOD: Deterministic time
func TestExpiry(t *testing.T) {
    fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
    expiryTime := fixedTime.Add(1 * time.Hour)

    token := &Token{ExpiresAt: expiryTime}
    assert.False(t, token.IsExpiredAt(fixedTime))
    assert.True(t, token.IsExpiredAt(fixedTime.Add(2 * time.Hour)))
}
```

---

## 10. Running Tests

### Commands

```bash
# Unit tests only (fast)
go test ./internal/... -short

# Integration tests only
go test ./tests/integration/... -tags=integration -v

# All tests
go test ./... -tags=integration -v

# Specific test
go test ./tests/integration/payment/... -tags=integration -run TestBrowserPost_SALE

# With timeout
go test ./tests/integration/... -tags=integration -timeout=15m

# Coverage
go test ./internal/... -cover
go test ./internal/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

### CI/CD Integration

Tests run in GitHub Actions:

```yaml
# .github/workflows/ci-cd.yml
jobs:
  unit-tests:
    name: Unit Tests
    runs-on: ubuntu-latest
    steps:
      - name: Run unit tests
        run: go test ./internal/... -short -v

  integration-tests:
    name: Integration Tests
    needs: unit-tests
    runs-on: ubuntu-latest
    steps:
      - name: Run integration tests
        run: go test ./tests/integration/... -tags=integration -v -timeout=15m
        env:
          SERVICE_URL: http://${{ needs.deploy-staging.outputs.host }}
          EPX_MAC_STAGING: ${{ secrets.EPX_MAC_STAGING }}
```

---

## 11. Common Patterns

### Pattern 1: Table-Driven Tests

From `internal/domain/transaction_test.go`:

```go
func TestTransaction_CanBeVoided(t *testing.T) {
    tests := []struct {
        name     string
        txType   TransactionType
        status   TransactionStatus
        expected bool
    }{
        {
            name:     "approved_auth_can_be_voided",
            txType:   TransactionTypeAuth,
            status:   TransactionStatusApproved,
            expected: true,
        },
        {
            name:     "declined_auth_cannot_be_voided",
            txType:   TransactionTypeAuth,
            status:   TransactionStatusDeclined,
            expected: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tx := &Transaction{
                Type:   tt.txType,
                Status: tt.status,
            }
            assert.Equal(t, tt.expected, tx.CanBeVoided())
        })
    }
}
```

---

### Pattern 2: Workflow Testing

From `tests/integration/payment/browser_post_workflow_test.go`:

```go
func TestBrowserPost_Workflows(t *testing.T) {
    tests := []struct {
        name            string
        transactionType string
        amount          string
        workflow        []string
    }{
        {
            name:            "SALE_to_REFUND",
            transactionType: "SALE",
            amount:          "50.00",
            workflow:        []string{"SALE", "REFUND"},
        },
        {
            name:            "AUTH_CAPTURE_REFUND",
            transactionType: "AUTH",
            amount:          "50.00",
            workflow:        []string{"AUTH", "CAPTURE", "REFUND"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Execute workflow steps sequentially
            for _, operation := range tt.workflow {
                // ... execute operation
            }
        })
    }
}
```

---

### Pattern 3: Helper Functions

From `internal/domain/transaction_test.go`:

```go
// Helper function to create a pointer to a string
func stringPtr(s string) *string {
    return &s
}

// Usage
func TestTransaction_IsApproved(t *testing.T) {
    tx := &Transaction{
        AuthResp: stringPtr("00"),
    }
    assert.True(t, tx.IsApproved())
}
```

---

## 12. Anti-Patterns

### Anti-Pattern 1: Testing Implementation Details

❌ **BAD:**
```go
func TestPaymentService_Sale_CallsAdapter(t *testing.T) {
    mockAdapter := new(mockServerPost)
    service := NewPaymentService(queries, mockAdapter, logger)

    service.Sale(ctx, req)

    // DON'T verify internal calls
    mockAdapter.AssertCalled(t, "ProcessTransaction", mock.Anything, mock.Anything)
}
```

✅ **GOOD:**
```go
func TestPaymentService_Sale_ReturnsTransaction(t *testing.T) {
    mockAdapter := new(mockServerPost)
    mockAdapter.On("ProcessTransaction", mock.Anything, mock.Anything).
        Return(&ServerPostResponse{AuthResp: "00"}, nil)

    service := NewPaymentService(queries, mockAdapter, logger)

    tx, err := service.Sale(ctx, req)

    // DO verify behavior
    assert.NoError(t, err)
    assert.True(t, tx.IsApproved())
}
```

---

### Anti-Pattern 2: Brittle Assertions (When Not Needed)

❌ **BAD:**
```go
func TestDatabase_ConnectionError(t *testing.T) {
    _, err := db.Connect("invalid")

    // DON'T test exact database driver error
    assert.Equal(t, err.Error(), "pq: connection to server at \"invalid\" failed")
}
```

✅ **GOOD:**
```go
func TestDatabase_ConnectionError(t *testing.T) {
    _, err := db.Connect("invalid")

    // DO test that error occurred
    assert.Error(t, err)
    assert.Nil(t, db)
}
```

**Exception:** If this is EPX error that user sees, exact message OK.

---

### Anti-Pattern 3: Test Interdependence

❌ **BAD:**
```go
var sharedTransaction *Transaction

func TestPayment_Sale(t *testing.T) {
    tx, _ := service.Sale(ctx, req)
    sharedTransaction = tx  // DON'T share state
}

func TestPayment_Refund(t *testing.T) {
    service.Refund(ctx, sharedTransaction.ID)  // Depends on previous test!
}
```

✅ **GOOD:**
```go
func TestPayment_Sale(t *testing.T) {
    tx, _ := service.Sale(ctx, req)
    assert.NotNil(t, tx)
}

func TestPayment_Refund(t *testing.T) {
    // Create own transaction
    tx, _ := service.Sale(ctx, req)

    // Test refund
    refundTx, _ := service.Refund(ctx, tx.ID)
    assert.NotNil(t, refundTx)
}
```

---

### Anti-Pattern 4: Ignoring Errors

❌ **BAD:**
```go
func TestPayment_Sale(t *testing.T) {
    tx, _ := service.Sale(ctx, req)  // Ignoring error!
    assert.Equal(t, "approved", tx.Status)  // Will panic if tx is nil
}
```

✅ **GOOD:**
```go
func TestPayment_Sale(t *testing.T) {
    tx, err := service.Sale(ctx, req)
    require.NoError(t, err)  // Stop if error
    assert.Equal(t, "approved", tx.Status)
}
```

---

### Anti-Pattern 5: Sleep Instead of Polling

❌ **BAD:**
```go
func TestAsync_Operation(t *testing.T) {
    service.StartAsync()
    time.Sleep(5 * time.Second)  // Hope it's done?
    assert.True(t, service.IsComplete())
}
```

✅ **GOOD:**
```go
func TestAsync_Operation(t *testing.T) {
    service.StartAsync()

    // Poll with timeout
    assert.Eventually(t, func() bool {
        return service.IsComplete()
    }, 10*time.Second, 100*time.Millisecond)
}
```

**Exception:** Fixed sleep is acceptable when testing EPX staging environment with real network calls to avoid rate limiting:
```go
// ✅ ACCEPTABLE: External API integration test
func TestBrowserPost_Idempotency(t *testing.T) {
    saleResult := testutil.GetRealBRICForSaleAutomated(t, client, cfg, "50.00", callbackURL, jwtToken)
    time.Sleep(2 * time.Second)  // Allow EPX callback to complete

    resp, err := client.DoConnectRPC("payment.v1.PaymentService", "GetTransaction", ...)
}
```

---

### Anti-Pattern 6: Testing Concurrent Operations Without Proper Synchronization

❌ **BAD:**
```go
func TestConcurrent_Refunds(t *testing.T) {
    // Launch concurrent requests without waiting
    go client.Refund(req)
    go client.Refund(req)

    // Check immediately (race condition!)
    count := countRefunds()
    assert.Equal(t, 1, count)
}
```

✅ **GOOD:**
```go
func TestConcurrent_Refunds(t *testing.T) {
    var wg sync.WaitGroup
    results := make([]map[string]interface{}, 10)
    errors := make([]error, 10)

    // Launch concurrent requests with synchronization
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(index int) {
            defer wg.Done()
            resp, err := client.Refund(req)
            errors[index] = err
            results[index] = resp
        }(i)
    }

    wg.Wait()  // Wait for all to complete

    // Verify all returned same transaction (idempotency)
    for i, result := range results {
        assert.Equal(t, firstTxID, result["transactionId"])
    }
}
```

**Real example:** See `tests/integration/payment/server_post_idempotency_test.go:TestIntegration_ServerPost_Refund_IdempotencyConcurrent`

---

### Anti-Pattern 7: Not Verifying Both Success and Failure Paths

❌ **BAD:**
```go
func TestRefund_Validation(t *testing.T) {
    // Only test success
    tx, err := service.Refund(validReq)
    require.NoError(t, err)
    assert.Equal(t, "approved", tx.Status)
}
```

✅ **GOOD:**
```go
func TestRefund_Validation(t *testing.T) {
    tests := []struct {
        name               string
        capturedCents      int64
        refundedSoFarCents int64
        refundAmountCents  int64
        expectAllow        bool
        expectReason       string
    }{
        // Success cases
        {
            name:               "full refund",
            capturedCents:      10000,
            refundedSoFarCents: 0,
            refundAmountCents:  10000,
            expectAllow:        true,
        },
        // Failure cases
        {
            name:               "exceed captured by 1 cent",
            capturedCents:      10000,
            refundedSoFarCents: 0,
            refundAmountCents:  10001,
            expectAllow:        false,
            expectReason:       "exceeds remaining refundable amount",
        },
        {
            name:               "refund without capture",
            capturedCents:      0,
            refundedSoFarCents: 0,
            refundAmountCents:  5000,
            expectAllow:        false,
            expectReason:       "no captured amount to refund",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            canRefund, reason := state.CanRefund(tt.refundAmountCents)
            assert.Equal(t, tt.expectAllow, canRefund)
            if !tt.expectAllow {
                assert.Contains(t, reason, tt.expectReason)
            }
        })
    }
}
```

**Real example:** See `internal/services/payment/validation_test.go:TestRefundValidation_TableDriven`

---

### Anti-Pattern 8: Creating Over-Complicated Mocks

❌ **BAD:**
```go
// Mocking entire database with 70+ methods
type MockQuerier struct {
    mock.Mock
}

func (m *MockQuerier) GetTransaction(...) {...}
func (m *MockQuerier) CreateTransaction(...) {...}
func (m *MockQuerier) UpdateTransaction(...) {...}
// ... 67 more methods ...
```

✅ **GOOD:**
```go
// Test pure business logic without database
func TestRefundValidation_TableDriven(t *testing.T) {
    state := &GroupState{
        CapturedAmount: 10000,
        RefundedAmount: 0,
    }

    canRefund, reason := state.CanRefund(5000)
    assert.True(t, canRefund)
}

// Use integration tests for database operations
//go:build integration
func TestRefund_WithDatabase(t *testing.T) {
    // Use real PostgreSQL with testcontainers
    db := testutil.SetupTestDB(t)
    // ... test with real database ...
}
```

**When to use mocks vs integration tests:**
- **Mock:** External APIs (EPX adapter), simple interfaces (1-5 methods)
- **Integration test:** Database operations, complex workflows, state validation

**Real example:** See comment in `internal/services/payment/payment_service_test.go:94-101`

---

### Anti-Pattern 9: Type Assertions Without Checking

❌ **BAD:**
```go
func TestAPI_Response(t *testing.T) {
    var response map[string]interface{}
    json.Unmarshal(body, &response)

    // Will panic if field is missing or wrong type!
    amount := response["amount"].(string)
    status := response["status"].(string)
}
```

✅ **GOOD:**
```go
func TestAPI_Response(t *testing.T) {
    var response map[string]interface{}
    require.NoError(t, json.Unmarshal(body, &response))

    // Check type assertion with ok pattern
    amount, ok := response["amount"].(string)
    require.True(t, ok, "amount should be a string")

    // Or handle both string and numeric
    var amountStr string
    if amt, ok := response["amount"].(string); ok {
        amountStr = amt
    } else if amt, ok := response["amountCents"].(float64); ok {
        amountStr = fmt.Sprintf("%.2f", amt/100)
    }
}
```

**Real example:** See `tests/integration/payment/browser_post_idempotency_test.go:56-68`

---

### Anti-Pattern 10: Not Testing Edge Cases

❌ **BAD:**
```go
func TestTransaction_Amount(t *testing.T) {
    tx := &Transaction{AmountCents: 10000}
    assert.Equal(t, int64(10000), tx.AmountCents)
}
```

✅ **GOOD:**
```go
func TestTransactionAmountEdgeCases(t *testing.T) {
    tests := []struct {
        name                string
        transactions        []*domain.Transaction
        expectedAuthCents   int64
        expectedCapCents    int64
        expectedRefundCents int64
    }{
        {
            name: "zero amounts",
            transactions: []*domain.Transaction{
                makeTransaction("auth1", domain.TransactionTypeAuth, 0, "bric1"),
            },
            expectedAuthCents:   0,
            expectedCapCents:    0,
            expectedRefundCents: 0,
        },
        {
            name: "very small amounts",
            transactions: []*domain.Transaction{
                makeTransaction("auth1", domain.TransactionTypeAuth, 1, "bric1"),
            },
            expectedAuthCents: 1,
        },
        {
            name: "large amounts",
            transactions: []*domain.Transaction{
                makeTransaction("auth1", domain.TransactionTypeAuth, 99999999, "bric1"),
            },
            expectedAuthCents: 99999999,
        },
        {
            name: "multiple captures with rounding",
            transactions: []*domain.Transaction{
                makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric1"),
                makeTransaction("cap1", domain.TransactionTypeCapture, 3333, "bric2"),
                makeTransaction("cap2", domain.TransactionTypeCapture, 3333, "bric3"),
                makeTransaction("cap3", domain.TransactionTypeCapture, 3334, "bric4"),
            },
            expectedCapCents: 10000, // 3333 + 3333 + 3334 = 10000
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            state := ComputeGroupState(tt.transactions)
            assert.Equal(t, tt.expectedAuthCents, state.ActiveAuthAmount)
        })
    }
}
```

**Edge cases to test:**
- Zero values
- Very small values (1 cent)
- Very large values (max int64)
- Rounding errors
- Off-by-one errors
- Empty/nil values
- Boundary conditions

**Real example:** See `internal/services/payment/validation_test.go:TestTransactionAmountEdgeCases`

---

### Anti-Pattern 11: Testing Implementation Comments Instead of Behavior

❌ **BAD:**
```go
func TestPaymentService_Sale(t *testing.T) {
    // This test just documents what the code does
    service := NewPaymentService(...)

    // Call CreateTransaction
    tx, err := service.queries.CreateTransaction(...)

    // Then call EPX
    epxResp, err := service.epxAdapter.ProcessTransaction(...)

    // Then update transaction
    updated, err := service.queries.UpdateTransaction(...)
}
```

✅ **GOOD:**
```go
func TestPaymentService_Sale(t *testing.T) {
    // Test BEHAVIOR: Sale creates approved transaction with correct amount
    service := NewPaymentService(...)

    tx, err := service.Sale(ctx, &SaleRequest{
        Amount: "10.00",
        Card: validCard,
    })

    require.NoError(t, err)
    assert.Equal(t, "approved", tx.Status)
    assert.Equal(t, int64(1000), tx.AmountCents)
    assert.NotEmpty(t, tx.AuthorizationCode)
}
```

---

### Anti-Pattern 12: Hard-Coding Test Data Without Context

❌ **BAD:**
```go
func TestPayment(t *testing.T) {
    // Magic numbers with no context
    result := service.Charge("4111111111111111", "1225", "123", "10.20")
}
```

✅ **GOOD:**
```go
func TestPayment_EPXDeclineCodeHandling(t *testing.T) {
    tests := []struct {
        name         string
        cardDetails  *testutil.CardDetails
        amount       string // EPX uses last 3 digits as response code trigger
        expectStatus string
    }{
        {
            name:         "insufficient_funds_code_51",
            cardDetails:  testutil.VisaDeclineCard(),
            amount:       "1.20", // .20 → EPX code 51 (DECLINE)
            expectStatus: "TRANSACTION_STATUS_DECLINED",
        },
        {
            name:         "generic_decline_code_05",
            cardDetails:  testutil.VisaDeclineCard(),
            amount:       "1.05", // .05 → EPX code 05 (DECLINE)
            expectStatus: "TRANSACTION_STATUS_DECLINED",
        },
    }
}
```

**Use test helpers for common data:**
```go
// testutil/cards.go
func DefaultApprovalCard() *CardDetails {
    return &CardDetails{
        Number: "4111111111111111", // Standard Visa test card
        Exp:    "1225",
        CVV:    "123",
    }
}

func VisaDeclineCard() *CardDetails {
    return &CardDetails{
        Number: "4000000000000002", // Visa decline test card
        Exp:    "1225",
        CVV:    "123",
    }
}
```

**Real example:** See `tests/integration/payment/payment_service_critical_test.go:TestEPXDeclineCodeHandling`

---

## Checklist

Before committing tests:

- [ ] **Build tags:** Integration tests have `//go:build integration`
- [ ] **Table-driven:** Use for >2 similar test cases
- [ ] **Helpers:** Extract common setup to helper functions
- [ ] **Assertions:** Use `require` for prerequisites, `assert` for checks
- [ ] **Errors:** Never ignore errors in tests
- [ ] **Independence:** Tests don't depend on each other
- [ ] **Brittleness:** Only test implementation details when necessary (see section 4)
- [ ] **Names:** Descriptive test names (`TestFunction_Scenario`)
- [ ] **Coverage:** Check coverage after adding tests

---

## Related Documentation

- [STYLE_GUIDE.md](STYLE_GUIDE.md) - Code style conventions
- [TESTING_BEST_PRACTICES.md](TESTING_BEST_PRACTICES.md) - Error assertion guidelines
- [DEVELOP.md](DEVELOP.md) - Development workflow

---

**Questions?** See examples in `internal/domain/*_test.go` and `tests/integration/payment/*_test.go`