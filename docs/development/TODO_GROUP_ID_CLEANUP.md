# TODO: group_id vs parent_transaction_id Cleanup

**Status:** 🔴 BLOCKED - Waiting for implementation decision
**Created:** 2025-11-20
**Priority:** HIGH - Schema inconsistency affecting documentation accuracy

## Problem

The database schema uses `parent_transaction_id` to link transactions, but documentation and some code still references a non-existent `group_id` column.

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

## Action Items (After Implementation Decision)

### Code Changes
- [ ] Decide: Remove group_id OR Add to transactions
- [ ] Update chargebacks table migration
- [ ] Update proto/chargeback/v1/chargeback.proto
- [ ] Update all test files
- [ ] Update example files
- [ ] Regenerate sqlc models
- [ ] Update API handlers

### Documentation Updates
- [ ] docs/DATAFLOW.md - Fix all group_id references
- [ ] docs/API_SPECS.md - Update response examples
- [ ] docs/DATABASE.md - Fix schema documentation
- [ ] docs/INTEGRATION_GUIDE.md - Update integration examples
- [ ] docs/AUTH.md - Fix authorization examples
- [ ] docs/API_DESIGN_AND_DATAFLOW.md - Update architecture
- [ ] docs/wiki-templates/FAQ.md
- [ ] docs/INTEGRATION_TEST_STRATEGY.md
- [ ] tests/manual/README.md

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

## Notes

- The current schema is inconsistent and broken
- Chargebacks cannot properly link to transactions
- Documentation is misleading developers
- Should be fixed before production deployment
