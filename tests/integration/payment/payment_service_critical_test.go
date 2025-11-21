//go:build integration
// +build integration

package payment_test

import (
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEPXDeclineCodeHandling verifies decline code processing
// Risk: p90 probability, medium impact (customer experience with error messages)
func TestEPXDeclineCodeHandling(t *testing.T) {
	tests := []struct {
		name         string
		cardDetails  *testutil.CardDetails
		amount       string // EPX uses last 3 digits as response code trigger
		expectStatus string
	}{
		{
			name:         "insufficient_funds_code_51",
			cardDetails:  testutil.VisaDeclineCard(),
			amount:       "1.20", // .20 → EPX code 51 (DECLINE)
			expectStatus: "TRANSACTION_STATUS_DECLINED",
		},
		{
			name:         "generic_decline_code_05",
			cardDetails:  testutil.VisaDeclineCard(),
			amount:       "1.05", // .05 → EPX code 05 (DECLINE)
			expectStatus: "TRANSACTION_STATUS_DECLINED",
		},
		{
			name:         "approval_with_standard_card",
			cardDetails:  testutil.DefaultApprovalCard(),
			amount:       "10.00",
			expectStatus: "TRANSACTION_STATUS_APPROVED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, client := testutil.Setup(t)
			time.Sleep(2 * time.Second)

			// Load test service credentials for JWT authentication
			services, err := testutil.LoadTestServices()
			require.NoError(t, err, "Failed to load test services")
			require.NotEmpty(t, services, "No test services found")

			// Generate JWT token for test service
			jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, "00000000-0000-0000-0000-000000000001", time.Hour)
			require.NoError(t, err, "Failed to generate JWT token")

			// Create SALE with specified card and amount
			t.Logf("[SETUP] Creating SALE card=%s amount=$%s", tt.cardDetails.Number, tt.amount)
			saleResult := testutil.GetRealBRICForSaleAutomatedWithCard(
				t, client, cfg, tt.amount, "http://localhost:8081", tt.cardDetails, jwtToken)
			t.Logf("[CREATED] TX=%s GROUP=%s", saleResult.TransactionID, saleResult.GroupID)
			time.Sleep(2 * time.Second)

			// Set JWT auth for transaction query
			client.SetHeader("Authorization", "Bearer "+jwtToken)
			defer client.ClearHeaders()

			// Fetch transaction details
			t.Log("[TEST] Fetching transaction status...")
			resp, err := client.DoConnectRPC("payment.v1.PaymentService", "GetTransaction", map[string]interface{}{
				"transaction_id": saleResult.TransactionID,
			})
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, 200, resp.StatusCode)

			var transaction map[string]interface{}
			require.NoError(t, testutil.DecodeResponse(resp, &transaction))
			status := transaction["status"].(string)

			// Verify expected status
			assert.Equal(t, tt.expectStatus, status,
				"Expected status=%s, got status=%s", tt.expectStatus, status)
			t.Logf("[PASS] Status verified: %s", status)
		})
	}
}
