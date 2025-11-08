package integration

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// TestConfig holds configuration for integration tests
type TestConfig struct {
	GRPCHost    string
	HTTPHost    string
	CronSecret  string
	Environment string
}

// LoadTestConfig loads configuration from environment variables
func LoadTestConfig() *TestConfig {
	return &TestConfig{
		GRPCHost:    getEnv("GRPC_HOST", "localhost:8080"),
		HTTPHost:    getEnv("HTTP_HOST", "localhost:8081"),
		CronSecret:  getEnv("CRON_SECRET", ""),
		Environment: getEnv("ENVIRONMENT", "staging"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestHealthEndpoints verifies all health check endpoints are responding
func TestHealthEndpoints(t *testing.T) {
	cfg := LoadTestConfig()

	tests := []struct {
		name     string
		endpoint string
		wantCode int
	}{
		{
			name:     "Cron Health Check",
			endpoint: fmt.Sprintf("http://%s/cron/health", cfg.HTTPHost),
			wantCode: http.StatusOK,
		},
		{
			name:     "Cron Stats",
			endpoint: fmt.Sprintf("http://%s/cron/stats", cfg.HTTPHost),
			wantCode: http.StatusOK,
		},
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Get(tt.endpoint)
			if err != nil {
				t.Fatalf("Failed to GET %s: %v", tt.endpoint, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantCode {
				t.Errorf("GET %s status = %d, want %d", tt.endpoint, resp.StatusCode, tt.wantCode)
			}

			t.Logf("✅ %s responded with %d", tt.name, resp.StatusCode)
		})
	}
}

// TestGRPCConnectivity verifies gRPC server is accessible
func TestGRPCConnectivity(t *testing.T) {
	cfg := LoadTestConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try both secure and insecure connections
	tests := []struct {
		name   string
		secure bool
	}{
		{"Insecure Connection", false},
		{"Secure Connection (if available)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []grpc.DialOption

			if tt.secure {
				creds := credentials.NewTLS(&tls.Config{
					InsecureSkipVerify: true,
				})
				opts = append(opts, grpc.WithTransportCredentials(creds))
			} else {
				opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
			}

			conn, err := grpc.DialContext(ctx, cfg.GRPCHost, opts...)
			if err != nil {
				if tt.secure {
					t.Skipf("Secure connection failed (expected if TLS not configured): %v", err)
					return
				}
				t.Fatalf("Failed to connect to gRPC server: %v", err)
			}
			defer conn.Close()

			// Check connection state
			state := conn.GetState()
			t.Logf("✅ gRPC connection state: %s", state)

			if state.String() == "SHUTDOWN" {
				t.Error("gRPC connection is SHUTDOWN")
			}
		})
	}
}

// TestServiceReadiness verifies the service is fully ready
func TestServiceReadiness(t *testing.T) {
	cfg := LoadTestConfig()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Poll health endpoint until ready or timeout
	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		resp, err := client.Get(fmt.Sprintf("http://%s/cron/health", cfg.HTTPHost))
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			t.Logf("✅ Service ready after %d attempts", i+1)
			return
		}

		if resp != nil {
			resp.Body.Close()
		}

		if i == maxAttempts-1 {
			t.Fatalf("Service not ready after %d attempts (last error: %v)", maxAttempts, err)
		}

		t.Logf("Waiting for service readiness (attempt %d/%d)...", i+1, maxAttempts)
		time.Sleep(2 * time.Second)
	}
}

// TestEnvironmentConfiguration verifies environment is correctly configured
func TestEnvironmentConfiguration(t *testing.T) {
	cfg := LoadTestConfig()

	if cfg.Environment != "staging" {
		t.Errorf("Expected staging environment, got: %s", cfg.Environment)
	}

	t.Logf("✅ Environment: %s", cfg.Environment)
	t.Logf("✅ gRPC Host: %s", cfg.GRPCHost)
	t.Logf("✅ HTTP Host: %s", cfg.HTTPHost)
}

// TestDatabaseConnectivity verifies database is accessible
// This test will call a simple endpoint that requires DB access
func TestDatabaseConnectivity(t *testing.T) {
	cfg := LoadTestConfig()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Stats endpoint requires DB access to count subscriptions/transactions
	resp, err := client.Get(fmt.Sprintf("http://%s/cron/stats", cfg.HTTPHost))
	if err != nil {
		t.Fatalf("Failed to call stats endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Stats endpoint returned %d, database may not be accessible", resp.StatusCode)
	}

	t.Log("✅ Database connectivity verified via stats endpoint")
}

// TestCronAuthentication verifies cron endpoints require authentication
func TestCronAuthentication(t *testing.T) {
	cfg := LoadTestConfig()

	if cfg.CronSecret == "" {
		t.Skip("CRON_SECRET not set, skipping authentication test")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Test without authentication
	t.Run("Without Auth", func(t *testing.T) {
		resp, err := client.Get(fmt.Sprintf("http://%s/cron/run", cfg.HTTPHost))
		if err != nil {
			t.Fatalf("Failed to call endpoint: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
			t.Errorf("Expected 401/403 without auth, got %d", resp.StatusCode)
		}

		t.Log("✅ Cron endpoints properly require authentication")
	})

	// Test with authentication
	t.Run("With Auth", func(t *testing.T) {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/cron/run", cfg.HTTPHost), nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		req.Header.Set("X-Cron-Secret", cfg.CronSecret)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to call endpoint: %v", err)
		}
		defer resp.Body.Close()

		// Should not be unauthorized (may be 200 or other status, but not 401/403)
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			t.Errorf("Authentication failed with correct secret, got %d", resp.StatusCode)
		}

		t.Logf("✅ Cron authentication working (status: %d)", resp.StatusCode)
	})
}

// TestResponseTimes verifies endpoints respond within acceptable time
func TestResponseTimes(t *testing.T) {
	cfg := LoadTestConfig()

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	tests := []struct {
		name        string
		endpoint    string
		maxDuration time.Duration
	}{
		{
			name:        "Health Check",
			endpoint:    fmt.Sprintf("http://%s/cron/health", cfg.HTTPHost),
			maxDuration: 1 * time.Second,
		},
		{
			name:        "Stats Endpoint",
			endpoint:    fmt.Sprintf("http://%s/cron/stats", cfg.HTTPHost),
			maxDuration: 3 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			resp, err := client.Get(tt.endpoint)
			duration := time.Since(start)

			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			resp.Body.Close()

			if duration > tt.maxDuration {
				t.Errorf("Response time %v exceeded maximum %v", duration, tt.maxDuration)
			}

			t.Logf("✅ %s responded in %v (max: %v)", tt.name, duration, tt.maxDuration)
		})
	}
}
