# Database Table Usage Analysis

**Payment Service Application - Database Architecture & Usage Patterns**

**Generated:** 2025-11-23
**Version:** 1.0
**Purpose:** Technical reference for understanding how each database table is utilized in business logic

---

## Table of Contents

1. [Core Business Tables](#core-business-tables)
   - [merchants](#1-merchants)
   - [customer_payment_methods](#2-customer_payment_methods)
   - [transactions](#3-transactions)
   - [subscriptions](#4-subscriptions)
   - [chargebacks](#5-chargebacks)
2. [Authentication & Authorization Tables](#authentication--authorization-tables)
   - [services](#6-services)
   - [service_merchants](#7-service_merchants)
   - [admins](#8-admins)
3. [System Tables](#system-tables)
   - [webhook_subscriptions](#9-webhook_subscriptions)
   - [webhook_deliveries](#10-webhook_deliveries)
   - [audit_logs](#11-audit_logs)
   - [rate_limit_buckets](#12-rate_limit_buckets)
   - [jwt_blacklist](#13-jwt_blacklist)
   - [epx_ip_whitelist](#14-epx_ip_whitelist)
4. [Data Flow Diagrams](#data-flow-diagrams)
5. [Performance Summary](#performance-summary)

---

## Core Business Tables

### 1. merchants

**Purpose:** Stores merchant configuration and EPX gateway credentials. Acts as the multi-tenant boundary - all payment operations are scoped to a merchant.

**Schema Overview:**
- **Primary Key:** `id` (UUID)
- **Unique Constraints:** `slug` (human-readable identifier)
- **EPX Credentials:** `cust_nbr`, `merch_nbr`, `dba_nbr`, `terminal_nbr`, `mac_secret_path`
- **Soft Delete:** `deleted_at`

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Get by ID | **VERY HIGH** | Direct lookup by UUID | `GetMerchantByID(uuid)` |
| Get by Slug | **HIGH** | Direct lookup by slug | `GetMerchantBySlug(slug)` |
| List Active | **LOW** | Filter by `is_active = true` | `ListActiveMerchants()` |
| List with Filters | **MEDIUM** | Paginated list with environment/status filters | `ListMerchants(env, active, limit, offset)` |
| Check Existence | **MEDIUM** | Validation queries | `MerchantExists(id)`, `MerchantExistsBySlug(slug)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **RARE** | Admin onboards new merchant | Admin creates merchant via API, generates MAC secret in Vault |
| Update | **RARE** | Merchant config changes | Updates EPX credentials or business details |
| Activate/Deactivate | **RARE** | Status changes | Changes `is_active` flag to enable/disable transactions |
| Update MAC Path | **VERY RARE** | Secret rotation | Updates `mac_secret_path` after rotating MAC secret in Vault |

**Business Logic Dependencies:**

```
Services:
- payment_service.go (lines 98-121): Fetches merchant + MAC secret for every payment operation
- merchant_service.go: CRUD operations for merchant management
- authorization/merchant_resolver.go: Resolves merchant from JWT context
- authorization/merchant_credentials.go: Fetches merchant credentials with caching

Handlers:
- merchant_handler.go: Admin merchant management API
- payment_handler.go: Validates merchant for all payment operations
```

**Performance Characteristics:**

- **Query Frequency:** VERY HIGH (every payment operation requires merchant lookup)
- **Caching Strategy:**
  - **MerchantCredentialCache** (70% DB load reduction)
  - TTL: Configurable (default: 5-15 minutes)
  - Cache hit rate: 90-95%
  - LRU eviction with max size limit
  - Invalidation: On merchant update/deactivation
- **Index Usage:**
  - `idx_merchants_slug`: For slug lookups (UNIQUE)
  - `idx_merchants_is_active`: For filtering active merchants
  - `idx_merchants_environment`: For environment-specific queries
- **Estimated Traffic:**
  - Reads: 1000+ TPS (before cache), ~50-100 TPS (after cache)
  - Writes: <1 per hour

**Relationships:**

```
merchants (1) ──< (many) customer_payment_methods
merchants (1) ──< (many) transactions
merchants (1) ──< (many) subscriptions
merchants (1) ──< (many) service_merchants (authorization)
```

**Business Flow Example:**

```
1. Client calls Payment API with merchant_id/slug
2. PaymentService.Sale() → GetMerchantByID()/GetMerchantBySlug()
3. Check merchant.is_active (reject if inactive)
4. Fetch merchant.mac_secret_path from Vault
5. Build EPX request using merchant.cust_nbr, merch_nbr, dba_nbr, terminal_nbr
6. Send transaction to EPX gateway
```

---

### 2. customer_payment_methods

**Purpose:** Stores tokenized payment methods (credit cards, ACH accounts) with BRIC tokens from EPX. Supports saved payment methods for recurring billing and customer convenience.

**Schema Overview:**
- **Primary Key:** `id` (UUID)
- **BRIC Token:** `bric` (TEXT) - EPX AUTH_GUID from STORAGE transaction
- **Multi-tenant:** `merchant_id` + `customer_id` (UUID)
- **Payment Types:** `credit_card`, `ach`
- **ACH Verification:** `verification_status` (pending/verified/failed), `prenote_transaction_id`
- **Unique Constraint:** `(merchant_id, customer_id, bric)` - prevents duplicate payment methods

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Get by ID | **VERY HIGH** | Direct lookup by UUID (cached) | `GetPaymentMethodByID(uuid)` |
| List by Customer | **HIGH** | Filter by merchant + customer | `ListPaymentMethodsByCustomer(merchant, customer)` |
| Get Default | **HIGH** | Find default payment method | `GetDefaultPaymentMethod(merchant, customer)` |
| ACH Verification Queries | **MEDIUM** | Cron job batch queries | `GetPendingACHVerifications(cutoff, limit)` |
| ACH Statistics | **LOW** | Monitoring/reporting | `CountTotalACH()`, `CountPendingACH()`, `CountVerifiedACH()` |
| Get by PreNote Transaction | **LOW** | Return code processing | `GetPaymentMethodByPreNoteTransaction(txn_id)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **HIGH** | Customer saves card/ACH | Tokenization via BrowserPost/ServerPost, stores BRIC |
| Mark Used | **VERY HIGH** | After successful payment | Updates `last_used_at` timestamp |
| Set Default | **MEDIUM** | Customer selects default | Unsets all defaults, sets new default (transaction) |
| Verify ACH | **MEDIUM** | Cron job (3 days after pre-note) | Updates `verification_status = 'verified'`, `is_verified = true`, `verified_at` |
| Mark Verified | **MEDIUM** | Manual verification | Sets `is_verified = true`, `verification_status = 'verified'` |
| Deactivate | **MEDIUM** | User/system deactivates | Sets `is_active = false` (with optional reason) |
| Increment Return Count | **LOW** | ACH return received | Increments `return_count`, auto-deactivates if threshold reached |
| Mark Verification Failed | **LOW** | Pre-note returns | Sets `verification_status = 'failed'`, deactivates |
| Soft Delete | **LOW** | Customer deletes payment method | Sets `deleted_at` timestamp |

**Business Logic Dependencies:**

```
Services:
- payment_method_service.go: CRUD operations, ACH verification
- payment_service.go (lines 1478-1531): Resolves payment method for transactions
- subscription_service.go: Uses payment methods for recurring billing

Handlers:
- payment_method_handler.go: Payment method management API
- cron/ach_verification_handler.go: Auto-verifies ACH accounts after 3 days

Cron Jobs:
- ach_verification_handler.go: Batch verifies ACH accounts (default 3 days)
```

**Performance Characteristics:**

- **Query Frequency:**
  - VERY HIGH for payment operations (80% of payments use saved methods)
  - At 1000 TPS: ~800 payment method lookups/sec
- **Caching Strategy:**
  - **PaymentMethodCache** (60% faster lookups)
  - TTL: Configurable (default: 5-10 minutes)
  - Cache hit rate: 90-95%
  - LRU eviction with max size limit
  - Invalidation: On update, delete, verification status change
  - Customer-level invalidation: When setting default (affects multiple records)
- **Index Usage:**
  - `idx_customer_payment_methods_merchant_customer`: Primary lookup path (composite)
  - `idx_customer_payment_methods_is_default`: Fast default lookup
  - `idx_customer_payment_methods_is_active`: Filter active methods
  - `idx_payment_methods_sorted`: Optimized listing (migration 012)
  - `idx_ach_verification`: ACH verification queries (migration 022)
- **Estimated Traffic:**
  - Reads: 800+ TPS (before cache), ~40-80 TPS (after cache)
  - Writes: 100-200 TPS (mark used + new tokenizations)

**ACH Verification Workflow:**

```
Day 0: Customer adds ACH account
  ├─> StoreACHAccount() sends Pre-Note (CKC0/CKS0) to EPX
  ├─> EPX returns AUTH_GUID (BRIC)
  ├─> Store payment method: verification_status='pending', is_active=false, is_verified=false
  └─> Store prenote_transaction_id for return tracking

Day 3: Cron job runs (POST /cron/verify-ach)
  ├─> FindEligibleACHForVerification(cutoff_date = now - 3 days)
  ├─> VerifyACHPaymentMethod() for each eligible account
  │   ├─> Sets verification_status='verified'
  │   ├─> Sets is_verified=true, verified_at=NOW()
  │   └─> Sets is_active=true (now usable for payments)
  └─> Customer can now use ACH account for payments

ACH Return Processing (if return received):
  ├─> GetPaymentMethodByPreNoteTransaction(prenote_txn_id)
  ├─> IncrementReturnCount()
  │   ├─> Increments return_count
  │   └─> Auto-deactivates if return_count >= threshold (e.g., 2)
  └─> MarkVerificationFailed() if critical return code
```

**Relationships:**

```
merchants (1) ──< (many) customer_payment_methods
customer_payment_methods (1) ──< (many) transactions (payment_method_id)
customer_payment_methods (1) ──< (many) subscriptions (payment_method_id)
customer_payment_methods (1) ──o (0-1) transactions (prenote_transaction_id)
```

**Business Flow Example - Saved Payment:**

```
1. Client calls Payment.Sale(payment_method_id)
2. PaymentService.resolvePaymentToken()
   ├─> PaymentMethodCache.Get(payment_method_id)
   │   ├─> Cache HIT (90-95% of time): Return cached payment method
   │   └─> Cache MISS: GetPaymentMethodByID() → Cache result
   ├─> Validate: CanUseForAmount(amount)
   │   ├─> Check is_active
   │   ├─> Check credit card expiration
   │   └─> Check ACH verification (is_verified=true)
   └─> Return BRIC token + payment method info
3. PaymentService builds EPX request with BRIC
4. After successful payment: MarkPaymentMethodUsed(id) → Updates last_used_at
```

---

### 3. transactions

**Purpose:** Immutable append-only event log of all payment operations. Supports transaction chains (AUTH→CAPTURE→REFUND) with parent-child relationships. Implements idempotency via client-provided UUIDs.

**Schema Overview:**
- **Primary Key:** `id` (UUID) - Client-provided via idempotency_key
- **Parent Relationship:** `parent_transaction_id` (UUID) - Links CAPTURE→AUTH, REFUND→SALE/CAPTURE, VOID→AUTH
- **Types:** SALE, AUTH, CAPTURE, REFUND, VOID, STORAGE, DEBIT
- **Status:** GENERATED column based on `auth_resp` (pending/approved/declined/failed)
- **EPX Fields:** `tran_nbr` (10-digit deterministic ID), `auth_guid` (BRIC for this txn), `auth_resp`, `auth_code`
- **Immutability:** No UPDATE operations (except EPX response callback), only INSERT

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Get by ID | **VERY HIGH** | Direct lookup by UUID | `GetTransactionByID(uuid)` |
| Get Transaction Tree | **HIGH** | Recursive CTE to fetch parent + all descendants | `GetTransactionTree(txn_id)` |
| Get by TRAN_NBR | **HIGH** | EPX callback lookup | `GetTransactionByTranNbr(tran_nbr)` |
| List Transactions | **HIGH** | Paginated list with filters | `ListTransactions(merchant, customer, status, type, limit, offset)` |
| Get by Idempotency Key | **HIGH** | Duplicate request detection | Same as Get by ID (idempotency_key IS transaction.id) |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **VERY HIGH** | All payment operations | Idempotent INSERT with ON CONFLICT DO NOTHING |
| Create Pending | **HIGH** | Before EPX call (CAPTURE/REFUND/VOID) | Pre-creates transaction with null auth_resp |
| Update from EPX Response | **HIGH** | EPX callback (Browser Post) | Updates pending txn with EPX response (SECURITY: Only updates PENDING transactions) |

**Idempotency Strategy:**
```
1. Client provides idempotency_key (UUID) in request
2. PaymentService uses UUID as transaction.id
3. INSERT ... ON CONFLICT (id) DO NOTHING
4. If conflict (duplicate request): Return existing transaction
5. SECURITY: UpdateTransactionFromEPXResponse only updates WHERE status='PENDING'
   - Prevents TAC replay attacks
   - Ensures completed transactions cannot be modified
```

**Business Logic Dependencies:**

```
Services:
- payment_service.go: Core payment operations (Sale, Auth, Capture, Refund, Void)
  ├─> CreateTransaction() - Idempotent insert
  ├─> CreatePendingTransaction() - Pre-create before EPX call
  ├─> UpdateTransactionWithEPXResponse() - EPX callback processing
  ├─> GetTransactionTree() - Fetch parent + children for state computation
  └─> ComputeGroupState() - WAL-based state machine
- subscription_service.go: Creates transactions for recurring billing
- payment/group_state.go: Computes aggregate state from transaction tree

Handlers:
- payment_handler.go: Payment API operations
- browser_post_callback_handler.go: EPX Browser Post callback processing
```

**Transaction State Machine (WAL-based):**

```
Transaction Group State is computed from entire transaction tree:

SALE (standalone):
  ├─> APPROVED: Money captured
  └─> DECLINED: No money movement

AUTH → CAPTURE chain:
  AUTH (APPROVED)
    ├─> CAPTURE (APPROVED): Money captured (can refund)
    ├─> CAPTURE (DECLINED): Auth still active (can retry capture)
    └─> VOID: Auth cancelled, no capture allowed

  AUTH (DECLINED)
    └─> Cannot capture or void

REFUND chain:
  SALE/CAPTURE (APPROVED)
    ├─> REFUND (partial): Remaining amount can still be refunded
    ├─> REFUND (full): No more refunds allowed
    └─> Multiple partial refunds allowed (up to captured amount)

State Validation (group_state.go):
- CanCapture(amount): Checks active auth exists, amount <= auth amount
- CanRefund(amount): Checks captured amount, amount <= (captured - refunded)
- CanVoid(): Checks no captures exist, auth is active
```

**Performance Characteristics:**

- **Query Frequency:**
  - VERY HIGH (all payment operations + transaction history)
  - At 1000 TPS: ~1000 writes + ~2000 reads/sec
- **Caching Strategy:**
  - **NO CACHING** (transactions change frequently, need real-time data)
  - GetTransactionTree queries may hit same txn multiple times (e.g., CAPTURE → fetch AUTH tree)
- **Index Usage:**
  - `idx_transactions_tran_nbr`: EPX callback lookup (UNIQUE on tran_nbr)
  - `idx_transactions_parent_id`: Transaction tree queries
  - `idx_transactions_merchant_customer`: Customer transaction history
  - `idx_transactions_created_at DESC`: Recent transactions listing
  - `idx_transaction_listing_indexes`: Optimized listing (migration 020)
- **Partitioning:** None (consider time-based partitioning for scale)
- **Estimated Traffic:**
  - Reads: 2000+ TPS
  - Writes: 1000+ TPS

**GetTransactionTree CTE Explanation:**

```sql
-- Recursive CTE walks UP to root, then DOWN to get all descendants
-- Example: GetTransactionTree(capture_id) returns [auth, auth2, capture, refund]

WITH RECURSIVE
  -- Step 1: Walk UP parent chain to find root
  find_root AS (
    SELECT * FROM transactions WHERE id = $transaction_id
    UNION ALL
    SELECT t.* FROM transactions t
    INNER JOIN find_root fr ON fr.parent_transaction_id = t.id
    WHERE fr.depth < 100  -- Prevent infinite recursion
  ),
  -- Step 2: Get root (no parent)
  root AS (
    SELECT * FROM find_root WHERE parent_transaction_id IS NULL LIMIT 1
  ),
  -- Step 3: Walk DOWN from root to get all descendants
  full_tree AS (
    SELECT * FROM root
    UNION ALL
    SELECT t.* FROM transactions t
    INNER JOIN full_tree ft ON t.parent_transaction_id = ft.id
    WHERE ft.depth < 100
  )
SELECT * FROM full_tree ORDER BY created_at ASC;
```

**Relationships:**

```
merchants (1) ──< (many) transactions
customer_payment_methods (1) ──< (many) transactions (payment_method_id)
subscriptions (1) ──< (many) transactions (subscription_id)
transactions (1) ──< (many) transactions (parent_transaction_id) - Self-referential
transactions (1) ──o (0-1) chargebacks
```

**Business Flow Example - CAPTURE:**

```
1. Client calls Payment.Capture(auth_transaction_id, amount)
2. PaymentService.Capture():
   ├─> GetTransactionByID(auth_transaction_id)
   ├─> ValidateTransactionAccess() - Check merchant owns transaction
   ├─> Generate capture_txn_id (from idempotency_key or new UUID)
   ├─> Check idempotency: GetTransactionByID(capture_txn_id)
   │   └─> If exists and complete: Return existing (idempotent)
   ├─> GetTransactionTree(auth_transaction_id)
   │   └─> Returns [auth, auth2, capture2, refund2] (entire tree)
   ├─> ComputeGroupState(tree)
   │   ├─> Find active auth (latest APPROVED AUTH with no VOID)
   │   ├─> Calculate captured amount (sum of APPROVED CAPTURE)
   │   └─> CanCapture(amount)? Check: amount <= (auth_amount - captured)
   ├─> CreatePendingTransaction(capture_txn_id, parent=auth_txn_id)
   │   └─> INSERT with auth_resp=NULL (status=pending)
   ├─> Call EPX ServerPost (CAPTURE, ORIG_AUTH_GUID=auth_bric)
   └─> UpdateTransactionWithEPXResponse(tran_nbr, epx_response)
       └─> UPDATE transactions SET auth_resp=..., auth_code=... WHERE tran_nbr=... AND status='PENDING'
3. Return updated transaction
```

---

### 4. subscriptions

**Purpose:** Manages recurring billing subscriptions. Tracks billing intervals, payment method, retry logic, and subscription lifecycle.

**Schema Overview:**
- **Primary Key:** `id` (UUID)
- **Billing Interval:** `interval_value` + `interval_unit` (e.g., 1 month, 2 weeks)
- **Status:** active, paused, cancelled, past_due
- **Payment Method:** `payment_method_id` (FK with ON DELETE RESTRICT - cannot delete PM with active subscriptions)
- **Retry Logic:** `failure_retry_count`, `max_retries`
- **Next Billing:** `next_billing_date` (DATE)

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Get by ID | **HIGH** | Direct lookup | `GetSubscriptionByID(uuid)` |
| List by Customer | **HIGH** | Filter by merchant + customer | `ListSubscriptionsByCustomer(merchant, customer)` |
| List Due for Billing | **VERY HIGH** | Cron job batch query | `ListSubscriptionsDueForBilling(next_billing_date <= now, limit)` |
| List with Filters | **MEDIUM** | Paginated list | `ListSubscriptions(merchant, customer, status, limit, offset)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **MEDIUM** | Customer subscribes | Creates subscription with next_billing_date calculated |
| Update | **LOW** | Customer changes plan/payment method | Updates amount, interval, payment_method_id |
| Update Status | **MEDIUM** | Lifecycle changes | Updates status (active/paused/cancelled/past_due) |
| Update Billing | **VERY HIGH** | After successful billing | Updates next_billing_date, resets failure_retry_count |
| Increment Failure Count | **MEDIUM** | Billing failure | Increments failure_retry_count, updates status to past_due if exceeded |
| Reset Retry Count | **MEDIUM** | After successful retry | Resets failure_retry_count to 0 |
| Cancel | **MEDIUM** | Customer cancels | Sets status='cancelled', cancelled_at timestamp |

**Business Logic Dependencies:**

```
Services:
- subscription_service.go: CRUD operations, billing processing
  ├─> ProcessDueBilling(as_of_date, batch_size) - Cron job
  ├─> ProcessSubscriptionBilling(subscription_id) - Single billing
  └─> Retry logic with exponential backoff

Handlers:
- subscription_handler.go: Subscription management API
- cron/billing_handler.go: Cron job endpoint for recurring billing
```

**Billing Workflow:**

```
Cron Job (runs daily or hourly):
  ├─> ListSubscriptionsDueForBilling(next_billing_date <= now, limit=100)
  ├─> For each subscription:
  │   ├─> Get payment_method (validates is_active, is_verified)
  │   ├─> Call PaymentService.Sale(amount, payment_method_id, subscription_id)
  │   ├─> If SUCCESS:
  │   │   ├─> UpdateSubscriptionBilling():
  │   │   │   ├─> Calculate next_billing_date (e.g., +1 month)
  │   │   │   ├─> Reset failure_retry_count=0
  │   │   │   └─> Ensure status='active'
  │   │   └─> Create transaction with subscription_id link
  │   └─> If FAILURE:
  │       ├─> IncrementSubscriptionFailureCount():
  │       │   ├─> Increment failure_retry_count
  │       │   └─> If retry_count >= max_retries: Set status='past_due'
  │       └─> Schedule retry (exponential backoff)
  └─> Return: processed, success_count, failure_count

Retry Schedule:
  - Attempt 1: Immediately (T+0)
  - Attempt 2: +1 day (T+1d)
  - Attempt 3: +3 days (T+4d)
  - Attempt 4: +7 days (T+11d)
  - After max_retries: status='past_due', customer notified
```

**Performance Characteristics:**

- **Query Frequency:**
  - MEDIUM (mostly read for subscription management)
  - Cron job: Batch queries every hour/day
- **Caching Strategy:**
  - **NO CACHING** (next_billing_date changes frequently)
- **Index Usage:**
  - `idx_subscriptions_next_billing_date`: WHERE status='active' for cron job
  - `idx_subscriptions_merchant_customer`: Customer subscription listing
  - `idx_subscriptions_status`: Status filtering
- **Estimated Traffic:**
  - Reads: 50-100 TPS
  - Writes: 10-20 TPS (billing updates)

**Relationships:**

```
merchants (1) ──< (many) subscriptions
customer_payment_methods (1) ──< (many) subscriptions (RESTRICT delete)
subscriptions (1) ──< (many) transactions (subscription_id)
```

**Business Flow Example - Recurring Billing:**

```
1. Cron job hits /cron/process-billing (hourly)
2. SubscriptionService.ProcessDueBilling():
   ├─> ListSubscriptionsDueForBilling(now, 100)
   ├─> For subscription in subscriptions:
   │   ├─> Get payment_method (cache hit ~90%)
   │   ├─> Validate: is_active, is_verified (ACH)
   │   ├─> PaymentService.Sale():
   │   │   ├─> Create transaction with subscription_id
   │   │   ├─> Call EPX ServerPost
   │   │   └─> Return transaction result
   │   ├─> If transaction.status='approved':
   │   │   └─> UpdateSubscriptionBilling():
   │   │       ├─> next_billing_date = calculate_next_date(interval)
   │   │       ├─> failure_retry_count = 0
   │   │       └─> status = 'active'
   │   └─> If transaction.status='declined':
   │       └─> IncrementSubscriptionFailureCount():
   │           ├─> failure_retry_count++
   │           └─> If count >= max_retries: status='past_due'
   └─> Return summary: processed=100, success=95, failed=5
```

---

### 5. chargebacks

**Purpose:** Tracks dispute/chargeback cases from card networks. Links to original transaction for chargeback defense.

**Schema Overview:**
- **Primary Key:** `id` (UUID)
- **Transaction Link:** `transaction_id` (FK to original transaction)
- **Merchant ID:** `agent_id` (merchant/agent identifier from gateway)
- **Case Details:** `case_number`, `dispute_date`, `chargeback_date`, `reason_code`
- **Evidence:** `evidence_files` (TEXT[]), `response_notes`, `internal_notes`
- **Lifecycle:** `status` (new, under_review, responded, won, lost, etc.), `respond_by_date`

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Get by ID | **LOW** | Direct lookup | `GetChargebackByID(uuid)` |
| Get by Case Number | **LOW** | Gateway sync | `GetChargebackByCaseNumber(agent_id, case_number)` |
| Get by Transaction | **LOW** | Transaction dispute lookup | `GetChargebackByTransactionID(txn_id)` |
| List Chargebacks | **MEDIUM** | Admin dashboard, filters | `ListChargebacks(agent_id, customer_id, status, date_range, limit, offset)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **RARE** | Gateway webhook or sync | Creates chargeback from gateway notification |
| Update Status | **RARE** | Status changes | Updates status, resolved_at timestamp |
| Add Evidence File | **RARE** | Merchant uploads evidence | Appends file URL to evidence_files array |
| Update Response | **RARE** | Merchant submits response | Sets response_notes, response_submitted_at, status='responded' |
| Mark Resolved | **RARE** | Final outcome | Sets status (won/lost), resolved_at timestamp |

**Business Logic Dependencies:**

```
Handlers:
- chargeback_handler.go: Chargeback management API (likely admin-only)

Cron Jobs:
- dispute_sync_handler.go: Syncs chargebacks from gateway
```

**Performance Characteristics:**

- **Query Frequency:** **VERY LOW** (chargebacks are rare, typically <1% of transactions)
- **Caching Strategy:** **NO CACHING** (low traffic, not worth it)
- **Index Usage:**
  - `transaction_id`: Link to original transaction
  - `agent_id`, `case_number`: Gateway sync
  - `status`, `dispute_date`: Filtering/reporting
- **Estimated Traffic:**
  - Reads: <1 TPS
  - Writes: <0.1 TPS (very rare)

**Relationships:**

```
transactions (1) ──o (0-1) chargebacks
```

**Business Flow Example:**

```
1. Gateway sends chargeback webhook OR Cron sync detects new chargeback
2. ChargebackHandler.CreateChargeback():
   ├─> Parse gateway data (case_number, reason_code, amount, dispute_date)
   ├─> Find transaction by gateway transaction ID
   ├─> Create chargeback record with status='new'
   └─> Notify merchant (email/webhook)
3. Merchant uploads evidence via API
   ├─> AddEvidenceFile(chargeback_id, file_url)
   └─> Append to evidence_files array
4. Merchant submits response
   ├─> UpdateChargebackResponse(chargeback_id, response_notes)
   └─> Sets status='responded', response_submitted_at
5. Gateway sends outcome
   ├─> MarkChargebackResolved(chargeback_id, status='won' or 'lost')
   └─> Sets resolved_at timestamp
```

---

## Authentication & Authorization Tables

### 6. services

**Purpose:** Stores API client credentials (services/applications) that access the payment API. Supports JWT-based authentication with RSA public keys. Implements service-based rate limiting.

**Schema Overview:**
- **Primary Key:** `id` (UUID)
- **Unique Identifier:** `service_id` (VARCHAR, e.g., "acme-web-app", "billing-service")
- **JWT Auth:** `public_key` (RSA public key for JWT signature verification), `public_key_fingerprint`
- **Rate Limits:** `requests_per_second`, `burst_limit` (per service, not per merchant)
- **Environment:** `environment` (staging, production)

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Get by Service ID | **VERY HIGH** | JWT verification | `GetServiceByServiceID(service_id)` |
| List Active Public Keys | **VERY HIGH** | JWT key rotation | `ListActiveServicePublicKeys()` |
| Get Rate Limit | **VERY HIGH** | Rate limiting | `GetServiceRateLimit(service_id)` |
| List Services | **LOW** | Admin management | `ListServices(environment, is_active, limit, offset)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **RARE** | Admin onboards service | Generates service credentials |
| Update | **RARE** | Service config changes | Updates rate limits, name, etc. |
| Rotate Key | **RARE** | Key rotation | Updates public_key, public_key_fingerprint |
| Activate/Deactivate | **RARE** | Enable/disable service | Sets is_active flag |

**Business Logic Dependencies:**

```
Middleware:
- JWT authentication middleware (validates all API requests)
- Rate limiting middleware

Services:
- authorization/merchant_authorization.go: JWT parsing and validation

Handlers:
- admin/service_handler.go: Service management API
```

**JWT Authentication Flow:**

```
1. Client sends JWT in Authorization header
2. JWT Middleware:
   ├─> Extract service_id from JWT claims
   ├─> GetServiceByServiceID(service_id)
   │   └─> Check is_active=true
   ├─> ListActiveServicePublicKeys()
   │   └─> Cache public keys in memory (refresh periodically)
   ├─> Verify JWT signature using public_key
   ├─> Check JWT expiration (exp claim)
   ├─> Check JWT blacklist (IsJWTBlacklisted(jti))
   └─> Extract merchant_id/merchant_slug from claims
3. Request proceeds with authenticated context
```

**Performance Characteristics:**

- **Query Frequency:** **VERY HIGH** (every API request requires JWT validation)
  - At 1000 TPS: ~1000 service lookups/sec
- **Caching Strategy:**
  - Public keys cached in memory (refresh every 5-15 minutes)
  - Service metadata cached (TTL: 5-15 minutes)
- **Index Usage:**
  - `service_id UNIQUE`: Fast service lookup
  - `is_active`: Filter active services
  - `environment`: Environment-specific queries
- **Estimated Traffic:**
  - Reads: 1000+ TPS (before cache), ~10-50 TPS (after cache)
  - Writes: <1 per day

**Relationships:**

```
services (1) ──< (many) service_merchants (authorization mapping)
services (1) ──< (many) jwt_blacklist (revoked tokens)
services (1) ──< (many) rate_limit_buckets
admins (1) ──< (many) services (created_by)
```

---

### 7. service_merchants

**Purpose:** Junction table linking services to merchants with scoped permissions. Implements fine-grained access control for multi-tenant API clients.

**Schema Overview:**
- **Primary Key:** `(service_id, merchant_id)` (composite)
- **Scopes:** `scopes` (TEXT[]) - Array of permissions (e.g., ['payment:create', 'payment:read', 'subscription:manage'])
- **Expiration:** `expires_at` (TIMESTAMP) - Optional time-based access expiration
- **Audit:** `granted_by` (admin_id), `granted_at`

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Get Access | **VERY HIGH** | Check service-merchant access | `GetServiceMerchantAccess(service_id, merchant_id)` |
| Check Access (by ID) | **VERY HIGH** | Authorization check | `CheckServiceMerchantAccessByID(service_id, merchant_id)` |
| Check Access (by Slug) | **VERY HIGH** | Authorization check | `CheckServiceMerchantAccessBySlug(service_id, merchant_slug)` |
| Check Scope | **HIGH** | Permission check | `CheckServiceHasScope(service_id, merchant_id, scope)` |
| List Service Merchants | **LOW** | Admin dashboard | `ListServiceMerchants(service_id)` |
| List Merchant Services | **LOW** | Admin dashboard | `ListMerchantServices(merchant_id)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Grant Access | **RARE** | Admin grants service access to merchant | UPSERT with scopes |
| Update Scopes | **RARE** | Admin updates permissions | Updates scopes array |
| Revoke Access | **RARE** | Admin revokes access | DELETE |

**Business Logic Dependencies:**

```
Services:
- authorization/merchant_authorization.go: Validates service-merchant access

Handlers:
- admin/service_handler.go: Service-merchant access management
```

**Authorization Flow:**

```
1. JWT Middleware extracts: service_id, merchant_id/merchant_slug
2. MerchantAuthorizationService.ValidateAccess():
   ├─> CheckServiceMerchantAccessByID(service_id, merchant_id)
   │   OR
   │   CheckServiceMerchantAccessBySlug(service_id, merchant_slug)
   ├─> Checks:
   │   ├─> service.is_active = true
   │   ├─> merchant.is_active = true
   │   └─> (expires_at IS NULL OR expires_at > NOW())
   └─> Returns: has_access (boolean)
3. Optional: CheckServiceHasScope(service_id, merchant_id, 'payment:create')
4. If authorized: Request proceeds
   Else: Return 403 Forbidden
```

**Performance Characteristics:**

- **Query Frequency:** **VERY HIGH** (every API request validates access)
  - At 1000 TPS: ~1000 access checks/sec
- **Caching Strategy:**
  - Could cache service-merchant access (TTL: 5-15 minutes)
  - Currently NO CACHING (authorize on every request for security)
- **Index Usage:**
  - `PRIMARY KEY (service_id, merchant_id)`: Fast access lookup
  - `idx_service_merchants_service`: List merchants for service
  - `idx_service_merchants_merchant`: List services for merchant
  - `idx_service_merchants_expires`: Filter expired access
- **Estimated Traffic:**
  - Reads: 1000+ TPS
  - Writes: <1 per hour

**Relationships:**

```
services (1) ──< (many) service_merchants
merchants (1) ──< (many) service_merchants
admins (1) ──< (many) service_merchants (granted_by)
```

---

### 8. admins

**Purpose:** Stores admin user credentials for administrative access. Supports admin sessions and audit logging.

**Schema Overview:**
- **Primary Key:** `id` (UUID)
- **Email:** `email` (UNIQUE)
- **Auth:** `password_hash` (bcrypt)
- **Role:** `role` (admin, super_admin, etc.)
- **Status:** `is_active` (boolean)

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Get by Email | **LOW** | Admin login | `GetAdminByEmail(email)` |
| Get by ID | **LOW** | Session validation | `GetAdminByID(uuid)` |
| List Admins | **VERY LOW** | Admin management | `ListAdmins(role, is_active, limit, offset)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **VERY RARE** | Super admin creates admin | Creates admin with hashed password |
| Update Password | **RARE** | Admin changes password | Updates password_hash |
| Update Role | **RARE** | Admin role change | Updates role |
| Activate/Deactivate | **RARE** | Enable/disable admin | Sets is_active flag |

**Business Logic Dependencies:**

```
Handlers:
- admin/service_handler.go: Admin management API

Sessions:
- admin_sessions table: Track active admin sessions
```

**Performance Characteristics:**

- **Query Frequency:** **VERY LOW** (admin operations are infrequent)
- **Caching Strategy:** **NO CACHING** (security-sensitive, low traffic)
- **Index Usage:**
  - `email UNIQUE`: Admin login
  - `is_active`: Filter active admins
- **Estimated Traffic:**
  - Reads: <1 TPS
  - Writes: <0.01 TPS (very rare)

**Relationships:**

```
admins (1) ──< (many) services (created_by)
admins (1) ──< (many) merchants (created_by, approved_by)
admins (1) ──< (many) service_merchants (granted_by)
admins (1) ──< (many) admin_sessions
admins (1) ──< (many) jwt_blacklist (blacklisted_by)
```

---

## System Tables

### 9. webhook_subscriptions

**Purpose:** Stores webhook endpoints for event notifications (payment.completed, subscription.billed, etc.).

**Schema Overview:**
- **Primary Key:** `id` (UUID)
- **Agent/Merchant:** `agent_id` (merchant identifier)
- **Event Type:** `event_type` (payment.completed, payment.failed, subscription.billed, etc.)
- **Endpoint:** `webhook_url` (HTTPS endpoint)
- **Security:** `secret` (HMAC secret for signature verification)
- **Status:** `is_active` (boolean)

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| List Active by Event | **HIGH** | Webhook dispatch | `ListActiveWebhooksByEvent(agent_id, event_type)` |
| Get by ID | **LOW** | Webhook management | `GetWebhookSubscription(id)` |
| List Subscriptions | **LOW** | Admin dashboard | `ListWebhookSubscriptions(agent_id, is_active)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **LOW** | Merchant subscribes to webhook | Creates webhook subscription |
| Update | **LOW** | URL/secret change | Updates webhook_url, secret, is_active |
| Delete | **LOW** | Merchant unsubscribes | Deletes subscription |

**Business Logic Dependencies:**

```
Services:
- webhook_delivery_service.go: Webhook dispatch logic

Event Triggers:
- After successful payment: Trigger payment.completed webhook
- After subscription billing: Trigger subscription.billed webhook
- After payment failure: Trigger payment.failed webhook
```

**Performance Characteristics:**

- **Query Frequency:** **HIGH** (every payment event triggers webhook lookup)
  - At 1000 TPS payment operations: ~1000 webhook lookups/sec
- **Caching Strategy:**
  - Could cache active webhooks by event type
  - Currently NO CACHING (ensure fresh webhook config)
- **Index Usage:**
  - `agent_id`, `event_type`, `is_active`: Fast webhook lookup
- **Estimated Traffic:**
  - Reads: 1000+ TPS
  - Writes: <1 per hour

**Relationships:**

```
webhook_subscriptions (1) ──< (many) webhook_deliveries
```

---

### 10. webhook_deliveries

**Purpose:** Tracks webhook delivery attempts with retry logic. Ensures reliable event delivery with exponential backoff.

**Schema Overview:**
- **Primary Key:** `id` (UUID)
- **Subscription:** `subscription_id` (FK to webhook_subscriptions)
- **Event:** `event_type`, `payload` (JSONB)
- **Delivery Status:** `status` (pending, succeeded, failed), `attempts`, `http_status_code`, `error_message`
- **Retry:** `next_retry_at` (TIMESTAMP)
- **Timestamps:** `delivered_at`, `created_at`

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| List Pending | **HIGH** | Retry worker | `ListPendingWebhookDeliveries(next_retry_at <= now, limit)` |
| Get Delivery History | **LOW** | Admin debugging | `GetWebhookDeliveryHistory(subscription_id, limit, offset)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **VERY HIGH** | Every payment event | Creates delivery record with status='pending' |
| Update Status | **VERY HIGH** | After delivery attempt | Updates status, attempts, http_status_code, error_message, next_retry_at |

**Business Logic Dependencies:**

```
Services:
- webhook_delivery_service.go: Webhook dispatch and retry logic

Workers:
- Webhook retry worker (cron job): Processes pending deliveries
```

**Webhook Delivery Flow:**

```
1. Payment event occurs (e.g., payment.completed)
2. WebhookDeliveryService.Dispatch():
   ├─> ListActiveWebhooksByEvent(agent_id, 'payment.completed')
   ├─> For each webhook subscription:
   │   ├─> CreateWebhookDelivery(subscription_id, payload, status='pending')
   │   └─> Attempt delivery (async):
   │       ├─> POST webhook_url with payload + HMAC signature
   │       ├─> If SUCCESS (200-299):
   │       │   └─> UpdateWebhookDeliveryStatus(id, status='succeeded', delivered_at=now)
   │       └─> If FAILURE:
   │           └─> UpdateWebhookDeliveryStatus(id, status='pending', attempts++, next_retry_at=calculate_backoff)
   └─> Return

Retry Worker (runs every 5 minutes):
  ├─> ListPendingWebhookDeliveries(next_retry_at <= now, limit=100)
  ├─> For each delivery:
  │   ├─> Attempt delivery
  │   ├─> If SUCCESS: Mark succeeded
  │   └─> If FAILURE:
  │       ├─> If attempts < max_attempts: Schedule retry with backoff
  │       └─> Else: Mark failed permanently
  └─> Return

Retry Schedule:
  - Attempt 1: Immediately (T+0)
  - Attempt 2: +5 minutes (T+5m)
  - Attempt 3: +15 minutes (T+20m)
  - Attempt 4: +1 hour (T+1h20m)
  - Attempt 5: +6 hours (T+7h20m)
  - After max_attempts: status='failed', delivery abandoned
```

**Performance Characteristics:**

- **Query Frequency:**
  - Writes: **VERY HIGH** (every payment event creates delivery)
  - Reads: **MEDIUM** (retry worker polls periodically)
- **Caching Strategy:** **NO CACHING** (delivery state changes frequently)
- **Index Usage:**
  - `subscription_id`: Delivery history lookup
  - `status`, `next_retry_at`: Pending deliveries query
- **Estimated Traffic:**
  - Reads: 100-200 TPS (retry worker)
  - Writes: 1000+ TPS (event generation)

**Relationships:**

```
webhook_subscriptions (1) ──< (many) webhook_deliveries
```

---

### 11. audit_logs

**Purpose:** Comprehensive audit trail of all system actions. Partitioned by month for performance. Supports compliance and security investigations.

**Schema Overview:**
- **Partitioned Table:** Partitioned by `performed_at` (monthly partitions)
- **Actor:** `actor_type` (admin, service, system), `actor_id`, `actor_name`
- **Action:** `action` (create_payment, update_merchant, etc.), `entity_type`, `entity_id`
- **Changes:** `changes` (JSONB) - Before/after values
- **Context:** `ip_address`, `user_agent`, `request_id`
- **Result:** `success` (boolean), `error_message`

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| List Audit Logs | **LOW** | Admin dashboard, filters | `ListAuditLogs(actor_type, action, entity_type, date_range, limit, offset)` |
| Get by Entity | **LOW** | Entity history | `GetAuditLogsByEntity(entity_type, entity_id, limit)` |
| Get by Actor | **LOW** | User activity | `GetAuditLogsByActor(actor_type, actor_id, limit)` |
| Count Logs | **LOW** | Statistics | `CountAuditLogs(filters)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Create | **VERY HIGH** | All state-changing operations | Logs action with context |
| Delete Old Logs | **LOW** | Cron job (retention policy) | Deletes logs older than retention period (default 90 days) |

**Business Logic Dependencies:**

```
Middleware:
- Audit logging middleware (logs all API requests)

Services:
- All services log state changes

Handlers:
- cron/audit_cleanup_handler.go: Retention policy enforcement
```

**Audit Logging Flow:**

```
1. State-changing operation occurs (e.g., CreatePayment)
2. Audit Middleware/Service:
   ├─> Extract context: actor (service/admin), IP, user_agent, request_id
   ├─> Capture changes: before/after state (JSONB)
   ├─> CreateAuditLog():
   │   ├─> actor_type='service', actor_id=service_id
   │   ├─> action='create_payment', entity_type='transaction', entity_id=txn_id
   │   ├─> changes={ "before": null, "after": {...} }
   │   ├─> success=true (or false if error)
   │   └─> INSERT INTO audit_logs (routes to correct partition by performed_at)
   └─> Continue operation
3. Async: Partition maintenance creates new monthly partitions

Cron Job - Audit Cleanup (runs daily at 2 AM UTC):
  ├─> Calculate cutoff_date = now - retention_days (default 90)
  ├─> DeleteOldAuditLogs(cutoff_date)
  │   └─> DELETE FROM audit_logs WHERE performed_at < cutoff_date
  └─> Drop old partitions (if all data deleted)
```

**Performance Characteristics:**

- **Query Frequency:**
  - Writes: **VERY HIGH** (all state changes logged)
    - At 1000 TPS operations: ~1000 audit writes/sec
  - Reads: **LOW** (admin debugging, compliance audits)
- **Caching Strategy:** **NO CACHING** (append-only, no reads during normal ops)
- **Partitioning:**
  - Monthly partitions (e.g., audit_logs_2025_01, audit_logs_2025_02)
  - Automatic partition pruning via retention policy
  - Index per partition for fast queries
- **Index Usage:**
  - `idx_audit_logs_actor`: Actor-based queries
  - `idx_audit_logs_action`: Action-based queries
  - `idx_audit_logs_entity`: Entity history queries
  - `idx_audit_logs_ip`: IP-based investigation
- **Estimated Traffic:**
  - Reads: <10 TPS
  - Writes: 1000+ TPS

**Partitioning Strategy:**

```sql
-- Parent table (partitioned by performed_at)
CREATE TABLE audit_logs (...) PARTITION BY RANGE (performed_at);

-- Monthly partitions
CREATE TABLE audit_logs_2025_01 PARTITION OF audit_logs
  FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

-- New partitions created automatically (migration 021 adds 2025 partitions)
-- Old partitions dropped after retention period
```

**Relationships:**

```
admins (1) ──< (many) audit_logs (actor)
services (1) ──< (many) audit_logs (actor)
(No FKs - audit_logs uses actor_id/entity_id as strings for flexibility)
```

---

### 12. rate_limit_buckets

**Purpose:** Token bucket implementation for rate limiting. Tracks token consumption per service (or service+merchant).

**Schema Overview:**
- **Primary Key:** `bucket_key` (VARCHAR) - Format: "service_id:merchant_id" or "service_id"
- **Tokens:** `tokens` (INTEGER) - Remaining tokens in bucket
- **Last Refill:** `last_refill` (TIMESTAMP) - When bucket was last refilled

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Consume Token | **VERY HIGH** | Atomic token consumption | `ConsumeRateLimitToken(bucket_key, initial_tokens)` |
| Get Bucket | **LOW** | Check current state | `GetRateLimitBucket(bucket_key)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Consume Token | **VERY HIGH** | Every API request | UPSERT with atomic decrement |
| Refill Bucket | **HIGH** | Token bucket algorithm | Sets tokens=max, last_refill=now |
| Cleanup Old Buckets | **LOW** | Cron job | DELETE WHERE last_refill < now - 1 hour |

**Business Logic Dependencies:**

```
Middleware:
- Rate limiting middleware (token bucket algorithm)
```

**Token Bucket Algorithm:**

```
1. API request arrives
2. Rate Limit Middleware:
   ├─> Extract: service_id (from JWT)
   ├─> Get service rate limit: GetServiceRateLimit(service_id)
   │   └─> Returns: requests_per_second=100, burst_limit=200
   ├─> Build bucket_key = service_id (or service_id:merchant_id)
   ├─> ConsumeRateLimitToken(bucket_key, initial_tokens=200):
   │   └─> UPSERT rate_limit_buckets:
   │       - If NOT EXISTS: INSERT (bucket_key, tokens=200, last_refill=now)
   │       - If EXISTS:
   │         ├─> Calculate refill: tokens_to_add = (now - last_refill) * requests_per_second
   │         ├─> New tokens = MIN(tokens + tokens_to_add, burst_limit)
   │         └─> UPDATE SET tokens = GREATEST(new_tokens - 1, 0), last_refill=now
   │         └─> RETURNING tokens
   ├─> If tokens > 0: Allow request
   └─> Else: Return 429 Too Many Requests

Cleanup Cron (runs hourly):
  └─> CleanupOldRateLimitBuckets()
      └─> DELETE FROM rate_limit_buckets WHERE last_refill < now - 1 hour
```

**Performance Characteristics:**

- **Query Frequency:** **VERY HIGH** (every API request)
  - At 1000 TPS: ~1000 token consumption queries/sec
- **Caching Strategy:** **NO CACHING** (requires atomic operations)
- **Index Usage:**
  - `PRIMARY KEY (bucket_key)`: Fast bucket lookup
- **Estimated Traffic:**
  - Reads+Writes: 1000+ TPS (atomic UPSERT)

**Relationships:**

```
services (1) ──< (many) rate_limit_buckets (via bucket_key)
(No FK - bucket_key is a string for flexibility)
```

---

### 13. jwt_blacklist

**Purpose:** Emergency revocation of JWT tokens. Stores revoked tokens until expiration.

**Schema Overview:**
- **Primary Key:** `jti` (VARCHAR) - JWT ID (unique token identifier)
- **Context:** `service_id`, `merchant_id`
- **Expiration:** `expires_at` (TIMESTAMP) - When JWT naturally expires
- **Audit:** `blacklisted_by` (admin_id), `reason`

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| Is Blacklisted | **VERY HIGH** | JWT validation | `IsJWTBlacklisted(jti)` |
| Blacklist JWT | **VERY RARE** | Admin revokes token | `BlacklistJWT(jti, service_id, merchant_id, expires_at, reason)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Blacklist JWT | **VERY RARE** | Emergency revocation | INSERT into blacklist |
| Cleanup Expired | **LOW** | Cron job | DELETE WHERE expires_at < now |

**Business Logic Dependencies:**

```
Middleware:
- JWT authentication middleware (checks blacklist)

Handlers:
- admin/service_handler.go: JWT revocation API
```

**JWT Blacklist Flow:**

```
1. JWT Middleware validates request:
   ├─> Parse JWT, extract jti (JWT ID claim)
   ├─> IsJWTBlacklisted(jti):
   │   └─> SELECT EXISTS(SELECT 1 FROM jwt_blacklist WHERE jti=$1 AND expires_at > NOW())
   ├─> If blacklisted: Return 401 Unauthorized
   └─> Else: Continue validation

Emergency Revocation (Admin):
  ├─> Admin calls /admin/blacklist-jwt
  ├─> BlacklistJWT(jti, service_id, merchant_id, expires_at, reason)
  │   └─> INSERT INTO jwt_blacklist
  └─> Token immediately invalid on next request

Cleanup Cron (runs daily):
  └─> CleanupExpiredBlacklist()
      └─> DELETE FROM jwt_blacklist WHERE expires_at < NOW()
```

**Performance Characteristics:**

- **Query Frequency:** **VERY HIGH** (every JWT authentication)
  - At 1000 TPS: ~1000 blacklist checks/sec
- **Caching Strategy:**
  - Could cache non-blacklisted JTIs (negative cache)
  - Currently NO CACHING (ensure security)
- **Index Usage:**
  - `PRIMARY KEY (jti)`: Fast blacklist check
  - `idx_jwt_blacklist_expires`: Cleanup query
- **Estimated Traffic:**
  - Reads: 1000+ TPS
  - Writes: <1 per month (emergency only)

**Relationships:**

```
services (1) ──< (many) jwt_blacklist
admins (1) ──< (many) jwt_blacklist (blacklisted_by)
```

---

### 14. epx_ip_whitelist

**Purpose:** Security whitelist for EPX gateway callback IPs. Validates EPX callback authenticity.

**Schema Overview:**
- **Primary Key:** `id` (SERIAL)
- **IP Address:** `ip_address` (INET, UNIQUE)
- **Metadata:** `description`, `added_by` (admin_id)
- **Status:** `is_active` (boolean)

**Primary Access Patterns:**

| Query Type | Frequency | Access Pattern | Query Example |
|-----------|-----------|----------------|---------------|
| List Active IPs | **HIGH** | Callback validation | `ListActiveIPWhitelist()` |
| Get IP Entry | **LOW** | IP management | `GetIPWhitelistEntry(ip_address)` |

**Write Patterns:**

| Operation | Frequency | Trigger | Business Logic |
|-----------|-----------|---------|----------------|
| Add IP | **VERY RARE** | EPX adds new IP | Admin adds IP to whitelist |
| Deactivate IP | **VERY RARE** | EPX retires IP | Sets is_active=false |

**Business Logic Dependencies:**

```
Middleware:
- EPX callback authentication middleware

Handlers:
- payment/browser_post_callback_handler.go: Validates callback IP
```

**EPX Callback Validation:**

```
1. EPX sends callback to /callback/browser-post
2. Callback Middleware:
   ├─> Extract client IP from request
   ├─> ListActiveIPWhitelist()
   │   └─> SELECT ip_address FROM epx_ip_whitelist WHERE is_active=true
   ├─> Check if client IP in whitelist
   ├─> If NOT in whitelist: Return 403 Forbidden
   └─> Else: Continue to callback handler
3. Callback Handler processes EPX response
```

**Performance Characteristics:**

- **Query Frequency:** **HIGH** (every EPX callback)
  - At ~100 callbacks/sec: ~100 whitelist checks/sec
- **Caching Strategy:**
  - **IN-MEMORY CACHE** (IP list cached, refresh every 5 minutes)
  - Very small dataset (<10 IPs)
- **Index Usage:**
  - `ip_address UNIQUE`: Fast IP lookup
  - `is_active`: Filter active IPs
- **Estimated Traffic:**
  - Reads: 100 TPS (before cache), ~1 TPS (after cache)
  - Writes: <1 per year (very rare)

**Relationships:**

```
admins (1) ──< (many) epx_ip_whitelist (added_by)
```

---

## Data Flow Diagrams

### Payment Transaction Flow

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ POST /payment.sale
       │ (merchant_id, amount, payment_method_id/token)
       ▼
┌────────────────────────────────────────────────────────────┐
│ JWT Auth Middleware                                        │
│ ├─> GetServiceByServiceID(service_id)                     │
│ ├─> CheckServiceMerchantAccess(service_id, merchant_id)   │
│ └─> Verify JWT signature                                  │
└──────┬─────────────────────────────────────────────────────┘
       │ Authorized
       ▼
┌────────────────────────────────────────────────────────────┐
│ PaymentService.Sale()                                      │
│ ├─> GetMerchantByID(merchant_id) [CACHE HIT 90-95%]      │
│ │   └─> MerchantCredentialCache.Get()                    │
│ ├─> resolvePaymentToken(payment_method_id)               │
│ │   └─> PaymentMethodCache.Get(pm_id) [CACHE HIT 90%]   │
│ │       └─> Validate: is_active, is_verified, expiration │
│ ├─> CreateTransaction(id=idempotency_key, ...)           │
│ │   └─> INSERT INTO transactions ON CONFLICT DO NOTHING  │
│ ├─> ServerPost.ProcessTransaction(EPX)                   │
│ │   └─> EPX returns: auth_resp, auth_code, auth_guid     │
│ └─> Update transaction with EPX response                 │
│     └─> MarkPaymentMethodUsed(pm_id)                     │
└──────┬─────────────────────────────────────────────────────┘
       │ Transaction Created
       ▼
┌────────────────────────────────────────────────────────────┐
│ Webhook Dispatch (Async)                                  │
│ ├─> ListActiveWebhooksByEvent('payment.completed')       │
│ ├─> CreateWebhookDelivery(subscription_id, payload)      │
│ └─> Attempt delivery with retry                          │
└──────┬─────────────────────────────────────────────────────┘
       │
       ▼
┌────────────────────────────────────────────────────────────┐
│ Audit Log (Async)                                         │
│ └─> CreateAuditLog(actor, action, entity, changes)       │
└────────────────────────────────────────────────────────────┘
```

### ACH Verification Flow

```
Day 0: Store ACH Account
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ POST /payment-method.store-ach
       │ (routing, account, customer info)
       ▼
┌────────────────────────────────────────────────────────────┐
│ PaymentMethodService.StoreACHAccount()                    │
│ ├─> GetMerchantByID(merchant_id)                         │
│ ├─> ServerPost.ProcessTransaction(EPX Pre-Note CKC0)     │
│ │   └─> EPX returns: auth_guid (BRIC)                    │
│ └─> CreatePaymentMethod():                               │
│     ├─> bric = auth_guid                                 │
│     ├─> verification_status = 'pending'                  │
│     ├─> is_active = false                                │
│     ├─> is_verified = false                              │
│     └─> prenote_transaction_id = transaction_id          │
└────────────────────────────────────────────────────────────┘

Day 3: Cron Job Auto-Verification
┌────────────────────────────────────────────────────────────┐
│ POST /cron/verify-ach (Cloud Scheduler)                  │
└──────┬─────────────────────────────────────────────────────┘
       │ X-Cron-Secret: xxx
       ▼
┌────────────────────────────────────────────────────────────┐
│ ACHVerificationHandler.VerifyACH()                        │
│ ├─> Calculate cutoff_date = now - 3 days                 │
│ ├─> FindEligibleACHForVerification(cutoff_date, 100)     │
│ │   └─> SELECT * FROM customer_payment_methods          │
│ │       WHERE payment_type='ach'                         │
│ │         AND verification_status='pending'              │
│ │         AND created_at <= cutoff_date                  │
│ ├─> For each payment_method:                             │
│ │   └─> VerifyACHPaymentMethod(pm_id):                  │
│ │       └─> UPDATE customer_payment_methods SET          │
│ │           verification_status='verified',              │
│ │           is_verified=true,                            │
│ │           is_active=true,                              │
│ │           verified_at=NOW()                            │
│ │       WHERE id=pm_id AND verification_status='pending' │
│ └─> Return: verified=95, skipped=0, errors=[]            │
└────────────────────────────────────────────────────────────┘
```

### Subscription Billing Flow

```
Cron Job: Recurring Billing
┌────────────────────────────────────────────────────────────┐
│ POST /cron/process-billing (Cloud Scheduler - Hourly)    │
└──────┬─────────────────────────────────────────────────────┘
       │ X-Cron-Secret: xxx
       ▼
┌────────────────────────────────────────────────────────────┐
│ BillingHandler.ProcessBilling()                           │
└──────┬─────────────────────────────────────────────────────┘
       │ Calls SubscriptionService.ProcessDueBilling()
       ▼
┌────────────────────────────────────────────────────────────┐
│ SubscriptionService.ProcessDueBilling(now, batch=100)     │
│ ├─> ListSubscriptionsDueForBilling(next_billing_date<=now)│
│ │   └─> SELECT * FROM subscriptions                      │
│ │       WHERE status='active' AND next_billing_date<=now  │
│ │       ORDER BY next_billing_date ASC LIMIT 100         │
│ └─> For each subscription:                                │
│     ├─> GetPaymentMethod(payment_method_id)              │
│     │   └─> PaymentMethodCache.Get() [CACHE HIT ~90%]   │
│     ├─> Validate: is_active, is_verified                 │
│     ├─> PaymentService.Sale(amount, pm_id, sub_id)       │
│     │   └─> Creates transaction linked to subscription   │
│     ├─> If transaction.status='approved':                │
│     │   └─> UpdateSubscriptionBilling():                 │
│     │       ├─> next_billing_date = calculate_next()     │
│     │       ├─> failure_retry_count = 0                  │
│     │       └─> status = 'active'                        │
│     └─> If transaction.status='declined':                │
│         └─> IncrementSubscriptionFailureCount():         │
│             ├─> failure_retry_count++                    │
│             └─> If count >= max_retries:                 │
│                 └─> status='past_due'                    │
└────────────────────────────────────────────────────────────┘
```

---

## Performance Summary

### High-Traffic Tables (>100 TPS)

| Table | Est. Reads/sec | Est. Writes/sec | Cache Hit Rate | Primary Bottleneck |
|-------|----------------|-----------------|----------------|-------------------|
| **transactions** | 2000+ | 1000+ | N/A (no cache) | Write throughput |
| **customer_payment_methods** | 40-80 (cached) | 100-200 | 90-95% | MarkPaymentMethodUsed |
| **merchants** | 50-100 (cached) | <1 | 90-95% | Cache invalidation |
| **services** | 10-50 (cached) | <1 | 95%+ | JWT validation |
| **service_merchants** | 1000+ | <1 | Could cache | Authorization checks |
| **rate_limit_buckets** | 1000+ | 1000+ | N/A (atomic) | Token consumption |
| **jwt_blacklist** | 1000+ | <1 | Could cache | Blacklist checks |
| **webhook_subscriptions** | 1000+ | <1 | Could cache | Webhook lookup |
| **webhook_deliveries** | 100-200 | 1000+ | N/A | Delivery writes |
| **audit_logs** | <10 | 1000+ | N/A (append) | Write throughput |

### Medium-Traffic Tables (10-100 TPS)

| Table | Est. Reads/sec | Est. Writes/sec | Notes |
|-------|----------------|-----------------|-------|
| **subscriptions** | 50-100 | 10-20 | Cron job batch queries |
| **epx_ip_whitelist** | ~1 (cached) | <1 | Callback validation |

### Low-Traffic Tables (<10 TPS)

| Table | Usage | Notes |
|-------|-------|-------|
| **chargebacks** | <1 TPS | Rare events |
| **admins** | <1 TPS | Admin operations |

### Caching Summary

| Cache | Tables Cached | TTL | Hit Rate | Load Reduction |
|-------|---------------|-----|----------|----------------|
| **MerchantCredentialCache** | merchants + MAC secrets | 5-15 min | 90-95% | 70% DB load |
| **PaymentMethodCache** | customer_payment_methods | 5-10 min | 90-95% | 60% faster |
| **Service Public Keys** | services (public_key) | 5-15 min | 95%+ | In-memory |
| **EPX IP Whitelist** | epx_ip_whitelist | 5 min | 99%+ | In-memory (<10 IPs) |

### Index Performance

All high-traffic queries use indexes:
- **merchants**: `idx_merchants_slug`, `idx_merchants_is_active`
- **customer_payment_methods**: `idx_customer_payment_methods_merchant_customer`, `idx_ach_verification`
- **transactions**: `idx_transactions_tran_nbr`, `idx_transactions_parent_id`, `idx_transaction_listing_indexes`
- **subscriptions**: `idx_subscriptions_next_billing_date` (filtered WHERE status='active')
- **service_merchants**: Composite PK `(service_id, merchant_id)`

### Partitioning

- **audit_logs**: Monthly partitions (automatic pruning via retention policy)
- **Future consideration**: Time-based partitioning for transactions (if volume exceeds millions/day)

---

## Recommendations

### 1. Caching Improvements

**Current State:**
- ✅ MerchantCredentialCache (70% load reduction)
- ✅ PaymentMethodCache (60% faster)
- ❌ No cache for service_merchants (authorization checks)

**Recommendations:**
```go
// Add ServiceMerchantCache for authorization checks
// Cache key: "service:{service_id}:merchant:{merchant_id}"
// TTL: 5-15 minutes
// Expected hit rate: 90-95%
// Load reduction: ~900 DB queries/sec at 1000 TPS
```

### 2. Read Replicas

**High-read tables that could benefit:**
- `transactions` (listing, history)
- `customer_payment_methods` (listing)
- `audit_logs` (admin queries)

**Recommendation:** Configure read replica for reporting/analytics queries

### 3. Connection Pooling

**Current bottleneck:** Write-heavy workload (transactions, audit_logs, webhook_deliveries)

**Recommendation:**
```
Min Pool Size: 10
Max Pool Size: 50 (or 2x CPU cores)
Max Idle Time: 30 seconds
Connection Lifetime: 30 minutes
```

### 4. Monitoring Alerts

**Key metrics to monitor:**
- Transaction write latency (p95, p99)
- Cache hit rates (merchant, payment_method)
- Rate limit bucket contention
- Audit log partition size
- Webhook delivery retry queue depth

### 5. Future Scaling

**When to partition transactions:**
- Volume: >10M transactions/day
- Strategy: Time-based partitioning (daily or weekly)
- Benefit: Faster queries, easier archival

**When to add caching layer (Redis):**
- Cache hit rate drops below 85%
- Application memory exceeds limits
- Need distributed caching for horizontal scaling

---

## Appendix: Query Performance Benchmarks

### Merchant Lookup (Cached)

```
Without Cache:
  GetMerchantByID: ~5-10ms (DB query)
  At 1000 TPS: ~1000 DB queries/sec

With MerchantCredentialCache:
  Cache Hit (90-95%): <1ms (memory)
  Cache Miss (5-10%): ~5-10ms (DB query + cache store)
  At 1000 TPS: ~50-100 DB queries/sec (10x reduction)
```

### Payment Method Lookup (Cached)

```
Without Cache:
  GetPaymentMethodByID: ~5-10ms (DB query)
  At 800 PM lookups/sec: ~800 DB queries/sec

With PaymentMethodCache:
  Cache Hit (90-95%): <1ms (memory)
  Cache Miss (5-10%): ~5-10ms (DB query + cache store)
  At 800 PM lookups/sec: ~40-80 DB queries/sec (10x reduction)
```

### Transaction Tree Query

```
GetTransactionTree (CTE):
  Simple chain (AUTH→CAPTURE): ~10-15ms
  Complex chain (AUTH→CAPTURE→REFUND×3): ~15-25ms
  Deep chain (5+ levels): ~25-50ms

Optimization: Depth limit of 100 prevents DoS
```

### Audit Log Write Performance

```
Write to Partitioned Table:
  Single INSERT: ~2-5ms
  Batch INSERT (100 rows): ~10-20ms
  Partition routing: Automatic (by performed_at)

Monthly Partition Maintenance:
  Create new partition: ~50-100ms
  Drop old partition: ~100-500ms (depends on size)
```

---

**Document Version:** 1.0
**Last Updated:** 2025-11-23
**Maintained By:** Development Team

For questions or updates, contact the platform engineering team.
