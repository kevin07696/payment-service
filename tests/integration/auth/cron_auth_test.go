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
	t.Skip("Requires authentication enabled - will be enabled in task #6")

	cfg, client := testutil.Setup(t)

	// Create request with valid X-Cron-Secret header
	client.SetHeader("X-Cron-Secret", "change-me-in-production") // Default cron secret

	// Test ACH verification cron endpoint
	resp, err := client.Do("POST", "/cron/verify-ach", nil)
	require.NoError(t, err, "Request should complete")
	defer resp.Body.Close()

	// Should succeed with valid secret
	require.NotEqual(t, 401, resp.StatusCode, "Should not return 401 with valid cron secret")
	require.NotEqual(t, 403, resp.StatusCode, "Should not return 403 with valid cron secret")

	t.Logf("✅ Valid X-Cron-Secret accepted (status: %d)", resp.StatusCode)
	t.Logf("   Service URL: %s", cfg.ServiceURL)
}

// TestCronAuthentication_InvalidSecret tests cron endpoint with wrong secret is rejected
func TestCronAuthentication_InvalidSecret(t *testing.T) {
	t.Skip("Requires authentication enabled - will be enabled in task #6")

	_, client := testutil.Setup(t)

	// Create request with WRONG cron secret
	client.SetHeader("X-Cron-Secret", "wrong-secret-12345")

	// Test ACH verification cron endpoint
	resp, err := client.Do("POST", "/cron/verify-ach", nil)
	require.NoError(t, err, "Request should complete (not connection error)")
	defer resp.Body.Close()

	// Should reject with 401 Unauthorized
	require.Equal(t, 401, resp.StatusCode, "Should return 401 with invalid cron secret")

	t.Logf("✅ Invalid X-Cron-Secret rejected (status: %d)", resp.StatusCode)
}

// TestCronAuthentication_MissingSecret tests cron endpoint without X-Cron-Secret is rejected
func TestCronAuthentication_MissingSecret(t *testing.T) {
	t.Skip("Requires authentication enabled - will be enabled in task #6")

	_, client := testutil.Setup(t)

	// Make request WITHOUT any authentication headers
	client.ClearHeaders()

	// Test ACH verification cron endpoint
	resp, err := client.Do("POST", "/cron/verify-ach", nil)
	require.NoError(t, err, "Request should complete")
	defer resp.Body.Close()

	// Should reject with 401 Unauthorized
	require.Equal(t, 401, resp.StatusCode, "Should return 401 without authentication")

	t.Logf("✅ Request without authentication rejected (status: %d)", resp.StatusCode)
}

// TestCronAuthentication_BearerToken tests cron endpoint accepts Bearer token
func TestCronAuthentication_BearerToken(t *testing.T) {
	t.Skip("Requires authentication enabled - will be enabled in task #6")

	cfg, client := testutil.Setup(t)

	// Use Bearer token authentication instead of X-Cron-Secret
	client.SetHeader("Authorization", "Bearer change-me-in-production")

	// Test ACH verification cron endpoint
	resp, err := client.Do("POST", "/cron/verify-ach", nil)
	require.NoError(t, err, "Request should complete")
	defer resp.Body.Close()

	// Should succeed with valid Bearer token
	require.NotEqual(t, 401, resp.StatusCode, "Should not return 401 with valid Bearer token")
	require.NotEqual(t, 403, resp.StatusCode, "Should not return 403 with valid Bearer token")

	t.Logf("✅ Valid Bearer token accepted (status: %d)", resp.StatusCode)
	t.Logf("   Service URL: %s", cfg.ServiceURL)
}

// TestCronAuthentication_QueryParameter tests cron endpoint accepts secret as query param (insecure)
func TestCronAuthentication_QueryParameter(t *testing.T) {
	t.Skip("Requires authentication enabled - will be enabled in task #6")

	cfg, client := testutil.Setup(t)

	// Use query parameter authentication (insecure, for development only)
	// No authentication headers
	client.ClearHeaders()

	// Test ACH verification cron endpoint with secret query parameter
	resp, err := client.Do("POST", "/cron/verify-ach?secret=change-me-in-production", nil)
	require.NoError(t, err, "Request should complete")
	defer resp.Body.Close()

	// Should succeed but log warning
	require.NotEqual(t, 401, resp.StatusCode, "Should not return 401 with valid query param secret")
	require.NotEqual(t, 403, resp.StatusCode, "Should not return 403 with valid query param secret")

	t.Logf("✅ Query parameter secret accepted (status: %d)", resp.StatusCode)
	t.Logf("   ⚠️  Note: Query parameter authentication is insecure and logs warning")
	t.Logf("   Service URL: %s", cfg.ServiceURL)
}

// TestCronAuthentication_AllEndpoints tests all cron endpoints require authentication
func TestCronAuthentication_AllEndpoints(t *testing.T) {
	t.Skip("Requires authentication enabled - will be enabled in task #6")

	_, client := testutil.Setup(t)

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
		client.ClearHeaders()
		resp, err := client.Do(endpoint.method, endpoint.path, nil)
		require.NoError(t, err, "Request to %s should complete", endpoint.path)
		defer resp.Body.Close()

		require.Equal(t, 401, resp.StatusCode,
			"%s (%s %s) should return 401 without auth", endpoint.name, endpoint.method, endpoint.path)

		// Test WITH valid authentication - should succeed
		client.SetHeader("X-Cron-Secret", "change-me-in-production")
		respAuth, err := client.Do(endpoint.method, endpoint.path, nil)
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
	t.Skip("Requires health endpoints to be implemented")

	_, client := testutil.Setup(t)

	// Health check endpoints should be accessible without authentication
	healthEndpoints := []struct {
		method string
		path   string
		name   string
	}{
		{"GET", "/health", "Main Health Check"},
		{"GET", "/cron/health", "Cron Health Check"},
		{"GET", "/cron/ach/health", "ACH Health Check"},
	}

	t.Log("Testing health check endpoints don't require authentication:")

	for _, endpoint := range healthEndpoints {
		// Test WITHOUT authentication - should succeed
		client.ClearHeaders()
		resp, err := client.Do(endpoint.method, endpoint.path, nil)
		require.NoError(t, err, "Request to %s should complete", endpoint.path)
		defer resp.Body.Close()

		require.Equal(t, 200, resp.StatusCode,
			"%s (%s) should return 200 without auth for monitoring", endpoint.name, endpoint.path)

		t.Logf("  ✅ %s (%s %s): accessible without auth",
			endpoint.name, endpoint.method, endpoint.path)
	}

	t.Logf("✅ All %d health check endpoints accessible without authentication", len(healthEndpoints))
}
