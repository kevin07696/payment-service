# Quick Secrets Setup

## Option 1: Automated Script (Recommended)

```bash
./scripts/configure-github-secrets.sh
```

The script will:
- Auto-detect values from `~/.oci/config`
- Generate strong passwords
- Use EPX sandbox credentials
- Configure all 18 secrets automatically

---

## Option 2: Manual Configuration

### Prerequisites

```bash
# Install GitHub CLI if needed
brew install gh  # macOS
# or see https://cli.github.com

# Login
gh auth login
```

### Quick Commands

```bash
REPO="kevin07696/payment-service"

# 1. Oracle Cloud (auto-detect from ~/.oci/config)
gh secret set OCI_USER_OCID --repo $REPO --body "$(grep 'user=' ~/.oci/config | cut -d'=' -f2)"
gh secret set OCI_TENANCY_OCID --repo $REPO --body "$(grep 'tenancy=' ~/.oci/config | cut -d'=' -f2)"
gh secret set OCI_FINGERPRINT --repo $REPO --body "$(grep 'fingerprint=' ~/.oci/config | cut -d'=' -f2)"
gh secret set OCI_REGION --repo $REPO --body "$(grep 'region=' ~/.oci/config | cut -d'=' -f2)"
gh secret set OCI_PRIVATE_KEY --repo $REPO --body "$(cat ~/.oci/oci_api_key.pem)"

# Get compartment OCID
gh secret set OCI_COMPARTMENT_OCID --repo $REPO --body "$(oci iam compartment list --query 'data[0].id' --raw-output)"

# 2. OCIR (auto-detect)
gh secret set OCIR_REGION --repo $REPO --body "iad"  # or your region code
gh secret set OCIR_TENANCY_NAMESPACE --repo $REPO --body "$(oci os ns get --query 'data' --raw-output)"
gh secret set OCIR_USERNAME --repo $REPO --body "$(oci os ns get --query 'data' --raw-output)/$(oci iam user get --user-id $(grep 'user=' ~/.oci/config | cut -d'=' -f2) --query 'data.name' --raw-output)"

# OCIR Auth Token (create manually in Oracle Cloud Console)
gh secret set OCIR_AUTH_TOKEN --repo $REPO --body "YOUR_AUTH_TOKEN_HERE"

# 3. Database
gh secret set ORACLE_DB_PASSWORD --repo $REPO --body "$(openssl rand -base64 24)"

# 4. EPX Test Credentials (EPX sandbox)
gh secret set EPX_MAC_STAGING --repo $REPO --body "2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"
gh secret set EPX_CUST_NBR --repo $REPO --body "9001"
gh secret set EPX_MERCH_NBR --repo $REPO --body "900300"
gh secret set EPX_DBA_NBR --repo $REPO --body "2"
gh secret set EPX_TERMINAL_NBR --repo $REPO --body "77"

# 5. Application
gh secret set CRON_SECRET_STAGING --repo $REPO --body "$(openssl rand -hex 32)"

# 6. SSH Keys
gh secret set SSH_PUBLIC_KEY --repo $REPO --body "$(cat ~/.ssh/id_rsa.pub)"
gh secret set ORACLE_CLOUD_SSH_KEY --repo $REPO --body "$(cat ~/.ssh/id_rsa)"
```

---

## Verify Secrets

```bash
gh secret list --repo kevin07696/payment-service
```

Expected output (18 secrets):

```
CRON_SECRET_STAGING          Updated YYYY-MM-DD
EPX_CUST_NBR                 Updated YYYY-MM-DD
EPX_DBA_NBR                  Updated YYYY-MM-DD
EPX_MAC_STAGING              Updated YYYY-MM-DD
EPX_MERCH_NBR                Updated YYYY-MM-DD
EPX_TERMINAL_NBR             Updated YYYY-MM-DD
OCI_COMPARTMENT_OCID         Updated YYYY-MM-DD
OCI_FINGERPRINT              Updated YYYY-MM-DD
OCI_PRIVATE_KEY              Updated YYYY-MM-DD
OCI_REGION                   Updated YYYY-MM-DD
OCI_TENANCY_OCID             Updated YYYY-MM-DD
OCI_USER_OCID                Updated YYYY-MM-DD
OCIR_AUTH_TOKEN              Updated YYYY-MM-DD
OCIR_REGION                  Updated YYYY-MM-DD
OCIR_TENANCY_NAMESPACE       Updated YYYY-MM-DD
OCIR_USERNAME                Updated YYYY-MM-DD
ORACLE_CLOUD_SSH_KEY         Updated YYYY-MM-DD
ORACLE_DB_PASSWORD           Updated YYYY-MM-DD
SSH_PUBLIC_KEY               Updated YYYY-MM-DD
```

---

## Common Issues

### "OCIR_AUTH_TOKEN required"

Create auth token in Oracle Cloud Console:
1. Profile → Auth Tokens → Generate Token
2. Copy token (only shown once)
3. Set: `gh secret set OCIR_AUTH_TOKEN --repo $REPO --body "YOUR_TOKEN"`

### "No SSH key found"

Generate new SSH key:
```bash
ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N ""
```

### "OCI CLI not found"

Values must be entered manually or use the automated script.

---

## Next Steps

After configuring secrets:

1. **Uncomment deployment stages** in `.github/workflows/ci-cd.yml`
2. **Commit and push** to `develop` branch
3. **Watch GitHub Actions** run full CI/CD pipeline
4. **Integration tests** will run as deployment gate

See `docs/INTEGRATION_TESTS_SUMMARY.md` for complete workflow.
