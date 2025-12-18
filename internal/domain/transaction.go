package domain

import (
	"strings"
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
	TransactionTypeAuth     TransactionType = "AUTH"     // Authorization only (EPX TRAN_GROUP=A)
	TransactionTypeCapture  TransactionType = "CAPTURE"  // Capture authorized funds
	TransactionTypeSale     TransactionType = "SALE"     // Combined auth + capture (EPX TRAN_GROUP=U)
	TransactionTypeRefund   TransactionType = "REFUND"   // Return funds
	TransactionTypeVoid     TransactionType = "VOID"     // Cancel transaction before settlement
	TransactionTypeReversal TransactionType = "REVERSAL" // Reversal (CCE7) - releases auth hold AND voids in one call
	TransactionTypePreNote  TransactionType = "PRE_NOTE" // ACH verification
	TransactionTypeStorage  TransactionType = "STORAGE"  // Tokenization (credit card or ACH)
)

// RequestTransactionType represents the transaction type from an incoming request
// This is separate from TransactionType to handle request-specific values like ACH_STORAGE_C/S
type RequestTransactionType string

const (
	RequestTransactionTypeSale        RequestTransactionType = "SALE"
	RequestTransactionTypeAuth        RequestTransactionType = "AUTH"
	RequestTransactionTypeStorage     RequestTransactionType = "STORAGE"
	RequestTransactionTypeACHStorageC RequestTransactionType = "ACH_STORAGE_C" // ACH checking storage
	RequestTransactionTypeACHStorageS RequestTransactionType = "ACH_STORAGE_S" // ACH savings storage
)

// ParseRequestTransactionType parses a string into a RequestTransactionType
// Returns RequestTransactionTypeSale as default for unknown values
func ParseRequestTransactionType(input string) RequestTransactionType {
	switch strings.ToUpper(input) {
	case "AUTH":
		return RequestTransactionTypeAuth
	case "SALE":
		return RequestTransactionTypeSale
	case "STORAGE":
		return RequestTransactionTypeStorage
	case "ACH_STORAGE_C":
		return RequestTransactionTypeACHStorageC
	case "ACH_STORAGE_S":
		return RequestTransactionTypeACHStorageS
	default:
		return RequestTransactionTypeSale
	}
}

// ToTransactionType converts RequestTransactionType to the internal TransactionType
// ACH_STORAGE_C and ACH_STORAGE_S both map to STORAGE
func (r RequestTransactionType) ToTransactionType() TransactionType {
	switch r {
	case RequestTransactionTypeAuth:
		return TransactionTypeAuth
	case RequestTransactionTypeSale:
		return TransactionTypeSale
	case RequestTransactionTypeStorage, RequestTransactionTypeACHStorageC, RequestTransactionTypeACHStorageS:
		return TransactionTypeStorage
	default:
		return TransactionTypeSale
	}
}

// IsACHStorage returns true if this is an ACH storage request type
func (r RequestTransactionType) IsACHStorage() bool {
	return r == RequestTransactionTypeACHStorageC || r == RequestTransactionTypeACHStorageS
}

// IsCheckingAccount returns true if this is a checking account ACH storage
func (r RequestTransactionType) IsCheckingAccount() bool {
	return r == RequestTransactionTypeACHStorageC
}

// IsStorage returns true if this is any storage transaction type
func (r RequestTransactionType) IsStorage() bool {
	return r == RequestTransactionTypeStorage || r.IsACHStorage()
}

// ToPaymentMethodType returns the payment method type for this transaction
func (r RequestTransactionType) ToPaymentMethodType() PaymentMethodType {
	if r.IsACHStorage() {
		return PaymentMethodTypeACH
	}
	return PaymentMethodTypeCreditCard
}

// IsValid returns true if this is a valid transaction type
func (r RequestTransactionType) IsValid() bool {
	switch r {
	case RequestTransactionTypeSale, RequestTransactionTypeAuth, RequestTransactionTypeStorage,
		RequestTransactionTypeACHStorageC, RequestTransactionTypeACHStorageS:
		return true
	default:
		return false
	}
}

// ToEPXTranCode returns the EPX TRAN_CODE value for Browser POST forms
// Per EPX certification: TRAN_CODE uses text values (SALE, AUTH, STORAGE, ACHSTORAGE_C, ACHSTORAGE_S)
func (r RequestTransactionType) ToEPXTranCode() string {
	switch r {
	case RequestTransactionTypeSale:
		return "SALE"
	case RequestTransactionTypeAuth:
		return "AUTH"
	case RequestTransactionTypeStorage:
		return "STORAGE"
	case RequestTransactionTypeACHStorageC:
		return "ACHSTORAGE_C"
	case RequestTransactionTypeACHStorageS:
		return "ACHSTORAGE_S"
	default:
		return "SALE"
	}
}

// ToEPXTranGroup returns the EPX TRAN_GROUP value for Key Exchange
// This is the high-level category sent to EPX Key Exchange API
func (r RequestTransactionType) ToEPXTranGroup() string {
	switch r {
	case RequestTransactionTypeSale:
		return "SALE"
	case RequestTransactionTypeAuth:
		return "AUTH"
	case RequestTransactionTypeStorage, RequestTransactionTypeACHStorageC, RequestTransactionTypeACHStorageS:
		return "STORAGE"
	default:
		return "SALE"
	}
}

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
	Status            TransactionStatus `json:"status"`              // string alias
	Type              TransactionType   `json:"type"`                // string alias
	PaymentMethodType PaymentMethodType `json:"payment_method_type"` // string alias

	// Pointers (8 bytes each) grouped together
	ParentTransactionID *string `json:"parent_transaction_id"`
	CustomerID          *string `json:"customer_id"`
	OrderID             *string `json:"order_id"` // Merchant's external order/invoice ID
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

// CanBeReversed returns true if the transaction can be reversed (CCE7)
// Reversal releases the authorization hold AND voids the transaction in one call
// This provides immediate fund release to cardholder (vs Void which holds 3-10 days)
// Only approved AUTH and SALE transactions can be reversed (credit cards only)
func (t *Transaction) CanBeReversed() bool {
	return t.Status == TransactionStatusApproved &&
		(t.Type == TransactionTypeAuth || t.Type == TransactionTypeSale)
}

// GetCustomerID safely retrieves the customer ID
func (t *Transaction) GetCustomerID() string {
	if t.CustomerID != nil {
		return *t.CustomerID
	}
	return ""
}
