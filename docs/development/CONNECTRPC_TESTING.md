# ConnectRPC Testing Guide

This guide explains how to test the ConnectRPC migration and verify that all supported protocols work correctly.

## Overview

The ConnectRPC server supports 4 protocols simultaneously:
1. **gRPC** - Standard gRPC protocol (backward compatible)
2. **Connect** - Native Connect protocol (best for browsers)
3. **gRPC-Web** - Browser-compatible gRPC
4. **HTTP/JSON** - Automatic REST-like endpoints

## Test Structure

### Integration Tests

```
tests/integration/
├── grpc/                    # gRPC protocol tests (backward compatibility)
│   └── payment_grpc_test.go
├── connect/                 # Connect protocol tests (new)
│   └── connect_protocol_test.go
├── payment/                 # Business logic tests
├── subscription/
├── payment_method/
└── merchant/
```

## Running Tests

### Prerequisites

1. **Start the ConnectRPC server:**
   ```bash
   go run ./cmd/server
   ```

2. **Verify server is running:**
   ```bash
   curl -v http://localhost:8080/grpc.health.v1.Health/Check
   ```

### Run All Integration Tests

```bash
# Run all integration tests (requires running server)
go test -tags=integration ./tests/integration/... -v
```

### Run Specific Protocol Tests

#### gRPC Protocol Tests (Backward Compatibility)

```bash
# Test that existing gRPC clients still work
go test -tags=integration ./tests/integration/grpc/... -v
```

**What this validates:**
- ✅ gRPC clients can connect to ConnectRPC server
- ✅ Backward compatibility maintained
- ✅ All gRPC methods work correctly

#### Connect Protocol Tests (Native)

```bash
# Test the native Connect protocol
go test -tags=integration ./tests/integration/connect/... -v
```

**What this validates:**
- ✅ Connect clients work correctly
- ✅ Error handling via Connect error codes
- ✅ Headers are properly handled
- ✅ All RPC methods accessible via Connect

### Run Business Logic Tests

```bash
# Test payment workflows
go test -tags=integration ./tests/integration/payment/... -v

# Test subscription workflows
go test -tags=integration ./tests/integration/subscription/... -v

# Test payment method workflows
go test -tags=integration ./tests/integration/payment_method/... -v

# Test merchant workflows
go test -tags=integration ./tests/integration/merchant/... -v
```

## Test Categories

### 1. Protocol Compatibility Tests

**Location:** `tests/integration/grpc/` and `tests/integration/connect/`

**Purpose:** Verify all protocols work correctly

**Tests:**
- Service availability
- Basic CRUD operations (List, Get, Create, Update, Delete)
- Error handling
- Header propagation
- Request/response validation

### 2. Business Logic Tests

**Location:** `tests/integration/payment/`, `subscription/`, `payment_method/`, `merchant/`

**Purpose:** Verify business workflows work correctly

**Tests:**
- Payment state transitions
- Transaction workflows
- Idempotency guarantees
- BRIC storage integration
- Browser Post workflows
- Subscription billing
- Payment method management
- Merchant onboarding

### 3. Backward Compatibility Tests

**Purpose:** Ensure existing gRPC clients continue to work

**Strategy:**
- All existing gRPC tests run without modification
- Tests use standard `google.golang.org/grpc` client
- Connect server accepts gRPC connections on same port

## Writing New Tests

### Connect Protocol Test Template

```go
//go:build integration
// +build integration

package myservice_test

import (
    "context"
    "net/http"
    "testing"
    "time"

    "connectrpc.com/connect"
    "github.com/kevin07696/payment-service/proto/myservice/v1/myservicev1connect"
    myservicev1 "github.com/kevin07696/payment-service/proto/myservice/v1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

const connectAddress = "http://localhost:8080"

func setupConnectClient(t *testing.T) myservicev1connect.MyServiceClient {
    t.Helper()

    httpClient := &http.Client{
        Timeout: 30 * time.Second,
    }

    return myservicev1connect.NewMyServiceClient(
        httpClient,
        connectAddress,
    )
}

func TestConnect_MyMethod(t *testing.T) {
    client := setupConnectClient(t)

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    req := connect.NewRequest(&myservicev1.MyRequest{
        // ... request fields
    })

    resp, err := client.MyMethod(ctx, req)
    require.NoError(t, err)
    assert.NotNil(t, resp.Msg)

    t.Log("✅ Connect protocol: Test passed")
}
```

### gRPC Protocol Test Template

```go
//go:build integration
// +build integration

package myservice_test

import (
    "context"
    "testing"
    "time"

    myservicev1 "github.com/kevin07696/payment-service/proto/myservice/v1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

const grpcAddress = "localhost:8080"

func setupGRPCClient(t *testing.T) (myservicev1.MyServiceClient, *grpc.ClientConn) {
    t.Helper()

    conn, err := grpc.NewClient(
        grpcAddress,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)

    client := myservicev1.NewMyServiceClient(conn)
    return client, conn
}

func TestGRPC_MyMethod(t *testing.T) {
    client, conn := setupGRPCClient(t)
    defer conn.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    req := &myservicev1.MyRequest{
        // ... request fields
    }

    resp, err := client.MyMethod(ctx, req)
    require.NoError(t, err)
    assert.NotNil(t, resp)

    t.Log("✅ gRPC protocol: Test passed")
}
```

## Manual Protocol Testing

### Using grpcurl (gRPC)

```bash
# List services
grpcurl -plaintext localhost:8080 list

# Call a method
grpcurl -plaintext -d '{"merchant_id":"test","limit":10,"offset":0}' \
  localhost:8080 payment.v1.PaymentService/ListTransactions
```

### Using curl (HTTP/JSON)

ConnectRPC automatically provides HTTP/JSON endpoints:

```bash
# List transactions via HTTP/JSON
curl -X POST http://localhost:8080/payment.v1.PaymentService/ListTransactions \
  -H "Content-Type: application/json" \
  -d '{"merchant_id":"test","limit":10,"offset":0}'
```

### Using Connect Client (JavaScript/TypeScript)

```typescript
import { createPromiseClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { PaymentService } from "./gen/payment/v1/payment_connect";

const transport = createConnectTransport({
  baseUrl: "http://localhost:8080",
});

const client = createPromiseClient(PaymentService, transport);

const response = await client.listTransactions({
  merchantId: "test",
  limit: 10,
  offset: 0,
});
```

## Health Checks

### gRPC Health Check

```bash
grpcurl -plaintext localhost:8080 grpc.health.v1.Health/Check
```

### Connect Health Check

```bash
curl http://localhost:8080/grpc.health.v1.Health/Check
```

## Reflection

### gRPC Reflection

```bash
# List all services
grpcurl -plaintext localhost:8080 list

# Describe a service
grpcurl -plaintext localhost:8080 describe payment.v1.PaymentService
```

### Connect Reflection

```bash
curl http://localhost:8080/grpc.reflection.v1.ServerReflection/ServerReflectionInfo
```

## Test Coverage

### Generate Coverage Report

```bash
# Run tests with coverage
go test -tags=integration ./tests/integration/... -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out
```

### Coverage Goals

- **Protocol Tests**: >80% coverage of RPC methods
- **Business Logic Tests**: >90% coverage of critical workflows
- **Error Handling**: 100% coverage of error paths

## Continuous Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.22'

      - name: Start server
        run: |
          go run ./cmd/server &
          sleep 5

      - name: Run gRPC protocol tests
        run: go test -tags=integration ./tests/integration/grpc/... -v

      - name: Run Connect protocol tests
        run: go test -tags=integration ./tests/integration/connect/... -v

      - name: Run business logic tests
        run: go test -tags=integration ./tests/integration/... -v
```

## Troubleshooting

### Server Not Responding

```bash
# Check if server is running
curl -v http://localhost:8080/grpc.health.v1.Health/Check

# Check server logs
tail -f server.log
```

### Connection Refused

- Verify server is running on port 8080
- Check firewall settings
- Ensure H2C is enabled for Connect protocol

### Protocol Mismatch

- gRPC clients should use `localhost:8080` (no http:// prefix)
- Connect/HTTP clients should use `http://localhost:8080` (with http:// prefix)

### Test Timeout

- Increase context timeout in tests
- Check server resource usage
- Verify database connectivity

## Best Practices

1. **Always test all protocols** - Run both gRPC and Connect tests
2. **Test error cases** - Verify error handling works correctly
3. **Use integration tags** - Keep integration tests separate with `//go:build integration`
4. **Clean up resources** - Use `defer conn.Close()` and `defer cancel()`
5. **Parallel-safe** - Ensure tests can run concurrently
6. **Idempotent** - Tests should not depend on execution order
7. **Descriptive names** - Use clear test function names
8. **Logging** - Add `t.Log()` for debugging failed tests

## References

- [ConnectRPC Documentation](https://connectrpc.com/docs/)
- [Connect Protocol Specification](https://connectrpc.com/docs/protocol)
- [gRPC Testing Guide](https://grpc.io/docs/languages/go/basics/#testing)
- [Migration Guide](./CONNECTRPC_MIGRATION_GUIDE.md)
