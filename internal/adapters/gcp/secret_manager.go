package gcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// SecretManagerConfig contains configuration for GCP Secret Manager
type SecretManagerConfig struct {
	ProjectID string        // GCP Project ID (e.g., "my-project-123")
	CacheTTL  time.Duration // How long to cache secrets in memory (default: 5 minutes)
}

// DefaultSecretManagerConfig returns sensible defaults for GCP Secret Manager
func DefaultSecretManagerConfig(projectID string) *SecretManagerConfig {
	return &SecretManagerConfig{
		ProjectID: projectID,
		CacheTTL:  5 * time.Minute, // Cache for 5 minutes (secrets rarely change)
	}
}

// cachedSecret represents a secret with its cache metadata
type cachedSecret struct {
	secret    *ports.Secret
	expiresAt time.Time
}

// GCPSecretManager implements ports.SecretManagerAdapter for Google Cloud Secret Manager
// Provides in-memory per-instance caching for performance (stateless microservice pattern)
type GCPSecretManager struct {
	client    *secretmanager.Client
	projectID string
	cacheTTL  time.Duration
	logger    *zap.Logger

	// In-memory cache (per instance, not shared across instances)
	// Safe for stateless microservices - each instance has its own cache
	cache   map[string]*cachedSecret
	cacheMu sync.RWMutex
}

// NewGCPSecretManager creates a new GCP Secret Manager adapter with in-memory caching
// Cache is per-instance (stateless microservice pattern)
// Context should have GCP credentials configured via:
//   - GOOGLE_APPLICATION_CREDENTIALS env var pointing to service account JSON
//   - Or workload identity in GKE
//   - Or default application credentials
func NewGCPSecretManager(ctx context.Context, config *SecretManagerConfig, logger *zap.Logger) (*GCPSecretManager, error) {
	if config.ProjectID == "" {
		return nil, fmt.Errorf("GCP project ID is required")
	}

	// Create GCP Secret Manager client
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP Secret Manager client: %w", err)
	}

	sm := &GCPSecretManager{
		client:    client,
		projectID: config.ProjectID,
		cacheTTL:  config.CacheTTL,
		logger:    logger,
		cache:     make(map[string]*cachedSecret),
	}

	logger.Info("GCP Secret Manager initialized",
		zap.String("project_id", config.ProjectID),
		zap.Duration("cache_ttl", config.CacheTTL),
	)

	return sm, nil
}

// Close closes the GCP Secret Manager client
func (sm *GCPSecretManager) Close() error {
	return sm.client.Close()
}

// GetSecret retrieves a secret from GCP Secret Manager with in-memory caching
// Path format: "payment-service/merchants/{merchant_id}/mac"
// GCP converts to: projects/{project_id}/secrets/{secret_name}/versions/latest
func (sm *GCPSecretManager) GetSecret(ctx context.Context, path string) (*ports.Secret, error) {
	// Check cache first (per-instance cache for stateless microservice)
	sm.cacheMu.RLock()
	cached, exists := sm.cache[path]
	sm.cacheMu.RUnlock()

	if exists && time.Now().Before(cached.expiresAt) {
		sm.logger.Debug("Secret cache hit",
			zap.String("path", path),
			zap.Time("expires_at", cached.expiresAt),
		)
		return cached.secret, nil
	}

	// Cache miss or expired - fetch from GCP
	sm.logger.Debug("Secret cache miss - fetching from GCP",
		zap.String("path", path),
	)

	// Construct GCP secret name
	// Format: projects/{project_id}/secrets/{secret_name}/versions/latest
	secretName := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", sm.projectID, path)

	// Access secret version
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}

	result, err := sm.client.AccessSecretVersion(ctx, req)
	if err != nil {
		sm.logger.Error("Failed to access GCP secret",
			zap.String("path", path),
			zap.String("secret_name", secretName),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to access GCP secret %s: %w", path, err)
	}

	// Extract secret data
	secret := &ports.Secret{
		Value:   string(result.Payload.Data),
		Version: extractVersionFromName(result.Name), // e.g., "1", "2", "latest"
		Metadata: map[string]string{
			"gcp_project_id": sm.projectID,
			"gcp_secret":     path,
		},
		CreatedAt: time.Now().Format(time.RFC3339), // GCP doesn't return CreateTime in AccessSecretVersionResponse
	}

	// Cache the secret (per-instance, in-memory)
	sm.cacheMu.Lock()
	sm.cache[path] = &cachedSecret{
		secret:    secret,
		expiresAt: time.Now().Add(sm.cacheTTL),
	}
	sm.cacheMu.Unlock()

	sm.logger.Info("Secret fetched from GCP and cached",
		zap.String("path", path),
		zap.String("version", secret.Version),
		zap.Duration("cache_ttl", sm.cacheTTL),
	)

	return secret, nil
}

// GetSecretVersion retrieves a specific version of a secret
// Useful during secret rotation to access previous versions
func (sm *GCPSecretManager) GetSecretVersion(ctx context.Context, path string, version string) (*ports.Secret, error) {
	// Construct GCP secret name with specific version
	secretName := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", sm.projectID, path, version)

	sm.logger.Debug("Fetching specific secret version from GCP",
		zap.String("path", path),
		zap.String("version", version),
	)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}

	result, err := sm.client.AccessSecretVersion(ctx, req)
	if err != nil {
		sm.logger.Error("Failed to access GCP secret version",
			zap.String("path", path),
			zap.String("version", version),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to access GCP secret %s version %s: %w", path, version, err)
	}

	secret := &ports.Secret{
		Value:   string(result.Payload.Data),
		Version: version,
		Metadata: map[string]string{
			"gcp_project_id": sm.projectID,
			"gcp_secret":     path,
		},
		CreatedAt: time.Now().Format(time.RFC3339), // GCP doesn't return CreateTime in AccessSecretVersionResponse
	}

	sm.logger.Info("Specific secret version fetched from GCP",
		zap.String("path", path),
		zap.String("version", version),
	)

	return secret, nil
}

// PutSecret creates or updates a secret in GCP Secret Manager
// For updating, GCP creates a new version automatically
func (sm *GCPSecretManager) PutSecret(ctx context.Context, path string, value string, metadata map[string]string) (string, error) {
	sm.logger.Info("Creating/updating secret in GCP",
		zap.String("path", path),
	)

	// First, try to add a new version to existing secret
	secretName := fmt.Sprintf("projects/%s/secrets/%s", sm.projectID, path)

	addReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(value),
		},
	}

	result, err := sm.client.AddSecretVersion(ctx, addReq)
	if err != nil {
		// If secret doesn't exist, create it first
		sm.logger.Debug("Secret doesn't exist, creating new secret",
			zap.String("path", path),
		)

		createReq := &secretmanagerpb.CreateSecretRequest{
			Parent:   fmt.Sprintf("projects/%s", sm.projectID),
			SecretId: path,
			Secret: &secretmanagerpb.Secret{
				Replication: &secretmanagerpb.Replication{
					Replication: &secretmanagerpb.Replication_Automatic_{
						Automatic: &secretmanagerpb.Replication_Automatic{},
					},
				},
			},
		}

		_, err := sm.client.CreateSecret(ctx, createReq)
		if err != nil {
			sm.logger.Error("Failed to create GCP secret",
				zap.String("path", path),
				zap.Error(err),
			)
			return "", fmt.Errorf("failed to create GCP secret %s: %w", path, err)
		}

		// Now add the version
		result, err = sm.client.AddSecretVersion(ctx, addReq)
		if err != nil {
			sm.logger.Error("Failed to add version to newly created secret",
				zap.String("path", path),
				zap.Error(err),
			)
			return "", fmt.Errorf("failed to add version to GCP secret %s: %w", path, err)
		}
	}

	version := extractVersionFromName(result.Name)

	// Invalidate cache for this secret
	sm.cacheMu.Lock()
	delete(sm.cache, path)
	sm.cacheMu.Unlock()

	sm.logger.Info("Secret created/updated in GCP",
		zap.String("path", path),
		zap.String("version", version),
	)

	return version, nil
}

// RotateSecret rotates a secret by creating a new version
// GCP automatically handles versioning - old versions remain accessible
func (sm *GCPSecretManager) RotateSecret(ctx context.Context, path string, newValue string) (*ports.SecretRotationInfo, error) {
	sm.logger.Info("Rotating secret in GCP",
		zap.String("path", path),
	)

	// Get current version before rotation
	currentSecret, err := sm.GetSecret(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get current secret version: %w", err)
	}

	// Create new version
	newVersion, err := sm.PutSecret(ctx, path, newValue, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new secret version: %w", err)
	}

	rotationInfo := &ports.SecretRotationInfo{
		CurrentVersion:  newVersion,
		PreviousVersion: currentSecret.Version,
		NextRotation:    "", // GCP doesn't have built-in rotation scheduling
	}

	sm.logger.Info("Secret rotated successfully",
		zap.String("path", path),
		zap.String("previous_version", rotationInfo.PreviousVersion),
		zap.String("current_version", rotationInfo.CurrentVersion),
	)

	return rotationInfo, nil
}

// DeleteSecret permanently deletes a secret and all its versions
// USE WITH EXTREME CAUTION - this is irreversible
func (sm *GCPSecretManager) DeleteSecret(ctx context.Context, path string) error {
	secretName := fmt.Sprintf("projects/%s/secrets/%s", sm.projectID, path)

	sm.logger.Warn("DELETING SECRET - this is irreversible",
		zap.String("path", path),
		zap.String("secret_name", secretName),
	)

	req := &secretmanagerpb.DeleteSecretRequest{
		Name: secretName,
	}

	err := sm.client.DeleteSecret(ctx, req)
	if err != nil {
		sm.logger.Error("Failed to delete GCP secret",
			zap.String("path", path),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete GCP secret %s: %w", path, err)
	}

	// Remove from cache
	sm.cacheMu.Lock()
	delete(sm.cache, path)
	sm.cacheMu.Unlock()

	sm.logger.Info("Secret deleted from GCP",
		zap.String("path", path),
	)

	return nil
}

// extractVersionFromName extracts version number from GCP secret version name
// Format: projects/{project}/secrets/{secret}/versions/{version}
func extractVersionFromName(name string) string {
	// name format: projects/123/secrets/my-secret/versions/1
	// We want to extract "1"
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' {
			return name[i+1:]
		}
	}
	return "unknown"
}
