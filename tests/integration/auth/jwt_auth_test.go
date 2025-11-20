//go:build integration
// +build integration

package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/require"
)

// TestJWTAuthentication_ValidToken tests JWT authentication with valid RSA-signed token
func TestJWTAuthentication_ValidToken(t *testing.T) {
	t.Skip("TODO: Implement JWT authentication test - requires test service setup and auth enabled")

	// This test requires:
	// 1. Generate RSA key pair
	// 2. Insert test service into services table with public key
	// 3. Create service_merchants relationship
	// 4. Generate valid JWT signed with private key
	// 5. Make authenticated request
	// 6. Verify request succeeds

	_, _ = testutil.Setup(t)
	db := testutil.GetDB(t)

	// Generate RSA key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	publicKeyPEM := exportPublicKeyAsPEM(&privateKey.PublicKey)

	// Insert test service
	serviceID := "test-service-" + uuid.New().String()
	testServiceID := insertTestService(t, db, serviceID, publicKeyPEM)
	defer cleanupTestService(t, db, testServiceID)

	// Link service to merchant
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant
	linkServiceToMerchant(t, db, testServiceID, merchantID)

	// Generate JWT token
	_ = generateJWT(t, privateKey, serviceID, merchantID)

	// TODO: Enable authentication temporarily for this test
	// Make authenticated request
	// client.AddHeader("Authorization", "Bearer " + token)
	// resp, err := client.Do("POST", "/payment.v1.PaymentService/GetTransaction", req)
	// require.NoError(t, err)
	// assert.Equal(t, 200, resp.StatusCode)
}

// TestJWTAuthentication_InvalidSignature tests JWT with wrong signature is rejected
func TestJWTAuthentication_InvalidSignature(t *testing.T) {
	t.Skip("TODO: Implement invalid JWT signature test")

	// This test verifies that JWT signed with wrong private key is rejected
	// Expected: HTTP 401 Unauthorized
}

// TestJWTAuthentication_ExpiredToken tests expired JWT is rejected
func TestJWTAuthentication_ExpiredToken(t *testing.T) {
	t.Skip("TODO: Implement expired JWT test")

	// This test verifies that JWT with past expiration is rejected
	// Expected: HTTP 401 Unauthorized with "token expired" message
}

// TestJWTAuthentication_MissingIssuer tests JWT without issuer is rejected
func TestJWTAuthentication_MissingIssuer(t *testing.T) {
	t.Skip("TODO: Implement missing issuer test")

	// This test verifies that JWT without "iss" claim is rejected
	// Expected: HTTP 401 Unauthorized with "missing issuer" message
}

// TestJWTAuthentication_UnknownIssuer tests JWT from unknown service is rejected
func TestJWTAuthentication_UnknownIssuer(t *testing.T) {
	t.Skip("TODO: Implement unknown issuer test")

	// This test verifies that JWT from service not in database is rejected
	// Expected: HTTP 401 Unauthorized with "unknown issuer" message
}

// TestJWTAuthentication_NoMerchantAccess tests JWT for merchant without access is rejected
func TestJWTAuthentication_NoMerchantAccess(t *testing.T) {
	t.Skip("TODO: Implement no merchant access test")

	// This test verifies service can't access merchant it's not linked to
	// Expected: HTTP 401 Unauthorized with "access denied" message
}

// TestJWTAuthentication_BlacklistedToken tests blacklisted JWT is rejected
func TestJWTAuthentication_BlacklistedToken(t *testing.T) {
	t.Skip("TODO: Implement blacklisted token test")

	// This test verifies that JWT in blacklist table is rejected
	// Setup: Insert JTI into jwt_blacklist table
	// Expected: HTTP 401 Unauthorized with "token has been revoked" message
}

// TestJWTAuthentication_RateLimit tests rate limiting enforces request limits
func TestJWTAuthentication_RateLimit(t *testing.T) {
	t.Skip("TODO: Implement rate limit test")

	// This test verifies that exceeding rate limit returns 429
	// Setup: Service with low rate limit (e.g., 5 requests/second)
	// Action: Make 10 rapid requests
	// Expected: First 5 succeed (200), remaining fail (429 Too Many Requests)
}

// Helper functions

func insertTestService(t *testing.T, db *sql.DB, serviceID string, publicKeyPEM string) uuid.UUID {
	id := uuid.New()
	_, err := db.Exec(`
		INSERT INTO services (id, service_id, service_name, public_key, public_key_fingerprint, environment, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, id, serviceID, "Test Service", publicKeyPEM, "test-fingerprint", "test", true)
	require.NoError(t, err)
	return id
}

func cleanupTestService(t *testing.T, db *sql.DB, id uuid.UUID) {
	_, err := db.Exec(`DELETE FROM services WHERE id = $1`, id)
	if err != nil {
		t.Logf("Warning: Failed to cleanup test service: %v", err)
	}
}

func linkServiceToMerchant(t *testing.T, db *sql.DB, serviceID uuid.UUID, merchantID string) {
	_, err := db.Exec(`
		INSERT INTO service_merchants (service_id, merchant_id, scopes, granted_at)
		VALUES ($1, $2, $3, NOW())
	`, serviceID, merchantID, []string{"payment:create", "payment:read"})
	require.NoError(t, err)
}

func generateJWT(t *testing.T, privateKey *rsa.PrivateKey, issuer string, merchantID string) string {
	claims := jwt.MapClaims{
		"iss":         issuer,
		"merchant_id": merchantID,
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
		"jti":         uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	return tokenString
}

func exportPublicKeyAsPEM(publicKey *rsa.PublicKey) string {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		panic(err)
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(pubKeyPEM)
}
