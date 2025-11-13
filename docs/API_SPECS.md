# API Specifications

**Audience:** Developers integrating with the payment service APIs.
**Topic:** Complete API reference for all endpoints.
**Goal:** Enable developers to successfully make requests and interpret responses.

## Overview

This service provides both **gRPC** (port 8080) and **HTTP REST** (port 8081) APIs via gRPC-Gateway. The REST APIs are automatically generated from the gRPC definitions.

**Base URLs:**
- gRPC: `localhost:8080` (binary protobuf)
- HTTP REST: `http://localhost:8081` (JSON)

**Authentication:** All requests require JWT token in `Authorization: Bearer <token>` header.

## Table of Contents

1. [Payment Service](#payment-service)
2. [Payment Method Service](#payment-method-service)
3. [Subscription Service](#subscription-service)
4. [Merchant Service](#merchant-service)
5. [Browser Post APIs](#browser-post-apis)
6. [Cron/Health APIs](#cronhealth-apis)
7. [Error Handling](#error-handling)

---

## Payment Service

Handles all payment transactions including authorize, capture, sale, void, and refund operations.

### Authorize Payment

Holds funds on a payment method without capturing them.

**gRPC:** `payment.v1.PaymentService/Authorize`
**HTTP:** `POST /api/v1/payments/authorize`

**Request:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "amount": "99.99",
  "currency": "USD",
  "payment_method_id": "pm-uuid-here",
  "idempotency_key": "auth_20250113_001",
  "metadata": {
    "order_id": "ORDER-12345"
  }
}
```

**Alternative - Using One-Time Token:**
```json
{
  "merchant_id": "merchant-123",
  "amount": "99.99",
  "currency": "USD",
  "payment_token": "epx-bric-token",
  "idempotency_key": "auth_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | No | Customer identifier (null for guest) |
| `amount` | string | Yes | Decimal as string (e.g., "29.99") |
| `currency` | string | Yes | ISO 4217 code (e.g., "USD") |
| `payment_method_id` | string | Yes* | UUID of saved payment method |
| `payment_token` | string | Yes* | EPX BRIC token for one-time use |
| `idempotency_key` | string | Yes | Unique key to prevent duplicates |
| `metadata` | object | No | Key-value pairs for additional data |

*Either `payment_method_id` or `payment_token` required, not both.

**Response (200 OK):**
```json
{
  "transaction_id": "tx-uuid-here",
  "group_id": "grp-uuid-here",
  "amount": "99.99",
  "currency": "USD",
  "status": "TRANSACTION_STATUS_APPROVED",
  "type": "TRANSACTION_TYPE_AUTH",
  "is_approved": true,
  "authorization_code": "123456",
  "message": "Approved",
  "card": {
    "brand": "visa",
    "last_four": "1111"
  },
  "created_at": "2025-01-13T12:00:00Z"
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `transaction_id` | string | Unique transaction identifier |
| `group_id` | string | Groups related transactions (store this!) |
| `amount` | string | Transaction amount |
| `currency` | string | Currency code |
| `status` | enum | APPROVED or DECLINED |
| `type` | enum | Transaction type (AUTH, CAPTURE, etc.) |
| `is_approved` | boolean | Quick approval check |
| `authorization_code` | string | Bank authorization code |
| `message` | string | Human-readable response |
| `card` | object | Card information for display |
| `created_at` | timestamp | Transaction creation time |

---

### Capture Payment

Completes a previously authorized payment.

**gRPC:** `payment.v1.PaymentService/Capture`
**HTTP:** `POST /api/v1/payments/capture`

**Request:**
```json
{
  "transaction_id": "tx-uuid-here",
  "amount": "75.00",
  "idempotency_key": "capture_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `transaction_id` | string | Yes | Original authorization transaction ID |
| `amount` | string | No | Partial capture amount (omit for full) |
| `idempotency_key` | string | Yes | Unique key to prevent duplicates |

**Example - Full Capture:**
```json
{
  "transaction_id": "tx-uuid-here",
  "idempotency_key": "capture_20250113_001"
}
```

**Example - Partial Capture:**
```json
{
  "transaction_id": "tx-uuid-here",
  "amount": "75.00",
  "idempotency_key": "capture_20250113_001"
}
```

**Response:** Same format as `PaymentResponse` above with `type: "TRANSACTION_TYPE_CAPTURE"`.

---

### Sale

Combines authorize and capture in one operation.

**gRPC:** `payment.v1.PaymentService/Sale`
**HTTP:** `POST /api/v1/payments/sale`

**Request:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "amount": "49.99",
  "currency": "USD",
  "payment_method_id": "pm-uuid-here",
  "idempotency_key": "sale_20250113_001",
  "metadata": {
    "invoice_id": "INV-67890"
  }
}
```

**Parameters:** Same as Authorize.

**Response:** Same format as `PaymentResponse` with `type: "TRANSACTION_TYPE_CHARGE"`.

---

### Void Payment

Cancels an authorized or captured payment before settlement.

**gRPC:** `payment.v1.PaymentService/Void`
**HTTP:** `POST /api/v1/payments/void`

**Request:**
```json
{
  "group_id": "grp-uuid-here",
  "idempotency_key": "void_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `group_id` | string | Yes | Transaction group to void |
| `idempotency_key` | string | Yes | Unique key to prevent duplicates |

**Important:** Use `group_id` (not `transaction_id`) to ensure correct transaction is voided.

**Response:** Same format as `PaymentResponse` with `type: "TRANSACTION_TYPE_VOID"`.

---

### Refund Payment

Returns funds to the customer after settlement.

**gRPC:** `payment.v1.PaymentService/Refund`
**HTTP:** `POST /api/v1/payments/refund`

**Request - Full Refund:**
```json
{
  "group_id": "grp-uuid-here",
  "reason": "Customer requested refund",
  "idempotency_key": "refund_20250113_001"
}
```

**Request - Partial Refund:**
```json
{
  "group_id": "grp-uuid-here",
  "amount": "30.00",
  "reason": "Partial order cancellation",
  "idempotency_key": "refund_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `group_id` | string | Yes | Transaction group to refund |
| `amount` | string | No | Partial refund amount (omit for full) |
| `reason` | string | No | Refund reason for records |
| `idempotency_key` | string | Yes | Unique key to prevent duplicates |

**Note:** Multiple partial refunds allowed until full amount refunded.

**Response:** Same format as `PaymentResponse` with `type: "TRANSACTION_TYPE_REFUND"`.

---

### Get Transaction

Retrieves details of a specific transaction.

**gRPC:** `payment.v1.PaymentService/GetTransaction`
**HTTP:** `GET /api/v1/payments/{transaction_id}`

**Example:**
```http
GET /api/v1/payments/tx-uuid-here
```

**Response (200 OK):**
```json
{
  "id": "tx-uuid-here",
  "group_id": "grp-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "amount": "99.99",
  "currency": "USD",
  "status": "TRANSACTION_STATUS_APPROVED",
  "type": "TRANSACTION_TYPE_CHARGE",
  "payment_method_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "payment_method_id": "pm-uuid-here",
  "authorization_code": "123456",
  "message": "Approved",
  "card": {
    "brand": "visa",
    "last_four": "1111"
  },
  "idempotency_key": "sale_20250113_001",
  "created_at": "2025-01-13T12:00:00Z",
  "updated_at": "2025-01-13T12:00:00Z"
}
```

---

### List Transactions

Lists transactions with filtering options.

**gRPC:** `payment.v1.PaymentService/ListTransactions`
**HTTP:** `GET /api/v1/payments`

**Query Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes* | Merchant identifier |
| `customer_id` | string | No | Filter by customer |
| `group_id` | string | No | Get all transactions in group |
| `status` | enum | No | Filter by status |
| `limit` | int32 | No | Max results (default: 100) |
| `offset` | int32 | No | Pagination offset |

*Required unless using customer or admin token (see AUTH.md).

**Example - By Customer:**
```http
GET /api/v1/payments?merchant_id=merchant-123&customer_id=customer-456&limit=50
```

**Example - By Group (get sale + refunds):**
```http
GET /api/v1/payments?merchant_id=merchant-123&group_id=grp-uuid-here
```

**Response (200 OK):**
```json
{
  "transactions": [
    {
      "id": "tx-1",
      "group_id": "grp-uuid",
      "type": "TRANSACTION_TYPE_CHARGE",
      "status": "TRANSACTION_STATUS_APPROVED",
      "amount": "100.00",
      "created_at": "2025-01-13T12:00:00Z"
    },
    {
      "id": "tx-2",
      "group_id": "grp-uuid",
      "type": "TRANSACTION_TYPE_REFUND",
      "status": "TRANSACTION_STATUS_APPROVED",
      "amount": "30.00",
      "created_at": "2025-01-13T13:00:00Z"
    }
  ],
  "total_count": 2
}
```

---

## Payment Method Service

Manages saved payment methods (tokenized cards and ACH accounts).

### Save Payment Method

Stores a tokenized payment method for future use.

**gRPC:** `payment_method.v1.PaymentMethodService/SavePaymentMethod`
**HTTP:** `POST /api/v1/payment-methods`

**Request - Credit Card:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "payment_token": "epx-bric-token",
  "payment_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "last_four": "1111",
  "card_brand": "visa",
  "card_exp_month": 12,
  "card_exp_year": 2025,
  "is_default": true,
  "idempotency_key": "save_pm_20250113_001"
}
```

**Request - ACH:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "payment_token": "ach-bric-token",
  "payment_type": "PAYMENT_METHOD_TYPE_ACH",
  "last_four": "7890",
  "bank_name": "Chase",
  "account_type": "checking",
  "is_default": false,
  "idempotency_key": "save_pm_20250113_002"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `payment_token` | string | Yes | EPX BRIC token |
| `payment_type` | enum | Yes | CREDIT_CARD or ACH |
| `last_four` | string | Yes | Last 4 digits |
| `card_brand` | string | No* | Card brand (visa, mastercard, etc.) |
| `card_exp_month` | int32 | No* | Expiration month (1-12) |
| `card_exp_year` | int32 | No* | Expiration year (YYYY) |
| `bank_name` | string | No** | Bank name |
| `account_type` | string | No** | checking or savings |
| `is_default` | boolean | No | Mark as default (default: false) |
| `idempotency_key` | string | Yes | Unique key |

*Required for credit cards
**Required for ACH

**Response (201 Created):**
```json
{
  "payment_method_id": "pm-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "payment_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "last_four": "1111",
  "card_brand": "visa",
  "card_exp_month": 12,
  "card_exp_year": 2025,
  "is_default": true,
  "is_active": true,
  "is_verified": false,
  "created_at": "2025-01-13T12:00:00Z"
}
```

---

### Get Payment Method

Retrieves a specific payment method.

**gRPC:** `payment_method.v1.PaymentMethodService/GetPaymentMethod`
**HTTP:** `GET /api/v1/payment-methods/{payment_method_id}`

**Example:**
```http
GET /api/v1/payment-methods/pm-uuid-here
```

**Response (200 OK):**
```json
{
  "id": "pm-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "payment_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "last_four": "1111",
  "card_brand": "visa",
  "card_exp_month": 12,
  "card_exp_year": 2025,
  "is_default": true,
  "is_active": true,
  "is_verified": false,
  "created_at": "2025-01-13T12:00:00Z",
  "updated_at": "2025-01-13T12:00:00Z"
}
```

---

### List Payment Methods

Lists all payment methods for a customer.

**gRPC:** `payment_method.v1.PaymentMethodService/ListPaymentMethods`
**HTTP:** `GET /api/v1/payment-methods`

**Query Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `payment_type` | enum | No | Filter by type |
| `is_active` | boolean | No | Filter by active status |

**Example:**
```http
GET /api/v1/payment-methods?merchant_id=merchant-123&customer_id=customer-456
```

**Response (200 OK):**
```json
{
  "payment_methods": [
    {
      "id": "pm-1",
      "payment_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
      "last_four": "1111",
      "card_brand": "visa",
      "card_exp_month": 12,
      "card_exp_year": 2025,
      "is_default": true,
      "is_active": true,
      "created_at": "2025-01-13T10:00:00Z"
    }
  ]
}
```

---

### Delete Payment Method

Soft-deletes a payment method (sets `deleted_at` timestamp).

**gRPC:** `payment_method.v1.PaymentMethodService/DeletePaymentMethod`
**HTTP:** `DELETE /api/v1/payment-methods/{payment_method_id}`

**Request:**
```json
{
  "payment_method_id": "pm-uuid-here",
  "idempotency_key": "delete_pm_20250113_001"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Payment method deleted successfully"
}
```

---

## Subscription Service

Handles recurring billing subscriptions.

### Create Subscription

Creates a new recurring billing subscription.

**gRPC:** `subscription.v1.SubscriptionService/CreateSubscription`
**HTTP:** `POST /api/v1/subscriptions`

**Request:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "amount": "29.99",
  "currency": "USD",
  "interval_value": 1,
  "interval_unit": "INTERVAL_UNIT_MONTH",
  "payment_method_id": "pm-uuid-here",
  "start_date": "2025-01-15T00:00:00Z",
  "max_retries": 3,
  "metadata": {
    "plan_name": "Premium Monthly"
  },
  "idempotency_key": "sub_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `amount` | string | Yes | Decimal as string |
| `currency` | string | Yes | ISO 4217 code |
| `interval_value` | int32 | Yes | Billing interval (1, 2, 3, etc.) |
| `interval_unit` | enum | Yes | DAY, WEEK, MONTH, YEAR |
| `payment_method_id` | string | Yes | Saved payment method UUID |
| `start_date` | timestamp | No | When to start (default: now) |
| `max_retries` | int32 | No | Max retry attempts (default: 3) |
| `metadata` | object | No | Additional data |
| `idempotency_key` | string | Yes | Unique key |

**Response (201 Created):**
```json
{
  "subscription_id": "sub-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "amount": "29.99",
  "currency": "USD",
  "interval_value": 1,
  "interval_unit": "INTERVAL_UNIT_MONTH",
  "status": "SUBSCRIPTION_STATUS_ACTIVE",
  "payment_method_id": "pm-uuid-here",
  "next_billing_date": "2025-02-15T00:00:00Z",
  "created_at": "2025-01-13T12:00:00Z"
}
```

---

### Get Subscription

Retrieves subscription details.

**gRPC:** `subscription.v1.SubscriptionService/GetSubscription`
**HTTP:** `GET /api/v1/subscriptions/{subscription_id}`

**Example:**
```http
GET /api/v1/subscriptions/sub-uuid-here
```

**Response:** Same format as `SubscriptionResponse`.

---

### List Customer Subscriptions

Lists all subscriptions for a customer.

**gRPC:** `subscription.v1.SubscriptionService/ListCustomerSubscriptions`
**HTTP:** `GET /api/v1/subscriptions`

**Query Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `status` | enum | No | Filter by status |

**Example:**
```http
GET /api/v1/subscriptions?merchant_id=merchant-123&customer_id=customer-456&status=SUBSCRIPTION_STATUS_ACTIVE
```

---

### Cancel Subscription

Cancels an active subscription.

**gRPC:** `subscription.v1.SubscriptionService/CancelSubscription`
**HTTP:** `POST /api/v1/subscriptions/{subscription_id}/cancel`

**Request:**
```json
{
  "subscription_id": "sub-uuid-here",
  "cancel_at_period_end": false,
  "reason": "Customer requested cancellation",
  "idempotency_key": "cancel_sub_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `subscription_id` | string | Yes | Subscription identifier |
| `cancel_at_period_end` | boolean | No | Cancel after current period (default: false) |
| `reason` | string | No | Cancellation reason |
| `idempotency_key` | string | Yes | Unique key |

---

### Pause Subscription

Pauses billing temporarily.

**gRPC:** `subscription.v1.SubscriptionService/PauseSubscription`
**HTTP:** `POST /api/v1/subscriptions/{subscription_id}/pause`

**Request:**
```json
{
  "subscription_id": "sub-uuid-here"
}
```

---

### Resume Subscription

Resumes a paused subscription.

**gRPC:** `subscription.v1.SubscriptionService/ResumeSubscription`
**HTTP:** `POST /api/v1/subscriptions/{subscription_id}/resume`

**Request:**
```json
{
  "subscription_id": "sub-uuid-here"
}
```

---

## Merchant Service

**Internal/Admin use only** - Manages multi-tenant merchant credentials.

**Note:** This service is gRPC-only (no HTTP REST endpoints).

### Register Merchant

Adds a new merchant to the system.

**gRPC:** `merchant.v1.MerchantService/RegisterMerchant`

**Request:**
```protobuf
{
  merchant_id: "merchant-123"
  mac_secret: "secret-key-here"
  cust_nbr: "9001"
  merch_nbr: "900300"
  dba_nbr: "2"
  terminal_nbr: "77"
  environment: ENVIRONMENT_SANDBOX
  metadata: {"business_name": "Acme Corp"}
  idempotency_key: "register_20250113_001"
}
```

---

## Browser Post APIs

PCI-compliant payment form generation and callback handling.

### Get Payment Form

Generates configuration for browser-based payment form.

**HTTP:** `GET /api/v1/payments/browser-post/form`

**Query Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `amount` | string | Yes | Transaction amount |
| `return_url` | string | Yes | Where to redirect after payment |
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | No | Customer identifier |

**Example:**
```http
GET /api/v1/payments/browser-post/form?amount=99.99&return_url=https://app.example.com/complete&merchant_id=merchant-123
```

**Response (200 OK):**
```json
{
  "transaction_id": "tx-uuid-here",
  "group_id": "grp-uuid-here",
  "post_url": "https://secure.epxuap.com/browserpost",
  "amount": "99.99",
  "tran_nbr": "45062844883",
  "redirect_url": "http://payment-service:8081/api/v1/payments/browser-post/callback",
  "user_data_1": "return_url=https://app.example.com/complete",
  "cust_nbr": "9001",
  "merch_nbr": "900300"
}
```

**Usage:** See DATAFLOW.md for complete integration guide.

---

### Handle Payment Callback

**Internal endpoint** - Receives EPX callback after payment processing.

**HTTP:** `POST /api/v1/payments/browser-post/callback`

**Note:** Called by EPX, not by your application. Browser is automatically redirected here.

---

## Cron/Health APIs

Administrative endpoints for health checks and scheduled tasks.

### Health Check

Returns service health status.

**HTTP:** `GET /cron/health`

**Response (200 OK):**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-13T12:00:00Z"
}
```

---

### Process Recurring Billing

Triggers subscription billing for due subscriptions.

**HTTP:** `POST /cron/process-billing`
**Authentication:** Requires `X-Cron-Secret` header

**Response (200 OK):**
```json
{
  "processed": 45,
  "successful": 42,
  "failed": 3,
  "duration_ms": 1234
}
```

---

## Error Handling

### HTTP Status Codes

| Code | Meaning | When |
|------|---------|------|
| 200 | OK | Successful operation |
| 201 | Created | Resource created |
| 400 | Bad Request | Validation error |
| 401 | Unauthorized | Missing/invalid auth |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Duplicate idempotency key |
| 500 | Internal Server Error | Service error |

### Error Response Format

All errors follow gRPC-Gateway format:

```json
{
  "code": 3,
  "message": "payment_token is required",
  "details": []
}
```

**Common Error Codes:**
- `3` - INVALID_ARGUMENT (validation error)
- `5` - NOT_FOUND (resource not found)
- `7` - PERMISSION_DENIED (authorization failed)
- `13` - INTERNAL (server error)

### Validation Errors

```json
{
  "code": 3,
  "message": "amount is required",
  "details": []
}
```

### Gateway Errors

When EPX rejects a payment:

```json
{
  "transaction_id": "tx-uuid",
  "group_id": "grp-uuid",
  "status": "TRANSACTION_STATUS_DECLINED",
  "is_approved": false,
  "message": "Insufficient funds"
}
```

---

## Best Practices

### 1. Always Use Idempotency Keys

Prevent duplicate charges:

```json
{
  "merchant_id": "merchant-123",
  "amount": "99.99",
  "idempotency_key": "sale_20250113_001"
}
```

### 2. Use group_id for Refunds/Voids

Always use `group_id` (not `transaction_id`):

```json
{
  "group_id": "grp-uuid-here",
  "amount": "30.00"
}
```

### 3. Store group_id with Orders

Link payments to orders:

```sql
UPDATE orders
SET payment_group_id = 'grp-uuid-here'
WHERE order_id = 'ORDER-123';
```

### 4. Handle Async Callbacks

Browser Post completes asynchronously:
1. Accept `group_id` in return URL
2. Look up order by `group_id`
3. Update order status
4. Render receipt

### 5. Test with Sandbox

Use EPX sandbox test cards:
- Visa: `4111111111111111`
- Mastercard: `5555555555554444`
- CVV: `123`, Expiry: `12/2025`

---

## Rate Limits

Browser Post endpoints:
- **10 requests/second per IP**
- **Burst of 20 requests**

Exceeding limits returns:
```json
{
  "code": 8,
  "message": "Rate limit exceeded"
}
```

---

## References

- Dataflows: `DATAFLOW.md`
- Authentication: `AUTH.md`
- Database Schema: `DATABASE.md`
- Development Guide: `DEVELOP.md`
- CI/CD: `CICD.md`