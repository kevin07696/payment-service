# Local Testing Setup Guide

## Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ installed
- Make installed (optional, but recommended)

## Quick Start

```bash
# 1. Start test database
docker compose -f docker-compose.test.yml up -d

# 2. Run integration tests
make test-integration

# 3. Run server locally
make run

# 4. Stop everything
docker compose -f docker-compose.test.yml down
```

## Detailed Setup

### 1. Install Docker

**macOS:**
```bash
brew install --cask docker
```

**Linux:**
```bash
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
```

**Windows:**
Download from [https://www.docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop)

### 2. Start Test Database

The test database runs on port 5434 (different from dev database on 5432):

```bash
# Start PostgreSQL test database
docker compose -f docker-compose.test.yml up -d

# Verify it's running
docker compose -f docker-compose.test.yml ps

# View logs
docker compose -f docker-compose.test.yml logs -f postgres-test

# Check database is accessible
psql -h localhost -p 5434 -U payment_user -d payment_test
# Password: payment_pass
```

### 3. Run Integration Tests

Integration tests use the test database to verify:
- Repository CRUD operations
- Service business logic with real database
- Transaction handling
- Subscription billing workflows

```bash
# Method 1: Using Makefile
make test-integration

# Method 2: Using go test directly
go test -v ./test/integration/...

# Method 3: With coverage
make test-integration-cover

# Method 4: Run specific test
go test -v ./test/integration -run TestPaymentService_Integration_AuthorizeSale
```

**Expected Output:**
```
=== RUN   TestPaymentService_Integration_AuthorizeSale
=== RUN   TestPaymentService_Integration_AuthorizeSale/Authorize_Success
=== RUN   TestPaymentService_Integration_AuthorizeSale/Authorize_IdempotencyCheck
=== RUN   TestPaymentService_Integration_AuthorizeSale/Sale_Success
=== RUN   TestPaymentService_Integration_AuthorizeSale/Authorize_GatewayError
--- PASS: TestPaymentService_Integration_AuthorizeSale (0.15s)
    --- PASS: TestPaymentService_Integration_AuthorizeSale/Authorize_Success (0.04s)
    --- PASS: TestPaymentService_Integration_AuthorizeSale/Authorize_IdempotencyCheck (0.02s)
    --- PASS: TestPaymentService_Integration_AuthorizeSale/Sale_Success (0.05s)
    --- PASS: TestPaymentService_Integration_AuthorizeSale/Authorize_GatewayError (0.04s)
...
PASS
ok  	github.com/kevin07696/payment-service/test/integration	2.456s
```

### 4. Run All Tests (Unit + Integration)

```bash
# Run all tests
make test

# Run only unit tests (fast, no database)
make test-unit

# Run with coverage report
make test-cover
```

### 5. Start Development Server Locally

#### Option A: Using Docker Compose (Full Stack)

```bash
# Start PostgreSQL + pgAdmin
docker compose up -d

# Run migrations
make migrate-up

# Start server
make run
```

The server will start on:
- **gRPC:** `localhost:8080`
- **Metrics:** `http://localhost:9090/metrics`
- **Health:** `http://localhost:9090/health`

#### Option B: Using Existing PostgreSQL

If you have PostgreSQL running locally:

```bash
# Create database
createdb payment_service

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=your_user
export DB_PASSWORD=your_password
export DB_NAME=payment_service

# Run migrations
make migrate-up

# Start server
make run
```

### 6. Test gRPC Endpoints

Install `grpcurl` for testing:

```bash
# macOS
brew install grpcurl

# Linux
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

#### List Available Services

```bash
grpcurl -plaintext localhost:8080 list
```

**Output:**
```
grpc.health.v1.Health
grpc.reflection.v1alpha.ServerReflection
payment.v1.PaymentService
subscription.v1.SubscriptionService
```

#### List Service Methods

```bash
grpcurl -plaintext localhost:8080 list payment.v1.PaymentService
```

**Output:**
```
payment.v1.PaymentService.Authorize
payment.v1.PaymentService.Capture
payment.v1.PaymentService.GetTransaction
payment.v1.PaymentService.ListTransactions
payment.v1.PaymentService.Refund
payment.v1.PaymentService.Sale
payment.v1.PaymentService.Void
```

#### Test Authorize Endpoint

```bash
grpcurl -plaintext \
  -d '{
    "merchant_id": "MERCH-001",
    "customer_id": "CUST-12345",
    "amount": "100.00",
    "currency": "USD",
    "token": "tok_test_4111111111111111",
    "capture": true,
    "billing_info": {
      "first_name": "John",
      "last_name": "Doe",
      "email": "john@example.com",
      "phone": "555-1234",
      "address": {
        "street1": "123 Main St",
        "city": "New York",
        "state": "NY",
        "postal_code": "12345",
        "country": "US"
      }
    },
    "idempotency_key": "test-key-123",
    "metadata": {
      "order_id": "ORDER-123"
    }
  }' \
  localhost:8080 payment.v1.PaymentService/Authorize
```

**Expected Response:**
```json
{
  "id": "txn-abc123...",
  "merchantId": "MERCH-001",
  "customerId": "CUST-12345",
  "amount": "100.00",
  "currency": "USD",
  "status": "TRANSACTION_STATUS_CAPTURED",
  "type": "TRANSACTION_TYPE_SALE",
  "paymentMethodType": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "responseCode": "00",
  "message": "Approved",
  "authCode": "123456",
  "createdAt": "2025-10-20T12:34:56.789Z"
}
```

#### Test Subscription Creation

```bash
grpcurl -plaintext \
  -d '{
    "merchant_id": "MERCH-001",
    "customer_id": "CUST-12345",
    "amount": "29.99",
    "currency": "USD",
    "frequency": "BILLING_FREQUENCY_MONTHLY",
    "payment_method_token": "tok_test_pm_123",
    "start_date": "2025-10-20T00:00:00Z",
    "max_retries": 3,
    "failure_option": "FAILURE_OPTION_PAUSE",
    "idempotency_key": "sub-key-123"
  }' \
  localhost:8080 subscription.v1.SubscriptionService/CreateSubscription
```

### 7. Check Health and Metrics

```bash
# Health check
curl http://localhost:9090/health

# Expected: {"status":"healthy","database":"connected"}

# Prometheus metrics
curl http://localhost:9090/metrics
```

**Sample Metrics:**
```
# HELP payment_requests_total Total number of payment requests
# TYPE payment_requests_total counter
payment_requests_total{method="Authorize",status="success"} 42
payment_requests_total{method="Authorize",status="failure"} 3

# HELP payment_request_duration_seconds Payment request duration
# TYPE payment_request_duration_seconds histogram
payment_request_duration_seconds_bucket{method="Authorize",le="0.1"} 35
payment_request_duration_seconds_bucket{method="Authorize",le="0.5"} 42
```

### 8. Database Management

#### Connect to Test Database

```bash
# Using psql
psql -h localhost -p 5434 -U payment_user -d payment_test

# Using Docker exec
docker exec -it payment-test-db psql -U payment_user -d payment_test
```

#### View Data

```sql
-- List tables
\dt

-- View transactions
SELECT id, merchant_id, amount, status, created_at
FROM transactions
ORDER BY created_at DESC
LIMIT 10;

-- View subscriptions
SELECT id, customer_id, amount, frequency, status, next_billing_date
FROM subscriptions
WHERE status = 'active';

-- Check subscription billing
SELECT
    s.id,
    s.customer_id,
    s.amount,
    s.next_billing_date,
    s.failure_retry_count
FROM subscriptions s
WHERE s.status = 'active'
  AND s.next_billing_date <= CURRENT_DATE;
```

#### Clear Test Data

```bash
# Method 1: Drop and recreate database
docker exec payment-test-db psql -U payment_user -c "DROP DATABASE payment_test;"
docker exec payment-test-db psql -U payment_user -c "CREATE DATABASE payment_test;"
make migrate-up

# Method 2: Truncate tables
docker exec payment-test-db psql -U payment_user -d payment_test -c "
TRUNCATE transactions, subscriptions, audit_logs RESTART IDENTITY CASCADE;
"
```

### 9. Environment Variables

Create a `.env` file for local development:

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=payment_user
DB_PASSWORD=payment_pass
DB_NAME=payment_service
DB_SSL_MODE=disable
DB_MAX_CONNS=25
DB_MIN_CONNS=5

# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_METRICS_PORT=9090

# North Gateway
GATEWAY_BASE_URL=https://secure.epxuap.com
GATEWAY_USERNAME=your-epi-id
GATEWAY_EPI_KEY=your-epi-key

# Logging
LOG_LEVEL=debug
LOG_DEVELOPMENT=true
```

Load environment variables:
```bash
# Method 1: Using source
source .env

# Method 2: Using export
export $(cat .env | xargs)

# Method 3: Using direnv (recommended)
brew install direnv
echo 'eval "$(direnv hook bash)"' >> ~/.bashrc
direnv allow
```

### 10. Troubleshooting

#### Database Connection Errors

```bash
# Check database is running
docker compose -f docker-compose.test.yml ps

# Check database logs
docker compose -f docker-compose.test.yml logs postgres-test

# Restart database
docker compose -f docker-compose.test.yml restart postgres-test

# Recreate database
docker compose -f docker-compose.test.yml down -v
docker compose -f docker-compose.test.yml up -d
```

#### Port Already in Use

```bash
# Check what's using port 5434
lsof -i :5434
sudo lsof -i :5434

# Kill process
kill -9 <PID>

# Or use different port in docker-compose.test.yml
ports:
  - "5435:5432"  # Change host port
```

#### Migration Errors

```bash
# Check migration version
migrate -path internal/db/migrations -database "postgresql://payment_user:payment_pass@localhost:5434/payment_test?sslmode=disable" version

# Force version
migrate -path internal/db/migrations -database "..." force 1

# Rollback and retry
make migrate-down
make migrate-up
```

#### gRPC Connection Errors

```bash
# Check server is running
ps aux | grep payment-server

# Check port is open
nc -zv localhost 8080
telnet localhost 8080

# Enable gRPC debug logs
export GRPC_GO_LOG_VERBOSITY_LEVEL=99
export GRPC_GO_LOG_SEVERITY_LEVEL=info
make run
```

### 11. Performance Testing

#### Load Test with ghz

Install `ghz`:
```bash
go install github.com/bojand/ghz/cmd/ghz@latest
```

Run load test:
```bash
ghz \
  --insecure \
  --proto api/proto/payment/v1/payment.proto \
  --call payment.v1.PaymentService/Authorize \
  -d '{
    "merchant_id": "MERCH-001",
    "customer_id": "CUST-12345",
    "amount": "100.00",
    "currency": "USD",
    "token": "tok_test_123",
    "capture": true
  }' \
  --total 1000 \
  --concurrency 50 \
  localhost:8080
```

**Expected Output:**
```
Summary:
  Count:        1000
  Total:        2.50 s
  Slowest:      125.32 ms
  Fastest:      12.45 ms
  Average:      45.67 ms
  Requests/sec: 400.12

Status code distribution:
  [OK]   1000 responses
```

### 12. Continuous Integration (CI)

#### GitHub Actions Example

Create `.github/workflows/test.yml`:

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_USER: payment_user
          POSTGRES_PASSWORD: payment_pass
          POSTGRES_DB: payment_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5434:5432

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install dependencies
        run: go mod download

      - name: Run unit tests
        run: make test-unit

      - name: Run integration tests
        env:
          DB_HOST: localhost
          DB_PORT: 5434
          DB_USER: payment_user
          DB_PASSWORD: payment_pass
          DB_NAME: payment_test
        run: make test-integration

      - name: Upload coverage
        uses: codecov/codecov-action@v3
```

### 13. Development Workflow

Typical development workflow:

```bash
# 1. Start dependencies
docker compose -f docker-compose.test.yml up -d

# 2. Make code changes
vim internal/services/payment/payment_service.go

# 3. Run tests
make test-unit

# 4. Run integration tests if database logic changed
make test-integration

# 5. Test locally
make run

# 6. Test with grpcurl
grpcurl -plaintext -d '...' localhost:8080 payment.v1.PaymentService/Authorize

# 7. Check metrics
curl http://localhost:9090/metrics

# 8. Commit changes
git add .
git commit -m "Add feature X"
git push

# 9. Clean up
docker compose -f docker-compose.test.yml down
```

### 14. Makefile Commands Reference

```bash
# Build
make build              # Build server binary
make proto              # Generate protobuf code
make sqlc               # Generate SQLC code

# Testing
make test               # Run all tests
make test-unit          # Unit tests only (no DB)
make test-integration   # Integration tests (needs DB)
make test-cover         # Coverage report

# Database
make migrate-up         # Run migrations
make migrate-down       # Rollback migrations
make test-db-up         # Start test database
make test-db-down       # Stop test database
make test-db-logs       # View test DB logs

# Running
make run                # Run server locally
make docker-build       # Build Docker image
make docker-run         # Run in Docker

# Quality
make lint               # Run linters
make fmt                # Format code
make vet                # Run go vet
```

### 15. Next Steps

After setting up local testing:

1. **Configure North Gateway Credentials**
   - Get sandbox credentials from North
   - Update `.env` with EPI-Id and EPI-Key
   - Test with North test cards

2. **Frontend Integration**
   - Share `docs/FRONTEND_INTEGRATION.md` with frontend team
   - Set up North JavaScript SDK
   - Test tokenization flow end-to-end

3. **Monitoring Setup**
   - Set up Prometheus to scrape metrics
   - Create Grafana dashboards
   - Configure alerting rules

4. **Production Deployment**
   - Deploy to Kubernetes cluster
   - Set up production database
   - Configure secrets management
   - Enable TLS/SSL

## Additional Resources

- **Proto Documentation:** `docs/API.md`
- **Frontend Guide:** `docs/FRONTEND_INTEGRATION.md`
- **System Design:** `SYSTEM_DESIGN.md`
- **Changelog:** `CHANGELOG.md`
- **North API Docs:** Contact North for documentation
