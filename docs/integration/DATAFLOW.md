# Payment Dataflows

**Audience:** Developers integrating with the payment service.
**Topic:** Complete dataflow documentation for all payment methods and authentication flows.
**Goal:** Enable developers to understand and implement payment workflows correctly.

## Overview

This document explains the complete dataflow for all payment methods:
- **Browser Post** - PCI-compliant credit card payments
- **Server Post** - Direct server-to-server payments (credit cards & ACH)
- **ACH** - Bank account debits
- **Authentication** - Token-based authorization

---

## Browser Post Dataflow

### Overview

Browser Post enables **PCI-compliant credit card payments** where card data flows directly from browser to EPX gateway, never touching your backend.

**Key Pattern:** PENDING→UPDATE lifecycle
- Transaction created in PENDING state when form is generated
- Transaction updated to COMPLETED/FAILED when callback arrives
- Backend service stores `group_id` to link payment with order

### Architecture Diagram

```text
┌────────────┐    ┌────────────┐    ┌─────────────┐    ┌──────────┐
│  Backend   │    │  Payment   │    │   Browser   │    │   EPX    │
│  Service   │    │  Service   │    │             │    │ Gateway  │
└─────┬──────┘    └─────┬──────┘    └──────┬──────┘    └────┬─────┘
      │                 │                   │                │
      │ 1. GetPaymentForm(amount, return_url, merchant_id)  │
      ├────────────────>│                   │                │
      │                 │                   │                │
      │                 │ 2. CREATE PENDING TRANSACTION      │
      │                 │    (transaction_id, group_id)      │
      │                 │                   │                │
      │ 3. {transaction_id, group_id, form_config}          │
      │<────────────────┤                   │                │
      │                 │                   │                │
      │ 4. Store: order→group_id mapping    │                │
      │                 │                   │                │
      │ 5. Send form config to frontend     │                │
      ├─────────────────────────────────────>│                │
      │                 │                   │                │
      │                 │ 6. Render HTML form                │
      │                 │                   │                │
      │                 │ 7. User submits card data          │
      │                 │                   │                │
      │                 │ 8. POST card data to EPX           │
      │                 │                   ├───────────────>│
      │                 │                   │                │
      │                 │                   │ 9. Process     │
      │                 │                   │                │
      │                 │ 10. Redirect to callback           │
      │                 │                   │<───────────────┤
      │                 │                   │                │
      │                 │ 11. POST callback data             │
      │                 │<──────────────────┤                │
      │                 │                   │                │
      │                 │ 12. UPDATE PENDING→COMPLETED       │
      │                 │                   │                │
      │                 │ 13. Redirect to return_url         │
      │                 ├───────────────────>│                │
      │                 │                   │                │
      │ 14. Browser lands at /complete      │                │
      │<────────────────────────────────────┤                │
      │                 │                   │                │
      │ 15. Look up order by group_id       │                │
      │     Render receipt                  │                │
```

### Step-by-Step Flow

#### Step 1: Request Payment Form

**Option A: Frontend calls Payment Service directly**
```http
GET /api/v1/payments/browser-post/form?amount=99.99&return_url=https://app.example.com/complete&merchant_id=merchant-123
```

**Option B: Frontend → Backend Service → Payment Service**
```typescript
// Frontend → Your Backend
POST /api/orders/123/checkout

// Your Backend → Payment Service
GET /api/v1/payments/browser-post/form?amount=99.99&return_url=https://app.example.com/complete&merchant_id=merchant-123
```

**Parameters:**
- `amount` (required): Transaction amount as decimal string
- `return_url` (required): Where to redirect after payment
- `merchant_id` (required): Merchant identifier

#### Step 2: Service Creates PENDING Transaction

```go
CreateTransaction(ctx, CreateTransactionParams{
    ID:             txID,           // UUID
    GroupID:        groupID,        // Links related transactions
    MerchantID:     merchantID,
    Amount:         amount,
    Status:         "pending",
    IdempotencyKey: tranNbr,        // For callback lookup
})
```

#### Step 3: Return Form Configuration

```json
{
  "transaction_id": "054edaac-3770-4222-ab50-e09b41051cc4",
  "group_id": "9b3d3df9-e37b-47ca-83f8-106b51b0ff50",
  "post_url": "https://secure.epxuap.com/browserpost",
  "amount": "99.99",
  "tran_nbr": "45062844883",
  "redirect_url": "http://payment-service:8081/api/v1/payments/browser-post/callback",
  "user_data_1": "return_url=https://app.example.com/complete",
  "cust_nbr": "9001",
  "merch_nbr": "900300"
}
```

#### Step 4: Backend Service Stores Mapping

```sql
UPDATE orders
SET payment_group_id = '9b3d3df9-e37b-47ca-83f8-106b51b0ff50',
    payment_status = 'PENDING'
WHERE order_id = 'ORDER-123';
```

**Why?** Enables order lookup when payment completes.

#### Step 5-7: Frontend Renders and Submits Form

```html
<form method="POST" action="https://secure.epxuap.com/browserpost">
  <!-- Hidden fields from backend -->
  <input type="hidden" name="CUST_NBR" value="9001">
  <input type="hidden" name="AMOUNT" value="99.99">
  <input type="hidden" name="TRAN_NBR" value="45062844883">
  <input type="hidden" name="REDIRECT_URL" value="http://payment-service:8081/callback">

  <!-- User-entered card data -->
  <input type="text" name="CARD_NBR" placeholder="4111111111111111">
  <input type="text" name="EXP_MONTH" placeholder="12">
  <input type="text" name="EXP_YEAR" placeholder="2025">
  <input type="text" name="CVV" placeholder="123">

  <button type="submit">Pay $99.99</button>
</form>
```

**Security:** Card data POSTs directly to EPX, never touches your backend.

#### Step 8-12: EPX Processes and Callback

**EPX processes payment and redirects browser to callback:**

```text
POST /api/v1/payments/browser-post/callback
TRAN_NBR=45062844883
AUTH_GUID=ABC123XYZ
AUTH_RESP=00
AUTH_CODE=OK1234
```

**Payment Service updates transaction:**
```go
UpdateTransaction(ctx, UpdateTransactionParams{
    ID:       existingTx.ID,
    Status:   "completed",  // or "failed"
    AuthGuid: response.AuthGUID,
    AuthResp: response.AuthResp,
})
```

#### Step 13-14: Redirect to Backend Service

```text
https://app.example.com/complete?group_id=9b3d3df9-...&status=completed&amount=99.99&card_type=VISA
```

#### Step 15: Backend Service Renders Receipt

```sql
SELECT * FROM orders WHERE payment_group_id = '9b3d3df9-...'
```

Update order status and render complete receipt.

### Security Considerations

**State Parameter Pattern**
- `return_url` passed through EPX via USER_DATA_1
- Zero coupling between services

**Callback URL Whitelisting**
- REDIRECT_URL must be whitelisted with EPX
- Prevents malicious redirects

**Idempotency**
- Look up by `idempotency_key` prevents duplicate processing
- EPX may send multiple callbacks

---

## Server Post Dataflow

### Overview

Server POST supports **direct server-to-server** payments:
- Credit card payments using saved payment methods or tokens
- ACH payments (bank account debits)
- Synchronous response with complete transaction data

**Key Characteristics:**
- ConnectRPC API calls (HTTP/JSON or binary protocol)
- Uses saved payment methods or BRIC tokens
- PCI-compliant (no raw card data)

### Architecture Diagram

```text
┌─────────────────────────────────────────────────────────────┐
│ BACKEND SERVICE                                              │
│  1. Send SaleRequest via ConnectRPC:                        │
│     - merchant_id, amount_cents, currency                   │
│     - payment_method_id OR payment_token                    │
│     - idempotency_key                                       │
└──────────────────────────┬──────────────────────────────────┘
                           │ ConnectRPC
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ PAYMENT SERVICE (Handler)                                   │
│  - Validates request                                        │
│  - Calls service layer                                      │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ PAYMENT SERVICE (Service Layer)                             │
│  - Retrieves payment method or validates token             │
│  - Calls EPX adapter                                        │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ EPX GATEWAY                                                  │
│  - Processes transaction                                    │
│  - Returns: AUTH_GUID, AUTH_RESP, AUTH_CODE                │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ PAYMENT SERVICE (Service Layer)                             │
│  - Creates transaction record                               │
│  - Returns PaymentResponse                                  │
└──────────────────────────┬──────────────────────────────────┘
                           │ ConnectRPC
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ BACKEND SERVICE                                              │
│  2. Receive PaymentResponse:                               │
│     - transaction_id, parent_transaction_id, status        │
│     - auth_code, auth_resp                                 │
└─────────────────────────────────────────────────────────────┘
```

### Request/Response Example

**ConnectRPC Request:**
```protobuf
message SaleRequest {
  string merchant_id = 1;
  string customer_id = 2;
  int64 amount_cents = 3;           // Amount in cents (e.g., 9999 = $99.99)
  string currency = 4;
  oneof payment_method {
    string payment_method_id = 5;    // Saved payment method
    string payment_token = 6;        // EPX BRIC token
  }
  string idempotency_key = 7;
}
```

**ConnectRPC Response:**
```protobuf
message PaymentResponse {
  string transaction_id = 1;
  string parent_transaction_id = 2; // Links to parent transaction (empty for sale)
  int64 amount_cents = 5;           // Amount in cents
  TransactionStatus status = 7;     // COMPLETED, FAILED
  string auth_guid = 10;            // BRIC for future use
  string auth_resp = 11;            // "00" = approved
  bool is_approved = 17;
}
```

### Use Cases

**Recurring Payments:** Monthly subscriptions with saved payment method
**Saved Card Payments:** Fast checkout with stored token
**ACH Payments:** Invoice payments, large amounts

---

## ACH Payment Dataflow

### Overview

ACH (Automated Clearing House) enables **bank account debits**:
- Lower fees than cards (flat fee vs percentage)
- Settlement: 1-3 business days
- Good for subscriptions and invoices

### Architecture Diagram

```text
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Client    │         │   Backend   │         │     EPX     │
│             │         │   Service   │         │   Gateway   │
└──────┬──────┘         └──────┬──────┘         └──────┬──────┘
       │                       │                       │
       │  1. Provide Bank      │                       │
       │     Account Info      │                       │
       ├──────────────────────>│                       │
       │                       │                       │
       │                       │  2. Build ACH Request │
       │                       │                       │
       │                       │  3. POST to EPX       │
       │                       ├──────────────────────>│
       │                       │                       │
       │                       │                       │  4. Submit
       │                       │                       │     to ACH
       │                       │                       │     Network
       │                       │                       │
       │                       │  5. Response with     │
       │                       │     AUTH_GUID         │
       │                       │<──────────────────────┤
       │                       │                       │
       │  6. Confirmation      │                       │
       │<──────────────────────┤                       │
```

### Request Format

**Note:** This shows the raw EPX gateway format. When using the Payment Service API, use `amount_cents` (int64) - the service converts to EPX format internally.

**EPX Gateway Format:**
```
CUST_NBR=123456
&MERCH_NBR=789012
&TRAN_TYPE=ACE1
&AMOUNT=150.00
&ACCOUNT_NBR=1234567890
&ROUTING_NBR=021000021
&ACCOUNT_TYPE=C
&NAME=John Doe
&TRAN_NBR=ACH-2025-001
```

**Payment Service API Format:**
```json
{
  "merchant_id": "merchant-123",
  "amount_cents": 15000,
  "payment_method_id": "pm-ach-uuid",
  "idempotency_key": "ach_debit_001"
}
```

**Transaction Types:**
- `ACE1`: ACH Sale (debit)
- `ACC1`: ACH Credit
- `ACP1`: ACH Pre-Note (verify account)

### Response Format

```xml
<RESPONSE>
    <FIELD KEY="AUTH_GUID">ACH-BRIC-09MBFZ3PV6BTRETVQK2</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">ACH001084</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ACH ACCEPTED</FIELD>
</RESPONSE>
```

**Key Differences from Cards:**
- No AVS or CVV verification
- AUTH_RESP "00" = accepted (not settled)
- Actual settlement takes 1-3 days

### Recurring ACH Pattern

**First Payment - Collect Account:**
```go
request := &ACHDebitRequest{
    MerchantID:     "merchant-123",
    AmountCents:    15000,  // $150.00
    PaymentMethodID: "pm-ach-uuid",
    IdempotencyKey: "ach_001",
}
// Response includes AUTH_GUID (BRIC token) in payment method
```

**Future Payments - Use Saved Payment Method:**
```go
request := &ACHDebitRequest{
    MerchantID:     "merchant-123",
    AmountCents:    15000,  // $150.00
    PaymentMethodID: "pm-ach-uuid",  // Saved from first payment
    IdempotencyKey: "ach_002",
}
```

**Benefits:**
- No bank info needed after first payment
- Faster processing
- Lower compliance burden

### ACH vs Credit Card

| Feature | Credit Card | ACH |
|---------|------------|-----|
| **Settlement** | Real-time authorization | 1-3 business days |
| **Response Time** | ~1 second | ~1 second (acceptance) |
| **Fees** | 2-3% | Flat fee < $1 |
| **Reversals** | Chargebacks (60-120 days) | Returns (60 days) |

---

## Authentication Flows

### Overview

All API requests require authentication via **JWT tokens** that contain authorization context.

### Token Structure

```json
{
  "sub": "token_subject_id",
  "token_type": "merchant",
  "merchant_ids": ["merchant_123"],
  "customer_id": null,
  "scopes": ["payments:create", "payments:read"],
  "exp": 1736683200
}
```

### Token Types

#### 1. Merchant Token (Single Merchant)

**Use Case:** Backend service managing one merchant

```json
{
  "sub": "service_001",
  "token_type": "merchant",
  "merchant_ids": ["merchant_abc123"],
  "scopes": ["payments:create", "payments:read"],
  "exp": 1736683200
}
```

**Authorization:**
- Can create payments for `merchant_abc123`
- Can view transactions for `merchant_abc123`
- Cannot access other merchants

#### 2. Multi-Merchant Token

**Use Case:** Payment processor managing multiple merchants

```json
{
  "sub": "operator_001",
  "token_type": "merchant",
  "merchant_ids": ["merchant_1", "merchant_2", "merchant_3"],
  "scopes": ["payments:create"],
  "exp": 1736683200
}
```

**Authorization:**
- Must specify `merchant_id` in request
- Can only access merchants in array

#### 3. Customer Token

**Use Case:** Customer viewing their payment history

```json
{
  "sub": "customer_xyz789",
  "token_type": "customer",
  "merchant_ids": [],
  "customer_id": "customer_xyz789",
  "scopes": ["payments:read"],
  "exp": 1736683200
}
```

**Authorization:**
- Can view own transactions only
- Cannot create payments directly

#### 4. Guest Token

**Use Case:** Anonymous checkout

```json
{
  "sub": "guest_session_abc",
  "token_type": "guest",
  "merchant_ids": ["merchant_123"],
  "session_id": "sess_abc123",
  "scopes": ["payments:create"],
  "exp": 1736685000
}
```

**Authorization:**
- Short-lived (30 minutes)
- Can view transactions from session only

#### 5. Admin Token

**Use Case:** Support staff

```json
{
  "sub": "admin_001",
  "token_type": "admin",
  "merchant_ids": [],
  "scopes": ["*"],
  "exp": 1736683200
}
```

**Authorization:**
- Can access any data
- All actions logged

### Authorization Flow

```
1. Request with JWT in Authorization header
   ↓
2. Validate JWT signature and expiration
   ↓
3. Extract claims (token_type, merchant_ids, etc.)
   ↓
4. Apply authorization rules
   ↓
5. Return data or 404 (not 403 to prevent enumeration)
```

### Token Issuance

**Your Backend Service issues tokens:**

```typescript
const token = jwt.sign({
    sub: 'service_001',
    token_type: 'merchant',
    merchant_ids: ['merchant-123'],
    scopes: ['payments:create', 'payments:read'],
    exp: Math.floor(Date.now() / 1000) + (8 * 3600),
}, JWT_SECRET);
```

### Idempotency Pattern

**All payment operations require idempotency keys:**

```json
{
  "merchant_id": "merchant-123",
  "amount_cents": 9999,
  "idempotency_key": "payment_20250112_001"
}
```

**Behavior:**
1. First request creates transaction
2. Duplicate requests return existing transaction
3. Prevents double-charging on network retries

**Important:** Generate NEW key for each payment attempt.

---

## Summary

### Flow Selection Guide

| Use Case | Flow | Why |
|----------|------|-----|
| Checkout with card entry | Browser Post | PCI compliant |
| Saved card payment | Server Post | Fast, uses token |
| Subscription billing | Server Post | Automatic recurring |
| Invoice payment | ACH | Lower fees |
| Guest checkout | Browser Post | No account needed |

### Security Best Practices

1. **Never store raw card data** - Use tokens only
2. **Always use idempotency keys** - Prevent duplicates
3. **Return 404, not 403** - Prevent enumeration
4. **Store group_id with orders** - Enable refunds
5. **Use token-based auth** - Context in JWT

### Common Patterns

**Payment Lifecycle:**
```
Auth → Capture → Partial Refund
(All linked by group_id)
```

**Saved Payment Method:**
```
1. Browser Post → Get BRIC token
2. Convert to Storage BRIC
3. Use for recurring charges
```

---

## References

- API Specifications: `API_SPECS.md`
- Database Schema: `DATABASE.md`
- Authentication Details: `AUTH.md`
- Development Guide: `DEVELOP.md`