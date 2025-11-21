# GCP Production Setup

## Prerequisites

- GCP account with billing enabled
- `gcloud` CLI installed
- GitHub repository access
- EPX production credentials

## Quick Setup

```bash
# Set project variables
export PROJECT_ID="payment-service-prod"
export REGION="us-central1"
export DB_PASSWORD="$(openssl rand -base64 32)"

# Create project and enable APIs
gcloud projects create $PROJECT_ID
gcloud config set project $PROJECT_ID
gcloud services enable run.googleapis.com sqladmin.googleapis.com \
  artifactregistry.googleapis.com iam.googleapis.com

# Create Cloud SQL instance
gcloud sql instances create payment-service-db \
  --database-version=POSTGRES_15 \
  --tier=db-f1-micro \
  --region=$REGION

# Create database and user
gcloud sql databases create payment_service --instance=payment-service-db
gcloud sql users create payment_service_user \
  --instance=payment-service-db \
  --password=$DB_PASSWORD

# Create Artifact Registry
gcloud artifacts repositories create payment-service \
  --repository-format=docker \
  --location=$REGION

# Create service account
gcloud iam service-accounts create github-actions-deployer \
  --display-name="GitHub Actions Deployer"

# Grant roles
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions-deployer@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions-deployer@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/cloudsql.client"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions-deployer@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/artifactregistry.writer"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions-deployer@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountUser"

# Create and download service account key
gcloud iam service-accounts keys create gcp-key.json \
  --iam-account=github-actions-deployer@$PROJECT_ID.iam.gserviceaccount.com

# Note connection name for GitHub secrets
gcloud sql instances describe payment-service-db \
  --format='value(connectionName)'
```

## Components

### Cloud SQL PostgreSQL

| Setting | Value |
|---------|-------|
| Instance ID | `payment-service-db` |
| Version | PostgreSQL 15 |
| Tier | `db-f1-micro` (testing), `db-n1-standard-1` (production) |
| Region | Same as Cloud Run |
| Public IP | Enabled (use Cloud SQL Proxy) |
| Backups | Automated daily |
| Database | `payment_service` |
| User | `payment_service_user` |

**Connection Name Format:** `project-id:region:instance-id`

### Artifact Registry

| Setting | Value |
|---------|-------|
| Repository | `payment-service` |
| Format | Docker |
| Region | Same as Cloud Run |
| URL | `{region}-docker.pkg.dev/{project-id}/payment-service` |

### Service Account

**Name:** `github-actions-deployer@{project-id}.iam.gserviceaccount.com`

**Required Roles:**
- Cloud Run Admin (`roles/run.admin`)
- Cloud SQL Client (`roles/cloudsql.client`)
- Artifact Registry Writer (`roles/artifactregistry.writer`)
- Service Account User (`roles/iam.serviceAccountUser`)

### Cloud Run Configuration

| Setting | Value |
|---------|-------|
| Service Name | `payment-service` |
| Region | `us-central1` |
| CPU | 1 vCPU |
| Memory | 512 Mi |
| Min Instances | 0 (scale to zero) |
| Max Instances | 10 |
| Timeout | 300 seconds |
| Concurrency | 80 |

## GitHub Secrets

Configure in: **Settings → Environments → production**

### GCP Infrastructure (8)

| Secret | Value | Command to Get |
|--------|-------|----------------|
| `GCP_SA_KEY` | Service account JSON | Paste `gcp-key.json` contents |
| `GCP_PROJECT_ID` | Project ID | `gcloud config get-value project` |
| `GCP_REGION` | Region | `us-central1` |
| `GCP_DB_INSTANCE_CONNECTION_NAME` | Connection name | `gcloud sql instances describe payment-service-db --format='value(connectionName)'` |
| `GCP_DB_USER` | Database user | `payment_service_user` |
| `GCP_DB_PASSWORD` | Database password | From setup |
| `GCP_DB_NAME` | Database name | `payment_service` |
| `GCP_DATABASE_URL` | Connection string | See format below |

**GCP_DATABASE_URL Format:**
```
postgresql://payment_service_user:PASSWORD@/payment_service?host=/cloudsql/PROJECT:REGION:INSTANCE&sslmode=disable
```

### EPX Production (5)

| Secret | Value | Source |
|--------|-------|--------|
| `EPX_CUST_NBR_PRODUCTION` | Customer number | EPX support |
| `EPX_MERCH_NBR_PRODUCTION` | Merchant number | EPX support |
| `EPX_DBA_NBR_PRODUCTION` | DBA number | EPX support |
| `EPX_TERMINAL_NBR_PRODUCTION` | Terminal number | EPX support |
| `EPX_MAC_PRODUCTION` | MAC token | EPX support |

### Application (2)

| Secret | Value | Command |
|--------|-------|---------|
| `CRON_SECRET_PRODUCTION` | Random string | `openssl rand -base64 32` |
| `CALLBACK_BASE_URL_PRODUCTION` | Domain | `https://payments.yourdomain.com` |

**Total: 15 secrets**

## GitHub Environment Setup

```bash
# Create production environment in GitHub UI:
# Settings → Environments → New environment → "production"

# Protection rules:
# ✅ Required reviewers: 1-2 people
# ✅ Deployment branches: main only
```

## Deployment

### Initial Deployment

```bash
# Run migrations manually first time
./cloud_sql_proxy -instances=PROJECT:REGION:INSTANCE=tcp:5432 &

export DATABASE_URL="postgresql://payment_service_user:PASSWORD@localhost:5432/payment_service"
goose -dir migrations postgres "$DATABASE_URL" up

# Merge to main
git checkout main && git merge develop && git push origin main

# Approve deployment in GitHub Actions when prompted
```

### Automated Pipeline

```text
Push to main → Tests → Build → Migrations → Wait for approval → Deploy → Live
```

## Verification

```bash
# Get service URL
gcloud run services describe payment-service \
  --region=$REGION \
  --format='value(status.url)'

# Health check
curl $(gcloud run services describe payment-service --region=$REGION --format='value(status.url)')/cron/health

# View logs
gcloud run services logs read payment-service --region=$REGION --limit=50

# Check revisions
gcloud run revisions list --service=payment-service --region=$REGION
```

## Custom Domain (Optional)

```bash
# Add domain mapping
gcloud run domain-mappings create \
  --service=payment-service \
  --domain=payments.yourdomain.com \
  --region=$REGION

# Get DNS records
gcloud run domain-mappings describe \
  --domain=payments.yourdomain.com \
  --region=$REGION

# Add to DNS provider:
# Type: CNAME
# Name: payments
# Value: ghs.googlehosted.com
```

Wait 15-60 minutes for SSL cert provisioning.

## Monitoring

### Set Up Alerts

```bash
# Create uptime check
gcloud monitoring uptime create payment-health \
  --display-name="Payment Service Health" \
  --resource-type=uptime-url \
  --monitored-resource=https://YOUR-SERVICE-URL/cron/health

# Create alert for high error rate
# (Use Cloud Console: Monitoring → Alerting)
```

### View Metrics

```bash
# Real-time logs
gcloud run services logs read payment-service \
  --region=$REGION \
  --follow

# Filter errors
gcloud run services logs read payment-service \
  --region=$REGION \
  --log-filter="severity>=ERROR"

# Service stats
gcloud run services describe payment-service \
  --region=$REGION \
  --format="table(status.conditions[0].status,status.latestReadyRevision)"
```

## Cost Estimates

| Component | Testing | Production |
|-----------|---------|------------|
| Cloud SQL | $9-12/mo | $45-55/mo |
| Cloud Run | $5-15/mo | $15-30/mo |
| Artifact Registry | $0.10-1/mo | $1-3/mo |
| **Total** | **$15-30/mo** | **$60-90/mo** |

**Optimization:**
- Scale to zero for Cloud Run (configured)
- Set budget alerts at $50/mo
- Use committed use discounts for steady production

## Troubleshooting

### Migrations fail to connect

```bash
# Verify connection name format
echo "$GCP_DB_INSTANCE_CONNECTION_NAME"  # Should be: project:region:instance

# Check Cloud SQL Proxy logs in GitHub Actions
# Verify service account has cloudsql.client role
```

### Docker push fails

```bash
# Verify Artifact Registry API enabled
gcloud services list --enabled | grep artifactregistry

# Check service account has artifactregistry.writer role
gcloud projects get-iam-policy $PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members:github-actions-deployer"

# Authenticate Docker
gcloud auth configure-docker ${REGION}-docker.pkg.dev
```

### Deployment times out

```bash
# Check application logs for startup errors
gcloud run services logs read payment-service --region=$REGION --limit=100

# Verify health endpoint responds
curl https://YOUR-URL/cron/health

# Check database connection in Cloud Run environment variables
gcloud run services describe payment-service --region=$REGION --format="value(spec.template.spec.containers[0].env)"
```

### Permission denied

```bash
# Grant missing role
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions-deployer@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.admin"
```

### Cold start latency high

```bash
# Enable minimum instances (costs more)
gcloud run services update payment-service \
  --region=$REGION \
  --min-instances=1

# Or optimize Docker image size
```

## Rollback

### Quick Rollback (Cloud Run Console)

1. Cloud Run → payment-service → Revisions
2. Find previous stable revision
3. Manage traffic → 100% to previous revision
4. Save (10-30 second downtime)

### Git Rollback

```bash
git checkout main
git revert HEAD  # or specific commit hash
git push origin main
# Triggers automatic redeployment
```

### Database Rollback

```bash
# Connect via proxy
./cloud_sql_proxy -instances=PROJECT:REGION:INSTANCE=tcp:5432 &

# Rollback one migration
goose -dir migrations postgres "$DATABASE_URL" down

# Or to specific version
goose -dir migrations postgres "$DATABASE_URL" down-to VERSION
```

⚠️ Test database rollback in staging first

## Disaster Recovery

```bash
# List backups
gcloud sql backups list --instance=payment-service-db

# Create manual backup
gcloud sql backups create --instance=payment-service-db

# Restore from backup
gcloud sql backups restore BACKUP_ID \
  --backup-instance=payment-service-db

# Export to Cloud Storage
gcloud sql export sql payment-service-db \
  gs://my-backup-bucket/backup-$(date +%Y%m%d).sql \
  --database=payment_service
```

## References

- Cloud Run docs: https://cloud.google.com/run/docs
- Cloud SQL docs: https://cloud.google.com/sql/docs
- Pricing: https://cloud.google.com/run/pricing
- CI/CD workflow: `.github/workflows/ci-cd.yml`
- Branching: `docs/BRANCHING.md`
- Secrets: `docs/SECRETS.md`
