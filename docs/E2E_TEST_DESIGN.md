# End-to-End (E2E) Test Design

**Last Updated**: 2025-11-19
**Status**: Design Phase
**Purpose**: Define E2E test strategy that complements existing unit and integration tests

---

## Table of Contents

1. [Overview](#overview)
2. [E2E vs Integration Classification](#e2e-vs-integration-classification)
3. [Current Test Coverage Analysis](#current-test-coverage-analysis)
4. [E2E Test Strategy](#e2e-test-strategy)
5. [E2E Test Suite (4 Core Tests)](#e2e-test-suite-4-core-tests)
6. [Integration Test Enhancements](#integration-test-enhancements)
7. [Test Infrastructure](#test-infrastructure)
8. [Implementation Guidelines](#implementation-guidelines)

---

## Overview

### What is E2E Testing?

End-to-end testing validates complete business workflows from start to finish, involving multiple actors (admin, service, customer, guest) and spanning the entire authentication and authorization chain.

### Goals

- **Validate complete workflows** that span multiple services and actors
- **Test authentication/authorization boundaries** not covered by unit/integration tests
- **Verify token delegation patterns** (Service → Customer/Guest tokens)
- **Ensure multi-tenancy isolation** across merchants and services
- **Complement existing tests** without redundancy

### Principles

1. **Start from Admin** - Every E2E test begins with admin creating resources
2. **Multi-Actor** - Involve admin, service, customer, and/or guest actors
3. **Complete Flows** - Test end-to-end scenarios, not isolated operations
4. **Authorization Focus** - Emphasize auth boundaries and isolation
5. **Self-Contained** - Each test creates and cleans up its own data
6. **No Redundancy** - Don't duplicate existing integration test coverage

---

## E2E vs Integration Classification

### Decision Criteria

After careful analysis (see [E2E_VS_INTEGRATION_ANALYSIS.md](./E2E_VS_INTEGRATION_ANALYSIS.md) for full details), we've reclassified the proposed tests:

**E2E Tests** (4 tests - `tests/e2e/`):
- ✅ Multi-actor workflows (Admin + Service + Customer/Guest)
- ✅ Complete authentication/authorization chains
- ✅ Token delegation patterns
- ✅ Cross-service permission enforcement
- ✅ Multi-tenancy isolation
- Run: Nightly or pre-release

**Integration Tests** (5+ tests - `tests/integration/`):
- ✅ EPX adapter functionality
- ✅ Database integration
- ✅ Business logic validation
- ✅ Single-actor workflows
- ✅ Callback handling
- Run: On every commit

### Key Distinction

| Aspect | E2E | Integration |
|--------|-----|-------------|
| **Actors** | Multiple (Admin, Service, Customer) | Single (Service or EPX callback) |
| **Auth Testing** | Full chain (RSA → HMAC delegation) | Single token type |
| **Layers** | Admin API → Service API → Customer API | Payment API → EPX |
| **Focus** | Authorization boundaries | Feature functionality |
| **Duration** | 10-30 seconds per test | 2-5 seconds per test |
| **Run Frequency** | Nightly | Every commit |

### What Changed?

**Original Proposal**: 8 E2E tests
**Revised Plan**:
- **4 E2E tests** (multi-actor auth workflows)
- **5 Integration tests** (ACH/BRIC flows moved)

**Tests Reclassified as Integration**:
- ❌ ACH Complete Workflow → Split between existing integration tests
- ❌ ACH Return Handling → New integration test
- ❌ Browser Post Save Card → Enhanced existing Browser Post integration test
- ❌ Direct Storage BRIC → New integration test in payment_method_test.go

**Rationale**: These tests focus on EPX integration and business logic, not multi-actor authorization chains.

---

## Current Test Coverage Analysis

### ✅ Well-Covered Areas (Integration Tests)

**Browser Post Workflows** (`browser_post_workflow_test.go`):
- SALE → REFUND with real EPX BRIC
- AUTH → CAPTURE → REFUND workflow
- AUTH → VOID workflow
- Automated Chrome-based testing

**Payment Service Critical Tests** (`payment_service_critical_test.go`):
- Idempotency verification (duplicate prevention)
- Refund amount validation (business rules)
- Capture state validation (state machine)
- Concurrent operation handling (race conditions)
- EPX decline code handling (error scenarios)

**Payment Method Tests** (`payment_method_test.go`):
- Store credit card via BRIC
- Store ACH account
- Get/list payment methods
- Payment method management

**Other Coverage**:
- Transaction state transitions (`state_transition_test.go`)
- Subscription operations (`subscription_test.go`)
- Merchant operations (`merchant_test.go`)
- ConnectRPC protocol (`connect_protocol_test.go`)
- Auth primitives (`auth_test.go`)

### ❌ Gaps - What's NOT Covered

1. **Admin Service Creation Flow**
   - No tests for `CreateService` RPC
   - No tests for keypair generation → storage → verification
   - No tests for service registration end-to-end

2. **Service Authentication E2E**
   - Unit tests cover JWT generation/validation
   - But NOT the full: Admin creates service → Service authenticates → API call flow

3. **Token Delegation Pattern**
   - No tests for Service → Customer token → Customer API call
   - No tests for Service → Guest token → Guest API call
   - OAuth-style delegation pattern untested

4. **Multi-Merchant Authorization**
   - No tests for service access control across merchants
   - No tests for `service_merchants` scope enforcement
   - Multi-tenancy isolation untested

5. **ACH Complete Workflows**
   - No E2E tests for: StoreACHAccount → ACHDebit → Settlement monitoring
   - Pre-note verification flow not tested end-to-end
   - ACH return handling not tested

6. **Browser Post Storage BRIC Workflows**
   - Browser Post tested, but NOT conversion flow
   - Financial BRIC → Storage BRIC conversion untested
   - Account Verification (CCE0) flow not covered

---

## E2E Test Strategy

### Test Categories

```
┌─────────────────────────────────────────────────────────────┐
│                    E2E Test Categories                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Authentication Flows                                    │
│     ├─ Admin creates service                                │
│     ├─ Service authenticates with service token             │
│     └─ Token verification in middleware                     │
│                                                             │
│  2. Token Delegation Flows                                  │
│     ├─ Service → Customer token → Customer API              │
│     ├─ Service → Guest token → Guest API                    │
│     └─ Token scope enforcement                              │
│                                                             │
│  3. Multi-Merchant Authorization                            │
│     ├─ Service access across merchants                      │
│     ├─ Scope enforcement (read vs write)                    │
│     └─ Access revocation                                    │
│                                                             │
│  4. ACH Complete Workflows                                  │
│     ├─ StoreACHAccount → Pre-note → Verification            │
│     ├─ ACHDebit → Settlement → Return handling              │
│     └─ ACH Void timing constraints                          │
│                                                             │
│  5. Browser Post Storage BRIC                               │
│     ├─ Browser Post SALE → Save Card request                │
│     ├─ Financial BRIC → Storage BRIC conversion             │
│     └─ Account Verification (CCE0) validation               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### What NOT to Test (Redundant)

❌ **Browser Post transaction flows** - Already covered by `browser_post_workflow_test.go`
❌ **Transaction state transitions** - Already covered by `state_transition_test.go`
❌ **Idempotency** - Already covered by `payment_service_critical_test.go`
❌ **EPX integration details** - Already covered throughout integration tests
❌ **Business logic validation** - Already covered (refund limits, etc.)

---

## E2E Test Suite (4 Core Tests)

These tests focus on **multi-actor workflows** involving Admin, Service, Customer, and Guest actors with complete authentication/authorization chains.

**Rationale for E2E Classification**:
- ✅ Multiple distinct actors with different auth mechanisms
- ✅ Complete auth chain testing (RSA → HMAC delegation)
- ✅ Cross-service permission enforcement
- ✅ Multi-tenancy isolation validation
- ✅ Not testable with single-actor integration tests

### Test 1: Service Onboarding & Authentication Flow

**E2E Classification Reason**: Tests complete lifecycle from Admin (HMAC auth) → Service creation → Service (RSA auth) → Payment API, spanning 2 actors and 3 service layers.

**File**: `tests/e2e/service_onboarding_test.go`

**Purpose**: Validate the complete service registration and authentication flow from admin creation through API call.

**Flow**:
```
┌─────────────────────────────────────────────────────────────┐
│ E2E Test: Admin Creates Service → Service Authenticates     │
└─────────────────────────────────────────────────────────────┘

Phase 1: Admin Login
  ├─ Admin authenticates with email/password
  └─ Receives admin token (HMAC-signed)

Phase 2: Admin Creates Service
  ├─ POST /admin/v1/CreateService
  │   Headers: Authorization: Bearer <admin-token>
  │   Body: {
  │     service_id: "test-service-001",
  │     service_name: "Test Service",
  │     environment: "staging"
  │   }
  │
  └─ Response: {
      service: { id, service_id, public_key_fingerprint, ... },
      private_key: "-----BEGIN RSA PRIVATE KEY-----..." ← ONE-TIME
    }

Phase 3: Service Generates Token
  ├─ Load private key from response
  ├─ Create JWT with claims:
  │   {
  │     "iss": "test-service-001",
  │     "aud": "payment-service",
  │     "exp": <15 min from now>,
  │     "iat": <current time>
  │   }
  └─ Sign with RSA private key (RS256)

Phase 4: Service Makes API Call
  ├─ POST /payment/v1/Sale
  │   Headers: Authorization: Bearer <service-token>
  │   Body: {
  │     merchant_id: "...",
  │     amount: "50.00",
  │     payment_method_id: "..."
  │   }
  │
  └─ Middleware verifies:
      ├─ Extracts service_id from JWT
      ├─ Fetches public key from DB
      ├─ Verifies RSA signature
      ├─ Checks service is_active
      └─ Injects service context
```

**Assertions**:
- ✓ Admin receives private key exactly once
- ✓ Public key fingerprint matches
- ✓ Service token is valid (RS256)
- ✓ Payment API call succeeds with service auth
- ✓ Invalid signature is rejected
- ✓ Deactivated service is rejected
- ✓ Expired token is rejected

**Cleanup**:
- Delete test service
- Delete test transactions
- Delete test audit logs

---

### Test 2: Token Delegation - Service to Customer

**File**: `tests/e2e/token_delegation_customer_test.go`

**Purpose**: Validate OAuth-style token delegation from service to customer.

**Flow**:
```
┌────────────────────────────────────────────────────────────┐
│ E2E Test: Service → Customer Token → Customer Views Txns   │
└────────────────────────────────────────────────────────────┘

Phase 1: Setup Service Authentication
  └─ Use service from Test 1 or create new service

Phase 2: Service Creates Transaction for Customer
  ├─ Service authenticates with service token
  └─ POST /payment/v1/Sale
      Body: {
        merchant_id: "merchant-001",
        customer_id: "customer-001",  ← Links to customer
        amount: "29.99",
        payment_method_id: "..."
      }

Phase 3: Service Requests Customer Token
  ├─ POST /auth/v1/customer-token
  │   Headers: Authorization: Bearer <service-token>
  │   Body: {
  │     customer_id: "customer-001",
  │     merchant_id: "merchant-001"
  │   }
  │
  └─ Response: {
      token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",  ← HMAC-signed
      expires_at: "2025-11-19T15:30:00Z"  ← 30 minutes
    }

Phase 4: Customer Uses Customer Token
  ├─ GET /customer/v1/transactions
  │   Headers: Authorization: Bearer <customer-token>
  │
  └─ Response: {
      transactions: [
        { id: "tx-001", amount: "29.99", ... }  ← Customer's txn
      ]
    }

Phase 5: Verify Authorization Boundaries
  ├─ Create transaction for different customer (customer-002)
  ├─ Attempt to view customer-002's transaction with customer-001 token
  └─ Expected: 403 Forbidden (isolation enforced)
```

**Assertions**:
- ✓ Service can request customer token with valid service token
- ✓ Customer token is HMAC-signed (not RSA)
- ✓ Customer can view their own transactions
- ✓ Customer CANNOT view other customers' transactions
- ✓ Customer token expires after 30 minutes
- ✓ Customer token scope is `read:transactions`
- ✓ Customer token is rejected for write operations

**Cleanup**:
- Delete test transactions
- Delete test customers
- Invalidate tokens

---

### Test 3: Token Delegation - Service to Guest

**File**: `tests/e2e/token_delegation_guest_test.go`

**Purpose**: Validate guest token flow for anonymous order lookup.

**Flow**:
```
┌──────────────────────────────────────────────────────────┐
│ E2E Test: Service → Guest Token → Guest Views Order      │
└──────────────────────────────────────────────────────────┘

Phase 1: Service Creates Guest Transaction
  ├─ Service authenticates with service token
  └─ POST /payment/v1/Sale
      Body: {
        merchant_id: "merchant-001",
        customer_id: null,  ← No customer (guest checkout)
        amount: "99.99",
        payment_method_id: "..."
      }

Phase 2: Service Requests Guest Token
  ├─ POST /auth/v1/guest-token
  │   Headers: Authorization: Bearer <service-token>
  │   Body: {
  │     parent_transaction_id: "tx-guest-001"
  │   }
  │
  └─ Response: {
      token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
      expires_at: "2025-11-19T14:05:00Z"  ← 5 minutes (short-lived)
    }

Phase 3: Guest Views Order
  ├─ GET /guest/v1/orders/{parent_transaction_id}
  │   Headers: Authorization: Bearer <guest-token>
  │
  └─ Response: {
      transaction_id: "tx-guest-001",
      amount: "99.99",
      status: "approved"
    }

Phase 4: Verify Scoping
  ├─ Create different guest transaction (tx-guest-002)
  ├─ Attempt to view tx-guest-002 with tx-guest-001 token
  └─ Expected: 403 Forbidden (single-use scoping)
```

**Assertions**:
- ✓ Service can request guest token for specific transaction
- ✓ Guest token is short-lived (5 minutes)
- ✓ Guest can view only the specific order
- ✓ Guest CANNOT view other orders
- ✓ Guest token is single-use scoped
- ✓ Guest token expires quickly

---

### Test 4: Multi-Merchant Service Authorization

**File**: `tests/e2e/multi_merchant_access_test.go`

**Purpose**: Validate multi-tenancy isolation and service access control.

**Flow**:
```
┌──────────────────────────────────────────────────────────────┐
│ E2E Test: Service Access Control Across Multiple Merchants   │
└──────────────────────────────────────────────────────────────┘

Phase 1: Setup Multiple Merchants
  ├─ Admin creates merchant A ("acme-corp")
  └─ Admin creates merchant B ("widgets-inc")

Phase 2: Grant Service Access to Merchant A Only
  ├─ POST /admin/v1/service-merchants/grant
  │   Body: {
  │     service_id: "test-service-001",
  │     merchant_id: "acme-corp",
  │     scopes: ["payment:write", "payment:read"]
  │   }
  │
  └─ service_merchants table:
      service_id=test-service-001, merchant_id=acme-corp, scopes=[payment:write]

Phase 3: Test Authorized Access (Merchant A)
  ├─ Service authenticates with service token
  ├─ POST /payment/v1/Sale
  │   Body: { merchant_id: "acme-corp", amount: "50.00", ... }
  │
  └─ Middleware checks:
      ├─ Service token valid ✓
      ├─ CheckServiceHasScope(service=test-service-001, merchant=acme-corp, scope=payment:write)
      └─ Returns TRUE → Allow

Phase 4: Test Unauthorized Access (Merchant B)
  ├─ POST /payment/v1/Sale
  │   Body: { merchant_id: "widgets-inc", amount: "50.00", ... }
  │
  └─ Middleware checks:
      ├─ Service token valid ✓
      ├─ CheckServiceHasScope(service=test-service-001, merchant=widgets-inc, scope=payment:write)
      └─ Returns FALSE → 403 Forbidden

Phase 5: Test Scope Enforcement
  ├─ Grant service ONLY payment:read to merchant A
  ├─ Attempt POST /payment/v1/Sale (requires payment:write)
  └─ Expected: 403 Forbidden (insufficient scope)

Phase 6: Test Access Revocation
  ├─ Revoke service access to merchant A
  ├─ Attempt payment for merchant A
  └─ Expected: 403 Forbidden
```

**Assertions**:
- ✓ Service can access authorized merchant A
- ✓ Service CANNOT access unauthorized merchant B
- ✓ Scope enforcement works (read vs write)
- ✓ Access revocation is immediate
- ✓ Multi-tenancy isolation maintained

---

## Integration Test Enhancements

After careful analysis, Tests 5-8 should be **Integration Tests** rather than E2E tests. They focus on EPX adapter functionality and business logic with single-actor workflows, not multi-actor authorization chains.

**Integration Test Classification Reasons**:
- ❌ Single actor (Service or EPX callback)
- ❌ No multi-actor auth chain testing
- ❌ Focus on EPX integration and business logic
- ❌ Fits existing integration test patterns
- ✅ Should run on every commit (not nightly)
- ✅ Faster execution (2-5 seconds vs 10-30 seconds)

---

### Integration Test 1: ACH Pre-note → Storage BRIC Flow

**Integration Classification Reason**: Single-actor (Service) test focusing on EPX adapter CKC0/CKC8 integration and payment method storage business logic. No multi-actor auth chain testing.

**File**: `tests/integration/payment_method/ach_prenote_storage_test.go`

**RPC Used**: `PaymentMethodService.StoreACHAccount`

**Purpose**: Validate ACH account storage with automatic pre-note (CKC0) and Storage BRIC (CKC8) creation.

**Flow**:
```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: StoreACHAccount → Pre-note → Storage BRIC  │
└──────────────────────────────────────────────────────────────┘

Phase 1: Service Stores ACH Account
  ├─ RPC: PaymentMethodService.StoreACHAccount
  │   Body: {
  │     merchant_id: "test-merchant",
  │     customer_id: "test-customer",
  │     account_number: "1234567890",
  │     routing_number: "021000021",
  │     account_holder_name: "John Doe",
  │     account_type: ACCOUNT_TYPE_CHECKING,
  │     std_entry_class: STD_ENTRY_CLASS_PPD
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Validate routing number (checksum)
      ├─ Step 2: Send Pre-note (CKC0) to EPX
      │   TRAN_TYPE: CKC0
      │   AMOUNT: 0.00
      │   ACCOUNT_NBR: 1234567890
      │   ROUTING_NBR: 021000021
      │
      ├─ Step 3: EPX returns Financial BRIC (pre-note accepted)
      │
      ├─ Step 4: Convert to Storage BRIC (CKC8)
      │   TRAN_TYPE: CKC8
      │   ORIG_AUTH_GUID: [Financial BRIC]
      │
      ├─ Step 5: EPX returns Storage BRIC
      │
      └─ Step 6: Store in DB
          INSERT INTO customer_payment_methods (
            payment_token, payment_type, last_four,
            account_type, is_verified
          ) VALUES (
            '[Storage BRIC]', 'ach', '7890',
            'checking', true
          );

Phase 2: Verify Payment Method Response
  └─ Assertions:
      ✓ payment_method_id returned
      ✓ payment_type = PAYMENT_METHOD_TYPE_ACH
      ✓ last_four = '7890'
      ✓ account_type = 'checking'
      ✓ is_verified = true (pre-note passed)
      ✓ is_active = true

Phase 3: Verify Database Record
  ├─ RPC: PaymentMethodService.GetPaymentMethod
  └─ Assertions:
      ✓ payment_token = Storage BRIC (encrypted)
      ✓ Pre-note transaction created with type='pre_note'
      ✓ routing_number_hash matches
```

**Assertions**:
- ✓ Pre-note (CKC0) sent automatically
- ✓ Storage BRIC (CKC8) created from Financial BRIC
- ✓ Payment method saved with `is_verified = true`
- ✓ Last 4 digits extracted correctly
- ✓ Account type preserved

**NACHA Compliance Checks**:
- ✓ Pre-note sent before storage
- ✓ STD_ENTRY_CLASS preserved (PPD)
- ✓ Routing number validation

**Why Integration Test?**:
- Single actor (Service only)
- No auth chain complexity
- Tests EPX adapter CKC0/CKC8 flow
- Fits existing `payment_method_test.go` pattern

---

### Integration Test 2: ACH Debit with Verified Payment Method

**Integration Classification Reason**: Single-actor (Service) test focusing on EPX adapter CKC2 integration and transaction creation. Tests business logic that requires verified payment method.

**File**: `tests/integration/payment/ach_debit_test.go`

**RPC Used**: `PaymentService.ACHDebit`

**Purpose**: Validate ACH debit (CKC2) with verified Storage BRIC and transaction creation.

**Flow**:
```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: ACH Debit with Verified Payment Method     │
└──────────────────────────────────────────────────────────────┘

Phase 1: Setup - Create Verified ACH Payment Method
  └─ RPC: PaymentMethodService.StoreACHAccount (from Test 1)
      Returns: payment_method_id with is_verified=true

Phase 2: Process ACH Debit
  ├─ RPC: PaymentService.ACHDebit
  │   Body: {
  │     merchant_id: "test-merchant",
  │     customer_id: "test-customer",
  │     payment_method_id: "[From Phase 1]",
  │     amount: "150.00",
  │     currency: "USD"
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Validate payment method is_verified = true
      ├─ Step 2: Create transaction record (status='pending')
      ├─ Step 3: Send to EPX (CKC2)
      │   TRAN_TYPE: CKC2
      │   AMOUNT: 150.00
      │   AUTH_GUID: [Storage BRIC]
      │
      ├─ Step 4: EPX returns auth_resp='00' (accepted)
      └─ Step 5: Update transaction (status='approved')

Phase 3: Verify Transaction Response
  └─ Assertions:
      ✓ transaction_id returned
      ✓ type = TRANSACTION_TYPE_CHARGE
      ✓ status = TRANSACTION_STATUS_APPROVED
      ✓ amount = "150.00"
      ✓ payment_method_type = PAYMENT_METHOD_TYPE_ACH
      ✓ authorization_code present

Phase 4: Verify Database Record
  ├─ RPC: PaymentService.GetTransaction
  └─ Assertions:
      ✓ payment_method_id matches
      ✓ EPX tran_nbr stored
      ✓ auth_guid (Financial BRIC) stored
      ✓ Settlement status = pending (1-3 business days)
```

**Assertions**:
- ✓ ACH debit requires `is_verified = true`
- ✓ Transaction created with status='approved'
- ✓ EPX CKC2 sent with Storage BRIC
- ✓ Response contains auth_code
- ✓ Settlement pending status tracked

**Why Integration Test?**:
- Single actor (Service only)
- Tests EPX adapter CKC2 flow
- Tests business logic (verification requirement)
- No multi-actor workflow

---

### Integration Test 3: ACH Return Handling

**Integration Classification Reason**: Single-actor (EPX callback) test focusing on return code processing and payment method deactivation business logic. No auth chain testing.

**File**: `tests/integration/payment/ach_return_handling_test.go`

**Handler Used**: `BrowserPostCallbackHandler` (processes ACH returns)

**Purpose**: Validate ACH return code processing and automatic payment method deactivation after 2 returns.

**Flow**:
```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: ACH Return Codes → Auto-Deactivation       │
└──────────────────────────────────────────────────────────────┘

Phase 1: Setup - Create ACH Debit
  ├─ Create verified ACH payment method
  └─ Process ACH debit (from Test 2)
      Returns: transaction_id, tran_nbr

Phase 2: Simulate First ACH Return (R01 - Insufficient Funds)
  ├─ POST to callback handler with:
  │   {
  │     TRAN_NBR: "[Original debit tran_nbr]",
  │     RETURN_CODE: "R01",
  │     RETURN_DESC: "Insufficient Funds",
  │     EPX_MAC: "[Valid HMAC signature]"
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Verify EPX_MAC signature
      ├─ Step 2: Find transaction by tran_nbr
      ├─ Step 3: Update transaction status to 'returned'
      ├─ Step 4: Increment payment method return_count = 1
      ├─ Step 5: Keep is_active = true (< 2 returns)
      └─ Step 6: Create audit log entry

Phase 3: Verify First Return Processing
  ├─ RPC: PaymentService.GetTransaction
  └─ Assertions:
      ✓ status = 'returned'
      ✓ return_code = 'R01'
      ✓ return_description = 'Insufficient Funds'
  ├─ RPC: PaymentMethodService.GetPaymentMethod
  └─ Assertions:
      ✓ return_count = 1
      ✓ is_active = true (still active)

Phase 4: Simulate Second ACH Return (R03 - No Account)
  ├─ Create another ACH debit with same payment method
  ├─ POST to callback handler with:
  │   {
  │     TRAN_NBR: "[Second debit tran_nbr]",
  │     RETURN_CODE: "R03",
  │     RETURN_DESC: "No Account/Unable to Locate Account"
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Update transaction status to 'returned'
      ├─ Step 2: Increment payment method return_count = 2
      ├─ Step 3: **Auto-deactivate**: is_active = false
      └─ Step 4: Create audit log entry

Phase 5: Verify Auto-Deactivation
  ├─ RPC: PaymentMethodService.GetPaymentMethod
  └─ Assertions:
      ✓ return_count = 2
      ✓ is_active = false (auto-deactivated)
      ✓ deactivation_reason = 'excessive_returns'

Phase 6: Verify Deactivated Payment Method Cannot Be Used
  ├─ RPC: PaymentService.ACHDebit (with deactivated payment method)
  └─ Assertions:
      ✓ Error: "Payment method is inactive"
```

**Assertions**:
- ✓ Return codes R01-R05 processed correctly
- ✓ First return increments count, keeps active
- ✓ Second return auto-deactivates payment method
- ✓ Deactivated payment methods cannot process new debits
- ✓ Audit log created for each return

**Return Code Coverage**:
- R01: Insufficient Funds
- R02: Account Closed
- R03: No Account/Unable to Locate
- R04: Invalid Account Number
- R05: Unauthorized Debit

**Why Integration Test?**:
- Single actor (EPX callback only)
- Tests callback handler logic
- Tests business logic (auto-deactivation)
- Similar to existing `browser_post_callback_handler_test.go`

---

### Integration Test 4: Browser Post Save Card (Enhancement)

**Integration Classification Reason**: Single-actor (Customer via Browser Post) test focusing on Financial BRIC → Storage BRIC conversion and CCE8 integration. No multi-actor auth workflow.

**File**: `tests/integration/payment/browser_post_workflow_test.go` (enhancement to existing test)

**RPC Used**: `PaymentMethodService.ConvertFinancialBRICToStorageBRIC`

**New Test Function**: `TestIntegration_BrowserPost_SaveCard_Workflow`

**Purpose**: Validate Browser Post SALE with `save_card` flag triggers Financial→Storage BRIC conversion via CCE8 + Account Verification.

**Flow**:
```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: Browser Post SALE → Save Card → CCE8       │
└──────────────────────────────────────────────────────────────┘

Phase 1: Browser Post SALE with Save Card Flag
  ├─ Customer submits Browser Post form:
  │   TRAN_TYPE: CCE0 (SALE)
  │   TRAN_AMT: 29.99
  │   USER_DATA_2: "save_card=true"
  │   CALLBACK_URL: /webhooks/epx/callback
  │
  └─ EPX processes:
      ├─ Creates Financial BRIC (13-24 month expiry)
      ├─ Processes SALE transaction
      └─ Sends callback to server

Phase 2: Server Receives Callback with Financial BRIC
  ├─ POST /webhooks/epx/callback
  │   {
  │     TRAN_NBR: "1234567890",
  │     AUTH_GUID: "[Financial BRIC]",
  │     AUTH_RESP: "00",
  │     CARD_TYPE: "V",
  │     LAST_FOUR: "1111",
  │     USER_DATA_2: "save_card=true"  ← Trigger
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Verify EPX_MAC
      ├─ Step 2: Save transaction (Financial BRIC)
      ├─ Step 3: Detect USER_DATA_2="save_card=true"
      ├─ Step 4: Call ConvertFinancialBRICToStorageBRIC RPC
      │   {
      │     financial_bric: "[Financial BRIC]",
      │     transaction_id: "[SALE transaction ID]",
      │     payment_type: PAYMENT_METHOD_TYPE_CREDIT_CARD,
      │     last_four: "1111",
      │     card_brand: "visa"
      │   }

Phase 3: ConvertFinancialBRICToStorageBRIC Processing
  └─ Backend performs:
      ├─ Step 1: Send CCE8 to EPX
      │   TRAN_TYPE: CCE8
      │   ORIG_AUTH_GUID: [Financial BRIC]
      │   ACCOUNT_VERIFICATION: true
      │
      ├─ Step 2: EPX returns Storage BRIC (never expires)
      │
      └─ Step 3: Store in DB
          INSERT INTO customer_payment_methods (
            payment_token, payment_type, last_four,
            card_brand, is_verified
          ) VALUES (
            '[Storage BRIC]', 'credit_card', '1111',
            'visa', true
          );

Phase 4: Verify Payment Method Saved
  ├─ RPC: PaymentMethodService.GetPaymentMethod
  └─ Assertions:
      ✓ payment_type = PAYMENT_METHOD_TYPE_CREDIT_CARD
      ✓ last_four = '1111'
      ✓ card_brand = 'visa'
      ✓ is_verified = true (Account Verification passed)
      ✓ payment_token = Storage BRIC (encrypted)

Phase 5: Use Saved Card for Recurring Payment
  ├─ RPC: PaymentService.Sale
  │   {
  │     payment_method_id: "[From Phase 4]",
  │     amount: "49.99"
  │   }
  │
  └─ Backend performs:
      ├─ Query DB for Storage BRIC
      ├─ Send CCE0 with Storage BRIC
      └─ Process sale

Phase 6: Verify Recurring Payment
  └─ Assertions:
      ✓ Transaction created successfully
      ✓ Used Storage BRIC (not Financial BRIC)
      ✓ No card details in request
```

**Assertions**:
- ✓ `USER_DATA_2='save_card=true'` triggers conversion
- ✓ Financial BRIC → Storage BRIC via CCE8
- ✓ Account Verification performed
- ✓ Payment method saved for recurring use
- ✓ No card data stored (PCI compliance)
- ✓ Only Storage BRIC + last 4 digits stored

**PCI Compliance Checks**:
- ✓ No card number stored
- ✓ No CVV stored
- ✓ Only last 4 digits + brand stored
- ✓ Storage BRIC encrypted at rest

**Why Integration Test?**:
- Single actor (Customer via Browser Post)
- Tests callback handler + BRIC conversion
- Extension of existing Browser Post tests
- No multi-actor auth chain

---

### Integration Test 5: Direct Storage BRIC (Server Post)

**Integration Classification Reason**: Single-actor (Service) test focusing on EPX Server Post CCE8 integration for direct card tokenization. No multi-actor workflow.

**File**: `tests/integration/payment_method/credit_card_server_post_test.go`

**RPC Used**: `PaymentMethodService.SavePaymentMethod`

**New Test Function**: `TestStorePaymentMethod_CreditCard_ServerPost`

**Purpose**: Validate direct credit card tokenization via Server Post CCE8 (Storage BRIC) without Browser Post.

**Flow**:
```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: Raw Card → CCE8 → Storage BRIC (Direct)    │
└──────────────────────────────────────────────────────────────┘

Phase 1: Service Receives Raw Card Details
  ├─ (In production: from PCI-compliant form submission)
  ├─ Backend receives:
  │   card_number: "4111111111111111"
  │   exp_month: 12
  │   exp_year: 2025
  │   cvv: "123"
  │   billing_zip: "12345"
  │
  └─ Validate:
      ✓ Luhn checksum (card number)
      ✓ Expiry date (not expired)
      ✓ CVV format (3-4 digits)

Phase 2: Send to EPX Server Post (CCE8)
  └─ Backend performs:
      ├─ POST to EPX Server Post endpoint
      │   TRAN_TYPE: CCE8 (Storage BRIC)
      │   TRAN_AMT: 0.00 (Account Verification)
      │   CARD_NBR: 4111111111111111
      │   EXP_MONTH: 12
      │   EXP_YEAR: 2025
      │   CVV: 123
      │   BILLING_ZIP: 12345
      │   ACCOUNT_VERIFICATION: true
      │
      ├─ EPX validates card
      ├─ EPX performs $0 Account Verification
      └─ EPX returns Storage BRIC

Phase 3: Receive EPX Response
  └─ EPX returns:
      AUTH_GUID: [Storage BRIC - never expires]
      AUTH_RESP: "00" (approved)
      CARD_TYPE: "V" (Visa)
      LAST_FOUR: "1111"

Phase 4: Save Payment Method
  ├─ RPC: PaymentMethodService.SavePaymentMethod
  │   {
  │     payment_token: "[Storage BRIC]",
  │     payment_type: PAYMENT_METHOD_TYPE_CREDIT_CARD,
  │     last_four: "1111",
  │     card_brand: "visa",
  │     card_exp_month: 12,
  │     card_exp_year: 2025
  │   }
  │
  └─ Backend performs:
      ├─ Store in DB (NO card number, NO CVV)
      INSERT INTO customer_payment_methods (
        payment_token, payment_type, last_four,
        card_brand, card_exp_month, card_exp_year,
        is_verified, is_active
      ) VALUES (
        '[Storage BRIC]', 'credit_card', '1111',
        'visa', 12, 2025,
        true, true
      );

Phase 5: Verify Payment Method Saved
  ├─ RPC: PaymentMethodService.GetPaymentMethod
  └─ Assertions:
      ✓ payment_method_id returned
      ✓ payment_token = Storage BRIC (encrypted)
      ✓ last_four = '1111'
      ✓ card_brand = 'visa'
      ✓ card_exp_month = 12
      ✓ card_exp_year = 2025
      ✓ is_verified = true (Account Verification passed)
      ✓ is_active = true

Phase 6: Verify PCI Compliance
  └─ Database query assertions:
      ✓ card_number column does NOT exist
      ✓ cvv column does NOT exist
      ✓ Only payment_token (encrypted) stored
      ✓ Only last_four stored for display
```

**Assertions**:
- ✓ Luhn check validates card number
- ✓ CCE8 sent to EPX with raw card data
- ✓ Storage BRIC returned (never expires)
- ✓ Account Verification performed ($0 auth)
- ✓ Payment method saved successfully
- ✓ PCI compliance: No card/CVV stored

**PCI Compliance Checks**:
- ✓ Raw card data never stored in DB
- ✓ CVV never stored (even temporarily)
- ✓ Only Storage BRIC (NTID) stored
- ✓ Only last 4 digits for display
- ✓ Storage BRIC encrypted at rest

**Why Integration Test?**:
- Single actor (Service only)
- Tests EPX Server Post CCE8 flow
- Fits existing `payment_method_test.go` pattern
- No auth chain complexity

---

## Test Infrastructure

### Directory Structure

```
tests/
├── e2e/                                      # E2E tests (4 tests)
│   ├── README.md                             # E2E test documentation
│   ├── service_onboarding_test.go            # Test 1: Admin → Service auth
│   ├── token_delegation_customer_test.go     # Test 2: Service → Customer
│   ├── token_delegation_guest_test.go        # Test 3: Service → Guest
│   ├── multi_merchant_access_test.go         # Test 4: Multi-tenant isolation
│   └── helpers/
│       ├── admin_helpers.go                  # Admin operations (CreateService, etc.)
│       ├── service_helpers.go                # Service token generation
│       ├── token_helpers.go                  # Token generation utilities
│       ├── cleanup_helpers.go                # Test data cleanup
│       └── assertions.go                     # Custom assertion helpers
│
└── integration/                              # Integration tests (20+ tests)
    ├── payment/
    │   ├── browser_post_workflow_test.go     # Enhanced with save card test
    │   ├── ach_debit_test.go                 # NEW: ACH debit integration
    │   ├── ach_return_handling_test.go       # NEW: ACH return handling
    │   └── payment_service_critical_test.go  # Existing critical tests
    │
    └── payment_method/
        ├── ach_prenote_storage_test.go       # NEW: ACH pre-note integration
        ├── credit_card_server_post_test.go   # NEW: Direct Storage BRIC
        └── payment_method_test.go            # Existing payment method tests
```

### Test Helpers

**admin_helpers.go**:
```go
// AdminLogin authenticates admin and returns admin token
func AdminLogin(t *testing.T) string

// CreateService creates service via admin API and returns service details
func CreateService(t *testing.T, adminToken, serviceID, serviceName string) *CreateServiceResponse

// CreateMerchant creates merchant via admin API
func CreateMerchant(t *testing.T, adminToken, merchantSlug string) *Merchant

// GrantServiceAccess grants service access to merchant with scopes
func GrantServiceAccess(t *testing.T, adminToken, serviceID, merchantID string, scopes []string)

// RevokeServiceAccess revokes service access to merchant
func RevokeServiceAccess(t *testing.T, adminToken, serviceID, merchantID string)
```

**service_helpers.go**:
```go
// GenerateServiceToken creates RSA-signed service token
func GenerateServiceToken(t *testing.T, privateKey, serviceID string) string

// ValidateServiceToken verifies service token signature
func ValidateServiceToken(t *testing.T, token, publicKey string) jwt.MapClaims
```

**cleanup_helpers.go**:
```go
// CleanupService deletes service and all related data
func CleanupService(t *testing.T, serviceID string)

// CleanupMerchant deletes merchant and all related data
func CleanupMerchant(t *testing.T, merchantID string)

// CleanupTransactions deletes test transactions
func CleanupTransactions(t *testing.T, merchantID string)

// CleanupAll deletes all test data (use in defer)
func CleanupAll(t *testing.T, testData *TestData)
```

### Test Configuration

**test_config.yaml**:
```yaml
e2e:
  database_url: "postgres://postgres:postgres@localhost:5432/payments_test?sslmode=disable"
  service_url: "http://localhost:8081"
  epx_gateway_url: "https://secure.epxuap.com"

  admin:
    email: "admin@test.com"
    password: "test_admin_password"

  test_merchants:
    - slug: "test-merchant-001"
      name: "Test Merchant 001"
      cust_nbr: "9001"
      merch_nbr: "900300"
      dba_nbr: "2"
      terminal_nbr: "77"

  cleanup:
    enabled: true
    on_failure: keep  # keep test data if test fails for debugging
    on_success: delete
```

### Database Setup

**Before Each Test**:
```sql
-- Begin transaction for isolation
BEGIN;

-- Create test schema
CREATE SCHEMA IF NOT EXISTS e2e_test;
SET search_path TO e2e_test, public;

-- Run migrations
-- (use same migrations as main DB)
```

**After Each Test**:
```sql
-- Cleanup test data
DELETE FROM transactions WHERE merchant_id IN (SELECT id FROM merchants WHERE slug LIKE 'test-%');
DELETE FROM customer_payment_methods WHERE merchant_id IN (SELECT id FROM merchants WHERE slug LIKE 'test-%');
DELETE FROM service_merchants WHERE service_id IN (SELECT id FROM services WHERE service_id LIKE 'test-%');
DELETE FROM services WHERE service_id LIKE 'test-%';
DELETE FROM merchants WHERE slug LIKE 'test-%';

-- Rollback transaction
ROLLBACK;
```

---

## Implementation Guidelines

### Build Tags

All E2E tests use build tag `e2e`:

```go
//go:build e2e
// +build e2e

package e2e_test
```

Run with: `go test -tags=e2e ./tests/e2e/...`

### Test Execution Order

E2E tests should be runnable:
1. **Individually** - Each test is self-contained
2. **In parallel** - Tests don't interfere with each other
3. **In any order** - No dependencies between tests

### Test Data Isolation

Each test creates unique resources:
```go
func TestE2E_ServiceOnboarding(t *testing.T) {
    testID := uuid.New().String()[:8]  // Short UUID
    serviceID := fmt.Sprintf("test-service-%s", testID)
    merchantID := fmt.Sprintf("test-merchant-%s", testID)

    defer CleanupAll(t, &TestData{
        ServiceID:  serviceID,
        MerchantID: merchantID,
    })

    // Test implementation...
}
```

### Assertions

Use structured assertions:
```go
// Good: Clear, descriptive
assert.Equal(t, "approved", transaction.Status,
    "Transaction should be approved for valid card")

// Good: Multi-field validation
AssertPaymentMethod(t, pm, PaymentMethodExpectations{
    PaymentType: "credit_card",
    LastFour:    "1111",
    IsVerified:  true,
})

// Bad: Generic message
assert.True(t, pm.IsVerified)
```

### Error Handling

E2E tests should fail fast with clear messages:
```go
// Bad
if err != nil {
    t.Fatal(err)
}

// Good
if err != nil {
    t.Fatalf("Failed to create service: %v\nRequest: %+v", err, req)
}

// Better
require.NoError(t, err, "Service creation should succeed")
```

### Test Documentation

Each test file should have:
```go
/*
Package e2e_test contains end-to-end tests for the payment service.

Test: service_onboarding_test.go
Purpose: Validates complete service registration and authentication flow
Coverage: Admin creates service → Service authenticates → API call
Dependencies: PostgreSQL, Payment Service API

Setup Requirements:
- PostgreSQL running on localhost:5432
- Payment service running on localhost:8081
- Admin credentials in test config

Cleanup:
- Deletes test services
- Deletes test transactions
- Deletes test audit logs
*/
```

### Running E2E Tests

**Run all E2E tests**:
```bash
go test -tags=e2e -v ./tests/e2e/...
```

**Run specific test**:
```bash
go test -tags=e2e -v ./tests/e2e/ -run TestE2E_ServiceOnboarding
```

**Run with coverage**:
```bash
go test -tags=e2e -v -cover ./tests/e2e/...
```

**Run in CI/CD**:
```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests
on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Run migrations
        run: make migrate-up

      - name: Start payment service
        run: |
          make build
          ./bin/server &
          sleep 5

      - name: Run E2E tests
        run: go test -tags=e2e -v ./tests/e2e/...
```

---

## Success Criteria

### E2E Tests (4 tests)

- ✅ All 4 E2E tests implemented
- ✅ Tests pass consistently (< 1% flake rate)
- ✅ Test coverage >= 80% for auth flows
- ✅ Tests run in < 1 minute total
- ✅ Tests run nightly or pre-release
- ✅ Documentation complete and up-to-date
- ✅ Cleanup works (no test data pollution)
- ✅ Tests can run in parallel

### Integration Tests (5 new/enhanced tests)

- ✅ All 5 integration tests implemented
- ✅ Tests run on every commit
- ✅ Tests run in < 30 seconds total
- ✅ ACH and Storage BRIC flows fully covered
- ✅ EPX adapter functionality validated
- ✅ Business logic comprehensively tested

---

## Future Enhancements

### Phase 2 Additions

1. **Subscription E2E Tests**
   - CreateSubscription → Recurring billing → Cancel
   - Failed recurring payment handling
   - Subscription upgrade/downgrade flows

2. **Chargeback E2E Tests**
   - Chargeback notification → Dispute → Resolution
   - Chargeback impact on merchant account

3. **Multi-Currency E2E Tests**
   - Payment processing in different currencies
   - Currency conversion accuracy

4. **Webhook E2E Tests**
   - Webhook delivery and retry logic
   - Webhook signature verification
   - Webhook idempotency

---

## References

- [Authentication Guide](./AUTHENTICATION.md) - Complete auth documentation
- [API Design and Dataflow](./API_DESIGN_AND_DATAFLOW.md) - API specifications
- [ACH Business Logic](./ACH_BUSINESS_LOGIC.md) - ACH flow documentation
- [Credit Card Business Logic](./CREDIT_CARD_BUSINESS_LOGIC.md) - Credit card flows
- [Keypair Auto-Generation](./auth/keypair-auto-generation.md) - RSA keypair details

---

**Last Updated**: 2025-11-19
**Maintained By**: Backend Team
**Review Cycle**: Quarterly or with major auth changes
