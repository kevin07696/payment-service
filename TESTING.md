# Testing Guide

This document describes the testing strategy for the payment service, with a focus on EPX integration testing.

## Table of Contents

1. [Test Structure](#test-structure)
2. [Running Tests](#running-tests)
3. [Unit Tests](#unit-tests)
4. [Integration Tests](#integration-tests)
5. [Test Coverage](#test-coverage)
6. [Best Practices](#best-practices)
7. [CI/CD Integration](#cicd-integration)

---

## Test Structure

```
internal/adapters/epx/
├── server_post_adapter.go           # Implementation
├── server_post_adapter_test.go      # Unit tests
├── integration_test.go               # Integration tests (with build tag)
└── testdata/
    └── README.md                     # Test data documentation
```

### Test Types

1. **Unit Tests** (`*_test.go`)
   - Fast, isolated tests
   - No external dependencies
   - Run on every commit
   - Test individual functions and methods

2. **Integration Tests** (`integration_test.go`)
   - Test against real EPX sandbox API
   - Use build tags: `//go:build integration`
   - Run manually or in CI for specific branches
   - Test complete transaction flows

---

## Running Tests

### Quick Test (Unit Tests Only)

```bash
# Run all unit tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test ./internal/adapters/epx

# Run with coverage
go test -cover ./...
```

### Integration Tests

Integration tests require the `integration` build tag and connect to the EPX sandbox API.

```bash
# Run integration tests
go test -tags=integration ./internal/adapters/epx

# Run with verbose output
go test -tags=integration -v ./internal/adapters/epx

# Run specific integration test
go test -tags=integration -v -run TestSaleTransaction ./internal/adapters/epx

# Run complete test suite (unit + integration)
go test -tags=integration -v ./internal/adapters/epx
```

### Test with Custom Credentials

```bash
# Set environment variables for integration tests
export EPX_TEST_CUST_NBR="your_cust_nbr"
export EPX_TEST_MERCH_NBR="your_merch_nbr"
export EPX_TEST_DBA_NBR="your_dba_nbr"
export EPX_TEST_TERMINAL_NBR="your_terminal_nbr"

# Run integration tests with custom credentials
go test -tags=integration -v ./internal/adapters/epx
```

### Skip Long-Running Tests

```bash
# Skip integration tests with -short flag
go test -short ./...
```

---

## Unit Tests

Unit tests are located in `*_test.go` files alongside the implementation.

### What Unit Tests Cover

1. **Configuration**
   - `TestDefaultServerPostConfig` - Tests configuration for sandbox/production
   - Environment-specific settings

2. **Request Building**
   - `TestBuildFormData` - Tests form data construction for all transaction types
   - Field mapping validation
   - Optional field handling

3. **Validation**
   - `TestValidateRequest` - Tests request validation logic
   - Required field checks
   - Business rule validation
   - BRIC storage special cases

4. **Response Parsing**
   - `TestParseXMLResponse` - Tests XML parsing
   - Approved transaction responses
   - Declined transaction responses
   - Error handling for malformed XML

5. **Business Logic**
   - `TestIsApprovedLogic` - Tests approval determination
   - AUTH_RESP code handling
   - Transaction state logic

### Running Unit Tests

```bash
# Run all unit tests
go test ./internal/adapters/epx

# Run specific test
go test -v -run TestBuildFormData ./internal/adapters/epx

# Run with coverage report
go test -cover ./internal/adapters/epx

# Generate detailed coverage report
go test -coverprofile=coverage.out ./internal/adapters/epx
go tool cover -html=coverage.out
```

### Example Unit Test

```go
func TestBuildFormData(t *testing.T) {
    adapter := newTestAdapter(t)

    tests := []struct {
        name     string
        request  *ports.ServerPostRequest
        validate func(t *testing.T, formData map[string][]string)
    }{
        {
            name: "sale transaction with card details",
            request: &ports.ServerPostRequest{
                CustNbr:         "9001",
                TransactionType: ports.TransactionTypeSale,
                Amount:          "10.00",
                // ... other fields
            },
            validate: func(t *testing.T, formData map[string][]string) {
                assert.Equal(t, "CCE1", formData["TRAN_TYPE"][0])
                assert.Equal(t, "10.00", formData["AMOUNT"][0])
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            formData := adapter.buildFormData(tt.request)
            tt.validate(t, formData)
        })
    }
}
```

---

## Integration Tests

Integration tests validate the complete flow against the EPX sandbox API.

### What Integration Tests Cover

1. **Individual Transaction Types**
   - `TestSaleTransaction` - Sale (CCE1)
   - `TestAuthorizationOnly` - Auth-Only (CCE2)
   - `TestBRICStorage` - BRIC Storage (CCE8)

2. **Complete Flows**
   - `TestAuthCaptureFlow` - Auth → Capture
   - `TestSaleRefundFlow` - Sale → Refund
   - `TestSaleVoidFlow` - Sale → Void
   - `TestRecurringPaymentFlow` - Sale → BRIC Storage → Recurring Payment

3. **Error Handling**
   - `TestErrorHandling_InvalidCard` - Declined transactions
   - Invalid request parameters
   - Network error scenarios

4. **Performance**
   - `TestPerformance_ResponseTime` - Response time validation
   - Timeout handling

### Test Suite Structure

Integration tests use `testify/suite` for setup/teardown:

```go
type EPXIntegrationTestSuite struct {
    suite.Suite
    adapter         ports.ServerPostAdapter
    ctx             context.Context
    testCredentials *TestCredentials
}

// SetupSuite runs once before all tests
func (s *EPXIntegrationTestSuite) SetupSuite() {
    // Initialize adapter, logger, credentials
}

// SetupTest runs before each test
func (s *EPXIntegrationTestSuite) SetupTest() {
    // Add delay to avoid rate limiting
    time.Sleep(2 * time.Second)
}
```

### Running Specific Integration Tests

```bash
# Run all integration tests
go test -tags=integration -v ./internal/adapters/epx

# Run specific test
go test -tags=integration -v -run TestSaleTransaction ./internal/adapters/epx

# Run tests matching pattern
go test -tags=integration -v -run "Test.*Flow" ./internal/adapters/epx
```

### Integration Test Output

```
=== RUN   TestRunIntegrationSuite
=== RUN   TestRunIntegrationSuite/TestSaleTransaction
    integration_test.go:123: ✅ Sale approved: AUTH_GUID=09LMQ886L2K2W11MPX1, AUTH_CODE=057579
--- PASS: TestRunIntegrationSuite (2.51s)
    --- PASS: TestRunIntegrationSuite/TestSaleTransaction (0.28s)
```

---

## Test Coverage

### Generate Coverage Report

```bash
# Generate coverage for all packages
go test -coverprofile=coverage.out ./...

# View coverage report in terminal
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html

# Open in browser
open coverage.html  # macOS
xdg-open coverage.html  # Linux
```

### Coverage Goals

- **Unit Test Coverage**: > 80%
- **Integration Test Coverage**: All transaction types
- **Critical Path Coverage**: 100% (payment processing, BRIC handling)

### View Coverage by Package

```bash
# Coverage for specific package
go test -cover ./internal/adapters/epx

# Output:
# ok      github.com/.../epx    0.123s  coverage: 85.4% of statements
```

---

## Best Practices

### 1. Test Naming

```go
// ✅ Good - descriptive names
func TestSaleTransaction_WithValidCard_ShouldSucceed(t *testing.T) {}
func TestBuildFormData_WhenMissingAmount_ShouldReturnError(t *testing.T) {}

// ❌ Bad - vague names
func TestTransaction(t *testing.T) {}
func TestError(t *testing.T) {}
```

### 2. Table-Driven Tests

```go
func TestValidateRequest(t *testing.T) {
    tests := []struct {
        name    string
        request *ports.ServerPostRequest
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid sale request",
            request: &ports.ServerPostRequest{/* ... */},
            wantErr: false,
        },
        {
            name: "missing customer number",
            request: &ports.ServerPostRequest{/* ... */},
            wantErr: true,
            errMsg:  "customer number is required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateRequest(tt.request)
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 3. Test Helpers

```go
// Create reusable test helpers
func newTestAdapter(t *testing.T) *serverPostAdapter {
    logger := zap.NewNop()
    config := DefaultServerPostConfig("sandbox")
    return NewServerPostAdapter(config, logger).(*serverPostAdapter)
}

func strPtr(s string) *string {
    return &s
}

func generateUniqueTranNbr() string {
    return fmt.Sprintf("%d", time.Now().Unix()%100000)
}
```

### 4. Use Assertions

```go
// Use testify/assert for clear error messages
assert.Equal(t, expected, actual, "should match")
assert.True(t, condition, "should be true")
assert.NoError(t, err, "should not error")
require.NotNil(t, obj, "should not be nil") // stops test on failure
```

### 5. Test Cleanup

```go
func TestExample(t *testing.T) {
    resource := setupResource()

    // Cleanup after test
    t.Cleanup(func() {
        resource.Close()
    })

    // Test code...
}
```

### 6. Parallel Tests (Unit Tests Only)

```go
func TestParallelSafe(t *testing.T) {
    t.Parallel() // Run in parallel with other tests

    // Test code...
}

// Don't use t.Parallel() for integration tests (API rate limits)
```

### 7. Subtests

```go
func TestCompleteFlow(t *testing.T) {
    t.Run("authorization", func(t *testing.T) {
        // Test authorization
    })

    t.Run("capture", func(t *testing.T) {
        // Test capture (can depend on authorization result)
    })
}
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run unit tests
        run: go test -v -cover ./...

      - name: Generate coverage
        run: go test -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out

  integration-tests:
    runs-on: ubuntu-latest
    # Only run on main branch or PRs
    if: github.ref == 'refs/heads/main' || github.event_name == 'pull_request'
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run integration tests
        run: go test -tags=integration -v ./internal/adapters/epx
        env:
          EPX_TEST_CUST_NBR: ${{ secrets.EPX_TEST_CUST_NBR }}
          EPX_TEST_MERCH_NBR: ${{ secrets.EPX_TEST_MERCH_NBR }}
          EPX_TEST_DBA_NBR: ${{ secrets.EPX_TEST_DBA_NBR }}
          EPX_TEST_TERMINAL_NBR: ${{ secrets.EPX_TEST_TERMINAL_NBR }}
```

### Makefile Commands

```makefile
# Run unit tests
test:
	go test -v -cover ./...

# Run integration tests
test-integration:
	go test -tags=integration -v ./internal/adapters/epx

# Run all tests
test-all:
	go test -tags=integration -v -cover ./...

# Generate coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run tests with race detector
test-race:
	go test -race ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...
```

---

## Troubleshooting

### Integration Tests Failing

1. **Check EPX sandbox is accessible**
   ```bash
   curl -I https://secure.epxuap.com
   ```

2. **Verify credentials**
   ```bash
   echo $EPX_TEST_CUST_NBR
   echo $EPX_TEST_MERCH_NBR
   ```

3. **Check rate limiting**
   - EPX may throttle requests
   - Tests include 2-second delays between transactions
   - Run with `-v` to see timing

4. **Test timeout**
   ```bash
   # Increase timeout for slow connections
   go test -tags=integration -timeout 10m ./internal/adapters/epx
   ```

### Coverage Not Generating

```bash
# Ensure all packages are tested
go test -coverprofile=coverage.out ./...

# Check coverage file was created
ls -lh coverage.out

# View coverage manually
go tool cover -func=coverage.out
```

### Test Hanging

```bash
# Add timeout
go test -timeout 30s ./...

# Check for deadlocks with race detector
go test -race ./...
```

---

## Quick Reference

### Common Commands

```bash
# Unit tests only
go test ./...

# Unit tests with coverage
go test -cover ./...

# Integration tests
go test -tags=integration ./internal/adapters/epx

# Specific test
go test -v -run TestSaleTransaction ./internal/adapters/epx

# With race detector
go test -race ./...

# Generate coverage HTML
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# Benchmarks
go test -bench=. ./internal/adapters/epx
```

### Test Files

- `*_test.go` - Unit tests (always run)
- `integration_test.go` - Integration tests (requires `-tags=integration`)
- `testdata/` - Test fixtures and data

### Environment Variables

- `EPX_TEST_CUST_NBR` - EPX customer number
- `EPX_TEST_MERCH_NBR` - EPX merchant number
- `EPX_TEST_DBA_NBR` - EPX DBA number
- `EPX_TEST_TERMINAL_NBR` - EPX terminal number

---

## Next Steps

1. **Run Unit Tests**: `go test ./internal/adapters/epx`
2. **Run Integration Tests**: `go test -tags=integration -v ./internal/adapters/epx`
3. **Generate Coverage**: `go test -coverprofile=coverage.out ./...`
4. **Review Coverage**: `go tool cover -html=coverage.out`

For questions or issues, see [EPX_API_REFERENCE.md](EPX_API_REFERENCE.md).
