package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAddKey_GetPublicKey tests adding and retrieving keys
func TestAddKey_GetPublicKey(t *testing.T) {
	store := NewPublicKeyStore()

	// Generate test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Add key
	store.AddKey("test-issuer", &privateKey.PublicKey)

	// Retrieve key
	retrievedKey, err := store.GetPublicKey("test-issuer")
	require.NoError(t, err)
	assert.Equal(t, &privateKey.PublicKey, retrievedKey)
}

// TestGetPublicKey_UnknownIssuer tests error for unknown issuer
func TestGetPublicKey_UnknownIssuer(t *testing.T) {
	store := NewPublicKeyStore()

	key, err := store.GetPublicKey("unknown-issuer")
	assert.Error(t, err)
	assert.Nil(t, key)
	assert.Contains(t, err.Error(), "unknown issuer")
}

// TestHasIssuer tests issuer existence check
func TestHasIssuer(t *testing.T) {
	store := NewPublicKeyStore()

	// Should not have issuer initially
	assert.False(t, store.HasIssuer("test-issuer"))

	// Add key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	store.AddKey("test-issuer", &privateKey.PublicKey)

	// Should have issuer now
	assert.True(t, store.HasIssuer("test-issuer"))
}

// TestListIssuers tests listing all issuers
func TestListIssuers(t *testing.T) {
	store := NewPublicKeyStore()

	// Should be empty initially
	assert.Empty(t, store.ListIssuers())

	// Add multiple keys
	for i := 0; i < 3; i++ {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		store.AddKey(fmt.Sprintf("issuer-%d", i), &privateKey.PublicKey)
	}

	// Should list all issuers
	issuers := store.ListIssuers()
	assert.Len(t, issuers, 3)
	assert.Contains(t, issuers, "issuer-0")
	assert.Contains(t, issuers, "issuer-1")
	assert.Contains(t, issuers, "issuer-2")
}

// TestLoadKey tests loading key from PEM file
func TestLoadKey(t *testing.T) {
	store := NewPublicKeyStore()

	// Create temporary directory
	tempDir := t.TempDir()

	// Generate test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Write public key to PEM file
	keyPath := filepath.Join(tempDir, "test-issuer.pem")
	err = writePublicKeyPEM(keyPath, &privateKey.PublicKey)
	require.NoError(t, err)

	// Load key
	err = store.LoadKey("test-issuer", keyPath)
	require.NoError(t, err)

	// Verify key was loaded
	retrievedKey, err := store.GetPublicKey("test-issuer")
	require.NoError(t, err)
	assert.NotNil(t, retrievedKey)
	assert.Equal(t, privateKey.PublicKey.N, retrievedKey.N)
	assert.Equal(t, privateKey.PublicKey.E, retrievedKey.E)
}

// TestLoadKey_InvalidFile tests loading from non-existent file
func TestLoadKey_InvalidFile(t *testing.T) {
	store := NewPublicKeyStore()

	err := store.LoadKey("test-issuer", "/nonexistent/path/key.pem")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read key file")
}

// TestLoadKey_InvalidPEM tests loading invalid PEM data
func TestLoadKey_InvalidPEM(t *testing.T) {
	store := NewPublicKeyStore()

	// Create temporary file with invalid data
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "invalid.pem")
	err := os.WriteFile(keyPath, []byte("not a valid PEM file"), 0600)
	require.NoError(t, err)

	err = store.LoadKey("test-issuer", keyPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

// TestLoadKeysFromDirectory tests loading all keys from directory
func TestLoadKeysFromDirectory(t *testing.T) {
	store := NewPublicKeyStore()

	// Create temporary directory
	tempDir := t.TempDir()

	// Create multiple key files
	issuers := []string{"issuer-1", "issuer-2", "issuer-3"}
	for _, issuer := range issuers {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		keyPath := filepath.Join(tempDir, issuer+".pem")
		err = writePublicKeyPEM(keyPath, &privateKey.PublicKey)
		require.NoError(t, err)
	}

	// Create a non-PEM file (should be ignored)
	err := os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("test"), 0600)
	require.NoError(t, err)

	// Create a subdirectory (should be ignored)
	err = os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)
	require.NoError(t, err)

	// Load all keys
	err = store.LoadKeysFromDirectory(tempDir)
	require.NoError(t, err)

	// Verify all keys were loaded
	loadedIssuers := store.ListIssuers()
	assert.Len(t, loadedIssuers, 3)
	for _, issuer := range issuers {
		assert.True(t, store.HasIssuer(issuer))
	}
}

// TestLoadKeysFromDirectory_InvalidDirectory tests loading from non-existent directory
func TestLoadKeysFromDirectory_InvalidDirectory(t *testing.T) {
	store := NewPublicKeyStore()

	err := store.LoadKeysFromDirectory("/nonexistent/directory")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read keys directory")
}

// TestLoadKeysFromDirectory_InvalidKey tests handling invalid key in directory
func TestLoadKeysFromDirectory_InvalidKey(t *testing.T) {
	store := NewPublicKeyStore()

	// Create temporary directory
	tempDir := t.TempDir()

	// Create valid key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPath := filepath.Join(tempDir, "valid.pem")
	err = writePublicKeyPEM(keyPath, &privateKey.PublicKey)
	require.NoError(t, err)

	// Create invalid key
	invalidPath := filepath.Join(tempDir, "invalid.pem")
	err = os.WriteFile(invalidPath, []byte("invalid pem data"), 0600)
	require.NoError(t, err)

	// Should fail on invalid key
	err = store.LoadKeysFromDirectory(tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load key")
}

// TestConcurrentAccess tests thread-safety
func TestConcurrentAccess(t *testing.T) {
	store := NewPublicKeyStore()

	// Generate initial keys
	keys := make([]*rsa.PublicKey, 10)
	for i := 0; i < 10; i++ {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		keys[i] = &privateKey.PublicKey
	}

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			issuer := fmt.Sprintf("issuer-%d", index)
			store.AddKey(issuer, keys[index])
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			issuer := fmt.Sprintf("issuer-%d", index)
			// May not exist yet, but shouldn't panic
			_, _ = store.GetPublicKey(issuer)
		}(i)
	}

	// Concurrent checks
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			issuer := fmt.Sprintf("issuer-%d", index)
			_ = store.HasIssuer(issuer)
		}(i)
	}

	// Concurrent list operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.ListIssuers()
		}()
	}

	wg.Wait()

	// Verify all keys were added
	assert.Len(t, store.ListIssuers(), 10)
}

// TestParseRSAPublicKey tests PEM parsing
func TestParseRSAPublicKey(t *testing.T) {
	// Generate test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Marshal to PEM
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	// Parse PEM
	parsedKey, err := parseRSAPublicKey(pemData)
	require.NoError(t, err)
	assert.NotNil(t, parsedKey)
	assert.Equal(t, privateKey.PublicKey.N, parsedKey.N)
	assert.Equal(t, privateKey.PublicKey.E, parsedKey.E)
}

// TestParseRSAPublicKey_InvalidPEM tests parsing invalid PEM
func TestParseRSAPublicKey_InvalidPEM(t *testing.T) {
	tests := []struct {
		name    string
		pemData []byte
		errMsg  string
	}{
		{
			name:    "empty data",
			pemData: []byte(""),
			errMsg:  "failed to parse PEM block",
		},
		{
			name:    "not PEM format",
			pemData: []byte("not a PEM file"),
			errMsg:  "failed to parse PEM block",
		},
		{
			name: "invalid key data",
			pemData: pem.EncodeToMemory(&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: []byte("invalid key bytes"),
			}),
			errMsg: "failed to parse public key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := parseRSAPublicKey(tt.pemData)
			assert.Error(t, err)
			assert.Nil(t, key)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// Helper function to write RSA public key to PEM file
func writePublicKeyPEM(path string, publicKey *rsa.PublicKey) error {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return err
	}

	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return os.WriteFile(path, pemData, 0600)
}
