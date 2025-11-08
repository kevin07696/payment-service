##Database Seeding Guide

Test data seeding for staging environment to enable comprehensive testing without manual data entry.

## Overview

The staging environment includes automated database seeding with:
- ✅ **Test customers** - 10+ users with various scenarios
- ✅ **Payment methods** - Cards (BRIC tokens) and ACH accounts
- ✅ **Transactions** - Historical payments (success, failed, pending, refunded)
- ✅ **Subscriptions** - Active, cancelled, and paused subscriptions
- ✅ **Chargebacks** - Dispute scenarios
- ✅ **EPX test scenarios** - Specific EPX sandbox test cases
- ✅ **Webhooks** - Example webhook history
- ✅ **Audit logs** - System event history

---

## Seed Files

Located in `db/seeds/staging/`:

```
db/seeds/staging/
├── 001_test_data.sql           # General test data
│   ├── 5 test customers
│   ├── 5 payment methods
│   ├── 7 transactions
│   ├── 5 subscriptions
│   ├── 3 chargebacks
│   └── 3 webhooks
│
└── 002_epx_test_scenarios.sql  # EPX-specific test data
    ├── EPX test card numbers
    ├── Response code examples (00, 05, 51, 54, 82)
    ├── Browser Post scenarios
    └── Recurring billing tests
```

---

## Automatic Seeding

Seeds are applied automatically after staging deployment.

### Workflow

```
Push to develop
    ↓
Deploy to staging
    ↓
Integration tests complete
    ↓
Auto-seed database (if needed)
    ↓
Staging ready with test data ✅
```

**Trigger**: After successful deployment to `develop` branch

**Logic**:
- Checks if test data already exists
- If not, applies all seed files
- Verifies data was created successfully
- Posts summary to workflow

---

## Manual Seeding

### Via GitHub Actions

```
Actions → Seed Staging Database → Run workflow

Options:
- Force seed: Override existing data (checkbox)
```

**Use when**:
- Want fresh test data
- Testing seed files
- Resetting staging environment

### Via Script (Local/SSH)

```bash
# On your local machine or SSH to staging
cd ~/Documents/projects/payments

# Set environment variables
export DB_HOST="123.45.67.89"
export DB_PASSWORD="your-db-password"

# Run seed script
./scripts/seed-staging.sh

# Or with options
./scripts/seed-staging.sh --force      # Override existing data
./scripts/seed-staging.sh --dry-run    # Preview what would happen
```

### Via Direct SQL

```bash
# PostgreSQL
psql -h YOUR_STAGING_IP -p 1522 -U payment_service -d paymentdb \
     -f db/seeds/staging/001_test_data.sql

psql -h YOUR_STAGING_IP -p 1522 -U payment_service -d paymentdb \
     -f db/seeds/staging/002_epx_test_scenarios.sql

# Oracle (if using SQL*Plus)
export TNS_ADMIN=~/oracle-wallet
sqlplus payment_service/PASSWORD@paymentdb_tp @db/seeds/staging/001_test_data.sql
sqlplus payment_service/PASSWORD@paymentdb_tp @db/seeds/staging/002_epx_test_scenarios.sql
```

---

## Test Data Reference

### General Test Customers

| Email | Name | Payment Method | Use Case |
|-------|------|----------------|----------|
| john.doe@example.com | John Doe | Visa ***1111 | Active subscriber, successful payments |
| jane.smith@example.com | Jane Smith | Visa ***4242 | One-time purchases |
| bob.wilson@example.com | Bob Wilson | ACH (checking) | ACH payment testing |
| alice.brown@example.com | Alice Brown | 2x Visa cards | Multiple payment methods |
| charlie.davis@example.com | Charlie Davis | - | No payment method |

### EPX Test Scenarios

| Email | Card | Expected Result | Use Case |
|-------|------|-----------------|----------|
| approved@epxtest.com | Visa ***1111 | Approved (00) | Successful transactions |
| declined@epxtest.com | Visa ***0002 | Declined (05) | Generic decline |
| insufficient@epxtest.com | Visa ***9995 | Insufficient funds (51) | Insufficient funds |
| expired@epxtest.com | Visa ***0001 | Expired (54) | Expired card |
| invalid@epxtest.com | Visa ***0003 | Invalid CVV (82) | CVV validation failure |
| browserpost@epxtest.com | Visa ***1111 | Approved (00) | Browser Post tokenization |

### Subscriptions

| ID | Customer | Amount | Status | Next Billing |
|----|----------|--------|--------|--------------|
| sub_test_001 | john.doe@example.com | $49.99/mo | active | Tomorrow |
| sub_test_002 | jane.smith@example.com | $99.99/mo | active | In 5 days |
| sub_test_003 | bob.wilson@example.com | $19.99/mo | active | In 15 days |
| sub_test_004 | alice.brown@example.com | $49.99/mo | cancelled | - |
| sub_test_005 | charlie.davis@example.com | $49.99/mo | paused | In 30 days |
| sub_epx_success | approved@epxtest.com | $29.99/mo | active | In 1 hour |
| sub_epx_fail | insufficient@epxtest.com | $49.99/mo | active | In 2 hours |

### Transaction Examples

| ID | Type | Status | Amount | Description |
|----|------|--------|--------|-------------|
| txn_test_001 | sale | success | $50.00 | Successful payment |
| txn_test_004 | sale | failed | $75.00 | Insufficient funds |
| txn_test_006 | auth | pending | $30.00 | Authorization (not captured) |
| txn_test_007 | refund | success | $50.00 | Refund processed |
| txn_epx_00 | sale | success | $50.00 | EPX approved (00) |
| txn_epx_51 | sale | failed | $150.00 | EPX insufficient funds (51) |

### Chargebacks

| ID | Transaction | Amount | Status | Reason |
|----|-------------|--------|--------|--------|
| cb_test_001 | txn_test_001 | $50.00 | pending | Fraudulent transaction |
| cb_test_002 | txn_test_002 | $100.00 | won | Not as described |
| cb_test_003 | txn_test_003 | $150.00 | lost | Authorization issue |

---

## Using Test Data

### Test Successful Payment

```bash
# Use approved@epxtest.com customer
# Card: Visa ending in 1111
# Expected: Transaction approved
```

### Test Failed Payment

```bash
# Use insufficient@epxtest.com customer
# Card: Visa ending in 9995
# Expected: Insufficient funds (code 51)
```

### Test Subscription Billing

```bash
# Wait for sub_epx_success next billing (in 1 hour after seed)
# Or manually trigger cron:
curl -H "X-Cron-Secret: YOUR_SECRET" http://STAGING_IP:8081/cron/run
```

### Test Chargebacks

```bash
# Query existing chargebacks
# Check polling from North API
# Test webhook notifications
```

### Test Browser Post Flow

```bash
# Use browserpost@epxtest.com
# Simulates tokenization from frontend
# BRIC token: BRIC_BROWSER_POST_TOKEN_001
```

---

## Seed File Structure

### 001_test_data.sql

```sql
BEGIN;

-- Insert customers
INSERT INTO customers (id, email, first_name, last_name, ...)
VALUES (...) ON CONFLICT (id) DO NOTHING;

-- Insert payment methods
INSERT INTO payment_methods (...)
VALUES (...) ON CONFLICT (id) DO NOTHING;

-- Insert transactions
INSERT INTO transactions (...)
VALUES (...) ON CONFLICT (id) DO NOTHING;

-- Print summary
DO $$
BEGIN
    RAISE NOTICE 'Seed data loaded successfully!';
END $$;

COMMIT;
```

**Features**:
- Uses `ON CONFLICT DO NOTHING` for idempotency
- Can be run multiple times safely
- Transaction-wrapped for atomicity
- Prints summary at end

### 002_epx_test_scenarios.sql

```sql
BEGIN;

-- EPX-specific test data
INSERT INTO customers (id, email, ...)
VALUES ('cust_epx_approved', 'approved@epxtest.com', ...)
ON CONFLICT (id) DO NOTHING;

-- EPX test card numbers (from EPX documentation)
INSERT INTO cards (payment_method_id, last_four, ...)
VALUES ('pm_epx_approved', '1111', ...)
ON CONFLICT (payment_method_id) DO NOTHING;

-- EPX response code examples
INSERT INTO transactions (gateway_response_code, ...)
VALUES ('00', ...), ('51', ...), ('82', ...);

COMMIT;
```

**Features**:
- Based on EPX sandbox documentation
- Covers all major response codes
- Includes auth/capture/void scenarios

---

## Extending Seed Data

### Add New Test Customer

Edit `db/seeds/staging/001_test_data.sql`:

```sql
INSERT INTO customers (id, email, first_name, last_name, phone, created_at, updated_at)
VALUES
    ('cust_test_006', 'new.user@example.com', 'New', 'User', '+1234567895', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
```

### Add New Test Scenario

Create `db/seeds/staging/003_my_scenario.sql`:

```sql
BEGIN;

-- Your test data
INSERT INTO ...;

-- Summary
DO $$
BEGIN
    RAISE NOTICE 'My scenario loaded!';
END $$;

COMMIT;
```

Files are applied in alphabetical order (001, 002, 003, ...).

---

## Troubleshooting

### Seed Script Fails - "Test data already exists"

**Solution**: Use `--force` flag

```bash
./scripts/seed-staging.sh --force
```

Or via GitHub Actions: Check "Force seed" option

### Seed Script Fails - "Connection refused"

**Possible causes**:
- Database not accessible
- Wrong host/port
- Firewall blocking

**Solutions**:
```bash
# Test connection
psql -h YOUR_IP -p 1522 -U payment_service -d paymentdb -c "SELECT 1"

# Check if database is running
ssh -i ~/.ssh/oracle-staging ubuntu@YOUR_IP
docker logs payment-staging | grep -i postgres
```

### Seed Script Fails - "Permission denied"

**Possible causes**:
- Wrong database user
- User lacks CREATE/INSERT permissions

**Solutions**:
```bash
# Verify user permissions
psql -h YOUR_IP -p 1522 -U payment_service -d paymentdb \
     -c "SELECT has_table_privilege('payment_service', 'customers', 'INSERT')"
```

### Seed Data Not Appearing in Application

**Possible causes**:
- Cache issue
- Application connected to different database
- Data committed but not visible

**Solutions**:
```bash
# Verify data exists
psql -h YOUR_IP -p 1522 -U payment_service -d paymentdb \
     -c "SELECT COUNT(*) FROM customers WHERE id LIKE 'cust_test_%'"

# Restart application
ssh -i ~/.ssh/oracle-staging ubuntu@YOUR_IP
cd ~/payment-service
docker-compose restart
```

### Want to Clear All Test Data

```sql
-- ⚠️  DANGER: This deletes ALL test data!

BEGIN;

DELETE FROM audit_logs WHERE entity_id LIKE '%test%' OR entity_id LIKE '%epx%';
DELETE FROM webhooks WHERE id LIKE 'wh_test%';
DELETE FROM chargebacks WHERE id LIKE 'cb_test%';
DELETE FROM subscription_billings WHERE id LIKE 'sbill_test%';
DELETE FROM subscriptions WHERE id LIKE 'sub_test%' OR id LIKE 'sub_epx%';
DELETE FROM transactions WHERE id LIKE 'txn_test%' OR id LIKE 'txn_epx%';
DELETE FROM ach_accounts WHERE payment_method_id LIKE 'pm_test%' OR payment_method_id LIKE 'pm_epx%';
DELETE FROM cards WHERE payment_method_id LIKE 'pm_test%' OR payment_method_id LIKE 'pm_epx%' OR payment_method_id LIKE 'pm_browserpost%';
DELETE FROM payment_methods WHERE id LIKE 'pm_test%' OR id LIKE 'pm_epx%' OR id LIKE 'pm_browserpost%';
DELETE FROM customers WHERE id LIKE 'cust_test%' OR id LIKE 'cust_epx%' OR id = 'cust_browserpost';

COMMIT;
```

Then re-run seed script to recreate.

---

## Best Practices

### 1. Use Idempotent Seeds

✅ Use `ON CONFLICT DO NOTHING`
✅ Can run multiple times without errors
✅ Safe to re-seed anytime

### 2. Organize by Purpose

✅ General test data in 001_*
✅ EPX-specific in 002_*
✅ Feature-specific in 003_*+

### 3. Include Variety

✅ Success cases
✅ Failure cases
✅ Edge cases
✅ Realistic scenarios

### 4. Document Test Data

✅ Comment why data exists
✅ Reference EPX docs
✅ Explain expected behavior

### 5. Keep Seeds Fast

✅ Minimal necessary data
✅ ~100 records max
✅ Should complete in < 30 seconds

---

## Integration with Testing

### Integration Tests Can Use Seed Data

```go
func TestWithSeedData(t *testing.T) {
    // Test against john.doe@example.com (seeded customer)
    // Known payment method: Visa ***1111
    // Expected: Transaction succeeds
}
```

### Manual QA Can Use Seed Data

- Log in as test users
- View payment history
- Test subscription management
- Verify webhook delivery

### Demos Can Use Seed Data

- Show realistic data
- Demonstrate features
- Walk through flows
- Explain integrations

---

## Related Documentation

- [Integration Testing](./INTEGRATION_TESTING.md) - Automated tests
- [Staging Lifecycle](./STAGING_LIFECYCLE.md) - Auto-create/destroy
- [Branching Strategy](./BRANCHING_STRATEGY.md) - Deployment workflow

---

**Seed data makes staging useful from day one - no manual data entry required!** 🌱✅
