# Documentation Organization Plan

**Target Audience:** Documentation maintainers
**Topic:** Organizing documentation into integration/ and development/ directories
**Goal:** Create a clean, intuitive documentation structure for external and internal audiences

---

## Directory Purpose

### `docs/integration/`
**Audience:** External developers, DevOps engineers, other projects integrating with payment-service
**Focus:** How to USE the service
- API specifications and references
- Integration guides (React, browser-based, etc.)
- Admin CLI for setup
- Quick start guides
- Token generation and authentication

### `docs/development/`
**Audience:** Internal developers working on payment-service codebase
**Focus:** How to BUILD and MAINTAIN the service
- Setup and development environment
- Architecture decisions
- Database schema and migrations
- Testing strategies
- CI/CD and deployment
- Business logic documentation

---

## Current Documentation Analysis

### ✅ KEEP IN `integration/` (External Integration Guides)

| Document | Status | Reason |
|----------|--------|--------|
| `ADMIN_CLI.md` | ✅ Keep | External tool for setting up services/merchants |
| `API_SPECS.md` | ✅ Keep | Hand-written API documentation |
| `API_SPECS_GENERATED.md` | ✅ Keep | Auto-generated API reference |
| `BROWSER_POST_REFERENCE.md` | ✅ Keep | PCI-compliant integration guide |
| `EPX_API_REFERENCE.md` | ✅ Keep | EPX payment gateway reference |
| `REACT_INTEGRATION.md` | ✅ Keep | React/TypeScript integration guide |
| `TOKEN_GENERATION.md` | ✅ Keep | Tokenization implementation guide |

### ⚠️ REVIEW - Currently in `integration/` but might belong in `development/`

| Document | Current Location | Recommendation | Reason |
|----------|-----------------|----------------|--------|
| `DATABASE.md` | integration/ | **MOVE to development/** | Database is internal implementation detail |
| `DATAFLOW.md` | integration/ | **MOVE to development/** | Internal architecture, not integration guide |
| `INTEGRATION_GUIDE.md` | integration/ | **REVIEW** | Check if this duplicates other integration docs |
| `MODULE_INTEGRATION.md` | integration/ | **MOVE to development/** | Internal module structure |

### ✅ KEEP IN `development/` (Internal Development Docs)

| Document | Status | Reason |
|----------|--------|--------|
| `AUTH.md` | ✅ Keep | Internal authentication architecture |
| `CICD.md` | ✅ Keep | CI/CD pipeline for maintainers |
| `DATABASE_SCHEMA.md` | ✅ Keep | Auto-generated schema reference |
| `DEVELOP.md` | ✅ Keep | Development workflow guide |
| `GCP_PRODUCTION_SETUP.md` | ✅ Keep | Production deployment for maintainers |
| `SETUP.md` | ✅ Keep | Local development setup |

### 🗑️ ARCHIVE - Point-in-time analysis docs (Move to CHANGELOG.md or delete)

| Document | Recommendation | Reason |
|----------|---------------|--------|
| `3DS_PROVIDER_RESEARCH.md` | **Summarize in CHANGELOG** | Research doc, decision should be in code/CHANGELOG |
| `ACH_BUSINESS_LOGIC.md` | **Keep for now** | Business logic reference is valuable |
| `CONNECTRPC_TESTING.md` | **Summarize in CHANGELOG** | Migration complete, testing strategy in code |
| `CREDIT_CARD_BUSINESS_LOGIC.md` | **Keep for now** | Business logic reference is valuable |
| `CUSTOMER_ID_MIGRATION_PLAN.md` | **Summarize in CHANGELOG** | Migration plan, not ongoing reference |
| `E2E_TEST_DESIGN.md` | **Review - might keep** | If test strategy is still relevant |
| `E2E_VS_INTEGRATION_ANALYSIS.md` | **Summarize in CHANGELOG** | Analysis doc, decision made |
| `INTEGRATION_TEST_STRATEGY.md` | **Keep if current** | Ongoing testing strategy |
| `REFACTORING_ANALYSIS.md` | **Summarize in CHANGELOG** | Analysis doc, refactoring complete |
| `REST_VS_CONNECTRPC_ARCHITECTURE.md` | **Summarize in CHANGELOG** | Decision made, using ConnectRPC |
| `SUBSCRIPTION_SERVICE_TESTABILITY.md` | **Review** | If still relevant to testing |
| `TODO_COMPLETION_SUMMARY.md` | **DELETE** | Point-in-time summary, not needed |
| `UNIT_TEST_REFACTORING_ANALYSIS.md` | **Summarize in CHANGELOG** | Refactoring complete |
| `WIKI_SETUP.md` | **DELETE** | Wiki is set up, automation handles it now |

---

## Proposed Final Structure

### `docs/integration/` (7-8 documents)

**Quick Start & Setup:**
- `ADMIN_CLI.md` - Service/merchant setup tool
- *(NEW)* `QUICK_START.md` - Getting started guide

**Integration Guides:**
- `BROWSER_POST_REFERENCE.md` - Browser-based tokenization
- `REACT_INTEGRATION.md` - React/TypeScript integration
- `TOKEN_GENERATION.md` - Token generation

**API References:**
- `API_SPECS.md` - Hand-written API documentation
- `API_SPECS_GENERATED.md` - Auto-generated from proto (gitignored)
- `EPX_API_REFERENCE.md` - EPX gateway reference

### `docs/development/` (8-10 documents)

**Getting Started:**
- `SETUP.md` - Local development environment
- `DEVELOP.md` - Development workflow

**Architecture & Design:**
- `AUTH.md` - Authentication architecture
- `DATABASE.md` - Database design (moved from integration/)
- `DATABASE_SCHEMA.md` - Auto-generated schema (gitignored)
- `DATAFLOW.md` - System dataflow (moved from integration/)

**Business Logic:**
- `ACH_BUSINESS_LOGIC.md` - ACH payment logic
- `CREDIT_CARD_BUSINESS_LOGIC.md` - Credit card logic

**Operations:**
- `CICD.md` - CI/CD pipeline
- `GCP_PRODUCTION_SETUP.md` - Production deployment

**Testing (if keeping):**
- `INTEGRATION_TEST_STRATEGY.md` - Testing approach
- `E2E_TEST_DESIGN.md` - E2E test design

---

## Migration Tasks

### Phase 1: Move docs between directories
- [ ] Move `DATABASE.md` from integration/ to development/
- [ ] Move `DATAFLOW.md` from integration/ to development/
- [ ] Move `MODULE_INTEGRATION.md` from integration/ to development/
- [ ] Review `INTEGRATION_GUIDE.md` for duplication

### Phase 2: Archive/Summarize point-in-time docs
- [ ] Review each "ARCHIVE" doc above
- [ ] Summarize key decisions in CHANGELOG.md
- [ ] Delete obsolete docs
- [ ] Update any broken links

### Phase 3: Create missing docs
- [ ] Create `QUICK_START.md` in integration/ if needed
- [ ] Consolidate any duplicate integration guides

### Phase 4: Update all cross-references
- [ ] Fix links in README.md
- [ ] Fix links in style guide
- [ ] Fix links in wiki sync script
- [ ] Update sidebar in wiki action

---

## Decision Criteria

**Keep in integration/ if:**
- ✅ External developers need it to integrate
- ✅ Used by other projects/teams
- ✅ API reference or usage guide
- ✅ No internal implementation details

**Keep in development/ if:**
- ✅ Internal developers need it to maintain codebase
- ✅ Architecture or design decisions
- ✅ Development workflow or setup
- ✅ Implementation details

**Archive/Delete if:**
- ✅ Point-in-time analysis (migration plans, comparisons)
- ✅ Decision already made and implemented
- ✅ Information duplicated elsewhere
- ✅ Superseded by newer documentation

---

## Questions for Review

1. **Business Logic Docs:** Should `ACH_BUSINESS_LOGIC.md` and `CREDIT_CARD_BUSINESS_LOGIC.md` stay in development/, or are they reference docs that external developers might need?

2. **Integration Guide:** Is `INTEGRATION_GUIDE.md` duplicating content from the other integration guides? Can it be consolidated?

3. **Testing Docs:** Which testing strategy docs are still relevant vs point-in-time analysis?

4. **Quick Start:** Do we need a separate `QUICK_START.md` in integration/, or does README.md + ADMIN_CLI.md cover it?

5. **Auto-generated Docs:** Should auto-generated docs have the required headers (Target Audience, Topic, Goal) added to the generators?
