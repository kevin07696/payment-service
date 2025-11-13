# Development Guide

**Audience:** Developers contributing to the payment service.
**Topic:** Architecture, branching strategy, testing, and development workflow.
**Goal:** Enable developers to build, test, and deploy features following established patterns.

## Overview

This guide covers the development lifecycle from architecture decisions to deployment. The service follows **hexagonal architecture** (ports & adapters) with strict separation of concerns.

## Table of Contents

1. [Architecture](#architecture)
2. [Project Structure](#project-structure)
3. [Branching Strategy](#branching-strategy)
4. [Testing](#testing)
5. [Development Workflow](#development-workflow)
6. [Code Standards](#code-standards)

---

## Architecture

### Design Principles

**1. Hexagonal Architecture (Ports & Adapters)**

The service follows clean hexagonal architecture where **business logic is isolated** from external dependencies.

```text
┌────────────────────────────────────────────────────────────┐
│                        HANDLERS                            │
│              (gRPC/HTTP - API Layer)                       │
└──────────────────────────┬─────────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────────┐
│                        SERVICES                            │
│                   (Business Logic)                         │
└────────────┬──────────────────────────────┬────────────────┘
             │                               │
             ▼                               ▼
┌─────────────────────┐         ┌─────────────────────────┐
│   DOMAIN MODELS     │         │   PORTS (Interfaces)    │
│   (Core Entities)   │         │   - Payment Gateway     │
└─────────────────────┘         │   - Database            │
                                │   - Secret Manager      │
                                └──────────┬──────────────┘
                                           │
                                           ▼
                                ┌────────────────────────┐
                                │   ADAPTERS             │
                                │   - EPX Adapter        │
                                │   - PostgreSQL/SQLC    │
                                │   - GCP Secrets        │
                                └────────────────────────┘
```

**Benefits:**
- **Testability:** Mock any adapter via interface
- **Maintainability:** Swap implementations without changing business logic
- **Independence:** Domain logic has zero dependencies on frameworks

**2. Dependency Injection**

All dependencies are **interfaces** injected into services:

```go
type PaymentService struct {
    db              ports.PaymentRepository
    gateway         ports.PaymentGateway
    secretManager   ports.SecretManager
    logger          *zap.Logger
}

func NewPaymentService(
    db ports.PaymentRepository,
    gateway ports.PaymentGateway,
    secretManager ports.SecretManager,
    logger *zap.Logger,
) *PaymentService {
    return &PaymentService{
        db:            db,
        gateway:       gateway,
        secretManager: secretManager,
        logger:        logger,
    }
}
```

**3. Multi-Tenant Design**

All operations are scoped to `merchant_id`:

```go
// Queries always filter by merchant
SELECT * FROM transactions
WHERE merchant_id = $1 AND customer_id = $2;

// Authorization enforces merchant isolation
if token.MerchantID != nil && req.MerchantId != *token.MerchantID {
    return ErrUnauthorized
}
```

### Key Components

**Domain Layer** (`internal/domain/`)
- Pure business entities
- No external dependencies
- Immutable value objects

**Service Layer** (`internal/services/`)
- Business logic orchestration
- Transaction management
- Validation rules

**Handler Layer** (`internal/handlers/`)
- API request/response mapping
- Input validation
- Error translation

**Adapter Layer** (`internal/adapters/`)
- External system integration
- Database operations (SQLC)
- Third-party APIs (EPX)

**Port Layer** (`internal/services/ports/`)
- Interface definitions
- Contracts between layers

---

## Project Structure

```text
payment-service/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── adapters/                # External integrations
│   │   ├── epx/                 # EPX gateway adapter
│   │   ├── gcp/                 # GCP secret manager
│   │   └── mock/                # Test mocks
│   ├── auth/                    # JWT validation
│   ├── db/
│   │   ├── migrations/          # Database migrations (goose)
│   │   ├── queries/             # SQL queries (SQLC)
│   │   └── sqlc/                # Generated code
│   ├── domain/                  # Core business entities
│   │   ├── transaction.go
│   │   ├── payment_method.go
│   │   ├── subscription.go
│   │   └── merchant.go
│   ├── handlers/                # API handlers
│   │   ├── payment/
│   │   ├── payment_method/
│   │   ├── subscription/
│   │   └── merchant/
│   ├── middleware/              # gRPC interceptors
│   ├── services/                # Business logic
│   │   ├── payment/
│   │   ├── payment_method/
│   │   ├── subscription/
│   │   ├── merchant/
│   │   └── ports/               # Interface definitions
│   └── config/                  # Configuration
├── proto/                       # Protocol buffers
│   ├── payment/v1/
│   ├── payment_method/v1/
│   ├── subscription/v1/
│   └── merchant/v1/
├── tests/
│   └── integration/             # Integration tests
└── docs/                        # Documentation
```

### Directory Responsibilities

| Directory | Purpose | Dependencies |
|-----------|---------|--------------|
| `domain/` | Core entities | None |
| `services/` | Business logic | Domain + Ports |
| `handlers/` | API layer | Services + Proto |
| `adapters/` | External systems | Ports (implements) |
| `proto/` | API contracts | None |

---

## Branching Strategy

### Branch Model

| Branch | Environment | Platform | Deploy | Purpose |
|--------|-------------|----------|--------|---------|
| `main` | Production | Google Cloud Run | Manual approval | Stable production code |
| `develop` | Staging | Oracle Cloud | Automatic | Integration testing |
| `feature/*` | Local | - | No | New features |
| `bugfix/*` | Local | - | No | Bug fixes |
| `hotfix/*` | Local | - | No | Emergency fixes |

### Daily Workflow

#### Feature Development

```bash
# Start from develop
git checkout develop && git pull

# Create feature branch
git checkout -b feature/refund-api

# Work and commit
git add . && git commit -m "feat: Add refund API endpoint"
git push origin feature/refund-api

# Create PR: feature/refund-api → develop
# After merge: automatically deploys to staging
```

#### Deploy to Staging

```bash
# Direct work on develop for quick fixes
git checkout develop
git add . && git commit -m "fix: Correct amount validation"
git push origin develop
# Automatically deploys to staging
```

#### Deploy to Production

```bash
# Create PR on GitHub: develop → main
# Get team approval
# Merge PR
# Approve deployment in GitHub Actions
# Deploys to production
```

#### Hotfix

```bash
# Create from main
git checkout main && git pull
git checkout -b hotfix/security-patch

# Fix and push
git add . && git commit -m "fix: Security vulnerability"
git push origin hotfix/security-patch

# Create PR → main, get approval, merge
# After deploy: merge back to develop
git checkout develop && git merge hotfix/security-patch && git push
```

### Branch Protection

**main branch requires:**
- ✅ Unit tests passed
- ✅ Build successful
- ✅ Integration tests passed (from develop)
- ✅ 1 code review approval
- ✅ Branch up-to-date
- ❌ No force pushes
- ❌ No deletions

**develop branch requires:**
- ✅ Unit tests passed
- ❌ No force pushes

### CI/CD Pipeline Flow

```text
develop branch:
  Push → Tests → Build → Deploy staging → Integration tests → Keep running

main branch:
  PR → Tests → Build → Wait approval → Deploy production → Smoke tests
```

---

## Testing

### Test Types

| Type | Command | When | Duration |
|------|---------|------|----------|
| Unit | `go test ./...` | Every commit | <30s |
| Integration | `go test -tags=integration ./tests/integration/...` | Post-deploy staging | <5m |
| Coverage | `go test -cover ./...` | Before PR | <1m |
| Smoke | `./scripts/smoke-test.sh` | Post-production deploy | <30s |

### Unit Tests

**Run all tests:**
```bash
go test ./...
go test ./internal/services/payment/...  # Specific package
go test -v ./...                         # Verbose
go test -cover ./...                     # With coverage
```

**Coverage targets:** >80% overall, 100% critical payment paths

**Example test structure:**
```go
func TestPaymentService_Authorize(t *testing.T) {
    tests := []struct {
        name    string
        request *AuthorizeRequest
        want    *PaymentResponse
        wantErr bool
    }{
        {
            name: "valid authorization",
            request: &AuthorizeRequest{
                MerchantID: "merchant-123",
                Amount:     "100.00",
            },
            want: &PaymentResponse{
                Status: TransactionStatusCompleted,
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

**Prerequisites:**
1. Running payment service (Docker Compose or podman-compose)
2. PostgreSQL with migrations applied
3. EPX sandbox credentials

**Setup:**
```bash
# Start services
docker-compose up -d

# Verify health
curl http://localhost:8081/cron/health

# Set environment
export SERVICE_URL=http://localhost:8081
export EPX_MAC_STAGING=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y

# Run tests
go test -v -tags=integration ./tests/integration/...
```

**Test coverage:**
- ✅ Payment transactions (sale, authorize, capture)
- ✅ Refund and void operations
- ✅ Payment method storage
- ✅ Subscription lifecycle
- ✅ Validation errors
- ⏭️ BRIC storage (pending EPX sandbox setup)

**Example integration test:**
```go
//go:build integration

func TestSaleTransaction(t *testing.T) {
    client := testutil.NewPaymentClient()

    resp, err := client.Sale(context.Background(), &payment.SaleRequest{
        MerchantId:      "test-merchant-staging",
        Amount:          "50.00",
        PaymentMethodId: "pm_test_123",
        IdempotencyKey:  uuid.New().String(),
    })

    require.NoError(t, err)
    assert.Equal(t, payment.TransactionStatus_COMPLETED, resp.Status)
}
```

### Coverage Reports

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out       # Summary
go tool cover -html=coverage.out       # Browser view
```

### Writing Tests

**Table-driven tests:**
```go
tests := []struct {
    name    string
    input   Request
    wantErr bool
}{
    {"valid request", validReq, false},
    {"missing amount", invalidReq, true},
}
```

**Naming convention:**
- Good: `TestAuthorize_WithValidCard_ShouldSucceed`
- Bad: `TestTransaction`

---

## Development Workflow

### Local Development

**1. Setup environment:**
```bash
# Install dependencies
go mod download

# Install tools
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

**2. Start dependencies:**
```bash
docker-compose up -d postgres
```

**3. Run migrations:**
```bash
goose -dir internal/db/migrations postgres "postgresql://..." up
```

**4. Generate code:**
```bash
make proto   # Generate protobuf code
make sqlc    # Generate database code
```

**5. Run service:**
```bash
go run cmd/server/main.go
```

### Making Changes

**1. Update proto definitions:**
```bash
# Edit proto/payment/v1/payment.proto
make proto
```

**2. Update database schema:**
```bash
# Create migration
goose -dir internal/db/migrations create add_column sql

# Edit migration file
# Run migration
goose -dir internal/db/migrations postgres "postgresql://..." up

# Update queries in internal/db/queries/
make sqlc
```

**3. Implement business logic:**
```bash
# Edit service layer
internal/services/payment/payment_service.go

# Add/update tests
internal/services/payment/payment_service_test.go
```

**4. Run tests:**
```bash
go test ./...
go vet ./...
go build ./...
```

### Pre-Commit Checklist

- [ ] Tests pass: `go test ./...`
- [ ] No linting errors: `go vet ./...`
- [ ] Code builds: `go build ./...`
- [ ] Proto generated: `make proto`
- [ ] SQLC generated: `make sqlc`
- [ ] Commit follows convention: `feat:`, `fix:`, `docs:`

---

## Code Standards

### Commit Convention

Use conventional commits:

```bash
feat: Add refund API endpoint
fix: Correct amount validation logic
docs: Update API documentation
refactor: Simplify payment service
test: Add unit tests for authorize
chore: Update dependencies
```

### Go Code Style

**Follow Go idioms:**
- Use `gofmt` for formatting
- Follow effective Go guidelines
- Use meaningful variable names
- Add godoc comments for exported functions

**Error handling:**
```go
// Good
if err != nil {
    return nil, fmt.Errorf("failed to authorize payment: %w", err)
}

// Bad
if err != nil {
    return nil, err  // No context
}
```

**Logging:**
```go
logger.Info("processing payment",
    zap.String("transaction_id", txID),
    zap.String("merchant_id", merchantID),
    zap.String("amount", amount),
)
```

### Code Organization

**Service layer pattern:**
```go
type PaymentService struct {
    // Dependencies as interfaces
    db            ports.PaymentRepository
    gateway       ports.PaymentGateway
    logger        *zap.Logger
}

func (s *PaymentService) Authorize(ctx context.Context, req *AuthorizeRequest) (*PaymentResponse, error) {
    // 1. Validate input
    if err := s.validateAuthorizeRequest(req); err != nil {
        return nil, err
    }

    // 2. Business logic
    // 3. Call adapters
    // 4. Return result
}
```

### Environment Configuration

**Use .env files:**
```bash
# .env.development
DATABASE_URL=postgresql://localhost:5432/payment_service
EPX_BASE_URL=https://secure.epxuap.com
LOG_LEVEL=debug

# .env.staging
DATABASE_URL=postgresql://staging-db:5432/payment_service
EPX_BASE_URL=https://secure.epxuap.com
LOG_LEVEL=info

# .env.production
DATABASE_URL=postgresql://prod-db:5432/payment_service
EPX_BASE_URL=https://secure.epxnow.com
LOG_LEVEL=warn
```

**Load with:**
```go
cfg := config.LoadConfig()
```

---

## Summary

### Development Checklist

**Starting a feature:**
1. Create branch from `develop`
2. Write tests first (TDD)
3. Implement feature
4. Run tests locally
5. Create PR to `develop`

**Deploying to staging:**
1. Merge PR to `develop`
2. CI runs tests and deploys
3. Integration tests run automatically
4. Verify in staging environment

**Deploying to production:**
1. Create PR from `develop` to `main`
2. Get code review approval
3. Merge PR
4. Approve deployment in GitHub Actions
5. Monitor production logs

### Key Principles

1. **Interfaces over implementations** - All dependencies are ports
2. **Test coverage matters** - >80% overall, 100% critical paths
3. **Commit conventions** - Use conventional commits
4. **Branch protection** - Never force push to main/develop
5. **Integration tests** - Required before production

---

## References

- Dataflows: `DATAFLOW.md`
- API Specifications: `API_SPECS.md`
- CI/CD Pipeline: `CICD.md`
- Database Schema: `DATABASE.md`
- Authentication: `AUTH.md`