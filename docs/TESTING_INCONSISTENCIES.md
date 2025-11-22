# Testing Inconsistencies Report

**Date**: 2025-11-22
**Status**: ⚠️ NEEDS ATTENTION

## Overview

The integration test suite has port configuration inconsistencies that make automated testing error-prone. This document identifies all issues and provides recommendations.

---

## Port Architecture (Correct Design)

Based on `docker-compose.yml` and `cmd/server/main.go`:

| Port | Protocol | Purpose | Usage |
|------|----------|---------|-------|
| **8080** | ConnectRPC (HTTP/2) | Primary API server | All ConnectRPC/gRPC endpoints |
| **8081** | HTTP/1.1 | Utility endpoints | Browser Post callbacks, Cron handlers |

### Why Two Ports?

1. **Port 8080** - ConnectRPC protocol requires HTTP/2 with specific headers
2. **Port 8081** - Simple HTTP endpoints for:
   - EPX Browser Post callbacks (`/api/v1/payments/browser-post/callback`)
   - Cron endpoints (`/cron/*`)
   - Health checks

---

## Identified Inconsistencies

### 1. Hardcoded Port Values

**Issue**: Tests hardcode port numbers instead of using environment variables.

**Locations**:
```go
// testutil/browser_post_automated.go:86
httpClient := NewClient("http://localhost:8081")  // ❌ Hardcoded

// testutil/browser_post_automated.go:247
connectRPCClient := NewClient("http://localhost:8080")  // ❌ Hardcoded

// testutil/tokenization.go:342, 397
connectRPCClient := NewClient("http://localhost:8080")  // ❌ Hardcoded
```

**Impact**: Cannot run tests against staging/production environments without code changes.

**Files Affected**:
- `tests/integration/testutil/browser_post_automated.go`
- `tests/integration/testutil/tokenization.go`
- `tests/integration/cron/ach_verification_cron_test.go`
- `tests/integration/payment_method/payment_method_test.go`
- `tests/integration/payment/server_post_workflow_test.go`
- `tests/integration/payment/server_post_idempotency_test.go`
- `tests/integration/payment/browser_post_idempotency_test.go`
- `tests/integration/payment/browser_post_workflow_test.go`
- `tests/integration/subscription/recurring_billing_test.go`
- `tests/integration/merchant/merchant_test.go`

### 2. SERVICE_URL Confusion

**Issue**: `SERVICE_URL` environment variable is ambiguous - does it point to ConnectRPC (8080) or HTTP (8081)?

**Current Implementation**:
```go
// testutil/config.go:22
ServiceURL: getEnv("SERVICE_URL", "http://localhost:8080")  // Defaults to ConnectRPC
```

**Problems**:
- Some tests expect `SERVICE_URL` to be `:8080` (ConnectRPC)
- Other tests need `:8081` for browser post callbacks
- No clear separation of concerns

### 3. Callback URL Hardcoding

**Issue**: Browser Post callback URLs hardcoded to `http://localhost:8081`.

**Example**:
```go
// From multiple test files
callbackBaseURL := "http://localhost:8081"  // ❌ Hardcoded
```

**Impact**:
- Cannot use ngrok or other tunneling services without code changes
- Cannot test against deployed staging environments
- EPX callbacks fail when service runs on different host/port

### 4. No Environment Variable Standards

**Issue**: No consistent environment variables for different endpoint types.

**Current State**:
- ✅ `SERVICE_URL` exists (but ambiguous)
- ❌ No `CONNECTRPC_URL` variable
- ❌ No `HTTP_CALLBACK_URL` variable
- ❌ No `CRON_BASE_URL` variable

---

## Recommendations

### 1. Define Clear Environment Variables

Create distinct environment variables for each service type:

```bash
# Primary ConnectRPC server (port 8080)
export CONNECTRPC_URL="http://localhost:8080"

# HTTP callback server (port 8081)
export HTTP_CALLBACK_URL="http://localhost:8081"

# For ngrok tunneling (when needed)
export NGROK_URL="https://abc123.ngrok.io"
```

### 2. Update testutil/config.go

```go
type Config struct {
    // ConnectRPC endpoint (port 8080)
    ConnectRPCURL string

    // HTTP callback URL for Browser Post (port 8081)
    // Can be localhost or ngrok URL
    HTTPCallbackURL string

    // Cron endpoint (port 8081)
    CronURL string

    // Database connection
    DatabaseURL string

    // Test merchant credentials
    MerchantID string
    JWTID      string
}

func LoadConfig() *Config {
    return &Config{
        ConnectRPCURL:   getEnv("CONNECTRPC_URL", "http://localhost:8080"),
        HTTPCallbackURL: getEnv("HTTP_CALLBACK_URL", "http://localhost:8081"),
        CronURL:         getEnv("CRON_URL", "http://localhost:8081"),
        DatabaseURL:     getEnv("TEST_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"),
        MerchantID:      getEnv("TEST_MERCHANT_ID", "00000000-0000-0000-0000-000000000001"),
        JWTID:           getEnv("TEST_JWT_ID", "00000000-0000-0000-0000-000000000002"),
    }
}
```

### 3. Update testutil Helper Functions

```go
// NewConnectRPCClient creates a client for ConnectRPC endpoints (port 8080)
func NewConnectRPCClient() *http.Client {
    cfg := LoadConfig()
    return NewClient(cfg.ConnectRPCURL)
}

// NewHTTPClient creates a client for HTTP endpoints (port 8081)
func NewHTTPClient() *http.Client {
    cfg := LoadConfig()
    return NewClient(cfg.HTTPCallbackURL)
}

// GetCallbackURL returns the base URL for EPX callbacks
func GetCallbackURL() string {
    cfg := LoadConfig()
    // Use ngrok URL if available, otherwise HTTP callback URL
    if ngrokURL := os.Getenv("NGROK_URL"); ngrokURL != "" {
        return ngrokURL
    }
    return cfg.HTTPCallbackURL
}
```

### 4. Refactor Hardcoded Ports

**Before**:
```go
httpClient := NewClient("http://localhost:8081")
connectRPCClient := NewClient("http://localhost:8080")
callbackBaseURL := "http://localhost:8081"
```

**After**:
```go
httpClient := NewHTTPClient()
connectRPCClient := NewConnectRPCClient()
callbackBaseURL := GetCallbackURL()
```

### 5. Create Test Runner Scripts

Create helper scripts for different test scenarios:

**`scripts/test-local.sh`**:
```bash
#!/bin/bash
export CONNECTRPC_URL="http://localhost:8080"
export HTTP_CALLBACK_URL="http://localhost:8081"
export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"

go test -tags=integration ./tests/integration/... -count=1 "$@"
```

**`scripts/test-ngrok.sh`**:
```bash
#!/bin/bash
if [ -z "$NGROK_URL" ]; then
    echo "Error: NGROK_URL environment variable not set"
    echo "Usage: NGROK_URL=https://abc123.ngrok.io ./scripts/test-ngrok.sh"
    exit 1
fi

export CONNECTRPC_URL="http://localhost:8080"
export HTTP_CALLBACK_URL="$NGROK_URL"
export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"

echo "Testing with ngrok URL: $NGROK_URL"
go test -tags=integration ./tests/integration/... -count=1 "$@"
```

**`scripts/test-staging.sh`**:
```bash
#!/bin/bash
if [ -z "$STAGING_URL" ]; then
    echo "Error: STAGING_URL environment variable not set"
    exit 1
fi

export CONNECTRPC_URL="$STAGING_URL"
export HTTP_CALLBACK_URL="$STAGING_URL"
export TEST_DATABASE_URL="$STAGING_DATABASE_URL"

echo "Testing against staging: $STAGING_URL"
go test -tags=integration ./tests/integration/... -count=1 "$@"
```

### 6. Update Integration Test README

Add clear documentation in `tests/integration/README.md`:

```markdown
## Environment Variables

| Variable | Purpose | Default | Example |
|----------|---------|---------|---------|
| `CONNECTRPC_URL` | ConnectRPC server (port 8080) | `http://localhost:8080` | `https://staging.example.com` |
| `HTTP_CALLBACK_URL` | HTTP callback server (port 8081) | `http://localhost:8081` | `https://abc123.ngrok.io` |
| `NGROK_URL` | Override callback URL for ngrok | - | `https://abc123.ngrok.io` |
| `TEST_DATABASE_URL` | PostgreSQL connection | `postgres://...` | - |

## Running Tests

### Local Testing
```bash
./scripts/test-local.sh
```

### With ngrok (for real EPX callbacks)
```bash
ngrok http 8081  # Start ngrok
NGROK_URL=https://abc123.ngrok.io ./scripts/test-ngrok.sh
```

### Against Staging
```bash
STAGING_URL=https://staging.example.com \
STAGING_DATABASE_URL=postgres://... \
./scripts/test-staging.sh
```
```

---

## Priority Action Items

1. ✅ **HIGH**: Update `testutil/config.go` with distinct URL variables
2. ✅ **HIGH**: Create `NewConnectRPCClient()` and `NewHTTPClient()` helpers
3. ✅ **HIGH**: Refactor all hardcoded ports in `testutil/*.go`
4. ✅ **MEDIUM**: Update all test files to use new helpers
5. ✅ **MEDIUM**: Create test runner scripts
6. ✅ **LOW**: Update documentation

---

## Benefits of Fixing

1. **Automated Testing**: Tests can run against any environment without code changes
2. **CI/CD Friendly**: Environment-based configuration works in GitHub Actions
3. **Developer Experience**: Clear separation between ConnectRPC and HTTP endpoints
4. **Ngrok Support**: Easy to test with real EPX callbacks using ngrok
5. **Staging/Production**: Same tests work across all environments

---

## Current Workarounds

Until fixed, developers must:

1. Manually change hardcoded URLs for different environments
2. Run tests only against localhost
3. Remember which port is which (8080 vs 8081)
4. Cannot easily use ngrok for Browser Post testing

---

**Status**: 🔴 CRITICAL - Blocks automated testing and staging/production validation
