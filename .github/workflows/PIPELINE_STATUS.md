# CI/CD Pipeline Status

## Last Update: 2025-11-26

### Major Architecture Change: PostgreSQL Container

**Root Cause Identified and Fixed:**
The payment-service uses PostgreSQL (`pgx/v5` driver), but the staging environment was configured with Oracle Autonomous Database. This caused the application container to fail because it couldn't connect to Oracle.

**Solution:**
- Removed Oracle Autonomous Database from Terraform
- Added PostgreSQL container to cloud-init (runs alongside the app)
- Simplified deployment workflow to use `goose` for migrations (PostgreSQL-native)
- Reduced required GitHub secrets (no Oracle DB admin password needed)

### Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                  Oracle Cloud Free Tier                        │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              Compute Instance (E2.1.Micro)                │  │
│  │  ┌─────────────────────────────────────────────────────┐  │  │
│  │  │                Docker Network                       │  │  │
│  │  │  ┌─────────────────┐    ┌────────────────────────┐  │  │  │
│  │  │  │  PostgreSQL     │    │  Payment Service       │  │  │  │
│  │  │  │  (postgres:15)  │◄───│  (payment-server)      │  │  │  │
│  │  │  │  Port: 5432     │    │  Ports: 8080, 8081     │  │  │  │
│  │  │  └─────────────────┘    └────────────────────────┘  │  │  │
│  │  └─────────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

### Previous Fixes Applied (Infrastructure)

✅ **Cloud-init POSIX Compatibility Fixed**
- Fixed bashism (here-string `<<<`) in cloud-init.yaml
- Cloud-init runs with `/bin/sh` (dash on Ubuntu), not bash

✅ **Cloud-init User Timing Fixed**
- Fixed "Unknown user: ubuntu" error in write_files
- Added `defer: true` to defer file writes until ubuntu user exists

✅ **Terraform Backend Issue Fixed**
- Removed HTTP backend configuration
- Using local backend with GitHub Actions cache

✅ **SSH Public Key Heredoc Syntax Fixed**
- Changed from double quotes to heredoc (<<-EOT)

### Required GitHub Secrets

The simplified architecture requires fewer secrets:

| Secret Name | Description |
|------------|-------------|
| `OCI_USER_OCID` | Oracle Cloud user OCID |
| `OCI_TENANCY_OCID` | Oracle Cloud tenancy OCID |
| `OCI_COMPARTMENT_OCID` | Compartment OCID |
| `OCI_REGION` | Oracle Cloud region (e.g., us-ashburn-1) |
| `OCI_FINGERPRINT` | API key fingerprint |
| `OCI_PRIVATE_KEY` | API private key (PEM format) |
| `DB_PASSWORD` | PostgreSQL password |
| `EPX_MAC_STAGING` | EPX MAC key for staging |
| `CRON_SECRET_STAGING` | Cron endpoint secret |
| `OCIR_REGION` | Container registry region |
| `OCIR_TENANCY_NAMESPACE` | Container registry namespace |
| `OCIR_USERNAME` | Container registry username |
| `OCIR_AUTH_TOKEN` | Container registry auth token |

**Note:** `ORACLE_DB_ADMIN_PASSWORD` and `ORACLE_DB_PASSWORD` are no longer required.

### Next Steps

1. Clean up existing Oracle Autonomous Database resources
2. Add `DB_PASSWORD` secret to GitHub
3. Trigger new deployment to test PostgreSQL architecture
