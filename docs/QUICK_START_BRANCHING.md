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
│  ├─ Auto-deploys to Railway Staging                 │
│  └─ EPX Sandbox credentials                         │
└─────────────────────────────────────────────────────┘
                      │
                      │ (Create PR when ready)
                      ▼
┌─────────────────────────────────────────────────────┐
│  main branch                                        │
│  ├─ Merge from develop                              │
│  ├─ Deploys to Railway Production                   │
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
#    Test at: https://payment-service-staging.up.railway.app
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
4. ✅ **Deploy to Railway Staging** (automatic, no approval)

### On Push to `main`:
1. ✅ Run all tests
2. ✅ Build Docker image
3. ✅ Run migrations on production database
4. ⏸️ **Wait for approval** (you'll get notified)
5. ✅ **Deploy to Railway Production** (after you approve)

---

## Environment URLs

| Environment | Branch | URL | Auto-Deploy |
|-------------|--------|-----|-------------|
| **Staging** | `develop` | https://payment-service-staging.up.railway.app | ✅ Yes |
| **Production** | `main` | https://payments.yourdomain.com | ⏸️ After approval |

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
# Staging health check
curl https://payment-service-staging.up.railway.app/cron/health

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

**Secrets:**
- `RAILWAY_TOKEN` (same as current)
- `RAILWAY_PROJECT_ID` (same as current)
- `EPX_MAC_STAGING` = `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y`
- `CRON_SECRET_STAGING` (generate with `openssl rand -base64 32`)
- `CALLBACK_BASE_URL_STAGING` (your Railway staging URL)

### 2. Production Environment
**Setup:** GitHub → Settings → Environments → New environment → "production"

**Protection Rules:**
- ✅ Required reviewers: CHECK (1 person)
- Deployment branches: `main` only

**Secrets:**
- `RAILWAY_TOKEN_PRODUCTION` (from Railway production project)
- `RAILWAY_PROJECT_ID_PRODUCTION` (from Railway production project)
- `EPX_CUST_NBR_PRODUCTION` (get from EPX)
- `EPX_MERCH_NBR_PRODUCTION` (get from EPX)
- `EPX_DBA_NBR_PRODUCTION` (get from EPX)
- `EPX_TERMINAL_NBR_PRODUCTION` (get from EPX)
- `EPX_MAC_PRODUCTION` (get from EPX)
- `CRON_SECRET_PRODUCTION` (generate with `openssl rand -base64 32`)
- `CALLBACK_BASE_URL_PRODUCTION` (your production domain)

---

## Next Steps

### Right Now (For Staging):
1. ✅ Create "staging" environment in GitHub
2. ✅ Add 5 secrets to staging environment
3. ✅ Set up Railway staging project
4. ✅ Push to develop → test deployment

### Later (For Production):
1. Create "production" environment in GitHub
2. Add 9 secrets to production environment
3. Set up Railway production project (separate from staging)
4. Add production protection rules (require approval)
5. Get production EPX credentials from EPX support

---

## Full Documentation

- **Complete Git Workflow:** [BRANCHING_STRATEGY.md](./BRANCHING_STRATEGY.md)
- **Railway Setup:** [RAILWAY_SETUP.md](./RAILWAY_SETUP.md)
- **GitHub Environments:** [GITHUB_ENVIRONMENT_SETUP.md](./GITHUB_ENVIRONMENT_SETUP.md)
- **Deployment Checklist:** [RAILWAY_SETUP_CHECKLIST.md](./RAILWAY_SETUP_CHECKLIST.md)

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
