# Agent → Merchant Refactoring Plan

**Date**: 2025-11-13
**Status**: Planning
**Goal**: Systematically rename all "agent" terminology to "merchant" for consistency and clarity

## Executive Summary

The codebase currently uses "agent" and "merchant" terminology inconsistently. This refactoring will:
1. Unify terminology to use "merchant" throughout
2. Maintain backward compatibility where necessary (gRPC API)
3. Update database schema progressively
4. Preserve functionality while improving code clarity

## Impact Analysis

### Breaking Changes
- gRPC API changes (can be mitigated with aliasing/versioning)
- Database column renames (requires migration)
- Secret Manager path changes (requires data migration)

### Non-Breaking Changes
- Internal Go code (struct fields, function names, comments)
- Documentation updates
- Log messages

## Refactoring Strategy

### Phase 1: Internal Code (Non-Breaking) ✅ PRIORITY
Change internal Go code without affecting external APIs or database schema.

### Phase 2: Database Schema (Controlled Migration) ⚠️
Update database columns with proper migrations and backward compatibility.

### Phase 3: External APIs (Versioned) 🔴 BREAKING
Update gRPC proto definitions with versioning strategy.

### Phase 4: Infrastructure (Data Migration) 🔧
Migrate Secret Manager paths and external references.

---

## Phase 1: Internal Code Refactoring (Non-Breaking)

### Priority 1.1: Domain Errors
**File**: `/internal/domain/errors.go`

**Changes**:
```go
// OLD
ErrAgentNotFound      = errors.New("agent not found")
ErrAgentInactive      = errors.New("agent is inactive")
ErrAgentAlreadyExists = errors.New("agent already exists")

// NEW
ErrMerchantNotFound      = errors.New("merchant not found")
ErrMerchantInactive      = errors.New("merchant is inactive")
ErrMerchantAlreadyExists = errors.New("merchant already exists")
```

**Impact**: Update all references in:
- `/internal/handlers/agent/agent_handler.go`
- `/internal/services/merchant/merchant_service.go`
- Any other files using these errors

### Priority 1.2: Service Port Interface
**File**: `/internal/services/ports/merchant_service.go`

**Changes**:
```go
// Rename types
RegisterAgentRequest → RegisterMerchantRequest
UpdateAgentRequest → UpdateMerchantRequest
RotateMACRequest → RotateMerchantMACRequest (or keep as-is since MAC is the focus)

// Rename fields
AgentID → MerchantID (in all structs)
AgentName → MerchantName

// Rename interface methods
RegisterAgent() → RegisterMerchant()
GetAgent() → GetMerchant()
ListAgents() → ListMerchants()
UpdateAgent() → UpdateMerchant()
DeactivateAgent() → DeactivateMerchant()
```

**Impact**: Update all implementations and callers

### Priority 1.3: Domain Model Comments
**File**: `/internal/domain/merchant.go`

**Changes**:
- Line 15: "merchant/agent" → "merchant"
- Line 20: Keep `AgentID` field name for now (see Phase 2)
- Lines 45-76: Update method comments to reference "merchant" instead of "agent"

```go
// OLD
// IsSandbox returns true if this agent is using sandbox environment
// CanProcessTransactions returns true if the agent can process transactions
// Deactivate marks the agent as inactive

// NEW
// IsSandbox returns true if this merchant is using sandbox environment
// CanProcessTransactions returns true if the merchant can process transactions
// Deactivate marks the merchant as inactive
```

### Priority 1.4: Merchant Service Implementation
**File**: `/internal/services/merchant/merchant_service.go`

**Changes**:
1. Rename all function names:
   - `RegisterAgent()` → `RegisterMerchant()`
   - `GetAgent()` → `GetMerchant()`
   - `ListAgents()` → `ListMerchants()`
   - `UpdateAgent()` → `UpdateMerchant()`
   - `DeactivateAgent()` → `DeactivateMerchant()`
   - `getAgentByIdempotencyKey()` → `getMerchantByIdempotencyKey()`
   - `sqlcAgentToDomain()` → `sqlcMerchantToDomain()`

2. Update all comments and log messages:
   - "Registering new agent" → "Registering new merchant"
   - "agent_id" → "merchant_id" (in logs)
   - All comment references

3. Variable renames:
   - `agent` → `merchant`
   - `agents` → `merchants`
   - `dbMerchant` → keep (accurate since it's from sqlc)

4. **DEFER**: Secret path format (Phase 4)
   - Keep: `"payment-service/agents/%s/mac"` for now
   - Will migrate in Phase 4

### Priority 1.5: Agent Handler
**File**: `/internal/handlers/agent/agent_handler.go`

**Changes**:
1. Update all internal function names:
   - `RegisterAgent()` → `RegisterMerchant()`
   - `GetAgent()` → `GetMerchant()`
   - `ListAgents()` → `ListMerchants()`
   - `UpdateAgent()` → `UpdateMerchant()`
   - `DeactivateAgent()` → `DeactivateMerchant()`

2. Helper functions:
   - `agentToResponse()` → `merchantToResponse()`
   - `agentToProto()` → `merchantToProto()`
   - `agentToSummary()` → `merchantToSummary()`
   - `validateRegisterAgentRequest()` → `validateRegisterMerchantRequest()`

3. Variable renames:
   - `agent` → `merchant`
   - `agents` → `merchants`
   - `protoAgents` → `protoMerchants`

4. Update all comments and log messages

5. **KEEP**: Proto type references (until Phase 3)
   - `agentv1.AgentResponse`
   - `agentv1.RegisterAgentRequest`
   - etc.

### Priority 1.6: Package Rename Consideration
**Directory**: `/internal/handlers/agent/`

**Decision Point**: Rename to `/internal/handlers/merchant/`?

**Recommendation**:
- ✅ YES - for consistency
- Create new directory: `/internal/handlers/merchant/`
- Move `agent_handler.go` → `merchant_handler.go`
- Update all imports
- Update `cmd/server/main.go` to import from new location

---

## Phase 2: Database Schema (Controlled Migration)

### Priority 2.1: Field Name in Structs
**Files**:
- `/internal/domain/merchant.go` (line 20)
- `/internal/db/sqlc/models.go` (line 35)

**Current**:
```go
// domain.Merchant
AgentID string `json:"agent_id"`

// sqlc.Chargeback
AgentID string `json:"agent_id"`
```

**Strategy**:
1. Add new field alongside old one
2. Populate both during transition
3. Deprecate old field
4. Remove old field

**OR** (Simpler):
Keep `AgentID` as the field name since it's external-facing and used in APIs. This avoids database migration complexity.

**Recommendation**: **DEFER** - Keep `AgentID` field name
- Reasoning: It's used extensively in APIs, database columns, and proto definitions
- Changing it requires coordinated database migration + API changes
- The field semantically represents a merchant identifier, but the name can remain `AgentID` for backward compatibility

### Priority 2.2: Database Column Renames (If Needed)
**Files**: Migrations, queries, seed data

**Current columns**:
- `merchants.slug` (this is actually the agent_id/merchant_id)
- `chargebacks.agent_id`
- Various `agent_id` foreign keys

**Decision**: **DEFER** to Phase 4 or consider NOT doing
- Database column names can stay as `agent_id`
- SQL query comments can be updated
- Avoids complex migration

---

## Phase 3: External APIs (gRPC Proto)

### Priority 3.1: Proto Service and Messages
**File**: `/proto/agent/v1/agent.proto`

**Decision Point**: Breaking change vs. backward compatibility

**Option A: Version Bump (Recommended)**
1. Create `/proto/merchant/v1/merchant.proto`
2. Keep `/proto/agent/v1/agent.proto` with deprecation notices
3. Implement both services pointing to same backend
4. Document migration path for clients

**Option B: In-Place Rename (Breaking)**
1. Rename package: `agent.v1` → `merchant.v1`
2. Rename service: `AgentService` → `MerchantService`
3. Rename all message types
4. Document as breaking change
5. Provide migration guide

**Recommendation**: **Option A** - Version bump for safety

**Changes for Option A**:
```protobuf
// proto/merchant/v1/merchant.proto
package merchant.v1;

service MerchantService {
  rpc RegisterMerchant(RegisterMerchantRequest) returns (MerchantResponse);
  rpc GetMerchant(GetMerchantRequest) returns (Merchant);
  rpc ListMerchants(ListMerchantsRequest) returns (ListMerchantsResponse);
  rpc UpdateMerchant(UpdateMerchantRequest) returns (MerchantResponse);
  rpc DeactivateMerchant(DeactivateMerchantRequest) returns (MerchantResponse);
  rpc RotateMAC(RotateMACRequest) returns (RotateMACResponse);
}

message Merchant {
  int64 id = 1;
  string merchant_id = 2;  // Changed from agent_id
  // ... rest of fields
}
```

**Implementation**:
1. Create new proto file and generate code
2. Create new handler implementing `MerchantService`
3. Register both services in `cmd/server/main.go`
4. Both handlers can use same backend service
5. Add deprecation warnings to `AgentService` responses

---

## Phase 4: Infrastructure & Data Migration

### Priority 4.1: Secret Manager Paths
**Current**: `payment-service/agents/{merchant_id}/mac`
**Target**: `payment-service/merchants/{merchant_id}/mac`

**Migration Strategy**:
1. Update code to check both paths (fallback)
2. Write migration script to copy secrets to new paths
3. Update `merchant_service.go` to write to new path
4. Deprecate old path after transition period
5. Clean up old secrets

**Code Change** (`internal/services/merchant/merchant_service.go:75`):
```go
// OLD
macSecretPath := fmt.Sprintf("payment-service/agents/%s/mac", req.MerchantID)

// NEW
macSecretPath := fmt.Sprintf("payment-service/merchants/%s/mac", req.MerchantID)

// TRANSITION: Add fallback lookup
func (s *merchantService) getMAC(ctx context.Context, merchantID string) (string, error) {
    // Try new path first
    newPath := fmt.Sprintf("payment-service/merchants/%s/mac", merchantID)
    secret, err := s.secretManager.GetSecret(ctx, newPath)
    if err == nil {
        return secret, nil
    }

    // Fallback to old path
    oldPath := fmt.Sprintf("payment-service/agents/%s/mac", merchantID)
    return s.secretManager.GetSecret(ctx, oldPath)
}
```

### Priority 4.2: Documentation Updates
**Files to Update**:
- `/README.md` - References to "agent" (lines 178, 186, 196, 211)
- `/CHANGELOG.md` - Add migration entry (historical entries stay)
- `/docs/API_SPECIFICATION.md`
- `/docs/DATABASE_DESIGN.md`
- `/docs/SECRETS.md`
- `/docs/TESTING.md`
- `/docs/INTEGRATION_TEST_RESULTS.md`
- All dataflow and guide documents

**Strategy**:
- Update current documentation to use "merchant"
- Add note: "Previously called 'agent' in legacy documentation"
- Update all examples and code snippets

### Priority 4.3: Seed Data & Migration Files
**Files**:
- `/internal/db/seeds/staging/003_agent_credentials.sql`
  - Consider renaming to `003_merchant_credentials.sql`
  - Update comments

**Strategy**: Can be updated as comments/documentation

---

## Implementation Order

### Sprint 1: Internal Code (Safe)
1. ✅ Domain errors (`errors.go`)
2. ✅ Service ports (`ports/merchant_service.go`)
3. ✅ Service implementation (`merchant/merchant_service.go`)
4. ✅ Handler implementation (`agent/agent_handler.go`)
5. ✅ Domain model comments (`domain/merchant.go`)
6. ✅ Package/directory rename (`handlers/agent` → `handlers/merchant`)
7. ✅ Update all imports
8. ✅ Run tests and fix issues
9. ✅ Update documentation

### Sprint 2: Database & Proto (Careful)
1. ⚠️ Decide on database column rename strategy
2. ⚠️ Create new proto v1 merchant service (if doing versioning)
3. ⚠️ Implement new merchant handler for proto
4. ⚠️ Register both services
5. ⚠️ Test backward compatibility
6. ⚠️ Update client examples

### Sprint 3: Infrastructure (Coordinated)
1. 🔧 Secret Manager path migration script
2. 🔧 Update code to use new paths with fallback
3. 🔧 Execute secret migration
4. 🔧 Monitor and validate
5. 🔧 Clean up old secrets after verification period

---

## Testing Strategy

### Unit Tests
- [ ] Update test names: `TestRegisterAgent` → `TestRegisterMerchant`
- [ ] Update test data using new terminology
- [ ] Verify error type changes

### Integration Tests
- [ ] Test backward compatibility (if keeping old API)
- [ ] Test new merchant service endpoints
- [ ] Test secret manager fallback logic
- [ ] Test database queries with new/old terminology

### Manual Testing
- [ ] gRPC client compatibility
- [ ] Secret retrieval from both paths
- [ ] End-to-end transaction flow
- [ ] Subscription and payment method operations

---

## Rollback Plan

### Phase 1 Rollback (Internal Code)
- Git revert commits
- Restore old function/type names
- Re-run tests

### Phase 2 Rollback (Database)
- Keep old columns alongside new ones
- Switch code back to use old columns
- Remove new columns in future migration

### Phase 3 Rollback (Proto)
- If versioned: Remove new service, keep old one
- If renamed: Restore old proto definitions, regenerate

### Phase 4 Rollback (Infrastructure)
- Revert to old secret paths
- Keep new secrets as backup
- Update code to use old paths

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Breaking API clients | High | High | Use versioning, maintain backward compat |
| Database migration failure | Medium | High | Additive changes only, keep old columns |
| Secret Manager path issues | Medium | Medium | Implement fallback logic, test thoroughly |
| Test failures | Low | Low | Run comprehensive test suite before merge |
| Documentation inconsistency | Medium | Low | Systematic doc review, grep validation |

---

## Success Criteria

✅ All internal Go code uses "merchant" terminology consistently
✅ No compilation errors or test failures
✅ Backward compatibility maintained for external APIs
✅ Documentation updated and consistent
✅ Database migrations executed successfully (if applicable)
✅ Secret Manager paths migrated (if applicable)
✅ All tests passing (unit, integration, manual)

---

## Decision Log

### Decision 1: Keep `AgentID` field name
- **Date**: 2025-11-13
- **Reason**: Avoid complex database + API migration
- **Status**: Approved

### Decision 2: Version new proto service
- **Date**: 2025-11-13
- **Reason**: Maintain backward compatibility
- **Status**: Proposed (needs approval)

### Decision 3: Defer secret path migration
- **Date**: 2025-11-13
- **Reason**: Infrastructure risk, can be done later
- **Status**: Proposed

---

## Next Steps

1. **Review this plan** with team/stakeholders
2. **Get approval** for breaking changes (Proto API)
3. **Create feature branch**: `refactor/agent-to-merchant`
4. **Start Sprint 1**: Internal code refactoring
5. **Run full test suite** after each phase
6. **Update CHANGELOG.md** with refactoring details

---

## Notes

- The term "agent" was originally used to represent merchants in a multi-tenant system
- This refactoring improves code clarity and aligns with business domain language
- Some external-facing identifiers (like `agent_id` parameter names in APIs) may remain for backward compatibility
- Consider this refactoring as technical debt cleanup rather than new feature work
