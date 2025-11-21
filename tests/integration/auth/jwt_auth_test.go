//go:build integration
// +build integration

package auth_test

import (
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/require"
)

// TestJWTAuthentication_ValidToken tests JWT authentication with valid RSA-signed token
func TestJWTAuthentication_ValidToken(t *testing.T) {
	// Load pre-generated test services
	testServices, err := testutil.LoadTestServices()
	require.NoError(t, err, "Failed to load test services")
	require.NotEmpty(t, testServices, "No test services available")

	// Use first test service
	testService := testServices[0]
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant from seed

	// Generate valid JWT
	token, err := testutil.GenerateJWT(
		testService.PrivateKeyPEM,
		testService.ServiceID,
		merchantID,
		1*time.Hour, // 1 hour expiration
	)
	require.NoError(t, err, "Failed to generate JWT")

	// Setup client with JWT auth header
	cfg, client := testutil.Setup(t)
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request to a simple endpoint
	// Using a health or simple query endpoint to verify auth works
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": merchantID,
		"limit":      10,
	})
	require.NoError(t, err, "Request failed")
	defer resp.Body.Close()

	// Verify successful authentication (200 OK or valid response, not 401)
	require.NotEqual(t, 401, resp.StatusCode, "Authentication should succeed with valid JWT")

	t.Logf("✅ JWT authentication successful with service: %s", testService.ServiceID)
	t.Logf("   Service URL: %s", cfg.ServiceURL)
	t.Logf("   Response status: %d", resp.StatusCode)
}

// TestJWTAuthentication_InvalidSignature tests JWT with wrong signature is rejected
func TestJWTAuthentication_InvalidSignature(t *testing.T) {
	merchantID := "00000000-0000-0000-0000-000000000001"

	// Generate JWT signed with WRONG key (not in database)
	token, err := testutil.GenerateJWTWithWrongKey("unknown-service-123", merchantID)
	require.NoError(t, err, "Failed to generate JWT with wrong key")

	// Setup client with invalid JWT
	_, client := testutil.Setup(t)
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": merchantID,
		"limit":      10,
	})
	require.NoError(t, err, "Request should complete (not connection error)")
	defer resp.Body.Close()

	// Verify authentication failed with 401
	require.Equal(t, 401, resp.StatusCode, "Should reject JWT with invalid signature")

	t.Logf("✅ Correctly rejected JWT with invalid signature (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_ExpiredToken tests expired JWT is rejected
func TestJWTAuthentication_ExpiredToken(t *testing.T) {
	// Load test services
	testServices, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, testServices)

	testService := testServices[0]
	merchantID := "00000000-0000-0000-0000-000000000001"

	// Generate JWT that expired 1 hour ago
	token, err := testutil.GenerateJWT(
		testService.PrivateKeyPEM,
		testService.ServiceID,
		merchantID,
		-1*time.Hour, // Negative duration = already expired
	)
	require.NoError(t, err)

	// Setup client with expired JWT
	_, client := testutil.Setup(t)
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": merchantID,
		"limit":      10,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify authentication failed with 401
	require.Equal(t, 401, resp.StatusCode, "Should reject expired JWT")

	t.Logf("✅ Correctly rejected expired JWT (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_MissingIssuer tests JWT without issuer is rejected
func TestJWTAuthentication_MissingIssuer(t *testing.T) {
	// Load test services
	testServices, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, testServices)

	testService := testServices[0]
	merchantID := "00000000-0000-0000-0000-000000000001"

	// Generate JWT WITHOUT "iss" claim
	claims := map[string]interface{}{
		// "iss" is intentionally missing
		"merchant_id": merchantID,
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
	}

	token, err := testutil.GenerateJWTWithClaims(testService.PrivateKeyPEM, claims)
	require.NoError(t, err)

	// Setup client with JWT missing issuer
	_, client := testutil.Setup(t)
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": merchantID,
		"limit":      10,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify authentication failed with 401
	require.Equal(t, 401, resp.StatusCode, "Should reject JWT without issuer")

	t.Logf("✅ Correctly rejected JWT without issuer (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_UnknownIssuer tests JWT from unknown service is rejected
func TestJWTAuthentication_UnknownIssuer(t *testing.T) {
	// Load test services to get a valid private key
	testServices, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, testServices)

	testService := testServices[0]
	merchantID := "00000000-0000-0000-0000-000000000001"

	// Generate JWT with issuer NOT in database
	token, err := testutil.GenerateJWT(
		testService.PrivateKeyPEM,
		"unknown-service-not-in-db", // This service_id doesn't exist in database
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err)

	// Setup client with JWT from unknown issuer
	_, client := testutil.Setup(t)
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": merchantID,
		"limit":      10,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify authentication failed with 401
	require.Equal(t, 401, resp.StatusCode, "Should reject JWT from unknown issuer")

	t.Logf("✅ Correctly rejected JWT from unknown issuer (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_NoMerchantAccess tests JWT for merchant without access is rejected
func TestJWTAuthentication_NoMerchantAccess(t *testing.T) {
	// Load test services
	testServices, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, testServices)

	testService := testServices[0]
	// Use a different merchant ID that the service is NOT linked to
	// The seed script only links services to merchant 00000000-0000-0000-0000-000000000001
	unauthorizedMerchantID := "00000000-0000-0000-0000-000000000002"

	// Generate valid JWT for merchant the service doesn't have access to
	token, err := testutil.GenerateJWT(
		testService.PrivateKeyPEM,
		testService.ServiceID,
		unauthorizedMerchantID,
		1*time.Hour,
	)
	require.NoError(t, err)

	// Setup client with JWT
	_, client := testutil.Setup(t)
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": unauthorizedMerchantID,
		"limit":      10,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify authentication failed with 401 (service not authorized for this merchant)
	require.Equal(t, 401, resp.StatusCode, "Should reject JWT for merchant without access")

	t.Logf("✅ Correctly rejected service accessing unauthorized merchant (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_BlacklistedToken tests blacklisted JWT is rejected
func TestJWTAuthentication_BlacklistedToken(t *testing.T) {
	// Load test services
	testServices, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, testServices)

	testService := testServices[0]
	merchantID := "00000000-0000-0000-0000-000000000001"

	// Generate a JWT with a known JTI
	jti := "blacklisted-token-test-" + time.Now().Format("20060102-150405")
	claims := map[string]interface{}{
		"iss":         testService.ServiceID,
		"merchant_id": merchantID,
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
		"jti":         jti,
	}

	token, err := testutil.GenerateJWTWithClaims(testService.PrivateKeyPEM, claims)
	require.NoError(t, err)

	// Insert JTI into jwt_blacklist table
	db := testutil.GetDB(t)
	defer db.Close()

	_, err = db.Exec(`
		INSERT INTO jwt_blacklist (jti, blacklisted_at, expires_at)
		VALUES ($1, NOW(), NOW() + INTERVAL '2 hours')
	`, jti)
	require.NoError(t, err, "Failed to insert JTI into blacklist")

	t.Logf("Blacklisted JTI: %s", jti)

	// Setup client with blacklisted JWT
	_, client := testutil.Setup(t)
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": merchantID,
		"limit":      10,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify authentication failed with 401 (token revoked)
	require.Equal(t, 401, resp.StatusCode, "Should reject blacklisted JWT")

	t.Logf("✅ Correctly rejected blacklisted token (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_RateLimit tests rate limiting enforces request limits
func TestJWTAuthentication_RateLimit(t *testing.T) {
	t.Skip("Rate limiting test requires special setup with low rate limit")

	// Load test services
	testServices, err := testutil.LoadTestServices()
	require.NoError(t, err)
	require.NotEmpty(t, testServices)

	testService := testServices[0]
	merchantID := "00000000-0000-0000-0000-000000000001"

	// Generate valid JWT
	token, err := testutil.GenerateJWT(
		testService.PrivateKeyPEM,
		testService.ServiceID,
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err)

	// Setup client with JWT
	_, client := testutil.Setup(t)
	client.SetHeader("Authorization", "Bearer "+token)

	// Make rapid requests to trigger rate limiting
	// Note: The default test service has 100 requests/second limit
	// This test needs a service with lower rate limit to reliably test
	const numRequests = 150
	var successCount, rateLimitCount int

	for i := 0; i < numRequests; i++ {
		resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
			"merchantId": merchantID,
			"limit":      10,
		})
		require.NoError(t, err, "Request should complete (not connection error)")

		if resp.StatusCode == 200 {
			successCount++
		} else if resp.StatusCode == 429 {
			rateLimitCount++
		}
		resp.Body.Close()
	}

	t.Logf("Made %d requests: %d succeeded, %d rate-limited", numRequests, successCount, rateLimitCount)

	// Verify that rate limiting was enforced
	require.Greater(t, rateLimitCount, 0, "Should have some rate-limited requests when exceeding limit")

	t.Logf("✅ Rate limiting correctly enforced (%d requests rate-limited)", rateLimitCount)
}

// Note: Helper functions moved to testutil/auth_helpers.go for reuse across tests
