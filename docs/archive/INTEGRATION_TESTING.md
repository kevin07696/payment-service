# Integration Testing Guide

## Overview

This document provides instructions for running the comprehensive integration tests for the payment service. The integration tests cover Browser Post flows, refund idempotency, and payment state transitions.

## Test Summary

- **Total Tests**: 24 integration tests across 3 files
- **Test Files**:
  - `tests/integration/payment/browser_post_test.go` (13 tests)
  - `tests/integration/payment/idempotency_test.go` (5 tests)
  - `tests/integration/payment/state_transition_test.go` (6 tests)
- **Coverage**: Browser Post E2E, idempotency, validation, edge cases, state transitions

## Prerequisites

### 1. Database Setup

The merchants table must exist and contain test merchant data:

```bash
# Create merchants table (if not exists)
PGPASSWORD=postgres psql -h localhost -U postgres -d payment_service <<'EOF'
CREATE TABLE IF NOT EXISTS merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(255) UNIQUE NOT NULL,
    cust_nbr VARCHAR(50) NOT NULL,
    merch_nbr VARCHAR(50) NOT NULL,
    dba_nbr VARCHAR(50) NOT NULL,
    terminal_nbr VARCHAR(50) NOT NULL,
    mac_secret_path VARCHAR(500) NOT NULL,
    environment VARCHAR(20) NOT NULL DEFAULT 'production',
    is_active BOOLEAN NOT NULL DEFAULT true,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_merchants_slug ON merchants(slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchants_environment ON merchants(environment) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchants_is_active ON merchants(is_active) WHERE deleted_at IS NULL;
EOF

# Seed test merchant data
PGPASSWORD=postgres psql -h localhost -U postgres -d payment_service < internal/db/seeds/staging/004_test_merchants.sql
```

**Verify merchant exists:**
```bash
PGPASSWORD=postgres psql -h localhost -U postgres -d payment_service -c \
  "SELECT id, slug, name FROM merchants WHERE id = '00000000-0000-0000-0000-000000000001';"
```

Expected output:
```
                  id                  |         slug          |           name
--------------------------------------+-----------------------+---------------------------
 00000000-0000-0000-0000-000000000001 | test-merchant-staging | EPX Sandbox Test Merchant
```

### 2. Service Running

The payment service must be running and accessible:

```bash
# Using podman-compose
podman-compose up -d

# Verify service is running
podman-compose ps

# Check service health
curl http://localhost:8081/health
```

Expected services:
- `payment-postgres` on port 5432 (healthy)
- `payment-server` on ports 8080-8081

### 3. Environment Variables

Set required environment variables for tests:

```bash
export SERVICE_URL="http://localhost:8081"
export EPX_MAC_STAGING="2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"  # EPX sandbox MAC (for tokenization tests)
```

**Note**: Most tests work without `EPX_MAC_STAGING`. Tests requiring EPX BRIC Storage (tokenization) are marked with `testutil.SkipIfBRICStorageUnavailable(t)` and will be skipped if the variable is not set.

### 4. Secret Manager Configuration

The service must be configured with mock secret manager for testing:

In `docker-compose.yml` or `.env`:
```env
SECRET_MANAGER=mock  # Use mock implementation (not GCP)
```

The mock secret manager should return the MAC secret when queried for path:
`payments/merchants/test-merchant-staging/mac`

## Running Tests

### Run All Integration Tests

```bash
SERVICE_URL="http://localhost:8081" \
  go test -v -tags=integration ./tests/integration/payment/... -timeout 5m
```

### Run Specific Test Suites

**Browser Post Tests Only:**
```bash
SERVICE_URL="http://localhost:8081" \
  go test -v -tags=integration ./tests/integration/payment -run TestBrowserPost -timeout 3m
```

**Idempotency Tests Only:**
```bash
SERVICE_URL="http://localhost:8081" \
  go test -v -tags=integration ./tests/integration/payment -run TestRefund -timeout 3m
```

**State Transition Tests Only:**
```bash
SERVICE_URL="http://localhost:8081" \
  go test -v -tags=integration ./tests/integration/payment -run TestStateTransition -timeout 3m
```

### Run Individual Tests

```bash
# Browser Post E2E test
SERVICE_URL="http://localhost:8081" \
  go test -v -tags=integration ./tests/integration/payment -run TestBrowserPost_EndToEnd_Success

# Refund idempotency test
SERVICE_URL="http://localhost:8081" \
  go test -v -tags=integration ./tests/integration/payment -run TestRefund_Idempotency_ClientGeneratedUUID

# Transaction uniqueness test (quick verification)
SERVICE_URL="http://localhost:8081" \
  go test -v -tags=integration ./tests/integration/payment -run TestTransactionIDUniqueness
```

## Test Descriptions

### Browser Post Tests (13 tests)

**Happy Path:**
- `TestBrowserPost_EndToEnd_Success` - Complete flow: Form → Callback → DB verification
- `TestBrowserPost_Callback_Idempotency` - Duplicate callbacks don't create duplicates
- `TestBrowserPost_Callback_DeclinedTransaction` - Declined transactions recorded
- `TestBrowserPost_Callback_GuestCheckout` - Guest checkout without customer_id

**Validation & Error Handling:**
- `TestBrowserPost_FormGeneration_ValidationErrors` - 6 validation cases
- `TestBrowserPost_Callback_MissingRequiredFields` - 4 missing field cases
- `TestBrowserPost_Callback_InvalidDataTypes` - 3 invalid data cases
- `TestBrowserPost_FormGeneration_InvalidTransactionType` - Unsupported transaction types

**Edge Cases:**
- `TestBrowserPost_Callback_DifferentDeclineCodes` - 7 decline response codes
- `TestBrowserPost_Callback_LargeAmount` - Large amounts ($999K, $1M, $0.01)
- `TestBrowserPost_Callback_SpecialCharactersInFields` - XSS/injection protection
- `TestBrowserPost_Callback_InvalidMerchantID` - Non-existent merchant handling

### Idempotency Tests (5 tests)

- `TestRefund_Idempotency_ClientGeneratedUUID` - Client-generated UUID pattern
- `TestRefund_MultipleRefundsWithDifferentUUIDs` - Multiple legitimate refunds
- `TestRefund_ExceedOriginalAmount` - Over-refunding validation
- `TestConcurrentRefunds_SameUUID` - Concurrent retry protection
- `TestTransactionIDUniqueness` - Database constraint verification

### State Transition Tests (6 tests)

- `TestStateTransition_VoidAfterCapture` - Void fails on captured transactions
- `TestStateTransition_CaptureAfterVoid` - Capture fails on voided authorizations
- `TestStateTransition_PartialCaptureValidation` - Partial capture validation
- `TestStateTransition_MultipleCaptures` - Multiple capture handling
- `TestStateTransition_RefundWithoutCapture` - Refund fails on uncaptured auth
- `TestStateTransition_FullWorkflow` - Complete Auth → Capture → Refund workflow

## Skipped Tests

Tests requiring EPX BRIC Storage API access (tokenization) are automatically skipped when `EPX_MAC_STAGING` is not set:

```go
func TestSomething(t *testing.T) {
    testutil.SkipIfBRICStorageUnavailable(t)
    // Test code that requires tokenization...
}
```

To run these tests, set the environment variable:
```bash
EPX_MAC_STAGING="your-mac-secret" \
SERVICE_URL="http://localhost:8081" \
  go test -v -tags=integration ./tests/integration/payment/...
```

## Troubleshooting

### Tests Fail with "connection refused"

**Problem**: Service is not running.

**Solution**:
```bash
podman-compose up -d
podman-compose ps  # Verify payment-server is running
```

### Tests Fail with "merchants table does not exist"

**Problem**: Database migrations not applied.

**Solution**: Follow the Database Setup steps above to create the merchants table and seed data.

### Tests Fail with "merchant not found"

**Problem**: Test merchant seed data not loaded.

**Solution**:
```bash
PGPASSWORD=postgres psql -h localhost -U postgres -d payment_service < \
  internal/db/seeds/staging/004_test_merchants.sql
```

### Tests Timeout

**Problem**: Service is slow to respond or tests are waiting too long.

**Solution**: Increase timeout or check service logs:
```bash
podman logs payment-server

# Increase timeout
go test -v -tags=integration ./tests/integration/payment/... -timeout 10m
```

### TAC Not Generated in Form Response

**Problem**: Key Exchange adapter not working or MAC secret not available.

**Solution**: Verify secret manager configuration:
```bash
# Check service logs for secret manager errors
podman logs payment-server | grep -i "secret"

# Verify SECRET_MANAGER env var
podman exec payment-server env | grep SECRET_MANAGER
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration-tests:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15-alpine
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: payment_service
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Setup database
        run: |
          PGPASSWORD=postgres psql -h localhost -U postgres -d payment_service < \
            internal/db/migrations/006_merchants.sql
          PGPASSWORD=postgres psql -h localhost -U postgres -d payment_service < \
            internal/db/seeds/staging/004_test_merchants.sql

      - name: Build service
        run: go build -o payment-server ./cmd/server

      - name: Start service
        run: |
          export SECRET_MANAGER=mock
          ./payment-server &
          sleep 5

      - name: Run integration tests
        env:
          SERVICE_URL: http://localhost:8081
        run: go test -v -tags=integration ./tests/integration/payment/... -timeout 5m
```

## Test Data

### Test Merchant

```
ID:           00000000-0000-0000-0000-000000000001
Slug:         test-merchant-staging
Environment:  test (EPX sandbox)
CUST_NBR:     9001
MERCH_NBR:    900300
DBA_NBR:      2
TERMINAL_NBR: 77
MAC:          2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
```

### Test Cards (EPX Sandbox)

```
Approved: 4111111111111111 (Visa)
Declined: 4000000000000002 (Insufficient funds)
CVV:      123
Expiry:   12/25
```

## Additional Resources

- [Refund Idempotency Pattern](REFUND_IDEMPOTENCY.md)
- [Browser Post Data Flow](BROWSER_POST_DATAFLOW.md)
- [EPX API Documentation](https://developer.north.com/)

## Support

For issues or questions:
1. Check service logs: `podman logs payment-server`
2. Verify database state: `psql` queries
3. Review test output for detailed error messages
