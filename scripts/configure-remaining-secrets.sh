#!/bin/bash
set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Configure Remaining GitHub Secrets${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Check OCI CLI is available
if ! command -v oci &> /dev/null; then
    echo -e "${RED}Error: OCI CLI not found${NC}"
    exit 1
fi

# 1. OCI Compartment OCID (auto-detect if possible)
echo -e "${YELLOW}1. OCI_COMPARTMENT_OCID${NC}"
echo "Attempting to auto-detect compartment..."
export SUPPRESS_LABEL_WARNING=True

# Get tenancy OCID from config
TENANCY_OCID=$(grep "^tenancy=" ~/.oci/config | cut -d'=' -f2)

# Try to get root compartment (same as tenancy for most cases)
echo -e "Using root compartment (tenancy): ${BLUE}${TENANCY_OCID}${NC}"
read -p "Use this compartment? (y/n): " use_root

if [ "$use_root" = "y" ]; then
    echo "$TENANCY_OCID" | gh secret set OCI_COMPARTMENT_OCID
    echo -e "${GREEN}✓ OCI_COMPARTMENT_OCID${NC}\n"
else
    echo "Manual entry required"
    read -p "Enter OCI_COMPARTMENT_OCID: " value
    echo "$value" | gh secret set OCI_COMPARTMENT_OCID
    echo -e "${GREEN}✓ OCI_COMPARTMENT_OCID${NC}\n"
fi

# 2. SSH Keys (auto-detect)
echo -e "${YELLOW}2. SSH Keys for VM Access${NC}"
if [ -f "$HOME/.ssh/id_rsa.pub" ]; then
    echo -e "Found existing SSH key at ${BLUE}~/.ssh/id_rsa${NC}"
    cat "$HOME/.ssh/id_rsa.pub" | gh secret set SSH_PUBLIC_KEY
    echo -e "${GREEN}✓ SSH_PUBLIC_KEY${NC}"

    cat "$HOME/.ssh/id_rsa" | gh secret set ORACLE_CLOUD_SSH_KEY
    echo -e "${GREEN}✓ ORACLE_CLOUD_SSH_KEY${NC}\n"
else
    echo "No SSH key found. Generating new key..."
    ssh-keygen -t rsa -b 4096 -C "github-actions@payment-service" -f "$HOME/.ssh/id_rsa" -N ""
    cat "$HOME/.ssh/id_rsa.pub" | gh secret set SSH_PUBLIC_KEY
    cat "$HOME/.ssh/id_rsa" | gh secret set ORACLE_CLOUD_SSH_KEY
    echo -e "${GREEN}✓ SSH keys generated and configured${NC}\n"
fi

# 3. Oracle Database Passwords
echo -e "${YELLOW}3. Oracle Database Passwords${NC}"
echo "These will be used to create the Autonomous Database"

read -s -p "Enter ORACLE_DB_ADMIN_PASSWORD (16+ chars, mixed case, numbers): " value
echo ""
echo "$value" | gh secret set ORACLE_DB_ADMIN_PASSWORD
echo -e "${GREEN}✓ ORACLE_DB_ADMIN_PASSWORD${NC}"

read -s -p "Enter ORACLE_DB_PASSWORD (for app user 'payment_service'): " value
echo ""
echo "$value" | gh secret set ORACLE_DB_PASSWORD
echo -e "${GREEN}✓ ORACLE_DB_PASSWORD${NC}"

# Auto-set ORACLE_DB_USER (hardcoded in Terraform)
echo "payment_service" | gh secret set ORACLE_DB_USER
echo -e "${GREEN}✓ ORACLE_DB_USER (auto: payment_service)${NC}\n"

# 4. Oracle Container Registry
echo -e "${YELLOW}4. Oracle Container Registry (OCIR)${NC}"

# Try to get namespace automatically
echo "Fetching OCIR namespace..."
NAMESPACE=$(oci os ns get 2>/dev/null | grep -o '"namespace": "[^"]*"' | cut -d'"' -f4 || echo "")

if [ -n "$NAMESPACE" ]; then
    echo -e "Found namespace: ${BLUE}${NAMESPACE}${NC}"
    echo "$NAMESPACE" | gh secret set OCIR_TENANCY_NAMESPACE
    echo -e "${GREEN}✓ OCIR_TENANCY_NAMESPACE${NC}"
else
    echo "Could not auto-detect. Find at: Tenancy Details → Object Storage Namespace"
    read -p "Enter OCIR_TENANCY_NAMESPACE: " value
    echo "$value" | gh secret set OCIR_TENANCY_NAMESPACE
    echo -e "${GREEN}✓ OCIR_TENANCY_NAMESPACE${NC}"
    NAMESPACE="$value"
fi

echo -e "\nOCIR Username format: ${BLUE}${NAMESPACE}/oracleidentitycloudservice/your.email@example.com${NC}"
read -p "Enter your Oracle Cloud email: " email
OCIR_USER="${NAMESPACE}/oracleidentitycloudservice/${email}"
echo "$OCIR_USER" | gh secret set OCIR_USERNAME
echo -e "${GREEN}✓ OCIR_USERNAME${NC}"

echo -e "\nGenerate Auth Token at: User Settings → Auth Tokens → Generate Token"
read -s -p "Enter OCIR_AUTH_TOKEN: " value
echo ""
echo "$value" | gh secret set OCIR_AUTH_TOKEN
echo -e "${GREEN}✓ OCIR_AUTH_TOKEN${NC}\n"

# 5. EPX Payment Processor
echo -e "${YELLOW}5. EPX Payment Processor Credentials${NC}"
echo "EPX is your payment gateway for processing credit card transactions"
echo ""
echo "Options:"
echo "  1) Use SANDBOX credentials (for testing - recommended for staging)"
echo "  2) Use PRODUCTION credentials (for real transactions)"
echo ""
read -p "Enter choice [1-2]: " epx_choice

if [ "$epx_choice" = "1" ]; then
    echo -e "\n${BLUE}Using EPX Sandbox Credentials (from .env.example)${NC}"

    echo "2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" | gh secret set EPX_MAC_STAGING
    echo -e "${GREEN}✓ EPX_MAC_STAGING (sandbox)${NC}"

    echo "9001" | gh secret set EPX_CUST_NBR
    echo -e "${GREEN}✓ EPX_CUST_NBR (sandbox)${NC}"

    echo "900300" | gh secret set EPX_MERCH_NBR
    echo -e "${GREEN}✓ EPX_MERCH_NBR (sandbox)${NC}"

    echo "2" | gh secret set EPX_DBA_NBR
    echo -e "${GREEN}✓ EPX_DBA_NBR (sandbox)${NC}"

    echo "77" | gh secret set EPX_TERMINAL_NBR
    echo -e "${GREEN}✓ EPX_TERMINAL_NBR (sandbox)${NC}"

    echo -e "\n${YELLOW}Note: Sandbox credentials allow testing without processing real transactions${NC}\n"
else
    echo -e "\n${BLUE}Enter Production EPX Credentials${NC}"
    echo "Contact your EPX account representative if you don't have these"
    echo ""

    read -p "Enter EPX_MAC_STAGING: " value
    echo "$value" | gh secret set EPX_MAC_STAGING
    echo -e "${GREEN}✓ EPX_MAC_STAGING${NC}"

    read -p "Enter EPX_CUST_NBR: " value
    echo "$value" | gh secret set EPX_CUST_NBR
    echo -e "${GREEN}✓ EPX_CUST_NBR${NC}"

    read -p "Enter EPX_MERCH_NBR: " value
    echo "$value" | gh secret set EPX_MERCH_NBR
    echo -e "${GREEN}✓ EPX_MERCH_NBR${NC}"

    read -p "Enter EPX_DBA_NBR: " value
    echo "$value" | gh secret set EPX_DBA_NBR
    echo -e "${GREEN}✓ EPX_DBA_NBR${NC}"

    read -p "Enter EPX_TERMINAL_NBR: " value
    echo "$value" | gh secret set EPX_TERMINAL_NBR
    echo -e "${GREEN}✓ EPX_TERMINAL_NBR${NC}\n"
fi

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✓ All secrets configured successfully!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${BLUE}Configured secrets:${NC}"
gh secret list

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Uncomment deployment stages in .github/workflows/ci-cd.yml"
echo "2. Commit and push: git add . && git commit -m 'ci: Enable staging deployment'"
echo "3. Watch deployment at: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/actions"
