# Testing Best Practices

**Target Audience**: Developers
**Topic**: Unit and Integration Testing Guidelines
**Goal**: Establish consistent, maintainable testing patterns across the codebase

## Table of Contents
1. [Error Assertion Guidelines](#error-assertion-guidelines)
2. [What to Assert](#what-to-assert)
3. [Good vs Bad Test Patterns](#good-vs-bad-test-patterns)
4. [When to Assert Error Messages](#when-to-assert-error-messages)
5. [Refactoring Brittle Tests](#refactoring-brittle-tests)

---

## Error Assertion Guidelines

### The Golden Rule

**Test behavior, not implementation details.**

Error messages are implementation details unless they're part of a documented API contract that external consumers rely on.

### Categories of Errors

#### 1. Internal Implementation Errors (DON'T assert messages)

**Examples:**
- Database adapter connection errors
- HTTP client errors
- Cache lookup failures
- Internal package errors

**Why:** These errors get wrapped/transformed before reaching users. Asserting on their messages makes tests brittle and discourages improving error clarity.

**Bad Example:**
```go
// ❌ BRITTLE: Breaks when we improve error clarity
func TestDatabaseConnection_InvalidURL(t *testing.T) {
    adapter, err := database.NewAdapter("invalid-url")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to parse database URL")
    // ^ This breaks if we change message to "failed to parse configuration"
}
```

**Good Example:**
```go
// ✅ ROBUST: Tests behavior, not message wording
func TestDatabaseConnection_InvalidURL(t *testing.T) {
    adapter, err := database.NewAdapter("invalid-url")

    require.Error(t, err, "Should return error for invalid URL")
    require.Nil(t, adapter, "Adapter should be nil on error")

    // That's it! The important behavior is verified.
    // We don't care about exact message wording.
}
```

#### 2. User-Facing API Errors (DO assert messages carefully)

**Examples:**
- ConnectRPC error messages (returned to API clients)
- HTTP API response messages
- Validation error messages shown to users
- Error codes in public APIs

**Why:** External consumers may parse or display these messages. Changing them is a breaking change.

**Good Example:**
```go
// ✅ API contract - message is documented
func TestPayment_InsufficientFunds(t *testing.T) {
    response, err := client.CreatePayment(ctx, req)

    require.NoError(t, err) // RPC call succeeded
    require.False(t, response.Msg.IsApproved)

    // This message is shown to users - assert stability
    assert.Equal(t, "Insufficient funds", response.Msg.Message)
}
```

**Better Example with Error Types:**
```go
// ✅✅ BEST: Use error types/codes instead of strings
func TestPayment_InsufficientFunds(t *testing.T) {
    response, err := client.CreatePayment(ctx, req)

    require.NoError(t, err)
    require.False(t, response.Msg.IsApproved)

    // Assert on stable error code, not message
    assert.Equal(t, ErrorCodeInsufficientFunds, response.Msg.ErrorCode)

    // Message can evolve for clarity without breaking tests
}
```

#### 3. Validation Errors (Use error types preferred)

**Examples:**
- "merchant_id is required"
- "invalid email format"
- "amount must be positive"

**Current State:** Many service layer tests assert on these messages

**Recommendation:** Introduce structured error types

**Before (current):**
```go
// ⚠️ ACCEPTABLE but not ideal
func TestListTransactions_MissingMerchantID(t *testing.T) {
    transactions, err := service.ListTransactions(ctx, &filters)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "merchant_id is required")
}
```

**After (recommended):**
```go
// ✅ BETTER: Use error types
func TestListTransactions_MissingMerchantID(t *testing.T) {
    transactions, err := service.ListTransactions(ctx, &filters)

    assert.Error(t, err)
    assert.ErrorIs(t, err, domain.ErrMerchantIDRequired)

    // Or with structured errors:
    var validationErr *domain.ValidationError
    if assert.ErrorAs(t, err, &validationErr) {
        assert.Equal(t, "merchant_id", validationErr.Field)
    }
}
```

---

## What to Assert

### DO Assert:
✅ Error occurred (vs nil)
✅ Error types (`ErrorIs`, `ErrorAs`)
✅ Error codes (for public APIs)
✅ Behavior (nil return values, state changes)
✅ Side effects (database calls, logging)

### DON'T Assert:
❌ Exact error message text (for internal errors)
❌ Error message contains substring (brittle)
❌ Log message format
❌ Debug output

### MAYBE Assert (with caution):
⚠️ User-facing error messages (if part of API contract)
⚠️ Validation messages (if documented)
⚠️ Localized strings (for i18n consistency)

---

## Good vs Bad Test Patterns

### Pattern 1: Database/Adapter Errors

**❌ Bad (Brittle):**
```go
adapter, err := postgres.NewAdapter(invalidURL)
assert.Contains(t, err.Error(), "connection failed")
```

**✅ Good (Robust):**
```go
adapter, err := postgres.NewAdapter(invalidURL)
require.Error(t, err)
require.Nil(t, adapter)
```

**✅✅ Best (With Error Types):**
```go
adapter, err := postgres.NewAdapter(invalidURL)
require.Error(t, err)
require.Nil(t, adapter)
assert.ErrorIs(t, err, postgres.ErrInvalidConfig)
```

### Pattern 2: Context Cancellation

**❌ Bad (Brittle):**
```go
err := service.DoWork(ctx)
assert.Contains(t, err.Error(), "context canceled")
```

**✅ Good (Stable):**
```go
err := service.DoWork(ctx)
assert.ErrorIs(t, err, context.Canceled)
```

### Pattern 3: Validation Errors

**⚠️ Acceptable (Current Pattern):**
```go
err := service.CreateUser(req)
assert.Contains(t, err.Error(), "email is required")
```

**✅ Better (Error Types):**
```go
err := service.CreateUser(req)
var validationErr *domain.ValidationError
require.ErrorAs(t, err, &validationErr)
assert.Equal(t, "email", validationErr.Field)
assert.Equal(t, domain.ValidationRequired, validationErr.Type)
```

### Pattern 4: HTTP Client Errors

**❌ Bad (Implementation Detail):**
```go
response, err := httpClient.Get(url)
assert.Contains(t, err.Error(), "connection refused")
```

**✅ Good (Behavior):**
```go
response, err := httpClient.Get(url)
assert.Error(t, err)
assert.Nil(t, response)

// If we need more specificity:
var netErr *net.OpError
assert.ErrorAs(t, err, &netErr)
```

---

## When to Assert Error Messages

### ✅ Assert Messages When:

1. **Public API Contract**
   ```go
   // ConnectRPC response messages shown to users
   assert.Equal(t, "Payment declined", response.Msg.Message)
   ```

2. **Documented Error Codes**
   ```go
   // Error code is part of API specification
   assert.Equal(t, "ERR_INVALID_CARD", errorResponse.Code)
   ```

3. **Internationalization (i18n)**
   ```go
   // Ensuring translation keys are consistent
   assert.Equal(t, "error.payment.insufficient_funds", err.I18nKey())
   ```

4. **Regression Prevention (Rare)**
   ```go
   // When a specific message caused a production incident
   // and you want to prevent that exact message from reappearing
   ```

### ❌ Don't Assert Messages When:

1. **Internal Package Errors**
   - Database connection failures
   - Cache misses
   - File I/O errors

2. **Wrapped Errors**
   - Errors that get transformed by middleware
   - Errors that pass through multiple layers

3. **Debug/Development Messages**
   - Logging output
   - Stack traces
   - Debug strings

---

## Refactoring Brittle Tests

### Step 1: Identify Brittle Tests

Search for patterns:
```bash
grep -r "assert.Contains.*err.Error()" --include="*_test.go"
grep -r "assert.Equal.*err.Error()" --include="*_test.go"
```

### Step 2: Categorize

For each test, ask:
- Is this error user-facing?
- Is the message part of a documented API?
- Will external code parse this message?

### Step 3: Refactor Based on Category

**For Internal Errors:**
```go
// Before
assert.Contains(t, err.Error(), "failed to connect")

// After
require.Error(t, err)
// That's it!
```

**For Validation Errors (Interim):**
```go
// Keep for now, but add TODO
assert.Contains(t, err.Error(), "email is required")
// TODO: Replace with error types (see TESTING_BEST_PRACTICES.md)
```

**For API Errors:**
```go
// Keep, but document why
assert.Equal(t, "Payment declined", response.Message)
// This message is part of the public API contract
// and may be displayed to end users or parsed by clients
```

---

## Real-World Example: Database Adapter Test

### Before (Brittle):
```go
func TestNewPostgreSQLAdapter_InvalidURL(t *testing.T) {
    tests := []struct {
        name        string
        databaseURL string
        expectError string // ❌ Brittle
    }{
        {
            name:        "invalid URL format",
            databaseURL: "not-a-valid-url",
            expectError: "failed to parse database URL", // ❌ Breaks on rewording
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)
            assert.Error(t, err)
            assert.Contains(t, err.Error(), tt.expectError) // ❌ Brittle
        })
    }
}
```

**Why This is Bad:**
- Breaks when we improve error message clarity
- Discourages making errors more helpful
- Tests implementation details, not behavior
- Message text isn't a user-facing contract

### After (Robust):
```go
func TestNewPostgreSQLAdapter_InvalidURL(t *testing.T) {
    tests := []struct {
        name        string
        databaseURL string
        // No expectError field - we test behavior, not messages
    }{
        {
            name:        "empty_URL",
            databaseURL: "",
        },
        {
            name:        "invalid_URL_format",
            databaseURL: "not-a-valid-url",
        },
        {
            name:        "invalid_scheme",
            databaseURL: "mysql://localhost/db",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            adapter, err := NewPostgreSQLAdapter(ctx, cfg, logger)

            // Assert behavior, not implementation details
            require.Error(t, err, "Should return error for invalid URL")
            require.Nil(t, adapter, "Adapter should be nil on error")

            // ✅ Test passes regardless of error message wording
            // ✅ Encourages improving error messages
            // ✅ Tests the important behavior
        })
    }
}
```

**Why This is Better:**
- Won't break when we improve error messages
- Tests actual behavior (error occurred, adapter is nil)
- More maintainable
- Focuses on what matters

---

## Current Codebase Status

### ✅ Refactored (Following Best Practices)
- `internal/adapters/database/postgres_test.go` - Database adapter tests

### ⚠️ Needs Improvement (Future Refactoring)
- `internal/adapters/epx/server_post_error_test.go` - EPX adapter validation tests
- `internal/adapters/epx/server_post_adapter_test.go` - EPX adapter tests
- `internal/services/payment/payment_service_test.go` - Service validation (introduce error types)
- `internal/services/subscription/subscription_service_test.go` - Service validation (introduce error types)
- `internal/services/merchant/merchant_service_test.go` - Service validation (introduce error types)

### 📋 Recommended Improvements

1. **Short-term**: Add TODO comments to existing brittle tests
2. **Medium-term**: Introduce error types for validation errors
3. **Long-term**: Refactor all internal adapter tests to not assert messages

---

## Introducing Error Types (Future Work)

### Current Pattern:
```go
return fmt.Errorf("merchant_id is required")
```

### Recommended Pattern:
```go
// Define error types in domain package
var (
    ErrMerchantIDRequired = errors.New("merchant_id is required")
    ErrInvalidFormat     = errors.New("invalid format")
)

// Or structured errors:
type ValidationError struct {
    Field   string
    Type    ValidationType
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Usage in service:
if filters.MerchantID == nil {
    return nil, 0, domain.ErrMerchantIDRequired
}

// Test:
assert.ErrorIs(t, err, domain.ErrMerchantIDRequired)
```

---

## Summary

1. **Test behavior, not implementation details**
2. **Use error types over string matching**
3. **Only assert messages for public API contracts**
4. **Refactor brittle tests as you encounter them**
5. **Document why you're asserting on messages (if you must)**

**Bottom Line:** A good test should pass when you make the error message MORE helpful, not break.

---

**Related Documentation:**
- [Error Handling Guide](./ERROR_HANDLING.md) (TODO: Create this)
- [API Error Codes](./API_SPECS.md#error-codes)
- [Contributing Guidelines](../CONTRIBUTING.md)

**Last Updated:** 2025-11-22
