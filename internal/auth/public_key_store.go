package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// PublicKeyStore manages public keys for JWT verification
type PublicKeyStore struct {
	keys map[string]*rsa.PublicKey // issuer -> public key
	mu   sync.RWMutex
}

// NewPublicKeyStore creates a new public key store
func NewPublicKeyStore() *PublicKeyStore {
	return &PublicKeyStore{
		keys: make(map[string]*rsa.PublicKey),
	}
}

// LoadKeysFromDirectory loads all .pem files from a directory
func (s *PublicKeyStore) LoadKeysFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read keys directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".pem" {
			continue
		}

		issuerName := entry.Name()[:len(entry.Name())-4] // Remove .pem extension
		keyPath := filepath.Join(dir, entry.Name())

		if err := s.LoadKey(issuerName, keyPath); err != nil {
			return fmt.Errorf("failed to load key for %s: %w", issuerName, err)
		}
	}

	return nil
}

// LoadKey loads a public key from a PEM file
func (s *PublicKeyStore) LoadKey(issuerName, keyPath string) error {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	publicKey, err := parseRSAPublicKey(keyData)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[issuerName] = publicKey

	return nil
}

// AddKey adds a public key directly (useful for testing)
func (s *PublicKeyStore) AddKey(issuerName string, publicKey *rsa.PublicKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[issuerName] = publicKey
}

// GetPublicKey retrieves a public key for an issuer
func (s *PublicKeyStore) GetPublicKey(issuerName string) (*rsa.PublicKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.keys[issuerName]
	if !ok {
		return nil, fmt.Errorf("unknown issuer: %s", issuerName)
	}

	return key, nil
}

// HasIssuer checks if an issuer is registered
func (s *PublicKeyStore) HasIssuer(issuerName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.keys[issuerName]
	return ok
}

// ListIssuers returns all registered issuer names
func (s *PublicKeyStore) ListIssuers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	issuers := make([]string, 0, len(s.keys))
	for issuer := range s.keys {
		issuers = append(issuers, issuer)
	}
	return issuers
}

// parseRSAPublicKey parses a PEM-encoded RSA public key
func parseRSAPublicKey(keyData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}
