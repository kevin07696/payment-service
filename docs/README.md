# Payment Microservice Documentation

Welcome to the payment microservice documentation! This directory contains comprehensive guides for developers, frontend engineers, and operators.

## Quick Links

- 📱 [Frontend Integration Guide](./FRONTEND_INTEGRATION.md) - **Start here for frontend developers**
- 🧪 [Local Testing Setup](./LOCAL_TESTING_SETUP.md) - **Start here for backend developers**
- 📋 [System Design](../SYSTEM_DESIGN.md) - Architecture and design decisions
- 📝 [Changelog](../CHANGELOG.md) - Version history and changes

## For Frontend Developers

### Getting Started

1. Read the [Frontend Integration Guide](./FRONTEND_INTEGRATION.md)
2. Understand the PCI-compliant tokenization flow
3. Implement North Browser Post SDK
4. Test with sandbox credentials

### Key Concepts

**Tokenization Flow:**
```
User enters card → North SDK tokenizes → Returns BRIC token → Send to backend
```

**Security:**
- ✅ Backend NEVER receives raw card data
- ✅ Frontend posts cards directly to North Gateway
- ✅ Only tokens are sent to your backend
- ✅ PCI DSS compliance maintained

### API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/payment/authorize` | POST | Authorize payment with token |
| `/api/payment/capture` | POST | Capture authorized payment |
| `/api/payment/void` | POST | Void transaction |
| `/api/payment/refund` | POST | Refund transaction |
| `/api/subscription/create` | POST | Create recurring subscription |

See [Frontend Integration Guide](./FRONTEND_INTEGRATION.md) for detailed API documentation.

## For Backend Developers

### Getting Started

1. Read the [Local Testing Setup](./LOCAL_TESTING_SETUP.md)
2. Install Docker and Go
3. Start test database: `docker compose -f docker-compose.test.yml up -d`
4. Run tests: `make test`
5. Start server: `make run`

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     gRPC API Layer                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Payment    │  │ Subscription │  │     ACH      │         │
│  │   Handlers   │  │   Handlers   │  │   Handlers   │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────┴────────────────────────────────────┐
│                   Service Layer (Business Logic)                │
│  - Payment Service      - Subscription Service                  │
│  - Idempotency          - Transaction Management                │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────┴────────────────────────────────────┐
│                   Infrastructure Layer                          │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐   │
│  │ North Adapters │  │   PostgreSQL   │  │  Observability │   │
│  │ - BrowserPost  │  │  Repositories  │  │ - Prometheus   │   │
│  │ - Recurring    │  │  - Transactions│  │ - Health       │   │
│  │ - ACH          │  │  - Subscriptions│  │ - Logging      │   │
│  └────────────────┘  └────────────────┘  └────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Testing Strategy

**Unit Tests (Fast, No Database)**
```bash
make test-unit
# 159 tests covering:
# - North adapters (107 tests)
# - gRPC handlers (27 tests)
# - Service layer (25 tests)
```

**Integration Tests (With Database)**
```bash
make test-integration
# Tests covering:
# - Repository CRUD operations
# - Service layer with real DB
# - Transaction handling
# - Subscription billing
```

### Development Workflow

```bash
# 1. Start dependencies
docker compose -f docker-compose.test.yml up -d

# 2. Make changes
vim internal/services/payment/payment_service.go

# 3. Run tests
make test

# 4. Test locally
make run

# 5. Test with grpcurl
grpcurl -plaintext -d '{...}' localhost:8080 payment.v1.PaymentService/Authorize

# 6. Check metrics
curl http://localhost:9090/metrics
```

## For DevOps/SRE

### Deployment

**Prerequisites:**
- Kubernetes cluster
- PostgreSQL database
- North gateway credentials
- TLS certificates

**Steps:**
1. Build Docker image: `make docker-build`
2. Deploy to Kubernetes: `kubectl apply -f k8s/`
3. Run migrations: `kubectl exec -it payment-pod -- /app/migrate up`
4. Verify health: `curl https://api.example.com/health`

### Monitoring

**Health Checks:**
```bash
# Liveness probe
curl http://localhost:9090/health

# Expected: {"status":"healthy","database":"connected"}
```

**Metrics:**
```bash
# Prometheus metrics
curl http://localhost:9090/metrics

# Key metrics:
# - payment_requests_total
# - payment_request_duration_seconds
# - subscription_billing_total
# - database_connections_active
```

**Alerts:**
- Payment failure rate > 5%
- Database connection pool exhausted
- Response time > 500ms (p95)
- Health check failures

### Scaling

**Horizontal Scaling:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: payment-service
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: payment-service
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

**Database:**
- Use connection pooling (configured: 25 max connections)
- Read replicas for query operations
- Regular VACUUM and ANALYZE

## Project Structure

```
payment-microservice/
├── api/proto/                    # Protocol buffer definitions
│   ├── payment/v1/              # Payment service protos
│   └── subscription/v1/         # Subscription service protos
├── cmd/
│   ├── server/                  # Main server application
│   └── migrate/                 # Migration tool
├── internal/
│   ├── adapters/
│   │   ├── north/              # North gateway adapters
│   │   └── postgres/           # PostgreSQL repositories
│   ├── api/grpc/               # gRPC handlers
│   ├── services/               # Business logic
│   ├── domain/                 # Domain models & ports
│   └── db/                     # Database migrations & SQLC
├── test/
│   ├── integration/            # Integration tests
│   └── mocks/                  # Mock implementations
├── docs/                        # 📚 Documentation (you are here)
│   ├── README.md               # This file
│   ├── FRONTEND_INTEGRATION.md # Frontend guide
│   └── LOCAL_TESTING_SETUP.md  # Testing guide
├── Makefile                     # Build commands
├── docker-compose.yml           # Development environment
└── docker-compose.test.yml      # Test environment
```

## Key Features

### ✅ Implemented

- **Payment Operations**
  - Authorize payment with tokenized card
  - Capture authorized payment
  - Void transaction
  - Refund transaction
  - Idempotency support

- **Subscriptions**
  - Create recurring subscription
  - Update subscription (amount, frequency)
  - Cancel subscription
  - Pause/resume subscription
  - Automatic billing processing

- **ACH Payments**
  - Process bank transfers
  - Verify bank accounts
  - Refund ACH transactions

- **Security**
  - PCI-compliant tokenization (Browser Post)
  - HMAC-SHA256 authentication
  - TLS 1.2+ encryption
  - Idempotency keys

- **Observability**
  - Prometheus metrics
  - Structured logging (zap)
  - Health checks
  - gRPC interceptors

- **Testing**
  - 159 unit tests
  - Integration tests with PostgreSQL
  - Mock-based testing
  - 78-89% code coverage

### 🚧 Future Enhancements

- Webhooks (async payment notifications)
- Invoice API integration
- 3D Secure support
- Rate limiting middleware
- Circuit breaker pattern
- GraphQL API layer

## Environment Variables

Required environment variables:

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=payment_user
DB_PASSWORD=payment_pass
DB_NAME=payment_service
DB_MAX_CONNS=25

# Server
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_METRICS_PORT=9090

# North Gateway
GATEWAY_BASE_URL=https://secure.epxuap.com
GATEWAY_USERNAME=your-epi-id
GATEWAY_EPI_KEY=your-epi-key

# Logging
LOG_LEVEL=info
LOG_DEVELOPMENT=false
```

## Common Tasks

### Add New Payment Method

1. Define port interface in `internal/domain/ports/`
2. Implement adapter in `internal/adapters/north/`
3. Write tests in `internal/adapters/north/*_test.go`
4. Update service layer to use new adapter
5. Add gRPC endpoint if needed
6. Update documentation

### Add New Field to Transaction

1. Update protobuf: `api/proto/payment/v1/payment.proto`
2. Generate code: `make proto`
3. Update database migration: `internal/db/migrations/`
4. Update SQLC queries: `internal/db/queries/`
5. Generate SQLC code: `make sqlc`
6. Update repository layer
7. Update service layer
8. Write tests

### Deploy New Version

1. Update version in `CHANGELOG.md`
2. Run tests: `make test`
3. Build Docker image: `make docker-build`
4. Tag image: `docker tag payment-service:latest payment-service:v1.2.3`
5. Push image: `docker push payment-service:v1.2.3`
6. Update Kubernetes deployment: `kubectl set image deployment/payment-service payment-service=payment-service:v1.2.3`
7. Monitor rollout: `kubectl rollout status deployment/payment-service`
8. Verify health: `curl https://api.example.com/health`

## Support

### Getting Help

- **Documentation Issues:** Open issue on GitHub
- **Frontend Integration:** See [FRONTEND_INTEGRATION.md](./FRONTEND_INTEGRATION.md)
- **Testing Issues:** See [LOCAL_TESTING_SETUP.md](./LOCAL_TESTING_SETUP.md)
- **North Gateway:** Contact North support
- **System Design Questions:** See [SYSTEM_DESIGN.md](../SYSTEM_DESIGN.md)

### Useful Commands

```bash
# Quick reference
make help                    # Show all available commands
make test                    # Run all tests
make run                     # Start server locally
make docker-build            # Build Docker image
grpcurl -plaintext localhost:8080 list  # List gRPC services
```

## License

Copyright © 2025. All rights reserved.

## Version

Current version: **v0.1.0-alpha**

Last updated: **2025-10-20**
