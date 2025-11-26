# Used Functionality Report

**Generated:** 2025-11-26
**Purpose:** Document all ACTIVE functionality in the payments codebase

---

## 1. Server Entry Points

### Main Server (`cmd/server/main.go`)
The server runs two HTTP servers:

| Port | Purpose | Protocol |
|------|---------|----------|
| 8080 | ConnectRPC API | HTTP/2 + gRPC-Web |
| 8081 | Cron/Internal endpoints | HTTP/1.1 |

---

## 2. ConnectRPC Services (Port 8080)

### Payment Service (`internal/handlers/payment/payment_handler_connect.go`)
| Method | Description | Used By |
|--------|-------------|---------|
| `Authorize` | Hold funds without capturing | API clients |
| `Capture` | Complete authorized payment | API clients |
| `Sale` | Authorize + Capture in one call | API clients |
| `Void` | Cancel authorized/captured payment | API clients |
| `Refund` | Return funds to customer | API clients |
| `GetTransaction` | Retrieve transaction details | API clients |
| `ListTransactions` | List transactions with filters | API clients |

**Service Layer:** `internal/services/payment/payment_service.go`
**Adapter:** `internal/adapters/epx/server_post_adapter.go` (EPX gateway)

### Payment Method Service (`internal/handlers/payment_method/payment_method_handler_connect.go`)
| Method | Description | Used By |
|--------|-------------|---------|
| `GetPaymentMethod` | Retrieve payment method | API clients |
| `ListPaymentMethods` | List customer's payment methods | API clients |
| `UpdatePaymentMethodStatus` | Activate/deactivate | API clients |
| `DeletePaymentMethod` | Soft delete (90-day retention) | API clients |
| `SetDefaultPaymentMethod` | Mark as default | API clients |
| `VerifyACHAccount` | Send pre-note verification | API clients |
| `StoreACHAccount` | Store ACH + auto-verify | API clients |
| `UpdatePaymentMethod` | Update billing info (not implemented) | - |

**Service Layer:** `internal/services/payment_method/payment_method_service.go`
**Adapter:** `internal/adapters/epx/bric_adapter.go` (BRIC tokenization)

### Subscription Service (`internal/handlers/subscription/subscription_handler_connect.go`)
| Method | Description | Used By |
|--------|-------------|---------|
| `CreateSubscription` | Create recurring billing | API clients |
| `UpdateSubscription` | Modify subscription | API clients |
| `CancelSubscription` | Cancel (immediate or end of period) | API clients |
| `PauseSubscription` | Pause billing | API clients |
| `ResumeSubscription` | Resume paused subscription | API clients |
| `GetSubscription` | Retrieve subscription | API clients |
| `ListCustomerSubscriptions` | List customer's subscriptions | API clients |
| `ProcessDueBilling` | Process due subscriptions | Cron job |

**Service Layer:** `internal/services/subscription/subscription_service.go`

### Merchant Service (`internal/handlers/merchant/merchant_handler_connect.go`)
| Method | Description | Used By |
|--------|-------------|---------|
| `RegisterMerchant` | Add merchant with EPX credentials | Admin |
| `GetMerchant` | Retrieve merchant | Internal |
| `ListMerchants` | List all merchants | Admin |
| `UpdateMerchant` | Update credentials | Admin |
| `DeactivateMerchant` | Disable merchant | Admin |
| `RotateMAC` | Rotate MAC secret | Admin |

**Service Layer:** `internal/services/merchant/merchant_service.go`

### Chargeback Service (`internal/handlers/chargeback/chargeback_handler_connect.go`)
| Method | Description | Used By |
|--------|-------------|---------|
| `GetChargeback` | Retrieve chargeback details | API clients |
| `ListChargebacks` | List chargebacks with filters | API clients |

**Service Layer:** `internal/services/chargeback/chargeback_service.go`
**Adapter:** `internal/adapters/north/dispute_adapter.go` (North API)

---

## 3. HTTP Endpoints (Port 8081)

### Cron Endpoints
| Endpoint | Handler | Purpose |
|----------|---------|---------|
| `/cron/process-billing` | `cron_handler.go` | Process due subscriptions |
| `/cron/sync-disputes` | `cron_handler.go` | Sync chargebacks from North |
| `/cron/verify-ach` | `cron_handler.go` | Verify ACH via microdeposits |
| `/cron/cleanup-audit-logs` | `cron_handler.go` | Archive old audit logs |
| `/cron/cleanup-rate-limits` | `cron_handler.go` | Clean rate limit buckets |
| `/cron/health` | `cron_handler.go` | Health check |

### Browser Post (EPX Integration)
| Endpoint | Handler | Purpose |
|----------|---------|---------|
| `/browser-post-callback` | `browser_post_callback_handler.go` | EPX callback receiver |
| `/browser-post-demo` | `browser_post_demo_handler.go` | Interactive demo form |

---

## 4. CLI Tools

### Admin CLI (`cmd/admin/main.go`)
| Command | Description |
|---------|-------------|
| `create-service` | Create API service with RSA keypair |
| `list-services` | List registered services |
| `get-service` | Get service details |
| `rotate-key` | Rotate service RSA key |
| `deactivate-service` | Disable service |
| `activate-service` | Re-enable service |
| `create-merchant` | Register merchant with EPX credentials |
| `list-merchants` | List registered merchants |
| `get-merchant` | Get merchant details |
| `grant-access` | Grant service access to merchant |
| `revoke-access` | Revoke service access |
| `list-access` | List service-merchant grants |

### JWT Generator (`cmd/jwtgen/main.go`)
| Flag | Description |
|------|-------------|
| `-key` | Path to RSA private key |
| `-service` | Service ID (issuer) |
| `-merchant` | Merchant ID (UUID) |
| `-scopes` | Comma-separated scopes |
| `-expiry` | Token expiration duration |

---

## 5. Active Middleware Stack

### Authentication (`internal/middleware/connect_auth.go`)
| Feature | Description |
|---------|-------------|
| JWT Validation | RS256 signature verification |
| Service-Merchant Access | Verify service can access merchant |
| Token Blacklist | Check revoked tokens |
| Rate Limiting | Token bucket per service |
| Context Injection | Add auth info to context |

### Security (`internal/middleware/security.go`)
| Header | Value |
|--------|-------|
| `Content-Security-Policy` | Strict CSP |
| `X-Content-Type-Options` | nosniff |
| `X-Frame-Options` | DENY |
| `Strict-Transport-Security` | max-age=31536000 |

### EPX Callback Auth (`internal/middleware/epx_callback_auth.go`)
- HMAC signature verification for browser post callbacks

### Other Middleware
| Middleware | File | Purpose |
|------------|------|---------|
| Rate Limiter | `rate_limiter.go` | IP-based rate limiting |
| Gzip | `gzip.go` | Response compression |
| Timeout | `timeout.go` | Request timeout (30s) |
| Recovery | `recovery.go` | Panic recovery |
| Request ID | `request_id.go` | Add X-Request-ID |

---

## 6. Active Adapters

### EPX Gateway (`internal/adapters/epx/`)
| Adapter | File | Purpose |
|---------|------|---------|
| Server Post | `server_post_adapter.go` | Card payments (auth/capture/sale/void/refund) |
| Browser Post | `browser_post_adapter.go` | Hosted payment form |
| Key Exchange | `key_exchange_adapter.go` | Encryption key management |
| BRIC | `bric_adapter.go` | Card/ACH tokenization |
| ACH | `ach_adapter.go` | ACH payments and pre-notes |

### North API (`internal/adapters/north/`)
| Adapter | File | Purpose |
|---------|------|---------|
| Dispute | `dispute_adapter.go` | Sync chargebacks |

### Database (`internal/adapters/database/`)
| Component | File | Purpose |
|-----------|------|---------|
| PostgreSQL | `postgres.go` | Connection pool management |

---

## 7. Active Services

### Core Services (`internal/services/`)
| Service | File | Responsibilities |
|---------|------|------------------|
| Payment | `payment/payment_service.go` | Transaction orchestration |
| Payment Method | `payment_method/payment_method_service.go` | Token management |
| Subscription | `subscription/subscription_service.go` | Recurring billing |
| Merchant | `merchant/merchant_service.go` | Merchant management |
| Chargeback | `chargeback/chargeback_service.go` | Dispute handling |
| Webhook | `webhook/webhook_delivery_service.go` | Event delivery |
| Authorization | `authorization/merchant_authorization.go` | Service-merchant auth |

---

## 8. Secret Management

### Supported Backends (`internal/secrets/`)
| Backend | File | Use Case |
|---------|------|----------|
| GCP Secret Manager | `gcp.go` | Production (GCP) |
| AWS Secrets Manager | `aws.go` | Production (AWS) |
| HashiCorp Vault | `vault.go` | Enterprise |
| Local Filesystem | `local.go` | Development |
| Mock | `mock.go` | Testing |

---

## 9. Caching

### Merchant Cache (`internal/cache/`)
| Cache | TTL | Purpose |
|-------|-----|---------|
| Merchant credentials | 5 min | Reduce DB queries (~70%) |
| Payment methods | 2 min | Faster lookups (~60%) |

---

## 10. Database (SQLC Generated)

### Tables Used
| Table | Primary Operations |
|-------|-------------------|
| `transactions` | CRUD, ListByMerchant, ListByCustomer |
| `customer_payment_methods` | CRUD, ListByCustomer |
| `subscriptions` | CRUD, ListDue, UpdateNextBilling |
| `agents` (merchants) | CRUD, ListActive |
| `services` | CRUD, ListActive |
| `service_merchant_access` | Grant, Revoke, Check |
| `chargebacks` | CRUD, Sync, ListByMerchant |
| `audit_logs` | Create, Cleanup |
| `jwt_blacklist` | Add, Check, Cleanup |
| `rate_limit_buckets` | Consume, Cleanup |
| `webhook_deliveries` | CRUD, ListPending |

---

## 11. Proto/API Definitions

### Active Protos (`proto/`)
| Proto | Package | Services |
|-------|---------|----------|
| `payment/v1/payment.proto` | `payment.v1` | PaymentService |
| `payment_method/v1/payment_method.proto` | `payment_method.v1` | PaymentMethodService |
| `subscription/v1/subscription.proto` | `subscription.v1` | SubscriptionService |
| `merchant/v1/merchant.proto` | `merchant.v1` | MerchantService |
| `chargeback/v1/chargeback.proto` | `chargeback.v1` | ChargebackService |

---

## 12. Background Processes

| Process | Location | Purpose |
|---------|----------|---------|
| Public Key Refresh | `connect_auth.go` | Reload keys every 5 min |
| Rate Limit Cleanup | `connect_auth.go` | Clean old buckets every 1 min |
| Webhook Delivery | `webhook_delivery_service.go` | Retry failed deliveries |
| Graceful Shutdown | `main.go` | Drain in-flight requests |

---

## Summary: Active Code Paths

```
API Request Flow:
  Client → ConnectRPC (8080) → Auth Middleware → Handler → Service → Adapter → EPX/DB

Cron Flow:
  Scheduler → HTTP (8081) → Cron Handler → Service → Adapter → EPX/North/DB

Admin Flow:
  CLI → Database (direct) → Create/Update records

Browser Post Flow:
  User → EPX Hosted Form → EPX → Callback (8081) → Handler → Service → DB
```

**Total Active Production Code:**
- 5 ConnectRPC services
- 6 cron endpoints
- 2 browser post endpoints
- 5 EPX adapters
- 1 North adapter
- 7 core services
- 10+ middleware components
- 12 CLI commands
- 5 secret backends
