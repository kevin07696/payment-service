package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/mock"
	"github.com/kevin07696/payment-service/internal/ports"
	"go.uber.org/zap"
)

// secretManagerSingleton holds the singleton secret manager for tests
// PERFORMANCE: Reuses a single instance across all tests for in-memory secret storage
var (
	secretManager     ports.SecretManagerAdapter
	secretManagerOnce sync.Once
)

// GetSecretManager returns a singleton mock secret manager for integration tests.
// This allows tests to store and retrieve MAC secrets dynamically per merchant,
// matching the E2E test architecture.
//
// The mock secret manager stores secrets in-memory, providing:
// - Test isolation: each test can store unique MAC secrets
// - Fast access: no filesystem or network calls
// - Cleanup: secrets are cleared when tests complete
//
// Usage:
//
//	sm := testutil.GetSecretManager(t)
//	sm.PutSecret(ctx, "test/merchants/uuid/mac", "secret-value", nil)
//	secret, _ := sm.GetSecret(ctx, "test/merchants/uuid/mac")
func GetSecretManager(t interface {
	Helper()
}) ports.SecretManagerAdapter {
	t.Helper()

	secretManagerOnce.Do(func() {
		logger, _ := zap.NewDevelopment()
		secretManager = mock.NewMockSecretManager(logger)
	})

	return secretManager
}

// StoreTestMacSecret stores a MAC secret for a test merchant.
// Returns the secret path that should be stored on the merchant record.
//
// Usage:
//
//	macSecretPath := testutil.StoreTestMacSecret(t, merchantID, macSecret)
//	// Use macSecretPath when creating the merchant
func StoreTestMacSecret(t interface {
	Helper()
	Logf(format string, args ...interface{})
}, merchantID string, macSecret string) string {
	t.Helper()

	sm := GetSecretManager(t)
	secretPath := "integration/merchants/" + merchantID + "/mac"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := sm.PutSecret(ctx, secretPath, macSecret, map[string]string{
		"merchant_id": merchantID,
		"environment": "integration-test",
	})
	if err != nil {
		// Log but don't fail - some tests may not need MAC secrets
		t.Logf("Warning: Failed to store MAC secret for merchant %s: %v", merchantID, err)
	}

	return secretPath
}

// ResetSecretManager resets the singleton secret manager (for testing purposes only)
// WARNING: This clears all stored secrets
func ResetSecretManager() {
	secretManager = nil
	secretManagerOnce = sync.Once{}
}
