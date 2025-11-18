package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// AuthContext keys for storing auth information
type contextKey string

const (
	AuthTypeKey     contextKey = "auth_type"
	ServiceIDKey    contextKey = "service_id"
	MerchantIDKey   contextKey = "merchant_id"
	MerchantCodeKey contextKey = "merchant_code"
	TokenJTIKey     contextKey = "token_jti"
	RequestIDKey    contextKey = "request_id"
)

// AuthInterceptor provides authentication for ConnectRPC services
type AuthInterceptor struct {
	db         *sql.DB
	publicKeys map[string]*rsa.PublicKey // service_id -> public key
	logger     *zap.Logger
	saltPrefix string // For consistent hashing
}

// NewAuthInterceptor creates a new authentication interceptor
func NewAuthInterceptor(db *sql.DB, logger *zap.Logger) (*AuthInterceptor, error) {
	ai := &AuthInterceptor{
		db:         db,
		publicKeys: make(map[string]*rsa.PublicKey),
		logger:     logger,
		saltPrefix: "payment_service_", // This should be from config in production
	}

	// Load public keys from database
	if err := ai.loadPublicKeys(); err != nil {
		return nil, fmt.Errorf("failed to load public keys: %w", err)
	}

	// Start periodic refresh of public keys
	go ai.startPublicKeyRefresh()

	return ai, nil
}

// loadPublicKeys loads all active service public keys from the database
func (ai *AuthInterceptor) loadPublicKeys() error {
	rows, err := ai.db.Query(`
		SELECT service_id, public_key
		FROM registered_services
		WHERE is_active = true
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	newKeys := make(map[string]*rsa.PublicKey)

	for rows.Next() {
		var serviceID, publicKeyPEM string
		if err := rows.Scan(&serviceID, &publicKeyPEM); err != nil {
			ai.logger.Error("Failed to scan service key",
				zap.String("service_id", serviceID),
				zap.Error(err))
			continue
		}

		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
		if err != nil {
			ai.logger.Error("Failed to parse public key",
				zap.String("service_id", serviceID),
				zap.Error(err))
			continue
		}

		newKeys[serviceID] = publicKey
	}

	ai.publicKeys = newKeys
	ai.logger.Info("Loaded public keys",
		zap.Int("count", len(ai.publicKeys)))

	return nil
}

// startPublicKeyRefresh periodically refreshes public keys
func (ai *AuthInterceptor) startPublicKeyRefresh() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := ai.loadPublicKeys(); err != nil {
			ai.logger.Error("Failed to refresh public keys", zap.Error(err))
		}
	}
}

// WrapUnary provides authentication for unary RPC calls
func (ai *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Skip auth for health checks
		procedure := req.Spec().Procedure
		if strings.HasSuffix(procedure, "/Health") ||
			strings.HasSuffix(procedure, "/Ready") ||
			strings.HasSuffix(procedure, "/Check") {
			return next(ctx, req)
		}

		// Add request ID to context
		requestID := req.Header().Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		ctx = context.WithValue(ctx, RequestIDKey, requestID)

		// Try JWT authentication first (for services)
		if authHeader := req.Header().Get("Authorization"); authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				return ai.authenticateJWT(ctx, req, next, authHeader)
			}
		}

		// Try API key authentication (for merchants)
		if apiKey := req.Header().Get("X-API-Key"); apiKey != "" {
			apiSecret := req.Header().Get("X-API-Secret")
			return ai.authenticateAPIKey(ctx, req, next, apiKey, apiSecret)
		}

		// Log failed auth attempt
		ai.logAuth(ctx, false, "missing authentication credentials", req.Spec().Procedure)

		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("missing authentication"))
	}
}

// WrapStreamingClient provides authentication for streaming client calls
func (ai *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		// For now, streaming follows the same pattern
		// Authentication happens at stream initialization
		return next(ctx, spec)
	}
}

// WrapStreamingHandler provides authentication for streaming handler calls
func (ai *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		// Skip auth for health checks
		if strings.HasSuffix(conn.Spec().Procedure, "/Health") ||
			strings.HasSuffix(conn.Spec().Procedure, "/Watch") {
			return next(ctx, conn)
		}

		// Extract headers from connection
		authHeader := conn.RequestHeader().Get("Authorization")
		apiKey := conn.RequestHeader().Get("X-API-Key")

		// Authenticate based on available credentials
		var authenticated bool
		var authErr error

		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			ctx, authErr = ai.authenticateJWTContext(ctx, authHeader)
			authenticated = authErr == nil
		} else if apiKey != "" {
			apiSecret := conn.RequestHeader().Get("X-API-Secret")
			ctx, authErr = ai.authenticateAPIKeyContext(ctx, apiKey, apiSecret)
			authenticated = authErr == nil
		}

		if !authenticated {
			if authErr == nil {
				authErr = fmt.Errorf("missing authentication")
			}
			return connect.NewError(connect.CodeUnauthenticated, authErr)
		}

		return next(ctx, conn)
	}
}

// authenticateJWT handles JWT token authentication
func (ai *AuthInterceptor) authenticateJWT(ctx context.Context, req connect.AnyRequest,
	next connect.UnaryFunc, authHeader string) (connect.AnyResponse, error) {

	ctx, err := ai.authenticateJWTContext(ctx, authHeader)
	if err != nil {
		ai.logger.Warn("JWT validation failed",
			zap.String("procedure", req.Spec().Procedure),
			zap.Error(err))
		ai.logAuth(ctx, false, err.Error(), req.Spec().Procedure)
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Apply rate limiting
	if err := ai.checkRateLimit(ctx); err != nil {
		ai.logAuth(ctx, false, "rate limit exceeded", req.Spec().Procedure)
		return nil, connect.NewError(connect.CodeResourceExhausted, err)
	}

	// Log successful auth
	ai.logAuth(ctx, true, "", req.Spec().Procedure)

	return next(ctx, req)
}

// authenticateJWTContext validates JWT and adds auth info to context
func (ai *AuthInterceptor) authenticateJWTContext(ctx context.Context, authHeader string) (context.Context, error) {
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Parse and validate token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Check signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get issuer from claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, fmt.Errorf("invalid claims")
		}

		issuer, ok := claims["iss"].(string)
		if !ok {
			return nil, fmt.Errorf("missing issuer")
		}

		// Look up public key for issuer
		publicKey, exists := ai.publicKeys[issuer]
		if !exists {
			return nil, fmt.Errorf("unknown issuer: %s", issuer)
		}

		return publicKey, nil
	})

	if err != nil {
		return ctx, err
	}

	if !token.Valid {
		return ctx, fmt.Errorf("invalid token")
	}

	claims := token.Claims.(jwt.MapClaims)

	// Check token expiration (should be handled by jwt.Parse but double-check)
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return ctx, fmt.Errorf("token expired")
		}
	}

	// Extract merchant ID from claims
	merchantID, ok := claims["merchant_id"].(string)
	if !ok {
		return ctx, fmt.Errorf("missing merchant_id in token")
	}

	// Verify service has access to this merchant
	issuer := claims["iss"].(string)
	if err := ai.verifyServiceMerchantAccess(issuer, merchantID); err != nil {
		return ctx, fmt.Errorf("access denied: %w", err)
	}

	// Check if token is blacklisted
	if jti, ok := claims["jti"].(string); ok {
		if ai.isTokenBlacklisted(jti) {
			return ctx, fmt.Errorf("token has been revoked")
		}
	}

	// Add auth context
	ctx = context.WithValue(ctx, AuthTypeKey, "jwt")
	ctx = context.WithValue(ctx, ServiceIDKey, issuer)
	ctx = context.WithValue(ctx, MerchantIDKey, merchantID)
	if jti, ok := claims["jti"].(string); ok {
		ctx = context.WithValue(ctx, TokenJTIKey, jti)
	}

	return ctx, nil
}

// authenticateAPIKey handles API key authentication
func (ai *AuthInterceptor) authenticateAPIKey(ctx context.Context, req connect.AnyRequest,
	next connect.UnaryFunc, apiKey, apiSecret string) (connect.AnyResponse, error) {

	ctx, err := ai.authenticateAPIKeyContext(ctx, apiKey, apiSecret)
	if err != nil {
		ai.logger.Warn("API key auth failed",
			zap.String("api_key_prefix", truncateAPIKey(apiKey)),
			zap.Error(err))
		ai.logAuth(ctx, false, err.Error(), req.Spec().Procedure)
		return nil, connect.NewError(connect.CodeUnauthenticated,
			fmt.Errorf("invalid API credentials"))
	}

	// Apply rate limiting
	if err := ai.checkRateLimit(ctx); err != nil {
		ai.logAuth(ctx, false, "rate limit exceeded", req.Spec().Procedure)
		return nil, connect.NewError(connect.CodeResourceExhausted, err)
	}

	// Log successful auth
	ai.logAuth(ctx, true, "", req.Spec().Procedure)

	return next(ctx, req)
}

// authenticateAPIKeyContext validates API key and adds auth info to context
func (ai *AuthInterceptor) authenticateAPIKeyContext(ctx context.Context, apiKey, apiSecret string) (context.Context, error) {
	// Validate input
	if apiKey == "" || apiSecret == "" {
		return ctx, fmt.Errorf("missing API credentials")
	}

	// Hash the credentials
	apiKeyHash := ai.hashWithSalt(apiKey)
	apiSecretHash := ai.hashWithSalt(apiSecret)

	// Look up merchant
	var merchantID string
	var merchantCode string
	err := ai.db.QueryRow(`
		SELECT mc.merchant_id, m.merchant_code
		FROM merchant_credentials mc
		JOIN merchants m ON mc.merchant_id = m.id
		WHERE mc.api_key_hash = $1
		AND mc.api_secret_hash = $2
		AND mc.is_active = true
		AND (mc.expires_at IS NULL OR mc.expires_at > NOW())
		AND m.status = 'active'
	`, apiKeyHash, apiSecretHash).Scan(&merchantID, &merchantCode)

	if err != nil {
		if err == sql.ErrNoRows {
			return ctx, fmt.Errorf("invalid API credentials")
		}
		return ctx, fmt.Errorf("authentication error: %w", err)
	}

	// Update last used timestamp asynchronously
	go func() {
		_, err := ai.db.Exec(`
			UPDATE merchant_credentials
			SET last_used_at = NOW()
			WHERE api_key_hash = $1
		`, apiKeyHash)
		if err != nil {
			ai.logger.Error("Failed to update last_used_at",
				zap.String("merchant_id", merchantID),
				zap.Error(err))
		}
	}()

	// Add auth context
	ctx = context.WithValue(ctx, AuthTypeKey, "api_key")
	ctx = context.WithValue(ctx, MerchantIDKey, merchantID)
	ctx = context.WithValue(ctx, MerchantCodeKey, merchantCode)

	return ctx, nil
}

// verifyServiceMerchantAccess checks if a service has access to a merchant
func (ai *AuthInterceptor) verifyServiceMerchantAccess(serviceID, merchantID string) error {
	var hasAccess bool
	err := ai.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM service_merchants sm
			JOIN registered_services s ON sm.service_id = s.id
			JOIN merchants m ON sm.merchant_id = m.id
			WHERE s.service_id = $1
			AND m.id = $2
			AND s.is_active = true
			AND m.status = 'active'
			AND (sm.expires_at IS NULL OR sm.expires_at > NOW())
		)
	`, serviceID, merchantID).Scan(&hasAccess)

	if err != nil {
		return fmt.Errorf("failed to verify access: %w", err)
	}

	if !hasAccess {
		return fmt.Errorf("service %s not authorized for merchant %s",
			serviceID, merchantID)
	}

	return nil
}

// isTokenBlacklisted checks if a JWT has been blacklisted
func (ai *AuthInterceptor) isTokenBlacklisted(jti string) bool {
	var exists bool
	err := ai.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM jwt_blacklist
			WHERE jti = $1
			AND expires_at > NOW()
		)
	`, jti).Scan(&exists)

	if err != nil {
		ai.logger.Error("Failed to check JWT blacklist",
			zap.String("jti", jti),
			zap.Error(err))
		return false // Fail open for availability
	}

	return exists
}

// checkRateLimit implements token bucket rate limiting
func (ai *AuthInterceptor) checkRateLimit(ctx context.Context) error {
	// Extract entity info from context
	authType, _ := ctx.Value(AuthTypeKey).(string)

	var entityType, entityID string
	var limit int

	if authType == "jwt" {
		entityType = "service"
		entityID, _ = ctx.Value(ServiceIDKey).(string)

		// Get service rate limit
		err := ai.db.QueryRow(`
			SELECT requests_per_second FROM registered_services
			WHERE service_id = $1
		`, entityID).Scan(&limit)
		if err != nil {
			limit = 100 // Default limit
		}
	} else {
		entityType = "merchant"
		entityID, _ = ctx.Value(MerchantIDKey).(string)

		// Get merchant rate limit
		err := ai.db.QueryRow(`
			SELECT requests_per_second FROM merchants
			WHERE id = $1
		`, entityID).Scan(&limit)
		if err != nil {
			limit = 50 // Default limit
		}
	}

	// Build bucket key (per-minute buckets)
	bucketKey := fmt.Sprintf("%s:%s:%s",
		entityType,
		entityID,
		time.Now().Format("2006-01-02-15:04"))

	// Token bucket algorithm with database storage
	var tokens int
	err := ai.db.QueryRow(`
		INSERT INTO rate_limit_buckets (bucket_key, tokens, last_refill)
		VALUES ($1, $2, NOW())
		ON CONFLICT (bucket_key) DO UPDATE
		SET tokens = GREATEST(rate_limit_buckets.tokens - 1, 0),
			last_refill = NOW()
		RETURNING tokens
	`, bucketKey, limit).Scan(&tokens)

	if err != nil {
		ai.logger.Error("Rate limit check failed",
			zap.String("bucket_key", bucketKey),
			zap.Error(err))
		return nil // Fail open for availability
	}

	if tokens <= 0 {
		return fmt.Errorf("rate limit exceeded for %s %s", entityType, entityID)
	}

	return nil
}

// logAuth logs authentication attempts to the audit log
func (ai *AuthInterceptor) logAuth(ctx context.Context, success bool, errorMsg string, procedure string) {
	// Extract context values
	authType, _ := ctx.Value(AuthTypeKey).(string)
	requestID, _ := ctx.Value(RequestIDKey).(string)

	var actorID, actorName string
	if authType == "jwt" {
		actorID, _ = ctx.Value(ServiceIDKey).(string)
		actorName = fmt.Sprintf("service:%s", actorID)
	} else if authType == "api_key" {
		actorID, _ = ctx.Value(MerchantIDKey).(string)
		merchantCode, _ := ctx.Value(MerchantCodeKey).(string)
		actorName = fmt.Sprintf("merchant:%s", merchantCode)
	}

	// Log to audit table asynchronously
	go func() {
		metadata := map[string]interface{}{
			"procedure":  procedure,
			"request_id": requestID,
		}

		metadataJSON, _ := json.Marshal(metadata)

		_, err := ai.db.Exec(`
			INSERT INTO audit_log (
				actor_type, actor_id, actor_name, action,
				metadata, success, error_message,
				ip_address, request_id, performed_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		`, authType, actorID, actorName, "auth.attempt",
			metadataJSON, success, errorMsg,
			getClientIPFromContext(ctx), requestID)

		if err != nil {
			ai.logger.Error("Failed to log auth attempt",
				zap.String("actor_id", actorID),
				zap.Error(err))
		}
	}()
}

// hashWithSalt creates a salted hash of the input
func (ai *AuthInterceptor) hashWithSalt(input string) string {
	h := sha256.New()
	h.Write([]byte(ai.saltPrefix + input))
	return hex.EncodeToString(h.Sum(nil))
}

// Helper functions

func truncateAPIKey(key string) string {
	if len(key) > 10 {
		return key[:10] + "..."
	}
	return key
}

func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63())
}

func getClientIPFromContext(ctx context.Context) string {
	// This would need to be set by a previous interceptor or extracted from headers
	// For now, return empty string
	return ""
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}