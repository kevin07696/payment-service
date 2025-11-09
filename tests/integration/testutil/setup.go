package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Setup initializes test environment and returns config and client
func Setup(t *testing.T) (*Config, *Client) {
	t.Helper()

	// Load config from environment
	cfg, err := LoadConfig()
	require.NoError(t, err, "Failed to load test configuration")

	// Create API client
	client := NewClient(cfg.ServiceURL)

	t.Logf("Integration test setup complete - service: %s", cfg.ServiceURL)

	return cfg, client
}
