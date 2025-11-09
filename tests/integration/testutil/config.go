package testutil

import (
	"fmt"
	"os"
)

// Config holds integration test configuration
type Config struct {
	ServiceURL string

	// EPX test merchant credentials
	EPXMac        string
	EPXCustNbr    string
	EPXMerchNbr   string
	EPXDBANbr     string
	EPXTerminalNbr string
}

// LoadConfig loads test configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		ServiceURL:     getEnv("SERVICE_URL", "http://localhost:8080"),
		EPXMac:         getEnv("EPX_MAC_STAGING", ""),
		EPXCustNbr:     getEnv("EPX_CUST_NBR", ""),
		EPXMerchNbr:    getEnv("EPX_MERCH_NBR", ""),
		EPXDBANbr:      getEnv("EPX_DBA_NBR", ""),
		EPXTerminalNbr: getEnv("EPX_TERMINAL_NBR", ""),
	}

	// Validate required fields for tests that need them
	if cfg.EPXMac == "" {
		return nil, fmt.Errorf("EPX_MAC_STAGING is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
