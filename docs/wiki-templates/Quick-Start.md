# Quick Start (5 Minutes)

Get the payment service running locally in under 5 minutes using Docker.

## Prerequisites

- **Docker** installed ([Get Docker](https://www.docker.com/get-started))
- **EPX API credentials** (see [EPX Credentials Guide](EPX-Credentials) if you don't have them)

## Step 1: Clone Repository

```bash
git clone https://github.com/kevin07696/payment-service.git
cd payment-service
```

## Step 2: Configure Environment

```bash
# Copy environment template
cp .env.example .env

# Edit with your EPX credentials (or use sandbox defaults)
nano .env
```

**Minimum required configuration:**

```bash
# Database (Docker will create this automatically)
DATABASE_URL=postgres://postgres:postgres@postgres:5432/payments?sslmode=disable

# EPX Sandbox Credentials (replace with your own)
EPX_CUST_NBR=9001
EPX_MERCH_NBR=900300
EPX_DBA_NBR=2
EPX_TERMINAL_NBR=77
EPX_MAC_SECRET=your-epx-mac-secret-here

# EPX URLs (Sandbox/UAT)
EPX_API_URL=https://api.epxuap.com
EPX_BROWSER_POST_URL=https://services.epxuap.com/browserpost/
EPX_KEY_EXCHANGE_URL=https://services.epxuap.com/keyexchange/

# Server Configuration
SERVICE_URL=http://localhost:8081
```

## Step 3: Start Services

```bash
# Start PostgreSQL + Migrations + Payment Server
docker-compose up -d

# Wait for services to initialize (10-15 seconds)
sleep 15
```

## Step 4: Verify It's Running

```bash
# Health check
curl http://localhost:8081/cron/health

# Expected output:
# {"status":"healthy","database":"connected"}
```

## Step 5: Make Your First Payment (Browser Post)

The easiest way to test is using Browser Post (PCI-compliant frontend tokenization):

### 5.1 Generate TAC Token

```bash
# Get Browser Post form configuration
curl -X GET "http://localhost:8081/api/v1/payments/browser-post/form?transaction_id=test-$(date +%s)&merchant_id=00000000-0000-0000-0000-000000000001&amount=10.00&transaction_type=SALE&return_url=http://localhost:8081/api/v1/payments/browser-post/callback"
```

This returns form data including:
- `tac`: Temporary Authorization Code (valid for 4 hours)
- `postURL`: EPX Browser Post endpoint
- `epxTranNbr`: Numeric transaction number

### 5.2 Submit Test Payment

Create an HTML form and submit to EPX:

```html
<!DOCTYPE html>
<html>
<body>
<form method="POST" action="https://services.epxuap.com/browserpost/">
    <!-- Hidden fields from Step 5.1 -->
    <input type="hidden" name="TAC" value="<TAC from step 5.1>" />
    <input type="hidden" name="CUST_NBR" value="9001" />
    <input type="hidden" name="MERCH_NBR" value="900300" />
    <input type="hidden" name="DBA_NBR" value="2" />
    <input type="hidden" name="TERMINAL_NBR" value="77" />
    <input type="hidden" name="TRAN_NBR" value="<epxTranNbr from step 5.1>" />
    <input type="hidden" name="TRAN_CODE" value="SALE" />
    <input type="hidden" name="AMOUNT" value="10.00" />
    <input type="hidden" name="INDUSTRY_TYPE" value="E" />

    <!-- Test card (EPX sandbox approval card) -->
    <input type="text" name="ACCOUNT_NBR" value="4111111111111111" />
    <input type="text" name="EXP_DATE" value="1225" />
    <input type="text" name="CVV" value="123" />

    <button type="submit">Pay $10.00</button>
</form>
</body>
</html>
```

**Or use automated integration tests:**

```bash
# Run Phase 1 critical tests (includes Browser Post automation)
EPX_MAC_STAGING="$(cat secrets/epx/staging/mac_secret)" \
SERVICE_URL="http://localhost:8081" \
go test -tags=integration -v ./tests/integration/payment/ \
  -run TestBrowserPostIdempotency \
  -timeout 5m
```

## What's Running?

After `docker-compose up`:

- **PostgreSQL**: Database on `localhost:5432`
- **Payment Server (gRPC)**: `localhost:8080`
- **Payment Server (HTTP)**: `http://localhost:8081`
  - Browser Post callback: `POST /api/v1/payments/browser-post/callback`
  - Cron jobs: `/cron/*`
  - Health check: `GET /cron/health`
- **Prometheus Metrics**: `http://localhost:9090/metrics`

## View Logs

```bash
# All services
docker-compose logs -f

# Just payment server
docker-compose logs -f payment-server

# Just database
docker-compose logs -f postgres
```

## Stop Services

```bash
# Stop but keep data
docker-compose down

# Stop and remove all data
docker-compose down -v
```

## Next Steps

✅ **You're now running!** Here's what to explore next:

1. **[Complete Setup Guide](Setup-Guide)** - Detailed configuration options
2. **[EPX Credentials](EPX-Credentials)** - Get your own EPX merchant account
3. **[Browser Post Flow](DATAFLOW#browser-post-flow)** - Understand the payment flow
4. **[API Reference](API-Specs)** - Explore all available APIs
5. **[FAQ](FAQ)** - Common questions answered

## Troubleshooting

**Port already in use:**
```bash
# Change ports in docker-compose.yml or .env
# Or stop conflicting services
sudo lsof -ti:8080 | xargs kill
```

**Database connection failed:**
```bash
# Check PostgreSQL is running
docker ps | grep postgres

# Restart services
docker-compose restart
```

**EPX authentication failed (Code 58):**
- Verify `EPX_MAC_SECRET` in `.env` matches your EPX credentials
- Ensure `EPX_CUST_NBR`, `EPX_MERCH_NBR`, `EPX_DBA_NBR`, `EPX_TERMINAL_NBR` are correct
- Contact EPX to verify credentials are active in sandbox

**More help:** See [Troubleshooting Guide](Troubleshooting)
