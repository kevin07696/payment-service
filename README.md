# Payment Microservice

A production-ready payment microservice built with **Go** and **gRPC**, integrating with **North Payment Gateway (EPX)** using clean **Hexagonal Architecture (Ports & Adapters)** pattern.

## 🎯 Features

- ✅ **Credit Card Payments**: One-time, auth/capture flows
- ✅ **Recurring Billing**: Subscription management
- 🚧 **ACH Payments**: Bank transfers (in progress)
- 🚧 **Invoice Payments**: (planned)
- ✅ **PCI-Compliant**: Tokenization with BRIC tokens
- ✅ **Response Code Handling**: 40+ mapped codes with user-friendly messages
- ✅ **HMAC Authentication**: Secure API communication
- ✅ **Database Migrations**: Goose-based schema management
- ✅ **Observability**: Prometheus metrics & health checks
- ✅ **Comprehensive Testing**: 85.7% test coverage on adapters

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
│  │ North Adapters │  │   PostgreSQL   │  │     Logging    │   │
│  │ - Custom Pay ✅│  │ - Repos ✅     │  │ - Zap Logger ✅│   │
│  │ - Recurring ✅ │  │ - SQLC ✅      │  │                │   │
│  │ - ACH 🚧       │  │ - Pooling ✅   │  │                │   │
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

The server will start on `0.0.0.0:50051` (configurable via `SERVER_PORT`).

### Docker Setup (Recommended)

The easiest way to run the entire stack:

```bash
# Start PostgreSQL and payment server
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

Or using docker-compose directly:

```bash
# Set your North gateway credentials
export NORTH_EPI_KEY="your_key_here"
export NORTH_USERNAME="your_username"

# Start services
docker-compose up -d

# View logs
docker-compose logs -f payment-server

# Stop services
docker-compose down
```

Services will be available at:
- **gRPC API**: `localhost:50051`
- **Prometheus Metrics**: `http://localhost:9090/metrics`
- **Health Check**: `http://localhost:9090/health`
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

```
payment-service/
├── cmd/
│   ├── server/              # gRPC server entry point
│   └── migrate/             # Database migration CLI
├── internal/
│   ├── domain/
│   │   ├── models/          # Domain entities (Transaction, Subscription, etc.)
│   │   └── ports/           # Interface contracts (Gateway, Logger, HTTPClient)
│   ├── adapters/
│   │   ├── north/           # North payment gateway implementations
│   │   └── postgres/        # PostgreSQL repository implementations
│   ├── api/grpc/
│   │   ├── payment/         # Payment gRPC handlers
│   │   └── subscription/    # Subscription gRPC handlers
│   ├── services/
│   │   ├── payment/         # Payment business logic
│   │   └── subscription/    # Subscription business logic
│   ├── db/
│   │   ├── migrations/      # SQL migration files
│   │   ├── queries/         # SQL queries for SQLC
│   │   └── sqlc/            # Generated SQLC code
│   └── config/              # Configuration management
├── api/proto/
│   ├── payment/v1/          # Payment service protobuf definitions
│   └── subscription/v1/     # Subscription service protobuf definitions
├── pkg/
│   ├── errors/              # Custom error types
│   ├── security/            # Logger adapters, security utilities
│   └── observability/       # Metrics, health checks
├── test/
│   ├── mocks/               # Mock implementations for testing
│   └── integration/         # Integration tests with PostgreSQL
├── docs/                    # Architecture documentation
├── CHANGELOG.md             # Change history
├── SYSTEM_DESIGN.md         # System design document
└── README.md
```

## 🔧 Usage Example

### Creating a Custom Pay Adapter

```go
import (
    "github.com/kevin07696/payment-service/internal/adapters/north"
    "github.com/kevin07696/payment-service/internal/domain/ports"
    "github.com/kevin07696/payment-service/pkg/security"
)

// Setup
config := north.AuthConfig{
    EPIId:  "CUST_NBR-MERCH_NBR-DBA_NBR-TERMINAL_NBR",
    EPIKey: "your-secret-key",
}

logger, _ := security.NewZapLoggerProduction()
httpClient := &http.Client{Timeout: 30 * time.Second}

adapter := north.NewCustomPayAdapter(
    config,
    "https://api.epxuap.com",
    httpClient,
    logger,
)

// Authorize a payment
req := &ports.AuthorizeRequest{
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

- **North Adapters**: 85.7%
- **HMAC Authentication**: 100%
- **Response Code Mapper**: 100%
- **Integration Tests**: Repository, Payment Service, Subscription Service

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
adapter := north.NewCustomPayAdapter(config, url, httpClient, devLogger)

// Production: structured JSON logging
prodLogger, _ := security.NewZapLoggerProduction()
adapter := north.NewCustomPayAdapter(config, url, httpClient, prodLogger)

// Testing: mock logger
mockLogger := mocks.NewMockLogger()
adapter := north.NewCustomPayAdapter(config, url, httpClient, mockLogger)

// Custom: your own implementation
customLogger := MyLogger{}
adapter := north.NewCustomPayAdapter(config, url, httpClient, customLogger)
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

Run migrations manually:
```bash
# Build the migrate binary
go build -o bin/migrate ./cmd/migrate

# Run migrations
./bin/migrate up

# Check migration status
./bin/migrate status

# Rollback last migration
./bin/migrate down

# Create new migration
./bin/migrate create add_new_table sql
```

Migrations run automatically when using docker-compose.

## 📝 API Endpoints Implemented

### Custom Pay API ✅

- `POST /sale/{token}` - Authorize or sale with BRIC token
- `PUT /sale/{id}/capture` - Capture authorized payment
- `PUT /void/{id}` - Void transaction
- `POST /refund/{id}` - Refund payment
- `POST /avs` - Verify account

### Recurring Billing API ✅

- `POST /subscription` - Create new subscription
- `PUT /subscription/{id}` - Update subscription
- `POST /subscription/cancel` - Cancel subscription
- `POST /subscription/pause` - Pause subscription
- `POST /subscription/resume` - Resume subscription
- `GET /subscription/{id}` - Get subscription details
- `GET /subscription/list` - List customer subscriptions

### ACH API 🚧

- In progress

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

1. Define the port interface in `internal/domain/ports/`
2. Create implementation in `internal/adapters/{vendor}/`
3. Inject dependencies through constructor
4. Write unit tests with mocks
5. Achieve >80% test coverage

Example:

```go
// 1. Define port
type MyGateway interface {
    Process(ctx context.Context, req *Request) (*Result, error)
}

// 2. Create adapter
type MyAdapter struct {
    httpClient ports.HTTPClient
    logger     ports.Logger
}

func NewMyAdapter(httpClient ports.HTTPClient, logger ports.Logger) *MyAdapter {
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

- [SYSTEM_DESIGN.md](SYSTEM_DESIGN.md) - Comprehensive system design
- [docs/ARCHITECTURE_BENEFITS.md](docs/ARCHITECTURE_BENEFITS.md) - Ports & adapters benefits
- [CHANGELOG.md](CHANGELOG.md) - Version history and changes

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

### Phase 7: Remaining Adapters 🚧
- [x] North Recurring Billing adapter
- [ ] ACH adapter
- [ ] Browser Post adapter
- [ ] Webhook handler

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
