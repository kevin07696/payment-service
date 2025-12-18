package epx

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kevin07696/payment-service/internal/ports"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
// Production endpoint must be set via EPX_KEY_EXCHANGE_ENDPOINT environment variable
func DefaultKeyExchangeConfig(environment string) (*KeyExchangeConfig, error) {
	var baseURL string

	if environment == "sandbox" {
		baseURL = "https://keyexch.epxuap.com"
	} else {
		baseURL = os.Getenv("EPX_KEY_EXCHANGE_ENDPOINT")
		if baseURL == "" {
			return nil, fmt.Errorf("EPX_KEY_EXCHANGE_ENDPOINT environment variable is required for %s environment", environment)
		}
	}

	return &KeyExchangeConfig{
		BaseURL:            baseURL,
		Timeout:            30 * time.Second,
		InsecureSkipVerify: environment == "sandbox", // Only skip verification in sandbox
		TACExpiration:      4 * time.Hour,            // EPX TAC expires in 4 hours
	}, nil
}

// keyExchangeAdapter implements the KeyExchangeAdapter port
type keyExchangeAdapter struct {
	config     *KeyExchangeConfig
	httpClient *http.Client
	logger     *zap.Logger
	tracer     trace.Tracer
}

// NewKeyExchangeAdapter creates a new EPX Key Exchange adapter
func NewKeyExchangeAdapter(config *KeyExchangeConfig, logger *zap.Logger) ports.KeyExchangeAdapter {
	// SECURITY: Fail-safe guard against InsecureSkipVerify in production
	// Production EPX URLs use epxnow.com domain (not secure.epxuap.com sandbox)
	if config.InsecureSkipVerify {
		isProductionURL := strings.Contains(config.BaseURL, "epxnow.com") &&
			!strings.Contains(config.BaseURL, "sandbox")
		if isProductionURL {
			logger.Fatal("SECURITY VIOLATION: InsecureSkipVerify cannot be enabled for production EPX endpoints",
				zap.String("base_url", config.BaseURL),
			)
		}
		// Log clear warning for sandbox usage (audit trail)
		logger.Warn("TLS certificate verification disabled - SANDBOX MODE ONLY",
			zap.String("base_url", config.BaseURL),
			zap.Bool("insecure_skip_verify", true),
		)
	}

	// Configure HTTP client with HTTP/2, connection pooling, and TLS settings
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
		// HTTP/2 Configuration (P2-3 optimization)
		// Enables multiplexing, header compression, server push
		// Expected: 30% latency reduction vs HTTP/1.1
		ForceAttemptHTTP2: true,

		// Connection Pooling (already configured)
		// At 1000 TPS: reuses ~950 connections vs creating new ones
		// Saves ~50ms handshake per reused connection
		MaxIdleConns:        100,              // Total pool size across all hosts
		MaxIdleConnsPerHost: 100,              // Per-host pool (EPX is single host)
		IdleConnTimeout:     90 * time.Second, // Keep-alive duration
	}

	httpClient := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	return &keyExchangeAdapter{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
		tracer:     otel.Tracer(epxTracerName),
	}
}

// GetTAC requests a Terminal Authorization Code from EPX Key Exchange service
// Based on EPX Browser Post API documentation - Key Exchange Request (page 6)
func (a *keyExchangeAdapter) GetTAC(ctx context.Context, req *ports.KeyExchangeRequest) (*ports.KeyExchangeResponse, error) {
	// Start tracing span for key exchange call
	ctx, span := a.tracer.Start(ctx, "epx.key_exchange.get_tac",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gateway.name", "EPX"),
			attribute.String("gateway.endpoint", a.config.BaseURL),
			attribute.String("epx.tran_nbr", req.TranNbr),
			attribute.String("epx.tran_group", req.TranGroup),
			attribute.String("epx.amount", req.Amount),
		),
	)
	defer span.End()

	// Validate required fields
	if err := a.validateRequest(req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid request")
		a.logger.Error("Invalid Key Exchange request", zap.Error(err))
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Build form data for EPX Key Exchange
	formData := a.buildFormData(req)

	formDataEncoded := formData.Encode()
	a.logger.Info("EPX Key Exchange request",
		zap.String("url", a.config.BaseURL),
		zap.String("request_body", formDataEncoded),
	)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL, strings.NewReader(formDataEncoded))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		a.logger.Error("Failed to create HTTP request", zap.Error(err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request to EPX
	startTime := time.Now()
	httpResp, err := a.httpClient.Do(httpReq)
	duration := time.Since(startTime)
	span.SetAttributes(attribute.Int64("gateway.duration_ms", duration.Milliseconds()))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send request")
		a.logger.Error("Failed to send Key Exchange request",
			zap.Error(err),
			zap.Duration("elapsed", duration),
		)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", httpResp.StatusCode))

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to read response")
		a.logger.Error("Failed to read response body", zap.Error(err))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	a.logger.Info("Received Key Exchange response",
		zap.Int("status_code", httpResp.StatusCode),
		zap.Duration("elapsed", duration),
		zap.String("response_body", string(body)),
	)

	// Check HTTP status code
	if httpResp.StatusCode != http.StatusOK {
		err := fmt.Errorf("EPX returned status %d: %s", httpResp.StatusCode, string(body))
		span.RecordError(err)
		span.SetStatus(codes.Error, "EPX returned error status")
		a.logger.Error("EPX Key Exchange returned error",
			zap.Int("status_code", httpResp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, err
	}

	// Parse response
	response, err := a.parseResponse(body, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to parse response")
		a.logger.Error("Failed to parse Key Exchange response",
			zap.Error(err),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Record success attributes
	span.SetAttributes(attribute.String("epx.tac", response.TAC))
	span.SetStatus(codes.Ok, "TAC obtained successfully")

	a.logger.Info("Successfully obtained TAC",
		zap.String("merchant_id", req.MerchantID),
		zap.String("tran_nbr", response.TranNbr),
		zap.String("tac", response.TAC),
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
// Based on EPX Browser Post API certification sheet
// Key Exchange requires merchant credentials AND transaction parameters
func (a *keyExchangeAdapter) buildFormData(req *ports.KeyExchangeRequest) url.Values {
	data := url.Values{}

	// TRAN_GROUP values: "SALE", "AUTH", or "STORAGE"
	tranGroup := req.TranGroup

	// Normalize legacy single-letter codes to full strings for EPX compatibility
	if tranGroup == "U" {
		tranGroup = "SALE"
	} else if tranGroup == "A" {
		tranGroup = "AUTH"
	} else if tranGroup == "S" {
		tranGroup = "STORAGE"
	}

	// Merchant credentials - required per EPX certification sheet
	data.Set("CUST_NBR", req.CustNbr)
	data.Set("MERCH_NBR", req.MerchNbr)
	data.Set("DBA_NBR", req.DBAnbr)
	data.Set("TERMINAL_NBR", req.TerminalNbr)

	// Transaction parameters
	data.Set("TRAN_NBR", req.TranNbr)
	data.Set("AMOUNT", req.Amount)
	data.Set("MAC", req.MAC)
	data.Set("TRAN_GROUP", tranGroup)
	data.Set("REDIRECT_URL", req.RedirectURL)

	// Industry type (required for EPX certification)
	if req.IndustryType != "" {
		data.Set("INDUSTRY_TYPE", req.IndustryType)
	}

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
