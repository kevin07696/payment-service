# Database Schema Reference

**Auto-Generated:** $(date '+%Y-%m-%d %H:%M:%S')
**Source:** Goose migrations in `internal/db/migrations`

---

## Table of Contents

1. [Overview](#overview)
2. [Schema Design](#schema-design)
3. [Core Tables](#core-tables)
4. [Relationships](#relationships)
5. [Indexes](#indexes)
6. [Migrations](#migrations)
7. [Performance Considerations](#performance-considerations)

---

## Overview

**Database:** PostgreSQL 15+
**Migration Tool:** Goose
**Connection Pool:** pgxpool
**Query Builder:** sqlc (type-safe SQL)

### Design Principles

1. **Multi-Tenancy:** All tables include `merchant_id` for data isolation
2. **Immutability:** Transactions are append-only (no updates to financial records)
3. **Idempotency:** `idempotency_key` prevents duplicate operations
4. **Audit Trail:** All tables have `created_at` and `updated_at`
5. **Soft Deletes:** Use `deleted_at` instead of hard deletes (where applicable)

---

## Schema Design

### Entity Relationship Diagram

```
┌─────────────┐
│  merchants  │
└─────────────┘
       │
       │ 1:N
       ↓
┌─────────────┐         ┌──────────────────┐
│  services   │─────────│ service_secrets  │
└─────────────┘  1:N    └──────────────────┘
       │
       │ 1:N
       ↓
┌─────────────────┐
│ payment_methods │
└─────────────────┘
       │
       │ 1:N
       ↓
┌──────────────┐          ┌────────────────┐
│ transactions │──────────│ chargebacks    │
└──────────────┘   1:N    └────────────────┘
       │
       │ 1:N (parent_transaction_id)
       ↓
┌──────────────┐
│ transactions │  (REFUND, VOID, CAPTURE reference parent)
└──────────────┘
       │
       │ N:1
       ↓
┌──────────────────┐
│  subscriptions   │
└──────────────────┘
```

---

## Core Tables

### merchants

**Purpose:** Multi-tenant merchant accounts (e.g., "Acme Corp", "Widget Inc")

**Key Fields:**
- `id` (UUID, PK) - Merchant identifier
- `name` (TEXT) - Business name
- `email` (TEXT) - Contact email
- `created_at`, `updated_at` - Audit timestamps

**Business Logic:**
- One merchant can have multiple services
- All transactions scoped to merchant_id
- Soft delete with `deleted_at`

---

### services

**Purpose:** OAuth2-style service accounts for API authentication

**Key Fields:**
- `id` (UUID, PK) - Service identifier (used in JWT claims)
- `merchant_id` (UUID, FK) - Parent merchant
- `name` (TEXT) - Service name (e.g., "WordPress Plugin", "Mobile App")
- `public_key_pem` (TEXT) - RSA public key for JWT verification

**Business Logic:**
- Each service has RSA keypair (private key stored by client)
- JWT tokens signed with private key, verified with public key
- Multiple services per merchant for different applications

**Related:**
- `service_secrets` - Stores EPX credentials per service

---

### payment_methods

**Purpose:** Tokenized payment instruments (credit cards, ACH accounts)

**Key Fields:**
- `id` (UUID, PK) - Payment method identifier
- `merchant_id` (UUID, FK) - Owner merchant
- `customer_id` (TEXT, nullable) - Customer identifier
- `type` (TEXT) - "credit_card" or "ach"
- `epx_bric` (TEXT) - EPX tokenization BRIC (AUTH_GUID from STORAGE txn)
- `last_4` (TEXT) - Last 4 digits for display
- `expiry_month`, `expiry_year` (INT, nullable) - Card expiration

**Business Logic:**
- Stores EPX BRIC (tokenized reference), NOT raw card data (PCI compliance)
- `customer_id` nullable for guest checkouts
- Used for recurring billing and saved payment methods

**Indexes:**
- `idx_payment_methods_merchant_customer` - Query by merchant + customer
- `idx_payment_methods_epx_bric` - Lookup by EPX token

---

### transactions

**Purpose:** Immutable financial transaction log

**Key Fields:**
- `id` (UUID, PK) - Transaction identifier
- `merchant_id` (UUID, FK) - Owner merchant
- `parent_transaction_id` (UUID, FK, nullable) - Links REFUND/VOID/CAPTURE to original
- `customer_id` (TEXT, nullable) - Customer identifier
- `order_id` (VARCHAR(255), nullable) - Merchant's external order/invoice ID
- `subscription_id` (UUID, FK, nullable) - Links recurring billing transactions
- `payment_method_id` (UUID, FK, nullable) - Payment method used
- `amount_cents` (BIGINT) - Amount in cents (2999 = $29.99)
- `currency` (TEXT) - ISO 4217 code ("USD")
- `type` (TEXT) - Transaction type: "SALE", "AUTH", "CAPTURE", "REFUND", "VOID", "PRE_NOTE", "STORAGE"
- `payment_method_type` (TEXT) - "credit_card" or "ach"
- `auth_guid` (TEXT) - EPX transaction identifier (BRIC)
- `auth_resp` (TEXT) - EPX response code ("00" = approved)
- `auth_code` (TEXT) - EPX authorization code
- `status` (TEXT) - "approved" or "declined"
- `idempotency_key` (TEXT, unique per merchant) - Prevents duplicate charges

**Business Logic:**
- **Immutable:** Never UPDATE, only INSERT
- **Parent-Child:** REFUND/VOID/CAPTURE reference parent transaction
- **Idempotency:** `idempotency_key` unique constraint prevents duplicates
- **Group State:** Use `parent_transaction_id` to query transaction chains

**Transaction Lifecycle Examples:**

```sql
-- Example 1: SALE → REFUND
-- Original charge
INSERT INTO transactions (type, amount_cents, auth_resp, parent_transaction_id)
VALUES ('SALE', 10000, '00', NULL);  -- tx_001

-- Partial refund
INSERT INTO transactions (type, amount_cents, auth_resp, parent_transaction_id)
VALUES ('REFUND', 5000, '00', 'tx_001');  -- tx_002

-- Example 2: AUTH → CAPTURE → REFUND
-- Authorization only
INSERT INTO transactions (type, amount_cents, auth_resp, parent_transaction_id)
VALUES ('AUTH', 10000, '00', NULL);  -- tx_003

-- Capture funds
INSERT INTO transactions (type, amount_cents, auth_resp, parent_transaction_id)
VALUES ('CAPTURE', 10000, '00', 'tx_003');  -- tx_004

-- Refund captured amount
INSERT INTO transactions (type, amount_cents, auth_resp, parent_transaction_id)
VALUES ('REFUND', 10000, '00', 'tx_004');  -- tx_005
```

**Indexes:**
- `idx_transactions_merchant_id` - Query by merchant
- `idx_transactions_customer_id` - Query by customer
- `idx_transactions_parent_id` - Transaction chains
- `idx_transactions_subscription_id` - Recurring billing
- `idx_transactions_idempotency_key` - Idempotency lookups
- `idx_transactions_created_at` - Time-based queries
- **Critical:** `idx_ach_verification_pending` - High-performance ACH verification query

---

### subscriptions

**Purpose:** Recurring billing schedules

**Key Fields:**
- `id` (UUID, PK) - Subscription identifier
- `merchant_id` (UUID, FK) - Owner merchant
- `customer_id` (TEXT) - Customer identifier
- `payment_method_id` (UUID, FK) - Payment method to charge
- `amount_cents` (BIGINT) - Recurring charge amount
- `interval_type` (TEXT) - "daily", "weekly", "monthly", "yearly"
- `next_billing_date` (TIMESTAMP) - When to charge next
- `status` (TEXT) - "active", "paused", "cancelled"

**Business Logic:**
- Cron job queries `next_billing_date <= NOW() AND status = 'active'`
- Creates new transaction for each billing cycle
- Links transactions via `subscription_id` foreign key

---

### chargebacks

**Purpose:** Dispute tracking for transactions

**Key Fields:**
- `id` (UUID, PK) - Chargeback identifier
- `transaction_id` (UUID, FK) - Disputed transaction
- `merchant_id` (UUID, FK) - Owner merchant
- `amount_cents` (BIGINT) - Disputed amount
- `reason` (TEXT) - Chargeback reason code
- `status` (TEXT) - "pending", "won", "lost"
- `evidence_text` (TEXT, nullable) - Merchant's response

**Business Logic:**
- One-to-many: transaction can have multiple chargebacks
- EPX webhook creates chargeback records
- Merchant provides evidence to fight dispute

---

## Relationships

### One-to-Many Relationships

```sql
-- Merchant → Services
SELECT s.* FROM services s
WHERE s.merchant_id = 'merchant-uuid';

-- Service → Payment Methods
SELECT pm.* FROM payment_methods pm
JOIN services s ON pm.merchant_id = s.merchant_id
WHERE s.id = 'service-uuid';

-- Subscription → Transactions
SELECT t.* FROM transactions t
WHERE t.subscription_id = 'sub-uuid'
ORDER BY t.created_at DESC;

-- Parent Transaction → Child Transactions (refunds, voids, captures)
SELECT t.* FROM transactions t
WHERE t.parent_transaction_id = 'parent-tx-uuid';
```

### Complex Queries

**Get Transaction Group (parent + all children):**
```sql
WITH RECURSIVE transaction_tree AS (
  -- Base case: get parent transaction
  SELECT * FROM transactions WHERE id = 'tx-uuid'

  UNION ALL

  -- Recursive case: get children
  SELECT t.* FROM transactions t
  INNER JOIN transaction_tree tt ON t.parent_transaction_id = tt.id
)
SELECT * FROM transaction_tree ORDER BY created_at;
```

**Calculate Subscription Revenue:**
```sql
SELECT
  s.id,
  s.customer_id,
  COUNT(t.id) as billing_count,
  SUM(t.amount_cents) as total_revenue_cents,
  AVG(t.amount_cents) as avg_charge_cents
FROM subscriptions s
LEFT JOIN transactions t ON t.subscription_id = s.id AND t.status = 'approved'
WHERE s.merchant_id = 'merchant-uuid'
GROUP BY s.id, s.customer_id;
```

---

## Indexes

### Performance-Critical Indexes

**ACH Verification Query (Migration 022):**
```sql
CREATE INDEX idx_ach_verification_pending ON transactions (
  merchant_id,
  next_billing_date,
  status
) WHERE type = 'PRE_NOTE' AND status = 'pending';
```
**Impact:** 100ms → <5ms for cron job queries

**Idempotency Lookups:**
```sql
CREATE UNIQUE INDEX idx_transactions_idempotency_key
ON transactions (merchant_id, idempotency_key);
```
**Purpose:** Fast duplicate detection, ensures exactly-once processing

**Customer Transaction History:**
```sql
CREATE INDEX idx_transactions_customer_id
ON transactions (merchant_id, customer_id, created_at DESC);
```
**Purpose:** Fast pagination for customer transaction history

---

## Migrations

### Migration 001: merchants

**File:** `001_merchants.sql`

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(255) UNIQUE NOT NULL,

    -- EPX Credentials
    cust_nbr VARCHAR(50) NOT NULL,
    merch_nbr VARCHAR(50) NOT NULL,
    dba_nbr VARCHAR(50) NOT NULL,
    terminal_nbr VARCHAR(50) NOT NULL,

    -- Secret Manager integration
    mac_secret_path VARCHAR(500) NOT NULL,  -- Path to MAC secret in secret manager

    -- Environment and status
    environment VARCHAR(20) NOT NULL DEFAULT 'production',  -- 'production', 'staging', 'test'
    is_active BOOLEAN NOT NULL DEFAULT true,
    name VARCHAR(255) NOT NULL,

    -- Soft delete and timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Index for lookups
CREATE INDEX IF NOT EXISTS idx_merchants_slug ON merchants(slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchants_environment ON merchants(environment) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchants_is_active ON merchants(is_active) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_merchants_is_active;
DROP INDEX IF EXISTS idx_merchants_environment;
DROP INDEX IF EXISTS idx_merchants_slug;
DROP TABLE IF EXISTS merchants;
-- +goose StatementEnd
```

---

### Migration 002: customer_payment_methods

**File:** `002_customer_payment_methods.sql`

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS customer_payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Multi-tenant: which merchant + which customer
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id UUID NOT NULL,

    -- ✅ BRIC token from EPX (AUTH_GUID from STORAGE transaction)
    -- Example: "0V703LH1HDL006J74W1"
    bric TEXT NOT NULL,

    -- Payment method type
    payment_type VARCHAR(20) NOT NULL,

    -- ✅ Display metadata (last 4 only, NEVER full numbers)
    last_four VARCHAR(4) NOT NULL,

    -- ✅ Credit card metadata (for display/UI purposes)
    card_brand VARCHAR(20),         -- "visa", "mastercard", "amex", "discover"
    card_exp_month INTEGER,         -- 1-12 (optional, for expiration warnings)
    card_exp_year INTEGER,          -- 2025, 2026, etc.

    -- ✅ ACH metadata (user-provided labels for display)
    bank_name VARCHAR(255),         -- "Chase", "Bank of America", etc.
    account_type VARCHAR(20),       -- "checking" or "savings"

    -- Status tracking
    is_default BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    is_verified BOOLEAN DEFAULT false,  -- For ACH pre-note verification

    -- Timestamps
    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ,

    CONSTRAINT check_payment_type CHECK (payment_type IN ('credit_card', 'ach')),
    CONSTRAINT check_card_exp_month CHECK (card_exp_month IS NULL OR (card_exp_month >= 1 AND card_exp_month <= 12)),
    CONSTRAINT check_account_type CHECK (account_type IS NULL OR account_type IN ('checking', 'savings')),
    CONSTRAINT unique_bric UNIQUE (merchant_id, customer_id, bric)
);

-- Indexes for performance
CREATE INDEX idx_customer_payment_methods_merchant_customer ON customer_payment_methods(merchant_id, customer_id);
CREATE INDEX idx_customer_payment_methods_merchant_id ON customer_payment_methods(merchant_id);
CREATE INDEX idx_customer_payment_methods_customer_id ON customer_payment_methods(customer_id);
CREATE INDEX idx_customer_payment_methods_payment_type ON customer_payment_methods(payment_type);
CREATE INDEX idx_customer_payment_methods_is_default ON customer_payment_methods(merchant_id, customer_id, is_default) WHERE is_default = true;
CREATE INDEX idx_customer_payment_methods_is_active ON customer_payment_methods(is_active) WHERE is_active = true;
CREATE INDEX idx_customer_payment_methods_deleted_at ON customer_payment_methods(deleted_at) WHERE deleted_at IS NOT NULL;

-- Update timestamp trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger for updated_at
CREATE TRIGGER update_customer_payment_methods_updated_at
    BEFORE UPDATE ON customer_payment_methods
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_customer_payment_methods_updated_at ON customer_payment_methods;
DROP TABLE IF EXISTS customer_payment_methods;
DROP FUNCTION IF EXISTS update_updated_at_column();
-- +goose StatementEnd
```

---

### Migration 003: transactions

**File:** `003_transactions.sql`

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY,  -- Client-provided UUID via idempotency_key (REQUIRED for all operations)

    -- Transaction parent relationship: tracks transaction chains (AUTH -> CAPTURE -> REFUND)
    -- CAPTURE references AUTH, REFUND references SALE/CAPTURE, VOID references AUTH/SALE
    parent_transaction_id UUID REFERENCES transactions(id) ON DELETE RESTRICT,

    -- Multi-tenant: which merchant owns this transaction
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,

    -- Customer identification (NULL for guest transactions)
    customer_id UUID,

    -- Transaction details (amount in cents to avoid floating point issues)
    amount_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    type VARCHAR(20) NOT NULL,    -- SALE, AUTH, CAPTURE, REFUND, VOID, STORAGE, DEBIT

    -- Payment method
    payment_method_type VARCHAR(20) NOT NULL,  -- credit_card, ach
    payment_method_id UUID REFERENCES customer_payment_methods(id) ON DELETE SET NULL,  -- Reference to saved payment method (NULL for guest)

    -- Optional subscription reference (for recurring billing transactions)
    subscription_id UUID,  -- Links transaction to subscription (NULL for one-time payments) - FK added after subscriptions table created

    -- EPX Gateway response fields (queryable columns only)
    -- EPX TRAN_NBR: Deterministic 10-digit numeric ID derived from UUID (for EPX API calls)
    tran_nbr TEXT,

    -- EPX AUTH_GUID (BRIC) returned from gateway for this transaction
    -- IMPORTANT: Each transaction can have its own BRIC token
    -- - AUTH transaction: gets initial BRIC from EPX
    -- - CAPTURE: uses AUTH's BRIC as input, gets new BRIC as output
    -- - REFUND: uses CAPTURE's BRIC as input, gets new BRIC as output
    -- This allows querying by BRIC and supports EPX reconciliation
    auth_guid TEXT,

    auth_resp VARCHAR(10),               -- EPX approval code ("00" = approved, "05" = declined, "12" = invalid) - source of truth for status. NULL = pending/failed
    auth_code VARCHAR(50),               -- Bank authorization code (e.g., "123456") - required for chargeback defense, NULL if declined
    auth_card_type VARCHAR(20),          -- Card brand ("V" = Visa, "M" = Mastercard, "A" = Amex, "D" = Discover) - used for fees/reporting, NULL for ACH

    -- Status: Transaction outcome auto-generated from auth_resp
    -- EPX: "00" = approved, anything else = declined
    -- pending = not sent to EPX yet, failed = system error before reaching EPX
    status VARCHAR(20) GENERATED ALWAYS AS (
        CASE
            WHEN auth_resp IS NULL AND processed_at IS NULL THEN 'pending'
            WHEN auth_resp IS NULL AND processed_at IS NOT NULL THEN 'failed'
            WHEN auth_resp = '00' THEN 'approved'
            ELSE 'declined'
        END
    ) STORED,

    -- Timestamp when EPX responded (callback received)
    processed_at TIMESTAMPTZ,

    -- Metadata (auth_resp_text, auth_avs, auth_cvv2, card_last_4, card_holder_name, description, EPX raw response, integration-specific data)
    -- auth_resp_text: Human-readable response message ("APPROVED", "INSUFFICIENT FUNDS") - display only
    -- auth_avs: Address verification ("Y" = match, "N" = no match, "U" = unavailable) - fraud scoring, not queried
    -- auth_cvv2: CVV verification ("M" = match, "N" = no match, "P" = not processed) - fraud scoring, not queried
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Timestamps
    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT transactions_amount_cents_non_negative CHECK (amount_cents >= 0),
    CONSTRAINT transactions_type_valid CHECK (type IN ('SALE', 'AUTH', 'CAPTURE', 'REFUND', 'VOID', 'STORAGE', 'DEBIT')),
    -- Simple CHECK constraint as defense-in-depth (detailed validation in application layer)
    -- SALE, AUTH, STORAGE, DEBIT = standalone (no parent)
    -- CAPTURE, REFUND, VOID = must have parent
    CONSTRAINT transactions_parent_relationship CHECK (
        (type IN ('SALE', 'AUTH', 'STORAGE', 'DEBIT') AND parent_transaction_id IS NULL)
        OR
        (type IN ('CAPTURE', 'REFUND', 'VOID') AND parent_transaction_id IS NOT NULL)
    )
);

-- Indexes for performance
CREATE INDEX idx_transactions_parent_id ON transactions(parent_transaction_id) WHERE parent_transaction_id IS NOT NULL;
CREATE INDEX idx_transactions_merchant_id ON transactions(merchant_id);
CREATE INDEX idx_transactions_merchant_customer ON transactions(merchant_id, customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_transactions_customer_id ON transactions(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_transactions_tran_nbr ON transactions(tran_nbr) WHERE tran_nbr IS NOT NULL;
CREATE INDEX idx_transactions_auth_guid ON transactions(auth_guid) WHERE auth_guid IS NOT NULL;
CREATE INDEX idx_transactions_payment_method_id ON transactions(payment_method_id) WHERE payment_method_id IS NOT NULL;
CREATE INDEX idx_transactions_subscription_id ON transactions(subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_deleted_at ON transactions(deleted_at) WHERE deleted_at IS NOT NULL;

-- Comments explaining key design decisions
COMMENT ON COLUMN transactions.parent_transaction_id IS 'Foreign key to parent transaction. CAPTURE→AUTH, REFUND→SALE/CAPTURE, VOID→AUTH/SALE. NULL for standalone transactions (SALE, AUTH, STORAGE, DEBIT). Detailed validation in application layer.';
COMMENT ON COLUMN transactions.amount_cents IS 'Amount in cents (e.g., $10.50 = 1050). Using BIGINT avoids floating point precision issues.';
COMMENT ON COLUMN transactions.status IS 'Auto-generated from auth_resp: pending (not sent), failed (error), approved (00), declined (non-00).';
COMMENT ON COLUMN transactions.processed_at IS 'Timestamp when EPX responded (callback received). NULL if pending or failed before reaching EPX.';
COMMENT ON COLUMN transactions.tran_nbr IS 'EPX TRAN_NBR: Deterministic 10-digit numeric ID derived from transaction UUID via FNV-1a hash. Used for all EPX API calls.';
COMMENT ON COLUMN transactions.auth_guid IS 'EPX AUTH_GUID (BRIC) for this specific transaction. Each transaction can have its own BRIC. CAPTURE uses AUTH BRIC as input but gets new BRIC. REFUND uses CAPTURE BRIC.';

-- Create subscriptions table
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id UUID NOT NULL,
    amount_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',

    -- Billing interval (e.g., 1 month, 2 weeks, 3 months)
    interval_value INTEGER NOT NULL DEFAULT 1,  -- 1, 2, 3, etc.
    interval_unit VARCHAR(10) NOT NULL DEFAULT 'month',  -- 'day', 'week', 'month', 'year'

    status VARCHAR(20) NOT NULL,  -- 'active', 'paused', 'cancelled', 'past_due'
    payment_method_id UUID NOT NULL REFERENCES customer_payment_methods(id) ON DELETE RESTRICT,  -- Cannot delete payment method with active subscriptions
    next_billing_date DATE NOT NULL,

    -- Failure handling
    failure_retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,

    -- Optional: EPX gateway subscription ID if EPX provides one
    gateway_subscription_id VARCHAR(255),

    metadata JSONB DEFAULT '{}'::jsonb,
    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cancelled_at TIMESTAMPTZ,

    CONSTRAINT subscriptions_amount_cents_positive CHECK (amount_cents > 0),
    CONSTRAINT subscriptions_retry_count_non_negative CHECK (failure_retry_count >= 0),
    CONSTRAINT subscriptions_interval_value_positive CHECK (interval_value > 0),
    CONSTRAINT subscriptions_interval_unit_valid CHECK (interval_unit IN ('day', 'week', 'month', 'year')),
    CONSTRAINT subscriptions_status_valid CHECK (status IN ('active', 'paused', 'cancelled', 'past_due'))
);

-- Indexes for subscriptions
CREATE INDEX idx_subscriptions_merchant_id ON subscriptions(merchant_id);
CREATE INDEX idx_subscriptions_merchant_customer ON subscriptions(merchant_id, customer_id);
CREATE INDEX idx_subscriptions_next_billing_date ON subscriptions(next_billing_date) WHERE status = 'active';
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_gateway_subscription_id ON subscriptions(gateway_subscription_id) WHERE gateway_subscription_id IS NOT NULL;
CREATE INDEX idx_subscriptions_deleted_at ON subscriptions(deleted_at) WHERE deleted_at IS NOT NULL;

-- Add foreign key constraint from transactions to subscriptions (now that subscriptions table exists)
ALTER TABLE transactions ADD CONSTRAINT transactions_subscription_id_fkey
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL;

-- Apply update trigger to tables (function defined in 002_customer_payment_methods.sql)
CREATE TRIGGER update_transactions_updated_at BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_subscriptions_updated_at BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_subscriptions_updated_at ON subscriptions;
DROP TRIGGER IF EXISTS update_transactions_updated_at ON transactions;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS transactions;
-- +goose StatementEnd
```

---

### Migration 004: chargebacks

**File:** `004_chargebacks.sql`

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chargebacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Link to specific transaction being disputed
    -- Can traverse to parent/child transactions via transactions.parent_transaction_id
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
    merchant_id VARCHAR(100) NOT NULL,  -- Denormalized for querying (matches merchants.id)
    customer_id VARCHAR(100),  -- Denormalized for querying (NULL for guest)

    -- North API fields
    case_number VARCHAR(255) UNIQUE NOT NULL, -- North's caseNumber (unique ID)
    dispute_date TIMESTAMPTZ NOT NULL, -- North's disputeDate
    chargeback_date TIMESTAMPTZ NOT NULL, -- North's chargebackDate
    chargeback_amount VARCHAR(255) NOT NULL, -- Amount being disputed (stored as string to preserve precision)
    currency VARCHAR(3) NOT NULL DEFAULT 'USD', -- ISO currency code
    reason_code VARCHAR(50) NOT NULL, -- North's reasonCode (e.g., "P22", "F10")
    reason_description TEXT, -- North's reasonDescription

    -- Our status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'new', -- 'new', 'pending', 'responded', 'won', 'lost', 'accepted'
    respond_by_date DATE, -- Deadline to respond (calculated or from North)
    response_submitted_at TIMESTAMPTZ, -- When we submitted evidence
    resolved_at TIMESTAMPTZ, -- When outcome was determined

    -- Evidence and response (URLs to S3/blob storage)
    evidence_files TEXT[], -- Array of blob storage URLs: ["s3://bucket/receipt.pdf", "s3://bucket/tracking.jpg"]
    response_notes TEXT, -- Our written response to dispute
    internal_notes TEXT, -- Internal team notes

    -- Store full North API response
    raw_data JSONB NOT NULL, -- Full North disputes API response for this case

    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chargebacks_status_valid CHECK (status IN ('new', 'pending', 'responded', 'won', 'lost', 'accepted'))
);

-- Indexes for performance
CREATE INDEX idx_chargebacks_transaction_id ON chargebacks(transaction_id);
CREATE INDEX idx_chargebacks_merchant_id ON chargebacks(merchant_id);
CREATE INDEX idx_chargebacks_merchant_customer ON chargebacks(merchant_id, customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_chargebacks_customer_id ON chargebacks(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_chargebacks_status ON chargebacks(status);
CREATE INDEX idx_chargebacks_case_number ON chargebacks(case_number);
CREATE INDEX idx_chargebacks_respond_by_date ON chargebacks(respond_by_date) WHERE status = 'pending';
CREATE INDEX idx_chargebacks_created_at ON chargebacks(created_at DESC);
CREATE INDEX idx_chargebacks_deleted_at ON chargebacks(deleted_at) WHERE deleted_at IS NOT NULL;

-- Trigger for updated_at
CREATE TRIGGER update_chargebacks_updated_at
    BEFORE UPDATE ON chargebacks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_chargebacks_updated_at ON chargebacks;
DROP TABLE IF EXISTS chargebacks;
-- +goose StatementEnd
```

---

### Migration 006: soft_delete_cleanup

**File:** `006_soft_delete_cleanup.sql`

```sql
-- +goose Up
-- +goose StatementBegin

-- Note: pg_cron extension scheduling is optional and should be set up separately in production
-- For local development, this function can be called manually or via application cron

-- Create function to permanently delete soft-deleted records older than 90 days
CREATE OR REPLACE FUNCTION cleanup_soft_deleted_records()
RETURNS void AS $$
DECLARE
    deleted_count INTEGER;
    total_deleted INTEGER := 0;
BEGIN
    -- Soft delete PENDING transactions older than 1 hour (abandoned checkouts)
    UPDATE transactions
    SET deleted_at = CURRENT_TIMESTAMP
    WHERE status = 'pending'
      AND created_at < NOW() - INTERVAL '1 hour'
      AND deleted_at IS NULL;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RAISE NOTICE 'Soft deleted % abandoned PENDING transactions', deleted_count;

    -- Transactions (older than 90 days)
    DELETE FROM transactions
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % transactions', deleted_count;

    -- Subscriptions (older than 90 days)
    DELETE FROM subscriptions
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % subscriptions', deleted_count;

    -- Chargebacks (older than 90 days)
    DELETE FROM chargebacks
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % chargebacks', deleted_count;

    -- Customer Payment Methods (older than 90 days)
    DELETE FROM customer_payment_methods
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % payment methods', deleted_count;

    -- Agent Credentials (older than 90 days)
    DELETE FROM agent_credentials
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % agent credentials', deleted_count;

    RAISE NOTICE 'Total permanently deleted records: %', total_deleted;
END;
$$ LANGUAGE plpgsql;

-- Note: For production with pg_cron extension, schedule the cleanup job with:
-- SELECT cron.schedule(
--     'cleanup-soft-deleted-records',
--     '0 2 * * *',
--     'SELECT cleanup_soft_deleted_records();'
-- );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Note: If pg_cron scheduling was set up, unschedule with:
-- SELECT cron.unschedule('cleanup-soft-deleted-records');

-- Drop the cleanup function
DROP FUNCTION IF EXISTS cleanup_soft_deleted_records();

-- +goose StatementEnd
```

---

### Migration 007: webhook_subscriptions

**File:** `007_webhook_subscriptions.sql`

```sql
-- Migration: Add webhook subscriptions table
-- Purpose: Store merchant webhook URLs for chargeback notifications

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(100) NOT NULL,
    event_type VARCHAR(50) NOT NULL, -- 'chargeback.created', 'chargeback.updated', etc.
    webhook_url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL, -- Used to sign webhook payloads
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one active webhook per agent per event type
    CONSTRAINT unique_active_webhook UNIQUE (agent_id, event_type, webhook_url)
);

-- Index for fast lookups by agent and event type
CREATE INDEX idx_webhook_subscriptions_agent_event
ON webhook_subscriptions(agent_id, event_type)
WHERE is_active = true;

-- Table for webhook delivery logs (track success/failures)
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL, -- 'pending', 'success', 'failed'
    http_status_code INT,
    error_message TEXT,
    attempts INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT valid_status CHECK (status IN ('pending', 'success', 'failed'))
);

-- Index for retry queue
CREATE INDEX idx_webhook_deliveries_retry
ON webhook_deliveries(next_retry_at)
WHERE status = 'pending' AND next_retry_at IS NOT NULL;

-- Index for delivery history lookup
CREATE INDEX idx_webhook_deliveries_subscription
ON webhook_deliveries(subscription_id, created_at DESC);

COMMENT ON TABLE webhook_subscriptions IS 'Merchant webhook subscriptions for chargeback events';
COMMENT ON TABLE webhook_deliveries IS 'Webhook delivery log for tracking and retries';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_subscriptions;
-- +goose StatementEnd
```

---

### Migration 008: auth_tables

**File:** `008_auth_tables.sql`

```sql
-- +goose Up
-- Migration: 008_auth_tables.sql
-- Description: Clean separation - Services (auth) vs Merchants (business entities)
-- Architecture:
--   - services: ALL apps/clients (internal + external merchant apps) with JWT auth
--   - merchants: Pure business entity data + EPX gateway credentials
--   - service_merchants: Links services to merchants (many-to-many)
-- Author: Authentication System
-- Date: 2025-11-18

-- Admin users table
CREATE TABLE IF NOT EXISTS admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'admin',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Update merchants table to add business fields
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'pending_activation';
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS tier VARCHAR(50) DEFAULT 'standard';
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES admins(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES admins(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS approved_at TIMESTAMP;

-- Services table: ALL apps/clients (internal microservices + merchant apps)
-- Examples:
--   - Internal: billing-service, subscription-service (merchant_id = NULL in service_merchants)
--   - External: "ACME Web App", "ACME Mobile App" (linked via service_merchants)
CREATE TABLE IF NOT EXISTS services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id VARCHAR(100) UNIQUE NOT NULL,  -- e.g., "acme-web-app", "billing-service"
    service_name VARCHAR(255) NOT NULL,       -- e.g., "ACME Corp Web Application"
    public_key TEXT NOT NULL,                 -- RSA public key for JWT verification
    public_key_fingerprint VARCHAR(64) NOT NULL,
    environment VARCHAR(50) NOT NULL,         -- staging, production

    -- Rate limit configuration (per service, not per merchant)
    requests_per_second INTEGER DEFAULT 100,
    burst_limit INTEGER DEFAULT 200,

    is_active BOOLEAN DEFAULT true,
    created_by UUID REFERENCES admins(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Service-to-merchant access control (many-to-many)
-- Links services to merchants with scoped permissions
CREATE TABLE IF NOT EXISTS service_merchants (
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    scopes TEXT[], -- ['payment:create', 'payment:read', 'subscription:manage', etc.]
    granted_by UUID REFERENCES admins(id),
    granted_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    PRIMARY KEY (service_id, merchant_id)
);

-- Indexes for service_merchants
CREATE INDEX IF NOT EXISTS idx_service_merchants_service
    ON service_merchants(service_id);

CREATE INDEX IF NOT EXISTS idx_service_merchants_merchant
    ON service_merchants(merchant_id);

CREATE INDEX IF NOT EXISTS idx_service_merchants_expires
    ON service_merchants(expires_at)
    WHERE expires_at IS NOT NULL;

-- Merchant activation tokens (one-time use for onboarding)
CREATE TABLE IF NOT EXISTS merchant_activation_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Comprehensive audit log (partitioned by month)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID DEFAULT gen_random_uuid(),
    -- Actor
    actor_type VARCHAR(50), -- 'admin', 'service', 'system'
    actor_id VARCHAR(255),  -- service_id or admin_id
    actor_name VARCHAR(255),

    -- Action
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50),
    entity_id VARCHAR(255),

    -- Details
    changes JSONB,
    metadata JSONB,

    -- Context
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(100),

    -- Result
    success BOOLEAN DEFAULT true,
    error_message TEXT,

    performed_at TIMESTAMP DEFAULT NOW()
) PARTITION BY RANGE (performed_at);

-- Create monthly partitions for audit log (next 3 months)
CREATE TABLE IF NOT EXISTS audit_logs_2025_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_02 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- Add primary key to each partition
ALTER TABLE audit_logs_2025_01 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_02 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_03 ADD PRIMARY KEY (id);

-- Rate limit tracking (per service)
CREATE TABLE IF NOT EXISTS rate_limit_buckets (
    bucket_key VARCHAR(255) PRIMARY KEY,  -- Format: "service_id:merchant_id" or "service_id"
    tokens INTEGER NOT NULL,
    last_refill TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- EPX IP whitelist (for callback security)
CREATE TABLE IF NOT EXISTS epx_ip_whitelist (
    id SERIAL PRIMARY KEY,
    ip_address INET NOT NULL UNIQUE,
    description VARCHAR(255),
    added_by UUID REFERENCES admins(id),
    added_at TIMESTAMP DEFAULT NOW(),
    is_active BOOLEAN DEFAULT true
);

-- Insert default EPX IPs (update with real IPs in production)
INSERT INTO epx_ip_whitelist (ip_address, description) VALUES
    ('127.0.0.1', 'Local development'),
    ('::1', 'Local development IPv6')
ON CONFLICT (ip_address) DO NOTHING;

-- JWT token blacklist (for emergency revocation)
CREATE TABLE IF NOT EXISTS jwt_blacklist (
    jti VARCHAR(255) PRIMARY KEY, -- JWT ID
    service_id VARCHAR(100),
    merchant_id UUID,
    expires_at TIMESTAMP NOT NULL,
    blacklisted_at TIMESTAMP DEFAULT NOW(),
    blacklisted_by UUID REFERENCES admins(id),
    reason TEXT
);

-- Clean up expired blacklist entries periodically
CREATE INDEX IF NOT EXISTS idx_jwt_blacklist_expires ON jwt_blacklist(expires_at);

-- Session tracking for admin users
CREATE TABLE IF NOT EXISTS admin_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID REFERENCES admins(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for audit log queries
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs (actor_type, actor_id, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs (action, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs (entity_type, entity_id, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_ip ON audit_logs (ip_address, performed_at DESC) WHERE ip_address IS NOT NULL;

-- +goose Down
-- Drop all auth-related tables
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS jwt_blacklist;
DROP TABLE IF EXISTS epx_ip_whitelist;
DROP TABLE IF EXISTS rate_limit_buckets;
DROP TABLE IF EXISTS audit_logs_2025_03;
DROP TABLE IF EXISTS audit_logs_2025_02;
DROP TABLE IF EXISTS audit_logs_2025_01;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS merchant_activation_tokens;
DROP TABLE IF EXISTS service_merchants;
DROP TABLE IF EXISTS services;
ALTER TABLE merchants DROP COLUMN IF EXISTS status;
ALTER TABLE merchants DROP COLUMN IF EXISTS tier;
ALTER TABLE merchants DROP COLUMN IF EXISTS created_by;
ALTER TABLE merchants DROP COLUMN IF EXISTS approved_by;
ALTER TABLE merchants DROP COLUMN IF EXISTS approved_at;
DROP TABLE IF EXISTS admins;
```

---

### Migration 009: ach_verification_enhancements

**File:** `009_ach_verification_enhancements.sql`

```sql
-- +goose Up
-- +goose StatementBegin

-- Add verification tracking fields for ACH payment methods
ALTER TABLE customer_payment_methods
    ADD COLUMN verification_status VARCHAR(20) DEFAULT 'pending',
    ADD COLUMN prenote_transaction_id UUID,
    ADD COLUMN verified_at TIMESTAMPTZ,
    ADD COLUMN verification_failure_reason TEXT,
    ADD COLUMN return_count INTEGER DEFAULT 0 NOT NULL,
    ADD COLUMN deactivation_reason VARCHAR(100),
    ADD COLUMN deactivated_at TIMESTAMPTZ;

-- Add check constraint for verification_status
ALTER TABLE customer_payment_methods
    ADD CONSTRAINT check_verification_status
    CHECK (verification_status IN ('pending', 'verified', 'failed'));

-- Add check constraint for return_count
ALTER TABLE customer_payment_methods
    ADD CONSTRAINT check_return_count
    CHECK (return_count >= 0);

-- Create index for pending verifications (for cron job)
CREATE INDEX idx_customer_payment_methods_pending_verification
    ON customer_payment_methods(verification_status, created_at)
    WHERE verification_status = 'pending' AND payment_type = 'ach';

-- Create index for prenote transaction lookups
CREATE INDEX idx_customer_payment_methods_prenote_transaction
    ON customer_payment_methods(prenote_transaction_id)
    WHERE prenote_transaction_id IS NOT NULL;

-- Update existing records to have proper verification_status
-- Credit cards are always considered verified (no pre-note required)
UPDATE customer_payment_methods
SET verification_status = 'verified',
    verified_at = created_at
WHERE payment_type = 'credit_card';

-- Existing ACH payment methods with is_verified=true should be marked as verified
UPDATE customer_payment_methods
SET verification_status = 'verified',
    verified_at = created_at
WHERE payment_type = 'ach' AND is_verified = true;

-- Existing ACH payment methods with is_verified=false should stay pending
UPDATE customer_payment_methods
SET verification_status = 'pending'
WHERE payment_type = 'ach' AND is_verified = false;

-- Add comment explaining verification flow
COMMENT ON COLUMN customer_payment_methods.verification_status IS
'ACH verification status: pending (pre-note sent, awaiting clearance), verified (pre-note cleared after 3 days), failed (return code received)';

COMMENT ON COLUMN customer_payment_methods.prenote_transaction_id IS
'Links to the pre-note (CKC0) transaction used for ACH verification';

COMMENT ON COLUMN customer_payment_methods.verified_at IS
'Timestamp when ACH verification completed (3 days after pre-note with no returns)';

COMMENT ON COLUMN customer_payment_methods.verification_failure_reason IS
'Reason for verification failure (e.g., "R03: No Account/Unable to Locate")';

COMMENT ON COLUMN customer_payment_methods.return_count IS
'Number of ACH returns received. Auto-deactivate after 2+ returns';

COMMENT ON COLUMN customer_payment_methods.deactivation_reason IS
'Reason for deactivation (e.g., "excessive_returns", "manual_deactivation")';

COMMENT ON COLUMN customer_payment_methods.deactivated_at IS
'Timestamp when payment method was deactivated';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove indexes
DROP INDEX IF EXISTS idx_customer_payment_methods_pending_verification;
DROP INDEX IF EXISTS idx_customer_payment_methods_prenote_transaction;

-- Remove constraints
ALTER TABLE customer_payment_methods DROP CONSTRAINT IF EXISTS check_verification_status;
ALTER TABLE customer_payment_methods DROP CONSTRAINT IF EXISTS check_return_count;

-- Remove columns
ALTER TABLE customer_payment_methods
    DROP COLUMN IF EXISTS verification_status,
    DROP COLUMN IF EXISTS prenote_transaction_id,
    DROP COLUMN IF EXISTS verified_at,
    DROP COLUMN IF EXISTS verification_failure_reason,
    DROP COLUMN IF EXISTS return_count,
    DROP COLUMN IF EXISTS deactivation_reason,
    DROP COLUMN IF EXISTS deactivated_at;

-- +goose StatementEnd
```

---

### Migration 010: add_ach_verification_index

**File:** `010_add_ach_verification_index.sql`

```sql
-- +goose NO TRANSACTION
-- +goose Up
-- Partial index: Only index pending ACH verifications (much smaller, faster)
-- Optimizes GetPendingACHVerifications cron query (runs every 5 minutes)
-- Expected impact: 102ms → 5ms (-95% faster)
CREATE INDEX CONCURRENTLY idx_payment_methods_ach_verification
ON customer_payment_methods(payment_type, verification_status, created_at)
WHERE payment_type = 'ach'
  AND verification_status = 'pending'
  AND deleted_at IS NULL;

COMMENT ON INDEX idx_payment_methods_ach_verification IS
  'Optimizes GetPendingACHVerifications cron query. Partial index for pending ACH only. Query time: 102ms → 5ms.';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_payment_methods_ach_verification;
```

---

### Migration 011: add_prenote_transaction_index

**File:** `011_add_prenote_transaction_index.sql`

```sql
-- +goose NO TRANSACTION
-- +goose Up
-- Index on prenote_transaction_id for ACH return processing
-- Optimizes GetPaymentMethodByPreNoteTransaction query
-- Expected impact: 50-100ms → 2-5ms (-95% faster)
CREATE INDEX CONCURRENTLY idx_payment_methods_prenote_transaction
ON customer_payment_methods(prenote_transaction_id)
WHERE prenote_transaction_id IS NOT NULL
  AND deleted_at IS NULL;

COMMENT ON INDEX idx_payment_methods_prenote_transaction IS
  'Optimizes GetPaymentMethodByPreNoteTransaction for ACH return processing. Partial index excludes NULL values.';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_payment_methods_prenote_transaction;
```

---

### Migration 012: add_payment_methods_sorted_index

**File:** `012_add_payment_methods_sorted_index.sql`

```sql
-- +goose NO TRANSACTION
-- +goose Up
-- Composite index for sorted payment method listings
-- Optimizes ListPaymentMethodsByCustomer with pre-sorted results
-- Expected impact: 15ms → 3ms (-80% faster), eliminates sort operation
CREATE INDEX CONCURRENTLY idx_payment_methods_customer_sorted
ON customer_payment_methods(merchant_id, customer_id, is_default DESC, created_at DESC)
WHERE deleted_at IS NULL;

COMMENT ON INDEX idx_payment_methods_customer_sorted IS
  'Optimizes ListPaymentMethodsByCustomer with pre-sorted results. Eliminates sort operation.';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_payment_methods_customer_sorted;
```

---

### Migration 013: customer_id_to_varchar

**File:** `013_customer_id_to_varchar.sql`

```sql
-- +goose Up
-- =====================================================
-- Migration: Convert customer_id from UUID to VARCHAR
-- =====================================================
--
-- This migration changes customer_id from UUID to VARCHAR(100)
-- to support external service identifiers (e.g., Stripe customer IDs,
-- WordPress user IDs, etc.) while maintaining consistency with the
-- chargebacks table which already uses VARCHAR(100).
--
-- Affected tables:
--   - customer_payment_methods
--   - transactions
--   - subscriptions
--
-- Note: chargebacks table already uses VARCHAR(100) for customer_id

-- ========================================
-- 1. Drop existing indexes
-- ========================================

DROP INDEX IF EXISTS idx_customer_payment_methods_customer_id;
DROP INDEX IF EXISTS idx_customer_payment_methods_merchant_customer;
DROP INDEX IF EXISTS idx_customer_payment_methods_is_default;
DROP INDEX IF EXISTS idx_transactions_customer_id;
DROP INDEX IF EXISTS idx_transactions_merchant_customer;
DROP INDEX IF EXISTS idx_subscriptions_merchant_customer;

-- ========================================
-- 2. Convert customer_id columns to VARCHAR(100)
-- ========================================

-- Convert customer_payment_methods.customer_id
ALTER TABLE customer_payment_methods
  ALTER COLUMN customer_id TYPE VARCHAR(100) USING customer_id::TEXT;

-- Convert transactions.customer_id
ALTER TABLE transactions
  ALTER COLUMN customer_id TYPE VARCHAR(100) USING customer_id::TEXT;

-- Convert subscriptions.customer_id
ALTER TABLE subscriptions
  ALTER COLUMN customer_id TYPE VARCHAR(100) USING customer_id::TEXT;

-- ========================================
-- 3. Recreate indexes with same definitions
-- ========================================

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

-- +goose Down
-- Rollback: Convert customer_id back from VARCHAR to UUID

-- Drop indexes
DROP INDEX IF EXISTS idx_customer_payment_methods_customer_id;
DROP INDEX IF EXISTS idx_customer_payment_methods_merchant_customer;
DROP INDEX IF EXISTS idx_customer_payment_methods_is_default;
DROP INDEX IF EXISTS idx_transactions_customer_id;
DROP INDEX IF EXISTS idx_transactions_merchant_customer;
DROP INDEX IF EXISTS idx_subscriptions_merchant_customer;

-- Convert back to UUID
ALTER TABLE customer_payment_methods
  ALTER COLUMN customer_id TYPE UUID USING customer_id::UUID;

ALTER TABLE transactions
  ALTER COLUMN customer_id TYPE UUID USING customer_id::UUID;

ALTER TABLE subscriptions
  ALTER COLUMN customer_id TYPE UUID USING customer_id::UUID;

-- Recreate indexes
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
```

---

### Migration 019: standardize_timestamps_to_timestamptz

**File:** `019_standardize_timestamps_to_timestamptz.sql`

```sql
-- +goose Up
-- +goose StatementBegin
-- Standardize all TIMESTAMP columns to TIMESTAMPTZ for timezone consistency
-- This fixes critical timezone handling issues across all tables
-- Assumes existing data is in UTC (safest assumption for database timestamps)

-- Fix merchants table
ALTER TABLE merchants
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC',
  ALTER COLUMN deleted_at TYPE TIMESTAMPTZ USING deleted_at AT TIME ZONE 'UTC',
  ALTER COLUMN approved_at TYPE TIMESTAMPTZ USING approved_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN merchants.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.updated_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.deleted_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.approved_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix services table (auth)
ALTER TABLE services
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN services.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN services.updated_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix service_merchants table (auth) - uses granted_at and expires_at, not created_at
ALTER TABLE service_merchants
  ALTER COLUMN granted_at TYPE TIMESTAMPTZ USING granted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN service_merchants.granted_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN service_merchants.expires_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix admins table (auth)
ALTER TABLE admins
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN admins.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN admins.updated_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix admin_sessions table
ALTER TABLE admin_sessions
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN admin_sessions.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN admin_sessions.expires_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Skip audit_logs table - it's partitioned by performed_at (partition key cannot be altered)
-- Partitioned tables require recreating the entire partition structure to change key column types
-- This is acceptable as audit_logs is for logging and timezone inconsistency is less critical
-- Future partitions should be created with TIMESTAMPTZ from the start

-- Fix jwt_blacklist table
ALTER TABLE jwt_blacklist
  ALTER COLUMN blacklisted_at TYPE TIMESTAMPTZ USING blacklisted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN jwt_blacklist.blacklisted_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN jwt_blacklist.expires_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix epx_ip_whitelist table
ALTER TABLE epx_ip_whitelist
  ALTER COLUMN added_at TYPE TIMESTAMPTZ USING added_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN epx_ip_whitelist.added_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix merchant_activation_tokens table
ALTER TABLE merchant_activation_tokens
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN used_at TYPE TIMESTAMPTZ USING used_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN merchant_activation_tokens.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchant_activation_tokens.expires_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchant_activation_tokens.used_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix rate_limit_buckets table
ALTER TABLE rate_limit_buckets
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN rate_limit_buckets.created_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Verify all timestamp columns are now timezone-aware (except audit_logs partitions)
DO $$
DECLARE
    non_tz_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO non_tz_count
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND column_name LIKE '%_at'
      AND data_type = 'timestamp without time zone'
      AND table_name NOT LIKE 'audit_logs%'; -- Exclude audit_logs and its partitions

    IF non_tz_count > 0 THEN
        RAISE EXCEPTION 'Migration failed: % columns still using TIMESTAMP without timezone', non_tz_count;
    END IF;

    RAISE NOTICE 'SUCCESS: All timestamp columns are now timezone-aware (TIMESTAMPTZ), except audit_logs partitions';
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Revert to TIMESTAMP (not recommended, loses timezone information)
-- Only use this for rollback in case of issues

ALTER TABLE merchants
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
  ALTER COLUMN deleted_at TYPE TIMESTAMP USING deleted_at AT TIME ZONE 'UTC',
  ALTER COLUMN approved_at TYPE TIMESTAMP USING approved_at AT TIME ZONE 'UTC';

ALTER TABLE services
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE service_merchants
  ALTER COLUMN granted_at TYPE TIMESTAMP USING granted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE admins
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE admin_sessions
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC';

-- Skip audit_logs table (partitioned, cannot alter partition key)

ALTER TABLE jwt_blacklist
  ALTER COLUMN blacklisted_at TYPE TIMESTAMP USING blacklisted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE epx_ip_whitelist
  ALTER COLUMN added_at TYPE TIMESTAMP USING added_at AT TIME ZONE 'UTC';

ALTER TABLE merchant_activation_tokens
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN used_at TYPE TIMESTAMP USING used_at AT TIME ZONE 'UTC';

ALTER TABLE rate_limit_buckets
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';
-- +goose StatementEnd
```

---

### Migration 020: add_transaction_listing_indexes

**File:** `020_add_transaction_listing_indexes.sql`

```sql
-- +goose NO TRANSACTION
-- +goose Up
-- Optimize ListTransactions query performance (60-80% faster)
-- These indexes cover the most common query patterns for transaction listing

-- Index 1: Basic merchant transaction listing (most common case)
-- Covers merchant_id + created_at DESC ordering
-- Expected impact: 50-100ms → 10-20ms for merchant transaction list
CREATE INDEX CONCURRENTLY idx_transactions_merchant_created
ON transactions(merchant_id, created_at DESC);

COMMENT ON INDEX idx_transactions_merchant_created IS
  'Optimizes merchant transaction listing ordered by creation date. Query time: 50-100ms → 10-20ms (-80% faster).';

-- Index 2: Customer-specific transaction queries
-- Covers merchant_id + customer_id + created_at DESC
-- Optimizes filtered queries by customer
CREATE INDEX CONCURRENTLY idx_transactions_merchant_customer_created
ON transactions(merchant_id, customer_id, created_at DESC)
WHERE customer_id IS NOT NULL;

COMMENT ON INDEX idx_transactions_merchant_customer_created IS
  'Optimizes customer transaction history queries. Partial index (customer_id IS NOT NULL).';

-- Index 3: Subscription transaction queries
-- Covers merchant_id + subscription_id + created_at DESC
-- Optimizes filtered queries by subscription
CREATE INDEX CONCURRENTLY idx_transactions_merchant_subscription_created
ON transactions(merchant_id, subscription_id, created_at DESC)
WHERE subscription_id IS NOT NULL;

COMMENT ON INDEX idx_transactions_merchant_subscription_created IS
  'Optimizes subscription transaction queries. Partial index (subscription_id IS NOT NULL).';

-- Index 4: Payment method transaction queries
-- Covers merchant_id + payment_method_id + created_at DESC
-- Optimizes filtered queries by payment method
CREATE INDEX CONCURRENTLY idx_transactions_merchant_payment_method_created
ON transactions(merchant_id, payment_method_id, created_at DESC)
WHERE payment_method_id IS NOT NULL;

COMMENT ON INDEX idx_transactions_merchant_payment_method_created IS
  'Optimizes payment method transaction queries. Partial index (payment_method_id IS NOT NULL).';

-- Index 5: Transaction status queries
-- Covers merchant_id + status + created_at DESC
-- Optimizes filtered queries by transaction status
CREATE INDEX CONCURRENTLY idx_transactions_merchant_status_created
ON transactions(merchant_id, status, created_at DESC);

COMMENT ON INDEX idx_transactions_merchant_status_created IS
  'Optimizes transaction status filtering (approved/declined/pending queries).';

-- Index 6: Transaction type queries
-- Covers merchant_id + type + created_at DESC
-- Optimizes filtered queries by transaction type (sale/auth/capture/refund/void)
CREATE INDEX CONCURRENTLY idx_transactions_merchant_type_created
ON transactions(merchant_id, type, created_at DESC);

COMMENT ON INDEX idx_transactions_merchant_type_created IS
  'Optimizes transaction type filtering (sale/auth/capture/refund/void queries).';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_merchant_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_merchant_customer_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_merchant_subscription_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_merchant_payment_method_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_merchant_status_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_transactions_merchant_type_created;
```

---

### Migration 021: add_audit_logs_partitions_2025_remaining

**File:** `021_add_audit_logs_partitions_2025_remaining.sql`

```sql
-- +goose Up
-- Migration: 021_add_audit_logs_partitions_2025_remaining.sql
-- Description: Add missing partitions for audit_logs table for remainder of 2025
-- The original migration only created partitions for Jan-Mar 2025, causing failures
-- when trying to log audit events after March 2025.
-- Author: System
-- Date: 2025-11-22

-- Create monthly partitions for April-December 2025
CREATE TABLE IF NOT EXISTS audit_logs_2025_04 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-04-01') TO ('2025-05-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_05 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-05-01') TO ('2025-06-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_06 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_07 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_08 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-08-01') TO ('2025-09-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_09 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-09-01') TO ('2025-10-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_10 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-10-01') TO ('2025-11-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_11 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_12 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');

-- Add primary keys to each partition
ALTER TABLE audit_logs_2025_04 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_05 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_06 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_07 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_08 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_09 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_10 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_11 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_12 ADD PRIMARY KEY (id);

-- Add partitions for 2026 to prevent future issues
CREATE TABLE IF NOT EXISTS audit_logs_2026_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE IF NOT EXISTS audit_logs_2026_02 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE IF NOT EXISTS audit_logs_2026_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

ALTER TABLE audit_logs_2026_01 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2026_02 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2026_03 ADD PRIMARY KEY (id);

-- +goose Down
-- Drop 2026 partitions
DROP TABLE IF EXISTS audit_logs_2026_03;
DROP TABLE IF EXISTS audit_logs_2026_02;
DROP TABLE IF EXISTS audit_logs_2026_01;

-- Drop 2025 partitions
DROP TABLE IF EXISTS audit_logs_2025_12;
DROP TABLE IF EXISTS audit_logs_2025_11;
DROP TABLE IF EXISTS audit_logs_2025_10;
DROP TABLE IF EXISTS audit_logs_2025_09;
DROP TABLE IF EXISTS audit_logs_2025_08;
DROP TABLE IF EXISTS audit_logs_2025_07;
DROP TABLE IF EXISTS audit_logs_2025_06;
DROP TABLE IF EXISTS audit_logs_2025_05;
DROP TABLE IF EXISTS audit_logs_2025_04;
```

---

### Migration 022: add_ach_verification_index

**File:** `022_add_ach_verification_index.sql`

```sql
-- +goose NO TRANSACTION
-- +goose Up

-- CRITICAL PERFORMANCE FIX: ACH Verification Query Optimization
--
-- PROBLEM:
-- - ACH verification queries were doing full table scans (100ms per query)
-- - Cron job checking pending ACH verifications was slow and blocking
-- - No index on payment_type + verification_status combination
-- - Creates DoS vulnerability under high load
--
-- SOLUTION:
-- - Partial index specifically for pending ACH verifications
-- - Includes created_at for sorting oldest-first
-- - Only indexes non-deleted records (respects soft-delete pattern)
-- - Reduces query time from 100ms to <5ms (20x improvement)
--
-- IMPACT:
-- - ACH verification cron runs 20x faster
-- - Eliminates DoS vector from ACH verification queries
-- - Minimal storage overhead (partial index only for pending ACH)
-- - Query pattern: SELECT * FROM customer_payment_methods
--                  WHERE payment_type = 'ach'
--                    AND verification_status = 'pending'
--                    AND deleted_at IS NULL
--                  ORDER BY created_at ASC;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_methods_ach_verification
ON customer_payment_methods(payment_type, verification_status, created_at)
WHERE payment_type = 'ach'
  AND verification_status = 'pending'
  AND deleted_at IS NULL;

COMMENT ON INDEX idx_payment_methods_ach_verification IS
'Optimizes ACH verification cron queries. Reduces full table scan (100ms) to index scan (<5ms). Partial index only for pending ACH verifications.';

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS idx_payment_methods_ach_verification;
```

---


## Performance Considerations

### Connection Pooling

**Configuration (pgxpool):**
```go
config.MaxConns = 25              // Max connections
config.MinConns = 5               // Keep-alive connections
config.MaxConnLifetime = 1h       // Recycle connections hourly
config.MaxConnIdleTime = 30m      // Close idle after 30min
config.HealthCheckPeriod = 15s    // Health check interval
```

**Monitoring:**
- `pool_total_connections` - Total pool size
- `pool_idle_connections` - Available connections
- `pool_active_connections` - In-use connections
- `pool_wait_count` - Queue depth
- Alert when utilization > 80%

### Query Performance

**Best Practices:**
1. **Use indexes** - All foreign keys should have indexes
2. **Avoid N+1 queries** - Use JOINs or batch loading
3. **Pagination** - Always use LIMIT/OFFSET for large result sets
4. **Partial indexes** - Where clauses in index definitions for specific queries
5. **Analyze queries** - Use `EXPLAIN ANALYZE` to verify index usage

**Example - Find slow queries:**
```sql
-- Enable slow query logging
ALTER DATABASE payments SET log_min_duration_statement = 1000;  -- Log queries > 1s

-- Query pg_stat_statements
SELECT
  query,
  calls,
  total_exec_time / 1000 as total_seconds,
  mean_exec_time / 1000 as avg_seconds,
  max_exec_time / 1000 as max_seconds
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = 'payments')
ORDER BY total_exec_time DESC
LIMIT 10;
```

### Backup Strategy

**Automated Backups:**
- Daily full backup at 2 AM UTC
- 30-day retention
- Point-in-time recovery (WAL archiving)
- Test restore monthly

**Manual Backup:**
```bash
# Backup
pg_dump -h localhost -U postgres payments > backup_$(date +%Y%m%d).sql

# Restore
psql -h localhost -U postgres payments < backup_20241123.sql
```

---

## Migration Commands

```bash
# Check migration status
make migrate-status

# Apply all pending migrations
make migrate-up

# Apply specific migration
goose -dir internal/db/migrations postgres "postgresql://..." up-to 5

# Rollback last migration
make migrate-down

# Create new migration
make migrate-create NAME=add_webhooks_table

# Reset database (DANGER: drops all data)
make migrate-reset
```

---

## Related Documentation

- **[SETUP.md](./SETUP.md)** - Database setup for development
- **[API_SPECS.md](../integration/API_SPECS.md)** - API documentation
- **[STYLE_GUIDE.md](./STYLE_GUIDE.md)** - Code style including database queries

---

**Maintenance Checklist:**

- [ ] Weekly: Review slow query log
- [ ] Monthly: Analyze table bloat (`pg_stat_user_tables`)
- [ ] Monthly: Verify backup restore
- [ ] Quarterly: Review index usage (`pg_stat_user_indexes`)
- [ ] Quarterly: Vacuum analyze large tables

