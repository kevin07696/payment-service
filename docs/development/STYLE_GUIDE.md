# Go Code Style Guide

**Target Audience:** Developers contributing to the payment service codebase
**Topic:** Code style conventions, architectural patterns, and best practices
**Goal:** Write consistent, maintainable, and idiomatic Go code

**Last Updated:** 2025-11-23

---

## Table of Contents

1. [Project Structure](#1-project-structure)
2. [Dependency Rules](#2-dependency-rules)
3. [Package Organization](#3-package-organization)
4. [Naming Conventions](#4-naming-conventions)
5. [Type Design](#5-type-design)
6. [Dependency Injection](#6-dependency-injection)
7. [Error Handling](#7-error-handling)
8. [Code Organization](#8-code-organization)
9. [Comments & Documentation](#9-comments--documentation)
10. [Performance Considerations](#10-performance-considerations)
11. [Testing](#11-testing)

---

## 1. Project Structure

### Architecture

This project follows **Hexagonal Architecture** (Ports & Adapters pattern):

```
internal/
├── domain/              # Pure business entities (no external dependencies)
│   ├── transaction.go
│   ├── merchant.go
│   └── errors.go
│
├── services/            # Business logic layer
│   ├── payment/
│   ├── subscription/
│   └── ports/          # Service interfaces (contracts)
│
├── adapters/            # External integrations
│   ├── database/       # PostgreSQL adapter
│   ├── epx/           # EPX payment gateway
│   ├── north/         # North API
│   └── ports/         # Adapter interfaces
│
├── handlers/           # HTTP/gRPC handlers (ConnectRPC)
│   ├── payment/
│   └── subscription/
│
├── db/                # Database layer
│   ├── migrations/    # Goose migrations
│   └── sqlc/         # Generated queries
│
├── converters/        # Type conversions between layers
└── util/             # Generic utilities
```

### Layer Responsibilities

**Domain Layer (`domain/`):**
- Pure business entities and value objects
- Business logic and invariants
- **NO external dependencies** (only standard library)
- **NO knowledge of adapters, handlers, or infrastructure**
- Domain entities should reflect **business reality**, not idealized abstractions
  - If EPX is your only payment gateway, EPX fields (`AuthGUID`, `AuthResp`) belong in domain
  - Don't hide adapter details in `map[string]string` if services need type-safe access
  - Abstract when you add a **second adapter**, not before (YAGNI principle)

**Service Layer (`services/`):**
- Orchestrates business workflows
- Implements use cases
- Depends on **ports** (interfaces), NOT concrete adapters
- Contains business rules that span multiple entities

**Adapter Layer (`adapters/`):**
- Implements **adapter ports** (interfaces)
- Handles external systems (database, APIs, etc.)
- Converts between external formats and domain types

**Handler Layer (`handlers/`):**
- HTTP/gRPC request handling
- Protocol-specific logic (ConnectRPC, REST)
- Calls **service ports** (interfaces)
- Handles authentication, validation, serialization

---

## 2. Dependency Rules

### Pragmatic Layered Dependencies

Follow these dependency rules strictly:

```
domain/ ──────────────────────────> (stdlib only)
   ↑
services/ports/ ──────────────────> domain/
adapters/ports/ ──────────────────> domain/
   ↑
services/ ────────────────────────> services/ports/, adapters/ports/, domain/
adapters/ ────────────────────────> adapters/ports/, domain/
   ↑
handlers/ ────────────────────────> services/ports/, domain/
```

### Rules

1. **Domain**: MUST NOT import anything except standard library
2. **Services**: MAY import adapter ports for dependency injection, MUST NOT import concrete adapters
3. **Adapters**: MAY import adapter ports and domain, MUST NOT import services
4. **Handlers**: MAY import service ports and domain, MUST NOT import concrete services or adapters

### Examples

✅ **Good:**
```go
// Service depends on port interface
package payment

import (
    "github.com/kevin07696/payment-service/internal/adapters/ports"  // Interface
    "github.com/kevin07696/payment-service/internal/domain"
)

type paymentService struct {
    serverPost ports.ServerPostAdapter  // Interface, not concrete type
}
```

❌ **Bad:**
```go
// Service depends on concrete adapter
package payment

import (
    "github.com/kevin07696/payment-service/internal/adapters/epx"  // Concrete adapter!
)

type paymentService struct {
    serverPost *epx.ServerPostAdapter  // Tight coupling!
}
```

### Don't Prematurely Abstract Adapters

Domain should reflect your **current business reality**, not idealized future scenarios.

✅ **Good - Pragmatic Domain (when EPX is your only gateway):**
```go
// domain/transaction.go
type Transaction struct {
    ID          string
    MerchantID  string
    AmountCents int64
    Status      TransactionStatus  // approved/declined (derived from AuthResp)
    Type        TransactionType    // sale/auth/capture/refund/void

    // EPX fields - your business runs on EPX, own it
    AuthGUID string   // EPX transaction identifier (BRIC)
    AuthResp *string  // EPX response code ("00" = approved)
    AuthCode *string  // EPX authorization code
    AuthAVS  *string  // EPX AVS response
    AuthCVV2 *string  // EPX CVV2 response
}

func (t *Transaction) IsApproved() bool {
    return t.AuthResp != nil && *t.AuthResp == "00"  // Clear EPX logic
}
```

**Why this is good:**
- Type-safe access to EPX fields
- Services get IDE autocomplete on `tx.AuthGUID`
- Business logic is explicit about EPX response codes
- No performance overhead from JSON parsing
- YAGNI: Don't abstract until you **actually need** multiple gateways

❌ **Bad - Premature Abstraction:**
```go
// domain/transaction.go
type Transaction struct {
    ID                   string
    MerchantID           string
    AmountCents          int64
    GatewayTransactionID string            // "Generic" but only used for EPX
    GatewayMetadata      map[string]string // EPX data hidden in JSON
}

func (t *Transaction) IsApproved() bool {
    // Lost type safety, added complexity, no actual benefit
    authResp := t.GatewayMetadata["epx_auth_resp"]
    return authResp == "00"
}

// Services now do string parsing everywhere
func (s *service) ProcessRefund(tx *Transaction) {
    authGUID := tx.GatewayMetadata["epx_auth_guid"]  // No autocomplete
    authCode := tx.GatewayMetadata["epx_auth_code"]  // String parsing
    // ...
}
```

**Why this is bad:**
- Lost type safety and IDE support
- JSON parsing overhead on every access
- Business logic still knows EPX codes anyway
- Added complexity with no actual benefit (you're not using multiple gateways)
- Harder to read and maintain

**When to abstract:**
- When you **actually add a second payment gateway** (Stripe, Adyen, etc.)
- Then extract common fields (`Status`, `TransactionID`) and use adapter pattern
- Keep gateway-specific fields in adapter-specific structs or metadata

**Rule of thumb:**
- One adapter = keep adapter fields in domain
- Two+ adapters = abstract common parts, keep specifics in metadata

---

## 3. Package Organization

### Layer-Based with Feature Sub-Packages

**Base structure:** Organize by architectural layer
**Sub-packages:** Use when feature has >5 related files

```go
// Main packages by layer
internal/services/payment/           # Payment service package
internal/services/subscription/      # Subscription service package

// Sub-packages when feature grows
internal/services/payment/
├── payment_service.go               # Main service
├── transaction_helper.go
├── idempotency.go
└── browser_post/                   # Sub-package (>5 files)
    ├── workflow.go
    ├── tac_handler.go
    └── callback_handler.go
```

### Package Naming Rules

1. **Lowercase, single word** - No underscores (`payment`, not `payment_service`)
2. **Singular nouns** - `payment`, not `payments` (exception: `handlers`)
3. **Short, descriptive** - Package name should be clear from context
4. **No redundancy** - Avoid `paymentservice.PaymentService` (stutter)

✅ **Good:**
```go
package payment  // internal/services/payment/

type Service struct {}  // payment.Service (clear from context)
```

❌ **Bad:**
```go
package payment_service  // Underscore

type PaymentService struct {}  // payment_service.PaymentService (stutter)
```

### When to Create a New Package

**Create new package when:**
- Feature has >5 related files
- Feature has distinct sub-features (e.g., `browser_post/`, `server_post/`)
- Code can be understood independently
- Has its own lifecycle and can evolve separately

**DON'T create new package when:**
- Only 1-3 files
- Tightly coupled to parent package
- Just for "organization" (keep related code together)

---

## 4. Naming Conventions

### Layer-Based Naming Strategy

Use different naming styles depending on the layer:

**1. Public APIs** (handlers/, services/ports/):
- Use descriptive, self-documenting names
- Follow Go conventions (no "Get" prefix, Is/Has/Can for booleans)
- Link to domain concepts in comments

**2. Domain Layer** (domain/):
- Type names: Descriptive (`TransactionTypeSale`, not `CCE1Sale`)
- Type values: Preserve external codes when relevant (`"CCE1"`, `"approved"`)
- Acronyms: Define with comment (BRIC, TAC, AVS)
- Methods: Follow Go conventions

**3. Adapter Layer** (adapters/epx/):
- Match external API exactly (`CustNbr`, `MerchNbr`, `TranType`)
- Comment with human-readable meaning

### Go Convention Rules

**Acronyms:** All caps
```go
// ✅ Good
type MerchantID string
type HTTPClient interface
type URLParser struct

// ❌ Bad
type MerchantId string
type HttpClient interface
type UrlParser struct
```

**Getters:** No "Get" prefix
```go
// ✅ Good
func (m *Merchant) ID() string
func (m *Merchant) Name() string

// ❌ Bad
func (m *Merchant) GetID() string
func (m *Merchant) GetName() string
```

**Setters:** Use "Set" prefix
```go
// ✅ Good
func (m *Merchant) SetName(name string)
```

**Booleans:** Prefix with Is/Has/Can/Should
```go
// ✅ Good
func (t *Transaction) IsApproved() bool
func (m *Merchant) HasCredentials() bool
func (u *User) CanRefund() bool

// ❌ Bad
func (t *Transaction) Approved() bool
func (m *Merchant) Credentials() bool
```

**Errors:** Prefix with Err or suffix with Error
```go
// ✅ Good
var ErrInvalidAmount = errors.New("invalid amount")
var ErrMerchantNotFound = errors.New("merchant not found")

// ❌ Bad
var InvalidAmount = errors.New("invalid amount")
var MerchantNotFoundError = errors.New("merchant not found")
```

**Interfaces:** Describe capability, no "I" prefix
```go
// ✅ Good
type PaymentService interface {}
type ServerPostAdapter interface {}

// ❌ Bad
type IPaymentService interface {}
type PaymentServiceInterface interface {}
```

### Naming Examples

✅ **Good:**
```go
// Public API
func (s *PaymentService) Sale(ctx context.Context, req *SaleRequest) error

// Domain
const TransactionTypeSale TransactionType = "CCE1"  // EPX Sale transaction

// Adapter
type ServerPostRequest struct {
    TranType string  // EPX transaction type code
}
```

❌ **Bad:**
```go
// Don't use "Get" prefix
func (m *Merchant) GetID() string

// Don't translate EPX codes at adapter layer
type ServerPostRequest struct {
    TransactionType string  // Should be TranType to match EPX
}

// Don't use unclear boolean names
func (t *Transaction) Approved() bool  // Should be IsApproved()
```

---

## 5. Type Design

### Interface at Boundaries Only

**Create interfaces for:**
- ✅ Service layer (ports)
- ✅ Adapter layer (ports)
- ✅ External integrations (EPX, database, secrets)

**DON'T create interfaces for:**
- ❌ Domain entities
- ❌ Request/Response types
- ❌ Internal helpers
- ❌ Value objects

### Examples

✅ **Good:**
```go
// Service port (interface)
type PaymentService interface {
    Sale(ctx context.Context, req *SaleRequest) (*domain.Transaction, error)
}

// Adapter port (interface)
type ServerPostAdapter interface {
    ProcessTransaction(ctx context.Context, req *ServerPostRequest) (*ServerPostResponse, error)
}

// Domain entity (concrete type, NO interface)
type Transaction struct {
    ID          string
    AmountCents int64
}

// Value object (concrete type, NO interface)
type Money struct {
    AmountCents int64
    Currency    string
}
```

❌ **Bad:**
```go
// Don't create interfaces for everything
type TransactionInterface interface {
    GetID() string
    GetAmount() int64
}

type MoneyInterface interface {
    Amount() int64
    Currency() string
}
```

### Return Concrete Types from Constructors

Constructors return concrete types, callers can treat as interface:

```go
// ✅ Good: Return concrete type
func NewPaymentService(
    queries sqlc.Querier,
    adapter ServerPostAdapter,
) *paymentService {
    return &paymentService{...}
}

// Caller can still use as interface
var svc ports.PaymentService = NewPaymentService(...)

// ❌ Less flexible: Return interface
func NewPaymentService(...) ports.PaymentService {
    return &paymentService{...}
}
```

---

## 6. Dependency Injection

### Constructor Injection Pattern

**Rule:** All dependencies passed to constructor

```go
// Constructor with dependencies
func NewPaymentService(
    queries sqlc.Querier,
    txManager database.TransactionManager,
    serverPost adapterports.ServerPostAdapter,
    secretManager adapterports.SecretManagerAdapter,
    merchantResolver *authorization.MerchantResolver,
    merchantCache *merchantsvc.MerchantCredentialCache,
    logger *zap.Logger,
) ports.PaymentService {
    return &paymentService{
        queries:       queries,
        txManager:     txManager,
        serverPost:    serverPost,
        secretManager: secretManager,
        // ...
        logger:        logger,
    }
}
```

### When to Use Config Struct

**Rule:** Use config struct when constructor has >7 parameters

```go
// Config struct for many dependencies
type PaymentServiceConfig struct {
    Queries           sqlc.Querier
    TxManager         database.TransactionManager
    ServerPost        adapterports.ServerPostAdapter
    SecretManager     adapterports.SecretManagerAdapter
    MerchantResolver  *authorization.MerchantResolver
    MerchantCache     *merchantsvc.MerchantCredentialCache
    Logger            *zap.Logger
    // ... more fields ...
}

func NewPaymentService(cfg PaymentServiceConfig) ports.PaymentService {
    // Validate required fields
    if cfg.Queries == nil {
        panic("queries required")
    }
    if cfg.Logger == nil {
        panic("logger required")
    }

    return &paymentService{
        queries:   cfg.Queries,
        txManager: cfg.TxManager,
        // ...
    }
}

// Usage
svc := NewPaymentService(PaymentServiceConfig{
    Queries:    queries,
    TxManager:  txManager,
    ServerPost: serverPost,
    Logger:     logger,
})
```

### Dependency Injection Rules

1. **≤7 params:** Use constructor injection
2. **>7 params:** Use config struct
3. **Required deps:** Pass to constructor (fail fast if missing)
4. **Optional deps:** Use config struct with defaults or nil checks
5. **Never use globals:** Always inject dependencies

---

## 7. Error Handling

### Hybrid Approach

Use different error patterns for different situations:

**1. Domain Errors (Sentinel Errors)**

```go
// domain/errors.go
var (
    ErrInvalidAmount      = errors.New("invalid amount")
    ErrMerchantNotFound   = errors.New("merchant not found")
    ErrInsufficientFunds  = errors.New("insufficient funds")
    ErrTransactionVoided  = errors.New("transaction already voided")
)

// Usage
if amount <= 0 {
    return nil, fmt.Errorf("%w: %d", domain.ErrInvalidAmount, amount)
}

// Caller can check
if errors.Is(err, domain.ErrInvalidAmount) {
    // Handle invalid amount
}
```

**2. Validation Errors (Custom Types)**

```go
// domain/errors.go
type ValidationError struct {
    Field   string
    Message string
    Value   interface{}
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// Usage
return nil, &ValidationError{
    Field:   "amount",
    Message: "must be positive",
    Value:   amount,
}

// Caller can extract details
var validationErr *ValidationError
if errors.As(err, &validationErr) {
    log.Printf("Field: %s, Message: %s, Value: %v",
        validationErr.Field,
        validationErr.Message,
        validationErr.Value)
}
```

**3. Infrastructure Errors (Wrap with Context)**

```go
// Always wrap infrastructure errors with context
if err := adapter.ProcessPayment(ctx, req); err != nil {
    return nil, fmt.Errorf("failed to process payment for merchant %s: %w", merchantID, err)
}
```

### Error Handling Rules

1. **Always use `%w`** to preserve error chain (for `errors.Is` and `errors.As`)
2. **Domain errors:** Use sentinel errors in `domain/errors.go`
3. **Validation errors:** Use custom types when you need structured context
4. **Infrastructure errors:** Wrap with context using `fmt.Errorf`
5. **Don't ignore errors:** Handle or propagate every error
6. **Log at origin:** Log errors where they occur, not at every layer

### Examples

✅ **Good:**
```go
// Domain error
if merchantID == "" {
    return nil, domain.ErrMerchantNotFound
}

// Wrap with context
tx, err := s.queries.GetTransaction(ctx, txID)
if err != nil {
    return nil, fmt.Errorf("failed to fetch transaction %s: %w", txID, err)
}

// Check error type
if errors.Is(err, domain.ErrInsufficientFunds) {
    s.logger.Warn("Insufficient funds", zap.String("merchant_id", merchantID))
    return nil, err
}
```

❌ **Bad:**
```go
// Don't use %v (loses error chain)
return nil, fmt.Errorf("failed to process: %v", err)

// Don't create new error strings for same concept
return nil, errors.New("merchant not found")  // Use sentinel error instead

// Don't ignore errors
tx, _ := s.queries.GetTransaction(ctx, txID)  // Ignoring error!
```

---

## 8. Code Organization

### Import Order

**Three groups, separated by blank lines:**

```go
import (
    // 1. Standard library (alphabetical)
    "context"
    "fmt"
    "time"

    // 2. Third-party dependencies (alphabetical)
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"
    "go.uber.org/zap"

    // 3. Internal packages (with aliases when helpful)
    "github.com/kevin07696/payment-service/internal/domain"
    adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
    merchantsvc "github.com/kevin07696/payment-service/internal/services/merchant"
)
```

### Import Aliases

Use aliases to:
- Avoid name collisions
- Clarify package purpose
- Make long import paths readable

```go
// ✅ Good: Clarify purpose
import (
    adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
    serviceports "github.com/kevin07696/payment-service/internal/services/ports"
)

// ❌ Bad: Unclear alias
import (
    ap "github.com/kevin07696/payment-service/internal/adapters/ports"
)
```

### File Organization

**Order within a file:**

1. Package declaration and imports
2. Constants
3. Variables
4. Type definitions
5. Constructor functions
6. Methods (grouped by receiver)
7. Private helper functions

```go
package payment

import (...)

// Constants
const (
    MaxRetries = 3
    Timeout    = 30 * time.Second
)

// Variables
var (
    ErrTimeout = errors.New("timeout")
)

// Type definitions
type paymentService struct {
    logger *zap.Logger
}

// Constructor
func NewPaymentService(logger *zap.Logger) *paymentService {
    return &paymentService{logger: logger}
}

// Methods
func (s *paymentService) Sale(ctx context.Context, req *SaleRequest) error {
    return s.processPayment(ctx, req)
}

// Private helpers
func (s *paymentService) processPayment(ctx context.Context, req *SaleRequest) error {
    // ...
}
```

---

## 9. Comments & Documentation

### Documentation Standards

**All exported types and functions MUST have doc comments:**

```go
// PaymentService handles payment processing operations including
// sales, authorizations, captures, refunds, and voids.
//
// All methods require authentication via JWT token in context.
// Methods are idempotent when IdempotencyKey is provided.
type PaymentService interface {
    // Sale combines authorization and capture in one operation.
    // This is the most common payment flow for immediate payment processing.
    //
    // Maps to EPX transaction type CCE1.
    Sale(ctx context.Context, req *SaleRequest) (*domain.Transaction, error)
}
```

### Comment Guidelines

**Explain WHY, not WHAT:**

✅ **Good:**
```go
// Field order optimized for memory alignment to reduce struct size
// from 120 bytes to 104 bytes (13% reduction)
type Transaction struct {
    // Timestamps first (24 bytes each - largest fields)
    CreatedAt time.Time
    UpdatedAt time.Time
    // ...
}

// Using sync.Pool to reduce allocations by 70% under high load
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}
```

❌ **Bad:**
```go
// ID is the transaction ID
ID string

// This function processes a payment
func processPayment(...)
```

### Comment Rules

1. **All exported symbols:** Must have doc comments
2. **Package comments:** Top of main file in package
3. **Complex logic:** Explain non-obvious decisions
4. **Performance:** Document optimizations and measurements
5. **TODOs:** Include issue number and context
6. **Links:** Reference related docs or external specs

### Examples

```go
// Package payment provides payment processing services including
// credit card and ACH transactions via the EPX payment gateway.
//
// All operations require merchant authentication and support idempotent
// retries through unique idempotency keys.
//
// For integration guide, see docs/integration/GETTING_STARTED.md
package payment

// TODO(#123): Add support for recurring billing schedule customization
// Currently only supports monthly billing cycles

// BRIC (Bank Routing Instrument Code) is EPX's tokenization system.
// Financial BRICs expire after 13 months.
// Storage BRICs never expire and are used for recurring payments.
//
// See docs/integration/EPX_API_REFERENCE.md for details.
type BRIC string
```

---

## 10. Performance Considerations

### When to Optimize

**Don't optimize prematurely, but document when you do:**

✅ **Optimize when:**
- Profiling shows bottleneck
- Performance requirement exists
- Working with high-volume operations

✅ **Document optimization:**
```go
// merchantCredentialCache reduces database load by 70% (measured under production traffic).
// Cache hit rate: 95%, average latency: 2ms vs 25ms for DB query.
// TTL set to 5 minutes to balance freshness with performance.
type MerchantCredentialCache struct {
    cache *sync.Map
    ttl   time.Duration
}
```

❌ **Don't:**
- Optimize without measuring
- Sacrifice readability for minimal gains
- Add complexity without clear benefit

### Memory Optimization

**Struct field alignment:**

Document when you optimize struct layout for memory:

```go
// Transaction represents a payment transaction.
// Field order optimized for memory alignment (largest to smallest):
// - time.Time (24 bytes) first
// - map (8 bytes header)
// - strings (16 bytes)
// - pointers (8 bytes)
// - int64 (8 bytes)
// This reduces struct size from 120 to 104 bytes (13% reduction).
type Transaction struct {
    // Timestamps (24 bytes each) - largest fields first
    CreatedAt time.Time
    UpdatedAt time.Time

    // Map (8 byte header + data)
    Metadata map[string]interface{}

    // Strings (16 bytes each on 64-bit)
    ID         string
    MerchantID string

    // Pointers (8 bytes each)
    CustomerID *string

    // int64 (8 bytes)
    AmountCents int64
}
```

### Concurrency Patterns

**Use sync.Pool for frequently allocated objects:**

```go
// bufferPool reduces allocations for XML marshaling (70% reduction measured).
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func marshalXML(v interface{}) ([]byte, error) {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()

    encoder := xml.NewEncoder(buf)
    if err := encoder.Encode(v); err != nil {
        return nil, err
    }

    return buf.Bytes(), nil
}
```

### Performance Rules

1. **Measure first:** Use pprof, benchmarks
2. **Document gains:** Include measurements in comments
3. **Trade-offs:** Note readability impact if significant
4. **Caching:** Document TTL, invalidation strategy, hit rate
5. **Pools:** Document allocation savings

---

## 11. Testing

**See:** [TESTING_GUIDE.md](TESTING_GUIDE.md) for comprehensive testing standards.

**Quick reference:**

- **Unit tests:** Test behavior, not implementation
- **Table-driven:** Use for multiple test cases
- **Mocking:** Use interfaces for testability
- **Naming:** `TestFunctionName_Scenario` or `TestFunctionName_ScenarioName`

---

## Checklist

Before submitting code, verify:

- [ ] **Dependencies:** Services depend on ports, not concrete adapters
- [ ] **Naming:** Follows Go conventions (acronyms, getters, booleans, errors)
- [ ] **Interfaces:** Only at layer boundaries
- [ ] **Errors:** Use `%w` for wrapping, sentinel errors for domain
- [ ] **Imports:** Ordered (stdlib → third-party → internal)
- [ ] **Comments:** All exported symbols documented
- [ ] **Tests:** Behavior tested, not implementation
- [ ] **Performance:** Optimizations measured and documented

---

## Related Documentation

- [TESTING_GUIDE.md](TESTING_GUIDE.md) - Testing standards (to be written)
- [TESTING_BEST_PRACTICES.md](TESTING_BEST_PRACTICES.md) - Error assertion guidelines
- [DEVELOP.md](DEVELOP.md) - Development workflow
- [Domain Refactoring Plan](../refactor/DOMAIN_REFACTORING_PLAN.md) - Future pure domain architecture

---

## Examples from Codebase

### Good Examples to Follow

**Service with dependency injection:**
- `internal/services/payment/payment_service.go`

**Pure domain entity:**
- `internal/domain/transaction.go`

**Adapter port interface:**
- `internal/adapters/ports/server_post.go`

**Error handling:**
- `internal/domain/errors.go`

### Anti-Patterns to Avoid

**DON'T:**
- Import concrete adapters in services
- Use "Get" prefix for getters
- Ignore errors
- Create interfaces for everything
- Optimize without measuring

---

**Questions?** See [DEVELOP.md](DEVELOP.md) or ask in team discussions.