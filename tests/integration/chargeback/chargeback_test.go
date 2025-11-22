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

// setupChargebackClient creates a Connect protocol client for ChargebackService
func setupChargebackClient(t *testing.T) chargebackv1connect.ChargebackServiceClient {
	t.Helper()

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client := chargebackv1connect.NewChargebackServiceClient(
		httpClient,
		connectAddress,
	)

	return client
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

// TestChargeback_ListChargebacks tests listing chargebacks for a merchant
func TestChargeback_ListChargebacks(t *testing.T) {
	client := setupChargebackClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test merchant UUID (from test fixtures/database)
	merchantID := "acme-corp" // Agent ID (slug)

	req := connect.NewRequest(&chargebackv1.ListChargebacksRequest{
		AgentId: merchantID,
		Limit:   10,
		Offset:  0,
	})
	addAuthToRequest(t, req, merchantID)

	resp, err := client.ListChargebacks(ctx, req)
	require.NoError(t, err, "ListChargebacks should succeed")
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Chargebacks)
	assert.GreaterOrEqual(t, resp.Msg.TotalCount, int32(0), "Total count should be non-negative")

	t.Logf("✅ Listed %d chargebacks (total: %d)", len(resp.Msg.Chargebacks), resp.Msg.TotalCount)
}

// TestChargeback_ListChargebacksWithStatusFilter tests filtering by status
func TestChargeback_ListChargebacksWithStatusFilter(t *testing.T) {
	client := setupChargebackClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	merchantID := "acme-corp"
	status := chargebackv1.ChargebackStatus_CHARGEBACK_STATUS_NEW

	req := connect.NewRequest(&chargebackv1.ListChargebacksRequest{
		AgentId: merchantID,
		Status:  &status,
		Limit:   10,
		Offset:  0,
	})
	addAuthToRequest(t, req, merchantID)

	resp, err := client.ListChargebacks(ctx, req)
	require.NoError(t, err, "ListChargebacks with status filter should succeed")
	assert.NotNil(t, resp)

	// Verify all returned chargebacks have the requested status
	for _, cb := range resp.Msg.Chargebacks {
		assert.Equal(t, status, cb.Status, "All chargebacks should have status NEW")
	}

	t.Logf("✅ Listed %d NEW chargebacks", len(resp.Msg.Chargebacks))
}

// TestChargeback_GetChargeback tests retrieving a specific chargeback
func TestChargeback_GetChargeback(t *testing.T) {
	client := setupChargebackClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	merchantID := "acme-corp"

	// First, list chargebacks to get a valid ID
	listReq := connect.NewRequest(&chargebackv1.ListChargebacksRequest{
		AgentId: merchantID,
		Limit:   1,
		Offset:  0,
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
		AgentId:      merchantID,
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
	client := setupChargebackClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	merchantID := "acme-corp"
	nonExistentID := uuid.New().String()

	req := connect.NewRequest(&chargebackv1.GetChargebackRequest{
		ChargebackId: nonExistentID,
		AgentId:      merchantID,
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
	client := setupChargebackClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to access with wrong merchant ID
	wrongMerchantID := "wrong-merchant"

	// First, get a real chargeback ID from correct merchant
	correctMerchantID := "acme-corp"
	listReq := connect.NewRequest(&chargebackv1.ListChargebacksRequest{
		AgentId: correctMerchantID,
		Limit:   1,
		Offset:  0,
	})
	addAuthToRequest(t, listReq, correctMerchantID)

	listResp, err := client.ListChargebacks(ctx, listReq)
	require.NoError(t, err)

	if len(listResp.Msg.Chargebacks) == 0 {
		t.Skip("No chargebacks available for testing")
	}

	chargebackID := listResp.Msg.Chargebacks[0].Id

	// Try to get the chargeback with wrong merchant ID
	getReq := connect.NewRequest(&chargebackv1.GetChargebackRequest{
		ChargebackId: chargebackID,
		AgentId:      wrongMerchantID,
	})
	addAuthToRequest(t, getReq, wrongMerchantID)

	_, err = client.GetChargeback(ctx, getReq)
	require.Error(t, err, "Should deny access to other merchant's chargebacks")

	// Verify it's a PermissionDenied error
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code(), "Should return PermissionDenied")

	t.Logf("✅ Correctly denied unauthorized access")
}

// TestChargeback_ValidationErrors tests input validation
func TestChargeback_ValidationErrors(t *testing.T) {
	client := setupChargebackClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	merchantID := "acme-corp"

	tests := []struct {
		name          string
		chargebackID  string
		agentID       string
		expectedCode  connect.Code
		expectedError string
	}{
		{
			name:          "Missing chargeback_id",
			chargebackID:  "",
			agentID:       merchantID,
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "chargeback_id is required",
		},
		{
			name:          "Missing agent_id",
			chargebackID:  uuid.New().String(),
			agentID:       "",
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "agent_id is required",
		},
		{
			name:          "Invalid chargeback_id format",
			chargebackID:  "not-a-uuid",
			agentID:       merchantID,
			expectedCode:  connect.CodeInvalidArgument,
			expectedError: "invalid chargeback_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := connect.NewRequest(&chargebackv1.GetChargebackRequest{
				ChargebackId: tt.chargebackID,
				AgentId:      tt.agentID,
			})
			if tt.agentID != "" {
				addAuthToRequest(t, req, tt.agentID)
			}

			_, err := client.GetChargeback(ctx, req)
			require.Error(t, err)

			var connectErr *connect.Error
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, tt.expectedCode, connectErr.Code())
			assert.Contains(t, connectErr.Message(), tt.expectedError)

			t.Logf("✅ Validation error: %s", connectErr.Message())
		})
	}
}
