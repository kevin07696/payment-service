//go:build integration
// +build integration

package connect_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	connectAddress = "http://localhost:8080"
)

// setupConnectClient creates a Connect protocol client
func setupConnectClient(t *testing.T) paymentv1connect.PaymentServiceClient {
	t.Helper()

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client := paymentv1connect.NewPaymentServiceClient(
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

// TestConnect_ListTransactions tests the Connect protocol ListTransactions endpoint
func TestConnect_ListTransactions(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List transactions for test merchant
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: merchantID,
		Limit:      10,
		Offset:     0,
	})
	addAuthToRequest(t, req, merchantID)

	resp, err := client.ListTransactions(ctx, req)
	require.NoError(t, err, "ListTransactions should succeed via Connect protocol")
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Transactions)

	t.Logf("✅ Connect protocol: Successfully retrieved %d transactions", len(resp.Msg.Transactions))
}

// TestConnect_GetTransaction tests retrieving a specific transaction via Connect protocol
func TestConnect_GetTransaction(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First, list transactions to get a valid ID
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: merchantID,
		Limit:      1,
		Offset:     0,
	})
	addAuthToRequest(t, listReq, merchantID)

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)

	if len(listResp.Msg.Transactions) == 0 {
		t.Skip("No transactions available for testing")
	}

	// Get the first transaction
	transactionID := listResp.Msg.Transactions[0].Id
	getReq := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: transactionID,
	})
	addAuthToRequest(t, getReq, merchantID)

	tx, err := client.GetTransaction(ctx, getReq)
	require.NoError(t, err, "GetTransaction should succeed via Connect protocol")
	assert.Equal(t, transactionID, tx.Msg.Id)

	t.Logf("✅ Connect protocol: Successfully retrieved transaction %s", transactionID)
}

// TestConnect_ServiceAvailability tests that Connect protocol is available
func TestConnect_ServiceAvailability(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try a simple list request to verify service availability
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: merchantID,
		Limit:      1,
		Offset:     0,
	})
	addAuthToRequest(t, req, merchantID)

	_, err := client.ListTransactions(ctx, req)
	require.NoError(t, err, "Service should be available via Connect protocol")

	t.Log("✅ Connect protocol PaymentService is available at " + connectAddress)
}

// TestConnect_ErrorHandling tests that errors are properly propagated through Connect
func TestConnect_ErrorHandling(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get a non-existent transaction (use valid UUID format)
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID
	req := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: "00000000-0000-0000-0000-000000000000",
	})
	addAuthToRequest(t, req, merchantID)

	_, err := client.GetTransaction(ctx, req)
	require.Error(t, err, "Should return error for non-existent transaction")

	// Verify it's a Connect error with the right code
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code(), "Should return NotFound error code")

	t.Logf("✅ Connect protocol: Error handling works correctly (got %v)", connectErr.Code())
}

// TestConnect_ListTransactionsByGroup tests filtering by parent_transaction_id via Connect protocol
func TestConnect_ListTransactionsByGroup(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	merchantID := "00000000-0000-0000-0000-000000000001"

	// Query all transactions to find one with parent_transaction_id set
	// (REFUND, CAPTURE, VOID transactions have parent_transaction_id)
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: merchantID,
		Limit:      100,
		Offset:     0,
	})
	addAuthToRequest(t, listReq, merchantID)

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)

	if len(listResp.Msg.Transactions) == 0 {
		t.Skip("No transactions available for testing")
	}

	// Find a transaction with parent_transaction_id
	var parentTxID string
	for _, tx := range listResp.Msg.Transactions {
		if tx.ParentTransactionId != "" {
			parentTxID = tx.ParentTransactionId
			break
		}
	}

	if parentTxID == "" {
		t.Skip("No transactions with parent_transaction_id found (need REFUND/CAPTURE/VOID transactions)")
	}

	// Now list transactions filtered by parent_transaction_id
	groupReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId:          merchantID,
		ParentTransactionId: parentTxID,
		Limit:               100,
		Offset:              0,
	})
	addAuthToRequest(t, groupReq, merchantID)

	groupResp, err := client.ListTransactions(ctx, groupReq)
	require.NoError(t, err, "ListTransactions by parent_transaction_id should succeed")
	assert.NotNil(t, groupResp)
	assert.GreaterOrEqual(t, len(groupResp.Msg.Transactions), 1, "Should have at least 1 transaction in group")

	// Verify all transactions have same parent_transaction_id
	for _, tx := range groupResp.Msg.Transactions {
		assert.Equal(t, parentTxID, tx.ParentTransactionId, "All transactions should have same parent_transaction_id")
	}

	t.Logf("✅ Connect protocol: Successfully retrieved %d transactions for parent_transaction_id %s",
		len(groupResp.Msg.Transactions), parentTxID)
}

// TestConnect_Headers tests that Connect headers are properly handled
func TestConnect_Headers(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request with custom headers
	merchantID := "00000000-0000-0000-0000-000000000001" // Test merchant UUID
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: merchantID,
		Limit:      1,
		Offset:     0,
	})

	// Add custom header
	req.Header().Set("X-Test-Header", "test-value")

	// Add authentication
	addAuthToRequest(t, req, merchantID)

	resp, err := client.ListTransactions(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	t.Log("✅ Connect protocol: Headers are properly handled")
}
