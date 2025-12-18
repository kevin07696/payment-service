package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// RegisterMerchantRequest contains parameters for registering a merchant.
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

// UpdateMerchantRequest contains parameters for updating a merchant.
type UpdateMerchantRequest struct {
	AgentID        string
	MACSecret      *string
	CustNbr        *string
	MerchNbr       *string
	DBAnbr         *string
	TerminalNbr    *string
	Environment    *domain.Environment
	MerchantName   *string
	IdempotencyKey *string
}

// RotateMerchantMACRequest contains parameters for rotating MAC secret.
type RotateMerchantMACRequest struct {
	AgentID      string
	NewMACSecret string
}

// MerchantService defines the port for merchant credential management.
type MerchantService interface {
	RegisterMerchant(ctx context.Context, req *RegisterMerchantRequest) (*domain.Merchant, error)
	GetMerchant(ctx context.Context, agentID string) (*domain.Merchant, error)
	ListMerchants(ctx context.Context, environment *domain.Environment, isActive *bool, limit, offset int) ([]*domain.Merchant, int, error)
	UpdateMerchant(ctx context.Context, req *UpdateMerchantRequest) (*domain.Merchant, error)
	DeactivateMerchant(ctx context.Context, agentID, reason string) error
	RotateMerchantMAC(ctx context.Context, req *RotateMerchantMACRequest) error
}
