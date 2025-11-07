package epx

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// BRICStorageConfig contains configuration for EPX BRIC Storage adapter
type BRICStorageConfig struct {
	// Base URL for BRIC Storage requests
	// Sandbox: https://epxnow.com/epx/server_post_sandbox
	// Production: https://epxnow.com/epx/server_post
	// Note: BRIC Storage uses the same endpoint as Server Post
	BaseURL string

	// HTTP client timeout
	Timeout time.Duration

	// TLS configuration
	InsecureSkipVerify bool

	// Retry configuration
	MaxRetries      int
	RetryDelay      time.Duration
	RetryableErrors []string
}

// DefaultBRICStorageConfig returns default configuration for BRIC Storage adapter
func DefaultBRICStorageConfig(environment string) *BRICStorageConfig {
	baseURL := "https://epxnow.com/epx/server_post" // Production
	if environment == "sandbox" {
		baseURL = "https://secure.epxuap.com"
	}

	return &BRICStorageConfig{
		BaseURL:            baseURL,
		Timeout:            30 * time.Second,
		InsecureSkipVerify: environment == "sandbox",
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		RetryableErrors:    []string{"timeout", "connection", "temporary"},
	}
}

// bricStorageAdapter implements the BRICStorageAdapter port
type bricStorageAdapter struct {
	config     *BRICStorageConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewBRICStorageAdapter creates a new EPX BRIC Storage adapter
func NewBRICStorageAdapter(config *BRICStorageConfig, logger *zap.Logger) ports.BRICStorageAdapter {
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

	return &bricStorageAdapter{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
	}
}

// ConvertFinancialBRICToStorage converts a Financial BRIC to a Storage BRIC
// Based on EPX Transaction Specs - BRIC Storage.pdf (page 12)
func (a *bricStorageAdapter) ConvertFinancialBRICToStorage(ctx context.Context, req *ports.BRICStorageRequest) (*ports.BRICStorageResponse, error) {
	if req.FinancialBRIC == nil || *req.FinancialBRIC == "" {
		return nil, fmt.Errorf("financial_bric is required for conversion")
	}

	a.logger.Info("Converting Financial BRIC to Storage BRIC",
		zap.String("financial_bric", *req.FinancialBRIC),
		zap.String("payment_type", string(req.PaymentType)),
		zap.String("tran_nbr", req.TranNbr),
	)

	// Build XML request for BRIC Storage conversion
	xmlRequest := a.buildConversionXML(req)

	// Send request to EPX
	response, err := a.sendRequest(ctx, xmlRequest, req)
	if err != nil {
		a.logger.Error("Failed to convert Financial BRIC to Storage BRIC", zap.Error(err))
		return nil, fmt.Errorf("conversion failed: %w", err)
	}

	a.logger.Info("Successfully converted Financial BRIC to Storage BRIC",
		zap.String("storage_bric", response.StorageBRIC),
		zap.Bool("is_approved", response.IsApproved),
	)

	return response, nil
}

// CreateStorageBRICFromAccount creates a Storage BRIC from account information
// Based on EPX Transaction Specs - BRIC Storage.pdf (page 8)
func (a *bricStorageAdapter) CreateStorageBRICFromAccount(ctx context.Context, req *ports.BRICStorageRequest) (*ports.BRICStorageResponse, error) {
	if req.AccountNumber == nil || *req.AccountNumber == "" {
		return nil, fmt.Errorf("account_number is required for creating Storage BRIC from account")
	}

	a.logger.Info("Creating Storage BRIC from account information",
		zap.String("payment_type", string(req.PaymentType)),
		zap.String("tran_nbr", req.TranNbr),
	)

	// Build XML request for BRIC Storage creation
	xmlRequest := a.buildAccountXML(req)

	// Send request to EPX
	response, err := a.sendRequest(ctx, xmlRequest, req)
	if err != nil {
		a.logger.Error("Failed to create Storage BRIC from account", zap.Error(err))
		return nil, fmt.Errorf("creation failed: %w", err)
	}

	a.logger.Info("Successfully created Storage BRIC from account",
		zap.String("storage_bric", response.StorageBRIC),
		zap.Bool("is_approved", response.IsApproved),
	)

	return response, nil
}

// UpdateStorageBRIC updates reference data for an existing Storage BRIC
// Based on EPX Transaction Specs - BRIC Storage.pdf (page 10)
func (a *bricStorageAdapter) UpdateStorageBRIC(ctx context.Context, req *ports.BRICStorageRequest) (*ports.BRICStorageResponse, error) {
	if req.FinancialBRIC == nil || *req.FinancialBRIC == "" {
		return nil, fmt.Errorf("financial_bric (original Storage BRIC) is required for update")
	}

	a.logger.Info("Updating Storage BRIC reference data",
		zap.String("storage_bric", *req.FinancialBRIC),
		zap.String("tran_nbr", req.TranNbr),
	)

	// Build XML request for BRIC Storage update
	xmlRequest := a.buildUpdateXML(req)

	// Send request to EPX
	response, err := a.sendRequest(ctx, xmlRequest, req)
	if err != nil {
		a.logger.Error("Failed to update Storage BRIC", zap.Error(err))
		return nil, fmt.Errorf("update failed: %w", err)
	}

	a.logger.Warn("Storage BRIC updated - IMPORTANT: Continue using original BRIC",
		zap.String("original_bric", *req.FinancialBRIC),
		zap.String("new_bric_returned", response.StorageBRIC),
	)

	return response, nil
}

// buildConversionXML builds XML for converting Financial BRIC to Storage BRIC
// Based on EPX Transaction Specs - BRIC Storage.pdf (page 12)
func (a *bricStorageAdapter) buildConversionXML(req *ports.BRICStorageRequest) string {
	tranType := "CCE8" // Credit card ecommerce
	if req.PaymentType == ports.PaymentMethodTypeACH {
		tranType = "CKC8" // ACH checking
	}

	cardEntMeth := "Z" // BRIC-based transaction

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(fmt.Sprintf(`<DETAIL cust_nbr="%s" merch_nbr="%s" dba_nbr="%s" terminal_nbr="%s">`,
		req.CustNbr, req.MerchNbr, req.DBAnbr, req.TerminalNbr))
	sb.WriteString(fmt.Sprintf(`<TRAN_TYPE>%s</TRAN_TYPE>`, tranType))
	sb.WriteString(fmt.Sprintf(`<BATCH_ID>%s</BATCH_ID>`, req.BatchID))
	sb.WriteString(fmt.Sprintf(`<TRAN_NBR>%s</TRAN_NBR>`, req.TranNbr))
	sb.WriteString(fmt.Sprintf(`<ORIG_AUTH_GUID>%s</ORIG_AUTH_GUID>`, *req.FinancialBRIC))
	sb.WriteString(fmt.Sprintf(`<CARD_ENT_METH>%s</CARD_ENT_METH>`, cardEntMeth))
	sb.WriteString(`<INDUSTRY_TYPE>E</INDUSTRY_TYPE>`) // Ecommerce

	// For credit cards: include billing info for Account Verification
	if req.PaymentType == ports.PaymentMethodTypeCreditCard {
		if req.Address != nil {
			sb.WriteString(fmt.Sprintf(`<ADDRESS>%s</ADDRESS>`, xmlEscape(*req.Address)))
		}
		if req.ZipCode != nil {
			sb.WriteString(fmt.Sprintf(`<ZIP_CODE>%s</ZIP_CODE>`, xmlEscape(*req.ZipCode)))
		}
		if req.City != nil {
			sb.WriteString(fmt.Sprintf(`<CITY>%s</CITY>`, xmlEscape(*req.City)))
		}
		if req.State != nil {
			sb.WriteString(fmt.Sprintf(`<STATE>%s</STATE>`, xmlEscape(*req.State)))
		}
	}

	// Optional fields
	if req.FirstName != nil {
		sb.WriteString(fmt.Sprintf(`<FIRST_NAME>%s</FIRST_NAME>`, xmlEscape(*req.FirstName)))
	}
	if req.LastName != nil {
		sb.WriteString(fmt.Sprintf(`<LAST_NAME>%s</LAST_NAME>`, xmlEscape(*req.LastName)))
	}
	if req.UserData1 != nil {
		sb.WriteString(fmt.Sprintf(`<USER_DATA_1>%s</USER_DATA_1>`, xmlEscape(*req.UserData1)))
	}

	sb.WriteString(`</DETAIL>`)
	return sb.String()
}

// buildAccountXML builds XML for creating Storage BRIC from account information
// Based on EPX Transaction Specs - BRIC Storage.pdf (page 8)
func (a *bricStorageAdapter) buildAccountXML(req *ports.BRICStorageRequest) string {
	tranType := "CCE8" // Credit card ecommerce
	cardEntMeth := "E" // Account-based transaction

	if req.PaymentType == ports.PaymentMethodTypeACH {
		tranType = "CKC8" // ACH checking
		cardEntMeth = "X" // ACH entry method
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(fmt.Sprintf(`<DETAIL cust_nbr="%s" merch_nbr="%s" dba_nbr="%s" terminal_nbr="%s">`,
		req.CustNbr, req.MerchNbr, req.DBAnbr, req.TerminalNbr))
	sb.WriteString(fmt.Sprintf(`<TRAN_TYPE>%s</TRAN_TYPE>`, tranType))
	sb.WriteString(fmt.Sprintf(`<BATCH_ID>%s</BATCH_ID>`, req.BatchID))
	sb.WriteString(fmt.Sprintf(`<TRAN_NBR>%s</TRAN_NBR>`, req.TranNbr))
	sb.WriteString(fmt.Sprintf(`<ACCOUNT_NBR>%s</ACCOUNT_NBR>`, *req.AccountNumber))
	sb.WriteString(fmt.Sprintf(`<CARD_ENT_METH>%s</CARD_ENT_METH>`, cardEntMeth))
	sb.WriteString(`<INDUSTRY_TYPE>E</INDUSTRY_TYPE>`)

	// Credit card specific fields
	if req.PaymentType == ports.PaymentMethodTypeCreditCard {
		if req.ExpirationDate != nil {
			sb.WriteString(fmt.Sprintf(`<EXP_DATE>%s</EXP_DATE>`, *req.ExpirationDate))
		}
		if req.CVV != nil {
			sb.WriteString(fmt.Sprintf(`<CVV2>%s</CVV2>`, *req.CVV))
		}
		if req.Address != nil {
			sb.WriteString(fmt.Sprintf(`<ADDRESS>%s</ADDRESS>`, xmlEscape(*req.Address)))
		}
		if req.ZipCode != nil {
			sb.WriteString(fmt.Sprintf(`<ZIP_CODE>%s</ZIP_CODE>`, xmlEscape(*req.ZipCode)))
		}
		if req.City != nil {
			sb.WriteString(fmt.Sprintf(`<CITY>%s</CITY>`, xmlEscape(*req.City)))
		}
		if req.State != nil {
			sb.WriteString(fmt.Sprintf(`<STATE>%s</STATE>`, xmlEscape(*req.State)))
		}
	}

	// ACH specific fields
	if req.PaymentType == ports.PaymentMethodTypeACH {
		if req.RoutingNumber != nil {
			sb.WriteString(fmt.Sprintf(`<ROUTING_NBR>%s</ROUTING_NBR>`, *req.RoutingNumber))
		}
	}

	// Common fields
	if req.FirstName != nil {
		sb.WriteString(fmt.Sprintf(`<FIRST_NAME>%s</FIRST_NAME>`, xmlEscape(*req.FirstName)))
	}
	if req.LastName != nil {
		sb.WriteString(fmt.Sprintf(`<LAST_NAME>%s</LAST_NAME>`, xmlEscape(*req.LastName)))
	}
	if req.UserData1 != nil {
		sb.WriteString(fmt.Sprintf(`<USER_DATA_1>%s</USER_DATA_1>`, xmlEscape(*req.UserData1)))
	}

	sb.WriteString(`</DETAIL>`)
	return sb.String()
}

// buildUpdateXML builds XML for updating Storage BRIC reference data
// Based on EPX Transaction Specs - BRIC Storage.pdf (page 10)
func (a *bricStorageAdapter) buildUpdateXML(req *ports.BRICStorageRequest) string {
	tranType := "CCE8" // Credit card ecommerce
	if req.PaymentType == ports.PaymentMethodTypeACH {
		tranType = "CKC8" // ACH checking
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(fmt.Sprintf(`<DETAIL cust_nbr="%s" merch_nbr="%s" dba_nbr="%s" terminal_nbr="%s">`,
		req.CustNbr, req.MerchNbr, req.DBAnbr, req.TerminalNbr))
	sb.WriteString(fmt.Sprintf(`<TRAN_TYPE>%s</TRAN_TYPE>`, tranType))
	sb.WriteString(fmt.Sprintf(`<BATCH_ID>%s</BATCH_ID>`, req.BatchID))
	sb.WriteString(fmt.Sprintf(`<TRAN_NBR>%s</TRAN_NBR>`, req.TranNbr))
	sb.WriteString(fmt.Sprintf(`<ORIG_AUTH_GUID>%s</ORIG_AUTH_GUID>`, *req.FinancialBRIC))

	// Only include fields that need updating
	// Non-PCI updates (don't trigger Account Verification)
	if req.FirstName != nil {
		sb.WriteString(fmt.Sprintf(`<FIRST_NAME>%s</FIRST_NAME>`, xmlEscape(*req.FirstName)))
	}
	if req.LastName != nil {
		sb.WriteString(fmt.Sprintf(`<LAST_NAME>%s</LAST_NAME>`, xmlEscape(*req.LastName)))
	}
	if req.City != nil {
		sb.WriteString(fmt.Sprintf(`<CITY>%s</CITY>`, xmlEscape(*req.City)))
	}
	if req.State != nil {
		sb.WriteString(fmt.Sprintf(`<STATE>%s</STATE>`, xmlEscape(*req.State)))
	}
	if req.UserData1 != nil {
		sb.WriteString(fmt.Sprintf(`<USER_DATA_1>%s</USER_DATA_1>`, xmlEscape(*req.UserData1)))
	}

	// PCI updates (trigger Account Verification for credit cards)
	if req.Address != nil {
		sb.WriteString(fmt.Sprintf(`<ADDRESS>%s</ADDRESS>`, xmlEscape(*req.Address)))
	}
	if req.ZipCode != nil {
		sb.WriteString(fmt.Sprintf(`<ZIP_CODE>%s</ZIP_CODE>`, xmlEscape(*req.ZipCode)))
	}
	if req.ExpirationDate != nil {
		sb.WriteString(fmt.Sprintf(`<EXP_DATE>%s</EXP_DATE>`, *req.ExpirationDate))
	}

	sb.WriteString(`</DETAIL>`)
	return sb.String()
}

// BRICStorageXMLResponse represents the XML response from EPX BRIC Storage
type BRICStorageXMLResponse struct {
	XMLName              xml.Name `xml:"DETAIL"`
	AuthGUID             string   `xml:"AUTH_GUID"`
	AuthResp             string   `xml:"AUTH_RESP"`
	AuthRespText         string   `xml:"AUTH_RESP_TEXT"`
	AuthCode             string   `xml:"AUTH_CODE"`
	AuthAVS              string   `xml:"AUTH_AVS"`
	AuthCVV2             string   `xml:"AUTH_CVV2"`
	AuthCardType         string   `xml:"AUTH_CARD_TYPE"`
	NetworkTransactionID string   `xml:"NTID"` // Network Transaction ID
	TranNbr              string   `xml:"TRAN_NBR"`
	BatchID              string   `xml:"BATCH_ID"`
}

// sendRequest sends the XML request to EPX and parses the response
func (a *bricStorageAdapter) sendRequest(ctx context.Context, xmlRequest string, req *ports.BRICStorageRequest) (*ports.BRICStorageResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= a.config.MaxRetries; attempt++ {
		if attempt > 0 {
			a.logger.Info("Retrying BRIC Storage request",
				zap.Int("attempt", attempt),
				zap.Int("max_retries", a.config.MaxRetries),
			)
			time.Sleep(a.config.RetryDelay)
		}

		// Create HTTP request
		httpReq, err := http.NewRequestWithContext(ctx, "POST", a.config.BaseURL, strings.NewReader(xmlRequest))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/xml")

		// Send request
		startTime := time.Now()
		httpResp, err := a.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			if a.isRetryable(err) && attempt < a.config.MaxRetries {
				a.logger.Warn("Retryable error occurred", zap.Error(err))
				continue
			}
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
		defer httpResp.Body.Close()

		// Read response
		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		a.logger.Info("Received BRIC Storage response",
			zap.Int("status_code", httpResp.StatusCode),
			zap.Duration("elapsed", time.Since(startTime)),
		)

		// Parse XML response
		var xmlResp BRICStorageXMLResponse
		if err := xml.Unmarshal(body, &xmlResp); err != nil {
			a.logger.Error("Failed to parse XML response",
				zap.Error(err),
				zap.String("body", string(body)),
			)
			return nil, fmt.Errorf("failed to parse XML response: %w", err)
		}

		// Build response
		response := a.buildResponse(&xmlResp, string(body))

		return response, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", a.config.MaxRetries, lastErr)
}

// buildResponse builds the BRICStorageResponse from XML response
func (a *bricStorageAdapter) buildResponse(xmlResp *BRICStorageXMLResponse, rawXML string) *ports.BRICStorageResponse {
	// AUTH_RESP "00" = approved, "85" = not declined (treated as approval)
	isApproved := xmlResp.AuthResp == "00" || xmlResp.AuthResp == "85"

	response := &ports.BRICStorageResponse{
		StorageBRIC:  xmlResp.AuthGUID,
		AuthResp:     xmlResp.AuthResp,
		AuthRespText: xmlResp.AuthRespText,
		IsApproved:   isApproved,
		TranNbr:      xmlResp.TranNbr,
		BatchID:      xmlResp.BatchID,
		RawXML:       rawXML,
	}

	// Network Transaction ID (credit cards only)
	if xmlResp.NetworkTransactionID != "" {
		response.NetworkTransactionID = &xmlResp.NetworkTransactionID
	}

	// Account Verification results (credit cards)
	if xmlResp.AuthAVS != "" {
		response.AuthAVS = &xmlResp.AuthAVS
	}
	if xmlResp.AuthCVV2 != "" {
		response.AuthCVV2 = &xmlResp.AuthCVV2
	}
	if xmlResp.AuthCardType != "" {
		response.AuthCardType = &xmlResp.AuthCardType
	}

	return response
}

// isRetryable determines if an error should trigger a retry
func (a *bricStorageAdapter) isRetryable(err error) bool {
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

// xmlEscape escapes special XML characters
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
