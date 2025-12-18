# Get Real BRIC for Integration Tests

This directory contains a manual tool to obtain real BRIC tokens from EPX for use in integration test fixtures.

## Why Do We Need This?

EPX Browser Post requires a real browser to complete the payment flow. Automated tools get rejected with "unrecoverable error". To test CAPTURE, VOID, and REFUND operations, we need real BRICs from EPX.

## BRIC Expiration

- **Financial BRICs** (from AUTH/SALE): Valid for **13-24 months**
- **Storage BRICs** (from tokenization): **Never expire**

This means we can collect a BRIC once and use it for months in our test fixtures!

## Steps to Get Real BRIC

### 1. Start Your Local Server

```bash
# Make sure containers are running
podman-compose up -d

# Check server is healthy
curl http://localhost:8081/cron/health
```

### 2. Expose Localhost with ngrok

EPX needs to call back to your server, so expose it publicly:

```bash
# Install ngrok if you don't have it
# Download from: https://ngrok.com/download

# Start ngrok tunnel
ngrok http 8081

# Copy the HTTPS URL (e.g., https://abc123.ngrok-free.app)
```

### 3. Open the Test Page

```bash
# Open in your browser
xdg-open tests/manual/get_real_bric.html

# Or manually open:
# file:///home/kevinlam/Documents/projects/payments/tests/manual/get_real_bric.html
```

### 4. Fill in the Form

1. **Callback Base URL**: Paste your ngrok URL (e.g., `https://abc123.ngrok-free.app`)
2. **Transaction Type**: Select "AUTH" (for testing CAPTURE/VOID) or "SALE" (for testing REFUND)
3. **Amount**: Keep default $10.00 or change as needed
4. Click **"Get Form Config"**
5. Click **"Submit to EPX"**

### 5. EPX Processes Payment

- EPX will show payment form (already filled with test card)
- EPX processes the transaction
- EPX redirects back to your callback URL
- Your server receives the BRIC and stores it in database

### 6. Retrieve the BRIC

```bash
# Check database for the BRIC
podman exec payment-postgres psql -U postgres -d payment_service -c \
  "SELECT group_id, auth_guid, created_at FROM transaction_groups WHERE auth_guid IS NOT NULL ORDER BY created_at DESC LIMIT 1;"

# Output example:
#                group_id                |               auth_guid                |         created_at
# ---------------------------------------+----------------------------------------+----------------------------
#  12345678-1234-5678-1234-567812345678 | KBFEQxxxxxREALBRICFROMEPXxxxxxxxxx...  | 2025-11-12 16:30:00.123456
```

### 7. Copy BRIC to Fixtures

Copy the `auth_guid` value and add it to `tests/integration/fixtures/epx_brics.go` (we'll create this file next).

## Test Cards (EPX Sandbox)

Use these test cards in EPX sandbox:

- **Visa**: 4111111111111111
- **Mastercard**: 5454545454545454
- **Amex**: 378282246310005
- **Discover**: 6011111111111117

**Expiration**: 12/25 (MMYY format: 1225)
**CVV**: 123

## Troubleshooting

### "Form generation should succeed" - HTTP 500

**Problem**: Server can't find MAC secret

**Solution**:
```bash
# Create EPX staging secrets
mkdir -p secrets/epx/staging
cat secrets/payments/merchants/test-merchant-staging/mac > secrets/epx/staging/mac_secret

# Restart server
podman-compose restart payment-server
```

### ngrok URL Not Working

**Problem**: EPX can't reach ngrok URL

**Solution**:
- Make sure ngrok is running: `ngrok http 8081`
- Use the **HTTPS** URL (not HTTP)
- Check ngrok web interface: http://127.0.0.1:4040

### "Transaction not found" After Callback

**Problem**: Callback didn't create transaction

**Solution**:
- Check server logs: `podman logs payment-server`
- Verify merchant exists in database
- Check callback received: `podman logs payment-server | grep callback`

## Next Steps

After getting a real BRIC:
1. Add it to `tests/integration/fixtures/epx_brics.go`
2. Update integration tests to use the fixture
3. Tests will now use real BRICs without needing EPX calls!

## Security Note

⚠️ **Never commit real production BRICs to git!** These are sandbox test BRICs only.
