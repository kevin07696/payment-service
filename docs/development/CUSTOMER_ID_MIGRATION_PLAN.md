# Customer ID Migration: UUID → VARCHAR

**Current State:** customer_id is UUID in most tables
**Requested Change:** Change to VARCHAR/TEXT to support external service IDs
**Complexity:** Medium (2-4 hours work)
**Risk:** Medium (requires data migration)

## Current Schema Analysis

### Tables Using customer_id

| Table | Current Type | Nullable | Notes |
|-------|-------------|----------|-------|
| `customer_payment_methods` | UUID | NOT NULL | 4 indexes reference it |
| `transactions` | UUID | NULL | 2 indexes |
| `subscriptions` | UUID | NOT NULL | 1 index |
| `chargebacks` | **VARCHAR(100)** | NULL | Already VARCHAR! |

**Key Finding:** `chargebacks` table already uses `VARCHAR(100)` for customer_id!

### Code Impact

**Go Code:**
- 182 references to `CustomerID` across codebase
- 4 SQLC query files use customer_id
- Mix of types: `uuid.UUID`, `pgtype.UUID`, `pgtype.Text`

**API/Proto:**
- 21 references in proto files
- Currently defined as `string` in most proto definitions

**Good News:** Proto already uses `string`, so no API contract changes needed!

## Migration Strategy

### Option 1: VARCHAR(100) - RECOMMENDED ✅

**Pros:**
- Matches existing chargebacks table
- Sufficient for most external IDs (UUIDs, Stripe IDs, etc.)
- Indexed efficiently
- Prevents abuse (no 10KB strings)

**Cons:**
- Fixed length limit

**Recommended Type:** `VARCHAR(100)`

### Option 2: TEXT (Unlimited)

**Pros:**
- No length restrictions
- Maximum flexibility

**Cons:**
- Can't use certain index types efficiently
- Potential for abuse
- Inconsistent with chargebacks table

### Option 3: Keep UUID, Add customer_external_id

**Pros:**
- No breaking changes
- Keep UUID benefits (indexing, storage)
- Support both internal and external IDs

**Cons:**
- More complex schema
- Two customer identifiers to manage

## Implementation Plan (Option 1: VARCHAR)

### Phase 1: Database Migration (30 min)

Create migration `013_customer_id_to_varchar.sql`:

```sql
-- =====================================================
-- Migration: Convert customer_id from UUID to VARCHAR
-- =====================================================

BEGIN;

-- 1. Drop existing indexes
DROP INDEX IF EXISTS idx_customer_payment_methods_customer_id;
DROP INDEX IF EXISTS idx_customer_payment_methods_merchant_customer;
DROP INDEX IF EXISTS idx_customer_payment_methods_is_default;
DROP INDEX IF EXISTS idx_transactions_customer_id;
DROP INDEX IF EXISTS idx_transactions_merchant_customer;
DROP INDEX IF EXISTS idx_subscriptions_merchant_customer;

-- 2. Convert customer_payment_methods.customer_id
ALTER TABLE customer_payment_methods
  ALTER COLUMN customer_id TYPE VARCHAR(100) USING customer_id::TEXT;

-- 3. Convert transactions.customer_id
ALTER TABLE transactions
  ALTER COLUMN customer_id TYPE VARCHAR(100) USING customer_id::TEXT;

-- 4. Convert subscriptions.customer_id
ALTER TABLE subscriptions
  ALTER COLUMN customer_id TYPE VARCHAR(100) USING customer_id::TEXT;

-- 5. Recreate indexes
CREATE INDEX idx_customer_payment_methods_customer_id
  ON customer_payment_methods(customer_id);

CREATE INDEX idx_customer_payment_methods_merchant_customer
  ON customer_payment_methods(merchant_id, customer_id);

CREATE INDEX idx_customer_payment_methods_is_default
  ON customer_payment_methods(merchant_id, customer_id, is_default)
  WHERE is_default = true;

CREATE INDEX idx_transactions_customer_id
  ON transactions(customer_id) WHERE customer_id IS NOT NULL;

CREATE INDEX idx_transactions_merchant_customer
  ON transactions(merchant_id, customer_id) WHERE customer_id IS NOT NULL;

CREATE INDEX idx_subscriptions_merchant_customer
  ON subscriptions(merchant_id, customer_id);

COMMIT;
```

### Phase 2: Regenerate SQLC Models (5 min)

```bash
sqlc generate
```

This will update:
- `internal/db/sqlc/models.go` - CustomerID fields → `string` or `pgtype.Text`
- `internal/db/sqlc/*.sql.go` - All query functions

### Phase 3: Update Go Code (1-2 hours)

**Files to Update:**

1. **Domain Models** (`internal/domain/*.go`)
   - Change `CustomerID uuid.UUID` → `CustomerID string`
   - ~10 occurrences

2. **Service Layer** (`internal/services/**/*.go`)
   - Update method signatures
   - Remove UUID parsing logic
   - ~50 occurrences

3. **Handlers** (`internal/handlers/**/*.go`)
   - Update request/response handling
   - Remove UUID validation
   - ~30 occurrences

4. **Adapters** (`internal/adapters/**/*.go`)
   - Update EPX adapter customer ID handling
   - ~20 occurrences

5. **Tests** (`**/*_test.go`)
   - Update test fixtures
   - Replace `uuid.New()` with string IDs
   - ~70 occurrences

**Search & Replace Pattern:**
```bash
# Find UUID-based customer_id usage
grep -r "CustomerID.*uuid.UUID" internal/
grep -r "uuid.New()" internal/ | grep -i customer
```

### Phase 4: Update Tests (30 min)

Update test fixtures to use string customer IDs:

```go
// Before
customerID := uuid.New()

// After
customerID := "cust_ext_123456789"  // External service format
```

### Phase 5: Data Migration Script (30 min)

If production data exists:

```sql
-- Production migration converts existing UUIDs to strings
-- UUIDs remain valid as strings: "550e8400-e29b-41d4-a716-446655440000"
-- Or you can map to external IDs if you have the mapping
```

## Estimated Timeline

| Phase | Time | Complexity |
|-------|------|------------|
| Database Migration | 30 min | Low |
| SQLC Regeneration | 5 min | Low |
| Go Code Updates | 1-2 hours | Medium |
| Test Updates | 30 min | Low |
| Data Migration (if needed) | 30 min | Medium |
| Testing & QA | 1 hour | Medium |
| **TOTAL** | **3-4.5 hours** | **Medium** |

## Risks & Mitigation

### Risk 1: Data Loss During Migration
**Mitigation:**
- Run migration in transaction
- Test on staging first
- Backup database before migration
- Use `USING customer_id::TEXT` to preserve existing UUID values

### Risk 2: Breaking API Compatibility
**Mitigation:**
- Proto already uses `string` type
- No API contract changes needed
- Existing UUID strings remain valid

### Risk 3: Test Failures
**Mitigation:**
- Update all test fixtures systematically
- Run full test suite after each phase
- Fix compilation errors before moving to next phase

## Rollback Plan

If issues arise:

```sql
-- Rollback migration
ALTER TABLE customer_payment_methods
  ALTER COLUMN customer_id TYPE UUID USING customer_id::UUID;

ALTER TABLE transactions
  ALTER COLUMN customer_id TYPE UUID USING customer_id::UUID;

ALTER TABLE subscriptions
  ALTER COLUMN customer_id TYPE UUID USING customer_id::UUID;
```

**Note:** Only possible if all customer_ids are valid UUIDs!

## Alternative: Minimal Change Approach

If you want to minimize risk, just update validation:

1. Keep UUID type in database
2. Accept string from API
3. Generate deterministic UUID from external ID:

```go
func externalIDToUUID(externalID string) uuid.UUID {
    // Generate deterministic UUID from external ID
    hash := sha256.Sum256([]byte(externalID))
    return uuid.FromBytesOrNil(hash[:16])
}
```

**Pros:** No schema changes, minimal code changes
**Cons:** Requires mapping table, more complex

## Recommendation

**Use Option 1: VARCHAR(100) Migration**

**Reasoning:**
- Clean solution that matches business need
- chargebacks table already uses VARCHAR(100)
- Proto already uses string (no API changes)
- Straightforward migration path
- 3-4 hours of focused work

**Next Steps:**
1. Create feature branch: `feature/customer-id-varchar`
2. Implement Phase 1 (database migration)
3. Regenerate SQLC
4. Fix compilation errors systematically
5. Update tests
6. Run full test suite
7. Test on staging environment
8. Merge when verified

**Total Effort:** Half day of focused work + testing
