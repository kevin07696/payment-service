# Implementation Checklist - Oracle Cloud Staging

Follow these steps to set up your Oracle Cloud staging environment.

## ✅ Checklist

### Step 1: Generate Oracle Cloud API Key (2 minutes)

Run the automated script:

```bash
cd ~/Documents/projects/payments
./scripts/generate-oracle-api-key.sh
```

**Output will show:**
- ✅ Private key location: `~/.oci/oci_api_key.pem`
- ✅ Public key location: `~/.oci/oci_api_key.pub`
- ✅ Fingerprint: `aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99`
- ✅ Public key content (to upload)

**Save this information** - you'll need it for GitHub secrets.

---

### Step 2: Upload API Key to Oracle Cloud (2 minutes)

1. Go to: https://cloud.oracle.com
2. Click **Profile Icon** (top right) → **User Settings**
3. Click **API Keys** (left sidebar)
4. Click **Add API Key**
5. Select **Paste Public Key**
6. Paste the public key from Step 1 output
7. Click **Add**
8. **Verify fingerprint matches** the one from Step 1

---

### Step 3: Get Oracle Cloud OCIDs (2 minutes)

While in Oracle Cloud Console:

**User OCID:**
- Profile Icon → User Settings → Copy OCID
- Format: `ocid1.user.oc1..aaaaaaaaxxxxxxxx`

**Tenancy OCID:**
- Profile Icon → Tenancy: [your-tenancy-name] → Copy OCID
- Format: `ocid1.tenancy.oc1..aaaaaaaaxxxxxxxx`

**Compartment OCID:**
- Usually same as Tenancy OCID (for root compartment)

**Region:**
- Top-right corner shows your region
- Examples: `us-ashburn-1`, `us-phoenix-1`, `ap-tokyo-1`

**Save these values** - you'll need them for GitHub secrets.

---

### Step 4: Generate Database Passwords (1 minute)

Run these commands to generate secure passwords:

```bash
# Admin password
openssl rand -base64 24
# Example output: 5xK2mP9qR3nT8vL1wY4aZ6bC

# App password
openssl rand -base64 24
# Example output: 7dF5gH8jK3lN6mQ9rT2uV1wX
```

**Save these passwords** - you'll need them for:
- GitHub secret: `ORACLE_DB_ADMIN_PASSWORD`
- GitHub secret: `ORACLE_DB_PASSWORD`

---

### Step 5: Generate Application Secrets (1 minute)

```bash
# Cron secret
openssl rand -base64 32
# Example output: 3vB5nM8qT1xY4aC7eF9hK2lO6pR8tU0wZ3

# EPX MAC (already provided for sandbox)
echo "2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y"
```

**Save these** - you'll need them for GitHub secrets.

---

### Step 6: Get SSH Public Key (1 minute)

If you have an existing SSH key:

```bash
cat ~/.ssh/id_rsa.pub
```

Or generate a new one:

```bash
ssh-keygen -t rsa -b 4096 -f ~/.ssh/oracle-staging-key -N ""
cat ~/.ssh/oracle-staging-key.pub
```

**Save the public key** - you'll need it for GitHub secret: `SSH_PUBLIC_KEY`

---

### Step 7: Get Oracle Container Registry Details (2 minutes)

```bash
# Get tenancy namespace
oci os ns get
# Example output: axxxxxxxxxxx

# Or via web console:
# Profile Icon → Tenancy → Object Storage Namespace
```

**Save the namespace** - you'll need it for `OCIR_TENANCY_NAMESPACE`

**Generate Auth Token:**
1. Profile Icon → User Settings
2. **Auth Tokens** (left sidebar)
3. Click **Generate Token**
4. Description: "GitHub Actions OCIR"
5. Click **Generate Token**
6. **Copy the token immediately** (you can't see it again!)
7. Save for GitHub secret: `OCIR_AUTH_TOKEN`

**OCIR Username format:**
```
<namespace>/oracleidentitycloudservice/<your-email>
```
Example: `axxxxxxxxxxx/oracleidentitycloudservice/you@example.com`

---

### Step 8: Add Secrets to GitHub (5 minutes)

Go to: `https://github.com/YOUR_USERNAME/payment-service/settings/environments`

#### Create Staging Environment

1. Click **New environment**
2. Name: `staging`
3. Click **Configure environment**
4. Under **Deployment branches**: Click **Add deployment branch rule**
5. Branch name pattern: `develop`
6. Click **Add rule**
7. **Don't** check "Required reviewers" (auto-deploy)
8. Click **Save protection rules**

#### Add Environment Secrets

Click **Add environment secret** for each:

| Secret Name | Value | From Step |
|-------------|-------|-----------|
| `OCI_USER_OCID` | `ocid1.user.oc1..xxx` | Step 3 |
| `OCI_TENANCY_OCID` | `ocid1.tenancy.oc1..xxx` | Step 3 |
| `OCI_COMPARTMENT_OCID` | Same as tenancy | Step 3 |
| `OCI_REGION` | `us-ashburn-1` | Step 3 |
| `OCI_FINGERPRINT` | `aa:bb:cc:...` | Step 1 output |
| `OCI_PRIVATE_KEY` | Entire PEM file content | `cat ~/.oci/oci_api_key.pem` |
| `ORACLE_DB_ADMIN_PASSWORD` | Generated password | Step 4 |
| `ORACLE_DB_PASSWORD` | Generated password | Step 4 |
| `EPX_MAC_STAGING` | `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` | Step 5 |
| `CRON_SECRET_STAGING` | Generated base64 string | Step 5 |
| `SSH_PUBLIC_KEY` | Your SSH public key | Step 6 |
| `OCIR_REGION` | Same as `OCI_REGION` | Step 3 |
| `OCIR_TENANCY_NAMESPACE` | Namespace | Step 7 |
| `OCIR_USERNAME` | `namespace/oracleidentitycloudservice/email` | Step 7 |
| `OCIR_AUTH_TOKEN` | Generated token | Step 7 |

**For `OCI_PRIVATE_KEY`:**
```bash
cat ~/.oci/oci_api_key.pem
```
Copy the **entire output** including:
```
-----BEGIN RSA PRIVATE KEY-----
... all the content ...
-----END RSA PRIVATE KEY-----
```

---

### Step 9: Run Terraform Workflow (10 minutes)

1. Go to **Actions** tab in your repository
2. Click **Terraform Infrastructure**
3. Click **Run workflow** (top right)
4. Select branch: `develop`
5. Select action: `apply`
6. Click **Run workflow**

**What happens:**
- Terraform provisions Oracle Cloud resources
- Creates Autonomous Database (20GB)
- Creates Ampere A1 Compute (4 cores, 24GB RAM)
- Sets up VCN networking
- Installs Docker
- Takes ~10-12 minutes

**Monitor the workflow** - it will show each step.

---

### Step 10: Get Terraform Outputs (1 minute)

After Terraform completes successfully:

1. Click on the completed workflow run
2. Scroll to **Terraform Outputs** section
3. You'll see formatted GitHub secrets including:
   - `ORACLE_CLOUD_HOST` (public IP)
   - Possibly `ORACLE_CLOUD_SSH_KEY` (if auto-generated)

**Add these additional secrets** to the staging environment.

---

### Step 11: Deploy Application (3 minutes)

```bash
cd ~/Documents/projects/payments

# Commit all changes
git add .
git commit -m "feat: Add Terraform infrastructure and auto-lifecycle for Oracle Cloud staging"

# Push to develop
git checkout develop
git push origin develop
```

**What happens:**
1. CI/CD pipeline runs
2. Tests pass
3. Docker image builds
4. Pushes to Oracle Container Registry
5. SSHs to staging instance
6. Deploys application
7. Runs integration tests
8. Seeds database with test data
9. Posts success summary

**Monitor:** `https://github.com/YOUR_USERNAME/payment-service/actions`

---

### Step 12: Verify Deployment (1 minute)

Once deployment completes:

```bash
# Get your staging IP from Terraform outputs or GitHub secrets
STAGING_IP="xxx.xxx.xxx.xxx"

# Test health endpoint
curl http://$STAGING_IP:8081/cron/health

# Expected response:
# {"status":"healthy","time":"2025-11-08T..."}

# Test stats endpoint
curl http://$STAGING_IP:8081/cron/stats

# Should show counts of subscriptions and transactions
```

---

## 🎉 Success Criteria

After completing all steps, you should have:

- ✅ Oracle Cloud staging environment running
- ✅ Autonomous Database (20GB) created
- ✅ Ampere A1 Compute (4 cores, 24GB RAM) running
- ✅ Application deployed and healthy
- ✅ Integration tests passing
- ✅ Database seeded with test data
- ✅ Can access: `http://STAGING_IP:8081/cron/health`

---

## 📋 Quick Reference

### What You'll Need

**From Oracle Cloud Console:**
- User OCID
- Tenancy OCID
- Region
- Tenancy Namespace
- Auth Token (for OCIR)

**Generated Locally:**
- API key pair (`~/.oci/oci_api_key.pem`)
- API fingerprint
- Database passwords (2)
- Cron secret
- SSH public key

**From EPX Docs:**
- EPX MAC (sandbox): `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y`

### Total Time Estimate

- Setup: ~20 minutes
- Terraform provisioning: ~10 minutes
- First deployment: ~5 minutes
- **Total: ~35 minutes**

### Cost

**$0.00/month** - Everything uses Oracle Always Free tier

---

## 🆘 Need Help?

If you get stuck:

1. Check GitHub Actions logs
2. Review Terraform output
3. Verify all secrets are correct
4. Check Oracle Cloud Console for resources
5. SSH to instance and check Docker logs:
   ```bash
   ssh -i ~/.ssh/oracle-staging ubuntu@$STAGING_IP
   docker logs payment-staging -f
   ```

---

## Next Steps After Setup

Once staging is running:

1. Test EPX integration with seed data
2. Verify subscription billing cron
3. Test chargeback polling
4. Run manual integration tests
5. When ready, merge to `main` for production deploy
6. Staging will auto-destroy after production success

---

**Ready to begin? Start with Step 1!** 🚀
