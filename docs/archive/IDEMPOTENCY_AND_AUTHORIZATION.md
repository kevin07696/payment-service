# Idempotency and Authorization Strategy

**Version**: 1.0
**Last Updated**: 2025-01-12
**Status**: ✅ ACTIVE

---

## Table of Contents

1. [Overview](#overview)
2. [Idempotency Strategy](#idempotency-strategy)
3. [Authorization Strategy](#authorization-strategy)
4. [API Endpoint Reference](#api-endpoint-reference)
5. [Role-Based Access Control](#role-based-access-control)
6. [Edge Cases and Best Practices](#edge-cases-and-best-practices)

---

## Overview

This document defines how the payment service handles:
- **Idempotency**: Preventing duplicate payments and ensuring safe retries
- **Authorization**: Controlling who can access what data and operations

### Core Principles

1. **Idempotency Key = Payment Attempt**: Each unique attempt gets its own key
2. **Group ID = Payment Lifecycle**: All related transactions share a group_id
3. **Status at Transaction Level**: Each transaction has its own status
4. **Authorization by Context**: Access determined by actor type and ownership

---

## Idempotency Strategy

### What is Idempotency?

Idempotency ensures that **retrying the same request multiple times produces the same result as making it once**, preventing duplicate charges.

### How It Works

```
┌────────────────────────────────────────────────────────────┐
│ Client generates idempotency_key (UUID)                    │
│ ↓                                                           │
│ Payment Service checks if key exists in database           │
│ ↓                                                           │
│ IF EXISTS: Return existing transaction (no gateway call)   │
│ IF NEW: Process payment and save with that key             │
└────────────────────────────────────────────────────────────┘
```

### Database Implementation

```sql
-- Transactions table
CREATE TABLE transactions (
    id UUID PRIMARY KEY,  -- Idempotency key becomes PK
    group_id UUID NOT NULL,
    status VARCHAR(20) GENERATED ALWAYS AS (
        CASE
            WHEN auth_resp = '00' THEN 'approved'
            ELSE 'declined'
        END
    ) STORED,
    -- ...
);

-- Insert with conflict handling
INSERT INTO transactions (id, group_id, ...)
VALUES ($1, $2, ...)
ON CONFLICT (id) DO NOTHING  -- Returns no rows if key exists
RETURNING *;
```

### When to Insert Transactions

| Scenario | Gateway Response | Insert Transaction? | Retryable with Same Key? |
|----------|------------------|---------------------|--------------------------|
| **Network timeout** | `err != nil` | ❌ No | ✅ Yes |
| **Gateway 500 error** | `err != nil` | ❌ No | ✅ Yes |
| **Connection refused** | `err != nil` | ❌ No | ✅ Yes |
| **Payment approved** | `auth_resp="00"` | ✅ **Yes** | ❌ No - returns existing |
| **Payment declined** | `auth_resp="05"` | ✅ **Yes** | ❌ No - returns existing |
| **Invalid card** | `auth_resp="54"` | ✅ **Yes** | ❌ No - returns existing |
| **Fraud decline** | `auth_resp="59"` | ✅ **Yes** | ❌ No - returns existing |

### Why Insert Declined Transactions?

```
Scenario: Customer tries to pay with declined card

Attempt 1:
  idempotency_key: "payment_abc123"
  Result: Declined (insufficient funds)
  Database: ✅ Saved with status=declined

Attempt 2 (network retry):
  idempotency_key: "payment_abc123" (same key)
  Result: Returns existing declined transaction
  Database: No gateway call, prevents double-attempt

Attempt 3 (user fixes card):
  idempotency_key: "payment_def456" (NEW key)
  Result: Approved
  Database: New transaction created
```

**Benefits:**
1. **Safety**: Prevents double-charging if network flakes during retry
2. **Audit Trail**: PCI compliance requires logging all attempts
3. **Fraud Detection**: Analyze patterns in declined attempts
4. **Industry Standard**: Matches Stripe, Square, PayPal behavior

---

## Authorization Strategy

### Actor Types

| Actor Type | Description | Authentication Method |
|------------|-------------|----------------------|
| **Customer** | Signed-in user with account | JWT (via e-commerce backend) |
| **Guest** | Anonymous checkout user | Session ID + Order ID |
| **Merchant** | Business/store owner | JWT + API Key (via POS backend) |
| **Admin** | Platform administrator | JWT + Admin token |
| **Service** | Backend service (POS, e-commerce) | API Key only |

### Authentication Context

Every request includes an `AuthContext`:

```go
type AuthContext struct {
    // Who is making the request
    ActorType    ActorType  // "customer", "guest", "merchant", "admin", "service"
    ActorID      string     // customer_id, merchant_id, admin_id, service_account_id

    // What they have access to
    MerchantID   *string    // NULL for customers/guests, specific for merchants
    CustomerID   *string    // NULL for merchants, specific for customers

    // Session info (for guests)
    SessionID    *string    // Guest checkout session

    // Permissions
    Scopes       []string   // ["read:transactions", "write:payments", "refund:all"]

    // Audit trail
    IPAddress    string
    UserAgent    string
    RequestID    string
}
```

### Authorization Flow

```
1. Request arrives with authentication headers
   ↓
2. Interceptor validates credentials (API key or JWT)
   ↓
3. Build AuthContext from claims/service account
   ↓
4. Store context in request context
   ↓
5. Handler retrieves context and checks permissions
   ↓
6. Apply authorization filters to queries
   ↓
7. Return data or 404 (never 403 to prevent enumeration)
```

---

## API Endpoint Reference

### Payment Operations (Create Transactions)

#### `Authorize` - Hold funds without capturing

**Endpoint**: `POST /api/v1/payments/authorize`

**Idempotency**:
- ✅ **Required**: `idempotency_key` in request
- **Behavior**: Creates transaction with type=`auth`, stores BRIC token in transaction_groups
- **Retry**: New key required if declined
- **Returns**: `PaymentResponse` with `transaction_id` and `group_id`

**Authorization**:

| Actor Type | Allowed? | Conditions |
|------------|----------|------------|
| Customer | ❌ No | Cannot authorize (only purchase) |
| Guest | ❌ No | Cannot authorize (only purchase) |
| Merchant | ✅ Yes | Must own merchant_id in request |
| Admin | ✅ Yes | Can authorize for any merchant |
| Service | ✅ Yes | Must be authorized for merchant_id |

**Example**:
```json
// Request
{
  "merchant_id": "merch_123",
  "customer_id": "cust_789",
  "amount": "100.00",
  "currency": "USD",
  "payment_method_id": "pm_abc",
  "idempotency_key": "auth_20250112_001"
}

// Response (Success)
{
  "transaction_id": "tx_def456",
  "group_id": "grp_xyz789",
  "amount": "100.00",
  "status": "APPROVED",
  "is_approved": true
}

// Response (Declined)
{
  "transaction_id": "tx_def456",
  "group_id": "grp_xyz789",
  "amount": "100.00",
  "status": "DECLINED",
  "is_approved": false,
  "message": "Insufficient funds"
}
```

---

#### `Sale` - Combined authorize + capture

**Endpoint**: `POST /api/v1/payments/sale`

**Idempotency**:
- ✅ **Required**: `idempotency_key` in request
- **Behavior**: Creates transaction with type=`sale`, stores BRIC token
- **Retry**: New key required if declined
- **Returns**: `PaymentResponse` with `transaction_id` and `group_id`

**Authorization**:

| Actor Type | Allowed? | Conditions |
|------------|----------|------------|
| Customer | ✅ Yes | Via e-commerce backend (service account) |
| Guest | ✅ Yes | Via e-commerce backend (service account) |
| Merchant | ✅ Yes | Must own merchant_id in request |
| Admin | ✅ Yes | Can process for any merchant |
| Service | ✅ Yes | Must be authorized for merchant_id |

**Note**: Customers and guests never call this directly - always through e-commerce backend acting as service account.

---

#### `Capture` - Complete previously authorized payment

**Endpoint**: `POST /api/v1/payments/capture`

**Idempotency**:
- ✅ **Required**: `idempotency_key` in request
- **Key Requirement**: Uses `group_id` (not transaction_id) to identify AUTH
- **Behavior**:
  - Retrieves BRIC token from transaction_groups
  - Creates new transaction with type=`capture`
  - Both AUTH and CAPTURE share same group_id
- **Retry**: New key required if declined

**Authorization**:

| Actor Type | Allowed? | Conditions |
|------------|----------|------------|
| Customer | ❌ No | Cannot capture |
| Guest | ❌ No | Cannot capture |
| Merchant | ✅ Yes | Must own the group_id |
| Admin | ✅ Yes | Can capture any group |
| Service | ✅ Yes | Must be authorized for merchant owning group |

**Request**:
```json
{
  "group_id": "grp_xyz789",  // NOT transaction_id!
  "amount": "75.00",         // Optional: partial capture
  "idempotency_key": "capture_20250112_001"
}
```

**Flow**:
```
1. Lookup transaction_groups by group_id
2. Retrieve auth_guid (BRIC token)
3. Call gateway with BRIC to capture
4. Create new transaction record with type=capture
```

---

#### `Void` - Cancel authorized or captured payment

**Endpoint**: `POST /api/v1/payments/void`

**Idempotency**:
- ✅ **Required**: `idempotency_key` in request
- **Key Requirement**: Uses `group_id` to identify payment
- **Behavior**:
  - Retrieves BRIC from transaction_groups
  - Creates new transaction with type=`void`
  - Original and void share same group_id

**Authorization**:

| Actor Type | Allowed? | Conditions |
|------------|----------|------------|
| Customer | ❌ No | Cannot void |
| Guest | ❌ No | Cannot void |
| Merchant | ✅ Yes | Must own the group_id |
| Admin | ✅ Yes | Can void any group |
| Service | ✅ Yes | Must be authorized for merchant owning group |

---

#### `Refund` - Return funds to customer

**Endpoint**: `POST /api/v1/payments/refund`

**Idempotency**:
- ✅ **Required**: `idempotency_key` in request
- **Key Requirement**: Uses `group_id` to identify payment
- **Behavior**:
  - Retrieves BRIC from transaction_groups
  - Creates new transaction with type=`refund`
  - Supports partial refunds

**Authorization**:

| Actor Type | Allowed? | Conditions |
|------------|----------|------------|
| Customer | ❌ No | Cannot refund |
| Guest | ❌ No | Cannot refund |
| Merchant | ✅ Yes | Must own the group_id |
| Admin | ✅ Yes | Can refund any group |
| Service | ✅ Yes | Must be authorized for merchant owning group |

**Request**:
```json
{
  "group_id": "grp_xyz789",
  "amount": "50.00",  // Optional: partial refund
  "reason": "Customer requested refund",
  "idempotency_key": "refund_20250112_001"
}
```

---

### Query Operations (Read Transactions)

#### `GetTransactionsByGroupID` - Get full payment history

**Endpoint**: `GET /api/v1/payments/groups/{group_id}`

**Idempotency**:
- ❌ **Not applicable**: Read-only operation, no idempotency needed

**Authorization**:

| Actor Type | Allowed? | Conditions | Returns |
|------------|----------|------------|---------|
| Customer | ✅ Yes | Must own customer_id in transactions | All txs in group if owned |
| Guest | ✅ Yes | session_id must match original transaction metadata | All txs if session matches |
| Merchant | ✅ Yes | Must own merchant_id | All txs in group |
| Admin | ✅ Yes | No restrictions | All txs in group |
| Service | ✅ Yes | Must be authorized for merchant | All txs in group |

**Use Case**: Customer checks if refund was processed

```typescript
// Frontend calls backend
const response = await fetch('/api/orders/12345/payment-status', {
  headers: { 'Authorization': 'Bearer customer_jwt' }
});

// Backend calls payment service
const txs = await paymentClient.GetTransactionsByGroupID({
  group_id: "grp_xyz789"
});

// Response
{
  "transactions": [
    { "id": "tx_1", "type": "SALE", "amount": "100", "status": "APPROVED" },
    { "id": "tx_2", "type": "REFUND", "amount": "100", "status": "APPROVED" }
  ],
  "summary": {
    "original_amount": "100.00",
    "refunded_amount": "100.00",
    "net_amount": "0.00",
    "fully_refunded": true
  }
}
```

---

#### `GetTransactionsByGroups` - Batch fetch for customer dashboard

**Endpoint**: `POST /api/v1/payments/batch-get`

**Idempotency**:
- ❌ **Not applicable**: Read-only operation

**Authorization**:

| Actor Type | Allowed? | Conditions |
|------------|----------|------------|
| Customer | ✅ Yes | E-commerce backend provides group_ids for that customer |
| Guest | ❌ No | Cannot list multiple groups |
| Merchant | ✅ Yes | Can query groups they own |
| Admin | ✅ Yes | No restrictions |
| Service | ✅ Yes | Provides group_ids from their orders table |

**Use Case**: Customer dashboard showing all payments

```typescript
// E-commerce backend
const orders = await db.query(`
  SELECT op.payment_group_id
  FROM order_payments op
  JOIN orders o ON o.id = op.order_id
  WHERE o.customer_id = $1
`, [customerId]);

const groupIds = orders.map(o => o.payment_group_id);

// Call payment service
const txs = await paymentClient.GetTransactionsByGroups({
  group_ids: groupIds
});
```

---

#### `ListTransactions` - Paginated list with filters

**Endpoint**: `GET /api/v1/payments?merchant_id=X&customer_id=Y&limit=100`

**Idempotency**:
- ❌ **Not applicable**: Read-only operation

**Authorization**:

| Actor Type | Allowed? | Filters Applied |
|------------|----------|-----------------|
| Customer | ✅ Yes | **FORCED**: customer_id = actor's customer_id |
| Guest | ❌ No | Cannot list (only GetTransactionsByGroupID) |
| Merchant | ✅ Yes | **FORCED**: merchant_id = actor's merchant_id |
| Admin | ✅ Yes | No forced filters (can query all) |
| Service | ✅ Yes | Filtered to allowed merchants |

**Example Authorization Logic**:
```go
func (s *AuthorizationService) BuildTransactionFilters(
    authCtx *AuthContext,
    baseFilters *ListTransactionsFilters,
) (*ListTransactionsFilters, error) {
    switch authCtx.ActorType {
    case ActorTypeCustomer:
        // Override any merchant_id customer tried to pass
        baseFilters.MerchantID = nil
        // Force filter to customer's transactions only
        baseFilters.CustomerID = authCtx.CustomerID
        return baseFilters, nil

    case ActorTypeMerchant:
        // Force filter to merchant's transactions only
        baseFilters.MerchantID = authCtx.MerchantID
        return baseFilters, nil

    case ActorTypeAdmin:
        // No forced filters - admin sees all
        return baseFilters, nil
    }
}
```

---

## Role-Based Access Control

### Customer Access Pattern

**Authentication**: JWT issued by e-commerce backend after login

**Typical Flow**:
```
Browser → E-commerce Backend → Payment Service
  (JWT)       (validates JWT)      (API key)
              (builds context)
```

**Access Rules**:
```go
Customer "cust_123" can:
✅ View transactions WHERE customer_id = "cust_123"
✅ View groups WHERE original transaction has customer_id = "cust_123"
❌ Create payments (goes through backend)
❌ Refund/void
❌ View other customers' data
❌ List all transactions
```

**Implementation**:
```typescript
// E-commerce backend endpoint
app.get('/api/my-transactions', authenticateJWT, async (req, res) => {
  const customerId = req.user.id; // From JWT

  // Get group_ids from orders table
  const orders = await db.query(`
    SELECT op.payment_group_id
    FROM order_payments op
    JOIN orders o ON o.id = op.order_id
    WHERE o.customer_id = $1
  `, [customerId]);

  // Call payment service with service API key
  const txs = await paymentClient.GetTransactionsByGroups({
    group_ids: orders.map(o => o.payment_group_id)
  }, {
    headers: {
      'X-API-Key': process.env.ECOM_API_KEY,
      'X-Actor-Type': 'customer',
      'X-Actor-ID': customerId
    }
  });

  res.json(txs);
});
```

---

### Guest Access Pattern

**Authentication**: Session ID + Order ID/Tracking Token

**Typical Flow**:
```
Browser → E-commerce Backend → Payment Service
 (session)    (validates session)    (API key + session context)
              (includes session_id)
```

**Access Rules**:
```go
Guest with session_id = "sess_abc123" can:
✅ View group WHERE metadata.session_id = "sess_abc123"
❌ List transactions
❌ Create payments directly (goes through backend)
❌ Refund/void
❌ View other sessions' data
```

**Implementation**:
```typescript
// Guest payment status check
app.get('/api/orders/:orderId/status', async (req, res) => {
  const { orderId } = req.params;
  const sessionId = req.session.id;

  // Verify order belongs to session
  const order = await db.query(`
    SELECT payment_group_id
    FROM orders
    WHERE id = $1 AND session_id = $2
  `, [orderId, sessionId]);

  if (!order) {
    return res.status(404).json({ error: 'Order not found' });
  }

  // Call payment service
  const txs = await paymentClient.GetTransactionsByGroupID({
    group_id: order.payment_group_id
  }, {
    headers: {
      'X-API-Key': process.env.ECOM_API_KEY,
      'X-Actor-Type': 'guest',
      'X-Session-ID': sessionId
    }
  });

  res.json(txs);
});
```

**Session Expiry Handling**:
```typescript
// Option: Email-based fallback
app.post('/api/orders/lookup', async (req, res) => {
  const { orderId, email } = req.body;

  // Verify email matches order
  const order = await db.query(`
    SELECT payment_group_id, email_hash
    FROM orders
    WHERE id = $1 AND email_hash = $2
  `, [orderId, hashEmail(email)]);

  // Allows guest to check status even after session expires
});
```

---

### Merchant Access Pattern

**Authentication**: API Key (service account) + JWT (staff identity)

**Typical Flow**:
```
POS Terminal → POS Backend → Payment Service
   (JWT)       (validates JWT)   (API key)
               (staff auth)
```

**Access Rules**:
```go
Merchant staff at "store_123" can:
✅ Create payments for merchant_id = "store_123"
✅ Void/refund payments for merchant_id = "store_123"
✅ View transactions WHERE merchant_id = "store_123"
✅ Capture authorizations for merchant_id = "store_123"
❌ Access other merchants' data
❌ View customer payment methods (get masked data only)
```

**Implementation**:
```go
// POS Backend
func (h *POSHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
    // Validate staff JWT
    staffClaims, err := validateStaffJWT(r.Header.Get("Authorization"))

    // Check staff has permission for this merchant
    if staffClaims.MerchantID != requestedMerchantID {
        http.Error(w, "Unauthorized", http.StatusForbidden)
        return
    }

    // Call payment service
    tx, err := paymentClient.Sale(ctx, &payment.SaleRequest{
        MerchantID: staffClaims.MerchantID,
        Amount: req.Amount,
        // ...
    }, grpc.PerRPCCredentials(&apiKeyAuth{
        apiKey: os.Getenv("POS_API_KEY"),
        actorType: "merchant",
        actorID: staffClaims.MerchantID,
    }))
}
```

---

### Admin Access Pattern

**Authentication**: Admin JWT + Admin API Key

**Access Rules**:
```go
Admin can:
✅ View ALL transactions (no filters)
✅ Refund/void ANY payment
✅ Create payments for ANY merchant
✅ View ALL customer data
✅ Access audit logs
✅ Manage service accounts
⚠️ All actions logged for compliance
```

**Implementation**:
```go
func (i *AuthInterceptor) authenticate(ctx context.Context, apiKey string) (*AuthContext, error) {
    // Admin uses special API key
    if apiKey == os.Getenv("ADMIN_API_KEY") {
        return &AuthContext{
            ActorType: ActorTypeAdmin,
            ActorID: "admin",
            Scopes: []string{"*:*"}, // Full access
        }, nil
    }

    // Regular service account lookup
    svcAccount, err := i.serviceAccounts.ValidateAPIKey(ctx, apiKey)
    // ...
}
```

---

### Service Account Access Pattern

**Authentication**: API Key only (long-lived)

**Types of Service Accounts**:
```
1. E-commerce Backend:
   - Scopes: ["create:payments", "read:transactions"]
   - Merchants: [merch_ecom_1, merch_ecom_2]

2. POS Backend:
   - Scopes: ["create:payments", "refund:payments", "read:transactions"]
   - Merchants: [merch_pos_1, merch_pos_2, ...]

3. Subscription Service:
   - Scopes: ["create:payments", "read:subscriptions"]
   - Merchants: [all]

4. Reporting Service:
   - Scopes: ["read:transactions", "read:analytics"]
   - Merchants: [all]
```

**Implementation**:
```sql
-- Service accounts table
CREATE TABLE service_accounts (
    id UUID PRIMARY KEY,
    service_name VARCHAR(100) NOT NULL,  -- 'pos-backend', 'ecommerce-backend'
    api_key_hash VARCHAR(255) NOT NULL,
    allowed_merchants UUID[],  -- NULL = all merchants
    scopes TEXT[],
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Example
INSERT INTO service_accounts (id, service_name, api_key_hash, allowed_merchants, scopes)
VALUES (
    gen_random_uuid(),
    'ecommerce-backend',
    hash('ecom_live_abc123xyz'),
    ARRAY['merch_ecom_1', 'merch_ecom_2']::UUID[],
    ARRAY['create:payments', 'read:transactions']
);
```

---

## Edge Cases and Best Practices

### Edge Case 1: Idempotency Key Reuse After Decline

**Scenario**:
```
Attempt 1: Card declined (insufficient funds)
Client retries with SAME idempotency key
```

**Behavior**:
```go
// First attempt
req := &SaleRequest{
    IdempotencyKey: "payment_abc123",
    Amount: "50.00",
}
// Result: Declined, saved to database

// Retry (client bug or network layer retry)
req := &SaleRequest{
    IdempotencyKey: "payment_abc123", // Same key!
    Amount: "50.00",
}

// Database check:
existingTx, _ := db.GetTransactionByID("payment_abc123")
if existingTx != nil {
    // Return existing declined transaction
    return existingTx, nil
}
```

**Response to Client**:
```json
{
  "transaction_id": "payment_abc123",
  "status": "DECLINED",
  "message": "Insufficient funds",
  "is_approved": false
}
```

**Client Action**: Generate NEW idempotency key and retry

---

### Edge Case 2: Network Failure During Gateway Call

**Scenario**:
```
Payment service calls gateway
Network timeout occurs
No response from gateway
```

**Behavior**:
```go
epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
if err != nil {
    // ❌ NO database insert
    return nil, fmt.Errorf("gateway error: %w", err)
}

// No transaction created
// Client can retry with SAME idempotency key
```

**Client Sees**: `500 Internal Server Error: gateway timeout`

**Client Action**: Retry with SAME idempotency key (safe because nothing was saved)

---

### Edge Case 3: Guest Session Expires

**Scenario**:
```
Day 1: Guest makes payment (session_id = "sess_abc")
Day 7: Session expires, new session = "sess_xyz"
Day 7: Guest tries to check refund status
```

**Solution**: Email-based fallback

```typescript
// Guest lookup endpoint
app.post('/api/orders/lookup', async (req, res) => {
  const { orderId, email } = req.body;

  // Rate limit this endpoint heavily (prevent enumeration)
  if (!await rateLimiter.check(req.ip, 'order-lookup', 3, 3600)) {
    return res.status(429).json({ error: 'Too many requests' });
  }

  // Verify order exists and email matches
  const order = await db.query(`
    SELECT o.id, o.payment_group_id, o.email
    FROM orders o
    WHERE o.id = $1
    AND o.email = $2
    AND o.created_at > NOW() - INTERVAL '90 days'
  `, [orderId, email.toLowerCase()]);

  if (!order) {
    // Return same error for not found vs wrong email (prevent enumeration)
    await sleep(randomInt(100, 500)); // Timing attack prevention
    return res.status(404).json({ error: 'Order not found' });
  }

  // Fetch payment status
  const txs = await paymentClient.GetTransactionsByGroupID({
    group_id: order.payment_group_id
  });

  res.json(txs);
});
```

---

### Edge Case 4: Partial Capture/Refund Tracking

**Scenario**:
```
Original AUTH: $100
Capture 1: $60
Capture 2: $40
Refund 1: $30
```

**Solution**: Transaction group aggregation

```go
type TransactionSummary struct {
    OriginalAmount  decimal.Decimal
    CapturedAmount  decimal.Decimal
    RefundedAmount  decimal.Decimal
    VoidedAmount    decimal.Decimal
    NetAmount       decimal.Decimal
    FullyRefunded   bool
    Voided          bool
}

func calculateGroupSummary(transactions []*Transaction) *TransactionSummary {
    summary := &TransactionSummary{}

    for _, tx := range transactions {
        if tx.Status != TransactionStatusApproved {
            continue // Skip declined
        }

        switch tx.Type {
        case TransactionTypeAuth, TransactionTypeSale:
            summary.OriginalAmount = summary.OriginalAmount.Add(tx.Amount)
        case TransactionTypeCapture:
            summary.CapturedAmount = summary.CapturedAmount.Add(tx.Amount)
        case TransactionTypeRefund:
            summary.RefundedAmount = summary.RefundedAmount.Add(tx.Amount)
        case TransactionTypeVoid:
            summary.Voided = true
        }
    }

    summary.NetAmount = summary.OriginalAmount.
        Sub(summary.RefundedAmount)

    summary.FullyRefunded = summary.NetAmount.IsZero() || summary.Voided

    return summary
}
```

---

### Best Practice 1: Client Idempotency Key Generation

```typescript
// ✅ CORRECT: Unique key per attempt
function generateIdempotencyKey(operation: string): string {
  return `${operation}_${Date.now()}_${uuidv4()}`;
}

// Usage
const saleKey = generateIdempotencyKey('sale');
// Result: "sale_1736683200000_550e8400-e29b-41d4-a716-446655440000"

// ❌ WRONG: Reusing key for retries
const key = generateIdempotencyKey('sale');
for (let i = 0; i < 3; i++) {
  await paymentClient.sale({ idempotency_key: key }); // Bad!
}

// ✅ CORRECT: New key per retry
for (let i = 0; i < 3; i++) {
  const key = generateIdempotencyKey('sale'); // New key each time
  try {
    return await paymentClient.sale({ idempotency_key: key });
  } catch (error) {
    if (isNetworkError(error)) continue;
    throw error;
  }
}
```

---

### Best Practice 2: Authorization Error Handling

```go
// ✅ CORRECT: Return 404, not 403
func (h *Handler) GetTransactionsByGroupID(ctx context.Context, req *GetTransactionsByGroupIDRequest) (*GetTransactionsByGroupIDResponse, error) {
    authCtx, _ := middleware.GetAuthContext(ctx)

    txs, err := h.service.GetTransactionsByGroupID(ctx, req.GroupId)
    if err != nil {
        return nil, status.Error(codes.NotFound, "not found")
    }

    // Authorization check
    if err := h.authz.CanAccessTransactionGroup(authCtx, txs); err != nil {
        h.logger.Warn("unauthorized access attempt",
            zap.String("actor", authCtx.ActorID),
            zap.String("group_id", req.GroupId),
        )
        // ✅ Return NotFound (not PermissionDenied)
        return nil, status.Error(codes.NotFound, "not found")
    }

    return toProto(txs), nil
}

// ❌ WRONG: Return 403
return nil, status.Error(codes.PermissionDenied, "access denied")
// This reveals the resource exists!
```

**Why return 404 instead of 403?**
- Prevents enumeration attacks
- Attacker cannot distinguish "doesn't exist" from "exists but unauthorized"
- Industry standard (AWS, Stripe, etc.)

---

### Best Practice 3: Rate Limiting by Operation

```go
// Different rate limits per operation type
var rateLimits = map[string]RateLimit{
    "GetTransactionsByGroupID": {
        Requests: 10,
        Window:   time.Minute,
    },
    "Sale": {
        Requests: 100,
        Window:   time.Minute,
    },
    "ListTransactions": {
        Requests: 10,
        Window:   time.Minute,
    },
}

func (i *AuthInterceptor) checkRateLimit(ctx context.Context, method string, actorID string) error {
    limit := rateLimits[method]
    key := fmt.Sprintf("ratelimit:%s:%s", method, actorID)

    count, err := redis.Incr(ctx, key)
    if err != nil {
        return err
    }

    if count == 1 {
        redis.Expire(ctx, key, limit.Window)
    }

    if count > limit.Requests {
        return status.Error(codes.ResourceExhausted, "rate limit exceeded")
    }

    return nil
}
```

---

### Best Practice 4: Audit Logging

```go
// Log all authorization decisions
func (s *AuthorizationService) CanAccessTransaction(authCtx *AuthContext, tx *Transaction) error {
    allowed := false
    reason := ""

    switch authCtx.ActorType {
    case ActorTypeCustomer:
        allowed = authCtx.CustomerID != nil &&
                  tx.CustomerID != nil &&
                  *authCtx.CustomerID == *tx.CustomerID
        reason = "customer_ownership"

    case ActorTypeMerchant:
        allowed = authCtx.MerchantID != nil &&
                  tx.MerchantID == *authCtx.MerchantID
        reason = "merchant_ownership"

    case ActorTypeAdmin:
        allowed = true
        reason = "admin_access"
    }

    // Log decision
    s.auditLogger.Log(AuditEvent{
        EventType:     "authorization_check",
        ActorType:     authCtx.ActorType,
        ActorID:       authCtx.ActorID,
        Resource:      "transaction",
        ResourceID:    tx.ID,
        Action:        "read",
        Allowed:       allowed,
        Reason:        reason,
        IPAddress:     authCtx.IPAddress,
        Timestamp:     time.Now(),
    })

    if !allowed {
        return ErrUnauthorized
    }

    return nil
}
```

---

## Summary Tables

### Idempotency by Endpoint

| Endpoint | Idempotency Key Required? | Insert on Decline? | Retry Behavior |
|----------|---------------------------|-------------------|----------------|
| `Authorize` | ✅ Yes | ✅ Yes | New key required |
| `Sale` | ✅ Yes | ✅ Yes | New key required |
| `Capture` | ✅ Yes | ✅ Yes | New key required |
| `Void` | ✅ Yes | ✅ Yes | New key required |
| `Refund` | ✅ Yes | ✅ Yes | New key required |
| `GetTransactionsByGroupID` | ❌ No | N/A | Freely retryable |
| `GetTransactionsByGroups` | ❌ No | N/A | Freely retryable |
| `ListTransactions` | ❌ No | N/A | Freely retryable |

### Authorization Matrix

| Operation | Customer | Guest | Merchant | Admin | Service |
|-----------|----------|-------|----------|-------|---------|
| **Create Payment** | Via Backend | Via Backend | ✅ Yes | ✅ Yes | ✅ Yes |
| **Capture** | ❌ No | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes |
| **Void** | ❌ No | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes |
| **Refund** | ❌ No | ❌ No | ✅ Yes | ✅ Yes | ✅ Yes |
| **Get by Group** | ✅ Own Only | ✅ Session Only | ✅ Own Only | ✅ All | ✅ Scoped |
| **List Transactions** | ✅ Own Only | ❌ No | ✅ Own Only | ✅ All | ✅ Scoped |

---

**Document Version**: 1.0
**Last Updated**: 2025-01-12
**Status**: ✅ ACTIVE
