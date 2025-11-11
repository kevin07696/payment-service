# GitHub Secrets Setup

## Quick Setup

```bash
./scripts/configure-github-secrets.sh
```

Auto-configures all 18 secrets from `~/.oci/config` and generates passwords.

## Required Secrets (18 total)

Configure in: **GitHub Repository → Settings → Secrets and variables → Actions**

### Oracle Cloud Infrastructure (6)

| Secret | Where to Get |
|--------|--------------|
| `OCI_USER_OCID` | `~/.oci/config` → `user=` |
| `OCI_TENANCY_OCID` | `~/.oci/config` → `tenancy=` |
| `OCI_COMPARTMENT_OCID` | `oci iam compartment list --query 'data[0].id' --raw-output` |
| `OCI_REGION` | `~/.oci/config` → `region=` |
| `OCI_FINGERPRINT` | `~/.oci/config` → `fingerprint=` |
| `OCI_PRIVATE_KEY` | `cat ~/.oci/oci_api_key.pem` |

### Container Registry (4)

| Secret | Where to Get |
|--------|--------------|
| `OCIR_REGION` | Region code (e.g., `iad` for us-ashburn-1) |
| `OCIR_TENANCY_NAMESPACE` | `oci os ns get --query 'data' --raw-output` |
| `OCIR_USERNAME` | `<tenancy>/<username>` format |
| `OCIR_AUTH_TOKEN` | Oracle Cloud Console → User Settings → Auth Tokens → Generate |

### Database (1)

| Secret | Where to Get |
|--------|--------------|
| `ORACLE_DB_PASSWORD` | `openssl rand -base64 32` (generate strong password) |

### EPX Test Credentials (5)

| Secret | Value |
|--------|-------|
| `EPX_MAC_STAGING` | `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` |
| `EPX_CUST_NBR` | `9001` |
| `EPX_MERCH_NBR` | `900300` |
| `EPX_DBA_NBR` | `2` |
| `EPX_TERMINAL_NBR` | `77` |

Note: These are public EPX sandbox credentials, safe to commit.

### Application (2)

| Secret | Where to Get |
|--------|--------------|
| `CRON_SECRET_STAGING` | `openssl rand -base64 32` |
| `SSH_PUBLIC_KEY` | `cat ~/.ssh/id_rsa.pub` (or `ssh-keygen -t rsa -b 4096`) |

## Manual Setup

### Prerequisites

```bash
# Install GitHub CLI
brew install gh  # macOS
# or see https://cli.github.com

# Login
gh auth login
```

### Set Secrets

```bash
REPO="kevin07696/payment-service"

# Oracle Cloud (auto-detect from ~/.oci/config)
gh secret set OCI_USER_OCID --repo $REPO \
  --body "$(grep 'user=' ~/.oci/config | cut -d'=' -f2)"

gh secret set OCI_TENANCY_OCID --repo $REPO \
  --body "$(grep 'tenancy=' ~/.oci/config | cut -d'=' -f2)"

gh secret set OCI_FINGERPRINT --repo $REPO \
  --body "$(grep 'fingerprint=' ~/.oci/config | cut -d'=' -f2)"

gh secret set OCI_REGION --repo $REPO \
  --body "$(grep 'region=' ~/.oci/config | cut -d'=' -f2)"

gh secret set OCI_PRIVATE_KEY --repo $REPO \
  --body "$(cat ~/.oci/oci_api_key.pem)"

gh secret set OCI_COMPARTMENT_OCID --repo $REPO \
  --body "$(oci iam compartment list --query 'data[0].id' --raw-output)"

# Container Registry
gh secret set OCIR_REGION --repo $REPO --body "iad"

gh secret set OCIR_TENANCY_NAMESPACE --repo $REPO \
  --body "$(oci os ns get --query 'data' --raw-output)"

gh secret set OCIR_USERNAME --repo $REPO \
  --body "$(oci os ns get --query 'data' --raw-output)/$(oci iam user get --user-id $(grep 'user=' ~/.oci/config | cut -d'=' -f2) --query 'data.name' --raw-output)"

# OCIR Auth Token - generate manually:
# Oracle Cloud Console → User Settings → Auth Tokens → Generate Token
gh secret set OCIR_AUTH_TOKEN --repo $REPO  # paste token when prompted

# Database
gh secret set ORACLE_DB_PASSWORD --repo $REPO \
  --body "$(openssl rand -base64 32)"

# EPX Sandbox
gh secret set EPX_MAC_STAGING --repo $REPO \
  --body "2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"

gh secret set EPX_CUST_NBR --repo $REPO --body "9001"
gh secret set EPX_MERCH_NBR --repo $REPO --body "900300"
gh secret set EPX_DBA_NBR --repo $REPO --body "2"
gh secret set EPX_TERMINAL_NBR --repo $REPO --body "77"

# Application
gh secret set CRON_SECRET_STAGING --repo $REPO \
  --body "$(openssl rand -base64 32)"

gh secret set SSH_PUBLIC_KEY --repo $REPO \
  --body "$(cat ~/.ssh/id_rsa.pub)"
```

## Secret Architecture

See `docs/ARCHITECTURE_SECRETS.md` for detailed explanation of:
- Separation between infrastructure and service secrets
- How secrets flow from GitHub → Terraform → OCI Vault
- Runtime secret access via OCI SDK
- IAM permissions required

## Verification

```bash
# List all configured secrets
gh secret list --repo kevin07696/payment-service

# Check specific secret exists (won't show value)
gh api repos/kevin07696/payment-service/actions/secrets/OCI_USER_OCID
```

## Troubleshooting

### "Secret not found" in workflow

```bash
# Verify secret name matches exactly (case-sensitive)
gh secret list --repo kevin07696/payment-service | grep OCI

# Re-set the secret
gh secret set SECRET_NAME --repo kevin07696/payment-service
```

### OCI CLI commands fail

```bash
# Test OCI authentication
oci iam region list

# Verify ~/.oci/config exists
cat ~/.oci/config

# Check private key permissions
chmod 600 ~/.oci/oci_api_key.pem
```

### OCIR authentication fails

```bash
# Test OCIR login
docker login <region>.ocir.io \
  -u '<tenancy>/<username>' \
  -p '<auth-token>'

# Verify auth token is valid (generate new if expired)
# Oracle Cloud Console → User Settings → Auth Tokens
```

## References

- Secret architecture: `docs/ARCHITECTURE_SECRETS.md`
- CI/CD workflow: `.github/workflows/ci-cd.yml`
- Configuration script: `scripts/configure-github-secrets.sh`
- Oracle Cloud setup: `docs/ORACLE_FREE_TIER_STAGING.md`
