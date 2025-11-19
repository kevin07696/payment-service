# Authentication Guide

**Last Updated**: 2025-11-19 (Consolidated endpoints: 43 → 40)

## Overview

This document provides comprehensive authentication documentation for all API endpoints in the payment service. The payment service uses a multi-tier authentication system with different token types for different actors.

## Table of Contents

1. [Token Types](#token-types)
2. [Authentication Architecture](#authentication-architecture)
3. [Service Authentication](#service-authentication)
4. [Admin Authentication](#admin-authentication)
5. [Customer Authentication](#customer-authentication)
6. [Guest Authentication](#guest-authentication)
7. [Merchant Portal Authentication](#merchant-portal-authentication)
8. [API Endpoint Authentication Matrix](#api-endpoint-authentication-matrix)
9. [Token Issuance Flows](#token-issuance-flows)
10. [Security Best Practices](#security-best-practices)

---

## Token Types

The payment service uses **five distinct token types**, each designed for specific actors and use cases:

| Token Type | Actor | Signed By | Verified By | Lifespan | Use Case |
|------------|-------|-----------|-------------|----------|----------|
| **Service Token** | Apps/Integrations | Service (RSA private key) | Payment Service (public key from DB) | 15 min | Service-to-service API calls |
| **Admin Token** | Admin Users | Payment Service (HMAC secret) | Payment Service (HMAC secret) | 2 hours | Admin panel access |
| **Merchant Portal Token** | Merchant Users | Payment Service (HMAC secret) | Payment Service (HMAC secret) | 2 hours | Merchant dashboard access |
| **Customer Token** | End-User Customers | Payment Service (HMAC secret) | Payment Service (HMAC secret) | 30 min | Customer transaction views |
| **Guest Token** | Anonymous Users | Payment Service (HMAC secret) | Payment Service (HMAC secret) | 5 min | One-time order lookup |

---

## Authentication Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Payment Service API                          │
│                                                                     │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐       │
│  │ Public Routes  │  │ Service Routes │  │  Admin Routes  │       │
│  │ (No Auth)      │  │ (Service Token)│  │ (Admin Token)  │       │
│  └────────────────┘  └────────────────┘  └────────────────┘       │
│                                                                     │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐       │
│  │ Customer Routes│  │  Guest Routes  │  │ Merchant Portal│       │
│  │(Customer Token)│  │  (Guest Token) │  │(Merchant Token)│       │
│  └────────────────┘  └────────────────┘  └────────────────┘       │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
        ┌───────────────────────────────────────────┐
        │     Authentication Middleware             │
        │                                           │
        │  1. Extract token from Authorization     │
        │  2. Determine token type                 │
        │  3. Verify signature                     │
        │  4. Check expiration                     │
        │  5. Validate claims                      │
        │  6. Inject context (user/service ID)     │
        └───────────────────────────────────────────┘
```

---

## Service Authentication

**Who**: Apps, integrations, microservices (internal or external)

**Method**: JWT signed with RSA private key, verified with public key from database

### 1. Service Registration & Keypair Generation

Services must be registered by an admin. During registration, the payment service **auto-generates** an RSA keypair:

```bash
# Admin uses CLI or admin panel to create service
POST /admin/v1/services

Request:
{
  "service_id": "acme-web-app",
  "service_name": "ACME Web Application",
  "environment": "production",
  "requests_per_second": 100,
  "burst_limit": 200
}

Response:
{
  "service": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "service_id": "acme-web-app",
    "service_name": "ACME Web Application",
    "public_key_fingerprint": "a3f2c8d9e5b1a2c4f6d8e9b1c2d3e4f5...",
    "environment": "production",
    "is_active": true
  },
  "private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA2...\n-----END RSA PRIVATE KEY-----",
  "message": "⚠️  SAVE THIS PRIVATE KEY - IT WILL NOT BE SHOWN AGAIN!"
}
```

**Important**:
- Private key is **shown only once** during creation
- Admin must save the private key securely
- Private key is **never stored** in the database
- Public key is stored in the `services` table for JWT verification

See [Keypair Auto-Generation](./auth/keypair-auto-generation.md) for implementation details.

### 2. Service Authentication Flow

```
┌──────────────┐                           ┌─────────────────┐
│   Service    │                           │ Payment Service │
│ (acme-web-app)│                          │                 │
└──────────────┘                           └─────────────────┘
       │                                            │
       │ 1. Load private key from secure storage   │
       │    (e.g., AWS Secrets Manager, k8s secret)│
       │                                            │
       │ 2. Create JWT with claims:                │
       │    {                                       │
       │      "iss": "acme-web-app",                │
       │      "aud": "payment-service",             │
       │      "exp": <15 min from now>,             │
       │      "iat": <current time>                 │
       │    }                                       │
       │                                            │
       │ 3. Sign JWT with RSA private key          │
       │    (RS256 algorithm)                       │
       │                                            │
       │ 4. Send API request with token            │
       │────────────────────────────────────────────>│
       │    Authorization: Bearer <signed-jwt>      │
       │                                            │
       │                                            │ 5. Extract service_id from JWT
       │                                            │ 6. Query DB for public_key
       │                                            │    SELECT public_key
       │                                            │    FROM services
       │                                            │    WHERE service_id = 'acme-web-app'
       │                                            │
       │                                            │ 7. Verify JWT signature using public_key
       │                                            │ 8. Check expiration
       │                                            │ 9. Validate iss, aud claims
       │                                            │
       │ 10. API response                           │
       │<────────────────────────────────────────────│
```

### 3. Service Token Example (Go)

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
    // Load private key from secure storage
    privateKey, err := loadPrivateKey("/secrets/acme-web-app.pem")
    if err != nil {
        panic(err)
    }

    // Create service token
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

    // Use in API requests
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

### 4. Service Token Claims

```json
{
  "iss": "acme-web-app",        // service_id (required)
  "aud": "payment-service",      // audience (required)
  "exp": 1700000000,             // expiration timestamp (required)
  "iat": 1699999000              // issued at timestamp (required)
}
```

### 5. Key Rotation

When a service key is compromised or needs rotation:

```bash
POST /admin/v1/services/acme-web-app/rotate-key

Request:
{
  "service_id": "acme-web-app",
  "reason": "Routine 90-day rotation"
}

Response:
{
  "service": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "service_id": "acme-web-app",
    "public_key_fingerprint": "b4f3d9e0f6c2b3d5g7e9f0c1d2e3f4g5...",  // NEW
    "environment": "production",
    "is_active": true
  },
  "private_key": "-----BEGIN RSA PRIVATE KEY-----\nNEW_KEY...\n-----END RSA PRIVATE KEY-----",
  "message": "⚠️  KEY ROTATED - SAVE NEW PRIVATE KEY AND UPDATE SERVICE CONFIG!"
}
```

**Rotation Process**:
1. Admin rotates key via admin panel/CLI
2. New keypair is generated
3. Old public key is replaced with new public key in DB
4. New private key is returned (shown once)
5. Admin updates service configuration with new private key
6. Service restarts with new private key
7. Old tokens remain valid until expiration (15 min max)

---

## Admin Authentication

**Who**: System administrators

**Method**: JWT signed with HMAC-SHA256, session-based

### 1. Admin Login Flow

```
┌──────────┐                           ┌─────────────────┐
│  Admin   │                           │ Payment Service │
│  User    │                           │                 │
└──────────┘                           └─────────────────┘
       │                                        │
       │ 1. POST /admin/v1/auth/login          │
       │    {email, password}                   │
       │────────────────────────────────────────>│
       │                                        │
       │                                        │ 2. Verify password hash
       │                                        │ 3. Check is_active
       │                                        │ 4. Create admin session
       │                                        │ 5. Generate HMAC-signed JWT
       │                                        │
       │ 6. Return token                        │
       │<────────────────────────────────────────│
       │    {token, expires_at}                 │
       │                                        │
       │ 7. Subsequent requests                 │
       │    Authorization: Bearer <token>       │
       │────────────────────────────────────────>│
       │                                        │
       │                                        │ 8. Verify HMAC signature
       │                                        │ 9. Check session valid
       │                                        │ 10. Check expiration
       │                                        │
       │ 11. API response                       │
       │<────────────────────────────────────────│
```

### 2. Admin Token Claims

```json
{
  "sub": "admin:550e8400-e29b-41d4-a716-446655440000",  // admin_id
  "email": "admin@example.com",
  "role": "super_admin",
  "session_id": "660f9511-f39c-42e5-b827-557766551111",
  "aud": "payment-service",
  "exp": 1700000000,  // 2 hours from issuance
  "iat": 1699993200
}
```

### 3. Admin Logout

```bash
POST /admin/v1/auth/logout

Headers:
  Authorization: Bearer <admin-token>

# Deletes admin session, invalidates token
```

---

## Customer Authentication

**Who**: End-user customers viewing their transactions

**Method**: OAuth-style delegation - Service vouches for customer

### 1. Customer Token Issuance Flow

```
┌──────────┐         ┌──────────────┐         ┌─────────────────┐
│ Customer │         │   Service    │         │ Payment Service │
│(Browser) │         │(acme-web-app)│         │                 │
└──────────┘         └──────────────┘         └─────────────────┘
      │                      │                         │
      │ 1. Login to merchant │                         │
      │    website/app       │                         │
      │─────────────────────>│                         │
      │                      │                         │
      │                      │ 2. Verify customer      │
      │                      │    in merchant system   │
      │                      │                         │
      │                      │ 3. Request customer     │
      │                      │    token from payment   │
      │                      │    service              │
      │                      │                         │
      │                      │ POST /auth/v1/customer-token
      │                      │ Authorization: Bearer <service-token>
      │                      │ {customer_id, merchant_id}
      │                      │─────────────────────────>│
      │                      │                         │
      │                      │                         │ 4. Verify service token
      │                      │                         │ 5. Check service has access to merchant
      │                      │                         │ 6. Generate HMAC-signed customer token
      │                      │                         │
      │                      │ 7. Return customer token│
      │                      │<─────────────────────────│
      │                      │    {token, expires_at}  │
      │                      │                         │
      │ 8. Return token to   │                         │
      │    customer (browser)│                         │
      │<─────────────────────│                         │
      │                      │                         │
      │ 9. Customer uses token to view transactions   │
      │    GET /customer/v1/transactions               │
      │    Authorization: Bearer <customer-token>      │
      │────────────────────────────────────────────────>│
      │                                                │
      │                                                │ 10. Verify customer token
      │                                                │ 11. Check customer_id matches
      │                                                │
      │ 12. Return transactions                        │
      │<────────────────────────────────────────────────│
```

### 2. Request Customer Token (Service → Payment Service)

```bash
POST /auth/v1/customer-token

Headers:
  Authorization: Bearer <service-token>

Request:
{
  "customer_id": "770fa622-g40d-43f6-c938-668877662222",
  "merchant_id": "880fb733-h51e-54g7-d049-779988773333"
}

Response:
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-11-19T15:30:00Z"  // 30 minutes
}
```

### 3. Customer Token Claims

```json
{
  "sub": "customer:770fa622-g40d-43f6-c938-668877662222",
  "customer_id": "770fa622-g40d-43f6-c938-668877662222",
  "merchant_id": "880fb733-h51e-54g7-d049-779988773333",
  "scope": "read:transactions",
  "aud": "payment-service",
  "exp": 1700000000,  // 30 minutes
  "iat": 1699998200
}
```

### 4. Customer API Calls

```bash
# View own transactions
GET /customer/v1/transactions
Authorization: Bearer <customer-token>

# View specific transaction
GET /customer/v1/transactions/{transaction_id}
Authorization: Bearer <customer-token>

# View payment methods
GET /customer/v1/payment-methods
Authorization: Bearer <customer-token>
```

---

## Guest Authentication

**Who**: Anonymous users looking up a specific order/transaction

**Method**: OAuth-style delegation - Service vouches for guest access to specific transaction

### 1. Guest Token Issuance Flow

```
┌──────────┐         ┌──────────────┐         ┌─────────────────┐
│  Guest   │         │   Service    │         │ Payment Service │
│(Browser) │         │(acme-web-app)│         │                 │
└──────────┘         └──────────────┘         └─────────────────┘
      │                      │                         │
      │ 1. Visit "Track Order"                        │
      │    page with order #  │                        │
      │─────────────────────>│                         │
      │                      │                         │
      │                      │ 2. Verify order #       │
      │                      │    exists in merchant   │
      │                      │    system               │
      │                      │                         │
      │                      │ 3. Request guest token  │
      │                      │    for this order       │
      │                      │                         │
      │                      │ POST /auth/v1/guest-token
      │                      │ Authorization: Bearer <service-token>
      │                      │ {parent_transaction_id}
      │                      │─────────────────────────>│
      │                      │                         │
      │                      │                         │ 4. Verify service token
      │                      │                         │ 5. Verify transaction exists
      │                      │                         │ 6. Generate HMAC-signed guest token
      │                      │                         │
      │                      │ 7. Return guest token   │
      │                      │<─────────────────────────│
      │                      │    {token, expires_at}  │
      │                      │                         │
      │ 8. Return token to   │                         │
      │    guest (browser)   │                         │
      │<─────────────────────│                         │
      │                      │                         │
      │ 9. Guest uses token to view order status      │
      │    GET /guest/v1/orders/{parent_transaction_id}
      │    Authorization: Bearer <guest-token>         │
      │────────────────────────────────────────────────>│
      │                                                │
      │                                                │ 10. Verify guest token
      │                                                │ 11. Check parent_transaction_id matches
      │                                                │
      │ 12. Return order status                        │
      │<────────────────────────────────────────────────│
```

### 2. Request Guest Token (Service → Payment Service)

```bash
POST /auth/v1/guest-token

Headers:
  Authorization: Bearer <service-token>

Request:
{
  "parent_transaction_id": "990gc844-i62f-65h8-e150-880099884444"
}

Response:
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-11-19T14:05:00Z"  // 5 minutes
}
```

### 3. Guest Token Claims

```json
{
  "sub": "guest:990gc844-i62f-65h8-e150-880099884444",
  "parent_transaction_id": "990gc844-i62f-65h8-e150-880099884444",
  "scope": "read:order_status",
  "aud": "payment-service",
  "exp": 1700000000,  // 5 minutes (short-lived)
  "iat": 1699999700
}
```

### 4. Guest API Calls

```bash
# View order status by parent transaction ID
GET /guest/v1/orders/{parent_transaction_id}
Authorization: Bearer <guest-token>

# Guest token is single-use, scoped to specific order
```

---

## Merchant Portal Authentication

**Who**: Merchant users accessing their dashboard

**Method**: JWT signed with HMAC-SHA256, session-based

### 1. Merchant Login Flow

Similar to admin authentication, but scoped to specific merchant:

```bash
POST /merchant/v1/auth/login

Request:
{
  "email": "merchant@acme.com",
  "password": "secure_password"
}

Response:
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-11-19T16:00:00Z",  // 2 hours
  "merchant": {
    "id": "880fb733-h51e-54g7-d049-779988773333",
    "slug": "acme-corp",
    "name": "ACME Corporation"
  }
}
```

### 2. Merchant Token Claims

```json
{
  "sub": "merchant:880fb733-h51e-54g7-d049-779988773333",
  "merchant_id": "880fb733-h51e-54g7-d049-779988773333",
  "email": "merchant@acme.com",
  "role": "merchant_admin",
  "session_id": "aa0gd955-j73g-76i9-f261-991100995555",
  "aud": "payment-service",
  "exp": 1700000000,  // 2 hours
  "iat": 1699993200
}
```

### 3. Merchant API Calls

```bash
# View merchant transactions
GET /merchant/v1/transactions
Authorization: Bearer <merchant-token>

# View merchant payment methods
GET /merchant/v1/payment-methods
Authorization: Bearer <merchant-token>

# Update merchant settings
PUT /merchant/v1/settings
Authorization: Bearer <merchant-token>
```

---

## API Endpoint Authentication Matrix

This table shows which token type is required for each API endpoint:

| Endpoint | Token Type | Required Claims | Example |
|----------|-----------|-----------------|---------|
| **Payment Service** ||||
| `POST /payment/v1/sale` | Service | `iss` (service_id) | Process credit card sale |
| `POST /payment/v1/auth` | Service | `iss` (service_id) | Authorize payment |
| `POST /payment/v1/capture` | Service | `iss` (service_id) | Capture authorized payment |
| `POST /payment/v1/refund` | Service | `iss` (service_id) | Refund transaction |
| `POST /payment/v1/void` | Service | `iss` (service_id) | Void transaction |
| **Payment Method Service** ||||
| `POST /payment-method/v1/store` | Service | `iss` (service_id) | Store payment method (BRIC) |
| `PUT /payment-method/v1/{id}` | Service | `iss` (service_id) | Update payment method |
| `DELETE /payment-method/v1/{id}` | Service | `iss` (service_id) | Delete payment method |
| **Subscription Service** ||||
| `POST /subscription/v1/create` | Service | `iss` (service_id) | Create subscription |
| `PUT /subscription/v1/{id}` | Service | `iss` (service_id) | Update subscription |
| `DELETE /subscription/v1/{id}` | Service | `iss` (service_id) | Cancel subscription |
| **Customer Endpoints** ||||
| `GET /customer/v1/transactions` | Customer | `customer_id` | View own transactions |
| `GET /customer/v1/payment-methods` | Customer | `customer_id` | View own payment methods |
| **Guest Endpoints** ||||
| `GET /guest/v1/orders/{id}` | Guest | `parent_transaction_id` | View order status |
| **Admin Endpoints** ||||
| `POST /admin/v1/services` | Admin | `role=admin` | Create service |
| `POST /admin/v1/merchants` | Admin | `role=admin` | Create merchant |
| `POST /admin/v1/services/{id}/rotate-key` | Admin | `role=admin` | Rotate service key |
| `GET /admin/v1/audit-logs` | Admin | `role=admin` | View audit logs |
| **Merchant Portal** ||||
| `GET /merchant/v1/transactions` | Merchant | `merchant_id` | View merchant transactions |
| `GET /merchant/v1/dashboard` | Merchant | `merchant_id` | View dashboard |
| **Public Endpoints (No Auth)** ||||
| `GET /health` | None | - | Health check |
| `POST /webhooks/epx/callback` | None (MAC verified) | - | EPX Server Post callback |

---

## Detailed Endpoint Authentication Strategies

This section provides comprehensive authentication strategies for every API endpoint, including required tokens, authorization checks, and data access rules.

**API Endpoints Summary**: The payment service has **40 consolidated endpoints** across all services. Consolidation uses status/flag fields instead of separate action endpoints, and replaces unused RPCs with Browser Post integration for cleaner API design:
- **PaymentService**: 10 endpoints
- **PaymentMethodService**: 10 endpoints (removed SavePaymentMethod, ConvertFinancialBRICToStorageBRIC; added GetPaymentForm, BrowserPostCallback)
- **SubscriptionService**: 6 endpoints (consolidated from 8)
- **ChargebackService**: 2 endpoints
- **AdminService**: 4 endpoints (consolidated from 6)
- **MerchantService**: 5 endpoints (consolidated from 6)
- **Public Endpoints**: 2 endpoints (no authentication)
- **Internal/Future**: Customer, Guest, Merchant Portal endpoints (to be implemented)

### PaymentService (payment.v1.PaymentService)

All payment endpoints require **Service Token** authentication.

#### Authorization Strategy

**Token Required**: Service Token (RSA-signed JWT)

**Authorization Flow**:
1. Extract service context from auth middleware
2. Verify service has access to the merchant specified in request
3. Check service has `payment:write` scope for merchant
4. Validate request parameters
5. Process payment operation

**Service-Merchant Access Check**:
```sql
SELECT EXISTS(
    SELECT 1 FROM service_merchants
    WHERE service_id = $1
        AND merchant_id = $2
        AND 'payment:write' = ANY(scopes)
        AND (expires_at IS NULL OR expires_at > NOW())
) as has_access;
```

#### Endpoints

##### POST /payment/v1/Authorize
**Description**: Authorize payment without capturing funds

**Authentication**:
- Token: Service
- Required Scope: `payment:write`

**Authorization**:
```go
serviceCtx := auth.GetServiceContext(ctx)
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: uuid.Parse(req.MerchantId),
    Scope:      "payment:write",
})
```

**Data Access**:
- Service can authorize payments for any customer under merchants they have access to
- Customer ID is optional (guest transactions)
- Payment method must belong to the merchant

**Example**:
```bash
POST /payment/v1/Authorize
Authorization: Bearer <service-token>

{
  "merchant_id": "550e8400-...",
  "customer_id": "770fa622-...",  # Optional
  "amount": "29.99",
  "currency": "USD",
  "payment_method_id": "aa1hd066-..."
}
```

##### POST /payment/v1/Capture
**Description**: Capture previously authorized payment

**Authentication**:
- Token: Service
- Required Scope: `payment:write`

**Authorization**:
- Service must have access to the merchant that owns the original authorization
- Capture amount cannot exceed authorized amount

**Data Access**:
```go
// Fetch original authorization transaction
authTx := queries.GetTransaction(ctx, req.TransactionId)

// Verify service has access to merchant
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: authTx.MerchantID,
    Scope:      "payment:write",
})

// Verify transaction type is AUTH
if authTx.Type != "AUTH" {
    return ErrInvalidTransactionType
}

// Verify capture amount <= auth amount
if req.Amount > authTx.AmountCents {
    return ErrCaptureExceedsAuthorization
}
```

##### POST /payment/v1/Sale
**Description**: Combined authorize + capture

**Authentication**:
- Token: Service
- Required Scope: `payment:write`

**Authorization**: Same as Authorize

**Data Access**: Same as Authorize

##### POST /payment/v1/Void
**Description**: Cancel authorized or captured payment

**Authentication**:
- Token: Service
- Required Scope: `payment:write`

**Authorization**:
- Service must have access to merchant that owns the transaction
- Transaction must be voidable (AUTH or SALE, not yet settled)

##### POST /payment/v1/Refund
**Description**: Refund captured payment

**Authentication**:
- Token: Service
- Required Scope: `payment:refund`

**Authorization**:
- Service must have access to merchant
- Requires explicit `payment:refund` scope (more sensitive operation)
- Transaction must be captured and settled

**Data Access**:
```go
// Fetch original transaction
origTx := queries.GetTransaction(ctx, req.GroupId)

// Verify service has REFUND scope
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: origTx.MerchantID,
    Scope:      "payment:refund",  // More restrictive scope
})

// Verify transaction is refundable
if origTx.Type != "CAPTURE" && origTx.Type != "SALE" {
    return ErrTransactionNotRefundable
}

// Refund amount validation
if req.Amount > origTx.AmountCents {
    return ErrRefundExceedsOriginal
}
```

##### POST /payment/v1/ACHDebit
**Description**: Pull money from bank account

**Authentication**:
- Token: Service
- Required Scope: `payment:ach`

**Authorization**:
- Service must have ACH-specific scope
- Payment method must be ACH Storage BRIC
- Account must be verified (pre-note completed)

**Data Access**:
```go
// Verify service has ACH scope
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: uuid.Parse(req.MerchantId),
    Scope:      "payment:ach",
})

// Verify payment method is ACH and verified
pm := queries.GetPaymentMethod(ctx, req.PaymentMethodId)
if pm.PaymentType != "ACH" {
    return ErrInvalidPaymentMethodType
}
if !pm.IsVerified {
    return ErrACHAccountNotVerified
}
```

##### POST /payment/v1/ACHCredit
**Description**: Send money to bank account

**Authentication**: Same as ACHDebit

**Authorization**: Same as ACHDebit + requires `payment:payout` scope for credits

##### POST /payment/v1/ACHVoid
**Description**: Cancel ACH transaction before settlement

**Authentication**: Same as ACHDebit

**Authorization**: Transaction must be ACH and not yet settled

##### GET /payment/v1/GetTransaction
**Description**: Retrieve transaction details

**Authentication**:
- Token: Service OR Customer OR Guest
- Scope: `payment:read` (for service)

**Authorization**:
```go
switch tokenType {
case TokenTypeService:
    // Service needs access to merchant
    tx := queries.GetTransaction(ctx, req.TransactionId)
    hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
        ServiceID:  serviceCtx.ServiceUUID,
        MerchantID: tx.MerchantID,
        Scope:      "payment:read",
    })

case TokenTypeCustomer:
    // Customer can only view their own transactions
    customerCtx := auth.GetCustomerContext(ctx)
    tx := queries.GetTransaction(ctx, req.TransactionId)
    if tx.CustomerID != customerCtx.CustomerID {
        return ErrUnauthorized
    }

case TokenTypeGuest:
    // Guest can only view transactions matching their parent_transaction_id
    guestCtx := auth.GetGuestContext(ctx)
    tx := queries.GetTransaction(ctx, req.TransactionId)
    if tx.ParentTransactionID != guestCtx.ParentTransactionID {
        return ErrUnauthorized
    }
}
```

##### GET /payment/v1/ListTransactions
**Description**: List transactions

**Authentication**:
- Token: Service OR Customer
- Scope: `payment:read`

**Authorization**:
```go
switch tokenType {
case TokenTypeService:
    // Service can list all transactions for merchants they have access to
    // Filter by merchant_id in request
    hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
        ServiceID:  serviceCtx.ServiceUUID,
        MerchantID: uuid.Parse(req.MerchantId),
        Scope:      "payment:read",
    })
    txs := queries.ListTransactionsByMerchant(ctx, req.MerchantId)

case TokenTypeCustomer:
    // Customer can only list their own transactions
    customerCtx := auth.GetCustomerContext(ctx)
    txs := queries.ListTransactionsByCustomer(ctx, customerCtx.CustomerID)
}
```

---

### PaymentMethodService (payment_method.v1.PaymentMethodService)

Payment method endpoints use **Service Token** authentication for most operations. Some read operations also support **Customer Token** for customers viewing their own payment methods.

#### GET /payment-method/v1/GetPaymentForm
**Description**: Generate payment form configuration for Browser Post integration (REST endpoint, returns JSON)

**Authentication**:
- Token: Service
- Required Scope: `payment_method:read`

**Authorization**:
```go
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: uuid.Parse(req.MerchantId),
    Scope:      "payment_method:read",
})
```

**Data Access**:
- Generates payment form session token
- Returns JSON with EPX URL, merchant config, callback URL
- Use case: Frontend requests payment form configuration before rendering EPX Browser Post

**Response**:
```json
{
  "form_token": "ft_abc123...",
  "epx_url": "https://epx.gateway.com/browserpost",
  "merchant_id": "merchant_123",
  "transaction_type": "CCE2",
  "amount": "29.99",
  "callback_url": "https://yourserver.com/api/payment/callback",
  "return_url": "https://yoursite.com/payment/success",
  "session_expires_at": "2025-11-19T15:30:00Z"
}
```

#### POST /payment/callback (BrowserPostCallback)
**Description**: Handle Browser Post callbacks from EPX gateway (REST endpoint, processes credit card transactions)

**Authentication**:
- No token required (uses EPX_MAC signature verification instead)
- MAC secret verified against merchant's stored secret

**Authorization**:
```go
// Verify EPX MAC signature (HMAC-SHA256)
expectedMAC := ComputeEPXMAC(merchantMACSecret, callbackFields)
if !hmac.Equal([]byte(req.EPX_MAC), []byte(expectedMAC)) {
    return ErrInvalidSignature
}
```

**Data Access**:
- Processes CCE1 (auth-only): Creates transaction with type="auth", status="authorized"
- Processes CCE2 (sale): Creates transaction with type="sale", status="approved"
- Processes CCE8 (storage): Saves payment method to `customer_payment_methods`, no transaction
- Extracts Storage BRIC from AUTH_GUID
- Stores card metadata (last_four, card_brand, expiry)

**Processing Logic**:
```go
// Route by transaction type
switch req.TRAN_TYPE {
case "CCE1": // Auth Only
    tx := CreateTransaction(ctx, TransactionParams{
        Type:     "auth",
        Status:   "authorized",
        AuthGuid: req.AUTH_GUID, // Financial BRIC
    })

case "CCE2": // Sale
    tx := CreateTransaction(ctx, TransactionParams{
        Type:     "sale",
        Status:   "approved",
        AuthGuid: req.AUTH_GUID, // Financial BRIC
    })

case "CCE8": // Storage Only
    pm := SavePaymentMethod(ctx, PaymentMethodParams{
        Bric:      req.AUTH_GUID, // Storage BRIC
        LastFour:  req.LAST_FOUR,
        CardBrand: req.CARD_TYPE,
    })
}
```

#### POST /payment-method/v1/StoreACHAccount
**Description**: Store ACH account with automatic pre-note verification

**Authentication**:
- Token: Service
- Required Scope: `payment_method:ach`

**Authorization**: Same as SavePaymentMethod + ACH-specific scope

**Data Access**:
```go
// Verify service has ACH scope
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: uuid.Parse(req.MerchantId),
    Scope:      "payment_method:ach",
})

// Process: Create Storage BRIC → Send Pre-note → Save to DB
```

#### GET /payment-method/v1/GetPaymentMethod
**Description**: Retrieve payment method details

**Authentication**:
- Token: Service OR Customer
- Scope: `payment_method:read`

**Authorization**:
```go
switch tokenType {
case TokenTypeService:
    pm := queries.GetPaymentMethod(ctx, req.Id)
    hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
        ServiceID:  serviceCtx.ServiceUUID,
        MerchantID: pm.MerchantID,
        Scope:      "payment_method:read",
    })

case TokenTypeCustomer:
    // Customer can only view their own payment methods
    customerCtx := auth.GetCustomerContext(ctx)
    pm := queries.GetPaymentMethod(ctx, req.Id)
    if pm.CustomerID != customerCtx.CustomerID {
        return ErrUnauthorized
    }
}
```

#### GET /payment-method/v1/ListPaymentMethods
**Description**: List payment methods

**Authentication**:
- Token: Service OR Customer
- Scope: `payment_method:read`

**Authorization**:
```go
switch tokenType {
case TokenTypeService:
    // Service can list payment methods for customers under their merchants
    hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
        ServiceID:  serviceCtx.ServiceUUID,
        MerchantID: uuid.Parse(req.MerchantId),
        Scope:      "payment_method:read",
    })
    pms := queries.ListPaymentMethods(ctx, ListPaymentMethodsParams{
        MerchantID: uuid.Parse(req.MerchantId),
        CustomerID: uuid.Parse(req.CustomerId),
    })

case TokenTypeCustomer:
    // Customer can only list their own payment methods
    customerCtx := auth.GetCustomerContext(ctx)
    pms := queries.ListPaymentMethodsByCustomer(ctx, customerCtx.CustomerID)
}
```

#### PUT /payment-method/v1/UpdatePaymentMethod
**Description**: Update payment method metadata only (no account/routing changes)

**Authentication**:
- Token: Service
- Required Scope: `payment_method:write`

**Authorization**: Service must have access to merchant that owns payment method

**Data Access**:
- Can only update metadata (billing address, nickname)
- **Cannot** update account numbers, routing numbers, card numbers
- For security, sensitive fields are immutable

#### DELETE /payment-method/v1/DeletePaymentMethod
**Description**: Soft delete payment method

**Authentication**:
- Token: Service
- Required Scope: `payment_method:delete`

**Authorization**:
```go
pm := queries.GetPaymentMethod(ctx, req.Id)
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: pm.MerchantID,
    Scope:      "payment_method:delete",
})

// Soft delete: set deleted_at timestamp
queries.SoftDeletePaymentMethod(ctx, req.Id)
```

#### POST /payment-method/v1/SetDefaultPaymentMethod
**Description**: Set customer's default payment method

**Authentication**:
- Token: Service OR Customer
- Scope: `payment_method:write`

**Authorization**:
```go
pm := queries.GetPaymentMethod(ctx, req.Id)

switch tokenType {
case TokenTypeService:
    hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
        ServiceID:  serviceCtx.ServiceUUID,
        MerchantID: pm.MerchantID,
        Scope:      "payment_method:write",
    })

case TokenTypeCustomer:
    customerCtx := auth.GetCustomerContext(ctx)
    if pm.CustomerID != customerCtx.CustomerID {
        return ErrUnauthorized
    }
}
```

#### POST /payment-method/v1/VerifyACHAccount
**Description**: Verify ACH account with micro-deposits

**Authentication**:
- Token: Service
- Required Scope: `payment_method:ach`

**Authorization**: Service must have ACH scope and access to merchant

---

### SubscriptionService (subscription.v1.SubscriptionService)

All subscription endpoints require **Service Token** authentication.

#### POST /subscription/v1/CreateSubscription
**Description**: Create recurring subscription

**Authentication**:
- Token: Service
- Required Scope: `subscription:write`

**Authorization**:
```go
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: uuid.Parse(req.MerchantId),
    Scope:      "subscription:write",
})

// Verify payment method belongs to merchant and customer
pm := queries.GetPaymentMethod(ctx, req.PaymentMethodId)
if pm.MerchantID != req.MerchantId || pm.CustomerID != req.CustomerId {
    return ErrPaymentMethodMismatch
}
```

**Data Access**:
- Creates subscription linked to customer and payment method
- Sets up recurring billing schedule
- Stores gateway_subscription_id if provided

#### PUT /subscription/v1/UpdateSubscription
**Description**: Update subscription amount, interval, payment method, or status

**Authentication**:
- Token: Service
- Required Scope: `subscription:write`

**Authorization**: Service must have access to merchant that owns subscription

**Data Access**:
```go
sub := queries.GetSubscription(ctx, req.SubscriptionId)
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: sub.MerchantID,
    Scope:      "subscription:write",
})

// Update allowed fields
if req.AmountCents != nil {
    sub.AmountCents = *req.AmountCents
}
if req.PaymentMethodId != nil {
    // Verify new payment method belongs to same customer
    pm := queries.GetPaymentMethod(ctx, *req.PaymentMethodId)
    if pm.CustomerID != sub.CustomerID {
        return ErrPaymentMethodCustomerMismatch
    }
    sub.PaymentMethodID = *req.PaymentMethodId
}
if req.Status != nil {
    // Update status: ACTIVE, PAUSED, or CANCELLED
    switch *req.Status {
    case "PAUSED":
        // Preserve next_billing_date for resume
        sub.Status = "paused"
    case "ACTIVE":
        // Resume subscription billing
        sub.Status = "active"
    case "CANCELLED":
        // Cancel subscription
        sub.Status = "cancelled"
        sub.CancelledAt = time.Now()
    }
}
```

**Status Management**:
- Use `status: "PAUSED"` to pause billing (replaces PauseSubscription)
- Use `status: "ACTIVE"` to resume billing (replaces ResumeSubscription)
- Use `status: "CANCELLED"` to cancel subscription (alternative to CancelSubscription)

#### POST /subscription/v1/CancelSubscription
**Description**: Cancel active subscription (convenience endpoint)

**Authentication**:
- Token: Service
- Required Scope: `subscription:write`

**Authorization**: Service must have access to merchant

**Data Access**:
```go
sub := queries.GetSubscription(ctx, req.SubscriptionId)
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: sub.MerchantID,
    Scope:      "subscription:write",
})

// Set status = 'cancelled', cancelled_at = NOW()
queries.CancelSubscription(ctx, req.SubscriptionId)
```

**Note**: Can also use `PUT /subscription/v1/UpdateSubscription` with `status: "CANCELLED"`

#### GET /subscription/v1/GetSubscription
**Description**: Retrieve subscription details

**Authentication**:
- Token: Service OR Customer
- Scope: `subscription:read`

**Authorization**:
```go
switch tokenType {
case TokenTypeService:
    sub := queries.GetSubscription(ctx, req.SubscriptionId)
    hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
        ServiceID:  serviceCtx.ServiceUUID,
        MerchantID: sub.MerchantID,
        Scope:      "subscription:read",
    })

case TokenTypeCustomer:
    customerCtx := auth.GetCustomerContext(ctx)
    sub := queries.GetSubscription(ctx, req.SubscriptionId)
    if sub.CustomerID != customerCtx.CustomerID {
        return ErrUnauthorized
    }
}
```

#### GET /subscription/v1/ListCustomerSubscriptions
**Description**: List customer's subscriptions

**Authentication**:
- Token: Service OR Customer
- Scope: `subscription:read`

**Authorization**:
```go
switch tokenType {
case TokenTypeService:
    hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
        ServiceID:  serviceCtx.ServiceUUID,
        MerchantID: uuid.Parse(req.MerchantId),
        Scope:      "subscription:read",
    })
    subs := queries.ListSubscriptions(ctx, ListSubscriptionsParams{
        MerchantID: uuid.Parse(req.MerchantId),
        CustomerID: uuid.Parse(req.CustomerId),
    })

case TokenTypeCustomer:
    customerCtx := auth.GetCustomerContext(ctx)
    subs := queries.ListSubscriptionsByCustomer(ctx, customerCtx.CustomerID)
}
```

#### POST /subscription/v1/ProcessDueBilling
**Description**: Process subscriptions due for billing (cron job)

**Authentication**:
- Token: Service (internal cron service)
- Required Scope: `subscription:process_billing`

**Authorization**:
- Only internal cron service should have this scope
- Processes all subscriptions where next_billing_date <= NOW()

**Data Access**:
```go
// This is typically called by internal cron service
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: ANY,  // Cron service processes all merchants
    Scope:      "subscription:process_billing",
})

// Find due subscriptions
dueSubs := queries.GetDueSubscriptions(ctx, time.Now())

// Process each subscription
for _, sub := range dueSubs {
    // Charge payment method
    // Update next_billing_date
    // Handle failures (retry logic)
}
```

---

### ChargebackService (chargeback.v1.ChargebackService)

All chargeback endpoints require **Service Token** authentication.

#### GET /chargeback/v1/GetChargeback
**Description**: Retrieve chargeback details

**Authentication**:
- Token: Service
- Required Scope: `chargeback:read`

**Authorization**:
```go
cb := queries.GetChargeback(ctx, req.ChargebackId)
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: cb.GroupID,  // GroupID is merchant_id
    Scope:      "chargeback:read",
})
```

**Data Access**:
- Service can only view chargebacks for merchants they have access to

#### GET /chargeback/v1/ListChargebacks
**Description**: List chargebacks with filters

**Authentication**:
- Token: Service
- Required Scope: `chargeback:read`

**Authorization**:
```go
hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
    ServiceID:  serviceCtx.ServiceUUID,
    MerchantID: uuid.Parse(req.MerchantId),
    Scope:      "chargeback:read",
})

// Filter chargebacks by merchant, status, date range
cbs := queries.ListChargebacks(ctx, ListChargebacksParams{
    MerchantID: uuid.Parse(req.MerchantId),
    Status:     req.Status,
    StartDate:  req.StartDate,
    EndDate:    req.EndDate,
})
```

---

### AdminService (admin.v1.AdminService)

All admin endpoints require **Admin Token** authentication.

#### POST /admin/v1/CreateService
**Description**: Create new service with auto-generated keypair

**Authentication**:
- Token: Admin
- Required Role: `super_admin` or `admin`

**Authorization**:
```go
adminCtx := auth.GetAdminContext(ctx)
if adminCtx.Role != "super_admin" && adminCtx.Role != "admin" {
    return connect.NewError(connect.CodePermissionDenied,
        fmt.Errorf("insufficient permissions"))
}

// Create service
service := queries.CreateService(ctx, CreateServiceParams{...})

// Audit log
auditLog := CreateAuditLog(ctx, AuditLogParams{
    ActorType:  "admin",
    ActorID:    adminCtx.AdminID.String(),
    ActorName:  adminCtx.Email,
    Action:     "service.created",
    EntityType: "service",
    EntityID:   service.ID.String(),
})
```

**Data Access**:
- Auto-generates RSA keypair
- Returns private key **once**
- Stores public key + fingerprint in DB

#### POST /admin/v1/RotateServiceKey
**Description**: Rotate service RSA keypair

**Authentication**: Same as CreateService

**Authorization**: Same as CreateService

**Data Access**:
- Generates new keypair
- Updates public key in services table
- Returns new private key **once**
- Audit logs rotation with reason

#### GET /admin/v1/GetService
**Description**: Retrieve service details

**Authentication**:
- Token: Admin
- Required Role: Any admin role

**Authorization**:
```go
adminCtx := auth.GetAdminContext(ctx)
// All admin roles can view services
service := queries.GetServiceByServiceID(ctx, req.ServiceId)
```

**Data Access**:
- Returns service info including public key fingerprint
- **Does not** return private key (never stored)

#### GET /admin/v1/ListServices
**Description**: List all services with filtering

**Authentication**: Same as GetService

**Authorization**: Same as GetService

**Data Access**:
- Can filter by environment, is_active
- Pagination support

#### PUT /admin/v1/UpdateService
**Description**: Update service configuration including activation status

**Authentication**:
- Token: Admin
- Required Role: `super_admin` or `admin`

**Authorization**: Same as CreateService

**Data Access**:
```go
adminCtx := auth.GetAdminContext(ctx)
if adminCtx.Role != "super_admin" && adminCtx.Role != "admin" {
    return connect.NewError(connect.CodePermissionDenied,
        fmt.Errorf("insufficient permissions"))
}

service := queries.GetServiceByServiceID(ctx, req.ServiceId)

// Update allowed fields
if req.IsActive != nil {
    queries.UpdateServiceActiveStatus(ctx, UpdateServiceActiveStatusParams{
        ID:       service.ID,
        IsActive: *req.IsActive,
    })
}
if req.RequestsPerSecond != nil {
    service.RequestsPerSecond = *req.RequestsPerSecond
}
if req.BurstLimit != nil {
    service.BurstLimit = *req.BurstLimit
}

// Audit log
action := "service.updated"
if req.IsActive != nil {
    if *req.IsActive {
        action = "service.activated"
    } else {
        action = "service.deactivated"
    }
}

auditLog := CreateAuditLog(ctx, AuditLogParams{
    ActorType:  "admin",
    ActorID:    adminCtx.AdminID.String(),
    Action:     action,
    EntityID:   service.ID.String(),
    Metadata:   map[string]interface{}{"reason": req.Reason},
})
```

**Activation Control**:
- Use `is_active: false` to deactivate service (replaces DeactivateService)
- Use `is_active: true` to activate service (replaces ActivateService)
- Can also update rate limits in same request

---

### MerchantService (merchant.v1.MerchantService)

Merchant management endpoints require **Admin Token** or **Service Token** depending on operation.

#### POST /merchant/v1/RegisterMerchant
**Description**: Register new merchant with EPX credentials

**Authentication**:
- Token: Admin OR Service
- Admin Role: `super_admin` or `admin`
- Service Scope: `merchant:create`

**Authorization**:
```go
switch tokenType {
case TokenTypeAdmin:
    adminCtx := auth.GetAdminContext(ctx)
    if adminCtx.Role != "super_admin" && adminCtx.Role != "admin" {
        return ErrInsufficientPermissions
    }

case TokenTypeService:
    serviceCtx := auth.GetServiceContext(ctx)
    hasAccess := queries.CheckServiceHasGlobalScope(ctx, CheckServiceHasGlobalScopeParams{
        ServiceID: serviceCtx.ServiceUUID,
        Scope:     "merchant:create",
    })
}
```

**Data Access**:
- Creates merchant with EPX credentials (cust_nbr, merch_nbr, dba_nbr, terminal_nbr)
- Stores MAC secret path securely
- Generates unique slug

#### GET /merchant/v1/GetMerchant
**Description**: Retrieve merchant details

**Authentication**:
- Token: Admin OR Service
- Service Scope: Must have access to specific merchant

**Authorization**:
```go
switch tokenType {
case TokenTypeAdmin:
    // Admins can view all merchants
    merchant := queries.GetMerchant(ctx, req.MerchantId)

case TokenTypeService:
    serviceCtx := auth.GetServiceContext(ctx)
    hasAccess := queries.CheckServiceHasScope(ctx, CheckServiceHasScopeParams{
        ServiceID:  serviceCtx.ServiceUUID,
        MerchantID: uuid.Parse(req.MerchantId),
        Scope:      "merchant:read",
    })
}
```

#### GET /merchant/v1/ListMerchants
**Description**: List merchants with filtering

**Authentication**:
- Token: Admin OR Service

**Authorization**:
```go
switch tokenType {
case TokenTypeAdmin:
    // Admins can list all merchants
    merchants := queries.ListMerchants(ctx, ListMerchantsParams{...})

case TokenTypeService:
    // Services can only list merchants they have access to
    serviceCtx := auth.GetServiceContext(ctx)
    merchants := queries.ListServiceMerchants(ctx, serviceCtx.ServiceUUID)
}
```

#### PUT /merchant/v1/UpdateMerchant
**Description**: Update merchant details including activation status

**Authentication**:
- Token: Admin
- Required Role: `super_admin` or `admin`

**Authorization**: Admins only

**Data Access**:
```go
adminCtx := auth.GetAdminContext(ctx)
if adminCtx.Role != "super_admin" && adminCtx.Role != "admin" {
    return connect.NewError(connect.CodePermissionDenied,
        fmt.Errorf("insufficient permissions"))
}

merchant := queries.GetMerchant(ctx, req.MerchantId)

// Update allowed fields
if req.Name != nil {
    merchant.Name = *req.Name
}
if req.Tier != nil {
    merchant.Tier = *req.Tier
}
if req.IsActive != nil {
    merchant.IsActive = *req.IsActive

    // Audit log activation/deactivation
    action := "merchant.updated"
    if !*req.IsActive {
        action = "merchant.deactivated"
        // Prevents all payment operations for merchant
    } else {
        action = "merchant.activated"
    }

    auditLog := CreateAuditLog(ctx, AuditLogParams{
        ActorType:  "admin",
        ActorID:    adminCtx.AdminID.String(),
        Action:     action,
        EntityID:   req.MerchantId,
        Metadata:   map[string]interface{}{"reason": req.Reason},
    })
}
```

**Activation Control**:
- Use `is_active: false` to deactivate merchant (replaces DeactivateMerchant)
- Use `is_active: true` to activate merchant
- Can update name, tier, and status in same request
- **Cannot** update EPX credentials directly (use RotateMAC for security)

#### POST /merchant/v1/RotateMAC
**Description**: Rotate EPX MAC secret for merchant

**Authentication**:
- Token: Admin
- Required Role: `super_admin`

**Authorization**: Only super admins can rotate MAC secrets

**Data Access**:
```go
adminCtx := auth.GetAdminContext(ctx)
if adminCtx.Role != "super_admin" {
    return ErrInsufficientPermissions
}

// Generate new MAC secret
// Update mac_secret_path in merchants table
// Audit log rotation
auditLog := CreateAuditLog(ctx, AuditLogParams{
    ActorType:  "admin",
    ActorID:    adminCtx.AdminID.String(),
    Action:     "merchant.mac_rotated",
    EntityID:   req.MerchantId,
    Metadata:   map[string]interface{}{"reason": req.Reason},
})
```

---

### Public Endpoints (No Authentication)

#### GET /health
**Description**: Health check endpoint

**Authentication**: None

**Authorization**: Public

**Data Access**: Returns service status

#### POST /webhooks/epx/callback
**Description**: EPX Server Post callback

**Authentication**: None (uses MAC verification instead)

**Authorization**:
- Verifies EPX MAC signature in callback
- Checks source IP against whitelist

**Data Access**:
```go
// Extract MAC from callback
callbackMAC := req.MAC

// Compute expected MAC
merchant := queries.GetMerchantByCustNbr(ctx, req.CUST_NBR)
macSecret := loadMACSecret(merchant.MacSecretPath)
expectedMAC := computeHMAC(req, macSecret)

// Verify MAC
if callbackMAC != expectedMAC {
    return ErrInvalidMAC
}

// Verify source IP
if !isWhitelistedIP(req.RemoteAddr) {
    return ErrUnauthorizedIP
}

// Process callback...
```

---

## Summary: Authentication Decision Tree

```
Incoming Request
      │
      ▼
Extract Authorization header
      │
      ├─ Missing? → Check if public endpoint
      │             ├─ Yes → Allow
      │             └─ No → 401 Unauthenticated
      │
      ▼
Parse JWT to examine claims
      │
      ├─ Has "iss" claim? → Service Token
      │   │
      │   ├─ Fetch public key from services table
      │   ├─ Verify RSA signature
      │   ├─ Check is_active
      │   ├─ Check endpoint permissions
      │   └─ Check merchant access + scopes
      │
      ├─ Has "sub:admin:*"? → Admin Token
      │   │
      │   ├─ Verify HMAC signature
      │   ├─ Check role
      │   ├─ Check endpoint permissions
      │   └─ Inject admin context
      │
      ├─ Has "sub:customer:*"? → Customer Token
      │   │
      │   ├─ Verify HMAC signature
      │   ├─ Check endpoint permissions
      │   ├─ Verify customer owns resources
      │   └─ Inject customer context
      │
      ├─ Has "sub:guest:*"? → Guest Token
      │   │
      │   ├─ Verify HMAC signature
      │   ├─ Check endpoint permissions
      │   ├─ Verify parent_transaction_id matches
      │   └─ Inject guest context
      │
      └─ Has "sub:merchant:*"? → Merchant Token
          │
          ├─ Verify HMAC signature
          ├─ Check endpoint permissions
          ├─ Verify merchant owns resources
          └─ Inject merchant context
```

---

## Token Issuance Flows

### Summary of Token Issuance

```
Service Token:  Generated by SERVICE, verified by PAYMENT SERVICE
                (RSA keypair, service signs with private key)

Admin Token:    Generated by PAYMENT SERVICE after admin login
                (HMAC, payment service signs and verifies)

Merchant Token: Generated by PAYMENT SERVICE after merchant login
                (HMAC, payment service signs and verifies)

Customer Token: Generated by PAYMENT SERVICE via SERVICE request
                (HMAC, service vouches for customer identity)

Guest Token:    Generated by PAYMENT SERVICE via SERVICE request
                (HMAC, service vouches for guest access to order)
```

### Token Delegation Pattern (Customer & Guest)

The customer and guest tokens use an **OAuth-style delegation pattern**:

1. **Service authenticates** with service token (RSA-signed)
2. **Service requests** customer/guest token from payment service
3. **Payment service verifies** service token and permissions
4. **Payment service issues** customer/guest token (HMAC-signed)
5. **Customer/guest uses** token directly to access payment API

This pattern allows:
- Customers to "own" their payment data
- Merchant's service to vouch for customer identity
- Short-lived, scoped tokens for security
- No direct customer credentials in payment service

---

## Authentication Middleware Implementation

This section explains how to implement authentication for API endpoints in the payment service using ConnectRPC interceptors.

### Architecture Overview

```
Incoming Request
      │
      ▼
┌──────────────────────────────────────┐
│  ConnectRPC Interceptor Chain        │
│                                      │
│  1. Extract Authorization Header     │
│  2. Determine Token Type             │
│  3. Route to Appropriate Verifier    │
│                                      │
│     ┌─────────────────────┐         │
│     │  Token Type Router  │         │
│     └─────────────────────┘         │
│              │                       │
│    ┌─────────┴─────────┐            │
│    │                   │            │
│    ▼                   ▼            │
│ Service Token      HMAC Token       │
│ Verifier          Verifier          │
│ (RSA)             (Admin/Customer)  │
│    │                   │            │
│    └─────────┬─────────┘            │
│              │                       │
│  4. Inject Actor Context             │
│     (service_id, customer_id, etc)  │
│                                      │
│  5. Check Permissions/Scopes         │
│                                      │
└──────────────────────────────────────┘
      │
      ▼
Handler Execution
```

### Token Type Detection

Tokens are distinguished by their claims structure:

```go
// pkg/auth/token_type.go
package auth

import "github.com/golang-jwt/jwt/v5"

type TokenType string

const (
    TokenTypeService  TokenType = "service"
    TokenTypeAdmin    TokenType = "admin"
    TokenTypeCustomer TokenType = "customer"
    TokenTypeGuest    TokenType = "guest"
    TokenTypeMerchant TokenType = "merchant"
)

// DetectTokenType examines JWT claims to determine token type
func DetectTokenType(claims jwt.MapClaims) TokenType {
    sub, ok := claims["sub"].(string)
    if !ok {
        // No sub claim - check issuer for service token
        if iss, ok := claims["iss"].(string); ok && iss != "" {
            return TokenTypeService
        }
        return ""
    }

    // Parse sub prefix (e.g., "admin:uuid", "customer:uuid", "guest:uuid")
    if strings.HasPrefix(sub, "admin:") {
        return TokenTypeAdmin
    }
    if strings.HasPrefix(sub, "customer:") {
        return TokenTypeCustomer
    }
    if strings.HasPrefix(sub, "guest:") {
        return TokenTypeGuest
    }
    if strings.HasPrefix(sub, "merchant:") {
        return TokenTypeMerchant
    }

    // Legacy or service token with iss claim
    if iss, ok := claims["iss"].(string); ok && iss != "" {
        return TokenTypeService
    }

    return ""
}
```

### Service Token Verification (RSA)

```go
// pkg/auth/service_verifier.go
package auth

import (
    "context"
    "fmt"
    "github.com/golang-jwt/jwt/v5"
    "github.com/kevin07696/payment-service/internal/db/sqlc"
    "github.com/kevin07696/payment-service/pkg/crypto"
)

type ServiceVerifier struct {
    queries *sqlc.Queries
}

func NewServiceVerifier(queries *sqlc.Queries) *ServiceVerifier {
    return &ServiceVerifier{queries: queries}
}

// VerifyServiceToken verifies RSA-signed service token
func (v *ServiceVerifier) VerifyServiceToken(ctx context.Context, tokenString string) (*ServiceClaims, error) {
    // Parse without verification first to get issuer (service_id)
    parser := jwt.NewParser()
    token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
    if err != nil {
        return nil, fmt.Errorf("failed to parse token: %w", err)
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return nil, fmt.Errorf("invalid claims type")
    }

    // Extract service_id from issuer
    serviceID, ok := claims["iss"].(string)
    if !ok || serviceID == "" {
        return nil, fmt.Errorf("missing or invalid iss claim")
    }

    // Fetch service from database to get public key
    service, err := v.queries.GetServiceByServiceID(ctx, serviceID)
    if err != nil {
        return nil, fmt.Errorf("service not found: %w", err)
    }

    // Check if service is active
    if !service.IsActive.Bool {
        return nil, fmt.Errorf("service is deactivated")
    }

    // Parse RSA public key
    publicKey, err := crypto.ParsePublicKey(service.PublicKey)
    if err != nil {
        return nil, fmt.Errorf("failed to parse public key: %w", err)
    }

    // Verify token signature with public key
    verifiedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        // Verify signing method is RS256
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return publicKey, nil
    })
    if err != nil {
        return nil, fmt.Errorf("token verification failed: %w", err)
    }

    if !verifiedToken.Valid {
        return nil, fmt.Errorf("invalid token")
    }

    // Extract and validate claims
    verifiedClaims, ok := verifiedToken.Claims.(jwt.MapClaims)
    if !ok {
        return nil, fmt.Errorf("invalid claims")
    }

    // Validate audience
    aud, ok := verifiedClaims["aud"].(string)
    if !ok || aud != "payment-service" {
        return nil, fmt.Errorf("invalid audience")
    }

    return &ServiceClaims{
        ServiceID:   serviceID,
        ServiceUUID: service.ID,
        Environment: service.Environment,
        Claims:      verifiedClaims,
    }, nil
}

type ServiceClaims struct {
    ServiceID   string
    ServiceUUID uuid.UUID
    Environment string
    Claims      jwt.MapClaims
}
```

### HMAC Token Verification (Admin/Customer/Guest/Merchant)

```go
// pkg/auth/hmac_verifier.go
package auth

import (
    "context"
    "fmt"
    "os"
    "strings"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

type HMACVerifier struct {
    secretKey []byte
}

func NewHMACVerifier() *HMACVerifier {
    // Load HMAC secret from environment
    secret := os.Getenv("JWT_HMAC_SECRET")
    if secret == "" {
        panic("JWT_HMAC_SECRET environment variable not set")
    }
    return &HMACVerifier{
        secretKey: []byte(secret),
    }
}

// VerifyHMACToken verifies HMAC-signed token (admin/customer/guest/merchant)
func (v *HMACVerifier) VerifyHMACToken(ctx context.Context, tokenString string) (*HMACClaims, error) {
    // Parse and verify token
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        // Verify signing method is HMAC
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return v.secretKey, nil
    })
    if err != nil {
        return nil, fmt.Errorf("token verification failed: %w", err)
    }

    if !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return nil, fmt.Errorf("invalid claims")
    }

    // Validate audience
    aud, ok := claims["aud"].(string)
    if !ok || aud != "payment-service" {
        return nil, fmt.Errorf("invalid audience")
    }

    // Extract sub claim to determine actor type
    sub, ok := claims["sub"].(string)
    if !ok || sub == "" {
        return nil, fmt.Errorf("missing sub claim")
    }

    // Parse sub to get type and ID (format: "type:uuid")
    parts := strings.Split(sub, ":")
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid sub format")
    }

    actorType := parts[0]
    actorID := parts[1]

    // Parse UUID
    actorUUID, err := uuid.Parse(actorID)
    if err != nil {
        // Guest tokens might have transaction ID instead of UUID
        if actorType != "guest" {
            return nil, fmt.Errorf("invalid actor ID: %w", err)
        }
    }

    return &HMACClaims{
        ActorType:  actorType,
        ActorID:    actorID,
        ActorUUID:  actorUUID,
        Claims:     claims,
    }, nil
}

type HMACClaims struct {
    ActorType  string // "admin", "customer", "guest", "merchant"
    ActorID    string
    ActorUUID  uuid.UUID
    Claims     jwt.MapClaims
}
```

### ConnectRPC Interceptor

```go
// internal/middleware/auth_interceptor.go
package middleware

import (
    "context"
    "strings"
    "connectrpc.com/connect"
    "github.com/kevin07696/payment-service/pkg/auth"
)

type AuthInterceptor struct {
    serviceVerifier *auth.ServiceVerifier
    hmacVerifier    *auth.HMACVerifier
}

func NewAuthInterceptor(
    serviceVerifier *auth.ServiceVerifier,
    hmacVerifier *auth.HMACVerifier,
) *AuthInterceptor {
    return &AuthInterceptor{
        serviceVerifier: serviceVerifier,
        hmacVerifier:    hmacVerifier,
    }
}

// WrapUnary wraps unary RPC calls with authentication
func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
    return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
        // Check if endpoint requires authentication
        if !requiresAuth(req.Spec().Procedure) {
            return next(ctx, req)
        }

        // Extract authorization header
        authHeader := req.Header().Get("Authorization")
        if authHeader == "" {
            return nil, connect.NewError(connect.CodeUnauthenticated,
                fmt.Errorf("missing authorization header"))
        }

        // Extract bearer token
        if !strings.HasPrefix(authHeader, "Bearer ") {
            return nil, connect.NewError(connect.CodeUnauthenticated,
                fmt.Errorf("invalid authorization header format"))
        }
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")

        // Verify token and inject context
        ctx, err := i.verifyAndInjectContext(ctx, tokenString, req.Spec().Procedure)
        if err != nil {
            return nil, connect.NewError(connect.CodeUnauthenticated, err)
        }

        return next(ctx, req)
    }
}

// verifyAndInjectContext verifies token and injects actor context
func (i *AuthInterceptor) verifyAndInjectContext(
    ctx context.Context,
    tokenString string,
    procedure string,
) (context.Context, error) {
    // Parse token to detect type
    parser := jwt.NewParser()
    token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
    if err != nil {
        return nil, fmt.Errorf("failed to parse token: %w", err)
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return nil, fmt.Errorf("invalid claims")
    }

    tokenType := auth.DetectTokenType(claims)

    switch tokenType {
    case auth.TokenTypeService:
        return i.verifyServiceToken(ctx, tokenString, procedure)
    case auth.TokenTypeAdmin:
        return i.verifyAdminToken(ctx, tokenString, procedure)
    case auth.TokenTypeCustomer:
        return i.verifyCustomerToken(ctx, tokenString, procedure)
    case auth.TokenTypeGuest:
        return i.verifyGuestToken(ctx, tokenString, procedure)
    case auth.TokenTypeMerchant:
        return i.verifyMerchantToken(ctx, tokenString, procedure)
    default:
        return nil, fmt.Errorf("unknown token type")
    }
}

// verifyServiceToken verifies service token and checks permissions
func (i *AuthInterceptor) verifyServiceToken(
    ctx context.Context,
    tokenString string,
    procedure string,
) (context.Context, error) {
    serviceClaims, err := i.serviceVerifier.VerifyServiceToken(ctx, tokenString)
    if err != nil {
        return nil, err
    }

    // Check if service has access to this endpoint
    if !canServiceAccessEndpoint(procedure) {
        return nil, fmt.Errorf("service cannot access this endpoint")
    }

    // Inject service context
    ctx = auth.WithServiceContext(ctx, &auth.ServiceContext{
        ServiceID:   serviceClaims.ServiceID,
        ServiceUUID: serviceClaims.ServiceUUID,
        Environment: serviceClaims.Environment,
    })

    return ctx, nil
}

// verifyAdminToken verifies admin token
func (i *AuthInterceptor) verifyAdminToken(
    ctx context.Context,
    tokenString string,
    procedure string,
) (context.Context, error) {
    hmacClaims, err := i.hmacVerifier.VerifyHMACToken(ctx, tokenString)
    if err != nil {
        return nil, err
    }

    if hmacClaims.ActorType != "admin" {
        return nil, fmt.Errorf("expected admin token")
    }

    // Check if admin has access to this endpoint
    if !canAdminAccessEndpoint(procedure) {
        return nil, fmt.Errorf("admin cannot access this endpoint")
    }

    // Extract role from claims
    role, _ := hmacClaims.Claims["role"].(string)

    // Inject admin context
    ctx = auth.WithAdminContext(ctx, &auth.AdminContext{
        AdminID: hmacClaims.ActorUUID,
        Email:   hmacClaims.Claims["email"].(string),
        Role:    role,
    })

    return ctx, nil
}

// verifyCustomerToken verifies customer token
func (i *AuthInterceptor) verifyCustomerToken(
    ctx context.Context,
    tokenString string,
    procedure string,
) (context.Context, error) {
    hmacClaims, err := i.hmacVerifier.VerifyHMACToken(ctx, tokenString)
    if err != nil {
        return nil, err
    }

    if hmacClaims.ActorType != "customer" {
        return nil, fmt.Errorf("expected customer token")
    }

    // Check if customer has access to this endpoint
    if !canCustomerAccessEndpoint(procedure) {
        return nil, fmt.Errorf("customer cannot access this endpoint")
    }

    // Extract merchant_id from claims
    merchantID, _ := hmacClaims.Claims["merchant_id"].(string)
    merchantUUID, _ := uuid.Parse(merchantID)

    // Inject customer context
    ctx = auth.WithCustomerContext(ctx, &auth.CustomerContext{
        CustomerID: hmacClaims.ActorUUID,
        MerchantID: merchantUUID,
    })

    return ctx, nil
}

// verifyGuestToken verifies guest token
func (i *AuthInterceptor) verifyGuestToken(
    ctx context.Context,
    tokenString string,
    procedure string,
) (context.Context, error) {
    hmacClaims, err := i.hmacVerifier.VerifyHMACToken(ctx, tokenString)
    if err != nil {
        return nil, err
    }

    if hmacClaims.ActorType != "guest" {
        return nil, fmt.Errorf("expected guest token")
    }

    // Check if guest has access to this endpoint
    if !canGuestAccessEndpoint(procedure) {
        return nil, fmt.Errorf("guest cannot access this endpoint")
    }

    // Extract parent_transaction_id from claims
    parentTxID, _ := hmacClaims.Claims["parent_transaction_id"].(string)

    // Inject guest context
    ctx = auth.WithGuestContext(ctx, &auth.GuestContext{
        ParentTransactionID: parentTxID,
    })

    return ctx, nil
}

// verifyMerchantToken verifies merchant token
func (i *AuthInterceptor) verifyMerchantToken(
    ctx context.Context,
    tokenString string,
    procedure string,
) (context.Context, error) {
    hmacClaims, err := i.hmacVerifier.VerifyHMACToken(ctx, tokenString)
    if err != nil {
        return nil, err
    }

    if hmacClaims.ActorType != "merchant" {
        return nil, fmt.Errorf("expected merchant token")
    }

    // Check if merchant has access to this endpoint
    if !canMerchantAccessEndpoint(procedure) {
        return nil, fmt.Errorf("merchant cannot access this endpoint")
    }

    // Extract merchant_id from claims
    merchantID, _ := hmacClaims.Claims["merchant_id"].(string)
    merchantUUID, _ := uuid.Parse(merchantID)

    // Inject merchant context
    ctx = auth.WithMerchantContext(ctx, &auth.MerchantContext{
        MerchantID: merchantUUID,
        Email:      hmacClaims.Claims["email"].(string),
    })

    return ctx, nil
}
```

### Endpoint Permission Checks

```go
// pkg/auth/permissions.go
package auth

// requiresAuth determines if an endpoint requires authentication
func requiresAuth(procedure string) bool {
    // Public endpoints (no auth required)
    publicEndpoints := []string{
        "/health",
        "/webhooks.v1.WebhookService/EPXCallback",
    }

    for _, endpoint := range publicEndpoints {
        if procedure == endpoint {
            return false
        }
    }

    return true
}

// canServiceAccessEndpoint checks if service token can access endpoint
func canServiceAccessEndpoint(procedure string) bool {
    // Services can access most endpoints except admin and merchant portal
    forbiddenPrefixes := []string{
        "/admin.v1.AdminService/",
        "/merchant.v1.MerchantPortalService/",
    }

    for _, prefix := range forbiddenPrefixes {
        if strings.HasPrefix(procedure, prefix) {
            return false
        }
    }

    return true
}

// canAdminAccessEndpoint checks if admin token can access endpoint
func canAdminAccessEndpoint(procedure string) bool {
    // Admins can only access admin endpoints
    return strings.HasPrefix(procedure, "/admin.v1.AdminService/")
}

// canCustomerAccessEndpoint checks if customer token can access endpoint
func canCustomerAccessEndpoint(procedure string) bool {
    // Customers can only access customer endpoints
    return strings.HasPrefix(procedure, "/customer.v1.CustomerService/")
}

// canGuestAccessEndpoint checks if guest token can access endpoint
func canGuestAccessEndpoint(procedure string) bool {
    // Guests can only access guest endpoints
    return strings.HasPrefix(procedure, "/guest.v1.GuestService/")
}

// canMerchantAccessEndpoint checks if merchant token can access endpoint
func canMerchantAccessEndpoint(procedure string) bool {
    // Merchants can only access merchant portal endpoints
    return strings.HasPrefix(procedure, "/merchant.v1.MerchantPortalService/")
}
```

### Context Helpers

```go
// pkg/auth/context.go
package auth

import (
    "context"
    "fmt"
    "github.com/google/uuid"
)

type contextKey string

const (
    serviceContextKey  contextKey = "service"
    adminContextKey    contextKey = "admin"
    customerContextKey contextKey = "customer"
    guestContextKey    contextKey = "guest"
    merchantContextKey contextKey = "merchant"
)

// Service Context
type ServiceContext struct {
    ServiceID   string
    ServiceUUID uuid.UUID
    Environment string
}

func WithServiceContext(ctx context.Context, sc *ServiceContext) context.Context {
    return context.WithValue(ctx, serviceContextKey, sc)
}

func GetServiceContext(ctx context.Context) (*ServiceContext, error) {
    sc, ok := ctx.Value(serviceContextKey).(*ServiceContext)
    if !ok {
        return nil, fmt.Errorf("service context not found")
    }
    return sc, nil
}

// Admin Context
type AdminContext struct {
    AdminID uuid.UUID
    Email   string
    Role    string
}

func WithAdminContext(ctx context.Context, ac *AdminContext) context.Context {
    return context.WithValue(ctx, adminContextKey, ac)
}

func GetAdminContext(ctx context.Context) (*AdminContext, error) {
    ac, ok := ctx.Value(adminContextKey).(*AdminContext)
    if !ok {
        return nil, fmt.Errorf("admin context not found")
    }
    return ac, nil
}

// Customer Context
type CustomerContext struct {
    CustomerID uuid.UUID
    MerchantID uuid.UUID
}

func WithCustomerContext(ctx context.Context, cc *CustomerContext) context.Context {
    return context.WithValue(ctx, customerContextKey, cc)
}

func GetCustomerContext(ctx context.Context) (*CustomerContext, error) {
    cc, ok := ctx.Value(customerContextKey).(*CustomerContext)
    if !ok {
        return nil, fmt.Errorf("customer context not found")
    }
    return cc, nil
}

// Guest Context
type GuestContext struct {
    ParentTransactionID string
}

func WithGuestContext(ctx context.Context, gc *GuestContext) context.Context {
    return context.WithValue(ctx, guestContextKey, gc)
}

func GetGuestContext(ctx context.Context) (*GuestContext, error) {
    gc, ok := ctx.Value(guestContextKey).(*GuestContext)
    if !ok {
        return nil, fmt.Errorf("guest context not found")
    }
    return gc, nil
}

// Merchant Context
type MerchantContext struct {
    MerchantID uuid.UUID
    Email      string
}

func WithMerchantContext(ctx context.Context, mc *MerchantContext) context.Context {
    return context.WithValue(ctx, merchantContextKey, mc)
}

func GetMerchantContext(ctx context.Context) (*MerchantContext, error) {
    mc, ok := ctx.Value(merchantContextKey).(*MerchantContext)
    if !ok {
        return nil, fmt.Errorf("merchant context not found")
    }
    return mc, nil
}
```

### Server Setup with Interceptor

```go
// cmd/server/main.go
package main

import (
    "net/http"
    "connectrpc.com/connect"
    "github.com/kevin07696/payment-service/internal/middleware"
    "github.com/kevin07696/payment-service/pkg/auth"
)

func main() {
    // Initialize database
    db := initDB()
    queries := sqlc.New(db)

    // Initialize verifiers
    serviceVerifier := auth.NewServiceVerifier(queries)
    hmacVerifier := auth.NewHMACVerifier()

    // Create auth interceptor
    authInterceptor := middleware.NewAuthInterceptor(serviceVerifier, hmacVerifier)

    // Create interceptor options
    interceptors := connect.WithInterceptors(authInterceptor)

    // Register handlers with interceptor
    mux := http.NewServeMux()

    // Payment service
    paymentHandler := payment.NewPaymentHandler(queries)
    mux.Handle(paymentv1connect.NewPaymentServiceHandler(paymentHandler, interceptors))

    // Admin service
    adminHandler := admin.NewServiceHandler(queries)
    mux.Handle(adminv1connect.NewAdminServiceHandler(adminHandler, interceptors))

    // Customer service
    customerHandler := customer.NewCustomerHandler(queries)
    mux.Handle(customerv1connect.NewCustomerServiceHandler(customerHandler, interceptors))

    // Start server
    http.ListenAndServe(":8080", mux)
}
```

### Using Context in Handlers

```go
// Example: Payment handler using service context
func (h *PaymentHandler) Sale(
    ctx context.Context,
    req *connect.Request[paymentv1.SaleRequest],
) (*connect.Response[paymentv1.SaleResponse], error) {
    // Extract service context injected by auth middleware
    serviceCtx, err := auth.GetServiceContext(ctx)
    if err != nil {
        return nil, connect.NewError(connect.CodeUnauthenticated, err)
    }

    // Verify service has access to this merchant
    hasAccess, err := h.queries.CheckServiceHasScope(ctx, sqlc.CheckServiceHasScopeParams{
        ServiceID:  serviceCtx.ServiceUUID,
        MerchantID: uuid.MustParse(req.Msg.MerchantId),
        Scope:      "payment:write",
    })
    if err != nil || !hasAccess {
        return nil, connect.NewError(connect.CodePermissionDenied,
            fmt.Errorf("service does not have access to this merchant"))
    }

    // Process payment...
    // Service is authenticated and authorized
}

// Example: Customer handler using customer context
func (h *CustomerHandler) GetTransactions(
    ctx context.Context,
    req *connect.Request[customerv1.GetTransactionsRequest],
) (*connect.Response[customerv1.GetTransactionsResponse], error) {
    // Extract customer context injected by auth middleware
    customerCtx, err := auth.GetCustomerContext(ctx)
    if err != nil {
        return nil, connect.NewError(connect.CodeUnauthenticated, err)
    }

    // Customer can only view their own transactions
    transactions, err := h.queries.GetTransactionsByCustomer(ctx,
        customerCtx.CustomerID)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // Return transactions...
}
```

### Rate Limiting by Service

```go
// pkg/auth/rate_limiter.go
package auth

import (
    "context"
    "fmt"
    "time"
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    limiters map[string]*rate.Limiter
    queries  *sqlc.Queries
}

func NewRateLimiter(queries *sqlc.Queries) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        queries:  queries,
    }
}

// CheckRateLimit checks if service has exceeded rate limit
func (rl *RateLimiter) CheckRateLimit(ctx context.Context, serviceID string) error {
    limiter, err := rl.getLimiter(ctx, serviceID)
    if err != nil {
        return err
    }

    if !limiter.Allow() {
        return fmt.Errorf("rate limit exceeded")
    }

    return nil
}

func (rl *RateLimiter) getLimiter(ctx context.Context, serviceID string) (*rate.Limiter, error) {
    // Check if limiter exists
    if limiter, ok := rl.limiters[serviceID]; ok {
        return limiter, nil
    }

    // Fetch service rate limits from DB
    service, err := rl.queries.GetServiceByServiceID(ctx, serviceID)
    if err != nil {
        return nil, err
    }

    // Create new limiter
    rps := rate.Limit(service.RequestsPerSecond.Int32)
    burst := int(service.BurstLimit.Int32)
    limiter := rate.NewLimiter(rps, burst)

    rl.limiters[serviceID] = limiter
    return limiter, nil
}
```

---

## Security Best Practices

### Private Key Management (Services)

1. **Never commit private keys to version control**
   - Use `.gitignore` to exclude `.pem`, `.key` files
   - Use secret management systems (AWS Secrets Manager, k8s secrets)

2. **Store private keys securely**
   ```bash
   # File permissions: read/write for owner only
   chmod 600 /path/to/private.pem
   ```

3. **Rotate keys regularly**
   - Recommended: Every 90 days
   - Immediately if compromised
   - Use `POST /admin/v1/services/{id}/rotate-key`

4. **Log key fingerprint on service startup**
   ```go
   fingerprint, _ := crypto.ComputeFingerprint(publicKeyPEM)
   log.Info("Service started", "fingerprint", fingerprint)
   ```

### Token Security

1. **Always use HTTPS/TLS**
   - Never send tokens over unencrypted connections

2. **Short token lifespans**
   - Service: 15 minutes
   - Customer: 30 minutes
   - Guest: 5 minutes (single-use)
   - Admin/Merchant: 2 hours

3. **Validate all JWT claims**
   ```go
   // Verify issuer, audience, expiration
   if claims.Issuer != expectedIssuer {
       return ErrInvalidIssuer
   }
   if time.Now().After(claims.ExpiresAt) {
       return ErrTokenExpired
   }
   ```

4. **Rate limiting per service**
   - Configured in `services` table: `requests_per_second`, `burst_limit`
   - Prevents abuse if token is compromised

### Audit Logging

All authentication events should be logged:

```go
auditLog := AuditLog{
    ActorType:  "service",
    ActorID:    serviceID,
    Action:     "api.call",
    EntityType: "transaction",
    EntityID:   transactionID,
    Success:    true,
    IPAddress:  clientIP,
    UserAgent:  userAgent,
}
```

### Token Blacklisting

For immediate token revocation (e.g., security incident):

```sql
-- jwt_blacklist table
INSERT INTO jwt_blacklist (jti, service_id, expires_at, reason)
VALUES ('token-id', 'acme-web-app', '2025-11-19T15:00:00Z', 'Suspected compromise');
```

---

## Complete Examples

### Example 1: Service Processes Payment

```go
// 1. Service creates service token
serviceToken := generateServiceToken(privateKey, "acme-web-app")

// 2. Service calls payment API
client := payment.NewPaymentServiceClient(conn)
req := &paymentv1.SaleRequest{
    MerchantId:  "880fb733-h51e-54g7-d049-779988773333",
    CustomerId:  "770fa622-g40d-43f6-c938-668877662222",
    AmountCents: 2999,  // $29.99
    Currency:    "USD",
    PaymentMethodId: "aa1hd066-k84h-87j0-g372-002211006666",
}

ctx := metadata.AppendToOutgoingContext(context.Background(),
    "authorization", "Bearer "+serviceToken,
)

resp, err := client.Sale(ctx, req)
// Process response
```

### Example 2: Service Issues Customer Token

```go
// 1. Service creates service token
serviceToken := generateServiceToken(privateKey, "acme-web-app")

// 2. Service requests customer token
client := auth.NewAuthServiceClient(conn)
req := &authv1.CustomerTokenRequest{
    CustomerId:  "770fa622-g40d-43f6-c938-668877662222",
    MerchantId:  "880fb733-h51e-54g7-d049-779988773333",
}

ctx := metadata.AppendToOutgoingContext(context.Background(),
    "authorization", "Bearer "+serviceToken,
)

resp, err := client.IssueCustomerToken(ctx, req)

// 3. Return customer token to frontend
customerToken := resp.Token
// Frontend uses customerToken to call /customer/v1/* endpoints
```

### Example 3: Customer Views Transactions

```javascript
// Frontend JavaScript (customer already has customer token from service)

const customerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...";

fetch('https://payment-api.example.com/customer/v1/transactions', {
  headers: {
    'Authorization': `Bearer ${customerToken}`,
    'Content-Type': 'application/json'
  }
})
.then(response => response.json())
.then(transactions => {
  console.log('My transactions:', transactions);
});
```

### Example 4: Guest Looks Up Order

```javascript
// Frontend JavaScript (guest has guest token from service)

const guestToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...";
const orderID = "990gc844-i62f-65h8-e150-880099884444";

fetch(`https://payment-api.example.com/guest/v1/orders/${orderID}`, {
  headers: {
    'Authorization': `Bearer ${guestToken}`,
    'Content-Type': 'application/json'
  }
})
.then(response => response.json())
.then(order => {
  console.log('Order status:', order);
});
```

---

## Troubleshooting

### Common Authentication Errors

#### 1. "Invalid signature"

**Cause**: Service is using wrong private key, or public key in DB is outdated

**Solution**:
- Verify service is using correct private key
- Check public key fingerprint in DB matches service's public key:
  ```bash
  # On service
  openssl rsa -in private.pem -pubout | openssl sha256

  # In DB
  SELECT public_key_fingerprint FROM services WHERE service_id = 'acme-web-app';
  ```
- If mismatch, rotate key: `POST /admin/v1/services/{id}/rotate-key`

#### 2. "Token expired"

**Cause**: Token lifespan exceeded

**Solution**:
- Service tokens: Generate new token (15 min lifespan)
- Customer/Guest tokens: Request new token from service
- Admin/Merchant: Re-login

#### 3. "Service not found"

**Cause**: Service is not registered or deactivated

**Solution**:
- Check service exists: `GET /admin/v1/services/{service_id}`
- Check `is_active = true`
- Register service if needed: `POST /admin/v1/services`

#### 4. "Forbidden: Insufficient permissions"

**Cause**: Service doesn't have access to requested merchant

**Solution**:
- Check service-merchant link: `SELECT * FROM service_merchants WHERE service_id = ? AND merchant_id = ?`
- Grant access: `POST /admin/v1/service-merchants/grant`

---

## Related Documentation

- [Keypair Auto-Generation](./auth/keypair-auto-generation.md) - Implementation details for RSA keypair generation
- [API Design and Dataflow](./API_DESIGN_AND_DATAFLOW.md) - Complete API documentation
- [Service Onboarding Guide](./SERVICE_ONBOARDING.md) - How to integrate a new service (TODO)
- [Admin Panel Guide](./ADMIN_PANEL.md) - Admin operations guide (TODO)

---

**Questions or Issues?**

- Review the [Security Best Practices](#security-best-practices) section
- Check [Troubleshooting](#troubleshooting) for common errors
- Contact the platform team for support
