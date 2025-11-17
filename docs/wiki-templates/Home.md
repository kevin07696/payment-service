# Payment Service Wiki

Welcome! This is your complete guide to integrating, developing, and deploying the payment microservice.

## 🚀 New Here? Start With These

| Guide | Time | What You'll Learn |
|-------|------|-------------------|
| **[Quick Start](Quick-Start)** | 5 min | Get the service running locally with Docker |
| **[EPX Credentials Guide](EPX-Credentials)** | 15 min | Obtain EPX API keys for sandbox and production |
| **[FAQ - Browser Post](FAQ#browser-post--callbacks)** | 10 min | How Browser Post callbacks work |

---

## 📚 Documentation by Topic

### Getting Started

**Setup & Configuration**
- **[Quick Start (5 Minutes)](Quick-Start)** - Get running with Docker
- **[Complete Setup Guide](DEVELOP)** - Detailed local development setup
- **[EPX Credentials](EPX-Credentials)** - How to get API keys from EPX

**First Integration**
- **[Your First Payment](Quick-Start#step-5-make-your-first-payment-browser-post)** - Test Browser Post flow
- **[Understanding Payment Flows](DATAFLOW)** - Browser Post, Server Post, ACH
- **[Authentication Setup](AUTH)** - Token-based multi-tenant auth

### Integration Guides

**Payment Methods**
- **[Browser Post Integration](DATAFLOW#browser-post-flow)** - PCI-compliant frontend tokenization
- **[Server Post Integration](DATAFLOW#server-post-flow)** - Backend API integration
- **[ACH Payments](DATAFLOW#ach-dataflow)** - Bank account processing
- **[Recurring Billing](DATAFLOW#subscription-billing-cron-job)** - Subscription management

**Advanced Features**
- **[Saved Payment Methods](DATAFLOW#saved-payment-methods-bric-storage)** - Card-on-file with BRIC Storage
- **[Webhooks](DATAFLOW#webhook-delivery)** - Event notifications with retries
- **[Idempotency](FAQ#how-does-idempotency-work-with-browser-post-callbacks)** - Safe retry patterns

### API Reference

- **[Complete API Specification](API-Specs)** - gRPC & REST endpoints
- **[EPX API Reference](EPX-API-REFERENCE)** - EPX gateway documentation
- **[Error Codes](API-Specs#error-codes)** - Response code meanings
- **[Database Schema](DATABASE)** - Tables, relationships, queries

### Architecture

- **[System Overview](DEVELOP#architecture)** - Hexagonal architecture
- **[Payment Dataflows](DATAFLOW)** - End-to-end transaction flows
- **[Database Design](DATABASE)** - Schema and migrations
- **[Security Model](AUTH)** - Authentication & authorization

### Testing

- **[Integration Test Strategy](INTEGRATION-TEST-STRATEGY)** - Test philosophy and coverage
- **[Running Tests](INTEGRATION-TEST-STRATEGY#running-integration-tests)** - How to run test suites
- **[Writing Tests](INTEGRATION-TEST-STRATEGY#test-structure)** - Best practices
- **[Test Card Numbers](FAQ#what-test-cards-work-in-epx-sandbox)** - EPX sandbox test data

### Deployment

- **[Google Cloud Run Setup](GCP-PRODUCTION-SETUP)** - Managed container deployment
- **[Docker Deployment](DEVELOP#docker-setup-recommended)** - Self-hosted containerization
- **[CI/CD Pipeline](CICD)** - Automated deployments
- **[Environment Configuration](EPX-Credentials#configuring-the-payment-service)** - Production setup

### Operations

- **[Troubleshooting Guide](Troubleshooting)** - Common issues & solutions
- **[FAQ](FAQ)** - Frequently asked questions
- **[Monitoring Setup](FAQ#what-monitoring-should-i-set-up)** - Prometheus metrics
- **[Database Migrations](FAQ#how-do-i-handle-database-migrations-in-production)** - Schema evolution

---

## 🔑 Key Concepts

### What is EPX?

**EPX** (formerly Element Payment Services, now part of North) is the payment gateway that processes credit card and ACH transactions. This service integrates with EPX to handle payments securely.

- **Documentation**: [developer.north.com](https://developer.north.com)
- **Get Credentials**: [EPX Credentials Guide](EPX-Credentials)

### What is a BRIC Token?

**BRIC** (Bank Routing Information Code) is EPX's tokenization system:

- **Financial BRIC**: Single-use token from transactions, used for refunds/voids
- **Storage BRIC**: Multi-use token for saved payment methods, never expires

**Benefit**: No raw card data stored (PCI-reduced scope)

### Browser Post vs Server Post

| Method | Use Case | PCI Scope | Card Data Flow |
|--------|----------|-----------|----------------|
| **Browser Post** | Direct customer payments | Reduced | User Browser → EPX → Callback |
| **Server Post** | Recurring/backend processing | Higher (if collecting cards) | Your Backend ↔ EPX API |

**Recommendation**: Use Browser Post for all direct customer payments (checkout pages)

### Payment Flow Types

1. **AUTH + CAPTURE** (Two-step): Authorize first, capture later
2. **SALE** (One-step): Authorize and capture immediately
3. **VOID**: Cancel before settlement
4. **REFUND**: Return funds after settlement

See [Payment Dataflows](DATAFLOW) for complete diagrams.

---

## 📖 Common Tasks

### I want to...

**Get Started**
- → **[Run the service locally](Quick-Start)** (5 min)
- → **[Get EPX credentials](EPX-Credentials)** (sandbox account)
- → **[Make my first payment](Quick-Start#step-5-make-your-first-payment-browser-post)**

**Integrate Payments**
- → **[Add Browser Post to my frontend](DATAFLOW#browser-post-flow)**
- → **[Process backend payments with Server Post](DATAFLOW#server-post-flow)**
- → **[Save payment methods for later](DATAFLOW#saved-payment-methods-bric-storage)**
- → **[Set up recurring billing](DATAFLOW#subscription-billing-cron-job)**

**Test & Debug**
- → **[Run integration tests](INTEGRATION-TEST-STRATEGY#running-integration-tests)**
- → **[Debug callback issues](FAQ#how-do-i-debug-callback-issues)**
- → **[Use test card numbers](FAQ#what-test-cards-work-in-epx-sandbox)**
- → **[Understand idempotency](FAQ#how-does-idempotency-work-with-browser-post-callbacks)**

**Deploy to Production**
- → **[Deploy to Google Cloud Run](GCP-PRODUCTION-SETUP)**
- → **[Configure production environment](EPX-Credentials#production-credentials-for-live-transactions)**
- → **[Set up monitoring](FAQ#what-monitoring-should-i-set-up)**
- → **[Handle database migrations](FAQ#how-do-i-handle-database-migrations-in-production)**

**Troubleshoot**
- → **[Common errors & solutions](Troubleshooting)**
- → **[FAQ - Browser Post callbacks](FAQ#browser-post--callbacks)**
- → **[EPX authentication failed](FAQ#epx-integration)**

---

## 🏗️ Architecture Overview

This service uses **hexagonal architecture** (ports & adapters):

```
┌─────────────────────────────────────────┐
│         gRPC/REST API Layer             │
│  Payment, Subscription, PaymentMethod   │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│        Business Logic Layer             │
│  Payment Service, Subscription Service  │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│          Domain Layer (Core)            │
│  Transaction, Subscription, BRIC Models │
└─────────────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│      Infrastructure Layer (Adapters)    │
│  EPX Gateway | PostgreSQL | Secrets     │
└─────────────────────────────────────────┘
```

**Benefits:**
- ✅ Testable (mock all external dependencies)
- ✅ Flexible (swap implementations easily)
- ✅ Maintainable (clear boundaries)

See [Development Guide](DEVELOP#architecture) for details.

---

## 🛡️ Security & Compliance

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

See [Security FAQs](FAQ#security--pci-compliance) for more.

---

## 🔗 External Resources

- **[EPX API Documentation](https://developer.north.com)** - Official EPX/North developer docs
- **[GitHub Repository](https://github.com/kevin07696/payment-service)** - Source code
- **[Report Issues](https://github.com/kevin07696/payment-service/issues)** - Bug reports & feature requests
- **[Changelog](https://github.com/kevin07696/payment-service/blob/main/CHANGELOG.md)** - Version history

---

## ❓ Need Help?

1. **Check [FAQ](FAQ)** - Common questions answered
2. **See [Troubleshooting](Troubleshooting)** - Known issues & solutions
3. **Open [GitHub Issue](https://github.com/kevin07696/payment-service/issues)** - Report bugs or ask questions

---

**Last Updated**: 2025-01-17
**Version**: 1.0.0
