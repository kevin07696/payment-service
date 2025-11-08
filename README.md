# Payment Microservice

A production-ready payment microservice built with **Go** and **gRPC**, integrating with **North Payment Gateway (EPX)** using clean **Hexagonal Architecture (Ports & Adapters)** pattern.

## 🎯 Features

- ✅ **Credit Card Payments**: One-time, auth/capture flows (Server Post & Browser Post)
- ✅ **Saved Payment Methods**: Storage BRIC conversion for card-on-file and recurring payments
- ✅ **Recurring Billing**: Subscription management with automatic cron billing
- ✅ **ACH Payments**: Bank transfers (checking/savings accounts)
- ✅ **Chargeback Management**: Automated polling from North API, local storage, webhook notifications
- ✅ **Webhook System**: Outbound webhooks with HMAC signatures, automatic retries
- 🚧 **Invoice Payments**: (planned)
- ✅ **PCI-Compliant**: Browser Post tokenization with BRIC tokens (frontend-to-backend)
- ✅ **Account Verification**: $0.00 verification with card networks for saved cards
- ✅ **Response Code Handling**: 40+ mapped codes with user-friendly messages
- ✅ **HMAC Authentication**: Secure API communication & webhook signatures
- ✅ **Database Migrations**: SQL-based schema management
- ✅ **Observability**: Prometheus metrics, health checks, structured logging
- ✅ **Comprehensive Testing**: Unit and integration test suites for EPX payment gateway

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     gRPC API Layer ✅                           │
│       Payment Handler ✅ | Subscription Handler ✅             │
└─────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────────┐
│                  Business Logic Layer ✅                        │
│       Payment Service ✅ | Subscription Service ✅             │
└─────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────────┐
│                     Domain Layer (Core) ✅                      │
│  ┌──────────────────────────────────────────────────────┐      │
│  │  Ports (Interfaces)                                  │      │
│  │  - PaymentService, SubscriptionService, Repos        │      │
│  │  - Logger, HTTPClient, CreditCardGateway, etc.      │      │
│  └──────────────────────────────────────────────────────┘      │
│  ┌──────────────────────────────────────────────────────┐      │
│  │  Models                                              │      │
│  │  - Transaction, Subscription, PaymentMethod          │      │
│  └──────────────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────────┐
│                   Infrastructure Layer ✅                       │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐   │
│  │  EPX Adapters  │  │   PostgreSQL   │  │     Logging    │   │
│  │ - Server Post ✅│  │ - Repos ✅     │  │ - Zap Logger ✅│   │
│  │ - Browser Post✅│  │ - SQLC ✅      │  │                │   │
│  │ - BRIC Store ✅│  │ - Pooling ✅   │  │                │   │
│  │ - Key Exch. ✅ │  │ - Migrations✅ │  │                │   │
│  └────────────────┘  └────────────────┘  └────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

**Legend:** ✅ Complete | 🚧 In Progress | Planned

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- PostgreSQL 15+
- protoc (Protocol Buffers compiler)

### Setup

```bash
# Clone the repository
git clone https://github.com/kevin07696/payment-service.git
cd payment-service

# Install dependencies
go mod download

# Configure environment
cp .env.example .env
# Edit .env with your database and North gateway credentials

# Run tests
go test ./... -cover

# Build server
go build -o bin/payment-server ./cmd/server

# Run server
./bin/payment-server
```

The server will start on `0.0.0.0:8080` for gRPC and `0.0.0.0:8081` for HTTP/cron endpoints.

### Docker Setup (Recommended)

The easiest way to run the entire stack (PostgreSQL + migrations + payment server):

```bash
# Copy environment variables template
cp .env.example .env

# Edit .env with your EPX and North credentials (if needed)
# nano .env

# Start PostgreSQL and payment server
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

Or using docker-compose directly:

```bash
# Copy environment file
cp .env.example .env

# Start all services (postgres + migrations + payment-server)
docker-compose up -d

# View logs
docker-compose logs -f payment-server

# Stop services
docker-compose down

# Clean up volumes
docker-compose down -v
```

Services will be available at:

- **gRPC API**: `localhost:8080`
  - Payment, Subscription, PaymentMethod, Agent, Chargeback services
- **HTTP Endpoints**: `http://localhost:8081`
  - **Browser Post Callback**:
    - `POST /api/v1/payments/browser-post/callback` - EPX redirect callback (transaction results)
  - **Cron Jobs**:
    - `POST /cron/process-billing` - Process recurring billing
    - `POST /cron/sync-disputes` - Sync chargebacks from North API
    - `GET /cron/health` - Health check
    - `GET /cron/stats` - Billing statistics
- **PostgreSQL**: `localhost:5432`

### Using the Makefile

```bash
make help              # Show all available commands
make build             # Build binary locally
make test              # Run tests
make test-cover        # Run tests with coverage report
make docker-build      # Build Docker image
make docker-up         # Start all services
make docker-down       # Stop all services
make proto             # Generate protobuf code
make sqlc              # Generate SQLC code
```

## 📦 Project Structure

Clean layered architecture (Handlers → Services → Adapters):

```
payment-service/
├── cmd/
│   └── server/              # gRPC/HTTP server entry point
├── internal/
│   ├── handlers/            # 🌐 Presentation Layer (gRPC/HTTP)
│   │   ├── payment/         # Payment API handlers
│   │   ├── subscription/    # Subscription API handlers
│   │   ├── payment_method/  # Payment method handlers
│   │   ├── agent/           # Multi-tenant agent handlers
│   │   ├── chargeback/      # Chargeback/dispute handlers
│   │   ├── cron/            # Cron job HTTP endpoints
│   │   └── webhook/         # Webhook delivery handlers
│   ├── services/            # 💼 Business Logic Layer
│   │   ├── payment/         # Payment processing
│   │   ├── subscription/    # Recurring billing
│   │   ├── payment_method/  # Payment method management
│   │   ├── agent/           # Multi-tenant agent service
│   │   ├── webhook/         # Webhook delivery service
│   │   └── ports/           # Service interfaces
│   ├── adapters/            # 🔌 Infrastructure Layer
│   │   ├── epx/             # EPX Gateway (Browser Post, Server Post)
│   │   ├── north/           # North Merchant Reporting (disputes)
│   │   ├── database/        # Database adapter
│   │   ├── secrets/         # Secret management (AWS/Vault/Local)
│   │   └── ports/           # Adapter interfaces
│   ├── domain/              # 📦 Domain Models (Core Entities)
│   │   ├── agent.go         # Multi-tenant agent
│   │   ├── chargeback.go    # Dispute/chargeback
│   │   ├── payment_method.go
│   │   ├── subscription.go
│   │   ├── transaction.go
│   │   └── errors.go
│   ├── db/
│   │   ├── migrations/      # SQL migration files (Goose)
│   │   ├── queries/         # SQL queries for SQLC
│   │   └── sqlc/            # Generated SQLC code
│   └── config/              # Configuration management
├── proto/               # Protocol Buffer Definitions
│   ├── payment/v1/
│   ├── subscription/v1/
│   ├── payment_method/v1/
│   ├── agent/v1/
│   └── chargeback/v1/
├── pkg/
│   ├── errors/              # Custom error types
│   ├── security/            # Logger, crypto utilities
│   └── observability/       # Metrics, tracing
├── test/
│   └── integration/         # Integration tests
│       └── testdb/          # Test database utilities
├── .env.example             # Environment variables template
├── docker-compose.yml       # Local development stack
├── docker-compose.test.yml  # Test database
├── Dockerfile               # Production image
├── Makefile                 # Build & dev commands
├── CHANGELOG.md             # Version history
├── DOCUMENTATION.md         # Complete documentation
└── README.md
```

## 🔧 Usage Example

### Using EPX Payment Adapters

```go
import (
    "github.com/kevin07696/payment-service/internal/adapters/epx"
    "github.com/kevin07696/payment-service/internal/adapters/ports"
    "github.com/kevin07696/payment-service/pkg/security"
)

// Setup logger and HTTP client
logger, _ := security.NewZapLoggerProduction()
httpClient := &http.Client{Timeout: 30 * time.Second}

// Create EPX Browser Post adapter for hosted payment pages
browserAdapter := epx.NewBrowserPostAdapter(
    "https://api.epxuap.com",
    httpClient,
    logger,
)

// Or create EPX Server Post adapter for direct API integration
serverAdapter := epx.NewServerPostAdapter(
    "https://api.epxuap.com",
    httpClient,
    logger,
)

// Use the adapter (example with Server Post)
req := &ports.ServerPostRequest{
    Amount:   decimal.NewFromFloat(100.00),
    Currency: "USD",
    Token:    "bric-token-from-browser-post",
    Capture:  true,
}

result, err := adapter.Authorize(context.Background(), req)
if err != nil {
    // Handle error - check if retriable
    if paymentErr, ok := err.(*pkgerrors.PaymentError); ok {
        if paymentErr.IsRetriable {
            // Retry logic
        }
    }
}

fmt.Printf("Transaction ID: %s\n", result.TransactionID)
fmt.Printf("Status: %s\n", result.Status)
```

### Complete Browser Post Flow Example

**Browser Post** is the recommended PCI-compliant approach where card data goes directly from the user's browser to EPX (never touching your backend). Here's the complete flow:

**1. Backend: Generate TAC Token**

```go
// Your backend generates a TAC token via Key Exchange API
tacResponse, err := keyExchangeAdapter.GenerateTAC(ctx, &ports.KeyExchangeRequest{
    Amount:      "99.99",
    TranNbr:     "TXN-12345",     // Your unique transaction ID
    TranGroup:   "SALE",
    RedirectURL: "http://localhost:8081/api/v1/payments/browser-post/callback",
})
// Returns: TAC token valid for 4 hours
```

**2. Backend: Build Form Data**

```go
// Construct form data for frontend
formData, err := browserPostAdapter.BuildFormData(
    tacResponse.TAC,
    "99.99",
    "TXN-12345",
    "SALE",
    "http://localhost:8081/api/v1/payments/browser-post/callback",
)
// Returns: PostURL, TAC, and other form fields
```

**3. Frontend: Render Payment Form**

```html
<!-- User's browser submits card data directly to EPX -->
<form method="POST" action="{{.PostURL}}">
    <input type="hidden" name="TAC" value="{{.TAC}}" />
    <input type="hidden" name="TRAN_NBR" value="TXN-12345" />
    <input type="hidden" name="AMOUNT" value="99.99" />
    <input type="hidden" name="TRAN_GROUP" value="SALE" />
    <input type="hidden" name="REDIRECT_URL" value="http://localhost:8081/api/v1/payments/browser-post/callback" />

    <!-- Optional: Enable payment method saving -->
    <input type="hidden" name="USER_DATA_1" value="save_payment_method=true" />
    <input type="hidden" name="USER_DATA_2" value="{{.CustomerID}}" />

    <input type="text" name="CARD_NBR" placeholder="Card Number" required />
    <input type="text" name="EXP_MONTH" placeholder="MM" required />
    <input type="text" name="EXP_YEAR" placeholder="YYYY" required />
    <input type="text" name="CVV" placeholder="CVV" required />

    <!-- Optional: For Account Verification (if saving payment method) -->
    <input type="text" name="FIRST_NAME" placeholder="First Name" />
    <input type="text" name="LAST_NAME" placeholder="Last Name" />
    <input type="text" name="ADDRESS" placeholder="Billing Address" />
    <input type="text" name="CITY" placeholder="City" />
    <input type="text" name="STATE" placeholder="State" />
    <input type="text" name="ZIP_CODE" placeholder="ZIP" />

    <label>
        <input type="checkbox" id="savePaymentMethod" /> Save for future use
    </label>

    <button type="submit">Pay $99.99</button>
</form>
```

**4. EPX: Process Payment**

- User's browser POSTs to EPX (card data never touches your server)
- EPX validates card, processes payment
- EPX redirects browser back to your REDIRECT_URL with results

**5. Backend: Callback Handler Receives Results**

```go
// Automatically handled by BrowserPostCallbackHandler
// File: internal/handlers/payment/browser_post_callback_handler.go

POST /api/v1/payments/browser-post/callback

// Received fields:
// - AUTH_GUID: Transaction token (Financial BRIC) for refunds/voids
// - AUTH_RESP: "00" = approved
// - AUTH_CODE: Bank authorization code
// - AUTH_CARD_TYPE, AUTH_AVS, AUTH_CVV2: Verification
// - TRAN_NBR, AMOUNT: Echo back your values
// - USER_DATA_1, USER_DATA_2: Custom data (save flag, customer_id)

// Handler Flow:
// 1. Parses response
// 2. Validates fields
// 3. Checks for duplicates (idempotency)
// 4. Stores transaction in database with Financial BRIC (AUTH_GUID)
// 5. If USER_DATA_1 contains "save_payment_method=true":
//    a. Converts Financial BRIC to Storage BRIC via EPX
//    b. For credit cards: EPX performs $0.00 Account Verification
//    c. Saves Storage BRIC to customer_payment_methods table
//    d. Storage BRIC never expires (use for recurring payments)
// 6. Renders HTML receipt page to user
```

**6. User: Sees Receipt Page**

- Success: Shows masked card, auth code, transaction ID
- Failure: Shows error message with retry option

**Key Benefits:**

- ✅ PCI-compliant (card data never hits your server)
- ✅ No PCI certification needed
- ✅ Simple integration
- ✅ AUTH_GUID stored for future refunds/voids
- ✅ Automatic duplicate detection

## 🧪 Testing

### Unit Tests

```bash
# Run all tests (unit + integration)
make test

# Run unit tests only (skip integration)
make test-unit

# Run tests with coverage
make test-cover

# Run specific adapter tests
go test -v ./internal/adapters/north
```

### Integration Tests

Integration tests verify the full stack with a real PostgreSQL database.

```bash
# Start test database
make test-db-up

# Run integration tests
make test-integration

# Run integration tests with coverage
make test-integration-cover

# Stop test database
make test-db-down
```

**What's tested:**

- Repository layer with real PostgreSQL
- Payment Service with database transactions
- Subscription Service with billing logic
- Idempotency key handling
- Transaction lifecycle (authorize, capture, void, refund)
- Subscription lifecycle (create, update, cancel, billing)

See [test/integration/README.md](test/integration/README.md) for detailed documentation.

### Test Coverage

- **EPX Adapter Unit Tests**: 13.5% (focused on business logic, not HTTP/API calls)
- **EPX Adapter Integration Tests**: 9 comprehensive tests covering all 7 transaction types
- **Chargeback Handler Tests**: 72.6%

**EPX Integration Tests:**

- ✅ Sale (CCE1) - Authorization + Capture
- ✅ Authorization Only (CCE2)
- ✅ Complete Auth → Capture flow
- ✅ Complete Sale → Refund flow
- ✅ Complete Sale → Void flow
- ✅ BRIC Storage (CCE8) tokenization
- ✅ Complete recurring payment flow
- ✅ Error handling (invalid cards, declines)
- ✅ Performance validation (response times)

See [TESTING.md](TESTING.md) for complete testing guide.

## 🏛️ Architecture Benefits

### Dependency Injection with Interfaces

All dependencies are injected through interfaces, enabling:

✅ **Easy Testing**: Mock all external dependencies
✅ **Flexibility**: Swap implementations without code changes
✅ **Maintainability**: Clear boundaries and responsibilities
✅ **Team Velocity**: Parallel development on interfaces

### Example: Swapping Loggers

```go
// Development: verbose logging
devLogger, _ := security.NewZapLoggerDevelopment()
adapter := epx.NewServerPostAdapter(url, httpClient, devLogger)

// Production: structured JSON logging
prodLogger, _ := security.NewZapLoggerProduction()
adapter := epx.NewServerPostAdapter(url, httpClient, prodLogger)

// Testing: mock logger
mockLogger := mocks.NewMockLogger()
adapter := epx.NewServerPostAdapter(url, httpClient, mockLogger)

// Custom: your own implementation
customLogger := MyLogger{}
adapter := epx.NewServerPostAdapter(url, httpClient, customLogger)
```

See [docs/ARCHITECTURE_BENEFITS.md](docs/ARCHITECTURE_BENEFITS.md) for detailed benefits and examples.

## 📊 Response Codes

The system handles 40+ response codes with user-friendly messages:

| Code | Display | Category | Retriable | User Message |
|------|---------|----------|-----------|--------------|
| 00 | APPROVAL | Approved | No | Payment successful |
| 51 | INSUFF FUNDS | Insufficient Funds | Yes | Insufficient funds. Please use a different payment method. |
| 54 | EXP CARD | Expired Card | Yes | Your card has expired. |
| 82 | CVV ERROR | Invalid Card | Yes | Incorrect CVV. Please check the security code. |
| 59 | SUSPECTED FRAUD | Fraud | No | Transaction declined for security reasons. |
| 96 | SYSTEM ERROR | System Error | Yes | System error. Please try again. |

## 🔐 Security

- **PCI-Reduced Scope**: Backend only handles BRIC tokens, never raw card data
- **HMAC-SHA256 Authentication**: All North API calls are signed
- **TLS 1.3**: Encrypted communication
- **Tokenization**: Cards tokenized via Browser Post (frontend)

## 📊 Observability

### Prometheus Metrics

The service exposes Prometheus metrics on port 9090:

```bash
curl http://localhost:9090/metrics
```

**Available Metrics:**

- `grpc_requests_total{method, status}` - Total gRPC requests
- `grpc_request_duration_seconds{method}` - Request duration histogram
- `grpc_requests_in_flight` - Current concurrent requests

### Health Checks

**Liveness Probe:**

```bash
curl http://localhost:9090/health
```

Returns JSON with database connectivity status:

```json
{
  "status": "healthy",
  "timestamp": "2025-10-20T12:00:00Z",
  "checks": {
    "database": "healthy"
  }
}
```

**Readiness Probe:**

```bash
curl http://localhost:9090/ready
```

### Database Migrations

We use [Goose](https://github.com/pressly/goose) for database migrations.

**Using Makefile (recommended):**

```bash
# Run all pending migrations
make migrate-up

# Check migration status
make migrate-status

# Rollback last migration
make migrate-down

# Create new migration
make migrate-create NAME=add_users_table
```

**Using goose CLI directly:**

```bash
# Install goose
go install github.com/pressly/goose/v3/cmd/goose@latest

# Run migrations
goose -dir internal/db/migrations postgres "host=localhost port=5432 user=postgres password=postgres dbname=payment_service sslmode=disable" up

# Check status
goose -dir internal/db/migrations postgres "host=localhost port=5432 user=postgres password=postgres dbname=payment_service sslmode=disable" status

# Create new migration
goose -dir internal/db/migrations create add_users_table sql
```

**Docker:** Migrations run automatically when using `docker-compose up`

## 📝 API Endpoints Implemented

### Server Post API ✅

- `Authorize()` - Authorize payment with token
- `Capture()` - Capture authorized payment
- `Sale()` - One-step authorize and capture
- `Void()` - Void transaction
- `Refund()` - Refund payment

### Subscription Service ✅

- `CreateSubscription()` - Create new recurring subscription
- `UpdateSubscription()` - Update subscription details
- `CancelSubscription()` - Cancel subscription
- `PauseSubscription()` - Pause subscription billing
- `ResumeSubscription()` - Resume paused subscription
- `GetSubscription()` - Get subscription details
- `ListSubscriptions()` - List customer subscriptions

### Payment Method Service ✅

- `SavePaymentMethod()` - Save payment method with existing token
- `ConvertFinancialBRICToStorageBRIC()` - Convert Financial BRIC to Storage BRIC
  - Credit cards: Triggers $0.00 Account Verification with card networks
  - ACH: Validates routing number
  - Returns Storage BRIC (never expires) for recurring payments
- `GetPaymentMethod()` - Get payment method details
- `ListPaymentMethods()` - List customer payment methods
- `UpdatePaymentMethodStatus()` - Activate/deactivate payment method
- `DeletePaymentMethod()` - Soft delete payment method (90-day retention)
- `SetDefaultPaymentMethod()` - Mark payment method as default
- `VerifyACHAccount()` - Send pre-note for ACH verification

### ACH Payments (via Server Post) ✅

- ACH debit transactions (checking/savings)
- ACH credit transactions (refunds)
- Bank account verification
- Pre-note verification for new accounts

### Browser Post API ✅

- `BuildFormData()` - Generate payment form data with TAC token
- `ParseRedirectResponse()` - Process payment callback from EPX
- `ValidateResponseMAC()` - Validate response signature
- Frontend tokenization for PCI compliance

**REDIRECT_URL Configuration:**
When configuring your EPX Browser Post credentials, provide this URL where EPX will redirect after processing:

- **Local Development**: `http://localhost:8081/api/v1/payments/browser-post/callback`
- **Production**: `https://yourdomain.com/api/v1/payments/browser-post/callback`

The callback endpoint:

1. Receives POST redirect from EPX with transaction results
2. Parses and validates the response
3. Stores transaction in database (including AUTH_GUID for refunds)
4. Displays HTML receipt page to user
5. Handles duplicate callbacks (PRG pattern)

## 🛠️ Development

### Quality Assurance

```bash
# Run linters
go vet ./...

# Check for common issues
staticcheck ./...

# Format code
go fmt ./...

# Build verification
go build ./...
```

### Adding a New Adapter

1. Define the port interface in `internal/adapters/ports/` (for adapters) or `internal/services/ports/` (for services)
2. Create implementation in `internal/adapters/{vendor}/`
3. Inject dependencies through constructor
4. Write unit tests with mocks
5. Achieve >80% test coverage

Example:

```go
// 1. Define port in internal/adapters/ports/
package ports

type MyGateway interface {
    Process(ctx context.Context, req *Request) (*Result, error)
}

// 2. Create adapter in internal/adapters/myvendor/
package myvendor

import "github.com/kevin07696/payment-service/internal/adapters/ports"

type MyAdapter struct {
    httpClient ports.HTTPClient
    logger     ports.Logger
}

func NewMyAdapter(httpClient ports.HTTPClient, logger ports.Logger) ports.MyGateway {
    return &MyAdapter{httpClient: httpClient, logger: logger}
}

// 3. Implement interface
func (a *MyAdapter) Process(ctx context.Context, req *Request) (*Result, error) {
    // Implementation
}

// 4. Write tests
func TestMyAdapter_Process(t *testing.T) {
    mockHTTP := mocks.NewMockHTTPClient(...)
    mockLogger := mocks.NewMockLogger()
    adapter := NewMyAdapter(mockHTTP, mockLogger)
    // Test cases
}
```

## 📚 Documentation

**[DOCUMENTATION.md](DOCUMENTATION.md)** - **Complete Guide (START HERE)**

Comprehensive documentation covering:

- Quick Start & Setup
- Architecture & Design Patterns
- Frontend & Backend Integration
- North Gateway APIs
- Chargeback Management (READ-ONLY)
- Webhook System
- Testing & Deployment
- API Reference
- Troubleshooting

**[CHANGELOG.md](CHANGELOG.md)** - Version history and changes

## 🗺️ Roadmap

### Phase 1: Foundation ✅

- [x] Project structure
- [x] Domain models
- [x] Port interfaces
- [x] HMAC authentication
- [x] Response code mapping
- [x] Custom Pay adapter
- [x] Testing infrastructure

### Phase 2: Business Logic ✅

- [x] Payment service
- [x] Subscription service
- [x] Idempotency middleware

### Phase 3: Data Layer ✅

- [x] PostgreSQL repositories
- [x] Database migrations with Goose
- [x] Audit logging schema

### Phase 4: API Layer ✅

- [x] gRPC proto definitions
- [x] gRPC service implementation
- [x] gRPC server with interceptors

### Phase 5: Observability ✅

- [x] Prometheus metrics
- [x] Health checks
- [ ] OpenTelemetry tracing (optional)

### Phase 6: Deployment ✅

- [x] Docker containerization
- [x] Docker Compose orchestration
- [x] Automated migrations on startup
- [ ] Kubernetes manifests (optional)
- [ ] CI/CD pipeline (optional)

### Phase 7: Payment Adapters ✅

- [x] EPX Server Post adapter (card & ACH payments)
- [x] EPX Browser Post adapter (PCI-compliant tokenization)
- [x] EPX BRIC Storage adapter (Storage BRIC conversion)
- [x] EPX Key Exchange adapter (credential management)
- [x] North Merchant Reporting adapter (read-only disputes)
- [x] Webhook delivery system with retries
- [x] Payment Method Service (saved payment methods)
- [x] Storage BRIC conversion with Account Verification
- [x] Auto-save payment methods in Browser Post callback

### Phase 8: Testing & Integration 🚧

- [x] Integration tests with PostgreSQL
- [ ] Integration tests with North sandbox (requires credentials)
- [ ] End-to-end gRPC tests
- [ ] Load testing

## 🤝 Contributing

1. Follow hexagonal architecture principles
2. Use dependency injection for all external dependencies
3. Write tests with >80% coverage
4. Document public APIs
5. Update CHANGELOG.md

## 📄 License

[License Type] - See LICENSE file for details

## 📞 Contact

Kevin Lam - [@kevin07696](https://github.com/kevin07696)

Project Link: [https://github.com/kevin07696/payment-service](https://github.com/kevin07696/payment-service)

---

**Built with ❤️ using Go, Clean Architecture, and TDD**
