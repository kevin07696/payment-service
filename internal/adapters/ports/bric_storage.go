package ports

import (
	"context"
)

// BRICStorageRequest contains parameters for creating or converting to Storage BRIC
// Based on EPX Transaction Specs - BRIC Storage.pdf
type BRICStorageRequest struct {
	// Agent credentials (required)
	CustNbr     string
	MerchNbr    string
	DBAnbr      string
	TerminalNbr string

	// Transaction identification
	BatchID string // Batch identifier
	TranNbr string // Unique transaction number

	// Payment type (determines transaction type)
	PaymentType PaymentMethodType // credit_card or ach

	// Option 1: Convert Financial BRIC to Storage BRIC
	// Use when customer completes payment and wants to save payment method
	FinancialBRIC *string // AUTH_GUID from previous financial transaction

	// Option 2: Create Storage BRIC from account information
	// Use when customer adds payment method without making a purchase
	AccountNumber  *string // Card number or bank account number
	RoutingNumber  *string // For ACH only
	ExpirationDate *string // YYMM format for credit cards
	CVV            *string // CVV for initial validation (not stored)

	// Billing information (required for Account Verification on credit cards)
	FirstName *string
	LastName  *string
	Address   *string
	City      *string
	State     *string
	ZipCode   *string

	// User data fields (up to 10 available)
	UserData1 *string
}

// BRICStorageResponse contains the response from BRIC Storage transaction
type BRICStorageResponse struct {
	// Storage BRIC (never expires)
	StorageBRIC string // AUTH_GUID to store in payment_methods table

	// Network Transaction ID (for credit cards - card-on-file compliance)
	// This is linked to the Storage BRIC and proves Account Verification was performed
	NetworkTransactionID *string // NTID from Account Verification

	// Account Verification results (credit cards only)
	AuthResp     string  // "00" = approved, "85" = not declined (treated as approval)
	AuthRespText string  // Human-readable response
	AuthAVS      *string // Address verification result
	AuthCVV2     *string // CVV verification result
	AuthCardType *string // Card brand ("V"/"M"/"A"/"D")

	// ACH validation results
	RoutingNumberValid *bool // true if routing number is valid

	// Echo-back fields
	TranNbr string
	BatchID string

	// Whether the request was approved
	IsApproved bool // Derived from AuthResp

	// Raw response for debugging
	RawXML string
}

// BRICStorageAdapter defines the port for BRIC Storage operations
// Implements EPX BRIC Storage API for creating long-term payment tokens
//
// Storage BRICs:
//   - Never expire (indefinite lifetime)
//   - Used for recurring payments and card-on-file
//   - One-time fee (billed 1 month in arrears by EPX business team)
//   - Credit cards: trigger Account Verification to card networks
//   - ACH: internal routing number validation only
type BRICStorageAdapter interface {
	// ConvertFinancialBRICToStorage converts a Financial BRIC to a Storage BRIC
	//
	// Use case: Customer completes a payment and wants to save their payment method
	//
	// For credit cards:
	//   - Sends TRAN_TYPE=CCE8 with ORIG_AUTH_GUID
	//   - EPX routes as $0.00 Account Verification (CCx0) to card networks
	//   - Issuer must approve for Storage BRIC creation
	//   - Returns Storage BRIC + Network Transaction ID (NTID)
	//
	// For ACH:
	//   - Sends TRAN_TYPE=CKC8 with ORIG_AUTH_GUID
	//   - EPX performs routing number validation
	//   - Returns Storage BRIC
	//
	// Important: When updating Storage BRIC, keep using the original BRIC.
	// The new BRIC returned from update transactions cannot be used.
	ConvertFinancialBRICToStorage(ctx context.Context, req *BRICStorageRequest) (*BRICStorageResponse, error)

	// CreateStorageBRICFromAccount creates a Storage BRIC from account information
	//
	// Use case: Customer adds a payment method without making a purchase
	//
	// For credit cards:
	//   - Sends TRAN_TYPE=CCE8 with ACCOUNT_NBR, EXP_DATE, CVV2
	//   - EPX routes as $0.00 Account Verification to card networks
	//   - Validates card details with issuer
	//   - Returns Storage BRIC + Network Transaction ID
	//
	// For ACH:
	//   - Sends TRAN_TYPE=CKC8 with ACCOUNT_NBR, ROUTING_NBR
	//   - EPX validates routing number
	//   - Returns Storage BRIC
	CreateStorageBRICFromAccount(ctx context.Context, req *BRICStorageRequest) (*BRICStorageResponse, error)

	// UpdateStorageBRIC updates reference data for an existing Storage BRIC
	//
	// Important: The original Storage BRIC must continue to be used.
	// The new BRIC returned from this transaction CANNOT be used for payments.
	//
	// Non-PCI updates (do NOT trigger Account Verification):
	//   - CITY, FIRST_NAME, LAST_NAME, SOFT_DESCRIPTOR, STATE, USER_DATA_1-10
	//
	// PCI updates (trigger Account Verification for credit cards):
	//   - ACCOUNT_NBR, EXP_DATE, ADDRESS, ZIP_CODE
	UpdateStorageBRIC(ctx context.Context, req *BRICStorageRequest) (*BRICStorageResponse, error)
}
