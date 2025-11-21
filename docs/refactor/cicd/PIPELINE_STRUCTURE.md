# CI/CD Pipeline Structure Recommendations

## Current Pipeline Overview

### Workflow Stages

```
┌─────────────────────────────────────────────────────────────────┐
│                     COMMON STAGES (All Branches)                 │
├─────────────────────────────────────────────────────────────────┤
│ 1. Build Verification → 2. Unit Tests → 3. Build Docker Image  │
└─────────────────────────────────────────────────────────────────┘
                              ↓
                    ┌─────────┴─────────┐
                    ↓                   ↓
        ┌────────────────────┐  ┌──────────────────┐
        │  DEVELOP BRANCH    │  │   MAIN BRANCH    │
        │  (Staging)         │  │   (Production)   │
        └────────────────────┘  └──────────────────┘
                ↓                        ↓
    ┌───────────────────────┐    ┌─────────────────┐
    │ 4. Provision Infra    │    │ 6. Deploy Prod  │
    │ 5. Deploy Staging     │    │ 7. Smoke Tests  │
    │ 6. Integration Tests  │    │ 8. Cleanup Stage│
    │ 7. Cleanup on Fail    │    └─────────────────┘
    └───────────────────────┘
```

---

## Recommended Pipeline Structure

### 1. Improved Job Naming Convention

**Current Issues:**
- Inconsistent numbering (0, 1, 2, then 3+4 combined)
- Unclear separation between stages
- Hard to identify parallel vs sequential jobs

**Recommended Approach:**

Use semantic naming with clear stage indicators:

```yaml
jobs:
  # ============================================================================
  # STAGE 1: CODE QUALITY & VALIDATION
  # Runs on: All branches (main, develop, PR branches)
  # Purpose: Fast feedback on code quality issues
  # ============================================================================

  code-quality-build:
    name: "Quality Gate: Build Verification"
    # ...

  code-quality-vet:
    name: "Quality Gate: Go Vet"
    # ...

  code-quality-lint:
    name: "Quality Gate: Linting"
    # Optional: Add golangci-lint
    # ...

  # ============================================================================
  # STAGE 2: UNIT TESTING
  # Runs on: All branches
  # Purpose: Verify business logic without external dependencies
  # ============================================================================

  test-unit:
    name: "Tests: Unit Tests"
    needs: [code-quality-build]
    # ...

  test-unit-coverage:
    name: "Tests: Coverage Report"
    needs: [test-unit]
    # Optional: Generate coverage report
    # ...

  # ============================================================================
  # STAGE 3: BUILD & PACKAGE
  # Runs on: All branches
  # Purpose: Create deployable artifacts
  # ============================================================================

  build-docker:
    name: "Build: Docker Image"
    needs: [test-unit]
    # ...

  build-scan-security:
    name: "Build: Security Scan"
    needs: [build-docker]
    # Optional: Trivy/Snyk container scanning
    # ...

  # ============================================================================
  # STAGE 4: STAGING DEPLOYMENT (develop branch only)
  # Runs on: develop branch push
  # Purpose: Deploy to ephemeral staging environment for integration testing
  # ============================================================================

  staging-provision-infra:
    name: "Staging: Provision Infrastructure"
    if: github.ref == 'refs/heads/develop' && github.event_name == 'push'
    needs: [build-docker]
    # ...

  staging-validate-outputs:
    name: "Staging: Validate Infrastructure"
    needs: [staging-provision-infra]
    # ...

  staging-deploy-database:
    name: "Staging: Database Migrations"
    needs: [staging-validate-outputs]
    # ...

  staging-deploy-app:
    name: "Staging: Deploy Application"
    needs: [staging-deploy-database]
    # ...

  staging-health-check:
    name: "Staging: Health Check"
    needs: [staging-deploy-app]
    # ...

  # ============================================================================
  # STAGE 5: INTEGRATION TESTING (staging environment)
  # Runs on: develop branch after successful staging deployment
  # Purpose: Validate end-to-end workflows against real infrastructure
  # ============================================================================

  test-integration:
    name: "Tests: Integration Tests"
    needs: [staging-health-check]
    # ...

  test-integration-report:
    name: "Tests: Integration Test Report"
    needs: [test-integration]
    # ...

  # ============================================================================
  # STAGE 6: STAGING CLEANUP
  # Runs on: Failure or after production deployment
  # Purpose: Prevent resource leaks
  # ============================================================================

  staging-cleanup-on-failure:
    name: "Staging: Cleanup on Failure"
    needs: [staging-provision-infra, staging-deploy-app, test-integration]
    if: |
      always() &&
      github.ref == 'refs/heads/develop' &&
      contains(needs.*.result, 'failure')
    # ...

  # ============================================================================
  # STAGE 7: PRODUCTION DEPLOYMENT (main branch only)
  # Runs on: main branch push
  # Purpose: Deploy to production after successful staging validation
  # ============================================================================

  production-deploy:
    name: "Production: Deploy Application"
    if: github.ref == 'refs/heads/main' && github.event_name == 'push'
    needs: [build-docker]
    # ...

  production-smoke-tests:
    name: "Production: Smoke Tests"
    needs: [production-deploy]
    # ...

  production-cleanup-staging:
    name: "Production: Cleanup Staging"
    needs: [production-smoke-tests]
    if: always() && github.ref == 'refs/heads/main'
    # ...
```

**Benefits:**
- Clear stage boundaries (Quality, Tests, Build, Staging, Production)
- Prefix indicates environment (code-quality-, test-, build-, staging-, production-)
- Easy to filter in GitHub Actions UI
- Self-documenting job purpose

---

### 2. Job Dependency Optimization

**Current Structure (Sequential):**
```
build-check → unit-tests → build-docker → ... → integration-tests
```

**Recommended Structure (Parallel where possible):**

```yaml
# Stage 1: Parallel quality checks
code-quality-build ─┐
code-quality-vet   ─┼─→ [Gate]
code-quality-lint  ─┘

# Stage 2: Tests (after quality gate passes)
[Gate] → test-unit ─┐
                     ├─→ [Build Gate]
                     └─→ test-coverage

# Stage 3: Build (after tests pass)
[Build Gate] → build-docker → build-scan

# Stage 4+: Deployment (sequential, environment-specific)
```

**Implementation:**

```yaml
jobs:
  # Parallel quality checks
  quality-build:
    runs-on: ubuntu-latest
    steps: [...]

  quality-vet:
    runs-on: ubuntu-latest
    steps: [...]

  quality-lint:
    runs-on: ubuntu-latest
    steps: [...]

  # Unit tests wait for ALL quality checks
  test-unit:
    needs: [quality-build, quality-vet, quality-lint]
    # ...

  # Coverage can run in parallel with build
  test-coverage:
    needs: [test-unit]
    # ...

  build-docker:
    needs: [test-unit]  # Don't wait for coverage
    # ...
```

**Time Savings:**
- Current: ~15 minutes (sequential)
- Optimized: ~10 minutes (parallel quality checks)

---

### 3. Environment-Specific Stages

**Recommended Environment Strategy:**

| Environment | Branch | Purpose | Lifecycle | Tests Run |
|------------|--------|---------|-----------|-----------|
| **Development** | feature/* | Local dev | Manual | Unit only |
| **Staging** | develop | Integration testing | Ephemeral (deploy → test → destroy) | Unit + Integration |
| **Production** | main | Live service | Persistent | Unit + Smoke |

**Staging Environment Lifecycle:**

```
Develop Push
    ↓
Provision Infrastructure (OCI Free Tier)
    ↓
Deploy Application + Migrations
    ↓
Run Integration Tests
    ↓
┌─────────────┬──────────────┐
│ Tests PASS  │ Tests FAIL   │
├─────────────┼──────────────┤
│ Keep alive  │ Destroy      │
│ for 24h     │ immediately  │
│ (manual     │              │
│ testing)    │              │
└─────────────┴──────────────┘
    ↓                ↓
Main Push        Fix & Retry
    ↓
Destroy Staging
    ↓
Deploy Production
```

**Configuration:**

```yaml
staging-provision-infra:
  uses: ./.github/workflows/infrastructure-lifecycle.yml
  with:
    action: create
    environment: staging
    ttl_hours: 24  # Auto-destroy after 24h if not cleaned up

staging-cleanup-scheduled:
  # Cleanup job that runs daily via cron
  schedule:
    - cron: '0 2 * * *'  # 2 AM UTC daily
  runs-on: ubuntu-latest
  steps:
    - name: Cleanup stale staging environments
      run: |
        # Query OCI for staging instances older than 24h
        # Destroy them to free quota
```

---

### 4. Pull Request Workflow

**Current:** PR workflow is implicit (triggered by `pull_request` event)

**Recommended:** Explicit PR workflow with different test strategy

```yaml
name: Pull Request Validation

on:
  pull_request:
    branches: [main, develop]

permissions:
  contents: read
  pull-requests: write
  checks: write

jobs:
  # Fast feedback loop for PRs
  pr-quality-check:
    name: "PR: Code Quality"
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Run linters
        run: |
          go vet ./...
          # Add golangci-lint, staticcheck, etc.

      - name: Comment on PR
        uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: '✅ Code quality checks passed!'
            })

  pr-unit-tests:
    name: "PR: Unit Tests"
    needs: [pr-quality-check]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Run unit tests
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Coverage report
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
          echo "Total coverage: $COVERAGE"

      - name: Comment coverage on PR
        uses: actions/github-script@v7
        with:
          script: |
            const coverage = process.env.COVERAGE;
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `## Test Coverage\n\n**Total:** ${coverage}`
            })

  pr-build-check:
    name: "PR: Build Check"
    needs: [pr-unit-tests]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true

      - name: Build application
        run: go build -v ./...

  # Don't run integration tests on PRs (too expensive)
  # Wait for merge to develop
```

**PR Workflow Benefits:**
- Fast feedback (3-5 minutes vs 15+ minutes)
- No infrastructure costs
- Clear pass/fail criteria before merge
- Integration tests run after merge to develop

---

### 5. Monitoring & Observability Integration

**Recommended Additions:**

```yaml
  # After successful production deployment
  production-register-deployment:
    name: "Production: Register Deployment"
    needs: [production-deploy]
    runs-on: ubuntu-latest
    steps:
      - name: Send deployment event to monitoring
        run: |
          # Send event to Datadog, New Relic, etc.
          curl -X POST https://monitoring.example.com/events \
            -H "Authorization: Bearer ${{ secrets.MONITORING_API_KEY }}" \
            -d '{
              "event": "deployment",
              "service": "payment-service",
              "version": "${{ github.sha }}",
              "environment": "production",
              "deployed_by": "${{ github.actor }}"
            }'

      - name: Create GitHub deployment
        uses: actions/github-script@v7
        with:
          script: |
            await github.rest.repos.createDeployment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              ref: context.sha,
              environment: 'production',
              auto_merge: false,
              required_contexts: []
            });
```

---

### 6. Security Scanning Integration

**Add to build stage:**

```yaml
  build-security-scan:
    name: "Build: Security Scan"
    needs: [build-docker]
    runs-on: ubuntu-latest
    steps:
      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: 'ghcr.io/${{ github.repository }}:${{ github.sha }}'
          format: 'sarif'
          output: 'trivy-results.sarif'

      - name: Upload Trivy results to GitHub Security
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: 'trivy-results.sarif'

      - name: Fail on HIGH/CRITICAL vulnerabilities
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: 'ghcr.io/${{ github.repository }}:${{ github.sha }}'
          exit-code: 1
          severity: 'CRITICAL,HIGH'
```

---

## Recommended Workflow Organization

### File Structure

```
.github/
└── workflows/
    ├── ci-cd.yml                    # Main pipeline (current)
    ├── pr-validation.yml            # NEW: Fast PR checks
    ├── nightly-tests.yml            # NEW: Comprehensive nightly tests
    ├── manual-infrastructure.yml    # Existing: Manual infra control
    ├── security-scan.yml            # NEW: Scheduled security scans
    └── cleanup-staging.yml          # NEW: Scheduled staging cleanup
```

### Separation of Concerns

**ci-cd.yml** (Main deployment pipeline):
- Triggered: Push to main/develop
- Purpose: Build, test, deploy
- Duration: 15-20 minutes
- Cost: Medium (staging infrastructure)

**pr-validation.yml** (Fast feedback):
- Triggered: Pull request
- Purpose: Code quality + unit tests only
- Duration: 3-5 minutes
- Cost: Low (compute only)

**nightly-tests.yml** (Comprehensive validation):
- Triggered: Cron schedule (nightly)
- Purpose: Full test suite including long-running tests
- Duration: 30-60 minutes
- Cost: Medium

**security-scan.yml** (Vulnerability detection):
- Triggered: Cron schedule (daily/weekly)
- Purpose: Scan dependencies and containers
- Duration: 10-15 minutes
- Cost: Low

**cleanup-staging.yml** (Resource management):
- Triggered: Cron schedule (daily)
- Purpose: Destroy stale staging environments
- Duration: 5 minutes
- Cost: None (saves money)

---

## Pipeline Metrics & KPIs

### Recommended Metrics to Track

```yaml
# Add to all jobs
  - name: Record pipeline metrics
    if: always()
    run: |
      cat << EOF >> $GITHUB_STEP_SUMMARY
      ## Pipeline Metrics

      | Metric | Value |
      |--------|-------|
      | Duration | ${{ job.duration }} |
      | Status | ${{ job.status }} |
      | Runner | ${{ runner.os }} |
      | Trigger | ${{ github.event_name }} |
      | Actor | ${{ github.actor }} |
      EOF
```

**KPIs to Monitor:**
- Pipeline success rate (target: >95%)
- Average pipeline duration (target: <15 minutes)
- Time to deploy (commit to production: target: <30 minutes)
- Infrastructure provisioning time (target: <5 minutes)
- Integration test duration (target: <10 minutes)
- Staging environment uptime cost (target: <10 hours/month)

---

## Advanced Recommendations

### 1. Matrix Testing Strategy

For broader compatibility testing:

```yaml
  test-unit-matrix:
    name: "Tests: Unit Tests (Go ${{ matrix.go-version }})"
    strategy:
      matrix:
        go-version: ['1.22', '1.23']
        os: [ubuntu-latest, macos-latest]
      fail-fast: false
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}
      - run: go test ./...
```

**When to use:**
- Testing multiple Go versions for library compatibility
- Cross-platform testing (Linux, macOS, Windows)
- Not recommended for every PR (too expensive)

---

### 2. Deployment Strategies

**Current:** Replace deployment (single instance)

**Recommended for Production:**

Blue-Green Deployment:
```yaml
  production-deploy-blue-green:
    steps:
      - name: Deploy to green environment
        run: |
          # Deploy new version to "green" instance
          # Run smoke tests
          # If pass: switch load balancer to green
          # If fail: keep blue active, destroy green
```

Canary Deployment:
```yaml
  production-deploy-canary:
    steps:
      - name: Deploy canary (10% traffic)
        run: |
          # Deploy new version
          # Route 10% traffic to new version
          # Monitor metrics for 15 minutes
          # If error rate <1%: promote to 100%
          # If error rate >1%: rollback
```

---

### 3. Rollback Automation

```yaml
  production-auto-rollback:
    name: "Production: Auto-Rollback on Failure"
    needs: [production-smoke-tests]
    if: failure()
    runs-on: ubuntu-latest
    steps:
      - name: Rollback to previous version
        run: |
          PREVIOUS_SHA=$(git rev-parse HEAD~1)
          echo "Rolling back to $PREVIOUS_SHA"

          # Re-deploy previous Docker image
          kubectl set image deployment/payment-service \
            payment-service=ghcr.io/${{ github.repository }}:$PREVIOUS_SHA

      - name: Notify team
        run: |
          # Send Slack/email notification
          # Create incident ticket
          # Tag on-call engineer
```

---

## Pipeline Structure Summary

### Optimal Job Flow

```
┌────────────────────────────────────────────────────┐
│              STAGE 1: CODE QUALITY                  │
│  (Parallel: 2-3 minutes)                           │
├────────────────────────────────────────────────────┤
│  • Build Verification                              │
│  • Go Vet                                          │
│  • Linting (optional)                              │
└────────────────────────────────────────────────────┘
                       ↓
┌────────────────────────────────────────────────────┐
│              STAGE 2: UNIT TESTING                  │
│  (Sequential: 3-5 minutes)                         │
├────────────────────────────────────────────────────┤
│  • Unit Tests                                      │
│  • Coverage Report                                 │
└────────────────────────────────────────────────────┘
                       ↓
┌────────────────────────────────────────────────────┐
│              STAGE 3: BUILD & SCAN                  │
│  (Sequential: 4-6 minutes)                         │
├────────────────────────────────────────────────────┤
│  • Build Docker Image                              │
│  • Security Scan                                   │
│  • Push to Registry                                │
└────────────────────────────────────────────────────┘
                       ↓
        ┌──────────────┴──────────────┐
        ↓                             ↓
┌──────────────────┐          ┌──────────────────┐
│ DEVELOP BRANCH   │          │  MAIN BRANCH     │
│ (Staging)        │          │  (Production)    │
└──────────────────┘          └──────────────────┘
        ↓                             ↓
┌──────────────────┐          ┌──────────────────┐
│ Provision Infra  │          │ Deploy to Prod   │
│      (3 min)     │          │      (5 min)     │
└──────────────────┘          └──────────────────┘
        ↓                             ↓
┌──────────────────┐          ┌──────────────────┐
│ Deploy Staging   │          │  Smoke Tests     │
│      (2 min)     │          │      (2 min)     │
└──────────────────┘          └──────────────────┘
        ↓                             ↓
┌──────────────────┐          ┌──────────────────┐
│ Integration Tests│          │ Cleanup Staging  │
│     (10 min)     │          │      (2 min)     │
└──────────────────┘          └──────────────────┘
        ↓
┌──────────────────┐
│ Cleanup (Fail)   │
│      (2 min)     │
└──────────────────┘

Total Time:
- PR: 8-12 minutes (no deployment)
- Develop: 20-25 minutes (staging + tests)
- Main: 15-20 minutes (production)
```

---

## Migration Plan

### Phase 1: Critical Fixes (Week 1)
- Fix Go version
- Add infrastructure output validation
- Improve health checks
- Add error handling

### Phase 2: Structure Improvements (Week 2)
- Rename jobs for consistency
- Add PR validation workflow
- Implement caching
- Add deployment summaries

### Phase 3: Advanced Features (Week 3-4)
- Security scanning integration
- Monitoring/observability hooks
- Nightly comprehensive test suite
- Automated staging cleanup

### Phase 4: Production Hardening (Week 5-6)
- Blue-green deployment
- Auto-rollback on failure
- Canary deployment (future)
- Incident response automation
