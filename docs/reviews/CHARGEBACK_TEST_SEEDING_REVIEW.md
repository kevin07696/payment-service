# Chargeback Test Seeding - Go Best Practices Review

**Reviewer:** Claude Code
**Date:** 2025-11-24
**Files Reviewed:**
- `/home/kevinlam/Documents/projects/payments/tests/integration/testutil/setup.go` (Lines 148-247)
- `/home/kevinlam/Documents/projects/payments/tests/integration/chargeback/chargeback_test.go`

## Executive Summary

The chargeback test seeding implementation is **generally well-structured** but has several opportunities for improvement in Go idioms, performance, and maintainability. This review provides specific, actionable recommendations aligned with Go best practices and your codebase patterns.

**Overall Grade:** B+ (Good, with room for improvement)

---

## 1. Go Idioms & Style

### 1.1 Connection String Building - Anti-Pattern Detected

**Current Issue (Lines 59, 98):**
```go
// ANTI-PATTERN: Manual string concatenation
connStr := "host=" + dbHost + " port=" + dbPort + " user=" + dbUser +
    " password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"
```

**Problems:**
1. Inefficient: Creates multiple intermediate string allocations
2. Error-prone: Easy to miss spaces or ordering
3. Not idiomatic: Go has better tools for this

**Recommendation: Use fmt.Sprintf or strings.Builder**

```go
// GOOD: Clear, single allocation, compile-time checked
connStr := fmt.Sprintf(
    "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
    dbHost, dbPort, dbUser, dbPassword, dbName,
)

// ALTERNATIVE: For very large strings, use strings.Builder
var b strings.Builder
b.WriteString("host=")
b.WriteString(dbHost)
b.WriteString(" port=")
b.WriteString(dbPort)
// ... etc
connStr := b.String()
```

**Impact:** Low effort, immediate readability improvement

---

### 1.2 Error Handling Pattern - require.NoError vs Error Return

**Current Pattern:**
```go
func SeedChargebacks(t *testing.T) {
    t.Helper()

    db := GetDB(t)
    defer db.Close()

    // ... operations that call require.NoError
}
```

**Analysis:** ✅ **This is CORRECT for test helpers**

The use of `require.NoError` is appropriate because:
1. **Test helpers should fail fast** - If DB seeding fails, tests can't run
2. **Clear failure location** - `t.Helper()` ensures test failures point to calling code
3. **Matches your codebase pattern** - Consistent with other test utilities

**Alternative Pattern (NOT recommended here):**
```go
// DON'T do this for critical test setup
func SeedChargebacks(t *testing.T) error {
    // Returning errors from test setup creates confusion
    // about where the test actually failed
}
```

**Recommendation:** ✅ Keep current pattern, but see Section 4.2 for optional enhancements

---

### 1.3 Variable Naming - Good Overall, Minor Improvement

**Current:**
```go
testMerchantUUID := "00000000-0000-0000-0000-000000000001"
testMerchantSlug := "test-merchant-staging"
```

✅ **Good:** Clear, descriptive names following Go conventions

**Minor Enhancement:**
```go
const (
    // Test fixtures - centralize magic values
    testMerchantID   = "00000000-0000-0000-0000-000000000001"
    testMerchantSlug = "test-merchant-staging"
    testCustomerID   = "cust_test_001"
)
```

**Benefits:**
- Reusable across test functions
- Single source of truth
- Compile-time constants vs runtime strings

---

### 1.4 Defer Statement Usage - Excellent

```go
db, err := sql.Open("pgx", connStr)
require.NoError(t, err, "Failed to connect to database")
defer db.Close() // ✅ CORRECT: Immediately after resource acquisition
```

✅ **This is textbook Go** - defer immediately after acquiring resources

---

### 1.5 Comment Quality - Good, Could Be Better

**Current:**
```go
// SeedChargebacks seeds test chargeback data for integration tests
// This function is idempotent - it can be called multiple times safely
func SeedChargebacks(t *testing.T) {
```

✅ **Good:** Explains idempotency

**Enhancement for godoc:**
```go
// SeedChargebacks creates test chargeback data with various statuses for integration tests.
//
// This function is idempotent and safe to call multiple times due to ON CONFLICT DO NOTHING.
// It creates:
//   - 1 test transaction (or reuses existing)
//   - 5 chargebacks with statuses: new, pending, responded, won, lost
//
// The function calls t.Helper() and will fail the test immediately on any database error.
func SeedChargebacks(t *testing.T) {
```

**Benefits:**
- Better godoc rendering (bullet points, line breaks)
- Explicit about what gets created
- Documents behavior clearly

---

## 2. Database Patterns

### 2.1 Connection Pooling - CRITICAL ISSUE

**Current Problem:**
```go
func GetDB(t *testing.T) *sql.DB {
    t.Helper()

    // ❌ PROBLEM: Creates NEW connection pool on every call
    db, err := sql.Open("pgx", connStr)
    require.NoError(t, err)

    err = db.Ping()
    require.NoError(t, err)

    return db // ❌ Caller must close - easy to leak
}

func SeedChargebacks(t *testing.T) {
    db := GetDB(t)      // New pool
    defer db.Close()    // Closes pool
    // ...
}

func seedTestMerchant(t *testing.T, cfg *Config) {
    db, err := sql.Open("pgx", connStr)  // Another new pool!
    require.NoError(t, err)
    defer db.Close()
    // ...
}
```

**Why This Is Bad:**
1. **Connection pool overhead** - Each `sql.Open()` creates pool management structures
2. **Resource waste** - Multiple pools when one would suffice
3. **Slower tests** - Connection establishment time on every helper call
4. **Inconsistent with production** - Your main.go uses pgxpool (lines 64-68)

**Production Pattern (main.go):**
```go
// main.go uses connection pool properly
dbPool, err := initDatabase(cfg, logger)
if err != nil {
    logger.Fatal("Failed to initialize database", zap.Error(err))
}
defer dbPool.Close()  // ✅ Single pool, closed at program end
```

**RECOMMENDED SOLUTION: Singleton Pattern for Tests**

Create a package-level connection pool that's reused across all tests:

```go
// testutil/db.go
package testutil

import (
    "context"
    "database/sql"
    "fmt"
    "os"
    "sync"
    "testing"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jackc/pgx/v5/stdlib"
    "github.com/stretchr/testify/require"
)

var (
    testDBPool     *pgxpool.Pool
    testDB         *sql.DB
    testDBInitOnce sync.Once
    testDBInitErr  error
)

// GetTestDB returns a shared database connection pool for all integration tests.
// The pool is created once and reused across all tests for performance.
// The connection is NOT closed - it lives for the test suite duration.
func GetTestDB(t *testing.T) *sql.DB {
    t.Helper()

    testDBInitOnce.Do(func() {
        ctx := context.Background()
        connStr := buildTestDatabaseURL()

        // Use pgxpool like production
        testDBPool, testDBInitErr = pgxpool.New(ctx, connStr)
        if testDBInitErr != nil {
            return
        }

        // Ping to verify connection
        testDBInitErr = testDBPool.Ping(ctx)
        if testDBInitErr != nil {
            testDBPool.Close()
            return
        }

        // Create sql.DB wrapper for standard database/sql interface
        testDB = stdlib.OpenDBFromPool(testDBPool)
    })

    require.NoError(t, testDBInitErr, "Failed to initialize test database pool")
    return testDB
}

// CloseTestDB closes the shared test database pool.
// Call this in TestMain if you want explicit cleanup (optional).
func CloseTestDB() {
    if testDBPool != nil {
        testDBPool.Close()
    }
}

func buildTestDatabaseURL() string {
    host := getEnvOrDefault("DB_HOST", "localhost")
    port := getEnvOrDefault("DB_PORT", "5432")
    user := getEnvOrDefault("DB_USER", "postgres")
    password := getEnvOrDefault("DB_PASSWORD", "postgres")
    dbname := getEnvOrDefault("DB_NAME", "payment_service")

    return fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        host, port, user, password, dbname,
    )
}

func getEnvOrDefault(key, defaultVal string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultVal
}
```

**Usage in SeedChargebacks:**
```go
func SeedChargebacks(t *testing.T) {
    t.Helper()

    db := GetTestDB(t)  // ✅ Reuses shared pool
    // NO defer db.Close() - pool lives for test suite duration

    // ... rest of function
}
```

**Benefits:**
1. ✅ Matches production pattern (pgxpool)
2. ✅ Single connection pool across all tests
3. ✅ Faster test execution (no reconnection overhead)
4. ✅ Thread-safe (sync.Once pattern)
5. ✅ Consistent with Go best practices

**Migration Path:**
1. Create new `testutil/db.go` with GetTestDB
2. Update `SeedChargebacks`, `seedTestMerchant` to use GetTestDB
3. Remove `defer db.Close()` calls from helper functions
4. Optional: Add TestMain for explicit cleanup

---

### 2.2 Transaction Usage - Missing Opportunity

**Current Pattern:**
```go
func SeedChargebacks(t *testing.T) {
    db := GetDB(t)

    // ❌ Multiple operations without transaction
    var testTxnID string
    err := db.QueryRow(`SELECT ...`).Scan(&testTxnID)

    if err == sql.ErrNoRows {
        err = db.QueryRow(`INSERT ...`).Scan(&testTxnID)
        // ...
    }

    result, err := db.Exec(`INSERT INTO chargebacks ...`)
    // ...
}
```

**Problem:** If second INSERT fails, you have partial state

**RECOMMENDATION: Use Transaction for Atomicity**

```go
func SeedChargebacks(t *testing.T) {
    t.Helper()

    db := GetTestDB(t)

    // Begin transaction for atomic seeding
    tx, err := db.Begin()
    require.NoError(t, err, "Failed to begin transaction")

    // Defer rollback - will be no-op if commit succeeds
    defer tx.Rollback()

    // Step 1: Get or create transaction
    var testTxnID string
    err = tx.QueryRow(`SELECT id FROM transactions...`).Scan(&testTxnID)

    if err == sql.ErrNoRows {
        err = tx.QueryRow(`INSERT INTO transactions...`).Scan(&testTxnID)
        require.NoError(t, err, "Failed to create test transaction")
    } else {
        require.NoError(t, err, "Failed to query test transaction")
    }

    // Step 2: Seed chargebacks
    result, err := tx.Exec(`INSERT INTO chargebacks...`)
    require.NoError(t, err, "Failed to seed chargebacks")

    // Commit transaction
    err = tx.Commit()
    require.NoError(t, err, "Failed to commit seeding transaction")

    rowsAffected, _ := result.RowsAffected()
    if rowsAffected > 0 {
        t.Logf("✅ Seeded %d test chargebacks", rowsAffected)
    }
}
```

**Benefits:**
1. ✅ Atomic seeding - all or nothing
2. ✅ Cleaner test database state
3. ✅ No partial failures
4. ✅ Standard Go transaction pattern

**When to use transactions in tests:**
- ✅ **DO** for multi-step setup that should be atomic
- ❌ **DON'T** if you're using ON CONFLICT and idempotency is sufficient
- ✅ **DO** if failure modes need to be tested separately

**Decision:** Given your use of `ON CONFLICT DO NOTHING`, transactions are **nice-to-have but not critical** here. However, they demonstrate good database hygiene.

---

### 2.3 Context Usage - Missing Critical Timeout

**Current:**
```go
err := db.QueryRow(`SELECT ...`).Scan(&testTxnID)
```

**Problem:** No timeout - could hang indefinitely

**RECOMMENDATION: Use context.WithTimeout**

```go
func SeedChargebacks(t *testing.T) {
    t.Helper()

    db := GetTestDB(t)

    // Create context with timeout for database operations
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    var testTxnID string
    err := db.QueryRowContext(ctx, `
        SELECT id FROM transactions
        WHERE merchant_id = $1::uuid
        AND status = 'approved'
        LIMIT 1
    `, testMerchantID).Scan(&testTxnID)

    if err == sql.ErrNoRows {
        err = db.QueryRowContext(ctx, `INSERT ...`).Scan(&testTxnID)
        require.NoError(t, err, "Failed to create test transaction")
    }

    _, err = db.ExecContext(ctx, `INSERT INTO chargebacks ...`)
    require.NoError(t, err, "Failed to seed chargebacks")
}
```

**Benefits:**
1. ✅ Prevents hung tests
2. ✅ Fast failure on database issues
3. ✅ Matches production patterns (your main.go uses timeouts)
4. ✅ Required for context-aware operations

**Impact:** Medium effort, high reliability improvement

---

### 2.4 Prepared Statements - Not Needed Here

**Current:**
```go
result, err := db.Exec(`INSERT INTO chargebacks ...`, testTxnID, testMerchantSlug)
```

**Question:** Should we use prepared statements?

**Answer:** ❌ **No, not for test setup code**

**Rationale:**
1. ✅ Test setup runs once - no benefit from caching
2. ✅ Simpler code without prepare/close lifecycle
3. ✅ Your codebase uses sqlc for production queries (good!)
4. ❌ Prepared statements add complexity for zero gain in tests

**When to use prepared statements:**
- ✅ Repeated queries in loops
- ✅ Production hot paths
- ❌ One-time test setup

---

## 3. Testing Patterns

### 3.1 t.Helper() Usage - PERFECT ✅

```go
func SeedChargebacks(t *testing.T) {
    t.Helper()  // ✅ EXCELLENT: Marks as helper for better error reporting
    // ...
}

func GetDB(t *testing.T) *sql.DB {
    t.Helper()  // ✅ EXCELLENT
    // ...
}
```

**Why this is excellent:**
- When tests fail, stack traces skip helpers and point to actual test
- Standard Go testing best practice
- Consistent across your codebase

**Recommendation:** ✅ Keep this pattern everywhere

---

### 3.2 Test Organization - Good Structure

**Current:**
```go
// chargeback_test.go
func setupChargebackTest(t *testing.T) (*testutil.Config, chargebackv1connect.ChargebackServiceClient) {
    t.Helper()
    cfg, _ := testutil.Setup(t)  // ✅ Calls SeedChargebacks internally
    client := chargebackv1connect.NewChargebackServiceClient(...)
    return cfg, client
}

func TestChargeback_ListChargebacks(t *testing.T) {
    _, client := setupChargebackTest(t)
    // ...
}
```

✅ **Good pattern:** Test-specific setup function that calls shared setup

**Enhancement Suggestion:**
```go
// Consider splitting concerns
func setupChargebackTest(t *testing.T) (*testutil.Config, chargebackv1connect.ChargebackServiceClient) {
    t.Helper()

    // Option 1: Explicit seeding (more control)
    cfg, baseClient := testutil.Setup(t)
    testutil.SeedChargebacks(t)  // ✅ Explicit, clear dependency

    // Option 2: Keep implicit (current - simpler)
    cfg, _ := testutil.Setup(t)  // Seeds everything

    client := chargebackv1connect.NewChargebackServiceClient(...)
    return cfg, client
}
```

**Current pattern is fine**, but explicit seeding can be clearer for new developers.

---

### 3.3 Setup/Teardown Patterns - Missing TestMain

**Current Issue:** No explicit cleanup for shared resources

**RECOMMENDATION: Add TestMain for Lifecycle Management**

```go
// tests/integration/chargeback/chargeback_test.go
package chargeback_test

import (
    "os"
    "testing"

    "github.com/kevin07696/payment-service/tests/integration/testutil"
)

func TestMain(m *testing.M) {
    // Run all tests
    exitCode := m.Run()

    // Cleanup shared resources
    testutil.CloseTestDB()

    os.Exit(exitCode)
}
```

**Benefits:**
1. ✅ Explicit resource cleanup
2. ✅ Clear test lifecycle
3. ✅ Prevents connection leaks in CI/CD
4. ✅ Standard Go testing pattern

**Note:** Most test runners will clean up on process exit, so this is **nice-to-have** but not critical.

---

### 3.4 Test Fixtures vs Factories - Current Hybrid Approach is Good

**Current Pattern:**
```go
// Fixtures: Hardcoded test data
const testMerchantID = "00000000-0000-0000-0000-000000000001"
const testMerchantSlug = "test-merchant-staging"

// Factory-like: Dynamic test data
err = db.QueryRow(`
    INSERT INTO transactions (
        id, merchant_id, customer_id,
        amount_cents, currency, type,
        tran_nbr,  -- Dynamic: 'TXN-' || substr(gen_random_uuid()::text, 1, 10)
        auth_guid, -- Dynamic: 'BRIC-' || gen_random_uuid()::text
        ...
    )
`).Scan(&testTxnID)
```

✅ **Good hybrid approach:**
- Fixtures for stable IDs (merchant, customer)
- Factories for variable data (transaction IDs, GUIDs)

**Alternative Pattern (if tests need more control):**

```go
// testutil/fixtures.go
type ChargebackFixture struct {
    TransactionID string
    MerchantSlug  string
    CustomerID    string
    CaseNumber    string
    Amount        string
    Status        string
}

func DefaultChargebackFixture() ChargebackFixture {
    return ChargebackFixture{
        TransactionID: uuid.New().String(),
        MerchantSlug:  testMerchantSlug,
        CustomerID:    "cust_test_001",
        CaseNumber:    "CB-TEST-" + uuid.New().String()[:8],
        Amount:        "100.00",
        Status:        "new",
    }
}

func SeedChargeback(t *testing.T, fixture ChargebackFixture) string {
    db := GetTestDB(t)
    var chargebackID string
    err := db.QueryRow(`
        INSERT INTO chargebacks (transaction_id, merchant_id, ...)
        VALUES ($1, $2, ...) RETURNING id
    `, fixture.TransactionID, fixture.MerchantSlug, ...).Scan(&chargebackID)
    require.NoError(t, err)
    return chargebackID
}
```

**Recommendation:** Current approach is fine for now. Add factory pattern if tests need custom fixtures.

---

## 4. Performance

### 4.1 Batch Operations - Already Optimal ✅

```go
result, err := db.Exec(`
    INSERT INTO chargebacks (...) VALUES
    -- NEW chargeback
    (...),
    -- PENDING chargeback
    (...),
    -- RESPONDED chargeback
    (...),
    -- WON chargeback
    (...),
    -- LOST chargeback
    (...)
    ON CONFLICT (case_number) DO NOTHING
`)
```

✅ **EXCELLENT:** Single INSERT for 5 rows is optimal

**Alternative (SLOWER, don't use):**
```go
// ❌ BAD: 5 round-trips to database
for _, chargeback := range chargebacks {
    db.Exec(`INSERT INTO chargebacks ...`)
}
```

**Your approach is textbook Go database performance** - well done!

---

### 4.2 Memory Allocations - Minimal Concern

**Current:**
```go
testMerchantUUID := "00000000-0000-0000-0000-000000000001"
testMerchantSlug := "test-merchant-staging"
```

**Benchmark:**
```
String allocation: ~16 bytes per string
Impact: Negligible (test setup runs once)
```

**Recommendation:** ✅ No changes needed. Test setup isn't a hot path.

**If you really wanted to optimize (NOT recommended):**
```go
const (
    testMerchantUUID = "00000000-0000-0000-0000-000000000001"
    testMerchantSlug = "test-merchant-staging"
)
// Saves ~32 bytes per function call - not worth the effort
```

---

### 4.3 String Building - See Section 1.1

Already covered - use `fmt.Sprintf` for connection strings.

---

## 5. Interface Design & API Ergonomics

### 5.1 Current API - Simple But Inflexible

```go
func SeedChargebacks(t *testing.T) {
    // ❌ No customization options
    // ❌ Always creates same test data
    // ❌ Can't control what gets seeded
}
```

**When this is a problem:**
- Different tests need different chargeback statuses
- Need specific amounts or dates
- Want to test edge cases (e.g., only 1 chargeback)

---

### 5.2 RECOMMENDED: Functional Options Pattern

```go
// testutil/chargeback_seeding.go
package testutil

import (
    "database/sql"
    "testing"
)

// ChargebackSeedOptions controls what test data gets created
type ChargebackSeedOptions struct {
    merchantID     string
    merchantSlug   string
    customerID     string
    statuses       []string      // Which statuses to create
    customFixtures []Chargeback  // Custom chargeback data
    skipTransaction bool          // Don't create transaction if one exists
}

// ChargebackSeedOption is a functional option for configuring seeding
type ChargebackSeedOption func(*ChargebackSeedOptions)

// WithMerchant sets the merchant for chargebacks
func WithMerchant(id, slug string) ChargebackSeedOption {
    return func(opts *ChargebackSeedOptions) {
        opts.merchantID = id
        opts.merchantSlug = slug
    }
}

// WithStatuses limits which chargeback statuses get created
func WithStatuses(statuses ...string) ChargebackSeedOption {
    return func(opts *ChargebackSeedOptions) {
        opts.statuses = statuses
    }
}

// WithCustomerID sets the customer ID for chargebacks
func WithCustomerID(customerID string) ChargebackSeedOption {
    return func(opts *ChargebackSeedOptions) {
        opts.customerID = customerID
    }
}

// SeedChargebacks creates test chargeback data with optional customization.
//
// By default, it creates:
//   - 1 test transaction (or reuses existing)
//   - 5 chargebacks with statuses: new, pending, responded, won, lost
//
// Example usage:
//   // Default: All statuses
//   SeedChargebacks(t)
//
//   // Only new and pending chargebacks
//   SeedChargebacks(t, WithStatuses("new", "pending"))
//
//   // Custom merchant
//   SeedChargebacks(t, WithMerchant(merchantID, merchantSlug))
func SeedChargebacks(t *testing.T, opts ...ChargebackSeedOption) {
    t.Helper()

    // Default options
    options := ChargebackSeedOptions{
        merchantID:   "00000000-0000-0000-0000-000000000001",
        merchantSlug: "test-merchant-staging",
        customerID:   "cust_test_001",
        statuses:     []string{"new", "pending", "responded", "won", "lost"},
    }

    // Apply functional options
    for _, opt := range opts {
        opt(&options)
    }

    db := GetTestDB(t)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Get or create transaction
    testTxnID := getOrCreateTestTransaction(t, db, ctx, options.merchantID)

    // Seed chargebacks based on statuses
    for _, status := range options.statuses {
        seedChargebackWithStatus(t, db, ctx, testTxnID, options.merchantSlug,
                                  options.customerID, status)
    }
}

func getOrCreateTestTransaction(t *testing.T, db *sql.DB, ctx context.Context,
                                 merchantID string) string {
    t.Helper()

    var testTxnID string
    err := db.QueryRowContext(ctx, `
        SELECT id FROM transactions
        WHERE merchant_id = $1::uuid
        AND status = 'approved'
        LIMIT 1
    `, merchantID).Scan(&testTxnID)

    if err == sql.ErrNoRows {
        err = db.QueryRowContext(ctx, `
            INSERT INTO transactions (
                id, merchant_id, customer_id,
                amount_cents, currency, type, payment_method_type,
                tran_nbr, auth_guid, auth_resp, auth_code,
                processed_at, created_at, updated_at
            ) VALUES (
                gen_random_uuid(), $1::uuid, gen_random_uuid(),
                10000, 'USD', 'SALE', 'credit_card',
                'TXN-' || substr(gen_random_uuid()::text, 1, 10),
                'BRIC-' || gen_random_uuid()::text,
                '00', 'AUTH123',
                NOW(), NOW(), NOW()
            ) RETURNING id
        `, merchantID).Scan(&testTxnID)
        require.NoError(t, err, "Failed to create test transaction")
        t.Logf("✅ Created test transaction: %s", testTxnID)
    } else {
        require.NoError(t, err, "Failed to query test transaction")
        t.Logf("✅ Using existing test transaction: %s", testTxnID)
    }

    return testTxnID
}

func seedChargebackWithStatus(t *testing.T, db *sql.DB, ctx context.Context,
                               txnID, merchantSlug, customerID, status string) {
    t.Helper()

    caseNumber := fmt.Sprintf("CB-%s-TEST", strings.ToUpper(status))

    chargebackData := getChargebackDataForStatus(status)

    _, err := db.ExecContext(ctx, `
        INSERT INTO chargebacks (
            transaction_id, merchant_id, customer_id,
            case_number, dispute_date, chargeback_date,
            chargeback_amount, currency, reason_code, reason_description,
            status, raw_data
        ) VALUES (
            $1::uuid, $2, $3,
            $4, NOW() - $5::interval, NOW() - $6::interval,
            $7, 'USD', $8, $9,
            $10, $11::jsonb
        )
        ON CONFLICT (case_number) DO NOTHING
    `, txnID, merchantSlug, customerID,
       caseNumber, chargebackData.DisputeDaysAgo, chargebackData.ChargebackDaysAgo,
       chargebackData.Amount, chargebackData.ReasonCode, chargebackData.ReasonDesc,
       status, chargebackData.RawData)

    require.NoError(t, err, "Failed to seed %s chargeback", status)
}

type chargebackData struct {
    DisputeDaysAgo    string
    ChargebackDaysAgo string
    Amount            string
    ReasonCode        string
    ReasonDesc        string
    RawData           string
}

func getChargebackDataForStatus(status string) chargebackData {
    statusData := map[string]chargebackData{
        "new": {
            DisputeDaysAgo:    "5 days",
            ChargebackDaysAgo: "3 days",
            Amount:            "50.00",
            ReasonCode:        "P22",
            ReasonDesc:        "Cardholder disputes quality of goods or services",
            RawData:           `{"status": "NEW", "source": "test_seed", "test": true}`,
        },
        "pending": {
            DisputeDaysAgo:    "10 days",
            ChargebackDaysAgo: "7 days",
            Amount:            "75.50",
            ReasonCode:        "F10",
            ReasonDesc:        "Fraudulent transaction - card absent environment",
            RawData:           `{"status": "PENDING", "source": "test_seed", "test": true}`,
        },
        "responded": {
            DisputeDaysAgo:    "20 days",
            ChargebackDaysAgo: "15 days",
            Amount:            "100.00",
            ReasonCode:        "C08",
            ReasonDesc:        "Goods/Services not received",
            RawData:           `{"status": "RESPONDED", "source": "test_seed", "test": true}`,
        },
        "won": {
            DisputeDaysAgo:    "30 days",
            ChargebackDaysAgo: "25 days",
            Amount:            "125.00",
            ReasonCode:        "P08",
            ReasonDesc:        "Credit not processed",
            RawData:           `{"status": "WON", "source": "test_seed", "test": true}`,
        },
        "lost": {
            DisputeDaysAgo:    "35 days",
            ChargebackDaysAgo: "30 days",
            Amount:            "200.00",
            ReasonCode:        "F29",
            ReasonDesc:        "Card not present fraud",
            RawData:           `{"status": "LOST", "source": "test_seed", "test": true}`,
        },
    }

    return statusData[status]
}
```

**Usage Examples:**

```go
// Test 1: Default - all chargebacks
func TestListAllChargebacks(t *testing.T) {
    testutil.SeedChargebacks(t)  // ✅ Creates all 5 statuses
    // ...
}

// Test 2: Only new chargebacks
func TestNewChargebacksOnly(t *testing.T) {
    testutil.SeedChargebacks(t, testutil.WithStatuses("new"))  // ✅ Only creates "new"
    // ...
}

// Test 3: Multiple statuses
func TestPendingAndRespondedChargebacks(t *testing.T) {
    testutil.SeedChargebacks(t, testutil.WithStatuses("pending", "responded"))
    // ...
}

// Test 4: Custom merchant
func TestOtherMerchantChargebacks(t *testing.T) {
    testutil.SeedChargebacks(t,
        testutil.WithMerchant("other-merchant-id", "other-merchant-slug"),
        testutil.WithStatuses("new", "won"))
    // ...
}
```

**Benefits:**
1. ✅ **Backward compatible** - `SeedChargebacks(t)` still works with defaults
2. ✅ **Flexible** - Tests can customize what they need
3. ✅ **Readable** - Self-documenting API
4. ✅ **Idiomatic Go** - Functional options are standard pattern
5. ✅ **Testable** - Easy to add new options without breaking existing tests

---

### 5.3 Should SeedChargebacks Return Created IDs?

**Current:**
```go
func SeedChargebacks(t *testing.T) {
    // ❌ Doesn't return anything
}
```

**Pros of returning IDs:**
- Tests can assert on specific chargebacks
- Easier to test edge cases

**Cons of returning IDs:**
- Tests that don't need IDs have to ignore return value
- Forces tests to know about implementation details

**RECOMMENDATION: Add Optional Return via Options**

```go
type ChargebackSeedResult struct {
    TransactionID  string
    ChargebackIDs  map[string]string  // status -> chargeback_id
}

type ChargebackSeedOptions struct {
    // ... existing fields
    returnIDs bool
}

func WithReturnIDs() ChargebackSeedOption {
    return func(opts *ChargebackSeedOptions) {
        opts.returnIDs = true
    }
}

// Two versions for different use cases
func SeedChargebacks(t *testing.T, opts ...ChargebackSeedOption) {
    t.Helper()
    // Original version for simple cases
    _, _ = SeedChargebacksWithIDs(t, opts...)
}

func SeedChargebacksWithIDs(t *testing.T, opts ...ChargebackSeedOption) *ChargebackSeedResult {
    t.Helper()

    result := &ChargebackSeedResult{
        ChargebackIDs: make(map[string]string),
    }

    // ... seeding logic, populate result

    return result
}
```

**Usage:**
```go
// Simple case - don't care about IDs
testutil.SeedChargebacks(t)

// Complex case - need IDs for assertions
result := testutil.SeedChargebacksWithIDs(t, testutil.WithStatuses("new", "won"))
newChargebackID := result.ChargebackIDs["new"]
// ... assert on specific chargeback
```

---

## 6. Comparison with Existing Patterns

### 6.1 Payment Test Patterns

**Your payment tests follow good patterns:**

```go
// browser_post_workflow_test.go
func TestBrowserPost_Workflows(t *testing.T) {
    tests := []struct {
        name            string
        transactionType string
        amount          string
        workflow        []string
        refundAmount    string
    }{
        // ✅ Table-driven tests
        {name: "SALE_to_REFUND", ...},
        {name: "AUTH_CAPTURE_REFUND", ...},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg, client := testutil.Setup(t)  // ✅ Uses setup helper
            // ...
        })
    }
}
```

**Apply same pattern to chargeback seeding:**

```go
// Option: Add table-driven test for seeding itself
func TestSeedChargebacks_Variants(t *testing.T) {
    tests := []struct {
        name     string
        opts     []testutil.ChargebackSeedOption
        expected int  // expected count
    }{
        {
            name:     "default_all_statuses",
            opts:     nil,
            expected: 5,
        },
        {
            name:     "only_new",
            opts:     []testutil.ChargebackSeedOption{testutil.WithStatuses("new")},
            expected: 1,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            testutil.SeedChargebacks(t, tt.opts...)
            // Assert count in database
        })
    }
}
```

---

### 6.2 Authentication Pattern Consistency

**Your auth helpers are clean:**

```go
// auth_helpers.go
func GenerateJWT(privateKeyPEM, issuer, merchantID string, expiresIn time.Duration) (string, error) {
    // ✅ Returns error instead of calling t.Fatal
    // ✅ Pure function - no side effects
    // ✅ Reusable outside tests
}
```

**Apply to database helpers:**

```go
// Option: Pure functions for building SQL
func BuildChargebackInsertSQL(statuses []string) (string, []interface{}) {
    // ✅ Pure function
    // ✅ Testable without database
    // ✅ Reusable in different contexts
}

// Test helper wraps pure function
func SeedChargebacks(t *testing.T, opts ...ChargebackSeedOption) {
    t.Helper()
    sql, args := BuildChargebackInsertSQL(options.statuses)
    _, err := db.Exec(sql, args...)
    require.NoError(t, err)
}
```

---

## 7. Summary of Recommendations

### 7.1 CRITICAL (Implement Now)

| Priority | Item | Effort | Impact | Location |
|----------|------|--------|--------|----------|
| 🔴 **P0** | Fix connection pooling with singleton pattern | 2 hours | High | Section 2.1 |
| 🔴 **P0** | Add context timeouts to DB operations | 30 min | High | Section 2.3 |
| 🔴 **P0** | Use fmt.Sprintf for connection strings | 5 min | Low | Section 1.1 |

### 7.2 RECOMMENDED (Implement Soon)

| Priority | Item | Effort | Impact | Location |
|----------|------|--------|--------|----------|
| 🟡 **P1** | Add functional options for flexibility | 4 hours | Medium | Section 5.2 |
| 🟡 **P1** | Wrap seeding in transaction | 1 hour | Medium | Section 2.2 |
| 🟡 **P1** | Add TestMain for cleanup | 30 min | Low | Section 3.3 |
| 🟡 **P1** | Improve godoc comments | 30 min | Low | Section 1.5 |

### 7.3 OPTIONAL (Nice to Have)

| Priority | Item | Effort | Impact | Location |
|----------|------|--------|--------|----------|
| 🟢 **P2** | Extract constants for test IDs | 15 min | Low | Section 1.3 |
| 🟢 **P2** | Return chargeback IDs for assertions | 2 hours | Low | Section 5.3 |
| 🟢 **P2** | Add table-driven test for seeding | 1 hour | Low | Section 6.1 |

---

## 8. Implementation Checklist

### Phase 1: Critical Fixes (Week 1)

- [ ] Create `testutil/db.go` with singleton DB pool (Section 2.1)
- [ ] Update `GetDB()` to use shared pool
- [ ] Update `SeedChargebacks()` to use `GetTestDB()`
- [ ] Update `seedTestMerchant()` to use `GetTestDB()`
- [ ] Remove all `defer db.Close()` from helpers
- [ ] Add `context.WithTimeout()` to all DB operations (Section 2.3)
- [ ] Replace connection string concatenation with `fmt.Sprintf` (Section 1.1)
- [ ] Run full integration test suite to verify
- [ ] Run `go vet`, `golangci-lint`, `staticcheck` (per your QA process)

### Phase 2: Recommended Improvements (Week 2)

- [ ] Add transaction wrapper to `SeedChargebacks()` (Section 2.2)
- [ ] Create functional options types and functions (Section 5.2)
- [ ] Refactor `SeedChargebacks()` to accept options
- [ ] Add `TestMain()` to chargeback tests (Section 3.3)
- [ ] Update godoc comments (Section 1.5)
- [ ] Test backward compatibility (ensure existing tests still pass)

### Phase 3: Optional Enhancements (Future)

- [ ] Extract test ID constants (Section 1.3)
- [ ] Add `SeedChargebacksWithIDs()` variant (Section 5.3)
- [ ] Create factory helpers for custom fixtures (Section 3.4)
- [ ] Add table-driven test for seeding variations (Section 6.1)

---

## 9. Code Examples: Before & After

### 9.1 Connection Pooling (CRITICAL)

**Before:**
```go
// ❌ Creates new pool on every call
func SeedChargebacks(t *testing.T) {
    t.Helper()
    db := GetDB(t)  // sql.Open() every time
    defer db.Close()
    // ...
}
```

**After:**
```go
// ✅ Reuses shared pool
func SeedChargebacks(t *testing.T) {
    t.Helper()
    db := GetTestDB(t)  // Returns singleton pool
    // No defer - pool lives for test suite
    // ...
}
```

### 9.2 Context Timeouts (CRITICAL)

**Before:**
```go
// ❌ No timeout - could hang
err := db.QueryRow(`SELECT ...`).Scan(&testTxnID)
```

**After:**
```go
// ✅ 10-second timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
err := db.QueryRowContext(ctx, `SELECT ...`).Scan(&testTxnID)
```

### 9.3 String Building (CRITICAL)

**Before:**
```go
// ❌ Multiple allocations, error-prone
connStr := "host=" + dbHost + " port=" + dbPort + " user=" + dbUser +
    " password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"
```

**After:**
```go
// ✅ Single allocation, clear format
connStr := fmt.Sprintf(
    "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
    dbHost, dbPort, dbUser, dbPassword, dbName,
)
```

### 9.4 Functional Options (RECOMMENDED)

**Before:**
```go
// ❌ No customization
func TestNewChargebacks(t *testing.T) {
    testutil.SeedChargebacks(t)  // Always seeds all 5 statuses
    // ... test only needs "new" status
}
```

**After:**
```go
// ✅ Flexible, explicit
func TestNewChargebacks(t *testing.T) {
    testutil.SeedChargebacks(t, testutil.WithStatuses("new"))  // Only seeds what's needed
    // ... test uses exactly what it needs
}
```

---

## 10. References & Further Reading

### Go Best Practices
- [Effective Go](https://go.dev/doc/effective_go) - Official Go documentation
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) - Community standards
- [database/sql Tutorial](https://go.dev/doc/database/sql-injection) - Official SQL guide

### Patterns in Your Codebase
- `/home/kevinlam/Documents/projects/payments/cmd/server/main.go:64-68` - Production pgxpool usage
- `/home/kevinlam/Documents/projects/payments/tests/integration/testutil/auth_helpers.go` - Pure function pattern
- `/home/kevinlam/Documents/projects/payments/tests/integration/payment/browser_post_workflow_test.go` - Table-driven tests

### Connection Pooling
- [pgx Wiki: Connection Pool](https://github.com/jackc/pgx/wiki/Getting-started-with-pgx#connection-pool)
- [sync.Once Pattern](https://go.dev/tour/concurrency/9) - Thread-safe initialization

### Testing Patterns
- [Testing Best Practices](https://go.dev/doc/testing) - Official guide
- [testify/require](https://pkg.go.dev/github.com/stretchr/testify/require) - Assertion library docs
- [Functional Options Pattern](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis) - Dave Cheney

---

## Appendix A: Full Refactored Code

### A.1 testutil/db.go (New File)

```go
package testutil

import (
    "context"
    "database/sql"
    "fmt"
    "os"
    "sync"
    "testing"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jackc/pgx/v5/stdlib"
    "github.com/stretchr/testify/require"
)

var (
    testDBPool     *pgxpool.Pool
    testDB         *sql.DB
    testDBInitOnce sync.Once
    testDBInitErr  error
)

// GetTestDB returns a shared database connection pool for all integration tests.
// The pool is created once and reused across all tests for performance.
// The connection is NOT closed - it lives for the test suite duration.
//
// This matches the production pattern in cmd/server/main.go and ensures
// efficient resource usage during test execution.
func GetTestDB(t *testing.T) *sql.DB {
    t.Helper()

    testDBInitOnce.Do(func() {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        connStr := buildTestDatabaseURL()

        // Use pgxpool like production
        testDBPool, testDBInitErr = pgxpool.New(ctx, connStr)
        if testDBInitErr != nil {
            return
        }

        // Ping to verify connection
        testDBInitErr = testDBPool.Ping(ctx)
        if testDBInitErr != nil {
            testDBPool.Close()
            return
        }

        // Create sql.DB wrapper for standard database/sql interface
        testDB = stdlib.OpenDBFromPool(testDBPool)
    })

    require.NoError(t, testDBInitErr, "Failed to initialize test database pool")
    return testDB
}

// CloseTestDB closes the shared test database pool.
// Call this in TestMain if you want explicit cleanup (optional).
func CloseTestDB() {
    if testDBPool != nil {
        testDBPool.Close()
    }
}

func buildTestDatabaseURL() string {
    host := getEnvOrDefault("DB_HOST", "localhost")
    port := getEnvOrDefault("DB_PORT", "5432")
    user := getEnvOrDefault("DB_USER", "postgres")
    password := getEnvOrDefault("DB_PASSWORD", "postgres")
    dbname := getEnvOrDefault("DB_NAME", "payment_service")

    return fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        host, port, user, password, dbname,
    )
}

func getEnvOrDefault(key, defaultVal string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultVal
}
```

### A.2 testutil/setup.go (Refactored)

```go
package testutil

import (
    "context"
    "testing"
    "time"

    _ "github.com/jackc/pgx/v5/stdlib"
    "github.com/stretchr/testify/require"
)

const (
    testMerchantID   = "00000000-0000-0000-0000-000000000001"
    testMerchantSlug = "test-merchant-staging"
    testCustomerID   = "cust_test_001"
)

// Setup initializes test environment and returns config and client
func Setup(t *testing.T) (*Config, *Client) {
    t.Helper()

    // Load config from environment
    cfg, err := LoadConfig()
    require.NoError(t, err, "Failed to load test configuration")

    // Seed test merchant with EPX credentials from environment
    seedTestMerchant(t, cfg)

    // Seed test chargebacks for chargeback integration tests
    SeedChargebacks(t)

    // Create API client
    client := NewClient(cfg.ServiceURL)

    t.Logf("Integration test setup complete - service: %s", cfg.ServiceURL)

    return cfg, client
}

// seedTestMerchant ensures the test merchant exists with correct EPX credentials
func seedTestMerchant(t *testing.T, cfg *Config) {
    t.Helper()

    db := GetTestDB(t)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Insert or update test merchant with EPX credentials from environment
    _, err := db.ExecContext(ctx, `
        INSERT INTO merchants (
            id,
            slug,
            mac_secret_path,
            cust_nbr,
            merch_nbr,
            dba_nbr,
            terminal_nbr,
            environment,
            name,
            is_active,
            created_at,
            updated_at
        ) VALUES (
            $1::uuid,
            $2,
            'epx/staging/mac_secret',
            $3, $4, $5, $6,
            'staging',
            'Test Merchant (Staging)',
            true,
            NOW(),
            NOW()
        ) ON CONFLICT (id) DO UPDATE SET
            slug = $2,
            mac_secret_path = EXCLUDED.mac_secret_path,
            cust_nbr = EXCLUDED.cust_nbr,
            merch_nbr = EXCLUDED.merch_nbr,
            dba_nbr = EXCLUDED.dba_nbr,
            terminal_nbr = EXCLUDED.terminal_nbr,
            environment = 'staging',
            name = 'Test Merchant (Staging)',
            updated_at = NOW()
    `, testMerchantID, testMerchantSlug,
       cfg.EPXCustNbr, cfg.EPXMerchNbr, cfg.EPXDBANbr, cfg.EPXTerminalNbr)

    require.NoError(t, err, "Failed to seed test merchant")

    t.Logf("✅ Test merchant seeded with EPX credentials: CUST_NBR=%s, MERCH_NBR=%s, DBA_NBR=%s, TERMINAL_NBR=%s",
        cfg.EPXCustNbr, cfg.EPXMerchNbr, cfg.EPXDBANbr, cfg.EPXTerminalNbr)
}

// SeedChargebacks creates test chargeback data with various statuses for integration tests.
//
// This function is idempotent and safe to call multiple times due to ON CONFLICT DO NOTHING.
// It creates:
//   - 1 test transaction (or reuses existing)
//   - 5 chargebacks with statuses: new, pending, responded, won, lost
//
// The function calls t.Helper() and will fail the test immediately on any database error.
func SeedChargebacks(t *testing.T) {
    t.Helper()

    db := GetTestDB(t)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Begin transaction for atomic seeding
    tx, err := db.BeginTx(ctx, nil)
    require.NoError(t, err, "Failed to begin transaction")
    defer tx.Rollback() // Will be no-op if commit succeeds

    // Step 1: Get or create a test transaction
    var testTxnID string
    err = tx.QueryRowContext(ctx, `
        SELECT id FROM transactions
        WHERE merchant_id = $1::uuid
        AND status = 'approved'
        LIMIT 1
    `, testMerchantID).Scan(&testTxnID)

    if err == sql.ErrNoRows {
        // Create a test transaction
        err = tx.QueryRowContext(ctx, `
            INSERT INTO transactions (
                id, merchant_id, customer_id,
                amount_cents, currency, type, payment_method_type,
                tran_nbr, auth_guid, auth_resp, auth_code,
                processed_at, created_at, updated_at
            ) VALUES (
                gen_random_uuid(), $1::uuid, gen_random_uuid(),
                10000, 'USD', 'SALE', 'credit_card',
                'TXN-' || substr(gen_random_uuid()::text, 1, 10),
                'BRIC-' || gen_random_uuid()::text,
                '00', 'AUTH123',
                NOW(), NOW(), NOW()
            ) RETURNING id
        `, testMerchantID).Scan(&testTxnID)
        require.NoError(t, err, "Failed to create test transaction for chargebacks")
        t.Logf("✅ Created test transaction for chargebacks: %s", testTxnID)
    } else {
        require.NoError(t, err, "Failed to query test transaction")
        t.Logf("✅ Using existing test transaction for chargebacks: %s", testTxnID)
    }

    // Step 2: Seed test chargebacks with various statuses
    result, err := tx.ExecContext(ctx, `
        INSERT INTO chargebacks (
            transaction_id, merchant_id, customer_id,
            case_number, dispute_date, chargeback_date,
            chargeback_amount, currency, reason_code, reason_description,
            status, raw_data
        ) VALUES
        -- NEW chargeback
        (
            $1::uuid, $2, $3,
            'CB-NEW-TEST', NOW() - INTERVAL '5 days', NOW() - INTERVAL '3 days',
            '50.00', 'USD', 'P22', 'Cardholder disputes quality of goods or services',
            'new', '{"status": "NEW", "source": "test_seed", "test": true}'::jsonb
        ),
        -- PENDING chargeback
        (
            $1::uuid, $2, $3,
            'CB-PENDING-TEST', NOW() - INTERVAL '10 days', NOW() - INTERVAL '7 days',
            '75.50', 'USD', 'F10', 'Fraudulent transaction - card absent environment',
            'pending', '{"status": "PENDING", "source": "test_seed", "test": true}'::jsonb
        ),
        -- RESPONDED chargeback
        (
            $1::uuid, $2, $3,
            'CB-RESPONDED-TEST', NOW() - INTERVAL '20 days', NOW() - INTERVAL '15 days',
            '100.00', 'USD', 'C08', 'Goods/Services not received',
            'responded', '{"status": "RESPONDED", "source": "test_seed", "test": true}'::jsonb
        ),
        -- WON chargeback
        (
            $1::uuid, $2, $3,
            'CB-WON-TEST', NOW() - INTERVAL '30 days', NOW() - INTERVAL '25 days',
            '125.00', 'USD', 'P08', 'Credit not processed',
            'won', '{"status": "WON", "source": "test_seed", "test": true}'::jsonb
        ),
        -- LOST chargeback
        (
            $1::uuid, $2, $3,
            'CB-LOST-TEST', NOW() - INTERVAL '35 days', NOW() - INTERVAL '30 days',
            '200.00', 'USD', 'F29', 'Card not present fraud',
            'lost', '{"status": "LOST", "source": "test_seed", "test": true}'::jsonb
        )
        ON CONFLICT (case_number) DO NOTHING
    `, testTxnID, testMerchantSlug, testCustomerID)

    require.NoError(t, err, "Failed to seed test chargebacks")

    // Commit transaction
    err = tx.Commit()
    require.NoError(t, err, "Failed to commit seeding transaction")

    rowsAffected, _ := result.RowsAffected()
    if rowsAffected > 0 {
        t.Logf("✅ Seeded %d test chargebacks (statuses: new, pending, responded, won, lost)", rowsAffected)
    } else {
        t.Logf("✅ Test chargebacks already exist (skipped seeding)")
    }
}
```

### A.3 tests/integration/chargeback/chargeback_test.go (Add TestMain)

```go
//go:build integration
// +build integration

package chargeback_test

import (
    "os"
    "testing"

    "github.com/kevin07696/payment-service/tests/integration/testutil"
)

func TestMain(m *testing.M) {
    // Run all tests
    exitCode := m.Run()

    // Cleanup shared resources
    testutil.CloseTestDB()

    os.Exit(exitCode)
}

// ... rest of file unchanged
```

---

## Conclusion

Your chargeback test seeding implementation is **solid** with good fundamentals. The critical improvements (connection pooling, context timeouts, string building) will bring it to **production-grade quality** that aligns with Go best practices and your existing codebase patterns.

**Next Steps:**
1. Implement Phase 1 (Critical Fixes) this week
2. Review and validate with full test suite
3. Plan Phase 2 (Recommended Improvements) for next sprint
4. Track progress in `/home/kevinlam/Documents/projects/payments/CHANGELOG.md`

**Estimated Total Effort:** 8-12 hours for Phases 1-2

**Questions or need clarification on any recommendations?** Feel free to ask!
