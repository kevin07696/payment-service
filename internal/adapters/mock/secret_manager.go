package mock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// MockSecretManager is a file-based implementation for local development
// Reads secrets from ./secrets directory structure
type MockSecretManager struct {
	logger      *zap.Logger
	secretsRoot string
}

// NewMockSecretManager creates a new mock secret manager
func NewMockSecretManager(logger *zap.Logger) *MockSecretManager {
	return &MockSecretManager{
		logger:      logger,
		secretsRoot: "./secrets", // Root directory for secrets
	}
}

// GetSecret reads a secret from the local filesystem
// Secret path format: "payments/merchants/test-merchant/mac"
// File location: "./secrets/payments/merchants/test-merchant/mac"
func (m *MockSecretManager) GetSecret(ctx context.Context, secretPath string) (*ports.Secret, error) {
	m.logger.Warn("Using mock secret manager - NOT for production use",
		zap.String("secret_path", secretPath),
	)

	// Construct file path
	filePath := filepath.Join(m.secretsRoot, secretPath)

	// Read secret from file
	data, err := os.ReadFile(filePath)
	if err != nil {
		m.logger.Error("Failed to read secret from file",
			zap.Error(err),
			zap.String("secret_path", secretPath),
			zap.String("file_path", filePath),
		)
		return nil, fmt.Errorf("failed to read secret from %s: %w", filePath, err)
	}

	// Trim whitespace from secret value
	secretValue := strings.TrimSpace(string(data))

	if secretValue == "" {
		return nil, fmt.Errorf("empty secret value in file %s", filePath)
	}

	m.logger.Info("Successfully read secret from file",
		zap.String("secret_path", secretPath),
		zap.String("file_path", filePath),
		zap.Int("value_length", len(secretValue)),
	)

	return &ports.Secret{
		Value:     secretValue,
		Version:   "file-v1",
		Metadata:  map[string]string{"environment": "development", "source": "filesystem"},
		CreatedAt: "2025-01-01T00:00:00Z",
	}, nil
}

// GetSecretVersion returns a specific version of a secret (mock implementation)
func (m *MockSecretManager) GetSecretVersion(ctx context.Context, secretPath, version string) (*ports.Secret, error) {
	m.logger.Warn("Using mock secret manager - NOT for production use",
		zap.String("secret_path", secretPath),
		zap.String("version", version),
	)

	return &ports.Secret{
		Value:     "MOCK_MAC_SECRET_FOR_DEVELOPMENT_ONLY",
		Version:   version,
		Metadata:  map[string]string{"environment": "development"},
		CreatedAt: "2025-01-01T00:00:00Z",
	}, nil
}

// PutSecret is not implemented for mock
func (m *MockSecretManager) PutSecret(ctx context.Context, secretPath, value string, metadata map[string]string) (string, error) {
	return "", fmt.Errorf("PutSecret not implemented in mock secret manager")
}

// RotateSecret is not implemented for mock
func (m *MockSecretManager) RotateSecret(ctx context.Context, secretPath, newValue string) (*ports.SecretRotationInfo, error) {
	return nil, fmt.Errorf("RotateSecret not implemented in mock secret manager")
}

// DeleteSecret is not implemented for mock
func (m *MockSecretManager) DeleteSecret(ctx context.Context, secretPath string) error {
	return fmt.Errorf("DeleteSecret not implemented in mock secret manager")
}
