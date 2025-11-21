# Module Integration Guide

**Target Audience:** Go developers integrating payment service as a library/module (not as a separate microservice)
**Topic:** Using the payment service as an embedded Go module in your application
**Goal:** Import and use payment service logic directly in your Go application without running a separate server

---

## Overview

The payment service can be integrated in two ways:

### Option 1: Microservice Architecture (Recommended)
- Payment service runs as a **separate server** (port 8081)
- Your app makes **HTTP/ConnectRPC calls** to the payment service
- **Pros:** Service isolation, independent scaling, language-agnostic clients
- **Cons:** Network latency, additional operational complexity
- **See:** [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md)

### Option 2: Module/Library Integration (This Guide)
- Payment service imported as a **Go module** into your application
- Your app uses payment **services directly** (no HTTP calls)
- **Pros:** No network latency, simpler deployment, single binary
- **Cons:** Go-only, tight coupling, shared resources

**When to use module integration:**
- ✅ Your application is written in Go
- ✅ You want to minimize network overhead
- ✅ You prefer a monolithic architecture
- ✅ You don't need independent scaling of payment logic
- ✅ You're building a small-to-medium application

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Database Setup](#database-setup)
4. [Configuration](#configuration)
5. [Initializing Services](#initializing-services)
6. [Using Payment Services](#using-payment-services)
7. [Complete Example](#complete-example)
8. [Best Practices](#best-practices)
9. [Migration to Microservice](#migration-to-microservice)

---

## Prerequisites

**Required:**
- Go 1.21+ installed
- PostgreSQL 15+ running
- EPX merchant account and credentials
- Basic understanding of Go modules and dependency injection

**Your application must:**
- Be written in Go
- Use PostgreSQL (payment service uses pgx/v5)
- Support dependency injection pattern

---

## Installation

### Step 1: Add Module Dependency

```bash
# Add payment service as a dependency
go get github.com/kevin07696/payment-service@latest

# Download dependencies
go mod tidy
```

### Step 2: Verify Import

```go
import (
    "github.com/kevin07696/payment-service/internal/services/payment"
    "github.com/kevin07696/payment-service/internal/services/payment_method"
    "github.com/kevin07696/payment-service/internal/services/subscription"
    "github.com/kevin07696/payment-service/internal/adapters/epx"
    "github.com/kevin07696/payment-service/internal/db/sqlc"
)
```

---

## Database Setup

The payment service requires its own database tables. Run migrations in your PostgreSQL database:

### Option A: Use Payment Service Migrations

```bash
# Install goose migration tool
go install github.com/pressly/goose/v3/cmd/goose@latest

# Run payment service migrations
export DATABASE_URL="postgres://user:pass@localhost:5432/yourdb?sslmode=disable"
cd $GOPATH/pkg/mod/github.com/kevin07696/payment-service@<version>/internal/db/migrations
goose postgres "$DATABASE_URL" up
```

### Option B: Copy Migrations to Your Project

```bash
# Copy migration files to your project
cp -r $GOPATH/pkg/mod/github.com/kevin07696/payment-service@<version>/internal/db/migrations/* \
  ./db/migrations/payment_service/

# Run with your existing migration tool
goose postgres "$DATABASE_URL" up
```

**Tables created:**
- `merchants` - Merchant accounts and EPX credentials
- `services` - Service authentication and JWT keys
- `transactions` - Payment transactions (auth, capture, refund, void)
- `payment_methods` - Stored payment methods (credit cards, ACH)
- `subscriptions` - Recurring billing subscriptions
- `chargebacks` - Chargeback records
- `ach_verifications` - ACH micro-deposit verifications

---

## Configuration

### Step 1: Create Configuration Struct

```go
package main

import (
    "context"
    "os"
)

type PaymentConfig struct {
    // Database
    DatabaseURL string

    // EPX Configuration
    EPXBaseURL        string // e.g., "https://services.epxuap.com"
    EPXBrowserPostURL string // e.g., "https://services.epxuap.com/browserpost"

    // Secrets
    MacSecretPath string // Path to MAC secret file or secret manager key

    // Optional: Secret Manager
    SecretManagerType string // "file", "gcp", "aws"
}

func LoadPaymentConfig() *PaymentConfig {
    return &PaymentConfig{
        DatabaseURL:       os.Getenv("DATABASE_URL"),
        EPXBaseURL:        getEnvOrDefault("EPX_BASE_URL", "https://services.epxuap.com"),
        EPXBrowserPostURL: getEnvOrDefault("EPX_BROWSER_POST_URL", "https://services.epxuap.com/browserpost"),
        MacSecretPath:     os.Getenv("MAC_SECRET_PATH"),
        SecretManagerType: getEnvOrDefault("SECRET_MANAGER_TYPE", "file"),
    }
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### Step 2: Environment Variables

Create `.env` file:

```bash
# Database
DATABASE_URL=postgres://user:pass@localhost:5432/yourdb?sslmode=disable

# EPX Gateway
EPX_BASE_URL=https://services.epxuap.com
EPX_BROWSER_POST_URL=https://services.epxuap.com/browserpost

# Secrets (use absolute path or secret manager key)
MAC_SECRET_PATH=/path/to/secrets/epx/staging/mac_secret

# Optional: Secret Manager
SECRET_MANAGER_TYPE=file  # or "gcp", "aws"
```

---

## Initializing Services

### Step 1: Initialize Database Connection

```go
package main

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/kevin07696/payment-service/internal/db/sqlc"
    "go.uber.org/zap"
)

func initDatabase(ctx context.Context, cfg *PaymentConfig) (*pgxpool.Pool, *sqlc.Queries, error) {
    // Create connection pool
    pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to create connection pool: %w", err)
    }

    // Test connection
    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, nil, fmt.Errorf("failed to ping database: %w", err)
    }

    // Create SQLC queries
    queries := sqlc.New(pool)

    return pool, queries, nil
}
```

### Step 2: Initialize EPX Adapters

```go
import (
    "github.com/kevin07696/payment-service/internal/adapters/epx"
    "github.com/kevin07696/payment-service/internal/adapters/ports"
)

func initEPXAdapters(cfg *PaymentConfig, logger *zap.Logger) (
    ports.ServerPostAdapter,
    ports.BrowserPostAdapter,
    ports.KeyExchangeAdapter,
) {
    // Server Post adapter (for backend payments)
    serverPostAdapter := epx.NewServerPostAdapter(
        cfg.EPXBaseURL,
        logger,
    )

    // Browser Post adapter (for frontend forms)
    browserPostAdapter := epx.NewBrowserPostAdapter(
        cfg.EPXBrowserPostURL,
        logger,
    )

    // Key Exchange adapter (for TAC tokens)
    keyExchangeAdapter := epx.NewKeyExchangeAdapter(
        cfg.EPXBaseURL,
        logger,
    )

    return serverPostAdapter, browserPostAdapter, keyExchangeAdapter
}
```

### Step 3: Initialize Secret Manager

```go
import (
    "github.com/kevin07696/payment-service/internal/adapters/secrets"
    "github.com/kevin07696/payment-service/internal/adapters/ports"
)

func initSecretManager(cfg *PaymentConfig) (ports.SecretManagerAdapter, error) {
    switch cfg.SecretManagerType {
    case "file":
        return secrets.NewFileSecretManager(), nil
    case "gcp":
        // Implement GCP Secret Manager adapter
        return nil, fmt.Errorf("GCP secret manager not implemented")
    case "aws":
        // Implement AWS Secrets Manager adapter
        return nil, fmt.Errorf("AWS secret manager not implemented")
    default:
        return secrets.NewFileSecretManager(), nil
    }
}
```

### Step 4: Initialize Payment Services

```go
import (
    "github.com/kevin07696/payment-service/internal/services/payment"
    "github.com/kevin07696/payment-service/internal/services/payment_method"
    "github.com/kevin07696/payment-service/internal/services/subscription"
)

type PaymentServices struct {
    Payment       *payment.PaymentService
    PaymentMethod *payment_method.PaymentMethodService
    Subscription  *subscription.SubscriptionService
}

func initPaymentServices(
    queries *sqlc.Queries,
    serverPost ports.ServerPostAdapter,
    browserPost ports.BrowserPostAdapter,
    secretManager ports.SecretManagerAdapter,
    logger *zap.Logger,
) *PaymentServices {
    // Payment Service
    paymentService := payment.NewPaymentService(
        queries,
        serverPost,
        secretManager,
        logger,
    )

    // Payment Method Service
    paymentMethodService := payment_method.NewPaymentMethodService(
        queries,
        serverPost,
        browserPost,
        secretManager,
        logger,
    )

    // Subscription Service
    subscriptionService := subscription.NewSubscriptionService(
        queries,
        paymentService,
        logger,
    )

    return &PaymentServices{
        Payment:       paymentService,
        PaymentMethod: paymentMethodService,
        Subscription:  subscriptionService,
    }
}
```

---

## Using Payment Services

### Example 1: Process a Payment (Auth + Capture)

```go
package main

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/kevin07696/payment-service/internal/services/payment"
)

func processPayment(ctx context.Context, services *PaymentServices) error {
    // Step 1: Authorize payment
    authReq := &payment.AuthRequest{
        MerchantID:      uuid.MustParse("your-merchant-id"),
        CustomerID:      uuid.MustParse("customer-id"),
        AmountCents:     9999, // $99.99
        Currency:        "USD",
        PaymentMethodID: uuid.MustParse("payment-method-id"),
        IdempotencyKey:  "auth_" + uuid.New().String(),
    }

    authResp, err := services.Payment.Authorize(ctx, authReq)
    if err != nil {
        return fmt.Errorf("authorization failed: %w", err)
    }

    if authResp.Status != "TRANSACTION_STATUS_APPROVED" {
        return fmt.Errorf("payment declined: %s", authResp.Status)
    }

    fmt.Printf("✅ Authorized: %s ($%.2f)\n",
        authResp.TransactionID,
        float64(authResp.AmountCents)/100,
    )

    // Step 2: Capture payment
    captureReq := &payment.CaptureRequest{
        MerchantID:     authReq.MerchantID,
        TransactionID:  authResp.TransactionID,
        AmountCents:    authResp.AmountCents,
        IdempotencyKey: "capture_" + uuid.New().String(),
    }

    captureResp, err := services.Payment.Capture(ctx, captureReq)
    if err != nil {
        return fmt.Errorf("capture failed: %w", err)
    }

    fmt.Printf("✅ Captured: %s ($%.2f)\n",
        captureResp.TransactionID,
        float64(captureResp.AmountCents)/100,
    )

    return nil
}
```

### Example 2: Store Payment Method (Credit Card)

```go
import (
    "github.com/kevin07696/payment-service/internal/services/payment_method"
)

func storePaymentMethod(ctx context.Context, services *PaymentServices, bricToken string) error {
    req := &payment_method.StorePaymentMethodRequest{
        MerchantID:  uuid.MustParse("your-merchant-id"),
        CustomerID:  uuid.MustParse("customer-id"),
        BRICToken:   bricToken,
        IsDefault:   true,
        Description: "Visa ending in 1111",
    }

    resp, err := services.PaymentMethod.StorePaymentMethod(ctx, req)
    if err != nil {
        return fmt.Errorf("failed to store payment method: %w", err)
    }

    fmt.Printf("✅ Stored payment method: %s\n", resp.PaymentMethodID)
    return nil
}
```

### Example 3: Create Subscription

```go
import (
    "time"
    "github.com/kevin07696/payment-service/internal/services/subscription"
)

func createSubscription(ctx context.Context, services *PaymentServices) error {
    req := &subscription.CreateSubscriptionRequest{
        MerchantID:      uuid.MustParse("your-merchant-id"),
        CustomerID:      uuid.MustParse("customer-id"),
        PaymentMethodID: uuid.MustParse("payment-method-id"),
        AmountCents:     2999, // $29.99/month
        Currency:        "USD",
        Interval:        "MONTHLY",
        StartDate:       time.Now(),
    }

    resp, err := services.Subscription.CreateSubscription(ctx, req)
    if err != nil {
        return fmt.Errorf("failed to create subscription: %w", err)
    }

    fmt.Printf("✅ Created subscription: %s ($%.2f/%s)\n",
        resp.SubscriptionID,
        float64(resp.AmountCents)/100,
        resp.Interval,
    )

    return nil
}
```

---

## Complete Example

### main.go

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/google/uuid"
    "github.com/kevin07696/payment-service/internal/services/payment"
    "go.uber.org/zap"
)

func main() {
    ctx := context.Background()

    // 1. Load configuration
    cfg := LoadPaymentConfig()

    // 2. Initialize logger
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // 3. Initialize database
    pool, queries, err := initDatabase(ctx, cfg)
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    defer pool.Close()

    // 4. Initialize EPX adapters
    serverPost, browserPost, keyExchange := initEPXAdapters(cfg, logger)

    // 5. Initialize secret manager
    secretManager, err := initSecretManager(cfg)
    if err != nil {
        log.Fatalf("Failed to initialize secret manager: %v", err)
    }

    // 6. Initialize payment services
    services := initPaymentServices(
        queries,
        serverPost,
        browserPost,
        secretManager,
        logger,
    )

    // 7. Use payment services
    merchantID := uuid.MustParse(os.Getenv("MERCHANT_ID"))
    customerID := uuid.MustParse(os.Getenv("CUSTOMER_ID"))
    paymentMethodID := uuid.MustParse(os.Getenv("PAYMENT_METHOD_ID"))

    // Process a $99.99 payment
    authReq := &payment.AuthRequest{
        MerchantID:      merchantID,
        CustomerID:      customerID,
        AmountCents:     9999,
        Currency:        "USD",
        PaymentMethodID: paymentMethodID,
        IdempotencyKey:  "order_" + uuid.New().String(),
    }

    authResp, err := services.Payment.Authorize(ctx, authReq)
    if err != nil {
        log.Fatalf("Authorization failed: %v", err)
    }

    if authResp.Status == "TRANSACTION_STATUS_APPROVED" {
        fmt.Printf("✅ Payment authorized: %s\n", authResp.TransactionID)

        // Capture the payment
        captureReq := &payment.CaptureRequest{
            MerchantID:     merchantID,
            TransactionID:  authResp.TransactionID,
            AmountCents:    authResp.AmountCents,
            IdempotencyKey: "capture_" + uuid.New().String(),
        }

        captureResp, err := services.Payment.Capture(ctx, captureReq)
        if err != nil {
            log.Fatalf("Capture failed: %v", err)
        }

        fmt.Printf("✅ Payment captured: %s ($%.2f)\n",
            captureResp.TransactionID,
            float64(captureResp.AmountCents)/100,
        )
    } else {
        fmt.Printf("❌ Payment declined: %s\n", authResp.Status)
    }
}
```

---

## Best Practices

### 1. Use Dependency Injection

Don't create payment services globally. Inject them into your handlers/controllers:

```go
type OrderHandler struct {
    paymentService *payment.PaymentService
    db             *sqlc.Queries
}

func NewOrderHandler(paymentService *payment.PaymentService, db *sqlc.Queries) *OrderHandler {
    return &OrderHandler{
        paymentService: paymentService,
        db:             db,
    }
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *CreateOrderRequest) error {
    // Use injected payment service
    authResp, err := h.paymentService.Authorize(ctx, &payment.AuthRequest{
        // ...
    })
    // ...
}
```

### 2. Share Database Connection Pool

Reuse the same `pgxpool.Pool` for both your app and payment services:

```go
// Don't create multiple pools
pool, _ := pgxpool.New(ctx, databaseURL)
queries := sqlc.New(pool)

// Share the pool
appQueries := yourapp.New(pool)
paymentQueries := sqlc.New(pool) // Payment service uses same pool
```

### 3. Use Transactions for Atomic Operations

Wrap payment operations in database transactions:

```go
import "github.com/jackc/pgx/v5"

func createOrderWithPayment(ctx context.Context, pool *pgxpool.Pool, services *PaymentServices) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)

    // Create order in your database
    orderID, err := createOrder(ctx, tx, orderData)
    if err != nil {
        return err
    }

    // Process payment (payment service uses same transaction)
    authResp, err := services.Payment.AuthorizeWithTx(ctx, tx, &payment.AuthRequest{
        // ...
    })
    if err != nil {
        return err // Rollback order and payment
    }

    // Commit both order and payment
    return tx.Commit(ctx)
}
```

### 4. Handle Idempotency

Always use unique idempotency keys to prevent duplicate charges:

```go
// Good: Order-specific idempotency key
idempotencyKey := fmt.Sprintf("order_%s_payment", orderID)

// Bad: Random key (can't retry safely)
idempotencyKey := uuid.New().String()
```

### 5. Log Payment Operations

Use structured logging for payment operations:

```go
logger.Info("Processing payment",
    zap.String("order_id", orderID.String()),
    zap.String("customer_id", customerID.String()),
    zap.Int64("amount_cents", amountCents),
)

authResp, err := services.Payment.Authorize(ctx, authReq)
if err != nil {
    logger.Error("Payment authorization failed",
        zap.Error(err),
        zap.String("order_id", orderID.String()),
    )
    return err
}

logger.Info("Payment authorized",
    zap.String("transaction_id", authResp.TransactionID.String()),
    zap.String("auth_code", authResp.AuthorizationCode),
)
```

### 6. Error Handling

Handle payment-specific errors appropriately:

```go
authResp, err := services.Payment.Authorize(ctx, authReq)
if err != nil {
    // Log error with context
    logger.Error("Authorization failed", zap.Error(err))

    // Return user-friendly error
    return fmt.Errorf("unable to process payment: %w", err)
}

// Check authorization status
switch authResp.Status {
case "TRANSACTION_STATUS_APPROVED":
    // Proceed with order fulfillment
case "TRANSACTION_STATUS_DECLINED":
    return fmt.Errorf("payment declined: insufficient funds or invalid card")
case "TRANSACTION_STATUS_ERROR":
    return fmt.Errorf("payment gateway error: please try again")
default:
    return fmt.Errorf("unexpected payment status: %s", authResp.Status)
}
```

---

## Migration to Microservice

If your application grows and you need to migrate to a microservice architecture:

### Step 1: Extract Payment Service

```bash
# Deploy payment service as separate server
docker build -t payment-service .
docker run -p 8081:8081 payment-service
```

### Step 2: Switch to ConnectRPC Client

Replace direct service calls with ConnectRPC calls:

```go
// Before: Direct service call
authResp, err := services.Payment.Authorize(ctx, authReq)

// After: ConnectRPC call
import "connectrpc.com/connect"
import paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"

client := paymentv1.NewPaymentServiceClient(
    http.DefaultClient,
    "http://localhost:8081",
)

authResp, err := client.Authorize(ctx, connect.NewRequest(&paymentv1.AuthRequest{
    MerchantId:      merchantID.String(),
    AmountCents:     9999,
    PaymentMethodId: paymentMethodID.String(),
    // ...
}))
```

### Step 3: Remove Module Dependency

```bash
# Remove payment service module
go mod edit -droprequire github.com/kevin07696/payment-service
go mod tidy
```

---

## Troubleshooting

### Issue: Import cycle detected

**Cause:** Circular dependency between your app and payment service

**Solution:** Use interfaces to break the cycle:

```go
// Define interface in your app
type PaymentProcessor interface {
    Authorize(ctx context.Context, req *AuthRequest) (*AuthResponse, error)
}

// Payment service implements the interface
var _ PaymentProcessor = (*payment.PaymentService)(nil)
```

### Issue: Database connection pool exhausted

**Cause:** Too many concurrent payment operations

**Solution:** Limit concurrent operations:

```go
import "golang.org/x/sync/semaphore"

// Limit to 10 concurrent payments
sem := semaphore.NewWeighted(10)

func processPayment(ctx context.Context) error {
    if err := sem.Acquire(ctx, 1); err != nil {
        return err
    }
    defer sem.Release(1)

    // Process payment
    return services.Payment.Authorize(ctx, authReq)
}
```

### Issue: MAC secret not found

**Cause:** Secret manager can't find MAC secret file

**Solution:** Use absolute paths in configuration:

```bash
# Use absolute path
MAC_SECRET_PATH=/absolute/path/to/secrets/epx/staging/mac_secret

# Or use secret manager
SECRET_MANAGER_TYPE=gcp
MAC_SECRET_PATH=projects/my-project/secrets/epx-mac/versions/latest
```

---

## Next Steps

- **[INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md)** - Microservice integration alternative
- **[API_SPECS.md](API_SPECS.md)** - Complete service API reference
- **[DATABASE.md](DATABASE.md)** - Database schema documentation
- **[DEVELOP.md](DEVELOP.md)** - Contributing to the payment service

---

**Questions?** Open an issue on [GitHub](https://github.com/kevin07696/payment-service/issues)
