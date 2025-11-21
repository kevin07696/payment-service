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
	"github.com/kevin07696/payment-service/pkg/resilience"
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
	MaxRetries      int
	RetryDelay      time.Duration
	RetryableErrors []string // Error codes that should trigger retry
}

// DefaultServerPostConfig returns default configuration for Server Post adapter
func DefaultServerPostConfig(environment string) *ServerPostConfig {
	baseURL := "https://epxnow.com/epx/server_post" // Production
	socketEndpoint := "epxnow.com:8086"             // Production
	if environment == "sandbox" {
		baseURL = "https://secure.epxuap.com"
		socketEndpoint = "secure.epxuap.com:8087"
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
	config         *ServerPostConfig
	httpClient     *http.Client
	logger         *zap.Logger
	circuitBreaker *CircuitBreaker
	backoff        resilience.BackoffStrategy
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

	// Initialize circuit breaker with defaults
	circuitBreaker := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	return &serverPostAdapter{
		config:         config,
		httpClient:     httpClient,
		logger:         logger,
		circuitBreaker: circuitBreaker,
		backoff:        resilience.DefaultExponentialBackoff(),
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

	// Execute request through circuit breaker
	var response *ports.ServerPostResponse
	err = a.circuitBreaker.Call(func() error {
		// Send request with retries
		var lastErr error
		for attempt := 0; attempt <= a.config.MaxRetries; attempt++ {
			if attempt > 0 {
				// Calculate exponential backoff delay with jitter
				delay := a.backoff.NextDelay(attempt - 1)
				a.logger.Info("Retrying Server Post request with exponential backoff",
					zap.Int("attempt", attempt),
					zap.Int("max_retries", a.config.MaxRetries),
					zap.Duration("backoff_delay", delay),
				)
				// Respect context cancellation during retry delay
				select {
				case <-ctx.Done():
					return fmt.Errorf("retry cancelled: %w", ctx.Err())
				case <-time.After(delay):
					// Continue to retry
				}
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
				return fmt.Errorf("failed to send request: %w", err)
			}
			defer httpResp.Body.Close()

			// Read response body
			body, err := io.ReadAll(httpResp.Body)
			if err != nil {
				a.logger.Error("Failed to read response body", zap.Error(err))
				return fmt.Errorf("failed to read response: %w", err)
			}

			a.logger.Info("Received Server Post response",
				zap.Int("status_code", httpResp.StatusCode),
				zap.Duration("elapsed", time.Since(startTime)),
				zap.Int("body_length", len(body)),
				zap.String("response_body", string(body)),
			)

			// Parse response
			parsedResp, err := a.parseResponse(body, req)
			if err != nil {
				a.logger.Error("Failed to parse Server Post response",
					zap.Error(err),
					zap.String("body", string(body)),
				)
				return fmt.Errorf("failed to parse response: %w", err)
			}

			a.logger.Info("Successfully processed Server Post transaction",
				zap.String("auth_guid", parsedResp.AuthGUID),
				zap.String("auth_resp", parsedResp.AuthResp),
				zap.String("auth_resp_text", parsedResp.AuthRespText),
				zap.Bool("is_approved", parsedResp.IsApproved),
			)

			response = parsedResp
			return nil
		}

		return fmt.Errorf("failed after %d retries: %w", a.config.MaxRetries, lastErr)
	})

	if err != nil {
		// Check if circuit breaker rejected the request
		if err == ErrCircuitOpen {
			a.logger.Warn("Circuit breaker is open, rejecting EPX request",
				zap.String("circuit_state", a.circuitBreaker.State().String()),
			)
		}
		return nil, err
	}

	return response, nil
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

	// Execute socket call through circuit breaker
	var response *ports.ServerPostResponse
	err := a.circuitBreaker.Call(func() error {
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
			return fmt.Errorf("failed to connect to socket: %w", err)
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
			return fmt.Errorf("failed to write to socket: %w", err)
		}

		// Read response
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		if err != nil {
			a.logger.Error("Failed to read from socket", zap.Error(err))
			return fmt.Errorf("failed to read from socket: %w", err)
		}

		responseXML := buffer[:n]

		a.logger.Info("Received Socket response",
			zap.Duration("elapsed", time.Since(startTime)),
			zap.Int("bytes_received", n),
		)

		// Parse XML response
		parsedResp, err := a.parseXMLResponse(responseXML, req)
		if err != nil {
			a.logger.Error("Failed to parse Socket response",
				zap.Error(err),
				zap.String("xml", string(responseXML)),
			)
			return fmt.Errorf("failed to parse response: %w", err)
		}

		a.logger.Info("Successfully processed Socket transaction",
			zap.String("auth_guid", parsedResp.AuthGUID),
			zap.String("auth_resp", parsedResp.AuthResp),
			zap.Bool("is_approved", parsedResp.IsApproved),
		)

		response = parsedResp
		return nil
	})

	if err != nil {
		// Check if circuit breaker rejected the request
		if err == ErrCircuitOpen {
			a.logger.Warn("Circuit breaker is open, rejecting EPX socket request",
				zap.String("circuit_state", a.circuitBreaker.State().String()),
			)
		}
		return nil, err
	}

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
	if req.TranNbr == "" {
		return fmt.Errorf("tran_nbr is required")
	}

	// Amount is optional for BRIC Storage (uses $0.00 Account Verification)
	if req.TransactionType != ports.TransactionTypeBRICStorageCC && req.TransactionType != ports.TransactionTypeBRICStorageACH {
		if req.Amount == "" {
			return fmt.Errorf("amount is required")
		}
		// Validate amount format
		if _, err := strconv.ParseFloat(req.Amount, 64); err != nil {
			return fmt.Errorf("amount must be numeric: %w", err)
		}
	} else if req.Amount != "" {
		// If amount is provided for BRIC Storage, validate it
		if _, err := strconv.ParseFloat(req.Amount, 64); err != nil {
			return fmt.Errorf("amount must be numeric: %w", err)
		}
	}

	// Validate transaction type
	validTypes := map[ports.TransactionType]bool{
		// Credit Card
		ports.TransactionTypeSale:          true,
		ports.TransactionTypeAuthOnly:      true,
		ports.TransactionTypeCapture:       true,
		ports.TransactionTypeRefund:        true,
		ports.TransactionTypeVoid:          true,
		ports.TransactionTypeReversal:      true,
		ports.TransactionTypeBRICStorageCC: true,
		// ACH Checking
		ports.TransactionTypeACHDebit:         true,
		ports.TransactionTypeACHCredit:        true,
		ports.TransactionTypeACHPreNoteDebit:  true,
		ports.TransactionTypeACHPreNoteCredit: true,
		ports.TransactionTypeACHVoid:          true,
		ports.TransactionTypeBRICStorageACH:   true,
		// ACH Savings
		ports.TransactionTypeACHSavingsDebit:         true,
		ports.TransactionTypeACHSavingsCredit:        true,
		ports.TransactionTypeACHSavingsPreNoteDebit:  true,
		ports.TransactionTypeACHSavingsPreNoteCredit: true,
		ports.TransactionTypeACHSavingsVoid:          true,
		// PIN-less Debit
		ports.TransactionTypePINlessDebitPurchase: true,
		ports.TransactionTypePINlessDebitReturn:   true,
		ports.TransactionTypePINlessDebitVoid:     true,
	}
	if !validTypes[req.TransactionType] {
		return fmt.Errorf("invalid transaction type: %s", req.TransactionType)
	}

	// For capture/void/refund, require original AUTH_GUID
	if req.TransactionType == ports.TransactionTypeCapture ||
		req.TransactionType == ports.TransactionTypeVoid ||
		req.TransactionType == ports.TransactionTypeRefund ||
		req.TransactionType == ports.TransactionTypeACHVoid ||
		req.TransactionType == ports.TransactionTypeACHSavingsVoid ||
		req.TransactionType == ports.TransactionTypeACHCredit ||
		req.TransactionType == ports.TransactionTypeACHSavingsCredit {
		if req.OriginalAuthGUID == "" {
			return fmt.Errorf("original_auth_guid is required for %s transactions", req.TransactionType)
		}
	}

	// For ACH transactions, validate required fields
	isACH := req.PaymentType == ports.PaymentMethodTypeACH ||
		req.TransactionType == ports.TransactionTypeACHDebit ||
		req.TransactionType == ports.TransactionTypeACHCredit ||
		req.TransactionType == ports.TransactionTypeACHPreNoteDebit ||
		req.TransactionType == ports.TransactionTypeACHPreNoteCredit ||
		req.TransactionType == ports.TransactionTypeACHVoid ||
		req.TransactionType == ports.TransactionTypeACHSavingsDebit ||
		req.TransactionType == ports.TransactionTypeACHSavingsCredit ||
		req.TransactionType == ports.TransactionTypeACHSavingsPreNoteDebit ||
		req.TransactionType == ports.TransactionTypeACHSavingsPreNoteCredit ||
		req.TransactionType == ports.TransactionTypeACHSavingsVoid ||
		req.TransactionType == ports.TransactionTypeBRICStorageACH

	if isACH {
		// For new ACH transactions (not using existing BRIC), require account details
		if req.AuthGUID == "" && req.OriginalAuthGUID == "" {
			if req.AccountNumber == nil || *req.AccountNumber == "" {
				return fmt.Errorf("account_number is required for ACH transactions")
			}
			if req.RoutingNumber == nil || *req.RoutingNumber == "" {
				return fmt.Errorf("routing_number is required for ACH transactions")
			}
			if req.FirstName == nil || *req.FirstName == "" {
				return fmt.Errorf("first_name is required for ACH transactions")
			}
			if req.LastName == nil || *req.LastName == "" {
				return fmt.Errorf("last_name is required for ACH transactions")
			}
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

	// BATCH_ID: EPX requires date format YYYYMMDD or simple number (max 8 chars)
	// Using today's date in YYYYMMDD format
	now := time.Now()
	batchID := now.Format("20060102") // Format as YYYYMMDD
	data.Set("BATCH_ID", batchID)

	// LOCAL_DATE and LOCAL_TIME: Required by EPX
	localDate := now.Format("010206") // MMDDYY format
	localTime := now.Format("150405") // HHMMSS format
	data.Set("LOCAL_DATE", localDate)
	data.Set("LOCAL_TIME", localTime)

	// Payment token (BRIC)
	// Per EPX documentation: When using a BRIC for AUTH/SALE transactions, it should be sent as ORIG_AUTH_GUID
	// AUTH_GUID is only returned in responses, never sent in requests for financial transactions
	if req.AuthGUID != "" {
		data.Set("ORIG_AUTH_GUID", req.AuthGUID)
	}

	// For capture/void/refund (use OriginalAuthGUID field to reference the original transaction)
	if req.OriginalAuthGUID != "" {
		data.Set("ORIG_AUTH_GUID", req.OriginalAuthGUID)
	}

	// Account information (for new card transactions)
	if req.AccountNumber != nil && *req.AccountNumber != "" {
		data.Set("ACCOUNT_NBR", *req.AccountNumber)
	}

	if req.RoutingNumber != nil && *req.RoutingNumber != "" {
		data.Set("ROUTING_NBR", *req.RoutingNumber)
	}

	if req.ExpirationDate != nil && *req.ExpirationDate != "" {
		data.Set("EXP_DATE", *req.ExpirationDate)
	}

	if req.CVV != nil && *req.CVV != "" {
		data.Set("CVV2", *req.CVV)
	}

	// Card entry method and industry type
	if req.CardEntryMethod != nil && *req.CardEntryMethod != "" {
		data.Set("CARD_ENT_METH", *req.CardEntryMethod)
	}

	if req.IndustryType != nil && *req.IndustryType != "" {
		data.Set("INDUSTRY_TYPE", *req.IndustryType)
	}

	// Authorization Characteristics Indicator Extension (for COF, MIT, Recurring)
	if req.ACIExt != nil && *req.ACIExt != "" {
		data.Set("ACI_EXT", *req.ACIExt)
	}

	// Billing information
	if req.FirstName != nil && *req.FirstName != "" {
		data.Set("FIRST_NAME", *req.FirstName)
	}

	if req.LastName != nil && *req.LastName != "" {
		data.Set("LAST_NAME", *req.LastName)
	}

	if req.Address != nil && *req.Address != "" {
		data.Set("ADDRESS", *req.Address)
	}

	if req.City != nil && *req.City != "" {
		data.Set("CITY", *req.City)
	}

	if req.State != nil && *req.State != "" {
		data.Set("STATE", *req.State)
	}

	if req.ZipCode != nil && *req.ZipCode != "" {
		data.Set("ZIP_CODE", *req.ZipCode)
	}

	// ACH-specific fields
	if req.StdEntryClass != nil && *req.StdEntryClass != "" {
		data.Set("STD_ENTRY_CLASS", *req.StdEntryClass)
	}

	if req.ReceiverName != nil && *req.ReceiverName != "" {
		data.Set("RECV_NAME", *req.ReceiverName)
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
// EPX returns responses in <FIELD KEY="xxx">value</FIELD> format
type EPXResponse struct {
	XMLName xml.Name  `xml:"RESPONSE"`
	Fields  EPXFields `xml:"FIELDS"`
}

type EPXFields struct {
	Fields []EPXField `xml:"FIELD"`
}

type EPXField struct {
	Key   string `xml:"KEY,attr"`
	Value string `xml:",chardata"`
}

// parseResponse parses key-value response from HTTPS POST
func (a *serverPostAdapter) parseResponse(body []byte, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
	// Parse response (could be XML or key-value pairs)
	responseStr := strings.TrimSpace(string(body))

	// Check if response is XML
	if strings.HasPrefix(responseStr, "<") {
		a.logger.Info("Parsing as XML response")
		return a.parseXMLResponse(body, req)
	}

	// Try parsing as URL-encoded key-value pairs
	params, err := url.ParseQuery(responseStr)
	if err == nil && len(params) > 0 {
		a.logger.Info("Parsing as key-value response")
		return a.parseKeyValueResponse(params, req)
	}

	// Default to XML if parsing fails
	a.logger.Info("Defaulting to XML parsing")
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

	// Convert field array to map for easy lookup
	fieldMap := make(map[string]string)
	for _, field := range epxResp.Fields.Fields {
		fieldMap[field.Key] = field.Value
	}

	authGUID := fieldMap["AUTH_GUID"]
	authResp := fieldMap["AUTH_RESP"]

	if authGUID == "" {
		return nil, fmt.Errorf("AUTH_GUID is missing from XML response")
	}
	if authResp == "" {
		return nil, fmt.Errorf("AUTH_RESP is missing from XML response")
	}

	isApproved := authResp == "00"

	return &ports.ServerPostResponse{
		AuthGUID:     authGUID,
		AuthResp:     authResp,
		AuthCode:     fieldMap["AUTH_CODE"],
		AuthRespText: fieldMap["AUTH_RESP_TEXT"],
		IsApproved:   isApproved,
		AuthCardType: fieldMap["AUTH_CARD_TYPE"],
		AuthAVS:      fieldMap["AUTH_AVS"],
		AuthCVV2:     fieldMap["AUTH_CVV2"],
		TranNbr:      fieldMap["TRAN_NBR"],
		TranGroup:    fieldMap["TRAN_GROUP"],
		Amount:       fieldMap["AMOUNT"],
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
