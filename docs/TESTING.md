# Testing

## Quick Reference

| Type | Command | When | Duration |
|------|---------|------|----------|
| Unit | `go test ./...` | Every commit | <30s |
| Integration | `go test ./tests/integration/... -tags=integration` | Post-deploy to staging | <5m |
| Coverage | `go test -cover ./...` | Before PR | <1m |
| Smoke | `./scripts/smoke-test.sh IP 8081 8080` | Post-deploy | <30s |

## Running Tests

### Unit Tests

```bash
go test ./...                              # All unit tests
go test ./internal/adapters/epx            # Specific package
go test -v ./...                           # Verbose output
go test -cover ./...                       # With coverage
go test -short ./...                       # Skip integration tests
```

### Integration Tests

Requires deployed service (local or staging):

```bash
# Set environment
export SERVICE_URL=http://localhost:8080
export CRON_SECRET=your-secret
export EPX_MAC_STAGING=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
export EPX_CUST_NBR=9001
export EPX_MERCH_NBR=900300
export EPX_DBA_NBR=2
export EPX_TERMINAL_NBR=77

# Run tests
go test ./tests/integration/... -v -tags=integration
go test ./tests/integration/... -v -tags=integration -timeout=15m
```

### Coverage Reports

```bash
go test -coverprofile=coverage.out ./...   # Generate
go tool cover -func=coverage.out           # View summary
go tool cover -html=coverage.out           # View in browser
```

Coverage targets: >80% overall, 100% critical payment paths.

## CI/CD Pipeline

Amazon deployment gate pattern - integration tests block bad deployments:

```text
develop branch:
  Unit tests → Build → Deploy staging → Integration tests → Keep running
                                              ↓ blocks if failed

main branch:
  Branch protection (requires integration tests passed)
    → Deploy production → Smoke tests → Cleanup staging
```

Pipeline stages in `.github/workflows/ci-cd.yml`:


1. **Unit tests** - Pre-build validation
2. **Build & push** - Docker image to OCIR
3. **Deploy staging** - Provision infra, migrate DB, seed data, deploy service
4. **Integration tests** - Test against deployed service (DEPLOYMENT GATE)
5. **Keep staging** - Available for continued testing

## Test Structure

```text
tests/integration/
├── merchant/
│   └── merchant_test.go        # Merchant API tests
├── payment/                    # (Future) Payment processing
├── epx/                        # (Future) EPX adapter
└── testutil/
    ├── config.go               # Environment config
    ├── client.go               # HTTP client
    └── setup.go                # Test fixtures
```

## Writing Tests

### Table-Driven Tests

```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   Request
        wantErr bool
    }{
        {"valid", validReq, false},
        {"missing amount", invalidReq, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validate(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Integration Test Suite

```go
//go:build integration

type IntegrationSuite struct {
    suite.Suite
    adapter ports.ServerPostAdapter
}

func (s *IntegrationSuite) SetupTest() {
    time.Sleep(2 * time.Second)  // EPX rate limiting
}
```

### Naming

Good: `TestSaleTransaction_WithValidCard_ShouldSucceed`
Bad: `TestTransaction`

## GitHub Secrets

Required for CI/CD integration tests:

**Infrastructure (6):** OCI_USER_OCID, OCI_TENANCY_OCID, OCI_COMPARTMENT_OCID, OCI_REGION, OCI_FINGERPRINT, OCI_PRIVATE_KEY

**Container Registry (4):** OCIR_REGION, OCIR_TENANCY_NAMESPACE, OCIR_USERNAME, OCIR_AUTH_TOKEN

**Database (1):** ORACLE_DB_PASSWORD

**EPX Test (5):** EPX_MAC_STAGING, EPX_CUST_NBR, EPX_MERCH_NBR, EPX_DBA_NBR, EPX_TERMINAL_NBR

**Application (3):** CRON_SECRET_STAGING, SSH_PUBLIC_KEY, ORACLE_CLOUD_SSH_KEY

## Troubleshooting

### Integration Tests Fail

```bash
# Check service health
curl http://STAGING_IP:8081/cron/health

# View logs
ssh ubuntu@STAGING_IP
docker logs payment-staging --tail 100

# Verify EPX
curl -I https://secure.epxuap.com

# Check database
docker exec payment-staging env | grep DB
```

### Coverage Issues

```bash
# Verify file generated
ls -lh coverage.out

# Check specific package
go test -cover ./internal/adapters/epx
```

### Tests Timeout

```bash
go test -timeout 30s ./...     # Increase timeout
go test -race ./...            # Check race conditions
```

## References

- Unit tests: `internal/**/*_test.go`
- Integration tests: `tests/integration/**`
- Test data: `internal/db/seeds/staging/003_agent_credentials.sql`
- EPX API: `docs/EPX_API_REFERENCE.md`
- CI/CD: `.github/workflows/ci-cd.yml`
- Future E2E: `docs/FUTURE_E2E_TESTING.md`
