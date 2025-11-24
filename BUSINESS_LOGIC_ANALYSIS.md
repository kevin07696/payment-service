# Business Logic Analysis: Payment Service

**Generated:** 2025-11-23
**Project:** Payment Service (EPX Gateway Integration)
**Analysis Scope:** Comprehensive business logic implementation for all database tables

---

## Executive Summary

This document provides a comprehensive analysis of the business logic implementation across all database entities in the payment service. The service demonstrates **strong domain-driven design principles** with well-layered architecture (domain models, service layer, handlers), comprehensive validation, and robust error handling.

### Key Strengths
- ✅ **Strong domain models** with business methods and state validation
- ✅ **Comprehensive error handling** with structured error codes
- ✅ **Idempotency support** for critical operations
- ✅ **Transaction safety** with database transaction management
- ✅ **Multi-tenant architecture** with proper isolation
- ✅ **ACH verification workflow** with automated cron jobs
- ✅ **Subscription billing** with retry logic and state management
- ✅ **Performance optimizations** (caching, connection pooling)

### Areas for Improvement
- ⚠️ Some validation rules exist only in domain layer (missing database constraints)
- ⚠️ Chargeback entity has read-only business logic (pending integration)
- ⚠️ Limited webhook delivery retry logic visibility
- ⚠️ Some edge cases in concurrent transaction state transitions

---

## Table of Contents

1. [Merchants Table](#1-merchants-table)
2. [Customer Payment Methods Table](#2-customer-payment-methods-table)
3. [Transactions Table](#3-transactions-table)
4. [Subscriptions Table](#4-subscriptions-table)
5. [Chargebacks Table](#5-chargebacks-table)
6. [Webhook Tables](#6-webhook-tables)
7. [Authentication Tables](#7-authentication-tables)
8. [Cross-Cutting Concerns](#8-cross-cutting-concerns)
9. [Recommendations](#9-recommendations)

---

## 1. Merchants Table

**Database Location:** `/internal/db/migrations/001_merchants.sql`
**Domain Model:** `/internal/domain/merchant.go`
**Service Layer:** `/internal/services/merchant/merchant_service.go`
**Handlers:** `/internal/handlers/merchant/`

### 1.1 Service Layer Implementation

**Location:** `/internal/services/merchant/merchant_service.go`

**Business Logic Methods:**
- **Credential Management:** MAC secret resolution with caching (70% DB load reduction)
- **Multi-environment Support:** Sandbox vs Production environment handling
- **Active Status Checks:** Validates merchant can process transactions

**Key Features:**
```go
// Domain business methods
func (m *Merchant) CanProcessTransactions() bool {
    return m.IsActive
}

func (m *Merchant) IsSandbox() bool {
    return m.Environment == EnvironmentSandbox
}

func (m *Merchant) Deactivate() {
    m.IsActive = false
    m.UpdatedAt = timeutil.Now()
}
```

**Credential Caching:**
- **Implementation:** `MerchantCredentialCache` with TTL-based invalidation
- **Performance Impact:** 70% reduction in database queries
- **Cache Key:** Merchant ID
- **TTL:** Configurable (production: 5 minutes)

### 1.2 Domain Model

**State Validation:**
- ✅ Environment validation (sandbox/production)
- ✅ Active status checks before transaction processing
- ✅ MAC secret path validation

**Business Methods:**
- `CanProcessTransactions()` - Validates merchant is active
- `GetMACSecretPath()` - Returns secret manager path
- `Activate()/Deactivate()` - State transitions with timestamp updates

### 1.3 Handler Layer

**Authentication:** JWT-based with service-to-merchant authorization
**Endpoints:**
- `GET /merchants/{id}` - Retrieve merchant details
- Credentials secured via Secret Manager (never exposed in API)

**Authorization:**
- Services must be linked to merchants via `service_merchants` table
- Scoped permissions per service-merchant relationship

### 1.4 Business Rules

**Invariants Enforced:**
1. ✅ Merchant must be active to process transactions (checked in payment service)
2. ✅ MAC secrets stored in Secret Manager (never in database)
3. ✅ Slug uniqueness enforced (database constraint)
4. ✅ Soft delete pattern (deleted_at timestamp)

**Integration Points:**
- **EPX Gateway:** Credentials (cust_nbr, merch_nbr, dba_nbr, terminal_nbr)
- **Secret Manager:** MAC secret retrieval for request signing
- **Multi-tenant Isolation:** All transactions/payment methods scoped to merchant_id

### 1.5 Data Integrity

**Foreign Key Handling:**
- ✅ `ON DELETE RESTRICT` on merchant references in transactions/subscriptions
- ✅ Prevents deletion of merchants with active data

**Soft Delete:**
- ✅ Implemented via `deleted_at` timestamp
- ✅ Partial indexes exclude soft-deleted records

**Audit Trail:**
- ✅ `created_at`, `updated_at` timestamps
- ✅ `created_by`, `approved_by` admin references (migration 008)

### 1.6 Edge Cases & Error Handling

**Error Scenarios:**
| Scenario | Handling | Error Type |
|----------|----------|------------|
| Merchant not found | Return `ErrMerchantNotFound` | Domain Error |
| Merchant inactive | Return `ErrMerchantInactive` | Domain Error |
| Invalid environment | Validate on creation | Validation Error |
| MAC secret retrieval failure | Fail transaction with error | Gateway Error |

**Missing Validations:**
- ⚠️ No business rule preventing deletion of merchants with active subscriptions (database prevents via FK constraint)
- ⚠️ No workflow for merchant reactivation after deactivation

---

## 2. Customer Payment Methods Table

**Database Location:** `/internal/db/migrations/002_customer_payment_methods.sql`, `009_ach_verification_enhancements.sql`
**Domain Model:** `/internal/domain/payment_method.go`
**Service Layer:** `/internal/services/payment_method/payment_method_service.go`
**Handlers:** `/internal/handlers/payment_method/`
**Cron Jobs:** `/internal/handlers/cron/ach_verification_handler.go`

### 2.1 Service Layer Implementation

**Business Logic Methods:**
```go
// Core CRUD
- GetPaymentMethod(ctx, paymentMethodID) - Cached retrieval (60% faster)
- ListPaymentMethods(ctx, merchantID, customerID)
- UpdatePaymentMethodStatus(ctx, paymentMethodID, isActive)
- DeletePaymentMethod(ctx, paymentMethodID) - Soft delete
- SetDefaultPaymentMethod(ctx, paymentMethodID)

// ACH-specific
- StoreACHAccount(ctx, req) - Creates ACH with pre-note verification
- VerifyACHAccount(ctx, paymentMethodID) - Manual verification trigger
```

**ACH Pre-Note Workflow:**
```
1. Client calls StoreACHAccount with account details
2. Service sends EPX pre-note (CKC0 for checking, CKS0 for savings)
3. EPX returns AUTH_GUID (BRIC token)
4. Service stores payment method with:
   - verification_status = 'pending'
   - is_active = false
   - is_verified = false
   - prenote_transaction_id = transaction UUID
5. Cron job runs daily, marks accounts as verified after 3 days
6. Once verified:
   - verification_status = 'verified'
   - is_active = true
   - is_verified = true
   - verified_at = timestamp
```

**Cron Job (ACH Verification):**
- **Frequency:** Daily (configurable via Cloud Scheduler)
- **Logic:** Finds ACH accounts pending > 3 days, marks as verified
- **Query:** `FindEligibleACHForVerification` (SQLC)
- **Batch Size:** 100 (configurable)
- **Idempotency:** Update query checks verification_status='pending'

### 2.2 Domain Model

**Payment Method Types:**
```go
const (
    PaymentMethodTypeCreditCard = "credit_card"
    PaymentMethodTypeACH        = "ach"
)
```

**Verification Status (ACH only):**
```go
const (
    VerificationStatusPending  = "pending"   // Pre-note sent, awaiting clearance
    VerificationStatusVerified = "verified"  // Cleared after 3 days
    VerificationStatusFailed   = "failed"    // Return code received
)
```

**Business Methods:**
```go
// Validation
func (pm *PaymentMethod) CanUseForAmount(amountCents int64) (bool, string) {
    // 1. Check active status FIRST
    if !pm.IsActive {
        return false, "payment method is not active"
    }

    // 2. Credit card expiration
    if pm.IsCreditCard() && pm.IsExpired() {
        return false, "credit card is expired"
    }

    // 3. ACH verification (NO GRACE PERIOD)
    if pm.IsACH() && !pm.IsVerified {
        return false, "ACH account must be verified before use"
    }

    return true, ""
}

// Display
func (pm *PaymentMethod) GetDisplayName() string {
    // Credit card: "Visa •••• 4242"
    // ACH: "Chase Checking •••• 1234"
}

// State checks
func (pm *PaymentMethod) IsExpired() bool {
    // Checks card_exp_month/card_exp_year
}
```

### 2.3 Handler Layer

**API Endpoints:**
- `POST /payment-methods/tokenize` - Browser Post tokenization (credit card)
- `POST /payment-methods/ach` - Store ACH account (Server Post with pre-note)
- `GET /payment-methods` - List customer's payment methods
- `GET /payment-methods/{id}` - Get specific payment method
- `PATCH /payment-methods/{id}/status` - Activate/deactivate
- `POST /payment-methods/{id}/set-default` - Set as default
- `DELETE /payment-methods/{id}` - Soft delete

**JWT Authentication:**
- ✅ Service must have permission to merchant
- ✅ Customer ID validated from JWT claims

**Input Validation:**
- ✅ Payment method ID format (UUID)
- ✅ Merchant/customer ownership checks
- ✅ ACH account type (CHECKING/SAVINGS)
- ✅ Credit card expiration format

### 2.4 Business Rules

**Invariants Enforced:**

| Rule | Enforcement Layer | Implementation |
|------|-------------------|----------------|
| ACH must be verified before use | Domain + Service | `CanUseForAmount()` checks `is_verified` |
| Credit cards auto-verified | Database Migration | Migration 009 sets status='verified' for CC |
| Only one default per customer | Database + Service | `SetPaymentMethodAsDefault` unsets others in transaction |
| BRIC token uniqueness | Database Constraint | `UNIQUE (merchant_id, customer_id, bric)` |
| Return count auto-deactivation | Database Query | `IncrementReturnCount` deactivates at threshold (2+) |
| Pre-note required for ACH | Service Layer | `StoreACHAccount` sends CKC0/CKS0 transaction |

**State Transitions:**

```
ACH Payment Method Lifecycle:
[Created] → verification_status='pending', is_active=false, is_verified=false
    ↓ (3 days + cron job)
[Verified] → verification_status='verified', is_active=true, is_verified=true
    OR
[Failed] → verification_status='failed', is_active=false (if return code received)

[Active] ↔ [Inactive] (user action via UpdatePaymentMethodStatus)

[Active] → [Deactivated] (excessive returns: return_count >= 2)
```

**Business Workflows:**

1. **ACH Verification (3-day pre-note):**
   - Day 0: Pre-note sent to EPX
   - Day 0-3: Status = 'pending', cannot be used for transactions
   - Day 3+: Cron job marks as verified, activates account
   - If return code: Mark as 'failed', deactivate

2. **Credit Card Tokenization:**
   - Browser Post: EPX returns BRIC via callback
   - Service stores with status='verified' (no verification needed)
   - Immediately usable for transactions

3. **Return Code Handling:**
   - EPX returns ACH return code (e.g., R03: No Account)
   - Service increments `return_count`
   - If count >= 2: Auto-deactivate with reason='excessive_returns'

### 2.5 Data Integrity

**Foreign Key Constraints:**
```sql
merchant_id UUID REFERENCES merchants(id) ON DELETE RESTRICT
prenote_transaction_id UUID REFERENCES transactions(id) [implied]
```

**Unique Constraints:**
```sql
UNIQUE (merchant_id, customer_id, bric)
```

**Check Constraints:**
```sql
CHECK (payment_type IN ('credit_card', 'ach'))
CHECK (account_type IN ('checking', 'savings'))
CHECK (verification_status IN ('pending', 'verified', 'failed'))
CHECK (card_exp_month >= 1 AND card_exp_month <= 12)
CHECK (return_count >= 0)
```

**Soft Delete:**
- ✅ `deleted_at TIMESTAMPTZ`
- ✅ All queries filter `WHERE deleted_at IS NULL`
- ✅ Partial indexes for performance

### 2.6 Edge Cases & Error Handling

**Comprehensive Error Scenarios:**

| Scenario | Current Handling | Gap/Issue |
|----------|------------------|-----------|
| Use unverified ACH | ✅ Domain validation blocks | None - well handled |
| Use expired credit card | ✅ `IsExpired()` check | None |
| Use inactive payment method | ✅ Active status checked first | None |
| Concurrent default setting | ✅ Database transaction | None - transaction isolation |
| ACH return after verification | ✅ Increment return_count, auto-deactivate | Could add webhook notification |
| Pre-note EPX failure | ✅ Return error, don't save PM | None |
| Cron job failure (verification) | ✅ Logs errors, continues batch | No dead-letter queue for retries |
| Payment method deleted mid-transaction | ✅ FK allows NULL on transactions | None - transaction retains PM ID |

**Missing Validations:**
- ⚠️ No rate limiting on failed pre-note attempts (could prevent abuse)
- ⚠️ No notification when ACH verification fails (customer unaware)
- ⚠️ No automated cleanup of permanently failed ACH accounts

**Race Conditions:**
- ✅ **Handled:** Concurrent default setting (database transaction)
- ✅ **Handled:** Cron verification race (query checks pending status)
- ⚠️ **Potential:** Concurrent return code processing (no explicit locking)

---

## 3. Transactions Table

**Database Location:** `/internal/db/migrations/003_transactions.sql`
**Domain Model:** `/internal/domain/transaction.go`
**Service Layer:** `/internal/services/payment/payment_service.go`
**State Computation:** `/internal/services/payment/group_state.go`
**Handlers:** `/internal/handlers/payment/`

### 3.1 Service Layer Implementation

**Transaction Types Supported:**
```go
const (
    TransactionTypeAuth    = "AUTH"     // Authorization only (hold funds)
    TransactionTypeCapture = "CAPTURE"  // Capture authorized funds
    TransactionTypeSale    = "SALE"     // Combined auth + capture
    TransactionTypeRefund  = "REFUND"   // Return funds
    TransactionTypeVoid    = "VOID"     // Cancel transaction
    TransactionTypePreNote = "PRE_NOTE" // ACH verification ($0)
    TransactionTypeStorage = "STORAGE"  // Tokenization
)
```

**Business Logic Methods:**
```go
// Payment Operations
- Sale(ctx, req) - AUTH + CAPTURE combined (credit card, ACH)
- Authorize(ctx, req) - Authorization only (credit card only)
- Capture(ctx, req) - Capture authorized funds
- Void(ctx, req) - Cancel AUTH or SALE before settlement
- Refund(ctx, req) - Return funds from SALE/CAPTURE

// Retrieval
- GetTransaction(ctx, transactionID)
- GetTransactionByIdempotencyKey(ctx, key)
- ListTransactions(ctx, filters)
- GetTransactionsByGroup(ctx, parentTransactionID)
```

**Idempotency Implementation:**
```
Key Strategy: idempotency_key IS the transaction ID (UUID)

Benefits:
✅ No separate idempotency table
✅ Natural deduplication via primary key
✅ Deterministic TRAN_NBR generation (FNV-1a hash of UUID)
✅ Safe retries of EPX calls

Flow:
1. Client provides idempotency_key (UUID)
2. Service parses as transaction ID
3. Check if transaction exists (SELECT by ID)
4. If exists and complete (auth_resp set): return existing
5. If exists and pending: retry EPX call (same TRAN_NBR)
6. If not exists: create pending transaction, call EPX
```

**State Computation (WAL-based):**

**File:** `/internal/services/payment/group_state.go`

```go
type GroupState struct {
    // Active authorization tracking
    ActiveAuthID     *string // Current AUTH transaction ID
    ActiveAuthBRIC   string  // BRIC token for operations
    ActiveAuthAmount int64   // Original auth amount

    // Amounts tracked
    CapturedAmount   int64   // Total captured
    RefundedAmount   int64   // Total refunded
    VoidedAuthAmount int64   // Amount voided

    // State flags
    IsVoided     bool
    AllRefunded  bool
}

func ComputeGroupState(txs []*domain.Transaction) *GroupState {
    // WAL-based state computation:
    // 1. Find most recent AUTH (approved)
    // 2. Sum CAPTUREs
    // 3. Sum REFUNDs
    // 4. Check for VOIDs
    // 5. Compute remaining balances
}

func (s *GroupState) CanCapture(amount int64) (bool, string) {
    // Validation logic:
    // - Must have active AUTH
    // - AUTH must not be voided
    // - Amount <= (ActiveAuthAmount - CapturedAmount)
}

func (s *GroupState) CanRefund(amount int64) (bool, string) {
    // - Must have captures
    // - Amount <= (CapturedAmount - RefundedAmount)
}

func (s *GroupState) CanVoid() (bool, string) {
    // - Must have active AUTH
    // - AUTH must not be voided
    // - No captures yet (or full amount available)
}
```

**Why WAL-based State?**
- ✅ **Immutable Event Log:** Transactions never updated (except EPX response)
- ✅ **Auditability:** Full transaction history preserved
- ✅ **Consistency:** State computed from authoritative source (database)
- ✅ **Concurrency Safety:** No race conditions on state updates

### 3.2 Domain Model

**Transaction Status (Database Generated):**
```go
// Auto-computed from auth_resp field
const (
    TransactionStatusApproved = "approved" // auth_resp='00'
    TransactionStatusDeclined = "declined" // auth_resp != '00'
    // Implied states:
    // "pending" - auth_resp IS NULL, processed_at IS NULL
    // "failed"  - auth_resp IS NULL, processed_at IS NOT NULL
)
```

**Business Methods:**
```go
func (t *Transaction) IsApproved() bool {
    return t.AuthResp != nil && *t.AuthResp == "00"
}

func (t *Transaction) CanBeVoided() bool {
    return t.Status == TransactionStatusApproved &&
        (t.Type == TransactionTypeAuth || t.Type == TransactionTypeSale)
}

func (t *Transaction) CanBeCaptured() bool {
    return t.Status == TransactionStatusApproved &&
        t.Type == TransactionTypeAuth
}

func (t *Transaction) CanBeRefunded() bool {
    return t.Status == TransactionStatusApproved &&
        (t.Type == TransactionTypeSale || t.Type == TransactionTypeCapture)
}
```

### 3.3 Handler Layer

**API Endpoints:**
- `POST /payments/sale` - Sale (auth + capture)
- `POST /payments/authorize` - Authorization only
- `POST /payments/{id}/capture` - Capture authorization
- `POST /payments/{id}/void` - Void transaction
- `POST /payments/{id}/refund` - Refund payment
- `GET /payments/{id}` - Get transaction details
- `GET /payments` - List transactions (filtered)

**Request Validation:**
```go
// Sale/Authorize
- amount_cents > 0
- currency (default: USD)
- payment_method_id OR payment_token (required)
- idempotency_key (UUID, required)
- merchant_id (from JWT or explicit)
- customer_id (optional)

// Capture
- transaction_id (parent AUTH)
- amount_cents (optional, default: full amount)
- idempotency_key (optional)

// Void
- parent_transaction_id (required)
- idempotency_key (optional)

// Refund
- parent_transaction_id (required)
- amount_cents (optional, default: full amount)
- reason (optional)
- idempotency_key (optional)
```

**Authorization:**
- ✅ JWT-based service authentication
- ✅ Merchant ID validation (service must have access to merchant)
- ✅ Transaction access validation (transaction belongs to merchant)

### 3.4 Business Rules

**Invariants Enforced:**

| Rule | Enforcement | Implementation |
|------|-------------|----------------|
| Standalone types have no parent | Database Constraint | `CHECK (type IN ('SALE', 'AUTH', 'STORAGE', 'DEBIT') AND parent_transaction_id IS NULL)` |
| Dependent types require parent | Database Constraint | `CHECK (type IN ('CAPTURE', 'REFUND', 'VOID') AND parent_transaction_id IS NOT NULL)` |
| Amount must be non-negative | Database Constraint | `CHECK (amount_cents >= 0)` |
| Capture amount <= AUTH amount | Service Layer | `GroupState.CanCapture()` validates |
| Refund amount <= Captured amount | Service Layer | `GroupState.CanRefund()` validates |
| Only approved txns can be captured/refunded | Service Layer | Domain model checks `IsApproved()` |
| ACH must be verified | Service Layer | `resolvePaymentToken()` validates |

**State Transitions:**

```
Transaction Lifecycle (AUTH → CAPTURE flow):

[AUTH Created] → status='pending', auth_resp=NULL
    ↓ (EPX response)
[AUTH Approved] → status='approved', auth_resp='00'
    ↓ (Capture request)
[CAPTURE Created] → parent_transaction_id=AUTH.id, status='pending'
    ↓ (EPX response)
[CAPTURE Approved] → status='approved', auth_resp='00'
    ↓ (Refund request)
[REFUND Created] → parent_transaction_id=AUTH.id, status='pending'
    ↓ (EPX response)
[REFUND Approved] → status='approved', auth_resp='00'

Alternative flows:
[AUTH Approved] → [VOID] (cancel before capture)
[SALE Approved] → [REFUND] (direct refund, no separate auth)
```

**Business Workflows:**

1. **Sale Workflow (Credit Card / ACH):**
   ```
   Client Request → Validate PM → Call EPX (CCE1 or CKC2)
   → Save Transaction → Return Response
   ```

2. **Auth → Capture Workflow:**
   ```
   AUTH Request → EPX (AUTH) → Save AUTH
   → (later) CAPTURE Request → Validate State → EPX (CAPTURE)
   → Save CAPTURE → Return
   ```

3. **Refund Workflow:**
   ```
   REFUND Request → Get Transaction Tree → Compute State
   → Validate Refund Amount → EPX (REFUND)
   → Save REFUND → Return
   ```

4. **Idempotency Workflow:**
   ```
   Request with idempotency_key → Check if exists
   → If complete: return existing
   → If pending: retry EPX (same TRAN_NBR)
   → If not exists: create and process
   ```

### 3.5 Data Integrity

**Foreign Key Constraints:**
```sql
parent_transaction_id UUID REFERENCES transactions(id) ON DELETE RESTRICT
merchant_id UUID REFERENCES merchants(id) ON DELETE RESTRICT
payment_method_id UUID REFERENCES customer_payment_methods(id) ON DELETE SET NULL
subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL
```

**Unique Constraints:**
- Primary Key: `id` (client-provided idempotency key)

**Generated Columns:**
```sql
status VARCHAR(20) GENERATED ALWAYS AS (
    CASE
        WHEN auth_resp IS NULL AND processed_at IS NULL THEN 'pending'
        WHEN auth_resp IS NULL AND processed_at IS NOT NULL THEN 'failed'
        WHEN auth_resp = '00' THEN 'approved'
        ELSE 'declined'
    END
) STORED
```

**Indexes:**
```sql
-- Performance critical
idx_transactions_parent_id (parent_transaction_id)
idx_transactions_merchant_id (merchant_id)
idx_transactions_merchant_customer (merchant_id, customer_id)
idx_transactions_tran_nbr (tran_nbr) -- EPX reconciliation
idx_transactions_auth_guid (auth_guid) -- BRIC lookups
idx_transactions_created_at (created_at DESC) -- Listing
idx_transactions_status (status) -- Filtering
```

### 3.6 Edge Cases & Error Handling

**Comprehensive Error Scenarios:**

| Scenario | Handling | Robustness |
|----------|----------|------------|
| **Idempotency: Same key, same request** | ✅ Return existing transaction | Perfect |
| **Idempotency: Same key, different request** | ✅ Return existing (ignore new params) | Correct (idempotency wins) |
| **Concurrent CAPTURE on same AUTH** | ✅ WAL-based state computation | ⚠️ Potential race if no locking |
| **EPX timeout during CAPTURE** | ✅ Transaction stays pending, retry safe | Perfect (TRAN_NBR deterministic) |
| **Capture amount > AUTH amount** | ✅ GroupState validation blocks | Perfect |
| **Refund after partial captures** | ✅ Computes captured - refunded | Perfect |
| **Void after partial capture** | ✅ Validation blocks (cannot void) | Perfect |
| **Payment method deleted mid-transaction** | ✅ FK SET NULL preserves transaction | Good |
| **BRIC token rotation** | ✅ Each transaction stores own auth_guid | Perfect |
| **Parent transaction not found** | ✅ GetTransactionByID fails → error | Good |
| **Parent transaction declined** | ✅ CanCapture checks IsApproved | Perfect |

**Race Condition Analysis:**

**Scenario 1: Concurrent CAPTUREs on same AUTH**
```
Thread A: CAPTURE $50 → Read tree → Compute state (available: $100) → Call EPX
Thread B: CAPTURE $60 → Read tree → Compute state (available: $100) → Call EPX

Problem: Both see $100 available, could double-capture

Current Mitigation:
- Database transaction isolates CREATE transaction
- EPX may reject duplicate TRAN_NBR
- Idempotency key must be unique per operation

Recommendation:
⚠️ Add row-level locking on parent transaction:
   SELECT ... FROM transactions WHERE id = $1 FOR UPDATE
```

**Scenario 2: Concurrent REFUND + VOID**
```
Thread A: REFUND → Reads state → Processes
Thread B: VOID → Reads state → Processes

Current: Both operations read same state, could conflict

Mitigation:
- EPX enforces business rules (cannot void captured transaction)
- Service layer validates state before EPX call
```

**Missing Validations:**
- ⚠️ No rate limiting on transaction creation (DoS prevention)
- ⚠️ No maximum refund count per transaction
- ⚠️ No transaction aging/archival strategy

**Error Handling Patterns:**

```go
// Payment Service Error Wrapping
if err := s.serverPost.ProcessTransaction(ctx, epxReq); err != nil {
    s.logger.Error("EPX transaction failed", zap.Error(err))
    return nil, fmt.Errorf("gateway error: %w", err)
}

// Domain Error Usage
if !merchant.IsActive {
    return nil, domain.ErrMerchantInactive
}

// Validation Errors
if req.AmountCents <= 0 {
    return nil, fmt.Errorf("amount must be greater than zero")
}
```

---

## 4. Subscriptions Table

**Database Location:** `/internal/db/migrations/003_transactions.sql` (lines 104-157)
**Domain Model:** `/internal/domain/subscription.go`
**Service Layer:** `/internal/services/subscription/subscription_service.go`
**Handlers:** `/internal/handlers/subscription/`, `/internal/handlers/cron/billing_handler.go`

### 4.1 Service Layer Implementation

**Business Logic Methods:**
```go
// CRUD Operations
- CreateSubscription(ctx, req) - Create recurring billing
- UpdateSubscription(ctx, req) - Modify amount/interval/payment method
- CancelSubscription(ctx, req) - Cancel immediately or at period end
- PauseSubscription(ctx, subscriptionID) - Temporarily halt billing
- ResumeSubscription(ctx, subscriptionID) - Restart paused subscription
- GetSubscription(ctx, subscriptionID)
- ListCustomerSubscriptions(ctx, merchantID, customerID)

// Billing Operations
- ProcessDueBilling(ctx, asOfDate, batchSize) - Cron job entry point
- processSubscriptionBilling(ctx, sub) - Single subscription billing
- handleBillingFailure(ctx, sub, err) - Retry logic
```

**Subscription Billing Workflow:**

```
Cron Trigger (Daily):
1. Find subscriptions where next_billing_date <= TODAY and status='active'
2. For each subscription:
   a. Generate deterministic transaction ID:
      txID = SHA1(subscription_id + next_billing_date)
   b. Check if already charged (idempotency):
      If transaction exists → skip, update next_billing_date
   c. Get payment method, validate is active/verified
   d. Call EPX with:
      - TransactionType: SALE (CCE1 or CKC2)
      - OriginalAuthGUID: payment_method.bric
      - ACIExt: "RB" (Recurring Billing indicator)
      - CardEntryMethod: "Z" (stored credential)
      - IndustryType: "E" (E-commerce)
   e. If approved:
      - Save transaction with subscription_id link
      - Calculate next_billing_date (interval + current date)
      - Reset failure_retry_count = 0
      - Set status = 'active'
   f. If declined/failed:
      - Increment failure_retry_count
      - If count >= max_retries: status = 'past_due'
      - Else: status = 'active' (will retry next run)

Deterministic Transaction ID:
- Ensures idempotency across cron runs
- Format: SHA1(sub_id + billing_date)
- Same billing cycle = same transaction ID
- Prevents duplicate charges
```

**Retry Logic:**
```go
func (s *subscriptionService) handleBillingFailure(ctx, sub, billingErr) error {
    newRetryCount := sub.FailureRetryCount + 1

    if newRetryCount >= sub.MaxRetries {
        // Max retries reached
        status = "past_due"
        // TODO: Send webhook notification
    } else {
        // Still have retries
        status = "active" // Will retry on next cron run
    }

    // Update subscription
    UPDATE subscriptions SET
        failure_retry_count = newRetryCount,
        status = status
    WHERE id = sub.id

    return billingErr // Propagate error for logging
}
```

### 4.2 Domain Model

**Subscription Status:**
```go
const (
    SubscriptionStatusActive    = "active"    // Billing normally
    SubscriptionStatusPaused    = "paused"    // User paused (no billing)
    SubscriptionStatusPastDue   = "past_due"  // Max retries exceeded
    SubscriptionStatusCancelled = "cancelled" // Permanently cancelled
)
```

**Interval Units:**
```go
const (
    IntervalUnitDay   = "day"
    IntervalUnitWeek  = "week"
    IntervalUnitMonth = "month"
    IntervalUnitYear  = "year"
)
```

**Business Methods:**
```go
func (s *Subscription) IsActive() bool {
    return s.Status == SubscriptionStatusActive
}

func (s *Subscription) CanBeBilled() bool {
    return s.IsActive() && timeutil.Now().After(s.NextBillingDate)
}

func (s *Subscription) ShouldRetry() bool {
    return s.FailureRetryCount < s.MaxRetries
}

func (s *Subscription) IncrementRetryCount() {
    s.FailureRetryCount++
    if s.FailureRetryCount >= s.MaxRetries {
        s.Status = SubscriptionStatusPastDue
    }
}

func (s *Subscription) CalculateNextBillingDate() time.Time {
    switch s.IntervalUnit {
    case IntervalUnitDay:
        return s.NextBillingDate.AddDate(0, 0, s.IntervalValue)
    case IntervalUnitWeek:
        return s.NextBillingDate.AddDate(0, 0, s.IntervalValue*7)
    case IntervalUnitMonth:
        return s.NextBillingDate.AddDate(0, s.IntervalValue, 0)
    case IntervalUnitYear:
        return s.NextBillingDate.AddDate(s.IntervalValue, 0, 0)
    }
}
```

### 4.3 Handler Layer

**API Endpoints:**
- `POST /subscriptions` - Create subscription
- `GET /subscriptions/{id}` - Get subscription details
- `GET /subscriptions` - List customer subscriptions
- `PATCH /subscriptions/{id}` - Update subscription
- `POST /subscriptions/{id}/cancel` - Cancel subscription
- `POST /subscriptions/{id}/pause` - Pause subscription
- `POST /subscriptions/{id}/resume` - Resume subscription

**Cron Endpoint:**
- `POST /cron/process-billing` - Trigger billing process
  - Authentication: X-Cron-Secret header
  - Body: `{ "as_of_date": "2025-11-23", "batch_size": 100 }`
  - Returns: `{ "processed": N, "success": M, "failed": K, "errors": [...] }`

**Request Validation:**
```go
// Create Subscription
- merchant_id (required)
- customer_id (required)
- payment_method_id (required, must be active)
- amount_cents > 0
- interval_value > 0
- interval_unit IN ('day', 'week', 'month', 'year')
- max_retries (default: 3)
- start_date (default: today)

// Update Subscription
- Can only update active/past_due subscriptions
- New payment method must belong to same customer
- Amount must be > 0 if provided

// Cancel Subscription
- cancel_at_period_end (bool)
  - true: mark for cancellation, still active until next billing
  - false: cancel immediately, status='cancelled'
```

### 4.4 Business Rules

**Invariants Enforced:**

| Rule | Enforcement | Implementation |
|------|-------------|----------------|
| Amount must be positive | Database + Service | `CHECK (amount_cents > 0)` |
| Interval value must be positive | Database + Service | `CHECK (interval_value > 0)` |
| Payment method must be active | Service Layer | Validated on create/update |
| Cannot delete payment method with active subs | Database | `ON DELETE RESTRICT` |
| Max 3 retries before past_due | Service Layer | `IncrementRetryCount` logic |
| One charge per billing cycle | Service Layer | Deterministic transaction ID |

**State Transitions:**

```
Subscription Lifecycle:

[Created] → status='active', failure_retry_count=0
    ↓
[Billing Due] → Cron job processes
    ├─ Success → next_billing_date += interval, failure_retry_count=0
    └─ Failure → failure_retry_count++
        ├─ count < max_retries → status='active' (retry next time)
        └─ count >= max_retries → status='past_due'

[User Actions]
- Pause: active → paused (no billing while paused)
- Resume: paused → active
- Cancel (immediate): active → cancelled
- Cancel (at period end): active (marked for cancellation)

[Cannot Transition]
- paused → past_due (no billing occurs)
- cancelled → active (permanent)
```

**Business Workflows:**

1. **Create Subscription:**
   ```
   Validate customer → Validate payment method (active + verified)
   → Calculate next_billing_date (start_date + interval)
   → Create subscription record → Return
   ```

2. **Daily Billing Process:**
   ```
   Cron trigger → Query due subscriptions (next_billing_date <= today, status='active')
   → For each: Generate deterministic txID → Check if already charged
   → If not charged: Process billing → Update subscription
   ```

3. **Billing Success:**
   ```
   EPX approved → Create transaction (subscription_id link)
   → Calculate next billing date → Reset failure_retry_count
   → Update subscription
   ```

4. **Billing Failure:**
   ```
   EPX declined → Increment failure_retry_count
   → If count >= max_retries: status='past_due'
   → Else: status='active' (retry tomorrow)
   → Log error for monitoring
   ```

### 4.5 Data Integrity

**Foreign Key Constraints:**
```sql
merchant_id UUID REFERENCES merchants(id) ON DELETE RESTRICT
payment_method_id UUID REFERENCES customer_payment_methods(id) ON DELETE RESTRICT
```

**Check Constraints:**
```sql
CHECK (amount_cents > 0)
CHECK (failure_retry_count >= 0)
CHECK (interval_value > 0)
CHECK (interval_unit IN ('day', 'week', 'month', 'year'))
CHECK (status IN ('active', 'paused', 'cancelled', 'past_due'))
```

**Indexes:**
```sql
idx_subscriptions_merchant_id (merchant_id)
idx_subscriptions_merchant_customer (merchant_id, customer_id)
idx_subscriptions_next_billing_date (next_billing_date) WHERE status='active'
idx_subscriptions_status (status)
```

### 4.6 Edge Cases & Error Handling

**Comprehensive Error Scenarios:**

| Scenario | Handling | Robustness |
|----------|----------|------------|
| **Payment method deleted** | ✅ FK RESTRICT prevents deletion | Perfect |
| **Payment method inactive** | ✅ Validation on create/update | Good, ⚠️ not checked before billing |
| **Payment method unverified (ACH)** | ⚠️ Not validated before billing | **GAP** - should check `is_verified` |
| **EPX timeout during billing** | ✅ Increment retry count, retry later | Good |
| **Cron job runs twice** | ✅ Deterministic txID prevents double-charge | Perfect |
| **Billing date calculation overflow** | ✅ Go time.Time handles edge cases | Good |
| **Concurrent pause + billing** | ⚠️ No explicit locking | Potential race |
| **Update payment method during billing** | ⚠️ No locking, could use old method | Potential race |
| **Cancel during billing** | ⚠️ Status check not transaction-safe | Potential race |
| **Max retries reached** | ✅ Status='past_due', no more attempts | Good, ⚠️ no webhook |

**Critical Gaps Identified:**

1. **ACH Verification Not Checked Before Billing:**
   ```go
   // Current: Only checks is_active
   if !pm.IsActive.Valid || !pm.IsActive.Bool {
       return fmt.Errorf("payment method is not active")
   }

   // Should also check:
   if pm.PaymentType == "ach" && (!pm.IsVerified.Valid || !pm.IsVerified.Bool) {
       return fmt.Errorf("ACH payment method is not verified")
   }
   ```

2. **No Webhook Notification for Billing Failures:**
   - When subscription goes `past_due`, merchant is not notified
   - Customer is not notified of failed billing attempts
   - Recommendation: Add webhook events:
     - `subscription.billing_failed` (after each failure)
     - `subscription.past_due` (when max retries hit)

3. **Concurrent Modification Races:**
   ```
   Scenario: User updates payment method while cron job is billing

   Current: Cron job reads payment_method_id, no locking
   Result: Could use old payment method if update happens concurrently

   Fix: Use row-level locking in billing query:
   SELECT * FROM subscriptions
   WHERE next_billing_date <= $1 AND status='active'
   FOR UPDATE SKIP LOCKED
   ```

4. **No Dunning Management:**
   - Once subscription is `past_due`, it stays there forever
   - No retry schedule after max retries
   - No automatic cancellation after X days past due
   - Recommendation: Add dunning workflow:
     - Retry past_due subscriptions with exponential backoff
     - Auto-cancel after 30 days past due

**Error Handling Patterns:**

```go
// Billing Service
processed, success, failed, errors := ProcessDueBilling(ctx, date, batchSize)
// Returns counts + array of errors for monitoring

// Individual subscription billing
if err := processSubscriptionBilling(ctx, sub); err != nil {
    failed++
    errors = append(errors, fmt.Errorf("subscription %s: %w", sub.ID, err))
    logger.Error("Failed to process subscription", zap.Error(err))
    // Continues to next subscription (no fail-fast)
}
```

---

## 5. Chargebacks Table

**Database Location:** `/internal/db/migrations/004_chargebacks.sql`
**Domain Model:** `/internal/domain/chargeback.go`
**Service Layer:** *Not fully implemented - read-only*
**Handlers:** `/internal/handlers/chargeback/`

### 5.1 Service Layer Implementation

**Current Status:** ⚠️ **Read-only implementation**

**Available Methods:**
- ✅ Retrieve chargeback data (synced from North API)
- ⚠️ **Missing:** Write operations (North does not provide write APIs)

**Architecture Note:**
```
North API Integration:
- North provides read-only dispute/chargeback data
- Merchants must respond to chargebacks via North's web portal
- This service provides:
  ✅ Read-only access to chargeback data
  ✅ Webhook notifications when chargebacks are created/updated
  ❌ Cannot submit evidence programmatically (North limitation)
```

### 5.2 Domain Model

**Chargeback Status:**
```go
const (
    ChargebackStatusNew       = "new"       // Just synced from North
    ChargebackStatusPending   = "pending"   // Under review
    ChargebackStatusResponded = "responded" // Evidence submitted (via portal)
    ChargebackStatusWon       = "won"       // Merchant won dispute
    ChargebackStatusLost      = "lost"      // Merchant lost dispute
    ChargebackStatusAccepted  = "accepted"  // Merchant accepted chargeback
)
```

**Business Methods:**
```go
func (c *Chargeback) IsOpen() bool {
    return c.Status == ChargebackStatusNew || c.Status == ChargebackStatusPending
}

func (c *Chargeback) IsResolved() bool {
    return c.Status == ChargebackStatusWon ||
        c.Status == ChargebackStatusLost ||
        c.Status == ChargebackStatusAccepted
}

func (c *Chargeback) CanRespond() bool {
    if !c.IsOpen() {
        return false
    }

    // Check deadline
    if c.RespondByDate != nil && timeutil.Now().After(*c.RespondByDate) {
        return false
    }

    // Already responded?
    if c.ResponseSubmittedAt != nil {
        return false
    }

    return true
}

func (c *Chargeback) IsOverdue() bool {
    if c.RespondByDate == nil {
        return false
    }
    return c.IsOpen() && timeutil.Now().After(*c.RespondByDate)
}

func (c *Chargeback) DaysUntilDeadline() int {
    if c.RespondByDate == nil {
        return 0
    }
    duration := c.RespondByDate.Sub(timeutil.Now())
    return int(duration.Hours() / 24)
}

func (c *Chargeback) MarkResponded() {
    now := timeutil.Now()
    c.ResponseSubmittedAt = &now
    c.Status = ChargebackStatusResponded
    c.UpdatedAt = now
}

func (c *Chargeback) MarkResolved(status ChargebackStatus) error {
    validStatuses := map[ChargebackStatus]bool{
        ChargebackStatusWon:      true,
        ChargebackStatusLost:     true,
        ChargebackStatusAccepted: true,
    }

    if !validStatuses[status] {
        return ErrInvalidChargebackStatus
    }

    now := timeutil.Now()
    c.Status = status
    c.ResolvedAt = &now
    c.UpdatedAt = now

    return nil
}
```

### 5.3 Handler Layer

**API Endpoints (Read-only):**
- `GET /chargebacks` - List chargebacks (filtered by merchant/customer)
- `GET /chargebacks/{case_number}` - Get chargeback details

**Webhook Integration:**
- `POST /webhooks/chargebacks` - Receive chargeback updates from North
- Events: `chargeback.created`, `chargeback.updated`
- Webhook delivery tracked in `webhook_deliveries` table

### 5.4 Business Rules

**Invariants Enforced:**

| Rule | Enforcement | Implementation |
|------|-------------|----------------|
| Case number uniqueness | Database Constraint | `UNIQUE (case_number)` |
| Valid status values | Database Constraint | `CHECK (status IN ('new', 'pending', ...))` |
| Cannot respond after deadline | Domain Layer | `CanRespond()` checks deadline |
| Cannot respond twice | Domain Layer | Checks `ResponseSubmittedAt` |

**State Transitions:**

```
Chargeback Lifecycle (Read-only):

[Synced from North] → status='new'
    ↓ (Merchant reviews)
[Under Review] → status='pending'
    ↓ (Merchant responds via North portal)
[Responded] → status='responded', response_submitted_at set
    ↓ (North resolves)
[Resolved] → status IN ('won', 'lost', 'accepted'), resolved_at set

Note: All transitions happen via North API sync, not via service writes
```

### 5.5 Data Integrity

**Foreign Key Constraints:**
- ❌ **None** (chargebacks reference transactions via `group_id`, but no FK)
- ✅ Denormalized `agent_id`, `customer_id` for querying

**Unique Constraints:**
```sql
UNIQUE (case_number)
```

**Check Constraints:**
```sql
CHECK (status IN ('new', 'pending', 'responded', 'won', 'lost', 'accepted'))
```

**Indexes:**
```sql
idx_chargebacks_case_number (case_number)
idx_chargebacks_agent_id (agent_id)
idx_chargebacks_status (status)
idx_chargebacks_respond_by_date WHERE status='pending'
```

### 5.6 Edge Cases & Error Handling

**Current Limitations:**

| Scenario | Handling | Status |
|----------|----------|--------|
| **Submit evidence** | ❌ Not supported (North limitation) | **GAP** |
| **Auto-respond to chargebacks** | ❌ Not possible | **GAP** |
| **Retrieve chargeback details** | ✅ Synced from North API | Good |
| **Webhook notifications** | ✅ Merchants notified of new chargebacks | Good |
| **Link to transactions** | ⚠️ `group_id` field, but no FK | Weak |
| **Evidence file storage** | ✅ `evidence_files` array (S3 URLs) | Good (if implemented) |

**Recommendations:**

1. **Add Transaction Link FK:**
   ```sql
   ALTER TABLE chargebacks
   ADD COLUMN transaction_id UUID REFERENCES transactions(id);

   -- Migrate existing data
   UPDATE chargebacks SET transaction_id = (
       SELECT id FROM transactions WHERE id = chargebacks.group_id
   );
   ```

2. **Evidence Upload Service:**
   - Allow merchants to upload evidence files via API
   - Store in S3/GCS with signed URLs
   - Store URLs in `evidence_files` array
   - (Note: Still need to manually submit via North portal)

3. **Chargeback Analytics:**
   - Track chargeback rate by merchant
   - Alert when rate exceeds thresholds
   - Identify high-risk transaction patterns

---

## 6. Webhook Tables

**Database Location:** `/internal/db/migrations/007_webhook_subscriptions.sql`
**Domain Model:** *Not explicitly defined*
**Service Layer:** `/internal/services/webhook/webhook_delivery_service.go`
**Handlers:** *Webhook delivery is background process*

### 6.1 Service Layer Implementation

**Tables:**
1. **webhook_subscriptions** - Merchant webhook URLs by event type
2. **webhook_deliveries** - Delivery log for tracking/retries

**Webhook Events (Inferred):**
```
- chargeback.created
- chargeback.updated
(Future: subscription.billing_failed, payment.declined, etc.)
```

**Delivery Flow:**
```
1. Event occurs (e.g., chargeback created)
2. Find active webhook subscriptions for event_type
3. For each subscription:
   a. Create webhook_delivery record (status='pending')
   b. Sign payload with subscription.secret (HMAC-SHA256)
   c. POST to webhook_url with:
      - Headers: X-Webhook-Signature, X-Event-Type
      - Body: JSON event payload
   d. Update delivery record:
      - Success: status='success', delivered_at
      - Failure: status='pending', next_retry_at, increment attempts
4. Retry failed deliveries (exponential backoff)
```

**Schema:**
```sql
CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY,
    agent_id VARCHAR(100) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    webhook_url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL, -- HMAC signing secret
    is_active BOOLEAN DEFAULT true,

    UNIQUE (agent_id, event_type, webhook_url)
);

CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY,
    subscription_id UUID REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL, -- 'pending', 'success', 'failed'
    http_status_code INT,
    error_message TEXT,
    attempts INT DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),

    CHECK (status IN ('pending', 'success', 'failed'))
);
```

### 6.2 Business Rules

**Invariants:**
- ✅ One active webhook per (agent_id, event_type, webhook_url)
- ✅ Delivery records cascade delete with subscription
- ✅ Retry queue indexed for efficient processing

**Retry Logic (Inferred):**
```
Attempt 1: Immediate
Attempt 2: +5 minutes
Attempt 3: +15 minutes
Attempt 4: +1 hour
Attempt 5: +6 hours
Max attempts: 5 (or configurable)

After max attempts: status='failed', stop retrying
```

### 6.3 Data Integrity

**Foreign Key Constraints:**
```sql
subscription_id UUID REFERENCES webhook_subscriptions(id) ON DELETE CASCADE
```

**Indexes:**
```sql
idx_webhook_subscriptions_agent_event (agent_id, event_type) WHERE is_active=true
idx_webhook_deliveries_retry (next_retry_at) WHERE status='pending'
idx_webhook_deliveries_subscription (subscription_id, created_at DESC)
```

### 6.4 Edge Cases & Error Handling

**Scenarios:**

| Scenario | Handling | Status |
|----------|----------|--------|
| **Webhook URL unreachable** | ✅ Retry with exponential backoff | Good |
| **Invalid webhook secret** | ⚠️ No validation on creation | **GAP** |
| **Duplicate webhook subscriptions** | ✅ Unique constraint prevents | Good |
| **Webhook URL returns 4xx** | ⚠️ Unclear if retries (should not retry) | Needs clarification |
| **Webhook URL returns 5xx** | ✅ Should retry | Assumed good |
| **Delivery log retention** | ⚠️ No cleanup strategy | **GAP** |

**Recommendations:**

1. **Retry Policy Refinement:**
   ```go
   // Don't retry 4xx errors (client error, won't fix itself)
   if httpStatus >= 400 && httpStatus < 500 {
       delivery.Status = "failed"
       delivery.ErrorMessage = "Client error, no retry"
   }

   // Retry 5xx errors (server error, may be transient)
   if httpStatus >= 500 {
       scheduleRetry(delivery)
   }
   ```

2. **Webhook Secret Validation:**
   ```go
   // Validate webhook URL is reachable (optional challenge)
   func CreateWebhookSubscription(url string, secret string) error {
       // Send test POST to URL
       // Expect 200 OK or special validation response
   }
   ```

3. **Delivery Log Cleanup:**
   ```sql
   -- Delete successful deliveries older than 30 days
   DELETE FROM webhook_deliveries
   WHERE status='success' AND delivered_at < NOW() - INTERVAL '30 days';

   -- Keep failed deliveries for 90 days (debugging)
   DELETE FROM webhook_deliveries
   WHERE status='failed' AND created_at < NOW() - INTERVAL '90 days';
   ```

---

## 7. Authentication Tables

**Database Location:** `/internal/db/migrations/008_auth_tables.sql`
**Domain Model:** `/internal/domain/auth_context.go`
**Service Layer:** `/internal/services/authorization/`
**Handlers:** `/internal/handlers/admin/`

### 7.1 Tables Overview

**Core Tables:**
1. **admins** - Admin user accounts
2. **services** - All apps/clients (internal + external merchant apps)
3. **service_merchants** - Service-to-merchant access control (many-to-many)
4. **audit_logs** - Comprehensive audit trail (partitioned by month)
5. **rate_limit_buckets** - Token bucket rate limiting
6. **epx_ip_whitelist** - EPX callback IP validation
7. **jwt_blacklist** - Emergency token revocation

**Architecture:**
```
Services (auth) vs Merchants (business entities)

Services:
- Internal: "billing-service", "subscription-service"
- External: "ACME Web App", "ACME Mobile App"
- Auth: JWT with RSA public key verification

Merchants:
- Pure business entity (name, credentials, status)
- No auth (services have auth, merchants don't)

service_merchants (many-to-many):
- Links services to merchants
- Scopes: ['payment:create', 'payment:read', 'subscription:manage']
- Expiration: Optional (expires_at)
```

### 7.2 Business Rules

**JWT Authentication:**
```
Token Structure:
{
    "service_id": "acme-web-app",
    "merchant_id": "uuid",  // Optional (for merchant-scoped tokens)
    "scopes": ["payment:create", "payment:read"],
    "exp": 1234567890
}

Verification:
1. Extract JWT from Authorization: Bearer <token>
2. Verify signature using service.public_key
3. Check expiration
4. Check if token is blacklisted (jwt_blacklist)
5. Validate service is active
6. If merchant_id in token:
   - Verify service has access to merchant (service_merchants)
   - Verify scopes match required permission
```

**Rate Limiting:**
```
Token Bucket Algorithm:
- Bucket key: "service_id:merchant_id" or "service_id"
- Refill rate: service.requests_per_second
- Burst limit: service.burst_limit
- Storage: rate_limit_buckets table

Pseudocode:
tokens_available = min(
    bucket.tokens + (now - last_refill) * refill_rate,
    burst_limit
)
if tokens_available >= 1:
    consume 1 token
    allow request
else:
    reject with 429 Too Many Requests
```

**Audit Logging:**
```
Partitioned by month (audit_logs_YYYY_MM):
- Actor: service_id, admin_id, or system
- Action: payment.create, subscription.cancel, etc.
- Entity: transaction, subscription, payment_method
- Changes: JSONB diff of old/new values
- Context: IP address, user agent, request ID
- Result: success/failure, error message
```

### 7.3 Data Integrity

**Foreign Key Constraints:**
```sql
-- service_merchants
service_id UUID REFERENCES services(id) ON DELETE CASCADE
merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE

-- audit_logs
-- No FKs (actor_id/entity_id stored as VARCHAR for flexibility)
```

**Unique Constraints:**
```sql
-- services
UNIQUE (service_id)

-- service_merchants
PRIMARY KEY (service_id, merchant_id)

-- epx_ip_whitelist
UNIQUE (ip_address)

-- jwt_blacklist
PRIMARY KEY (jti)
```

**Indexes:**
```sql
-- audit_logs (partitioned)
idx_audit_logs_actor (actor_type, actor_id, performed_at DESC)
idx_audit_logs_action (action, performed_at DESC)
idx_audit_logs_entity (entity_type, entity_id, performed_at DESC)
idx_audit_logs_ip (ip_address, performed_at DESC)

-- rate_limit_buckets
PRIMARY KEY (bucket_key)

-- jwt_blacklist
idx_jwt_blacklist_expires (expires_at) -- for cleanup
```

### 7.4 Edge Cases & Error Handling

**Scenarios:**

| Scenario | Handling | Status |
|----------|----------|--------|
| **Token signature invalid** | ✅ Reject with 401 Unauthorized | Good |
| **Token expired** | ✅ Reject with 401 Unauthorized | Good |
| **Token blacklisted** | ✅ Check jwt_blacklist, reject | Good |
| **Service inactive** | ✅ Check services.is_active | Good |
| **Service lacks merchant access** | ✅ Check service_merchants | Good |
| **Insufficient scopes** | ✅ Validate scopes match required permission | Good |
| **Rate limit exceeded** | ✅ Return 429 Too Many Requests | Good |
| **EPX callback from unknown IP** | ✅ Check epx_ip_whitelist | Good |
| **Audit log partition missing** | ⚠️ Insert fails if partition doesn't exist | **GAP** |

**Recommendations:**

1. **Auto-create Audit Log Partitions:**
   ```sql
   -- Cron job to create next month's partition
   CREATE TABLE IF NOT EXISTS audit_logs_2025_12 PARTITION OF audit_logs
       FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');
   ```

2. **JWT Blacklist Cleanup:**
   ```sql
   -- Cron job to delete expired blacklist entries
   DELETE FROM jwt_blacklist WHERE expires_at < NOW();
   ```

3. **Rate Limit Bucket Cleanup:**
   ```sql
   -- Delete stale buckets (no activity in 24 hours)
   DELETE FROM rate_limit_buckets
   WHERE last_refill < NOW() - INTERVAL '24 hours';
   ```

---

## 8. Cross-Cutting Concerns

### 8.1 Error Handling Architecture

**Structured Error Codes:**
```go
// Domain errors with machine-readable codes
const (
    ErrorCodeAuthMissing          = "AUTH_MISSING"
    ErrorCodeMerchantInactive     = "MERCHANT_INACTIVE"
    ErrorCodeTxnNotFound          = "TXN_NOT_FOUND"
    ErrorCodePMNotVerified        = "PM_NOT_VERIFIED"
    ErrorCodeGatewayError         = "GATEWAY_ERROR"
    // ... 20+ error codes
)

type DomainError struct {
    Code    ErrorCode
    Message string
    Err     error
    Details map[string]interface{}
}
```

**Error Propagation:**
```
Layer 1: EPX Gateway
    ↓ (network error, timeout, declined)
Layer 2: Adapter
    ↓ (wrap with context)
Layer 3: Service Layer
    ↓ (domain error with code)
Layer 4: Handler
    ↓ (HTTP status + JSON error response)
Client
```

**HTTP Error Mapping:**
```go
// Handler layer
switch {
case domain.IsNotFoundError(err):
    return 404, {"error": "RESOURCE_NOT_FOUND", "message": "..."}
case domain.IsAuthError(err):
    return 401, {"error": "UNAUTHORIZED", "message": "..."}
case domain.IsValidationError(err):
    return 400, {"error": "VALIDATION_FAILED", "message": "..."}
case domain.IsGatewayError(err):
    return 502, {"error": "GATEWAY_ERROR", "message": "..."}
default:
    return 500, {"error": "INTERNAL_ERROR", "message": "..."}
}
```

### 8.2 Idempotency Patterns

**Transaction Idempotency:**
```
Strategy: idempotency_key IS transaction ID

Advantages:
✅ No separate idempotency table
✅ Natural deduplication via PK
✅ Deterministic TRAN_NBR (FNV-1a hash of UUID)
✅ Safe retries across service restarts

Example:
POST /payments/sale
{
    "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",
    "amount_cents": 1000,
    ...
}

Service:
txID = parse(idempotency_key)
existing = GetTransaction(txID)
if existing && existing.auth_resp != null:
    return existing  // Already processed
else:
    // Process (same TRAN_NBR if retry)
```

**Subscription Billing Idempotency:**
```
Strategy: Deterministic transaction ID per billing cycle

txID = SHA1(subscription_id + next_billing_date)

Example:
- Subscription: sub_123
- Billing date: 2025-11-23
- Transaction ID: SHA1("sub_123-2025-11-23") = abc123...

Cron job:
if transaction_exists(txID):
    skip billing, update next_billing_date
else:
    process billing with txID
```

### 8.3 Caching Strategy

**Merchant Credentials Cache:**
```
Cache: In-memory LRU with TTL
Key: merchant_id
Value: { cust_nbr, merch_nbr, dba_nbr, terminal_nbr, mac_secret_path }
TTL: 5 minutes
Hit rate: ~80% (70% DB load reduction)

Invalidation:
- TTL expiry (automatic)
- Manual invalidation on merchant update
```

**Payment Method Cache:**
```
Cache: In-memory LRU with TTL
Key: payment_method_id
Value: PaymentMethod domain object
TTL: 2 minutes
Hit rate: ~60% (payment method lookups)

Invalidation:
- TTL expiry
- Explicit invalidation on:
  - UpdatePaymentMethodStatus
  - DeletePaymentMethod
  - SetDefaultPaymentMethod
  - MarkPaymentMethodVerified
```

### 8.4 Transaction Management

**Database Transaction Patterns:**
```go
// Pattern 1: WithTx wrapper (most common)
err := txManager.WithTx(ctx, func(q sqlc.Querier) error {
    // Multiple operations in same transaction
    result1, err := q.CreateTransaction(ctx, params1)
    if err != nil {
        return err // Automatic rollback
    }

    result2, err := q.UpdateSubscription(ctx, params2)
    if err != nil {
        return err // Automatic rollback
    }

    return nil // Automatic commit
})

// Pattern 2: Explicit transaction (rare)
tx, err := db.Begin(ctx)
defer tx.Rollback() // No-op if committed
...
tx.Commit()
```

**Transaction Isolation:**
```
Default: READ COMMITTED (PostgreSQL default)

Critical sections (should use locking):
- Concurrent captures on same AUTH
- Concurrent default payment method updates
- Concurrent subscription updates during billing

Recommendation:
SELECT ... FOR UPDATE (row-level locking)
SELECT ... FOR UPDATE SKIP LOCKED (cron jobs)
```

### 8.5 Concurrency Patterns

**WAL-based State (Transactions):**
```
Advantages:
✅ No mutable state (append-only)
✅ Full audit trail
✅ Recomputable from log
✅ No race conditions on state updates

Computation:
state = ComputeGroupState(all_transactions)

Trade-off:
⚠️ O(N) computation (N = transactions in group)
   Mitigated by: Groups typically small (< 10 txns)
```

**Optimistic Locking (Alternative):**
```sql
-- Not currently used, but could add:
UPDATE subscriptions
SET status = 'cancelled', version = version + 1
WHERE id = $1 AND version = $2
```

**Row-level Locking (Recommended for critical sections):**
```sql
-- Prevent concurrent captures on same AUTH
SELECT * FROM transactions
WHERE id = $1
FOR UPDATE;

-- Prevent concurrent billing on same subscription
SELECT * FROM subscriptions
WHERE next_billing_date <= $1 AND status='active'
FOR UPDATE SKIP LOCKED
LIMIT $2;
```

### 8.6 Performance Optimizations

**Database Indexes:**
```
Query: List transactions by merchant + customer
Index: idx_transactions_merchant_customer (merchant_id, customer_id)

Query: Find pending ACH verifications
Index: idx_customer_payment_methods_pending_verification
       (verification_status, created_at) WHERE verification_status='pending'

Query: Find subscriptions due for billing
Index: idx_subscriptions_next_billing_date (next_billing_date)
       WHERE status='active'

Query: Lookup transaction by TRAN_NBR (EPX reconciliation)
Index: idx_transactions_tran_nbr (tran_nbr)
```

**Partial Indexes:**
```sql
-- Index only active payment methods
CREATE INDEX idx_customer_payment_methods_is_active
    ON customer_payment_methods(is_active)
    WHERE is_active = true;

-- Index only soft-deleted records (for cleanup queries)
CREATE INDEX idx_transactions_deleted_at
    ON transactions(deleted_at)
    WHERE deleted_at IS NOT NULL;
```

**Connection Pooling:**
```
PgxPool configuration:
- Max connections: 25 (production)
- Min connections: 5
- Max connection lifetime: 1 hour
- Max connection idle time: 10 minutes
- Health check period: 1 minute
```

---

## 9. Recommendations

### 9.1 Critical Issues (High Priority)

**1. Add ACH Verification Check Before Billing:**
```go
// subscription_service.go - processSubscriptionBilling
pm, err := s.queries.GetPaymentMethodByID(ctx, sub.PaymentMethodID)
if err != nil {
    return fmt.Errorf("failed to get payment method: %w", err)
}

// ADD THIS:
if pm.PaymentType == "ach" && (!pm.IsVerified.Valid || !pm.IsVerified.Bool) {
    return fmt.Errorf("ACH payment method is not verified")
}
```

**2. Add Row-level Locking for Concurrent Operations:**
```sql
-- payment_service.go - Capture
SELECT * FROM transactions
WHERE id = $1
FOR UPDATE;

-- subscription_service.go - ProcessDueBilling
SELECT * FROM subscriptions
WHERE next_billing_date <= $1 AND status='active'
FOR UPDATE SKIP LOCKED
LIMIT $2;
```

**3. Add Webhook Notifications for Billing Failures:**
```go
// After billing failure
if newRetryCount >= sub.MaxRetries {
    // Send webhook: subscription.past_due
    webhookService.Send(ctx, WebhookEvent{
        Type: "subscription.past_due",
        Data: subscription,
    })
}
```

### 9.2 Important Improvements (Medium Priority)

**4. Implement Dunning Management:**
```go
// Cron job: Retry past_due subscriptions with exponential backoff
// Day 1: Retry immediately
// Day 3: Retry again
// Day 7: Retry again
// Day 14: Retry again
// Day 30: Auto-cancel subscription
```

**5. Add Chargeback-Transaction Link:**
```sql
ALTER TABLE chargebacks
ADD COLUMN transaction_id UUID REFERENCES transactions(id);

CREATE INDEX idx_chargebacks_transaction_id
    ON chargebacks(transaction_id);
```

**6. Webhook Retry Policy Refinement:**
```go
// Don't retry 4xx errors
if httpStatus >= 400 && httpStatus < 500 {
    delivery.Status = "failed"
    delivery.ErrorMessage = "Client error, no retry"
    return
}

// Retry 5xx errors with exponential backoff
if httpStatus >= 500 {
    scheduleRetry(delivery, exponentialBackoff(delivery.Attempts))
}
```

### 9.3 Nice-to-Have Enhancements (Low Priority)

**7. Add Transaction Archival Strategy:**
```sql
-- Move transactions older than 2 years to archive table
CREATE TABLE transactions_archive (LIKE transactions);

INSERT INTO transactions_archive
SELECT * FROM transactions
WHERE created_at < NOW() - INTERVAL '2 years';

DELETE FROM transactions
WHERE created_at < NOW() - INTERVAL '2 years';
```

**8. Add Payment Method Usage Analytics:**
```sql
-- Track which payment methods are used most
CREATE MATERIALIZED VIEW payment_method_usage AS
SELECT
    pm.id,
    pm.payment_type,
    COUNT(t.id) AS transaction_count,
    SUM(t.amount_cents) AS total_volume,
    MAX(t.created_at) AS last_used_at
FROM customer_payment_methods pm
LEFT JOIN transactions t ON t.payment_method_id = pm.id
WHERE pm.deleted_at IS NULL
GROUP BY pm.id;

REFRESH MATERIALIZED VIEW payment_method_usage; -- Cron job
```

**9. Add Merchant Chargeback Rate Monitoring:**
```sql
-- Alert when chargeback rate exceeds 1%
SELECT
    m.id,
    m.name,
    COUNT(DISTINCT t.id) AS total_transactions,
    COUNT(DISTINCT c.id) AS total_chargebacks,
    (COUNT(DISTINCT c.id)::float / NULLIF(COUNT(DISTINCT t.id), 0)) AS chargeback_rate
FROM merchants m
LEFT JOIN transactions t ON t.merchant_id = m.id AND t.created_at > NOW() - INTERVAL '30 days'
LEFT JOIN chargebacks c ON c.agent_id = m.id::text AND c.created_at > NOW() - INTERVAL '30 days'
GROUP BY m.id
HAVING (COUNT(DISTINCT c.id)::float / NULLIF(COUNT(DISTINCT t.id), 0)) > 0.01;
```

### 9.4 Documentation Improvements

**10. Add Business Rules Documentation:**
```markdown
# Business Rules Reference

## Payment Method Verification

### ACH Accounts
- Pre-note required (CKC0 for checking, CKS0 for savings)
- 3-day verification period (calendar days)
- Cannot be used until verified
- Auto-deactivate after 2+ returns

### Credit Cards
- No verification required (instantly usable)
- Expiration checked before each transaction
- No grace period for expired cards

## Transaction State Transitions

### AUTH → CAPTURE
- Can capture up to AUTH amount
- Partial captures allowed
- Multiple captures allowed (total <= AUTH amount)
- Cannot void after any capture

### SALE → REFUND
- Can refund up to SALE amount
- Partial refunds allowed
- Multiple refunds allowed (total <= SALE amount)

## Subscription Billing

### Retry Policy
- Max retries: 3 (configurable)
- Retry interval: Daily (next cron run)
- After max retries: status='past_due'

### Payment Method Requirements
- Must be active
- Must be verified (ACH)
- Must not be expired (credit card)
```

---

## Conclusion

### Overall Assessment

The payment service demonstrates **strong domain-driven design** with:
- ✅ Well-structured domain models with business methods
- ✅ Comprehensive validation at multiple layers
- ✅ Robust error handling with structured error codes
- ✅ Effective idempotency patterns
- ✅ Performance optimizations (caching, indexing)
- ✅ Multi-tenant isolation
- ✅ Automated workflows (ACH verification, billing)

### Critical Strengths

1. **Idempotency:** Transaction ID = idempotency key is elegant
2. **WAL-based State:** Transaction groups computed from event log
3. **ACH Verification:** Automated 3-day pre-note workflow
4. **Error Handling:** Structured domain errors with machine-readable codes
5. **Caching:** Significant performance gains (60-70% reduction)

### Key Gaps Identified

1. ⚠️ **ACH verification not checked before subscription billing** (Critical)
2. ⚠️ **Race conditions on concurrent captures** (Important)
3. ⚠️ **No webhook notifications for billing failures** (Important)
4. ⚠️ **Chargeback-transaction link missing** (Medium)
5. ⚠️ **No dunning management for past_due subscriptions** (Medium)

### Next Steps

1. **Immediate:** Implement critical fixes (ACH check, locking)
2. **Short-term:** Add webhook notifications, refine retry policies
3. **Long-term:** Dunning management, analytics, archival

---

**End of Business Logic Analysis**

