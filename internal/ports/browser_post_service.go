package ports

import (
	"context"
	"time"

	"github.com/kevin07696/payment-service/internal/domain"
)

// GenerateFormConfigRequest contains parameters for generating Browser Post form configuration.
type GenerateFormConfigRequest struct {
	TransactionID   string // Frontend-generated UUID
	MerchantID      string // Merchant UUID
	Amount          string // Amount in decimal format (e.g., "10.00")
	TransactionType string // SALE, AUTH, STORAGE, ACH_STORAGE_C, ACH_STORAGE_S
	CustomerID      string // Optional customer ID (required for STORAGE)
	ReturnURL       string // URL to redirect after payment
}

// GenerateFormConfigResponse contains the form configuration for Browser Post.
type GenerateFormConfigResponse struct {
	TransactionID string
	EPXTranNbr    string
	TAC           string
	ExpiresAt     time.Time
	PostURL       string
	CustNbr       string
	MerchNbr      string
	DBAName       string
	TerminalNbr   string
	IndustryType  string
	TranCode      string // EPX TRAN_CODE for Browser POST form (SALE, AUTH, STORAGE, ACH_STORAGE_C, ACH_STORAGE_S)
	RedirectURL   string
	ReturnURL     string
	MerchantID    string
	MerchantName  string
}

// ProcessCallbackRequest contains parameters for processing Browser Post callback.
type ProcessCallbackRequest struct {
	// Parsed from EPX response
	TranNbr      string
	AuthGUID     string
	AuthResp     string
	AuthCode     string
	AuthCardType string
	AuthRespText string
	AuthAVS      string
	AuthCVV2     string
	Amount       string
	IsApproved   bool

	// From REDIRECT_URL query params
	TransactionID   string
	MerchantID      string
	TransactionType domain.RequestTransactionType
	CustomerID      string

	// Raw params for additional fields
	RawParams map[string]string
}

// ProcessCallbackResponse contains the result of processing a callback.
type ProcessCallbackResponse struct {
	TransactionID       string
	ParentTransactionID string
	Status              string
	PaymentMethodID     string
	ReturnURL           string
}

// BrowserPostService defines the port for Browser Post operations.
type BrowserPostService interface {
	// GenerateFormConfig validates merchant, gets TAC, creates pending transaction, and returns form config.
	GenerateFormConfig(ctx context.Context, req *GenerateFormConfigRequest) (*GenerateFormConfigResponse, error)

	// ParseRedirectResponse parses the query parameters from EPX redirect into a structured response.
	ParseRedirectResponse(params map[string][]string) (*BrowserPostResponse, error)

	// ProcessCallback updates transaction with EPX response.
	ProcessCallback(ctx context.Context, req *ProcessCallbackRequest) (*ProcessCallbackResponse, error)
}
