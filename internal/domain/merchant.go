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
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	Metadata      map[string]interface{} `json:"metadata"`
	ID            string                 `json:"id"`
	AgentID       string                 `json:"agent_id"`
	CustNbr       string                 `json:"cust_nbr"`
	MerchNbr      string                 `json:"merch_nbr"`
	DBAnbr        string                 `json:"dba_nbr"`
	TerminalNbr   string                 `json:"terminal_nbr"`
	MACSecretPath string                 `json:"mac_secret_path"`
	Environment   Environment            `json:"environment"`
	IsActive      bool                   `json:"is_active"`
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
