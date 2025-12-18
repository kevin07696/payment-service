package seeder

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_ProductionBlocked(t *testing.T) {
	// Save and restore original environment
	originalEnv := os.Getenv("ENVIRONMENT")
	defer func() {
		if originalEnv != "" {
			os.Setenv("ENVIRONMENT", originalEnv)
		} else {
			os.Unsetenv("ENVIRONMENT")
		}
	}()

	// Set production environment
	os.Setenv("ENVIRONMENT", "production")

	cfg := LoadConfig()

	assert.Nil(t, cfg, "LoadConfig must return nil in production to prevent auto-seeding")
}

func TestLoadConfig_StagingAllowed(t *testing.T) {
	// Save and restore original environment
	originalEnv := os.Getenv("ENVIRONMENT")
	defer func() {
		if originalEnv != "" {
			os.Setenv("ENVIRONMENT", originalEnv)
		} else {
			os.Unsetenv("ENVIRONMENT")
		}
	}()

	os.Setenv("ENVIRONMENT", "staging")
	os.Setenv("SANDBOX_MERCHANT_SLUG", "staging-merchant")
	defer os.Unsetenv("SANDBOX_MERCHANT_SLUG")

	cfg := LoadConfig()

	require.NotNil(t, cfg, "LoadConfig should allow staging environment")
	assert.Equal(t, "staging", cfg.Environment)
}

func TestLoadConfig_DevelopmentAllowed(t *testing.T) {
	// Save and restore original environment
	originalEnv := os.Getenv("ENVIRONMENT")
	defer func() {
		if originalEnv != "" {
			os.Setenv("ENVIRONMENT", originalEnv)
		} else {
			os.Unsetenv("ENVIRONMENT")
		}
	}()

	os.Setenv("ENVIRONMENT", "development")
	os.Setenv("SANDBOX_MERCHANT_SLUG", "dev-merchant")
	defer os.Unsetenv("SANDBOX_MERCHANT_SLUG")

	cfg := LoadConfig()

	require.NotNil(t, cfg, "LoadConfig should allow development environment")
	assert.Equal(t, "development", cfg.Environment)
}

func TestLoadConfig_LoadsEPXCredentials(t *testing.T) {
	// Save and restore original environment
	envVarsToSave := []string{
		"ENVIRONMENT", "SANDBOX_MERCHANT_ID", "SANDBOX_MERCHANT_SLUG",
		"SANDBOX_MERCHANT_NAME", "EPX_CUST_NBR", "EPX_MERCH_NBR",
		"EPX_DBA_NBR", "EPX_TERMINAL_NBR", "EPX_SANDBOX_MAC",
	}
	savedVars := make(map[string]string)
	for _, k := range envVarsToSave {
		savedVars[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range savedVars {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	// Clear all env vars first
	for _, k := range envVarsToSave {
		os.Unsetenv(k)
	}

	// Set test values
	os.Setenv("ENVIRONMENT", "sandbox")
	os.Setenv("SANDBOX_MERCHANT_ID", "11111111-1111-1111-1111-111111111111")
	os.Setenv("SANDBOX_MERCHANT_SLUG", "test-merchant")
	os.Setenv("SANDBOX_MERCHANT_NAME", "Test Merchant Inc")
	os.Setenv("EPX_CUST_NBR", "9001")
	os.Setenv("EPX_MERCH_NBR", "900300")
	os.Setenv("EPX_DBA_NBR", "2")
	os.Setenv("EPX_TERMINAL_NBR", "77")
	os.Setenv("EPX_SANDBOX_MAC", "test-mac-secret")

	cfg := LoadConfig()

	require.NotNil(t, cfg)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", cfg.MerchantID)
	assert.Equal(t, "test-merchant", cfg.MerchantSlug)
	assert.Equal(t, "Test Merchant Inc", cfg.MerchantName)
	assert.Equal(t, "9001", cfg.CustNbr)
	assert.Equal(t, "900300", cfg.MerchNbr)
	assert.Equal(t, "2", cfg.DbaNbr)
	assert.Equal(t, "77", cfg.TerminalNbr)
	assert.Equal(t, "test-mac-secret", cfg.MAC)
}

func TestLoadConfig_MacSecretPathFormat(t *testing.T) {
	// Save and restore original environment
	originalEnv := os.Getenv("ENVIRONMENT")
	originalSlug := os.Getenv("SANDBOX_MERCHANT_SLUG")
	defer func() {
		if originalEnv != "" {
			os.Setenv("ENVIRONMENT", originalEnv)
		} else {
			os.Unsetenv("ENVIRONMENT")
		}
		if originalSlug != "" {
			os.Setenv("SANDBOX_MERCHANT_SLUG", originalSlug)
		} else {
			os.Unsetenv("SANDBOX_MERCHANT_SLUG")
		}
	}()

	os.Unsetenv("ENVIRONMENT") // Not production

	tests := []struct {
		name         string
		merchantSlug string
		expectedPath string
	}{
		{
			name:         "standard slug generates correct path",
			merchantSlug: "acme-corp",
			expectedPath: "payments/merchants/acme-corp/mac",
		},
		{
			name:         "slug with underscores",
			merchantSlug: "my_test_merchant",
			expectedPath: "payments/merchants/my_test_merchant/mac",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SANDBOX_MERCHANT_SLUG", tt.merchantSlug)

			cfg := LoadConfig()

			require.NotNil(t, cfg)
			assert.Equal(t, tt.expectedPath, cfg.MacSecretPath,
				"MAC secret path should follow 'payments/merchants/{slug}/mac' format")
		})
	}
}

func TestGenerateFingerprint_SHA256Format(t *testing.T) {
	input := []byte("-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\n-----END PUBLIC KEY-----")

	result := generateFingerprint(input)

	// Verify SHA256 prefix
	assert.True(t, len(result) > 7, "Fingerprint must be longer than prefix")
	assert.Equal(t, "SHA256:", result[:7], "Fingerprint must start with 'SHA256:'")

	// Verify fixed length (truncated to 50 chars for storage)
	assert.Len(t, result, 50, "Fingerprint must be exactly 50 characters")
}

func TestGenerateFingerprint_Deterministic(t *testing.T) {
	// Same input should always produce same fingerprint
	input := []byte("test-public-key-content-for-reproducibility")

	result1 := generateFingerprint(input)
	result2 := generateFingerprint(input)
	result3 := generateFingerprint(input)

	assert.Equal(t, result1, result2, "Fingerprint must be deterministic")
	assert.Equal(t, result2, result3, "Fingerprint must be deterministic")
}

func TestGenerateFingerprint_UniquePerKey(t *testing.T) {
	// Different keys should produce different fingerprints
	key1 := []byte("-----BEGIN PUBLIC KEY-----\nKEY1\n-----END PUBLIC KEY-----")
	key2 := []byte("-----BEGIN PUBLIC KEY-----\nKEY2\n-----END PUBLIC KEY-----")

	fp1 := generateFingerprint(key1)
	fp2 := generateFingerprint(key2)

	assert.NotEqual(t, fp1, fp2, "Different keys must produce different fingerprints")
}

func TestGenerateFingerprint_HandlesEmptyInput(t *testing.T) {
	// Even empty input should produce a valid fingerprint
	result := generateFingerprint([]byte{})

	assert.Len(t, result, 50, "Empty input should still produce 50-char fingerprint")
	assert.Equal(t, "SHA256:", result[:7], "Empty input should still have SHA256 prefix")
}
