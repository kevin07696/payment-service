# CI/CD Pipeline Visual Flow

## Current Pipeline Flow (With Issues)

```
┌─────────────────────────────────────────────────────────────────┐
│                          TRIGGER                                 │
│  Push to: develop | main    OR    Pull Request                  │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                    STEP 0: BUILD VERIFICATION                    │
│  Job: build-check                                               │
│  Duration: ~2 min                                               │
│  ❌ ISSUE: Go version 1.24 doesn't exist → startup_failure      │
├─────────────────────────────────────────────────────────────────┤
│  Actions:                                                       │
│  1. Checkout code                                               │
│  2. Setup Go 1.24 ❌ FAILS HERE                                 │
│  3. go build ./...                                              │
│  4. go vet ./...                                                │
└─────────────────────────────────────────────────────────────────┘
                              ↓ BLOCKED
┌─────────────────────────────────────────────────────────────────┐
│                    STEP 1: UNIT TESTS                           │
│  Job: unit-tests                                                │
│  ⚠️  Uses reusable workflow (also broken by Go version)         │
└─────────────────────────────────────────────────────────────────┘
                              ↓ BLOCKED
                   ALL REMAINING STEPS SKIPPED
```

---

## Fixed Pipeline Flow - Develop Branch

```
┌───────────────────────────────────────────────────────────────────┐
│                   TRIGGER: Push to develop                        │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 1: CODE QUALITY                                             │
├───────────────────────────────────────────────────────────────────┤
│ Step 1: Build Verification                                        │
│ ✅ Go 1.23, caching enabled, go build, go vet                     │
│ Duration: 2 min → 1 min (with caching)                            │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 2: UNIT TESTING                                             │
├───────────────────────────────────────────────────────────────────┤
│ Step 2: Unit Tests                                                │
│ ✅ All tests in internal/* (excluding tests/integration)          │
│ Duration: 3 min                                                   │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 3: BUILD & PACKAGE                                          │
├───────────────────────────────────────────────────────────────────┤
│ Step 3: Build Docker Image                                        │
│ ✅ Multi-stage build, push to ghcr.io                             │
│ Duration: 5 min → 3 min (with Docker layer caching)               │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 4: STAGING INFRASTRUCTURE                                   │
├───────────────────────────────────────────────────────────────────┤
│ Step 4: Provision Staging Infrastructure                          │
│ ✅ OCI compute instance + Oracle database                         │
│ ⚠️  OUTPUTS: oracle_cloud_host, db_host, db_port, etc.           │
│ Duration: 3 min                                                   │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 5: VALIDATE INFRASTRUCTURE (NEW)                            │
├───────────────────────────────────────────────────────────────────┤
│ Step 5: Validate Infrastructure Outputs                           │
│ ✅ Verify all outputs are not empty                               │
│ ❌ Fail early if outputs missing (prevent deployment failures)    │
│ Duration: 30 sec                                                  │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 6: DEPLOYMENT                                               │
├───────────────────────────────────────────────────────────────────┤
│ Step 6: Database Migrations + Deploy Application                  │
│ ✅ Run goose migrations → Start Docker container                  │
│ Duration: 2 min                                                   │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 7: INTEGRATION TESTING                                      │
├───────────────────────────────────────────────────────────────────┤
│ Step 7a: Wait for Service Ready (IMPROVED)                        │
│ ✅ Health check with 10 min timeout (was 5 min)                   │
│ ✅ Better error messages for debugging                            │
│ Duration: 1-5 min (depends on first deploy)                       │
├───────────────────────────────────────────────────────────────────┤
│ Step 7b: Pre-flight Checks (NEW)                                  │
│ ✅ Validate SERVICE_URL not empty                                 │
│ ✅ Verify service connectivity before running tests               │
│ Duration: 30 sec                                                  │
├───────────────────────────────────────────────────────────────────┤
│ Step 7c: Run Integration Tests                                    │
│ ✅ go test ./tests/integration/... -tags=integration              │
│ ✅ JSON output uploaded as artifact                               │
│ Duration: 10-15 min                                               │
└───────────────────────────────────────────────────────────────────┘
                                ↓
                    ┌───────────┴───────────┐
                    ↓                       ↓
        ┌─────────────────────┐  ┌─────────────────────┐
        │  TESTS PASS ✅      │  │  TESTS FAIL ❌      │
        └─────────────────────┘  └─────────────────────┘
                    ↓                       ↓
        ┌─────────────────────┐  ┌─────────────────────┐
        │ Keep staging alive  │  │ Cleanup staging     │
        │ for manual testing  │  │ infrastructure      │
        │ (24h TTL)           │  │ (destroy OCI)       │
        └─────────────────────┘  └─────────────────────┘

Total Time: 20-25 minutes (success path)
```

---

## Fixed Pipeline Flow - Main Branch (Production)

```
┌───────────────────────────────────────────────────────────────────┐
│                   TRIGGER: Push to main                           │
│  (Usually after merging develop → main)                           │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 1-3: BUILD & TEST (Same as develop)                         │
├───────────────────────────────────────────────────────────────────┤
│ Step 1: Build Verification                                        │
│ Step 2: Unit Tests                                                │
│ Step 3: Build Docker Image                                        │
│ Duration: 10 min                                                  │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 7: PRODUCTION DEPLOYMENT                                    │
├───────────────────────────────────────────────────────────────────┤
│ Step 6: Deploy to Production                                      │
│ ⚠️  Currently placeholder (TODO)                                  │
│ Future: Deploy to OCI production or other cloud                   │
│ Duration: 5 min (when implemented)                                │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 8: PRODUCTION VALIDATION                                    │
├───────────────────────────────────────────────────────────────────┤
│ Step 7: Production Smoke Tests                                    │
│ ✅ Health check                                                    │
│ ✅ Database connectivity                                           │
│ ✅ Metrics endpoint                                                │
│ ✅ Critical read operation (no side effects)                       │
│ Duration: 2 min                                                   │
│ ❌ If fails → Auto-rollback triggered                             │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 9: CLEANUP                                                  │
├───────────────────────────────────────────────────────────────────┤
│ Step 8: Cleanup Staging Infrastructure                            │
│ ✅ Destroy staging environment (free up OCI quota)                │
│ Duration: 2 min                                                   │
└───────────────────────────────────────────────────────────────────┘

Total Time: 15-20 minutes
```

---

## Pull Request Validation Flow (Recommended - To Be Implemented)

```
┌───────────────────────────────────────────────────────────────────┐
│              TRIGGER: Pull Request (any branch → develop/main)    │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 1: FAST FEEDBACK                                            │
├───────────────────────────────────────────────────────────────────┤
│ Job: pr-quality-check (PARALLEL)                                  │
│ ├─ Code linting (golangci-lint)                                   │
│ ├─ go vet                                                         │
│ └─ go build                                                       │
│ Duration: 2 min                                                   │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 2: UNIT TESTS                                               │
├───────────────────────────────────────────────────────────────────┤
│ Job: pr-unit-tests                                                │
│ ✅ Unit tests only (no integration tests)                         │
│ ✅ Coverage report posted to PR                                   │
│ Duration: 3 min                                                   │
└───────────────────────────────────────────────────────────────────┘
                                ↓
┌───────────────────────────────────────────────────────────────────┐
│ STAGE 3: DOCKER BUILD (Optional)                                  │
├───────────────────────────────────────────────────────────────────┤
│ Job: pr-build-check                                               │
│ ✅ Verify Docker image builds                                     │
│ ❌ Don't push to registry (save time/bandwidth)                   │
│ Duration: 3 min                                                   │
└───────────────────────────────────────────────────────────────────┘

Total Time: 5-8 minutes
❌ NO infrastructure provisioning
❌ NO integration tests (run after merge to develop)

Benefit: Fast feedback, low cost, encourages frequent PRs
```

---

## Test Execution Flow by Stage

### Unit Tests (Stage 2)

```
go test $(go list ./... | grep -v /tests/integration)
    ↓
Tests Run (150+ test cases):
├─ internal/domain/*_test.go
│  ├─ payment_method_test.go (card validation, etc.)
│  ├─ transaction_test.go (state machine)
│  ├─ merchant_test.go
│  ├─ subscription_test.go
│  └─ chargeback_test.go
├─ internal/services/*_test.go
│  ├─ payment/payment_service_test.go (with mocks)
│  ├─ payment/validation_test.go
│  ├─ payment_method/payment_method_service_test.go
│  └─ subscription/subscription_service_test.go
├─ internal/adapters/*_test.go
│  ├─ epx/server_post_adapter_test.go
│  └─ epx/server_post_error_test.go
└─ internal/handlers/*_test.go
   └─ payment/browser_post_callback_handler_test.go

Duration: <1 minute
Failures: Immediate (no infrastructure needed)
```

### Integration Tests (Stage 7)

```
go test ./tests/integration/... -tags=integration
    ↓
Tests Run (80+ test cases):
├─ tests/integration/auth/
│  ├─ jwt_auth_test.go
│  ├─ epx_callback_auth_test.go
│  └─ cron_auth_test.go
├─ tests/integration/payment/
│  ├─ payment_service_critical_test.go ⭐ (decline handling)
│  ├─ browser_post_workflow_test.go
│  ├─ server_post_workflow_test.go
│  ├─ browser_post_idempotency_test.go
│  ├─ server_post_idempotency_test.go
│  └─ payment_ach_verification_test.go
├─ tests/integration/payment_method/
│  └─ payment_method_test.go
├─ tests/integration/subscription/
│  └─ recurring_billing_test.go
├─ tests/integration/cron/
│  └─ ach_verification_cron_test.go
└─ tests/integration/connect/
   └─ connect_protocol_test.go

Duration: 10-15 minutes
Requires: Deployed service + EPX sandbox access
Failures: May need to check service logs
```

### Smoke Tests (Stage 8 - Production)

```
go test ./tests/smoke/... -tags=smoke
    ↓
Tests Run (5-10 test cases):
├─ health_test.go
│  └─ GET /cron/health
├─ database_test.go
│  └─ SELECT 1 (verify DB connectivity)
├─ secrets_test.go
│  └─ Verify EPX credentials loaded
├─ metrics_test.go
│  └─ GET /metrics (Prometheus endpoint)
└─ payment_retrieval_test.go
   └─ GET /payment/{known_test_id}

Duration: 2 minutes
Failures: Trigger auto-rollback
```

---

## Error Flow - What Happens When Things Fail

### Build Verification Fails

```
build-check job fails
    ↓
❌ All downstream jobs are SKIPPED
❌ No unit tests run
❌ No Docker image built
❌ No deployment attempted
    ↓
Developer is notified immediately
    ↓
Fix code → Push again → Pipeline restarts
```

### Unit Tests Fail

```
unit-tests job fails
    ↓
❌ Docker build is SKIPPED
❌ No deployment attempted
    ↓
Fast feedback (within 5 minutes)
    ↓
Fix tests → Push again → Pipeline restarts from build-check
```

### Infrastructure Provisioning Fails

```
ensure-staging-infrastructure job fails
    ↓
❌ deploy-staging is SKIPPED
❌ integration-tests is SKIPPED
    ↓
cleanup-staging-on-failure job runs
    ↓
✅ Destroys any partially-created infrastructure
    ↓
Developer reviews OCI/Terraform logs
```

### Integration Tests Fail

```
integration-tests job fails
    ↓
cleanup-staging-on-failure job runs
    ↓
✅ Destroys staging infrastructure
    ↓
❌ Main branch deployment is BLOCKED
    ↓
Developer can access staging logs before cleanup
    ↓
Fix code → Push to develop → Full staging deployment again
```

### Production Smoke Tests Fail

```
production-smoke-tests job fails
    ↓
Auto-rollback triggered (future)
    ↓
✅ Reverts to previous Docker image
    ↓
❌ Incident created
❌ On-call engineer notified
    ↓
Investigate production logs
    ↓
Hotfix deployed or full rollback to stable version
```

---

## Cleanup Strategy

### Staging Environment Lifecycle

```
┌────────────────────────────────────────────────────────────────┐
│ SCENARIO 1: Successful Deployment                              │
├────────────────────────────────────────────────────────────────┤
│ Deploy to staging → Tests pass → Keep alive for 24h           │
│                                                                │
│ Why: Allows manual testing, debugging                         │
│ Cleanup: Manual via manual-infrastructure.yml workflow        │
│         OR automatic via nightly cron                          │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│ SCENARIO 2: Failed Deployment or Tests                         │
├────────────────────────────────────────────────────────────────┤
│ Deploy to staging → Tests fail → Cleanup immediately          │
│                                                                │
│ Why: Free OCI quota, prevent dangling resources               │
│ Cleanup: Automatic via cleanup-staging-on-failure job         │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│ SCENARIO 3: Production Deployment                              │
├────────────────────────────────────────────────────────────────┤
│ Production deployed → Staging no longer needed → Cleanup      │
│                                                                │
│ Why: Staging validated code already in production             │
│ Cleanup: Automatic via cleanup-staging-after-production job   │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│ SCENARIO 4: Workflow Cancelled                                 │
├────────────────────────────────────────────────────────────────┤
│ Developer cancels workflow → Infrastructure may be dangling   │
│                                                                │
│ Why: Cancelled jobs don't trigger cleanup                     │
│ Fix: Update cleanup job to also trigger on 'cancelled' state  │
│ Cleanup: Improved cleanup-staging-on-failure job              │
└────────────────────────────────────────────────────────────────┘
```

---

## Cost Optimization Strategy

### Current Costs

```
Per Develop Push:
├─ GitHub Actions Compute:
│  ├─ Build + Unit Tests: ~5 min → $0.008 (Linux runner)
│  ├─ Docker Build: ~5 min → $0.008
│  ├─ Integration Tests: ~15 min → $0.024
│  └─ Total: ~25 min → $0.04 per run
│
├─ OCI Infrastructure (Free Tier):
│  ├─ Compute: 2 ARM instances (always-free)
│  ├─ Database: 2 Oracle DBs (always-free)
│  └─ Cost: $0 (within free tier limits)
│
└─ GitHub Container Registry:
   ├─ Storage: ~500MB per image
   ├─ Free: 500MB storage + unlimited public pulls
   └─ Cost: $0

Monthly Cost (20 develop pushes):
├─ GitHub Actions: ~$0.80/month
├─ OCI: $0 (free tier)
└─ Total: ~$1/month
```

### Optimizations (After Fixes)

```
With Caching:
├─ Go module caching: Save ~1 min per build
├─ Docker layer caching: Save ~2 min per build
├─ New total: ~20 min per run → $0.032 per run
└─ Monthly savings: ~$0.16/month (20% reduction)

With PR Workflow Separation:
├─ PR validation: 5 min (no infra) → $0.008 per PR
├─ Develop deployment: 20 min → $0.032 per merge
├─ Assuming 50 PRs, 20 merges per month:
│  ├─ PRs: 50 × $0.008 = $0.40
│  ├─ Merges: 20 × $0.032 = $0.64
│  └─ Total: $1.04/month
└─ But: Faster feedback, fewer wasted full deploys

Net Result: Similar cost, better developer experience
```

---

## Key Metrics to Track

### Pipeline Health

```
┌─────────────────────────┬──────────┬────────────┐
│ Metric                  │ Current  │ Target     │
├─────────────────────────┼──────────┼────────────┤
│ Success Rate            │ ~20%     │ >95%       │
│ Average Duration        │ N/A      │ <15 min    │
│ Time to Deploy          │ N/A      │ <30 min    │
│ Staging Uptime Cost     │ Unknown  │ <10h/month │
│ Integration Test Flake  │ Unknown  │ <5%        │
└─────────────────────────┴──────────┴────────────┘
```

### After Fixes (Expected)

```
┌─────────────────────────┬────────────┐
│ Metric                  │ Expected   │
├─────────────────────────┼────────────┤
│ Success Rate            │ 90-95%     │
│ Average Duration        │ 22 min     │
│ Time to Deploy          │ 25 min     │
│ Staging Uptime Cost     │ 8h/month   │
│ Integration Test Flake  │ 2-3%       │
└─────────────────────────┴────────────┘
```

---

## Related Diagrams

See also:
- `PIPELINE_ANALYSIS.md` - Issue identification
- `RECOMMENDED_FIXES.md` - Implementation steps
- `PIPELINE_STRUCTURE.md` - Job organization
- `TEST_STRATEGY.md` - Test execution details
