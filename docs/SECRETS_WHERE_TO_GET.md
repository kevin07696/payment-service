# Where to Get Each GitHub Secret

This guide shows exactly where to obtain each secret value that's still needed.

## Already Configured ✅ (7 secrets)

These were auto-configured from your `~/.oci/config`:
- ✅ OCI_USER_OCID
- ✅ OCI_TENANCY_OCID
- ✅ OCI_FINGERPRINT
- ✅ OCI_PRIVATE_KEY
- ✅ OCI_REGION
- ✅ OCIR_REGION
- ✅ CRON_SECRET_STAGING (auto-generated)

---

## Still Need to Configure (10 secrets)

### 1. OCI_COMPARTMENT_OCID

**What it is**: The Oracle Cloud compartment where your resources will be created

**Where to get it**:
```bash
# Option 1: Use root compartment (same as tenancy OCID)
echo $TENANCY_OCID  # Usually same as OCI_TENANCY_OCID

# Option 2: Get from Oracle Cloud Console
# Go to: Identity & Security → Compartments → Select compartment → Copy OCID
```

**Auto-detection**: The script will suggest using your tenancy OCID as the root compartment

---

### 2. SSH_PUBLIC_KEY

**What it is**: SSH public key for accessing Oracle Cloud VMs

**Where to get it**:
```bash
# If you have an existing SSH key:
cat ~/.ssh/id_rsa.pub

# If you don't have one, generate it once:
ssh-keygen -t rsa -b 4096 -C "github-actions" -f ~/.ssh/id_rsa -N ""
cat ~/.ssh/id_rsa.pub
```

**Auto-detection**: Script automatically detects `~/.ssh/id_rsa.pub`

**Note**: This is a **shared SSH key** used across all your services. You generate it once on your local machine and configure it in each service's GitHub repo secrets. This is simpler than managing separate keys per service.

---

### 3. ORACLE_CLOUD_SSH_KEY

**What it is**: SSH private key (companion to the public key above)

**Where to get it**:
```bash
cat ~/.ssh/id_rsa
```

**Auto-detection**: Script automatically reads `~/.ssh/id_rsa`

**Note**: Same SSH key pair used across all your services for consistency and simplicity.

---

### 4. ORACLE_DB_ADMIN_PASSWORD

**What it is**: Admin password for the Oracle Autonomous Database

**Where to get it**: **YOU CREATE THIS**

**Requirements**:
- 12-30 characters
- At least 1 uppercase letter
- At least 1 lowercase letter
- At least 1 number
- Cannot contain the username "admin"
- Cannot contain double quotes (")

**Example**: `WelcomePass2024!Secure`

**Note**: This password will be used by Terraform to create the database. Store it securely!

---

### 5. ORACLE_DB_PASSWORD

**What it is**: Password for the application database user `payment_service`

**Where to get it**: **YOU CREATE THIS**

**Requirements**: Same as ORACLE_DB_ADMIN_PASSWORD above

**Example**: `AppUser2024!Secure`

**Note**: Different from admin password! This is what the payment service uses to connect.

---

### 6. OCIR_TENANCY_NAMESPACE

**What it is**: Your Oracle Cloud object storage namespace (used for container registry)

**Where to get it**:
```bash
# Option 1: Via OCI CLI (auto-detected by script)
oci os ns get

# Option 2: Oracle Cloud Console
# Go to: Profile (top right) → Tenancy → Object Storage Namespace
```

**Auto-detection**: Script automatically runs `oci os ns get`

---

### 7. OCIR_USERNAME

**What it is**: Username for Oracle Container Image Registry (OCIR)

**Format**: `<namespace>/<identity-provider>/<username>`

**Where to get it**: **CONSTRUCTED BY SCRIPT**

The script will construct this from:
- Namespace (auto-detected above)
- Identity provider: `oracleidentitycloudservice` (standard for Oracle Identity)
- Your email: The email you use to log into Oracle Cloud

**Example**: `axabcde/oracleidentitycloudservice/kevinlam.vn@gmail.com`

**Auto-construction**: Script asks for your Oracle Cloud email and builds this

---

### 8. OCIR_AUTH_TOKEN

**What it is**: Authentication token for pushing Docker images to OCIR

**Where to get it**:
1. Go to Oracle Cloud Console
2. Profile (top right) → User Settings
3. Resources (left sidebar) → Auth Tokens
4. Click "Generate Token"
5. Give it a description: "GitHub Actions OCIR Push"
6. Copy the token **immediately** (you can't view it again!)

**Important**:
- You can only view the token when it's first created
- Store it securely
- You can have multiple auth tokens (useful for rotation)

---

### 9. EPX_MAC_STAGING (Test Merchant MAC Key)

**What it is**: MAC (Merchant Authorization Code) secret for the EPX test merchant

**Where to get it**: **From `.env.example`**

```bash
EPX_MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
```

**Important - Multi-Tenant Architecture**:

This service follows **multi-tenant architecture** where EPX merchant credentials are stored **per-agent in the database**, not as service-wide environment variables.

**How it works**:
1. Merchant credentials (CUST_NBR, MERCH_NBR, DBA_NBR, TERMINAL_NBR) are stored in the `agent_credentials` database table
2. Seed data (`internal/db/seeds/staging/003_agent_credentials.sql`) creates a test agent with EPX sandbox credentials
3. Only the MAC secret for the test merchant is provided as an environment variable
4. Integration tests query the database for test merchant credentials

**Why this architecture**:
- ✅ Proper multi-tenancy (each merchant has their own credentials)
- ✅ No hardcoded credentials in service configuration
- ✅ Easy to add new merchants (just insert into database)
- ✅ Credentials isolated per agent/merchant

**Test Agent in Staging**:
- Agent ID: `test-merchant-staging`
- CUST_NBR: `9001` (in database)
- MERCH_NBR: `900300` (in database)
- DBA_NBR: `2` (in database)
- TERMINAL_NBR: `77` (in database)
- MAC Secret: `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y` (from EPX_MAC_STAGING secret)

---

## Summary: Quick Action Items

**Total: 9 secrets** (reduced from 17 by using DB seed data for EPX credentials!)

### Can Auto-Configure (5 secrets):
1. OCI_COMPARTMENT_OCID → Script suggests tenancy OCID
2. SSH_PUBLIC_KEY → Script detects `~/.ssh/id_rsa.pub`
3. ORACLE_CLOUD_SSH_KEY → Script detects `~/.ssh/id_rsa`
4. OCIR_TENANCY_NAMESPACE → Script runs `oci os ns get`
5. OCIR_USERNAME → Script constructs from namespace + your email

### Need to Create (2 secrets):
6. ORACLE_DB_ADMIN_PASSWORD → Create strong password (16+ chars)
7. ORACLE_DB_PASSWORD → Create different strong password (16+ chars)

### Need to Generate (1 secret):
8. OCIR_AUTH_TOKEN → Generate in Oracle Cloud Console

### Use Sandbox Value (1 secret):
9. EPX_MAC_STAGING → Copy from `.env.example` line 48: `2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y`

**Note**: EPX merchant credentials (CUST_NBR, MERCH_NBR, DBA_NBR, TERMINAL_NBR) are stored in the database via seed data, not as secrets!

---

## Running the Configuration Script

```bash
# Make sure you're in the payment service directory
cd ~/Documents/projects/payments

# Run the configuration script
./scripts/configure-remaining-secrets.sh
```

The script will:
1. Auto-detect 5 values from your OCI CLI and SSH keys
2. Prompt you to create 2 database passwords
3. Ask you to generate 1 auth token
4. Use sandbox EPX MAC key
5. Configure all 9 secrets in GitHub

**Estimated time**: 5-10 minutes

**Note**: The SSH keys (`~/.ssh/id_rsa` and `~/.ssh/id_rsa.pub`) are shared across all your services. You only need to generate them once on your local machine, then configure them in each service repo's GitHub secrets.

---

## After Configuration

Once all secrets are configured:

1. Uncomment deployment stages in `.github/workflows/ci-cd.yml`
2. Commit and push to `develop` branch
3. Watch the magic happen! 🚀

The pipeline will:
- Create Oracle Cloud infrastructure (VM + Database)
- Deploy your payment service
- Run integration tests
- Clean up infrastructure (saves money!)
