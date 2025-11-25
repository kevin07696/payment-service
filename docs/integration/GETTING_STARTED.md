# Getting Started with Payment Service

**Target Audience:** Developers integrating the payment service into their applications
**Topic:** Quick start guide to register, authenticate, and integrate payment processing
**Goal:** Successfully process your first payment in under 30 minutes

---

## Overview

This guide provides a high-level roadmap for integrating with the payment service. Each step links to detailed documentation for implementation specifics.

**Integration Flow:**
1. ✅ Register merchant account
2. ✅ Set up authentication
3. ✅ Choose integration method
4. ✅ Implement payment flow
5. ✅ Test and go live

**Time to First Payment:** ~30 minutes

---

## Step 1: Register Your Merchant Account

**What you need:** EPX merchant credentials and merchant information

**How to register:**
Contact your payment service administrator to register your merchant account using the Payment CLI.

📖 **See:** [Admin CLI Guide](ADMIN_CLI.md) - Complete merchant registration steps

**You'll receive:**
- `merchant_id` - Your unique identifier for all API requests
- `service_id` - For authentication token generation
- Environment configuration (sandbox/production)

---

## Step 2: Set Up Authentication

**All API requests require JWT authentication.**

JWT tokens are generated **client-side** using your RSA private key. The payment service validates tokens using the public key registered during service creation.

**Quick setup (Node.js example):**
```javascript
const jwt = require('jsonwebtoken');
const fs = require('fs');

// Load private key from service credentials file
const credentials = JSON.parse(fs.readFileSync('service_my-app_credentials.json'));
const privateKey = credentials.private_key;

// Generate JWT token
const token = jwt.sign({
  iss: 'my-app',                    // Your service_id
  sub: 'merchant-uuid-here',        // Merchant ID
  merchant_id: 'merchant-uuid-here',
  service_id: 'my-app',
  scopes: ['payment:create', 'payment:read'],
  exp: Math.floor(Date.now() / 1000) + 300,  // 5 minutes
  iat: Math.floor(Date.now() / 1000),
}, privateKey, { algorithm: 'RS256' });
```

**Use token in all requests:**
```bash
curl -X POST http://localhost:8080/payment.v1.PaymentService/Sale \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"
```

📖 **See:** [Token Generation Guide](TOKEN_GENERATION.md) - Complete examples in Node.js, Go, Python, PHP

---

## Step 3: Choose Your Integration Method

Pick the integration method that fits your use case:

### Option A: Browser Post (PCI-Compliant) ⭐ Recommended

**Best for:** Web applications, minimizing PCI compliance scope

**How it works:**
- Customer enters card on EPX-hosted form
- Card data goes directly to EPX (never touches your server)
- You receive tokenized payment method (BRIC)

**Supported:**
- Credit card payments (Sale, Auth Only)
- Save card for recurring payments

📖 **See:** [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md) - Complete implementation guide

---

### Option B: React/TypeScript Integration

**Best for:** React applications, modern frontend frameworks

**How it works:**
- Pre-built React components
- TypeScript type safety
- ConnectRPC client integration

**Includes:**
- Payment form components
- Token management
- Error handling

📖 **See:** [React Integration Guide](REACT_INTEGRATION.md) - React components and hooks

---

### Option C: Direct API Integration

**Best for:** Backend services, server-to-server integration, ACH payments

**How it works:**
- Call ConnectRPC/gRPC APIs directly
- Full control over payment flow
- Required for ACH operations

**Supported:**
- Credit card: Sale, Auth, Capture, Refund, Void
- ACH: Debit, Credit, Void, Account verification
- Subscription/recurring billing

📖 **See:** [API Specifications](API_SPECS.md) - Complete API reference

---

### Option D: Go Module Integration

**Best for:** Go applications, embedding payment logic

**How it works:**
- Import payment service as Go module
- Use services directly (no HTTP overhead)
- Single binary deployment

**Trade-offs:**
- ✅ No network latency
- ✅ Simpler deployment
- ❌ Go-only
- ❌ Tight coupling

📖 **See:** [Module Integration Guide](MODULE_INTEGRATION.md) - Go library usage

---

## Step 4: Implement Payment Flow

### Browser Post Flow

```
1. Frontend → Backend: Request payment form configuration
2. Backend → Payment Service: GET /browser-post/form
3. Payment Service → Backend: TAC token + form config
4. Backend → Frontend: Form configuration
5. Frontend: Display EPX-hosted form
6. Customer: Enters card details
7. Browser → EPX: Submit card data
8. EPX → Browser: Redirect to callback URL
9. Browser → Backend: Callback with payment result
10. Backend: Save transaction, show confirmation
```

📖 **Detailed flow:** [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md#flow)

---

### Direct API Flow (ConnectRPC)

```typescript
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { PaymentService } from './gen/payment/v1/payment_connect';

const transport = createConnectTransport({
  baseUrl: 'http://localhost:8080',
  headers: { 'Authorization': 'Bearer YOUR_JWT_TOKEN' }
});

const client = createPromiseClient(PaymentService, transport);

// Process sale
const response = await client.sale({
  merchantId: 'your-merchant-id',
  customerId: 'customer-123',
  amountCents: BigInt(9999), // $99.99
  paymentMethodId: 'pm-saved-card',
  idempotencyKey: `sale_${Date.now()}`
});

if (response.transaction.status === 'approved') {
  console.log('Payment successful!', response.transaction.id);
}
```

📖 **Complete examples:** [API Specifications](API_SPECS.md)

---

## Step 5: Test Your Integration

### Sandbox Testing

**Endpoint:** `http://localhost:8080` (or your staging environment)

**Test Credit Cards:**
| Card Number | Brand | Result |
|-------------|-------|--------|
| `4111111111111111` | Visa | Approved |
| `5499740000000057` | Mastercard | Approved |
| `340000000000009` | Amex | Approved |
| `4000300011112220` | Visa | Declined |

**Test ACH Accounts:**
| Account | Routing | Result |
|---------|---------|--------|
| Any 10-12 digits | `021000021` | Approved |
| Any | `000000000` | Declined |

📖 **Testing guides:**
- [EPX API Reference](EPX_API_REFERENCE.md#testing) - Test cards and response codes
- [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md#testing) - Browser Post testing

---

## Step 6: Production Checklist

Before going live:

### Security
- [ ] HTTPS/TLS enabled on all endpoints
- [ ] JWT tokens stored securely (never in client-side code)
- [ ] Private keys stored in secret manager (not in code/env files)
- [ ] Callback endpoints validate TAC signatures

### Error Handling
- [ ] Retry logic for retryable errors (51, 61, 91)
- [ ] User-friendly error messages (don't expose internal errors)
- [ ] Logging for debugging (no sensitive data: card numbers, CVV)

### Testing
- [ ] All payment flows tested in sandbox
- [ ] Refund/void flows tested
- [ ] Error scenarios handled gracefully
- [ ] Idempotency keys prevent duplicate charges

### Monitoring
- [ ] Transaction success/failure metrics
- [ ] Alert on high failure rates
- [ ] Monitor response times

### Compliance
- [ ] PCI compliance if handling raw card data (not needed for Browser Post)
- [ ] NACHA compliance for ACH (pre-note verification, return handling)

📖 **Security best practices:** [Authentication Architecture](../development/AUTH.md)

---

## Common Integration Patterns

### Save Card for Recurring Payments

**Browser Post:**
1. Customer completes payment via Browser Post
2. Receive Financial BRIC in callback
3. Convert to Storage BRIC (never expires)
4. Save to customer payment methods
5. Use for future charges

📖 **See:** [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md#saving-cards)

---

### Subscription Billing

**Flow:**
1. Customer creates subscription
2. Save payment method (Browser Post or Server Post)
3. Cron job processes due subscriptions daily
4. Charge saved payment methods
5. Handle failures and retries

📖 **See:** [API Specifications](API_SPECS.md) - Subscription service

---

### ACH Payments

**Pre-Note Verification Required:**
1. Collect bank account details
2. Send Pre-Note ($0.00 verification)
3. Wait 1-3 business days for clearing
4. Mark account as verified
5. Process ACH debits

📖 **See:**
- [EPX API Reference](EPX_API_REFERENCE.md#ach-checking-account-transactions) - ACH transaction types
- [API Specifications](API_SPECS.md) - ACH service methods

---

## Troubleshooting

### Common Issues

**Issue: "Invalid JWT token"**
- **Cause:** Token expired or incorrect signature
- **Solution:** Generate new token, verify private key matches service

**Issue: "Transaction declined (AUTH_RESP=05)"**
- **Cause:** Card restricted or insufficient funds
- **Solution:** Ask customer to use different payment method

**Issue: "TAC token expired"**
- **Cause:** TAC tokens expire in 4 hours
- **Solution:** Request new form configuration

**Issue: "Duplicate transaction detected"**
- **Cause:** Same idempotency key used twice
- **Solution:** Use unique idempotency key per transaction

📖 **Full troubleshooting guides:**
- [Admin CLI](ADMIN_CLI.md#troubleshooting) - Merchant setup issues
- [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md#troubleshooting) - Browser Post errors
- [EPX API Reference](EPX_API_REFERENCE.md#common-response-codes) - EPX response codes

---

## Next Steps

### Production Deployment
📖 [CI/CD Guide](../development/CICD.md) - Deployment pipeline and staging

### Advanced Features
- **Chargebacks:** Read-only chargeback management (respond via North portal)
- **Webhooks:** Real-time payment notifications (if enabled)
- **Analytics:** Transaction reporting and reconciliation

### Need Help?

**Documentation:**
- [API Specifications](API_SPECS.md) - Complete API reference
- [EPX API Reference](EPX_API_REFERENCE.md) - EPX-specific details
- [Token Generation](TOKEN_GENERATION.md) - JWT token management

**Support:**
- Check application logs for detailed error messages
- Review transaction history in database
- Contact payment service administrator

---

**Last Updated:** 2025-11-22
