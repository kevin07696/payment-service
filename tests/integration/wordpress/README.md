# WordPress Plugin Integration Testing Framework

This directory contains automated tests for the WordPress WooCommerce North Payments plugin.

## Overview

The testing framework provides:
1. **Manual verification scripts** - Interactive checkout testing with API verification
2. **Automated test framework** - Table-driven tests for WordPress admin operations
3. **Payment Service API integration** - All tests verify results via payment service APIs (not database queries)

## Current Status

### ✅ Completed
- Verification script using payment service APIs (`/tmp/verify_checkout_via_api.go`)
- Test framework structure (`admin_operations_test.go`)
- JWT authentication helpers
- API client setup
- Test case definitions for all admin operations

### 🚧 In Progress
- WordPress checkout automation
- WordPress admin UI automation
- API verification implementations

## Quick Start

### 1. Manual Verification Script

Test WordPress checkout and verify transactions via payment service API:

```bash
cd /home/kevinlam/Documents/projects/payments
go run /tmp/verify_checkout_via_api.go
```

This script will:
1. Open WordPress in a browser
2. Wait for you to complete a checkout manually
3. List recent transactions via payment service API
4. Show transaction details via API
5. Open WordPress admin transactions page for visual verification

**Manual steps:**
1. Login to WordPress
2. Add a product to cart
3. Go to checkout
4. Fill in billing details
5. Select North Payments
6. Fill card details: `4111111111111111`, `12/25`, `123`, `12345`
7. Click "Place Order"
8. Press ENTER in terminal when complete

### 2. Automated Tests (Framework Ready)

The test framework structure is in place at:
`tests/integration/wordpress/admin_operations_test.go`

**Test cases defined:**
1. ✅ Bulk Capture - 2 AUTH transactions
2. ✅ Bulk Refund - 2 SALE transactions
3. ✅ Partial Capture - 1 AUTH transaction ($100 auth → $50 capture)
4. ✅ Partial Refund - 1 SALE transaction ($200 sale → $75 refund)
5. ✅ SALE and Full Refund
6. ✅ AUTH and Void

**Run tests (once implementation is complete):**
```bash
cd /home/kevinlam/Documents/projects/payments
go test -v -tags=integration ./tests/integration/wordpress/... -run TestWordPressAdminOperations
```

## Architecture

### Authentication
All tests use JWT authentication with RS256 signing:
- Private key: test-service-001 private key
- Merchant ID: 00000000-0000-0000-0000-000000000001
- Service ID: test-service-001

### API Clients
Tests act as consumers of the payment service API:
- ConnectRPC client at `http://localhost:8080`
- JWT bearer token auth
- No direct database access

### Test Flow
1. **Setup**: Create transactions via WordPress checkout
2. **Verify Setup**: Confirm transactions exist via `GetTransaction` API
3. **Perform Operation**: Use WordPress admin UI (capture/refund/void)
4. **Verify Result**: Check operation results via payment service API

## Test Data

### Transaction Setup Requirements

| Test Case                | Setup Transactions     | Operation             | Verification                    |
|-------------------------|------------------------|-----------------------|---------------------------------|
| Bulk Capture            | 2 × AUTH ($50, $75)    | Bulk capture both     | 2 CAPTURE children created      |
| Bulk Refund             | 2 × SALE ($100, $150)  | Bulk refund both      | 2 REFUND children created       |
| Partial Capture         | 1 × AUTH ($100)        | Capture $50           | CAPTURE child with $50          |
| Partial Refund          | 1 × SALE ($200)        | Refund $75            | REFUND child with $75           |
| SALE + Full Refund      | 1 × SALE ($125)        | Refund full amount    | REFUND child matching SALE      |
| AUTH + Void             | 1 × AUTH ($99)         | Void                  | VOID child created              |

**Total transactions needed**: 7 transactions (4 AUTH + 3 SALE)

## Implementation Checklist

### Phase 1: Checkout Automation ⏳
- [ ] Implement `performCheckout()` function
  - [ ] Navigate to product page
  - [ ] Add product to cart
  - [ ] Go to checkout
  - [ ] Fill billing details
  - [ ] Select North Payments
  - [ ] Fill card details
  - [ ] Submit order
  - [ ] Extract transaction ID from response

### Phase 2: Admin UI Automation ⏳
- [ ] Implement `performBulkCapture()`
  - [ ] Select transaction checkboxes
  - [ ] Choose "Capture" from bulk actions
  - [ ] Click Apply
  - [ ] Confirm action

- [ ] Implement `performBulkRefund()`
- [ ] Implement `performPartialCapture()`
- [ ] Implement `performPartialRefund()`
- [ ] Implement `performFullRefund()`
- [ ] Implement `performVoid()`

### Phase 3: API Verification ⏳
- [ ] Implement `verifyBulkCapture()`
  - [ ] Query parent AUTH transactions via API
  - [ ] Verify CAPTURE children exist
  - [ ] Verify CAPTURE amounts match

- [ ] Implement `verifyBulkRefund()`
- [ ] Implement `verifyPartialCapture()`
- [ ] Implement `verifyPartialRefund()`
- [ ] Implement `verifyFullRefund()`
- [ ] Implement `verifyVoid()`

## WordPress Admin UI Selectors

To implement admin UI automation, you'll need to identify:
- Transaction list table selector
- Checkbox selectors for transaction rows
- Bulk action dropdown selector
- Capture/Refund/Void button selectors
- Modal/form selectors for partial amounts

**Recommended approach:**
1. Inspect WordPress admin transactions page HTML
2. Document CSS selectors or XPath
3. Update automation functions with correct selectors

## API Reference

### List Transactions
```go
req := &paymentv1.ListTransactionsRequest{
    MerchantId: merchantID,
    Limit:      100,
}
resp, err := client.ListTransactions(ctx, connect.NewRequest(req))
```

### Get Transaction
```go
req := &paymentv1.GetTransactionRequest{
    TransactionId: txID,
}
resp, err := client.GetTransaction(ctx, connect.NewRequest(req))
```

### Capture
```go
req := &paymentv1.CaptureRequest{
    TransactionId: authTxID,
    AmountCents:   5000, // Optional: for partial capture
}
resp, err := client.Capture(ctx, connect.NewRequest(req))
```

### Refund
```go
req := &paymentv1.RefundRequest{
    TransactionId: saleTxID,
    AmountCents:   7500, // Optional: for partial refund
    Reason:        "Customer request",
}
resp, err := client.Refund(ctx, connect.NewRequest(req))
```

### Void
```go
req := &paymentv1.VoidRequest{
    TransactionId: authTxID,
}
resp, err := client.Void(ctx, connect.NewRequest(req))
```

## Transaction Relationships

Transactions form parent-child relationships:

```
AUTH (parent_transaction_id: NULL)
  └─> CAPTURE (parent_transaction_id: AUTH.id)
      └─> REFUND (parent_transaction_id: CAPTURE.id)

SALE/CHARGE (parent_transaction_id: NULL)
  └─> REFUND (parent_transaction_id: SALE.id)

AUTH (parent_transaction_id: NULL)
  └─> VOID (parent_transaction_id: AUTH.id)
```

## Debugging

### Check running tests
```bash
# List background processes
/tasks

# Check specific test output
# (from Claude Code output)
```

### Verify WordPress is running
```bash
podman-compose ps
# Should show north-payments-wp running
```

### Verify payment service is running
```bash
podman ps --filter "name=payment"
# Should show payment-server running
```

### Test payment service API directly
```bash
curl http://localhost:8080/payment.v1.PaymentService/ListTransactions \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"merchant_id":"00000000-0000-0000-0000-000000000001","limit":5}'
```

## Contributing

When implementing automation:
1. Follow existing test patterns in `tests/integration/payment/`
2. Use chromedp for browser automation
3. Always verify via payment service APIs, never direct DB access
4. Add comprehensive logging (`t.Logf()`)
5. Handle errors gracefully with `require.NoError()`

## Related Files

- `/tmp/verify_checkout_via_api.go` - Manual verification script
- `tests/integration/wordpress/wordpress_checkout_test.go` - WordPress checkout test (basic)
- `tests/integration/wordpress/admin_operations_test.go` - Admin operations test framework
- `tests/integration/testutil/browser_post_automated.go` - Browser automation helpers
