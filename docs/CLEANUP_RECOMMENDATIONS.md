# Documentation Cleanup Recommendations

**Date:** 2025-11-23
**Purpose:** Identify temporary/generated documentation that can be archived or deleted

---

## Summary

After completing Style Guide and Testing Guide, several temporary planning/analysis documents can be cleaned up:

**Action Plan:**
- ✅ **Permanent docs:** Keep in current locations
- 📦 **Archive:** Move to `docs/archive/` subdirectories
- 🗑️ **Delete:** Remove after extracting value to permanent docs

---

## Files to Archive

### Planning Documents → `docs/archive/planning/`

These were temporary planning docs that served their purpose:

```bash
# Move these to docs/archive/planning/
docs/refactor/CODE_CLEANLINESS_PLAN.md              # Cleanup plan (completed)
docs/refactor/INTEGRATION_TEST_PLAN.md              # Test plan (completed)
docs/refactor/REFACTOR_PLAN.md                      # General refactor plan
docs/refactor/TDD_REFACTOR_PLAN.md                  # TDD refactor (completed)
docs/development/CUSTOMER_ID_MIGRATION_PLAN.md      # Migration plan (if completed)
```

**Rationale:** Planning documents that guided implementation but are no longer actively referenced.

---

### Analysis Documents → `docs/archive/analysis/`

These were point-in-time analysis docs:

```bash
# Move these to docs/archive/analysis/
docs/optimizations/SECURITY_SCALING_ANALYSIS.md     # One-time analysis
docs/optimizations/DATABASE_INDEX_ANALYSIS.md       # Index analysis (applied)
docs/optimizations/TIMEZONE_ANALYSIS.md             # Timezone analysis
docs/optimizations/TEST_COVERAGE_REPORT.md          # Point-in-time report
docs/optimizations/TEST_QUALITY_ANALYSIS.md         # Analysis (applied to Testing Guide)
```

**Rationale:** These were snapshots/analyses that informed decisions but aren't living documents.

---

### Implementation Reports → `docs/archive/reports/`

These document completed work:

```bash
# Move these to docs/archive/reports/
docs/refactor/cicd/IMPLEMENTATION_COMPLETE.md       # CI/CD implementation report
docs/refactor/cicd/PIPELINE_ANALYSIS.md             # Pipeline analysis (applied)
```

**Rationale:** Implementation completion reports - useful for historical reference but not active.

---

## Files to Keep (Permanent Documentation)

### Core Development Guides (`docs/development/`)

```bash
✅ KEEP docs/development/STYLE_GUIDE.md              # Living style guide
✅ KEEP docs/development/TESTING_GUIDE.md            # Living test guide
✅ KEEP docs/development/DEVELOP.md                  # Development workflow
```

**Rationale:** These are living documents that developers actively reference.

---

### Optimization References (`docs/optimizations/`)

```bash
✅ KEEP docs/optimizations/CACHING_STRATEGY.md
✅ KEEP docs/optimizations/RESILIENCE_PATTERNS.md
✅ KEEP docs/optimizations/MONITORING_OBSERVABILITY.md
✅ KEEP docs/optimizations/DATABASE_OPTIMIZATION.md
✅ KEEP docs/optimizations/OPTIMIZATION_ROADMAP.md
✅ KEEP docs/optimizations/README.md
```

**Rationale:** Active optimization strategies and patterns, not one-time plans.

---

### Integration Documentation (`docs/integration/`)

```bash
✅ KEEP docs/integration/TOKEN_GENERATION.md
✅ KEEP docs/integration/EPX_INTEGRATION.md (if exists)
✅ KEEP docs/integration/BROWSER_POST_WORKFLOW.md (if exists)
```

**Rationale:** Integration guides for external systems.

---

## Special Cases

### `docs/refactor/` Directory

**Recommendation:** Leave as-is for now, delete manually when refactoring work is complete.

**Contents:**
- `docs/refactor/DOMAIN_REFACTORING_PLAN.md` - **Can be deleted** (based on flawed "gateway-agnostic" premise - Style Guide now has pragmatic guidance instead)
- `docs/refactor/cicd/` - CI/CD refactor docs
- `docs/refactor/security/` - Security refactor docs

**Why:** You mentioned you'll delete `docs/refactor/` yourself when done with refactoring work.

---

## Cleanup Commands

When ready to archive, run these commands:

```bash
# Create archive subdirectories if needed
mkdir -p docs/archive/planning
mkdir -p docs/archive/analysis
mkdir -p docs/archive/reports

# Move planning docs
mv docs/refactor/CODE_CLEANLINESS_PLAN.md docs/archive/planning/
mv docs/refactor/INTEGRATION_TEST_PLAN.md docs/archive/planning/
mv docs/refactor/REFACTOR_PLAN.md docs/archive/planning/
mv docs/refactor/TDD_REFACTOR_PLAN.md docs/archive/planning/
mv docs/development/CUSTOMER_ID_MIGRATION_PLAN.md docs/archive/planning/ # if applicable

# Move analysis docs
mv docs/optimizations/SECURITY_SCALING_ANALYSIS.md docs/archive/analysis/
mv docs/optimizations/DATABASE_INDEX_ANALYSIS.md docs/archive/analysis/
mv docs/optimizations/TIMEZONE_ANALYSIS.md docs/archive/analysis/
mv docs/optimizations/TEST_COVERAGE_REPORT.md docs/archive/reports/
mv docs/optimizations/TEST_QUALITY_ANALYSIS.md docs/archive/analysis/

# Move implementation reports
mv docs/refactor/cicd/IMPLEMENTATION_COMPLETE.md docs/archive/reports/
mv docs/refactor/cicd/PIPELINE_ANALYSIS.md docs/archive/analysis/
```

---

## Archive Directory Structure

After cleanup, `docs/archive/` structure:

```
docs/archive/
├── analysis/               # Point-in-time analyses
│   ├── E2E_VS_INTEGRATION_ANALYSIS.md
│   ├── REFACTORING_ANALYSIS.md
│   ├── SECURITY_SCALING_ANALYSIS.md
│   ├── DATABASE_INDEX_ANALYSIS.md
│   ├── TIMEZONE_ANALYSIS.md
│   ├── TEST_QUALITY_ANALYSIS.md
│   └── PIPELINE_ANALYSIS.md
│
├── planning/               # Completed planning docs
│   ├── TODO_IMPLEMENTATION_PLAN.md
│   ├── TODO_IMPLEMENTATION_PLAN_UPDATED.md
│   ├── TODO_REVIEW.md
│   ├── DOCUMENTATION_ORGANIZATION_PLAN.md
│   ├── CODE_CLEANLINESS_PLAN.md
│   ├── INTEGRATION_TEST_PLAN.md
│   ├── REFACTOR_PLAN.md
│   ├── TDD_REFACTOR_PLAN.md
│   └── CUSTOMER_ID_MIGRATION_PLAN.md
│
├── reports/                # Implementation reports
│   ├── TEST_COVERAGE_REPORT.md
│   └── IMPLEMENTATION_COMPLETE.md
│
├── research/               # Research docs (if any)
└── reviews/                # Code review docs (if any)
```

---

## Benefits of Archiving

1. **Cleaner docs/ structure** - Easier to find active documentation
2. **Historical reference preserved** - Archive maintains context for future reference
3. **Clear separation** - Active vs completed documentation
4. **Better navigation** - Developers find living guides without clutter

---

## What NOT to Archive

**Never archive:**
- Living guides (STYLE_GUIDE.md, TESTING_GUIDE.md, DEVELOP.md)
- Active integration docs (TOKEN_GENERATION.md)
- Ongoing optimization strategies (CACHING_STRATEGY.md, RESILIENCE_PATTERNS.md)
- API specifications (API_SPECS.md)
- Database documentation (DATABASE.md)
- Migration files (db/migrations/*.sql)

---

**Next Steps:**
1. ✅ Review this cleanup plan
2. ⏳ Run cleanup commands when ready
3. ⏳ Delete `docs/refactor/` when refactoring work complete
4. ⏳ Delete this file after cleanup done
