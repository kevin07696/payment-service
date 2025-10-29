package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// localSecretManager implements SecretManagerAdapter using local filesystem
// WARNING: This is for development only. Use AWS Secrets Manager or Vault in production.
type localSecretManager struct {
	basePath string
	logger   *zap.Logger
}

// NewLocalSecretManager creates a new local filesystem secret manager
func NewLocalSecretManager(basePath string, logger *zap.Logger) ports.SecretManagerAdapter {
	return &localSecretManager{
		basePath: basePath,
		logger:   logger,
	}
}

// GetSecret retrieves a secret from the local filesystem
func (m *localSecretManager) GetSecret(ctx context.Context, secretPath string) (*ports.Secret, error) {
	filePath := filepath.Join(m.basePath, secretPath)

	m.logger.Debug("Reading secret from filesystem",
		zap.String("path", secretPath),
	)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("secret not found: %s", secretPath)
		}
		return nil, fmt.Errorf("failed to read secret: %w", err)
	}

	// Support both plain text and JSON format
	var secretData struct {
		Value     string            `json:"value"`
		Tags      map[string]string `json:"tags"`
		CreatedAt string            `json:"created_at"`
	}
	if err := json.Unmarshal(data, &secretData); err == nil {
		return &ports.Secret{
			Value:     secretData.Value,
			Version:   "v1",
			Metadata:  secretData.Tags,
			CreatedAt: secretData.CreatedAt,
		}, nil
	}

	// Return as plain text if not JSON
	return &ports.Secret{
		Value:   string(data),
		Version: "v1",
	}, nil
}

// PutSecret stores a secret in the local filesystem
func (m *localSecretManager) PutSecret(ctx context.Context, secretPath, secretValue string, tags map[string]string) (string, error) {
	filePath := filepath.Join(m.basePath, secretPath)

	m.logger.Info("Storing secret to filesystem",
		zap.String("path", secretPath),
	)

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Store as JSON with metadata
	secretData := map[string]interface{}{
		"value":      secretValue,
		"tags":       tags,
		"created_at": time.Now().UTC(),
	}

	data, err := json.MarshalIndent(secretData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal secret: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write secret: %w", err)
	}

	return secretPath, nil
}

// DeleteSecret removes a secret from the local filesystem
func (m *localSecretManager) DeleteSecret(ctx context.Context, secretPath string) error {
	filePath := filepath.Join(m.basePath, secretPath)

	m.logger.Info("Deleting secret from filesystem",
		zap.String("path", secretPath),
	)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("secret not found: %s", secretPath)
		}
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return nil
}

// GetSecretVersion retrieves a specific version of a secret
// For local filesystem, we only support "latest" version
func (m *localSecretManager) GetSecretVersion(ctx context.Context, path string, version string) (*ports.Secret, error) {
	// For local filesystem, all versions are "latest"
	return m.GetSecret(ctx, path)
}

// RotateSecret rotates a secret (updates it with a new value)
func (m *localSecretManager) RotateSecret(ctx context.Context, secretPath, newValue string) (*ports.SecretRotationInfo, error) {
	_, err := m.PutSecret(ctx, secretPath, newValue, nil)
	if err != nil {
		return nil, err
	}

	return &ports.SecretRotationInfo{
		CurrentVersion:  "v1",
		PreviousVersion: "v0",
	}, nil
}
