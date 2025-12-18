package ports

import (
	"context"
	"time"
)

// TransactionStatus represents the current status of a transaction.
type TransactionStatus string

const (
	TransactionStatusApproved TransactionStatus = "approved"
	TransactionStatusDeclined TransactionStatus = "declined"
	TransactionStatusPending  TransactionStatus = "pending"
	TransactionStatusReturned TransactionStatus = "returned"
	TransactionStatusVoided   TransactionStatus = "voided"
	TransactionStatusRefunded TransactionStatus = "refunded"
	TransactionStatusSettled  TransactionStatus = "settled"
)

// TransactionDetails contains detailed information about a transaction.
type TransactionDetails struct {
	AuthGUID         string
	TranNbr          string
	TranType         string
	Status           TransactionStatus
	AuthResp         string
	AuthRespText     string
	IsACHReturn      bool
	ACHReturnCode    string
	ACHReturnReason  string
	ACHReturnDate    *time.Time
	OriginalAuthGUID string
	Amount           string
	CurrencyCode     string
	TransactionDate  time.Time
	SettlementDate   *time.Time
	PaymentMethod    string
	MaskedAccountNbr string
	CardType         string
	CustNbr          string
	MerchNbr         string
	DBAnbr           string
	TerminalNbr      string
	BatchID          string
	Metadata         map[string]string
}

// TransactionQueryParams contains parameters for querying transactions.
type TransactionQueryParams struct {
	StartDate      *time.Time
	EndDate        *time.Time
	TranTypes      []string
	Statuses       []TransactionStatus
	CustNbr        string
	MerchNbr       string
	DBAnbr         string
	PaymentMethods []string
	ACHReturnsOnly bool
	Limit          int
	Offset         int
}

// TransactionQueryResult contains the results of a transaction query.
type TransactionQueryResult struct {
	Transactions []*TransactionDetails
	TotalCount   int
	HasMore      bool
}

// BusinessReportingAdapter defines the port for EPX Business Reporting API.
type BusinessReportingAdapter interface {
	GetTransaction(ctx context.Context, authGUID string) (*TransactionDetails, error)
	QueryTransactions(ctx context.Context, params *TransactionQueryParams) (*TransactionQueryResult, error)
	CheckACHReturns(ctx context.Context, authGUID string) (hasReturn bool, returnCode string, returnReason string, err error)
	GetACHReturnsForDateRange(ctx context.Context, startDate, endDate time.Time) ([]*TransactionDetails, error)
}
