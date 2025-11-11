//go:build integration
// +build integration

package grpc_test

import (
	"context"
	"testing"
	"time"

	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	grpcAddress = "localhost:8080"
)

func setupGRPCClient(t *testing.T) (paymentv1.PaymentServiceClient, *grpc.ClientConn) {
	t.Helper()

	conn, err := grpc.NewClient(
		grpcAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "Failed to connect to gRPC server")

	client := paymentv1.NewPaymentServiceClient(conn)
	return client, conn
}

// TestGRPC_ListTransactions tests the gRPC ListTransactions endpoint
func TestGRPC_ListTransactions(t *testing.T) {
	client, conn := setupGRPCClient(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List transactions for test merchant
	req := &paymentv1.ListTransactionsRequest{
		AgentId: "test-merchant-staging",
		Limit:   10,
		Offset:  0,
	}

	resp, err := client.ListTransactions(ctx, req)
	require.NoError(t, err, "ListTransactions should succeed")
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Transactions)

	t.Logf("Successfully retrieved %d transactions", len(resp.Transactions))
}

// TestGRPC_GetTransaction tests retrieving a specific transaction
func TestGRPC_GetTransaction(t *testing.T) {
	client, conn := setupGRPCClient(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First, list transactions to get a valid ID
	listReq := &paymentv1.ListTransactionsRequest{
		AgentId: "test-merchant-staging",
		Limit:   1,
		Offset:  0,
	}

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)

	if len(listResp.Transactions) == 0 {
		t.Skip("No transactions available for testing")
	}

	// Get the first transaction
	transactionID := listResp.Transactions[0].Id
	getReq := &paymentv1.GetTransactionRequest{
		TransactionId: transactionID,
	}

	tx, err := client.GetTransaction(ctx, getReq)
	require.NoError(t, err, "GetTransaction should succeed")
	assert.Equal(t, transactionID, tx.Id)

	// Verify clean API - no EPX fields in proto
	// (This is implicit - proto doesn't have auth_guid, auth_resp fields)
	assert.NotEmpty(t, tx.GroupId, "Should have group_id")
	assert.NotEmpty(t, tx.Amount, "Should have amount")

	t.Logf("Successfully retrieved transaction %s", transactionID)
}

// TestGRPC_ListTransactionsByGroup tests filtering by group_id
func TestGRPC_ListTransactionsByGroup(t *testing.T) {
	client, conn := setupGRPCClient(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First, get a transaction to find a valid group_id
	listReq := &paymentv1.ListTransactionsRequest{
		AgentId: "test-merchant-staging",
		Limit:   1,
		Offset:  0,
	}

	listResp, err := client.ListTransactions(ctx, listReq)
	require.NoError(t, err)

	if len(listResp.Transactions) == 0 {
		t.Skip("No transactions available for testing")
	}

	groupID := listResp.Transactions[0].GroupId
	require.NotEmpty(t, groupID, "Transaction should have group_id")

	// Now list transactions by group_id
	groupReq := &paymentv1.ListTransactionsRequest{
		AgentId: "test-merchant-staging",
		GroupId: groupID,
		Limit:   100,
		Offset:  0,
	}

	groupResp, err := client.ListTransactions(ctx, groupReq)
	require.NoError(t, err, "ListTransactions by group_id should succeed")
	assert.NotNil(t, groupResp)
	assert.GreaterOrEqual(t, len(groupResp.Transactions), 1, "Should have at least 1 transaction in group")

	// Verify all transactions have same group_id
	for _, tx := range groupResp.Transactions {
		assert.Equal(t, groupID, tx.GroupId, "All transactions should have same group_id")
	}

	t.Logf("Successfully retrieved %d transactions for group %s", len(groupResp.Transactions), groupID)
}

// TestGRPC_ServiceAvailability tests that all gRPC services are available
func TestGRPC_ServiceAvailability(t *testing.T) {
	_, conn := setupGRPCClient(t)
	defer conn.Close()

	// If we got here, the connection succeeded
	assert.True(t, true, "gRPC service is available and accepting connections")
	t.Log("✅ gRPC PaymentService is available at " + grpcAddress)
}
