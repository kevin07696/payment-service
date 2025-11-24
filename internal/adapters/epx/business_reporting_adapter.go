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

	"github.com/kevin07696/payment-service/internal/adapters/ports"
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
	}
}

// GetTransaction retrieves detailed information about a single transaction by AUTH_GUID
func (a *businessReportingAdapter) GetTransaction(ctx context.Context, authGUID string) (*ports.TransactionDetails, error) {
	if authGUID == "" {
		return nil, fmt.Errorf("authGUID is required")
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
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	a.addAuthHeaders(req)

	// Execute request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.logger.Error("Failed to query transaction",
			zap.String("auth_guid", authGUID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to query transaction: %w", err)
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		a.logger.Error("Business Reporting API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
		)

		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("transaction not found: %s", authGUID)
		}

		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp apiTransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert API response to domain model
	txn := a.convertAPIResponseToTransaction(&apiResp)

	a.logger.Info("Transaction retrieved successfully",
		zap.String("auth_guid", txn.AuthGUID),
		zap.String("status", string(txn.Status)),
		zap.Bool("is_ach_return", txn.IsACHReturn),
	)

	return txn, nil
}

// QueryTransactions searches for transactions matching the given criteria
func (a *businessReportingAdapter) QueryTransactions(ctx context.Context, params *ports.TransactionQueryParams) (*ports.TransactionQueryResult, error) {
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
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	a.addAuthHeaders(req)

	// Execute request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.logger.Error("Failed to query transactions", zap.Error(err))
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer resp.Body.Close()

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		a.logger.Error("Business Reporting API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
		)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp apiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
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
	AuthGUID        string  `json:"auth_guid"`
	TranNbr         string  `json:"tran_nbr"`
	TranType        string  `json:"tran_type"`
	Status          string  `json:"status"`
	AuthResp        string  `json:"auth_resp"`
	AuthRespText    string  `json:"auth_resp_text"`
	Amount          string  `json:"amount"`
	CurrencyCode    string  `json:"currency_code"`
	TransactionDate string  `json:"transaction_date"`
	SettlementDate  string  `json:"settlement_date,omitempty"`
	PaymentMethod   string  `json:"payment_method"`
	MaskedAccountNbr string `json:"masked_account_nbr"`
	CardType        string  `json:"card_type,omitempty"`
	CustNbr         string  `json:"cust_nbr"`
	MerchNbr        string  `json:"merch_nbr"`
	DBAnbr          string  `json:"dba_nbr"`
	TerminalNbr     string  `json:"terminal_nbr"`
	BatchID         string  `json:"batch_id"`
	ACHReturn       *apiACHReturn `json:"ach_return,omitempty"`
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
