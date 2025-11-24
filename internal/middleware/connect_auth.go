package middleware

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/pkg/timeutil"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// tokenBucket represents an in-memory rate limit bucket
type tokenBucket struct {
	tokens    int32
	timestamp time.Time
	mu        sync.Mutex
}

// AuthInterceptor provides authentication for ConnectRPC services
type AuthInterceptor struct {
	queries         sqlc.Querier
	publicKeys      map[string]*rsa.PublicKey // service_id -> public key
	logger          *zap.Logger
	stopCh          chan struct{}             // Channel to signal goroutine shutdown
	rateLimitCB     *gobreaker.CircuitBreaker // Circuit breaker for rate limit DB calls
	memoryBuckets   sync.Map                  // In-memory fallback for rate limiting (bucket_key -> *tokenBucket)
	bucketCleanupCh chan struct{}             // Channel to signal bucket cleanup goroutine shutdown
}

// NewAuthInterceptor creates a new authentication interceptor
func NewAuthInterceptor(queries sqlc.Querier, logger *zap.Logger) (*AuthInterceptor, error) {
	// Configure circuit breaker for rate limit DB calls
	// After 5 consecutive failures, open circuit for 30 seconds
	cbSettings := gobreaker.Settings{
		Name:        "RateLimitDB",
		MaxRequests: 3,                // Allow 3 requests in half-open state
		Interval:    time.Minute,      // Reset failure count every minute
		Timeout:     30 * time.Second, // Stay open for 30 seconds before trying again
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Open circuit after 5 consecutive failures
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Warn("Rate limit circuit breaker state changed",
				zap.String("from", from.String()),
				zap.String("to", to.String()))
		},
	}

	ai := &AuthInterceptor{
		queries:         queries,
		publicKeys:      make(map[string]*rsa.PublicKey),
		logger:          logger,
		stopCh:          make(chan struct{}),
		rateLimitCB:     gobreaker.NewCircuitBreaker(cbSettings),
		bucketCleanupCh: make(chan struct{}),
	}

	// Load public keys from database
	if err := ai.loadPublicKeys(); err != nil {
		return nil, fmt.Errorf("failed to load public keys: %w", err)
	}

	// Start periodic refresh of public keys
	go ai.startPublicKeyRefresh()

	// Start periodic cleanup of old in-memory buckets
	go ai.startBucketCleanup()

	return ai, nil
}

// loadPublicKeys loads all active service public keys from the database
func (ai *AuthInterceptor) loadPublicKeys() error {
	ctx := context.Background()
	keys, err := ai.queries.ListActiveServicePublicKeys(ctx)
	if err != nil {
		return err
	}

	newKeys := make(map[string]*rsa.PublicKey)

	for _, key := range keys {
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(key.PublicKey))
		if err != nil {
			ai.logger.Error("Failed to parse public key",
				zap.String("service_id", key.ServiceID),
				zap.Error(err))
			continue
		}

		newKeys[key.ServiceID] = publicKey
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

	for {
		select {
		case <-ticker.C:
			if err := ai.loadPublicKeys(); err != nil {
				ai.logger.Error("Failed to refresh public keys", zap.Error(err))
			}
		case <-ai.stopCh:
			ai.logger.Info("Stopping public key refresh goroutine")
			return
		}
	}
}

// startBucketCleanup periodically removes expired in-memory rate limit buckets
func (ai *AuthInterceptor) startBucketCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := timeutil.Now()
			ai.memoryBuckets.Range(func(key, value interface{}) bool {
				bucket := value.(*tokenBucket)
				bucket.mu.Lock()
				// Remove buckets older than 2 minutes
				if now.Sub(bucket.timestamp) > 2*time.Minute {
					ai.memoryBuckets.Delete(key)
				}
				bucket.mu.Unlock()
				return true
			})
		case <-ai.bucketCleanupCh:
			ai.logger.Info("Stopping bucket cleanup goroutine")
			return
		}
	}
}

// Shutdown gracefully stops background goroutines
func (ai *AuthInterceptor) Shutdown() {
	close(ai.stopCh)
	close(ai.bucketCleanupCh)
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
		ctx = context.WithValue(ctx, auth.RequestIDKey, requestID)

		// Extract client IP from headers
		clientIP := extractClientIP(req.Header())
		if clientIP != "" {
			ctx = context.WithValue(ctx, auth.ClientIPKey, clientIP)
		}

		// Extract User-Agent
		userAgent := req.Header().Get("User-Agent")
		if userAgent != "" {
			ctx = context.WithValue(ctx, auth.UserAgentKey, userAgent)
		}

		// JWT authentication (for services only)
		if authHeader := req.Header().Get("Authorization"); authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				return ai.authenticateJWT(ctx, req, next, authHeader)
			}
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

		// Extract Authorization header from connection
		authHeader := conn.RequestHeader().Get("Authorization")

		// JWT authentication only (for services)
		var authErr error
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			ctx, authErr = ai.authenticateJWTContext(ctx, authHeader)
			if authErr == nil {
				return next(ctx, conn)
			}
		} else {
			authErr = fmt.Errorf("missing authentication")
		}

		return connect.NewError(connect.CodeUnauthenticated, authErr)
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
		if timeutil.Now().Unix() > int64(exp) {
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
	ctx = context.WithValue(ctx, auth.AuthTypeKey, "jwt")
	ctx = context.WithValue(ctx, auth.ServiceIDKey, issuer)
	ctx = context.WithValue(ctx, auth.MerchantIDKey, merchantID)
	if jti, ok := claims["jti"].(string); ok {
		ctx = context.WithValue(ctx, auth.TokenJTIKey, jti)
	}

	return ctx, nil
}

// verifyServiceMerchantAccess checks if a service has access to a merchant
func (ai *AuthInterceptor) verifyServiceMerchantAccess(serviceID, merchantID string) error {
	ctx := context.Background()

	// Parse merchant UUID
	merchantUUID, err := uuid.Parse(merchantID)
	if err != nil {
		return fmt.Errorf("invalid merchant ID: %w", err)
	}

	hasAccess, err := ai.queries.CheckServiceMerchantAccessByID(ctx, sqlc.CheckServiceMerchantAccessByIDParams{
		ServiceID:  serviceID,
		MerchantID: merchantUUID,
	})

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
	ctx := context.Background()
	isBlacklisted, err := ai.queries.IsJWTBlacklisted(ctx, jti)

	if err != nil {
		// SECURITY: Fail closed - treat as blacklisted if we can't verify
		// This prevents revoked tokens from being accepted during DB outages
		ai.logger.Error("Failed to check JWT blacklist - treating as blacklisted for security",
			zap.String("jti", jti),
			zap.Error(err))
		return true // Fail closed for security
	}

	return isBlacklisted
}

// checkRateLimit implements token bucket rate limiting with circuit breaker and in-memory fallback
func (ai *AuthInterceptor) checkRateLimit(ctx context.Context) error {
	// Extract service info from context (JWT auth only)
	entityType := "service"
	entityID, _ := ctx.Value(auth.ServiceIDKey).(string)

	// Get service rate limit using sqlc
	rateLimit, err := ai.queries.GetServiceRateLimit(ctx, entityID)
	limit := 100 // Default limit
	if err == nil && rateLimit.Valid {
		limit = int(rateLimit.Int32)
	}

	// Build bucket key (per-minute buckets)
	bucketKey := fmt.Sprintf("%s:%s:%s",
		entityType,
		entityID,
		timeutil.Now().Format("2006-01-02-15:04"))

	// Try database rate limiting through circuit breaker
	result, err := ai.rateLimitCB.Execute(func() (interface{}, error) {
		tokens, err := ai.queries.ConsumeRateLimitToken(ctx, sqlc.ConsumeRateLimitTokenParams{
			BucketKey:     bucketKey,
			InitialTokens: int32(limit),
		})
		return tokens, err
	})

	// If circuit breaker succeeded, use database result
	if err == nil {
		tokens := result.(int32)
		if tokens <= 0 {
			return fmt.Errorf("rate limit exceeded for %s %s", entityType, entityID)
		}
		return nil
	}

	// Circuit is open or database failed - fall back to in-memory rate limiting
	ai.logger.Warn("Using in-memory rate limiting fallback",
		zap.String("bucket_key", bucketKey),
		zap.String("entity_type", entityType),
		zap.String("entity_id", entityID),
		zap.Error(err))

	return ai.checkMemoryRateLimit(bucketKey, int32(limit))
}

// checkMemoryRateLimit implements in-memory token bucket rate limiting
func (ai *AuthInterceptor) checkMemoryRateLimit(bucketKey string, limit int32) error {
	now := timeutil.Now()

	// Get or create bucket
	value, loaded := ai.memoryBuckets.LoadOrStore(bucketKey, &tokenBucket{
		tokens:    limit,
		timestamp: now,
	})

	bucket := value.(*tokenBucket)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// If this is a fresh bucket or minute has changed, reset tokens
	if !loaded || bucket.timestamp.Format("2006-01-02-15:04") != now.Format("2006-01-02-15:04") {
		bucket.tokens = limit
		bucket.timestamp = now
	}

	// Try to consume a token
	if bucket.tokens <= 0 {
		return fmt.Errorf("rate limit exceeded (in-memory)")
	}

	bucket.tokens--
	return nil
}

// logAuth logs authentication attempts to database audit_log table
func (ai *AuthInterceptor) logAuth(ctx context.Context, success bool, errorMsg string, procedure string) {
	// Extract context values (JWT auth only)
	authType, _ := ctx.Value(auth.AuthTypeKey).(string)
	requestID, _ := ctx.Value(auth.RequestIDKey).(string)

	// Extract service info
	actorID, _ := ctx.Value(auth.ServiceIDKey).(string)

	// Parse IP address from context
	ipStr := auth.GetClientIP(ctx)
	var ipAddr *netip.Addr
	if ipStr != "" {
		if parsed, err := netip.ParseAddr(ipStr); err == nil {
			ipAddr = &parsed
		}
	}

	// Build audit log params
	params := sqlc.CreateAuditLogParams{
		ID:     pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Action: "authenticate",
		Success: pgtype.Bool{
			Bool:  success,
			Valid: true,
		},
	}

	// Set actor info if available
	if actorID != "" {
		params.ActorType = pgtype.Text{String: authType, Valid: true}
		params.ActorID = pgtype.Text{String: actorID, Valid: true}
	}

	// Set request context
	if requestID != "" {
		params.RequestID = pgtype.Text{String: requestID, Valid: true}
	}
	if ipAddr != nil {
		params.IpAddress = ipAddr
	}

	// Set error if failed
	if !success && errorMsg != "" {
		params.ErrorMessage = pgtype.Text{String: errorMsg, Valid: true}
	}

	// Set metadata with procedure info
	if procedure != "" {
		metadataJSON := fmt.Sprintf(`{"procedure":"%s"}`, procedure)
		params.Metadata = []byte(metadataJSON)
	}

	// Write to database (async to avoid blocking the request)
	go func() {
		insertCtx := context.Background()
		if err := ai.queries.CreateAuditLog(insertCtx, params); err != nil {
			ai.logger.Error("Failed to write audit log to database",
				zap.String("actor_id", actorID),
				zap.Error(err))
		}
	}()

	// Also log to structured logger as backup
	if success {
		ai.logger.Info("Auth attempt succeeded",
			zap.String("actor_id", actorID),
			zap.String("auth_type", authType),
			zap.String("procedure", procedure),
			zap.String("request_id", requestID),
			zap.String("ip_address", ipStr))
	} else {
		ai.logger.Warn("Auth attempt failed",
			zap.String("actor_id", actorID),
			zap.String("auth_type", authType),
			zap.String("procedure", procedure),
			zap.String("request_id", requestID),
			zap.String("error", errorMsg),
			zap.String("ip_address", ipStr))
	}
}

// Helper functions

func generateRequestID() string {
	// Use cryptographically secure UUID v4 for request IDs
	// This prevents collisions and provides non-predictable request tracking
	return uuid.New().String()
}

// extractClientIP extracts client IP from HTTP headers (for ConnectRPC)
func extractClientIP(headers http.Header) string {
	// Check X-Forwarded-For header first (standard proxy header)
	xff := headers.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first (client)
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP (nginx proxy header)
	xri := headers.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// ConnectRPC doesn't provide RemoteAddr in headers
	// If neither proxy header is present, IP extraction may require
	// additional middleware at the HTTP layer
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
