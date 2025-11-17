# Integration Guide

**Target Audience:** Developers integrating the payment service into their applications
**Topic:** Step-by-step guide to register, authenticate, and integrate payment processing
**Goal:** Successfully process your first payment end-to-end

---

## Overview

This guide walks you through:
1. **Merchant Registration** - Setting up your merchant account
2. **Authentication Setup** - Getting API access tokens
3. **Payment Integration** - Implementing payment flows
4. **Testing** - Verifying your integration works
5. **Production Deployment** - Going live

**Time to First Payment:** ~30 minutes

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Step 1: Register Your Merchant](#step-1-register-your-merchant)
3. [Step 2: Configure Authentication](#step-2-configure-authentication)
4. [Step 3: Integrate Browser Post (Frontend)](#step-3-integrate-browser-post-frontend)
5. [Step 4: Handle Payment Callbacks](#step-4-handle-payment-callbacks)
6. [Step 5: Integrate Server APIs (Backend)](#step-5-integrate-server-apis-backend)
7. [Step 6: Test Your Integration](#step-6-test-your-integration)
8. [Step 7: Production Checklist](#step-7-production-checklist)
9. [Common Integration Patterns](#common-integration-patterns)
10. [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before you begin, ensure you have:

- ✅ **EPX Merchant Account** - See [EPX Credentials Guide](EPX-Credentials.md)
- ✅ **Payment Service Running** - See [Quick Start](../docs/wiki-templates/Quick-Start.md)
- ✅ **Development Environment** - Node.js/Python/Go/etc. for your application
- ✅ **HTTPS/TLS** - Required for production (use ngrok for local dev)

---

## Step 1: Register Your Merchant

### What is a Merchant?

A **merchant** represents your business/organization in the payment service. Each merchant has:
- Unique identifier (`merchant_id`)
- EPX credentials (for payment processing)
- Isolated data (multi-tenant architecture)
- Authentication keys (for API access)

### Register via API

**Endpoint:** `POST /api/v1/merchants`

**Request:**
```bash
curl -X POST http://localhost:8081/api/v1/merchants \
  -H "Content-Type: application/json" \
  -d '{
    "slug": "my-company",
    "name": "My Company Inc",
    "epx_credentials": {
      "cust_nbr": "9001",
      "merch_nbr": "900300",
      "dba_nbr": "2",
      "terminal_nbr": "77"
    },
    "mac_secret_path": "secrets/epx/staging/mac_secret",
    "environment": "sandbox"
  }'
```

**Response:**
```json
{
  "merchant_id": "01234567-89ab-cdef-0123-456789abcdef",
  "slug": "my-company",
  "name": "My Company Inc",
  "is_active": true,
  "environment": "sandbox",
  "created_at": "2025-01-17T12:00:00Z"
}
```

**Save the `merchant_id`** - You'll need it for all API requests.

### Register via SQL (Direct Database Access)

If you have direct database access:

```sql
INSERT INTO merchants (
    slug,
    name,
    cust_nbr,
    merch_nbr,
    dba_nbr,
    terminal_nbr,
    mac_secret_path,
    environment,
    is_active
) VALUES (
    'my-company',
    'My Company Inc',
    '9001',
    '900300',
    '2',
    '77',
    'secrets/epx/staging/mac_secret',
    'sandbox',
    true
);
```

---

## Step 2: Configure Authentication

All API requests require JWT authentication.

### Option A: Token-Based Auth (Recommended)

**Generate a JWT Token:**

```bash
# Using the payment service's token generation endpoint
curl -X POST http://localhost:8081/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{
    "merchant_id": "01234567-89ab-cdef-0123-456789abcdef",
    "api_key": "your-api-key-here"
  }'
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-01-18T12:00:00Z"
}
```

**Use the token in all requests:**
```bash
curl -X POST http://localhost:8081/api/v1/payments/authorize \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -H "Content-Type: application/json" \
  -d '...'
```

### Option B: API Key (If Supported)

Some deployments support API key authentication:

```bash
curl -X POST http://localhost:8081/api/v1/payments/authorize \
  -H "X-API-Key: your-api-key-here" \
  -H "Content-Type: application/json" \
  -d '...'
```

See [AUTH.md](AUTH.md) for complete authentication documentation.

---

## Step 3: Integrate Browser Post (Frontend)

**Browser Post** is EPX's PCI-compliant method where card data goes directly from the user's browser to EPX (never touching your backend).

### 3.1: Generate TAC Token (Backend)

First, your backend must request a TAC (Transaction Authentication Code) from EPX:

```javascript
// Example: Node.js backend
const response = await fetch('http://localhost:8081/api/v1/payments/browser-post/tac', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${jwtToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    merchant_id: 'your-merchant-id',
    amount: '99.99',
    transaction_type: 'sale', // or 'auth'
    return_url: 'https://yourapp.com/payment/callback'
  })
});

const { tac_token, form_url } = await response.json();
```

**Response:**
```json
{
  "tac_token": "abc123xyz456",
  "form_url": "https://services.epxuap.com/browserpost/",
  "expires_at": "2025-01-17T12:15:00Z"
}
```

### 3.2: Build Payment Form (Frontend)

Create an HTML form that submits to EPX:

```html
<!-- Payment form on your checkout page -->
<form id="payment-form" action="https://services.epxuap.com/browserpost/" method="POST">
  <!-- EPX Credentials (hidden fields) -->
  <input type="hidden" name="tac" value="abc123xyz456">
  <input type="hidden" name="cust_nbr" value="9001">
  <input type="hidden" name="merch_nbr" value="900300">
  <input type="hidden" name="dba_nbr" value="2">
  <input type="hidden" name="terminal_nbr" value="77">

  <!-- Transaction Details -->
  <input type="hidden" name="tran_type" value="CCE1"> <!-- SALE -->
  <input type="hidden" name="tran_amt" value="99.99">
  <input type="hidden" name="tran_nbr" value="ORDER-12345">

  <!-- Callback URL -->
  <input type="hidden" name="return_url" value="https://yourapp.com/payment/callback">

  <!-- Custom Data (echoed back in callback) -->
  <input type="hidden" name="user_data_1" value="customer_id=456">
  <input type="hidden" name="user_data_2" value="order_id=ORDER-12345">

  <!-- Card Details (user enters) -->
  <label>
    Card Number:
    <input type="text" name="card_number" placeholder="4111111111111111" required>
  </label>

  <label>
    Expiration (MMYY):
    <input type="text" name="exp_date" placeholder="1225" required>
  </label>

  <label>
    CVV:
    <input type="text" name="cvv2" placeholder="123" required>
  </label>

  <label>
    Billing ZIP:
    <input type="text" name="avs_zip" placeholder="12345" required>
  </label>

  <button type="submit">Pay $99.99</button>
</form>
```

### 3.3: Submit Form

```javascript
document.getElementById('payment-form').addEventListener('submit', (e) => {
  // Form submits to EPX, user's browser is redirected
  console.log('Processing payment via EPX...');
});
```

**What happens:**
1. User submits form → Browser sends card data to EPX
2. EPX processes payment
3. EPX redirects browser to your `return_url` with results

---

## Step 4: Handle Payment Callbacks

When EPX completes the payment, it redirects the user's browser to your callback URL with payment results.

### 4.1: Callback Endpoint (Backend)

Create an endpoint to receive the callback:

```javascript
// Example: Express.js
app.post('/payment/callback', async (req, res) => {
  const {
    AUTH_GUID,      // BRIC token for refunds/voids
    AUTH_RESP,      // Response code (00 = approved)
    AUTH_CODE,      // Bank authorization code
    AUTH_AMOUNT,    // Authorized amount
    AUTH_CARD_TYPE, // Card brand (VISA, MC, etc.)
    AUTH_CARD_NBR,  // Masked card number (XXXX1111)
    TRAN_NBR,       // Your transaction ID (echoed back)
    USER_DATA_1,    // Custom data (echoed back)
    USER_DATA_2     // Custom data (echoed back)
  } = req.body;

  // Save transaction to your database
  if (AUTH_RESP === '00') {
    await saveTransaction({
      transaction_id: TRAN_NBR,
      bric_token: AUTH_GUID,
      amount: AUTH_AMOUNT,
      status: 'approved',
      card_last_four: AUTH_CARD_NBR.slice(-4),
      card_brand: AUTH_CARD_TYPE
    });

    // Return success page to user
    res.send(`
      <html>
        <body>
          <h1>✅ Payment Successful!</h1>
          <p>Authorization Code: ${AUTH_CODE}</p>
          <p>Amount: $${AUTH_AMOUNT}</p>
          <a href="/orders/${USER_DATA_2}">View Order</a>
        </body>
      </html>
    `);
  } else {
    // Payment declined
    res.send(`
      <html>
        <body>
          <h1>❌ Payment Declined</h1>
          <p>Reason: ${AUTH_RESP_TEXT || 'Card declined'}</p>
          <a href="/checkout">Try Again</a>
        </body>
      </html>
    `);
  }
});
```

### 4.2: Store Transaction in Payment Service

Optionally, store the transaction in the payment service database:

```bash
curl -X POST http://localhost:8081/api/v1/payments/browser-post/callback \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "AUTH_GUID=bric-123&AUTH_RESP=00&AUTH_AMOUNT=99.99&TRAN_NBR=ORDER-12345"
```

This enables:
- Transaction lookup via API
- Refunds/voids using the payment service
- Reporting and analytics

---

## Step 5: Integrate Server APIs (Backend)

For recurring payments, refunds, or backend-only operations, use the Server Post APIs.

### 5.1: Authorize Payment (Server-Side)

```bash
curl -X POST http://localhost:8081/api/v1/payments/authorize \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "merchant_id": "your-merchant-id",
    "customer_id": "customer-456",
    "amount": "99.99",
    "currency": "USD",
    "payment_method_id": "pm-uuid-here",
    "idempotency_key": "auth_20250117_001"
  }'
```

**Response:**
```json
{
  "transaction_id": "tx-uuid",
  "group_id": "grp-uuid",
  "amount": "99.99",
  "status": "TRANSACTION_STATUS_APPROVED",
  "type": "TRANSACTION_TYPE_AUTH",
  "authorization_code": "123456",
  "created_at": "2025-01-17T12:00:00Z"
}
```

### 5.2: Capture Payment

```bash
curl -X POST http://localhost:8081/api/v1/payments/capture \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "merchant_id": "your-merchant-id",
    "transaction_id": "tx-uuid",
    "amount": "99.99",
    "idempotency_key": "capture_20250117_001"
  }'
```

### 5.3: Refund Payment

```bash
curl -X POST http://localhost:8081/api/v1/payments/refund \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "merchant_id": "your-merchant-id",
    "transaction_id": "tx-uuid",
    "amount": "99.99",
    "reason": "Customer requested refund",
    "idempotency_key": "refund_20250117_001"
  }'
```

See [API_SPECS.md](API_SPECS.md) for complete API reference.

---

## Step 6: Test Your Integration

### 6.1: Use EPX Sandbox Test Cards

**Approval Card:**
```
Card: 4111111111111111
CVV: 123
Exp: 12/25
ZIP: 12345
```

**Decline Card (triggers error codes):**
```
Card: 4000000000000002
CVV: 123
Exp: 12/25

Amount triggers:
- $1.05 → Code 05 (Do Not Honor)
- $1.20 → Code 51 (Insufficient Funds)
- $1.54 → Code 54 (Expired Card)
```

### 6.2: Test Idempotency

Submit the same payment twice with the same `idempotency_key`:

```bash
# First request
curl -X POST http://localhost:8081/api/v1/payments/authorize \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -d '{"idempotency_key": "test_001", ...}'

# Second request (should return same transaction)
curl -X POST http://localhost:8081/api/v1/payments/authorize \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  -d '{"idempotency_key": "test_001", ...}'
```

**Expected:** Both requests return the same `transaction_id` (no duplicate charge).

### 6.3: Test Refunds

1. Create a payment
2. Refund it
3. Try to refund again (should fail - already refunded)
4. Try to refund more than original amount (should fail - validation error)

---

## Step 7: Production Checklist

Before going live:

- [ ] **EPX Production Credentials** - Obtained and configured
- [ ] **HTTPS/TLS** - All endpoints use HTTPS
- [ ] **MAC_SECRET Security** - Stored in secret manager (not .env)
- [ ] **Database Backups** - Automated daily backups configured
- [ ] **Monitoring** - Prometheus metrics + alerts set up
- [ ] **Error Handling** - All error codes handled gracefully
- [ ] **Webhook Validation** - HMAC signatures verified
- [ ] **Rate Limiting** - Prevent abuse (if applicable)
- [ ] **Logging** - Transaction logs for compliance (NO card data!)
- [ ] **Testing** - All payment flows tested in production sandbox
- [ ] **PCI Compliance** - Reviewed security requirements

See [GCP_PRODUCTION_SETUP.md](GCP_PRODUCTION_SETUP.md) for deployment guide.

---

## Common Integration Patterns

### Pattern 1: E-commerce Checkout

**Flow:**
1. User adds items to cart
2. User clicks "Checkout"
3. Your backend generates TAC token
4. User enters card details in Browser Post form
5. EPX processes payment
6. User redirected to order confirmation page
7. Your backend receives callback, marks order as paid

**Use Case:** Online stores, SaaS subscriptions, digital goods

### Pattern 2: Subscription Billing

**Flow:**
1. User signs up and saves payment method (Browser Post with BRIC Storage)
2. Your backend creates subscription record
3. Cron job triggers monthly billing via Server Post
4. If payment fails, send notification and retry
5. If payment succeeds, extend subscription

**Use Case:** Monthly SaaS, memberships, recurring donations

### Pattern 3: Marketplace (Multi-Merchant)

**Flow:**
1. Each vendor registered as separate merchant
2. Platform collects payments on behalf of vendors
3. Payment split between platform fee and vendor payout
4. Separate reconciliation per vendor

**Use Case:** Marketplace platforms, multi-vendor stores

---

## Troubleshooting

### Issue: Browser Post callback not received

**Solution:**
- Verify `return_url` is publicly accessible (use ngrok for local dev)
- Check firewall allows EPX IPs
- Verify HTTPS/TLS certificate is valid
- Check server logs for incoming POST requests

### Issue: "Authentication failed" (EPX Code 58)

**Solution:**
- Verify EPX credentials (CUST_NBR, MERCH_NBR, etc.)
- Check MAC_SECRET matches EPX account
- Ensure TAC token hasn't expired (15 min lifetime)
- Verify signature calculation is correct

### Issue: "Idempotency key already used"

**Solution:**
- This is expected! The payment service returned the existing transaction
- Don't retry - use the returned `transaction_id`
- Generate new `idempotency_key` only for new payments

### Issue: Refund fails with "Amount exceeds original"

**Solution:**
- Check refund amount ≤ captured amount
- Partial refunds are supported (just use smaller amount)
- Can't refund more than was captured (validation error)

See [FAQ](wiki-templates/FAQ.md) for more troubleshooting.

---

## Next Steps

- **[API Reference](API_SPECS.md)** - Complete endpoint documentation
- **[DATAFLOW](DATAFLOW.md)** - Detailed payment flow diagrams
- **[FAQ](wiki-templates/FAQ.md)** - Common questions answered
- **[Support](https://github.com/kevin07696/payment-service/issues)** - Report issues

---

**Questions?** Open an issue on [GitHub](https://github.com/kevin07696/payment-service/issues) or check the [FAQ](wiki-templates/FAQ.md).
