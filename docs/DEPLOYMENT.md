# Payment Service - Deployment Guide

This guide explains how to deploy the payment service using GitHub Actions and Fly.io.

## 📋 Prerequisites

1. **GitHub Account** with repository access
2. **Fly.io Account** (FREE tier, no credit card required)
3. **Fly.io CLI** installed locally

## 🚀 Quick Start

### 1. Install Fly.io CLI

```bash
curl -L https://fly.io/install.sh | sh
```

Add to your PATH:
```bash
export PATH="$HOME/.fly/bin:$PATH"
```

### 2. Login to Fly.io

```bash
flyctl auth login
```

This opens a browser window for authentication.

### 3. Create Fly.io Apps

**Staging:**
```bash
flyctl apps create payment-service-staging --org personal
```

**Production:**
```bash
flyctl apps create payment-service-production --org personal
```

### 4. Create PostgreSQL Databases

**Staging Database:**
```bash
flyctl postgres create \
  --name payment-service-staging-db \
  --region ord \
  --vm-size shared-cpu-1x \
  --volume-size 1 \
  --initial-cluster-size 1
```

**Production Database:**
```bash
flyctl postgres create \
  --name payment-service-production-db \
  --region ord \
  --vm-size shared-cpu-1x \
  --volume-size 1 \
  --initial-cluster-size 1
```

**Note:** Fly.io PostgreSQL is FREE for:
- 1 shared-cpu-1x VM (256MB RAM)
- 1GB storage

### 5. Attach Databases to Apps

**Staging:**
```bash
flyctl postgres attach \
  payment-service-staging-db \
  --app payment-service-staging
```

**Production:**
```bash
flyctl postgres attach \
  payment-service-production-db \
  --app payment-service-production
```

This automatically sets `DATABASE_URL` environment variable.

### 6. Set Environment Secrets

**Staging:**
```bash
flyctl secrets set \
  --app payment-service-staging \
  DB_PASSWORD="$(flyctl postgres list --json | jq -r '.[0].password')" \
  CRON_SECRET="$(openssl rand -hex 32)"
```

**Production:**
```bash
flyctl secrets set \
  --app payment-service-production \
  DB_PASSWORD="$(flyctl postgres list --json | jq -r '.[1].password')" \
  CRON_SECRET="$(openssl rand -hex 32)" \
  EPX_BASE_URL="https://secure.epxnow.com" \
  CALLBACK_BASE_URL="https://payment-service-production.fly.dev"
```

### 7. Configure GitHub Secrets

Go to your GitHub repository: **Settings → Secrets and Variables → Actions**

Click **New repository secret** and add:

**Secret Name:** `FLY_API_TOKEN`
**Secret Value:** Get your token with `flyctl auth token`

### 8. Push to GitHub

The CI/CD pipeline will automatically:
- Run tests
- Build Docker image
- Deploy to staging (on push to `main`)
- Deploy to production (on git tag `v*.*.*`)

```bash
git add .
git commit -m "feat: Add CI/CD deployment configuration"
git push origin main
```

### 9. Create Production Release

When ready for production:

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

This triggers production deployment automatically!

---

## 📖 Detailed Configuration

### Environment Variables

The service uses these environment variables (configured in `fly.toml`):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | gRPC server port |
| `HTTP_PORT` | 8081 | HTTP server port (cron + Browser Post) |
| `DB_HOST` | (from Fly.io) | PostgreSQL host |
| `DB_PORT` | 5432 | PostgreSQL port |
| `DB_USER` | postgres | PostgreSQL user |
| `DB_PASSWORD` | (secret) | PostgreSQL password |
| `DB_NAME` | payment_service | Database name |
| `DB_SSL_MODE` | require | SSL mode for production |
| `EPX_CUST_NBR` | 9001 | EPX customer number |
| `EPX_MERCH_NBR` | 900300 | EPX merchant number |
| `EPX_DBA_NBR` | 2 | EPX DBA number |
| `EPX_TERMINAL_NBR` | 77 | EPX terminal number |
| `EPX_BASE_URL` | https://secure.epxuap.com | EPX API endpoint |
| `CALLBACK_BASE_URL` | (app URL) | Base URL for callbacks |
| `CRON_SECRET` | (secret) | Secret for cron endpoint authentication |

### Health Checks

The service exposes health check endpoints:

- **HTTP Health:** `GET http://localhost:8081/cron/health`
- **Metrics:** `GET http://localhost:9091/metrics` (if enabled)

### Database Migrations

Run migrations manually:

```bash
# Connect to staging database
flyctl proxy 5432 -a payment-service-staging-db

# In another terminal
psql postgresql://postgres:password@localhost:5432/payment_service < migrations/001_initial.sql
```

Or use a migration tool like `goose`:

```bash
goose -dir ./migrations postgres "postgresql://..." up
```

---

## 🔧 Manual Deployment

If you need to deploy manually (bypass CI/CD):

### Deploy to Staging

```bash
flyctl deploy \
  --config fly.toml \
  --app payment-service-staging \
  --strategy rolling
```

### Deploy to Production

```bash
flyctl deploy \
  --config fly.toml \
  --app payment-service-production \
  --strategy rolling
```

---

## 📊 Monitoring & Logs

### View Logs

**Staging:**
```bash
flyctl logs --app payment-service-staging
```

**Production:**
```bash
flyctl logs --app payment-service-production
```

**Follow logs in real-time:**
```bash
flyctl logs --app payment-service-staging -f
```

### Check App Status

```bash
flyctl status --app payment-service-staging
```

### View Metrics

```bash
flyctl metrics --app payment-service-staging
```

### SSH into VM

```bash
flyctl ssh console --app payment-service-staging
```

---

## 🔄 Rollback

### View Releases

```bash
flyctl releases --app payment-service-staging
```

### Rollback to Previous Version

```bash
flyctl releases rollback --app payment-service-staging
```

### Rollback to Specific Version

```bash
flyctl releases rollback v42 --app payment-service-staging
```

---

## 🐛 Troubleshooting

### App Not Starting

1. **Check logs:**
   ```bash
   flyctl logs --app payment-service-staging
   ```

2. **Verify secrets:**
   ```bash
   flyctl secrets list --app payment-service-staging
   ```

3. **Check VM resources:**
   ```bash
   flyctl status --app payment-service-staging
   ```

### Database Connection Issues

1. **Verify DATABASE_URL is set:**
   ```bash
   flyctl secrets list --app payment-service-staging
   ```

2. **Test database connection:**
   ```bash
   flyctl ssh console --app payment-service-staging
   wget -qO- localhost:8081/cron/health
   ```

3. **Check PostgreSQL status:**
   ```bash
   flyctl status --app payment-service-staging-db
   ```

### Health Check Failing

1. **Test health endpoint locally:**
   ```bash
   curl http://payment-service-staging.fly.dev/cron/health
   ```

2. **Check if ports are exposed:**
   - gRPC: port 8080
   - HTTP: port 8081

3. **Verify Dockerfile EXPOSE directives**

### Out of Memory

If you see OOM errors:

1. **Upgrade VM size** (requires paid plan):
   ```bash
   flyctl scale vm shared-cpu-2x --app payment-service-staging
   ```

2. **Optimize Go memory usage:**
   - Add `GOMEMLIMIT` environment variable
   - Profile memory with `pprof`

---

## 💰 Cost Management

### Free Tier Limits

Fly.io FREE tier includes:
- ✅ 3 shared-cpu-1x VMs (256MB RAM each)
- ✅ 3GB persistent volume storage
- ✅ 160GB outbound data transfer/month

### Current Resource Usage

**Staging:**
- 1 VM for app (payment-service-staging)
- 1 VM for PostgreSQL (payment-service-staging-db)

**Production:**
- 1 VM for app (payment-service-production)
- 1 VM for PostgreSQL (payment-service-production-db)

**Total: 4 VMs** (1 over free tier, ~$2-3/month)

### Cost Optimization

1. **Use single PostgreSQL for both environments:**
   ```bash
   # Use staging DB for both, with different schemas
   CREATE SCHEMA staging;
   CREATE SCHEMA production;
   ```

2. **Auto-stop machines when idle:**
   ```toml
   auto_stop_machines = true
   auto_start_machines = true
   ```

3. **Reduce volume size to 1GB minimum**

---

## 🔐 Security Best Practices

1. **Rotate secrets regularly:**
   ```bash
   flyctl secrets set CRON_SECRET="$(openssl rand -hex 32)" --app payment-service-production
   ```

2. **Use different credentials for staging/production:**
   - Different EPX credentials
   - Different database passwords
   - Different CRON_SECRET values

3. **Enable 2FA on Fly.io account**

4. **Review GitHub Actions logs for sensitive data leaks**

5. **Use Fly.io private networking for database connections**

---

## 📚 Additional Resources

- [Fly.io Documentation](https://fly.io/docs/)
- [Fly.io Go Guide](https://fly.io/docs/languages-and-frameworks/golang/)
- [Fly.io PostgreSQL Guide](https://fly.io/docs/postgres/)
- [GitHub Actions Docs](https://docs.github.com/en/actions)
- [Shared Workflows Repository](../deployment-workflows/)

---

## 🆘 Getting Help

**Fly.io Community Forum:** https://community.fly.io/
**Fly.io Status Page:** https://status.flyio.net/

**GitHub Issues:** Report issues in this repository

---

Last Updated: 2025-11-07
