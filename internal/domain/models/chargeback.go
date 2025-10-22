package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// ChargebackStatus represents the status of a chargeback
type ChargebackStatus string

const (
	ChargebackPending   ChargebackStatus = "pending"
	ChargebackResponded ChargebackStatus = "responded"
	ChargebackWon       ChargebackStatus = "won"
	ChargebackLost      ChargebackStatus = "lost"
	ChargebackAccepted  ChargebackStatus = "accepted" // Not contesting
)

// ChargebackOutcome represents the final outcome of a chargeback dispute
type ChargebackOutcome string

const (
	OutcomeReversed ChargebackOutcome = "reversed" // Merchant won
	OutcomeUpheld   ChargebackOutcome = "upheld"   // Customer won
	OutcomePartial  ChargebackOutcome = "partial"  // Split decision
)

// ChargebackCategory categorizes the type of chargeback
type ChargebackCategory string

const (
	CategoryFraud           ChargebackCategory = "fraud"
	CategoryAuthorization   ChargebackCategory = "authorization"
	CategoryProcessingError ChargebackCategory = "processing_error"
	CategoryConsumerDispute ChargebackCategory = "consumer_dispute"
)

// Chargeback represents a payment dispute filed by a cardholder
type Chargeback struct {
	ID            string
	TransactionID string
	MerchantID    string
	CustomerID    string

	// Chargeback identification
	ChargebackID string // Gateway's chargeback ID

	// Financial details
	Amount   decimal.Decimal
	Currency string

	// Reason information
	ReasonCode        string
	ReasonDescription string
	Category          ChargebackCategory

	// Timeline
	ChargebackDate      time.Time  // When chargeback was filed by customer
	ReceivedDate        time.Time  // When we were notified
	RespondByDate       *time.Time // Deadline to submit evidence
	ResponseSubmittedAt *time.Time // When we submitted evidence
	ResolvedAt          *time.Time // When outcome was determined

	// Status and outcome
	Status  ChargebackStatus
	Outcome *ChargebackOutcome

	// Evidence and documentation
	EvidenceFiles []string // File paths or URLs
	ResponseNotes string   // Notes included with evidence submission
	InternalNotes string   // Internal team notes

	// Metadata
	RawData   map[string]interface{} // Raw webhook/notification data
	CreatedAt time.Time
	UpdatedAt time.Time
}
