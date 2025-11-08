# Oracle Cloud Free Tier Staging Deployment

Deploy your entire staging environment on Oracle Cloud for **$0/month forever** using Always Free tier.

## Architecture Overview

```
GitHub Actions (develop branch)
    ↓
Run Tests & Build
    ↓
Build Docker Image
    ↓
Push to Oracle Container Registry (OCIR)
    ↓
Deploy to Oracle Compute Instance
    ↓
Connect to Autonomous Database
    ↓
Staging Live (Always Free)
```

## Oracle Always Free Resources

You get these resources **forever** with no credit card expiration:

**Compute:**
- Option A: 2 AMD VMs (1/8 OCPU, 1GB RAM each)
- Option B: 4 Arm Ampere A1 cores + 24GB RAM total ⭐ **Recommended**

**Database:**
- 2 Autonomous Databases (20GB each)

**Storage:**
- 200 GB Block Volumes
- 10 GB Object Storage

**Networking:**
- Virtual Cloud Network (VCN)
- Load Balancer (10 Mbps)

**Our Setup:**
- 1 Ampere A1 VM (4 cores, 24GB RAM) - Application server
- 1 Autonomous Database (20GB) - PostgreSQL database
- Cost: **$0 forever**

---

## Part 1: Oracle Cloud Account Setup

### Step 1.1: Create Account

1. Go to: https://www.oracle.com/cloud/free/
2. Click **Start for free**
3. Fill in account details:
   - Email: Your email
   - Country/Territory: Select your country
   - **Cloud Account Name**: `payment-service` (this becomes part of your tenancy)
4. **Home Region**: Choose closest (e.g., US East, Canada, EU Frankfurt)
   - ⚠️ **Cannot change later** - choose wisely
5. Verify email
6. Complete phone verification
7. Add payment method (for identity verification only - won't be charged for Always Free)

### Step 1.2: Access Console

After signup:
1. Go to: https://cloud.oracle.com
2. **Cloud Account Name**: `payment-service` (or what you chose)
3. Click **Next**
4. Sign in with your credentials
5. You'll see the Oracle Cloud Console dashboard

---

## Part 2: Set Up Autonomous Database

### Step 2.1: Create Autonomous Database

1. Click hamburger menu (☰) → **Oracle Database** → **Autonomous Database**
2. Click **Create Autonomous Database**

**Configuration:**
- **Compartment**: root
- **Display name**: `payment-staging-db`
- **Database name**: `paymentdb`
- **Workload type**: Transaction Processing
- **Deployment type**: Serverless
- ✅ **Always Free**: CHECK THIS BOX
- **Database version**: 23ai (or 19c)
- **OCPU**: 1 (fixed for Always Free)
- **Storage**: 20 GB (fixed for Always Free)

**Admin Credentials:**
- **Username**: ADMIN
- **Password**: Create strong password
  - Example: `PaymentDB2025!`
  - **SAVE THIS PASSWORD**

**Network Access:**
- Select: **Secure access from everywhere**

**License**: License Included

3. Click **Create Autonomous Database**
4. Wait 2-3 minutes for provisioning

### Step 2.2: Download Wallet

1. Click on database name once **Available**
2. Click **Database connection**
3. Click **Download wallet**
4. **Wallet password**: Create password (can match DB password)
5. Save `Wallet_paymentdb.zip`

### Step 2.3: Create Application User

```bash
# Install Oracle SQL*Plus (if needed)
# Download from: https://www.oracle.com/database/technologies/instant-client/downloads.html

# Extract wallet
mkdir -p ~/oracle-wallet
cd ~/oracle-wallet
unzip ~/Downloads/Wallet_paymentdb.zip

# Set environment
export TNS_ADMIN=~/oracle-wallet

# Connect as ADMIN
sqlplus ADMIN@paymentdb_tp
```

```sql
-- Create application user
CREATE USER payment_service IDENTIFIED BY "PaymentApp2025!";

-- Grant permissions
GRANT CONNECT, RESOURCE TO payment_service;
GRANT UNLIMITED TABLESPACE TO payment_service;
GRANT CREATE SESSION, CREATE TABLE, CREATE SEQUENCE, CREATE TRIGGER TO payment_service;

-- Verify
SELECT username FROM all_users WHERE username = 'PAYMENT_SERVICE';

EXIT;
```

**Save credentials:**
- Username: `payment_service`
- Password: `PaymentApp2025!`

---

## Part 3: Set Up Compute Instance

### Step 3.1: Create Compute Instance

1. Click hamburger menu (☰) → **Compute** → **Instances**
2. Click **Create Instance**

**Name and Compartment:**
- **Name**: `payment-staging-server`
- **Compartment**: root

**Image and Shape:**
- **Image**: Canonical Ubuntu 22.04 (or Oracle Linux 8)
- Click **Change Shape**
  - **Shape series**: Ampere
  - **Shape name**: VM.Standard.A1.Flex
  - ✅ **Always Free-eligible** - Verify this shows
  - **Number of OCPUs**: 4 (max for Always Free)
  - **Amount of memory (GB)**: 24 (max for Always Free)
  - Click **Select shape**

**Networking:**
- **Virtual cloud network**: Create new VCN
  - **Name**: `payment-staging-vcn`
  - ✅ **Create in compartment**: root
  - ✅ **Create public subnet**
- **Subnet**: Public Subnet (auto-created)
- ✅ **Assign a public IPv4 address**: CHECK THIS

**Add SSH keys:**
- **Generate SSH key pair**: Click this
- **Save private key**: Download and save (e.g., `ssh-key-payment-staging.key`)
- **Save public key**: Download and save (optional, for backup)

**Boot volume:**
- Leave default (47 GB boot volume is free)

3. Click **Create**
4. Wait 2-3 minutes for provisioning

### Step 3.2: Note Instance Details

Once instance is **Running**:
- **Public IP**: Note this (e.g., `xxx.xxx.xxx.xxx`)
- **Private IP**: Note this (e.g., `10.0.0.x`)

---

## Part 4: Configure Networking

### Step 4.1: Update Security List

1. Click hamburger menu → **Networking** → **Virtual Cloud Networks**
2. Click **payment-staging-vcn**
3. Click **Security Lists** (left menu)
4. Click **Default Security List for payment-staging-vcn**
5. Click **Add Ingress Rules**

**Rule 1: HTTP (8081 - cron endpoints)**
- **Source CIDR**: `0.0.0.0/0`
- **IP Protocol**: TCP
- **Destination Port Range**: `8081`
- **Description**: HTTP server for cron endpoints

**Rule 2: gRPC (8080)**
- **Source CIDR**: `0.0.0.0/0`
- **IP Protocol**: TCP
- **Destination Port Range**: `8080`
- **Description**: gRPC server

6. Click **Add Ingress Rules**

### Step 4.2: Configure Firewall on Instance

```bash
# SSH into instance
chmod 400 ~/Downloads/ssh-key-payment-staging.key
ssh -i ~/Downloads/ssh-key-payment-staging.key ubuntu@YOUR_PUBLIC_IP

# Update system
sudo apt update && sudo apt upgrade -y

# Configure firewall (Ubuntu)
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 8080/tcp  # gRPC
sudo ufw allow 8081/tcp  # HTTP
sudo ufw enable
sudo ufw status

# Or for Oracle Linux:
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=8081/tcp
sudo firewall-cmd --reload
```

---

## Part 5: Install Docker on Compute Instance

### Step 5.1: Install Docker

**Ubuntu:**
```bash
# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Verify
docker --version
docker run hello-world
```

**Oracle Linux:**
```bash
sudo dnf install -y dnf-utils
sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker $USER
newgrp docker
```

### Step 5.2: Install Docker Compose

```bash
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
docker-compose --version
```

---

## Part 6: Deploy Application

### Step 6.1: Upload Oracle Wallet to Server

From your local machine:

```bash
# SCP wallet to server
scp -i ~/Downloads/ssh-key-payment-staging.key \
    ~/oracle-wallet/* \
    ubuntu@YOUR_PUBLIC_IP:/home/ubuntu/oracle-wallet/
```

Or create on server:

```bash
# On server
mkdir -p ~/oracle-wallet
cd ~/oracle-wallet

# Upload files manually or use FTP client
# You'll need: tnsnames.ora, sqlnet.ora, cwallet.sso, ewallet.p12
```

### Step 6.2: Create Environment File

```bash
# On server
mkdir -p ~/payment-service
cd ~/payment-service

cat > .env <<'EOF'
# Server Configuration
PORT=8080
HTTP_PORT=8081
ENVIRONMENT=staging

# Database (Oracle Autonomous)
DATABASE_URL=oracle://payment_service:PaymentApp2025!@paymentdb_tp
TNS_ADMIN=/app/oracle-wallet
DB_SSL_MODE=require

# EPX Payment Gateway (Sandbox)
EPX_SERVER_POST_URL=https://secure.epxuap.com
EPX_TIMEOUT=30
EPX_CUST_NBR=9001
EPX_MERCH_NBR=900300
EPX_DBA_NBR=2
EPX_TERMINAL_NBR=77
EPX_MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y

# North Merchant Reporting API
NORTH_MERCHANT_REPORTING_URL=https://api.north.com
NORTH_TIMEOUT=30

# Application
CALLBACK_BASE_URL=http://YOUR_PUBLIC_IP:8081
CRON_SECRET=$(openssl rand -base64 32)

# Logging
LOG_LEVEL=debug
EOF

# Update with actual public IP
PUBLIC_IP=$(curl -s ifconfig.me)
sed -i "s/YOUR_PUBLIC_IP/$PUBLIC_IP/" .env
```

### Step 6.3: Create docker-compose.yml

```bash
cat > docker-compose.yml <<'EOF'
version: '3.8'

services:
  payment-service:
    image: payment-service:latest
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
```

### Step 6.4: Build and Run (Manual First Deployment)

For the first deployment, build locally or on the server:

```bash
# Clone repository
git clone https://github.com/YOUR_USERNAME/payment-service.git
cd payment-service

# Build Docker image
docker build -t payment-service:latest .

# Run migrations
docker run --rm \
  -v /home/ubuntu/oracle-wallet:/app/oracle-wallet \
  -e TNS_ADMIN=/app/oracle-wallet \
  -e DATABASE_URL="oracle://payment_service:PaymentApp2025!@paymentdb_tp" \
  payment-service:latest \
  /app/goose -dir /app/migrations oracle "payment_service/PaymentApp2025!@paymentdb_tp" up

# Start service
cd ~/payment-service
docker-compose up -d

# Check logs
docker-compose logs -f
```

### Step 6.5: Verify Deployment

```bash
# Health check
curl http://localhost:8081/cron/health

# From your local machine
curl http://YOUR_PUBLIC_IP:8081/cron/health
```

Expected response:
```json
{
  "status": "healthy",
  "time": "2025-11-07T20:00:00Z"
}
```

---

## Part 7: Set Up Oracle Container Registry (OCIR)

For automated deployments, push images to Oracle's container registry.

### Step 7.1: Create Auth Token

1. Click profile icon (top right) → **User Settings**
2. Click **Auth Tokens** (left menu)
3. Click **Generate Token**
4. **Description**: `github-actions-ocir`
5. Click **Generate Token**
6. **Copy the token** - you won't see it again
   - Example: `AbCdEf123456...`

### Step 7.2: Get OCIR Details

**Registry hostname format:**
```
<region-key>.ocir.io
```

**Region keys:**
- US East (Ashburn): `iad`
- US West (Phoenix): `phx`
- Canada (Toronto): `yyz`
- EU (Frankfurt): `fra`
- EU (Amsterdam): `ams`

Example: `iad.ocir.io`

**Tenancy namespace:**
1. Click profile icon → **Tenancy: <your-tenancy-name>**
2. Note **Object Storage Namespace** (e.g., `abcdef123456`)

**Full image path format:**
```
<region-key>.ocir.io/<tenancy-namespace>/<repo-name>:<tag>
```

Example:
```
iad.ocir.io/abcdef123456/payment-service:latest
```

### Step 7.3: Login to OCIR from Compute Instance

```bash
# On your compute instance
docker login iad.ocir.io
# Username: <tenancy-namespace>/oracleidentitycloudservice/<your-email>
# Password: <auth-token>

# Example:
# Username: abcdef123456/oracleidentitycloudservice/user@example.com
# Password: AbCdEf123456...
```

---

## Part 8: GitHub Actions CI/CD

### Step 8.1: Add GitHub Secrets

Go to GitHub → Settings → Secrets and variables → Actions → New repository secret

Add these secrets:

| Secret Name | Value | How to Get |
|-------------|-------|------------|
| `ORACLE_CLOUD_HOST` | Public IP of compute instance | Oracle Console → Compute → Instances |
| `ORACLE_CLOUD_SSH_KEY` | Private SSH key content | Content of `ssh-key-payment-staging.key` |
| `OCIR_REGION` | Region key (e.g., `iad`) | Your home region |
| `OCIR_TENANCY_NAMESPACE` | Tenancy namespace | Profile → Tenancy → Object Storage Namespace |
| `OCIR_USERNAME` | OCIR username | `<namespace>/oracleidentitycloudservice/<email>` |
| `OCIR_AUTH_TOKEN` | Auth token | From Step 7.1 |
| `EPX_MAC_STAGING` | `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` | Sandbox MAC |
| `ORACLE_DB_PASSWORD` | `PaymentApp2025!` | Database password |
| `CRON_SECRET_STAGING` | Generate: `openssl rand -base64 32` | Random secret |

### Step 8.2: Create GitHub Environment

1. Go to Settings → Environments → New environment
2. Name: `staging`
3. Protection rules:
   - ⬜ Required reviewers: UNCHECKED (auto-deploy for staging)
   - Deployment branches: `develop` only
4. Click **Configure environment**

Add environment secrets (same as above repository secrets).

### Step 8.3: Update CI/CD Workflow

Update `.github/workflows/ci-cd.yml`:

```yaml
name: CI/CD

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main, develop]

jobs:
  test:
    name: Run Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Run tests
        run: go test -v ./...

  build:
    name: Build Docker Image
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build Docker image
        run: docker build -t payment-service:${{ github.sha }} .

  # ===================================
  # STAGING DEPLOYMENT (Oracle Cloud)
  # ===================================
  deploy-staging:
    name: Deploy to Oracle Cloud Staging
    needs: build
    if: github.ref == 'refs/heads/develop' && github.event_name == 'push'
    runs-on: ubuntu-latest
    environment: staging
    steps:
      - uses: actions/checkout@v4

      - name: Login to Oracle Container Registry
        run: |
          echo "${{ secrets.OCIR_AUTH_TOKEN }}" | docker login ${{ secrets.OCIR_REGION }}.ocir.io \
            -u "${{ secrets.OCIR_USERNAME }}" \
            --password-stdin

      - name: Build and Push Docker Image
        run: |
          IMAGE_TAG="${{ secrets.OCIR_REGION }}.ocir.io/${{ secrets.OCIR_TENANCY_NAMESPACE }}/payment-service:${{ github.sha }}"
          IMAGE_LATEST="${{ secrets.OCIR_REGION }}.ocir.io/${{ secrets.OCIR_TENANCY_NAMESPACE }}/payment-service:latest"

          docker build -t ${IMAGE_TAG} -t ${IMAGE_LATEST} .
          docker push ${IMAGE_TAG}
          docker push ${IMAGE_LATEST}

      - name: Deploy to Oracle Compute Instance
        uses: appleboy/ssh-action@master
        with:
          host: ${{ secrets.ORACLE_CLOUD_HOST }}
          username: ubuntu
          key: ${{ secrets.ORACLE_CLOUD_SSH_KEY }}
          script: |
            # Login to OCIR
            echo "${{ secrets.OCIR_AUTH_TOKEN }}" | docker login ${{ secrets.OCIR_REGION }}.ocir.io \
              -u "${{ secrets.OCIR_USERNAME }}" \
              --password-stdin

            # Pull latest image
            docker pull ${{ secrets.OCIR_REGION }}.ocir.io/${{ secrets.OCIR_TENANCY_NAMESPACE }}/payment-service:latest

            # Stop existing container
            docker-compose -f ~/payment-service/docker-compose.yml down || true

            # Start new container
            cd ~/payment-service
            docker-compose up -d

            # Wait for health check
            sleep 10
            curl -f http://localhost:8081/cron/health || exit 1

      - name: Verify Deployment
        run: |
          sleep 5
          curl -f http://${{ secrets.ORACLE_CLOUD_HOST }}:8081/cron/health

  # ===================================
  # PRODUCTION DEPLOYMENT (Google Cloud Run)
  # ===================================
  deploy-production:
    name: Deploy to Google Cloud Run
    needs: build
    if: github.ref == 'refs/heads/main' && github.event_name == 'push'
    runs-on: ubuntu-latest
    environment: production
    # ... (existing GCP deployment)
```

---

## Part 9: Monitoring and Management

### Step 9.1: View Application Logs

```bash
# SSH into server
ssh -i ~/Downloads/ssh-key-payment-staging.key ubuntu@YOUR_PUBLIC_IP

# View logs
docker-compose logs -f

# View last 100 lines
docker-compose logs --tail=100

# Filter by service
docker logs payment-staging -f
```

### Step 9.2: Restart Service

```bash
# SSH into server
ssh -i ~/Downloads/ssh-key-payment-staging.key ubuntu@YOUR_PUBLIC_IP

# Restart
cd ~/payment-service
docker-compose restart

# Or pull latest and restart
docker-compose pull
docker-compose up -d
```

### Step 9.3: Database Monitoring

```sql
-- Connect to database
sqlplus payment_service@paymentdb_tp

-- Check table sizes
SELECT table_name, num_rows
FROM user_tables
ORDER BY num_rows DESC;

-- Check active sessions
SELECT username, status, machine
FROM v$session
WHERE username = 'PAYMENT_SERVICE';

EXIT;
```

### Step 9.4: Resource Monitoring

```bash
# On compute instance

# Check disk usage
df -h

# Check memory
free -h

# Check CPU
top

# Check Docker stats
docker stats

# Check container health
docker ps
```

---

## Part 10: Scaling and Optimization

### 10.1: Horizontal Scaling (Future)

Oracle Free Tier allows 2 compute instances. For more traffic:

1. Create second compute instance (same specs)
2. Set up load balancer (10 Mbps free tier)
3. Deploy to both instances
4. Configure round-robin or least connections

### 10.2: Vertical Scaling

Already at max for Always Free:
- 4 Ampere A1 cores
- 24 GB RAM
- 20 GB database storage

To scale beyond, upgrade to paid tier.

### 10.3: Performance Optimization

**Application:**
- Enable HTTP/2
- Use connection pooling (already configured)
- Add caching layer (Redis - can run in container)

**Database:**
- Create indexes on frequently queried columns
- Monitor slow queries
- Use prepared statements (already implemented)

---

## Part 11: Backup and Disaster Recovery

### 11.1: Database Backups

Oracle Autonomous Database automatically backs up daily:
- Retention: 60 days
- No configuration needed

**Manual backup:**
```bash
# Export data
sqlplus payment_service@paymentdb_tp

SQL> CREATE DIRECTORY backup_dir AS '/tmp/backup';
SQL> GRANT READ, WRITE ON DIRECTORY backup_dir TO payment_service;

expdp payment_service/PaymentApp2025!@paymentdb_tp \
  DIRECTORY=backup_dir \
  DUMPFILE=payment_service_backup.dmp \
  LOGFILE=payment_service_backup.log
```

### 11.2: Application Backups

```bash
# Backup on compute instance

# Create backup directory
mkdir -p ~/backups

# Backup environment and config
tar -czf ~/backups/payment-service-$(date +%Y%m%d).tar.gz \
  ~/payment-service/.env \
  ~/payment-service/docker-compose.yml \
  ~/oracle-wallet

# Schedule automatic backups (cron)
crontab -e

# Add:
# Daily backup at 2 AM
0 2 * * * tar -czf ~/backups/payment-service-$(date +\%Y\%m\%d).tar.gz ~/payment-service ~/oracle-wallet
```

### 11.3: Disaster Recovery Plan

**If compute instance fails:**
1. Create new compute instance (same specs)
2. Restore from backup
3. Update DNS/IP in GitHub secrets
4. Redeploy via GitHub Actions

**If database fails:**
1. Restore from automatic backup (Oracle handles this)
2. Or create new database and import data

---

## Cost Management

**Always Free Limits:**
- 2 Compute VMs (Ampere A1: 4 cores, 24GB total)
- 2 Autonomous Databases (20GB each)
- 200 GB Block Volumes
- 10 GB Object Storage

**Monitoring:**
1. Oracle Cloud Console → **Cost Analysis**
2. Check "Always Free" resource status
3. Set up budget alerts

**Monthly Cost Estimate:**
- Compute: $0
- Database: $0
- Networking: $0
- **Total: $0**

**Warning:** Accidentally upgrading resources will incur charges. Always verify "Always Free" label.

---

## Troubleshooting

### Issue: SSH connection refused

**Solution:**
```bash
# Check security list allows port 22
# Check instance is running
# Verify SSH key permissions
chmod 400 ~/Downloads/ssh-key-payment-staging.key
```

### Issue: Docker container won't start

**Solution:**
```bash
# Check logs
docker logs payment-staging

# Common issues:
# 1. Oracle wallet missing
ls -la /home/ubuntu/oracle-wallet

# 2. Environment variables not set
docker-compose config

# 3. Port already in use
sudo netstat -tulpn | grep 8080
```

### Issue: Cannot connect to database

**Solution:**
```bash
# Verify TNS_ADMIN
echo $TNS_ADMIN

# Test connection
sqlplus payment_service@paymentdb_tp

# Check wallet files
ls -la ~/oracle-wallet

# Verify network access in Oracle Console
```

### Issue: Out of memory

**Solution:**
```bash
# Check memory usage
free -h

# Check docker stats
docker stats

# Adjust docker-compose memory limits if needed
```

---

## Summary

You now have:

✅ **Oracle Cloud staging environment**
- Ampere A1 compute (4 cores, 24GB RAM)
- Autonomous Database (20GB)
- Automated CI/CD from GitHub
- $0/month forever

**Next Steps:**
1. Push to `develop` branch
2. GitHub Actions automatically deploys
3. Access at `http://YOUR_PUBLIC_IP:8081/cron/health`

**Staging URL:** `http://YOUR_PUBLIC_IP:8081`

**Resources:**
- Oracle Free Tier: https://www.oracle.com/cloud/free/
- Oracle Container Registry: https://docs.oracle.com/en-us/iaas/Content/Registry/home.htm
- Autonomous Database: https://docs.oracle.com/en/cloud/paas/autonomous-database/
