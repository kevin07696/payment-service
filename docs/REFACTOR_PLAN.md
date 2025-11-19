# API Endpoint Consolidation - Refactor Plan

**Date**: 2025-11-19
**Objective**: Consolidate API endpoints from 43 → 40 by removing unused RPCs and updating proto definitions

## Current Status

✅ **Database & Queries**: Already up to date (no changes needed)
✅ **Documentation**: Fully updated (AUTHENTICATION.md, API_DESIGN_AND_DATAFLOW.md, CHANGELOG.md)
🔄 **Business Logic**: Needs refactoring (protos, handlers, services)
🧪 **Tests**: Being handled by other agents

---

## Changes Required

### 1. SubscriptionService (8 → 6 endpoints)

**Remove**:
- `PauseSubscription` RPC
- `ResumeSubscription` RPC

**Update**:
- `UpdateSubscription` - Add `optional SubscriptionStatus status` field

**Impact**:
- Proto: `proto/subscription/v1/subscription.proto`
- Handler: `internal/handlers/subscription/subscription_handler.go`
- Generated: `proto/subscription/v1/*.pb.go` (regenerated via `make proto`)

### 2. AdminService (6 → 4 endpoints)

**Remove**:
- `DeactivateService` RPC
- `ActivateService` RPC

**Add**:
- `UpdateService` RPC (with `is_active`, `requests_per_second`, `burst_limit`, `reason` fields)

**Impact**:
- Proto: `proto/admin/v1/admin.proto`
- Handler: `internal/handlers/admin/service_handler.go`
- Generated: `proto/admin/v1/*.pb.go` (regenerated via `make proto`)

### 3. MerchantService (6 → 5 endpoints)

**Remove**:
- `DeactivateMerchant` RPC

**Update**:
- `UpdateMerchant` - Add `optional bool is_active` and `optional string reason` fields

**Impact**:
- Proto: `proto/merchant/v1/merchant.proto`
- Handler: `internal/handlers/merchant/merchant_handler.go`
- Generated: `proto/merchant/v1/*.pb.go` (regenerated via `make proto`)

### 4. PaymentMethodService (10 → 10 endpoints, refined)

**Remove**:
- `SavePaymentMethod` RPC
- `ConvertFinancialBRICToStorageBRIC` RPC

**Add** (REST endpoints, not RPCs):
- `GET /payment-method/v1/payment-form` - GetPaymentForm handler
- `POST /payment/callback` - BrowserPostCallback handler (already exists!)

**Impact**:
- Proto: `proto/payment_method/v1/payment_method.proto`
- Handler: `internal/handlers/payment_method/payment_method_handler.go`
- REST Handler: `internal/handlers/payment/get_payment_form_handler.go` (NEW)
- Callback Handler: `internal/handlers/payment/browser_post_callback_handler.go` (EXISTS)
- Generated: `proto/payment_method/v1/*.pb.go` (regenerated via `make proto`)

---

## Refactor Order (Phases)

### Phase 1: Proto Updates (Foundation)
**Objective**: Update all proto definitions to match documentation

**Steps**:
1. Update `proto/subscription/v1/subscription.proto`
   - Remove `PauseSubscription` RPC (lines 43-45)
   - Remove `ResumeSubscription` RPC (lines 47-49)
   - Remove `PauseSubscriptionRequest` message (lines 100-103)
   - Remove `ResumeSubscriptionRequest` message (lines 105-108)
   - Add `optional SubscriptionStatus status` to `UpdateSubscriptionRequest` (line 89)

2. Update `proto/admin/v1/admin.proto`
   - Remove `DeactivateService` RPC (lines 25-26)
   - Remove `ActivateService` RPC (lines 28-29)
   - Remove `DeactivateServiceRequest` message (lines 121-128)
   - Remove `DeactivateServiceResponse` message (lines 130-134)
   - Remove `ActivateServiceRequest` message (lines 136-140)
   - Remove `ActivateServiceResponse` message (lines 142-146)
   - Add `UpdateService` RPC after `ListServices`
   - Add `UpdateServiceRequest` message
   - Add `UpdateServiceResponse` message

3. Update `proto/merchant/v1/merchant.proto`
   - Remove `DeactivateMerchant` RPC (lines 31-32)
   - Remove `DeactivateMerchantRequest` message (lines 83-87)
   - Add `optional bool is_active` to `UpdateMerchantRequest` (after line 79)
   - Add `optional string reason` to `UpdateMerchantRequest` (for audit trail)

4. Update `proto/payment_method/v1/payment_method.proto`
   - Remove `SavePaymentMethod` RPC (lines 18-20)
   - Remove `ConvertFinancialBRICToStorageBRIC` RPC (lines 46-49)
   - Remove `SavePaymentMethodRequest` message (lines 62-83)
   - Remove `ConvertFinancialBRICRequest` message (lines 145-175)

5. Regenerate proto files
   ```bash
   make proto
   ```

**Validation**:
- All proto files compile successfully
- Generated Go files created in `proto/**/v1/*.pb.go`
- No build errors

---

### Phase 2: Handler Updates (Business Logic)
**Objective**: Update handlers to match new proto definitions

**Steps**:

1. **SubscriptionService Handler** (`internal/handlers/subscription/subscription_handler.go`)
   - Remove `PauseSubscription` method
   - Remove `ResumeSubscription` method
   - Update `UpdateSubscription` to handle `status` field:
     ```go
     if req.Status != nil {
         switch *req.Status {
         case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAUSED:
             // Set status = 'paused', preserve next_billing_date
         case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_ACTIVE:
             // Resume: set status = 'active'
         case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_CANCELLED:
             // Cancel: set status = 'cancelled', cancelled_at = NOW()
         }
     }
     ```

2. **AdminService Handler** (`internal/handlers/admin/service_handler.go`)
   - Remove `DeactivateService` method (lines 245-287)
   - Remove `ActivateService` method (lines 289-329)
   - Add `UpdateService` method:
     ```go
     func (h *ServiceHandler) UpdateService(
         ctx context.Context,
         req *connect.Request[adminv1.UpdateServiceRequest],
     ) (*connect.Response[adminv1.UpdateServiceResponse], error) {
         // Handle is_active, requests_per_second, burst_limit updates
         // Audit log with reason field
     }
     ```

3. **MerchantService Handler** (`internal/handlers/merchant/merchant_handler.go`)
   - Remove `DeactivateMerchant` method
   - Update `UpdateMerchant` to handle `is_active` and `reason` fields:
     ```go
     if req.IsActive != nil {
         params.IsActive = pgtype.Bool{Bool: *req.IsActive, Valid: true}

         // Audit log activation/deactivation
         if req.Reason != nil {
             auditLog.Reason = *req.Reason
         }
     }
     ```

4. **PaymentMethodService Handler** (`internal/handlers/payment_method/payment_method_handler.go`)
   - Remove `SavePaymentMethod` method
   - Remove `ConvertFinancialBRICToStorageBRIC` method
   - Keep all other methods (GetPaymentMethod, ListPaymentMethods, etc.)

5. **Create GetPaymentForm Handler** (`internal/handlers/payment/get_payment_form_handler.go`)
   - NEW FILE - Create REST handler for payment form configuration
   - Returns JSON response with EPX URL, merchant config, callback URL
   - Use Service Token authentication

6. **Verify BrowserPostCallback Handler** (`internal/handlers/payment/browser_post_callback_handler.go`)
   - Already exists! ✅
   - Verify it handles CCE1 (auth), CCE2 (sale), CCE8 (storage)
   - Verify EPX_MAC signature verification

**Validation**:
- All handlers compile
- Removed methods no longer referenced
- New methods properly integrated

---

### Phase 3: Server Registration (Routing)
**Objective**: Update server to register new routes and remove old ones

**Steps**:

1. **Check server registration** (`cmd/server/main.go` or similar)
   - Remove routes for deleted RPCs
   - Add route for `UpdateService` RPC
   - Verify REST routes for GetPaymentForm and BrowserPostCallback

2. **Update ConnectRPC service registration**
   - Ensure all services properly registered
   - Verify middleware applied correctly

**Validation**:
- Server starts without errors
- All routes accessible
- Old routes return 404

---

### Phase 4: SQLC Queries (If Needed)
**Objective**: Verify database queries support new functionality

**Steps**:

1. **Check if new queries needed**:
   - `UpdateService` - Needs query to update `is_active`, `requests_per_second`, `burst_limit`
   - `UpdateSubscription` with status - Needs query to update status
   - `UpdateMerchant` with `is_active` - Needs query to update activation

2. **Add queries if missing** (`internal/db/queries/*.sql`)
   ```sql
   -- name: UpdateServiceActiveStatus :one
   UPDATE services
   SET is_active = $2, updated_at = NOW()
   WHERE id = $1
   RETURNING *;

   -- name: UpdateServiceRateLimits :one
   UPDATE services
   SET requests_per_second = $2, burst_limit = $3, updated_at = NOW()
   WHERE id = $1
   RETURNING *;

   -- name: UpdateSubscriptionStatus :one
   UPDATE subscriptions
   SET status = $2,
       cancelled_at = CASE WHEN $2 = 'cancelled' THEN NOW() ELSE cancelled_at END,
       updated_at = NOW()
   WHERE id = $1
   RETURNING *;

   -- name: UpdateMerchantActiveStatus :one
   UPDATE merchants
   SET is_active = $2, updated_at = NOW()
   WHERE merchant_id = $1
   RETURNING *;
   ```

3. **Regenerate SQLC**
   ```bash
   make sqlc
   ```

**Validation**:
- SQLC generates successfully
- New query functions available in generated code

---

### Phase 5: Integration & Testing
**Objective**: Ensure all components work together

**Steps**:

1. **Build verification**
   ```bash
   make build
   make lint
   go vet ./...
   ```

2. **Manual API testing**
   - Test `UpdateSubscription` with status field
   - Test `UpdateService` RPC
   - Test `UpdateMerchant` with is_active
   - Test `GetPaymentForm` REST endpoint
   - Test `BrowserPostCallback` with EPX simulator

3. **Verify error handling**
   - Invalid status transitions
   - Missing required fields
   - Authorization checks

**Validation**:
- All builds succeed
- No linting errors
- Manual tests pass

---

## Files to Modify

### Proto Files (4 files)
```
proto/subscription/v1/subscription.proto
proto/admin/v1/admin.proto
proto/merchant/v1/merchant.proto
proto/payment_method/v1/payment_method.proto
```

### Handler Files (5 files)
```
internal/handlers/subscription/subscription_handler.go
internal/handlers/admin/service_handler.go
internal/handlers/merchant/merchant_handler.go
internal/handlers/payment_method/payment_method_handler.go
internal/handlers/payment/get_payment_form_handler.go (NEW)
internal/handlers/payment/browser_post_callback_handler.go (VERIFY)
```

### Query Files (1 file, if needed)
```
internal/db/queries/services.sql
internal/db/queries/subscriptions.sql
internal/db/queries/merchants.sql
```

### Server Files (1-2 files)
```
cmd/server/main.go (or wherever routes are registered)
```

---

## Risk Assessment

### Low Risk
- Proto updates (backward compatible with optional fields)
- Removing unused RPCs (no current usage)
- Adding new UpdateService RPC

### Medium Risk
- Consolidating Pause/Resume into UpdateSubscription
- Handler method removal (ensure no internal callers)

### High Risk
- None identified (database already supports all fields)

---

## Rollback Plan

If issues arise:

1. **Proto rollback**: Revert proto files, regenerate
2. **Handler rollback**: Restore old handler methods
3. **Database**: No rollback needed (DB unchanged)

---

## Success Criteria

✅ All proto files compile
✅ All handlers compile and register correctly
✅ Server starts without errors
✅ All endpoints accessible via API
✅ Documentation matches implementation
✅ Tests pass (when test agent completes)
✅ No breaking changes to existing clients

---

## Notes

- **Browser Post Callback**: Already implemented at `internal/handlers/payment/browser_post_callback_handler.go`
- **GetPaymentForm**: Needs new REST handler implementation
- **Database**: Already supports all required fields (verified)
- **Queries**: May need new SQLC queries for update operations
- **Tests**: Being handled by separate test-writing agents

---

## Next Steps

1. Start with **Phase 1: Proto Updates**
2. Proceed sequentially through phases
3. Validate at each phase before proceeding
4. Coordinate with test agents for integration testing
