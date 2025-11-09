#!/bin/bash
# Configure all GitHub secrets for payment-service CI/CD
# Run this script to set up all required secrets at once

set -e

REPO="kevin07696/payment-service"

echo "🔐 Configuring GitHub Secrets for $REPO"
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) is not installed"
    echo "Install it with: brew install gh (macOS) or see https://cli.github.com"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "❌ Not authenticated with GitHub CLI"
    echo "Run: gh auth login"
    exit 1
fi

echo "✅ GitHub CLI is ready"
echo ""

# ============================================================================
# 1. Oracle Cloud Infrastructure (6 secrets)
# ============================================================================
echo "📦 Configuring OCI secrets..."

# Auto-detect from ~/.oci/config
if [ -f "$HOME/.oci/config" ]; then
    OCI_USER_OCID=$(grep -A 5 "\[DEFAULT\]" ~/.oci/config | grep "user=" | cut -d'=' -f2)
    OCI_TENANCY_OCID=$(grep -A 5 "\[DEFAULT\]" ~/.oci/config | grep "tenancy=" | cut -d'=' -f2)
    OCI_FINGERPRINT=$(grep -A 5 "\[DEFAULT\]" ~/.oci/config | grep "fingerprint=" | cut -d'=' -f2)
    OCI_REGION=$(grep -A 5 "\[DEFAULT\]" ~/.oci/config | grep "region=" | cut -d'=' -f2)

    echo "  Auto-detected from ~/.oci/config:"
    echo "    User OCID: ${OCI_USER_OCID:0:20}..."
    echo "    Tenancy OCID: ${OCI_TENANCY_OCID:0:20}..."
    echo "    Fingerprint: $OCI_FINGERPRINT"
    echo "    Region: $OCI_REGION"
else
    echo "  ⚠️  ~/.oci/config not found - you'll need to enter values manually"
fi

# Get compartment OCID
echo ""
read -p "Enter OCI_COMPARTMENT_OCID (or press Enter to detect): " OCI_COMPARTMENT_OCID
if [ -z "$OCI_COMPARTMENT_OCID" ]; then
    echo "  Detecting compartment OCID..."
    OCI_COMPARTMENT_OCID=$(oci iam compartment list --compartment-id-in-subtree true --query 'data[0].id' --raw-output 2>/dev/null || echo "")
    if [ -n "$OCI_COMPARTMENT_OCID" ]; then
        echo "  ✅ Auto-detected: ${OCI_COMPARTMENT_OCID:0:20}..."
    fi
fi

# Read private key
if [ -f "$HOME/.oci/oci_api_key.pem" ]; then
    OCI_PRIVATE_KEY=$(cat "$HOME/.oci/oci_api_key.pem")
    echo "  ✅ Read private key from ~/.oci/oci_api_key.pem"
else
    echo "  ⚠️  Private key not found at ~/.oci/oci_api_key.pem"
    read -p "Enter path to OCI private key: " KEY_PATH
    OCI_PRIVATE_KEY=$(cat "$KEY_PATH")
fi

# Set OCI secrets
gh secret set OCI_USER_OCID --repo $REPO --body "$OCI_USER_OCID"
gh secret set OCI_TENANCY_OCID --repo $REPO --body "$OCI_TENANCY_OCID"
gh secret set OCI_COMPARTMENT_OCID --repo $REPO --body "$OCI_COMPARTMENT_OCID"
gh secret set OCI_REGION --repo $REPO --body "$OCI_REGION"
gh secret set OCI_FINGERPRINT --repo $REPO --body "$OCI_FINGERPRINT"
gh secret set OCI_PRIVATE_KEY --repo $REPO --body "$OCI_PRIVATE_KEY"

echo "  ✅ OCI secrets configured (6/13)"
echo ""

# ============================================================================
# 2. Oracle Container Image Registry (3 secrets)
# ============================================================================
echo "🐳 Configuring OCIR secrets..."

# Auto-detect OCIR region from OCI region
case "$OCI_REGION" in
    us-ashburn-1) OCIR_REGION="iad" ;;
    us-phoenix-1) OCIR_REGION="phx" ;;
    *) OCIR_REGION="iad" ;;
esac
echo "  Auto-detected OCIR_REGION: $OCIR_REGION"

# Get tenancy namespace
OCIR_TENANCY_NAMESPACE=$(oci os ns get --query 'data' --raw-output 2>/dev/null || echo "")
if [ -n "$OCIR_TENANCY_NAMESPACE" ]; then
    echo "  ✅ Auto-detected OCIR_TENANCY_NAMESPACE: $OCIR_TENANCY_NAMESPACE"
else
    read -p "Enter OCIR_TENANCY_NAMESPACE: " OCIR_TENANCY_NAMESPACE
fi

# Get username
OCI_USERNAME=$(oci iam user get --user-id "$OCI_USER_OCID" --query 'data.name' --raw-output 2>/dev/null || echo "")
if [ -n "$OCI_USERNAME" ]; then
    OCIR_USERNAME="${OCIR_TENANCY_NAMESPACE}/${OCI_USERNAME}"
    echo "  ✅ Auto-detected OCIR_USERNAME: $OCIR_USERNAME"
else
    read -p "Enter OCIR_USERNAME (format: tenancy/username): " OCIR_USERNAME
fi

# Auth token needs to be created manually
echo "  ⚠️  OCIR_AUTH_TOKEN must be created manually:"
echo "      1. Go to Oracle Cloud Console → Profile → Auth Tokens"
echo "      2. Click 'Generate Token'"
echo "      3. Copy the token (only shown once)"
read -p "Enter OCIR_AUTH_TOKEN: " OCIR_AUTH_TOKEN

# Set OCIR secrets
gh secret set OCIR_REGION --repo $REPO --body "$OCIR_REGION"
gh secret set OCIR_TENANCY_NAMESPACE --repo $REPO --body "$OCIR_TENANCY_NAMESPACE"
gh secret set OCIR_USERNAME --repo $REPO --body "$OCIR_USERNAME"
gh secret set OCIR_AUTH_TOKEN --repo $REPO --body "$OCIR_AUTH_TOKEN"

echo "  ✅ OCIR secrets configured (9/13)"
echo ""

# ============================================================================
# 3. Database Passwords (1 secret - admin password in deployment-workflows)
# ============================================================================
echo "🗄️  Configuring database secrets..."

echo "  Generate strong password for ORACLE_DB_PASSWORD:"
DB_PASSWORD=$(openssl rand -base64 24)
echo "  Generated: ${DB_PASSWORD:0:10}... (24 characters)"
read -p "Press Enter to use generated password, or type your own: " CUSTOM_DB_PASSWORD
if [ -n "$CUSTOM_DB_PASSWORD" ]; then
    DB_PASSWORD="$CUSTOM_DB_PASSWORD"
fi

gh secret set ORACLE_DB_PASSWORD --repo $REPO --body "$DB_PASSWORD"

echo "  ✅ Database secrets configured (10/13)"
echo ""

# ============================================================================
# 4. EPX Test Credentials (5 secrets)
# ============================================================================
echo "💳 Configuring EPX test credentials..."

EPX_MAC_STAGING="2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"
EPX_CUST_NBR="9001"
EPX_MERCH_NBR="900300"
EPX_DBA_NBR="2"
EPX_TERMINAL_NBR="77"

echo "  Using EPX sandbox credentials (public test credentials):"
echo "    MAC: ${EPX_MAC_STAGING:0:20}..."
echo "    CUST_NBR: $EPX_CUST_NBR"
echo "    MERCH_NBR: $EPX_MERCH_NBR"
echo "    DBA_NBR: $EPX_DBA_NBR"
echo "    TERMINAL_NBR: $EPX_TERMINAL_NBR"

gh secret set EPX_MAC_STAGING --repo $REPO --body "$EPX_MAC_STAGING"
gh secret set EPX_CUST_NBR --repo $REPO --body "$EPX_CUST_NBR"
gh secret set EPX_MERCH_NBR --repo $REPO --body "$EPX_MERCH_NBR"
gh secret set EPX_DBA_NBR --repo $REPO --body "$EPX_DBA_NBR"
gh secret set EPX_TERMINAL_NBR --repo $REPO --body "$EPX_TERMINAL_NBR"

echo "  ✅ EPX secrets configured (15/16 - note: total is 16 now, not 13)"
echo ""

# ============================================================================
# 5. Application Secrets (2 secrets)
# ============================================================================
echo "🔑 Configuring application secrets..."

# Generate CRON_SECRET
CRON_SECRET=$(openssl rand -hex 32)
echo "  Generated CRON_SECRET_STAGING: ${CRON_SECRET:0:20}..."

gh secret set CRON_SECRET_STAGING --repo $REPO --body "$CRON_SECRET"

echo "  ✅ Application secrets configured (16/16)"
echo ""

# ============================================================================
# 6. SSH Keys (2 secrets)
# ============================================================================
echo "🔐 Configuring SSH keys..."

# Check for existing SSH keys
if [ -f "$HOME/.ssh/id_rsa.pub" ]; then
    SSH_PUBLIC_KEY=$(cat "$HOME/.ssh/id_rsa.pub")
    echo "  ✅ Found SSH public key at ~/.ssh/id_rsa.pub"
else
    echo "  ⚠️  No SSH key found at ~/.ssh/id_rsa.pub"
    read -p "Generate new SSH key? (y/n): " GENERATE_SSH
    if [ "$GENERATE_SSH" = "y" ]; then
        ssh-keygen -t rsa -b 4096 -f "$HOME/.ssh/id_rsa" -N ""
        SSH_PUBLIC_KEY=$(cat "$HOME/.ssh/id_rsa.pub")
        echo "  ✅ Generated new SSH key"
    else
        read -p "Enter path to SSH public key: " SSH_KEY_PATH
        SSH_PUBLIC_KEY=$(cat "$SSH_KEY_PATH")
    fi
fi

if [ -f "$HOME/.ssh/id_rsa" ]; then
    ORACLE_CLOUD_SSH_KEY=$(cat "$HOME/.ssh/id_rsa")
    echo "  ✅ Found SSH private key at ~/.ssh/id_rsa"
else
    read -p "Enter path to SSH private key: " SSH_PRIVATE_KEY_PATH
    ORACLE_CLOUD_SSH_KEY=$(cat "$SSH_PRIVATE_KEY_PATH")
fi

gh secret set SSH_PUBLIC_KEY --repo $REPO --body "$SSH_PUBLIC_KEY"
gh secret set ORACLE_CLOUD_SSH_KEY --repo $REPO --body "$ORACLE_CLOUD_SSH_KEY"

echo "  ✅ SSH keys configured (18/18)"
echo ""

# ============================================================================
# Summary
# ============================================================================
echo "✅ All secrets configured!"
echo ""
echo "📊 Summary:"
gh secret list --repo $REPO

echo ""
echo "🚀 Next steps:"
echo "  1. Uncomment deployment stages in .github/workflows/ci-cd.yml"
echo "  2. Commit and push to 'develop' branch"
echo "  3. Watch GitHub Actions run the full CI/CD pipeline"
echo ""
echo "📚 Documentation:"
echo "  - docs/TESTING_STRATEGY.md - Testing architecture"
echo "  - docs/INTEGRATION_TESTS_SUMMARY.md - Quick reference"
echo "  - docs/GITHUB_SECRETS_SETUP.md - Secret details"
