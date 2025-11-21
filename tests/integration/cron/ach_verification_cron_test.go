//go:build integration
// +build integration

package cron_test

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestACHVerificationCron_Basic tests the basic ACH verification cron job workflow
func TestACHVerificationCron_Basic(t *testing.T) {
	t.Skip("TODO: Implement StoreACHAccount RPC - TokenizeAndSaveACH not yet available")

	cfg, client := testutil.Setup(t)
	db := testutil.GetDB(t)

	// Create separate client for cron HTTP endpoints (port 8081)
	cronClient := testutil.NewClient("http://localhost:8081")

	// Create 3 pending ACH accounts with backdated timestamps (4 days ago)
	customerID1 := "20000000-0000-0000-0000-000000000001"
	customerID2 := "20000000-0000-0000-0000-000000000002"
	customerID3 := "20000000-0000-0000-0000-000000000003"
	merchantID := "00000000-0000-0000-0000-000000000001"

	// Save 3 ACH accounts
	paymentMethodID1, err := testutil.TokenizeAndSaveACH(cfg, client, merchantID, customerID1, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	paymentMethodID2, err := testutil.TokenizeAndSaveACH(cfg, client, merchantID, customerID2, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	paymentMethodID3, err := testutil.TokenizeAndSaveACH(cfg, client, merchantID, customerID3, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Backdate the created_at timestamps to 4 days ago (past 3-day verification period)
	fourDaysAgo := time.Now().AddDate(0, 0, -4)
	_, err = db.Exec(`
		UPDATE customer_payment_methods
		SET created_at = $1, updated_at = $1
		WHERE id = ANY($2::uuid[])
	`, fourDaysAgo, []string{paymentMethodID1, paymentMethodID2, paymentMethodID3})
	require.NoError(t, err)

	// Verify all 3 are pending before cron runs
	for i, pmID := range []string{paymentMethodID1, paymentMethodID2, paymentMethodID3} {
		status, isVerified, err := testutil.GetACHVerificationStatus(db, pmID)
		require.NoError(t, err)
		assert.Equal(t, "pending", status, "Account %d should be pending before cron", i+1)
		assert.False(t, isVerified, "Account %d should not be verified before cron", i+1)
	}

	t.Logf("✅ Created 3 pending ACH accounts backdated to 4 days ago")

	// Call the cron job endpoint
	cronClient.SetHeader("X-Cron-Secret", "change-me-in-production")

	cronReq := map[string]interface{}{
		"verification_days": 3,
		"batch_size":        100,
	}

	cronResp, err := cronClient.Do("POST", "/cron/verify-ach", cronReq)
	require.NoError(t, err, "Cron endpoint request should succeed")
	defer cronResp.Body.Close()

	// Should return 200 OK
	require.Equal(t, 200, cronResp.StatusCode, "Cron job should return 200 OK")

	// Parse response
	body, err := io.ReadAll(cronResp.Body)
	require.NoError(t, err)

	var cronResult map[string]interface{}
	err = json.Unmarshal(body, &cronResult)
	require.NoError(t, err)

	t.Logf("Cron response: %+v", cronResult)

	// Verify response structure
	assert.True(t, cronResult["success"].(bool), "Cron job should succeed")
	assert.Equal(t, float64(3), cronResult["verified"].(float64), "Should verify 3 accounts")
	assert.Equal(t, float64(0), cronResult["skipped"].(float64), "Should skip 0 accounts")

	// Verify all 3 accounts are now verified in database
	for i, pmID := range []string{paymentMethodID1, paymentMethodID2, paymentMethodID3} {
		status, isVerified, err := testutil.GetACHVerificationStatus(db, pmID)
		require.NoError(t, err)
		assert.Equal(t, "verified", status, "Account %d should be verified after cron", i+1)
		assert.True(t, isVerified, "Account %d should be marked verified after cron", i+1)
	}

	t.Logf("✅ ACH verification cron successfully verified 3 accounts")
}

// TestACHVerificationCron_VerificationDays tests custom verification period
func TestACHVerificationCron_VerificationDays(t *testing.T) {
	t.Skip("TODO: Implement StoreACHAccount RPC - TokenizeAndSaveACH not yet available")

	cfg, client := testutil.Setup(t)
	db := testutil.GetDB(t)

	// Create separate client for cron HTTP endpoints (port 8081)
	cronClient := testutil.NewClient("http://localhost:8081")

	// Create 2 ACH accounts with different ages
	customerID1 := "20000000-0000-0000-0000-000000000004" // 5 days old
	customerID2 := "20000000-0000-0000-0000-000000000005" // 2 days old
	merchantID := "00000000-0000-0000-0000-000000000001"

	paymentMethodID1, err := testutil.TokenizeAndSaveACH(cfg, client, merchantID, customerID1, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	paymentMethodID2, err := testutil.TokenizeAndSaveACH(cfg, client, merchantID, customerID2, testutil.TestACHChecking)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// Backdate accounts
	fiveDaysAgo := time.Now().AddDate(0, 0, -5)
	twoDaysAgo := time.Now().AddDate(0, 0, -2)

	_, err = db.Exec(`UPDATE customer_payment_methods SET created_at = $1, updated_at = $1 WHERE id = $2`, fiveDaysAgo, paymentMethodID1)
	require.NoError(t, err)

	_, err = db.Exec(`UPDATE customer_payment_methods SET created_at = $1, updated_at = $1 WHERE id = $2`, twoDaysAgo, paymentMethodID2)
	require.NoError(t, err)

	// Call cron with verification_days = 3
	// Only account1 (5 days old) should be verified
	// Account2 (2 days old) should be skipped
	cronClient.SetHeader("X-Cron-Secret", "change-me-in-production")

	cronReq := map[string]interface{}{
		"verification_days": 3,
		"batch_size":        100,
	}

	cronResp, err := cronClient.Do("POST", "/cron/verify-ach", cronReq)
	require.NoError(t, err)
	defer cronResp.Body.Close()

	require.Equal(t, 200, cronResp.StatusCode)

	body, err := io.ReadAll(cronResp.Body)
	require.NoError(t, err)

	var cronResult map[string]interface{}
	err = json.Unmarshal(body, &cronResult)
	require.NoError(t, err)

	t.Logf("Cron response: %+v", cronResult)

	// Only 1 account should be verified (the 5-day-old one)
	assert.Equal(t, float64(1), cronResult["verified"].(float64), "Should verify 1 account (5 days old)")

	// Verify account1 is verified, account2 is still pending
	status1, isVerified1, err := testutil.GetACHVerificationStatus(db, paymentMethodID1)
	require.NoError(t, err)
	assert.Equal(t, "verified", status1)
	assert.True(t, isVerified1)

	status2, isVerified2, err := testutil.GetACHVerificationStatus(db, paymentMethodID2)
	require.NoError(t, err)
	assert.Equal(t, "pending", status2)
	assert.False(t, isVerified2)

	t.Logf("✅ Verification days filter works correctly")
}

// TestACHVerificationCron_BatchSize tests batch size limiting
func TestACHVerificationCron_BatchSize(t *testing.T) {
	t.Skip("TODO: Implement StoreACHAccount RPC - TokenizeAndSaveACH not yet available")

	cfg, client := testutil.Setup(t)
	db := testutil.GetDB(t)

	// Create separate client for cron HTTP endpoints (port 8081)
	cronClient := testutil.NewClient("http://localhost:8081")

	// Create 3 pending ACH accounts
	customerIDs := []string{
		"20000000-0000-0000-0000-000000000006",
		"20000000-0000-0000-0000-000000000007",
		"20000000-0000-0000-0000-000000000008",
	}
	merchantID := "00000000-0000-0000-0000-000000000001"
	var paymentMethodIDs []string

	for _, customerID := range customerIDs {
		pmID, err := testutil.TokenizeAndSaveACH(cfg, client, merchantID, customerID, testutil.TestACHChecking)
		require.NoError(t, err)
		paymentMethodIDs = append(paymentMethodIDs, pmID)
		time.Sleep(1 * time.Second)
	}

	// Backdate all to 4 days ago
	fourDaysAgo := time.Now().AddDate(0, 0, -4)
	for _, pmID := range paymentMethodIDs {
		_, err := db.Exec(`UPDATE customer_payment_methods SET created_at = $1, updated_at = $1 WHERE id = $2`, fourDaysAgo, pmID)
		require.NoError(t, err)
	}

	// Call cron with batch_size = 2 (should only verify 2 out of 3)
	cronClient.SetHeader("X-Cron-Secret", "change-me-in-production")

	cronReq := map[string]interface{}{
		"verification_days": 3,
		"batch_size":        2, // Limit to 2
	}

	cronResp, err := cronClient.Do("POST", "/cron/verify-ach", cronReq)
	require.NoError(t, err)
	defer cronResp.Body.Close()

	require.Equal(t, 200, cronResp.StatusCode)

	body, err := io.ReadAll(cronResp.Body)
	require.NoError(t, err)

	var cronResult map[string]interface{}
	err = json.Unmarshal(body, &cronResult)
	require.NoError(t, err)

	t.Logf("Cron response: %+v", cronResult)

	// Should only verify 2 accounts (batch size limit)
	assert.Equal(t, float64(2), cronResult["verified"].(float64), "Should verify only 2 accounts (batch size)")

	// Count how many are verified
	verifiedCount := 0
	for _, pmID := range paymentMethodIDs {
		status, _, err := testutil.GetACHVerificationStatus(db, pmID)
		require.NoError(t, err)
		if status == "verified" {
			verifiedCount++
		}
	}

	assert.Equal(t, 2, verifiedCount, "Exactly 2 accounts should be verified")

	t.Logf("✅ Batch size limiting works correctly")
}

// TestACHVerificationCron_Authentication tests that cron endpoint requires authentication
func TestACHVerificationCron_Authentication(t *testing.T) {
	_, _ = testutil.Setup(t)

	// Create separate client for cron HTTP endpoints (port 8081)
	cronClient := testutil.NewClient("http://localhost:8081")

	// Try WITHOUT authentication
	cronClient.ClearHeaders()

	cronResp, err := cronClient.Do("POST", "/cron/verify-ach", nil)
	require.NoError(t, err)
	defer cronResp.Body.Close()

	// Should return 401 Unauthorized
	assert.Equal(t, 401, cronResp.StatusCode, "Should reject request without authentication")

	t.Logf("✅ Cron endpoint correctly requires authentication")
}

// TestACHVerificationCron_NoEligibleAccounts tests behavior when no accounts are eligible
func TestACHVerificationCron_NoEligibleAccounts(t *testing.T) {
	_, _ = testutil.Setup(t)

	// Create separate client for cron HTTP endpoints (port 8081)
	cronClient := testutil.NewClient("http://localhost:8081")

	// Call cron when no accounts are eligible for verification
	cronClient.SetHeader("X-Cron-Secret", "change-me-in-production")

	cronReq := map[string]interface{}{
		"verification_days": 3,
		"batch_size":        100,
	}

	cronResp, err := cronClient.Do("POST", "/cron/verify-ach", cronReq)
	require.NoError(t, err)
	defer cronResp.Body.Close()

	require.Equal(t, 200, cronResp.StatusCode)

	body, err := io.ReadAll(cronResp.Body)
	require.NoError(t, err)

	var cronResult map[string]interface{}
	err = json.Unmarshal(body, &cronResult)
	require.NoError(t, err)

	// Should succeed (verified count depends on database state from previous tests)
	assert.True(t, cronResult["success"].(bool))
	assert.GreaterOrEqual(t, cronResult["verified"].(float64), float64(0))

	t.Logf("✅ Cron handles no eligible accounts gracefully (verified %v accounts)", cronResult["verified"])
}

// TestACHVerificationCron_InvalidParameters tests parameter validation
func TestACHVerificationCron_InvalidParameters(t *testing.T) {
	_, _ = testutil.Setup(t)

	// Create separate client for cron HTTP endpoints (port 8081)
	cronClient := testutil.NewClient("http://localhost:8081")
	cronClient.SetHeader("X-Cron-Secret", "change-me-in-production")

	testCases := []struct {
		name             string
		verificationDays *int
		batchSize        *int
		expectedStatus   int
		errorContains    string
	}{
		{
			name:             "Verification days too low",
			verificationDays: intPtr(0),
			expectedStatus:   400,
			errorContains:    "verification_days must be between 1 and 30",
		},
		{
			name:             "Verification days too high",
			verificationDays: intPtr(31),
			expectedStatus:   400,
			errorContains:    "verification_days must be between 1 and 30",
		},
		{
			name:           "Batch size too low",
			batchSize:      intPtr(0),
			expectedStatus: 400,
			errorContains:  "batch_size must be between 1 and 1000",
		},
		{
			name:           "Batch size too high",
			batchSize:      intPtr(1001),
			expectedStatus: 400,
			errorContains:  "batch_size must be between 1 and 1000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cronReq := make(map[string]interface{})
			if tc.verificationDays != nil {
				cronReq["verification_days"] = *tc.verificationDays
			}
			if tc.batchSize != nil {
				cronReq["batch_size"] = *tc.batchSize
			}

			cronResp, err := cronClient.Do("POST", "/cron/verify-ach", cronReq)
			require.NoError(t, err)
			defer cronResp.Body.Close()

			assert.Equal(t, tc.expectedStatus, cronResp.StatusCode, fmt.Sprintf("%s should return %d", tc.name, tc.expectedStatus))

			if tc.errorContains != "" {
				body, _ := io.ReadAll(cronResp.Body)
				assert.Contains(t, string(body), tc.errorContains, "Error message should contain expected text")
			}
		})
	}

	t.Logf("✅ Parameter validation works correctly")
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
