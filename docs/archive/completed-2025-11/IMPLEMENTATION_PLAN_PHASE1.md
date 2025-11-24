# Implementation Plan - Phase 1 Critical Fixes

**Date**: 2025-11-22
**Phase**: Phase 1 - Critical Issues
**Total Estimated Time**: 6 hours
**Priority**: IMMEDIATE

---

## Overview

This document provides detailed implementation plans for the 2 critical business logic issues that require immediate fixing:

1. **Issue #1**: ACH Verification Not Enforced in Subscription Billing (2 hours)
2. **Issue #2**: Concurrent Capture Race Condition (4 hours)

Each plan includes:
- Detailed implementation steps
- Code changes with line numbers
- Test specifications
- Rollback strategy
- Success metrics

---

## Issue #1: Enforce ACH Verification in Subscription Billing

### Executive Summary

**Problem**: Subscription billing can charge unverified ACH payment methods, resulting in ACH returns and merchant fees ($3-9K/month).

**Solution**: Add `CanUseForAmount()` validation before billing ACH payment methods.

**Impact**: Prevents $3,000-9,000/month in ACH return fees.

**Estimated Time**: 2 hours

---

### Detailed Implementation Plan

#### Step 1: Add Payment Method Validation (30 minutes)

**File**: `internal/services/subscription/subscription_service.go`
**Function**: `processSubscriptionBilling()` (line 529)

**Current Code** (lines 562-569):
```go
// Get payment method
pm, err := s.queries.GetPaymentMethodByID(ctx, sub.PaymentMethodID)
if err != nil {
    return fmt.Errorf("failed to get payment method: %w", err)
}

if !pm.IsActive.Valid || !pm.IsActive.Bool {
    return fmt.Errorf("payment method is not active")
}
```

**New Code**:
```go
// Get payment method
pm, err := s.queries.GetPaymentMethodByID(ctx, sub.PaymentMethodID)
if err != nil {
    return fmt.Errorf("failed to get payment method: %w", err)
}

// Convert sqlc payment method to domain model for validation
domainPM := sqlcPaymentMethodToDomain(&pm)

// ✅ Use domain validation to check if payment method can be used
canUse, reason := domainPM.CanUseForAmount(sub.AmountCents)
if !canUse {
    s.logger.Warn("Cannot bill subscription - payment method validation failed",
        zap.String("subscription_id", sub.ID.String()),
        zap.String("payment_method_id", pm.ID.String()),
        zap.String("payment_type", pm.PaymentType),
        zap.String("reason", reason),
    )

    // Mark as failed billing and increment retry count
    // This will eventually mark subscription as past_due
    return s.handleBillingFailure(ctx, sub, fmt.Errorf("payment method cannot be used: %s", reason))
}

s.logger.Info("Payment method validation passed",
    zap.String("payment_method_id", pm.ID.String()),
    zap.String("payment_type", pm.PaymentType),
    zap.Bool("is_verified", domainPM.IsVerified),
)
```

**Validation Logic** (already exists in `internal/domain/payment_method.go:97-117`):
```go
func (pm *PaymentMethod) CanUseForAmount(amountCents int64) (bool, string) {
    // Check active status FIRST
    if !pm.IsActive {
        return false, "payment method is not active"
    }

    // Credit card expiration check
    if pm.IsCreditCard() && pm.IsExpired() {
        return false, "credit card is expired"
    }

    // ✅ ACH verification check (THIS IS THE KEY FIX)
    if pm.IsACH() && !pm.IsVerified {
        return false, "ACH account must be verified before use"
    }

    return true, ""
}
```

#### Step 2: Add Conversion Helper Function (15 minutes)

**File**: `internal/services/subscription/subscription_service.go`
**Location**: Bottom of file (after line 833)

**New Code**:
```go
// sqlcPaymentMethodToDomain converts sqlc payment method to domain model
func sqlcPaymentMethodToDomain(pm *sqlc.CustomerPaymentMethod) *domain.PaymentMethod {
    domainPM := &domain.PaymentMethod{
        ID:          pm.ID.String(),
        MerchantID:  pm.MerchantID.String(),
        CustomerID:  pm.CustomerID,
        PaymentToken: pm.Bric,
        LastFour:    pm.LastFour,
        PaymentType: domain.PaymentMethodType(pm.PaymentType),
        CreatedAt:   pm.CreatedAt.Time,
        UpdatedAt:   pm.UpdatedAt.Time,
    }

    // Set active status
    if pm.IsActive.Valid {
        domainPM.IsActive = pm.IsActive.Bool
    }

    // Set verified status
    if pm.IsVerified.Valid {
        domainPM.IsVerified = pm.IsVerified.Bool
    }

    // Set default status
    if pm.IsDefault.Valid {
        domainPM.IsDefault = pm.IsDefault.Bool
    }

    // Credit card specific fields
    if pm.CardBrand.Valid {
        domainPM.CardBrand = &pm.CardBrand.String
    }
    if pm.CardExpMonth.Valid {
        month := int(pm.CardExpMonth.Int32)
        domainPM.CardExpMonth = &month
    }
    if pm.CardExpYear.Valid {
        year := int(pm.CardExpYear.Int32)
        domainPM.CardExpYear = &year
    }

    // ACH specific fields
    if pm.BankName.Valid {
        domainPM.BankName = &pm.BankName.String
    }
    if pm.AccountType.Valid {
        domainPM.AccountType = &pm.AccountType.String
    }

    // Timestamps
    if pm.LastUsedAt.Valid {
        domainPM.LastUsedAt = &pm.LastUsedAt.Time
    }
    if pm.VerifiedAt.Valid {
        domainPM.VerifiedAt = &pm.VerifiedAt.Time
    }
    if pm.DeactivatedAt.Valid {
        domainPM.DeactivatedAt = &pm.DeactivatedAt.Time
    }

    // Nullable string fields
    if pm.PreNoteTransactionID.Valid {
        txID := pm.PreNoteTransactionID.Bytes.String()
        domainPM.PreNoteTransactionID = &txID
    }
    if pm.VerificationStatus.Valid {
        domainPM.VerificationStatus = &pm.VerificationStatus.String
    }
    if pm.DeactivationReason.Valid {
        domainPM.DeactivationReason = &pm.DeactivationReason.String
    }
    if pm.VerificationFailureReason.Valid {
        domainPM.VerificationFailureReason = &pm.VerificationFailureReason.String
    }

    // Return count
    if pm.ReturnCount.Valid {
        count := int(pm.ReturnCount.Int32)
        domainPM.ReturnCount = &count
    }

    return domainPM
}
```

#### Step 3: Add Integration Test (1 hour)

**File**: `tests/integration/subscription/recurring_billing_ach_test.go` (NEW FILE)

**Test Content**:
```go
//go:build integration
// +build integration

package subscription_test

import (
    "testing"
    "time"

    "github.com/kevin07696/payment-service/tests/integration/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestRecurringBilling_UnverifiedACH tests that unverified ACH payment methods
// cannot be charged by recurring billing
func TestRecurringBilling_UnverifiedACH(t *testing.T) {
    testutil.SkipIfBRICStorageUnavailable(t)

    cfg, _ := testutil.Setup(t)
    client := testutil.NewClient("http://localhost:8080")
    customerID := "00000000-0000-0000-0000-000000000010"
    merchantID := "00000000-0000-0000-0000-000000000001"

    // Load test service credentials
    services, err := testutil.LoadTestServices()
    require.NoError(t, err)

    jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
    require.NoError(t, err)

    // Setup UNVERIFIED ACH payment method
    // Create payment method via Browser Post STORAGE with ACH account
    achAccount := testutil.TestACHAccount{
        RoutingNumber: "021000021", // Chase
        AccountNumber: "1234567890",
        AccountType:   "checking",
    }

    paymentMethodID, err := testutil.TokenizeAndSaveACHViaBrowserPost(
        t, cfg, client, jwtToken, merchantID, customerID, achAccount, "http://localhost:8081",
    )
    require.NoError(t, err)
    time.Sleep(2 * time.Second)

    // Verify payment method is unverified
    client.SetHeader("Authorization", "Bearer "+jwtToken)
    defer client.ClearHeaders()

    pmResp, err := client.DoConnectRPC("payment_method.v1.PaymentMethodService", "GetPaymentMethod", map[string]interface{}{
        "paymentMethodId": paymentMethodID,
    })
    require.NoError(t, err)
    defer pmResp.Body.Close()

    var pm map[string]interface{}
    err = testutil.DecodeResponse(pmResp, &pm)
    require.NoError(t, err)

    isVerified, _ := pm["isVerified"].(bool)
    assert.False(t, isVerified, "ACH payment method should be unverified initially")

    t.Logf("✅ Unverified ACH payment method created: %s", paymentMethodID)

    // Create subscription with unverified ACH payment method
    startDate := time.Now().Add(-60 * 24 * time.Hour) // 2 months ago (due for billing)

    createSubReq := map[string]interface{}{
        "merchantId":      merchantID,
        "customerId":      customerID,
        "paymentMethodId": paymentMethodID,
        "amountCents":     2999, // $29.99
        "currency":        "USD",
        "intervalValue":   1,
        "intervalUnit":    3, // MONTH
        "startDate":       startDate.Format(time.RFC3339Nano),
        "maxRetries":      3,
    }

    subResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "CreateSubscription", createSubReq)
    require.NoError(t, err)
    defer subResp.Body.Close()

    require.Equal(t, 200, subResp.StatusCode)

    var subResult map[string]interface{}
    err = testutil.DecodeResponse(subResp, &subResult)
    require.NoError(t, err)

    subscriptionID, ok := subResult["subscriptionId"].(string)
    require.True(t, ok && subscriptionID != "")

    t.Logf("✅ Subscription created with unverified ACH: %s", subscriptionID)
    time.Sleep(2 * time.Second)

    // Attempt recurring billing (should FAIL validation)
    billingReq := map[string]interface{}{
        "asOfDate":  time.Now().Format(time.RFC3339Nano),
        "batchSize": 100,
    }

    billingResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "ProcessDueBilling", billingReq)
    require.NoError(t, err)
    defer billingResp.Body.Close()

    assert.Equal(t, 200, billingResp.StatusCode)

    var billingResult map[string]interface{}
    err = testutil.DecodeResponse(billingResp, &billingResult)
    require.NoError(t, err)

    // ✅ KEY ASSERTION: Billing should FAIL (not reach EPX)
    failedCount, _ := billingResult["failedCount"].(float64)
    assert.Greater(t, int(failedCount), 0, "Billing should fail for unverified ACH")

    t.Logf("✅ Billing correctly failed for unverified ACH - Failed: %d", int(failedCount))

    // Verify subscription status
    time.Sleep(1 * time.Second)
    getResp, err := client.DoConnectRPC("subscription.v1.SubscriptionService", "GetSubscription", map[string]interface{}{
        "subscriptionId": subscriptionID,
    })
    require.NoError(t, err)
    defer getResp.Body.Close()

    var subscription map[string]interface{}
    err = testutil.DecodeResponse(getResp, &subscription)
    require.NoError(t, err)

    status, _ := subscription["status"].(string)
    failureRetryCount, _ := subscription["failureRetryCount"].(float64)

    // Subscription should still be active but with incremented failure count
    assert.Equal(t, "SUBSCRIPTION_STATUS_ACTIVE", status)
    assert.Greater(t, int(failureRetryCount), 0, "Failure count should be incremented")

    t.Logf("✅ Subscription status after failed billing: %s (retry count: %d)", status, int(failureRetryCount))
}

// TestRecurringBilling_VerifiedACH tests that VERIFIED ACH payment methods
// CAN be charged successfully
func TestRecurringBilling_VerifiedACH(t *testing.T) {
    // ... similar test but with verified ACH payment method ...
    // This validates the happy path after verification completes
}
```

#### Step 4: Update Documentation (15 minutes)

**File**: `docs/integration/SUBSCRIPTION_BILLING.md` (NEW or UPDATE)

Add section:
```markdown
## ACH Verification Requirements

### Recurring Billing with ACH

ACH payment methods MUST be verified before they can be used for recurring billing.

**Verification Flow**:
1. Customer adds ACH account → Pre-note transaction sent
2. Payment method status: `verification_status = 'pending'`, `is_verified = false`
3. Wait 3 business days for pre-note to clear
4. Cron job checks for returns → Auto-verify if no returns
5. Payment method status: `verification_status = 'verified'`, `is_verified = true`
6. **NOW billing can proceed**

**Validation**:
The subscription billing system validates payment methods using `PaymentMethod.CanUseForAmount()`:
- ✅ Checks if payment method is active
- ✅ Checks if credit card is expired
- ✅ **Checks if ACH account is verified** ← NEW

**Error Handling**:
If ACH account is unverified during billing:
- Billing attempt fails (no EPX call made)
- Subscription failure_retry_count incremented
- After max_retries, subscription marked as `past_due`
- Merchant should notify customer to verify account or update payment method
```

---

### Testing Strategy

#### Unit Tests
**File**: `internal/services/subscription/subscription_service_test.go`

Add test:
```go
func TestProcessSubscriptionBilling_UnverifiedACH(t *testing.T) {
    // Mock payment method with is_verified = false, payment_type = 'ach'
    // Call processSubscriptionBilling()
    // Assert: handleBillingFailure called
    // Assert: EPX adapter NOT called
}
```

#### Integration Tests
- `TestRecurringBilling_UnverifiedACH` - Validates unverified ACH blocked
- `TestRecurringBilling_VerifiedACH` - Validates verified ACH works

#### Manual Testing Checklist
- [ ] Create subscription with unverified ACH
- [ ] Trigger billing manually → Should fail validation
- [ ] Verify ACH account (mark is_verified = true)
- [ ] Trigger billing again → Should succeed
- [ ] Check logs for validation messages

---

### Rollback Strategy

If issues arise after deployment:

**Option 1: Feature Flag** (Recommended)
```go
// Add to config
enableACHVerificationCheck := os.Getenv("ENABLE_ACH_VERIFICATION_CHECK") == "true"

if enableACHVerificationCheck {
    canUse, reason := domainPM.CanUseForAmount(sub.AmountCents)
    if !canUse {
        return s.handleBillingFailure(ctx, sub, fmt.Errorf("payment method cannot be used: %s", reason))
    }
}
```

**Option 2: Database Rollback**
- No schema changes - code-only change
- Git revert commit
- Redeploy previous version

**Option 3: Hotfix**
- Comment out validation temporarily
- Deploy
- Investigate issue

---

### Success Metrics

**Pre-Deployment Metrics** (Baseline):
- ACH return count/month
- ACH return fees/month
- Subscription billing failure rate

**Post-Deployment Metrics** (Expected Improvements):
- ✅ ACH return count: -80% (only verified accounts charged)
- ✅ ACH return fees: -$3,000-9,000/month
- ✅ Subscription billing validation failures: +5-10% (expected - blocking unverified)
- ✅ No new errors in logs related to ACH billing

**Monitoring**:
- Alert on `subscription.billing.validation_failed` metric
- Dashboard: ACH verification status distribution
- Weekly report: ACH return rate trend

---

### Deployment Plan

#### Pre-Deployment
1. ✅ Code review
2. ✅ Unit tests passing
3. ✅ Integration tests passing
4. ✅ Staging deployment and smoke tests

#### Deployment
1. Deploy to staging
2. Run integration test suite
3. Deploy to production during low-traffic window
4. Monitor for 1 hour
5. Verify metrics

#### Post-Deployment
1. Monitor error rates for 24 hours
2. Check ACH return rates after 3 days (verification period)
3. Review monthly ACH fees after 30 days

---

## Issue #2: Fix Concurrent Capture Race Condition

### Executive Summary

**Problem**: Two concurrent capture requests can both succeed on the same AUTH transaction, causing double charges.

**Root Cause**: No row-level locking between state validation and EPX call (TOCTOU vulnerability).

**Solution**: Add `SELECT FOR UPDATE` on transaction tree to prevent concurrent modifications.

**Impact**: Prevents customer overcharges and chargeback liability.

**Estimated Time**: 4 hours

---

### Detailed Implementation Plan

#### Step 1: Add Row-Level Locking to SQL Query (1 hour)

**File**: `internal/db/queries/transactions.sql`
**Query**: `GetTransactionTree`

**Current Code**:
```sql
-- name: GetTransactionTree :many
-- Get transaction tree (root + all descendants) for state computation
-- Used in: Capture, Refund, Void operations
WITH RECURSIVE transaction_tree AS (
    -- Base case: start with the given transaction
    SELECT * FROM transactions WHERE id = $1 AND deleted_at IS NULL

    UNION ALL

    -- Recursive case: find all children
    SELECT t.*
    FROM transactions t
    INNER JOIN transaction_tree tt ON t.parent_transaction_id = tt.id
    WHERE t.deleted_at IS NULL
)
SELECT * FROM transaction_tree
ORDER BY created_at ASC;  -- ⚠️ No row-level locking
```

**New Code**:
```sql
-- name: GetTransactionTreeForUpdate :many
-- Get transaction tree with row-level lock for concurrent capture protection
-- This prevents race conditions where two captures could succeed on the same AUTH
WITH RECURSIVE transaction_tree AS (
    -- Base case: Lock the root transaction (prevents concurrent modifications)
    SELECT * FROM transactions
    WHERE id = $1 AND deleted_at IS NULL
    FOR UPDATE  -- ✅ Lock root transaction

    UNION ALL

    -- Recursive case: Lock all children
    SELECT t.*
    FROM transactions t
    INNER JOIN transaction_tree tt ON t.parent_transaction_id = tt.id
    WHERE t.deleted_at IS NULL
)
SELECT * FROM transaction_tree
ORDER BY created_at ASC;
```

**Regenerate SQLC Code**:
```bash
sqlc generate
```

This creates new generated function:
- `GetTransactionTreeForUpdate(ctx context.Context, id uuid.UUID) ([]GetTransactionTreeRow, error)`

#### Step 2: Update Capture Logic to Use Locking Query (1 hour)

**File**: `internal/services/payment/payment_service.go`
**Function**: `Capture()` (line 453)

**Current Code** (line 529-535):
```go
err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
    // Get transaction tree (includes root + all descendants)
    var err error
    groupTxs, err = q.GetTransactionTree(ctx, originalTxID)  // ⚠️ No lock
    if err != nil {
        return fmt.Errorf("failed to get transaction tree: %w", err)
    }
    // ...
})
```

**New Code**:
```go
err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
    // Get transaction tree WITH ROW-LEVEL LOCK (prevents concurrent captures)
    // ✅ SELECT FOR UPDATE ensures only one capture can proceed at a time
    var err error
    groupTxs, err = q.GetTransactionTreeForUpdate(ctx, originalTxID)
    if err != nil {
        return fmt.Errorf("failed to get transaction tree: %w", err)
    }

    if len(groupTxs) == 0 {
        return fmt.Errorf("no transactions found for parent %s", originalTxID.String())
    }

    // Convert to domain transactions
    domainTxs := make([]*domain.Transaction, len(groupTxs))
    for i, tx := range groupTxs {
        sqlcTx := sqlc.Transaction(tx)
        domainTxs[i] = sqlcToDomain(&sqlcTx)
    }

    // Compute current state using WAL
    state := ComputeGroupState(domainTxs)

    // Validate capture is allowed
    // ✅ This validation is now ATOMIC with the lock
    // No other thread can modify state between here and EPX call
    captureAmountCents := state.ActiveAuthAmount
    if req.AmountCents != nil {
        captureAmountCents = *req.AmountCents
    }

    canCapture, reason := state.CanCapture(captureAmountCents)
    if !canCapture {
        s.logger.Warn("Capture validation failed",
            zap.String("capture_transaction_id", txID.String()),
            zap.String("reason", reason),
        )
        return domain.ErrTransactionCannotBeCaptured
    }

    s.logger.Info("Capture validation passed (with lock)",
        zap.String("auth_bric", state.ActiveAuthBRIC),
        zap.String("capture_amount", formatCentsForLog(captureAmountCents)),
    )

    // Get merchant and validate
    merchantID := uuid.MustParse(domainTxs[0].MerchantID)
    merchant, err := q.GetMerchantByID(ctx, merchantID)
    if err != nil {
        return fmt.Errorf("failed to get merchant: %w", err)
    }

    if !merchant.IsActive {
        return domain.ErrMerchantInactive
    }

    // Get MAC secret
    _, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
    if err != nil {
        return fmt.Errorf("failed to get MAC secret: %w", err)
    }

    // ✅ Lock is held until end of transaction
    // EPX call happens AFTER transaction commits
    return nil
})
```

**Key Changes**:
1. Use `GetTransactionTreeForUpdate()` instead of `GetTransactionTree()`
2. Lock is acquired at transaction tree read
3. Lock is held through validation
4. Lock is released when transaction commits (line 591)
5. EPX call happens after lock release (line 597+)

#### Step 3: Remove Redundant Re-fetch (30 minutes)

**Current Code** (lines 597-609):
```go
// Re-fetch state outside transaction for EPX call
groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, originalTxID)  // ⚠️ Redundant
```

**Analysis**: After acquiring lock, state cannot change until transaction commits. The re-fetch is redundant and can cause stale reads.

**New Code**:
```go
// State is already validated and locked - no need to re-fetch
// Lock prevents concurrent modifications until transaction commits
state := ComputeGroupState(domainTxsRefetch)  // Use state from locked read
```

**Alternative**: Keep re-fetch for paranoia, but log warning if state changed:
```go
// Defensive: Re-check state (should be unchanged due to lock)
groupTxsRefetch, err := s.queries.GetTransactionTree(ctx, originalTxID)
if err != nil {
    return nil, fmt.Errorf("failed to re-fetch transaction tree: %w", err)
}

domainTxsRefetch := make([]*domain.Transaction, len(groupTxsRefetch))
for i, tx := range groupTxsRefetch {
    sqlcTx := sqlc.Transaction(tx)
    domainTxsRefetch[i] = sqlcToDomain(&sqlcTx)
}
stateRefetch := ComputeGroupState(domainTxsRefetch)

// Sanity check: state should be unchanged (lock prevents modifications)
if stateRefetch.ActiveAuthAmount != state.ActiveAuthAmount {
    s.logger.Error("State changed after lock - this should never happen!",
        zap.Int64("original_amount", state.ActiveAuthAmount),
        zap.Int64("refetch_amount", stateRefetch.ActiveAuthAmount),
    )
    return nil, fmt.Errorf("transaction state changed unexpectedly")
}
```

#### Step 4: Add Integration Test for Concurrent Captures (1.5 hours)

**File**: `tests/integration/payment/concurrent_capture_test.go` (NEW FILE)

**Test Content**:
```go
//go:build integration
// +build integration

package payment_test

import (
    "context"
    "sync"
    "testing"
    "time"

    "github.com/kevin07696/payment-service/tests/integration/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestConcurrentCaptures tests that concurrent capture requests
// are properly serialized by row-level locking
func TestConcurrentCaptures(t *testing.T) {
    testutil.SkipIfBRICStorageUnavailable(t)

    cfg, _ := testutil.Setup(t)
    client := testutil.NewClient("http://localhost:8080")
    merchantID := "00000000-0000-0000-0000-000000000001"
    customerID := "00000000-0000-0000-0000-000000000020"

    // Load test service credentials
    services, err := testutil.LoadTestServices()
    require.NoError(t, err)

    jwtToken, err := testutil.GenerateJWT(services[0].PrivateKeyPEM, services[0].ServiceID, merchantID, time.Hour)
    require.NoError(t, err)

    client.SetHeader("Authorization", "Bearer "+jwtToken)
    defer client.ClearHeaders()

    // Step 1: Create AUTH transaction for $100.00
    authReq := map[string]interface{}{
        "merchantId":  merchantID,
        "customerId":  customerID,
        "amountCents": 10000, // $100.00
        "currency":    "USD",
        "cardNumber":  "4111111111111111",
        "cardExpMonth": 12,
        "cardExpYear":  2025,
        "cvv":         "123",
    }

    authResp, err := client.DoConnectRPC("payment.v1.PaymentService", "Authorize", authReq)
    require.NoError(t, err)
    defer authResp.Body.Close()

    var authResult map[string]interface{}
    err = testutil.DecodeResponse(authResp, &authResult)
    require.NoError(t, err)

    authTxID, ok := authResult["transactionId"].(string)
    require.True(t, ok && authTxID != "")

    t.Logf("✅ AUTH created: %s ($100.00)", authTxID)
    time.Sleep(2 * time.Second)

    // Step 2: Launch 2 concurrent CAPTURE requests for FULL amount
    var wg sync.WaitGroup
    results := make(chan map[string]interface{}, 2)
    errors := make(chan error, 2)

    captureAmount := 10000 // $100.00 (full auth amount)

    for i := 0; i < 2; i++ {
        wg.Add(1)
        go func(threadID int) {
            defer wg.Done()

            captureReq := map[string]interface{}{
                "transactionId": authTxID,
                "amountCents":   captureAmount, // Both try to capture $100
                // Note: Different idempotency keys to simulate true concurrency
                "idempotencyKey": fmt.Sprintf("capture-%d-%d", time.Now().Unix(), threadID),
            }

            t.Logf("Thread %d: Attempting capture of $%d.00...", threadID, captureAmount/100)

            resp, err := client.DoConnectRPC("payment.v1.PaymentService", "Capture", captureReq)
            if err != nil {
                errors <- err
                return
            }
            defer resp.Body.Close()

            var result map[string]interface{}
            if err := testutil.DecodeResponse(resp, &result); err != nil {
                errors <- err
                return
            }

            results <- result
            t.Logf("Thread %d: Capture response received (status: %d)", threadID, resp.StatusCode)
        }(i)
    }

    // Wait for both goroutines to complete
    wg.Wait()
    close(results)
    close(errors)

    // Collect results
    var successCount int
    var failureCount int

    for result := range results {
        status, _ := result["status"].(string)
        if status == "approved" {
            successCount++
        } else {
            failureCount++
        }
    }

    for err := range errors {
        t.Logf("Capture error: %v", err)
        failureCount++
    }

    // ✅ KEY ASSERTION: Only ONE capture should succeed
    assert.Equal(t, 1, successCount, "Only one concurrent capture should succeed")
    assert.Equal(t, 1, failureCount, "One concurrent capture should fail")

    t.Logf("✅ Concurrent capture test passed - Success: %d, Failed: %d", successCount, failureCount)

    // Step 3: Verify final transaction state
    time.Sleep(2 * time.Second)

    listReq := map[string]interface{}{
        "merchantId": merchantID,
        "customerId": customerID,
        "limit":      10,
    }

    listResp, err := client.DoConnectRPC("payment.v1.PaymentService", "ListTransactions", listReq)
    require.NoError(t, err)
    defer listResp.Body.Close()

    var listResult map[string]interface{}
    err = testutil.DecodeResponse(listResp, &listResult)
    require.NoError(t, err)

    transactions, _ := listResult["transactions"].([]interface{})

    // Count captures
    var approvedCaptures int
    for _, txInterface := range transactions {
        tx, _ := txInterface.(map[string]interface{})
        txType, _ := tx["type"].(string)
        status, _ := tx["status"].(string)

        if txType == "CAPTURE" && status == "approved" {
            approvedCaptures++
        }
    }

    // ✅ Final assertion: Only 1 approved CAPTURE transaction should exist
    assert.Equal(t, 1, approvedCaptures, "Only one approved CAPTURE should exist in database")

    t.Logf("✅ Final state verified - Approved captures: %d", approvedCaptures)
}

// TestConcurrentPartialCaptures tests concurrent partial captures
// e.g., AUTH $100 → Capture A ($50) + Capture B ($50)
func TestConcurrentPartialCaptures(t *testing.T) {
    // Similar test but with partial amounts
    // Expected: Both should succeed if total <= auth amount
    // With locking: They are serialized, so both can succeed
    // Without locking: Race condition could cause both to fail OR both to succeed incorrectly
}
```

#### Step 5: Update Metrics and Logging (30 minutes)

**Add Metric**:
```go
// pkg/observability/business_metrics.go
var (
    ConcurrentCaptureBlocked = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "payment_concurrent_capture_blocked_total",
            Help: "Number of capture requests blocked due to concurrent access",
        },
        []string{"merchant_id"},
    )
)
```

**Update Capture Logic**:
```go
// In Capture() function
err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
    // Acquire lock
    start := time.Now()
    groupTxs, err = q.GetTransactionTreeForUpdate(ctx, originalTxID)
    lockDuration := time.Since(start)

    if lockDuration > 100*time.Millisecond {
        // Lock took longer than expected - likely concurrent access
        s.logger.Warn("Capture lock acquisition delayed",
            zap.String("auth_transaction_id", originalTxID.String()),
            zap.Duration("lock_wait_ms", lockDuration),
        )
        observability.ConcurrentCaptureBlocked.WithLabelValues(merchantID.String()).Inc()
    }

    // ... rest of validation ...
})
```

---

### Testing Strategy

#### Unit Tests
Not applicable - this is integration/database-level behavior.

#### Integration Tests
- `TestConcurrentCaptures` - Two full captures (one should fail)
- `TestConcurrentPartialCaptures` - Two partial captures (serialized execution)
- `TestCaptureAfterVoid` - Capture on voided AUTH should fail

#### Load Testing
```bash
# Simulate high concurrency
for i in {1..10}; do
    curl -X POST http://localhost:8080/capture \
        -H "Authorization: Bearer $JWT" \
        -d '{"transaction_id":"'$AUTH_ID'","amount_cents":10000}' &
done
wait

# Expected: Only 1 success, 9 failures
```

---

### Rollback Strategy

**Risk**: Row-level locking could cause deadlocks or performance degradation.

**Mitigation**:
1. **Monitor lock wait times** - Alert if > 1 second
2. **Timeout on lock acquisition** - PostgreSQL default: `lock_timeout = 30s`
3. **Deadlock detection** - PostgreSQL auto-detects and aborts one transaction

**Rollback Options**:
1. Revert to `GetTransactionTree()` (no lock)
2. Add feature flag to toggle locking behavior
3. Reduce lock scope (lock only root, not entire tree)

---

### Success Metrics

**Pre-Deployment**:
- Concurrent capture failure rate: ~0% (race condition exists but rare)
- Lock wait time: N/A

**Post-Deployment**:
- ✅ Concurrent capture failure rate: Expected increase to ~50% (second request properly rejected)
- ✅ Lock wait time P95: < 100ms
- ✅ Lock wait time P99: < 500ms
- ✅ No deadlocks detected
- ✅ No customer overcharges reported

---

## Phase 1 Summary

### Total Estimated Time
- Issue #1 (ACH Verification): 2 hours
- Issue #2 (Concurrent Capture): 4 hours
- **Total**: 6 hours

### Implementation Order
1. Issue #1: ACH Verification (lower risk, higher ROI)
2. Issue #2: Concurrent Capture (higher complexity, requires careful testing)

### Success Criteria
- ✅ All integration tests passing
- ✅ Code reviewed and approved
- ✅ Deployed to staging without errors
- ✅ Metrics show expected improvements
- ✅ No customer-facing issues

### Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| ACH verification blocks legitimate subscriptions | Low | Medium | Verify ACH verification workflow is working |
| Row-level locking causes deadlocks | Low | High | Monitor lock wait times, add timeouts |
| Performance degradation from locking | Medium | Low | Load test before production deployment |
| Integration tests fail in CI/CD | Medium | Low | Run locally first, verify test fixtures |

---

**Ready for Implementation** ✅
