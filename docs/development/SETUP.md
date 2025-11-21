# Payment Service Setup Guide

**Target Audience:** DevOps engineers, infrastructure operators, service maintainers
**Purpose:** Set up and run the payment service infrastructure
**For API Integration:** See [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md)

Complete guide to setting up and running the payment service locally and in production.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Getting EPX API Credentials](#getting-epx-api-credentials)
- [Local Development Setup](#local-development-setup)
- [Environment Configuration](#environment-configuration)
- [Running Integration Tests](#running-integration-tests)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software

- **Go 1.24+**: [Download](https://golang.org/dl/)
- **PostgreSQL 15+**: [Download](https://www.postgresql.org/download/)
- **Protocol Buffers Compiler (protoc)**: [Installation Guide](https://grpc.io/docs/protoc-installation/)
- **Docker** (optional but recommended): [Download](https://www.docker.com/get-started)
- **Chrome/Chromium** (for integration tests): Required for headless browser automation

### Optional Tools

- **ngrok** (for Browser Post testing): Exposes local server for EPX callbacks
- **Postman/Insomnia**: For API testing
- **pgAdmin**: PostgreSQL GUI client

## Getting EPX API Credentials

### 1. Contact EPX Sales

EPX (formerly Element Payment Services) is the payment gateway provider. You need a merchant account to use this service.

**Contact Information:**
- **Website**: [https://www.epxuap.com](https://www.epxuap.com)
- **Sales Email**: sales@epxuap.com
- **Phone**: Contact your EPX sales representative

**What to Request:**
- Merchant account for credit card processing
- Sandbox/UAT environment credentials (for development)
- Production credentials (when ready to go live)
- Enable the following transaction types:
  - **Server Post API** (CCE1, CCE2, CCE3, CCE5, CCE6)
  - **Browser Post API** (for PCI-compliant frontend tokenization)
  - **BRIC Storage** (CCE8/CKC8 for saved payment methods)
  - **Key Exchange API** (for TAC token generation)

### 2. Receive Merchant Credentials

EPX will provide you with the following credentials:

#### Server Post Credentials
```
CUST_NBR=1234            # Customer number (merchant ID)
MERCH_NBR=567890         # Merchant number (location ID)
DBA_NBR=1                # DBA number (business name)
TERMINAL_NBR=1           # Terminal number (POS identifier)
MAC_SECRET=abc123...     # HMAC secret for authentication
```

#### Browser Post Credentials
```
Same as Server Post, plus:
REDIRECT_URL=https://yourdomain.com/api/v1/payments/browser-post/callback
```

#### Environment URLs
```
UAT/Sandbox:
- Server Post API: https://api.epxuap.com
- Browser Post API: https://services.epxuap.com/browserpost/
- Key Exchange: https://services.epxuap.com/keyexchange/

Production:
- Server Post API: https://api.epx.com
- Browser Post API: https://services.epx.com/browserpost/
- Key Exchange: https://services.epx.com/keyexchange/
```

### 3. Test Credentials Setup

EPX provides test card numbers for sandbox testing:

**Approval Card:**
```
Card Number: 4111111111111111
CVV: 123
Exp Date: 12/25 (any future date)
ZIP: 12345
```

**Decline Card (for testing error handling):**
```
Card Number: 4000000000000002
CVV: 123
Exp Date: 12/25
Amount Triggers: Last 3 digits determine response code
  - $1.05 = Code 05 (Do Not Honor)
  - $1.20 = Code 51 (Insufficient Funds)
  - $1.54 = Code 54 (Expired Card)
```

See EPX documentation: "Response Code Triggers - Visa.pdf"

## Local Development Setup

### Option 1: Docker (Recommended)

**Quickest way to get started** - runs PostgreSQL, migrations, and payment server automatically.

```bash
# 1. Clone the repository
git clone https://github.com/kevin07696/payment-service.git
cd payment-service

# 2. Copy environment template
cp .env.example .env

# 3. Edit .env with your EPX credentials
nano .env

# 4. Start all services (PostgreSQL + migrations + payment server)
docker-compose up -d

# 5. View logs
docker-compose logs -f payment-server

# 6. Test the service
curl http://localhost:8081/cron/health
```

**Services will be available at:**
- gRPC API: `localhost:8080`
- HTTP endpoints: `http://localhost:8081`
- PostgreSQL: `localhost:5432`
- Prometheus metrics: `http://localhost:9090/metrics`

### Option 2: Local Go Binary

**For development with hot-reload and debugging.**

```bash
# 1. Clone repository
git clone https://github.com/kevin07696/payment-service.git
cd payment-service

# 2. Install dependencies
go mod download

# 3. Install required tools
go install github.com/pressly/goose/v3/cmd/goose@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 4. Start PostgreSQL (via Docker or local install)
# Docker:
docker run -d \
  --name payment-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=payments \
  -p 5432:5432 \
  postgres:15-alpine

# Or use existing PostgreSQL installation

# 5. Run database migrations
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"
goose -dir internal/db/migrations postgres "$DATABASE_URL" up

# 6. Copy environment template
cp .env.example .env

# 7. Edit .env with your credentials
nano .env

# 8. Build and run the server
go build -o bin/payment-server ./cmd/server
./bin/payment-server
```

## Environment Configuration

### .env File Structure

Create a `.env` file in the project root with your EPX credentials:

```bash
# =============================================================================
# Database Configuration
# =============================================================================
DATABASE_URL=postgres://postgres:postgres@localhost:5432/payments?sslmode=disable

# =============================================================================
# EPX Server Post Credentials (UAT/Sandbox)
# =============================================================================
# These credentials are for direct API integration (Server Post)
# Contact EPX to obtain these values for your merchant account

EPX_CUST_NBR=9001                           # Customer number (merchant ID)
EPX_MERCH_NBR=900300                        # Merchant number (location ID)
EPX_DBA_NBR=2                               # DBA number (business name)
EPX_TERMINAL_NBR=77                         # Terminal number (POS ID)
EPX_MAC_SECRET=your-mac-secret-here         # HMAC secret for authentication
EPX_API_URL=https://api.epxuap.com          # UAT/Sandbox URL

# =============================================================================
# EPX Browser Post URLs (UAT/Sandbox)
# =============================================================================
# These URLs are for PCI-compliant frontend tokenization

EPX_BROWSER_POST_URL=https://services.epxuap.com/browserpost/
EPX_KEY_EXCHANGE_URL=https://services.epxuap.com/keyexchange/

# =============================================================================
# Server Configuration
# =============================================================================
GRPC_PORT=8080                              # gRPC API port
HTTP_PORT=8081                              # HTTP endpoints (callbacks, cron jobs)
METRICS_PORT=9090                           # Prometheus metrics

# =============================================================================
# Integration Test Configuration
# =============================================================================
# Used by integration tests to connect to running service

SERVICE_URL=http://localhost:8081           # Base URL for HTTP endpoints
EPX_MAC_STAGING=your-mac-secret-here        # Same as EPX_MAC_SECRET (for tests)

# =============================================================================
# Browser Post Callback URL
# =============================================================================
# This is where EPX redirects users after payment processing
# For local development with Browser Post:
#   - Use ngrok to expose localhost: ngrok http 8081
#   - Set CALLBACK_BASE_URL to your ngrok URL
#   - EPX will redirect to: {CALLBACK_BASE_URL}/api/v1/payments/browser-post/callback

CALLBACK_BASE_URL=http://localhost:8081

# For production:
# CALLBACK_BASE_URL=https://yourdomain.com

# =============================================================================
# Optional: Webhook Configuration
# =============================================================================
# For outbound webhooks to notify external systems of payment events

WEBHOOK_SECRET=your-webhook-hmac-secret     # HMAC secret for webhook signatures
WEBHOOK_RETRY_ATTEMPTS=3                    # Number of retry attempts
WEBHOOK_TIMEOUT_SECONDS=30                  # HTTP timeout for webhook delivery
```

### Production Environment Variables

For production deployment (Google Cloud Run, AWS ECS, Kubernetes, etc.):

```bash
# Database (use managed PostgreSQL)
DATABASE_URL=postgres://user:pass@production-db-host:5432/payments?sslmode=require

# EPX Production URLs
EPX_API_URL=https://api.epx.com
EPX_BROWSER_POST_URL=https://services.epx.com/browserpost/
EPX_KEY_EXCHANGE_URL=https://services.epx.com/keyexchange/

# Production Credentials (use EPX production merchant account)
EPX_CUST_NBR=<production-customer-number>
EPX_MERCH_NBR=<production-merchant-number>
EPX_DBA_NBR=<production-dba-number>
EPX_TERMINAL_NBR=<production-terminal-number>
EPX_MAC_SECRET=<production-mac-secret>

# Production Callback URL
CALLBACK_BASE_URL=https://api.yourdomain.com

# Security
TLS_CERT_PATH=/etc/ssl/certs/server.crt
TLS_KEY_PATH=/etc/ssl/private/server.key
```

**Security Best Practices:**
- ✅ **Never commit `.env` to version control**
- ✅ Use secret management services (AWS Secrets Manager, GCP Secret Manager, Vault)
- ✅ Rotate MAC_SECRET periodically
- ✅ Use TLS in production
- ✅ Restrict database access by IP

## Secret Manager Configuration

The payment service supports multiple secret management backends for storing sensitive credentials (EPX MAC secrets). Choose the backend that matches your deployment environment:

### Available Backends

| Backend | Use Case | Complexity |
|---------|----------|------------|
| **Mock** | Local development, testing | ⭐ Very Easy |
| **Local Files** | Local development with real credentials | ⭐ Easy |
| **GCP Secret Manager** | Production on Google Cloud | ⭐⭐ Moderate |
| **AWS Secrets Manager** | Production on AWS | ⭐⭐ Moderate |
| **HashiCorp Vault** | Enterprise, multi-cloud | ⭐⭐⭐ Complex |

### Mock Secret Manager (Development)

**Use for:** Local development, automated testing

```bash
# .env
SECRET_MANAGER=mock
```

Returns hardcoded test values. No additional configuration required.

### Local File-Based Secrets (Development)

**Use for:** Local development with real EPX credentials

```bash
# 1. Create secrets directory
mkdir -p secrets/epx/staging
chmod 700 secrets/

# 2. Store MAC secret
echo "your-epx-mac-secret" > secrets/epx/staging/mac_secret
chmod 600 secrets/epx/staging/mac_secret

# 3. Configure .env
SECRET_MANAGER=local
LOCAL_SECRETS_BASE_PATH=/absolute/path/to/secrets

# 4. Add to .gitignore
echo "secrets/" >> .gitignore
```

**Important:** Never commit the `secrets/` directory to version control.

### GCP Secret Manager (Production)

**Use for:** Production deployment on Google Cloud Platform

```bash
# 1. Enable Secret Manager API
export GCP_PROJECT_ID="your-project-id"
gcloud services enable secretmanager.googleapis.com

# 2. Create service account
gcloud iam service-accounts create payment-service \
  --display-name="Payment Service"

# 3. Grant permissions
gcloud projects add-iam-policy-binding $GCP_PROJECT_ID \
  --member="serviceAccount:payment-service@${GCP_PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# 4. Create service account key (local dev only)
gcloud iam service-accounts keys create gcp-key.json \
  --iam-account=payment-service@${GCP_PROJECT_ID}.iam.gserviceaccount.com

# 5. Create MAC secret
echo -n "your-mac-secret" | \
  gcloud secrets create payment-service/merchants/acme-corp/mac \
    --data-file=- \
    --replication-policy="automatic"

# 6. Configure .env
SECRET_MANAGER=gcp
GCP_PROJECT_ID=your-project-id
GOOGLE_APPLICATION_CREDENTIALS=/path/to/gcp-key.json
SECRET_CACHE_TTL_MINUTES=5

# 7. Update database
# UPDATE merchants SET mac_secret_path = 'payment-service/merchants/acme-corp/mac' WHERE slug = 'acme-corp';
```

**Production:** Use Workload Identity in GKE (no key file needed).

### AWS Secrets Manager (Production)

**Use for:** Production deployment on Amazon Web Services

```bash
# 1. Create IAM policy
cat > payment-secrets-policy.json <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "secretsmanager:GetSecretValue",
      "secretsmanager:DescribeSecret"
    ],
    "Resource": "arn:aws:secretsmanager:*:*:secret:payment-service/*"
  }]
}
EOF

aws iam create-policy \
  --policy-name PaymentServiceSecretsReadOnly \
  --policy-document file://payment-secrets-policy.json

# 2. Create IAM role (for EC2/ECS/EKS)
aws iam create-role \
  --role-name PaymentServiceRole \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": {"Service": "ec2.amazonaws.com"},
      "Action": "sts:AssumeRole"
    }]
  }'

# 3. Attach policy to role
aws iam attach-role-policy \
  --role-name PaymentServiceRole \
  --policy-arn arn:aws:iam::YOUR_ACCOUNT_ID:policy/PaymentServiceSecretsReadOnly

# 4. Create MAC secret
aws secretsmanager create-secret \
  --name payment-service/merchants/acme-corp/mac \
  --description "EPX MAC secret for ACME Corp" \
  --secret-string "your-mac-secret" \
  --region us-east-1

# 5. Configure .env
SECRET_MANAGER=aws
AWS_REGION=us-east-1
SECRET_CACHE_TTL_MINUTES=5
# Production: No AWS credentials needed - uses IAM role
# Local dev: Add AWS_PROFILE=my-dev-profile

# 6. Update database
# UPDATE merchants SET mac_secret_path = 'payment-service/merchants/acme-corp/mac' WHERE slug = 'acme-corp';
```

### HashiCorp Vault (Enterprise)

**Use for:** Enterprise deployments, multi-cloud, strict compliance

```bash
# 1. Enable KV secrets engine
export VAULT_ADDR="https://vault.yourcompany.com:8200"
vault login
vault secrets enable -path=secret kv-v2

# 2. Create policy
cat > payment-service-policy.hcl <<EOF
path "secret/data/payment-service/*" {
  capabilities = ["read"]
}
EOF

vault policy write payment-service payment-service-policy.hcl

# 3. Configure authentication (choose one)

# Option A: Token Auth (Development)
vault token create -policy=payment-service -ttl=24h

# Option B: AppRole Auth (Production)
vault auth enable approle
vault write auth/approle/role/payment-service \
  token_policies="payment-service" \
  token_ttl=1h

vault read auth/approle/role/payment-service/role-id
vault write -f auth/approle/role/payment-service/secret-id

# 4. Store MAC secret
vault kv put secret/payment-service/merchants/acme-corp/mac \
  value="your-mac-secret"

# 5. Configure .env (Token Auth)
SECRET_MANAGER=vault
VAULT_ADDR=https://vault.yourcompany.com:8200
VAULT_AUTH_METHOD=token
VAULT_TOKEN=s.xxxxxxxxxxxxx
VAULT_MOUNT_PATH=secret
VAULT_KV_VERSION=v2

# OR configure .env (AppRole Auth)
SECRET_MANAGER=vault
VAULT_ADDR=https://vault.yourcompany.com:8200
VAULT_AUTH_METHOD=approle
VAULT_ROLE_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
VAULT_SECRET_ID=yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy
VAULT_MOUNT_PATH=secret
VAULT_KV_VERSION=v2

# 6. Update database
# UPDATE merchants SET mac_secret_path = 'payment-service/merchants/acme-corp/mac' WHERE slug = 'acme-corp';
```

### Environment Variables Reference

| Variable | Required | Values | Description |
|----------|----------|--------|-------------|
| `SECRET_MANAGER` | Yes | `mock`, `local`, `gcp`, `aws`, `vault` | Secret manager backend |
| `LOCAL_SECRETS_BASE_PATH` | If `local` | path | Absolute path to secrets directory |
| `GCP_PROJECT_ID` | If `gcp` | string | GCP project ID |
| `GOOGLE_APPLICATION_CREDENTIALS` | If `gcp` | path | Path to GCP service account JSON |
| `AWS_REGION` | If `aws` | string | AWS region (e.g., `us-east-1`) |
| `AWS_PROFILE` | No | string | AWS profile name (local dev only) |
| `VAULT_ADDR` | If `vault` | URL | Vault server address |
| `VAULT_AUTH_METHOD` | If `vault` | `token`, `approle`, `kubernetes` | Auth method |
| `VAULT_TOKEN` | If token auth | string | Vault token |
| `VAULT_ROLE_ID` | If approle | string | AppRole role ID |
| `VAULT_SECRET_ID` | If approle | string | AppRole secret ID |
| `VAULT_MOUNT_PATH` | If `vault` | string | KV mount path (default: `secret`) |
| `VAULT_KV_VERSION` | If `vault` | `v1`, `v2` | KV version (default: `v2`) |
| `SECRET_CACHE_TTL_MINUTES` | No | int | Cache TTL in minutes (default: 5) |

### Testing Secret Manager Configuration

```bash
# Start payment service and check logs
go run cmd/server/main.go

# Look for initialization message:
# - "Mock secret manager initialized" (mock)
# - "GCP Secret Manager initialized successfully" (gcp)
# - "AWS Secrets Manager initialized successfully" (aws)
# - "Vault adapter initialized successfully" (vault)

# Run integration tests
go test -v ./tests/integration/merchant/... -run TestMerchantSecretRetrieval
```

## Running Integration Tests

### Setup

Integration tests require a running payment server and PostgreSQL database.

```bash
# 1. Start the full stack
docker-compose up -d

# Wait for services to be ready
sleep 5

# 2. Verify services are running
curl http://localhost:8081/cron/health

# 3. Run integration tests
EPX_MAC_STAGING="$(cat secrets/epx/staging/mac_secret)" \
SERVICE_URL="http://localhost:8081" \
go test -tags=integration -v ./tests/integration/payment/ -timeout 15m
```

### Test Suites

**Phase 1: Critical Business Logic (5 tests)**
```bash
# Tests the 5 most critical payment scenarios
go test -tags=integration -v ./tests/integration/payment/ \
  -run "TestBrowserPostIdempotency|TestRefundAmountValidation|TestCaptureStateValidation|TestConcurrentOperationHandling|TestEPXDeclineCodeHandling" \
  -timeout 15m
```

**Server Post Idempotency (5 tests)**
```bash
# Tests idempotency for Refund, Void, Capture
go test -tags=integration -v ./tests/integration/payment/ \
  -run "TestServerPostIdempotency" \
  -timeout 10m
```

**Browser Post Workflow (3 tests)**
```bash
# Tests complete Browser Post flows
go test -tags=integration -v ./tests/integration/payment/ \
  -run "TestBrowserPostWorkflow" \
  -timeout 15m
```

### Running Tests with ngrok (Browser Post Callbacks)

For Browser Post tests that require EPX to call back to your local machine:

```bash
# 1. Install ngrok
brew install ngrok  # macOS
# or download from https://ngrok.com/download

# 2. Start ngrok tunnel
ngrok http 8081

# Output:
# Forwarding  https://abc123.ngrok.io -> http://localhost:8081

# 3. Run tests with ngrok URL
SERVICE_URL="https://abc123.ngrok.io" \
EPX_MAC_STAGING="$(cat secrets/epx/staging/mac_secret)" \
go test -tags=integration -v ./tests/integration/payment/ -timeout 15m
```

## Troubleshooting

### Common Issues

#### 1. Database Connection Failed

**Error:**
```
Error: pq: password authentication failed for user "postgres"
```

**Solution:**
```bash
# Check PostgreSQL is running
docker ps | grep postgres

# Check DATABASE_URL in .env
cat .env | grep DATABASE_URL

# Restart PostgreSQL
docker-compose restart postgres
```

#### 2. EPX Authentication Failed (Code 58)

**Error:**
```
EPX Error: Code 58 - Authentication Failed
```

**Solution:**
- ✅ Verify `EPX_MAC_SECRET` matches EPX credentials
- ✅ Check `EPX_CUST_NBR`, `EPX_MERCH_NBR`, `EPX_DBA_NBR`, `EPX_TERMINAL_NBR`
- ✅ Ensure you're using UAT credentials for sandbox: `EPX_API_URL=https://api.epxuap.com`
- ✅ Contact EPX to verify credentials are active

#### 3. Browser Post Callback Not Received

**Error:**
```
Integration test timeout waiting for callback
```

**Solution:**
```bash
# 1. Check server is running and accessible
curl http://localhost:8081/cron/health

# 2. If using ngrok, verify tunnel is active
curl https://abc123.ngrok.io/cron/health

# 3. Check callback URL in EPX configuration matches CALLBACK_BASE_URL
echo $CALLBACK_BASE_URL

# 4. View server logs for callback requests
docker-compose logs -f payment-server | grep callback
```

#### 4. Integration Tests Failing with "Chrome not found"

**Error:**
```
Error: chrome executable not found
```

**Solution:**
```bash
# Install Chrome/Chromium

# macOS
brew install --cask google-chrome

# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y chromium-browser

# Fedora
sudo dnf install -y chromium
```

#### 5. Migration Errors

**Error:**
```
Error: goose: no such table: goose_db_version
```

**Solution:**
```bash
# Re-run migrations
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"
goose -dir internal/db/migrations postgres "$DATABASE_URL" up

# Or rebuild Docker containers
docker-compose down -v
docker-compose up -d
```

## Next Steps

After setup is complete:

**For Client Developers (integrating with the payment service):**
1. **Integration Guide**: [INTEGRATION_GUIDE.md](INTEGRATION_GUIDE.md) - Step-by-step API integration
2. **API Documentation**: [API_SPECS.md](API_SPECS.md) - Complete endpoint reference
3. **Payment Flows**: [DATAFLOW.md](DATAFLOW.md) - Understand payment workflows

**For Service Operators:**
1. **Testing Strategy**: [INTEGRATION_TEST_STRATEGY.md](INTEGRATION_TEST_STRATEGY.md)
2. **Production Deployment**: [GCP_PRODUCTION_SETUP.md](GCP_PRODUCTION_SETUP.md)
3. **FAQ**: Check common questions and answers
