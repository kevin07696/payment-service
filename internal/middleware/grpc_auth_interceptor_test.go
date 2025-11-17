package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Test helper to create test key pair
func createTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privateKey, &privateKey.PublicKey
}

// Test helper to create test token
func createTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims *domain.TokenClaims) string {
	t.Helper()

	// Set default registered claims if not provided
	if claims.ExpiresAt == nil {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(1 * time.Hour))
	}
	if claims.IssuedAt == nil {
		claims.IssuedAt = jwt.NewNumericDate(time.Now())
	}
	if claims.NotBefore == nil {
		claims.NotBefore = jwt.NewNumericDate(time.Now())
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)
	return tokenString
}

// Test helper to create interceptor with test key
func setupTestInterceptor(t *testing.T) (*GRPCAuthInterceptor, *rsa.PrivateKey) {
	t.Helper()

	privateKey, publicKey := createTestKeyPair(t)

	keyStore := auth.NewPublicKeyStore()
	keyStore.AddKey("test-issuer", publicKey)

	logger := zap.NewNop()
	interceptor := NewGRPCAuthInterceptor(keyStore, logger)

	return interceptor, privateKey
}

// TestUnaryServerInterceptor_Success tests successful authentication
func TestUnaryServerInterceptor_Success(t *testing.T) {
	interceptor, privateKey := setupTestInterceptor(t)

	// Create valid merchant token
	customerID := "customer-123"
	claims := &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  "test-issuer",
			Subject: "merchant-app",
		},
		TokenType:   domain.TokenTypeMerchant,
		MerchantIDs: []string{"merchant-1", "merchant-2"},
		Scopes:      []string{"payments:create", "payments:read"},
		CustomerID:  &customerID,
	}

	tokenString := createTestToken(t, privateKey, claims)

	// Create context with authorization header
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + tokenString,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// Mock handler
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true

		// Verify claims are in context
		contextClaims, err := GetTokenFromContext(ctx)
		require.NoError(t, err)
		assert.Equal(t, claims.Issuer, contextClaims.Issuer)
		assert.Equal(t, claims.TokenType, contextClaims.TokenType)
		assert.Equal(t, claims.MerchantIDs, contextClaims.MerchantIDs)

		return "success", nil
	}

	// Execute interceptor
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	result, err := interceptor.UnaryServerInterceptor()(ctx, nil, info, handler)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.True(t, handlerCalled)
}

// TestUnaryServerInterceptor_MissingMetadata tests missing metadata
func TestUnaryServerInterceptor_MissingMetadata(t *testing.T) {
	interceptor, _ := setupTestInterceptor(t)

	ctx := context.Background() // No metadata

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("Handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	result, err := interceptor.UnaryServerInterceptor()(ctx, nil, info, handler)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "missing metadata")
}

// TestUnaryServerInterceptor_MissingAuthHeader tests missing authorization header
func TestUnaryServerInterceptor_MissingAuthHeader(t *testing.T) {
	interceptor, _ := setupTestInterceptor(t)

	md := metadata.New(map[string]string{})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("Handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	result, err := interceptor.UnaryServerInterceptor()(ctx, nil, info, handler)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "missing authorization header")
}

// TestUnaryServerInterceptor_InvalidAuthFormat tests invalid auth format
func TestUnaryServerInterceptor_InvalidAuthFormat(t *testing.T) {
	interceptor, _ := setupTestInterceptor(t)

	md := metadata.New(map[string]string{
		"authorization": "InvalidFormat token-here",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("Handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	result, err := interceptor.UnaryServerInterceptor()(ctx, nil, info, handler)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "invalid authorization format")
}

// TestUnaryServerInterceptor_InvalidToken tests invalid token
func TestUnaryServerInterceptor_InvalidToken(t *testing.T) {
	interceptor, _ := setupTestInterceptor(t)

	md := metadata.New(map[string]string{
		"authorization": "Bearer invalid.token.here",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("Handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	result, err := interceptor.UnaryServerInterceptor()(ctx, nil, info, handler)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "invalid token")
}

// TestUnaryServerInterceptor_ExpiredToken tests expired token
func TestUnaryServerInterceptor_ExpiredToken(t *testing.T) {
	interceptor, privateKey := setupTestInterceptor(t)

	// Create expired token
	claims := &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "test-user",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Expired
		},
		TokenType:   domain.TokenTypeMerchant,
		MerchantIDs: []string{"merchant-1"},
		Scopes:      []string{"payments:read"},
	}

	tokenString := createTestToken(t, privateKey, claims)

	md := metadata.New(map[string]string{
		"authorization": "Bearer " + tokenString,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("Handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	result, err := interceptor.UnaryServerInterceptor()(ctx, nil, info, handler)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestUnaryServerInterceptor_UnknownIssuer tests unknown issuer
func TestUnaryServerInterceptor_UnknownIssuer(t *testing.T) {
	interceptor, _ := setupTestInterceptor(t)

	// Create key pair for unknown issuer
	unknownPrivateKey, _ := createTestKeyPair(t)

	claims := &domain.TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  "unknown-issuer",
			Subject: "test-user",
		},
		TokenType:   domain.TokenTypeMerchant,
		MerchantIDs: []string{"merchant-1"},
		Scopes:      []string{"payments:read"},
	}

	tokenString := createTestToken(t, unknownPrivateKey, claims)

	md := metadata.New(map[string]string{
		"authorization": "Bearer " + tokenString,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("Handler should not be called")
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
	result, err := interceptor.UnaryServerInterceptor()(ctx, nil, info, handler)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "unknown issuer")
}

// TestValidateClaims_MerchantToken tests merchant token validation
func TestValidateClaims_MerchantToken(t *testing.T) {
	interceptor, _ := setupTestInterceptor(t)

	tests := []struct {
		name    string
		claims  *domain.TokenClaims
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid merchant token",
			claims: &domain.TokenClaims{
				TokenType:   domain.TokenTypeMerchant,
				MerchantIDs: []string{"merchant-1"},
				Scopes:      []string{"payments:read"},
			},
			wantErr: false,
		},
		{
			name: "merchant token without merchant_ids",
			claims: &domain.TokenClaims{
				TokenType:   domain.TokenTypeMerchant,
				MerchantIDs: []string{},
				Scopes:      []string{"payments:read"},
			},
			wantErr: true,
			errMsg:  "at least one merchant_id",
		},
		{
			name: "merchant token without scopes",
			claims: &domain.TokenClaims{
				TokenType:   domain.TokenTypeMerchant,
				MerchantIDs: []string{"merchant-1"},
				Scopes:      []string{},
			},
			wantErr: true,
			errMsg:  "at least one scope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := interceptor.validateClaims(tt.claims)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateClaims_CustomerToken tests customer token validation
func TestValidateClaims_CustomerToken(t *testing.T) {
	interceptor, _ := setupTestInterceptor(t)

	customerID := "customer-123"
	emptyCustomerID := ""

	tests := []struct {
		name    string
		claims  *domain.TokenClaims
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid customer token",
			claims: &domain.TokenClaims{
				TokenType:  domain.TokenTypeCustomer,
				CustomerID: &customerID,
				Scopes:     []string{"payments:create"},
			},
			wantErr: false,
		},
		{
			name: "customer token without customer_id",
			claims: &domain.TokenClaims{
				TokenType:  domain.TokenTypeCustomer,
				CustomerID: nil,
				Scopes:     []string{"payments:create"},
			},
			wantErr: true,
			errMsg:  "must have customer_id",
		},
		{
			name: "customer token with empty customer_id",
			claims: &domain.TokenClaims{
				TokenType:  domain.TokenTypeCustomer,
				CustomerID: &emptyCustomerID,
				Scopes:     []string{"payments:create"},
			},
			wantErr: true,
			errMsg:  "must have customer_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := interceptor.validateClaims(tt.claims)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateClaims_GuestToken tests guest token validation
func TestValidateClaims_GuestToken(t *testing.T) {
	interceptor, _ := setupTestInterceptor(t)

	sessionID := "session-123"
	emptySessionID := ""

	tests := []struct {
		name    string
		claims  *domain.TokenClaims
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid guest token",
			claims: &domain.TokenClaims{
				TokenType:   domain.TokenTypeGuest,
				SessionID:   &sessionID,
				MerchantIDs: []string{"merchant-1"},
				Scopes:      []string{"payments:create"},
			},
			wantErr: false,
		},
		{
			name: "guest token without session_id",
			claims: &domain.TokenClaims{
				TokenType:   domain.TokenTypeGuest,
				SessionID:   nil,
				MerchantIDs: []string{"merchant-1"},
				Scopes:      []string{"payments:create"},
			},
			wantErr: true,
			errMsg:  "must have session_id",
		},
		{
			name: "guest token with empty session_id",
			claims: &domain.TokenClaims{
				TokenType:   domain.TokenTypeGuest,
				SessionID:   &emptySessionID,
				MerchantIDs: []string{"merchant-1"},
				Scopes:      []string{"payments:create"},
			},
			wantErr: true,
			errMsg:  "must have session_id",
		},
		{
			name: "guest token with multiple merchants",
			claims: &domain.TokenClaims{
				TokenType:   domain.TokenTypeGuest,
				SessionID:   &sessionID,
				MerchantIDs: []string{"merchant-1", "merchant-2"},
				Scopes:      []string{"payments:create"},
			},
			wantErr: true,
			errMsg:  "exactly one merchant_id",
		},
		{
			name: "guest token without merchant",
			claims: &domain.TokenClaims{
				TokenType:   domain.TokenTypeGuest,
				SessionID:   &sessionID,
				MerchantIDs: []string{},
				Scopes:      []string{"payments:create"},
			},
			wantErr: true,
			errMsg:  "exactly one merchant_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := interceptor.validateClaims(tt.claims)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateClaims_InvalidTokenType tests invalid token type
func TestValidateClaims_InvalidTokenType(t *testing.T) {
	interceptor, _ := setupTestInterceptor(t)

	claims := &domain.TokenClaims{
		TokenType: "invalid-type",
		Scopes:    []string{"payments:read"},
	}

	err := interceptor.validateClaims(claims)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token_type")
}

// TestGetTokenFromContext tests retrieving token from context
func TestGetTokenFromContext(t *testing.T) {
	// Create test claims
	claims := &domain.TokenClaims{
		TokenType:   domain.TokenTypeMerchant,
		MerchantIDs: []string{"merchant-1"},
	}

	// Store in context
	ctx := context.WithValue(context.Background(), tokenClaimsKey, claims)

	// Retrieve from context
	retrievedClaims, err := GetTokenFromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, claims, retrievedClaims)
}

// TestGetTokenFromContext_NoToken tests missing token in context
func TestGetTokenFromContext_NoToken(t *testing.T) {
	ctx := context.Background()

	claims, err := GetTokenFromContext(ctx)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "no token claims in context")
}

// TestMustGetTokenFromContext tests MustGetTokenFromContext
func TestMustGetTokenFromContext(t *testing.T) {
	// Create test claims
	claims := &domain.TokenClaims{
		TokenType:   domain.TokenTypeMerchant,
		MerchantIDs: []string{"merchant-1"},
	}

	// Store in context
	ctx := context.WithValue(context.Background(), tokenClaimsKey, claims)

	// Should not panic
	retrievedClaims := MustGetTokenFromContext(ctx)
	assert.Equal(t, claims, retrievedClaims)
}

// TestMustGetTokenFromContext_Panic tests panic on missing token
func TestMustGetTokenFromContext_Panic(t *testing.T) {
	ctx := context.Background()

	assert.Panics(t, func() {
		MustGetTokenFromContext(ctx)
	})
}
