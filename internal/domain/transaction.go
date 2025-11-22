package domain

import (
	"time"
)

// TransactionStatus represents the outcome of a transaction (approved/declined by gateway)
// This is NOT the transaction lifecycle state - use TransactionType for that
type TransactionStatus string

const (
	TransactionStatusApproved TransactionStatus = "approved" // Gateway approved (auth_resp='00')
	TransactionStatusDeclined TransactionStatus = "declined" // Gateway declined (auth_resp != '00')
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeAuth    TransactionType = "AUTH"     // Authorization only (EPX TRAN_GROUP=A)
	TransactionTypeCapture TransactionType = "CAPTURE"  // Capture authorized funds
	TransactionTypeSale    TransactionType = "SALE"     // Combined auth + capture (EPX TRAN_GROUP=U)
	TransactionTypeRefund  TransactionType = "REFUND"   // Return funds
	TransactionTypeVoid    TransactionType = "VOID"     // Cancel transaction before settlement
	TransactionTypePreNote TransactionType = "PRE_NOTE" // ACH verification
	TransactionTypeStorage TransactionType = "STORAGE"  // Tokenization (credit card or ACH)
)

// PaymentMethodType represents the payment method used
type PaymentMethodType string

const (
	PaymentMethodTypeCreditCard PaymentMethodType = "credit_card"
	PaymentMethodTypeACH        PaymentMethodType = "ach"
)

// Transaction represents a payment transaction
// Field order optimized for memory alignment (largest to smallest):
// - time.Time (24 bytes) first
// - map (8 bytes header)
// - strings (16 bytes)
// - pointers (8 bytes)
// - int64 (8 bytes)
type Transaction struct {
	// Timestamps (24 bytes each) - largest fields first
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Map (8 byte header + data)
	Metadata map[string]interface{} `json:"metadata"`

	// Strings (16 bytes each on 64-bit)
	ID                string            `json:"id"`
	MerchantID        string            `json:"merchant_id"`
	AuthGUID          string            `json:"auth_guid"`
	Currency          string            `json:"currency"`
	Status            TransactionStatus `json:"status"` // string alias
	Type              TransactionType   `json:"type"`   // string alias
	PaymentMethodType PaymentMethodType `json:"payment_method_type"` // string alias

	// Pointers (8 bytes each) grouped together
	ParentTransactionID *string `json:"parent_transaction_id"`
	CustomerID          *string `json:"customer_id"`
	SubscriptionID      *string `json:"subscription_id"`
	PaymentMethodID     *string `json:"payment_method_id"`
	IdempotencyKey      *string `json:"idempotency_key"`
	AuthResp            *string `json:"auth_resp"`
	AuthCode            *string `json:"auth_code"`
	AuthRespText        *string `json:"auth_resp_text"`
	AuthCardType        *string `json:"auth_card_type"`
	AuthAVS             *string `json:"auth_avs"`
	AuthCVV2            *string `json:"auth_cvv2"`

	// int64 (8 bytes) - same size as pointers, grouped at end
	AmountCents int64 `json:"amount_cents"`
}

// IsApproved returns true if the transaction was approved by the gateway
func (t *Transaction) IsApproved() bool {
	return t.AuthResp != nil && *t.AuthResp == "00"
}

// CanBeVoided returns true if the transaction can be voided
func (t *Transaction) CanBeVoided() bool {
	return t.Status == TransactionStatusApproved &&
		(t.Type == TransactionTypeAuth || t.Type == TransactionTypeSale)
}

// CanBeCaptured returns true if the transaction can be captured
func (t *Transaction) CanBeCaptured() bool {
	return t.Status == TransactionStatusApproved && t.Type == TransactionTypeAuth
}

// CanBeRefunded returns true if the transaction can be refunded
func (t *Transaction) CanBeRefunded() bool {
	return t.Status == TransactionStatusApproved &&
		(t.Type == TransactionTypeSale || t.Type == TransactionTypeCapture)
}

// GetCustomerID safely retrieves the customer ID
func (t *Transaction) GetCustomerID() string {
	if t.CustomerID != nil {
		return *t.CustomerID
	}
	return ""
}
