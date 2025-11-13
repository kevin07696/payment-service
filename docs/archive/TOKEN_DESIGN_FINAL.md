# Token Design - Final Specification

**Version**: 3.0 (Simplified with single merchant_ids field)
**Date**: 2025-01-13
**Status**: 🎯 READY TO IMPLEMENT

---

## Core Principle: Single `merchant_ids` Array

Instead of having both `merchant_id` (string) and `merchant_ids` (array), we use **only** `merchant_ids` as a string array.

- **Single merchant**: `merchant_ids: ["merchant_123"]` (array with 1 element)
- **Multiple merchants**: `merchant_ids: ["m1", "m2", "m3"]` (array with N elements)
- **No merchants**: `merchant_ids: []` (empty array for customers/guests)

---

## Token Structure (Final)

```go
type TokenClaims struct {
    // Standard JWT claims
    Sub string `json:"sub"` // Token subject (unique ID)
    Iss string `json:"iss"` // Issuer
    Exp int64  `json:"exp"` // Expiration timestamp
    Iat int64  `json:"iat"` // Issued at

    // Authorization Context
    TokenType   string   `json:"token_type"`   // "merchant" | "customer" | "admin"
    MerchantIDs []string `json:"merchant_ids"` // ✅ ALWAYS array (empty for customers)
    CustomerID  *string  `json:"customer_id"`  // Only for customer tokens
    SessionID   *string  `json:"session_id"`   // Only for guest tokens

    // Permissions
    Scopes []string `json:"scopes"` // ["payments:create", "payments:read"]
}
```

---

## Token Types

### 1. **Merchant Token (Single Merchant) - POS Cashier**

```json
{
    "sub": "pos_terminal_001",
    "token_type": "merchant",
    "merchant_ids": ["merchant_abc123"],  // ✅ Array with 1 element
    "customer_id": null,
    "scopes": ["payments:create", "payments:read", "payments:void", "payments:refund"],
    "exp": 1736683200
}
```

**Usage**:
```typescript
// API call: NO merchant_id needed
await client.authorize({
    customer_id: "walk_in_123",
    amount: "25.00"
    // merchant_id: OMITTED
});

// Backend logic:
// len(token.merchant_ids) == 1  →  Use token.merchant_ids[0]
```

---

### 2. **Merchant Token (Multi-Merchant) - Remote Operator**

```json
{
    "sub": "operator_service_001",
    "token_type": "merchant",
    "merchant_ids": ["merchant_1", "merchant_2", "merchant_3"],  // ✅ Array with N elements
    "customer_id": null,
    "scopes": ["payments:create", "storage:tokenize"],
    "exp": 1736683200
}
```

**Usage**:
```typescript
// API call: MUST specify merchant_id
await client.authorize({
    merchant_id: "merchant_2",  // ✅ REQUIRED (validated against array)
    customer_id: "customer_xyz",
    amount: "50.00"
});

// Backend logic:
// len(token.merchant_ids) > 1  →  req.merchant_id must be in array
```

---

### 3. **Customer Token**

```json
{
    "sub": "customer_xyz789",
    "token_type": "customer",
    "merchant_ids": [],  // ✅ Empty array (customers don't own merchants)
    "customer_id": "customer_xyz789",
    "scopes": ["payments:read", "payment_methods:read"],
    "exp": 1736683200
}
```

**Usage**:
```typescript
// API call: View own transactions
await client.listTransactions({
    limit: 100
    // merchant_id: IGNORED
    // customer_id: FORCED to token.customer_id
});

// Backend logic:
// token.customer_id != null  →  Filter by customer_id
```

---

### 4. **Guest Token**

```json
{
    "sub": "guest_session_abc",
    "token_type": "guest",
    "merchant_ids": ["merchant_123"],  // ✅ Which merchant they're buying from
    "customer_id": null,
    "session_id": "sess_abc123",
    "scopes": ["payments:create"],
    "exp": 1736685000
}
```

**Usage**:
```typescript
// API call: Guest checkout
await client.browserPostCallback({
    amount: "99.99"
    // merchant_id: OMITTED (from token)
    // customer_id: null
});

// Backend logic:
// len(token.merchant_ids) == 1  →  Use token.merchant_ids[0]
```

---

### 5. **Admin Token**

```json
{
    "sub": "admin_support_001",
    "token_type": "admin",
    "merchant_ids": [],  // ✅ Empty (admin can access any merchant)
    "customer_id": null,
    "scopes": ["*"],
    "exp": 1736683200
}
```

**Usage**:
```typescript
// API call: Admin creates payment for any merchant
await client.authorize({
    merchant_id: "any_merchant_999",  // ✅ REQUIRED
    customer_id: "customer_abc",
    amount: "100.00"
});

// Backend logic:
// token.token_type == "admin"  →  Allow any merchant_id
```

---

## Authorization Logic (Simplified)

### Merchant ID Resolution
```go
func (s *PaymentService) resolveMerchantID(token *TokenClaims, requestedMerchantID string) (string, error) {
    switch token.TokenType {
    case "merchant":
        return s.resolveMerchantToken(token, requestedMerchantID)
    case "customer":
        return "", connect.NewError(connect.CodePermissionDenied,
            errors.New("customers cannot create payments"))
    case "admin":
        return s.resolveAdminToken(requestedMerchantID)
    default:
        return "", connect.NewError(connect.CodeUnauthenticated,
            errors.New("invalid token type"))
    }
}

func (s *PaymentService) resolveMerchantToken(token *TokenClaims, requested string) (string, error) {
    // No merchants in token
    if len(token.MerchantIDs) == 0 {
        return "", connect.NewError(connect.CodeUnauthenticated,
            errors.New("token has no merchant access"))
    }

    // Single merchant (POS cashier)
    if len(token.MerchantIDs) == 1 {
        // Use the only merchant in token (ignore request)
        return token.MerchantIDs[0], nil
    }

    // Multiple merchants (operator)
    if requested == "" {
        return "", connect.NewError(connect.CodeInvalidArgument,
            errors.New("merchant_id required: token has multiple merchants"))
    }

    // Validate requested merchant is in token's list
    if !contains(token.MerchantIDs, requested) {
        return "", connect.NewError(connect.CodePermissionDenied,
            fmt.Errorf("merchant_id '%s' not in allowed list", requested))
    }

    return requested, nil
}

func (s *PaymentService) resolveAdminToken(requested string) (string, error) {
    if requested == "" {
        return "", connect.NewError(connect.CodeInvalidArgument,
            errors.New("merchant_id required for admin"))
    }
    return requested, nil
}
```

**Key Logic**:
```go
switch len(token.MerchantIDs) {
case 0:
    // Customer or admin - different logic
case 1:
    // Single merchant - use token.MerchantIDs[0]
default:
    // Multi-merchant - validate requested merchant_id
}
```

---

## List Operations Filter Logic

```go
func (s *PaymentService) ListTransactions(ctx context.Context, req *ListTransactionsRequest) {
    token := getTokenFromContext(ctx)

    switch token.TokenType {
    case "merchant":
        // Apply merchant filter based on token
        if len(token.MerchantIDs) == 0 {
            return ErrNoMerchantAccess
        } else if len(token.MerchantIDs) == 1 {
            // Single merchant: FORCE filter
            req.MerchantId = token.MerchantIDs[0]
        } else {
            // Multiple merchants
            if req.MerchantId != "" {
                // Validate specific merchant requested
                if !contains(token.MerchantIDs, req.MerchantId) {
                    return ErrUnauthorized
                }
                // Use requested merchant_id (validated)
            } else {
                // No specific merchant - query all their merchants
                // SQL: WHERE merchant_id IN (token.MerchantIDs)
                req.MerchantIds = token.MerchantIDs
            }
        }

    case "customer":
        // FORCE customer filter
        req.CustomerId = *token.CustomerID
        req.MerchantId = ""  // Ignore merchant filter

    case "admin":
        // No forced filters
    }

    return s.db.ListTransactions(ctx, req)
}
```

---

## Benefits of Single `merchant_ids` Array

### 1. **Simpler Token Structure**
```go
// ❌ OLD: Two fields
type TokenClaims struct {
    MerchantID  *string  `json:"merchant_id"`   // Single
    MerchantIDs []string `json:"merchant_ids"`  // Multiple
}

// ✅ NEW: One field
type TokenClaims struct {
    MerchantIDs []string `json:"merchant_ids"`  // Always array
}
```

### 2. **Cleaner Logic**
```go
// ❌ OLD: Check both fields
if token.MerchantID != nil {
    // Single merchant logic
} else if len(token.MerchantIDs) > 0 {
    // Multi-merchant logic
}

// ✅ NEW: Check array length
switch len(token.MerchantIDs) {
case 0:  // No merchants
case 1:  // Single merchant
default: // Multiple merchants
}
```

### 3. **Consistent Data Type**
```go
// ✅ Always work with []string
for _, merchantID := range token.MerchantIDs {
    // Process merchant
}

// ✅ SQL IN clause is natural
// WHERE merchant_id IN (token.MerchantIDs)
```

### 4. **No Nil Pointer Checks**
```go
// ❌ OLD: Need nil checks
if token.MerchantID != nil {
    merchantID := *token.MerchantID
}

// ✅ NEW: No nil pointers
if len(token.MerchantIDs) > 0 {
    merchantID := token.MerchantIDs[0]
}
```

---

## Token Issuance Examples

### POS Backend (Single Merchant)
```typescript
// Staff logs in to POS terminal
app.post('/api/pos/login', async (req, res) => {
    const staff = await db.validateStaff(req.body.username, req.body.password);

    const token = jwt.sign({
        sub: `pos_${req.body.terminalId}`,
        token_type: 'merchant',
        merchant_ids: [staff.merchant_id],  // ✅ Array with 1 element
        scopes: ['payments:create', 'payments:read', 'payments:void', 'payments:refund'],
        exp: Math.floor(Date.now() / 1000) + (8 * 3600), // 8 hours
    }, JWT_SECRET);

    res.json({ token });
});
```

### Operator Backend (Multi-Merchant)
```typescript
// Operator service account
const token = jwt.sign({
    sub: 'operator_service_001',
    token_type: 'merchant',
    merchant_ids: ['merchant_1', 'merchant_2', 'merchant_3'],  // ✅ Array with N elements
    scopes: ['payments:create', 'storage:tokenize'],
    exp: Math.floor(Date.now() / 1000) + (24 * 3600),
}, JWT_SECRET);
```

### E-commerce Backend (Customer)
```typescript
// Customer logs in
app.post('/api/login', async (req, res) => {
    const customer = await db.validateCustomer(req.body.email, req.body.password);

    const token = jwt.sign({
        sub: customer.id,
        token_type: 'customer',
        merchant_ids: [],  // ✅ Empty array (customers don't own merchants)
        customer_id: customer.id,
        scopes: ['payments:read', 'payment_methods:read'],
        exp: Math.floor(Date.now() / 1000) + (24 * 3600),
    }, JWT_SECRET);

    res.json({ token });
});
```

### E-commerce Backend (Guest)
```typescript
// Guest starts checkout
app.post('/api/guest-checkout', async (req, res) => {
    const token = jwt.sign({
        sub: `guest_${req.session.id}`,
        token_type: 'guest',
        merchant_ids: [req.body.merchant_id],  // ✅ Array with merchant they're buying from
        session_id: req.session.id,
        scopes: ['payments:create'],
        exp: Math.floor(Date.now() / 1000) + (30 * 60), // 30 min
    }, JWT_SECRET);

    res.json({ token });
});
```

---

## Decision Matrix

| Token Type | `len(merchant_ids)` | `merchant_id` in Request | Resolution |
|------------|-------------------|-------------------------|------------|
| **Merchant (Single)** | 1 | Omit | Use `merchant_ids[0]` |
| **Merchant (Multi)** | > 1 | Required | Validate against array |
| **Customer** | 0 | Ignored | N/A |
| **Guest** | 1 | Omit | Use `merchant_ids[0]` |
| **Admin** | 0 | Required | Any merchant allowed |

---

## Database Schema

### Transactions Table (No change needed)
```sql
CREATE TABLE transactions (
    id UUID PRIMARY KEY,
    merchant_id UUID NOT NULL,  -- ✅ Always single merchant per transaction
    customer_id UUID,            -- ✅ Nullable (guests, walk-ins)
    amount DECIMAL(19, 4) NOT NULL,
    -- ...
);
```

### Service Accounts Table (For token issuance)
```sql
CREATE TABLE service_accounts (
    id UUID PRIMARY KEY,
    service_name VARCHAR(100) NOT NULL,  -- 'pos-backend', 'operator-service'
    api_key_hash VARCHAR(255) NOT NULL,
    merchant_ids UUID[],                  -- ✅ Array of allowed merchants
    scopes TEXT[],
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Example: Single merchant POS
INSERT INTO service_accounts (service_name, api_key_hash, merchant_ids, scopes)
VALUES ('pos-coffee-shop', hash('key123'), ARRAY['merchant_abc123']::UUID[],
        ARRAY['payments:create', 'payments:read']);

-- Example: Multi-merchant operator
INSERT INTO service_accounts (service_name, api_key_hash, merchant_ids, scopes)
VALUES ('operator-restaurants', hash('key456'),
        ARRAY['restaurant_1', 'restaurant_2', 'restaurant_3']::UUID[],
        ARRAY['payments:create', 'storage:tokenize']);
```

---

## Summary

### Key Changes from Previous Design
| Aspect | Old | New |
|--------|-----|-----|
| **Merchant field(s)** | `merchant_id` + `merchant_ids` | `merchant_ids` only |
| **Single merchant** | `merchant_id: "m123"` | `merchant_ids: ["m123"]` |
| **Multi-merchant** | `merchant_ids: ["m1","m2"]` | `merchant_ids: ["m1","m2"]` |
| **Logic** | Check both fields | `len(merchant_ids)` |

### Benefits
1. ✅ **Single field**: No confusion about which field to use
2. ✅ **Consistent type**: Always `[]string`
3. ✅ **Simple logic**: `len()` check handles all cases
4. ✅ **SQL friendly**: Direct use in `IN` clauses
5. ✅ **No nil pointers**: Array is never nil, just empty

---

**Implementation Ready**: This design is final and ready to implement with ConnectRPC.

