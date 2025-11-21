package main

import (
	"context"
	"os"

	"github.com/kevin07696/payment-service/internal/adapters/gcp"
	"github.com/kevin07696/payment-service/internal/adapters/mock"
	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/adapters/secrets"
	"go.uber.org/zap"
)

// initSecretManager initializes the appropriate secret manager based on environment
// Supports:
//   - GCP Secret Manager (production): Set SECRET_MANAGER=gcp and GCP_PROJECT_ID
//   - AWS Secrets Manager (production): Set SECRET_MANAGER=aws and AWS_REGION
//   - HashiCorp Vault (enterprise): Set SECRET_MANAGER=vault and VAULT_ADDR
//   - Local file-based (development): Set SECRET_MANAGER=local and LOCAL_SECRETS_BASE_PATH
//   - Mock (development/testing): Default when SECRET_MANAGER is not set
//
// Environment Variables:
//   - SECRET_MANAGER: "gcp", "aws", "vault", "local", or "mock" (default: mock)
//   - GCP_PROJECT_ID: Your GCP project ID (required when SECRET_MANAGER=gcp)
//   - GOOGLE_APPLICATION_CREDENTIALS: Path to GCP service account JSON (required for GCP)
//   - AWS_REGION: AWS region (required when SECRET_MANAGER=aws)
//   - AWS_PROFILE: AWS profile name (optional, for local development)
//   - VAULT_ADDR: Vault server address (required when SECRET_MANAGER=vault)
//   - VAULT_TOKEN: Vault token (for token auth)
//   - VAULT_ROLE_ID: Vault AppRole role ID (for approle auth)
//   - VAULT_SECRET_ID: Vault AppRole secret ID (for approle auth)
//   - LOCAL_SECRETS_BASE_PATH: Base path for local file secrets (required when SECRET_MANAGER=local)
//   - SECRET_CACHE_TTL_MINUTES: Cache TTL in minutes (default: 5)
func initSecretManager(ctx context.Context, cfg *Config, logger *zap.Logger) ports.SecretManagerAdapter {
	secretManagerType := getEnv("SECRET_MANAGER", "mock")

	switch secretManagerType {
	case "gcp":
		return initGCPSecretManager(ctx, logger)
	case "aws":
		return initAWSSecretsManager(ctx, logger)
	case "vault":
		return initVaultAdapter(ctx, logger)
	case "local":
		return initLocalSecretManager(logger)
	case "mock":
		return initMockSecretManager(logger)
	default:
		logger.Warn("Unknown SECRET_MANAGER type, falling back to mock",
			zap.String("secret_manager", secretManagerType),
		)
		return initMockSecretManager(logger)
	}
}

// initGCPSecretManager initializes Google Cloud Secret Manager
func initGCPSecretManager(ctx context.Context, logger *zap.Logger) ports.SecretManagerAdapter {
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		logger.Fatal("GCP_PROJECT_ID environment variable is required when SECRET_MANAGER=gcp")
	}

	config := gcp.DefaultSecretManagerConfig(projectID)

	// Allow customizing cache TTL via environment variable
	if cacheTTLMinutes := getEnvInt("SECRET_CACHE_TTL_MINUTES", 0); cacheTTLMinutes > 0 {
		config.CacheTTL = getEnvDuration("SECRET_CACHE_TTL_MINUTES", 5) * 60 // Convert minutes to seconds
	}

	sm, err := gcp.NewGCPSecretManager(ctx, config, logger)
	if err != nil {
		logger.Fatal("Failed to initialize GCP Secret Manager",
			zap.Error(err),
			zap.String("project_id", projectID),
		)
	}

	logger.Info("GCP Secret Manager initialized successfully",
		zap.String("project_id", projectID),
		zap.Duration("cache_ttl", config.CacheTTL),
	)

	return sm
}

// initAWSSecretsManager initializes AWS Secrets Manager
func initAWSSecretsManager(ctx context.Context, logger *zap.Logger) ports.SecretManagerAdapter {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		logger.Fatal("AWS_REGION environment variable is required when SECRET_MANAGER=aws")
	}

	config := secrets.DefaultAWSSecretsManagerConfig(region)

	// Optional: AWS profile for local development
	if profile := os.Getenv("AWS_PROFILE"); profile != "" {
		config.Profile = profile
		logger.Info("Using AWS profile", zap.String("profile", profile))
	}

	// Optional: Custom endpoint (for LocalStack)
	if endpoint := os.Getenv("AWS_SECRETS_ENDPOINT"); endpoint != "" {
		config.Endpoint = endpoint
		logger.Info("Using custom AWS endpoint", zap.String("endpoint", endpoint))
	}

	// Allow customizing cache TTL via environment variable
	if cacheTTLMinutes := getEnvInt("SECRET_CACHE_TTL_MINUTES", 0); cacheTTLMinutes > 0 {
		config.CacheTTL = getEnvDuration("SECRET_CACHE_TTL_MINUTES", 5) * 60
	}

	sm, err := secrets.NewAWSSecretsManagerAdapter(ctx, config, logger)
	if err != nil {
		logger.Fatal("Failed to initialize AWS Secrets Manager",
			zap.Error(err),
			zap.String("region", region),
		)
	}

	logger.Info("AWS Secrets Manager initialized successfully",
		zap.String("region", region),
		zap.Duration("cache_ttl", config.CacheTTL),
	)

	return sm
}

// initVaultAdapter initializes HashiCorp Vault adapter
func initVaultAdapter(ctx context.Context, logger *zap.Logger) ports.SecretManagerAdapter {
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		logger.Fatal("VAULT_ADDR environment variable is required when SECRET_MANAGER=vault")
	}

	config := secrets.DefaultVaultConfig(vaultAddr)

	// Configure authentication method
	authMethod := getEnv("VAULT_AUTH_METHOD", "token")
	config.AuthMethod = authMethod

	switch authMethod {
	case "token":
		token := os.Getenv("VAULT_TOKEN")
		if token == "" {
			logger.Fatal("VAULT_TOKEN is required for token authentication")
		}
		config.Token = token

	case "approle":
		roleID := os.Getenv("VAULT_ROLE_ID")
		secretID := os.Getenv("VAULT_SECRET_ID")
		if roleID == "" || secretID == "" {
			logger.Fatal("VAULT_ROLE_ID and VAULT_SECRET_ID are required for approle authentication")
		}
		config.RoleID = roleID
		config.SecretID = secretID

	case "kubernetes":
		role := os.Getenv("VAULT_K8S_ROLE")
		if role == "" {
			logger.Fatal("VAULT_K8S_ROLE is required for kubernetes authentication")
		}
		config.K8sRole = role
		config.K8sTokenPath = getEnv("VAULT_K8S_TOKEN_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/token")
	}

	// Optional configuration
	if namespace := os.Getenv("VAULT_NAMESPACE"); namespace != "" {
		config.Namespace = namespace
	}
	if mountPath := os.Getenv("VAULT_MOUNT_PATH"); mountPath != "" {
		config.MountPath = mountPath
	}
	config.KVVersion = getEnv("VAULT_KV_VERSION", "v2")

	// Cache TTL
	if cacheTTLMinutes := getEnvInt("SECRET_CACHE_TTL_MINUTES", 0); cacheTTLMinutes > 0 {
		config.CacheTTL = getEnvDuration("SECRET_CACHE_TTL_MINUTES", 5) * 60
	}

	sm, err := secrets.NewVaultAdapter(ctx, config, logger)
	if err != nil {
		logger.Fatal("Failed to initialize Vault adapter",
			zap.Error(err),
			zap.String("vault_addr", vaultAddr),
		)
	}

	logger.Info("Vault adapter initialized successfully",
		zap.String("vault_addr", vaultAddr),
		zap.String("auth_method", authMethod),
		zap.String("mount_path", config.MountPath),
		zap.String("kv_version", config.KVVersion),
	)

	return sm
}

// initLocalSecretManager initializes local file-based secret manager
func initLocalSecretManager(logger *zap.Logger) ports.SecretManagerAdapter {
	basePath := os.Getenv("LOCAL_SECRETS_BASE_PATH")
	if basePath == "" {
		logger.Fatal("LOCAL_SECRETS_BASE_PATH environment variable is required when SECRET_MANAGER=local")
	}

	logger.Warn("Using LOCAL file-based secret manager - NOT for production use!",
		zap.String("secret_manager", "local"),
		zap.String("base_path", basePath),
	)

	return secrets.NewLocalSecretManager(basePath, logger)
}

// initMockSecretManager initializes the mock secret manager for development
func initMockSecretManager(logger *zap.Logger) ports.SecretManagerAdapter {
	logger.Warn("Using MOCK secret manager - NOT for production use!",
		zap.String("secret_manager", "mock"),
	)
	return mock.NewMockSecretManager(logger)
}
