# TDD-Based API Consolidation Refactor Plan

**Date**: 2025-11-19
**Last Updated**: 2025-11-19
**Approach**: Test-Driven Development (Red → Green → Refactor)
**Objective**: Consolidate 43 → 40 endpoints using TDD methodology
**Strategy**: Sequential phases, minimal implementation then refactor, propose queries for review

---

## 📚 Key Document References

This plan references and builds upon the following documents:

| Document | Purpose | Used For |
|----------|---------|----------|
| **[UNIT_TEST_REFACTORING_ANALYSIS.md](./UNIT_TEST_REFACTORING_ANALYSIS.md)** | Mock/fixture patterns, test duplication analysis | Phase 0: Test infrastructure design |
| **[API_DESIGN_AND_DATAFLOW.md](./API_DESIGN_AND_DATAFLOW.md)** | Complete API specifications, dataflows, EPX integration | Phases 1-4: Implementation reference |
| **[AUTHENTICATION.md](./AUTHENTICATION.md)** | Authentication strategy per endpoint, token types | Phases 1-4: Auth requirements |
| **[E2E_TEST_DESIGN.md](./E2E_TEST_DESIGN.md)** | E2E vs integration test classification | Phase 5: Test coordination |

**Usage**: Each phase links to specific sections of these documents where relevant.

---

## 🔌 EPX Field Reference

**CRITICAL**: EPX field names vs Database field names - DO NOT CONFUSE THEM!

### EPX Transaction Relationship Fields

| EPX Field | Type | Purpose | Valid Values | Usage |
|-----------|------|---------|--------------|-------|
| `TRAN_TYPE` | Code | Operation type | CCE1, CCE2, CCE4, CCE9, CCE8, CCEX, CKC2, etc. | Specifies what operation to perform |
| **`TRAN_GROUP`** | String | **Transaction CLASS** | **"AUTH"** or **"SALE"** | **NOT for linking!** Indicates if auth-only or immediate capture |
| `TRAN_NBR` | String(10) | Unique transaction ID | Numeric string | Derived from UUID, identifies this specific transaction |
| `AUTH_GUID` | String(20) | BRIC token FOR this txn | Alphanumeric | **Returned BY EPX** for this transaction |
| `ORIG_AUTH_GUID` | String(20) | Parent's BRIC token | Alphanumeric | **Used TO link** to parent transaction |
| `ORIG_AUTH_AMOUNT` | Decimal | Original amount | e.g., "100.00" | Required for CAPTURE/REFUND |
| `ORIG_AUTH_TRAN_IDENT` | String(15) | Network Transaction ID | From parent response | Required for COF compliance |

### Database Transaction Relationship Fields

| Database Field | Type | Purpose | Usage |
|----------------|------|---------|-------|
| `parent_transaction_id` | UUID | Links to parent transaction | CAPTURE→AUTH, REFUND→SALE/CAPTURE, VOID→AUTH/SALE |
| `auth_guid` | TEXT | Stores EPX's AUTH_GUID | BRIC token returned for THIS transaction |
| `tran_nbr` | TEXT | Stores EPX's TRAN_NBR | 10-digit deterministic ID from UUID |

### ⚠️ CRITICAL: `TRAN_GROUP` vs `parent_transaction_id`

**WRONG (Misuse)**:
```go
// ❌ Using TRAN_GROUP as a grouping mechanism
TranGroup: groupID.String()  // WRONG! TRAN_GROUP is "AUTH" or "SALE"
```

**CORRECT (Proper Usage)**:
```go
// ✅ TRAN_GROUP specifies transaction type
TranGroup: "AUTH"  // Authorization-only transaction
TranGroup: "SALE"  // Combined auth + capture

// ✅ Use parent_transaction_id for linking in database
ParentTransactionID: &authTransactionID  // Links CAPTURE to AUTH

// ✅ Use ORIG_AUTH_GUID for linking in EPX API
OrigAuthGUID: authTransaction.AuthGuid  // Uses parent's BRIC
```

### Transaction Chain Pattern (EPX + Database)

```
1. AUTH (CCE2)
   EPX Request:  TRAN_TYPE=CCE2, TRAN_GROUP="AUTH"
   EPX Response: AUTH_GUID="09LMQ886..."
   Database:     parent_transaction_id=NULL, auth_guid="09LMQ886..."

2. CAPTURE (CCE4)
   EPX Request:  TRAN_TYPE=CCE4, ORIG_AUTH_GUID="09LMQ886..."
   EPX Response: AUTH_GUID="NEW_BRIC..."
   Database:     parent_transaction_id=<AUTH_ID>, auth_guid="NEW_BRIC..."

3. REFUND (CCE9)
   EPX Request:  TRAN_TYPE=CCE9, ORIG_AUTH_GUID="NEW_BRIC..."
   EPX Response: AUTH_GUID="REFUND_BRIC..."
   Database:     parent_transaction_id=<CAPTURE_ID>, auth_guid="REFUND_BRIC..."
```

### Quick Reference: When to Use Each Field

- **`TRAN_GROUP`**: Only when specifying AUTH vs SALE transaction type
- **`ORIG_AUTH_GUID`**: When making EPX API call that references parent transaction
- **`parent_transaction_id`**: When storing transaction chain relationship in database
- **`auth_guid`**: When storing EPX's returned BRIC token for this transaction

**Sources**:
- `/home/kevinlam/Downloads/supplemental-resources/supplemental-resources/APIs/EPX_API_REFERENCE.md`
- `/home/kevinlam/Downloads/supplemental-resources/supplemental-resources/Dictionaries/EPX_FIELD_DICTIONARY.md`
- `internal/adapters/ports/server_post.go`
- `internal/db/migrations/003_transactions.sql`

---

## ⚠️ Deviation Protocol

### When Deviations Are Allowed

Deviations from this plan are permitted when:

1. **Blocking Issues Discovered**
   - Proto compilation fails unexpectedly
   - Database migration conflict found
   - SQLC query generation error

2. **Better Approach Identified**
   - During TDD, discover cleaner implementation
   - Simpler solution emerges during refactor
   - Performance optimization opportunity

3. **Test Failures Reveal Issues**
   - Unit test uncovers edge case
   - Integration test reveals auth hole
   - Existing test conflicts with changes

### How to Document Deviations

**Mark deviations clearly in progress updates**:

```
⚠️ DEVIATION: [Phase] [Component]

Why: [Concise 1-2 sentence explanation]
Impact: [What changed from plan]
Approved: [If needed, note approval]

Example:
⚠️ DEVIATION: Phase 2 - AdminService UpdateService query

Why: UpdateServiceActiveStatus query already exists in services.sql
Impact: Skipped query creation step, used existing query
Approved: N/A (no new query needed)
```

### What Requires Pre-Approval

**Must propose and get approval before**:

1. **SQLC Query Changes**
   - Adding new queries to `internal/db/queries/*.sql`
   - Modifying existing query signatures
   - **Protocol**: Show SQL, explain purpose, wait for approval

2. **Proto Signature Changes**
   - Changing field types (breaking change)
   - Renaming fields (breaking change)
   - **Protocol**: Show proto diff, explain impact, get approval

3. **Architecture Changes**
   - Moving handlers to different package
   - Changing service dependencies
   - **Protocol**: Explain rationale, discuss alternatives

**Deviations that don't need approval**:
- Test implementation details
- Refactoring internal code structure
- Adding helper functions
- Improving error messages

---

## 🎯 TDD Philosophy for This Refactor

### Why TDD Works Here

1. **Clear Requirements**: Documentation already defines expected behavior
2. **Existing Tests**: Can leverage `UNIT_TEST_REFACTORING_ANALYSIS.md` patterns
3. **Low Risk**: Writing tests first catches breaking changes early
4. **Confidence**: Green tests = safe to deploy

### TDD Cycle for Each Change

**User Preference**: Minimal implementation then refactor (true TDD)

```
1. RED   → Write failing test for new behavior
           Test should fail (compilation error or assertion failure)

2. GREEN → Implement MINIMAL code to make test pass
           Just enough to turn test green
           Don't worry about error handling, validation, etc. yet
           Keep it simple!

3. REFACTOR → Clean up and enhance implementation
           Add error handling
           Improve validation
           Extract helper functions
           Add logging/audit trail
           Optimize performance

4. REPEAT → Next requirement/test
```

**Example**:
- RED: Write test for `UpdateSubscription` with status field
- GREEN: Add status field handling (minimal if/else)
- REFACTOR: Extract status validation, add better errors, audit log

---

## 📋 Phases Overview

**Execution Strategy**: Sequential (complete each phase before starting next)
**Reference**: User preference - safest approach for coordinated refactor

### Phase 0: Test Infrastructure Setup
**Duration**: 2-3 hours
**MUST DO FIRST** - Creates foundation for all TDD work
**Reference**: `UNIT_TEST_REFACTORING_ANALYSIS.md` (Mock/Fixture patterns)

### Phase 1: SubscriptionService (TDD)
**Duration**: 3-4 hours
**Approach**: Write tests → Update proto → Implement → Refactor
**Reference**: `AUTHENTICATION.md` (SubscriptionService auth strategy)

### Phase 2: AdminService (TDD)
**Duration**: 3-4 hours
**Approach**: Write tests → Update proto → Implement → Refactor
**Reference**: `AUTHENTICATION.md` (AdminService auth strategy)

### Phase 3: MerchantService (TDD)
**Duration**: 2-3 hours
**Approach**: Write tests → Update proto → Implement → Refactor
**Reference**: `AUTHENTICATION.md` (MerchantService auth strategy)

### Phase 4: PaymentMethodService (TDD)
**Duration**: 2-3 hours
**Approach**: Write tests → Update proto → Implement → Refactor
**Reference**: `API_DESIGN_AND_DATAFLOW.md` (Browser Post flow)

### Phase 5: Testing Coordination
**When**: After Phases 0-4 complete (all unit tests passing)
**Integration Tests**: Run on every commit (existing + new)
**E2E Tests**: Run nightly (4 multi-actor auth workflows)
**Reference**: `E2E_TEST_DESIGN.md` (E2E vs Integration classification)

**Integration Test Scope** (run on every commit):
- SubscriptionService: Status transition integration tests
- AdminService: Service activation integration tests
- MerchantService: Merchant activation integration tests
- PaymentMethodService: GetPaymentForm + BrowserPostCallback integration tests

**E2E Test Scope** (run nightly, separate from this refactor):
- Service Onboarding & Authentication (Admin → Service → API call)
- Token Delegation - Customer (Service → Customer token → Transactions)
- Token Delegation - Guest (Service → Guest token → Order lookup)
- Multi-Merchant Authorization (Service access control)

---

## Phase 0: Test Infrastructure Setup (FOUNDATION)

**Duration**: 2-3 hours
**Status**: MUST COMPLETE BEFORE STARTING TDD

### Step 0.1: Create Shared Mock Package

**Purpose**: Eliminate ~300 lines of duplicated mock code

**Create**: `internal/testutil/mocks/`

```bash
mkdir -p internal/testutil/mocks
```

**Files to create**:

1. `internal/testutil/mocks/database.go`
```go
package mocks

import (
    "context"
    "github.com/google/uuid"
    "github.com/kevin07696/payment-service/internal/db/sqlc"
    "github.com/stretchr/testify/mock"
)

// MockQuerier - single source of truth for DB mocks
type MockQuerier struct {
    mock.Mock
}

// Frequently used methods - full implementation
func (m *MockQuerier) GetServiceByServiceID(ctx context.Context, serviceID string) (sqlc.Service, error) {
    args := m.Called(ctx, serviceID)
    return args.Get(0).(sqlc.Service), args.Error(1)
}

func (m *MockQuerier) UpdateServiceActiveStatus(ctx context.Context, params sqlc.UpdateServiceActiveStatusParams) (sqlc.Service, error) {
    args := m.Called(ctx, params)
    return args.Get(0).(sqlc.Service), args.Error(1)
}

func (m *MockQuerier) GetSubscription(ctx context.Context, id uuid.UUID) (sqlc.Subscription, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) UpdateSubscriptionStatus(ctx context.Context, params sqlc.UpdateSubscriptionStatusParams) (sqlc.Subscription, error) {
    args := m.Called(ctx, params)
    return args.Get(0).(sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) GetMerchant(ctx context.Context, merchantID string) (sqlc.Merchant, error) {
    args := m.Called(ctx, merchantID)
    return args.Get(0).(sqlc.Merchant), args.Error(1)
}

func (m *MockQuerier) UpdateMerchantActiveStatus(ctx context.Context, params sqlc.UpdateMerchantActiveStatusParams) (sqlc.Merchant, error) {
    args := m.Called(ctx, params)
    return args.Get(0).(sqlc.Merchant), args.Error(1)
}

// Add more as needed during TDD...
```

2. `internal/testutil/mocks/README.md`
```markdown
# Test Mocks

Shared mock implementations for unit tests.

## Usage

import "github.com/kevin07696/payment-service/internal/testutil/mocks"

mockDB := &mocks.MockQuerier{}
mockDB.On("GetServiceByServiceID", mock.Anything, "acme-web").
    Return(service, nil)
```

**Validation**:
```bash
go build ./internal/testutil/mocks/...
```

### Step 0.2: Create Test Fixtures Package

**Purpose**: Eliminate duplicated test helpers (StringPtr, TransactionBuilder, etc.)

**Create**: `internal/testutil/fixtures/`

```bash
mkdir -p internal/testutil/fixtures
```

**Files to create**:

1. `internal/testutil/fixtures/pointers.go`
```go
package fixtures

// StringPtr returns pointer to string (eliminates duplication across 3+ test files)
func StringPtr(s string) *string {
    return &s
}

func IntPtr(i int32) *int32 {
    return &i
}

func BoolPtr(b bool) *bool {
    return &b
}
```

2. `internal/testutil/fixtures/services.go`
```go
package fixtures

import (
    "github.com/google/uuid"
    "github.com/kevin07696/payment-service/internal/db/sqlc"
    "github.com/jackc/pgx/v5/pgtype"
    "time"
)

// ServiceBuilder provides fluent API for building test services
type ServiceBuilder struct {
    service sqlc.Service
}

func NewService() *ServiceBuilder {
    return &ServiceBuilder{
        service: sqlc.Service{
            ID:                   uuid.New(),
            ServiceID:            "test-service",
            ServiceName:          "Test Service",
            Environment:          "staging",
            RequestsPerSecond:    pgtype.Int4{Int32: 100, Valid: true},
            BurstLimit:           pgtype.Int4{Int32: 200, Valid: true},
            IsActive:             pgtype.Bool{Bool: true, Valid: true},
            CreatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
            UpdatedAt:            pgtype.Timestamp{Time: time.Now(), Valid: true},
        },
    }
}

func (b *ServiceBuilder) WithServiceID(id string) *ServiceBuilder {
    b.service.ServiceID = id
    return b
}

func (b *ServiceBuilder) Inactive() *ServiceBuilder {
    b.service.IsActive = pgtype.Bool{Bool: false, Valid: true}
    return b
}

func (b *ServiceBuilder) Build() sqlc.Service {
    return b.service
}
```

**Validation**:
```bash
go build ./internal/testutil/fixtures/...
```

---

## Phase 1: SubscriptionService (TDD)

**Duration**: 3-4 hours
**Changes**: Remove Pause/Resume RPCs, add status to UpdateSubscription

### Step 1.1: RED - Write Failing Tests

**File**: `internal/handlers/subscription/subscription_handler_test.go`

**New test**: Test UpdateSubscription with status field

```go
func TestUpdateSubscription_WithStatusPaused(t *testing.T) {
    // ARRANGE
    mockDB := &mocks.MockQuerier{}
    handler := NewSubscriptionHandler(mockDB)

    subID := uuid.New()
    existingSub := fixtures.NewSubscription().
        WithID(subID).
        WithStatus("active").
        Build()

    mockDB.On("GetSubscription", mock.Anything, subID).
        Return(existingSub, nil)

    mockDB.On("UpdateSubscriptionStatus", mock.Anything, mock.MatchedBy(func(params sqlc.UpdateSubscriptionStatusParams) bool {
        return params.ID == subID && params.Status == "paused"
    })).Return(sqlc.Subscription{
        ID:     subID,
        Status: "paused",
    }, nil)

    req := &connect.Request[subscriptionv1.UpdateSubscriptionRequest]{
        Msg: &subscriptionv1.UpdateSubscriptionRequest{
            SubscriptionId: subID.String(),
            Status:         fixtures.SubscriptionStatusPtr(subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAUSED),
        },
    }

    // ACT
    resp, err := handler.UpdateSubscription(context.Background(), req)

    // ASSERT
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAUSED, resp.Msg.Status)
    mockDB.AssertExpectations(t)
}

func TestUpdateSubscription_WithStatusActive(t *testing.T) {
    // Test resuming paused subscription
    // Similar structure...
}

func TestUpdateSubscription_WithStatusCancelled(t *testing.T) {
    // Test cancelling subscription
    // Should set cancelled_at timestamp
}

func TestUpdateSubscription_StatusTransitionValidation(t *testing.T) {
    // Test invalid transitions (e.g., cancelled → active)
}
```

**Run tests** (should FAIL):
```bash
go test ./internal/handlers/subscription/... -v
# Expected: Compilation errors (status field doesn't exist yet)
```

### Step 1.2: Update Proto (Make Tests Compile)

**File**: `proto/subscription/v1/subscription.proto`

**Changes**:

1. Remove RPCs (lines 43-49):
```proto
// DELETE THESE:
  // PauseSubscription pauses an active subscription
  rpc PauseSubscription(PauseSubscriptionRequest) returns (SubscriptionResponse) {
  }

  // ResumeSubscription resumes a paused subscription
  rpc ResumeSubscription(ResumeSubscriptionRequest) returns (SubscriptionResponse) {
  }
```

2. Remove messages (lines 100-108):
```proto
// DELETE THESE:
// PauseSubscriptionRequest pauses a subscription
message PauseSubscriptionRequest {
  string subscription_id = 1;
}

// ResumeSubscriptionRequest resumes a subscription
message ResumeSubscriptionRequest {
  string subscription_id = 1;
}
```

3. Update UpdateSubscriptionRequest (line 89):
```proto
// UpdateSubscriptionRequest updates subscription properties
message UpdateSubscriptionRequest {
  string subscription_id = 1;
  optional string amount = 2; // Optional: update amount
  optional int32 interval_value = 3; // Optional: update interval value
  optional IntervalUnit interval_unit = 4; // Optional: update interval unit
  optional string payment_method_id = 5; // Optional: update payment method
  optional SubscriptionStatus status = 6; // NEW: Update status (pause/resume/cancel)
  string idempotency_key = 7;
}
```

**Regenerate**:
```bash
make proto
```

**Verify**:
```bash
go build ./proto/subscription/v1/...
```

### Step 1.3: GREEN - Implement Handler (Pass Tests)

**File**: `internal/handlers/subscription/subscription_handler.go`

**Remove methods**:
```go
// DELETE: func (h *SubscriptionHandler) PauseSubscription(...)
// DELETE: func (h *SubscriptionHandler) ResumeSubscription(...)
```

**Update method**:
```go
func (h *SubscriptionHandler) UpdateSubscription(
    ctx context.Context,
    req *connect.Request[subscriptionv1.UpdateSubscriptionRequest],
) (*connect.Response[subscriptionv1.SubscriptionResponse], error) {
    // Existing validation...

    // NEW: Handle status updates
    if req.Msg.Status != nil {
        switch *req.Msg.Status {
        case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_PAUSED:
            // Pause subscription - preserve next_billing_date
            _, err = h.queries.UpdateSubscriptionStatus(ctx, sqlc.UpdateSubscriptionStatusParams{
                ID:     uuid.MustParse(req.Msg.SubscriptionId),
                Status: "paused",
            })
            if err != nil {
                return nil, connect.NewError(connect.CodeInternal, err)
            }

        case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_ACTIVE:
            // Resume subscription
            _, err = h.queries.UpdateSubscriptionStatus(ctx, sqlc.UpdateSubscriptionStatusParams{
                ID:     uuid.MustParse(req.Msg.SubscriptionId),
                Status: "active",
            })
            if err != nil {
                return nil, connect.NewError(connect.CodeInternal, err)
            }

        case subscriptionv1.SubscriptionStatus_SUBSCRIPTION_STATUS_CANCELLED:
            // Cancel subscription - set cancelled_at
            _, err = h.queries.CancelSubscription(ctx, uuid.MustParse(req.Msg.SubscriptionId))
            if err != nil {
                return nil, connect.NewError(connect.CodeInternal, err)
            }
        }
    }

    // Existing amount/payment method update logic...
}
```

**Run tests** (should PASS):
```bash
go test ./internal/handlers/subscription/... -v
```

### Step 1.4: REFACTOR - Clean Up

1. **Remove dead code**: Delete PauseSubscription, ResumeSubscription methods
2. **Extract status validation**: Consider separate function for valid transitions
3. **Add error handling**: Better error messages for invalid transitions

**Run tests again**:
```bash
go test ./internal/handlers/subscription/... -v
```

### Step 1.5: Add SQLC Query (If Missing)

**Check if query exists**: `internal/db/queries/subscriptions.sql`

**If missing, add**:
```sql
-- name: UpdateSubscriptionStatus :one
UPDATE subscriptions
SET status = $2,
    cancelled_at = CASE
        WHEN $2 = 'cancelled' THEN NOW()
        ELSE cancelled_at
    END,
    updated_at = NOW()
WHERE id = $1
RETURNING *;
```

**Regenerate**:
```bash
make sqlc
```

---

## Phase 2: AdminService (TDD)

**Duration**: 3-4 hours
**Changes**: Remove Deactivate/Activate RPCs, add UpdateService RPC

### Step 2.1: RED - Write Failing Tests

**File**: `internal/handlers/admin/service_handler_test.go`

```go
func TestUpdateService_ActivateService(t *testing.T) {
    // ARRANGE
    mockDB := &mocks.MockQuerier{}
    handler := NewServiceHandler(mockDB)

    serviceID := "acme-web"
    inactiveService := fixtures.NewService().
        WithServiceID(serviceID).
        Inactive().
        Build()

    mockDB.On("GetServiceByServiceID", mock.Anything, serviceID).
        Return(inactiveService, nil)

    mockDB.On("UpdateServiceActiveStatus", mock.Anything, mock.MatchedBy(func(params sqlc.UpdateServiceActiveStatusParams) bool {
        return params.ServiceID == serviceID && params.IsActive.Bool == true
    })).Return(fixtures.NewService().WithServiceID(serviceID).Build(), nil)

    req := &connect.Request[adminv1.UpdateServiceRequest]{
        Msg: &adminv1.UpdateServiceRequest{
            ServiceId: serviceID,
            IsActive:  fixtures.BoolPtr(true),
            Reason:    fixtures.StringPtr("Reactivating after incident resolution"),
        },
    }

    // ACT
    resp, err := handler.UpdateService(context.Background(), req)

    // ASSERT
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.True(t, resp.Msg.Service.IsActive)
    mockDB.AssertExpectations(t)
}

func TestUpdateService_DeactivateService(t *testing.T) {
    // Test deactivation with reason
}

func TestUpdateService_UpdateRateLimits(t *testing.T) {
    // Test updating requests_per_second and burst_limit
}

func TestUpdateService_CombinedUpdates(t *testing.T) {
    // Test updating is_active + rate limits together
}
```

**Run tests** (should FAIL):
```bash
go test ./internal/handlers/admin/... -v
# Expected: UpdateService method doesn't exist
```

### Step 2.2: Update Proto

**File**: `proto/admin/v1/admin.proto`

**Remove RPCs and messages** (lines 25-146):
```proto
// DELETE:
  rpc DeactivateService(DeactivateServiceRequest) returns (DeactivateServiceResponse);
  rpc ActivateService(ActivateServiceRequest) returns (ActivateServiceResponse);

// DELETE messages:
message DeactivateServiceRequest { ... }
message DeactivateServiceResponse { ... }
message ActivateServiceRequest { ... }
message ActivateServiceResponse { ... }
```

**Add new RPC** (after ListServices):
```proto
  // UpdateService updates service configuration including activation status
  rpc UpdateService(UpdateServiceRequest) returns (UpdateServiceResponse);
```

**Add new messages**:
```proto
// UpdateServiceRequest updates service configuration
message UpdateServiceRequest {
  // Service ID to update
  string service_id = 1;

  // Optional: Update activation status
  optional bool is_active = 2;

  // Optional: Update rate limit (requests per second)
  optional int32 requests_per_second = 3;

  // Optional: Update burst limit
  optional int32 burst_limit = 4;

  // Reason for change (audit trail)
  optional string reason = 5;
}

// UpdateServiceResponse returns updated service
message UpdateServiceResponse {
  // The updated service
  Service service = 1;

  // Confirmation message
  string message = 2;
}
```

**Regenerate**:
```bash
make proto
```

### Step 2.3: GREEN - Implement Handler

**File**: `internal/handlers/admin/service_handler.go`

**Remove methods**:
```go
// DELETE: func (h *ServiceHandler) DeactivateService(...)
// DELETE: func (h *ServiceHandler) ActivateService(...)
```

**Add method**:
```go
func (h *ServiceHandler) UpdateService(
    ctx context.Context,
    req *connect.Request[adminv1.UpdateServiceRequest],
) (*connect.Response[adminv1.UpdateServiceResponse], error) {
    // Validate
    if req.Msg.ServiceId == "" {
        return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("service_id is required"))
    }

    // Get service
    service, err := h.queries.GetServiceByServiceID(ctx, req.Msg.ServiceId)
    if err != nil {
        return nil, connect.NewError(connect.CodeNotFound, err)
    }

    // Update fields
    if req.Msg.IsActive != nil {
        service, err = h.queries.UpdateServiceActiveStatus(ctx, sqlc.UpdateServiceActiveStatusParams{
            ID:       service.ID,
            IsActive: pgtype.Bool{Bool: *req.Msg.IsActive, Valid: true},
        })
        if err != nil {
            return nil, connect.NewError(connect.CodeInternal, err)
        }
    }

    if req.Msg.RequestsPerSecond != nil || req.Msg.BurstLimit != nil {
        // Update rate limits...
    }

    // TODO: Audit log with reason

    message := "Service updated successfully"
    if req.Msg.IsActive != nil {
        if *req.Msg.IsActive {
            message = "Service activated successfully"
        } else {
            message = "Service deactivated successfully"
        }
    }

    return connect.NewResponse(&adminv1.UpdateServiceResponse{
        Service: convertToProtoService(service),
        Message: message,
    }), nil
}
```

**Run tests**:
```bash
go test ./internal/handlers/admin/... -v
```

### Step 2.4: REFACTOR

1. Extract audit logging
2. Validate reason field when deactivating
3. Better error messages

---

## Phase 3: MerchantService (TDD)

**Duration**: 2-3 hours
**Changes**: Remove DeactivateMerchant RPC, add is_active to UpdateMerchant

### Step 3.1: RED - Write Failing Tests

**File**: `internal/handlers/merchant/merchant_handler_test.go`

```go
func TestUpdateMerchant_WithActivation(t *testing.T) {
    // Test is_active = true
}

func TestUpdateMerchant_WithDeactivation(t *testing.T) {
    // Test is_active = false with reason
}

func TestUpdateMerchant_RequiresReasonForDeactivation(t *testing.T) {
    // Validate reason is required when deactivating
}
```

### Step 3.2: Update Proto

**File**: `proto/merchant/v1/merchant.proto`

**Remove**:
```proto
// DELETE:
  rpc DeactivateMerchant(DeactivateMerchantRequest) returns (MerchantResponse);

// DELETE:
message DeactivateMerchantRequest { ... }
```

**Update**:
```proto
message UpdateMerchantRequest {
  string merchant_id = 1;
  optional string mac_secret = 2;
  optional string cust_nbr = 3;
  optional string merch_nbr = 4;
  optional string dba_nbr = 5;
  optional string terminal_nbr = 6;
  optional Environment environment = 7;
  optional bool is_active = 8;  // NEW
  optional string reason = 9;    // NEW - for audit trail
  map<string, string> metadata = 10;
  string idempotency_key = 11;
}
```

### Step 3.3-3.4: GREEN + REFACTOR

Similar process to AdminService...

---

## Phase 4: PaymentMethodService (TDD)

**Duration**: 2-3 hours
**Changes**: Remove SavePaymentMethod & ConvertFinancialBRIC RPCs

### Step 4.1: RED - Write Tests for GetPaymentForm

**File**: `internal/handlers/payment/get_payment_form_handler_test.go` (NEW)

```go
func TestGetPaymentForm_Success(t *testing.T) {
    // Test REST handler returns JSON configuration
}

func TestGetPaymentForm_RequiresServiceToken(t *testing.T) {
    // Test authentication
}
```

### Step 4.2: Update Proto

**File**: `proto/payment_method/v1/payment_method.proto`

**Remove**:
```proto
// DELETE:
  rpc SavePaymentMethod(SavePaymentMethodRequest) returns (PaymentMethodResponse);
  rpc ConvertFinancialBRICToStorageBRIC(ConvertFinancialBRICRequest) returns (PaymentMethodResponse);

// DELETE messages:
message SavePaymentMethodRequest { ... }
message ConvertFinancialBRICRequest { ... }
```

### Step 4.3: Implement GetPaymentForm Handler

**File**: `internal/handlers/payment/get_payment_form_handler.go` (NEW)

```go
package payment

import (
    "context"
    "encoding/json"
    "net/http"
)

type GetPaymentFormHandler struct {
    queries *sqlc.Queries
}

func NewGetPaymentFormHandler(queries *sqlc.Queries) *GetPaymentFormHandler {
    return &GetPaymentFormHandler{queries: queries}
}

// Handle returns JSON payment form configuration
func (h *GetPaymentFormHandler) Handle(w http.ResponseWriter, r *http.Request) {
    // TODO: Verify Service Token authentication

    // Parse query params
    merchantID := r.URL.Query().Get("merchant_id")
    amount := r.URL.Query().Get("amount")
    txType := r.URL.Query().Get("transaction_type")

    // Generate form token (session-based)
    formToken := generateFormToken()

    // Build response
    response := map[string]interface{}{
        "form_token":        formToken,
        "epx_url":           "https://epx.gateway.com/browserpost",
        "merchant_id":       merchantID,
        "transaction_type":  txType,
        "amount":            amount,
        "callback_url":      "https://yourserver.com/api/payment/callback",
        "return_url":        "https://yoursite.com/payment/success",
        "session_expires_at": time.Now().Add(15 * time.Minute),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

### Step 4.4: Remove Handler Methods

**File**: `internal/handlers/payment_method/payment_method_handler.go`

```go
// DELETE:
// func (h *PaymentMethodHandler) SavePaymentMethod(...)
// func (h *PaymentMethodHandler) ConvertFinancialBRICToStorageBRIC(...)
```

---

## Phase 5: Integration Testing Coordination

### 🚦 Readiness Checkpoints

After completing Phases 0-4, verify these before signaling for parallel integration testing:

#### ✅ Infrastructure Requirements

```bash
# 1. Test database running
docker-compose -f docker-compose.test.yml up -d
psql -h localhost -p 5434 -U postgres -c "SELECT 1"

# 2. Migrations applied
make migrate-up

# 3. Service builds
make build

# 4. Unit tests pass
go test ./... -short
```

#### ✅ Proto RPCs Finalized

```bash
# Verify proto compilation
make proto
go build ./proto/...

# Verify no signature changes expected
git diff proto/
```

#### ✅ Stub Handlers Committed

**Purpose**: Tests can compile even if handlers not fully implemented

**Verify stub handlers return "not implemented"**:
```bash
grep -r "not implemented" internal/handlers/
```

**Example stub**:
```go
func (h *SubscriptionHandler) UpdateSubscription(...) {
    // Stub: Return not implemented until TDD complete
    return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("not implemented"))
}
```

#### ✅ Configuration Agreement

**Document**: `docs/INTEGRATION_TEST_CONFIG.md`

**Must agree on**:
- EPX environment (staging UAT)
- Test merchant credentials (CUST_NBR, MERCH_NBR, etc.)
- Error handling strategy
- Test data cleanup approach

---

## 🎯 Coordination Protocol

### When to Signal "Ready for Integration Tests"

**After Phase 4 Complete + All Checkpoints ✅**:

1. Post in team channel:
   ```
   🟢 READY FOR INTEGRATION TESTS

   Phases 0-4 complete:
   ✅ Unit tests passing
   ✅ Proto RPCs finalized
   ✅ Stub handlers committed
   ✅ Test DB running
   ✅ Service builds

   Integration test team can proceed with parallel implementation.
   ```

2. Provide test merchant config:
   ```
   Test Merchant: test-merchant-staging
   CUST_NBR: [redacted]
   MERCH_NBR: [redacted]
   DBA_NBR: [redacted]
   TERMINAL_NBR: [redacted]
   EPX Environment: https://epx-staging.example.com
   ```

### What Integration Tests Can Run in Parallel

Once signaled, integration test team can implement:

1. **SubscriptionService Integration Tests**
   - Test UpdateSubscription with status transitions
   - Verify pause/resume behavior
   - Test billing cycle handling

2. **AdminService Integration Tests**
   - Test UpdateService activation/deactivation
   - Verify rate limit updates
   - Test audit logging

3. **MerchantService Integration Tests**
   - Test UpdateMerchant with is_active
   - Verify merchant deactivation prevents transactions

4. **PaymentMethodService Integration Tests**
   - Test GetPaymentForm endpoint
   - Test BrowserPostCallback with real EPX staging
   - Verify Storage BRIC creation

### Sync Points

**Daily standup** to coordinate:
- Which integration tests are passing
- Any blockers discovered
- Handler implementation progress

---

## 📊 Progress Tracking

### Phase Completion Checklist

- [ ] **Phase 0: Test Infrastructure** (2-3 hours)
  - [ ] Shared mocks package created
  - [ ] Test fixtures package created
  - [ ] All builds successful

- [ ] **Phase 1: SubscriptionService** (3-4 hours)
  - [ ] RED: Tests written
  - [ ] GREEN: Proto updated
  - [ ] GREEN: Handler implemented
  - [ ] REFACTOR: Code cleaned
  - [ ] All unit tests passing

- [ ] **Phase 2: AdminService** (3-4 hours)
  - [ ] RED: Tests written
  - [ ] GREEN: Proto updated
  - [ ] GREEN: Handler implemented
  - [ ] REFACTOR: Code cleaned
  - [ ] All unit tests passing

- [ ] **Phase 3: MerchantService** (2-3 hours)
  - [ ] RED: Tests written
  - [ ] GREEN: Proto updated
  - [ ] GREEN: Handler implemented
  - [ ] REFACTOR: Code cleaned
  - [ ] All unit tests passing

- [ ] **Phase 4: PaymentMethodService** (2-3 hours)
  - [ ] RED: Tests written
  - [ ] GREEN: Proto updated
  - [ ] GREEN: GetPaymentForm handler created
  - [ ] GREEN: Removed SavePaymentMethod/ConvertFinancialBRIC
  - [ ] REFACTOR: Code cleaned
  - [ ] All unit tests passing

- [ ] **Phase 5: Integration Readiness** (1 hour)
  - [ ] All checkpoints verified
  - [ ] Configuration documented
  - [ ] Team signaled
  - [ ] Parallel integration testing begins

---

## 🎓 TDD Best Practices for This Refactor

### 1. Write Minimal Tests First

**Good**:
```go
func TestUpdateSubscription_PausesSubscription(t *testing.T) {
    // Single assertion: status = paused
}
```

**Avoid**:
```go
func TestUpdateSubscription_AllScenarios(t *testing.T) {
    // Testing 10 different scenarios in one test
}
```

### 2. One Test Per Behavior

- `TestUpdateSubscription_WithStatusPaused`
- `TestUpdateSubscription_WithStatusActive`
- `TestUpdateSubscription_WithStatusCancelled`

### 3. Use Table-Driven Tests for Multiple Cases

```go
func TestUpdateSubscription_StatusTransitions(t *testing.T) {
    tests := []struct {
        name       string
        fromStatus string
        toStatus   string
        wantErr    bool
    }{
        {"active to paused", "active", "paused", false},
        {"paused to active", "paused", "active", false},
        {"cancelled to active", "cancelled", "active", true}, // Invalid
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test transition...
        })
    }
}
```

### 4. Mock Only What You Need

**Good**:
```go
mockDB.On("GetSubscription", mock.Anything, subID).Return(sub, nil)
mockDB.On("UpdateSubscriptionStatus", mock.Anything, mock.Anything).Return(updatedSub, nil)
```

**Avoid**:
```go
// Setting up 20 mock methods when only 2 are used
```

### 5. Test One Thing at a Time

Each test should verify ONE behavior:
- ✅ Status updates correctly
- ✅ Error handling works
- ✅ Validation rejects invalid input

Don't test everything in one massive test.

---

## 📝 Summary

### Total Estimated Time
- Phase 0: 2-3 hours (test infrastructure)
- Phases 1-4: 10-14 hours (TDD implementation)
- **Total**: 12-17 hours for complete refactor

### Benefits of TDD Approach
1. **Confidence**: Tests written first catch regressions
2. **Documentation**: Tests serve as examples
3. **Design**: Writing tests first improves API design
4. **Safety**: Can refactor with green tests as safety net
5. **Parallel Work**: Integration tests can start early

### Risk Mitigation
- Unit tests catch breaking changes early
- Stubs allow integration tests to compile
- Shared mocks reduce maintenance burden
- Fixtures make tests readable and maintainable

---

## 🚀 Next Steps

1. **Review this plan** with team
2. **Start Phase 0** (test infrastructure)
3. **Proceed through Phases 1-4** using TDD
4. **Signal when ready** for parallel integration tests
5. **Coordinate daily** with integration test team

Ready to proceed with Phase 0: Test Infrastructure Setup?
