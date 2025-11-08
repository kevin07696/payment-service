# Single Credit Card Transaction - Browser Post API Dataflow

**Date**: 2025-11-03
**Transaction Type**: One-time credit card payment (guest checkout)
**API**: EPX Browser Post API
**Use Case**: PCI-compliant card payment where card data never touches merchant backend

---

## Overview

This document describes the complete dataflow for processing a single credit card transaction using the EPX Browser Post API. This flow is used when a customer makes a one-time payment without saving their card for future use.

---

## Complete Transaction Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│              SINGLE CREDIT CARD TRANSACTION - BROWSER POST          │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Customer  │         │   Merchant  │         │     EPX     │
│   Browser   │         │   Backend   │         │   Gateway   │
└──────┬──────┘         └──────┬──────┘         └──────┬──────┘
       │                       │                       │
       │  1. Checkout          │                       │
       ├──────────────────────>│                       │
       │                       │                       │
       │                       │  2. Generate TAC      │
       │                       │      (merchant-specific)
       │                       │                       │
       │  3. Payment Form      │                       │
       │     with TAC          │                       │
       │<──────────────────────┤                       │
       │                       │                       │
       │  4. Enter Card Info   │                       │
       │     + Submit          │                       │
       │                       │                       │
       │  5. POST to EPX       │                       │
       │     (card data)       │                       │
       ├───────────────────────────────────────────────>│
       │                       │                       │
       │                       │                       │  6. Process
       │                       │                       │     Payment
       │                       │                       │
       │  7. Redirect to       │                       │
       │     Callback URL      │                       │
       │<───────────────────────────────────────────────┤
       │                       │                       │
       │  8. POST transaction  │                       │
       │     results           │                       │
       ├──────────────────────>│                       │
       │                       │                       │
       │                       │  9. Store Transaction │
       │                       │     + AUTH_GUID       │
       │                       │                       │
       │  10. Receipt Page     │                       │
       │<──────────────────────┤                       │
       │                       │                       │
```

---

## Detailed Step-by-Step Flow

### Step 1: Customer Initiates Checkout

**Actor**: Customer Browser
**Action**: User clicks "Checkout" or "Pay Now" on merchant website

**Data**:
- Cart items
- Total amount: `$99.99`
- Shipping/billing address (optional)

**Result**: Merchant backend receives checkout request

---

### Step 2: Merchant Generates TAC Token

**Actor**: Merchant Backend
**Component**: TAC generation system (merchant-specific implementation)
**File**: N/A (merchant's proprietary system)

**Input**:
- Amount: `"99.99"`
- Transaction Number: `"TXN-2025-001"`
- Transaction Group: `"SALE"`
- Redirect URL: `"https://merchant.com/api/v1/payments/browser-post/callback"`
- MAC (Merchant Authorization Code): Merchant's EPX credential

**Process**:
1. Merchant generates unique TRAN_NBR for this transaction
2. Merchant creates TAC request with all required fields
3. TAC system encrypts the data into a TAC token
4. TAC expires in 4 hours

**Output**:
```
TAC Token: "FEU86Eo5/S1CZwZemrks4P7w1IpJSFIi7qTQ+sNYgqi0jAAyC+7GDOq5wojO94pNE7gQDCjClXMgo+Gez9GBgSAQXaF/rX7J"
```

---

### Step 3: Render Payment Form to Customer

**Actor**: Merchant Backend → Customer Browser
**Component**: BrowserPostAdapter.BuildFormData()
**File**: `internal/adapters/epx/browser_post_adapter.go:62`

**Input**:
- TAC token (from Step 2)
- Amount: `"99.99"`
- TRAN_NBR: `"TXN-2025-001"`
- TRAN_GROUP: `"SALE"`
- REDIRECT_URL: `"https://merchant.com/api/v1/payments/browser-post/callback"`

**HTML Form** (rendered to customer):
```html
<form method="POST" action="https://epxnow.com/epx/browser_post_sandbox">
  <!-- Hidden fields from merchant -->
  <input type="hidden" name="TAC" value="FEU86Eo5..." />
  <input type="hidden" name="TRAN_CODE" value="SALE" />
  <input type="hidden" name="INDUSTRY_TYPE" value="E" />
  <input type="hidden" name="TRAN_NBR" value="TXN-2025-001" />
  <input type="hidden" name="AMOUNT" value="99.99" />
  <input type="hidden" name="TRAN_GROUP" value="SALE" />
  <input type="hidden" name="REDIRECT_URL" value="https://merchant.com/api/v1/payments/browser-post/callback" />
  <input type="hidden" name="CUST_NBR" value="123456" />
  <input type="hidden" name="MERCH_NBR" value="789012" />
  <input type="hidden" name="DBA_NBR" value="1" />
  <input type="hidden" name="TERMINAL_NBR" value="1" />

  <!-- Customer enters card data -->
  <label>Card Number:</label>
  <input type="text" name="CARD_NBR" placeholder="4111111111111111" />

  <label>Expiration:</label>
  <input type="text" name="EXP_MONTH" placeholder="12" />
  <input type="text" name="EXP_YEAR" placeholder="2025" />

  <label>CVV:</label>
  <input type="text" name="CVV" placeholder="123" />

  <button type="submit">Pay $99.99</button>
</form>
```

**Result**: Customer sees payment form in their browser

---

### Step 4: Customer Enters Card Information

**Actor**: Customer Browser
**Action**: User enters payment details

**Entered Data**:
- Card Number: `4111111111111111` (Visa test card)
- Expiration: `12/2025`
- CVV: `123`
- Cardholder Name: `John Doe` (optional)

**Action**: Customer clicks "Pay $99.99"

**Result**: Browser POSTs all form data directly to EPX (NOT merchant backend)

**PCI Compliance Note**: Card data never touches merchant servers

---

### Step 5: Browser POSTs to EPX

**Actor**: Customer Browser → EPX Gateway
**Destination**: `https://epxnow.com/epx/browser_post_sandbox`
**Method**: POST

**Posted Data**:
```
TAC=FEU86Eo5...
TRAN_CODE=SALE
INDUSTRY_TYPE=E
TRAN_NBR=TXN-2025-001
AMOUNT=99.99
TRAN_GROUP=SALE
REDIRECT_URL=https://merchant.com/api/v1/payments/browser-post/callback
CUST_NBR=123456
MERCH_NBR=789012
DBA_NBR=1
TERMINAL_NBR=1
CARD_NBR=4111111111111111
EXP_MONTH=12
EXP_YEAR=2025
CVV=123
```

**Result**: EPX receives transaction request

---

### Step 6: EPX Processes Payment

**Actor**: EPX Payment Gateway
**Component**: Browser Post API + Payment Gateway

**Validation Phase**:
1. Decrypt TAC token
2. Verify TAC not expired (< 4 hours old)
3. Compare TAC fields with POSTed fields (anti-tampering)
4. Validate all field formats with regex
5. Verify REDIRECT_URL is authorized for merchant

**Payment Processing Phase**:
1. Send card data to card network (Visa in this case)
2. Perform AVS (Address Verification System) check
3. Perform CVV verification
4. Get authorization from issuing bank
5. Generate AUTH_GUID (Financial BRIC token)
6. Record transaction in EPX systems

**Output**:
```
AUTH_GUID: "09MBFZ3PV6BTRETVQK2"
AUTH_RESP: "00" (approved)
AUTH_CODE: "001084" (bank auth code)
AUTH_RESP_TEXT: "EXACT MATCH"
AUTH_CARD_TYPE: "V" (Visa)
AUTH_AVS: "Y" (address match)
AUTH_CVV2: "M" (CVV match)
TRAN_NBR: "TXN-2025-001" (echo back)
AMOUNT: "99.99" (echo back)
CARD_NBR: "XXXXXXXXXXXX1111" (masked)
```

**Financial BRIC Token (AUTH_GUID)**:
- Type: Financial BRIC
- Lifetime: 13-24 months
- Can be used for: Refunds, voids, chargebacks, recurring payments
- Can be converted to: Storage BRIC (never expires) for saved payment methods

---

### Step 7: EPX Redirects to Callback URL

**Actor**: EPX Gateway → Customer Browser
**Method**: POST-REDIRECT-GET (PRG) Pattern

**Process**:
1. EPX creates response page with JavaScript auto-POST form
2. Browser loads response page
3. JavaScript immediately POSTs to REDIRECT_URL
4. Prevents duplicate submissions on browser "Back" or "Refresh"

**Redirect**: Browser navigates to merchant's callback endpoint

---

### Step 8: Browser POSTs Transaction Results to Merchant

**Actor**: Customer Browser → Merchant Backend
**Destination**: `https://merchant.com/api/v1/payments/browser-post/callback`
**Method**: POST
**Port**: 8081 (HTTP server)

**Posted Data** (from EPX):
```
AUTH_GUID=09MBFZ3PV6BTRETVQK2
AUTH_RESP=00
AUTH_CODE=001084
AUTH_RESP_TEXT=EXACT MATCH
AUTH_CARD_TYPE=V
AUTH_AVS=Y
AUTH_CVV2=M
TRAN_NBR=TXN-2025-001
AMOUNT=99.99
TRAN_GROUP=SALE
CARD_NBR=XXXXXXXXXXXX1111
EXP_MONTH=12
EXP_YEAR=2025
CUST_NBR=123456
MERCH_NBR=789012
DBA_NBR=1
TERMINAL_NBR=1
LOCAL_DATE=110325
LOCAL_TIME=143022
BATCH_ID=20251103
```

**Result**: Merchant backend receives transaction results

---

### Step 9: Merchant Stores Transaction

**Actor**: Merchant Backend
**Component**: BrowserPostCallbackHandler
**File**: `internal/handlers/payment/browser_post_callback_handler.go:45`

**Sub-Step 9a: Parse Response**

**Method**: `browserPost.ParseRedirectResponse(params)`
**File**: `internal/adapters/epx/browser_post_adapter.go:107`

**Process**:
1. Extract all form parameters
2. Validate AUTH_GUID exists
3. Validate AUTH_RESP exists
4. Determine if approved: `AUTH_RESP == "00"`
5. Parse amount, card type, verification fields
6. Store raw parameters for debugging

**Sub-Step 9b: Check for Duplicates**

**Method**: `dbAdapter.GetTransactionByIdempotencyKey("TXN-2025-001")`

**Why**: EPX PRG pattern may send same response multiple times if user hits "Back" or "Refresh"

**Process**:
```sql
SELECT * FROM transactions WHERE idempotency_key = 'TXN-2025-001';
```

**Result**: No existing transaction found, proceed to storage

**Sub-Step 9c: Store in Database**

**Method**: `storeTransaction(ctx, response)`
**Table**: `transactions`

**SQL Insert**:
```sql
INSERT INTO transactions (
  id,
  group_id,
  agent_id,
  customer_id,           -- NULL (guest checkout)
  amount,
  currency,
  status,
  type,
  payment_method_type,
  payment_method_id,     -- NULL (not saved)
  auth_guid,             -- CRITICAL: For refunds/recurring
  auth_resp,
  auth_code,
  auth_resp_text,
  auth_card_type,
  auth_avs,
  auth_cvv2,
  idempotency_key,
  metadata,
  created_at,
  updated_at
) VALUES (
  '550e8400-e29b-41d4-a716-446655440000',  -- UUID
  '660e8400-e29b-41d4-a716-446655440000',  -- Group UUID
  'AGT-123456-789012-1-1',                 -- From CUST_NBR-MERCH_NBR-DBA_NBR-TERMINAL_NBR
  NULL,                                     -- Guest checkout
  99.99,
  'USD',
  'completed',
  'charge',
  'credit_card',
  NULL,                                     -- Not saved
  '09MBFZ3PV6BTRETVQK2',                   -- Financial BRIC
  '00',
  '001084',
  'EXACT MATCH',
  'V',
  'Y',
  'M',
  'TXN-2025-001',                          -- Idempotency key
  '{}',
  NOW(),
  NOW()
);
```

**Why Store AUTH_GUID for Guest Checkout?**
1. **Refunds**: Customer may request refund later
2. **Voids**: Cancel before settlement (same day)
3. **Chargebacks**: Dispute resolution requires AUTH_GUID
4. **Reconciliation**: Match with EPX reporting
5. **Future Use**: Can be used for recurring payments if customer signs up later

---

### Step 10: Render Receipt Page to Customer

**Actor**: Merchant Backend → Customer Browser
**Component**: `renderReceiptPage(w, response, txID)`
**File**: `browser_post_callback_handler.go:218`

**Success Page HTML** (AUTH_RESP == "00"):
```html
<!DOCTYPE html>
<html>
<head>
    <title>Payment Successful</title>
    <style>
        .success { color: green; font-size: 24px; }
        .details { margin: 20px; }
    </style>
</head>
<body>
    <div class="success">✓ Payment Successful</div>

    <div class="details">
        <h2>Transaction Details</h2>
        <p><strong>Amount:</strong> $99.99 USD</p>
        <p><strong>Card:</strong> Visa ending in 1111</p>
        <p><strong>Authorization Code:</strong> 001084</p>
        <p><strong>Transaction ID:</strong> 550e8400-e29b-41d4-a716-446655440000</p>
        <p><strong>Reference Number:</strong> TXN-2025-001</p>
        <p><strong>Date:</strong> 2025-11-03 14:30:22</p>

        <p>Thank you for your payment!</p>

        <a href="/">Return to Home</a>
    </div>
</body>
</html>
```

**Result**: Customer sees success receipt in their browser

---

## Data Summary

### Initial Request
```
Amount: $99.99
Transaction: One-time credit card payment
Customer Type: Guest (no account)
```

### Final Result
```
Status: Completed
Transaction ID: 550e8400-e29b-41d4-a716-446655440000
Financial BRIC: 09MBFZ3PV6BTRETVQK2
Lifetime: 13-24 months
Can be used for: Refunds, voids, recurring payments
Can be converted to: Storage BRIC for saved payment methods
```

---

## Security & Compliance

### PCI Compliance
- ✅ Card data NEVER touches merchant backend
- ✅ Card data POSTed directly from browser to EPX
- ✅ Merchant only receives tokenized AUTH_GUID
- ✅ Merchant stores only last 4 digits (masked)

### Data Protection
- ✅ TAC token encrypted (4-hour expiration)
- ✅ HTTPS required for all communications
- ✅ Anti-tampering: TAC validation
- ✅ Idempotency: Duplicate detection via TRAN_NBR

### Fraud Prevention
- ✅ AVS verification (Address match)
- ✅ CVV verification (CVV match)
- ✅ Card network authorization required

---

## What Happens Next?

### For Guest Checkout (Current Implementation):
1. Transaction stored with AUTH_GUID
2. Customer can request refund using AUTH_GUID
3. Merchant can void transaction (same day) using AUTH_GUID
4. AUTH_GUID expires in 13-24 months

### For Saved Payment Methods (Future Enhancement):
If customer creates account and wants to save payment method:

1. **Convert Financial BRIC to Storage BRIC**:
   - Call EPX API to convert AUTH_GUID
   - Storage BRIC never expires
   - One-time conversion fee

2. **Store in customer_payment_methods**:
   ```sql
   INSERT INTO customer_payment_methods (
     id,
     customer_id,
     agent_id,
     payment_type,
     bric_token,           -- Storage BRIC
     card_type,
     last_four,
     exp_month,
     exp_year,
     is_default,
     created_at,
     updated_at
   ) VALUES (
     UUID(),
     'CUST-12345',
     'AGT-123456-789012-1-1',
     'credit_card',
     'STORAGE-BRIC-TOKEN',
     'V',
     '1111',
     12,
     2025,
     true,
     NOW(),
     NOW()
   );
   ```

3. **Update transaction record**:
   ```sql
   UPDATE transactions
   SET payment_method_id = 'new-payment-method-uuid',
       customer_id = 'CUST-12345'
   WHERE id = '550e8400-e29b-41d4-a716-446655440000';
   ```

4. **Use for recurring payments**:
   - Storage BRIC can be used with Server Post API
   - No card data needed for future charges
   - Customer experiences one-click checkout

---

## Implementation Status

### ✅ Completed
- TAC generation (merchant-specific)
- Form rendering with TAC
- Browser POST to EPX
- EPX payment processing
- Callback endpoint
- Response parsing
- Duplicate detection
- Transaction storage with Financial BRIC
- Receipt page rendering

### ⏳ Pending
- Convert Financial BRIC to Storage BRIC
- Link to customer_payment_methods table
- Use Storage BRIC for recurring payments
- One-click checkout with saved cards

---

**Document Version**: 1.0
**Last Updated**: 2025-11-03
**Status**: ✅ ACTIVE
