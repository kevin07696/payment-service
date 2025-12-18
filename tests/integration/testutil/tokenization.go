package testutil

import (
	"fmt"
	"testing"
	"time"
)

// TestCard represents a test credit card
type TestCard struct {
	Number   string
	ExpMonth string
	ExpYear  string
	CVV      string
	ZipCode  string
	CardType string // "visa", "mastercard", "amex", "discover"
	LastFour string
}

// TestACH represents a test ACH account
type TestACH struct {
	RoutingNumber string
	AccountNumber string
	AccountType   string // "checking" or "savings"
	LastFour      string
}

// Test cards (EPX sandbox)
// Per EPX Certification Response Code Triggers documentation:
// - Card: 4000000000000002
// - CVV: 123
// - Address: 123 N CENTRAL
// - Zip: 12345
// - Exp Date: 2512 (December 2025)
// - Amount triggers: $1.00 = approval (00), $1.05 = decline (05), etc.
var (
	TestVisaCard = TestCard{
		Number:   "4000000000000002", // EPX sandbox approval card
		ExpMonth: "12",
		ExpYear:  "25", // EPX docs specify 2512
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "visa",
		LastFour: "0002",
	}

	TestMastercardCard = TestCard{
		Number:   "5555555555554444",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "mastercard",
		LastFour: "4444",
	}

	TestAmexCard = TestCard{
		Number:   "378282246310005",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "1234",
		ZipCode:  "12345",
		CardType: "amex",
		LastFour: "0005",
	}

	TestDiscoverCard = TestCard{
		Number:   "6011111111111117",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "discover",
		LastFour: "1117",
	}

	TestACHChecking = TestACH{
		RoutingNumber: "021000021",
		AccountNumber: "1234567890",
		AccountType:   "checking",
		LastFour:      "7890",
	}

	TestACHSavings = TestACH{
		RoutingNumber: "021000021",
		AccountNumber: "9876543210",
		AccountType:   "savings",
		LastFour:      "3210",
	}

	// Test debit cards (for PIN-less debit DB0P transactions)
	// PIN-less debit uses same card numbers as credit cards but different transaction type
	TestVisaDebitCard = TestCard{
		Number:   "4111111111111111",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "visa",
		LastFour: "1111",
	}

	TestMastercardDebitCard = TestCard{
		Number:   "5555555555554444",
		ExpMonth: "12",
		ExpYear:  fmt.Sprintf("%d", time.Now().Year()+1),
		CVV:      "123",
		ZipCode:  "12345",
		CardType: "mastercard",
		LastFour: "4444",
	}
)

// SkipIfBRICStorageUnavailable skips tests that require EPX BRIC Storage
// BRIC Storage (CCE8/CKC8) is now available and working for ACH and credit cards
// This function is kept for backward compatibility but no longer skips
func SkipIfBRICStorageUnavailable(t *testing.T) {
	// BRIC Storage is working - no longer skip
	// Tests will fail if credentials are missing, which is the desired behavior
}
