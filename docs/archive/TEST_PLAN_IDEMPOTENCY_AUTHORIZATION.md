# Test Plan: Idempotency and Authorization

**Version**: 1.0
**Last Updated**: 2025-01-12
**Status**: 📋 IN PROGRESS

---

## Overview

This document outlines required tests for idempotency and authorization features. These are **critical security and reliability features** that must be thoroughly tested.

---

## Test Coverage Status

### ✅ Existing Tests (idempotency_test.go)

| Test | Status | File |
|------|--------|------|
| Refund idempotency with client UUID | ✅ Exists | idempotency_test.go:19 |
| Multiple refunds with different UUIDs | ✅ Exists | idempotency_test.go:126 |
| Concurrent refunds with same UUID | ✅ Exists | idempotency_test.go:312 |
| Refund exceeds original amount | ⚠️ Validation gap noted | idempotency_test.go:234 |

### ❌ Missing Tests

#### Idempotency Tests Needed:
1. ❌ Sale idempotency (declined transaction blocks retry)
2. ❌ Authorize idempotency
3. ❌ Capture idempotency
4. ❌ Void idempotency
5. ❌ Network error NOT creating transaction
6. ❌ Declined vs approved idempotency behavior

#### Authorization Tests Needed:
1. ❌ Customer can only see own transactions
2. ❌ Guest access with session_id
3. ❌ Merchant isolation (cannot access other merchants)
4. ❌ Admin full access
5. ❌ Service account scopes
6. ❌ Return 404 instead of 403 for unauthorized
7. ❌ Forced filters (customer cannot override)
8. ❌ Rate limiting

---

## Test Suite 1: Idempotency Tests

### Test 1.1: Sale Idempotency - Approved Payment

**File**: `tests/integration/payment/idempotency_sale_test.go`

```go
func TestSale_Idempotency_Approved(t *testing.T) {
    client := testutil.Setup(t)

    // Generate idempotency key
    idempotencyKey := uuid.New().String()

    saleReq := &payment.SaleRequest{
        MerchantID: "test-merchant",
        Amount: "50.00",
        Currency: "USD",
        PaymentMethodID: savedCardID,
        IdempotencyKey: idempotencyKey,
    }

    // First attempt
    resp1, err := client.Sale(ctx, saleReq)
    require.NoError(t, err)
    assert.Equal(t, "approved", resp1.Status)
    txID1 := resp1.TransactionID

    // Retry with SAME idempotency key
    resp2, err := client.Sale(ctx, saleReq)
    require.NoError(t, err)

    // Should return SAME transaction (idempotent)
    assert.Equal(t, txID1, resp2.TransactionID)
    assert.Equal(t, "approved", resp2.Status)

    // Verify only ONE transaction created
    txs := getTransactionsByGroup(resp1.GroupID)
    assert.Equal(t, 1, len(txs))
}
```

---

### Test 1.2: Sale Idempotency - Declined Payment

**File**: `tests/integration/payment/idempotency_sale_test.go`

```go
func TestSale_Idempotency_Declined(t *testing.T) {
    client := testutil.Setup(t)

    // Use test card that will be declined
    declinedCardID := createDeclinedCard(t)
    idempotencyKey := uuid.New().String()

    saleReq := &payment.SaleRequest{
        MerchantID: "test-merchant",
        Amount: "50.00",
        Currency: "USD",
        PaymentMethodID: declinedCardID,
        IdempotencyKey: idempotencyKey,
    }

    // First attempt - declined
    resp1, err := client.Sale(ctx, saleReq)
    require.NoError(t, err)
    assert.Equal(t, "declined", resp1.Status)
    assert.False(t, resp1.IsApproved)
    txID1 := resp1.TransactionID

    // Retry with SAME idempotency key
    resp2, err := client.Sale(ctx, saleReq)
    require.NoError(t, err)

    // Should return SAME declined transaction
    assert.Equal(t, txID1, resp2.TransactionID)
    assert.Equal(t, "declined", resp2.Status)

    // Verify only ONE declined transaction created
    txs := getTransactionsByGroup(resp1.GroupID)
    assert.Equal(t, 1, len(txs))
    assert.Equal(t, "declined", txs[0].Status)

    // Now retry with NEW idempotency key and valid card
    validCardID := createValidCard(t)
    saleReq.PaymentMethodID = validCardID
    saleReq.IdempotencyKey = uuid.New().String() // NEW key

    resp3, err := client.Sale(ctx, saleReq)
    require.NoError(t, err)
    assert.Equal(t, "approved", resp3.Status)
    assert.NotEqual(t, txID1, resp3.TransactionID) // Different transaction
}
```

---

### Test 1.3: Network Error Does NOT Create Transaction

**File**: `tests/integration/payment/idempotency_network_error_test.go`

```go
func TestSale_NetworkError_NoTransactionCreated(t *testing.T) {
    client := testutil.Setup(t)
    idempotencyKey := uuid.New().String()

    // Mock gateway to return network error
    mockGateway := testutil.MockGatewayWithNetworkError()

    saleReq := &payment.SaleRequest{
        MerchantID: "test-merchant",
        Amount: "50.00",
        Currency: "USD",
        PaymentMethodID: savedCardID,
        IdempotencyKey: idempotencyKey,
    }

    // First attempt - network error
    _, err := client.Sale(ctx, saleReq)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "gateway error")

    // Verify NO transaction created in database
    tx, err := getTransactionByID(idempotencyKey)
    assert.Error(t, err) // Should not exist
    assert.Nil(t, tx)

    // Restore real gateway
    mockGateway.Restore()

    // Retry with SAME idempotency key - should succeed now
    resp, err := client.Sale(ctx, saleReq)
    require.NoError(t, err)
    assert.Equal(t, "approved", resp.Status)

    // Verify transaction created THIS time
    tx, err = getTransactionByID(idempotencyKey)
    require.NoError(t, err)
    assert.NotNil(t, tx)
    assert.Equal(t, idempotencyKey, tx.ID)
}
```

---

### Test 1.4: Capture Idempotency

**File**: `tests/integration/payment/idempotency_capture_test.go`

```go
func TestCapture_Idempotency(t *testing.T) {
    client := testutil.Setup(t)

    // Create authorization
    authResp, _ := client.Authorize(ctx, &payment.AuthorizeRequest{
        MerchantID: "test-merchant",
        Amount: "100.00",
        Currency: "USD",
        PaymentMethodID: savedCardID,
        IdempotencyKey: uuid.New().String(),
    })

    groupID := authResp.GroupID
    captureIdempotencyKey := uuid.New().String()

    captureReq := &payment.CaptureRequest{
        GroupID: groupID,
        Amount: "75.00", // Partial capture
        IdempotencyKey: captureIdempotencyKey,
    }

    // First capture
    cap1, err := client.Capture(ctx, captureReq)
    require.NoError(t, err)
    assert.Equal(t, "approved", cap1.Status)
    capTxID1 := cap1.TransactionID

    // Retry with SAME idempotency key
    cap2, err := client.Capture(ctx, captureReq)
    require.NoError(t, err)

    // Should return SAME capture transaction
    assert.Equal(t, capTxID1, cap2.TransactionID)
    assert.Equal(t, "75.00", cap2.Amount)

    // Verify group has 2 transactions: AUTH + 1 CAPTURE (not 3)
    txs := getTransactionsByGroup(groupID)
    assert.Equal(t, 2, len(txs))

    captureTxs := filterByType(txs, "capture")
    assert.Equal(t, 1, len(captureTxs))
}
```

---

### Test 1.5: Void Idempotency

**File**: `tests/integration/payment/idempotency_void_test.go`

```go
func TestVoid_Idempotency(t *testing.T) {
    client := testutil.Setup(t)

    // Create sale
    saleResp, _ := client.Sale(ctx, &payment.SaleRequest{
        MerchantID: "test-merchant",
        Amount: "50.00",
        Currency: "USD",
        PaymentMethodID: savedCardID,
        IdempotencyKey: uuid.New().String(),
    })

    groupID := saleResp.GroupID
    voidIdempotencyKey := uuid.New().String()

    voidReq := &payment.VoidRequest{
        GroupID: groupID,
        IdempotencyKey: voidIdempotencyKey,
    }

    // First void
    void1, err := client.Void(ctx, voidReq)
    require.NoError(t, err)
    assert.Equal(t, "approved", void1.Status)
    voidTxID1 := void1.TransactionID

    // Retry with SAME idempotency key
    void2, err := client.Void(ctx, voidReq)
    require.NoError(t, err)

    // Should return SAME void transaction
    assert.Equal(t, voidTxID1, void2.TransactionID)

    // Verify group has 2 transactions: SALE + 1 VOID (not 3)
    txs := getTransactionsByGroup(groupID)
    assert.Equal(t, 2, len(txs))

    voidTxs := filterByType(txs, "void")
    assert.Equal(t, 1, len(voidTxs))
}
```

---

## Test Suite 2: Authorization Tests

### Test 2.1: Customer Can Only See Own Transactions

**File**: `tests/integration/authorization/customer_authorization_test.go`

```go
func TestCustomer_CanOnlyAccessOwnTransactions(t *testing.T) {
    client := testutil.Setup(t)

    // Create transactions for two different customers
    customer1 := "cust_alice"
    customer2 := "cust_bob"

    tx1 := createSale(t, customer1, "50.00")
    tx2 := createSale(t, customer2, "75.00")

    // Customer 1 tries to access own transaction
    authCtx := &AuthContext{
        ActorType: ActorTypeCustomer,
        ActorID: customer1,
        CustomerID: &customer1,
    }

    resp1, err := client.GetTransactionsByGroupID(ctx, tx1.GroupID, authCtx)
    require.NoError(t, err)
    assert.NotNil(t, resp1)

    // Customer 1 tries to access Customer 2's transaction
    resp2, err := client.GetTransactionsByGroupID(ctx, tx2.GroupID, authCtx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not found") // 404, not 403!

    // Customer 1 tries to list transactions with merchant filter
    listReq := &payment.ListTransactionsRequest{
        MerchantID: "test-merchant", // Customer tries to specify merchant
        Limit: 100,
    }

    listResp, err := client.ListTransactions(ctx, listReq, authCtx)
    require.NoError(t, err)

    // All returned transactions should belong to customer1
    for _, tx := range listResp.Transactions {
        assert.Equal(t, customer1, tx.CustomerID)
    }
}
```

---

### Test 2.2: Guest Access With Session ID

**File**: `tests/integration/authorization/guest_authorization_test.go`

```go
func TestGuest_AccessWithSessionID(t *testing.T) {
    client := testutil.Setup(t)

    sessionID := "guest_session_abc123"

    // Create payment with session_id in metadata
    saleResp := createSaleWithMetadata(t, map[string]interface{}{
        "session_id": sessionID,
        "email_hash": hashEmail("guest@example.com"),
    })

    groupID := saleResp.GroupID

    // Guest with MATCHING session can access
    authCtx := &AuthContext{
        ActorType: ActorTypeGuest,
        SessionID: &sessionID,
    }

    resp1, err := client.GetTransactionsByGroupID(ctx, groupID, authCtx)
    require.NoError(t, err)
    assert.NotNil(t, resp1)

    // Guest with DIFFERENT session cannot access
    differentSession := "guest_session_xyz789"
    authCtx2 := &AuthContext{
        ActorType: ActorTypeGuest,
        SessionID: &differentSession,
    }

    resp2, err := client.GetTransactionsByGroupID(ctx, groupID, authCtx2)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not found") // 404, not 403!

    // Guest cannot list transactions
    _, err = client.ListTransactions(ctx, &payment.ListTransactionsRequest{}, authCtx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "permission denied")
}
```

---

### Test 2.3: Merchant Isolation

**File**: `tests/integration/authorization/merchant_authorization_test.go`

```go
func TestMerchant_Isolation(t *testing.T) {
    client := testutil.Setup(t)

    // Create transactions for two merchants
    merchant1 := "merch_store_a"
    merchant2 := "merch_store_b"

    tx1 := createSale(t, "cust_123", "50.00", merchant1)
    tx2 := createSale(t, "cust_456", "75.00", merchant2)

    // Merchant 1 can access own transaction
    authCtx1 := &AuthContext{
        ActorType: ActorTypeMerchant,
        ActorID: merchant1,
        MerchantID: &merchant1,
    }

    resp1, err := client.GetTransactionsByGroupID(ctx, tx1.GroupID, authCtx1)
    require.NoError(t, err)
    assert.NotNil(t, resp1)

    // Merchant 1 CANNOT access Merchant 2's transaction
    resp2, err := client.GetTransactionsByGroupID(ctx, tx2.GroupID, authCtx1)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not found")

    // Merchant 1 lists transactions
    listResp, err := client.ListTransactions(ctx, &payment.ListTransactionsRequest{
        Limit: 100,
    }, authCtx1)
    require.NoError(t, err)

    // All returned transactions should belong to merchant1
    for _, tx := range listResp.Transactions {
        assert.Equal(t, merchant1, tx.MerchantID)
    }

    // Merchant 1 tries to specify merchant2 in filter (should be ignored/overridden)
    listResp2, err := client.ListTransactions(ctx, &payment.ListTransactionsRequest{
        MerchantID: merchant2, // Merchant tries to access other merchant
        Limit: 100,
    }, authCtx1)
    require.NoError(t, err)

    // Filter should be FORCED to merchant1
    for _, tx := range listResp2.Transactions {
        assert.Equal(t, merchant1, tx.MerchantID)
    }
}
```

---

### Test 2.4: Admin Full Access

**File**: `tests/integration/authorization/admin_authorization_test.go`

```go
func TestAdmin_FullAccess(t *testing.T) {
    client := testutil.Setup(t)

    // Create transactions for different customers and merchants
    tx1 := createSale(t, "cust_1", "50.00", "merch_a")
    tx2 := createSale(t, "cust_2", "75.00", "merch_b")

    authCtx := &AuthContext{
        ActorType: ActorTypeAdmin,
        ActorID: "admin_user",
        Scopes: []string{"*:*"},
    }

    // Admin can access ANY transaction
    resp1, err := client.GetTransactionsByGroupID(ctx, tx1.GroupID, authCtx)
    require.NoError(t, err)
    assert.NotNil(t, resp1)

    resp2, err := client.GetTransactionsByGroupID(ctx, tx2.GroupID, authCtx)
    require.NoError(t, err)
    assert.NotNil(t, resp2)

    // Admin can list ALL transactions (no forced filters)
    listResp, err := client.ListTransactions(ctx, &payment.ListTransactionsRequest{
        Limit: 100,
    }, authCtx)
    require.NoError(t, err)
    assert.Greater(t, len(listResp.Transactions), 0)

    // Admin can filter by any merchant
    listRespMerchA, err := client.ListTransactions(ctx, &payment.ListTransactionsRequest{
        MerchantID: "merch_a",
        Limit: 100,
    }, authCtx)
    require.NoError(t, err)

    for _, tx := range listRespMerchA.Transactions {
        assert.Equal(t, "merch_a", tx.MerchantID)
    }

    // Admin can refund ANY transaction
    _, err = client.Refund(ctx, &payment.RefundRequest{
        GroupID: tx1.GroupID,
        Amount: "25.00",
        Reason: "Admin refund",
        IdempotencyKey: uuid.New().String(),
    }, authCtx)
    require.NoError(t, err)
}
```

---

### Test 2.5: Service Account Scopes

**File**: `tests/integration/authorization/service_account_authorization_test.go`

```go
func TestServiceAccount_Scopes(t *testing.T) {
    client := testutil.Setup(t)

    // Create service account with limited scopes
    svcAccount := createServiceAccount(t, &ServiceAccount{
        Name: "ecommerce-backend",
        AllowedMerchants: []string{"merch_ecom_1", "merch_ecom_2"},
        Scopes: []string{"create:payments", "read:transactions"},
    })

    authCtx := &AuthContext{
        ActorType: ActorTypeService,
        ActorID: svcAccount.ID,
        Scopes: svcAccount.Scopes,
    }

    // Service can create payments for allowed merchants
    saleResp, err := client.Sale(ctx, &payment.SaleRequest{
        MerchantID: "merch_ecom_1", // Allowed
        Amount: "50.00",
        Currency: "USD",
        PaymentMethodID: savedCardID,
        IdempotencyKey: uuid.New().String(),
    }, authCtx)
    require.NoError(t, err)

    // Service CANNOT create payments for non-allowed merchants
    _, err = client.Sale(ctx, &payment.SaleRequest{
        MerchantID: "merch_pos_1", // NOT allowed
        Amount: "50.00",
        Currency: "USD",
        PaymentMethodID: savedCardID,
        IdempotencyKey: uuid.New().String(),
    }, authCtx)
    assert.Error(t, err)

    // Service can read transactions for allowed merchants
    _, err = client.GetTransactionsByGroupID(ctx, saleResp.GroupID, authCtx)
    require.NoError(t, err)

    // Service CANNOT refund (not in scopes)
    _, err = client.Refund(ctx, &payment.RefundRequest{
        GroupID: saleResp.GroupID,
        Amount: "25.00",
        Reason: "Test refund",
        IdempotencyKey: uuid.New().String(),
    }, authCtx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "permission denied")
}
```

---

### Test 2.6: Return 404 Instead of 403

**File**: `tests/integration/authorization/error_handling_test.go`

```go
func TestAuthorization_Returns404Not403(t *testing.T) {
    client := testutil.Setup(t)

    // Create transaction for customer A
    customerA := "cust_alice"
    txA := createSale(t, customerA, "50.00")

    // Customer B tries to access Customer A's transaction
    customerB := "cust_bob"
    authCtx := &AuthContext{
        ActorType: ActorTypeCustomer,
        ActorID: customerB,
        CustomerID: &customerB,
    }

    _, err := client.GetTransactionsByGroupID(ctx, txA.GroupID, authCtx)
    assert.Error(t, err)

    // Should be "not found", NOT "permission denied"
    // This prevents enumeration attacks
    grpcErr, ok := status.FromError(err)
    require.True(t, ok)
    assert.Equal(t, codes.NotFound, grpcErr.Code())
    assert.Contains(t, grpcErr.Message(), "not found")
    assert.NotContains(t, grpcErr.Message(), "permission")
    assert.NotContains(t, grpcErr.Message(), "unauthorized")

    // Test with non-existent transaction too
    fakeGroupID := uuid.New().String()
    _, err2 := client.GetTransactionsByGroupID(ctx, fakeGroupID, authCtx)
    assert.Error(t, err2)

    grpcErr2, ok := status.FromError(err2)
    require.True(t, ok)
    assert.Equal(t, codes.NotFound, grpcErr2.Code())

    // Both errors should look identical (prevents enumeration)
    assert.Equal(t, grpcErr.Message(), grpcErr2.Message())
}
```

---

### Test 2.7: Forced Filters

**File**: `tests/integration/authorization/forced_filters_test.go`

```go
func TestCustomer_CannotOverrideFilters(t *testing.T) {
    client := testutil.Setup(t)

    customerA := "cust_alice"
    customerB := "cust_bob"

    // Create transactions for both customers
    createSale(t, customerA, "50.00")
    createSale(t, customerB, "75.00")

    authCtx := &AuthContext{
        ActorType: ActorTypeCustomer,
        ActorID: customerA,
        CustomerID: &customerA,
    }

    // Customer A tries to list transactions with customer_id=B filter
    listResp, err := client.ListTransactions(ctx, &payment.ListTransactionsRequest{
        CustomerID: customerB, // Customer tries to see other customer's data
        Limit: 100,
    }, authCtx)
    require.NoError(t, err)

    // Filter should be FORCED to customerA (ignores customerB in request)
    for _, tx := range listResp.Transactions {
        assert.Equal(t, customerA, tx.CustomerID)
        assert.NotEqual(t, customerB, tx.CustomerID)
    }

    // Customer A tries to list with merchant_id filter
    listResp2, err := client.ListTransactions(ctx, &payment.ListTransactionsRequest{
        MerchantID: "some_merchant", // Customer shouldn't filter by merchant
        Limit: 100,
    }, authCtx)
    require.NoError(t, err)

    // merchant_id filter should be REMOVED, customer_id forced
    for _, tx := range listResp2.Transactions {
        assert.Equal(t, customerA, tx.CustomerID)
    }
}
```

---

## Test Suite 3: Edge Cases

### Test 3.1: Session Expiry - Email Fallback

**File**: `tests/integration/authorization/session_expiry_test.go`

```go
func TestGuest_SessionExpiry_EmailFallback(t *testing.T) {
    client := testutil.Setup(t)

    email := "guest@example.com"
    emailHash := hashEmail(email)
    orderID := uuid.New().String()
    originalSessionID := "sess_original_123"

    // Create payment with session and email
    saleResp := createSaleWithMetadata(t, map[string]interface{}{
        "session_id": originalSessionID,
        "email_hash": emailHash,
        "order_id": orderID,
    })

    groupID := saleResp.GroupID

    // Original session works
    authCtx1 := &AuthContext{
        ActorType: ActorTypeGuest,
        SessionID: &originalSessionID,
    }

    _, err := client.GetTransactionsByGroupID(ctx, groupID, authCtx1)
    require.NoError(t, err)

    // Session expires (simulated by different session ID)
    newSessionID := "sess_new_xyz789"
    authCtx2 := &AuthContext{
        ActorType: ActorTypeGuest,
        SessionID: &newSessionID,
    }

    // New session cannot access
    _, err = client.GetTransactionsByGroupID(ctx, groupID, authCtx2)
    assert.Error(t, err)

    // But can access via email fallback endpoint
    lookupResp, err := client.LookupOrderByEmail(ctx, &payment.LookupOrderRequest{
        OrderID: orderID,
        Email: email,
    })
    require.NoError(t, err)
    assert.Equal(t, groupID, lookupResp.GroupID)
}
```

---

### Test 3.2: Rate Limiting

**File**: `tests/integration/authorization/rate_limiting_test.go`

```go
func TestRateLimiting_ByEndpoint(t *testing.T) {
    client := testutil.Setup(t)

    authCtx := &AuthContext{
        ActorType: ActorTypeCustomer,
        ActorID: "cust_123",
        CustomerID: stringPtr("cust_123"),
    }

    groupID := createSale(t, "cust_123", "50.00").GroupID

    // GetTransactionsByGroupID: 10 requests per minute
    successCount := 0
    rateLimitCount := 0

    for i := 0; i < 15; i++ {
        _, err := client.GetTransactionsByGroupID(ctx, groupID, authCtx)
        if err != nil {
            grpcErr, ok := status.FromError(err)
            if ok && grpcErr.Code() == codes.ResourceExhausted {
                rateLimitCount++
            }
        } else {
            successCount++
        }
    }

    // Should allow 10 requests, then rate limit the rest
    assert.Equal(t, 10, successCount)
    assert.Equal(t, 5, rateLimitCount)

    // Wait for rate limit window to reset
    time.Sleep(61 * time.Second)

    // Should work again
    _, err := client.GetTransactionsByGroupID(ctx, groupID, authCtx)
    require.NoError(t, err)
}
```

---

## Implementation Plan

### Phase 1: Critical Idempotency Tests (Week 1)
- ✅ Test 1.2: Sale idempotency with declined transactions
- ✅ Test 1.3: Network error handling
- ✅ Test 1.4: Capture idempotency
- ✅ Test 1.5: Void idempotency

### Phase 2: Authorization Tests (Week 2)
- ✅ Test 2.1: Customer isolation
- ✅ Test 2.2: Guest session access
- ✅ Test 2.3: Merchant isolation
- ✅ Test 2.4: Admin access
- ✅ Test 2.6: 404 vs 403 behavior
- ✅ Test 2.7: Forced filters

### Phase 3: Advanced Tests (Week 3)
- ✅ Test 2.5: Service account scopes
- ✅ Test 3.1: Session expiry
- ✅ Test 3.2: Rate limiting
- ✅ Concurrent transaction tests
- ✅ Performance tests under load

---

## Test Execution

### Running Tests

```bash
# Run all idempotency tests
go test -tags=integration ./tests/integration/payment/idempotency_*.go -v

# Run all authorization tests
go test -tags=integration ./tests/integration/authorization/ -v

# Run specific test
go test -tags=integration ./tests/integration/authorization/ -run TestCustomer_CanOnlyAccessOwnTransactions -v

# Run with race detector
go test -tags=integration -race ./tests/integration/authorization/ -v
```

### Test Coverage Goals

| Category | Target Coverage | Current |
|----------|----------------|---------|
| Idempotency | 100% | 40% |
| Authorization | 100% | 0% |
| Edge Cases | 80% | 20% |
| **Overall** | **95%** | **25%** |

---

## Summary

### Priority 1 (P0) - Must Have Before Production
- ✅ Sale/Authorize idempotency with declined transactions
- ✅ Network error handling (no transaction created)
- ✅ Customer authorization (isolation)
- ✅ Merchant authorization (isolation)
- ✅ 404 vs 403 behavior (prevent enumeration)

### Priority 2 (P1) - Should Have
- ✅ Guest session authorization
- ✅ Admin full access
- ✅ Forced filters (cannot override)
- ✅ Capture/Void idempotency
- ✅ Service account scopes

### Priority 3 (P2) - Nice to Have
- ✅ Session expiry with email fallback
- ✅ Rate limiting
- ✅ Concurrent request handling
- ✅ Performance under load

---

**Status**: 📋 Test plan documented, implementation pending
**Next Steps**: Begin Phase 1 implementation
