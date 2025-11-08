# Set Up Staging Environment (Infrastructure as Code)

**Terraform-based setup** - No manual provisioning!

## TL;DR - 4 Commands

```bash
# 1. Generate Oracle Cloud API key (2 minutes)
./scripts/generate-oracle-api-key.sh

# 2. Upload public key to Oracle Cloud Console (2 minutes)
# Profile Icon → User Settings → API Keys → Add API Key

# 3. Add GitHub secrets (3 minutes)
# See: terraform/README.md for list of secrets

# 4. Run Terraform via GitHub Actions (10 minutes)
# Actions → Terraform Infrastructure → Run workflow → apply
```

Done! Your staging is live at `http://YOUR_IP:8081`

**For detailed instructions:** See [terraform/README.md](terraform/README.md) or [docs/TERRAFORM_ORACLE_SETUP.md](docs/TERRAFORM_ORACLE_SETUP.md)

---

## Why Terraform?

✅ **Infrastructure as Code** - Everything in version control
✅ **No manual setup** - Fully automated via GitHub Actions
✅ **Repeatable** - Destroy and recreate anytime
✅ **Token-based auth** - No interactive CLI configuration
✅ **State tracking** - Know exactly what's deployed

---

## Detailed Instructions

### Step 1: Create Oracle Cloud Account (5 minutes)

**Only if you don't have an account:**

1. Go to: https://www.oracle.com/cloud/free/
2. Click "Start for free"
3. Fill in email and details
4. Verify email
5. Add payment method (won't be charged)

---

### Step 2: Generate API Key (2 minutes)

```bash
cd ~/Documents/projects/payments

# Run the automated script
./scripts/generate-oracle-api-key.sh
```

**Output:**
- Creates: `~/.oci/oci_api_key.pem` (private key)
- Creates: `~/.oci/oci_api_key.pub` (public key)
- Displays: Fingerprint and public key to upload

**Next:** Upload the public key to Oracle Cloud.

---

### Step 3: Upload API Key to Oracle Cloud (2 minutes)

1. Go to: https://cloud.oracle.com
2. Click **Profile Icon** (top right) → **User Settings**
3. Click **API Keys** (left sidebar)
4. Click **Add API Key**
5. Select **Paste Public Key**
6. Paste the public key from Step 2
7. Click **Add**
8. **Verify fingerprint** matches the one displayed

**Also get these OCIDs while you're here:**
- User OCID: User Settings → Copy OCID
- Tenancy OCID: Profile Icon → Tenancy → Copy OCID
- Region: Top-right corner (e.g., "us-ashburn-1")

---

### Step 4: Add GitHub Secrets (3 minutes)

#### 4.1 Create Staging Environment

1. Go to: `https://github.com/YOUR_USERNAME/payment-service/settings/environments`
2. Click "New environment"
3. Name: `staging`
4. Click "Configure environment"
5. **Don't** check "Required reviewers"
6. Under "Deployment branches": Add rule for `develop`
7. Click "Save protection rules"

#### 4.2 Add Secrets

**Required Secrets for Terraform:**

| Secret Name | Value | Where to Get |
|-------------|-------|--------------|
| `OCI_USER_OCID` | `ocid1.user.oc1..xxx` | User Settings → OCID |
| `OCI_TENANCY_OCID` | `ocid1.tenancy.oc1..xxx` | Tenancy → OCID |
| `OCI_COMPARTMENT_OCID` | Same as tenancy | Tenancy → OCID |
| `OCI_REGION` | `us-ashburn-1` | Console top-right |
| `OCI_FINGERPRINT` | `aa:bb:cc:...` | From Step 2 output |
| `OCI_PRIVATE_KEY` | PEM format | `cat ~/.oci/oci_api_key.pem` |
| `ORACLE_DB_ADMIN_PASSWORD` | Your choice | Complex password |
| `ORACLE_DB_PASSWORD` | Your choice | Complex password |
| `EPX_MAC_STAGING` | `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` | From EPX docs |
| `CRON_SECRET_STAGING` | Random string | `openssl rand -base64 32` |
| `SSH_PUBLIC_KEY` | Your SSH key | `cat ~/.ssh/id_rsa.pub` |

#### How to add:
1. Click "Add environment secret"
2. Name: (from table above)
3. Value: (from table above)
4. Click "Add secret"
5. Repeat for all 11 secrets

---

### Step 5: Run Terraform (10 minutes)

**Via GitHub Actions:**

1. Go to **Actions** tab in your repository
2. Click **Terraform Infrastructure** workflow
3. Click **Run workflow** (top right)
4. Select action: `apply`
5. Click **Run workflow**

**What Terraform creates:**
- Autonomous Database (20GB, Always Free)
- Ampere A1 Compute (4 cores, 24GB RAM, Always Free)
- VCN Networking (Internet Gateway, Security Lists)
- Docker pre-installed
- Environment configured

**⏱️ Takes ~10 minutes**

**Monitor:** `https://github.com/YOUR_USERNAME/payment-service/actions`

---

### Step 5b: Add Terraform Output Secrets (1 minute)

After Terraform completes:

1. Click on the completed workflow run
2. Scroll to bottom
3. Copy **Terraform Outputs**
4. Add these additional secrets to **staging** environment:

```bash
ORACLE_CLOUD_HOST=xxx.xxx.xxx.xxx  # Public IP
ORACLE_CLOUD_SSH_KEY=... (if auto-generated)
```

---

### Step 6: Deploy Application (3 minutes)

```bash
git checkout develop
git add .
git commit -m "feat: Add Terraform infrastructure for Oracle Cloud"
git push origin develop
```

**GitHub Actions will:**
1. Run tests ✅
2. Build Docker image ✅
3. Push to Oracle Container Registry ✅
4. SSH deploy to your server ✅
5. Start application ✅

**Monitor:** `https://github.com/YOUR_USERNAME/payment-service/actions`

---

### Step 6: Verify It Works

```bash
# Get your IP from github-secrets.txt
PUBLIC_IP="xxx.xxx.xxx.xxx"

# Test health endpoint
curl http://$PUBLIC_IP:8081/cron/health

# Should return:
# {"status":"healthy","time":"..."}
```

**Success!** 🎉

Your staging environment is live at: `http://YOUR_IP:8081`

---

## What You Just Created

✅ **Autonomous Database**
- 20 GB storage
- PostgreSQL-compatible
- Automatic backups
- Free forever

✅ **Ampere A1 Server**
- 4 ARM cores
- 24 GB RAM
- Free forever

✅ **CI/CD Pipeline**
- Auto-deploy on push to `develop`
- Build → Test → Deploy
- GitHub Actions

✅ **Cost: $0/month**
- Always Free tier
- No expiration
- No credit card charges

---

## Daily Usage

### Deploy Changes

```bash
git checkout develop
# ... make changes ...
git add .
git commit -m "feat: My changes"
git push origin develop

# Automatically deploys to staging!
```

### View Logs

```bash
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP
docker logs payment-staging -f
```

### Restart Service

```bash
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP
cd ~/payment-service
docker-compose restart
```

---

## Troubleshooting

### "oci: command not found"

```bash
# Install OCI CLI
bash -c "$(curl -L https://raw.githubusercontent.com/oracle/oci-cli/master/scripts/install/install.sh)"

# Reload shell
source ~/.bashrc
```

### GitHub Actions: "Permission denied (publickey)"

Double-check `ORACLE_CLOUD_SSH_KEY` secret:
```bash
cat ~/.ssh/oracle-staging
# Copy ENTIRE output including BEGIN/END lines
```

### Can't access http://PUBLIC_IP:8081

```bash
# Check if container is running
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP
docker ps

# If not running, deploy manually
cd ~/payment-service
docker-compose up -d
```

### Need to start over?

```bash
# Delete all resources
source ~/oracle-staging-setup/oracle-config.env

oci compute instance terminate --instance-id "$INSTANCE_OCID" --force
oci db autonomous-database delete --autonomous-database-id "$DB_OCID" --force
oci network vcn delete --vcn-id "$VCN_OCID" --force

# Run setup again
./scripts/setup-oracle-staging.sh
```

---

## Documentation

- **Quick Start:** [ORACLE_QUICK_START.md](./ORACLE_QUICK_START.md)
- **Full CLI Guide:** [docs/ORACLE_CLI_SETUP.md](./docs/ORACLE_CLI_SETUP.md)
- **Full Web Guide:** [docs/ORACLE_CLOUD_STAGING.md](./docs/ORACLE_CLOUD_STAGING.md)
- **Branching Strategy:** [docs/BRANCHING_STRATEGY.md](./docs/BRANCHING_STRATEGY.md)

---

## Support

Having issues? Check:
1. Run verification: `./scripts/verify-oracle-setup.sh`
2. Check GitHub Actions logs
3. SSH into server and check Docker logs
4. Review configuration: `~/oracle-staging-setup/oracle-config.env`

---

## Summary

✅ Free staging environment on Oracle Cloud
✅ 4 cores, 24GB RAM, 20GB database
✅ Automated CI/CD deployment
✅ $0/month forever

**Total setup time:** ~20 minutes
**Next deployment:** Just `git push origin develop`
