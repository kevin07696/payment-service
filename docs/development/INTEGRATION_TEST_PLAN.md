# Integration Test Implementation Plan

**Last Updated**: 2025-11-19
**Status**: Ready for Implementation
**Purpose**: Implementation plan for 5 new/enhanced integration tests covering ACH and Storage BRIC flows

---

## Overview

This plan implements integration tests for ACH payment flows and Storage BRIC conversions using Server Post API. These tests complement existing Browser Post workflow tests and focus on EPX adapter functionality and business logic validation.

**Key Classification**: Integration tests (NOT E2E)
- Single-actor workflows (Service or EPX callback)
- EPX adapter integration testing
- Business logic validation
- Run on every commit (fast execution: 2-5 seconds each)

---

## Test Suite Summary

| Test | File | RPC/Handler | Priority | Status |
|------|------|-------------|----------|--------|
| 1. ACH Pre-note → Storage BRIC | `ach_prenote_storage_test.go` | `StoreACHAccount` | HIGH | Not started |
| 2. ACH Debit with Verified PM | `ach_debit_test.go` | `ACHDebit` | HIGH | Not started |
| 3. ACH Return Handling | `ach_return_handling_test.go` | Callback Handler | MEDIUM | Not started |
| 4. Browser Post Save Card | `browser_post_workflow_test.go` | Enhancement | MEDIUM | Not started |
| 5. Direct Storage BRIC | `credit_card_server_post_test.go` | `SavePaymentMethod` | LOW | Not started |

---

## Test 1: ACH Pre-note → Storage BRIC Flow

### File Location
`tests/integration/payment_method/ach_prenote_storage_test.go`

### Purpose
Validate ACH account storage with automatic pre-note (CKC0) and Storage BRIC (CKC8) creation via Server Post API.

### API Used
```protobuf
rpc StoreACHAccount(StoreACHAccountRequest) returns (PaymentMethodResponse)
```

### Test Flow

```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: StoreACHAccount → Pre-note → Storage BRIC  │
└──────────────────────────────────────────────────────────────┘

Phase 1: Service Stores ACH Account
  ├─ RPC: PaymentMethodService.StoreACHAccount
  │   Request: {
  │     merchant_id: "test-merchant-uuid",
  │     customer_id: "test-customer-uuid",
  │     account_number: "1234567890",
  │     routing_number: "021000021",  // Valid JPMorgan Chase routing
  │     account_holder_name: "John Doe",
  │     account_type: ACCOUNT_TYPE_CHECKING,
  │     std_entry_class: STD_ENTRY_CLASS_PPD,
  │     bank_name: "Chase",
  │     is_default: true,
  │     idempotency_key: "test-ach-prenote-{timestamp}"
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Validate routing number (ABA checksum)
      ├─ Step 2: Send Pre-note (CKC0) via Server Post
      │   TransactionType: "CKC0"
      │   Amount: "0.00"
      │   AccountNumber: "1234567890"
      │   RoutingNumber: "021000021"
      │   ReceiverName: "John Doe"
      │   StdEntryClass: "PPD"
      │
      ├─ Step 3: EPX returns Financial BRIC
      │   AuthResp: "00" (accepted)
      │   AuthGUID: "[Financial BRIC]"
      │
      ├─ Step 4: Convert to Storage BRIC (CKC8)
      │   TransactionType: "CKC8"
      │   OriginalAuthGUID: "[Financial BRIC]"
      │
      ├─ Step 5: EPX returns Storage BRIC
      │   AuthGUID: "[Storage BRIC]" (never expires)
      │
      └─ Step 6: Store in DB
          INSERT INTO customer_payment_methods (
            payment_token, payment_type, last_four,
            account_type, is_verified, is_default
          ) VALUES (
            '[Storage BRIC]', 'ach', '7890',
            'checking', true, true
          )

Phase 2: Verify Response
  └─ Assertions:
      ✓ payment_method_id is UUID
      ✓ payment_type = PAYMENT_METHOD_TYPE_ACH
      ✓ last_four = "7890"
      ✓ account_type = "checking"
      ✓ bank_name = "Chase"
      ✓ is_verified = false (NOT verified yet - EPX accepted but bank hasn't verified)
      ✓ verification_status = "pending" (waiting for 3-day clearance)
      ✓ prenote_transaction_id is set (links to pre-note transaction)
      ✓ is_default = true
      ✓ is_active = true

Phase 3: Verify Database Record
  ├─ Query DB: SELECT * FROM customer_payment_methods WHERE id = ?
  └─ Assertions:
      ✓ payment_token (Storage BRIC) is encrypted
      ✓ Pre-note transaction exists with type='pre_note'
      ✓ verification_status = 'pending'
      ✓ prenote_transaction_id links to pre-note transaction
      ✓ is_verified = false (will be true after 3 days)
      ✓ verified_at is NULL (will be set after verification)
      ✓ created_at timestamp is recent

Phase 4: Test Idempotency
  ├─ Call StoreACHAccount again with same idempotency_key
  └─ Assertions:
      ✓ Returns same payment_method_id
      ✓ No duplicate payment methods created
      ✓ No duplicate pre-note transactions sent
```

### Implementation Details

**Test Function**:
```go
func TestIntegration_StoreACHAccount_PreNoteToStorageBRIC(t *testing.T) {
    // Setup
    ctx := context.Background()
    merchantID := createTestMerchant(t)
    customerID := createTestCustomer(t)
    defer cleanupTestData(t, merchantID, customerID)

    // Phase 1: Store ACH account
    req := &paymentmethodv1.StoreACHAccountRequest{
        MerchantId:        merchantID,
        CustomerId:        customerID,
        AccountNumber:     "1234567890",
        RoutingNumber:     "021000021",
        AccountHolderName: "John Doe",
        AccountType:       paymentmethodv1.AccountType_ACCOUNT_TYPE_CHECKING,
        StdEntryClass:     paymentmethodv1.StdEntryClass_STD_ENTRY_CLASS_PPD,
        BankName:          ptr("Chase"),
        IsDefault:         true,
        IdempotencyKey:    fmt.Sprintf("test-ach-%d", time.Now().Unix()),
    }

    resp, err := paymentMethodClient.StoreACHAccount(ctx, req)
    require.NoError(t, err, "StoreACHAccount should succeed")

    // Phase 2: Verify response
    assert.NotEmpty(t, resp.PaymentMethodId, "Should return payment method ID")
    assert.Equal(t, paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_ACH, resp.PaymentType)
    assert.Equal(t, "7890", resp.LastFour)
    assert.Equal(t, "checking", resp.AccountType)
    assert.True(t, resp.IsVerified, "Pre-note passed, should be verified")
    assert.True(t, resp.IsDefault)
    assert.True(t, resp.IsActive)

    // Phase 3: Verify database record
    pm := getPaymentMethodFromDB(t, resp.PaymentMethodId)
    assert.NotEmpty(t, pm.PaymentToken, "Storage BRIC should be stored")
    assert.Equal(t, "ach", pm.PaymentType)

    // Verify pre-note transaction was created
    preNoteTx := getTransactionByType(t, merchantID, "pre_note")
    assert.Equal(t, int64(0), preNoteTx.AmountCents, "Pre-note amount should be $0.00")

    // Phase 4: Test idempotency
    resp2, err := paymentMethodClient.StoreACHAccount(ctx, req)
    require.NoError(t, err, "Idempotent call should succeed")
    assert.Equal(t, resp.PaymentMethodId, resp2.PaymentMethodId, "Should return same payment method")

    // Verify only 1 payment method exists
    pms := listPaymentMethods(t, merchantID, customerID)
    assert.Len(t, pms, 1, "Should have exactly 1 payment method")
}
```

### NACHA Compliance Checks
- ✓ Pre-note sent before any debit transactions
- ✓ STD_ENTRY_CLASS preserved (PPD for personal accounts)
- ✓ Routing number validation (ABA checksum)
- ✓ Account holder name required
- ✓ is_verified = false initially (honest verification status)
- ✓ Grace period allows 0-3 day transaction window
- ✓ Verification completes after 3 days with no returns

### Safe Verification Flow
- ✓ EPX acceptance != bank verification
- ✓ Payment method created with verification_status='pending'
- ✓ Cron job marks as verified after 3 days
- ✓ Return codes immediately mark as failed
- ✓ Grace period allows customer-friendly UX

### Dependencies
- EPX Server Post adapter (`internal/adapters/epx/server_post_adapter.go`)
- Payment Method service (`internal/services/payment_method/payment_method_service.go`)
- Test merchant and customer fixtures

---

## Test 2: ACH Debit with Verified Payment Method

### File Location
`tests/integration/payment/ach_debit_test.go`

### Purpose
Validate ACH debit (CKC2) with verified Storage BRIC and transaction creation.

### API Used
```protobuf
rpc ACHDebit(ACHDebitRequest) returns (PaymentResponse)
```

### Test Flow

```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: ACH Debit with Verified Payment Method     │
└──────────────────────────────────────────────────────────────┘

Phase 1: Setup - Create Pending ACH Payment Method
  └─ Use StoreACHAccount (from Test 1)
      Returns: payment_method_id with is_verified=false, verification_status='pending'
      Note: Within 3-day grace period, so ACHDebit will be allowed

Phase 2: Process ACH Debit
  ├─ RPC: PaymentService.ACHDebit
  │   Request: {
  │     merchant_id: "test-merchant-uuid",
  │     customer_id: "test-customer-uuid",
  │     payment_method_id: "[From Phase 1]",
  │     amount_cents: 15000,  // $150.00
  │     currency: "USD",
  │     idempotency_key: "test-ach-debit-{timestamp}"
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Validate payment method
      │   - Check is_verified = true
      │   - Check is_active = true
      │   - Check payment_type = 'ach'
      │
      ├─ Step 2: Create transaction (status='pending')
      │   INSERT INTO transactions (
      │     amount_cents, type, payment_method_type,
      │     payment_method_id
      │   ) VALUES (15000, 'charge', 'ach', ?)
      │
      ├─ Step 3: Send to EPX (CKC2)
      │   TransactionType: "CKC2"
      │   Amount: "150.00"  // Converted from 15000 cents
      │   AuthGUID: "[Storage BRIC]"
      │   CardEntryMethod: "Z"
      │
      ├─ Step 4: EPX returns
      │   AuthResp: "00" (accepted)
      │   AuthGUID: "[Financial BRIC]"
      │   TranNbr: "1234567890"
      │   AuthCode: "123456"
      │
      └─ Step 5: Update transaction
          UPDATE transactions SET
            auth_resp = '00',
            auth_guid = '[Financial BRIC]',
            tran_nbr = '1234567890',
            auth_code = '123456',
            processed_at = NOW()
          WHERE id = ?

Phase 3: Verify Transaction Response
  └─ Assertions:
      ✓ transaction_id is UUID
      ✓ type = TRANSACTION_TYPE_CHARGE
      ✓ status = TRANSACTION_STATUS_APPROVED
      ✓ amount_cents = 15000
      ✓ currency = "USD"
      ✓ payment_method_type = PAYMENT_METHOD_TYPE_ACH
      ✓ authorization_code present
      ✓ is_approved = true

Phase 4: Verify Database Record
  ├─ Query: SELECT * FROM transactions WHERE id = ?
  └─ Assertions:
      ✓ payment_method_id matches
      ✓ tran_nbr (EPX transaction number) stored
      ✓ auth_guid (Financial BRIC) stored
      ✓ processed_at timestamp set
      ✓ status generated from auth_resp='00' → 'approved'

Phase 5: Verify Payment Method Last Used Updated
  ├─ Query: SELECT last_used_at FROM customer_payment_methods WHERE id = ?
  └─ Assertions:
      ✓ last_used_at timestamp is recent (within last minute)

Phase 6: Test Business Logic Validations
  ├─ Test 1: Debit with unverified payment method
  │   - Create ACH payment method without pre-note
  │   - Attempt ACHDebit
  │   - Expected: Error "Payment method not verified"
  │
  ├─ Test 2: Debit with inactive payment method
  │   - Deactivate payment method (is_active=false)
  │   - Attempt ACHDebit
  │   - Expected: Error "Payment method is inactive"
  │
  └─ Test 3: Idempotency
      - Call ACHDebit twice with same idempotency_key
      - Expected: Returns same transaction_id, no duplicate charge
```

### Implementation Details

**Test Function**:
```go
func TestIntegration_ACHDebit_VerifiedPaymentMethod(t *testing.T) {
    ctx := context.Background()
    merchantID := createTestMerchant(t)
    customerID := createTestCustomer(t)
    defer cleanupTestData(t, merchantID, customerID)

    // Phase 1: Create verified ACH payment method
    pmResp, err := paymentMethodClient.StoreACHAccount(ctx, &paymentmethodv1.StoreACHAccountRequest{
        MerchantId:        merchantID,
        CustomerId:        customerID,
        AccountNumber:     "1234567890",
        RoutingNumber:     "021000021",
        AccountHolderName: "John Doe",
        AccountType:       paymentmethodv1.AccountType_ACCOUNT_TYPE_CHECKING,
        StdEntryClass:     paymentmethodv1.StdEntryClass_STD_ENTRY_CLASS_PPD,
    })
    require.NoError(t, err)
    require.True(t, pmResp.IsVerified, "Payment method should be verified")

    // Phase 2: Process ACH debit
    debitReq := &paymentv1.ACHDebitRequest{
        MerchantId:      merchantID,
        CustomerId:      customerID,
        PaymentMethodId: pmResp.PaymentMethodId,
        AmountCents:     15000, // $150.00
        Currency:        "USD",
        IdempotencyKey:  fmt.Sprintf("test-ach-debit-%d", time.Now().Unix()),
    }

    txResp, err := paymentClient.ACHDebit(ctx, debitReq)
    require.NoError(t, err, "ACHDebit should succeed")

    // Phase 3: Verify transaction response
    assert.NotEmpty(t, txResp.TransactionId)
    assert.Equal(t, paymentv1.TransactionType_TRANSACTION_TYPE_CHARGE, txResp.Type)
    assert.Equal(t, paymentv1.TransactionStatus_TRANSACTION_STATUS_APPROVED, txResp.Status)
    assert.Equal(t, int64(15000), txResp.AmountCents)
    assert.Equal(t, "USD", txResp.Currency)
    assert.True(t, txResp.IsApproved)
    assert.NotEmpty(t, txResp.AuthorizationCode)

    // Phase 4: Verify database record
    tx := getTransactionFromDB(t, txResp.TransactionId)
    assert.Equal(t, pmResp.PaymentMethodId, tx.PaymentMethodID)
    assert.NotEmpty(t, tx.TranNbr, "EPX transaction number should be stored")
    assert.NotEmpty(t, tx.AuthGUID, "Financial BRIC should be stored")
    assert.NotNil(t, tx.ProcessedAt)

    // Phase 5: Verify payment method last_used_at updated
    pm := getPaymentMethodFromDB(t, pmResp.PaymentMethodId)
    assert.WithinDuration(t, time.Now(), pm.LastUsedAt, 1*time.Minute)

    // Phase 6: Test business logic validations
    t.Run("UnverifiedPaymentMethod", func(t *testing.T) {
        // Create unverified payment method (skip pre-note for testing)
        // This would require a helper that bypasses verification
        // Expected error: "Payment method not verified"
    })

    t.Run("InactivePaymentMethod", func(t *testing.T) {
        // Deactivate payment method
        _, err := paymentMethodClient.UpdatePaymentMethodStatus(ctx, &paymentmethodv1.UpdatePaymentMethodStatusRequest{
            PaymentMethodId: pmResp.PaymentMethodId,
            MerchantId:      merchantID,
            CustomerId:      customerID,
            IsActive:        false,
        })
        require.NoError(t, err)

        // Attempt debit
        _, err = paymentClient.ACHDebit(ctx, &paymentv1.ACHDebitRequest{
            MerchantId:      merchantID,
            CustomerId:      customerID,
            PaymentMethodId: pmResp.PaymentMethodId,
            AmountCents:     5000,
        })
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "inactive")
    })

    t.Run("Idempotency", func(t *testing.T) {
        // Reactivate payment method first
        _, err := paymentMethodClient.UpdatePaymentMethodStatus(ctx, &paymentmethodv1.UpdatePaymentMethodStatusRequest{
            PaymentMethodId: pmResp.PaymentMethodId,
            MerchantId:      merchantID,
            CustomerId:      customerID,
            IsActive:        true,
        })
        require.NoError(t, err)

        // First debit
        req := &paymentv1.ACHDebitRequest{
            MerchantId:      merchantID,
            CustomerId:      customerID,
            PaymentMethodId: pmResp.PaymentMethodId,
            AmountCents:     10000,
            IdempotencyKey:  "idempotent-key-123",
        }
        resp1, err := paymentClient.ACHDebit(ctx, req)
        require.NoError(t, err)

        // Second debit with same key
        resp2, err := paymentClient.ACHDebit(ctx, req)
        require.NoError(t, err)
        assert.Equal(t, resp1.TransactionId, resp2.TransactionId, "Should return same transaction")
    })
}
```

### Business Logic Validations
- ✓ ACH debit allowed within 3-day grace period (optimistic verification)
- ✓ ACH debit rejected outside grace period if unverified
- ✓ ACH debit allowed if is_verified = true (any age)
- ✓ ACH debit requires `is_active = true`
- ✓ Amount converted cents → dollars for EPX
- ✓ Last used timestamp updated
- ✓ Idempotency enforced

### Grace Period Handling
- ✓ Transactions allowed during days 0-3 (grace period)
- ✓ Transactions rejected after day 3 if still unverified
- ✓ Clear logging when using grace period
- ✓ Risk acknowledged in service logs

---

## Test 3: ACH Return Handling

### File Location
`tests/integration/payment/ach_return_handling_test.go`

### Purpose
Validate ACH return code processing and automatic payment method deactivation after 2 returns.

### Handler Used
`BrowserPostCallbackHandler` (processes ACH return notifications from EPX)

### Test Flow

```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: ACH Return Codes → Auto-Deactivation       │
└──────────────────────────────────────────────────────────────┘

Phase 1: Setup - Create ACH Debit
  ├─ Create verified ACH payment method
  └─ Process ACH debit (from Test 2)
      Returns: transaction_id, tran_nbr

Phase 2: Simulate First ACH Return (R01 - Insufficient Funds)
  ├─ POST to /webhooks/epx/callback
  │   Form Data: {
  │     TRAN_NBR: "[Original debit tran_nbr]",
  │     AUTH_RESP: "R01",  // Return code
  │     RESPONSE_TEXT: "Insufficient Funds",
  │     EPX_MAC: "[Valid HMAC signature]",
  │     TRAN_TYPE: "ACHRETURN",
  │     ORIGINAL_TRAN_TYPE: "CKC2"
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Verify EPX_MAC signature (security)
      ├─ Step 2: Find transaction by tran_nbr
      ├─ Step 3: Update transaction
      │   UPDATE transactions SET
      │     status = 'returned',
      │     metadata = jsonb_set(metadata, '{return_code}', '"R01"'),
      │     metadata = jsonb_set(metadata, '{return_description}', '"Insufficient Funds"')
      │   WHERE tran_nbr = ?
      │
      ├─ Step 4: Increment payment method return count
      │   UPDATE customer_payment_methods SET
      │     return_count = return_count + 1
      │   WHERE id = (SELECT payment_method_id FROM transactions WHERE tran_nbr = ?)
      │
      ├─ Step 5: Check return count threshold
      │   - return_count = 1 → Keep is_active = true
      │
      └─ Step 6: Create audit log entry
          INSERT INTO audit_logs (
            entity_type, entity_id, action, details
          ) VALUES (
            'payment_method', ?, 'ach_return',
            '{"return_code": "R01", "transaction_id": "..."}'
          )

Phase 3: Verify First Return Processing
  ├─ Query transaction: SELECT * FROM transactions WHERE tran_nbr = ?
  └─ Assertions:
      ✓ status = 'returned'
      ✓ metadata->>'return_code' = 'R01'
      ✓ metadata->>'return_description' = 'Insufficient Funds'

  ├─ Query payment method: SELECT * FROM customer_payment_methods WHERE id = ?
  └─ Assertions:
      ✓ return_count = 1
      ✓ is_active = true (still active after 1 return)

Phase 4: Simulate Second ACH Return (R03 - No Account)
  ├─ Create another ACH debit with same payment method
  ├─ POST to /webhooks/epx/callback
  │   {
  │     TRAN_NBR: "[Second debit tran_nbr]",
  │     AUTH_RESP: "R03",
  │     RESPONSE_TEXT: "No Account/Unable to Locate Account"
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Update transaction (status='returned')
      ├─ Step 2: Increment return_count = 2
      ├─ Step 3: **Auto-deactivate**: is_active = false
      │   UPDATE customer_payment_methods SET
      │     is_active = false,
      │     deactivation_reason = 'excessive_returns',
      │     deactivated_at = NOW()
      │   WHERE id = ?
      │
      └─ Step 4: Create audit log

Phase 5: Verify Auto-Deactivation
  ├─ Query payment method
  └─ Assertions:
      ✓ return_count = 2
      ✓ is_active = false (auto-deactivated)
      ✓ deactivation_reason = 'excessive_returns'
      ✓ deactivated_at timestamp set

Phase 6: Verify Deactivated Payment Method Cannot Be Used
  ├─ Attempt ACHDebit with deactivated payment method
  └─ Assertions:
      ✓ Error: "Payment method is inactive"
      ✓ No transaction created
```

### Implementation Details

**Test Function**:
```go
func TestIntegration_ACHReturnHandling_AutoDeactivation(t *testing.T) {
    ctx := context.Background()
    merchantID := createTestMerchant(t)
    customerID := createTestCustomer(t)
    defer cleanupTestData(t, merchantID, customerID)

    // Phase 1: Setup - Create ACH debit
    pmResp, _ := createVerifiedACHPaymentMethod(t, merchantID, customerID)
    tx1, _ := createACHDebit(t, merchantID, customerID, pmResp.PaymentMethodId, 15000)

    // Phase 2: Simulate first ACH return (R01)
    callbackData := url.Values{
        "TRAN_NBR":       {tx1.TranNbr},
        "AUTH_RESP":      {"R01"},
        "RESPONSE_TEXT":  {"Insufficient Funds"},
        "TRAN_TYPE":      {"ACHRETURN"},
        "ORIGINAL_TRAN_TYPE": {"CKC2"},
    }

    // Add EPX_MAC signature
    mac := generateEPXMAC(t, callbackData)
    callbackData.Set("EPX_MAC", mac)

    resp, err := http.PostForm("http://localhost:8081/webhooks/epx/callback", callbackData)
    require.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    // Phase 3: Verify first return processing
    tx := getTransactionFromDB(t, tx1.ID)
    assert.Equal(t, "returned", tx.Status)
    assert.Equal(t, "R01", tx.Metadata["return_code"])
    assert.Equal(t, "Insufficient Funds", tx.Metadata["return_description"])

    pm := getPaymentMethodFromDB(t, pmResp.PaymentMethodId)
    assert.Equal(t, 1, pm.ReturnCount)
    assert.True(t, pm.IsActive, "Should still be active after 1 return")

    // Phase 4: Simulate second ACH return (R03)
    tx2, _ := createACHDebit(t, merchantID, customerID, pmResp.PaymentMethodId, 10000)

    callbackData2 := url.Values{
        "TRAN_NBR":      {tx2.TranNbr},
        "AUTH_RESP":     {"R03"},
        "RESPONSE_TEXT": {"No Account/Unable to Locate Account"},
        "TRAN_TYPE":     {"ACHRETURN"},
    }
    mac2 := generateEPXMAC(t, callbackData2)
    callbackData2.Set("EPX_MAC", mac2)

    resp, err = http.PostForm("http://localhost:8081/webhooks/epx/callback", callbackData2)
    require.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    // Phase 5: Verify auto-deactivation
    pm = getPaymentMethodFromDB(t, pmResp.PaymentMethodId)
    assert.Equal(t, 2, pm.ReturnCount)
    assert.False(t, pm.IsActive, "Should be deactivated after 2 returns")
    assert.Equal(t, "excessive_returns", pm.DeactivationReason)
    assert.NotNil(t, pm.DeactivatedAt)

    // Phase 6: Verify deactivated payment method cannot be used
    _, err = paymentClient.ACHDebit(ctx, &paymentv1.ACHDebitRequest{
        MerchantId:      merchantID,
        CustomerId:      customerID,
        PaymentMethodId: pmResp.PaymentMethodId,
        AmountCents:     5000,
    })
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "inactive")
}

func TestIntegration_ACHReturnCodes_Coverage(t *testing.T) {
    // Test all common return codes
    testCases := []struct {
        returnCode string
        description string
        shouldDeactivate bool
    }{
        {"R01", "Insufficient Funds", false},
        {"R02", "Account Closed", true},  // Immediate deactivation
        {"R03", "No Account/Unable to Locate", true},
        {"R04", "Invalid Account Number", true},
        {"R05", "Unauthorized Debit", true},
    }

    for _, tc := range testCases {
        t.Run(tc.returnCode, func(t *testing.T) {
            // Test return code handling
            // Verify description stored correctly
            // Verify deactivation logic
        })
    }
}
```

### Return Code Coverage
- R01: Insufficient Funds (warning, 2nd return → deactivate)
- R02: Account Closed (immediate deactivation)
- R03: No Account (immediate deactivation)
- R04: Invalid Account Number (immediate deactivation)
- R05: Unauthorized Debit (immediate deactivation)

### Business Logic
- ✓ First return increments count, keeps active
- ✓ Second return auto-deactivates payment method
- ✓ Critical returns (R02-R05) may deactivate immediately
- ✓ Audit log created for each return
- ✓ Deactivated payment methods block new transactions

---

## Test 4: Browser Post Save Card (Enhancement)

### File Location
`tests/integration/payment/browser_post_workflow_test.go` (add new test function)

### Purpose
Enhance existing Browser Post tests to validate Financial BRIC → Storage BRIC conversion via CCE8.

### New Test Function
`TestIntegration_BrowserPost_SaveCard_Workflow`

### Test Flow

```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: Browser Post SALE → Save Card → CCE8       │
└──────────────────────────────────────────────────────────────┘

Phase 1: Browser Post SALE with Save Card Flag
  ├─ Customer submits Browser Post form:
  │   TRAN_TYPE: "CCE0" (SALE)
  │   TRAN_AMT: "29.99"
  │   USER_DATA_2: "save_card=true"  ← Trigger save card flow
  │   CALLBACK_URL: "http://localhost:8081/webhooks/epx/callback"
  │
  └─ EPX processes:
      ├─ Creates Financial BRIC (13-24 month expiry)
      ├─ Processes SALE transaction
      └─ Sends callback to server

Phase 2: Server Receives Callback with Financial BRIC
  ├─ POST /webhooks/epx/callback
  │   {
  │     TRAN_NBR: "1234567890",
  │     AUTH_GUID: "[Financial BRIC]",
  │     AUTH_RESP: "00",
  │     CARD_TYPE: "V",
  │     LAST_FOUR: "1111",
  │     EXP_MONTH: "12",
  │     EXP_YEAR: "25",
  │     USER_DATA_2: "save_card=true"  ← Detected
  │   }
  │
  └─ Backend performs:
      ├─ Step 1: Verify EPX_MAC
      ├─ Step 2: Save transaction
      ├─ Step 3: Detect USER_DATA_2="save_card=true"
      ├─ Step 4: Call ConvertFinancialBRICToStorageBRIC RPC

Phase 3: Convert Financial BRIC to Storage BRIC (CCE8)
  ├─ RPC: PaymentMethodService.ConvertFinancialBRICToStorageBRIC
  │   {
  │     merchant_id: "...",
  │     customer_id: "...",
  │     financial_bric: "[Financial BRIC]",
  │     transaction_id: "[SALE transaction ID]",
  │     payment_type: PAYMENT_METHOD_TYPE_CREDIT_CARD,
  │     last_four: "1111",
  │     card_brand: "visa",
  │     card_exp_month: 12,
  │     card_exp_year: 2025
  │   }
  │
  └─ Backend performs:
      ├─ Send CCE8 to EPX
      │   TRAN_TYPE: "CCE8"
      │   ORIG_AUTH_GUID: "[Financial BRIC]"
      │   ACCOUNT_VERIFICATION: "true"
      │
      ├─ EPX returns Storage BRIC (never expires)
      │   AUTH_GUID: "[Storage BRIC]"
      │
      └─ Store in DB
          INSERT INTO customer_payment_methods (
            payment_token, payment_type, last_four,
            card_brand, card_exp_month, card_exp_year,
            is_verified
          ) VALUES (
            '[Storage BRIC]', 'credit_card', '1111',
            'visa', 12, 2025, true
          )

Phase 4: Verify Payment Method Saved
  ├─ Query: SELECT * FROM customer_payment_methods WHERE merchant_id = ? AND customer_id = ?
  └─ Assertions:
      ✓ payment_type = 'credit_card'
      ✓ last_four = '1111'
      ✓ card_brand = 'visa'
      ✓ card_exp_month = 12
      ✓ card_exp_year = 2025
      ✓ is_verified = true (Account Verification passed)
      ✓ payment_token = Storage BRIC (encrypted)

Phase 5: Use Saved Card for Recurring Payment
  ├─ RPC: PaymentService.Sale
  │   {
  │     payment_method_id: "[From Phase 4]",
  │     amount_cents: 4999  // $49.99
  │   }
  │
  └─ Backend uses Storage BRIC for payment

Phase 6: Verify Recurring Payment Success
  └─ Assertions:
      ✓ Transaction approved
      ✓ Used Storage BRIC (not Financial BRIC)
      ✓ No card details in request
```

### Implementation Details

**Add to existing file**:
```go
func TestIntegration_BrowserPost_SaveCard_Workflow(t *testing.T) {
    // This test extends the existing Browser Post workflow tests
    // Add new test case to browser_post_workflow_test.go

    ctx := context.Background()
    merchantID := createTestMerchant(t)
    customerID := createTestCustomer(t)
    defer cleanupTestData(t, merchantID, customerID)

    // Phase 1: Browser Post SALE with save_card flag
    // (Use existing Chrome automation from browser_post_workflow_test.go)
    callbackReceived := make(chan map[string]string, 1)

    // Submit Browser Post form with USER_DATA_2="save_card=true"
    // Wait for callback

    callbackData := <-callbackReceived

    // Phase 2 is handled automatically by BrowserPostCallbackHandler

    // Phase 4: Verify payment method was saved
    pmList, err := paymentMethodClient.ListPaymentMethods(ctx, &paymentmethodv1.ListPaymentMethodsRequest{
        MerchantId: merchantID,
        CustomerId: customerID,
    })
    require.NoError(t, err)
    require.Len(t, pmList.PaymentMethods, 1, "Should have 1 saved payment method")

    pm := pmList.PaymentMethods[0]
    assert.Equal(t, "1111", pm.LastFour)
    assert.Equal(t, "visa", pm.CardBrand)
    assert.True(t, pm.IsVerified)

    // Phase 5: Use saved card for recurring payment
    saleResp, err := paymentClient.Sale(ctx, &paymentv1.SaleRequest{
        MerchantId:      merchantID,
        CustomerId:      customerID,
        PaymentMethodId: pm.Id,
        AmountCents:     4999,
        Currency:        "USD",
    })
    require.NoError(t, err)

    // Phase 6: Verify recurring payment success
    assert.Equal(t, paymentv1.TransactionStatus_TRANSACTION_STATUS_APPROVED, saleResp.Status)
    assert.Equal(t, int64(4999), saleResp.AmountCents)
}
```

### PCI Compliance Checks
- ✓ No card number stored in database
- ✓ No CVV stored
- ✓ Only last 4 digits + brand stored for display
- ✓ Storage BRIC encrypted at rest
- ✓ Financial BRIC → Storage BRIC conversion automatic

---

## Test 5: Direct Storage BRIC (Server Post)

### File Location
`tests/integration/payment_method/credit_card_server_post_test.go`

### Purpose
Validate direct credit card tokenization via Server Post CCE8 (Storage BRIC) without Browser Post.

### API Used
```protobuf
rpc SavePaymentMethod(SavePaymentMethodRequest) returns (PaymentMethodResponse)
```

### Test Flow

```
┌──────────────────────────────────────────────────────────────┐
│ Integration Test: Raw Card → CCE8 → Storage BRIC (Direct)    │
└──────────────────────────────────────────────────────────────┘

Phase 1: Service Tokenizes Raw Card (Backend Only)
  ├─ Backend receives raw card (via PCI-compliant form)
  │   card_number: "4111111111111111"
  │   exp_month: 12
  │   exp_year: 2025
  │   cvv: "123"
  │   billing_zip: "12345"
  │
  └─ Validate:
      ✓ Luhn checksum (card number)
      ✓ Expiry date (not expired)
      ✓ CVV format (3-4 digits)

Phase 2: Send to EPX Server Post (CCE8)
  └─ POST to EPX Server Post endpoint
      TransactionType: "CCE8" (Storage BRIC)
      Amount: "0.00" (Account Verification)
      CardNumber: "4111111111111111"
      ExpMonth: "12"
      ExpYear: "2025"
      CVV: "123"
      BillingZip: "12345"
      AccountVerification: "true"

Phase 3: Receive EPX Response
  └─ EPX returns:
      AuthGUID: "[Storage BRIC]" (never expires)
      AuthResp: "00" (approved)
      CardType: "V" (Visa)
      LastFour: "1111"

Phase 4: Save Payment Method
  ├─ RPC: PaymentMethodService.SavePaymentMethod
  │   {
  │     payment_token: "[Storage BRIC]",
  │     payment_type: PAYMENT_METHOD_TYPE_CREDIT_CARD,
  │     last_four: "1111",
  │     card_brand: "visa",
  │     card_exp_month: 12,
  │     card_exp_year: 2025
  │   }
  │
  └─ Store in DB (NO card number, NO CVV)

Phase 5: Verify Payment Method Saved
  └─ Assertions:
      ✓ payment_token = Storage BRIC (encrypted)
      ✓ No card_number column in DB
      ✓ No cvv column in DB
      ✓ Only last_four stored for display

Phase 6: Verify PCI Compliance
  └─ Database schema assertions:
      ✓ card_number column does NOT exist
      ✓ cvv column does NOT exist
      ✓ Only payment_token (encrypted) stored
```

### Implementation Details

**Test Function**:
```go
func TestIntegration_DirectStorageBRIC_ServerPost(t *testing.T) {
    // This test validates direct card tokenization
    // In production, this happens server-side after PCI-compliant form submission

    ctx := context.Background()
    merchantID := createTestMerchant(t)
    customerID := createTestCustomer(t)
    defer cleanupTestData(t, merchantID, customerID)

    // Phase 1-3: Backend tokenizes card via Server Post
    // (This would be done by payment_method_service.go internally)
    // For testing, we'll call the RPC directly with a pre-tokenized BRIC

    // In a real scenario, SavePaymentMethod would:
    // 1. Receive raw card details
    // 2. Send to EPX Server Post CCE8
    // 3. Receive Storage BRIC
    // 4. Store in DB

    // For this test, we assume step 1-3 happened and we have a Storage BRIC
    storageBRIC := createStorageBRICViaServerPost(t, "4111111111111111", 12, 2025, "123", "12345")

    // Phase 4: Save payment method
    resp, err := paymentMethodClient.SavePaymentMethod(ctx, &paymentmethodv1.SavePaymentMethodRequest{
        MerchantId:    merchantID,
        CustomerId:    customerID,
        PaymentToken:  storageBRIC,
        PaymentType:   paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD,
        LastFour:      "1111",
        CardBrand:     ptr("visa"),
        CardExpMonth:  ptr(int32(12)),
        CardExpYear:   ptr(int32(2025)),
        IsDefault:     true,
    })
    require.NoError(t, err)

    // Phase 5: Verify payment method saved
    assert.NotEmpty(t, resp.PaymentMethodId)
    assert.Equal(t, "1111", resp.LastFour)
    assert.Equal(t, "visa", resp.CardBrand)
    assert.True(t, resp.IsVerified, "Account Verification passed")

    // Phase 6: Verify PCI compliance
    pm := getPaymentMethodFromDB(t, resp.PaymentMethodId)

    // Verify schema does NOT have sensitive fields
    assert.NotContains(t, pm, "card_number", "card_number should not exist in DB")
    assert.NotContains(t, pm, "cvv", "cvv should not exist in DB")
    assert.NotEmpty(t, pm.PaymentToken, "Only encrypted Storage BRIC stored")
    assert.Equal(t, "1111", pm.LastFour, "Only last 4 digits for display")
}

// Helper function that simulates Server Post CCE8 call
func createStorageBRICViaServerPost(t *testing.T, cardNumber string, expMonth, expYear int, cvv, zip string) string {
    // This helper would call the EPX Server Post adapter
    // For testing purposes, it returns a test Storage BRIC

    req := &ports.ServerPostRequest{
        TransactionType: ports.TransactionTypeBRICStorageCard,
        Amount:          "0.00",
        CardNumber:      &cardNumber,
        ExpMonth:        &expMonth,
        ExpYear:         &expYear,
        CVV:             &cvv,
        BillingZip:      &zip,
        AccountVerification: ptr(true),
    }

    resp, err := serverPostAdapter.ProcessTransaction(context.Background(), req)
    require.NoError(t, err)
    require.Equal(t, "00", resp.AuthResp, "Account Verification should succeed")

    return resp.AuthGUID // Storage BRIC
}
```

### PCI Compliance Validation
- ✓ Raw card data never stored in DB
- ✓ CVV never stored (even temporarily)
- ✓ Only Storage BRIC (NTID) stored
- ✓ Only last 4 digits for display
- ✓ Storage BRIC encrypted at rest

---

## Implementation Priority

### Phase 1: ACH Core Flows (Week 1)
1. ✅ Test 1: ACH Pre-note → Storage BRIC
2. ✅ Test 2: ACH Debit with Verified PM

### Phase 2: ACH Returns & Edge Cases (Week 2)
3. ✅ Test 3: ACH Return Handling

### Phase 3: Credit Card Storage BRIC (Week 3)
4. ✅ Test 4: Browser Post Save Card (Enhancement)
5. ✅ Test 5: Direct Storage BRIC

---

## Test Infrastructure Requirements

### Database Setup
- Test database with migrations applied
- Cleanup scripts for test data
- Fixtures for merchants and customers

### EPX Test Environment
- EPX staging credentials
- EPX MAC secret for callback validation
- Test routing numbers (021000021 - JPMorgan Chase)
- Test card numbers (4111111111111111 - Visa test card)

### Helper Functions Needed

**Test Fixtures**:
```go
func createTestMerchant(t *testing.T) string
func createTestCustomer(t *testing.T) string
func createVerifiedACHPaymentMethod(t *testing.T, merchantID, customerID string) *PaymentMethodResponse
func createACHDebit(t *testing.T, merchantID, customerID, paymentMethodID string, amountCents int64) *Transaction
```

**EPX Helpers**:
```go
func generateEPXMAC(t *testing.T, data url.Values) string
func createStorageBRICViaServerPost(t *testing.T, cardNumber string, expMonth, expYear int, cvv, zip string) string
```

**DB Helpers**:
```go
func getTransactionFromDB(t *testing.T, transactionID string) *Transaction
func getPaymentMethodFromDB(t *testing.T, paymentMethodID string) *PaymentMethod
func cleanupTestData(t *testing.T, merchantID, customerID string)
```

---

## Success Criteria

### All Tests Pass
- ✅ All 5 integration tests implemented
- ✅ Tests pass consistently (< 1% flake rate)
- ✅ Tests run on every commit
- ✅ Total execution time < 30 seconds

### Code Coverage
- ✅ ACH flows: 80%+ coverage
- ✅ Payment method service: 75%+ coverage
- ✅ Callback handler: 70%+ coverage

### Documentation
- ✅ Test plan documented (this file)
- ✅ Each test has clear purpose and flow
- ✅ PCI compliance validated
- ✅ NACHA compliance validated

---

## Next Steps

1. **Review this plan** - Confirm approach with team
2. **Create test fixtures** - Merchant/customer helpers
3. **Implement Test 1** - ACH Pre-note → Storage BRIC
4. **Implement Test 2** - ACH Debit
5. **Implement Test 3** - ACH Return Handling
6. **Implement Test 4** - Browser Post Save Card
7. **Implement Test 5** - Direct Storage BRIC
8. **Update CI/CD** - Add to commit hooks

---

**Last Updated**: 2025-11-19
**Maintained By**: Backend Team
**Review Required**: After each implementation phase
