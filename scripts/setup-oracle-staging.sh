#!/bin/bash
set -e

# Oracle Cloud Staging Setup Script
# This script sets up your complete Oracle Cloud staging environment
# and generates all GitHub secrets needed for CI/CD

echo "=================================================="
echo "Oracle Cloud Staging Setup"
echo "=================================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if OCI CLI is installed
if ! command -v oci &> /dev/null; then
    echo -e "${RED}❌ OCI CLI not found. Installing...${NC}"
    bash -c "$(curl -L https://raw.githubusercontent.com/oracle/oci-cli/master/scripts/install/install.sh)"
    echo "Please reload your shell and run this script again."
    exit 1
fi

echo -e "${GREEN}✅ OCI CLI found${NC}"
echo ""

# Create directories
mkdir -p ~/oracle-staging-setup
mkdir -p ~/oracle-wallet
cd ~/oracle-staging-setup

# Configuration file
CONFIG_FILE="./oracle-config.env"

# Check if already configured
if [ -f "$CONFIG_FILE" ]; then
    echo -e "${YELLOW}⚠️  Found existing configuration. Load it? (y/n)${NC}"
    read -r LOAD_CONFIG
    if [ "$LOAD_CONFIG" = "y" ]; then
        source "$CONFIG_FILE"
        echo -e "${GREEN}✅ Configuration loaded${NC}"
    fi
fi

echo "=================================================="
echo "Step 1: Configuration"
echo "=================================================="
echo ""

# Get user inputs
echo "Enter configuration (press Enter for defaults):"
echo ""

read -p "Region (e.g., us-ashburn-1, us-phoenix-1, ca-toronto-1) [$REGION]: " INPUT_REGION
REGION=${INPUT_REGION:-${REGION:-us-ashburn-1}}

read -p "Database Name (max 14 chars) [$DB_NAME]: " INPUT_DB_NAME
DB_NAME=${INPUT_DB_NAME:-${DB_NAME:-paymentdb}}

read -p "Database Admin Password [$DB_ADMIN_PASSWORD]: " INPUT_DB_ADMIN_PASSWORD
DB_ADMIN_PASSWORD=${INPUT_DB_ADMIN_PASSWORD:-${DB_ADMIN_PASSWORD:-PaymentDB2025!}}

read -p "App User Password [$DB_APP_PASSWORD]: " INPUT_DB_APP_PASSWORD
DB_APP_PASSWORD=${INPUT_DB_APP_PASSWORD:-${DB_APP_PASSWORD:-PaymentApp2025!}}

read -p "Instance Name [$INSTANCE_NAME]: " INPUT_INSTANCE_NAME
INSTANCE_NAME=${INPUT_INSTANCE_NAME:-${INSTANCE_NAME:-payment-staging}}

# Fixed values
DB_APP_USER="payment_service"
INSTANCE_SHAPE="VM.Standard.A1.Flex"
INSTANCE_OCPUS=4
INSTANCE_MEMORY_GB=24

# EPX Sandbox Credentials
EPX_MAC="2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"

# Generate CRON_SECRET
if [ -z "$CRON_SECRET" ]; then
    CRON_SECRET=$(openssl rand -base64 32)
fi

# Save configuration
cat > "$CONFIG_FILE" <<EOF
# Oracle Cloud Configuration
export REGION="$REGION"
export DB_NAME="$DB_NAME"
export DB_ADMIN_PASSWORD="$DB_ADMIN_PASSWORD"
export DB_APP_USER="$DB_APP_USER"
export DB_APP_PASSWORD="$DB_APP_PASSWORD"
export INSTANCE_NAME="$INSTANCE_NAME"
export INSTANCE_SHAPE="$INSTANCE_SHAPE"
export INSTANCE_OCPUS=$INSTANCE_OCPUS
export INSTANCE_MEMORY_GB=$INSTANCE_MEMORY_GB
export EPX_MAC="$EPX_MAC"
export CRON_SECRET="$CRON_SECRET"
EOF

source "$CONFIG_FILE"

echo ""
echo -e "${GREEN}✅ Configuration saved to $CONFIG_FILE${NC}"
echo ""

# Check OCI CLI configuration
echo "=================================================="
echo "Step 2: OCI CLI Configuration"
echo "=================================================="
echo ""

if [ ! -f ~/.oci/config ]; then
    echo -e "${YELLOW}⚠️  OCI CLI not configured. Running setup...${NC}"
    echo ""
    echo "You'll need:"
    echo "  1. User OCID (Profile → User Settings → OCID)"
    echo "  2. Tenancy OCID (Profile → Tenancy → OCID)"
    echo "  3. Home Region (e.g., us-ashburn-1)"
    echo ""
    read -p "Press Enter to continue..."
    oci setup config

    echo ""
    echo -e "${YELLOW}⚠️  IMPORTANT: Add the API key to your Oracle Cloud account${NC}"
    echo "Public key location: ~/.oci/oci_api_key_public.pem"
    echo ""
    echo "Run this command to upload it:"
    echo "  oci iam user api-key upload --user-id \$(oci iam user list --all | jq -r '.data[0].id') --key-file ~/.oci/oci_api_key_public.pem"
    echo ""
    read -p "Press Enter after adding the API key..."
fi

echo -e "${GREEN}✅ OCI CLI configured${NC}"
echo ""

# Test OCI CLI
echo "Testing OCI CLI connection..."
if ! oci iam region list > /dev/null 2>&1; then
    echo -e "${RED}❌ Failed to connect to Oracle Cloud. Please check your configuration.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Connected to Oracle Cloud${NC}"
echo ""

# Get compartment ID and tenancy namespace
echo "Getting account details..."
COMPARTMENT_ID=$(oci iam compartment list --all 2>/dev/null | jq -r '.data[0].id' || oci iam availability-domain list 2>/dev/null | jq -r '.data[0]."compartment-id"')
TENANCY_NAMESPACE=$(oci os ns get | jq -r '.data')

echo "export COMPARTMENT_ID=\"$COMPARTMENT_ID\"" >> "$CONFIG_FILE"
echo "export TENANCY_NAMESPACE=\"$TENANCY_NAMESPACE\"" >> "$CONFIG_FILE"
source "$CONFIG_FILE"

echo -e "${GREEN}✅ Compartment ID: $COMPARTMENT_ID${NC}"
echo -e "${GREEN}✅ Tenancy Namespace: $TENANCY_NAMESPACE${NC}"
echo ""

# Map region to OCIR region key
case "$REGION" in
  "us-ashburn-1") OCIR_REGION="iad" ;;
  "us-phoenix-1") OCIR_REGION="phx" ;;
  "ca-toronto-1") OCIR_REGION="yyz" ;;
  "eu-frankfurt-1") OCIR_REGION="fra" ;;
  "uk-london-1") OCIR_REGION="lhr" ;;
  "ap-tokyo-1") OCIR_REGION="nrt" ;;
  "ap-seoul-1") OCIR_REGION="icn" ;;
  "ap-mumbai-1") OCIR_REGION="bom" ;;
  "ap-sydney-1") OCIR_REGION="syd" ;;
  *) OCIR_REGION="${REGION%-*}" ;;
esac

echo "export OCIR_REGION=\"$OCIR_REGION\"" >> "$CONFIG_FILE"
source "$CONFIG_FILE"

echo -e "${GREEN}✅ OCIR Region: $OCIR_REGION${NC}"
echo ""

echo "=================================================="
echo "Step 3: Create Autonomous Database"
echo "=================================================="
echo ""

# Check if database already exists
DB_OCID=$(oci db autonomous-database list \
  --compartment-id "$COMPARTMENT_ID" \
  --display-name "payment-staging-db" 2>/dev/null \
  | jq -r '.data[0].id // empty')

if [ -n "$DB_OCID" ]; then
    echo -e "${YELLOW}⚠️  Database already exists: $DB_OCID${NC}"
    echo "Do you want to use this database? (y/n)"
    read -r USE_EXISTING
    if [ "$USE_EXISTING" != "y" ]; then
        echo "Please delete the existing database first or choose a different name."
        exit 1
    fi
else
    echo "Creating Autonomous Database (this takes ~3 minutes)..."

    oci db autonomous-database create \
      --compartment-id "$COMPARTMENT_ID" \
      --db-name "$DB_NAME" \
      --display-name "payment-staging-db" \
      --admin-password "$DB_ADMIN_PASSWORD" \
      --cpu-core-count 1 \
      --data-storage-size-in-tbs 0.02 \
      --db-version "19c" \
      --db-workload "OLTP" \
      --is-free-tier true \
      --license-model "LICENSE_INCLUDED" \
      --wait-for-state "AVAILABLE" \
      > /dev/null

    DB_OCID=$(oci db autonomous-database list \
      --compartment-id "$COMPARTMENT_ID" \
      --display-name "payment-staging-db" \
      | jq -r '.data[0].id')

    echo -e "${GREEN}✅ Database created${NC}"
fi

echo "export DB_OCID=\"$DB_OCID\"" >> "$CONFIG_FILE"
source "$CONFIG_FILE"

echo "Database OCID: $DB_OCID"
echo ""

# Download wallet
echo "Downloading database wallet..."
oci db autonomous-database generate-wallet \
  --autonomous-database-id "$DB_OCID" \
  --password "$DB_ADMIN_PASSWORD" \
  --file ~/oracle-wallet/wallet.zip

cd ~/oracle-wallet
unzip -o wallet.zip
rm wallet.zip
cd ~/oracle-staging-setup

echo -e "${GREEN}✅ Wallet downloaded to ~/oracle-wallet${NC}"
echo ""

echo "=================================================="
echo "Step 4: Create VCN and Networking"
echo "=================================================="
echo ""

# Check if VCN exists
VCN_OCID=$(oci network vcn list \
  --compartment-id "$COMPARTMENT_ID" \
  --display-name "payment-staging-vcn" 2>/dev/null \
  | jq -r '.data[0].id // empty')

if [ -z "$VCN_OCID" ]; then
    echo "Creating VCN..."
    VCN_OCID=$(oci network vcn create \
      --compartment-id "$COMPARTMENT_ID" \
      --display-name "payment-staging-vcn" \
      --cidr-block "10.0.0.0/16" \
      --dns-label "paymentvcn" \
      --wait-for-state "AVAILABLE" \
      | jq -r '.data.id')

    echo -e "${GREEN}✅ VCN created${NC}"
else
    echo -e "${YELLOW}⚠️  VCN already exists${NC}"
fi

echo "export VCN_OCID=\"$VCN_OCID\"" >> "$CONFIG_FILE"
source "$CONFIG_FILE"

# Create Internet Gateway
IGW_OCID=$(oci network internet-gateway list \
  --compartment-id "$COMPARTMENT_ID" \
  --vcn-id "$VCN_OCID" 2>/dev/null \
  | jq -r '.data[0].id // empty')

if [ -z "$IGW_OCID" ]; then
    echo "Creating Internet Gateway..."
    IGW_OCID=$(oci network internet-gateway create \
      --compartment-id "$COMPARTMENT_ID" \
      --vcn-id "$VCN_OCID" \
      --display-name "payment-igw" \
      --is-enabled true \
      --wait-for-state "AVAILABLE" \
      | jq -r '.data.id')

    echo -e "${GREEN}✅ Internet Gateway created${NC}"
else
    echo -e "${YELLOW}⚠️  Internet Gateway already exists${NC}"
fi

echo "export IGW_OCID=\"$IGW_OCID\"" >> "$CONFIG_FILE"
source "$CONFIG_FILE"

# Update default route table
RT_OCID=$(oci network route-table list \
  --compartment-id "$COMPARTMENT_ID" \
  --vcn-id "$VCN_OCID" \
  | jq -r '.data[0].id')

echo "Updating route table..."
oci network route-table update \
  --rt-id "$RT_OCID" \
  --route-rules "[{\"destination\":\"0.0.0.0/0\",\"destinationType\":\"CIDR_BLOCK\",\"networkEntityId\":\"$IGW_OCID\"}]" \
  --force > /dev/null

echo -e "${GREEN}✅ Route table updated${NC}"

# Update default security list
SL_OCID=$(oci network security-list list \
  --compartment-id "$COMPARTMENT_ID" \
  --vcn-id "$VCN_OCID" \
  | jq -r '.data[0].id')

echo "Updating security list..."
oci network security-list update \
  --security-list-id "$SL_OCID" \
  --ingress-security-rules '[
    {
      "source": "0.0.0.0/0",
      "protocol": "6",
      "tcpOptions": {"destinationPortRange": {"min": 22, "max": 22}},
      "description": "SSH"
    },
    {
      "source": "0.0.0.0/0",
      "protocol": "6",
      "tcpOptions": {"destinationPortRange": {"min": 8080, "max": 8080}},
      "description": "gRPC"
    },
    {
      "source": "0.0.0.0/0",
      "protocol": "6",
      "tcpOptions": {"destinationPortRange": {"min": 8081, "max": 8081}},
      "description": "HTTP"
    }
  ]' \
  --force > /dev/null

echo -e "${GREEN}✅ Security list updated (ports 22, 8080, 8081 open)${NC}"

# Create subnet
SUBNET_OCID=$(oci network subnet list \
  --compartment-id "$COMPARTMENT_ID" \
  --vcn-id "$VCN_OCID" \
  --display-name "payment-public-subnet" 2>/dev/null \
  | jq -r '.data[0].id // empty')

if [ -z "$SUBNET_OCID" ]; then
    echo "Creating subnet..."
    SUBNET_OCID=$(oci network subnet create \
      --compartment-id "$COMPARTMENT_ID" \
      --vcn-id "$VCN_OCID" \
      --display-name "payment-public-subnet" \
      --cidr-block "10.0.1.0/24" \
      --dns-label "paymentpub" \
      --route-table-id "$RT_OCID" \
      --security-list-ids "[\"$SL_OCID\"]" \
      --wait-for-state "AVAILABLE" \
      | jq -r '.data.id')

    echo -e "${GREEN}✅ Subnet created${NC}"
else
    echo -e "${YELLOW}⚠️  Subnet already exists${NC}"
fi

echo "export SUBNET_OCID=\"$SUBNET_OCID\"" >> "$CONFIG_FILE"
source "$CONFIG_FILE"
echo ""

echo "=================================================="
echo "Step 5: Create Compute Instance"
echo "=================================================="
echo ""

# Check if instance exists
INSTANCE_OCID=$(oci compute instance list \
  --compartment-id "$COMPARTMENT_ID" \
  --display-name "$INSTANCE_NAME" 2>/dev/null \
  | jq -r '.data[0].id // empty')

if [ -n "$INSTANCE_OCID" ]; then
    echo -e "${YELLOW}⚠️  Instance already exists: $INSTANCE_OCID${NC}"
    INSTANCE_STATE=$(oci compute instance get --instance-id "$INSTANCE_OCID" | jq -r '.data."lifecycle-state"')
    echo "Instance state: $INSTANCE_STATE"

    if [ "$INSTANCE_STATE" != "RUNNING" ]; then
        echo "Starting instance..."
        oci compute instance action --instance-id "$INSTANCE_OCID" --action START --wait-for-state RUNNING > /dev/null
    fi
else
    # Get availability domain
    AD=$(oci iam availability-domain list --compartment-id "$COMPARTMENT_ID" | jq -r '.data[0].name')

    # Get Ubuntu 22.04 Arm image
    echo "Finding Ubuntu 22.04 Arm image..."
    IMAGE_OCID=$(oci compute image list \
      --compartment-id "$COMPARTMENT_ID" \
      --operating-system "Canonical Ubuntu" \
      --operating-system-version "22.04" \
      --shape "$INSTANCE_SHAPE" \
      --sort-by "TIMECREATED" \
      --sort-order "DESC" \
      --limit 1 \
      | jq -r '.data[0].id')

    if [ -z "$IMAGE_OCID" ] || [ "$IMAGE_OCID" = "null" ]; then
        echo -e "${RED}❌ Could not find Ubuntu 22.04 image for Ampere${NC}"
        echo "Trying Oracle Linux instead..."
        IMAGE_OCID=$(oci compute image list \
          --compartment-id "$COMPARTMENT_ID" \
          --operating-system "Oracle Linux" \
          --shape "$INSTANCE_SHAPE" \
          --sort-by "TIMECREATED" \
          --sort-order "DESC" \
          --limit 1 \
          | jq -r '.data[0].id')
    fi

    # Generate SSH key if it doesn't exist
    if [ ! -f ~/.ssh/oracle-staging ]; then
        echo "Generating SSH key..."
        ssh-keygen -t rsa -b 4096 -f ~/.ssh/oracle-staging -N ""
        echo -e "${GREEN}✅ SSH key generated at ~/.ssh/oracle-staging${NC}"
    fi

    echo "Creating compute instance (this takes ~2 minutes)..."
    echo "  Shape: $INSTANCE_SHAPE"
    echo "  OCPUs: $INSTANCE_OCPUS"
    echo "  Memory: ${INSTANCE_MEMORY_GB}GB"
    echo "  Image: $IMAGE_OCID"

    INSTANCE_OCID=$(oci compute instance launch \
      --compartment-id "$COMPARTMENT_ID" \
      --availability-domain "$AD" \
      --display-name "$INSTANCE_NAME" \
      --shape "$INSTANCE_SHAPE" \
      --shape-config "{\"ocpus\":$INSTANCE_OCPUS,\"memoryInGBs\":$INSTANCE_MEMORY_GB}" \
      --image-id "$IMAGE_OCID" \
      --subnet-id "$SUBNET_OCID" \
      --assign-public-ip true \
      --ssh-authorized-keys-file ~/.ssh/oracle-staging.pub \
      --wait-for-state "RUNNING" \
      | jq -r '.data.id')

    echo -e "${GREEN}✅ Instance created${NC}"
fi

echo "export INSTANCE_OCID=\"$INSTANCE_OCID\"" >> "$CONFIG_FILE"
source "$CONFIG_FILE"

# Get public IP
echo "Getting public IP..."
sleep 5  # Wait for network to be ready
VNIC_ID=$(oci compute instance list-vnics --instance-id "$INSTANCE_OCID" | jq -r '.data[0].id')
PUBLIC_IP=$(oci network vnic get --vnic-id "$VNIC_ID" | jq -r '.data."public-ip"')

echo "export PUBLIC_IP=\"$PUBLIC_IP\"" >> "$CONFIG_FILE"
source "$CONFIG_FILE"

echo -e "${GREEN}✅ Instance Public IP: $PUBLIC_IP${NC}"
echo ""

# Wait for SSH to be ready
echo "Waiting for SSH to be ready..."
sleep 30
for i in {1..10}; do
    if ssh -i ~/.ssh/oracle-staging -o StrictHostKeyChecking=no -o ConnectTimeout=5 ubuntu@$PUBLIC_IP "echo 'SSH ready'" 2>/dev/null; then
        echo -e "${GREEN}✅ SSH connection successful${NC}"
        break
    fi
    echo "Attempt $i/10... waiting..."
    sleep 10
done

echo ""

echo "=================================================="
echo "Step 6: Configure Compute Instance"
echo "=================================================="
echo ""

echo "Installing Docker and dependencies..."
ssh -i ~/.ssh/oracle-staging -o StrictHostKeyChecking=no ubuntu@$PUBLIC_IP 'bash -s' <<'ENDSSH'
set -e

# Update system
sudo apt update

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
rm get-docker.sh

# Add user to docker group
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Configure firewall
sudo ufw allow 22/tcp
sudo ufw allow 8080/tcp
sudo ufw allow 8081/tcp
sudo ufw --force enable

# Verify
docker --version
docker-compose --version

echo "Docker and Docker Compose installed successfully"
ENDSSH

echo -e "${GREEN}✅ Docker installed${NC}"

# Upload Oracle wallet
echo "Uploading Oracle wallet..."
scp -i ~/.ssh/oracle-staging -o StrictHostKeyChecking=no -r ~/oracle-wallet ubuntu@$PUBLIC_IP:~/

echo -e "${GREEN}✅ Wallet uploaded${NC}"
echo ""

echo "=================================================="
echo "Step 7: Create Auth Token for OCIR"
echo "=================================================="
echo ""

# Check if auth token file exists
AUTH_TOKEN_FILE="./ocir-auth-token.txt"

if [ -f "$AUTH_TOKEN_FILE" ]; then
    echo -e "${YELLOW}⚠️  Using existing auth token from $AUTH_TOKEN_FILE${NC}"
    AUTH_TOKEN=$(cat "$AUTH_TOKEN_FILE")
else
    echo "Creating auth token for Oracle Container Registry..."
    USER_OCID=$(oci iam user list --all | jq -r '.data[0].id')

    AUTH_TOKEN=$(oci iam auth-token create \
      --user-id "$USER_OCID" \
      --description "github-actions-ocir-$(date +%Y%m%d)" \
      | jq -r '.data.token')

    echo "$AUTH_TOKEN" > "$AUTH_TOKEN_FILE"
    chmod 600 "$AUTH_TOKEN_FILE"

    echo -e "${GREEN}✅ Auth token created and saved to $AUTH_TOKEN_FILE${NC}"
    echo -e "${YELLOW}⚠️  IMPORTANT: This token is only shown once. Keep the file safe!${NC}"
fi

# Get OCIR username
USER_EMAIL=$(oci iam user list --all | jq -r '.data[0].email')
OCIR_USERNAME="${TENANCY_NAMESPACE}/oracleidentitycloudservice/${USER_EMAIL}"

echo "export AUTH_TOKEN=\"$AUTH_TOKEN\"" >> "$CONFIG_FILE"
echo "export OCIR_USERNAME=\"$OCIR_USERNAME\"" >> "$CONFIG_FILE"
source "$CONFIG_FILE"

echo ""
echo "OCIR Details:"
echo "  Region: $OCIR_REGION"
echo "  Username: $OCIR_USERNAME"
echo "  Registry: ${OCIR_REGION}.ocir.io/${TENANCY_NAMESPACE}/payment-service"
echo ""

echo "=================================================="
echo "Step 8: Generate GitHub Secrets"
echo "=================================================="
echo ""

GITHUB_SECRETS_FILE="./github-secrets.txt"

cat > "$GITHUB_SECRETS_FILE" <<EOF
# ================================================
# GitHub Secrets for Oracle Cloud Staging
# ================================================
# Add these to: GitHub → Settings → Secrets and variables → Actions
# Environment: staging

ORACLE_CLOUD_HOST=$PUBLIC_IP
OCIR_REGION=$OCIR_REGION
OCIR_TENANCY_NAMESPACE=$TENANCY_NAMESPACE
OCIR_USERNAME=$OCIR_USERNAME
OCIR_AUTH_TOKEN=$AUTH_TOKEN
EPX_MAC_STAGING=$EPX_MAC
ORACLE_DB_PASSWORD=$DB_APP_PASSWORD
CRON_SECRET_STAGING=$CRON_SECRET

# SSH Private Key (ORACLE_CLOUD_SSH_KEY):
# Copy the content of: ~/.ssh/oracle-staging
# Including the BEGIN and END lines

# ================================================
# Service URLs
# ================================================
Staging URL: http://$PUBLIC_IP:8081
Database: ${DB_NAME}_tp
App User: ${DB_APP_USER}

# ================================================
# To add secrets to GitHub:
# ================================================
# 1. Go to: https://github.com/YOUR_USERNAME/payment-service/settings/secrets/actions
# 2. Click "New repository secret" for each secret above
# 3. For ORACLE_CLOUD_SSH_KEY, run: cat ~/.ssh/oracle-staging
#    Copy the ENTIRE output including BEGIN/END lines
EOF

echo -e "${GREEN}✅ GitHub secrets saved to: $GITHUB_SECRETS_FILE${NC}"
echo ""

echo "=================================================="
echo "Step 9: Test Deployment"
echo "=================================================="
echo ""

# Create application directory on server
echo "Setting up application directory..."
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP <<ENDSSH
# Create directory
mkdir -p ~/payment-service

# Create .env file
cat > ~/payment-service/.env <<'EOF'
PORT=8080
HTTP_PORT=8081
ENVIRONMENT=staging

DATABASE_URL=oracle://${DB_APP_USER}:${DB_APP_PASSWORD}@${DB_NAME}_tp
TNS_ADMIN=/app/oracle-wallet
DB_SSL_MODE=require

EPX_SERVER_POST_URL=https://secure.epxuap.com
EPX_TIMEOUT=30
EPX_CUST_NBR=9001
EPX_MERCH_NBR=900300
EPX_DBA_NBR=2
EPX_TERMINAL_NBR=77
EPX_MAC=${EPX_MAC}

NORTH_MERCHANT_REPORTING_URL=https://api.north.com
NORTH_TIMEOUT=30

CALLBACK_BASE_URL=http://${PUBLIC_IP}:8081
CRON_SECRET=${CRON_SECRET}

LOG_LEVEL=debug
EOF

# Create docker-compose.yml
cat > ~/payment-service/docker-compose.yml <<'DOCKERCOMPOSE'
version: '3.8'

services:
  payment-service:
    image: ${OCIR_REGION}.ocir.io/${TENANCY_NAMESPACE}/payment-service:latest
    container_name: payment-staging
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "8081:8081"
    env_file:
      - .env
    volumes:
      - /home/ubuntu/oracle-wallet:/app/oracle-wallet:ro
    environment:
      - TNS_ADMIN=/app/oracle-wallet
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8081/cron/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
DOCKERCOMPOSE

# Replace variables in .env
sed -i "s/\${DB_APP_USER}/${DB_APP_USER}/" .env
sed -i "s/\${DB_APP_PASSWORD}/${DB_APP_PASSWORD}/" .env
sed -i "s/\${DB_NAME}/${DB_NAME}/" .env
sed -i "s/\${EPX_MAC}/${EPX_MAC}/" .env
sed -i "s/\${PUBLIC_IP}/${PUBLIC_IP}/" .env
sed -i "s/\${CRON_SECRET}/${CRON_SECRET}/" .env

# Replace variables in docker-compose.yml
sed -i "s/\${OCIR_REGION}/${OCIR_REGION}/" docker-compose.yml
sed -i "s/\${TENANCY_NAMESPACE}/${TENANCY_NAMESPACE}/" docker-compose.yml

echo "Application directory configured"
ENDSSH

echo -e "${GREEN}✅ Application directory configured on server${NC}"
echo ""

echo "=================================================="
echo "SETUP COMPLETE!"
echo "=================================================="
echo ""
echo -e "${GREEN}✅ All resources created successfully!${NC}"
echo ""
echo "Resources:"
echo "  - Autonomous Database: ${DB_NAME}"
echo "  - Compute Instance: $INSTANCE_NAME"
echo "  - Public IP: $PUBLIC_IP"
echo "  - VCN: payment-staging-vcn"
echo ""
echo "Next Steps:"
echo ""
echo "1. Add GitHub Secrets:"
echo "   - File: $GITHUB_SECRETS_FILE"
echo "   - Go to: https://github.com/YOUR_USERNAME/payment-service/settings/secrets/actions"
echo "   - Add each secret from the file"
echo "   - For ORACLE_CLOUD_SSH_KEY, run: cat ~/.ssh/oracle-staging"
echo ""
echo "2. Deploy Application:"
echo "   git checkout develop"
echo "   git add ."
echo "   git commit -m \"feat: Oracle Cloud staging setup\""
echo "   git push origin develop"
echo ""
echo "3. Monitor Deployment:"
echo "   - GitHub: https://github.com/YOUR_USERNAME/payment-service/actions"
echo "   - Staging URL: http://$PUBLIC_IP:8081/cron/health"
echo ""
echo "Configuration saved to: $CONFIG_FILE"
echo "GitHub secrets saved to: $GITHUB_SECRETS_FILE"
echo "Auth token saved to: $AUTH_TOKEN_FILE"
echo ""
echo -e "${YELLOW}Cost: \$0/month forever (Always Free Tier)${NC}"
echo ""
