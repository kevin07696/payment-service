package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/domain"
)

type contextKey string

const tokenClaimsKey contextKey = "token_claims"

// GRPCAuthInterceptor handles JWT authentication and authorization for gRPC
type GRPCAuthInterceptor struct {
	keyStore *auth.PublicKeyStore
	logger   *zap.Logger
}

// NewGRPCAuthInterceptor creates a new gRPC auth interceptor
func NewGRPCAuthInterceptor(keyStore *auth.PublicKeyStore, logger *zap.Logger) *GRPCAuthInterceptor {
	return &GRPCAuthInterceptor{
		keyStore: keyStore,
		logger:   logger,
	}
}

// UnaryServerInterceptor returns a gRPC unary server interceptor for auth
func (i *GRPCAuthInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract metadata from context
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		// Extract Authorization header
		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		authHeader := authHeaders[0]

		// Parse Bearer token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format: expected 'Bearer <token>'")
		}

		// Parse and verify token
		claims, err := i.verifyToken(tokenString)
		if err != nil {
			i.logger.Warn("token verification failed",
				zap.Error(err),
				zap.String("method", info.FullMethod))
			return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("invalid token: %v", err))
		}

		// Validate claims
		if err := i.validateClaims(claims); err != nil {
			i.logger.Warn("token claims validation failed",
				zap.Error(err),
				zap.String("method", info.FullMethod),
				zap.String("token_type", claims.TokenType))
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		// Store claims in context
		ctx = context.WithValue(ctx, tokenClaimsKey, claims)

		// Log successful authentication
		i.logger.Debug("token authenticated",
			zap.String("subject", claims.Subject),
			zap.String("issuer", claims.Issuer),
			zap.String("token_type", claims.TokenType),
			zap.Strings("merchant_ids", claims.MerchantIDs))

		// Call next handler
		return handler(ctx, req)
	}
}

// verifyToken parses and verifies a JWT token
func (i *GRPCAuthInterceptor) verifyToken(tokenString string) (*domain.TokenClaims, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &domain.TokenClaims{},
		func(token *jwt.Token) (interface{}, error) {
			// Verify signing method
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			// Get claims to extract issuer
			claims, ok := token.Claims.(*domain.TokenClaims)
			if !ok {
				return nil, errors.New("invalid token claims")
			}

			// Get public key for this issuer
			publicKey, err := i.keyStore.GetPublicKey(claims.Issuer)
			if err != nil {
				return nil, fmt.Errorf("unknown issuer '%s': %w", claims.Issuer, err)
			}

			return publicKey, nil
		})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("token is not valid")
	}

	claims, ok := token.Claims.(*domain.TokenClaims)
	if !ok {
		return nil, errors.New("invalid token claims type")
	}

	return claims, nil
}

// validateClaims validates token claims
func (i *GRPCAuthInterceptor) validateClaims(claims *domain.TokenClaims) error {
	// Check token type
	switch claims.TokenType {
	case domain.TokenTypeMerchant, domain.TokenTypeCustomer, domain.TokenTypeGuest, domain.TokenTypeAdmin:
		// Valid token types
	default:
		return fmt.Errorf("invalid token_type: %s", claims.TokenType)
	}

	// Validate merchant token
	if claims.TokenType == domain.TokenTypeMerchant {
		if len(claims.MerchantIDs) == 0 {
			return errors.New("merchant token must have at least one merchant_id")
		}
	}

	// Validate customer token
	if claims.TokenType == domain.TokenTypeCustomer {
		if claims.CustomerID == nil || *claims.CustomerID == "" {
			return errors.New("customer token must have customer_id")
		}
	}

	// Validate guest token
	if claims.TokenType == domain.TokenTypeGuest {
		if claims.SessionID == nil || *claims.SessionID == "" {
			return errors.New("guest token must have session_id")
		}
		if len(claims.MerchantIDs) != 1 {
			return errors.New("guest token must have exactly one merchant_id")
		}
	}

	// Check scopes
	if len(claims.Scopes) == 0 {
		return errors.New("token must have at least one scope")
	}

	return nil
}

// GetTokenFromContext retrieves token claims from context
func GetTokenFromContext(ctx context.Context) (*domain.TokenClaims, error) {
	claims, ok := ctx.Value(tokenClaimsKey).(*domain.TokenClaims)
	if !ok {
		return nil, errors.New("no token claims in context")
	}
	return claims, nil
}

// MustGetTokenFromContext retrieves token claims from context or panics
func MustGetTokenFromContext(ctx context.Context) *domain.TokenClaims {
	claims, err := GetTokenFromContext(ctx)
	if err != nil {
		panic(err)
	}
	return claims
}
