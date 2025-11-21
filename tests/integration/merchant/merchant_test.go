//go:build integration
// +build integration

package merchant_test

import (
	"testing"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMerchant_FromSeedData(t *testing.T) {
	t.Skip("AgentService is gRPC-only (no HTTP REST endpoints). Use gRPC client to test agent/merchant management.")

	// NOTE: The AgentService is designed as an internal/admin-only service
	// accessible via gRPC only. To test agent retrieval, use a gRPC client:
	//
	// import agentv1 "github.com/kevin07696/payment-service/proto/agent/v1"
	// conn, _ := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	// client := agentv1.NewAgentServiceClient(conn)
	// resp, _ := client.GetAgent(ctx, &agentv1.GetAgentRequest{AgentId: "test-merchant-staging"})
}

func TestHealthCheck(t *testing.T) {
	testutil.Setup(t) // Seed test merchant

	// Health endpoint is on HTTP server (port 8081), not ConnectRPC server (port 8080)
	httpClient := testutil.NewClient("http://localhost:8081")

	resp, err := httpClient.Do("GET", "/cron/health", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Health check should return 200")

	// Verify response structure
	var health map[string]interface{}
	err = testutil.DecodeResponse(resp, &health)
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"])
	assert.NotEmpty(t, health["time"])
}
