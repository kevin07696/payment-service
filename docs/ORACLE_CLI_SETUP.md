# Oracle Cloud Staging Setup - CLI Only

Set up your entire Oracle Cloud staging environment using **only the command line** (after initial account creation).

## Prerequisites

1. **Oracle Cloud account** (must create via web once)
   - Go to https://www.oracle.com/cloud/free/
   - Complete signup
   - Verify email and payment method
   - That's it for web console!

2. **Local machine** (Linux, macOS, or WSL)

---

## Part 1: Install Oracle CLI

### Step 1.1: Install OCI CLI

**Linux/macOS:**
```bash
bash -c "$(curl -L https://raw.githubusercontent.com/oracle/oci-cli/master/scripts/install/install.sh)"
```

During installation:
- Install location: `/home/yourusername/bin` (or default)
- Add to PATH: Yes
- Install Python dependencies: Yes
- Update PATH in bashrc: Yes

**Verify installation:**
```bash
# Reload shell
source ~/.bashrc

# Verify
oci --version
# Output: 3.x.x
```

### Step 1.2: Configure OCI CLI

```bash
oci setup config
```

You'll be prompted for:

1. **Config file location**: Press Enter (default: `~/.oci/config`)

2. **User OCID**:
   - Quick way: Go to https://cloud.oracle.com → Profile Icon → User Settings
   - Copy "OCID" (looks like: `ocid1.user.oc1..aaaa...`)
   - Or run: `oci iam user list --all | jq -r '.data[0].id'`

3. **Tenancy OCID**:
   - Profile Icon → Tenancy → Copy OCID
   - Or: Look in web console under "Tenancy Information"

4. **Region**:
   - Enter your home region (e.g., `us-ashburn-1`, `us-phoenix-1`, `ca-toronto-1`)
   - List regions: https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm

5. **Generate RSA key pair**: `Y`
   - Key location: Press Enter (default: `~/.oci/oci_api_key.pem`)

6. **Passphrase**: Press Enter (no passphrase for automation)

**Add public key to Oracle Cloud:**
```bash
# Display your public key
cat ~/.oci/oci_api_key_public.pem

# Copy the output
```

Now add it to your Oracle Cloud account:
```bash
# Via CLI (easier!)
oci iam user api-key upload \
  --user-id $(oci iam user list --all | jq -r '.data[0].id') \
  --key-file ~/.oci/oci_api_key_public.pem
```

Or via web console:
1. Go to Profile Icon → User Settings
2. Click "API Keys" (left menu)
3. Click "Add API Key"
4. Paste the public key
5. Click "Add"

**Test configuration:**
```bash
oci iam region list
# Should show list of regions
```

---

## Part 2: Set Up Variables

Create a configuration file for easy reference:

```bash
cat > ~/oracle-setup.env <<'EOF'
# Oracle Cloud Configuration
export COMPARTMENT_ID="ocid1.compartment.oc1..aaaa..."  # We'll get this
export REGION="us-ashburn-1"  # Your home region
export TENANCY_NAMESPACE="abcdef123456"  # We'll get this

# Compute
export INSTANCE_SHAPE="VM.Standard.A1.Flex"
export INSTANCE_OCPUS=4
export INSTANCE_MEMORY_GB=24
export INSTANCE_NAME="payment-staging"

# Database
export DB_NAME="paymentdb"
export DB_ADMIN_PASSWORD="PaymentDB2025!"
export DB_APP_USER="payment_service"
export DB_APP_PASSWORD="PaymentApp2025!"

# Application
export EPX_MAC="2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"
export CRON_SECRET=$(openssl rand -base64 32)
EOF

# Load variables
source ~/oracle-setup.env
```

**Get Compartment ID:**
```bash
# Get root compartment (usually what you want for Always Free)
COMPARTMENT_ID=$(oci iam compartment list --all | jq -r '.data[0].id')
echo "COMPARTMENT_ID=$COMPARTMENT_ID"

# Update the env file
sed -i "s|COMPARTMENT_ID=.*|COMPARTMENT_ID=\"$COMPARTMENT_ID\"|" ~/oracle-setup.env
source ~/oracle-setup.env
```

**Get Tenancy Namespace:**
```bash
TENANCY_NAMESPACE=$(oci os ns get | jq -r '.data')
echo "TENANCY_NAMESPACE=$TENANCY_NAMESPACE"

# Update the env file
sed -i "s|TENANCY_NAMESPACE=.*|TENANCY_NAMESPACE=\"$TENANCY_NAMESPACE\"|" ~/oracle-setup.env
source ~/oracle-setup.env
```

---

## Part 3: Create Autonomous Database (CLI)

### Step 3.1: Create Database

```bash
# Create Autonomous Database
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
  --wait-for-state "AVAILABLE"
```

This creates:
- ✅ Always Free tier database
- ✅ 20 GB storage
- ✅ 1 OCPU
- ✅ Transaction Processing workload

**Save Database OCID:**
```bash
DB_OCID=$(oci db autonomous-database list \
  --compartment-id "$COMPARTMENT_ID" \
  --display-name "payment-staging-db" \
  | jq -r '.data[0].id')

echo "DB_OCID=$DB_OCID"
echo "export DB_OCID=\"$DB_OCID\"" >> ~/oracle-setup.env
```

### Step 3.2: Download Wallet

```bash
# Create directory for wallet
mkdir -p ~/oracle-wallet

# Download wallet
oci db autonomous-database generate-wallet \
  --autonomous-database-id "$DB_OCID" \
  --password "$DB_ADMIN_PASSWORD" \
  --file ~/oracle-wallet/wallet.zip

# Extract wallet
cd ~/oracle-wallet
unzip wallet.zip
rm wallet.zip

echo "✅ Database wallet downloaded to ~/oracle-wallet"
```

### Step 3.3: Create Application User

```bash
# Install Oracle Instant Client (if not installed)
# Ubuntu/Debian:
sudo apt install -y alien libaio1
wget https://download.oracle.com/otn_software/linux/instantclient/2340000/oracle-instantclient-basic-23.4.0.24.05-1.el9.x86_64.rpm
sudo alien -i oracle-instantclient-basic-23.4.0.24.05-1.el9.x86_64.rpm
wget https://download.oracle.com/otn_software/linux/instantclient/2340000/oracle-instantclient-sqlplus-23.4.0.24.05-1.el9.x86_64.rpm
sudo alien -i oracle-instantclient-sqlplus-23.4.0.24.05-1.el9.x86_64.rpm

# Or Fedora/RHEL:
# sudo dnf install oracle-instantclient-basic oracle-instantclient-sqlplus

# Set TNS_ADMIN
export TNS_ADMIN=~/oracle-wallet
echo "export TNS_ADMIN=~/oracle-wallet" >> ~/.bashrc

# Create application user
sqlplus ADMIN/${DB_ADMIN_PASSWORD}@${DB_NAME}_tp <<EOF
CREATE USER ${DB_APP_USER} IDENTIFIED BY "${DB_APP_PASSWORD}";
GRANT CONNECT, RESOURCE TO ${DB_APP_USER};
GRANT UNLIMITED TABLESPACE TO ${DB_APP_USER};
GRANT CREATE SESSION, CREATE TABLE, CREATE SEQUENCE, CREATE TRIGGER TO ${DB_APP_USER};
SELECT username FROM all_users WHERE username = 'PAYMENT_SERVICE';
EXIT;
EOF

echo "✅ Application user created"
```

---

## Part 4: Create VCN and Networking (CLI)

### Step 4.1: Create VCN

```bash
# Create VCN
VCN_OCID=$(oci network vcn create \
  --compartment-id "$COMPARTMENT_ID" \
  --display-name "payment-staging-vcn" \
  --cidr-block "10.0.0.0/16" \
  --dns-label "paymentvcn" \
  --wait-for-state "AVAILABLE" \
  | jq -r '.data.id')

echo "VCN_OCID=$VCN_OCID"
echo "export VCN_OCID=\"$VCN_OCID\"" >> ~/oracle-setup.env
```

### Step 4.2: Create Internet Gateway

```bash
IGW_OCID=$(oci network internet-gateway create \
  --compartment-id "$COMPARTMENT_ID" \
  --vcn-id "$VCN_OCID" \
  --display-name "payment-igw" \
  --is-enabled true \
  --wait-for-state "AVAILABLE" \
  | jq -r '.data.id')

echo "IGW_OCID=$IGW_OCID"
echo "export IGW_OCID=\"$IGW_OCID\"" >> ~/oracle-setup.env
```

### Step 4.3: Create Route Table

```bash
# Get default route table
RT_OCID=$(oci network route-table list \
  --compartment-id "$COMPARTMENT_ID" \
  --vcn-id "$VCN_OCID" \
  | jq -r '.data[0].id')

# Add route to internet gateway
oci network route-table update \
  --rt-id "$RT_OCID" \
  --route-rules '[{"destination":"0.0.0.0/0","destinationType":"CIDR_BLOCK","networkEntityId":"'$IGW_OCID'"}]' \
  --force
```

### Step 4.4: Create Security List

```bash
# Get default security list
SL_OCID=$(oci network security-list list \
  --compartment-id "$COMPARTMENT_ID" \
  --vcn-id "$VCN_OCID" \
  | jq -r '.data[0].id')

# Add ingress rules
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
  --force
```

### Step 4.5: Create Subnet

```bash
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

echo "SUBNET_OCID=$SUBNET_OCID"
echo "export SUBNET_OCID=\"$SUBNET_OCID\"" >> ~/oracle-setup.env
```

---

## Part 5: Create Compute Instance (CLI)

### Step 5.1: Get Image OCID

```bash
# Find latest Ubuntu 22.04 Arm image for your region
IMAGE_OCID=$(oci compute image list \
  --compartment-id "$COMPARTMENT_ID" \
  --operating-system "Canonical Ubuntu" \
  --operating-system-version "22.04" \
  --shape "$INSTANCE_SHAPE" \
  --sort-by "TIMECREATED" \
  --sort-order "DESC" \
  --limit 1 \
  | jq -r '.data[0].id')

echo "IMAGE_OCID=$IMAGE_OCID"
```

### Step 5.2: Generate SSH Key

```bash
# Generate SSH key pair
ssh-keygen -t rsa -b 4096 -f ~/.ssh/oracle-staging -N ""

# Save public key for later
SSH_PUBLIC_KEY=$(cat ~/.ssh/oracle-staging.pub)
echo "SSH key generated at ~/.ssh/oracle-staging"
```

### Step 5.3: Create Compute Instance

```bash
# Get availability domain
AD=$(oci iam availability-domain list \
  --compartment-id "$COMPARTMENT_ID" \
  | jq -r '.data[0].name')

echo "Using Availability Domain: $AD"

# Create instance
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

echo "INSTANCE_OCID=$INSTANCE_OCID"
echo "export INSTANCE_OCID=\"$INSTANCE_OCID\"" >> ~/oracle-setup.env

echo "⏳ Waiting for instance to fully initialize..."
sleep 30
```

### Step 5.4: Get Public IP

```bash
# Get VNIC ID
VNIC_ID=$(oci compute instance list-vnics \
  --instance-id "$INSTANCE_OCID" \
  | jq -r '.data[0].id')

# Get public IP
PUBLIC_IP=$(oci network vnic get \
  --vnic-id "$VNIC_ID" \
  | jq -r '.data."public-ip"')

echo "✅ Instance created!"
echo "Public IP: $PUBLIC_IP"
echo "export PUBLIC_IP=\"$PUBLIC_IP\"" >> ~/oracle-setup.env

# Save to GitHub secret file for later
echo "ORACLE_CLOUD_HOST=$PUBLIC_IP" >> ~/github-secrets.txt
```

---

## Part 6: Configure Compute Instance (CLI/SSH)

### Step 6.1: SSH into Instance

```bash
# Test SSH connection
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP "echo 'SSH connection successful!'"
```

### Step 6.2: Install Docker

```bash
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP <<'ENDSSH'
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

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

echo "✅ Docker installed successfully"
ENDSSH
```

### Step 6.3: Upload Oracle Wallet

```bash
# Copy wallet to server
scp -i ~/.ssh/oracle-staging -r ~/oracle-wallet ubuntu@$PUBLIC_IP:~/
```

### Step 6.4: Create Application Directory

```bash
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP <<ENDSSH
# Create app directory
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
cat > ~/payment-service/docker-compose.yml <<'EOF'
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
EOF

echo "✅ Application directory configured"
ENDSSH
```

---

## Part 7: Set Up Oracle Container Registry (CLI)

### Step 7.1: Create Auth Token

```bash
# Get user OCID
USER_OCID=$(oci iam user list --all | jq -r '.data[0].id')

# Create auth token
AUTH_TOKEN=$(oci iam auth-token create \
  --user-id "$USER_OCID" \
  --description "github-actions-ocir" \
  | jq -r '.data.token')

echo "✅ Auth token created"
echo "IMPORTANT: Save this token - you won't see it again!"
echo "AUTH_TOKEN=$AUTH_TOKEN"

# Save to GitHub secrets file
echo "OCIR_AUTH_TOKEN=$AUTH_TOKEN" >> ~/github-secrets.txt
```

### Step 7.2: Get OCIR Details

```bash
# Map region to region key
case "$REGION" in
  "us-ashburn-1") OCIR_REGION="iad" ;;
  "us-phoenix-1") OCIR_REGION="phx" ;;
  "ca-toronto-1") OCIR_REGION="yyz" ;;
  "eu-frankfurt-1") OCIR_REGION="fra" ;;
  "uk-london-1") OCIR_REGION="lhr" ;;
  *) OCIR_REGION="${REGION%-*}" ;;
esac

# Get user email
USER_EMAIL=$(oci iam user list --all | jq -r '.data[0].email')

# OCIR username format
OCIR_USERNAME="${TENANCY_NAMESPACE}/oracleidentitycloudservice/${USER_EMAIL}"

echo "OCIR Region: $OCIR_REGION"
echo "OCIR Username: $OCIR_USERNAME"

# Save to env and GitHub secrets
echo "export OCIR_REGION=\"$OCIR_REGION\"" >> ~/oracle-setup.env
echo "export OCIR_USERNAME=\"$OCIR_USERNAME\"" >> ~/oracle-setup.env

cat >> ~/github-secrets.txt <<EOF
OCIR_REGION=$OCIR_REGION
OCIR_TENANCY_NAMESPACE=$TENANCY_NAMESPACE
OCIR_USERNAME=$OCIR_USERNAME
EOF
```

### Step 7.3: Test OCIR Login

```bash
# Login to OCIR locally
echo "$AUTH_TOKEN" | docker login ${OCIR_REGION}.ocir.io \
  -u "$OCIR_USERNAME" \
  --password-stdin

echo "✅ Successfully logged into Oracle Container Registry"
```

---

## Part 8: Summary and GitHub Secrets

### All Resources Created (CLI Only!)

```bash
echo "========================================="
echo "ORACLE CLOUD RESOURCES CREATED"
echo "========================================="
echo ""
echo "Autonomous Database:"
echo "  - Name: $DB_NAME"
echo "  - OCID: $DB_OCID"
echo "  - Admin User: ADMIN"
echo "  - App User: $DB_APP_USER"
echo ""
echo "Compute Instance:"
echo "  - Name: $INSTANCE_NAME"
echo "  - Shape: $INSTANCE_SHAPE (4 cores, 24GB RAM)"
echo "  - Public IP: $PUBLIC_IP"
echo "  - SSH: ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP"
echo ""
echo "Networking:"
echo "  - VCN: payment-staging-vcn"
echo "  - Subnet: payment-public-subnet"
echo "  - Ports: 22 (SSH), 8080 (gRPC), 8081 (HTTP)"
echo ""
echo "Container Registry:"
echo "  - Region: $OCIR_REGION"
echo "  - Repository: ${OCIR_REGION}.ocir.io/${TENANCY_NAMESPACE}/payment-service"
echo ""
echo "========================================="
```

### GitHub Secrets to Add

All secrets have been saved to `~/github-secrets.txt`. Add these to GitHub:

```bash
cat ~/github-secrets.txt

# Should show:
# ORACLE_CLOUD_HOST=xxx.xxx.xxx.xxx
# OCIR_AUTH_TOKEN=AbCdEf123...
# OCIR_REGION=iad
# OCIR_TENANCY_NAMESPACE=abcdef123456
# OCIR_USERNAME=abcdef123456/oracleidentitycloudservice/user@example.com
```

**Add SSH key to GitHub:**
```bash
# Display private key
cat ~/.ssh/oracle-staging

# Copy entire output (including BEGIN/END lines)
# Add as GitHub secret: ORACLE_CLOUD_SSH_KEY
```

**Add other secrets manually:**
```bash
EPX_MAC_STAGING=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
ORACLE_DB_PASSWORD=PaymentApp2025!
CRON_SECRET_STAGING=<from ~/oracle-setup.env>
```

---

## Part 9: Deploy Application

### Step 9.1: Build and Push First Image

```bash
# Clone your repository
cd ~/Documents/projects/payments

# Build image
docker build -t ${OCIR_REGION}.ocir.io/${TENANCY_NAMESPACE}/payment-service:latest .

# Push to OCIR
docker push ${OCIR_REGION}.ocir.io/${TENANCY_NAMESPACE}/payment-service:latest

echo "✅ Image pushed to Oracle Container Registry"
```

### Step 9.2: Deploy to Instance

```bash
# Update docker-compose.yml on server with actual values
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP <<ENDSSH
cd ~/payment-service

# Update .env with actual values
sed -i "s/\${DB_APP_USER}/${DB_APP_USER}/" .env
sed -i "s/\${DB_APP_PASSWORD}/${DB_APP_PASSWORD}/" .env
sed -i "s/\${DB_NAME}/${DB_NAME}/" .env
sed -i "s/\${EPX_MAC}/${EPX_MAC}/" .env
sed -i "s/\${PUBLIC_IP}/${PUBLIC_IP}/" .env
sed -i "s/\${CRON_SECRET}/${CRON_SECRET}/" .env

# Update docker-compose with registry details
sed -i "s/\${OCIR_REGION}/${OCIR_REGION}/" docker-compose.yml
sed -i "s/\${TENANCY_NAMESPACE}/${TENANCY_NAMESPACE}/" docker-compose.yml

# Login to OCIR on server
echo "${AUTH_TOKEN}" | docker login ${OCIR_REGION}.ocir.io \
  -u "${OCIR_USERNAME}" \
  --password-stdin

# Pull and run
docker-compose pull
docker-compose up -d

echo "✅ Application deployed"
ENDSSH
```

### Step 9.3: Verify Deployment

```bash
# Wait for startup
sleep 10

# Check health
curl http://$PUBLIC_IP:8081/cron/health

# Expected: {"status":"healthy","time":"2025-11-07T..."}
```

---

## Part 10: Automated Scripts

Save these scripts for easy management:

### cleanup.sh - Delete All Resources

```bash
cat > ~/cleanup-oracle.sh <<'EOF'
#!/bin/bash
source ~/oracle-setup.env

echo "⚠️  WARNING: This will delete ALL Oracle Cloud resources!"
read -p "Are you sure? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
  echo "Cancelled"
  exit 0
fi

# Delete compute instance
oci compute instance terminate --instance-id "$INSTANCE_OCID" --force

# Delete database
oci db autonomous-database delete --autonomous-database-id "$DB_OCID" --force

# Delete VCN (this also deletes subnets, security lists, etc.)
oci network vcn delete --vcn-id "$VCN_OCID" --force

echo "✅ All resources deleted"
EOF

chmod +x ~/cleanup-oracle.sh
```

### redeploy.sh - Quick Redeploy

```bash
cat > ~/redeploy-staging.sh <<'EOF'
#!/bin/bash
source ~/oracle-setup.env

echo "🚀 Redeploying to Oracle Cloud staging..."

# SSH and redeploy
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP <<'ENDSSH'
cd ~/payment-service
docker-compose pull
docker-compose down
docker-compose up -d
ENDSSH

echo "✅ Redeployment complete"
echo "📍 Staging URL: http://$PUBLIC_IP:8081"
EOF

chmod +x ~/redeploy-staging.sh
```

---

## CLI vs Web Console Comparison

| Task | CLI | Web Console |
|------|-----|-------------|
| Create account | ❌ Must use web | ✅ Required |
| Create database | ✅ `oci db autonomous-database create` | ✅ Click through wizard |
| Create compute | ✅ `oci compute instance launch` | ✅ Click through wizard |
| Set up networking | ✅ Multiple `oci network` commands | ✅ Auto-created or manual |
| Download wallet | ✅ `oci db autonomous-database generate-wallet` | ✅ Click "Download" button |
| Create auth token | ✅ `oci iam auth-token create` | ✅ Click "Generate Token" |
| Deploy application | ✅ SSH + docker-compose | ❌ Manual SSH |
| Monitor resources | ✅ `oci monitoring` commands | ✅ Visual dashboards |
| Delete resources | ✅ `oci <service> delete` | ✅ Click delete |

**CLI Advantages:**
- ✅ Scriptable and repeatable
- ✅ Version control infrastructure
- ✅ Faster (no clicking)
- ✅ Automation-friendly
- ✅ Can be run remotely
- ✅ Easy to share with team

**Web Console Advantages:**
- ✅ Visual confirmation
- ✅ Easier for first-time users
- ✅ Better for learning
- ✅ Nice dashboards

---

## Cheat Sheet

```bash
# Source your config anytime
source ~/oracle-setup.env

# SSH into instance
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP

# View logs
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP "docker logs payment-staging -f"

# Restart application
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP "cd ~/payment-service && docker-compose restart"

# Check database
sqlplus ${DB_APP_USER}/${DB_APP_PASSWORD}@${DB_NAME}_tp

# List all resources
oci compute instance list --compartment-id "$COMPARTMENT_ID"
oci db autonomous-database list --compartment-id "$COMPARTMENT_ID"
oci network vcn list --compartment-id "$COMPARTMENT_ID"

# Check Always Free usage
oci limits resource-availability get \
  --compartment-id "$COMPARTMENT_ID" \
  --service-name "compute" \
  --limit-name "standard-a1-core-count"
```

---

## Summary

You just set up your entire Oracle Cloud staging environment using **only the CLI**:

✅ Autonomous Database (20GB, Always Free)
✅ Ampere A1 Compute (4 cores, 24GB RAM, Always Free)
✅ VCN with public subnet
✅ Security lists configured
✅ Oracle Container Registry
✅ Application deployed
✅ **100% scripted and reproducible**

**Cost: $0/month forever**

**Time to deploy:** ~10 minutes via CLI vs ~30 minutes via web console

**Next deployment:** Just run `~/redeploy-staging.sh`
