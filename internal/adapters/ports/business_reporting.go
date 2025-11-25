package ports

import (
	"context"
	"time"
)

// TransactionStatus represents the current status of a transaction
type TransactionStatus string

const (
	TransactionStatusApproved TransactionStatus = "approved"
	TransactionStatusDeclined TransactionStatus = "declined"
	TransactionStatusPending  TransactionStatus = "pending"
	TransactionStatusReturned TransactionStatus = "returned" // ACH return
	TransactionStatusVoided   TransactionStatus = "voided"
	TransactionStatusRefunded TransactionStatus = "refunded"
	TransactionStatusSettled  TransactionStatus = "settled"
)

// TransactionDetails contains detailed information about a transaction
type TransactionDetails struct {
	// Core transaction identifiers
	AuthGUID string // EPX BRIC/GUID
	TranNbr  string // Merchant transaction number
	TranType string // Transaction type (CKC0, CKC2, CCE1, etc.)

	// Transaction status
	Status       TransactionStatus
	AuthResp     string // EPX response code (00, 05, 14, etc.)
	AuthRespText string // Human-readable response message

	// ACH-specific return information
	IsACHReturn      bool   // True if this is an ACH return
	ACHReturnCode    string // NACHA return code (R01, R02, R03, etc.)
	ACHReturnReason  string // Human-readable return reason
	ACHReturnDate    *time.Time
	OriginalAuthGUID string // Original transaction if this is a return

	// Transaction amounts
	Amount       string // Transaction amount
	CurrencyCode string // USD, etc.

	// Timestamps
	TransactionDate time.Time
	SettlementDate  *time.Time

	// Payment method information
	PaymentMethod    string // credit_card, ach_checking, ach_savings
	MaskedAccountNbr string // Last 4 digits
	CardType         string // V, M, A, D (for cards) or empty (for ACH)

	// Merchant information
	CustNbr     string
	MerchNbr    string
	DBAnbr      string
	TerminalNbr string
	BatchID     string

	// Additional metadata
	Metadata map[string]string
}

// TransactionQueryParams contains parameters for querying transactions
type TransactionQueryParams struct {
	// Filter by date range
	StartDate *time.Time
	EndDate   *time.Time

	// Filter by transaction types
	TranTypes []string // CKC0, CKC2, CCE1, etc.

	// Filter by status
	Statuses []TransactionStatus

	// Filter by merchant credentials
	CustNbr  string
	MerchNbr string
	DBAnbr   string

	// Filter by payment method
	PaymentMethods []string // credit_card, ach_checking, ach_savings

	// Only include ACH returns
	ACHReturnsOnly bool

	// Pagination
	Limit  int
	Offset int
}

// TransactionQueryResult contains the results of a transaction query
type TransactionQueryResult struct {
	Transactions []*TransactionDetails
	TotalCount   int
	HasMore      bool
}

// BusinessReportingAdapter defines the port for EPX Business Reporting API
// Used to query transaction details, check for ACH returns, and generate reports
type BusinessReportingAdapter interface {
	// GetTransaction retrieves detailed information about a single transaction by AUTH_GUID
	// Returns error if:
	//   - AUTH_GUID not found
	//   - Network communication fails
	//   - API credentials are invalid
	GetTransaction(ctx context.Context, authGUID string) (*TransactionDetails, error)

	// QueryTransactions searches for transactions matching the given criteria
	// Returns paginated results with total count
	// Useful for:
	//   - Finding all ACH returns in a date range
	//   - Checking for returns on multiple BRICs
	//   - Generating reports
	QueryTransactions(ctx context.Context, params *TransactionQueryParams) (*TransactionQueryResult, error)

	// CheckACHReturns checks if a specific AUTH_GUID has any ACH returns
	// This is a convenience method that wraps GetTransaction
	// Returns:
	//   - hasReturn: true if transaction was returned by bank
	//   - returnCode: NACHA return code (R01, R02, R03, etc.)
	//   - error: if query fails
	CheckACHReturns(ctx context.Context, authGUID string) (hasReturn bool, returnCode string, returnReason string, error error)

	// GetACHReturnsForDateRange retrieves all ACH returns within a date range
	// Useful for batch processing of returns in cron jobs
	// Returns all transactions with ACHReturnsOnly=true in the query
	GetACHReturnsForDateRange(ctx context.Context, startDate, endDate time.Time) ([]*TransactionDetails, error)
}
