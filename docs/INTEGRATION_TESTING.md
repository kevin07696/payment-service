# Integration Testing Guide

Automated integration testing for staging environment to ensure deployments are healthy before considering them stable.

## Overview

Integration tests run automatically after every staging deployment to verify:
- ✅ Service is running and accessible
- ✅ All critical endpoints respond correctly
- ✅ Database connectivity works
- ✅ EPX sandbox integration is functional
- ✅ Performance is acceptable
- ✅ Security controls are in place

## Test Suites

### 1. Smoke Tests (Critical)
**Fast, essential checks that must pass**

- Health endpoint (`/cron/health`)
- Stats endpoint (`/cron/stats`)
- gRPC server connectivity
- Response time checks

**Run time:** ~30 seconds
**Failure impact:** Deployment considered failed

### 2. Integration Tests (Comprehensive)
**Full test suite covering all functionality**

- Health endpoints verification
- gRPC connectivity (secure & insecure)
- Service readiness polling
- Environment configuration
- Database connectivity
- Cron authentication
- Response time benchmarks

**Run time:** ~3-5 minutes
**Failure impact:** Alerts team, may block production

### 3. EPX Integration Tests (External)
**Tests EPX sandbox connectivity**

- EPX sandbox reachability
- Response code validation
- Database persistence
- Card processing

**Run time:** ~5-10 minutes
**Failure impact:** Warning only (EPX may have issues)

### 4. Load Tests (Optional)
**Concurrency and performance validation**

- Concurrent request handling
- Service scalability
- Resource utilization

**Run time:** ~10-15 minutes
**Failure impact:** Advisory only

---

## Automatic Triggers

### After Staging Deployment

Integration tests run automatically when:
```
Push to develop → Deploy to staging → Integration tests
```

**What happens:**
1. Waits 30 seconds for deployment to stabilize
2. Runs smoke tests (critical)
3. Runs full integration suite
4. Runs EPX integration tests
5. Posts results to workflow summary
6. Comments on commit if failed

### Manual Trigger

Run tests anytime via GitHub Actions:

```
Actions → Integration Tests → Run workflow
```

**Options:**
- `target_host` - Override target (default: staging)

---

## Running Tests Locally

### Prerequisites

```bash
# Install Go 1.24+
go version

# Install dependencies
go mod download
```

### Environment Variables

```bash
export GRPC_HOST="localhost:8080"           # or staging IP
export HTTP_HOST="localhost:8081"           # or staging IP
export CRON_SECRET="your-cron-secret"
export ENVIRONMENT="staging"
```

### Run All Tests

```bash
# Full integration suite
go test -v ./test/integration/...

# Specific test
go test -v ./test/integration -run TestHealthEndpoints

# With timeout
go test -v -timeout 10m ./test/integration/...
```

### Run Smoke Test

```bash
# Using script
./scripts/smoke-test.sh YOUR_STAGING_IP 8081 8080

# Manual curl
curl http://YOUR_STAGING_IP:8081/cron/health
curl http://YOUR_STAGING_IP:8081/cron/stats
```

### Skip Certain Tests

```bash
# Skip EPX tests (if EPX is down)
export SKIP_EPX_TESTS=true
go test -v ./test/integration/...

# Skip load tests
export SKIP_LOAD_TESTS=true
go test -v ./test/integration/...
```

---

## Test Structure

```
test/integration/
├── staging_test.go          # Core integration tests
│   ├── TestHealthEndpoints
│   ├── TestGRPCConnectivity
│   ├── TestServiceReadiness
│   ├── TestEnvironmentConfiguration
│   ├── TestDatabaseConnectivity
│   ├── TestCronAuthentication
│   └── TestResponseTimes
│
└── epx_integration_test.go  # EPX-specific tests
    ├── TestEPXSandboxConnectivity
    ├── TestEPXResponseCodes
    ├── TestDatabasePersistence
    └── TestServiceConcurrency
```

---

## Test Details

### Health Endpoints
```go
func TestHealthEndpoints(t *testing.T)
```
**Verifies:**
- `/cron/health` returns 200
- `/cron/stats` returns 200

**Critical:** Yes

### gRPC Connectivity
```go
func TestGRPCConnectivity(t *testing.T)
```
**Verifies:**
- gRPC server accepts connections
- Both secure and insecure modes work
- Connection state is healthy

**Critical:** Yes

### Service Readiness
```go
func TestServiceReadiness(t *testing.T)
```
**Verifies:**
- Service becomes ready within timeout
- Health endpoint responds correctly
- Polls up to 30 attempts

**Critical:** Yes

### Database Connectivity
```go
func TestDatabaseConnectivity(t *testing.T)
```
**Verifies:**
- Stats endpoint can query database
- Database connection is working
- Queries return data

**Critical:** Yes

### Cron Authentication
```go
func TestCronAuthentication(t *testing.T)
```
**Verifies:**
- Endpoints require authentication
- Correct secret grants access
- Wrong/missing secret returns 401/403

**Critical:** Yes (security)

### Response Times
```go
func TestResponseTimes(t *testing.T)
```
**Verifies:**
- Health check < 1 second
- Stats endpoint < 3 seconds

**Critical:** No (advisory)

### EPX Sandbox Connectivity
```go
func TestEPXSandboxConnectivity(t *testing.T)
```
**Verifies:**
- EPX sandbox is reachable
- Can process test transactions
- Response structure is valid

**Critical:** No (EPX may be down)

### EPX Response Codes
```go
func TestEPXResponseCodes(t *testing.T)
```
**Verifies:**
- EPX returns valid response codes
- Approved/declined flow works
- Response parsing is correct

**Critical:** No

### Database Persistence
```go
func TestDatabasePersistence(t *testing.T)
```
**Verifies:**
- Transactions are saved to database
- Transaction IDs are generated
- Data persists correctly

**Critical:** No

### Service Concurrency
```go
func TestServiceConcurrency(t *testing.T)
```
**Verifies:**
- Service handles 10 concurrent requests
- No race conditions
- All requests complete successfully

**Critical:** No (load test)

---

## Continuous Integration

### Workflow: `.github/workflows/integration-tests.yml`

```yaml
Trigger: After staging deployment
├── Job: integration-tests
│   ├── Wait for deployment (30s)
│   ├── Run health checks
│   ├── Run connectivity tests
│   ├── Run EPX tests (continue-on-error)
│   ├── Run performance tests
│   ├── Run security tests
│   ├── Generate test report
│   ├── Upload artifacts
│   └── Post to workflow summary
│
├── Job: smoke-test
│   ├── Wait for service (20s)
│   ├── Health check
│   ├── Stats endpoint
│   └── Log success
│
└── Job: load-test (manual only)
    ├── Run concurrency tests
    └── Upload load test results
```

### Test Artifacts

After each test run, artifacts are saved:

```
integration-test-results/
├── health-tests.log
├── connectivity-tests.log
├── epx-tests.log
├── performance-tests.log
├── security-tests.log
└── test-report.md
```

**Retention:** 30 days

---

## Interpreting Results

### All Tests Passed ✅
```
✅ All critical tests passed
```
**Action:** Deployment is healthy, safe to proceed

### Critical Tests Failed ❌
```
❌ Health check tests failed - this is critical
❌ Connectivity tests failed - this is critical
```
**Action:**
1. Check workflow logs
2. SSH to staging and check Docker logs
3. Verify deployment completed successfully
4. May need to redeploy or investigate issues

### EPX Tests Failed ⚠️
```
⚠️ EPX connectivity test failed
```
**Action:**
- Check EPX sandbox status
- Verify EPX credentials
- EPX may be experiencing issues (not critical for deployment)

### Performance Tests Slow ⚠️
```
⚠️ Response time 2.5s > 2.0s
```
**Action:**
- Monitor trends
- May indicate database performance issue
- Not critical but worth investigating

---

## Troubleshooting

### Tests Timeout

**Symptom:** Tests hang or timeout

**Possible causes:**
- Service not fully started
- Database connection issues
- Network connectivity problems

**Solutions:**
```bash
# Check service logs
ssh -i ~/.ssh/oracle-staging ubuntu@STAGING_IP
docker logs payment-staging -f

# Check if containers are running
docker ps

# Restart service
cd ~/payment-service
docker-compose restart
```

### Health Checks Fail

**Symptom:** `/cron/health` returns non-200

**Possible causes:**
- Service crashed
- Port mapping incorrect
- Firewall blocking

**Solutions:**
```bash
# Check service status
docker logs payment-staging --tail 100

# Check ports
docker ps | grep payment-staging

# Test locally on server
curl localhost:8081/cron/health

# Check firewall
sudo ufw status
```

### Database Connectivity Fails

**Symptom:** Stats endpoint errors

**Possible causes:**
- Database credentials wrong
- Oracle wallet missing
- Database not accessible

**Solutions:**
```bash
# Check wallet
ls -la ~/oracle-wallet/

# Check database env vars
docker exec payment-staging env | grep DB

# Test database from container
docker exec -it payment-staging bash
# Inside container, test connection
```

### EPX Tests Always Fail

**Symptom:** EPX tests consistently fail

**Solutions:**
```bash
# Skip EPX tests in CI
# Add to workflow:
env:
  SKIP_EPX_TESTS: "true"

# Or check EPX credentials
# Verify in .env on staging
```

---

## Best Practices

### 1. Run Tests After Every Deploy

✅ Automated via workflow
✅ Catches issues immediately
✅ Fast feedback loop

### 2. Monitor Test Trends

Track over time:
- Response times increasing?
- Tests becoming flaky?
- New failure patterns?

### 3. Keep Tests Fast

- Smoke tests < 1 minute
- Integration tests < 5 minutes
- Load tests only when needed

### 4. Make Tests Reliable

- Use proper timeouts
- Handle flaky external dependencies (EPX)
- Don't fail on warnings

### 5. Document Failures

When tests fail:
- Log to workflow summary
- Comment on commit
- Save artifacts
- Post to Slack (optional)

---

## Extending Tests

### Add New Integration Test

```go
// test/integration/staging_test.go

func TestMyNewFeature(t *testing.T) {
    cfg := LoadTestConfig()

    client := &http.Client{Timeout: 10 * time.Second}

    resp, err := client.Get(fmt.Sprintf("http://%s/my/endpoint", cfg.HTTPHost))
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }

    t.Log("✅ My new feature test passed")
}
```

### Add to Workflow

```yaml
# .github/workflows/integration-tests.yml

- name: Run new feature tests
  run: |
    go test -v -timeout 5m ./test/integration \
      -run "TestMyNewFeature" \
      2>&1 | tee feature-tests.log
```

---

## Metrics & Monitoring

### Key Metrics

Track these over time:

| Metric | Target | Alert If |
|--------|--------|----------|
| Health response time | < 500ms | > 2s |
| Stats response time | < 2s | > 5s |
| Test success rate | > 95% | < 90% |
| EPX availability | > 90% | < 80% |
| Concurrent requests | All succeed | > 10% fail |

### Dashboards (Optional)

Set up monitoring:
- Prometheus metrics from tests
- Grafana dashboard
- Alert manager for failures

---

## FAQ

**Q: Do tests run on every push to develop?**
A: Yes, after deployment completes successfully.

**Q: Can I skip tests?**
A: Not recommended. Tests ensure deployment health.

**Q: What if EPX tests fail?**
A: EPX tests don't fail the build. They're advisory only.

**Q: How long do tests take?**
A: Smoke tests ~30s, full suite ~5 minutes.

**Q: Can I run tests against production?**
A: Yes, but use manual trigger and be careful with load tests.

**Q: Do tests cost money?**
A: No, they run on free GitHub Actions runners.

---

## Related Documentation

- [Staging Lifecycle](./STAGING_LIFECYCLE.md) - Auto-create/destroy
- [Branching Strategy](./BRANCHING_STRATEGY.md) - Deployment workflow
- [Terraform Setup](../terraform/README.md) - Infrastructure

---

**Integration testing ensures every staging deployment is healthy before you rely on it.** 🧪✅
