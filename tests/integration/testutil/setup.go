package testutil

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

// Setup initializes test environment and returns config and client
func Setup(t *testing.T) (*Config, *Client) {
	t.Helper()

	// Load config from environment
	cfg, err := LoadConfig()
	require.NoError(t, err, "Failed to load test configuration")

	// Seed test merchant with EPX credentials from environment
	seedTestMerchant(t, cfg)

	// Create API client
	client := NewClient(cfg.ServiceURL)

	t.Logf("Integration test setup complete - service: %s", cfg.ServiceURL)

	return cfg, client
}

// GetDB returns a database connection for direct SQL operations in tests
func GetDB(t *testing.T) *sql.DB {
	t.Helper()

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "payment_service"
	}

	connStr := "host=" + dbHost + " port=" + dbPort + " user=" + dbUser +
		" password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err, "Failed to connect to database")

	// Test connection
	err = db.Ping()
	require.NoError(t, err, "Failed to ping database")

	return db
}

// seedTestMerchant ensures the test merchant exists with correct EPX credentials
func seedTestMerchant(t *testing.T, cfg *Config) {
	t.Helper()

	// Connect to database
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "payment_service"
	}

	connStr := "host=" + dbHost + " port=" + dbPort + " user=" + dbUser +
		" password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Insert or update test merchant with EPX credentials from environment
	_, err = db.Exec(`
		INSERT INTO merchants (
			id,
			slug,
			mac_secret_path,
			cust_nbr,
			merch_nbr,
			dba_nbr,
			terminal_nbr,
			environment,
			name,
			is_active,
			created_at,
			updated_at
		) VALUES (
			'00000000-0000-0000-0000-000000000001'::uuid,
			'test-merchant-integration',
			'/epx/staging/mac_secret',
			$1, $2, $3, $4,
			'test',
			'Integration Test Merchant',
			true,
			NOW(),
			NOW()
		) ON CONFLICT (id) DO UPDATE SET
			cust_nbr = EXCLUDED.cust_nbr,
			merch_nbr = EXCLUDED.merch_nbr,
			dba_nbr = EXCLUDED.dba_nbr,
			terminal_nbr = EXCLUDED.terminal_nbr,
			updated_at = NOW()
	`, cfg.EPXCustNbr, cfg.EPXMerchNbr, cfg.EPXDBANbr, cfg.EPXTerminalNbr)

	require.NoError(t, err, "Failed to seed test merchant")

	t.Logf("✅ Test merchant seeded with EPX credentials: CUST_NBR=%s, MERCH_NBR=%s, DBA_NBR=%s, TERMINAL_NBR=%s",
		cfg.EPXCustNbr, cfg.EPXMerchNbr, cfg.EPXDBANbr, cfg.EPXTerminalNbr)
}
