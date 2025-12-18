package testutil

// TestConstants defines test data values used across integration tests
// NOTE: Merchant/customer IDs should NOT be hardcoded. Use factory.CreateTestContext(t) instead.
// NOTE: Card/ACH data is defined as TestCard/TestACH variables in tokenization.go

const (
	// Amount Constants (in cents to avoid magic numbers)
	DefaultAmountCents        = int64(5000)  // $50.00 - standard test amount
	SmallAmountCents          = int64(100)   // $1.00 - minimum amount tests
	LargeAmountCents          = int64(99999) // $999.99 - large amount tests
	PartialRefundCents        = int64(2500)  // $25.00 - partial refund tests
	DefaultSubscriptionAmount = int64(1999)  // $19.99 - subscription tests

	// Amount thresholds
	MaxAmountCents  = int64(99999999) // $999,999.99 - max allowed
	ZeroAmountCents = int64(0)        // Zero amount edge case
	OnecentAmount   = int64(1)        // Smallest possible amount

	// Currency
	DefaultCurrency = "USD"

	// Timeouts (in seconds)
	ServerReadyTimeout = 30  // seconds to wait for server to be ready
	TransactionTimeout = 60  // seconds for transaction to complete
	BrowserPostTimeout = 120 // seconds for browser automation

	// Database Timeouts (in seconds)
	DBQueryTimeout  = 10 // SELECT queries with simple WHERE clauses
	DBInsertTimeout = 15 // INSERT/UPDATE operations with complex data
	DBSelectTimeout = 5  // Simple SELECT queries for lookups
)
