package epx

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// BusinessReportingConfig contains configuration for EPX Business Reporting API
type BusinessReportingConfig struct {
	// Base URL for Business Reporting API
	// Sandbox: https://api-sandbox.north.com/reporting/v1
	// Production: https://api.north.com/reporting/v1
	BaseURL string

	// API credentials
	APIKey    string
	APISecret string

	// Merchant credentials (for filtering)
	CustNbr  string
	MerchNbr string
	DBAnbr   string

	// HTTP client timeout
	Timeout time.Duration

	// TLS configuration
	InsecureSkipVerify bool

	// Retry configuration
	MaxRetries int
	RetryDelay time.Duration
}

// DefaultBusinessReportingConfig returns default configuration for Business Reporting adapter
func DefaultBusinessReportingConfig(environment string) *BusinessReportingConfig {
	baseURL := "https://api.north.com/reporting/v1" // Production
	if environment == "sandbox" {
		baseURL = "https://api-sandbox.north.com/reporting/v1"
	}

	return &BusinessReportingConfig{
		BaseURL:            baseURL,
		Timeout:            30 * time.Second,
		InsecureSkipVerify: environment == "sandbox",
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
	}
}

// businessReportingAdapter implements the BusinessReportingAdapter port
type businessReportingAdapter struct {
	config     *BusinessReportingConfig
	httpClient *http.Client
	logger     *zap.Logger
	tracer     trace.Tracer
}

// NewBusinessReportingAdapter creates a new EPX Business Reporting adapter
func NewBusinessReportingAdapter(config *BusinessReportingConfig, logger *zap.Logger) ports.BusinessReportingAdapter {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	httpClient := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	logger.Info("Business Reporting API adapter initialized",
		zap.String("base_url", config.BaseURL),
	)

	return &businessReportingAdapter{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
		tracer:     otel.Tracer(epxTracerName),
	}
}

// GetTransaction retrieves detailed information about a single transaction by AUTH_GUID
func (a *businessReportingAdapter) GetTransaction(ctx context.Context, authGUID string) (*ports.TransactionDetails, error) {
	// Start tracing span for the API call
	ctx, span := a.tracer.Start(ctx, "epx.business_reporting.get_transaction",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gateway.name", "North Business Reporting"),
			attribute.String("gateway.endpoint", a.config.BaseURL),
			attribute.String("epx.auth_guid", authGUID),
		),
	)
	defer span.End()

	if authGUID == "" {
		err := fmt.Errorf("authGUID is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid request")
		return nil, err
	}

	a.logger.Info("Querying transaction details",
		zap.String("auth_guid", authGUID),
	)

	// Build request URL
	// NOTE: The actual endpoint may differ - this is based on common REST API patterns
	// Check North Developer documentation for the exact endpoint
	endpoint := fmt.Sprintf("%s/transactions/%s", a.config.BaseURL, authGUID)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	a.addAuthHeaders(req)

	// Execute request
	startTime := time.Now()
	resp, err := a.httpClient.Do(req)
	duration := time.Since(startTime)
	span.SetAttributes(attribute.Int64("gateway.duration_ms", duration.Milliseconds()))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to query transaction")
		a.logger.Error("Failed to query transaction",
			zap.String("auth_guid", authGUID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to query transaction: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			a.logger.Warn("Failed to read error response body", zap.Error(readErr))
			body = []byte("(failed to read response)")
		}
		a.logger.Error("Business Reporting API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
		)

		if resp.StatusCode == http.StatusNotFound {
			err := fmt.Errorf("transaction not found: %s", authGUID)
			span.RecordError(err)
			span.SetStatus(codes.Error, "transaction not found")
			return nil, err
		}

		err := fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
		span.RecordError(err)
		span.SetStatus(codes.Error, "API error")
		return nil, err
	}

	// Parse response
	var apiResp apiTransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to parse response")
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert API response to domain model
	txn := a.convertAPIResponseToTransaction(&apiResp)

	// Record success attributes
	span.SetAttributes(
		attribute.String("epx.transaction_status", string(txn.Status)),
		attribute.Bool("epx.is_ach_return", txn.IsACHReturn),
	)
	span.SetStatus(codes.Ok, "transaction retrieved")

	a.logger.Info("Transaction retrieved successfully",
		zap.String("auth_guid", txn.AuthGUID),
		zap.String("status", string(txn.Status)),
		zap.Bool("is_ach_return", txn.IsACHReturn),
	)

	return txn, nil
}

// QueryTransactions searches for transactions matching the given criteria
func (a *businessReportingAdapter) QueryTransactions(ctx context.Context, params *ports.TransactionQueryParams) (*ports.TransactionQueryResult, error) {
	// Start tracing span for the API call
	ctx, span := a.tracer.Start(ctx, "epx.business_reporting.query_transactions",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("gateway.name", "North Business Reporting"),
			attribute.String("gateway.endpoint", a.config.BaseURL),
			attribute.Bool("epx.ach_returns_only", params.ACHReturnsOnly),
			attribute.Int("epx.limit", params.Limit),
		),
	)
	defer span.End()

	a.logger.Info("Querying transactions",
		zap.Bool("ach_returns_only", params.ACHReturnsOnly),
		zap.Int("limit", params.Limit),
	)

	// Build query parameters
	queryParams := a.buildQueryParams(params)

	// Build request URL
	endpoint := fmt.Sprintf("%s/transactions?%s", a.config.BaseURL, queryParams.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	a.addAuthHeaders(req)

	// Execute request
	startTime := time.Now()
	resp, err := a.httpClient.Do(req)
	duration := time.Since(startTime)
	span.SetAttributes(attribute.Int64("gateway.duration_ms", duration.Milliseconds()))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to query transactions")
		a.logger.Error("Failed to query transactions", zap.Error(err))
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			a.logger.Warn("Failed to read error response body", zap.Error(readErr))
			body = []byte("(failed to read response)")
		}
		err := fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
		span.RecordError(err)
		span.SetStatus(codes.Error, "API error")
		a.logger.Error("Business Reporting API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
		)
		return nil, err
	}

	// Parse response
	var apiResp apiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to parse response")
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert API response to domain model
	result := &ports.TransactionQueryResult{
		Transactions: make([]*ports.TransactionDetails, len(apiResp.Transactions)),
		TotalCount:   apiResp.TotalCount,
		HasMore:      apiResp.HasMore,
	}

	for i, apiTxn := range apiResp.Transactions {
		result.Transactions[i] = a.convertAPIResponseToTransaction(&apiTxn)
	}

	// Record success attributes
	span.SetAttributes(
		attribute.Int("epx.result_count", len(result.Transactions)),
		attribute.Int("epx.total_count", result.TotalCount),
		attribute.Bool("epx.has_more", result.HasMore),
	)
	span.SetStatus(codes.Ok, "transactions queried")

	a.logger.Info("Transactions queried successfully",
		zap.Int("count", len(result.Transactions)),
		zap.Int("total", result.TotalCount),
	)

	return result, nil
}

// CheckACHReturns checks if a specific AUTH_GUID has any ACH returns
func (a *businessReportingAdapter) CheckACHReturns(ctx context.Context, authGUID string) (bool, string, string, error) {
	txn, err := a.GetTransaction(ctx, authGUID)
	if err != nil {
		return false, "", "", err
	}

	if txn.IsACHReturn {
		return true, txn.ACHReturnCode, txn.ACHReturnReason, nil
	}

	return false, "", "", nil
}

// GetACHReturnsForDateRange retrieves all ACH returns within a date range
func (a *businessReportingAdapter) GetACHReturnsForDateRange(ctx context.Context, startDate, endDate time.Time) ([]*ports.TransactionDetails, error) {
	params := &ports.TransactionQueryParams{
		StartDate:      &startDate,
		EndDate:        &endDate,
		ACHReturnsOnly: true,
		CustNbr:        a.config.CustNbr,
		MerchNbr:       a.config.MerchNbr,
		DBAnbr:         a.config.DBAnbr,
		Limit:          1000, // Max batch size
	}

	result, err := a.QueryTransactions(ctx, params)
	if err != nil {
		return nil, err
	}

	return result.Transactions, nil
}

// addAuthHeaders adds authentication headers to the request
func (a *businessReportingAdapter) addAuthHeaders(req *http.Request) {
	// NOTE: The actual authentication method may differ
	// Common patterns:
	// 1. API Key in header: X-API-Key: <key>
	// 2. Basic Auth: Authorization: Basic <base64(key:secret)>
	// 3. Bearer Token: Authorization: Bearer <token>
	//
	// Check North Developer documentation for the exact authentication method

	if a.config.APIKey != "" {
		req.Header.Set("X-API-Key", a.config.APIKey)
	}

	if a.config.APISecret != "" {
		req.Header.Set("X-API-Secret", a.config.APISecret)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

// buildQueryParams converts TransactionQueryParams to URL query parameters
func (a *businessReportingAdapter) buildQueryParams(params *ports.TransactionQueryParams) url.Values {
	q := url.Values{}

	if params.StartDate != nil {
		q.Set("start_date", params.StartDate.Format(time.RFC3339))
	}

	if params.EndDate != nil {
		q.Set("end_date", params.EndDate.Format(time.RFC3339))
	}

	if len(params.TranTypes) > 0 {
		q.Set("tran_types", strings.Join(params.TranTypes, ","))
	}

	if len(params.Statuses) > 0 {
		statuses := make([]string, len(params.Statuses))
		for i, s := range params.Statuses {
			statuses[i] = string(s)
		}
		q.Set("statuses", strings.Join(statuses, ","))
	}

	if params.CustNbr != "" {
		q.Set("cust_nbr", params.CustNbr)
	}

	if params.MerchNbr != "" {
		q.Set("merch_nbr", params.MerchNbr)
	}

	if params.DBAnbr != "" {
		q.Set("dba_nbr", params.DBAnbr)
	}

	if len(params.PaymentMethods) > 0 {
		q.Set("payment_methods", strings.Join(params.PaymentMethods, ","))
	}

	if params.ACHReturnsOnly {
		q.Set("ach_returns_only", "true")
	}

	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}

	if params.Offset > 0 {
		q.Set("offset", strconv.Itoa(params.Offset))
	}

	return q
}

// convertAPIResponseToTransaction converts API response to domain model
func (a *businessReportingAdapter) convertAPIResponseToTransaction(apiResp *apiTransactionResponse) *ports.TransactionDetails {
	txn := &ports.TransactionDetails{
		AuthGUID:     apiResp.AuthGUID,
		TranNbr:      apiResp.TranNbr,
		TranType:     apiResp.TranType,
		AuthResp:     apiResp.AuthResp,
		AuthRespText: apiResp.AuthRespText,
		Amount:       apiResp.Amount,
		CurrencyCode: apiResp.CurrencyCode,
		CustNbr:      apiResp.CustNbr,
		MerchNbr:     apiResp.MerchNbr,
		DBAnbr:       apiResp.DBAnbr,
		TerminalNbr:  apiResp.TerminalNbr,
		BatchID:      apiResp.BatchID,
		Metadata:     make(map[string]string),
	}

	// Map status string to enum
	txn.Status = a.mapStatus(apiResp.Status)

	// Parse timestamps
	if apiResp.TransactionDate != "" {
		if t, err := time.Parse(time.RFC3339, apiResp.TransactionDate); err == nil {
			txn.TransactionDate = t
		}
	}

	if apiResp.SettlementDate != "" {
		if t, err := time.Parse(time.RFC3339, apiResp.SettlementDate); err == nil {
			txn.SettlementDate = &t
		}
	}

	// Handle ACH return information
	if apiResp.ACHReturn != nil {
		txn.IsACHReturn = true
		txn.ACHReturnCode = apiResp.ACHReturn.ReturnCode
		txn.ACHReturnReason = apiResp.ACHReturn.ReturnReason
		txn.OriginalAuthGUID = apiResp.ACHReturn.OriginalAuthGUID

		if apiResp.ACHReturn.ReturnDate != "" {
			if t, err := time.Parse(time.RFC3339, apiResp.ACHReturn.ReturnDate); err == nil {
				txn.ACHReturnDate = &t
			}
		}
	}

	// Map payment method
	txn.PaymentMethod = a.mapPaymentMethod(apiResp.PaymentMethod)
	txn.MaskedAccountNbr = apiResp.MaskedAccountNbr
	txn.CardType = apiResp.CardType

	return txn
}

// mapStatus maps API status string to TransactionStatus enum
func (a *businessReportingAdapter) mapStatus(status string) ports.TransactionStatus {
	switch strings.ToLower(status) {
	case "approved", "success":
		return ports.TransactionStatusApproved
	case "declined", "failed":
		return ports.TransactionStatusDeclined
	case "pending":
		return ports.TransactionStatusPending
	case "returned":
		return ports.TransactionStatusReturned
	case "voided":
		return ports.TransactionStatusVoided
	case "refunded":
		return ports.TransactionStatusRefunded
	case "settled":
		return ports.TransactionStatusSettled
	default:
		return ports.TransactionStatusPending
	}
}

// mapPaymentMethod maps API payment method string to standardized format
func (a *businessReportingAdapter) mapPaymentMethod(method string) string {
	switch strings.ToLower(method) {
	case "credit_card", "card", "cc":
		return "credit_card"
	case "ach_checking", "checking", "ach":
		return "ach_checking"
	case "ach_savings", "savings":
		return "ach_savings"
	default:
		return method
	}
}

// API response structures
// NOTE: These are based on common REST API patterns
// The actual structure may differ - check North Developer documentation

type apiTransactionResponse struct {
	AuthGUID         string        `json:"auth_guid"`
	TranNbr          string        `json:"tran_nbr"`
	TranType         string        `json:"tran_type"`
	Status           string        `json:"status"`
	AuthResp         string        `json:"auth_resp"`
	AuthRespText     string        `json:"auth_resp_text"`
	Amount           string        `json:"amount"`
	CurrencyCode     string        `json:"currency_code"`
	TransactionDate  string        `json:"transaction_date"`
	SettlementDate   string        `json:"settlement_date,omitempty"`
	PaymentMethod    string        `json:"payment_method"`
	MaskedAccountNbr string        `json:"masked_account_nbr"`
	CardType         string        `json:"card_type,omitempty"`
	CustNbr          string        `json:"cust_nbr"`
	MerchNbr         string        `json:"merch_nbr"`
	DBAnbr           string        `json:"dba_nbr"`
	TerminalNbr      string        `json:"terminal_nbr"`
	BatchID          string        `json:"batch_id"`
	ACHReturn        *apiACHReturn `json:"ach_return,omitempty"`
}

type apiACHReturn struct {
	ReturnCode       string `json:"return_code"`
	ReturnReason     string `json:"return_reason"`
	ReturnDate       string `json:"return_date"`
	OriginalAuthGUID string `json:"original_auth_guid"`
}

type apiQueryResponse struct {
	Transactions []apiTransactionResponse `json:"transactions"`
	TotalCount   int                      `json:"total_count"`
	HasMore      bool                     `json:"has_more"`
}
