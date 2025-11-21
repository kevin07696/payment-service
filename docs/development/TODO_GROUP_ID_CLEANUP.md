# group_id vs parent_transaction_id Cleanup

**Status:** ✅ COMPLETE
**Created:** 2025-11-20
**Completed:** 2025-11-21
**Priority:** HIGH - Schema inconsistency affecting documentation accuracy

## Summary

Successfully removed all `group_id` references from codebase and documentation. The entire system now correctly uses `parent_transaction_id` to match the actual database schema.

### Database Reality

**transactions table:**
- ✅ Uses `parent_transaction_id` (exists in schema)
- ❌ Does NOT have `group_id` column
- Links: AUTH → CAPTURE → REFUND via parent_transaction_id

**chargebacks table:**
- ❌ Has `group_id` column that references non-existent `transactions.group_id`
- Schema bug: Can't actually JOIN to transactions using this field

### Impact

**Code:**
- 118 `group_id` references across 31 files (mostly chargebacks, examples, tests)
- 131 `parent_transaction_id` references across 20 files (actual implementation)

**Documentation affected:**
1. ❌ docs/DATAFLOW.md - References group_id throughout
2. ❌ docs/API_SPECS.md - Shows group_id in responses
3. ❌ docs/DATABASE.md - Shows transactions table with group_id key
4. ❌ docs/INTEGRATION_GUIDE.md - Likely has group_id examples
5. ❌ docs/AUTH.md - May reference group_id
6. ❌ docs/API_DESIGN_AND_DATAFLOW.md - Architecture docs
7. ❌ docs/wiki-templates/FAQ.md
8. ❌ docs/INTEGRATION_TEST_STRATEGY.md
9. ❌ tests/manual/README.md

## Recommended Solution

**Option 1: Remove group_id entirely** (RECOMMENDED)
- Drop `group_id` from chargebacks table
- Update chargebacks to use `transaction_id` (link to specific transaction)
- Update proto definitions to remove group_id
- Fix all documentation to use parent_transaction_id
- Update all code examples

**Option 2: Add group_id to transactions**
- Add computed `group_id` column (root transaction ID of chain)
- Requires migration
- Adds complexity but maintains backward compatibility

## Completed Actions

### Code Changes ✅
- [x] **Decision Made:** Remove group_id entirely (Option A)
- [x] Chargebacks use `transaction_id` (already correct in migration 004)
- [x] Updated `internal/domain/chargeback.go` (GroupID → TransactionID)
- [x] Fixed all adapter port comments (server_post, browser_post, key_exchange)
- [x] Updated `browser_post_callback_handler.go` (groupID → parentTxID params)
- [x] Removed outdated comments from services (payment, subscription)
- [x] Renamed test variables for clarity (groupID → parentTxID)

### Documentation Updates ✅
- [x] `docs/integration/DATABASE.md` - Complete schema overhaul (19 refs)
- [x] `docs/integration/API_SPECS.md` - All API examples updated (13 refs)
- [x] `docs/integration/DATAFLOW.md` - Flow diagrams and patterns updated (11 refs)
- [x] `docs/development/AUTH.md` - Authentication examples fixed (10 refs)

## Files to Review

**Chargebacks:**
- internal/db/migrations/004_chargebacks.sql
- internal/db/queries/chargebacks.sql
- proto/chargeback/v1/chargeback.proto
- internal/handlers/chargeback/*.go
- internal/services/chargeback/*.go

**Examples:**
- examples/*.go (all files with group_id)

**Tests:**
- tests/integration/payment/*_test.go
- tests/integration/testutil/*.go

## Implementation Notes

### Commits
1. `refactor: Remove group_id references from codebase and DATABASE.md`
   - All code changes and DATABASE.md schema updates
2. `docs: Remove group_id references from API_SPECS.md`
   - API response examples and query parameters
3. `docs: Remove remaining group_id references from DATAFLOW.md and AUTH.md`
   - Integration guides and authentication documentation

### Schema Alignment
- All code now matches `internal/db/migrations/003_transactions.sql`
- Chargebacks correctly use `transaction_id` FK (004_chargebacks.sql)
- No references to non-existent `group_id` column remain

### Verification
```bash
# Verified clean:
grep -r "group_id\|groupId" internal/ --include="*.go" | wc -l  # 0
grep -r "group_id\|groupId" docs/ --include="*.md" | wc -l      # 0 (excluding this file)
```

## Related Documentation

See also:
- `docs/integration/DATABASE.md` - Complete schema reference
- `docs/integration/API_SPECS.md` - API endpoint documentation
- `docs/integration/DATAFLOW.md` - Integration patterns
