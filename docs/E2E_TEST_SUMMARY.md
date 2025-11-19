# E2E and Integration Test Design Summary

**Last Updated**: 2025-11-19

---

## Test Classification Summary

### ✅ E2E Tests (4 Tests - `tests/e2e/`)

These tests focus on **multi-actor workflows** with complete authentication/authorization chains:

1. **Service Onboarding & Authentication** (`service_onboarding_test.go`)
   - Actors: Admin + Service
   - Flow: Admin creates service → Service authenticates → Payment API call
   - Tests: RSA keypair generation, storage, verification chain

2. **Token Delegation - Customer** (`token_delegation_customer_test.go`)
   - Actors: Service + Customer
   - Flow: Service → Customer token → Customer API
   - Tests: OAuth-style delegation, data isolation, token scoping

3. **Token Delegation - Guest** (`token_delegation_guest_test.go`)
   - Actors: Service + Guest
   - Flow: Service → Guest token → Guest views order
   - Tests: Single-use scoping, short-lived tokens (5 min)

4. **Multi-Merchant Authorization** (`multi_merchant_access_test.go`)
   - Actors: Admin + Service (across merchants)
   - Flow: Admin grants access → Service attempts access → Verify isolation
   - Tests: Multi-tenancy, scope enforcement, access revocation

**Run Frequency**: Nightly or pre-release
**Duration**: ~30-60 seconds total

---

### ✅ Integration Tests (5 Tests - `tests/integration/`)

These tests focus on **EPX adapter functionality** and **business logic** with single-actor workflows:

1. **ACH Pre-note → Storage BRIC** (`payment_method/ach_prenote_storage_test.go`)
   - Actor: Service only
   - Tests: CKC0 pre-note → CKC8 Storage BRIC → DB storage
   - Validates: Routing number validation, NACHA compliance, payment method storage

2. **ACH Debit with Verified Payment Method** (`payment/ach_debit_test.go`)
   - Actor: Service only
   - Tests: ACH debit (CKC2) with verified payment method
   - Validates: Verification requirement, transaction creation, settlement pending status

3. **ACH Return Handling** (`payment/ach_return_handling_test.go`)
   - Actor: EPX callback only
   - Tests: Return code processing (R01-R05) → Payment method deactivation
   - Validates: Return count tracking, auto-deactivation after 2 returns

4. **Browser Post Save Card** (Enhancement to `payment/browser_post_workflow_test.go`)
   - Actor: Customer via Browser Post
   - Tests: USER_DATA_2='save_card' → Financial→Storage BRIC → Saved for recurring
   - Validates: CCE8 conversion, Account Verification, NTID storage, PCI compliance

5. **Direct Storage BRIC (Server Post)** (`payment_method/credit_card_server_post_test.go`)
   - Actor: Service only
   - Tests: Raw card → CCE8 → Storage BRIC
   - Validates: Luhn check, PCI compliance (no card/CVV storage), NTID storage

**Run Frequency**: On every commit
**Duration**: ~10-25 seconds total

---

## Directory Structure

```
tests/
├── e2e/                                  # E2E tests (4 tests)
│   ├── README.md
│   ├── service_onboarding_test.go
│   ├── token_delegation_customer_test.go
│   ├── token_delegation_guest_test.go
│   ├── multi_merchant_access_test.go
│   └── helpers/
│       ├── admin_helpers.go
│       ├── service_helpers.go
│       ├── token_helpers.go
│       └── cleanup_helpers.go
│
└── integration/                          # Integration tests (20+ tests)
    ├── payment/
    │   ├── browser_post_workflow_test.go       # Enhanced with save card test
    │   ├── ach_debit_test.go                   # New
    │   ├── ach_return_handling_test.go         # New
    │   └── payment_service_critical_test.go    # Existing
    │
    └── payment_method/
        ├── ach_prenote_storage_test.go         # New
        ├── credit_card_server_post_test.go     # New
        └── payment_method_test.go              # Existing
```

---

## Success Criteria

### E2E Tests
- ✅ 4 E2E tests implemented (not 8)
- ✅ Tests pass consistently (< 1% flake rate)
- ✅ Test coverage >= 80% for auth flows
- ✅ Tests run in < 1 minute total
- ✅ Run nightly or pre-release

### Integration Tests
- ✅ 5 new/enhanced integration tests
- ✅ Run on every commit
- ✅ Tests run in < 30 seconds total
- ✅ Cover ACH and Storage BRIC flows

---

## Why This Classification?

### E2E Tests Focus On:
- ✅ Multi-actor workflows (Admin + Service + Customer/Guest)
- ✅ Authentication/authorization chain testing (RSA → HMAC delegation)
- ✅ Cross-service permission enforcement
- ✅ Multi-tenancy isolation
- ✅ Token delegation patterns (OAuth-style)

### Integration Tests Focus On:
- ✅ EPX adapter functionality (CKC0, CKC2, CKC8, CCE8)
- ✅ Database integration
- ✅ Business logic validation (returns, deactivation)
- ✅ Single-actor workflows
- ✅ Callback handling

---

## Implementation Priority

### Phase 1 (High Priority)
1. ✅ E2E Test 1: Service Onboarding & Authentication
2. ✅ Integration Test 1: ACH Pre-note → Storage BRIC
3. ✅ Integration Test 4: Browser Post Save Card

### Phase 2 (Medium Priority)
4. ✅ E2E Test 4: Multi-Merchant Authorization
5. ✅ Integration Test 2: ACH Debit
6. ✅ Integration Test 3: ACH Return Handling

### Phase 3 (Lower Priority)
7. ✅ E2E Test 2: Token Delegation - Customer
8. ✅ E2E Test 3: Token Delegation - Guest
9. ✅ Integration Test 5: Direct Storage BRIC

---

## References

- Full E2E Test Design: [E2E_TEST_DESIGN.md](./E2E_TEST_DESIGN.md)
- E2E vs Integration Analysis: [E2E_VS_INTEGRATION_ANALYSIS.md](./E2E_VS_INTEGRATION_ANALYSIS.md)
- Authentication Guide: [AUTHENTICATION.md](./AUTHENTICATION.md)
- ACH Business Logic: [ACH_BUSINESS_LOGIC.md](./ACH_BUSINESS_LOGIC.md)
- Credit Card Business Logic: [CREDIT_CARD_BUSINESS_LOGIC.md](./CREDIT_CARD_BUSINESS_LOGIC.md)

---

**Maintained By**: Backend Team
**Review Date**: 2025-11-19