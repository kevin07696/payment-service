package testutil

// TestConstants defines all test data identifiers used across integration tests
// This provides a single source of truth for test merchants, customers, and other test fixtures
//
// IMPORTANT: These constants must be kept in sync with:
// - scripts/seed_test_chargebacks.sh
// - Any manual test data seeding scripts
// - Database seed files
//
// SECURITY: Test data is marked with raw_data->>'test' = 'true' to enable safe cleanup
// without affecting production or staging data
const (
	// Test Merchant Identifiers
	// These identify the test merchant used across all integration tests
	// The merchant is automatically seeded during test setup with EPX credentials
	TestMerchantUUID = "00000000-0000-0000-0000-000000000001" // UUID format for database queries
	TestMerchantSlug = "test-merchant-staging"                // Human-readable slug for API requests
	TestMerchantName = "Test Merchant (Staging)"              // Display name

	// Test Customer Identifiers
	// Used for creating test transactions and chargebacks
	TestCustomerID = "cust_test_001"

	// Test Chargeback Case Numbers (fixed for idempotency)
	// These case numbers are used consistently across test runs to prevent duplicates
	// Each represents a different chargeback status for testing state transitions
	ChargebackCaseNumberNew       = "CB-NEW-TEST"       // Newly received chargeback
	ChargebackCaseNumberPending   = "CB-PENDING-TEST"   // Under review
	ChargebackCaseNumberResponded = "CB-RESPONDED-TEST" // Evidence submitted
	ChargebackCaseNumberWon       = "CB-WON-TEST"       // Merchant won dispute
	ChargebackCaseNumberLost      = "CB-LOST-TEST"      // Merchant lost dispute

	// Test Data Markers
	// These JSON fields mark test data for safe cleanup
	// CLEANUP: Use WHERE raw_data->>'test' = 'true' to delete only test data
	TestDataMarkerKey   = "test"
	TestDataMarkerValue = true
	TestDataSourceKey   = "source"
	TestDataSource      = "test_seed"

	// Database Timeouts (in seconds)
	// These timeouts prevent tests from hanging indefinitely
	// Adjust based on database performance in CI/CD environment
	DBQueryTimeout  = 10 // SELECT queries with simple WHERE clauses
	DBInsertTimeout = 15 // INSERT/UPDATE operations with complex data
	DBSelectTimeout = 5  // Simple SELECT queries for lookups
)

// ChargebackTestCases returns all test chargeback case numbers
// Use this helper to iterate over all test chargebacks in tests
func ChargebackTestCases() []string {
	return []string{
		ChargebackCaseNumberNew,
		ChargebackCaseNumberPending,
		ChargebackCaseNumberResponded,
		ChargebackCaseNumberWon,
		ChargebackCaseNumberLost,
	}
}

// CleanupTestData provides SQL to clean up test data from database
// DEPRECATED: Use CleanupChargebacks(t) and CleanupTestTransactions(t) instead
// This function is kept for backward compatibility with manual cleanup scripts
//
// SECURITY: Only deletes data marked with test markers or belonging to test merchant
func CleanupTestData() string {
	return `
		DELETE FROM chargebacks WHERE raw_data->>'test' = 'true';
		DELETE FROM transactions WHERE merchant_id = '00000000-0000-0000-0000-000000000001'::uuid;
	`
}
