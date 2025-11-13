# JWT Signing Strategy - Public/Private Key Architecture

**Version**: 1.0
**Date**: 2025-01-13
**Status**: 🎯 IMPLEMENTATION GUIDE

---

## Overview

**Problem**: How do different services issue and verify JWT tokens securely?

**Solution**: Asymmetric key cryptography (RSA or ECDSA)
- **Private Key**: Used by token issuers to **sign** tokens
- **Public Key**: Used by payment service to **verify** tokens

---

## Architecture

### Token Flow
```
┌─────────────────────┐
│  POS Backend        │  Has: Private Key
│  (Token Issuer)     │  Does: Signs JWT tokens for cashiers
└──────────┬──────────┘
           │ Issues JWT (signed with private key)
           │ {
           │   "merchant_ids": ["merchant_123"],
           │   "token_type": "merchant",
           │   ...
           │ }
           │ Signature: <signed with private key>
           ↓
┌─────────────────────┐
│  Client (POS)       │  Has: JWT token
│                     │  Sends: Authorization: Bearer <token>
└──────────┬──────────┘
           │ API Request with JWT
           ↓
┌─────────────────────┐
│  Payment Service    │  Has: Public Key(s)
│  (Token Verifier)   │  Does: Verifies JWT signature
│                     │  Extracts: merchant_ids, scopes, etc.
└─────────────────────┘
```

---

## Key Management Strategy

### Option 1: **Single Key Pair** (Simple - Development/Small Scale)

**Setup**: One private key, one public key

```
Payment Service:
  - Has: 1 public key (for verification)
  - Verifies: All tokens with this public key

All Token Issuers (POS, E-commerce, Operator):
  - Share: Same private key
  - Sign: All tokens with this key
```

**Pros**:
- ✅ Simple setup
- ✅ Easy key distribution

**Cons**:
- ❌ If key compromised, all services affected
- ❌ Can't revoke individual service's access
- ❌ All services equally trusted

**When to Use**: Development, single organization, low risk

---

### Option 2: **Multiple Key Pairs** (Recommended - Production)

**Setup**: Each issuer has their own key pair

```
Payment Service:
  - Has: Public keys from all issuers
  - Verifies: Token based on "iss" (issuer) claim

POS Backend:
  - Has: Private key #1
  - Issues: JWT with "iss": "pos-backend"
  - Signs: With private key #1

E-commerce Backend:
  - Has: Private key #2
  - Issues: JWT with "iss": "ecommerce-backend"
  - Signs: With private key #2

Operator Service:
  - Has: Private key #3
  - Issues: JWT with "iss": "operator-service"
  - Signs: With private key #3
```

**Pros**:
- ✅ Granular revocation (revoke one issuer)
- ✅ Different trust levels per issuer
- ✅ Better security isolation

**Cons**:
- ⚠️ More keys to manage
- ⚠️ Need key rotation strategy

**When to Use**: Production, multi-tenant, high security

---

## Implementation: Option 2 (Multi-Issuer with Public Keys)

### 1. Key Generation (Per Issuer)

#### Generate RSA Key Pair
```bash
# POS Backend generates their key pair
openssl genrsa -out pos_backend_private.pem 2048
openssl rsa -in pos_backend_private.pem -pubout -out pos_backend_public.pem

# E-commerce Backend generates their key pair
openssl genrsa -out ecommerce_private.pem 2048
openssl rsa -in ecommerce_private.pem -pubout -out ecommerce_public.pem

# Operator Service generates their key pair
openssl genrsa -out operator_private.pem 2048
openssl rsa -in operator_private.pem -pubout -out operator_public.pem
```

#### Or Use ECDSA (Smaller keys, faster)
```bash
# Generate ECDSA P-256 key pair (recommended for production)
openssl ecparam -genkey -name prime256v1 -noout -out pos_backend_private_ec.pem
openssl ec -in pos_backend_private_ec.pem -pubout -out pos_backend_public_ec.pem
```

---

### 2. Token Issuance (Issuer Side)

#### POS Backend Issues Token
```typescript
// pos-backend/src/auth/token-issuer.ts
import jwt from 'jsonwebtoken';
import fs from 'fs';

const PRIVATE_KEY = fs.readFileSync('./keys/pos_backend_private.pem');

export function issueToken(staff: Staff): string {
    const token = jwt.sign(
        {
            // Standard claims
            sub: `pos_${staff.terminalId}`,
            iss: 'pos-backend',  // ✅ Identifies this issuer
            iat: Math.floor(Date.now() / 1000),
            exp: Math.floor(Date.now() / 1000) + (8 * 3600), // 8 hours

            // Custom claims
            token_type: 'merchant',
            merchant_ids: [staff.merchant_id],
            scopes: ['payments:create', 'payments:read', 'payments:void', 'payments:refund'],
        },
        PRIVATE_KEY,
        {
            algorithm: 'RS256',  // RSA with SHA-256
            keyid: 'pos-backend-2025-01',  // Key ID for rotation
        }
    );

    return token;
}
```

#### E-commerce Backend Issues Token
```typescript
// ecommerce-backend/src/auth/token-issuer.ts
import jwt from 'jsonwebtoken';
import fs from 'fs';

const PRIVATE_KEY = fs.readFileSync('./keys/ecommerce_private.pem');

export function issueCustomerToken(customer: Customer): string {
    return jwt.sign(
        {
            sub: customer.id,
            iss: 'ecommerce-backend',  // ✅ Different issuer
            iat: Math.floor(Date.now() / 1000),
            exp: Math.floor(Date.now() / 1000) + (24 * 3600), // 24 hours

            token_type: 'customer',
            merchant_ids: [],
            customer_id: customer.id,
            scopes: ['payments:read', 'payment_methods:read'],
        },
        PRIVATE_KEY,
        { algorithm: 'RS256', keyid: 'ecommerce-2025-01' }
    );
}

export function issueGuestToken(sessionId: string, merchantId: string): string {
    return jwt.sign(
        {
            sub: `guest_${sessionId}`,
            iss: 'ecommerce-backend',
            iat: Math.floor(Date.now() / 1000),
            exp: Math.floor(Date.now() / 1000) + (30 * 60), // 30 min

            token_type: 'guest',
            merchant_ids: [merchantId],
            session_id: sessionId,
            scopes: ['payments:create'],
        },
        PRIVATE_KEY,
        { algorithm: 'RS256', keyid: 'ecommerce-2025-01' }
    );
}
```

---

### 3. Token Verification (Payment Service Side)

#### Store Public Keys
```go
// internal/config/public_keys.go
package config

import (
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "os"
)

type PublicKeyStore struct {
    keys map[string]*rsa.PublicKey  // issuer -> public key
}

func NewPublicKeyStore() (*PublicKeyStore, error) {
    store := &PublicKeyStore{
        keys: make(map[string]*rsa.PublicKey),
    }

    // Load public keys for each issuer
    issuers := []struct {
        name    string
        keyPath string
    }{
        {"pos-backend", "./keys/pos_backend_public.pem"},
        {"ecommerce-backend", "./keys/ecommerce_public.pem"},
        {"operator-service", "./keys/operator_public.pem"},
    }

    for _, issuer := range issuers {
        key, err := loadPublicKey(issuer.keyPath)
        if err != nil {
            return nil, fmt.Errorf("failed to load key for %s: %w", issuer.name, err)
        }
        store.keys[issuer.name] = key
    }

    return store, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
    keyData, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    block, _ := pem.Decode(keyData)
    if block == nil {
        return nil, fmt.Errorf("failed to parse PEM block")
    }

    pub, err := x509.ParsePKIXPublicKey(block.Bytes)
    if err != nil {
        return nil, err
    }

    rsaPub, ok := pub.(*rsa.PublicKey)
    if !ok {
        return nil, fmt.Errorf("not an RSA public key")
    }

    return rsaPub, nil
}

func (s *PublicKeyStore) GetPublicKey(issuer string) (*rsa.PublicKey, error) {
    key, ok := s.keys[issuer]
    if !ok {
        return nil, fmt.Errorf("unknown issuer: %s", issuer)
    }
    return key, nil
}
```

#### ConnectRPC Interceptor with Verification
```go
// internal/middleware/auth_interceptor.go
package middleware

import (
    "context"
    "errors"
    "strings"

    "connectrpc.com/connect"
    "github.com/golang-jwt/jwt/v5"
    "github.com/kevin07696/payment-service/internal/config"
)

type TokenClaims struct {
    jwt.RegisteredClaims
    TokenType   string   `json:"token_type"`
    MerchantIDs []string `json:"merchant_ids"`
    CustomerID  *string  `json:"customer_id"`
    SessionID   *string  `json:"session_id"`
    Scopes      []string `json:"scopes"`
}

type AuthInterceptor struct {
    keyStore *config.PublicKeyStore
}

func NewAuthInterceptor(keyStore *config.PublicKeyStore) *AuthInterceptor {
    return &AuthInterceptor{
        keyStore: keyStore,
    }
}

func (i *AuthInterceptor) Intercept() connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            // Extract Authorization header
            authHeader := req.Header().Get("Authorization")
            if authHeader == "" {
                return nil, connect.NewError(connect.CodeUnauthenticated,
                    errors.New("missing authorization header"))
            }

            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            if tokenString == authHeader {
                return nil, connect.NewError(connect.CodeUnauthenticated,
                    errors.New("invalid authorization format"))
            }

            // Parse token to get issuer claim
            token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{},
                func(token *jwt.Token) (interface{}, error) {
                    // Verify signing method
                    if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
                        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                    }

                    // Get issuer from token
                    claims, ok := token.Claims.(*TokenClaims)
                    if !ok {
                        return nil, errors.New("invalid token claims")
                    }

                    // Get public key for this issuer
                    publicKey, err := i.keyStore.GetPublicKey(claims.Issuer)
                    if err != nil {
                        return nil, fmt.Errorf("unknown issuer: %s", claims.Issuer)
                    }

                    return publicKey, nil
                })

            if err != nil {
                return nil, connect.NewError(connect.CodeUnauthenticated,
                    fmt.Errorf("invalid token: %w", err))
            }

            if !token.Valid {
                return nil, connect.NewError(connect.CodeUnauthenticated,
                    errors.New("token is not valid"))
            }

            claims := token.Claims.(*TokenClaims)

            // Store claims in context
            ctx = context.WithValue(ctx, tokenClaimsKey, claims)

            // Call next handler
            return next(ctx, req)
        }
    }
}
```

---

### 4. Public Key Distribution

#### Option A: Embed Public Keys in Payment Service
```go
// config/public_keys.go
const (
    POS_BACKEND_PUBLIC_KEY = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----`

    ECOMMERCE_PUBLIC_KEY = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----`
)
```

**Pros**: Simple, no external dependencies
**Cons**: Requires redeployment to add/rotate keys

---

#### Option B: Store in Database (Recommended)
```sql
CREATE TABLE issuer_public_keys (
    issuer_name VARCHAR(100) PRIMARY KEY,  -- 'pos-backend', 'ecommerce-backend'
    public_key TEXT NOT NULL,              -- PEM-encoded public key
    algorithm VARCHAR(20) NOT NULL,        -- 'RS256', 'ES256'
    key_id VARCHAR(100),                   -- 'pos-backend-2025-01' (for rotation)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    is_active BOOLEAN DEFAULT true
);

-- Insert public keys
INSERT INTO issuer_public_keys (issuer_name, public_key, algorithm, key_id)
VALUES
('pos-backend', '-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----', 'RS256', 'pos-backend-2025-01'),

('ecommerce-backend', '-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----', 'RS256', 'ecommerce-2025-01');
```

**Pros**: Dynamic key management, no redeployment for rotation
**Cons**: Database dependency for auth

---

#### Option C: JWKS Endpoint (Industry Standard)
Each issuer exposes a JSON Web Key Set endpoint:

```typescript
// POS Backend exposes JWKS
// https://pos-backend.example.com/.well-known/jwks.json
app.get('/.well-known/jwks.json', (req, res) => {
    res.json({
        keys: [
            {
                kty: 'RSA',
                use: 'sig',
                kid: 'pos-backend-2025-01',
                n: 'base64-encoded-modulus',
                e: 'AQAB'
            }
        ]
    });
});
```

Payment service fetches and caches public keys:
```go
// internal/auth/jwks_fetcher.go
func FetchJWKS(issuerURL string) (*JWKSet, error) {
    resp, err := http.Get(issuerURL + "/.well-known/jwks.json")
    // Parse and cache keys
}
```

**Pros**: Industry standard, supports key rotation, discovery
**Cons**: Network dependency, need caching strategy

---

## Key Rotation Strategy

### Graceful Key Rotation
```
1. Generate new key pair (key_id: 'pos-backend-2025-02')
2. Add new public key to payment service (keep old key active)
3. POS backend starts signing with new key (but old tokens still valid)
4. Wait for old tokens to expire (max token lifetime)
5. Remove old public key from payment service
```

**Implementation**:
```sql
-- Add new key
INSERT INTO issuer_public_keys (issuer_name, public_key, algorithm, key_id, is_active)
VALUES ('pos-backend', 'NEW_PUBLIC_KEY', 'RS256', 'pos-backend-2025-02', true);

-- After grace period, deactivate old key
UPDATE issuer_public_keys
SET is_active = false
WHERE issuer_name = 'pos-backend' AND key_id = 'pos-backend-2025-01';
```

---

## Security Best Practices

### 1. **Private Key Protection**
```bash
# Store private keys securely
chmod 600 pos_backend_private.pem

# Use secret management (production)
export PRIVATE_KEY=$(gcloud secrets versions access latest --secret=pos-backend-private-key)

# Never commit private keys to git
echo "*.pem" >> .gitignore
```

### 2. **Token Expiration**
```go
// Short-lived tokens
const (
    MerchantTokenTTL = 8 * time.Hour   // POS tokens
    CustomerTokenTTL = 24 * time.Hour  // Customer tokens
    GuestTokenTTL    = 30 * time.Minute // Guest tokens
    AdminTokenTTL    = 1 * time.Hour   // Admin tokens (very short)
)
```

### 3. **Token Validation Checklist**
```go
func validateToken(claims *TokenClaims) error {
    // 1. Check expiration
    if time.Now().Unix() > claims.ExpiresAt.Unix() {
        return ErrTokenExpired
    }

    // 2. Check issuer is known
    if !isKnownIssuer(claims.Issuer) {
        return ErrUnknownIssuer
    }

    // 3. Check token type
    if claims.TokenType == "" {
        return ErrMissingTokenType
    }

    // 4. Validate merchant_ids based on token type
    if claims.TokenType == "merchant" && len(claims.MerchantIDs) == 0 {
        return ErrInvalidMerchantIDs
    }

    // 5. Check scopes
    if len(claims.Scopes) == 0 {
        return ErrMissingScopes
    }

    return nil
}
```

---

## Environment Configuration

### Payment Service (.env)
```bash
# Option A: File-based public keys
PUBLIC_KEY_DIR=/app/keys

# Option B: Database-based keys
DB_CONNECTION_STRING=postgresql://...

# Option C: JWKS endpoints
ISSUER_POS_BACKEND_JWKS=https://pos-backend.example.com/.well-known/jwks.json
ISSUER_ECOMMERCE_JWKS=https://ecommerce.example.com/.well-known/jwks.json
```

### POS Backend (.env)
```bash
# Private key for signing tokens
PRIVATE_KEY_PATH=/app/keys/pos_backend_private.pem

# Or from secret manager
PRIVATE_KEY=$(gcloud secrets versions access latest --secret=pos-private-key)

# Issuer identity
TOKEN_ISSUER=pos-backend
TOKEN_KEY_ID=pos-backend-2025-01
```

---

## Summary

### Recommended Setup for Production

1. **Use RSA-256 or ECDSA-256** for signing
2. **Multiple key pairs** (one per issuer)
3. **Store public keys in database** for easy rotation
4. **Include `iss` claim** to identify issuer
5. **Include `kid` header** for key rotation
6. **Short token lifetimes** (8h for merchants, 30m for guests)
7. **Graceful key rotation** (keep old keys active during transition)

### Token Verification Flow
```
1. Extract JWT from Authorization header
2. Parse token without verification (get issuer + kid)
3. Fetch public key for issuer+kid
4. Verify signature with public key
5. Validate claims (exp, iss, token_type, etc.)
6. Store claims in context
7. Proceed to handler
```

### Key Files
```
payment-service/
├── keys/
│   ├── pos_backend_public.pem      # Public keys (committed)
│   ├── ecommerce_public.pem
│   └── operator_public.pem

pos-backend/
├── keys/
│   └── pos_backend_private.pem     # Private key (NOT committed)

ecommerce-backend/
├── keys/
│   └── ecommerce_private.pem       # Private key (NOT committed)
```

---

**Ready to implement?** This design gives you:
- ✅ Secure token verification
- ✅ Multi-issuer support
- ✅ Key rotation capability
- ✅ Granular revocation
- ✅ Industry-standard practices

