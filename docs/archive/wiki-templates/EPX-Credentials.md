# EPX Credentials Guide

Complete guide to obtaining and configuring EPX (North) payment gateway credentials for development and production.

## What is EPX?

**EPX** (formerly Element Payment Services, now part of North) is the payment gateway that processes credit card and ACH transactions. This service integrates with EPX to handle payments securely.

- **Website**: [https://www.north.com](https://www.north.com) / [https://developer.north.com](https://developer.north.com)
- **Type**: Payment Gateway / Processor
- **Integration**: Server Post API, Browser Post API, BRIC Storage

## Getting EPX Credentials

### Step 1: Contact EPX Sales

You need a merchant account to use EPX services.

**Contact Information:**
- **Sales Email**: sales@epxuap.com (or contact through [north.com](https://www.north.com/contact))
- **Phone**: Available through North sales team
- **Website Form**: [https://www.north.com/contact](https://www.north.com/contact)

**What to Tell Them:**

"I need a merchant account for credit card processing with the following features:
- **Server Post API** for backend integration (transaction types: CCE1, CCE2, CCE3, CCE5, CCE6)
- **Browser Post API** for PCI-compliant frontend tokenization
- **BRIC Storage** (CCE8/CKC8) for saved payment methods and recurring billing
- **Key Exchange API** for TAC token generation
- **Sandbox/UAT environment** for development and testing
- **Production environment** when ready to go live"

### Step 2: Account Setup

EPX will guide you through:

1. **Merchant Application** - Business information, processing volume estimates
2. **Underwriting** - Credit check, business verification
3. **Pricing Agreement** - Transaction fees, monthly fees, gateway costs
4. **Technical Setup** - API credentials provisioned

**Timeline:** 1-2 weeks for approval, 1-2 days for credential provisioning

### Step 3: Receive Credentials

EPX will provide you with credentials for both **Sandbox (UAT)** and **Production** environments.

#### Sandbox/UAT Credentials (for development)

```bash
# Merchant Identification
CUST_NBR=9001                    # Customer number (merchant ID)
MERCH_NBR=900300                 # Merchant number (location/store ID)
DBA_NBR=2                        # DBA number (business name ID)
TERMINAL_NBR=77                  # Terminal number (POS device ID)

# Authentication Secret
MAC_SECRET=abc123def456...       # HMAC-SHA256 secret key (keep this secure!)

# API Endpoints
EPX_API_URL=https://api.epxuap.com
EPX_BROWSER_POST_URL=https://services.epxuap.com/browserpost/
EPX_KEY_EXCHANGE_URL=https://services.epxuap.com/keyexchange/
```

#### Production Credentials (for live transactions)

```bash
# Same structure, different values
CUST_NBR=<your-production-cust-nbr>
MERCH_NBR=<your-production-merch-nbr>
DBA_NBR=<your-production-dba-nbr>
TERMINAL_NBR=<your-production-terminal-nbr>
MAC_SECRET=<your-production-mac-secret>

# Production URLs
EPX_API_URL=https://api.epx.com
EPX_BROWSER_POST_URL=https://services.epx.com/browserpost/
EPX_KEY_EXCHANGE_URL=https://services.epx.com/keyexchange/
```

## Understanding the Credentials

### CUST_NBR (Customer Number)
- Identifies your overall merchant account
- Same across all locations/terminals
- Used in all API calls

### MERCH_NBR (Merchant Number)
- Identifies specific location or business entity
- Can have multiple merchant numbers under one customer number
- For multi-location businesses

### DBA_NBR (DBA Number)
- "Doing Business As" identifier
- Determines what appears on customer's credit card statement
- Usually `1` or `2` for most merchants

### TERMINAL_NBR (Terminal Number)
- Identifies specific point-of-sale terminal
- For online payments, usually a fixed value (e.g., `77` or `1`)
- Can have multiple terminals per merchant

### MAC_SECRET (HMAC Secret)
- **Critical security credential** - keep this SECRET!
- Used to sign all API requests with HMAC-SHA256
- Prevents unauthorized access to your merchant account
- Rotate periodically for security

**⚠️ NEVER commit MAC_SECRET to version control!**

## Configuring the Payment Service

### Local Development (.env file)

Create `.env` file in project root:

```bash
# =============================================================================
# EPX Sandbox/UAT Credentials
# =============================================================================
EPX_CUST_NBR=9001
EPX_MERCH_NBR=900300
EPX_DBA_NBR=2
EPX_TERMINAL_NBR=77
EPX_MAC_SECRET=your-sandbox-mac-secret-here

EPX_API_URL=https://api.epxuap.com
EPX_BROWSER_POST_URL=https://services.epxuap.com/browserpost/
EPX_KEY_EXCHANGE_URL=https://services.epxuap.com/keyexchange/

# =============================================================================
# Database
# =============================================================================
DATABASE_URL=postgres://postgres:postgres@localhost:5432/payments?sslmode=disable

# =============================================================================
# Server Configuration
# =============================================================================
SERVICE_URL=http://localhost:8081
CALLBACK_BASE_URL=http://localhost:8081
```

### Production Deployment

**Option 1: Environment Variables (Recommended)**

Set environment variables in your deployment platform:

```bash
# Google Cloud Run / Cloud Functions
gcloud run services update payment-service \
  --set-env-vars="EPX_CUST_NBR=<prod-cust-nbr>,EPX_MERCH_NBR=<prod-merch-nbr>,..." \
  --set-secrets="EPX_MAC_SECRET=epx-mac-secret:latest"

# AWS ECS / Fargate
aws ecs update-service \
  --cluster payment-cluster \
  --service payment-service \
  --task-definition payment-task:latest

# Kubernetes Secret
kubectl create secret generic epx-credentials \
  --from-literal=cust-nbr=<prod-cust-nbr> \
  --from-literal=merch-nbr=<prod-merch-nbr> \
  --from-literal=mac-secret=<prod-mac-secret>
```

**Option 2: Secret Management Service**

Use managed secret services for production:

- **AWS Secrets Manager**
- **GCP Secret Manager** (see [GCP Production Setup](GCP-PRODUCTION-SETUP))
- **Azure Key Vault**
- **HashiCorp Vault**

Example (AWS Secrets Manager):

```bash
# Store EPX credentials
aws secretsmanager create-secret \
  --name payment-service/epx \
  --description "EPX production credentials" \
  --secret-string '{
    "cust_nbr": "1234",
    "merch_nbr": "567890",
    "dba_nbr": "1",
    "terminal_nbr": "1",
    "mac_secret": "your-secret-here"
  }'

# Application retrieves at runtime
export EPX_MAC_SECRET=$(aws secretsmanager get-secret-value \
  --secret-id payment-service/epx \
  --query SecretString \
  --output text | jq -r .mac_secret)
```

## Browser Post Callback Configuration

When using Browser Post (PCI-compliant frontend tokenization), EPX needs to know where to redirect users after payment processing.

### Local Development (with ngrok)

```bash
# 1. Start ngrok tunnel
ngrok http 8081

# Output:
# Forwarding  https://abc123.ngrok.io -> http://localhost:8081

# 2. Set callback URL in .env
CALLBACK_BASE_URL=https://abc123.ngrok.io

# 3. EPX will redirect to:
# https://abc123.ngrok.io/api/v1/payments/browser-post/callback
```

### Production

```bash
# Set to your production domain
CALLBACK_BASE_URL=https://api.yourdomain.com

# EPX redirects to:
# https://api.yourdomain.com/api/v1/payments/browser-post/callback
```

**Important:**
- Callback URL must be HTTPS in production
- EPX sends payment results as POST request to this URL
- Your server must be publicly accessible

## Testing Your Credentials

### 1. Verify Credentials Work

```bash
# Start the service
docker-compose up -d

# Check health (verifies database connection)
curl http://localhost:8081/cron/health

# Test EPX connection with integration test
EPX_MAC_STAGING="$(cat secrets/epx/staging/mac_secret)" \
SERVICE_URL="http://localhost:8081" \
go test -tags=integration -v ./tests/integration/payment/ \
  -run TestBrowserPostIdempotency \
  -timeout 5m
```

### 2. Test Card Numbers (Sandbox Only)

EPX provides test cards for sandbox testing:

**Approval Card (always approves):**
```
Card Number: 4111111111111111
CVV: 123
Exp Date: 12/25 (any future date)
ZIP: 12345
```

**Decline Card (triggers specific error codes):**
```
Card Number: 4000000000000002
CVV: 123
Exp Date: 12/25
ZIP: 12345

Amount Triggers (last 3 digits determine response code):
- $1.05 = Code 05 (Do Not Honor)
- $1.20 = Code 51 (Insufficient Funds)
- $1.54 = Code 54 (Expired Card)
- $1.82 = Code 82 (CVV Error)
```

See EPX documentation: "Response Code Triggers - Visa.pdf"

### 3. Common Error Codes

| Code | Meaning | Solution |
|------|---------|----------|
| 58 | Authentication Failed | Verify `MAC_SECRET` matches EPX credentials |
| 96 | System Error | Check EPX status, verify API URL is correct |
| 91 | Issuer/Switch Inoperative | EPX sandbox may be down, try again later |

## Credential Rotation (Security Best Practice)

Rotate your `MAC_SECRET` periodically:

1. **Contact EPX** to request new MAC_SECRET
2. **Update credentials** in your secret management system
3. **Deploy new version** with updated secret
4. **Verify** old secret is deactivated

**Recommended rotation schedule:**
- Development: Every 6 months
- Production: Every 3 months or immediately if compromised

## Support

**EPX Technical Support:**
- Email: support@epxuap.com (or through [developer.north.com](https://developer.north.com))
- Hours: Business hours (EST)
- Documentation: [EPX API Reference](EPX-API-REFERENCE)

**Common Support Requests:**
- Credential verification
- Enable BRIC Storage (CCE8/CKC8) in sandbox
- Production credential provisioning
- Transaction troubleshooting

## Next Steps

✅ **Credentials configured!** What's next:

1. **[Quick Start](Quick-Start)** - Get service running in 5 minutes
2. **[Complete Setup Guide](Setup-Guide)** - Detailed configuration
3. **[Browser Post Flow](DATAFLOW#browser-post-flow)** - PCI-compliant payments
4. **[Testing Guide](INTEGRATION-TEST-STRATEGY)** - Run integration tests
5. **[Production Deployment](GCP-PRODUCTION-SETUP)** - Deploy to Google Cloud

## Security Checklist

Before going to production:

- [ ] `MAC_SECRET` stored in secret management service (not `.env`)
- [ ] `.env` file added to `.gitignore`
- [ ] Credentials rotated from development defaults
- [ ] HTTPS enabled for all endpoints
- [ ] Callback URL uses production domain with TLS
- [ ] Database credentials secured
- [ ] IP allowlist configured (if required by EPX)
- [ ] Monitoring and alerting enabled
- [ ] Tested with EPX production sandbox first
