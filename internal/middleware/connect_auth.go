package middleware

import (
	"context"
	"crypto/rsa"
	"fmt"
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

// NOTE on authentication model:
// - Token identifies the SERVICE (via "iss" claim)
// - Request body specifies the MERCHANT (via merchant_id field)
// - Authorization service validates service has access to the merchant per-request
// - This allows one token to work for multiple merchants

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
	keysMu          sync.RWMutex              // Mutex for thread-safe public key access
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
	// Use timeout context for database query (5 seconds should be sufficient for key lookup)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	keys, err := ai.queries.ListActiveServicePublicKeys(ctx)
	if err != nil {
		return fmt.Errorf("listing active service public keys: %w", err)
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

	// Thread-safe replacement of public keys map
	ai.keysMu.Lock()
	ai.publicKeys = newKeys
	ai.keysMu.Unlock()

	ai.logger.Info("Loaded public keys",
		zap.Int("count", len(newKeys)))

	return nil
}

// getPublicKey retrieves a public key for the given issuer (service_id).
// It first checks the in-memory cache, then falls back to database lookup.
// This enables newly created services to be authenticated immediately without
// waiting for the periodic cache refresh.
func (ai *AuthInterceptor) getPublicKey(ctx context.Context, issuer string) (*rsa.PublicKey, error) {
	// Fast path: check cache with read lock
	ai.keysMu.RLock()
	publicKey, exists := ai.publicKeys[issuer]
	ai.keysMu.RUnlock()

	if exists {
		return publicKey, nil
	}

	// Slow path: cache miss - query database directly
	ai.logger.Debug("Public key cache miss, querying database",
		zap.String("service_id", issuer))

	service, err := ai.queries.GetServiceByServiceID(ctx, issuer)
	if err != nil {
		ai.logger.Warn("Service not found in database",
			zap.String("service_id", issuer),
			zap.Error(err))
		return nil, fmt.Errorf("unknown issuer: %s", issuer)
	}

	// Check if service is active
	if !service.IsActive.Bool {
		ai.logger.Warn("Service is inactive",
			zap.String("service_id", issuer))
		return nil, fmt.Errorf("service is inactive: %s", issuer)
	}

	// Parse the public key
	publicKey, err = jwt.ParseRSAPublicKeyFromPEM([]byte(service.PublicKey))
	if err != nil {
		ai.logger.Error("Failed to parse public key from database",
			zap.String("service_id", issuer),
			zap.Error(err))
		return nil, fmt.Errorf("invalid public key for issuer: %s", issuer)
	}

	// Cache the key for future requests
	ai.keysMu.Lock()
	ai.publicKeys[issuer] = publicKey
	ai.keysMu.Unlock()

	ai.logger.Info("Loaded and cached public key on-demand",
		zap.String("service_id", issuer))

	return publicKey, nil
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
	// Note: We use a closure to capture ctx for the database fallback lookup
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

		// Look up public key for issuer (with database fallback on cache miss)
		return ai.getPublicKey(ctx, issuer)
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

	issuer := claims["iss"].(string)

	// NOTE: merchant_id is NOT in the token
	// Token identifies the SERVICE only
	// Merchant is specified per-request in the request body
	// Service-merchant access is validated per-request by the authorization service

	// Check if token is blacklisted
	if jti, ok := claims["jti"].(string); ok {
		if ai.isTokenBlacklisted(ctx, jti) {
			return ctx, fmt.Errorf("token has been revoked")
		}
	}

	// Extract scopes from token
	var scopes []string
	if scopesClaim, ok := claims["scopes"].([]interface{}); ok {
		for _, s := range scopesClaim {
			if str, ok := s.(string); ok {
				scopes = append(scopes, str)
			}
		}
	}

	// Add auth context
	// NOTE: MerchantIDKey is NOT set here - it comes from request body
	ctx = context.WithValue(ctx, auth.AuthTypeKey, "jwt")
	ctx = context.WithValue(ctx, auth.ServiceIDKey, issuer)
	if len(scopes) > 0 {
		ctx = context.WithValue(ctx, auth.ScopesKey, scopes)
	}
	if jti, ok := claims["jti"].(string); ok {
		ctx = context.WithValue(ctx, auth.TokenJTIKey, jti)
	}

	return ctx, nil
}

// isTokenBlacklisted checks if a JWT has been blacklisted
func (ai *AuthInterceptor) isTokenBlacklisted(ctx context.Context, jti string) bool {
	// Use timeout context for database query (2 seconds should be sufficient)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
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
	// Use detached context with timeout since request context may be cancelled
	go func() {
		insertCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
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

// HTTPAuthMiddleware wraps an HTTP handler with JWT authentication
// This is used for standard HTTP endpoints (not ConnectRPC) that need auth
func (ai *AuthInterceptor) HTTPAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Add request ID to context
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		ctx = context.WithValue(ctx, auth.RequestIDKey, requestID)

		// Extract client IP from headers
		clientIP := extractClientIP(r.Header)
		if clientIP != "" {
			ctx = context.WithValue(ctx, auth.ClientIPKey, clientIP)
		}

		// Extract User-Agent
		userAgent := r.Header.Get("User-Agent")
		if userAgent != "" {
			ctx = context.WithValue(ctx, auth.UserAgentKey, userAgent)
		}

		// JWT authentication
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			ai.logAuth(ctx, false, "missing authentication credentials", r.URL.Path)
			http.Error(w, "missing authentication", http.StatusUnauthorized)
			return
		}

		// Validate JWT and add auth info to context
		ctx, err := ai.authenticateJWTContext(ctx, authHeader)
		if err != nil {
			ai.logger.Warn("JWT validation failed",
				zap.String("path", r.URL.Path),
				zap.Error(err))
			ai.logAuth(ctx, false, err.Error(), r.URL.Path)
			http.Error(w, "invalid authentication", http.StatusUnauthorized)
			return
		}

		// Apply rate limiting
		if err := ai.checkRateLimit(ctx); err != nil {
			ai.logAuth(ctx, false, "rate limit exceeded", r.URL.Path)
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Log successful auth
		ai.logAuth(ctx, true, "", r.URL.Path)

		// Call next handler with authenticated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
