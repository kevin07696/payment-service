package epx

import (
	"fmt"
	"os"
	"time"

	"github.com/kevin07696/payment-service/internal/ports"
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
}

// DefaultBrowserPostConfig returns default configuration for Browser Post adapter
// Production endpoint must be set via EPX_BROWSER_POST_ENDPOINT environment variable
func DefaultBrowserPostConfig(environment string) (*BrowserPostConfig, error) {
	var postURL string

	if environment == "sandbox" {
		postURL = "https://services.epxuap.com/browserpost/"
	} else {
		postURL = os.Getenv("EPX_BROWSER_POST_ENDPOINT")
		if postURL == "" {
			return nil, fmt.Errorf("EPX_BROWSER_POST_ENDPOINT environment variable is required for %s environment", environment)
		}
	}

	return &BrowserPostConfig{
		PostURL:      postURL,
		MerchantName: "Payment Service",
	}, nil
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

// ParseRedirectResponse parses the query parameters from EPX redirect
// Based on EPX Browser Post API Integration Guide - Response Fields (page 7)
func (a *browserPostAdapter) ParseRedirectResponse(params map[string][]string) (*ports.BrowserPostResponse, error) {
	// Helper to get first value from slice
	getValue := func(key string) string {
		if values, ok := params[key]; ok && len(values) > 0 {
			return values[0]
		}
		return ""
	}

	// Extract required fields per North Developer Browser Post API Guide
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

	// Build response with EPX field names per North Developer Browser Post API Guide (page 7)
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
		Amount:       getValue("AUTH_AMOUNT"), // EPX returns amount in AUTH_AMOUNT field (page 7: AMOUNT=100.00)
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
