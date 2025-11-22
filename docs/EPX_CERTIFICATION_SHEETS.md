# EPX Payment Gateway Certification Sheets

**Version**: 1.0
**Date**: 2025-11-22
**Environment**: EPX UAP Staging
**Status**: ✅ VERIFIED via Integration Tests

---

## Table of Contents

1. [Service Architecture](#service-architecture)
2. [Test Merchant Credentials](#test-merchant-credentials)
3. [EPX Key Exchange](#1-epx-key-exchange)
4. [Browser Post Transactions](#2-browser-post-transactions)
5. [Server Post Operations](#3-server-post-operations)
6. [Verification Summary](#verification-summary)

---

## Service Architecture

The payment service runs on two ports:

| Port | Protocol | Purpose |
|------|----------|---------|
| **8080** | ConnectRPC (HTTP/2) | Primary API for all payment operations |
| **8081** | HTTP/1.1 | EPX Browser Post callbacks, Cron endpoints |

### Why Two Ports?

- **Port 8080**: ConnectRPC requires HTTP/2 with specific headers for gRPC-compatible communication
- **Port 8081**: EPX Browser Post callbacks require simple HTTP/1.1 endpoints

### Integration Test Configuration

```bash
# ConnectRPC operations (Authorize, Sale, Capture, Void, Refund)
export CONNECTRPC_URL="http://localhost:8080"

# Browser Post callbacks, Cron handlers
export HTTP_CALLBACK_URL="http://localhost:8081"

# Database
export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable"
```

---

## Test Merchant Credentials

**⚠️ SANDBOX CREDENTIALS - For Testing Only**

```
CUST_NBR:     9001
MERCH_NBR:    900300
DBA_NBR:      2
TERMINAL_NBR: 77
MAC_SECRET:   [Stored in secrets manager]
```

**Test Merchant UUID**: `00000000-0000-0000-0000-000000000001`

### Environment

- **EPX Server Post**: `https://services.epxuap.com`
- **EPX Key Exchange**: `https://keyexch.epxuap.com`
- **EPX Browser Post**: `https://epxuap.com`

---

## 1. EPX Key Exchange

EPX Key Exchange generates TAC (Transaction Access Code) tokens for Browser Post transactions.

### Test Coverage

✅ Verified in: `tests/integration/payment/browser_post_workflow_test.go`

### 1.1 SALE Transaction

**Purpose**: Generate TAC for immediate payment capture.

#### Request

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TRAN_NBR=1000028096" \
  -d "AMOUNT=50.00" \
  -d "MAC=<MAC_SECRET>" \
  -d "TRAN_GROUP=SALE" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `TRAN_NBR` | string | ✅ | Unique transaction number (10 digits) |
| `AMOUNT` | decimal | ✅ | Transaction amount (e.g., "50.00") |
| `MAC` | string | ✅ | Merchant Authentication Code |
| `TRAN_GROUP` | string | ✅ | Transaction type: `SALE`, `AUTH`, or `STORAGE` |
| `REDIRECT_URL` | string | ✅ | URL for EPX to redirect after payment |

#### Response

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELD KEY="TAC">dGVzdHRhY3ZhbHVl...==</FIELD>
</RESPONSE>
```

#### Integration Test

```go
// From tests/integration/payment/browser_post_workflow_test.go
func TestBrowserPost_Workflows(t *testing.T) {
    // Test automatically calls EPX Key Exchange
    saleResult := testutil.GetRealBRICForSaleAutomated(
        t, client, cfg, "50.00",
        "http://localhost:8081",
        jwtToken,
    )

    // Verifies:
    // ✅ TAC received from EPX
    // ✅ TAC length ~300-400 characters (base64)
    // ✅ Transaction approved by EPX
}
```

### 1.2 AUTH Transaction

**Purpose**: Generate TAC for authorization-only (no capture).

#### Request

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TRAN_NBR=1000022329" \
  -d "AMOUNT=50.00" \
  -d "MAC=<MAC_SECRET>" \
  -d "TRAN_GROUP=AUTH" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Integration Test

```go
// From tests/integration/payment/browser_post_workflow_test.go
func TestBrowserPost_Workflows(t *testing.T) {
    t.Run("AUTH_CAPTURE_REFUND", func(t *testing.T) {
        // 1. Get AUTH via Browser Post
        authResult := testutil.GetRealBRICForAuthAutomated(...)

        // Verifies:
        // ✅ AUTH TAC received
        // ✅ Storage BRIC returned
        // ✅ Transaction status = AUTHORIZED
    })
}
```

### 1.3 STORAGE Transaction

**Purpose**: Generate TAC for card tokenization (no payment).

#### Request

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TRAN_NBR=1000029490" \
  -d "AMOUNT=0.00" \
  -d "MAC=<MAC_SECRET>" \
  -d "TRAN_GROUP=STORAGE" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Integration Test

```go
// From tests/integration/payment_method/payment_method_test.go
func TestStorePaymentMethod_CreditCard(t *testing.T) {
    // Get STORAGE BRIC via Browser Post
    bric := testutil.GetRealBRICForStorageAutomated(...)

    // Verifies:
    // ✅ STORAGE TAC received
    // ✅ Reusable BRIC returned
    // ✅ Can use BRIC in future transactions
}
```

### Key Exchange Results

| TRAN_GROUP | TAC Length | Status | Test Coverage |
|------------|------------|--------|---------------|
| `SALE` | ~305 chars | ✅ SUCCESS | `browser_post_workflow_test.go` |
| `AUTH` | ~305 chars | ✅ SUCCESS | `browser_post_workflow_test.go` |
| `STORAGE` | ~305 chars | ✅ SUCCESS | `payment_method_test.go` |

**Critical Finding**: EPX requires full transaction type strings (`SALE`/`AUTH`/`STORAGE`), NOT single-letter codes (`U`/`A`/`S`).

---

## 2. Browser Post Transactions

Browser Post allows customers to enter payment details directly on EPX's hosted payment page.

### Test Coverage

✅ Verified in: `tests/integration/payment/browser_post_workflow_test.go`

### 2.1 Generate Browser Post Form

**Endpoint**: `GET /api/v1/payments/browser-post/form` (Port 8081)

#### Request

```bash
curl "http://localhost:8081/api/v1/payments/browser-post/form?transaction_id=<UUID>&merchant_id=00000000-0000-0000-0000-000000000001&amount=50.00&transaction_type=SALE&return_url=http://localhost:8082/payment-result"
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `transaction_id` | UUID | ✅ | Unique transaction identifier |
| `merchant_id` | UUID | ✅ | Merchant account UUID |
| `amount` | decimal | ✅ | Transaction amount |
| `transaction_type` | string | ✅ | `SALE`, `AUTH`, or `STORAGE` |
| `return_url` | string | ✅ | URL to redirect customer after payment |

#### Response

```json
{
  "tac": "dGVzdHRhY3ZhbHVl...==",
  "postURL": "https://epxuap.com",
  "epxTranNbr": "1000028096",
  "merchantCredentials": {
    "custNbr": "9001",
    "merchNbr": "900300",
    "dbaNbr": "2",
    "terminalNbr": "77"
  }
}
```

#### Integration Test

```go
// From tests/integration/payment/browser_post_workflow_test.go
func TestBrowserPost_Workflows(t *testing.T) {
    t.Run("SALE_to_REFUND", func(t *testing.T) {
        // 1. Get Browser Post form
        // 2. Submit to EPX
        // 3. Process callback
        saleResult := testutil.GetRealBRICForSaleAutomated(...)

        // Verifies:
        // ✅ TAC generated
        // ✅ EPX approves transaction
        // ✅ Real auth code returned (e.g., "052598")
        // ✅ Transaction status = APPROVED
        // ✅ Financial BRIC stored
    })
}
```

### 2.2 Browser Post Callback

**Endpoint**: `POST /api/v1/payments/browser-post/callback` (Port 8081)

EPX redirects customer here after payment completion.

#### EPX Callback Data

```
RESP_CODE=00
AUTH_CODE=052598
TRAN_TYPE=U
AMOUNT=50.00
BRIC=<base64_encoded_bric>
SIGNATURE=<epx_signature>
```

#### Our Processing

```go
// From internal/handlers/payment/browser_post_callback_handler.go
func (h *BrowserPostCallbackHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
    // 1. Parse EPX response
    // 2. Verify signature
    // 3. Update transaction status
    // 4. Store BRIC if approved
    // 5. Redirect customer to return_url
}
```

#### Integration Test Coverage

```go
// Verified in browser_post_workflow_test.go
✅ SALE transaction approval
✅ AUTH transaction approval
✅ STORAGE transaction approval
✅ Signature verification
✅ BRIC storage
✅ Status updates
```

### 2.3 Complete Workflows

#### SALE → REFUND

**Test**: `TestBrowserPost_Workflows/SALE_to_REFUND`

```go
1. Browser Post SALE ($50.00)
   ✅ EPX approves with auth code "052598"
   ✅ Status: TRANSACTION_STATUS_APPROVED
   ✅ Financial BRIC stored

2. Server Post REFUND ($25.00) via ConnectRPC
   ✅ Partial refund successful
   ✅ Status: TRANSACTION_STATUS_REFUNDED
```

#### AUTH → CAPTURE → REFUND

**Test**: `TestBrowserPost_Workflows/AUTH_CAPTURE_REFUND`

```go
1. Browser Post AUTH ($50.00)
   ✅ EPX approves authorization
   ✅ Storage BRIC stored
   ✅ Status: TRANSACTION_STATUS_AUTHORIZED

2. Server Post CAPTURE ($50.00) via ConnectRPC
   ✅ Uses financial BRIC from AUTH
   ✅ Status: TRANSACTION_STATUS_CAPTURED

3. Server Post REFUND ($25.00) via ConnectRPC
   ✅ Partial refund successful
   ✅ Status: TRANSACTION_STATUS_REFUNDED
```

#### AUTH → VOID

**Test**: `TestBrowserPost_Workflows/AUTH_VOID`

```go
1. Browser Post AUTH ($100.00)
   ✅ EPX approves authorization
   ✅ Status: TRANSACTION_STATUS_AUTHORIZED

2. Server Post VOID via ConnectRPC
   ✅ Cancels authorization before settlement
   ✅ Status: TRANSACTION_STATUS_VOIDED
```

---

## 3. Server Post Operations

Server Post operations use stored BRICs for payment processing via ConnectRPC.

### Test Coverage

✅ Verified in: `tests/integration/payment/server_post_workflow_test.go`

### Architecture

```
Client → ConnectRPC (Port 8080) → Payment Service → EPX Server Post API
```

### 3.1 Authorize with Stored Card

**Operation**: Create authorization using stored payment method.

#### ConnectRPC Request

```bash
# Protocol: ConnectRPC over HTTP/2
# Port: 8080

POST /payment.v1.PaymentService/Authorize
Host: localhost:8080
Content-Type: application/json
Authorization: Bearer <JWT_TOKEN>

{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "<payment-method-uuid>"
}
```

#### Response

```json
{
  "transaction": {
    "id": "<transaction-uuid>",
    "status": "TRANSACTION_STATUS_AUTHORIZED",
    "amountCents": 15000,
    "authorizationCode": "052585",
    "gatewayResponse": "EXACT MATCH"
  }
}
```

#### Integration Test

```go
// From tests/integration/payment/server_post_workflow_test.go
func TestServerPost_AuthorizeWithStoredCard(t *testing.T) {
    // 1. Store card via Browser Post STORAGE
    bric := testutil.GetRealBRICForStorageAutomated(...)

    // 2. Authorize using stored card
    authResp, err := client.Authorize(ctx, connect.NewRequest(&paymentv1.AuthorizeRequest{
        MerchantId:      merchantID,
        CustomerId:      customerID,
        AmountCents:     15000,
        PaymentMethodId: paymentMethodID,
    }))

    // Verifies:
    // ✅ EPX approves (RESP_CODE=00)
    // ✅ Real auth code received
    // ✅ Status = AUTHORIZED
    // ✅ Financial BRIC stored
}
```

### 3.2 Sale with Stored Card

**Operation**: Immediate capture using stored payment method.

#### ConnectRPC Request

```bash
POST /payment.v1.PaymentService/Sale
Host: localhost:8080
Authorization: Bearer <JWT_TOKEN>

{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "<payment-method-uuid>"
}
```

#### Integration Test

```go
func TestServerPost_SaleWithStoredCard(t *testing.T) {
    // 1. Store card
    // 2. Execute SALE
    saleResp, err := client.Sale(ctx, ...)

    // Verifies:
    // ✅ EPX approves
    // ✅ Auth code: "052587"
    // ✅ Status = APPROVED
}
```

### 3.3 Capture

**Operation**: Capture funds from previous authorization.

#### ConnectRPC Request

```bash
POST /payment.v1.PaymentService/Capture
Host: localhost:8080
Authorization: Bearer <JWT_TOKEN>

{
  "transactionId": "<auth-transaction-uuid>",
  "amountCents": 15000
}
```

#### Integration Test

```go
func TestServerPost_CapturePartialAmount(t *testing.T) {
    // 1. Authorize $150.00
    authResp := ...

    // 2. Capture $100.00 (partial)
    captureResp, err := client.Capture(ctx, connect.NewRequest(&paymentv1.CaptureRequest{
        TransactionId: authResp.Msg.Transaction.Id,
        AmountCents:   10000,
    }))

    // Verifies:
    // ✅ EPX approves partial capture
    // ✅ Uses financial BRIC from AUTH
    // ✅ Status = CAPTURED
}
```

### 3.4 Void

**Operation**: Cancel authorization before settlement.

#### ConnectRPC Request

```bash
POST /payment.v1.PaymentService/Void
Host: localhost:8080
Authorization: Bearer <JWT_TOKEN>

{
  "transactionId": "<auth-transaction-uuid>"
}
```

#### Integration Test

```go
func TestServerPost_VoidAuthorization(t *testing.T) {
    // 1. Authorize
    authResp := ...

    // 2. Void before settlement
    voidResp, err := client.Void(ctx, ...)

    // Verifies:
    // ✅ EPX approves void
    // ✅ Auth code: "052591"
    // ✅ Status = VOIDED
}
```

### 3.5 Refund

**Operation**: Return funds from captured transaction.

#### ConnectRPC Request

```bash
POST /payment.v1.PaymentService/Refund
Host: localhost:8080
Authorization: Bearer <JWT_TOKEN>

{
  "transactionId": "<sale-transaction-uuid>",
  "amountCents": 5000,
  "reason": "Customer request"
}
```

#### Integration Test

```go
func TestServerPost_PartialRefund(t *testing.T) {
    // 1. SALE $100.00
    saleResp := ...

    // 2. Refund $50.00 (partial)
    refundResp, err := client.Refund(ctx, connect.NewRequest(&paymentv1.RefundRequest{
        TransactionId: saleResp.Msg.Transaction.Id,
        AmountCents:   5000,
        Reason:        "Customer request",
    }))

    // Verifies:
    // ✅ EPX approves partial refund
    // ✅ Auth code: "052593"
    // ✅ Status = REFUNDED
}
```

### 3.6 Concurrent Operations

**Test**: `TestServerPost_ConcurrentOperations`

#### Scenario

```go
// Start with one AUTH transaction
authResp := authorizeTransaction(...)

// Attempt concurrent CAPTURE + VOID
go func() { client.Capture(...) }()
go func() { client.Void(...) }()
```

#### Verifies

```
✅ No data corruption
✅ Proper transaction locking
✅ One operation succeeds, other fails with clear error
✅ Database consistency maintained
```

---

## Verification Summary

### EPX Integration Tests

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
| | Concurrent operations | ✅ PASS |

### Test Execution

```bash
# ConnectRPC operations (port 8080)
SERVICE_URL="http://localhost:8080" go test -tags=integration ./tests/integration/payment/... -run TestServerPost

# Browser Post workflows (port 8081)
SERVICE_URL="http://localhost:8081" go test -tags=integration ./tests/integration/payment/... -run TestBrowserPost

# Payment method tokenization
go test -tags=integration ./tests/integration/payment_method/...
```

### Critical Fixes Implemented

1. **TRAN_GROUP Format**: Changed from single-letter codes to full strings
   - Before: `U`, `A`, `S`
   - After: `SALE`, `AUTH`, `STORAGE`
   - Impact: All EPX transactions now approved (no more BP_129 errors)

2. **Port Separation**: Clarified ConnectRPC vs HTTP usage
   - Port 8080: ConnectRPC operations
   - Port 8081: Browser Post callbacks, Cron handlers

3. **BRIC Management**: Proper Storage vs Financial BRIC handling
   - Storage BRIC: From STORAGE transactions, reusable
   - Financial BRIC: From AUTH/SALE, for CAPTURE/VOID/REFUND only

4. **Transaction Status**: Accurate status tracking
   - `AUTHORIZED` → `CAPTURED` → `REFUNDED`
   - `AUTHORIZED` → `VOIDED`
   - `APPROVED` → `REFUNDED`

### EPX Compliance

✅ All required fields present
✅ TRAN_GROUP values match EPX specifications
✅ TAC format verified (base64, ~300-400 chars)
✅ Signature verification implemented
✅ Real authorization codes from EPX staging
✅ Transaction workflows tested end-to-end

---

## Running Integration Tests

### Prerequisites

```bash
# Start services
docker-compose up -d

# Wait for health checks
docker-compose ps
```

### Execute Tests

```bash
# Full integration suite
SERVICE_URL="http://localhost:8080" go test -tags=integration ./tests/integration/... -count=1

# Browser Post only
SERVICE_URL="http://localhost:8081" go test -tags=integration ./tests/integration/payment/... -run TestBrowserPost

# Server Post only
SERVICE_URL="http://localhost:8080" go test -tags=integration ./tests/integration/payment/... -run TestServerPost

# Payment method tokenization
go test -tags=integration ./tests/integration/payment_method/...
```

### With Verbose Output

```bash
SERVICE_URL="http://localhost:8080" go test -v -tags=integration ./tests/integration/payment/... -run TestServerPost_AuthorizeWithStoredCard
```

---

## Appendix: EPX Response Codes

| Code | Meaning | Action |
|------|---------|--------|
| `00` | Approved | Success |
| `05` | Declined | Notify customer |
| `51` | Insufficient funds | Notify customer |
| `BP_113` | Redirect URL mismatch | Check merchant config |
| `BP_129` | Invalid TRAN_GROUP | Use SALE/AUTH/STORAGE (not U/A/S) |
| `RR` | Retry/review | Contact EPX support |

---

**Last Updated**: 2025-11-22
**Verified Against**: EPX UAP Staging Environment
**Test Suite**: `tests/integration/payment/*_test.go`
**All Systems**: ✅ OPERATIONAL
