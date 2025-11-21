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
type Transaction struct {
	// Identity
	ID                  string  `json:"id"`                    // UUID
	ParentTransactionID *string `json:"parent_transaction_id"` // Links related transactions (auth → capture → refund)

	// Multi-tenant
	MerchantID string `json:"merchant_id"` // Which merchant owns this transaction

	// Customer
	CustomerID *string `json:"customer_id"` // NULL for guest transactions

	// Optional subscription reference (for recurring billing)
	SubscriptionID *string `json:"subscription_id"` // NULL for one-time payments

	// Transaction details
	AmountCents       int64             `json:"amount_cents"` // Amount in cents (e.g., 10050 = $100.50)
	Currency          string            `json:"currency"`     // ISO 4217 code (e.g., "USD")
	Status            TransactionStatus `json:"status"`
	Type              TransactionType   `json:"type"`
	PaymentMethodType PaymentMethodType `json:"payment_method_type"`
	PaymentMethodID   *string           `json:"payment_method_id"` // References saved payment method (NULL if one-time)

	// EPX Gateway response fields
	AuthGUID     string  `json:"auth_guid"`      // EPX AUTH_GUID (BRIC token) for this transaction
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
