# Oracle Cloud Staging - Quick Start Guide

Set up your entire staging environment in **15 minutes** using the automated script.

## Prerequisites

1. **Oracle Cloud Account** (free, takes 5 minutes)
   - Go to: https://www.oracle.com/cloud/free/
   - Sign up with email
   - Verify email and payment method (won't be charged)
   - Done!

2. **Your local machine** with:
   - Linux, macOS, or WSL
   - Git installed
   - curl installed

---

## Step 1: Run the Setup Script

```bash
cd ~/Documents/projects/payments

# Run the automated setup script
./scripts/setup-oracle-staging.sh
```

**What it does:**
- ✅ Installs Oracle CLI (if needed)
- ✅ Configures Oracle CLI
- ✅ Creates Autonomous Database (20GB, free forever)
- ✅ Creates Ampere A1 Instance (4 cores, 24GB RAM, free forever)
- ✅ Sets up networking (VCN, firewall, etc.)
- ✅ Installs Docker on the instance
- ✅ Generates all GitHub secrets
- ✅ Creates application configuration

**Time:** ~10-15 minutes

---

## Step 2: Add GitHub Secrets

After the script completes, it will create a file: `~/oracle-staging-setup/github-secrets.txt`

### 2.1 Go to GitHub

```
https://github.com/YOUR_USERNAME/payment-service/settings/secrets/actions
```

### 2.2 Create "staging" Environment

1. Click **Environments** (left sidebar)
2. Click **New environment**
3. Name: `staging`
4. Click **Configure environment**
5. **Don't** check "Required reviewers" (auto-deploy for staging)
6. Under "Deployment branches": Click **Add deployment branch rule**
   - Branch name pattern: `develop`
7. Click **Add rule**

### 2.3 Add Secrets to Staging Environment

Click **Add environment secret** for each secret from `github-secrets.txt`:

| Secret Name | Value | Where to Get |
|-------------|-------|--------------|
| `ORACLE_CLOUD_HOST` | `xxx.xxx.xxx.xxx` | From github-secrets.txt |
| `OCIR_REGION` | `iad` (or your region) | From github-secrets.txt |
| `OCIR_TENANCY_NAMESPACE` | `abcdef123456` | From github-secrets.txt |
| `OCIR_USERNAME` | `namespace/oracleidentitycloudservice/email` | From github-secrets.txt |
| `OCIR_AUTH_TOKEN` | Long token string | From github-secrets.txt |
| `EPX_MAC_STAGING` | `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` | From github-secrets.txt |
| `ORACLE_DB_PASSWORD` | Your DB password | From github-secrets.txt |
| `CRON_SECRET_STAGING` | Random base64 string | From github-secrets.txt |
| `ORACLE_CLOUD_SSH_KEY` | SSH private key | See below |

### 2.4 Add SSH Private Key

This is the most important one:

```bash
# Display your SSH private key
cat ~/.ssh/oracle-staging
```

Copy the **ENTIRE output**, including:
- `-----BEGIN OPENSSH PRIVATE KEY-----`
- All the key content
- `-----END OPENSSH PRIVATE KEY-----`

Add as secret: `ORACLE_CLOUD_SSH_KEY`

---

## Step 3: Deploy!

```bash
# Switch to develop branch
git checkout develop

# Add and commit your changes
git add .
git commit -m "feat: Set up Oracle Cloud staging environment"

# Push to trigger deployment
git push origin develop
```

**What happens:**
1. ✅ GitHub Actions starts automatically
2. ✅ Runs tests
3. ✅ Builds Docker image
4. ✅ Pushes to Oracle Container Registry
5. ✅ SSHs into your instance
6. ✅ Deploys application
7. ✅ Runs health check

**Monitor at:** `https://github.com/YOUR_USERNAME/payment-service/actions`

---

## Step 4: Verify It Works

### Check GitHub Actions

1. Go to **Actions** tab in GitHub
2. Click on the latest workflow run
3. Watch it complete (~3-5 minutes)
4. All steps should show ✅

### Test Your Staging Server

```bash
# Get your public IP from the setup output
PUBLIC_IP="xxx.xxx.xxx.xxx"  # From github-secrets.txt

# Test health endpoint
curl http://$PUBLIC_IP:8081/cron/health

# Expected response:
# {"status":"healthy","time":"2025-11-07T..."}
```

### SSH into Your Instance (Optional)

```bash
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP

# View logs
docker logs payment-staging -f

# Check status
docker ps
```

---

## Troubleshooting

### Script Fails: "OCI CLI not configured"

**Solution:**
```bash
oci setup config
```

Follow the prompts. You'll need:
- User OCID: Profile Icon → User Settings → OCID
- Tenancy OCID: Profile Icon → Tenancy → OCID
- Region: Your home region (e.g., `us-ashburn-1`)

Then add the API key:
```bash
oci iam user api-key upload \
  --user-id $(oci iam user list --all | jq -r '.data[0].id') \
  --key-file ~/.oci/oci_api_key_public.pem
```

### GitHub Actions Fails: "Permission denied (publickey)"

**Solution:** Double-check the SSH private key secret

```bash
# Display the key again
cat ~/.ssh/oracle-staging

# Make sure you copied:
# - The entire key
# - Including BEGIN and END lines
# - No extra spaces or line breaks
```

### GitHub Actions Fails: "Login to OCIR failed"

**Solution:** Verify OCIR credentials

Check these secrets are correct:
- `OCIR_USERNAME` - Format: `namespace/oracleidentitycloudservice/email@example.com`
- `OCIR_AUTH_TOKEN` - From `~/oracle-staging-setup/ocir-auth-token.txt`
- `OCIR_REGION` - Should match your Oracle region

### Can't Access http://PUBLIC_IP:8081

**Solution:** Check firewall

```bash
# SSH into instance
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP

# Check if container is running
docker ps

# Check firewall
sudo ufw status

# If ports not open, add them:
sudo ufw allow 8081/tcp
```

### Health Check Returns 404

**Solution:** Application may not be deployed yet

```bash
# SSH into instance
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP

# Check logs
docker logs payment-staging

# Manually deploy
cd ~/payment-service
docker-compose pull
docker-compose up -d
```

---

## Daily Usage

### Deploy New Changes

```bash
# Work on develop branch
git checkout develop

# Make your changes
# ... edit code ...

# Commit and push
git add .
git commit -m "feat: My new feature"
git push origin develop

# GitHub Actions automatically deploys!
```

### View Logs

```bash
# SSH into instance
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP

# View logs
docker logs payment-staging -f

# Last 100 lines
docker logs payment-staging --tail 100
```

### Restart Application

```bash
# SSH into instance
ssh -i ~/.ssh/oracle-staging ubuntu@$PUBLIC_IP

# Restart
cd ~/payment-service
docker-compose restart

# Or redeploy latest
docker-compose pull
docker-compose down
docker-compose up -d
```

### Check Database

```bash
# On your local machine
export TNS_ADMIN=~/oracle-wallet

# Connect to database
sqlplus payment_service/YOUR_PASSWORD@paymentdb_tp

# Run queries
SQL> SELECT * FROM payments LIMIT 10;
SQL> EXIT;
```

---

## Resource Management

### View All Resources

```bash
# Load your configuration
source ~/oracle-staging-setup/oracle-config.env

# List instances
oci compute instance list --compartment-id "$COMPARTMENT_ID"

# List databases
oci db autonomous-database list --compartment-id "$COMPARTMENT_ID"

# Check Always Free usage
oci limits resource-availability get \
  --compartment-id "$COMPARTMENT_ID" \
  --service-name "compute" \
  --limit-name "standard-a1-core-count"
```

### Stop Instance (to save resources - optional)

```bash
# Stop instance
oci compute instance action \
  --instance-id "$INSTANCE_OCID" \
  --action STOP

# Start instance
oci compute instance action \
  --instance-id "$INSTANCE_OCID" \
  --action START
```

**Note:** Always Free resources are free even when running 24/7. No need to stop unless you're not using them.

### Delete Everything

```bash
# DANGER: This deletes ALL staging resources!

# Delete instance
oci compute instance terminate --instance-id "$INSTANCE_OCID" --force

# Delete database
oci db autonomous-database delete --autonomous-database-id "$DB_OCID" --force

# Delete VCN (deletes subnets, security lists, etc.)
oci network vcn delete --vcn-id "$VCN_OCID" --force
```

---

## URLs and Credentials

### Staging URLs

- **Health Check:** `http://YOUR_IP:8081/cron/health`
- **Stats:** `http://YOUR_IP:8081/cron/stats`
- **gRPC Server:** `YOUR_IP:8080`

### Database

- **Connection:** `paymentdb_tp`
- **User:** `payment_service`
- **Password:** From setup script
- **TNS_ADMIN:** `~/oracle-wallet`

### Oracle Cloud Console

- **Console:** https://cloud.oracle.com
- **Compute Instances:** Hamburger Menu → Compute → Instances
- **Databases:** Hamburger Menu → Oracle Database → Autonomous Database

---

## Cost

**Total Monthly Cost: $0.00**

Your Always Free tier includes:
- ✅ 2 Autonomous Databases (20GB each)
- ✅ 4 Ampere A1 cores + 24GB RAM (total across 2 VMs)
- ✅ 200GB block storage
- ✅ 10GB object storage
- ✅ Networking

**You're using:**
- 1 Autonomous Database (20GB) - **$0**
- 1 Ampere A1 VM (4 cores, 24GB) - **$0**

**Forever free. No expiration. No credit card charges.**

---

## Next Steps

Once staging is working:

1. **Set up production** (when ready)
   - Follow: [GCP_PRODUCTION_SETUP.md](./docs/GCP_PRODUCTION_SETUP.md)
   - Uses Google Cloud Run
   - Triggered by merging to `main` branch

2. **Test payment flows**
   - Use EPX sandbox credentials
   - Test transactions
   - Monitor in staging

3. **Monitor and iterate**
   - Check logs regularly
   - Fix bugs in `develop` branch
   - Auto-deploys to staging

4. **Deploy to production**
   - When ready, create PR: `develop` → `main`
   - Get approval
   - Auto-deploys to Google Cloud Run production

---

## Support

- **Oracle Cloud Support:** https://docs.oracle.com/en-us/iaas/
- **OCI CLI Docs:** https://docs.oracle.com/en-us/iaas/tools/oci-cli/
- **Always Free FAQs:** https://www.oracle.com/cloud/free/faq.html

---

## Summary

You've set up:

✅ **Free staging environment** on Oracle Cloud
✅ **Autonomous Database** (20GB, PostgreSQL-compatible)
✅ **Ampere Compute** (4 cores, 24GB RAM)
✅ **CI/CD** via GitHub Actions
✅ **Auto-deployment** on every push to `develop`

**Total time:** 15 minutes
**Total cost:** $0/month forever
**Next deployment:** Just `git push origin develop`

Enjoy your free, production-grade staging environment! 🎉
