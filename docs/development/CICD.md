# CI/CD Pipeline and Deployment

**Target Audience:** Developers configuring CI/CD or deploying the payment service
**Topic:** Continuous Integration, Continuous Deployment, and infrastructure management
**Goal:** Deploy the payment service to staging and production with confidence

---

## Overview

The payment service uses a **two-environment deployment model**:

| Environment | Trigger | Platform | Purpose |
|------------|---------|----------|---------|
| **Staging** | Push to `develop` | Oracle Cloud (Free Tier) | Automated integration testing |
| **Production** | Push to `main` | Google Cloud Run | Live customer traffic |

**Key Features:**
- Automated testing at every stage
- Infrastructure-as-code provisioning
- Auto-cleanup on failure (saves free tier quota)
- Zero-downtime deployments
- Comprehensive secrets management

---

## Quick Start

### 1. Configure Secrets (One-Time Setup)

```bash
# Auto-configure all 18 required secrets
./scripts/configure-github-secrets.sh

# Verify configuration
gh secret list --repo kevin07696/payment-service
```

### 2. Deploy to Staging

```bash
git checkout develop
git commit -m "feat: Add new feature"
git push origin develop

# Pipeline automatically:
# 1. Runs unit tests
# 2. Builds Docker image
# 3. Provisions Oracle Cloud infrastructure
# 4. Runs database migrations
# 5. Deploys application
# 6. Runs integration tests
```

### 3. Promote to Production

```bash
git checkout main
git merge develop
git push origin main

# Pipeline automatically:
# 1. Runs unit tests
# 2. Builds Docker image
# 3. Deploys to Google Cloud Run
# 4. Runs smoke tests
# 5. Cleans up staging
```

---

## Pipeline Architecture

### Staging Pipeline (develop branch)

```
┌─────────────────────────────────────────────────────────────┐
│                    STAGING DEPLOYMENT                        │
│                  (develop branch → Oracle Cloud)             │
└─────────────────────────────────────────────────────────────┘

Step 1: Unit Tests
├─ Run all Go unit tests
└─ Verify code quality

Step 2: Build Docker Image
├─ Build service Docker image
└─ No push (local build only)

Step 3: Provision Infrastructure
├─ Terraform provisions Oracle Cloud resources:
│  ├─ Autonomous Database (Always Free)
│  ├─ Compute instance (ARM, Always Free)
│  └─ OCI Vault (for secrets)
└─ Outputs: DB connection details, VM host

Step 4: Deploy Application + Migrate Database
├─ Copy Docker image to VM
├─ Run Goose migrations (atomically)
├─ Start payment service
└─ Configure environment variables

Step 5: Integration Tests
├─ Wait for service health check
├─ Run full integration test suite
└─ Test against real EPX sandbox

┌─────────────────────────────────────────────────────────────┐
│  ✅ SUCCESS: Staging kept running for debugging              │
│  ❌ FAILURE: Auto-cleanup to free Oracle quota              │
└─────────────────────────────────────────────────────────────┘
```

### Production Pipeline (main branch)

```
┌─────────────────────────────────────────────────────────────┐
│                  PRODUCTION DEPLOYMENT                       │
│                 (main branch → Google Cloud Run)             │
└─────────────────────────────────────────────────────────────┘

Step 1: Unit Tests
└─ Verify code quality

Step 2: Build Docker Image
└─ Build production image

Step 3: Deploy to Cloud Run
├─ Push image to Google Container Registry
├─ Deploy to Cloud Run
└─ Configure production secrets

Step 4: Production Smoke Tests
├─ Health check endpoint
├─ Basic API validation
└─ Verify service responding

Step 5: Cleanup Staging
└─ Destroy Oracle Cloud staging infrastructure
```

---

## Secrets Management

### Architecture

**Two-Repository Model:**

| Repository | Responsibility | Secrets Stored |
|-----------|----------------|----------------|
| **payment-service** | Service configuration | `ORACLE_DB_PASSWORD`, `EPX_MAC_STAGING` |
| **deployment-workflows** | Infrastructure provisioning | OCI credentials, OCIR credentials, SSH keys |

**Secret Flow:**
```
GitHub Secrets
  → Terraform (provisions infrastructure)
    → OCI Vault (stores runtime secrets)
      → Service reads at runtime
```

### Required Secrets (18 Total)

Configure in: **GitHub Repository → Settings → Secrets and variables → Actions**

#### Oracle Cloud Infrastructure (6 secrets)

| Secret | Description | Where to Get |
|--------|-------------|--------------|
| `OCI_USER_OCID` | User identifier | `~/.oci/config` → `user=` |
| `OCI_TENANCY_OCID` | Tenancy identifier | `~/.oci/config` → `tenancy=` |
| `OCI_COMPARTMENT_OCID` | Compartment identifier | `oci iam compartment list --query 'data[0].id'` |
| `OCI_REGION` | Oracle Cloud region | `~/.oci/config` → `region=` (e.g., `us-ashburn-1`) |
| `OCI_FINGERPRINT` | API key fingerprint | `~/.oci/config` → `fingerprint=` |
| `OCI_PRIVATE_KEY` | API private key | `cat ~/.oci/oci_api_key.pem` |

**Usage:** Terraform authenticates to Oracle Cloud to provision database and compute instances.

#### Container Registry (4 secrets)

| Secret | Description | Where to Get |
|--------|-------------|--------------|
| `OCIR_REGION` | Registry region code | Region code (e.g., `iad` for us-ashburn-1) |
| `OCIR_TENANCY_NAMESPACE` | Registry namespace | `oci os ns get --query 'data' --raw-output` |
| `OCIR_USERNAME` | Registry username | `<tenancy>/<username>` format |
| `OCIR_AUTH_TOKEN` | Registry password | Oracle Cloud Console → User Settings → Auth Tokens |

**Usage:** Docker push/pull access to Oracle Container Image Registry.

#### Database (1 secret)

| Secret | Description | Generation |
|--------|-------------|------------|
| `ORACLE_DB_PASSWORD` | PostgreSQL admin password | `openssl rand -base64 32` |

**Usage:** Database connection credentials passed to application.

#### EPX Payment Gateway (5 secrets)

| Secret | Value | Environment |
|--------|-------|-------------|
| `EPX_MAC_STAGING` | `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` | Test/Staging |
| `EPX_CUST_NBR` | `9001` | Test/Staging |
| `EPX_MERCH_NBR` | `900300` | Test/Staging |
| `EPX_DBA_NBR` | `2` | Test/Staging |
| `EPX_TERMINAL_NBR` | `77` | Test/Staging |

**Usage:** EPX sandbox credentials for integration testing. These are public test credentials.

#### Application Secrets (2 secrets)

| Secret | Description | Generation |
|--------|-------------|------------|
| `CRON_SECRET_STAGING` | Cron endpoint authentication | `openssl rand -base64 32` |
| `SSH_PUBLIC_KEY` | VM access key | `cat ~/.ssh/id_rsa.pub` |

**Usage:** Service configuration and infrastructure access.

### Automated Setup

```bash
# Prerequisites
brew install gh  # macOS
gh auth login

# Run automated configuration script
./scripts/configure-github-secrets.sh

# Script performs:
# 1. Reads ~/.oci/config for OCI credentials
# 2. Generates secure passwords
# 3. Sets all 18 secrets via GitHub CLI
# 4. Verifies configuration

# Verify
gh secret list --repo kevin07696/payment-service
```

### Manual Setup

```bash
REPO="kevin07696/payment-service"

# Oracle Cloud (from ~/.oci/config)
gh secret set OCI_USER_OCID --repo $REPO \
  --body "$(grep 'user=' ~/.oci/config | cut -d'=' -f2)"

gh secret set OCI_TENANCY_OCID --repo $REPO \
  --body "$(grep 'tenancy=' ~/.oci/config | cut -d'=' -f2)"

gh secret set OCI_FINGERPRINT --repo $REPO \
  --body "$(grep 'fingerprint=' ~/.oci/config | cut -d'=' -f2)"

gh secret set OCI_REGION --repo $REPO \
  --body "$(grep 'region=' ~/.oci/config | cut -d'=' -f2)"

gh secret set OCI_PRIVATE_KEY --repo $REPO \
  --body "$(cat ~/.oci/oci_api_key.pem)"

gh secret set OCI_COMPARTMENT_OCID --repo $REPO \
  --body "$(oci iam compartment list --query 'data[0].id' --raw-output)"

# Container Registry
gh secret set OCIR_REGION --repo $REPO --body "iad"

gh secret set OCIR_TENANCY_NAMESPACE --repo $REPO \
  --body "$(oci os ns get --query 'data' --raw-output)"

gh secret set OCIR_USERNAME --repo $REPO \
  --body "$(oci os ns get --query 'data' --raw-output)/$(oci iam user get --user-id $(grep 'user=' ~/.oci/config | cut -d'=' -f2) --query 'data.name' --raw-output)"

# Generate and set OCIR Auth Token manually:
# Oracle Cloud Console → User Settings → Auth Tokens → Generate Token
gh secret set OCIR_AUTH_TOKEN --repo $REPO  # paste token when prompted

# Database
gh secret set ORACLE_DB_PASSWORD --repo $REPO \
  --body "$(openssl rand -base64 32)"

# EPX Sandbox (public test credentials)
gh secret set EPX_MAC_STAGING --repo $REPO \
  --body "2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"

gh secret set EPX_CUST_NBR --repo $REPO --body "9001"
gh secret set EPX_MERCH_NBR --repo $REPO --body "900300"
gh secret set EPX_DBA_NBR --repo $REPO --body "2"
gh secret set EPX_TERMINAL_NBR --repo $REPO --body "77"

# Application
gh secret set CRON_SECRET_STAGING --repo $REPO \
  --body "$(openssl rand -base64 32)"

gh secret set SSH_PUBLIC_KEY --repo $REPO \
  --body "$(cat ~/.ssh/id_rsa.pub)"
```

### Runtime Secret Access

At runtime, the service retrieves secrets from **OCI Vault**:

```go
// Service reads merchant credentials
1. Query agent_credentials table for merchant
2. Get mac_secret_path (Vault OCID)
3. Use OCI SDK to read secret from Vault
4. Use MAC secret to sign EPX requests
```

**IAM Permissions Required:** Service compute instance needs `read` permission on Vault secrets via OCI dynamic group policy.

---

## Manual Infrastructure Management

### Create Staging Environment

```bash
# Via GitHub Actions UI
# Actions → Manual Infrastructure Management → Run workflow
# - Action: create
# - Environment: staging

# Via GitHub CLI
gh workflow run manual-infrastructure.yml \
  -f action=create \
  -f environment=staging
```

### Destroy Staging Environment

```bash
# Via GitHub CLI
gh workflow run manual-infrastructure.yml \
  -f action=destroy \
  -f environment=staging

# Check status
gh run list --workflow=manual-infrastructure.yml
```

### Check Infrastructure Status

```bash
gh workflow run manual-infrastructure.yml \
  -f action=check \
  -f environment=staging

# View results
gh run view --log
```

---

## Deployment Workflows

### Staging Deployment (Automatic on develop push)

File: `.github/workflows/ci-cd.yml`

```yaml
on:
  push:
    branches: [develop]

jobs:
  unit-tests:
    uses: deployment-workflows/.github/workflows/go-test.yml@main
    with:
      go-version: '1.24'

  build-docker-image:
    needs: unit-tests
    uses: deployment-workflows/.github/workflows/go-build-docker.yml@main
    with:
      service-name: payment-service

  ensure-staging-infrastructure:
    needs: build-docker-image
    uses: deployment-workflows/.github/workflows/infrastructure-lifecycle.yml@main
    secrets: inherit
    with:
      action: create
      environment: staging

  deploy-staging:
    needs: ensure-staging-infrastructure
    uses: deployment-workflows/.github/workflows/deploy-oracle-staging.yml@main
    secrets: inherit
    with:
      service-name: payment-service
      oracle-cloud-host: ${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}
      db-host: ${{ needs.ensure-staging-infrastructure.outputs.db_host }}

  integration-tests:
    needs: deploy-staging
    runs-on: ubuntu-latest
    steps:
      - name: Wait for service
        run: |
          for i in {1..30}; do
            if curl -f "http://${{ needs.ensure-staging-infrastructure.outputs.oracle_cloud_host }}:8081/cron/health"; then
              exit 0
            fi
            sleep 10
          done
          exit 1

      - name: Run tests
        run: go test ./tests/integration/... -v -tags=integration -timeout=15m

  cleanup-staging-on-failure:
    needs: [ensure-staging-infrastructure, deploy-staging, integration-tests]
    if: always() && (needs.*.result == 'failure')
    uses: deployment-workflows/.github/workflows/infrastructure-lifecycle.yml@main
    secrets: inherit
    with:
      action: destroy
      environment: staging
```

### Production Deployment (Automatic on main push)

```yaml
on:
  push:
    branches: [main]

jobs:
  deploy-production:
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to Cloud Run
        run: |
          # TODO: Implement Cloud Run deployment
          echo "Production deployment placeholder"

  production-smoke-tests:
    needs: deploy-production
    runs-on: ubuntu-latest
    steps:
      - name: Health check
        run: |
          # TODO: Replace with production URL
          # curl -f https://payment-service.example.com/health

  cleanup-staging-after-production:
    needs: production-smoke-tests
    if: always()
    uses: deployment-workflows/.github/workflows/infrastructure-lifecycle.yml@main
    secrets: inherit
    with:
      action: destroy
      environment: staging
```

---

## Monitoring Deployments

### Watch Pipeline Execution

```bash
# List recent runs
gh run list

# Watch active run
gh run watch

# View specific run
gh run view 12345678 --log

# View run in browser
gh run view 12345678 --web
```

### Check Staging Service Health

```bash
# Get staging host from workflow outputs
STAGING_HOST=$(gh api repos/kevin07696/payment-service/actions/runs/XXXXX \
  --jq '.jobs[0].steps[] | select(.name=="ensure-staging-infrastructure") | .outputs.oracle_cloud_host')

# Health check
curl http://$STAGING_HOST:8081/cron/health

# Test payment form
curl http://$STAGING_HOST:8081/api/v1/payment/form
```

### View Integration Test Results

```bash
# Get test logs
gh run view --log | grep "integration-tests"

# Download test artifacts
gh run download 12345678
```

---

## Troubleshooting

### Pipeline Failures

#### "Secret not found"

```bash
# Verify secret exists
gh secret list --repo kevin07696/payment-service | grep SECRET_NAME

# Re-set the secret (case-sensitive)
gh secret set SECRET_NAME --repo kevin07696/payment-service
```

#### "OCI authentication failed"

```bash
# Test OCI CLI locally
oci iam region list

# Verify config file
cat ~/.oci/config

# Check private key permissions
chmod 600 ~/.oci/oci_api_key.pem

# Verify fingerprint matches
openssl rsa -pubout -outform DER -in ~/.oci/oci_api_key.pem | \
  openssl md5 -c | \
  awk '{print $2}'
```

#### "Infrastructure provisioning timeout"

```bash
# Check Oracle Cloud quota
oci limits resource-availability get \
  --compartment-id $OCI_COMPARTMENT_OCID \
  --service-name compute \
  --limit-name vm-standard-a1-core-count

# Manually destroy stuck resources
gh workflow run manual-infrastructure.yml \
  -f action=destroy \
  -f environment=staging
```

#### "Integration tests failing"

```bash
# SSH into staging VM
ssh ubuntu@$STAGING_HOST

# Check service logs
sudo journalctl -u payment-service -f

# Check database connectivity
docker exec payment-service-db psql -U payment_service -c "\dt"

# Verify EPX connectivity
curl -X POST https://certapia.transnox.com/transit-tsys-securelink/xmlMsg
```

### Deployment Issues

#### "Service not responding after deployment"

```bash
# SSH to VM
ssh ubuntu@$STAGING_HOST

# Check Docker container
docker ps
docker logs payment-service

# Check port binding
netstat -tlnp | grep 8081

# Restart service
docker restart payment-service
```

#### "Database migration failed"

```bash
# SSH to VM
ssh ubuntu@$STAGING_HOST

# Check migration status
docker exec payment-service-db psql -U payment_service -d payment_service \
  -c "SELECT * FROM goose_db_version ORDER BY id DESC LIMIT 5;"

# Manually run migrations
cd /app
goose -dir internal/db/migrations postgres \
  "host=$DB_HOST port=$DB_PORT user=payment_service password=$DB_PASSWORD dbname=payment_service sslmode=require" \
  up
```

#### "OCIR authentication failed"

```bash
# Test OCIR login locally
docker login iad.ocir.io \
  -u '<tenancy-namespace>/<username>' \
  -p '<auth-token>'

# Regenerate auth token
# Oracle Cloud Console → User Settings → Auth Tokens → Generate Token

# Update GitHub secret
gh secret set OCIR_AUTH_TOKEN --repo kevin07696/payment-service
```

### Resource Cleanup

#### "Oracle free tier quota exceeded"

```bash
# List all running compute instances
oci compute instance list --all \
  --compartment-id $OCI_COMPARTMENT_OCID \
  --query 'data[*].{Name:"display-name", State:"lifecycle-state", OCID:id}'

# Terminate specific instance
oci compute instance terminate --instance-id ocid1.instance.oc1...

# List databases
oci db autonomous-database list --all \
  --compartment-id $OCI_COMPARTMENT_OCID

# Delete database
oci db autonomous-database delete --autonomous-database-id ocid1.autonomousdatabase.oc1...

# Verify cleanup
gh workflow run manual-infrastructure.yml -f action=check -f environment=staging
```

---

## Best Practices

### 1. Branch Protection

**main branch:**
- Require pull request reviews (1+ approvers)
- Require status checks to pass (unit-tests, build-docker-image)
- Require branches to be up to date
- No force push

**develop branch:**
- Require status checks to pass
- Allow force push (for rebasing)

```bash
# Configure via GitHub Settings → Branches → Add rule
# Or via API:
gh api repos/kevin07696/payment-service/branches/main/protection \
  --method PUT \
  --field required_status_checks[strict]=true \
  --field required_pull_request_reviews[required_approving_review_count]=1
```

### 2. Secret Rotation

```bash
# Rotate database password quarterly
NEW_PASSWORD=$(openssl rand -base64 32)
gh secret set ORACLE_DB_PASSWORD --repo kevin07696/payment-service --body "$NEW_PASSWORD"

# Trigger redeployment to update service
git commit --allow-empty -m "chore: Rotate database password"
git push origin develop
```

### 3. Deployment Verification

Always verify deployments:

```bash
# Staging verification checklist
curl http://$STAGING_HOST:8081/cron/health  # Health check
curl http://$STAGING_HOST:8081/api/v1/payment/form  # Payment form
go test ./tests/integration/... -v  # Full test suite

# Production verification checklist
curl https://payment-service.example.com/health
# Monitor error rates in production logs
# Check payment success rate metrics
```

### 4. Rollback Strategy

**Staging:** Destroy and redeploy from previous commit
```bash
git revert HEAD
git push origin develop
```

**Production:**
```bash
# Revert to previous version
git revert HEAD
git push origin main

# Or rollback Cloud Run revision
gcloud run services update-traffic payment-service \
  --to-revisions=payment-service-00042-xyz=100
```

### 5. Cost Optimization

**Staging:**
- Automatic cleanup on failure (saves Oracle quota)
- Manual cleanup after production promotion
- No persistent staging environment

**Production:**
- Cloud Run scales to zero (pay per request)
- Configure max instances to control costs
- Use Cloud CDN for static assets

---

## Related Documentation

- **DEVELOP.md** - Branching strategy and testing guidelines
- **DATAFLOW.md** - Understanding service architecture
- **DATABASE.md** - Database migrations and schema
- **AUTH.md** - Token management and authentication