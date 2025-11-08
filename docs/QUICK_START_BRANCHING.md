# Quick Start: Branching & Deployment

**TL;DR:**
- Work on `develop` → auto-deploys to **staging**
- Merge to `main` → deploys to **production** (after approval)

---

## Branch Overview

```
┌─────────────────────────────────────────────────────┐
│  develop branch                                     │
│  ├─ Push changes here                               │
│  ├─ Auto-deploys to Oracle Cloud Staging                 │
│  └─ EPX Sandbox credentials                         │
└─────────────────────────────────────────────────────┘
                      │
                      │ (Create PR when ready)
                      ▼
┌─────────────────────────────────────────────────────┐
│  main branch                                        │
│  ├─ Merge from develop                              │
│  ├─ Deploys to Google Cloud Run                     │
│  ├─ Requires approval                                │
│  └─ EPX Production credentials                      │
└─────────────────────────────────────────────────────┘
```

---

## Daily Workflow

### For New Features

```bash
# 1. Start from develop
git checkout develop
git pull origin develop

# 2. Create feature branch
git checkout -b feature/add-new-payment-method

# 3. Make changes, commit
git add .
git commit -m "feat: Add new payment method"
git push origin feature/add-new-payment-method

# 4. Create Pull Request on GitHub
#    Base: develop
#    Compare: feature/add-new-payment-method

# 5. After PR is merged → automatically deploys to staging
#    Test at: http://{ORACLE_CLOUD_HOST}:8081/cron/health
```

### Push to Staging (Quick)

If you're working directly on develop (for quick fixes):

```bash
git checkout develop
git add .
git commit -m "fix: Quick bug fix"
git push origin develop
# ✅ Automatically deploys to staging
```

### Deploy to Production

When staging is tested and ready:

```bash
# 1. Create PR from develop to main on GitHub
#    Base: main
#    Compare: develop

# 2. Get team approval

# 3. Merge PR

# 4. Approve deployment in GitHub Actions
#    (You'll get a notification)

# ✅ Deploys to production
```

---

## What Happens Automatically

### On Push to `develop`:
1. ✅ Run all tests
2. ✅ Build Docker image
3. ✅ Run migrations on staging database
4. ✅ **Deploy to Oracle Cloud Staging** (automatic, no approval)

### On Push to `main`:
1. ✅ Run all tests
2. ✅ Build Docker image
3. ✅ Run migrations on production database (Cloud SQL)
4. ⏸️ **Wait for approval** (you'll get notified)
5. ✅ **Build and push to Artifact Registry** (after approval)
6. ✅ **Deploy to Google Cloud Run** (after approval)

---

## Environment URLs

| Environment | Branch | Platform | URL | Auto-Deploy |
|-------------|--------|----------|-----|-------------|
| **Staging** | `develop` | Oracle Cloud | http://YOUR_ORACLE_IP:8081 | ✅ Yes |
| **Production** | `main` | Google Cloud Run | https://payment-service-xxx.a.run.app | ⏸️ After approval |

---

## Quick Commands

### Daily Development
```bash
# Pull latest from develop
git checkout develop && git pull

# Create feature branch
git checkout -b feature/my-feature

# Commit and push
git add . && git commit -m "feat: My feature"
git push origin feature/my-feature

# Create PR to develop on GitHub
```

### Check Deployment Status
```bash
# Staging health check (replace with actual Oracle Cloud IP)
curl http://${ORACLE_CLOUD_HOST}:8081/cron/health

# View GitHub Actions
open https://github.com/kevin07696/payment-service/actions
```

### Rollback Staging
```bash
git checkout develop
git revert HEAD  # Reverts last commit
git push origin develop
# Staging automatically redeploys with reverted code
```

---

## GitHub Environments Needed

You need to create two environments in GitHub:

### 1. Staging Environment
**Setup:** GitHub → Settings → Environments → New environment → "staging"

**Protection Rules:**
- ☐ Required reviewers: UNCHECKED (auto-deploy)
- Deployment branches: `develop` only

**Secrets:** (see [ORACLE_FREE_TIER_STAGING.md](./ORACLE_FREE_TIER_STAGING.md) for complete list)
- `OCI_USER_OCID`, `OCI_TENANCY_OCID`, `OCI_COMPARTMENT_OCID`, `OCI_REGION`, `OCI_FINGERPRINT`, `OCI_PRIVATE_KEY`
- `ORACLE_DB_ADMIN_PASSWORD`, `ORACLE_DB_PASSWORD`
- `ORACLE_CLOUD_HOST`, `ORACLE_CLOUD_SSH_KEY` (from Terraform outputs)
- `OCIR_*` secrets for Oracle Container Registry
- `EPX_MAC_STAGING` = `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y`
- `CRON_SECRET_STAGING` (generate with `openssl rand -base64 32`)

### 2. Production Environment
**Setup:** GitHub → Settings → Environments → New environment → "production"

**Protection Rules:**
- ✅ Required reviewers: CHECK (1-2 people)
- Deployment branches: `main` only

**Secrets (15 total):**
- `GCP_SA_KEY` (Service account JSON key)
- `GCP_PROJECT_ID` (Google Cloud project ID)
- `GCP_REGION` (e.g., us-central1)
- `GCP_DB_INSTANCE_CONNECTION_NAME` (Cloud SQL connection string)
- `GCP_DB_USER` (Database username)
- `GCP_DB_PASSWORD` (Database password)
- `GCP_DB_NAME` (Database name)
- `GCP_DATABASE_URL` (Full PostgreSQL connection URL)
- `EPX_CUST_NBR_PRODUCTION` (get from EPX)
- `EPX_MERCH_NBR_PRODUCTION` (get from EPX)
- `EPX_DBA_NBR_PRODUCTION` (get from EPX)
- `EPX_TERMINAL_NBR_PRODUCTION` (get from EPX)
- `EPX_MAC_PRODUCTION` (get from EPX)
- `CRON_SECRET_PRODUCTION` (generate with `openssl rand -base64 32`)
- `CALLBACK_BASE_URL_PRODUCTION` (your Cloud Run URL or custom domain)

---

## Next Steps

### Right Now (For Staging):
1. ✅ Create "staging" environment in GitHub
2. ✅ Add 15 secrets to staging environment (see [ORACLE_FREE_TIER_STAGING.md](./ORACLE_FREE_TIER_STAGING.md))
3. ✅ Run Terraform to provision Oracle Cloud infrastructure
4. ✅ Push to develop → test deployment

### Later (For Production):
1. Set up Google Cloud Platform project
2. Create Cloud SQL PostgreSQL instance
3. Create Artifact Registry repository
4. Create service account and JSON key
5. Create "production" environment in GitHub
6. Add 15 secrets to production environment
7. Add production protection rules (require approval)
8. Get production EPX credentials from EPX support
9. Deploy to Google Cloud Run

---

## Full Documentation

- **Complete Git Workflow:** [BRANCHING_STRATEGY.md](./BRANCHING_STRATEGY.md)
- **Oracle Cloud Staging Setup:** [ORACLE_FREE_TIER_STAGING.md](./ORACLE_FREE_TIER_STAGING.md)
- **Google Cloud Run Production Setup:** [GCP_PRODUCTION_SETUP.md](./GCP_PRODUCTION_SETUP.md)
- **GitHub Environments:** [GITHUB_ENVIRONMENT_SETUP.md](./GITHUB_ENVIRONMENT_SETUP.md)
- **Terraform Infrastructure:** [../terraform/README.md](../terraform/README.md)

---

## Common Questions

**Q: Which branch do I work on?**
A: `develop` - this deploys to staging for testing

**Q: How do I deploy to staging?**
A: Just push to develop branch, deployment is automatic

**Q: How do I deploy to production?**
A: Create PR from develop → main, get approval, merge

**Q: Can I push directly to main?**
A: No, main is protected. Always merge via PR from develop

**Q: What if staging breaks?**
A: Revert the commit and push, staging will auto-redeploy

**Q: How long does deployment take?**
A: ~3-4 minutes from push to live

**Q: Where do I see deployment status?**
A: GitHub Actions tab in your repository

---

## Help

Stuck? Check the detailed guides in `docs/` folder or ask the team!
