package fixtures

import "time"

// EPXBRICFixture represents a real BRIC token obtained from EPX for testing
type EPXBRICFixture struct {
	BRIC          string    // The actual BRIC token from EPX
	Type          string    // "AUTH" or "SALE"
	Amount        string    // Amount used when generating the BRIC
	ObtainedAt    time.Time // When this BRIC was obtained
	ExpiresAt     time.Time // When this BRIC expires (13-24 months from obtained date)
	Notes         string    // Additional notes about this BRIC
	TransactionID string    // Original transaction ID used to generate this BRIC
	GroupID       string    // Group ID associated with this BRIC
}

// Real BRICs obtained from EPX sandbox for integration testing
// These BRICs are valid for 13-24 months and can be reused across tests
// To get new BRICs, see tests/manual/README.md

var (
	// ValidAuthBRIC is a real BRIC from EPX for AUTH transactions
	// Use this to test CAPTURE and VOID operations
	// TODO: Replace with real BRIC from EPX sandbox
	// Run: xdg-open tests/manual/get_real_bric.html
	ValidAuthBRIC = EPXBRICFixture{
		BRIC:       "REPLACE-WITH-REAL-BRIC-FROM-EPX-AFTER-RUNNING-MANUAL-TEST",
		Type:       "AUTH",
		Amount:     "10.00",
		ObtainedAt: time.Date(2025, 11, 12, 0, 0, 0, 0, time.UTC),
		ExpiresAt:  time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC), // 13 months
		Notes:      "AUTH BRIC for testing CAPTURE and VOID operations. Obtain from manual test.",
	}

	// ValidSaleBRIC is a real BRIC from EPX for SALE transactions
	// Use this to test REFUND operations
	// TODO: Replace with real BRIC from EPX sandbox
	// Run: xdg-open tests/manual/get_real_bric.html (select SALE type)
	ValidSaleBRIC = EPXBRICFixture{
		BRIC:       "REPLACE-WITH-REAL-BRIC-FROM-EPX-AFTER-RUNNING-MANUAL-TEST",
		Type:       "SALE",
		Amount:     "10.00",
		ObtainedAt: time.Date(2025, 11, 12, 0, 0, 0, 0, time.UTC),
		ExpiresAt:  time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC), // 13 months
		Notes:      "SALE BRIC for testing REFUND operations. Obtain from manual test.",
	}
)

// IsExpired checks if a BRIC fixture has expired
func (b *EPXBRICFixture) IsExpired() bool {
	return time.Now().After(b.ExpiresAt)
}

// IsPlaceholder checks if this is a placeholder that needs to be replaced with a real BRIC
func (b *EPXBRICFixture) IsPlaceholder() bool {
	return b.BRIC == "" || b.BRIC == "REPLACE-WITH-REAL-BRIC-FROM-EPX-AFTER-RUNNING-MANUAL-TEST"
}

// NeedsRefresh checks if this BRIC needs to be refreshed (expired or placeholder)
func (b *EPXBRICFixture) NeedsRefresh() bool {
	return b.IsPlaceholder() || b.IsExpired()
}

// GetValidBRIC returns a valid BRIC for the given transaction type
// Returns empty string and false if no valid BRIC is available
func GetValidBRIC(transactionType string) (string, bool) {
	var fixture *EPXBRICFixture

	switch transactionType {
	case "AUTH":
		fixture = &ValidAuthBRIC
	case "SALE":
		fixture = &ValidSaleBRIC
	default:
		return "", false
	}

	if fixture.NeedsRefresh() {
		return "", false
	}

	return fixture.BRIC, true
}

// CheckFixtures validates all BRIC fixtures and returns status
func CheckFixtures() map[string]string {
	status := make(map[string]string)

	if ValidAuthBRIC.IsPlaceholder() {
		status["AUTH"] = "❌ Placeholder - needs real BRIC from EPX"
	} else if ValidAuthBRIC.IsExpired() {
		status["AUTH"] = "⚠️  Expired - needs refresh"
	} else {
		status["AUTH"] = "✅ Valid"
	}

	if ValidSaleBRIC.IsPlaceholder() {
		status["SALE"] = "❌ Placeholder - needs real BRIC from EPX"
	} else if ValidSaleBRIC.IsExpired() {
		status["SALE"] = "⚠️  Expired - needs refresh"
	} else {
		status["SALE"] = "✅ Valid"
	}

	return status
}
