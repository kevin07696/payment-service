# Railway Staging Deployment Setup

This guide walks you through setting up Railway staging deployment with automated CI/CD.

## Branching Strategy

We use a Git Flow workflow:
- **`develop` branch** → Deploys to **Railway Staging** (automatic)
- **`main` branch** → Deploys to **Railway Production** (with approval)

See [BRANCHING_STRATEGY.md](./BRANCHING_STRATEGY.md) for complete workflow details.

## Prerequisites

- GitHub account with repository access
- Railway account (sign up at https://railway.app)
- EPX sandbox credentials

---

## Step 1: Create Railway Project

### 1.1 Sign up for Railway
1. Go to https://railway.app
2. Sign up with your GitHub account
3. You'll get $5 free credit and 30-day trial

### 1.2 Create New Project
1. Click **"New Project"**
2. Select **"Deploy from GitHub repo"**
3. Choose `kevin07696/payment-service`
4. Railway will automatically detect it's a Go project

### 1.3 Add PostgreSQL Database
1. In your Railway project, click **"New"** → **"Database"** → **"PostgreSQL"**
2. Railway will automatically provision the database
3. The `DATABASE_URL` environment variable will be set automatically

---

## Step 2: Configure Railway Service Settings

### 2.1 Enable CI/CD Integration
1. Go to your service → **Settings** → **Deploy**
2. Enable **"Wait for CI"** ✅
   - This makes Railway wait for GitHub Actions to pass before deploying
3. Enable **"Teardown idle builds"** ✅
   - Saves costs by removing idle deployments

### 2.2 Enable Serverless (Optional but Recommended)
1. Go to **Settings** → **Deploy**
2. Check **"Enable Serverless"** ✅
   - Scales to zero when idle (~$1-2/month vs $3-5/month)
   - Accept 30-second cold start delay

### 2.3 Set Health Check
1. Go to **Settings** → **Deploy**
2. Set **Health Check Path**: `/cron/health`
3. Set **Health Check Timeout**: `100` seconds

---

## Step 3: Get Railway Credentials

### 3.1 Get Railway Token
1. Go to Railway dashboard → **Account Settings** → **Tokens**
2. Click **"Create New Token"**
3. Name it: `github-actions-staging`
4. Copy the token (you'll need this for GitHub secrets)

### 3.2 Get Railway Project ID
1. Go to your Railway project
2. Click **Settings** (gear icon)
3. Copy the **Project ID** from the URL or settings page
   - Format: `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`

---

## Step 4: Configure GitHub Secrets

Go to your GitHub repository → **Settings** → **Secrets and variables** → **Actions**

Add the following secrets:

### Required Secrets

| Secret Name | Value | Description |
|-------------|-------|-------------|
| `RAILWAY_TOKEN` | `your-railway-token` | Token from Railway Account Settings |
| `RAILWAY_PROJECT_ID` | `your-project-id` | Project ID from Railway project settings |
| `EPX_MAC_STAGING` | `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` | EPX Merchant Authorization Code (sandbox) |
| `CRON_SECRET_STAGING` | Generate with `openssl rand -base64 32` | Secret for cron endpoint authentication |
| `CALLBACK_BASE_URL_STAGING` | `https://your-app.up.railway.app` | Railway app URL (update after first deploy) |

### How to Add Secrets
1. Click **"New repository secret"**
2. Enter **Name** (e.g., `RAILWAY_TOKEN`)
3. Enter **Value**
4. Click **"Add secret"**
5. Repeat for all required secrets

---

## Step 5: Initial Deployment

### 5.1 First Deployment (Manual)

Since `CALLBACK_BASE_URL_STAGING` isn't known yet, you'll deploy twice:

**Option A: Deploy via Railway Dashboard**
1. Go to Railway project
2. Click **"Deploy"**
3. Wait for deployment to complete
4. Note your Railway URL: `https://payment-service-staging-xxxxx.up.railway.app`

**Option B: Deploy via GitHub Actions**
1. Temporarily set `CALLBACK_BASE_URL_STAGING` to `http://localhost:8081`
2. Push to main branch
3. GitHub Actions will deploy
4. Note the deployment URL from logs

### 5.2 Update Callback URL
1. Copy your Railway app URL
2. Update GitHub secret `CALLBACK_BASE_URL_STAGING`:
   - Go to **Settings** → **Secrets and variables** → **Actions**
   - Click `CALLBACK_BASE_URL_STAGING`
   - Click **"Update secret"**
   - Enter your Railway URL: `https://payment-service-staging-xxxxx.up.railway.app`
   - Click **"Update secret"**

### 5.3 Redeploy with Correct Callback URL
1. Make a trivial change (e.g., add comment to README)
2. Commit and push to main
3. GitHub Actions will automatically:
   - Run tests
   - Build Docker image
   - Run migrations
   - Update environment variables (including new callback URL)
   - Deploy to Railway

---

## Step 6: Verify Deployment

### 6.1 Check GitHub Actions
1. Go to **Actions** tab in your GitHub repo
2. Find the latest workflow run
3. Verify all jobs passed:
   - ✅ Run Tests
   - ✅ Build Docker Image
   - ✅ Run Database Migrations (Staging)
   - ✅ Deploy to Railway Staging

### 6.2 Check Railway Deployment
1. Go to Railway project dashboard
2. Click on your service
3. Check **Deployments** tab
4. Latest deployment should show **"Active"**

### 6.3 Test Health Endpoint
```bash
curl https://your-app.up.railway.app/cron/health
```

Expected response:
```json
{
  "status": "healthy",
  "time": "2025-11-07T19:30:00Z"
}
```

---

## CI/CD Workflow Explained

Your CI/CD pipeline now runs automatically on every push to main:

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Push to Main Branch                                      │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Run Tests (Go 1.24)                                      │
│    - Unit tests                                              │
│    - Integration tests                                       │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Build Docker Image                                       │
│    - Multi-stage build                                       │
│    - Security scanning                                       │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Run Database Migrations (Staging)                        │
│    - Connect to Railway PostgreSQL                           │
│    - Run Goose migrations                                    │
│    - Verify migration success                                │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Deploy to Railway Staging                                │
│    - Link to Railway project                                 │
│    - Update environment variables                            │
│    - Deploy application                                      │
│    - Get deployment URL                                      │
└─────────────────────────────────────────────────────────────┘
```

### Environment Variables Set by CI/CD

The deployment job automatically sets:
- Server configuration (PORT, HTTP_PORT, ENVIRONMENT)
- Database settings (DB_SSL_MODE)
- EPX sandbox credentials (CUST_NBR, MERCH_NBR, DBA_NBR, TERMINAL_NBR, MAC)
- North API configuration
- Callback URL for Browser Post
- Security settings (CRON_SECRET)
- Logging level

---

## Troubleshooting

### Issue: Railway Token Invalid
**Error:** `Authentication failed`

**Solution:**
1. Regenerate token in Railway dashboard
2. Update `RAILWAY_TOKEN` secret in GitHub
3. Retry deployment

### Issue: Migrations Fail
**Error:** `Failed to connect to database`

**Solution:**
1. Verify PostgreSQL is running in Railway
2. Check DATABASE_URL is set in Railway
3. Ensure Railway service has network access to database

### Issue: Deployment Timeout
**Error:** `Deployment timed out`

**Solution:**
1. Check Railway service logs
2. Verify health check endpoint is responding
3. Increase health check timeout in Railway settings

### Issue: Environment Variables Not Set
**Error:** `Missing required environment variable`

**Solution:**
1. Check GitHub secrets are configured correctly
2. Verify `railway variables set` commands in workflow
3. Check Railway dashboard → Variables tab

---

## Cost Management

### Current Configuration
- **Database**: PostgreSQL (~$1-2/month with free credit)
- **Service**: With serverless enabled (~$1-2/month with free credit)
- **Total**: ~$2-4/month (covered by $5 free credit)

### Cost Optimization Tips
1. ✅ Enable serverless mode (scale to zero)
2. ✅ Enable teardown idle builds
3. ✅ Use rate limiting (already configured)
4. Set resource limits in `railway.toml` (already configured)

### Monitor Usage
1. Go to Railway dashboard → **Usage**
2. Check monthly costs
3. Set up billing alerts

---

## Next Steps

Once staging is working:
1. Test all payment flows in staging
2. Monitor logs and errors
3. Configure production environment (separate Railway project)
4. Update workflow to deploy to production from `production` branch

---

## Support

- Railway Docs: https://docs.railway.app
- Railway Discord: https://discord.gg/railway
- GitHub Actions Docs: https://docs.github.com/actions
- EPX Support: Contact North/EPX support team
