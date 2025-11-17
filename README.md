# Payment Microservice

A production-ready payment microservice built with **Go** and **gRPC**, integrating with **North Payment Gateway (EPX)** using clean **Hexagonal Architecture**.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> **📚 Full Documentation**: [Visit the Wiki](../../wiki) for complete guides, API reference, and integration tutorials.

---

## 🎯 Features

- ✅ **Credit Card Payments**: One-time, auth/capture, recurring (EPX Browser Post & Server Post)
- ✅ **ACH Payments**: Bank transfers (checking/savings accounts)
- ✅ **PCI-Compliant**: Browser Post tokenization (card data never touches backend)
- ✅ **Saved Payment Methods**: BRIC Storage for card-on-file and subscriptions
- ✅ **Recurring Billing**: Automated subscription management with cron billing
- ✅ **Idempotent**: Safe retries with duplicate prevention (database PRIMARY KEY)
- ✅ **Webhook System**: Outbound notifications with HMAC signatures and retries
- ✅ **Production-Ready**: Docker, CI/CD, monitoring, health checks

## 🚀 Quick Start (5 Minutes)

Get the service running locally with Docker:

```bash
# 1. Clone repository
git clone https://github.com/kevin07696/payment-service.git
cd payment-service

# 2. Configure environment
cp .env.example .env
# Edit .env with your EPX credentials (see wiki for how to get them)

# 3. Start services (PostgreSQL + Payment Server)
docker-compose up -d

# 4. Verify it's running
curl http://localhost:8081/cron/health
# Response: {"status":"healthy","database":"connected"}
```

**Services Running:**
- gRPC API: `localhost:8080`
- HTTP endpoints: `http://localhost:8081`
- PostgreSQL: `localhost:5432`
- Metrics: `http://localhost:9090/metrics`

**Next Steps:**
- **[Get EPX Credentials](../../wiki/EPX-Credentials)** - How to obtain API keys
- **[Complete Setup Guide](../../wiki/Quick-Start)** - Detailed configuration
- **[Your First Payment](../../wiki/Quick-Start#step-5-make-your-first-payment-browser-post)** - Test Browser Post flow

## 🏗️ Architecture

Clean hexagonal architecture with dependency injection:

```
┌─────────────────────────────────────┐
│       gRPC/REST API Layer           │
│  Payment | Subscription | Webhooks  │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│      Business Logic Layer           │
│  Payment Service | Domain Models    │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│    Infrastructure Layer             │
│  EPX Gateway | PostgreSQL | Secrets │
└─────────────────────────────────────┘
```

**Key Benefits:**
- ✅ **Testable**: Mock all external dependencies
- ✅ **Flexible**: Swap implementations without code changes
- ✅ **Maintainable**: Clear boundaries and responsibilities

**[Learn More](../../wiki/DEVELOP#architecture)** about the architecture and design patterns.

## 📚 Documentation

Comprehensive documentation is available in the **[GitHub Wiki](../../wiki)**:

### Getting Started
- **[Quick Start (5 min)](../../wiki/Quick-Start)** - Get running with Docker
- **[EPX Credentials Guide](../../wiki/EPX-Credentials)** - How to get API keys from EPX
- **[Complete Setup Guide](../../wiki/DEVELOP)** - Local development & production config

### Integration Guides
- **[Browser Post Flow](../../wiki/DATAFLOW#browser-post-flow)** - PCI-compliant frontend tokenization
- **[Server Post Flow](../../wiki/DATAFLOW#server-post-flow)** - Backend API integration
- **[ACH Payments](../../wiki/DATAFLOW#ach-dataflow)** - Bank account processing
- **[Recurring Billing](../../wiki/DATAFLOW#subscription-billing-cron-job)** - Subscription management
- **[Saved Payment Methods](../../wiki/DATAFLOW#saved-payment-methods-bric-storage)** - Card-on-file with BRIC Storage

### API & Reference
- **[API Specification](../../wiki/API-Specs)** - Complete gRPC & REST API reference
- **[EPX API Reference](../../wiki/EPX-API-REFERENCE)** - EPX gateway documentation
- **[Database Schema](../../wiki/DATABASE)** - Tables, relationships, migrations
- **[Error Codes](../../wiki/API-Specs#error-codes)** - Response code meanings

### Testing & Deployment
- **[Integration Test Strategy](../../wiki/INTEGRATION-TEST-STRATEGY)** - Test philosophy and coverage
- **[Running Tests](../../wiki/INTEGRATION-TEST-STRATEGY#running-integration-tests)** - How to run test suites
- **[Google Cloud Run Setup](../../wiki/GCP-PRODUCTION-SETUP)** - Production deployment
- **[CI/CD Pipeline](../../wiki/CICD)** - Automated deployments

### Operations
- **[FAQ](../../wiki/FAQ)** - Common questions (Browser Post callbacks, EPX setup, etc.)
- **[Troubleshooting](../../wiki/Troubleshooting)** - Common issues & solutions

## 🧪 Testing

### Run All Tests

```bash
# Unit tests
go test ./... -cover

# Integration tests (requires Docker)
docker-compose up -d
EPX_MAC_STAGING="$(cat secrets/epx/staging/mac_secret)" \
SERVICE_URL="http://localhost:8081" \
go test -tags=integration -v ./tests/integration/payment/ -timeout 15m
```

### Phase 1 Critical Business Logic Tests

5 critical tests covering highest-risk scenarios (p90-p99.9 likelihood × impact):

```bash
go test -tags=integration -v ./tests/integration/payment/ \
  -run "TestBrowserPostIdempotency|TestRefundAmountValidation|TestCaptureStateValidation|TestConcurrentOperationHandling|TestEPXDeclineCodeHandling" \
  -timeout 15m
```

**What's Tested:**
- ✅ Browser Post idempotency (database PRIMARY KEY prevents duplicates)
- ✅ Refund amount validation (prevents over-refunding)
- ✅ Capture state validation (cannot capture SALE transactions)
- ✅ Concurrent operation handling (race condition prevention)
- ✅ EPX decline code handling (insufficient funds, expired card, etc.)

**[Testing Documentation](../../wiki/INTEGRATION-TEST-STRATEGY)** - Complete test strategy and coverage.

## 🔐 Security & PCI Compliance

### PCI-Reduced Scope

When using **Browser Post**, your backend never touches raw card data:

✅ **What We Store**: BRIC tokens, transaction metadata
❌ **What We DON'T Store**: Card numbers, CVV codes

**Result**: SAQ A or SAQ A-EP compliance (simplified PCI requirements)

### Best Practices

- Use **Browser Post** for all direct customer payments
- Store `MAC_SECRET` in secret management service (AWS Secrets Manager, GCP Secret Manager)
- Never log card data
- Use HTTPS/TLS for all connections
- Rotate credentials every 3-6 months

**[Security Documentation](../../wiki/FAQ#security--pci-compliance)** - Complete security guide.

## 📊 Key Concepts

### What is EPX?

**EPX** (formerly Element Payment Services, now part of North) is the payment gateway that processes credit card and ACH transactions.

- **Get Credentials**: [EPX Credentials Guide](../../wiki/EPX-Credentials)
- **EPX Docs**: [developer.north.com](https://developer.north.com)

### What is a BRIC Token?

**BRIC** (Bank Routing Information Code) is EPX's tokenization system:

- **Financial BRIC**: Single-use token from transactions (used for refunds/voids)
- **Storage BRIC**: Multi-use token for saved payment methods (never expires)

**Benefit**: No raw card data stored (PCI-reduced scope)

### Browser Post vs Server Post

| Method | Use Case | PCI Scope | Card Data Flow |
|--------|----------|-----------|----------------|
| **Browser Post** | Direct customer payments | Reduced | User Browser → EPX → Callback |
| **Server Post** | Recurring/backend processing | Higher | Your Backend ↔ EPX API |

**Recommendation**: Use Browser Post for all direct customer payments (checkout pages).

**[Learn More](../../wiki/FAQ#browser-post--callbacks)** about Browser Post callbacks and how they work.

## 🛠️ Development

### Prerequisites

- Go 1.24+
- PostgreSQL 15+
- Docker (recommended)
- EPX merchant account ([Get Credentials](../../wiki/EPX-Credentials))

### Using Makefile

```bash
make help              # Show all available commands
make build             # Build binary locally
make test              # Run unit tests
make test-integration  # Run integration tests
make docker-up         # Start all services
make docker-down       # Stop all services
```

### Environment Configuration

Required environment variables (`.env` file):

```bash
# Database
DATABASE_URL=postgres://postgres:postgres@localhost:5432/payments?sslmode=disable

# EPX Credentials (from EPX merchant account)
EPX_CUST_NBR=9001
EPX_MERCH_NBR=900300
EPX_DBA_NBR=2
EPX_TERMINAL_NBR=77
EPX_MAC_SECRET=your-mac-secret-here

# EPX URLs (Sandbox)
EPX_API_URL=https://api.epxuap.com
EPX_BROWSER_POST_URL=https://services.epxuap.com/browserpost/

# Server Configuration
SERVICE_URL=http://localhost:8081
```

**[Complete Setup Guide](../../wiki/EPX-Credentials#configuring-the-payment-service)** - Detailed environment configuration.

## 🚀 Deployment

### Docker

```bash
# Build image
docker build -t payment-service:latest .

# Run with docker-compose
docker-compose up -d
```

### Google Cloud Run

One-click deployment to Google Cloud Run:

```bash
gcloud run deploy payment-service \
  --source . \
  --region us-central1 \
  --allow-unauthenticated
```

**[Production Deployment Guide](../../wiki/GCP-PRODUCTION-SETUP)** - Complete Google Cloud Run setup.

### Health Checks

```bash
# Liveness probe
curl http://localhost:9090/health

# Readiness probe
curl http://localhost:9090/ready

# Prometheus metrics
curl http://localhost:9090/metrics
```

## 📖 Common Tasks

**I want to...**

- → **[Run the service locally](../../wiki/Quick-Start)** (5 min Docker setup)
- → **[Get EPX credentials](../../wiki/EPX-Credentials)** (sandbox account)
- → **[Make my first payment](../../wiki/Quick-Start#step-5-make-your-first-payment-browser-post)** (Browser Post test)
- → **[Integrate Browser Post](../../wiki/DATAFLOW#browser-post-flow)** (PCI-compliant frontend)
- → **[Set up recurring billing](../../wiki/DATAFLOW#subscription-billing-cron-job)** (subscriptions)
- → **[Run integration tests](../../wiki/INTEGRATION-TEST-STRATEGY#running-integration-tests)** (test suite)
- → **[Deploy to production](../../wiki/GCP-PRODUCTION-SETUP)** (Google Cloud Run)
- → **[Debug callback issues](../../wiki/FAQ#how-do-i-debug-callback-issues)** (troubleshooting)

## 🤝 Contributing

1. Follow hexagonal architecture principles
2. Use dependency injection for all external dependencies
3. Write tests with >80% coverage
4. Document public APIs
5. Update [CHANGELOG.md](CHANGELOG.md)

**[Development Guide](../../wiki/DEVELOP)** - Complete development workflow.

## 📄 License

[MIT License](LICENSE) - See LICENSE file for details.

## 📞 Support

- **[FAQ](../../wiki/FAQ)** - Common questions answered
- **[Troubleshooting Guide](../../wiki/Troubleshooting)** - Known issues & solutions
- **[GitHub Issues](https://github.com/kevin07696/payment-service/issues)** - Report bugs or ask questions

## 🔗 Links

- **[Full Documentation (Wiki)](../../wiki)** - Complete guides and API reference
- **[EPX API Documentation](https://developer.north.com)** - Official EPX/North developer docs
- **[Changelog](CHANGELOG.md)** - Version history
- **[GitHub Repository](https://github.com/kevin07696/payment-service)** - Source code

---

**Built with ❤️ using Go, Clean Architecture, and TDD**

**[Get Started Now](../../wiki/Quick-Start)** | **[Get EPX Credentials](../../wiki/EPX-Credentials)** | **[View Wiki](../../wiki)**
