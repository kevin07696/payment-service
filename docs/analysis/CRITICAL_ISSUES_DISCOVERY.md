# Critical Business Logic Issues - Discovery Report

**Date**: 2025-11-22
**Status**: Discovery Phase Complete - Ready for Planning
**Analyst**: System Architecture Review

---

## Executive Summary

This discovery phase analyzed 5 critical business logic issues identified in the BUSINESS_LOGIC_ANALYSIS. After systematic code review, **2 CRITICAL issues confirmed**, 3 classified as IMPORTANT/NICE-TO-HAVE.

### Issues by Severity

**CRITICAL** (Immediate Fix Required):
1. ✅ ACH Verification Not Enforced in Subscription Billing - **CONFIRMED**
2. ✅ Race Condition on Concurrent Captures - **PARTIALLY MITIGATED** (idempotency exists, but gap remains)

**IMPORTANT** (High Priority):
3. ✅ Missing Chargeback Foreign Key Constraint - **CONFIRMED**
4. ✅ No Webhook Notifications for Billing Failures - **CONFIRMED**

**NICE-TO-HAVE**:
5. ✅ No Dunning Management for Past Due Subscriptions - **CONFIRMED**

---

## Issue #1: ACH Verification Not Enforced in Subscription Billing 🚨

### Status: CRITICAL - CONFIRMED

### Discovery Details

**Location**: `internal/services/subscription/subscription_service.go:562-569`

**Code Review**:
```go
// Line 562-569 in processSubscriptionBilling()
pm, err := s.queries.GetPaymentMethodByID(ctx, sub.PaymentMethodID)
if err != nil {
    return fmt.Errorf("failed to get payment method: %w", err)
}

if !pm.IsActive.Valid || !pm.IsActive.Bool {
    return fmt.Errorf("payment method is not active")  // ⚠️ ONLY checks is_active
}

// ⚠️ MISSING VALIDATION:
// - No check for pm.IsVerified (for ACH payment methods)
// - No check for pm.VerificationStatus == 'verified'
// - No check for pm.PaymentType to differentiate ACH vs credit card

// Proceeds directly to EPX billing at line 610
epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
```

**Comparison with Domain Logic**:

The `PaymentMethod` domain model has proper validation:

```go
// internal/domain/payment_method.go:97-117
func (pm *PaymentMethod) CanUseForAmount(amountCents int64) (bool, string) {
    // Check active status FIRST
    if !pm.IsActive {
        return false, "payment method is not active"
    }

    // Credit card expiration check
    if pm.IsCreditCard() && pm.IsExpired() {
        return false, "credit card is expired"
    }

    // ✅ ACH verification check (THIS IS WHAT'S MISSING IN SUBSCRIPTION BILLING)
    if pm.IsACH() && !pm.IsVerified {
        return false, "ACH account must be verified before use"
    }

    return true, ""
}
```

**Impact Analysis**:

1. **Unverified ACH Accounts Can Be Charged**:
   - Subscription billing bypasses domain validation
   - Payment methods with `is_verified = false` can be charged
   - ACH accounts in `verification_status = 'pending'` can be billed

2. **Financial Consequences**:
   - ACH returns (R03: No Account/Unable to Locate, R04: Invalid Account Number)
   - Merchant fees: $15-30 per return
   - At 100 subscriptions/day with 10% unverified → 10 returns/day → $150-300/day in fees
   - Monthly impact: ~$4,500-9,000 in avoidable fees

3. **Customer Experience**:
   - Failed payments for legitimate customers
   - Subscription marked as `past_due` incorrectly
   - Customer receives decline notification for valid account (pre-note still pending)

**Test Coverage Gap**:

Integration test `tests/integration/subscription/recurring_billing_test.go`:
- Line 29: Uses credit card only: `setupPaymentMethod(..., testutil.TestVisaCard)`
- Line 167: Failed billing test also uses credit card
- **No ACH recurring billing tests exist**

**Evidence from ACH Verification Cron**:

The ACH verification cron job (`cmd/cron/handlers/ach_verification_handler.go`) properly implements the 3-day verification workflow, but subscription billing ignores verification status.

**Frequency Estimate**:
- **Recurring**: Every billing cycle for subscriptions with unverified ACH
- **Traffic**: Low-medium (depends on merchant ACH adoption)
- **Risk**: HIGH (financial impact per occurrence)

---

## Issue #2: Race Condition on Concurrent Captures 🚨

### Status: CRITICAL - PARTIALLY MITIGATED

### Discovery Details

**Location**: `internal/services/payment/payment_service.go:527-591`

**Current Implementation**:

```go
// Line 527-591 in Capture()
err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
    // Get transaction tree (NO row-level locking)
    groupTxs, err = q.GetTransactionTree(ctx, originalTxID)  // ⚠️ No SELECT FOR UPDATE
    if err != nil {
        return fmt.Errorf("failed to get transaction tree: %w", err)
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
    canCapture, reason := state.CanCapture(captureAmountCents)
    if !canCapture {
        return domain.ErrTransactionCannotBeCaptured
    }

    // ⚠️ RACE CONDITION WINDOW: Between validation and EPX call
    // Another thread can capture here!

    return nil // Exit transaction for EPX call
})

// Line 597-626: Re-fetch state (NO lock) and call EPX
// ⚠️ State could have changed between line 591 and line 626
```

**Race Condition Scenario**:

```
Time    Thread A (Capture #1)              Thread B (Capture #2)
----    -----------------------             -----------------------
T0      GetTransactionTree(AUTH-123)
T1      ComputeGroupState()
        → ActiveAuthAmount: $100.00
        → CapturedAmount: $0
T2      CanCapture($100.00)? → YES
T3                                          GetTransactionTree(AUTH-123)
T4                                          ComputeGroupState()
                                            → ActiveAuthAmount: $100.00
                                            → CapturedAmount: $0  (same as T1!)
T5                                          CanCapture($100.00)? → YES
T6      Exit DB transaction
T7                                          Exit DB transaction
T8      Call EPX Capture($100)
T9                                          Call EPX Capture($100)
T10     EPX approves → AUTH_RESP='00'
T11                                         EPX approves → AUTH_RESP='00'
T12     Save CAPTURE-1 transaction
T13                                         Save CAPTURE-2 transaction

RESULT: $200 captured on $100 AUTH (DOUBLE CHARGE)
```

**Partial Mitigation - Idempotency**:

The code does have idempotency protection:

```go
// Line 488-516: Idempotency check
if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
    txID, err = uuid.Parse(*req.IdempotencyKey)
    if err != nil {
        return nil, fmt.Errorf("invalid idempotency_key format: %w", err)
    }
} else {
    txID = uuid.New()  // ⚠️ Problem: Auto-generated IDs bypass idempotency
}

// Check if transaction already exists
existingTxDB, existErr := s.queries.GetTransactionByID(ctx, txID)
if existErr == nil {
    // Transaction exists - check if complete
    if existingTx.AuthResp.Valid && existingTx.AuthResp.String != "" {
        // Complete - return existing (idempotent)
        return sqlcToDomain(existingTx), nil
    }
}
```

**Gap in Mitigation**:

1. **Client-provided idempotency keys required**: If client doesn't provide `idempotency_key`, UUID is auto-generated → no deduplication
2. **No row-level locking**: Even with idempotency, concurrent requests with different keys can both succeed
3. **Time-of-check to time-of-use (TOCTOU)**: State validated at T2, but used at T8 (stale read)

**Impact Analysis**:

1. **Double Capture Risk**:
   - Two API calls with different idempotency keys (or no keys) can both succeed
   - Customer overcharged (e.g., $100 AUTH → $200 captured)
   - Chargeback risk + merchant liability

2. **Partial Capture Issues**:
   - AUTH $100 → Capture A ($50) + Capture B ($50) = OK
   - BUT: Concurrent Capture A ($75) + Capture B ($75) = $150 captured on $100 AUTH

3. **Frequency**:
   - **Low probability** (requires exact timing + duplicate requests)
   - **HIGH impact** when it occurs (financial loss + trust damage)

**Current Safety Mechanisms**:

1. ✅ WAL-based state computation (immutable append-only)
2. ✅ Idempotency via transaction ID (when client provides keys)
3. ✅ Database transaction isolation for writes
4. ⚠️ **MISSING**: Row-level locking on parent transaction
5. ⚠️ **MISSING**: Optimistic locking / version column

---

## Issue #3: Missing Chargeback Foreign Key Constraint

### Status: IMPORTANT - CONFIRMED

**Location**: `internal/db/migrations/004_chargebacks.sql:8`

**Current Schema**:
```sql
-- Line 8: No FK constraint
transaction_id UUID NOT NULL,  -- ⚠️ No REFERENCES clause
```

**Expected Schema**:
```sql
transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
```

**Impact**:
- Orphaned chargeback records if transaction deleted
- Referential integrity violations
- Cannot enforce business rule: "Chargebacks must reference valid transactions"

**Severity**: IMPORTANT (data integrity issue, but not immediate financial risk)

---

## Issue #4: No Webhook Notifications for Billing Failures

### Status: IMPORTANT - CONFIRMED

**Location**: `internal/services/subscription/subscription_service.go:716-754`

**Current Code**:
```go
// Line 716-754: handleBillingFailure()
func (s *subscriptionService) handleBillingFailure(ctx context.Context, sub *sqlc.Subscription, billingErr error) error {
    return s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
        newRetryCount := sub.FailureRetryCount + 1
        var newStatus string

        if newRetryCount >= sub.MaxRetries {
            newStatus = string(domain.SubscriptionStatusPastDue)  // ⚠️ No webhook sent
        } else {
            newStatus = string(domain.SubscriptionStatusActive)  // ⚠️ No webhook sent
        }

        // Update subscription status
        params := sqlc.IncrementSubscriptionFailureCountParams{
            ID:                sub.ID,
            FailureRetryCount: newRetryCount,
            Status:            newStatus,
        }

        _, err := q.IncrementSubscriptionFailureCount(ctx, params)
        if err != nil {
            return fmt.Errorf("failed to update failure count: %w", err)
        }

        return billingErr  // ⚠️ No webhook delivery attempted
    })
}
```

**Missing Events**:
1. `subscription.payment_failed` - Individual billing attempt failed
2. `subscription.past_due` - Max retries reached
3. `subscription.payment_retry` - Retry scheduled

**Impact**:
- Merchants not notified of billing issues
- Cannot implement custom recovery flows
- Poor visibility into subscription health

**Comparison**: Chargeback webhooks ARE implemented (`internal/services/webhook/webhook_delivery_service.go`)

---

## Issue #5: No Dunning Management for Past Due Subscriptions

### Status: NICE-TO-HAVE - CONFIRMED

**Location**: `internal/services/subscription/subscription_service.go:716-754`

**Current Behavior**:
```go
if newRetryCount >= sub.MaxRetries {
    // Max retries reached - mark as past_due
    newStatus = string(domain.SubscriptionStatusPastDue)
    // ⚠️ Subscription stays in past_due forever
    // No recovery attempts
    // No email notifications
    // No grace period logic
}
```

**Missing Features**:
1. **Retry Schedule**: Exponential backoff (Day 1, 3, 7, 14, 30)
2. **Customer Notifications**: Email on payment failure
3. **Grace Period**: Allow 7-14 days before cancellation
4. **Payment Method Update**: Webhook to request new payment method
5. **Auto-cancel**: Cancel after 30+ days past due

**Impact**:
- Lost revenue (subscriptions never recover)
- Poor customer experience
- No automated recovery workflow

**Workaround**: Merchants must manually monitor `past_due` subscriptions

---

## Summary of Findings

### Confirmed Issues

| # | Issue | Severity | Location | Impact |
|---|-------|----------|----------|--------|
| 1 | ACH Verification Not Checked | CRITICAL | `subscription_service.go:562-569` | $4.5K-9K/month fees |
| 2 | Concurrent Capture Race | CRITICAL | `payment_service.go:527-591` | Double charge risk |
| 3 | Missing Chargeback FK | IMPORTANT | `migrations/004_chargebacks.sql:8` | Data integrity |
| 4 | No Billing Failure Webhooks | IMPORTANT | `subscription_service.go:716-754` | Poor visibility |
| 5 | No Dunning Management | NICE-TO-HAVE | `subscription_service.go:716-754` | Lost revenue |

### Complexity Estimates

| Issue | Complexity | Est. Time | Dependencies |
|-------|------------|-----------|--------------|
| #1 ACH Verification | LOW | 2 hours | Domain method exists |
| #2 Concurrent Capture | MEDIUM | 4 hours | Requires SQL query change |
| #3 Chargeback FK | LOW | 30 min | Migration only |
| #4 Billing Webhooks | MEDIUM | 3 hours | Webhook service exists |
| #5 Dunning Management | HIGH | 16 hours | New feature |

### Recommended Fix Order

**Phase 1** (Critical - Do Immediately):
1. Issue #1: ACH Verification (2 hours) - Highest financial impact
2. Issue #2: Concurrent Capture (4 hours) - Prevents overcharges

**Phase 2** (Important - Do Soon):
3. Issue #3: Chargeback FK (30 min) - Quick data integrity fix
4. Issue #4: Billing Webhooks (3 hours) - Improves observability

**Phase 3** (Nice-to-Have - Future Enhancement):
5. Issue #5: Dunning Management (16 hours) - Revenue optimization

---

## Next Steps

1. **Review Discovery Findings** - Validate findings with stakeholders
2. **Create Implementation Plans** - Detailed fix strategies for each issue
3. **Write Tests** - Integration tests for ACH recurring billing, concurrent capture scenarios
4. **Implement Fixes** - Systematic implementation in priority order
5. **Deploy to Staging** - Test in production-like environment
6. **Monitor Metrics** - Track impact after deployment

---

## Appendices

### Appendix A: Code References

**ACH Verification**:
- Domain validation: `internal/domain/payment_method.go:97-117`
- Subscription billing: `internal/services/subscription/subscription_service.go:529-687`
- ACH verification cron: `cmd/cron/handlers/ach_verification_handler.go`

**Concurrent Capture**:
- Capture logic: `internal/services/payment/payment_service.go:453-714`
- State computation: `internal/services/payment/state_computation.go`
- Transaction tree query: `internal/db/queries/transactions.sql`

**Webhook Infrastructure**:
- Webhook delivery: `internal/services/webhook/webhook_delivery_service.go`
- Webhook subscriptions: `internal/db/migrations/007_webhook_subscriptions.sql`

### Appendix B: Test Files to Update

1. `tests/integration/subscription/recurring_billing_test.go` - Add ACH test
2. `tests/integration/payment/payment_transactions_test.go` - Add concurrent capture test
3. `tests/integration/webhook/webhook_test.go` - Add billing failure webhook test

### Appendix C: Financial Impact Calculations

**ACH Verification Issue**:
- Assumptions:
  - 100 subscriptions/day
  - 10% use ACH
  - 50% of ACH are unverified on first billing attempt
  - 100% return rate for unverified ACH
  - $20 average return fee
- Calculation: 100 × 10% × 50% = 5 returns/day × $20 = $100/day × 30 days = $3,000/month
- **Conservative estimate: $3,000-9,000/month** depending on ACH adoption

**Concurrent Capture Issue**:
- Low probability (~0.01% of captures)
- Average overcharge: $50-500
- Estimated annual impact: $5,000-15,000 (including chargebacks, refunds, customer service)

---

**Discovery Phase Complete** ✅
**Ready for Planning Phase** 🚀
