# GitHub Environment Setup Guide

This guide shows you how to create the "staging" environment in GitHub for CI/CD deployment.

## Step 1: Create Staging Environment

1. Go to your repository: https://github.com/kevin07696/payment-service
2. Click **Settings** (top right, next to About)
3. In left sidebar, scroll down to **Environments**
4. Click **New environment** button
5. Enter environment name: `staging`
6. Click **Configure environment**

## Step 2: Configure Environment (Optional Protection)

After creating the environment, you'll see the configuration page:

### Deployment Protection Rules (Optional)

**For Staging (Auto-deploy):**
- ☐ Required reviewers: Leave UNCHECKED
- ☐ Wait timer: Leave at 0 minutes
- ✅ Deployment branches: Select "Selected branches"
  - Click "Add deployment branch rule"
  - Select branch: `main`
  - Click "Add rule"

This ensures only pushes to `main` trigger staging deployment.

### Environment Secrets (Recommended)

Instead of using repository-wide secrets, you can add secrets specific to this environment:

Click **Add secret** and add each of these:

| Secret Name | Value | Notes |
|-------------|-------|-------|
| `RAILWAY_TOKEN` | Your Railway token | From Railway Account Settings |
| `RAILWAY_PROJECT_ID` | Your project ID | From Railway project settings |
| `EPX_MAC_STAGING` | `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` | EPX Merchant Authorization Code |
| `CRON_SECRET_STAGING` | Generate with `openssl rand -base64 32` | Unique secret for staging |
| `CALLBACK_BASE_URL_STAGING` | `http://localhost:8081` (temporary) | Update after first deploy |

**Benefit:** Secrets are scoped to staging environment only, more secure.

## Step 3: Verify Environment Setup

After creating the environment, verify:

1. Go to Settings → Environments
2. You should see "staging" listed
3. Click on "staging" to see:
   - Environment secrets (if you added them)
   - Deployment protection rules
   - Deployment branches

## Alternative: Use Repository Secrets

If you prefer simpler setup, you can use repository-level secrets instead:

1. Go to Settings → Secrets and variables → Actions
2. Click **Secrets** tab
3. Click **New repository secret**
4. Add all 5 secrets here

**Difference:**
- Repository secrets: Available to all workflows and environments
- Environment secrets: Only available to jobs running in that specific environment

**Recommendation:** Use environment secrets for production, repository secrets are fine for staging.

## Step 4: Test the Environment

Once the environment is created and secrets are added:

1. Make a small change to any file (e.g., add a comment to README)
2. Commit and push to main:
   ```bash
   git add .
   git commit -m "test: Trigger staging deployment"
   git push origin main
   ```
3. Go to **Actions** tab in GitHub
4. Click on the running workflow
5. You should see the "staging" environment badge on the deployment job

## Troubleshooting

### "Environment not found" Error

**Error:** `Environment 'staging' not found`

**Solution:**
1. Go to Settings → Environments
2. Verify "staging" environment exists
3. Check spelling is exactly `staging` (lowercase)

### "Secrets not found" Error

**Error:** `Secret RAILWAY_TOKEN not found`

**Solution:**
1. If using environment secrets: Go to Settings → Environments → staging → Secrets
2. If using repository secrets: Go to Settings → Secrets and variables → Actions → Secrets
3. Verify all 5 required secrets are added
4. Check secret names match exactly (case-sensitive)

### Deployment Waiting for Approval

**Issue:** Deployment is waiting and not proceeding

**Solution:**
1. Go to Settings → Environments → staging
2. Under "Deployment protection rules"
3. Uncheck "Required reviewers" for automatic deployment
4. Or manually approve the deployment from the Actions tab

## Visual Guide

### Where to Find Settings
```
GitHub Repository
├── Code
├── Issues
├── Pull requests
├── Actions
└── Settings  ← Click here
    ├── General
    ├── Collaborators
    ├── ...
    └── Environments  ← Then click here
        └── New environment  ← Create "staging"
```

### Where to Add Secrets

**Option 1: Environment Secrets (Recommended for Production)**
```
Settings
└── Environments
    └── staging  ← Click environment name
        └── Environment secrets
            └── Add secret  ← Click here
```

**Option 2: Repository Secrets (Simpler for Staging)**
```
Settings
└── Secrets and variables
    └── Actions
        └── Secrets  ← Tab
            └── New repository secret  ← Click here
```

## Next Steps

After environment setup:
1. ✅ Environment created: `staging`
2. ✅ Secrets added (5 required secrets)
3. ✅ Protection rules configured (optional)
4. Ready to deploy!

Continue with [RAILWAY_SETUP.md](./RAILWAY_SETUP.md) for Railway configuration.
