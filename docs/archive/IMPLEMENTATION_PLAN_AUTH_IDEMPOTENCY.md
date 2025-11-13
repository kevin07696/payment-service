# Implementation Plan: Authorization & Idempotency

**Version**: 1.0
**Last Updated**: 2025-01-12
**Approach**: TDD for unit tests, Integration tests for edge cases

---

## Phase 1: Authorization Infrastructure (Week 1)

### 1.1 AuthContext + Interceptor (TDD)

**Files to Create:**
```
internal/middleware/auth_context.go
internal/middleware/auth_context_test.go
internal/middleware/auth_interceptor.go
internal/middleware/auth_interceptor_test.go
```

**Unit Tests (Write First):**
- `TestAuthContext_Creation`
- `TestAuthContext_FromHeaders`
- `TestInterceptor_ValidateAPIKey`
- `TestInterceptor_BuildAuthContext`
- `TestInterceptor_InvalidCredentials`

**Implementation:**
```go
// AuthContext structure
type AuthContext struct {
    ActorType  ActorType
    ActorID    string
    MerchantID *string
    CustomerID *string
    SessionID  *string
    Scopes     []string
}

// Interceptor validates credentials, builds context
type AuthInterceptor struct {
    apiKeyStore APIKeyStore
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc
```

**Integration Tests:**
- Real API key validation
- Missing credentials returns 401
- Invalid API key returns 401

---

### 1.2 Authorization Service (TDD)

**Files to Create:**
```
internal/services/authorization/authorization_service.go
internal/services/authorization/authorization_service_test.go
internal/services/authorization/errors.go
```

**Unit Tests (Write First):**
- `TestCanAccessTransaction_Customer_OwnTransaction`
- `TestCanAccessTransaction_Customer_OtherTransaction`
- `TestCanAccessTransaction_Merchant_OwnMerchant`
- `TestCanAccessTransaction_Merchant_OtherMerchant`
- `TestCanAccessTransaction_Guest_MatchingSession`
- `TestCanAccessTransaction_Guest_DifferentSession`
- `TestCanAccessTransaction_Admin_AllAccess`
- `TestBuildTransactionFilters_CustomerForced`
- `TestBuildTransactionFilters_MerchantForced`

**Implementation:**
```go
type AuthorizationService interface {
    CanAccessTransaction(ctx *AuthContext, tx *Transaction) error
    CanAccessTransactionGroup(ctx *AuthContext, groupID string, txs []*Transaction) error
    BuildTransactionFilters(ctx *AuthContext, filters *ListFilters) (*ListFilters, error)
}
```

**Integration Tests:**
- Database queries with authorization filters
- Forced filters actually work (customer cannot see other data)

---

### 1.3 Update Handlers (TDD)

**Files to Modify:**
```
handlers/payment/payment_handler.go
handlers/payment/payment_handler_test.go (unit)
tests/integration/authorization/ (integration)
```

**Unit Tests (Write First):**
- `TestGetTransactionsByGroupID_WithAuthContext`
- `TestListTransactions_ForcedFilters`
- `TestGetTransactionsByGroupID_Returns404NotForbidden`

**Implementation:**
- Add `authCtx, _ := middleware.GetAuthContext(ctx)` to all handlers
- Call `authzService.CanAccessTransaction()` before returning data
- Apply `authzService.BuildTransactionFilters()` to list queries
- Return `codes.NotFound` (never `codes.PermissionDenied`)

**Integration Tests (Pain Points):**
- Customer enumeration attempt (tries random transaction IDs)
- Merchant tries to access other merchant's data
- Guest with expired session
- Return 404 looks identical for "not found" vs "unauthorized"

---

## Phase 2: Idempotency Gaps (Week 2)

### 2.1 Sale/Authorize Idempotency with Declined (TDD)

**Files to Create:**
```
internal/services/payment/idempotency.go
internal/services/payment/idempotency_test.go (unit)
tests/integration/payment/idempotency_declined_test.go (integration)
```

**Unit Tests (Write First):**
- `TestHandleIdempotencyConflict_ReturnsExisting`
- `TestShouldInsertTransaction_GatewaySuccess`
- `TestShouldInsertTransaction_GatewayError`
- `TestCheckExistingTransaction_Found`
- `TestCheckExistingTransaction_NotFound`

**Implementation:**
```go
func (s *paymentService) Sale(ctx context.Context, req *SaleRequest) (*Transaction, error) {
    // Call gateway
    epxResp, err := s.gateway.ProcessTransaction(ctx, epxReq)
    if err != nil {
        // ❌ Network error - DON'T insert
        return nil, fmt.Errorf("gateway error: %w", err)
    }

    // ✅ Gateway responded (approved OR declined) - DO insert
    err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
        dbTx, err := q.CreateTransaction(ctx, params)
        if err != nil {
            // Check for idempotency conflict
            if isUniqueViolation(err) {
                existing, fetchErr := q.GetTransactionByID(ctx, txID)
                if fetchErr == nil {
                    transaction = dbTransactionToDomain(existing)
                    return nil
                }
            }
            return err
        }
        transaction = dbTransactionToDomain(dbTx)
        return nil
    })
}
```

**Integration Tests (Pain Points):**
- Declined transaction blocks retry with same key
- Client generates NEW key and succeeds
- Network timeout allows retry with SAME key
- Concurrent requests with same idempotency key
- Approved → Retry returns same transaction
- Declined → Retry returns same declined transaction

---

### 2.2 Capture/Void Idempotency (TDD)

**Files to Modify:**
```
internal/services/payment/payment_service.go
internal/services/payment/payment_service_test.go (unit)
tests/integration/payment/idempotency_capture_void_test.go (integration)
```

**Unit Tests (Write First):**
- `TestCapture_Idempotency_ReturnsExisting`
- `TestVoid_Idempotency_ReturnsExisting`
- `TestCapture_DifferentKeys_CreatesMultiple`

**Implementation:**
- Apply same idempotency pattern to Capture/Void/Refund
- Use `ON CONFLICT DO NOTHING RETURNING *` pattern
- Check for no rows returned, fetch existing if conflict

**Integration Tests (Pain Points):**
- Partial captures with different idempotency keys (both succeed)
- Multiple refunds with different keys (all succeed)
- Retry capture with same key (returns existing)

---

## Phase 3: Edge Cases & Security (Week 3)

### 3.1 Session Expiry + Email Fallback (Integration Focus)

**Files to Create:**
```
handlers/payment/lookup_handler.go
handlers/payment/lookup_handler_test.go
tests/integration/authorization/session_expiry_test.go
```

**Unit Tests (Minimal):**
- `TestLookupOrderByEmail_ValidatesEmail`
- `TestLookupOrderByEmail_RateLimiting`

**Integration Tests (Pain Points):**
- Guest creates payment with session_id
- Session expires (different session_id)
- Guest cannot access via old endpoint
- Guest CAN access via email lookup endpoint
- Rate limiting prevents brute force email enumeration
- Timing attacks prevented (constant-time response)

---

### 3.2 Rate Limiting (Integration Focus)

**Files to Create:**
```
internal/middleware/rate_limiter.go
internal/middleware/rate_limiter_test.go (unit)
tests/integration/authorization/rate_limiting_test.go (integration)
```

**Unit Tests:**
- `TestRateLimiter_AllowsWithinLimit`
- `TestRateLimiter_BlocksOverLimit`
- `TestRateLimiter_ResetsAfterWindow`

**Integration Tests (Pain Points):**
- 10 requests succeed, 11th gets 429
- Rate limit per actor (customer A doesn't affect customer B)
- Rate limit per endpoint (different limits for read vs write)
- Wait for window reset, works again

---

### 3.3 Audit Logging (TDD)

**Files to Create:**
```
internal/services/audit/audit_service.go
internal/services/audit/audit_service_test.go
tests/integration/audit/audit_log_test.go
```

**Unit Tests:**
- `TestAuditLog_AuthorizationDecision`
- `TestAuditLog_PaymentOperation`
- `TestAuditLog_IncludesContext`

**Implementation:**
```go
func (s *AuthorizationService) CanAccessTransaction(ctx *AuthContext, tx *Transaction) error {
    allowed := s.checkAccess(ctx, tx)

    // Log decision
    s.auditLogger.Log(AuditEvent{
        EventType:  "authorization_check",
        ActorType:  ctx.ActorType,
        ActorID:    ctx.ActorID,
        Resource:   "transaction",
        ResourceID: tx.ID,
        Allowed:    allowed,
    })

    if !allowed {
        return ErrUnauthorized
    }
    return nil
}
```

**Integration Tests (Pain Points):**
- Audit log entry created for every authorization check
- Failed authorization logged with details
- PCI-compliant audit trail (immutable, queryable)

---

## Phase 4: Validation & Performance (Week 4)

### 4.1 Business Rule Validation (TDD)

**Files to Create:**
```
internal/services/payment/validation.go
internal/services/payment/validation_test.go
tests/integration/payment/validation_test.go
```

**Unit Tests:**
- `TestValidateRefundAmount_NotExceedOriginal`
- `TestValidateCaptureAmount_NotExceedAuthorization`
- `TestValidateTransactionState_CanBeRefunded`

**Integration Tests (Pain Points):**
- Cannot refund more than original amount
- Cannot capture more than authorized
- Cannot void already-refunded transaction
- Database constraints prevent over-refunding

---

### 4.2 Performance & Concurrency (Integration Focus)

**Files to Create:**
```
tests/integration/performance/concurrent_requests_test.go
tests/integration/performance/high_volume_test.go
```

**Integration Tests (Pain Points):**
- 100 concurrent requests with same idempotency key (only 1 transaction)
- 1000 requests/sec sustained load
- Authorization checks don't slow down queries
- Database connection pool handles load
- Rate limiting works under load

---

## Summary: Test Organization

### Unit Tests (TDD - Write First)
```
internal/middleware/auth_context_test.go              ✅ Fast, isolated
internal/middleware/auth_interceptor_test.go          ✅ Mock dependencies
internal/services/authorization/authorization_service_test.go  ✅ Pure logic
internal/services/payment/idempotency_test.go         ✅ Business rules
internal/services/audit/audit_service_test.go         ✅ Mock database
```

### Integration Tests (Edge Cases & Pain Points)
```
tests/integration/authorization/
  ├── customer_authorization_test.go        ❌ Real DB queries
  ├── merchant_authorization_test.go        ❌ Multi-tenant isolation
  ├── guest_authorization_test.go           ❌ Session matching
  ├── admin_authorization_test.go           ❌ Full access
  ├── error_handling_test.go                ❌ 404 vs 403
  ├── forced_filters_test.go                ❌ Cannot override
  ├── session_expiry_test.go                ❌ Email fallback
  └── rate_limiting_test.go                 ❌ Load scenarios

tests/integration/payment/
  ├── idempotency_declined_test.go          ❌ Gateway responses
  ├── idempotency_network_error_test.go     ❌ Error scenarios
  ├── idempotency_capture_void_test.go      ❌ Capture/Void flow
  ├── validation_test.go                    ❌ Business rules
  └── concurrent_requests_test.go           ❌ Race conditions
```

---

## Execution Plan

### Week 1: Authorization Core
```
Day 1-2: AuthContext + Interceptor (TDD)
Day 3-4: AuthorizationService (TDD)
Day 5:   Update handlers, integration tests
```

### Week 2: Idempotency Fixes
```
Day 1-2: Sale/Authorize declined handling (TDD)
Day 3-4: Capture/Void idempotency (TDD)
Day 5:   Integration tests for pain points
```

### Week 3: Edge Cases
```
Day 1-2: Session expiry + email fallback
Day 3-4: Rate limiting + audit logging
Day 5:   Security testing (enumeration, timing)
```

### Week 4: Validation & Polish
```
Day 1-2: Business rule validation
Day 3-4: Performance testing
Day 5:   Documentation + code review
```

---

## Success Criteria

### Code Coverage
- Unit tests: 90%+ for new code
- Integration tests: Cover all edge cases
- Overall: 95%+ coverage

### Security
- ✅ Authorization enforced on every endpoint
- ✅ No enumeration attacks possible
- ✅ Audit trail for compliance
- ✅ Rate limiting prevents abuse

### Reliability
- ✅ Idempotency prevents double-charging
- ✅ Network errors safely retryable
- ✅ Concurrent requests handled correctly

### Performance
- ✅ Authorization adds <10ms latency
- ✅ 1000 req/s sustained throughput
- ✅ No N+1 query problems

---

## Quick Start

```bash
# Week 1 Day 1: Start with AuthContext
cd internal/middleware
touch auth_context.go auth_context_test.go

# Write tests first
go test -v ./internal/middleware/auth_context_test.go

# Implement until tests pass
go test -v ./internal/middleware/

# Repeat for each component
```

---

**Approach**: TDD for business logic, Integration for pain points
**Timeline**: 4 weeks to production-ready
**Priority**: P0 (authorization + declined idempotency) in Week 1-2
