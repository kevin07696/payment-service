package testutil

import (
	"os"
)

// Config holds integration test configuration
type Config struct {
	ServiceURL string

	// EPX test merchant credentials (for tokenization and API calls)
	EPXMac         string
	EPXCustNbr     string
	EPXMerchNbr    string
	EPXDBANbr      string
	EPXTerminalNbr string
}

// LoadConfig loads test configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		ServiceURL:     getEnv("SERVICE_URL", "http://localhost:8081"),
		EPXMac:         getEnv("EPX_MAC_STAGING", ""),
		EPXCustNbr:     getEnv("EPX_CUST_NBR", "9001"),    // EPX sandbox defaults
		EPXMerchNbr:    getEnv("EPX_MERCH_NBR", "900300"),
		EPXDBANbr:      getEnv("EPX_DBA_NBR", "2"),
		EPXTerminalNbr: getEnv("EPX_TERMINAL_NBR", "77"),
	}

	// EPX credentials are optional - they're only required for tests that use tokenization
	// The tokenization functions will check and fail if credentials are missing

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
