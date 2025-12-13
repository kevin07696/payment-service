package testutil

import (
	"fmt"
	"os"
)

// Config holds integration test configuration
// All values are required - no hardcoded defaults to ensure fail-fast behavior
type Config struct {
	ServiceURL      string // Server URL (ConnectRPC + REST on same port)
	CallbackBaseURL string // Base URL for Browser Post callbacks
	CronSecret      string // Secret for authenticating cron requests

	// EPX test merchant credentials (for tokenization and API calls)
	EPXMac         string
	EPXCustNbr     string
	EPXMerchNbr    string
	EPXDBANbr      string
	EPXTerminalNbr string
}

// LoadConfig loads test configuration from environment variables.
// Returns error if required variables are missing - fail fast, no silent defaults.
func LoadConfig() (*Config, error) {
	var missing []string

	// Required for all integration tests
	serviceURL := os.Getenv("SERVICE_URL")
	if serviceURL == "" {
		missing = append(missing, "SERVICE_URL")
	}

	cronSecret := os.Getenv("CRON_SECRET")
	if cronSecret == "" {
		missing = append(missing, "CRON_SECRET")
	}

	// EPX credentials - required for payment tests
	epxCustNbr := os.Getenv("EPX_CUST_NBR")
	if epxCustNbr == "" {
		missing = append(missing, "EPX_CUST_NBR")
	}

	epxMerchNbr := os.Getenv("EPX_MERCH_NBR")
	if epxMerchNbr == "" {
		missing = append(missing, "EPX_MERCH_NBR")
	}

	epxDbaNbr := os.Getenv("EPX_DBA_NBR")
	if epxDbaNbr == "" {
		missing = append(missing, "EPX_DBA_NBR")
	}

	epxTerminalNbr := os.Getenv("EPX_TERMINAL_NBR")
	if epxTerminalNbr == "" {
		missing = append(missing, "EPX_TERMINAL_NBR")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"missing required environment variables for integration tests: %v\n\n"+
				"Set these before running tests:\n"+
				"  export SERVICE_URL=\"http://localhost:8081\"\n"+
				"  export CRON_SECRET=\"your-cron-secret-at-least-32-chars\"\n"+
				"  export EPX_CUST_NBR=\"9001\"\n"+
				"  export EPX_MERCH_NBR=\"900300\"\n"+
				"  export EPX_DBA_NBR=\"2\"\n"+
				"  export EPX_TERMINAL_NBR=\"77\"",
			missing,
		)
	}

	// Optional: CALLBACK_BASE_URL defaults to SERVICE_URL if not set
	callbackBaseURL := os.Getenv("CALLBACK_BASE_URL")
	if callbackBaseURL == "" {
		callbackBaseURL = serviceURL
	}

	// Optional: EPX_MAC_STAGING only needed for tokenization tests
	epxMac := os.Getenv("EPX_MAC_STAGING")

	return &Config{
		ServiceURL:      serviceURL,
		CallbackBaseURL: callbackBaseURL,
		CronSecret:      cronSecret,
		EPXMac:          epxMac,
		EPXCustNbr:      epxCustNbr,
		EPXMerchNbr:     epxMerchNbr,
		EPXDBANbr:       epxDbaNbr,
		EPXTerminalNbr:  epxTerminalNbr,
	}, nil
}
