package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// SavePaymentMethodRequest contains parameters for saving a payment method
type SavePaymentMethodRequest struct {
	AgentID        string
	CustomerID     string
	PaymentToken   string // EPX token (AUTH_GUID)
	PaymentType    domain.PaymentMethodType
	LastFour       string
	CardBrand      *string
	CardExpMonth   *int
	CardExpYear    *int
	BankName       *string
	AccountType    *string
	IsDefault      bool
	IdempotencyKey *string
}

// ConvertFinancialBRICRequest contains parameters for converting Financial BRIC to Storage BRIC
type ConvertFinancialBRICRequest struct {
	AgentID        string
	CustomerID     string
	FinancialBRIC  string                   // AUTH_GUID from completed transaction
	PaymentType    domain.PaymentMethodType // credit_card or ach
	TransactionID  string                   // Reference to the original transaction
	LastFour       string                   // For display purposes
	CardBrand      *string                  // For credit cards
	CardExpMonth   *int                     // For credit cards
	CardExpYear    *int                     // For credit cards
	BankName       *string                  // For ACH
	AccountType    *string                  // For ACH (checking/savings)
	IsDefault      bool
	IdempotencyKey *string

	// Billing information (required for Account Verification on credit cards)
	FirstName *string
	LastName  *string
	Address   *string
	City      *string
	State     *string
	ZipCode   *string
}

// VerifyACHAccountRequest contains parameters for ACH verification
type VerifyACHAccountRequest struct {
	PaymentMethodID string
	AgentID         string
	CustomerID      string
}

// PaymentMethodService defines the port for payment method operations
type PaymentMethodService interface {
	// SavePaymentMethod tokenizes and saves a payment method
	SavePaymentMethod(ctx context.Context, req *SavePaymentMethodRequest) (*domain.PaymentMethod, error)

	// ConvertFinancialBRICToStorageBRIC converts a Financial BRIC to a Storage BRIC and saves it
	//
	// Use case: Customer completes a payment and wants to save their payment method
	//
	// Process:
	//   1. Calls EPX BRIC Storage API to convert Financial BRIC to Storage BRIC
	//   2. For credit cards: EPX performs $0.00 Account Verification with card networks
	//   3. For ACH: EPX validates routing number
	//   4. If approved: saves Storage BRIC to customer_payment_methods table
	//   5. Returns saved payment method
	//
	// Important: Storage BRICs never expire and are used for recurring payments
	ConvertFinancialBRICToStorageBRIC(ctx context.Context, req *ConvertFinancialBRICRequest) (*domain.PaymentMethod, error)

	// GetPaymentMethod retrieves a specific payment method
	GetPaymentMethod(ctx context.Context, paymentMethodID string) (*domain.PaymentMethod, error)

	// ListPaymentMethods lists all payment methods for a customer
	ListPaymentMethods(ctx context.Context, agentID, customerID string) ([]*domain.PaymentMethod, error)

	// UpdatePaymentMethodStatus updates the active status of a payment method
	UpdatePaymentMethodStatus(ctx context.Context, paymentMethodID, agentID, customerID string, isActive bool) (*domain.PaymentMethod, error)

	// DeletePaymentMethod soft deletes a payment method (sets deleted_at)
	DeletePaymentMethod(ctx context.Context, paymentMethodID string) error

	// SetDefaultPaymentMethod marks a payment method as default
	SetDefaultPaymentMethod(ctx context.Context, paymentMethodID, agentID, customerID string) (*domain.PaymentMethod, error)

	// VerifyACHAccount sends pre-note for ACH verification
	VerifyACHAccount(ctx context.Context, req *VerifyACHAccountRequest) error
}
