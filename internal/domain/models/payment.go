package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// TransactionStatus represents the current state of a transaction
type TransactionStatus string

const (
	StatusPending    TransactionStatus = "pending"
	StatusAuthorized TransactionStatus = "authorized"
	StatusCaptured   TransactionStatus = "captured"
	StatusVoided     TransactionStatus = "voided"
	StatusRefunded   TransactionStatus = "refunded"
	StatusFailed     TransactionStatus = "failed"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	TypeAuthorization TransactionType = "authorization"
	TypeCapture       TransactionType = "capture"
	TypeSale          TransactionType = "sale"
	TypeRefund        TransactionType = "refund"
	TypeVoid          TransactionType = "void"
	TypeVerification  TransactionType = "verification"
)

// PaymentMethodType represents the payment method used
type PaymentMethodType string

const (
	PaymentMethodCreditCard PaymentMethodType = "credit_card"
	PaymentMethodDebitCard  PaymentMethodType = "debit_card"
	PaymentMethodACH        PaymentMethodType = "ach"
	PaymentMethodToken      PaymentMethodType = "token"
)

// Transaction represents a payment transaction
type Transaction struct {
	ID                   string
	MerchantID           string
	CustomerID           string
	Amount               decimal.Decimal
	Currency             string
	Status               TransactionStatus
	Type                 TransactionType
	PaymentMethodType    PaymentMethodType
	PaymentMethodToken   string // BRIC token
	GatewayTransactionID string
	GatewayResponseCode  string
	GatewayResponseMsg   string
	AVSResponse          string // AVS verification result (Y=match, N=no match, etc.)
	CVVResponse          string // CVV verification result
	IdempotencyKey       string
	Metadata             map[string]string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// GatewayResponse contains the raw gateway response details
type GatewayResponse struct {
	Code      string
	Message   string
	AuthCode  string
	Reference string
	Timestamp time.Time
}

// BillingInfo represents billing information for a payment
type BillingInfo struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
	Address   string
	City      string
	State     string
	ZipCode   string
	Country   string
}

// PaymentMethod represents a tokenized payment method
type PaymentMethod struct {
	Token     string
	Type      PaymentMethodType
	LastFour  string
	Brand     string
	ExpiryMonth int
	ExpiryYear  int
}
