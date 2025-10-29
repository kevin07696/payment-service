package ports

import (
	"context"
	"time"
)

// TransactionType represents the type of EPX Server Post transaction
type TransactionType string

const (
	TransactionTypeAuthOnly TransactionType = "A" // Authorization only
	TransactionTypeCapture  TransactionType = "D" // Capture previous authorization
	TransactionTypeSale     TransactionType = "S" // Sale (auth + capture)
	TransactionTypeRefund   TransactionType = "C" // Credit/refund
	TransactionTypeVoid     TransactionType = "V" // Void transaction
	TransactionTypePreNote  TransactionType = "P" // ACH pre-note verification
)

// PaymentMethodType represents the payment method
type PaymentMethodType string

const (
	PaymentMethodTypeCreditCard PaymentMethodType = "credit_card"
	PaymentMethodTypeACH        PaymentMethodType = "ach"
)

// ServerPostRequest contains all parameters for EPX Server Post transaction
// Based on EPX Server Post API - Request Fields (page 7-11)
type ServerPostRequest struct {
	// Agent credentials (required)
	CustNbr     string // EPX customer number
	MerchNbr    string // EPX merchant number
	DBAnbr      string // EPX DBA number
	TerminalNbr string // EPX terminal number

	// Transaction details (required)
	TransactionType TransactionType   // A, D, S, C, V, P
	Amount          string            // Transaction amount (e.g., "29.99")
	PaymentType     PaymentMethodType // credit_card or ach

	// Payment token (required for BRIC/recurring)
	AuthGUID string // EPX BRIC token from previous transaction or tokenization

	// Transaction identification
	TranNbr   string // Unique transaction number
	TranGroup string // Transaction group ID (our group_id)

	// For capture/void/refund: reference to original transaction
	OriginalAuthGUID string // AUTH_GUID of transaction to capture/void/refund
	OriginalAmount   string // Original transaction amount (for partial refunds)

	// Optional metadata
	CustomerID string            // Our internal customer ID
	Metadata   map[string]string // Additional metadata
}

// ServerPostResponse contains parsed response from EPX Server Post
// Based on EPX Server Post API - Response Format (page 12-15)
type ServerPostResponse struct {
	// Core response fields
	AuthGUID     string // EPX transaction token (BRIC format)
	AuthResp     string // EPX approval code ("00" = approved, "05" = declined, "12" = invalid)
	AuthCode     string // Bank authorization code (NULL if declined)
	AuthRespText string // Human-readable response message
	IsApproved   bool   // Derived from AuthResp ("00" = true)

	// Card/ACH verification (credit card only)
	AuthCardType string // Card brand ("V"/"M"/"A"/"D") - empty for ACH
	AuthAVS      string // Address verification - empty for ACH
	AuthCVV2     string // CVV verification - empty for ACH

	// Transaction echo-back
	TranNbr   string // Echo back transaction number
	TranGroup string // Echo back transaction group
	Amount    string // Echo back amount

	// Timestamps
	ProcessedAt time.Time // When EPX processed the transaction

	// Raw response for debugging
	RawXML string // Raw XML response from EPX
}

// ServerPostAdapter defines the port for EPX Server Post API (direct server-to-server transactions)
// Used primarily for recurring charges with BRIC tokens
// Implementation should support both:
//   - HTTPS POST (key-value pairs over port 443)
//   - XML Socket (port 8086)
type ServerPostAdapter interface {
	// ProcessTransaction sends a transaction request to EPX Server Post API
	// Supports all transaction types: auth, capture, sale, refund, void, pre-note
	// Returns error if:
	//   - Network communication fails
	//   - Credentials are invalid
	//   - EPX service is unavailable
	//   - Request parameters are invalid
	//   - Transaction is declined (error contains decline reason)
	ProcessTransaction(ctx context.Context, req *ServerPostRequest) (*ServerPostResponse, error)

	// ProcessTransactionViaSocket sends transaction via XML Socket connection (port 8086)
	// Useful for high-volume batch processing (socket stays open 30 seconds)
	// Same return signature as ProcessTransaction
	ProcessTransactionViaSocket(ctx context.Context, req *ServerPostRequest) (*ServerPostResponse, error)

	// ValidateToken checks if a BRIC token (AUTH_GUID) is still valid
	// Performs a $0.00 authorization to verify token status
	// Returns error if token is expired or invalid
	ValidateToken(ctx context.Context, authGUID string) error
}
