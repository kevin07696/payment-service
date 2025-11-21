package domain

import (
	"time"

	"github.com/kevin07696/payment-service/pkg/timeutil"
)

// Environment represents the EPX environment
type Environment string

const (
	EnvironmentSandbox    Environment = "sandbox"
	EnvironmentProduction Environment = "production"
)

// Merchant represents a merchant in the multi-tenant system
// Merchant credentials are stored securely with MAC secrets in a secret manager
type Merchant struct {
	// Identity
	ID      string `json:"id"`       // UUID (internal)
	AgentID string `json:"agent_id"` // Unique merchant identifier (external-facing)

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

// IsSandbox returns true if this merchant is using sandbox environment
func (m *Merchant) IsSandbox() bool {
	return m.Environment == EnvironmentSandbox
}

// IsProduction returns true if this merchant is using production environment
func (m *Merchant) IsProduction() bool {
	return m.Environment == EnvironmentProduction
}

// CanProcessTransactions returns true if the merchant can process transactions
func (m *Merchant) CanProcessTransactions() bool {
	return m.IsActive
}

// GetMACSecretPath returns the path to the MAC secret in the secret manager
// Format: "payment-service/merchants/{merchant_id}/mac"
func (m *Merchant) GetMACSecretPath() string {
	return m.MACSecretPath
}

// Deactivate marks the merchant as inactive
func (m *Merchant) Deactivate() {
	m.IsActive = false
	m.UpdatedAt = timeutil.Now()
}

// Activate marks the merchant as active
func (m *Merchant) Activate() {
	m.IsActive = true
	m.UpdatedAt = timeutil.Now()
}
