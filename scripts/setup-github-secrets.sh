#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}GitHub Secrets Setup for CI/CD Pipeline${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Check if gh CLI is installed and authenticated
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Error: GitHub CLI (gh) is not installed${NC}"
    echo "Install it: https://cli.github.com/"
    exit 1
fi

if ! gh auth status &> /dev/null; then
    echo -e "${RED}Error: Not authenticated with GitHub CLI${NC}"
    echo "Run: gh auth login"
    exit 1
fi

echo -e "${GREEN}✓ GitHub CLI authenticated${NC}\n"

# Get repository info
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
echo -e "Repository: ${BLUE}$REPO${NC}\n"

# Read existing OCI config
OCI_CONFIG="$HOME/.oci/config"
OCI_PRIVATE_KEY="$HOME/.oci/oci_api_key.pem"

if [ ! -f "$OCI_CONFIG" ]; then
    echo -e "${RED}Error: OCI config not found at $OCI_CONFIG${NC}"
    exit 1
fi

if [ ! -f "$OCI_PRIVATE_KEY" ]; then
    echo -e "${RED}Error: OCI private key not found at $OCI_PRIVATE_KEY${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Found existing OCI credentials${NC}\n"

# Extract values from OCI config
OCI_USER_OCID=$(grep "^user=" "$OCI_CONFIG" | cut -d'=' -f2)
OCI_FINGERPRINT=$(grep "^fingerprint=" "$OCI_CONFIG" | cut -d'=' -f2)
OCI_TENANCY_OCID=$(grep "^tenancy=" "$OCI_CONFIG" | cut -d'=' -f2)
OCI_REGION=$(grep "^region=" "$OCI_CONFIG" | cut -d'=' -f2)
OCI_PRIVATE_KEY_CONTENT=$(cat "$OCI_PRIVATE_KEY")

echo -e "${BLUE}Setting Oracle Cloud Infrastructure secrets...${NC}"
echo "$OCI_USER_OCID" | gh secret set OCI_USER_OCID
echo -e "${GREEN}✓ OCI_USER_OCID${NC}"

echo "$OCI_FINGERPRINT" | gh secret set OCI_FINGERPRINT
echo -e "${GREEN}✓ OCI_FINGERPRINT${NC}"

echo "$OCI_TENANCY_OCID" | gh secret set OCI_TENANCY_OCID
echo -e "${GREEN}✓ OCI_TENANCY_OCID${NC}"

echo "$OCI_REGION" | gh secret set OCI_REGION
echo -e "${GREEN}✓ OCI_REGION${NC}"

echo "$OCI_PRIVATE_KEY_CONTENT" | gh secret set OCI_PRIVATE_KEY
echo -e "${GREEN}✓ OCI_PRIVATE_KEY${NC}\n"

# Prompt for OCI Compartment OCID
echo -e "${YELLOW}=== Oracle Cloud Compartment ===${NC}"
echo "You can find your compartment OCID at:"
echo "1. Go to: https://cloud.oracle.com"
echo "2. Navigate to: Identity & Security → Compartments"
echo "3. Select your compartment → Copy OCID"
echo ""
read -p "Enter OCI_COMPARTMENT_OCID: " OCI_COMPARTMENT_OCID
echo "$OCI_COMPARTMENT_OCID" | gh secret set OCI_COMPARTMENT_OCID
echo -e "${GREEN}✓ OCI_COMPARTMENT_OCID${NC}\n"

# Oracle Database secrets
echo -e "${YELLOW}=== Oracle Autonomous Database ===${NC}"
echo "Enter your Oracle Autonomous Database credentials"
echo ""

read -s -p "Enter ORACLE_DB_ADMIN_PASSWORD: " ORACLE_DB_ADMIN_PASSWORD
echo ""
echo "$ORACLE_DB_ADMIN_PASSWORD" | gh secret set ORACLE_DB_ADMIN_PASSWORD
echo -e "${GREEN}✓ ORACLE_DB_ADMIN_PASSWORD${NC}"

echo ""
echo "Enter the connection string from your wallet's tnsnames.ora"
echo "Example: payment_staging_high = (description= (retry_count=20)...)"
read -p "Enter ORACLE_DB_CONNECTION_STRING: " ORACLE_DB_CONNECTION_STRING
echo "$ORACLE_DB_CONNECTION_STRING" | gh secret set ORACLE_DB_CONNECTION_STRING
echo -e "${GREEN}✓ ORACLE_DB_CONNECTION_STRING${NC}"

read -s -p "Enter ORACLE_DB_WALLET_PASSWORD: " ORACLE_DB_WALLET_PASSWORD
echo ""
echo "$ORACLE_DB_WALLET_PASSWORD" | gh secret set ORACLE_DB_WALLET_PASSWORD
echo -e "${GREEN}✓ ORACLE_DB_WALLET_PASSWORD${NC}\n"

# Oracle Container Registry
echo -e "${YELLOW}=== Oracle Container Registry ===${NC}"

# OCIR region (3-letter code)
OCIR_REGION_CODE="${OCI_REGION:0:3}"
if [ "$OCI_REGION" = "us-ashburn-1" ]; then
    OCIR_REGION_CODE="iad"
elif [ "$OCI_REGION" = "us-phoenix-1" ]; then
    OCIR_REGION_CODE="phx"
fi

echo "$OCIR_REGION_CODE" | gh secret set OCIR_REGION
echo -e "${GREEN}✓ OCIR_REGION (auto-detected: $OCIR_REGION_CODE)${NC}"

echo "Find your namespace at: Tenancy Details → Object Storage Namespace"
read -p "Enter OCIR_NAMESPACE: " OCIR_NAMESPACE
echo "$OCIR_NAMESPACE" | gh secret set OCIR_NAMESPACE
echo -e "${GREEN}✓ OCIR_NAMESPACE${NC}"

echo "Format: <namespace>/<username> (e.g., $OCIR_NAMESPACE/oracleidentitycloudservice/your.email@example.com)"
read -p "Enter OCIR_USERNAME: " OCIR_USERNAME
echo "$OCIR_USERNAME" | gh secret set OCIR_USERNAME
echo -e "${GREEN}✓ OCIR_USERNAME${NC}"

echo "Generate at: User Settings → Auth Tokens → Generate Token"
read -s -p "Enter OCIR_AUTH_TOKEN: " OCIR_AUTH_TOKEN
echo ""
echo "$OCIR_AUTH_TOKEN" | gh secret set OCIR_AUTH_TOKEN
echo -e "${GREEN}✓ OCIR_AUTH_TOKEN${NC}\n"

# EPX Payment Processor
echo -e "${YELLOW}=== EPX Payment Processor ===${NC}"
echo "Contact your EPX account representative for these values"
echo ""

read -p "Enter EPX_MAC_STAGING: " EPX_MAC_STAGING
echo "$EPX_MAC_STAGING" | gh secret set EPX_MAC_STAGING
echo -e "${GREEN}✓ EPX_MAC_STAGING${NC}"

read -p "Enter EPX_CUST_NBR: " EPX_CUST_NBR
echo "$EPX_CUST_NBR" | gh secret set EPX_CUST_NBR
echo -e "${GREEN}✓ EPX_CUST_NBR${NC}"

read -s -p "Enter EPX_API_KEY: " EPX_API_KEY
echo ""
echo "$EPX_API_KEY" | gh secret set EPX_API_KEY
echo -e "${GREEN}✓ EPX_API_KEY${NC}"

read -p "Enter EPX_TERM_NBR: " EPX_TERM_NBR
echo "$EPX_TERM_NBR" | gh secret set EPX_TERM_NBR
echo -e "${GREEN}✓ EPX_TERM_NBR${NC}\n"

# Application Secrets
echo -e "${YELLOW}=== Application Secrets ===${NC}"
CRON_SECRET=$(openssl rand -base64 32)
echo "$CRON_SECRET" | gh secret set CRON_SECRET_STAGING
echo -e "${GREEN}✓ CRON_SECRET_STAGING (auto-generated)${NC}\n"

# GCP (Production) - Optional
echo -e "${YELLOW}=== Google Cloud Platform (Production) ===${NC}"
read -p "Do you want to configure GCP production secrets now? (y/n): " SETUP_GCP

if [ "$SETUP_GCP" = "y" ]; then
    read -p "Enter GCP_PROJECT_ID: " GCP_PROJECT_ID
    echo "$GCP_PROJECT_ID" | gh secret set GCP_PROJECT_ID
    echo -e "${GREEN}✓ GCP_PROJECT_ID${NC}"

    echo "Provide the path to your GCP service account JSON key file"
    read -p "Enter path to JSON key: " GCP_KEY_PATH
    if [ -f "$GCP_KEY_PATH" ]; then
        cat "$GCP_KEY_PATH" | gh secret set GCP_SERVICE_ACCOUNT_KEY
        echo -e "${GREEN}✓ GCP_SERVICE_ACCOUNT_KEY${NC}"
    else
        echo -e "${RED}File not found: $GCP_KEY_PATH${NC}"
    fi
else
    echo -e "${YELLOW}Skipping GCP configuration. You can add these later.${NC}"
fi

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✓ All secrets configured successfully!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${BLUE}Next steps:${NC}"
echo "1. Uncomment deployment stages in .github/workflows/ci-cd.yml"
echo "2. Commit and push the changes"
echo "3. Watch the CI/CD pipeline deploy automatically!"
echo ""
echo "View secrets at: https://github.com/$REPO/settings/secrets/actions"
