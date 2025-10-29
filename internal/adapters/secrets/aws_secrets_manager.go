package secrets

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	secretsmanagertypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// AWSSecretsManagerConfig contains configuration for AWS Secrets Manager adapter
type AWSSecretsManagerConfig struct {
	// AWS Region (e.g., "us-east-1")
	Region string

	// Optional: AWS profile name (for local development)
	Profile string

	// Optional: Custom endpoint (for LocalStack testing)
	Endpoint string

	// Cache TTL for secrets (default: 5 minutes)
	CacheTTL time.Duration

	// Enable caching
	EnableCache bool
}

// DefaultAWSSecretsManagerConfig returns default configuration
func DefaultAWSSecretsManagerConfig(region string) *AWSSecretsManagerConfig {
	return &AWSSecretsManagerConfig{
		Region:      region,
		CacheTTL:    5 * time.Minute,
		EnableCache: true,
	}
}

// awsSecretsManagerAdapter implements the SecretManagerAdapter port for AWS Secrets Manager
type awsSecretsManagerAdapter struct {
	client *secretsmanager.Client
	config *AWSSecretsManagerConfig
	logger *zap.Logger
	cache  *secretCache
}

// secretCache implements a simple in-memory cache for secrets
type secretCache struct {
	entries map[string]*cacheEntry
	enabled bool
	ttl     time.Duration
}

type cacheEntry struct {
	secret    *ports.Secret
	expiresAt time.Time
}

// NewAWSSecretsManagerAdapter creates a new AWS Secrets Manager adapter
func NewAWSSecretsManagerAdapter(ctx context.Context, cfg *AWSSecretsManagerConfig, logger *zap.Logger) (ports.SecretManagerAdapter, error) {
	// Load AWS SDK config
	var awsConfig aws.Config
	var err error

	if cfg.Profile != "" {
		// Use specific profile (local development)
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
			config.WithSharedConfigProfile(cfg.Profile),
		)
	} else {
		// Use default credentials chain (IAM role in production)
		awsConfig, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create Secrets Manager client
	clientOptions := []func(*secretsmanager.Options){}
	if cfg.Endpoint != "" {
		// Custom endpoint (for LocalStack)
		clientOptions = append(clientOptions, func(o *secretsmanager.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := secretsmanager.NewFromConfig(awsConfig, clientOptions...)

	logger.Info("AWS Secrets Manager adapter initialized",
		zap.String("region", cfg.Region),
		zap.Bool("cache_enabled", cfg.EnableCache),
		zap.Duration("cache_ttl", cfg.CacheTTL),
	)

	return &awsSecretsManagerAdapter{
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

// GetSecret retrieves a secret by its path
// Path format: "payment-service/agents/{agent_id}/mac" or full ARN
func (a *awsSecretsManagerAdapter) GetSecret(ctx context.Context, path string) (*ports.Secret, error) {
	// Check cache first
	if cached := a.cache.get(path); cached != nil {
		a.logger.Debug("Secret retrieved from cache", zap.String("path", path))
		return cached, nil
	}

	a.logger.Info("Retrieving secret from AWS Secrets Manager", zap.String("path", path))

	// Call AWS Secrets Manager
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(path),
	}

	startTime := time.Now()
	result, err := a.client.GetSecretValue(ctx, input)
	if err != nil {
		a.logger.Error("Failed to retrieve secret",
			zap.String("path", path),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get secret %s: %w", path, err)
	}

	a.logger.Info("Secret retrieved successfully",
		zap.String("path", path),
		zap.Duration("elapsed", time.Since(startTime)),
	)

	// Parse secret
	secret := &ports.Secret{
		Value:   aws.ToString(result.SecretString),
		Version: aws.ToString(result.VersionId),
		CreatedAt: result.CreatedDate.Format(time.RFC3339),
		Metadata: make(map[string]string),
	}

	// Extract metadata from ARN
	if result.ARN != nil {
		secret.Metadata["arn"] = *result.ARN
	}
	if result.Name != nil {
		secret.Metadata["name"] = *result.Name
	}

	// Cache the secret
	a.cache.set(path, secret)

	return secret, nil
}

// GetSecretVersion retrieves a specific version of a secret
func (a *awsSecretsManagerAdapter) GetSecretVersion(ctx context.Context, path string, version string) (*ports.Secret, error) {
	a.logger.Info("Retrieving secret version from AWS Secrets Manager",
		zap.String("path", path),
		zap.String("version", version),
	)

	input := &secretsmanager.GetSecretValueInput{
		SecretId:  aws.String(path),
		VersionId: aws.String(version),
	}

	result, err := a.client.GetSecretValue(ctx, input)
	if err != nil {
		a.logger.Error("Failed to retrieve secret version",
			zap.String("path", path),
			zap.String("version", version),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get secret version %s: %w", version, err)
	}

	secret := &ports.Secret{
		Value:     aws.ToString(result.SecretString),
		Version:   aws.ToString(result.VersionId),
		CreatedAt: result.CreatedDate.Format(time.RFC3339),
		Metadata:  make(map[string]string),
	}

	if result.ARN != nil {
		secret.Metadata["arn"] = *result.ARN
	}

	return secret, nil
}

// PutSecret creates or updates a secret
func (a *awsSecretsManagerAdapter) PutSecret(ctx context.Context, path string, value string, metadata map[string]string) (string, error) {
	a.logger.Info("Putting secret to AWS Secrets Manager", zap.String("path", path))

	// Try to update existing secret first
	updateInput := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(path),
		SecretString: aws.String(value),
	}

	result, err := a.client.PutSecretValue(ctx, updateInput)
	if err != nil {
		// If secret doesn't exist, create it
		createInput := &secretsmanager.CreateSecretInput{
			Name:         aws.String(path),
			SecretString: aws.String(value),
			Description:  aws.String("Payment service agent MAC secret"),
		}

		// Add tags from metadata
		if len(metadata) > 0 {
			tags := make([]secretsmanagertypes.Tag, 0, len(metadata))
			for key, val := range metadata {
				tags = append(tags, secretsmanagertypes.Tag{
					Key:   aws.String(key),
					Value: aws.String(val),
				})
			}
			createInput.Tags = tags
		}

		createResult, createErr := a.client.CreateSecret(ctx, createInput)
		if createErr != nil {
			a.logger.Error("Failed to create secret",
				zap.String("path", path),
				zap.Error(createErr),
			)
			return "", fmt.Errorf("failed to create secret: %w", createErr)
		}

		a.logger.Info("Secret created successfully",
			zap.String("path", path),
			zap.String("version", aws.ToString(createResult.VersionId)),
		)

		// Invalidate cache
		a.cache.invalidate(path)

		return aws.ToString(createResult.VersionId), nil
	}

	a.logger.Info("Secret updated successfully",
		zap.String("path", path),
		zap.String("version", aws.ToString(result.VersionId)),
	)

	// Invalidate cache
	a.cache.invalidate(path)

	return aws.ToString(result.VersionId), nil
}

// RotateSecret rotates a secret by creating a new version
func (a *awsSecretsManagerAdapter) RotateSecret(ctx context.Context, path string, newValue string) (*ports.SecretRotationInfo, error) {
	a.logger.Info("Rotating secret", zap.String("path", path))

	// Get current version before rotation
	currentSecret, err := a.GetSecret(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get current secret: %w", err)
	}

	currentVersion := currentSecret.Version

	// Put new secret value (creates new version)
	newVersion, err := a.PutSecret(ctx, path, newValue, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to put new secret version: %w", err)
	}

	a.logger.Info("Secret rotated successfully",
		zap.String("path", path),
		zap.String("previous_version", currentVersion),
		zap.String("new_version", newVersion),
	)

	return &ports.SecretRotationInfo{
		CurrentVersion:  newVersion,
		PreviousVersion: currentVersion,
		NextRotation:    "", // AWS Secrets Manager handles rotation scheduling separately
	}, nil
}

// DeleteSecret permanently deletes a secret
func (a *awsSecretsManagerAdapter) DeleteSecret(ctx context.Context, path string) error {
	a.logger.Warn("Deleting secret (IRREVERSIBLE)",
		zap.String("path", path),
	)

	input := &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(path),
		ForceDeleteWithoutRecovery: aws.Bool(false), // Default: 30-day recovery window
		RecoveryWindowInDays:       aws.Int64(30),
	}

	_, err := a.client.DeleteSecret(ctx, input)
	if err != nil {
		a.logger.Error("Failed to delete secret",
			zap.String("path", path),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	a.logger.Info("Secret scheduled for deletion",
		zap.String("path", path),
		zap.Int("recovery_window_days", 30),
	)

	// Invalidate cache
	a.cache.invalidate(path)

	return nil
}

// secretCache methods

func (c *secretCache) get(key string) *ports.Secret {
	if !c.enabled {
		return nil
	}

	entry, ok := c.entries[key]
	if !ok {
		return nil
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil
	}

	return entry.secret
}

func (c *secretCache) set(key string, secret *ports.Secret) {
	if !c.enabled {
		return
	}

	c.entries[key] = &cacheEntry{
		secret:    secret,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *secretCache) invalidate(key string) {
	delete(c.entries, key)
}

func (c *secretCache) clear() {
	c.entries = make(map[string]*cacheEntry)
}
