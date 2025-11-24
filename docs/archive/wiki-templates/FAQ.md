# Frequently Asked Questions (FAQ)

Common questions and answers about the payment service.

## Table of Contents

- [General Questions](#general-questions)
- [Browser Post & Callbacks](#browser-post--callbacks)
- [EPX Integration](#epx-integration)
- [Security & PCI Compliance](#security--pci-compliance)
- [Testing & Development](#testing--development)
- [Deployment & Production](#deployment--production)

---

## General Questions

### What payment methods are supported?

- ✅ **Credit Cards** (Visa, MasterCard, Amex, Discover)
- ✅ **ACH/Bank Transfers** (Checking & Savings accounts)
- ✅ **Recurring Billing** (Subscriptions)
- ✅ **Saved Payment Methods** (Card-on-file with BRIC Storage)

### What is a BRIC token?

**BRIC** (Bank Routing Information Code) is EPX's tokenization system that replaces sensitive card data:

- **Financial BRIC** (`AUTH_GUID`): Single-use token from auth/sale transactions, used for refunds/voids
- **Storage BRIC**: Multi-use token for saved payment methods, never expires, used for recurring billing

**Benefits:**
- No raw card data stored in your database (PCI-reduced scope)
- Secure token-based references
- Enables recurring payments

### How do I get started?

1. **[Get EPX Credentials](EPX-Credentials)** - Obtain sandbox merchant account
2. **[Quick Start](Quick-Start)** - Run service in 5 minutes with Docker
3. **[Make First Payment](Quick-Start#step-5-make-your-first-payment-browser-post)** - Test Browser Post flow
4. **[Read Payment Flows](DATAFLOW)** - Understand how payments work

---

## Browser Post & Callbacks

### What is Browser Post and why use it?

**Browser Post** is EPX's PCI-compliant method where card data goes directly from the user's browser to EPX (never touching your backend):

```
User Browser → EPX (processes card) → Redirects back to your callback URL
```

**Benefits:**
- ✅ **PCI-Reduced Scope**: Your backend never sees raw card data
- ✅ **No PCI Certification Required**: EPX handles sensitive data
- ✅ **Simple Integration**: Just build an HTML form
- ✅ **Secure**: Card data encrypted in transit to EPX

**When to use:**
- Direct customer payments (checkout pages)
- Initial payment method setup
- Any scenario where end-user enters card details

### How does the Browser Post callback work?

**Complete Flow:**

```
1. Your Backend: Generate TAC token via Key Exchange API
   ↓
2. Your Backend: Build HTML form with TAC + merchant credentials
   ↓
3. User Browser: Submit form to EPX Browser Post endpoint
   ↓
4. EPX: Process payment, validate card
   ↓
5. EPX: Redirect browser to your CALLBACK_URL with results
   ↓
6. Your Backend: Receive POST at /api/v1/payments/browser-post/callback
   ↓
7. Your Backend: Parse response, store transaction, return HTML receipt
```

**Key Point:** EPX sends payment results **directly to the user's browser**, which then POSTs to your callback endpoint. This is **not** a server-to-server call.

### What data does EPX send to the callback?

EPX includes payment results in the POST body:

**Success Response:**
```
AUTH_GUID=bric-token-12345          # Financial BRIC for refunds/voids
AUTH_RESP=00                         # Response code (00 = approved)
AUTH_CODE=ABC123                     # Bank authorization code
AUTH_AMOUNT=99.99                    # Authorized amount
AUTH_CARD_TYPE=VISA                  # Card brand
AUTH_CARD_NBR=XXXX1111              # Masked card number
AUTH_AVS=Y                          # AVS verification result
AUTH_CVV2=M                         # CVV verification result
TRAN_NBR=TXN-12345                  # Your transaction ID (echo back)
USER_DATA_1=customer_id=123         # Custom data (echo back)
USER_DATA_2=save_payment_method     # Custom data (echo back)
```

**Decline Response:**
```
AUTH_RESP=51                        # Decline code (51 = insufficient funds)
AUTH_RESP_TEXT=INSUFF FUNDS         # Human-readable message
TRAN_NBR=TXN-12345                  # Your transaction ID (echo back)
```

See [Browser Post Dataflow](DATAFLOW#browser-post-flow) for complete field reference.

### Why do I need ngrok for local development?

EPX needs to **redirect the user's browser to your callback URL**. When developing locally, `localhost:8081` isn't accessible from the internet.

**ngrok** creates a secure tunnel:

```bash
ngrok http 8081
# Creates: https://abc123.ngrok.io → http://localhost:8081
```

**Alternative Solutions:**
1. **Deploy to staging** (Google Cloud Run, Heroku, etc.)
2. **Use integration tests** (our tests use headless Chrome to simulate the browser flow locally)
3. **Port forwarding** (if you have a public IP and router access)

**For production:** Use your real domain (e.g., `https://api.yourdomain.com`)

### How do I set the callback URL?

The callback URL is configured in two places:

**1. Environment Variable (.env):**
```bash
CALLBACK_BASE_URL=http://localhost:8081  # Local dev
# or
CALLBACK_BASE_URL=https://abc123.ngrok.io  # ngrok
# or
CALLBACK_BASE_URL=https://api.yourdomain.com  # Production
```

**2. Browser Post Form (Key Exchange):**
```bash
# When generating TAC token, specify return URL
return_url=${CALLBACK_BASE_URL}/api/v1/payments/browser-post/callback
```

**What happens:**
- EPX redirects browser to: `https://abc123.ngrok.io/api/v1/payments/browser-post/callback?transaction_id=...&merchant_id=...&transaction_type=...`
- Your server receives POST with payment results
- Your server stores transaction in database
- Your server returns HTML receipt to user

### Can I test Browser Post without ngrok?

Yes! Use our **automated integration tests** that simulate the browser flow using headless Chrome:

```bash
# Tests handle Browser Post automation locally
EPX_MAC_STAGING="$(cat secrets/epx/staging/mac_secret)" \
SERVICE_URL="http://localhost:8081" \
go test -tags=integration -v ./tests/integration/payment/ \
  -run TestBrowserPostIdempotency \
  -timeout 5m
```

**What the test does:**
1. Generates TAC token from your local server
2. Launches headless Chrome
3. Submits payment form to EPX
4. EPX processes payment
5. EPX redirects to `http://localhost:8081/api/v1/payments/browser-post/callback`
6. Test verifies transaction was stored correctly

This works **without ngrok** because the browser runs on the same machine as your server.

### What's the difference between Browser Post callback and Server Post?

| Aspect | Browser Post Callback | Server Post |
|--------|----------------------|-------------|
| **Flow** | User Browser → EPX → Your Callback URL | Your Backend → EPX API (direct) |
| **PCI Scope** | Reduced (card never touches backend) | Higher (if you collect card data) |
| **Use Case** | Direct customer payments | Backend processing with existing tokens |
| **Callback Type** | Browser redirect (GET/POST) | No callback (direct API response) |
| **BRIC Source** | Returns Financial BRIC in callback | Requires existing BRIC token as input |

**When to use Browser Post:** Initial payment capture (user checkout)
**When to use Server Post:** Recurring payments, refunds, captures (using existing BRIC)

### How does idempotency work with Browser Post callbacks?

**Problem:** EPX might call your callback multiple times (browser refreshes, network retries).

**Solution:** Database PRIMARY KEY on `transaction_id` prevents duplicates:

```sql
-- internal/db/migrations/001_initial_schema.sql
CREATE TABLE transactions (
    id UUID PRIMARY KEY,              -- Prevents duplicate transactions
    group_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL,
    amount DECIMAL(19, 4) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
```

**What happens:**
1. First callback: Transaction inserted ✅
2. Second callback (duplicate): INSERT fails with unique constraint error
3. Handler catches error, returns existing transaction (idempotent!)

**Tested in:** `TestBrowserPostIdempotency` (tests/integration/payment/payment_service_critical_test.go:25)

---

## EPX Integration

### What EPX transaction types are supported?

**Credit Card Transactions:**
- ✅ **CCE1 (SALE)**: Authorization + Capture (one-step)
- ✅ **CCE2 (AUTH)**: Authorization only
- ✅ **CCE3 (CAPTURE)**: Capture previously authorized transaction
- ✅ **CCE5 (VOID)**: Cancel transaction before settlement
- ✅ **CCE6 (REFUND)**: Return funds to customer
- ✅ **CCE8 (BRIC Storage)**: Convert Financial BRIC to Storage BRIC

**ACH Transactions:**
- ✅ **CKC1 (DEBIT)**: Bank account debit
- ✅ **CKC6 (CREDIT)**: Bank account credit (refund)
- ✅ **CKC8 (BRIC Storage)**: Save bank account for recurring billing

### How do I get EPX sandbox credentials?

See complete guide: **[EPX Credentials Guide](EPX-Credentials)**

**Quick version:**
1. Contact EPX sales: sales@epxuap.com
2. Request sandbox merchant account
3. Enable Server Post, Browser Post, BRIC Storage, Key Exchange
4. Receive credentials: `CUST_NBR`, `MERCH_NBR`, `DBA_NBR`, `TERMINAL_NBR`, `MAC_SECRET`

### What's the difference between Sandbox (UAT) and Production?

| Environment | Purpose | URLs | Credentials | Card Numbers |
|-------------|---------|------|-------------|--------------|
| **Sandbox (UAT)** | Development & testing | `api.epxuap.com` | EPX sandbox account | Test cards (4111...) |
| **Production** | Live transactions | `api.epx.com` | EPX production account | Real customer cards |

**Important:** Sandbox and Production use **different credentials**. Never use production credentials in development!

---

## Security & PCI Compliance

### Is this service PCI-compliant?

**Yes, with PCI-reduced scope** when using Browser Post:

✅ **What we DO:**
- Store BRIC tokens (EPX's tokenization)
- Process payments via EPX API
- Store transaction metadata (amount, status, timestamps)

❌ **What we DON'T do:**
- Store raw card numbers (PANs)
- Store CVV codes
- Store unencrypted cardholder data

**Result:** SAQ A or SAQ A-EP compliance (reduced PCI scope)

**Best Practices:**
- Use Browser Post for all direct customer payments
- Never log card data
- Use HTTPS/TLS for all connections
- Rotate `MAC_SECRET` periodically
- Store credentials in secret management service

### How is the MAC_SECRET used?

The `MAC_SECRET` is used for **HMAC-SHA256 authentication** to sign all API requests:

```go
// internal/adapters/epx/server_post_adapter.go
message := custNbr + merchNbr + dbaNbr + terminalNbr + amount + ...
signature := HMAC_SHA256(message, MAC_SECRET)

// EPX validates signature matches before processing
```

**Security:**
- ✅ Prevents unauthorized access to your merchant account
- ✅ Ensures requests haven't been tampered with
- ✅ Validates sender authenticity

**⚠️ Keep MAC_SECRET secure:**
- Never commit to version control
- Use secret management (AWS Secrets Manager, GCP Secret Manager)
- Rotate every 3-6 months

---

## Testing & Development

### How do I run integration tests?

```bash
# 1. Start services
docker-compose up -d

# 2. Run Phase 1 critical tests (5 tests, ~2.5 minutes)
EPX_MAC_STAGING="$(cat secrets/epx/staging/mac_secret)" \
SERVICE_URL="http://localhost:8081" \
go test -tags=integration -v ./tests/integration/payment/ \
  -run "TestBrowserPostIdempotency|TestRefundAmountValidation|TestCaptureStateValidation|TestConcurrentOperationHandling|TestEPXDeclineCodeHandling" \
  -timeout 15m

# 3. Run Server Post idempotency tests (5 tests)
go test -tags=integration -v ./tests/integration/payment/ \
  -run TestServerPostIdempotency \
  -timeout 10m
```

See [Integration Test Strategy](INTEGRATION-TEST-STRATEGY) for complete guide.

### What test cards work in EPX sandbox?

**Approval Card (always approves):**
```
Card: 4111111111111111
CVV: 123
Exp: 12/25 (any future date)
ZIP: 12345
```

**Decline Card (triggers error codes):**
```
Card: 4000000000000002
CVV: 123
Exp: 12/25

Amount triggers:
- $1.05 → Code 05 (Do Not Honor)
- $1.20 → Code 51 (Insufficient Funds)
- $1.54 → Code 54 (Expired Card)
- $1.82 → Code 82 (CVV Error)
```

**Reference:** EPX documentation "Response Code Triggers - Visa.pdf"

### How do I debug callback issues?

**1. Check server logs:**
```bash
docker-compose logs -f payment-server | grep callback
```

**2. Verify callback URL:**
```bash
# Should match CALLBACK_BASE_URL in .env
echo $CALLBACK_BASE_URL

# Test endpoint is accessible
curl $CALLBACK_BASE_URL/cron/health
```

**3. Use integration tests:**
```bash
# Tests show detailed logs of Browser Post flow
go test -tags=integration -v ./tests/integration/payment/ \
  -run TestBrowserPostIdempotency -timeout 5m
```

**Common issues:**
- Callback URL not publicly accessible (use ngrok)
- Wrong `CALLBACK_BASE_URL` in `.env`
- Firewall blocking EPX redirects
- Server not running

---

## Deployment & Production

### How do I deploy to production?

See guides:
- **[GCP Cloud Run](GCP-PRODUCTION-SETUP)** - Managed containers
- **[Docker Deployment](Setup-Guide#production-deployment)** - Self-hosted

**Pre-deployment checklist:**
- [ ] EPX production credentials obtained
- [ ] `MAC_SECRET` stored in secret manager (not `.env`)
- [ ] Database uses managed PostgreSQL (Cloud SQL, RDS, etc.)
- [ ] HTTPS/TLS enabled
- [ ] Callback URL uses production domain
- [ ] Monitoring and alerts configured
- [ ] Tested with EPX production sandbox first

### How do I handle database migrations in production?

**Automated (Recommended):**
```bash
# Migrations run automatically on container startup
# See: cmd/server/main.go runMigrations()
```

**Manual (if needed):**
```bash
# Using goose CLI
export DATABASE_URL="postgres://user:pass@prod-db:5432/payments?sslmode=require"
goose -dir internal/db/migrations postgres "$DATABASE_URL" up
```

**Best Practices:**
- Test migrations in staging first
- Use backward-compatible migrations
- Never drop columns (deprecate + cleanup later)
- Run migrations during low-traffic windows

### What monitoring should I set up?

**Prometheus Metrics** (exposed on `:9090/metrics`):
```
grpc_requests_total{method, status}
grpc_request_duration_seconds{method}
grpc_requests_in_flight
```

**Health Checks:**
```bash
# Liveness probe
curl http://your-service:9090/health

# Readiness probe
curl http://your-service:9090/ready
```

**Alerts to Configure:**
- High error rate (> 5%)
- Slow response times (> 2s p95)
- Database connection failures
- EPX authentication failures (Code 58)

---

## Still Have Questions?

- **[Quick Start](Quick-Start)** - Get running in 5 minutes
- **[Setup Guide](Setup-Guide)** - Complete configuration
- **[Troubleshooting](Troubleshooting)** - Common issues & solutions
- **[EPX API Reference](EPX-API-REFERENCE)** - API documentation
- **[GitHub Issues](https://github.com/kevin07696/payment-service/issues)** - Report bugs or ask questions
