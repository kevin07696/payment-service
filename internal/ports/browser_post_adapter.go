package ports

import "time"

// BrowserPostResponse contains parsed response from EPX after redirect.
// Based on EPX Browser Post API Integration Guide - Response Fields (page 7).
type BrowserPostResponse struct {
	// Core response fields
	AuthGUID     string // EPX transaction token (BRIC format) - AUTH_GUID
	AuthResp     string // EPX approval code ("00" = approved) - AUTH_RESP
	AuthCode     string // Bank authorization code - AUTH_CODE
	AuthRespText string // Human-readable response message - AUTH_RESP_TEXT
	IsApproved   bool   // Derived from AuthResp ("00" = true)

	// Card verification
	AuthCardType string // Card brand ("V" = Visa, "M" = Mastercard, "A" = Amex, "D" = Discover) - AUTH_CARD_TYPE
	AuthAVS      string // Address verification ("Y" = match, "N" = no match, "U" = unavailable) - AUTH_AVS
	AuthCVV2     string // CVV verification ("M" = match, "N" = no match, "P" = not processed) - AUTH_CVV2

	// Transaction echo-back
	TranNbr   string // Echo back transaction number - TRAN_NBR
	TranGroup string // Echo back transaction group - TRAN_GROUP
	Amount    string // Echo back amount - AUTH_AMOUNT

	// Timestamps
	ProcessedAt time.Time // When EPX processed the transaction

	// Raw response for debugging
	RawParams map[string]string // All URL parameters from redirect
}

// BrowserPostAdapter defines the port for Browser Post flow utilities.
// Note: Browser Post flow is client-side (browser posts directly to EPX), so this adapter
// provides utilities for parsing responses, not making API calls.
type BrowserPostAdapter interface {
	// ParseRedirectResponse parses the query parameters from EPX redirect.
	ParseRedirectResponse(params map[string][]string) (*BrowserPostResponse, error)
}
