package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// VaultConfig contains configuration for HashiCorp Vault adapter
type VaultConfig struct {
	// Vault server address (e.g., "https://vault.example.com:8200")
	Address string

	// Authentication method: "token", "approle", "kubernetes"
	AuthMethod string

	// Token for token authentication
	Token string

	// AppRole credentials (if using AppRole auth)
	RoleID   string
	SecretID string

	// Kubernetes service account token path (if using Kubernetes auth)
	K8sTokenPath string
	K8sRole      string

	// Vault namespace (Vault Enterprise)
	Namespace string

	// KV secrets engine mount path (default: "secret")
	MountPath string

	// KV version: "v1" or "v2" (default: "v2")
	KVVersion string

	// Cache TTL
	CacheTTL time.Duration

	// Enable caching
	EnableCache bool

	// TLS configuration
	TLSSkipVerify bool
}

// DefaultVaultConfig returns default configuration for Vault adapter
func DefaultVaultConfig(address string) *VaultConfig {
	return &VaultConfig{
		Address:     address,
		AuthMethod:  "token",
		MountPath:   "secret",
		KVVersion:   "v2",
		CacheTTL:    5 * time.Minute,
		EnableCache: true,
	}
}

// vaultAdapter implements the SecretManagerAdapter port for HashiCorp Vault
type vaultAdapter struct {
	client *vault.Client
	config *VaultConfig
	logger *zap.Logger
	cache  *secretCache
}

// NewVaultAdapter creates a new HashiCorp Vault adapter
func NewVaultAdapter(ctx context.Context, cfg *VaultConfig, logger *zap.Logger) (ports.SecretManagerAdapter, error) {
	// Create Vault client config
	vaultConfig := vault.DefaultConfig()
	vaultConfig.Address = cfg.Address

	if cfg.TLSSkipVerify {
		tlsConfig := &vault.TLSConfig{
			Insecure: true,
		}
		if err := vaultConfig.ConfigureTLS(tlsConfig); err != nil {
			return nil, fmt.Errorf("failed to configure TLS: %w", err)
		}
	}

	// Create Vault client
	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Set namespace if using Vault Enterprise
	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}

	// Authenticate
	if err := authenticateVault(ctx, client, cfg); err != nil {
		return nil, fmt.Errorf("failed to authenticate with Vault: %w", err)
	}

	logger.Info("Vault adapter initialized",
		zap.String("address", cfg.Address),
		zap.String("auth_method", cfg.AuthMethod),
		zap.String("mount_path", cfg.MountPath),
		zap.String("kv_version", cfg.KVVersion),
	)

	return &vaultAdapter{
		client: client,
		config: cfg,
		logger: logger,
		cache: &secretCache{
			entries: make(map[string]*cacheEntry),
			enabled: cfg.EnableCache,
			ttl:     cfg.CacheTTL,
		},
	}, nil
}

// authenticateVault handles authentication with Vault
func authenticateVault(ctx context.Context, client *vault.Client, cfg *VaultConfig) error {
	switch cfg.AuthMethod {
	case "token":
		if cfg.Token == "" {
			return fmt.Errorf("token is required for token auth")
		}
		client.SetToken(cfg.Token)
		return nil

	case "approle":
		if cfg.RoleID == "" || cfg.SecretID == "" {
			return fmt.Errorf("role_id and secret_id are required for AppRole auth")
		}

		// AppRole login
		data := map[string]interface{}{
			"role_id":   cfg.RoleID,
			"secret_id": cfg.SecretID,
		}
		resp, err := client.Logical().Write("auth/approle/login", data)
		if err != nil {
			return fmt.Errorf("AppRole login failed: %w", err)
		}
		if resp.Auth == nil {
			return fmt.Errorf("AppRole login returned no auth info")
		}
		client.SetToken(resp.Auth.ClientToken)
		return nil

	case "kubernetes":
		if cfg.K8sTokenPath == "" || cfg.K8sRole == "" {
			return fmt.Errorf("k8s_token_path and k8s_role are required for Kubernetes auth")
		}

		// Read service account token
		// jwt, err := os.ReadFile(cfg.K8sTokenPath)
		// if err != nil {
		// 	return fmt.Errorf("failed to read k8s token: %w", err)
		// }

		// Kubernetes auth login
		// data := map[string]interface{}{
		// 	"jwt":  string(jwt),
		// 	"role": cfg.K8sRole,
		// }
		// resp, err := client.Logical().Write("auth/kubernetes/login", data)
		// if err != nil {
		// 	return fmt.Errorf("Kubernetes login failed: %w", err)
		// }
		// if resp.Auth == nil {
		// 	return fmt.Errorf("Kubernetes login returned no auth info")
		// }
		// client.SetToken(resp.Auth.ClientToken)
		return fmt.Errorf("kubernetes auth not fully implemented yet")

	default:
		return fmt.Errorf("unsupported auth method: %s", cfg.AuthMethod)
	}
}

// GetSecret retrieves a secret by its path
// Path format: "payment-service/agents/{agent_id}/mac"
func (a *vaultAdapter) GetSecret(ctx context.Context, path string) (*ports.Secret, error) {
	// Check cache first
	if cached := a.cache.get(path); cached != nil {
		a.logger.Debug("Secret retrieved from cache", zap.String("path", path))
		return cached, nil
	}

	a.logger.Info("Retrieving secret from Vault", zap.String("path", path))

	// Build full path based on KV version
	var fullPath string
	if a.config.KVVersion == "v2" {
		fullPath = fmt.Sprintf("%s/data/%s", a.config.MountPath, path)
	} else {
		fullPath = fmt.Sprintf("%s/%s", a.config.MountPath, path)
	}

	startTime := time.Now()
	secret, err := a.client.Logical().Read(fullPath)
	if err != nil {
		a.logger.Error("Failed to retrieve secret from Vault",
			zap.String("path", path),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to read secret from Vault: %w", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret not found: %s", path)
	}

	a.logger.Info("Secret retrieved successfully",
		zap.String("path", path),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	// Extract secret data based on KV version
	var secretData map[string]interface{}
	var version string
	var createdTime string

	if a.config.KVVersion == "v2" {
		// KV v2 wraps data in "data" field
		data, ok := secret.Data["data"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid secret format from Vault")
		}
		secretData = data

		// Get metadata
		if metadata, ok := secret.Data["metadata"].(map[string]interface{}); ok {
			if v, ok := metadata["version"].(json.Number); ok {
				version = v.String()
			}
			if ct, ok := metadata["created_time"].(string); ok {
				createdTime = ct
			}
		}
	} else {
		// KV v1 returns data directly
		secretData = secret.Data
		version = "1"
	}

	// Extract the actual secret value
	// Assuming the secret is stored under "value" key
	var secretValue string
	if val, ok := secretData["value"].(string); ok {
		secretValue = val
	} else {
		// If no "value" key, try to extract first string value
		for _, v := range secretData {
			if str, ok := v.(string); ok {
				secretValue = str
				break
			}
		}
	}

	if secretValue == "" {
		return nil, fmt.Errorf("secret value is empty or not found")
	}

	result := &ports.Secret{
		Value:     secretValue,
		Version:   version,
		CreatedAt: createdTime,
		Metadata:  make(map[string]string),
	}

	// Add all secret data as metadata
	for k, v := range secretData {
		if str, ok := v.(string); ok && k != "value" {
			result.Metadata[k] = str
		}
	}

	// Cache the secret
	a.cache.set(path, result)

	return result, nil
}

// GetSecretVersion retrieves a specific version of a secret (KV v2 only)
func (a *vaultAdapter) GetSecretVersion(ctx context.Context, path string, version string) (*ports.Secret, error) {
	if a.config.KVVersion != "v2" {
		return nil, fmt.Errorf("GetSecretVersion requires KV v2")
	}

	a.logger.Info("Retrieving secret version from Vault",
		zap.String("path", path),
		zap.String("version", version),
	)

	fullPath := fmt.Sprintf("%s/data/%s?version=%s", a.config.MountPath, path, version)

	secret, err := a.client.Logical().Read(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret version: %w", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret version not found: %s v%s", path, version)
	}

	// Extract data (same as GetSecret)
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid secret format")
	}

	var secretValue string
	if val, ok := data["value"].(string); ok {
		secretValue = val
	}

	return &ports.Secret{
		Value:     secretValue,
		Version:   version,
		Metadata:  make(map[string]string),
	}, nil
}

// PutSecret creates or updates a secret
func (a *vaultAdapter) PutSecret(ctx context.Context, path string, value string, metadata map[string]string) (string, error) {
	a.logger.Info("Putting secret to Vault", zap.String("path", path))

	// Build secret data
	secretData := map[string]interface{}{
		"value": value,
	}

	// Add metadata fields
	for k, v := range metadata {
		secretData[k] = v
	}

	// Build full path based on KV version
	var fullPath string
	var writeData map[string]interface{}

	if a.config.KVVersion == "v2" {
		fullPath = fmt.Sprintf("%s/data/%s", a.config.MountPath, path)
		writeData = map[string]interface{}{
			"data": secretData,
		}
	} else {
		fullPath = fmt.Sprintf("%s/%s", a.config.MountPath, path)
		writeData = secretData
	}

	// Write secret
	resp, err := a.client.Logical().Write(fullPath, writeData)
	if err != nil {
		a.logger.Error("Failed to write secret to Vault",
			zap.String("path", path),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to write secret: %w", err)
	}

	// Extract version from response (KV v2)
	version := "1"
	if a.config.KVVersion == "v2" && resp != nil && resp.Data != nil {
		if v, ok := resp.Data["version"].(json.Number); ok {
			version = v.String()
		}
	}

	a.logger.Info("Secret written successfully",
		zap.String("path", path),
		zap.String("version", version),
	)

	// Invalidate cache
	a.cache.invalidate(path)

	return version, nil
}

// RotateSecret rotates a secret by creating a new version
func (a *vaultAdapter) RotateSecret(ctx context.Context, path string, newValue string) (*ports.SecretRotationInfo, error) {
	a.logger.Info("Rotating secret", zap.String("path", path))

	// Get current version
	currentSecret, err := a.GetSecret(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get current secret: %w", err)
	}

	// Put new version
	newVersion, err := a.PutSecret(ctx, path, newValue, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to write new secret version: %w", err)
	}

	a.logger.Info("Secret rotated successfully",
		zap.String("path", path),
		zap.String("previous_version", currentSecret.Version),
		zap.String("new_version", newVersion),
	)

	return &ports.SecretRotationInfo{
		CurrentVersion:  newVersion,
		PreviousVersion: currentSecret.Version,
		NextRotation:    "",
	}, nil
}

// DeleteSecret permanently deletes a secret
func (a *vaultAdapter) DeleteSecret(ctx context.Context, path string) error {
	a.logger.Warn("Deleting secret from Vault", zap.String("path", path))

	var fullPath string
	if a.config.KVVersion == "v2" {
		// KV v2: delete metadata (permanent delete)
		fullPath = fmt.Sprintf("%s/metadata/%s", a.config.MountPath, path)
	} else {
		fullPath = fmt.Sprintf("%s/%s", a.config.MountPath, path)
	}

	_, err := a.client.Logical().Delete(fullPath)
	if err != nil {
		a.logger.Error("Failed to delete secret from Vault",
			zap.String("path", path),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	a.logger.Info("Secret deleted successfully", zap.String("path", path))

	// Invalidate cache
	a.cache.invalidate(path)

	return nil
}
