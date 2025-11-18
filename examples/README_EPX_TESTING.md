# EPX Sandbox Testing Guide

This guide explains how to test payments with real EPX sandbox credentials.

## Current Configuration

Your payment service is configured with EPX sandbox credentials:
- **Customer**: 9001
- **Merchant**: 900300
- **DBA**: 2
- **Terminal**: 77
- **Environment**: UAT (https://secure.epxuap.com)

## Step 1: Get a BRIC Token

EPX uses BRIC tokens for PCI-compliant card tokenization. To get a BRIC:

### Option A: Browser Form (Recommended)
1. Open the tokenization form:
   ```bash
   firefox examples/epx_tokenize.html
   ```

2. Use a test card:
   - **Approved**: 4788250000028291
   - **Declined**: 4000300011112220
   - **Exp**: 12/25
   - **CVV**: 123

3. Click "Generate Token (BRIC)"

4. A popup window will show the EPX response. Look for:
   - `<BRIC>...</BRIC>` or
   - `<AUTH_GUID>...</AUTH_GUID>`

5. Copy the token value

### Option B: Programmatic (For Testing)
```bash
go run examples/epx_tokenization.go
```
This attempts direct tokenization but may require additional EPX configuration.

## Step 2: Test a Payment

Once you have a BRIC token:

### Sale Transaction (Charge immediately)
```bash
go run examples/test_real_bric.go \
  -bric=YOUR_BRIC_HERE \
  -amount=10.00 \
  -op=sale
```

### Authorization (Hold funds)
```bash
go run examples/test_real_bric.go \
  -bric=YOUR_BRIC_HERE \
  -amount=10.00 \
  -op=auth
```

### List Recent Transactions
```bash
go run examples/test_real_bric.go -op=list
```

## Step 3: Test Payment Lifecycle

After a successful sale or auth, you can test:

### Refund (After Sale)
Save the `Group ID` from the sale response, then:
```bash
go run examples/test_refund.go \
  -group=GROUP_ID \
  -amount=5.00
```

### Capture (After Auth)
Save the `Transaction ID` from the auth response, then:
```bash
go run examples/test_capture.go \
  -tx=TRANSACTION_ID \
  -amount=10.00
```

### Void (Cancel Auth/Sale)
```bash
go run examples/test_void.go \
  -group=GROUP_ID
```

## Expected Results

### Successful Payment
```
✅ SALE SUCCESSFUL!
  Transaction ID: abc123...
  Group ID: def456...
  Status: TRANSACTION_STATUS_APPROVED
  Approved: true
  Amount: 10.00 USD
  Card: visa ending in 8291
  Message: Approved
  Auth Code: 123456
```

### Declined Payment
```
  Status: TRANSACTION_STATUS_DECLINED
  Approved: false
  Message: Insufficient Funds
```

## Test Cards

| Card Number | Brand | Result |
|-------------|-------|---------|
| 4788250000028291 | Visa | Approved |
| 4000300011112220 | Visa | Declined |
| 5454545454545454 | MasterCard | Approved |
| 371449635398431 | Amex | Approved |
| 6011000995500000 | Discover | Approved |

## Troubleshooting

### "Invalid BATCH_ID[LEN]"
This means the BRIC token is not valid. Get a fresh token from the HTML form.

### "Unrecoverable error" in HTML form
- Check that the MAC key is correct (2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y)
- Verify merchant credentials are active in EPX sandbox
- Try a different test card

### "Authentication failed"
- Ensure the WordPress service is registered
- Check that ACME merchant status is 'active'
- Verify JWT token generation

## Server Logs

Monitor server activity:
```bash
tail -f /tmp/server-sandbox.log
```

Look for:
- `Processing EPX Server Post transaction`
- `Sale transaction completed`
- Transaction numbers from EPX

## Integration Status

✅ **Working:**
- JWT authentication
- ConnectRPC communication
- EPX sandbox integration
- Transaction processing

⏳ **Pending:**
- Real BRIC tokens for approved transactions
- Full payment lifecycle testing
- Production deployment

## Files Reference

- `epx_tokenize.html` - Browser form for getting BRICs
- `test_real_bric.go` - Test payments with real BRICs
- `epx_tokenization.go` - Complete flow demonstration
- `wordpress_sandbox_demo.go` - WordPress integration test

## Next Steps

1. Get a real BRIC from the HTML form
2. Test an approved transaction
3. Test the full payment lifecycle
4. Monitor server logs for transaction details

The system is ready for real sandbox testing!