# Browser POST Payment Flow - Technical Reference

**Date**: 2025-11-11
**Status**: ✅ CURRENT - Reflects PENDING→UPDATE transaction pattern
**Architecture**: Payment Service = Gateway Integration ONLY

---

## Overview

EPX Browser POST API enables PCI-compliant **credit card payments** where card data flows directly from browser to EPX, never touching merchant backend. Payment Service creates PENDING transactions immediately for audit trail, then updates them when EPX callback arrives.

> **Payment Method Support**: Browser POST API supports **credit cards only**. For ACH payments, use Server POST API instead.

**Key Pattern**: PENDING→UPDATE lifecycle
- Transaction created in PENDING state at form generation
- Transaction updated to COMPLETED/FAILED at callback
- Calling service (POS/e-commerce) stores group_id and renders complete receipts

---

## Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────┐
│                  BROWSER POST PAYMENT FLOW (PENDING→UPDATE)          │
└──────────────────────────────────────────────────────────────────────┘

┌────────────┐    ┌────────────┐    ┌─────────────┐    ┌──────────┐
│    POS     │    │  Payment   │    │   Browser   │    │   EPX    │
│  Backend   │    │  Service   │    │             │    │ Gateway  │
└─────┬──────┘    └─────┬──────┘    └──────┬──────┘    └────┬─────┘
      │                 │                   │                │
      │ 1. GetPaymentForm(amount, return_url, agent_id)     │
      ├────────────────>│                   │                │
      │                 │                   │                │
      │                 │ 2. CREATE PENDING TRANSACTION      │
      │                 │    (txID, groupID)                 │
      │                 │                   │                │
      │ 3. {transactionId, groupId, formFields}              │
      │<────────────────┤                   │                │
      │                 │                   │                │
      │ 4. Store: order→groupId mapping     │                │
      │                 │                   │                │
      │ 5. Send form config to frontend     │                │
      ├─────────────────────────────────────>│                │
      │                 │                   │                │
      │                 │ 6. Render HTML form with EPX fields│
      │                 │                   │                │
      │                 │ 7. User enters card + submits      │
      │                 │                   │                │
      │                 │ 8. POST card data directly to EPX  │
      │                 │                   ├───────────────>│
      │                 │                   │                │
      │                 │                   │ 9. Process payment
      │                 │                   │    (authorize)  │
      │                 │                   │                │
      │                 │ 10. Redirect to Payment Service callback
      │                 │                   │<───────────────┤
      │                 │                   │                │
      │                 │ 11. POST callback with EPX response│
      │                 │<──────────────────┤                │
      │                 │                   │                │
      │                 │ 12. UPDATE PENDING→COMPLETED/FAILED│
      │                 │     Look up by idempotency_key     │
      │                 │                   │                │
      │                 │ 13. Extract return_url from USER_DATA_1
      │                 │                   │                │
      │                 │ 14. Redirect browser to POS        │
      │                 │     with groupId + status          │
      │                 ├───────────────────>│                │
      │                 │                   │                │
      │ 15. Browser lands at POS /complete  │                │
      │<────────────────────────────────────┤                │
      │                 │                   │                │
      │ 16. Look up order by groupId        │                │
      │     Mark as PAID                    │                │
      │                 │                   │                │
      │ 17. Render complete receipt         │                │
      │     (order items, cash, tips, etc.) │                │
      ├─────────────────────────────────────>│                │
```

---

## Step-by-Step Dataflow

### Step 1: POS Calls GetPaymentForm

**Endpoint**: `GET /api/v1/payments/browser-post/form`

**Request**:
```
GET /api/v1/payments/browser-post/form?amount=99.99&return_url=https://pos.example.com/payment-complete&agent_id=merchant-123
```

**Parameters**:
- `amount` (required): Transaction amount as decimal string
- `return_url` (required): Where to redirect after payment completes
- `agent_id` (optional): Merchant identifier (defaults to EPX custNbr)

**Purpose**: Request form configuration for browser POST payment

---

### Step 2: Payment Service Creates PENDING Transaction

**Component**: BrowserPostCallbackHandler.GetPaymentForm()
**File**: `internal/handlers/payment/browser_post_callback_handler.go:73-212`

**Process**:
1. Generate unique transaction number (TRAN_NBR) using timestamp+microseconds
2. Create UUIDs for transaction_id and group_id
3. Create transaction in database with status=PENDING:
   ```go
   CreateTransaction(ctx, CreateTransactionParams{
       ID:             txID,           // UUID
       GroupID:        groupID,        // UUID for linking
       AgentID:        agentID,
       Amount:         amountNumeric,
       Currency:       "USD",
       Status:         "pending",      // PENDING - audit trail
       Type:           "charge",
       IdempotencyKey: tranNbr,        // For callback lookup
   })
   ```
4. Pass return_url through EPX via USER_DATA_1 field (state parameter pattern)

**Result**: PENDING transaction exists before user even sees payment form

**Benefit**: Complete audit trail of all payment attempts, including abandoned

---

### Step 3: Payment Service Returns Form Configuration

**Response** (JSON):
```json
{
  "transactionId": "054edaac-3770-4222-ab50-e09b41051cc4",
  "groupId": "9b3d3df9-e37b-47ca-83f8-106b51b0ff50",
  "postURL": "https://secure.epxuap.com/browserpost",
  "amount": "99.99",
  "tranNbr": "45062844883",
  "tranGroup": "SALE",
  "tranCode": "SALE",
  "industryType": "E",
  "cardEntMeth": "E",
  "redirectURL": "http://payment-service:8081/api/v1/payments/browser-post/callback",
  "userData1": "return_url=https://pos.example.com/payment-complete",
  "custNbr": "9001",
  "merchNbr": "900300",
  "dBAnbr": "2",
  "terminalNbr": "77",
  "merchantName": "Payment Service"
}
```

**Critical Fields**:
- `transactionId`, `groupId`: Already created PENDING transaction
- `userData1`: Contains return_url (state parameter pattern)
- EPX credentials: custNbr, merchNbr, dBAnbr, terminalNbr
- `redirectURL`: Payment Service callback (EPX whitelisted)

---

### Step 4: POS Stores group_id Mapping

**Action**: POS backend receives form configuration

**POS Database**:
```sql
UPDATE orders
SET payment_group_id = '9b3d3df9-e37b-47ca-83f8-106b51b0ff50',
    payment_status = 'PENDING'
WHERE order_id = 'ORDER-123';
```

**Purpose**: Link order to payment before completion
- Enables order lookup when callback arrives
- Tracks abandoned payments (orders stuck in PENDING)

---

### Step 5-7: Frontend Renders and Submits Form

**POS sends form config to frontend**, frontend creates HTML form:

```html
<form method="POST" action="https://secure.epxuap.com/browserpost">
  <!-- Hidden fields from backend -->
  <input type="hidden" name="CUST_NBR" value="9001">
  <input type="hidden" name="MERCH_NBR" value="900300">
  <input type="hidden" name="AMOUNT" value="99.99">
  <input type="hidden" name="TRAN_NBR" value="45062844883">
  <input type="hidden" name="USER_DATA_1" value="return_url=https://pos.example.com/payment-complete">
  <input type="hidden" name="REDIRECT_URL" value="http://payment-service:8081/api/v1/payments/browser-post/callback">

  <!-- User-entered card data -->
  <input type="text" name="CARD_NBR" placeholder="4111111111111111">
  <input type="text" name="EXP_MONTH" placeholder="12">
  <input type="text" name="EXP_YEAR" placeholder="2025">
  <input type="text" name="CVV" placeholder="123">
  <input type="text" name="FIRST_NAME">
  <input type="text" name="LAST_NAME">

  <button type="submit">Pay $99.99</button>
</form>
```

**PCI Compliance**: Card data POSTs directly to EPX, never touches POS or Payment Service backend

---

### Step 8-9: EPX Processes Payment

**EPX receives** browser POST with card data + hidden fields

**EPX validates**:
1. Required fields present
2. Card data format valid
3. REDIRECT_URL whitelisted for merchant

**EPX processes**:
1. Authorizes transaction with card network
2. Generates AUTH_GUID (Financial BRIC token for refunds)
3. Determines result: approved (AUTH_RESP=00) or declined

---

### Step 10-11: EPX Redirects to Payment Service Callback

**EPX redirects** browser to REDIRECT_URL with POST data:

**Endpoint**: `POST /api/v1/payments/browser-post/callback`
**Component**: BrowserPostCallbackHandler.HandleCallback()
**File**: `internal/handlers/payment/browser_post_callback_handler.go:217-336`

**EPX POST Data**:
```
TRAN_NBR=45062844883
AUTH_GUID=ABC123XYZ (Financial BRIC)
AUTH_RESP=00 (00=approved, others=declined)
AUTH_CODE=OK1234
AMOUNT=99.99
CARD_TYPE=V (Visa)
USER_DATA_1=return_url=https://pos.example.com/payment-complete
... (30+ additional fields)
```

---

### Step 12: Payment Service Updates PENDING Transaction

**Process**:
1. Parse EPX response fields from POST data
2. **Look up PENDING transaction** by idempotency key (TRAN_NBR):
   ```go
   existingTx, err := h.dbAdapter.Queries().GetTransactionByIdempotencyKey(ctx,
       pgtype.Text{String: response.TranNbr, Valid: true})
   ```
3. **Update transaction** with EPX response:
   ```go
   UpdateTransaction(ctx, UpdateTransactionParams{
       ID:           existingTx.ID,
       Status:       "completed",  // or "failed" based on AUTH_RESP
       AuthGuid:     response.AuthGUID,
       AuthResp:     response.AuthResp,
       AuthCode:     response.AuthCode,
       AuthCardType: response.AuthCardType,
       AuthAvs:      response.AuthAVS,
       AuthCvv2:     response.AuthCVV2,
   })
   ```

**Result**: PENDING transaction updated to COMPLETED or FAILED

**Idempotency**: Duplicate callbacks reuse existing transaction (EPX sometimes sends multiple callbacks)

---

### Step 13-14: Payment Service Redirects to POS

**Extract return_url** from USER_DATA_1:
```go
func extractReturnURL(rawParams map[string]string) string {
    userData1 := rawParams["USER_DATA_1"]
    if strings.HasPrefix(userData1, "return_url=") {
        return userData1[11:] // Extract URL after prefix
    }
    return ""
}
```

**Redirect browser** to POS with transaction data:
```
https://pos.example.com/payment-complete?groupId=9b3d3df9-e37b-47ca-83f8-106b51b0ff50&transactionId=054edaac-3770-4222-ab50-e09b41051cc4&status=completed&amount=99.99&cardType=VISA&authCode=OK1234
```

**Query Parameters Returned**:
- `groupId`: Payment group ID (use to look up order in your database)
- `transactionId`: Specific transaction UUID
- `status`: "completed" or "failed"
- `amount`: Transaction amount (e.g., "99.99")
- `cardType`: Card brand ("VISA", "MASTERCARD", "AMEX", "DISCOVER")
- `authCode`: Authorization code from EPX (for receipt display)

**Method**: HTML with auto-redirect (meta refresh + JavaScript)

> **For complete integration examples**: See [BROWSER_POST_FRONTEND_GUIDE.md](./BROWSER_POST_FRONTEND_GUIDE.md) Section: "Return URL Query Parameters"

---

### Step 15-17: POS Renders Complete Receipt

**POS receives** browser redirect with query parameters

**POS process**:
1. Extract groupId from URL
2. Look up order by payment_group_id:
   ```sql
   SELECT * FROM orders WHERE payment_group_id = '9b3d3df9-...'
   ```
3. Update order status to PAID
4. Render complete receipt with:
   - Order items (sandwiches, drinks, etc.)
   - Cash payments (if any)
   - Tips, taxes, discounts
   - Transaction details (from query params)

**Why POS renders receipt**:
- Payment Service doesn't know about order items, cash, tips
- POS has complete order context
- Maintains clean architecture (Payment Service = Gateway Integration ONLY)

---

## Data Models

### Transaction (PENDING State)

Created at Step 2 (GetPaymentForm):

```sql
id:                UUID (primary key)
group_id:          UUID (for linking related transactions)
agent_id:          VARCHAR (merchant identifier)
customer_id:       VARCHAR (NULL for browser POST - guest checkout)
amount:            NUMERIC (99.99)
currency:          VARCHAR (USD)
status:            VARCHAR (pending)
type:              VARCHAR (charge)
payment_method_type: VARCHAR (credit_card)
payment_method_id: UUID (NULL for browser POST - not saved)
idempotency_key:   VARCHAR (TRAN_NBR for callback lookup)
metadata:          JSONB ({})
created_at:        TIMESTAMP
updated_at:        TIMESTAMP
```

### Transaction (COMPLETED State)

Updated at Step 12 (HandleCallback):

```sql
status:           VARCHAR (completed or failed)
auth_guid:        VARCHAR (ABC123XYZ - Financial BRIC for refunds)
auth_resp:        VARCHAR (00 = approved, 05 = declined, etc.)
auth_code:        VARCHAR (OK1234 - authorization code)
auth_resp_text:   VARCHAR (Approved or Declined reason)
auth_card_type:   VARCHAR (V=Visa, M=Mastercard, A=Amex, D=Discover)
auth_avs:         VARCHAR (Y=match, N=no match, etc.)
auth_cvv2:        VARCHAR (M=match, N=no match, etc.)
updated_at:       TIMESTAMP (callback time)
```

---

## Security

### 1. State Parameter Pattern

**Problem**: How to redirect back to POS without storing return_url in database?

**Solution**: Pass return_url through EPX via USER_DATA_1 field
- Payment Service never stores POS URLs
- Zero coupling between services
- EPX relays state back to Payment Service

### 2. Callback URL Whitelisting

**EPX requirement**: REDIRECT_URL must be whitelisted with EPX merchant setup

**Current whitelist**:
- Sandbox: `http://localhost:8081/api/v1/payments/browser-post/callback`
- Staging: `https://staging.payment-service.com/api/v1/payments/browser-post/callback`
- Production: `https://payment-service.com/api/v1/payments/browser-post/callback`

**Why**: Prevents attackers from redirecting payment results to malicious URLs

### 3. Idempotency

**Problem**: EPX may send duplicate callbacks (network retries, user refresh)

**Solution**: Look up by idempotency_key (TRAN_NBR)
- First callback: Updates PENDING→COMPLETED
- Subsequent callbacks: Returns existing transaction without re-processing

---

## Error Handling

### Abandoned Payments

**Scenario**: User closes browser after Step 7 (form submission) but before EPX callback

**Database State**: Transaction remains in PENDING status

**Benefit**: Complete audit trail of payment attempts
- Reports can show abandoned payment rate
- Customer service can investigate stuck transactions
- POS can show "Payment in progress" for pending orders

### Callback Never Arrives

**Causes**:
1. Network failure between EPX and Payment Service
2. Payment Service downtime during callback
3. User closes browser before EPX redirect completes

**Detection**: Cronjob finds transactions PENDING > 30 minutes

**Resolution**:
1. Query EPX Merchant Reporting API for transaction status
2. Update local transaction to match EPX status
3. Notify POS via webhook if status changed

### Duplicate Callbacks

**Scenario**: EPX sends callback multiple times (network retry, EPX bug)

**Handling**: Idempotency via TRAN_NBR lookup
- Update is idempotent (same fields set to same values)
- Response always contains correct transaction state
- POS receives consistent redirect

---

## EPX Endpoints

**Sandbox** (Testing):
```
https://secure.epxuap.com/browserpost
```

**Production**:
```
https://secure.epxnow.com/browserpost
```

---

## Key Differences from Old Flow

### Old Flow (Problematic)
1. GetPaymentForm: Returns form config (no transaction created)
2. User submits to EPX
3. Callback: **Creates** transaction for first time
4. Payment Service renders receipt

**Problems**:
- No audit trail for abandoned payments
- POS can't link order to payment until completion
- Payment Service renders incomplete receipts (no order context)

### New Flow (PENDING→UPDATE)
1. GetPaymentForm: **Creates PENDING transaction**, returns form config with IDs
2. POS stores group_id with order
3. User submits to EPX
4. Callback: **Updates** PENDING transaction to COMPLETED/FAILED
5. Payment Service redirects to POS
6. POS renders complete receipt

**Benefits**:
- Complete audit trail (all attempts tracked)
- POS can link order before completion
- POS renders receipts (has full context)
- Clean architecture (zero coupling)

---

## Testing

### Test Credentials (Sandbox)

```
EPX Endpoint: https://secure.epxuap.com/browserpost
CUST_NBR:     9001
MERCH_NBR:    900300
DBA_NBR:      2
TERMINAL_NBR: 77
```

### Test Cards

**Approved**:
- Visa: `4111111111111111`
- Mastercard: `5499740000000057`
- CVV: `123`, Expiry: `12/2025`

**Declined**:
- Visa: `4000000000000002`

### Test Flow

1. Call GetPaymentForm API
2. Extract transactionId and groupId
3. Verify PENDING transaction exists in database
4. Render HTML form with test card
5. Submit to EPX
6. EPX redirects to callback
7. Verify transaction updated to COMPLETED
8. Verify redirect to return_url with query params

---

## Summary

**Browser POST = PCI-Compliant Payment with PENDING→UPDATE Audit Trail**

**Flow**: POS → Payment Service (PENDING) → Frontend → EPX → Payment Service (UPDATE) → POS (Receipt)

**Key Components**:
- GetPaymentForm: Creates PENDING, returns form config + IDs
- HTML Form: Submits card data directly to EPX
- EPX: Processes payment, redirects to callback
- HandleCallback: Updates PENDING→COMPLETED/FAILED, redirects to POS
- POS: Renders complete receipt

**Architecture Benefits**:
- ✅ Complete audit trail (all attempts tracked)
- ✅ PCI compliant (card data never touches backend)
- ✅ Clean separation (Payment Service = Gateway only)
- ✅ Scalable (POS stores own mappings)
- ✅ Zero coupling (state parameter pattern)

---

**Last Updated**: 2025-11-11
**Related Docs**:
- [Browser POST Frontend Guide](./BROWSER_POST_FRONTEND_GUIDE.md) - Frontend integration
- [Credit Card Browser POST Dataflow](./CREDIT_CARD_BROWSER_POST_DATAFLOW.md) - Use case walkthrough
