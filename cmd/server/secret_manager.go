package main

import (
	"context"
	"os"

	"github.com/kevin07696/payment-service/internal/adapters/gcp"
	"github.com/kevin07696/payment-service/internal/adapters/mock"
	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// initSecretManager initializes the appropriate secret manager based on environment
// Supports:
//   - GCP Secret Manager (production): Set SECRET_MANAGER=gcp and GCP_PROJECT_ID
//   - Mock (development/testing): Default when SECRET_MANAGER is not set
//
// Environment Variables:
//   - SECRET_MANAGER: "gcp" or "mock" (default: mock)
//   - GCP_PROJECT_ID: Your GCP project ID (required when SECRET_MANAGER=gcp)
//   - GOOGLE_APPLICATION_CREDENTIALS: Path to GCP service account JSON (required for GCP)
//   - SECRET_CACHE_TTL_MINUTES: Cache TTL in minutes (default: 5)
func initSecretManager(ctx context.Context, cfg *Config, logger *zap.Logger) ports.SecretManagerAdapter {
	secretManagerType := getEnv("SECRET_MANAGER", "mock")

	switch secretManagerType {
	case "gcp":
		return initGCPSecretManager(ctx, logger)
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

// initMockSecretManager initializes the mock secret manager for development
func initMockSecretManager(logger *zap.Logger) ports.SecretManagerAdapter {
	logger.Warn("Using MOCK secret manager - NOT for production use!",
		zap.String("secret_manager", "mock"),
	)
	return mock.NewMockSecretManager(logger)
}
