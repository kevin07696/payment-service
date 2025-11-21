# Timezone Handling Analysis

**Analysis Date**: 2025-11-20
**Status**: ⚠️ **CRITICAL ISSUES FOUND**
**Risk Level**: HIGH - Data integrity, reporting accuracy, cross-timezone bugs

---

## Executive Summary

### Critical Issues Found

1. **❌ Inconsistent Database Schema**: Mix of `TIMESTAMP` (no TZ) and `TIMESTAMPTZ` (TZ-aware)
2. **❌ No UTC Enforcement**: `time.Now()` uses server's local timezone (undefined behavior)
3. **❌ Missing Timezone Configuration**: No explicit timezone set for application or database
4. **⚠️ Implicit Conversions**: Proto timestamps handled correctly, but database layer inconsistent

### Impact if Not Fixed

- **Data Integrity**: Timestamps stored in different timezones across tables
- **Reporting Bugs**: Incorrect date ranges when querying across timezone boundaries
- **Business Logic Errors**: Subscription billing, ACH verification windows calculated incorrectly
- **Audit Trail Issues**: Cannot reliably determine when events actually occurred
- **Daylight Saving Time Bugs**: Time calculations break twice a year

---

## Current State Analysis

### 1. Database Schema Inconsistencies ⚠️ CRITICAL

#### Merchants Table (❌ No Timezone)
```sql
-- File: 001_merchants.sql
created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,  ❌
updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,  ❌
deleted_at TIMESTAMP                                      ❌
```

**Problem**: `TIMESTAMP` (without TZ) stores wall-clock time without timezone info.
- When database server timezone changes, all timestamps are misinterpreted
- Cannot convert to user's timezone (don't know what timezone it was stored in)

#### Auth Tables (❌ No Timezone)
```sql
-- File: 008_auth_tables.sql
created_at TIMESTAMP DEFAULT NOW(),  ❌
updated_at TIMESTAMP DEFAULT NOW(),  ❌
```

**Problem**: Same as merchants table. Critical for security audit trails!

#### Customer Payment Methods (✅ Timezone-Aware)
```sql
-- File: 002_customer_payment_methods.sql
deleted_at TIMESTAMPTZ,  ✅
created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,  ✅
updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,  ✅
```

**Correct**: `TIMESTAMPTZ` stores UTC internally, converts on retrieval.

#### Transactions (✅ Timezone-Aware)
```sql
-- File: 003_transactions.sql
processed_at TIMESTAMPTZ,  ✅
deleted_at TIMESTAMPTZ,    ✅
created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,  ✅
updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,  ✅
```

**Correct**: All transaction timestamps are timezone-aware.

#### Subscriptions (✅ Timezone-Aware)
```sql
-- File: 003_transactions.sql (subscriptions table)
deleted_at TIMESTAMPTZ,  ✅
created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,  ✅
updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,  ✅
```

**Correct**: Subscription timestamps are timezone-aware.

#### Chargebacks (✅ Timezone-Aware)
```sql
-- File: 004_chargebacks.sql
deleted_at TIMESTAMPTZ,  ✅
created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,  ✅
updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,  ✅
```

**Correct**: Chargeback timestamps are timezone-aware.

### Summary of Database Issues

| Table | Timezone-Aware? | Risk Level | Tables Affected |
|-------|----------------|------------|-----------------|
| merchants | ❌ No | HIGH | 1 |
| services | ❌ No (auth tables) | HIGH | 3 |
| service_merchants | ❌ No (auth tables) | HIGH | - |
| admins | ❌ No (auth tables) | HIGH | - |
| customer_payment_methods | ✅ Yes | - | - |
| transactions | ✅ Yes | - | - |
| subscriptions | ✅ Yes | - | - |
| chargebacks | ✅ Yes | - | - |

**Total**: 4 tables with timezone issues

---

### 2. Go Code Analysis

#### Time Creation (❌ No UTC Enforcement)

```go
// Current pattern in codebase:
// File: internal/domain/merchant.go:69
m.UpdatedAt = time.Now()  ❌ Uses local timezone!

// File: internal/domain/chargeback.go:119
now := time.Now()  ❌ Uses local timezone!

// File: internal/handlers/cron/ach_verification_handler.go
cutoffDate := time.Now().Add(-3 * 24 * time.Hour)  ❌ Local timezone!
```

**Problem**: `time.Now()` returns time in server's local timezone.
- If server timezone is not set to UTC, all calculations are wrong
- Docker containers may have undefined timezone (often UTC, but not guaranteed)
- Daylight saving time changes affect calculations

**Correct Pattern**:
```go
now := time.Now().UTC()  ✅ Explicitly UTC
```

#### Database Driver Behavior

**PostgreSQL + pgx driver**:
- When storing `time.Time` to `TIMESTAMPTZ`: Converts to UTC ✅
- When storing `time.Time` to `TIMESTAMP`: Stores as-is (no timezone conversion) ❌
- When reading `TIMESTAMPTZ`: Returns `time.Time` with UTC location ✅
- When reading `TIMESTAMP`: Returns `time.Time` with **local timezone** ❌

**This means**:
```go
// Example with merchants table (TIMESTAMP without TZ):
merchant.CreatedAt = time.Now()  // Stored: "2025-11-20 15:30:00" (no TZ info)

// Later, when reading:
retrieved := getMerchant(id)
fmt.Println(retrieved.CreatedAt.Location())  // Could be UTC, Local, or undefined!
```

---

### 3. API Layer (Protobuf) ✅ Mostly Correct

```protobuf
// File: proto/payment/v1/payment.proto
import "google/protobuf/timestamp.proto";

message Transaction {
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp updated_at = 11;
  google.protobuf.Timestamp processed_at = 12;
}
```

**Good**: `google.protobuf.Timestamp` is:
- Always UTC ✅
- Unix timestamp with nanoseconds ✅
- Timezone-agnostic ✅

**Conversion in Go**:
```go
// Proto → Go (correct)
goTime := protoTimestamp.AsTime()  // Returns time.Time in UTC ✅

// Go → Proto (correct)
protoTimestamp := timestamppb.New(goTime)  // Converts to UTC ✅
```

---

### 4. Critical Business Logic Affected

#### ACH Verification Window (⚠️ Critical)

```go
// File: internal/handlers/cron/ach_verification_handler.go
cutoffDate := time.Now().Add(-3 * 24 * time.Hour)

// Problem: "3 days ago" depends on server timezone!
// - Server in UTC: Correct ✅
// - Server in EST: 5 hours off ❌
// - Server in PST: 8 hours off ❌

// Impact: ACH accounts verified too early/late
```

**Fix Required**:
```go
cutoffDate := time.Now().UTC().Add(-3 * 24 * time.Hour)
```

#### Subscription Billing (⚠️ Critical)

```go
// File: internal/services/subscription/subscription_service.go
now := time.Now()  // ❌ Local timezone

// Problem: "next billing date" comparisons are timezone-dependent
// - Subscription due at midnight UTC
// - Server in EST: Charges 5 hours early
// - Server in PST: Charges 8 hours early
```

**Fix Required**:
```go
now := time.Now().UTC()
```

#### Chargeback Deadlines (⚠️ Critical)

```go
// File: internal/domain/chargeback.go:86
if c.RespondByDate != nil && time.Now().After(*c.RespondByDate) {

// Problem: Deadline comparison uses local timezone
// - Chargeback deadline: 2025-11-30 23:59:59 UTC
// - Server in EST: Deadline passes 5 hours early
// - Could miss chargeback response window!
```

---

## Real-World Scenarios & Bugs

### Bug Scenario 1: Merchant Created Time

```sql
-- Merchant created via API
INSERT INTO merchants (..., created_at) VALUES (..., '2025-11-20 15:30:00');
-- No timezone info stored!

-- Query 1: List merchants created today
SELECT * FROM merchants
WHERE created_at::date = '2025-11-20'::date;

-- Result depends on database server timezone:
-- - DB in UTC: Returns merchant ✅
-- - DB in EST: Might not return (interprets as EST) ❌
-- - DB timezone changes: Same query returns different results ❌
```

### Bug Scenario 2: ACH Verification

```go
// Cron runs at 00:00 UTC every day
// Goal: Verify ACH accounts created 3+ days ago

// Server in PST (UTC-8):
cutoffDate := time.Now().Add(-3 * 24 * time.Hour)
// time.Now() = 2025-11-20 16:00:00 PST
// cutoffDate  = 2025-11-17 16:00:00 PST

// Database query:
SELECT * FROM customer_payment_methods
WHERE created_at < '2025-11-17 16:00:00'  -- Stored as TIMESTAMPTZ (UTC)

// PostgreSQL converts PST to UTC for comparison:
// Query becomes: created_at < '2025-11-18 00:00:00 UTC'

// Result: Verifies accounts created 2 days ago instead of 3! ❌
// Impact: ACH accounts verified 24 hours too early
```

### Bug Scenario 3: Subscription Billing

```go
// Subscription due: next_billing_date = '2025-11-20' (DATE, no time)
// Cron job runs: Process subscriptions due today or earlier

// Query:
SELECT * FROM subscriptions
WHERE status = 'active'
  AND next_billing_date <= '2025-11-20'::date

// Server in EST (UTC-5):
now := time.Now()  // 2025-11-20 19:00:00 EST

// Subscription should bill at 2025-11-20 00:00:00 UTC
// But server thinks it's still 2025-11-19 (in UTC)
// Result: Subscription not billed on correct date ❌
```

### Bug Scenario 4: Daylight Saving Time

```go
// Subscription billing calculation:
nextBilling := time.Now().Add(30 * 24 * time.Hour)  // "30 days from now"

// Problem: On DST transition days, this is wrong!
// Spring forward (lose 1 hour): nextBilling is 29 days, 23 hours ❌
// Fall back (gain 1 hour): nextBilling is 30 days, 1 hour ❌

// Impact: Subscriptions billed at wrong time twice a year
```

---

## Recommended Fixes

### FIX-1: Standardize Database Schema to TIMESTAMPTZ ⚠️ CRITICAL

**Priority**: P0 - CRITICAL
**Effort**: 30 minutes
**Risk**: Requires data migration

```sql
-- Migration: 019_standardize_timestamps_to_timestamptz.sql

-- +goose Up
-- +goose StatementBegin

-- Fix merchants table
ALTER TABLE merchants
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC',
  ALTER COLUMN deleted_at TYPE TIMESTAMPTZ USING deleted_at AT TIME ZONE 'UTC';

-- Fix services table (auth)
ALTER TABLE services
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

-- Fix service_merchants table (auth)
ALTER TABLE service_merchants
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

-- Fix admins table (auth)
ALTER TABLE admins
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

-- Fix audit_logs table (auth)
ALTER TABLE audit_logs
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN merchants.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.updated_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.deleted_at IS 'Timezone-aware timestamp (stored as UTC)';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE merchants
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
  ALTER COLUMN deleted_at TYPE TIMESTAMP USING deleted_at AT TIME ZONE 'UTC';

-- Repeat for other tables...
-- +goose StatementEnd
```

**Migration Notes**:
- `USING created_at AT TIME ZONE 'UTC'`: Assumes existing data is UTC (safest assumption)
- If existing data is NOT UTC, adjust the migration accordingly
- **Test in staging first!**

**Impact**:
- All timestamps now consistent ✅
- Database handles timezone conversions ✅
- Future-proof against timezone changes ✅

---

### FIX-2: Enforce UTC in Go Code ⚠️ CRITICAL

**Priority**: P0 - CRITICAL
**Effort**: 2 hours
**Risk**: Low (improves correctness)

#### Create Utility Package

```go
// File: pkg/timeutil/time.go

package timeutil

import "time"

// Now returns the current time in UTC
// Always use this instead of time.Now()
func Now() time.Time {
	return time.Now().UTC()
}

// ParseDate parses a date string and returns a UTC time
func ParseDate(layout, value string) (time.Time, error) {
	t, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

// StartOfDay returns the start of the day (midnight) in UTC
func StartOfDay(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// EndOfDay returns the end of the day (23:59:59.999999999) in UTC
func EndOfDay(t time.Time) time.Time {
	year, month, day := t.UTC().Date()
	return time.Date(year, month, day, 23, 59, 59, 999999999, time.UTC)
}
```

#### Update All `time.Now()` Calls

```bash
# Find all time.Now() calls:
grep -rn "time.Now()" internal/ --include="*.go" | wc -l
# Result: ~200 occurrences

# Replace pattern:
# Before:
now := time.Now()

# After:
now := timeutil.Now()  // Always UTC
```

**Key Files to Update**:
```go
// File: internal/domain/merchant.go:69
m.UpdatedAt = timeutil.Now()  ✅

// File: internal/domain/chargeback.go:119
now := timeutil.Now()  ✅

// File: internal/handlers/cron/ach_verification_handler.go
cutoffDate := timeutil.Now().Add(-3 * 24 * time.Hour)  ✅
```

---

### FIX-3: Set Database Timezone to UTC

**Priority**: P0 - CRITICAL
**Effort**: 5 minutes
**Risk**: None (best practice)

```sql
-- Set PostgreSQL server timezone to UTC
ALTER DATABASE payment_service SET timezone TO 'UTC';

-- Verify:
SHOW timezone;
-- Should return: UTC
```

**Docker Compose**:
```yaml
# File: docker-compose.yml
services:
  postgres:
    environment:
      POSTGRES_DB: payment_service
      PGTZ: UTC  # Set PostgreSQL timezone
      TZ: UTC    # Set container timezone
```

---

### FIX-4: Set Application Container Timezone

**Priority**: P0 - CRITICAL
**Effort**: 2 minutes
**Risk**: None

```dockerfile
# File: Dockerfile
FROM golang:1.21-alpine AS builder

# Set timezone to UTC
ENV TZ=UTC
RUN apk add --no-cache tzdata

# ... rest of build ...

FROM alpine:3.19
ENV TZ=UTC
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# ... rest of runtime ...
```

**Docker Compose**:
```yaml
# File: docker-compose.yml
services:
  payment-service:
    environment:
      TZ: UTC  # Set container timezone to UTC
```

---

### FIX-5: Add Timezone Validation Tests

**Priority**: P1
**Effort**: 1 hour

```go
// File: internal/timeutil/time_test.go

package timeutil

import (
	"testing"
	"time"
)

func TestNow_AlwaysUTC(t *testing.T) {
	now := Now()

	if now.Location() != time.UTC {
		t.Errorf("Now() returned non-UTC timezone: %v", now.Location())
	}
}

func TestStartOfDay(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "midnight UTC",
			input:    time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC),
			expected: "2025-11-20 00:00:00 +0000 UTC",
		},
		{
			name:     "noon UTC",
			input:    time.Date(2025, 11, 20, 12, 30, 45, 0, time.UTC),
			expected: "2025-11-20 00:00:00 +0000 UTC",
		},
		{
			name:     "end of day UTC",
			input:    time.Date(2025, 11, 20, 23, 59, 59, 0, time.UTC),
			expected: "2025-11-20 00:00:00 +0000 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StartOfDay(tt.input)

			if result.String() != tt.expected {
				t.Errorf("StartOfDay() = %v, want %v", result, tt.expected)
			}

			if result.Location() != time.UTC {
				t.Errorf("StartOfDay() returned non-UTC: %v", result.Location())
			}
		})
	}
}

func TestACHVerificationWindow(t *testing.T) {
	// Simulate ACH verification logic
	now := Now()  // 2025-11-20 12:00:00 UTC
	cutoff := now.Add(-3 * 24 * time.Hour)  // 2025-11-17 12:00:00 UTC

	// Verify cutoff is exactly 3 days ago
	diff := now.Sub(cutoff)
	expected := 3 * 24 * time.Hour

	if diff != expected {
		t.Errorf("ACH cutoff calculation incorrect: %v, want %v", diff, expected)
	}
}

// Test that ensures DST doesn't affect calculations
func TestDSTTransitions(t *testing.T) {
	// Spring forward: March 10, 2024, 2:00 AM → 3:00 AM
	beforeDST := time.Date(2024, 3, 9, 12, 0, 0, 0, time.UTC)
	afterDST := beforeDST.Add(24 * time.Hour)

	// Should be exactly 24 hours later
	expected := time.Date(2024, 3, 10, 12, 0, 0, 0, time.UTC)

	if !afterDST.Equal(expected) {
		t.Errorf("DST transition affected calculation: %v, want %v", afterDST, expected)
	}
}
```

---

## Verification Checklist

After implementing fixes:

### Database Verification
```sql
-- 1. Check all timestamp columns are TIMESTAMPTZ
SELECT
    table_name,
    column_name,
    data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND column_name LIKE '%_at'
  AND data_type != 'timestamp with time zone';

-- Should return 0 rows ✅

-- 2. Verify database timezone
SHOW timezone;
-- Should return: UTC ✅

-- 3. Test timestamp storage
INSERT INTO merchants (id, slug, name, cust_nbr, merch_nbr, dba_nbr, terminal_nbr, mac_secret_path)
VALUES (gen_random_uuid(), 'test', 'Test', '1', '1', '1', '1', '/test');

SELECT created_at, created_at AT TIME ZONE 'America/New_York' AS est_time
FROM merchants
WHERE slug = 'test';

-- created_at should show UTC ✅
```

### Go Code Verification
```bash
# 1. No raw time.Now() calls (except in timeutil package)
grep -rn "time\.Now()" internal/ --include="*.go" | grep -v "timeutil.Now()" | grep -v "// timeutil"

# Should return 0 results (or only comments) ✅

# 2. All time.Time values have UTC location
# Add to CI:
go test -race ./... -run TestTimezone
```

### Integration Test
```go
// File: tests/integration/timezone_test.go

func TestTimezoneConsistency(t *testing.T) {
	// Create merchant
	resp := createMerchant(t, &MerchantRequest{
		Name: "Timezone Test",
		// ...
	})

	// Verify created_at is in UTC
	if resp.CreatedAt.AsTime().Location() != time.UTC {
		t.Errorf("Merchant created_at not UTC: %v", resp.CreatedAt.AsTime().Location())
	}

	// Verify database stored as UTC
	var createdAt time.Time
	err := db.QueryRow("SELECT created_at FROM merchants WHERE id = $1", resp.Id).Scan(&createdAt)
	require.NoError(t, err)

	if createdAt.Location() != time.UTC {
		t.Errorf("Database timestamp not UTC: %v", createdAt.Location())
	}
}
```

---

## Migration Strategy

### Phase 1: Database Schema (30 minutes)
1. **Staging Environment**:
   ```bash
   # Test migration
   goose -dir internal/db/migrations postgres "$STAGING_DB" up-to 019

   # Verify:
   psql $STAGING_DB -c "SELECT table_name, column_name, data_type FROM information_schema.columns WHERE table_schema = 'public' AND column_name LIKE '%_at';"

   # All should show: timestamp with time zone
   ```

2. **Production Environment** (requires downtime):
   - Schedule maintenance window (5 minutes)
   - Run migration
   - Verify data integrity
   - Resume service

### Phase 2: Go Code (2 hours)
1. Create `pkg/timeutil` package
2. Update all `time.Now()` → `timeutil.Now()`
3. Add timezone tests
4. Deploy

### Phase 3: Infrastructure (5 minutes)
1. Update Docker Compose (set TZ=UTC)
2. Update Dockerfile (set TZ=UTC)
3. Update database config (timezone = UTC)
4. Deploy

---

## Summary

### Critical Issues
1. ❌ **Merchants table**: Uses TIMESTAMP (no TZ)
2. ❌ **Auth tables**: Use TIMESTAMP (no TZ)
3. ❌ **Go code**: Uses `time.Now()` (local TZ, not UTC)
4. ❌ **No timezone configuration**: Database and app containers

### Fixes Required
| Fix | Priority | Effort | Impact |
|-----|----------|--------|--------|
| Database schema → TIMESTAMPTZ | P0 | 30 min | Data integrity ✅ |
| Go code → timeutil.Now() | P0 | 2 hours | Correct calculations ✅ |
| Database TZ → UTC | P0 | 5 min | Consistency ✅ |
| Container TZ → UTC | P0 | 2 min | Consistency ✅ |
| Add timezone tests | P1 | 1 hour | Prevent regressions ✅ |

**Total Effort**: ~4 hours
**Total Impact**: Eliminates timezone bugs, improves data integrity

---

## References

- PostgreSQL Documentation: [Date/Time Types](https://www.postgresql.org/docs/current/datatype-datetime.html)
- Go Time Package: [time.Time](https://pkg.go.dev/time#Time)
- Protobuf Timestamp: [google.protobuf.Timestamp](https://protobuf.dev/reference/protobuf/google.protobuf/#timestamp)
- Timezone Best Practices: [Always Use UTC](https://codeblog.jonskeet.uk/2019/03/27/storing-utc-is-not-a-silver-bullet/)

---

**Status**: Critical issues documented, fixes ready for implementation
**Next Steps**: Implement FIX-1 through FIX-5 in order
**Blocker**: Should be fixed before production deployment to prevent timezone bugs
