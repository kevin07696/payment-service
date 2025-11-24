//go:build integration
// +build integration

package payment_test

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

// TestPayment_ListTransactions tests the ListTransactions endpoint
func TestPayment_ListTransactions(t *testing.T) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := paymentv1connect.NewPaymentServiceClient(httpClient, "http://localhost:8080")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	merchantID := "00000000-0000-0000-0000-000000000001"

	// Load test services for JWT
	services, err := testutil.LoadTestServices()
	require.NoError(t, err)
	service := services[0]

	token, err := testutil.GenerateJWT(
		service.PrivateKeyPEM,
		service.ServiceID,
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err)

	req := connect.NewRequest(&paymentv1.ListTransactionsRequest{
		MerchantId: merchantID,
		Limit:      10,
		Offset:     0,
	})
	req.Header().Set("Authorization", "Bearer "+token)

	resp, err := client.ListTransactions(ctx, req)
	require.NoError(t, err, "ListTransactions should succeed")
	assert.NotNil(t, resp)
	// Note: Empty repeated fields in protobuf may be nil or empty slice
	assert.GreaterOrEqual(t, resp.Msg.TotalCount, int32(0), "Total count should be non-negative")

	t.Logf("✅ Listed %d transactions (total: %d)", len(resp.Msg.Transactions), resp.Msg.TotalCount)
}
