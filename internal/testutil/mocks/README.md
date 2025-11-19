# Test Mocks Package

This package provides shared mock implementations for testing, eliminating ~300 lines of duplicated mock code across test files.

## Available Mocks

### MockQuerier

Mocks the `sqlc.Querier` interface for database operations.

**Usage:**

```go
import "github.com/kevin07696/payment-service/internal/testutil/mocks"

func TestMyHandler(t *testing.T) {
    mockDB := &mocks.MockQuerier{}

    // Set up expectations
    mockDB.On("GetMerchantByID", mock.Anything, merchantID).
        Return(merchant, nil)

    // Use mockDB in your test...

    // Assert expectations were met
    mockDB.AssertExpectations(t)
}
```

**Best Practices:**

1. **Only mock what you need**: The mock provides implementations for all methods, but you only need to set up expectations for methods your test actually calls.

2. **Use mock.Anything for context**: Unless you need to assert specific context values, use `mock.Anything` for context parameters.

3. **Return realistic data**: Use the fixtures package to create realistic test data instead of empty structs.

4. **Assert expectations**: Always call `mockDB.AssertExpectations(t)` at the end of your test to verify all expected calls were made.

## Benefits

- **Single source of truth**: All mock implementations in one place
- **Easier maintenance**: Update mocks in one location
- **Consistency**: Same mock behavior across all tests
- **Reduced duplication**: Eliminates ~300 lines of duplicated code
- **Better testability**: Encourages proper dependency injection

## Future Additions

As we identify more duplicated mocks in the codebase, we'll add them here:

- Server Post Adapter mock
- Secret Manager Adapter mock
- Other adapter mocks as needed
