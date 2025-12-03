package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetDefaultDBURL tests the database URL construction logic
func TestGetDefaultDBURL(t *testing.T) {
	// Save current env vars and restore after test
	originalDatabaseURL := os.Getenv("DATABASE_URL")
	originalDBHost := os.Getenv("DB_HOST")
	originalDBPort := os.Getenv("DB_PORT")
	originalDBUser := os.Getenv("DB_USER")
	originalDBPassword := os.Getenv("DB_PASSWORD")
	originalDBName := os.Getenv("DB_NAME")
	originalDBSSLMode := os.Getenv("DB_SSL_MODE")

	defer func() {
		os.Setenv("DATABASE_URL", originalDatabaseURL)
		os.Setenv("DB_HOST", originalDBHost)
		os.Setenv("DB_PORT", originalDBPort)
		os.Setenv("DB_USER", originalDBUser)
		os.Setenv("DB_PASSWORD", originalDBPassword)
		os.Setenv("DB_NAME", originalDBName)
		os.Setenv("DB_SSL_MODE", originalDBSSLMode)
	}()

	t.Run("returns DATABASE_URL when set", func(t *testing.T) {
		// Clear all vars first
		clearDBEnvVars()

		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb")
		result := getDefaultDBURL()
		assert.Equal(t, "postgres://user:pass@localhost:5432/testdb", result)
	})

	t.Run("constructs URL from individual DB_* vars when DATABASE_URL not set", func(t *testing.T) {
		clearDBEnvVars()

		os.Setenv("DB_HOST", "localhost")
		os.Setenv("DB_PORT", "5432")
		os.Setenv("DB_USER", "testuser")
		os.Setenv("DB_PASSWORD", "testpass")
		os.Setenv("DB_NAME", "testdb")
		os.Setenv("DB_SSL_MODE", "disable")

		result := getDefaultDBURL()
		assert.Equal(t, "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable", result)
	})

	t.Run("uses defaults for port, user, and sslmode", func(t *testing.T) {
		clearDBEnvVars()

		os.Setenv("DB_HOST", "localhost")
		os.Setenv("DB_NAME", "testdb")

		result := getDefaultDBURL()
		assert.Equal(t, "postgres://postgres@localhost:5432/testdb?sslmode=disable", result)
	})

	t.Run("constructs URL without password if not provided", func(t *testing.T) {
		clearDBEnvVars()

		os.Setenv("DB_HOST", "localhost")
		os.Setenv("DB_PORT", "5433")
		os.Setenv("DB_USER", "myuser")
		os.Setenv("DB_NAME", "mydb")

		result := getDefaultDBURL()
		assert.Equal(t, "postgres://myuser@localhost:5433/mydb?sslmode=disable", result)
	})

	t.Run("returns empty string when minimum vars not set", func(t *testing.T) {
		clearDBEnvVars()

		// Only host, no name
		os.Setenv("DB_HOST", "localhost")
		result := getDefaultDBURL()
		assert.Equal(t, "", result)

		// Only name, no host
		clearDBEnvVars()
		os.Setenv("DB_NAME", "testdb")
		result = getDefaultDBURL()
		assert.Equal(t, "", result)
	})

	t.Run("DATABASE_URL takes precedence over DB_* vars", func(t *testing.T) {
		clearDBEnvVars()

		os.Setenv("DATABASE_URL", "postgres://priority@host:5432/db")
		os.Setenv("DB_HOST", "other-host")
		os.Setenv("DB_NAME", "other-db")

		result := getDefaultDBURL()
		assert.Equal(t, "postgres://priority@host:5432/db", result)
	})
}

// TestGenerateFingerprint tests the fingerprint generation for public keys
func TestGenerateFingerprint(t *testing.T) {
	t.Run("generates consistent fingerprint", func(t *testing.T) {
		publicKeyPEM := []byte(`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAs60D2vOdaxN5FxCp1ruP
Pf02WYloMve5Ky4fxyHpbrOGI8Yh1ZrO06+TzTOIUQ6Gvglb5xYWbcljuEVqI65t
b4RKw9lA5bmOuxfVvgCcpzTY2DTkOSSvJW3sS4rG+HPHnHZiEhvHh+PG7/iYhpn0
FxMJ9EQH4nE6JHWf+4TrwaIHKs/oyp4g0db0lm0TDZHblTyvC6lNQ1LrrV9uywOI
EdcuibCr/JhUcTEUB65+S2oyqhbtryp06KYrhXuApPW9X3I7UgKa/Re39cbVJpNI
rbJzVKXxyDLU0NWV54ELfOUy6eG6KrAPvPVpRYfOtXZEHvZ+e0qBw0U0/fs02loA
jwIDAQAB
-----END PUBLIC KEY-----`)

		fp1 := generateFingerprint(publicKeyPEM)
		fp2 := generateFingerprint(publicKeyPEM)

		assert.Equal(t, fp1, fp2, "fingerprint should be deterministic")
		assert.True(t, len(fp1) == 50, "fingerprint should be 50 characters")
		assert.Contains(t, fp1, "SHA256:", "fingerprint should start with SHA256:")
	})

	t.Run("different keys produce different fingerprints", func(t *testing.T) {
		key1 := []byte("test-key-1")
		key2 := []byte("test-key-2")

		fp1 := generateFingerprint(key1)
		fp2 := generateFingerprint(key2)

		assert.NotEqual(t, fp1, fp2, "different keys should have different fingerprints")
	})
}

// TestPayCLIStructure verifies PayCLI struct can be instantiated
func TestPayCLIStructure(t *testing.T) {
	t.Run("PayCLI can be created with nil queries", func(t *testing.T) {
		cli := &PayCLI{
			ctx:     nil,
			queries: nil,
		}
		assert.NotNil(t, cli)
	})
}

// TestServiceDataStructure tests the service data parsing
func TestServiceDataStructure(t *testing.T) {
	t.Run("service data JSON structure", func(t *testing.T) {
		jsonStr := `{
			"service_id": "test-service",
			"service_name": "Test Service",
			"environment": "staging",
			"requests_per_second": 1000,
			"burst_limit": 2000,
			"generate_keypair": true
		}`

		var serviceData struct {
			ServiceID         string `json:"service_id"`
			ServiceName       string `json:"service_name"`
			Environment       string `json:"environment"`
			RequestsPerSecond int    `json:"requests_per_second"`
			BurstLimit        int    `json:"burst_limit"`
			GenerateKeypair   bool   `json:"generate_keypair"`
			PublicKey         string `json:"public_key,omitempty"`
		}

		err := parseJSON([]byte(jsonStr), &serviceData)
		require.NoError(t, err)

		assert.Equal(t, "test-service", serviceData.ServiceID)
		assert.Equal(t, "Test Service", serviceData.ServiceName)
		assert.Equal(t, "staging", serviceData.Environment)
		assert.Equal(t, 1000, serviceData.RequestsPerSecond)
		assert.Equal(t, 2000, serviceData.BurstLimit)
		assert.True(t, serviceData.GenerateKeypair)
	})

	t.Run("service data with existing public key", func(t *testing.T) {
		jsonStr := `{
			"service_id": "existing-key-service",
			"service_name": "Existing Key Service",
			"environment": "production",
			"generate_keypair": false,
			"public_key": "-----BEGIN PUBLIC KEY-----\nMIIBIjAN...\n-----END PUBLIC KEY-----"
		}`

		var serviceData struct {
			ServiceID         string `json:"service_id"`
			ServiceName       string `json:"service_name"`
			Environment       string `json:"environment"`
			RequestsPerSecond int    `json:"requests_per_second"`
			BurstLimit        int    `json:"burst_limit"`
			GenerateKeypair   bool   `json:"generate_keypair"`
			PublicKey         string `json:"public_key,omitempty"`
		}

		err := parseJSON([]byte(jsonStr), &serviceData)
		require.NoError(t, err)

		assert.Equal(t, "existing-key-service", serviceData.ServiceID)
		assert.False(t, serviceData.GenerateKeypair)
		assert.Contains(t, serviceData.PublicKey, "BEGIN PUBLIC KEY")
	})
}

// TestMerchantDataStructure tests the merchant data parsing
func TestMerchantDataStructure(t *testing.T) {
	t.Run("merchant data JSON structure", func(t *testing.T) {
		jsonStr := `{
			"slug": "test-merchant",
			"name": "Test Merchant",
			"cust_nbr": "CUST001",
			"merch_nbr": "MERCH001",
			"dba_nbr": "DBA001",
			"terminal_nbr": "TERM001",
			"mac_secret_path": "/secrets/test-merchant",
			"environment": "staging",
			"tier": "premium",
			"requests_per_second": 500
		}`

		var merchantData struct {
			Slug              string `json:"slug"`
			Name              string `json:"name"`
			CustNbr           string `json:"cust_nbr"`
			MerchNbr          string `json:"merch_nbr"`
			DbaNbr            string `json:"dba_nbr"`
			TerminalNbr       string `json:"terminal_nbr"`
			MacSecretPath     string `json:"mac_secret_path"`
			Environment       string `json:"environment"`
			Tier              string `json:"tier"`
			RequestsPerSecond int    `json:"requests_per_second"`
		}

		err := parseJSON([]byte(jsonStr), &merchantData)
		require.NoError(t, err)

		assert.Equal(t, "test-merchant", merchantData.Slug)
		assert.Equal(t, "Test Merchant", merchantData.Name)
		assert.Equal(t, "CUST001", merchantData.CustNbr)
		assert.Equal(t, "MERCH001", merchantData.MerchNbr)
		assert.Equal(t, "DBA001", merchantData.DbaNbr)
		assert.Equal(t, "TERM001", merchantData.TerminalNbr)
		assert.Equal(t, "/secrets/test-merchant", merchantData.MacSecretPath)
		assert.Equal(t, "staging", merchantData.Environment)
		assert.Equal(t, "premium", merchantData.Tier)
		assert.Equal(t, 500, merchantData.RequestsPerSecond)
	})
}

// Helper function to clear all DB-related environment variables
func clearDBEnvVars() {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DB_SSL_MODE")
}

// Helper function to parse JSON (used by tests)
func parseJSON(data []byte, v interface{}) error {
	return jsonUnmarshal(data, v)
}

// jsonUnmarshal wraps json.Unmarshal for testing
var jsonUnmarshal = func(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
