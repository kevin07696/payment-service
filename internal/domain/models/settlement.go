package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// SettlementStatus represents the status of a settlement batch
type SettlementStatus string

const (
	SettlementPending     SettlementStatus = "pending"
	SettlementReconciled  SettlementStatus = "reconciled"
	SettlementDiscrepancy SettlementStatus = "discrepancy"
	SettlementCompleted   SettlementStatus = "completed"
)

// SettlementBatch represents a batch of settled transactions
type SettlementBatch struct {
	ID       string
	MerchantID string

	// Settlement identification
	SettlementBatchID string    // Gateway's batch ID
	SettlementDate    time.Time // Date transactions settled
	DepositDate       *time.Time // Date funds deposited to bank

	// Financial summary
	TotalSales       decimal.Decimal
	TotalRefunds     decimal.Decimal
	TotalChargebacks decimal.Decimal
	TotalFees        decimal.Decimal
	NetAmount        decimal.Decimal // Actual amount deposited
	Currency         string

	// Transaction counts
	SalesCount      int32
	RefundCount     int32
	ChargebackCount int32

	// Reconciliation
	Status            SettlementStatus
	ReconciledAt      *time.Time
	DiscrepancyAmount *decimal.Decimal // Difference between expected and actual
	DiscrepancyNotes  string           // Explanation of discrepancy

	// Metadata
	RawData   map[string]interface{} // Raw settlement report data
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SettlementTransaction represents an individual transaction within a settlement batch
type SettlementTransaction struct {
	ID                string
	SettlementBatchID string
	TransactionID     *string // Link to our transaction record (if available)

	// Transaction identification
	GatewayTransactionID string    // Gateway's transaction ID
	TransactionDate      time.Time // Original transaction date
	SettlementDate       time.Time // Date it settled

	// Financial details
	GrossAmount decimal.Decimal // Original transaction amount
	FeeAmount   decimal.Decimal // Processing fees
	NetAmount   decimal.Decimal // Net after fees
	Currency    string

	// Transaction information
	TransactionType string // "SALE", "REFUND", "CHARGEBACK"
	CardBrand       string // "VISA", "MASTERCARD", "AMEX", "DISCOVER"
	CardType        string // "CREDIT", "DEBIT"

	// Interchange (if provided by gateway)
	InterchangeRate *decimal.Decimal // e.g., 0.0195 for 1.95%
	InterchangeFee  *decimal.Decimal // Actual interchange fee amount

	CreatedAt time.Time
}
