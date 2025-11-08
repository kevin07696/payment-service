# Git Branching Strategy

This document describes the branching strategy and CI/CD workflow for the payment service.

## Branch Structure

We use a **Git Flow** inspired branching strategy with two main branches:

```
main (production) ─────────────────────────────────►
  │
  └─► develop (staging) ──────────────────────────►
       │
       └─► feature/* ──────────►
```

### Branch Purposes

| Branch | Environment | Purpose | Auto-Deploy |
|--------|-------------|---------|-------------|
| `main` | **Production** | Production-ready code | ✅ Yes (with approval) |
| `develop` | **Staging** | Integration and testing | ✅ Yes (automatic) |
| `feature/*` | Local | New features | ❌ No |
| `bugfix/*` | Local | Bug fixes | ❌ No |
| `hotfix/*` | Local | Emergency production fixes | ❌ No |

---

## Workflow

### 1. Feature Development

```bash
# Start from develop
git checkout develop
git pull origin develop

# Create feature branch
git checkout -b feature/payment-refund

# Work on your feature...
git add .
git commit -m "feat: Add refund functionality"

# Push to remote
git push origin feature/payment-refund
```

**Create Pull Request:**
- Base: `develop`
- Compare: `feature/payment-refund`
- Get code review
- Merge to `develop`

**Result:** Automatically deploys to **staging** after merge

### 2. Staging Testing

After merge to `develop`:
1. ✅ Tests run automatically
2. ✅ Docker image builds
3. ✅ Migrations run on staging database
4. ✅ **Deploys to Railway staging**
5. Test in staging environment
6. Verify with India team

**Staging URL:** `https://payment-service-staging.up.railway.app`

### 3. Production Release

When staging is stable and tested:

```bash
# Checkout develop
git checkout develop
git pull origin develop

# Merge to main
git checkout main
git pull origin main
git merge develop

# Push to trigger production deployment
git push origin main
```

**Create Pull Request (Recommended):**
- Base: `main`
- Compare: `develop`
- Get approval from team lead
- Merge to `main`

**Result:**
1. ✅ Tests run
2. ✅ Docker image builds
3. ✅ Migrations run on production database
4. ⏸️ **Waits for manual approval** (production environment protection)
5. ✅ Deploys to Railway production

**Production URL:** `https://payments.yourdomain.com`

### 4. Hotfixes (Emergency Production Fixes)

For urgent production bugs:

```bash
# Create hotfix from main
git checkout main
git pull origin main
git checkout -b hotfix/critical-payment-bug

# Fix the bug
git add .
git commit -m "fix: Resolve critical payment processing issue"

# Push and create PR to main
git push origin hotfix/critical-payment-bug
```

**After merge to main:**
1. Production deploys after approval
2. **Important:** Merge hotfix back to develop:
   ```bash
   git checkout develop
   git merge hotfix/critical-payment-bug
   git push origin develop
   ```

---

## CI/CD Pipeline Details

### Triggers

| Event | Branch | Jobs Run |
|-------|--------|----------|
| Push to `develop` | `develop` | Test → Build → Migrate Staging → Deploy Staging |
| Push to `main` | `main` | Test → Build → Migrate Production → Deploy Production |
| Pull Request | Any | Test → Build (no deployment) |

### Deployment Flow

#### Staging (develop branch)
```
Push to develop
    ↓
Run Tests (Go 1.24)
    ↓
Build Docker Image
    ↓
Run Database Migrations (Staging)
    ↓
Deploy to Railway Staging ✅ (automatic)
```

**Staging Credentials:**
- EPX: Sandbox credentials
- Database: Railway staging PostgreSQL
- Environment: `ENVIRONMENT=staging`

#### Production (main branch)
```
Push to main
    ↓
Run Tests (Go 1.24)
    ↓
Build Docker Image
    ↓
Run Database Migrations (Production)
    ↓
⏸️ Wait for Manual Approval
    ↓
Deploy to Railway Production ✅ (after approval)
```

**Production Credentials:**
- EPX: Production credentials
- Database: Railway production PostgreSQL
- Environment: `ENVIRONMENT=production`

---

## Environment Configuration

### Staging (develop → Railway Staging)

**EPX Configuration:**
- Base URL: `https://secure.epxuap.com` (sandbox)
- Credentials: Sandbox (CUST_NBR=9001, etc.)
- MAC: Sandbox MAC token

**Database:**
- Railway PostgreSQL (staging project)
- SSL Mode: `require`

**Logging:**
- Level: `debug` (verbose for troubleshooting)

### Production (main → Railway Production)

**EPX Configuration:**
- Base URL: `https://secure.epxnow.com` (production)
- Credentials: Production credentials from EPX
- MAC: Production MAC token

**Database:**
- Railway PostgreSQL (production project)
- SSL Mode: `require`

**Logging:**
- Level: `info` (less verbose)

---

## GitHub Environments

### Staging Environment
- **Name:** `staging`
- **Protection:** No approval required (automatic deployment)
- **Branches:** `develop` only
- **Secrets:**
  - `RAILWAY_TOKEN`
  - `RAILWAY_PROJECT_ID`
  - `EPX_MAC_STAGING`
  - `CRON_SECRET_STAGING`
  - `CALLBACK_BASE_URL_STAGING`

### Production Environment
- **Name:** `production`
- **Protection:** ✅ Required reviewers (1-2 people)
- **Branches:** `main` only
- **Secrets:**
  - `RAILWAY_TOKEN_PRODUCTION`
  - `RAILWAY_PROJECT_ID_PRODUCTION`
  - `EPX_CUST_NBR_PRODUCTION`
  - `EPX_MERCH_NBR_PRODUCTION`
  - `EPX_DBA_NBR_PRODUCTION`
  - `EPX_TERMINAL_NBR_PRODUCTION`
  - `EPX_MAC_PRODUCTION`
  - `CRON_SECRET_PRODUCTION`
  - `CALLBACK_BASE_URL_PRODUCTION`

---

## Branch Protection Rules

### `main` Branch
- ✅ Require pull request before merging
- ✅ Require approvals: 1
- ✅ Require status checks to pass
  - CI tests must pass
  - Docker build must succeed
- ✅ Require branches to be up to date
- ✅ Do not allow force pushes
- ✅ Do not allow deletions

### `develop` Branch
- ✅ Require pull request before merging (recommended)
- ✅ Require status checks to pass
  - CI tests must pass
- ⚠️ Allow force pushes: No
- ⚠️ Allow deletions: No

### Feature Branches
- No protection rules
- Delete after merge to `develop`

---

## Common Workflows

### 1. New Feature Development
```bash
# 1. Create feature branch from develop
git checkout develop && git pull
git checkout -b feature/new-payment-method

# 2. Develop and test locally
# ... make changes ...

# 3. Commit and push
git add .
git commit -m "feat: Add new payment method"
git push origin feature/new-payment-method

# 4. Create PR: feature/new-payment-method → develop
# 5. After merge, automatically deploys to staging
# 6. Test in staging
# 7. When ready, create PR: develop → main
# 8. After approval and merge, deploys to production
```

### 2. Bug Fix in Staging
```bash
# 1. Create bugfix branch from develop
git checkout develop && git pull
git checkout -b bugfix/fix-payment-callback

# 2. Fix the bug
# ... make changes ...

# 3. Commit and push
git add .
git commit -m "fix: Resolve payment callback issue"
git push origin bugfix/fix-payment-callback

# 4. Create PR → develop
# 5. Merge and test in staging
# 6. If good, merge develop → main for production
```

### 3. Production Hotfix
```bash
# 1. Create hotfix from main
git checkout main && git pull
git checkout -b hotfix/critical-security-fix

# 2. Fix the issue
# ... make changes ...

# 3. Commit and push
git add .
git commit -m "fix: Patch critical security vulnerability"
git push origin hotfix/critical-security-fix

# 4. Create PR → main
# 5. After approval, deploys to production
# 6. IMPORTANT: Merge back to develop
git checkout develop
git merge hotfix/critical-security-fix
git push origin develop
```

---

## Release Process

### Weekly Release (develop → main)

**Every Friday (or bi-weekly):**

1. **Verify staging is stable**
   - All features tested
   - India team confirms functionality
   - No critical bugs

2. **Create release PR**
   ```bash
   # From develop
   git checkout develop && git pull

   # Push any final changes
   git push origin develop
   ```

   - Create PR: `develop` → `main`
   - Title: `Release: Week of YYYY-MM-DD`
   - Description: List of changes/features

3. **Get approval**
   - Team lead reviews
   - Approves deployment

4. **Merge to main**
   - Production deployment starts
   - Monitor logs
   - Verify production health

5. **Tag the release**
   ```bash
   git checkout main && git pull
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

---

## Rollback Strategy

### Staging Rollback
If staging deployment fails:
```bash
# Revert the merge commit
git checkout develop
git revert -m 1 <merge-commit-hash>
git push origin develop
# Staging redeploys with previous version
```

### Production Rollback
If production deployment fails or has critical bug:

**Option 1: Revert via Git**
```bash
git checkout main
git revert -m 1 <merge-commit-hash>
git push origin main
# Production redeploys with previous version
```

**Option 2: Redeploy Previous Release**
- Go to Railway production dashboard
- Find previous successful deployment
- Click "Redeploy"

---

## Best Practices

### Commits
- ✅ Use conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`
- ✅ Keep commits small and focused
- ✅ Write clear commit messages
- ✅ Reference issue numbers: `fix: Resolve payment timeout (#123)`

### Pull Requests
- ✅ Fill out PR template
- ✅ Link related issues
- ✅ Add screenshots for UI changes
- ✅ Request review from team members
- ✅ Wait for CI to pass before merge

### Testing
- ✅ Write tests for new features
- ✅ Test locally before pushing
- ✅ Verify in staging before production
- ✅ Run integration tests: `go test -tags=integration ./...`

### Deployment
- ✅ Always deploy to staging first
- ✅ Test thoroughly in staging
- ✅ Deploy to production during low-traffic hours
- ✅ Monitor logs after deployment
- ✅ Have rollback plan ready

---

## Monitoring Deployments

### Check Deployment Status

**Staging:**
```bash
# Health check
curl https://payment-service-staging.up.railway.app/cron/health

# View logs
railway logs --project staging
```

**Production:**
```bash
# Health check
curl https://payments.yourdomain.com/cron/health

# View logs
railway logs --project production
```

### GitHub Actions
- Go to: https://github.com/kevin07696/payment-service/actions
- View workflow runs
- Check job status
- Review logs

---

## Quick Reference

| Task | Command | Branch |
|------|---------|--------|
| New feature | `git checkout -b feature/name` | from `develop` |
| Bug fix | `git checkout -b bugfix/name` | from `develop` |
| Hotfix | `git checkout -b hotfix/name` | from `main` |
| Deploy staging | `git push origin develop` | `develop` |
| Deploy production | `git push origin main` (after PR) | `main` |
| Rollback staging | `git revert <commit>` on `develop` | `develop` |
| Rollback production | `git revert <commit>` on `main` | `main` |

---

## Support

- Staging Issues: Check Railway staging logs
- Production Issues: Check Railway production logs + contact on-call
- CI/CD Issues: Check GitHub Actions logs
- Git Help: `git --help` or ask team lead
