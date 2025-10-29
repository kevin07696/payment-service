package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// TransactionStatus represents the current state of a transaction
type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
	TransactionStatusRefunded  TransactionStatus = "refunded"
	TransactionStatusVoided    TransactionStatus = "voided"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeAuth    TransactionType = "auth"     // Authorization only
	TransactionTypeCapture TransactionType = "capture"  // Capture authorized funds
	TransactionTypeCharge  TransactionType = "charge"   // Combined auth + capture (sale)
	TransactionTypeRefund  TransactionType = "refund"   // Return funds
	TransactionTypePreNote TransactionType = "pre_note" // ACH verification
)

// PaymentMethodType represents the payment method used
type PaymentMethodType string

const (
	PaymentMethodTypeCreditCard PaymentMethodType = "credit_card"
	PaymentMethodTypeACH        PaymentMethodType = "ach"
)

// Transaction represents a payment transaction
type Transaction struct {
	// Identity
	ID      string `json:"id"`       // UUID
	GroupID string `json:"group_id"` // Links related transactions (auth → capture → refund)

	// Multi-tenant
	AgentID string `json:"agent_id"` // Which agent/merchant owns this transaction

	// Customer
	CustomerID *string `json:"customer_id"` // NULL for guest transactions

	// Transaction details
	Amount              decimal.Decimal   `json:"amount"`
	Currency            string            `json:"currency"` // ISO 4217 code (e.g., "USD")
	Status              TransactionStatus `json:"status"`
	Type                TransactionType   `json:"type"`
	PaymentMethodType   PaymentMethodType `json:"payment_method_type"`
	PaymentMethodID     *string           `json:"payment_method_id"` // References saved payment method (NULL if one-time)

	// EPX Gateway response fields
	AuthGUID     *string `json:"auth_guid"`      // EPX transaction token (BRIC format) - required for refunds/voids/captures
	AuthResp     *string `json:"auth_resp"`      // EPX approval code ("00" = approved, "05" = declined)
	AuthCode     *string `json:"auth_code"`      // Bank authorization code (NULL if declined)
	AuthRespText *string `json:"auth_resp_text"` // Human-readable response message
	AuthCardType *string `json:"auth_card_type"` // Card brand ("V"/"M"/"A"/"D") - NULL for ACH
	AuthAVS      *string `json:"auth_avs"`       // Address verification result
	AuthCVV2     *string `json:"auth_cvv2"`      // CVV verification result

	// Idempotency and metadata
	IdempotencyKey *string                `json:"idempotency_key"`
	Metadata       map[string]interface{} `json:"metadata"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsApproved returns true if the transaction was approved by the gateway
func (t *Transaction) IsApproved() bool {
	return t.AuthResp != nil && *t.AuthResp == "00"
}

// CanBeVoided returns true if the transaction can be voided
func (t *Transaction) CanBeVoided() bool {
	return t.Status == TransactionStatusCompleted &&
		(t.Type == TransactionTypeAuth || t.Type == TransactionTypeCharge)
}

// CanBeCaptured returns true if the transaction can be captured
func (t *Transaction) CanBeCaptured() bool {
	return t.Status == TransactionStatusCompleted && t.Type == TransactionTypeAuth
}

// CanBeRefunded returns true if the transaction can be refunded
func (t *Transaction) CanBeRefunded() bool {
	return t.Status == TransactionStatusCompleted &&
		(t.Type == TransactionTypeCharge || t.Type == TransactionTypeCapture)
}

// GetAuthGUID safely retrieves the AUTH_GUID
func (t *Transaction) GetAuthGUID() string {
	if t.AuthGUID != nil {
		return *t.AuthGUID
	}
	return ""
}

// GetCustomerID safely retrieves the customer ID
func (t *Transaction) GetCustomerID() string {
	if t.CustomerID != nil {
		return *t.CustomerID
	}
	return ""
}