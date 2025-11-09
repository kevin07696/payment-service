// +build integration

package merchant_test

import (
	"testing"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMerchant_FromSeedData(t *testing.T) {
	_, client := testutil.Setup(t)

	// Test merchant seeded from 003_agent_credentials.sql
	testMerchantID := "test-merchant-staging"

	resp, err := client.Do("GET", "/api/v1/merchants/"+testMerchantID, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Should retrieve seeded test merchant")

	// Verify merchant details
	var merchant map[string]interface{}
	err = testutil.DecodeResponse(resp, &merchant)
	require.NoError(t, err)

	assert.Equal(t, testMerchantID, merchant["agent_id"])
	assert.Equal(t, "EPX Sandbox Test Merchant", merchant["agent_name"])
	assert.True(t, merchant["is_active"].(bool))
}

func TestHealthCheck(t *testing.T) {
	_, client := testutil.Setup(t)

	resp, err := client.Do("GET", "/health", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Health check should return 200")
}
