# Oracle Cloud Staging with Terraform (Infrastructure as Code)

**Zero manual provisioning.** Everything in code. Repeatable. Destroyable.

## Why Terraform?

✅ **No manual clicking** - Everything automated
✅ **Version controlled** - Infrastructure in Git
✅ **Repeatable** - Destroy and recreate anytime
✅ **Token-based auth** - No interactive setup
✅ **GitHub Actions** - Deploy from CI/CD

## Architecture

```
GitHub Repo
  └── terraform/
      ├── main.tf           # Terraform config
      ├── variables.tf      # Input variables
      ├── networking.tf     # VCN, security
      ├── database.tf       # Autonomous DB
      └── compute.tf        # Compute instance

GitHub Actions
  ├── Terraform Workflow  # Provisions infrastructure
  └── CI/CD Workflow      # Deploys application

Oracle Cloud (Always Free)
  ├── Autonomous Database (20GB)
  ├── Ampere A1 Compute (4 cores, 24GB)
  └── VCN Networking
```

---

## Quick Start (10 minutes)

### Step 1: Generate API Key (2 minutes)

```bash
cd ~/Documents/projects/payments

# Run the automated script
./scripts/generate-oracle-api-key.sh
```

**Output:**
- Creates API key pair in `~/.oci/`
- Displays public key to upload
- Shows fingerprint

### Step 2: Upload to Oracle Cloud (2 minutes)

1. Go to: https://cloud.oracle.com
2. Click **Profile Icon** → **User Settings**
3. Click **API Keys** (left sidebar)
4. Click **Add API Key**
5. Select **Paste Public Key**
6. Paste the public key from step 1
7. Click **Add**
8. **Verify fingerprint matches** the one displayed

### Step 3: Get OCIDs (2 minutes)

```bash
# In Oracle Cloud Console:

# 1. User OCID
#    Profile Icon → User Settings → Copy OCID
#    Looks like: ocid1.user.oc1..aaaaa...

# 2. Tenancy OCID
#    Profile Icon → Tenancy → Copy OCID
#    Looks like: ocid1.tenancy.oc1..aaaaa...

# 3. Compartment OCID
#    Usually same as Tenancy OCID (for root compartment)

# 4. Region
#    Top-right corner (e.g., "us-ashburn-1")
```

### Step 4: Add GitHub Secrets (3 minutes)

Go to: `https://github.com/YOUR_USERNAME/payment-service/settings/environments`

Create **staging** environment, then add these secrets:

```bash
# Oracle Cloud Authentication
OCI_USER_OCID=ocid1.user.oc1..your-user-ocid
OCI_TENANCY_OCID=ocid1.tenancy.oc1..your-tenancy-ocid
OCI_COMPARTMENT_OCID=ocid1.tenancy.oc1..your-tenancy-ocid  # Same as tenancy
OCI_REGION=us-ashburn-1
OCI_FINGERPRINT=aa:bb:cc:dd:ee:ff...  # From step 1

# Private key (paste entire file content)
OCI_PRIVATE_KEY=
-----BEGIN RSA PRIVATE KEY-----
... paste from: cat ~/.oci/oci_api_key.pem ...
-----END RSA PRIVATE KEY-----

# Database passwords (choose secure passwords)
ORACLE_DB_ADMIN_PASSWORD=YourSecureAdminPassword123!
ORACLE_DB_PASSWORD=YourSecureAppPassword123!

# Application secrets
EPX_MAC_STAGING=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
CRON_SECRET_STAGING=$(openssl rand -base64 32)  # Generate random

# SSH public key (your existing key or generate new one)
SSH_PUBLIC_KEY=ssh-rsa AAAAB3NzaC1... your-key
```

### Step 5: Run Terraform (1 minute)

**Via GitHub Actions UI:**

1. Go to **Actions** tab
2. Click **Terraform Infrastructure**
3. Click **Run workflow**
4. Select: `apply`
5. Click **Run workflow**

GitHub Actions will:
1. Configure Terraform
2. Create Autonomous Database (20GB)
3. Create Compute Instance (4 cores, 24GB)
4. Set up networking
5. Install Docker
6. Generate outputs

**⏱️ Takes ~10 minutes**

### Step 6: Get Outputs & Add More Secrets

After Terraform completes:

1. Click on the completed workflow run
2. Scroll to bottom - see **Terraform Outputs**
3. Copy the additional secrets:

```bash
ORACLE_CLOUD_HOST=xxx.xxx.xxx.xxx  # Public IP from Terraform
```

If Terraform auto-generated SSH key:
```bash
ORACLE_CLOUD_SSH_KEY=
-----BEGIN OPENSSH PRIVATE KEY-----
... from Terraform outputs ...
-----END OPENSSH PRIVATE KEY-----
```

Add these to your **staging** environment secrets.

### Step 7: Deploy Application

```bash
git checkout develop
git add .
git commit -m "feat: Add Terraform infrastructure"
git push origin develop
```

Your existing CI/CD workflow will:
1. Build Docker image
2. Push to Oracle Container Registry
3. SSH to instance and deploy
4. Run health check

**Done! Your staging is live at `http://PUBLIC_IP:8081`**

---

## Daily Workflow

### Deploy Code Changes

```bash
git checkout develop
# ... make changes ...
git add .
git commit -m "feat: My feature"
git push origin develop
```

**Automatic deployment via GitHub Actions**

### Modify Infrastructure

```bash
# Edit Terraform config
nano terraform/compute.tf

# Commit changes
git add terraform/
git commit -m "infra: Increase memory to 32GB"
git push

# Run Terraform workflow
# Actions → Terraform Infrastructure → apply
```

### View Infrastructure State

```bash
# Locally (if you ran Terraform locally)
cd terraform
terraform show

# Or check GitHub Actions logs
```

### Destroy Everything

**DANGER:** Deletes all resources!

```bash
# Via GitHub Actions
# Actions → Terraform Infrastructure → destroy
```

---

## Advantages over Bash Script

| Feature | Bash Script | Terraform |
|---------|-------------|-----------|
| Manual setup | ❌ Requires OCI CLI config | ✅ Token-based |
| Version control | ⚠️ Output only | ✅ Full IaC |
| Repeatability | ⚠️ One-time | ✅ Destroy/recreate |
| State management | ❌ None | ✅ State tracking |
| Change preview | ❌ None | ✅ terraform plan |
| Rollback | ❌ Manual | ✅ Git revert |
| Collaboration | ❌ Difficult | ✅ Easy |
| CI/CD integration | ⚠️ Basic | ✅ Native |

---

## Terraform Files Explained

### `main.tf`
- Terraform version
- Provider configuration (OCI)
- Backend configuration (state storage)

### `variables.tf`
- Input variables (OCIDs, passwords, config)
- Defaults and descriptions
- Sensitive variable marking

### `networking.tf`
- VCN (Virtual Cloud Network)
- Internet Gateway
- Route Table
- Security Lists (firewall rules)
- Subnet

### `database.tf`
- Autonomous Database (Always Free)
- Wallet download
- Connection strings

### `compute.tf`
- Compute instance (Ampere A1)
- Ubuntu ARM image
- cloud-init configuration
- SSH key setup

### `cloud-init.yaml`
- Docker installation
- Environment variables
- Application setup
- Firewall configuration

### `outputs.tf`
- Public IP
- OCIDs
- GitHub secrets (formatted)
- Connection details

---

## Troubleshooting

### Terraform Init Fails

```bash
cd terraform
rm -rf .terraform .terraform.lock.hcl
terraform init
```

### Invalid Credentials Error

Check GitHub secrets:
- `OCI_USER_OCID` - Correct user OCID?
- `OCI_TENANCY_OCID` - Correct tenancy?
- `OCI_FINGERPRINT` - Matches uploaded key?
- `OCI_PRIVATE_KEY` - Full PEM format?

Verify fingerprint:
```bash
cat ~/.oci/oci_fingerprint.txt
```

### Resource Already Exists

If you ran the bash script before:

```bash
# Import existing resources
terraform import oci_core_instance.payment_instance ocid1.instance.oc1...
terraform import oci_database_autonomous_database.payment_db ocid1.autonomousdatabase.oc1...
```

Or delete existing resources:
```bash
# Via OCI CLI
oci compute instance terminate --instance-id <OCID> --force
oci db autonomous-database delete --autonomous-database-id <OCID> --force
```

### Capacity Unavailable

Always Free ARM instances may be unavailable. Try:

**Option 1: Different Availability Domain**
```hcl
# In terraform/compute.tf
availability_domain = data.oci_identity_availability_domains.ads.availability_domains[1].name
```

**Option 2: Use x86 Instead (also Always Free)**
```hcl
# In terraform.tfvars
instance_shape = "VM.Standard.E2.1.Micro"
instance_ocpus = 1
instance_memory_gb = 1
```

### Terraform Apply Hangs

Some resources take time:
- Database: ~5-10 minutes
- Compute: ~3-5 minutes

Check GitHub Actions logs or Oracle Console.

---

## Cost

**$0.00/month forever**

Uses Oracle Cloud Always Free tier:
- ✅ 2 Autonomous Databases (20GB each)
- ✅ 4 Ampere A1 cores + 24GB RAM
- ✅ 200GB block storage
- ✅ Networking (10TB/month)
- ✅ Container Registry (500GB storage)

**No expiration. No credit card charges.**

---

## Security

### API Key Security
- ✅ Private key never committed to Git
- ✅ Stored as GitHub secret
- ✅ Only used in GitHub Actions
- ✅ Can be rotated anytime

### Database Security
- ✅ Passwords in GitHub secrets
- ✅ SSL/TLS connections only
- ✅ Wallet-based authentication
- ✅ Private subnet option available

### Compute Security
- ✅ Security lists (firewall)
- ✅ SSH key authentication only
- ✅ Ubuntu auto-updates enabled
- ✅ ufw firewall configured

---

## Comparison: Manual vs Terraform

### Manual Setup (old bash script)
```bash
# Run once, hope it works
./scripts/setup-oracle-staging.sh

# If something breaks, start over
# If you want to change something, manual edits
# No version control for infrastructure
```

### Terraform Setup (new)
```bash
# Define infrastructure as code
# Version controlled
# Repeatable
# Preview changes before applying

# Want to change memory? Edit and apply:
terraform apply

# Want to destroy and recreate?
terraform destroy
terraform apply

# Everything is tracked and reproducible
```

---

## Migration from Bash Script

If you already ran the bash setup script:

### Option 1: Import Existing Resources

```bash
cd terraform

# Get OCIDs from existing setup
source ~/oracle-staging-setup/oracle-config.env

# Import resources
terraform import oci_core_instance.payment_instance $INSTANCE_OCID
terraform import oci_database_autonomous_database.payment_db $DB_OCID
terraform import oci_core_vcn.payment_vcn $VCN_OCID
# ... etc

# Verify
terraform plan  # Should show no changes
```

### Option 2: Clean Slate (Recommended)

```bash
# Delete old resources via bash script or OCI CLI
source ~/oracle-staging-setup/oracle-config.env

oci compute instance terminate --instance-id "$INSTANCE_OCID" --force
oci db autonomous-database delete --autonomous-database-id "$DB_OCID" --force

# Wait for deletion (~5 minutes)

# Then run Terraform
# Actions → Terraform Infrastructure → apply
```

---

## Next Steps

1. ✅ Generate API key
2. ✅ Upload to Oracle Cloud
3. ✅ Add GitHub secrets
4. ✅ Run Terraform workflow
5. ✅ Add output secrets (ORACLE_CLOUD_HOST, etc.)
6. ✅ Push to develop → auto-deploy
7. ✅ Access: `http://PUBLIC_IP:8081`

**Infrastructure as Code. Zero manual provisioning.** 🚀

---

## Resources

- **Terraform Docs**: https://registry.terraform.io/providers/oracle/oci/
- **Oracle Cloud Free Tier**: https://www.oracle.com/cloud/free/
- **OCI Provider**: https://github.com/oracle/terraform-provider-oci

---

## Support

Having issues?

1. Check GitHub Actions logs
2. Verify all secrets are correct
3. Check Terraform state: `terraform show`
4. Review Oracle Cloud Console
5. Try `terraform destroy` and `terraform apply` again
