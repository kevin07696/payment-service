package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
)

// TestServiceCredentials represents a test service with RSA keys
type TestServiceCredentials struct {
	ServiceID            string `json:"service_id"`
	ServiceName          string `json:"service_name"`
	Environment          string `json:"environment"`
	PrivateKeyPEM        string `json:"private_key_pem"`
	PublicKeyPEM         string `json:"public_key_pem"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
}

func main() {
	outputPath := flag.String("output", "", "Output file path for test_services.json")
	flag.Parse()

	if *outputPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -output flag is required\n")
		os.Exit(1)
	}

	// Generate 3 test services with unique keys
	services := []TestServiceCredentials{}
	for i := 1; i <= 3; i++ {
		service, err := generateTestService(i)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating service %d: %v\n", i, err)
			os.Exit(1)
		}
		services = append(services, service)
		fmt.Printf("✅ Generated test-service-%03d\n", i)
	}

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	// Write to file
	if err := os.WriteFile(*outputPath, jsonData, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Successfully wrote %d test services to %s\n", len(services), *outputPath)
}

// generateTestService creates a test service with fresh RSA keys
func generateTestService(index int) (TestServiceCredentials, error) {
	// Generate 2048-bit RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return TestServiceCredentials{}, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Encode private key to PEM
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key to PEM
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return TestServiceCredentials{}, fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Calculate public key fingerprint (SHA-256 hash)
	hash := sha256.Sum256(publicKeyBytes)
	fingerprint := hex.EncodeToString(hash[:])

	return TestServiceCredentials{
		ServiceID:            fmt.Sprintf("test-service-%03d", index),
		ServiceName:          fmt.Sprintf("Test Service %d", index),
		Environment:          "test",
		PrivateKeyPEM:        string(privateKeyPEM),
		PublicKeyPEM:         string(publicKeyPEM),
		PublicKeyFingerprint: fingerprint,
	}, nil
}
