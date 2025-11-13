package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// RegisterMerchantRequest contains parameters for registering a merchant
type RegisterMerchantRequest struct {
	AgentID        string
	MACSecret      string
	CustNbr        string
	MerchNbr       string
	DBAnbr         string
	TerminalNbr    string
	Environment    domain.Environment
	MerchantName   string
	IdempotencyKey *string
}

// UpdateMerchantRequest contains parameters for updating a merchant
type UpdateMerchantRequest struct {
	AgentID        string
	MACSecret      *string // Optional: rotate MAC secret
	CustNbr        *string
	MerchNbr       *string
	DBAnbr         *string
	TerminalNbr    *string
	Environment    *domain.Environment
	MerchantName   *string
	IdempotencyKey *string
}

// RotateMerchantMACRequest contains parameters for rotating MAC secret
type RotateMerchantMACRequest struct {
	AgentID      string
	NewMACSecret string
}

// MerchantService defines the port for merchant credential management
type MerchantService interface {
	// RegisterMerchant adds a new merchant to the system
	RegisterMerchant(ctx context.Context, req *RegisterMerchantRequest) (*domain.Merchant, error)

	// GetMerchant retrieves merchant credentials (internal use only)
	GetMerchant(ctx context.Context, agentID string) (*domain.Merchant, error)

	// ListMerchants lists all registered merchants
	ListMerchants(ctx context.Context, environment *domain.Environment, isActive *bool, limit, offset int) ([]*domain.Merchant, int, error)

	// UpdateMerchant updates merchant credentials
	UpdateMerchant(ctx context.Context, req *UpdateMerchantRequest) (*domain.Merchant, error)

	// DeactivateMerchant deactivates a merchant
	DeactivateMerchant(ctx context.Context, agentID, reason string) error

	// RotateMerchantMAC rotates MAC secret in secret manager
	RotateMerchantMAC(ctx context.Context, req *RotateMerchantMACRequest) error
}
