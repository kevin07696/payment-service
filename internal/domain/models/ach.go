package models

// ACHAccountType represents the type of bank account
type ACHAccountType string

const (
	AccountTypeChecking ACHAccountType = "checking"
	AccountTypeSavings  ACHAccountType = "savings"
)

// SECCode represents Standard Entry Class codes for ACH
type SECCode string

const (
	SECCodePPD SECCode = "PPD" // Prearranged Payment and Deposit
	SECCodeWEB SECCode = "WEB" // Internet-Initiated Entry
	SECCodeCCD SECCode = "CCD" // Corporate Credit or Debit
	SECCodeTEL SECCode = "TEL" // Telephone-Initiated Entry
	SECCodeARC SECCode = "ARC" // Accounts Receivable Entry
)

// ACHTransaction represents an ACH payment transaction
type ACHTransaction struct {
	Transaction // Embeds base transaction
	AccountType ACHAccountType
	SECCode     SECCode
	RoutingNumber string
	AccountNumberLastFour string
}
