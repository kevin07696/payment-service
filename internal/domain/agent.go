package domain

import (
	"time"
)

// Environment represents the EPX environment
type Environment string

const (
	EnvironmentSandbox    Environment = "sandbox"
	EnvironmentProduction Environment = "production"
)

// Agent represents a merchant/agent in the multi-tenant system
// Agent credentials are stored securely with MAC secrets in a secret manager
type Agent struct {
	// Identity
	ID      string `json:"id"`       // UUID (internal)
	AgentID string `json:"agent_id"` // Unique agent identifier (external-facing)

	// EPX credentials
	CustNbr     string `json:"cust_nbr"`     // EPX customer number
	MerchNbr    string `json:"merch_nbr"`    // EPX merchant number
	DBAnbr      string `json:"dba_nbr"`      // EPX DBA number
	TerminalNbr string `json:"terminal_nbr"` // EPX terminal number

	// Secret Manager reference (NEVER store actual MAC in database)
	MACSecretPath string `json:"mac_secret_path"` // Path to MAC secret in secret manager

	// Environment
	Environment Environment `json:"environment"` // sandbox or production

	// Status
	IsActive bool `json:"is_active"`

	// Additional metadata
	Metadata map[string]interface{} `json:"metadata"` // Business name, contact info, etc.

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsSandbox returns true if this agent is using sandbox environment
func (a *Agent) IsSandbox() bool {
	return a.Environment == EnvironmentSandbox
}

// IsProduction returns true if this agent is using production environment
func (a *Agent) IsProduction() bool {
	return a.Environment == EnvironmentProduction
}

// CanProcessTransactions returns true if the agent can process transactions
func (a *Agent) CanProcessTransactions() bool {
	return a.IsActive
}

// GetMACSecretPath returns the path to the MAC secret in the secret manager
// Format: "payment-service/agents/{agent_id}/mac"
func (a *Agent) GetMACSecretPath() string {
	return a.MACSecretPath
}

// Deactivate marks the agent as inactive
func (a *Agent) Deactivate() {
	a.IsActive = false
	a.UpdatedAt = time.Now()
}

// Activate marks the agent as active
func (a *Agent) Activate() {
	a.IsActive = true
	a.UpdatedAt = time.Now()
}