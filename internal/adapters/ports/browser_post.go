package ports

import (
	"time"
)

// BrowserPostFormData contains all fields needed to construct the HTML form that posts to EPX
// Based on EPX Browser Post API - Required Fields (page 8-13)
type BrowserPostFormData struct {
	// EPX endpoint
	PostURL string // EPX Browser Post endpoint URL

	// Authentication
	TAC string // Terminal Authorization Code from Key Exchange

	// Transaction details
	Amount    string // Transaction amount (e.g., "29.99")
	TranNbr   string // Unique transaction number (matches TAC request)
	TranGroup string // Transaction group ID (our group_id)

	// Redirect URLs
	RedirectURL        string // Success redirect URL
	RedirectURLDecline string // Decline redirect URL (optional)
	RedirectURLError   string // Error redirect URL (optional)

	// Optional fields for display
	MerchantName string            // Merchant display name
	Metadata     map[string]string // Additional metadata to pass through
}

// BrowserPostResponse contains parsed response from EPX after redirect
// Based on EPX Browser Post API - Response Fields (page 14-17)
type BrowserPostResponse struct {
	// Core response fields
	AuthGUID     string // EPX transaction token (BRIC format) - required for refunds/voids/captures
	AuthResp     string // EPX approval code ("00" = approved, "05" = declined, "12" = invalid)
	AuthCode     string // Bank authorization code (NULL if declined)
	AuthRespText string // Human-readable response message
	IsApproved   bool   // Derived from AuthResp ("00" = true)

	// Card verification
	AuthCardType string // Card brand ("V" = Visa, "M" = Mastercard, "A" = Amex, "D" = Discover)
	AuthAVS      string // Address verification ("Y" = match, "N" = no match, "U" = unavailable)
	AuthCVV2     string // CVV verification ("M" = match, "N" = no match, "P" = not processed)

	// Transaction echo-back
	TranNbr   string // Echo back transaction number
	TranGroup string // Echo back transaction group
	Amount    string // Echo back amount

	// Timestamps
	ProcessedAt time.Time // When EPX processed the transaction

	// Raw response for debugging
	RawParams map[string]string // All URL parameters from redirect
}

// BrowserPostAdapter defines the port for Browser Post flow utilities
// Note: Browser Post flow is client-side (browser posts directly to EPX), so this adapter
// provides utilities for constructing forms and parsing responses, not making API calls
type BrowserPostAdapter interface {
	// BuildFormData constructs the form data structure needed for Browser Post
	// Returns ready-to-use form fields that can be rendered in HTML or sent to frontend
	BuildFormData(tac, amount, tranNbr, tranGroup, redirectURL string) (*BrowserPostFormData, error)

	// ParseRedirectResponse parses the query parameters from EPX redirect
	// Validates response signature and extracts transaction result
	// Returns error if:
	//   - Required fields are missing
	//   - Response signature is invalid
	//   - Response format is unexpected
	ParseRedirectResponse(params map[string][]string) (*BrowserPostResponse, error)

	// ValidateResponseMAC validates the MAC signature in the redirect response
	// Ensures the response hasn't been tampered with
	ValidateResponseMAC(params map[string][]string, mac string) error
}
