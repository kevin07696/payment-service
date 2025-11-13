# Payment Service API Specification

**Version**: 1.0.0 | **Date**: 2025-11-11

## Overview

| Service | HTTP REST (8081) | gRPC (8080) | Access |
|---------|:----------------:|:-----------:|--------|
| **Payment** | ✅ | ✅ | Public |
| **Payment Method** | ✅ | ✅ | Public |
| **Subscription** | ✅ | ✅ | Public |
| **Browser Post** | ✅ | ❌ | Public |
| **Cron/Health** | ✅ | ❌ | Public (cron=protected) |
| **Agent** | ❌ | ✅ | Internal/admin only |
| **Chargeback** | ❌ | ✅ | Internal/admin only |

**Notes**:
- HTTP REST auto-generated from gRPC via gRPC-Gateway
- Agent & Chargeback services intentionally gRPC-only (no HTTP gateway)

---

## Base URLs & Ports

**Development**: `http://localhost:8081` (HTTP) | `localhost:8080` (gRPC)

**Ports**:
- 8080: gRPC (binary)
- 8081: HTTP REST + Browser Post + Cron

---

## Authentication

**Multi-Tenant**: All requests require `agent_id` parameter (identifies merchant)

**Agent Credentials** (stored in DB):
- EPX: cust_nbr, merch_nbr, dba_nbr, terminal_nbr
- MAC secret (in secret manager)

**Cron Security**: `/cron/*` endpoints require `X-Cron-Secret` header (set via `CRON_SECRET` env var)

---

## HTTP REST APIs

### Payment APIs

Base path: `/api/v1/payments`

#### 1. Authorize Payment

Holds funds without capturing. Use for pre-authorization flows.

**Endpoint**: `POST /api/v1/payments/authorize`

**Request Body**:
```json
{
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "amount": "99.99",
  "currency": "USD",
  "payment_method_id": "pm-uuid-here",
  "idempotency_key": "order-12345",
  "metadata": {
    "order_id": "ORDER-12345",
    "notes": "VIP customer"
  }
}
```

**Alternative - One-Time Token**:
```json
{
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "amount": "99.99",
  "currency": "USD",
  "payment_token": "epx-auth-guid-token",
  "idempotency_key": "order-12345"
}
```

**Response** (200 OK):
```json
{
  "transaction_id": "tx-uuid-here",
  "group_id": "grp-uuid-here",
  "amount": "99.99",
  "currency": "USD",
  "status": "AUTHORIZED",
  "is_approved": true,
  "authorization_code": "123456",
  "card": {
    "brand": "visa",
    "last_four": "1111"
  },
  "created_at": "2025-11-11T12:00:00Z"
}
```

---

#### 2. Capture Payment

Captures a previously authorized payment.

**Endpoint**: `POST /api/v1/payments/capture`

**Full Capture**:
```json
{
  "transaction_id": "tx-uuid-here",
  "idempotency_key": "capture-12345"
}
```

**Partial Capture** (e.g., capture $75 of $100 auth):
```json
{
  "transaction_id": "tx-uuid-here",
  "amount": "75.00",
  "idempotency_key": "capture-12345"
}
```

**Response** (200 OK):
```json
{
  "transaction_id": "tx-new-uuid",
  "group_id": "grp-same-as-auth",
  "amount": "75.00",
  "currency": "USD",
  "status": "COMPLETED",
  "is_approved": true,
  "authorization_code": "789012",
  "created_at": "2025-11-11T12:05:00Z"
}
```

---

#### 3. Sale (Authorize + Capture)

Single-step payment operation.

**Endpoint**: `POST /api/v1/payments/sale`

**Request Body**:
```json
{
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "amount": "49.99",
  "currency": "USD",
  "payment_method_id": "pm-uuid-here",
  "idempotency_key": "sale-12345",
  "metadata": {
    "order_id": "ORDER-67890"
  }
}
```

**Response** (200 OK):
```json
{
  "transaction_id": "tx-uuid-here",
  "group_id": "grp-uuid-here",
  "amount": "49.99",
  "currency": "USD",
  "status": "COMPLETED",
  "is_approved": true,
  "authorization_code": "ABC123",
  "card": {
    "brand": "mastercard",
    "last_four": "4444"
  },
  "created_at": "2025-11-11T12:10:00Z"
}
```

---

#### 4. Void Transaction

Cancels an authorized or captured payment before settlement.

**Endpoint**: `POST /api/v1/payments/void`

**Request Body**:
```json
{
  "group_id": "grp-uuid-here",
  "idempotency_key": "void-12345"
}
```

**Response** (200 OK):
```json
{
  "transaction_id": "tx-new-uuid",
  "group_id": "grp-same-as-original",
  "status": "VOIDED",
  "is_approved": true,
  "created_at": "2025-11-11T12:15:00Z"
}
```

**Important**: Use `group_id` (not `transaction_id`) to void. This ensures you void the correct transaction even if multiple operations exist in the group.

---

#### 5. Refund Transaction

Returns funds to customer after settlement.

**Endpoint**: `POST /api/v1/payments/refund`

**Full Refund**:
```json
{
  "group_id": "grp-uuid-here",
  "reason": "Customer requested refund",
  "idempotency_key": "refund-12345"
}
```

**Partial Refund** (e.g., refund $30 of $100 sale):
```json
{
  "group_id": "grp-uuid-here",
  "amount": "30.00",
  "reason": "Partial order cancellation",
  "idempotency_key": "refund-12345"
}
```

**Response** (200 OK):
```json
{
  "transaction_id": "tx-new-uuid",
  "group_id": "grp-same-as-original",
  "amount": "30.00",
  "currency": "USD",
  "status": "REFUNDED",
  "is_approved": true,
  "created_at": "2025-11-11T12:20:00Z"
}
```

**Multiple Refunds**: You can issue multiple partial refunds against the same group_id until the full amount is refunded.

---

#### 6. Get Transaction

Retrieves details of a specific transaction.

**Endpoint**: `GET /api/v1/payments/{transaction_id}`

**Example**:
```bash
GET /api/v1/payments/tx-uuid-here
```

**Response** (200 OK):
```json
{
  "id": "tx-uuid-here",
  "group_id": "grp-uuid-here",
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "payment_method_id": "pm-uuid-here",
  "type": "SALE",
  "status": "COMPLETED",
  "amount": "49.99",
  "currency": "USD",
  "is_approved": true,
  "authorization_code": "ABC123",
  "card": {
    "brand": "visa",
    "last_four": "1111"
  },
  "metadata": {
    "order_id": "ORDER-67890"
  },
  "created_at": "2025-11-11T12:00:00Z",
  "updated_at": "2025-11-11T12:00:00Z"
}
```

---

#### 7. List Transactions

Lists transactions with filtering options.

**Endpoint**: `GET /api/v1/payments`

**Query Parameters**:
- `agent_id` (required): Merchant identifier
- `customer_id` (optional): Filter by customer
- `group_id` (optional): Get all transactions in a group (sale + refunds)
- `status` (optional): Filter by status (PENDING, COMPLETED, FAILED, etc.)
- `limit` (optional): Max results (default: 100)
- `offset` (optional): Pagination offset

**Example - List by Customer**:
```bash
GET /api/v1/payments?agent_id=merchant-123&customer_id=cust-456&limit=50
```

**Example - List by Group** (get sale + refunds):
```bash
GET /api/v1/payments?agent_id=merchant-123&group_id=grp-uuid-here
```

**Response** (200 OK):
```json
{
  "transactions": [
    {
      "id": "tx-1",
      "group_id": "grp-uuid",
      "type": "SALE",
      "status": "COMPLETED",
      "amount": "100.00",
      "created_at": "2025-11-11T12:00:00Z"
    },
    {
      "id": "tx-2",
      "group_id": "grp-uuid",
      "type": "REFUND",
      "status": "COMPLETED",
      "amount": "30.00",
      "created_at": "2025-11-11T12:20:00Z"
    }
  ],
  "total_count": 2
}
```

---

### Payment Method APIs

Base path: `/api/v1/payment-methods`

Payment methods represent saved/tokenized payment instruments (credit cards, ACH accounts).

#### 1. Store Payment Method

Stores a tokenized payment method for future use.

**Endpoint**: `POST /api/v1/payment-methods`

**Request Body**:
```json
{
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "payment_token": "epx-bric-token-here",
  "is_default": true,
  "metadata": {
    "label": "Primary Card",
    "billing_zip": "12345"
  }
}
```

**Response** (201 Created):
```json
{
  "id": "pm-uuid-here",
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "card_brand": "visa",
  "last_four": "1111",
  "exp_month": "12",
  "exp_year": "2025",
  "is_default": true,
  "created_at": "2025-11-11T12:00:00Z"
}
```

**Note**: ACH payment methods also supported:
```json
{
  "id": "pm-uuid-here",
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "account_type": "checking",
  "last_four": "7890",
  "routing_number_last_four": "0021",
  "is_default": true,
  "created_at": "2025-11-11T12:00:00Z"
}
```

---

#### 2. Get Payment Method

Retrieves a specific payment method.

**Endpoint**: `GET /api/v1/payment-methods/{id}`

**Example**:
```bash
GET /api/v1/payment-methods/pm-uuid-here
```

**Response** (200 OK):
```json
{
  "id": "pm-uuid-here",
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "card_brand": "mastercard",
  "last_four": "4444",
  "exp_month": "06",
  "exp_year": "2026",
  "is_default": false,
  "created_at": "2025-11-11T12:00:00Z"
}
```

---

#### 3. List Payment Methods

Lists all payment methods for a customer.

**Endpoint**: `GET /api/v1/payment-methods`

**Query Parameters**:
- `agent_id` (required): Merchant identifier
- `customer_id` (required): Customer identifier

**Example**:
```bash
GET /api/v1/payment-methods?agent_id=merchant-123&customer_id=cust-456
```

**Response** (200 OK):
```json
{
  "payment_methods": [
    {
      "id": "pm-1",
      "card_brand": "visa",
      "last_four": "1111",
      "exp_month": "12",
      "exp_year": "2025",
      "is_default": true,
      "created_at": "2025-01-15T10:00:00Z"
    },
    {
      "id": "pm-2",
      "card_brand": "mastercard",
      "last_four": "4444",
      "exp_month": "06",
      "exp_year": "2026",
      "is_default": false,
      "created_at": "2025-03-20T14:30:00Z"
    }
  ]
}
```

---

#### 4. Delete Payment Method

Soft-deletes a payment method (marks as deleted, doesn't physically remove).

**Endpoint**: `DELETE /api/v1/payment-methods/{id}`

**Example**:
```bash
DELETE /api/v1/payment-methods/pm-uuid-here
```

**Response** (200 OK):
```json
{
  "success": true,
  "deleted_at": "2025-11-11T12:30:00Z"
}
```

---

### Subscription APIs

Base path: `/api/v1/subscriptions`

Subscriptions enable recurring billing for SaaS, memberships, etc.

#### 1. Create Subscription

Creates a new recurring billing subscription.

**Endpoint**: `POST /api/v1/subscriptions`

**Request Body**:
```json
{
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "payment_method_id": "pm-uuid-here",
  "plan_id": "monthly-premium",
  "amount": "29.99",
  "currency": "USD",
  "billing_cycle": "monthly",
  "start_date": "2025-11-15T00:00:00Z",
  "metadata": {
    "plan_name": "Premium Monthly",
    "features": "unlimited-access"
  }
}
```

**Response** (201 Created):
```json
{
  "subscription_id": "sub-uuid-here",
  "agent_id": "merchant-123",
  "customer_id": "cust-456",
  "payment_method_id": "pm-uuid-here",
  "plan_id": "monthly-premium",
  "amount": "29.99",
  "currency": "USD",
  "billing_cycle": "monthly",
  "status": "active",
  "current_period_start": "2025-11-15T00:00:00Z",
  "current_period_end": "2025-12-15T00:00:00Z",
  "next_billing_date": "2025-12-15T00:00:00Z",
  "created_at": "2025-11-11T12:00:00Z"
}
```

---

#### 2. Get Subscription

Retrieves subscription details.

**Endpoint**: `GET /api/v1/subscriptions/{id}`

**Example**:
```bash
GET /api/v1/subscriptions/sub-uuid-here
```

**Response** (200 OK):
```json
{
  "subscription_id": "sub-uuid-here",
  "customer_id": "cust-456",
  "payment_method_id": "pm-uuid-here",
  "amount": "29.99",
  "currency": "USD",
  "billing_cycle": "monthly",
  "status": "active",
  "next_billing_date": "2025-12-15T00:00:00Z",
  "created_at": "2025-11-11T12:00:00Z"
}
```

---

#### 3. List Subscriptions

Lists subscriptions for a customer.

**Endpoint**: `GET /api/v1/subscriptions`

**Query Parameters**:
- `customer_id` (required): Customer identifier
- `status` (optional): Filter by status (active, paused, canceled)

**Example**:
```bash
GET /api/v1/subscriptions?customer_id=cust-456&status=active
```

**Response** (200 OK):
```json
{
  "subscriptions": [
    {
      "subscription_id": "sub-1",
      "plan_id": "monthly-premium",
      "amount": "29.99",
      "status": "active",
      "next_billing_date": "2025-12-15T00:00:00Z"
    }
  ]
}
```

---

#### 4. Cancel Subscription

Cancels a subscription (stops future billing).

**Endpoint**: `POST /api/v1/subscriptions/{id}/cancel`

**Request Body**:
```json
{
  "reason": "Customer requested cancellation"
}
```

**Response** (200 OK):
```json
{
  "subscription_id": "sub-uuid-here",
  "status": "canceled",
  "canceled_at": "2025-11-11T12:00:00Z"
}
```

---

#### 5. Pause Subscription

Temporarily pauses billing.

**Endpoint**: `POST /api/v1/subscriptions/{id}/pause`

**Response** (200 OK):
```json
{
  "subscription_id": "sub-uuid-here",
  "status": "paused",
  "paused_at": "2025-11-11T12:00:00Z"
}
```

---

#### 6. Resume Subscription

Resumes a paused subscription.

**Endpoint**: `POST /api/v1/subscriptions/{id}/resume`

**Response** (200 OK):
```json
{
  "subscription_id": "sub-uuid-here",
  "status": "active",
  "next_billing_date": "2025-12-15T00:00:00Z",
  "resumed_at": "2025-11-11T12:00:00Z"
}
```

---

#### 7. Update Payment Method

Changes the payment method for a subscription.

**Endpoint**: `PUT /api/v1/subscriptions/{id}/payment-method`

**Request Body**:
```json
{
  "payment_method_id": "pm-new-uuid-here"
}
```

**Response** (200 OK):
```json
{
  "subscription_id": "sub-uuid-here",
  "payment_method_id": "pm-new-uuid-here",
  "updated_at": "2025-11-11T12:00:00Z"
}
```

---

#### 8. Process Recurring Billing

Manually triggers billing for a subscription (typically called by cron).

**Endpoint**: `POST /api/v1/subscriptions/{id}/bill`

**Request Body**:
```json
{
  "subscription_id": "sub-uuid-here"
}
```

**Response** (200 OK):
```json
{
  "transaction_id": "tx-uuid-here",
  "group_id": "grp-uuid-here",
  "subscription_id": "sub-uuid-here",
  "amount": "29.99",
  "status": "completed",
  "is_approved": true,
  "next_billing_date": "2026-01-15T00:00:00Z"
}
```

---

### Browser Post APIs

Base path: `/api/v1/payments/browser-post`

Browser Post enables PCI-compliant credit card payments where card data flows directly from browser to EPX.

See [BROWSER_POST_DATAFLOW.md](./BROWSER_POST_DATAFLOW.md) for complete integration guide.

#### 1. Get Payment Form

Generates a payment form configuration for frontend rendering.

**Endpoint**: `GET /api/v1/payments/browser-post/form`

**Query Parameters**:
- `amount` (required): Transaction amount (decimal string, e.g., "99.99")
- `return_url` (required): URL to redirect after payment
- `agent_id` (optional): Merchant identifier (defaults to EPX custNbr)
- `customer_id` (optional): Customer identifier for tracking

**Example**:
```bash
GET /api/v1/payments/browser-post/form?amount=99.99&return_url=https://pos.example.com/complete&agent_id=merchant-123
```

**Response** (200 OK):
```json
{
  "transaction_id": "tx-uuid-here",
  "group_id": "grp-uuid-here",
  "form_config": {
    "action": "https://secure.epxuap.com/browserpost",
    "method": "POST",
    "fields": {
      "cust_nbr": "9001",
      "merch_nbr": "900300",
      "dba_nbr": "2",
      "terminal_nbr": "77",
      "tran_code": "RetailSale",
      "amount": "99.99",
      "callback_url": "http://localhost:8081/api/v1/payments/browser-post/callback",
      "idempotency_key": "form-uuid-here"
    }
  }
}
```

**Frontend Usage**:
```html
<form id="payment-form" action="{{form_config.action}}" method="{{form_config.method}}">
  <input type="hidden" name="cust_nbr" value="{{fields.cust_nbr}}">
  <!-- ... other hidden fields ... -->

  <!-- EPX-styled card inputs -->
  <input type="text" name="account_number" placeholder="Card Number">
  <input type="text" name="exp_date" placeholder="MMYY">
  <input type="text" name="cvv2" placeholder="CVV">

  <button type="submit">Pay $99.99</button>
</form>
```

---

#### 2. Handle Payment Callback

Internal endpoint that receives EPX's POST callback after payment processing.

**Endpoint**: `POST /api/v1/payments/browser-post/callback`

**Note**: This endpoint is called by EPX, not by your application. The browser is automatically redirected here after payment.

**EPX POST Data** (received from EPX):
```
auth_resp=00&
auth_resp_text=APPROVED&
auth_guid=ABC123DEF456&
idempotency_key=form-uuid-here&
amount=99.99&
...
```

**Service Actions**:
1. Validates EPX response
2. Updates transaction from PENDING → COMPLETED/FAILED
3. Extracts `return_url` from `USER_DATA_1` field
4. Redirects browser to: `{return_url}?group_id={group_id}&status={status}`

**Redirect Example**:
```
https://pos.example.com/complete?group_id=grp-uuid-here&status=completed
```

**Your Application's Job**:
Look up order by `group_id` and render complete receipt.

---

### Cron/Health APIs

Base path: `/cron`

Administrative endpoints for health checks and scheduled tasks.

#### 1. Health Check

Returns service health status.

**Endpoint**: `GET /cron/health`

**Example**:
```bash
curl http://localhost:8081/cron/health
```

**Response** (200 OK):
```json
{
  "status": "healthy",
  "time": "2025-11-11T12:00:00Z"
}
```

**Use Cases**:
- Kubernetes liveness/readiness probes
- Load balancer health checks
- Monitoring systems

---

#### 2. Service Stats

Returns service statistics and metrics.

**Endpoint**: `GET /cron/stats`

**Authentication**: Requires `X-Cron-Secret` header

**Example**:
```bash
curl -H "X-Cron-Secret: your-secret" http://localhost:8081/cron/stats
```

**Response** (200 OK):
```json
{
  "subscriptions": {
    "total_active": 1234,
    "total_paused": 56,
    "total_canceled": 789
  },
  "billing": {
    "next_billing_run": "2025-11-12T00:00:00Z",
    "last_successful_run": "2025-11-11T00:00:00Z"
  },
  "database": {
    "active_connections": 5,
    "max_connections": 25
  }
}
```

---

#### 3. Process Recurring Billing

Triggers subscription billing for due subscriptions.

**Endpoint**: `POST /cron/process-billing`

**Authentication**: Requires `X-Cron-Secret` header

**Example**:
```bash
curl -X POST -H "X-Cron-Secret: your-secret" http://localhost:8081/cron/process-billing
```

**Response** (200 OK):
```json
{
  "processed": 45,
  "successful": 42,
  "failed": 3,
  "duration_ms": 1234
}
```

**Cron Schedule**:
```yaml
# Cloud Scheduler (GCP)
schedule: "0 2 * * *"  # Daily at 2 AM UTC
```

---

#### 4. Sync Disputes/Chargebacks

Fetches new disputes from North Merchant Reporting API.

**Endpoint**: `POST /cron/sync-disputes`

**Authentication**: Requires `X-Cron-Secret` header

**Example**:
```bash
curl -X POST -H "X-Cron-Secret: your-secret" http://localhost:8081/cron/sync-disputes
```

**Response** (200 OK):
```json
{
  "new_disputes": 3,
  "updated_disputes": 5,
  "total_processed": 8,
  "duration_ms": 567
}
```

**Cron Schedule**:
```yaml
# Cloud Scheduler (GCP)
schedule: "0 */4 * * *"  # Every 4 hours
```

---

## gRPC-Only APIs

These services are gRPC-only (no HTTP REST endpoints) and are intended for internal/admin use.

### Agent Service

Service: `agent.v1.AgentService`
Port: **8080** (gRPC)
Proto: `proto/agent/v1/agent.proto`

**Purpose**: Multi-tenant agent/merchant credential management

**Methods**:
- `RegisterAgent` - Adds new agent/merchant to system
- `GetAgent` - Retrieves agent credentials (internal use)
- `ListAgents` - Lists all registered agents
- `UpdateAgent` - Updates agent credentials
- `DeactivateAgent` - Deactivates an agent
- `RotateMAC` - Rotates MAC secret in secret manager

**Access**: Internal/admin only - requires gRPC client

**Example Usage** (Go):
```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    agentv1 "github.com/kevin07696/payment-service/proto/agent/v1"
)

conn, _ := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
client := agentv1.NewAgentServiceClient(conn)
resp, _ := client.GetAgent(ctx, &agentv1.GetAgentRequest{
    AgentId: "merchant-123",
})
```

**Why gRPC-Only?**
Agent management is sensitive administrative functionality. Keeping it gRPC-only:
- Enforces internal access patterns
- Avoids accidental HTTP exposure
- Simplifies security (no gateway layer)

---

### Chargeback Service

Service: `chargeback.v1.ChargebackService`
Port: **8080** (gRPC)
Proto: `proto/chargeback/v1/chargeback.proto`

**Purpose**: Chargeback/dispute tracking and management

**Status**: Check proto file for available methods

---

## Error Handling

### HTTP Status Codes

| Code | Meaning | When |
|------|---------|------|
| 200 | OK | Successful operation |
| 201 | Created | Resource created (e.g., payment method, subscription) |
| 400 | Bad Request | Validation error, missing required fields |
| 401 | Unauthorized | Missing or invalid authentication |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Duplicate idempotency key |
| 500 | Internal Server Error | Service error |
| 503 | Service Unavailable | Service temporarily down |

### Error Response Format

All errors follow gRPC-Gateway error format:

```json
{
  "code": 3,
  "message": "payment_token is required",
  "details": []
}
```

**Common Error Codes**:
- `3` - INVALID_ARGUMENT (validation error)
- `5` - NOT_FOUND (resource not found)
- `13` - INTERNAL (server error)

### Validation Errors

```json
{
  "code": 3,
  "message": "amount is required",
  "details": []
}
```

### Not Found Errors

```json
{
  "code": 5,
  "message": "Not Found",
  "details": []
}
```

### Gateway Errors

When EPX rejects a payment:

```json
{
  "transaction_id": "tx-uuid",
  "group_id": "grp-uuid",
  "status": "FAILED",
  "is_approved": false,
  "error_message": "Insufficient funds",
  "error_code": "51"
}
```

---

## API Best Practices

### 1. Always Use Idempotency Keys

Prevent duplicate charges by including unique `idempotency_key`:

```json
{
  "agent_id": "merchant-123",
  "amount": "99.99",
  "idempotency_key": "order-12345"
}
```

### 2. Use group_id for Refunds/Voids

Always use `group_id` (not `transaction_id`) for refunds and voids:

```json
{
  "group_id": "grp-uuid-here",
  "amount": "30.00"
}
```

This ensures you refund the correct transaction even if multiple operations exist.

### 3. Store group_id with Orders

When creating orders, store the `group_id` returned from payment operations:

```sql
CREATE TABLE orders (
  id UUID PRIMARY KEY,
  payment_group_id UUID NOT NULL,  -- Store this!
  total_amount DECIMAL(10,2),
  status VARCHAR(50)
);
```

### 4. Handle Async Callbacks

Browser Post payments complete asynchronously. Your return URL endpoint must:
1. Accept `group_id` and `status` query parameters
2. Look up order by `group_id`
3. Update order status
4. Render receipt

### 5. Test with EPX Sandbox

Use EPX sandbox test cards:
- Visa: `4111111111111111`
- Mastercard: `5555555555554444`
- Amex: `378282246310005`

---

## Rate Limits

Browser Post endpoints include rate limiting:
- **10 requests/second per IP**
- **Burst of 20 requests**

Exceeding limits returns:
```json
{
  "code": 8,
  "message": "Rate limit exceeded",
  "details": []
}
```

---

## Related Documentation

- [Browser Post Dataflow](./BROWSER_POST_DATAFLOW.md) - Complete Browser Post integration guide
- [Server Post Dataflow](./SERVER_POST_DATAFLOW.md) - Server-side payment flow
- [ACH Server Post Dataflow](./ACH_SERVER_POST_DATAFLOW.md) - ACH payment flow
- [Database Design](./DATABASE_DESIGN.md) - Schema reference
- [Testing Guide](./TESTING.md) - Integration test setup
- [EPX API Reference](./EPX_API_REFERENCE.md) - EPX gateway documentation

---

## Changelog

| Date | Version | Changes |
|------|---------|---------|
| 2025-11-11 | 1.0.0 | Initial API specification |
