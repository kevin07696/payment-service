# Payment Service - Feature & API Chart

## Complete Feature-to-API Mapping

---

## ✅ IMPLEMENTED FEATURES

### Credit Card Processing - Browser Post API

| Feature | North API Endpoint | Request Method | Auth Method | Request Format | Response Format | Status |
|---------|-------------------|----------------|-------------|----------------|-----------------|--------|
| Authorize Payment | `/browserpost` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| Capture Payment | `/browserpost` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| Sale (Auth+Capture) | `/browserpost` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| Void Transaction | `/browserpost` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| Refund Transaction | `/browserpost` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| Verify Card/Account | `/browserpost` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| AVS Verification | `/browserpost` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| CVV Verification | `/browserpost` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |

---

### ACH/Bank Processing - Pay-by-Bank API

| Feature | North API Endpoint | Request Method | Auth Method | Request Format | Response Format | Status |
|---------|-------------------|----------------|-------------|----------------|-----------------|--------|
| Process ACH Payment (Checking) | `/paybybank` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| Process ACH Payment (Savings) | `/paybybank` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| Refund ACH Payment | `/paybybank` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |
| Verify Bank Account | `/paybybank` | POST | HMAC-SHA256 | Form-encoded | XML | ✅ |

---

### Recurring Billing - Recurring Billing API

| Feature | North API Endpoint | Request Method | Auth Method | Request Format | Response Format | Status |
|---------|-------------------|----------------|-------------|----------------|-----------------|--------|
| Create Subscription | `/subscription` | POST | HMAC-SHA256* | JSON | JSON | ✅ |
| Update Subscription | `/subscription` | PUT | HMAC-SHA256* | JSON | JSON | ✅ |
| Cancel Subscription | `/subscription` | DELETE | HMAC-SHA256* | JSON | JSON | ✅ |
| Pause Subscription | `/subscription/pause` | POST | HMAC-SHA256* | JSON | JSON | ✅ |
| Resume Subscription | `/subscription/resume` | POST | HMAC-SHA256* | JSON | JSON | ✅ |
| Get Subscription | `/subscription/{id}` | GET | HMAC-SHA256* | - | JSON | ✅ |
| List Subscriptions | `/subscriptions` | GET | HMAC-SHA256* | - | JSON | ✅ |
| Charge Stored Payment Method | `/chargepaymentmethod` | POST | HMAC-SHA256* | JSON | JSON | ✅ |

*Auth method assumed, needs confirmation from North

---

### Transaction Management - Local Database (No North API)

| Feature | Data Source | Query Method | Auth Method | Status |
|---------|-------------|--------------|-------------|--------|
| List Transactions | PostgreSQL | SQL Query | gRPC Auth | ✅ |
| Get Transaction by ID | PostgreSQL | SQL Query | gRPC Auth | ✅ |
| Search by Merchant | PostgreSQL | SQL Query | gRPC Auth | ✅ |
| Search by Customer | PostgreSQL | SQL Query | gRPC Auth | ✅ |
| Get by Idempotency Key | PostgreSQL | SQL Query | gRPC Auth | ✅ |

---

## ⏳ READY TO IMPLEMENT (Infrastructure Complete)

### Chargeback Management - Dispute API

| Feature | North API Endpoint | Request Method | Auth Method | Request Format | Response Format | Status |
|---------|-------------------|----------------|-------------|----------------|-----------------|--------|
| Search Disputes by Merchant | `/merchant/disputes/mid/search` | GET | ❓ Unknown | Query params | JSON | ⏳ |
| Search Disputes by External Key | `/merchant/disputes/key/search` | GET | ❓ Unknown | Query params | JSON | ⏳ |
| Get Dispute Details | ❓ TBD | GET | ❓ Unknown | - | JSON | ⏳ |
| Submit Evidence | ❓ TBD | POST | ❓ Unknown | ❓ TBD | JSON | ⏳ |

**Waiting For**: Authentication method, complete endpoint list, sample responses

---

### Settlement Reconciliation - Settlement API (Unknown)

| Feature | North API Endpoint | Request Method | Auth Method | Request Format | Response Format | Status |
|---------|-------------------|----------------|-------------|----------------|-----------------|--------|
| Get Settlement Report | ❓ Unknown | ❓ | ❓ Unknown | ❓ | ❓ CSV/XML/JSON? | ⏳ |
| List Settlement Batches | ❓ Unknown | ❓ | ❓ Unknown | ❓ | ❓ | ⏳ |
| Download Settlement File | ❓ SFTP? | - | ❓ | - | ❓ CSV/XML? | ⏳ |

**Waiting For**: Access method (API/SFTP/Portal), file format, sample file

---

## API Authentication Summary

| API Name | Base URL | Auth Method | Auth Headers | Status |
|----------|----------|-------------|--------------|--------|
| Browser Post API | `/browserpost` | HMAC-SHA256 | `EPI-Id`, `EPI-Signature` | ✅ Confirmed |
| Pay-by-Bank API | `/paybybank` | HMAC-SHA256 | `EPI-Id`, `EPI-Signature` | ✅ Confirmed |
| Recurring Billing API | `/subscription`, `/chargepaymentmethod` | HMAC-SHA256 | `EPI-Id`, `EPI-Signature` | ⚠️ Assumed |
| Dispute API | `/merchant/disputes/*` | ❓ Unknown | ❓ Unknown | ⏳ Need Info |
| Settlement API | ❓ Unknown | ❓ Unknown | ❓ Unknown | ⏳ Need Info |
| Business Reporting API | `/accounts/{id}/transactions` | JWT | `Authorization: Bearer {token}` | 🚫 Not Using |

---

## Request/Response Format Summary

| API | Request Format | Response Format | Notes |
|-----|---------------|-----------------|-------|
| Browser Post | Form-encoded (`application/x-www-form-urlencoded`) | XML | TRAN_TYPE, TOKEN, AMOUNT, etc. |
| Pay-by-Bank | Form-encoded (`application/x-www-form-urlencoded`) | XML | TRAN_TYPE, ROUTING_NUMBER, ACCOUNT_NUMBER, etc. |
| Recurring Billing | JSON (`application/json`) | JSON | customer_id, payment_method_token, amount, frequency |
| Dispute | Query params + ❓ | JSON | findBy=byMerchant:X,fromDate:Y,toDate:Z |
| Settlement | ❓ Unknown | ❓ CSV/XML/JSON? | Unknown format |

---

## Feature Implementation Status

| Category | Total Features | Implemented | Pending | Completion % |
|----------|---------------|-------------|---------|--------------|
| Credit Card Processing | 8 | 8 | 0 | 100% |
| ACH/Bank Processing | 4 | 4 | 0 | 100% |
| Recurring Billing | 8 | 8 | 0 | 100% |
| Transaction Management | 5 | 5 | 0 | 100% |
| Chargeback Management | 4 | 0 | 4 | 0% (infra ready) |
| Settlement Reconciliation | 3 | 0 | 3 | 0% (infra ready) |
| **TOTAL** | **32** | **25** | **7** | **78%** |

---

## Data Flow Chart

### Implemented Flows

```
┌─────────────────────────────────────────────────────────────────┐
│ CREDIT CARD PAYMENT FLOW                                        │
├─────────────────────────────────────────────────────────────────┤
│ Frontend → North JS SDK → BRIC Token                            │
│ Frontend → Our gRPC API → PaymentService                        │
│ PaymentService → BrowserPostAdapter → North /browserpost        │
│ North Response → BrowserPostAdapter → PaymentService            │
│ PaymentService → PostgreSQL (save transaction)                  │
│ PaymentService → Frontend (return response)                     │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ ACH PAYMENT FLOW                                                │
├─────────────────────────────────────────────────────────────────┤
│ Frontend → Our gRPC API → PaymentService                        │
│ PaymentService → ACHAdapter → North /paybybank                  │
│ North Response → ACHAdapter → PaymentService                    │
│ PaymentService → PostgreSQL (save transaction)                  │
│ PaymentService → Frontend (return response)                     │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ SUBSCRIPTION FLOW                                               │
├─────────────────────────────────────────────────────────────────┤
│ Frontend → Our gRPC API → SubscriptionService                   │
│ SubscriptionService → RecurringBillingAdapter → North /subscription │
│ North Response → RecurringBillingAdapter → SubscriptionService  │
│ SubscriptionService → PostgreSQL (save subscription)            │
│ SubscriptionService → Frontend (return response)                │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ LIST TRANSACTIONS FLOW                                          │
├─────────────────────────────────────────────────────────────────┤
│ Frontend → Our gRPC API → PaymentService                        │
│ PaymentService → TransactionRepository → PostgreSQL             │
│ PostgreSQL → TransactionRepository → PaymentService             │
│ PaymentService → Frontend (return transactions)                 │
│ NO NORTH API CALL (uses local database)                         │
└─────────────────────────────────────────────────────────────────┘
```

### Planned Flows

```
┌─────────────────────────────────────────────────────────────────┐
│ CHARGEBACK SYNC FLOW (Hourly Job)                              │
├─────────────────────────────────────────────────────────────────┤
│ Scheduled Job → DisputeAdapter → North /merchant/disputes/mid/search │
│ North Response → DisputeAdapter → SyncService                   │
│ SyncService → TransactionRepository (find original transaction) │
│ SyncService → ChargebackRepository → PostgreSQL (save)          │
│ SyncService → Alert System (notify team)                        │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ SETTLEMENT RECONCILIATION FLOW (Daily Job)                     │
├─────────────────────────────────────────────────────────────────┤
│ Scheduled Job → SettlementAdapter → North Settlement API/SFTP   │
│ North Response → SettlementParser (parse CSV/XML/JSON)          │
│ SettlementParser → SettlementService                            │
│ SettlementService → SettlementRepository → PostgreSQL (save)    │
│ ReconciliationService → Compare our txns vs settlement          │
│ ReconciliationService → Alert on discrepancies                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Environment Variables by API

| API | Environment Variable | Example Value | Status |
|-----|---------------------|---------------|--------|
| Browser Post | `NORTH_BASE_URL` | `https://api.north.com/api/browserpost` | ✅ |
| Browser Post | `NORTH_EPI_ID` | `7000-700010-1-1` | ✅ |
| Browser Post | `NORTH_EPI_KEY` | `your_secret_key_here` | ✅ |
| Browser Post | `NORTH_TIMEOUT` | `30` | ✅ |
| Pay-by-Bank | (same as Browser Post) | - | ✅ |
| Recurring Billing | (same as Browser Post) | - | ✅ |
| Dispute API | `NORTH_DISPUTE_???` | ❓ TBD | ⏳ |
| Settlement API | `NORTH_SETTLEMENT_???` | ❓ TBD | ⏳ |

---

## Code Files by Feature

| Feature Category | Adapter File | Service File | Handler File | Repository File | Status |
|-----------------|--------------|--------------|--------------|-----------------|--------|
| Credit Card | `internal/adapters/north/browser_post_adapter.go` | `internal/services/payment/payment_service.go` | `internal/api/grpc/payment/payment_handler.go` | `internal/adapters/postgres/transaction_repository.go` | ✅ |
| ACH/Bank | `internal/adapters/north/ach_adapter.go` | `internal/services/payment/payment_service.go` | `internal/api/grpc/payment/payment_handler.go` | `internal/adapters/postgres/transaction_repository.go` | ✅ |
| Subscriptions | `internal/adapters/north/recurring_billing_adapter.go` | `internal/services/subscription/subscription_service.go` | `internal/api/grpc/subscription/subscription_handler.go` | `internal/adapters/postgres/subscription_repository.go` | ✅ |
| Chargebacks | ⏳ `dispute_adapter.go` (template ready) | ⏳ `chargeback/sync_service.go` (template ready) | - | ⏳ Interface defined | ⏳ |
| Settlements | ⏳ `settlement_adapter.go` (template ready) | ⏳ `settlement/settlement_service.go` (template ready) | - | ⏳ Interface defined | ⏳ |

---

## Database Tables by Feature

| Feature Category | Tables Used | Migration File | Status |
|-----------------|-------------|----------------|--------|
| All Transactions | `transactions` | `001_transactions.sql` | ✅ |
| Subscriptions | `subscriptions` | `001_transactions.sql` | ✅ |
| Audit Logs | `audit_logs` | `001_transactions.sql` | ✅ |
| Chargebacks | `chargebacks` | `002_chargebacks.sql` | ✅ Ready |
| Settlements | `settlement_batches`, `settlement_transactions` | `003_settlements.sql` | ✅ Ready |

---

## Test Coverage by API

| API/Adapter | Test File | Test Count | Coverage | Status |
|-------------|-----------|------------|----------|--------|
| Browser Post | `browser_post_adapter_test.go` | 19 tests | 89.0% | ✅ |
| ACH | `ach_adapter_test.go` | 16 tests | 88-100% | ✅ |
| Recurring Billing | `recurring_billing_adapter_test.go` | 14 tests | High | ✅ |
| Payment Service | `payment_service_test.go` | 7 tests | 76.0% | ✅ |
| Subscription Service | `subscription_service_test.go` | 15 tests | 77.0% | ✅ |
| Dispute Adapter | - | 0 tests | - | ⏳ |
| Settlement Adapter | - | 0 tests | - | ⏳ |

---

## API Response Codes

### Browser Post & Pay-by-Bank API

| Code | Category | Meaning | Retry? | Status |
|------|----------|---------|--------|--------|
| `00` | Approved | Success | - | ✅ Handled |
| `05` | Declined | Do not honor | No | ✅ Handled |
| `14` | Invalid | Invalid card number | No | ✅ Handled |
| `41` | Fraud | Lost card | No | ✅ Handled |
| `43` | Fraud | Stolen card | No | ✅ Handled |
| `51` | Declined | Insufficient funds | No | ✅ Handled |
| `54` | Invalid | Expired card | No | ✅ Handled |
| `59` | Fraud | Suspected fraud | No | ✅ Handled |
| `82` | Invalid | CVV mismatch | No | ✅ Handled |
| `91` | System | Issuer unavailable | Yes | ✅ Handled |
| `96` | System | System error | Yes | ✅ Handled |

### Dispute API

| Status | Our Mapping | Action | Status |
|--------|-------------|--------|--------|
| `NEW` | `pending` | Needs review | ⏳ Ready |
| `PENDING` | `pending` | Awaiting response | ⏳ Ready |
| `WON` | `won` | Merchant won | ⏳ Ready |
| `LOST` | `lost` | Customer won | ⏳ Ready |
| `ACCEPTED` | `accepted` | Not contested | ⏳ Ready |
| ❓ Others? | ❓ | ❓ | ⏳ Need from North |

---

## Questions for North Support

| Category | Question | Priority | Status |
|----------|----------|----------|--------|
| Dispute API | What authentication method? (HMAC/JWT/API Key?) | 🔴 High | ⏳ |
| Dispute API | What headers are required? | 🔴 High | ⏳ |
| Dispute API | Complete list of status values? | 🟡 Medium | ⏳ |
| Dispute API | Complete list of disputeType values? | 🟡 Medium | ⏳ |
| Dispute API | Evidence submission endpoint? | 🟡 Medium | ⏳ |
| Dispute API | Pagination support? | 🟢 Low | ⏳ |
| Dispute API | Rate limits? | 🟢 Low | ⏳ |
| Settlement | Access method (API/SFTP/Portal/Email)? | 🔴 High | ⏳ |
| Settlement | File format (CSV/XML/JSON)? | 🔴 High | ⏳ |
| Settlement | Sample settlement report? | 🔴 High | ⏳ |
| Settlement | Settlement schedule (daily/weekly)? | 🟡 Medium | ⏳ |
| Settlement | Settlement timing (T+1/T+2/T+3)? | 🟡 Medium | ⏳ |
| Recurring Billing | Confirm HMAC authentication? | 🟡 Medium | ⏳ |

---

## Implementation Timeline

| Phase | Features | APIs | Duration | Dependencies | Status |
|-------|----------|------|----------|--------------|--------|
| Phase 1 | Credit Card, ACH, Transactions | Browser Post, Pay-by-Bank | 2 weeks | None | ✅ Complete |
| Phase 2 | Subscriptions, Stored Methods | Recurring Billing | 1 week | Phase 1 | ✅ Complete |
| Phase 3 | Chargebacks | Dispute API | 2-3 days | North auth details | ⏳ Waiting |
| Phase 4 | Settlements | Settlement API/SFTP | 1-2 days | North access method | ⏳ Waiting |

**Current Status**: 78% complete (25/32 features)
**Blocking Item**: North support response
**ETA to 100%**: 3-5 days after North responds
