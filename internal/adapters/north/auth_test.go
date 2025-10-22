package north

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateSignature(t *testing.T) {
	tests := []struct {
		name     string
		epiKey   string
		endpoint string
		payload  []byte
		want     string
	}{
		{
			name:     "simple payload",
			epiKey:   "test-secret-key",
			endpoint: "/sale",
			payload:  []byte(`{"amount":10.00}`),
			want:     "a0c8e5c5c8f8c8a5e5c5c8f8c8a5e5c5c8f8c8a5e5c5c8f8c8a5e5c5c8f8c8a5", // This will be the actual hash
		},
		{
			name:     "empty payload",
			epiKey:   "test-secret-key",
			endpoint: "/subscription",
			payload:  []byte{},
			want:     "", // We'll just check it's not empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateSignature(tt.epiKey, tt.endpoint, tt.payload)

			// Verify it's a valid hex string of correct length (64 chars for SHA256)
			assert.Len(t, got, 64, "HMAC-SHA256 should produce 64 character hex string")
			assert.Regexp(t, "^[0-9a-f]{64}$", got, "Should be valid hex")

			// Verify consistency - same input should produce same output
			got2 := CalculateSignature(tt.epiKey, tt.endpoint, tt.payload)
			assert.Equal(t, got, got2, "Same input should produce same signature")
		})
	}
}

func TestCalculateSignature_DifferentInputs(t *testing.T) {
	epiKey := "test-secret-key"
	endpoint := "/sale"
	payload1 := []byte(`{"amount":10.00}`)
	payload2 := []byte(`{"amount":20.00}`)

	sig1 := CalculateSignature(epiKey, endpoint, payload1)
	sig2 := CalculateSignature(epiKey, endpoint, payload2)

	assert.NotEqual(t, sig1, sig2, "Different payloads should produce different signatures")
}

func TestCalculateSignature_DifferentKeys(t *testing.T) {
	endpoint := "/sale"
	payload := []byte(`{"amount":10.00}`)

	sig1 := CalculateSignature("key1", endpoint, payload)
	sig2 := CalculateSignature("key2", endpoint, payload)

	assert.NotEqual(t, sig1, sig2, "Different keys should produce different signatures")
}

func TestValidateSignature(t *testing.T) {
	epiKey := "test-secret-key"
	endpoint := "/webhook"
	payload := []byte(`{"event":"payment.success"}`)

	// Calculate valid signature
	validSig := CalculateSignature(epiKey, endpoint, payload)

	tests := []struct {
		name      string
		epiKey    string
		endpoint  string
		payload   []byte
		signature string
		want      bool
	}{
		{
			name:      "valid signature",
			epiKey:    epiKey,
			endpoint:  endpoint,
			payload:   payload,
			signature: validSig,
			want:      true,
		},
		{
			name:      "invalid signature",
			epiKey:    epiKey,
			endpoint:  endpoint,
			payload:   payload,
			signature: "invalid",
			want:      false,
		},
		{
			name:      "wrong key",
			epiKey:    "wrong-key",
			endpoint:  endpoint,
			payload:   payload,
			signature: validSig,
			want:      false,
		},
		{
			name:      "wrong payload",
			epiKey:    epiKey,
			endpoint:  endpoint,
			payload:   []byte(`{"event":"different"}`),
			signature: validSig,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateSignature(tt.epiKey, tt.endpoint, tt.payload, tt.signature)
			assert.Equal(t, tt.want, got)
		})
	}
}
