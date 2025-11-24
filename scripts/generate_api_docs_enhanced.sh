#!/bin/bash
# generate_api_docs_enhanced.sh - Enhanced API documentation with examples
# Usage: ./scripts/generate_api_docs_enhanced.sh

set -e

echo "📡 Generating enhanced API documentation from proto files..."

OUTPUT_FILE="docs/integration/API_SPECS.md"
PROTO_DIR="proto"

# Create header
cat > "$OUTPUT_FILE" <<'EOF'
# API Specification

**Auto-Generated:** $(date '+%Y-%m-%d %H:%M:%S')
**Source:** Protocol Buffer definitions in `proto/`

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Services](#services)
   - [Payment Service](#payment-service)
   - [Payment Method Service](#payment-method-service)
   - [Subscription Service](#subscription-service)
   - [Chargeback Service](#chargeback-service)
4. [Request/Response Examples](#requestresponse-examples)
5. [Error Handling](#error-handling)
6. [Testing with curl](#testing-with-curl)

---

## Overview

This service provides ConnectRPC and gRPC APIs for payment processing, subscription management, and chargebacks.

**Base URL (ConnectRPC):** `https://api.example.com`
**Base URL (gRPC):** `api.example.com:8080`

### Integration Guides

For detailed integration examples:
- **[PAYMENT_CLI.md](./PAYMENT_CLI.md)** - Service and merchant management
- **[REACT_INTEGRATION.md](./REACT_INTEGRATION.md)** - React/TypeScript integration
- **[BROWSER_POST_FORM_SETUP.md](./BROWSER_POST_FORM_SETUP.md)** - PCI-compliant tokenization
- **[TOKEN_GENERATION.md](./TOKEN_GENERATION.md)** - JWT authentication

---

## Authentication

All API requests require JWT authentication via `Authorization` header:

```
Authorization: Bearer <JWT_TOKEN>
```

**JWT Claims Required:**
- `service_id` - Your service UUID
- `merchant_id` - Merchant UUID (for merchant-scoped operations)
- `exp` - Token expiration (recommended: 1 hour)

**Example JWT Generation (Node.js):**
```typescript
import jwt from 'jsonwebtoken';
import fs from 'fs';

const privateKey = fs.readFileSync('service_private_key.pem');

const token = jwt.sign(
  {
    service_id: 'your-service-uuid',
    merchant_id: 'your-merchant-uuid',
  },
  privateKey,
  {
    algorithm: 'RS256',
    expiresIn: '1h',
  }
);
```

See [TOKEN_GENERATION.md](./TOKEN_GENERATION.md) for complete guide.

---

## Services

EOF

# Function to generate service documentation
generate_service_docs() {
    local service_name=$1
    local proto_file=$2

    echo "### $service_name" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    echo "**Proto File:** \`$proto_file\`" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"

    # Extract RPC methods
    echo "**RPC Methods:**" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    echo "| Method | Request | Response | Description |" >> "$OUTPUT_FILE"
    echo "|--------|---------|----------|-------------|" >> "$OUTPUT_FILE"

    awk '
    /rpc / {
        match($0, /rpc ([A-Za-z0-9_]+)\s*\(([^)]+)\)\s*returns\s*\(([^)]+)\)/, arr)
        if (arr[1]) {
            method = arr[1]
            request = arr[2]
            response = arr[3]

            # Read next few lines for comment
            getline
            desc = ""
            if (/\/\//) {
                desc = $0
                gsub(/^\s*\/\/\s*/, "", desc)
            }

            print "| " method " | `" request "` | `" response "` | " desc " |"
        }
    }
    ' "$proto_file" >> "$OUTPUT_FILE"

    echo "" >> "$OUTPUT_FILE"
    echo "---" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
}

# Generate docs for each service
generate_service_docs "Payment Service" "proto/payment/v1/payment.proto"
generate_service_docs "Payment Method Service" "proto/payment_method/v1/payment_method.proto"
generate_service_docs "Subscription Service" "proto/subscription/v1/subscription.proto"
generate_service_docs "Chargeback Service" "proto/chargeback/v1/chargeback.proto"

# Add request/response examples
cat >> "$OUTPUT_FILE" <<'EOF'

## Request/Response Examples

### Payment Service

#### Sale Transaction (Credit Card)

**Request:**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "amountCents": "10000",
  "currency": "USD",
  "paymentMethodId": "pm_abc123def456",
  "idempotencyKey": "sale_1234567890_abc123",
  "metadata": {
    "order_id": "ORD-2024-001",
    "customer_email": "customer@example.com"
  }
}
```

**Response (Success):**
```json
{
  "transactionId": "tx_9876543210",
  "isApproved": true,
  "status": "TRANSACTION_STATUS_APPROVED",
  "amountCents": "10000",
  "authorizationCode": "123456",
  "cardInfo": {
    "brand": "visa",
    "last4": "1111",
    "expiryMonth": 12,
    "expiryYear": 2025
  },
  "createdAt": "2024-11-23T10:30:00Z"
}
```

**Response (Declined):**
```json
{
  "transactionId": "tx_9876543211",
  "isApproved": false,
  "status": "TRANSACTION_STATUS_DECLINED",
  "amountCents": "10000",
  "errorMessage": "Insufficient funds",
  "createdAt": "2024-11-23T10:30:00Z"
}
```

---

#### Refund Transaction

**Request:**
```json
{
  "transactionId": "tx_9876543210",
  "amountCents": "5000",
  "reason": "Customer returned item",
  "idempotencyKey": "refund_1234567890_xyz789"
}
```

**Response:**
```json
{
  "transactionId": "tx_refund_001",
  "parentTransactionId": "tx_9876543210",
  "isApproved": true,
  "status": "TRANSACTION_STATUS_APPROVED",
  "amountCents": "5000",
  "type": "TRANSACTION_TYPE_REFUND",
  "createdAt": "2024-11-23T11:00:00Z"
}
```

---

#### ACH Debit

**Request:**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "paymentMethodId": "pm_ach_abc123",
  "amountCents": "25000",
  "currency": "USD",
  "idempotencyKey": "ach_debit_1234567890",
  "metadata": {
    "invoice_id": "INV-2024-001"
  }
}
```

**Response:**
```json
{
  "transactionId": "tx_ach_001",
  "isApproved": true,
  "status": "TRANSACTION_STATUS_APPROVED",
  "amountCents": "25000",
  "achInfo": {
    "accountLast4": "6789",
    "accountType": "checking",
    "routingNumber": "021000021"
  },
  "createdAt": "2024-11-23T10:30:00Z"
}
```

---

### Payment Method Service

#### Tokenize Credit Card

**Request:**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "paymentToken": "bric_epx_abc123def456",
  "type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "metadata": {
    "nickname": "Primary Visa"
  }
}
```

**Response:**
```json
{
  "paymentMethodId": "pm_abc123def456",
  "type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "cardInfo": {
    "brand": "visa",
    "last4": "1111",
    "expiryMonth": 12,
    "expiryYear": 2025
  },
  "createdAt": "2024-11-23T10:30:00Z"
}
```

---

### Subscription Service

#### Create Recurring Subscription

**Request:**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "paymentMethodId": "pm_abc123def456",
  "amountCents": "2999",
  "currency": "USD",
  "intervalType": "monthly",
  "startDate": "2024-12-01T00:00:00Z",
  "metadata": {
    "plan_name": "Premium Monthly",
    "plan_id": "plan_premium_monthly"
  }
}
```

**Response:**
```json
{
  "subscriptionId": "sub_xyz789",
  "status": "SUBSCRIPTION_STATUS_ACTIVE",
  "amountCents": "2999",
  "intervalType": "monthly",
  "nextBillingDate": "2024-12-01T00:00:00Z",
  "createdAt": "2024-11-23T10:30:00Z"
}
```

---

## Error Handling

### ConnectRPC Error Format

```json
{
  "code": "invalid_argument",
  "message": "amount_cents must be greater than 0",
  "details": [
    {
      "type": "BadRequest",
      "fieldViolations": [
        {
          "field": "amount_cents",
          "description": "Must be a positive integer"
        }
      ]
    }
  ]
}
```

### Common Error Codes

| Code | HTTP Status | Description | Example |
|------|-------------|-------------|---------|
| `invalid_argument` | 400 | Invalid request parameters | Missing required field, invalid UUID format |
| `unauthenticated` | 401 | Missing or invalid JWT token | Expired token, invalid signature |
| `permission_denied` | 403 | Insufficient permissions | Accessing another merchant's data |
| `not_found` | 404 | Resource not found | Transaction ID doesn't exist |
| `already_exists` | 409 | Duplicate idempotency key | Retrying with same idempotency key |
| `failed_precondition` | 412 | Operation not allowed | Refunding more than captured amount |
| `internal` | 500 | Server error | Database connection failure |

### Idempotency

All mutation operations (`Sale`, `Capture`, `Refund`, etc.) require an `idempotency_key`.

**Best Practice:**
```typescript
const idempotencyKey = `${operation}_${timestamp}_${randomUUID()}`;
// Example: "sale_1700000000_abc123-def456-789"
```

**Behavior:**
- Same idempotency key returns same result (no duplicate charge)
- Different idempotency key creates new transaction
- Idempotency keys expire after 24 hours

---

## Testing with curl

### ConnectRPC (JSON over HTTP)

**Sale Transaction:**
```bash
curl -X POST https://api.example.com/payment.v1.PaymentService/Sale \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "amountCents": "10000",
    "currency": "USD",
    "paymentMethodId": "pm_abc123",
    "idempotencyKey": "sale_'"$(date +%s)"'_'"$(uuidgen)"'"
  }'
```

**Get Transaction:**
```bash
curl -X POST https://api.example.com/payment.v1.PaymentService/GetTransaction \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "transactionId": "tx_9876543210"
  }'
```

**Refund:**
```bash
curl -X POST https://api.example.com/payment.v1.PaymentService/Refund \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "transactionId": "tx_9876543210",
    "amountCents": "5000",
    "reason": "Customer request",
    "idempotencyKey": "refund_'"$(date +%s)"'_'"$(uuidgen)"'"
  }'
```

**List Transactions:**
```bash
curl -X POST https://api.example.com/payment.v1.PaymentService/ListTransactions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "limit": 10,
    "offset": 0
  }'
```

### Helper Script

Save as `scripts/api_request.sh`:
```bash
#!/bin/bash
# Usage: ./api_request.sh <method> <payload_json>

JWT_TOKEN=$(./scripts/generate_jwt.sh)
API_URL="https://api.example.com"
METHOD=$1
PAYLOAD=$2

curl -X POST "$API_URL/$METHOD" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d "$PAYLOAD"
```

**Example usage:**
```bash
./scripts/api_request.sh payment.v1.PaymentService/Sale '{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "amountCents": "10000",
  "currency": "USD",
  "paymentMethodId": "pm_abc123",
  "idempotencyKey": "sale_test_001"
}'
```

---

## Client Libraries

### TypeScript/JavaScript (ConnectRPC)

```typescript
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { PaymentService } from './gen/payment/v1/payment_connect';

const transport = createConnectTransport({
  baseUrl: 'https://api.example.com',
  interceptors: [(next) => async (req) => {
    req.header.set('Authorization', `Bearer ${getJwtToken()}`);
    return await next(req);
  }],
});

const client = createPromiseClient(PaymentService, transport);

// Example: Sale
const response = await client.sale({
  merchantId: '00000000-0000-0000-0000-000000000001',
  amountCents: BigInt(10000),
  currency: 'USD',
  paymentMethodId: 'pm_abc123',
  idempotencyKey: `sale_${Date.now()}_${crypto.randomUUID()}`,
});

console.log('Transaction ID:', response.transactionId);
console.log('Approved:', response.isApproved);
```

### Go (gRPC)

```go
import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
    paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
)

conn, err := grpc.Dial("api.example.com:8080", grpc.WithInsecure())
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client := paymentv1.NewPaymentServiceClient(conn)

// Add JWT to context
ctx := metadata.AppendToOutgoingContext(
    context.Background(),
    "authorization", "Bearer "+jwtToken,
)

// Example: Sale
response, err := client.Sale(ctx, &paymentv1.SaleRequest{
    MerchantId:      "00000000-0000-0000-0000-000000000001",
    AmountCents:     10000,
    Currency:        "USD",
    PaymentMethodId: "pm_abc123",
    IdempotencyKey:  fmt.Sprintf("sale_%d_%s", time.Now().Unix(), uuid.New().String()),
})

if err != nil {
    log.Fatal(err)
}

fmt.Println("Transaction ID:", response.TransactionId)
fmt.Println("Approved:", response.IsApproved)
```

---

## Related Documentation

- **[Authentication](../development/AUTH.md)** - JWT token authentication
- **[Error Handling Guide](../development/ERROR_HANDLING.md)** - Error types and handling patterns
- **[PAYMENT_CLI.md](./PAYMENT_CLI.md)** - Creating services and obtaining credentials
- **[REACT_INTEGRATION.md](./REACT_INTEGRATION.md)** - Frontend integration guide
- **[Testing Guide](../development/TESTING_GUIDE.md)** - Testing your integration

---

## Generating Updated Documentation

This file is auto-generated. To regenerate:

```bash
# Regenerate all docs
make docs

# Or just API docs
./scripts/generate_api_docs_enhanced.sh
```

EOF

echo "✅ Enhanced API documentation generated: $OUTPUT_FILE"
echo "   Includes: Request/Response examples, curl commands, error handling"
