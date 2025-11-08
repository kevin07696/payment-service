#!/bin/bash

# Oracle Cloud Setup Verification Script
# Run this after setup-oracle-staging.sh to verify everything is working

echo "=================================================="
echo "Oracle Cloud Staging Verification"
echo "=================================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

ERRORS=0
WARNINGS=0

# Load configuration
CONFIG_FILE="$HOME/oracle-staging-setup/oracle-config.env"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}❌ Configuration file not found: $CONFIG_FILE${NC}"
    echo "Please run setup-oracle-staging.sh first"
    exit 1
fi

source "$CONFIG_FILE"

echo "Loaded configuration from: $CONFIG_FILE"
echo ""

# Function to check
check() {
    local name="$1"
    local command="$2"

    echo -n "Checking $name... "
    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}✅${NC}"
        return 0
    else
        echo -e "${RED}❌${NC}"
        ERRORS=$((ERRORS + 1))
        return 1
    fi
}

# Function to check with warning
check_warn() {
    local name="$1"
    local command="$2"

    echo -n "Checking $name... "
    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}✅${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠️${NC}"
        WARNINGS=$((WARNINGS + 1))
        return 1
    fi
}

echo "=================================================="
echo "1. Local Prerequisites"
echo "=================================================="
echo ""

check "OCI CLI installed" "command -v oci"
check "OCI CLI configured" "test -f ~/.oci/config"
check "jq installed" "command -v jq"
check "SSH key exists" "test -f ~/.ssh/oracle-staging"
check "Oracle wallet exists" "test -d ~/oracle-wallet"
check "Oracle wallet files" "test -f ~/oracle-wallet/tnsnames.ora"

echo ""

echo "=================================================="
echo "2. Oracle Cloud Connection"
echo "=================================================="
echo ""

check "OCI API connection" "oci iam region list"
check "Compartment accessible" "oci iam compartment list --compartment-id '$COMPARTMENT_ID'"

echo ""

echo "=================================================="
echo "3. Database"
echo "=================================================="
echo ""

if [ -n "$DB_OCID" ]; then
    check "Database exists" "oci db autonomous-database get --autonomous-database-id '$DB_OCID'"
    DB_STATE=$(oci db autonomous-database get --autonomous-database-id "$DB_OCID" 2>/dev/null | jq -r '.data."lifecycle-state"')

    if [ "$DB_STATE" = "AVAILABLE" ]; then
        echo -e "Database state: ${GREEN}$DB_STATE${NC}"
    else
        echo -e "Database state: ${YELLOW}$DB_STATE${NC}"
        WARNINGS=$((WARNINGS + 1))
    fi

    # Try SQL*Plus connection (if installed)
    if command -v sqlplus &> /dev/null; then
        export TNS_ADMIN=~/oracle-wallet
        check_warn "Database connection (ADMIN)" "echo 'SELECT 1 FROM DUAL;' | sqlplus -S ADMIN/${DB_ADMIN_PASSWORD}@${DB_NAME}_tp"
        check_warn "Database connection (App User)" "echo 'SELECT 1 FROM DUAL;' | sqlplus -S ${DB_APP_USER}/${DB_APP_PASSWORD}@${DB_NAME}_tp"
    else
        echo "SQL*Plus not installed - skipping database connection test"
    fi
else
    echo -e "${RED}❌ DB_OCID not set${NC}"
    ERRORS=$((ERRORS + 1))
fi

echo ""

echo "=================================================="
echo "4. Compute Instance"
echo "=================================================="
echo ""

if [ -n "$INSTANCE_OCID" ]; then
    check "Instance exists" "oci compute instance get --instance-id '$INSTANCE_OCID'"

    INSTANCE_STATE=$(oci compute instance get --instance-id "$INSTANCE_OCID" 2>/dev/null | jq -r '.data."lifecycle-state"')

    if [ "$INSTANCE_STATE" = "RUNNING" ]; then
        echo -e "Instance state: ${GREEN}$INSTANCE_STATE${NC}"
    else
        echo -e "Instance state: ${YELLOW}$INSTANCE_STATE${NC}"
        WARNINGS=$((WARNINGS + 1))
    fi

    if [ -n "$PUBLIC_IP" ]; then
        echo "Public IP: $PUBLIC_IP"

        check "SSH connection" "ssh -i ~/.ssh/oracle-staging -o ConnectTimeout=10 -o StrictHostKeyChecking=no ubuntu@$PUBLIC_IP 'echo test'"
        check "Docker installed" "ssh -i ~/.ssh/oracle-staging -o StrictHostKeyChecking=no ubuntu@$PUBLIC_IP 'docker --version'"
        check "Docker Compose installed" "ssh -i ~/.ssh/oracle-staging -o StrictHostKeyChecking=no ubuntu@$PUBLIC_IP 'docker-compose --version'"
        check "Wallet uploaded" "ssh -i ~/.ssh/oracle-staging -o StrictHostKeyChecking=no ubuntu@$PUBLIC_IP 'test -d ~/oracle-wallet'"
        check "App directory exists" "ssh -i ~/.ssh/oracle-staging -o StrictHostKeyChecking=no ubuntu@$PUBLIC_IP 'test -d ~/payment-service'"
    else
        echo -e "${RED}❌ PUBLIC_IP not set${NC}"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo -e "${RED}❌ INSTANCE_OCID not set${NC}"
    ERRORS=$((ERRORS + 1))
fi

echo ""

echo "=================================================="
echo "5. Networking"
echo "=================================================="
echo ""

check "VCN exists" "oci network vcn get --vcn-id '$VCN_OCID'"
check "Internet Gateway exists" "oci network internet-gateway get --ig-id '$IGW_OCID'"
check "Subnet exists" "oci network subnet get --subnet-id '$SUBNET_OCID'"

echo ""

echo "=================================================="
echo "6. Container Registry (OCIR)"
echo "=================================================="
echo ""

if [ -n "$AUTH_TOKEN" ]; then
    echo "Auth token: *** (hidden)"
    check "OCIR login" "echo '$AUTH_TOKEN' | docker login ${OCIR_REGION}.ocir.io -u '$OCIR_USERNAME' --password-stdin"
else
    echo -e "${RED}❌ AUTH_TOKEN not set${NC}"
    ERRORS=$((ERRORS + 1))
fi

echo "OCIR Region: $OCIR_REGION"
echo "OCIR Username: $OCIR_USERNAME"
echo "OCIR Namespace: $TENANCY_NAMESPACE"
echo "Image repository: ${OCIR_REGION}.ocir.io/${TENANCY_NAMESPACE}/payment-service"

echo ""

echo "=================================================="
echo "7. GitHub Secrets File"
echo "=================================================="
echo ""

SECRETS_FILE="$HOME/oracle-staging-setup/github-secrets.txt"
if [ -f "$SECRETS_FILE" ]; then
    echo -e "${GREEN}✅ Secrets file exists: $SECRETS_FILE${NC}"

    # Check required secrets in file
    required_secrets=(
        "ORACLE_CLOUD_HOST"
        "OCIR_REGION"
        "OCIR_TENANCY_NAMESPACE"
        "OCIR_USERNAME"
        "OCIR_AUTH_TOKEN"
        "EPX_MAC_STAGING"
        "ORACLE_DB_PASSWORD"
        "CRON_SECRET_STAGING"
    )

    for secret in "${required_secrets[@]}"; do
        if grep -q "^$secret=" "$SECRETS_FILE"; then
            echo -e "  ${GREEN}✅${NC} $secret"
        else
            echo -e "  ${RED}❌${NC} $secret (missing)"
            ERRORS=$((ERRORS + 1))
        fi
    done
else
    echo -e "${RED}❌ Secrets file not found: $SECRETS_FILE${NC}"
    ERRORS=$((ERRORS + 1))
fi

echo ""

echo "=================================================="
echo "8. Application Deployment Test"
echo "=================================================="
echo ""

if [ -n "$PUBLIC_IP" ]; then
    # Test if health endpoint is accessible
    echo -n "Testing health endpoint (http://$PUBLIC_IP:8081/cron/health)... "

    if curl -sf --connect-timeout 10 "http://$PUBLIC_IP:8081/cron/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✅${NC}"
        echo "Application is running!"
    else
        echo -e "${YELLOW}⚠️${NC}"
        echo "Application not deployed yet (this is OK if first time)"
        echo "It will be deployed automatically on first GitHub Actions run"
        WARNINGS=$((WARNINGS + 1))
    fi
fi

echo ""

echo "=================================================="
echo "Summary"
echo "=================================================="
echo ""

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}✅ All checks passed!${NC}"
    echo ""
    echo "Your Oracle Cloud staging environment is ready!"
    echo ""
    echo "Next steps:"
    echo "1. Add GitHub secrets from: $SECRETS_FILE"
    echo "2. Add SSH private key: cat ~/.ssh/oracle-staging"
    echo "3. Push to develop branch:"
    echo "   git checkout develop"
    echo "   git push origin develop"
    echo ""
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}⚠️  $WARNINGS warnings (setup incomplete but can proceed)${NC}"
    echo ""
    echo "You can continue with deployment, but some features may not work yet."
    echo ""
elif [ $ERRORS -le 3 ]; then
    echo -e "${YELLOW}⚠️  $ERRORS errors, $WARNINGS warnings${NC}"
    echo ""
    echo "Please fix the errors above before proceeding."
    echo ""
    exit 1
else
    echo -e "${RED}❌ $ERRORS errors, $WARNINGS warnings${NC}"
    echo ""
    echo "Significant issues found. Please review the errors above."
    echo ""
    echo "Common fixes:"
    echo "  - Re-run setup script: ./scripts/setup-oracle-staging.sh"
    echo "  - Check OCI CLI config: oci setup config"
    echo "  - Verify Oracle Cloud Console access"
    echo ""
    exit 1
fi

echo "Configuration file: $CONFIG_FILE"
echo "GitHub secrets file: $SECRETS_FILE"
echo "SSH key: ~/.ssh/oracle-staging"
echo ""
echo "Staging URL: http://$PUBLIC_IP:8081"
echo ""
