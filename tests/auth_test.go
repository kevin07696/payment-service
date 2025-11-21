//go:build integration
// +build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAuthenticationFlow(t *testing.T) {
	// Skip if not integration test
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup database connection
	ctx := context.Background()
	dbURL := "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	t.Run("JWT Token Generation and Validation", func(t *testing.T) {
		// Generate RSA key pair
		privateKey, _, err := auth.GenerateRSAKeyPair(2048)
		require.NoError(t, err)

		// Create JWT manager
		privateKeyPEM := auth.PrivateKeyToPEM(privateKey)
		jwtManager, err := auth.NewJWTManager(privateKeyPEM, "test-service", 5*time.Minute)
		require.NoError(t, err)

		// Generate token
		merchantID := "test-merchant-123"
		scopes := []string{"payment:create", "payment:read"}
		token, err := jwtManager.GenerateToken(merchantID, scopes)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate token
		claims, err := jwtManager.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, merchantID, claims.MerchantID)
		assert.Equal(t, "test-service", claims.ServiceID)
		assert.Equal(t, scopes, claims.Scopes)
	})

	// NOTE: API Key authentication has been removed.
	// The system now uses RSA keypair-based JWT authentication for services.
	// Merchants store EPX credentials only, not API keys.
	// See migration 008_auth_tables.sql for the correct architecture.
	t.Run("Service RSA Authentication", func(t *testing.T) {
		t.Skip("TODO: Add service authentication tests using RSA keypairs")
	})

	t.Run("Auth Context Operations", func(t *testing.T) {
		ctx := context.Background()

		// Create auth info
		authInfo := &auth.AuthInfo{
			Type:         auth.AuthTypeAPIKey,
			MerchantID:   "merchant-123",
			MerchantCode: "MERCH001",
			RequestID:    "req-456",
			Environment:  "production",
		}

		// Add to context
		ctx = auth.WithAuth(ctx, authInfo)

		// Retrieve auth info
		retrieved := auth.GetAuthInfo(ctx)
		assert.Equal(t, auth.AuthTypeAPIKey, retrieved.Type)
		assert.Equal(t, "merchant-123", retrieved.MerchantID)
		assert.Equal(t, "MERCH001", retrieved.MerchantCode)

		// Check authentication
		assert.True(t, auth.IsAuthenticated(ctx))

		// Get merchant ID
		merchantID, err := auth.GetMerchantID(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "merchant-123", merchantID)

		// Test unauthenticated context
		emptyCtx := context.Background()
		assert.False(t, auth.IsAuthenticated(emptyCtx))
		_, err = auth.GetMerchantID(emptyCtx)
		assert.Error(t, err)
	})
}

func TestAuthInterceptor(t *testing.T) {
	// Skip if not integration test
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup database connection
	ctx := context.Background()
	dbURL := "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	logger := zap.NewNop()

	t.Run("Auth Interceptor Initialization", func(t *testing.T) {
		// Create auth interceptor
		authInterceptor, err := middleware.NewAuthInterceptor(sqlDB, logger)
		require.NoError(t, err)
		assert.NotNil(t, authInterceptor)

		// The interceptor should be ready to use
		// In a real scenario, we would test it with actual ConnectRPC handlers
	})

	t.Run("EPX Callback Auth", func(t *testing.T) {
		// Create EPX callback auth
		epxAuth, err := middleware.NewEPXCallbackAuth(sqlDB, "test-secret", logger)
		require.NoError(t, err)
		assert.NotNil(t, epxAuth)

		// Test IP whitelist loading
		err = epxAuth.RefreshIPWhitelist()
		assert.NoError(t, err)
	})
}
