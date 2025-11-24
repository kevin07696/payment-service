# Domain Refactoring Plan: Gateway-Agnostic Architecture

**Date:** 2025-11-23
**Status:** 📋 Planned (Not Started)
**Priority:** Medium (Post Style Guide)

---

## Goal

Refactor domain layer to be **gateway-agnostic** - remove EPX-specific concepts from domain entities and move them to the EPX adapter layer.

---

## Current State (Option A: Mixed - Domain has EPX leakage)

**Problems:**
- ❌ Domain knows about EPX (`AuthGUID`, `AuthResp`, `AuthCode`, `AuthAVS`, `AuthCVV2`)
- ❌ Can't swap payment gateways without changing domain
- ❌ Business logic tied to EPX response codes (`"00"` for approval)
- ❌ Database schema tied to EPX field names

**Example of EPX leakage in domain:**
```go
// domain/transaction.go
type Transaction struct {
    AuthGUID     string   // EPX-specific
    AuthResp     *string  // EPX response code "00"
    AuthCode     *string  // EPX auth code
    AuthAVS      *string  // EPX AVS response
    AuthCVV2     *string  // EPX CVV2 response
}

func (t *Transaction) IsApproved() bool {
    return t.AuthResp != nil && *t.AuthResp == "00"  // EPX-specific logic!
}
```

---

## Target State (Option B: Pure Domain)

**Benefits:**
- ✅ **Pure domain** - no gateway coupling
- ✅ **Swappable gateways** - add Stripe/Adyen without changing domain
- ✅ **Business logic isolated** - `IsApproved()` works with any gateway
- ✅ **Testable** - mock gateway responses easily
- ✅ **Database agnostic** - schema not tied to EPX

**Architecture:**

```
┌─────────────────────────────────────┐
│ Domain Layer (Pure Business Logic)  │
│ - Transaction (NO EPX concepts)     │
│ - GatewayTransactionID (generic)    │
│ - Status: approved/declined         │
│ - Business rules (CanBeVoided, etc.)│
└─────────────────────────────────────┘
           ↓
┌─────────────────────────────────────┐
│ Service Layer (Gateway-Agnostic)    │
│ - Uses domain types only            │
│ - Calls adapter port interfaces     │
└─────────────────────────────────────┘
           ↓
┌─────────────────────────────────────┐
│ EPX Adapter (Implementation)        │
│ - Maps EPX → domain                 │
│ - Stores EPX data in metadata       │
└─────────────────────────────────────┘
```

**Example of pure domain:**
```go
// domain/transaction.go
type Transaction struct {
    ID                   string
    MerchantID           string
    AmountCents          int64
    Status               TransactionStatus  // approved/declined (generic)
    Type                 TransactionType    // sale/auth/capture (generic)

    // Gateway-agnostic fields
    GatewayTransactionID string              // Could be EPX GUID, Stripe ID, etc.
    GatewayMetadata      map[string]string   // Gateway-specific data as key-value
}

func (t *Transaction) IsApproved() bool {
    return t.Status == TransactionStatusApproved  // Gateway-agnostic!
}
```

---

## Implementation Plan

### Phase 1: Add Gateway-Agnostic Fields (Non-Breaking)

**Step 1.1: Update Domain Types**

```go
// domain/transaction.go
type Transaction struct {
    // ... existing fields ...

    // NEW: Gateway-agnostic fields (add alongside EPX fields)
    GatewayTransactionID string            // Generic transaction ID
    GatewayMetadata      map[string]string // Gateway-specific data

    // KEEP (for now): EPX-specific fields for backwards compatibility
    AuthGUID     string
    AuthResp     *string
    AuthCode     *string
    AuthAVS      *string
    AuthCVV2     *string
}

// NEW: Gateway-agnostic business logic
func (t *Transaction) IsApprovedGeneric() bool {
    return t.Status == TransactionStatusApproved
}

// KEEP (deprecated): EPX-specific logic for backwards compatibility
func (t *Transaction) IsApproved() bool {
    return t.AuthResp != nil && *t.AuthResp == "00"
}
```

**Step 1.2: Database Migration**

```sql
-- Migration: Add gateway-agnostic columns
ALTER TABLE transactions
    ADD COLUMN gateway_transaction_id VARCHAR(255),
    ADD COLUMN gateway_metadata JSONB,
    ADD COLUMN status VARCHAR(20);  -- approved/declined/pending

-- Add index for common queries
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_gateway_tx_id ON transactions(gateway_transaction_id);
```

**Step 1.3: Update EPX Adapter to Populate Both**

```go
// adapters/epx/converter.go
func (a *epxAdapter) toDomainTransaction(epxResp *ServerPostResponse, req *ServerPostRequest) *domain.Transaction {
    status := a.mapStatus(epxResp.AuthResp)

    return &domain.Transaction{
        // ... other fields ...

        // NEW: Populate gateway-agnostic fields
        GatewayTransactionID: epxResp.AuthGUID,
        Status:               status,
        GatewayMetadata: map[string]string{
            "gateway":            "epx",
            "epx_auth_resp":      epxResp.AuthResp,
            "epx_auth_code":      epxResp.AuthCode,
            "epx_auth_resp_text": epxResp.AuthRespText,
            "epx_auth_avs":       epxResp.AuthAVS,
            "epx_auth_cvv2":      epxResp.AuthCVV2,
            "epx_card_type":      epxResp.AuthCardType,
        },

        // KEEP: Also populate old EPX fields (backwards compatibility)
        AuthGUID: epxResp.AuthGUID,
        AuthResp: &epxResp.AuthResp,
        AuthCode: &epxResp.AuthCode,
        AuthAVS:  &epxResp.AuthAVS,
        AuthCVV2: &epxResp.AuthCVV2,
    }
}

func (a *epxAdapter) mapStatus(authResp string) domain.TransactionStatus {
    switch authResp {
    case "00", "85":
        return domain.TransactionStatusApproved
    default:
        return domain.TransactionStatusDeclined
    }
}
```

---

### Phase 2: Migrate Existing Data

**Step 2.1: Data Migration Script**

```sql
-- Migrate existing transactions to new fields
UPDATE transactions SET
    gateway_transaction_id = auth_guid,
    status = CASE
        WHEN auth_resp IN ('00', '85') THEN 'approved'
        ELSE 'declined'
    END,
    gateway_metadata = jsonb_build_object(
        'gateway', 'epx',
        'epx_auth_resp', auth_resp,
        'epx_auth_code', auth_code,
        'epx_auth_resp_text', auth_resp_text,
        'epx_auth_avs', auth_avs,
        'epx_auth_cvv2', auth_cvv2,
        'epx_card_type', auth_card_type
    )
WHERE gateway_transaction_id IS NULL;

-- Verify migration
SELECT
    COUNT(*) as total,
    COUNT(gateway_transaction_id) as migrated,
    COUNT(*) - COUNT(gateway_transaction_id) as remaining
FROM transactions;
```

**Step 2.2: Add NOT NULL Constraints**

```sql
-- After migration completes and is verified
ALTER TABLE transactions
    ALTER COLUMN gateway_transaction_id SET NOT NULL,
    ALTER COLUMN status SET NOT NULL,
    ALTER COLUMN gateway_metadata SET NOT NULL,
    ALTER COLUMN gateway_metadata SET DEFAULT '{}';
```

---

### Phase 3: Update Code to Use New Fields

**Step 3.1: Update Services**

```go
// Before (EPX-coupled)
if tx.AuthResp != nil && *tx.AuthResp == "00" {
    // approved
}

// After (gateway-agnostic)
if tx.IsApproved() {  // Uses tx.Status == "approved"
    // approved
}
```

**Step 3.2: Update Business Logic**

```go
// domain/transaction.go

// Remove EPX-specific logic
func (t *Transaction) IsApproved() bool {
    return t.Status == TransactionStatusApproved  // Gateway-agnostic
}

func (t *Transaction) CanBeVoided() bool {
    return t.Status == TransactionStatusApproved &&
        (t.Type == TransactionTypeAuth || t.Type == TransactionTypeSale)
}

// Helper methods for EPX-specific data (when needed)
func (t *Transaction) EPXAuthResp() string {
    if t.GatewayMetadata == nil {
        return ""
    }
    return t.GatewayMetadata["epx_auth_resp"]
}

func (t *Transaction) EPXAuthCode() string {
    if t.GatewayMetadata == nil {
        return ""
    }
    return t.GatewayMetadata["epx_auth_code"]
}
```

**Step 3.3: Update Tests**

```go
// Before (EPX-specific)
func TestTransaction_CanBeRefunded(t *testing.T) {
    authResp := "00"
    tx := &domain.Transaction{
        AuthResp: &authResp,
        Type:     domain.TransactionTypeSale,
    }
    assert.True(t, tx.CanBeRefunded())
}

// After (gateway-agnostic)
func TestTransaction_CanBeRefunded(t *testing.T) {
    tx := &domain.Transaction{
        Status: domain.TransactionStatusApproved,
        Type:   domain.TransactionTypeSale,
    }
    assert.True(t, tx.CanBeRefunded())
}
```

---

### Phase 4: Remove EPX-Specific Fields (Breaking Change)

**Step 4.1: Deprecation Period**

Add deprecation notices:
```go
// domain/transaction.go

// Deprecated: Use GatewayTransactionID instead
// Will be removed in v2.0
AuthGUID string

// Deprecated: Use Status == TransactionStatusApproved instead
// Will be removed in v2.0
AuthResp *string
```

**Step 4.2: Remove Old Fields**

After deprecation period (e.g., 3 months):

```sql
-- Drop old EPX-specific columns
ALTER TABLE transactions
    DROP COLUMN auth_guid,
    DROP COLUMN auth_resp,
    DROP COLUMN auth_code,
    DROP COLUMN auth_avs,
    DROP COLUMN auth_cvv2,
    DROP COLUMN auth_card_type,
    DROP COLUMN auth_resp_text;
```

```go
// domain/transaction.go
type Transaction struct {
    ID                   string
    MerchantID           string
    AmountCents          int64
    Status               TransactionStatus
    Type                 TransactionType
    GatewayTransactionID string
    GatewayMetadata      map[string]string
    // ... other fields ...
}
```

---

## Benefits After Refactoring

### 1. Multi-Gateway Support (Easy to Add Stripe)

```go
// adapters/stripe/stripe_adapter.go
func (a *stripeAdapter) toDomainTransaction(stripeCharge *stripe.Charge) *domain.Transaction {
    return &domain.Transaction{
        GatewayTransactionID: stripeCharge.ID,
        Status:               a.mapStatus(stripeCharge.Status),
        GatewayMetadata: map[string]string{
            "gateway":            "stripe",
            "stripe_charge_id":   stripeCharge.ID,
            "stripe_status":      string(stripeCharge.Status),
            "stripe_receipt_url": stripeCharge.ReceiptURL,
        },
    }
}

func (a *stripeAdapter) mapStatus(status stripe.ChargeStatus) domain.TransactionStatus {
    switch status {
    case stripe.ChargeStatusSucceeded:
        return domain.TransactionStatusApproved
    case stripe.ChargeStatusFailed:
        return domain.TransactionStatusDeclined
    default:
        return domain.TransactionStatusPending
    }
}
```

### 2. Clean Database Schema

```sql
-- Query all approved transactions (regardless of gateway)
SELECT * FROM transactions WHERE status = 'approved';

-- Query EPX-specific details when needed
SELECT
    id,
    gateway_metadata->>'epx_auth_resp' as epx_auth_code,
    gateway_metadata->>'epx_avs' as avs_result
FROM transactions
WHERE gateway_metadata->>'gateway' = 'epx';
```

### 3. Pure Business Logic

```go
// Business rules work with any gateway
func (s *subscriptionService) ProcessRecurringBilling(ctx context.Context) error {
    for _, subscription := range subscriptions {
        tx, err := s.paymentService.Charge(subscription)

        // Gateway-agnostic logic
        if tx.IsApproved() {
            s.markSubscriptionPaid(subscription)
        } else {
            s.handleFailedPayment(subscription)
        }
    }
}
```

---

## Testing Strategy

### Unit Tests
- Test domain logic without EPX knowledge
- Test adapter conversions (EPX ↔ domain)

### Integration Tests
- Verify both old and new fields populated during transition
- Test data migration script on copy of production data
- Verify backwards compatibility during Phase 1-3

### Acceptance Criteria
- ✅ All existing functionality works with new fields
- ✅ Can query transactions by status (gateway-agnostic)
- ✅ EPX-specific details still accessible via metadata
- ✅ Zero downtime migration
- ✅ All tests pass

---

## Rollback Plan

### During Phase 1-3 (Non-Breaking)
- Both old and new fields exist
- Can simply stop using new fields
- No data loss

### After Phase 4 (Breaking)
- Cannot rollback without restoring EPX columns
- Must keep backup before dropping columns
- Rollback script:
  ```sql
  ALTER TABLE transactions
      ADD COLUMN auth_guid VARCHAR(255),
      ADD COLUMN auth_resp VARCHAR(10);

  UPDATE transactions SET
      auth_guid = gateway_transaction_id,
      auth_resp = gateway_metadata->>'epx_auth_resp'
  WHERE gateway_metadata->>'gateway' = 'epx';
  ```

---

## Related Documentation

- [Style Guide](STYLE_GUIDE.md) - Will document pure domain principles
- [Domain-Driven Design Principles](https://martinfowler.com/bliki/DomainDrivenDesign.html)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)

---

**Next Steps:**
1. ✅ Document in refactor plan (this file)
2. ⏳ Complete Style Guide first
3. ⏳ Get approval for refactoring approach
4. ⏳ Start Phase 1 implementation

