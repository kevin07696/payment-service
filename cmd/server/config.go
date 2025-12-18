package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// envRequire returns the environment variable value or an error if not set.
// This is a pure function with no side effects - caller handles the error.
func envRequire(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("required environment variable %s not set", key)
	}
	return value, nil
}

// envParseInt parses a string to int.
// Returns an error if the value cannot be parsed.
func envParseInt(value string) (int, error) {
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value: %q", value)
	}
	return i, nil
}

// envParseInt32 parses a string to int32.
// Returns an error if the value cannot be parsed or is out of range.
func envParseInt32(value string) (int32, error) {
	i, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid int32 value: %q", value)
	}
	return int32(i), nil
}

// envParseFloat parses a string to float64.
// Returns an error if the value cannot be parsed.
func envParseFloat(value string) (float64, error) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float value: %q", value)
	}
	return f, nil
}

// envParseBool parses a string to bool.
// Accepts "true", "false", "1", "0" (case-insensitive for true/false).
// Returns an error for any other value.
func envParseBool(value string) (bool, error) {
	lower := strings.ToLower(value)
	switch lower {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %q (must be true, false, 1, or 0)", value)
	}
}

// envParseStringList parses a comma-separated string into a slice of strings.
// Empty string returns an empty slice (not an error).
// Whitespace around items is trimmed.
func envParseStringList(value string) []string {
	if value == "" {
		return nil
	}
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// configLoader collects configuration errors and provides helper methods
// for loading environment variables with proper error handling.
type configLoader struct {
	errs []error
}

// mustString requires and returns a string environment variable.
// Errors are collected, not immediately fatal.
func (c *configLoader) mustString(key string) string {
	v, err := envRequire(key)
	if err != nil {
		c.errs = append(c.errs, err)
	}
	return v
}

// mustInt requires and parses an integer environment variable.
// Errors are collected, not immediately fatal.
func (c *configLoader) mustInt(key string) int {
	v, err := envRequire(key)
	if err != nil {
		c.errs = append(c.errs, err)
		return 0
	}
	i, err := envParseInt(v)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("%s: %w", key, err))
	}
	return i
}

// mustInt32 requires and parses an int32 environment variable.
// Errors are collected, not immediately fatal.
func (c *configLoader) mustInt32(key string) int32 {
	v, err := envRequire(key)
	if err != nil {
		c.errs = append(c.errs, err)
		return 0
	}
	i, err := envParseInt32(v)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("%s: %w", key, err))
	}
	return i
}

// mustFloat requires and parses a float64 environment variable.
// Errors are collected, not immediately fatal.
func (c *configLoader) mustFloat(key string) float64 {
	v, err := envRequire(key)
	if err != nil {
		c.errs = append(c.errs, err)
		return 0
	}
	f, err := envParseFloat(v)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("%s: %w", key, err))
	}
	return f
}

// mustBool requires and parses a boolean environment variable.
// Errors are collected, not immediately fatal.
func (c *configLoader) mustBool(key string) bool {
	v, err := envRequire(key)
	if err != nil {
		c.errs = append(c.errs, err)
		return false
	}
	b, err := envParseBool(v)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("%s: %w", key, err))
	}
	return b
}

// hasErrors returns true if any errors were collected.
func (c *configLoader) hasErrors() bool {
	return len(c.errs) > 0
}

// errors returns all collected errors.
func (c *configLoader) errors() []error {
	return c.errs
}

// optionalString returns the env var value or empty string if not set.
// Used for optional values that can be empty (e.g., TRUSTED_PROXIES).
func (c *configLoader) optionalString(key string) string {
	return os.Getenv(key)
}

// optionalStringList parses an optional comma-separated list env var.
// Returns empty slice if not set.
func (c *configLoader) optionalStringList(key string) []string {
	return envParseStringList(os.Getenv(key))
}

// Config holds application configuration
type Config struct {
	Port        int    // Server port for all endpoints (ConnectRPC + REST)
	Environment string // Environment: development, staging, production

	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	MaxConns   int32
	MinConns   int32

	// EPX Payment Gateway - endpoints configured via environment variables in adapters
	// Requires: EPX_SERVER_POST_ENDPOINT, EPX_SERVER_POST_SOCKET_ENDPOINT,
	//           EPX_BROWSER_POST_ENDPOINT, EPX_KEY_EXCHANGE_ENDPOINT
	EPXTimeout     int
	EPXCustNbr     string
	EPXMerchNbr    string
	EPXDBAnbr      string
	EPXTerminalNbr string

	// North Merchant Reporting API (for disputes/chargebacks)
	NorthMerchantReportingURL string
	NorthTimeout              int

	// Browser Post Configuration
	CallbackBaseURL string

	// Cron configuration
	CronSecret            string
	CronJobTimeoutSeconds int
	CronDefaultBatchSize  int
	CronMaxBatchSize      int

	// Subscription configuration
	SubscriptionDefaultMaxRetries  int
	SubscriptionDefaultGracePeriod int
	SubscriptionRetryBaseDelaySecs int
	SubscriptionRetryMaxDelaySecs  int
	SubscriptionRetryMultiplier    float64

	// Authentication
	AuthSaltPrefix string
	EPXMacSecret   string
	AuthEnabled    bool

	// EPX API Credentials (for Business Reporting API)
	EPXAPIKey    string
	EPXAPISecret string

	// Rate Limiting
	TrustedProxies []string // List of trusted proxy IPs for X-Forwarded-For header

	// Browser Post Security
	AllowedReturnDomains []string // List of allowed domains for return_url (prevents open redirect)

	// Observability
	TracingEnabled    bool
	TracingSampleRate float64
	OTLPEndpoint      string
	MetricsEnabled    bool
	MetricsPort       string
}

// loadConfig loads configuration from environment variables.
// All required environment variables must be set - errors are collected and reported together.
func loadConfig(logger *zap.Logger) *Config {
	c := &configLoader{}

	cfg := &Config{
		// Port (unified for ConnectRPC + REST)
		Port:        c.mustInt("PORT"),
		Environment: c.mustString("ENVIRONMENT"),

		// Database
		DBHost:     c.mustString("DB_HOST"),
		DBPort:     c.mustInt("DB_PORT"),
		DBUser:     c.mustString("DB_USER"),
		DBPassword: c.mustString("DB_PASSWORD"),
		DBName:     c.mustString("DB_NAME"),
		DBSSLMode:  c.mustString("DB_SSL_MODE"),
		MaxConns:   c.mustInt32("DB_MAX_CONNS"),
		MinConns:   c.mustInt32("DB_MIN_CONNS"),

		// EPX configuration (endpoints configured in adapters via EPX_*_ENDPOINT env vars)
		EPXTimeout:     c.mustInt("EPX_TIMEOUT"),
		EPXCustNbr:     c.mustString("EPX_CUST_NBR"),
		EPXMerchNbr:    c.mustString("EPX_MERCH_NBR"),
		EPXDBAnbr:      c.mustString("EPX_DBA_NBR"),
		EPXTerminalNbr: c.mustString("EPX_TERMINAL_NBR"),

		// North Reporting API
		NorthMerchantReportingURL: c.mustString("NORTH_MERCHANT_REPORTING_URL"),
		NorthTimeout:              c.mustInt("NORTH_TIMEOUT"),

		// Browser POST callback
		CallbackBaseURL: c.mustString("CALLBACK_BASE_URL"),

		// Cron configuration
		CronSecret:            c.mustString("CRON_SECRET"),
		CronJobTimeoutSeconds: c.mustInt("CRON_JOB_TIMEOUT_SECONDS"),
		CronDefaultBatchSize:  c.mustInt("CRON_DEFAULT_BATCH_SIZE"),
		CronMaxBatchSize:      c.mustInt("CRON_MAX_BATCH_SIZE"),

		// Subscription configuration
		SubscriptionDefaultMaxRetries:  c.mustInt("SUBSCRIPTION_DEFAULT_MAX_RETRIES"),
		SubscriptionDefaultGracePeriod: c.mustInt("SUBSCRIPTION_DEFAULT_GRACE_PERIOD_DAYS"),
		SubscriptionRetryBaseDelaySecs: c.mustInt("SUBSCRIPTION_RETRY_BASE_DELAY_SECS"),
		SubscriptionRetryMaxDelaySecs:  c.mustInt("SUBSCRIPTION_RETRY_MAX_DELAY_SECS"),
		SubscriptionRetryMultiplier:    c.mustFloat("SUBSCRIPTION_RETRY_MULTIPLIER"),

		// Authentication
		AuthSaltPrefix: c.mustString("AUTH_SALT_PREFIX"),
		EPXMacSecret:   c.optionalString("EPX_SANDBOX_MAC"), // Optional: per-merchant MAC in production
		AuthEnabled:    c.mustBool("AUTH_ENABLED"),

		// EPX API Credentials (optional: for Business Reporting API)
		EPXAPIKey:    c.optionalString("EPX_API_KEY"),
		EPXAPISecret: c.optionalString("EPX_API_SECRET"),

		// Rate Limiting - comma-separated list of trusted proxy IPs (can be empty)
		TrustedProxies: c.optionalStringList("TRUSTED_PROXIES"),

		// Browser Post Security - comma-separated list of allowed domains (can be empty in dev)
		AllowedReturnDomains: c.optionalStringList("ALLOWED_RETURN_DOMAINS"),

		// Observability - optional with sensible defaults
		TracingEnabled:    envParseBoolDefault(os.Getenv("TRACING_ENABLED"), false),
		TracingSampleRate: envParseFloatDefault(os.Getenv("TRACING_SAMPLE_RATE"), 0.1),
		OTLPEndpoint:      envStringDefault(os.Getenv("OTLP_ENDPOINT"), "localhost:4318"),
		MetricsEnabled:    envParseBoolDefault(os.Getenv("METRICS_ENABLED"), true),
		MetricsPort:       envStringDefault(os.Getenv("METRICS_PORT"), "9090"),
	}

	// Report all configuration errors at once (better DX)
	if c.hasErrors() {
		for _, err := range c.errors() {
			logger.Error("Configuration error", zap.Error(err))
		}
		logger.Fatal("Failed to load configuration",
			zap.Int("error_count", len(c.errors())),
			zap.String("suggestion", "Ensure all required environment variables are set"),
		)
	}

	// Post-load validations
	validateConfig(cfg, logger)

	logger.Info("Configuration loaded",
		zap.Int("port", cfg.Port),
		zap.String("environment", cfg.Environment),
		zap.String("db_host", cfg.DBHost),
		zap.Int("db_port", cfg.DBPort),
	)

	return cfg
}

// validateConfig validates configuration constraints after loading
func validateConfig(cfg *Config, logger *zap.Logger) {
	// Validate CRON_SECRET security requirements
	if cfg.CronSecret == "change-me-in-production" {
		logger.Fatal("CRON_SECRET must be changed from default value",
			zap.String("suggestion", "Generate with: openssl rand -base64 32"),
		)
	}

	// Minimum length check - ensure sufficient entropy (256 bits recommended)
	if len(cfg.CronSecret) < 32 {
		logger.Fatal("CRON_SECRET must be at least 32 characters for sufficient entropy",
			zap.Int("current_length", len(cfg.CronSecret)),
			zap.Int("required_length", 32),
			zap.String("suggestion", "Generate with: openssl rand -base64 32"),
		)
	}

	// Validate ALLOWED_RETURN_DOMAINS in production to prevent open redirect attacks
	if cfg.Environment == "production" && len(cfg.AllowedReturnDomains) == 0 {
		logger.Fatal("ALLOWED_RETURN_DOMAINS is required in production to prevent open redirect attacks",
			zap.String("example", "example.com,app.example.com"),
			zap.String("documentation", "Set comma-separated list of allowed domains for Browser POST return URLs"),
		)
	}
}

// Helper functions for optional values with defaults

func envStringDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func envParseBoolDefault(value string, defaultValue bool) bool {
	if value == "" {
		return defaultValue
	}
	b, err := envParseBool(value)
	if err != nil {
		return defaultValue
	}
	return b
}

func envParseFloatDefault(value string, defaultValue float64) float64 {
	if value == "" {
		return defaultValue
	}
	f, err := envParseFloat(value)
	if err != nil {
		return defaultValue
	}
	return f
}
