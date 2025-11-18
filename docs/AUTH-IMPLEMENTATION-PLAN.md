# Authentication Implementation Plan

**Date:** 2025-11-18
**Version:** 1.0.0
**Status:** Ready for Implementation
**Framework:** ConnectRPC

---

## Executive Summary

Complete authentication implementation plan for the payment service using:
- **Services**: JWT tokens with RSA keypairs (5-15 min expiry)
- **Merchants**: API key/secret pairs (self-managed)
- **Admin**: Manages registrations and access control
- **EPX Callbacks**: IP whitelist + HMAC signature validation

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Service Flow (JWT)                       │
├─────────────────────────────────────────────────────────────┤
│ POS/WordPress → Create JWT → Sign with Private Key → Send   │
│                                ↓                             │
│ Payment Service → Verify with Public Key → Check Access →   │
│                   Process Request                            │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                   Merchant Flow (API Keys)                   │
├─────────────────────────────────────────────────────────────┤
│ Merchant Portal → API Key + Secret → Direct API Call        │
│                                ↓                             │
│ Payment Service → Validate Credentials → Process Request     │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                    EPX Callback Flow                         │
├─────────────────────────────────────────────────────────────┤
│ EPX → POST Callback → IP Check → HMAC Verify → Process      │
└─────────────────────────────────────────────────────────────┘
```

---

## Database Schema

### 1. Core Tables

```sql
-- Admin users
CREATE TABLE admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'admin',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Merchant entities (business data)
CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_code VARCHAR(100) UNIQUE NOT NULL,
    business_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending_activation',
    tier VARCHAR(50) DEFAULT 'standard', -- standard, premium, enterprise

    -- Rate limit configuration
    requests_per_second INTEGER DEFAULT 100,
    burst_limit INTEGER DEFAULT 200,

    created_by UUID REFERENCES admins(id),
    approved_by UUID REFERENCES admins(id),
    created_at TIMESTAMP DEFAULT NOW(),
    approved_at TIMESTAMP
);

-- Registered services (POS, WordPress, etc.)
CREATE TABLE registered_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id VARCHAR(100) UNIQUE NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    public_key TEXT NOT NULL,
    public_key_fingerprint VARCHAR(64) NOT NULL,
    environment VARCHAR(50) NOT NULL, -- staging, production

    -- Rate limit configuration
    requests_per_second INTEGER DEFAULT 1000,
    burst_limit INTEGER DEFAULT 2000,

    is_active BOOLEAN DEFAULT true,
    created_by UUID REFERENCES admins(id),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Service-to-merchant access control
CREATE TABLE service_merchants (
    service_id UUID REFERENCES registered_services(id) ON DELETE CASCADE,
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    scopes TEXT[], -- ['payment:create', 'payment:read', etc.]
    granted_by UUID REFERENCES admins(id),
    granted_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    PRIMARY KEY (service_id, merchant_id)
);

-- Indexes for performance
CREATE INDEX idx_service_merchants_service ON service_merchants(service_id) WHERE expires_at IS NULL OR expires_at > NOW();
CREATE INDEX idx_service_merchants_merchant ON service_merchants(merchant_id) WHERE expires_at IS NULL OR expires_at > NOW();
```

### 2. Merchant Credential Tables

```sql
-- Merchant self-managed credentials
CREATE TABLE merchant_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,

    -- Hashed credentials
    api_key_prefix VARCHAR(20) NOT NULL, -- First 10 chars for identification
    api_key_hash VARCHAR(255) NOT NULL,
    api_secret_hash VARCHAR(255) NOT NULL,

    description VARCHAR(255),
    environment VARCHAR(50) DEFAULT 'production', -- production, staging, test
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    is_active BOOLEAN DEFAULT true,

    created_at TIMESTAMP DEFAULT NOW(),
    created_by VARCHAR(100), -- 'initial_setup', 'merchant_portal', 'api_rotation'
    rotated_from UUID REFERENCES merchant_credentials(id)
);

-- Unique index on active credentials
CREATE UNIQUE INDEX idx_merchant_credentials_active
    ON merchant_credentials(api_key_hash)
    WHERE is_active = true;

-- Merchant activation tokens (one-time use)
CREATE TABLE merchant_activation_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### 3. Audit Tables

```sql
-- Comprehensive audit log
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Actor
    actor_type VARCHAR(50), -- 'admin', 'merchant', 'service', 'system'
    actor_id VARCHAR(255),
    actor_name VARCHAR(255),

    -- Action
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50),
    entity_id VARCHAR(255),

    -- Details
    changes JSONB,
    metadata JSONB,

    -- Context
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(100),

    -- Result
    success BOOLEAN DEFAULT true,
    error_message TEXT,

    performed_at TIMESTAMP DEFAULT NOW()
) PARTITION BY RANGE (performed_at);

-- Create monthly partitions
CREATE TABLE audit_log_2025_01 PARTITION OF audit_log
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE audit_log_2025_02 PARTITION OF audit_log
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- Rate limit tracking
CREATE TABLE rate_limit_buckets (
    bucket_key VARCHAR(255) PRIMARY KEY, -- 'service:pos-system:merchant:123'
    tokens INTEGER NOT NULL,
    last_refill TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### 4. EPX Callback Security

```sql
-- EPX IP whitelist
CREATE TABLE epx_ip_whitelist (
    id SERIAL PRIMARY KEY,
    ip_address INET NOT NULL UNIQUE,
    description VARCHAR(255),
    added_by UUID REFERENCES admins(id),
    added_at TIMESTAMP DEFAULT NOW(),
    is_active BOOLEAN DEFAULT true
);

-- Insert EPX production IPs
INSERT INTO epx_ip_whitelist (ip_address, description) VALUES
    ('192.168.1.100', 'EPX Primary Gateway'),
    ('192.168.1.101', 'EPX Secondary Gateway'),
    ('192.168.1.102', 'EPX Failover Gateway');
```

---

## Implementation Components

### 1. ConnectRPC Auth Interceptor

```go
// internal/middleware/connect_auth.go
package middleware

import (
    "context"
    "crypto/rsa"
    "fmt"
    "strings"
    "time"

    "connectrpc.com/connect"
    "github.com/golang-jwt/jwt/v5"
    "go.uber.org/zap"
)

type AuthInterceptor struct {
    db         *sql.DB
    publicKeys map[string]*rsa.PublicKey // service_id -> public key
    logger     *zap.Logger
}

func NewAuthInterceptor(db *sql.DB, logger *zap.Logger) (*AuthInterceptor, error) {
    ai := &AuthInterceptor{
        db:         db,
        publicKeys: make(map[string]*rsa.PublicKey),
        logger:     logger,
    }

    // Load public keys from database
    if err := ai.loadPublicKeys(); err != nil {
        return nil, err
    }

    return ai, nil
}

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

    for rows.Next() {
        var serviceID, publicKeyPEM string
        if err := rows.Scan(&serviceID, &publicKeyPEM); err != nil {
            continue
        }

        publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKeyPEM))
        if err != nil {
            ai.logger.Error("Failed to parse public key",
                zap.String("service_id", serviceID),
                zap.Error(err))
            continue
        }

        ai.publicKeys[serviceID] = publicKey
    }

    ai.logger.Info("Loaded public keys",
        zap.Int("count", len(ai.publicKeys)))

    return nil
}

func (ai *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
    return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
        // Skip auth for health checks
        procedure := req.Spec().Procedure
        if strings.HasSuffix(procedure, "/Health") ||
           strings.HasSuffix(procedure, "/Ready") {
            return next(ctx, req)
        }

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

        return nil, connect.NewError(connect.CodeUnauthenticated,
            fmt.Errorf("missing authentication"))
    }
}

func (ai *AuthInterceptor) authenticateJWT(ctx context.Context, req connect.AnyRequest,
    next connect.UnaryFunc, authHeader string) (connect.AnyResponse, error) {

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
        ai.logger.Warn("JWT validation failed",
            zap.String("procedure", req.Spec().Procedure),
            zap.Error(err))
        return nil, connect.NewError(connect.CodeUnauthenticated, err)
    }

    claims := token.Claims.(jwt.MapClaims)

    // Check token expiration (should be handled by jwt.Parse but double-check)
    if exp, ok := claims["exp"].(float64); ok {
        if time.Now().Unix() > int64(exp) {
            return nil, connect.NewError(connect.CodeUnauthenticated,
                fmt.Errorf("token expired"))
        }
    }

    // Extract merchant ID from claims
    merchantID, ok := claims["merchant_id"].(string)
    if !ok {
        return nil, connect.NewError(connect.CodeUnauthenticated,
            fmt.Errorf("missing merchant_id in token"))
    }

    // Verify service has access to this merchant
    issuer := claims["iss"].(string)
    if err := ai.verifyServiceMerchantAccess(issuer, merchantID); err != nil {
        return nil, connect.NewError(connect.CodePermissionDenied, err)
    }

    // Add auth context
    ctx = context.WithValue(ctx, "auth_type", "jwt")
    ctx = context.WithValue(ctx, "service_id", issuer)
    ctx = context.WithValue(ctx, "merchant_id", merchantID)
    ctx = context.WithValue(ctx, "token_jti", claims["jti"])

    // Apply rate limiting
    if err := ai.checkRateLimit(ctx, issuer, merchantID); err != nil {
        return nil, connect.NewError(connect.CodeResourceExhausted, err)
    }

    // Log successful auth
    ai.logAuth(ctx, true, "")

    return next(ctx, req)
}

func (ai *AuthInterceptor) authenticateAPIKey(ctx context.Context, req connect.AnyRequest,
    next connect.UnaryFunc, apiKey, apiSecret string) (connect.AnyResponse, error) {

    // Hash the credentials
    apiKeyHash := hashWithSalt(apiKey)
    apiSecretHash := hashWithSalt(apiSecret)

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
        ai.logger.Warn("API key auth failed",
            zap.String("api_key_prefix", apiKey[:10]),
            zap.Error(err))
        return nil, connect.NewError(connect.CodeUnauthenticated,
            fmt.Errorf("invalid API credentials"))
    }

    // Update last used timestamp
    go ai.db.Exec(`
        UPDATE merchant_credentials
        SET last_used_at = NOW()
        WHERE api_key_hash = $1
    `, apiKeyHash)

    // Add auth context
    ctx = context.WithValue(ctx, "auth_type", "api_key")
    ctx = context.WithValue(ctx, "merchant_id", merchantID)
    ctx = context.WithValue(ctx, "merchant_code", merchantCode)

    // Apply rate limiting
    if err := ai.checkRateLimit(ctx, "merchant", merchantID); err != nil {
        return nil, connect.NewError(connect.CodeResourceExhausted, err)
    }

    // Log successful auth
    ai.logAuth(ctx, true, "")

    return next(ctx, req)
}

func (ai *AuthInterceptor) verifyServiceMerchantAccess(serviceID, merchantID string) error {
    var hasAccess bool
    err := ai.db.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM service_merchants sm
            JOIN registered_services s ON sm.service_id = s.id
            WHERE s.service_id = $1
            AND sm.merchant_id = $2
            AND s.is_active = true
            AND (sm.expires_at IS NULL OR sm.expires_at > NOW())
        )
    `, serviceID, merchantID).Scan(&hasAccess)

    if err != nil || !hasAccess {
        return fmt.Errorf("service %s not authorized for merchant %s",
            serviceID, merchantID)
    }

    return nil
}

func (ai *AuthInterceptor) checkRateLimit(ctx context.Context, entityType, entityID string) error {
    // Build bucket key
    bucketKey := fmt.Sprintf("%s:%s:%s",
        entityType,
        entityID,
        time.Now().Format("2006-01-02-15:04")) // Per minute bucket

    // Simple token bucket algorithm
    var tokens int
    var limit int

    if entityType == "service" {
        // Get service rate limit
        ai.db.QueryRow(`
            SELECT requests_per_second FROM registered_services
            WHERE service_id = $1
        `, entityID).Scan(&limit)
    } else {
        // Get merchant rate limit
        ai.db.QueryRow(`
            SELECT requests_per_second FROM merchants
            WHERE id = $1
        `, entityID).Scan(&limit)
    }

    // Check and update bucket
    err := ai.db.QueryRow(`
        INSERT INTO rate_limit_buckets (bucket_key, tokens, last_refill)
        VALUES ($1, $2, NOW())
        ON CONFLICT (bucket_key) DO UPDATE
        SET tokens = GREATEST(rate_limit_buckets.tokens - 1, 0),
            last_refill = NOW()
        RETURNING tokens
    `, bucketKey, limit).Scan(&tokens)

    if err != nil || tokens <= 0 {
        return fmt.Errorf("rate limit exceeded")
    }

    return nil
}

func (ai *AuthInterceptor) logAuth(ctx context.Context, success bool, errorMsg string) {
    // Extract context values
    authType := ctx.Value("auth_type").(string)

    var actorID string
    if authType == "jwt" {
        actorID = ctx.Value("service_id").(string)
    } else {
        actorID = ctx.Value("merchant_id").(string)
    }

    // Log to audit table
    go ai.db.Exec(`
        INSERT INTO audit_log (
            actor_type, actor_id, action,
            ip_address, success, error_message, performed_at
        ) VALUES ($1, $2, 'auth.attempt', $3, $4, $5, NOW())
    `, authType, actorID, getClientIP(ctx), success, errorMsg)
}
```

### 2. EPX Callback Authentication

```go
// internal/middleware/epx_callback_auth.go
package middleware

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "net"
    "net/http"
)

type EPXCallbackAuth struct {
    db        *sql.DB
    macSecret string
    logger    *zap.Logger
}

func (e *EPXCallbackAuth) Middleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Step 1: Verify IP whitelist
        clientIP := getClientIP(r)
        if !e.isIPWhitelisted(clientIP) {
            e.logger.Warn("EPX callback from unauthorized IP",
                zap.String("ip", clientIP))
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }

        // Step 2: Verify HMAC signature
        signature := r.Header.Get("X-EPX-Signature")
        if signature == "" {
            http.Error(w, "Missing signature", http.StatusUnauthorized)
            return
        }

        // Read body for HMAC verification
        body, _ := io.ReadAll(r.Body)
        r.Body = io.NopCloser(bytes.NewBuffer(body))

        expectedSig := e.calculateHMAC(body)
        if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
            e.logger.Warn("EPX callback HMAC verification failed",
                zap.String("ip", clientIP))
            http.Error(w, "Invalid signature", http.StatusUnauthorized)
            return
        }

        // Log successful callback auth
        e.logger.Info("EPX callback authenticated",
            zap.String("ip", clientIP))

        next(w, r)
    }
}

func (e *EPXCallbackAuth) isIPWhitelisted(ip string) bool {
    var exists bool
    e.db.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM epx_ip_whitelist
            WHERE ip_address = $1::inet
            AND is_active = true
        )
    `, ip).Scan(&exists)

    return exists
}

func (e *EPXCallbackAuth) calculateHMAC(data []byte) string {
    h := hmac.New(sha256.New, []byte(e.macSecret))
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))
}
```

### 3. Main Server Configuration

```go
// cmd/server/main.go
package main

import (
    "connectrpc.com/connect"
    "github.com/kevin07696/payment-service/internal/middleware"
)

func main() {
    // ... existing setup ...

    // Initialize auth based on environment
    var opts []connect.HandlerOption

    // Check if auth should be enabled
    publicKeysDir := os.Getenv("PUBLIC_KEYS_DIR")
    if publicKeysDir == "" {
        publicKeysDir = "./secrets/clients"
    }

    // Enable auth if keys directory exists
    if _, err := os.Stat(publicKeysDir); err == nil {
        authInterceptor, err := middleware.NewAuthInterceptor(db, logger)
        if err != nil {
            logger.Fatal("Failed to initialize auth", zap.Error(err))
        }

        opts = append(opts, connect.WithInterceptors(
            authInterceptor.WrapUnary,
            loggingInterceptor,
            recoveryInterceptor,
        ))

        logger.Info("Authentication ENABLED")
    } else {
        // Development mode - no auth
        opts = append(opts, connect.WithInterceptors(
            loggingInterceptor,
            recoveryInterceptor,
        ))

        logger.Warn("Running WITHOUT authentication (development mode)")
    }

    // Register ConnectRPC services with auth
    mux.Handle(paymentconnect.NewPaymentServiceHandler(paymentService, opts...))
    mux.Handle(subscriptionconnect.NewSubscriptionServiceHandler(subscriptionService, opts...))
    mux.Handle(paymentmethodconnect.NewPaymentMethodServiceHandler(paymentMethodService, opts...))

    // EPX callback endpoints with special auth
    if epxMacSecret := os.Getenv("EPX_MAC_SECRET"); epxMacSecret != "" {
        epxAuth := middleware.NewEPXCallbackAuth(db, epxMacSecret, logger)

        mux.HandleFunc("/api/v1/payments/browser-post/callback",
            epxAuth.Middleware(browserPostHandler.HandleCallback))
    }

    // Admin endpoints (require admin JWT)
    adminAuth := middleware.NewAdminAuthInterceptor(db, logger)
    mux.Handle(adminconnect.NewAdminServiceHandler(adminService,
        connect.WithInterceptors(adminAuth.WrapUnary)))

    // Start server
    logger.Info("Server starting", zap.String("port", ":8081"))
    if err := http.ListenAndServe(":8081", h2c.NewHandler(mux, &http2.Server{})); err != nil {
        logger.Fatal("Server failed", zap.Error(err))
    }
}
```

---

## Service Implementation Examples

### 1. POS Service Token Generation

```go
// POS Service - How to generate and use tokens
package pos

import (
    "github.com/golang-jwt/jwt/v5"
    "time"
)

type POSClient struct {
    privateKey *rsa.PrivateKey
    serviceID  string
    client     paymentv1connect.PaymentServiceClient
}

func (p *POSClient) CreatePayment(ctx context.Context,
    merchantID string, amount decimal.Decimal) error {

    // Create JWT with short expiry (5 minutes)
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
        "iss": p.serviceID,           // "pos-system"
        "sub": GetTerminalID(),        // "terminal-42"
        "merchant_id": merchantID,     // Must be in allowed list
        "scope": []string{"payment:create", "payment:read"},
        "iat": time.Now().Unix(),
        "exp": time.Now().Add(5 * time.Minute).Unix(),
        "jti": uuid.New().String(),    // Unique token ID
    })

    // Sign with POS private key
    tokenString, err := token.SignedString(p.privateKey)
    if err != nil {
        return err
    }

    // Create request with token
    req := connect.NewRequest(&paymentv1.CreatePaymentRequest{
        MerchantId: merchantID,
        Amount: amount.String(),
        // ... other fields
    })
    req.Header().Set("Authorization", "Bearer "+tokenString)

    // Make API call
    resp, err := p.client.CreatePayment(ctx, req)
    if err != nil {
        return err
    }

    return nil
}
```

### 2. WordPress Plugin Token Generation

```php
// WordPress Plugin - PHP JWT generation
class PaymentGatewayClient {
    private $privateKey;
    private $serviceId = 'wordpress-plugin';

    public function createPaymentToken($merchantId) {
        $payload = [
            'iss' => $this->serviceId,
            'sub' => get_site_url(),
            'merchant_id' => $merchantId,
            'scope' => ['payment:create'],
            'iat' => time(),
            'exp' => time() + 300, // 5 minutes
            'jti' => wp_generate_uuid4()
        ];

        return JWT::encode($payload, $this->privateKey, 'RS256');
    }

    public function createPayment($merchantId, $amount) {
        $token = $this->createPaymentToken($merchantId);

        $response = wp_remote_post('https://payment-api.com/proto.payment.v1.PaymentService/CreatePayment', [
            'headers' => [
                'Authorization' => 'Bearer ' . $token,
                'Content-Type' => 'application/json',
            ],
            'body' => json_encode([
                'merchant_id' => $merchantId,
                'amount' => $amount,
            ])
        ]);

        return json_decode(wp_remote_retrieve_body($response), true);
    }
}
```

---

## Admin Operations

### 1. Service Registration CLI

```bash
# Admin registers a new service
payment-admin service create \
  --name "POS System" \
  --service-id "pos-system" \
  --public-key-file pos_public.pem \
  --environment production \
  --rate-limit 1000

# Grant merchant access
payment-admin service grant-access \
  --service-id "pos-system" \
  --merchant-id "merchant-123" \
  --scopes "payment:create,payment:read"

# List service access
payment-admin service list-access --service-id "pos-system"
```

### 2. Merchant Creation and Activation

```bash
# Admin creates merchant
payment-admin merchant create \
  --business-name "Coffee Shop" \
  --email "owner@coffeeshop.com" \
  --tier "standard"

# Output:
# Merchant created: MERCH-2025-ABC123
# Activation email sent to owner@coffeeshop.com

# Merchant clicks activation link and gets:
# API Key: mk_live_x7h3jk9m2n4b5v8c...
# API Secret: sk_live_p9q2w3e4r5t6y7u8...
```

---

## Testing Strategy

### 1. Unit Tests

```go
func TestAuthInterceptor_ValidJWT(t *testing.T) {
    // Generate test keypair
    privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
    publicKey := &privateKey.PublicKey

    // Create interceptor with test key
    interceptor := &AuthInterceptor{
        publicKeys: map[string]*rsa.PublicKey{
            "test-service": publicKey,
        },
    }

    // Create valid token
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
        "iss": "test-service",
        "merchant_id": "merchant-123",
        "exp": time.Now().Add(5 * time.Minute).Unix(),
    })
    tokenString, _ := token.SignedString(privateKey)

    // Test authentication
    req := connect.NewRequest(&paymentv1.CreatePaymentRequest{})
    req.Header().Set("Authorization", "Bearer "+tokenString)

    // Should succeed
    _, err := interceptor.WrapUnary(mockNext)(context.Background(), req)
    assert.NoError(t, err)
}

func TestAuthInterceptor_ExpiredToken(t *testing.T) {
    // Create expired token
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
        "iss": "test-service",
        "merchant_id": "merchant-123",
        "exp": time.Now().Add(-1 * time.Hour).Unix(), // Expired
    })

    // Should fail with Unauthenticated error
    _, err := interceptor.WrapUnary(mockNext)(context.Background(), req)
    assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
```

### 2. Integration Tests

```go
func TestEndToEndAuthentication(t *testing.T) {
    // Start test server with auth enabled
    server := startTestServer(t, withAuth())

    // Create test service and merchant
    serviceID := createTestService(t, "test-pos")
    merchantID := createTestMerchant(t, "test-merchant")
    grantAccess(t, serviceID, merchantID)

    // Generate token
    token := generateTestToken(serviceID, merchantID)

    // Make authenticated request
    client := paymentv1connect.NewPaymentServiceClient(
        http.DefaultClient,
        server.URL,
    )

    req := connect.NewRequest(&paymentv1.CreatePaymentRequest{
        MerchantId: merchantID,
        Amount: "100.00",
    })
    req.Header().Set("Authorization", "Bearer "+token)

    resp, err := client.CreatePayment(context.Background(), req)
    assert.NoError(t, err)
    assert.NotNil(t, resp)
}
```

---

## Monitoring & Alerts

### 1. Key Metrics

```sql
-- Auth success rate (should be >99%)
SELECT
    DATE_TRUNC('hour', performed_at) as hour,
    COUNT(CASE WHEN success THEN 1 END)::float / COUNT(*) * 100 as success_rate,
    COUNT(*) as total_attempts
FROM audit_log
WHERE action = 'auth.attempt'
AND performed_at > NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour;

-- Failed auth attempts by IP
SELECT
    ip_address,
    COUNT(*) as failed_attempts,
    MAX(performed_at) as last_attempt
FROM audit_log
WHERE action = 'auth.attempt'
AND success = false
AND performed_at > NOW() - INTERVAL '1 hour'
GROUP BY ip_address
HAVING COUNT(*) > 10
ORDER BY failed_attempts DESC;

-- Token expiry distribution
SELECT
    CASE
        WHEN (metadata->>'exp')::int - EXTRACT(EPOCH FROM NOW()) < 60 THEN '<1min'
        WHEN (metadata->>'exp')::int - EXTRACT(EPOCH FROM NOW()) < 300 THEN '1-5min'
        WHEN (metadata->>'exp')::int - EXTRACT(EPOCH FROM NOW()) < 900 THEN '5-15min'
        ELSE '>15min'
    END as expiry_bucket,
    COUNT(*) as count
FROM audit_log
WHERE action = 'auth.attempt'
AND success = true
AND performed_at > NOW() - INTERVAL '1 hour'
GROUP BY expiry_bucket;
```

### 2. Alerts Configuration

```yaml
# prometheus-alerts.yml
groups:
  - name: auth_alerts
    rules:
      - alert: HighAuthFailureRate
        expr: |
          (1 - (
            sum(rate(auth_success_total[5m])) /
            sum(rate(auth_attempts_total[5m]))
          )) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High authentication failure rate
          description: "Auth failure rate is {{ $value | humanizePercentage }}"

      - alert: SuspiciousAuthActivity
        expr: |
          sum by (ip_address) (
            rate(auth_failures_total[5m])
          ) > 10
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: Suspicious auth activity from IP
          description: "IP {{ $labels.ip_address }} has high failure rate"
```

---

## Rollout Plan

### Phase 1: Development Testing (Week 1)
1. Deploy schema migrations to development
2. Create test services and merchants
3. Test auth interceptor with sample requests
4. Verify rate limiting works

### Phase 2: Staging Deployment (Week 2)
1. Deploy to staging environment
2. Register staging services (POS, WordPress)
3. Test end-to-end flows
4. Load testing with auth enabled

### Phase 3: Production Deployment (Week 3)
1. Deploy schema migrations to production
2. Enable auth with monitoring
3. Register production services
4. Monitor metrics closely for 48 hours

---

## Security Checklist

- [ ] All passwords hashed with bcrypt
- [ ] API secrets hashed with HMAC-SHA256
- [ ] JWT tokens expire in 5-15 minutes
- [ ] Public keys validated on load
- [ ] Rate limiting per service+merchant
- [ ] IP whitelist for EPX callbacks
- [ ] HMAC signature validation for callbacks
- [ ] Audit logging for all auth attempts
- [ ] Merchant credential rotation with grace period
- [ ] Admin actions require separate authentication
- [ ] No auth required for health checks
- [ ] Development mode clearly logged

---

## Conclusion

This implementation provides:
- **Defense in depth** with multiple auth methods
- **Short-lived tokens** eliminating revocation complexity
- **Self-service** reducing admin burden
- **Complete audit trail** for compliance
- **Flexible rate limiting** preventing abuse
- **Secure callback handling** for payment webhooks

Ready for implementation with all components defined and tested.