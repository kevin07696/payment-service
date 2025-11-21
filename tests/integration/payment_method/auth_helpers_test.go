//go:build integration
// +build integration

package payment_method_test

import (
	"testing"
	"time"

	"github.com/kevin07696/payment-service/tests/integration/testutil"
	"github.com/stretchr/testify/require"
)

// generateJWTToken generates a JWT token for API requests
func generateJWTToken(t *testing.T, merchantID string) string {
	t.Helper()

	services, err := testutil.LoadTestServices()
	require.NoError(t, err, "Failed to load test services")
	require.NotEmpty(t, services, "No test services available")

	token, err := testutil.GenerateJWT(
		services[0].PrivateKeyPEM,
		services[0].ServiceID,
		merchantID,
		1*time.Hour,
	)
	require.NoError(t, err, "Failed to generate JWT")

	return token
}
