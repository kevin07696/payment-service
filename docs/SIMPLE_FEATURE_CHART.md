# Payment Service - Feature to API Mapping

## Credit Card Processing

| Feature | North API Endpoint |
|---------|-------------------|
| Authorize Payment | `/browserpost` |
| Capture Payment | `/browserpost` |
| Sale (Auth+Capture) | `/browserpost` |
| Void Transaction | `/browserpost` |
| Refund Transaction | `/browserpost` |
| Verify Card | `/browserpost` |
| AVS Verification | `/browserpost` |
| CVV Verification | `/browserpost` |

---

## ACH/Bank Processing

| Feature | North API Endpoint |
|---------|-------------------|
| Process ACH Payment (Checking) | `/paybybank` |
| Process ACH Payment (Savings) | `/paybybank` |
| Refund ACH Payment | `/paybybank` |
| Verify Bank Account | `/paybybank` |

---

## Recurring Billing / Subscriptions

| Feature | North API Endpoint |
|---------|-------------------|
| Create Subscription | `/subscription` |
| Update Subscription | `/subscription` |
| Cancel Subscription | `/subscription` |
| Pause Subscription | `/subscription/pause` |
| Resume Subscription | `/subscription/resume` |
| Get Subscription | `/subscription/{id}` |
| List Subscriptions | `/subscriptions` |
| Charge Stored Payment Method | `/chargepaymentmethod` |

---

## Transaction Management

| Feature | API/Data Source |
|---------|----------------|
| List Transactions | PostgreSQL (Local Database) |
| Get Transaction by ID | PostgreSQL (Local Database) |
| Search by Merchant | PostgreSQL (Local Database) |
| Search by Customer | PostgreSQL (Local Database) |
| Get by Idempotency Key | PostgreSQL (Local Database) |

---

## Chargeback Management

| Feature | North API Endpoint |
|---------|-------------------|
| Search Disputes by Merchant ID | `/merchant/disputes/mid/search` |
| Search Disputes by External Key | `/merchant/disputes/key/search` |
| Get Dispute Details | TBD (need from North) |
| Submit Evidence | TBD (need from North) |

---

## Settlement Reconciliation

| Feature | North API Endpoint |
|---------|-------------------|
| Get Settlement Report | Unknown (need from North) |
| List Settlement Batches | Unknown (need from North) |
| Download Settlement File | Unknown (need from North) |
