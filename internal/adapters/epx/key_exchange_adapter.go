package epx

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// KeyExchangeConfig contains configuration for EPX Key Exchange adapter
type KeyExchangeConfig struct {
	// Base URL for EPX Key Exchange service
	// Sandbox: https://epxnow.com/epx/key_exchange_sandbox
	// Production: https://epxnow.com/epx/key_exchange
	BaseURL string

	// HTTP client timeout
	Timeout time.Duration

	// TLS configuration (production should verify certificates)
	InsecureSkipVerify bool

	// TAC expiration duration (default: 4 hours per EPX documentation)
	TACExpiration time.Duration
}

// DefaultKeyExchangeConfig returns default configuration for Key Exchange adapter
func DefaultKeyExchangeConfig(environment string) *KeyExchangeConfig {
	baseURL := "https://epxnow.com/epx/key_exchange" // Production
	if environment == "sandbox" {
		baseURL = "https://epxnow.com/epx/key_exchange_sandbox"
	}

	return &KeyExchangeConfig{
		BaseURL:            baseURL,
		Timeout:            30 * time.Second,
		InsecureSkipVerify: environment == "sandbox", // Only skip verification in sandbox
		TACExpiration:      4 * time.Hour,            // EPX TAC expires in 4 hours
	}
}

// keyExchangeAdapter implements the KeyExchangeAdapter port
type keyExchangeAdapter struct {
	config     *KeyExchangeConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewKeyExchangeAdapter creates a new EPX Key Exchange adapter
func NewKeyExchangeAdapter(config *KeyExchangeConfig, logger *zap.Logger) ports.KeyExchangeAdapter {
	// Configure HTTP client with timeout and TLS settings
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	httpClient := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	return &keyExchangeAdapter{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
	}
}

// GetTAC requests a Terminal Authorization Code from EPX Key Exchange service
// Based on EPX Browser Post API documentation - Key Exchange Request (page 6)
func (a *keyExchangeAdapter) GetTAC(ctx context.Context, req *ports.KeyExchangeRequest) (*ports.KeyExchangeResponse, error) {
	// Validate required fields
	if err := a.validateRequest(req); err != nil {
		a.logger.Error("Invalid Key Exchange request", zap.Error(err))
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Build form data for EPX Key Exchange
	formData := a.buildFormData(req)

	a.logger.Info("Requesting TAC from EPX Key Exchange",
		zap.String("agent_id", req.AgentID),
		zap.String("tran_nbr", req.TranNbr),
		zap.String("amount", req.Amount),
	)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL, strings.NewReader(formData.Encode()))
	if err != nil {
		a.logger.Error("Failed to create HTTP request", zap.Error(err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request to EPX
	startTime := time.Now()
	httpResp, err := a.httpClient.Do(httpReq)
	if err != nil {
		a.logger.Error("Failed to send Key Exchange request",
			zap.Error(err),
			zap.Duration("elapsed", time.Since(startTime)),
		)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		a.logger.Error("Failed to read response body", zap.Error(err))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	a.logger.Info("Received Key Exchange response",
		zap.Int("status_code", httpResp.StatusCode),
		zap.Duration("elapsed", time.Since(startTime)),
		zap.Int("body_length", len(body)),
	)

	// Check HTTP status code
	if httpResp.StatusCode != http.StatusOK {
		a.logger.Error("EPX Key Exchange returned error",
			zap.Int("status_code", httpResp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("EPX returned status %d: %s", httpResp.StatusCode, string(body))
	}

	// Parse response
	response, err := a.parseResponse(body, req)
	if err != nil {
		a.logger.Error("Failed to parse Key Exchange response",
			zap.Error(err),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	a.logger.Info("Successfully obtained TAC",
		zap.String("agent_id", req.AgentID),
		zap.String("tran_nbr", response.TranNbr),
		zap.Time("expires_at", response.ExpiresAt),
	)

	return response, nil
}

// validateRequest validates the Key Exchange request parameters
func (a *keyExchangeAdapter) validateRequest(req *ports.KeyExchangeRequest) error {
	if req.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if req.CustNbr == "" {
		return fmt.Errorf("cust_nbr is required")
	}
	if req.MerchNbr == "" {
		return fmt.Errorf("merch_nbr is required")
	}
	if req.DBAnbr == "" {
		return fmt.Errorf("dba_nbr is required")
	}
	if req.TerminalNbr == "" {
		return fmt.Errorf("terminal_nbr is required")
	}
	if req.MAC == "" {
		return fmt.Errorf("mac is required")
	}
	if req.Amount == "" {
		return fmt.Errorf("amount is required")
	}
	if req.TranNbr == "" {
		return fmt.Errorf("tran_nbr is required")
	}
	if req.TranGroup == "" {
		return fmt.Errorf("tran_group is required")
	}
	if req.RedirectURL == "" {
		return fmt.Errorf("redirect_url is required")
	}

	// Validate amount format (must be numeric)
	if _, err := strconv.ParseFloat(req.Amount, 64); err != nil {
		return fmt.Errorf("amount must be numeric: %w", err)
	}

	return nil
}

// buildFormData constructs URL-encoded form data for EPX Key Exchange request
// Based on EPX Browser Post API - Key Exchange Required Fields (page 6)
func (a *keyExchangeAdapter) buildFormData(req *ports.KeyExchangeRequest) url.Values {
	data := url.Values{}

	// Required EPX credentials
	data.Set("CUST_NBR", req.CustNbr)
	data.Set("MERCH_NBR", req.MerchNbr)
	data.Set("DBA_NBR", req.DBAnbr)
	data.Set("TERMINAL_NBR", req.TerminalNbr)
	data.Set("MAC", req.MAC)

	// Transaction details
	data.Set("AMOUNT", req.Amount)
	data.Set("TRAN_NBR", req.TranNbr)
	data.Set("TRAN_GROUP", req.TranGroup)
	data.Set("REDIRECT_URL", req.RedirectURL)

	// Optional fields
	if req.CustomerID != "" {
		data.Set("CUSTOMER_ID", req.CustomerID)
	}

	// Add metadata as custom fields (if EPX supports them)
	for key, value := range req.Metadata {
		data.Set(key, value)
	}

	return data
}

// parseResponse parses the EPX Key Exchange response
// EPX returns the TAC token in the response body (format depends on EPX implementation)
func (a *keyExchangeAdapter) parseResponse(body []byte, req *ports.KeyExchangeRequest) (*ports.KeyExchangeResponse, error) {
	responseStr := strings.TrimSpace(string(body))

	// EPX typically returns the TAC token as plain text or in a simple format
	// Example response: "TAC=abc123xyz..." or just "abc123xyz..."
	// Parse based on actual EPX response format

	var tac string

	// Check if response is in key=value format
	if strings.Contains(responseStr, "=") {
		params, err := url.ParseQuery(responseStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		tac = params.Get("TAC")
		if tac == "" {
			return nil, fmt.Errorf("TAC not found in response")
		}
	} else {
		// Assume response is the TAC itself
		tac = responseStr
	}

	if tac == "" {
		return nil, fmt.Errorf("empty TAC received")
	}

	// Calculate expiration time (TAC expires in 4 hours per EPX documentation)
	expiresAt := time.Now().Add(a.config.TACExpiration)

	return &ports.KeyExchangeResponse{
		TAC:       tac,
		ExpiresAt: expiresAt,
		TranNbr:   req.TranNbr,
		TranGroup: req.TranGroup,
	}, nil
}
