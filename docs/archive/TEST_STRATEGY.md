# Test Strategy: Unit vs Integration

## Principle: Test the Right Thing at the Right Level

### Unit Tests (Fast, Isolated, Business Logic)
**What to test:**
- ✅ Business logic validation (WAL state computation)
- ✅ Domain rules (CanCapture, CanVoid, CanRefund)
- ✅ Edge cases in pure functions
- ✅ Mathematical calculations (amount tracking)
- ✅ State transitions without external dependencies

**What NOT to test:**
- ❌ Database queries
- ❌ HTTP/gRPC handlers
- ❌ External API calls (EPX)
- ❌ JSON marshaling/unmarshaling
- ❌ Authentication/authorization

### Integration Tests (Slower, Real Dependencies)
**What to test:**
- ✅ End-to-end workflows through HTTP/gRPC API
- ✅ Database persistence and retrieval
- ✅ External API integration (EPX Browser Post/Server Post)
- ✅ Transaction isolation and concurrency
- ✅ Request/response parsing
- ✅ Authentication/authorization flows
- ✅ Idempotency with real database

**What NOT to test:**
- ❌ Business logic validation (move to unit tests)
- ❌ Complex state computation (move to unit tests)
- ❌ Edge cases of pure functions

---

## Current Integration Tests: Analysis

### ✅ **KEEP - Tests Real Integration**

#### 1. **Browser Post End-to-End** (`browser_post_test.go`)
```go
TestBrowserPost_EndToEnd_Success
TestBrowserPost_Callback_Idempotency
TestBrowserPost_Callback_DeclinedTransaction
TestBrowserPost_Callback_GuestCheckout
```
**Why Keep:** Tests real EPX Key Exchange API, form generation, callback handling, database persistence
**Tests:** HTTP handlers, EPX API, database, signature validation

#### 2. **Browser Post Workflow** (`browser_post_workflow_test.go`)
```go
TestBrowserPost_AuthCapture_Workflow
```
**Why Keep:** Tests complete AUTH → CAPTURE → REFUND flow with real EPX BRIC
**Tests:** Multi-step workflow, BRIC persistence, EPX API chaining

#### 3. **gRPC API** (`grpc_grpc_test.go`)
```go
TestGRPC_ListTransactions
TestGRPC_GetTransaction
```
**Why Keep:** Tests gRPC protocol, request/response marshaling, database queries
**Tests:** gRPC handlers, protobuf serialization, pagination

#### 4. **Payment Method Storage** (`payment_method_test.go`)
```go
TestStorePaymentMethod_CreditCard
```
**Why Keep:** Tests EPX BRIC tokenization, database storage, encryption
**Tests:** EPX tokenization API, database persistence, BRIC storage

#### 5. **Merchant/Subscription** (`merchant_test.go`, `subscription_test.go`)
```go
TestGetMerchant_FromSeedData
TestCreateSubscription_WithStoredCard
```
**Why Keep:** Tests database seeding, multi-tenant isolation, foreign keys
**Tests:** Database queries, data integrity, tenant isolation

---

### ❌ **REFACTOR - Move to Unit Tests**

#### 1. **State Transition Tests** (`state_transition_test.go`)
```go
TestStateTransition_VoidAfterCapture
TestStateTransition_CaptureAfterVoid
TestStateTransition_PartialCaptureValidation
TestStateTransition_MultipleCaptures
TestStateTransition_RefundWithoutCapture
```
**Why Refactor:** These test business logic (state validation), not integration
**Move to:** `group_state_test.go` as table-driven unit tests
**Already covered by:** Our new unit tests (TestComputeGroupState_*, TestCan*)

#### 2. **Idempotency Logic Tests** (`idempotency_test.go`)
```go
TestRefund_Idempotency_ClientGeneratedUUID (business logic part)
TestConcurrentRefunds_SameUUID (validation part)
TestTransactionIDUniqueness (business logic part)
```
**Why Refactor:**
- Business logic (can we refund?) → unit test
- Database constraint (PK uniqueness) → keep as integration test
- Concurrent requests → keep as integration test

**Split into:**
- **Unit test:** Test that validation prevents double refund
- **Integration test:** Test that database prevents duplicate transaction_id

#### 3. **Refund/Void Validation** (`refund_void_test.go`)
```go
TestRefundValidation (business logic)
TestVoidValidation (business logic)
TestMultipleRefunds_SameGroup (business logic)
```
**Why Refactor:** These test validation rules, not integration
**Move to:** Table-driven unit tests in `group_state_test.go`
**Already covered by:** TestCanRefund_Success, TestCanVoid_Success

---

## Integration Tests: What Should Remain

### **Category 1: External API Integration**
```
✅ TestBrowserPost_EndToEnd_Success
   - EPX Key Exchange API (TAC generation)
   - EPX Browser Post callback (signature validation)
   - Database persistence with BRIC

✅ TestBrowserPost_Callback_Idempotency
   - EPX callback retry behavior
   - Database ON CONFLICT DO NOTHING
   - Real network retries

✅ TestStorePaymentMethod_CreditCard
   - EPX tokenization API (BRIC storage)
   - Database encryption/decryption
```

### **Category 2: End-to-End Workflows**
```
✅ TestBrowserPost_AuthCapture_Workflow
   - Browser Post AUTH → Server Post CAPTURE → Server Post REFUND
   - BRIC token chaining
   - Multi-step state persistence

✅ TestSaleTransaction_WithStoredCard (if using real EPX)
   - Retrieve stored BRIC from database
   - EPX Server Post SALE
   - Transaction creation
```

### **Category 3: Database & Concurrency**
```
✅ TestConcurrentRefunds_SameUUID
   - Race condition handling
   - Row-level locking (SELECT FOR UPDATE)
   - Database isolation levels

✅ TestTransactionIDUniqueness
   - Primary key constraint enforcement
   - Database error handling
```

### **Category 4: HTTP/gRPC Protocol**
```
✅ TestGRPC_ListTransactions
   - gRPC request/response
   - Pagination logic
   - Error handling

✅ TestBrowserPost_Callback_DeclinedTransaction
   - HTTP callback parsing
   - Error response handling
```

### **Category 5: Multi-Tenant & Security**
```
✅ TestGetMerchant_FromSeedData
   - Merchant isolation
   - Credentials storage
   - Secret manager integration

✅ TestBrowserPost_Callback_GuestCheckout
   - Guest vs registered user flows
   - Customer ID handling
```

---

## Unit Tests: What Should Be Added

### **Table-Driven Test Pattern**

#### Example: State Validation (Already Implemented ✅)
```go
func TestCanRefund_Success(t *testing.T) {
    tests := []struct {
        name           string
        capturedAmount string
        refundedAmount string
        refundAmount   string
        shouldAllow    bool
    }{
        {"full refund", "100.00", "0", "100.00", true},
        {"partial refund", "100.00", "0", "60.00", true},
        {"exceed captured", "100.00", "0", "100.01", false},
        // ... 7 test cases total
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test business logic without database
        })
    }
}
```

#### Add: Amount Calculation Tests
```go
func TestComputeRefundableAmount(t *testing.T) {
    tests := []struct {
        name       string
        captured   decimal.Decimal
        refunded   decimal.Decimal
        expected   decimal.Decimal
    }{
        {"no refunds", d("100.00"), d("0"), d("100.00")},
        {"partial refund", d("100.00"), d("30.00"), d("70.00")},
        {"full refund", d("100.00"), d("100.00"), d("0")},
    }
    // ...
}
```

#### Add: BRIC Selection Tests (Already Implemented ✅)
```go
func TestGetBRICForOperation(t *testing.T) {
    tests := []struct {
        name         string
        state        *GroupState
        operation    domain.TransactionType
        expectedBRIC string
    }{
        {"CAPTURE uses AUTH BRIC", stateWithAuth, CAPTURE, "bric_auth"},
        {"REFUND uses CAPTURE BRIC", stateWithCapture, REFUND, "bric_capture"},
        // ... 4 test cases total
    }
    // ...
}
```

---

## Refactoring Plan

### Phase 1: Move Business Logic to Unit Tests ✅ DONE
- [x] Create `group_state_test.go` with table-driven tests
- [x] Test WAL state computation (12 tests)
- [x] Test validation rules (19 tests)
- [x] Test BRIC selection (4 tests)
- [x] Test complex workflows (1 test)

### Phase 2: Clean Up Integration Tests
- [ ] Remove business logic tests from `state_transition_test.go`
- [ ] Keep only database/API integration tests in `idempotency_test.go`
- [ ] Remove validation logic tests from `refund_void_test.go`
- [ ] Keep only end-to-end workflow tests

### Phase 3: Add Missing Unit Tests
- [ ] Edge cases for decimal arithmetic
- [ ] VOID metadata parsing logic
- [ ] Transaction type conversions
- [ ] Nullable field handling

### Phase 4: Add Missing Integration Tests
- [ ] Concurrent CAPTURE on same AUTH
- [ ] Concurrent REFUND on same CAPTURE
- [ ] Database rollback on EPX failure
- [ ] Network timeout handling
- [ ] Webhook signature validation

---

## Test Pyramid

```
              /\
             /  \    E2E Tests (Browser Post workflow)
            /____\   Integration Tests (EPX API, Database, gRPC)
           /      \  Unit Tests (Business Logic, Validation)
          /        \
         /__________\

Unit:        36 tests (fast, isolated)
Integration: ~15 tests (real dependencies)
E2E:         ~3 tests (complete workflows)
```

---

## Examples: Before & After

### BEFORE (Integration test testing business logic ❌)
```go
// state_transition_test.go
func TestStateTransition_RefundWithoutCapture(t *testing.T) {
    // Setup real database
    // Create AUTH transaction in DB
    // Try REFUND via HTTP API
    // Assert it fails

    // ❌ This tests business logic, not integration!
}
```

### AFTER (Unit test ✅)
```go
// group_state_test.go
func TestCanRefund_NoCapturedAmount(t *testing.T) {
    state := &GroupState{
        CapturedAmount: decimal.Zero,
        RefundedAmount: decimal.Zero,
    }

    refundAmt, _ := decimal.NewFromString("50.00")
    canRefund, reason := state.CanRefund(refundAmt)

    assert.False(t, canRefund)
    assert.Equal(t, "no captured amount to refund", reason)

    // ✅ Fast, no database, tests business rule
}
```

### KEEP (Integration test testing real integration ✅)
```go
// browser_post_test.go
func TestBrowserPost_EndToEnd_Success(t *testing.T) {
    // Call real EPX Key Exchange API
    // Generate form with TAC
    // Simulate EPX callback with signature
    // Verify database persistence
    // Verify BRIC storage

    // ✅ Tests real EPX API, database, HTTP handlers
}
```

---

## Summary

### Unit Tests Should Test:
1. ✅ Pure business logic (no I/O)
2. ✅ Validation rules
3. ✅ State computation (WAL)
4. ✅ Edge cases in calculations
5. ✅ Domain object behavior

### Integration Tests Should Test:
1. ✅ External API calls (EPX)
2. ✅ Database queries and transactions
3. ✅ HTTP/gRPC request/response
4. ✅ End-to-end workflows
5. ✅ Concurrency and locking
6. ✅ Authentication/authorization
7. ✅ Idempotency with real persistence

### What We've Accomplished:
- ✅ Created 36 unit tests for business logic
- ✅ Used table-driven test pattern
- ✅ Separated concerns correctly
- ✅ Fast tests (0.004s for all unit tests)
- 🔄 Integration tests need cleanup (remove business logic tests)
