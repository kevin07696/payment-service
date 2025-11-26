package config

// Config holds all application configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Gateway  GatewayConfig
	Logger   LoggerConfig
}

// ServerConfig holds gRPC server configuration
type ServerConfig struct {
	Port        int
	Host        string
	MetricsPort int
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	MaxConns int32
	MinConns int32
}

// GatewayConfig holds North payment gateway configuration
type GatewayConfig struct {
	BaseURL string // Base URL for North API (e.g., https://api.north.com/api/browserpost)
	EPIId   string // EPI-Id in format: CUST_NBR-MERCH_NBR-TERM_NBR-1 (e.g., 7000-700010-1-1)
	EPIKey  string // Secret key for HMAC-SHA256 authentication
	Timeout int    // Request timeout in seconds (default: 30)
}

// LoggerConfig holds logging configuration
type LoggerConfig struct {
	Level       string // debug, info, warn, error
	Development bool
}

// Note: Configuration is loaded via DATABASE_URL environment variable directly.
// The struct types above are retained for reference but LoadFromEnv() and helpers
// were removed as unused - the server reads DATABASE_URL directly.
