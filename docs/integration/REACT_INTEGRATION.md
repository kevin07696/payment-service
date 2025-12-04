# React Integration Guide

**Audience:** React developers
**Goal:** Integrate payment functionality using ConnectRPC

---

## Prerequisites

Before starting, complete these steps:

1. **Payment service running** - See [Setup Guide](../development/SETUP.md)
2. **Service registered** - See [Payment CLI Guide](ADMIN_CLI.md)
3. **JWT token generation working** - See [Token Generation Guide](TOKEN_GENERATION.md)

You should have:
- `service_credentials.json` with your RSA private key
- `merchant_id` UUID for transactions
- Working token endpoint on your backend

---

## Project Setup

### Install Dependencies

```bash
npm install @connectrpc/connect @connectrpc/connect-web
npm install --save-dev @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es
```

### Get Proto Files

```bash
mkdir -p proto/payment/v1 proto/payment_method/v1 proto/subscription/v1

# From payment service repo:
cp payment-service/proto/payment/v1/payment.proto proto/payment/v1/
cp payment-service/proto/payment_method/v1/payment_method.proto proto/payment_method/v1/
cp payment-service/proto/subscription/v1/subscription.proto proto/subscription/v1/
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
```

---

## ConnectRPC Client

**File:** `src/lib/payment-client.ts`

```typescript
import { createPromiseClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { PaymentService } from '../gen/payment/v1/payment_connect';
import { PaymentMethodService } from '../gen/payment_method/v1/payment_method_connect';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:8081';

/**
 * Fetch JWT token from your backend
 * See: docs/integration/TOKEN_GENERATION.md for backend implementation
 */
async function getAuthToken(): Promise<string> {
  const response = await fetch('/api/payment-token', {
    method: 'POST',
    credentials: 'include',
  });

  if (!response.ok) {
    throw new Error('Failed to fetch payment token');
  }

  const { token } = await response.json();
  return token;
}

function createAuthTransport() {
  return createConnectTransport({
    baseUrl: API_URL,
    interceptors: [
      (next) => async (req) => {
        const token = await getAuthToken();
        req.header.set('Authorization', `Bearer ${token}`);
        return next(req);
      },
    ],
  });
}

export const paymentClient = createPromiseClient(PaymentService, createAuthTransport());
export const paymentMethodClient = createPromiseClient(PaymentMethodService, createAuthTransport());
```

---

## React Hooks

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
        idempotencyKey: crypto.randomUUID(),
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
        idempotencyKey: crypto.randomUUID(),
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

---

## React Components

### Payment Form

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

## Browser Post (PCI-Compliant)

For card collection where card data never touches your server:

📖 **See:** [Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md)

```typescript
import React, { useState, useEffect } from 'react';

export function BrowserPostForm({ merchantId, amount, returnUrl }: Props) {
  const [formConfig, setFormConfig] = useState(null);

  useEffect(() => {
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
      {/* See BROWSER_POST_FORM_SETUP.md for all fields */}
      <input name="ACCOUNT_NBR" placeholder="Card Number" required />
      <input name="EXP_DATE" placeholder="MMYY" required />
      <input name="CVV2" placeholder="CVV" required />
      <button type="submit">Pay ${amount}</button>
    </form>
  );
}
```

---

## React-Specific Best Practices

### Use BigInt for Amounts

```typescript
// ✅ Correct - proto expects bigint
const amountCents = BigInt(9999);

// ❌ Wrong - type error
const amountCents = 9999;
```

### UUID Idempotency Keys

```typescript
// ✅ Correct - valid UUID format
const idempotencyKey = crypto.randomUUID();

// ❌ Wrong - not a UUID
const idempotencyKey = `sale_${orderId}_${Date.now()}`;
```

### Environment Variables

```bash
# .env
REACT_APP_API_URL=http://localhost:8081
```

### CORS (Development)

```json
// package.json
{
  "proxy": "http://localhost:8081"
}
```

---

## Related Documentation

| Topic | Guide |
|-------|-------|
| JWT Token Generation | [TOKEN_GENERATION.md](TOKEN_GENERATION.md) |
| Service/Merchant Setup | [ADMIN_CLI.md](ADMIN_CLI.md) |
| API Reference | [API_SPECS.md](API_SPECS.md) |
| Browser Post | [BROWSER_POST_FORM_SETUP.md](BROWSER_POST_FORM_SETUP.md) |
| Authentication | [AUTH.md](../development/AUTH.md) |
| Troubleshooting | [TOKEN_GENERATION.md#troubleshooting](TOKEN_GENERATION.md#troubleshooting) |
