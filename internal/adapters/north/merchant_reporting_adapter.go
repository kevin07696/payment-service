package north

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kevin07696/payment-service/internal/ports"
)

// MerchantReportingConfig contains configuration for merchant reporting adapter
type MerchantReportingConfig struct {
	BaseURL string // e.g., "https://api.north.com"
	Timeout time.Duration
}

// merchantReportingAdapter implements the MerchantReportingAdapter port
type merchantReportingAdapter struct {
	config     *MerchantReportingConfig
	httpClient ports.HTTPClient
	logger     ports.Logger
}

// NewMerchantReportingAdapter creates a new merchant reporting adapter
func NewMerchantReportingAdapter(
	config *MerchantReportingConfig,
	httpClient ports.HTTPClient,
	logger ports.Logger,
) ports.MerchantReportingAdapter {
	return &merchantReportingAdapter{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
	}
}

// North API response structures
type northDisputeSearchResponse struct {
	Status string `json:"status"`
	Data   struct {
		Disputes []struct {
			CaseNumber         string  `json:"caseNumber"`
			DisputeDate        string  `json:"disputeDate"`
			ChargebackDate     string  `json:"chargebackDate"`
			DisputeType        string  `json:"disputeType"`
			Status             string  `json:"status"`
			CardBrand          string  `json:"cardBrand"`
			CardNumberLastFour string  `json:"cardNumberLastFour"`
			TransactionNumber  string  `json:"transactionNumber"`
			ReasonCode         string  `json:"reasonCode"`
			ReasonDescription  string  `json:"reasonDescription"`
			TransactionAmount  float64 `json:"transactionAmount"`
			TransactionDate    string  `json:"transactionDate"`
			ChargebackAmount   float64 `json:"chargebackAmount"`
		} `json:"disputes"`
		Meta struct {
			TotalDisputes      int `json:"totalDisputes"`
			CurrentResultCount int `json:"currentResultCount"`
		} `json:"meta"`
	} `json:"data"`
	Link string `json:"link"`
}

// SearchDisputes retrieves dispute/chargeback data for a merchant
func (a *merchantReportingAdapter) SearchDisputes(ctx context.Context, req *ports.DisputeSearchRequest) (*ports.DisputeSearchResponse, error) {
	a.logger.Info("Searching disputes",
		ports.String("merchant_id", req.MerchantID),
	)

	// Build findBy parameter
	findBy := fmt.Sprintf("byMerchant:%s", req.MerchantID)

	if req.FromDate != nil {
		findBy += fmt.Sprintf(",fromDate:%s", req.FromDate.Format("2006-01-02"))
	}

	if req.ToDate != nil {
		findBy += fmt.Sprintf(",toDate:%s", req.ToDate.Format("2006-01-02"))
	}

	// Build request URL
	endpoint := fmt.Sprintf("%s/merchant/disputes/mid/search", a.config.BaseURL)
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	// Add query parameters
	query := reqURL.Query()
	query.Set("findBy", findBy)
	reqURL.RawQuery = query.Encode()

	a.logger.Info("Calling North Dispute API",
		ports.String("url", reqURL.String()),
		ports.String("findBy", findBy),
	)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Accept", "application/json")

	// Execute request
	startTime := time.Now()
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		a.logger.Error("North Dispute API request failed",
			ports.Err(err),
			ports.String("elapsed", time.Since(startTime).String()),
		)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	a.logger.Info("North Dispute API response",
		ports.Int("status_code", resp.StatusCode),
		ports.String("elapsed", time.Since(startTime).String()),
	)

	// Check status code
	if resp.StatusCode != http.StatusOK {
		a.logger.Error("North Dispute API returned non-200 status",
			ports.Int("status_code", resp.StatusCode),
			ports.String("response_body", string(body)),
		)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var northResp northDisputeSearchResponse
	if err := json.Unmarshal(body, &northResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Check response status
	if northResp.Status != "success" {
		return nil, fmt.Errorf("API returned non-success status: %s", northResp.Status)
	}

	// Convert to domain model
	disputes := make([]*ports.Dispute, len(northResp.Data.Disputes))
	for i, d := range northResp.Data.Disputes {
		disputes[i] = &ports.Dispute{
			CaseNumber:         d.CaseNumber,
			DisputeDate:        d.DisputeDate,
			ChargebackDate:     d.ChargebackDate,
			DisputeType:        d.DisputeType,
			Status:             d.Status,
			CardBrand:          d.CardBrand,
			CardNumberLastFour: d.CardNumberLastFour,
			TransactionNumber:  d.TransactionNumber,
			ReasonCode:         d.ReasonCode,
			ReasonDescription:  d.ReasonDescription,
			TransactionAmount:  d.TransactionAmount,
			TransactionDate:    d.TransactionDate,
			ChargebackAmount:   d.ChargebackAmount,
		}
	}

	a.logger.Info("Disputes retrieved successfully",
		ports.Int("total_disputes", northResp.Data.Meta.TotalDisputes),
		ports.Int("current_result_count", northResp.Data.Meta.CurrentResultCount),
	)

	return &ports.DisputeSearchResponse{
		Disputes:           disputes,
		TotalDisputes:      northResp.Data.Meta.TotalDisputes,
		CurrentResultCount: northResp.Data.Meta.CurrentResultCount,
	}, nil
}
