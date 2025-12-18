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

// setupConnectClient creates a Connect protocol client using config from environment
func setupConnectClient(t *testing.T) (paymentv1connect.PaymentServiceClient, *testutil.Config) {
	t.Helper()

	cfg, err := testutil.LoadConfig()
	require.NoError(t, err, "Failed to load test config")

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client := paymentv1connect.NewPaymentServiceClient(
		httpClient,
		cfg.ServiceURL,
	)

	return client, cfg
}

// addAuthToRequest adds JWT authentication to a Connect request
func addAuthToRequest[T any](t *testing.T, req *connect.Request[T], ctx *testutil.TestContext) {
	t.Helper()

	// Generate JWT token
	token, err := testutil.GenerateJWT(
		ctx.Service.PrivateKeyPEM,
		ctx.Service.ServiceID,
		ctx.Merchant.ID.String(),
		1*time.Hour,
	)
	require.NoError(t, err, "Failed to generate JWT")

	// Add authorization header
	req.Header().Set("Authorization", "Bearer "+token)
}

// TestConnect_ListTransactions tests the Connect protocol ListTransactions endpoint
func TestConnect_ListTransactions(t *testing.T) {
	client, _ := setupConnectClient(t)
	factory := testutil.NewFactory(t)
	testCtx := factory.CreateTestContext(t)

	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List transactions for test merchant
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: testCtx.Merchant.ID.String(),
		Limit:      10,
		Offset:     0,
	})
	addAuthToRequest(t, req, testCtx)

	resp, err := client.ListTransactions(reqCtx, req)
	require.NoError(t, err, "ListTransactions should succeed via Connect protocol")
	require.NotNil(t, resp, "Response should not be nil")
	require.NotNil(t, resp.Msg, "Response message should not be nil")
	// Note: Transactions slice may be nil or empty when no transactions exist - both are valid

	txCount := 0
	if resp.Msg.Transactions != nil {
		txCount = len(resp.Msg.Transactions)
	}
	t.Logf("Connect protocol: Successfully retrieved %d transactions", txCount)
}

// TestConnect_GetTransaction tests retrieving a specific transaction via Connect protocol
func TestConnect_GetTransaction(t *testing.T) {
	client, _ := setupConnectClient(t)
	factory := testutil.NewFactory(t)
	testCtx := factory.CreateTestContext(t)

	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First, list transactions to get a valid ID
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: testCtx.Merchant.ID.String(),
		Limit:      1,
		Offset:     0,
	})
	addAuthToRequest(t, listReq, testCtx)

	listResp, err := client.ListTransactions(reqCtx, listReq)
	require.NoError(t, err)

	if len(listResp.Msg.Transactions) == 0 {
		t.Skip("No transactions available for testing")
	}

	// Get the first transaction
	transactionID := listResp.Msg.Transactions[0].Id
	getReq := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: transactionID,
	})
	addAuthToRequest(t, getReq, testCtx)

	tx, err := client.GetTransaction(reqCtx, getReq)
	require.NoError(t, err, "GetTransaction should succeed via Connect protocol")
	assert.Equal(t, transactionID, tx.Msg.Id)

	t.Logf("Connect protocol: Successfully retrieved transaction %s", transactionID)
}

// TestConnect_ServiceAvailability tests that Connect protocol is available
func TestConnect_ServiceAvailability(t *testing.T) {
	client, cfg := setupConnectClient(t)
	factory := testutil.NewFactory(t)
	testCtx := factory.CreateTestContext(t)

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try a simple list request to verify service availability
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: testCtx.Merchant.ID.String(),
		Limit:      1,
		Offset:     0,
	})
	addAuthToRequest(t, req, testCtx)

	_, err := client.ListTransactions(reqCtx, req)
	require.NoError(t, err, "Service should be available via Connect protocol")

	t.Log("Connect protocol PaymentService is available at " + cfg.ServiceURL)
}

// TestConnect_ErrorHandling tests that errors are properly propagated through Connect
func TestConnect_ErrorHandling(t *testing.T) {
	client, _ := setupConnectClient(t)
	factory := testutil.NewFactory(t)
	testCtx := factory.CreateTestContext(t)

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get a non-existent transaction (use valid UUID format)
	req := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: "00000000-0000-0000-0000-000000000000",
	})
	addAuthToRequest(t, req, testCtx)

	_, err := client.GetTransaction(reqCtx, req)
	require.Error(t, err, "Should return error for non-existent transaction")

	// Verify it's a Connect error with the right code
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code(), "Should return NotFound error code")

	t.Logf("Connect protocol: Error handling works correctly (got %v)", connectErr.Code())
}

// TestConnect_ListTransactionsByGroup tests filtering by parent_transaction_id via Connect protocol
func TestConnect_ListTransactionsByGroup(t *testing.T) {
	client, _ := setupConnectClient(t)
	factory := testutil.NewFactory(t)
	testCtx := factory.CreateTestContext(t)

	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Query all transactions to find one with parent_transaction_id set
	// (REFUND, CAPTURE, VOID transactions have parent_transaction_id)
	listReq := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: testCtx.Merchant.ID.String(),
		Limit:      100,
		Offset:     0,
	})
	addAuthToRequest(t, listReq, testCtx)

	listResp, err := client.ListTransactions(reqCtx, listReq)
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
		MerchantId:          testCtx.Merchant.ID.String(),
		ParentTransactionId: parentTxID,
		Limit:               100,
		Offset:              0,
	})
	addAuthToRequest(t, groupReq, testCtx)

	groupResp, err := client.ListTransactions(reqCtx, groupReq)
	require.NoError(t, err, "ListTransactions by parent_transaction_id should succeed")
	assert.NotNil(t, groupResp)
	assert.GreaterOrEqual(t, len(groupResp.Msg.Transactions), 1, "Should have at least 1 transaction in group")

	// Verify all transactions have same parent_transaction_id
	for _, tx := range groupResp.Msg.Transactions {
		assert.Equal(t, parentTxID, tx.ParentTransactionId, "All transactions should have same parent_transaction_id")
	}

	t.Logf("Connect protocol: Successfully retrieved %d transactions for parent_transaction_id %s",
		len(groupResp.Msg.Transactions), parentTxID)
}

// TestConnect_Headers tests that Connect headers are properly handled
func TestConnect_Headers(t *testing.T) {
	client, _ := setupConnectClient(t)
	factory := testutil.NewFactory(t)
	testCtx := factory.CreateTestContext(t)

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request with custom headers
	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: testCtx.Merchant.ID.String(),
		Limit:      1,
		Offset:     0,
	})

	// Add custom header
	req.Header().Set("X-Test-Header", "test-value")

	// Add authentication
	addAuthToRequest(t, req, testCtx)

	resp, err := client.ListTransactions(reqCtx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	t.Log("Connect protocol: Headers are properly handled")
}
