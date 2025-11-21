# Authentication and Authorization

**Target Audience:** Developers implementing auth, client applications, and security reviewers
**Topic:** Token-based authentication, authorization patterns, and multi-tenant access control
**Goal:** Secure API access with proper merchant isolation and customer privacy

---

## Overview

The payment service uses **token-based authentication** with **role-based authorization**:

- **Authentication:** JWT tokens issued by client applications (POS, e-commerce backends)
- **Authorization:** Role-based access control (RBAC) with merchant isolation
- **Multi-Tenant:** Each request is scoped to specific merchant(s)
- **Idempotency:** Duplicate prevention through unique keys

**Key Principles:**
- No passwords stored in payment service (authentication happens in client applications)
- Tokens carry context (who, what merchants, what permissions)
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
                              │ (scopes: payment:create, payment:read, etc.)
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
    scopes TEXT[] NOT NULL,                   -- ['payment:create', 'payment:read']
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

### Authentication Flow

```
┌──────────────┐
│ POS App      │
│ (Service)    │
└──────┬───────┘
       │
       │ 1. Sign JWT with RSA private key
       │    Claims: { service_id, merchant_id, scopes }
       │
       ↓
┌──────────────────────────────────────────────────────┐
│ Payment Service API                                  │
│                                                      │
│ 2. Verify JWT signature using public key            │
│ 3. Check service_merchants for access               │
│ 4. Validate scopes                                   │
│ 5. Fetch merchant's EPX credentials                 │
│ 6. Process payment via EPX gateway                  │
└──────────────────────────────────────────────────────┘
```

### Why This Architecture?

**Separation of Concerns:**
- Merchants = Business entities (what you charge)
- Services = Technical integrations (how you charge)

**Security:**
- Service compromise doesn't expose merchant EPX credentials
- Each service has limited scopes (principle of least privilege)
- Private keys never stored in database
- Easy to rotate service keys without touching merchants

**Flexibility:**
- One service can access multiple merchants (franchise POS)
- One merchant can be accessed by multiple services (POS + web)
- Grant/revoke access without recreating entities

**Audit:**
- Track which service performed which action
- service_merchants table logs access grants
- Easy to trace payment operations to specific services

---

## Quick Start

### Issuing Tokens (Client Application)

**POS Backend** (single merchant):

```typescript
const token = jwt.sign({
  sub: 'pos_terminal_001',
  token_type: 'merchant',
  merchant_ids: ['merchant_abc123'],  // Single merchant array
  scopes: ['payments:create', 'payments:read', 'payments:refund'],
  exp: Math.floor(Date.now() / 1000) + (8 * 3600), // 8 hours
}, JWT_SECRET);
```

**E-commerce Backend** (customer):

```typescript
const token = jwt.sign({
  sub: customer.id,
  token_type: 'customer',
  merchant_ids: [],  // Empty (customers don't own merchants)
  customer_id: customer.id,
  scopes: ['payments:read', 'payment_methods:read'],
  exp: Math.floor(Date.now() / 1000) + (24 * 3600),
}, JWT_SECRET);
```

### Making Authenticated Requests

```typescript
// ConnectRPC request with token
const response = await paymentClient.Sale({
  amount_cents: 10000, // $100.00 in cents
  payment_method_id: 'pm_123',
  idempotency_key: generateIdempotencyKey('sale'),
}, {
  metadata: {
    'authorization': `Bearer ${token}`,
  },
});
```

---

## Token Structure

### JWT Claims

```typescript
interface TokenClaims {
  // Standard JWT claims
  sub: string;        // Token subject (unique ID)
  iss: string;        // Issuer (client application)
  exp: number;        // Expiration timestamp
  iat: number;        // Issued at timestamp

  // Authorization context
  token_type: 'merchant' | 'customer' | 'admin' | 'guest';
  merchant_ids: string[];  // Always array (empty for customers)
  customer_id?: string;    // Only for customer tokens
  session_id?: string;     // Only for guest tokens

  // Permissions
  scopes: string[];   // ['payments:create', 'payments:read']
}
```

### Token Types

| Type | merchant_ids | customer_id | Use Case |
|------|-------------|-------------|----------|
| **merchant** (single) | `['m1']` | `null` | POS terminal |
| **merchant** (multi) | `['m1','m2','m3']` | `null` | Multi-location operator |
| **customer** | `[]` | `'cust_123'` | E-commerce logged-in user |
| **guest** | `['m1']` | `null` | E-commerce guest checkout |
| **admin** | `[]` | `null` | Platform administrators |

---

## Authentication Patterns

### 1. Merchant Token (Single Merchant)

**Use Case:** POS cashier, single-location business

```json
{
  "sub": "pos_terminal_001",
  "token_type": "merchant",
  "merchant_ids": ["merchant_abc123"],
  "customer_id": null,
  "scopes": [
    "payments:create",
    "payments:read",
    "payments:void",
    "payments:refund"
  ],
  "exp": 1736683200
}
```

**Request Behavior:**
- `merchant_id` parameter **OMITTED** from API calls
- Payment service uses `merchant_ids[0]` automatically
- Cannot access other merchants' data

**Example:**

```typescript
// API call: NO merchant_id needed
await client.Authorize({
  customer_id: 'walk_in_123',
  amount_cents: 2500, // $25.00 in cents
  // merchant_id: OMITTED
});

// Backend logic automatically uses token.merchant_ids[0]
```

---

### 2. Merchant Token (Multi-Merchant)

**Use Case:** Remote operator, franchise manager, service provider

```json
{
  "sub": "operator_service_001",
  "token_type": "merchant",
  "merchant_ids": ["merchant_1", "merchant_2", "merchant_3"],
  "customer_id": null,
  "scopes": ["payments:create", "storage:tokenize"],
  "exp": 1736683200
}
```

**Request Behavior:**
- `merchant_id` parameter **REQUIRED** in API calls
- Payment service validates `merchant_id` is in `merchant_ids` array
- Access to multiple merchants, must specify which one

**Example:**

```typescript
// API call: MUST specify merchant_id
await client.Authorize({
  merchant_id: 'merchant_2',  // REQUIRED (validated against array)
  customer_id: 'customer_xyz',
  amount_cents: 5000, // $50.00 in cents
});

// Backend logic validates: 'merchant_2' IN token.merchant_ids
```

---

### 3. Customer Token

**Use Case:** E-commerce logged-in user viewing order history

```json
{
  "sub": "customer_xyz789",
  "token_type": "customer",
  "merchant_ids": [],
  "customer_id": "customer_xyz789",
  "scopes": ["payments:read", "payment_methods:read"],
  "exp": 1736683200
}
```

**Request Behavior:**
- Can only view own transactions
- `customer_id` filter **FORCED** to token's `customer_id`
- Cannot create payments directly (must go through backend)

**Example:**

```typescript
// E-commerce backend queries customer's orders
const orders = await db.query(`
  SELECT payment_parent_transaction_id FROM orders WHERE customer_id = $1
`, [customerId]);

// Calls payment service with customer token
const transactions = await client.GetTransactionsByGroups({
  parent_transaction_ids: orders.map(o => o.payment_parent_transaction_id),
});

// Service automatically filters to customer_id from token
```

---

### 4. Guest Token

**Use Case:** E-commerce guest checkout

```json
{
  "sub": "guest_session_abc",
  "token_type": "guest",
  "merchant_ids": ["merchant_123"],
  "customer_id": null,
  "session_id": "sess_abc123",
  "scopes": ["payments:create"],
  "exp": 1736685000
}
```

**Request Behavior:**
- Can only complete checkout for single session
- Short-lived tokens (30 minutes typical)
- Limited to payment creation only

**Example:**

```typescript
// Guest starts checkout
app.post('/api/guest-checkout', async (req, res) => {
  const token = jwt.sign({
    sub: `guest_${req.session.id}`,
    token_type: 'guest',
    merchant_ids: [req.body.merchant_id],
    session_id: req.session.id,
    scopes: ['payments:create'],
    exp: Math.floor(Date.now() / 1000) + (30 * 60),
  }, JWT_SECRET);

  res.json({ token });
});
```

---

### 5. Admin Token

**Use Case:** Platform support, internal operations

```json
{
  "sub": "admin_support_001",
  "token_type": "admin",
  "merchant_ids": [],
  "customer_id": null,
  "scopes": ["*"],
  "exp": 1736683200
}
```

**Request Behavior:**
- Full access to all merchants and customers
- `merchant_id` parameter **REQUIRED** for write operations
- All actions logged for compliance

**Example:**

```typescript
// Admin refunds payment for any merchant
await client.Refund({
  merchant_id: 'any_merchant_999',  // REQUIRED
  transaction_id: 'tx_xyz789', // Specific transaction to refund
  amount_cents: 10000, // $100.00 in cents
  reason: 'Customer service adjustment',
  idempotency_key: generateKey(),
});
```

---

## Authorization Logic

### Merchant ID Resolution

**Single Merchant Token** (`len(merchant_ids) == 1`):
```go
// Use token's merchant ID (ignore request parameter)
merchantID := token.MerchantIDs[0]
```

**Multi-Merchant Token** (`len(merchant_ids) > 1`):
```go
// Validate requested merchant_id is in allowed list
if requestedMerchantID == "" {
    return Error("merchant_id required: token has multiple merchants")
}
if !contains(token.MerchantIDs, requestedMerchantID) {
    return Error("merchant_id not in allowed list")
}
merchantID := requestedMerchantID
```

**Customer Token** (`token_type == "customer"`):
```go
// Cannot create payments
return Error("customers cannot create payments")
```

**Admin Token** (`token_type == "admin"`):
```go
// Require explicit merchant_id
if requestedMerchantID == "" {
    return Error("merchant_id required for admin")
}
merchantID := requestedMerchantID  // Any merchant allowed
```

### Implementation Example

```go
func (s *PaymentService) resolveMerchantID(
    token *TokenClaims,
    requestedMerchantID string,
) (string, error) {
    switch token.TokenType {
    case "merchant":
        return s.resolveMerchantToken(token, requestedMerchantID)
    case "customer":
        return "", connect.NewError(
            connect.CodePermissionDenied,
            errors.New("customers cannot create payments"),
        )
    case "admin":
        if requestedMerchantID == "" {
            return "", connect.NewError(
                connect.CodeInvalidArgument,
                errors.New("merchant_id required for admin"),
            )
        }
        return requestedMerchantID, nil
    default:
        return "", connect.NewError(
            connect.CodeUnauthenticated,
            errors.New("invalid token type"),
        )
    }
}

func (s *PaymentService) resolveMerchantToken(
    token *TokenClaims,
    requested string,
) (string, error) {
    if len(token.MerchantIDs) == 0 {
        return "", connect.NewError(
            connect.CodeUnauthenticated,
            errors.New("token has no merchant access"),
        )
    }

    // Single merchant
    if len(token.MerchantIDs) == 1 {
        return token.MerchantIDs[0], nil
    }

    // Multiple merchants
    if requested == "" {
        return "", connect.NewError(
            connect.CodeInvalidArgument,
            errors.New("merchant_id required: token has multiple merchants"),
        )
    }

    if !contains(token.MerchantIDs, requested) {
        return "", connect.NewError(
            connect.CodePermissionDenied,
            fmt.Errorf("merchant_id '%s' not in allowed list", requested),
        )
    }

    return requested, nil
}
```

---

## Query Authorization

### List Operations Filtering

**Merchant Token:**
```go
// Single merchant: FORCE filter
if len(token.MerchantIDs) == 1 {
    req.MerchantId = token.MerchantIDs[0]
}

// Multiple merchants
if len(token.MerchantIDs) > 1 {
    if req.MerchantId != "" {
        // Validate specific merchant requested
        if !contains(token.MerchantIDs, req.MerchantId) {
            return ErrUnauthorized
        }
    } else {
        // Query all their merchants
        // SQL: WHERE merchant_id IN (token.MerchantIDs)
        req.MerchantIds = token.MerchantIDs
    }
}
```

**Customer Token:**
```go
// FORCE customer filter
req.CustomerId = token.CustomerID
req.MerchantId = ""  // Ignore merchant filter
```

**Admin Token:**
```go
// No forced filters (can query anything)
```

### Authorization Error Handling

**Always return 404, never 403:**

```go
// ✅ CORRECT
if err := h.authz.CanAccessTransactionGroup(authCtx, txs); err != nil {
    h.logger.Warn("unauthorized access attempt",
        zap.String("actor", authCtx.ActorID),
        zap.String("parent_transaction_id", req.GroupId),
    )
    return nil, status.Error(codes.NotFound, "not found")
}

// ❌ WRONG
return nil, status.Error(codes.PermissionDenied, "access denied")
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
| **Network timeout** | `err != nil` | ❌ No | ✅ Yes |
| **Gateway 500 error** | `err != nil` | ❌ No | ✅ Yes |
| **Payment approved** | `auth_resp="00"` | ✅ Yes | ❌ No |
| **Payment declined** | `auth_resp="05"` | ✅ Yes | ❌ No |

**Why insert declined transactions?**
1. Safety: Prevents double-charging if network flakes during retry
2. Audit trail: PCI compliance requires logging all attempts
3. Fraud detection: Analyze patterns in declined attempts
4. Industry standard: Matches Stripe, Square, PayPal behavior

### Idempotency Key Generation

```typescript
// ✅ CORRECT: Unique key per attempt
function generateIdempotencyKey(operation: string): string {
  return `${operation}_${Date.now()}_${uuidv4()}`;
}

// Usage
const saleKey = generateIdempotencyKey('sale');
// Result: "sale_1736683200000_550e8400-e29b-41d4-a716-446655440000"

// ❌ WRONG: Reusing key for retries after decline
const key = generateIdempotencyKey('sale');
await paymentClient.sale({ idempotency_key: key });  // Declined
await paymentClient.sale({ idempotency_key: key });  // Returns existing decline

// ✅ CORRECT: New key after decline
const key1 = generateIdempotencyKey('sale');
await paymentClient.sale({ idempotency_key: key1 });  // Declined

const key2 = generateIdempotencyKey('sale');  // NEW key
await paymentClient.sale({ idempotency_key: key2 });  // New attempt
```

---

## Role-Based Access Control

### Access Matrix

| Operation | Customer | Guest | Merchant | Admin | Service |
|-----------|----------|-------|----------|-------|---------|
| **Create Payment** | Via Backend | Via Backend | ✅ Yes | ✅ Yes | ✅ Yes |
| **Capture** | ❌ No | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes |
| **Void** | ❌ No | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes |
| **Refund** | ❌ No | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes |
| **Get by Group** | ✅ Own Only | ✅ Session Only | ✅ Own Only | ✅ All | ✅ Scoped |
| **List Transactions** | ✅ Own Only | ❌ No | ✅ Own Only | ✅ All | ✅ Scoped |

### Customer Access Pattern

**Can:**
- View transactions WHERE `customer_id` = token's `customer_id`
- View transaction groups if owns any transaction in group
- List payment methods

**Cannot:**
- Create payments directly (must go through backend)
- Refund or void payments
- View other customers' data
- Access merchant-level operations

**Example Flow:**

```typescript
// E-commerce backend endpoint
app.get('/api/my-transactions', authenticateJWT, async (req, res) => {
  const customerId = req.user.id;

  // Get payment group IDs from orders
  const orders = await db.query(`
    SELECT payment_parent_transaction_id FROM orders WHERE customer_id = $1
  `, [customerId]);

  // Call payment service (backend acts as service account)
  const txs = await paymentClient.GetTransactionsByGroups({
    parent_transaction_ids: orders.map(o => o.payment_parent_transaction_id),
  });

  res.json(txs);
});
```

---

### Guest Access Pattern

**Can:**
- View transaction group WHERE `metadata.session_id` = token's `session_id`
- Complete checkout for single session

**Cannot:**
- List transactions
- Create payments after session expires
- View other sessions' data

**Session Expiry Fallback:**

```typescript
// Email-based order lookup (rate-limited)
app.post('/api/orders/lookup', async (req, res) => {
  const { orderId, email } = req.body;

  // Rate limit heavily (3 requests/hour)
  if (!await rateLimiter.check(req.ip, 'order-lookup', 3, 3600)) {
    return res.status(429).json({ error: 'Too many requests' });
  }

  // Verify order and email
  const order = await db.query(`
    SELECT payment_parent_transaction_id FROM orders
    WHERE id = $1 AND email = $2
    AND created_at > NOW() - INTERVAL '90 days'
  `, [orderId, email.toLowerCase()]);

  if (!order) {
    await sleep(randomInt(100, 500));  // Timing attack prevention
    return res.status(404).json({ error: 'Order not found' });
  }

  const txs = await paymentClient.GetTransactionsByGroupID({
    parent_transaction_id: order.payment_parent_transaction_id,
  });

  res.json(txs);
});
```

---

### Merchant Access Pattern

**Can:**
- Create payments for owned merchants
- Void/refund payments for owned merchants
- View transactions WHERE `merchant_id` IN token's `merchant_ids`
- Capture authorizations for owned merchants

**Cannot:**
- Access other merchants' data
- View full customer payment methods (masked data only)
- Perform admin operations

**Example Flow:**

```go
// POS Backend
func (h *POSHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
    // Validate staff JWT
    staffClaims, _ := validateStaffJWT(r.Header.Get("Authorization"))

    // Check staff has permission for this merchant
    if staffClaims.MerchantID != requestedMerchantID {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Call payment service
    tx, _ := paymentClient.Sale(ctx, &payment.SaleRequest{
        // merchant_id omitted (single merchant token)
        Amount: req.Amount,
        PaymentMethodId: req.PaymentMethodId,
        IdempotencyKey: generateKey(),
    })
}
```

---

### Admin Access Pattern

**Can:**
- View ALL transactions (no filters)
- Refund/void ANY payment
- Create payments for ANY merchant
- View ALL customer data
- Access audit logs
- Manage service accounts

**All actions logged for compliance**

**Example:**

```go
func (i *AuthInterceptor) authenticate(ctx context.Context, apiKey string) (*AuthContext, error) {
    // Admin uses special API key
    if apiKey == os.Getenv("ADMIN_API_KEY") {
        return &AuthContext{
            ActorType: ActorTypeAdmin,
            ActorID: "admin",
            Scopes: []string{"*:*"},  // Full access
        }, nil
    }

    // Regular service account lookup
    svcAccount, _ := i.serviceAccounts.ValidateAPIKey(ctx, apiKey)
    // ...
}
```

---

## Security Best Practices

### 1. Token Security

**Short-Lived Tokens:**
```typescript
// POS terminal: 8 hours
exp: Math.floor(Date.now() / 1000) + (8 * 3600)

// Customer: 24 hours
exp: Math.floor(Date.now() / 1000) + (24 * 3600)

// Guest: 30 minutes
exp: Math.floor(Date.now() / 1000) + (30 * 60)
```

**Token Storage:**
- Store in HTTP-only cookies (not localStorage)
- Use secure flag in production
- Rotate tokens on sensitive operations

### 2. Rate Limiting

```go
var rateLimits = map[string]RateLimit{
    "GetTransactionsByGroupID": {Requests: 10, Window: time.Minute},
    "Sale":                      {Requests: 100, Window: time.Minute},
    "ListTransactions":          {Requests: 10, Window: time.Minute},
}
```

### 3. Audit Logging

```go
// Log all authorization decisions
s.auditLogger.Log(AuditEvent{
    EventType:  "authorization_check",
    ActorType:  authCtx.ActorType,
    ActorID:    authCtx.ActorID,
    Resource:   "transaction",
    ResourceID: tx.ID,
    Action:     "read",
    Allowed:    allowed,
    Reason:     reason,
    IPAddress:  authCtx.IPAddress,
    Timestamp:  time.Now(),
})
```

### 4. Enumeration Prevention

**Always return 404 for unauthorized access:**
```go
// Prevents distinguishing "doesn't exist" from "unauthorized"
if err := h.authz.CanAccess(authCtx, resource); err != nil {
    return nil, status.Error(codes.NotFound, "not found")
}
```

### 5. Input Validation

```go
// Validate token claims
if token.TokenType == "merchant" && len(token.MerchantIDs) == 0 {
    return errors.New("merchant token must have at least one merchant_id")
}

// Validate requested merchant_id format
if req.MerchantId != "" && !isValidUUID(req.MerchantId) {
    return errors.New("invalid merchant_id format")
}
```

---

## Integration Examples

### POS Application

```typescript
// 1. Staff login to POS backend
const staffToken = await posBackend.login(username, password);

// 2. POS backend generates payment service token
const paymentToken = jwt.sign({
  sub: `pos_${terminal.id}`,
  token_type: 'merchant',
  merchant_ids: [staff.merchant_id],
  scopes: ['payments:create', 'payments:refund'],
  exp: Math.floor(Date.now() / 1000) + (8 * 3600),
}, JWT_SECRET);

// 3. POS app calls payment service
const payment = await paymentClient.Sale({
  // merchant_id omitted (single merchant)
  amount_cents: 4599, // $45.99 in cents
  payment_method_id: 'pm_123',
  idempotency_key: generateKey(),
}, {
  metadata: { authorization: `Bearer ${paymentToken}` },
});
```

### E-commerce Application

```typescript
// 1. Customer logs in to e-commerce site
const customerToken = await ecomBackend.login(email, password);

// 2. Customer views order history
const orders = await ecomBackend.getMyOrders(customerToken);

// 3. E-commerce backend calls payment service
const transactions = await paymentClient.GetTransactionsByGroups({
  parent_transaction_ids: orders.map(o => o.payment_parent_transaction_id),
}, {
  metadata: { 'x-api-key': process.env.ECOM_API_KEY },
});

// 4. Return filtered data to customer
return transactions.filter(tx => tx.customer_id === customer.id);
```

---

## Related Documentation

- **DATAFLOW.md** - Understanding authentication flows in payment processes
- **API_SPECS.md** - API endpoints and authentication requirements
- **DATABASE.md** - Multi-tenant data isolation
- **DEVELOP.md** - Testing authentication and authorization