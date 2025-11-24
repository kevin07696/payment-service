# Documentation Cleanup Plan - 2025-11-24

**Purpose**: Clean up temporary planning/analysis docs while preserving valuable knowledge
**Status**: Ready for execution

---

## Executive Summary

**Current State**:
- 20+ temporary planning/analysis documents scattered across multiple directories
- 4 root-level markdown files that should be archived
- 1 certification sheet in wrong location (docs/ instead of root)
- Temporary directories: analysis, planning, optimizations, refactor, reports, wiki-templates

**Proposed Actions**:
1. Move certification_sheets.md to root (CERTIFICATION_SHEET.md)
2. Archive temporary planning/analysis docs to `docs/archive/`
3. Delete truly temporary files (duplicates, outdated)
4. Preserve permanent docs in integration/ and development/

---

## Directory Analysis

### 1. ROOT LEVEL FILES

#### Files to Archive (Move to docs/archive/analysis/):

**ANACHRONISTIC_CODE_AUDIT.md** (9.8 KB)
- **Status**: Temporary analysis report
- **Value**: Historical audit findings
- **Action**: `mv ANACHRONISTIC_CODE_AUDIT.md docs/archive/analysis/`
- **Reason**: Audit completed, issues addressed

**EPX_AUTH_RESP_TEXT_VALUES.md** (6.5 KB)
- **Status**: EPX integration reference
- **Value**: Actual EPX response values for certification
- **Action**: `mv EPX_AUTH_RESP_TEXT_VALUES.md docs/archive/analysis/`
- **Reason**: Valuable reference but not needed in root

**EPX_TOKENIZATION_ISSUE.md** (3.9 KB)
- **Status**: Troubleshooting document
- **Value**: Historical issue resolution
- **Action**: `mv EPX_TOKENIZATION_ISSUE.md docs/archive/analysis/`
- **Reason**: Issue resolved, kept for historical reference

#### Files to Keep in Root:

**README.md** ✅ KEEP
- Essential project documentation

**CHANGELOG.md** ✅ KEEP
- Version history and release notes

---

### 2. CERTIFICATION SHEET

**Current**: `docs/certification_sheets.md`
**Proposed**: `CERTIFICATION_SHEET.md` (root)

**Action**:
```bash
git mv docs/certification_sheets.md CERTIFICATION_SHEET.md
```

**Reason**: Certification sheet should be prominently accessible in root directory

---

### 3. docs/analysis/ (4 files, 160 KB)

**Purpose**: Temporary analysis documents from development

**Files**:
1. `BUSINESS_LOGIC_ANALYSIS.md` (64.5 KB) - Business logic audit
2. `CRITICAL_ISSUES_DISCOVERY.md` (15.6 KB) - Issue discovery report
3. `OPTIMIZATION_COMPLETE.md` (11.7 KB) - Optimization completion report
4. `TABLE_USAGE_ANALYSIS.md` (67.7 KB) - Database usage analysis

**Action**: ✅ KEEP IN PLACE
**Reason**: Already in docs/analysis/, may be referenced for historical understanding

---

### 4. docs/planning/ (1 file, 32 KB)

**Files**:
1. `IMPLEMENTATION_PLAN_PHASE1.md` (31.6 KB) - Phase 1 implementation plan

**Action**: Move to `docs/archive/planning/`
**Reason**: Implementation complete, archived for historical reference

---

### 5. docs/optimizations/ (21 files, 596 KB)

**Purpose**: Optimization research and planning docs

**Category**: Temporary planning documents
**Value**: Historical optimization research

**Action**: Move entire directory to `docs/archive/`
```bash
git mv docs/optimizations docs/archive/optimizations
```

**Reason**:
- Optimization work completed
- All findings integrated into code/permanent docs
- Historical value only

---

### 6. docs/refactor/ (15 files)

**Structure**:
```
docs/refactor/
├── cicd/ (10 files - CI/CD analysis)
├── CODE_CLEANLINESS_PLAN.md
├── DOMAIN_REFACTORING_PLAN.md (OUTDATED - gateway-agnostic pattern rejected)
├── INTEGRATION_TEST_PLAN.md
├── REFACTOR_PLAN.md
└── TDD_REFACTOR_PLAN.md
```

**Analysis**:

#### Keep (Still Relevant):
- **cicd/** - Move to `docs/development/CICD.md` (consolidated)
  - IMPLEMENTATION_COMPLETE.md has valuable deployment info
  - Other files can be summarized in permanent doc

#### Archive (Completed):
- `CODE_CLEANLINESS_PLAN.md` → docs/archive/planning/
- `INTEGRATION_TEST_PLAN.md` → docs/archive/planning/
- `REFACTOR_PLAN.md` → docs/archive/planning/
- `TDD_REFACTOR_PLAN.md` → docs/archive/planning/

#### Delete (Flawed/Outdated):
- **`DOMAIN_REFACTORING_PLAN.md`** - DELETE
  - User explicitly rejected this approach ("I'm not going to need it")
  - Premature abstraction pattern
  - Superseded by pragmatic YAGNI approach in STYLE_GUIDE.md

---

### 7. docs/reports/ (empty except subdir)

**Structure**:
```
docs/reports/
└── production-readiness/ (empty)
```

**Action**: Delete empty directory
```bash
rm -rf docs/reports/
```

---

### 8. docs/wiki-templates/ (6 files, 56 KB)

**Files**:
- EPX-Credentials.md
- FAQ.md
- _Footer.md
- Home.md
- Quick-Start.md
- _Sidebar.md

**Purpose**: GitHub Wiki templates (outdated approach)

**Status**: Superseded by docs/integration/ guides

**Action**: Archive entire directory
```bash
git mv docs/wiki-templates docs/archive/wiki-templates
```

**Reason**:
- Information already in ADMIN_CLI.md, GETTING_STARTED.md, etc.
- Wiki approach abandoned in favor of /docs/ directory
- Keep for historical reference

---

## Cleanup Commands

### Phase 1: Move Certification Sheet to Root

```bash
git mv docs/certification_sheets.md CERTIFICATION_SHEET.md
```

### Phase 2: Archive Root-Level Analysis Files

```bash
# Create archive directory if needed
mkdir -p docs/archive/analysis

# Move root analysis files
git mv ANACHRONISTIC_CODE_AUDIT.md docs/archive/analysis/
git mv EPX_AUTH_RESP_TEXT_VALUES.md docs/archive/analysis/
git mv EPX_TOKENIZATION_ISSUE.md docs/archive/analysis/
```

### Phase 3: Archive Temporary Directories

```bash
# Move planning docs
git mv docs/planning/IMPLEMENTATION_PLAN_PHASE1.md docs/archive/planning/

# Move optimizations
git mv docs/optimizations docs/archive/

# Move wiki templates
git mv docs/wiki-templates docs/archive/

# Move refactor planning docs
git mv docs/refactor/CODE_CLEANLINESS_PLAN.md docs/archive/planning/
git mv docs/refactor/INTEGRATION_TEST_PLAN.md docs/archive/planning/
git mv docs/refactor/REFACTOR_PLAN.md docs/archive/planning/
git mv docs/refactor/TDD_REFACTOR_PLAN.md docs/archive/planning/
```

### Phase 4: Delete Flawed/Outdated Files

```bash
# Delete rejected domain refactoring plan
git rm docs/refactor/DOMAIN_REFACTORING_PLAN.md

# Delete empty reports directory
rm -rf docs/reports/

# Clean up empty planning directory
rmdir docs/planning/
```

### Phase 5: Handle CI/CD Docs

```bash
# CI/CD docs should be consolidated into development/
# User mentioned they'll handle docs/refactor/ manually
# Leave docs/refactor/cicd/ for now - user will decide
```

---

## Post-Cleanup Structure

```
project-root/
├── CERTIFICATION_SHEET.md          ✅ MOVED (from docs/)
├── CHANGELOG.md                     ✅ KEEP
├── README.md                        ✅ KEEP
│
├── docs/
│   ├── archive/                     📦 ARCHIVED DOCS
│   │   ├── analysis/
│   │   │   ├── ANACHRONISTIC_CODE_AUDIT.md
│   │   │   ├── EPX_AUTH_RESP_TEXT_VALUES.md
│   │   │   ├── EPX_TOKENIZATION_ISSUE.md
│   │   │   ├── BUSINESS_LOGIC_ANALYSIS.md
│   │   │   ├── CRITICAL_ISSUES_DISCOVERY.md
│   │   │   ├── OPTIMIZATION_COMPLETE.md
│   │   │   └── TABLE_USAGE_ANALYSIS.md
│   │   ├── completed-2025-11/
│   │   ├── optimizations/           (entire dir moved)
│   │   ├── planning/
│   │   │   ├── CODE_CLEANLINESS_PLAN.md
│   │   │   ├── IMPLEMENTATION_PLAN_PHASE1.md
│   │   │   ├── INTEGRATION_TEST_PLAN.md
│   │   │   ├── REFACTOR_PLAN.md
│   │   │   └── TDD_REFACTOR_PLAN.md
│   │   └── wiki-templates/          (entire dir moved)
│   │
│   ├── development/                 ✅ PERMANENT DOCS
│   │   ├── AUTH.md
│   │   ├── DATABASE.md
│   │   ├── ERROR_HANDLING.md
│   │   ├── STYLE_GUIDE.md
│   │   ├── TESTING_GUIDE.md
│   │   └── ...
│   │
│   ├── integration/                 ✅ PERMANENT DOCS
│   │   ├── ADMIN_CLI.md
│   │   ├── API_SPECS.md
│   │   ├── BROWSER_POST_FORM_SETUP.md
│   │   ├── EPX_API_REFERENCE.md
│   │   ├── GETTING_STARTED.md
│   │   └── ...
│   │
│   └── refactor/                    ⏸️ USER WILL HANDLE
│       └── cicd/                    (user mentioned handling manually)
```

---

## Directories to Delete

1. ✅ `docs/reports/` - Empty
2. ✅ `docs/planning/` - Will be empty after moving files

---

## Files to Delete

1. ✅ `docs/refactor/DOMAIN_REFACTORING_PLAN.md` - Rejected approach, flawed premise

---

## Preservation Rationale

### Why Archive vs Delete:

**Archived** (not deleted):
- Analysis documents: May need to reference findings
- Planning documents: Show decision-making process
- Optimization research: Valuable context for future work
- Wiki templates: Historical approach to documentation

**Deleted**:
- DOMAIN_REFACTORING_PLAN.md: User explicitly rejected ("I'm not going to need it")
- Empty directories: No value

---

## User Decisions Needed

1. **docs/refactor/cicd/** - User mentioned handling manually
   - Should we consolidate into docs/development/CICD.md?
   - Or archive entire docs/refactor/ directory?

2. **docs/analysis/** - Currently has 4 files (160 KB)
   - Keep in place or move to docs/archive/analysis/?
   - These are already organized

---

## Execution Order

1. ✅ Move CERTIFICATION_SHEET.md to root (high priority)
2. ✅ Archive root-level analysis files
3. ✅ Archive temporary directories (optimizations, wiki-templates, planning)
4. ✅ Delete rejected/flawed docs
5. ⏸️ Wait for user decision on docs/refactor/cicd/

---

## Estimated Impact

**Files Moved**: ~35 files
**Directories Archived**: 3 (optimizations, wiki-templates, planning)
**Directories Deleted**: 1 (reports)
**Files Deleted**: 1 (DOMAIN_REFACTORING_PLAN.md)

**Result**:
- Cleaner root directory
- Organized archive for historical reference
- Clear separation between permanent and temporary docs
- CERTIFICATION_SHEET.md prominently accessible

---

## Validation

After cleanup, verify:

```bash
# Check root files
ls -la *.md

# Expected: CERTIFICATION_SHEET.md, CHANGELOG.md, README.md

# Check permanent docs
ls docs/development/ docs/integration/

# Expected: All permanent guides present

# Check archive
ls docs/archive/

# Expected: analysis/, completed-2025-11/, optimizations/, planning/, wiki-templates/
```
