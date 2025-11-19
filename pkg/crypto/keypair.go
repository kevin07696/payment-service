package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
)

// KeyPair represents an RSA keypair with fingerprint.
type KeyPair struct {
	PrivateKeyPEM string
	PublicKeyPEM  string
	Fingerprint   string
}

// GenerateRSAKeyPair generates a new 2048-bit RSA keypair.
// Returns the keypair with PEM-encoded keys and SHA-256 fingerprint.
func GenerateRSAKeyPair() (*KeyPair, error) {
	// Generate 2048-bit RSA keypair (industry standard for JWT)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Encode private key to PKCS#1 PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}))

	// Encode public key to PKIX PEM format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}))

	// Generate fingerprint (SHA-256 hash of public key DER bytes)
	hash := sha256.Sum256(publicKeyBytes)
	fingerprint := hex.EncodeToString(hash[:])

	return &KeyPair{
		PrivateKeyPEM: privateKeyPEM,
		PublicKeyPEM:  publicKeyPEM,
		Fingerprint:   fingerprint,
	}, nil
}

// ParsePublicKey parses a PEM-encoded RSA public key.
func ParsePublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

// ParsePrivateKey parses a PEM-encoded RSA private key.
func ParsePrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKey, nil
}

// ComputeFingerprint computes the SHA-256 fingerprint of a public key PEM.
func ComputeFingerprint(publicKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to parse PEM block")
	}

	hash := sha256.Sum256(block.Bytes)
	return hex.EncodeToString(hash[:]), nil
}
