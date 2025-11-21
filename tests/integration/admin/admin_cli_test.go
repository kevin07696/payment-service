//go:build integration
// +build integration

package admin

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdminCLI_ServiceCreation tests the complete flow:
// 1. Create service with RSA keypair
// 2. Verify service is stored in database
// 3. Verify public key is stored, private key is returned
func TestAdminCLI_ServiceCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup database
	ctx := context.Background()
	dbURL := getTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	// Test service creation flow
	t.Run("Create service with RSA keypair", func(t *testing.T) {
		// Generate RSA keypair (simulating admin CLI)
		privateKey, publicKey, err := auth.GenerateRSAKeyPair(2048)
		require.NoError(t, err)

		publicKeyPEM, err := auth.PublicKeyToPEM(publicKey)
		require.NoError(t, err)

		// Insert into services table
		serviceID := "test-cli-service-001"
		_, err = db.Exec(`
			INSERT INTO services (
				id, service_id, service_name, public_key,
				public_key_fingerprint, environment,
				requests_per_second, burst_limit, is_active
			) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (service_id) DO UPDATE SET
				public_key = EXCLUDED.public_key
		`, serviceID, "Test CLI Service", string(publicKeyPEM),
			"test-fingerprint", "staging", 100, 200, true)
		require.NoError(t, err)

		// Verify service was created
		var storedPublicKey string
		err = db.QueryRow(`
			SELECT public_key FROM services WHERE service_id = $1
		`, serviceID).Scan(&storedPublicKey)
		require.NoError(t, err)
		assert.Equal(t, string(publicKeyPEM), storedPublicKey)

		// Verify we have the private key (this would be saved to file in real CLI)
		privateKeyPEM := auth.PrivateKeyToPEM(privateKey)
		assert.NotEmpty(t, privateKeyPEM)
		assert.Contains(t, string(privateKeyPEM), "BEGIN RSA PRIVATE KEY")

		// Cleanup
		_, _ = db.Exec(`DELETE FROM services WHERE service_id = $1`, serviceID)
	})
}

// TestAdminCLI_MerchantCreation tests merchant creation:
// 1. Create merchant with EPX credentials
// 2. Verify NO API keys are generated (old broken behavior)
// 3. Verify merchant stores only EPX credentials
func TestAdminCLI_MerchantCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup database
	ctx := context.Background()
	dbURL := getTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	t.Run("Create merchant with EPX credentials only", func(t *testing.T) {
		merchantSlug := "test-cli-merchant-001"

		// Insert merchant (simulating admin CLI)
		_, err := db.Exec(`
			INSERT INTO merchants (
				id, slug, name, cust_nbr, merch_nbr, dba_nbr,
				terminal_nbr, mac_secret_path, environment,
				is_active, status, tier
			) VALUES (
				gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
			)
			ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		`, merchantSlug, "Test CLI Merchant", "9001", "900300", "2", "77",
			"/secrets/test", "staging", true, "active", "standard")
		require.NoError(t, err)

		// Verify merchant was created with EPX credentials
		var custNbr, merchNbr, dbaNbr, terminalNbr string
		err = db.QueryRow(`
			SELECT cust_nbr, merch_nbr, dba_nbr, terminal_nbr
			FROM merchants WHERE slug = $1
		`, merchantSlug).Scan(&custNbr, &merchNbr, &dbaNbr, &terminalNbr)
		require.NoError(t, err)
		assert.Equal(t, "9001", custNbr)
		assert.Equal(t, "900300", merchNbr)
		assert.Equal(t, "2", dbaNbr)
		assert.Equal(t, "77", terminalNbr)

		// Verify NO API credentials were created (merchant_credentials table doesn't exist)
		// This is correct behavior - merchants don't get API keys
		var count int
		err = db.QueryRow(`
			SELECT COUNT(*) FROM information_schema.tables
			WHERE table_name = 'merchant_credentials'
		`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "merchant_credentials table should not exist")

		// Cleanup
		_, _ = db.Exec(`DELETE FROM merchants WHERE slug = $1`, merchantSlug)
	})
}

// TestAdminCLI_GrantAccess tests service-to-merchant access control:
// 1. Create service
// 2. Create merchant
// 3. Grant service access to merchant
// 4. Verify scopes are stored correctly
func TestAdminCLI_GrantAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup database
	ctx := context.Background()
	dbURL := getTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	t.Run("Grant service access to merchant", func(t *testing.T) {
		// Create test service
		serviceID := "test-cli-service-grant"
		privateKey, publicKey, err := auth.GenerateRSAKeyPair(2048)
		require.NoError(t, err)
		publicKeyPEM, _ := auth.PublicKeyToPEM(publicKey)

		var serviceUUID string
		err = db.QueryRow(`
			INSERT INTO services (
				id, service_id, service_name, public_key,
				public_key_fingerprint, environment
			) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
			ON CONFLICT (service_id) DO UPDATE SET service_id = EXCLUDED.service_id
			RETURNING id
		`, serviceID, "Test Grant Service", string(publicKeyPEM),
			"test-fp", "staging").Scan(&serviceUUID)
		require.NoError(t, err)

		// Create test merchant
		merchantSlug := "test-cli-merchant-grant"
		var merchantUUID string
		err = db.QueryRow(`
			INSERT INTO merchants (
				id, slug, name, cust_nbr, merch_nbr, dba_nbr,
				terminal_nbr, mac_secret_path, environment,
				is_active, status
			) VALUES (
				gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
			)
			ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
			RETURNING id
		`, merchantSlug, "Test Grant Merchant", "9001", "900300", "2", "77",
			"/secrets/test", "staging", true, "active").Scan(&merchantUUID)
		require.NoError(t, err)

		// Grant access (simulating admin CLI grant-access command)
		scopes := []string{
			"payment:create",
			"payment:read",
			"payment:update",
			"payment:refund",
			"subscription:manage",
			"payment_method:manage",
		}

		_, err = db.Exec(`
			INSERT INTO service_merchants (
				service_id, merchant_id, scopes
			) VALUES ($1, $2, $3)
			ON CONFLICT (service_id, merchant_id) DO UPDATE SET
				scopes = EXCLUDED.scopes
		`, serviceUUID, merchantUUID, scopes)
		require.NoError(t, err)

		// Verify access was granted
		var storedScopes []string
		err = db.QueryRow(`
			SELECT scopes FROM service_merchants
			WHERE service_id = $1 AND merchant_id = $2
		`, serviceUUID, merchantUUID).Scan(pq.Array(&storedScopes))
		require.NoError(t, err)
		assert.ElementsMatch(t, scopes, storedScopes)

		// Verify we can use the private key to sign JWTs
		privateKeyPEM := auth.PrivateKeyToPEM(privateKey)
		jwtManager, err := auth.NewJWTManager(privateKeyPEM, serviceID, 0)
		require.NoError(t, err)

		token, err := jwtManager.GenerateToken(merchantUUID, scopes)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Cleanup
		_, _ = db.Exec(`DELETE FROM service_merchants WHERE service_id = $1`, serviceUUID)
		_, _ = db.Exec(`DELETE FROM services WHERE service_id = $1`, serviceID)
		_, _ = db.Exec(`DELETE FROM merchants WHERE slug = $1`, merchantSlug)
	})
}

// TestAdminCLI_ArchitectureVerification verifies the correct architecture:
// - Merchants have EPX credentials only
// - Services have RSA keypairs only
// - NO merchant_credentials table exists
func TestAdminCLI_ArchitectureVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	dbURL := getTestDatabaseURL()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	t.Run("Verify merchants table schema", func(t *testing.T) {
		// Check merchants table has EPX fields
		var count int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name = 'merchants'
			AND column_name IN ('cust_nbr', 'merch_nbr', 'dba_nbr', 'terminal_nbr', 'mac_secret_path')
		`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 5, count, "Merchants should have 5 EPX credential fields")

		// Check merchants table does NOT have API key fields
		err = db.QueryRow(`
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name = 'merchants'
			AND column_name IN ('api_key', 'api_secret', 'api_key_hash')
		`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Merchants should NOT have API key fields")
	})

	t.Run("Verify services table schema", func(t *testing.T) {
		// Check services table has RSA public key fields
		var count int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name = 'services'
			AND column_name IN ('public_key', 'public_key_fingerprint')
		`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count, "Services should have public key fields")

		// Services should NOT have private key (stored client-side only)
		err = db.QueryRow(`
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name = 'services'
			AND column_name = 'private_key'
		`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Services should NOT store private keys in database")
	})

	t.Run("Verify merchant_credentials table does not exist", func(t *testing.T) {
		var count int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM information_schema.tables
			WHERE table_name = 'merchant_credentials'
		`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "merchant_credentials table should not exist - it's dead code")
	})

	t.Run("Verify service_merchants junction table exists", func(t *testing.T) {
		var count int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM information_schema.tables
			WHERE table_name = 'service_merchants'
		`).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "service_merchants table must exist for access control")
	})
}

func getTestDatabaseURL() string {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"
	}
	return dbURL
}
