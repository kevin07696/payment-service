# Integration Tests

This directory contains integration tests for the payment microservice that test the full stack with a real PostgreSQL database.

## Prerequisites

- PostgreSQL 15+ running locally or in Docker
- Go 1.21+

## Quick Start

### Using Docker Compose (Recommended)

The easiest way to run integration tests is to use the provided PostgreSQL container:

```bash
# Start PostgreSQL test database
docker-compose -f docker-compose.test.yml up -d

# Run integration tests
make test-integration

# Stop test database
docker-compose -f docker-compose.test.yml down
```

### Manual Setup

If you prefer to use your own PostgreSQL instance:

```bash
# Create test database
createdb payment_service_test

# Set environment variables
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=postgres
export TEST_DB_PASSWORD=postgres
export TEST_DB_NAME=payment_service_test

# Run integration tests
go test -v ./test/integration/...
```

## Test Structure

### Repository Tests (`repository_test.go`)
Tests the database layer with real PostgreSQL:
- `TestTransactionRepository_Integration` - CRUD operations for transactions
- `TestSubscriptionRepository_Integration` - CRUD operations for subscriptions

### Service Tests (`payment_service_test.go`, `subscription_service_test.go`)
Tests business logic with database persistence:
- `TestPaymentService_Integration_AuthorizeSale` - Payment operations
- `TestPaymentService_Integration_CaptureVoidRefund` - Transaction lifecycle
- `TestSubscriptionService_Integration_Lifecycle` - Subscription management
- `TestSubscriptionService_Integration_ProcessBilling` - Batch billing

## Environment Variables

Configure the test database connection:

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_DB_HOST` | `localhost` | PostgreSQL host |
| `TEST_DB_PORT` | `5432` | PostgreSQL port |
| `TEST_DB_USER` | `postgres` | Database user |
| `TEST_DB_PASSWORD` | `postgres` | Database password |
| `TEST_DB_NAME` | `payment_service_test` | Test database name |

## Running Tests

### Run all integration tests
```bash
make test-integration
```

### Run specific test suite
```bash
go test -v ./test/integration -run TestTransactionRepository
go test -v ./test/integration -run TestPaymentService
go test -v ./test/integration -run TestSubscriptionService
```

### Skip integration tests (unit tests only)
```bash
go test -short ./...
```

## Test Database Management

### Clean Database
The test setup automatically:
1. Creates tables using migrations
2. Truncates all tables before each test
3. Closes connections after tests

### Manual Cleanup
```bash
# Drop and recreate test database
dropdb payment_service_test
createdb payment_service_test
```

## CI/CD Integration

Integration tests can be run in CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Start PostgreSQL
  uses: postgres:15-alpine
  env:
    POSTGRES_DB: payment_service_test
    POSTGRES_PASSWORD: postgres

- name: Run integration tests
  run: make test-integration
  env:
    TEST_DB_HOST: localhost
    TEST_DB_PASSWORD: postgres
```

## Best Practices

1. **Test Isolation**: Each test cleans the database before running
2. **Fast Tests**: Use `testing.Short()` to skip in unit test runs
3. **Real Database**: Tests use actual PostgreSQL, not mocks
4. **Transactions**: Services use database transactions for atomicity
5. **Idempotency**: Tests verify idempotency key handling

## Troubleshooting

### Connection Refused
```
Error: failed to ping database: connection refused
```
**Solution**: Ensure PostgreSQL is running on the configured host/port

### Database Does Not Exist
```
Error: database "payment_service_test" does not exist
```
**Solution**: Create the test database: `createdb payment_service_test`

### Permission Denied
```
Error: permission denied for database
```
**Solution**: Grant proper permissions to the test user

### Slow Tests
Integration tests are slower than unit tests due to database I/O.
To run only unit tests: `go test -short ./...`

## Coverage

Check integration test coverage:
```bash
go test -cover ./test/integration/...
```

Generate coverage report:
```bash
go test -coverprofile=integration-coverage.out ./test/integration/...
go tool cover -html=integration-coverage.out
```
