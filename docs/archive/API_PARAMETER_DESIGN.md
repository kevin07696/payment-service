# API Parameter Design: Token Context vs Request Data

## Problem Statement

**Current Issue**: APIs require `merchant_id` in every request, even when token already contains it.

```protobuf
// ❌ CURRENT: Redundant merchant_id
message AuthorizeRequest {
  string merchant_id = 1;  // Why? Token already has this for single-merchant!
  string customer_id = 2;
  string amount = 3;
  // ...
}
```

**Use Cases**:
1. **Local Cashier** (Single Merchant): Token has `merchant_id` → Don't make them pass it again
2. **Remote Operator** (Multi-Merchant): Token has `merchant_ids` list → They MUST specify which merchant

---

## Solution: Conditional Parameter Requirement

### Rule: `merchant_id` is **optional** in request, **required** by authorization

```protobuf
message AuthorizeRequest {
  string merchant_id = 1;  // Optional: Only if token has multiple merchants
  string customer_id = 2;
  string amount = 3;
  string currency = 4;
  // ...
}
```

**Authorization Logic**:
```go
func (s *PaymentService) Authorize(ctx context.Context, req *AuthorizeRequest) {
    token := getTokenFromContext(ctx)

    // Determine actual merchant_id to use
    merchantID, err := s.resolveMerchantID(token, req.MerchantId)
    if err != nil {
        return nil, err
    }

    // Use resolved merchantID for payment processing
    // ...
}

func (s *PaymentService) resolveMerchantID(token *TokenClaims, requestedMerchantID string) (string, error) {
    // Case 1: Single-merchant token (local cashier)
    if token.MerchantID != nil {
        // Ignore what they passed, use token's merchant
        return *token.MerchantID, nil
    }

    // Case 2: Multi-merchant token (operator)
    if len(token.MerchantIDs) > 0 {
        // They MUST specify which merchant
        if requestedMerchantID == "" {
            return "", connect.NewError(connect.CodeInvalidArgument,
                errors.New("merchant_id required for multi-merchant token"))
        }

        // Validate it's in their allowed list
        if !contains(token.MerchantIDs, requestedMerchantID) {
            return "", connect.NewError(connect.CodePermissionDenied,
                errors.New("merchant_id not in allowed list"))
        }

        return requestedMerchantID, nil
    }

    // Case 3: Customer/Guest token
    if token.TokenType == "customer" || token.TokenType == "guest" {
        // Customers/guests don't create payments directly
        return "", connect.NewError(connect.CodePermissionDenied,
            errors.New("invalid token type for payment creation"))
    }

    // Case 4: Admin token
    if token.TokenType == "admin" {
        // Admin MUST specify merchant
        if requestedMerchantID == "" {
            return "", connect.NewError(connect.CodeInvalidArgument,
                errors.New("merchant_id required for admin"))
        }
        return requestedMerchantID, nil
    }

    return "", connect.NewError(connect.CodeUnauthenticated,
        errors.New("invalid token"))
}
```

---

## API Design Examples

### 1. Authorize/Sale/Capture (Create Payment)

#### Local Cashier Usage (Single Merchant)
```typescript
// Token:
{
    token_type: "merchant",
    merchant_id: "merchant_123",
    scopes: ["payments:create"]
}

// API Call: NO merchant_id needed
const payment = await client.authorize({
    // merchant_id: OMITTED (uses token's merchant_id)
    customer_id: "walk_in_customer_001",
    amount: "25.00",
    currency: "USD",
    payment_method_id: "pm_abc123",
    idempotency_key: "auth_001"
});

// Backend resolves: merchant_id = "merchant_123" from token
```

#### Remote Operator Usage (Multi-Merchant)
```typescript
// Token:
{
    token_type: "merchant",
    merchant_ids: ["merchant_1", "merchant_2", "merchant_3"],
    scopes: ["payments:create"]
}

// API Call: MUST specify merchant_id
const payment = await client.authorize({
    merchant_id: "merchant_2",  // ✅ REQUIRED (operator has multiple)
    customer_id: "customer_xyz",
    amount: "50.00",
    currency: "USD",
    payment_method_id: "pm_xyz789",
    idempotency_key: "auth_002"
});

// Backend validates: "merchant_2" is in token.merchant_ids
```

#### Admin Usage
```typescript
// Token:
{
    token_type: "admin",
    scopes: ["*"]
}

// API Call: MUST specify merchant_id
const payment = await client.authorize({
    merchant_id: "merchant_999",  // ✅ REQUIRED (admin can use any)
    customer_id: "customer_abc",
    amount: "100.00",
    currency: "USD",
    payment_method_id: "pm_def456",
    idempotency_key: "auth_003"
});
```

---

### 2. ListTransactions (Query)

#### Local Cashier Usage
```typescript
// Token:
{
    token_type: "merchant",
    merchant_id: "merchant_123"
}

// API Call: NO merchant_id needed
const txs = await client.listTransactions({
    // merchant_id: OMITTED (forced to token's merchant)
    status: "APPROVED",
    limit: 100
});

// Backend adds: WHERE merchant_id = 'merchant_123'
```

#### Remote Operator Usage
```typescript
// Token:
{
    token_type: "merchant",
    merchant_ids: ["merchant_1", "merchant_2", "merchant_3"]
}

// Option A: Query specific merchant
const txs = await client.listTransactions({
    merchant_id: "merchant_1",  // ✅ Optional: query specific merchant
    status: "APPROVED",
    limit: 100
});
// Backend: WHERE merchant_id = 'merchant_1'

// Option B: Query all their merchants
const txs = await client.listTransactions({
    // merchant_id: OMITTED (query all their merchants)
    status: "APPROVED",
    limit: 100
});
// Backend: WHERE merchant_id IN ('merchant_1', 'merchant_2', 'merchant_3')
```

#### Customer Usage
```typescript
// Token:
{
    token_type: "customer",
    customer_id: "customer_xyz"
}

// API Call: NO customer_id needed
const txs = await client.listTransactions({
    // customer_id: OMITTED (forced to token's customer)
    // merchant_id: IGNORED (customers see all merchants they paid)
    limit: 100
});

// Backend adds: WHERE customer_id = 'customer_xyz'
```

---

## Updated Authorization Logic

### resolveMerchantID Helper
```go
// internal/services/authorization/merchant_resolver.go
package authorization

type MerchantResolver struct{}

func (r *MerchantResolver) Resolve(token *TokenClaims, requestedMerchantID string) (string, error) {
    switch token.TokenType {
    case "merchant":
        return r.resolveMerchantToken(token, requestedMerchantID)
    case "customer", "guest":
        return "", connect.NewError(connect.CodePermissionDenied,
            errors.New("customers/guests cannot create payments"))
    case "admin":
        return r.resolveAdminToken(requestedMerchantID)
    default:
        return "", connect.NewError(connect.CodeUnauthenticated,
            errors.New("invalid token type"))
    }
}

func (r *MerchantResolver) resolveMerchantToken(token *TokenClaims, requested string) (string, error) {
    // Single merchant token
    if token.MerchantID != nil {
        // Ignore requested, use token's merchant
        // (Even if they passed a different merchant_id, we use token's)
        return *token.MerchantID, nil
    }

    // Multi-merchant token
    if len(token.MerchantIDs) > 0 {
        if requested == "" {
            return "", connect.NewError(connect.CodeInvalidArgument,
                errors.New("merchant_id required: token has multiple merchants"))
        }

        if !contains(token.MerchantIDs, requested) {
            return "", connect.NewError(connect.CodePermissionDenied,
                fmt.Errorf("merchant_id '%s' not in allowed list", requested))
        }

        return requested, nil
    }

    return "", connect.NewError(connect.CodeUnauthenticated,
        errors.New("token has no merchant access"))
}

func (r *MerchantResolver) resolveAdminToken(requested string) (string, error) {
    if requested == "" {
        return "", connect.NewError(connect.CodeInvalidArgument,
            errors.New("merchant_id required for admin"))
    }
    return requested, nil
}
```

### Updated Payment Service
```go
// internal/services/payment/payment_service.go
func (s *PaymentService) Authorize(ctx context.Context, req *AuthorizeRequest) (*PaymentResponse, error) {
    token, err := middleware.GetTokenFromContext(ctx)
    if err != nil {
        return nil, err
    }

    // Check scope
    if !hasScope(token.Scopes, "payments:create") {
        return nil, connect.NewError(connect.CodePermissionDenied,
            errors.New("insufficient permissions"))
    }

    // Resolve merchant_id from token + request
    merchantID, err := s.merchantResolver.Resolve(token, req.MerchantId)
    if err != nil {
        return nil, err
    }

    // Use resolved merchantID (not req.MerchantId)
    // Process payment...
    payment := &domain.Payment{
        MerchantID: merchantID,  // ✅ From token resolution
        CustomerID: req.CustomerId,
        Amount:     req.Amount,
        // ...
    }

    return s.processAuthorization(ctx, payment)
}
```

---

## Benefits of This Approach

### 1. **Better Developer Experience**
```typescript
// ✅ Local cashier: Clean API
await client.authorize({
    amount: "25.00",
    currency: "USD",
    payment_method_id: "pm_abc"
    // No merchant_id clutter!
});

// vs

// ❌ Old way: Redundant
await client.authorize({
    merchant_id: "merchant_123",  // Redundant! Already in token
    amount: "25.00",
    // ...
});
```

### 2. **Security by Default**
```typescript
// Local cashier CANNOT bypass merchant isolation
await client.authorize({
    merchant_id: "OTHER_MERCHANT",  // ❌ Ignored! Token wins
    amount: "25.00"
});
// Backend uses token.merchant_id, not request
```

### 3. **Flexibility for Multi-Tenant**
```typescript
// Operator can specify which merchant
await client.authorize({
    merchant_id: "merchant_2",  // ✅ Validated against token list
    amount: "50.00"
});
```

### 4. **Clear Error Messages**
```go
// Multi-merchant token without merchant_id
// Error: "merchant_id required: token has multiple merchants"

// Multi-merchant token with invalid merchant_id
// Error: "merchant_id 'merchant_999' not in allowed list"

// Single-merchant token with wrong merchant_id
// (Silently uses token's merchant - no error needed)
```

---

## ListTransactions Filter Logic

### Updated Implementation
```go
func (s *PaymentService) ListTransactions(ctx context.Context, req *ListTransactionsRequest) (*ListTransactionsResponse, error) {
    token, err := middleware.GetTokenFromContext(ctx)
    if err != nil {
        return nil, err
    }

    // Build filters based on token type
    filters := s.buildFilters(token, req)

    // Execute query
    txs, total, err := s.db.ListTransactions(ctx, filters)
    if err != nil {
        return nil, err
    }

    return &ListTransactionsResponse{
        Transactions: txs,
        TotalCount:   total,
    }, nil
}

func (s *PaymentService) buildFilters(token *TokenClaims, req *ListTransactionsRequest) *db.TransactionFilters {
    filters := &db.TransactionFilters{
        Status: req.Status,
        Limit:  req.Limit,
        Offset: req.Offset,
    }

    switch token.TokenType {
    case "merchant":
        if token.MerchantID != nil {
            // Single merchant: FORCE filter
            filters.MerchantID = *token.MerchantID

        } else if len(token.MerchantIDs) > 0 {
            // Multi-merchant
            if req.MerchantId != "" {
                // They specified one - validate it
                if contains(token.MerchantIDs, req.MerchantId) {
                    filters.MerchantID = req.MerchantId
                } else {
                    // Invalid merchant_id - return empty results
                    filters.MerchantID = "INVALID"  // Will match nothing
                }
            } else {
                // No specific merchant - query all their merchants
                filters.MerchantIDs = token.MerchantIDs
            }
        }

    case "customer":
        // FORCE customer filter, ignore merchant
        filters.CustomerID = *token.CustomerID

    case "admin":
        // Use whatever they requested
        if req.MerchantId != "" {
            filters.MerchantID = req.MerchantId
        }
        if req.CustomerId != "" {
            filters.CustomerID = req.CustomerId
        }

    case "guest":
        // Guests cannot list - return error upstream
    }

    return filters
}
```

---

## Database Query Implementation

### SQL Query Builder
```go
// internal/db/sqlc/transactions.sql
-- name: ListTransactions :many
SELECT *
FROM transactions
WHERE 1=1
    -- Single merchant filter
    AND ($1::uuid IS NULL OR merchant_id = $1)
    -- Multi-merchant filter (ANY operator)
    AND ($2::uuid[] IS NULL OR merchant_id = ANY($2))
    -- Customer filter
    AND ($3::uuid IS NULL OR customer_id = $3)
    -- Status filter
    AND ($4::text IS NULL OR status = $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;
```

### SQLC Usage
```go
txs, err := s.queries.ListTransactions(ctx, db.ListTransactionsParams{
    MerchantID:  filters.MerchantID,   // Single merchant (nullable)
    MerchantIDs: filters.MerchantIDs,  // Multi-merchant (nullable array)
    CustomerID:  filters.CustomerID,   // Customer (nullable)
    Status:      filters.Status,       // Status (nullable)
    Limit:       filters.Limit,
    Offset:      filters.Offset,
})
```

---

## Summary: Parameter Design Rules

| API Parameter | Single Merchant | Multi-Merchant | Customer | Guest | Admin |
|--------------|----------------|----------------|----------|-------|-------|
| **Create Operations** |
| `merchant_id` | **Omit** (from token) | **Required** | N/A | **Omit** (from token) | **Required** |
| **Query Operations** |
| `merchant_id` | **Omit** (forced) | Optional (filter) | **Ignored** | N/A | Optional |
| `customer_id` | Optional | Optional | **Omit** (forced) | N/A | Optional |

### Key Principles
1. ✅ **Single-merchant token**: Never pass `merchant_id` in request
2. ✅ **Multi-merchant token**: Always pass `merchant_id` in request
3. ✅ **Token always wins**: Request params validated against token
4. ✅ **Clear errors**: "merchant_id required for multi-merchant token"
5. ✅ **Flexible queries**: Multi-merchant can query all or specific merchant

---

## Migration Path

### Phase 1: Make merchant_id Optional (Backward Compatible)
```protobuf
message AuthorizeRequest {
  string merchant_id = 1;  // Now optional
  // ...
}
```

### Phase 2: Update Clients
```diff
  // Old clients (still work)
- client.authorize({ merchant_id: "m123", amount: "50" })
+ client.authorize({ merchant_id: "m123", amount: "50" })  // Still works

  // New single-merchant clients
+ client.authorize({ amount: "50" })  // merchant_id from token

  // Multi-merchant clients (must specify)
+ client.authorize({ merchant_id: "m2", amount: "50" })  // Required
```

### Phase 3: Documentation
Update API docs to clarify:
- Single-merchant tokens: `merchant_id` parameter ignored (uses token)
- Multi-merchant tokens: `merchant_id` parameter required
- Errors if multi-merchant forgets `merchant_id`

