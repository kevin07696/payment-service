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
// Uses factory-created test data for proper test isolation
func TestJWTAuthentication_ValidToken(t *testing.T) {
	// Setup test environment and create isolated test data
	cfg, client := testutil.Setup(t)
	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t) // Creates merchant + service + access

	// Generate valid JWT using factory-created service and merchant
	token, err := testutil.GenerateJWT(
		ctx.Service.PrivateKeyPEM,
		ctx.Service.ServiceID,
		ctx.Merchant.ID.String(),
		1*time.Hour, // 1 hour expiration
	)
	require.NoError(t, err, "Failed to generate JWT")

	// Setup client with JWT auth header
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request to a simple endpoint
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": ctx.Merchant.ID.String(),
		"limit":      10,
	})
	require.NoError(t, err, "Request failed")
	defer resp.Body.Close()

	// Verify successful authentication (200 OK or valid response, not 401)
	require.NotEqual(t, 401, resp.StatusCode, "Authentication should succeed with valid JWT")

	t.Logf("JWT authentication successful with service: %s", ctx.Service.ServiceID)
	t.Logf("   Merchant: %s (%s)", ctx.Merchant.Slug, ctx.Merchant.ID)
	t.Logf("   Service URL: %s", cfg.ServiceURL)
	t.Logf("   Response status: %d", resp.StatusCode)
}

// TestJWTAuthentication_InvalidSignature tests JWT with wrong signature is rejected
func TestJWTAuthentication_InvalidSignature(t *testing.T) {
	// Create isolated merchant (even though we're testing invalid signature)
	_, client := testutil.Setup(t)
	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)
	merchantID := ctx.Merchant.ID.String()

	// Generate JWT signed with WRONG key (not in database)
	token, err := testutil.GenerateJWTWithWrongKey("unknown-service-123", merchantID)
	require.NoError(t, err, "Failed to generate JWT with wrong key")

	// Setup client with invalid JWT
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

	t.Logf("Correctly rejected JWT with invalid signature (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_ExpiredToken tests expired JWT is rejected
func TestJWTAuthentication_ExpiredToken(t *testing.T) {
	// Create isolated test data
	_, client := testutil.Setup(t)
	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	// Generate JWT that expired 1 hour ago
	token, err := testutil.GenerateJWT(
		ctx.Service.PrivateKeyPEM,
		ctx.Service.ServiceID,
		ctx.Merchant.ID.String(),
		-1*time.Hour, // Negative duration = already expired
	)
	require.NoError(t, err)

	// Setup client with expired JWT
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": ctx.Merchant.ID.String(),
		"limit":      10,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify authentication failed with 401
	require.Equal(t, 401, resp.StatusCode, "Should reject expired JWT")

	t.Logf("Correctly rejected expired JWT (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_MissingIssuer tests JWT without issuer is rejected
func TestJWTAuthentication_MissingIssuer(t *testing.T) {
	// Create isolated test data
	_, client := testutil.Setup(t)
	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)
	merchantID := ctx.Merchant.ID.String()

	// Generate JWT WITHOUT "iss" claim
	now := time.Now()
	claims := map[string]interface{}{
		// "iss" is intentionally missing
		// "sub" is also intentionally missing (as it should match iss)
		"merchant_id": merchantID,
		"scopes":      []string{"payments:create", "payments:read"},
		"exp":         now.Add(1 * time.Hour).Unix(),
		"iat":         now.Unix(),
		"nbf":         now.Unix(),
		"jti":         "missing-iss-test",
	}

	token, err := testutil.GenerateJWTWithClaims(ctx.Service.PrivateKeyPEM, claims)
	require.NoError(t, err)

	// Setup client with JWT missing issuer
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

	t.Logf("Correctly rejected JWT without issuer (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_UnknownIssuer tests JWT from unknown service is rejected
func TestJWTAuthentication_UnknownIssuer(t *testing.T) {
	// Create isolated test data
	_, client := testutil.Setup(t)
	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)
	merchantID := ctx.Merchant.ID.String()

	// Generate JWT with issuer NOT in database (using our valid key but wrong issuer)
	token, err := testutil.GenerateJWT(
		ctx.Service.PrivateKeyPEM,
		"unknown-service-not-in-db", // This service_id doesn't exist in database
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err)

	// Setup client with JWT from unknown issuer
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

	t.Logf("Correctly rejected JWT from unknown issuer (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_NoMerchantAccess tests JWT for merchant without access is rejected
func TestJWTAuthentication_NoMerchantAccess(t *testing.T) {
	// Create isolated test data - one merchant WITH access, one WITHOUT
	_, client := testutil.Setup(t)
	factory := testutil.NewFactory(t)

	// Create authorized merchant with service access
	ctx := factory.CreateTestContext(t)

	// Create another merchant that the service does NOT have access to
	unauthorizedMerchant := factory.NewMerchant(t).WithName("Unauthorized Test Merchant").Create()
	// Note: No GrantAccess() call - service has no access to this merchant

	// Generate valid JWT but for merchant the service doesn't have access to
	token, err := testutil.GenerateJWT(
		ctx.Service.PrivateKeyPEM,
		ctx.Service.ServiceID,
		unauthorizedMerchant.ID.String(), // Different merchant
		1*time.Hour,
	)
	require.NoError(t, err)

	// Setup client with JWT
	client.SetHeader("Authorization", "Bearer "+token)

	// Make authenticated request
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": unauthorizedMerchant.ID.String(),
		"limit":      10,
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify access denied with 403 (service authenticated but not authorized for this merchant)
	// Note: 403 Permission Denied is correct because the service IS authenticated (JWT is valid)
	// but lacks authorization to access this specific merchant
	require.Equal(t, 403, resp.StatusCode, "Should reject JWT for merchant without access with 403 Forbidden")

	t.Logf("Correctly rejected service accessing unauthorized merchant (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_BlacklistedToken tests blacklisted JWT is rejected
func TestJWTAuthentication_BlacklistedToken(t *testing.T) {
	// Create isolated test data
	_, client := testutil.Setup(t)
	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)
	merchantID := ctx.Merchant.ID.String()

	// Generate a JWT with a known JTI
	jti := "blacklisted-token-test-" + time.Now().Format("20060102-150405")
	now := time.Now()
	claims := map[string]interface{}{
		"iss":         ctx.Service.ServiceID,
		"sub":         ctx.Service.ServiceID, // sub matches iss for service auth
		"merchant_id": merchantID,
		"scopes":      []string{"payments:create", "payments:read"},
		"exp":         now.Add(1 * time.Hour).Unix(),
		"iat":         now.Unix(),
		"nbf":         now.Unix(), // Token valid immediately
		"jti":         jti,
	}

	token, err := testutil.GenerateJWTWithClaims(ctx.Service.PrivateKeyPEM, claims)
	require.NoError(t, err)

	// Insert JTI into jwt_blacklist table
	db := testutil.GetDB(t)
	// NOTE: Don't close shared pool - GetDB returns singleton

	_, err = db.Exec(`
		INSERT INTO jwt_blacklist (jti, blacklisted_at, expires_at)
		VALUES ($1, NOW(), NOW() + INTERVAL '2 hours')
	`, jti)
	require.NoError(t, err, "Failed to insert JTI into blacklist")

	// Cleanup blacklisted JTI after test
	t.Cleanup(func() {
		db.Exec("DELETE FROM jwt_blacklist WHERE jti = $1", jti)
	})

	t.Logf("Blacklisted JTI: %s", jti)

	// Setup client with blacklisted JWT
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

	t.Logf("Correctly rejected blacklisted token (status: %d)", resp.StatusCode)
}

// TestJWTAuthentication_RateLimit tests rate limiting enforces request limits
// Uses a service with low rate limit (5 requests per minute) to quickly trigger rate limiting
func TestJWTAuthentication_RateLimit(t *testing.T) {
	// Create isolated test data with LOW rate limit
	_, client := testutil.Setup(t)
	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContextWithRateLimit(t, 5) // 5 requests per minute
	merchantID := ctx.Merchant.ID.String()

	// Generate valid JWT
	token, err := testutil.GenerateJWT(
		ctx.Service.PrivateKeyPEM,
		ctx.Service.ServiceID,
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err)

	// Setup client with JWT
	client.SetHeader("Authorization", "Bearer "+token)

	// Make 5 requests - all should succeed (within rate limit)
	for i := 0; i < 5; i++ {
		resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
			"merchantId": merchantID,
			"limit":      10,
		})
		require.NoError(t, err, "Request should complete (not connection error)")
		require.NotEqual(t, 429, resp.StatusCode, "Request %d should succeed (within rate limit)", i+1)
		resp.Body.Close()
	}

	// 6th request should be rate limited
	resp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", map[string]interface{}{
		"merchantId": merchantID,
		"limit":      10,
	})
	require.NoError(t, err, "Request should complete (not connection error)")
	defer resp.Body.Close()

	require.Equal(t, 429, resp.StatusCode, "6th request should be rate limited (exceeded 5 req/min)")

	t.Logf("Rate limiting correctly enforced after 5 requests (status: %d)", resp.StatusCode)
}

// Note: Helper functions moved to testutil/auth_helpers.go for reuse across tests
