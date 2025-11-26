package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadCredentials tests the credential loading functionality
func TestLoadCredentials(t *testing.T) {
	t.Run("valid credentials file", func(t *testing.T) {
		// Create temp credentials file
		creds := ServiceCredentials{
			ServiceID:   "test-service",
			ServiceName: "Test Service",
			Environment: "staging",
			PrivateKey:  testPrivateKey,
			PublicKey:   testPublicKey,
		}

		tmpFile := createTempCredentialsFile(t, creds)
		defer os.Remove(tmpFile)

		loaded, err := loadCredentials(tmpFile)
		require.NoError(t, err)
		assert.Equal(t, "test-service", loaded.ServiceID)
		assert.Equal(t, "Test Service", loaded.ServiceName)
		assert.Equal(t, "staging", loaded.Environment)
		assert.NotEmpty(t, loaded.PrivateKey)
	})

	t.Run("missing service_id", func(t *testing.T) {
		creds := ServiceCredentials{
			ServiceID:   "", // Missing
			ServiceName: "Test Service",
			PrivateKey:  testPrivateKey,
		}

		tmpFile := createTempCredentialsFile(t, creds)
		defer os.Remove(tmpFile)

		_, err := loadCredentials(tmpFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing service_id")
	})

	t.Run("missing private_key", func(t *testing.T) {
		creds := ServiceCredentials{
			ServiceID:   "test-service",
			ServiceName: "Test Service",
			PrivateKey:  "", // Missing
		}

		tmpFile := createTempCredentialsFile(t, creds)
		defer os.Remove(tmpFile)

		_, err := loadCredentials(tmpFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing private_key")
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := loadCredentials("/nonexistent/path/creds.json")
		require.Error(t, err)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "invalid.json")
		err := os.WriteFile(tmpFile, []byte("not valid json"), 0600)
		require.NoError(t, err)

		_, err = loadCredentials(tmpFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse JSON")
	})
}

// TestParsePrivateKey tests the private key parsing functionality
func TestParsePrivateKey(t *testing.T) {
	t.Run("valid PKCS1 key", func(t *testing.T) {
		key, err := parsePrivateKey(testPrivateKey)
		require.NoError(t, err)
		assert.NotNil(t, key)
	})

	t.Run("invalid PEM", func(t *testing.T) {
		_, err := parsePrivateKey("not a pem key")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode PEM")
	})
}

// TestGenerateToken tests JWT token generation
// NOTE: Token only contains service info, NOT merchant_id
// Merchant is specified per-request in the request body
func TestGenerateToken(t *testing.T) {
	creds := &ServiceCredentials{
		ServiceID:   "test-service",
		ServiceName: "Test Service",
		PrivateKey:  testPrivateKey,
	}

	t.Run("generates valid token without merchant_id", func(t *testing.T) {
		scopes := []string{"payments:create", "payments:read"}
		expiry := time.Hour

		tokenStr, err := generateToken(creds, scopes, expiry)
		require.NoError(t, err)
		assert.NotEmpty(t, tokenStr)

		// Verify token structure (3 parts: header.payload.signature)
		parts := strings.Split(tokenStr, ".")
		assert.Len(t, parts, 3, "JWT should have 3 parts")

		// Parse and verify claims
		parser := jwt.NewParser()
		token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
		require.NoError(t, err)

		claims, ok := token.Claims.(jwt.MapClaims)
		require.True(t, ok)

		// Verify all expected claims
		assert.Equal(t, "test-service", claims["iss"])
		assert.Equal(t, "test-service", claims["sub"])
		assert.Nil(t, claims["merchant_id"], "merchant_id should NOT be in token")
		assert.NotNil(t, claims["scopes"])
		assert.NotNil(t, claims["exp"])
		assert.NotNil(t, claims["iat"])
		assert.NotNil(t, claims["nbf"])
		assert.NotNil(t, claims["jti"])

		// Verify scopes array
		scopesClaim, ok := claims["scopes"].([]interface{})
		require.True(t, ok)
		assert.Len(t, scopesClaim, 2)
	})

	t.Run("expiry is set correctly", func(t *testing.T) {
		scopes := []string{"payments:create"}
		expiry := 30 * time.Minute

		tokenStr, err := generateToken(creds, scopes, expiry)
		require.NoError(t, err)

		parser := jwt.NewParser()
		token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
		require.NoError(t, err)

		claims := token.Claims.(jwt.MapClaims)
		exp := int64(claims["exp"].(float64))
		iat := int64(claims["iat"].(float64))

		// Expiry should be ~30 minutes from issued time
		diff := exp - iat
		assert.InDelta(t, 1800, diff, 5, "Expiry should be ~30 minutes (1800 seconds)")
	})

	t.Run("each token has unique jti", func(t *testing.T) {
		scopes := []string{"payments:create"}
		expiry := time.Hour

		token1, err := generateToken(creds, scopes, expiry)
		require.NoError(t, err)

		token2, err := generateToken(creds, scopes, expiry)
		require.NoError(t, err)

		// Parse both tokens
		parser := jwt.NewParser()
		t1, _, _ := parser.ParseUnverified(token1, jwt.MapClaims{})
		t2, _, _ := parser.ParseUnverified(token2, jwt.MapClaims{})

		jti1 := t1.Claims.(jwt.MapClaims)["jti"].(string)
		jti2 := t2.Claims.(jwt.MapClaims)["jti"].(string)

		assert.NotEqual(t, jti1, jti2, "Each token should have unique jti")
	})
}

// TestResolveString tests the flag resolution helper
func TestResolveString(t *testing.T) {
	t.Run("short takes precedence", func(t *testing.T) {
		result := resolveString("short", "long")
		assert.Equal(t, "short", result)
	})

	t.Run("long used when short empty", func(t *testing.T) {
		result := resolveString("", "long")
		assert.Equal(t, "long", result)
	})

	t.Run("returns empty when both empty", func(t *testing.T) {
		result := resolveString("", "")
		assert.Equal(t, "", result)
	})
}

// TestOutputFormats tests different output format functions
func TestOutputFormats(t *testing.T) {
	t.Run("JSON output structure", func(t *testing.T) {
		// Test that TokenOutput struct marshals correctly
		output := TokenOutput{
			Token:     "test-token",
			ExpiresAt: time.Now().Add(time.Hour),
			ServiceID: "test-service",
			Scopes:    []string{"payments:create", "payments:read"},
		}

		data, err := json.MarshalIndent(output, "", "  ")
		require.NoError(t, err)

		// Verify all fields are present
		var parsed map[string]interface{}
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "test-token", parsed["token"])
		assert.Equal(t, "test-service", parsed["service_id"])
		assert.Nil(t, parsed["merchant_id"], "merchant_id should NOT be in output")
		assert.NotNil(t, parsed["expires_at"])
		assert.NotNil(t, parsed["scopes"])
	})
}

// Helper to create temp credentials file
func createTempCredentialsFile(t *testing.T, creds ServiceCredentials) string {
	t.Helper()
	tmpFile := filepath.Join(t.TempDir(), "test_creds.json")
	data, err := json.Marshal(creds)
	require.NoError(t, err)
	err = os.WriteFile(tmpFile, data, 0600)
	require.NoError(t, err)
	return tmpFile
}

// Test RSA keys for unit testing
// These are ephemeral keys generated for testing only - NOT production keys
// Using PKCS1 format (BEGIN RSA PRIVATE KEY) that parsePrivateKey handles first
const testPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAs60D2vOdaxN5FxCp1ruPPf02WYloMve5Ky4fxyHpbrOGI8Yh
1ZrO06+TzTOIUQ6Gvglb5xYWbcljuEVqI65tb4RKw9lA5bmOuxfVvgCcpzTY2DTk
OSSvJW3sS4rG+HPHnHZiEhvHh+PG7/iYhpn0FxMJ9EQH4nE6JHWf+4TrwaIHKs/o
yp4g0db0lm0TDZHblTyvC6lNQ1LrrV9uywOIEdcuibCr/JhUcTEUB65+S2oyqhbt
ryp06KYrhXuApPW9X3I7UgKa/Re39cbVJpNIrbJzVKXxyDLU0NWV54ELfOUy6eG6
KrAPvPVpRYfOtXZEHvZ+e0qBw0U0/fs02loAjwIDAQABAoIBABgrfglGHDb7N57S
rwYj1PERzu3cfhfdGxuj6MJw3WX24GSPkp2ZZZk0VT2VYREGUzndKG+9mObL4I45
SD3kiPQnZ6dQ4loEzB5+5lHY4zna8hCjjM/jD2yJjO/ci0eAy6lQg4DMG9s72NcP
KfVxYFR6SyyAuk7LzHZ7HDpJdy1k1L1TcGeD9Oyy6+5G30lEriXavZ/1adtLUcxv
GSsvnxk7Q/eBHVsPLpp3eApKa+IMI5KH8yl70Efqh/Ny5AXeQgeDI6BHSEnG5lhW
F2WCBLxt6bERCOBg1qkRRkHaUgYErRg7b+RwC+PWHe0i4bx7H5nmaAF/1ZDS3Feu
SdCorwECgYEA2RrscnYEbAYvJKc9AtWt0Kk/LTj9XAl6kLmwDL25sugpTVeLUrgP
dEeXDkpPRarmtZrGGHehxGeZju3WrSmwlJAsL+eh/Awiq641TZu71ht8FpOBlD+e
bd3+vDmf+eGp/AN1BNILJzcZwcO2JNd0MbSlJMLyicH/DQLdZT40c+cCgYEA0915
IesB4df43k1sEZuqriiYlfR/++Q05aUj1l7N8qQJBkjDo7dk0fGsEcS79sFEml5t
8B9nays/Qg1XBPDADTrpunlkjbhMZBzth6uONNK3WJpEmW1ZqCvBf3RAM4NXdxsk
dTvXE55sj7ewtYA/g6Jcch1moZGF49d/Vemp+RkCgYASA7fm74ACbqjuw6m+WHip
vcFuQTJUtryi0aWYCQ4lmDoFHuSCop81qNMR7nyRbVLjcspJMXQM1gPZ5kZP7Auo
6CWie/fm8CLYWAY4QFnftDwhq2+vG3BL8YW3nJh3pY/zR14oXj1qrZnHiDPO7snH
bhPd7wctAxnkvH4eboDvtQKBgBoZGRfVhCjW2uA/d0WAAHltpMYsFSvpQ57aRdzd
Vs9B556vjfH34GKAO6sAqgrOae3+HdrLc4jfDe7MB+Ei6vV2QV5oH7vZbQeUDKp+
tojJQC6Y6kRgFQBDS5Wws0vlLPwOCuKqGWdgR404mnrxLmG/uVWRS5gxfeXAIP5r
RzXZAoGACsScBNd9WQknHXCLzQsOBOSJ08lG0by84hQDYdjO61fKRjfeuPEydzYX
KqGC0LiJo9rLnHC1+O7cBMfDOe8M+JBuhzeXDNyscRD+H+kqiCmpOa9bDBv4L23I
0ttqrosArGoebCLPNCBrew98nDhgNuNOj7cNo1PkqlZw+XtnbbQ=
-----END RSA PRIVATE KEY-----`

const testPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAs60D2vOdaxN5FxCp1ruP
Pf02WYloMve5Ky4fxyHpbrOGI8Yh1ZrO06+TzTOIUQ6Gvglb5xYWbcljuEVqI65t
b4RKw9lA5bmOuxfVvgCcpzTY2DTkOSSvJW3sS4rG+HPHnHZiEhvHh+PG7/iYhpn0
FxMJ9EQH4nE6JHWf+4TrwaIHKs/oyp4g0db0lm0TDZHblTyvC6lNQ1LrrV9uywOI
EdcuibCr/JhUcTEUB65+S2oyqhbtryp06KYrhXuApPW9X3I7UgKa/Re39cbVJpNI
rbJzVKXxyDLU0NWV54ELfOUy6eG6KrAPvPVpRYfOtXZEHvZ+e0qBw0U0/fs02loA
jwIDAQAB
-----END PUBLIC KEY-----`
