package ports

import (
	"context"
	"time"
)

// KeyExchangeRequest contains parameters for requesting a TAC token from EPX Key Exchange service
// Based on EPX Browser Post API - Key Exchange Request (page 6)
type KeyExchangeRequest struct {
	// Agent credentials
	AgentID     string // Our internal agent ID
	CustNbr     string // EPX customer number
	MerchNbr    string // EPX merchant number
	DBAnbr      string // EPX DBA number
	TerminalNbr string // EPX terminal number
	MAC         string // Message Authentication Code from secret manager

	// Transaction details
	Amount      string // Transaction amount (e.g., "29.99")
	TranNbr     string // Unique transaction number
	TranGroup   string // Transaction group ID (our group_id)
	RedirectURL string // URL where EPX will redirect after payment

	// Optional fields
	CustomerID string            // Our internal customer ID (optional)
	Metadata   map[string]string // Additional metadata (optional)
}

// KeyExchangeResponse contains the TAC token and metadata from EPX
type KeyExchangeResponse struct {
	TAC       string    // Terminal Authorization Code (encrypted token, expires in 4 hours)
	ExpiresAt time.Time // TAC expiration timestamp
	TranNbr   string    // Echo back transaction number
	TranGroup string    // Echo back transaction group
}

// KeyExchangeAdapter defines the port for requesting TAC tokens from EPX Key Exchange service
// TAC tokens are required for Browser Post flow to ensure transaction integrity
// Implementation should handle:
//   - HTTPS communication with EPX Key Exchange endpoint
//   - Request signing with MAC
//   - Response parsing and validation
//   - Error handling for network failures, invalid credentials, etc.
type KeyExchangeAdapter interface {
	// GetTAC requests a Terminal Authorization Code from EPX Key Exchange service
	// TAC is valid for 4 hours and must be used in Browser Post form
	// Returns error if:
	//   - Network communication fails
	//   - MAC authentication fails (invalid credentials)
	//   - EPX service is unavailable
	//   - Request parameters are invalid
	GetTAC(ctx context.Context, req *KeyExchangeRequest) (*KeyExchangeResponse, error)
}
