# ACH Safe Verification Implementation Guide

**Last Updated**: 2025-11-19
**Status**: Ready for Implementation
**Purpose**: Implement safe ACH verification flow with proper verification status tracking

---

## Overview

This document describes the **safest approach** for ACH account verification using pre-notes, with proper handling of the 1-3 day verification window and ACH return codes.

### Key Principle

**Never mark an ACH account as verified until the bank actually verifies it** (3 days after pre-note with no returns).

---

## Architecture Changes

### Database Schema (Migration 009)

New fields in `customer_payment_methods`:

```sql
verification_status VARCHAR(20)           -- 'pending', 'verified', 'failed'
prenote_transaction_id UUID              -- Links to pre-note transaction
verified_at TIMESTAMPTZ                  -- When verification completed
verification_failure_reason TEXT         -- Why verification failed
return_count INTEGER DEFAULT 0           -- Number of ACH returns
deactivation_reason VARCHAR(100)         -- Why payment method deactivated
deactivated_at TIMESTAMPTZ               -- When deactivated
```

### Verification Status Flow

```
┌─────────────────────────────────────────────────────────┐
│                ACH Verification Lifecycle                │
└─────────────────────────────────────────────────────────┘

Day 0: StoreACHAccount called
    ↓
Send CKC0 (pre-note) to EPX
    ↓
EPX responds "00" immediately (accepted for processing)
    ↓
Send CKC8 (Storage BRIC conversion)
    ↓
Store in DB:
    is_verified = false
    verification_status = 'pending'
    prenote_transaction_id = [CKC0 transaction ID]
    ↓
Return to customer immediately

Day 0-3: Grace Period
    ↓
ACHDebit allowed (optimistic - within grace period)
    ↓
[Two possible outcomes]

Outcome 1: No Return Code (Success)          Outcome 2: Return Code (Failure)
    ↓                                            ↓
Day 3+: Cron job runs                        Day 1-3: EPX sends return code
    ↓                                            ↓
UPDATE:                                       Webhook handler processes:
    is_verified = true                           is_verified = false
    verification_status = 'verified'             verification_status = 'failed'
    verified_at = NOW()                          is_active = false
    ↓                                            deactivation_reason = 'verification_failed'
Account fully verified                           verification_failure_reason = 'R03: No Account'
                                                 ↓
                                             Account unusable
```

---

## Implementation Components

### 1. StoreACHAccount Service (Updated)

**File**: `internal/services/payment_method/payment_method_service.go`

```go
func (s *PaymentMethodService) StoreACHAccount(ctx context.Context,
    req *ports.StoreACHAccountRequest) (*domain.PaymentMethod, error) {

    logger := s.logger.With(
        zap.String("merchant_id", req.MerchantID),
        zap.String("customer_id", req.CustomerID),
    )

    // 1. Validate inputs
    if err := validateACHAccount(req); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // 2. Check idempotency (by routing + last 4)
    existing, err := s.db.GetPaymentMethodByRoutingAndLastFour(ctx,
        req.MerchantID, req.CustomerID, req.RoutingNumber, getLastFour(req.AccountNumber))
    if err == nil && existing != nil {
        logger.Info("Payment method already exists (idempotent)")
        return existing, nil
    }

    // 3. Send Pre-note (CKC0) to EPX
    logger.Info("Sending pre-note transaction (CKC0)")
    preNoteReq := &ports.ServerPostRequest{
        TransactionType: ports.TransactionTypeACHPreNoteDebit,
        Amount:          "0.00",
        AccountNumber:   &req.AccountNumber,
        RoutingNumber:   &req.RoutingNumber,
        ReceiverName:    &req.AccountHolderName,
        StdEntryClass:   mapStdEntryClass(req.StdEntryClass),
        CardEntryMethod: stringPtr("X"), // Manual entry
    }

    preNoteResp, err := s.serverPost.ProcessTransaction(ctx, preNoteReq)
    if err != nil {
        return nil, fmt.Errorf("pre-note transaction failed: %w", err)
    }

    if preNoteResp.AuthResp != "00" {
        return nil, fmt.Errorf("pre-note declined: %s - %s",
            preNoteResp.AuthResp, preNoteResp.AuthRespText)
    }

    logger.Info("Pre-note accepted by EPX",
        zap.String("auth_resp", preNoteResp.AuthResp),
        zap.String("tran_nbr", preNoteResp.TranNbr))

    // 4. Convert Financial BRIC to Storage BRIC (CKC8)
    logger.Info("Converting to Storage BRIC (CKC8)")
    storageBRICReq := &ports.ServerPostRequest{
        TransactionType:  ports.TransactionTypeBRICStorageACH,
        Amount:           "0.00",
        OriginalAuthGUID: preNoteResp.AuthGUID,
        CardEntryMethod:  stringPtr("Z"), // Token
    }

    storageBRICResp, err := s.serverPost.ProcessTransaction(ctx, storageBRICReq)
    if err != nil {
        return nil, fmt.Errorf("storage BRIC creation failed: %w", err)
    }

    if storageBRICResp.AuthResp != "00" {
        return nil, fmt.Errorf("storage BRIC declined: %s - %s",
            storageBRICResp.AuthResp, storageBRICResp.AuthRespText)
    }

    logger.Info("Storage BRIC created",
        zap.String("storage_bric", storageBRICResp.AuthGUID[:10]+"..."))

    // 5. Create pre-note transaction record
    preNoteTxID := uuid.New()
    preNoteTx := &domain.Transaction{
        ID:                uuid.MustParse(preNoteTxID.String()),
        MerchantID:        uuid.MustParse(req.MerchantID),
        CustomerID:        uuidPtr(req.CustomerID),
        AmountCents:       0, // Pre-note is $0.00
        Currency:          "USD",
        Type:              domain.TransactionTypePreNote,
        PaymentMethodType: domain.PaymentMethodTypeACH,
        TranNbr:           &preNoteResp.TranNbr,
        AuthResp:          &preNoteResp.AuthResp,
        AuthGUID:          &preNoteResp.AuthGUID,
        ProcessedAt:       timePtr(time.Now()),
    }

    err = s.db.CreateTransaction(ctx, preNoteTx)
    if err != nil {
        return nil, fmt.Errorf("failed to store pre-note transaction: %w", err)
    }

    // 6. Store payment method with PENDING verification status
    pmID := uuid.New()
    pm := &domain.PaymentMethod{
        ID:                   uuid.MustParse(pmID.String()),
        MerchantID:           uuid.MustParse(req.MerchantID),
        CustomerID:           uuid.MustParse(req.CustomerID),
        PaymentToken:         storageBRICResp.AuthGUID,
        PaymentType:          domain.PaymentMethodTypeACH,
        LastFour:             getLastFour(req.AccountNumber),
        BankName:             req.BankName,
        AccountType:          string(req.AccountType),
        IsDefault:            req.IsDefault,
        IsActive:             true,

        // Key: NOT verified yet! EPX accepted, but bank hasn't verified
        IsVerified:           false,
        VerificationStatus:   domain.VerificationStatusPending,
        PreNoteTransactionID: &preNoteTxID,
    }

    err = s.db.CreatePaymentMethod(ctx, pm)
    if err != nil {
        return nil, fmt.Errorf("failed to store payment method: %w", err)
    }

    logger.Info("ACH payment method created",
        zap.String("payment_method_id", pm.ID.String()),
        zap.String("verification_status", string(pm.VerificationStatus)),
        zap.Bool("is_verified", pm.IsVerified))

    return pm, nil
}

func validateACHAccount(req *ports.StoreACHAccountRequest) error {
    // Validate routing number (ABA checksum)
    if !isValidRoutingNumber(req.RoutingNumber) {
        return errors.New("invalid routing number")
    }

    // Validate account number length
    if len(req.AccountNumber) < 4 || len(req.AccountNumber) > 17 {
        return errors.New("account number must be 4-17 digits")
    }

    // Validate account holder name
    if req.AccountHolderName == "" {
        return errors.New("account holder name required")
    }

    return nil
}

func isValidRoutingNumber(routing string) bool {
    // ABA routing number checksum validation
    if len(routing) != 9 {
        return false
    }

    weights := []int{3, 7, 1, 3, 7, 1, 3, 7, 1}
    sum := 0

    for i, digit := range routing {
        n := int(digit - '0')
        sum += n * weights[i]
    }

    return sum%10 == 0
}
```

### 2. ACHDebit Service with Grace Period

**File**: `internal/services/payment/payment_service.go`

```go
const (
    // ACH verification grace period
    // Allows transactions during 0-3 day verification window
    ACHVerificationGracePeriod = 3 * 24 * time.Hour
)

func (s *PaymentService) ACHDebit(ctx context.Context,
    req *ports.ACHDebitRequest) (*domain.Transaction, error) {

    logger := s.logger.With(
        zap.String("merchant_id", req.MerchantID),
        zap.String("payment_method_id", req.PaymentMethodID),
    )

    // 1. Get payment method
    pm, err := s.db.GetPaymentMethodByID(ctx, uuid.MustParse(req.PaymentMethodID))
    if err != nil {
        return nil, fmt.Errorf("payment method not found: %w", err)
    }

    // 2. Validate payment method type
    if pm.PaymentType != domain.PaymentMethodTypeACH {
        return nil, errors.New("payment method is not ACH")
    }

    // 3. Check if payment method is active
    if !pm.IsActive {
        return nil, errors.New("payment method is inactive")
    }

    // 4. Check verification with grace period
    withinGracePeriod := time.Since(pm.CreatedAt) < ACHVerificationGracePeriod

    if !pm.IsVerified && !withinGracePeriod {
        logger.Warn("ACH debit rejected: not verified and outside grace period",
            zap.String("verification_status", string(pm.VerificationStatus)),
            zap.Time("created_at", pm.CreatedAt),
            zap.Duration("age", time.Since(pm.CreatedAt)))

        return nil, fmt.Errorf("payment method not verified (created %s ago, grace period: %s)",
            time.Since(pm.CreatedAt).Round(time.Hour), ACHVerificationGracePeriod)
    }

    if !pm.IsVerified && withinGracePeriod {
        logger.Info("ACH debit allowed: within grace period (optimistic verification)",
            zap.String("verification_status", string(pm.VerificationStatus)),
            zap.Duration("age", time.Since(pm.CreatedAt)))
    }

    // 5. Check amount limits
    if req.AmountCents <= 0 {
        return nil, errors.New("amount must be greater than 0")
    }

    maxAmount := int64(2500000) // $25,000 default limit
    if req.AmountCents > maxAmount {
        return nil, fmt.Errorf("amount exceeds daily limit ($%s)",
            centsToDecimalString(maxAmount))
    }

    // 6. Create transaction record (status will be pending initially)
    txID := uuid.New()
    tx := &domain.Transaction{
        ID:                uuid.MustParse(txID.String()),
        MerchantID:        uuid.MustParse(req.MerchantID),
        CustomerID:        uuidPtr(req.CustomerID),
        AmountCents:       req.AmountCents,
        Currency:          req.Currency,
        Type:              domain.TransactionTypeCharge,
        PaymentMethodType: domain.PaymentMethodTypeACH,
        PaymentMethodID:   &pm.ID,
    }

    err = s.db.CreateTransaction(ctx, tx)
    if err != nil {
        return nil, fmt.Errorf("failed to create transaction: %w", err)
    }

    // 7. Send ACH debit to EPX (CKC2)
    logger.Info("Sending ACH debit to EPX (CKC2)",
        zap.String("amount", centsToDecimalString(req.AmountCents)))

    epxReq := &ports.ServerPostRequest{
        TransactionType: ports.TransactionTypeACHDebit,
        Amount:          centsToDecimalString(req.AmountCents),
        AuthGUID:        pm.PaymentToken,
        CardEntryMethod: stringPtr("Z"), // Token
        IndustryType:    stringPtr("E"), // E-commerce
    }

    epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
    if err != nil {
        // Update transaction as failed
        s.db.UpdateTransactionStatus(ctx, tx.ID, "failed")
        return nil, fmt.Errorf("EPX transaction failed: %w", err)
    }

    // 8. Update transaction with EPX response
    tx.AuthResp = &epxResp.AuthResp
    tx.AuthGUID = &epxResp.AuthGUID
    tx.TranNbr = &epxResp.TranNbr
    tx.AuthCode = &epxResp.AuthCode
    tx.ProcessedAt = timePtr(time.Now())

    err = s.db.UpdateTransaction(ctx, tx)
    if err != nil {
        return nil, fmt.Errorf("failed to update transaction: %w", err)
    }

    // 9. Update payment method last_used_at
    err = s.db.MarkPaymentMethodUsed(ctx, pm.ID)
    if err != nil {
        logger.Warn("Failed to update payment method last_used_at", zap.Error(err))
    }

    logger.Info("ACH debit completed",
        zap.String("transaction_id", tx.ID.String()),
        zap.String("auth_resp", *tx.AuthResp),
        zap.String("tran_nbr", *tx.TranNbr))

    return tx, nil
}
```

### 3. Cron Job: Verify Pending ACH Accounts

**File**: `cmd/cron/verify_ach_accounts.go`

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "time"

    "github.com/kevin07696/payment-service/internal/adapters/database"
    "github.com/kevin07696/payment-service/internal/db/sqlc"
    "go.uber.org/zap"
)

func main() {
    var (
        databaseURL = flag.String("database-url", "", "PostgreSQL connection URL")
        batchSize   = flag.Int("batch-size", 100, "Number of records to process per batch")
        dryRun      = flag.Bool("dry-run", false, "Don't actually update, just log what would happen")
    )
    flag.Parse()

    // Setup logger
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // Connect to database
    ctx := context.Background()
    dbConfig := database.DefaultPostgreSQLConfig(*databaseURL)

    db, err := database.NewPostgreSQLAdapter(ctx, dbConfig, logger)
    if err != nil {
        logger.Fatal("Failed to connect to database", zap.Error(err))
    }
    defer db.Close()

    queries := db.Queries()

    // Run verification
    processed, verified, failed, err := verifyPendingACHAccounts(ctx, queries, *batchSize, *dryRun, logger)
    if err != nil {
        logger.Fatal("Verification failed", zap.Error(err))
    }

    logger.Info("ACH verification completed",
        zap.Int("processed", processed),
        zap.Int("verified", verified),
        zap.Int("failed", failed))
}

func verifyPendingACHAccounts(
    ctx context.Context,
    queries *sqlc.Queries,
    batchSize int,
    dryRun bool,
    logger *zap.Logger,
) (processed, verified, failed int, err error) {

    // Get ACH payment methods pending verification older than 3 days
    cutoffDate := time.Now().Add(-3 * 24 * time.Hour)

    logger.Info("Fetching pending ACH verifications",
        zap.Time("cutoff_date", cutoffDate),
        zap.Int("batch_size", batchSize))

    pendingMethods, err := queries.GetPendingACHVerifications(ctx, sqlc.GetPendingACHVerificationsParams{
        CutoffDate: cutoffDate,
        LimitCount: int32(batchSize),
    })
    if err != nil {
        return 0, 0, 0, fmt.Errorf("failed to fetch pending verifications: %w", err)
    }

    logger.Info("Found pending ACH verifications",
        zap.Int("count", len(pendingMethods)))

    for _, pm := range pendingMethods {
        processed++

        logger := logger.With(
            zap.String("payment_method_id", pm.ID.String()),
            zap.String("merchant_id", pm.MerchantID.String()),
            zap.String("customer_id", pm.CustomerID.String()),
            zap.String("last_four", pm.LastFour),
        )

        // Check if pre-note transaction has any return codes
        hasReturn := false
        var returnCode string

        if pm.PrenoteTransactionID != nil {
            // Check transaction for return code in metadata
            tx, err := queries.GetTransactionByID(ctx, *pm.PrenoteTransactionID)
            if err == nil && tx.Metadata != nil {
                if rc, ok := tx.Metadata["return_code"].(string); ok && rc != "" {
                    hasReturn = true
                    returnCode = rc
                }
            }
        }

        if hasReturn {
            // Verification failed - return code received
            failed++

            logger.Warn("ACH verification failed: return code received",
                zap.String("return_code", returnCode))

            if !dryRun {
                failureReason := fmt.Sprintf("Pre-note return: %s", returnCode)
                err = queries.MarkVerificationFailed(ctx, sqlc.MarkVerificationFailedParams{
                    ID:            pm.ID,
                    FailureReason: failureReason,
                })
                if err != nil {
                    logger.Error("Failed to mark verification as failed", zap.Error(err))
                    continue
                }
            }

            logger.Info("Marked verification as failed")
        } else {
            // No return code after 3 days = verification successful
            verified++

            logger.Info("ACH verification successful: no returns after 3 days")

            if !dryRun {
                err = queries.MarkPaymentMethodVerified(ctx, pm.ID)
                if err != nil {
                    logger.Error("Failed to mark payment method as verified", zap.Error(err))
                    continue
                }
            }

            logger.Info("Marked payment method as verified")
        }
    }

    return processed, verified, failed, nil
}
```

**Crontab Entry**:
```bash
# Run every hour to verify pending ACH accounts
0 * * * * /usr/local/bin/verify-ach-accounts --database-url="postgres://..." >> /var/log/verify-ach.log 2>&1
```

### 4. Webhook Handler: ACH Return Codes

**File**: `internal/handlers/payment/browser_post_callback_handler.go` (enhancement)

```go
func (h *BrowserPostCallbackHandler) handleACHReturn(
    ctx context.Context,
    data *CallbackData,
) error {
    logger := h.logger.With(
        zap.String("tran_nbr", data.TranNbr),
        zap.String("return_code", data.AuthResp),
    )

    logger.Info("Processing ACH return code")

    // 1. Find original transaction by tran_nbr
    tx, err := h.db.GetTransactionByTranNbr(ctx, data.TranNbr)
    if err != nil {
        return fmt.Errorf("transaction not found: %w", err)
    }

    // 2. Update transaction with return information
    metadata := tx.Metadata
    if metadata == nil {
        metadata = make(map[string]interface{})
    }
    metadata["return_code"] = data.AuthResp
    metadata["return_description"] = data.ResponseText
    metadata["return_received_at"] = time.Now().Format(time.RFC3339)

    err = h.db.UpdateTransactionMetadata(ctx, tx.ID, metadata)
    if err != nil {
        return fmt.Errorf("failed to update transaction: %w", err)
    }

    logger.Info("Updated transaction with return code",
        zap.String("transaction_id", tx.ID.String()))

    // 3. Get associated payment method
    if tx.PaymentMethodID == nil {
        logger.Warn("Transaction has no associated payment method")
        return nil
    }

    pm, err := h.db.GetPaymentMethodByID(ctx, *tx.PaymentMethodID)
    if err != nil {
        return fmt.Errorf("payment method not found: %w", err)
    }

    // 4. Handle return code based on severity
    returnCode := data.AuthResp

    // Critical return codes: immediate deactivation
    criticalCodes := map[string]string{
        "R02": "Account Closed",
        "R03": "No Account/Unable to Locate Account",
        "R04": "Invalid Account Number",
        "R05": "Unauthorized Debit",
        "R07": "Authorization Revoked",
        "R10": "Customer Advises Not Authorized",
    }

    if description, isCritical := criticalCodes[returnCode]; isCritical {
        logger.Warn("Critical ACH return code - marking verification failed",
            zap.String("payment_method_id", pm.ID.String()),
            zap.String("return_code", returnCode),
            zap.String("description", description))

        failureReason := fmt.Sprintf("%s: %s", returnCode, description)
        err = h.db.MarkVerificationFailed(ctx, sqlc.MarkVerificationFailedParams{
            ID:            pm.ID,
            FailureReason: failureReason,
        })
        if err != nil {
            return fmt.Errorf("failed to mark verification failed: %w", err)
        }

        return nil
    }

    // Non-critical return codes: increment return count
    // Auto-deactivate after 2 returns
    logger.Info("Incrementing return count",
        zap.String("payment_method_id", pm.ID.String()),
        zap.Int("current_return_count", pm.ReturnCount))

    err = h.db.IncrementReturnCount(ctx, sqlc.IncrementReturnCountParams{
        ID:                     pm.ID,
        DeactivationThreshold:  2, // Deactivate after 2 returns
    })
    if err != nil {
        return fmt.Errorf("failed to increment return count: %w", err)
    }

    // Log if payment method was auto-deactivated
    pm, _ = h.db.GetPaymentMethodByID(ctx, pm.ID)
    if !pm.IsActive {
        logger.Warn("Payment method auto-deactivated due to excessive returns",
            zap.String("payment_method_id", pm.ID.String()),
            zap.Int("return_count", pm.ReturnCount))
    }

    return nil
}
```

---

## Testing Strategy

### Unit Tests

1. **StoreACHAccount**:
   - ✓ Stores with `is_verified=false`, `verification_status='pending'`
   - ✓ Links to pre-note transaction
   - ✓ Validates routing number checksum
   - ✓ Idempotency works correctly

2. **ACHDebit**:
   - ✓ Allows debit within grace period (unverified)
   - ✓ Rejects debit outside grace period (unverified)
   - ✓ Allows debit when verified
   - ✓ Rejects debit when inactive

3. **Cron Job**:
   - ✓ Marks verified after 3 days with no returns
   - ✓ Marks failed when return code present
   - ✓ Processes batch correctly
   - ✓ Dry-run mode works

4. **Return Handler**:
   - ✓ Critical codes → immediate deactivation
   - ✓ Non-critical codes → increment count
   - ✓ Auto-deactivate after 2 returns
   - ✓ Updates transaction metadata

### Integration Tests

See `docs/INTEGRATION_TEST_PLAN.md` for detailed integration test specifications.

---

## Deployment Checklist

- [ ] Run migration 009
- [ ] Generate sqlc code: `sqlc generate`
- [ ] Update domain models with new fields
- [ ] Deploy StoreACHAccount service changes
- [ ] Deploy ACHDebit service changes
- [ ] Deploy webhook handler changes
- [ ] Deploy cron job
- [ ] Set up cron schedule
- [ ] Monitor cron job logs for first 7 days
- [ ] Monitor webhook handler for return codes

---

## Monitoring

### Key Metrics

1. **Verification Success Rate**:
   ```sql
   SELECT
       COUNT(*) FILTER (WHERE verification_status = 'verified') AS verified,
       COUNT(*) FILTER (WHERE verification_status = 'failed') AS failed,
       COUNT(*) FILTER (WHERE verification_status = 'pending') AS pending
   FROM customer_payment_methods
   WHERE payment_type = 'ach'
     AND created_at > NOW() - INTERVAL '30 days';
   ```

2. **Grace Period Usage**:
   ```sql
   SELECT COUNT(*)
   FROM transactions t
   JOIN customer_payment_methods pm ON t.payment_method_id = pm.id
   WHERE pm.payment_type = 'ach'
     AND pm.is_verified = false
     AND t.created_at > NOW() - INTERVAL '30 days';
   ```

3. **Return Code Distribution**:
   ```sql
   SELECT
       metadata->>'return_code' AS return_code,
       metadata->>'return_description' AS description,
       COUNT(*)
   FROM transactions
   WHERE metadata->>'return_code' IS NOT NULL
     AND created_at > NOW() - INTERVAL '30 days'
   GROUP BY metadata->>'return_code', metadata->>'return_description'
   ORDER BY COUNT(*) DESC;
   ```

### Alerts

1. **High Verification Failure Rate**:
   - Alert if > 10% of ACH verifications fail in 24 hours
   - Indicates potential routing number validation issue

2. **Pending Verifications Stuck**:
   - Alert if pending verifications > 7 days old
   - Indicates cron job not running

3. **High Return Rate**:
   - Alert if return rate > 2% in 7 days
   - Indicates potential fraud or data quality issue

---

## Summary

This implementation provides:

✅ **Accurate Verification Tracking**: Never claims verified before bank verifies
✅ **Customer-Friendly UX**: Grace period allows immediate use
✅ **NACHA Compliance**: Proper pre-note handling
✅ **Robust Return Handling**: Auto-deactivation on excessive returns
✅ **Production Ready**: Monitoring, alerts, and cron jobs

**This is the safest approach for ACH verification.**
