# Test Fixtures Package

This package provides test data builders and helper functions, eliminating ~100 lines of duplicated test helper code across test files.

## Available Builders

### Pointer Helpers

Convenience functions for creating pointers to primitive types:

```go
import "github.com/kevin07696/payment-service/internal/testutil/fixtures"

// String pointers
name := fixtures.StringPtr("John Doe")

// Numeric pointers
count := fixtures.IntPtr(42)
amount := fixtures.Int64Ptr(10000)
rate := fixtures.Float64Ptr(0.05)

// Boolean pointers
isActive := fixtures.BoolPtr(true)

// Time and UUID pointers
now := fixtures.TimePtr(time.Now())
id := fixtures.UUIDPtr(uuid.New())
```

### MerchantBuilder

Build test merchants with fluent API:

```go
merchant := fixtures.NewMerchant().
    WithSlug("acme-corp").
    WithName("ACME Corporation").
    WithEnvironment("production").
    Active().
    Build()

// Convenience functions
activeMerchant := fixtures.ActiveMerchant(merchantID, "acme-corp")
inactiveMerchant := fixtures.InactiveMerchant(merchantID, "test-corp")
```

### SubscriptionBuilder

Build test subscriptions with realistic data:

```go
subscription := fixtures.NewSubscription().
    WithMerchantID(merchantID).
    WithCustomerID(customerID).
    WithPaymentMethodID(paymentMethodID).
    WithAmountCents(2999). // $29.99
    WithInterval(1, "month").
    Active().
    Build()

// Convenience functions
activeSub := fixtures.ActiveSubscription(merchantID, customerID, paymentMethodID)
cancelledSub := fixtures.CancelledSubscription(merchantID, customerID, paymentMethodID)
pastDueSub := fixtures.PastDueSubscription(merchantID, customerID, paymentMethodID, 2)
```

### TransactionBuilder

Build test transactions for various scenarios:

```go
// Sale transaction
sale := fixtures.NewTransaction().
    WithMerchantID(merchantID).
    WithAmountCents(10000). // $100.00
    Sale().
    CreditCard().
    Approved().
    Build()

// Auth transaction
auth := fixtures.NewTransaction().
    WithMerchantID(merchantID).
    WithAmountCents(5000).
    Auth().
    WithAuthGuid("bric_auth_123").
    Approved().
    Build()

// Convenience functions
approvedSale := fixtures.ApprovedSale(merchantID, 10000)
approvedAuth := fixtures.ApprovedAuth(merchantID, 5000, "bric_auth_123")
declinedSale := fixtures.DeclinedSale(merchantID, 10000)
capture := fixtures.CaptureTransaction(merchantID, parentAuthID, 5000, "bric_auth_123")
refund := fixtures.RefundTransaction(merchantID, parentSaleID, 5000)
```

### PaymentMethodBuilder

Build test payment methods for cards and ACH:

```go
// Credit card
visa := fixtures.NewPaymentMethod().
    WithMerchantID(merchantID).
    WithCustomerID(customerID).
    WithBric("bric_storage_123").
    CreditCard().
    WithCardBrand("visa").
    WithLastFour("4242").
    WithCardExpiration(12, 2025).
    Verified().
    Active().
    Default().
    Build()

// ACH account
checking := fixtures.NewPaymentMethod().
    WithMerchantID(merchantID).
    WithCustomerID(customerID).
    WithBric("bric_ach_456").
    ACH().
    WithBankName("Test Bank").
    WithAccountType("checking").
    WithLastFour("6789").
    Verified().
    Active().
    Build()

// Convenience functions
visa := fixtures.VisaCard(merchantID, customerID, "bric_storage_123")
defaultVisa := fixtures.DefaultVisaCard(merchantID, customerID, "bric_storage_123")
ach := fixtures.CheckingAccount(merchantID, customerID, "bric_ach_456")
```

### ServiceBuilder

Build test services for authentication testing:

```go
service := fixtures.NewService().
    WithServiceID("acme-app").
    WithServiceName("ACME Application").
    WithEnvironment("production").
    WithRateLimit(100, 200).
    Active().
    Build()

// Convenience functions
activeService := fixtures.ActiveService("acme-app")
inactiveService := fixtures.InactiveService("test-app")
```

## Usage Patterns

### Table-Driven Tests

```go
func TestPaymentProcessing(t *testing.T) {
    testCases := []struct {
        name        string
        transaction sqlc.Transaction
        wantError   bool
    }{
        {
            name:        "approved sale",
            transaction: fixtures.ApprovedSale(merchantID, 10000),
            wantError:   false,
        },
        {
            name:        "declined sale",
            transaction: fixtures.DeclinedSale(merchantID, 10000),
            wantError:   true,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Test logic using tc.transaction
        })
    }
}
```

### Builder Customization

```go
// Start with a base and customize
baseSubscription := fixtures.NewSubscription().
    WithMerchantID(merchantID).
    WithCustomerID(customerID)

// Create variations
activeSub := baseSubscription.Active().Build()
pausedSub := baseSubscription.Paused().Build()
cancelledSub := baseSubscription.Cancelled().Build()
```

### Complex Test Scenarios

```go
// Create a complete transaction flow
merchant := fixtures.ActiveMerchant(merchantID, "acme-corp")
customer := customerID
paymentMethod := fixtures.DefaultVisaCard(merchantID, customer, "bric_123")

// Auth → Capture → Refund chain
auth := fixtures.ApprovedAuth(merchantID, 10000, "bric_auth_123")
capture := fixtures.CaptureTransaction(merchantID, auth.ID, 10000, "bric_auth_123")
refund := fixtures.RefundTransaction(merchantID, capture.ID, 5000)
```

## Best Practices

1. **Use builders for complex objects**: Prefer fluent API for readability
2. **Use convenience functions for simple cases**: Quick one-liners for common scenarios
3. **Start with defaults**: Builders provide sensible defaults, override only what you need
4. **Chain methods**: Build expressive test setups with method chaining
5. **Extract common setups**: Create helper functions in your test files for repeated patterns

## Benefits

- **Eliminates duplication**: Shared builders across all test files
- **Improves readability**: Fluent API makes tests self-documenting
- **Reduces maintenance**: Update builders in one place
- **Consistent test data**: Realistic defaults based on actual data models
- **Type-safe**: Compile-time checks for valid field types
