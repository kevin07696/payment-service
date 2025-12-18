//go:build integration
// +build integration

package payment_method_test

import (
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/require"
)

// generateJWTToken generates a JWT token for API requests using factory-created service
func generateJWTToken(t *testing.T, factory *testutil.TestFactory, ctx *testutil.TestContext) string {
	t.Helper()

	token, err := testutil.GenerateJWT(
		ctx.Service.PrivateKeyPEM,
		ctx.Service.ServiceID,
		ctx.Merchant.ID.String(),
		1*time.Hour,
	)
	require.NoError(t, err, "Failed to generate JWT")

	return token
}
