# Browser Post Refactoring Plan

## Overview
Refactor the Browser Post implementation to use frontend-controlled UUIDs, eliminate PENDING transactions, integrate TAC (Terminal Authorization Code) from EPX Key Exchange, and rename agent → merchant throughout the codebase.

## Current State Analysis

### Problems with Current Implementation

1. **Redundant UUID Fields**
   - Database has both `id` (primary key) and `idempotency_key` (unique constraint)
   - Backend generates transaction ID, frontend has no control
   - Two fields serving the same purpose

2. **PENDING Transactions**
   - `GetPaymentForm` creates PENDING transaction immediately
   - Creates dangling records if user never submits form
   - Requires UPDATE operation on callback instead of INSERT

3. **Missing TAC Integration**
   - Key Exchange adapter exists but isn't used
   - GetPaymentForm doesn't call EPX Key Exchange
   - TAC is required for Browser Post security

4. **Inconsistent Terminology**
   - Uses "agent_id" but should be "merchant_id"
   - Need systematic rename throughout codebase

### Current Flow

```
Frontend Request
    ↓
GET /api/v1/payments/browser-post/form?amount=100&return_url=...&agent_id=...
    ↓
Handler generates UUIDs (transaction_id, group_id)
    ↓
Creates PENDING transaction in database ❌
    ↓
Returns form config (no TAC) ❌
    ↓
Frontend submits form to EPX
    ↓
EPX processes payment
    ↓
Callback to /callback
    ↓
Handler UPDATES transaction from PENDING → COMPLETED/FAILED
```

## Desired State Architecture

### Design Principles

1. **Frontend-Controlled Primary Keys**: Frontend generates UUID, uses as transaction primary key
2. **No Speculative Writes**: Only create transaction when payment actually happens (callback)
3. **TAC Integration**: Call Key Exchange to get TAC for form security
4. **Single Source of Truth**: One UUID field, no redundant idempotency_key
5. **Clear Terminology**: Use "merchant" instead of "agent"

### New Flow

```
Frontend generates UUID (transaction_id)
    ↓
GET /api/v1/payments/browser-post/form?transaction_id={uuid}&merchant_id=...&amount=100&return_url=...
    ↓
Handler validates parameters
    ↓
Handler calls EPX Key Exchange with transaction_id
    ↓
EPX returns TAC (expires in 4 hours)
    ↓
Handler returns JSON response:
  - TAC
  - merchant credentials
  - transaction_id (echoed back)
  - postURL
  - static config
    ↓
Frontend builds HTML form with TAC
    ↓
Form submits directly to EPX
    ↓
EPX processes payment
    ↓
Callback to /callback with transaction_id
    ↓
Handler creates transaction (INSERT with frontend UUID as primary key)
    ↓
If duplicate transaction_id → idempotent (return existing)
```

## Implementation Plan

### Phase 1: Database Schema Migration

**File**: `/home/kevinlam/Documents/projects/payments/internal/db/migrations/003_refactor_transactions.sql`

**Changes**:
1. Remove `idempotency_key` column (redundant with `id`)
2. Remove unique constraint on `idempotency_key`
3. Remove index `idx_transactions_idempotency_key`
4. Rename `agent_id` → `merchant_id` throughout
5. Update indexes to use `merchant_id`

**Migration**:
```sql
-- +goose Up
-- +goose StatementBegin

-- Drop old index
DROP INDEX IF EXISTS idx_transactions_idempotency_key;

-- Remove idempotency_key column
ALTER TABLE transactions DROP COLUMN IF EXISTS idempotency_key;

-- Rename agent_id to merchant_id
ALTER TABLE transactions RENAME COLUMN agent_id TO merchant_id;

-- Update indexes
DROP INDEX IF EXISTS idx_transactions_agent_id;
DROP INDEX IF EXISTS idx_transactions_agent_customer;
CREATE INDEX idx_transactions_merchant_id ON transactions(merchant_id);
CREATE INDEX idx_transactions_merchant_customer ON transactions(merchant_id, customer_id) WHERE customer_id IS NOT NULL;

-- Update subscriptions table
ALTER TABLE subscriptions RENAME COLUMN agent_id TO merchant_id;
DROP INDEX IF EXISTS idx_subscriptions_agent_id;
DROP INDEX IF EXISTS idx_subscriptions_agent_customer;
CREATE INDEX idx_subscriptions_merchant_id ON subscriptions(merchant_id);
CREATE INDEX idx_subscriptions_merchant_customer ON subscriptions(merchant_id, customer_id);

-- Update audit_logs table
ALTER TABLE audit_logs RENAME COLUMN agent_id TO merchant_id;
DROP INDEX IF EXISTS idx_audit_logs_agent_id;
CREATE INDEX idx_audit_logs_merchant_id ON audit_logs(merchant_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Reverse all changes
ALTER TABLE audit_logs RENAME COLUMN merchant_id TO agent_id;
DROP INDEX IF EXISTS idx_audit_logs_merchant_id;
CREATE INDEX idx_audit_logs_agent_id ON audit_logs(agent_id);

ALTER TABLE subscriptions RENAME COLUMN merchant_id TO agent_id;
DROP INDEX IF EXISTS idx_subscriptions_merchant_id;
DROP INDEX IF EXISTS idx_subscriptions_merchant_customer;
CREATE INDEX idx_subscriptions_agent_id ON subscriptions(agent_id);
CREATE INDEX idx_subscriptions_agent_customer ON subscriptions(agent_id, customer_id);

ALTER TABLE transactions RENAME COLUMN merchant_id TO agent_id;
DROP INDEX IF EXISTS idx_transactions_merchant_id;
DROP INDEX IF EXISTS idx_transactions_merchant_customer;
CREATE INDEX idx_transactions_agent_id ON transactions(agent_id);
CREATE INDEX idx_transactions_agent_customer ON transactions(agent_id, customer_id) WHERE customer_id IS NOT NULL;

ALTER TABLE transactions ADD COLUMN idempotency_key VARCHAR(255) UNIQUE;
CREATE INDEX idx_transactions_idempotency_key ON transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- +goose StatementEnd
```

### Phase 2: Update SQL Queries

**File**: `/home/kevinlam/Documents/projects/payments/internal/db/queries/transactions.sql`

**Changes**:

1. **Remove idempotency_key from CreateTransaction**:
```sql
-- name: CreateTransaction :one
INSERT INTO transactions (
    id,              -- Now accepts frontend UUID
    group_id,
    merchant_id,     -- Renamed from agent_id
    customer_id,
    amount,
    currency,
    status,
    type,
    payment_method_type,
    payment_method_id,
    auth_guid,
    auth_resp,
    auth_code,
    auth_resp_text,
    auth_card_type,
    auth_avs,
    auth_cvv2,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
) RETURNING *;
```

2. **Remove GetTransactionByIdempotencyKey** (no longer needed - use GetTransactionByID)

3. **Rename agent_id → merchant_id in all queries**:
- GetTransactionsByAgentID → GetTransactionsByMerchantID
- GetTransactionsByAgentAndCustomer → GetTransactionsByMerchantAndCustomer
- Update all WHERE clauses

**Regenerate sqlc**:
```bash
sqlc generate
```

### Phase 2.5: Update Merchant Schema and Add Constraints

**File**: `/home/kevinlam/Documents/projects/payments/internal/db/migrations/004_agent_credentials.sql`

**Changes**:
1. Change `id` from VARCHAR(255) to UUID PRIMARY KEY
2. Add `slug VARCHAR(255) NOT NULL UNIQUE` - human-readable identifier for debugging/admin
   - Auto-generated from merchant name with random hash suffix
   - Example: "WordPress Site" → "wordpress-site-a3f2"
   - Format: `slugify(name) + "-" + random_hash(4)`
3. Add UNIQUE constraint on `cust_nbr` - prevents EPX credential conflicts
4. Add FK constraints on all tables:
   - `transactions(merchant_id) REFERENCES merchants(id)`
   - `subscriptions(merchant_id) REFERENCES merchants(id)`
   - `audit_logs(merchant_id) REFERENCES merchants(id)`
   - `customer_payment_methods(merchant_id) REFERENCES merchants(id)`

**Slug Generation Logic** (to be implemented in application):
```go
func GenerateSlug(name string) string {
    // Slugify: lowercase, replace spaces/special chars with hyphens
    base := slugify(name) // "WordPress Site" → "wordpress-site"

    // Generate 4-character random hash (alphanumeric)
    hash := generateRandomHash(4) // e.g., "a3f2"

    return fmt.Sprintf("%s-%s", base, hash)
}
```

**Benefits**:
- Prevents race conditions (no need to check existence)
- Scalable (no querying in loop)
- Still human-readable
- Unique by design

**Logging Strategy**:
- **Always use UUID in logs** - permanent identifier that never changes
- Slug is ONLY for admin UI display
- Slug can change (mutable) - so logs would become confusing if we used slug
- Example log format:
  ```go
  h.logger.Info("Transaction created",
      zap.String("merchant_id", merchant.ID.String()),  // UUID - permanent
      zap.String("merchant_slug", merchant.Slug),       // Optional display hint
  )
  ```

### Phase 3: Refactor GetPaymentForm Handler

**File**: `/home/kevinlam/Documents/projects/payments/internal/handlers/payment/browser_post_callback_handler.go`

**Method**: `GetPaymentForm` (keep as GET, optionally rename to `GetPaymentFormData`)

**New Signature**:
```go
// GET /api/v1/payments/browser-post/form?transaction_id={uuid}&merchant_id={id}&amount={amount}&return_url={url}
func (h *BrowserPostCallbackHandler) GetPaymentForm(w http.ResponseWriter, r *http.Request) error
```

**Pass merchant_id through USER_DATA_3**:
- USER_DATA_1 = return_url (for redirect after payment)
- USER_DATA_2 = customer_id (optional, for logged-in users)
- USER_DATA_3 = merchant_id UUID (required, for transaction creation)

**Why pass merchant_id through USER_DATA?**
- EPX only returns CUST_NBR (EPX credential) in callback
- Multiple merchants could share same CUST_NBR (no unique constraint initially)
- Pass-through is more explicit and avoids DB lookup
- FK constraint validates merchant_id exists

**Complete Implementation**:

1. **Extract and validate query parameters**:
```go
// Extract frontend-generated transaction ID
transactionIDStr := r.URL.Query().Get("transaction_id")
if transactionIDStr == "" {
    return fmt.Errorf("transaction_id is required")
}
transactionID, err := uuid.Parse(transactionIDStr)
if err != nil {
    return fmt.Errorf("invalid transaction_id format: %w", err)
}

merchantID := r.URL.Query().Get("merchant_id")
if merchantID == "" {
    return fmt.Errorf("merchant_id is required")
}

amountStr := r.URL.Query().Get("amount")
if amountStr == "" {
    return fmt.Errorf("amount is required")
}

returnURL := r.URL.Query().Get("return_url")
if returnURL == "" {
    return fmt.Errorf("return_url is required")
}
```

2. **Generate group_id for EPX Key Exchange** (not stored yet):
```go
// Generate group ID for this payment session
groupID := uuid.New()
```

3. **Call Key Exchange to get TAC**:
```go
// Call EPX Key Exchange to get TAC
keyExchangeReq := &ports.KeyExchangeRequest{
    MerchantID:  merchantID,  // Renamed from AgentID
    CustNbr:     h.config.EPXCustNbr,
    MerchNbr:    h.config.EPXMerchNbr,
    DBAnbr:      h.config.EPXDBAnbr,
    TerminalNbr: h.config.EPXTerminalNbr,
    MAC:         h.config.EPXMAC,
    Amount:      amountStr,
    TranNbr:     transactionID.String(),  // Use frontend UUID
    TranGroup:   groupID.String(),
    RedirectURL: fmt.Sprintf("%s/api/v1/payments/browser-post/callback", h.config.BaseURL),
}

keyExchangeResp, err := h.keyExchangeAdapter.GetTAC(r.Context(), keyExchangeReq)
if err != nil {
    h.logger.Error("Failed to get TAC from Key Exchange",
        zap.Error(err),
        zap.String("transaction_id", transactionID.String()),
        zap.String("merchant_id", merchantID),
    )
    return fmt.Errorf("failed to get TAC: %w", err)
}
```

4. **Return form configuration with TAC** (NO database write):
```go
// Build response with TAC and credentials
formConfig := map[string]interface{}{
    // Frontend UUID echoed back
    "transactionId": transactionID.String(),
    "groupId":       groupID.String(),

    // TAC from Key Exchange
    "tac":       keyExchangeResp.TAC,
    "expiresAt": keyExchangeResp.ExpiresAt.Unix(),

    // EPX endpoint
    "postURL": h.config.EPXBrowserPostURL,

    // Merchant credentials (static)
    "custNbr":     h.config.EPXCustNbr,
    "merchNbr":    h.config.EPXMerchNbr,
    "dbaName":     h.config.EPXDBAnbr,
    "terminalNbr": h.config.EPXTerminalNbr,

    // Static config
    "industryType": "E",  // E-commerce
    "tranType":     "S",  // Sale (auth + capture)

    // Return URL for EPX callback
    "returnURL": returnURL,
}

w.Header().Set("Content-Type", "application/json")
return json.NewEncoder(w).Encode(formConfig)
```

**Key Points**:
- Endpoint remains GET (no database write)
- Accepts frontend UUID as parameter
- Calls Key Exchange to get TAC
- Returns TAC + credentials
- No PENDING transaction creation

### Phase 4: Refactor HandleCallback

**File**: `/home/kevinlam/Documents/projects/payments/internal/handlers/payment/browser_post_callback_handler.go`

**Method**: `HandleCallback`

**Implementation Changes**:

1. **Extract transaction_id from callback** (TRAN_NBR field):
```go
tranNbr := r.FormValue("TRAN_NBR")
transactionID, err := uuid.Parse(tranNbr)
if err != nil {
    h.logger.Error("Invalid transaction ID in callback",
        zap.Error(err),
        zap.String("tran_nbr", tranNbr),
    )
    return fmt.Errorf("invalid transaction ID: %w", err)
}
```

2. **Extract group_id from callback** (TRAN_GROUP field):
```go
tranGroup := r.FormValue("TRAN_GROUP")
groupID, err := uuid.Parse(tranGroup)
if err != nil {
    h.logger.Error("Invalid group ID in callback",
        zap.Error(err),
        zap.String("tran_group", tranGroup),
    )
    return fmt.Errorf("invalid group ID: %w", err)
}
```

3. **Attempt to create transaction with frontend UUID as primary key**:
```go
// Try to create transaction (INSERT with frontend UUID)
tx, err := h.dbAdapter.Queries().CreateTransaction(ctx, sqlc.CreateTransactionParams{
    ID:                transactionID,  // Frontend UUID as primary key
    GroupID:           groupID,
    MerchantID:        merchantID,  // Renamed from AgentID
    CustomerID:        pgtype.Text{String: customerID, Valid: customerID != ""},
    Amount:            amount,
    Currency:          "USD",
    Status:            status,
    Type:              "charge",
    PaymentMethodType: paymentMethodType,
    PaymentMethodID:   paymentMethodUUID,
    AuthGuid:          pgtype.Text{String: authGuid, Valid: true},
    AuthResp:          pgtype.Text{String: authResp, Valid: true},
    AuthCode:          pgtype.Text{String: authCode, Valid: true},
    AuthRespText:      pgtype.Text{String: authRespText, Valid: true},
    AuthCardType:      pgtype.Text{String: authCardType, Valid: true},
    AuthAvs:           pgtype.Text{String: authAvs, Valid: true},
    AuthCvv2:          pgtype.Text{String: authCvv2, Valid: true},
    Metadata:          metadata,
})

if err != nil {
    // Check if it's a duplicate key violation (idempotent retry)
    if isDuplicateKeyError(err) {
        h.logger.Info("Duplicate transaction callback (idempotent)",
            zap.String("transaction_id", transactionID.String()),
        )

        // Fetch existing transaction
        existingTx, err := h.dbAdapter.Queries().GetTransactionByID(ctx, transactionID)
        if err != nil {
            return fmt.Errorf("failed to fetch existing transaction: %w", err)
        }

        // Return success with existing transaction
        return h.redirectWithSuccess(w, r, existingTx, returnURL)
    }

    return fmt.Errorf("failed to create transaction: %w", err)
}
```

4. **Handle success/failure redirect**:
```go
// Redirect to return URL with transaction details
return h.redirectWithSuccess(w, r, tx, returnURL)
```

**Key Points**:
- Uses frontend UUID directly as primary key
- INSERT operation (not UPDATE)
- Duplicate key = idempotent retry (fetch existing)
- No PENDING transaction lookup

### Phase 5: Update Port Interface

**File**: `/home/kevinlam/Documents/projects/payments/internal/adapters/ports/key_exchange.go`

**Rename AgentID → MerchantID**:
```go
type KeyExchangeRequest struct {
    MerchantID  string  // Renamed from AgentID
    CustNbr     string
    MerchNbr    string
    DBAnbr      string
    TerminalNbr string
    MAC         string
    Amount      string
    TranNbr     string  // Frontend UUID
    TranGroup   string
    RedirectURL string
}
```

### Phase 6: Update Key Exchange Adapter

**File**: `/home/kevinlam/Documents/projects/payments/internal/adapters/epx/key_exchange_adapter.go`

**Rename references to AgentID → MerchantID**:
- Update struct field references
- Update logging statements
- Update error messages

### Phase 7: Systematic Agent → Merchant Rename

**Files to Update**:
1. All handler files
2. All service files
3. All adapter files
4. All test files
5. Proto definitions
6. Configuration structs
7. Documentation

**Search and Replace**:
```bash
# Find all occurrences
grep -r "agent_id\|agentId\|agentID\|AgentID" --include="*.go" --include="*.proto" --include="*.sql"

# Systematic replacements (case-sensitive):
agent_id    → merchant_id
agentId     → merchantId
agentID     → merchantID
AgentID     → MerchantID
agent       → merchant (where contextually appropriate)
Agent       → Merchant (where contextually appropriate)
```

### Phase 8: Update Documentation

**Files to Update**:

1. **BROWSER_POST_DATAFLOW.md**:
   - Remove PENDING transaction creation
   - Add TAC flow from Key Exchange
   - Show frontend UUID as primary key
   - Update terminology (agent → merchant)

2. **BROWSER_POST_FRONTEND_GUIDE.md**:
   - Add UUID generation instructions
   - Update API endpoint parameters
   - Add TAC handling
   - Show new response format

3. **Create BROWSER_POST_MIGRATION.md**:
   - Document migration from old to new architecture
   - Explain breaking changes
   - Provide before/after examples

### Phase 9: Update WordPress Plugin

**File**: `/home/kevinlam/Documents/projects/north-payments-wordpress/includes/class-north-payment-gateway.php`

**Changes**:
1. Generate UUID on frontend (JavaScript)
2. Call GET endpoint with transaction_id parameter
3. Render form with TAC locally
4. Update callback handling

## Implementation Timeline

### Step 1: Database Migration (30 minutes)
- Create migration file
- Test locally
- Run migration

### Step 2: Update SQL Queries (20 minutes)
- Modify queries
- Regenerate sqlc
- Fix compilation errors

### Step 3: Refactor GetPaymentForm (1 hour)
- Update handler
- Integrate Key Exchange
- Test TAC retrieval

### Step 4: Refactor HandleCallback (45 minutes)
- Update to use frontend UUID
- Handle idempotency via primary key
- Test callback flow

### Step 5: Systematic Rename (1 hour)
- Search and replace
- Fix compilation errors
- Update tests

### Step 6: Update Documentation (30 minutes)
- Revise dataflow docs
- Update frontend guide
- Create migration guide

### Step 7: Update WordPress Plugin (1 hour)
- Add UUID generation
- Update API calls
- Test end-to-end

**Total Estimated Time**: ~5 hours

## Risks and Mitigation

### Risk 1: Breaking Changes for Existing Clients
- **Impact**: HIGH - API contract changes
- **Mitigation**:
  - Version the API (keep old endpoint temporarily)
  - Provide migration guide
  - Coordinate with frontend teams

### Risk 2: Data Migration Issues
- **Impact**: MEDIUM - Existing transactions have idempotency_key
- **Mitigation**:
  - Migration only affects schema, not data
  - idempotency_key column drop is safe (data moves to id)
  - Test migration on staging first

### Risk 3: TAC Integration Failures
- **Impact**: HIGH - Payments won't work without TAC
- **Mitigation**:
  - Key Exchange adapter already exists and tested
  - Add retry logic
  - Implement fallback error handling
  - Monitor TAC expiration

### Risk 4: Idempotency Handling
- **Impact**: MEDIUM - Duplicate callbacks could create issues
- **Mitigation**:
  - Primary key constraint enforces uniqueness
  - Handle duplicate key errors gracefully
  - Log all idempotent retries

## Testing Strategy

### Unit Tests
- Test GetPaymentForm parameter validation
- Test Key Exchange TAC retrieval
- Test HandleCallback idempotency
- Test UUID parsing and validation

### Integration Tests
- Test full Browser Post flow end-to-end
- Test callback with duplicate transaction_id
- Test TAC expiration handling
- Test merchant credential validation

### Manual Testing
- Test WordPress plugin integration
- Test form submission to EPX sandbox
- Test callback handling
- Test error scenarios

## Success Criteria

1. ✅ Database schema simplified (single UUID field)
2. ✅ No PENDING transactions created
3. ✅ TAC integrated from Key Exchange
4. ✅ Frontend controls transaction ID
5. ✅ Idempotency via primary key constraint
6. ✅ All "agent" references renamed to "merchant"
7. ✅ Documentation updated
8. ✅ WordPress plugin working end-to-end
9. ✅ All tests passing

## Rollback Plan

If issues arise:

1. **Database**: Run migration down
```bash
goose -dir internal/db/migrations postgres "connection_string" down
```

2. **Code**: Revert git commits
```bash
git revert HEAD~5  # Revert last 5 commits
```

3. **API**: Keep old endpoint active temporarily for backward compatibility

## Next Steps

After reviewing this plan:
1. Get approval for breaking changes
2. Schedule implementation window
3. Coordinate with frontend teams
4. Execute migration in phases
5. Monitor production metrics post-deployment
