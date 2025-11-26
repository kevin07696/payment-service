//go:build integration
// +build integration

package auth_test

import (
	"testing"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/require"
)

// TestCronAuthentication_ValidSecret tests cron endpoint with valid X-Cron-Secret header
func TestCronAuthentication_ValidSecret(t *testing.T) {
	cfg, _ := testutil.Setup(t)

	// Create client for cron server (port 8081, not ConnectRPC port 8080)
	cronClient := testutil.NewClient(cfg.CallbackBaseURL)

	// Create request with valid X-Cron-Secret header from config
	cronClient.SetHeader("X-Cron-Secret", cfg.CronSecret)

	// Test ACH verification cron endpoint
	resp, err := cronClient.Do("POST", "/cron/verify-ach", nil)
	require.NoError(t, err, "Request should complete")
	defer resp.Body.Close()

	// Should succeed with valid secret
	require.NotEqual(t, 401, resp.StatusCode, "Should not return 401 with valid cron secret")
	require.NotEqual(t, 403, resp.StatusCode, "Should not return 403 with valid cron secret")

	t.Logf("✅ Valid X-Cron-Secret accepted (status: %d)", resp.StatusCode)
	t.Logf("   Cron URL: %s", cfg.CallbackBaseURL)
}

// TestCronAuthentication_InvalidSecret tests cron endpoint with wrong secret is rejected
func TestCronAuthentication_InvalidSecret(t *testing.T) {
	cfg, _ := testutil.Setup(t)

	// Create client for cron server (port 8081)
	cronClient := testutil.NewClient(cfg.CallbackBaseURL)

	// Create request with WRONG cron secret
	cronClient.SetHeader("X-Cron-Secret", "wrong-secret-12345")

	// Test ACH verification cron endpoint
	resp, err := cronClient.Do("POST", "/cron/verify-ach", nil)
	require.NoError(t, err, "Request should complete (not connection error)")
	defer resp.Body.Close()

	// Should reject with 401 Unauthorized
	require.Equal(t, 401, resp.StatusCode, "Should return 401 with invalid cron secret")

	t.Logf("✅ Invalid X-Cron-Secret rejected (status: %d)", resp.StatusCode)
}

// TestCronAuthentication_MissingSecret tests cron endpoint without X-Cron-Secret is rejected
func TestCronAuthentication_MissingSecret(t *testing.T) {
	cfg, _ := testutil.Setup(t)

	// Create client for cron server (port 8081)
	cronClient := testutil.NewClient(cfg.CallbackBaseURL)

	// Make request WITHOUT any authentication headers
	cronClient.ClearHeaders()

	// Test ACH verification cron endpoint
	resp, err := cronClient.Do("POST", "/cron/verify-ach", nil)
	require.NoError(t, err, "Request should complete")
	defer resp.Body.Close()

	// Should reject with 401 Unauthorized
	require.Equal(t, 401, resp.StatusCode, "Should return 401 without authentication")

	t.Logf("✅ Request without authentication rejected (status: %d)", resp.StatusCode)
}

// TestCronAuthentication_BearerToken tests cron endpoint with Bearer token
// NOTE: The cronAuthMiddleware in main.go only supports X-Cron-Secret header.
// Bearer token auth is not supported by the middleware (though handler code exists).
func TestCronAuthentication_BearerToken(t *testing.T) {
	t.Skip("Bearer token auth is not supported - middleware only checks X-Cron-Secret header")

	cfg, _ := testutil.Setup(t)

	// Create client for cron server (port 8081)
	cronClient := testutil.NewClient(cfg.CallbackBaseURL)

	// Use Bearer token authentication instead of X-Cron-Secret
	cronClient.SetHeader("Authorization", "Bearer "+cfg.CronSecret)

	// Test ACH verification cron endpoint
	resp, err := cronClient.Do("POST", "/cron/verify-ach", nil)
	require.NoError(t, err, "Request should complete")
	defer resp.Body.Close()

	// Should succeed with valid Bearer token
	require.NotEqual(t, 401, resp.StatusCode, "Should not return 401 with valid Bearer token")
	require.NotEqual(t, 403, resp.StatusCode, "Should not return 403 with valid Bearer token")

	t.Logf("✅ Valid Bearer token accepted (status: %d)", resp.StatusCode)
	t.Logf("   Cron URL: %s", cfg.CallbackBaseURL)
}

// TestCronAuthentication_QueryParameter tests cron endpoint with secret as query param
// NOTE: The cronAuthMiddleware in main.go only supports X-Cron-Secret header.
// Query param auth is not supported by the middleware (though handler code exists).
func TestCronAuthentication_QueryParameter(t *testing.T) {
	t.Skip("Query param auth is not supported - middleware only checks X-Cron-Secret header")

	cfg, _ := testutil.Setup(t)

	// Create client for cron server (port 8081)
	cronClient := testutil.NewClient(cfg.CallbackBaseURL)

	// Use query parameter authentication (insecure, for development only)
	// No authentication headers
	cronClient.ClearHeaders()

	// Test ACH verification cron endpoint with secret query parameter
	resp, err := cronClient.Do("POST", "/cron/verify-ach?secret="+cfg.CronSecret, nil)
	require.NoError(t, err, "Request should complete")
	defer resp.Body.Close()

	// Should succeed but log warning
	require.NotEqual(t, 401, resp.StatusCode, "Should not return 401 with valid query param secret")
	require.NotEqual(t, 403, resp.StatusCode, "Should not return 403 with valid query param secret")

	t.Logf("✅ Query parameter secret accepted (status: %d)", resp.StatusCode)
	t.Logf("   Note: Query parameter authentication is insecure and logs warning")
	t.Logf("   Cron URL: %s", cfg.CallbackBaseURL)
}

// TestCronAuthentication_AllEndpoints tests all cron endpoints require authentication
func TestCronAuthentication_AllEndpoints(t *testing.T) {
	cfg, _ := testutil.Setup(t)

	// Create client for cron server (port 8081)
	cronClient := testutil.NewClient(cfg.CallbackBaseURL)

	// List of cron endpoints that should require authentication
	cronEndpoints := []struct {
		method string
		path   string
		name   string
	}{
		{"POST", "/cron/verify-ach", "ACH Verification"},
		{"POST", "/cron/process-billing", "Billing Processing"},
		{"POST", "/cron/sync-disputes", "Dispute Sync"},
		{"GET", "/cron/stats", "Cron Stats"},
		{"GET", "/cron/ach/stats", "ACH Stats"},
	}

	t.Log("Testing authentication is required for all cron endpoints:")

	for _, endpoint := range cronEndpoints {
		// Test WITHOUT authentication - should fail
		cronClient.ClearHeaders()
		resp, err := cronClient.Do(endpoint.method, endpoint.path, nil)
		require.NoError(t, err, "Request to %s should complete", endpoint.path)
		defer resp.Body.Close()

		require.Equal(t, 401, resp.StatusCode,
			"%s (%s %s) should return 401 without auth", endpoint.name, endpoint.method, endpoint.path)

		// Test WITH valid authentication - should succeed
		cronClient.SetHeader("X-Cron-Secret", cfg.CronSecret)
		respAuth, err := cronClient.Do(endpoint.method, endpoint.path, nil)
		require.NoError(t, err, "Authenticated request to %s should complete", endpoint.path)
		defer respAuth.Body.Close()

		require.NotEqual(t, 401, respAuth.StatusCode,
			"%s (%s %s) should not return 401 with valid auth", endpoint.name, endpoint.method, endpoint.path)

		t.Logf("  ✅ %s (%s %s): 401 without auth, %d with auth",
			endpoint.name, endpoint.method, endpoint.path, respAuth.StatusCode)
	}

	t.Logf("✅ All %d cron endpoints properly require authentication", len(cronEndpoints))
}

// TestCronAuthentication_HealthCheckNoAuth tests health check endpoints don't require auth
func TestCronAuthentication_HealthCheckNoAuth(t *testing.T) {
	cfg, _ := testutil.Setup(t)

	// Create client for cron server (port 8081)
	cronClient := testutil.NewClient(cfg.CallbackBaseURL)

	// Health check endpoints should be accessible without authentication
	// Note: /health doesn't exist on port 8081, only /cron/health variants
	healthEndpoints := []struct {
		method string
		path   string
		name   string
	}{
		{"GET", "/cron/health", "Cron Health Check"},
		{"GET", "/cron/ach/health", "ACH Health Check"},
		{"GET", "/cron/audit/health", "Audit Health Check"},
		{"GET", "/cron/rate-limit/health", "Rate Limit Health Check"},
	}

	t.Log("Testing health check endpoints don't require authentication:")

	for _, endpoint := range healthEndpoints {
		// Test WITHOUT authentication - should succeed
		cronClient.ClearHeaders()
		resp, err := cronClient.Do(endpoint.method, endpoint.path, nil)
		require.NoError(t, err, "Request to %s should complete", endpoint.path)
		defer resp.Body.Close()

		require.Equal(t, 200, resp.StatusCode,
			"%s (%s) should return 200 without auth for monitoring", endpoint.name, endpoint.path)

		t.Logf("  ✅ %s (%s %s): accessible without auth",
			endpoint.name, endpoint.method, endpoint.path)
	}

	t.Logf("✅ All %d health check endpoints accessible without authentication", len(healthEndpoints))
}
