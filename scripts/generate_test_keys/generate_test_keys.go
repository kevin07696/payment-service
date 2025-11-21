package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kevin07696/payment-service/pkg/crypto"
)

// TestService represents a test service with credentials
type TestService struct {
	ServiceID            string `json:"service_id"`
	ServiceName          string `json:"service_name"`
	Environment          string `json:"environment"`
	PrivateKeyPEM        string `json:"private_key_pem"`
	PublicKeyPEM         string `json:"public_key_pem"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
}

func main() {
	// Define test services to generate
	services := []struct {
		id   string
		name string
		env  string
	}{
		{"test-service-001", "Test Service 1", "test"},
		{"test-service-002", "Test Service 2", "test"},
		{"test-service-003", "Test Service 3", "test"},
	}

	// Create output directory
	outputDir := "tests/fixtures/auth"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	var testServices []TestService

	// Generate keys for each service
	for _, svc := range services {
		fmt.Printf("Generating RSA key pair for %s...\n", svc.name)

		keyPair, err := crypto.GenerateRSAKeyPair()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate key pair for %s: %v\n", svc.name, err)
			os.Exit(1)
		}

		testService := TestService{
			ServiceID:            svc.id,
			ServiceName:          svc.name,
			Environment:          svc.env,
			PrivateKeyPEM:        keyPair.PrivateKeyPEM,
			PublicKeyPEM:         keyPair.PublicKeyPEM,
			PublicKeyFingerprint: keyPair.Fingerprint,
		}

		testServices = append(testServices, testService)

		// Write individual key files
		privateKeyPath := filepath.Join(outputDir, fmt.Sprintf("%s_private.pem", svc.id))
		if err := os.WriteFile(privateKeyPath, []byte(keyPair.PrivateKeyPEM), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write private key: %v\n", err)
			os.Exit(1)
		}

		publicKeyPath := filepath.Join(outputDir, fmt.Sprintf("%s_public.pem", svc.id))
		if err := os.WriteFile(publicKeyPath, []byte(keyPair.PublicKeyPEM), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write public key: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("  ✓ Generated keys for %s\n", svc.name)
		fmt.Printf("    Service ID: %s\n", svc.id)
		fmt.Printf("    Fingerprint: %s\n", keyPair.Fingerprint)
		fmt.Printf("    Private key: %s\n", privateKeyPath)
		fmt.Printf("    Public key: %s\n\n", publicKeyPath)
	}

	// Write JSON file with all services
	jsonPath := filepath.Join(outputDir, "test_services.json")
	jsonData, err := json.MarshalIndent(testServices, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write JSON file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Successfully generated %d test service key pairs\n", len(testServices))
	fmt.Printf("   Output directory: %s\n", outputDir)
	fmt.Printf("   JSON manifest: %s\n", jsonPath)
}
