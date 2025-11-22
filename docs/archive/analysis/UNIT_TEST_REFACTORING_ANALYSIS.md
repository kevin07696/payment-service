# Unit Test Refactoring Analysis

**Generated**: 2025-11-19
**Scope**: Analysis of unit test code quality, patterns, duplication, and gaps
**Context**: Based on `docs/API_DESIGN_AND_DATAFLOW.md` and `docs/AUTHENTICATION.md`
**Status**: Analysis Complete - Recommendations Ready

---

## Executive Summary

The payment service has **good unit test coverage for pure functions** (state computation, validation) but suffers from:

1. **Massive mock duplication** (~300 lines of duplicated mock code across files)
2. **Missing unit tests** for components we plan to extract (credential resolver, token resolver)
3. **Insufficient callback handler testing** (750 lines but 200 are mock stubs!)
4. **Helper function proliferation** (ptr, strPtr, makeTransaction duplicated everywhere)
5. **No shared test utilities** package

**Overall Unit Test Health**: ⚠️ **Needs Improvement** (6/10)
- ✅ Excellent: `group_state_test.go`, `validation_test.go` (table-driven, pure functions)
- ✅ Good: `merchant_resolver_test.go` (449 lines - comprehensive auth testing)
- ⚠️ Good: `server_post_adapter_test.go`, `browser_post_callback_handler_test.go`
- ❌ Poor: Mock implementations, test helpers (duplicated, disorganized)

---

## Context: System Architecture

### Payment Flow (from API_DESIGN_AND_DATAFLOW.md)

```
Customer → EPX Browser Post → EPX Gateway → Callback → Payment Service
                                              ↓
                                    BRIC Token (Storage or Financial)
                                              ↓
                                    Transaction Types:
                                    - CCE8: Storage BRIC (save card)
                                    - CCE2: Sale (immediate payment)
                                    - CCE1: Auth (hold funds)
```

### Authentication (from AUTHENTICATION.md)

```
5 Token Types:
- Service Token (RSA-signed, 15 min) → Apps/Services
- Admin Token (HMAC-signed, 2 hours) → Admins
- Merchant Portal Token (HMAC-signed, 2 hours) → Merchants
- Customer Token (HMAC-signed, 30 min) → Customers
- Guest Token (HMAC-signed, 5 min) → Anonymous
```

**Current Test Coverage**:
- ✅ Merchant resolver (`merchant_resolver_test.go` - 449 lines)
- ❌ Service token verification (missing)
- ❌ Token type routing (missing)
- ❌ Scope validation (partial - only in merchant_resolver_test.go)

---

## Test File Inventory

### Unit Test Files (by line count)

```
750 lines - browser_post_callback_handler_test.go ⚠️ (200 lines are mock stubs!)
477 lines - group_state_test.go ✅ (Excellent - table-driven, pure functions)
468 lines - server_post_adapter_test.go ✅ (Good - adapter unit tests)
465 lines - validation_test.go ✅ (Excellent - comprehensive validation tests)
449 lines - merchant_resolver_test.go ✅ (Good - comprehensive auth tests)
404 lines - server_post_error_test.go
340 lines - public_key_store_test.go
333 lines - postgres_test.go
289 lines - payment_service_test.go ⚠️ (Only tests helper functions!)
```

**Total Unit Test LOC**: ~4,175 lines
**Duplicated Mock/Helper LOC**: ~300 lines (7% waste)

---

## Problem 1: Mock Implementation Duplication (HIGH PRIORITY)

### Current State: Mocks Duplicated Across Files

**File: `browser_post_callback_handler_test.go` (lines 39-200)**

This file contains a **MockQuerier** with **60+ stub methods** (200+ lines of boilerplate):

```go
type MockQuerier struct {
    mock.Mock
}

// 60+ methods like this:
func (m *MockQuerier) GetMerchantByID(ctx context.Context, id uuid.UUID) (sqlc.Merchant, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(sqlc.Merchant), args.Error(1)
}

// ... 59 more stub methods that just return nil/empty structs
func (m *MockQuerier) GetTransactionByID(...) { return sqlc.Transaction{}, nil }
func (m *MockQuerier) CreatePaymentMethod(...) { return sqlc.CustomerPaymentMethod{}, nil }
func (m *MockQuerier) ListSubscriptions(...) { return nil, nil }
// ... and so on
```

**File: `payment_service_test.go` (lines 23-86)**

```go
// MockServerPostAdapter (duplicated)
type MockServerPostAdapter struct {
    mock.Mock
}
// ... 4 methods

// MockSecretManagerAdapter (duplicated)
type MockSecretManagerAdapter struct {
    mock.Mock
}
// ... 6 methods
```

### Impact

- **~300 lines of duplicated mock code**
- Changes to interfaces require updating mocks in multiple places
- No consistency in mock behavior
- Violates DRY principle

### Solution: Create Shared Mock Package

**Create**: `internal/testutil/mocks/` package

```
internal/testutil/mocks/
├── database.go         # MockDatabaseAdapter, MockQuerier
├── server_post.go      # MockServerPostAdapter
├── secret_manager.go   # MockSecretManagerAdapter
└── README.md           # Usage guide
```

**Example: `internal/testutil/mocks/database.go`**

```go
package mocks

import (
    "context"
    "github.com/google/uuid"
    "github.com/kevin07696/payment-service/internal/db/sqlc"
    "github.com/stretchr/testify/mock"
)

// MockQuerier provides a full mock implementation of sqlc.Querier
// Only methods used in tests need full implementation.
// Unused methods return sensible zero values.
type MockQuerier struct {
    mock.Mock
}

// Frequently used methods - full mock implementation
func (m *MockQuerier) GetMerchantByID(ctx context.Context, id uuid.UUID) (sqlc.Merchant, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(sqlc.Merchant), args.Error(1)
}

func (m *MockQuerier) CreateTransaction(ctx context.Context, params sqlc.CreateTransactionParams) (sqlc.Transaction, error) {
    args := m.Called(ctx, params)
    return args.Get(0).(sqlc.Transaction), args.Error(1)
}

// ... other frequently used methods

// Stub methods - return zero values (auto-generated)
// Only add mocks for methods actually used in tests
```

**Usage in tests**:

```go
import "github.com/kevin07696/payment-service/internal/testutil/mocks"

func TestMyHandler(t *testing.T) {
    mockDB := &mocks.MockQuerier{}
    mockDB.On("GetMerchantByID", mock.Anything, merchantID).
        Return(merchant, nil)

    // Use mockDB in test...
}
```

**Benefits**:
- Single source of truth for mocks
- Easier to maintain
- Consistent across all tests
- Reduces ~300 lines of duplication

**Effort**: 3-4 hours
**Risk**: Low (pure extraction, update imports)

---

## Problem 2: Test Helper Duplication (MEDIUM PRIORITY)

### Current State: Helpers Scattered Across Files

**Pointer helpers duplicated in 3+ files**:

```go
// payment_service_test.go:102
func ptr(s string) *string { return &s }

// server_post_adapter_test.go:21
func strPtr(s string) *string { return &s }

// group_state_test.go:475
func stringPtr(s string) *string { return &s }
```

**Transaction builders duplicated**:

```go
// group_state_test.go:13
func makeTransaction(id string, txType domain.TransactionType, amount string, authGUID string) *domain.Transaction

// validation_test.go:13
func makeTransaction(...) // Same implementation

// validation_test.go:27
func makeDeclinedTransaction(...)

// validation_test.go:461
func makeVoidTransactionWithType(...)
```

### Solution: Create Shared Fixtures Package

**Create**: `internal/testutil/fixtures/` package

```
internal/testutil/fixtures/
├── pointers.go         # StringPtr, IntPtr, BoolPtr
├── transactions.go     # TransactionBuilder
├── merchants.go        # MerchantBuilder
├── payment_methods.go  # PaymentMethodBuilder
└── README.md           # Usage guide
```

**Example: `internal/testutil/fixtures/transactions.go`**

```go
package fixtures

import (
    "github.com/kevin07696/payment-service/internal/domain"
    "github.com/shopspring/decimal"
)

// TransactionBuilder provides fluent API for building test transactions
type TransactionBuilder struct {
    tx *domain.Transaction
}

func NewTransaction() *TransactionBuilder {
    return &TransactionBuilder{
        tx: &domain.Transaction{
            Metadata: make(map[string]interface{}),
        },
    }
}

func (b *TransactionBuilder) WithID(id string) *TransactionBuilder {
    b.tx.ID = id
    return b
}

func (b *TransactionBuilder) WithType(txType domain.TransactionType) *TransactionBuilder {
    b.tx.Type = txType
    return b
}

func (b *TransactionBuilder) WithAmount(amount string) *TransactionBuilder {
    amt, _ := decimal.NewFromString(amount)
    b.tx.Amount = amt
    return b
}

func (b *TransactionBuilder) WithAuthGUID(guid string) *TransactionBuilder {
    b.tx.AuthGUID = guid
    return b
}

func (b *TransactionBuilder) Approved() *TransactionBuilder {
    b.tx.Status = domain.TransactionStatusApproved
    return b
}

func (b *TransactionBuilder) Declined() *TransactionBuilder {
    b.tx.Status = domain.TransactionStatusDeclined
    return b
}

func (b *TransactionBuilder) Build() *domain.Transaction {
    return b.tx
}

// Backward-compatible convenience functions
func ApprovedTransaction(id string, txType domain.TransactionType, amount string, authGUID string) *domain.Transaction {
    return NewTransaction().
        WithID(id).
        WithType(txType).
        WithAmount(amount).
        WithAuthGUID(authGUID).
        Approved().
        Build()
}
```

**Usage**:

```go
// Clean, expressive test setup
tx := fixtures.NewTransaction().
    WithID("auth1").
    WithType(domain.TransactionTypeAuth).
    WithAmount("100.00").
    WithAuthGUID("bric_auth1").
    Approved().
    Build()
```

**Effort**: 2-3 hours
**Risk**: Low

---

## Problem 3: Missing Unit Tests for Extracted Components (HIGH PRIORITY)

### Component 1: Merchant Credential Resolver (TO BE EXTRACTED)

**Current**: Logic embedded in `payment_service.go` (repeated 8+ times)
**Target**: Extract to `internal/services/merchant/credential_resolver.go`

**Tests Needed**: `merchant_credential_resolver_test.go`

```go
func TestCredentialResolver_Resolve_Success(t *testing.T) {
    // Test: Active merchant + valid secret = success
}

func TestCredentialResolver_Resolve_InactiveMerchant(t *testing.T) {
    // Test: IsActive = false → domain.ErrMerchantInactive
}

func TestCredentialResolver_Resolve_MerchantNotFound(t *testing.T) {
    // Test: GetMerchantByID error → wrapped error
}

func TestCredentialResolver_Resolve_SecretFetchError(t *testing.T) {
    // Test: Secret manager error → wrapped error
}
```

**Test Cases**:
1. ✅ Success: Active merchant with valid MAC secret
2. ❌ Merchant not found in database
3. ❌ Merchant inactive (`IsActive = false`)
4. ❌ Secret fetch error (secret manager unavailable)

**Effort**: 1-2 hours (150 lines)

---

### Component 2: Payment Token Resolver (TO BE EXTRACTED)

**Current**: Logic duplicated in Sale, Authorize, Refund methods
**Target**: Extract to `resolvePaymentToken()` helper or separate package

**Context from API_DESIGN_AND_DATAFLOW.md**:
- **Storage BRIC (CCE8)**: Save card, no charge → saved in `payment_methods` table
- **Financial BRIC (CCE2/CCE1)**: Used for transactions → ephemeral or from storage

**Tests Needed**: `payment_token_resolver_test.go`

```go
func TestResolvePaymentToken_FromPaymentMethodID(t *testing.T) {
    // Test: payment_method_id → query DB → return Storage BRIC
    // Expected: tokenInfo.Token = pm.PaymentToken
    //          tokenInfo.PaymentMethodID = &pmID
}

func TestResolvePaymentToken_FromDirectToken(t *testing.T) {
    // Test: payment_token (Financial BRIC) → use directly
    // Expected: tokenInfo.Token = directToken
    //          tokenInfo.PaymentMethodID = nil
}

func TestResolvePaymentToken_BothProvided(t *testing.T) {
    // Test: Both payment_method_id and payment_token provided
    // Expected: Prefer payment_method_id (Storage BRIC)
}

func TestResolvePaymentToken_NeitherProvided(t *testing.T) {
    // Test: Neither provided → error
    // Expected: error "either payment_method_id or payment_token required"
}

func TestResolvePaymentToken_InvalidUUID(t *testing.T) {
    // Test: payment_method_id = "not-a-uuid"
    // Expected: error "invalid payment_method_id format"
}

func TestResolvePaymentToken_PaymentMethodNotFound(t *testing.T) {
    // Test: payment_method_id doesn't exist in DB
    // Expected: error "payment method not found"
}
```

**Test Cases**:
1. ✅ Resolve from `payment_method_id` (Storage BRIC from DB)
2. ✅ Resolve from direct `payment_token` (Financial BRIC)
3. ⚠️ Both provided (prefer `payment_method_id`)
4. ❌ Neither provided → error
5. ❌ Invalid UUID format → error
6. ❌ Payment method not found → error

**Effort**: 1-2 hours (180 lines)

---

### Component 3: Browser Post Callback Handler (NEEDS BETTER TESTS)

**Current**: `browser_post_callback_handler_test.go` (750 lines, but 200 are mock stubs)

**Context from API_DESIGN_AND_DATAFLOW.md**:
- Handles 3 transaction types: CCE8 (Storage), CCE2 (Sale), CCE1 (Auth)
- Must verify EPX MAC signature
- Must route by `tranType`
- Must handle declined transactions

**Tests Needed** (currently missing or insufficient):

```go
func TestBrowserPostCallback_CCE8_StorageBRIC(t *testing.T) {
    // Test: CCE8 callback → SavePaymentMethod() called
    // Verify: payment_token, card_brand, last_four saved
}

func TestBrowserPostCallback_CCE2_Sale(t *testing.T) {
    // Test: CCE2 callback → CreateTransaction(type=sale, status=approved)
}

func TestBrowserPostCallback_CCE1_Auth(t *testing.T) {
    // Test: CCE1 callback → CreateTransaction(type=auth, status=approved)
}

func TestBrowserPostCallback_InvalidMAC(t *testing.T) {
    // Test: epxMac doesn't match computed MAC
    // Expected: 401 Unauthorized
}

func TestBrowserPostCallback_DeclinedTransaction(t *testing.T) {
    // Test: authResp != "00"
    // Expected: Log declined, return 200 OK (acknowledge)
}

func TestBrowserPostCallback_UnknownTranType(t *testing.T) {
    // Test: tranType = "UNKNOWN"
    // Expected: 400 Bad Request
}

func TestBrowserPostCallback_MissingAuthGUID(t *testing.T) {
    // Test: authGuid missing from payload
    // Expected: error
}
```

**Gap Analysis**:
- Current tests: Form generation, basic callback handling
- Missing: MAC verification tests, transaction type routing tests, error scenario tests

**Effort**: 2-3 hours (add 200 lines of focused tests)

---

## Problem 4: No Tests for Authentication Components (CRITICAL)

**Current State**:
- ✅ `merchant_resolver_test.go` (449 lines) - GOOD coverage
  - Tests merchant token resolution
  - Tests scope validation
  - Tests customer access validation

**Missing Tests**:

### Service Token Verification (MISSING)

**From AUTHENTICATION.md**: Service tokens are RSA-signed JWTs verified with public key from DB.

**Tests Needed**: `internal/auth/service_token_test.go`

```go
func TestVerifyServiceToken_ValidToken(t *testing.T) {
    // Test: Valid RSA-signed token → verified successfully
    // Setup: Generate RSA keypair, sign JWT, verify
}

func TestVerifyServiceToken_ExpiredToken(t *testing.T) {
    // Test: exp claim in the past → error
}

func TestVerifyServiceToken_InvalidSignature(t *testing.T) {
    // Test: Token signed with wrong key → verification fails
}

func TestVerifyServiceToken_MissingClaims(t *testing.T) {
    // Test: Missing iss, aud, exp claims → error
}

func TestVerifyServiceToken_ServiceNotFound(t *testing.T) {
    // Test: service_id not in database → error
}

func TestVerifyServiceToken_InactiveService(t *testing.T) {
    // Test: is_active = false → error
}
```

**Effort**: 2-3 hours (200 lines)

---

### Token Type Routing (MISSING)

**From AUTHENTICATION.md**: 5 token types with different verification methods.

**Tests Needed**: `internal/auth/token_router_test.go`

```go
func TestDetermineTokenType_ServiceToken(t *testing.T) {
    // Test: JWT with "iss" claim → TokenTypeService
}

func TestDetermineTokenType_AdminToken(t *testing.T) {
    // Test: JWT with "admin_id" claim → TokenTypeAdmin
}

func TestDetermineTokenType_CustomerToken(t *testing.T) {
    // Test: JWT with "customer_id" claim → TokenTypeCustomer
}

func TestDetermineTokenType_GuestToken(t *testing.T) {
    // Test: JWT with "guest" claim → TokenTypeGuest
}

func TestDetermineTokenType_InvalidToken(t *testing.T) {
    // Test: Malformed JWT → error
}
```

**Effort**: 1-2 hours (150 lines)

---

## Refactoring Recommendations

### Priority 1: High Impact, Low Risk (Do These First)

#### 1.1 Create Shared Mock Package ⭐⭐⭐

**Effort**: 3-4 hours | **LOC Savings**: -300 lines

**Steps**:
1. Create `internal/testutil/mocks/` package
2. Extract `MockServerPostAdapter`
3. Extract `MockSecretManagerAdapter`
4. Create minimal `MockQuerier` (only implement methods used)
5. Update all test files to import

---

#### 1.2 Create Shared Test Fixtures Package ⭐⭐⭐

**Effort**: 2-3 hours | **LOC Savings**: -50 lines

**Steps**:
1. Create `internal/testutil/fixtures/` package
2. Create `pointers.go`, `transactions.go`, `merchants.go`
3. Update existing tests (backward compatible)

---

### Priority 2: Critical for Refactoring (Write Before Implementation)

#### 2.1 Write Unit Tests for Merchant Credential Resolver ⭐⭐⭐

**Effort**: 1-2 hours | **LOC Added**: +150 lines

**Why**: Extracting this component from `payment_service.go` - need TDD tests first

---

#### 2.2 Write Unit Tests for Payment Token Resolver ⭐⭐⭐

**Effort**: 1-2 hours | **LOC Added**: +180 lines

**Why**: Extracting this component - need to understand Storage BRIC vs Financial BRIC handling

---

#### 2.3 Improve Browser Post Callback Handler Tests ⭐⭐

**Effort**: 2-3 hours | **LOC Added**: +200 lines

**Why**: Critical payment flow entry point - needs comprehensive tests for MAC verification, transaction type routing

---

### Priority 3: Fill Authentication Test Gaps

#### 3.1 Write Service Token Verification Tests ⭐⭐

**Effort**: 2-3 hours | **LOC Added**: +200 lines

**Why**: Service authentication is critical security component

---

#### 3.2 Write Token Type Routing Tests ⭐⭐

**Effort**: 1-2 hours | **LOC Added**: +150 lines

**Why**: Multi-tier authentication needs proper routing tests

---

## Implementation Roadmap

### Phase 1: Test Infrastructure (1 week)

**Day 1-2**: Shared mocks package
- Create `internal/testutil/mocks`
- Extract mock implementations
- Update imports

**Day 3-4**: Shared fixtures package
- Create `internal/testutil/fixtures`
- Create builders (transactions, merchants, payment methods)
- Update existing tests

**Day 5**: Documentation
- Write `tests/README.md`
- Write `testutil/mocks/README.md`
- Write `testutil/fixtures/README.md`

**Deliverables**:
- Eliminates 350 lines of duplication
- Shared test utilities

---

### Phase 2: Fill Test Gaps (1 week)

**Day 1**: Merchant credential resolver tests
**Day 2**: Payment token resolver tests
**Day 3**: Browser Post callback handler tests
**Day 4**: Service token verification tests
**Day 5**: Token type routing tests

**Deliverables**:
- +880 lines of new tests
- Comprehensive coverage for extracted components
- Ready for TDD refactoring

---

## Metrics to Track

### Before Refactoring

- **Unit test files**: 9
- **Total unit test LOC**: ~4,175 lines
- **Duplicated mock/helper LOC**: ~300 lines (7% waste)
- **Missing tests**: 5 components
- **Shared test utilities**: 0 packages

### After Refactoring (Target)

- **Unit test files**: 14 (+5)
- **Total unit test LOC**: ~4,700 lines
- **Duplicated LOC**: <50 lines (<1% waste) ✅
- **Missing tests**: 0 ✅
- **Shared test utilities**: 2 packages ✅

---

## Summary

| Opportunity | Impact | Effort | Risk | LOC Change | Priority |
|-------------|--------|--------|------|------------|----------|
| Create shared mocks | High | 3-4h | Low | -300 | P1 ⭐⭐⭐ |
| Create shared fixtures | Medium | 2-3h | Low | -50 | P1 ⭐⭐⭐ |
| Credential resolver tests | High | 1-2h | Low | +150 | P2 ⭐⭐⭐ |
| Token resolver tests | High | 1-2h | Low | +180 | P2 ⭐⭐⭐ |
| Callback handler tests | High | 2-3h | Low | +200 | P2 ⭐⭐ |
| Service token tests | Medium | 2-3h | Low | +200 | P3 ⭐⭐ |
| Token routing tests | Medium | 1-2h | Low | +150 | P3 ⭐⭐ |

**Total Effort**: 13-20 hours (~2 weeks)
**Net LOC**: -350 (removed) + 880 (added) = **+530 lines of better tests**

---

## Conclusion

The unit tests need improvement in **test infrastructure** and **coverage for authentication/callback handling**.

**Recommended Next Steps**:
1. ✅ Create shared test infrastructure (Phase 1)
2. ✅ Write unit tests for extracted components (Phase 2)
3. ✅ Use tests as safety net for implementation refactoring

**Overall Recommendation**: ✅ Proceed with test refactoring - clean infrastructure + comprehensive coverage will make implementation refactoring safer.

---

**Generated by**: Claude Code
**Review Date**: 2025-11-19
**Next Review**: After Phase 1 completion