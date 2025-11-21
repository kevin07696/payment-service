# TODO Implementation Plan

**Date:** 2025-11-20
**Status:** Planning Phase - Awaiting Approval
**Estimated Total Time:** 6-8 hours

## Overview

This plan addresses all 33 TODOs found in the codebase:
- **11 Stale TODOs:** Clean up/remove (30 mins)
- **22 Valid TODOs:** Implement features (6-7.5 hours)

## Phase 1: Clean Up Stale TODOs (30 minutes)

### 1.1 Update Browser Post STORAGE Test Files

**Files to Update:**
- `tests/integration/payment_method/payment_method_test.go`

**Changes:**

#### Test 1: `TestStorePaymentMethod_CreditCard` (Line 18-44)
```go
// REMOVE: t.Skip("TODO: Update to use Browser Post STORAGE...")

// UPDATE: Replace TokenizeAndSaveCard with TokenizeAndSaveCardViaBrowserPost
paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
    t,
    cfg,
    client,
    jwtToken,        // Need to generate JWT token first
    "test-merchant-staging",
    "test-customer-001",
    testutil.TestVisaCard,
    cfg.CallbackBaseURL,
)
```

**Dependencies:** Need to add JWT token generation helper to test setup.

#### Tests 2-5: Similar pattern for other credit card tests (Lines 91, 129, 168, 269)
- Remove `t.Skip()` calls
- Update to use `TokenizeAndSaveCardViaBrowserPost`
- Add JWT token generation

**Action Items:**
- [ ] Create JWT token helper for tests
- [ ] Remove 6 `t.Skip()` calls
- [ ] Update function calls to use `TokenizeAndSaveCardViaBrowserPost`
- [ ] Add proper assertions

### 1.2 Remove Stale TODO Comments

**File:** `tests/integration/payment/browser_post_workflow_test.go:224`
```diff
- // TODO: Implement SALE with stored BRIC when STORAGE endpoint is available
+ // SALE with stored BRIC (using Browser Post STORAGE flow)
```

**File:** `tests/integration/testutil/tokenization.go:392`
```diff
 func TokenizeAndSaveCard(...) (string, error) {
-    // TODO: Update to use Browser Post STORAGE flow
     return "", fmt.Errorf("TokenizeAndSaveCard deprecated - use TokenizeAndSaveCardViaBrowserPost instead")
 }
```

**File:** `tests/integration/fixtures/epx_brics.go:24, 37`
- **Decision Needed:** Keep or remove? Tests work with mock BRICs.
- **Recommendation:** Change to low-priority note, not TODO

### 1.3 Handle Unclear TODO

**File:** `tests/integration/payment_method/payment_method_test.go:216`

**Current:**
```go
t.Skip("TODO: Update to use ConnectRPC StorePaymentMethod endpoint")
```

**Issue:** There's no `StorePaymentMethod` RPC. This test should probably:
1. Use `TokenizeAndSaveCardViaBrowserPost` for credit cards, OR
2. Wait for `StoreACHAccount` if it's testing ACH

**Question for You:** What is this test supposed to validate?

---

## Phase 2: Implement StoreACHAccount RPC (3-4 hours)

### 2.1 Architecture Analysis

**Current State:**
- Handler exists but returns `CodeUnimplemented`
- Database schema ready (payment_methods table supports ACH)
- Browser Post adapter supports STORAGE transaction

**Required Implementation:**

#### Step 1: ACH Account Storage Handler (1.5 hours)

**File:** `internal/handlers/payment_method/payment_method_handler_connect.go`

**Implementation:**
```go
func (h *ConnectHandler) StoreACHAccount(
    ctx context.Context,
    req *connect.Request[paymentmethodv1.StoreACHAccountRequest],
) (*connect.Response[paymentmethodv1.PaymentMethodResponse], error) {

    // 1. Validate request
    if req.Msg.MerchantId == "" || req.Msg.CustomerId == "" {
        return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id and customer_id required"))
    }

    // 2. Get merchant credentials from DB
    merchant, err := h.dbAdapter.Queries().GetMerchantByID(ctx, merchantID)
    if err != nil {
        return nil, handleServiceErrorConnect(err)
    }

    // 3. Fetch EPX MAC secret
    macSecret, err := h.secretManager.GetSecret(ctx, merchant.MacSecretPath)
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // 4. Call Key Exchange for TAC (STORAGE transaction, $0.00)
    tacResp, err := h.keyExchangeAdapter.GetTAC(ctx, &ports.TACRequest{
        MerchantNumber: merchant.MerchantNumber,
        DBANumber:      merchant.DBANumber,
        TerminalNumber: merchant.TerminalNumber,
        Amount:         "0.00",
        TransactionType: "STORAGE",
        AccountType:    req.Msg.AccountType, // CHECKING or SAVINGS
        RoutingNumber:  req.Msg.RoutingNumber,
        AccountNumber:  req.Msg.AccountNumber,
        MACSecret:      macSecret,
    })
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // 5. Submit ACH to EPX Browser Post for STORAGE BRIC
    storageBRIC, err := h.browserPost.SubmitACHStorage(ctx, &ports.ACHStorageRequest{
        TAC:           tacResp.TAC,
        RoutingNumber: req.Msg.RoutingNumber,
        AccountNumber: req.Msg.AccountNumber,
        AccountType:   req.Msg.AccountType,
        NameOnAccount: req.Msg.NameOnAccount,
    })
    if err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // 6. Save storage BRIC to payment_methods table
    paymentMethod, err := h.paymentMethodService.SaveACHPaymentMethod(ctx, &ports.SaveACHPaymentMethodRequest{
        MerchantID:    req.Msg.MerchantId,
        CustomerID:    req.Msg.CustomerId,
        StorageBRIC:   storageBRIC.BRIC,
        AccountType:   req.Msg.AccountType,
        Last4:         req.Msg.AccountNumber[len(req.Msg.AccountNumber)-4:],
        NameOnAccount: req.Msg.NameOnAccount,
        BillingInfo:   req.Msg.BillingInfo,
    })
    if err != nil {
        return nil, handleServiceErrorConnect(err)
    }

    // 7. Return payment method response
    return connect.NewResponse(&paymentmethodv1.PaymentMethodResponse{
        Id:         paymentMethod.ID,
        MerchantId: paymentMethod.MerchantID,
        CustomerId: paymentMethod.CustomerID,
        Type:       "ach",
        Status:     "active",
        CreatedAt:  timestamppb.New(paymentMethod.CreatedAt),
    }), nil
}
```

**Questions for You:**
1. Should ACH accounts require microdeposit verification before use?
2. Or can they be used immediately for payments?
3. Should we auto-trigger verification pre-note on storage?

#### Step 2: Browser Post Adapter Extension (1 hour)

**File:** `internal/adapters/epx/browser_post_adapter.go`

**Add Method:**
```go
func (a *BrowserPostAdapter) SubmitACHStorage(ctx context.Context, req *ports.ACHStorageRequest) (*ports.StorageBRICResponse, error) {
    // Build EPX Browser Post request for ACH STORAGE
    // Submit to EPX
    // Parse response for storage BRIC
    // Return BRIC
}
```

**Question:** Does EPX Browser Post support ACH STORAGE transactions, or only credit cards?

#### Step 3: Payment Method Service Extension (1 hour)

**File:** `internal/services/payment_method/payment_method_service.go`

**Add Method:**
```go
func (s *paymentMethodService) SaveACHPaymentMethod(ctx context.Context, req *ports.SaveACHPaymentMethodRequest) (*domain.PaymentMethod, error) {
    // 1. Create payment method in DB
    // 2. Store storage BRIC encrypted
    // 3. Return payment method
}
```

#### Step 4: Test Utilities (30 minutes)

**File:** `tests/integration/testutil/tokenization.go`

**Update `TokenizeAndSaveACH` from stub:**
```go
func TokenizeAndSaveACH(cfg *Config, client *Client, merchantID, customerID string, achAccount TestACH) (string, error) {
    // Call StoreACHAccount RPC via ConnectRPC client
    resp, err := client.DoConnectRPC("payment_method.v1.PaymentMethodService", "StoreACHAccount", map[string]interface{}{
        "merchant_id":    merchantID,
        "customer_id":    customerID,
        "routing_number": achAccount.RoutingNumber,
        "account_number": achAccount.AccountNumber,
        "account_type":   achAccount.AccountType,
        "name_on_account": achAccount.NameOnAccount,
    })
    // Parse response, return payment method ID
}
```

### 2.2 Testing Strategy

**Unit Tests:**
- [ ] Test `StoreACHAccount` handler validation
- [ ] Test `SaveACHPaymentMethod` service logic
- [ ] Test Browser Post ACH adapter

**Integration Tests (Unskip 11 tests):**
- [ ] `tests/integration/cron/ach_verification_cron_test.go` (3 tests)
- [ ] `tests/integration/payment/payment_ach_verification_test.go` (5 tests)
- [ ] `tests/integration/payment_method/payment_method_test.go` (2 tests)

**Manual Testing:**
- [ ] Test with EPX sandbox ACH account
- [ ] Verify storage BRIC is created
- [ ] Verify payment method is saved to DB

---

## Phase 3: Implement Admin Audit Logging (1 hour)

### 3.1 Simple Zap Logger Approach

**Philosophy:** Use structured logging now, can migrate to audit_logs table later.

#### Step 1: Create Audit Logger Wrapper (20 minutes)

**File:** `internal/util/audit.go` (NEW)

```go
package util

import "go.uber.org/zap"

type AuditEvent struct {
    Action     string // "service.created", "service.rotated", "service.deactivated"
    ServiceID  string
    AdminID    string // From auth context
    Reason     string // Optional
    IPAddress  string
    UserAgent  string
}

func LogAudit(logger *zap.Logger, event AuditEvent) {
    logger.Info("AUDIT",
        zap.String("action", event.Action),
        zap.String("service_id", event.ServiceID),
        zap.String("admin_id", event.AdminID),
        zap.String("reason", event.Reason),
        zap.String("ip_address", event.IPAddress),
        zap.String("user_agent", event.UserAgent),
    )
}
```

#### Step 2: Extract Admin ID from Auth Context (20 minutes)

**File:** `internal/middleware/connect_auth.go`

**Add to JWT validation:**
```go
// After validating JWT, extract admin ID and add to context
type adminContextKey string
const adminIDKey adminContextKey = "admin_id"

// In ValidateJWT:
adminID := claims["admin_id"].(string) // or "sub" depending on JWT structure
ctx = context.WithValue(ctx, adminIDKey, adminID)
```

**Helper function:**
```go
func GetAdminIDFromContext(ctx context.Context) string {
    adminID, ok := ctx.Value(adminIDKey).(string)
    if !ok {
        return "unknown"
    }
    return adminID
}
```

#### Step 3: Add Audit Logging to Admin Handlers (20 minutes)

**File:** `internal/handlers/admin/service_handler.go`

**Update TODO locations:**

Line 77-78:
```go
CreatedBy: pgtype.UUID{
    Bytes: uuid.MustParse(middleware.GetAdminIDFromContext(ctx)),
    Valid: true,
}
```

Line 84:
```go
util.LogAudit(h.logger, util.AuditEvent{
    Action:    "service.created",
    ServiceID: service.ServiceID,
    AdminID:   middleware.GetAdminIDFromContext(ctx),
    IPAddress: getClientIP(ctx),
})
```

Line 137:
```go
util.LogAudit(h.logger, util.AuditEvent{
    Action:    "service.key_rotated",
    ServiceID: req.Msg.ServiceId,
    AdminID:   middleware.GetAdminIDFromContext(ctx),
    Reason:    req.Msg.Reason,
    IPAddress: getClientIP(ctx),
})
```

Line 265:
```go
util.LogAudit(h.logger, util.AuditEvent{
    Action:    "service.deactivated",
    ServiceID: req.Msg.ServiceId,
    AdminID:   middleware.GetAdminIDFromContext(ctx),
    Reason:    req.Msg.Reason,
    IPAddress: getClientIP(ctx),
})
```

**Question:** Do your JWT tokens include an `admin_id` or `sub` claim?

---

## Phase 4: Implement Other Valid TODOs (1.5-2 hours)

### 4.1 Update Payment Method Metadata (30 minutes)

**File:** `internal/handlers/payment_method/payment_method_handler_connect.go:259`

**Implementation:**
```go
func (h *ConnectHandler) UpdatePaymentMethod(
    ctx context.Context,
    req *connect.Request[paymentmethodv1.UpdatePaymentMethodRequest],
) (*connect.Response[paymentmethodv1.PaymentMethodResponse], error) {

    // Validate request
    if req.Msg.PaymentMethodId == "" {
        return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payment_method_id required"))
    }

    // Update metadata only (billing info, nickname)
    // NO changes to payment token/BRIC
    updated, err := h.paymentMethodService.UpdateMetadata(ctx, &ports.UpdatePaymentMethodMetadataRequest{
        PaymentMethodID: req.Msg.PaymentMethodId,
        Nickname:        req.Msg.Nickname,
        BillingInfo:     req.Msg.BillingInfo,
    })
    if err != nil {
        return nil, handleServiceErrorConnect(err)
    }

    return connect.NewResponse(&paymentmethodv1.PaymentMethodResponse{
        Id:         updated.ID,
        MerchantId: updated.MerchantID,
        CustomerId: updated.CustomerID,
        Type:       updated.Type,
        Status:     updated.Status,
        Nickname:   updated.Nickname,
    }), nil
}
```

**Add Service Method:**
```go
// internal/services/payment_method/payment_method_service.go
func (s *paymentMethodService) UpdateMetadata(ctx context.Context, req *ports.UpdatePaymentMethodMetadataRequest) (*domain.PaymentMethod, error) {
    // Update nickname, billing_info only
    // Return updated payment method
}
```

### 4.2 Extract Payment Metadata (30 minutes)

**File:** `internal/handlers/payment/payment_handler.go:381`

**Context:** Currently card type is hardcoded, should extract from EPX metadata.

**Implementation:**
```go
// Extract card type from EPX response metadata
cardType := "credit" // Default
if metadata, ok := epxResponse["CARD_TYPE"]; ok {
    cardType = extractCardType(metadata)
}

func extractCardType(metadata string) string {
    // Parse EPX metadata fields
    // Extract card type (credit/debit)
    // Extract card brand (visa/mastercard/amex/discover)
    return cardType
}
```

**Question:** What metadata fields does EPX return? Do you have example EPX responses?

### 4.3 Fix Service Count Query (15 minutes)

**File:** `internal/handlers/admin/service_handler.go:241`

**Current:**
```go
Total: int64(len(services)), // TODO: Get actual count from DB
```

**Fix:**
```go
// Add count query to sqlc
-- name: CountActiveServices :one
SELECT COUNT(*) FROM services WHERE is_active = true;

// Update handler
count, err := h.dbAdapter.Queries().CountActiveServices(ctx)
if err != nil {
    // Log error but don't fail request
    count = int64(len(services))
}
Total: count,
```

### 4.4 Security Tests (1 hour)

**File:** `tests/integration/auth/epx_callback_auth_test.go`

#### Replay Attack Test (30 minutes)
```go
func TestEPXCallbackAuth_ReplayAttack(t *testing.T) {
    // 1. Make valid EPX callback request
    // 2. Capture the exact request (params + signature)
    // 3. Send same request again (replay)
    // 4. Should reject with 401 Unauthorized
    // 5. Check for "replay detected" or similar message
}
```

**Implementation Notes:**
- Need replay detection mechanism (timestamp validation, nonce tracking)
- EPX includes timestamp in signature - check if too old

#### IP Whitelist Test (30 minutes)
```go
func TestEPXCallbackAuth_IPWhitelist(t *testing.T) {
    // 1. Configure EPX IP whitelist (e.g., 127.0.0.1)
    // 2. Make request from whitelisted IP - should succeed
    // 3. Make request from non-whitelisted IP - should fail
    // 4. Verify 403 Forbidden response
}
```

**Question:** Do you have EPX IP ranges to whitelist? Should this be configurable?

### 4.5 Database Integration Test (15 minutes)

**File:** `internal/adapters/database/postgres_test.go:383`

```go
func TestPostgresAdapter_FullTransactionIntegration(t *testing.T) {
    // 1. Create payment method
    // 2. Create transaction
    // 3. Update transaction state
    // 4. Create refund transaction
    // 5. Verify all relationships in DB
    // 6. Clean up test data
}
```

---

## Phase 5: Testing & Validation (1 hour)

### 5.1 Run All Tests
```bash
# Unit tests
go test ./... -short

# Integration tests (with ACH now working)
go test -tags=integration ./tests/integration/...

# Specific ACH tests
go test -tags=integration ./tests/integration/payment/payment_ach_verification_test.go -v
```

### 5.2 QA Checks
```bash
go vet ./...
staticcheck ./...
golangci-lint run
go build ./...
```

### 5.3 Manual Testing Checklist
- [ ] StoreACHAccount creates payment method
- [ ] ACH payment methods can be used for transactions
- [ ] Audit logs appear in application logs
- [ ] Admin ID is correctly extracted from JWT
- [ ] UpdatePaymentMethod works for metadata
- [ ] Replay attack test passes
- [ ] IP whitelist test passes

---

## Questions Requiring Answers Before Implementation

### Critical (Blocks ACH Implementation):
1. **Does EPX Browser Post support ACH STORAGE transactions?**
   - If no, we need different approach (Server Post?)
2. **Should ACH accounts require microdeposit verification before use?**
   - Affects workflow and database schema
3. **Do JWT tokens include `admin_id` or `sub` claim for admin identification?**
   - Affects audit logging implementation

### Important (Affects Implementation Details):
4. **What EPX metadata fields are available for extracting card type?**
   - Need example EPX response
5. **Should IP whitelist be configurable or hardcoded?**
   - Affects security test design
6. **What is test `payment_method_test.go:216` supposed to validate?**
   - Currently references non-existent RPC

### Low Priority:
7. **Keep or remove EPX BRIC fixture TODOs?**
   - Tests work with mocks, may not need real BRICs

---

## Implementation Order (Recommended)

### Day 1 (4 hours):
1. ✅ Answer critical questions (30 min)
2. Phase 1: Clean up stale TODOs (30 min)
3. Phase 2: Implement StoreACHAccount RPC (3 hours)

### Day 2 (3-4 hours):
4. Phase 3: Implement audit logging (1 hour)
5. Phase 4: Implement other TODOs (1.5-2 hours)
6. Phase 5: Testing & validation (1 hour)

---

## Success Criteria

- [ ] All 11 stale TODOs removed/updated
- [ ] 6-8 Browser Post STORAGE tests unskipped and passing
- [ ] StoreACHAccount RPC fully functional
- [ ] 11 ACH tests unskipped and passing
- [ ] Admin audit logging working with Zap
- [ ] UpdatePaymentMethod implemented
- [ ] Payment metadata extraction working
- [ ] Service count query fixed
- [ ] Security tests implemented and passing
- [ ] All QA checks pass (go vet, staticcheck, build)
- [ ] Integration test suite passes
- [ ] Documentation updated in CHANGELOG.md

---

**Ready for approval. Please answer critical questions above before I begin implementation.**
