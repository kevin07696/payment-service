# Payment Microservice - System Design

## Executive Summary

A production-ready payment microservice built with **Go** and **gRPC**, integrating with North Payment Gateway (EPX) to support:
- Credit card payments (one-time, auth/capture)
- Recurring billing subscriptions
- ACH bank transfers
- Invoice payments
- PCI-compliant tokenization
- Comprehensive audit logging and observability

## Architecture Pattern: Hexagonal (Ports & Adapters)

```
┌─────────────────────────────────────────────────────────────────┐
│                     gRPC API Layer                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Payment    │  │ Subscription │  │     ACH      │         │
│  │   Service    │  │   Service    │  │   Service    │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────────┐
│                   Application Layer                             │
│  ┌──────────────────────────────────────────────────────┐      │
│  │  Business Logic (Use Cases)                          │      │
│  │  - ProcessPayment  - ManageSubscription             │      │
│  │  - IssueRefund     - HandleWebhook                  │      │
│  └──────────────────────────────────────────────────────┘      │
│                                                                  │
│  ┌──────────────────────────────────────────────────────┐      │
│  │  Middleware                                          │      │
│  │  - Idempotency  - Rate Limiting  - Circuit Breaker  │      │
│  └──────────────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────────┐
│                     Domain Layer (Core)                         │
│  ┌──────────────────────────────────────────────────────┐      │
│  │  Domain Models                                       │      │
│  │  - Payment  - Transaction  - Subscription           │      │
│  │  - Invoice  - Customer     - PaymentMethod          │      │
│  └──────────────────────────────────────────────────────┘      │
│                                                                  │
│  ┌──────────────────────────────────────────────────────┐      │
│  │  Port Interfaces (Contracts)                         │      │
│  │  - PaymentGateway  - TokenVault  - Repository       │      │
│  │  - AuditLogger     - EventPublisher                 │      │
│  └──────────────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────────┐
│                   Infrastructure Layer                          │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐   │
│  │ North Adapters │  │   PostgreSQL   │  │  Observability │   │
│  │ - Custom Pay   │  │  Repositories  │  │ - OpenTelemetry│   │
│  │ - Recurring    │  │  - Transactions│  │ - Prometheus   │   │
│  │ - ACH          │  │  - Subs        │  │ - Jaeger       │   │
│  │ - Browser Post │  │  - Invoices    │  │                │   │
│  └────────────────┘  └────────────────┘  └────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Technology Stack

### Core
- **Language**: Go 1.21+
- **API Protocol**: gRPC with Protocol Buffers
- **Database**: PostgreSQL 15+ (ACID compliance, JSONB support)
- **Migration Tool**: golang-migrate

### Libraries
- **gRPC**: `google.golang.org/grpc`
- **Protobufs**: `google.golang.org/protobuf`
- **Database**: `github.com/lib/pq` or `github.com/jackc/pgx/v5`
- **HTTP Client**: `net/http` with custom retry logic
- **Decimal Math**: `github.com/shopspring/decimal` (PCI requirement: no float64 for money)
- **Logging**: `go.uber.org/zap` (structured logging)
- **Validation**: `github.com/go-playground/validator/v10`
- **Testing**: `github.com/stretchr/testify`

### Observability
- **Tracing**: OpenTelemetry + Jaeger
- **Metrics**: Prometheus
- **Health Checks**: gRPC health checking protocol

### Infrastructure
- **Containerization**: Docker
- **Orchestration**: Kubernetes
- **Config Management**: Environment variables + Viper
- **Secrets**: Kubernetes Secrets / HashiCorp Vault

## Directory Structure

```
payment-microservice/
├── api/
│   └── proto/                    # Protocol buffer definitions
│       ├── payment.proto         # Payment service (one-time, auth/capture)
│       ├── subscription.proto    # Recurring billing service
│       ├── ach.proto            # ACH/bank transfer service
│       ├── invoice.proto        # Invoice service
│       └── common.proto         # Shared types (Money, Address, etc.)
│
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
│
├── internal/
│   ├── domain/                  # Core business logic (no external deps)
│   │   ├── models/              # Domain entities
│   │   │   ├── payment.go
│   │   │   ├── transaction.go
│   │   │   ├── subscription.go
│   │   │   ├── customer.go
│   │   │   └── payment_method.go
│   │   └── ports/               # Interface definitions (contracts)
│   │       ├── payment_gateway.go
│   │       ├── repository.go
│   │       ├── token_vault.go
│   │       ├── audit_logger.go
│   │       └── event_publisher.go
│   │
│   ├── application/             # Use cases / business workflows
│   │   ├── services/
│   │   │   ├── payment_service.go
│   │   │   ├── subscription_service.go
│   │   │   ├── ach_service.go
│   │   │   └── webhook_service.go
│   │   └── middleware/
│   │       ├── idempotency.go
│   │       ├── rate_limiter.go
│   │       ├── circuit_breaker.go
│   │       └── auth.go
│   │
│   ├── adapters/                # External system implementations
│   │   ├── north/               # North Payment Gateway adapters
│   │   │   ├── custom_pay_adapter.go
│   │   │   ├── recurring_billing_adapter.go
│   │   │   ├── ach_adapter.go
│   │   │   ├── browser_post_adapter.go
│   │   │   ├── response_codes.go
│   │   │   └── hmac_auth.go
│   │   └── database/            # PostgreSQL implementations
│   │       ├── transaction_repository.go
│   │       ├── subscription_repository.go
│   │       ├── invoice_repository.go
│   │       └── idempotency_repository.go
│   │
│   └── infrastructure/          # Cross-cutting concerns
│       ├── config/
│       │   └── config.go        # Configuration management
│       ├── logging/
│       │   └── logger.go        # Structured logging
│       └── metrics/
│           └── prometheus.go    # Metrics collection
│
├── pkg/                         # Public libraries (reusable)
│   ├── security/
│   │   ├── encryption.go        # AES-256 encryption
│   │   └── token_manager.go     # BRIC token management
│   ├── errors/
│   │   └── errors.go           # Custom error types
│   └── validation/
│       └── validators.go        # Input validation
│
├── migrations/                  # Database migrations
│   ├── 001_create_transactions.up.sql
│   ├── 001_create_transactions.down.sql
│   ├── 002_create_subscriptions.up.sql
│   └── ...
│
├── test/
│   ├── integration/             # Integration tests
│   │   ├── payment_flow_test.go
│   │   └── subscription_flow_test.go
│   └── mocks/                   # Mock implementations
│       ├── mock_gateway.go
│       └── mock_repository.go
│
├── docs/                        # Documentation
│   ├── api/                     # API documentation
│   └── architecture/            # Architecture diagrams
│
├── scripts/                     # Utility scripts
│   ├── generate-proto.sh        # Protobuf code generation
│   └── run-migrations.sh        # Database migration script
│
├── deployments/                 # Deployment configs
│   ├── kubernetes/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   └── configmap.yaml
│   └── docker/
│       └── Dockerfile
│
├── go.mod
├── go.sum
├── Makefile
├── .env.example
└── README.md
```

## Component Design

### 1. gRPC API Layer

**Payment Service** (`api/proto/payment.proto`)
```protobuf
service PaymentService {
  rpc AuthorizePayment(AuthorizeRequest) returns (PaymentResponse);
  rpc CapturePayment(CaptureRequest) returns (PaymentResponse);
  rpc VoidPayment(VoidRequest) returns (PaymentResponse);
  rpc RefundPayment(RefundRequest) returns (PaymentResponse);
  rpc GetPaymentStatus(GetPaymentRequest) returns (PaymentResponse);
}
```

**Subscription Service** (`api/proto/subscription.proto`)
```protobuf
service SubscriptionService {
  rpc CreateSubscription(CreateSubscriptionRequest) returns (SubscriptionResponse);
  rpc UpdateSubscription(UpdateSubscriptionRequest) returns (SubscriptionResponse);
  rpc CancelSubscription(CancelSubscriptionRequest) returns (SubscriptionResponse);
  rpc PauseSubscription(PauseSubscriptionRequest) returns (SubscriptionResponse);
  rpc ResumeSubscription(ResumeSubscriptionRequest) returns (SubscriptionResponse);
  rpc GetSubscription(GetSubscriptionRequest) returns (SubscriptionResponse);
  rpc ListSubscriptions(ListSubscriptionsRequest) returns (ListSubscriptionsResponse);
}
```

**ACH Service** (`api/proto/ach.proto`)
```protobuf
service ACHService {
  rpc ProcessACHPayment(ACHPaymentRequest) returns (PaymentResponse);
  rpc RefundACHPayment(RefundRequest) returns (PaymentResponse);
  rpc VerifyBankAccount(VerifyAccountRequest) returns (VerificationResponse);
}
```

### 2. Domain Models

**Transaction** (internal/domain/models/transaction.go)
```go
type Transaction struct {
    ID                    string
    MerchantID            string
    CustomerID            string
    Amount                decimal.Decimal
    Currency              string
    Status                TransactionStatus
    Type                  TransactionType
    PaymentMethod         PaymentMethod
    GatewayTransactionID  string  // BRIC token from North
    GatewayResponse       GatewayResponse
    Metadata              map[string]string
    IdempotencyKey        string
    CreatedAt             time.Time
    UpdatedAt             time.Time
}

type TransactionStatus string
const (
    StatusPending    TransactionStatus = "pending"
    StatusAuthorized TransactionStatus = "authorized"
    StatusCaptured   TransactionStatus = "captured"
    StatusVoided     TransactionStatus = "voided"
    StatusRefunded   TransactionStatus = "refunded"
    StatusFailed     TransactionStatus = "failed"
)

type TransactionType string
const (
    TypeAuthorization TransactionType = "authorization"
    TypeCapture       TransactionType = "capture"
    TypeSale          TransactionType = "sale"
    TypeRefund        TransactionType = "refund"
    TypeVoid          TransactionType = "void"
)
```

**Subscription** (internal/domain/models/subscription.go)
```go
type Subscription struct {
    ID                    string
    MerchantID            string
    CustomerID            string
    Amount                decimal.Decimal
    Currency              string
    Frequency             BillingFrequency
    Status                SubscriptionStatus
    PaymentMethodToken    string  // BRIC token
    NextBillingDate       time.Time
    FailureRetryCount     int
    FailureOption         FailureOption
    GatewaySubscriptionID string
    Metadata              map[string]string
    CreatedAt             time.Time
    UpdatedAt             time.Time
    CancelledAt           *time.Time
}

type BillingFrequency string
const (
    FrequencyWeekly   BillingFrequency = "weekly"
    FrequencyBiWeekly BillingFrequency = "biweekly"
    FrequencyMonthly  BillingFrequency = "monthly"
    FrequencyYearly   BillingFrequency = "yearly"
)

type SubscriptionStatus string
const (
    SubStatusActive    SubscriptionStatus = "active"
    SubStatusPaused    SubscriptionStatus = "paused"
    SubStatusCancelled SubscriptionStatus = "cancelled"
    SubStatusExpired   SubscriptionStatus = "expired"
)

type FailureOption string
const (
    FailureForward FailureOption = "forward"  // Move billing date forward
    FailureSkip    FailureOption = "skip"     // Skip this billing cycle
    FailurePause   FailureOption = "pause"    // Pause subscription
)
```

### 3. Port Interfaces (Contracts)

**PaymentGateway** (internal/domain/ports/payment_gateway.go)
```go
type CreditCardGateway interface {
    Authorize(ctx context.Context, req *AuthorizeRequest) (*PaymentResult, error)
    Capture(ctx context.Context, transactionID string, amount decimal.Decimal) (*PaymentResult, error)
    Void(ctx context.Context, transactionID string) (*PaymentResult, error)
    Refund(ctx context.Context, transactionID string, amount decimal.Decimal) (*PaymentResult, error)
    VerifyAccount(ctx context.Context, req *VerifyAccountRequest) (*VerificationResult, error)
}

type RecurringBillingGateway interface {
    CreateSubscription(ctx context.Context, req *SubscriptionRequest) (*SubscriptionResult, error)
    UpdateSubscription(ctx context.Context, subscriptionID string, req *UpdateSubscriptionRequest) (*SubscriptionResult, error)
    CancelSubscription(ctx context.Context, subscriptionID string, immediate bool) (*SubscriptionResult, error)
    PauseSubscription(ctx context.Context, subscriptionID string) (*SubscriptionResult, error)
    ResumeSubscription(ctx context.Context, subscriptionID string) (*SubscriptionResult, error)
    GetSubscription(ctx context.Context, subscriptionID string) (*SubscriptionResult, error)
    ListSubscriptions(ctx context.Context, customerID string) ([]*SubscriptionResult, error)
}

type ACHGateway interface {
    ProcessPayment(ctx context.Context, req *ACHRequest) (*PaymentResult, error)
    RefundPayment(ctx context.Context, transactionID string, amount decimal.Decimal) (*PaymentResult, error)
    VerifyBankAccount(ctx context.Context, req *BankAccountVerificationRequest) (*VerificationResult, error)
}
```

**Repository** (internal/domain/ports/repository.go)
```go
type TransactionRepository interface {
    Create(ctx context.Context, tx *Transaction) error
    GetByID(ctx context.Context, id string) (*Transaction, error)
    GetByIdempotencyKey(ctx context.Context, key string) (*Transaction, error)
    Update(ctx context.Context, tx *Transaction) error
    List(ctx context.Context, filters *TransactionFilters) ([]*Transaction, error)
}

type SubscriptionRepository interface {
    Create(ctx context.Context, sub *Subscription) error
    GetByID(ctx context.Context, id string) (*Subscription, error)
    Update(ctx context.Context, sub *Subscription) error
    ListByCustomer(ctx context.Context, customerID string) ([]*Subscription, error)
    ListDueForBilling(ctx context.Context, before time.Time) ([]*Subscription, error)
}
```

**AuditLogger** (internal/domain/ports/audit_logger.go)
```go
type AuditLogger interface {
    LogPaymentAttempt(ctx context.Context, event *PaymentAttemptEvent) error
    LogPaymentSuccess(ctx context.Context, event *PaymentSuccessEvent) error
    LogPaymentFailure(ctx context.Context, event *PaymentFailureEvent) error
    LogSubscriptionChange(ctx context.Context, event *SubscriptionChangeEvent) error
}
```

### 4. North Payment Gateway Adapters

**Custom Pay Adapter** (internal/adapters/north/custom_pay_adapter.go)
- **Purpose**: One-time credit card payments, auth/capture flows
- **Base URL**: `https://api.epxuap.com`
- **Authentication**: HMAC-SHA256 (EPI-Id + EPI-Signature headers)
- **Format**: JSON requests/responses
- **Operations**:
  - POST /sale (authorize + capture or authorize only)
  - PUT /sale/{BRIC}/capture (capture authorized transaction)
  - POST /sale/{BRIC} (sale with token)
  - POST /refund/{BRIC} (refund)
  - PUT /void/{BRIC} (void before settlement)
  - POST /reverse/{BRIC} (auth reversal)

**Recurring Billing Adapter** (internal/adapters/north/recurring_billing_adapter.go)
- **Purpose**: Subscription management
- **Base URL**: `https://billing.epxuap.com`
- **Authentication**: HMAC-SHA256
- **Format**: JSON
- **Operations**:
  - POST /subscription (create)
  - PUT /subscription/{ID} (update)
  - POST /subscription/cancel (cancel)
  - POST /subscription/pause (pause)
  - POST /subscription/resume (resume)
  - GET /subscription/list (list by customer)

**ACH Adapter** (internal/adapters/north/ach_adapter.go)
- **Purpose**: Bank transfers (checking/savings)
- **Base URL**: `https://secure.epxuap.com`
- **Authentication**: 4-part key in request body
- **Format**: Form-encoded requests, XML responses
- **Operations**:
  - TRAN_TYPE=CKC2 (checking account debit)
  - TRAN_TYPE=CKS2 (savings account debit)
  - TRAN_TYPE=CKC3 (checking refund)
  - TRAN_TYPE=CKS3 (savings refund)
- **Features**:
  - Account validation (routing/account number check)
  - SEC codes (PPD, WEB, CCD, etc.)
  - 60-day dispute window handling

**Browser Post Adapter** (internal/adapters/north/browser_post_adapter.go)
- **Purpose**: Backend operations for tokenized payments
- **Base URL**: `https://secure.epxuap.com`
- **Authentication**: HMAC-SHA256
- **Format**: Form-encoded requests, XML responses
- **Operations**:
  - POST /refund (refund by transaction ID)
  - POST /refund/{bric} (refund by BRIC)
  - POST /reverse/{bric} (auth reversal)
  - PUT /void/{bric} (void)
  - POST /sale/{bric} (tip adjust)

### 5. Database Schema

**transactions** table
```sql
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(100),
    amount NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status VARCHAR(20) NOT NULL,
    type VARCHAR(20) NOT NULL,
    payment_method_type VARCHAR(20) NOT NULL,
    payment_method_token VARCHAR(255),  -- BRIC token
    gateway_transaction_id VARCHAR(255),
    gateway_response_code VARCHAR(10),
    gateway_response_message TEXT,
    idempotency_key VARCHAR(255) UNIQUE,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    INDEX idx_merchant_id (merchant_id),
    INDEX idx_customer_id (customer_id),
    INDEX idx_idempotency_key (idempotency_key),
    INDEX idx_gateway_transaction_id (gateway_transaction_id),
    INDEX idx_created_at (created_at DESC)
);
```

**subscriptions** table
```sql
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(100) NOT NULL,
    amount NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    frequency VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    payment_method_token VARCHAR(255) NOT NULL,
    next_billing_date DATE NOT NULL,
    failure_retry_count INT NOT NULL DEFAULT 0,
    failure_option VARCHAR(20) NOT NULL,
    gateway_subscription_id VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    cancelled_at TIMESTAMP,

    INDEX idx_merchant_customer (merchant_id, customer_id),
    INDEX idx_next_billing_date (next_billing_date),
    INDEX idx_status (status),
    INDEX idx_gateway_subscription_id (gateway_subscription_id)
);
```

**audit_logs** table
```sql
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    merchant_id VARCHAR(100) NOT NULL,
    user_id VARCHAR(100),
    action VARCHAR(50) NOT NULL,
    before_state JSONB,
    after_state JSONB,
    metadata JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    INDEX idx_entity (entity_type, entity_id),
    INDEX idx_merchant_id (merchant_id),
    INDEX idx_created_at (created_at DESC)
);
```

**idempotency_keys** table
```sql
CREATE TABLE idempotency_keys (
    key VARCHAR(255) PRIMARY KEY,
    request_hash VARCHAR(64) NOT NULL,
    response JSONB NOT NULL,
    status_code INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,

    INDEX idx_expires_at (expires_at)
);
```

## Data Flow

### Flow 1: One-Time Credit Card Payment (with tokenization)

```
┌─────────┐    1. POST /sale     ┌─────────────┐
│ Client  │───(card data)────────>│   Browser   │
│  App    │                       │    Post     │
└─────────┘                       │   (North)   │
     │                            └─────────────┘
     │                                   │
     │    2. Return BRIC token           │
     │<──────────────────────────────────┘
     │
     │    3. gRPC AuthorizePayment
     │       (BRIC token)
     v
┌──────────────────────┐
│  Payment Service     │
│  (gRPC Handler)      │
└──────────────────────┘
     │
     │    4. Check idempotency
     v
┌──────────────────────┐
│ Idempotency Checker  │
└──────────────────────┘
     │
     │    5. Process payment
     v
┌──────────────────────┐
│  Payment Use Case    │
│ (Application Layer)  │
└──────────────────────┘
     │
     │    6. Call gateway
     v
┌──────────────────────┐
│ Custom Pay Adapter   │
│  POST /sale/{BRIC}   │
└──────────────────────┘
     │
     │    7. HMAC auth
     v
┌──────────────────────┐
│   North Gateway      │
│    (EPX API)         │
└──────────────────────┘
     │
     │    8. Response (code 00)
     v
┌──────────────────────┐
│  Save Transaction    │
│   (PostgreSQL)       │
└──────────────────────┘
     │
     │    9. Audit log
     v
┌──────────────────────┐
│   Audit Logger       │
└──────────────────────┘
     │
     │    10. Return result
     v
┌──────────────────────┐
│  Client Response     │
│  (gRPC)              │
└──────────────────────┘
```

### Flow 2: Recurring Subscription Creation

```
Client App
    │
    │  1. gRPC CreateSubscription
    │     (BRIC token, schedule)
    v
Subscription Service
    │
    │  2. Validate input
    v
Subscription Use Case
    │
    │  3. Call gateway
    v
Recurring Billing Adapter
    │
    │  4. POST /subscription
    │     (HMAC auth)
    v
North Gateway
    │
    │  5. Create subscription
    │     Return subscription ID
    v
Save Subscription (PostgreSQL)
    │
    │  6. Store with next_billing_date
    v
Audit Log
    │
    │  7. Log subscription creation
    v
Client Response
```

### Flow 3: ACH Payment Processing

```
Client App
    │
    │  1. gRPC ProcessACHPayment
    │     (routing, account, amount)
    v
ACH Service
    │
    │  2. Validate bank account format
    v
ACH Use Case
    │
    │  3. Call gateway
    v
ACH Adapter
    │
    │  4. POST (form-encoded)
    │     TRAN_TYPE=CKC2
    v
North Gateway
    │
    │  5. Validate account (real-time)
    │     Process ACH
    v
Parse XML Response
    │
    │  6. Extract AUTH_RESP code
    v
Save Transaction
    │
    │  7. Store with 60-day dispute window
    v
Client Response
```

## Security Architecture

### 1. PCI DSS 4.0 Compliance

**Tokenization Strategy**:
- Frontend uses Browser Post to tokenize cards → returns BRIC token
- Backend **NEVER** handles raw card numbers (PCI-reduced scope)
- All payment methods stored as BRIC tokens only
- Token vault managed by North (offload PCI responsibility)

**Data Protection**:
- TLS 1.3 for all communications
- AES-256-GCM for sensitive data at rest
- Database column-level encryption for PII
- Secrets stored in Kubernetes Secrets / Vault

**Access Control**:
- API authentication via OAuth 2.0 / JWT
- Role-based access control (RBAC)
- Principle of least privilege

### 2. HMAC Authentication

All North API calls use HMAC-SHA256:
```go
func calculateSignature(epiKey, endpoint string, payload []byte) string {
    concat := []byte(endpoint)
    concat = append(concat, payload...)
    h := hmac.New(sha256.New, []byte(epiKey))
    h.Write(concat)
    return hex.EncodeToString(h.Sum(nil))
}

// Headers
EPI-Id: CUST_NBR-MERCH_NBR-DBA_NBR-TERMINAL_NBR
EPI-Signature: <hmac_signature>
```

### 3. Idempotency

Prevent duplicate charges:
```go
// Client sends idempotency key in request
IdempotencyKey: "uuid-or-client-generated-key"

// Server checks if key exists
if cached := idempotencyStore.Get(key); cached != nil {
    return cached.Response  // Return original response
}

// Process request
result := processPayment(req)

// Cache result (24-hour TTL)
idempotencyStore.Set(key, result, 24*time.Hour)
```

### 4. Circuit Breaker

Protect against cascade failures:
```go
type CircuitBreaker struct {
    maxFailures int
    timeout     time.Duration
    state       State  // Closed, Open, HalfOpen
}

// Wrap all gateway calls
func (cb *CircuitBreaker) Execute(fn func() error) error {
    if cb.state == Open {
        return ErrCircuitOpen
    }

    err := fn()
    if err != nil {
        cb.recordFailure()
        if cb.failures >= cb.maxFailures {
            cb.state = Open
            time.AfterFunc(cb.timeout, cb.halfOpen)
        }
    } else {
        cb.reset()
    }
    return err
}
```

## Error Handling

### Response Code Processing

```go
type PaymentError struct {
    Code           string
    Message        string
    GatewayMessage string
    IsRetriable    bool
    Category       ErrorCategory
}

type ErrorCategory string
const (
    CategoryApproved          ErrorCategory = "approved"
    CategoryDeclined          ErrorCategory = "declined"
    CategoryInsufficientFunds ErrorCategory = "insufficient_funds"
    CategoryInvalidCard       ErrorCategory = "invalid_card"
    CategoryFraud             ErrorCategory = "fraud"
    CategorySystemError       ErrorCategory = "system_error"
)

// Map response codes to categories
var responseCodeMap = map[string]PaymentError{
    "00": {IsApproved: true, Category: CategoryApproved},
    "51": {IsRetriable: true, Category: CategoryInsufficientFunds},
    "82": {IsRetriable: true, Category: CategoryInvalidCard},
    "96": {IsRetriable: true, Category: CategorySystemError},
    // ... (40+ codes)
}
```

### Error Propagation

```
Gateway Error
    │
    │  Parse response code
    v
PaymentError
    │
    │  Map to gRPC status
    v
gRPC Error
    │
    ├─ codes.InvalidArgument (4xx errors)
    ├─ codes.Internal (5xx errors)
    ├─ codes.Unavailable (network errors)
    └─ codes.FailedPrecondition (business rule violations)
```

## Observability

### 1. Distributed Tracing (OpenTelemetry + Jaeger)

```go
// Trace entire payment flow
ctx, span := tracer.Start(ctx, "payment.authorize")
defer span.End()

span.SetAttributes(
    attribute.String("merchant.id", merchantID),
    attribute.Float64("amount", amount.InexactFloat64()),
    attribute.String("gateway", "north.custom_pay"),
)

// Child spans for each operation
gatewaySpan := tracer.Start(ctx, "gateway.request")
defer gatewaySpan.End()
```

### 2. Metrics (Prometheus)

```go
// Business metrics
var (
    paymentTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "payments_total",
            Help: "Total number of payment attempts",
        },
        []string{"method", "status", "gateway"},
    )

    paymentAmount = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "payment_amount_usd",
            Help: "Payment amounts in USD",
            Buckets: []float64{1, 10, 50, 100, 500, 1000, 5000},
        },
        []string{"method"},
    )

    gatewayLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "gateway_request_duration_seconds",
            Help: "Gateway request latency",
            Buckets: prometheus.DefBuckets,
        },
        []string{"gateway", "operation"},
    )
)
```

### 3. Structured Logging (Zap)

```go
logger.Info("payment authorized",
    zap.String("transaction_id", txID),
    zap.String("merchant_id", merchantID),
    zap.Float64("amount", amount.InexactFloat64()),
    zap.String("gateway_response", responseCode),
    zap.Duration("latency", duration),
)
```

## Testing Strategy

### 1. Unit Tests
- Domain logic (models, validation)
- Use case business rules
- Port interface compliance
- Mock all external dependencies

### 2. Integration Tests
- Database operations (with testcontainers)
- North gateway calls (sandbox environment)
- Response code triggers (amount-based testing)

### 3. Certification Test Suite
```go
func TestNorthCertification(t *testing.T) {
    scenarios := []struct {
        name         string
        amount       decimal.Decimal
        expectedCode string
    }{
        {"Approval", decimal.NewFromFloat(1.00), "00"},
        {"Insufficient Funds", decimal.NewFromFloat(1.20), "51"},
        {"CVV Error", decimal.NewFromFloat(1.40), "82"},
        // 40+ scenarios
    }

    for _, tc := range scenarios {
        t.Run(tc.name, func(t *testing.T) {
            // Test with North sandbox
        })
    }
}
```

## Configuration Management

```yaml
# config/config.yaml
server:
  port: 50051
  grpc:
    max_connection_age: 30m
    keepalive_time: 10s

database:
  host: ${DB_HOST}
  port: ${DB_PORT}
  name: ${DB_NAME}
  user: ${DB_USER}
  password: ${DB_PASSWORD}
  ssl_mode: require
  max_connections: 25
  connection_timeout: 30s

north:
  custom_pay:
    epi_id: ${NORTH_CUSTOM_PAY_EPI_ID}
    epi_key: ${NORTH_CUSTOM_PAY_EPI_KEY}
    base_url: https://api.epxuap.com
    timeout: 30s

  recurring_billing:
    epi_id: ${NORTH_RECURRING_EPI_ID}
    epi_key: ${NORTH_RECURRING_EPI_KEY}
    base_url: https://billing.epxuap.com
    timeout: 30s

  ach:
    epi_id: ${NORTH_ACH_EPI_ID}
    base_url: https://secure.epxuap.com
    timeout: 45s

security:
  encryption_key: ${ENCRYPTION_KEY}
  jwt_secret: ${JWT_SECRET}

observability:
  jaeger_endpoint: ${JAEGER_ENDPOINT}
  prometheus_port: 9090
  log_level: info
```

## Deployment Architecture

```
                      ┌─────────────────┐
                      │   Load Balancer │
                      │   (Kubernetes)  │
                      └─────────────────┘
                              │
                ┌─────────────┴─────────────┐
                │                           │
        ┌───────────────┐          ┌───────────────┐
        │  Payment Pod  │          │  Payment Pod  │
        │  (Replica 1)  │          │  (Replica 2)  │
        └───────────────┘          └───────────────┘
                │                           │
                └─────────────┬─────────────┘
                              │
                    ┌─────────────────┐
                    │   PostgreSQL    │
                    │   (StatefulSet) │
                    └─────────────────┘
```

### Kubernetes Resources

- **Deployment**: 3 replicas for high availability
- **Service**: ClusterIP for internal gRPC communication
- **Ingress**: gRPC load balancing
- **ConfigMap**: Non-sensitive configuration
- **Secret**: Credentials and encryption keys
- **PersistentVolumeClaim**: PostgreSQL storage

## Performance Targets

- **Latency**:
  - p50 < 100ms
  - p95 < 500ms
  - p99 < 1000ms
- **Throughput**: 1000+ transactions/second
- **Availability**: 99.9% uptime (43 minutes downtime/month)
- **Data Durability**: PostgreSQL ACID guarantees
- **Fault Tolerance**: Circuit breaker prevents cascade failures

## Development Workflow

1. **Proto Definition**: Define gRPC service contracts
2. **Code Generation**: `make generate-proto`
3. **Port Interfaces**: Define domain contracts
4. **Business Logic**: Implement use cases
5. **Adapters**: Implement North gateway adapters
6. **Tests**: Write unit and integration tests
7. **Migrations**: Create database schemas
8. **Deploy**: Docker + Kubernetes

## Open Questions / TODOs

1. **Browser Post Frontend**: Need JavaScript SDK documentation for frontend tokenization
2. **Invoice API**: Need API documentation (URL returned 403)
3. **Webhooks**: Need webhook signature verification specification
4. **3D Secure**: Clarify if North supports 3DS and integration method
5. **Multi-tenancy**: Decide on single vs multi-tenant architecture
6. **Rate Limiting**: Define rate limits per merchant
7. **Disaster Recovery**: Define backup and restore procedures

## Next Steps

1. ✅ Create directory structure
2. ✅ Define gRPC proto files
3. ✅ Generate Go code from protos
4. ✅ Implement domain models and ports
5. ✅ Implement North adapters (Custom Pay, Recurring, ACH)
6. ✅ Create database schema and migrations
7. ✅ Implement use case business logic
8. ✅ Set up observability (tracing, metrics, logging)
9. ✅ Write comprehensive tests
10. ✅ Create Docker and Kubernetes configs
11. ✅ Run quality assurance (go vet, golangci-lint, build)
