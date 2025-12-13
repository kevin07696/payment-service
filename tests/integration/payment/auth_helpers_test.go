//go:build integration
// +build integration

package payment_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/require"
)

// addJWTAuth adds JWT authentication to a Connect request using factory-created service
func addJWTAuth[T any](t *testing.T, req *connect.Request[T], cfg *testutil.Config, merchantID string) {
	t.Helper()

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	token, err := testutil.GenerateJWT(
		ctx.Service.PrivateKeyPEM,
		ctx.Service.ServiceID,
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err, "Failed to generate JWT")

	req.Header().Set("Authorization", "Bearer "+token)
}

// generateJWTToken generates a JWT token for API requests using factory-created service
func generateJWTToken(t *testing.T, merchantID string) string {
	t.Helper()

	factory := testutil.NewFactory(t)
	ctx := factory.CreateTestContext(t)

	token, err := testutil.GenerateJWT(
		ctx.Service.PrivateKeyPEM,
		ctx.Service.ServiceID,
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err, "Failed to generate JWT")

	return token
}
