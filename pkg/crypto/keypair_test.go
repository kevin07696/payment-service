package crypto_test

import (
	"strings"
	"testing"

	"github.com/kevin07696/payment-service/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRSAKeyPair(t *testing.T) {
	kp, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)
	require.NotNil(t, kp)

	// Verify private key format (PKCS#1)
	assert.Contains(t, kp.PrivateKeyPEM, "-----BEGIN RSA PRIVATE KEY-----")
	assert.Contains(t, kp.PrivateKeyPEM, "-----END RSA PRIVATE KEY-----")
	assert.True(t, len(kp.PrivateKeyPEM) > 100, "private key should be substantial")

	// Verify public key format (PKIX)
	assert.Contains(t, kp.PublicKeyPEM, "-----BEGIN PUBLIC KEY-----")
	assert.Contains(t, kp.PublicKeyPEM, "-----END PUBLIC KEY-----")
	assert.True(t, len(kp.PublicKeyPEM) > 100, "public key should be substantial")

	// Verify fingerprint (64 hex chars = 32 bytes SHA-256)
	assert.Len(t, kp.Fingerprint, 64)
	assert.Regexp(t, "^[0-9a-f]{64}$", kp.Fingerprint, "fingerprint should be lowercase hex")

	// Verify we can parse the keys
	pubKey, err := crypto.ParsePublicKey(kp.PublicKeyPEM)
	require.NoError(t, err)
	assert.NotNil(t, pubKey)
	assert.Equal(t, 2048, pubKey.N.BitLen(), "should be 2048-bit key")

	privKey, err := crypto.ParsePrivateKey(kp.PrivateKeyPEM)
	require.NoError(t, err)
	assert.NotNil(t, privKey)
	assert.Equal(t, 2048, privKey.N.BitLen(), "should be 2048-bit key")
}

func TestGenerateRSAKeyPair_Uniqueness(t *testing.T) {
	kp1, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)

	kp2, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)

	// Each generation should produce unique keys
	assert.NotEqual(t, kp1.PrivateKeyPEM, kp2.PrivateKeyPEM, "private keys should be unique")
	assert.NotEqual(t, kp1.PublicKeyPEM, kp2.PublicKeyPEM, "public keys should be unique")
	assert.NotEqual(t, kp1.Fingerprint, kp2.Fingerprint, "fingerprints should be unique")
}

func TestParsePublicKey_Success(t *testing.T) {
	kp, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)

	pubKey, err := crypto.ParsePublicKey(kp.PublicKeyPEM)
	require.NoError(t, err)
	assert.NotNil(t, pubKey)
	assert.Equal(t, 2048, pubKey.N.BitLen())
}

func TestParsePublicKey_InvalidFormat(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errMsg string
	}{
		{
			name:   "empty string",
			input:  "",
			errMsg: "failed to parse PEM block",
		},
		{
			name:   "invalid PEM",
			input:  "not a valid PEM",
			errMsg: "failed to parse PEM block",
		},
		{
			name:   "wrong key type",
			input:  "-----BEGIN CERTIFICATE-----\nMIIC\n-----END CERTIFICATE-----",
			errMsg: "failed to parse public key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := crypto.ParsePublicKey(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestParsePrivateKey_Success(t *testing.T) {
	kp, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)

	privKey, err := crypto.ParsePrivateKey(kp.PrivateKeyPEM)
	require.NoError(t, err)
	assert.NotNil(t, privKey)
	assert.Equal(t, 2048, privKey.N.BitLen())
}

func TestParsePrivateKey_InvalidFormat(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		errMsg string
	}{
		{
			name:   "empty string",
			input:  "",
			errMsg: "failed to parse PEM block",
		},
		{
			name:   "invalid PEM",
			input:  "not a valid PEM",
			errMsg: "failed to parse PEM block",
		},
		{
			name:   "wrong key type",
			input:  "-----BEGIN PUBLIC KEY-----\nMIIB\n-----END PUBLIC KEY-----",
			errMsg: "failed to parse private key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := crypto.ParsePrivateKey(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestComputeFingerprint(t *testing.T) {
	kp, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)

	// Compute fingerprint from public key
	fingerprint, err := crypto.ComputeFingerprint(kp.PublicKeyPEM)
	require.NoError(t, err)

	// Should match the fingerprint from generation
	assert.Equal(t, kp.Fingerprint, fingerprint)

	// Should be 64 hex characters
	assert.Len(t, fingerprint, 64)
	assert.Regexp(t, "^[0-9a-f]{64}$", fingerprint)
}

func TestComputeFingerprint_Consistency(t *testing.T) {
	kp, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)

	// Computing fingerprint multiple times should give same result
	fp1, err := crypto.ComputeFingerprint(kp.PublicKeyPEM)
	require.NoError(t, err)

	fp2, err := crypto.ComputeFingerprint(kp.PublicKeyPEM)
	require.NoError(t, err)

	assert.Equal(t, fp1, fp2, "fingerprint should be deterministic")
}

func TestComputeFingerprint_InvalidPEM(t *testing.T) {
	_, err := crypto.ComputeFingerprint("not a valid PEM")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse PEM block")
}

func TestKeyPair_RoundTrip(t *testing.T) {
	// Generate keypair
	kp, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)

	// Parse both keys
	privKey, err := crypto.ParsePrivateKey(kp.PrivateKeyPEM)
	require.NoError(t, err)

	pubKey, err := crypto.ParsePublicKey(kp.PublicKeyPEM)
	require.NoError(t, err)

	// Public key from private key should match parsed public key
	assert.Equal(t, pubKey.N, privKey.PublicKey.N, "public key modulus should match")
	assert.Equal(t, pubKey.E, privKey.PublicKey.E, "public key exponent should match")
}

func TestGenerateRSAKeyPair_PEMFormat(t *testing.T) {
	kp, err := crypto.GenerateRSAKeyPair()
	require.NoError(t, err)

	// Private key should have exactly one BEGIN and END marker
	assert.Equal(t, 1, strings.Count(kp.PrivateKeyPEM, "BEGIN RSA PRIVATE KEY"))
	assert.Equal(t, 1, strings.Count(kp.PrivateKeyPEM, "END RSA PRIVATE KEY"))

	// Public key should have exactly one BEGIN and END marker
	assert.Equal(t, 1, strings.Count(kp.PublicKeyPEM, "BEGIN PUBLIC KEY"))
	assert.Equal(t, 1, strings.Count(kp.PublicKeyPEM, "END PUBLIC KEY"))

	// Keys should end with newline
	assert.True(t, strings.HasSuffix(kp.PrivateKeyPEM, "\n"))
	assert.True(t, strings.HasSuffix(kp.PublicKeyPEM, "\n"))
}
