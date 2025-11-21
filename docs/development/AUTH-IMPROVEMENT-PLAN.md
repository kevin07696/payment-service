# Authentication Improvement Plan

**Date:** 2025-11-18
**Status:** Updated for ConnectRPC
**Current Version:** 0.2.0
**Last Updated:** 2025-11-18

---

## Executive Summary

The payment service has **JWT-based authentication code written** but **NOT enabled**. This document outlines the current state, identifies security gaps, and provides a prioritized implementation plan.

**Quick Answer:**
- ✅ Auth code exists and is well-designed
- ❌ Auth is NOT enabled (running in open mode)
- 🎯 Can enable with minimal changes (< 1 hour)

---

## Current RPC Framework

### What Framework is This?

**Answer: ConnectRPC (Unified HTTP/gRPC Framework)**

```
Architecture:
┌─────────────────────────────────────────────────┐
│  Client Requests                                 │
├─────────────────────────────────────────────────┤
│                                                  │
│  Unified HTTP Server (Port 8081)                │
│  ↓                                               │
│  ConnectRPC Handler                             │
│  ├─ Supports: gRPC, gRPC-Web, Connect Protocol │
│  ├─ Content-Type negotiation                    │
│  └─ Single port for all protocols               │
│  ↓                                               │
│  Business Logic                                  │
│                                                  │
└─────────────────────────────────────────────────┘
```

**Evidence:**
```go
// go.mod - Primary RPC framework
connectrpc.com/connect v1.19.1  // Main RPC framework

// Generated code uses ConnectRPC
// gen/proto/payment/v1/paymentv1connect/payment.connect.go
// gen/proto/subscription/v1/subscriptionv1connect/subscription.connect.go
```

**Benefits of ConnectRPC:**
- Single port for all protocols (HTTP/1.1, HTTP/2, gRPC)
- Built-in JSON/Protobuf support
- Simpler interceptor model
- Better browser compatibility
- No need for separate grpc-gateway

**Endpoints:**
```
Port 8081: Unified HTTP Server
├─ ConnectRPC Services:
│  ├─ /proto.payment.v1.PaymentService/*
│  ├─ /proto.subscription.v1.SubscriptionService/*
│  └─ /proto.payment_method.v1.PaymentMethodService/*
│
└─ REST Endpoints (custom HTTP handlers):
   ├─ /api/v1/payments/browser-post/form      (GET - returns TAC form)
   └─ /api/v1/payments/browser-post/callback  (POST - EPX callback)
```

---

## Current Authentication Status

### What Exists (Code Written)

#### 1. **JWT Token Infrastructure** ✅
- Location: `internal/middleware/connect_auth_interceptor.go` (needs adaptation)
- RSA signature validation
- Token claims validation
- Multi-tenant support (merchant_ids array)
- Scope-based permissions

#### 2. **Public Key Management** ✅
- Location: `internal/auth/public_key_store.go`
- Load keys from directory
- Support multiple issuers
- Thread-safe key storage

#### 3. **Authorization Logic** ✅
- Location: `internal/services/authorization/merchant_resolver.go`
- Merchant isolation (single/multi-tenant)
- Customer privacy protection
- Admin access controls

#### 4. **Token Types Supported** ✅
```typescript
{
  token_type: "merchant",   // POS, WordPress
  token_type: "customer",   // E-commerce users
  token_type: "guest",      // Guest checkout
  token_type: "admin",      // Platform admin
}
```

### What's NOT Enabled ❌

**cmd/server/main.go - ConnectRPC setup:**
```go
// Current state - no auth interceptor
mux := http.NewServeMux()

// ConnectRPC services mounted WITHOUT authentication
mux.Handle(paymentconnect.NewPaymentServiceHandler(
    paymentService,
    connect.WithInterceptors(
        loggingInterceptor,    // ✅ Enabled
        recoveryInterceptor,   // ✅ Enabled
        // ❌ authInterceptor NOT HERE!
    ),
))
```

**Impact:**
```
Current State: OPEN SYSTEM
├─ ConnectRPC endpoints: No authentication
├─ REST callbacks: No authentication
├─ Browser Post forms: No authentication
└─ Only protection: Rate limiting (10 req/sec per IP)
```

---

## Security Gaps Analysis

### Critical Gaps

#### 1. **No Client Authentication** (Severity: CRITICAL)
**Risk:** Anyone who can reach the service can initiate payments

```bash
# Current: Anyone can do this
curl http://localhost:8081/api/v1/payments/browser-post/form?...
# No token required!
```

**Attack Scenario:**
```
1. Attacker discovers payment service URL
2. Calls /api/v1/payments/browser-post/form
3. Gets TAC token for EPX
4. Initiates fraudulent payments
```

#### 2. **No Token Revocation** (Severity: HIGH)
**Risk:** Compromised tokens valid until expiration (8 hours)

```typescript
// Token issued at 10:00 AM with 8hr expiry
{
  exp: 1736683200  // Expires at 6:00 PM
}

// If WordPress gets hacked at 11:00 AM:
// - Attacker has token
// - Can use until 6:00 PM
// - No way to revoke it!
```

#### 3. **HTTP Endpoints Unprotected** (Severity: HIGH)
**Risk:** Browser Post callbacks can be spoofed

```go
// cmd/server/main.go lines 125-126
httpMux.HandleFunc("/api/v1/payments/browser-post/form", ...)
httpMux.HandleFunc("/api/v1/payments/browser-post/callback", ...)

// NO authentication middleware!
```

#### 4. **Static Key Management** (Severity: MEDIUM)
**Risk:** Can't rotate keys without service restart

```go
// Keys loaded once at startup
keyStore.LoadKeysFromDirectory("./secrets/clients")

// To rotate keys:
// 1. Replace key file
// 2. Restart service
// 3. Downtime!
```

#### 5. **No Per-Merchant Rate Limiting** (Severity: MEDIUM)
**Risk:** Compromised token can spam from multiple IPs

```go
// Current: Only IP-based
rateLimiter := middleware.NewRateLimiter(10, 20)

// Attacker can:
// - Use 100 different IPs
// - Each gets 10 req/sec
// - = 1000 req/sec total!
```

---

## Improvement Plan

### Phase 1: Enable Authentication (CRITICAL - 1 hour)

**Goal:** Turn on JWT authentication for immediate protection

**Changes:**

1. **Create public keys directory:**
```bash
mkdir -p secrets/clients
```

2. **Create ConnectRPC Auth Interceptor (`internal/middleware/connect_auth.go`):**
```go
package middleware

import (
    "context"
    "strings"

    "connectrpc.com/connect"
    "github.com/kevin07696/payment-service/internal/auth"
    "go.uber.org/zap"
)

type ConnectAuthInterceptor struct {
    keyStore *auth.PublicKeyStore
    logger   *zap.Logger
}

func NewConnectAuthInterceptor(keyStore *auth.PublicKeyStore, logger *zap.Logger) *ConnectAuthInterceptor {
    return &ConnectAuthInterceptor{
        keyStore: keyStore,
        logger:   logger,
    }
}

func (i *ConnectAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
    return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
        // Skip auth for health checks
        if strings.HasSuffix(req.Spec().Procedure, "/Health") {
            return next(ctx, req)
        }

        // Extract token from Authorization header
        authHeader := req.Header().Get("Authorization")
        if authHeader == "" {
            return nil, connect.NewError(connect.CodeUnauthenticated,
                fmt.Errorf("missing authorization header"))
        }

        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := i.verifyJWT(tokenString)
        if err != nil {
            i.logger.Warn("Auth failed",
                zap.String("procedure", req.Spec().Procedure),
                zap.Error(err))
            return nil, connect.NewError(connect.CodeUnauthenticated, err)
        }

        // Add claims to context
        ctx = context.WithValue(ctx, tokenClaimsKey, claims)
        return next(ctx, req)
    }
}
```

3. **Update main.go to use ConnectRPC interceptors:**
```go
// Initialize authentication (optional for development)
publicKeysDir := "./secrets/clients"
var authInterceptor connect.UnaryInterceptorFunc

if _, err := os.Stat(publicKeysDir); err == nil {
    // Keys exist - enable auth
    keyStore := auth.NewPublicKeyStore()
    if err := keyStore.LoadKeysFromDirectory(publicKeysDir); err != nil {
        logger.Fatal("Failed to load keys", zap.Error(err))
    }

    connectAuth := middleware.NewConnectAuthInterceptor(keyStore, logger)
    authInterceptor = connectAuth.WrapUnary
    logger.Info("Authentication ENABLED",
        zap.Strings("issuers", keyStore.ListIssuers()))
} else {
    // No keys - development mode
    logger.Warn("Running WITHOUT authentication (development)")
}

// Build interceptor chain
interceptors := []connect.UnaryInterceptorFunc{
    loggingInterceptor,
    recoveryInterceptor,
}
if authInterceptor != nil {
    interceptors = append([]connect.UnaryInterceptorFunc{authInterceptor}, interceptors...)
}

// Apply to all ConnectRPC services
opts := connect.WithInterceptors(interceptors...)

mux.Handle(paymentconnect.NewPaymentServiceHandler(paymentService, opts))
mux.Handle(subscriptionconnect.NewSubscriptionServiceHandler(subscriptionService, opts))
```

**Benefits:**
- ✅ Optional authentication (backward compatible)
- ✅ No keys = development mode (current behavior)
- ✅ Keys present = auth enforced

**Testing:**
```bash
# 1. Before adding keys: Works as before
curl http://localhost:8081/proto.payment.v1.PaymentService/CreatePayment \
  -H "Content-Type: application/json" \
  -d '{"amount": "100.00"}'
# Success (no auth)

# 2. After adding keys: Requires token
curl http://localhost:8081/proto.payment.v1.PaymentService/CreatePayment \
  -H "Content-Type: application/json" \
  -d '{"amount": "100.00"}'
# Error: Unauthenticated

curl -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  http://localhost:8081/proto.payment.v1.PaymentService/CreatePayment \
  -d '{"amount": "100.00"}'
# Success (with valid token)
```

---

### Phase 2: HTTP Endpoint Protection (HIGH - 2 hours)

**Goal:** Protect Browser Post endpoints from unauthorized access

**New File: `pkg/middleware/http_auth.go`**
```go
package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/kevin07696/payment-service/internal/auth"
    "go.uber.org/zap"
)

type HTTPAuthMiddleware struct {
    keyStore *auth.PublicKeyStore
    logger   *zap.Logger
}

func NewHTTPAuthMiddleware(keyStore *auth.PublicKeyStore, logger *zap.Logger) *HTTPAuthMiddleware {
    return &HTTPAuthMiddleware{
        keyStore: keyStore,
        logger:   logger,
    }
}

func (m *HTTPAuthMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        // Verify token using same logic as gRPC interceptor
        claims, err := verifyJWT(tokenString, m.keyStore)
        if err != nil {
            m.logger.Warn("HTTP auth failed", zap.Error(err))
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Add claims to context
        ctx := context.WithValue(r.Context(), tokenClaimsKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Apply to Browser Post:**
```go
// cmd/server/main.go
if authInterceptor != nil {
    httpAuth := middleware.NewHTTPAuthMiddleware(keyStore, logger)

    httpMux.Handle("/api/v1/payments/browser-post/form",
        httpAuth.Middleware(
            rateLimiter.HTTPHandlerFunc(deps.browserPostCallbackHandler.GetPaymentForm),
        ),
    )
}
```

---

### Phase 3: Token Revocation (CRITICAL - 4 hours)

**Goal:** Ability to immediately revoke compromised tokens

**Approach: Redis Blacklist**

**New File: `pkg/auth/token_blacklist.go`**
```go
package auth

import (
    "context"
    "time"

    "github.com/redis/go-redis/v9"
)

type TokenBlacklist struct {
    redis *redis.Client
}

func NewTokenBlacklist(redisURL string) (*TokenBlacklist, error) {
    opts, err := redis.ParseURL(redisURL)
    if err != nil {
        return nil, err
    }

    return &TokenBlacklist{
        redis: redis.NewClient(opts),
    }, nil
}

// RevokeToken adds a token to the blacklist
func (b *TokenBlacklist) RevokeToken(ctx context.Context, jti string, expiresAt time.Time) error {
    ttl := time.Until(expiresAt)
    return b.redis.Set(ctx, "revoked:"+jti, "1", ttl).Err()
}

// IsRevoked checks if a token is revoked
func (b *TokenBlacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
    exists, err := b.redis.Exists(ctx, "revoked:"+jti).Result()
    return exists > 0, err
}

// RevokeMerchant revokes all tokens for a merchant
func (b *TokenBlacklist) RevokeMerchant(ctx context.Context, merchantID string, duration time.Duration) error {
    // Add merchant to revocation list
    return b.redis.Set(ctx, "merchant:revoked:"+merchantID, "1", duration).Err()
}
```

**Update Interceptor:**
```go
// Check revocation before processing
if revoked, _ := blacklist.IsRevoked(ctx, claims.ID); revoked {
    return nil, status.Error(codes.Unauthenticated, "token revoked")
}
```

**Add to docker-compose.yml:**
```yaml
redis:
  image: redis:7-alpine
  ports:
    - "6379:6379"
  volumes:
    - redis_data:/data
```

---

### Phase 4: Refresh Token Pattern (HIGH - 8 hours)

**Goal:** Short-lived access tokens + long-lived refresh tokens

**Benefits:**
- Access token: 15 minutes (limited exposure)
- Refresh token: 30 days (stored in database)
- Can revoke refresh tokens immediately

**Database Migration:**
```sql
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    token_hash VARCHAR(128) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_merchant ON refresh_tokens(merchant_id)
    WHERE revoked_at IS NULL;
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash)
    WHERE revoked_at IS NULL;
```

**New Auth Service (proto/auth/v1/auth.proto):**
```protobuf
syntax = "proto3";

package proto.auth.v1;

option go_package = "github.com/kevin07696/payment-service/gen/proto/auth/v1;authv1";

service AuthService {
    // Refresh an access token using a refresh token
    rpc RefreshToken(RefreshTokenRequest) returns (TokenPair);

    // Revoke a refresh token
    rpc RevokeToken(RevokeTokenRequest) returns (RevokeTokenResponse);
}

message RefreshTokenRequest {
    string refresh_token = 1;
}

message TokenPair {
    string access_token = 1;   // 15 min
    string refresh_token = 2;  // 30 days
    int32 expires_in = 3;      // 900 seconds
}

message RevokeTokenRequest {
    string refresh_token = 1;
}

message RevokeTokenResponse {
    bool success = 1;
}
```

**ConnectRPC Implementation:**
```go
// Register the service with ConnectRPC
mux.Handle(authconnect.NewAuthServiceHandler(authService, opts))
```

---

### Phase 5: Enhanced Rate Limiting (MEDIUM - 4 hours)

**Goal:** Per-merchant rate limits

**New File: `pkg/middleware/merchant_rate_limiter.go`**
```go
type MerchantRateLimiter struct {
    limiters sync.Map
    tiers    map[string]RateLimit
}

type RateLimit struct {
    RequestsPerSecond int
    Burst            int
}

var tierLimits = map[string]RateLimit{
    "free":       {RequestsPerSecond: 10, Burst: 20},
    "standard":   {RequestsPerSecond: 50, Burst: 100},
    "premium":    {RequestsPerSecond: 200, Burst: 400},
    "enterprise": {RequestsPerSecond: 1000, Burst: 2000},
}
```

**Add merchant tier to database:**
```sql
ALTER TABLE merchants ADD COLUMN tier VARCHAR(20) DEFAULT 'standard';
```

---

### Phase 6: Audit Logging (MEDIUM - 6 hours)

**Goal:** Comprehensive audit trail for compliance

**Database:**
```sql
CREATE TABLE audit_log (
    id UUID PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    subject VARCHAR(255) NOT NULL,
    merchant_id UUID,
    action VARCHAR(100) NOT NULL,
    resource_id UUID,
    success BOOLEAN NOT NULL,
    ip_address INET,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
) PARTITION BY RANGE (timestamp);
```

**Log all payment operations:**
```go
auditLog.Log(ctx, AuditEvent{
    Subject:    claims.Subject,
    MerchantID: merchantID,
    Action:     "payment.sale",
    ResourceID: transactionID,
    Success:    true,
    IPAddress:  getClientIP(ctx),
})
```

---

## Implementation Timeline

### Week 1: Critical Security
| Task | Time | Priority |
|------|------|----------|
| Phase 1: Enable auth interceptor | 1 hour | P0 |
| Phase 2: HTTP endpoint protection | 2 hours | P0 |
| Phase 3: Token blacklist (Redis) | 4 hours | P0 |
| **Total Week 1** | **7 hours** | |

### Week 2: Token Management
| Task | Time | Priority |
|------|------|----------|
| Phase 4: Refresh token system | 8 hours | P1 |
| Phase 5: Per-merchant rate limiting | 4 hours | P1 |
| **Total Week 2** | **12 hours** | |

### Week 3: Observability
| Task | Time | Priority |
|------|------|----------|
| Phase 6: Audit logging | 6 hours | P2 |
| Documentation updates | 2 hours | P2 |
| Testing & QA | 4 hours | P2 |
| **Total Week 3** | **12 hours** | |

**Total Effort: ~31 hours (4 work days)**

---

## Quick Win: Enable Auth Now (< 1 hour)

**Minimal changes to go production-ready:**

```bash
# 1. Create keys directory
mkdir -p secrets/clients

# 2. Generate WordPress keys
openssl genrsa -out wordpress_private.pem 2048
openssl rsa -in wordpress_private.pem -pubout -out secrets/clients/wordpress.pem

# 3. Add to main.go (paste code from Phase 1)

# 4. Rebuild & restart
docker-compose down
docker-compose build
docker-compose up -d

# 5. Update WordPress to send JWT tokens
```

**Result:** Service now requires authentication! 🔒

---

## Recommendations Summary

### Immediate (Do This Week)
1. ✅ **Enable auth interceptor** - Protects gRPC/REST endpoints
2. ✅ **Add HTTP auth middleware** - Protects Browser Post
3. ✅ **Setup Redis blacklist** - Can revoke tokens

### Soon (Next 2 Weeks)
4. ✅ **Implement refresh tokens** - Reduce token exposure window
5. ✅ **Add per-merchant rate limiting** - Prevent abuse

### Later (Future Sprints)
6. ✅ **Comprehensive audit logging** - Compliance & forensics
7. ✅ **Dynamic key management** - Zero-downtime key rotation
8. ✅ **Anomaly detection** - Automated threat detection

---

## Conclusion

**Current State:**
- Auth code needs adaptation for ConnectRPC ⚡
- ConnectRPC interceptors simpler than gRPC ✅
- Can go from "open" to "secure" in < 1 hour

**Recommendation:**
**Enable Phase 1 immediately.** This gives you production-grade security with minimal effort. Then implement phases 2-6 incrementally.

**Key Advantages with ConnectRPC:**
- Single interceptor model for all protocols ✅
- Simpler error handling with connect.NewError() ✅
- Better browser compatibility (no grpc-web needed) ✅
- Unified authentication across gRPC/HTTP/Connect protocols ✅

**Questions?**
- Is this ConnectRPC? **Yes, unified ConnectRPC server as of latest migration**
- Is auth ready? **Needs minor adaptation, then can be enabled**
- How long to implement? **1 hour for critical, 4 days for complete**
