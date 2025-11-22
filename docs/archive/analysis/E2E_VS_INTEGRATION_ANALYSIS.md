# E2E vs Integration Test Classification Analysis

**Date**: 2025-11-19
**Purpose**: Clarify boundaries between E2E and Integration tests for payment service

---

## Test Classification Criteria

### Integration Tests
**Characteristics**:
- Test integration between 2-3 components
- May use real dependencies (database, EPX gateway)
- Focused on specific feature workflows
- Single actor or simple multi-actor scenarios
- Faster execution (< 5 seconds per test)
- Run frequently (on every commit)
- Example: `TestStorePaymentMethod_ACH` - stores ACH, verifies DB record

**Location**: `tests/integration/`

### E2E Tests
**Characteristics**:
- Test complete business workflows start-to-finish
- Involve multiple distinct actors (admin + service + customer)
- Test entire authentication/authorization chain
- Span multiple service layers/boundaries
- Slower execution (5-30 seconds per test)
- Run less frequently (nightly, pre-release)
- Example: Admin creates service → Service authenticates → Service calls API

**Location**: `tests/e2e/`

---

## Proposed Test Analysis

### ✅ TRUE E2E TESTS (Keep as E2E)

#### Test 1: Service Onboarding & Authentication
**Actors**: Admin, Service
**Flow**: Admin login → CreateService RPC → Service generates token → Service API call
**Layers**: Admin API → Database → Service Token → Middleware → Payment API
**Justification**:
- Involves 2 distinct actors with different auth mechanisms
- Tests complete service lifecycle from registration to API usage
- Validates RSA keypair generation, storage, and verification chain
- No existing test covers this full flow
**Classification**: ✅ **E2E**

#### Test 2: Token Delegation - Service to Customer
**Actors**: Service, Customer
**Flow**: Service auth → Service creates transaction → Service requests customer token → Customer views transactions
**Layers**: Service API → Token delegation → Customer API → Authorization boundaries
**Justification**:
- Tests OAuth-style delegation pattern
- Involves service acting on behalf of customer
- Validates token scope enforcement and data isolation
- Multi-actor with different privilege levels
**Classification**: ✅ **E2E**

#### Test 3: Token Delegation - Service to Guest
**Actors**: Service, Guest
**Flow**: Service auth → Guest transaction → Service requests guest token → Guest views order
**Layers**: Service API → Token delegation → Guest API → Scoped authorization
**Justification**:
- Tests guest delegation with single-use scoping
- Validates anonymous user authorization boundaries
- Short-lived token testing (5 min expiry)
**Classification**: ✅ **E2E**

#### Test 4: Multi-Merchant Service Authorization
**Actors**: Admin, Service (across multiple merchants)
**Flow**: Admin creates merchants → Admin grants access → Service attempts access → Verify isolation
**Layers**: Admin API → Service-Merchant permissions → Payment API → Multi-tenancy
**Justification**:
- Tests multi-tenancy isolation across merchants
- Validates `service_merchants` table and scope enforcement
- Critical for production security (prevents cross-merchant data access)
- Involves multiple merchants and access control logic
**Classification**: ✅ **E2E**

---

### ❌ INTEGRATION TESTS (Move from E2E to Integration)

#### Test 5: ACH Complete Workflow (SPLIT)
**Original Proposal**: StoreACHAccount → Pre-note → ACHDebit → Void → Settlement

**Analysis**:
- **ACH Storage**: `TestStorePaymentMethod_ACH` already exists in `payment_method_test.go`
- **Pre-note flow**: Should be enhanced in existing integration test
- **ACH Debit**: Should be new integration test in `tests/integration/payment/`
- **Settlement monitoring**: This would be E2E (requires time-based monitoring)

**Recommendation**:
1. ✅ **Integration Test**: Enhance `TestStorePaymentMethod_ACH` to verify pre-note→storage BRIC flow
2. ✅ **Integration Test**: Add `TestACHDebit` to test debit with verified payment method
3. ❌ **Remove from E2E**: Not a multi-actor workflow, just EPX integration

**Rationale**:
- Single actor (service)
- Testing EPX adapter functionality
- No auth chain testing
- Fits existing integration test structure

#### Test 6: ACH Return Handling (MOVE TO INTEGRATION)
**Original Proposal**: Return webhook → Payment method deactivation → Business logic

**Analysis**:
- This tests webhook handling + business logic for returns
- Single actor (EPX callback → backend)
- Focuses on return code processing and payment method state changes
- Similar to existing `browser_post_callback_handler_test.go`

**Recommendation**: ❌ **Integration Test** in `tests/integration/payment/ach_return_handling_test.go`

**Rationale**:
- Tests EPX callback adapter
- Tests business logic for return handling
- No multi-actor workflow
- No auth chain testing
- Fits pattern of existing callback handler tests

#### Test 7: Browser Post Save Card (MOVE TO INTEGRATION)
**Original Proposal**: Browser Post SALE → Save card → Financial BRIC → Storage BRIC

**Analysis**:
- `browser_post_workflow_test.go` already tests Browser Post flows
- This is an extension to test `USER_DATA_2='save_card'` parameter
- Tests CCE8 conversion in callback handler
- Single workflow, no multi-actor

**Recommendation**: ❌ **Integration Test** - Add to existing `browser_post_workflow_test.go`

**New Test**: `TestIntegration_BrowserPost_SaveCard_Workflow`
```go
func TestIntegration_BrowserPost_SaveCard_Workflow(t *testing.T) {
    // Phase 1: Browser Post SALE with save_card flag
    // Phase 2: Callback with USER_DATA_2='save_card'
    // Phase 3: Verify Financial→Storage BRIC conversion
    // Phase 4: Verify payment method saved
    // Phase 5: Use saved card for recurring payment
}
```

**Rationale**:
- Extension of existing Browser Post tests
- Tests callback handler + BRIC conversion
- Single actor (customer via Browser Post)
- Fits existing integration test structure

#### Test 8: Direct Storage BRIC (MOVE TO INTEGRATION)
**Original Proposal**: Raw card → CCE8 → Storage BRIC (Server Post)

**Analysis**:
- This is testing a single RPC: `StoreCreditCard`
- `TestStorePaymentMethod_CreditCard` already exists but might use Browser Post
- Testing EPX Server Post adapter with CCE8
- Single actor, single operation

**Recommendation**: ❌ **Integration Test** in `payment_method_test.go`

**New Test**: `TestStorePaymentMethod_CreditCard_ServerPost`
```go
func TestStorePaymentMethod_CreditCard_ServerPost(t *testing.T) {
    // Phase 1: Call StoreCreditCard with raw card details
    // Phase 2: Verify CCE8 sent to EPX
    // Phase 3: Verify Storage BRIC created
    // Phase 4: Verify PCI compliance (no card number stored)
    // Phase 5: Verify NTID stored
}
```

**Rationale**:
- Single RPC testing
- EPX adapter integration
- Fits existing payment method test pattern
- No multi-actor workflow

---

## Revised Test Plan

### E2E Test Suite (4 tests)
**Location**: `tests/e2e/`

1. ✅ `service_onboarding_test.go` - Admin → Service → API
2. ✅ `token_delegation_customer_test.go` - Service → Customer token → Customer API
3. ✅ `token_delegation_guest_test.go` - Service → Guest token → Guest API
4. ✅ `multi_merchant_access_test.go` - Multi-tenant isolation

**Run**: Nightly or pre-release
**Duration**: ~30-60 seconds total

### Enhanced Integration Tests
**Location**: `tests/integration/`

#### New Integration Tests:
1. ❌ `payment/ach_debit_test.go` - ACH debit with verified payment method
2. ❌ `payment/ach_return_handling_test.go` - Return code processing
3. ❌ `payment_method/credit_card_server_post_test.go` - Direct Storage BRIC

#### Enhanced Existing Tests:
1. ❌ `payment_method/payment_method_test.go::TestStorePaymentMethod_ACH`
   - Add pre-note verification checks
   - Add Storage BRIC conversion validation

2. ❌ `payment/browser_post_workflow_test.go::TestIntegration_BrowserPost_SaveCard`
   - Add USER_DATA_2='save_card' test
   - Add Financial→Storage BRIC conversion
   - Add saved card reuse test

**Run**: On every commit
**Duration**: ~2-5 seconds per test

---

## Decision Matrix

| Test | Multi-Actor? | Auth Chain? | Layers Spanned | Classification |
|------|-------------|-------------|----------------|----------------|
| Service Onboarding | ✓ (Admin + Service) | ✓ (Admin HMAC + Service RSA) | Admin API → Service API | **E2E** |
| Token Delegation (Customer) | ✓ (Service + Customer) | ✓ (Service RSA → Customer HMAC) | Service → Token API → Customer API | **E2E** |
| Token Delegation (Guest) | ✓ (Service + Guest) | ✓ (Service RSA → Guest HMAC) | Service → Token API → Guest API | **E2E** |
| Multi-Merchant Auth | ✓ (Admin + Service) | ✓ (Full permission model) | Admin → Permissions → Service API | **E2E** |
| ACH Storage | ✗ (Service only) | ✗ (Single service token) | Payment Method API → EPX | **Integration** |
| ACH Debit | ✗ (Service only) | ✗ (Single service token) | Payment API → EPX | **Integration** |
| ACH Returns | ✗ (EPX callback) | ✗ (MAC verification) | Webhook → Business Logic | **Integration** |
| Browser Post Save Card | ✗ (Customer only) | ✗ (Browser Post → Callback) | Browser Post → Callback → Storage | **Integration** |
| Direct Storage BRIC | ✗ (Service only) | ✗ (Single service token) | Payment Method API → EPX | **Integration** |

---

## Key Differences Highlighted

### E2E Tests Focus On:
- ✅ Multi-actor workflows (Admin + Service + Customer/Guest)
- ✅ Authentication/authorization chain testing
- ✅ Token delegation patterns (OAuth-style)
- ✅ Cross-service permission enforcement
- ✅ Multi-tenancy isolation
- ✅ Complete business flows involving different privilege levels

### Integration Tests Focus On:
- ✅ EPX adapter functionality
- ✅ Database integration
- ✅ Business logic validation
- ✅ Payment method storage/retrieval
- ✅ Transaction processing
- ✅ Callback handling
- ✅ Single-actor workflows

---

## Rationale for Reclassification

### Why Move ACH/BRIC Tests to Integration?

1. **No Multi-Actor Workflows**
   - All ACH/BRIC tests involve single actor (service or EPX callback)
   - No admin setup required
   - No token delegation
   - No privilege escalation/delegation

2. **Existing Test Structure Alignment**
   - `payment_method_test.go` already has ACH/credit card storage tests
   - `browser_post_workflow_test.go` already has Browser Post tests
   - These are natural homes for the proposed tests

3. **Execution Speed**
   - ACH/BRIC tests are fast (< 5 seconds)
   - Don't need slower E2E execution cadence
   - Can run on every commit

4. **Focus on Integration, Not Authorization**
   - These tests focus on EPX integration
   - They test adapter functionality and business logic
   - Authorization is not the primary concern (service token is assumed valid)

5. **Reduced E2E Maintenance**
   - E2E tests should be high-value, multi-actor scenarios
   - Keeping E2E suite small (4 tests) makes it maintainable
   - Integration tests can be more numerous and detailed

---

## Final Recommendation

**E2E Test Suite**: 4 tests focused on multi-actor auth workflows
**Integration Test Enhancements**: 5 new/enhanced tests for ACH/BRIC flows

This provides:
- ✅ Clear separation of concerns
- ✅ Faster CI/CD (integration tests run frequently)
- ✅ High-value E2E tests (run less frequently)
- ✅ No redundancy with existing tests
- ✅ Better maintainability

---

**Approved By**: Backend Team
**Review Date**: 2025-11-19
