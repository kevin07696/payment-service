# Authentication and Authorization

**Target Audience:** Developers implementing auth, client applications, and security reviewers
**Topic:** Token-based authentication, authorization patterns, and multi-tenant access control
**Goal:** Secure API access with proper merchant isolation and customer privacy

---

## Overview

The payment service uses **token-based authentication** with **scope-based authorization**:

- **Authentication:** JWT tokens signed with RSA keys, issued by registered services
- **Authorization:** Scope-based permissions with merchant isolation
- **Multi-Tenant:** Each request is scoped to a specific merchant
- **Idempotency:** Duplicate prevention through unique keys

**Key Principles:**
- Services authenticate using RSA-signed JWT tokens
- Tokens carry context (service_id, merchant_id, scopes)
- Always return 404 (never 403) to prevent enumeration attacks
- All authorization decisions logged for audit

---

## Architecture: Services vs Merchants

### Core Concepts

The payment service separates **business entities (merchants)** from **API access (services)**:

```
┌─────────────────────────────────────────────────────────────┐
│ SERVICES (Apps/Integrations)                                │
│ - POS systems, e-commerce backends, mobile apps             │
│ - Authenticate using RSA keypairs (JWT tokens)              │
│ - Public key stored in database                             │
│ - Private key returned ONCE, stored by service owner        │
│ - Granted access to specific merchants via scopes           │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ service_merchants junction table
                              │ (scopes: payments:create, payments:read, etc.)
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ MERCHANTS (Business Entities)                               │
│ - Restaurants, stores, organizations                        │
│ - Store EPX credentials ONLY (gateway access)               │
│ - NO API keys or authentication credentials                 │
│ - Pure business data (name, tier, rate limits)              │
└─────────────────────────────────────────────────────────────┘
```

### Database Tables

**services table:**
```sql
CREATE TABLE services (
    id UUID PRIMARY KEY,
    service_id VARCHAR(100) UNIQUE NOT NULL,  -- 'acme-pos-system'
    service_name VARCHAR(200) NOT NULL,       -- 'ACME POS System'
    public_key TEXT NOT NULL,                 -- RSA public key (PEM)
    public_key_fingerprint VARCHAR(64) NOT NULL,
    environment VARCHAR(20) NOT NULL,         -- 'production', 'staging'
    requests_per_second INTEGER,
    burst_limit INTEGER,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);
```

**merchants table:**
```sql
CREATE TABLE merchants (
    id UUID PRIMARY KEY,
    slug VARCHAR(100) UNIQUE NOT NULL,        -- 'downtown-pizza'
    name VARCHAR(200) NOT NULL,               -- 'Downtown Pizza LLC'
    cust_nbr VARCHAR(50) NOT NULL,            -- EPX credentials
    merch_nbr VARCHAR(50) NOT NULL,
    dba_nbr VARCHAR(50) NOT NULL,
    terminal_nbr VARCHAR(50) NOT NULL,
    mac_secret_path VARCHAR(500) NOT NULL,    -- Path to MAC secret file
    environment VARCHAR(20) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    status VARCHAR(20) DEFAULT 'active',
    tier VARCHAR(20) DEFAULT 'standard'
);
```

**service_merchants junction table:**
```sql
CREATE TABLE service_merchants (
    service_id UUID REFERENCES services(id),
    merchant_id UUID REFERENCES merchants(id),
    scopes TEXT[] NOT NULL,                   -- ['payments:create', 'payments:read']
    granted_at TIMESTAMP DEFAULT NOW(),
    granted_by UUID,                          -- Admin who granted access
    PRIMARY KEY (service_id, merchant_id)
);
```

### Admin CLI Workflow

**1. Create a Service (POS system, e-commerce backend, etc.):**
```bash
./admin -action=create-service
```

This generates:
- RSA keypair (2048-bit)
- Stores public key in database
- Returns private key **ONCE** (save it!)
- Service uses private key to sign JWT tokens

**2. Create a Merchant (business entity):**
```bash
./admin -action=create-merchant
```

This creates:
- Merchant record with EPX credentials
- NO API keys generated (merchants don't authenticate)
- Merchant is pure business data

**3. Grant Service Access to Merchant:**
```bash
./admin -action=grant-access
```

This creates:
- Link between service and merchant
- Scopes defining what service can do
- Service can now create payments for this merchant

---

## JWT Token Structure

### Token Claims

Services generate JWT tokens with the following claims:

```json
{
  "iss": "acme-pos-system",
  "sub": "acme-pos-system",
  "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
  "scopes": ["payments:create", "payments:read", "payments:refund"],
  "exp": 1736683200,
  "iat": 1736679600,
  "nbf": 1736679600,
  "jti": "unique-request-id-uuid"
}
```

| Claim | Required | Description |
|-------|----------|-------------|
| `iss` | Yes | Issuer - the service_id |
| `sub` | Yes | Subject - the service_id |
| `merchant_id` | Yes | Single merchant UUID for this request |
| `scopes` | Yes | Array of permission scopes |
| `exp` | Yes | Expiration timestamp |
| `iat` | Yes | Issued at timestamp |
| `nbf` | Yes | Not before timestamp |
| `jti` | Yes | Unique JWT ID (prevents replay) |

### Available Scopes

| Scope | Description |
|-------|-------------|
| `payments:create` | Create payments (auth, capture, sale) |
| `payments:read` | View transactions |
| `payments:void` | Void payments |
| `payments:refund` | Issue refunds |
| `payment_methods:read` | View saved payment methods |
| `payment_methods:create` | Store payment methods |
| `storage:tokenize` | Create secure tokens |
| `storage:detokenize` | Retrieve tokenized data |
| `subscriptions:manage` | Create/update/cancel subscriptions |
| `subscriptions:read` | View subscriptions |
| `*` | Wildcard - all permissions |

### Why Single merchant_id Instead of Array?

The simplified design uses a single `merchant_id` per token because:

1. **Clear ownership**: Each API request is unambiguously for one merchant
2. **Simpler validation**: No need to check arrays or resolve which merchant
3. **Better audit trails**: Every request has exactly one merchant context
4. **Services act as intermediaries**: Services that manage multiple merchants generate a new token per merchant when needed

---

## Authentication Flow

```
┌──────────────┐
│ POS App      │
│ (Service)    │
└──────┬───────┘
       │
       │ 1. Sign JWT with RSA private key
       │    Claims: { iss, sub, merchant_id, scopes }
       │
       ↓
┌──────────────────────────────────────────────────┐
│ Payment Service API                              │
│                                                  │
│ 2. Extract JWT from Authorization header         │
│ 3. Verify signature using service's public key   │
│ 4. Validate claims (exp, merchant_id, scopes)    │
│ 5. Check service_merchants for access rights     │
│ 6. Process payment via EPX gateway               │
└──────────────────────────────────────────────────┘
```

### Request Flow

```typescript
// Service generates token for specific merchant
const token = jwtgen.generate({
  merchantId: "550e8400-e29b-41d4-a716-446655440000",
  scopes: ["payments:create", "payments:read"],
  expiresIn: "1h"
});

// API call with token
const response = await paymentClient.Sale({
  amount_cents: 10000,
  payment_token: "tok_xyz",
  customer_id: "cust_123",  // Customer passed in request body
  idempotency_key: "sale_unique_key"
}, {
  headers: {
    'Authorization': `Bearer ${token}`
  }
});
```

### Customer ID Handling

**Important**: Customer ID is passed in the request body, NOT in the JWT token.

**Why?**
- Services act as trusted intermediaries between customers and the payment system
- A service may process payments for many customers using the same merchant
- Customer context is per-request, not per-authentication session
- Keeps tokens reusable across customer interactions

```go
// Request structure
type SaleRequest struct {
    AmountCents    int64  `json:"amount_cents"`
    PaymentToken   string `json:"payment_token"`
    CustomerID     string `json:"customer_id"`     // Customer in request
    IdempotencyKey string `json:"idempotency_key"`
}

// Token only contains service context
// JWT Claims: { merchant_id, scopes, ... }
```

---

## jwtgen CLI Tool

For local development and testing, use the `jwtgen` CLI tool to generate tokens:

### Build

```bash
go build -o bin/jwtgen ./cmd/jwtgen
```

### Usage

```bash
# Basic usage - generate token with default scopes
./bin/jwtgen -c service_acme_credentials.json -m "550e8400-e29b-41d4-a716-446655440000"

# Custom expiry and scopes
./bin/jwtgen -c creds.json -m "uuid" -e 30m -s "payments:create,payments:read"

# Output as curl command
./bin/jwtgen -c creds.json -m "uuid" -o curl

# JSON output with metadata
./bin/jwtgen -c creds.json -m "uuid" -o json

# Show decoded claims for verification
./bin/jwtgen -c creds.json -m "uuid" --decode
```

### Options

| Flag | Description |
|------|-------------|
| `-c, --credentials` | Path to service credentials JSON file (required) |
| `-m, --merchant-id` | Merchant UUID (required) |
| `-e, --expires` | Token expiry duration (default: 1h) |
| `-s, --scopes` | Comma-separated scopes (default: all payment scopes) |
| `-o, --output` | Output format: token, json, curl (default: token) |
| `--decode` | Show decoded claims for verification |
| `-h, --help` | Show help message |

---

## Authorization Logic

### Scope Validation

```go
// Check if scopes include required permission
func HasScope(scopes []string, scope string) bool {
    for _, s := range scopes {
        if s == scope || s == "*" {  // Wildcard grants all
            return true
        }
    }
    return false
}

// Handler validates scope before operation
func (h *PaymentHandler) Sale(ctx context.Context, req *SaleRequest) (*Transaction, error) {
    claims := auth.GetClaimsFromContext(ctx)

    if !domain.HasScope(claims.Scopes, domain.ScopePaymentsCreate) {
        return nil, connect.NewError(connect.CodePermissionDenied,
            errors.New("missing scope: payments:create"))
    }

    // Process payment...
}
```

### Merchant Access Validation

```go
// Validate service has access to merchant
func (a *AuthInterceptor) ValidateMerchantAccess(
    ctx context.Context,
    serviceID, merchantID string,
) error {
    // Check service_merchants junction table
    access, err := a.db.GetServiceMerchantAccess(ctx, serviceID, merchantID)
    if err != nil {
        return connect.NewError(connect.CodeNotFound, errors.New("not found"))
    }

    if !access.IsActive {
        return connect.NewError(connect.CodeNotFound, errors.New("not found"))
    }

    return nil
}
```

### Authorization Error Handling

**Always return 404, never 403:**

```go
// CORRECT: Return 404 for unauthorized access
if err := h.authz.CanAccessMerchant(ctx, merchantID); err != nil {
    h.logger.Warn("unauthorized access attempt",
        zap.String("service_id", claims.ServiceID),
        zap.String("merchant_id", merchantID),
    )
    return nil, connect.NewError(connect.CodeNotFound, errors.New("not found"))
}

// WRONG: Never expose authorization failures
// return nil, connect.NewError(connect.CodePermissionDenied, errors.New("access denied"))
```

**Why 404 instead of 403?**
- Prevents enumeration attacks
- Attacker cannot distinguish "doesn't exist" from "exists but unauthorized"
- Industry standard (AWS, Stripe, PayPal, etc.)

---

## Idempotency

### What is Idempotency?

**Idempotency ensures that retrying the same request multiple times produces the same result as making it once**, preventing duplicate charges.

### How It Works

```
┌───────────────────────────────────────────────┐
│ 1. Client generates idempotency_key (UUID)   │
│ 2. Payment service checks if key exists       │
│ 3. IF EXISTS: Return existing transaction    │
│ 4. IF NEW: Process payment and save           │
└───────────────────────────────────────────────┘
```

### Database Implementation

```sql
-- Transactions table
CREATE TABLE transactions (
    id UUID PRIMARY KEY,  -- Idempotency key becomes PK
    parent_transaction_id UUID NOT NULL,
    status VARCHAR(20) GENERATED ALWAYS AS (
        CASE WHEN auth_resp = '00' THEN 'approved' ELSE 'declined' END
    ) STORED,
    -- ...
);

-- Insert with conflict handling
INSERT INTO transactions (id, parent_transaction_id, ...)
VALUES ($1, $2, ...)
ON CONFLICT (id) DO NOTHING  -- Returns no rows if key exists
RETURNING *;
```

### When to Insert Transactions

| Scenario | Gateway Response | Insert Transaction? | Retryable with Same Key? |
|----------|------------------|---------------------|--------------------------|
| **Network timeout** | `err != nil` | No | Yes |
| **Gateway 500 error** | `err != nil` | No | Yes |
| **Payment approved** | `auth_resp="00"` | Yes | No |
| **Payment declined** | `auth_resp="05"` | Yes | No |

### Idempotency Key Generation

```typescript
// CORRECT: Unique key per attempt
function generateIdempotencyKey(operation: string): string {
  return `${operation}_${Date.now()}_${uuidv4()}`;
}

// Usage
const saleKey = generateIdempotencyKey('sale');
// Result: "sale_1736683200000_550e8400-e29b-41d4-a716-446655440000"

// WRONG: Reusing key for retries after decline
const key = generateIdempotencyKey('sale');
await paymentClient.sale({ idempotency_key: key });  // Declined
await paymentClient.sale({ idempotency_key: key });  // Returns existing decline

// CORRECT: New key after decline
const key1 = generateIdempotencyKey('sale');
await paymentClient.sale({ idempotency_key: key1 });  // Declined

const key2 = generateIdempotencyKey('sale');  // NEW key
await paymentClient.sale({ idempotency_key: key2 });  // New attempt
```

---

## Security Best Practices

### 1. Token Security

**Short-Lived Tokens:**
```typescript
// Typical: 1 hour
exp: Math.floor(Date.now() / 1000) + (1 * 3600)

// High security: 5-15 minutes
exp: Math.floor(Date.now() / 1000) + (5 * 60)
```

**Token Storage:**
- Store private keys in secure secret management (Vault, AWS Secrets Manager)
- Never commit private keys to version control
- Use environment variables for key paths

### 2. Rate Limiting

```go
var rateLimits = map[string]RateLimit{
    "Sale":             {Requests: 100, Window: time.Minute},
    "Authorize":        {Requests: 100, Window: time.Minute},
    "ListTransactions": {Requests: 10, Window: time.Minute},
}
```

### 3. Audit Logging

```go
// Log all authorization decisions
s.auditLogger.Log(AuditEvent{
    EventType:  "authorization_check",
    ServiceID:  claims.ServiceID,
    MerchantID: claims.MerchantID,
    Resource:   "transaction",
    ResourceID: tx.ID,
    Action:     "create",
    Allowed:    allowed,
    Scopes:     claims.Scopes,
    IPAddress:  ctx.Value("remote_addr").(string),
    Timestamp:  time.Now(),
})
```

### 4. Input Validation

```go
// Validate token claims
if claims.MerchantID == "" {
    return errors.New("merchant_id is required in token")
}

// Validate merchant_id is valid UUID
if _, err := uuid.Parse(claims.MerchantID); err != nil {
    return errors.New("invalid merchant_id format")
}

// Validate scopes not empty
if len(claims.Scopes) == 0 {
    return errors.New("scopes are required in token")
}
```

---

## Integration Examples

### POS Application

```typescript
// 1. Service has credentials from admin setup
const credentials = loadCredentials('service_pos_credentials.json');

// 2. Generate token for merchant
const token = await generateToken(credentials, {
  merchantId: 'merchant-uuid',
  scopes: ['payments:create', 'payments:refund'],
  expiresIn: '1h'
});

// 3. Process payment
const payment = await paymentClient.Sale({
  amount_cents: 4599,
  payment_token: 'tok_card_123',
  customer_id: 'walk_in_customer',
  idempotency_key: generateKey(),
}, {
  headers: { 'Authorization': `Bearer ${token}` },
});
```

### E-commerce Application

```typescript
// 1. Customer places order on e-commerce site
// 2. E-commerce backend (service) processes payment

const token = await generateToken(credentials, {
  merchantId: 'ecom-merchant-uuid',
  scopes: ['payments:create', 'payments:read'],
  expiresIn: '15m'
});

// 3. Create payment with customer context in request body
const payment = await paymentClient.Sale({
  amount_cents: 9999,
  payment_token: 'tok_saved_card',
  customer_id: order.customerId,  // Customer passed in request
  idempotency_key: `order_${order.id}`,
}, {
  headers: { 'Authorization': `Bearer ${token}` },
});

// 4. Customer can view their order
// E-commerce backend filters by customer_id when querying
const customerOrders = await db.query(
  'SELECT * FROM orders WHERE customer_id = $1',
  [customerId]
);
```

---

## Cron Authentication

The payment service exposes internal cron endpoints for scheduled tasks (ACH verification, billing processing, dispute sync, etc.). These endpoints use a **separate authentication mechanism** from the main API.

### Authentication Method

Cron endpoints authenticate using the **X-Cron-Secret** header only:

```bash
# Correct: Use X-Cron-Secret header
curl -X POST http://localhost:8081/cron/verify-ach \
  -H "X-Cron-Secret: your-cron-secret"

# Incorrect: Bearer tokens are NOT supported for cron endpoints
# curl -X POST http://localhost:8081/cron/verify-ach \
#   -H "Authorization: Bearer your-cron-secret"  # Will return 401
```

### Why X-Cron-Secret Instead of JWT?

| Aspect | JWT (API) | X-Cron-Secret (Cron) |
|--------|-----------|---------------------|
| **Use Case** | External API calls | Internal scheduled tasks |
| **Authentication** | RSA-signed tokens | Shared secret |
| **Caller** | Services/Applications | Cloud Scheduler/Cron jobs |
| **Complexity** | Full JWT validation | Simple header check |
| **Rate Limiting** | Per-service limits | Single cron job |

### Cron Endpoints

All cron endpoints are served on port **8081** (not 8080):

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/cron/verify-ach` | POST | Process pending ACH verifications |
| `/cron/process-billing` | POST | Process subscription billing |
| `/cron/sync-disputes` | POST | Sync disputes from gateway |
| `/cron/cleanup-audit-logs` | POST | Clean up old audit logs |
| `/cron/cleanup-rate-limits` | POST | Clean up expired rate limit buckets |
| `/cron/stats` | GET | Get billing cron statistics |
| `/cron/ach/stats` | GET | Get ACH verification statistics |

### Health Check Endpoints (No Auth Required)

Health endpoints are accessible without authentication for monitoring:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/cron/health` | GET | Main cron health check |
| `/cron/ach/health` | GET | ACH verification health |
| `/cron/audit/health` | GET | Audit cleanup health |
| `/cron/rate-limit/health` | GET | Rate limit cleanup health |

### Configuration

Set the cron secret via environment variable:

```bash
# Production: Use a strong, randomly generated secret (32+ characters)
export CRON_SECRET="your-secure-random-secret-at-least-32-chars"

# Development: Default value for local testing
# CRON_SECRET="test-cron-secret-at-least-32-characters-long"
```

### Security Considerations

1. **Use HTTPS in production**: The cron secret is sent in headers, so always use TLS
2. **Rotate secrets regularly**: Change the cron secret periodically
3. **Restrict network access**: Cron endpoints should only be accessible from your scheduler (Cloud Scheduler, cron server)
4. **Monitor for abuse**: Log and alert on failed authentication attempts

---

## Related Documentation

- **TOKEN_GENERATION.md** - Token generation code examples for all languages
- **API_SPECS.md** - API endpoints and authentication requirements
- **DATABASE.md** - Multi-tenant data isolation
- **DEVELOP.md** - Testing authentication and authorization
