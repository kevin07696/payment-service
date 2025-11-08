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

**What it is**: SSH public key for accessing the Oracle Cloud VM

**Where to get it**:
```bash
# If you have an existing SSH key:
cat ~/.ssh/id_rsa.pub

# If you don't have one, the script will generate it automatically
```

**Auto-detection**: Script automatically detects `~/.ssh/id_rsa.pub`

---

### 3. ORACLE_CLOUD_SSH_KEY

**What it is**: SSH private key (companion to the public key above)

**Where to get it**:
```bash
cat ~/.ssh/id_rsa
```

**Auto-detection**: Script automatically reads `~/.ssh/id_rsa`

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

### 9-13. EPX Payment Gateway Credentials (5 secrets)

**What they are**: Credentials for EPX payment processor (used for ALL payment processing, not just testing)

#### EPX_CUST_NBR (Customer Number)
#### EPX_MERCH_NBR (Merchant Number)
#### EPX_DBA_NBR (DBA Number)
#### EPX_TERMINAL_NBR (Terminal Number)
#### EPX_MAC (Browser Post MAC Key)

**Where to get them**:

**Option 1: For Development/Testing (SANDBOX)**
Use the sandbox credentials from `.env.example`:
```bash
EPX_CUST_NBR=9001
EPX_MERCH_NBR=900300
EPX_DBA_NBR=2
EPX_TERMINAL_NBR=77
EPX_MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
```

**Option 2: For Production/Real Processing**
Contact EPX integration team:
- Email: Your EPX account representative
- They will provide your 4-part key when you're ready to implement
- See: `~/Downloads/supplemental-resources/Testing Information/EPX Certification - Credentials.pdf`

**What each part means**:
- `CUST_NBR`: Customer number (identifies you as EPX customer)
- `MERCH_NBR`: Merchant number (your merchant account)
- `DBA_NBR`: DBA number (doing-business-as identifier)
- `TERMINAL_NBR`: Terminal number (specific terminal/location)
- `EPX_MAC`: Merchant Authorization Code (for Browser Post API - PCI-compliant payment forms)

**For staging deployment**: Use the **sandbox credentials** from `.env.example` so you can test payment flows without processing real transactions.

---

## Summary: Quick Action Items

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

### Use Sandbox Values (5 secrets):
9-13. EPX credentials → Copy from `.env.example` (lines 42-48)

---

## Running the Configuration Script

```bash
# Make sure you're in the payment service directory
cd ~/Documents/projects/payments

# Run the configuration script
./scripts/configure-remaining-secrets.sh
```

The script will:
1. Auto-detect 5 values from your OCI CLI
2. Prompt you to create 2 database passwords
3. Ask you to generate 1 auth token (opens browser)
4. Ask if you want to use sandbox EPX credentials or enter production ones
5. Configure all 17 secrets in GitHub

**Estimated time**: 10-15 minutes

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
