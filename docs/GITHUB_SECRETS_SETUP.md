# GitHub Secrets Configuration for CI/CD

This document lists all GitHub secrets required for the complete CI/CD pipeline with automatic staging deployments.

## Required Secrets

Configure these in: **GitHub Repository → Settings → Secrets and variables → Actions**

### Oracle Cloud Infrastructure (Staging Deployment)

| Secret Name | Description | Where to Find |
|------------|-------------|---------------|
| `OCI_USER_OCID` | Oracle Cloud user OCID | Oracle Cloud Console → Profile → User Settings |
| `OCI_TENANCY_OCID` | Oracle Cloud tenancy OCID | Oracle Cloud Console → Tenancy |
| `OCI_COMPARTMENT_OCID` | Compartment OCID for resources | Oracle Cloud Console → Identity → Compartments |
| `OCI_REGION` | Oracle Cloud region (e.g., `us-ashburn-1`) | Your chosen region |
| `OCI_FINGERPRINT` | API key fingerprint | Generated when creating API key |
| `OCI_PRIVATE_KEY` | Private key for API authentication | Downloaded when creating API key (PEM format) |
| `SSH_PUBLIC_KEY` | SSH public key for VM access | `cat ~/.ssh/id_rsa.pub` |

### Oracle Database (Staging)

| Secret Name | Description | Example Value |
|------------|-------------|---------------|
| `ORACLE_DB_ADMIN_PASSWORD` | Admin password for Oracle DB | Strong password (16+ chars) |
| `ORACLE_DB_PASSWORD` | Application user password | Strong password (16+ chars) |
| `ORACLE_DB_USER` | Database username | `payment_service` |

### Oracle Container Image Registry (OCIR)

| Secret Name | Description | Example Value |
|------------|-------------|---------------|
| `OCIR_REGION` | OCIR region code | `iad` (for us-ashburn-1) |
| `OCIR_TENANCY_NAMESPACE` | Tenancy namespace | Found in Oracle Cloud Console |
| `OCIR_USERNAME` | OCIR username | `<tenancy>/<username>` |
| `OCIR_AUTH_TOKEN` | Auth token for OCIR | Generated in Oracle Cloud Console |

### Payment Processor (EPX)

| Secret Name | Description | Example Value |
|------------|-------------|---------------|
| `EPX_MAC_STAGING` | EPX MAC key for staging | Provided by EPX |
| `EPX_CUST_NBR` | EPX customer number | Provided by EPX |
| `EPX_MERCH_NBR` | EPX merchant number | Provided by EPX |
| `EPX_DBA_NBR` | EPX DBA number | Provided by EPX |
| `EPX_TERMINAL_NBR` | EPX terminal number | Provided by EPX |

### Application Secrets

| Secret Name | Description | Example Value |
|------------|-------------|---------------|
| `CRON_SECRET_STAGING` | Secret for cron endpoint auth | Generate: `openssl rand -hex 32` |
| `ORACLE_CLOUD_HOST` | Staging server hostname/IP | Set after infrastructure provisioning |
| `ORACLE_CLOUD_SSH_KEY` | Private SSH key for server access | `cat ~/.ssh/id_rsa` |

### Google Cloud (Production Deployment)

| Secret Name | Description | Example Value |
|------------|-------------|---------------|
| `GCP_PROJECT_ID` | Google Cloud project ID | `payment-service-prod` |
| `GCP_SERVICE_ACCOUNT_KEY` | Service account JSON key | JSON credentials file |
| `GCP_REGION` | GCP region | `us-central1` |

### Integration Testing

| Secret Name | Description | Example Value |
|------------|-------------|---------------|
| `TEST_HOST` | Host to run integration tests against | Set by workflow (auto) |
| `CRON_SECRET` | Cron secret for test environment | Same as `CRON_SECRET_STAGING` |

---

## How to Set Up

### 1. Oracle Cloud Setup

```bash
# Generate API key
oci setup config

# This creates:
# - ~/.oci/config (contains OCID values)
# - ~/.oci/oci_api_key.pem (private key)
# - Fingerprint shown during setup

# Extract values for GitHub secrets
cat ~/.oci/config
cat ~/.oci/oci_api_key.pem
```

### 2. Generate Application Secrets

```bash
# Generate CRON_SECRET_STAGING
openssl rand -hex 32

# Generate SSH key if you don't have one
ssh-keygen -t rsa -b 4096 -C "github-actions@payment-service"
cat ~/.ssh/id_rsa.pub  # For SSH_PUBLIC_KEY
cat ~/.ssh/id_rsa      # For ORACLE_CLOUD_SSH_KEY
```

### 3. Add Secrets to GitHub

```bash
# Option 1: Via GitHub UI
# Go to: Repository → Settings → Secrets and variables → Actions
# Click: "New repository secret"

# Option 2: Via GitHub CLI
gh secret set OCI_USER_OCID < value.txt
gh secret set OCI_TENANCY_OCID < value.txt
gh secret set OCI_PRIVATE_KEY < ~/.oci/oci_api_key.pem
# ... repeat for all secrets
```

---

## Workflow Execution

Once secrets are configured, the CI/CD pipeline will automatically:

### On Push to `develop` Branch:

```
1. ✅ Run Tests
2. ✅ Build Docker Image
3. 🔧 Ensure Staging Infrastructure (Terraform)
4. 🚀 Deploy to Oracle Cloud Staging
5. 🧪 Run Integration Tests (HTTP, gRPC, EPX)
6. 🧹 Cleanup Staging Infrastructure (saves cost)
```

### On Push to `main` Branch:

```
1. ✅ Run Tests
2. ✅ Build Docker Image
3. 🚀 Deploy to GCP Production
4. 🧪 Run Production Integration Tests
```

---

## Verification

After adding all secrets, test the pipeline:

```bash
# Make a trivial change
git commit --allow-empty -m "test: Verify CI/CD pipeline"
git push origin develop

# Monitor at:
# https://github.com/kevin07696/payment-service/actions
```

---

## Security Best Practices

1. ✅ **Use environment-specific secrets** - Different keys for staging/production
2. ✅ **Rotate credentials regularly** - Update secrets every 90 days
3. ✅ **Limit secret access** - Use GitHub environments for production
4. ✅ **Never log secrets** - Secrets are automatically masked in logs
5. ✅ **Use strong passwords** - 16+ characters with mixed case, numbers, symbols

---

## Troubleshooting

### "Secret not found" error
- Verify secret name matches exactly (case-sensitive)
- Check that `secrets: inherit` is in workflow job

### "Unauthorized" errors
- Verify OCI credentials are correct
- Check API key is not expired
- Ensure compartment OCID has proper permissions

### "Infrastructure provisioning failed"
- Check OCI quotas and limits
- Verify compartment has resources available
- Review Terraform state in workflow logs

---

## Cost Management

**Staging infrastructure is ephemeral** - it's created for each deployment and destroyed after tests complete.

**Estimated costs per deployment:**
- Oracle Cloud VM: ~$0.10/hour
- Oracle Database: ~$0.15/hour
- Total per test run: ~$0.05 (20 minutes)

**Production runs continuously** - budget accordingly.
