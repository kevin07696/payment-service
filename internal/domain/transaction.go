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
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	AuthCardType        *string                `json:"auth_card_type"`
	AuthRespText        *string                `json:"auth_resp_text"`
	SubscriptionID      *string                `json:"subscription_id"`
	ParentTransactionID *string                `json:"parent_transaction_id"`
	Metadata            map[string]interface{} `json:"metadata"`
	IdempotencyKey      *string                `json:"idempotency_key"`
	AuthCVV2            *string                `json:"auth_cvv2"`
	AuthAVS             *string                `json:"auth_avs"`
	PaymentMethodID     *string                `json:"payment_method_id"`
	CustomerID          *string                `json:"customer_id"`
	AuthResp            *string                `json:"auth_resp"`
	AuthCode            *string                `json:"auth_code"`
	AuthGUID            string                 `json:"auth_guid"`
	ID                  string                 `json:"id"`
	PaymentMethodType   PaymentMethodType      `json:"payment_method_type"`
	Type                TransactionType        `json:"type"`
	Status              TransactionStatus      `json:"status"`
	Currency            string                 `json:"currency"`
	MerchantID          string                 `json:"merchant_id"`
	AmountCents         int64                  `json:"amount_cents"`
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
