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

**✨ Authentication-Free CLI:**
- No admin accounts or login required
- No authentication checks
- Direct database access for administrative operations
- Designed as a local development/operations tool

---

## Prerequisites

Before using the admin CLI, you need:
1. Payment service database initialized with migrations
2. Database connection URL (via `-db` flag or `DATABASE_URL` environment variable)

That's it! No authentication setup required.

---

## Building the CLI

```bash
# Build the admin CLI
go build -o admin ./cmd/admin

# Verify it works
./admin -help
```

**Windows (PowerShell):**
```powershell
go build -o admin.exe .\cmd\admin
.\admin.exe -help
```

**Flags:**
```
Usage of ./admin:
  -action string
        Action to perform: create-service, create-merchant, grant-access
  -db string
        Database URL (default: postgres://postgres:postgres@localhost:5432/payments?sslmode=disable)
  -json string
        JSON file with service/merchant details
```

**Quick Start:**
```bash
# Create a service
./admin -action=create-service -json=service.json

# Create a merchant
./admin -action=create-merchant -json=merchant.json

# Grant access (interactive prompts)
./admin -action=grant-access
```

---

## Actions

### 1. Create Service

**Purpose:** Register a new service (POS system, e-commerce backend, mobile app)

**Using JSON Config:**
```bash
# Create service.json
cat > service.json <<EOF
{
  "service_id": "acme-pos-system",
  "service_name": "ACME POS System",
  "environment": "production",
  "generate_keypair": true
}
EOF

# Run with config
./admin -action=create-service -json=service.json
```

**Output:**
```
========================================
✅ SERVICE CREATED SUCCESSFULLY
========================================
Service ID: acme-pos-system
Service Name: ACME POS System
Environment: production
Rate Limit: 0 req/s (burst: 0)

📁 Credentials saved to: service_acme-pos-system_credentials.json
⚠️  Keep the private key secure!
========================================
```

**Generated Credentials File** (`service_acme-pos-system_credentials.json`):
```json
{
  "service_id": "acme-pos-system",
  "service_name": "ACME POS System",
  "environment": "production",
  "private_key": "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----\n",
  "public_key": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----\n",
  "note": "Keep the private key secure! Use it to sign JWT tokens."
}
```

**What Happens:**
1. Generates 2048-bit RSA keypair
2. Stores **public key** in database (`services` table)
3. Returns **private key** in credentials JSON file (never stored in database)
4. Service uses private key to sign JWT tokens

**⚠️ CRITICAL SECURITY:**
```bash
# 1. Save credentials file securely
chmod 600 service_acme-pos-system_credentials.json

# 2. Extract private key for use
jq -r '.private_key' service_acme-pos-system_credentials.json > keys/acme-pos-system.pem
chmod 600 keys/acme-pos-system.pem

# 3. Add to .gitignore
echo "keys/*.pem" >> .gitignore
echo "service_*_credentials.json" >> .gitignore

# 4. Use in application
export PRIVATE_KEY_PATH=keys/acme-pos-system.pem
```

---

### 2. Create Merchant

**Purpose:** Register a new merchant (business entity) with EPX credentials

**Using JSON Config:**
```bash
# Create merchant.json
cat > merchant.json <<EOF
{
  "name": "Downtown Pizza LLC",
  "slug": "downtown-pizza",
  "payment_gateway": "epx",
  "is_active": true
}
EOF

# Run with config
./admin -action=create-merchant -json=merchant.json
```

**Output:**
```
========================================
✅ MERCHANT CREATED SUCCESSFULLY
========================================
Merchant ID: 550e8400-e29b-41d4-a716-446655440000
Slug: downtown-pizza
Name: Downtown Pizza LLC
Environment:
Tier:
Rate Limit: 0 req/s

📝 Next Steps:
  1. Create a Service: ./admin -action=create-service
  2. Grant access: ./admin -action=grant-access
  3. Service uses RSA private key to sign JWT tokens

📁 Info saved to: merchant_downtown-pizza_info.json
========================================
```

**What Happens:**
1. Creates merchant record in database
2. Stores merchant information (name, slug, payment gateway)
3. Merchant is now available for service access grants
4. NO API keys generated (merchants don't authenticate directly)

**Merchant Info File** (`merchant_downtown-pizza_info.json`):
```json
{
  "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
  "slug": "downtown-pizza",
  "name": "Downtown Pizza LLC",
  "payment_gateway": "epx",
  "is_active": true
}
```

---

### 3. Grant Access

**Purpose:** Allow a service to access a merchant with specific permissions

**Command (Interactive):**
```bash
# Using stdin pipe for automation
echo -e "acme-pos-system\ndowntown-pizza" | ./admin -action=grant-access

# Or run interactively
./admin -action=grant-access
```

**Interactive Prompts:**
```
Service ID (e.g., wordpress-plugin): acme-pos-system
Merchant slug: downtown-pizza
```

**Output:**
```
Granting scopes: [payment:create payment:read payment:update payment:refund subscription:manage payment_method:manage]

✅ Access granted successfully!
Service 'acme-pos-system' now has access to merchant 'downtown-pizza'
```

**What Happens:**
1. Links service to merchant in `service_merchants` table
2. Grants default scopes for all payment operations
3. Service can now generate JWT tokens for this merchant
4. Payment API validates service has required scopes

**Default Scopes Granted:**
- `payment:create` - Create payment transactions
- `payment:read` - Read payment transactions
- `payment:update` - Update payment status
- `payment:refund` - Refund payments
- `subscription:manage` - Manage subscriptions
- `payment_method:manage` - Manage saved payment methods

---

## Complete Workflow Example

### Scenario: Setting Up a New POS System for a Restaurant

**Step 1: Create the Service**

```bash
# Create service configuration
cat > service.json <<EOF
{
  "service_id": "downtown-pos",
  "service_name": "Downtown POS System",
  "environment": "production",
  "generate_keypair": true
}
EOF

# Create the service
./admin -action=create-service -json=service.json

# Output: service_downtown-pos_credentials.json created
# Extract and secure the private key
jq -r '.private_key' service_downtown-pos_credentials.json > keys/downtown-pos.pem
chmod 600 keys/downtown-pos.pem
```

**Step 2: Create the Merchant**

```bash
# Create merchant configuration
cat > merchant.json <<EOF
{
  "name": "Downtown Pizza LLC",
  "slug": "downtown-pizza",
  "payment_gateway": "epx",
  "is_active": true
}
EOF

# Create the merchant
./admin -action=create-merchant -json=merchant.json

# Output: merchant_downtown-pizza_info.json created
```

**Step 3: Grant Service Access to Merchant**

```bash
# Grant access (interactive)
echo -e "downtown-pos\ndowntown-pizza" | ./admin -action=grant-access

# Output: Access granted with all default scopes
```

**Step 4: Use in Application**

```go
// In your POS application:
import (
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "github.com/golang-jwt/jwt/v5"
    "os"
    "time"
)

// Load private key
privateKeyPEM, _ := os.ReadFile("keys/downtown-pos.pem")
block, _ := pem.Decode(privateKeyPEM)
privateKey, _ := x509.ParsePKCS1PrivateKey(block.Bytes)

// Generate JWT token
token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
    "iss": "downtown-pos",           // service_id
    "sub": merchantID,                // downtown-pizza merchant UUID
    "merchant_id": merchantID,
    "service_id": "downtown-pos",
    "scopes": []string{"payment:create", "payment:read"},
    "exp": time.Now().Add(8 * time.Hour).Unix(),
    "iat": time.Now().Unix(),
})

tokenString, _ := token.SignedString(privateKey)

// Use token in API requests
// Authorization: Bearer <tokenString>
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
    created_at TIMESTAMP DEFAULT NOW()
);
```

### merchants table
```sql
CREATE TABLE merchants (
    id UUID PRIMARY KEY,
    slug VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(200) NOT NULL,
    payment_gateway VARCHAR(50) NOT NULL,
    environment VARCHAR(20),
    is_active BOOLEAN DEFAULT true,
    status VARCHAR(20) DEFAULT 'active',
    tier VARCHAR(20) DEFAULT 'standard',
    created_at TIMESTAMP DEFAULT NOW()
);
```

### service_merchants junction table
```sql
CREATE TABLE service_merchants (
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    scopes TEXT[] NOT NULL,
    granted_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    PRIMARY KEY (service_id, merchant_id)
);
```

**Note:** Audit columns (`created_by`, `granted_by`, `approved_by`) have been removed for simplicity. The admin CLI operates without authentication or audit trail.

---

## Related Documentation

- **[AUTH.md](../development/AUTH.md)** - Complete authentication architecture and JWT validation
- **[SETUP.md](../development/SETUP.md)** - Secret manager configuration and environment setup
- **[API_SPECS.md](./API_SPECS.md)** - API authentication requirements
- **Migration 008_auth_tables.sql** - Database schema for authentication system
