#!/bin/bash
# generate_schema_docs_enhanced.sh - Enhanced database schema documentation
# Usage: ./scripts/generate_schema_docs_enhanced.sh

set -e

echo "📊 Generating enhanced database schema documentation..."

OUTPUT_FILE="docs/development/DATABASE.md"
MIGRATIONS_DIR="internal/db/migrations"

# Create output file with comprehensive schema documentation
cat > "$OUTPUT_FILE" <<'EOF'
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

EOF

# Process each migration file
for migration in "$MIGRATIONS_DIR"/*.sql; do
    # Skip if no migrations found
    [ -f "$migration" ] || continue

    filename=$(basename "$migration")

    # Extract number and name from filename (format: 001_name.sql or 20231120000000_name.sql)
    if [[ "$filename" =~ ^([0-9]+)_(.+)\.sql$ ]]; then
        number="${BASH_REMATCH[1]}"
        name="${BASH_REMATCH[2]}"

        echo "### Migration $number: $name" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo "**File:** \`$filename\`" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo '```sql' >> "$OUTPUT_FILE"
        cat "$migration" >> "$OUTPUT_FILE"
        echo '```' >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo "---" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
    fi
done

# Add footer with performance considerations
cat >> "$OUTPUT_FILE" <<'EOF'

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

EOF

echo "✅ Enhanced schema documentation generated: $OUTPUT_FILE"
echo "   Found $(ls -1 $MIGRATIONS_DIR/*.sql 2>/dev/null | wc -l) migration files"
echo "   Includes: ERD, relationships, indexes, performance tips"
