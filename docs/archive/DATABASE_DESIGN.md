# Database Design

## Architecture Overview

**Purpose**: Payment gateway integration service database

**Design Principles**:
- Multi-tenant by `agent_id` (merchant/reseller isolation)
- Soft deletes with automatic cleanup (90-day retention)
- Idempotency via unique constraints
- PCI compliance via tokenization (no raw card data)
- Audit trail for compliance

**Database**: PostgreSQL 15+

**Migration Tool**: Goose

## Core Entities

### Transactions

**Purpose**: All payment activity (charges, refunds, authorizations, captures, voids)

**Key Features**:
- `group_id` links related transactions (auth → capture → refund)
- `idempotency_key` prevents duplicate processing
- Stores EPX gateway response fields for refunds/voids/recurring
- No coupling to external systems (POS, e-commerce, etc.)
- Status = state (pending/completed/failed)
- Type = operation (charge/refund/void/etc.)

**Table**: `transactions`

| Column | Type | Purpose |
|--------|------|---------|
| `id` | UUID | Primary key |
| `group_id` | UUID | Groups related transactions (auth → capture → refund) |
| `agent_id` | VARCHAR(100) | Multi-tenant: which merchant |
| `customer_id` | VARCHAR(100) | Customer ID (NULL for guest transactions) |
| `amount` | NUMERIC(19,4) | Transaction amount |
| `currency` | VARCHAR(3) | ISO 4217 code (default: USD) |
| `status` | VARCHAR(20) | **pending, completed, failed** |
| `type` | VARCHAR(20) | **charge, refund, void, pre_note, auth, capture** |
| `payment_method_type` | VARCHAR(20) | credit_card, ach |
| `payment_method_id` | UUID | FK to saved payment method (NULL for guest) |
| `auth_guid` | VARCHAR(255) | EPX BRIC token (required for refunds/voids/recurring) |
| `auth_resp` | VARCHAR(10) | EPX approval code (00=approved) |
| `auth_code` | VARCHAR(50) | Bank authorization code (for chargeback defense) |
| `auth_resp_text` | TEXT | Human-readable response (display to users) |
| `auth_card_type` | VARCHAR(20) | Card brand (V/M/A/D, NULL for ACH) |
| `auth_avs` | VARCHAR(10) | Address verification (fraud prevention) |
| `auth_cvv2` | VARCHAR(10) | CVV verification (fraud prevention) |
| `idempotency_key` | VARCHAR(255) | Unique TRAN_NBR from merchant |
| `metadata` | JSONB | Additional EPX fields |
| `deleted_at` | TIMESTAMPTZ | Soft delete timestamp |
| `created_at` | TIMESTAMPTZ | Record creation |
| `updated_at` | TIMESTAMPTZ | Last update (auto-updated) |

**Status Values**:
- `pending` - Transaction created, awaiting completion
- `completed` - Transaction successfully processed
- `failed` - Transaction declined or error

**Type Values**:
- `charge` - Debit customer (sale)
- `refund` - Return funds to customer
- `void` - Cancel transaction before settlement
- `pre_note` - ACH account verification
- `auth` - Authorization hold (no capture)
- `capture` - Capture previously authorized funds

**Indexes**:
- `group_id` - Group related transactions
- `agent_id` - Multi-tenant queries
- `agent_id, customer_id` - Customer transaction history
- `idempotency_key` - Duplicate prevention
- `auth_guid` - Lookup for refunds/voids
- `created_at DESC` - Recent transactions
- `status` - Filter by status
- `deleted_at` - Soft delete queries

**Why Store auth_guid?**
- Required for refunds and voids
- Can be reused for recurring payments (13-24 month lifetime)
- Can be converted to Storage BRIC (never expires)
- Required for chargeback defense
- Required for reconciliation with EPX reports

### Customer Payment Methods

**Purpose**: Saved payment methods (cards and bank accounts)

**Security**: Only stores EPX BRIC tokens and last 4 digits (PCI compliant)

**Table**: `customer_payment_methods`

| Column | Type | Purpose |
|--------|------|---------|
| `id` | UUID | Primary key |
| `agent_id` | VARCHAR(255) | Multi-tenant: which merchant |
| `customer_id` | VARCHAR(255) | Which customer |
| `payment_token` | TEXT | EPX Storage BRIC token |
| `payment_type` | VARCHAR(20) | credit_card, ach |
| `last_four` | VARCHAR(4) | Last 4 digits (display only) |
| `card_brand` | VARCHAR(20) | visa, mastercard, amex, discover |
| `card_exp_month` | INTEGER | 1-12 (for expiration warnings) |
| `card_exp_year` | INTEGER | YYYY format |
| `bank_name` | VARCHAR(255) | User-provided label (ACH only) |
| `account_type` | VARCHAR(20) | checking, savings (ACH only) |
| `is_default` | BOOLEAN | Default payment method |
| `is_active` | BOOLEAN | Active/inactive |
| `is_verified` | BOOLEAN | ACH pre-note verification status |
| `deleted_at` | TIMESTAMPTZ | Soft delete timestamp |
| `created_at` | TIMESTAMPTZ | Record creation |
| `updated_at` | TIMESTAMPTZ | Last update (auto-updated) |
| `last_used_at` | TIMESTAMPTZ | Last transaction timestamp |

**Indexes**:
- `agent_id, customer_id` - List customer payment methods
- `agent_id, customer_id, is_default` - Find default payment method
- `is_active` - Active payment methods only
- `payment_type` - Filter by type

**Unique Constraint**: `(agent_id, customer_id, payment_token)` - Prevent duplicate tokens

**Security Notes**:
- NEVER store full card numbers or bank account numbers
- Only store EPX BRIC tokens (already tokenized)
- Last 4 digits only for display purposes
- Expiration dates for warning users before card expires

### Subscriptions

**Purpose**: Recurring billing schedules

**Table**: `subscriptions`

| Column | Type | Purpose |
|--------|------|---------|
| `id` | UUID | Primary key |
| `agent_id` | VARCHAR(100) | Multi-tenant: which merchant |
| `customer_id` | VARCHAR(100) | Which customer |
| `amount` | NUMERIC(19,4) | Recurring amount |
| `currency` | VARCHAR(3) | ISO 4217 code (default: USD) |
| `interval_value` | INTEGER | Billing interval (1, 2, 3, etc.) |
| `interval_unit` | VARCHAR(10) | day, week, month, year |
| `status` | VARCHAR(20) | active, paused, cancelled, past_due |
| `payment_method_id` | UUID | FK to customer_payment_methods (CASCADE RESTRICT) |
| `next_billing_date` | DATE | Next charge date |
| `failure_retry_count` | INT | Failed attempts counter |
| `max_retries` | INT | Max retry attempts (default: 3) |
| `gateway_subscription_id` | VARCHAR(255) | EPX subscription ID (optional) |
| `metadata` | JSONB | Additional data |
| `deleted_at` | TIMESTAMPTZ | Soft delete timestamp |
| `created_at` | TIMESTAMPTZ | Record creation |
| `updated_at` | TIMESTAMPTZ | Last update (auto-updated) |
| `cancelled_at` | TIMESTAMPTZ | Cancellation timestamp |

**Indexes**:
- `agent_id, customer_id` - List customer subscriptions
- `next_billing_date WHERE status='active'` - Billing queue
- `status` - Filter by status
- `gateway_subscription_id` - Lookup by EPX ID

**Cascade Protection**: Cannot delete payment method with active subscriptions (`ON DELETE RESTRICT`)

### Chargebacks

**Purpose**: Dispute tracking and evidence management

**Integration**: North Capital disputes API

**Table**: `chargebacks`

| Column | Type | Purpose |
|--------|------|---------|
| `id` | UUID | Primary key |
| `group_id` | UUID | References transactions.group_id |
| `agent_id` | VARCHAR(100) | Multi-tenant: which merchant |
| `customer_id` | VARCHAR(100) | Which customer (NULL for guest) |
| `case_number` | VARCHAR(255) | North's unique case ID |
| `dispute_date` | TIMESTAMPTZ | When dispute was filed |
| `chargeback_date` | TIMESTAMPTZ | When chargeback occurred |
| `chargeback_amount` | VARCHAR(255) | Disputed amount (string for precision) |
| `currency` | VARCHAR(3) | ISO 4217 code (default: USD) |
| `reason_code` | VARCHAR(50) | Chargeback reason (P22, F10, etc.) |
| `reason_description` | TEXT | Human-readable reason |
| `status` | VARCHAR(50) | new, pending, responded, won, lost, accepted |
| `respond_by_date` | DATE | Response deadline |
| `response_submitted_at` | TIMESTAMPTZ | When evidence submitted |
| `resolved_at` | TIMESTAMPTZ | Final outcome timestamp |
| `evidence_files` | TEXT[] | S3/blob storage URLs |
| `response_notes` | TEXT | Written response to dispute |
| `internal_notes` | TEXT | Internal team notes |
| `raw_data` | JSONB | Full North API response |
| `deleted_at` | TIMESTAMPTZ | Soft delete timestamp |
| `created_at` | TIMESTAMPTZ | Record creation |
| `updated_at` | TIMESTAMPTZ | Last update (auto-updated) |

**Indexes**:
- `group_id` - Link to transaction
- `agent_id, customer_id` - Merchant/customer disputes
- `case_number` - Unique North case ID
- `status` - Filter by status
- `respond_by_date WHERE status='pending'` - Response deadline queue
- `created_at DESC` - Recent disputes

**Unique Constraint**: `case_number` - North API unique identifier

**Why group_id instead of transaction_id?**
- Chargebacks can reference entire transaction flow (auth → capture → partial refund)
- `group_id` links all related transactions
- JOIN on `group_id` gets complete transaction history

### Agent Credentials

**Purpose**: Multi-tenant merchant EPX credentials

**Security**: MAC secrets stored in OCI Vault (only path stored in DB)

**Table**: `agent_credentials`

| Column | Type | Purpose |
|--------|------|---------|
| `id` | UUID | Primary key |
| `agent_id` | VARCHAR(255) | Unique agent identifier |
| `mac_secret_path` | TEXT | OCI Vault secret path |
| `cust_nbr` | VARCHAR(50) | EPX customer number |
| `merch_nbr` | VARCHAR(50) | EPX merchant number |
| `dba_nbr` | VARCHAR(50) | EPX DBA number |
| `terminal_nbr` | VARCHAR(50) | EPX terminal number |
| `environment` | VARCHAR(20) | test, production |
| `agent_name` | VARCHAR(255) | Display name |
| `is_active` | BOOLEAN | Active/inactive |
| `deleted_at` | TIMESTAMPTZ | Soft delete timestamp |
| `created_at` | TIMESTAMPTZ | Record creation |
| `updated_at` | TIMESTAMPTZ | Last update (auto-updated) |

**Indexes**:
- `agent_id` - Unique lookup
- `is_active` - Active agents only

**Unique Constraints**:
- `agent_id` - One record per agent
- `mac_secret_path` - One vault secret per agent

**Security Pattern**:
1. Service queries `agent_credentials` by `agent_id`
2. Gets `mac_secret_path` (OCI Vault OCID)
3. Uses OCI SDK to read actual MAC secret
4. MAC never stored in database

### Audit Logs

**Purpose**: PCI compliance audit trail

**Table**: `audit_logs`

| Column | Type | Purpose |
|--------|------|---------|
| `id` | BIGSERIAL | Primary key |
| `event_type` | VARCHAR(50) | Event category |
| `entity_type` | VARCHAR(50) | Table/entity type |
| `entity_id` | VARCHAR(255) | Record ID |
| `agent_id` | VARCHAR(100) | Which merchant |
| `user_id` | VARCHAR(100) | Which user (if applicable) |
| `action` | VARCHAR(50) | create, update, delete, etc. |
| `before_state` | JSONB | Record state before change |
| `after_state` | JSONB | Record state after change |
| `metadata` | JSONB | Additional context |
| `ip_address` | INET | Request IP |
| `user_agent` | TEXT | Request user agent |
| `created_at` | TIMESTAMPTZ | Event timestamp |

**Indexes**:
- `entity_type, entity_id` - Lookup entity history
- `agent_id` - Merchant audit trail
- `created_at DESC` - Recent events
- `event_type` - Filter by event type

**No Updates/Deletes**: Immutable append-only log

### Webhook Subscriptions

**Purpose**: Merchant webhook URLs for event notifications

**Table**: `webhook_subscriptions`

| Column | Type | Purpose |
|--------|------|---------|
| `id` | UUID | Primary key |
| `agent_id` | VARCHAR(100) | Which merchant |
| `event_type` | VARCHAR(50) | chargeback.created, chargeback.updated, etc. |
| `webhook_url` | TEXT | Merchant's webhook endpoint |
| `secret` | VARCHAR(255) | Webhook signature secret |
| `is_active` | BOOLEAN | Active/inactive |
| `created_at` | TIMESTAMPTZ | Record creation |
| `updated_at` | TIMESTAMPTZ | Last update (auto-updated) |

**Indexes**:
- `agent_id, event_type WHERE is_active=true` - Active webhooks lookup

**Unique Constraint**: `(agent_id, event_type, webhook_url)` - One webhook per agent per event

### Webhook Deliveries

**Purpose**: Webhook delivery tracking and retry queue

**Table**: `webhook_deliveries`

| Column | Type | Purpose |
|--------|------|---------|
| `id` | UUID | Primary key |
| `subscription_id` | UUID | FK to webhook_subscriptions (CASCADE) |
| `event_type` | VARCHAR(50) | Event type |
| `payload` | JSONB | Full webhook payload |
| `status` | VARCHAR(20) | pending, success, failed |
| `http_status_code` | INT | HTTP response code |
| `error_message` | TEXT | Error details |
| `attempts` | INT | Retry attempts counter |
| `next_retry_at` | TIMESTAMPTZ | Next retry timestamp |
| `delivered_at` | TIMESTAMPTZ | Successful delivery timestamp |
| `created_at` | TIMESTAMPTZ | Record creation |

**Indexes**:
- `next_retry_at WHERE status='pending'` - Retry queue
- `subscription_id, created_at DESC` - Delivery history

**Cascade**: Deleting webhook subscription deletes delivery logs

## Data Lifecycle

### Transaction Status Flow

**Checkout Flow**:
```
pending → completed  (EPX approved)
pending → failed     (EPX declined)
pending → [soft deleted after 1 hour] (abandoned checkout)
```

**Refund Flow** (creates new transaction):
```
Original: type='charge', status='completed'
Refund:   type='refund', status='completed'
Both linked by group_id
```

**Void Flow** (creates new transaction):
```
Original: type='auth', status='completed'
Void:     type='void', status='completed'
Both linked by group_id
```

### Soft Delete Pattern

```sql
-- Soft delete
UPDATE transactions
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = '550e8400-...';

-- Query active records
SELECT * FROM transactions
WHERE agent_id = 'AGENT-123'
  AND deleted_at IS NULL;

-- Query deleted records (admin only)
SELECT * FROM transactions
WHERE agent_id = 'AGENT-123'
  AND deleted_at IS NOT NULL;
```

### Automatic Cleanup

Function: `cleanup_soft_deleted_records()`

**Step 1: Soft delete abandoned PENDING transactions**
- Marks PENDING transactions older than 1 hour as deleted
- Prevents accumulation of abandoned checkouts

**Step 2: Permanently delete old soft-deleted records**
- Deletes records where `deleted_at < NOW() - 90 days`
- Runs on all tables: transactions, subscriptions, chargebacks, payment methods, agent credentials

**Schedule (production)**:
```sql
SELECT cron.schedule(
  'cleanup-soft-deleted-records',
  '0 2 * * *',  -- 2 AM daily
  'SELECT cleanup_soft_deleted_records();'
);
```

## Relationships

### Transaction Flow

```
customer_payment_methods (saved card/bank)
         ↓
    transactions (charge using saved method)
         ↓
    chargebacks (dispute on transaction)
         ↓
    audit_logs (all changes tracked)
```

### Subscription Flow

```
customer_payment_methods (saved card/bank)
         ↓
    subscriptions (recurring billing schedule)
         ↓
    transactions (each billing creates transaction)
         ↓
    audit_logs (all changes tracked)
```

### Multi-Tenant Pattern

All tables have `agent_id`:
- Partition by merchant/reseller
- Queries always filtered by `agent_id`
- Prevents cross-tenant data leakage

**Example**:
```sql
-- Always include agent_id in WHERE clause
SELECT * FROM transactions
WHERE agent_id = 'AGENT-123'
  AND customer_id = 'CUST-456';
```

### Group ID Pattern

`group_id` links related transactions:

**Example**: Authorization → Capture → Partial Refund
```sql
-- Original authorization
INSERT INTO transactions (group_id, type, amount, status)
VALUES ('550e8400-...', 'auth', 100.00, 'completed');

-- Capture authorization
INSERT INTO transactions (group_id, type, amount, status)
VALUES ('550e8400-...', 'capture', 100.00, 'completed');

-- Partial refund
INSERT INTO transactions (group_id, type, amount, status)
VALUES ('550e8400-...', 'refund', 30.00, 'completed');

-- Get complete flow
SELECT type, amount, status, created_at
FROM transactions
WHERE group_id = '550e8400-...'
ORDER BY created_at;
```

### Foreign Keys

| From Table | To Table | Relationship | On Delete |
|------------|----------|--------------|-----------|
| `transactions.payment_method_id` | `customer_payment_methods.id` | Optional (NULL for guest) | SET NULL |
| `subscriptions.payment_method_id` | `customer_payment_methods.id` | Required | RESTRICT |
| `webhook_deliveries.subscription_id` | `webhook_subscriptions.id` | Required | CASCADE |

**Why RESTRICT on subscriptions?**
- Cannot delete payment method with active subscriptions
- Forces explicit subscription cancellation first

**Why SET NULL on transactions?**
- Historical transactions remain valid even if payment method deleted
- `auth_guid` field preserved for refunds/voids

## Migration Files

| File | Purpose |
|------|---------|
| `000_init_schema.sql` | Goose version tracking |
| `001_customer_payment_methods.sql` | Saved payment methods + update trigger |
| `002_transactions.sql` | Transactions, subscriptions, audit logs |
| `003_chargebacks.sql` | Dispute tracking |
| `004_agent_credentials.sql` | Multi-tenant EPX credentials |
| `005_soft_delete_cleanup.sql` | Automatic cleanup function + PENDING cleanup |
| `007_webhook_subscriptions.sql` | Webhook management |

**Missing 006**: Optional pg_cron setup (not required)

**Missing 008**: Deleted (attempted to remove phantom fields)

## Query Patterns

### List Customer Transactions

```sql
SELECT t.id, t.group_id, t.type, t.amount, t.status,
       t.created_at, pm.last_four, pm.card_brand
FROM transactions t
LEFT JOIN customer_payment_methods pm ON t.payment_method_id = pm.id
WHERE t.agent_id = $1
  AND t.customer_id = $2
  AND t.deleted_at IS NULL
ORDER BY t.created_at DESC
LIMIT 50;
```

### Find Transaction Group

```sql
SELECT type, amount, status, auth_code, created_at
FROM transactions
WHERE group_id = $1
  AND deleted_at IS NULL
ORDER BY created_at;
```

### Active Subscriptions Due for Billing

```sql
SELECT s.id, s.customer_id, s.amount, s.payment_method_id,
       pm.payment_token, pm.payment_type
FROM subscriptions s
JOIN customer_payment_methods pm ON s.payment_method_id = pm.id
WHERE s.agent_id = $1
  AND s.status = 'active'
  AND s.next_billing_date <= CURRENT_DATE
  AND s.deleted_at IS NULL
  AND pm.deleted_at IS NULL
  AND pm.is_active = true;
```

### Pending Chargebacks Needing Response

```sql
SELECT id, case_number, chargeback_amount, reason_description, respond_by_date
FROM chargebacks
WHERE agent_id = $1
  AND status = 'pending'
  AND respond_by_date >= CURRENT_DATE
  AND deleted_at IS NULL
ORDER BY respond_by_date;
```

### Webhook Retry Queue

```sql
SELECT wd.id, wd.subscription_id, wd.payload, wd.attempts,
       ws.webhook_url, ws.secret
FROM webhook_deliveries wd
JOIN webhook_subscriptions ws ON wd.subscription_id = ws.id
WHERE wd.status = 'pending'
  AND wd.next_retry_at <= CURRENT_TIMESTAMP
  AND ws.is_active = true
ORDER BY wd.next_retry_at
LIMIT 100;
```

## Security & Compliance

### PCI Compliance

**Card Data**: NEVER stored
- Only EPX BRIC tokens stored
- Last 4 digits only (display purposes)
- Full card numbers NEVER touch database

**Pattern**:
1. User submits card to EPX (Browser Post API)
2. EPX returns BRIC token
3. Service stores only BRIC token

**Benefit**: Zero PCI scope for card data storage

### Secret Management

**MAC Secrets**: NEVER in database
- Only vault path stored: `payments/agents/agent1/mac`
- Actual MAC in OCI Vault
- Service reads at runtime via OCI SDK

**Pattern**:
```sql
SELECT mac_secret_path FROM agent_credentials WHERE agent_id = 'AGENT-123';
-- Returns: "ocid1.vaultsecret.oc1..."

-- Service then:
oci_client.GetSecret(secret_id=mac_secret_path)
-- Returns actual MAC for EPX signing
```

### Audit Trail

All sensitive operations logged to `audit_logs`:
- Payment method creation/deletion
- Transaction creation/updates
- Subscription changes
- Agent credential changes

**Immutable**: No updates or deletes allowed

## References

- Migrations: `internal/db/migrations/`
- Queries: `internal/db/queries/`
- Schema seeds: `internal/db/seeds/staging/`
- EPX API: `docs/EPX_API_REFERENCE.md`
- Testing guide: `docs/TESTING.md`
