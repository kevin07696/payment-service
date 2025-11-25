# React Integration Guide

**Target Audience:** React developers integrating payment functionality into web applications
**Topic:** Building type-safe React payment components using ConnectRPC with your payment service
**Goal:** Accept payments in React applications within 30 minutes using JWT authentication and ConnectRPC

---

## Overview

This guide shows how to integrate **your payment service** into React applications using ConnectRPC for type-safe API calls.

**What you'll build:**
- JWT-authenticated API client
- Payment form components
- Saved payment method management
- PCI-compliant Browser Post integration

**Prerequisites completed:**
- ✅ Payment service running ([Setup Guide](../development/SETUP.md))
- ✅ Service registered via admin CLI ([Admin CLI Guide](ADMIN_CLI.md))
- ✅ Merchant created and access granted ([Admin CLI Guide](ADMIN_CLI.md#complete-workflow-example))
- ✅ RSA private key saved securely

**Integration Flow:**
```
1. Payment Service Running → localhost:8080 (gRPC), localhost:8081 (HTTP)
2. Service Registered → RSA keypair (private key saved)
3. React Backend → Generates JWT tokens using private key
4. React Frontend → Calls payment APIs with JWT auth
5. Production → Deploy with proper secret management
```

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Project Setup](#project-setup)
3. [Backend: JWT Token Generation](#backend-jwt-token-generation)
4. [Frontend: ConnectRPC Client](#frontend-connectrpc-client)
5. [Payment Operations](#payment-operations)
6. [Browser Post Integration](#browser-post-integration)
7. [Testing with curl](#testing-with-curl)
8. [Best Practices](#best-practices)
9. [Troubleshooting](#troubleshooting)

---

## Quick Start

### Prerequisites Check

**1. Payment service running:**
```bash
curl http://localhost:8081/cron/health
# Expected: {"status":"healthy","database":"connected"}
```

**2. Service registered:**
```bash
# Should have completed via admin CLI:
./admin -action=create-service
# Output: RSA private key (saved to keys/your-service.pem)
```

**3. Have credentials:**
- `service_id` - Your service identifier (e.g., `react-app`)
- `private_key.pem` - RSA private key from admin CLI
- `merchant_id` - Merchant UUID from admin CLI

📖 **Missing these?** See [Admin CLI Guide](ADMIN_CLI.md#complete-workflow-example)

---

## Project Setup

### Install Dependencies

```bash
npm install @connectrpc/connect @connectrpc/connect-web
npm install --save-dev @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es
```

### Get Proto Files

Proto files are in the payment service repository. Copy them to your project:

```bash
# Clone or copy proto files from payment service repo
mkdir -p proto/payment/v1 proto/payment_method/v1 proto/subscription/v1

# From payment service repo:
cp payment-service/proto/payment/v1/payment.proto proto/payment/v1/
cp payment-service/proto/payment_method/v1/payment_method.proto proto/payment_method/v1/
cp payment-service/proto/subscription/v1/subscription.proto proto/subscription/v1/
```

Or fetch from GitHub (if public):
```bash
# Replace with your actual repo URL
curl -o proto/payment/v1/payment.proto \
  https://raw.githubusercontent.com/your-org/payment-service/main/proto/payment/v1/payment.proto
```

### Generate TypeScript Types

**Create `buf.gen.yaml`:**
```yaml
version: v1
plugins:
  - plugin: es
    out: src/gen
    opt: target=ts
  - plugin: connect-es
    out: src/gen
    opt: target=ts
```

**Generate:**
```bash
npx buf generate

# Output:
# ✅ src/gen/payment/v1/payment_pb.ts
# ✅ src/gen/payment/v1/payment_connect.ts
# ✅ src/gen/payment_method/v1/payment_method_pb.ts
# ✅ src/gen/payment_method/v1/payment_method_connect.ts
# ✅ src/gen/subscription/v1/subscription_pb.ts
# ✅ src/gen/subscription/v1/subscription_connect.ts
```

---

## Backend: JWT Token Generation

**⚠️ SECURITY:** Generate JWT tokens on your backend (Node.js/Express), NOT in the browser.

📖 **Complete JWT guide:** [Authentication Architecture](../development/AUTH.md)

### Backend Endpoint (Node.js/Express)

```typescript
// backend/routes/auth.ts
import express from 'express';
import jwt from 'jsonwebtoken';
import fs from 'fs';

const router = express.Router();

// Load RSA private key from admin CLI
const privateKey = fs.readFileSync(
  process.env.PRIVATE_KEY_PATH || 'keys/react-app.pem'
);
const serviceId = process.env.SERVICE_ID || 'react-app';
const merchantId = process.env.MERCHANT_ID!;

router.post('/api/payment-token', (req, res) => {
  const now = Math.floor(Date.now() / 1000);

  const token = jwt.sign({
    iss: serviceId,
    sub: merchantId,
    merchant_id: merchantId,
    service_id: serviceId,
    scopes: ['payment:create', 'payment:read', 'payment:refund', 'payment:void'],
    env: 'production',
    exp: now + 300, // 5 minutes
    iat: now,
    nbf: now,
    jti: crypto.randomUUID(),
  }, privateKey, { algorithm: 'RS256' });

  res.json({ token, expiresIn: 300 });
});

export default router;
```

**Environment variables:**
```bash
# .env (backend)
PRIVATE_KEY_PATH=keys/react-app.pem
SERVICE_ID=react-app
MERCHANT_ID=550e8400-e29b-41d4-a716-446655440000
```

📖 **More languages:** See [Admin CLI Guide](ADMIN_CLI.md#complete-workflow-example) for Go example

---

## Frontend: ConnectRPC Client

### Payment Client Setup

**File:** `src/lib/payment-client.ts`

```typescript
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { PaymentService } from '../gen/payment/v1/payment_connect';
import { PaymentMethodService } from '../gen/payment_method/v1/payment_method_connect';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';

/**
 * Fetch JWT token from your backend
 */
async function getAuthToken(): Promise<string> {
  const response = await fetch('http://localhost:3001/api/payment-token', {
    method: 'POST',
    credentials: 'include', // Send cookies if using session auth
  });

  if (!response.ok) {
    throw new Error('Failed to fetch payment token');
  }

  const { token } = await response.json();
  return token;
}

/**
 * Create transport with automatic JWT authentication
 */
function createAuthTransport() {
  return createConnectTransport({
    baseUrl: API_URL,
    interceptors: [
      (next) => async (req) => {
        // Fetch fresh token for each request
        const token = await getAuthToken();
        req.header.set('Authorization', `Bearer ${token}`);
        return next(req);
      },
    ],
  });
}

// Export typed clients
export const paymentClient = createPromiseClient(
  PaymentService,
  createAuthTransport()
);

export const paymentMethodClient = createPromiseClient(
  PaymentMethodService,
  createAuthTransport()
);
```

**Environment variables:**
```bash
# .env (React frontend)
REACT_APP_API_URL=http://localhost:8080
```

---

## Payment Operations

### usePayment Hook

**File:** `src/hooks/usePayment.ts`

```typescript
import { useState } from 'react';
import { paymentClient } from '../lib/payment-client';

export function usePayment(merchantId: string) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const sale = async (
    customerId: string,
    amountCents: bigint,
    paymentMethodId: string
  ) => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.sale({
        merchantId,
        customerId,
        amountCents,
        currency: 'USD',
        paymentMethodId,
        idempotencyKey: `sale_${Date.now()}_${crypto.randomUUID()}`,
      });

      return response;
    } catch (err) {
      setError(err as Error);
      return null;
    } finally {
      setLoading(false);
    }
  };

  const refund = async (transactionId: string, amountCents: bigint, reason: string) => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.refund({
        transactionId,
        amountCents,
        reason,
        idempotencyKey: `refund_${Date.now()}_${crypto.randomUUID()}`,
      });

      return response;
    } catch (err) {
      setError(err as Error);
      return null;
    } finally {
      setLoading(false);
    }
  };

  return { sale, refund, loading, error };
}
```

**Available operations** (from `payment.v1.PaymentService`):
- `authorize` - Hold funds without capturing
- `capture` - Complete a previous authorization
- `sale` - Authorize + capture in one step
- `void` - Cancel authorization
- `refund` - Return funds to customer
- `achDebit` - Pull money from bank account
- `achCredit` - Send money to bank account
- `getTransaction` - Retrieve transaction details
- `listTransactions` - List transactions for merchant/customer

📖 **Complete API reference:** [proto/payment/v1/payment.proto](../../proto/payment/v1/payment.proto)

### Payment Form Component

**File:** `src/components/PaymentForm.tsx`

```typescript
import React, { useState } from 'react';
import { usePayment } from '../hooks/usePayment';

interface PaymentFormProps {
  merchantId: string;
  customerId: string;
  paymentMethodId: string;
  onSuccess: (transactionId: string) => void;
}

export function PaymentForm({ merchantId, customerId, paymentMethodId, onSuccess }: PaymentFormProps) {
  const { sale, loading, error } = usePayment(merchantId);
  const [amount, setAmount] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const amountCents = BigInt(Math.round(parseFloat(amount) * 100));
    const response = await sale(customerId, amountCents, paymentMethodId);

    if (response?.isApproved) {
      onSuccess(response.transactionId);
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      <input
        type="number"
        step="0.01"
        value={amount}
        onChange={(e) => setAmount(e.target.value)}
        placeholder="Amount (USD)"
        required
      />
      <button type="submit" disabled={loading}>
        {loading ? 'Processing...' : `Pay $${amount || '0.00'}`}
      </button>
      {error && <div className="error">{error.message}</div>}
    </form>
  );
}
```

---

## Browser Post Integration

For PCI-compliant card collection where card data never touches your server.

📖 **Complete guide:** [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md)

**Quick example:**

```typescript
import React, { useState, useEffect } from 'react';

export function BrowserPostForm({ merchantId, amount, returnUrl }: Props) {
  const [formConfig, setFormConfig] = useState(null);

  useEffect(() => {
    // Call your payment service to get Browser Post config
    fetch(`http://localhost:8081/api/v1/payments/browser-post/form?` +
      `merchant_id=${merchantId}&amount=${amount}&transaction_type=SALE&return_url=${returnUrl}`)
      .then(res => res.json())
      .then(setFormConfig);
  }, []);

  if (!formConfig) return <div>Loading...</div>;

  return (
    <form method="POST" action={formConfig.postURL}>
      <input type="hidden" name="TAC" value={formConfig.tac} />
      <input type="hidden" name="CUST_NBR" value={formConfig.custNbr} />
      {/* ...other hidden fields... */}

      <input name="CARD_NBR" placeholder="Card Number" required />
      <input name="EXP_DATE" placeholder="MMYY" required />
      <input name="CVV" placeholder="CVV" required />
      <button type="submit">Pay ${amount}</button>
    </form>
  );
}
```

📖 **Complete implementation:** [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md#complete-html-form-example)

---

## Testing with curl

Test your payment service APIs before React integration:

```bash
# 1. Generate JWT token (using your backend endpoint)
TOKEN=$(curl -s http://localhost:3001/api/payment-token | jq -r .token)

# 2. Test Sale transaction
curl -X POST http://localhost:8080/payment.v1.PaymentService/Sale \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
    "customer_id": "customer_123",
    "amount_cents": "9999",
    "currency": "USD",
    "payment_method_id": "pm-uuid-here",
    "idempotency_key": "sale_test_001"
  }'

# 3. Test List Transactions
curl -X POST http://localhost:8080/payment.v1.PaymentService/ListTransactions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
    "limit": 10
  }'
```

---

## Best Practices

### 1. Always Use BigInt for Amounts

```typescript
// ✅ Correct
const amountCents = BigInt(9999); // $99.99

// ❌ Wrong
const amountCents = 9999; // Type error - proto expects bigint
```

### 2. Generate Unique Idempotency Keys

```typescript
// ✅ Good
const idempotencyKey = `sale_${orderId}_${Date.now()}_${crypto.randomUUID()}`;

// ❌ Bad
const idempotencyKey = 'sale_key'; // Reusing prevents ALL operations
```

### 3. Token Caching

Your backend should cache JWT tokens to avoid regenerating every request:

```typescript
let tokenCache: { token: string; expiresAt: number } | null = null;

async function getAuthToken() {
  if (tokenCache && tokenCache.expiresAt > Date.now() + 30000) {
    return tokenCache.token;
  }

  // Generate new token...
  tokenCache = { token, expiresAt: Date.now() + 300000 };
  return token;
}
```

### 4. Never Put Private Keys in Browser

```bash
# ❌ WRONG - Never do this
REACT_APP_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----

# ✅ CORRECT - Private key stays on backend
# Backend .env:
PRIVATE_KEY_PATH=keys/react-app.pem
```

---

## Troubleshooting

### "Unauthenticated" Error

**Cause:** JWT token invalid, expired, or missing.

**Solution:**
```bash
# Decode token to inspect claims
echo "$TOKEN" | cut -d. -f2 | base64 -d | jq

# Check:
# - exp (expiration) is in future
# - service_id matches your admin CLI registration
# - merchant_id is correct
```

📖 **More details:** [Authentication Architecture](../development/AUTH.md)

### "Permission Denied" Error

**Cause:** Service lacks required scopes for operation.

**Solution:**
```bash
# Grant additional scopes via admin CLI
./admin -action=grant-access

# Or check current scopes:
psql $DATABASE_URL -c "
  SELECT s.service_id, m.slug, sm.scopes
  FROM service_merchants sm
  JOIN services s ON s.id = sm.service_id
  JOIN merchants m ON m.id = sm.merchant_id
  WHERE s.service_id = 'react-app'
"
```

### CORS Error

**Cause:** Payment service doesn't allow requests from your React app origin.

**Solution:**
Configure CORS in payment service (server config) or use proxy:

```json
// package.json (React development)
{
  "proxy": "http://localhost:8080"
}
```

---

## Related Documentation

**Setup & Configuration:**
- [Setup Guide](../development/SETUP.md) - Running the payment service
- [Admin CLI Guide](ADMIN_CLI.md) - Creating services and merchants
- [Authentication Architecture](../development/AUTH.md) - JWT implementation details

**Integration Guides:**
- [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md) - PCI-compliant card tokenization
- [Getting Started](GETTING_STARTED.md) - Quick start overview

**API Reference:**
- [proto/payment/v1/payment.proto](../../proto/payment/v1/payment.proto) - Payment operations
- [proto/payment_method/v1/payment_method.proto](../../proto/payment_method/v1/payment_method.proto) - Saved payment methods
- [proto/subscription/v1/subscription.proto](../../proto/subscription/v1/subscription.proto) - Recurring billing

---

**Last Updated:** 2025-11-24
