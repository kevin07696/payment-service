# Token-Based Authorization Strategy (ConnectRPC)

**Version**: 2.0 (Simplified)
**Date**: 2025-01-13
**Status**: 🎯 READY TO IMPLEMENT

---

## Core Principle: Token Carries Context, Not Complex Roles

Instead of roles (Customer, Merchant, Admin), the **token itself contains the authorization context**.

---

## Token Structure

### JWT Claims
```go
type TokenClaims struct {
    // Standard JWT claims
    Sub string `json:"sub"` // Token subject (unique ID)
    Iss string `json:"iss"` // Issuer (e.g., "payment-service")
    Exp int64  `json:"exp"` // Expiration timestamp
    Iat int64  `json:"iat"` // Issued at timestamp

    // Authorization Context
    TokenType    string   `json:"token_type"`    // "merchant" | "customer" | "guest" | "admin"
    MerchantID   *string  `json:"merchant_id"`   // Single merchant (POS cashier)
    CustomerID   *string  `json:"customer_id"`   // Single customer
    SessionID    *string  `json:"session_id"`    // Guest session

    // Multi-tenant tokens (for operators)
    MerchantIDs  []string `json:"merchant_ids"`  // Multiple merchants (remote operator)

    // Permissions (simple scopes)
    Scopes       []string `json:"scopes"`        // ["payments:create", "payments:read"]
}
```

---

## Token Types & Authorization Rules

### 1. **Merchant Token** (Single Merchant - POS Cashier)

**Use Case**: Local cashier at coffee shop processing in-person payments

**Token**:
```json
{
    "sub": "pos_terminal_001",
    "token_type": "merchant",
    "merchant_id": "merchant_abc123",
    "customer_id": null,
    "merchant_ids": null,
    "scopes": ["payments:create", "payments:read", "payments:void", "payments:refund"],
    "exp": 1736683200
}
```

**Authorization Rules**:
```go
// CREATE OPERATIONS (Authorize, Sale, Capture, Void, Refund)
✅ Can create payments for merchant_id = "merchant_abc123"
❌ REJECT if request.merchant_id != token.merchant_id

// READ OPERATIONS
✅ ListTransactions: FORCE filter WHERE merchant_id = "merchant_abc123"
   - Even if request asks for merchant_id = "other_merchant", override it
✅ GetTransaction: Check transaction.merchant_id = "merchant_abc123"

// WHAT THEY SEE
✅ All transactions for their merchant
✅ All customers who paid at their merchant
❌ Cannot see other merchants' data
```

**Implementation**:
```go
func (s *PaymentService) Authorize(ctx context.Context, req *AuthorizeRequest) {
    token := getTokenFromContext(ctx)

    // ENFORCE merchant isolation
    if token.MerchantID != nil && req.MerchantId != *token.MerchantID {
        return connect.NewError(connect.CodePermissionDenied,
            errors.New("cannot create payment for different merchant"))
    }
}

func (s *PaymentService) ListTransactions(ctx context.Context, req *ListTransactionsRequest) {
    token := getTokenFromContext(ctx)

    // OVERRIDE any merchant_id they tried to pass
    if token.MerchantID != nil {
        req.MerchantId = *token.MerchantID  // FORCE to token's merchant
    }

    // Query will only return merchant_abc123's transactions
    return s.db.ListTransactions(ctx, req)
}
```

---

### 2. **Multi-Merchant Token** (Remote Operator)

**Use Case**: Central payment operator managing storage API for multiple restaurant chains

**Token**:
```json
{
    "sub": "operator_service_001",
    "token_type": "merchant",
    "merchant_id": null,
    "merchant_ids": ["merchant_1", "merchant_2", "merchant_3"],
    "scopes": ["storage:tokenize", "storage:detokenize", "payments:read"],
    "exp": 1736683200
}
```

**Authorization Rules**:
```go
// CREATE OPERATIONS
✅ Can create payments for ANY merchant in merchant_ids list
❌ REJECT if request.merchant_id NOT IN token.merchant_ids

// READ OPERATIONS
✅ ListTransactions: Filter WHERE merchant_id IN ("merchant_1", "merchant_2", "merchant_3")
✅ GetTransaction: Check transaction.merchant_id IN allowed list

// WHAT THEY SEE
✅ Transactions for merchants 1, 2, 3
✅ All customers across those 3 merchants
❌ Cannot see merchant_4's data
```

**Implementation**:
```go
func (s *PaymentService) ListTransactions(ctx context.Context, req *ListTransactionsRequest) {
    token := getTokenFromContext(ctx)

    // If token has merchant_ids list, filter to those
    if len(token.MerchantIDs) > 0 {
        // User requested specific merchant?
        if req.MerchantId != "" {
            // Check if it's in their allowed list
            if !contains(token.MerchantIDs, req.MerchantId) {
                return connect.NewError(connect.CodePermissionDenied,
                    errors.New("merchant not in allowed list"))
            }
            // They can query that specific merchant
        } else {
            // No merchant specified - return ALL their merchants
            // Add WHERE merchant_id IN (...) to SQL query
            req.MerchantIds = token.MerchantIDs
        }
    }

    return s.db.ListTransactions(ctx, req)
}
```

---

### 3. **Customer Token** (Signed-In User)

**Use Case**: Customer checking their order history, managing saved cards

**Token**:
```json
{
    "sub": "customer_xyz789",
    "token_type": "customer",
    "merchant_id": null,
    "customer_id": "customer_xyz789",
    "scopes": ["payments:read", "payment_methods:read"],
    "exp": 1736683200
}
```

**Authorization Rules**:
```go
// CREATE OPERATIONS
❌ Customers CANNOT directly create payments
   - They go through merchant's e-commerce backend

// READ OPERATIONS
✅ ListTransactions: FORCE filter WHERE customer_id = "customer_xyz789"
✅ GetTransaction: Check transaction.customer_id = "customer_xyz789"

// WHAT THEY SEE
✅ Only THEIR OWN transactions
✅ Across ALL merchants they've paid
❌ Cannot see other customers' transactions
❌ Cannot see merchant's other customers
```

**Implementation**:
```go
func (s *PaymentService) ListTransactions(ctx context.Context, req *ListTransactionsRequest) {
    token := getTokenFromContext(ctx)

    // Customer can ONLY see their own transactions
    if token.CustomerID != nil {
        req.CustomerId = *token.CustomerID  // FORCE to their customer_id
        req.MerchantId = ""                 // Clear any merchant_id they tried to pass
    }

    return s.db.ListTransactions(ctx, req)
}
```

---

### 4. **Guest Token** (Anonymous Checkout)

**Use Case**: Guest checking out without account, tracking order status

**Token** (short-lived, 30 min):
```json
{
    "sub": "guest_session_abc",
    "token_type": "guest",
    "merchant_id": "merchant_123",
    "customer_id": null,
    "session_id": "sess_abc123",
    "scopes": ["payments:create"],
    "exp": 1736685000
}
```

**Authorization Rules**:
```go
// CREATE OPERATIONS
✅ Can create payment for merchant_id = "merchant_123"
❌ REJECT if request.merchant_id != token.merchant_id

// READ OPERATIONS
✅ GetTransaction: ONLY if transaction.session_id = "sess_abc123"
❌ ListTransactions: NOT ALLOWED (guests can't list)

// WHAT THEY SEE
✅ Only transactions from THEIR session
❌ No listing capability
❌ After session expires, need email lookup fallback
```

**Implementation**:
```go
func (s *PaymentService) GetTransaction(ctx context.Context, req *GetTransactionRequest) {
    token := getTokenFromContext(ctx)

    tx, err := s.db.GetTransaction(ctx, req.TransactionId)
    if err != nil {
        return nil, connect.NewError(connect.CodeNotFound, err)
    }

    // Guest authorization: check session_id
    if token.SessionID != nil {
        if tx.Metadata["session_id"] != *token.SessionID {
            // Return 404, not 403 (prevent enumeration)
            return nil, connect.NewError(connect.CodeNotFound,
                errors.New("transaction not found"))
        }
    }

    return tx, nil
}

func (s *PaymentService) ListTransactions(ctx context.Context, req *ListTransactionsRequest) {
    token := getTokenFromContext(ctx)

    // Guests cannot list transactions
    if token.TokenType == "guest" {
        return nil, connect.NewError(connect.CodePermissionDenied,
            errors.New("guests cannot list transactions"))
    }

    // ...
}
```

---

### 5. **Admin Token** (Platform Administrator)

**Use Case**: Support staff investigating disputes, refunds, debugging

**Token**:
```json
{
    "sub": "admin_support_001",
    "token_type": "admin",
    "merchant_id": null,
    "customer_id": null,
    "merchant_ids": null,
    "scopes": ["*"],
    "exp": 1736683200
}
```

**Authorization Rules**:
```go
// ALL OPERATIONS
✅ No forced filters - can query ANY merchant, ANY customer
✅ Can create payments for ANY merchant
✅ Can void/refund ANY transaction
⚠️ ALL actions logged to audit trail

// WHAT THEY SEE
✅ Everything
```

**Implementation**:
```go
func (s *PaymentService) ListTransactions(ctx context.Context, req *ListTransactionsRequest) {
    token := getTokenFromContext(ctx)

    // Admin bypass - no forced filters
    if token.TokenType == "admin" {
        // Log the access
        s.auditLog.Log(ctx, "admin_list_transactions", token.Sub, req)

        // Return whatever they asked for
        return s.db.ListTransactions(ctx, req)
    }

    // ... apply filters for other token types
}
```

---

## API Authorization Matrix

### Create Operations (Authorize, Sale, Capture, Void, Refund)

| Operation | Merchant Token | Multi-Merchant | Customer | Guest | Admin |
|-----------|---------------|----------------|----------|-------|-------|
| **merchant_id in request** | Must match token | Must be in list | N/A | Must match token | Any |
| **Scope Required** | `payments:create` | `payments:create` | N/A | `payments:create` | Any |

**Key Rule**: `merchant_id` in request MUST be validated against token

```go
func validateMerchantAccess(token *TokenClaims, requestedMerchantID string) error {
    // Admin bypass
    if token.TokenType == "admin" {
        return nil
    }

    // Single merchant token
    if token.MerchantID != nil {
        if requestedMerchantID != *token.MerchantID {
            return ErrUnauthorized
        }
        return nil
    }

    // Multi-merchant token
    if len(token.MerchantIDs) > 0 {
        if !contains(token.MerchantIDs, requestedMerchantID) {
            return ErrUnauthorized
        }
        return nil
    }

    // No merchant access
    return ErrUnauthorized
}
```

---

### List Operations (ListTransactions)

| Token Type | Filter Behavior | What They See |
|------------|----------------|---------------|
| **Merchant (single)** | FORCE `merchant_id = token.merchant_id` | Only their merchant |
| **Merchant (multi)** | FORCE `merchant_id IN token.merchant_ids` | Only their merchants |
| **Customer** | FORCE `customer_id = token.customer_id` | Only their transactions |
| **Guest** | REJECT (not allowed) | Nothing |
| **Admin** | No forced filters | Everything |

**Implementation**:
```go
func (s *PaymentService) ListTransactions(ctx context.Context, req *ListTransactionsRequest) (*ListTransactionsResponse, error) {
    token := getTokenFromContext(ctx)

    // Apply token-based filters
    if err := s.applyTokenFilters(token, req); err != nil {
        return nil, err
    }

    // Execute query with filters
    return s.db.ListTransactions(ctx, req)
}

func (s *PaymentService) applyTokenFilters(token *TokenClaims, req *ListTransactionsRequest) error {
    switch token.TokenType {
    case "merchant":
        if token.MerchantID != nil {
            // Single merchant - FORCE filter
            req.MerchantId = *token.MerchantID
        } else if len(token.MerchantIDs) > 0 {
            // Multi-merchant
            if req.MerchantId != "" {
                // They requested specific merchant - validate it's allowed
                if !contains(token.MerchantIDs, req.MerchantId) {
                    return ErrUnauthorized
                }
            } else {
                // No specific merchant - query all their merchants
                req.MerchantIds = token.MerchantIDs
            }
        }

    case "customer":
        // FORCE customer filter
        req.CustomerId = *token.CustomerID
        req.MerchantId = "" // Clear merchant filter

    case "guest":
        return connect.NewError(connect.CodePermissionDenied,
            errors.New("guests cannot list transactions"))

    case "admin":
        // No filters - let them query whatever they want

    default:
        return ErrInvalidTokenType
    }

    return nil
}
```

---

### Get Operations (GetTransaction)

| Token Type | Authorization Check |
|------------|-------------------|
| **Merchant (single)** | `transaction.merchant_id == token.merchant_id` |
| **Merchant (multi)** | `transaction.merchant_id IN token.merchant_ids` |
| **Customer** | `transaction.customer_id == token.customer_id` |
| **Guest** | `transaction.session_id == token.session_id` |
| **Admin** | Always allowed |

**Implementation**:
```go
func (s *PaymentService) GetTransaction(ctx context.Context, req *GetTransactionRequest) (*Transaction, error) {
    token := getTokenFromContext(ctx)

    // Fetch transaction
    tx, err := s.db.GetTransaction(ctx, req.TransactionId)
    if err != nil {
        return nil, connect.NewError(connect.CodeNotFound, err)
    }

    // Check access
    if !s.canAccessTransaction(token, tx) {
        // Return 404, not 403 (prevent enumeration)
        return nil, connect.NewError(connect.CodeNotFound,
            errors.New("transaction not found"))
    }

    return tx, nil
}

func (s *PaymentService) canAccessTransaction(token *TokenClaims, tx *Transaction) bool {
    switch token.TokenType {
    case "merchant":
        if token.MerchantID != nil {
            return tx.MerchantID == *token.MerchantID
        }
        if len(token.MerchantIDs) > 0 {
            return contains(token.MerchantIDs, tx.MerchantID)
        }

    case "customer":
        return tx.CustomerID != nil && *tx.CustomerID == *token.CustomerID

    case "guest":
        sessionID, ok := tx.Metadata["session_id"]
        return ok && sessionID == *token.SessionID

    case "admin":
        return true

    default:
        return false
    }

    return false
}
```

---

## Problem: "merchant_id" and "customer_id" in Request vs Token

### Current API Issue
```protobuf
message ListTransactionsRequest {
  string merchant_id = 1;  // ⚠️ Client can pass ANY merchant_id
  string customer_id = 2;  // ⚠️ Client can pass ANY customer_id
  // ...
}
```

### Solution: Token Overrides Request Parameters

**Rule**: Token context takes precedence over request parameters

```go
// ❌ BAD: Trust the request
func ListTransactions(req *ListTransactionsRequest) {
    // Query WHERE merchant_id = req.MerchantId
    // Attacker can query any merchant!
}

// ✅ GOOD: Token overrides request
func ListTransactions(token *TokenClaims, req *ListTransactionsRequest) {
    // Merchant token with single merchant
    if token.MerchantID != nil {
        req.MerchantId = *token.MerchantID  // OVERRIDE whatever they passed
    }

    // Customer token
    if token.CustomerID != nil {
        req.CustomerId = *token.CustomerID   // OVERRIDE
        req.MerchantId = ""                  // CLEAR merchant filter
    }

    // Multi-merchant token
    if len(token.MerchantIDs) > 0 {
        if req.MerchantId != "" {
            // Validate it's in allowed list
            if !contains(token.MerchantIDs, req.MerchantId) {
                return ErrUnauthorized
            }
        } else {
            // Add WHERE merchant_id IN (...) filter
            req.MerchantIds = token.MerchantIDs
        }
    }
}
```

---

## Token Issuance (Who Creates Tokens?)

### Merchant Tokens
**Issued by**: POS Backend or Merchant Portal
**When**: Staff logs in to POS terminal
**Contains**: Single `merchant_id` or list of `merchant_ids`

```typescript
// POS Backend issues token after staff login
app.post('/api/pos/login', async (req, res) => {
    const { username, password, terminalId } = req.body;

    // Validate staff credentials
    const staff = await db.validateStaff(username, password);

    // Issue JWT with merchant context
    const token = jwt.sign({
        sub: `pos_${terminalId}`,
        token_type: 'merchant',
        merchant_id: staff.merchant_id,
        scopes: ['payments:create', 'payments:read', 'payments:void', 'payments:refund'],
        exp: Math.floor(Date.now() / 1000) + (8 * 3600), // 8 hours
    }, JWT_SECRET);

    res.json({ token });
});
```

### Customer Tokens
**Issued by**: E-commerce Backend
**When**: Customer logs in to their account
**Contains**: `customer_id`

```typescript
// E-commerce backend issues token after login
app.post('/api/login', async (req, res) => {
    const { email, password } = req.body;

    // Validate customer credentials
    const customer = await db.validateCustomer(email, password);

    // Issue JWT with customer context
    const token = jwt.sign({
        sub: customer.id,
        token_type: 'customer',
        customer_id: customer.id,
        scopes: ['payments:read', 'payment_methods:read'],
        exp: Math.floor(Date.now() / 1000) + (24 * 3600), // 24 hours
    }, JWT_SECRET);

    res.json({ token });
});
```

### Guest Tokens
**Issued by**: E-commerce Backend
**When**: Guest starts checkout
**Contains**: `session_id`, `merchant_id`

```typescript
// Guest checkout flow
app.post('/api/guest-checkout', async (req, res) => {
    const sessionId = req.session.id;
    const merchantId = req.body.merchant_id;

    // Issue short-lived guest token
    const token = jwt.sign({
        sub: `guest_${sessionId}`,
        token_type: 'guest',
        merchant_id: merchantId,
        session_id: sessionId,
        scopes: ['payments:create'],
        exp: Math.floor(Date.now() / 1000) + (30 * 60), // 30 minutes
    }, JWT_SECRET);

    res.json({ token });
});
```

---

## ConnectRPC Implementation

### Interceptor (Middleware)
```go
// internal/middleware/auth_interceptor.go
package middleware

import (
    "context"
    "errors"
    "strings"

    "connectrpc.com/connect"
    "github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
    jwt.RegisteredClaims
    TokenType   string   `json:"token_type"`
    MerchantID  *string  `json:"merchant_id"`
    CustomerID  *string  `json:"customer_id"`
    SessionID   *string  `json:"session_id"`
    MerchantIDs []string `json:"merchant_ids"`
    Scopes      []string `json:"scopes"`
}

type contextKey string

const tokenClaimsKey contextKey = "token_claims"

func NewAuthInterceptor(jwtSecret []byte) connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            // Extract Authorization header
            authHeader := req.Header().Get("Authorization")
            if authHeader == "" {
                return nil, connect.NewError(connect.CodeUnauthenticated,
                    errors.New("missing authorization header"))
            }

            // Parse Bearer token
            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            if tokenString == authHeader {
                return nil, connect.NewError(connect.CodeUnauthenticated,
                    errors.New("invalid authorization format"))
            }

            // Parse JWT
            token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{},
                func(token *jwt.Token) (interface{}, error) {
                    return jwtSecret, nil
                })

            if err != nil {
                return nil, connect.NewError(connect.CodeUnauthenticated, err)
            }

            if !token.Valid {
                return nil, connect.NewError(connect.CodeUnauthenticated,
                    errors.New("invalid token"))
            }

            claims := token.Claims.(*TokenClaims)

            // Store claims in context
            ctx = context.WithValue(ctx, tokenClaimsKey, claims)

            // Call next handler
            return next(ctx, req)
        }
    }
}

func GetTokenFromContext(ctx context.Context) (*TokenClaims, error) {
    claims, ok := ctx.Value(tokenClaimsKey).(*TokenClaims)
    if !ok {
        return nil, errors.New("no token in context")
    }
    return claims, nil
}
```

---

## Summary

### Token Design Principles
1. ✅ **Token carries context** - No external role lookups
2. ✅ **Forced filters** - Token overrides request parameters
3. ✅ **Simple validation** - Just check token fields
4. ✅ **Scalable** - Multi-merchant support via `merchant_ids` array
5. ✅ **404 not 403** - Prevent resource enumeration

### Token Types Summary
| Type | merchant_id | merchant_ids | customer_id | session_id | Use Case |
|------|------------|--------------|-------------|------------|----------|
| **merchant** | ✅ Single | null | null | null | POS cashier (1 store) |
| **merchant** | null | ✅ Array | null | null | Remote operator (N stores) |
| **customer** | null | null | ✅ | null | Logged-in customer |
| **guest** | ✅ | null | null | ✅ | Anonymous checkout |
| **admin** | null | null | null | null | Support staff |

### Request Parameter Handling
| Parameter | Merchant Token | Customer Token | Guest Token | Admin Token |
|-----------|---------------|----------------|-------------|-------------|
| `merchant_id` | **FORCED** to token | **IGNORED** | **VALIDATED** | Allowed |
| `customer_id` | Allowed | **FORCED** to token | null | Allowed |
| `group_id` | Checked | Checked | Checked | Allowed |

---

**Next Steps**:
1. Implement ConnectRPC interceptor
2. Add token validation to all handlers
3. Update proto services to use ConnectRPC
4. Write authorization tests

