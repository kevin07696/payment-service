package secrets

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewFactoryConfigFromEnv(t *testing.T) {
	// Save original env vars
	originalEnv := map[string]string{
		"SECRET_MANAGER":           os.Getenv("SECRET_MANAGER"),
		"SECRET_CACHE_TTL_MINUTES": os.Getenv("SECRET_CACHE_TTL_MINUTES"),
		"GCP_PROJECT_ID":           os.Getenv("GCP_PROJECT_ID"),
		"AWS_REGION":               os.Getenv("AWS_REGION"),
		"VAULT_ADDR":               os.Getenv("VAULT_ADDR"),
		"LOCAL_SECRETS_BASE_PATH":  os.Getenv("LOCAL_SECRETS_BASE_PATH"),
	}

	// Restore env vars after test
	defer func() {
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	t.Run("defaults_to_mock_when_no_env_set", func(t *testing.T) {
		// Clear all relevant env vars
		os.Unsetenv("SECRET_MANAGER")
		os.Unsetenv("SECRET_CACHE_TTL_MINUTES")

		config := NewFactoryConfigFromEnv()

		assert.Equal(t, SecretManagerMock, config.Type)
		assert.Equal(t, 5, config.CacheTTLMinutes) // Default
	})

	t.Run("reads_gcp_config_from_env", func(t *testing.T) {
		os.Setenv("SECRET_MANAGER", "gcp")
		os.Setenv("GCP_PROJECT_ID", "my-test-project")
		os.Setenv("SECRET_CACHE_TTL_MINUTES", "10")

		config := NewFactoryConfigFromEnv()

		assert.Equal(t, SecretManagerGCP, config.Type)
		assert.Equal(t, "my-test-project", config.GCPProjectID)
		assert.Equal(t, 10, config.CacheTTLMinutes)
	})

	t.Run("reads_aws_config_from_env", func(t *testing.T) {
		os.Setenv("SECRET_MANAGER", "aws")
		os.Setenv("AWS_REGION", "us-east-1")
		os.Setenv("AWS_PROFILE", "dev-profile")
		os.Setenv("AWS_SECRETS_ENDPOINT", "http://localhost:4566")

		config := NewFactoryConfigFromEnv()

		assert.Equal(t, SecretManagerAWS, config.Type)
		assert.Equal(t, "us-east-1", config.AWSRegion)
		assert.Equal(t, "dev-profile", config.AWSProfile)
		assert.Equal(t, "http://localhost:4566", config.AWSEndpoint)
	})

	t.Run("reads_vault_config_from_env", func(t *testing.T) {
		os.Setenv("SECRET_MANAGER", "vault")
		os.Setenv("VAULT_ADDR", "https://vault.example.com:8200")
		os.Setenv("VAULT_AUTH_METHOD", "approle")
		os.Setenv("VAULT_ROLE_ID", "test-role-id")
		os.Setenv("VAULT_SECRET_ID", "test-secret-id")
		os.Setenv("VAULT_MOUNT_PATH", "kv")
		os.Setenv("VAULT_KV_VERSION", "v1")

		config := NewFactoryConfigFromEnv()

		assert.Equal(t, SecretManagerVault, config.Type)
		assert.Equal(t, "https://vault.example.com:8200", config.VaultAddr)
		assert.Equal(t, "approle", config.VaultAuthMethod)
		assert.Equal(t, "test-role-id", config.VaultRoleID)
		assert.Equal(t, "test-secret-id", config.VaultSecretID)
		assert.Equal(t, "kv", config.VaultMountPath)
		assert.Equal(t, "v1", config.VaultKVVersion)
	})

	t.Run("reads_local_config_from_env", func(t *testing.T) {
		os.Setenv("SECRET_MANAGER", "local")
		os.Setenv("LOCAL_SECRETS_BASE_PATH", "/var/secrets")

		config := NewFactoryConfigFromEnv()

		assert.Equal(t, SecretManagerLocal, config.Type)
		assert.Equal(t, "/var/secrets", config.LocalBasePath)
	})
}

func TestNewSecretManager_Mock(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	config := &FactoryConfig{
		Type: SecretManagerMock,
	}

	sm, err := NewSecretManager(ctx, config, logger)

	require.NoError(t, err)
	assert.NotNil(t, sm)
}

func TestNewSecretManager_UnknownType_FallsBackToMock(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	config := &FactoryConfig{
		Type: "unknown-type",
	}

	sm, err := NewSecretManager(ctx, config, logger)

	require.NoError(t, err)
	assert.NotNil(t, sm)
}

func TestNewSecretManager_NilConfig_UsesEnvDefaults(t *testing.T) {
	// Save and clear env
	originalEnv := os.Getenv("SECRET_MANAGER")
	os.Unsetenv("SECRET_MANAGER")
	defer func() {
		if originalEnv != "" {
			os.Setenv("SECRET_MANAGER", originalEnv)
		}
	}()

	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	// Passing nil config should read from env (defaults to mock)
	sm, err := NewSecretManager(ctx, nil, logger)

	require.NoError(t, err)
	assert.NotNil(t, sm)
}

func TestNewSecretManager_GCP_RequiresProjectID(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	config := &FactoryConfig{
		Type:         SecretManagerGCP,
		GCPProjectID: "", // Missing required field
	}

	_, err := NewSecretManager(ctx, config, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "GCP_PROJECT_ID")
}

func TestNewSecretManager_AWS_RequiresRegion(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	config := &FactoryConfig{
		Type:      SecretManagerAWS,
		AWSRegion: "", // Missing required field
	}

	_, err := NewSecretManager(ctx, config, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AWS_REGION")
}

func TestNewSecretManager_Vault_RequiresAddr(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	config := &FactoryConfig{
		Type:      SecretManagerVault,
		VaultAddr: "", // Missing required field
	}

	_, err := NewSecretManager(ctx, config, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "VAULT_ADDR")
}

func TestNewSecretManager_Vault_TokenAuth_RequiresToken(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	config := &FactoryConfig{
		Type:            SecretManagerVault,
		VaultAddr:       "https://vault.example.com:8200",
		VaultAuthMethod: "token",
		VaultToken:      "", // Missing required field
	}

	_, err := NewSecretManager(ctx, config, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "VAULT_TOKEN")
}

func TestNewSecretManager_Vault_AppRoleAuth_RequiresCredentials(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	t.Run("missing_role_id", func(t *testing.T) {
		config := &FactoryConfig{
			Type:            SecretManagerVault,
			VaultAddr:       "https://vault.example.com:8200",
			VaultAuthMethod: "approle",
			VaultRoleID:     "",
			VaultSecretID:   "secret-id",
		}

		_, err := NewSecretManager(ctx, config, logger)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "VAULT_ROLE_ID")
	})

	t.Run("missing_secret_id", func(t *testing.T) {
		config := &FactoryConfig{
			Type:            SecretManagerVault,
			VaultAddr:       "https://vault.example.com:8200",
			VaultAuthMethod: "approle",
			VaultRoleID:     "role-id",
			VaultSecretID:   "",
		}

		_, err := NewSecretManager(ctx, config, logger)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "VAULT_SECRET_ID")
	})
}

func TestNewSecretManager_Vault_KubernetesAuth_RequiresRole(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	config := &FactoryConfig{
		Type:            SecretManagerVault,
		VaultAddr:       "https://vault.example.com:8200",
		VaultAuthMethod: "kubernetes",
		VaultK8sRole:    "", // Missing required field
	}

	_, err := NewSecretManager(ctx, config, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "VAULT_K8S_ROLE")
}

func TestNewSecretManager_Local_RequiresBasePath(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	config := &FactoryConfig{
		Type:          SecretManagerLocal,
		LocalBasePath: "", // Missing required field
	}

	_, err := NewSecretManager(ctx, config, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "LOCAL_SECRETS_BASE_PATH")
}

func TestNewSecretManager_Local_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	config := &FactoryConfig{
		Type:          SecretManagerLocal,
		LocalBasePath: "/tmp/test-secrets",
	}

	sm, err := NewSecretManager(ctx, config, logger)

	require.NoError(t, err)
	assert.NotNil(t, sm)
}

func TestSecretManagerType_Constants(t *testing.T) {
	// Verify constant values match expected strings
	assert.Equal(t, SecretManagerType("mock"), SecretManagerMock)
	assert.Equal(t, SecretManagerType("local"), SecretManagerLocal)
	assert.Equal(t, SecretManagerType("gcp"), SecretManagerGCP)
	assert.Equal(t, SecretManagerType("aws"), SecretManagerAWS)
	assert.Equal(t, SecretManagerType("vault"), SecretManagerVault)
}

func TestGetEnv(t *testing.T) {
	t.Run("returns_env_value_when_set", func(t *testing.T) {
		os.Setenv("TEST_VAR", "test-value")
		defer os.Unsetenv("TEST_VAR")

		result := getEnv("TEST_VAR", "default")
		assert.Equal(t, "test-value", result)
	})

	t.Run("returns_default_when_not_set", func(t *testing.T) {
		os.Unsetenv("TEST_VAR_NOT_SET")

		result := getEnv("TEST_VAR_NOT_SET", "default-value")
		assert.Equal(t, "default-value", result)
	})
}

func TestGetEnvInt(t *testing.T) {
	t.Run("returns_int_value_when_set", func(t *testing.T) {
		os.Setenv("TEST_INT_VAR", "42")
		defer os.Unsetenv("TEST_INT_VAR")

		result := getEnvInt("TEST_INT_VAR", 10)
		assert.Equal(t, 42, result)
	})

	t.Run("returns_default_when_not_set", func(t *testing.T) {
		os.Unsetenv("TEST_INT_VAR_NOT_SET")

		result := getEnvInt("TEST_INT_VAR_NOT_SET", 10)
		assert.Equal(t, 10, result)
	})

	t.Run("returns_default_when_invalid", func(t *testing.T) {
		os.Setenv("TEST_INT_VAR_INVALID", "not-a-number")
		defer os.Unsetenv("TEST_INT_VAR_INVALID")

		result := getEnvInt("TEST_INT_VAR_INVALID", 10)
		assert.Equal(t, 10, result)
	})
}

// TestMockSecretManager_IsLabeledAsMock verifies the mock adapter warns it's not for production
func TestMockSecretManager_IsLabeledAsMock(t *testing.T) {
	// Create a logger that captures output
	core, logs := observerCore()
	logger := zap.New(core)
	ctx := context.Background()

	config := &FactoryConfig{
		Type: SecretManagerMock,
	}

	_, err := NewSecretManager(ctx, config, logger)
	require.NoError(t, err)

	// Verify warning was logged
	found := false
	for _, entry := range logs.All() {
		if entry.Level == zap.WarnLevel &&
			entry.Message == "Using MOCK secret manager - NOT for production use!" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected warning about mock secret manager not being for production")
}

// TestLocalSecretManager_IsLabeledAsNotForProduction verifies the local adapter warns it's not for production
func TestLocalSecretManager_IsLabeledAsNotForProduction(t *testing.T) {
	core, logs := observerCore()
	logger := zap.New(core)
	ctx := context.Background()

	config := &FactoryConfig{
		Type:          SecretManagerLocal,
		LocalBasePath: "/tmp/test-secrets",
	}

	_, err := NewSecretManager(ctx, config, logger)
	require.NoError(t, err)

	// Verify warning was logged
	found := false
	for _, entry := range logs.All() {
		if entry.Level == zap.WarnLevel &&
			entry.Message == "Using LOCAL file-based secret manager - NOT for production use!" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected warning about local secret manager not being for production")
}

// observerCore creates a zap core that captures log entries for testing
func observerCore() (zapcore.Core, *observer.ObservedLogs) {
	return observer.New(zapcore.DebugLevel)
}
