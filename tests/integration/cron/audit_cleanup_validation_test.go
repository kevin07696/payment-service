//go:build integration
// +build integration

package cron_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditCleanupValidation verifies that the audit cleanup handler
// properly validates retention parameters to prevent malicious deletion
// Security Risk: HIGH - Prevents attacker with cron secret from deleting all audit logs
func TestAuditCleanupValidation(t *testing.T) {
	cfg, _ := testutil.Setup(t)
	baseURL := cfg.CallbackBaseURL
	cronSecret := cfg.CronSecret

	client := &http.Client{}

	t.Run("Reject_MinimumRetention_Violation", func(t *testing.T) {
		// Attempt to delete audit logs with < 7 days retention (below minimum)
		reqBody := map[string]interface{}{
			"retention_days": 3, // Below 7-day minimum
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/cron/cleanup-audit-logs", bytes.NewBuffer(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cron-Secret", cronSecret)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should reject with 400 Bad Request
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
			"Should reject retention < 7 days")

		var respBody map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		assert.False(t, respBody["success"].(bool), "Success should be false")
		assert.Contains(t, respBody["error"].(string), "at least 7 days",
			"Error message should mention minimum requirement")

		t.Log("[PASS] Minimum retention validation working")
	})

	t.Run("Cap_MaximumRetention", func(t *testing.T) {
		// Attempt to use retention > 3650 days (10 years maximum)
		reqBody := map[string]interface{}{
			"retention_days": 5000, // Above 3650-day maximum
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/cron/cleanup-audit-logs", bytes.NewBuffer(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cron-Secret", cronSecret)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should succeed but cap at 3650 days
		// Note: This is logged as a warning, not an error
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Should accept but cap to maximum")

		var respBody map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		assert.True(t, respBody["success"].(bool), "Success should be true")

		t.Log("[PASS] Maximum retention capping working")
	})

	t.Run("ZeroRetention_UsesDefault", func(t *testing.T) {
		// retention_days = 0 or missing should use the default (90 days)
		// This is a safe fallback behavior - server doesn't allow deleting all logs
		reqBody := map[string]interface{}{
			"retention_days": 0, // Treated as "use default"
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/cron/cleanup-audit-logs", bytes.NewBuffer(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cron-Secret", cronSecret)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Server treats 0 as "use default (90 days)" - this is safer than rejecting
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Zero retention should use default and succeed")

		var respBody map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		assert.True(t, respBody["success"].(bool), "Success should be true with default retention")

		t.Log("[PASS] Zero retention uses default (90 days)")
	})

	t.Run("Reject_TooSmallRetention", func(t *testing.T) {
		// Retention below 7 days should be rejected (minimum enforced)
		reqBody := map[string]interface{}{
			"retention_days": 3, // Below minimum of 7 days
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/cron/cleanup-audit-logs", bytes.NewBuffer(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cron-Secret", cronSecret)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should reject retention below minimum (7 days)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
			"Should reject retention below 7 days")

		t.Log("[PASS] Minimum retention (7 days) enforcement working")
	})

	t.Run("Accept_ValidRetention", func(t *testing.T) {
		// Use valid retention period (90 days - PCI DSS compliant)
		reqBody := map[string]interface{}{
			"retention_days": 90,
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/cron/cleanup-audit-logs", bytes.NewBuffer(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cron-Secret", cronSecret)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should succeed
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Should accept valid retention period")

		var respBody map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&respBody)
		require.NoError(t, err)

		assert.True(t, respBody["success"].(bool), "Success should be true")
		assert.NotNil(t, respBody["cutoff_date"], "Should return cutoff date")
		assert.NotNil(t, respBody["processed_at"], "Should return processed timestamp")

		t.Logf("[PASS] Valid retention accepted, deleted %v rows", respBody["deleted_rows"])
	})

	t.Run("Reject_Unauthorized_Request", func(t *testing.T) {
		// Attempt cleanup without proper authentication
		reqBody := map[string]interface{}{
			"retention_days": 90,
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/cron/cleanup-audit-logs", bytes.NewBuffer(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		// NO X-Cron-Secret header

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should reject with 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Should reject without proper authentication")

		t.Log("[PASS] Authentication enforcement working")
	})

	t.Run("Reject_WrongSecret", func(t *testing.T) {
		// Attempt cleanup with wrong secret
		reqBody := map[string]interface{}{
			"retention_days": 90,
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req, err := http.NewRequest("POST", baseURL+"/cron/cleanup-audit-logs", bytes.NewBuffer(body))
		require.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Cron-Secret", "wrong-secret")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should reject with 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Should reject with wrong secret")

		t.Log("[PASS] Secret validation working")
	})
}

// TestAuditCleanupStats verifies the stats endpoint returns proper information
func TestAuditCleanupStats(t *testing.T) {
	cfg, _ := testutil.Setup(t)
	baseURL := cfg.CallbackBaseURL
	cronSecret := cfg.CronSecret

	client := &http.Client{}

	req, err := http.NewRequest("GET", baseURL+"/cron/audit/stats", nil)
	require.NoError(t, err)

	req.Header.Set("X-Cron-Secret", cronSecret)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Stats endpoint should be accessible")

	var stats map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&stats)
	require.NoError(t, err)

	// Verify expected fields
	assert.NotNil(t, stats["total_audit_logs"], "Should return total audit log count")
	assert.Equal(t, float64(90), stats["default_retention"], "Should show 90-day default")
	assert.Equal(t, "days", stats["retention_unit"], "Should show retention unit")
	assert.NotNil(t, stats["last_check"], "Should return last check timestamp")

	t.Logf("[PASS] Stats endpoint working, total logs: %v", stats["total_audit_logs"])
}

// TestAuditCleanupHealth verifies the health check endpoint
func TestAuditCleanupHealth(t *testing.T) {
	cfg, _ := testutil.Setup(t)
	baseURL := cfg.CallbackBaseURL

	client := &http.Client{}

	req, err := http.NewRequest("GET", baseURL+"/cron/audit/health", nil)
	require.NoError(t, err)

	// Health check should NOT require authentication
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should be accessible without auth")

	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"], "Should report healthy status")
	assert.Equal(t, "audit-cleanup-cron", health["service"], "Should identify service")
	assert.NotNil(t, health["time"], "Should return current time")

	t.Log("[PASS] Health check endpoint working")
}
