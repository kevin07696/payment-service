# Token Generation Guide

**Audience:** Developers integrating external applications (WordPress, custom e-commerce, mobile apps)
**Topic:** Service registration and JWT token generation for API authentication
**Goal:** Successfully authenticate API requests using service-based JWT tokens

---

## Overview

The payment service uses **RSA-signed JWT tokens** for authentication. Each external application (service) receives:

1. **Service ID**: Unique identifier (e.g., `acme-web-app`)
2. **RSA Key Pair**: Private key for signing tokens, public key stored in payment service
3. **Merchant Access**: Which merchants this service can transact for (via `grant-access`)
4. **Scopes**: Permissions (e.g., `payments:create`, `payments:read`)

### Key Architecture Points

- **Token identifies the SERVICE** (via `iss`/`sub` claims)
- **Merchant is specified per-request** (in request body, NOT in token)
- **One token works for all merchants** the service has access to
- **Database validates access** per-request based on service→merchant grants

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
       │    POST /payment.v1/Sale        │                                │
       │    Body: {merchant_id: "..."}   │                                │
       │─────────────────────────────>  │                                │
       │                                 │                                │
       │                                 │ 3. Verify JWT signature        │
       │                                 │    Check service→merchant access│
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

## Step 1: Service Registration (Admin CLI)

An administrator must register your application as a service before you can generate tokens.

### Create Service

**Docker (recommended):**

```bash
# Create service interactively
podman exec -it payment-server ./paycli -action=create-service

# Or with JSON file
podman exec payment-server sh -c 'cat > service.json << EOF
{
  "service_id": "acme-web-app",
  "service_name": "ACME Corp Web Application",
  "environment": "production",
  "generate_keypair": true
}
EOF'
podman exec payment-server ./paycli -action=create-service -json=service.json

# Copy credentials to host
podman cp payment-server:/home/appuser/service_acme-web-app_credentials.json .
```

**Local:**

```bash
# Build CLI
go build -o bin/paycli ./cmd/paycli

# Set database URL
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"

# Create service
./bin/paycli -action=create-service
```

### Output: Credentials File

The admin CLI outputs a credentials file (`service_acme-web-app_credentials.json`):

```json
{
  "service_id": "acme-web-app",
  "service_name": "ACME Corp Web Application",
  "environment": "production",
  "private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...",
  "public_key": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...",
  "note": "Keep the private key secure! Use it to sign JWT tokens."
}
```

**CRITICAL:** Save the credentials file immediately. The private key will **never be shown again**.

### Grant Merchant Access

After creating a service and merchant, grant the service access to the merchant:

```bash
# Interactive
./bin/paycli -action=grant-access
# Enter: service_id, merchant_slug

# This creates an entry in service_merchants table with scopes
```

---

## Step 2: Token Structure

### JWT Claims (Service-Only)

Tokens identify the **service only**. The merchant is specified in each API request body.

```json
{
  "iss": "acme-web-app",
  "sub": "acme-web-app",
  "scopes": ["payments:create", "payments:read", "payments:refund"],
  "exp": 1736683500,
  "iat": 1736683200,
  "nbf": 1736683200,
  "jti": "550e8400-e29b-41d4-a716-446655440000"
}
```

| Claim | Required | Description |
|-------|----------|-------------|
| `iss` | Yes | Issuer - your service_id |
| `sub` | Yes | Subject - your service_id |
| `scopes` | Yes | Array of permission scopes |
| `exp` | Yes | Expiration timestamp (Unix) |
| `iat` | Yes | Issued at timestamp (Unix) |
| `nbf` | Yes | Not before timestamp (Unix) |
| `jti` | Yes | Unique JWT ID (UUID, prevents replay) |

**Note:** `merchant_id` is NOT in the token. It's passed in the request body.

---

## Step 3: Generate JWT Tokens

### Using Admin CLI (Recommended)

The admin CLI includes token generation. No database connection required.

```bash
# Build CLI (if not already built)
go build -o bin/paycli ./cmd/paycli

# Generate token with default settings (1h expiry, all scopes)
./bin/paycli -action=generate-token -c service_acme-web-app_credentials.json

# Custom expiry duration
./bin/paycli -action=generate-token -c creds.json -e 30m

# Specific scopes only
./bin/paycli -action=generate-token -c creds.json -s "payments:create,payments:read"

# Output as JSON (includes metadata)
./bin/paycli -action=generate-token -c creds.json -o json

# Output as ready-to-use curl command
./bin/paycli -action=generate-token -c creds.json -o curl

# Verify token by showing decoded claims
./bin/paycli -action=generate-token -c creds.json --decode
```

**Admin CLI Token Options:**

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--credentials` | `-c` | Service credentials JSON file | (required) |
| `--expires` | `-e` | Token expiry duration | `1h` |
| `--scopes` | `-s` | Comma-separated scopes | all scopes |
| `--output` | `-o` | Output format: `token`, `json`, `curl` | `token` |
| `--decode` | | Show decoded claims | false |

### Example Output

```bash
$ ./bin/paycli -action=generate-token -c service_acme-web-app_credentials.json --decode

eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3NjQxNTU...

# Decoded Claims:
{
  "exp": 1764155382,
  "iat": 1764151782,
  "iss": "acme-web-app",
  "jti": "05549f51-0974-4366-bbce-98e684c9c2c4",
  "nbf": 1764151782,
  "scopes": [
    "payments:create",
    "payments:read",
    "payments:void",
    "payments:refund",
    "payment_methods:read",
    "payment_methods:create",
    "subscriptions:manage",
    "subscriptions:read"
  ],
  "sub": "acme-web-app"
}
```

---

## Step 4: Programmatic Token Generation

For production applications, generate tokens in your code.

### Node.js / TypeScript

```typescript
import * as jwt from 'jsonwebtoken';
import { v4 as uuidv4 } from 'uuid';
import * as fs from 'fs';

interface JWTClaims {
  iss: string;        // Issuer (your service_id)
  sub: string;        // Subject (your service_id)
  scopes: string[];
  exp: number;        // Expiration timestamp
  iat: number;        // Issued at timestamp
  nbf: number;        // Not before timestamp
  jti: string;        // JWT ID (unique)
}

class PaymentTokenGenerator {
  private privateKey: Buffer;
  private serviceId: string;
  private tokenExpiry: number;

  constructor(privateKeyPath: string, serviceId: string, tokenExpirySeconds: number = 3600) {
    this.privateKey = fs.readFileSync(privateKeyPath);
    this.serviceId = serviceId;
    this.tokenExpiry = tokenExpirySeconds;
  }

  generateToken(scopes: string[]): string {
    const now = Math.floor(Date.now() / 1000);

    const claims: JWTClaims = {
      iss: this.serviceId,
      sub: this.serviceId,
      scopes: scopes,
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
  3600  // 1 hour
);

const token = tokenGen.generateToken([
  'payments:create',
  'payments:read',
  'payments:refund',
]);

console.log('JWT Token:', token);
```

### Go

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
	Scopes []string `json:"scopes"`
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
func (tg *TokenGenerator) GenerateToken(scopes []string) (string, error) {
	now := time.Now()
	jti := uuid.New().String()

	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tg.serviceID,
			Subject:   tg.serviceID,
			ExpiresAt: jwt.NewNumericDate(now.Add(tg.expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
		Scopes: scopes,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(tg.privateKey)
}

func main() {
	tokenGen, err := NewTokenGenerator(
		"/secure/path/to/private_key.pem",
		"acme-web-app",
		time.Hour,
	)
	if err != nil {
		panic(err)
	}

	token, err := tokenGen.GenerateToken([]string{
		"payments:create",
		"payments:read",
		"payments:refund",
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("JWT Token: %s\n", token)
}
```

### Python

```python
import jwt
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path

class PaymentTokenGenerator:
    def __init__(self, private_key_path: str, service_id: str, token_expiry_seconds: int = 3600):
        self.private_key = Path(private_key_path).read_text()
        self.service_id = service_id
        self.token_expiry = token_expiry_seconds

    def generate_token(self, scopes: list[str]) -> str:
        now = datetime.now(timezone.utc)

        claims = {
            'iss': self.service_id,
            'sub': self.service_id,
            'scopes': scopes,
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
    3600  # 1 hour
)

token = token_gen.generate_token([
    'payments:create',
    'payments:read',
    'payments:refund',
])

print(f'JWT Token: {token}')
```

### PHP (WordPress)

```php
<?php
require 'vendor/autoload.php';

use Firebase\JWT\JWT;
use Ramsey\Uuid\Uuid;

class PaymentTokenGenerator {
    private $privateKey;
    private $serviceId;
    private $tokenExpiry;

    public function __construct(string $privateKeyPath, string $serviceId, int $tokenExpirySeconds = 3600) {
        $this->privateKey = file_get_contents($privateKeyPath);
        $this->serviceId = $serviceId;
        $this->tokenExpiry = $tokenExpirySeconds;
    }

    public function generateToken(array $scopes): string {
        $now = time();

        $claims = [
            'iss' => $this->serviceId,
            'sub' => $this->serviceId,
            'scopes' => $scopes,
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
    3600  // 1 hour
);

$token = $tokenGen->generateToken([
    'payments:create',
    'payments:read',
    'payments:refund',
]);

echo "JWT Token: " . $token . "\n";
```

---

## Step 5: Make Authenticated API Requests

**Important:** The `merchant_id` is specified in the **request body**, not in the token.

### Using cURL

```bash
# Generate token using admin CLI
TOKEN=$(./bin/paycli -action=generate-token -c service_acme-web-app_credentials.json)

# Sale request - merchant_id in request body
curl -X POST http://localhost:8081/payment.v1.PaymentService/Sale \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
    "customer_id": "cust_123",
    "amount_cents": 9999,
    "currency": "USD",
    "payment_method_id": "pm-uuid-here",
    "idempotency_key": "sale_20250120_001"
  }'

# List transactions request
curl -X POST http://localhost:8081/payment.v1.PaymentService/ListTransactions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
    "limit": 50
  }'
```

### Using ConnectRPC Client (Node.js)

```typescript
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-node';
import { PaymentService } from './gen/payment/v1/payment_connect';

// Create transport with auth interceptor
const transport = createConnectTransport({
  baseUrl: 'http://localhost:8081',
  httpVersion: '2',
  interceptors: [
    (next) => async (req) => {
      // Generate or retrieve cached token
      const token = tokenGen.generateToken([
        'payments:create',
        'payments:read',
      ]);

      req.header.set('Authorization', `Bearer ${token}`);
      return next(req);
    },
  ],
});

// Create client
const client = createPromiseClient(PaymentService, transport);

// Sale request - merchant_id in request body
const response = await client.sale({
  merchantId: '550e8400-e29b-41d4-a716-446655440000',
  customerId: 'cust_123',
  amountCents: 9999,
  currency: 'USD',
  paymentMethodId: 'pm-uuid-here',
  idempotencyKey: 'sale_20250120_001',
});

console.log('Transaction:', response);
```

---

## Complete Workflow Example

```bash
# 1. Start payment service
podman-compose up -d

# 2. Create service (generates RSA keypair)
podman exec -it payment-server ./paycli -action=create-service
# Enter: acme-web-app, ACME Corp, production, yes (generate keypair)

# 3. Create merchant
podman exec -it payment-server ./paycli -action=create-merchant
# Enter: acme-merchant, ACME Store, EPX credentials...

# 4. Grant service access to merchant
podman exec -it payment-server ./paycli -action=grant-access
# Enter: acme-web-app, acme-merchant

# 5. Copy credentials to local machine
podman cp payment-server:/home/appuser/service_acme-web-app_credentials.json .

# 6. Build admin CLI locally (for token generation)
go build -o bin/paycli ./cmd/paycli

# 7. Generate token
TOKEN=$(./bin/paycli -action=generate-token -c service_acme-web-app_credentials.json)

# 8. Make API request
curl -X POST http://localhost:8081/payment.v1.PaymentService/ListTransactions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"merchant_id": "your-merchant-uuid-from-step-3"}'
```

---

## Available Scopes

| Scope | Description |
|-------|-------------|
| `payments:create` | Create payments (authorize, sale, capture) |
| `payments:read` | View transaction details and history |
| `payments:void` | Void authorized or captured payments |
| `payments:refund` | Issue refunds |
| `payment_methods:create` | Store payment methods (tokenization) |
| `payment_methods:read` | View saved payment methods |
| `subscriptions:manage` | Create/update/cancel subscriptions |
| `subscriptions:read` | View subscription details |

---

## Token Management Best Practices

### 1. Token Caching

**Don't generate a new token for every request.** Cache tokens and reuse them until near expiry.

```typescript
class TokenCache {
  private token: string | null = null;
  private expiresAt: Date | null = null;

  getOrCreate(generator: () => string, expirySeconds: number): string {
    const now = new Date();

    // Refresh if expired or expiring soon (30 second buffer)
    if (this.token && this.expiresAt && new Date(this.expiresAt.getTime() - 30000) > now) {
      return this.token;
    }

    // Generate new token
    this.token = generator();
    this.expiresAt = new Date(now.getTime() + expirySeconds * 1000);
    return this.token;
  }
}
```

### 2. Token Expiry

**Recommended expiry times:**
- **Short-lived (5-15 minutes)**: For high-security payment operations
- **Medium-lived (1 hour)**: For typical operations (default)
- **Do NOT use tokens longer than 24 hours**

### 3. Secure Key Storage

**Never:**
- Commit private keys to version control
- Store keys in application code
- Share keys across environments (dev/staging/prod)
- Log or display private keys

**Always:**
- Use environment variables or secret managers
- Set file permissions to 600 (read/write owner only)
- Rotate keys periodically (every 90 days)
- Use separate keys per environment

---

## Troubleshooting

### Error: "Invalid signature"

**Cause:** Private key doesn't match the public key registered with the service.

**Solution:**
1. Verify you're using the correct credentials file
2. Ensure no extra whitespace or line breaks in key
3. Check the service_id matches what was registered

### Error: "Token expired"

**Cause:** Token `exp` claim is in the past.

**Solution:**
1. Ensure server clocks are synchronized (use NTP)
2. Generate tokens with appropriate expiry
3. Implement token caching with refresh logic

### Error: "Permission denied"

**Cause:** Service lacks required scopes for the operation.

**Solution:**
1. Check the scopes in your token match the API operation
2. Regenerate token with required scopes
3. Verify `service_merchants` table has correct scopes

### Error: "Service does not have access to merchant"

**Cause:** Service→merchant grant doesn't exist.

**Solution:**
1. Run `./paycli -action=grant-access`
2. Verify with `./paycli -action=list-services` to see merchant associations

---

## Security Checklist

Before going to production, verify:

- [ ] Private key stored securely (secret manager, not in code)
- [ ] File permissions set to 600 for key files
- [ ] Keys not committed to version control
- [ ] `.gitignore` includes `*.pem`, `*.key`, `*_credentials.json`
- [ ] Token expiry set to reasonable time (1 hour recommended)
- [ ] Token caching implemented to avoid regenerating every request
- [ ] Only necessary scopes requested (principle of least privilege)
- [ ] Error handling includes token refresh logic
- [ ] Separate keys for dev/staging/production environments
- [ ] Key rotation plan in place (every 90 days)

---

## Related Documentation

- [Payment CLI](ADMIN_CLI.md) - Service/merchant management
- [Authentication Guide](../development/AUTH.md) - Detailed auth architecture
- [API Specifications](API_SPECS.md) - Complete API reference
- [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md) - PCI-compliant card collection
- [React Integration](REACT_INTEGRATION.md) - React/ConnectRPC integration

---

**Questions?** Contact the payment service team or open an issue on GitHub.
