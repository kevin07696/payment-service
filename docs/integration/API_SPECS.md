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
- **[ADMIN_CLI.md](./ADMIN_CLI.md)** - Service and merchant management
- **[REACT_INTEGRATION.md](./REACT_INTEGRATION.md)** - React/TypeScript integration
- **[BROWSER_POST_FORM_SETUP.md](./BROWSER_POST_FORM_SETUP.md)** - PCI-compliant tokenization
- **[TOKEN_GENERATION.md](./TOKEN_GENERATION.md)** - JWT authentication

---

## Authentication

All API requests require JWT authentication via `Authorization` header:

```
Authorization: Bearer <JWT_TOKEN>
```

**JWT Claims Structure:**
- `iss` / `sub` - Your service_id (identifies the calling service)
- `scopes` - Array of permission scopes (e.g., `["payments:create", "payments:read"]`)
- `exp` - Token expiration (recommended: 1 hour)
- `iat` / `nbf` - Issued at / Not before timestamps
- `jti` - Unique JWT ID (prevents replay attacks)

**Important:** `merchant_id` is NOT in the token. It's passed in each API request body.

**Example JWT Generation (Node.js):**
```typescript
import jwt from 'jsonwebtoken';
import { v4 as uuidv4 } from 'uuid';
import fs from 'fs';

const privateKey = fs.readFileSync('service_private_key.pem');
const now = Math.floor(Date.now() / 1000);

const token = jwt.sign(
  {
    iss: 'your-service-id',
    sub: 'your-service-id',
    scopes: ['payments:create', 'payments:read'],
    exp: now + 3600,
    iat: now,
    nbf: now,
    jti: uuidv4(),
  },
  privateKey,
  { algorithm: 'RS256' }
);
```

**Or use the admin CLI:**
```bash
./bin/admin -action=generate-token -c service_credentials.json
```

See [TOKEN_GENERATION.md](./TOKEN_GENERATION.md) for complete guide.

---

## Services

### Payment Service

**Proto File:** `proto/payment/v1/payment.proto`

**RPC Methods:**

| Method | Request | Response | Description |
|--------|---------|----------|-------------|
| Authorize | `AuthorizeRequest` | `PaymentResponse` | Hold funds without capturing |
| Capture | `CaptureRequest` | `PaymentResponse` | Capture previously authorized funds |
| Sale | `SaleRequest` | `PaymentResponse` | Combined auth + capture (also used for ACH debit) |
| Void | `VoidRequest` | `PaymentResponse` | Cancel before settlement (also used for ACH void) |
| Refund | `RefundRequest` | `PaymentResponse` | Return funds to customer (also used for ACH credit) |
| GetTransaction | `GetTransactionRequest` | `Transaction` | Retrieve transaction details |
| ListTransactions | `ListTransactionsRequest` | `ListTransactionsResponse` | List transactions with filters |

**Note:** ACH operations use the standard methods above:
- ACH Debit: Use `Sale()` with ACH payment method
- ACH Credit/Refund: Use `Refund()` with ACH payment method
- ACH Void: Use `Void()` with ACH payment method

---

### Payment Method Service

**Proto File:** `proto/payment_method/v1/payment_method.proto`

**RPC Methods:**

| Method | Request | Response | Description |
|--------|---------|----------|-------------|
| GetPaymentMethod | `GetPaymentMethodRequest` | `PaymentMethod` |  |
| ListPaymentMethods | `ListPaymentMethodsRequest` | `ListPaymentMethodsResponse` |  |
| UpdatePaymentMethodStatus | `UpdatePaymentMethodStatusRequest` | `PaymentMethodResponse` |  |
| DeletePaymentMethod | `DeletePaymentMethodRequest` | `DeletePaymentMethodResponse` |  |
| SetDefaultPaymentMethod | `SetDefaultPaymentMethodRequest` | `PaymentMethodResponse` |  |
| VerifyACHAccount | `VerifyACHAccountRequest` | `VerifyACHAccountResponse` |  |
| StoreACHAccount | `StoreACHAccountRequest` | `PaymentMethodResponse` |  |
| UpdatePaymentMethod | `UpdatePaymentMethodRequest` | `PaymentMethodResponse` |  |

---

### Subscription Service

**Proto File:** `proto/subscription/v1/subscription.proto`

**RPC Methods:**

| Method | Request | Response | Description |
|--------|---------|----------|-------------|
| CreateSubscription | `CreateSubscriptionRequest` | `SubscriptionResponse` |  |
| UpdateSubscription | `UpdateSubscriptionRequest` | `SubscriptionResponse` |  |
| CancelSubscription | `CancelSubscriptionRequest` | `SubscriptionResponse` |  |
| PauseSubscription | `PauseSubscriptionRequest` | `SubscriptionResponse` |  |
| ResumeSubscription | `ResumeSubscriptionRequest` | `SubscriptionResponse` |  |
| GetSubscription | `GetSubscriptionRequest` | `Subscription` |  |
| ListCustomerSubscriptions | `ListCustomerSubscriptionsRequest` | `ListCustomerSubscriptionsResponse` |  |
| ProcessDueBilling | `ProcessDueBillingRequest` | `ProcessDueBillingResponse` |  |

---

### Chargeback Service

**Proto File:** `proto/chargeback/v1/chargeback.proto`

**RPC Methods:**

| Method | Request | Response | Description |
|--------|---------|----------|-------------|
| GetChargeback | `GetChargebackRequest` | `Chargeback` |  |
| ListChargebacks | `ListChargebacksRequest` | `ListChargebacksResponse` |  |

---


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

#### ACH Debit (via Sale)

ACH debits use the standard `Sale` method with an ACH payment method.

**Request (POST /payment.v1.PaymentService/Sale):**
```json
{
  "merchant_id": "00000000-0000-0000-0000-000000000001",
  "customer_id": "cust_1234567890",
  "payment_method_id": "pm_ach_abc123",
  "amount_cents": 25000,
  "currency": "USD",
  "idempotency_key": "ach_debit_1234567890",
  "metadata": {
    "invoice_id": "INV-2024-001"
  }
}
```

**Response:**
```json
{
  "transaction_id": "tx_ach_001",
  "is_approved": true,
  "status": "TRANSACTION_STATUS_APPROVED",
  "type": "TRANSACTION_TYPE_CHARGE",
  "amount_cents": 25000,
  "created_at": "2024-11-23T10:30:00Z"
}
```

---

### Payment Method Service

#### Credit Card Tokenization (Browser Post)

Credit cards are tokenized via PCI-compliant Browser Post flow, not via direct API. See [BROWSER_POST_FORM_SETUP.md](./BROWSER_POST_FORM_SETUP.md).

The Browser Post callback automatically creates the payment method and returns the `payment_method_id` for use in payments.

#### Store ACH Account

**Request (POST /payment_method.v1.PaymentMethodService/StoreACHAccount):**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "accountNumber": "123456789",
  "routingNumber": "021000021",
  "accountHolderName": "John Doe",
  "accountType": "ACCOUNT_TYPE_CHECKING",
  "stdEntryClass": "STD_ENTRY_CLASS_WEB",
  "firstName": "John",
  "lastName": "Doe",
  "address": "123 Main St",
  "city": "New York",
  "state": "NY",
  "zipCode": "10001",
  "bankName": "Chase",
  "nickname": "Primary Checking",
  "isDefault": true,
  "idempotencyKey": "store_ach_1234567890"
}
```

**Response:**
```json
{
  "paymentMethodId": "pm_ach_abc123def456",
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "paymentType": "PAYMENT_METHOD_TYPE_ACH",
  "lastFour": "6789",
  "bankName": "Chase",
  "accountType": "checking",
  "isDefault": true,
  "isActive": true,
  "isVerified": false,
  "createdAt": "2024-11-23T10:30:00Z"
}
```

#### List Payment Methods

**Request (POST /payment_method.v1.PaymentMethodService/ListPaymentMethods):**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890"
}
```

**Response:**
```json
{
  "paymentMethods": [
    {
      "id": "pm_abc123def456",
      "merchantId": "00000000-0000-0000-0000-000000000001",
      "customerId": "cust_1234567890",
      "paymentType": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
      "lastFour": "1111",
      "cardBrand": "visa",
      "cardExpMonth": 12,
      "cardExpYear": 2025,
      "isDefault": true,
      "isActive": true,
      "createdAt": "2024-11-23T10:30:00Z"
    }
  ]
}
```

---

### Subscription Service

#### Create Recurring Subscription

**Request (POST /subscription.v1.SubscriptionService/CreateSubscription):**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "amountCents": "2999",
  "currency": "USD",
  "intervalValue": 1,
  "intervalUnit": "INTERVAL_UNIT_MONTH",
  "paymentMethodId": "pm_abc123def456",
  "startDate": "2024-12-01T00:00:00Z",
  "maxRetries": 3,
  "metadata": {
    "plan_name": "Premium Monthly",
    "plan_id": "plan_premium_monthly"
  },
  "idempotencyKey": "sub_create_1234567890"
}
```

**Interval Units:** `INTERVAL_UNIT_DAY`, `INTERVAL_UNIT_WEEK`, `INTERVAL_UNIT_MONTH`, `INTERVAL_UNIT_YEAR`

**Response:**
```json
{
  "subscriptionId": "sub_xyz789",
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "amountCents": "2999",
  "currency": "USD",
  "intervalValue": 1,
  "intervalUnit": "INTERVAL_UNIT_MONTH",
  "status": "SUBSCRIPTION_STATUS_ACTIVE",
  "paymentMethodId": "pm_abc123def456",
  "nextBillingDate": "2024-12-01T00:00:00Z",
  "createdAt": "2024-11-23T10:30:00Z"
}
```

#### List Customer Subscriptions

**Request (POST /subscription.v1.SubscriptionService/ListCustomerSubscriptions):**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890"
}
```

**Response:**
```json
{
  "subscriptions": [
    {
      "id": "sub_xyz789",
      "merchantId": "00000000-0000-0000-0000-000000000001",
      "customerId": "cust_1234567890",
      "amountCents": "2999",
      "currency": "USD",
      "intervalValue": 1,
      "intervalUnit": "INTERVAL_UNIT_MONTH",
      "status": "SUBSCRIPTION_STATUS_ACTIVE",
      "paymentMethodId": "pm_abc123def456",
      "nextBillingDate": "2024-12-01T00:00:00Z",
      "createdAt": "2024-11-23T10:30:00Z"
    }
  ]
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
    "customerId": "cust_1234567890",
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
  "customerId": "cust_1234567890",
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
  customerId: 'cust_1234567890',
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
    CustomerId:      "cust_1234567890",
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
- **[ADMIN_CLI.md](./ADMIN_CLI.md)** - Creating services and obtaining credentials
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

