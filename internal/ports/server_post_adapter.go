package ports

import (
	"context"
	"time"
)

// TransactionType represents the type of EPX Server Post transaction.
type TransactionType string

const (
	// Credit Card E-commerce Transactions
	TransactionTypeSale     TransactionType = "CCE1" // CC Ecommerce Sale (auth + capture)
	TransactionTypeAuthOnly TransactionType = "CCE2" // CC Ecommerce Auth Only
	TransactionTypeCapture  TransactionType = "CCE4" // CC Ecommerce Capture
	TransactionTypeRefund   TransactionType = "CCE9" // CC Ecommerce Refund/Credit
	TransactionTypeVoid     TransactionType = "CCEX" // CC Ecommerce Void
	TransactionTypeReversal TransactionType = "CCE7" // CC Ecommerce Reversal

	// BRIC Storage (Tokenization)
	TransactionTypeBRICStorageCC  TransactionType = "CCE8" // BRIC Storage - Credit Card
	TransactionTypeBRICStorageACH TransactionType = "CKC8" // BRIC Storage - ACH Checking

	// ACH Checking Account Transactions
	TransactionTypeACHDebit         TransactionType = "CKC2" // ACH Checking Debit/Sale
	TransactionTypeACHCredit        TransactionType = "CKC3" // ACH Checking Credit/Refund
	TransactionTypeACHPreNoteDebit  TransactionType = "CKC0" // ACH Checking Pre-Note Debit
	TransactionTypeACHPreNoteCredit TransactionType = "CKC1" // ACH Checking Pre-Note Credit
	TransactionTypeACHVoid          TransactionType = "CKCX" // ACH Checking Void

	// ACH Savings Account Transactions
	TransactionTypeACHSavingsDebit         TransactionType = "CKS2" // ACH Savings Debit/Sale
	TransactionTypeACHSavingsCredit        TransactionType = "CKS3" // ACH Savings Credit/Refund
	TransactionTypeACHSavingsPreNoteDebit  TransactionType = "CKS0" // ACH Savings Pre-Note Debit
	TransactionTypeACHSavingsPreNoteCredit TransactionType = "CKS1" // ACH Savings Pre-Note Credit
	TransactionTypeACHSavingsVoid          TransactionType = "CKSX" // ACH Savings Void

	// PIN-less Debit Transactions
	TransactionTypePINlessDebitPurchase TransactionType = "DB0P" // PIN-less Debit Purchase
	TransactionTypePINlessDebitReturn   TransactionType = "DB0S" // PIN-less Debit Return
	TransactionTypePINlessDebitVoid     TransactionType = "DB0V" // PIN-less Debit Void
)

// Operation represents a semantic payment operation (gateway-agnostic).
type Operation string

const (
	OperationSale      Operation = "sale"
	OperationRefund    Operation = "refund"
	OperationVoid      Operation = "void"
	OperationAuthorize Operation = "authorize"
	OperationCapture   Operation = "capture"
	OperationStorage   Operation = "storage"
)

// PaymentMethodType represents the payment method.
type PaymentMethodType string

const (
	PaymentMethodTypeCreditCard   PaymentMethodType = "credit_card"
	PaymentMethodTypeACH          PaymentMethodType = "ach"
	PaymentMethodTypePINlessDebit PaymentMethodType = "pinless_debit"
)

// ServerPostRequest contains all parameters for EPX Server Post transaction.
type ServerPostRequest struct {
	// Agent credentials (required)
	CustNbr     string
	MerchNbr    string
	DBAnbr      string
	TerminalNbr string

	// Semantic operation (gateway-agnostic)
	Operation Operation

	// Transaction details (required)
	TransactionType TransactionType
	Amount          string
	PaymentType     PaymentMethodType

	// Payment token (required for BRIC/recurring)
	AuthGUID string

	// Transaction identification
	TranNbr   string
	TranGroup string

	// For capture/void/refund
	OriginalAuthGUID string
	OriginalAmount   string

	// Account information
	AccountNumber  *string
	RoutingNumber  *string
	ExpirationDate *string
	CVV            *string

	// Billing information
	FirstName *string
	LastName  *string
	Address   *string
	City      *string
	State     *string
	ZipCode   *string

	// Card Entry Method ("E" = ecommerce, "Z" = BRIC/token, "X" = ACH)
	CardEntryMethod *string

	// Industry Type ("E" = ecommerce)
	IndustryType *string

	// ACI Extension (for COF, MIT, Recurring)
	ACIExt *string

	// ACH-specific fields
	StdEntryClass *string
	ReceiverName  *string

	// Optional metadata
	CustomerID string
	Metadata   map[string]string
}

// ServerPostResponse contains parsed response from EPX Server Post.
type ServerPostResponse struct {
	AuthGUID             string
	AuthResp             string
	AuthCode             string
	AuthRespText         string
	IsApproved           bool
	AuthCardType         string
	AuthAVS              string
	AuthCVV2             string
	NetworkTransactionID *string
	TranNbr              string
	TranGroup            string
	Amount               string
	ProcessedAt          time.Time
	RawXML               string
}

// ServerPostAdapter defines the port for EPX Server Post API.
type ServerPostAdapter interface {
	ProcessTransaction(ctx context.Context, req *ServerPostRequest) (*ServerPostResponse, error)
	ProcessTransactionViaSocket(ctx context.Context, req *ServerPostRequest) (*ServerPostResponse, error)
	ValidateToken(ctx context.Context, authGUID string) error
}
