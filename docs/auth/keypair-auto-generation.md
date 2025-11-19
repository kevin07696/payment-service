# RSA Keypair Auto-Generation for Services

**Status**: Planning
**Created**: 2025-11-19
**Owner**: Backend Team

## Overview

Currently, services must generate their own RSA keypairs and provide the public key during registration. This document outlines a simplified approach where the payment service auto-generates keypairs during service creation, ensuring consistency and simplifying onboarding.

## Current State

### Service Registration Flow (Manual Keypair)
```
1. Service generates RSA keypair locally
   $ openssl genrsa -out private.pem 2048
   $ openssl rsa -in private.pem -pubout -out public.pem

2. Admin registers service with public key
   POST /admin/services
   {
     "service_id": "acme-web-app",
     "public_key": "<contents of public.pem>"
   }

3. Service stores private key
4. Payment service stores public key in DB
```

### Problems with Current Approach
- Services must understand RSA key generation
- Inconsistent key formats/strengths across services
- Additional setup complexity for service developers
- No guarantee of key quality/security

## Proposed State

### Auto-Generation Flow
```
1. Admin creates service via admin panel
   POST /admin/services
   {
     "service_id": "acme-web-app",
     "service_name": "ACME Web App"
   }

2. Payment service auto-generates RSA keypair
3. Payment service stores public key in DB
4. Payment service returns private key (ONE-TIME ONLY)
5. Admin saves private key securely
6. Admin provides private key to service team
```

### Benefits
✅ Consistent key generation (2048-bit RSA, PKCS#1/PKIX format)
✅ Simplified service onboarding
✅ Guaranteed key strength/security
✅ Single source of truth for key generation
✅ Easy to add key rotation later

---

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Admin Panel                          │
│  - Service creation form                                │
│  - One-time private key display                         │
│  - Key rotation UI                                      │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│              Admin Service (ConnectRPC)                 │
│  - CreateService(req) → (service, privateKey)           │
│  - RotateServiceKey(serviceId) → (service, privateKey)  │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│              Crypto Package (pkg/crypto)                │
│  - GenerateRSAKeyPair() → (private, public, fingerprint)│
│  - VerifyJWT(token, publicKey) → valid                  │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│                  Database (services)                    │
│  - public_key (TEXT)                                    │
│  - public_key_fingerprint (VARCHAR)                     │
│  - (private key NEVER stored)                           │
└─────────────────────────────────────────────────────────┘
```

### Key Generation Algorithm

**Algorithm**: RSA-2048
**Private Key Format**: PKCS#1 PEM (`-----BEGIN RSA PRIVATE KEY-----`)
**Public Key Format**: PKIX PEM (`-----BEGIN PUBLIC KEY-----`)
**Fingerprint**: SHA-256 hash of public key DER bytes (hex-encoded)

---

## Implementation Plan

### 1. Create Crypto Package

**File**: `pkg/crypto/keypair.go`

```go
package crypto

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "crypto/x509"
    "encoding/hex"
    "encoding/pem"
    "fmt"
)

// KeyPair represents an RSA keypair with fingerprint
type KeyPair struct {
    PrivateKeyPEM string
    PublicKeyPEM  string
    Fingerprint   string
}

// GenerateRSAKeyPair generates a new 2048-bit RSA keypair
func GenerateRSAKeyPair() (*KeyPair, error) {
    // Generate 2048-bit RSA keypair
    privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return nil, fmt.Errorf("failed to generate RSA key: %w", err)
    }

    // Encode private key to PKCS#1 PEM format
    privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
    privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{
        Type:  "RSA PRIVATE KEY",
        Bytes: privateKeyBytes,
    }))

    // Encode public key to PKIX PEM format
    publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal public key: %w", err)
    }
    publicKeyPEM := string(pem.EncodeToMemory(&pem.Block{
        Type:  "PUBLIC KEY",
        Bytes: publicKeyBytes,
    }))

    // Generate fingerprint (SHA-256 hash of public key DER bytes)
    hash := sha256.Sum256(publicKeyBytes)
    fingerprint := hex.EncodeToString(hash[:])

    return &KeyPair{
        PrivateKeyPEM: privateKeyPEM,
        PublicKeyPEM:  publicKeyPEM,
        Fingerprint:   fingerprint,
    }, nil
}

// ParsePublicKey parses a PEM-encoded public key
func ParsePublicKey(publicKeyPEM string) (*rsa.PublicKey, error) {
    block, _ := pem.Decode([]byte(publicKeyPEM))
    if block == nil {
        return nil, fmt.Errorf("failed to parse PEM block")
    }

    pub, err := x509.ParsePKIXPublicKey(block.Bytes)
    if err != nil {
        return nil, fmt.Errorf("failed to parse public key: %w", err)
    }

    rsaPub, ok := pub.(*rsa.PublicKey)
    if !ok {
        return nil, fmt.Errorf("not an RSA public key")
    }

    return rsaPub, nil
}
```

**Tests**: `pkg/crypto/keypair_test.go`

```go
package crypto_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "your-module/pkg/crypto"
)

func TestGenerateRSAKeyPair(t *testing.T) {
    kp, err := crypto.GenerateRSAKeyPair()
    require.NoError(t, err)
    require.NotNil(t, kp)

    // Verify private key format
    assert.Contains(t, kp.PrivateKeyPEM, "-----BEGIN RSA PRIVATE KEY-----")
    assert.Contains(t, kp.PrivateKeyPEM, "-----END RSA PRIVATE KEY-----")

    // Verify public key format
    assert.Contains(t, kp.PublicKeyPEM, "-----BEGIN PUBLIC KEY-----")
    assert.Contains(t, kp.PublicKeyPEM, "-----END PUBLIC KEY-----")

    // Verify fingerprint (64 hex chars = 32 bytes SHA-256)
    assert.Len(t, kp.Fingerprint, 64)

    // Verify we can parse the public key
    pubKey, err := crypto.ParsePublicKey(kp.PublicKeyPEM)
    require.NoError(t, err)
    assert.NotNil(t, pubKey)
}

func TestGenerateRSAKeyPair_Uniqueness(t *testing.T) {
    kp1, err := crypto.GenerateRSAKeyPair()
    require.NoError(t, err)

    kp2, err := crypto.GenerateRSAKeyPair()
    require.NoError(t, err)

    // Each generation should produce unique keys
    assert.NotEqual(t, kp1.PrivateKeyPEM, kp2.PrivateKeyPEM)
    assert.NotEqual(t, kp1.PublicKeyPEM, kp2.PublicKeyPEM)
    assert.NotEqual(t, kp1.Fingerprint, kp2.Fingerprint)
}
```

---

### 2. Update Proto Definitions

**File**: `api/proto/admin/v1/admin.proto`

```protobuf
// CreateService request - no public_key needed
message CreateServiceRequest {
  string service_id = 1;
  string service_name = 2;
  string environment = 3;  // staging, production
  int32 requests_per_second = 4;
  int32 burst_limit = 5;
}

// CreateService response - includes private key (ONE-TIME ONLY)
message CreateServiceResponse {
  Service service = 1;
  string private_key = 2;  // ⚠️ ONLY SHOWN ONCE - MUST BE SAVED
  string message = 3;      // Warning message
}

// RotateServiceKey request
message RotateServiceKeyRequest {
  string service_id = 1;
  string reason = 2;  // Audit trail
}

// RotateServiceKey response
message RotateServiceKeyResponse {
  Service service = 1;
  string private_key = 2;  // New private key (ONE-TIME ONLY)
  string message = 3;
}

message Service {
  string id = 1;
  string service_id = 2;
  string service_name = 3;
  string public_key_fingerprint = 4;  // NOT the full key
  string environment = 5;
  int32 requests_per_second = 6;
  int32 burst_limit = 7;
  bool is_active = 8;
  google.protobuf.Timestamp created_at = 9;
  google.protobuf.Timestamp updated_at = 10;
}

service AdminService {
  rpc CreateService(CreateServiceRequest) returns (CreateServiceResponse);
  rpc RotateServiceKey(RotateServiceKeyRequest) returns (RotateServiceKeyResponse);
  // ... other endpoints
}
```

**Generate Proto**:
```bash
make proto
```

---

### 3. Update Admin Service Handler

**File**: `internal/handlers/admin/service_handler.go`

```go
package admin

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"
    adminv1 "your-module/api/proto/admin/v1"
    "your-module/internal/db/sqlc"
    "your-module/pkg/crypto"
    "google.golang.org/protobuf/types/known/timestamppb"
)

func (h *AdminHandler) CreateService(
    ctx context.Context,
    req *adminv1.CreateServiceRequest,
) (*adminv1.CreateServiceResponse, error) {
    // Validate request
    if req.ServiceId == "" {
        return nil, fmt.Errorf("service_id is required")
    }
    if req.ServiceName == "" {
        return nil, fmt.Errorf("service_name is required")
    }

    // Auto-generate RSA keypair
    keypair, err := crypto.GenerateRSAKeyPair()
    if err != nil {
        return nil, fmt.Errorf("failed to generate keypair: %w", err)
    }

    // Create service in database (store only public key)
    service, err := h.queries.CreateService(ctx, sqlc.CreateServiceParams{
        ID:                   uuid.New(),
        ServiceID:            req.ServiceId,
        ServiceName:          req.ServiceName,
        PublicKey:            keypair.PublicKeyPEM,
        PublicKeyFingerprint: keypair.Fingerprint,
        Environment:          req.Environment,
        RequestsPerSecond: pgtype.Int4{
            Int32: req.RequestsPerSecond,
            Valid: req.RequestsPerSecond > 0,
        },
        BurstLimit: pgtype.Int4{
            Int32: req.BurstLimit,
            Valid: req.BurstLimit > 0,
        },
        IsActive: pgtype.Bool{Bool: true, Valid: true},
        CreatedBy: pgtype.UUID{
            Bytes: h.getAdminID(ctx), // from auth middleware
            Valid: true,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create service: %w", err)
    }

    // Audit log
    h.auditLog(ctx, "service.created", service.ID.String(), map[string]interface{}{
        "service_id":   service.ServiceID,
        "service_name": service.ServiceName,
        "environment":  service.Environment,
        "fingerprint":  service.PublicKeyFingerprint,
    })

    // Return service + private key (ONE-TIME ONLY)
    return &adminv1.CreateServiceResponse{
        Service: &adminv1.Service{
            Id:                   service.ID.String(),
            ServiceId:            service.ServiceID,
            ServiceName:          service.ServiceName,
            PublicKeyFingerprint: service.PublicKeyFingerprint,
            Environment:          service.Environment,
            RequestsPerSecond:    service.RequestsPerSecond.Int32,
            BurstLimit:           service.BurstLimit.Int32,
            IsActive:             service.IsActive.Bool,
            CreatedAt:            timestamppb.New(service.CreatedAt.Time),
            UpdatedAt:            timestamppb.New(service.UpdatedAt.Time),
        },
        PrivateKey: keypair.PrivateKeyPEM, // ⚠️ ONLY SHOWN ONCE
        Message:    "⚠️  SAVE THIS PRIVATE KEY - IT WILL NOT BE SHOWN AGAIN!",
    }, nil
}

func (h *AdminHandler) RotateServiceKey(
    ctx context.Context,
    req *adminv1.RotateServiceKeyRequest,
) (*adminv1.RotateServiceKeyResponse, error) {
    // Validate request
    if req.ServiceId == "" {
        return nil, fmt.Errorf("service_id is required")
    }

    // Generate new keypair
    keypair, err := crypto.GenerateRSAKeyPair()
    if err != nil {
        return nil, fmt.Errorf("failed to generate keypair: %w", err)
    }

    // Get service UUID from service_id
    oldService, err := h.queries.GetServiceByServiceID(ctx, req.ServiceId)
    if err != nil {
        return nil, fmt.Errorf("service not found: %w", err)
    }

    // Update service with new public key
    service, err := h.queries.RotateServiceKey(ctx, sqlc.RotateServiceKeyParams{
        ID:                   oldService.ID,
        PublicKey:            keypair.PublicKeyPEM,
        PublicKeyFingerprint: keypair.Fingerprint,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to rotate key: %w", err)
    }

    // Audit log
    h.auditLog(ctx, "service.key_rotated", service.ID.String(), map[string]interface{}{
        "service_id":          service.ServiceID,
        "old_fingerprint":     oldService.PublicKeyFingerprint,
        "new_fingerprint":     service.PublicKeyFingerprint,
        "rotation_reason":     req.Reason,
    })

    return &adminv1.RotateServiceKeyResponse{
        Service: &adminv1.Service{
            Id:                   service.ID.String(),
            ServiceId:            service.ServiceID,
            ServiceName:          service.ServiceName,
            PublicKeyFingerprint: service.PublicKeyFingerprint,
            Environment:          service.Environment,
            RequestsPerSecond:    service.RequestsPerSecond.Int32,
            BurstLimit:           service.BurstLimit.Int32,
            IsActive:             service.IsActive.Bool,
            CreatedAt:            timestamppb.New(service.CreatedAt.Time),
            UpdatedAt:            timestamppb.New(service.UpdatedAt.Time),
        },
        PrivateKey: keypair.PrivateKeyPEM, // ⚠️ NEW PRIVATE KEY
        Message:    "⚠️  KEY ROTATED - SAVE NEW PRIVATE KEY AND UPDATE SERVICE CONFIG!",
    }, nil
}
```

---

### 4. Database Changes

**No schema changes needed** - current `services` table already supports this:

```sql
CREATE TABLE services (
    id UUID PRIMARY KEY,
    service_id VARCHAR(100) UNIQUE NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    public_key TEXT NOT NULL,              -- Auto-generated
    public_key_fingerprint VARCHAR(64) NOT NULL,  -- Auto-generated
    environment VARCHAR(50) NOT NULL,
    requests_per_second INTEGER DEFAULT 100,
    burst_limit INTEGER DEFAULT 200,
    is_active BOOLEAN DEFAULT true,
    created_by UUID REFERENCES admins(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

**Existing SQLC Query** (already exists in `internal/db/queries/services.sql`):

```sql
-- name: RotateServiceKey :one
UPDATE services
SET
    public_key = sqlc.arg(public_key),
    public_key_fingerprint = sqlc.arg(public_key_fingerprint),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;
```

---

### 5. Admin CLI Tool

**File**: `cmd/admin/service.go`

```go
package main

import (
    "context"
    "fmt"
    "os"
    adminv1 "your-module/api/proto/admin/v1"
)

func createService(serviceID, serviceName, environment string) error {
    client := getAdminClient() // ConnectRPC client

    resp, err := client.CreateService(context.Background(), &adminv1.CreateServiceRequest{
        ServiceId:          serviceID,
        ServiceName:        serviceName,
        Environment:        environment,
        RequestsPerSecond:  100,
        BurstLimit:         200,
    })
    if err != nil {
        return fmt.Errorf("failed to create service: %w", err)
    }

    // Display service info
    fmt.Printf("✅ Service created successfully!\n\n")
    fmt.Printf("Service ID:   %s\n", resp.Service.ServiceId)
    fmt.Printf("Service Name: %s\n", resp.Service.ServiceName)
    fmt.Printf("Fingerprint:  %s\n", resp.Service.PublicKeyFingerprint)
    fmt.Printf("Environment:  %s\n\n", resp.Service.Environment)

    // Display private key warning
    fmt.Printf("⚠️  %s\n\n", resp.Message)
    fmt.Printf("PRIVATE KEY:\n")
    fmt.Printf("─────────────────────────────────────────────────────\n")
    fmt.Printf("%s\n", resp.PrivateKey)
    fmt.Printf("─────────────────────────────────────────────────────\n\n")

    // Save to file
    filename := fmt.Sprintf("%s.pem", serviceID)
    if err := os.WriteFile(filename, []byte(resp.PrivateKey), 0600); err != nil {
        return fmt.Errorf("failed to save private key: %w", err)
    }

    fmt.Printf("💾 Private key saved to: %s\n", filename)
    fmt.Printf("🔒 File permissions: 0600 (read/write for owner only)\n\n")

    return nil
}

// Usage:
// ./admin create-service --service-id=acme-web-app --name="ACME Web App" --env=production
```

---

### 6. Service Integration Example

**File**: `examples/service-auth/main.go`

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
)

func main() {
    // Load private key (provided by admin during service creation)
    privateKey, err := loadPrivateKey("acme-web-app.pem")
    if err != nil {
        panic(err)
    }

    // Create service token (JWT signed with private key)
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
        "iss": "acme-web-app",           // service_id
        "aud": "payment-service",
        "exp": time.Now().Add(15 * time.Minute).Unix(),
        "iat": time.Now().Unix(),
    })

    // Sign token with private key
    signedToken, err := token.SignedString(privateKey)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Service Token: %s\n", signedToken)

    // Use token in API calls
    // req.Header.Set("Authorization", "Bearer "+signedToken)
}

func loadPrivateKey(filename string) (*rsa.PrivateKey, error) {
    keyBytes, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to read private key: %w", err)
    }

    block, _ := pem.Decode(keyBytes)
    if block == nil {
        return nil, fmt.Errorf("failed to parse PEM block")
    }

    privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
    if err != nil {
        return nil, fmt.Errorf("failed to parse private key: %w", err)
    }

    return privateKey, nil
}
```

---

## Security Considerations

### Private Key Handling

1. **Never stored in database** - Only public key is stored
2. **Shown only once** - During service creation/rotation
3. **Transmitted securely** - Over TLS/HTTPS only
4. **Admin responsibility** - Admin must save and secure the private key
5. **File permissions** - Saved with 0600 (owner read/write only)

### Key Rotation

1. **Manual rotation** - Admin triggers via `RotateServiceKey` endpoint
2. **Audit trail** - All rotations logged with reason
3. **Graceful migration** - Services must update private key in config
4. **No automatic invalidation** - Old tokens remain valid until expiry (15 min)

### Fingerprint Verification

1. **Display in admin panel** - Show fingerprint for verification
2. **Service logs fingerprint** - Services log their key fingerprint on startup
3. **Mismatch detection** - If service fingerprint doesn't match DB → alert

---

## Migration Strategy

### For New Services
- Use auto-generation from day 1
- Admin creates service → receives private key → provides to service

### For Existing Services (if any)
1. **Phase 1**: Add auto-generation alongside manual key registration
2. **Phase 2**: Notify existing services to migrate
3. **Phase 3**: Admin rotates keys using new endpoint
4. **Phase 4**: Remove manual key registration option

---

## Testing Strategy

### Unit Tests

```go
// pkg/crypto/keypair_test.go
- TestGenerateRSAKeyPair
- TestGenerateRSAKeyPair_Uniqueness
- TestParsePublicKey
- TestParsePublicKey_InvalidFormat

// internal/handlers/admin/service_handler_test.go
- TestCreateService_Success
- TestCreateService_ReturnsPrivateKey
- TestCreateService_StoresPublicKeyOnly
- TestRotateServiceKey_Success
- TestRotateServiceKey_AuditsRotation
```

### Integration Tests

```go
// tests/integration/admin/service_test.go
- TestCreateService_EndToEnd
  1. Create service via admin API
  2. Verify service in DB (public key stored)
  3. Verify private key returned
  4. Use private key to sign JWT
  5. Verify JWT with public key from DB

- TestRotateServiceKey_EndToEnd
  1. Create service
  2. Rotate key
  3. Verify old JWT fails
  4. Verify new JWT succeeds
```

### Security Tests

```go
// tests/security/keypair_test.go
- TestPrivateKey_NotStoredInDatabase
- TestPrivateKey_NotLoggedAnywhere
- TestKeyGeneration_MinimumStrength (2048-bit)
- TestFingerprint_UniquePerKey
```

---

## Rollout Plan

### Phase 1: Implementation (Week 1)
- [ ] Create `pkg/crypto/keypair.go`
- [ ] Write unit tests
- [ ] Update proto definitions
- [ ] Regenerate proto code
- [ ] Update admin service handler
- [ ] Write handler tests

### Phase 2: CLI Tool (Week 1)
- [ ] Create admin CLI command
- [ ] Add file saving functionality
- [ ] Test end-to-end flow

### Phase 3: Documentation (Week 2)
- [ ] Update service onboarding docs
- [ ] Create key rotation runbook
- [ ] Update security best practices
- [ ] Add examples for service integration

### Phase 4: Testing (Week 2)
- [ ] Integration tests
- [ ] Security audit
- [ ] Load testing (key generation performance)
- [ ] Manual QA

### Phase 5: Deployment (Week 3)
- [ ] Deploy to staging
- [ ] Create test service
- [ ] Verify end-to-end flow
- [ ] Deploy to production
- [ ] Monitor for issues

---

## Open Questions

1. **Key Size**: Should we support 4096-bit keys for high-security services?
2. **Key Expiry**: Should we enforce automatic key rotation (e.g., every 90 days)?
3. **Backup Strategy**: Should we support key backup/recovery for services?
4. **Multi-Key Support**: Should services support multiple active keys during rotation?
5. **HSM Integration**: Should we support Hardware Security Modules for key generation?

---

## Success Metrics

- [ ] 100% of new services use auto-generated keys
- [ ] Key generation time < 100ms
- [ ] Zero private keys found in logs/database
- [ ] All key rotations successfully audited
- [ ] Service onboarding time reduced by 50%

---

## References

- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
- [RSA Key Management](https://www.rfc-editor.org/rfc/rfc3447)
- [Go crypto/rsa Documentation](https://pkg.go.dev/crypto/rsa)
- Current implementation: `internal/db/queries/services.sql`
