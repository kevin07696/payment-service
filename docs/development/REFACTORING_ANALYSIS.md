# Payment Service Refactoring Analysis

**Generated**: 2025-11-19
**Scope**: Business logic review based on API design, authentication docs, and unit tests
**Status**: Analysis Complete - Recommendations Ready

---

## Executive Summary

The payment service demonstrates **excellent architectural patterns** with port/adapter design, comprehensive testing, and clean separation of concerns. However, there are opportunities to improve **maintainability**, **reduce code duplication**, and **enhance modularity** as the codebase grows.

**Overall Health**: ✅ **Good** (7/10)
- Strong foundation with ports/adapters
- Excellent test coverage (unit + integration)
- WAL-style state computation is elegant
- Some refactoring needed to reduce file size and duplication

---

## Current Architecture Assessment

### ✅ Strengths

1. **Port/Adapter Pattern**
   - Clean separation between domain logic and infrastructure
   - Easy to mock dependencies for testing
   - Database, EPX gateway, and secret manager all properly abstracted
   - Location: `internal/adapters/ports/`, `internal/services/ports/`

2. **Transaction State Management**
   - WAL-style state computation in `group_state.go` (175 lines)
   - Pure functions for state transitions
   - Comprehensive validation logic
   - Well-tested with table-driven tests

3. **Test Coverage**
   - Excellent unit tests with mocks
   - Table-driven validation tests
   - Integration tests for critical paths
   - Tests demonstrate how ports enable testability

4. **Domain Modeling**
   - Clear domain types (Transaction, PaymentMethod, Merchant)
   - Transaction types well-defined (Auth, Sale, Capture, Refund, Void)
   - BRIC token handling abstracted properly

5. **Idempotency**
   - Transaction ID as idempotency key
   - Database constraints prevent duplicates
   - Proper error handling for unique violations

### ⚠️ Areas for Improvement

1. **Large Service Files**
   - `payment_service.go`: **1530 lines** (exceeds recommended 500-800 lines)
   - Contains 10+ methods with similar patterns
   - Mixed concerns: auth, validation, EPX communication, DB operations

2. **Code Duplication**
   - Merchant credential fetching repeated in every method
   - Payment token resolution pattern duplicated
   - Transaction ID parsing duplicated
   - EPX request building has similar structure across methods

3. **Mixed Responsibilities**
   - Authentication/authorization logic in service layer
   - Should be middleware or interceptor
   - `resolveMerchantID()` and `validateTransactionAccess()` not core business logic

4. **Helper Function Proliferation**
   - 10+ conversion helpers scattered in test files and services
   - `sqlcToDomain`, `toNullableText`, `toNullableUUID`, etc.
   - Should be consolidated into converter package

5. **Limited Error Granularity**
   - Good error wrapping but few custom error types
   - Would benefit from structured error codes for API responses
   - Client needs to parse error strings

---

## Refactoring Recommendations

### Priority 1: High Impact, Low Risk

#### 1.1 Extract Merchant Operations (High Impact)

**Problem**: Merchant credential fetching duplicated in 8+ locations

**Solution**: Create `MerchantCredentialResolver` service

```go
// internal/services/merchant/credential_resolver.go
type CredentialResolver struct {
    db            *database.PostgreSQLAdapter
    secretManager adapterports.SecretManagerAdapter
    logger        *zap.Logger
}

type MerchantCredentials struct {
    Merchant  sqlc.Merchant
    MACSecret string
}

func (r *CredentialResolver) Resolve(ctx context.Context, merchantID uuid.UUID) (*MerchantCredentials, error) {
    // Fetch merchant
    merchant, err := r.db.Queries().GetMerchantByID(ctx, merchantID)
    if err != nil {
        return nil, fmt.Errorf("merchant not found: %w", err)
    }

    // Check active status
    if !merchant.IsActive {
        return nil, domain.ErrMerchantInactive
    }

    // Fetch MAC secret
    secret, err := r.secretManager.GetSecret(ctx, merchant.MacSecretPath)
    if err != nil {
        return nil, fmt.Errorf("failed to get MAC secret: %w", err)
    }

    return &MerchantCredentials{
        Merchant:  merchant,
        MACSecret: secret.Value,
    }, nil
}
```

**Benefits**:
- Eliminates 50+ lines of duplicated code
- Centralizes merchant validation logic
- Easier to add caching later
- Single place to update merchant-related errors

**Affected Files**:
- `internal/services/payment/payment_service.go` (8 methods)
- `internal/services/payment_method/payment_method_service.go` (3 methods)
- `internal/handlers/payment/browser_post_callback_handler.go` (2 methods)

**Effort**: 2-3 hours
**Risk**: Low (pure extraction, no logic changes)

---

#### 1.2 Extract Payment Token Resolution (Medium Impact)

**Problem**: Payment token resolution duplicated in Sale, Authorize, Refund

**Current Pattern** (repeated 3x):
```go
var authGUID string
var paymentMethodUUID *uuid.UUID
if req.PaymentMethodID != nil {
    pmID, err := uuid.Parse(*req.PaymentMethodID)
    if err != nil {
        return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
    }
    paymentMethodUUID = &pmID
    pm, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)
    if err != nil {
        return nil, fmt.Errorf("failed to get payment method: %w", err)
    }
    authGUID = pm.PaymentToken
} else if req.PaymentToken != nil {
    authGUID = *req.PaymentToken
} else {
    return nil, fmt.Errorf("either payment_method_id or payment_token is required")
}
```

**Solution**: Create helper in payment service

```go
type PaymentTokenInfo struct {
    Token            string
    PaymentMethodID  *uuid.UUID
}

func (s *paymentService) resolvePaymentToken(ctx context.Context, paymentMethodID *string, paymentToken *string) (*PaymentTokenInfo, error) {
    if paymentMethodID != nil {
        pmID, err := uuid.Parse(*paymentMethodID)
        if err != nil {
            return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
        }

        pm, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)
        if err != nil {
            return nil, fmt.Errorf("failed to get payment method: %w", err)
        }

        return &PaymentTokenInfo{
            Token:           pm.PaymentToken,
            PaymentMethodID: &pmID,
        }, nil
    }

    if paymentToken != nil {
        return &PaymentTokenInfo{
            Token:           *paymentToken,
            PaymentMethodID: nil,
        }, nil
    }

    return nil, fmt.Errorf("either payment_method_id or payment_token is required")
}
```

**Benefits**:
- Eliminates 45+ lines of duplicated code
- Consistent validation logic
- Single point for payment method access control

**Effort**: 1 hour
**Risk**: Low

---

#### 1.3 Create Converter Package (Medium Impact)

**Problem**: Helper functions scattered across files

**Current State**:
- `sqlcToDomain()` in payment_service.go
- `toNullableText()`, `toNullableUUID()`, `toNumeric()` duplicated
- `stringOrEmpty()`, `stringToUUIDPtr()` in multiple files

**Solution**: Create `internal/converters` package

```go
// internal/converters/sqlc.go
package converters

// ToNullableText converts a string pointer to pgtype.Text
func ToNullableText(s *string) pgtype.Text {
    if s == nil {
        return pgtype.Text{Valid: false}
    }
    return pgtype.Text{String: *s, Valid: true}
}

// ToNullableUUID converts a string pointer to pgtype.UUID
func ToNullableUUID(s *string) pgtype.UUID {
    if s == nil {
        return pgtype.UUID{Valid: false}
    }
    id, err := uuid.Parse(*s)
    if err != nil {
        return pgtype.UUID{Valid: false}
    }
    return pgtype.UUID{Bytes: id, Valid: true}
}

// ToNumeric converts decimal.Decimal to pgtype.Numeric
func ToNumeric(d decimal.Decimal) pgtype.Numeric {
    return pgtype.Numeric{
        Int:   d.Coefficient(),
        Exp:   d.Exponent(),
        Valid: true,
    }
}

// StringOrEmpty returns empty string if nil, otherwise value
func StringOrEmpty(s *string) string {
    if s == nil {
        return ""
    }
    return *s
}
```

```go
// internal/converters/domain.go
package converters

// TransactionToDomain converts sqlc.Transaction to domain.Transaction
func TransactionToDomain(tx *sqlc.Transaction) *domain.Transaction {
    // ... conversion logic
}

// PaymentMethodToDomain converts sqlc.CustomerPaymentMethod to domain.PaymentMethod
func PaymentMethodToDomain(pm *sqlc.CustomerPaymentMethod) *domain.PaymentMethod {
    // ... conversion logic
}
```

**Benefits**:
- Centralized conversion logic
- Reusable across all services
- Easier to add new types
- Better testability

**Effort**: 2 hours
**Risk**: Low

---

### Priority 2: Medium Impact, Medium Risk

#### 2.1 Split Payment Service into Transaction Type Services

**Problem**: payment_service.go is 1530 lines with mixed concerns

**Current Structure**:
```
payment_service.go (1530 lines)
├── Sale()
├── Authorize()
├── Capture()
├── Void()
├── Refund()
├── GetTransaction()
├── ListTransactions()
├── resolveMerchantID()
└── validateTransactionAccess()
```

**Proposed Structure**:
```
internal/services/payment/
├── payment_service.go (main orchestrator, 200 lines)
├── sale_handler.go (Sale logic)
├── auth_handler.go (Authorize logic)
├── capture_handler.go (Capture logic)
├── void_handler.go (Void logic)
├── refund_handler.go (Refund logic)
├── query_handler.go (Get/List logic)
├── group_state.go (existing, keep as-is)
└── validation.go (extracted validation)
```

**Alternative Approach** (More Radical):
Use **Command Pattern** with handlers

```go
// internal/services/payment/commands/sale_command.go
type SaleCommand struct {
    db               *database.PostgreSQLAdapter
    serverPost       adapterports.ServerPostAdapter
    credentialResolver *merchant.CredentialResolver
    logger           *zap.Logger
}

func (c *SaleCommand) Execute(ctx context.Context, req *ports.SaleRequest) (*domain.Transaction, error) {
    // Sale-specific logic only
}
```

```go
// internal/services/payment/payment_service.go
type paymentService struct {
    saleCmd    *commands.SaleCommand
    authCmd    *commands.AuthorizeCommand
    captureCmd *commands.CaptureCommand
    // ... other commands
}

func (s *paymentService) Sale(ctx context.Context, req *ports.SaleRequest) (*domain.Transaction, error) {
    return s.saleCmd.Execute(ctx, req)
}
```

**Benefits**:
- Smaller, focused files (150-200 lines each)
- Easier to understand individual transaction types
- Parallel development (different devs, different commands)
- Better testability (test each command independently)
- Follows Single Responsibility Principle

**Drawbacks**:
- More files to navigate
- Potential for code duplication if not careful
- Requires shared utilities (credentialResolver, tokenResolver)

**Recommendation**: Use **file-per-transaction-type** approach, not full command pattern yet

**Effort**: 8-12 hours
**Risk**: Medium (requires careful extraction and testing)

---

#### 2.2 Extract Authentication/Authorization to Interceptor

**Problem**: Auth logic mixed into service layer

**Current State**:
- `resolveMerchantID()` called in every service method
- `validateTransactionAccess()` called in query methods
- Auth concerns leak into business logic

**Solution**: Create gRPC/Connect interceptor

```go
// internal/interceptors/auth_interceptor.go
type AuthInterceptor struct {
    merchantResolver *authorization.MerchantResolver
    logger           *zap.Logger
}

func (i *AuthInterceptor) Intercept(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    // Extract auth info from context
    authInfo := auth.GetAuthInfo(ctx)

    // If request has merchant_id field, resolve and validate
    if merchantIDField := extractMerchantID(req); merchantIDField != "" {
        resolvedID, err := i.resolveMerchantID(ctx, authInfo, merchantIDField)
        if err != nil {
            return nil, status.Errorf(codes.PermissionDenied, "merchant access denied: %v", err)
        }

        // Inject resolved merchant_id into context
        ctx = context.WithValue(ctx, "resolved_merchant_id", resolvedID)
    }

    return handler(ctx, req)
}
```

**Benefits**:
- Centralized auth logic
- Service methods cleaner (no auth boilerplate)
- Consistent auth behavior across all endpoints
- Easier to add rate limiting, audit logging

**Effort**: 6-8 hours
**Risk**: Medium (affects all endpoints)

---

#### 2.3 Introduce Structured Error Types

**Problem**: Error handling via string wrapping

**Current**:
```go
return nil, fmt.Errorf("failed to get merchant: %w", err)
```

**Proposed**:
```go
// internal/domain/errors.go
type ErrorCode string

const (
    ErrCodeMerchantNotFound     ErrorCode = "MERCHANT_NOT_FOUND"
    ErrCodeMerchantInactive     ErrorCode = "MERCHANT_INACTIVE"
    ErrCodeInvalidAmount        ErrorCode = "INVALID_AMOUNT"
    ErrCodeInsufficientFunds    ErrorCode = "INSUFFICIENT_FUNDS"
    ErrCodeInvalidState         ErrorCode = "INVALID_STATE"
    ErrCodeDuplicateTransaction ErrorCode = "DUPLICATE_TRANSACTION"
    // ... more codes
)

type DomainError struct {
    Code    ErrorCode
    Message string
    Cause   error
}

func (e *DomainError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Helper constructors
func NewMerchantNotFoundError(merchantID string) error {
    return &DomainError{
        Code:    ErrCodeMerchantNotFound,
        Message: fmt.Sprintf("merchant %s not found", merchantID),
    }
}
```

**Benefits**:
- Clients can programmatically handle errors
- Better API error responses (JSON with error codes)
- Easier to map to gRPC status codes
- Internationalization-friendly

**Effort**: 4-6 hours
**Risk**: Medium (affects error handling throughout)

---

### Priority 3: Low Impact, Future Considerations

#### 3.1 Add Caching Layer for Merchants

**Rationale**: Every transaction fetches merchant from DB

**Solution**: Add Redis/in-memory cache

```go
type CachedMerchantResolver struct {
    db    *database.PostgreSQLAdapter
    cache cache.Cache // Redis or in-memory
    ttl   time.Duration
}

func (r *CachedMerchantResolver) Resolve(ctx context.Context, merchantID uuid.UUID) (*MerchantCredentials, error) {
    // Check cache first
    cached, found := r.cache.Get(merchantID.String())
    if found {
        return cached.(*MerchantCredentials), nil
    }

    // Fetch from DB
    creds, err := r.fetchFromDB(ctx, merchantID)
    if err != nil {
        return nil, err
    }

    // Store in cache
    r.cache.Set(merchantID.String(), creds, r.ttl)
    return creds, nil
}
```

**Benefits**:
- Reduced DB load
- Lower latency
- Better scalability

**Effort**: 6-8 hours (includes cache invalidation strategy)
**Risk**: Medium (cache invalidation complexity)

---

#### 3.2 Extract Transaction Metadata Builder

**Problem**: Metadata building duplicated in Sale, Authorize, Capture

**Current**:
```go
metadata := req.Metadata
if metadata == nil {
    metadata = make(map[string]interface{})
}
metadata["auth_resp_text"] = epxResp.AuthRespText
metadata["auth_avs"] = epxResp.AuthAVS
metadata["auth_cvv2"] = epxResp.AuthCVV2
metadataJSON, err := json.Marshal(metadata)
```

**Solution**:
```go
type MetadataBuilder struct {
    base map[string]interface{}
}

func NewMetadataBuilder(base map[string]interface{}) *MetadataBuilder {
    if base == nil {
        base = make(map[string]interface{})
    }
    return &MetadataBuilder{base: base}
}

func (b *MetadataBuilder) WithEPXResponse(resp *adapterports.ServerPostResponse) *MetadataBuilder {
    b.base["auth_resp_text"] = resp.AuthRespText
    b.base["auth_avs"] = resp.AuthAVS
    b.base["auth_cvv2"] = resp.AuthCVV2
    return b
}

func (b *MetadataBuilder) Build() ([]byte, error) {
    return json.Marshal(b.base)
}
```

**Effort**: 1 hour
**Risk**: Low

---

## Testing Implications

### Current Test Coverage

**Strengths**:
- Excellent unit test coverage with mocks
- Table-driven tests for validation logic
- Integration tests for critical paths
- Tests demonstrate port/adapter benefits

**Gaps** (noted in comments):
1. Integration tests needed for idempotency (noted in payment_service_test.go:127-131)
2. Concurrent transaction tests needed (noted in payment_service_test.go:130)
3. EPX decline handling integration tests (noted in payment_service_test.go:131)

### Post-Refactoring Testing Strategy

After implementing refactorings:

1. **Unit Tests**: Update existing tests to use new structure
   - Merchant credential resolver tests
   - Payment token resolver tests
   - Converter package tests

2. **Integration Tests**: Add missing critical tests
   - Idempotency under concurrent load
   - Race conditions in capture/void
   - EPX error scenarios

3. **Regression Tests**: Ensure no behavior changes
   - Run full test suite before/after
   - Compare test coverage metrics
   - Verify integration tests still pass

---

## Implementation Roadmap

### Phase 1: Quick Wins (1-2 weeks)
1. Extract Merchant Credential Resolver (Priority 1.1)
2. Extract Payment Token Resolution (Priority 1.2)
3. Create Converter Package (Priority 1.3)
4. Extract Metadata Builder (Priority 3.2)

**Impact**: Reduces duplication by ~200 lines, improves readability

---

### Phase 2: Structural Improvements (2-3 weeks)
1. Split Payment Service by Transaction Type (Priority 2.1)
2. Extract Auth to Interceptor (Priority 2.2)
3. Add Integration Tests for Critical Paths

**Impact**: Major maintainability improvement, better testing

---

### Phase 3: Polish (1-2 weeks)
1. Introduce Structured Error Types (Priority 2.3)
2. Add Merchant Caching (Priority 3.1)
3. Performance testing and optimization

**Impact**: Better error handling, improved performance

---

## Refactoring Principles to Follow

1. **No Big Bang**: Refactor incrementally, one change at a time
2. **Test First**: Ensure tests pass before and after each change
3. **Backwards Compatibility**: Maintain API contracts during refactoring
4. **Extract, Don't Rewrite**: Preserve existing logic, just reorganize
5. **Review PRs Small**: Each refactoring should be reviewable in <1 hour

---

## Metrics to Track

### Before Refactoring
- Lines of code: `payment_service.go` = 1530 lines
- Code duplication: ~250 lines duplicated across files
- Test coverage: ~80% (estimated)
- Cyclomatic complexity: Payment service methods avg 8-12

### After Refactoring (Target)
- Lines of code: Largest file <500 lines
- Code duplication: <50 lines
- Test coverage: >85%
- Cyclomatic complexity: Methods avg 4-6

---

## Risk Assessment

### Low Risk Refactorings
- Merchant credential resolver ✅
- Payment token resolver ✅
- Converter package ✅
- Metadata builder ✅

### Medium Risk Refactorings
- Split payment service ⚠️ (requires careful testing)
- Auth interceptor ⚠️ (affects all endpoints)
- Structured errors ⚠️ (changes error contracts)

### High Risk Refactorings
- None identified - current architecture is sound

---

## Conclusion

The payment service codebase is **well-architected** with excellent patterns (ports/adapters, WAL-style state, comprehensive testing). The primary opportunity is **reducing code duplication** and **improving modularity** through:

1. **Extract shared logic** (merchant resolver, token resolver, converters)
2. **Split large files** (payment_service.go into focused handlers)
3. **Elevate cross-cutting concerns** (auth to interceptor)

**Recommended Next Steps**:
1. Implement Phase 1 quick wins (2 weeks)
2. Gather team feedback
3. Plan Phase 2 structural improvements
4. Iterate based on learnings

**Overall Recommendation**: ✅ Proceed with refactoring - strong foundation makes this low-risk, high-value work.

---

**Generated by**: Claude Code
**Review Date**: 2025-11-19
**Next Review**: After Phase 1 completion