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
	// Identity
	ID string `json:"id"` // UUID

	// Link to transaction group (get all transaction data via JOIN)
	GroupID string `json:"group_id"` // References transactions.group_id

	// Multi-tenant
	AgentID string `json:"agent_id"`

	// Customer (can be NULL for guest transactions)
	CustomerID *string `json:"customer_id"`

	// North API fields
	CaseNumber        string          `json:"case_number"` // North's unique case identifier
	DisputeDate       time.Time       `json:"dispute_date"`
	ChargebackDate    time.Time       `json:"chargeback_date"`
	ChargebackAmount  decimal.Decimal `json:"chargeback_amount"`
	Currency          string          `json:"currency"`
	ReasonCode        string          `json:"reason_code"`        // North's reason code (e.g., "P22", "F10")
	ReasonDescription *string         `json:"reason_description"` // Human-readable reason

	// Status and timeline
	Status              ChargebackStatus `json:"status"`
	RespondByDate       *time.Time       `json:"respond_by_date"`
	ResponseSubmittedAt *time.Time       `json:"response_submitted_at"`
	ResolvedAt          *time.Time       `json:"resolved_at"`

	// Evidence and response (read-only, synced from North API or manually updated via North portal)
	EvidenceFileURLs []string `json:"evidence_file_urls"` // File URLs if provided by North API
	ResponseText     *string  `json:"response_text"`      // Written response to dispute (if submitted via North portal)
	InternalNotes    *string  `json:"internal_notes"`     // Internal team notes (local tracking only)

	// Store full North API response for debugging
	RawData map[string]interface{} `json:"raw_data"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsOpen returns true if the chargeback is still open/actionable
func (c *Chargeback) IsOpen() bool {
	return c.Status == ChargebackStatusNew || c.Status == ChargebackStatusPending
}

// IsResolved returns true if the chargeback has been resolved
func (c *Chargeback) IsResolved() bool {
	return c.Status == ChargebackStatusWon ||
		c.Status == ChargebackStatusLost ||
		c.Status == ChargebackStatusAccepted
}

// CanRespond returns true if the merchant can still submit evidence
func (c *Chargeback) CanRespond() bool {
	if !c.IsOpen() {
		return false
	}

	// Check if response deadline has passed
	if c.RespondByDate != nil && time.Now().After(*c.RespondByDate) {
		return false
	}

	// Check if already responded
	if c.ResponseSubmittedAt != nil {
		return false
	}

	return true
}

// IsOverdue returns true if the response deadline has passed
func (c *Chargeback) IsOverdue() bool {
	if c.RespondByDate == nil {
		return false
	}

	return c.IsOpen() && time.Now().After(*c.RespondByDate)
}

// DaysUntilDeadline returns the number of days until the response deadline
func (c *Chargeback) DaysUntilDeadline() int {
	if c.RespondByDate == nil {
		return 0
	}

	duration := time.Until(*c.RespondByDate)
	return int(duration.Hours() / 24)
}

// MarkResponded marks the chargeback as responded with evidence
func (c *Chargeback) MarkResponded() {
	now := time.Now()
	c.ResponseSubmittedAt = &now
	c.Status = ChargebackStatusResponded
	c.UpdatedAt = now
}

// MarkResolved marks the chargeback as resolved with the given outcome
func (c *Chargeback) MarkResolved(status ChargebackStatus) error {
	validStatuses := map[ChargebackStatus]bool{
		ChargebackStatusWon:      true,
		ChargebackStatusLost:     true,
		ChargebackStatusAccepted: true,
	}

	if !validStatuses[status] {
		return ErrInvalidChargebackStatus
	}

	now := time.Now()
	c.Status = status
	c.ResolvedAt = &now
	c.UpdatedAt = now

	return nil
}

// GetCustomerID safely retrieves the customer ID
func (c *Chargeback) GetCustomerID() string {
	if c.CustomerID != nil {
		return *c.CustomerID
	}
	return ""
}
