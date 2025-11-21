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

**Authentication:** All ConnectRPC requests require JWT token in `Authorization: Bearer <token>` header. See [TOKEN_GENERATION.md](TOKEN_GENERATION.md) for details.

**URL Format:** `/{package}.{service}/{Method}`
- Example: `POST /payment.v1.PaymentService/Authorize`

**Proto Files Reference:**
- Payment Service: `proto/payment/v1/payment.proto`
- Payment Method Service: `proto/payment_method/v1/payment_method.proto`
- Subscription Service: `proto/subscription/v1/subscription.proto`

### Example cURL Request

```bash
curl -X POST http://localhost:8080/payment.v1.PaymentService/Authorize \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGc..." \
  -d '{
    "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
    "customer_id": "customer-uuid-here",
    "amount_cents": 9999,
    "currency": "USD",
    "payment_method_id": "pm-uuid-here",
    "idempotency_key": "auth_20250121_001"
  }'
```

### ConnectRPC Client Libraries

For production integrations, use official Connect client libraries:
- **Go**: `go get connectrpc.com/connect`
- **TypeScript/JavaScript**: `npm install @connectrpc/connect`
- **Swift**: Connect Swift client
- **Kotlin**: Connect Kotlin client

See [ConnectRPC documentation](https://connectrpc.com/docs/) for client setup guides.

---

## Table of Contents

### ConnectRPC APIs (Port 8080)

**Payment Service** (`payment.v1.PaymentService`)
1. [Authorize Payment](#authorize-payment)
2. [Capture Payment](#capture-payment)
3. [Sale (Auth + Capture)](#sale-auth--capture)
4. [Void Payment](#void-payment)
5. [Refund Payment](#refund-payment)
6. [ACH Debit](#ach-debit)
7. [ACH Credit](#ach-credit)
8. [ACH Void](#ach-void)
9. [Get Transaction](#get-transaction)
10. [List Transactions](#list-transactions)

**Payment Method Service** (`payment_method.v1.PaymentMethodService`)
11. [Get Payment Method](#get-payment-method)
12. [List Payment Methods](#list-payment-methods)
13. [Update Payment Method Status](#update-payment-method-status)
14. [Delete Payment Method](#delete-payment-method)
15. [Set Default Payment Method](#set-default-payment-method)
16. [Verify ACH Account](#verify-ach-account)
17. [Store ACH Account](#store-ach-account)
18. [Update Payment Method](#update-payment-method)

**Subscription Service** (`subscription.v1.SubscriptionService`)
19. [Create Subscription](#create-subscription)
20. [Update Subscription](#update-subscription)
21. [Cancel Subscription](#cancel-subscription)
22. [Pause Subscription](#pause-subscription)
23. [Resume Subscription](#resume-subscription)
24. [Get Subscription](#get-subscription)
25. [List Customer Subscriptions](#list-customer-subscriptions)
26. [Process Due Billing](#process-due-billing)

### REST APIs (Port 8081)

**Browser Post APIs**
27. [Get Payment Form Configuration](#get-payment-form-configuration)
28. [Browser Post Callback](#browser-post-callback)

**Cron/Health APIs**
29. [Health Check](#health-check)
30. [ACH Verification Cron](#ach-verification-cron)
31. [Recurring Billing Cron](#recurring-billing-cron)

### General
32. [Error Handling](#error-handling)
33. [Best Practices](#best-practices)
34. [Real-World Scenarios](#real-world-scenarios)

---

# ConnectRPC Payment Service

Proto: `proto/payment/v1/payment.proto`
Package: `payment.v1`
Service: `PaymentService`

## Authorize Payment

Holds funds on a payment method without capturing them. Use for pre-authorization workflows where you need to verify funds before completing the transaction.

**Method:** `Authorize`
**URL:** `POST /payment.v1.PaymentService/Authorize`
**Proto:** `rpc Authorize(AuthorizeRequest) returns (PaymentResponse)`

### AuthorizeRequest

**Proto Definition:**
```protobuf
message AuthorizeRequest {
  string merchant_id = 1;
  string customer_id = 2;
  int64 amount_cents = 3;
  string currency = 4;
  oneof payment_method {
    string payment_method_id = 5;
    string payment_token = 6;
  }
  string idempotency_key = 7;
  map<string, string> metadata = 8;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `merchant_id` | `string` (UUID) | Yes | Multi-tenant merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | No | Customer identifier (nullable for guest transactions) | `"customer-uuid-here"` |
| `amount_cents` | `int64` | Yes | Amount in smallest currency unit (cents for USD). Must be positive integer. | `9999` ($99.99) |
| `currency` | `string` | Yes | ISO 4217 currency code. Currently supports: USD | `"USD"` |
| `payment_method_id` | `string` (UUID) | Yes* | UUID of saved payment method from storage BRIC. Use for returning customers. | `"pm-uuid-here"` |
| `payment_token` | `string` | Yes* | EPX BRIC token (AUTH_GUID) for one-time use. From Browser Post tokenization. | `"epx-bric-token"` |
| `idempotency_key` | `string` | Yes | Unique key to prevent duplicate transactions. Recommended format: `{type}_{date}_{counter}` | `"auth_20250121_001"` |
| `metadata` | `map<string, string>` | No | Key-value pairs for additional data (max 50 keys, 500 chars per value) | `{"order_id": "ORDER-12345"}` |

*Either `payment_method_id` OR `payment_token` required, not both.

**Request Example:**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "amount_cents": 9999,
  "currency": "USD",
  "payment_method_id": "pm-uuid-here",
  "idempotency_key": "auth_20250121_001",
  "metadata": {
    "order_id": "ORDER-12345",
    "session_id": "sess_abc123"
  }
}
```

### PaymentResponse

**Proto Definition:**
```protobuf
message PaymentResponse {
  string transaction_id = 1;
  string parent_transaction_id = 2;
  int64 amount_cents = 3;
  string currency = 4;
  TransactionStatus status = 5;
  TransactionType type = 6;
  bool is_approved = 7;
  string authorization_code = 8;
  string message = 9;
  CardInfo card = 10;
  google.protobuf.Timestamp created_at = 11;
}

message CardInfo {
  string brand = 1;
  string last_four = 2;
}
```

**Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `transaction_id` | `string` (UUID) | Unique transaction identifier. Store this for future operations (capture, void, refund). | `"tx-uuid-here"` |
| `parent_transaction_id` | `string` (UUID) | Links to parent transaction. Empty for initial auth, populated for capture/refund/void. | `""` (empty for auth) |
| `amount_cents` | `int64` | Amount processed in smallest currency unit | `9999` |
| `currency` | `string` | ISO 4217 currency code | `"USD"` |
| `status` | `TransactionStatus` enum | Transaction outcome from gateway. `APPROVED` (EPX auth_resp='00') or `DECLINED` (EPX auth_resp != '00') | `"TRANSACTION_STATUS_APPROVED"` |
| `type` | `TransactionType` enum | Transaction type for this operation | `"TRANSACTION_TYPE_AUTH"` |
| `is_approved` | `bool` | Quick approval check. `true` if status is APPROVED. | `true` |
| `authorization_code` | `string` | Bank authorization code from issuer (6 digits typically). Empty if declined. | `"123456"` |
| `message` | `string` | Human-readable response message from gateway | `"Approved"` or `"Insufficient funds"` |
| `card` | `CardInfo` | Card information for display (never includes full card number) | See CardInfo below |
| `created_at` | `Timestamp` | Transaction creation timestamp in UTC | `"2025-01-21T12:00:00Z"` |

**CardInfo Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `brand` | `string` | Card brand/network | `"visa"`, `"mastercard"`, `"amex"`, `"discover"` |
| `last_four` | `string` | Last 4 digits of card number (PCI-safe) | `"1111"` |

**TransactionStatus Enum:**
```protobuf
enum TransactionStatus {
  TRANSACTION_STATUS_UNSPECIFIED = 0;
  TRANSACTION_STATUS_APPROVED = 1;   // EPX auth_resp='00'
  TRANSACTION_STATUS_DECLINED = 2;   // EPX auth_resp != '00'
}
```

**TransactionType Enum:**
```protobuf
enum TransactionType {
  TRANSACTION_TYPE_UNSPECIFIED = 0;
  TRANSACTION_TYPE_AUTH = 1;      // Authorization only
  TRANSACTION_TYPE_CAPTURE = 2;   // Capture authorized funds
  TRANSACTION_TYPE_CHARGE = 3;    // Combined auth + capture (sale)
  TRANSACTION_TYPE_REFUND = 4;    // Return funds
  TRANSACTION_TYPE_VOID = 5;      // Cancel before settlement
  TRANSACTION_TYPE_PRE_NOTE = 6;  // ACH verification
}
```

**Response Example (Success):**
```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
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
  "created_at": "2025-01-21T12:00:00Z"
}
```

**Response Example (Declined):**
```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440001",
  "parent_transaction_id": "",
  "amount_cents": 9999,
  "currency": "USD",
  "status": "TRANSACTION_STATUS_DECLINED",
  "type": "TRANSACTION_TYPE_AUTH",
  "is_approved": false,
  "authorization_code": "",
  "message": "Insufficient funds",
  "card": {
    "brand": "visa",
    "last_four": "1111"
  },
  "created_at": "2025-01-21T12:00:00Z"
}
```

---

## Capture Payment

Completes a previously authorized payment by capturing the held funds. Supports full or partial capture.

**Method:** `Capture`
**URL:** `POST /payment.v1.PaymentService/Capture`
**Proto:** `rpc Capture(CaptureRequest) returns (PaymentResponse)`

### CaptureRequest

**Proto Definition:**
```protobuf
message CaptureRequest {
  string transaction_id = 1;
  int64 amount_cents = 2;
  string idempotency_key = 3;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `transaction_id` | `string` (UUID) | Yes | Original authorization transaction ID to capture | `"550e8400-e29b-41d4-a716-446655440000"` |
| `amount_cents` | `int64` | No | Optional: Partial capture amount in cents. Omit for full capture. Must be ≤ authorized amount. | `7500` ($75.00 of $99.99 auth) |
| `idempotency_key` | `string` | Yes | Unique key to prevent duplicate captures | `"capture_20250121_001"` |

**Request Example (Full Capture):**
```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "idempotency_key": "capture_20250121_001"
}
```

**Request Example (Partial Capture):**
```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "amount_cents": 7500,
  "idempotency_key": "capture_20250121_001"
}
```

### Response

Returns `PaymentResponse` with:
- `type`: `"TRANSACTION_TYPE_CAPTURE"`
- `parent_transaction_id`: Original auth transaction ID
- `amount_cents`: Amount actually captured

---

## Sale (Auth + Capture)

Combines authorize and capture in one operation. Use for immediate payment completion.

**Method:** `Sale`
**URL:** `POST /payment.v1.PaymentService/Sale`
**Proto:** `rpc Sale(SaleRequest) returns (PaymentResponse)`

### SaleRequest

**Proto Definition:**
```protobuf
message SaleRequest {
  string merchant_id = 1;
  string customer_id = 2;
  int64 amount_cents = 3;
  string currency = 4;
  oneof payment_method {
    string payment_method_id = 5;
    string payment_token = 6;
  }
  string idempotency_key = 8;
  map<string, string> metadata = 9;
}
```

**Fields:** Same as `AuthorizeRequest` (see above)

**Request Example:**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "amount_cents": 4999,
  "currency": "USD",
  "payment_method_id": "pm-uuid-here",
  "idempotency_key": "sale_20250121_001",
  "metadata": {
    "order_id": "ORDER-12346"
  }
}
```

### Response

Returns `PaymentResponse` with `type`: `"TRANSACTION_TYPE_CHARGE"`

---

## Void Payment

Cancels an authorized or captured payment before settlement. Use within the same business day to avoid refund fees.

**Method:** `Void`
**URL:** `POST /payment.v1.PaymentService/Void`
**Proto:** `rpc Void(VoidRequest) returns (PaymentResponse)`

### VoidRequest

**Proto Definition:**
```protobuf
message VoidRequest {
  string transaction_id = 1;
  string idempotency_key = 2;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `transaction_id` | `string` (UUID) | Yes | Transaction to void (becomes parent_transaction_id of VOID record) | `"550e8400-e29b-41d4-a716-446655440000"` |
| `idempotency_key` | `string` | Yes | Unique key to prevent duplicate voids | `"void_20250121_001"` |

**Request Example:**
```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "idempotency_key": "void_20250121_001"
}
```

### Response

Returns `PaymentResponse` with:
- `type`: `"TRANSACTION_TYPE_VOID"`
- `parent_transaction_id`: Original transaction ID that was voided

---

## Refund Payment

Returns funds to the customer for a captured payment. Supports full or partial refunds.

**Method:** `Refund`
**URL:** `POST /payment.v1.PaymentService/Refund`
**Proto:** `rpc Refund(RefundRequest) returns (PaymentResponse)`

### RefundRequest

**Proto Definition:**
```protobuf
message RefundRequest {
  string transaction_id = 1;
  int64 amount_cents = 2;
  string reason = 3;
  string idempotency_key = 4;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `transaction_id` | `string` (UUID) | Yes | Transaction to refund (becomes parent_transaction_id of REFUND record) | `"550e8400-e29b-41d4-a716-446655440000"` |
| `amount_cents` | `int64` | No | Optional: Partial refund amount in cents. Omit for full refund. | `2500` ($25.00 of $99.99 sale) |
| `reason` | `string` | No | Refund reason (max 255 chars). For internal tracking. | `"Customer requested refund"` |
| `idempotency_key` | `string` | Yes | Unique key to prevent duplicate refunds | `"refund_20250121_001"` |

**Request Example (Full Refund):**
```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "reason": "Customer returned item",
  "idempotency_key": "refund_20250121_001"
}
```

**Request Example (Partial Refund):**
```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "amount_cents": 2500,
  "reason": "Partial refund - damaged item",
  "idempotency_key": "refund_20250121_001"
}
```

### Response

Returns `PaymentResponse` with:
- `type`: `"TRANSACTION_TYPE_REFUND"`
- `parent_transaction_id`: Original transaction ID that was refunded
- `amount_cents`: Amount actually refunded

---

## ACH Debit

Pulls money from a bank account using ACH. Requires a saved ACH payment method (Storage BRIC).

**Method:** `ACHDebit`
**URL:** `POST /payment.v1.PaymentService/ACHDebit`
**Proto:** `rpc ACHDebit(ACHDebitRequest) returns (PaymentResponse)`

### ACHDebitRequest

**Proto Definition:**
```protobuf
message ACHDebitRequest {
  string merchant_id = 1;
  string customer_id = 2;
  string payment_method_id = 3;
  int64 amount_cents = 4;
  string currency = 5;
  string idempotency_key = 6;
  map<string, string> metadata = 7;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `merchant_id` | `string` (UUID) | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | No | Customer identifier (optional for guest/one-time transactions) | `"customer-uuid-here"` |
| `payment_method_id` | `string` (UUID) | Yes | ACH Storage BRIC (from StoreACHAccount). Must be verified before first debit. | `"pm-ach-uuid-here"` |
| `amount_cents` | `int64` | Yes | Amount in cents (e.g., 2999 = $29.99) | `2999` |
| `currency` | `string` | Yes | ISO 4217 code | `"USD"` |
| `idempotency_key` | `string` | Yes | Unique key to prevent duplicate debits | `"ach_debit_20250121_001"` |
| `metadata` | `map<string, string>` | No | Additional data | `{"subscription_id": "sub_123"}` |

**Request Example:**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "payment_method_id": "pm-ach-uuid-here",
  "amount_cents": 2999,
  "currency": "USD",
  "idempotency_key": "ach_debit_20250121_001",
  "metadata": {
    "subscription_id": "sub_123",
    "billing_period": "2025-01"
  }
}
```

### Response

Returns `PaymentResponse` with `type`: `"TRANSACTION_TYPE_CHARGE"`

**Note:** ACH transactions settle in 2-3 business days. The response indicates acceptance by the gateway, not final settlement.

---

## ACH Credit

Sends money to a bank account using ACH. Used for payouts and refunds.

**Method:** `ACHCredit`
**URL:** `POST /payment.v1.PaymentService/ACHCredit`
**Proto:** `rpc ACHCredit(ACHCreditRequest) returns (PaymentResponse)`

### ACHCreditRequest

**Proto Definition:**
```protobuf
message ACHCreditRequest {
  string merchant_id = 1;
  string customer_id = 2;
  string payment_method_id = 3;
  int64 amount_cents = 4;
  string currency = 5;
  string reason = 6;
  string idempotency_key = 7;
  map<string, string> metadata = 8;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `merchant_id` | `string` (UUID) | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | No | Customer identifier (optional) | `"customer-uuid-here"` |
| `payment_method_id` | `string` (UUID) | Yes | ACH Storage BRIC | `"pm-ach-uuid-here"` |
| `amount_cents` | `int64` | Yes | Amount in cents | `5000` ($50.00) |
| `currency` | `string` | Yes | ISO 4217 code | `"USD"` |
| `reason` | `string` | No | Reason for credit (e.g., "refund", "payout") | `"Monthly payout"` |
| `idempotency_key` | `string` | Yes | Unique key | `"ach_credit_20250121_001"` |
| `metadata` | `map<string, string>` | No | Additional data | `{"payout_id": "pay_123"}` |

**Request Example:**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "payment_method_id": "pm-ach-uuid-here",
  "amount_cents": 5000,
  "currency": "USD",
  "reason": "Monthly affiliate payout",
  "idempotency_key": "ach_credit_20250121_001",
  "metadata": {
    "payout_id": "pay_123",
    "period": "2025-01"
  }
}
```

### Response

Returns `PaymentResponse` with `type`: `"TRANSACTION_TYPE_REFUND"` (for credits)

---

## ACH Void

Cancels an ACH transaction before settlement.

**Method:** `ACHVoid`
**URL:** `POST /payment.v1.PaymentService/ACHVoid`
**Proto:** `rpc ACHVoid(ACHVoidRequest) returns (PaymentResponse)`

### ACHVoidRequest

**Proto Definition:**
```protobuf
message ACHVoidRequest {
  string transaction_id = 1;
  string idempotency_key = 2;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `transaction_id` | `string` (UUID) | Yes | Original ACH transaction to void | `"550e8400-e29b-41d4-a716-446655440000"` |
| `idempotency_key` | `string` | Yes | Unique key | `"ach_void_20250121_001"` |

**Request Example:**
```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "idempotency_key": "ach_void_20250121_001"
}
```

### Response

Returns `PaymentResponse` with `type`: `"TRANSACTION_TYPE_VOID"`

---

## Get Transaction

Retrieves transaction details by ID.

**Method:** `GetTransaction`
**URL:** `POST /payment.v1.PaymentService/GetTransaction`
**Proto:** `rpc GetTransaction(GetTransactionRequest) returns (Transaction)`

### GetTransactionRequest

**Proto Definition:**
```protobuf
message GetTransactionRequest {
  string transaction_id = 1;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `transaction_id` | `string` (UUID) | Yes | Transaction ID to retrieve | `"550e8400-e29b-41d4-a716-446655440000"` |

**Request Example:**
```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Transaction

**Proto Definition:**
```protobuf
message Transaction {
  string id = 1;
  string parent_transaction_id = 2;
  string merchant_id = 3;
  string customer_id = 4;
  int64 amount_cents = 5;
  string currency = 6;
  TransactionStatus status = 7;
  TransactionType type = 8;
  PaymentMethodType payment_method_type = 9;
  string payment_method_id = 10;
  string authorization_code = 11;
  string message = 12;
  CardInfo card = 13;
  string idempotency_key = 14;
  google.protobuf.Timestamp created_at = 15;
  google.protobuf.Timestamp updated_at = 16;
}
```

**Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `id` | `string` (UUID) | Transaction identifier | `"550e8400-e29b-41d4-a716-446655440000"` |
| `parent_transaction_id` | `string` (UUID) | Links related transactions (auth → capture → refund) | `"parent-tx-uuid"` or `""` |
| `merchant_id` | `string` (UUID) | Merchant who processed the transaction | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Customer (nullable for guest transactions) | `"customer-uuid-here"` or `""` |
| `amount_cents` | `int64` | Amount in cents | `9999` |
| `currency` | `string` | Currency code | `"USD"` |
| `status` | `TransactionStatus` enum | Approval status | `"TRANSACTION_STATUS_APPROVED"` |
| `type` | `TransactionType` enum | Transaction type | `"TRANSACTION_TYPE_AUTH"` |
| `payment_method_type` | `PaymentMethodType` enum | Payment method used | `"PAYMENT_METHOD_TYPE_CREDIT_CARD"` or `"PAYMENT_METHOD_TYPE_ACH"` |
| `payment_method_id` | `string` (UUID) | Saved payment method used (if any) | `"pm-uuid-here"` or `""` |
| `authorization_code` | `string` | Bank auth code | `"123456"` |
| `message` | `string` | Response message | `"Approved"` |
| `card` | `CardInfo` | Card display info | See CardInfo above |
| `idempotency_key` | `string` | Idempotency key from request | `"auth_20250121_001"` |
| `created_at` | `Timestamp` | Creation time (UTC) | `"2025-01-21T12:00:00Z"` |
| `updated_at` | `Timestamp` | Last update time (UTC) | `"2025-01-21T12:00:05Z"` |

**Response Example:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "parent_transaction_id": "",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "amount_cents": 9999,
  "currency": "USD",
  "status": "TRANSACTION_STATUS_APPROVED",
  "type": "TRANSACTION_TYPE_AUTH",
  "payment_method_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "payment_method_id": "pm-uuid-here",
  "authorization_code": "123456",
  "message": "Approved",
  "card": {
    "brand": "visa",
    "last_four": "1111"
  },
  "idempotency_key": "auth_20250121_001",
  "created_at": "2025-01-21T12:00:00Z",
  "updated_at": "2025-01-21T12:00:00Z"
}
```

---

## List Transactions

Lists transactions for a merchant with optional filtering.

**Method:** `ListTransactions`
**URL:** `POST /payment.v1.PaymentService/ListTransactions`
**Proto:** `rpc ListTransactions(ListTransactionsRequest) returns (ListTransactionsResponse)`

### ListTransactionsRequest

**Proto Definition:**
```protobuf
message ListTransactionsRequest {
  string merchant_id = 1;
  string customer_id = 2;
  string parent_transaction_id = 3;
  TransactionStatus status = 4;
  int32 limit = 5;
  int32 offset = 6;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `merchant_id` | `string` (UUID) | Yes | Merchant to query | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | No | Filter by customer | `"customer-uuid-here"` |
| `parent_transaction_id` | `string` (UUID) | No | Filter by parent (get transaction chain: auth → capture → refund) | `"parent-tx-uuid"` |
| `status` | `TransactionStatus` enum | No | Filter by status (APPROVED/DECLINED) | `"TRANSACTION_STATUS_APPROVED"` |
| `limit` | `int32` | No | Max results to return (default: 100, max: 1000) | `50` |
| `offset` | `int32` | No | Pagination offset (default: 0) | `0` |

**Request Example (All transactions for merchant):**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "limit": 100,
  "offset": 0
}
```

**Request Example (Customer's approved transactions):**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "status": "TRANSACTION_STATUS_APPROVED",
  "limit": 50
}
```

**Request Example (Transaction chain):**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "parent_transaction_id": "original-auth-uuid"
}
```

### ListTransactionsResponse

**Proto Definition:**
```protobuf
message ListTransactionsResponse {
  repeated Transaction transactions = 1;
  int32 total_count = 2;
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `transactions` | `Transaction[]` | Array of transaction records (see Transaction above) |
| `total_count` | `int32` | Total number of matching transactions (for pagination) |

**Response Example:**
```json
{
  "transactions": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "parent_transaction_id": "",
      "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
      "customer_id": "customer-uuid-here",
      "amount_cents": 9999,
      "currency": "USD",
      "status": "TRANSACTION_STATUS_APPROVED",
      "type": "TRANSACTION_TYPE_AUTH",
      "payment_method_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
      "payment_method_id": "pm-uuid-here",
      "authorization_code": "123456",
      "message": "Approved",
      "card": {
        "brand": "visa",
        "last_four": "1111"
      },
      "idempotency_key": "auth_20250121_001",
      "created_at": "2025-01-21T12:00:00Z",
      "updated_at": "2025-01-21T12:00:00Z"
    }
  ],
  "total_count": 1
}
```

---

# ConnectRPC Payment Method Service

Proto: `proto/payment_method/v1/payment_method.proto`
Package: `payment_method.v1`
Service: `PaymentMethodService`

## Get Payment Method

Retrieves a specific payment method by ID.

**Method:** `GetPaymentMethod`
**URL:** `POST /payment_method.v1.PaymentMethodService/GetPaymentMethod`
**Proto:** `rpc GetPaymentMethod(GetPaymentMethodRequest) returns (PaymentMethod)`

### GetPaymentMethodRequest

**Proto Definition:**
```protobuf
message GetPaymentMethodRequest {
  string payment_method_id = 1;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `payment_method_id` | `string` (UUID) | Yes | Payment method UUID | `"pm-uuid-here"` |

**Request Example:**
```json
{
  "payment_method_id": "pm-uuid-here"
}
```

### PaymentMethod

**Proto Definition:**
```protobuf
message PaymentMethod {
  string id = 1;
  string merchant_id = 2;
  string customer_id = 3;
  PaymentMethodType payment_type = 4;
  string last_four = 5;
  optional string card_brand = 6;
  optional int32 card_exp_month = 7;
  optional int32 card_exp_year = 8;
  optional string bank_name = 9;
  optional string account_type = 10;
  bool is_default = 11;
  bool is_active = 12;
  bool is_verified = 13;
  google.protobuf.Timestamp created_at = 14;
  google.protobuf.Timestamp updated_at = 15;
  google.protobuf.Timestamp last_used_at = 16;
}
```

**Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `id` | `string` (UUID) | Payment method identifier | `"pm-uuid-here"` |
| `merchant_id` | `string` (UUID) | Merchant who owns this payment method | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Customer who owns this payment method | `"customer-uuid-here"` |
| `payment_type` | `PaymentMethodType` enum | Type: CREDIT_CARD or ACH | `"PAYMENT_METHOD_TYPE_CREDIT_CARD"` |
| `last_four` | `string` | Last 4 digits (card number or account number) | `"1111"` |
| `card_brand` | `string` (optional) | Card brand (credit cards only) | `"visa"` |
| `card_exp_month` | `int32` (optional) | Card expiration month 1-12 (credit cards only) | `12` |
| `card_exp_year` | `int32` (optional) | Card expiration year (credit cards only) | `2028` |
| `bank_name` | `string` (optional) | Bank name (ACH only) | `"Chase"` |
| `account_type` | `string` (optional) | Account type (ACH only): "CHECKING" or "SAVINGS" | `"CHECKING"` |
| `is_default` | `bool` | Whether this is the customer's default payment method | `true` |
| `is_active` | `bool` | Whether this payment method is active for use | `true` |
| `is_verified` | `bool` | Whether ACH account is verified (always `true` for credit cards) | `true` |
| `created_at` | `Timestamp` | Creation time | `"2025-01-21T12:00:00Z"` |
| `updated_at` | `Timestamp` | Last update time | `"2025-01-21T12:00:00Z"` |
| `last_used_at` | `Timestamp` | Last transaction time (nullable) | `"2025-01-21T15:30:00Z"` |

**Response Example (Credit Card):**
```json
{
  "id": "pm-uuid-here",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "payment_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "last_four": "1111",
  "card_brand": "visa",
  "card_exp_month": 12,
  "card_exp_year": 2028,
  "is_default": true,
  "is_active": true,
  "is_verified": true,
  "created_at": "2025-01-21T12:00:00Z",
  "updated_at": "2025-01-21T12:00:00Z",
  "last_used_at": "2025-01-21T15:30:00Z"
}
```

**Response Example (ACH):**
```json
{
  "id": "pm-ach-uuid-here",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "payment_type": "PAYMENT_METHOD_TYPE_ACH",
  "last_four": "9876",
  "bank_name": "Chase",
  "account_type": "CHECKING",
  "is_default": false,
  "is_active": true,
  "is_verified": true,
  "created_at": "2025-01-15T10:00:00Z",
  "updated_at": "2025-01-16T08:00:00Z",
  "last_used_at": "2025-01-20T14:00:00Z"
}
```

---

## List Payment Methods

Lists all payment methods for a customer with optional filtering.

**Method:** `ListPaymentMethods`
**URL:** `POST /payment_method.v1.PaymentMethodService/ListPaymentMethods`
**Proto:** `rpc ListPaymentMethods(ListPaymentMethodsRequest) returns (ListPaymentMethodsResponse)`

### ListPaymentMethodsRequest

**Proto Definition:**
```protobuf
message ListPaymentMethodsRequest {
  string merchant_id = 1;
  string customer_id = 2;
  optional PaymentMethodType payment_type = 3;
  optional bool is_active = 4;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `merchant_id` | `string` (UUID) | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Yes | Customer identifier | `"customer-uuid-here"` |
| `payment_type` | `PaymentMethodType` enum | No | Filter by type (CREDIT_CARD or ACH) | `"PAYMENT_METHOD_TYPE_CREDIT_CARD"` |
| `is_active` | `bool` | No | Filter by active status | `true` |

**Request Example (All payment methods):**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here"
}
```

**Request Example (Active credit cards only):**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "payment_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "is_active": true
}
```

### ListPaymentMethodsResponse

**Proto Definition:**
```protobuf
message ListPaymentMethodsResponse {
  repeated PaymentMethod payment_methods = 1;
}
```

**Response Example:**
```json
{
  "payment_methods": [
    {
      "id": "pm-uuid-1",
      "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
      "customer_id": "customer-uuid-here",
      "payment_type": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
      "last_four": "1111",
      "card_brand": "visa",
      "card_exp_month": 12,
      "card_exp_year": 2028,
      "is_default": true,
      "is_active": true,
      "is_verified": true,
      "created_at": "2025-01-21T12:00:00Z",
      "updated_at": "2025-01-21T12:00:00Z",
      "last_used_at": "2025-01-21T15:30:00Z"
    },
    {
      "id": "pm-uuid-2",
      "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
      "customer_id": "customer-uuid-here",
      "payment_type": "PAYMENT_METHOD_TYPE_ACH",
      "last_four": "9876",
      "bank_name": "Chase",
      "account_type": "CHECKING",
      "is_default": false,
      "is_active": true,
      "is_verified": true,
      "created_at": "2025-01-15T10:00:00Z",
      "updated_at": "2025-01-16T08:00:00Z",
      "last_used_at": "2025-01-20T14:00:00Z"
    }
  ]
}
```

---

## Update Payment Method Status

Updates the active status of a payment method (activate or deactivate).

**Method:** `UpdatePaymentMethodStatus`
**URL:** `POST /payment_method.v1.PaymentMethodService/UpdatePaymentMethodStatus`
**Proto:** `rpc UpdatePaymentMethodStatus(UpdatePaymentMethodStatusRequest) returns (PaymentMethodResponse)`

### UpdatePaymentMethodStatusRequest

**Proto Definition:**
```protobuf
message UpdatePaymentMethodStatusRequest {
  string payment_method_id = 1;
  string merchant_id = 2;
  string customer_id = 3;
  bool is_active = 4;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `payment_method_id` | `string` (UUID) | Yes | Payment method to update | `"pm-uuid-here"` |
| `merchant_id` | `string` (UUID) | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Yes | Customer identifier | `"customer-uuid-here"` |
| `is_active` | `bool` | Yes | `true` = activate, `false` = deactivate | `false` |

**Request Example (Deactivate):**
```json
{
  "payment_method_id": "pm-uuid-here",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "is_active": false
}
```

### Response

Returns `PaymentMethodResponse` (see structure in Store ACH Account below)

---

## Delete Payment Method

Soft deletes a payment method (sets deleted_at timestamp).

**Method:** `DeletePaymentMethod`
**URL:** `POST /payment_method.v1.PaymentMethodService/DeletePaymentMethod`
**Proto:** `rpc DeletePaymentMethod(DeletePaymentMethodRequest) returns (DeletePaymentMethodResponse)`

### DeletePaymentMethodRequest

**Proto Definition:**
```protobuf
message DeletePaymentMethodRequest {
  string payment_method_id = 1;
  string idempotency_key = 2;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `payment_method_id` | `string` (UUID) | Yes | Payment method to delete | `"pm-uuid-here"` |
| `idempotency_key` | `string` | Yes | Unique key to prevent duplicate deletes | `"delete_pm_20250121_001"` |

**Request Example:**
```json
{
  "payment_method_id": "pm-uuid-here",
  "idempotency_key": "delete_pm_20250121_001"
}
```

### DeletePaymentMethodResponse

**Proto Definition:**
```protobuf
message DeletePaymentMethodResponse {
  bool success = 1;
  string message = 2;
}
```

**Response Example:**
```json
{
  "success": true,
  "message": "Payment method deleted successfully"
}
```

---

## Set Default Payment Method

Marks a payment method as the customer's default.

**Method:** `SetDefaultPaymentMethod`
**URL:** `POST /payment_method.v1.PaymentMethodService/SetDefaultPaymentMethod`
**Proto:** `rpc SetDefaultPaymentMethod(SetDefaultPaymentMethodRequest) returns (PaymentMethodResponse)`

### SetDefaultPaymentMethodRequest

**Proto Definition:**
```protobuf
message SetDefaultPaymentMethodRequest {
  string payment_method_id = 1;
  string merchant_id = 2;
  string customer_id = 3;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `payment_method_id` | `string` (UUID) | Yes | Payment method to set as default | `"pm-uuid-here"` |
| `merchant_id` | `string` (UUID) | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Yes | Customer identifier | `"customer-uuid-here"` |

**Request Example:**
```json
{
  "payment_method_id": "pm-uuid-here",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here"
}
```

### Response

Returns `PaymentMethodResponse` with `is_default`: `true`

---

## Verify ACH Account

Sends pre-note for ACH verification. Required before first ACH debit on a new account.

**Method:** `VerifyACHAccount`
**URL:** `POST /payment_method.v1.PaymentMethodService/VerifyACHAccount`
**Proto:** `rpc VerifyACHAccount(VerifyACHAccountRequest) returns (VerifyACHAccountResponse)`

### VerifyACHAccountRequest

**Proto Definition:**
```protobuf
message VerifyACHAccountRequest {
  string payment_method_id = 1;
  string merchant_id = 2;
  string customer_id = 3;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `payment_method_id` | `string` (UUID) | Yes | ACH payment method to verify | `"pm-ach-uuid-here"` |
| `merchant_id` | `string` (UUID) | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Yes | Customer identifier | `"customer-uuid-here"` |

**Request Example:**
```json
{
  "payment_method_id": "pm-ach-uuid-here",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here"
}
```

### VerifyACHAccountResponse

**Proto Definition:**
```protobuf
message VerifyACHAccountResponse {
  string payment_method_id = 1;
  string transaction_id = 2;
  string status = 3;
  string message = 4;
}
```

**Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `payment_method_id` | `string` (UUID) | Payment method being verified | `"pm-ach-uuid-here"` |
| `transaction_id` | `string` (UUID) | Pre-note transaction ID | `"tx-prenote-uuid"` |
| `status` | `string` | Verification status: "pending", "verified", "failed" | `"pending"` |
| `message` | `string` | Status message | `"Pre-note sent, verification pending"` |

**Response Example:**
```json
{
  "payment_method_id": "pm-ach-uuid-here",
  "transaction_id": "tx-prenote-uuid",
  "status": "pending",
  "message": "Pre-note sent, verification pending (2-3 business days)"
}
```

**Note:** Pre-note verification takes 2-3 business days. Use the ACH Verification Cron to check for completed verifications.

---

## Store ACH Account

Creates ACH Storage BRIC and sends pre-note for verification. Use when customer adds a bank account for recurring payments.

**Method:** `StoreACHAccount`
**URL:** `POST /payment_method.v1.PaymentMethodService/StoreACHAccount`
**Proto:** `rpc StoreACHAccount(StoreACHAccountRequest) returns (PaymentMethodResponse)`

### StoreACHAccountRequest

**Proto Definition:**
```protobuf
message StoreACHAccountRequest {
  string merchant_id = 1;
  string customer_id = 2;
  string account_number = 3;
  string routing_number = 4;
  string account_holder_name = 5;
  AccountType account_type = 6;
  StdEntryClass std_entry_class = 7;
  optional string first_name = 8;
  optional string last_name = 9;
  optional string address = 10;
  optional string city = 11;
  optional string state = 12;
  optional string zip_code = 13;
  optional string bank_name = 14;
  optional string nickname = 15;
  bool is_default = 16;
  string idempotency_key = 17;
}

enum AccountType {
  ACCOUNT_TYPE_UNSPECIFIED = 0;
  ACCOUNT_TYPE_CHECKING = 1;
  ACCOUNT_TYPE_SAVINGS = 2;
}

enum StdEntryClass {
  STD_ENTRY_CLASS_UNSPECIFIED = 0;
  STD_ENTRY_CLASS_PPD = 1;  // Prearranged Payment and Deposit (personal)
  STD_ENTRY_CLASS_CCD = 2;  // Corporate Credit or Debit
  STD_ENTRY_CLASS_WEB = 3;  // Internet-initiated entry
  STD_ENTRY_CLASS_TEL = 4;  // Telephone-initiated entry
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `merchant_id` | `string` (UUID) | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Yes | Customer identifier | `"customer-uuid-here"` |
| `account_number` | `string` | Yes | Bank account number (4-17 digits). **NEVER stored** - only sent to EPX for BRIC creation. | `"123456789"` |
| `routing_number` | `string` | Yes | Bank routing number (9 digits) | `"021000021"` |
| `account_holder_name` | `string` | Yes | Name on account (max 100 chars) | `"John Doe"` |
| `account_type` | `AccountType` enum | Yes | CHECKING or SAVINGS | `"ACCOUNT_TYPE_CHECKING"` |
| `std_entry_class` | `StdEntryClass` enum | Yes | SEC code for ACH transactions. Use PPD for personal, CCD for business, WEB for online. | `"STD_ENTRY_CLASS_WEB"` |
| `first_name` | `string` (optional) | No | Billing first name | `"John"` |
| `last_name` | `string` (optional) | No | Billing last name | `"Doe"` |
| `address` | `string` (optional) | No | Billing address | `"123 Main St"` |
| `city` | `string` (optional) | No | Billing city | `"New York"` |
| `state` | `string` (optional) | No | Billing state (2-letter code) | `"NY"` |
| `zip_code` | `string` (optional) | No | Billing ZIP code | `"10001"` |
| `bank_name` | `string` (optional) | No | Bank name for display | `"Chase"` |
| `nickname` | `string` (optional) | No | Customer-friendly label | `"Primary checking"` |
| `is_default` | `bool` | Yes | Set as default payment method | `true` |
| `idempotency_key` | `string` | Yes | Unique key | `"store_ach_20250121_001"` |

**Request Example:**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
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
  "idempotency_key": "store_ach_20250121_001"
}
```

### PaymentMethodResponse

**Proto Definition:**
```protobuf
message PaymentMethodResponse {
  string payment_method_id = 1;
  string merchant_id = 2;
  string customer_id = 3;
  PaymentMethodType payment_type = 4;
  string last_four = 5;
  optional string card_brand = 6;
  optional int32 card_exp_month = 7;
  optional int32 card_exp_year = 8;
  optional string bank_name = 9;
  optional string account_type = 10;
  bool is_default = 11;
  bool is_active = 12;
  bool is_verified = 13;
  google.protobuf.Timestamp created_at = 14;
  google.protobuf.Timestamp last_used_at = 15;
}
```

**Response Example:**
```json
{
  "payment_method_id": "pm-ach-new-uuid",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "payment_type": "PAYMENT_METHOD_TYPE_ACH",
  "last_four": "6789",
  "bank_name": "Chase",
  "account_type": "CHECKING",
  "is_default": true,
  "is_active": true,
  "is_verified": false,
  "created_at": "2025-01-21T16:00:00Z",
  "last_used_at": null
}
```

**Note:** `is_verified` will be `false` until pre-note verification completes (2-3 business days).

---

## Update Payment Method

Updates metadata only (billing info, nickname). Does NOT support changing account/routing numbers or card numbers.

**Method:** `UpdatePaymentMethod`
**URL:** `POST /payment_method.v1.PaymentMethodService/UpdatePaymentMethod`
**Proto:** `rpc UpdatePaymentMethod(UpdatePaymentMethodRequest) returns (PaymentMethodResponse)`

### UpdatePaymentMethodRequest

**Proto Definition:**
```protobuf
message UpdatePaymentMethodRequest {
  string payment_method_id = 1;
  string merchant_id = 2;
  string customer_id = 3;
  optional string billing_name = 4;
  optional string billing_address = 5;
  optional string billing_city = 6;
  optional string billing_state = 7;
  optional string billing_zip = 8;
  optional string nickname = 9;
  optional bool is_default = 10;
  string idempotency_key = 11;
}
```

**Fields:** All fields except IDs and idempotency_key are optional. Only provided fields will be updated.

**Request Example:**
```json
{
  "payment_method_id": "pm-uuid-here",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "billing_address": "456 New St",
  "billing_city": "San Francisco",
  "billing_state": "CA",
  "billing_zip": "94102",
  "nickname": "Work card",
  "idempotency_key": "update_pm_20250121_001"
}
```

### Response

Returns `PaymentMethodResponse` with updated fields.

---

# ConnectRPC Subscription Service

Proto: `proto/subscription/v1/subscription.proto`
Package: `subscription.v1`
Service: `SubscriptionService`

## Create Subscription

Creates a new recurring billing subscription.

**Method:** `CreateSubscription`
**URL:** `POST /subscription.v1.SubscriptionService/CreateSubscription`
**Proto:** `rpc CreateSubscription(CreateSubscriptionRequest) returns (SubscriptionResponse)`

### CreateSubscriptionRequest

**Proto Definition:**
```protobuf
message CreateSubscriptionRequest {
  string merchant_id = 1;
  string customer_id = 2;
  int64 amount_cents = 3;
  string currency = 4;
  int32 interval_value = 5;
  IntervalUnit interval_unit = 6;
  string payment_method_id = 7;
  google.protobuf.Timestamp start_date = 8;
  int32 max_retries = 9;
  map<string, string> metadata = 10;
  string idempotency_key = 11;
}

enum IntervalUnit {
  INTERVAL_UNIT_UNSPECIFIED = 0;
  INTERVAL_UNIT_DAY = 1;
  INTERVAL_UNIT_WEEK = 2;
  INTERVAL_UNIT_MONTH = 3;
  INTERVAL_UNIT_YEAR = 4;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `merchant_id` | `string` (UUID) | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Yes | Customer identifier | `"customer-uuid-here"` |
| `amount_cents` | `int64` | Yes | Recurring amount in cents | `2999` ($29.99/month) |
| `currency` | `string` | Yes | ISO 4217 code | `"USD"` |
| `interval_value` | `int32` | Yes | Billing interval (e.g., 1 month, 2 weeks, 3 months) | `1` |
| `interval_unit` | `IntervalUnit` enum | Yes | Time unit: DAY, WEEK, MONTH, YEAR | `"INTERVAL_UNIT_MONTH"` |
| `payment_method_id` | `string` (UUID) | Yes | Saved payment method for recurring charges. Must be verified if ACH. | `"pm-uuid-here"` |
| `start_date` | `Timestamp` | Yes | When billing should start (UTC) | `"2025-02-01T00:00:00Z"` |
| `max_retries` | `int32` | No | Max retry attempts for failed billing (default: 3) | `3` |
| `metadata` | `map<string, string>` | No | Additional data | `{"plan": "premium"}` |
| `idempotency_key` | `string` | Yes | Unique key | `"create_sub_20250121_001"` |

**Request Example (Monthly subscription):**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "amount_cents": 2999,
  "currency": "USD",
  "interval_value": 1,
  "interval_unit": "INTERVAL_UNIT_MONTH",
  "payment_method_id": "pm-uuid-here",
  "start_date": "2025-02-01T00:00:00Z",
  "max_retries": 3,
  "metadata": {
    "plan": "premium",
    "tier": "monthly"
  },
  "idempotency_key": "create_sub_20250121_001"
}
```

**Request Example (Weekly subscription):**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "amount_cents": 999,
  "currency": "USD",
  "interval_value": 1,
  "interval_unit": "INTERVAL_UNIT_WEEK",
  "payment_method_id": "pm-uuid-here",
  "start_date": "2025-01-28T00:00:00Z",
  "max_retries": 3,
  "idempotency_key": "create_sub_weekly_001"
}
```

### SubscriptionResponse

**Proto Definition:**
```protobuf
message SubscriptionResponse {
  string subscription_id = 1;
  string merchant_id = 2;
  string customer_id = 3;
  int64 amount_cents = 4;
  string currency = 5;
  int32 interval_value = 6;
  IntervalUnit interval_unit = 7;
  SubscriptionStatus status = 8;
  string payment_method_id = 9;
  google.protobuf.Timestamp next_billing_date = 10;
  string gateway_subscription_id = 11;
  google.protobuf.Timestamp created_at = 12;
  google.protobuf.Timestamp updated_at = 13;
  optional google.protobuf.Timestamp cancelled_at = 14;
}

enum SubscriptionStatus {
  SUBSCRIPTION_STATUS_UNSPECIFIED = 0;
  SUBSCRIPTION_STATUS_ACTIVE = 1;
  SUBSCRIPTION_STATUS_PAUSED = 2;
  SUBSCRIPTION_STATUS_CANCELLED = 3;
  SUBSCRIPTION_STATUS_PAST_DUE = 4;
}
```

**Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `subscription_id` | `string` (UUID) | Subscription identifier | `"sub-uuid-here"` |
| `merchant_id` | `string` (UUID) | Merchant | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Customer | `"customer-uuid-here"` |
| `amount_cents` | `int64` | Recurring amount | `2999` |
| `currency` | `string` | Currency | `"USD"` |
| `interval_value` | `int32` | Billing interval | `1` |
| `interval_unit` | `IntervalUnit` enum | Time unit | `"INTERVAL_UNIT_MONTH"` |
| `status` | `SubscriptionStatus` enum | ACTIVE, PAUSED, CANCELLED, or PAST_DUE | `"SUBSCRIPTION_STATUS_ACTIVE"` |
| `payment_method_id` | `string` (UUID) | Payment method used | `"pm-uuid-here"` |
| `next_billing_date` | `Timestamp` | Next billing date (UTC) | `"2025-02-01T00:00:00Z"` |
| `gateway_subscription_id` | `string` | EPX recurring billing ID | `"epx-rec-id"` |
| `created_at` | `Timestamp` | Creation time | `"2025-01-21T16:00:00Z"` |
| `updated_at` | `Timestamp` | Last update time | `"2025-01-21T16:00:00Z"` |
| `cancelled_at` | `Timestamp` (optional) | Cancellation time (null if active) | `null` |

**Response Example:**
```json
{
  "subscription_id": "sub-uuid-here",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "amount_cents": 2999,
  "currency": "USD",
  "interval_value": 1,
  "interval_unit": "INTERVAL_UNIT_MONTH",
  "status": "SUBSCRIPTION_STATUS_ACTIVE",
  "payment_method_id": "pm-uuid-here",
  "next_billing_date": "2025-02-01T00:00:00Z",
  "gateway_subscription_id": "epx-rec-12345",
  "created_at": "2025-01-21T16:00:00Z",
  "updated_at": "2025-01-21T16:00:00Z",
  "cancelled_at": null
}
```

---

## Update Subscription

Updates subscription properties (amount, interval, payment method).

**Method:** `UpdateSubscription`
**URL:** `POST /subscription.v1.SubscriptionService/UpdateSubscription`
**Proto:** `rpc UpdateSubscription(UpdateSubscriptionRequest) returns (SubscriptionResponse)`

### UpdateSubscriptionRequest

**Proto Definition:**
```protobuf
message UpdateSubscriptionRequest {
  string subscription_id = 1;
  optional int64 amount_cents = 2;
  optional int32 interval_value = 3;
  optional IntervalUnit interval_unit = 4;
  optional string payment_method_id = 5;
  string idempotency_key = 6;
}
```

**Fields:** All fields except subscription_id and idempotency_key are optional. Only provided fields will be updated.

**Request Example (Update amount):**
```json
{
  "subscription_id": "sub-uuid-here",
  "amount_cents": 3999,
  "idempotency_key": "update_sub_20250121_001"
}
```

**Request Example (Change payment method):**
```json
{
  "subscription_id": "sub-uuid-here",
  "payment_method_id": "pm-new-uuid",
  "idempotency_key": "update_sub_pm_001"
}
```

### Response

Returns `SubscriptionResponse` with updated fields.

---

## Cancel Subscription

Cancels an active subscription.

**Method:** `CancelSubscription`
**URL:** `POST /subscription.v1.SubscriptionService/CancelSubscription`
**Proto:** `rpc CancelSubscription(CancelSubscriptionRequest) returns (SubscriptionResponse)`

### CancelSubscriptionRequest

**Proto Definition:**
```protobuf
message CancelSubscriptionRequest {
  string subscription_id = 1;
  bool cancel_at_period_end = 2;
  string reason = 3;
  string idempotency_key = 4;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `subscription_id` | `string` (UUID) | Yes | Subscription to cancel | `"sub-uuid-here"` |
| `cancel_at_period_end` | `bool` | Yes | If `true`, cancel after current billing period. If `false`, cancel immediately. | `true` |
| `reason` | `string` | No | Cancellation reason | `"Customer requested"` |
| `idempotency_key` | `string` | Yes | Unique key | `"cancel_sub_20250121_001"` |

**Request Example (Cancel at period end):**
```json
{
  "subscription_id": "sub-uuid-here",
  "cancel_at_period_end": true,
  "reason": "Customer requested cancellation",
  "idempotency_key": "cancel_sub_20250121_001"
}
```

**Request Example (Cancel immediately):**
```json
{
  "subscription_id": "sub-uuid-here",
  "cancel_at_period_end": false,
  "reason": "Fraud detected",
  "idempotency_key": "cancel_sub_fraud_001"
}
```

### Response

Returns `SubscriptionResponse` with:
- `status`: `"SUBSCRIPTION_STATUS_CANCELLED"`
- `cancelled_at`: Timestamp of cancellation

---

## Pause Subscription

Pauses an active subscription. No billing occurs while paused.

**Method:** `PauseSubscription`
**URL:** `POST /subscription.v1.SubscriptionService/PauseSubscription`
**Proto:** `rpc PauseSubscription(PauseSubscriptionRequest) returns (SubscriptionResponse)`

### PauseSubscriptionRequest

**Proto Definition:**
```protobuf
message PauseSubscriptionRequest {
  string subscription_id = 1;
}
```

**Request Example:**
```json
{
  "subscription_id": "sub-uuid-here"
}
```

### Response

Returns `SubscriptionResponse` with `status`: `"SUBSCRIPTION_STATUS_PAUSED"`

---

## Resume Subscription

Resumes a paused subscription.

**Method:** `ResumeSubscription`
**URL:** `POST /subscription.v1.SubscriptionService/ResumeSubscription`
**Proto:** `rpc ResumeSubscription(ResumeSubscriptionRequest) returns (SubscriptionResponse)`

### ResumeSubscriptionRequest

**Proto Definition:**
```protobuf
message ResumeSubscriptionRequest {
  string subscription_id = 1;
}
```

**Request Example:**
```json
{
  "subscription_id": "sub-uuid-here"
}
```

### Response

Returns `SubscriptionResponse` with `status`: `"SUBSCRIPTION_STATUS_ACTIVE"`

---

## Get Subscription

Retrieves subscription details by ID.

**Method:** `GetSubscription`
**URL:** `POST /subscription.v1.SubscriptionService/GetSubscription`
**Proto:** `rpc GetSubscription(GetSubscriptionRequest) returns (Subscription)`

### GetSubscriptionRequest

**Proto Definition:**
```protobuf
message GetSubscriptionRequest {
  string subscription_id = 1;
}
```

**Request Example:**
```json
{
  "subscription_id": "sub-uuid-here"
}
```

### Subscription

**Proto Definition:**
```protobuf
message Subscription {
  string id = 1;
  string merchant_id = 2;
  string customer_id = 3;
  int64 amount_cents = 4;
  string currency = 5;
  int32 interval_value = 6;
  IntervalUnit interval_unit = 7;
  SubscriptionStatus status = 8;
  string payment_method_id = 9;
  google.protobuf.Timestamp next_billing_date = 10;
  string gateway_subscription_id = 11;
  int32 failure_retry_count = 12;
  int32 max_retries = 13;
  google.protobuf.Timestamp created_at = 14;
  google.protobuf.Timestamp updated_at = 15;
  optional google.protobuf.Timestamp cancelled_at = 16;
  map<string, string> metadata = 17;
}
```

**Additional Fields (vs SubscriptionResponse):**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `failure_retry_count` | `int32` | Current retry count (resets on successful billing) | `0` |
| `max_retries` | `int32` | Max retry attempts before marking PAST_DUE | `3` |
| `metadata` | `map<string, string>` | Custom data | `{"plan": "premium"}` |

**Response Example:**
```json
{
  "id": "sub-uuid-here",
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "amount_cents": 2999,
  "currency": "USD",
  "interval_value": 1,
  "interval_unit": "INTERVAL_UNIT_MONTH",
  "status": "SUBSCRIPTION_STATUS_ACTIVE",
  "payment_method_id": "pm-uuid-here",
  "next_billing_date": "2025-02-01T00:00:00Z",
  "gateway_subscription_id": "epx-rec-12345",
  "failure_retry_count": 0,
  "max_retries": 3,
  "created_at": "2025-01-21T16:00:00Z",
  "updated_at": "2025-01-21T16:00:00Z",
  "cancelled_at": null,
  "metadata": {
    "plan": "premium",
    "tier": "monthly"
  }
}
```

---

## List Customer Subscriptions

Lists all subscriptions for a customer with optional filtering.

**Method:** `ListCustomerSubscriptions`
**URL:** `POST /subscription.v1.SubscriptionService/ListCustomerSubscriptions`
**Proto:** `rpc ListCustomerSubscriptions(ListCustomerSubscriptionsRequest) returns (ListCustomerSubscriptionsResponse)`

### ListCustomerSubscriptionsRequest

**Proto Definition:**
```protobuf
message ListCustomerSubscriptionsRequest {
  string merchant_id = 1;
  string customer_id = 2;
  optional SubscriptionStatus status = 3;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `merchant_id` | `string` (UUID) | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `customer_id` | `string` (UUID) | Yes | Customer identifier | `"customer-uuid-here"` |
| `status` | `SubscriptionStatus` enum | No | Filter by status | `"SUBSCRIPTION_STATUS_ACTIVE"` |

**Request Example (All subscriptions):**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here"
}
```

**Request Example (Active only):**
```json
{
  "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
  "customer_id": "customer-uuid-here",
  "status": "SUBSCRIPTION_STATUS_ACTIVE"
}
```

### ListCustomerSubscriptionsResponse

**Proto Definition:**
```protobuf
message ListCustomerSubscriptionsResponse {
  repeated Subscription subscriptions = 1;
}
```

**Response Example:**
```json
{
  "subscriptions": [
    {
      "id": "sub-uuid-1",
      "merchant_id": "1a20fff8-2cec-48e5-af49-87e501652913",
      "customer_id": "customer-uuid-here",
      "amount_cents": 2999,
      "currency": "USD",
      "interval_value": 1,
      "interval_unit": "INTERVAL_UNIT_MONTH",
      "status": "SUBSCRIPTION_STATUS_ACTIVE",
      "payment_method_id": "pm-uuid-here",
      "next_billing_date": "2025-02-01T00:00:00Z",
      "gateway_subscription_id": "epx-rec-12345",
      "failure_retry_count": 0,
      "max_retries": 3,
      "created_at": "2025-01-21T16:00:00Z",
      "updated_at": "2025-01-21T16:00:00Z",
      "cancelled_at": null,
      "metadata": {
        "plan": "premium"
      }
    }
  ]
}
```

---

## Process Due Billing

Processes subscriptions due for billing. Used by recurring billing cron job (internal/admin use).

**Method:** `ProcessDueBilling`
**URL:** `POST /subscription.v1.SubscriptionService/ProcessDueBilling`
**Proto:** `rpc ProcessDueBilling(ProcessDueBillingRequest) returns (ProcessDueBillingResponse)`

### ProcessDueBillingRequest

**Proto Definition:**
```protobuf
message ProcessDueBillingRequest {
  google.protobuf.Timestamp as_of_date = 1;
  int32 batch_size = 2;
}
```

**Fields:**

| Field | Type | Required | Description | Example |
|-------|------|----------|-------------|---------|
| `as_of_date` | `Timestamp` | Yes | Process subscriptions due on or before this date | `"2025-01-21T23:59:59Z"` |
| `batch_size` | `int32` | No | Max subscriptions to process (default: 100, max: 1000) | `100` |

**Request Example:**
```json
{
  "as_of_date": "2025-01-21T23:59:59Z",
  "batch_size": 100
}
```

### ProcessDueBillingResponse

**Proto Definition:**
```protobuf
message ProcessDueBillingResponse {
  int32 processed_count = 1;
  int32 success_count = 2;
  int32 failed_count = 3;
  int32 skipped_count = 4;
  repeated BillingError errors = 5;
}

message BillingError {
  string subscription_id = 1;
  string customer_id = 2;
  string error = 3;
  bool retriable = 4;
}
```

**Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `processed_count` | `int32` | Total subscriptions processed | `100` |
| `success_count` | `int32` | Successfully billed | `95` |
| `failed_count` | `int32` | Failed billing attempts | `3` |
| `skipped_count` | `int32` | Skipped (paused, cancelled, etc.) | `2` |
| `errors` | `BillingError[]` | Details of failed billings | See below |

**BillingError Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `subscription_id` | `string` (UUID) | Failed subscription | `"sub-uuid-here"` |
| `customer_id` | `string` (UUID) | Customer | `"customer-uuid-here"` |
| `error` | `string` | Error message | `"Insufficient funds"` |
| `retriable` | `bool` | Whether retry is possible | `true` |

**Response Example:**
```json
{
  "processed_count": 100,
  "success_count": 95,
  "failed_count": 3,
  "skipped_count": 2,
  "errors": [
    {
      "subscription_id": "sub-failed-1",
      "customer_id": "customer-1",
      "error": "Insufficient funds",
      "retriable": true
    },
    {
      "subscription_id": "sub-failed-2",
      "customer_id": "customer-2",
      "error": "Card expired",
      "retriable": false
    },
    {
      "subscription_id": "sub-failed-3",
      "customer_id": "customer-3",
      "error": "Payment method deleted",
      "retriable": false
    }
  ]
}
```

---

# REST Browser Post APIs

Port: 8081
Protocol: Traditional REST (not ConnectRPC)

Browser Post is EPX's secure tokenization flow where the payment form submits directly to EPX, never exposing card data to your server. After processing, EPX redirects the user back with transaction results.

**Security Model:**
- **TAC (Terminal Authorization Code)**: Time-limited encrypted token (4-hour expiry) containing transaction details
- **Key Exchange**: Merchant requests TAC from EPX before showing payment form
- **Direct Submit**: Payment form POSTs to EPX with TAC + card data
- **Callback**: EPX redirects browser to your callback URL with results

## Get Payment Form Configuration

Generates form configuration for Browser Post payment. Returns TAC (from Key Exchange) and form parameters. **NO database write** - transaction only created on callback.

**HTTP Method:** `GET`
**URL:** `http://localhost:8081/api/v1/payments/browser-post/form`
**Authentication:** Requires JWT token

### Query Parameters

| Parameter | Type | Required | Description | Example |
|-----------|------|----------|-------------|---------|
| `transaction_id` | UUID | Yes | Frontend-generated transaction UUID. Used to prevent duplicate callbacks. | `"550e8400-e29b-41d4-a716-446655440000"` |
| `merchant_id` | UUID | Yes | Merchant identifier | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `amount` | string (decimal) | Yes | Transaction amount (e.g., "99.99" for $99.99) | `"99.99"` |
| `transaction_type` | string | Yes | "SALE" (auth+capture), "AUTH" (auth-only), or "STORAGE" (tokenize only) | `"SALE"` |
| `return_url` | URL | Yes | Where to redirect user after payment (your receipt page) | `"https://example.com/receipt"` |

**Request Example:**
```bash
GET /api/v1/payments/browser-post/form?transaction_id=550e8400-e29b-41d4-a716-446655440000&merchant_id=1a20fff8-2cec-48e5-af49-87e501652913&amount=99.99&transaction_type=SALE&return_url=https://example.com/receipt HTTP/1.1
Host: localhost:8081
Authorization: Bearer eyJhbGc...
```

### Response

**Status:** 200 OK
**Content-Type:** application/json

**Response Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `epxPostURL` | string (URL) | EPX Browser Post endpoint where form should POST | `"https://secure.epxuap.com/browserpost"` |
| `tac` | string | TAC token from Key Exchange (4-hour expiry). Include as hidden field. | `"encrypted-tac-value"` |
| `amount` | string | Transaction amount | `"99.99"` |
| `industryType` | string | Industry type: "E" (E-commerce), "R" (Retail), "M" (MOTO) | `"E"` |
| `tranType` | string | Transaction type: "S" (Sale), "A" (Auth), "K" (Storage) | `"S"` |
| `redirectURL` | string (URL) | Full callback URL (MUST match TAC). EPX redirects here after processing. | `"http://localhost:8081/api/v1/payments/browser-post/callback?transaction_id=550e8400-e29b-41d4-a716-446655440000&merchant_id=1a20fff8-2cec-48e5-af49-87e501652913"` |
| `returnUrl` | string (URL) | User's final destination (maps to EPX USER_DATA_1) | `"https://example.com/receipt"` |
| `merchantId` | UUID string | Merchant UUID (maps to EPX USER_DATA_3 for callback validation) | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `merchantName` | string | Merchant display name (for UI) | `"ACME Corp"` |

**Response Example:**
```json
{
  "epxPostURL": "https://secure.epxuap.com/browserpost",
  "tac": "encrypted-tac-value-here",
  "amount": "99.99",
  "industryType": "E",
  "tranType": "S",
  "redirectURL": "http://localhost:8081/api/v1/payments/browser-post/callback?transaction_id=550e8400-e29b-41d4-a716-446655440000&merchant_id=1a20fff8-2cec-48e5-af49-87e501652913",
  "returnUrl": "https://example.com/receipt",
  "merchantId": "1a20fff8-2cec-48e5-af49-87e501652913",
  "merchantName": "ACME Corp"
}
```

### Frontend Implementation

**Step 1:** Call this endpoint to get form config
**Step 2:** Create payment form that POSTs to `epxPostURL` with:
- Hidden field: `tac` = response.tac
- Hidden field: `amount` = response.amount
- Hidden fields: Any other pass-through data
- User inputs: Card number, CVV, expiration, billing info

**Example HTML Form:**
```html
<form action="https://secure.epxuap.com/browserpost" method="POST">
  <!-- From API response -->
  <input type="hidden" name="tac" value="encrypted-tac-value-here">
  <input type="hidden" name="amount" value="99.99">

  <!-- User inputs -->
  <input type="text" name="card_number" placeholder="Card Number" required>
  <input type="text" name="cvv" placeholder="CVV" required>
  <input type="text" name="exp_month" placeholder="MM" required>
  <input type="text" name="exp_year" placeholder="YYYY" required>

  <button type="submit">Pay $99.99</button>
</form>
```

**Step 3:** User submits form → Browser POSTs directly to EPX
**Step 4:** EPX processes payment → Redirects to `redirectURL` with results
**Step 5:** Your callback handler processes results → Redirects user to `returnUrl`

---

## Browser Post Callback

Processes the Browser Post redirect callback from EPX. This endpoint receives transaction results as a self-posting HTML form.

**HTTP Method:** `POST`
**URL:** `http://localhost:8081/api/v1/payments/browser-post/callback`
**Authentication:** Not required (EPX callback uses TAC validation)

**Security:**
- TAC validation (expired TACs rejected by EPX)
- Transaction ID validation (must match pending transaction)
- Merchant ID validation (callback must be for correct merchant)

**Note:** This endpoint does NOT use MAC signatures (those are for Server Post only). Browser Post security relies on TAC + transaction validation.

### Form Data (from EPX)

EPX redirects the browser with these fields as form POST data:

**EPX Response Fields:**

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `AUTH_RESP` | string (2 chars) | **Authorization response code from issuing bank.** `"00"` = Approved, all other codes = Declined. See [EPX Response Codes](https://developer.north.com/docs/response-codes) for full list. | `"00"` (approved) or `"51"` (insufficient funds) |
| `AUTH_RESP_TEXT` | string | Human-readable response message from gateway. Changes frequently - do NOT use in code validation. | `"Approved"` or `"Insufficient funds"` |
| `AUTH_CODE` | string (6 chars) | Bank authorization code (6 digits). Only present if approved. Empty if declined. | `"123456"` |
| `BRIC` | string (20 chars) | **EPX Auth GUID (Financial BRIC).** EPX token for this transaction. Lifetime: 13-24 months. Use for linked operations (capture, void, refund). Also called AUTH_GUID. | `"AABBCCDDEEFFGGHHIIyy"` |
| `GUID` | string (20 chars) | **EPX Storage BRIC.** Only present if `transaction_type=STORAGE`. Permanent token for saved payment method. Use for recurring charges. | `"ZZYYXXWWVVUUTTSSRRqq"` |
| `LAST_FOUR` | string (4 digits) | Last 4 digits of card number (PCI-safe) | `"1111"` |
| `CARD_TYPE` | string | Card brand/network | `"VI"` (Visa), `"MC"` (Mastercard), `"AX"` (Amex), `"DS"` (Discover) |
| `TRAN_GROUP` | string | Transaction type performed by EPX: "A" or "AUTH" (auth-only), "U" or "SALE" (sale), "K" (storage) | `"U"` or `"SALE"` |
| `AMOUNT` | string (decimal) | Transaction amount processed | `"99.99"` |
| `USER_DATA_1` | string (URL) | Return URL passed through from form | `"https://example.com/receipt"` |
| `USER_DATA_2` | string (UUID) | Optional: customer_id if provided in form | `"customer-uuid-here"` |
| `USER_DATA_3` | string (UUID) | merchant_id for validation | `"1a20fff8-2cec-48e5-af49-87e501652913"` |
| `BP_RESP_CODE` | string | **Browser Post validation code.** Only present if EPX validation failed (before reaching payment gateway). `"BP_140"` = field validation failure, see BP_FIELD_ERRORS. | `"BP_140"` |
| `BP_RESP_TEXT` | string | Browser Post validation message. Only present if BP_RESP_CODE exists. | `"Invalid card number format"` |
| `BP_FIELD_ERRORS` | string (JSON) | JSON array of field validation errors. Only present if BP_RESP_CODE = "BP_140". | `'["card_number": "Invalid format"]'` |

**EPX Field Mappings:**

| EPX Field | Database Column | Description |
|-----------|----------------|-------------|
| `BRIC` (AUTH_GUID) | `epx_auth_guid` | Financial BRIC for transaction operations |
| `GUID` | `epx_storage_guid` | Storage BRIC for saved payment methods |
| `AUTH_RESP` | `epx_auth_resp` | Response code ("00" = approved) |
| `AUTH_RESP_TEXT` | `epx_auth_resp_text` | Response message |
| `AUTH_CODE` | `epx_auth_code` | Bank authorization code |
| `LAST_FOUR` | Display only | Last 4 card digits (not stored as sensitive data) |
| `CARD_TYPE` | `card_brand` | Card network (VI→visa, MC→mastercard, etc.) |

### Response (to Browser)

The handler renders an HTML page that redirects the user to `USER_DATA_1` (return_url):

**Success Response:**
```html
<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="refresh" content="0;url=https://example.com/receipt?status=success&transaction_id=550e8400-e29b-41d4-a716-446655440000">
</head>
<body>
  <p>Payment approved! Redirecting...</p>
</body>
</html>
```

**Declined Response:**
```html
<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="refresh" content="0;url=https://example.com/receipt?status=declined&reason=insufficient_funds">
</head>
<body>
  <p>Payment declined. Redirecting...</p>
</body>
</html>
```

**Validation Error Response:**
```html
<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="refresh" content="0;url=https://example.com/payment-form?error=validation&fields=card_number">
</head>
<body>
  <p>Please check your payment information and try again.</p>
</body>
</html>
```

### Callback Processing Flow

1. **Receive POST** from EPX with form data
2. **Parse EPX response** using BrowserPostAdapter
3. **Validate merchant_id** from USER_DATA_3
4. **Check for Browser Post validation errors** (BP_RESP_CODE)
   - If present: Redirect user with error, NO database write
5. **Check AUTH_RESP**:
   - `"00"` = Approved: Create/update transaction as APPROVED
   - Other codes = Declined: Create/update transaction as DECLINED
6. **Store EPX tokens**:
   - BRIC (AUTH_GUID) → epx_auth_guid
   - GUID (if STORAGE) → Create payment_method record
7. **Redirect browser** to USER_DATA_1 with status

### Example Callback Request (from EPX)

```http
POST /api/v1/payments/browser-post/callback?transaction_id=550e8400-e29b-41d4-a716-446655440000&merchant_id=1a20fff8-2cec-48e5-af49-87e501652913 HTTP/1.1
Host: localhost:8081
Content-Type: application/x-www-form-urlencoded

AUTH_RESP=00&AUTH_RESP_TEXT=Approved&AUTH_CODE=123456&BRIC=AABBCCDDEEFFGGHHIIyy&LAST_FOUR=1111&CARD_TYPE=VI&TRAN_GROUP=U&AMOUNT=99.99&USER_DATA_1=https://example.com/receipt&USER_DATA_3=1a20fff8-2cec-48e5-af49-87e501652913
```

---

# REST Cron/Health APIs

Port: 8081
Protocol: Traditional REST

## Health Check

Returns service health status.

**HTTP Method:** `GET`
**URL:** `http://localhost:8081/cron/health`
**Authentication:** Not required

**Response (200 OK):**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-21T12:00:00Z"
}
```

---

## ACH Verification Cron

Checks pending ACH pre-note verifications and updates payment method status. Run every 6 hours.

**HTTP Method:** `POST`
**URL:** `http://localhost:8081/cron/ach-verification`
**Authentication:** Requires cron API key in `X-API-Key` header

**Response (200 OK):**
```json
{
  "processed": 15,
  "verified": 12,
  "failed": 2,
  "pending": 1
}
```

**Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `processed` | int32 | Total pre-notes checked |
| `verified` | int32 | Successfully verified (payment_method.is_verified = true) |
| `failed` | int32 | Failed verification (payment_method deleted) |
| `pending` | int32 | Still pending (no response yet) |

---

## Recurring Billing Cron

Processes subscriptions due for billing. Run daily at midnight UTC.

**HTTP Method:** `POST`
**URL:** `http://localhost:8081/cron/recurring-billing`
**Authentication:** Requires cron API key in `X-API-Key` header

**Query Parameters:**

| Parameter | Type | Required | Description | Default |
|-----------|------|----------|-------------|---------|
| `as_of_date` | ISO 8601 | No | Process subscriptions due on/before this date | Current UTC time |
| `batch_size` | int32 | No | Max subscriptions to process | 100 |

**Request Example:**
```bash
POST /cron/recurring-billing?as_of_date=2025-01-21T23:59:59Z&batch_size=100 HTTP/1.1
Host: localhost:8081
X-API-Key: cron-api-key-here
```

**Response (200 OK):**
```json
{
  "processed_count": 100,
  "success_count": 95,
  "failed_count": 3,
  "skipped_count": 2,
  "errors": [
    {
      "subscription_id": "sub-failed-1",
      "customer_id": "customer-1",
      "error": "Insufficient funds",
      "retriable": true
    }
  ]
}
```

---

# Error Handling

## ConnectRPC Errors

ConnectRPC uses standard error codes. All errors include:
- `code`: Error code (enum)
- `message`: Human-readable error message
- `details`: Additional error context (optional)

**Common Error Codes:**

| Code | HTTP Status | Description | Example Scenario |
|------|-------------|-------------|------------------|
| `unauthenticated` | 401 | Missing or invalid JWT token | No Authorization header |
| `permission_denied` | 403 | JWT valid but insufficient scopes | Token missing `payment:create` scope |
| `invalid_argument` | 400 | Invalid request parameters | Negative amount_cents |
| `not_found` | 404 | Resource doesn't exist | Transaction ID not found |
| `already_exists` | 409 | Duplicate idempotency key | Same idempotency_key used twice |
| `failed_precondition` | 400 | Request cannot be completed | Capture on declined auth |
| `resource_exhausted` | 429 | Rate limit exceeded | Too many requests |
| `internal` | 500 | Server error | Database connection failed |

**Error Response Example:**
```json
{
  "code": "invalid_argument",
  "message": "amount_cents must be positive integer",
  "details": [
    {
      "field": "amount_cents",
      "value": "-100",
      "reason": "must be positive"
    }
  ]
}
```

## REST API Errors

REST endpoints return standard HTTP status codes with JSON error body:

**Error Response Format:**
```json
{
  "error": "Error message",
  "details": "Additional context"
}
```

**Common HTTP Status Codes:**

| Status | Description | Example |
|--------|-------------|---------|
| 400 | Bad Request | Missing required parameter |
| 401 | Unauthorized | Invalid JWT token |
| 403 | Forbidden | Insufficient permissions |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Duplicate idempotency key |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Server error |
| 503 | Service Unavailable | Gateway timeout |

## EPX Gateway Errors

EPX errors are abstracted into our response format. Check `AUTH_RESP` field:

**AUTH_RESP Codes:**
- `"00"` = Approved
- `"05"` = Do not honor
- `"14"` = Invalid card number
- `"41"` = Lost card
- `"43"` = Stolen card
- `"51"` = Insufficient funds
- `"54"` = Expired card
- `"55"` = Incorrect PIN
- `"57"` = Transaction not permitted
- `"61"` = Exceeds withdrawal limit
- `"65"` = Exceeds withdrawal frequency
- `"91"` = Issuer unavailable

For complete list, see [EPX Response Codes Dictionary](https://developer.north.com/docs/response-codes).

---

# Best Practices

## Idempotency

**Always use unique idempotency keys** to prevent duplicate transactions:

```javascript
const idempotencyKey = `${operation}_${Date.now()}_${randomUUID()}`;
// Example: "sale_1737470400000_uuid-here"
```

**Key Requirements:**
- Max length: 255 characters
- Unique per operation
- Recommended format: `{operation}_{timestamp}_{uuid}`
- Same key returns cached response (within 24 hours)

## Error Handling

**Check multiple fields** for transaction status:

```javascript
// ✅ Correct
if (response.is_approved && response.status === "TRANSACTION_STATUS_APPROVED") {
  // Transaction succeeded
} else {
  // Transaction failed - check response.message for reason
}

// ❌ Incorrect
if (response.transaction_id) {  // Transaction ID exists even if declined!
  // This will incorrectly treat declines as success
}
```

## Token Security

**Never log or store raw tokens:**
```javascript
// ❌ BAD
console.log(`Payment token: ${payment_token}`);  // Logs sensitive data

// ✅ GOOD
console.log(`Payment with saved method: ${payment_method_id}`);  // Only UUID
```

## Rate Limiting

**Current limits (per merchant):**
- Payment operations: 100 req/min
- Read operations: 500 req/min
- Cron endpoints: 10 req/hour

**Implement exponential backoff:**
```javascript
async function retryWithBackoff(fn, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn();
    } catch (error) {
      if (error.code !== 'resource_exhausted') throw error;
      if (i === maxRetries - 1) throw error;
      await sleep(Math.pow(2, i) * 1000);  // 1s, 2s, 4s
    }
  }
}
```

## Amount Handling

**Always use smallest currency unit (cents):**
```javascript
// ✅ Correct
const amount_cents = Math.round(amountDollars * 100);
// $99.99 → 9999 cents

// ❌ Incorrect
const amount_cents = 99.99;  // Float! Will cause precision errors
```

## Parent Transaction IDs

**Store parent_transaction_id** for linked operations:

```javascript
// 1. Authorize
const authResponse = await authorize({...});
const authTxId = authResponse.transaction_id;  // Store this!

// 2. Capture (later)
const captureResponse = await capture({
  transaction_id: authTxId,  // Reference original auth
  idempotency_key: "capture_001"
});

// 3. Capture response includes parent link
console.log(captureResponse.parent_transaction_id === authTxId);  // true
```

---

# Real-World Scenarios

## Scenario 1: E-Commerce Checkout (Immediate Payment)

**Goal:** Customer checks out with credit card, charge immediately.

**Flow:**
1. Customer adds items to cart → Total: $99.99
2. Frontend tokenizes card via Browser Post (STORAGE flow)
3. Backend receives GUID (Storage BRIC) from callback
4. Backend creates payment_method record
5. Backend calls Sale with payment_method_id
6. Show receipt if approved

**Code Example:**
```javascript
// Step 3-4: Callback receives GUID from Browser Post
async function handleBrowserPostCallback(req, res) {
  const { GUID, AUTH_RESP, merchant_id, customer_id } = req.body;

  if (AUTH_RESP !== "00" || !GUID) {
    return res.redirect(`/checkout?error=tokenization_failed`);
  }

  // GUID is now stored as payment_method - use it for sale
  const saleResponse = await paymentClient.Sale({
    merchant_id,
    customer_id,
    amount_cents: 9999,
    currency: "USD",
    payment_method_id: GUID,  // Use tokenized card
    idempotency_key: `sale_${Date.now()}_${randomUUID()}`
  });

  if (saleResponse.is_approved) {
    res.redirect(`/receipt?tx=${saleResponse.transaction_id}`);
  } else {
    res.redirect(`/checkout?error=${saleResponse.message}`);
  }
}
```

## Scenario 2: Two-Step Payment (Auth → Capture)

**Goal:** Hotel pre-authorizes card at check-in, captures at checkout.

**Flow:**
1. Check-in: Authorize $500 to hold funds
2. Store transaction_id
3. Check-out: Calculate final amount ($450)
4. Capture $450 (partial capture)
5. Remaining $50 released automatically

**Code Example:**
```javascript
// Day 1: Check-in
const authResponse = await paymentClient.Authorize({
  merchant_id,
  customer_id,
  amount_cents: 50000,  // $500 hold
  currency: "USD",
  payment_method_id: savedCardId,
  idempotency_key: `auth_checkin_${bookingId}`
});

// Store auth transaction ID with booking
await db.bookings.update(bookingId, {
  auth_transaction_id: authResponse.transaction_id
});

// Day 3: Check-out
const booking = await db.bookings.findById(bookingId);
const finalAmount = calculateBill(booking);  // $450

const captureResponse = await paymentClient.Capture({
  transaction_id: booking.auth_transaction_id,
  amount_cents: 45000,  // $450 (partial capture)
  idempotency_key: `capture_checkout_${bookingId}`
});

// $50 released back to customer automatically
```

## Scenario 3: Subscription with ACH

**Goal:** Customer subscribes to monthly service, pays via bank account.

**Flow:**
1. Customer enters bank account details
2. Create ACH Storage BRIC (StoreACHAccount)
3. Pre-note sent automatically (2-3 day verification)
4. Create subscription (starts after verification)
5. Cron job processes monthly billing

**Code Example:**
```javascript
// Step 1-2: Store ACH account
const achResponse = await paymentMethodClient.StoreACHAccount({
  merchant_id,
  customer_id,
  account_number: "123456789",  // Never stored - only sent to EPX
  routing_number: "021000021",
  account_holder_name: "John Doe",
  account_type: "ACCOUNT_TYPE_CHECKING",
  std_entry_class: "STD_ENTRY_CLASS_WEB",
  bank_name: "Chase",
  is_default: true,
  idempotency_key: `store_ach_${Date.now()}`
});

// Step 2-3: Pre-note sent, verification pending
const achId = achResponse.payment_method_id;
console.log(achResponse.is_verified);  // false (pending)

// Step 4: Create subscription (after verification - 2-3 days)
const subResponse = await subscriptionClient.CreateSubscription({
  merchant_id,
  customer_id,
  amount_cents: 2999,  // $29.99/month
  currency: "USD",
  interval_value: 1,
  interval_unit: "INTERVAL_UNIT_MONTH",
  payment_method_id: achId,  // ACH account
  start_date: new Date(Date.now() + 3 * 24 * 60 * 60 * 1000),  // 3 days
  max_retries: 3,
  idempotency_key: `create_sub_${Date.now()}`
});

// Step 5: Cron processes billing monthly
// POST /cron/recurring-billing runs daily at midnight UTC
```

## Scenario 4: Refund Processing

**Goal:** Customer returns item, issue partial refund.

**Flow:**
1. Customer requests return for 1 of 3 items
2. Calculate refund amount ($25 of $99 total)
3. Issue partial refund
4. Update order status

**Code Example:**
```javascript
// Original sale: $99 for 3 items
const originalTxId = "tx-original-uuid";

// Customer returns 1 item: refund $25
const refundResponse = await paymentClient.Refund({
  transaction_id: originalTxId,
  amount_cents: 2500,  // $25 partial refund
  reason: "Customer returned 1 item (blue widget)",
  idempotency_key: `refund_${Date.now()}_${orderItemId}`
});

if (refundResponse.is_approved) {
  await db.orders.update(orderId, {
    refund_amount: 2500,
    refund_transaction_id: refundResponse.transaction_id,
    status: "partially_refunded"
  });

  // Notify customer
  await sendEmail(customer.email, {
    subject: "Refund Processed",
    body: `$25.00 refunded to card ending in ${refundResponse.card.last_four}`
  });
}
```

## Scenario 5: Handling Declined Payments

**Goal:** Customer's card declined, offer alternative payment method.

**Flow:**
1. Attempt payment with saved card
2. Receive decline (insufficient funds)
3. Show customer their other saved methods
4. Retry with different card
5. If all fail, show ACH option

**Code Example:**
```javascript
async function processPayment(customer, amount) {
  // Get customer's payment methods (sorted by is_default)
  const methods = await paymentMethodClient.ListPaymentMethods({
    merchant_id,
    customer_id: customer.id,
    is_active: true
  });

  // Try each payment method
  for (const method of methods.payment_methods) {
    const response = await paymentClient.Sale({
      merchant_id,
      customer_id: customer.id,
      amount_cents: amount,
      currency: "USD",
      payment_method_id: method.id,
      idempotency_key: `sale_attempt_${method.id}_${Date.now()}`
    });

    if (response.is_approved) {
      return { success: true, response };
    }

    // Log decline reason
    console.log(`Payment method ${method.last_four} declined: ${response.message}`);
  }

  // All methods failed - prompt for ACH
  return {
    success: false,
    message: "All saved payment methods declined. Please add a bank account."
  };
}
```

## Scenario 6: Recurring Billing with Retry Logic

**Goal:** Monthly subscription billing with automatic retries on failure.

**Flow:**
1. Cron runs daily, finds subscriptions due
2. Attempt billing
3. If declined: increment retry_count, try again tomorrow
4. If max_retries exceeded: mark PAST_DUE, notify customer
5. Customer updates payment method → retry immediately

**Code Example:**
```javascript
// Cron handler (runs daily)
async function processRecurringBilling() {
  const response = await subscriptionClient.ProcessDueBilling({
    as_of_date: new Date().toISOString(),
    batch_size: 100
  });

  // Handle failures
  for (const error of response.errors) {
    if (error.retriable) {
      // Will retry tomorrow (handled automatically)
      console.log(`Subscription ${error.subscription_id} failed, will retry`);
    } else {
      // Non-retriable error (card expired, deleted, etc.)
      const subscription = await subscriptionClient.GetSubscription({
        subscription_id: error.subscription_id
      });

      // Notify customer to update payment method
      await sendEmail(subscription.customer_id, {
        subject: "Payment Method Needs Update",
        body: `Your subscription billing failed: ${error.error}. Please update your payment method.`
      });
    }
  }

  return response;
}

// Customer updates payment method
async function updateSubscriptionPayment(subscriptionId, newPaymentMethodId) {
  // Update subscription to use new payment method
  const response = await subscriptionClient.UpdateSubscription({
    subscription_id: subscriptionId,
    payment_method_id: newPaymentMethodId,
    idempotency_key: `update_sub_pm_${Date.now()}`
  });

  // If subscription is PAST_DUE, immediately retry billing
  if (response.status === "SUBSCRIPTION_STATUS_PAST_DUE") {
    await retryFailedSubscription(subscriptionId);
  }

  return response;
}
```

---

**Document Version:** 2.0
**Last Updated:** 2025-01-21
**Generated with:** Claude Code
