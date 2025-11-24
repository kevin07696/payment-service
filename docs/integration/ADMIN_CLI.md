# Admin CLI Documentation

**Target Audience:** Platform administrators, DevOps engineers
**Topic:** Service and merchant management using the admin CLI tool
**Goal:** Securely create and manage services, merchants, and access control

---

## Overview

The **admin CLI** (`cmd/admin/main.go`) is a command-line tool for managing:
- **Services** - Applications/integrations that authenticate via RSA keypairs
- **Merchants** - Business entities with EPX gateway credentials
- **Access Control** - Granting services access to merchants with scoped permissions

**Key Concepts:**
- Services use RSA keypairs for JWT-based authentication
- Merchants store EPX credentials (retrieved from secret manager)
- service_merchants junction table defines which services can access which merchants
- Private keys are returned ONCE and never stored in database

---

## Building the CLI

```bash
# Build the admin CLI
go build -o admin ./cmd/admin

# Verify it works
./admin -h
```

**Output:**
```
Admin CLI for Payment Service

Usage:
  ./admin -action=<action> [flags]

Actions:
  create-service    Create a new service with RSA keypair
  create-merchant   Create a new merchant with EPX credentials
  grant-access      Grant a service access to a merchant
  list-services     List all services
  list-merchants    List all merchants
  revoke-access     Revoke a service's access to a merchant

Flags:
  -action string
        Action to perform (required)
  -config string
        Path to JSON config file (optional)
  -database-url string
        Database connection URL (default: from DATABASE_URL env var)
```

---

## Actions

### 1. Create Service

**Purpose:** Register a new service (POS system, e-commerce backend, mobile app)

**Command:**
```bash
./admin -action=create-service
```

**Interactive Prompts:**
```
Enter Service ID (e.g., acme-pos-system): acme-pos-system
Enter Service Name (e.g., ACME POS System): ACME POS System
Enter Environment (production/staging): production
Enter Requests Per Second (default 1000): 1000
Enter Burst Limit (default 2000): 2000
```

**Output:**
```
✅ Service created successfully!

Service Details:
  Service ID: acme-pos-system
  Service Name: ACME POS System
  Environment: production
  Rate Limits: 1000 req/s, burst 2000

🔐 PRIVATE KEY (SAVE THIS - IT WILL NOT BE SHOWN AGAIN!):
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1234567890...
-----END RSA PRIVATE KEY-----

Public Key Fingerprint: SHA256:abc123def456...

⚠️  CRITICAL: Save the private key above immediately!
This key is required for JWT token signing and will never be shown again.

Store it securely:
  1. Save to file: keys/acme-pos-system.pem
  2. Set file permissions: chmod 600 keys/acme-pos-system.pem
  3. Use in environment: PRIVATE_KEY_PATH=keys/acme-pos-system.pem
```

**What Happens:**
1. Generates 2048-bit RSA keypair
2. Stores **public key** in database (`services` table)
3. Returns **private key** ONCE (never stored in database)
4. Service uses private key to sign JWT tokens

**Using JSON Config (Non-Interactive):**
```bash
# Create config.json
cat > service-config.json <<EOF
{
  "service_id": "acme-pos-system",
  "service_name": "ACME POS System",
  "environment": "production",
  "requests_per_second": 1000,
  "burst_limit": 2000
}
EOF

# Run with config
./admin -action=create-service -config=service-config.json
```

---

### 2. Create Merchant

**Purpose:** Register a new merchant (business entity) with EPX credentials

**Command:**
```bash
./admin -action=create-merchant
```

**Interactive Prompts:**
```
Enter Merchant Slug (e.g., downtown-pizza): downtown-pizza
Enter Merchant Name (e.g., Downtown Pizza LLC): Downtown Pizza LLC
Enter EPX CUST_NBR: 9001
Enter EPX MERCH_NBR: 900300
Enter EPX DBA_NBR: 2
Enter EPX TERMINAL_NBR: 77
Enter MAC Secret Path (e.g., /secrets/epx/prod/downtown-pizza): /secrets/epx/prod/downtown-pizza
Enter Environment (production/staging): production
Enter Tier (standard/premium/enterprise): standard
```

**Output:**
```
✅ Merchant created successfully!

Merchant Details:
  Merchant ID: 550e8400-e29b-41d4-a716-446655440000
  Slug: downtown-pizza
  Name: Downtown Pizza LLC
  Environment: production
  Tier: standard

EPX Credentials:
  CUST_NBR: 9001
  MERCH_NBR: 900300
  DBA_NBR: 2
  TERMINAL_NBR: 77

📝 Next Steps:
  1. Create a Service: ./admin -action=create-service
  2. Grant access: ./admin -action=grant-access
  3. Service uses RSA private key to sign JWT tokens

⚠️  IMPORTANT: Ensure MAC secret is stored in secret manager at:
    /secrets/epx/prod/downtown-pizza
```

**What Happens:**
1. Creates merchant record in database
2. Stores EPX credentials (CUST_NBR, MERCH_NBR, DBA_NBR, TERMINAL_NBR)
3. Stores **reference** to MAC secret (path in secret manager)
4. NO API keys generated (merchants don't authenticate directly)

**Using JSON Config:**
```bash
cat > merchant-config.json <<EOF
{
  "slug": "downtown-pizza",
  "name": "Downtown Pizza LLC",
  "cust_nbr": "9001",
  "merch_nbr": "900300",
  "dba_nbr": "2",
  "terminal_nbr": "77",
  "mac_secret_path": "/secrets/epx/prod/downtown-pizza",
  "environment": "production",
  "tier": "standard"
}
EOF

./admin -action=create-merchant -config=merchant-config.json
```

---

### 3. Grant Access

**Purpose:** Allow a service to access a merchant with specific permissions

**Command:**
```bash
./admin -action=grant-access
```

**Interactive Prompts:**
```
Enter Service ID: acme-pos-system
Enter Merchant Slug: downtown-pizza
Select scopes (comma-separated):
  payment:create    - Create payment transactions
  payment:read      - Read payment transactions
  payment:update    - Update payment status
  payment:refund    - Refund payments
  payment:void      - Void payments
  payment:capture   - Capture authorized payments
  subscription:manage - Manage subscriptions
  payment_method:manage - Manage saved payment methods

Enter scopes: payment:create,payment:read,payment:refund,payment:void
```

**Output:**
```
✅ Access granted successfully!

Access Details:
  Service: acme-pos-system (ACME POS System)
  Merchant: downtown-pizza (Downtown Pizza LLC)
  Scopes: [payment:create payment:read payment:refund payment:void]
  Granted At: 2025-11-21 14:30:00 UTC

The service can now:
  ✅ Create payments for this merchant
  ✅ Read payment transactions
  ✅ Refund payments
  ✅ Void payments
```

**What Happens:**
1. Links service to merchant in `service_merchants` table
2. Stores scopes (permissions)
3. Service can now generate JWT tokens for this merchant
4. Payment API validates service has required scopes

**Using JSON Config:**
```bash
cat > access-config.json <<EOF
{
  "service_id": "acme-pos-system",
  "merchant_slug": "downtown-pizza",
  "scopes": [
    "payment:create",
    "payment:read",
    "payment:refund",
    "payment:void"
  ]
}
EOF

./admin -action=grant-access -config=access-config.json
```

---

### 4. List Services

```bash
./admin -action=list-services
```

**Output:**
```
Services:
┌─────────────────────┬──────────────────────┬─────────────┬────────────┐
│ Service ID          │ Service Name         │ Environment │ Active     │
├─────────────────────┼──────────────────────┼─────────────┼────────────┤
│ acme-pos-system     │ ACME POS System      │ production  │ ✅ Yes     │
│ wordpress-plugin    │ WordPress Plugin     │ production  │ ✅ Yes     │
│ mobile-app          │ Mobile App           │ staging     │ ✅ Yes     │
└─────────────────────┴──────────────────────┴─────────────┴────────────┘
```

---

### 5. List Merchants

```bash
./admin -action=list-merchants
```

**Output:**
```
Merchants:
┌──────────────────┬──────────────────────┬─────────────┬────────┐
│ Slug             │ Name                 │ Environment │ Status │
├──────────────────┼──────────────────────┼─────────────┼────────┤
│ downtown-pizza   │ Downtown Pizza LLC   │ production  │ active │
│ main-street-cafe │ Main Street Cafe     │ production  │ active │
│ test-merchant    │ Test Merchant (Dev)  │ staging     │ active │
└──────────────────┴──────────────────────┴─────────────┴────────┘
```

---

### 6. Revoke Access

```bash
./admin -action=revoke-access
```

**Interactive Prompts:**
```
Enter Service ID: acme-pos-system
Enter Merchant Slug: downtown-pizza
Are you sure you want to revoke access? (yes/no): yes
```

**Output:**
```
✅ Access revoked successfully!

Service 'acme-pos-system' no longer has access to merchant 'downtown-pizza'
```

---

## Complete Workflow Example

### Scenario: Adding a New Restaurant

**1. Ensure MAC secret is in secret manager:**

```bash
# For GCP Secret Manager
echo -n "your-mac-secret-here" | gcloud secrets create epx-prod-downtown-pizza \
  --data-file=- \
  --replication-policy="automatic"

# For AWS Secrets Manager
aws secretsmanager create-secret \
  --name /secrets/epx/prod/downtown-pizza \
  --secret-string "your-mac-secret-here"

# For HashiCorp Vault
vault kv put secret/epx/prod/downtown-pizza value="your-mac-secret-here"

# For local file (development only)
mkdir -p secrets/epx/prod
echo "your-mac-secret-here" > secrets/epx/prod/downtown-pizza
chmod 600 secrets/epx/prod/downtown-pizza
```

**2. Create the merchant:**

```bash
./admin -action=create-merchant

# Enter details:
# - Slug: downtown-pizza
# - Name: Downtown Pizza LLC
# - CUST_NBR: 9001
# - MERCH_NBR: 900300
# - DBA_NBR: 2
# - TERMINAL_NBR: 77
# - MAC Secret Path: /secrets/epx/prod/downtown-pizza
# - Environment: production
```

**3. Create a service (if not exists):**

```bash
./admin -action=create-service

# Enter details:
# - Service ID: acme-pos-system
# - Service Name: ACME POS System
# - Environment: production

# ⚠️ SAVE THE PRIVATE KEY OUTPUT!
```

**4. Grant service access to merchant:**

```bash
./admin -action=grant-access

# Enter details:
# - Service ID: acme-pos-system
# - Merchant Slug: downtown-pizza
# - Scopes: payment:create,payment:read,payment:refund,payment:void
```

**5. Service generates JWT tokens:**

```go
// In your POS application:
import (
    "github.com/kevin07696/payment-service/internal/auth"
)

// Load private key (from environment or secure storage)
privateKeyPEM, _ := os.ReadFile("keys/acme-pos-system.pem")

// Create JWT manager
jwtManager, err := auth.NewJWTManager(
    privateKeyPEM,
    "acme-pos-system",  // service_id
    8 * time.Hour,       // token expiry
)

// Generate token for merchant
token, err := jwtManager.GenerateToken(
    merchantID,  // downtown-pizza merchant ID
    []string{"payment:create", "payment:read"},
)

// Use token in API requests
client := payment.NewPaymentServiceClient(conn)
ctx := metadata.AppendToOutgoingContext(
    context.Background(),
    "authorization", "Bearer "+token,
)
```

---

## Security Best Practices

### Private Key Storage

**❌ DON'T:**
- Store private keys in database
- Commit private keys to Git
- Share private keys via email/chat
- Store private keys unencrypted

**✅ DO:**
```bash
# 1. Save to file with restricted permissions
cat > keys/service-private.pem <<EOF
-----BEGIN RSA PRIVATE KEY-----
...
-----END RSA PRIVATE KEY-----
EOF
chmod 600 keys/service-private.pem

# 2. Add to .gitignore
echo "keys/*.pem" >> .gitignore

# 3. Use environment variables (never hardcode)
export PRIVATE_KEY_PATH=/path/to/keys/service-private.pem

# 4. For production: Use secret manager
# GCP:
gcloud secrets create service-private-key --data-file=keys/service-private.pem

# AWS:
aws secretsmanager create-secret \
  --name /keys/acme-pos-system \
  --secret-binary fileb://keys/service-private.pem

# Vault:
vault kv put secret/keys/acme-pos-system @keys/service-private.pem
```

### MAC Secret Storage

**❌ DON'T:**
- Store MAC secrets in database (only store the path/reference)
- Store MAC secrets in environment variables
- Hardcode MAC secrets in code

**✅ DO:**
```bash
# Store in secret manager (examples above)
# Database stores ONLY the reference:
mac_secret_path = "/secrets/epx/prod/downtown-pizza"

# Application retrieves at runtime:
macSecret, err := secretManager.GetSecret(ctx, merchant.MacSecretPath)
```

### Environment Variables

Required for admin CLI:

```bash
# Database connection
export DATABASE_URL="postgres://user:pass@localhost:5432/payments?sslmode=require"

# Secret manager configuration
export SECRET_MANAGER=gcp                    # or "aws", "vault", "local"
export GCP_PROJECT_ID=my-project-id          # for GCP
export AWS_REGION=us-east-1                  # for AWS
export VAULT_ADDR=https://vault.example.com  # for Vault
export LOCAL_SECRETS_BASE_PATH=/secrets      # for local files
```

---

## Architecture: Services vs Merchants

**Services** = Technical integrations (how payments are made)
- POS systems, e-commerce backends, mobile apps
- Authenticate using RSA keypairs (JWT tokens)
- Public key stored in database, private key kept by service owner
- Granted access to specific merchants

**Merchants** = Business entities (who receives payments)
- Restaurants, stores, organizations
- Store EPX credentials (CUST_NBR, MERCH_NBR, DBA_NBR, TERMINAL_NBR)
- NO authentication credentials (don't call APIs directly)
- Accessed by services that have been granted permission

**Access Control:**
- `service_merchants` junction table links services to merchants
- Scopes define what operations are allowed
- JWT tokens carry service_id, merchant_id, and scopes
- Payment API validates service has required scopes before processing

**Example:**
```
┌─────────────────────┐
│ ACME POS System     │ ← Service (has private key)
│ (Service)           │
└──────────┬──────────┘
           │
           │ service_merchants table:
           │ scopes = [payment:create, payment:read, payment:refund]
           │
           ↓
┌─────────────────────┐
│ Downtown Pizza LLC  │ ← Merchant (has EPX credentials)
│ (Merchant)          │
└─────────────────────┘
```

---

## Troubleshooting

### "Private key already exists for this service"

**Problem:** Trying to create a service that already exists

**Solution:**
```bash
# List existing services
./admin -action=list-services

# Either:
# 1. Use existing service (you should have saved the private key)
# 2. Delete old service and recreate (requires database access):
psql $DATABASE_URL -c "DELETE FROM services WHERE service_id = 'my-service'"
```

### "MAC secret not found in secret manager"

**Problem:** The path in `mac_secret_path` doesn't exist in secret manager

**Solution:**
```bash
# Check what's stored in database:
psql $DATABASE_URL -c "SELECT slug, mac_secret_path FROM merchants WHERE slug = 'my-merchant'"

# Create the secret:
# See "Ensure MAC secret is in secret manager" section above
```

### "Service does not have access to merchant"

**Problem:** JWT token validation fails because service isn't granted access

**Solution:**
```bash
# Grant access:
./admin -action=grant-access
# Enter service_id and merchant_slug

# Verify access:
psql $DATABASE_URL -c "
  SELECT s.service_id, m.slug, sm.scopes
  FROM service_merchants sm
  JOIN services s ON s.id = sm.service_id
  JOIN merchants m ON m.id = sm.merchant_id
  WHERE s.service_id = 'my-service' AND m.slug = 'my-merchant'
"
```

### "Failed to validate JWT signature"

**Problem:** Public key in database doesn't match private key used to sign token

**Causes:**
1. Using wrong private key
2. Service was recreated but client still has old private key
3. Public key corrupted in database

**Solution:**
```bash
# Verify public key fingerprint:
psql $DATABASE_URL -c "SELECT service_id, public_key_fingerprint FROM services WHERE service_id = 'my-service'"

# Compare with your private key:
openssl rsa -in keys/my-service.pem -pubout | openssl dgst -sha256

# If mismatch, recreate service and update client with new private key
```

---

## Database Schema Reference

### services table
```sql
CREATE TABLE services (
    id UUID PRIMARY KEY,
    service_id VARCHAR(100) UNIQUE NOT NULL,
    service_name VARCHAR(200) NOT NULL,
    public_key TEXT NOT NULL,
    public_key_fingerprint VARCHAR(64) NOT NULL,
    environment VARCHAR(20) NOT NULL,
    requests_per_second INTEGER DEFAULT 1000,
    burst_limit INTEGER DEFAULT 2000,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    created_by UUID REFERENCES admins(id)
);
```

### merchants table
```sql
CREATE TABLE merchants (
    id UUID PRIMARY KEY,
    slug VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(200) NOT NULL,
    cust_nbr VARCHAR(50) NOT NULL,        -- EPX credentials
    merch_nbr VARCHAR(50) NOT NULL,
    dba_nbr VARCHAR(50) NOT NULL,
    terminal_nbr VARCHAR(50) NOT NULL,
    mac_secret_path VARCHAR(500) NOT NULL,  -- Secret manager path
    environment VARCHAR(20) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    status VARCHAR(20) DEFAULT 'active',
    tier VARCHAR(20) DEFAULT 'standard',
    created_at TIMESTAMP DEFAULT NOW(),
    created_by UUID REFERENCES admins(id)
);
```

### service_merchants junction table
```sql
CREATE TABLE service_merchants (
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    scopes TEXT[] NOT NULL,
    granted_at TIMESTAMP DEFAULT NOW(),
    granted_by UUID REFERENCES admins(id),
    PRIMARY KEY (service_id, merchant_id)
);
```

---

## Related Documentation

- **[AUTH.md](../development/AUTH.md)** - Complete authentication architecture and JWT validation
- **[SETUP.md](../development/SETUP.md)** - Secret manager configuration and environment setup
- **[API_SPECS.md](./API_SPECS.md)** - API authentication requirements
- **Migration 008_auth_tables.sql** - Database schema for authentication system
