# Module Integration Guide

**Last Updated:** 2025-12-02
**Target Audience:** Go developers integrating payment service as a library/module
**Topic:** Using the payment service as an embedded Go module in your application
**Goal:** Import and use payment service logic directly without running a separate server

---

## Overview

The payment service can be integrated in two ways:

### Option 1: Microservice Architecture (Recommended for Production)
- Payment service runs as a **separate server** (ConnectRPC on port 8080)
- Your app makes **HTTP/ConnectRPC calls** to the payment service
- **Pros:** Service isolation, independent scaling, language-agnostic clients, easier updates
- **Cons:** Network latency (~1-2ms), additional operational complexity
- **See:** [GETTING_STARTED.md](GETTING_STARTED.md), [REACT_INTEGRATION.md](REACT_INTEGRATION.md)

### Option 2: Module/Library Integration (This Guide)
- Payment service imported as a **Go module** into your application
- Your app uses payment **services directly** (no HTTP calls)
- **Pros:** No network latency, simpler deployment, single binary
- **Cons:** Go-only, tight coupling, shared resources, harder to upgrade

**When to use module integration:**
- ✅ Your application is written in Go
- ✅ You want to minimize network overhead (< 1ms latency critical)
- ✅ You prefer a monolithic architecture
- ✅ You don't need independent scaling of payment logic
- ✅ You're building a small-to-medium application (< 1000 TPS)
- ✅ You have tight integration requirements (same database transactions)

**When NOT to use module integration:**
- ❌ You need to scale payment processing independently
- ❌ You have non-Go services that need payment capabilities
- ❌ You want to update payment service without redeploying your app
- ❌ You need to run multiple versions of payment logic
- ❌ You're building a microservices architecture

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Database Setup](#database-setup)
4. [Configuration](#configuration)
5. [Initializing Services](#initializing-services)
6. [Using Payment Services](#using-payment-services)
7. [Advanced Features](#advanced-features)
8. [Complete Example](#complete-example)
9. [Best Practices](#best-practices)
10. [Migration to Microservice](#migration-to-microservice)

---

## Prerequisites

**Required:**
- Go 1.21+ installed
- PostgreSQL 15+ running
- EPX merchant account and credentials (or North gateway account)
- Basic understanding of Go modules and dependency injection

**Your application must:**
- Be written in Go
- Use PostgreSQL (payment service uses pgx/v5)
- Support dependency injection pattern
- Handle context cancellation properly

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
package main

import (
    // Services
    "github.com/kevin07696/payment-service/internal/services/payment"
    "github.com/kevin07696/payment-service/internal/services/payment_method"
    "github.com/kevin07696/payment-service/internal/services/subscription"
    "github.com/kevin07696/payment-service/internal/services/merchant"

    // Adapters
    "github.com/kevin07696/payment-service/internal/adapters/epx"
    "github.com/kevin07696/payment-service/internal/adapters/database"
    "github.com/kevin07696/payment-service/internal/adapters/gcp"

    // Database (SQLC generated)
    "github.com/kevin07696/payment-service/internal/db/sqlc"

    // Ports (interfaces)
    "github.com/kevin07696/payment-service/internal/adapters/ports"
    "github.com/kevin07696/payment-service/internal/services/ports"
)
```

---

## Database Setup

The payment service requires its own database tables. Run migrations in your PostgreSQL database.

### Migration Files

The payment service includes **24 migrations** that create:
- `merchants` - Merchant accounts and EPX credentials
- `services` - Service authentication with RSA public keys
- `transactions` - Payment transactions (auth, capture, refund, void)
- `payment_methods` - Stored payment methods (credit cards, ACH)
- `subscriptions` - Recurring billing subscriptions
- `chargebacks` - Chargeback records
- `ach_verifications` - ACH micro-deposit verifications
- `service_public_keys` - JWT authentication keys
- `audit_logs` - Security audit trail
- `rate_limit_cache` - UNLOGGED table for distributed rate limiting

### Option A: Run Migrations from Module

```bash
# Install goose migration tool
go install github.com/pressly/goose/v3/cmd/goose@latest

# Set database URL
export DATABASE_URL="postgres://user:pass@localhost:5432/yourdb?sslmode=disable"

# Find module path
PAYMENT_MODULE=$(go list -m -f '{{.Dir}}' github.com/kevin07696/payment-service)

# Run migrations
cd "$PAYMENT_MODULE/internal/db/migrations"
goose postgres "$DATABASE_URL" up

# Verify
goose postgres "$DATABASE_URL" status
```

### Option B: Copy Migrations to Your Project

```bash
# Find module path
PAYMENT_MODULE=$(go list -m -f '{{.Dir}}' github.com/kevin07696/payment-service)

# Copy migration files
mkdir -p ./db/migrations/payment_service
cp "$PAYMENT_MODULE/internal/db/migrations"/*.sql ./db/migrations/payment_service/

# Run with your existing migration tool
goose postgres "$DATABASE_URL" -dir ./db/migrations/payment_service up
```

### Schema Verification

After migrations, verify tables exist:

```sql
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
  AND table_name IN (
    'merchants',
    'services',
    'transactions',
    'payment_methods',
    'subscriptions',
    'rate_limit_cache'
  )
ORDER BY table_name;
```

---

## Configuration

### Step 1: Create Configuration Struct

```go
package main

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

type PaymentConfig struct {
    // Database
    DBHost     string
    DBPort     int
    DBUser     string
    DBPassword string
    DBName     string
    DBSSLMode  string
    MaxConns   int32
    MinConns   int32

    // EPX Gateway
    EPXServerPostURL    string
    EPXBrowserPostURL   string
    EPXKeyExchangeURL   string
    EPXCustomerNumber   string
    EPXMerchantNumber   string
    EPXDBANumber        string
    EPXTerminalNumber   string
    EPXMACSecret        string
    EPXTimeout          time.Duration

    // Secret Management
    SecretManagerType   string // "gcp", "aws", "vault", "local", "mock"
    GCPProjectID        string // For GCP Secret Manager
    LocalSecretsPath    string // For local file-based secrets
    SecretCacheTTL      time.Duration

    // Application
    CallbackBaseURL     string // For Browser Post callbacks
    Environment         string // "development" or "production"
}

func LoadPaymentConfig() (*PaymentConfig, error) {
    cfg := &PaymentConfig{
        // Database
        DBHost:     getEnvOrDefault("DB_HOST", "localhost"),
        DBPort:     getEnvIntOrDefault("DB_PORT", 5432),
        DBUser:     getEnvOrDefault("DB_USER", "postgres"),
        DBPassword: getEnvOrDefault("DB_PASSWORD", "postgres"),
        DBName:     getEnvOrDefault("DB_NAME", "payment_service"),
        DBSSLMode:  getEnvOrDefault("DB_SSL_MODE", "disable"),
        MaxConns:   int32(getEnvIntOrDefault("DB_MAX_CONNS", 25)),
        MinConns:   int32(getEnvIntOrDefault("DB_MIN_CONNS", 5)),

        // EPX Gateway
        EPXServerPostURL:  getEnvOrDefault("EPX_SERVER_POST_URL", "https://services.epxuap.com"),
        EPXBrowserPostURL: getEnvOrDefault("EPX_BROWSER_POST_URL", "https://services.epxuap.com/browserpost"),
        EPXKeyExchangeURL: getEnvOrDefault("EPX_KEY_EXCHANGE_URL", "https://services.epxuap.com"),
        EPXCustomerNumber: os.Getenv("EPX_CUST_NBR"),
        EPXMerchantNumber: os.Getenv("EPX_MERCH_NBR"),
        EPXDBANumber:      os.Getenv("EPX_DBA_NBR"),
        EPXTerminalNumber: os.Getenv("EPX_TERMINAL_NBR"),
        EPXMACSecret:      os.Getenv("EPX_SANDBOX_MAC"),
        EPXTimeout:        time.Duration(getEnvIntOrDefault("EPX_TIMEOUT", 30)) * time.Second,

        // Secret Management
        SecretManagerType: getEnvOrDefault("SECRET_MANAGER", "local"),
        GCPProjectID:      os.Getenv("GCP_PROJECT_ID"),
        LocalSecretsPath:  getEnvOrDefault("LOCAL_SECRETS_BASE_PATH", "./secrets"),
        SecretCacheTTL:    time.Duration(getEnvIntOrDefault("SECRET_CACHE_TTL_MINUTES", 5)) * time.Minute,

        // Application
        CallbackBaseURL: getEnvOrDefault("CALLBACK_BASE_URL", "http://localhost:8081"),
        Environment:     getEnvOrDefault("ENVIRONMENT", "development"),
    }

    // Validate required fields
    if cfg.SecretManagerType == "gcp" && cfg.GCPProjectID == "" {
        return nil, fmt.Errorf("GCP_PROJECT_ID required when using GCP secret manager")
    }

    return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if intVal, err := strconv.Atoi(value); err == nil {
            return intVal
        }
    }
    return defaultValue
}
```

### Step 2: Environment Variables

Create `.env` file:

```bash
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=payment_service
DB_SSL_MODE=disable
DB_MAX_CONNS=25
DB_MIN_CONNS=5

# EPX Gateway Configuration
EPX_SERVER_POST_URL=https://services.epxuap.com
EPX_BROWSER_POST_URL=https://services.epxuap.com/browserpost
EPX_KEY_EXCHANGE_URL=https://services.epxuap.com
EPX_CUST_NBR=9001
EPX_MERCH_NBR=900300
EPX_DBA_NBR=2
EPX_TERMINAL_NBR=77
EPX_SANDBOX_MAC=your-sandbox-mac-here
EPX_TIMEOUT=30

# Secret Management (choose one)
SECRET_MANAGER=local  # Options: gcp, aws, vault, local, mock
LOCAL_SECRETS_BASE_PATH=./secrets
SECRET_CACHE_TTL_MINUTES=5

# GCP Secret Manager (if using)
# GCP_PROJECT_ID=your-project-id
# GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

# AWS Secrets Manager (if using)
# AWS_REGION=us-east-1
# AWS_PROFILE=your-profile

# HashiCorp Vault (if using)
# VAULT_ADDR=https://vault.example.com
# VAULT_TOKEN=your-vault-token
# Or use AppRole:
# VAULT_ROLE_ID=your-role-id
# VAULT_SECRET_ID=your-secret-id

# Application
CALLBACK_BASE_URL=http://localhost:8081
ENVIRONMENT=development
```

---

## Initializing Services

### Step 1: Initialize Database Connection

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/kevin07696/payment-service/internal/adapters/database"
    "github.com/kevin07696/payment-service/internal/db/sqlc"
    "go.uber.org/zap"
)

func initDatabase(ctx context.Context, cfg *PaymentConfig, logger *zap.Logger) (*database.PostgreSQLAdapter, *sqlc.Queries, error) {
    // Build connection string
    connString := fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
    )

    // Configure connection pool
    poolConfig, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to parse connection string: %w", err)
    }

    poolConfig.MaxConns = cfg.MaxConns
    poolConfig.MinConns = cfg.MinConns
    poolConfig.MaxConnLifetime = 1 * time.Hour
    poolConfig.MaxConnIdleTime = 30 * time.Minute
    poolConfig.HealthCheckPeriod = 1 * time.Minute

    // Create connection pool
    pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to create connection pool: %w", err)
    }

    // Test connection
    if err := pool.Ping(ctx); err != nil {
        pool.Close()
        return nil, nil, fmt.Errorf("failed to ping database: %w", err)
    }

    logger.Info("Database connection established",
        zap.String("host", cfg.DBHost),
        zap.Int("port", cfg.DBPort),
        zap.String("database", cfg.DBName),
        zap.Int32("max_conns", cfg.MaxConns),
    )

    // Create database adapter with query timeouts
    dbAdapter := database.NewPostgreSQLAdapter(pool, logger)

    // Create SQLC queries
    queries := sqlc.New(pool)

    return dbAdapter, queries, nil
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
    // Server Post adapter - HTTP/2 with circuit breaker and retry logic
    // Also handles BRIC storage (tokenization) via TransactionTypeBRICStorageCC/ACH
    serverPostAdapter := epx.NewServerPostAdapter(
        cfg.EPXServerPostURL,
        logger,
    )

    // Browser Post adapter - Frontend form tokenization
    browserPostAdapter := epx.NewBrowserPostAdapter(
        cfg.EPXBrowserPostURL,
        logger,
    )

    // Key Exchange adapter - TAC token generation for Browser Post
    keyExchangeAdapter := epx.NewKeyExchangeAdapter(
        cfg.EPXKeyExchangeURL,
        logger,
    )

    return serverPostAdapter, browserPostAdapter, keyExchangeAdapter
}
```

### Step 3: Initialize Secret Manager

```go
import (
    "github.com/kevin07696/payment-service/internal/adapters/gcp"
    "github.com/kevin07696/payment-service/internal/adapters/secrets"
    "github.com/kevin07696/payment-service/internal/adapters/ports"
)

func initSecretManager(ctx context.Context, cfg *PaymentConfig, logger *zap.Logger) (ports.SecretManagerAdapter, error) {
    switch cfg.SecretManagerType {
    case "gcp":
        return gcp.NewSecretManager(ctx, cfg.GCPProjectID, logger)

    case "aws":
        return secrets.NewAWSSecretsManager(ctx, logger)

    case "vault":
        return secrets.NewVaultAdapter(logger)

    case "local":
        return secrets.NewLocalSecretManager(cfg.LocalSecretsPath, logger), nil

    case "mock":
        logger.Warn("Using mock secret manager - not for production!")
        return secrets.NewMockSecretManager(), nil

    default:
        return nil, fmt.Errorf("unknown secret manager type: %s", cfg.SecretManagerType)
    }
}
```

### Step 4: Initialize Caches (Performance Optimization)

```go
import (
    merchantService "github.com/kevin07696/payment-service/internal/services/merchant"
    paymentMethodService "github.com/kevin07696/payment-service/internal/services/payment_method"
)

func initCaches(logger *zap.Logger) (
    *merchantService.MerchantCredentialCache,
    *paymentMethodService.PaymentMethodCache,
) {
    // Merchant Credential Cache (P2-1)
    // - 70% database load reduction
    // - 5-minute TTL, 1000 merchants max
    merchantCache := merchantService.NewMerchantCredentialCache(
        5*time.Minute,  // TTL
        1000,          // Max size
        logger,
    )

    // Payment Method Cache (P2-2)
    // - 60% faster lookups
    // - 2-minute TTL, 10K methods max
    paymentMethodCache := paymentMethodService.NewPaymentMethodCache(
        2*time.Minute,  // TTL (shorter for fresher data)
        10000,         // Max size
        logger,
    )

    return merchantCache, paymentMethodCache
}
```

### Step 5: Initialize Payment Services

```go
import (
    "github.com/kevin07696/payment-service/internal/services/payment"
    "github.com/kevin07696/payment-service/internal/services/payment_method"
    "github.com/kevin07696/payment-service/internal/services/subscription"
    "github.com/kevin07696/payment-service/internal/services/merchant"
    "github.com/kevin07696/payment-service/internal/services/webhook"
)

type PaymentServices struct {
    Payment       *payment.PaymentService
    PaymentMethod *payment_method.PaymentMethodService
    Subscription  *subscription.SubscriptionService
    Merchant      *merchant.MerchantService
    Webhook       *webhook.WebhookDeliveryService
}

func initPaymentServices(
    dbAdapter *database.PostgreSQLAdapter,
    queries *sqlc.Queries,
    serverPost ports.ServerPostAdapter,
    browserPost ports.BrowserPostAdapter,
    secretManager ports.SecretManagerAdapter,
    merchantCache *merchantService.MerchantCredentialCache,
    paymentMethodCache *paymentMethodService.PaymentMethodCache,
    logger *zap.Logger,
) *PaymentServices {
    // Transaction Manager for atomic operations
    txManager := database.NewTransactionManager(dbAdapter)

    // Merchant Service (credential management)
    merchantSvc := merchant.NewMerchantService(
        queries,
        txManager,
        secretManager,
        merchantCache,
        logger,
    )

    // Payment Service
    paymentSvc := payment.NewPaymentService(
        queries,
        txManager,
        serverPost,
        secretManager,
        merchantCache,
        logger,
    )

    // Payment Method Service
    // Note: BRIC storage (tokenization) is handled through ServerPostAdapter
    paymentMethodSvc := payment_method.NewPaymentMethodService(
        queries,
        txManager,
        browserPost,
        serverPost,  // Also handles BRIC storage via TransactionTypeBRICStorageCC/ACH
        secretManager,
        paymentMethodCache,
        logger,
    )

    // Webhook Delivery Service
    webhookSvc := webhook.NewWebhookDeliveryService(
        dbAdapter,
        logger,
    )

    // Subscription Service
    subscriptionSvc := subscription.NewSubscriptionService(
        queries,
        paymentSvc,
        webhookSvc,
        logger,
    )

    return &PaymentServices{
        Payment:       paymentSvc,
        PaymentMethod: paymentMethodSvc,
        Subscription:  subscriptionSvc,
        Merchant:      merchantSvc,
        Webhook:       webhookSvc,
    }
}
```

---

## Using Payment Services

### Example 1: Process a Sale (Auth + Capture in one call)

```go
package main

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/kevin07696/payment-service/internal/domain"
)

func processSale(ctx context.Context, services *PaymentServices) error {
    merchantID := uuid.MustParse("your-merchant-id")
    customerID := "customer-123"  // Your customer ID
    paymentMethodID := uuid.MustParse("payment-method-id")

    // Sale = Authorize + Capture in single operation
    orderID := "ORD-2024-001"
    resp, err := services.Payment.Sale(ctx, &domain.SaleRequest{
        MerchantID:      merchantID,
        CustomerID:      customerID,
        OrderID:         &orderID, // Link to merchant's order/invoice system
        AmountCents:     9999, // $99.99
        Currency:        "USD",
        PaymentMethodID: paymentMethodID,
        IdempotencyKey:  fmt.Sprintf("sale_%s_%s", customerID, uuid.New().String()),
        Metadata: map[string]string{
            "product": "Premium Subscription",
        },
    })

    if err != nil {
        return fmt.Errorf("sale failed: %w", err)
    }

    if !resp.IsApproved {
        return fmt.Errorf("payment declined: %s", resp.ErrorMessage)
    }

    fmt.Printf("✅ Sale completed: %s ($%.2f)\n",
        resp.TransactionID,
        float64(resp.AmountCents)/100,
    )
    fmt.Printf("   Authorization Code: %s\n", resp.AuthorizationCode)
    fmt.Printf("   Card: %s **** %s\n", resp.CardInfo.Brand, resp.CardInfo.Last4)

    return nil
}
```

### Example 2: Two-Step Payment (Authorize + Capture)

```go
func processTwoStepPayment(ctx context.Context, services *PaymentServices) error {
    merchantID := uuid.MustParse("your-merchant-id")
    customerID := "customer-123"
    paymentMethodID := uuid.MustParse("payment-method-id")

    // Step 1: Authorize (hold funds, don't capture)
    authResp, err := services.Payment.Authorize(ctx, &domain.AuthorizeRequest{
        MerchantID:      merchantID,
        CustomerID:      customerID,
        AmountCents:     9999,
        Currency:        "USD",
        PaymentMethodID: paymentMethodID,
        IdempotencyKey:  fmt.Sprintf("auth_%s_%s", customerID, uuid.New().String()),
    })

    if err != nil {
        return fmt.Errorf("authorization failed: %w", err)
    }

    if !authResp.IsApproved {
        return fmt.Errorf("payment declined: %s", authResp.ErrorMessage)
    }

    fmt.Printf("✅ Authorized: %s ($%.2f)\n",
        authResp.TransactionID,
        float64(authResp.AmountCents)/100,
    )

    // Step 2: Capture (complete the payment)
    // You might do this after shipping the product, for example
    captureResp, err := services.Payment.Capture(ctx, &domain.CaptureRequest{
        MerchantID:     merchantID,
        TransactionID:  authResp.TransactionID,
        AmountCents:    authResp.AmountCents, // Can be partial capture
        IdempotencyKey: fmt.Sprintf("capture_%s", authResp.TransactionID),
    })

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

### Example 3: Refund a Payment

```go
func processRefund(ctx context.Context, services *PaymentServices, originalTxnID uuid.UUID) error {
    merchantID := uuid.MustParse("your-merchant-id")

    resp, err := services.Payment.Refund(ctx, &domain.RefundRequest{
        MerchantID:     merchantID,
        TransactionID:  originalTxnID,
        AmountCents:    5000, // Partial refund of $50.00
        Reason:         "Customer requested refund",
        IdempotencyKey: fmt.Sprintf("refund_%s_%s", originalTxnID, uuid.New().String()),
    })

    if err != nil {
        return fmt.Errorf("refund failed: %w", err)
    }

    fmt.Printf("✅ Refunded: %s ($%.2f)\n",
        resp.TransactionID,
        float64(resp.AmountCents)/100,
    )

    return nil
}
```

### Example 4: Store Payment Method (Credit Card via BRIC Token)

```go
func storePaymentMethod(ctx context.Context, services *PaymentServices, bricToken string) error {
    merchantID := uuid.MustParse("your-merchant-id")
    customerID := "customer-123"

    resp, err := services.PaymentMethod.TokenizePaymentMethod(ctx, &domain.TokenizeRequest{
        MerchantID:  merchantID,
        CustomerID:  customerID,
        PaymentToken: bricToken,  // From Browser Post form
        Type:        domain.PaymentMethodTypeCreditCard,
        Metadata: map[string]string{
            "nickname": "Primary Visa",
            "billing_address": "123 Main St",
        },
    })

    if err != nil {
        return fmt.Errorf("failed to store payment method: %w", err)
    }

    fmt.Printf("✅ Payment method stored: %s\n", resp.PaymentMethodID)
    fmt.Printf("   Type: %s\n", resp.Type)
    if resp.CardInfo != nil {
        fmt.Printf("   Card: %s **** %s (exp %02d/%d)\n",
            resp.CardInfo.Brand,
            resp.CardInfo.Last4,
            resp.CardInfo.ExpiryMonth,
            resp.CardInfo.ExpiryYear,
        )
    }

    return nil
}
```

### Example 5: Create Recurring Subscription

```go
func createSubscription(ctx context.Context, services *PaymentServices) error {
    merchantID := uuid.MustParse("your-merchant-id")
    customerID := "customer-123"
    paymentMethodID := uuid.MustParse("payment-method-id")

    resp, err := services.Subscription.CreateSubscription(ctx, &domain.CreateSubscriptionRequest{
        MerchantID:      merchantID,
        CustomerID:      customerID,
        PaymentMethodID: paymentMethodID,
        AmountCents:     2999, // $29.99/month
        Currency:        "USD",
        IntervalType:    "monthly",
        StartDate:       time.Now().Add(24 * time.Hour), // Start tomorrow
        Metadata: map[string]string{
            "plan_name": "Premium Monthly",
            "plan_id":   "plan_premium_monthly",
        },
    })

    if err != nil {
        return fmt.Errorf("failed to create subscription: %w", err)
    }

    fmt.Printf("✅ Subscription created: %s\n", resp.SubscriptionID)
    fmt.Printf("   Amount: $%.2f/%s\n",
        float64(resp.AmountCents)/100,
        resp.IntervalType,
    )
    fmt.Printf("   Next billing: %s\n", resp.NextBillingDate.Format("2006-01-02"))

    return nil
}
```

### Example 6: List Transactions

```go
func listTransactions(ctx context.Context, services *PaymentServices) error {
    merchantID := uuid.MustParse("your-merchant-id")

    resp, err := services.Payment.ListTransactions(ctx, &domain.ListTransactionsRequest{
        MerchantID: merchantID,
        Limit:      10,
        Offset:     0,
    })

    if err != nil {
        return fmt.Errorf("failed to list transactions: %w", err)
    }

    fmt.Printf("Found %d transactions:\n", resp.Total)
    for _, txn := range resp.Transactions {
        fmt.Printf("  %s - $%.2f - %s - %s\n",
            txn.TransactionID,
            float64(txn.AmountCents)/100,
            txn.Type,
            txn.Status,
        )
    }

    return nil
}
```

---

## Advanced Features

### 1. Using Merchant Credential Cache

The payment service includes a built-in merchant credential cache that reduces database load by 70%:

```go
// Cache is already initialized and used internally by services
// No additional code needed - it just works!

// Cache configuration (done during initialization):
merchantCache := merchantService.NewMerchantCredentialCache(
    5*time.Minute,  // TTL - credentials cached for 5 minutes
    1000,          // Max size - up to 1000 merchants
    logger,
)

// Services automatically use the cache
// First call: hits database
// Subsequent calls: served from cache (< 1ms latency)
```

### 2. Using Payment Method Cache

The payment method cache provides 60% faster lookups:

```go
// Also automatically used by PaymentMethodService
// No configuration needed

// Cache invalidates on updates:
// - When payment method is updated
// - When payment method is deleted
// - After 2-minute TTL (for consistency)
```

### 3. Transaction Management (Atomic Operations)

Wrap multiple operations in a database transaction:

```go
import "github.com/jackc/pgx/v5"

func createOrderWithPayment(
    ctx context.Context,
    dbAdapter *database.PostgreSQLAdapter,
    services *PaymentServices,
) error {
    // Start transaction
    tx, err := dbAdapter.BeginTx(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)

    // Create order in your database
    orderID, err := createOrder(ctx, tx, orderData)
    if err != nil {
        return err // Automatic rollback
    }

    // Process payment (uses same transaction)
    authResp, err := services.Payment.AuthorizeWithTx(ctx, tx, &domain.AuthorizeRequest{
        // ...
    })
    if err != nil {
        return err // Automatic rollback (order + payment)
    }

    // Commit both operations atomically
    if err := tx.Commit(ctx); err != nil {
        return err
    }

    fmt.Printf("✅ Order %s created with payment %s\n", orderID, authResp.TransactionID)
    return nil
}
```

### 4. Error Handling with Domain Errors

The service uses structured domain errors with error codes:

```go
import "github.com/kevin07696/payment-service/internal/domain"

resp, err := services.Payment.Sale(ctx, req)
if err != nil {
    // Check for specific error types
    if domain.IsDomainError(err, domain.ErrorCodePMExpired) {
        return fmt.Errorf("payment method expired, please update card")
    }

    if domain.IsNotFoundError(err) {
        return fmt.Errorf("payment method not found")
    }

    if domain.IsAuthError(err) {
        return fmt.Errorf("authentication failed")
    }

    if domain.IsGatewayError(err) {
        return fmt.Errorf("payment gateway error, please try again")
    }

    // Get error code for logging/metrics
    errorCode := domain.GetErrorCode(err)
    logger.Error("Payment failed",
        zap.String("error_code", string(errorCode)),
        zap.Error(err),
    )

    return err
}
```

### 5. Secret Management Best Practices

```go
// Secrets are cached for 5 minutes by default
// For production, use GCP/AWS/Vault secret managers

// GCP Secret Manager example:
secretManager, err := gcp.NewSecretManager(ctx, "your-project-id", logger)

// Secrets are automatically fetched and cached:
// - MAC secrets for EPX authentication
// - JWT signing keys for service authentication
// - Any merchant-specific credentials

// Manual secret retrieval (if needed):
macSecret, err := secretManager.GetSecret(ctx, "epx/staging/mac_secret")
```

---

## Complete Example

### main.go - Full Integration

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/google/uuid"
    "github.com/joho/godotenv"
    "github.com/kevin07696/payment-service/internal/domain"
    "go.uber.org/zap"
)

func main() {
    // Load environment variables
    if err := godotenv.Load(); err != nil {
        log.Printf("Warning: .env file not found")
    }

    ctx := context.Background()

    // 1. Initialize logger
    logger, err := zap.NewProduction()
    if err != nil {
        log.Fatalf("Failed to create logger: %v", err)
    }
    defer logger.Sync()

    // 2. Load configuration
    cfg, err := LoadPaymentConfig()
    if err != nil {
        logger.Fatal("Failed to load config", zap.Error(err))
    }

    // 3. Initialize database
    dbAdapter, queries, err := initDatabase(ctx, cfg, logger)
    if err != nil {
        logger.Fatal("Failed to initialize database", zap.Error(err))
    }
    defer dbAdapter.Close()

    // 4. Initialize EPX adapters
    serverPost, browserPost, keyExchange := initEPXAdapters(cfg, logger)
    _ = keyExchange // Used for Browser Post TAC generation

    // 5. Initialize secret manager
    secretManager, err := initSecretManager(ctx, cfg, logger)
    if err != nil {
        logger.Fatal("Failed to initialize secret manager", zap.Error(err))
    }

    // 6. Initialize caches
    merchantCache, paymentMethodCache := initCaches(logger)

    // 7. Initialize payment services
    services := initPaymentServices(
        dbAdapter,
        queries,
        serverPost,
        browserPost,
        secretManager,
        merchantCache,
        paymentMethodCache,
        logger,
    )

    logger.Info("Payment services initialized successfully")

    // 8. Example usage
    merchantID := uuid.MustParse(os.Getenv("MERCHANT_ID"))
    customerID := "customer-123"
    paymentMethodID := uuid.MustParse(os.Getenv("PAYMENT_METHOD_ID"))

    // Process a $99.99 sale
    orderID := "ORD-2024-001"
    resp, err := services.Payment.Sale(ctx, &domain.SaleRequest{
        MerchantID:      merchantID,
        CustomerID:      customerID,
        OrderID:         &orderID, // Link to merchant's order/invoice system
        AmountCents:     9999,
        Currency:        "USD",
        PaymentMethodID: paymentMethodID,
        IdempotencyKey:  fmt.Sprintf("sale_%s_%d", customerID, time.Now().Unix()),
    })

    if err != nil {
        logger.Fatal("Payment failed", zap.Error(err))
    }

    if resp.IsApproved {
        logger.Info("Payment successful",
            zap.String("transaction_id", resp.TransactionID.String()),
            zap.Int64("amount_cents", resp.AmountCents),
            zap.String("auth_code", resp.AuthorizationCode),
        )
        fmt.Printf("✅ Payment successful: %s ($%.2f)\n",
            resp.TransactionID,
            float64(resp.AmountCents)/100,
        )
    } else {
        logger.Error("Payment declined",
            zap.String("transaction_id", resp.TransactionID.String()),
            zap.String("error_message", resp.ErrorMessage),
        )
        fmt.Printf("❌ Payment declined: %s\n", resp.ErrorMessage)
    }
}
```

---

## Best Practices

### 1. Use Dependency Injection (Not Globals)

```go
// ❌ BAD: Global services
var globalPaymentService *payment.PaymentService

// ✅ GOOD: Inject services into handlers
type OrderHandler struct {
    paymentService *payment.PaymentService
    db             *sqlc.Queries
    logger         *zap.Logger
}

func NewOrderHandler(paymentSvc *payment.PaymentService, db *sqlc.Queries, logger *zap.Logger) *OrderHandler {
    return &OrderHandler{
        paymentService: paymentSvc,
        db:             db,
        logger:         logger,
    }
}
```

### 2. Share Database Connection Pool

```go
// ✅ GOOD: Reuse the same pool
pool, _ := pgxpool.New(ctx, databaseURL)

// Your app queries
appQueries := yourapp.New(pool)

// Payment service uses same pool
dbAdapter := database.NewPostgreSQLAdapter(pool, logger)
paymentQueries := sqlc.New(pool)
```

### 3. Always Use Idempotency Keys

```go
// ✅ GOOD: Deterministic, order-based key
idempotencyKey := fmt.Sprintf("sale_%s_%s", orderID, customerID)

// ❌ BAD: Random key (can't safely retry)
idempotencyKey := uuid.New().String()

// ✅ GOOD: Timestamp-based for unique operations
idempotencyKey := fmt.Sprintf("refund_%s_%d", txnID, time.Now().Unix())
```

### 4. Handle Context Cancellation

```go
// Always respect context cancellation
resp, err := services.Payment.Sale(ctx, req)
if err != nil {
    if ctx.Err() == context.Canceled {
        logger.Warn("Payment canceled by client")
        return fmt.Errorf("request canceled")
    }
    if ctx.Err() == context.DeadlineExceeded {
        logger.Warn("Payment timed out")
        return fmt.Errorf("request timed out")
    }
    return fmt.Errorf("payment failed: %w", err)
}
```

### 5. Use Structured Logging

```go
// ✅ GOOD: Structured fields
logger.Info("Processing payment",
    zap.String("order_id", orderID),
    zap.String("customer_id", customerID),
    zap.Int64("amount_cents", amountCents),
    zap.String("last_4", last4), // Only log last 4 digits
)

// ❌ BAD: Never log full PAN
logger.Info("Card number: " + cardNumber) // PCI violation!

// ✅ GOOD: Use log scrubbing
import "github.com/kevin07696/payment-service/internal/middleware"

scrubbedMsg := middleware.ScrubString("Card 4111111111111111 charged")
logger.Info(scrubbedMsg) // Logs: "Card [REDACTED] charged"
```

### 6. Graceful Error Handling

```go
resp, err := services.Payment.Sale(ctx, req)
if err != nil {
    // Log with context
    logger.Error("Sale failed",
        zap.Error(err),
        zap.String("order_id", orderID),
        zap.String("customer_id", customerID),
    )

    // Return user-friendly error
    if domain.IsGatewayError(err) {
        return fmt.Errorf("payment gateway temporarily unavailable, please try again")
    }

    return fmt.Errorf("unable to process payment: %w", err)
}

// Check approval status
if !resp.IsApproved {
    logger.Warn("Payment declined",
        zap.String("transaction_id", resp.TransactionID.String()),
        zap.String("error_message", resp.ErrorMessage),
    )
    return fmt.Errorf("payment declined: %s", resp.ErrorMessage)
}
```

### 7. Monitor Cache Performance

```go
// Caches automatically track metrics (if Prometheus is available)
// - Cache hit rate
// - Cache evictions
// - Cache size

// Access cache stats programmatically:
stats := merchantCache.Stats()
logger.Info("Merchant cache stats",
    zap.Int("size", stats.Size),
    zap.Float64("hit_rate", stats.HitRate),
)
```

---

## Migration to Microservice

If your application grows, migrating to a microservice architecture is straightforward:

### Step 1: Deploy Payment Service as Separate Server

```bash
# Build payment service
cd $GOPATH/pkg/mod/github.com/kevin07696/payment-service@<version>
go build -o payment-server ./cmd/server

# Run payment service
./payment-server
# Or use Docker:
docker build -t payment-service .
docker run -p 8080:8080 -p 8081:8081 payment-service
```

### Step 2: Generate ConnectRPC Client Stubs

```bash
# Install buf (for proto code generation)
go install github.com/bufbuild/buf/cmd/buf@latest

# Generate Go client from protos
buf generate
```

### Step 3: Replace Direct Calls with ConnectRPC Calls

```go
// Before: Direct service call
resp, err := services.Payment.Sale(ctx, req)

// After: ConnectRPC call
import (
    "connectrpc.com/connect"
    paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
    "github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
)

client := paymentv1connect.NewPaymentServiceClient(
    http.DefaultClient,
    "http://localhost:8081",  // Payment service URL
    connect.WithGRPC(),       // Use gRPC protocol
)

connectReq := connect.NewRequest(&paymentv1.SaleRequest{
    MerchantId:      merchantID.String(),
    CustomerId:      customerID,
    AmountCents:     9999,
    Currency:        "USD",
    PaymentMethodId: paymentMethodID.String(),
    IdempotencyKey:  idempotencyKey,
})

// Add JWT token for authentication
connectReq.Header().Set("Authorization", "Bearer "+jwtToken)

connectResp, err := client.Sale(ctx, connectReq)
if err != nil {
    return fmt.Errorf("sale failed: %w", err)
}

resp := connectResp.Msg
```

### Step 4: Remove Module Dependency

```bash
# Remove payment service from go.mod
go mod edit -droprequire github.com/kevin07696/payment-service

# Clean up
go mod tidy
```

---

## Troubleshooting

### Issue: Import cycle detected

**Cause:** Circular dependency between your app and payment service

**Solution:** Use interfaces to break the cycle

```go
// Define interface in your app
package yourapp

type PaymentProcessor interface {
    Sale(ctx context.Context, req *SaleRequest) (*SaleResponse, error)
}

// Payment service implements the interface
var _ PaymentProcessor = (*payment.PaymentService)(nil)
```

### Issue: Database connection pool exhausted

**Cause:** Too many concurrent operations

**Solution:** Limit concurrent operations

```go
import "golang.org/x/sync/semaphore"

// Limit to 10 concurrent payments
sem := semaphore.NewWeighted(10)

func processPayment(ctx context.Context) error {
    if err := sem.Acquire(ctx, 1); err != nil {
        return err
    }
    defer sem.Release(1)

    return services.Payment.Sale(ctx, req)
}
```

### Issue: Secret not found

**Cause:** Secret manager can't find secret file/key

**Solution:**
```bash
# Use absolute paths for local secrets
LOCAL_SECRETS_BASE_PATH=/absolute/path/to/secrets

# Or use environment-specific secret manager
SECRET_MANAGER=gcp
GCP_PROJECT_ID=your-project-id
```

### Issue: Cache not working

**Symptom:** High database load despite caches

**Diagnosis:**
```go
// Check cache stats
stats := merchantCache.Stats()
logger.Info("Cache performance",
    zap.Int("size", stats.Size),
    zap.Float64("hit_rate", stats.HitRate),
    zap.Int64("hits", stats.Hits),
    zap.Int64("misses", stats.Misses),
)

// Low hit rate? Increase TTL or max size:
merchantCache := merchantService.NewMerchantCredentialCache(
    10*time.Minute,  // Increase from 5m
    2000,           // Increase from 1000
    logger,
)
```

---

## Next Steps

- **[GETTING_STARTED.md](GETTING_STARTED.md)** - Microservice integration (recommended for production)
- **[API_SPECS.md](API_SPECS.md)** - Complete ConnectRPC API reference
- **[REACT_INTEGRATION.md](REACT_INTEGRATION.md)** - Frontend integration guide
- **[BROWSER_POST_FORM_SETUP.md](BROWSER_POST_FORM_SETUP.md)** - PCI-compliant tokenization
- **[TOKEN_GENERATION.md](TOKEN_GENERATION.md)** - JWT authentication guide
- **[DATABASE.md](../development/DATABASE.md)** - Complete database schema reference
- **[RATE_LIMITING.md](../development/RATE_LIMITING.md)** - Rate limiting architecture

---

**Questions?** Open an issue on [GitHub](https://github.com/kevin07696/payment-service/issues)
