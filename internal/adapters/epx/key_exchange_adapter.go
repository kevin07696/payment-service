package epx

import (
	"context"
	"crypto/tls"
	"encoding/xml"
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
	// Sandbox: https://keyexch.epxuap.com
	// Production: https://epxnow.com/epx/key_exchange (or contact North for production URL)
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
	baseURL := "https://epxnow.com/epx/key_exchange" // Production (may need to contact North for production URL)
	if environment == "sandbox" {
		baseURL = "https://keyexch.epxuap.com"
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

	formDataEncoded := formData.Encode()
	a.logger.Info("Requesting TAC from EPX Key Exchange",
		zap.String("merchant_id", req.MerchantID),
		zap.String("tran_nbr", req.TranNbr),
		zap.String("amount", req.Amount),
		zap.String("url", a.config.BaseURL),
		zap.String("cust_nbr", req.CustNbr),
		zap.String("merch_nbr", req.MerchNbr),
		zap.String("dba_nbr", req.DBAnbr),
		zap.String("terminal_nbr", req.TerminalNbr),
		zap.Int("form_data_len", len(formDataEncoded)),
		zap.String("form_data", formDataEncoded), // Log full form data
	)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL, strings.NewReader(formDataEncoded))
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
		zap.String("merchant_id", req.MerchantID),
		zap.String("tran_nbr", response.TranNbr),
		zap.Time("expires_at", response.ExpiresAt),
	)

	return response, nil
}

// validateRequest validates the Key Exchange request parameters
// Per EPX Browser Post API documentation (page 3), only these fields are required for Key Exchange:
// - TRAN_NBR, AMOUNT, MAC, TRAN_GROUP, REDIRECT_URL
// Merchant credentials (CUST_NBR, MERCH_NBR, etc.) are embedded in the MAC, not sent separately
func (a *keyExchangeAdapter) validateRequest(req *ports.KeyExchangeRequest) error {
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
// Based on EPX Browser Post API - Key Exchange Required Fields (page 3)
// NOTE: Per EPX documentation example, Key Exchange request should ONLY include:
// - TRAN_NBR, AMOUNT, MAC, TRAN_GROUP, REDIRECT_URL
// Merchant credentials (CUST_NBR, MERCH_NBR, etc.) are NOT sent in Key Exchange,
// they are embedded in the MAC (Merchant Authorization Code)
func (a *keyExchangeAdapter) buildFormData(req *ports.KeyExchangeRequest) url.Values {
	data := url.Values{}

	// TRAN_GROUP values per EPX Data Dictionary:
	// - "SALE" for sale transactions (auth + capture)
	// - "AUTH" for authorization-only transactions
	// - "STORAGE" for card storage/tokenization (Browser Post uses TRAN_CODE=STORAGE, TRAN_TYPE=CCX8)
	// NOTE: Do NOT use single-letter codes (U/A) - those are for TRAN_CODE, not TRAN_GROUP
	tranGroup := req.TranGroup
	if tranGroup != "SALE" && tranGroup != "AUTH" && tranGroup != "STORAGE" {
		// If not already in correct format, normalize it
		if req.TranGroup == "U" {
			tranGroup = "SALE"
		} else if req.TranGroup == "A" {
			tranGroup = "AUTH"
		}
		// STORAGE is a valid TRAN_GROUP, pass through as-is
	}

	// Required fields per EPX Browser Post documentation (page 3)
	data.Set("TRAN_NBR", req.TranNbr)
	data.Set("AMOUNT", req.Amount)
	data.Set("MAC", req.MAC)
	data.Set("TRAN_GROUP", tranGroup)
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

// keyExchangeResponse represents the XML structure of EPX Key Exchange response
type keyExchangeResponse struct {
	XMLName xml.Name          `xml:"RESPONSE"`
	Fields  keyExchangeFields `xml:"FIELDS"`
}

type keyExchangeFields struct {
	Fields []keyExchangeField `xml:"FIELD"`
}

type keyExchangeField struct {
	Key   string `xml:"KEY,attr"`
	Value string `xml:",chardata"`
}

// parseResponse parses the EPX Key Exchange response
// EPX returns the TAC token in XML format:
// <RESPONSE><FIELDS><FIELD KEY="TAC">token_value</FIELD></FIELDS></RESPONSE>
func (a *keyExchangeAdapter) parseResponse(body []byte, req *ports.KeyExchangeRequest) (*ports.KeyExchangeResponse, error) {
	responseStr := strings.TrimSpace(string(body))

	var tac string

	// Try to parse as XML first (EPX standard format)
	if strings.HasPrefix(responseStr, "<RESPONSE>") {
		var xmlResp keyExchangeResponse
		if err := xml.Unmarshal(body, &xmlResp); err != nil {
			return nil, fmt.Errorf("failed to parse XML response: %w", err)
		}

		// Extract TAC from fields
		for _, field := range xmlResp.Fields.Fields {
			if field.Key == "TAC" {
				tac = field.Value
				break
			}
		}

		if tac == "" {
			return nil, fmt.Errorf("TAC not found in XML response")
		}
	} else if strings.Contains(responseStr, "=") {
		// Fallback: Try key=value format
		params, err := url.ParseQuery(responseStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		tac = params.Get("TAC")
		if tac == "" {
			return nil, fmt.Errorf("TAC not found in response")
		}
	} else {
		// Fallback: Assume response is the TAC itself
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
