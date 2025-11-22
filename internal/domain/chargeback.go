package domain

import (
	"time"

	"github.com/kevin07696/payment-service/pkg/timeutil"
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
	if c.RespondByDate != nil && timeutil.Now().After(*c.RespondByDate) {
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

	return c.IsOpen() && timeutil.Now().After(*c.RespondByDate)
}

// DaysUntilDeadline returns the number of days until the response deadline
func (c *Chargeback) DaysUntilDeadline() int {
	if c.RespondByDate == nil {
		return 0
	}

	duration := c.RespondByDate.Sub(timeutil.Now())
	return int(duration.Hours() / 24)
}

// MarkResponded marks the chargeback as responded with evidence
func (c *Chargeback) MarkResponded() {
	now := timeutil.Now()
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

	now := timeutil.Now()
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
