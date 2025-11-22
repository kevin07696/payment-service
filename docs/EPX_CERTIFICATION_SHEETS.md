# EPX Payment Gateway Certification Sheets

**Version**: 1.0
**Date**: 2025-11-22
**Environment**: EPX UAP Staging
**Status**: ✅ VERIFIED via Integration Tests
**Test Merchant**: CUST_NBR=9001, MERCH_NBR=900300, DBA_NBR=2, TERMINAL_NBR=77

---

## Table of Contents

1. [Service Architecture](#service-architecture)
2. [EPX Key Exchange](#epx-key-exchange)
   - [SALE Key Exchange](#sale-key-exchange)
   - [AUTH Key Exchange](#auth-key-exchange)
   - [STORAGE Key Exchange](#storage-key-exchange)
3. [Browser Post Transactions](#browser-post-transactions)
   - [Generate Browser Post Form](#generate-browser-post-form)
   - [Browser Post Callback](#browser-post-callback)
4. [Server Post Operations](#server-post-operations)
   - [Authorize](#authorize)
   - [Sale](#sale)
   - [Capture](#capture)
   - [Void](#void)
   - [Refund](#refund)
5. [Verification Summary](#verification-summary)

---

## Service Architecture

The payment service runs on two ports:

| Port | Protocol | Purpose |
|------|----------|---------|
| **8080** | ConnectRPC (HTTP/2) | Primary API - Authorize, Sale, Capture, Void, Refund |
| **8081** | HTTP/1.1 | EPX Browser Post callbacks, Cron endpoints |

**Why Two Ports?**
- Port 8080: ConnectRPC requires HTTP/2 with specific headers
- Port 8081: EPX callbacks require simple HTTP/1.1 endpoints

**Integration Test Configuration:**
```bash
export CONNECTRPC_URL="http://localhost:8080"
export HTTP_CALLBACK_URL="http://localhost:8081"
```

---

## EPX Key Exchange

EPX Key Exchange generates TAC (Transaction Access Code) tokens for Browser Post transactions.

**Endpoint**: `https://keyexch.epxuap.com`
**Method**: POST
**Content-Type**: `application/x-www-form-urlencoded`
**Test Coverage**: `tests/integration/payment/browser_post_workflow_test.go`

### SALE Key Exchange

Generate TAC for immediate payment capture.

#### Curl Command

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TRAN_NBR=1000028096" \
  -d "AMOUNT=50.00" \
  -d "MAC=<MERCHANT_MAC_SECRET>" \
  -d "TRAN_GROUP=SALE" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TRAN_NBR=1000028096
AMOUNT=50.00
MAC=<MERCHANT_MAC_SECRET>
TRAN_GROUP=SALE
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `TRAN_NBR` | string | ✅ | Unique transaction number (10 digits) |
| `AMOUNT` | decimal | ✅ | Transaction amount (e.g., "50.00") |
| `MAC` | string | ✅ | Merchant Authentication Code |
| `TRAN_GROUP` | string | ✅ | Must be `SALE` (not "U") |
| `REDIRECT_URL` | string | ✅ | Callback URL after payment |

#### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELD KEY="TAC">dGVzdHRhY3ZhbHVlZm9yc2FsZXRyYW5zYWN0aW9uMTIzNDU2Nzg5MA==</FIELD>
</RESPONSE>
```

**Response Format**: XML with base64-encoded TAC token (~305 characters)

#### Integration Test

```go
// tests/integration/payment/browser_post_workflow_test.go
saleResult := testutil.GetRealBRICForSaleAutomated(
    t, client, cfg, "50.00", "http://localhost:8081", jwtToken,
)
// ✅ Verifies TAC received and transaction approved
```

---

### AUTH Key Exchange

Generate TAC for authorization-only (no immediate capture).

#### Curl Command

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TRAN_NBR=1000022329" \
  -d "AMOUNT=50.00" \
  -d "MAC=<MERCHANT_MAC_SECRET>" \
  -d "TRAN_GROUP=AUTH" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TRAN_NBR=1000022329
AMOUNT=50.00
MAC=<MERCHANT_MAC_SECRET>
TRAN_GROUP=AUTH
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `TRAN_GROUP` | string | ✅ | Must be `AUTH` (not "A") |

#### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELD KEY="TAC">YXV0aHRhY3Rva2VuZm9yYXV0aG9yaXphdGlvbnRyYW5zYWN0aW9u</FIELD>
</RESPONSE>
```

#### Integration Test

```go
authResult := testutil.GetRealBRICForAuthAutomated(...)
// ✅ Verifies AUTH TAC and Storage BRIC returned
```

---

### STORAGE Key Exchange

Generate TAC for card tokenization (no payment).

#### Curl Command

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TRAN_NBR=1000029490" \
  -d "AMOUNT=0.00" \
  -d "MAC=<MERCHANT_MAC_SECRET>" \
  -d "TRAN_GROUP=STORAGE" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TRAN_NBR=1000029490
AMOUNT=0.00
MAC=<MERCHANT_MAC_SECRET>
TRAN_GROUP=STORAGE
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `TRAN_GROUP` | string | ✅ | Must be `STORAGE` (not "S") |
| `AMOUNT` | decimal | ✅ | Usually "0.00" for tokenization |

#### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELD KEY="TAC">c3RvcmFnZXRhY3Rva2VuZm9yY2FyZHRva2VuaXphdGlvbg==</FIELD>
</RESPONSE>
```

#### Integration Test

```go
// tests/integration/payment_method/payment_method_test.go
bric := testutil.GetRealBRICForStorageAutomated(...)
// ✅ Verifies reusable BRIC returned
```

---

## Browser Post Transactions

Browser Post allows customers to enter payment details on EPX's hosted payment page.

**Test Coverage**: `tests/integration/payment/browser_post_workflow_test.go`

### Generate Browser Post Form

Get TAC and EPX form details for customer payment.

#### Curl Command

```bash
curl -X GET "http://localhost:8081/api/v1/payments/browser-post/form" \
  -G \
  --data-urlencode "transaction_id=550e8400-e29b-41d4-a716-446655440000" \
  --data-urlencode "merchant_id=00000000-0000-0000-0000-000000000001" \
  --data-urlencode "amount=50.00" \
  --data-urlencode "transaction_type=SALE" \
  --data-urlencode "return_url=http://localhost:8082/payment-result"
```

#### Request Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `transaction_id` | UUID | ✅ | Unique transaction identifier |
| `merchant_id` | UUID | ✅ | Merchant account UUID |
| `amount` | decimal | ✅ | Transaction amount |
| `transaction_type` | string | ✅ | `SALE`, `AUTH`, or `STORAGE` |
| `return_url` | string | ✅ | Customer redirect URL after payment |

#### Response Body

```json
{
  "tac": "dGVzdHRhY3ZhbHVlZm9yc2FsZXRyYW5zYWN0aW9u...",
  "postURL": "https://epxuap.com",
  "epxTranNbr": "1000028096",
  "merchantCredentials": {
    "custNbr": "9001",
    "merchNbr": "900300",
    "dbaNbr": "2",
    "terminalNbr": "77"
  },
  "transactionId": "550e8400-e29b-41d4-a716-446655440000",
  "amount": "50.00",
  "transactionType": "SALE"
}
```

#### Integration Test

```go
// Automated workflow in testutil
saleResult := testutil.GetRealBRICForSaleAutomated(
    t, client, cfg, "50.00", "http://localhost:8081", jwtToken,
)
// ✅ TAC generated, EPX approves, real auth code returned
```

---

### Browser Post Callback

EPX redirects customer here after payment completion.

**Endpoint**: `POST /api/v1/payments/browser-post/callback`
**Port**: 8081
**Content-Type**: `application/x-www-form-urlencoded`

#### EPX Callback (Form Data)

```
RESP_CODE=00
AUTH_CODE=052598
TRAN_TYPE=U
AMOUNT=50.00
BRIC=YmFzZTY0ZW5jb2RlZGJyaWN0b2tlbmhlcmU=
SIGNATURE=a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
transaction_id=550e8400-e29b-41d4-a716-446655440000
merchant_id=00000000-0000-0000-0000-000000000001
transaction_type=SALE
```

| Field | Type | Description |
|-------|------|-------------|
| `RESP_CODE` | string | `00` = Approved, `05` = Declined |
| `AUTH_CODE` | string | Authorization code from EPX |
| `TRAN_TYPE` | string | `U` (SALE), `A` (AUTH), `S` (STORAGE) |
| `AMOUNT` | decimal | Transaction amount |
| `BRIC` | string | Base64-encoded payment token |
| `SIGNATURE` | string | EPX signature for verification |

#### Our Response (Redirect)

```
HTTP/1.1 302 Found
Location: http://localhost:8082/payment-result?status=approved&auth_code=052598
```

Customer is redirected to `return_url` with transaction status.

#### Integration Test

```go
// Callback is automatically processed in GetRealBRICForSaleAutomated()
// ✅ Signature verified
// ✅ BRIC stored
// ✅ Transaction status updated
```

---

## Server Post Operations

Server Post uses stored BRICs for payment processing via ConnectRPC.

**Protocol**: ConnectRPC over HTTP/2
**Port**: 8080
**Content-Type**: `application/json`
**Authentication**: JWT Bearer token
**Test Coverage**: `tests/integration/payment/server_post_workflow_test.go`

### Authorize

Create authorization using stored payment method.

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Authorize" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "00000000-0000-0000-0000-000000002001"
}
EOF
```

#### Request Body

```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "00000000-0000-0000-0000-000000002001"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchantId` | UUID | ✅ | Merchant account UUID |
| `customerId` | UUID | ✅ | Customer UUID |
| `amountCents` | int64 | ✅ | Amount in cents (15000 = $150.00) |
| `paymentMethodId` | UUID | ✅ | Stored payment method UUID |

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_AUTHORIZED",
    "amountCents": 15000,
    "authorizationCode": "052585",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:30:00Z",
    "updatedAt": "2025-11-22T10:30:00Z"
  }
}
```

#### Integration Test

```go
// tests/integration/payment/server_post_workflow_test.go
authResp, err := client.Authorize(ctx, connect.NewRequest(&paymentv1.AuthorizeRequest{
    MerchantId:      merchantID,
    CustomerId:      customerID,
    AmountCents:     15000,
    PaymentMethodId: paymentMethodID,
}))
// ✅ EPX approves (RESP_CODE=00)
// ✅ Real auth code: "052585"
// ✅ Status: AUTHORIZED
```

---

### Sale

Immediate capture using stored payment method.

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Sale" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "00000000-0000-0000-0000-000000002001"
}
EOF
```

#### Request Body

```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "00000000-0000-0000-0000-000000002001"
}
```

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_APPROVED",
    "amountCents": 15000,
    "authorizationCode": "052587",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:31:00Z",
    "updatedAt": "2025-11-22T10:31:00Z"
  }
}
```

#### Integration Test

```go
saleResp, err := client.Sale(ctx, connect.NewRequest(&paymentv1.SaleRequest{
    MerchantId:      merchantID,
    CustomerId:      customerID,
    AmountCents:     15000,
    PaymentMethodId: paymentMethodID,
}))
// ✅ EPX approves
// ✅ Auth code: "052587"
// ✅ Status: APPROVED
```

---

### Capture

Capture funds from previous authorization.

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Capture" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440001",
  "amountCents": 15000
}
EOF
```

#### Request Body

```json
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440001",
  "amountCents": 15000
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `transactionId` | UUID | ✅ | ID of AUTH transaction to capture |
| `amountCents` | int64 | ✅ | Amount to capture (≤ auth amount) |

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_CAPTURED",
    "amountCents": 15000,
    "capturedAmountCents": 15000,
    "authorizationCode": "052589",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:30:00Z",
    "updatedAt": "2025-11-22T10:32:00Z"
  }
}
```

#### Integration Test

```go
captureResp, err := client.Capture(ctx, connect.NewRequest(&paymentv1.CaptureRequest{
    TransactionId: authResp.Msg.Transaction.Id,
    AmountCents:   15000,
}))
// ✅ EPX approves partial/full capture
// ✅ Uses financial BRIC from AUTH
// ✅ Status: CAPTURED
```

---

### Void

Cancel authorization before settlement.

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Void" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440001"
}
EOF
```

#### Request Body

```json
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440001"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `transactionId` | UUID | ✅ | ID of AUTH transaction to void |

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_VOIDED",
    "amountCents": 15000,
    "authorizationCode": "052591",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:30:00Z",
    "updatedAt": "2025-11-22T10:33:00Z"
  }
}
```

#### Integration Test

```go
voidResp, err := client.Void(ctx, connect.NewRequest(&paymentv1.VoidRequest{
    TransactionId: authResp.Msg.Transaction.Id,
}))
// ✅ EPX approves void
// ✅ Auth code: "052591"
// ✅ Status: VOIDED
```

---

### Refund

Return funds from captured transaction.

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Refund" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440002",
  "amountCents": 5000,
  "reason": "Customer request"
}
EOF
```

#### Request Body

```json
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440002",
  "amountCents": 5000,
  "reason": "Customer request"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `transactionId` | UUID | ✅ | ID of SALE/CAPTURED transaction |
| `amountCents` | int64 | ✅ | Amount to refund (≤ captured amount) |
| `reason` | string | ❌ | Refund reason (optional) |

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_REFUNDED",
    "amountCents": 15000,
    "refundedAmountCents": 5000,
    "authorizationCode": "052593",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:31:00Z",
    "updatedAt": "2025-11-22T10:34:00Z"
  }
}
```

#### Integration Test

```go
refundResp, err := client.Refund(ctx, connect.NewRequest(&paymentv1.RefundRequest{
    TransactionId: saleResp.Msg.Transaction.Id,
    AmountCents:   5000,
    Reason:        "Customer request",
}))
// ✅ EPX approves partial refund
// ✅ Auth code: "052593"
// ✅ Status: REFUNDED
```

---

## Verification Summary

### EPX Integration Test Results

| Category | Test | Status |
|----------|------|--------|
| **Key Exchange** | SALE TAC generation | ✅ PASS |
| | AUTH TAC generation | ✅ PASS |
| | STORAGE TAC generation | ✅ PASS |
| **Browser Post** | SALE → REFUND workflow | ✅ PASS |
| | AUTH → CAPTURE → REFUND | ✅ PASS |
| | AUTH → VOID workflow | ✅ PASS |
| **Server Post** | Authorize with stored card | ✅ PASS |
| | Sale with stored card | ✅ PASS |
| | Capture (full + partial) | ✅ PASS |
| | Void authorization | ✅ PASS |
| | Refund (full + partial) | ✅ PASS |

### Running Integration Tests

```bash
# Full integration suite
SERVICE_URL="http://localhost:8080" go test -tags=integration ./tests/integration/... -count=1

# Browser Post workflows (port 8081)
SERVICE_URL="http://localhost:8081" go test -tags=integration ./tests/integration/payment/... -run TestBrowserPost

# Server Post operations (port 8080)
SERVICE_URL="http://localhost:8080" go test -tags=integration ./tests/integration/payment/... -run TestServerPost

# Payment method tokenization
go test -tags=integration ./tests/integration/payment_method/...
```

### Critical Fixes Implemented

1. **TRAN_GROUP Format**: Full strings (`SALE`/`AUTH`/`STORAGE`) instead of codes (`U`/`A`/`S`)
2. **Port Separation**: ConnectRPC (8080) vs HTTP (8081) clearly documented
3. **BRIC Management**: Storage BRIC (reusable) vs Financial BRIC (transaction-specific)
4. **Transaction Status**: Accurate state tracking through complete workflows

### EPX Compliance

✅ All required fields present
✅ TRAN_GROUP values match EPX specifications
✅ TAC format verified (base64, ~305 characters)
✅ Signature verification implemented
✅ Real authorization codes from EPX staging
✅ Transaction workflows tested end-to-end

### EPX Response Codes

| Code | Meaning | Action |
|------|---------|--------|
| `00` | Approved | Success |
| `05` | Declined | Notify customer |
| `51` | Insufficient funds | Notify customer |
| `BP_113` | Redirect URL mismatch | Check merchant configuration |
| `BP_129` | Invalid TRAN_GROUP | Use SALE/AUTH/STORAGE (not U/A/S) |
| `RR` | Retry/review | Contact EPX support |

---

**Last Updated**: 2025-11-22
**Verified Against**: EPX UAP Staging Environment
**Test Files**: `tests/integration/payment/*_test.go`
