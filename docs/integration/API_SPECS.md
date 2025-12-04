# API Specification - Copy-Paste Ready

## Quick Start

```bash
# 1. Start server
podman-compose up -d

# 2. Generate token (copy-paste this entire block)
TOKEN=$(./paycli -action=generate-token -c service_test-pos-system_credentials.json -o token) && echo "Token set: ${TOKEN:0:50}..."
```

### Test Data (Auto-Seeded)

**For Server POST (Sale, Authorize, Capture, Void, Refund):**

| Entity | ID |
|--------|-----|
| Merchant | `00000000-0000-0000-0000-000000000001` |
| Customer | `00000000-0000-0000-0000-000000000010` |
| Payment Method | `b921374c-643f-40cd-9aec-c24f92953ab7` |

**For Browser POST and List endpoints:**

| Entity | ID |
|--------|-----|
| Merchant | `ea164cb1-3995-4f62-b331-f846ecf42f2d` |
| Service | `test-pos-system` |
| Customer | `test-customer-001` |

---

## 1. Payment Service

### 1.1 Sale

```bash
curl -s -X POST "http://localhost:8081/payment.v1.PaymentService/Sale" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"merchant_id\": \"00000000-0000-0000-0000-000000000001\",
    \"customer_id\": \"00000000-0000-0000-0000-000000000010\",
    \"payment_method_id\": \"b921374c-643f-40cd-9aec-c24f92953ab7\",
    \"amount_cents\": 5000,
    \"currency\": \"USD\",
    \"idempotency_key\": \"$(cat /proc/sys/kernel/random/uuid)\"
  }"
```

**Response:**
```json
{
  "transactionId": "3e567bda-d353-408f-8099-f637b7317279",
  "amountCents": "100",
  "currency": "USD",
  "status": "TRANSACTION_STATUS_APPROVED",
  "type": "TRANSACTION_TYPE_CHARGE",
  "isApproved": true,
  "authorizationCode": "056755",
  "message": "EXACT MATCH",
  "card": {"brand": "Visa"},
  "createdAt": "2025-12-04T11:43:14.879539Z"
}
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `customer_id` | string | Yes | Your customer identifier |
| `payment_method_id` | UUID | Yes | Stored payment method to charge |
| `amount_cents` | int64 | Yes | Amount in cents (5000 = $50.00) |
| `currency` | string | Yes | ISO 4217 code (USD, CAD) |
| `idempotency_key` | UUID | Yes | Unique key to prevent duplicates |
| `order_id` | string | No | Your order reference |
| `metadata` | map | No | Custom key-value pairs |

</details>

<details>
<summary>Response Fields</summary>

| Field | Type | Description |
|-------|------|-------------|
| `transactionId` | UUID | Unique transaction identifier |
| `status` | enum | Transaction status (see below) |
| `isApproved` | bool | Quick check if transaction succeeded |
| `amountCents` | string | Amount charged in cents |
| `authorizationCode` | string | Gateway authorization code |
| `message` | string | Response message from gateway |
| `card` | object | Card info (brand, last4) |
| `createdAt` | timestamp | When transaction was created |

</details>

<details>
<summary>Transaction Status Enum</summary>

| Value | Description |
|-------|-------------|
| `TRANSACTION_STATUS_PENDING` | Processing, awaiting response |
| `TRANSACTION_STATUS_APPROVED` | Successfully approved |
| `TRANSACTION_STATUS_DECLINED` | Declined by issuer |
| `TRANSACTION_STATUS_VOIDED` | Cancelled before settlement |
| `TRANSACTION_STATUS_REFUNDED` | Funds returned to customer |
| `TRANSACTION_STATUS_ERROR` | Processing error occurred |

</details>

<details>
<summary>Transaction Type Enum</summary>

| Value | Description |
|-------|-------------|
| `TRANSACTION_TYPE_AUTHORIZATION` | Hold funds only |
| `TRANSACTION_TYPE_CAPTURE` | Capture held funds |
| `TRANSACTION_TYPE_CHARGE` | Auth + capture (sale) |
| `TRANSACTION_TYPE_REFUND` | Return funds |
| `TRANSACTION_TYPE_VOID` | Cancel transaction |

</details>

---

### 1.2 Authorize

```bash
curl -s -X POST "http://localhost:8081/payment.v1.PaymentService/Authorize" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"merchant_id\": \"00000000-0000-0000-0000-000000000001\",
    \"customer_id\": \"00000000-0000-0000-0000-000000000010\",
    \"payment_method_id\": \"b921374c-643f-40cd-9aec-c24f92953ab7\",
    \"amount_cents\": 7500,
    \"currency\": \"USD\",
    \"idempotency_key\": \"$(cat /proc/sys/kernel/random/uuid)\"
  }"
```

**Response:**
```json
{
  "transactionId": "6a7bdb7c-3932-46f4-955a-99f1c9af7acc",
  "amountCents": "200",
  "currency": "USD",
  "status": "TRANSACTION_STATUS_APPROVED",
  "type": "TRANSACTION_TYPE_AUTH",
  "isApproved": true,
  "authorizationCode": "056756",
  "message": "EXACT MATCH",
  "card": {"brand": "Visa"},
  "createdAt": "2025-12-04T11:43:15.193827Z"
}
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `customer_id` | string | Yes | Your customer identifier |
| `payment_method_id` | UUID | Yes | Stored payment method |
| `amount_cents` | int64 | Yes | Amount to hold in cents |
| `currency` | string | Yes | ISO 4217 code |
| `idempotency_key` | UUID | Yes | Unique key to prevent duplicates |

</details>

---

### 1.3 Capture

```bash
curl -s -X POST "http://localhost:8081/payment.v1.PaymentService/Capture" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"merchant_id\": \"00000000-0000-0000-0000-000000000001\",
    \"transaction_id\": \"AUTH_TRANSACTION_ID_HERE\",
    \"amount_cents\": 7500,
    \"idempotency_key\": \"$(cat /proc/sys/kernel/random/uuid)\"
  }"
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `transaction_id` | UUID | Yes | Authorization transaction to capture |
| `amount_cents` | int64 | No | Amount to capture (default: full auth amount) |
| `idempotency_key` | UUID | Yes | Unique key to prevent duplicates |

</details>

---

### 1.4 Void

```bash
curl -s -X POST "http://localhost:8081/payment.v1.PaymentService/Void" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"merchant_id\": \"00000000-0000-0000-0000-000000000001\",
    \"transaction_id\": \"TRANSACTION_ID_TO_VOID\",
    \"idempotency_key\": \"$(cat /proc/sys/kernel/random/uuid)\"
  }"
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `transaction_id` | UUID | Yes | Transaction to void (before settlement) |
| `idempotency_key` | UUID | Yes | Unique key to prevent duplicates |

</details>

---

### 1.5 Refund

```bash
curl -s -X POST "http://localhost:8081/payment.v1.PaymentService/Refund" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"merchant_id\": \"00000000-0000-0000-0000-000000000001\",
    \"transaction_id\": \"ORIGINAL_TRANSACTION_ID\",
    \"amount_cents\": 2500,
    \"reason\": \"Customer request\",
    \"idempotency_key\": \"$(cat /proc/sys/kernel/random/uuid)\"
  }"
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `transaction_id` | UUID | Yes | Original transaction to refund |
| `amount_cents` | int64 | No | Amount to refund (default: full amount) |
| `reason` | string | No | Reason for refund |
| `idempotency_key` | UUID | Yes | Unique key to prevent duplicates |

</details>

---

### 1.6 Get Transaction

```bash
curl -s -X POST "http://localhost:8081/payment.v1.PaymentService/GetTransaction" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"transaction_id": "TRANSACTION_ID_HERE"}'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `transaction_id` | UUID | Yes | Transaction to retrieve |

</details>

---

### 1.7 List Transactions

```bash
curl -s -X POST "http://localhost:8081/payment.v1.PaymentService/ListTransactions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "ea164cb1-3995-4f62-b331-f846ecf42f2d",
    "customer_id": "test-customer-001",
    "limit": 10
  }'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `customer_id` | string | No | Filter by customer |
| `order_id` | string | No | Filter by order ID |
| `status` | enum | No | Filter by status |
| `limit` | int32 | No | Max results (default: 100, max: 1000) |
| `offset` | int32 | No | Pagination offset |

</details>

---

## 2. Payment Method Service

### 2.1 List Payment Methods

```bash
curl -s -X POST "http://localhost:8081/payment_method.v1.PaymentMethodService/ListPaymentMethods" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "00000000-0000-0000-0000-000000000001",
    "customer_id": "00000000-0000-0000-0000-000000000010"
  }'
```

**Response:**
```json
{
  "paymentMethods": [
    {
      "id": "b921374c-643f-40cd-9aec-c24f92953ab7",
      "merchantId": "00000000-0000-0000-0000-000000000001",
      "customerId": "00000000-0000-0000-0000-000000000010",
      "paymentType": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
      "lastFour": "1111",
      "cardBrand": "Visa",
      "isActive": true,
      "isVerified": true,
      "createdAt": "2025-12-04T11:15:58.537846Z",
      "updatedAt": "2025-12-04T11:43:15.193827Z",
      "lastUsedAt": "2025-12-04T11:43:15.193827Z"
    }
  ],
  "totalCount": 1
}
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `limit` | int32 | No | Max results (default: 100) |
| `offset` | int32 | No | Pagination offset |

</details>

<details>
<summary>Response Fields</summary>

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Payment method identifier |
| `merchantId` | UUID | Merchant identifier |
| `customerId` | string | Customer identifier |
| `paymentType` | enum | Type of payment method |
| `lastFour` | string | Last 4 digits of card/account |
| `cardBrand` | string | Card brand (visa, mastercard, etc.) |
| `cardExpMonth` | int32 | Card expiration month |
| `cardExpYear` | int32 | Card expiration year |
| `isDefault` | bool | Is default payment method |
| `isActive` | bool | Is active/usable |

</details>

<details>
<summary>Payment Method Type Enum</summary>

| Value | Description |
|-------|-------------|
| `PAYMENT_METHOD_TYPE_CREDIT_CARD` | Credit/debit card |
| `PAYMENT_METHOD_TYPE_ACH` | ACH bank account |

</details>

---

### 2.2 Get Payment Method

```bash
curl -s -X POST "http://localhost:8081/payment_method.v1.PaymentMethodService/GetPaymentMethod" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "ea164cb1-3995-4f62-b331-f846ecf42f2d",
    "payment_method_id": "PAYMENT_METHOD_ID_HERE"
  }'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `payment_method_id` | UUID | Yes | Payment method to retrieve |

</details>

---

### 2.3 Set Default Payment Method

```bash
curl -s -X POST "http://localhost:8081/payment_method.v1.PaymentMethodService/SetDefaultPaymentMethod" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "ea164cb1-3995-4f62-b331-f846ecf42f2d",
    "customer_id": "test-customer-001",
    "payment_method_id": "PAYMENT_METHOD_ID_HERE"
  }'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `payment_method_id` | UUID | Yes | Payment method to set as default |

</details>

---

### 2.4 Delete Payment Method

```bash
curl -s -X POST "http://localhost:8081/payment_method.v1.PaymentMethodService/DeletePaymentMethod" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "ea164cb1-3995-4f62-b331-f846ecf42f2d",
    "payment_method_id": "PAYMENT_METHOD_ID_HERE"
  }'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `payment_method_id` | UUID | Yes | Payment method to delete |

</details>

---

## 3. Subscription Service

### 3.1 Create Subscription

```bash
curl -s -X POST "http://localhost:8081/subscription.v1.SubscriptionService/CreateSubscription" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"merchant_id\": \"ea164cb1-3995-4f62-b331-f846ecf42f2d\",
    \"customer_id\": \"test-customer-001\",
    \"amount_cents\": 2999,
    \"currency\": \"USD\",
    \"interval_value\": 1,
    \"interval_unit\": \"INTERVAL_UNIT_MONTH\",
    \"payment_method_id\": \"YOUR_PAYMENT_METHOD_ID\",
    \"start_date\": \"$(date -u -d '+1 day' +%Y-%m-%dT00:00:00Z)\",
    \"max_retries\": 3,
    \"idempotency_key\": \"$(cat /proc/sys/kernel/random/uuid)\"
  }"
```

**Response:**
```json
{"subscriptionId":"sub_xyz...","status":"SUBSCRIPTION_STATUS_ACTIVE","nextBillingDate":"2025-12-05T00:00:00Z"}
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `customer_id` | string | Yes | Customer identifier |
| `amount_cents` | int64 | Yes | Billing amount in cents |
| `currency` | string | Yes | ISO 4217 code (USD) |
| `interval_value` | int32 | Yes | Number of intervals between billings |
| `interval_unit` | enum | Yes | Interval unit (see below) |
| `payment_method_id` | UUID | Yes | Payment method to charge |
| `start_date` | timestamp | Yes | First billing date (RFC3339) |
| `max_retries` | int32 | No | Max retry attempts (default: 3) |
| `metadata` | map | No | Custom key-value pairs |
| `idempotency_key` | UUID | Yes | Unique key to prevent duplicates |

</details>

<details>
<summary>Interval Unit Enum</summary>

| Value | Description | Example |
|-------|-------------|---------|
| `INTERVAL_UNIT_DAY` | Daily billing | interval_value=1 = every day |
| `INTERVAL_UNIT_WEEK` | Weekly billing | interval_value=2 = every 2 weeks |
| `INTERVAL_UNIT_MONTH` | Monthly billing | interval_value=1 = every month |
| `INTERVAL_UNIT_YEAR` | Annual billing | interval_value=1 = every year |

</details>

<details>
<summary>Subscription Status Enum</summary>

| Value | Description |
|-------|-------------|
| `SUBSCRIPTION_STATUS_ACTIVE` | Active, will be billed |
| `SUBSCRIPTION_STATUS_PAUSED` | Paused, no billing until resumed |
| `SUBSCRIPTION_STATUS_CANCELLED` | Permanently cancelled |
| `SUBSCRIPTION_STATUS_PAST_DUE` | Payment failed after max retries |

</details>

---

### 3.2 Get Subscription

```bash
curl -s -X POST "http://localhost:8081/subscription.v1.SubscriptionService/GetSubscription" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"subscription_id": "66666666-6666-6666-6666-666666666666"}'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `subscription_id` | UUID | Yes | Subscription to retrieve |

</details>

<details>
<summary>Response Fields</summary>

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Subscription identifier |
| `merchantId` | UUID | Merchant identifier |
| `customerId` | string | Customer identifier |
| `amountCents` | string | Billing amount in cents |
| `currency` | string | Currency code |
| `intervalValue` | int32 | Interval count |
| `intervalUnit` | enum | Interval unit |
| `status` | enum | Subscription status |
| `paymentMethodId` | UUID | Payment method used |
| `nextBillingDate` | timestamp | Next billing date |
| `failureRetryCount` | int32 | Current retry count |
| `maxRetries` | int32 | Max retry attempts |
| `metadata` | map | Custom metadata |
| `createdAt` | timestamp | Creation timestamp |

</details>

---

### 3.3 List Subscriptions

```bash
curl -s -X POST "http://localhost:8081/subscription.v1.SubscriptionService/ListSubscriptions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "ea164cb1-3995-4f62-b331-f846ecf42f2d",
    "customer_id": "test-customer-001",
    "limit": 10
  }'
```

**Filter by status:**

```bash
curl -s -X POST "http://localhost:8081/subscription.v1.SubscriptionService/ListSubscriptions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "ea164cb1-3995-4f62-b331-f846ecf42f2d",
    "status": "SUBSCRIPTION_STATUS_ACTIVE",
    "limit": 10
  }'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `customer_id` | string | No | Filter by customer |
| `status` | enum | No | Filter by status |
| `limit` | int32 | No | Max results (default: 100) |
| `offset` | int32 | No | Pagination offset |

</details>

---

### 3.4 Update Subscription

```bash
curl -s -X POST "http://localhost:8081/subscription.v1.SubscriptionService/UpdateSubscription" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"subscription_id\": \"66666666-6666-6666-6666-666666666666\",
    \"amount_cents\": 3999,
    \"idempotency_key\": \"$(cat /proc/sys/kernel/random/uuid)\"
  }"
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `subscription_id` | UUID | Yes | Subscription to update |
| `amount_cents` | int64 | No | New billing amount |
| `interval_value` | int32 | No | New interval value |
| `interval_unit` | enum | No | New interval unit |
| `payment_method_id` | UUID | No | New payment method |
| `idempotency_key` | UUID | Yes | Unique key to prevent duplicates |

</details>

---

### 3.5 Pause Subscription

```bash
curl -s -X POST "http://localhost:8081/subscription.v1.SubscriptionService/PauseSubscription" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"subscription_id": "66666666-6666-6666-6666-666666666666"}'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `subscription_id` | UUID | Yes | Subscription to pause |

</details>

---

### 3.6 Resume Subscription

```bash
curl -s -X POST "http://localhost:8081/subscription.v1.SubscriptionService/ResumeSubscription" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"subscription_id": "77777777-7777-7777-7777-777777777777"}'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `subscription_id` | UUID | Yes | Subscription to resume |

</details>

---

### 3.7 Cancel Subscription

```bash
curl -s -X POST "http://localhost:8081/subscription.v1.SubscriptionService/CancelSubscription" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"subscription_id\": \"66666666-6666-6666-6666-666666666666\",
    \"cancel_at_period_end\": true,
    \"reason\": \"Customer request\",
    \"idempotency_key\": \"$(cat /proc/sys/kernel/random/uuid)\"
  }"
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `subscription_id` | UUID | Yes | Subscription to cancel |
| `cancel_at_period_end` | bool | No | If true, cancels after current period ends |
| `reason` | string | No | Cancellation reason |
| `idempotency_key` | UUID | Yes | Unique key to prevent duplicates |

</details>

---

## 4. Chargeback Service

### 4.1 Get Chargeback

```bash
curl -s -X POST "http://localhost:8081/chargeback.v1.ChargebackService/GetChargeback" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "chargeback_id": "CHARGEBACK_ID_HERE",
    "merchant_id": "ea164cb1-3995-4f62-b331-f846ecf42f2d"
  }'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `chargeback_id` | UUID | Yes | Chargeback to retrieve |
| `merchant_id` | UUID | Yes | Merchant identifier |

</details>

<details>
<summary>Response Fields</summary>

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Chargeback identifier |
| `transactionId` | UUID | Original transaction |
| `merchantId` | UUID | Merchant identifier |
| `customerId` | string | Customer identifier |
| `caseNumber` | string | Dispute case number |
| `disputeDate` | timestamp | When dispute was filed |
| `chargebackDate` | timestamp | When chargeback was created |
| `chargebackAmount` | string | Disputed amount |
| `currency` | string | Currency code |
| `reasonCode` | string | Network reason code |
| `reasonDescription` | string | Human-readable reason |
| `status` | enum | Chargeback status |
| `respondByDate` | timestamp | Deadline for response |

</details>

<details>
<summary>Chargeback Status Enum</summary>

| Value | Description |
|-------|-------------|
| `CHARGEBACK_STATUS_NEW` | Just received |
| `CHARGEBACK_STATUS_PENDING` | Under review |
| `CHARGEBACK_STATUS_RESPONDED` | Evidence submitted |
| `CHARGEBACK_STATUS_WON` | Merchant won dispute |
| `CHARGEBACK_STATUS_LOST` | Merchant lost dispute |
| `CHARGEBACK_STATUS_ACCEPTED` | Merchant accepted chargeback |

</details>

---

### 4.2 List Chargebacks

```bash
curl -s -X POST "http://localhost:8081/chargeback.v1.ChargebackService/ListChargebacks" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "merchant_id": "ea164cb1-3995-4f62-b331-f846ecf42f2d",
    "limit": 10
  }'
```

<details>
<summary>Request Fields</summary>

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `merchant_id` | UUID | Yes | Merchant identifier |
| `status` | enum | No | Filter by status |
| `customer_id` | string | No | Filter by customer |
| `transaction_id` | UUID | No | Filter by transaction |
| `dispute_date_from` | timestamp | No | Filter from date |
| `dispute_date_to` | timestamp | No | Filter to date |
| `limit` | int32 | No | Max results (default: 100) |
| `offset` | int32 | No | Pagination offset |

</details>

---

## 5. Browser POST

### 5.1 Credit Card STORAGE (tokenize card)

```bash
curl -s "http://localhost:8081/api/v1/payments/browser-post/form?merchant_id=ea164cb1-3995-4f62-b331-f846ecf42f2d&transaction_type=STORAGE&amount=0.00&customer_id=test-customer-001&return_url=http://localhost:8081/api/v1/payments/browser-post/callback&transaction_id=$(cat /proc/sys/kernel/random/uuid)" \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "custNbr": "9001",
  "dbaName": "2",
  "epxTranNbr": "1753144461",
  "expiresAt": 1764862995,
  "industryType": "E",
  "merchNbr": "900300",
  "merchantId": "ea164cb1-3995-4f62-b331-f846ecf42f2d",
  "merchantName": "Test Merchant (Development)",
  "postURL": "https://services.epxuap.com/browserpost/",
  "redirectURL": "http://localhost:8081/api/v1/payments/browser-post/callback?customer_id=test-customer-001&merchant_id=ea164cb1-3995-4f62-b331-f846ecf42f2d&transaction_id=25151953-4f59-4f46-8291-8bfe21d1304f&transaction_type=STORAGE",
  "returnUrl": "http://localhost:8081/api/v1/payments/browser-post/callback",
  "tac": "wm8k2HbA3pa8Ooh9kTUHMg==|YUxQcNGZt+laUsMcp6EQ...",
  "terminalNbr": "77",
  "tranCode": "STORAGE",
  "transactionId": "25151953-4f59-4f46-8291-8bfe21d1304f"
}
```

<details>
<summary>Query Parameters</summary>

| Parameter | Required | Description |
|-----------|----------|-------------|
| `merchant_id` | Yes | Merchant UUID |
| `transaction_type` | Yes | SALE, AUTH, STORAGE, ACH_STORAGE_C, ACH_STORAGE_S |
| `amount` | Yes | Amount as decimal string (e.g., "50.00", "0.00" for STORAGE) |
| `customer_id` | Yes | Customer identifier |
| `return_url` | Yes | Callback URL after EPX processing |
| `transaction_id` | Yes | Your unique transaction ID |

</details>

<details>
<summary>Transaction Type Enum</summary>

| Value | Description |
|-------|-------------|
| `SALE` | Charge card immediately |
| `AUTH` | Hold funds, capture later |
| `STORAGE` | Tokenize credit card only |
| `ACH_STORAGE_C` | Tokenize checking account |
| `ACH_STORAGE_S` | Tokenize savings account |

</details>

<details>
<summary>Response Fields</summary>

| Field | Description |
|-------|-------------|
| `tac` | Transaction Authentication Code for EPX |
| `postURL` | EPX Browser POST endpoint |
| `tranCode` | Transaction code for form |
| `custNbr` | EPX customer number |
| `merchNbr` | EPX merchant number |
| `terminalNbr` | EPX terminal number |
| `transactionId` | Your transaction ID |
| `redirectURL` | Full callback URL with params |

</details>

---

### 5.2 Credit Card SALE

```bash
curl -s "http://localhost:8081/api/v1/payments/browser-post/form?merchant_id=ea164cb1-3995-4f62-b331-f846ecf42f2d&transaction_type=SALE&amount=50.00&customer_id=test-customer-001&return_url=http://localhost:8081/api/v1/payments/browser-post/callback&transaction_id=$(cat /proc/sys/kernel/random/uuid)" \
  -H "Authorization: Bearer $TOKEN"
```

---

### 5.3 ACH Checking STORAGE

```bash
curl -s "http://localhost:8081/api/v1/payments/browser-post/form?merchant_id=ea164cb1-3995-4f62-b331-f846ecf42f2d&transaction_type=ACH_STORAGE_C&amount=0.00&customer_id=test-customer-001&return_url=http://localhost:8081/api/v1/payments/browser-post/callback&transaction_id=$(cat /proc/sys/kernel/random/uuid)" \
  -H "Authorization: Bearer $TOKEN"
```

---

## 6. Error Codes

<details>
<summary>ConnectRPC Error Codes</summary>

| Code | HTTP | Description |
|------|------|-------------|
| `invalid_argument` | 400 | Invalid request format or field values |
| `unauthenticated` | 401 | Missing, invalid, or expired JWT token |
| `permission_denied` | 403 | Service lacks access to merchant |
| `not_found` | 404 | Resource does not exist |
| `already_exists` | 409 | Duplicate idempotency key |
| `failed_precondition` | 412 | Invalid state for operation |
| `internal` | 500 | Server error |

</details>

<details>
<summary>Common Error Messages</summary>

| Message | Cause | Solution |
|---------|-------|----------|
| `invalid idempotency_key format` | Non-UUID idempotency key | Use `$(cat /proc/sys/kernel/random/uuid)` |
| `payment method does not belong to customer` | customer_id mismatch | Use correct customer_id |
| `failed to retrieve merchant credentials` | Invalid mac_secret_path | Check merchant config |
| `transaction not found` | Invalid transaction_id | Verify transaction exists |

</details>

---

## Related Docs

- [ADMIN_CLI.md](./ADMIN_CLI.md) - Service/merchant management
- [BROWSER_POST_FORM_SETUP.md](./BROWSER_POST_FORM_SETUP.md) - PCI tokenization guide
- [TOKEN_GENERATION.md](./TOKEN_GENERATION.md) - JWT authentication
- [certification_sheets.md](./certification_sheets.md) - EPX certification tests
