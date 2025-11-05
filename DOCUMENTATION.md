# Payment Microservice - Complete Documentation

**Version:** v0.2.0-alpha
**Last Updated:** 2025-10-29

---

## Table of Contents

1. [Introduction](#1-introduction)
   - [Overview](#overview)
   - [Key Features](#key-features)
   - [Technology Stack](#technology-stack)
2. [Quick Start](#2-quick-start)
   - [Prerequisites](#prerequisites)
   - [Local Setup](#local-setup)
   - [Docker Setup](#docker-setup)
   - [Testing](#testing)
3. [Architecture](#3-architecture)
   - [Hexagonal Architecture](#hexagonal-architecture)
   - [System Design](#system-design)
   - [Component Structure](#component-structure)
   - [Benefits](#architecture-benefits)
4. [Payment Integration](#4-payment-integration)
   - [Frontend Integration](#frontend-integration)
   - [Backend API](#backend-api)
   - [Payment Flows](#payment-flows)
5. [North Gateway Integration](#5-north-gateway-integration)
   - [Available APIs](#available-apis)
   - [Authentication](#authentication)
   - [Response Codes](#response-codes)
6. [Chargeback Management](#6-chargeback-management)
   - [Overview](#chargeback-overview)
   - [Polling Architecture](#polling-architecture)
   - [Database Schema](#chargeback-database-schema)
   - [gRPC API](#chargeback-grpc-api)
7. [Webhook System](#7-webhook-system)
   - [Architecture](#webhook-architecture)
   - [Event Types](#event-types)
   - [Security](#webhook-security)
   - [Retry Logic](#retry-logic)
8. [Database](#8-database)
   - [Schema](#database-schema)
   - [Migrations](#database-migrations)
   - [Queries](#database-queries)
9. [Testing](#9-testing)
   - [Unit Tests](#unit-tests)
   - [Integration Tests](#integration-tests)
   - [Test Coverage](#test-coverage)
10. [Deployment](#10-deployment)
    - [Environment Variables](#environment-variables)
    - [Production Setup](#production-setup)
    - [Security](#deployment-security)
11. [API Reference](#11-api-reference)
    - [Payment APIs](#payment-apis)
    - [Subscription APIs](#subscription-apis)
    - [Chargeback APIs](#chargeback-apis)
12. [Troubleshooting](#12-troubleshooting)
    - [Common Issues](#common-issues)
    - [Debugging](#debugging)

---

## 1. Introduction

### Overview

A production-ready payment microservice built with **Go** and **gRPC**, integrating with **North Payment Gateway (EPX)** using clean **Hexagonal Architecture (Ports & Adapters)** pattern.

**Core Capabilities:**
- ✅ Credit card payments (one-time, auth/capture)
- ✅ Recurring billing & subscriptions
- ✅ ACH bank transfers
- ✅ Chargeback tracking (READ-ONLY polling)
- ✅ Webhook notifications
- ✅ PCI-compliant tokenization
- ✅ Comprehensive observability

### Key Features

#### Payment Operations
- **Browser Post Tokenization**: Frontend tokenizes cards directly with North (PCI-compliant)
- **Auth/Capture Flows**: Two-step payment authorization and capture
- **One-Time Payments**: Immediate sales and purchases
- **Idempotency**: Prevent duplicate charges with idempotency keys

#### Subscription Management
- **Recurring Billing**: Automatic subscription charging via cron jobs
- **Flexible Frequencies**: Weekly, bi-weekly, monthly, yearly
- **Lifecycle Management**: Pause, resume, cancel subscriptions
- **Failure Handling**: Configurable retry logic and failure options

#### Chargeback Management
- **Automated Polling**: Cron job polls North Merchant Reporting API every 4 hours
- **Local Storage**: Chargebacks stored in PostgreSQL for fast queries
- **READ-ONLY**: Disputes are handled online at North's portal - we only track and notify
- **Webhook Notifications**: Automatic notifications for new/updated chargebacks

#### Webhook System
- **Outbound Webhooks**: Notify merchants of chargeback events
- **HMAC Signatures**: Secure webhook verification with HMAC-SHA256
- **Automatic Retries**: Exponential backoff retry logic for failed deliveries
- **Delivery Tracking**: Complete audit trail of all webhook deliveries

#### Security & Compliance
- **PCI-Reduced Scope**: Backend never handles raw card data
- **HMAC Authentication**: All North API calls use HMAC-SHA256 signatures
- **TLS 1.3**: Encrypted communication
- **Token-Only Processing**: Only BRIC tokens processed by backend

#### Observability
- **Prometheus Metrics**: Business and technical metrics
- **Structured Logging**: Zap logger with JSON output
- **Health Checks**: Liveness and readiness probes
- **gRPC Interceptors**: Request tracking and error handling

### Technology Stack

**Core:**
- **Language**: Go 1.24+
- **API Protocol**: gRPC with Protocol Buffers
- **Database**: PostgreSQL 15+ with JSONB support
- **Code Generation**: sqlc for type-safe SQL queries

**Libraries:**
- `google.golang.org/grpc` - gRPC server
- `github.com/jackc/pgx/v5` - PostgreSQL driver
- `github.com/shopspring/decimal` - Precise decimal math for money
- `go.uber.org/zap` - Structured logging
- `github.com/stretchr/testify` - Testing framework

**Infrastructure:**
- **Containerization**: Docker & Docker Compose
- **Orchestration**: Kubernetes (optional)
- **Metrics**: Prometheus
- **Migrations**: SQL-based with golang-migrate

---

## 2. Quick Start

### Prerequisites

- Go 1.24+
- PostgreSQL 15+
- Docker & Docker Compose
- protoc (Protocol Buffers compiler)

### Local Setup

```bash
# Clone repository
git clone https://github.com/kevin07696/payment-service.git
cd payment-service

# Install dependencies
go mod download

# Configure environment
cp .env.example .env
# Edit .env with your database and North gateway credentials

# Start test database
docker compose -f docker-compose.test.yml up -d

# Run tests
go test ./... -cover

# Build server
go build -o bin/payment-server ./cmd/server

# Run server
./bin/payment-server
```

Server starts on:
- **gRPC**: `0.0.0.0:8080`
- **HTTP (cron)**: `0.0.0.0:8081`
- **Metrics**: `http://localhost:9090/metrics`
- **Health**: `http://localhost:9090/health`

### Docker Setup

```bash
# Start all services (PostgreSQL + payment server)
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

**Services:**
- **gRPC API**: `localhost:8080`
- **Prometheus Metrics**: `http://localhost:9090/metrics`
- **Health Check**: `http://localhost:9090/health`
- **PostgreSQL**: `localhost:5432`

### Testing

```bash
# Run all tests
make test

# Run only unit tests (no database)
make test-unit

# Run integration tests (requires database)
make test-integration

# Run with coverage
make test-cover
```

**Test Coverage:**
- North Adapters: 85.7%
- HMAC Authentication: 100%
- Response Code Mapper: 100%
- Integration Tests: Repository, Services

---

## 3. Architecture

### Hexagonal Architecture

This service implements **Hexagonal Architecture (Ports & Adapters)** pattern with strict dependency injection through interfaces.

```
┌─────────────────────────────────────────────────────────────────┐
│                     gRPC API Layer                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Payment    │  │ Subscription │  │  Chargeback  │         │
│  │   Handlers   │  │   Handlers   │  │   Handlers   │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────┴────────────────────────────────────┐
│                   Service Layer (Business Logic)                │
│  - Payment Service      - Subscription Service                  │
│  - Idempotency          - Transaction Management                │
│  - Webhook Delivery     - Cron Job Handlers                     │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────┴────────────────────────────────────┐
│                   Domain Layer (Core)                           │
│  ┌────────────────────────────────────────────────────────┐    │
│  │  Ports (Interfaces)                                    │    │
│  │  - CreditCardGateway  - RecurringBillingGateway        │    │
│  │  - Logger             - HTTPClient                     │    │
│  │  - TransactionRepo    - SubscriptionRepo               │    │
│  └────────────────────────────────────────────────────────┘    │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────┴────────────────────────────────────┐
│                   Infrastructure Layer (Adapters)               │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐   │
│  │ North Adapters │  │   PostgreSQL   │  │  Observability │   │
│  │ - Browser Post │  │  Repositories  │  │ - Prometheus   │   │
│  │ - Recurring    │  │  - sqlc        │  │ - Zap Logger   │   │
│  │ - ACH          │  │  - Migrations  │  │ - Health       │   │
│  │ - Merchant API │  │                │  │                │   │
│  └────────────────┘  └────────────────┘  └────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

**Key Principle**: Domain layer defines interfaces (ports), infrastructure provides implementations (adapters). Dependencies always point inward.

### System Design

**Directory Structure:**

```
payment-service/
├── cmd/
│   └── server/             # gRPC server entry point
├── internal/
│   ├── domain/             # Core business entities & ports
│   │   ├── models/         # Transaction, Subscription, etc.
│   │   └── ports/          # Interface contracts
│   ├── adapters/           # External system implementations
│   │   ├── north/          # North gateway adapters
│   │   ├── database/       # PostgreSQL adapter
│   │   └── secrets/        # Secret manager adapters
│   ├── handlers/           # gRPC & HTTP handlers
│   │   ├── payment/        # Payment gRPC handlers
│   │   ├── subscription/   # Subscription gRPC handlers
│   │   ├── chargeback/     # Chargeback gRPC handlers
│   │   └── cron/           # Cron HTTP handlers
│   ├── services/           # Business logic
│   │   ├── payment/        # Payment service
│   │   ├── subscription/   # Subscription service
│   │   └── webhook/        # Webhook delivery service
│   └── db/                 # Database layer
│       ├── migrations/     # SQL migrations
│       ├── queries/        # SQL queries for sqlc
│       └── sqlc/           # Generated code
├── proto/              # Protocol buffer definitions
│   ├── payment/v1/
│   ├── subscription/v1/
│   └── chargeback/v1/
├── test/
│   ├── integration/        # Integration tests
│   └── mocks/              # Mock implementations
└── docs/                   # Documentation
```

### Component Structure

**1. Domain Models** (`internal/domain/`)
```go
type Transaction struct {
    ID               uuid.UUID
    GroupID          uuid.UUID
    AgentID          string
    CustomerID       string
    Amount           string
    Currency         string
    Status           string
    Type             string
    PaymentMethodType string
    Token            string
    ResponseCode     string
    ResponseMessage  string
    IdempotencyKey   string
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

type Subscription struct {
    ID                  uuid.UUID
    AgentID             string
    CustomerID          string
    Amount              string
    Currency            string
    Frequency           string
    Status              string
    PaymentMethodToken  string
    NextBillingDate     time.Time
    MaxRetries          int32
    FailureOption       string
    CreatedAt           time.Time
    UpdatedAt           time.Time
}

type Chargeback struct {
    ID                 uuid.UUID
    GroupID            uuid.UUID
    AgentID            string
    CustomerID         string
    CaseNumber         string
    DisputeDate        time.Time
    ChargebackDate     time.Time
    ChargebackAmount   string
    Currency           string
    ReasonCode         string
    ReasonDescription  string
    Status             string
    RawData            pgtype.JSONB
    CreatedAt          time.Time
    UpdatedAt          time.Time
}
```

**2. Port Interfaces**

**Adapter Ports** (`internal/adapters/ports/`)
```go
// EPX Gateway Integration
type BrowserPostAdapter interface {
    GeneratePaymentURL(ctx context.Context, req *BrowserPostRequest) (*BrowserPostResponse, error)
}

type ServerPostAdapter interface {
    ProcessPayment(ctx context.Context, req *ServerPostRequest) (*ServerPostResponse, error)
    RefundPayment(ctx context.Context, req *RefundRequest) (*RefundResponse, error)
}

// North Merchant Reporting (Read-Only)
type MerchantReportingAdapter interface {
    SearchDisputes(ctx context.Context, req *DisputeSearchRequest) (*DisputeSearchResponse, error)
}

// Infrastructure Interfaces
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

type Logger interface {
    Info(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Debug(msg string, fields ...Field)
}
```

**Service Ports** (`internal/services/ports/`)
```go
type PaymentService interface {
    ProcessPayment(ctx context.Context, req *ProcessPaymentRequest) (*PaymentResponse, error)
    RefundPayment(ctx context.Context, req *RefundPaymentRequest) (*PaymentResponse, error)
}

type SubscriptionService interface {
    CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*SubscriptionResponse, error)
    CancelSubscription(ctx context.Context, req *CancelSubscriptionRequest) (*SubscriptionResponse, error)
}
```

### Architecture Benefits

**Why Ports & Adapters?**

1. **Easy Testing**: Mock all external dependencies
   ```go
   // Test with mocks
   mockGateway := mocks.NewMockGateway()
   mockRepo := mocks.NewMockRepository()
   service := NewPaymentService(mockGateway, mockRepo, mockLogger)
   ```

2. **Swappable Implementations**: Change implementations without code changes
   ```go
   // Development: verbose logging
   devLogger := security.NewZapLoggerDevelopment()

   // Production: JSON logging
   prodLogger := security.NewZapLoggerProduction()

   // Same interface, different implementations
   ```

3. **Easy Migration**: Switch payment providers without changing business logic
   ```go
   // Current: North
   northAdapter := north.NewBrowserPostAdapter(...)

   // Future: Stripe (if needed)
   stripeAdapter := stripe.NewStripeAdapter(...)

   // Both implement CreditCardGateway interface
   ```

4. **Parallel Development**: Teams work on interfaces independently
5. **Future-Proofing**: Add features by wrapping, not modifying

---

## 4. Payment Integration

### Frontend Integration

**Tokenization Flow** (PCI-Compliant):

```
User Browser → North SDK → BRIC Token → Your Backend
  (card data)   (tokenize)  (returns)   (process payment)
```

**Step 1: Include North Browser Post SDK**

```html
<!DOCTYPE html>
<html>
<head>
    <script src="https://secure.epxuap.com/browserpost.js"></script>
</head>
<body>
    <form id="payment-form">
        <input id="card-number" placeholder="4111 1111 1111 1111" />
        <input id="exp-month" placeholder="MM" maxlength="2" />
        <input id="exp-year" placeholder="YYYY" maxlength="4" />
        <input id="cvv" placeholder="123" maxlength="4" />
        <button type="submit">Pay Now</button>
    </form>
</body>
</html>
```

**Step 2: Tokenize Card**

```javascript
document.getElementById('payment-form').addEventListener('submit', function(e) {
    e.preventDefault();

    const cardData = {
        cardNumber: document.getElementById('card-number').value,
        expMonth: document.getElementById('exp-month').value,
        expYear: document.getElementById('exp-year').value,
        cvv: document.getElementById('cvv').value
    };

    // Tokenize with North (direct to North, bypasses your backend)
    NorthBrowserPost.tokenize(cardData, function(response) {
        if (response.success && response.token) {
            // Send token to your backend
            processPaymentWithToken(response.token);
        }
    });
});
```

**Step 3: Send Token to Backend**

```javascript
async function processPaymentWithToken(bricToken) {
    const response = await fetch('http://localhost:8080/api/payment/authorize', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer YOUR_JWT_TOKEN'
        },
        body: JSON.stringify({
            merchantId: 'MERCH-001',
            customerId: 'CUST-12345',
            amount: '99.99',
            currency: 'USD',
            token: bricToken,           // BRIC token (NOT card number!)
            capture: true,              // true = immediate capture
            idempotencyKey: generateIdempotencyKey(),
            metadata: {
                orderId: 'ORDER-123'
            }
        })
    });

    const result = await response.json();

    if (result.status === 'TRANSACTION_STATUS_CAPTURED') {
        // Payment successful!
        window.location.href = '/success';
    } else {
        // Payment failed
        alert(result.message);
    }
}
```

### Backend API

**gRPC Services:**

```protobuf
service PaymentService {
  rpc Authorize(AuthorizeRequest) returns (Transaction);
  rpc Capture(CaptureRequest) returns (Transaction);
  rpc Void(VoidRequest) returns (Transaction);
  rpc Refund(RefundRequest) returns (Transaction);
  rpc Sale(SaleRequest) returns (Transaction);
  rpc GetTransaction(GetTransactionRequest) returns (Transaction);
  rpc ListTransactions(ListTransactionsRequest) returns (ListTransactionsResponse);
}

service SubscriptionService {
  rpc CreateSubscription(CreateSubscriptionRequest) returns (Subscription);
  rpc UpdateSubscription(UpdateSubscriptionRequest) returns (Subscription);
  rpc CancelSubscription(CancelSubscriptionRequest) returns (Subscription);
  rpc GetSubscription(GetSubscriptionRequest) returns (Subscription);
  rpc ListSubscriptions(ListSubscriptionsRequest) returns (ListSubscriptionsResponse);
}
```

### Payment Flows

**Flow 1: One-Time Payment**

```
Client App
    │
    │  1. Tokenize card with North SDK
    │     (frontend → North, returns BRIC token)
    │
    │  2. gRPC Authorize
    │     (BRIC token, amount, customer info)
    v
Payment Handler
    │
    │  3. Check idempotency
    v
Payment Service
    │
    │  4. Call Browser Post Adapter
    v
North Gateway (EPX)
    │
    │  5. Process payment
    │     Return response code (00 = approved)
    v
Save to Database
    │
    │  6. Return transaction to client
    v
Client Response
```

**Flow 2: Recurring Subscription**

```
Client App
    │  1. Create subscription
    │     (BRIC token, amount, frequency)
    v
Subscription Service
    │  2. Validate & create in DB
    │  3. Set next_billing_date
    v
Database

(Later...)

Cron Job (runs periodically)
    │  1. Find subscriptions due for billing
    v
Subscription Service
    │  2. Process billing for each subscription
    │  3. Update next_billing_date
    │  4. Handle failures (retry/pause/skip)
    v
Payment Service
    │  5. Charge stored payment method
    v
North Gateway
```

---

## 5. North Gateway Integration

### Available APIs

**1. Browser Post API** ✅ (One-Time Payments)
- **Purpose**: Tokenized one-time payments with PCI-compliant frontend-to-EPX flow
- **Adapter**: `BrowserPostAdapter`
- **Callback Handler**: `BrowserPostCallbackHandler` (HTTP endpoint)
- **Best For**: Checkout, one-time purchases, guest checkouts
- **Operations**: Authorize, Capture, Void, Refund
- **Flow**: Browser → EPX (payment) → Backend (via REDIRECT_URL) → User (receipt page)

**2. Recurring Billing API** ✅ (Subscriptions)
- **Purpose**: Store payment methods & recurring billing
- **Adapter**: `RecurringBillingAdapter`
- **Best For**: Subscriptions, stored payment methods
- **Operations**: Create, Update, Cancel, Pause, Resume subscriptions

**3. ACH API** ✅ (Bank Transfers)
- **Purpose**: Bank account payments (checking/savings)
- **Adapter**: `ACHAdapter`
- **Best For**: Large transactions, B2B payments
- **Operations**: Debit, Credit, Verify account

**4. Merchant Reporting API** ✅ (Chargebacks - READ ONLY)
- **Purpose**: Retrieve chargeback/dispute data
- **Adapter**: `MerchantReportingAdapter`
- **Best For**: Tracking chargebacks, reporting
- **Operations**: Search disputes (read-only)

### Authentication

**HMAC-SHA256 Signature:**

```go
// Calculate signature
func calculateSignature(epiKey, endpoint string, payload []byte) string {
    concat := []byte(endpoint)
    concat = append(concat, payload...)
    h := hmac.New(sha256.New, []byte(epiKey))
    h.Write(concat)
    return hex.EncodeToString(h.Sum(nil))
}

// Headers sent with every request
EPI-Id: CUST_NBR-MERCH_NBR-DBA_NBR-TERMINAL_NBR
EPI-Signature: <hmac_signature>
```

### Response Codes

Common response codes from North:

| Code | Display | Category | Retriable | User Message |
|------|---------|----------|-----------|--------------|
| 00 | APPROVAL | Approved | No | Payment successful |
| 51 | INSUFF FUNDS | Insufficient Funds | Yes | Insufficient funds. Please use a different payment method. |
| 54 | EXP CARD | Expired Card | Yes | Your card has expired. |
| 82 | CVV ERROR | Invalid Card | Yes | Incorrect CVV. Please check the security code. |
| 59 | SUSPECTED FRAUD | Fraud | No | Transaction declined for security reasons. |
| 96 | SYSTEM ERROR | System Error | Yes | System error. Please try again. |

### Browser Post Complete Flow

**EPX Browser Post** is a PCI-compliant payment flow where the user's browser posts card data directly to EPX (never touching your backend), and EPX redirects back with transaction results.

#### Flow Diagram

```
┌──────────────┐
│  Your Backend│
│              │  1. Generate TAC Token
│              │  ← Key Exchange API
└──────┬───────┘
       │ 2. Return payment form HTML
       │    (with TAC + REDIRECT_URL)
       ▼
┌──────────────┐
│ User Browser │
│              │  3. User enters card details
│              │  4. Form POSTs to EPX
│              │  → https://epxnow.com/epx/browser_post
└──────┬───────┘
       │
       ▼
┌──────────────┐
│     EPX      │
│              │  5. Process payment
│              │  6. Redirect browser to REDIRECT_URL
│              │  → POST to your callback endpoint
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Your Backend│
│  /api/v1/    │  7. Parse transaction results
│  payments/   │  8. Validate response
│  browser-    │  9. Store in database (with AUTH_GUID)
│  post/       │  10. Render HTML receipt page
│  callback    │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ User Browser │
│              │  11. See success/failure page
│              │      with transaction details
└──────────────┘
```

#### REDIRECT_URL Configuration

**CRITICAL**: EPX requires a `REDIRECT_URL` to be configured with your Browser Post credentials. This is where EPX sends transaction results.

**For Local Development:**
```
http://localhost:8081/api/v1/payments/browser-post/callback
```

**For Production:**
```
https://yourdomain.com/api/v1/payments/browser-post/callback
```

#### Implementation Details

**1. Backend Callback Handler**
- **File**: `internal/handlers/payment/browser_post_callback_handler.go`
- **Endpoint**: `POST /api/v1/payments/browser-post/callback`
- **Port**: `8081` (HTTP server, same as cron endpoints)

**2. What the Callback Handler Does:**
```go
1. Receives POST redirect from EPX with form-encoded transaction results
2. Parses response using BrowserPostAdapter.ParseRedirectResponse()
3. Validates AUTH_GUID and AUTH_RESP fields
4. Checks for duplicate transactions using TRAN_NBR (idempotency)
5. Stores transaction in database:
   - AUTH_GUID (BRIC) - Required for refunds, voids, disputes
   - AUTH_RESP - Approval status ("00" = approved)
   - AUTH_CODE - Bank authorization code
   - Card verification fields (AVS, CVV2)
6. Renders HTML receipt page to user
   - Success: Shows masked card, auth code, transaction ID
   - Failure: Shows error message with retry button
```

**3. Why Store AUTH_GUID for Guest Checkouts?**

Even though Browser Post is typically used for guest checkouts (no saved payment method), we MUST store the `AUTH_GUID` (BRIC token) because it's required for:
- **Refunds**: Most common post-transaction operation
- **Voids**: Cancel transaction before settlement
- **Chargeback Defense**: Reference original transaction
- **Reconciliation**: Match with EPX settlement reports

**4. Duplicate Detection (PRG Pattern)**

EPX implements the POST-REDIRECT-GET pattern, meaning:
- Transaction is processed once
- Browser is redirected to get the response
- If user clicks "Back" or "Refresh", same response is returned
- Your handler checks `TRAN_NBR` before inserting to prevent duplicates

**5. Response Fields Received:**

| Field | Description | Example | Purpose |
|-------|-------------|---------|---------|
| AUTH_GUID | Transaction token (BRIC) | `0V703LH1HDL006J74W1` | Refunds, voids, tracking |
| AUTH_RESP | Approval code | `00` (approved) | Determine success/failure |
| AUTH_CODE | Bank authorization | `123456` | Chargeback defense |
| AUTH_RESP_TEXT | Human message | `APPROVED` | Display to user |
| AUTH_CARD_TYPE | Card brand | `V` (Visa) | Reporting, fees |
| AUTH_AVS | Address verification | `Y` (match) | Fraud scoring |
| AUTH_CVV2 | CVV verification | `M` (match) | Fraud scoring |
| TRAN_NBR | Your transaction number | `TXN-12345` | Idempotency |
| AMOUNT | Transaction amount | `99.99` | Verification |

---

## 6. Chargeback Management

### Chargeback Overview

**IMPORTANT: Disputes are handled online at North's portal. We only READ chargeback data for tracking and notification purposes.**

**What We Do:**
- ✅ Poll North Merchant Reporting API for chargeback data
- ✅ Store chargebacks in local database
- ✅ Query chargebacks via gRPC
- ✅ Send webhook notifications for new/updated chargebacks

**What We DON'T Do:**
- ❌ Submit dispute responses (done online at North's portal)
- ❌ Upload evidence files
- ❌ Manage representment

### Polling Architecture

```
┌──────────────────────────────────────────────────────────────┐
│ 1. Cloud Scheduler (every 4 hours)                          │
│    POST /cron/sync-disputes                                  │
└────────────────┬─────────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────────┐
│ 2. DisputeSyncHandler                                        │
│    internal/handlers/cron/dispute_sync_handler.go            │
└────────────────┬─────────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────────┐
│ 3. North Merchant Reporting Adapter                         │
│    GET /merchant/disputes/mid/search                         │
│    (Polls for new/updated chargebacks)                       │
└────────────────┬─────────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────────┐
│ 4. Store/Update in chargebacks table                        │
│    - If new → INSERT + trigger chargeback.created webhook   │
│    - If exists → UPDATE + trigger chargeback.updated webhook│
└────────────────┬─────────────────────────────────────────────┘
                 │
                 ▼
┌──────────────────────────────────────────────────────────────┐
│ 5. Merchants query via gRPC                                  │
│    ChargebackService/GetChargeback                           │
│    ChargebackService/ListChargebacks                         │
│    (Queries local DB, NOT North API)                         │
└──────────────────────────────────────────────────────────────┘
```

**Polling Configuration:**

```bash
# Cloud Scheduler setup (every 4 hours)
gcloud scheduler jobs create http sync-disputes \
  --schedule="0 */4 * * *" \
  --uri="https://your-app.com/cron/sync-disputes" \
  --http-method=POST \
  --headers="X-Cron-Secret=your-production-secret" \
  --message-body='{"days_back":7}'
```

### Chargeback Database Schema

```sql
CREATE TABLE chargebacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID,                        -- Links to transaction
    agent_id VARCHAR(100) NOT NULL,       -- Merchant/agent
    customer_id VARCHAR(100),             -- Customer (if known)

    -- North API fields
    case_number VARCHAR(255) UNIQUE NOT NULL,
    dispute_date TIMESTAMPTZ NOT NULL,
    chargeback_date TIMESTAMPTZ NOT NULL,
    chargeback_amount VARCHAR(255) NOT NULL,  -- Decimal as string
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    reason_code VARCHAR(50) NOT NULL,
    reason_description TEXT,

    -- Status
    status VARCHAR(50) NOT NULL,          -- new, pending, won, lost

    -- Full North API response for debugging
    raw_data JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_chargebacks_agent_id ON chargebacks(agent_id);
CREATE INDEX idx_chargebacks_case_number ON chargebacks(case_number);
CREATE INDEX idx_chargebacks_group_id ON chargebacks(group_id);
CREATE INDEX idx_chargebacks_dispute_date ON chargebacks(dispute_date DESC);
```

### Chargeback gRPC API

```protobuf
service ChargebackService {
  rpc GetChargeback(GetChargebackRequest) returns (Chargeback);
  rpc ListChargebacks(ListChargebacksRequest) returns (ListChargebacksResponse);
  rpc GetChargebackStats(GetChargebackStatsRequest) returns (ChargebackStatsResponse);
}

message ListChargebacksRequest {
  string agent_id = 1;                    // Required
  optional string customer_id = 2;        // Filter by customer
  optional string group_id = 3;           // Filter by transaction
  optional ChargebackStatus status = 4;   // Filter by status
  optional google.protobuf.Timestamp dispute_date_from = 5;
  optional google.protobuf.Timestamp dispute_date_to = 6;
  int32 limit = 7;
  int32 offset = 8;
}
```

**Query Examples:**

```bash
# Get single chargeback
grpcurl -plaintext -d '{
  "chargeback_id": "550e8400-e29b-41d4-a716-446655440000",
  "agent_id": "merchant-123"
}' localhost:8080 chargeback.v1.ChargebackService/GetChargeback

# List chargebacks with filters
grpcurl -plaintext -d '{
  "agent_id": "merchant-123",
  "status": "CHARGEBACK_STATUS_NEW",
  "limit": 50
}' localhost:8080 chargeback.v1.ChargebackService/ListChargebacks
```

---

## 7. Webhook System

### Webhook Architecture

**Purpose**: Notify merchants when chargebacks are created or updated.

**Components:**
- `webhook_subscriptions` table - Stores merchant webhook URLs
- `webhook_deliveries` table - Tracks all delivery attempts
- `WebhookDeliveryService` - Sends webhooks with HMAC signatures
- Automatic retry with exponential backoff

### Event Types

#### chargeback.created
Fired when a new chargeback is detected from North API.

#### chargeback.updated
Fired when an existing chargeback's status or amount changes.

### Webhook Payload

```json
{
  "event_type": "chargeback.created",
  "agent_id": "merchant-123",
  "timestamp": "2025-10-29T12:00:00Z",
  "data": {
    "chargeback_id": "uuid",
    "case_number": "12345",
    "status": "new",
    "amount": "30.00",
    "currency": "USD",
    "reason_code": "P22",
    "reason_description": "Non-Matching Card Number",
    "dispute_date": "2024-03-08",
    "chargeback_date": "2024-03-18"
  }
}
```

### Webhook Security

**HMAC-SHA256 Signature:**

```http
POST https://merchant.com/webhooks/chargebacks
Content-Type: application/json
X-Webhook-Signature: abc123...
X-Webhook-Event-Type: chargeback.created
X-Webhook-Timestamp: 2025-10-29T12:00:00Z

{...payload...}
```

**Verification (Merchant Side):**

```go
func VerifyWebhookSignature(payload []byte, receivedSig string, secret string) bool {
    h := hmac.New(sha256.New, []byte(secret))
    h.Write(payload)
    expectedSig := hex.EncodeToString(h.Sum(nil))
    return hmac.Equal([]byte(expectedSig), []byte(receivedSig))
}
```

### Retry Logic

Failed deliveries are automatically retried:
- Attempt 1: Immediate
- Attempt 2: +5 minutes
- Attempt 3: +15 minutes (total 20min)
- Attempt 4: +25 minutes (total 45min)
- Attempt 5: +35 minutes (total 80min)

After 5 failures, delivery is marked as permanently failed.

**Managing Webhooks:**

```sql
-- Register webhook
INSERT INTO webhook_subscriptions (
    agent_id, event_type, webhook_url, secret, is_active
) VALUES (
    'merchant-123', 'chargeback.created',
    'https://merchant.com/webhooks', 'secret-key', true
);

-- View delivery history
SELECT created_at, event_type, status, http_status_code, attempts
FROM webhook_deliveries
WHERE subscription_id = 'uuid'
ORDER BY created_at DESC;
```

---

## 8. Database

### Database Schema

**Key Tables:**

```sql
-- Transactions
CREATE TABLE transactions (
    id UUID PRIMARY KEY,
    group_id UUID NOT NULL,
    agent_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(100),
    amount VARCHAR(255) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    status VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,
    payment_method_type VARCHAR(50) NOT NULL,
    token VARCHAR(255),
    response_code VARCHAR(10),
    response_message TEXT,
    idempotency_key VARCHAR(255) UNIQUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Subscriptions
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY,
    agent_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(100) NOT NULL,
    amount VARCHAR(255) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    frequency VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    payment_method_token VARCHAR(255) NOT NULL,
    next_billing_date DATE NOT NULL,
    max_retries INT DEFAULT 3,
    failure_option VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Chargebacks (read-only tracking)
CREATE TABLE chargebacks (
    id UUID PRIMARY KEY,
    group_id UUID,
    agent_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(100),
    case_number VARCHAR(255) UNIQUE NOT NULL,
    dispute_date TIMESTAMPTZ NOT NULL,
    chargeback_date TIMESTAMPTZ NOT NULL,
    chargeback_amount VARCHAR(255) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    reason_code VARCHAR(50) NOT NULL,
    reason_description TEXT,
    status VARCHAR(50) NOT NULL,
    raw_data JSONB,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Webhook Subscriptions
CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY,
    agent_id VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    webhook_url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Webhook Deliveries
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY,
    subscription_id UUID REFERENCES webhook_subscriptions(id),
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(50) NOT NULL,
    http_status_code INT,
    attempts INT DEFAULT 0,
    max_attempts INT DEFAULT 5,
    next_retry_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
```

### Database Migrations

**Run migrations:**

```bash
# Up (apply all)
make migrate-up

# Down (rollback one)
make migrate-down

# Force version
migrate -path internal/db/migrations -database "..." force 1
```

**Migration files:**
- `001_transactions.sql` - Transactions table
- `002_chargebacks.sql` - Chargebacks table
- `003_agent_credentials.sql` - Agent credentials
- `004_customer_payment_methods.sql` - Payment methods
- `005_soft_delete_cleanup.sql` - Soft delete support
- `007_webhook_subscriptions.sql` - Webhook system

### Database Queries

**Using sqlc for type-safe queries:**

```sql
-- name: CreateTransaction :one
INSERT INTO transactions (...)
VALUES (...)
RETURNING *;

-- name: GetTransactionByID :one
SELECT * FROM transactions WHERE id = $1;

-- name: ListChargebacks :many
SELECT * FROM chargebacks
WHERE agent_id = $1
  AND (COALESCE($2::varchar, '') = '' OR customer_id = $2)
  AND (COALESCE($3::uuid, '00000000-0000-0000-0000-000000000000'::uuid) = '00000000-0000-0000-0000-000000000000'::uuid OR group_id = $3)
ORDER BY dispute_date DESC
LIMIT $4 OFFSET $5;
```

---

## 9. Testing

### Unit Tests

**Run unit tests:**

```bash
# All unit tests
make test-unit

# Specific package
go test -v ./internal/adapters/north

# With coverage
go test -cover ./internal/adapters/north
```

**Test Structure:**

```go
func TestServerPostAdapter_Authorize(t *testing.T) {
    // Setup mocks
    mockHTTP := mocks.NewMockHTTPClient(...)
    mockLogger := mocks.NewMockLogger()

    adapter := epx.NewServerPostAdapter(url, mockHTTP, mockLogger)

    // Test
    result, err := adapter.Authorize(ctx, request)

    // Assertions
    assert.NoError(t, err)
    assert.Equal(t, "approved", result.Status)
}
```

### Integration Tests

**Run integration tests:**

```bash
# Start test database
make test-db-up

# Run integration tests
make test-integration

# Stop test database
make test-db-down
```

**What's tested:**
- Repository CRUD operations with real PostgreSQL
- Service business logic with database transactions
- Subscription billing workflows
- Idempotency key handling

### Test Coverage

Current coverage:
- **North Adapters**: 85.7%
- **HMAC Authentication**: 100%
- **Response Code Mapper**: 100%
- **Overall**: 85%+

---

## 10. Deployment

### Environment Variables

**Required:**

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=payment_user
DB_PASSWORD=payment_pass
DB_NAME=payment_service
DB_MAX_CONNS=25

# gRPC Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# HTTP Server (for cron endpoints)
HTTP_PORT=8081

# Metrics
SERVER_METRICS_PORT=9090

# North Gateway
GATEWAY_BASE_URL=https://secure.epxuap.com
GATEWAY_USERNAME=your-epi-id
GATEWAY_EPI_KEY=your-epi-key
NORTH_API_URL=https://api.north.com
NORTH_TIMEOUT=30

# Cron Jobs
CRON_SECRET=change-me-in-production

# Logging
LOG_LEVEL=info
LOG_DEVELOPMENT=false
```

### Production Setup

**1. Deploy to Kubernetes:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payment-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: payment-service
  template:
    metadata:
      labels:
        app: payment-service
    spec:
      containers:
      - name: payment-service
        image: payment-service:latest
        ports:
        - containerPort: 8080  # gRPC
        - containerPort: 9090  # Metrics
        env:
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: payment-secrets
              key: db-host
```

**2. Configure Cloud Scheduler:**

```bash
# Cron job for subscription billing
gcloud scheduler jobs create http subscription-billing \
  --schedule="0 */6 * * *" \
  --uri="https://your-app.com/cron/process-billing" \
  --http-method=POST \
  --headers="X-Cron-Secret=your-secret"

# Cron job for dispute sync
gcloud scheduler jobs create http dispute-sync \
  --schedule="0 */4 * * *" \
  --uri="https://your-app.com/cron/sync-disputes" \
  --http-method=POST \
  --headers="X-Cron-Secret=your-secret"
```

### Deployment Security

**PCI Compliance:**
- ✅ Backend never handles raw card data (tokenization)
- ✅ TLS 1.3 for all communications
- ✅ HMAC authentication for all API calls
- ✅ Secrets stored in secure vault
- ✅ No sensitive data in logs

**Best Practices:**
- Use managed PostgreSQL (AWS RDS, Cloud SQL)
- Store secrets in secrets manager
- Enable connection pooling (25 max connections)
- Set up monitoring and alerting
- Regular security audits

---

## 11. API Reference

### Payment APIs

```protobuf
service PaymentService {
  // Authorize payment (auth-only, no capture)
  rpc Authorize(AuthorizeRequest) returns (Transaction);

  // Capture previously authorized payment
  rpc Capture(CaptureRequest) returns (Transaction);

  // Void transaction (before settlement)
  rpc Void(VoidRequest) returns (Transaction);

  // Refund transaction
  rpc Refund(RefundRequest) returns (Transaction);

  // Sale (authorize + capture in one step)
  rpc Sale(SaleRequest) returns (Transaction);

  // Get transaction by ID
  rpc GetTransaction(GetTransactionRequest) returns (Transaction);

  // List transactions with filters
  rpc ListTransactions(ListTransactionsRequest) returns (ListTransactionsResponse);
}
```

### Subscription APIs

```protobuf
service SubscriptionService {
  // Create new subscription
  rpc CreateSubscription(CreateSubscriptionRequest) returns (Subscription);

  // Update subscription (amount, frequency)
  rpc UpdateSubscription(UpdateSubscriptionRequest) returns (Subscription);

  // Cancel subscription
  rpc CancelSubscription(CancelSubscriptionRequest) returns (Subscription);

  // Get subscription by ID
  rpc GetSubscription(GetSubscriptionRequest) returns (Subscription);

  // List subscriptions
  rpc ListSubscriptions(ListSubscriptionsRequest) returns (ListSubscriptionsResponse);
}
```

### Chargeback APIs

```protobuf
service ChargebackService {
  // Get chargeback by ID
  rpc GetChargeback(GetChargebackRequest) returns (Chargeback);

  // List chargebacks with filters
  rpc ListChargebacks(ListChargebacksRequest) returns (ListChargebacksResponse);

  // Get chargeback statistics
  rpc GetChargebackStats(GetChargebackStatsRequest) returns (ChargebackStatsResponse);
}
```

---

## 12. Troubleshooting

### Common Issues

**Database Connection Errors:**

```bash
# Check database is running
docker compose -f docker-compose.test.yml ps

# Check logs
docker compose -f docker-compose.test.yml logs postgres-test

# Restart database
docker compose -f docker-compose.test.yml restart postgres-test
```

**Port Already in Use:**

```bash
# Find process using port
lsof -i :8080

# Kill process
kill -9 <PID>
```

**gRPC Connection Errors:**

```bash
# Check server is running
ps aux | grep payment-server

# Check port is open
nc -zv localhost 8080

# Enable debug logs
export GRPC_GO_LOG_VERBOSITY_LEVEL=99
export GRPC_GO_LOG_SEVERITY_LEVEL=info
```

### Debugging

**Enable verbose logging:**

```bash
export LOG_LEVEL=debug
export LOG_DEVELOPMENT=true
```

**Check health:**

```bash
curl http://localhost:9090/health
```

**View metrics:**

```bash
curl http://localhost:9090/metrics
```

**Database queries:**

```sql
-- Recent transactions
SELECT id, agent_id, amount, status, created_at
FROM transactions
ORDER BY created_at DESC
LIMIT 10;

-- Chargebacks by status
SELECT status, COUNT(*)
FROM chargebacks
WHERE agent_id = 'merchant-123'
GROUP BY status;

-- Webhook delivery failures
SELECT event_type, error_message, attempts
FROM webhook_deliveries
WHERE status = 'failed'
ORDER BY created_at DESC;
```

---

## Support & Contact

**Documentation Issues**: Open issue on GitHub
**Integration Support**: See specific integration guides
**North Gateway**: Contact North support

**Project Repository**: [https://github.com/kevin07696/payment-service](https://github.com/kevin07696/payment-service)

---

**Last Updated**: 2025-10-29
**Version**: v0.2.0-alpha
