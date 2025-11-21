# React Integration Guide

**Target Audience:** React developers integrating the payment service
**Framework:** React 18+ with TypeScript
**Protocol:** ConnectRPC + REST
**Goal:** Build type-safe payment flows in React applications

---

## Table of Contents

1. [Critical Warnings](#critical-warnings) ⚠️
2. [Quick Start](#quick-start)
3. [Setup and Configuration](#setup-and-configuration)
4. [Authentication](#authentication)
5. [Payment Operations](#payment-operations)
6. [Payment Methods](#payment-methods)
7. [Subscriptions](#subscriptions)
8. [Browser Post Integration](#browser-post-integration)
9. [Idempotency Implementation](#idempotency-implementation) ⭐
10. [Error Handling](#error-handling)
11. [TypeScript Types](#typescript-types)
12. [Complete Examples](#complete-examples)
13. [Best Practices](#best-practices)
14. [Quick Reference](#quick-reference)

---

## Critical Warnings

### ⚠️ Browser Post Callback: Always Return HTTP 200

**CRITICAL:** When handling Browser Post callbacks from EPX, ALWAYS return HTTP 200, even for errors.

```typescript
// ✅ CORRECT: Always return 200
export async function handleBrowserPostCallback(req, res) {
  try {
    // Process callback...
    return res.status(200).redirect("/payment/success");
  } catch (error) {
    // STILL return 200 to prevent EPX infinite retries
    return res.status(200).redirect("/payment/error");
  }
}

// ❌ WRONG: Returning non-200 causes EPX to retry infinitely
export async function handleBrowserPostCallback(req, res) {
  try {
    // Process callback...
  } catch (error) {
    return res.status(500).send("Error"); // EPX will retry forever!
  }
}
```

**Why:** EPX retries failed callbacks on network failures, non-200 responses, and timeouts. Returning non-200 creates an infinite retry loop.

### ⚠️ Use Unique Idempotency Keys

Every payment operation MUST have a unique idempotency key to prevent duplicate charges:

```typescript
// ✅ CORRECT: Unique key per operation
const idempotencyKey = `sale_${orderId}_${Date.now()}_${Math.random()}`;

// ❌ WRONG: Reusing the same key prevents all payments
const idempotencyKey = "sale_key";
```

### ⚠️ Use BigInt for Amounts

Always use `bigint` type for amounts to avoid precision loss:

```typescript
// ✅ CORRECT: Use bigint for cents
const amountCents = BigInt(9999); // $99.99

// ❌ WRONG: Numbers lose precision
const amountCents = 9999.99; // Type error + precision loss
```

### ⚠️ Database Constraints Required

Implement database UNIQUE constraints to handle race conditions:

```sql
-- Required for Browser Post idempotency
CREATE UNIQUE INDEX idx_transactions_epx_tran_nbr
ON transactions(epx_tran_nbr)
WHERE epx_tran_nbr IS NOT NULL;

-- Required for ConnectRPC idempotency
CREATE UNIQUE INDEX idx_transactions_idempotency_key
ON transactions(merchant_id, idempotency_key)
WHERE idempotency_key IS NOT NULL;
```

---

## Quick Start

### Installation

```bash
npm install @connectrpc/connect @connectrpc/connect-web
npm install --save-dev @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es
```

### Generate TypeScript Clients

```bash
# Download proto files from payment service
curl -o payment.proto http://localhost:8080/proto/payment/v1/payment.proto
curl -o payment_method.proto http://localhost:8080/proto/payment_method/v1/payment_method.proto
curl -o subscription.proto http://localhost:8080/proto/subscription/v1/subscription.proto

# Generate TypeScript clients
npx buf generate
```

### Basic Usage

```typescript
import { createPromiseClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { PaymentService } from "./gen/payment/v1/payment_connect";

// Create transport
const transport = createConnectTransport({
  baseUrl: "http://localhost:8080",
});

// Create client
const client = createPromiseClient(PaymentService, transport);

// Make request
const response = await client.authorize({
  merchantId: "1a20fff8-2cec-48e5-af49-87e501652913",
  customerId: "customer-123",
  amountCents: 9999n,
  currency: "USD",
  paymentMethodId: "pm-uuid",
  idempotencyKey: `auth_${Date.now()}`,
});
```

---

## Setup and Configuration

### Project Structure

```
src/
├── lib/
│   ├── payment-client.ts      # Payment service client setup
│   ├── auth.ts                # JWT authentication
│   └── types.ts               # TypeScript types
├── hooks/
│   ├── usePayment.ts          # Payment operations hook
│   ├── usePaymentMethods.ts   # Payment methods hook
│   └── useSubscription.ts     # Subscription hook
├── components/
│   ├── PaymentForm.tsx        # Payment form component
│   ├── BrowserPost.tsx        # Browser Post integration
│   └── PaymentMethodList.tsx  # Saved payment methods
└── gen/                       # Generated proto types
    ├── payment/v1/
    ├── payment_method/v1/
    └── subscription/v1/
```

### Configure buf.gen.yaml

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

### Payment Client Setup

**File:** `src/lib/payment-client.ts`

```typescript
import { createPromiseClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { PaymentService } from "../gen/payment/v1/payment_connect";
import { PaymentMethodService } from "../gen/payment_method/v1/payment_method_connect";
import { SubscriptionService } from "../gen/subscription/v1/subscription_connect";
import { getAuthToken } from "./auth";

const BASE_URL = process.env.REACT_APP_API_URL || "http://localhost:8080";

// Create transport with authentication
function createAuthTransport() {
  return createConnectTransport({
    baseUrl: BASE_URL,
    interceptors: [
      (next) => async (req) => {
        // Add JWT token to all requests
        const token = await getAuthToken();
        if (token) {
          req.header.set("Authorization", `Bearer ${token}`);
        }
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

export const subscriptionClient = createPromiseClient(
  SubscriptionService,
  createAuthTransport()
);
```

---

## Authentication

### JWT Token Management

**File:** `src/lib/auth.ts`

```typescript
import { jwtDecode } from "jwt-decode";

interface JWTClaims {
  merchant_id: string;
  service_id: string;
  scopes: string[];
  exp: number;
  iat: number;
}

let cachedToken: string | null = null;
let tokenExpiry: number = 0;

/**
 * Get or refresh JWT token
 */
export async function getAuthToken(): Promise<string> {
  // Return cached token if still valid
  if (cachedToken && Date.now() < tokenExpiry - 30000) {
    return cachedToken;
  }

  // Generate new token from your auth service
  const response = await fetch("/api/auth/token", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      merchantId: process.env.REACT_APP_MERCHANT_ID,
      apiKey: process.env.REACT_APP_API_KEY,
    }),
  });

  const { token } = await response.json();
  const decoded = jwtDecode<JWTClaims>(token);

  cachedToken = token;
  tokenExpiry = decoded.exp * 1000;

  return token;
}

/**
 * Clear cached token (use on logout)
 */
export function clearAuthToken() {
  cachedToken = null;
  tokenExpiry = 0;
}
```

---

## Payment Operations

### usePayment Hook

**File:** `src/hooks/usePayment.ts`

```typescript
import { useState } from "react";
import { paymentClient } from "../lib/payment-client";
import {
  AuthorizeRequest,
  CaptureRequest,
  SaleRequest,
  RefundRequest,
  VoidRequest,
} from "../gen/payment/v1/payment_pb";
import { PaymentResponse } from "../gen/payment/v1/payment_pb";

export function usePayment() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  /**
   * Authorize payment (hold funds)
   */
  const authorize = async (
    merchantId: string,
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
        currency: "USD",
        paymentMethodId,
        idempotencyKey: `auth_${Date.now()}_${Math.random()}`,
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
    merchantId: string,
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
        idempotencyKey: `capture_${Date.now()}_${Math.random()}`,
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
    merchantId: string,
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
        currency: "USD",
        paymentMethodId,
        idempotencyKey: `sale_${Date.now()}_${Math.random()}`,
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
    merchantId: string,
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
        idempotencyKey: `refund_${Date.now()}_${Math.random()}`,
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
  const voidPayment = async (
    merchantId: string,
    transactionId: string
  ): Promise<PaymentResponse | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.void({
        merchantId,
        transactionId,
        idempotencyKey: `void_${Date.now()}_${Math.random()}`,
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
   * Get transaction details
   */
  const getTransaction = async (transactionId: string) => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.getTransaction({
        transactionId,
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
   * List transactions
   */
  const listTransactions = async (
    merchantId: string,
    customerId?: string,
    limit: number = 50
  ) => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentClient.listTransactions({
        merchantId,
        customerId,
        limit,
      });

      return response.transactions;
    } catch (err) {
      setError(err as Error);
      return [];
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
    getTransaction,
    listTransactions,
    loading,
    error,
  };
}
```

### Payment Form Component

**File:** `src/components/PaymentForm.tsx`

```typescript
import React, { useState } from "react";
import { usePayment } from "../hooks/usePayment";

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
  const { sale, loading, error } = usePayment();
  const [amount, setAmount] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const amountCents = BigInt(Math.round(parseFloat(amount) * 100));

    const response = await sale(
      merchantId,
      customerId,
      amountCents,
      paymentMethodId,
      {
        source: "web-checkout",
        ip_address: window.location.hostname,
      }
    );

    if (response?.isApproved) {
      onSuccess(response.transactionId);
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
          min="0"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          required
          disabled={loading}
        />
      </div>

      <button type="submit" disabled={loading}>
        {loading ? "Processing..." : "Pay Now"}
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

---

## Payment Methods

### usePaymentMethods Hook

**File:** `src/hooks/usePaymentMethods.ts`

```typescript
import { useState, useEffect } from "react";
import { paymentMethodClient } from "../lib/payment-client";
import { PaymentMethod } from "../gen/payment_method/v1/payment_method_pb";

export function usePaymentMethods(merchantId: string, customerId: string) {
  const [paymentMethods, setPaymentMethods] = useState<PaymentMethod[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  /**
   * Load payment methods
   */
  const loadPaymentMethods = async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await paymentMethodClient.listPaymentMethods({
        merchantId,
        customerId,
      });

      setPaymentMethods(response.paymentMethods);
    } catch (err) {
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  };

  /**
   * Delete payment method
   */
  const deletePaymentMethod = async (paymentMethodId: string) => {
    setLoading(true);
    setError(null);

    try {
      await paymentMethodClient.deletePaymentMethod({
        paymentMethodId,
      });

      // Refresh list
      await loadPaymentMethods();
    } catch (err) {
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  };

  /**
   * Set default payment method
   */
  const setDefaultPaymentMethod = async (paymentMethodId: string) => {
    setLoading(true);
    setError(null);

    try {
      await paymentMethodClient.setDefaultPaymentMethod({
        merchantId,
        customerId,
        paymentMethodId,
      });

      // Refresh list
      await loadPaymentMethods();
    } catch (err) {
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  };

  /**
   * Store ACH account
   */
  const storeACHAccount = async (
    accountNumber: string,
    routingNumber: string,
    accountHolderName: string,
    accountType: "checking" | "savings"
  ) => {
    setLoading(true);
    setError(null);

    try {
      await paymentMethodClient.storeACHAccount({
        merchantId,
        customerId,
        accountNumber,
        routingNumber,
        accountHolderName,
        accountType,
        isDefault: false,
      });

      // Refresh list
      await loadPaymentMethods();
    } catch (err) {
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  };

  // Load on mount
  useEffect(() => {
    loadPaymentMethods();
  }, [merchantId, customerId]);

  return {
    paymentMethods,
    loading,
    error,
    loadPaymentMethods,
    deletePaymentMethod,
    setDefaultPaymentMethod,
    storeACHAccount,
  };
}
```

### Payment Method List Component

**File:** `src/components/PaymentMethodList.tsx`

```typescript
import React from "react";
import { usePaymentMethods } from "../hooks/usePaymentMethods";

interface PaymentMethodListProps {
  merchantId: string;
  customerId: string;
  onSelect: (paymentMethodId: string) => void;
}

export function PaymentMethodList({
  merchantId,
  customerId,
  onSelect,
}: PaymentMethodListProps) {
  const {
    paymentMethods,
    loading,
    error,
    deletePaymentMethod,
    setDefaultPaymentMethod,
  } = usePaymentMethods(merchantId, customerId);

  if (loading) return <div>Loading payment methods...</div>;
  if (error) return <div>Error: {error.message}</div>;

  return (
    <div>
      <h2>Saved Payment Methods</h2>
      {paymentMethods.length === 0 ? (
        <p>No saved payment methods</p>
      ) : (
        <ul>
          {paymentMethods.map((pm) => (
            <li key={pm.id}>
              <div>
                <strong>
                  {pm.paymentType === "credit_card" ? "Card" : "Bank Account"}
                </strong>
                {pm.isDefault && <span className="badge">Default</span>}
              </div>
              <div>
                {pm.brand && `${pm.brand} `}
                ending in {pm.lastFour}
              </div>
              <div>
                <button onClick={() => onSelect(pm.id)}>
                  Use This
                </button>
                {!pm.isDefault && (
                  <button onClick={() => setDefaultPaymentMethod(pm.id)}>
                    Make Default
                  </button>
                )}
                <button onClick={() => deletePaymentMethod(pm.id)}>
                  Delete
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

---

## Subscriptions

### useSubscription Hook

**File:** `src/hooks/useSubscription.ts`

```typescript
import { useState } from "react";
import { subscriptionClient } from "../lib/payment-client";
import {
  Subscription,
  SubscriptionInterval,
} from "../gen/subscription/v1/subscription_pb";

export function useSubscription() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  /**
   * Create subscription
   */
  const createSubscription = async (
    merchantId: string,
    customerId: string,
    paymentMethodId: string,
    amountCents: bigint,
    intervalUnit: SubscriptionInterval,
    intervalCount: number,
    planName: string
  ): Promise<Subscription | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await subscriptionClient.createSubscription({
        merchantId,
        customerId,
        paymentMethodId,
        amountCents,
        currency: "USD",
        intervalUnit,
        intervalCount,
        planName,
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
   * Cancel subscription
   */
  const cancelSubscription = async (
    subscriptionId: string,
    reason?: string
  ) => {
    setLoading(true);
    setError(null);

    try {
      await subscriptionClient.cancelSubscription({
        subscriptionId,
        reason,
      });
    } catch (err) {
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  };

  /**
   * Update subscription
   */
  const updateSubscription = async (
    subscriptionId: string,
    updates: {
      amountCents?: bigint;
      paymentMethodId?: string;
      intervalCount?: number;
    }
  ) => {
    setLoading(true);
    setError(null);

    try {
      await subscriptionClient.updateSubscription({
        subscriptionId,
        ...updates,
      });
    } catch (err) {
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  };

  /**
   * List customer subscriptions
   */
  const listSubscriptions = async (
    merchantId: string,
    customerId: string
  ) => {
    setLoading(true);
    setError(null);

    try {
      const response = await subscriptionClient.listCustomerSubscriptions({
        merchantId,
        customerId,
      });

      return response.subscriptions;
    } catch (err) {
      setError(err as Error);
      return [];
    } finally {
      setLoading(false);
    }
  };

  return {
    createSubscription,
    cancelSubscription,
    updateSubscription,
    listSubscriptions,
    loading,
    error,
  };
}
```

---

## Browser Post Integration

### Browser Post Form Component

**File:** `src/components/BrowserPost.tsx`

```typescript
import React, { useState, useEffect } from "react";

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
  transactionType: "SALE" | "AUTH" | "STORAGE";
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
  const [formConfig, setFormConfig] = useState<BrowserPostFormConfig | null>(
    null
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadFormConfig();
  }, []);

  const loadFormConfig = async () => {
    try {
      // Generate unique transaction ID
      const transactionId = crypto.randomUUID();

      // Build query params
      const params = new URLSearchParams({
        transaction_id: transactionId,
        merchant_id: merchantId,
        amount: amount,
        transaction_type: transactionType,
        return_url: returnUrl,
      });

      if (customerId) {
        params.append("customer_id", customerId);
      }

      // Get form configuration from backend
      const response = await fetch(
        `http://localhost:8081/api/v1/payments/browser-post/form?${params}`
      );

      if (!response.ok) {
        throw new Error("Failed to load payment form");
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
        <input type="hidden" name="EPX_TRAN_NBR" value={formConfig.epxTranNbr} />
        <input type="hidden" name="TRAN_TYPE" value={transactionType === "SALE" ? "CCE1" : "CCE8"} />
        <input type="hidden" name="AMOUNT" value={amount} />
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
          <label htmlFor="CARDHOLDER_NAME">Name on Card</label>
          <input
            id="CARDHOLDER_NAME"
            name="CARDHOLDER_NAME"
            type="text"
            placeholder="John Doe"
            required
            autoComplete="cc-name"
          />
        </div>

        <div className="form-group">
          <label htmlFor="ZIP_CODE">ZIP Code</label>
          <input
            id="ZIP_CODE"
            name="ZIP_CODE"
            type="text"
            maxLength={10}
            placeholder="12345"
            required
            autoComplete="postal-code"
          />
        </div>

        <button type="submit" className="submit-button">
          {transactionType === "STORAGE" ? "Save Card" : `Pay $${amount}`}
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

### Browser Post Callback Handler

**File:** `src/pages/PaymentCallback.tsx`

```typescript
import React, { useEffect, useState } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";

export function PaymentCallback() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [status, setStatus] = useState<"processing" | "success" | "failed">(
    "processing"
  );

  useEffect(() => {
    processCallback();
  }, []);

  const processCallback = async () => {
    // Extract EPX response parameters
    const authResp = searchParams.get("AUTH_RESP");
    const authGuid = searchParams.get("AUTH_GUID");
    const guid = searchParams.get("GUID");
    const tranNbr = searchParams.get("TRAN_NBR");
    const authCode = searchParams.get("AUTH_CODE");
    const authAmount = searchParams.get("AUTH_AMOUNT");
    const authCardType = searchParams.get("AUTH_CARD_TYPE");
    const authCardNbr = searchParams.get("AUTH_CARD_NBR");

    // Check if payment was approved
    if (authResp === "00") {
      setStatus("success");

      // Store transaction in your database
      await fetch("/api/transactions/save", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          transactionId: tranNbr,
          bricToken: guid || authGuid,
          amount: authAmount,
          cardBrand: authCardType,
          lastFour: authCardNbr?.slice(-4),
          authCode,
        }),
      });

      // Redirect to success page after 2 seconds
      setTimeout(() => {
        navigate(`/orders/${tranNbr}?success=true`);
      }, 2000);
    } else {
      setStatus("failed");

      // Redirect to checkout with error after 2 seconds
      setTimeout(() => {
        navigate(`/checkout?error=payment_failed&code=${authResp}`);
      }, 2000);
    }
  };

  return (
    <div className="callback-container">
      {status === "processing" && (
        <div>
          <div className="spinner" />
          <p>Processing your payment...</p>
        </div>
      )}

      {status === "success" && (
        <div className="success">
          <h1>✅ Payment Successful!</h1>
          <p>Redirecting to your order...</p>
        </div>
      )}

      {status === "failed" && (
        <div className="error">
          <h1>❌ Payment Failed</h1>
          <p>Redirecting back to checkout...</p>
        </div>
      )}
    </div>
  );
}
```

---

## Idempotency Implementation

### Understanding Idempotency

**Idempotency** ensures that making the same request multiple times has the same effect as making it once. This is critical for payment operations to prevent duplicate charges.

### Idempotency Key Strategy

**Key Format:** `{operation}_{timestamp}_{random}`

```typescript
// Generate unique idempotency key
function generateIdempotencyKey(operation: string): string {
  const timestamp = Date.now();
  const random = Math.random().toString(36).substr(2, 9);
  return `${operation}_${timestamp}_${random}`;
}

// Examples
const authKey = generateIdempotencyKey("auth");     // "auth_1706123456789_k3j9d2x7q"
const captureKey = generateIdempotencyKey("capture"); // "capture_1706123456790_m8n4p1z5w"
```

### Frontend Idempotency: Preventing Double-Clicks

**File:** `src/hooks/useIdempotentRequest.ts`

```typescript
import { useState, useRef } from "react";

interface IdempotentRequest<T> {
  execute: () => Promise<T>;
  loading: boolean;
  error: Error | null;
}

/**
 * Hook to prevent duplicate requests from double-clicks
 */
export function useIdempotentRequest<T>(
  requestFn: () => Promise<T>
): IdempotentRequest<T> {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const requestInProgress = useRef(false);

  const execute = async (): Promise<T> => {
    // Prevent duplicate execution
    if (requestInProgress.current) {
      throw new Error("Request already in progress");
    }

    requestInProgress.current = true;
    setLoading(true);
    setError(null);

    try {
      const result = await requestFn();
      return result;
    } catch (err) {
      setError(err as Error);
      throw err;
    } finally {
      setLoading(false);
      requestInProgress.current = false;
    }
  };

  return { execute, loading, error };
}
```

**Usage in Components:**

```typescript
import { useIdempotentRequest } from "../hooks/useIdempotentRequest";

function CheckoutButton({ onPay }: { onPay: () => Promise<void> }) {
  const { execute, loading } = useIdempotentRequest(onPay);

  return (
    <button onClick={execute} disabled={loading}>
      {loading ? "Processing..." : "Pay Now"}
    </button>
  );
}
```

### Backend Idempotency: Browser Post Callback

The Browser Post callback is the most critical place for idempotency because **EPX will retry failed callbacks**.

**Problem:** EPX retries callbacks on:
- Network failures
- Non-200 HTTP responses
- Timeouts (> 30 seconds)

**Solution:** Store transaction ID and return cached response for duplicates.

**File:** `src/api/browser-post-callback.ts` (Backend/BFF)

```typescript
import { Request, Response } from "express";

interface CallbackData {
  AUTH_RESP: string;
  AUTH_GUID?: string;
  GUID?: string;
  TRAN_NBR: string;
  AUTH_CODE?: string;
  AUTH_AMOUNT: string;
  AUTH_CARD_TYPE?: string;
  AUTH_CARD_NBR?: string;
}

/**
 * Idempotent Browser Post callback handler
 */
export async function handleBrowserPostCallback(req: Request, res: Response) {
  const data: CallbackData = req.body;
  const transactionId = data.TRAN_NBR;

  // STEP 1: Check if we've already processed this callback
  const existing = await db.query(
    "SELECT * FROM transactions WHERE epx_tran_nbr = $1",
    [transactionId]
  );

  if (existing.rows.length > 0) {
    // Already processed - return same response
    console.log(`Duplicate callback for transaction ${transactionId}`);
    return res.redirect(
      `/payment/success?transaction_id=${transactionId}&duplicate=true`
    );
  }

  // STEP 2: Use database transaction with INSERT ... ON CONFLICT
  try {
    const result = await db.query(`
      INSERT INTO transactions (
        id,
        epx_tran_nbr,
        merchant_id,
        customer_id,
        amount_cents,
        currency,
        status,
        epx_auth_guid,
        epx_storage_guid,
        auth_code,
        card_brand,
        card_last_four,
        created_at
      ) VALUES (
        gen_random_uuid(),
        $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW()
      )
      ON CONFLICT (epx_tran_nbr) DO NOTHING
      RETURNING id
    `, [
      transactionId,
      req.query.merchant_id,
      req.query.customer_id,
      parseFloat(data.AUTH_AMOUNT) * 100,
      "USD",
      data.AUTH_RESP === "00" ? "approved" : "declined",
      data.AUTH_GUID,
      data.GUID,
      data.AUTH_CODE,
      data.AUTH_CARD_TYPE,
      data.AUTH_CARD_NBR?.slice(-4),
    ]);

    // If ON CONFLICT triggered, this was a race condition
    if (result.rows.length === 0) {
      console.log(`Race condition detected for ${transactionId}`);
      return res.redirect(
        `/payment/success?transaction_id=${transactionId}&duplicate=true`
      );
    }

    // STEP 3: Return success (always 200, even for declined transactions)
    if (data.AUTH_RESP === "00") {
      return res.redirect(
        `/payment/success?transaction_id=${transactionId}`
      );
    } else {
      return res.redirect(
        `/payment/failed?transaction_id=${transactionId}&code=${data.AUTH_RESP}`
      );
    }
  } catch (error) {
    console.error("Callback processing error:", error);

    // IMPORTANT: Still return 200 to prevent EPX retries
    // Log the error for investigation
    return res.redirect(
      `/payment/error?transaction_id=${transactionId}`
    );
  }
}
```

**Database Schema for Idempotency:**

```sql
-- Unique constraint on EPX transaction number
CREATE UNIQUE INDEX idx_transactions_epx_tran_nbr
ON transactions(epx_tran_nbr)
WHERE epx_tran_nbr IS NOT NULL;
```

### Backend Idempotency: ConnectRPC Endpoints

The payment service handles idempotency for ConnectRPC requests automatically, but your frontend/BFF layer should implement request deduplication.

**File:** `src/api/payment-proxy.ts` (Backend/BFF)

```typescript
import { paymentClient } from "../lib/payment-client";

// Cache for tracking in-flight requests
const requestCache = new Map<string, Promise<any>>();

/**
 * Idempotent payment request wrapper
 */
export async function idempotentPaymentRequest<T>(
  idempotencyKey: string,
  requestFn: () => Promise<T>
): Promise<T> {
  // Check if this exact request is already in-flight
  const inFlight = requestCache.get(idempotencyKey);
  if (inFlight) {
    console.log(`Returning in-flight request for key: ${idempotencyKey}`);
    return inFlight;
  }

  // Execute request and cache the promise
  const promise = requestFn()
    .finally(() => {
      // Remove from cache after completion (success or failure)
      requestCache.delete(idempotencyKey);
    });

  requestCache.set(idempotencyKey, promise);
  return promise;
}

/**
 * Example: Idempotent authorize endpoint
 */
export async function authorizePayment(
  merchantId: string,
  customerId: string,
  amountCents: bigint,
  paymentMethodId: string,
  idempotencyKey: string
) {
  return idempotentPaymentRequest(idempotencyKey, async () => {
    return await paymentClient.authorize({
      merchantId,
      customerId,
      amountCents,
      currency: "USD",
      paymentMethodId,
      idempotencyKey,
    });
  });
}
```

### Idempotency Patterns by Endpoint Type

#### 1. Authorize/Sale (Initial Payment)

```typescript
// Frontend generates unique key per payment attempt
const idempotencyKey = `sale_${orderId}_${Date.now()}_${random}`;

// If user clicks "Pay" multiple times, same key = same transaction
const response = await paymentClient.sale({
  merchantId,
  customerId,
  amountCents,
  currency: "USD",
  paymentMethodId,
  idempotencyKey, // CRITICAL: Must be unique per order, stable across retries
});
```

**Key Strategy:**
- Include `orderId` in key to link to business entity
- Safe to retry with same key = returns existing transaction
- Different `orderId` = different key = new transaction

#### 2. Capture (Following Authorization)

```typescript
// Generate capture key based on auth transaction
const idempotencyKey = `capture_${authTransactionId}_${Date.now()}_${random}`;

const response = await paymentClient.capture({
  merchantId,
  transactionId: authTransactionId,
  amountCents, // Can be less than auth for partial capture
  idempotencyKey,
});
```

**Key Strategy:**
- Include parent `authTransactionId` to link capture to auth
- Can have multiple partial captures with different keys
- Same key = same capture (prevents double-capture)

#### 3. Refund (Following Capture)

```typescript
// Generate refund key based on captured transaction
const idempotencyKey = `refund_${captureTransactionId}_${Date.now()}_${random}`;

const response = await paymentClient.refund({
  merchantId,
  transactionId: captureTransactionId,
  amountCents,
  reason: "Customer requested refund",
  idempotencyKey,
});
```

**Key Strategy:**
- Include parent `captureTransactionId`
- Can have multiple partial refunds with different keys
- Same key = same refund (prevents double-refund)

#### 4. Subscription Creation

```typescript
// Generate subscription key based on customer + plan
const idempotencyKey = `sub_${customerId}_${planId}_${Date.now()}_${random}`;

const response = await subscriptionClient.createSubscription({
  merchantId,
  customerId,
  paymentMethodId,
  amountCents,
  intervalUnit: "MONTH",
  intervalCount: 1,
  planName: "Pro Plan",
  idempotencyKey, // CRITICAL: Prevents duplicate subscriptions
});
```

**Key Strategy:**
- Include `customerId` and `planId` to identify unique subscription
- Safe to retry = returns existing subscription
- Prevents user from accidentally creating duplicate subscriptions

### Testing Idempotency

**Test 1: Double-Click Prevention**

```typescript
// Simulate rapid double-click
test("prevents double payment submission", async () => {
  const { getByText } = render(<PaymentForm {...props} />);
  const payButton = getByText("Pay Now");

  // Click twice rapidly
  fireEvent.click(payButton);
  fireEvent.click(payButton); // Second click should be ignored

  // Should only make ONE API call
  await waitFor(() => {
    expect(mockPaymentClient.sale).toHaveBeenCalledTimes(1);
  });
});
```

**Test 2: Browser Post Callback Retry**

```bash
# Simulate EPX retry by sending same callback twice
curl -X POST http://localhost:3000/api/browser-post/callback \
  -d "TRAN_NBR=1234567890&AUTH_RESP=00&AUTH_AMOUNT=99.99"

# Send again (simulating EPX retry after timeout)
curl -X POST http://localhost:3000/api/browser-post/callback \
  -d "TRAN_NBR=1234567890&AUTH_RESP=00&AUTH_AMOUNT=99.99"

# Should return same result, create only ONE database record
```

**Test 3: ConnectRPC Idempotency**

```typescript
test("returns same transaction for duplicate idempotency key", async () => {
  const idempotencyKey = "test_12345";

  // Make first request
  const response1 = await paymentClient.sale({
    merchantId: "merchant-1",
    customerId: "customer-1",
    amountCents: 9999n,
    currency: "USD",
    paymentMethodId: "pm-1",
    idempotencyKey,
  });

  // Make second request with SAME idempotency key
  const response2 = await paymentClient.sale({
    merchantId: "merchant-1",
    customerId: "customer-1",
    amountCents: 9999n,
    currency: "USD",
    paymentMethodId: "pm-1",
    idempotencyKey, // Same key
  });

  // Should return SAME transaction
  expect(response1.transactionId).toBe(response2.transactionId);
});
```

### Idempotency Checklist

**Frontend:**
- [ ] Generate unique idempotency keys per request
- [ ] Disable buttons while request is in-flight
- [ ] Cache in-flight requests to prevent concurrent duplicates
- [ ] Include business entity ID (orderId, subscriptionId) in key

**Backend/BFF:**
- [ ] Validate idempotency keys before calling payment service
- [ ] Use database constraints (UNIQUE on epx_tran_nbr)
- [ ] Use INSERT ... ON CONFLICT for Browser Post callbacks
- [ ] Always return 200 to EPX (even for errors) to prevent retries
- [ ] Cache in-flight requests by idempotency key

**Database:**
- [ ] UNIQUE constraint on epx_tran_nbr
- [ ] UNIQUE constraint on idempotency_key per merchant
- [ ] Consider TTL for old idempotency keys (e.g., 24 hours)

**Testing:**
- [ ] Test double-click prevention
- [ ] Test duplicate Browser Post callbacks
- [ ] Test same idempotency key returns same transaction
- [ ] Test different idempotency keys create different transactions

---

## Error Handling

### Error Handling Utility

**File:** `src/lib/error-handler.ts`

```typescript
import { ConnectError } from "@connectrpc/connect";

export interface PaymentError {
  code: string;
  message: string;
  userMessage: string;
  retryable: boolean;
}

/**
 * Parse ConnectRPC error into user-friendly format
 */
export function parsePaymentError(error: unknown): PaymentError {
  if (error instanceof ConnectError) {
    switch (error.code) {
      case "unauthenticated":
        return {
          code: "AUTH_FAILED",
          message: error.message,
          userMessage: "Authentication failed. Please log in again.",
          retryable: false,
        };

      case "permission_denied":
        return {
          code: "PERMISSION_DENIED",
          message: error.message,
          userMessage: "You don't have permission to perform this action.",
          retryable: false,
        };

      case "invalid_argument":
        return {
          code: "INVALID_INPUT",
          message: error.message,
          userMessage: "Please check your input and try again.",
          retryable: false,
        };

      case "resource_exhausted":
        return {
          code: "RATE_LIMIT",
          message: error.message,
          userMessage: "Too many requests. Please wait a moment and try again.",
          retryable: true,
        };

      case "unavailable":
        return {
          code: "SERVICE_UNAVAILABLE",
          message: error.message,
          userMessage: "Service temporarily unavailable. Please try again later.",
          retryable: true,
        };

      default:
        return {
          code: error.code,
          message: error.message,
          userMessage: "An error occurred. Please try again.",
          retryable: true,
        };
    }
  }

  return {
    code: "UNKNOWN_ERROR",
    message: (error as Error).message || "Unknown error",
    userMessage: "An unexpected error occurred. Please try again.",
    retryable: true,
  };
}

/**
 * Retry logic with exponential backoff
 */
export async function retryWithBackoff<T>(
  fn: () => Promise<T>,
  maxRetries: number = 3,
  initialDelay: number = 1000
): Promise<T> {
  let lastError: unknown;

  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error;
      const paymentError = parsePaymentError(error);

      // Don't retry if error is not retryable
      if (!paymentError.retryable) {
        throw error;
      }

      // Wait before retrying (exponential backoff)
      if (i < maxRetries - 1) {
        const delay = initialDelay * Math.pow(2, i);
        await new Promise((resolve) => setTimeout(resolve, delay));
      }
    }
  }

  throw lastError;
}
```

### Error Display Component

**File:** `src/components/ErrorDisplay.tsx`

```typescript
import React from "react";
import { parsePaymentError } from "../lib/error-handler";

interface ErrorDisplayProps {
  error: Error | null;
  onRetry?: () => void;
}

export function ErrorDisplay({ error, onRetry }: ErrorDisplayProps) {
  if (!error) return null;

  const paymentError = parsePaymentError(error);

  return (
    <div className="error-container">
      <div className="error-icon">⚠️</div>
      <h3>Payment Error</h3>
      <p>{paymentError.userMessage}</p>

      {paymentError.retryable && onRetry && (
        <button onClick={onRetry} className="retry-button">
          Try Again
        </button>
      )}

      <details>
        <summary>Technical Details</summary>
        <pre>
          Code: {paymentError.code}
          {"\n"}
          Message: {paymentError.message}
        </pre>
      </details>
    </div>
  );
}
```

---

## TypeScript Types

### Common Types

**File:** `src/lib/types.ts`

```typescript
/**
 * Merchant configuration
 */
export interface MerchantConfig {
  merchantId: string;
  merchantName: string;
  environment: "sandbox" | "production";
}

/**
 * Customer information
 */
export interface Customer {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
}

/**
 * Amount helper (converts dollars to cents)
 */
export function dollarsToCents(dollars: number): bigint {
  return BigInt(Math.round(dollars * 100));
}

/**
 * Amount helper (converts cents to dollars)
 */
export function centsToDollars(cents: bigint): number {
  return Number(cents) / 100;
}

/**
 * Format currency
 */
export function formatCurrency(cents: bigint, currency: string = "USD"): string {
  const dollars = centsToDollars(cents);
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
  }).format(dollars);
}

/**
 * Generate idempotency key
 */
export function generateIdempotencyKey(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}
```

---

## Complete Examples

### E-Commerce Checkout Flow

```typescript
import React, { useState } from "react";
import { usePayment } from "./hooks/usePayment";
import { PaymentMethodList } from "./components/PaymentMethodList";
import { BrowserPost } from "./components/BrowserPost";
import { dollarsToCents, formatCurrency } from "./lib/types";

interface CheckoutProps {
  merchantId: string;
  customerId: string;
  orderTotal: number;
}

export function Checkout({ merchantId, customerId, orderTotal }: CheckoutProps) {
  const { sale, loading, error } = usePayment();
  const [paymentMethod, setPaymentMethod] = useState<"saved" | "new">("saved");
  const [selectedPaymentMethodId, setSelectedPaymentMethodId] = useState<string | null>(null);

  const handlePayWithSavedCard = async () => {
    if (!selectedPaymentMethodId) return;

    const response = await sale(
      merchantId,
      customerId,
      dollarsToCents(orderTotal),
      selectedPaymentMethodId,
      {
        order_id: `ORDER-${Date.now()}`,
        source: "web-checkout",
      }
    );

    if (response?.isApproved) {
      window.location.href = `/orders/${response.transactionId}?success=true`;
    }
  };

  return (
    <div className="checkout">
      <h1>Checkout</h1>
      <div className="order-summary">
        <h2>Order Total: {formatCurrency(dollarsToCents(orderTotal))}</h2>
      </div>

      <div className="payment-options">
        <label>
          <input
            type="radio"
            value="saved"
            checked={paymentMethod === "saved"}
            onChange={() => setPaymentMethod("saved")}
          />
          Use Saved Payment Method
        </label>
        <label>
          <input
            type="radio"
            value="new"
            checked={paymentMethod === "new"}
            onChange={() => setPaymentMethod("new")}
          />
          Add New Card
        </label>
      </div>

      {paymentMethod === "saved" && (
        <>
          <PaymentMethodList
            merchantId={merchantId}
            customerId={customerId}
            onSelect={setSelectedPaymentMethodId}
          />
          <button
            onClick={handlePayWithSavedCard}
            disabled={!selectedPaymentMethodId || loading}
          >
            {loading ? "Processing..." : "Complete Purchase"}
          </button>
        </>
      )}

      {paymentMethod === "new" && (
        <BrowserPost
          merchantId={merchantId}
          amount={orderTotal.toFixed(2)}
          transactionType="SALE"
          customerId={customerId}
          returnUrl={`${window.location.origin}/payment/callback`}
        />
      )}

      {error && <div className="error">{error.message}</div>}
    </div>
  );
}
```

### Subscription Management

```typescript
import React, { useEffect, useState } from "react";
import { useSubscription } from "./hooks/useSubscription";
import { usePaymentMethods } from "./hooks/usePaymentMethods";
import { SubscriptionInterval } from "./gen/subscription/v1/subscription_pb";
import { dollarsToCents, formatCurrency } from "./lib/types";

interface SubscriptionManagerProps {
  merchantId: string;
  customerId: string;
}

export function SubscriptionManager({
  merchantId,
  customerId,
}: SubscriptionManagerProps) {
  const { createSubscription, listSubscriptions, cancelSubscription, loading } =
    useSubscription();
  const { paymentMethods } = usePaymentMethods(merchantId, customerId);
  const [subscriptions, setSubscriptions] = useState<any[]>([]);

  useEffect(() => {
    loadSubscriptions();
  }, []);

  const loadSubscriptions = async () => {
    const subs = await listSubscriptions(merchantId, customerId);
    setSubscriptions(subs);
  };

  const handleCreateSubscription = async () => {
    const defaultPaymentMethod = paymentMethods.find((pm) => pm.isDefault);
    if (!defaultPaymentMethod) {
      alert("Please add a payment method first");
      return;
    }

    await createSubscription(
      merchantId,
      customerId,
      defaultPaymentMethod.id,
      dollarsToCents(9.99),
      SubscriptionInterval.MONTH,
      1,
      "Monthly Pro Plan"
    );

    await loadSubscriptions();
  };

  const handleCancelSubscription = async (subscriptionId: string) => {
    if (!confirm("Are you sure you want to cancel this subscription?")) return;

    await cancelSubscription(subscriptionId, "Customer requested cancellation");
    await loadSubscriptions();
  };

  return (
    <div className="subscription-manager">
      <h1>My Subscriptions</h1>

      <button onClick={handleCreateSubscription} disabled={loading}>
        Subscribe to Pro Plan ($9.99/month)
      </button>

      <div className="subscriptions-list">
        {subscriptions.length === 0 ? (
          <p>No active subscriptions</p>
        ) : (
          subscriptions.map((sub) => (
            <div key={sub.id} className="subscription-card">
              <h3>{sub.planName}</h3>
              <p>
                {formatCurrency(sub.amountCents)} / {sub.intervalCount}{" "}
                {sub.intervalUnit.toLowerCase()}
              </p>
              <p>Status: {sub.status}</p>
              <p>Next billing: {new Date(sub.nextBillingDate).toLocaleDateString()}</p>
              <button onClick={() => handleCancelSubscription(sub.id)}>
                Cancel Subscription
              </button>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
```

---

## Quick Reference

### Common Operations Cheat Sheet

#### 1. Process a One-Time Payment (Sale)

```typescript
import { usePayment } from './hooks/usePayment';
import { dollarsToCents } from './lib/types';

function MyComponent() {
  const { sale, loading, error } = usePayment();

  const handlePayment = async () => {
    const response = await sale(
      "merchant-id",
      "customer-id",
      dollarsToCents(99.99),      // $99.99
      "payment-method-id",
      { order_id: "ORDER-123" }   // Optional metadata
    );

    if (response?.isApproved) {
      // Payment successful
      console.log("Transaction ID:", response.transactionId);
    }
  };

  return <button onClick={handlePayment} disabled={loading}>Pay Now</button>;
}
```

#### 2. Tokenize Card with Browser Post

```typescript
import { BrowserPost } from './components/BrowserPost';

function SaveCardForm() {
  return (
    <BrowserPost
      merchantId="merchant-id"
      amount="0.00"
      transactionType="STORAGE"  // Just tokenize, no charge
      customerId="customer-id"
      returnUrl="https://yourapp.com/payment/callback"
    />
  );
}
```

#### 3. Create a Subscription

```typescript
import { useSubscription } from './hooks/useSubscription';
import { SubscriptionInterval } from './gen/subscription/v1/subscription_pb';
import { dollarsToCents } from './lib/types';

function SubscribeButton() {
  const { createSubscription, loading } = useSubscription();

  const handleSubscribe = async () => {
    const subscription = await createSubscription(
      "merchant-id",
      "customer-id",
      "payment-method-id",
      dollarsToCents(9.99),                // $9.99/month
      SubscriptionInterval.MONTH,
      1,                                   // Every 1 month
      "Pro Plan"
    );

    if (subscription) {
      console.log("Subscription created:", subscription.id);
    }
  };

  return <button onClick={handleSubscribe} disabled={loading}>Subscribe</button>;
}
```

#### 4. List Saved Payment Methods

```typescript
import { usePaymentMethods } from './hooks/usePaymentMethods';

function SavedCards() {
  const { paymentMethods, loading, deletePaymentMethod } =
    usePaymentMethods("merchant-id", "customer-id");

  if (loading) return <div>Loading...</div>;

  return (
    <ul>
      {paymentMethods.map(pm => (
        <li key={pm.id}>
          {pm.brand} ending in {pm.lastFour}
          {pm.isDefault && <span>Default</span>}
          <button onClick={() => deletePaymentMethod(pm.id)}>Delete</button>
        </li>
      ))}
    </ul>
  );
}
```

#### 5. Refund a Payment

```typescript
const { refund } = usePayment();

const handleRefund = async () => {
  const response = await refund(
    "merchant-id",
    "transaction-id",
    dollarsToCents(99.99),         // Full refund
    "Customer requested refund"
  );

  if (response?.isApproved) {
    console.log("Refund successful");
  }
};
```

#### 6. Handle Browser Post Callback (Backend)

```typescript
// Express.js route
app.post('/payment/callback', async (req, res) => {
  const { AUTH_RESP, AUTH_GUID, TRAN_NBR, AUTH_AMOUNT } = req.body;

  // Check if already processed
  const existing = await db.query(
    "SELECT * FROM transactions WHERE epx_tran_nbr = $1",
    [TRAN_NBR]
  );

  if (existing.rows.length > 0) {
    // Already processed, return success
    return res.status(200).redirect(`/payment/success?transaction_id=${TRAN_NBR}`);
  }

  // Save transaction with ON CONFLICT protection
  try {
    await db.query(`
      INSERT INTO transactions (epx_tran_nbr, bric_token, amount_cents, status)
      VALUES ($1, $2, $3, $4)
      ON CONFLICT (epx_tran_nbr) DO NOTHING
    `, [TRAN_NBR, AUTH_GUID, AUTH_AMOUNT, AUTH_RESP === '00' ? 'approved' : 'declined']);

    // CRITICAL: Always return 200 to prevent EPX retries
    return res.status(200).redirect(
      AUTH_RESP === '00'
        ? `/payment/success?transaction_id=${TRAN_NBR}`
        : `/payment/failed?transaction_id=${TRAN_NBR}`
    );
  } catch (error) {
    // STILL return 200 even on error
    return res.status(200).redirect(`/payment/error?transaction_id=${TRAN_NBR}`);
  }
});
```

#### 7. Generate Idempotency Keys

```typescript
// For payment operations
const idempotencyKey = `sale_${orderId}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

// For captures
const idempotencyKey = `capture_${authTransactionId}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

// For refunds
const idempotencyKey = `refund_${captureTransactionId}_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
```

---

## Best Practices

### 1. Always Use Idempotency Keys

```typescript
// Good: Unique idempotency key per operation
const idempotencyKey = `sale_${Date.now()}_${Math.random()}`;

// Bad: Reusing keys
const idempotencyKey = "sale_key"; // Will prevent duplicate charges
```

### 2. Handle BigInt for Amounts

```typescript
// Good: Use BigInt for precise amounts
const amountCents = BigInt(9999); // $99.99

// Bad: Using numbers (may lose precision)
const amountCents = 9999.99; // Type error
```

### 3. Validate Input Before Submission

```typescript
// Validate amount
if (amountCents <= 0) {
  throw new Error("Amount must be positive");
}

// Validate currency
if (!["USD"].includes(currency)) {
  throw new Error("Invalid currency");
}
```

### 4. Use Environment Variables

```typescript
// .env
REACT_APP_API_URL=http://localhost:8080
REACT_APP_MERCHANT_ID=your-merchant-id
REACT_APP_API_KEY=your-api-key

// config.ts
export const config = {
  apiUrl: process.env.REACT_APP_API_URL,
  merchantId: process.env.REACT_APP_MERCHANT_ID,
  apiKey: process.env.REACT_APP_API_KEY,
};
```

### 5. Implement Loading States

```typescript
// Show loading indicator during API calls
{loading && <div className="spinner">Processing...</div>}
{!loading && <button>Submit Payment</button>}
```

### 6. Cache JWT Tokens

```typescript
// Reuse tokens until they expire
const cachedToken = localStorage.getItem("auth_token");
const tokenExpiry = localStorage.getItem("token_expiry");

if (cachedToken && Date.now() < Number(tokenExpiry)) {
  return cachedToken;
}
```

---

## Next Steps

- **[API Reference](API_SPECS.md)** - Complete endpoint documentation
- **[Authentication Guide](AUTH.md)** - JWT token generation
- **[Browser Post Reference](BROWSER_POST_REFERENCE.md)** - Detailed Browser Post integration
- **[Error Codes](API_SPECS.md#error-handling)** - Complete error code reference

---

**Questions?** Check the [FAQ](../wiki-templates/FAQ.md) or review the [Integration Guide](INTEGRATION_GUIDE.md).
