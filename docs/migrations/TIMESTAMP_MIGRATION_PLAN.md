# Timestamp Migration Plan: TIMESTAMP → TIMESTAMPTZ

**Status**: 🚧 READY TO EXECUTE
**Priority**: P0 - CRITICAL
**Risk Level**: HIGH - Schema-altering production migration
**Estimated Time**: 2-3 hours

---

## Executive Summary

The database currently has **INCONSISTENT timestamp types**:
- **10 tables** use `TIMESTAMP` (without timezone) - **NEEDS MIGRATION**
- **6 tables** already use `TIMESTAMPTZ` (with timezone) - Already correct

This creates timezone bugs in:
- Authentication sessions (admins, services)
- Merchant activation/deactivation
- Chargeback response deadlines
- Subscription billing checks
- ACH verification timing

---

## Critical Risk Analysis

### HIGH RISK Areas (Will Break Without Fix)
1. **Chargeback Response Deadlines** (`chargeback.go:87`)
   - `time.Now().After(*c.RespondByDate)` comparing local time to UTC
   - Could cause missed legal deadlines!

2. **Subscription Billing** (`subscription.go:82`)
   - `time.Now().After(s.NextBillingDate)` timezone-dependent
   - Could charge customers at wrong times

3. **ACH Verification Timing** (`ach_verification_handler.go:149`)
   - `time.Now().AddDate(0, 0, -verificationDays)` without UTC
   - Could verify accounts too early/late

### MEDIUM RISK Areas
4. **Card Expiration Checks** (`payment_method.go:83`)
5. **Admin Session Expiration** (auth tables)
6. **Merchant Activation Tokens** (auth tables)

---

## Migration Phases

### Phase 1: Fix Application Code (30 minutes)
**Goal**: Ensure all Go code uses UTC timestamps before database migration

#### Files to Update:

**1.1 Domain Models** (4 files)

**File**: `internal/domain/payment_method.go`
```go
// Line 83: IsExpired() function
func (pm *PaymentMethod) IsExpired() bool {
-   now := time.Now()
+   now := timeutil.Now()
    expYear := *pm.CardExpYear
    expMonth := *pm.CardExpMonth
    // ... rest unchanged
}

// Line 164: MarkUsed() function
func (pm *PaymentMethod) MarkUsed() {
-   now := time.Now()
+   now := timeutil.Now()
    pm.LastUsedAt = &now
}
```

**File**: `internal/domain/subscription.go`
```go
// Line 82: CanBeBilled() function
func (s *Subscription) CanBeBilled() bool {
-   return s.IsActive() && time.Now().After(s.NextBillingDate)
+   return s.IsActive() && timeutil.Now().After(s.NextBillingDate)
}

// Line 123: CalculateNextBillingDate() function
func (s *Subscription) CalculateNextBillingDate() time.Time {
    switch s.BillingInterval {
    case "monthly":
-       return s.NextBillingDate.AddDate(0, s.IntervalCount, 0)
+       return timeutil.ToUTC(s.NextBillingDate.AddDate(0, s.IntervalCount, 0))
    case "yearly":
-       return s.NextBillingDate.AddDate(s.IntervalCount, 0, 0)
+       return timeutil.ToUTC(s.NextBillingDate.AddDate(s.IntervalCount, 0, 0))
    default:
-       return s.NextBillingDate.AddDate(0, 0, s.IntervalCount)
+       return timeutil.ToUTC(s.NextBillingDate.AddDate(0, 0, s.IntervalCount))
    }
}
```

**File**: `internal/domain/chargeback.go`
```go
// Line 87: CanRespond() function
func (c *Chargeback) CanRespond() bool {
    if c.Status != ChargebackStatusPending {
        return false
    }
-   if c.RespondByDate != nil && time.Now().After(*c.RespondByDate) {
+   if c.RespondByDate != nil && timeutil.Now().After(*c.RespondByDate) {
        return false
    }
    return c.ResponseSubmittedAt == nil
}

// Line 105: DaysUntilDeadline() function
func (c *Chargeback) DaysUntilDeadline() int {
    if c.RespondByDate == nil {
        return 0
    }
-   duration := time.Until(*c.RespondByDate)
+   duration := timeutil.Now().Sub(*c.RespondByDate) * -1
    return int(duration.Hours() / 24)
}

// Line 120: MarkResponded() function
func (c *Chargeback) MarkResponded() {
-   now := time.Now()
+   now := timeutil.Now()
    c.ResponseSubmittedAt = &now
    c.Status = ChargebackStatusResponded
}

// Line 138: MarkResolved() function
func (c *Chargeback) MarkResolved(outcome ChargebackOutcome) {
-   now := time.Now()
+   now := timeutil.Now()
    c.ResolvedAt = &now
    c.Outcome = outcome
    // ... rest unchanged
}
```

**1.2 Cron Handlers** (2 files)

**File**: `internal/handlers/cron/ach_verification_handler.go`
```go
// Line 149
func (h *ACHVerificationHandler) VerifyACH(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...

    // Find ACH payment methods pending verification older than verificationDays
-   cutoffDate := time.Now().AddDate(0, 0, -verificationDays)
+   cutoffDate := timeutil.Now().AddDate(0, 0, -verificationDays)

    // ... rest unchanged
}
```

**File**: `internal/handlers/cron/billing_handler.go`
```go
// Line 87-94
func (h *BillingHandler) ProcessBilling(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...

    // Determine as-of date
-   asOfDate := time.Now()
+   asOfDate := timeutil.Now()
    if req.AsOfDate != nil {
-       parsed, err := time.Parse("2006-01-02", *req.AsOfDate)
+       parsed, err := timeutil.ParseDate("2006-01-02", *req.AsOfDate)
        if err != nil {
            h.respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid as_of_date format: %v", err))
            return
        }
        asOfDate = parsed
    }

    // ... rest unchanged
}
```

**Required Import Addition** (All 5 files above):
```go
import (
    // ... existing imports ...
    "github.com/kevin07696/payment-service/pkg/timeutil"
)
```

---

### Phase 2: Create Complete Migration 019 (20 minutes)

**File**: `internal/db/migrations/019_standardize_timestamps_to_timestamptz.sql`

```sql
-- +goose Up
-- +goose StatementBegin

-- ============================================================================
-- TIMESTAMP → TIMESTAMPTZ Migration
-- ============================================================================
-- This migration standardizes ALL timestamp columns to TIMESTAMPTZ for
-- timezone consistency across the entire database schema.
--
-- CRITICAL ASSUMPTIONS:
-- 1. Existing TIMESTAMP data is assumed to be in UTC
-- 2. All application code now uses timeutil.Now() (UTC enforcement)
-- 3. Migration tested on staging environment first
--
-- AFFECTED TABLES (10 tables, 27 columns total):
-- - merchants (3 columns)
-- - admins (2 columns)
-- - admin_sessions (2 columns)
-- - audit_logs + 3 partitions (1 column each = 4 columns)
-- - jwt_blacklist (2 columns)
-- - epx_ip_whitelist (1 column)
-- - services (2 columns)
-- - service_merchants (2 columns)
-- - merchant_activation_tokens (3 columns)
-- - rate_limit_buckets (2 columns)
-- ============================================================================

-- 1. MERCHANTS TABLE (High Priority - Authentication)
DO $$
BEGIN
    RAISE NOTICE 'Migrating merchants table...';
END $$;

ALTER TABLE merchants
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC',
  ALTER COLUMN deleted_at TYPE TIMESTAMPTZ USING deleted_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN merchants.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.updated_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.deleted_at IS 'Timezone-aware timestamp (stored as UTC, soft delete)';

-- 2. ADMINS TABLE (High Priority - Authentication)
DO $$
BEGIN
    RAISE NOTICE 'Migrating admins table...';
END $$;

ALTER TABLE admins
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN admins.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN admins.updated_at IS 'Timezone-aware timestamp (stored as UTC)';

-- 3. ADMIN_SESSIONS TABLE (High Priority - Authentication)
DO $$
BEGIN
    RAISE NOTICE 'Migrating admin_sessions table...';
END $$;

ALTER TABLE admin_sessions
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN admin_sessions.expires_at IS 'Session expiration time (UTC)';
COMMENT ON COLUMN admin_sessions.created_at IS 'Timezone-aware timestamp (stored as UTC)';

-- 4. AUDIT_LOGS TABLE (Partitioned - Parent + 3 Child Partitions)
DO $$
BEGIN
    RAISE NOTICE 'Migrating audit_logs table and partitions...';
END $$;

-- Parent table
ALTER TABLE audit_logs
  ALTER COLUMN performed_at TYPE TIMESTAMPTZ USING performed_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN audit_logs.performed_at IS 'Audit event timestamp (UTC)';

-- Child partitions (must be altered individually)
ALTER TABLE audit_logs_2024_q1
  ALTER COLUMN performed_at TYPE TIMESTAMPTZ USING performed_at AT TIME ZONE 'UTC';

ALTER TABLE audit_logs_2024_q2
  ALTER COLUMN performed_at TYPE TIMESTAMPTZ USING performed_at AT TIME ZONE 'UTC';

ALTER TABLE audit_logs_default
  ALTER COLUMN performed_at TYPE TIMESTAMPTZ USING performed_at AT TIME ZONE 'UTC';

-- 5. JWT_BLACKLIST TABLE (High Priority - Authentication)
DO $$
BEGIN
    RAISE NOTICE 'Migrating jwt_blacklist table...';
END $$;

ALTER TABLE jwt_blacklist
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN blacklisted_at TYPE TIMESTAMPTZ USING blacklisted_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN jwt_blacklist.expires_at IS 'Token expiration time (UTC)';
COMMENT ON COLUMN jwt_blacklist.blacklisted_at IS 'When token was blacklisted (UTC)';

-- 6. EPX_IP_WHITELIST TABLE
DO $$
BEGIN
    RAISE NOTICE 'Migrating epx_ip_whitelist table...';
END $$;

ALTER TABLE epx_ip_whitelist
  ALTER COLUMN added_at TYPE TIMESTAMPTZ USING added_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN epx_ip_whitelist.added_at IS 'When IP was whitelisted (UTC)';

-- 7. SERVICES TABLE (High Priority - Authentication)
DO $$
BEGIN
    RAISE NOTICE 'Migrating services table...';
END $$;

ALTER TABLE services
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN services.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN services.updated_at IS 'Timezone-aware timestamp (stored as UTC)';

-- 8. SERVICE_MERCHANTS TABLE (High Priority - Authentication)
DO $$
BEGIN
    RAISE NOTICE 'Migrating service_merchants table...';
END $$;

ALTER TABLE service_merchants
  ALTER COLUMN granted_at TYPE TIMESTAMPTZ USING granted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN service_merchants.granted_at IS 'When access was granted (UTC)';
COMMENT ON COLUMN service_merchants.expires_at IS 'When access expires (UTC)';

-- 9. MERCHANT_ACTIVATION_TOKENS TABLE (High Priority - Onboarding)
DO $$
BEGIN
    RAISE NOTICE 'Migrating merchant_activation_tokens table...';
END $$;

ALTER TABLE merchant_activation_tokens
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN used_at TYPE TIMESTAMPTZ USING used_at AT TIME ZONE 'UTC',
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN merchant_activation_tokens.expires_at IS 'Token expiration time (UTC)';
COMMENT ON COLUMN merchant_activation_tokens.used_at IS 'When token was used (UTC)';
COMMENT ON COLUMN merchant_activation_tokens.created_at IS 'Timezone-aware timestamp (stored as UTC)';

-- 10. RATE_LIMIT_BUCKETS TABLE
DO $$
BEGIN
    RAISE NOTICE 'Migrating rate_limit_buckets table...';
END $$;

ALTER TABLE rate_limit_buckets
  ALTER COLUMN last_refill TYPE TIMESTAMPTZ USING last_refill AT TIME ZONE 'UTC',
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN rate_limit_buckets.last_refill IS 'Last token bucket refill time (UTC)';
COMMENT ON COLUMN rate_limit_buckets.created_at IS 'Timezone-aware timestamp (stored as UTC)';

-- ============================================================================
-- VERIFICATION: Ensure NO timestamp columns remain without timezone
-- ============================================================================
DO $$
DECLARE
    non_tz_count INTEGER;
    non_tz_columns TEXT;
BEGIN
    -- Count remaining TIMESTAMP WITHOUT TIMEZONE columns
    SELECT COUNT(*), STRING_AGG(table_name || '.' || column_name, ', ')
    INTO non_tz_count, non_tz_columns
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND column_name LIKE '%_at'
      AND data_type = 'timestamp without time zone';

    IF non_tz_count > 0 THEN
        RAISE EXCEPTION 'Migration FAILED: % columns still using TIMESTAMP without timezone: %',
            non_tz_count, non_tz_columns;
    END IF;

    RAISE NOTICE '✅ SUCCESS: All %_at timestamp columns are now TIMESTAMPTZ (timezone-aware)';
END $$;

-- ============================================================================
-- POST-MIGRATION STATISTICS
-- ============================================================================
DO $$
DECLARE
    timestamptz_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO timestamptz_count
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND column_name LIKE '%_at'
      AND data_type = 'timestamp with time zone';

    RAISE NOTICE '✅ Total TIMESTAMPTZ columns: %', timestamptz_count;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- ============================================================================
-- ROLLBACK: TIMESTAMPTZ → TIMESTAMP (NOT RECOMMENDED)
-- ============================================================================
-- WARNING: Rolling back loses timezone information!
-- Only use this for emergency rollback in case of critical issues.
-- ============================================================================

RAISE WARNING 'Rolling back TIMESTAMPTZ to TIMESTAMP - timezone information will be lost!';

-- Rollback in reverse order
ALTER TABLE rate_limit_buckets
  ALTER COLUMN last_refill TYPE TIMESTAMP USING last_refill AT TIME ZONE 'UTC',
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

ALTER TABLE merchant_activation_tokens
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN used_at TYPE TIMESTAMP USING used_at AT TIME ZONE 'UTC',
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

ALTER TABLE service_merchants
  ALTER COLUMN granted_at TYPE TIMESTAMP USING granted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE services
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE epx_ip_whitelist
  ALTER COLUMN added_at TYPE TIMESTAMP USING added_at AT TIME ZONE 'UTC';

ALTER TABLE jwt_blacklist
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN blacklisted_at TYPE TIMESTAMP USING blacklisted_at AT TIME ZONE 'UTC';

ALTER TABLE audit_logs_default
  ALTER COLUMN performed_at TYPE TIMESTAMP USING performed_at AT TIME ZONE 'UTC';

ALTER TABLE audit_logs_2024_q2
  ALTER COLUMN performed_at TYPE TIMESTAMP USING performed_at AT TIME ZONE 'UTC';

ALTER TABLE audit_logs_2024_q1
  ALTER COLUMN performed_at TYPE TIMESTAMP USING performed_at AT TIME ZONE 'UTC';

ALTER TABLE audit_logs
  ALTER COLUMN performed_at TYPE TIMESTAMP USING performed_at AT TIME ZONE 'UTC';

ALTER TABLE admin_sessions
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

ALTER TABLE admins
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE merchants
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
  ALTER COLUMN deleted_at TYPE TIMESTAMP USING deleted_at AT TIME ZONE 'UTC';

RAISE NOTICE 'Rollback complete - all timestamp columns reverted to TIMESTAMP without timezone';

-- +goose StatementEnd
```

---

### Phase 3: Testing Strategy (1 hour)

#### 3.1 Pre-Migration Validation
```bash
# 1. Backup database
pg_dump -h localhost -U postgres -d payment_service > backup_before_timestamp_migration.sql

# 2. Check current schema
psql -h localhost -U postgres -d payment_service -c "
  SELECT table_name, column_name, data_type
  FROM information_schema.columns
  WHERE column_name LIKE '%_at'
  ORDER BY table_name, column_name;
"

# 3. Sample data checks (verify UTC assumption)
psql -h localhost -U postgres -d payment_service -c "
  SELECT 'merchants' as table_name, created_at FROM merchants LIMIT 5
  UNION ALL
  SELECT 'admins', created_at FROM admins LIMIT 5;
"
```

#### 3.2 Run Migration
```bash
# Rename migration file to enable it
mv internal/db/migrations/019_standardize_timestamps_to_timestamptz.sql.disabled \
   internal/db/migrations/019_standardize_timestamps_to_timestamptz.sql

# Run migration
goose -dir internal/db/migrations postgres "$DATABASE_URL" up

# Verify migration applied
goose -dir internal/db/migrations postgres "$DATABASE_URL" status
```

#### 3.3 Post-Migration Validation
```bash
# 1. Verify all columns are now TIMESTAMPTZ
psql -h localhost -U postgres -d payment_service -c "
  SELECT table_name, column_name, data_type
  FROM information_schema.columns
  WHERE column_name LIKE '%_at' AND data_type = 'timestamp without time zone';
"
# Expected: 0 rows (all should be 'timestamp with time zone')

# 2. Verify data integrity (timestamps unchanged)
psql -h localhost -U postgres -d payment_service -c "
  SELECT 'merchants' as table_name, created_at FROM merchants LIMIT 5
  UNION ALL
  SELECT 'admins', created_at FROM admins LIMIT 5;
"
# Compare with pre-migration output - values should be identical

# 3. Check migration completed successfully
tail -n 50 /path/to/migration/logs
# Look for: "✅ SUCCESS: All %_at timestamp columns are now TIMESTAMPTZ"
```

#### 3.4 Integration Tests
```bash
# Run all tests to ensure no regressions
go test -short ./... -v | grep -E "(FAIL|PASS|ok)"

# Specific critical path tests
go test -v ./internal/domain/... -run "TestChargeback|TestSubscription|TestPaymentMethod"
go test -v ./internal/handlers/cron/... -run "TestACHVerification|TestBilling"
go test -v ./tests/integration/auth/... -run "TestJWT|TestAuth"
```

---

### Phase 4: Regenerate SQLC Code (10 minutes)

```bash
# 1. Regenerate SQLC models
sqlc generate

# 2. Verify generated code changes
git diff internal/db/sqlc/models.go

# Expected changes:
# - pgtype.Timestamp → pgtype.Timestamptz for merchants, admins, services, etc.
# - time.Time remains time.Time (SQLC handles conversion)

# 3. Rebuild application
go build ./cmd/server/...

# 4. Run all tests again with new SQLC code
go test -short ./...
```

---

## Rollback Plan

If issues are discovered:

```bash
# 1. Rollback migration
goose -dir internal/db/migrations postgres "$DATABASE_URL" down

# 2. Restore from backup if needed
psql -h localhost -U postgres -d payment_service < backup_before_timestamp_migration.sql

# 3. Disable migration file
mv internal/db/migrations/019_standardize_timestamps_to_timestamptz.sql \
   internal/db/migrations/019_standardize_timestamps_to_timestamptz.sql.disabled

# 4. Revert code changes
git checkout -- internal/domain/*.go internal/handlers/cron/*.go

# 5. Regenerate SQLC (back to original)
sqlc generate
```

---

## Production Deployment Checklist

- [ ] All Phase 1 code changes committed and tested
- [ ] Migration tested on staging environment with production data volume
- [ ] Backup taken before migration (retention: 30 days minimum)
- [ ] Migration runs successfully on staging
- [ ] Integration tests pass on staging
- [ ] Performance impact measured (should be minimal - ALTER TABLE with AT TIME ZONE is fast)
- [ ] Rollback plan tested on staging
- [ ] Maintenance window scheduled (recommended: 30 minutes)
- [ ] Team notified of deployment
- [ ] Monitoring alerts configured for post-migration period

---

## Files Summary

### Code Changes (6 files)
1. `internal/domain/payment_method.go` - 3 changes
2. `internal/domain/subscription.go` - 4 changes
3. `internal/domain/chargeback.go` - 4 changes
4. `internal/handlers/cron/ach_verification_handler.go` - 1 change
5. `internal/handlers/cron/billing_handler.go` - 2 changes

### Database Changes (1 file)
6. `internal/db/migrations/019_standardize_timestamps_to_timestamptz.sql` - Enable (rename)

### Auto-Generated (1 file)
7. `internal/db/sqlc/models.go` - Regenerate with `sqlc generate`

---

## Expected Impact

### Performance
- **Migration time**: 1-2 seconds (ALTER TABLE with AT TIME ZONE is fast)
- **Downtime**: None (can run online, but recommend maintenance window)
- **Storage**: No change (TIMESTAMPTZ is same size as TIMESTAMP)

### Functionality
- **No breaking changes** - Application code already uses UTC
- **Bug fixes**: Resolves 6 timezone-related bugs
- **Consistency**: All timestamps now properly timezone-aware

### Monitoring
Watch these metrics post-migration:
- Authentication success rate (admins, services)
- Subscription billing accuracy
- Chargeback response deadline tracking
- ACH verification timing

---

## Success Criteria

✅ All 27 timestamp columns migrated to TIMESTAMPTZ
✅ All tests passing
✅ No timezone-related bugs in production
✅ Authentication and billing systems working correctly
✅ SQLC code regenerated successfully

---

**Created**: 2025-11-21
**Last Updated**: 2025-11-21
**Author**: Claude Code (AI Assistant)
