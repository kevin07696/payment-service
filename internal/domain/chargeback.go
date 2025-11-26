package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Chargeback domain model represents dispute data synced from North API (read-only).
// North does not provide write APIs for disputes - merchants must respond via North's web portal.
// This service provides read-only access to dispute data for monitoring and webhook notifications.

// ChargebackStatus represents the chargeback state
type ChargebackStatus string

const (
	ChargebackStatusNew       ChargebackStatus = "new"       // Just received from North API
	ChargebackStatusPending   ChargebackStatus = "pending"   // Under review
	ChargebackStatusResponded ChargebackStatus = "responded" // Evidence submitted
	ChargebackStatusWon       ChargebackStatus = "won"       // Merchant won the dispute
	ChargebackStatusLost      ChargebackStatus = "lost"      // Merchant lost the dispute
	ChargebackStatusAccepted  ChargebackStatus = "accepted"  // Merchant accepted the chargeback
)

// Chargeback represents a payment dispute/chargeback
type Chargeback struct {
	DisputeDate         time.Time              `json:"dispute_date"`
	UpdatedAt           time.Time              `json:"updated_at"`
	CreatedAt           time.Time              `json:"created_at"`
	ChargebackDate      time.Time              `json:"chargeback_date"`
	ReasonDescription   *string                `json:"reason_description"`
	InternalNotes       *string                `json:"internal_notes"`
	CustomerID          *string                `json:"customer_id"`
	RawData             map[string]interface{} `json:"raw_data"`
	ResponseText        *string                `json:"response_text"`
	ResolvedAt          *time.Time             `json:"resolved_at"`
	ResponseSubmittedAt *time.Time             `json:"response_submitted_at"`
	RespondByDate       *time.Time             `json:"respond_by_date"`
	Status              ChargebackStatus       `json:"status"`
	ID                  string                 `json:"id"`
	ReasonCode          string                 `json:"reason_code"`
	Currency            string                 `json:"currency"`
	CaseNumber          string                 `json:"case_number"`
	ChargebackAmount    decimal.Decimal        `json:"chargeback_amount"`
	AgentID             string                 `json:"agent_id"`
	TransactionID       string                 `json:"transaction_id"`
	EvidenceFileURLs    []string               `json:"evidence_file_urls"`
}

// Note: Domain methods (IsOpen, IsResolved, CanRespond, IsOverdue, DaysUntilDeadline,
// MarkResponded, MarkResolved, GetCustomerID) were removed because chargebacks are
// read-only in this system. North does not provide write APIs for disputes - merchants
// must respond via North's web portal. Handlers query the database directly.
