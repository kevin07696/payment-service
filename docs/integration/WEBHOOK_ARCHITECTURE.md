# Webhook Architecture Guide

**Last Updated**: 2025-12-05
**Status**: Current Implementation + Future Enhancements

---

## Table of Contents

1. [What Are Webhooks?](#what-are-webhooks)
2. [Architecture Overview](#architecture-overview)
3. [Current Implementation](#current-implementation)
4. [Event Types](#event-types)
5. [Webhook Flow](#webhook-flow)
6. [Security & Verification](#security--verification)
7. [Retry Logic](#retry-logic)
8. [Implementing a Webhook Endpoint](#implementing-a-webhook-endpoint)
9. [Best Practices](#best-practices)
10. [Future Enhancements](#future-enhancements)

---

## What Are Webhooks?

### Definition

Webhooks are **server-to-server HTTP callbacks** that notify your application when events occur in the payment service. They enable real-time, event-driven integrations without polling.

### Webhooks vs Other Communication Methods

| Method | Use Case | Delivery | Reliability |
|--------|----------|----------|-------------|
| **Webhooks** | Server-to-server events | Push (immediate) | At-least-once with retries |
| **Polling** | Fallback mechanism | Pull (delayed) | Eventual consistency |
| **Email** | End-user notifications | Async (delayed) | Best effort |
| **Push Notifications** | Mobile app alerts | Async (device-dependent) | Best effort |

### What Webhooks Are NOT

❌ **NOT for end-user notifications** - Use email/SMS/push notifications instead
❌ **NOT guaranteed immediate delivery** - Your server could be down
❌ **NOT a replacement for API polling** - Use as primary method with polling as fallback
❌ **NOT synchronous responses** - Webhooks are fire-and-forget

---

## Architecture Overview

### High-Level Flow

```
┌─────────────────────────────────────────────────────────────────┐
│  Payment Service Event Occurs                                    │
│  (e.g., Subscription billing fails)                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Webhook Delivery Queue (Database)                              │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ Table: webhook_deliveries                                │  │
│  │ - id, subscription_id, event_type                        │  │
│  │ - payload (JSON), status (pending/success/failed)        │  │
│  │ - attempts, next_retry_at, http_status_code             │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  HTTP POST to Your Webhook Endpoint                             │
│  POST https://your-app.com/webhooks/payment-service             │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ Headers:                                                 │  │
│  │   X-Signature: HMAC-SHA256(payload, secret)             │  │
│  │   X-Event-Type: subscription.payment_failed             │  │
│  │   Content-Type: application/json                        │  │
│  │                                                          │  │
│  │ Body:                                                    │  │
│  │   {                                                      │  │
│  │     "event_type": "subscription.payment_failed",        │  │
│  │     "merchant_id": "merchant-123",                         │  │
│  │     "data": { ... },                                    │  │
│  │     "timestamp": "2025-11-22T12:00:00Z"                 │  │
│  │   }                                                      │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                     ┌────────┴────────┐
                     ▼                 ▼
              ┌──────────┐      ┌──────────┐
              │ Success  │      │  Failed  │
              │ 200 OK   │      │ 4xx/5xx  │
              └──────────┘      └──────────┘
                     │                 │
                     ▼                 ▼
              Mark 'success'    Schedule Retry
              Delete/Archive    (exponential backoff)
```

### System Components

1. **Event Source**: Business logic that triggers events (e.g., subscription billing failure)
2. **Webhook Service**: `internal/services/webhook/webhook_delivery_service.go`
3. **Delivery Queue**: `webhook_deliveries` table (persistent queue)
4. **HTTP Client**: Sends POST requests to merchant endpoints
5. **Retry Worker**: Background cron job that processes failed deliveries

---

## Current Implementation

### Database Schema

**Table: `webhook_subscriptions`**
```sql
CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL,                -- Merchant ID (references merchants table)
    event_type VARCHAR(50) NOT NULL,          -- Event to subscribe to
    webhook_url TEXT NOT NULL,                -- Your endpoint URL
    secret VARCHAR(255) NOT NULL,             -- HMAC signing secret
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- One active webhook per merchant per event type per URL
    CONSTRAINT unique_active_webhook UNIQUE (merchant_id, event_type, webhook_url)
);
```

**Table: `webhook_deliveries`**
```sql
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,                   -- Full event payload
    status VARCHAR(20) NOT NULL,              -- 'pending', 'success', 'failed'
    http_status_code INT,                     -- Response status code
    error_message TEXT,                       -- Error details if failed
    attempts INT NOT NULL DEFAULT 0,          -- Retry count
    next_retry_at TIMESTAMPTZ,                -- When to retry next
    delivered_at TIMESTAMPTZ,                 -- When successfully delivered
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT valid_status CHECK (status IN ('pending', 'success', 'failed'))
);

-- Index for retry queue processing
CREATE INDEX idx_webhook_deliveries_retry
ON webhook_deliveries(next_retry_at)
WHERE status = 'pending' AND next_retry_at IS NOT NULL;
```

### Service Layer

**File**: `internal/services/webhook/webhook_delivery_service.go`

**Key Methods**:
- `DeliverEvent(ctx, event)` - Queue webhook for delivery
- `deliverToSubscription(ctx, subscription, event)` - Send HTTP POST
- `signPayload(payload, secret)` - Generate HMAC-SHA256 signature

**Current Implementation**:
```go
// WebhookDeliveryService handles webhook delivery to merchant endpoints
type WebhookDeliveryService struct {
    db         DatabaseAdapter
    httpClient *http.Client  // 10 second timeout
    logger     *zap.Logger
}

// DeliverEvent delivers a webhook event to all subscribed endpoints
func (s *WebhookDeliveryService) DeliverEvent(ctx context.Context, event *WebhookEvent) error {
    // 1. Find active subscriptions for this event type
    subscriptions, err := s.db.Queries().ListActiveWebhooksByEvent(ctx, params)

    // 2. Deliver to each subscription
    for _, subscription := range subscriptions {
        s.deliverToSubscription(ctx, subscription, event)
    }

    return nil
}
```

---

## Event Types

### Currently Implemented

| Event Type | Description | When Triggered | Payload Fields |
|------------|-------------|----------------|----------------|
| `chargeback.created` | New chargeback filed | Chargeback sync from North API | `chargeback_id`, `transaction_id`, `amount`, `reason_code` |
| `chargeback.updated` | Chargeback status changed | Admin updates chargeback | `chargeback_id`, `old_status`, `new_status` |
| `subscription.cancelled` | Subscription cancelled | Grace period expired or API cancellation | `subscription_id`, `reason` |
| `subscription.past_due` | Max retries reached | Subscription marked past_due | `subscription_id` |
| `payment.succeeded` | Payment successful | Payment processed successfully | `transaction_id`, `amount_cents` |
| `payment.failed` | Payment failed | Payment declined or error | `transaction_id`, `reason` |

### Planned (Future Enhancement)

| Event Type | Description | When Triggered | Priority |
|------------|-------------|----------------|----------|
| `subscription.created` | Subscription created | New subscription | MEDIUM |
| `subscription.updated` | Subscription updated | Subscription modified | MEDIUM |
| `payment.refunded` | Payment refunded | Refund processed | MEDIUM |
| `payment_method.verification_failed` | ACH verification failed | Pre-note returned | HIGH |
| `payment_method.verification_succeeded` | ACH verified | Pre-note cleared | MEDIUM |

---

## Webhook Flow

### Step-by-Step Delivery Process

#### 1. Event Occurs
```go
// Example: Chargeback created
chargeback := &domain.Chargeback{
    ID:            uuid.New(),
    TransactionID: txID,
    CaseNumber:    "CB-12345",
    Amount:        10000, // $100.00
}

// Save to database
err := chargebackRepo.Create(ctx, chargeback)
```

#### 2. Trigger Webhook
```go
// Create webhook event
event := &webhook.WebhookEvent{
    EventType: "chargeback.created",
    MerchantID: chargeback.MerchantID.String(),
    Data: map[string]interface{}{
        "chargeback_id":  chargeback.ID.String(),
        "transaction_id": chargeback.TransactionID.String(),
        "case_number":    chargeback.CaseNumber,
        "amount_cents":   chargeback.Amount,
        "reason_code":    chargeback.ReasonCode,
        "dispute_date":   chargeback.DisputeDate,
    },
    Timestamp: time.Now(),
}

// Queue for delivery
webhookService.DeliverEvent(ctx, event)
```

#### 3. Create Delivery Record
```sql
INSERT INTO webhook_deliveries (
    id, subscription_id, event_type, payload,
    status, attempts, next_retry_at, created_at
) VALUES (
    gen_random_uuid(),
    '...',  -- subscription_id
    'chargeback.created',
    '{"chargeback_id": "...", ...}',
    'pending',
    0,
    NOW(),  -- Immediate first attempt
    NOW()
);
```

#### 4. Send HTTP POST
```http
POST https://your-app.com/webhooks/payment-service
Content-Type: application/json
X-Event-Type: chargeback.created
X-Signature: a3f4b2c1... (HMAC-SHA256)

{
  "event_type": "chargeback.created",
  "merchant_id": "merchant-123",
  "data": {
    "chargeback_id": "cb_1a2b3c",
    "transaction_id": "tx_9z8y7x",
    "case_number": "CB-12345",
    "amount_cents": 10000,
    "reason_code": "4855",
    "dispute_date": "2025-11-22T12:00:00Z"
  },
  "timestamp": "2025-11-22T12:05:00Z"
}
```

#### 5. Handle Response

**Success (200 OK)**:
```sql
UPDATE webhook_deliveries SET
    status = 'success',
    http_status_code = 200,
    delivered_at = NOW()
WHERE id = '...';
```

**Failure (5xx or timeout)**:
```sql
UPDATE webhook_deliveries SET
    status = 'pending',
    attempts = attempts + 1,
    next_retry_at = NOW() + INTERVAL '1 minute',  -- Exponential backoff
    http_status_code = 500,
    error_message = 'Connection timeout'
WHERE id = '...';
```

---

## Security & Verification

### HMAC Signature Verification

Every webhook includes an `X-Signature` header for authenticity verification.

**How It Works**:
```
1. Payment Service:
   - Generates HMAC-SHA256(payload, merchant.secret)
   - Sends signature in X-Signature header

2. Your Server:
   - Computes HMAC-SHA256(payload, your_secret)
   - Compares with X-Signature header
   - If match → Authentic, process event
   - If mismatch → Reject (possible spoof attempt)
```

**Implementation (Your Server)**:

```javascript
// Node.js/Express example
const crypto = require('crypto');

app.post('/webhooks/payment-service', (req, res) => {
  // 1. Get signature from header
  const receivedSignature = req.headers['x-signature'];

  // 2. Compute expected signature
  const payload = JSON.stringify(req.body);
  const expectedSignature = crypto
    .createHmac('sha256', process.env.WEBHOOK_SECRET)
    .update(payload)
    .digest('hex');

  // 3. Compare (timing-safe comparison)
  if (!crypto.timingSafeEqual(
    Buffer.from(receivedSignature),
    Buffer.from(expectedSignature)
  )) {
    console.error('Invalid webhook signature');
    return res.status(401).send('Unauthorized');
  }

  // 4. Signature valid - process event
  processWebhookEvent(req.body);
  res.status(200).send('OK');
});
```

```go
// Go example
func webhookHandler(w http.ResponseWriter, r *http.Request) {
    // 1. Read body
    body, _ := io.ReadAll(r.Body)

    // 2. Get signature
    receivedSig := r.Header.Get("X-Signature")

    // 3. Compute expected signature
    mac := hmac.New(sha256.New, []byte(webhookSecret))
    mac.Write(body)
    expectedSig := hex.EncodeToString(mac.Sum(nil))

    // 4. Compare
    if !hmac.Equal([]byte(receivedSig), []byte(expectedSig)) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // 5. Process event
    var event WebhookEvent
    json.Unmarshal(body, &event)
    processEvent(event)

    w.WriteHeader(http.StatusOK)
}
```

```python
# Python/Flask example
import hmac
import hashlib

@app.route('/webhooks/payment-service', methods=['POST'])
def webhook():
    # 1. Get signature
    received_sig = request.headers.get('X-Signature')

    # 2. Compute expected signature
    payload = request.get_data()
    expected_sig = hmac.new(
        WEBHOOK_SECRET.encode(),
        payload,
        hashlib.sha256
    ).hexdigest()

    # 3. Verify
    if not hmac.compare_digest(received_sig, expected_sig):
        return 'Unauthorized', 401

    # 4. Process event
    event = request.get_json()
    process_webhook_event(event)

    return 'OK', 200
```

### Best Security Practices

1. ✅ **Always verify signatures** - Prevents spoofing attacks
2. ✅ **Use HTTPS for webhook URLs** - Prevents man-in-the-middle
3. ✅ **Rotate secrets periodically** - Update via API every 90 days
4. ✅ **Whitelist IP addresses** (optional) - Only allow requests from payment service IPs
5. ✅ **Implement idempotency** - Handle duplicate deliveries gracefully

---

## Retry Logic

### Retry Schedule

Webhooks use **exponential backoff** with jitter for retries:

| Attempt | Delay | Total Time Elapsed |
|---------|-------|--------------------|
| 1 | Immediate | 0s |
| 2 | 1 minute | 1m |
| 3 | 5 minutes | 6m |
| 4 | 30 minutes | 36m |
| 5 | 2 hours | 2h 36m |
| 6 | 6 hours | 8h 36m |
| 7 (final) | 24 hours | 32h 36m |

After 7 attempts, webhook is marked as **permanently failed** and requires manual intervention.

### Retry Worker

**Cron Job**: Runs every 1 minute

```sql
-- Find deliveries ready for retry
SELECT * FROM webhook_deliveries
WHERE status = 'pending'
  AND next_retry_at <= NOW()
ORDER BY next_retry_at ASC
LIMIT 100;
```

```go
// Pseudo-code for retry worker
func processRetryQueue() {
    deliveries := getReadyForRetry()

    for _, delivery := range deliveries {
        if delivery.Attempts >= maxRetries {
            markAsFailed(delivery)
            alertOps("Webhook permanently failed", delivery)
        } else {
            attemptDelivery(delivery)
        }
    }
}
```

---

## Implementing a Webhook Endpoint

### Requirements

Your webhook endpoint must:

1. ✅ Accept HTTP POST requests
2. ✅ Respond within 10 seconds (service timeout)
3. ✅ Return `200 OK` on success
4. ✅ Return `4xx/5xx` on failure (triggers retry)
5. ✅ Verify HMAC signature
6. ✅ Be idempotent (handle duplicate events)

### Recommended Architecture

**Pattern 1: Queue for Processing** (Recommended)

```javascript
// Webhook handler - acknowledge FAST, process ASYNC
app.post('/webhooks/payment-service', async (req, res) => {
  // 1. Verify signature (fast)
  if (!verifySignature(req)) {
    return res.status(401).send('Unauthorized');
  }

  // 2. Acknowledge receipt immediately (< 1 second)
  res.status(200).send('OK');

  // 3. Queue for background processing (don't wait!)
  await jobQueue.add('process-webhook', {
    eventType: req.body.event_type,
    payload: req.body.data
  });
});

// Background worker - process events asynchronously
jobQueue.process('process-webhook', async (job) => {
  const { eventType, payload } = job.data;

  // Check if already processed (idempotency)
  if (await isEventProcessed(payload.id)) {
    return; // Already handled
  }

  // Process event
  switch (eventType) {
    case 'subscription.payment_failed':
      await handlePaymentFailed(payload);
      break;
    // ...
  }

  // Mark as processed
  await markEventProcessed(payload.id);
});
```

**Pattern 2: Synchronous Processing** (Not Recommended)

```javascript
// ⚠️ ANTI-PATTERN - Slow processing blocks webhook
app.post('/webhooks/payment-service', async (req, res) => {
  if (!verifySignature(req)) {
    return res.status(401).send('Unauthorized');
  }

  // ⚠️ Slow database operations
  await db.subscriptions.update(...);
  await db.invoices.create(...);
  await sendEmail(...);  // ⚠️ Network call

  // ⚠️ If this takes > 10 seconds, webhook times out and retries
  res.status(200).send('OK');
});
```

### Idempotency Implementation

**Database Tracking**:
```sql
CREATE TABLE processed_webhook_events (
    event_id VARCHAR(255) PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Check if processed
SELECT EXISTS(SELECT 1 FROM processed_webhook_events WHERE event_id = $1);

-- Mark as processed
INSERT INTO processed_webhook_events (event_id, event_type)
VALUES ($1, $2)
ON CONFLICT (event_id) DO NOTHING;
```

**In-Memory Caching** (optional):
```javascript
const processedEvents = new Set();

function isEventProcessed(eventId) {
  return processedEvents.has(eventId);
}

function markEventProcessed(eventId) {
  processedEvents.add(eventId);
}
```

---

## Best Practices

### DO ✅

1. **Respond Quickly** - Acknowledge within 1-2 seconds, process asynchronously
2. **Be Idempotent** - Handle duplicate events gracefully
3. **Verify Signatures** - Always validate `X-Signature` header
4. **Use HTTPS** - Encrypt webhook traffic
5. **Log Everything** - Track all webhooks (success, failure, retries)
6. **Monitor Failures** - Alert on high failure rates
7. **Test Locally** - Use ngrok/localtunnel for development

### DON'T ❌

1. **Don't Block** - Don't wait for slow operations (DB writes, emails, API calls)
2. **Don't Trust Input** - Validate all payload fields
3. **Don't Ignore Failures** - Investigate why webhooks are failing
4. **Don't Hardcode Secrets** - Use environment variables
5. **Don't Use HTTP** - Always use HTTPS for webhook URLs
6. **Don't Process Twice** - Implement idempotency checks

---

## Testing Webhooks

### Local Development

**Option 1: ngrok** (Recommended)
```bash
# 1. Install ngrok
brew install ngrok  # macOS
# or download from https://ngrok.com

# 2. Start your local server
npm run dev  # Listening on localhost:3000

# 3. Create tunnel
ngrok http 3000

# Output:
# Forwarding https://abc123.ngrok.io -> http://localhost:3000
```

**Option 2: localtunnel**
```bash
# 1. Install
npm install -g localtunnel

# 2. Create tunnel
lt --port 3000

# Output:
# your url is: https://xyz789.loca.lt
```

### Testing with cURL

```bash
# Simulate webhook delivery
curl -X POST https://your-app.com/webhooks/payment-service \
  -H "Content-Type: application/json" \
  -H "X-Event-Type: subscription.payment_failed" \
  -H "X-Signature: $(echo -n '{"event_type":"subscription.payment_failed"}' | openssl dgst -sha256 -hmac 'your-secret' | awk '{print $2}')" \
  -d '{
    "event_type": "subscription.payment_failed",
    "merchant_id": "merchant-123",
    "data": {
      "subscription_id": "sub_abc123",
      "customer_id": "cus_xyz789",
      "amount_cents": 2999,
      "failure_reason": "Card declined"
    },
    "timestamp": "2025-11-22T12:00:00Z"
  }'
```

### Webhook Debugging

**Check Delivery Logs**:
```sql
-- Recent webhook deliveries
SELECT
    wd.id,
    wd.event_type,
    wd.status,
    wd.attempts,
    wd.http_status_code,
    wd.error_message,
    ws.webhook_url,
    wd.created_at,
    wd.delivered_at
FROM webhook_deliveries wd
JOIN webhook_subscriptions ws ON wd.subscription_id = ws.id
WHERE ws.merchant_id = 'your-merchant-id'
ORDER BY wd.created_at DESC
LIMIT 50;
```

**Common Issues**:

| Issue | Cause | Solution |
|-------|-------|----------|
| Signature mismatch | Wrong secret or payload format | Verify secret, log raw payload |
| Timeout (no response) | Endpoint down or slow | Check server logs, optimize processing |
| 401 Unauthorized | Signature verification failed | Log signature computation steps |
| 500 Internal Server Error | Bug in webhook handler | Check application logs |
| Duplicate events | Retry after slow response | Implement idempotency |

---

## Future Enhancements

### Completed Features

1. ✅ Add `subscription.cancelled` webhook (2025-12-05)
2. ✅ Add `subscription.past_due` webhook (2025-12-05)
3. ✅ Add `payment.succeeded` webhook (2025-12-05)
4. ✅ Add `payment.failed` webhook (2025-12-05)

### Planned Features

**Priority 1 - High (Next Sprint)**:
1. Add `payment_method.verification_failed` webhook
2. Add `payment_method.verification_succeeded` webhook
3. Add `payment.refunded` webhook

**Priority 2 - Medium (Next Month)**:
4. Add webhook retry dashboard (admin UI)
5. Add webhook testing UI (send test events)
6. Add webhook delivery analytics (success rate, latency)

**Priority 3 - Low (Next Quarter)**:
7. Add webhook event filtering (choose specific events)
8. Add webhook batching (group multiple events)
9. Add webhook rate limiting (prevent abuse)

### Webhook Dashboard (Mockup)

```
┌─────────────────────────────────────────────────────────────┐
│  Webhook Subscriptions                                       │
├─────────────────────────────────────────────────────────────┤
│  Event Type              │ URL                    │ Status  │
│  ─────────────────────────────────────────────────────────  │
│  chargeback.created      │ https://app.com/wh     │ Active  │
│  subscription.failed     │ https://app.com/wh     │ Active  │
│                                                              │
│  [+ Add Webhook Subscription]                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  Recent Deliveries                                           │
├─────────────────────────────────────────────────────────────┤
│  Time          │ Event Type        │ Status   │ Attempts    │
│  ────────────────────────────────────────────────────────── │
│  12:05:23 PM   │ chargeback.created│ Success  │ 1          │
│  12:00:15 PM   │ subscription.failed│ Failed   │ 3/7       │
│  11:55:42 AM   │ chargeback.updated│ Success  │ 1          │
│                                                              │
│  [View All Deliveries]                                      │
└─────────────────────────────────────────────────────────────┘
```

---

## API Reference

### Register Webhook Subscription

**Endpoint**: `POST /api/v1/webhooks/subscriptions`

**Request**:
```json
{
  "event_type": "subscription.payment_failed",
  "webhook_url": "https://your-app.com/webhooks/payment-service",
  "secret": "your-secret-key-for-hmac"  // Generated by you
}
```

**Response**:
```json
{
  "subscription_id": "sub_abc123",
  "event_type": "subscription.payment_failed",
  "webhook_url": "https://your-app.com/webhooks/payment-service",
  "is_active": true,
  "created_at": "2025-11-22T12:00:00Z"
}
```

### List Webhook Deliveries

**Endpoint**: `GET /api/v1/webhooks/deliveries`

**Query Parameters**:
- `event_type` (optional): Filter by event type
- `status` (optional): Filter by status (pending/success/failed)
- `limit` (optional): Max results (default: 50)

**Response**:
```json
{
  "deliveries": [
    {
      "id": "del_xyz789",
      "event_type": "subscription.payment_failed",
      "status": "success",
      "attempts": 1,
      "http_status_code": 200,
      "delivered_at": "2025-11-22T12:05:00Z",
      "created_at": "2025-11-22T12:05:00Z"
    }
  ],
  "total": 150,
  "has_more": true
}
```

---

## Support

### Troubleshooting

**Q: Why am I not receiving webhooks?**
1. Check webhook subscription is active: `GET /api/v1/webhooks/subscriptions`
2. Verify webhook URL is reachable (test with cURL)
3. Check delivery logs for errors: `GET /api/v1/webhooks/deliveries`
4. Ensure HTTPS certificate is valid (not self-signed)

**Q: Why do I receive duplicate events?**
- Webhooks use at-least-once delivery
- Implement idempotency checks (see examples above)
- Don't rely on exactly-once delivery

**Q: How long do retries continue?**
- Retries for up to 32 hours (7 attempts)
- After 7 attempts, marked as permanently failed
- Contact support to manually redeliver

### Contact

- **Documentation**: https://docs.payment-service.com/webhooks
- **API Reference**: https://api.payment-service.com/docs
- **Support**: support@payment-service.com
- **Status Page**: https://status.payment-service.com

---

**Last Updated**: 2025-12-05
**Version**: 1.2
**Status**: Production Ready (Chargeback + Subscription + Payment webhooks)
