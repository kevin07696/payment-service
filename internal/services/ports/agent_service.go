package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain"
)

// RegisterAgentRequest contains parameters for registering an agent
type RegisterAgentRequest struct {
	AgentID        string
	MACSecret      string
	CustNbr        string
	MerchNbr       string
	DBAnbr         string
	TerminalNbr    string
	Environment    domain.Environment
	AgentName      string
	IdempotencyKey *string
}

// UpdateAgentRequest contains parameters for updating an agent
type UpdateAgentRequest struct {
	AgentID        string
	MACSecret      *string // Optional: rotate MAC secret
	CustNbr        *string
	MerchNbr       *string
	DBAnbr         *string
	TerminalNbr    *string
	Environment    *domain.Environment
	AgentName      *string
	IdempotencyKey *string
}

// RotateMACRequest contains parameters for rotating MAC secret
type RotateMACRequest struct {
	AgentID      string
	NewMACSecret string
}

// AgentService defines the port for agent/merchant credential management
type AgentService interface {
	// RegisterAgent adds a new agent/merchant to the system
	RegisterAgent(ctx context.Context, req *RegisterAgentRequest) (*domain.Agent, error)

	// GetAgent retrieves agent credentials (internal use only)
	GetAgent(ctx context.Context, agentID string) (*domain.Agent, error)

	// ListAgents lists all registered agents
	ListAgents(ctx context.Context, environment *domain.Environment, isActive *bool, limit, offset int) ([]*domain.Agent, int, error)

	// UpdateAgent updates agent credentials
	UpdateAgent(ctx context.Context, req *UpdateAgentRequest) (*domain.Agent, error)

	// DeactivateAgent deactivates an agent
	DeactivateAgent(ctx context.Context, agentID, reason string) error

	// RotateMAC rotates MAC secret in secret manager
	RotateMAC(ctx context.Context, req *RotateMACRequest) error
}
