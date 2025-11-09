# POS Option 2 Refactoring - Implementation Summary

**Date**: 2025-11-09
**Branch**: develop
**Architecture**: Payment Service = Gateway Integration ONLY

## Overview

Successfully refactored Payment Service to align with POS Option 2 architecture, where:
- **POS stores complete payment data locally** (receipts, transaction history, cash payments)
- **Payment Service handles ONLY gateway integration** (EPX credit card processing)
- **No POS domain knowledge in Payment Service** (no cash, no orders, no cashiers)

---

## Changes Implemented

### ✅ 1. Database Schema (Migration 008)

**File**: `internal/db/migrations/008_pos_option2_refactoring.sql`

**Changes**:
- Added `external_reference_id VARCHAR(100)` - Opaque POS reference (e.g., "order-123")
- Added `return_url TEXT` - POS callback URL for browser redirect
- Added index `idx_transactions_external_ref` for POS queries
- Migrated existing `metadata->>'order_id'` to `external_reference_id`
- Marked `metadata` column as deprecated

**Why**: Payment Service should not understand POS domain concepts. Use opaque reference instead.

---

### ✅ 2. Proto Definitions

**File**: `proto/payment/v1/payment_browserpost.proto` (NEW)

**Changes**:
- Created new `BrowserPostService` for POS browser flow
- Added `CreateBrowserPostFormRequest` with `external_reference_id` and `return_url`
- Removed POS-specific metadata (cashier_id, table_number, etc.)
- Simplified `Transaction` message - removed metadata, added `external_reference_id`
- Removed cash payment types (Payment Service = gateway only)

**Key Message**:
```protobuf
message CreateBrowserPostFormRequest {
  string agent_id = 1;
  string amount = 2;
  string currency = 3;
  string return_url = 4;                  // ✅ NEW: POS callback URL
  string external_reference_id = 5;        // ✅ NEW: Opaque reference
  string idempotency_key = 6;
}
```

**Why**: Clean separation - Payment Service doesn't need to know about POS orders, just gateway operations.

---

### ✅ 3. JWT Service for Receipt Generation

**File**: `internal/services/jwt_service.go` (NEW)

**Features**:
- Generates signed JWT containing transaction receipt data
- 5-minute expiry (short-lived, one-time use)
- RS256 signing (RSA private key)
- Contains: transaction_id, amount, currency, status, card details, external_reference_id

**Receipt JWT Claims**:
```json
{
  "transaction_id": "uuid",
  "amount": "29.99",
  "currency": "USD",
  "status": "completed",
  "card_type": "V",
  "last_four": "4242",
  "auth_code": "123456",
  "external_reference_id": "order-123",
  "exp": 1699999999,
  "iat": 1699999694,
  "iss": "payment-service"
}
```

**Why**: POS needs transaction data without making an extra RPC call. JWT is secure, tamper-proof, and self-contained.

---

### ✅ 4. RSA Key Pair

**Location**: `keys/` (gitignored)

**Files**:
- `jwt_private_key.pem` (2048-bit RSA) - Payment Service signs JWTs
- `jwt_public_key.pem` - POS validates JWTs

**Generation**:
```bash
openssl genrsa -out keys/jwt_private_key.pem 2048
openssl rsa -in keys/jwt_private_key.pem -pubout -out keys/jwt_public_key.pem
```

**Why**: Asymmetric signing allows POS to verify receipts without shared secrets.

---

### ✅ 5. Callback Handler

**File**: `internal/handlers/browserpost/callback_handler.go` (NEW)

**Flow**:
1. **Receive EPX callback** - Browser redirects from EPX with payment result
2. **Validate EPX response** - Parse and validate MAC signature
3. **Update transaction** - Store EPX response (auth_guid, auth_code, status)
4. **Generate receipt JWT** - Create signed JWT with transaction data
5. **Redirect to POS** - Send browser to `return_url?receipt={JWT}`

**Example Redirect**:
```
https://pos.example.com/payment-callback?receipt=eyJhbGciOiJSUzI1NiIs...
```

**HTML Response**:
- Auto-redirect with `<meta http-equiv="refresh">`
- JavaScript fallback: `window.location.href`
- Manual link if JavaScript disabled
- Branded loading spinner UI

**Why**: Seamless user experience - browser flows from POS → EPX → Payment Service → back to POS with receipt.

---

### ✅ 6. Domain Model Updates

**File**: `internal/domain/transaction.go`

**Changes**:
- Added `ExternalReferenceID *string` - POS order reference
- Added `ReturnURL *string` - Where to redirect browser
- Marked `Metadata map[string]interface{}` as deprecated

**Why**: Explicit fields instead of unstructured metadata. Cleaner domain model.

---

### ✅ 7. Environment Configuration

**File**: `.env.example`

**Added**:
```bash
# JWT RECEIPT SIGNING (POS Option 2)
JWT_PRIVATE_KEY_PATH=./keys/jwt_private_key.pem
JWT_PUBLIC_KEY_PATH=./keys/jwt_public_key.pem
```

**Why**: JWT signing requires RSA keys. Configuration points to key files.

---

## Removed (Not Implemented Yet)

These changes from the refactoring guide were **NOT** implemented in this commit:

### ❌ Cash Payment Handling
- Still exists in current proto (will be removed in future commit)
- `RecordCashPayment` RPC still present
- `TRANSACTION_TYPE_CASH` enum still present

**Reason**: Deferred to separate PR to reduce scope. Cash handling removal is independent change.

### ❌ Old Proto Cleanup
- Original `payment.proto` still exists
- New `payment_browserpost.proto` added alongside (not replacing)

**Reason**: Gradual migration. Old proto can be deprecated once POS migrates.

---

## Testing Requirements

### Database Migration
```bash
# Run migration
goose -dir internal/db/migrations postgres "connection-string" up

# Verify columns
psql -d payment_service -c "\d transactions"
```

### JWT Generation
```go
// Test JWT service
jwtService := services.NewJWTService(privateKey, publicKey)
jwt, err := jwtService.GenerateReceiptJWT(txn)

// Validate JWT
claims, err := jwtService.ValidateReceiptJWT(jwt)
```

### Callback Flow
1. Create browser post form
2. Simulate EPX redirect to callback URL
3. Verify JWT generated and redirect happens
4. Verify transaction updated with EPX response

---

## Deployment Checklist

### 1. Generate RSA Keys (Production)
```bash
# Generate keys securely
openssl genrsa -out jwt_private_key.pem 2048
openssl rsa -in jwt_private_key.pem -pubout -out jwt_public_key.pem

# Store in secret manager (e.g., Google Secret Manager, Kubernetes Secrets)
# DO NOT commit keys to git
```

### 2. Run Database Migration
```bash
goose -dir internal/db/migrations postgres "$DB_URL" up
```

### 3. Update Environment Variables
```bash
export JWT_PRIVATE_KEY_PATH=/secrets/jwt_private_key.pem
export JWT_PUBLIC_KEY_PATH=/secrets/jwt_public_key.pem
```

### 4. Deploy New Version
- Build Docker image with new code
- Deploy to staging first
- Test browser post flow end-to-end
- Deploy to production

### 5. Share Public Key with POS
```bash
# POS needs public key to validate receipt JWTs
# Share jwt_public_key.pem securely
```

---

## Integration with POS

### POS Changes Required

**1. Send `return_url` when creating payment:**
```javascript
const paymentRequest = {
  agent_id: "merchant-123",
  amount: "29.99",
  currency: "USD",
  return_url: "https://pos.example.com/payment-callback",  // ← NEW
  external_reference_id: "order-789",                       // ← NEW
  idempotency_key: uuidv4()
};
```

**2. Handle receipt JWT in callback:**
```javascript
// POS callback route: /payment-callback?receipt={JWT}
app.get('/payment-callback', async (req, res) => {
  const receiptJWT = req.query.receipt;

  // Validate and decode JWT using Payment Service public key
  const claims = jwt.verify(receiptJWT, publicKey, { algorithms: ['RS256'] });

  // Extract transaction data
  const {
    transaction_id,
    amount,
    currency,
    status,
    card_type,
    last_four,
    auth_code,
    external_reference_id  // Your order ID
  } = claims;

  // Update local order with payment result
  await updateOrder(external_reference_id, {
    payment_status: status,
    payment_transaction_id: transaction_id,
    payment_amount: amount,
    payment_card_type: card_type,
    payment_last_four: last_four
  });

  // Show receipt to user
  res.render('receipt', { ...claims });
});
```

---

## Benefits

✅ **Clean Architecture** - Payment Service = Gateway Integration ONLY
✅ **No POS Domain Knowledge** - Opaque references instead of POS concepts
✅ **Secure Receipt Transfer** - JWT signed, tamper-proof, time-limited
✅ **Seamless Browser Flow** - POS → EPX → Payment Service → POS
✅ **No Extra RPC Call** - POS gets all data in JWT redirect
✅ **Multi-Tenant Ready** - External reference works for any POS order ID format

---

## Future Work

### Phase 2: Complete Cleanup
- Remove cash payment RPC and types
- Deprecate old `payment.proto`
- Remove `metadata` column from database
- Add proto generation to CI/CD

### Phase 3: Enhanced Features
- Add partial refunds via `external_reference_id` lookup
- Add transaction search by POS reference
- Add webhook notifications (optional)
- Add receipt JWT refresh endpoint (if 5min expiry too short)

---

## Files Changed

### Added
- ✅ `internal/db/migrations/008_pos_option2_refactoring.sql`
- ✅ `proto/payment/v1/payment_browserpost.proto`
- ✅ `internal/services/jwt_service.go`
- ✅ `internal/handlers/browserpost/callback_handler.go`
- ✅ `keys/jwt_private_key.pem`
- ✅ `keys/jwt_public_key.pem`

### Modified
- ✅ `internal/domain/transaction.go` - Added ExternalReferenceID, ReturnURL
- ✅ `.env.example` - Added JWT_PRIVATE_KEY_PATH, JWT_PUBLIC_KEY_PATH

### Not Changed (Future Work)
- ⏳ `proto/payment/v1/payment.proto` - Keep for backward compatibility
- ⏳ Handlers for cash payments - Defer removal
- ⏳ Database metadata column - Keep for backward compatibility

---

## Questions or Issues?

Contact: POS development team
Reference: `/home/kevinlam/Documents/projects/pos/docs/PAYMENT_SERVICE_REFACTORING.md`

---

**Status**: ✅ **REFACTORING COMPLETE - READY FOR TESTING**
