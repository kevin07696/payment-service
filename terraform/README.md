# Terraform Infrastructure for Oracle Cloud Staging

**Infrastructure as Code** - No manual provisioning needed!

## Quick Start (5 minutes)

### 1. Generate Oracle Cloud API Key

```bash
cd terraform

# Generate API key pair
mkdir -p ~/.oci
ssh-keygen -t rsa -b 4096 -f ~/.oci/oci_api_key -N ""

# Convert to PEM format (required by Oracle)
openssl rsa -in ~/.oci/oci_api_key -out ~/.oci/oci_api_key.pem -traditional

# Get the public key fingerprint
openssl rsa -pubout -outform DER -in ~/.oci/oci_api_key.pem | openssl md5 -c
```

### 2. Upload API Key to Oracle Cloud

```bash
# Display public key to upload
cat ~/.oci/oci_api_key.pub
```

**In Oracle Cloud Console:**
1. Click Profile Icon → **User Settings**
2. Click **API Keys** (left sidebar)
3. Click **Add API Key**
4. Select **Paste Public Key**
5. Paste the output from above
6. Click **Add**
7. **Copy the fingerprint** (you'll need this for GitHub secrets)

### 3. Get Required OCIDs

From Oracle Cloud Console:

```bash
# Get User OCID
# Profile Icon → User Settings → Copy OCID

# Get Tenancy OCID
# Profile Icon → Tenancy → Copy OCID

# Compartment OCID (usually same as Tenancy for root compartment)
# Same as Tenancy OCID

# Region
# Visible in top-right (e.g., "us-ashburn-1")
```

### 4. Add GitHub Secrets

Go to: `https://github.com/YOUR_USERNAME/payment-service/settings/secrets/actions`

Add these secrets to the **staging** environment:

| Secret Name | Value | Where to Get |
|-------------|-------|--------------|
| `OCI_USER_OCID` | `ocid1.user.oc1..xxx` | User Settings → OCID |
| `OCI_TENANCY_OCID` | `ocid1.tenancy.oc1..xxx` | Tenancy → OCID |
| `OCI_COMPARTMENT_OCID` | Same as tenancy | Tenancy → OCID |
| `OCI_FINGERPRINT` | `aa:bb:cc:...` | From step 2 |
| `OCI_PRIVATE_KEY` | PEM format | `cat ~/.oci/oci_api_key.pem` |
| `OCI_REGION` | `us-ashburn-1` | Console top-right |
| `ORACLE_DB_ADMIN_PASSWORD` | Your choice | Complex password |
| `ORACLE_DB_PASSWORD` | Your choice | Complex password |
| `EPX_MAC_STAGING` | `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` | From EPX docs |
| `CRON_SECRET_STAGING` | Random string | `openssl rand -base64 32` |
| `SSH_PUBLIC_KEY` | Your SSH key | `cat ~/.ssh/id_rsa.pub` |

### 5. Run Terraform via GitHub Actions

**Option A: Via GitHub UI**
1. Go to **Actions** tab
2. Click **Terraform Infrastructure**
3. Click **Run workflow**
4. Select action: `apply`
5. Click **Run workflow**

**Option B: Locally (if you prefer)**

```bash
cd terraform

# Create terraform.tfvars from example
cp terraform.tfvars.example terraform.tfvars

# Edit with your values
nano terraform.tfvars

# Initialize Terraform
terraform init

# Preview changes
terraform plan

# Apply infrastructure
terraform apply
```

### 6. Get Outputs

After Terraform completes:

```bash
# View all outputs
terraform output

# Get formatted GitHub secrets
terraform output -raw github_secrets
```

Copy the additional GitHub secrets that Terraform generates:
- `ORACLE_CLOUD_HOST` (public IP)
- `ORACLE_CLOUD_SSH_KEY` (if auto-generated)

---

## What Gets Created

✅ **Autonomous Database**
- 20GB PostgreSQL-compatible
- Always Free tier
- Automatic backups
- Wallet auto-downloaded

✅ **Compute Instance**
- Ampere A1 (4 cores, 24GB RAM)
- Ubuntu 22.04 ARM
- Always Free tier
- Docker pre-installed

✅ **Networking**
- VCN (Virtual Cloud Network)
- Internet Gateway
- Security lists (ports: 22, 8080, 8081)
- Public subnet

✅ **Configuration**
- Oracle wallet downloaded
- Environment variables configured
- docker-compose.yml created
- Firewall rules applied

---

## Daily Usage

### Deploy Application Changes

Your existing CI/CD workflow (`.github/workflows/ci-cd.yml`) handles deployments automatically:

```bash
git checkout develop
git add .
git commit -m "feat: My changes"
git push origin develop
```

GitHub Actions will:
1. Build Docker image
2. Push to Oracle Container Registry
3. SSH to instance and deploy

### Modify Infrastructure

To change infrastructure (e.g., increase memory):

```bash
cd terraform

# Edit variables
nano terraform.tfvars

# Apply changes
terraform apply
```

Or update via GitHub Actions.

### Destroy Everything

**DANGER:** This deletes all resources!

```bash
cd terraform
terraform destroy
```

Or via GitHub Actions:
- Actions → Terraform Infrastructure → Run workflow → Select `destroy`

---

## Architecture

```
┌─────────────────────────────────────┐
│ GitHub Actions                      │
│ ├── Terraform (infrastructure)      │
│ └── CI/CD (application deployment)  │
└─────────────────────────────────────┘
              ▼
┌─────────────────────────────────────┐
│ Oracle Cloud (Always Free)          │
│                                     │
│ ┌─────────────────────────────────┐ │
│ │ Autonomous Database             │ │
│ │ - 20GB PostgreSQL               │ │
│ │ - Auto backups                  │ │
│ └─────────────────────────────────┘ │
│                                     │
│ ┌─────────────────────────────────┐ │
│ │ Ampere A1 Compute               │ │
│ │ - 4 cores, 24GB RAM             │ │
│ │ - Docker installed              │ │
│ │ - Auto-configured               │ │
│ └─────────────────────────────────┘ │
│                                     │
│ ┌─────────────────────────────────┐ │
│ │ VCN Networking                  │ │
│ │ - Public subnet                 │ │
│ │ - Internet gateway              │ │
│ │ - Security lists                │ │
│ └─────────────────────────────────┘ │
└─────────────────────────────────────┘
```

---

## Troubleshooting

### Terraform Init Fails

```bash
# Re-initialize
cd terraform
rm -rf .terraform .terraform.lock.hcl
terraform init
```

### Invalid Credentials

Double-check:
- User OCID matches your user
- Tenancy OCID is correct
- API key fingerprint matches uploaded key
- Private key is in PEM format

```bash
# Verify PEM format
head -n 1 ~/.oci/oci_api_key.pem
# Should show: -----BEGIN RSA PRIVATE KEY-----
```

### Resource Already Exists

If a resource already exists from manual setup:

```bash
# Import existing resources
terraform import oci_core_instance.payment_instance ocid1.instance.oc1...
terraform import oci_database_autonomous_database.payment_db ocid1.autonomousdatabase.oc1...
```

Or destroy the manual resources first.

### Capacity Unavailable

If Always Free ARM instances are unavailable in your region:

```bash
# Try different availability domain
terraform apply -var="instance_shape=VM.Standard.E2.1.Micro"  # x86 Always Free
```

---

## Cost

**Total: $0.00/month**

All resources use Oracle Cloud Always Free tier:
- ✅ Autonomous Database: Free forever
- ✅ Ampere A1 Compute: Free forever
- ✅ Networking: Free
- ✅ Container Registry: Free

No expiration. No credit card charges.

---

## Files

```
terraform/
├── main.tf              # Terraform configuration
├── variables.tf         # Input variables
├── outputs.tf           # Output values
├── networking.tf        # VCN, security lists
├── database.tf          # Autonomous Database
├── compute.tf           # Compute instance
├── cloud-init.yaml      # Instance setup script
├── terraform.tfvars.example  # Example config
└── README.md            # This file
```

---

## Next Steps

1. ✅ Run Terraform to create infrastructure
2. ✅ Add GitHub secrets (from Terraform outputs)
3. ✅ Push to `develop` branch
4. ✅ GitHub Actions deploys automatically
5. ✅ Access your app at `http://PUBLIC_IP:8081`

**No manual provisioning. Just code.** 🚀
