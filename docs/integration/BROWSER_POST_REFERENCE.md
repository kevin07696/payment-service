# Browser Post Reference

**Target Audience:** Developers implementing EPX Browser Post payment forms
**Topic:** Complete reference for building Browser Post payment forms
**Goal:** Understand how to construct and submit Browser Post forms with TAC authentication

---

## Overview

Browser Post is EPX's PCI-compliant payment method where card data is submitted directly from the user's browser to EPX, never touching your backend servers. This document provides complete examples and field reference for implementing Browser Post forms.

**Flow:**
1. Backend calls payment service to get form configuration (includes TAC token)
2. Frontend builds HTML form with card input fields
3. User enters card details
4. Form submits directly to EPX
5. EPX processes payment and redirects back to your callback URL

**Security:** Card data never touches your servers, reducing PCI compliance scope.

---

## Table of Contents

1. [Getting Form Configuration](#getting-form-configuration)
2. [Complete HTML Form Example](#complete-html-form-example)
3. [JavaScript Example](#javascript-example)
4. [Field Reference](#field-reference)
5. [Transaction Types](#transaction-types)
6. [Test Cards](#test-cards)
7. [Common Issues](#common-issues)

---

## Getting Form Configuration

### Step 1: Backend Calls Payment Service

Your backend must first request form configuration from the payment service:

**Endpoint:** `GET /api/v1/payments/browser-post/form`

**Query Parameters:**
- `transaction_id` (UUID) - Frontend-generated unique transaction ID
- `merchant_id` (UUID) - Your merchant identifier
- `amount` (string) - Payment amount as decimal (e.g., "99.99")
- `transaction_type` (string) - "SALE", "AUTH", or "STORAGE"
- `return_url` (string) - URL where EPX will redirect after processing

**Example Request:**
```javascript
// Backend (Node.js example)
const transactionId = generateUUID();
const formConfigUrl = `http://localhost:8081/api/v1/payments/browser-post/form?` +
  `transaction_id=${transactionId}&` +
  `merchant_id=${merchantId}&` +
  `amount=99.99&` +
  `transaction_type=SALE&` +
  `return_url=${encodeURIComponent('https://yourapp.com/payment/callback')}`;

const response = await fetch(formConfigUrl);
const formConfig = await response.json();
```

**Response:**
```json
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440000",
  "epxTranNbr": "1234567890",
  "tac": "abc123xyz456",
  "expiresAt": 1642445100,
  "postURL": "https://services.epxuap.com/browserpost/",
  "custNbr": "9001",
  "merchNbr": "900300",
  "dbaName": "2",
  "terminalNbr": "77",
  "redirectURL": "https://yourapp.com/payment/callback?transaction_id=550e8400...",
  "merchantId": "01234567-89ab-cdef-0123-456789abcdef",
  "merchantName": "ACME Corporation"
}
```

**Important:** TAC tokens expire in 15 minutes. Generate a new one for each payment attempt.

### Step 2: Pass Configuration to Frontend

Send the form configuration to your frontend (via API response, render in template, etc.):

```javascript
// Frontend receives config
const formConfig = await fetchFormConfig(transactionId, amount);
```

---

## Complete HTML Form Example

### Basic Payment Form

This example shows a complete, working Browser Post form:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Payment</title>
  <style>
    .payment-form { max-width: 400px; margin: 50px auto; }
    .form-group { margin-bottom: 15px; }
    label { display: block; margin-bottom: 5px; font-weight: bold; }
    input { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
    button { width: 100%; padding: 10px; background: #4CAF50; color: white; border: none; border-radius: 4px; cursor: pointer; }
    button:hover { background: #45a049; }
  </style>
</head>
<body>
  <div class="payment-form">
    <h2>Payment Details</h2>

    <!-- This form submits directly to EPX -->
    <form id="payment-form" action="https://services.epxuap.com/browserpost/" method="POST">

      <!-- EPX Authentication & Merchant Credentials (hidden) -->
      <input type="hidden" name="TAC" value="abc123xyz456">
      <input type="hidden" name="CUST_NBR" value="9001">
      <input type="hidden" name="MERCH_NBR" value="900300">
      <input type="hidden" name="DBA_NBR" value="2">
      <input type="hidden" name="TERMINAL_NBR" value="77">

      <!-- Transaction Details (hidden) -->
      <input type="hidden" name="TRAN_NBR" value="1234567890">
      <input type="hidden" name="TRAN_GROUP" value="U"> <!-- U=SALE, A=AUTH -->
      <input type="hidden" name="AMOUNT" value="99.99">
      <input type="hidden" name="INDUSTRY_TYPE" value="E"> <!-- E=E-commerce -->

      <!-- Callback URL (hidden) -->
      <input type="hidden" name="REDIRECT_URL" value="https://yourapp.com/payment/callback">

      <!-- Custom Data (optional, echoed back in callback) -->
      <input type="hidden" name="USER_DATA_1" value="customer_id=456">
      <input type="hidden" name="USER_DATA_2" value="order_id=ORDER-12345">

      <!-- Card Details (user enters these) -->
      <div class="form-group">
        <label for="card_number">Card Number</label>
        <input type="text" id="card_number" name="CARD_NBR" placeholder="4111111111111111" required maxlength="16">
      </div>

      <div class="form-group">
        <label for="exp_date">Expiration (MMYY)</label>
        <input type="text" id="exp_date" name="EXP_DATE" placeholder="1225" required maxlength="4">
      </div>

      <div class="form-group">
        <label for="cvv">CVV</label>
        <input type="text" id="cvv" name="CVV" placeholder="123" required maxlength="4">
      </div>

      <div class="form-group">
        <label for="zip">Billing ZIP Code</label>
        <input type="text" id="zip" name="AVS_ZIP" placeholder="12345" required maxlength="10">
      </div>

      <button type="submit">Pay $99.99</button>
    </form>
  </div>
</body>
</html>
```

**What Happens:**
1. User fills out card details
2. User clicks "Pay $99.99"
3. Browser submits form directly to EPX (`https://services.epxuap.com/browserpost/`)
4. EPX processes payment
5. EPX redirects browser to your `REDIRECT_URL` with results

---

## JavaScript Example

### Dynamic Form Population

Most applications generate forms dynamically using the configuration from the payment service:

```javascript
async function createPaymentForm(merchantId, amount, transactionType) {
  // Step 1: Get form configuration from payment service
  const transactionId = generateUUID();
  const returnUrl = `${window.location.origin}/payment/callback`;

  const formConfigUrl = `/api/v1/payments/browser-post/form?` +
    `transaction_id=${transactionId}&` +
    `merchant_id=${merchantId}&` +
    `amount=${amount}&` +
    `transaction_type=${transactionType}&` +
    `return_url=${encodeURIComponent(returnUrl)}`;

  const response = await fetch(formConfigUrl);
  const config = await response.json();

  // Step 2: Create form element
  const form = document.createElement('form');
  form.method = 'POST';
  form.action = config.postURL; // EPX endpoint

  // Step 3: Add hidden fields
  const hiddenFields = {
    'TAC': config.tac,
    'CUST_NBR': config.custNbr,
    'MERCH_NBR': config.merchNbr,
    'DBA_NBR': config.dbaName,
    'TERMINAL_NBR': config.terminalNbr,
    'TRAN_NBR': config.epxTranNbr,
    'TRAN_GROUP': transactionType === 'AUTH' ? 'A' : 'U',
    'AMOUNT': amount,
    'INDUSTRY_TYPE': 'E',
    'REDIRECT_URL': config.redirectURL,
    'USER_DATA_1': merchantId,
    'USER_DATA_2': transactionId,
  };

  Object.entries(hiddenFields).forEach(([name, value]) => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = name;
    input.value = value;
    form.appendChild(input);
  });

  // Step 4: Add card input fields (or get from existing form)
  form.innerHTML += `
    <label>Card Number: <input type="text" name="CARD_NBR" required maxlength="16"></label>
    <label>Expiration (MMYY): <input type="text" name="EXP_DATE" required maxlength="4"></label>
    <label>CVV: <input type="text" name="CVV" required maxlength="4"></label>
    <label>ZIP: <input type="text" name="AVS_ZIP" required maxlength="10"></label>
    <button type="submit">Pay $${amount}</button>
  `;

  // Step 5: Add to page and submit
  document.body.appendChild(form);
  return form;
}

// Usage
const form = await createPaymentForm('merchant-id-uuid', '99.99', 'SALE');

function generateUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0;
    const v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}
```

---

## Field Reference

### Required Hidden Fields

These fields must be included in every Browser Post form:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `TAC` | string | Transaction Authentication Code from payment service | `"abc123xyz456"` |
| `CUST_NBR` | string | EPX customer number | `"9001"` |
| `MERCH_NBR` | string | EPX merchant number | `"900300"` |
| `DBA_NBR` | string | EPX DBA number | `"2"` |
| `TERMINAL_NBR` | string | EPX terminal number | `"77"` |
| `TRAN_NBR` | string | Transaction number (numeric, max 10 digits) | `"1234567890"` |
| `TRAN_GROUP` | string | Transaction type: `U` (SALE) or `A` (AUTH) | `"U"` |
| `AMOUNT` | string | Transaction amount (decimal) | `"99.99"` |
| `REDIRECT_URL` | string | URL where EPX redirects after processing | `"https://yourapp.com/callback"` |

### Required Card Fields (User Input)

These fields are filled by the user:

| Field | Type | Description | Example | Validation |
|-------|------|-------------|---------|------------|
| `CARD_NBR` | string | Credit card number (no spaces) | `"4111111111111111"` | 13-16 digits |
| `EXP_DATE` | string | Expiration date (MMYY format) | `"1225"` | 4 digits |
| `CVV` | string | Card verification value | `"123"` | 3-4 digits |
| `AVS_ZIP` | string | Billing ZIP code (AVS verification) | `"12345"` | 5-10 chars |

### Optional Fields

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `INDUSTRY_TYPE` | string | Industry type (E=E-commerce, R=Retail) | `"E"` |
| `USER_DATA_1` | string | Custom data (echoed back in callback) | `"customer_id=456"` |
| `USER_DATA_2` | string | Custom data (echoed back in callback) | `"order_id=12345"` |
| `USER_DATA_3` | string | Custom data (echoed back in callback) | `"campaign=summer"` |
| `AVS_STREET` | string | Billing street address (AVS verification) | `"123 Main St"` |

### BRIC Storage Fields (Save Card for Future Use)

To store a card as a BRIC token for future payments:

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `TRAN_GROUP` | string | Must be `"S"` for storage | `"S"` |
| `AMOUNT` | string | Must be `"0.00"` for storage-only | `"0.00"` |

**Note:** Storage-only transactions create a BRIC token without charging the card.

---

## Transaction Types

### SALE (Auth + Capture)

Authorizes and captures funds in a single step:

```javascript
{
  "TRAN_GROUP": "U",  // U = SALE
  "AMOUNT": "99.99"
}
```

**Use Case:** Standard e-commerce checkout, immediate payment

### AUTH (Authorization Only)

Holds funds but doesn't capture (capture later via Server Post):

```javascript
{
  "TRAN_GROUP": "A",  // A = AUTH
  "AMOUNT": "99.99"
}
```

**Use Case:** Pre-authorization (hotels, car rentals), delayed capture

**Follow-up:** Use Server Post `Capture` RPC to capture funds later

### STORAGE (Save Card Only)

Stores card as BRIC token without charging:

```javascript
{
  "TRAN_GROUP": "S",  // S = STORAGE
  "AMOUNT": "0.00"    // Must be 0.00
}
```

**Use Case:** Save payment method for future subscriptions/payments

**Follow-up:** Use BRIC token with Server Post APIs for future charges

---

## Test Cards

### EPX Sandbox Test Cards

**Approval Cards:**

| Card Number | Brand | CVV | Exp | Result |
|-------------|-------|-----|-----|--------|
| `4111111111111111` | Visa | 123 | 12/25 | ✅ Approved |
| `4788250000028291` | Visa | 123 | 12/25 | ✅ Approved |
| `5454545454545454` | Mastercard | 123 | 12/25 | ✅ Approved |

**Decline/Error Cards:**

Use approved card numbers with specific amounts to trigger error codes:

| Amount | Response Code | Meaning |
|--------|--------------|---------|
| `$1.05` | 05 | Do Not Honor |
| `$1.20` | 51 | Insufficient Funds |
| `$1.54` | 54 | Expired Card |
| `$1.91` | 91 | Issuer Unavailable |

**Example:**
```javascript
// To test "Insufficient Funds" decline:
{
  "CARD_NBR": "4111111111111111",
  "AMOUNT": "1.20",  // Triggers code 51
  "CVV": "123",
  "EXP_DATE": "1225"
}
```

---

## Common Issues

### Issue: "TAC validation failed" (EPX Code 58)

**Cause:** Invalid or expired TAC token

**Solutions:**
- TAC expires in 15 minutes - generate new one if expired
- Verify MAC_SECRET matches EPX account
- Check merchant credentials (CUST_NBR, MERCH_NBR, etc.) are correct
- Ensure amount, tran_nbr match what was sent in Key Exchange request

### Issue: Callback not received

**Cause:** EPX cannot reach your callback URL

**Solutions:**
- Verify `REDIRECT_URL` is publicly accessible (use ngrok for local dev)
- Check HTTPS/TLS certificate is valid
- Verify firewall allows EPX IPs
- Check server logs for incoming POST requests

### Issue: Form submits but nothing happens

**Cause:** EPX endpoint URL incorrect

**Solutions:**
- Verify form `action` is correct EPX endpoint:
  - Sandbox: `https://services.epxuap.com/browserpost/`
  - Production: `https://secure.epxuap.com/browserpost/`
- Check browser console for CORS errors
- Ensure form method is `POST`

### Issue: Card declined unexpectedly

**Cause:** AVS (Address Verification System) mismatch

**Solutions:**
- For test cards, use ZIP: `12345`
- Ensure AVS_ZIP matches card billing address
- Check EPX merchant settings for AVS requirements

### Issue: "Amount mismatch" error

**Cause:** Amount in form doesn't match TAC request

**Solutions:**
- Ensure `AMOUNT` field matches the amount used in Key Exchange request
- TAC is tied to specific amount - can't change after generation
- Generate new TAC if amount changes

---

## Best Practices

1. **Always use HTTPS** - Even in development (use ngrok)
2. **Validate card data client-side** - Before submitting to EPX (Luhn check, expiration date)
3. **Show loading state** - Disable submit button while processing
4. **Handle popup blockers** - If using `target="_blank"`, ensure user interaction triggered it
5. **Implement timeouts** - TAC expires in 15 minutes
6. **Never log card data** - PCI compliance violation
7. **Use USER_DATA fields** - To track transactions in your system
8. **Test all flows** - Approval, decline, timeout, callback failure

---

## Next Steps

- **[Integration Guide](INTEGRATION_GUIDE.md)** - Step-by-step integration walkthrough
- **[API Specs](API_SPECS.md)** - Complete API reference
- **[DATAFLOW](DATAFLOW.md)** - Detailed payment flow diagrams
- **[FAQ](wiki-templates/FAQ.md)** - Common questions answered

---

**Questions?** Open an issue on [GitHub](https://github.com/kevin07696/payment-service/issues) or check the [FAQ](wiki-templates/FAQ.md).
