# Recommended CI/CD Pipeline Fixes

## Critical Fixes (Immediate Action Required)

### Fix 1: Correct Go Version

**Priority:** CRITICAL - Blocks all workflows

**Files to Modify:**
- `.github/workflows/ci-cd.yml`
- `go.mod`

**Changes for ci-cd.yml:**

```yaml
# Line 27 - build-check job
- name: Setup Go
  uses: actions/setup-go@v5
  with:
    go-version: '1.23'  # Changed from '1.24'
    cache: true         # Added: Enable Go module caching

# Line 47 - unit-tests job (reusable workflow parameter)
unit-tests:
  name: 1. Unit Tests
  needs: build-check
  uses: kevin07696/deployment-workflows/.github/workflows/go-test.yml@main
  with:
    go-version: '1.23'  # Changed from '1.24'

# Line 104 - integration-tests job
- name: Setup Go
  uses: actions/setup-go@v5
  with:
    go-version: '1.23'  # Changed from '1.24'
    cache: true         # Added: Enable Go module caching

# Line 188 - production-smoke-tests job
- name: Setup Go
  uses: actions/setup-go@v5
  with:
    go-version: '1.23'  # Changed from '1.24'
    cache: true         # Added: Enable Go module caching
```

**Changes for go.mod:**

```go
// Line 3
go 1.23.0  // Changed from 1.24.0
```

**Validation:**
```bash
# After changes, verify:
go version  # Should show go1.23.x
go mod tidy
go build ./...
```

---

### Fix 2: Verify and Document Infrastructure Outputs

**Priority:** HIGH - Required for staging deployments

**Investigation Required:**

The `infrastructure-lifecycle.yml` reusable workflow must export these outputs:

```yaml
# In kevin07696/deployment-workflows/.github/workflows/infrastructure-lifecycle.yml
# Add/verify outputs section:

outputs:
  oracle_cloud_host:
    description: "Public IP or hostname of OCI compute instance"
    value: ${{ jobs.provision.outputs.oracle_cloud_host }}
  db_host:
    description: "Database host endpoint"
    value: ${{ jobs.provision.outputs.db_host }}
  db_port:
    description: "Database port (default: 1521 for Oracle)"
    value: ${{ jobs.provision.outputs.db_port }}
  db_service_name:
    description: "Oracle database service name"
    value: ${{ jobs.provision.outputs.db_service_name }}
  db_user:
    description: "Database user for migrations"
    value: ${{ jobs.provision.outputs.db_user }}
```

**Changes for ci-cd.yml:**

Add validation step before deployment:

```yaml
# Insert after line 79 (before deploy-staging job)

validate-infrastructure-outputs:
  name: Validate Infrastructure Outputs
  needs: ensure-staging-infrastructure
  if: github.ref == 'refs/heads/develop' && github.event_name == 'push'
  runs-on: ubuntu-latest
  steps:
    - name: Validate required outputs
      run: |
        ORACLE_HOST="${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}"
        DB_HOST="${{ needs.ensure-staging-infrastructure.outputs.db_host }}"
        DB_PORT="${{ needs.ensure-staging-infrastructure.outputs.db_port }}"
        DB_SERVICE="${{ needs.ensure-staging-infrastructure.outputs.db_service_name }}"
        DB_USER="${{ needs.ensure-staging-infrastructure.outputs.db_user }}"

        echo "🔍 Validating infrastructure outputs..."

        if [ -z "$ORACLE_HOST" ]; then
          echo "❌ ERROR: oracle_cloud_host is empty"
          exit 1
        fi
        if [ -z "$DB_HOST" ]; then
          echo "❌ ERROR: db_host is empty"
          exit 1
        fi
        if [ -z "$DB_PORT" ]; then
          echo "❌ ERROR: db_port is empty"
          exit 1
        fi
        if [ -z "$DB_SERVICE" ]; then
          echo "❌ ERROR: db_service_name is empty"
          exit 1
        fi
        if [ -z "$DB_USER" ]; then
          echo "❌ ERROR: db_user is empty"
          exit 1
        fi

        echo "✅ All infrastructure outputs validated:"
        echo "  - Oracle Host: $ORACLE_HOST"
        echo "  - DB Host: $DB_HOST"
        echo "  - DB Port: $DB_PORT"
        echo "  - DB Service: $DB_SERVICE"
        echo "  - DB User: $DB_USER"

# Update deploy-staging to depend on validation
deploy-staging:
  name: 3. Database Migrations + 4. Deploy Application
  needs: [ensure-staging-infrastructure, validate-infrastructure-outputs]  # Changed
  # ... rest unchanged
```

---

### Fix 3: Improve Health Check Robustness

**Priority:** MEDIUM - Prevents false negatives

**Replace lines 106-118 in ci-cd.yml:**

```yaml
- name: Wait for service to be ready
  run: |
    echo "⏳ Waiting for service to be ready..."
    ORACLE_HOST="${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}"
    HEALTH_URL="http://${ORACLE_HOST}:8081/cron/health"

    # Validate URL before attempting connection
    if [ -z "$ORACLE_HOST" ]; then
      echo "❌ ERROR: Oracle host is empty - cannot perform health check"
      exit 1
    fi

    echo "🔍 Health check endpoint: $HEALTH_URL"

    # Extended timeout: 40 attempts × 15 seconds = 10 minutes
    # Accounts for:
    # - Docker image pull (1-2 min)
    # - Database migrations (1-2 min)
    # - Application startup (30-60 sec)
    # - Buffer for network latency (5 min)
    MAX_ATTEMPTS=40
    SLEEP_INTERVAL=15

    for i in $(seq 1 $MAX_ATTEMPTS); do
      echo "Attempt $i/$MAX_ATTEMPTS..."

      if curl -f -s -m 5 "$HEALTH_URL" > /dev/null 2>&1; then
        echo "✅ Service is ready!"

        # Verify service is actually responding with valid health status
        HEALTH_RESPONSE=$(curl -s "$HEALTH_URL")
        echo "📋 Health check response: $HEALTH_RESPONSE"
        exit 0
      fi

      if [ $i -eq $MAX_ATTEMPTS ]; then
        echo "❌ Service did not become ready after $((MAX_ATTEMPTS * SLEEP_INTERVAL)) seconds"
        echo "🔍 Troubleshooting info:"
        echo "  - Health endpoint: $HEALTH_URL"
        echo "  - Verify Oracle host is accessible: ping $ORACLE_HOST"
        echo "  - Check Docker container status on instance"
        echo "  - Review application logs"
        exit 1
      fi

      sleep $SLEEP_INTERVAL
    done
```

**Benefits:**
- 10-minute timeout accommodates slow first deployments
- URL validation prevents cryptic curl failures
- Better error messages for debugging
- Connection timeout per attempt (`-m 5`) prevents hanging

---

### Fix 4: Add Pre-Flight Checks to Integration Tests

**Priority:** MEDIUM - Better debugging experience

**Replace lines 120-130 in ci-cd.yml:**

```yaml
- name: Pre-flight checks
  env:
    SERVICE_URL: http://${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}
  run: |
    echo "🔍 Running pre-flight checks..."

    # Validate environment variables
    if [ -z "$SERVICE_URL" ]; then
      echo "❌ ERROR: SERVICE_URL is empty"
      exit 1
    fi

    if [ -z "${{ secrets.EPX_MAC_STAGING }}" ]; then
      echo "⚠️  WARNING: EPX_MAC_STAGING secret is empty"
    fi

    if [ -z "${{ secrets.EPX_CUST_NBR }}" ]; then
      echo "⚠️  WARNING: EPX_CUST_NBR secret is empty"
    fi

    # Verify service connectivity
    echo "📡 Testing connectivity to: $SERVICE_URL:8081"
    if ! curl -f -s -m 10 "$SERVICE_URL:8081/cron/health" > /dev/null 2>&1; then
      echo "❌ ERROR: Cannot reach service health endpoint"
      echo "🔍 Attempted URL: $SERVICE_URL:8081/cron/health"
      exit 1
    fi

    echo "✅ Pre-flight checks passed"

- name: Run integration tests
  env:
    SERVICE_URL: http://${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}
    EPX_MAC_STAGING: ${{ secrets.EPX_MAC_STAGING }}
    EPX_CUST_NBR: ${{ secrets.EPX_CUST_NBR }}
    EPX_MERCH_NBR: ${{ secrets.EPX_MERCH_NBR }}
    EPX_DBA_NBR: ${{ secrets.EPX_DBA_NBR }}
    EPX_TERMINAL_NBR: ${{ secrets.EPX_TERMINAL_NBR }}
  run: |
    echo "🧪 Running integration tests against deployed service..."
    echo "📍 Service URL: $SERVICE_URL"

    # Run tests with structured output
    go test ./tests/integration/... \
      -v \
      -tags=integration \
      -timeout=10m \
      -json | tee test-results.json

    # Parse results for summary
    TEST_STATUS=$?

    if [ $TEST_STATUS -eq 0 ]; then
      echo "✅ All integration tests passed"
    else
      echo "❌ Integration tests failed with exit code: $TEST_STATUS"
      echo "📋 Review test-results.json for details"
      exit $TEST_STATUS
    fi

- name: Upload test results
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: integration-test-results
    path: test-results.json
    retention-days: 30
```

**Benefits:**
- Catch configuration issues before running tests
- JSON output for programmatic parsing
- Artifact upload for post-mortem analysis
- Reduced timeout from 15m to 10m (faster failure detection)

---

### Fix 5: Handle Workflow Cancellation in Cleanup

**Priority:** MEDIUM - Prevent resource leaks

**Replace lines 144-158 in ci-cd.yml:**

```yaml
cleanup-staging-on-failure:
  name: Cleanup Staging on Failure
  needs: [ensure-staging-infrastructure, deploy-staging, integration-tests]
  if: |
    always() &&
    github.ref == 'refs/heads/develop' &&
    github.event_name == 'push' &&
    (needs.ensure-staging-infrastructure.result == 'failure' ||
     needs.ensure-staging-infrastructure.result == 'cancelled' ||
     needs.deploy-staging.result == 'failure' ||
     needs.deploy-staging.result == 'cancelled' ||
     needs.integration-tests.result == 'failure' ||
     needs.integration-tests.result == 'cancelled')
  uses: kevin07696/deployment-workflows/.github/workflows/infrastructure-lifecycle.yml@main
  secrets: inherit
  with:
    action: destroy
    environment: staging
```

**Changes:**
- Added `cancelled` state to trigger cleanup
- Prevents dangling resources when workflows are manually cancelled

---

## Performance Optimizations

### Fix 6: Add Go Module Caching to Build Check

**Replace lines 21-39 in ci-cd.yml:**

```yaml
- name: Checkout code
  uses: actions/checkout@v4

- name: Setup Go
  uses: actions/setup-go@v5
  with:
    go-version: '1.23'
    cache: true  # Added: Enables automatic caching of Go modules

- name: Cache Go build cache
  uses: actions/cache@v4
  with:
    path: |
      ~/.cache/go-build
      ~/go/pkg/mod
    key: ${{ runner.os }}-go-build-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-go-build-

- name: Verify all packages build
  run: |
    echo "🔨 Building all packages..."
    go build ./...
    echo "✅ All packages build successfully"

- name: Run go vet
  run: |
    echo "🔍 Running go vet..."
    go vet ./...
    echo "✅ go vet passed"
```

**Benefits:**
- Reduces build time from 2-3 minutes to 30-60 seconds
- Cached modules persist between workflow runs
- Automatic cache invalidation when go.sum changes

---

### Fix 7: Add Docker Layer Caching (if using reusable workflow)

**Note to check in `kevin07696/deployment-workflows/.github/workflows/go-build-docker.yml`:**

Ensure the reusable workflow includes Docker layer caching:

```yaml
- name: Set up Docker Buildx
  uses: docker/setup-buildx-action@v3

- name: Build and push
  uses: docker/build-push-action@v5
  with:
    context: .
    push: true
    tags: |
      ghcr.io/${{ github.repository }}:${{ github.sha }}
      ghcr.io/${{ github.repository }}:latest
    cache-from: type=gha
    cache-to: type=gha,mode=max
```

**Benefits:**
- Reduces Docker build time from 3-5 minutes to 1-2 minutes
- Reuses unchanged layers across builds
- Particularly beneficial for Go module layer

---

## Quality of Life Improvements

### Fix 8: Consistent Job Naming

**Replace job names throughout ci-cd.yml:**

```yaml
build-check:
  name: "Step 1: Build Verification"  # Changed from "0. Build Verification"

unit-tests:
  name: "Step 2: Unit Tests"          # Changed from "1. Unit Tests"

build-docker-image:
  name: "Step 3: Build Docker Image"  # Changed from "2. Build Docker Image"

validate-infrastructure-outputs:
  name: "Step 4: Validate Infrastructure"  # New job

deploy-staging:
  name: "Step 5: Database Migrations"  # Changed from "3. Database Migrations + 4. Deploy Application"

deploy-staging-app:  # Split into separate job
  name: "Step 6: Deploy Application"

integration-tests:
  name: "Step 7: Integration Tests"   # Changed from "5. Integration Tests"
```

**Rationale:**
- Clear sequential numbering
- Separate concerns (migrations vs deployment)
- Easier to track progress in GitHub Actions UI

---

### Fix 9: Add Deployment Summary

**Add to end of integration-tests job (after line 141):**

```yaml
- name: Deployment summary
  if: always()
  run: |
    cat << EOF >> $GITHUB_STEP_SUMMARY
    ## Staging Deployment Summary

    **Environment:** Staging
    **Branch:** \`${{ github.ref_name }}\`
    **Commit:** \`${{ github.sha }}\`
    **Triggered by:** ${{ github.actor }}

    ### Infrastructure
    - **Oracle Host:** ${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}
    - **Database:** ${{ needs.ensure-staging-infrastructure.outputs.db_host }}:${{ needs.ensure-staging-infrastructure.outputs.db_port }}

    ### Test Results
    - **Status:** ${{ job.status }}
    - **Integration Tests:** $([ "${{ job.status }}" == "success" ] && echo "✅ PASSED" || echo "❌ FAILED")

    ### Access
    - **Service URL:** http://${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}:8081
    - **Health Check:** http://${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}:8081/cron/health

    ---

    **Note:** Staging environment will remain active until next deployment or manual cleanup.
    EOF
```

---

## Complete Updated Workflow Snippet

Here's the critical section with all fixes applied:

```yaml
jobs:
  # ===== STEP 1: BUILD VERIFICATION =====
  build-check:
    name: "Step 1: Build Verification"
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Cache Go build cache
        uses: actions/cache@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
          key: ${{ runner.os }}-go-build-${{ hashFiles('**/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-build-

      - name: Verify all packages build
        run: |
          echo "🔨 Building all packages..."
          go build ./...
          echo "✅ All packages build successfully"

      - name: Run go vet
        run: |
          echo "🔍 Running go vet..."
          go vet ./...
          echo "✅ go vet passed"

  # ===== STEP 2: UNIT TESTS =====
  unit-tests:
    name: "Step 2: Unit Tests"
    needs: build-check
    uses: kevin07696/deployment-workflows/.github/workflows/go-test.yml@main
    with:
      go-version: '1.23'

  # ===== STEP 3: BUILD DOCKER IMAGE =====
  build-docker-image:
    name: "Step 3: Build Docker Image"
    needs: unit-tests
    uses: kevin07696/deployment-workflows/.github/workflows/go-build-docker.yml@main
    with:
      service-name: payment-service

  # ===== STAGING DEPLOYMENT PIPELINE =====
  ensure-staging-infrastructure:
    name: "Step 4: Provision Staging Infrastructure"
    needs: build-docker-image
    if: github.ref == 'refs/heads/develop' && github.event_name == 'push'
    uses: kevin07696/deployment-workflows/.github/workflows/infrastructure-lifecycle.yml@main
    secrets: inherit
    with:
      action: create
      environment: staging

  validate-infrastructure-outputs:
    name: "Step 5: Validate Infrastructure"
    needs: ensure-staging-infrastructure
    if: github.ref == 'refs/heads/develop' && github.event_name == 'push'
    runs-on: ubuntu-latest
    steps:
      - name: Validate required outputs
        run: |
          # ... validation script from Fix 2 ...

  deploy-staging:
    name: "Step 6: Deploy Application + Migrations"
    needs: [ensure-staging-infrastructure, validate-infrastructure-outputs]
    if: github.ref == 'refs/heads/develop' && github.event_name == 'push'
    uses: kevin07696/deployment-workflows/.github/workflows/deploy-oracle-staging.yml@main
    secrets: inherit
    with:
      service-name: payment-service
      oracle-cloud-host: ${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}
      db-host: ${{ needs.ensure-staging-infrastructure.outputs.db_host }}
      db-port: ${{ needs.ensure-staging-infrastructure.outputs.db_port }}
      db-service-name: ${{ needs.ensure-staging-infrastructure.outputs.db_service_name }}
      db-user: ${{ needs.ensure-staging-infrastructure.outputs.db_user }}

  integration-tests:
    name: "Step 7: Integration Tests"
    needs: [ensure-staging-infrastructure, deploy-staging]
    if: github.ref == 'refs/heads/develop' && github.event_name == 'push'
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Wait for service to be ready
        run: |
          # ... improved health check from Fix 3 ...

      - name: Pre-flight checks
        run: |
          # ... pre-flight checks from Fix 4 ...

      - name: Run integration tests
        run: |
          # ... improved test execution from Fix 4 ...

      - name: Upload test results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: integration-test-results
          path: test-results.json
          retention-days: 30

      - name: Deployment summary
        if: always()
        run: |
          # ... summary from Fix 9 ...

  cleanup-staging-on-failure:
    name: Cleanup Staging on Failure/Cancellation
    needs: [ensure-staging-infrastructure, deploy-staging, integration-tests]
    if: |
      always() &&
      github.ref == 'refs/heads/develop' &&
      github.event_name == 'push' &&
      (needs.ensure-staging-infrastructure.result == 'failure' ||
       needs.ensure-staging-infrastructure.result == 'cancelled' ||
       needs.deploy-staging.result == 'failure' ||
       needs.deploy-staging.result == 'cancelled' ||
       needs.integration-tests.result == 'failure' ||
       needs.integration-tests.result == 'cancelled')
    uses: kevin07696/deployment-workflows/.github/workflows/infrastructure-lifecycle.yml@main
    secrets: inherit
    with:
      action: destroy
      environment: staging
```

---

## Implementation Checklist

- [ ] Update Go version to 1.23 in all 4 locations in ci-cd.yml
- [ ] Update go.mod to specify Go 1.23.0
- [ ] Add Go module caching to build-check job
- [ ] Add validate-infrastructure-outputs job
- [ ] Update deploy-staging dependencies
- [ ] Improve health check with extended timeout
- [ ] Add pre-flight checks to integration tests
- [ ] Add test results upload artifact
- [ ] Update cleanup job to handle cancellation
- [ ] Improve job naming consistency
- [ ] Add deployment summary step
- [ ] Verify infrastructure-lifecycle.yml exports required outputs
- [ ] Test workflow on feature branch before merging to develop

---

## Testing Strategy

### Test the Fixes

1. **Create test branch:**
   ```bash
   git checkout -b fix/cicd-pipeline-issues
   ```

2. **Apply critical fixes:**
   - Update Go version in ci-cd.yml and go.mod
   - Add validation job
   - Improve health check

3. **Push to test branch:**
   ```bash
   git add .github/workflows/ci-cd.yml go.mod
   git commit -m "fix: Correct Go version and improve CI/CD robustness"
   git push origin fix/cicd-pipeline-issues
   ```

4. **Verify in GitHub Actions UI:**
   - Check that build-check job succeeds
   - Verify Go 1.23 is used
   - Confirm caching is working

5. **Test staging deployment:**
   ```bash
   git checkout develop
   git merge fix/cicd-pipeline-issues
   git push origin develop
   ```

6. **Monitor deployment:**
   - Watch infrastructure provisioning
   - Verify validation job catches any output issues
   - Confirm health check completes successfully
   - Check integration tests run against deployed service

---

## Rollback Plan

If fixes cause issues:

1. **Immediate rollback:**
   ```bash
   git revert <commit-sha>
   git push origin develop
   ```

2. **Manual cleanup if needed:**
   - Use manual-infrastructure.yml workflow
   - Select "destroy" action for staging environment

3. **Review logs:**
   - Check GitHub Actions workflow logs
   - Review OCI console for infrastructure state
   - Examine application logs on compute instance
