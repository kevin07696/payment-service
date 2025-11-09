# Testing Strategy

## Overview

This document outlines the testing approach for payment-service, following Amazon-style deployment gates and industry best practices.

## Test Types

### 1. Unit Tests (`internal/*/`)
**Purpose**: Test individual functions and components in isolation

**Location**: Alongside source code
```
internal/
├── adapters/
│   └── epx/
│       ├── adapter.go
│       └── adapter_test.go  ← Unit tests here
```

**When they run**: Pre-build (fast feedback)
**Dependencies**: None (mocked)
**Command**: `go test ./internal/... -v`

---

### 2. Integration Tests (`tests/integration/`)
**Purpose**: Test service integration with external systems (database, EPX, etc.) against DEPLOYED service

**Location**: `tests/integration/`
```
tests/integration/
├── merchant/
│   ├── register_test.go      # Test merchant registration API
│   └── update_test.go         # Test merchant updates
├── payment/
│   ├── create_test.go         # Test payment creation
│   └── process_test.go        # Test EPX payment processing
├── epx/
│   └── integration_test.go    # Test EPX adapter
└── testutil/
    ├── setup.go               # Test setup helpers
    ├── client.go              # API client
    └── fixtures.go            # Test data fixtures
```

**When they run**: Post-deployment to staging (deployment gate)
**Dependencies**: Deployed service, database, EPX sandbox
**Command**: `go test ./tests/integration/... -v -tags=integration`

**Key characteristic**: Tests run against DEPLOYED service URL, not localhost

---

### 3. End-to-End Tests (Future: `e2e-tests` repo)
**Purpose**: Test workflows spanning multiple services

**Location**: Separate `e2e-tests` repository (when we have multiple services)
```
e2e-tests/
└── tests/
    ├── subscription_payment_flow_test.go  # Payment + Subscription services
    └── notification_flow_test.go           # Payment + Notification services
```

**When they run**: Post-deployment to staging, before production
**Dependencies**: All deployed services
**Status**: Not yet implemented (only have one service currently)

---

## Deployment Pipeline (Amazon Pattern)

### Current Pipeline

#### develop Branch Flow
```
┌─────────────────────────────────────────────────────────────┐
│ 1. Unit Tests (pre-build)                                   │
│    ├─ Fast feedback                                         │
│    └─ No external dependencies                              │
├─────────────────────────────────────────────────────────────┤
│ 2. Build Docker Image                                       │
│    └─ Only if unit tests pass                               │
├─────────────────────────────────────────────────────────────┤
│ 3. Deploy to Staging                                        │
│    ├─ Oracle Cloud staging environment                      │
│    ├─ Run migrations                                        │
│    ├─ Run seed data (includes test merchant)                │
│    └─ Deploy service container                              │
├─────────────────────────────────────────────────────────────┤
│ 4. Integration Tests (POST-DEPLOYMENT GATE)                 │
│    ├─ Test against DEPLOYED staging service                 │
│    ├─ Use test merchant from seed data                      │
│    ├─ Validate EPX integration                              │
│    ├─ Verify API endpoints                                  │
│    └─ BLOCKS production if tests fail                       │
├─────────────────────────────────────────────────────────────┤
│ 5. Keep Staging Running ✅                                  │
│    └─ Staging persists for continued development testing    │
└─────────────────────────────────────────────────────────────┘
```

#### main Branch Flow
```
┌─────────────────────────────────────────────────────────────┐
│ 1. Unit Tests (pre-build)                                   │
│    └─ Fast validation before production deployment          │
├─────────────────────────────────────────────────────────────┤
│ 2. Build Docker Image                                       │
│    └─ Production-ready image                                │
├─────────────────────────────────────────────────────────────┤
│ 3. Deploy to Production                                     │
│    └─ Production environment deployment                     │
├─────────────────────────────────────────────────────────────┤
│ 4. Cleanup Staging Infrastructure                           │
│    └─ Tear down staging (no longer needed)                  │
└─────────────────────────────────────────────────────────────┘
```

**Why this design?**
- **develop**: Staging stays up for ongoing testing and iteration
- **main**: Staging cleaned up after production deployment (resources freed)

### Future Pipeline (with multiple services)

```
┌─────────────────────────────────────────────────────────────┐
│ Deploy payment-service, subscription-service, etc. to staging│
├─────────────────────────────────────────────────────────────┤
│ Integration Tests (per-service)                             │
│ ├─ payment-service/tests/integration/                       │
│ └─ subscription-service/tests/integration/                  │
├─────────────────────────────────────────────────────────────┤
│ E2E Tests (cross-service)                                   │
│ └─ e2e-tests repo tests all services together               │
├─────────────────────────────────────────────────────────────┤
│ Deploy to Production                                        │
│ └─ ONLY if all tests pass                                   │
└─────────────────────────────────────────────────────────────┘
```

---

## Test Data Strategy

### Development Environment
**Source**: Seed data in `internal/db/seeds/development/`
**Purpose**: Local development and manual testing
**Credentials**: Local test data

### Staging Environment
**Source**: Seed data in `internal/db/seeds/staging/`
**Purpose**: Integration testing against deployed service
**Test Merchant**: `003_agent_credentials.sql` with EPX sandbox credentials
**Credentials**: EPX sandbox (public test credentials)

### Production Environment
**Source**: Real merchant data
**Credentials**: OCI Vault (secure secret management)
**No test data**: Production database has only real merchants

---

## Environment Variables for Tests

### Integration Tests Configuration

```bash
# .env.integration (for local integration test runs)
SERVICE_URL=http://localhost:8080  # Or deployed staging URL
DB_CONNECTION_STRING=postgresql://...

# EPX Sandbox Credentials (safe to commit - public test credentials)
EPX_MAC_STAGING=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
EPX_CUST_NBR=9001
EPX_MERCH_NBR=900300
EPX_DBA_NBR=2
EPX_TERMINAL_NBR=77
```

### GitHub Secrets (payment-service repo)

**Service Secrets:**
- `ORACLE_DB_PASSWORD` - Database password
- `CRON_SECRET_STAGING` - Cron endpoint auth

**Test Credentials (can be GitHub Secrets or committed):**
- `EPX_MAC_STAGING` - EPX sandbox MAC
- `EPX_CUST_NBR` - EPX Customer Number
- `EPX_MERCH_NBR` - EPX Merchant Number
- `EPX_DBA_NBR` - EPX DBA Number
- `EPX_TERMINAL_NBR` - EPX Terminal Number

**Note**: EPX sandbox credentials are public and could be committed in `.env.integration.example`, but we use GitHub Secrets for consistency.

---

## Running Tests

### Locally

**Unit Tests:**
```bash
go test ./internal/... -v
```

**Integration Tests (against local service):**
```bash
# Start local environment
docker-compose up -d

# Run integration tests
export SERVICE_URL=http://localhost:8080
export EPX_MAC_STAGING=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
# ... other EPX vars
go test ./tests/integration/... -v -tags=integration
```

**Integration Tests (against deployed staging):**
```bash
export SERVICE_URL=http://your-staging-url.com
go test ./tests/integration/... -v -tags=integration
```

### In CI/CD

**GitHub Actions automatically:**
1. Runs unit tests before build
2. Deploys to staging
3. Runs integration tests against deployed staging
4. Blocks production deployment if tests fail

---

## When to Add E2E Tests Repository

**Create `e2e-tests` repo when:**
- ✅ You have 2+ services that interact
- ✅ You need to test cross-service workflows
- ✅ You want to test complete user journeys

**Example scenarios:**
- Subscription service triggers payment via payment-service
- Payment completion sends notification via notification-service
- User signs up → creates subscription → processes payment

**Until then:** Keep integration tests in `payment-service/tests/integration/`

---

## Benefits of This Approach

✅ **Amazon-style deployment gate**: Integration tests block bad deployments
✅ **Standard structure**: Tests live with service code
✅ **Atomic commits**: Update code and tests together
✅ **Simple local development**: One repo, clear structure
✅ **Scalable**: Easy to add e2e-tests repo later for cross-service tests
✅ **Fast feedback**: Unit tests → Integration tests → Production

---

## References

- Unit tests: Tests individual components
- Integration tests: Tests service integration with external systems
- E2E tests: Tests complete user workflows across services
- Deployment gates: Quality checks that block bad deployments
