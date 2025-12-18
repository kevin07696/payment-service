// Package secrets provides secret manager adapter implementations and factory.
package secrets

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/gcp"
	"github.com/kevin07696/payment-service/internal/adapters/mock"
	"github.com/kevin07696/payment-service/internal/ports"
	"go.uber.org/zap"
)

// SecretManagerType represents the type of secret manager to use
type SecretManagerType string

const (
	// SecretManagerMock uses a file-based mock for local development
	// Reads secrets from ./secrets directory - NOT for production use
	SecretManagerMock SecretManagerType = "mock"

	// SecretManagerLocal uses local filesystem for development with real credentials
	// Requires LOCAL_SECRETS_BASE_PATH environment variable - NOT for production use
	SecretManagerLocal SecretManagerType = "local"

	// SecretManagerGCP uses Google Cloud Secret Manager for production
	// Requires GCP_PROJECT_ID and GOOGLE_APPLICATION_CREDENTIALS
	SecretManagerGCP SecretManagerType = "gcp"

	// SecretManagerAWS uses AWS Secrets Manager for production
	// Requires AWS_REGION (and optionally AWS_PROFILE for local dev)
	SecretManagerAWS SecretManagerType = "aws"

	// SecretManagerVault uses HashiCorp Vault for enterprise deployments
	// Requires VAULT_ADDR and authentication configuration
	SecretManagerVault SecretManagerType = "vault"
)

// FactoryConfig contains configuration for creating secret managers
type FactoryConfig struct {
	// Type specifies which secret manager implementation to use
	Type SecretManagerType

	// CacheTTLMinutes specifies cache TTL (default: 5 minutes)
	CacheTTLMinutes int

	// GCP-specific configuration
	GCPProjectID string

	// AWS-specific configuration
	AWSRegion   string
	AWSProfile  string
	AWSEndpoint string // For LocalStack testing

	// Vault-specific configuration
	VaultAddr       string
	VaultAuthMethod string // "token", "approle", "kubernetes"
	VaultToken      string
	VaultRoleID     string
	VaultSecretID   string
	VaultNamespace  string
	VaultMountPath  string
	VaultKVVersion  string
	VaultK8sRole    string
	VaultK8sTokenPath string

	// Local-specific configuration
	LocalBasePath string
}

// NewFactoryConfigFromEnv creates a FactoryConfig from environment variables
func NewFactoryConfigFromEnv() *FactoryConfig {
	return &FactoryConfig{
		Type:            SecretManagerType(getEnv("SECRET_MANAGER", "mock")),
		CacheTTLMinutes: getEnvInt("SECRET_CACHE_TTL_MINUTES", 5),

		// GCP
		GCPProjectID: os.Getenv("GCP_PROJECT_ID"),

		// AWS
		AWSRegion:   os.Getenv("AWS_REGION"),
		AWSProfile:  os.Getenv("AWS_PROFILE"),
		AWSEndpoint: os.Getenv("AWS_SECRETS_ENDPOINT"),

		// Vault
		VaultAddr:       os.Getenv("VAULT_ADDR"),
		VaultAuthMethod: getEnv("VAULT_AUTH_METHOD", "token"),
		VaultToken:      os.Getenv("VAULT_TOKEN"),
		VaultRoleID:     os.Getenv("VAULT_ROLE_ID"),
		VaultSecretID:   os.Getenv("VAULT_SECRET_ID"),
		VaultNamespace:  os.Getenv("VAULT_NAMESPACE"),
		VaultMountPath:  getEnv("VAULT_MOUNT_PATH", "secret"),
		VaultKVVersion:  getEnv("VAULT_KV_VERSION", "v2"),
		VaultK8sRole:    os.Getenv("VAULT_K8S_ROLE"),
		VaultK8sTokenPath: getEnv("VAULT_K8S_TOKEN_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/token"),

		// Local
		LocalBasePath: os.Getenv("LOCAL_SECRETS_BASE_PATH"),
	}
}

// NewSecretManager creates a secret manager adapter based on configuration.
// This is the main factory function for creating secret manager instances.
//
// Supported types:
//   - "mock": File-based mock for local development (reads from ./secrets/)
//   - "local": Local filesystem with configurable base path
//   - "gcp": Google Cloud Secret Manager (production)
//   - "aws": AWS Secrets Manager (production)
//   - "vault": HashiCorp Vault (enterprise)
//
// Environment Variables:
//   - SECRET_MANAGER: "mock", "local", "gcp", "aws", or "vault" (default: "mock")
//   - SECRET_CACHE_TTL_MINUTES: Cache TTL in minutes (default: 5)
//   - GCP_PROJECT_ID: Required for GCP
//   - AWS_REGION: Required for AWS
//   - VAULT_ADDR: Required for Vault
//   - LOCAL_SECRETS_BASE_PATH: Required for local
func NewSecretManager(ctx context.Context, config *FactoryConfig, logger *zap.Logger) (ports.SecretManagerAdapter, error) {
	if config == nil {
		config = NewFactoryConfigFromEnv()
	}

	switch config.Type {
	case SecretManagerGCP:
		return newGCPSecretManager(ctx, config, logger)
	case SecretManagerAWS:
		return newAWSSecretsManager(ctx, config, logger)
	case SecretManagerVault:
		return newVaultAdapter(ctx, config, logger)
	case SecretManagerLocal:
		return newLocalSecretManager(config, logger)
	case SecretManagerMock:
		return newMockSecretManager(logger), nil
	default:
		logger.Warn("Unknown SECRET_MANAGER type, falling back to mock",
			zap.String("secret_manager", string(config.Type)),
		)
		return newMockSecretManager(logger), nil
	}
}

// newGCPSecretManager creates a GCP Secret Manager adapter
func newGCPSecretManager(ctx context.Context, config *FactoryConfig, logger *zap.Logger) (ports.SecretManagerAdapter, error) {
	if config.GCPProjectID == "" {
		return nil, fmt.Errorf("GCP_PROJECT_ID environment variable is required when SECRET_MANAGER=gcp")
	}

	gcpConfig := gcp.DefaultSecretManagerConfig(config.GCPProjectID)

	if config.CacheTTLMinutes > 0 {
		gcpConfig.CacheTTL = time.Duration(config.CacheTTLMinutes) * time.Minute
	}

	sm, err := gcp.NewGCPSecretManager(ctx, gcpConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCP Secret Manager: %w", err)
	}

	logger.Info("GCP Secret Manager initialized successfully",
		zap.String("project_id", config.GCPProjectID),
		zap.Duration("cache_ttl", gcpConfig.CacheTTL),
	)

	return sm, nil
}

// newAWSSecretsManager creates an AWS Secrets Manager adapter
func newAWSSecretsManager(ctx context.Context, config *FactoryConfig, logger *zap.Logger) (ports.SecretManagerAdapter, error) {
	if config.AWSRegion == "" {
		return nil, fmt.Errorf("AWS_REGION environment variable is required when SECRET_MANAGER=aws")
	}

	awsConfig := DefaultAWSSecretsManagerConfig(config.AWSRegion)

	if config.AWSProfile != "" {
		awsConfig.Profile = config.AWSProfile
		logger.Info("Using AWS profile", zap.String("profile", config.AWSProfile))
	}

	if config.AWSEndpoint != "" {
		awsConfig.Endpoint = config.AWSEndpoint
		logger.Info("Using custom AWS endpoint", zap.String("endpoint", config.AWSEndpoint))
	}

	if config.CacheTTLMinutes > 0 {
		awsConfig.CacheTTL = time.Duration(config.CacheTTLMinutes) * time.Minute
	}

	sm, err := NewAWSSecretsManagerAdapter(ctx, awsConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AWS Secrets Manager: %w", err)
	}

	logger.Info("AWS Secrets Manager initialized successfully",
		zap.String("region", config.AWSRegion),
		zap.Duration("cache_ttl", awsConfig.CacheTTL),
	)

	return sm, nil
}

// newVaultAdapter creates a HashiCorp Vault adapter
func newVaultAdapter(ctx context.Context, config *FactoryConfig, logger *zap.Logger) (ports.SecretManagerAdapter, error) {
	if config.VaultAddr == "" {
		return nil, fmt.Errorf("VAULT_ADDR environment variable is required when SECRET_MANAGER=vault")
	}

	vaultConfig := DefaultVaultConfig(config.VaultAddr)
	vaultConfig.AuthMethod = config.VaultAuthMethod

	switch config.VaultAuthMethod {
	case "token":
		if config.VaultToken == "" {
			return nil, fmt.Errorf("VAULT_TOKEN is required for token authentication")
		}
		vaultConfig.Token = config.VaultToken

	case "approle":
		if config.VaultRoleID == "" || config.VaultSecretID == "" {
			return nil, fmt.Errorf("VAULT_ROLE_ID and VAULT_SECRET_ID are required for approle authentication")
		}
		vaultConfig.RoleID = config.VaultRoleID
		vaultConfig.SecretID = config.VaultSecretID

	case "kubernetes":
		if config.VaultK8sRole == "" {
			return nil, fmt.Errorf("VAULT_K8S_ROLE is required for kubernetes authentication")
		}
		vaultConfig.K8sRole = config.VaultK8sRole
		vaultConfig.K8sTokenPath = config.VaultK8sTokenPath
	}

	if config.VaultNamespace != "" {
		vaultConfig.Namespace = config.VaultNamespace
	}
	if config.VaultMountPath != "" {
		vaultConfig.MountPath = config.VaultMountPath
	}
	vaultConfig.KVVersion = config.VaultKVVersion

	if config.CacheTTLMinutes > 0 {
		vaultConfig.CacheTTL = time.Duration(config.CacheTTLMinutes) * time.Minute
	}

	sm, err := NewVaultAdapter(ctx, vaultConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Vault adapter: %w", err)
	}

	logger.Info("Vault adapter initialized successfully",
		zap.String("vault_addr", config.VaultAddr),
		zap.String("auth_method", config.VaultAuthMethod),
		zap.String("mount_path", vaultConfig.MountPath),
		zap.String("kv_version", vaultConfig.KVVersion),
	)

	return sm, nil
}

// newLocalSecretManager creates a local filesystem secret manager
func newLocalSecretManager(config *FactoryConfig, logger *zap.Logger) (ports.SecretManagerAdapter, error) {
	if config.LocalBasePath == "" {
		return nil, fmt.Errorf("LOCAL_SECRETS_BASE_PATH environment variable is required when SECRET_MANAGER=local")
	}

	logger.Warn("Using LOCAL file-based secret manager - NOT for production use!",
		zap.String("secret_manager", "local"),
		zap.String("base_path", config.LocalBasePath),
	)

	return NewLocalSecretManager(config.LocalBasePath, logger), nil
}

// newMockSecretManager creates a mock secret manager for development
func newMockSecretManager(logger *zap.Logger) ports.SecretManagerAdapter {
	logger.Warn("Using MOCK secret manager - NOT for production use!",
		zap.String("secret_manager", "mock"),
	)
	return mock.NewMockSecretManager(logger)
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
