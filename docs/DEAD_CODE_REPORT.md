# Dead Code Analysis Report

**Generated:** 2025-11-26
**Updated:** 2025-11-26 (Dependency Analysis Complete)
**Tool:** golang.org/x/tools/cmd/deadcode
**Total Unreachable Functions:** 521 (219 production, 302 test utilities)

## Summary

The deadcode analysis found significant amounts of unused code. This report categorizes the findings by priority and provides **detailed dependency analysis** showing how the codebase would work with dead code removed.

---

## HIGH PRIORITY - Entire Files/Modules Unused

### 1. Legacy gRPC Handlers (Shared Helper Functions Issue)
These files contain old gRPC handlers that are NOT registered in `main.go`. The server uses ConnectRPC handlers (`*_handler_connect.go`) exclusively.

**DEPENDENCY ANALYSIS - What Connect handlers use from gRPC handler files:**

#### Payment Handler (`payment_handler.go`)
Connect handler (`payment_handler_connect.go`) uses these shared functions:
| Function | Used In Connect Handler | Lines |
|----------|------------------------|-------|
| `validateAuthorizeRequest()` | Yes - Line 43 | 280-293 |
| `validateSaleRequest()` | Yes - Line 126 | 296-309 |
| `convertMetadata()` | Yes - Lines 52, 134 | 314-322 |
| `transactionToPaymentResponse()` | Yes - Lines 80, 121, 172, 203, 239 | 325-338 |
| `transactionToProto()` | Yes - Lines 258, 298 | 399-422 |
| `mapDomainStatusToProto()` | Yes - via converters | 425-433 |
| `mapProtoStatusToDomain()` | Yes - Line 287 | 436-444 |
| `transactionTypeToProto()` | Yes - via converters | 447-461 |
| `paymentMethodTypeToProto()` | Yes - via converters | 464-472 |
| `stringPtrToString()` | Yes - via converters | 475-479 |
| `extractCardInfo()` | Yes - via converters | 342-358 |
| `epxCardTypeToBrand()` | Yes - via converters | 361-374 |
| `extractLastFour()` | Yes - via converters | 377-396 |
| `handleServiceError()` | NO - Connect uses `handleServiceErrorConnect()` | 484-514 |

**gRPC Handler types/methods (safe to remove):**
- `type Handler struct` (lines 19-24)
- `NewHandler()` (lines 27-31)
- `(h *Handler) Authorize()` (lines 35-79)
- `(h *Handler) Capture()` (lines 83-115)
- `(h *Handler) Sale()` (lines 119-160)
- `(h *Handler) Void()` (lines 164-186)
- `(h *Handler) Refund()` (lines 190-217)
- `(h *Handler) GetTransaction()` (lines 221-234)
- `(h *Handler) ListTransactions()` (lines 238-275)

#### Payment Method Handler (`payment_method_handler.go`)
| Function | Used In Connect Handler | Lines |
|----------|------------------------|-------|
| `paymentMethodToResponse()` | Yes - Lines 134, 199, 343 | 218-252 |
| `paymentMethodToProto()` | Yes - Lines 49, 96 | 255-290 |
| `paymentMethodTypeToProto()` | Yes - via converters | 293-301 |
| `paymentMethodTypeFromProto()` | Yes - Lines 73, 94 | 304-312 |
| `handleServiceError()` | NO - Connect uses `handleServiceErrorConnect()` | 317-341 |

**gRPC Handler types/methods (safe to remove):**
- `type Handler struct` (lines 19-23)
- `NewHandler()` (lines 26-30)
- All `(h *Handler) *` methods (lines 34-212)

#### Subscription Handler (`subscription_handler.go`)
| Function | Used In Connect Handler | Lines |
|----------|------------------------|-------|
| `validateCreateSubscriptionRequest()` | Yes - Line 44 | 275-297 |
| `convertMetadata()` | Yes - Line 59 | 302-310 |
| `subscriptionToResponse()` | Yes - Lines 77, 126, 160, 183, 206 | 313-337 |
| `subscriptionToProto()` | Yes - Lines 228, 264 | 340-367 |
| `intervalUnitToProto()` | Yes - via converters | 370-382 |
| `intervalUnitFromProto()` | Yes - Lines 55, 109 | 385-397 |
| `subscriptionStatusToProto()` | Yes - via converters | 400-412 |
| `subscriptionStatusFromProto()` | Yes - Line 252 | 415-427 |
| `convertMetadataToProto()` | Yes - via converters | 430-438 |
| `isRetriableError()` | Yes - Line 298 | 441-456 |
| `handleServiceError()` | NO - Connect uses `handleServiceErrorConnect()` | 461-495 |

#### Merchant Handler (`merchant_handler.go`)
| Function | Used In Connect Handler | Lines |
|----------|------------------------|-------|
| `validateRegisterMerchantRequest()` | Yes - Line 45 | 237-259 |
| `merchantToResponse()` | Yes - Lines 72, 187, 220 | 264-276 |
| `merchantToProto()` | Yes - Line 94 | 279-293 |
| `merchantToSummary()` | Yes - Line 129 | 296-303 |
| `environmentToProto()` | Yes - via converters | 306-314 |
| `environmentFromProto()` | Yes - Lines 57, 101, 113, 157, 175 | 317-325 |
| `handleServiceError()` | NO - Connect uses `handleServiceErrorConnect()` | 330-350 |

**REFACTORING REQUIRED:** Extract shared helper functions to `*_converters.go` files before removing gRPC handlers.

### 2. Admin Service Handler (Entire Module) - SAFE TO DELETE
`internal/handlers/admin/service_handler.go` - 10 unreachable functions

This is a ConnectRPC handler for admin operations, but it's **never registered** in the server. Admin operations are done via the CLI (`cmd/admin/`) instead.

**DEPENDENCY CHECK: NO dependencies from other files.** This module can be safely deleted.

| Function | Lines |
|----------|-------|
| `NewServiceHandler` | 26 |
| `CreateService` | 33 |
| `RotateServiceKey` | 112 |
| `GetService` | 168 |
| `ListServices` | 198 |
| `DeactivateService` | 265 |
| `ActivateService` | 313 |
| `auditServiceCreation` | 355 |
| `auditKeyRotation` | 409 |
| `auditServiceDeactivation` | 478 |

**Recommendation:** Remove entire `internal/handlers/admin/` directory and its test file.

---

## MEDIUM PRIORITY - Unused Auth/Security Code

### 3. Auth Context Functions
`internal/auth/context.go` - PARTIAL dead code

**DEPENDENCY ANALYSIS:**
| Function | Status | Used By |
|----------|--------|---------|
| `GetAuthInfo` | **USED** | `internal/middleware/auth_context.go` |
| `GetClientIP` | **USED** | `internal/middleware/connect_auth.go:478`, `internal/handlers/admin/service_handler.go:365,420,489` |
| `GetUserAgent` | **USED** | `internal/handlers/admin/service_handler.go:371,426,495` |
| `AuthTypeKey`, `ServiceIDKey`, `MerchantIDKey`, etc. | **USED** | `internal/middleware/connect_auth.go` (context value keys) |
| `IsAuthenticated` | Unused | - |
| `RequireMerchant` | Unused | - |
| `RequireService` | Unused | - |
| `RequireScope` | Unused | - |
| `RequireAnyScope` | Unused | - |
| `WithAuth` | Unused | - |
| `WithInternalAuth` | Unused | - |
| `IsInternalAuth` | Unused | - |
| `GetMerchantID` | Unused | - |
| `GetServiceID` | Unused | - |
| `GetRequestID` | Unused | - |

**Recommendation:** Keep file but remove unused helper functions. Core context keys and `GetAuthInfo`, `GetClientIP`, `GetUserAgent` are REQUIRED.

### 4. JWT Utils - SAFE TO DELETE
`internal/auth/jwt_utils.go` - 7 unreachable functions

**DEPENDENCY ANALYSIS:** No production code uses this file.

| Function | Status |
|----------|--------|
| `NewJWTManager` | Unused - middleware uses `jwt.Parse()` directly |
| `JWTManager.GenerateToken` | Unused - `cmd/jwtgen/` has its own implementation |
| `JWTManager.GenerateRefreshToken` | Unused |
| `JWTManager.ValidateToken` | Unused - middleware validates differently |
| `JWTManager.GetPublicKeyPEM` | Unused |
| `ParsePublicKeyFromPEM` | Unused |
| `ValidateScopes` | Unused |
| `GenerateRSAKeyPair` | Unused - `pkg/crypto` has its own implementation |
| `PrivateKeyToPEM` | Unused |
| `PublicKeyToPEM` | Unused |

**Note:** JWT generation is done via `cmd/jwtgen/` (uses `pkg/crypto`) and validation via `internal/middleware/connect_auth.go` (uses `jwt.Parse()` directly). This file is a duplicate implementation.

**Recommendation:** DELETE entire file `internal/auth/jwt_utils.go`

### 5. Public Key Store - SAFE TO DELETE
`internal/auth/public_key_store.go` - 8 unreachable functions

**DEPENDENCY ANALYSIS:** No production code uses this file. The `AuthInterceptor` in `internal/middleware/connect_auth.go` stores public keys in its own `publicKeys map[string]*rsa.PublicKey` and loads from database via `ListActiveServicePublicKeys`.

| Function | Status |
|----------|--------|
| `NewPublicKeyStore` | Unused |
| `LoadKeysFromDirectory` | Unused |
| `LoadKey` | Unused |
| `AddKey` | Unused |
| `GetPublicKey` | Unused |
| `HasIssuer` | Unused |
| `ListIssuers` | Unused |
| `parseRSAPublicKey` | Unused |

**Recommendation:** DELETE entire file `internal/auth/public_key_store.go`

### 6. Merchant Validator Middleware - SAFE TO DELETE
`internal/middleware/merchant_validator.go` - 8 unreachable functions

**DEPENDENCY ANALYSIS:** No production code uses this file. Merchant validation is handled inside `AuthInterceptor.verifyServiceMerchantAccess()`.

**Recommendation:** DELETE entire file `internal/middleware/merchant_validator.go`

---

## MEDIUM PRIORITY - Unused Domain/Service Code

### 7. Chargeback Domain Methods
`internal/domain/chargeback.go` - 8 unreachable methods

| Method | Purpose |
|--------|---------|
| `IsOpen` | Check if chargeback open |
| `IsResolved` | Check if resolved |
| `CanRespond` | Check response eligibility |
| `IsOverdue` | Check if overdue |
| `DaysUntilDeadline` | Calculate deadline |
| `MarkResponded` | Update status |
| `MarkResolved` | Update status |
| `GetCustomerID` | Get customer ID |

**Recommendation:** These are business logic helpers that may be intended for future use. Review before removing.

### 8. Domain Error Helpers
`internal/domain/errors.go` - 7 unreachable functions

| Function | Purpose |
|----------|---------|
| `WrapError` | Wrap errors |
| `IsDomainError` | Type check |
| `GetErrorCode` | Extract code |
| `IsNotFoundError` | Type check |
| `IsAuthError` | Type check |
| `IsValidationError` | Type check |
| `IsGatewayError` | Type check |

### 9. Config Loading
`internal/config/config.go` - 5 unreachable functions

| Function | Purpose |
|----------|---------|
| `LoadFromEnv` | Load config from env |
| `DatabaseConfig.ConnectionString` | Build DB URL |
| `getEnv` | Get env var |
| `getEnvAsInt` | Get env as int |
| `getEnvAsBool` | Get env as bool |

**Note:** Server uses different config loading in `main.go`.

---

## LOW PRIORITY - Utility/Pool Code

### 10. Object Pools (Performance Optimization)
These pools were created for performance but may not be integrated:

| File | Functions |
|------|-----------|
| `internal/adapters/epx/pool.go` | 4 funcs |
| `internal/services/payment/pool.go` | 6 funcs |
| `pkg/pool/epx_pool.go` | 6 funcs |
| `pkg/encoding/pool.go` | 4 funcs |

### 11. Observability/Metrics
`pkg/observability/business_metrics.go` - 8 unreachable functions

Business metrics recording functions that may not be wired up:
- `RecordPaymentTransaction`
- `RecordACHVerification`
- `RecordSubscriptionBilling`
- `RecordWebhookDelivery`

### 12. Log Scrubbing
`internal/middleware/log_scrubbing.go` - 5 unreachable functions

PII/PCI scrubbing utilities that may not be integrated:
- `ScrubString`
- `ScrubField`
- `ScrubZapField`
- `ScrubLogMessage`
- `NewScrubLogger`

---

## TEST UTILITIES (302 items - May Be Intentional)

Test mocks and fixtures are reported as dead code because they're only used when tests run:

| Location | Count | Notes |
|----------|-------|-------|
| `internal/testutil/mocks/database.go` | 130 | Mock implementations |
| `internal/testutil/fixtures/transactions.go` | 38 | Test data |
| `internal/testutil/fixtures/payment_methods.go` | 24 | Test data |
| `internal/testutil/fixtures/subscriptions.go` | 20 | Test data |
| `internal/testutil/fixtures/merchants.go` | 17 | Test data |
| `internal/testutil/fixtures/services.go` | 13 | Test data |
| `tests/integration/testutil/` | ~30 | Integration test helpers |

**Recommendation:** Keep these - they support testing.

---

## Implementation Plan - How The Codebase Would Work With Dead Code Removed

### Phase 1: Safe Deletions (No Dependencies)
These files/modules can be deleted immediately without any refactoring:

| File/Directory | Reason Safe |
|----------------|-------------|
| `internal/handlers/admin/` | Never registered, admin done via CLI |
| `internal/auth/jwt_utils.go` | Duplicate of `pkg/crypto` + middleware implementation |
| `internal/auth/public_key_store.go` | Middleware stores keys directly in `map[string]*rsa.PublicKey` |
| `internal/middleware/merchant_validator.go` | Validation done in `AuthInterceptor.verifyServiceMerchantAccess()` |

**Estimated Lines Removed:** ~800 lines

### Phase 2: Refactoring Required (Has Dependencies)
For each gRPC handler file, we need to extract shared helpers before removing handler types:

#### Step 2.1: Create Converter Files
Create new `*_converters.go` files with ONLY the helper functions:

```
internal/handlers/payment/payment_converters.go
internal/handlers/payment_method/payment_method_converters.go
internal/handlers/subscription/subscription_converters.go
internal/handlers/merchant/merchant_converters.go
```

#### Step 2.2: Move Helper Functions
For each handler package, move these functions to converters:

**payment_converters.go:**
- `validateAuthorizeRequest()`, `validateSaleRequest()`
- `convertMetadata()`, `transactionToPaymentResponse()`, `transactionToProto()`
- `mapDomainStatusToProto()`, `mapProtoStatusToDomain()`
- `transactionTypeToProto()`, `paymentMethodTypeToProto()`, `stringPtrToString()`
- `extractCardInfo()`, `epxCardTypeToBrand()`, `extractLastFour()`

**payment_method_converters.go:**
- `paymentMethodToResponse()`, `paymentMethodToProto()`
- `paymentMethodTypeToProto()`, `paymentMethodTypeFromProto()`

**subscription_converters.go:**
- `validateCreateSubscriptionRequest()`, `convertMetadata()`
- `subscriptionToResponse()`, `subscriptionToProto()`
- `intervalUnitToProto()`, `intervalUnitFromProto()`
- `subscriptionStatusToProto()`, `subscriptionStatusFromProto()`
- `convertMetadataToProto()`, `isRetriableError()`

**merchant_converters.go:**
- `validateRegisterMerchantRequest()`
- `merchantToResponse()`, `merchantToProto()`, `merchantToSummary()`
- `environmentToProto()`, `environmentFromProto()`

#### Step 2.3: Remove gRPC Handler Files
After extracting helpers, remove the original files:
- `internal/handlers/payment/payment_handler.go`
- `internal/handlers/payment_method/payment_method_handler.go`
- `internal/handlers/subscription/subscription_handler.go`
- `internal/handlers/merchant/merchant_handler.go`

**Estimated Lines Removed:** ~1,200 lines
**Net Lines After Converters:** ~600 lines removed

### Phase 3: Clean Up Auth Context
Remove unused helper functions from `internal/auth/context.go`:
- `IsAuthenticated()`, `RequireMerchant()`, `RequireService()`
- `RequireScope()`, `RequireAnyScope()`
- `WithAuth()`, `WithInternalAuth()`, `IsInternalAuth()`
- `GetMerchantID()`, `GetServiceID()`, `GetRequestID()`

**Keep:**
- All context keys (`AuthTypeKey`, `ServiceIDKey`, etc.)
- `GetAuthInfo()`, `GetClientIP()`, `GetUserAgent()`
- `AuthType` constants and `AuthInfo` struct

**Estimated Lines Removed:** ~100 lines

### Phase 4: Review Before Removing
These items should be reviewed for business value before deletion:

| Item | Review Notes |
|------|--------------|
| `internal/domain/chargeback.go` methods | May be needed for chargeback feature |
| `internal/domain/errors.go` helpers | May simplify error handling |
| `internal/config/config.go` | Alternative config loading approach |
| Object pools (`pkg/pool/`, etc.) | Performance optimization - worth integrating? |
| Business metrics (`pkg/observability/`) | Should be wired up, not removed |
| Log scrubbing (`internal/middleware/log_scrubbing.go`) | PCI compliance - should be integrated |

---

## Summary

| Phase | Files Affected | Lines Removed | Dependencies |
|-------|---------------|---------------|--------------|
| Phase 1 | 4 files | ~800 | None |
| Phase 2 | 4 files | ~600 net | Requires refactoring |
| Phase 3 | 1 file | ~100 | Partial removal |
| **Total** | **9 files** | **~1,500** | |

**Result:** The codebase will continue to function exactly as before because:
1. gRPC handlers were never registered - only ConnectRPC handlers run
2. Duplicate auth utilities were never called - middleware has its own implementation
3. Admin handler was never registered - admin operations use CLI

---

## Command to Re-run Analysis

```bash
# Full analysis
~/go/bin/deadcode ./...

# Excluding test utilities
~/go/bin/deadcode ./... 2>&1 | grep -v "testutil\|fixtures\|mocks\|_test.go"

# Count by file
~/go/bin/deadcode ./... 2>&1 | cut -d: -f1 | sort | uniq -c | sort -rn
```
