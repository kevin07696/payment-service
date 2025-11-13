# Database Schema and Design

**Target Audience:** Developers working with database queries, migrations, or data modeling
**Topic:** PostgreSQL schema design, table relationships, and data lifecycle
**Goal:** Understand the database structure and query patterns for the payment service

---

## Overview

**Database:** PostgreSQL 15+
**Migration Tool:** Goose (SQL-based migrations)
**Design Principles:**
- Multi-tenant architecture with merchant isolation
- Soft deletes with automatic cleanup (90-day retention)
- Idempotency via unique constraints
- PCI compliance through tokenization (no raw card data)
- Immutable audit trail for compliance

---

## Quick Reference

### Table Summary

| Table | Purpose | Key Column |
|-------|---------|------------|
| `merchants` | Merchant credentials and configuration | `slug` |
| `customer_payment_methods` | Saved payment methods (tokenized) | `payment_token` |
| `transactions` | All payment operations | `group_id` |
| `subscriptions` | Recurring billing schedules | `next_billing_date` |
| `chargebacks` | Dispute tracking | `case_number` |
| `audit_logs` | Immutable audit trail | `entity_id` |
| `webhook_subscriptions` | Webhook endpoints | `event_type` |
| `webhook_deliveries` | Webhook delivery tracking | `next_retry_at` |

### Common Query Patterns

```sql
-- List customer transactions
SELECT * FROM transactions
WHERE merchant_id = $1 AND customer_id = $2 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- Get transaction group (auth → capture → refund)
SELECT * FROM transactions
WHERE group_id = $1 AND deleted_at IS NULL
ORDER BY created_at;

-- Find active subscriptions due for billing
SELECT * FROM subscriptions
WHERE merchant_id = $1 AND status = 'active'
  AND next_billing_date <= CURRENT_DATE AND deleted_at IS NULL;
```

---

## Schema Details

### merchants

**Purpose:** Multi-tenant merchant configuration and EPX credentials

**Table Structure:**

```sql
CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(255) UNIQUE NOT NULL,

    -- EPX Credentials
    cust_nbr VARCHAR(50) NOT NULL,
    merch_nbr VARCHAR(50) NOT NULL,
    dba_nbr VARCHAR(50) NOT NULL,
    terminal_nbr VARCHAR(50) NOT NULL,

    -- Secret Manager integration
    mac_secret_path VARCHAR(500) NOT NULL,

    -- Environment and status
    environment VARCHAR(20) NOT NULL DEFAULT 'production',
    is_active BOOLEAN NOT NULL DEFAULT true,
    name VARCHAR(255) NOT NULL,

    -- Soft delete and timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
```

**Column Reference:**

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | UUID | NOT NULL | Primary key (auto-generated) |
| `slug` | VARCHAR(255) | NOT NULL | Unique merchant identifier (e.g., "acme-corp") |
| `cust_nbr` | VARCHAR(50) | NOT NULL | EPX customer number |
| `merch_nbr` | VARCHAR(50) | NOT NULL | EPX merchant number |
| `dba_nbr` | VARCHAR(50) | NOT NULL | EPX DBA (Doing Business As) number |
| `terminal_nbr` | VARCHAR(50) | NOT NULL | EPX terminal number |
| `mac_secret_path` | VARCHAR(500) | NOT NULL | OCI Vault secret path (NOT the actual secret) |
| `environment` | VARCHAR(20) | NOT NULL | `production`, `staging`, `test` |
| `is_active` | BOOLEAN | NOT NULL | Active status (false = disabled) |
| `name` | VARCHAR(255) | NOT NULL | Display name for merchant |
| `created_at` | TIMESTAMP | NOT NULL | Record creation timestamp |
| `updated_at` | TIMESTAMP | NOT NULL | Last update timestamp (auto-updated via trigger) |
| `deleted_at` | TIMESTAMP | NULL | Soft delete timestamp |

**Indexes:**

```sql
idx_merchants_slug           ON (slug) WHERE deleted_at IS NULL
idx_merchants_environment    ON (environment) WHERE deleted_at IS NULL
idx_merchants_is_active      ON (is_active) WHERE deleted_at IS NULL
```

**Unique Constraints:**
- `slug` - One merchant per slug

**Security Notes:**
- `mac_secret_path` stores OCI Vault OCID, NOT the actual MAC secret
- Service reads secret at runtime using OCI SDK
- Prevents MAC secrets from ever being stored in database

**Example:**

```sql
-- Find merchant by slug
SELECT id, name, cust_nbr, merch_nbr
FROM merchants
WHERE slug = 'acme-corp' AND deleted_at IS NULL;

-- Get MAC secret path for EPX signing
SELECT mac_secret_path FROM merchants
WHERE id = '550e8400-...' AND is_active = true;
```

---

### customer_payment_methods

**Purpose:** Saved payment methods (credit cards and bank accounts) using EPX BRIC tokens

**Table Structure:**

```sql
CREATE TABLE customer_payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id VARCHAR(255) NOT NULL,
    payment_token TEXT NOT NULL,
    payment_type VARCHAR(20) NOT NULL,
    last_four VARCHAR(4) NOT NULL,

    -- Credit card metadata
    card_brand VARCHAR(20),
    card_exp_month INTEGER,
    card_exp_year INTEGER,

    -- ACH metadata
    bank_name VARCHAR(255),
    account_type VARCHAR(20),

    -- Status tracking
    is_default BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    is_verified BOOLEAN DEFAULT false,

    -- Timestamps
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ
);
```

**Column Reference:**

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | UUID | NOT NULL | Primary key (auto-generated) |
| `merchant_id` | UUID | NOT NULL | FK to `merchants.id` (multi-tenant isolation) |
| `customer_id` | VARCHAR(255) | NOT NULL | Customer identifier from client application |
| `payment_token` | TEXT | NOT NULL | EPX BRIC token (e.g., "0V703LH1HDL006J74W1") |
| `payment_type` | VARCHAR(20) | NOT NULL | `credit_card` or `ach` |
| `last_four` | VARCHAR(4) | NOT NULL | Last 4 digits for display (NOT for security) |
| `card_brand` | VARCHAR(20) | NULL | `visa`, `mastercard`, `amex`, `discover` |
| `card_exp_month` | INTEGER | NULL | Expiration month (1-12) |
| `card_exp_year` | INTEGER | NULL | Expiration year (YYYY format, e.g., 2025) |
| `bank_name` | VARCHAR(255) | NULL | User-provided bank label (ACH only) |
| `account_type` | VARCHAR(20) | NULL | `checking` or `savings` (ACH only) |
| `is_default` | BOOLEAN | NOT NULL | Default payment method flag |
| `is_active` | BOOLEAN | NOT NULL | Active status (false = disabled) |
| `is_verified` | BOOLEAN | NOT NULL | ACH pre-note verification status |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete timestamp |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last update timestamp (auto-updated via trigger) |
| `last_used_at` | TIMESTAMPTZ | NULL | Last transaction timestamp |

**Indexes:**

```sql
idx_customer_payment_methods_merchant_customer    ON (merchant_id, customer_id)
idx_customer_payment_methods_merchant_id         ON (merchant_id)
idx_customer_payment_methods_customer_id         ON (customer_id)
idx_customer_payment_methods_payment_type        ON (payment_type)
idx_customer_payment_methods_is_default          ON (merchant_id, customer_id, is_default) WHERE is_default = true
idx_customer_payment_methods_is_active           ON (is_active) WHERE is_active = true
idx_customer_payment_methods_deleted_at          ON (deleted_at) WHERE deleted_at IS NOT NULL
```

**Unique Constraints:**
- `(merchant_id, customer_id, payment_token)` - Prevent duplicate tokens

**Foreign Keys:**
- `merchant_id` → `merchants(id)` ON DELETE RESTRICT

**Check Constraints:**
- `payment_type IN ('credit_card', 'ach')`
- `card_exp_month IS NULL OR (card_exp_month >= 1 AND card_exp_month <= 12)`
- `account_type IS NULL OR account_type IN ('checking', 'savings')`

**PCI Compliance:**
- ✅ Only stores EPX BRIC tokens (never raw card numbers)
- ✅ Only stores last 4 digits (display purposes only)
- ✅ No CVV, PIN, or full PAN stored
- ✅ Expiration dates for user warnings only

**Example:**

```sql
-- Save new payment method
INSERT INTO customer_payment_methods (
    merchant_id, customer_id, payment_token, payment_type,
    last_four, card_brand, card_exp_month, card_exp_year
) VALUES (
    '550e8400-...', 'CUST-123', '0V703LH1HDL006J74W1', 'credit_card',
    '4242', 'visa', 12, 2025
);

-- List customer payment methods
SELECT id, payment_type, last_four, card_brand, is_default
FROM customer_payment_methods
WHERE merchant_id = $1 AND customer_id = $2 AND deleted_at IS NULL
ORDER BY is_default DESC, created_at DESC;

-- Get default payment method
SELECT payment_token, payment_type, last_four
FROM customer_payment_methods
WHERE merchant_id = $1 AND customer_id = $2
  AND is_default = true AND is_active = true AND deleted_at IS NULL;
```

---

### transactions

**Purpose:** All payment operations (auth, sale, capture, refund, void, pre_note)

**Table Structure:**

```sql
CREATE TABLE transactions (
    id UUID PRIMARY KEY,
    group_id UUID NOT NULL DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id VARCHAR(100),
    amount NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    type VARCHAR(20) NOT NULL,
    payment_method_type VARCHAR(20) NOT NULL,
    payment_method_id UUID REFERENCES customer_payment_methods(id) ON DELETE SET NULL,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,

    -- EPX response fields
    auth_guid TEXT,
    auth_resp VARCHAR(10) NOT NULL,
    auth_code VARCHAR(50),
    auth_card_type VARCHAR(20),

    -- Generated status column
    status VARCHAR(20) GENERATED ALWAYS AS (
        CASE WHEN auth_resp = '00' THEN 'approved' ELSE 'declined' END
    ) STORED,

    metadata JSONB DEFAULT '{}'::jsonb,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Column Reference:**

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | UUID | NOT NULL | Primary key (client-provided via idempotency) |
| `group_id` | UUID | NOT NULL | Groups related transactions (auth → capture → refund) |
| `merchant_id` | UUID | NOT NULL | FK to `merchants.id` (multi-tenant isolation) |
| `customer_id` | VARCHAR(100) | NULL | Customer identifier (NULL for guest transactions) |
| `amount` | NUMERIC(19,4) | NOT NULL | Transaction amount (supports up to 4 decimal places) |
| `currency` | VARCHAR(3) | NOT NULL | ISO 4217 currency code (default: `USD`) |
| `type` | VARCHAR(20) | NOT NULL | `auth`, `sale`, `capture`, `refund`, `void`, `pre_note` |
| `payment_method_type` | VARCHAR(20) | NOT NULL | `credit_card` or `ach` |
| `payment_method_id` | UUID | NULL | FK to saved payment method (NULL for guest transactions) |
| `subscription_id` | UUID | NULL | FK to subscription (NULL for one-time payments) |
| `auth_guid` | TEXT | NULL | EPX BRIC token returned for this transaction |
| `auth_resp` | VARCHAR(10) | NOT NULL | EPX response code (`00` = approved, `05` = declined) |
| `auth_code` | VARCHAR(50) | NULL | Bank authorization code (for chargeback defense) |
| `auth_card_type` | VARCHAR(20) | NULL | Card brand (`V`, `M`, `A`, `D`; NULL for ACH) |
| `status` | VARCHAR(20) | NOT NULL | **Generated column:** `approved` or `declined` based on `auth_resp` |
| `metadata` | JSONB | NOT NULL | Additional fields: `auth_resp_text`, `auth_avs`, `auth_cvv2`, etc. |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete timestamp |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last update timestamp (auto-updated via trigger) |

**Indexes:**

```sql
idx_transactions_group_id              ON (group_id)
idx_transactions_merchant_id           ON (merchant_id)
idx_transactions_merchant_customer     ON (merchant_id, customer_id) WHERE customer_id IS NOT NULL
idx_transactions_customer_id           ON (customer_id) WHERE customer_id IS NOT NULL
idx_transactions_auth_guid             ON (auth_guid) WHERE auth_guid IS NOT NULL
idx_transactions_payment_method_id     ON (payment_method_id) WHERE payment_method_id IS NOT NULL
idx_transactions_subscription_id       ON (subscription_id) WHERE subscription_id IS NOT NULL
idx_transactions_created_at            ON (created_at DESC)
idx_transactions_status                ON (status)
idx_transactions_deleted_at            ON (deleted_at) WHERE deleted_at IS NOT NULL
```

**Foreign Keys:**
- `merchant_id` → `merchants(id)` ON DELETE RESTRICT
- `payment_method_id` → `customer_payment_methods(id)` ON DELETE SET NULL
- `subscription_id` → `subscriptions(id)` ON DELETE SET NULL

**Check Constraints:**
- `amount >= 0`
- `type IN ('auth', 'sale', 'capture', 'refund', 'void', 'pre_note')`

**Transaction Types:**

| Type | Description | Use Case |
|------|-------------|----------|
| `auth` | Authorization hold | Reserve funds, capture later |
| `sale` | Direct charge | Immediate payment |
| `capture` | Capture authorized funds | Complete auth transaction |
| `refund` | Return funds | Customer refund |
| `void` | Cancel transaction | Cancel before settlement |
| `pre_note` | ACH verification | Verify bank account (0.00 amount) |

**Status Values (Generated):**
- `approved` - `auth_resp = '00'`
- `declined` - `auth_resp != '00'`

**group_id Pattern:**

Transactions are linked by `group_id`:

```
┌─────────────────────────────────────────┐
│ group_id: 550e8400-e29b-41d4-a716-...   │
├─────────────────────────────────────────┤
│ 1. type='auth',    amount=100.00        │
│ 2. type='capture', amount=100.00        │
│ 3. type='refund',  amount=30.00         │
└─────────────────────────────────────────┘
```

**auth_guid Usage:**

Each transaction can have its own BRIC token:
- **AUTH** transaction: Gets initial BRIC from EPX
- **CAPTURE**: Uses AUTH's BRIC as input, gets new BRIC as output
- **REFUND**: Uses CAPTURE's BRIC as input, gets new BRIC as output

**Example:**

```sql
-- Create authorization
INSERT INTO transactions (
    id, group_id, merchant_id, customer_id, amount, type,
    payment_method_type, auth_guid, auth_resp, auth_code
) VALUES (
    '550e8400-...', '550e8400-...', 'merchant-uuid', 'CUST-123',
    100.00, 'auth', 'credit_card', 'BRIC-TOKEN-123', '00', 'AUTH123'
);

-- Capture authorization
INSERT INTO transactions (
    id, group_id, merchant_id, amount, type,
    payment_method_type, auth_guid, auth_resp
) VALUES (
    gen_random_uuid(), '550e8400-...', 'merchant-uuid',
    100.00, 'capture', 'credit_card', 'BRIC-TOKEN-456', '00'
);

-- Get transaction group
SELECT type, amount, status, auth_code, created_at
FROM transactions
WHERE group_id = '550e8400-...' AND deleted_at IS NULL
ORDER BY created_at;
```

---

### subscriptions

**Purpose:** Recurring billing schedules

**Table Structure:**

```sql
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id VARCHAR(100) NOT NULL,
    amount NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    interval_value INTEGER NOT NULL DEFAULT 1,
    interval_unit VARCHAR(10) NOT NULL DEFAULT 'month',
    status VARCHAR(20) NOT NULL,
    payment_method_id UUID NOT NULL REFERENCES customer_payment_methods(id) ON DELETE RESTRICT,
    next_billing_date DATE NOT NULL,
    failure_retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    gateway_subscription_id VARCHAR(255),
    metadata JSONB DEFAULT '{}'::jsonb,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cancelled_at TIMESTAMPTZ
);
```

**Column Reference:**

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | UUID | NOT NULL | Primary key (auto-generated) |
| `merchant_id` | UUID | NOT NULL | FK to `merchants.id` (multi-tenant isolation) |
| `customer_id` | VARCHAR(100) | NOT NULL | Customer identifier from client application |
| `amount` | NUMERIC(19,4) | NOT NULL | Recurring charge amount |
| `currency` | VARCHAR(3) | NOT NULL | ISO 4217 currency code (default: `USD`) |
| `interval_value` | INTEGER | NOT NULL | Billing interval count (e.g., 1, 2, 3) |
| `interval_unit` | VARCHAR(10) | NOT NULL | `day`, `week`, `month`, `year` |
| `status` | VARCHAR(20) | NOT NULL | `active`, `paused`, `cancelled`, `past_due` |
| `payment_method_id` | UUID | NOT NULL | FK to saved payment method (CASCADE RESTRICT) |
| `next_billing_date` | DATE | NOT NULL | Next billing date |
| `failure_retry_count` | INT | NOT NULL | Failed billing attempts counter |
| `max_retries` | INT | NOT NULL | Maximum retry attempts (default: 3) |
| `gateway_subscription_id` | VARCHAR(255) | NULL | EPX subscription ID (if supported) |
| `metadata` | JSONB | NOT NULL | Additional subscription data |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete timestamp |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last update timestamp (auto-updated via trigger) |
| `cancelled_at` | TIMESTAMPTZ | NULL | Cancellation timestamp |

**Indexes:**

```sql
idx_subscriptions_merchant_id           ON (merchant_id)
idx_subscriptions_merchant_customer     ON (merchant_id, customer_id)
idx_subscriptions_next_billing_date     ON (next_billing_date) WHERE status = 'active'
idx_subscriptions_status                ON (status)
idx_subscriptions_gateway_subscription_id  ON (gateway_subscription_id) WHERE gateway_subscription_id IS NOT NULL
idx_subscriptions_deleted_at            ON (deleted_at) WHERE deleted_at IS NOT NULL
```

**Foreign Keys:**
- `merchant_id` → `merchants(id)` ON DELETE RESTRICT
- `payment_method_id` → `customer_payment_methods(id)` ON DELETE RESTRICT

**Check Constraints:**
- `amount > 0`
- `failure_retry_count >= 0`
- `interval_value > 0`
- `interval_unit IN ('day', 'week', 'month', 'year')`
- `status IN ('active', 'paused', 'cancelled', 'past_due')`

**Status Values:**

| Status | Description |
|--------|-------------|
| `active` | Billing active |
| `paused` | Temporarily suspended |
| `cancelled` | Permanently cancelled |
| `past_due` | Failed billing, awaiting retry |

**Cascade Protection:**
- Cannot delete `payment_method` with active subscriptions
- Forces explicit subscription cancellation first

**Example:**

```sql
-- Create monthly subscription
INSERT INTO subscriptions (
    merchant_id, customer_id, amount, interval_value, interval_unit,
    status, payment_method_id, next_billing_date
) VALUES (
    'merchant-uuid', 'CUST-123', 29.99, 1, 'month',
    'active', 'payment-method-uuid', CURRENT_DATE + INTERVAL '1 month'
);

-- Find subscriptions due for billing
SELECT s.id, s.customer_id, s.amount, s.payment_method_id,
       pm.payment_token, pm.payment_type
FROM subscriptions s
JOIN customer_payment_methods pm ON s.payment_method_id = pm.id
WHERE s.merchant_id = $1 AND s.status = 'active'
  AND s.next_billing_date <= CURRENT_DATE
  AND s.deleted_at IS NULL AND pm.deleted_at IS NULL;
```

---

### chargebacks

**Purpose:** Dispute tracking and evidence management

**Table Structure:**

```sql
CREATE TABLE chargebacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID,
    agent_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(100),
    case_number VARCHAR(255) UNIQUE NOT NULL,
    dispute_date TIMESTAMPTZ NOT NULL,
    chargeback_date TIMESTAMPTZ NOT NULL,
    chargeback_amount VARCHAR(255) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    reason_code VARCHAR(50) NOT NULL,
    reason_description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'new',
    respond_by_date DATE,
    response_submitted_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    evidence_files TEXT[],
    response_notes TEXT,
    internal_notes TEXT,
    raw_data JSONB NOT NULL,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Column Reference:**

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | UUID | NOT NULL | Primary key (auto-generated) |
| `group_id` | UUID | NULL | Logical reference to `transactions.group_id` |
| `agent_id` | VARCHAR(100) | NOT NULL | Denormalized merchant ID for querying |
| `customer_id` | VARCHAR(100) | NULL | Customer identifier (NULL for guest) |
| `case_number` | VARCHAR(255) | NOT NULL | North Capital unique case ID |
| `dispute_date` | TIMESTAMPTZ | NOT NULL | When dispute was filed |
| `chargeback_date` | TIMESTAMPTZ | NOT NULL | When chargeback occurred |
| `chargeback_amount` | VARCHAR(255) | NOT NULL | Disputed amount (string for precision) |
| `currency` | VARCHAR(3) | NOT NULL | ISO 4217 currency code (default: `USD`) |
| `reason_code` | VARCHAR(50) | NOT NULL | Chargeback reason code (e.g., "P22", "F10") |
| `reason_description` | TEXT | NULL | Human-readable reason |
| `status` | VARCHAR(50) | NOT NULL | `new`, `pending`, `responded`, `won`, `lost`, `accepted` |
| `respond_by_date` | DATE | NULL | Response deadline |
| `response_submitted_at` | TIMESTAMPTZ | NULL | When evidence was submitted |
| `resolved_at` | TIMESTAMPTZ | NULL | Final outcome timestamp |
| `evidence_files` | TEXT[] | NULL | Array of S3/blob storage URLs |
| `response_notes` | TEXT | NULL | Written response to dispute |
| `internal_notes` | TEXT | NULL | Internal team notes |
| `raw_data` | JSONB | NOT NULL | Full North API response |
| `deleted_at` | TIMESTAMPTZ | NULL | Soft delete timestamp |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last update timestamp (auto-updated via trigger) |

**Indexes:**

```sql
idx_chargebacks_group_id            ON (group_id) WHERE group_id IS NOT NULL
idx_chargebacks_agent_id            ON (agent_id)
idx_chargebacks_agent_customer      ON (agent_id, customer_id) WHERE customer_id IS NOT NULL
idx_chargebacks_customer_id         ON (customer_id) WHERE customer_id IS NOT NULL
idx_chargebacks_status              ON (status)
idx_chargebacks_case_number         ON (case_number)
idx_chargebacks_respond_by_date     ON (respond_by_date) WHERE status = 'pending'
idx_chargebacks_created_at          ON (created_at DESC)
idx_chargebacks_deleted_at          ON (deleted_at) WHERE deleted_at IS NOT NULL
```

**Unique Constraints:**
- `case_number` - North API unique identifier

**Status Values:**

| Status | Description |
|--------|-------------|
| `new` | Chargeback received |
| `pending` | Awaiting response |
| `responded` | Evidence submitted |
| `won` | Dispute won |
| `lost` | Dispute lost |
| `accepted` | Chargeback accepted (no defense) |

**Why group_id instead of transaction_id?**
- Chargebacks can reference entire transaction flow (auth → capture → partial refund)
- `group_id` links all related transactions
- JOIN on `group_id` gets complete transaction history

**Example:**

```sql
-- Find pending chargebacks needing response
SELECT id, case_number, chargeback_amount, reason_description, respond_by_date
FROM chargebacks
WHERE agent_id = $1 AND status = 'pending'
  AND respond_by_date >= CURRENT_DATE AND deleted_at IS NULL
ORDER BY respond_by_date;

-- Link chargeback to transaction group
SELECT t.type, t.amount, t.status, t.created_at,
       c.case_number, c.chargeback_amount, c.reason_description
FROM chargebacks c
JOIN transactions t ON c.group_id = t.group_id
WHERE c.id = $1 AND t.deleted_at IS NULL
ORDER BY t.created_at;
```

---

### audit_logs

**Purpose:** Immutable audit trail for PCI compliance

**Table Structure:**

```sql
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    user_id VARCHAR(100),
    action VARCHAR(50) NOT NULL,
    before_state JSONB,
    after_state JSONB,
    metadata JSONB DEFAULT '{}'::jsonb,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Column Reference:**

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | BIGSERIAL | NOT NULL | Primary key (auto-incrementing) |
| `event_type` | VARCHAR(50) | NOT NULL | Event category (e.g., "payment.created") |
| `entity_type` | VARCHAR(50) | NOT NULL | Table/entity type (e.g., "transaction") |
| `entity_id` | VARCHAR(255) | NOT NULL | Record ID being audited |
| `merchant_id` | UUID | NOT NULL | FK to `merchants.id` (multi-tenant isolation) |
| `user_id` | VARCHAR(100) | NULL | User performing action (if applicable) |
| `action` | VARCHAR(50) | NOT NULL | `create`, `update`, `delete`, `read` |
| `before_state` | JSONB | NULL | Record state before change |
| `after_state` | JSONB | NULL | Record state after change |
| `metadata` | JSONB | NOT NULL | Additional context |
| `ip_address` | INET | NULL | Request IP address |
| `user_agent` | TEXT | NULL | Request user agent |
| `created_at` | TIMESTAMPTZ | NOT NULL | Event timestamp |

**Indexes:**

```sql
idx_audit_logs_entity         ON (entity_type, entity_id)
idx_audit_logs_merchant_id    ON (merchant_id)
idx_audit_logs_created_at     ON (created_at DESC)
idx_audit_logs_event_type     ON (event_type)
```

**Foreign Keys:**
- `merchant_id` → `merchants(id)` ON DELETE RESTRICT

**Immutability:**
- No updates or deletes allowed
- Append-only log
- Permanent record for compliance

**Example:**

```sql
-- Log payment method creation
INSERT INTO audit_logs (
    event_type, entity_type, entity_id, merchant_id,
    user_id, action, after_state, metadata, ip_address
) VALUES (
    'payment_method.created', 'customer_payment_methods', '550e8400-...',
    'merchant-uuid', 'USER-123', 'create',
    '{"payment_type": "credit_card", "last_four": "4242"}'::jsonb,
    '{"source": "api"}'::jsonb, '192.168.1.1'::inet
);

-- Query entity history
SELECT action, before_state, after_state, created_at
FROM audit_logs
WHERE entity_type = 'customer_payment_methods' AND entity_id = $1
ORDER BY created_at DESC;
```

---

### webhook_subscriptions

**Purpose:** Merchant webhook URLs for event notifications

**Table Structure:**

```sql
CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(100) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    webhook_url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Column Reference:**

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | UUID | NOT NULL | Primary key (auto-generated) |
| `agent_id` | VARCHAR(100) | NOT NULL | Merchant identifier |
| `event_type` | VARCHAR(50) | NOT NULL | Event type (e.g., `chargeback.created`) |
| `webhook_url` | TEXT | NOT NULL | Merchant's webhook endpoint URL |
| `secret` | VARCHAR(255) | NOT NULL | Webhook signature secret |
| `is_active` | BOOLEAN | NOT NULL | Active status |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes:**

```sql
idx_webhook_subscriptions_agent_event  ON (agent_id, event_type) WHERE is_active = true
```

**Unique Constraints:**
- `(agent_id, event_type, webhook_url)` - One webhook per agent per event

**Example:**

```sql
-- Register webhook
INSERT INTO webhook_subscriptions (agent_id, event_type, webhook_url, secret)
VALUES ('AGENT-123', 'chargeback.created', 'https://example.com/webhook', 'secret-key');

-- Find active webhooks for event
SELECT id, webhook_url, secret
FROM webhook_subscriptions
WHERE agent_id = $1 AND event_type = $2 AND is_active = true;
```

---

### webhook_deliveries

**Purpose:** Webhook delivery tracking and retry queue

**Table Structure:**

```sql
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL,
    http_status_code INT,
    error_message TEXT,
    attempts INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Column Reference:**

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| `id` | UUID | NOT NULL | Primary key (auto-generated) |
| `subscription_id` | UUID | NOT NULL | FK to `webhook_subscriptions.id` |
| `event_type` | VARCHAR(50) | NOT NULL | Event type |
| `payload` | JSONB | NOT NULL | Full webhook payload |
| `status` | VARCHAR(20) | NOT NULL | `pending`, `success`, `failed` |
| `http_status_code` | INT | NULL | HTTP response code from webhook |
| `error_message` | TEXT | NULL | Error details |
| `attempts` | INT | NOT NULL | Retry attempts counter |
| `next_retry_at` | TIMESTAMPTZ | NULL | Next retry timestamp |
| `delivered_at` | TIMESTAMPTZ | NULL | Successful delivery timestamp |
| `created_at` | TIMESTAMPTZ | NOT NULL | Record creation timestamp |

**Indexes:**

```sql
idx_webhook_deliveries_retry         ON (next_retry_at) WHERE status = 'pending' AND next_retry_at IS NOT NULL
idx_webhook_deliveries_subscription  ON (subscription_id, created_at DESC)
```

**Foreign Keys:**
- `subscription_id` → `webhook_subscriptions(id)` ON DELETE CASCADE

**Check Constraints:**
- `status IN ('pending', 'success', 'failed')`

**Cascade Behavior:**
- Deleting `webhook_subscription` deletes all delivery logs

**Example:**

```sql
-- Queue webhook delivery
INSERT INTO webhook_deliveries (
    subscription_id, event_type, payload, status, next_retry_at
) VALUES (
    'subscription-uuid', 'chargeback.created',
    '{"case_number": "12345", "amount": "100.00"}'::jsonb,
    'pending', CURRENT_TIMESTAMP
);

-- Get retry queue
SELECT wd.id, wd.payload, wd.attempts, ws.webhook_url
FROM webhook_deliveries wd
JOIN webhook_subscriptions ws ON wd.subscription_id = ws.id
WHERE wd.status = 'pending' AND wd.next_retry_at <= CURRENT_TIMESTAMP
ORDER BY wd.next_retry_at LIMIT 100;
```

---

## Relationships and Foreign Keys

### Entity Relationship Diagram

```
merchants (1) ────┬──< customer_payment_methods (N)
                  │
                  ├──< transactions (N)
                  │
                  ├──< subscriptions (N)
                  │
                  └──< audit_logs (N)

customer_payment_methods (1) ──< subscriptions (N)
                               └──< transactions (N)

subscriptions (1) ──< transactions (N)

transactions.group_id ──< chargebacks.group_id (logical)

webhook_subscriptions (1) ──< webhook_deliveries (N)
```

### Foreign Key Summary

| From Table | Column | To Table | To Column | On Delete |
|------------|--------|----------|-----------|-----------|
| `customer_payment_methods` | `merchant_id` | `merchants` | `id` | RESTRICT |
| `transactions` | `merchant_id` | `merchants` | `id` | RESTRICT |
| `transactions` | `payment_method_id` | `customer_payment_methods` | `id` | SET NULL |
| `transactions` | `subscription_id` | `subscriptions` | `id` | SET NULL |
| `subscriptions` | `merchant_id` | `merchants` | `id` | RESTRICT |
| `subscriptions` | `payment_method_id` | `customer_payment_methods` | `id` | RESTRICT |
| `audit_logs` | `merchant_id` | `merchants` | `id` | RESTRICT |
| `webhook_deliveries` | `subscription_id` | `webhook_subscriptions` | `id` | CASCADE |

**Cascade Rules Explained:**

**RESTRICT** (Prevent deletion if referenced):
- Cannot delete merchant with active data
- Cannot delete payment method with active subscriptions

**SET NULL** (Allow deletion, null reference):
- Can delete payment method even if used in transactions (history preserved)
- Can delete subscription without breaking transaction history

**CASCADE** (Delete child records):
- Deleting webhook subscription deletes all delivery logs

---

## Data Lifecycle

### Soft Delete Pattern

All main tables use soft deletes:

```sql
-- Soft delete
UPDATE transactions SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1;

-- Query active records
SELECT * FROM transactions WHERE merchant_id = $1 AND deleted_at IS NULL;

-- Query deleted records (admin only)
SELECT * FROM transactions WHERE deleted_at IS NOT NULL;
```

### Automatic Cleanup

**Function:** `cleanup_soft_deleted_records()`

**Step 1:** Soft delete abandoned PENDING transactions
- Marks transactions older than 1 hour as deleted
- Prevents accumulation of abandoned checkouts

**Step 2:** Permanently delete old soft-deleted records
- Deletes records where `deleted_at < NOW() - 90 days`
- Runs on: transactions, subscriptions, chargebacks, payment_methods

**Schedule (production):**

```sql
SELECT cron.schedule(
  'cleanup-soft-deleted-records',
  '0 2 * * *',  -- 2 AM daily
  'SELECT cleanup_soft_deleted_records();'
);
```

### Transaction Status Flow

```
Browser Post:
┌─────────┐
│ PENDING │ ──[EPX Approved]──> APPROVED
└─────────┘
     │
     └─[EPX Declined]──> DECLINED
     │
     └─[1 hour timeout]──> SOFT DELETED

Direct Post:
┌──────────┐
│ APPROVED │  (created directly)
└──────────┘
```

---

## Security and Compliance

### PCI Compliance

**✅ Card Data NEVER Stored:**
- Only EPX BRIC tokens stored
- Last 4 digits only (display purposes)
- No CVV, PIN, or full PAN
- Zero PCI scope for card data storage

**Pattern:**
```
1. User submits card to EPX (Browser Post API)
2. EPX returns BRIC token
3. Service stores only BRIC token in database
```

### Secret Management

**✅ MAC Secrets NEVER in Database:**
- Only OCI Vault path stored: `mac_secret_path`
- Actual MAC in OCI Vault
- Service reads at runtime via OCI SDK

**Pattern:**
```sql
-- Get vault path
SELECT mac_secret_path FROM merchants WHERE id = $1;
-- Returns: "ocid1.vaultsecret.oc1..."

-- Service then:
oci_client.GetSecret(secret_id=mac_secret_path)
-- Returns actual MAC for EPX signing
```

### Audit Trail

**All sensitive operations logged:**
- Payment method creation/deletion
- Transaction creation/updates
- Subscription changes
- Merchant credential changes

**Immutable log:**
- No updates or deletes
- Permanent compliance record

---

## Migration Management

### Migration Files

| File | Purpose |
|------|---------|
| `001_merchants.sql` | Merchant credentials table |
| `002_customer_payment_methods.sql` | Payment methods + update trigger function |
| `003_transactions.sql` | Transactions, subscriptions, audit_logs |
| `004_chargebacks.sql` | Chargeback tracking |
| `006_soft_delete_cleanup.sql` | Automatic cleanup function |
| `007_webhook_subscriptions.sql` | Webhook management |

### Running Migrations

```bash
# Apply all pending migrations
goose -dir internal/db/migrations postgres \
  "host=$DB_HOST port=$DB_PORT user=payment_service password=$DB_PASSWORD dbname=payment_service sslmode=require" \
  up

# Check migration status
goose -dir internal/db/migrations postgres "$DATABASE_URL" status

# Rollback one migration
goose -dir internal/db/migrations postgres "$DATABASE_URL" down
```

### Migration Best Practices

1. **Always test migrations locally first**
2. **Never modify existing migrations** (create new ones)
3. **Use transactions for atomic migrations**
4. **Include both Up and Down migrations**
5. **Test rollback (Down) before deploying**

---

## Query Performance Tips

### Multi-Tenant Queries

Always include `merchant_id` in WHERE clause:

```sql
-- ✅ Good: Uses merchant_id index
SELECT * FROM transactions
WHERE merchant_id = $1 AND customer_id = $2;

-- ❌ Bad: Missing merchant_id (slow query)
SELECT * FROM transactions WHERE customer_id = $1;
```

### Soft Delete Queries

Always exclude soft-deleted records:

```sql
-- ✅ Good: Filters out deleted records
SELECT * FROM transactions
WHERE merchant_id = $1 AND deleted_at IS NULL;

-- ❌ Bad: Includes deleted records
SELECT * FROM transactions WHERE merchant_id = $1;
```

### Index Usage

Use indexed columns in WHERE clauses:

```sql
-- ✅ Good: Uses idx_transactions_status
SELECT * FROM transactions WHERE status = 'approved';

-- ✅ Good: Uses idx_subscriptions_next_billing_date
SELECT * FROM subscriptions
WHERE status = 'active' AND next_billing_date <= CURRENT_DATE;
```

### Pagination

Use `created_at DESC` index for pagination:

```sql
-- Efficient pagination
SELECT * FROM transactions
WHERE merchant_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 50 OFFSET 0;
```

---

## Related Documentation

- **DATAFLOW.md** - Understanding payment flows and data movement
- **API_SPECS.md** - API endpoints that interact with database
- **DEVELOP.md** - Development and testing guidelines
- **CICD.md** - Database migrations in CI/CD pipeline