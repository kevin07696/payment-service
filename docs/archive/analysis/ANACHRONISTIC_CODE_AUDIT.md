# Anachronistic Code Audit Report

**Date**: 2025-11-23
**Purpose**: Identify outdated code, deprecated patterns, and inconsistencies between documentation and implementation

---

## Executive Summary

This audit identified several categories of anachronistic code that should be cleaned up:

1. **Deprecated Functions** - 1 deprecated function still in codebase
2. **Incomplete TODOs** - 21 TODO/FIXME comments requiring attention
3. **Documentation Discrepancies** - Functions documented but not implemented
4. **Skipped Tests** - 22 integration tests with t.Skip() markers
5. **Inconsistent Naming** - Mix of old and new terminology

---

## 1. Deprecated Functions (Action Required)

### 1.1 TokenizeAndSaveCard - DEPRECATED

**File**: `tests/integration/testutil/tokenization.go:453-456`

```go
// TokenizeAndSaveCard is deprecated - use TokenizeAndSaveCardViaBrowserPost instead
func TokenizeAndSaveCard(...) (string, error) {
    return "", fmt.Errorf("TokenizeAndSaveCard deprecated - use TokenizeAndSaveCardViaBrowserPost instead")
}
```

**Status**: ❌ Function exists but only returns an error
**Action**: Remove entirely from codebase
**Impact**: Low (returns error, so already unusable)

---

## 2. Documentation vs Implementation Discrepancies

### 2.1 ConvertFinancialBRICToStorageBRIC - NOT IMPLEMENTED

**Mentioned In**:
- `CHANGELOG.md` (6 references)
- `docs/refactor/TDD_REFACTOR_PLAN.md`
- `docs/refactor/INTEGRATION_TEST_PLAN.md`
- `docs/development/CREDIT_CARD_BUSINESS_LOGIC.md`

**Status**: ❌ Function never implemented or was removed during refactoring
**Action**:
- Update CHANGELOG to reflect removal
- Remove from planning documents or mark as "Removed during refactoring"
- Verify `docs/refactor/REFACTOR_PLAN.md` lines 109 & 177 which mention removal

**Note**: The **adapter-level** function `ConvertFinancialBRICToStorage` DOES exist in:
- `internal/adapters/ports/bric_storage.go:103`
- `internal/adapters/epx/bric_storage_adapter.go:103`

This is different from the **service-level** RPC that was planned but not implemented.

---

## 3. Incomplete TODO Comments (21 Total)

### 3.1 High Priority - Authentication & Security

**File**: `internal/middleware/connect_auth.go`

```go
// Line 345: TODO: Implement circuit breaker pattern or in-memory fallback for better availability
// Line 362: TODO: Implement database audit_log table and replace with DB logging
```

**Priority**: 🔴 High
**Reason**: Authentication resilience and audit logging are security-critical

### 3.2 Medium Priority - Admin Handler Metadata

**File**: `internal/handlers/admin/service_handler.go` (6 instances)

```go
// Line 260: TODO: Get actual count from DB
// Lines 387-388, 443-444, 498-499: TODO: Extract from HTTP request (requires HTTP interceptor)
//   - IpAddress extraction
//   - UserAgent extraction
```

**Priority**: 🟡 Medium
**Reason**: Audit trail completeness for admin operations

### 3.3 Medium Priority - Chargeback Linking

**File**: `internal/handlers/cron/dispute_sync_handler.go:320`

```go
// TODO: Link to payment group by finding the transaction and its group
```

**Priority**: 🟡 Medium
**Reason**: Affects chargeback-to-payment reconciliation
**Context**: Recent change from `transaction_id` to `group_id` in chargebacks table

### 3.4 Low Priority - WordPress Admin UI Tests

**File**: `tests/integration/wordpress/admin_operations_test.go` (10 instances)

```go
// Lines 424-444: TODO: Implement UI automation for:
//   - Bulk refund
//   - Partial capture
//   - Partial refund
//   - Full refund
//   - Void
// Lines 477-497: TODO: Implement verification (5 instances)
```

**Priority**: 🟢 Low
**Reason**: Test automation improvements, not blocking functionality

### 3.5 Low Priority - Test Implementation

**File**: `tests/integration/payment/tac_replay_protection_test.go:142`

```go
t.Skip("TODO: Implement concurrent callback test - requires mocking EPX or controlling callback timing")
```

**Priority**: 🟢 Low
**Reason**: Advanced test scenario

**File**: `tests/auth_test.go:66`

```go
t.Skip("TODO: Add service authentication tests using RSA keypairs")
```

**Priority**: 🟢 Low
**Reason**: Additional test coverage

---

## 4. Skipped Integration Tests (22 Total)

### 4.1 Authentication Disabled Tests (7 tests)

**File**: `tests/integration/auth/cron_auth_test.go`

All tests skip with: `"Requires authentication enabled - will be enabled in task #6"`

**Tests**:
- `TestCronEndpointRequiresAuth`
- `TestCronEndpointWithValidSignature`
- `TestCronEndpointWithInvalidSignature`
- `TestCronEndpointWithExpiredTimestamp`
- `TestCronEndpointWithMissingHeaders`
- `TestCronEndpointWithTamperedPayload`
- `TestHealthEndpoint` (line 170)

**Action**:
- If authentication is now enabled, remove t.Skip() and enable tests
- If still planned, update task reference

### 4.2 Database Integration Tests (11 tests)

**File**: `internal/adapters/database/postgres_test.go`

All skip when `TEST_DATABASE_URL` not set

**Action**: These are properly conditional - no changes needed

### 4.3 Other Conditional Skips

- **ngrok tests**: Skip when ngrok unavailable (proper)
- **Short mode tests**: Skip in short mode (proper)
- **Data-dependent tests**: Skip when no data available (proper)
- **Merchant gRPC test**: Documented skip (proper)

---

## 5. Inconsistent Terminology

### 5.1 Logging Variable Names

**File**: `internal/services/payment/payment_service.go:521`

```go
zap.String("original_transaction_id", req.TransactionID)
```

**Issue**: Uses `original_transaction_id` in logs but system uses `parent_transaction_id` in database

**Action**: Standardize logging field names to match database schema

### 5.2 SQL-to-Domain Conversion Functions

**Pattern Found**: Multiple naming conventions for conversion functions

```go
// Subscription
sqlcSubscriptionToDomain()

// Payment Method
sqlcPaymentMethodToDomain()

// Payment Transaction
sqlcToDomain()  // ⚠️ Less descriptive

// Merchant
sqlcMerchantToDomain()
```

**Issue**: `sqlcToDomain()` in payment_service.go is less descriptive than others

**Action**: Consider renaming to `sqlcTransactionToDomain()` for consistency

---

## 6. Proto-to-Domain Mapping Functions

**Pattern Found**: Inconsistent naming

```go
protoStatusToDomain()        // payment_handler.go:436
mapProtoStatusToDomain()     // chargeback_handler_connect.go:290
```

**Action**: Standardize to one convention (recommend `mapProtoStatusToDomain` prefix for all)

---

## 7. Archived Documentation

**Location**: `/home/kevinlam/Documents/projects/payments/docs/archive/`

**Contents**:
- `analysis/` - 8+ files
- `planning/` - 8+ files
- `reports/`
- `research/`
- `reviews/`

**Includes**:
- `E2E_TEST_DESIGN.md` - References `ConvertFinancialBRICToStorageBRIC`
- Planning docs with outdated RPC designs

**Action**:
- Add README.md to `/docs/archive/` explaining these are historical documents
- Consider moving truly obsolete docs to a separate `/docs/archive/obsolete/` folder

---

## 8. Legacy Code Markers

### 8.1 Legacy Browser Support

**File**: `internal/middleware/security_headers.go:34`

```go
// Note: Modern browsers use CSP instead, but this helps legacy browsers
```

**Status**: ✅ Properly documented, intentional support

### 8.2 Legacy Error Definitions

**File**: `internal/domain/errors.go:199`

```go
// Common domain errors (legacy - kept for backward compatibility)
```

**Action**: Review if these errors are still used; remove if not

---

## Action Items Summary

### Immediate Actions (Do Now)

1. ✅ **Remove deprecated function**
   - Delete `TokenizeAndSaveCard` from `tests/integration/testutil/tokenization.go`

2. ✅ **Update documentation**
   - Add note to CHANGELOG about `ConvertFinancialBRICToStorageBRIC` being removed/never implemented
   - Update `docs/development/CREDIT_CARD_BUSINESS_LOGIC.md` to remove references

3. ✅ **Fix logging inconsistency**
   - Rename `original_transaction_id` to `parent_transaction_id` in logs

### High Priority (This Sprint)

4. 🔴 **Implement authentication improvements**
   - `connect_auth.go:345` - Circuit breaker pattern
   - `connect_auth.go:362` - Database audit logging

5. 🔴 **Enable or remove auth tests**
   - Review `tests/integration/auth/cron_auth_test.go` (7 tests)
   - Enable if auth is ready, or update task reference

6. 🟡 **Complete chargeback linking**
   - `dispute_sync_handler.go:320` - Link to payment group_id

### Medium Priority (Next Sprint)

7. 🟡 **Standardize function naming**
   - Rename `sqlcToDomain()` → `sqlcTransactionToDomain()`
   - Standardize `protoStatusToDomain()` vs `mapProtoStatusToDomain()`

8. 🟡 **Complete admin handler metadata extraction**
   - IP address extraction (3 locations)
   - User agent extraction (3 locations)

### Low Priority (Backlog)

9. 🟢 **WordPress admin test automation** (10 TODOs)
10. 🟢 **Advanced test scenarios** (2 TODOs)
11. 🟢 **Review legacy error definitions**

---

## Metrics

- **Deprecated Functions**: 1
- **Documentation Discrepancies**: 1 major (ConvertFinancialBRICToStorageBRIC)
- **TODO Comments**: 21
- **Skipped Tests**: 22 (7 action-required, 15 properly conditional)
- **Naming Inconsistencies**: 3 categories
- **Estimated Cleanup Effort**: 2-3 days

---

## Recommendations

1. **Code Cleanliness**: Remove deprecated `TokenizeAndSaveCard` function immediately
2. **Documentation Accuracy**: Update docs to match actual implementation
3. **Naming Standards**: Create coding standards doc for conversion function naming
4. **Test Coverage**: Enable skipped auth tests once authentication is production-ready
5. **Audit Archive**: Add README to `/docs/archive/` to prevent confusion

---

**Report Generated By**: Claude Code Anachronistic Code Audit
**Next Review**: After completing P2 optimization tasks
