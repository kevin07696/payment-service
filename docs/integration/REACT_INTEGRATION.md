# React Integration Guide

**Target Audience:** React developers integrating payment functionality into web applications
**Topic:** Complete React integration using ConnectRPC with TypeScript type safety
**Goal:** Accept payments in a React application within 30 minutes, from API testing to production-ready components

---

## Overview

This guide provides a complete path to integrating payment functionality into React applications. You'll learn to test APIs with curl first, then build type-safe React components using ConnectRPC.

**What you'll build:**
- Authentication with JWT tokens
- Payment form components with idempotency
- Saved payment method management
- Subscription billing interfaces
- Browser Post PCI-compliant forms

**Integration Flow:**
```
1. Test APIs with curl → Understand responses
2. Set up authentication → Generate JWT tokens
3. Generate TypeScript types → Type-safe client
4. Build React components → Payment UI
5. Handle errors → User-friendly messages
6. Go to production → Security checklist
```

**Time to First Payment:** ~30 minutes

**Key Concepts:**
- **ConnectRPC**: Modern RPC framework over HTTP/2 or HTTP/1.1
- **JWT Authentication**: RSA-signed tokens for API access
- **Idempotency**: Preventing duplicate charges with unique keys
- **Browser Post**: PCI-compliant card tokenization (card data never touches your server)

---

## Prerequisites

Before you begin, ensure you have:

### 1. Service Registration and Keys

You need a registered service with JWT authentication credentials.

**📖 See:** [Token Generation Guide](TOKEN_GENERATION.md) - Complete service registration and JWT setup

**What you'll receive:**
- `service_id` - Your application identifier (e.g., `acme-web-app`)
- `private_key.pem` - RSA private key for signing JWT tokens
- `merchant_id` - Your merchant identifier for transactions

### 2. Development Environment

- Node.js 18+ or 20+
- React 18+
- TypeScript 5+
- Package manager (npm, yarn, or pnpm)

### 3. Payment Service Access

- API endpoint URL (e.g., `http://localhost:8080` for development)
- EPX credentials configured in payment service (handled by admin)

---

## Quick Start (5 Minutes)

### Step 1: Test the API with curl

Before writing any React code, verify the API works with curl.

**Generate a JWT token:**

```bash
# Save your private key to a file
cat > /tmp/private_key.pem <<'EOF'
-----BEGIN RSA PRIVATE KEY-----
[Your private key from service registration]
-----END RSA PRIVATE KEY-----
EOF

chmod 600 /tmp/private_key.pem
```

**Use this Node.js script to generate a token:**

```javascript
// generate-token.js
const jwt = require('jsonwebtoken');
const fs = require('fs');

const privateKey = fs.readFileSync('/tmp/private_key.pem');
const now = Math.floor(Date.now() / 1000);

const token = jwt.sign({
  iss: 'your-service-id',           // Replace with your service_id
  sub: 'your-merchant-id',           // Replace with your merchant_id
  merchant_id: 'your-merchant-id',
  service_id: 'your-service-id',
  scopes: ['payment:create', 'payment:read'],
  env: 'production',
  exp: now + 300,  // 5 minutes
  iat: now,
  nbf: now,
  jti: require('crypto').randomUUID()
}, privateKey, { algorithm: 'RS256' });

console.log(token);
```

```bash
# Install jsonwebtoken
npm install jsonwebtoken

# Generate token
TOKEN=$(node generate-token.js)

# Save token for testing
echo $TOKEN
```

**Test the API:**

```bash
# Test 1: Authorize payment (hold funds)
curl -X POST http://localhost:8080/payment.v1.PaymentService/Authorize \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "your-merchant-id",
    "customer_id": "customer_test_123",
    "amount_cents": "9999",
    "currency": "USD",
    "payment_method_id": "pm-test-uuid",
    "idempotency_key": "auth_test_001"
  }'

# Expected response (200 OK):
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "merchant_id": "your-merchant-id",
    "customer_id": "customer_test_123",
    "amount_cents": "9999",
    "currency": "USD",
    "status": "approved",
    "transaction_type": "auth",
    "auth_code": "123456",
    "created_at": "2025-01-20T12:00:00Z"
  }
}
```

```bash
# Test 2: Sale (authorize + capture in one step)
curl -X POST http://localhost:8080/payment.v1.PaymentService/Sale \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "your-merchant-id",
    "customer_id": "customer_test_123",
    "amount_cents": "9999",
    "currency": "USD",
    "payment_method_id": "pm-test-uuid",
    "idempotency_key": "sale_test_001"
  }'
```

```bash
# Test 3: List transactions
curl -X POST http://localhost:8080/payment.v1.PaymentService/ListTransactions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "your-merchant-id",
    "customer_id": "customer_test_123",
    "limit": 10
  }'
```

**If curl tests work, you're ready for React integration!**

**📖 For more curl examples:** [API Specs](API_SPECS.md), [Token Generation](TOKEN_GENERATION.md)

---

### Step 2: Install React Dependencies

```bash
npm install @connectrpc/connect @connectrpc/connect-web
npm install --save-dev @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es
```

### Step 3: Download Proto Files

```bash
# Create proto directory
mkdir -p proto/payment/v1 proto/payment_method/v1 proto/subscription/v1

# Download proto files from payment service
curl -o proto/payment/v1/payment.proto \
  http://localhost:8080/proto/payment/v1/payment.proto

curl -o proto/payment_method/v1/payment_method.proto \
  http://localhost:8080/proto/payment_method/v1/payment_method.proto

curl -o proto/subscription/v1/subscription.proto \
  http://localhost:8080/proto/subscription/v1/subscription.proto
```

### Step 4: Generate TypeScript Clients

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

**Generate clients:**

```bash
# Using buf (recommended)
npx buf generate

# Output:
# ✅ src/gen/payment/v1/payment_pb.ts
# ✅ src/gen/payment/v1/payment_connect.ts
# ✅ src/gen/payment_method/v1/payment_method_pb.ts
# ✅ src/gen/payment_method/v1/payment_method_connect.ts
# ✅ src/gen/subscription/v1/subscription_pb.ts
# ✅ src/gen/subscription/v1/subscription_connect.ts
```

### Step 5: Create Your First Payment

**File:** `src/App.tsx`

```typescript
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { PaymentService } from './gen/payment/v1/payment_connect';

const transport = createConnectTransport({
  baseUrl: 'http://localhost:8080',
  interceptors: [(next) => async (req) => {
    // TODO: Add JWT token here (see Authentication section)
    req.header.set('Authorization', 'Bearer YOUR_JWT_TOKEN');
    return next(req);
  }],
});

const client = createPromiseClient(PaymentService, transport);

async function processPayment() {
  const response = await client.sale({
    merchantId: 'your-merchant-id',
    customerId: 'customer_123',
    amountCents: BigInt(9999), // $99.99
    currency: 'USD',
    paymentMethodId: 'pm-saved-card',
    idempotencyKey: `sale_${Date.now()}_${crypto.randomUUID()}`,
  });

  if (response.transaction.status === 'approved') {
    console.log('✅ Payment successful!', response.transaction.id);
  }
}
```

**📖 Next:** Continue to [Authentication](#authentication) to set up JWT token management properly.

---

## Project Structure

Recommended file organization for React payment integration:

```
src/
├── lib/
│   ├── payment-client.ts      # ConnectRPC client setup
│   ├── auth.ts                # JWT token generation/management
│   ├── error-handler.ts       # Payment error parsing
│   └── types.ts               # Common TypeScript types
├── hooks/
│   ├── usePayment.ts          # Payment operations hook
│   ├── usePaymentMethods.ts   # Payment methods hook
│   ├── useSubscription.ts     # Subscription billing hook
│   └── useAuth.ts             # Authentication hook
├── components/
│   ├── PaymentForm.tsx        # Payment form component
│   ├── BrowserPost.tsx        # Browser Post integration
│   ├── PaymentMethodList.tsx  # Saved payment methods
│   └── ErrorDisplay.tsx       # Error message component
├── gen/                       # Generated proto types (from buf)
│   ├── payment/v1/
│   ├── payment_method/v1/
│   └── subscription/v1/
└── config/
    └── payment.config.ts      # Payment service configuration
```

---

## Authentication

### JWT Token Management

The payment service requires JWT tokens for all API requests. Here's how to generate and manage them in React.

**📖 Complete guide:** [Token Generation](TOKEN_GENERATION.md#step-3-generate-jwt-tokens)

### Backend Token Generation (Recommended)

**Security best practice:** Generate JWT tokens on your backend, not in the browser.

**Why?** Private keys should never be in client-side code (security risk).

**Architecture:**
```
React App → Your Backend → Payment Service
           (generates JWT)
```

**Backend endpoint example (Node.js/Express):**

```typescript
// backend/routes/auth.ts
import express from 'express';
import jwt from 'jsonwebtoken';
import fs from 'fs';

const router = express.Router();

const privateKey = fs.readFileSync(process.env.JWT_PRIVATE_KEY_PATH!);
const serviceId = process.env.SERVICE_ID!;

router.post('/api/auth/payment-token', (req, res) => {
  const { merchantId } = req.body;
  const now = Math.floor(Date.now() / 1000);

  const token = jwt.sign({
    iss: serviceId,
    sub: merchantId,
    merchant_id: merchantId,
    service_id: serviceId,
    scopes: ['payment:create', 'payment:read', 'payment:refund'],
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

### Frontend Token Management

**File:** `src/lib/auth.ts`

```typescript
import { jwtDecode } from 'jwt-decode';

interface JWTClaims {
  merchant_id: string;
  service_id: string;
  scopes: string[];
  exp: number;
  iat: number;
}

interface TokenCache {
  token: string;
  expiresAt: number;
}

class PaymentAuth {
  private cache: TokenCache | null = null;
  private merchantId: string;
  private backendUrl: string;

  constructor(merchantId: string, backendUrl: string = '/api') {
    this.merchantId = merchantId;
    this.backendUrl = backendUrl;
  }

  /**
   * Get valid JWT token (from cache or fetch new)
   */
  async getToken(): Promise<string> {
    // Return cached token if still valid (with 30s buffer)
    const now = Date.now();
    if (this.cache && this.cache.expiresAt > now + 30000) {
      return this.cache.token;
    }

    // Fetch new token from backend
    const response = await fetch(`${this.backendUrl}/auth/payment-token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        merchantId: this.merchantId,
      }),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch payment token');
    }

    const { token, expiresIn } = await response.json();
    const decoded = jwtDecode<JWTClaims>(token);

    // Cache token with expiry
    this.cache = {
      token,
      expiresAt: decoded.exp * 1000,
    };

    return token;
  }

  /**
   * Clear cached token (use on logout)
   */
  clearToken() {
    this.cache = null;
  }

  /**
   * Get merchant ID
   */
  getMerchantId(): string {
    return this.merchantId;
  }
}

// Export singleton instance
export const paymentAuth = new PaymentAuth(
  process.env.REACT_APP_MERCHANT_ID || ''
);

export async function getAuthToken(): Promise<string> {
  return paymentAuth.getToken();
}

export function clearAuthToken() {
  paymentAuth.clearToken();
}
```

### React Hook for Authentication

**File:** `src/hooks/useAuth.ts`

```typescript
import { useState, useEffect } from 'react';
import { getAuthToken, clearAuthToken } from '../lib/auth';

export function useAuth() {
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    loadToken();
  }, []);

  const loadToken = async () => {
    try {
      setLoading(true);
      const newToken = await getAuthToken();
      setToken(newToken);
      setError(null);
    } catch (err) {
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  };

  const logout = () => {
    clearAuthToken();
    setToken(null);
  };

  return { token, loading, error, logout, refreshToken: loadToken };
}
```

**📖 For backend JWT generation examples:** [Token Generation - Step 3](TOKEN_GENERATION.md#step-3-generate-jwt-tokens)

---

## Setup and Configuration

### Payment Client Setup

**File:** `src/lib/payment-client.ts`

```typescript
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { PaymentService } from '../gen/payment/v1/payment_connect';
import { PaymentMethodService } from '../gen/payment_method/v1/payment_method_connect';
import { SubscriptionService } from '../gen/subscription/v1/subscription_connect';
import { getAuthToken } from './auth';

const BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';

/**
 * Create transport with automatic JWT authentication
 */
function createAuthTransport() {
  return createConnectTransport({
    baseUrl: BASE_URL,
    interceptors: [
      (next) => async (req) => {
        // Automatically add JWT token to all requests
        const token = await getAuthToken();
        if (token) {
          req.header.set('Authorization', `Bearer ${token}`);
        }
        return next(req);
      },
    ],
  });
}

// Create typed clients
export const paymentClient = createPromiseClient(
  PaymentService,
  createAuthTransport()
);

export const paymentMethodClient = createPromiseClient(
  PaymentMethodService,
  createAuthTransport()
);

export const subscriptionClient = createPromiseClient(
  SubscriptionService,
  createAuthTransport()
);
```

### Environment Configuration

**File:** `.env`

```bash
# Payment Service Configuration
REACT_APP_API_URL=http://localhost:8080
REACT_APP_MERCHANT_ID=550e8400-e29b-41d4-a716-446655440000

# Backend API for JWT generation
REACT_APP_BACKEND_URL=http://localhost:3001/api
```

**⚠️ SECURITY:** Never put private keys in `.env` files in React projects (they're exposed in the browser).

---

## Payment Operations

### usePayment Hook

**File:** `src/hooks/usePayment.ts`

```typescript
import { useState } from 'react';
import { paymentClient } from '../lib/payment-client';
import { PaymentResponse } from '../gen/payment/v1/payment_pb';

export function usePayment(merchantId: string) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  /**
   * Generate unique idempotency key
   */
  const generateIdempotencyKey = (prefix: string): string => {
    return `${prefix}_${Date.now()}_${crypto.randomUUID().slice(0, 8)}`;
  };

  /**
   * Authorize payment (hold funds)
   */
  const authorize = async (
    customerId: string,
    amountCents: bigint,
    paymentMethodId: string,
    metadata?: Record<string, string>
  ): Promise<PaymentResponse | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.authorize({
        merchantId,
        customerId,
        amountCents,
        currency: 'USD',
        paymentMethodId,
        idempotencyKey: generateIdempotencyKey('auth'),
        metadata,
      });

      return response;
    } catch (err) {
      setError(err as Error);
      return null;
    } finally {
      setLoading(false);
    }
  };

  /**
   * Capture authorized payment
   */
  const capture = async (
    transactionId: string,
    amountCents?: bigint
  ): Promise<PaymentResponse | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.capture({
        merchantId,
        transactionId,
        amountCents, // Optional for partial capture
        idempotencyKey: generateIdempotencyKey('capture'),
      });

      return response;
    } catch (err) {
      setError(err as Error);
      return null;
    } finally {
      setLoading(false);
    }
  };

  /**
   * Sale (authorize + capture in one step)
   */
  const sale = async (
    customerId: string,
    amountCents: bigint,
    paymentMethodId: string,
    metadata?: Record<string, string>
  ): Promise<PaymentResponse | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.sale({
        merchantId,
        customerId,
        amountCents,
        currency: 'USD',
        paymentMethodId,
        idempotencyKey: generateIdempotencyKey('sale'),
        metadata,
      });

      return response;
    } catch (err) {
      setError(err as Error);
      return null;
    } finally {
      setLoading(false);
    }
  };

  /**
   * Refund payment
   */
  const refund = async (
    transactionId: string,
    amountCents: bigint,
    reason: string
  ): Promise<PaymentResponse | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.refund({
        merchantId,
        transactionId,
        amountCents,
        reason,
        idempotencyKey: generateIdempotencyKey('refund'),
      });

      return response;
    } catch (err) {
      setError(err as Error);
      return null;
    } finally {
      setLoading(false);
    }
  };

  /**
   * Void payment (cancel authorization)
   */
  const voidPayment = async (transactionId: string): Promise<PaymentResponse | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.void({
        merchantId,
        transactionId,
        idempotencyKey: generateIdempotencyKey('void'),
      });

      return response;
    } catch (err) {
      setError(err as Error);
      return null;
    } finally {
      setLoading(false);
    }
  };

  return {
    authorize,
    capture,
    sale,
    refund,
    voidPayment,
    loading,
    error,
  };
}
```

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
  onError: (error: Error) => void;
}

export function PaymentForm({
  merchantId,
  customerId,
  paymentMethodId,
  onSuccess,
  onError,
}: PaymentFormProps) {
  const { sale, loading, error } = usePayment(merchantId);
  const [amount, setAmount] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Convert dollars to cents (as bigint)
    const amountCents = BigInt(Math.round(parseFloat(amount) * 100));

    const response = await sale(
      customerId,
      amountCents,
      paymentMethodId,
      {
        source: 'web-checkout',
        user_agent: navigator.userAgent,
      }
    );

    if (response?.transaction.status === 'approved') {
      onSuccess(response.transaction.id);
    } else if (error) {
      onError(error);
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      <div>
        <label htmlFor="amount">Amount (USD)</label>
        <input
          id="amount"
          type="number"
          step="0.01"
          min="0.01"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          required
          disabled={loading}
          placeholder="99.99"
        />
      </div>

      <button type="submit" disabled={loading || !amount}>
        {loading ? 'Processing...' : `Pay $${amount || '0.00'}`}
      </button>

      {error && (
        <div className="error">
          {error.message}
        </div>
      )}
    </form>
  );
}
```

**📖 For more payment operation examples:** [Getting Started](GETTING_STARTED.md#step-4-implement-payment-flow)

---

## Testing Your APIs with curl

Before building React components, test each API endpoint with curl to understand the request/response format.

### Test Sale Transaction

```bash
# Generate token (see Quick Start section)
TOKEN=$(node generate-token.js)

# Test sale
curl -X POST http://localhost:8080/payment.v1.PaymentService/Sale \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "your-merchant-id",
    "customer_id": "customer_123",
    "amount_cents": "9999",
    "currency": "USD",
    "payment_method_id": "pm-test-uuid",
    "idempotency_key": "sale_test_001"
  }'

# Response:
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "approved",
    "auth_code": "123456",
    "amount_cents": "9999"
  }
}
```

### Test Refund

```bash
curl -X POST http://localhost:8080/payment.v1.PaymentService/Refund \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "your-merchant-id",
    "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
    "amount_cents": "9999",
    "reason": "Customer requested refund",
    "idempotency_key": "refund_test_001"
  }'
```

### Test List Transactions

```bash
curl -X POST http://localhost:8080/payment.v1.PaymentService/ListTransactions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "your-merchant-id",
    "customer_id": "customer_123",
    "limit": 10
  }'
```

**📖 More curl examples:** [Browser Post Reference](BROWSER_POST_FORM_SETUP.md#getting-form-configuration)

---

## Browser Post Integration (PCI-Compliant)

Browser Post allows you to collect card details without touching your server, reducing PCI compliance scope.

**📖 Complete guide:** [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md)

### Browser Post Component

**File:** `src/components/BrowserPost.tsx`

```typescript
import React, { useState, useEffect } from 'react';

interface BrowserPostFormConfig {
  transactionId: string;
  tac: string;
  postURL: string;
  custNbr: string;
  merchNbr: string;
  dbaName: string;
  terminalNbr: string;
  epxTranNbr: string;
  redirectURL: string;
  expiresAt: number;
}

interface BrowserPostProps {
  merchantId: string;
  amount: string;
  transactionType: 'SALE' | 'AUTH' | 'STORAGE';
  customerId?: string;
  returnUrl: string;
}

export function BrowserPost({
  merchantId,
  amount,
  transactionType,
  customerId,
  returnUrl,
}: BrowserPostProps) {
  const [formConfig, setFormConfig] = useState<BrowserPostFormConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadFormConfig();
  }, []);

  const loadFormConfig = async () => {
    try {
      const transactionId = crypto.randomUUID();
      const params = new URLSearchParams({
        transaction_id: transactionId,
        merchant_id: merchantId,
        amount: amount,
        transaction_type: transactionType,
        return_url: returnUrl,
      });

      if (customerId) {
        params.append('customer_id', customerId);
      }

      const response = await fetch(
        `http://localhost:8081/api/v1/payments/browser-post/form?${params}`
      );

      if (!response.ok) {
        throw new Error('Failed to load payment form');
      }

      const config = await response.json();
      setFormConfig(config);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  if (loading) return <div>Loading payment form...</div>;
  if (error) return <div>Error: {error}</div>;
  if (!formConfig) return null;

  return (
    <div className="browser-post-container">
      <h2>Enter Payment Information</h2>
      <form method="POST" action={formConfig.postURL}>
        {/* Hidden EPX fields */}
        <input type="hidden" name="TAC" value={formConfig.tac} />
        <input type="hidden" name="CUST_NBR" value={formConfig.custNbr} />
        <input type="hidden" name="MERCH_NBR" value={formConfig.merchNbr} />
        <input type="hidden" name="DBA_NBR" value={formConfig.dbaName} />
        <input type="hidden" name="TERMINAL_NBR" value={formConfig.terminalNbr} />
        <input type="hidden" name="TRAN_NBR" value={formConfig.epxTranNbr} />
        <input type="hidden" name="TRAN_GROUP" value={transactionType === 'SALE' ? 'U' : transactionType === 'AUTH' ? 'A' : 'S'} />
        <input type="hidden" name="AMOUNT" value={amount} />
        <input type="hidden" name="INDUSTRY_TYPE" value="E" />
        <input type="hidden" name="REDIRECT_URL" value={formConfig.redirectURL} />

        {/* Card input fields */}
        <div className="form-group">
          <label htmlFor="CARD_NBR">Card Number</label>
          <input
            id="CARD_NBR"
            name="CARD_NBR"
            type="text"
            maxLength={16}
            placeholder="4111111111111111"
            required
            autoComplete="cc-number"
          />
        </div>

        <div className="form-row">
          <div className="form-group">
            <label htmlFor="EXP_DATE">Expiration (MMYY)</label>
            <input
              id="EXP_DATE"
              name="EXP_DATE"
              type="text"
              maxLength={4}
              placeholder="1225"
              required
              autoComplete="cc-exp"
            />
          </div>

          <div className="form-group">
            <label htmlFor="CVV">CVV</label>
            <input
              id="CVV"
              name="CVV"
              type="text"
              maxLength={4}
              placeholder="123"
              required
              autoComplete="cc-csc"
            />
          </div>
        </div>

        <div className="form-group">
          <label htmlFor="AVS_ZIP">ZIP Code</label>
          <input
            id="AVS_ZIP"
            name="AVS_ZIP"
            type="text"
            maxLength={10}
            placeholder="12345"
            required
            autoComplete="postal-code"
          />
        </div>

        <button type="submit" className="submit-button">
          {transactionType === 'STORAGE' ? 'Save Card' : `Pay $${amount}`}
        </button>
      </form>

      <p className="security-notice">
        🔒 Your card information is sent directly to our secure payment processor
        and never touches our servers.
      </p>
    </div>
  );
}
```

**📖 See also:** [Browser Post Reference](BROWSER_POST_FORM_SETUP.md#complete-html-form-example)

---

## Complete E-Commerce Example

**File:** `src/pages/Checkout.tsx`

```typescript
import React, { useState } from 'react';
import { usePayment } from '../hooks/usePayment';
import { BrowserPost } from '../components/BrowserPost';

interface CheckoutProps {
  merchantId: string;
  customerId: string;
  orderTotal: number;
  orderId: string;
}

export function Checkout({ merchantId, customerId, orderTotal, orderId }: CheckoutProps) {
  const { sale, loading } = usePayment(merchantId);
  const [paymentMethod, setPaymentMethod] = useState<'new' | 'saved'>('new');
  const [selectedPaymentMethodId, setSelectedPaymentMethodId] = useState<string>('');

  const handlePayWithSavedCard = async () => {
    if (!selectedPaymentMethodId) return;

    const amountCents = BigInt(Math.round(orderTotal * 100));
    const response = await sale(
      customerId,
      amountCents,
      selectedPaymentMethodId,
      {
        order_id: orderId,
        source: 'web-checkout',
      }
    );

    if (response?.transaction.status === 'approved') {
      window.location.href = `/orders/${orderId}?success=true`;
    }
  };

  return (
    <div className="checkout">
      <h1>Checkout</h1>

      <div className="order-summary">
        <h2>Order Total: ${orderTotal.toFixed(2)}</h2>
        <p>Order ID: {orderId}</p>
      </div>

      <div className="payment-options">
        <label>
          <input
            type="radio"
            value="new"
            checked={paymentMethod === 'new'}
            onChange={() => setPaymentMethod('new')}
          />
          Add New Card
        </label>
        <label>
          <input
            type="radio"
            value="saved"
            checked={paymentMethod === 'saved'}
            onChange={() => setPaymentMethod('saved')}
          />
          Use Saved Payment Method
        </label>
      </div>

      {paymentMethod === 'new' && (
        <BrowserPost
          merchantId={merchantId}
          amount={orderTotal.toFixed(2)}
          transactionType="SALE"
          customerId={customerId}
          returnUrl={`${window.location.origin}/payment/callback?order_id=${orderId}`}
        />
      )}

      {paymentMethod === 'saved' && (
        <div>
          {/* PaymentMethodList component here */}
          <button
            onClick={handlePayWithSavedCard}
            disabled={!selectedPaymentMethodId || loading}
          >
            {loading ? 'Processing...' : 'Complete Purchase'}
          </button>
        </div>
      )}
    </div>
  );
}
```

---

## Best Practices

### 1. Always Use Idempotency Keys

```typescript
// ✅ Good: Unique key per operation
const idempotencyKey = `sale_${orderId}_${Date.now()}_${crypto.randomUUID().slice(0, 8)}`;

// ❌ Bad: Reusing same key prevents duplicate operations (but also prevents ALL operations)
const idempotencyKey = 'sale_key';
```

### 2. Handle BigInt for Amounts

```typescript
// ✅ Good: Use BigInt for precise amounts
const amountCents = BigInt(9999); // $99.99

// ❌ Bad: Using numbers may lose precision
const amountCents = 9999.99; // Type error + precision issues
```

### 3. Cache JWT Tokens

```typescript
// ✅ Good: Cache tokens and reuse until expiry
const token = await getAuthToken(); // Returns cached token if valid

// ❌ Bad: Generate new token for every request
const token = jwt.sign(...); // Unnecessary overhead
```

### 4. Use Environment Variables

```bash
# .env
REACT_APP_API_URL=http://localhost:8080
REACT_APP_MERCHANT_ID=your-merchant-id
REACT_APP_BACKEND_URL=http://localhost:3001/api
```

### 5. Implement Loading States

```typescript
{loading && <div className="spinner">Processing...</div>}
{!loading && <button>Submit Payment</button>}
```

---

## Troubleshooting

### Issue: "Unauthenticated" Error

**Cause:** JWT token is invalid, expired, or missing.

**Solution:**
1. Verify token is being added to request headers
2. Check token hasn't expired (use jwt.io to decode and inspect)
3. Ensure private key matches the public key registered with service
4. Verify `Authorization: Bearer TOKEN` header format

```bash
# Debug: Inspect token claims
echo "YOUR_JWT_TOKEN" | cut -d. -f2 | base64 -d | jq
```

### Issue: "Permission Denied" Error

**Cause:** Service lacks required scopes for the operation.

**Solution:**
1. Check token includes necessary scopes (e.g., `payment:create`)
2. Contact admin to update service permissions
3. Verify merchant access in `service_merchants` table

### Issue: Type Error with BigInt

**Cause:** JavaScript number used instead of BigInt for amounts.

**Solution:**
```typescript
// ✅ Correct
const amountCents = BigInt(9999);

// ❌ Wrong
const amountCents = 9999; // Type error
```

### Issue: CORS Error

**Cause:** Payment service doesn't allow requests from your origin.

**Solution:**
1. Check payment service CORS configuration
2. Ensure `REACT_APP_API_URL` is correct
3. For development, use proxy in `package.json`:

```json
{
  "proxy": "http://localhost:8080"
}
```

### Issue: Browser Post Callback Not Received

**Cause:** EPX cannot reach your callback URL.

**Solution:**
1. Verify callback URL is publicly accessible
2. Use ngrok for local development:

```bash
ngrok http 3000
# Use ngrok URL as return_url
```

**📖 More troubleshooting:** [Browser Post Reference](BROWSER_POST_FORM_SETUP.md#common-issues)

---

## Related Documentation

- **[Getting Started](GETTING_STARTED.md)** - Quick start integration guide
- **[Token Generation](TOKEN_GENERATION.md)** - JWT authentication setup
- **[Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md)** - PCI-compliant card tokenization
- **[API Specs](API_SPECS.md)** - Complete API reference
- **[Authentication Architecture](../development/AUTH.md)** - Detailed auth implementation

---

**Questions?** Check the [FAQ](../wiki-templates/FAQ.md) or review other integration guides.

**Last Updated:** 2025-01-20
