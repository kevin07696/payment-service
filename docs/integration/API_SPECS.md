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
7. [Test Data Setup](#test-data-setup)

---

## Overview

This service provides ConnectRPC and gRPC APIs for payment processing, subscription management, and chargebacks.

**Base URL (ConnectRPC):** `https://api.example.com`
**Base URL (gRPC):** `api.example.com:8080`

### Data Format Requirements

**IMPORTANT:** The following fields have strict format requirements. Requests with invalid formats will be rejected.

| Field | Format | Example | Notes |
|-------|--------|---------|-------|
| `idempotency_key` | UUID | `550e8400-e29b-41d4-a716-446655440000` | Required for all mutations |
| `merchant_id` | UUID | `550e8400-e29b-41d4-a716-446655440001` | Required for all requests |
| `payment_method_id` | UUID | `550e8400-e29b-41d4-a716-446655440002` | References stored payment method |
| `transaction_id` | UUID | `550e8400-e29b-41d4-a716-446655440003` | References existing transaction |
| `subscription_id` | UUID | `550e8400-e29b-41d4-a716-446655440004` | References existing subscription |
| `start_date` | RFC3339 Timestamp | `2024-12-01T00:00:00Z` | Full timestamp required, not just date |
| `amount_cents` | Integer/String | `10000` or `"10000"` | Amount in cents (100 = $1.00) |

**Common Mistakes:**
- Using `"sale_123_abc"` instead of UUID for `idempotency_key` - will be rejected
- Using `"2024-12-01"` instead of `"2024-12-01T00:00:00Z"` for `start_date` - will be rejected
- Missing `customer_id` in `SetDefaultPaymentMethod` requests - will fail

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

**Or use the payment CLI:**
```bash
./bin/paycli -action=generate-token -c service_credentials.json
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
| CreateSubscription | `CreateSubscriptionRequest` | `SubscriptionResponse` | Create recurring billing |
| UpdateSubscription | `UpdateSubscriptionRequest` | `SubscriptionResponse` | Update subscription settings |
| CancelSubscription | `CancelSubscriptionRequest` | `SubscriptionResponse` | Cancel subscription |
| PauseSubscription | `PauseSubscriptionRequest` | `SubscriptionResponse` | Pause billing |
| ResumeSubscription | `ResumeSubscriptionRequest` | `SubscriptionResponse` | Resume billing |
| GetSubscription | `GetSubscriptionRequest` | `Subscription` | Get subscription details |
| ListSubscriptions | `ListSubscriptionsRequest` | `ListSubscriptionsResponse` | List subscriptions (paginated) |

**Note:** Billing processing runs via HTTP cron endpoint (`POST /cron/process-billing`), not via RPC.

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
  "orderId": "ORD-2024-001",
  "amountCents": "10000",
  "currency": "USD",
  "paymentMethodId": "550e8400-e29b-41d4-a716-446655440001",
  "idempotencyKey": "550e8400-e29b-41d4-a716-446655440002",
  "metadata": {
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
  "transactionId": "550e8400-e29b-41d4-a716-446655440010",
  "amountCents": "5000",
  "reason": "Customer returned item",
  "idempotencyKey": "550e8400-e29b-41d4-a716-446655440011"
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

> **Note:** ACH transactions automatically use SEC code WEB (internet-initiated). For subscriptions, the billing service automatically uses PPD.

**Request (POST /payment.v1.PaymentService/Sale):**
```json
{
  "merchant_id": "00000000-0000-0000-0000-000000000001",
  "customer_id": "cust_1234567890",
  "order_id": "INV-2024-001",
  "payment_method_id": "550e8400-e29b-41d4-a716-446655440020",
  "amount_cents": 25000,
  "currency": "USD",
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440021",
  "metadata": {}
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

> **Note:** For PCI compliance, prefer using Browser Post ACH storage (`ACH_STORAGE_C` or `ACH_STORAGE_S`) instead of this direct API. See [Browser Post Form Setup](./BROWSER_POST_FORM_SETUP.md).

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
  "idempotencyKey": "550e8400-e29b-41d4-a716-446655440030"
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

#### Set Default Payment Method

**Request (POST /payment_method.v1.PaymentMethodService/SetDefaultPaymentMethod):**

**IMPORTANT:** Both `merchant_id` and `customer_id` are required. Requests without `customer_id` will fail.

```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "paymentMethodId": "550e8400-e29b-41d4-a716-446655440050",
  "customerId": "cust_1234567890"
}
```

**Response:**
```json
{
  "paymentMethodId": "550e8400-e29b-41d4-a716-446655440050",
  "isDefault": true
}
```

---

### Subscription Service

Manages recurring billing subscriptions with automatic payment processing.

#### Interval Units

| Value | Description | Example |
|-------|-------------|---------|
| `INTERVAL_UNIT_DAY` | Daily billing | `intervalValue: 1` = every day |
| `INTERVAL_UNIT_WEEK` | Weekly billing | `intervalValue: 2` = every 2 weeks |
| `INTERVAL_UNIT_MONTH` | Monthly billing | `intervalValue: 1` = every month |
| `INTERVAL_UNIT_YEAR` | Annual billing | `intervalValue: 1` = every year |

#### Subscription Statuses

| Status | Description |
|--------|-------------|
| `SUBSCRIPTION_STATUS_ACTIVE` | Subscription is active and will be billed |
| `SUBSCRIPTION_STATUS_PAUSED` | Temporarily paused, not billed until resumed |
| `SUBSCRIPTION_STATUS_CANCELLED` | Permanently cancelled |
| `SUBSCRIPTION_STATUS_PAST_DUE` | Payment failed after max retries |

---

#### Create Subscription

**Request (POST /subscription.v1.SubscriptionService/CreateSubscription):**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "amountCents": "2999",
  "currency": "USD",
  "intervalValue": 1,
  "intervalUnit": "INTERVAL_UNIT_MONTH",
  "paymentMethodId": "550e8400-e29b-41d4-a716-446655440040",
  "startDate": "2024-12-01T00:00:00Z",
  "maxRetries": 3,
  "metadata": {
    "plan_name": "Premium Monthly",
    "plan_id": "plan_premium_monthly"
  },
  "idempotencyKey": "550e8400-e29b-41d4-a716-446655440041"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchantId` | UUID | Yes | Merchant identifier |
| `customerId` | string | Yes | Customer identifier |
| `amountCents` | int64 | Yes | Amount in cents (e.g., 2999 = $29.99) |
| `currency` | string | Yes | ISO 4217 currency code (e.g., "USD") |
| `intervalValue` | int32 | Yes | Number of interval units between billings |
| `intervalUnit` | enum | Yes | Time unit: DAY, WEEK, MONTH, YEAR |
| `paymentMethodId` | UUID | Yes | Saved payment method to charge |
| `startDate` | timestamp | Yes | First billing date (RFC3339 format) |
| `maxRetries` | int32 | No | Max retry attempts on failure (default: 3) |
| `metadata` | map | No | Custom key-value metadata |
| `idempotencyKey` | UUID | Yes | Unique key for idempotency |

**Important:**
- `startDate` **MUST be RFC3339 timestamp format** (e.g., `"2024-12-01T00:00:00Z"`). Plain dates rejected.
- `idempotencyKey` and `paymentMethodId` **MUST be valid UUID format**

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
  "paymentMethodId": "550e8400-e29b-41d4-a716-446655440040",
  "nextBillingDate": "2024-12-01T00:00:00Z",
  "createdAt": "2024-11-23T10:30:00Z"
}
```

---

#### Get Subscription

**Request (POST /subscription.v1.SubscriptionService/GetSubscription):**
```json
{
  "subscriptionId": "sub_xyz789"
}
```

**Response:**
```json
{
  "id": "sub_xyz789",
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "amountCents": "2999",
  "currency": "USD",
  "intervalValue": 1,
  "intervalUnit": "INTERVAL_UNIT_MONTH",
  "status": "SUBSCRIPTION_STATUS_ACTIVE",
  "paymentMethodId": "550e8400-e29b-41d4-a716-446655440040",
  "nextBillingDate": "2024-12-01T00:00:00Z",
  "gatewaySubscriptionId": "epx_sub_123",
  "failureRetryCount": 0,
  "maxRetries": 3,
  "createdAt": "2024-11-23T10:30:00Z",
  "updatedAt": "2024-11-23T10:30:00Z",
  "metadata": {
    "plan_name": "Premium Monthly"
  }
}
```

---

#### Update Subscription

Update subscription amount, interval, or payment method. All fields are optional.

**Request (POST /subscription.v1.SubscriptionService/UpdateSubscription):**
```json
{
  "subscriptionId": "sub_xyz789",
  "amountCents": "3999",
  "intervalValue": 1,
  "intervalUnit": "INTERVAL_UNIT_MONTH",
  "paymentMethodId": "550e8400-e29b-41d4-a716-446655440050",
  "idempotencyKey": "550e8400-e29b-41d4-a716-446655440042"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `subscriptionId` | string | Yes | Subscription to update |
| `amountCents` | int64 | No | New amount in cents |
| `intervalValue` | int32 | No | New interval value |
| `intervalUnit` | enum | No | New interval unit |
| `paymentMethodId` | UUID | No | New payment method |
| `idempotencyKey` | UUID | Yes | Unique key for idempotency |

**Response:** Same as Create Subscription response with updated values.

---

#### Cancel Subscription

**Request (POST /subscription.v1.SubscriptionService/CancelSubscription):**
```json
{
  "subscriptionId": "sub_xyz789",
  "cancelAtPeriodEnd": true,
  "reason": "Customer requested cancellation",
  "idempotencyKey": "550e8400-e29b-41d4-a716-446655440043"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `subscriptionId` | string | Yes | Subscription to cancel |
| `cancelAtPeriodEnd` | bool | No | If true, cancels after current billing period ends. If false, cancels immediately. |
| `reason` | string | No | Cancellation reason for records |
| `idempotencyKey` | UUID | Yes | Unique key for idempotency |

**Response:**
```json
{
  "subscriptionId": "sub_xyz789",
  "status": "SUBSCRIPTION_STATUS_CANCELLED",
  "cancelledAt": "2024-11-25T15:30:00Z"
}
```

---

#### Pause Subscription

Temporarily pause billing. Subscription can be resumed later.

**Request (POST /subscription.v1.SubscriptionService/PauseSubscription):**
```json
{
  "subscriptionId": "sub_xyz789"
}
```

**Response:**
```json
{
  "subscriptionId": "sub_xyz789",
  "status": "SUBSCRIPTION_STATUS_PAUSED",
  "updatedAt": "2024-11-25T15:30:00Z"
}
```

---

#### Resume Subscription

Resume a paused subscription.

**Request (POST /subscription.v1.SubscriptionService/ResumeSubscription):**
```json
{
  "subscriptionId": "sub_xyz789"
}
```

**Response:**
```json
{
  "subscriptionId": "sub_xyz789",
  "status": "SUBSCRIPTION_STATUS_ACTIVE",
  "nextBillingDate": "2024-12-15T00:00:00Z",
  "updatedAt": "2024-11-25T15:30:00Z"
}
```

---

#### List Customer Subscriptions

**Request (POST /subscription.v1.SubscriptionService/ListCustomerSubscriptions):**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "status": "SUBSCRIPTION_STATUS_ACTIVE"
}
```

**Request Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchantId` | UUID | Yes | Merchant identifier |
| `customerId` | string | Yes | Customer identifier |
| `status` | enum | No | Filter by status (optional) |

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
      "paymentMethodId": "550e8400-e29b-41d4-a716-446655440040",
      "nextBillingDate": "2024-12-01T00:00:00Z",
      "createdAt": "2024-11-23T10:30:00Z"
    }
  ]
}
```

---

#### Recurring Billing Compliance

The subscription service automatically handles card network recurring billing indicators:
- First attempt of each billing cycle uses `ACI_EXT=RB` (Recurring Billing)
- Retries of failed transactions use `ACI_EXT=RS` (Resubmission) within 30-day window
- After 30 days from original decline, reverts to `ACI_EXT=RB`

**Retry Behavior:**
- Transient errors (network issues): Exponential backoff retry
- Permanent errors (declined): Recorded, may retry on next cron
- Max retries reached: Subscription status becomes `SUBSCRIPTION_STATUS_PAST_DUE`

This ensures compliance with Visa/Mastercard/Discover recurring transaction requirements.

---

### Chargeback Service

Chargebacks represent payment disputes synced from the North API. This service provides read-only access to dispute data for monitoring and webhook notifications. Merchants must respond to disputes via North's web portal.

#### Get Chargeback

**Request (POST /chargeback.v1.ChargebackService/GetChargeback):**
```json
{
  "chargebackId": "cb_abc123def456",
  "merchantId": "00000000-0000-0000-0000-000000000001"
}
```

**Response:**
```json
{
  "id": "cb_abc123def456",
  "transactionId": "tx_9876543210",
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "cust_1234567890",
  "caseNumber": "CB-2024-001234",
  "disputeDate": "2024-11-20T00:00:00Z",
  "chargebackDate": "2024-11-22T00:00:00Z",
  "chargebackAmount": "100.00",
  "currency": "USD",
  "reasonCode": "4837",
  "reasonDescription": "No Cardholder Authorization",
  "status": "CHARGEBACK_STATUS_NEW",
  "respondByDate": "2024-12-07T00:00:00Z",
  "evidenceFileUrls": [],
  "createdAt": "2024-11-23T10:30:00Z",
  "updatedAt": "2024-11-23T10:30:00Z"
}
```

#### List Chargebacks

**Request (POST /chargeback.v1.ChargebackService/ListChargebacks):**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "limit": 10,
  "offset": 0
}
```

**Request with filters:**
```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "status": "CHARGEBACK_STATUS_NEW",
  "customerId": "cust_1234567890",
  "transactionId": "tx_9876543210",
  "disputeDateFrom": "2024-11-01T00:00:00Z",
  "disputeDateTo": "2024-11-30T00:00:00Z",
  "limit": 10,
  "offset": 0
}
```

**Response:**
```json
{
  "chargebacks": [
    {
      "id": "cb_abc123def456",
      "transactionId": "tx_9876543210",
      "merchantId": "00000000-0000-0000-0000-000000000001",
      "customerId": "cust_1234567890",
      "caseNumber": "CB-2024-001234",
      "disputeDate": "2024-11-20T00:00:00Z",
      "chargebackDate": "2024-11-22T00:00:00Z",
      "chargebackAmount": "100.00",
      "currency": "USD",
      "reasonCode": "4837",
      "reasonDescription": "No Cardholder Authorization",
      "status": "CHARGEBACK_STATUS_NEW",
      "respondByDate": "2024-12-07T00:00:00Z",
      "createdAt": "2024-11-23T10:30:00Z",
      "updatedAt": "2024-11-23T10:30:00Z"
    }
  ],
  "totalCount": 1
}
```

**Chargeback Status Values:**
- `CHARGEBACK_STATUS_NEW` - Just received from North API
- `CHARGEBACK_STATUS_PENDING` - Under review
- `CHARGEBACK_STATUS_RESPONDED` - Evidence submitted via North portal
- `CHARGEBACK_STATUS_WON` - Merchant won the dispute
- `CHARGEBACK_STATUS_LOST` - Merchant lost the dispute
- `CHARGEBACK_STATUS_ACCEPTED` - Merchant accepted the chargeback

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

**IMPORTANT:** The `idempotency_key` MUST be a valid UUID format (e.g., `550e8400-e29b-41d4-a716-446655440000`).

**Best Practice:**
```typescript
// ✅ Correct - Valid UUID format
const idempotencyKey = crypto.randomUUID();
// Example: "550e8400-e29b-41d4-a716-446655440000"

// ❌ Wrong - Not a UUID, will be rejected
const idempotencyKey = `sale_${orderId}_${Date.now()}`;
```

**Behavior:**
- Same idempotency key returns same result (no duplicate charge)
- Different idempotency key creates new transaction
- Idempotency keys expire after 24 hours

---

## Testing with curl

### Prerequisites

1. **Generate a JWT Token:**
```bash
# Using paycli (recommended)
./paycli -action=generate-token -c=service_test-pos-system_credentials.json -o=token

# Save to environment variable
export TOKEN=$(./paycli -action=generate-token -c=service_test-pos-system_credentials.json -o=token)
```

2. **Set up environment:**
```bash
# Local development
export API_URL="http://localhost:8081"

# Staging
export API_URL="https://staging-api.example.com"
```

---

### Verified Working Examples (Sandbox)

The following curl commands have been tested and verified working against the EPX sandbox environment.

**Test Configuration:**
- Merchant ID: `f37b03e6-aef3-428d-984e-862af7e6b4e9` (sandbox merchant)
- Service ID: `test-pos-system`
- Server Port: `8081` (ConnectRPC + REST)

---

#### 1. Sale Transaction (CCE1/CCE2 - Credit Card Sale)

```bash
# Generate unique idempotency key (must be UUID format)
SALE_ID="$(cat /proc/sys/kernel/random/uuid)"

curl -s -X POST "${API_URL}/payment.v1.PaymentService/Sale" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d "{
    \"merchant_id\": \"f37b03e6-aef3-428d-984e-862af7e6b4e9\",
    \"customer_id\": \"test-customer-001\",
    \"amount_cents\": 5000,
    \"currency\": \"USD\",
    \"payment_method_id\": \"YOUR_PAYMENT_METHOD_ID\",
    \"idempotency_key\": \"${SALE_ID}\"
  }"
```

**Expected Response:**
```json
{
  "transactionId": "b1234567-89ab-4cde-f012-3456789abcde",
  "amountCents": "5000",
  "currency": "USD",
  "status": "TRANSACTION_STATUS_APPROVED",
  "type": "TRANSACTION_TYPE_CHARGE",
  "isApproved": true,
  "authorizationCode": "015321",
  "message": "EXACT MATCH",
  "card": {"brand": "visa"},
  "createdAt": "2025-12-02T14:58:02.870215Z"
}
```

---

#### 2. Refund Transaction (CCE9 - Credit Card Refund)

```bash
# Refund a previous transaction (partial or full)
REFUND_ID="$(cat /proc/sys/kernel/random/uuid)"
ORIGINAL_TX_ID="b1234567-89ab-4cde-f012-3456789abcde"  # Transaction ID to refund

curl -s -X POST "${API_URL}/payment.v1.PaymentService/Refund" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d "{
    \"transaction_id\": \"${ORIGINAL_TX_ID}\",
    \"amount_cents\": 2500,
    \"reason\": \"Customer requested partial refund\",
    \"idempotency_key\": \"${REFUND_ID}\"
  }"
```

**Expected Response:**
```json
{
  "transactionId": "c2345678-90bc-4def-0123-4567890abcde",
  "parentTransactionId": "b1234567-89ab-4cde-f012-3456789abcde",
  "amountCents": "2500",
  "currency": "USD",
  "status": "TRANSACTION_STATUS_APPROVED",
  "type": "TRANSACTION_TYPE_REFUND",
  "isApproved": true,
  "authorizationCode": "015324",
  "message": "EXACT MATCH",
  "card": {"brand": "visa"},
  "createdAt": "2025-12-02T14:58:34.432971Z"
}
```

---

#### 3. Void Transaction (CCEX - Credit Card Void)

```bash
# Void a transaction before settlement
VOID_ID="$(cat /proc/sys/kernel/random/uuid)"
TX_TO_VOID="d3456789-01cd-4ef0-1234-567890abcdef"  # Transaction ID to void

curl -s -X POST "${API_URL}/payment.v1.PaymentService/Void" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d "{
    \"transaction_id\": \"${TX_TO_VOID}\",
    \"idempotency_key\": \"${VOID_ID}\"
  }"
```

**Expected Response:**
```json
{
  "transactionId": "e4567890-12de-4f01-2345-678901234567",
  "parentTransactionId": "d3456789-01cd-4ef0-1234-567890abcdef",
  "amountCents": "3000",
  "currency": "USD",
  "status": "TRANSACTION_STATUS_APPROVED",
  "isApproved": true,
  "message": "APPROVAL",
  "createdAt": "2025-12-02T15:02:35.185663Z"
}
```

---

#### 4. Browser POST - Get Form Configuration

The Browser POST flow uses EPX's Key Exchange and Browser Post APIs per the [North Developer Browser Post API Integration Guide](https://developer.north.com/products/online/browser-post/).

**Two-Phase Flow:**
1. **Key Exchange** (our backend → EPX): Uses `TRAN_GROUP` = `SALE`, `AUTH`, `STORAGE`
2. **Browser POST Form** (user's browser → EPX): Uses `TRAN_CODE` field

**Transaction Types and EPX TRAN_CODE Values:**
| `transaction_type` | `tranCode` (for Browser POST form) | Description |
|--------------------|-----------------------------------|-------------|
| `SALE` | `SALE` | Charge card immediately |
| `AUTH` | `AUTH` | Hold funds, capture later |
| `STORAGE` | `STORAGE` | Tokenize credit card |
| `ACH_STORAGE_C` | `ACH_STORAGE_C` | Tokenize checking account |
| `ACH_STORAGE_S` | `ACH_STORAGE_S` | Tokenize savings account |

Our `/form` endpoint returns `tranCode` which the frontend should use in the Browser POST form's `TRAN_CODE` field.

##### 4a. Credit Card SALE (charge immediately)

```bash
FORM_TX_ID="$(cat /proc/sys/kernel/random/uuid)"

curl -s "http://localhost:8081/api/v1/payments/browser-post/form?transaction_id=${FORM_TX_ID}&merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&amount=50.00&transaction_type=SALE&return_url=http://localhost:8081/api/v1/payments/browser-post/callback&customer_id=test-customer-001" \
  -H "Authorization: Bearer ${TOKEN}"
```

**Expected Response:**
```json
{
  "custNbr": "9001",
  "dbaName": "2",
  "epxTranNbr": "707630394",
  "expiresAt": 1764701128,
  "industryType": "E",
  "merchNbr": "900300",
  "merchantId": "f37b03e6-aef3-428d-984e-862af7e6b4e9",
  "merchantName": "Test Merchant (Development)",
  "postURL": "https://services.epxuap.com/browserpost/",
  "redirectURL": "http://localhost:8081/api/v1/payments/browser-post/callback?...",
  "returnUrl": "http://localhost:8081/api/v1/payments/browser-post/callback",
  "tac": "4zBV/afFLkFkdJ3gPR4y6Q==|...",
  "terminalNbr": "77",
  "tranCode": "SALE",
  "transactionId": "a1b2c3d4-e5f6-4789-abcd-ef0123456789"
}
```

##### 4b. Credit Card AUTH (hold funds only, capture later)

```bash
FORM_TX_ID="$(cat /proc/sys/kernel/random/uuid)"

curl -s "http://localhost:8081/api/v1/payments/browser-post/form?transaction_id=${FORM_TX_ID}&merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&amount=50.00&transaction_type=AUTH&return_url=http://localhost:8081/api/v1/payments/browser-post/callback&customer_id=test-customer-001" \
  -H "Authorization: Bearer ${TOKEN}"
```

**Expected Response:** Same structure as SALE, with `"tranCode": "AUTH"`

##### 4c. Credit Card STORAGE (tokenize only, no charge)

Use this to save a card for future payments. Amount should be `0.00`.

```bash
FORM_TX_ID="$(cat /proc/sys/kernel/random/uuid)"

curl -s "http://localhost:8081/api/v1/payments/browser-post/form?transaction_id=${FORM_TX_ID}&merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&amount=0.00&transaction_type=STORAGE&return_url=http://localhost:8081/api/v1/payments/browser-post/callback&customer_id=test-customer-001" \
  -H "Authorization: Bearer ${TOKEN}"
```

**Expected Response:** Same structure, with `"tranCode": "STORAGE"`

After EPX callback, the payment method is automatically saved and can be used with Server POST Sale.

##### 4d. ACH Checking Account STORAGE

Tokenize a checking account for future ACH debits. Amount should be `0.00`.

```bash
FORM_TX_ID="$(cat /proc/sys/kernel/random/uuid)"

curl -s "http://localhost:8081/api/v1/payments/browser-post/form?transaction_id=${FORM_TX_ID}&merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&amount=0.00&transaction_type=ACH_STORAGE_C&return_url=http://localhost:8081/api/v1/payments/browser-post/callback&customer_id=test-customer-001" \
  -H "Authorization: Bearer ${TOKEN}"
```

**Expected Response:** Same structure, with `"tranCode": "ACH_STORAGE_C"`

##### 4e. ACH Savings Account STORAGE

Tokenize a savings account for future ACH debits. Amount should be `0.00`.

```bash
FORM_TX_ID="$(cat /proc/sys/kernel/random/uuid)"

curl -s "http://localhost:8081/api/v1/payments/browser-post/form?transaction_id=${FORM_TX_ID}&merchant_id=f37b03e6-aef3-428d-984e-862af7e6b4e9&amount=0.00&transaction_type=ACH_STORAGE_S&return_url=http://localhost:8081/api/v1/payments/browser-post/callback&customer_id=test-customer-001" \
  -H "Authorization: Bearer ${TOKEN}"
```

**Expected Response:** Same structure, with `"tranCode": "ACH_STORAGE_S"`

---

#### 5. List Payment Methods

```bash
curl -s -X POST "${API_URL}/payment_method.v1.PaymentMethodService/ListPaymentMethods" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "merchant_id": "f37b03e6-aef3-428d-984e-862af7e6b4e9",
    "customer_id": "test-customer-001"
  }'
```

**Expected Response:**
```json
{
  "paymentMethods": [
    {
      "id": "738abbff-8c3f-4d3f-92c1-ea443602b30a",
      "merchantId": "f37b03e6-aef3-428d-984e-862af7e6b4e9",
      "customerId": "test-customer-001",
      "paymentType": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
      "lastFour": "1111",
      "cardBrand": "visa",
      "isDefault": true,
      "isActive": true
    }
  ]
}
```

---

#### 6. Get Transaction Details

```bash
curl -s -X POST "${API_URL}/payment.v1.PaymentService/GetTransaction" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "transaction_id": "b1234567-89ab-4cde-f012-3456789abcde"
  }'
```

---

#### 7. List Transactions

```bash
curl -s -X POST "${API_URL}/payment.v1.PaymentService/ListTransactions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "merchant_id": "f37b03e6-aef3-428d-984e-862af7e6b4e9",
    "customer_id": "test-customer-001",
    "limit": 10,
    "offset": 0
  }'
```

**Filter by Order ID:**
```bash
curl -s -X POST "${API_URL}/payment.v1.PaymentService/ListTransactions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "merchant_id": "f37b03e6-aef3-428d-984e-862af7e6b4e9",
    "order_id": "ORD-2024-001",
    "limit": 10,
    "offset": 0
  }'
```

---

### Quick Test Script

Save as `test_payment_api.sh`:

```bash
#!/bin/bash
# Payment API Test Script
# Usage: ./test_payment_api.sh

set -e

# Configuration
API_URL="${API_URL:-http://localhost:8081}"
MERCHANT_ID="f37b03e6-aef3-428d-984e-862af7e6b4e9"
CUSTOMER_ID="test-customer-001"

# Generate token (adjust path to your credentials file)
TOKEN=$(./paycli -action=generate-token -c=service_test-pos-system_credentials.json -o=token)

echo "=== Payment API Test ==="
echo "API URL: $API_URL"
echo "Merchant: $MERCHANT_ID"
echo ""

# Test 1: Sale
SALE_ID=$(cat /proc/sys/kernel/random/uuid)
echo "Test 1: SALE (ID: $SALE_ID)"
SALE_RESULT=$(curl -s -X POST "${API_URL}/payment.v1.PaymentService/Sale" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d "{
    \"merchant_id\": \"${MERCHANT_ID}\",
    \"customer_id\": \"${CUSTOMER_ID}\",
    \"amount_cents\": 5000,
    \"currency\": \"USD\",
    \"payment_method_id\": \"YOUR_PAYMENT_METHOD_ID\",
    \"idempotency_key\": \"${SALE_ID}\"
  }")
echo "$SALE_RESULT" | jq -r '.status // .code'
echo ""

# Test 2: Refund (if sale succeeded)
if echo "$SALE_RESULT" | jq -e '.isApproved == true' > /dev/null 2>&1; then
  REFUND_ID=$(cat /proc/sys/kernel/random/uuid)
  echo "Test 2: REFUND (ID: $REFUND_ID)"
  curl -s -X POST "${API_URL}/payment.v1.PaymentService/Refund" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "{
      \"transaction_id\": \"${SALE_ID}\",
      \"amount_cents\": 2500,
      \"reason\": \"Test refund\",
      \"idempotency_key\": \"${REFUND_ID}\"
    }" | jq -r '.status // .code'
fi

echo ""
echo "=== Tests Complete ==="
```

---

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| `invalid idempotency_key format` | Non-UUID idempotency key | Use UUID format: `$(cat /proc/sys/kernel/random/uuid)` |
| `idempotency_key is required` | Missing idempotency_key | Add `idempotency_key` field to request |
| `failed to retrieve merchant credentials` | Invalid mac_secret_path | Verify merchant has correct `mac_secret_path` in database |
| `payment method does not belong to customer` | customer_id mismatch | Ensure customer_id matches the payment method's owner |
| `unauthenticated` | Invalid/expired JWT | Regenerate token with paycli |
| `permission_denied` | Service lacks merchant access | Grant service access via admin CLI |

---

## Test Data Setup

For local development and testing, the seed data provides pre-configured test entities.

### Available Test Merchants

| Merchant ID | Name | cust_nbr | merch_nbr | Environment |
|-------------|------|----------|-----------|-------------|
| `f37b03e6-aef3-428d-984e-862af7e6b4e9` | Test Merchant (Development) | 9001 | 900300 | sandbox |
| `00000000-0000-0000-0000-000000000001` | Test Merchant (Staging) | 9001 | 900300 | staging |

**Recommended for testing:** Use `f37b03e6-aef3-428d-984e-862af7e6b4e9` (sandbox merchant)

### Test Service

| Field | Value | Description |
|-------|-------|-------------|
| `service_id` | `test-pos-system` | Has access to sandbox merchant |
| Credentials File | `service_test-pos-system_credentials.json` | Contains private key for JWT |

**Generate a token for testing:**
```bash
./paycli -action=generate-token -c=service_test-pos-system_credentials.json -o=token
```

### Test Payment Methods

Payment methods are created via Browser POST flow. After running the seeder, you can create test payment methods using the browser-post-demo page at `http://localhost:8081/browser-post-demo`.

| Payment Method ID | Merchant | Customer ID | Type | Last 4 |
|-------------------|----------|-------------|------|--------|
| `738abbff-8c3f-4d3f-92c1-ea443602b30a` | sandbox | `test-customer-sandbox-001` | Credit Card | 1111 |

### Important Notes

1. **Idempotency Keys must be UUIDs**: Use `$(cat /proc/sys/kernel/random/uuid)` or `crypto.randomUUID()` to generate valid idempotency keys. Non-UUID formats will be rejected.

2. **Customer ID Consistency**: When using a `payment_method_id`, the `customer_id` in your request **must match** the customer_id associated with that payment method. Mismatched customer IDs will result in "payment method does not belong to customer" error.

3. **Merchant Access**: Your service JWT must have access to the merchant you're making requests for. The service-merchant relationship is configured during service creation.

4. **MAC Secret Path**: Ensure the merchant has a valid `mac_secret_path` configured (e.g., `epx/staging/mac_secret`) for Browser POST operations.

### Example Test Flow

```bash
# 1. Generate a fresh JWT token
TOKEN=$(./paycli -action=generate-token -c=service_test-pos-system_credentials.json -o=token)

# 2. Make a Sale request with matching customer_id
SALE_ID=$(cat /proc/sys/kernel/random/uuid)

curl -s -X POST "http://localhost:8081/payment.v1.PaymentService/Sale" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"merchant_id\": \"f37b03e6-aef3-428d-984e-862af7e6b4e9\",
    \"customer_id\": \"test-customer-sandbox-001\",
    \"amount_cents\": 5000,
    \"currency\": \"USD\",
    \"payment_method_id\": \"738abbff-8c3f-4d3f-92c1-ea443602b30a\",
    \"idempotency_key\": \"${SALE_ID}\"
  }" | jq '.'

# Expected output:
# {
#   "transactionId": "...",
#   "status": "TRANSACTION_STATUS_APPROVED",
#   "isApproved": true,
#   ...
# }
```

### Running the Seeder

**Sandbox Merchant Auto-Seeding:**

In development/staging environments, a sandbox merchant is automatically created on server startup using the `SANDBOX_MERCHANT_*` and `EPX_*` environment variables. No manual seeding required.

**For additional merchants or services:**
```bash
# Create a new service
./paycli -action=create-service

# Create a new merchant
./paycli -action=create-merchant

# Grant service access to merchant
./paycli -action=grant-access
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
// IMPORTANT: idempotencyKey MUST be a valid UUID, paymentMethodId MUST be a valid UUID
const response = await client.sale({
  merchantId: '00000000-0000-0000-0000-000000000001',
  customerId: 'cust_1234567890',
  amountCents: BigInt(10000),
  currency: 'USD',
  paymentMethodId: '550e8400-e29b-41d4-a716-446655440001',
  idempotencyKey: crypto.randomUUID(), // Must be UUID format
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
// IMPORTANT: IdempotencyKey MUST be a valid UUID, PaymentMethodId MUST be a valid UUID
response, err := client.Sale(ctx, &paymentv1.SaleRequest{
    MerchantId:      "00000000-0000-0000-0000-000000000001",
    CustomerId:      "cust_1234567890",
    AmountCents:     10000,
    Currency:        "USD",
    PaymentMethodId: "550e8400-e29b-41d4-a716-446655440001",
    IdempotencyKey:  uuid.New().String(), // Must be UUID format
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

