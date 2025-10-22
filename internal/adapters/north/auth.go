package north

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// AuthConfig holds HMAC authentication configuration for North APIs
type AuthConfig struct {
	EPIId  string // Four-part key: CUST_NBR-MERCH_NBR-DBA_NBR-TERMINAL_NBR
	EPIKey string // Shared secret for HMAC signing
}

// CalculateSignature calculates HMAC-SHA256 signature for North API requests
// Signature = HMAC-SHA256(endpoint + payload, EPIKey)
func CalculateSignature(epiKey, endpoint string, payloadBytes []byte) string {
	// Concatenate endpoint and payload
	concat := append([]byte(endpoint), payloadBytes...)

	// Calculate HMAC-SHA256
	h := hmac.New(sha256.New, []byte(epiKey))
	h.Write(concat)

	// Return hex-encoded signature
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateSignature validates an HMAC signature (used for webhooks)
func ValidateSignature(epiKey, endpoint string, payloadBytes []byte, signature string) bool {
	expected := CalculateSignature(epiKey, endpoint, payloadBytes)
	return hmac.Equal([]byte(expected), []byte(signature))
}
