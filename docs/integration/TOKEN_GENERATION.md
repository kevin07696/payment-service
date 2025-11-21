# Token Generation Guide

**Audience:** Developers integrating external applications (WordPress, custom e-commerce, mobile apps)
**Topic:** Service registration and JWT token generation for API authentication
**Goal:** Successfully authenticate API requests using service-based JWT tokens

---

## Overview

The payment service uses **RSA-signed JWT tokens** for authentication. Each external application (service) receives:

1. **Service ID**: Unique identifier (e.g., `acme-web-app`)
2. **RSA Private Key**: Used to sign JWT tokens
3. **Merchant Access**: Which merchants this service can transact for
4. **Scopes**: Permissions (e.g., `payment:create`, `payment:read`)

**Authentication Flow:**

```
┌─────────────┐                  ┌──────────────┐                 ┌─────────────┐
│  Your App   │                  │   Payment    │                 │     EPX     │
│  (Client)   │                  │   Service    │                 │   Gateway   │
└─────────────┘                  └──────────────┘                 └─────────────┘
       │                                 │                                │
       │ 1. Generate JWT                │                                │
       │    (sign with private key)     │                                │
       │─────────────────────────────>  │                                │
       │                                 │                                │
       │ 2. API Request + JWT            │                                │
       │    POST /payment.v1/Authorize   │                                │
       │─────────────────────────────>  │                                │
       │                                 │                                │
       │                                 │ 3. Verify JWT signature        │
       │                                 │    (using public key)          │
       │                                 │                                │
       │                                 │ 4. Process payment             │
       │                                 │──────────────────────────────>│
       │                                 │                                │
       │                                 │ 5. Gateway response            │
       │                                 │<───────────────────────────────│
       │                                 │                                │
       │ 6. Payment response             │                                │
       │<─────────────────────────────  │                                │
       │                                 │                                │
```

---

## Step 1: Service Registration

### Via Admin API

An administrator must register your application as a service before you can generate tokens.

**Request to Admin:**

```bash
curl -X POST http://localhost:8081/admin/services \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "service_id": "acme-web-app",
    "service_name": "ACME Corp Web Application",
    "environment": "production",
    "requests_per_second": 100,
    "burst_limit": 200
  }'
```

**Response:**

```json
{
  "service": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "service_id": "acme-web-app",
    "service_name": "ACME Corp Web Application",
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...",
    "public_key_fingerprint": "sha256:abc123...",
    "environment": "production",
    "requests_per_second": 100,
    "burst_limit": 200,
    "is_active": true,
    "created_at": "2025-01-20T12:00:00Z"
  },
  "private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...",
  "message": "Service created successfully. SAVE THE PRIVATE KEY - it will not be shown again."
}
```

**⚠️ CRITICAL:** Save the `private_key` immediately. It will **never be shown again**.

### What You Receive

| Field | Description | Example |
|-------|-------------|---------|
| `service_id` | Your application identifier | `acme-web-app` |
| `private_key` | RSA private key (PEM format) | `-----BEGIN RSA PRIVATE KEY-----\n...` |
| `public_key` | Public key (stored in payment service) | `-----BEGIN PUBLIC KEY-----\n...` |
| `public_key_fingerprint` | SHA256 fingerprint for verification | `sha256:abc123...` |

---

## Step 2: Store Your Private Key Securely

### Environment Variables (Recommended)

```bash
# .env file
SERVICE_ID=acme-web-app
JWT_PRIVATE_KEY_PATH=/secure/path/to/private_key.pem
JWT_TOKEN_EXPIRY=300  # 5 minutes
```

**Create the key file:**

```bash
# Save private key to file
cat > /secure/path/to/private_key.pem <<'EOF'
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA...
[Your private key content here]
...
-----END RSA PRIVATE KEY-----
EOF

# Set restrictive permissions
chmod 600 /secure/path/to/private_key.pem
```

### Secret Manager (Production)

For production, use a secret management service:

- **AWS Secrets Manager**: Store in `payment-service/services/acme-web-app/private-key`
- **GCP Secret Manager**: Store in `payment-service/services/acme-web-app/private-key`
- **HashiCorp Vault**: Store in `secret/payment-service/services/acme-web-app/private-key`

---

## Step 3: Generate JWT Tokens

### Token Claims Structure

```json
{
  "iss": "acme-web-app",
  "sub": "merchant_abc123",
  "merchant_id": "merchant_abc123",
  "service_id": "acme-web-app",
  "scopes": ["payment:create", "payment:read", "payment:refund"],
  "env": "production",
  "exp": 1736683500,
  "iat": 1736683200,
  "nbf": 1736683200,
  "jti": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Node.js / TypeScript

**Install dependencies:**

```bash
npm install jsonwebtoken uuid
```

**Generate token:**

```typescript
import * as jwt from 'jsonwebtoken';
import { v4 as uuidv4 } from 'uuid';
import * as fs from 'fs';

interface JWTClaims {
  iss: string;        // Issuer (your service_id)
  sub: string;        // Subject (merchant_id)
  merchant_id: string;
  service_id: string;
  scopes: string[];
  env: string;
  exp: number;        // Expiration timestamp
  iat: number;        // Issued at timestamp
  nbf: number;        // Not before timestamp
  jti: string;        // JWT ID (unique)
}

class PaymentTokenGenerator {
  private privateKey: Buffer;
  private serviceId: string;
  private tokenExpiry: number;

  constructor(privateKeyPath: string, serviceId: string, tokenExpirySeconds: number = 300) {
    this.privateKey = fs.readFileSync(privateKeyPath);
    this.serviceId = serviceId;
    this.tokenExpiry = tokenExpirySeconds;
  }

  generateToken(merchantId: string, scopes: string[]): string {
    const now = Math.floor(Date.now() / 1000);

    const claims: JWTClaims = {
      iss: this.serviceId,
      sub: merchantId,
      merchant_id: merchantId,
      service_id: this.serviceId,
      scopes: scopes,
      env: process.env.ENVIRONMENT || 'production',
      exp: now + this.tokenExpiry,
      iat: now,
      nbf: now,
      jti: uuidv4(),
    };

    return jwt.sign(claims, this.privateKey, {
      algorithm: 'RS256',
    });
  }
}

// Usage
const tokenGen = new PaymentTokenGenerator(
  '/secure/path/to/private_key.pem',
  'acme-web-app',
  300  // 5 minutes
);

const token = tokenGen.generateToken('merchant_abc123', [
  'payment:create',
  'payment:read',
  'payment:refund',
]);

console.log('JWT Token:', token);
```

### Go

**Generate token:**

```go
package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims represents the token claims
type JWTClaims struct {
	jwt.RegisteredClaims
	MerchantID  string   `json:"merchant_id"`
	ServiceID   string   `json:"service_id"`
	Scopes      []string `json:"scopes"`
	Environment string   `json:"env"`
}

// TokenGenerator handles JWT token generation
type TokenGenerator struct {
	privateKey *rsa.PrivateKey
	serviceID  string
	expiry     time.Duration
}

// NewTokenGenerator creates a new token generator
func NewTokenGenerator(privateKeyPath, serviceID string, expiry time.Duration) (*TokenGenerator, error) {
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		privateKey = key.(*rsa.PrivateKey)
	}

	return &TokenGenerator{
		privateKey: privateKey,
		serviceID:  serviceID,
		expiry:     expiry,
	}, nil
}

// GenerateToken creates a new JWT token
func (tg *TokenGenerator) GenerateToken(merchantID string, scopes []string) (string, error) {
	now := time.Now()
	jti := uuid.New().String()

	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tg.serviceID,
			Subject:   merchantID,
			ExpiresAt: jwt.NewNumericDate(now.Add(tg.expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
		MerchantID:  merchantID,
		ServiceID:   tg.serviceID,
		Scopes:      scopes,
		Environment: getEnv("ENVIRONMENT", "production"),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(tg.privateKey)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	tokenGen, err := NewTokenGenerator(
		"/secure/path/to/private_key.pem",
		"acme-web-app",
		5*time.Minute,
	)
	if err != nil {
		panic(err)
	}

	token, err := tokenGen.GenerateToken("merchant_abc123", []string{
		"payment:create",
		"payment:read",
		"payment:refund",
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("JWT Token: %s\n", token)
}
```

### Python

**Install dependencies:**

```bash
pip install pyjwt cryptography
```

**Generate token:**

```python
import jwt
import uuid
from datetime import datetime, timedelta
from pathlib import Path

class PaymentTokenGenerator:
    def __init__(self, private_key_path: str, service_id: str, token_expiry_seconds: int = 300):
        self.private_key = Path(private_key_path).read_text()
        self.service_id = service_id
        self.token_expiry = token_expiry_seconds

    def generate_token(self, merchant_id: str, scopes: list[str]) -> str:
        now = datetime.utcnow()

        claims = {
            'iss': self.service_id,
            'sub': merchant_id,
            'merchant_id': merchant_id,
            'service_id': self.service_id,
            'scopes': scopes,
            'env': 'production',
            'exp': now + timedelta(seconds=self.token_expiry),
            'iat': now,
            'nbf': now,
            'jti': str(uuid.uuid4()),
        }

        return jwt.encode(claims, self.private_key, algorithm='RS256')

# Usage
token_gen = PaymentTokenGenerator(
    '/secure/path/to/private_key.pem',
    'acme-web-app',
    300  # 5 minutes
)

token = token_gen.generate_token('merchant_abc123', [
    'payment:create',
    'payment:read',
    'payment:refund',
])

print(f'JWT Token: {token}')
```

### PHP (WordPress)

**Install dependencies:**

```bash
composer require firebase/php-jwt
```

**Generate token:**

```php
<?php
require 'vendor/autoload.php';

use Firebase\JWT\JWT;
use Ramsey\Uuid\Uuid;

class PaymentTokenGenerator {
    private $privateKey;
    private $serviceId;
    private $tokenExpiry;

    public function __construct(string $privateKeyPath, string $serviceId, int $tokenExpirySeconds = 300) {
        $this->privateKey = file_get_contents($privateKeyPath);
        $this->serviceId = $serviceId;
        $this->tokenExpiry = $tokenExpirySeconds;
    }

    public function generateToken(string $merchantId, array $scopes): string {
        $now = time();

        $claims = [
            'iss' => $this->serviceId,
            'sub' => $merchantId,
            'merchant_id' => $merchantId,
            'service_id' => $this->serviceId,
            'scopes' => $scopes,
            'env' => 'production',
            'exp' => $now + $this->tokenExpiry,
            'iat' => $now,
            'nbf' => $now,
            'jti' => Uuid::uuid4()->toString(),
        ];

        return JWT::encode($claims, $this->privateKey, 'RS256');
    }
}

// Usage
$tokenGen = new PaymentTokenGenerator(
    '/secure/path/to/private_key.pem',
    'acme-web-app',
    300  // 5 minutes
);

$token = $tokenGen->generateToken('merchant_abc123', [
    'payment:create',
    'payment:read',
    'payment:refund',
]);

echo "JWT Token: " . $token . "\n";
```

---

## Step 4: Make Authenticated API Requests

### Using cURL

```bash
# Generate token (use one of the examples above)
TOKEN="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."

# Make API request
curl -X POST http://localhost:8080/payment.v1.PaymentService/Authorize \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "merchant_abc123",
    "customer_id": "customer_456",
    "amount_cents": 9999,
    "currency": "USD",
    "payment_method_id": "pm-uuid-here",
    "idempotency_key": "auth_20250120_001"
  }'
```

### Using ConnectRPC Client (Node.js)

```typescript
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-node';
import { PaymentService } from './gen/payment/v1/payment_connect';

// Create transport with auth interceptor
const transport = createConnectTransport({
  baseUrl: 'http://localhost:8080',
  httpVersion: '2',
  interceptors: [
    (next) => async (req) => {
      // Generate fresh token for each request
      const token = tokenGen.generateToken('merchant_abc123', [
        'payment:create',
        'payment:read',
      ]);

      req.header.set('Authorization', `Bearer ${token}`);
      return next(req);
    },
  ],
});

// Create client
const client = createPromiseClient(PaymentService, transport);

// Make API call
const response = await client.authorize({
  merchantId: 'merchant_abc123',
  customerId: 'customer_456',
  amountCents: 9999,
  currency: 'USD',
  paymentMethodId: 'pm-uuid-here',
  idempotencyKey: 'auth_20250120_001',
});

console.log('Transaction:', response);
```

---

## Token Management Best Practices

### 1. Token Caching

**Don't generate a new token for every request.** Cache tokens and reuse them until expiry.

```typescript
class TokenCache {
  private cache: Map<string, { token: string; expiresAt: Date }> = new Map();

  getOrCreate(merchantId: string, generator: () => string, expirySeconds: number): string {
    const cached = this.cache.get(merchantId);
    const now = new Date();

    // Refresh if expired or expiring soon (30 second buffer)
    if (cached && new Date(cached.expiresAt.getTime() - 30000) > now) {
      return cached.token;
    }

    // Generate new token
    const token = generator();
    const expiresAt = new Date(now.getTime() + expirySeconds * 1000);

    this.cache.set(merchantId, { token, expiresAt });
    return token;
  }
}
```

### 2. Token Expiry

**Recommended expiry times:**
- **Short-lived (5-15 minutes)**: For payment operations
- **Medium-lived (1 hour)**: For read-only operations
- **Do NOT use tokens longer than 24 hours**

### 3. Scope Principle of Least Privilege

Only request scopes your application actually needs:

```typescript
// ✅ Good: Minimal scopes
const scopes = ['payment:create', 'payment:read'];

// ❌ Bad: Requesting everything
const scopes = ['payment:*', 'merchant:*', 'admin:*'];
```

### 4. Secure Key Storage

**Never:**
- ❌ Commit private keys to version control
- ❌ Store keys in application code
- ❌ Share keys across environments (dev/staging/prod)
- ❌ Log or display private keys

**Always:**
- ✅ Use environment variables or secret managers
- ✅ Set file permissions to 600 (read/write owner only)
- ✅ Rotate keys periodically (every 90 days)
- ✅ Use separate keys per environment

### 5. Error Handling

```typescript
try {
  const response = await client.authorize(request);
  return response;
} catch (error) {
  if (error.code === 'UNAUTHENTICATED') {
    // Token expired or invalid - generate new token
    console.error('Authentication failed - token may be expired');
    // Clear token cache and retry
  } else if (error.code === 'PERMISSION_DENIED') {
    // Service lacks required scopes
    console.error('Insufficient permissions:', error.message);
  } else {
    // Other errors
    console.error('API error:', error);
  }
  throw error;
}
```

---

## Available Scopes

| Scope | Description |
|-------|-------------|
| `payment:create` | Create payments (authorize, sale, capture) |
| `payment:read` | View transaction details and history |
| `payment:void` | Void authorized or captured payments |
| `payment:refund` | Issue refunds |
| `payment_method:create` | Store payment methods (tokenization) |
| `payment_method:read` | View saved payment methods |
| `payment_method:update` | Update payment method status |
| `payment_method:delete` | Delete payment methods |
| `subscription:create` | Create recurring subscriptions |
| `subscription:read` | View subscription details |
| `subscription:update` | Update subscriptions |
| `subscription:cancel` | Cancel subscriptions |
| `merchant:read` | View merchant information |

---

## Troubleshooting

### Error: "Invalid signature"

**Cause:** Private key doesn't match the public key registered with the service.

**Solution:**
1. Verify you're using the correct private key
2. Check the key fingerprint matches
3. Ensure no extra whitespace or line breaks in key file

### Error: "Token expired"

**Cause:** Token `exp` claim is in the past.

**Solution:**
1. Ensure server clocks are synchronized (use NTP)
2. Reduce token expiry time
3. Implement token caching with refresh logic

### Error: "Permission denied"

**Cause:** Service lacks required scopes for the operation.

**Solution:**
1. Check the scopes in your token match the API operation
2. Contact admin to update service permissions
3. Verify `service_merchants` table has correct scopes

### Error: "Invalid merchant_id"

**Cause:** Service doesn't have access to the specified merchant.

**Solution:**
1. Verify the merchant ID exists
2. Check `service_merchants` table links your service to the merchant
3. Contact admin to grant merchant access

---

## Security Checklist

Before going to production, verify:

- [ ] Private key stored securely (secret manager, not in code)
- [ ] File permissions set to 600 for key files
- [ ] Keys not committed to version control
- [ ] `.gitignore` includes `*.pem`, `*.key`, `secrets/`
- [ ] Token expiry set to reasonable time (5-15 minutes recommended)
- [ ] Token caching implemented to avoid regenerating every request
- [ ] Only necessary scopes requested (principle of least privilege)
- [ ] Error handling includes token refresh logic
- [ ] Separate keys for dev/staging/production environments
- [ ] Key rotation plan in place (every 90 days)

---

## Next Steps

1. **Service Registration**: Contact payment service admin to register your application
2. **Implementation**: Use code examples above to generate tokens
3. **Testing**: Test with sandbox merchant in staging environment
4. **Production**: Request production service registration and deploy
5. **Monitoring**: Track authentication errors and token expiry rates

---

## Additional Resources

- [API Specifications](../API_SPECS.md) - Complete API reference
- [Authentication Guide](../AUTH.md) - Detailed auth architecture
- [Integration Guide](../INTEGRATION_GUIDE.md) - Full integration walkthrough
- [ConnectRPC Documentation](https://connectrpc.com/docs/) - Client libraries

---

**Questions?** Contact the payment service team or open an issue on GitHub.
