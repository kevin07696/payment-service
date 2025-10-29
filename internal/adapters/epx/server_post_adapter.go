package epx

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// ServerPostConfig contains configuration for EPX Server Post adapter
type ServerPostConfig struct {
	// Base URL for HTTPS POST method
	// Sandbox: https://epxnow.com/epx/server_post_sandbox
	// Production: https://epxnow.com/epx/server_post
	BaseURL string

	// Socket endpoint for XML Socket method (host:port)
	// Sandbox: epxnow.com:8087
	// Production: epxnow.com:8086
	SocketEndpoint string

	// HTTP client timeout
	Timeout time.Duration

	// Socket connection timeout
	SocketTimeout time.Duration

	// TLS configuration
	InsecureSkipVerify bool

	// Retry configuration
	MaxRetries    int
	RetryDelay    time.Duration
	RetryableErrors []string // Error codes that should trigger retry
}

// DefaultServerPostConfig returns default configuration for Server Post adapter
func DefaultServerPostConfig(environment string) *ServerPostConfig {
	baseURL := "https://epxnow.com/epx/server_post" // Production
	socketEndpoint := "epxnow.com:8086"              // Production
	if environment == "sandbox" {
		baseURL = "https://epxnow.com/epx/server_post_sandbox"
		socketEndpoint = "epxnow.com:8087"
	}

	return &ServerPostConfig{
		BaseURL:            baseURL,
		SocketEndpoint:     socketEndpoint,
		Timeout:            30 * time.Second,
		SocketTimeout:      30 * time.Second, // EPX socket stays open 30 seconds
		InsecureSkipVerify: environment == "sandbox",
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		RetryableErrors:    []string{"timeout", "connection", "temporary"},
	}
}

// serverPostAdapter implements the ServerPostAdapter port
type serverPostAdapter struct {
	config     *ServerPostConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewServerPostAdapter creates a new EPX Server Post adapter
func NewServerPostAdapter(config *ServerPostConfig, logger *zap.Logger) ports.ServerPostAdapter {
	// Configure HTTP client
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

	return &serverPostAdapter{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
	}
}

// ProcessTransaction sends a transaction request to EPX Server Post API via HTTPS POST
// Based on EPX Server Post API - HTTPS POST Method (page 3-5)
func (a *serverPostAdapter) ProcessTransaction(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
	// Validate request
	if err := a.validateRequest(req); err != nil {
		a.logger.Error("Invalid Server Post request", zap.Error(err))
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	a.logger.Info("Processing EPX Server Post transaction",
		zap.String("transaction_type", string(req.TransactionType)),
		zap.String("tran_nbr", req.TranNbr),
		zap.String("amount", req.Amount),
	)

	// Build form data
	formData := a.buildFormData(req)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL, strings.NewReader(formData.Encode()))
	if err != nil {
		a.logger.Error("Failed to create HTTP request", zap.Error(err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request with retries
	var lastErr error
	for attempt := 0; attempt <= a.config.MaxRetries; attempt++ {
		if attempt > 0 {
			a.logger.Info("Retrying Server Post request",
				zap.Int("attempt", attempt),
				zap.Int("max_retries", a.config.MaxRetries),
			)
			time.Sleep(a.config.RetryDelay)
		}

		startTime := time.Now()
		httpResp, err := a.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			if a.isRetryable(err) && attempt < a.config.MaxRetries {
				a.logger.Warn("Retryable error occurred",
					zap.Error(err),
					zap.Int("attempt", attempt),
				)
				continue
			}
			a.logger.Error("Failed to send Server Post request",
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

		a.logger.Info("Received Server Post response",
			zap.Int("status_code", httpResp.StatusCode),
			zap.Duration("elapsed", time.Since(startTime)),
			zap.Int("body_length", len(body)),
		)

		// Parse response
		response, err := a.parseResponse(body, req)
		if err != nil {
			a.logger.Error("Failed to parse Server Post response",
				zap.Error(err),
				zap.String("body", string(body)),
			)
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		a.logger.Info("Successfully processed Server Post transaction",
			zap.String("auth_guid", response.AuthGUID),
			zap.String("auth_resp", response.AuthResp),
			zap.Bool("is_approved", response.IsApproved),
		)

		return response, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", a.config.MaxRetries, lastErr)
}

// ProcessTransactionViaSocket sends transaction via XML Socket connection
// Based on EPX Server Post API - XML Socket Method (page 3-4)
func (a *serverPostAdapter) ProcessTransactionViaSocket(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
	// Validate request
	if err := a.validateRequest(req); err != nil {
		a.logger.Error("Invalid Server Post request", zap.Error(err))
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	a.logger.Info("Processing EPX Server Post via XML Socket",
		zap.String("transaction_type", string(req.TransactionType)),
		zap.String("tran_nbr", req.TranNbr),
		zap.String("socket_endpoint", a.config.SocketEndpoint),
	)

	// Build XML request
	xmlData := a.buildXMLRequest(req)

	// Connect to socket with timeout
	dialer := &net.Dialer{
		Timeout: a.config.SocketTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", a.config.SocketEndpoint)
	if err != nil {
		a.logger.Error("Failed to connect to EPX socket",
			zap.Error(err),
			zap.String("endpoint", a.config.SocketEndpoint),
		)
		return nil, fmt.Errorf("failed to connect to socket: %w", err)
	}
	defer conn.Close()

	// Set read/write deadlines
	deadline := time.Now().Add(a.config.SocketTimeout)
	conn.SetDeadline(deadline)

	// Send XML request
	startTime := time.Now()
	_, err = conn.Write([]byte(xmlData))
	if err != nil {
		a.logger.Error("Failed to write to socket", zap.Error(err))
		return nil, fmt.Errorf("failed to write to socket: %w", err)
	}

	// Read response
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		a.logger.Error("Failed to read from socket", zap.Error(err))
		return nil, fmt.Errorf("failed to read from socket: %w", err)
	}

	responseXML := buffer[:n]

	a.logger.Info("Received Socket response",
		zap.Duration("elapsed", time.Since(startTime)),
		zap.Int("bytes_received", n),
	)

	// Parse XML response
	response, err := a.parseXMLResponse(responseXML, req)
	if err != nil {
		a.logger.Error("Failed to parse Socket response",
			zap.Error(err),
			zap.String("xml", string(responseXML)),
		)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	a.logger.Info("Successfully processed Socket transaction",
		zap.String("auth_guid", response.AuthGUID),
		zap.String("auth_resp", response.AuthResp),
		zap.Bool("is_approved", response.IsApproved),
	)

	return response, nil
}

// ValidateToken checks if a BRIC token (AUTH_GUID) is still valid
func (a *serverPostAdapter) ValidateToken(ctx context.Context, authGUID string) error {
	a.logger.Info("Validating BRIC token", zap.String("auth_guid", authGUID))

	// Perform a $0.00 authorization to verify token
	req := &ports.ServerPostRequest{
		TransactionType: ports.TransactionTypeAuthOnly,
		Amount:          "0.00",
		PaymentType:     ports.PaymentMethodTypeCreditCard,
		AuthGUID:        authGUID,
		TranNbr:         fmt.Sprintf("validate-%d", time.Now().Unix()),
		TranGroup:       fmt.Sprintf("validate-%d", time.Now().Unix()),
	}

	response, err := a.ProcessTransaction(ctx, req)
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	if !response.IsApproved {
		return fmt.Errorf("token is invalid or expired: %s", response.AuthRespText)
	}

	a.logger.Info("Token validation successful", zap.String("auth_guid", authGUID))
	return nil
}

// validateRequest validates the Server Post request parameters
func (a *serverPostAdapter) validateRequest(req *ports.ServerPostRequest) error {
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
	if req.Amount == "" {
		return fmt.Errorf("amount is required")
	}
	if req.TranNbr == "" {
		return fmt.Errorf("tran_nbr is required")
	}

	// Validate amount format
	if _, err := strconv.ParseFloat(req.Amount, 64); err != nil {
		return fmt.Errorf("amount must be numeric: %w", err)
	}

	// Validate transaction type
	validTypes := map[ports.TransactionType]bool{
		ports.TransactionTypeAuthOnly: true,
		ports.TransactionTypeCapture:  true,
		ports.TransactionTypeSale:     true,
		ports.TransactionTypeRefund:   true,
		ports.TransactionTypeVoid:     true,
		ports.TransactionTypePreNote:  true,
	}
	if !validTypes[req.TransactionType] {
		return fmt.Errorf("invalid transaction type: %s", req.TransactionType)
	}

	// For capture/void/refund, require original AUTH_GUID
	if req.TransactionType == ports.TransactionTypeCapture ||
		req.TransactionType == ports.TransactionTypeVoid ||
		req.TransactionType == ports.TransactionTypeRefund {
		if req.OriginalAuthGUID == "" {
			return fmt.Errorf("original_auth_guid is required for %s transactions", req.TransactionType)
		}
	}

	return nil
}

// buildFormData constructs URL-encoded form data for HTTPS POST
func (a *serverPostAdapter) buildFormData(req *ports.ServerPostRequest) url.Values {
	data := url.Values{}

	// EPX credentials
	data.Set("CUST_NBR", req.CustNbr)
	data.Set("MERCH_NBR", req.MerchNbr)
	data.Set("DBA_NBR", req.DBAnbr)
	data.Set("TERMINAL_NBR", req.TerminalNbr)

	// Transaction details
	data.Set("TRAN_TYPE", string(req.TransactionType))
	data.Set("AMOUNT", req.Amount)
	data.Set("TRAN_NBR", req.TranNbr)

	if req.TranGroup != "" {
		data.Set("TRAN_GROUP", req.TranGroup)
	}

	// Payment token (BRIC)
	if req.AuthGUID != "" {
		data.Set("AUTH_GUID", req.AuthGUID)
	}

	// For capture/void/refund
	if req.OriginalAuthGUID != "" {
		data.Set("ORIG_AUTH_GUID", req.OriginalAuthGUID)
	}

	return data
}

// buildXMLRequest constructs XML request for socket method
func (a *serverPostAdapter) buildXMLRequest(req *ports.ServerPostRequest) string {
	// Simple XML structure based on EPX Server Post API
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<transaction>
	<CUST_NBR>%s</CUST_NBR>
	<MERCH_NBR>%s</MERCH_NBR>
	<DBA_NBR>%s</DBA_NBR>
	<TERMINAL_NBR>%s</TERMINAL_NBR>
	<TRAN_TYPE>%s</TRAN_TYPE>
	<AMOUNT>%s</AMOUNT>
	<AUTH_GUID>%s</AUTH_GUID>
	<TRAN_NBR>%s</TRAN_NBR>
	<TRAN_GROUP>%s</TRAN_GROUP>
</transaction>`,
		req.CustNbr,
		req.MerchNbr,
		req.DBAnbr,
		req.TerminalNbr,
		req.TransactionType,
		req.Amount,
		req.AuthGUID,
		req.TranNbr,
		req.TranGroup,
	)
}

// EPXResponse represents the XML response structure from EPX
type EPXResponse struct {
	XMLName      xml.Name `xml:"response"`
	AuthGUID     string   `xml:"AUTH_GUID"`
	AuthResp     string   `xml:"AUTH_RESP"`
	AuthCode     string   `xml:"AUTH_CODE"`
	AuthRespText string   `xml:"AUTH_RESP_TEXT"`
	AuthCardType string   `xml:"AUTH_CARD_TYPE"`
	AuthAVS      string   `xml:"AUTH_AVS"`
	AuthCVV2     string   `xml:"AUTH_CVV2"`
	TranNbr      string   `xml:"TRAN_NBR"`
	TranGroup    string   `xml:"TRAN_GROUP"`
	Amount       string   `xml:"AMOUNT"`
}

// parseResponse parses key-value response from HTTPS POST
func (a *serverPostAdapter) parseResponse(body []byte, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
	// Parse response (could be XML or key-value pairs)
	responseStr := strings.TrimSpace(string(body))

	// Try parsing as URL-encoded key-value pairs first
	params, err := url.ParseQuery(responseStr)
	if err == nil && len(params) > 0 {
		return a.parseKeyValueResponse(params, req)
	}

	// Try parsing as XML
	return a.parseXMLResponse(body, req)
}

// parseKeyValueResponse parses URL-encoded key-value response
func (a *serverPostAdapter) parseKeyValueResponse(params url.Values, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
	authGUID := params.Get("AUTH_GUID")
	authResp := params.Get("AUTH_RESP")

	if authGUID == "" {
		return nil, fmt.Errorf("AUTH_GUID is missing from response")
	}
	if authResp == "" {
		return nil, fmt.Errorf("AUTH_RESP is missing from response")
	}

	isApproved := authResp == "00"

	return &ports.ServerPostResponse{
		AuthGUID:     authGUID,
		AuthResp:     authResp,
		AuthCode:     params.Get("AUTH_CODE"),
		AuthRespText: params.Get("AUTH_RESP_TEXT"),
		IsApproved:   isApproved,
		AuthCardType: params.Get("AUTH_CARD_TYPE"),
		AuthAVS:      params.Get("AUTH_AVS"),
		AuthCVV2:     params.Get("AUTH_CVV2"),
		TranNbr:      params.Get("TRAN_NBR"),
		TranGroup:    params.Get("TRAN_GROUP"),
		Amount:       params.Get("AMOUNT"),
		ProcessedAt:  time.Now(),
		RawXML:       "",
	}, nil
}

// parseXMLResponse parses XML response from socket or HTTPS POST
func (a *serverPostAdapter) parseXMLResponse(body []byte, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
	var epxResp EPXResponse
	if err := xml.Unmarshal(body, &epxResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal XML: %w", err)
	}

	if epxResp.AuthGUID == "" {
		return nil, fmt.Errorf("AUTH_GUID is missing from XML response")
	}
	if epxResp.AuthResp == "" {
		return nil, fmt.Errorf("AUTH_RESP is missing from XML response")
	}

	isApproved := epxResp.AuthResp == "00"

	return &ports.ServerPostResponse{
		AuthGUID:     epxResp.AuthGUID,
		AuthResp:     epxResp.AuthResp,
		AuthCode:     epxResp.AuthCode,
		AuthRespText: epxResp.AuthRespText,
		IsApproved:   isApproved,
		AuthCardType: epxResp.AuthCardType,
		AuthAVS:      epxResp.AuthAVS,
		AuthCVV2:     epxResp.AuthCVV2,
		TranNbr:      epxResp.TranNbr,
		TranGroup:    epxResp.TranGroup,
		Amount:       epxResp.Amount,
		ProcessedAt:  time.Now(),
		RawXML:       string(body),
	}, nil
}

// isRetryable determines if an error should trigger a retry
func (a *serverPostAdapter) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	for _, retryable := range a.config.RetryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	return false
}
