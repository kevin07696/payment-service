# Production Deployment Guide

## North Payment Gateway - Production Credentials Checklist

This guide outlines everything you need from North Payment Gateway to deploy this payment service to production.

## =Ë Required Credentials from North

### 1. Merchant Account Information

Contact North Payment Gateway support to obtain:

#### **EPI-Id (Enterprise Payment Integration ID)**
- **Format**: `CUST_NBR-MERCH_NBR-TERM_NBR-1`
- **Example**: `7000-700010-1-1`
- **Components**:
  - `CUST_NBR`: Customer number (e.g., "7000")
  - `MERCH_NBR`: Merchant number (e.g., "700010")
  - `TERM_NBR`: Terminal number (typically "1")
- **Environment**: You need **separate credentials** for:
  - Sandbox/Test environment
  - Production environment

#### **EPI-Key (HMAC Secret Key)**
- **Purpose**: Used for HMAC-SHA256 request signature authentication
- **Security**: This is a **secret key** - treat it like a password
- **Storage**: Store in secrets manager (AWS Secrets Manager, HashiCorp Vault, etc.)
- **Rotation**: Ask North about their key rotation policy

### 2. API Endpoint URLs

You'll need to know the correct base URLs for each environment:

**Sandbox/Test:**
```
https://sandbox.north.com/api/browserpost
```

**Production:**
```
https://api.north.com/api/browserpost
```

*Note: Confirm actual URLs with North - these may vary*

### 3. Additional Gateway Services

Depending on your needs, you may also need credentials for:

- **Recurring Billing Gateway** - for subscription payments
- **ACH Processing** - for bank account payments
- **CustomPay** - for alternative payment methods
- **Tokenization Service** - BRIC token management

## = Security Requirements

### SSL/TLS Configuration

North Payment Gateway requires:
- **TLS 1.2 or higher**
- Valid SSL certificates for your servers
- May require specific CA certificate pinning

### IP Whitelisting

- North may require your production server IPs to be whitelisted
- Provide them with:
  - Production server IP addresses
  - Webhook callback server IPs
  - Any NAT gateway or proxy IPs

### PCI DSS Compliance

 **Good News**: This implementation uses BRIC tokens, which means:
- No raw card data ever touches your servers
- Significantly reduces PCI scope
- You're handling tokenized references only

W **Still Required**:
- Secure storage of EPI-Key (secrets manager)
- HTTPS-only communication
- Proper logging (no sensitive data in logs)
- Access controls and audit trails

## =Ý Information to Provide North

When setting up your account, North will need:

### Business Information
- Legal business name
- Business address
- Tax ID / EIN
- Business type (Corporation, LLC, etc.)
- Industry/MCC code

### Technical Information
- Estimated transaction volume
- Average transaction size
- Supported payment methods (credit card, ACH, etc.)
- Integration method (Browser Post API)
- Expected go-live date

### Security & Compliance
- Webhook callback URLs (if using webhooks)
- Return/redirect URLs (for browser-based flows)
- Your data retention policies
- Your security/compliance certifications

## ™ Environment Configuration

### Required Environment Variables

```bash
# Production
export NORTH_BASE_URL="https://api.north.com/api/browserpost"
export NORTH_EPI_ID="YOUR-PRODUCTION-EPI-ID"
export NORTH_EPI_KEY="YOUR-PRODUCTION-SECRET-KEY"
export NORTH_TIMEOUT="30"

# Database (use managed PostgreSQL in production)
export DB_HOST="your-db-host.rds.amazonaws.com"
export DB_PORT="5432"
export DB_USER="payment_service"
export DB_PASSWORD="<from-secrets-manager>"
export DB_NAME="payment_service"
export DB_SSL_MODE="require"  # or "verify-full"
export DB_MAX_CONNS="25"
export DB_MIN_CONNS="5"

# Server
export SERVER_PORT="50051"
export SERVER_HOST="0.0.0.0"

# Logging
export LOG_LEVEL="info"
export LOG_DEVELOPMENT="false"
```

### Secrets Management

**DO NOT** hardcode credentials. Use a secrets manager:

**AWS Secrets Manager:**
```go
import "github.com/aws/aws-sdk-go/service/secretsmanager"

// Retrieve NORTH_EPI_KEY from AWS Secrets Manager
secret, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
    SecretId: aws.String("production/north/epi-key"),
})
```

**HashiCorp Vault:**
```bash
vault kv get -field=epi_key secret/north/production
```

**Kubernetes Secrets:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: north-credentials
type: Opaque
data:
  epi-id: <base64-encoded-epi-id>
  epi-key: <base64-encoded-epi-key>
```

## =€ Deployment Checklist

### Pre-Deployment

- [ ] Obtained production EPI-Id from North
- [ ] Obtained production EPI-Key from North
- [ ] Confirmed production API URL with North
- [ ] Set up secrets manager for credential storage
- [ ] Configured IP whitelisting with North
- [ ] Tested in sandbox environment successfully
- [ ] Reviewed PCI compliance requirements
- [ ] Set up SSL/TLS certificates
- [ ] Configured database with SSL mode enabled
- [ ] Set up monitoring and alerting (see below)
- [ ] Configured rate limiting
- [ ] Set up backup and disaster recovery

### Monitoring & Observability

Set up monitoring for:

**Transaction Metrics:**
- Authorization success/failure rates
- Average response times
- AVS/CVV match rates
- Declined transaction reasons
- Gateway timeout errors

**System Metrics:**
- API request rates
- Database connection pool utilization
- gRPC server health
- Error rates by error type

**Alerts:**
- Gateway downtime or connectivity issues
- Elevated error rates (> 1%)
- Slow transaction times (> 2s)
- Database connection pool exhaustion
- High memory or CPU usage

### Post-Deployment

- [ ] Run smoke tests against production
- [ ] Verify AVS/CVV responses are captured correctly
- [ ] Test idempotency key behavior
- [ ] Monitor initial transactions closely
- [ ] Verify logging excludes sensitive data
- [ ] Test rollback procedure
- [ ] Document incident response procedures
- [ ] Set up on-call rotation

## >ê Testing in Sandbox

Before going to production, thoroughly test with sandbox credentials:

### Test Scenarios

1. **Successful Authorization**
   - Full AVS match (Y)
   - Full CVV match (M)

2. **AVS Variations**
   - ZIP match only (Z)
   - Address match only (A)
   - No match (N)
   - Unavailable (U)

3. **CVV Variations**
   - Match (M)
   - No match (N)
   - Not processed (P)

4. **Error Handling**
   - Network timeouts
   - Invalid card numbers
   - Declined transactions
   - Gateway errors

5. **Edge Cases**
   - Idempotency key reuse
   - Partial captures
   - Refunds and voids
   - Concurrent transactions

## =Þ North Support Contacts

Make sure you have:
- Technical support email/phone
- Account manager contact
- Emergency escalation contacts
- Status page URL for monitoring outages

## = Credential Rotation

Establish a process for rotating EPI-Key:

1. Request new key from North
2. Update secrets manager with new key
3. Deploy updated configuration
4. Verify new key works
5. Notify North to deactivate old key
6. Monitor for any issues

**Recommended rotation frequency**: Every 90 days

## =Ê Production Readiness Scorecard

| Category | Requirement | Status |
|----------|-------------|--------|
| **Credentials** | Production EPI-Id obtained |  |
| | Production EPI-Key obtained |  |
| | Sandbox testing completed |  |
| **Security** | Secrets manager configured |  |
| | SSL/TLS enabled |  |
| | IP whitelisting configured |  |
| | PCI compliance reviewed |  |
| **Infrastructure** | Database SSL enabled |  |
| | Monitoring configured |  |
| | Alerting set up |  |
| | Logging configured (no PII) |  |
| | Backup/DR procedures |  |
| **Testing** | All test scenarios passed |  |
| | Load testing completed |  |
| | Rollback tested |  |
| **Operations** | Runbook created |  |
| | On-call rotation set up |  |
| | Incident procedures documented |  |

## <˜ Troubleshooting

### Common Issues

**"Invalid EPI-Id format"**
- Verify format is `CUST_NBR-MERCH_NBR-TERM_NBR-1`
- Check for extra spaces or special characters
- Confirm you're using production credentials, not sandbox

**"HMAC signature verification failed"**
- Ensure EPI-Key matches the environment (prod vs sandbox)
- Check for whitespace in the key
- Verify key hasn't been rotated by North

**"Connection refused"**
- Verify production URL is correct
- Check IP whitelist with North
- Confirm firewall rules allow outbound HTTPS

**"Transaction timeout"**
- Check network connectivity to North
- Verify NORTH_TIMEOUT is reasonable (30s)
- Review North's status page for outages

## =Ú Additional Resources

- [North API Guide](./NORTH_API_GUIDE.md)
- [AVS/CVV Response Codes](#avs-cvv-codes)
- [3DS Implementation](./3DS_IMPLEMENTATION.md)
- [Frontend Integration](./FRONTEND_INTEGRATION.md)

## AVS/CVV Response Codes

### AVS Codes
- **Y** - Address and ZIP match
- **N** - No match
- **Z** - ZIP matches, address doesn't
- **A** - Address matches, ZIP doesn't
- **U** - AVS unavailable
- **R** - Retry (system unavailable)

### CVV Codes
- **M** - Match
- **N** - No match
- **P** - Not processed
- **U** - Unavailable

---

**Need Help?** Contact North Payment Gateway support or consult their integration documentation.
