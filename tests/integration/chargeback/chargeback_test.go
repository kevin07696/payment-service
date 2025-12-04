//go:build integration
// +build integration

package chargeback_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	chargebackv1 "github.com/kevin07696/payment-service/proto/chargeback/v1"
	"github.com/kevin07696/payment-service/proto/chargeback/v1/chargebackv1connect"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	connectAddress = "http://localhost:8080"
)

// setupChargebackTest initializes test environment and returns Connect client
func setupChargebackTest(t *testing.T) (*testutil.Config, chargebackv1connect.ChargebackServiceClient) {
	t.Helper()

	// Use standard setup which seeds merchants and chargebacks
	cfg, _ := testutil.Setup(t)

	// Register cleanup functions to remove test data after tests complete
	testutil.CleanupChargebacks(t)

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client := chargebackv1connect.NewChargebackServiceClient(
		httpClient,
		connectAddress,
	)

	return cfg, client
}

// addAuthToRequest adds JWT authentication to a Connect request
func addAuthToRequest[T any](t *testing.T, req *connect.Request[T], merchantID string) {
	t.Helper()

	// Load test services
	services, err := testutil.LoadTestServices()
	require.NoError(t, err, "Failed to load test services")
	require.NotEmpty(t, services, "No test services available")

	// Use first test service
	service := services[0]

	// Generate JWT token
	token, err := testutil.GenerateJWT(
		service.PrivateKeyPEM,
		service.ServiceID,
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err, "Failed to generate JWT")

	// Add authorization header
	req.Header().Set("Authorization", "Bearer "+token)
}

// TestChargeback_ListChargebacks tests listing chargebacks with various filters
func TestChargeback_ListChargebacks(t *testing.T) {
	_, client := setupChargebackTest(t)
	// Use constants for merchant IDs to ensure consistency
	merchantID := testutil.TestMerchantUUID
	testMerchantID := testutil.TestMerchantSlug

	tests := []struct {
		name        string
		request     *chargebackv1.ListChargebacksRequest
		description string
	}{
		{
			name: "List all chargebacks",
			request: &chargebackv1.ListChargebacksRequest{
				MerchantId: testMerchantID,
				Limit:      10,
				Offset:     0,
			},
			description: "Basic list without filters",
		},
		{
			name: "Filter by NEW status",
			request: &chargebackv1.ListChargebacksRequest{
				MerchantId: testMerchantID,
				Status: func() *chargebackv1.ChargebackStatus {
					s := chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_NEW
					return &s
				}(),
				Limit:  10,
				Offset: 0,
			},
			description: "List with status filter",
		},
		{
			name: "Pagination with offset",
			request: &chargebackv1.ListChargebacksRequest{
				MerchantId: testMerchantID,
				Limit:      5,
				Offset:     5,
			},
			description: "Test pagination",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req := connect.NewRequest(tt.request)
			addAuthToRequest(t, req, merchantID)

			resp, err := client.ListChargebacks(ctx, req)
			require.NoError(t, err, "ListChargebacks should succeed")
			assert.NotNil(t, resp)
			assert.GreaterOrEqual(t, resp.Msg.TotalCount, int32(0), "Total count should be non-negative")

			// If status filter is provided, verify all returned chargebacks match
			if tt.request.Status != nil {
				for _, cb := range resp.Msg.Chargebacks {
					assert.Equal(t, *tt.request.Status, cb.Status, "All chargebacks should match filter status")
				}
			}

			t.Logf("✅ %s: Listed %d chargebacks (total: %d)", tt.description, len(resp.Msg.Chargebacks), resp.Msg.TotalCount)
		})
	}
}

// TestChargeback_GetChargeback tests retrieving a specific chargeback
func TestChargeback_GetChargeback(t *testing.T) {
	_, client := setupChargebackTest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use constants for merchant IDs to ensure consistency
	merchantID := testutil.TestMerchantUUID
	testMerchantID := testutil.TestMerchantSlug

	// First, list chargebacks to get a valid ID
	listReq := connect.NewRequest(&chargebackv1.ListChargebacksRequest{
		MerchantId: testMerchantID,
		Limit:      1,
		Offset:     0,
	})
	addAuthToRequest(t, listReq, merchantID)

	listResp, err := client.ListChargebacks(ctx, listReq)
	require.NoError(t, err)

	if len(listResp.Msg.Chargebacks) == 0 {
		t.Skip("No chargebacks available for testing")
	}

	// Get the first chargeback
	chargebackID := listResp.Msg.Chargebacks[0].Id

	getReq := connect.NewRequest(&chargebackv1.GetChargebackRequest{
		ChargebackId: chargebackID,
		MerchantId:   testMerchantID,
	})
	addAuthToRequest(t, getReq, merchantID)

	resp, err := client.GetChargeback(ctx, getReq)
	require.NoError(t, err, "GetChargeback should succeed")
	assert.Equal(t, chargebackID, resp.Msg.Id)

	t.Logf("✅ Retrieved chargeback %s (case: %s, amount: %s)",
		resp.Msg.Id, resp.Msg.CaseNumber, resp.Msg.ChargebackAmount)
}

// TestChargeback_GetChargebackNotFound tests getting a non-existent chargeback
func TestChargeback_GetChargebackNotFound(t *testing.T) {
	_, client := setupChargebackTest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use constants for merchant IDs to ensure consistency
	merchantID := testutil.TestMerchantUUID
	testMerchantID := testutil.TestMerchantSlug
	nonExistentID := uuid.New().String()

	req := connect.NewRequest(&chargebackv1.GetChargebackRequest{
		ChargebackId: nonExistentID,
		MerchantId:   testMerchantID,
	})
	addAuthToRequest(t, req, merchantID)

	_, err := client.GetChargeback(ctx, req)
	require.Error(t, err, "Should return error for non-existent chargeback")

	// Verify it's a Connect error with NotFound code
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code(), "Should return NotFound error")

	t.Logf("✅ Correctly returned NotFound for non-existent chargeback")
}

// TestChargeback_UnauthorizedAccess tests that users can't access other merchants' chargebacks
func TestChargeback_UnauthorizedAccess(t *testing.T) {
	_, client := setupChargebackTest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use constants for correct merchant credentials
	correctMerchantID := testutil.TestMerchantUUID
	testMerchantID := testutil.TestMerchantSlug

	// Wrong merchant ID to test access control
	wrongMerchantID := "wrong-merchant"

	// First, get a real chargeback ID from correct merchant
	listReq := connect.NewRequest(&chargebackv1.ListChargebacksRequest{
		MerchantId: testMerchantID,
		Limit:      1,
		Offset:     0,
	})
	addAuthToRequest(t, listReq, correctMerchantID)

	listResp, err := client.ListChargebacks(ctx, listReq)
	require.NoError(t, err)

	if len(listResp.Msg.Chargebacks) == 0 {
		t.Skip("No chargebacks available for testing")
	}

	chargebackID := listResp.Msg.Chargebacks[0].Id

	// Try to get the chargeback with wrong merchant ID (but valid merchant JWT)
	getReq := connect.NewRequest(&chargebackv1.GetChargebackRequest{
		ChargebackId: chargebackID,
		MerchantId:   wrongMerchantID,
	})
	addAuthToRequest(t, getReq, correctMerchantID)

	_, err = client.GetChargeback(ctx, getReq)
	require.Error(t, err, "Should deny access to other merchant's chargebacks")

	// Verify it's a PermissionDenied error
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code(), "Should return PermissionDenied")

	t.Logf("✅ Correctly denied unauthorized access")
}

// NOTE: Validation tests have been moved to unit tests at:
// internal/handlers/chargeback/chargeback_handler_connect_test.go
// Integration tests focus on end-to-end API behavior with real HTTP calls
