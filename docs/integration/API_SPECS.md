# API Specifications

**Audience:** Developers integrating with the payment service APIs.
**Topic:** Complete API reference for all endpoints.
**Goal:** Enable developers to successfully make requests and interpret responses.

## Overview

This service provides **two types of APIs**:

1. **ConnectRPC APIs** (Port 8080): Modern RPC framework for payment operations
2. **REST APIs** (Port 8081): Browser Post and Cron endpoints

### ConnectRPC Protocol

ConnectRPC is a modern, type-safe RPC framework that:
- Uses **HTTP POST** for all RPC method calls
- Supports HTTP/1.1, HTTP/2, and HTTP/3
- Compatible with gRPC, gRPC-Web, and Connect clients
- Works with plain `curl` and standard HTTP libraries

**Base URLs:**
- **ConnectRPC APIs**: `http://localhost:8080` (all RPC services)
- **REST APIs**: `http://localhost:8081` (Browser Post, Cron jobs)

**Authentication:** All ConnectRPC requests require JWT token in `Authorization: Bearer <token>` header.

**URL Format:** `/{package}.{service}/{Method}`
- Example: `POST /payment.v1.PaymentService/Authorize`

**Request Format:**
```http
POST /{package}.{service}/{Method} HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Authorization: Bearer <jwt-token>

{
  "field": "value"
}
```

### Example cURL Request

```bash
curl -X POST http://localhost:8080/payment.v1.PaymentService/Authorize \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGc..." \
  -d '{
    "merchant_id": "merchant-123",
    "customer_id": "customer-456",
    "amount_cents": 9999,
    "currency": "USD",
    "payment_method_id": "pm-uuid-here",
    "idempotency_key": "auth_20250113_001"
  }'
```

### ConnectRPC Client Libraries

For production integrations, use official Connect client libraries:
- **Go**: `go get connectrpc.com/connect`
- **TypeScript/JavaScript**: `npm install @connectrpc/connect`
- **Swift**: Connect Swift client
- **Kotlin**: Connect Kotlin client

See [ConnectRPC documentation](https://connectrpc.com/docs/) for client setup guides.

## Table of Contents

**ConnectRPC APIs (Port 8080):**
1. [Payment Service](#payment-service)
2. [Payment Method Service](#payment-method-service)
3. [Subscription Service](#subscription-service)
4. [Merchant Service](#merchant-service)

**REST APIs (Port 8081):**
5. [Browser Post APIs](#browser-post-apis-rest)
6. [Cron/Health APIs](#cronhealth-apis-rest)

**General:**
7. [Error Handling](#error-handling)
8. [Best Practices](#best-practices)

---

## Payment Service

Handles all payment transactions including authorize, capture, sale, void, and refund operations.

### Authorize Payment

Holds funds on a payment method without capturing them.

**ConnectRPC:** `payment.v1.PaymentService/Authorize`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/Authorize`

**Request:**
```http
POST /payment.v1.PaymentService/Authorize HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Authorization: Bearer <jwt-token>

{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "amount_cents": 9999,
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
  "amount_cents": 9999,
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
| `amount_cents` | int64 | Yes | Amount in cents (e.g., 9999 = $99.99) |
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
  "parent_transaction_id": "",
  "amount_cents": 9999,
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
| `parent_transaction_id` | string | Links to parent transaction (empty for initial auth) |
| `amount_cents` | int64 | Amount in cents (e.g., 9999 = $99.99) |
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

**ConnectRPC:** `payment.v1.PaymentService/Capture`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/Capture`

**Request:**
```http
POST /payment.v1.PaymentService/Capture HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Authorization: Bearer <jwt-token>

{
  "transaction_id": "tx-uuid-here",
  "amount_cents": 7500,
  "idempotency_key": "capture_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `transaction_id` | string | Yes | Original authorization transaction ID |
| `amount_cents` | int64 | No | Partial capture amount in cents (omit for full) |
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
  "amount_cents": 7500,
  "idempotency_key": "capture_20250113_001"
}
```

**Response:** Same format as `PaymentResponse` above with `type: "TRANSACTION_TYPE_CAPTURE"`.

---

### Sale

Combines authorize and capture in one operation.

**ConnectRPC:** `payment.v1.PaymentService/Sale`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/Sale`

**Request:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "amount_cents": 4999,
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

**ConnectRPC:** `payment.v1.PaymentService/Void`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/Void`

**Request:**
```json
{
  "transaction_id": "tx-uuid-here",
  "idempotency_key": "void_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `transaction_id` | string | Yes | Transaction ID to void |
| `idempotency_key` | string | Yes | Unique key to prevent duplicates |

**Note:** This creates a new VOID transaction with the provided transaction_id as its parent_transaction_id.

**Response:** Same format as `PaymentResponse` with `type: "TRANSACTION_TYPE_VOID"`.

---

### Refund Payment

Returns funds to the customer after settlement.

**ConnectRPC:** `payment.v1.PaymentService/Refund`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/Refund`

**Request - Full Refund:**
```json
{
  "transaction_id": "tx-uuid-here",
  "reason": "Customer requested refund",
  "idempotency_key": "refund_20250113_001"
}
```

**Request - Partial Refund:**
```json
{
  "transaction_id": "tx-uuid-here",
  "amount_cents": 3000,
  "reason": "Partial order cancellation",
  "idempotency_key": "refund_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `transaction_id` | string | Yes | Transaction ID to refund |
| `amount_cents` | int64 | No | Partial refund amount in cents (omit for full) |
| `reason` | string | No | Refund reason for records |
| `idempotency_key` | string | Yes | Unique key to prevent duplicates |

**Note:** Multiple partial refunds allowed until full amount refunded. This creates a new REFUND transaction with the provided transaction_id as its parent_transaction_id.

**Response:** Same format as `PaymentResponse` with `type: "TRANSACTION_TYPE_REFUND"`.

---

### ACH Debit

Pulls money from a bank account using a stored ACH payment method.

**ConnectRPC:** `payment.v1.PaymentService/ACHDebit`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/ACHDebit`

**Request:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "payment_method_id": "pm-ach-uuid-here",
  "amount_cents": 5000,
  "currency": "USD",
  "idempotency_key": "ach_debit_20250113_001",
  "metadata": {
    "invoice_id": "INV-12345"
  }
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | No | Customer identifier (optional for guest transactions) |
| `payment_method_id` | string | Yes | ACH Storage BRIC (UUID of saved ACH payment method) |
| `amount_cents` | int64 | Yes | Amount in cents (e.g., 5000 = $50.00) |
| `currency` | string | Yes | ISO 4217 code (e.g., "USD") |
| `idempotency_key` | string | Yes | Unique key to prevent duplicates |
| `metadata` | object | No | Key-value pairs for additional data |

**Response:** Same format as `PaymentResponse` with `type: "TRANSACTION_TYPE_CHARGE"`.

---

### ACH Credit

Sends money to a bank account (e.g., for payouts or refunds).

**ConnectRPC:** `payment.v1.PaymentService/ACHCredit`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/ACHCredit`

**Request:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "payment_method_id": "pm-ach-uuid-here",
  "amount_cents": 2500,
  "currency": "USD",
  "reason": "Refund for order #12345",
  "idempotency_key": "ach_credit_20250113_001",
  "metadata": {
    "order_id": "ORDER-12345"
  }
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | No | Customer identifier (optional) |
| `payment_method_id` | string | Yes | ACH Storage BRIC (UUID of saved ACH payment method) |
| `amount_cents` | int64 | Yes | Amount in cents (e.g., 2500 = $25.00) |
| `currency` | string | Yes | ISO 4217 code |
| `reason` | string | No | Reason for credit (e.g., "refund", "payout") |
| `idempotency_key` | string | Yes | Unique key to prevent duplicates |
| `metadata` | object | No | Key-value pairs for additional data |

**Response:** Same format as `PaymentResponse` with appropriate transaction type.

---

### ACH Void

Cancels an ACH transaction before settlement.

**ConnectRPC:** `payment.v1.PaymentService/ACHVoid`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/ACHVoid`

**Request:**
```json
{
  "transaction_id": "tx-ach-uuid-here",
  "idempotency_key": "ach_void_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `transaction_id` | string | Yes | Original ACH transaction ID to void |
| `idempotency_key` | string | Yes | Unique key to prevent duplicates |

**Response:** Same format as `PaymentResponse` with `type: "TRANSACTION_TYPE_VOID"`.

---

### Get Transaction

Retrieves details of a specific transaction.

**ConnectRPC:** `payment.v1.PaymentService/GetTransaction`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/GetTransaction`

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

**ConnectRPC:** `payment.v1.PaymentService/ListTransactions`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment.v1.PaymentService/ListTransactions`

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

**Note:** Credit card tokenization happens via Browser Post workflow. Use StoreACHAccount for saving bank accounts.

### Store ACH Account

Creates an ACH Storage BRIC and sends a pre-note for verification.

**ConnectRPC:** `payment_method.v1.PaymentMethodService/StoreACHAccount`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment_method.v1.PaymentMethodService/StoreACHAccount`

**Request:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "account_number": "123456789",
  "routing_number": "021000021",
  "account_holder_name": "John Doe",
  "account_type": "ACCOUNT_TYPE_CHECKING",
  "std_entry_class": "STD_ENTRY_CLASS_WEB",
  "first_name": "John",
  "last_name": "Doe",
  "address": "123 Main St",
  "city": "New York",
  "state": "NY",
  "zip_code": "10001",
  "bank_name": "Chase",
  "nickname": "Primary checking",
  "is_default": true,
  "idempotency_key": "store_ach_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `account_number` | string | Yes | Bank account number |
| `routing_number` | string | Yes | Bank routing number |
| `account_holder_name` | string | Yes | Name on account |
| `account_type` | enum | Yes | CHECKING or SAVINGS |
| `std_entry_class` | enum | Yes | PPD, CCD, WEB, or TEL |
| `first_name` | string | No | Billing first name |
| `last_name` | string | No | Billing last name |
| `address` | string | No | Billing address |
| `city` | string | No | Billing city |
| `state` | string | No | Billing state |
| `zip_code` | string | No | Billing ZIP code |
| `bank_name` | string | No | Bank name for display |
| `nickname` | string | No | Custom nickname |
| `is_default` | boolean | No | Mark as default (default: false) |
| `idempotency_key` | string | Yes | Unique key |

**Response (201 Created):**
```json
{
  "payment_method_id": "pm-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "payment_type": "PAYMENT_METHOD_TYPE_ACH",
  "last_four": "6789",
  "bank_name": "Chase",
  "account_type": "checking",
  "is_default": true,
  "is_active": true,
  "is_verified": false,
  "created_at": "2025-01-13T12:00:00Z"
}
```

**Note:** A pre-note transaction is automatically sent for ACH verification. The account will be marked as `is_verified: true` once verification completes.

---

### Get Payment Method

Retrieves a specific payment method.

**ConnectRPC:** `payment_method.v1.PaymentMethodService/GetPaymentMethod`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment_method.v1.PaymentMethodService/GetPaymentMethod`

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

**ConnectRPC:** `payment_method.v1.PaymentMethodService/ListPaymentMethods`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment_method.v1.PaymentMethodService/ListPaymentMethods`

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

**ConnectRPC:** `payment_method.v1.PaymentMethodService/DeletePaymentMethod`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment_method.v1.PaymentMethodService/DeletePaymentMethod`

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

### Set Default Payment Method

Marks a payment method as the default for a customer.

**ConnectRPC:** `payment_method.v1.PaymentMethodService/SetDefaultPaymentMethod`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment_method.v1.PaymentMethodService/SetDefaultPaymentMethod`

**Request:**
```json
{
  "payment_method_id": "pm-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `payment_method_id` | string | Yes | Payment method ID to set as default |
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |

**Response:** Same format as `PaymentMethodResponse`.

---

### Update Payment Method Status

Activates or deactivates a payment method.

**ConnectRPC:** `payment_method.v1.PaymentMethodService/UpdatePaymentMethodStatus`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment_method.v1.PaymentMethodService/UpdatePaymentMethodStatus`

**Request:**
```json
{
  "payment_method_id": "pm-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "is_active": false
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `payment_method_id` | string | Yes | Payment method ID |
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `is_active` | boolean | Yes | true = activate, false = deactivate |

**Response:** Same format as `PaymentMethodResponse`.

---

### Verify ACH Account

Sends a pre-note to verify an ACH account.

**ConnectRPC:** `payment_method.v1.PaymentMethodService/VerifyACHAccount`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment_method.v1.PaymentMethodService/VerifyACHAccount`

**Request:**
```json
{
  "payment_method_id": "pm-ach-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `payment_method_id` | string | Yes | ACH payment method ID to verify |
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |

**Response:**
```json
{
  "payment_method_id": "pm-ach-uuid-here",
  "transaction_id": "tx-prenote-uuid",
  "status": "pending",
  "message": "Pre-note sent for verification"
}
```

---

### Update Payment Method

Updates payment method metadata only (billing info, nickname). Does NOT support changing account/routing numbers or card details.

**ConnectRPC:** `payment_method.v1.PaymentMethodService/UpdatePaymentMethod`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/payment_method.v1.PaymentMethodService/UpdatePaymentMethod`

**Request:**
```json
{
  "payment_method_id": "pm-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "billing_name": "John Smith",
  "billing_address": "456 Oak Ave",
  "billing_city": "Los Angeles",
  "billing_state": "CA",
  "billing_zip": "90001",
  "nickname": "Work checking account",
  "is_default": true,
  "idempotency_key": "update_pm_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `payment_method_id` | string | Yes | Payment method ID |
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `billing_name` | string | No | Updated billing name |
| `billing_address` | string | No | Updated billing address |
| `billing_city` | string | No | Updated billing city |
| `billing_state` | string | No | Updated billing state |
| `billing_zip` | string | No | Updated billing ZIP |
| `nickname` | string | No | Updated nickname |
| `is_default` | boolean | No | Update default status |
| `idempotency_key` | string | Yes | Unique key |

**Note:** To change account/routing numbers or card details, delete the old payment method and create a new one.

**Response:** Same format as `PaymentMethodResponse`.

---

## Subscription Service

Handles recurring billing subscriptions.

### Create Subscription

Creates a new recurring billing subscription.

**ConnectRPC:** `subscription.v1.SubscriptionService/CreateSubscription`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/subscription.v1.SubscriptionService/CreateSubscription`

**Request:**
```json
{
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "amount_cents": 2999,
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
| `amount_cents` | int64 | Yes | Amount in cents (e.g., 2999 = $29.99) |
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
  "amount_cents": 2999,
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

**ConnectRPC:** `subscription.v1.SubscriptionService/GetSubscription`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/subscription.v1.SubscriptionService/GetSubscription`

**Example:**
```http
GET /api/v1/subscriptions/sub-uuid-here
```

**Response:** Same format as `SubscriptionResponse`.

---

### List Customer Subscriptions

Lists all subscriptions for a customer.

**ConnectRPC:** `subscription.v1.SubscriptionService/ListCustomerSubscriptions`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/subscription.v1.SubscriptionService/ListCustomerSubscriptions`

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

**ConnectRPC:** `subscription.v1.SubscriptionService/CancelSubscription`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/subscription.v1.SubscriptionService/CancelSubscription`

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

**ConnectRPC:** `subscription.v1.SubscriptionService/PauseSubscription`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/subscription.v1.SubscriptionService/PauseSubscription`

**Request:**
```json
{
  "subscription_id": "sub-uuid-here"
}
```

---

### Resume Subscription

Resumes a paused subscription.

**ConnectRPC:** `subscription.v1.SubscriptionService/ResumeSubscription`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/subscription.v1.SubscriptionService/ResumeSubscription`

**Request:**
```json
{
  "subscription_id": "sub-uuid-here"
}
```

---

### Update Subscription

Updates subscription details like amount, interval, or payment method.

**ConnectRPC:** `subscription.v1.SubscriptionService/UpdateSubscription`
**HTTP Method:** `POST`
**URL:** `http://localhost:8080/subscription.v1.SubscriptionService/UpdateSubscription`

**Request:**
```json
{
  "subscription_id": "sub-uuid-here",
  "merchant_id": "merchant-123",
  "customer_id": "customer-456",
  "amount_cents": 3999,
  "interval_value": 1,
  "interval_unit": "INTERVAL_UNIT_MONTH",
  "payment_method_id": "pm-new-uuid-here",
  "idempotency_key": "update_sub_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `subscription_id` | string | Yes | Subscription identifier |
| `merchant_id` | string | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `amount_cents` | int64 | No | New amount in cents |
| `interval_value` | int32 | No | New billing interval |
| `interval_unit` | enum | No | New interval unit |
| `payment_method_id` | string | No | New payment method |
| `idempotency_key` | string | Yes | Unique key |

**Note:** At least one update field must be provided. Changes take effect on the next billing cycle.

**Response:** Same format as `SubscriptionResponse` with updated values.

---

## Merchant Service

**Internal/Admin use only** - Manages multi-tenant merchant credentials.

**Note:** This service is gRPC-only (no HTTP REST endpoints).

### Register Merchant

Adds a new merchant to the system.

**ConnectRPC:** `merchant.v1.MerchantService/RegisterMerchant`

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

### List Merchants

Lists all registered merchants.

**ConnectRPC:** `merchant.v1.MerchantService/ListMerchants`

**Request:**
```protobuf
{
  limit: 100
  offset: 0
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `limit` | int32 | No | Max results (default: 100) |
| `offset` | int32 | No | Pagination offset |

**Response:**
```protobuf
{
  merchants: [
    {
      merchant_id: "merchant-123"
      cust_nbr: "9001"
      merch_nbr: "900300"
      environment: ENVIRONMENT_SANDBOX
      is_active: true
      created_at: "2025-01-13T12:00:00Z"
    }
  ]
  total_count: 1
}
```

---

### Update Merchant

Updates merchant configuration or metadata.

**ConnectRPC:** `merchant.v1.MerchantService/UpdateMerchant`

**Request:**
```protobuf
{
  merchant_id: "merchant-123"
  dba_nbr: "3"
  terminal_nbr: "88"
  metadata: {"business_name": "Acme Corporation"}
  idempotency_key: "update_merchant_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `dba_nbr` | string | No | Updated DBA number |
| `terminal_nbr` | string | No | Updated terminal number |
| `metadata` | map | No | Updated metadata |
| `idempotency_key` | string | Yes | Unique key |

**Note:** MAC secret, customer number, and merchant number cannot be updated. Use RotateMAC to update the MAC secret.

---

### Deactivate Merchant

Deactivates a merchant, preventing all transactions.

**ConnectRPC:** `merchant.v1.MerchantService/DeactivateMerchant`

**Request:**
```protobuf
{
  merchant_id: "merchant-123"
  reason: "Account suspended"
  idempotency_key: "deactivate_merchant_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `reason` | string | No | Deactivation reason |
| `idempotency_key` | string | Yes | Unique key |

**Response:**
```protobuf
{
  merchant_id: "merchant-123"
  is_active: false
  deactivated_at: "2025-01-13T12:00:00Z"
}
```

---

### Rotate MAC Secret

Rotates the MAC secret for signature verification.

**ConnectRPC:** `merchant.v1.MerchantService/RotateMAC`

**Request:**
```protobuf
{
  merchant_id: "merchant-123"
  new_mac_secret: "new-secret-key-here"
  idempotency_key: "rotate_mac_20250113_001"
}
```

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `merchant_id` | string | Yes | Merchant identifier |
| `new_mac_secret` | string | Yes | New MAC secret for signatures |
| `idempotency_key` | string | Yes | Unique key |

**Response:**
```protobuf
{
  merchant_id: "merchant-123"
  mac_rotated_at: "2025-01-13T12:00:00Z"
  message: "MAC secret updated successfully"
}
```

**Important:** Update your application configuration immediately after rotating the MAC secret, as the old secret will no longer validate signatures.

---

## Browser Post APIs (REST)

**Port:** 8081 (HTTP REST endpoints, not ConnectRPC)

PCI-compliant payment form generation and callback handling.

**Note:** These are traditional REST endpoints, not ConnectRPC. They use standard HTTP methods (GET/POST).

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

## Cron/Health APIs (REST)

**Port:** 8081 (HTTP REST endpoints, not ConnectRPC)

Administrative REST endpoints for health checks and scheduled tasks. Cron endpoints require authentication via `X-Cron-Secret` header.

**Note:** These are traditional REST endpoints, not ConnectRPC.

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
  "amount_cents": 9999,
  "idempotency_key": "sale_20250113_001"
}
```

### 2. Use transaction_id for Refunds/Voids

Provide the specific transaction ID to refund or void:

```json
{
  "transaction_id": "tx-uuid-here",
  "amount_cents": 3000,
  "idempotency_key": "refund_20250113_001"
}
```

**Note:** The API uses `parent_transaction_id` internally to link refunds/voids to their original transactions.

### 3. Store group_id with Orders

Link payments to orders using group_id for easier transaction history lookup:

```sql
UPDATE orders
SET payment_group_id = 'grp-uuid-here'
WHERE order_id = 'ORDER-123';
```

This allows you to query all related transactions (auth, capture, refunds) using the group_id.

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