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

// TestConnect_ListTransactions tests the Connect protocol ListTransactions endpoint
func TestConnect_ListTransactions(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List transactions for test merchant
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: "test-merchant-staging",
		Limit:      10,
		Offset:     0,
	})

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
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: "test-merchant-staging",
		Limit:      1,
		Offset:     0,
	})

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
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: "test-merchant-staging",
		Limit:      1,
		Offset:     0,
	})

	_, err := client.ListTransactions(ctx, req)
	require.NoError(t, err, "Service should be available via Connect protocol")

	t.Log("✅ Connect protocol PaymentService is available at " + connectAddress)
}

// TestConnect_ErrorHandling tests that errors are properly propagated through Connect
func TestConnect_ErrorHandling(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get a non-existent transaction
	req := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: "non-existent-id-12345",
	})

	_, err := client.GetTransaction(ctx, req)
	require.Error(t, err, "Should return error for non-existent transaction")

	// Verify it's a Connect error with the right code
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code(), "Should return NotFound error code")

	t.Logf("✅ Connect protocol: Error handling works correctly (got %v)", connectErr.Code())
}

// TestConnect_ListTransactionsByGroup tests filtering by group_id via Connect protocol
func TestConnect_ListTransactionsByGroup(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First, get a transaction to find a valid group_id
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: "test-merchant-staging",
		Limit:      1,
		Offset:     0,
	})

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)

	if len(listResp.Msg.Transactions) == 0 {
		t.Skip("No transactions available for testing")
	}

	groupID := listResp.Msg.Transactions[0].GroupId
	require.NotEmpty(t, groupID, "Transaction should have group_id")

	// Now list transactions by group_id
	groupReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: "test-merchant-staging",
		GroupId:    groupID,
		Limit:      100,
		Offset:     0,
	})

	groupResp, err := client.ListTransactions(ctx, groupReq)
	require.NoError(t, err, "ListTransactions by group_id should succeed")
	assert.NotNil(t, groupResp)
	assert.GreaterOrEqual(t, len(groupResp.Msg.Transactions), 1, "Should have at least 1 transaction in group")

	// Verify all transactions have same group_id
	for _, tx := range groupResp.Msg.Transactions {
		assert.Equal(t, groupID, tx.GroupId, "All transactions should have same group_id")
	}

	t.Logf("✅ Connect protocol: Successfully retrieved %d transactions for group %s",
		len(groupResp.Msg.Transactions), groupID)
}

// TestConnect_Headers tests that Connect headers are properly handled
func TestConnect_Headers(t *testing.T) {
	client := setupConnectClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request with custom headers
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: "test-merchant-staging",
		Limit:      1,
		Offset:     0,
	})

	// Add custom header
	req.Header().Set("X-Test-Header", "test-value")

	resp, err := client.ListTransactions(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	t.Log("✅ Connect protocol: Headers are properly handled")
}
