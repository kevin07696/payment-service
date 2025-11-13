package epx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// BrowserPostConfig contains configuration for EPX Browser Post adapter
type BrowserPostConfig struct {
	// EPX Browser Post endpoint URL
	// Sandbox: https://services.epxuap.com/browserpost/
	// Production: https://epxnow.com/epx/browser_post (or contact North for production URL)
	PostURL string

	// Default merchant name for display
	MerchantName string

	// Whether to validate MAC signatures in responses
	ValidateMAC bool
}

// DefaultBrowserPostConfig returns default configuration for Browser Post adapter
func DefaultBrowserPostConfig(environment string) *BrowserPostConfig {
	postURL := "https://epxnow.com/epx/browser_post" // Production (may need to contact North for production URL)
	if environment == "sandbox" {
		postURL = "https://services.epxuap.com/browserpost/"
	}

	return &BrowserPostConfig{
		PostURL:      postURL,
		MerchantName: "Payment Service",
		ValidateMAC:  true,
	}
}

// browserPostAdapter implements the BrowserPostAdapter port
type browserPostAdapter struct {
	config *BrowserPostConfig
	logger *zap.Logger
}

// NewBrowserPostAdapter creates a new EPX Browser Post adapter
func NewBrowserPostAdapter(config *BrowserPostConfig, logger *zap.Logger) ports.BrowserPostAdapter {
	return &browserPostAdapter{
		config: config,
		logger: logger,
	}
}

// BuildFormData constructs the form data structure needed for Browser Post
// Based on EPX Browser Post API - Required Fields (page 8-13)
func (a *browserPostAdapter) BuildFormData(tac, amount, tranNbr, tranGroup, redirectURL string) (*ports.BrowserPostFormData, error) {
	// Validate required fields
	if tac == "" {
		return nil, fmt.Errorf("tac is required")
	}
	if amount == "" {
		return nil, fmt.Errorf("amount is required")
	}
	if tranNbr == "" {
		return nil, fmt.Errorf("tran_nbr is required")
	}
	if tranGroup == "" {
		return nil, fmt.Errorf("tran_group is required")
	}
	if redirectURL == "" {
		return nil, fmt.Errorf("redirect_url is required")
	}

	// Validate amount format
	if _, err := strconv.ParseFloat(amount, 64); err != nil {
		return nil, fmt.Errorf("amount must be numeric: %w", err)
	}

	a.logger.Info("Building Browser Post form data",
		zap.String("tran_nbr", tranNbr),
		zap.String("amount", amount),
	)

	return &ports.BrowserPostFormData{
		PostURL:     a.config.PostURL,
		TAC:         tac,
		Amount:      amount,
		TranNbr:     tranNbr,
		TranGroup:   tranGroup,
		RedirectURL: redirectURL,
		// Optional redirect URLs for decline/error (can be same as success URL)
		RedirectURLDecline: redirectURL,
		RedirectURLError:   redirectURL,
		MerchantName:       a.config.MerchantName,
		Metadata:           make(map[string]string),
	}, nil
}

// ParseRedirectResponse parses the query parameters from EPX redirect
// Based on EPX Browser Post API - Response Fields (page 14-17)
func (a *browserPostAdapter) ParseRedirectResponse(params map[string][]string) (*ports.BrowserPostResponse, error) {
	// Helper to get first value from slice
	getValue := func(key string) string {
		if values, ok := params[key]; ok && len(values) > 0 {
			return values[0]
		}
		return ""
	}

	// Extract required fields
	authGUID := getValue("AUTH_GUID")
	authResp := getValue("AUTH_RESP")
	authRespText := getValue("AUTH_RESP_TEXT")

	a.logger.Info("Parsing Browser Post redirect response",
		zap.String("auth_guid", authGUID),
		zap.String("auth_resp", authResp),
	)

	// Validate required fields
	if authGUID == "" {
		return nil, fmt.Errorf("AUTH_GUID is missing from response")
	}
	if authResp == "" {
		return nil, fmt.Errorf("AUTH_RESP is missing from response")
	}

	// Determine if transaction is approved
	// "00" = approved, anything else is declined or error
	isApproved := authResp == "00"

	// Parse timestamp if present
	processedAt := time.Now()
	if timestampStr := getValue("TIMESTAMP"); timestampStr != "" {
		if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
			processedAt = t
		}
	}

	// Convert params map to simple string map for raw params
	rawParams := make(map[string]string)
	for key, values := range params {
		if len(values) > 0 {
			rawParams[key] = values[0]
		}
	}

	response := &ports.BrowserPostResponse{
		AuthGUID:     authGUID,
		AuthResp:     authResp,
		AuthCode:     getValue("AUTH_CODE"),
		AuthRespText: authRespText,
		IsApproved:   isApproved,
		AuthCardType: getValue("AUTH_CARD_TYPE"),
		AuthAVS:      getValue("AUTH_AVS"),
		AuthCVV2:     getValue("AUTH_CVV2"),
		TranNbr:      getValue("TRAN_NBR"),
		TranGroup:    getValue("TRAN_GROUP"),
		Amount:       getValue("AMOUNT"), // EPX returns amount in AMOUNT field
		ProcessedAt:  processedAt,
		RawParams:    rawParams,
	}

	a.logger.Info("Parsed Browser Post response",
		zap.String("auth_guid", response.AuthGUID),
		zap.String("auth_resp", response.AuthResp),
		zap.Bool("is_approved", response.IsApproved),
		zap.String("tran_nbr", response.TranNbr),
	)

	return response, nil
}

// ValidateResponseMAC validates the MAC signature in the redirect response
// Ensures the response hasn't been tampered with
// Based on EPX Browser Post API - Response Validation (page 17-18)
func (a *browserPostAdapter) ValidateResponseMAC(params map[string][]string, mac string) error {
	if !a.config.ValidateMAC {
		a.logger.Warn("MAC validation is disabled")
		return nil
	}

	// Helper to get first value from slice
	getValue := func(key string) string {
		if values, ok := params[key]; ok && len(values) > 0 {
			return values[0]
		}
		return ""
	}

	// Get the MAC from response
	responseMAC := getValue("MAC")
	if responseMAC == "" {
		return fmt.Errorf("MAC is missing from response")
	}

	// Build signature string from response parameters
	// EPX typically signs specific fields in a deterministic order
	// Example: CUST_NBR + MERCH_NBR + AUTH_GUID + AUTH_RESP + AMOUNT + TRAN_NBR
	signatureFields := []string{
		getValue("CUST_NBR"),
		getValue("MERCH_NBR"),
		getValue("AUTH_GUID"),
		getValue("AUTH_RESP"),
		getValue("AMOUNT"),
		getValue("TRAN_NBR"),
		getValue("TRAN_GROUP"),
	}

	// Build signature string
	signatureStr := strings.Join(signatureFields, "")

	// Compute HMAC-SHA256
	expectedMAC := computeHMAC(signatureStr, mac)

	// Compare MACs (constant-time comparison)
	if !hmac.Equal([]byte(expectedMAC), []byte(responseMAC)) {
		a.logger.Error("MAC validation failed",
			zap.String("expected", expectedMAC),
			zap.String("received", responseMAC),
		)
		return fmt.Errorf("MAC validation failed: signature mismatch")
	}

	a.logger.Info("MAC validation successful")
	return nil
}

// computeHMAC computes HMAC-SHA256 signature
func computeHMAC(message, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// sortedKeys returns sorted keys from a map (for deterministic signing)
func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// buildSignatureString builds a canonical signature string from parameters
// Used for MAC validation - must match EPX's signing algorithm
func buildSignatureString(params map[string][]string, fieldsToSign []string) string {
	var parts []string
	for _, field := range fieldsToSign {
		if values, ok := params[field]; ok && len(values) > 0 {
			parts = append(parts, values[0])
		}
	}
	return strings.Join(parts, "")
}

// ValidateRedirectURL validates that the redirect URL matches expected format
// Prevents open redirect vulnerabilities
func (a *browserPostAdapter) ValidateRedirectURL(redirectURL string, allowedDomains []string) error {
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		return fmt.Errorf("invalid redirect URL: %w", err)
	}

	// Must be HTTPS in production
	if parsed.Scheme != "https" && a.config.PostURL != "" && !strings.Contains(a.config.PostURL, "sandbox") {
		return fmt.Errorf("redirect URL must use HTTPS in production")
	}

	// Check against allowed domains
	if len(allowedDomains) > 0 {
		allowed := false
		for _, domain := range allowedDomains {
			if parsed.Host == domain || strings.HasSuffix(parsed.Host, "."+domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("redirect URL domain %s is not in allowed list", parsed.Host)
		}
	}

	return nil
}
