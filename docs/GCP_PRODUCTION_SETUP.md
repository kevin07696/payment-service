# Google Cloud Run Production Deployment Setup

This guide walks you through setting up Google Cloud Run production deployment with automated CI/CD.

## Architecture Overview

```
GitHub Actions (main branch)
    ↓
Run Tests & Build
    ↓
Run Migrations (Cloud SQL via Proxy)
    ↓
Build & Push Docker Image (Artifact Registry)
    ↓
Deploy to Cloud Run (with approval)
    ↓
Production Live (auto-scaling containers)
```

## Prerequisites

- Google Cloud Platform account
- GitHub repository access
- EPX production credentials (from EPX support)
- Domain name for production (optional)

---

## Step 1: Google Cloud Project Setup

### 1.1 Create GCP Project

1. Go to https://console.cloud.google.com
2. Click **Select a project** → **New Project**
3. Enter project details:
   - **Project name**: `payment-service-production`
   - **Project ID**: (auto-generated or custom, note this down)
   - **Billing account**: Select or create billing account
4. Click **Create**

### 1.2 Enable Required APIs

Navigate to **APIs & Services** → **Library** and enable:

```bash
# Or use gcloud CLI to enable all at once:
gcloud services enable \
  run.googleapis.com \
  sqladmin.googleapis.com \
  artifactregistry.googleapis.com \
  cloudresourcemanager.googleapis.com \
  compute.googleapis.com \
  iam.googleapis.com
```

Required APIs:
- ✅ Cloud Run API
- ✅ Cloud SQL Admin API
- ✅ Artifact Registry API
- ✅ Cloud Resource Manager API
- ✅ Compute Engine API
- ✅ IAM API

---

## Step 2: Create Cloud SQL PostgreSQL Instance

### 2.1 Create Database Instance

1. Go to **SQL** (search in top bar)
2. Click **Create Instance**
3. Choose **PostgreSQL**
4. Configure instance:

**Instance Info:**
- **Instance ID**: `payment-service-db`
- **Password**: Generate strong password (save securely)
- **Database version**: PostgreSQL 15
- **Region**: Choose nearest region (e.g., `us-central1`)
- **Zonal availability**: Single zone (cheaper) or Multiple zones (HA)

**Machine Configuration:**
- **Preset**: Shared core (1 vCPU, 0.614 GB) for testing
- **Preset**: Dedicated core (1-2 vCPU, 3.75 GB) for production
- **Storage**: 10 GB SSD (auto-increase enabled)

**Connections:**
- ✅ Private IP: Leave unchecked for now
- ✅ Public IP: Enable (we'll use Cloud SQL Proxy)
- **Authorized networks**: Leave empty (Cloud SQL Proxy handles this)

**Backups:**
- ✅ Automated backups: Enable
- **Backup window**: Choose low-traffic time

5. Click **Create Instance** (takes 5-10 minutes)

### 2.2 Create Database and User

Once instance is ready:

1. Click on your instance name
2. Go to **Databases** tab
3. Click **Create Database**
   - **Database name**: `payment_service`
   - Click **Create**

4. Go to **Users** tab
5. Click **Add User Account**
   - **Username**: `payment_service_user`
   - **Password**: Generate strong password (save securely)
   - Click **Add**

### 2.3 Note Connection Details

From the **Overview** tab, note these values:

- **Connection name**: `project-id:region:instance-id` (e.g., `payment-service-prod-123456:us-central1:payment-service-db`)
- **Public IP address**: Note this down
- **Region**: Confirm region

---

## Step 3: Create Artifact Registry Repository

This stores your Docker images.

### 3.1 Create Repository

1. Go to **Artifact Registry** → **Repositories**
2. Click **Create Repository**
3. Configure:
   - **Name**: `payment-service`
   - **Format**: Docker
   - **Mode**: Standard
   - **Location type**: Region
   - **Region**: Same as Cloud SQL (e.g., `us-central1`)
   - **Encryption**: Google-managed
4. Click **Create**

### 3.2 Note Repository URL

Format: `{region}-docker.pkg.dev/{project-id}/{repository-name}`

Example: `us-central1-docker.pkg.dev/payment-service-prod-123456/payment-service`

---

## Step 4: Create Service Account for CI/CD

### 4.1 Create Service Account

1. Go to **IAM & Admin** → **Service Accounts**
2. Click **Create Service Account**
3. Configure:
   - **Service account name**: `github-actions-deployer`
   - **Service account ID**: `github-actions-deployer`
   - **Description**: Service account for GitHub Actions CI/CD
4. Click **Create and Continue**

### 4.2 Grant Permissions

Add these roles:

- **Cloud Run Admin** (`roles/run.admin`)
  - Deploy and manage Cloud Run services
- **Cloud SQL Client** (`roles/cloudsql.client`)
  - Connect to Cloud SQL via proxy for migrations
- **Artifact Registry Writer** (`roles/artifactregistry.writer`)
  - Push Docker images
- **Service Account User** (`roles/iam.serviceAccountUser`)
  - Act as service accounts
- **Storage Admin** (`roles/storage.admin`)
  - Manage Cloud Storage (for Cloud Run)

5. Click **Continue** → **Done**

### 4.3 Create JSON Key

1. Click on the service account you just created
2. Go to **Keys** tab
3. Click **Add Key** → **Create new key**
4. Choose **JSON** format
5. Click **Create**
6. **Save this JSON file securely** (you'll add it to GitHub secrets)

**⚠️ IMPORTANT:** Never commit this JSON file to Git. Store it securely.

---

## Step 5: Configure GitHub Secrets

Go to your GitHub repository → **Settings** → **Secrets and variables** → **Actions**

### 5.1 Create Production Environment

1. Go to **Settings** → **Environments**
2. Click **New environment**
3. Name: `production`
4. Click **Configure environment**

**Protection Rules:**
- ✅ **Required reviewers**: Check this
  - Add 1-2 team members who must approve production deployments
- **Deployment branches**: Selected branches
  - Add rule for `main` branch only

### 5.2 Add Environment Secrets

Click **Add secret** for each of these in the **production** environment:

#### Google Cloud Platform Secrets

| Secret Name | Value | How to Get |
|-------------|-------|------------|
| `GCP_SA_KEY` | Service account JSON key content | Paste entire JSON file content from Step 4.3 |
| `GCP_PROJECT_ID` | Your GCP project ID | From GCP Console → Project Info |
| `GCP_REGION` | Cloud Run region | e.g., `us-central1` |
| `GCP_DB_INSTANCE_CONNECTION_NAME` | Cloud SQL connection name | Format: `project-id:region:instance-id` |
| `GCP_DB_USER` | Database username | `payment_service_user` (from Step 2.2) |
| `GCP_DB_PASSWORD` | Database password | Password you created in Step 2.2 |
| `GCP_DB_NAME` | Database name | `payment_service` (from Step 2.2) |
| `GCP_DATABASE_URL` | Full PostgreSQL connection string | See format below |

**GCP_DATABASE_URL Format:**
```
postgresql://payment_service_user:YOUR_PASSWORD@/payment_service?host=/cloudsql/PROJECT:REGION:INSTANCE&sslmode=disable
```

Example:
```
postgresql://payment_service_user:MyS3cr3tP@ss@/payment_service?host=/cloudsql/payment-service-prod:us-central1:payment-service-db&sslmode=disable
```

#### EPX Production Credentials

| Secret Name | Value | How to Get |
|-------------|-------|------------|
| `EPX_CUST_NBR_PRODUCTION` | Your production customer number | Contact EPX/North support |
| `EPX_MERCH_NBR_PRODUCTION` | Your production merchant number | Contact EPX/North support |
| `EPX_DBA_NBR_PRODUCTION` | Your production DBA number | Contact EPX/North support |
| `EPX_TERMINAL_NBR_PRODUCTION` | Your production terminal number | Contact EPX/North support |
| `EPX_MAC_PRODUCTION` | Your production MAC token | Contact EPX/North support |

#### Application Secrets

| Secret Name | Value | How to Get |
|-------------|-------|------------|
| `CRON_SECRET_PRODUCTION` | Random secret string | Generate: `openssl rand -base64 32` |
| `CALLBACK_BASE_URL_PRODUCTION` | Your production domain/URL | e.g., `https://payments.yourdomain.com` |

**Total: 15 secrets**

---

## Step 6: Initial Deployment

### 6.1 Test Connection (Optional)

Test Cloud SQL connection locally using Cloud SQL Proxy:

```bash
# Download Cloud SQL Proxy
wget https://dl.google.com/cloudsql/cloud_sql_proxy.linux.amd64 -O cloud_sql_proxy
chmod +x cloud_sql_proxy

# Run proxy (replace with your connection name)
./cloud_sql_proxy -instances=PROJECT:REGION:INSTANCE=tcp:5432

# In another terminal, test connection
psql "postgresql://payment_service_user:PASSWORD@localhost:5432/payment_service"
```

### 6.2 Run Migrations Manually (First Time)

Before first deployment, run migrations manually:

```bash
# Set environment variables
export DB_HOST=127.0.0.1
export DB_PORT=5432
export DB_USER=payment_service_user
export DB_PASSWORD=your_password
export DB_NAME=payment_service

# Build connection string
DATABASE_URL="postgresql://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

# Run migrations
goose -dir migrations postgres "$DATABASE_URL" up
```

### 6.3 Trigger Production Deployment

1. **Merge develop to main:**
   ```bash
   git checkout main
   git pull origin main
   git merge develop
   git push origin main
   ```

2. **Monitor GitHub Actions:**
   - Go to **Actions** tab
   - Watch the workflow run
   - Jobs will run: Test → Build → Migrate Production → (Wait for Approval) → Deploy Production

3. **Approve Deployment:**
   - After migrations complete, you'll get a notification
   - Click **Review deployments**
   - Check **production**
   - Click **Approve and deploy**

4. **Monitor Cloud Run Deployment:**
   - GitHub Actions will deploy to Cloud Run
   - Wait 2-3 minutes for deployment
   - Check logs for any errors

### 6.4 Get Production URL

After deployment completes:

**Option 1: From GitHub Actions Logs**
- Look for output in "Get Service URL" step
- Copy the Cloud Run URL

**Option 2: From GCP Console**
```bash
gcloud run services describe payment-service \
  --region us-central1 \
  --format 'value(status.url)'
```

**Option 3: From Cloud Run Console**
1. Go to **Cloud Run** in GCP Console
2. Click on `payment-service`
3. Copy the URL at the top

Example URL: `https://payment-service-abc123xyz-uc.a.run.app`

---

## Step 7: Verify Deployment

### 7.1 Health Check

```bash
curl https://payment-service-abc123xyz-uc.a.run.app/cron/health
```

Expected response:
```json
{
  "status": "healthy",
  "time": "2025-11-07T20:00:00Z"
}
```

### 7.2 Check Cloud Run Logs

1. Go to **Cloud Run** → `payment-service`
2. Click **Logs** tab
3. Filter by severity if needed
4. Verify no startup errors

### 7.3 Test Database Connection

Check logs for successful database connection:
```
Successfully connected to database
Running migrations...
Server started on :8080
```

---

## Step 8: Configure Custom Domain (Optional)

### 8.1 Add Domain to Cloud Run

1. Go to **Cloud Run** → `payment-service`
2. Click **Manage Custom Domains**
3. Click **Add Mapping**
4. Select service: `payment-service`
5. Enter domain: `payments.yourdomain.com`
6. Click **Continue**

### 8.2 Update DNS

Cloud Run will provide DNS records. Add to your DNS provider:

```
Type: CNAME
Name: payments
Value: ghs.googlehosted.com
```

Or A/AAAA records if provided.

### 8.3 Wait for Verification

- SSL certificate provisioning: 15-60 minutes
- Check status in Cloud Run console
- Once verified, domain will show ✅

### 8.4 Update Callback URL

After custom domain is working:

1. Update GitHub secret `CALLBACK_BASE_URL_PRODUCTION`
   - New value: `https://payments.yourdomain.com`
2. Trigger redeployment:
   ```bash
   git commit --allow-empty -m "chore: Update callback URL"
   git push origin main
   ```

---

## CI/CD Pipeline Explained

### Workflow Triggers

```yaml
on:
  push:
    branches: [main]  # Only main triggers production
```

### Pipeline Stages

```
1. Push to Main Branch
    ↓
2. Run Tests (Go 1.24)
    - Unit tests
    - Integration tests
    ↓
3. Build Docker Image
    - Multi-stage build
    - Security scanning
    ↓
4. Run Database Migrations (Production)
    - Authenticate to GCP
    - Start Cloud SQL Proxy
    - Run Goose migrations
    - Verify success
    ↓
5. ⏸️ Wait for Manual Approval
    - Notification sent to reviewers
    - Reviewer approves/rejects
    ↓
6. Deploy to Cloud Run (After Approval)
    - Authenticate to GCP
    - Configure Docker for Artifact Registry
    - Build and tag Docker image
    - Push to Artifact Registry
    - Deploy to Cloud Run with config
    - Get service URL
    ↓
7. Production Live ✅
```

### Environment Variables Set by CI/CD

The deployment automatically configures:

**Server Configuration:**
- `PORT=8080` (gRPC server)
- `HTTP_PORT=8081` (HTTP server)
- `ENVIRONMENT=production`
- `DB_SSL_MODE=require`
- `LOG_LEVEL=info`

**EPX Production Credentials:**
- `EPX_BASE_URL=https://secure.epxnow.com`
- `EPX_TIMEOUT=30`
- `EPX_CUST_NBR` (from secret)
- `EPX_MERCH_NBR` (from secret)
- `EPX_DBA_NBR` (from secret)
- `EPX_TERMINAL_NBR` (from secret)
- `EPX_MAC` (from secret)

**North API:**
- `NORTH_API_URL=https://api.north.com`
- `NORTH_TIMEOUT=30`

**Application:**
- `CALLBACK_BASE_URL` (from secret)
- `CRON_SECRET` (from secret)
- `DATABASE_URL` (from secret, for Cloud SQL connection)

**Cloud Run Configuration:**
- CPU: 1 vCPU
- Memory: 512 Mi
- Min instances: 0 (scale to zero)
- Max instances: 10
- Timeout: 300 seconds
- Concurrency: 80 requests per container
- Cloud SQL connection via Unix socket

---

## Cost Management

### Estimated Monthly Costs

**Cloud SQL (Shared Core - Testing):**
- Instance: ~$7-10/month
- Storage (10GB): ~$1.70/month
- Backups: ~$0.50/month
- **Subtotal: ~$9-12/month**

**Cloud SQL (Dedicated Core - Production):**
- Instance (1 vCPU, 3.75GB): ~$40-50/month
- Storage (20GB): ~$3.40/month
- Backups: ~$2/month
- **Subtotal: ~$45-55/month**

**Cloud Run:**
- CPU-seconds: $0.00002400 per vCPU-second
- Memory: $0.00000250 per GiB-second
- Requests: $0.40 per million requests
- With free tier: 2 million requests/month free
- **Estimated: $5-15/month** (low traffic)

**Artifact Registry:**
- Storage: $0.10 per GB/month
- ~10 images × 100MB each = 1GB
- **Estimated: ~$0.10-1/month**

**Total Estimated Cost:**
- Testing/Low Traffic: $15-30/month
- Production with Moderate Traffic: $50-75/month

### Cost Optimization Tips

1. **Enable Scale to Zero**
   ```yaml
   --min-instances 0
   ```
   Cloud Run scales to 0 when idle (already configured)

2. **Set Up Budget Alerts**
   - Go to **Billing** → **Budgets & alerts**
   - Create budget: $50/month
   - Set alert at 50%, 90%, 100%

3. **Use Committed Use Discounts**
   - For Cloud SQL, commit to 1-3 years for 25-50% discount
   - Only if you're sure about production usage

4. **Monitor Usage**
   ```bash
   # Check Cloud Run metrics
   gcloud run services describe payment-service --region=us-central1

   # View costs
   gcloud billing accounts describe BILLING_ACCOUNT_ID
   ```

5. **Optimize Container**
   - Use multi-stage builds (already configured)
   - Minimize image size
   - Enable HTTP/2 and connection pooling

---

## Monitoring and Logging

### Cloud Run Logs

**View Real-time Logs:**
```bash
gcloud run services logs read payment-service \
  --region=us-central1 \
  --limit=50 \
  --follow
```

**Filter by Severity:**
```bash
gcloud run services logs read payment-service \
  --region=us-central1 \
  --log-filter="severity>=ERROR"
```

### Cloud SQL Logs

1. Go to **SQL** → Instance → **Logs**
2. Filter by log type:
   - Error logs
   - Slow query logs
   - Connection logs

### Set Up Alerts

**High Error Rate Alert:**
1. Go to **Cloud Run** → `payment-service`
2. Click **Metrics** tab
3. Click **Create Alert**
4. Configure:
   - Metric: Request count (filtered by 5xx)
   - Threshold: > 10 errors in 5 minutes
   - Notification: Email/SMS

**High Latency Alert:**
- Metric: Request latency
- Threshold: p99 > 2 seconds
- Window: 5 minutes

---

## Troubleshooting

### Issue: Migrations Fail to Connect

**Error:** `Failed to connect to Cloud SQL`

**Solution:**
1. Verify Cloud SQL instance is running
2. Check `GCP_DB_INSTANCE_CONNECTION_NAME` format:
   - Correct: `project:region:instance`
   - Incorrect: `project/region/instance`
3. Verify service account has `cloudsql.client` role
4. Check Cloud SQL Proxy logs in GitHub Actions

### Issue: Docker Build Fails

**Error:** `Failed to push to Artifact Registry`

**Solution:**
1. Verify Artifact Registry API is enabled
2. Check service account has `artifactregistry.writer` role
3. Verify repository exists in correct region
4. Check Docker authentication:
   ```bash
   gcloud auth configure-docker us-central1-docker.pkg.dev
   ```

### Issue: Cloud Run Deployment Times Out

**Error:** `Deployment timed out waiting for health check`

**Solution:**
1. Check application starts correctly (logs)
2. Verify health check endpoint responds: `/cron/health`
3. Increase Cloud Run timeout:
   ```yaml
   --timeout 300
   ```
4. Check database connection string is correct

### Issue: Service Account Permission Denied

**Error:** `Permission denied on Cloud Run service`

**Solution:**
1. Verify service account has all required roles
2. Add missing roles in IAM console:
   ```bash
   gcloud projects add-iam-policy-binding PROJECT_ID \
     --member="serviceAccount:github-actions-deployer@PROJECT_ID.iam.gserviceaccount.com" \
     --role="roles/run.admin"
   ```

### Issue: Environment Variables Not Set

**Error:** `Missing required environment variable EPX_MAC`

**Solution:**
1. Check all 15 GitHub secrets are configured
2. Verify they're in the `production` environment (not repository secrets)
3. Check spelling of secret names (case-sensitive)
4. Re-run deployment after adding missing secrets

### Issue: Cold Start Latency Too High

**Problem:** First request takes 10-30 seconds

**Solution:**
1. Enable minimum instances (costs more):
   ```yaml
   --min-instances 1
   ```
2. Optimize Docker image size
3. Use startup probes with longer timeout
4. Consider Cloud Run Gen2 execution environment

---

## Security Best Practices

### 1. Service Account Permissions

- ✅ Use dedicated service account for CI/CD
- ✅ Grant minimum necessary roles
- ✅ Never share service account keys
- ✅ Rotate keys every 90 days

### 2. Database Security

- ✅ Use Cloud SQL Proxy (never expose database publicly)
- ✅ Enable SSL/TLS for connections
- ✅ Use strong passwords (16+ characters)
- ✅ Limit database user permissions
- ✅ Enable automated backups
- ✅ Test restore process regularly

### 3. Secrets Management

- ✅ Use GitHub environment secrets (not repository secrets)
- ✅ Never commit secrets to Git
- ✅ Rotate secrets regularly
- ✅ Use Secret Manager for sensitive data (optional upgrade)

### 4. Network Security

- ✅ Use VPC if needed (advanced)
- ✅ Enable Cloud Armor for DDoS protection (optional)
- ✅ Configure rate limiting (already in application)
- ✅ Use Cloud CDN for static assets (optional)

### 5. Application Security

- ✅ Keep dependencies updated
- ✅ Enable security scanning in Docker builds
- ✅ Use HTTPS only (enforced by Cloud Run)
- ✅ Validate all input
- ✅ Log security events

---

## Rollback Strategy

### Quick Rollback via Cloud Run Console

1. Go to **Cloud Run** → `payment-service`
2. Click **Revisions** tab
3. Find previous stable revision
4. Click **⋮** (three dots) → **Manage traffic**
5. Set 100% traffic to previous revision
6. Click **Save**

**Downtime:** ~10-30 seconds

### Rollback via Git Revert

```bash
# Revert the problematic commit on main
git checkout main
git revert HEAD
git push origin main

# Or revert specific commit
git revert <commit-hash>
git push origin main
```

Triggers automatic redeployment with reverted code.

### Rollback Database Migrations

**⚠️ Careful:** Database rollbacks can cause data loss.

```bash
# Connect via Cloud SQL Proxy
./cloud_sql_proxy -instances=PROJECT:REGION:INSTANCE=tcp:5432 &

# Rollback one migration
goose -dir migrations postgres "$DATABASE_URL" down

# Rollback to specific version
goose -dir migrations postgres "$DATABASE_URL" down-to VERSION
```

**Best Practice:** Test rollback in staging first.

---

## Disaster Recovery

### Database Backup and Restore

**List Backups:**
```bash
gcloud sql backups list --instance=payment-service-db
```

**Create On-Demand Backup:**
```bash
gcloud sql backups create --instance=payment-service-db
```

**Restore from Backup:**
```bash
gcloud sql backups restore BACKUP_ID \
  --backup-instance=payment-service-db \
  --backup-instance=payment-service-db
```

### Export Database to Cloud Storage

```bash
gcloud sql export sql payment-service-db gs://my-backup-bucket/backup-$(date +%Y%m%d).sql \
  --database=payment_service
```

### Point-in-Time Recovery

Cloud SQL supports point-in-time recovery:

1. Go to **SQL** → Instance → **Backups**
2. Click **Create Clone**
3. Choose point in time
4. Creates new instance with data at that timestamp

---

## Next Steps After Setup

### 1. Test Production Deployment

- [ ] Verify health check endpoint
- [ ] Test a small EPX transaction (use test cards)
- [ ] Monitor logs for errors
- [ ] Check database connectivity
- [ ] Verify Browser Post callback works

### 2. Set Up Monitoring

- [ ] Create alerting policies
- [ ] Set up uptime checks
- [ ] Configure log-based metrics
- [ ] Create dashboard in Cloud Monitoring

### 3. Performance Tuning

- [ ] Review Cloud Run metrics after 1 week
- [ ] Adjust CPU/memory if needed
- [ ] Optimize database queries
- [ ] Configure connection pooling

### 4. Documentation

- [ ] Document production EPX credentials location
- [ ] Create runbook for common issues
- [ ] Document rollback procedures
- [ ] Share production URLs with team

---

## Support Resources

- **Google Cloud Documentation:** https://cloud.google.com/run/docs
- **Cloud SQL Docs:** https://cloud.google.com/sql/docs
- **Artifact Registry Docs:** https://cloud.google.com/artifact-registry/docs
- **Cloud Run Pricing:** https://cloud.google.com/run/pricing
- **Support:** https://cloud.google.com/support

---

## Appendix: Required GitHub Secrets Summary

### Production Environment Secrets (15 total)

#### GCP Infrastructure (8 secrets)
- `GCP_SA_KEY` - Service account JSON key
- `GCP_PROJECT_ID` - GCP project ID
- `GCP_REGION` - Deployment region (e.g., us-central1)
- `GCP_DB_INSTANCE_CONNECTION_NAME` - Format: project:region:instance
- `GCP_DB_USER` - Database username
- `GCP_DB_PASSWORD` - Database password
- `GCP_DB_NAME` - Database name
- `GCP_DATABASE_URL` - Full PostgreSQL connection string

#### EPX Production (5 secrets)
- `EPX_CUST_NBR_PRODUCTION` - Customer number
- `EPX_MERCH_NBR_PRODUCTION` - Merchant number
- `EPX_DBA_NBR_PRODUCTION` - DBA number
- `EPX_TERMINAL_NBR_PRODUCTION` - Terminal number
- `EPX_MAC_PRODUCTION` - MAC token

#### Application (2 secrets)
- `CRON_SECRET_PRODUCTION` - Cron endpoint authentication
- `CALLBACK_BASE_URL_PRODUCTION` - Production domain URL
