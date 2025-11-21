package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims represents the standard claims for our JWT tokens
type JWTClaims struct {
	jwt.RegisteredClaims
	MerchantID   string   `json:"merchant_id"`
	ServiceID    string   `json:"service_id,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Environment  string   `json:"env,omitempty"`
	RefreshToken bool     `json:"refresh,omitempty"`
}

// JWTManager handles JWT token generation and validation
type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
	expiry     time.Duration
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(privateKeyPEM []byte, issuer string, expiry time.Duration) (*JWTManager, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("key is not an RSA private key")
		}
	}

	return &JWTManager{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		issuer:     issuer,
		expiry:     expiry,
	}, nil
}

// GenerateToken generates a new JWT token
func (jm *JWTManager) GenerateToken(merchantID string, scopes []string) (string, error) {
	now := time.Now()
	jti := uuid.New().String()

	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jm.issuer,
			Subject:   merchantID,
			ExpiresAt: jwt.NewNumericDate(now.Add(jm.expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
		MerchantID:  merchantID,
		ServiceID:   jm.issuer,
		Scopes:      scopes,
		Environment: "production",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(jm.privateKey)
}

// GenerateRefreshToken generates a refresh token with longer expiry
func (jm *JWTManager) GenerateRefreshToken(merchantID string) (string, error) {
	now := time.Now()
	jti := uuid.New().String()

	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jm.issuer,
			Subject:   merchantID,
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)), // 24 hour refresh token
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
		MerchantID:   merchantID,
		ServiceID:    jm.issuer,
		RefreshToken: true,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(jm.privateKey)
}

// ValidateToken validates a JWT token
func (jm *JWTManager) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jm.publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	return claims, nil
}

// GetPublicKeyPEM returns the public key in PEM format
func (jm *JWTManager) GetPublicKeyPEM() ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(jm.publicKey)
	if err != nil {
		return nil, err
	}

	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	return pem.EncodeToMemory(publicKeyPEM), nil
}

// GenerateRSAKeyPair generates a new RSA key pair
func GenerateRSAKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

// PrivateKeyToPEM converts an RSA private key to PEM format
func PrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	return pem.EncodeToMemory(privateKeyPEM)
}

// PublicKeyToPEM converts an RSA public key to PEM format
func PublicKeyToPEM(publicKey *rsa.PublicKey) ([]byte, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	return pem.EncodeToMemory(publicKeyPEM), nil
}

// ParsePublicKeyFromPEM parses an RSA public key from PEM format
func ParsePublicKeyFromPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	publicKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return publicKey, nil
}

// ValidateScopes checks if the provided scopes match the required scopes
func ValidateScopes(providedScopes, requiredScopes []string) bool {
	scopeMap := make(map[string]bool)
	for _, scope := range providedScopes {
		scopeMap[scope] = true
	}

	for _, required := range requiredScopes {
		if !scopeMap[required] {
			return false
		}
	}

	return true
}
