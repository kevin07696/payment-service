//go:build integration
// +build integration

package payment_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBRICStorage_Refund_MultipleSameGroup tests multiple refunds on same transaction using BRIC Storage
// This test covers both full and partial refund scenarios
func TestBRICStorage_Refund_MultipleSameGroup(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-refund-003"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Process sale for $200
	saleReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "200.00",
		"currency":          "USD",
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	var saleResult map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &saleResult)
	require.NoError(t, err)

	groupID := saleResult["groupId"].(string)
	time.Sleep(2 * time.Second)

	// First refund: $50
	refund1Req := map[string]interface{}{
		"group_id": groupID,
		"amount":   "50.00",
		"reason":   "First partial refund",
	}

	refund1Resp, err := client.Do("POST", "/api/v1/payments/refund", refund1Req)
	require.NoError(t, err)
	defer refund1Resp.Body.Close()

	var refund1Result map[string]interface{}
	err = testutil.DecodeResponse(refund1Resp, &refund1Result)
	require.NoError(t, err)
	assert.Equal(t, "50.00", refund1Result["amount"])

	time.Sleep(2 * time.Second)

	// Second refund: $75
	refund2Req := map[string]interface{}{
		"group_id": groupID,
		"amount":   "75.00",
		"reason":   "Second partial refund",
	}

	refund2Resp, err := client.Do("POST", "/api/v1/payments/refund", refund2Req)
	require.NoError(t, err)
	defer refund2Resp.Body.Close()

	var refund2Result map[string]interface{}
	err = testutil.DecodeResponse(refund2Resp, &refund2Result)
	require.NoError(t, err)
	assert.Equal(t, "75.00", refund2Result["amount"])

	t.Logf("Multiple refunds completed - Total $125 refunded from $200 sale")

	time.Sleep(1 * time.Second)

	// Verify group has 3 transactions (sale + 2 refunds)
	listResp, err := client.Do("GET",
		fmt.Sprintf("/api/v1/payments?agent_id=test-merchant-staging&group_id=%s", groupID), nil)
	require.NoError(t, err)
	defer listResp.Body.Close()

	var listResult map[string]interface{}
	json.NewDecoder(listResp.Body).Decode(&listResult)

	transactions := listResult["transactions"].([]interface{})
	assert.GreaterOrEqual(t, len(transactions), 3, "Should have at least 3 transactions (1 sale + 2 refunds)")
}

// TestBRICStorage_Void_UsingGroupID tests void using group_id with BRIC Storage
func TestBRICStorage_Void_UsingGroupID(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-void-001"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestAmexCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Authorize (not captured yet, so can be voided)
	authReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "75.00",
		"currency":          "USD",
	}

	authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()

	var authResult map[string]interface{}
	err = testutil.DecodeResponse(authResp, &authResult)
	require.NoError(t, err)

	groupID := authResult["groupId"].(string)
	t.Logf("Authorization completed - Group ID: %s", groupID)
	time.Sleep(2 * time.Second)

	// Void using group_id
	voidReq := map[string]interface{}{
		"group_id": groupID,
	}

	voidResp, err := client.Do("POST", "/api/v1/payments/void", voidReq)
	require.NoError(t, err)
	defer voidResp.Body.Close()

	assert.Equal(t, 200, voidResp.StatusCode, "Void should succeed")

	var voidResult map[string]interface{}
	err = testutil.DecodeResponse(voidResp, &voidResult)
	require.NoError(t, err)

	assert.NotEmpty(t, voidResult["transactionId"])
	assert.Equal(t, groupID, voidResult["groupId"])
	assert.True(t, voidResult["isApproved"].(bool))

	t.Logf("Void completed - Transaction voided in group: %s", groupID)
}

// TestBRICStorage_Refund_Validation tests refund validation errors with BRIC Storage
func TestBRICStorage_Refund_Validation(t *testing.T) {
	_, client := testutil.Setup(t)

	testCases := []struct {
		name    string
		request map[string]interface{}
	}{
		{
			name: "missing group_id",
			request: map[string]interface{}{
				"amount": "50.00",
				"reason": "Test refund",
			},
		},
		{
			name: "non-existent group_id",
			request: map[string]interface{}{
				"group_id": "00000000-0000-0000-0000-000000000000",
				"reason":   "Test refund",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			time.Sleep(1 * time.Second)

			resp, err := client.Do("POST", "/api/v1/payments/refund", tc.request)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.NotEqual(t, 200, resp.StatusCode, tc.name+" should fail")
			t.Logf("%s: status %d", tc.name, resp.StatusCode)
		})
	}
}

// TestBRICStorage_Void_Validation tests void validation errors with BRIC Storage
func TestBRICStorage_Void_Validation(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-void-validation"
	time.Sleep(2 * time.Second)

	// Create and capture a transaction (cannot void after capture)
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Process sale (auth + capture)
	saleReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "50.00",
		"currency":          "USD",
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	var saleResult map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &saleResult)
	require.NoError(t, err)

	groupID := saleResult["groupId"].(string)
	time.Sleep(2 * time.Second)

	// Try to void captured transaction (should fail or behave as refund)
	voidReq := map[string]interface{}{
		"group_id": groupID,
	}

	voidResp, err := client.Do("POST", "/api/v1/payments/void", voidReq)
	require.NoError(t, err)
	defer voidResp.Body.Close()

	// EPX may reject void on captured transaction, or treat it as refund
	t.Logf("Void after capture: status %d", voidResp.StatusCode)
}

// TestBRICStorage_API_CleanAbstraction tests that EPX implementation details are not exposed with BRIC Storage
func TestBRICStorage_API_CleanAbstraction(t *testing.T) {
	testutil.SkipIfBRICStorageUnavailable(t) // TODO: Remove once EPX enables BRIC Storage in sandbox

	cfg, client := testutil.Setup(t)
	customerID := "test-customer-clean-api"
	time.Sleep(2 * time.Second)

	// Tokenize and save payment method
	paymentMethodID, err := testutil.TokenizeAndSaveCard(
		cfg, client,
		"test-merchant-staging",
		customerID,
		testutil.TestVisaCard,
	)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Process sale
	saleReq := map[string]interface{}{
		"agent_id":          "test-merchant-staging",
		"customer_id":       customerID,
		"payment_method_id": paymentMethodID,
		"amount":            "25.00",
		"currency":          "USD",
	}

	saleResp, err := client.Do("POST", "/api/v1/payments/sale", saleReq)
	require.NoError(t, err)
	defer saleResp.Body.Close()

	var saleResult map[string]interface{}
	err = testutil.DecodeResponse(saleResp, &saleResult)
	require.NoError(t, err)

	groupID := saleResult["groupId"].(string)
	time.Sleep(2 * time.Second)

	// Refund
	refundReq := map[string]interface{}{
		"group_id": groupID,
		"reason":   "Testing clean API",
	}

	refundResp, err := client.Do("POST", "/api/v1/payments/refund", refundReq)
	require.NoError(t, err)
	defer refundResp.Body.Close()

	var refundResult map[string]interface{}
	err = testutil.DecodeResponse(refundResp, &refundResult)
	require.NoError(t, err)

	// Verify EPX-specific fields are NOT exposed
	epxFields := []string{"authGuid", "authResp", "authCardType", "bric"}
	for _, field := range epxFields {
		assert.Nil(t, refundResult[field], "EPX field %s should not be exposed", field)
	}

	// Verify clean abstracted fields ARE present
	assert.NotEmpty(t, refundResult["authorizationCode"], "Should have authorization_code")
	assert.NotEmpty(t, refundResult["message"], "Should have message")
	assert.NotNil(t, refundResult["isApproved"], "Should have is_approved")

	if card, ok := refundResult["card"].(map[string]interface{}); ok && card != nil {
		assert.NotEmpty(t, card["brand"], "Card should have brand")
		assert.NotEmpty(t, card["lastFour"], "Card should have last_four")
	}

	t.Log("✅ Clean API verified - no EPX implementation details exposed")
}
