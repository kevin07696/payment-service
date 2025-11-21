package testutil

import (
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/pkg/crypto"
)

// TestServiceCredentials represents a test service with RSA keys
type TestServiceCredentials struct {
	ServiceID            string `json:"service_id"`
	ServiceName          string `json:"service_name"`
	Environment          string `json:"environment"`
	PrivateKeyPEM        string `json:"private_key_pem"`
	PublicKeyPEM         string `json:"public_key_pem"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
}

// LoadTestServices loads pre-generated test service credentials from JSON
func LoadTestServices() ([]TestServiceCredentials, error) {
	// Path is relative to test package directory (e.g., tests/integration/auth/)
	// Go up 2 levels to tests/, then into fixtures/auth/
	jsonPath := "../../fixtures/auth/test_services.json"
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}

	var services []TestServiceCredentials
	if err := json.Unmarshal(jsonData, &services); err != nil {
		return nil, err
	}

	return services, nil
}

// GenerateJWT creates a JWT signed with the service's private key
func GenerateJWT(privateKeyPEM, issuer, merchantID string, expiresIn time.Duration) (string, error) {
	// Parse private key
	privateKey, err := crypto.ParsePrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}

	// Create claims
	claims := jwt.MapClaims{
		"iss":         issuer,
		"merchant_id": merchantID,
		"exp":         time.Now().Add(expiresIn).Unix(),
		"iat":         time.Now().Unix(),
		"jti":         uuid.New().String(),
	}

	// Sign token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GenerateJWTWithClaims creates a JWT with custom claims for testing
func GenerateJWTWithClaims(privateKeyPEM string, claims jwt.MapClaims) (string, error) {
	privateKey, err := crypto.ParsePrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

// GenerateJWTWithWrongKey creates a JWT signed with a different private key (for testing invalid signatures)
func GenerateJWTWithWrongKey(issuer, merchantID string) (string, error) {
	// Generate a random key pair
	wrongKey, err := rsa.GenerateKey(cryptoRand.Reader, 2048)
	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{
		"iss":         issuer,
		"merchant_id": merchantID,
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
		"jti":         uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(wrongKey)
}
