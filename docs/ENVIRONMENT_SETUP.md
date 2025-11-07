# Environment Configuration Guide

This document explains how to configure environment variables for different deployment environments.

## Environment Files

We provide three environment templates:

- **`.env.example`** - Local development (sandbox credentials)
- **`.env.staging.example`** - Railway staging deployment (sandbox credentials)
- **`.env.production.example`** - Production deployment (production credentials)

## Key Differences Between Environments

### Sandbox vs Production URLs

| Service | Sandbox URL | Production URL |
|---------|-------------|----------------|
| EPX Server Post | `https://secure.epxuap.com` | `https://secure.epxnow.com` |
| EPX Browser Post | `https://secure.epxuap.com/browserpost` | `https://secure.epxnow.com/browserpost` |
| EPX Key Exchange | `https://keyexch.epxuap.com` | N/A (production uses different flow) |
| North Merchant Reporting | `https://api.north.com` | `https://api.north.com` |

**Important Notes:**
- Sandbox and Production use **different EPX credentials**
- North Merchant Reporting API uses the **same URL** for both environments
- Always use sandbox credentials for testing/staging
- Never use production credentials in development/staging

---

## Local Development Setup

### 1. Copy Environment File

```bash
cp .env.example .env
```

### 2. Update Sandbox Credentials

Get your sandbox credentials from North/EPX support and update:

```bash
EPX_CUST_NBR=your_sandbox_cust_nbr
EPX_MERCH_NBR=your_sandbox_merch_nbr
EPX_DBA_NBR=your_sandbox_dba_nbr
EPX_TERMINAL_NBR=your_sandbox_terminal_nbr
```

### 3. Set Callback URL

For local development:

```bash
CALLBACK_BASE_URL=http://localhost:8081
```

### 4. Start Local Database

```bash
docker-compose up postgres -d
```

### 5. Run Migrations

```bash
goose -dir migrations postgres "postgresql://localhost:5432/payment_service?sslmode=disable" up
```

### 6. Start Service

```bash
go run cmd/server/main.go
```

---

## Railway Staging Setup

### 1. Create Railway Project

See main [DEPLOYMENT.md](./DEPLOYMENT.md) for Railway setup.

### 2. Add PostgreSQL Database

Railway automatically provides `DATABASE_URL` when you add PostgreSQL.

### 3. Configure Environment Variables

In Railway service → **Variables** tab, add from `.env.staging.example`:

```bash
PORT=8080
HTTP_PORT=8081
ENVIRONMENT=staging
DB_SSL_MODE=require

# Sandbox EPX Configuration
EPX_BASE_URL=https://secure.epxuap.com
EPX_TIMEOUT=30
EPX_CUST_NBR=your_sandbox_cust_nbr
EPX_MERCH_NBR=your_sandbox_merch_nbr
EPX_DBA_NBR=your_sandbox_dba_nbr
EPX_TERMINAL_NBR=your_sandbox_terminal_nbr

# North API
NORTH_API_URL=https://api.north.com
NORTH_TIMEOUT=30

# Callback URL (update after deployment)
CALLBACK_BASE_URL=https://your-app-name.up.railway.app

# Security
CRON_SECRET=your-staging-secret-here
LOG_LEVEL=debug
```

### 4. Update Callback URL

After first deployment:
1. Note your Railway URL: `https://payment-service-staging-xxxxx.up.railway.app`
2. Update `CALLBACK_BASE_URL` variable in Railway

---

## Production Setup

### 1. Get Production Credentials

Contact North/EPX support for:
- Production EPX customer number
- Production EPX merchant number
- Production EPX DBA number
- Production EPX terminal number

⚠️ **CRITICAL:** Never use sandbox credentials in production!

### 2. Configure Production Environment

Use `.env.production.example` as reference:

```bash
PORT=8080
HTTP_PORT=8081
ENVIRONMENT=production
DB_SSL_MODE=require

# Production EPX Configuration
EPX_BASE_URL=https://secure.epxnow.com
EPX_TIMEOUT=30
EPX_CUST_NBR=YOUR_PROD_CUST_NBR
EPX_MERCH_NBR=YOUR_PROD_MERCH_NBR
EPX_DBA_NBR=YOUR_PROD_DBA_NBR
EPX_TERMINAL_NBR=YOUR_PROD_TERMINAL_NBR

# North API
NORTH_API_URL=https://api.north.com
NORTH_TIMEOUT=30

# Production Domain
CALLBACK_BASE_URL=https://payments.yourdomain.com

# Security
CRON_SECRET=use-strong-random-secret
LOG_LEVEL=info
```

### 3. Generate Secure Secrets

```bash
# Generate CRON_SECRET
openssl rand -base64 32
```

---

## Environment Variable Reference

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `PORT` | gRPC server port | `8080` |
| `HTTP_PORT` | HTTP server port | `8081` |
| `DATABASE_URL` | PostgreSQL connection string | Auto-set by Railway |
| `EPX_BASE_URL` | EPX gateway URL | `https://secure.epxuap.com` |
| `EPX_CUST_NBR` | EPX customer number | `9001` |
| `EPX_MERCH_NBR` | EPX merchant number | `900300` |
| `EPX_DBA_NBR` | EPX DBA number | `2` |
| `EPX_TERMINAL_NBR` | EPX terminal number | `77` |
| `CALLBACK_BASE_URL` | Base URL for callbacks | `https://your-app.railway.app` |

### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_SSL_MODE` | Database SSL mode | `disable` (local), `require` (cloud) |
| `ENVIRONMENT` | Environment name | `development` |
| `LOG_LEVEL` | Logging verbosity | `info` |
| `CRON_SECRET` | Cron endpoint auth | `change-me-in-production` |
| `EPX_TIMEOUT` | EPX request timeout (seconds) | `30` |
| `NORTH_TIMEOUT` | North API timeout (seconds) | `30` |

---

## Security Best Practices

### 1. Never Commit Secrets

The `.env` file is in `.gitignore`. Never commit:
- Actual EPX credentials
- Database passwords
- CRON_SECRET values
- Production configuration

### 2. Separate Credentials by Environment

| Environment | EPX URL | Credentials |
|-------------|---------|-------------|
| Development | Sandbox | Sandbox credentials |
| Staging | Sandbox | Sandbox credentials |
| Production | Production | Production credentials |

### 3. Rotate Secrets Regularly

- Change `CRON_SECRET` periodically
- Rotate database passwords
- Update EPX credentials if compromised

### 4. Use Strong Secrets

```bash
# Generate strong random secrets
openssl rand -base64 32

# Or use password manager
```

---

## Troubleshooting

### Invalid EPX Credentials

**Error:** `401 Unauthorized` from EPX

**Solution:**
- Verify you're using correct credentials for the environment
- Staging should use sandbox credentials
- Production should use production credentials
- Contact North/EPX support if credentials are incorrect

### Database Connection Failed

**Error:** `Failed to connect to database`

**Solution:**
- Check `DATABASE_URL` is set correctly
- For Railway, ensure PostgreSQL service is attached
- Verify `DB_SSL_MODE=require` for cloud databases
- Check database is running (`docker-compose ps` for local)

### Callback URL Not Working

**Error:** Browser Post callbacks failing

**Solution:**
- Verify `CALLBACK_BASE_URL` matches your deployed URL
- Check Railway service is publicly accessible
- Ensure `/api/v1/payments/browser-post/callback` endpoint is accessible
- Test with: `curl https://your-app.railway.app/cron/health`

---

## Next Steps

- [Main Deployment Guide](./DEPLOYMENT.md)
- [Migration Guide](../migrations/README.md)
- [API Documentation](../README.md)
