package config

import (
	"fmt"
	"os"
	"strconv"
)

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

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:        getEnvAsInt("SERVER_PORT", 50051),
			Host:        getEnv("SERVER_HOST", "0.0.0.0"),
			MetricsPort: getEnvAsInt("METRICS_PORT", 9090),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Database: getEnv("DB_NAME", "payment_service"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
			MaxConns: int32(getEnvAsInt("DB_MAX_CONNS", 25)),
			MinConns: int32(getEnvAsInt("DB_MIN_CONNS", 5)),
		},
		Gateway: GatewayConfig{
			BaseURL: getEnv("NORTH_BASE_URL", "https://sandbox.north.com/api/browserpost"),
			EPIId:   getEnv("NORTH_EPI_ID", ""),
			EPIKey:  getEnv("NORTH_EPI_KEY", ""),
			Timeout: getEnvAsInt("NORTH_TIMEOUT", 30),
		},
		Logger: LoggerConfig{
			Level:       getEnv("LOG_LEVEL", "info"),
			Development: getEnvAsBool("LOG_DEVELOPMENT", false),
		},
	}

	// Validate required fields
	if cfg.Database.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}
	if cfg.Gateway.EPIId == "" {
		return nil, fmt.Errorf("NORTH_EPI_ID is required")
	}
	if cfg.Gateway.EPIKey == "" {
		return nil, fmt.Errorf("NORTH_EPI_KEY is required")
	}

	return cfg, nil
}

// ConnectionString returns PostgreSQL connection string
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
