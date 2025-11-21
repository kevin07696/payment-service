#!/bin/bash

# Code Cleanliness Automation Script
# Automates repetitive cleanup tasks for the payment service
# Usage: ./scripts/cleanup-automation.sh [--dry-run] [--phase=<1|2|3|4|all>]

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
DRY_RUN=false
PHASE="all"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --phase=*)
            PHASE="${1#*=}"
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--dry-run] [--phase=<1|2|3|4|all>]"
            exit 1
            ;;
    esac
done

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

execute() {
    local cmd="$1"
    local desc="$2"

    if [ "$DRY_RUN" = true ]; then
        log_info "[DRY RUN] $desc"
        log_info "Would execute: $cmd"
    else
        log_info "$desc"
        if eval "$cmd"; then
            log_success "✓ $desc"
        else
            log_error "✗ Failed: $desc"
            return 1
        fi
    fi
}

# Phase 1: Quick Wins
phase1_quick_wins() {
    log_info "=== Phase 1: Quick Wins ==="

    # 1.1 Remove test binaries
    log_info "Removing test binaries..."
    if [ "$DRY_RUN" = true ]; then
        find "$PROJECT_ROOT" -name "*.test" -type f -print
    else
        find "$PROJECT_ROOT" -name "*.test" -type f -delete && \
            log_success "Removed test binaries" || \
            log_warning "No test binaries found"
    fi

    # 1.2 Check .gitignore has *.test
    if ! grep -q "^\*.test$" "$PROJECT_ROOT/.gitignore" 2>/dev/null; then
        execute "echo '*.test' >> $PROJECT_ROOT/.gitignore" \
                "Adding *.test to .gitignore"
    else
        log_success "✓ .gitignore already includes *.test"
    fi

    # 1.3 Create templates directory structure
    execute "mkdir -p $PROJECT_ROOT/internal/templates/browser_post" \
            "Creating templates directory"

    # 1.4 Create docs archive directory
    execute "mkdir -p $PROJECT_ROOT/docs/archive/2024-11-planning" \
            "Creating docs archive directory"

    log_success "Phase 1 completed!"
}

# Phase 2: TODO Extraction
phase2_todo_extraction() {
    log_info "=== Phase 2: TODO Extraction ==="

    local todo_file="$PROJECT_ROOT/docs/TODO_INVENTORY.md"

    log_info "Extracting TODOs from codebase..."

    if [ "$DRY_RUN" = false ]; then
        cat > "$todo_file" <<EOF
# TODO Inventory

**Generated:** $(date +%Y-%m-%d)
**Total TODOs Found:** $(grep -r "TODO\|FIXME\|XXX\|HACK" --include="*.go" "$PROJECT_ROOT" | wc -l)

## By Category

### High Priority - Unimplemented Features

EOF

        # Extract high-priority TODOs
        grep -rn "TODO.*[Ii]mplement\|TODO.*[Aa]udit" --include="*.go" "$PROJECT_ROOT" >> "$todo_file" || true

        cat >> "$todo_file" <<EOF

### Medium Priority - Test Updates

EOF
        grep -rn "TODO.*[Tt]est\|TODO.*RPC" --include="*.go" "$PROJECT_ROOT" >> "$todo_file" || true

        cat >> "$todo_file" <<EOF

### All TODOs (Full List)

EOF
        grep -rn "TODO\|FIXME\|XXX\|HACK" --include="*.go" "$PROJECT_ROOT" >> "$todo_file" || true

        log_success "TODO inventory created at $todo_file"
    else
        log_info "[DRY RUN] Would create TODO inventory at $todo_file"
    fi
}

# Phase 3: Documentation Consolidation
phase3_docs_consolidation() {
    log_info "=== Phase 3: Documentation Consolidation ==="

    # Archive old planning docs
    local docs_to_archive=(
        "AUTH-IMPLEMENTATION-PLAN.md"
        "AUTH-IMPROVEMENT-PLAN.md"
        "CONNECTRPC_DEPLOYMENT_READY.md"
        "CONNECTRPC_MIGRATION_GUIDE.md"
        "DEPLOYMENT_PLAN.md"
        "REFACTORING_ANALYSIS.md"
        "REFACTOR_PLAN.md"
        "TDD_REFACTOR_PLAN.md"
        "UNIT_TEST_REFACTORING_ANALYSIS.md"
    )

    for doc in "${docs_to_archive[@]}"; do
        if [ -f "$PROJECT_ROOT/docs/$doc" ]; then
            execute "mv $PROJECT_ROOT/docs/$doc $PROJECT_ROOT/docs/archive/2024-11-planning/" \
                    "Archiving $doc"
        fi
    done

    # Create documentation index
    local readme="$PROJECT_ROOT/docs/README.md"

    if [ "$DRY_RUN" = false ]; then
        cat > "$readme" <<EOF
# Payment Service Documentation

**Last Updated:** $(date +%Y-%m-%d)

## Quick Start

- [Setup Guide](SETUP.md) - Local development setup
- [Development Guide](DEVELOP.md) - Development workflow
- [Integration Guide](INTEGRATION_GUIDE.md) - How to integrate with the service

## API Documentation

- [API Specifications](API_SPECS.md) - Complete API reference
- [Browser Post Reference](BROWSER_POST_REFERENCE.md) - Browser Post implementation
- [EPX API Reference](EPX_API_REFERENCE.md) - EPX gateway documentation

## Architecture & Design

- [Authentication](AUTH.md) - JWT-based authentication system
- [Database Schema](DATABASE.md) - PostgreSQL schema and migrations
- [Data Flow](DATAFLOW.md) - How data flows through the system

## Business Logic

- [Credit Card Processing](CREDIT_CARD_BUSINESS_LOGIC.md)
- [ACH Processing](ACH_BUSINESS_LOGIC.md)
- [ACH Verification](ACH_SAFE_VERIFICATION_IMPLEMENTATION.md)

## Operations

- [CI/CD Pipeline](CICD.md) - GitHub Actions workflows
- [Production Setup](GCP_PRODUCTION_SETUP.md) - GCP deployment

## Testing

- [Integration Test Plan](INTEGRATION_TEST_PLAN.md)
- [Integration Test Strategy](INTEGRATION_TEST_STRATEGY.md)

## Performance & Optimization

See [optimizations/README.md](optimizations/README.md) for comprehensive optimization documentation.

## Research & Analysis

- [3D Secure Provider Research](3DS_PROVIDER_RESEARCH.md)

## Archive

Historical planning and migration documents are in [archive/](archive/).
EOF
        log_success "Created documentation index at $readme"
    else
        log_info "[DRY RUN] Would create documentation index at $readme"
    fi
}

# Phase 4: Code Quality Checks
phase4_quality_checks() {
    log_info "=== Phase 4: Code Quality Checks ==="

    cd "$PROJECT_ROOT"

    # Run go vet
    log_info "Running go vet..."
    if go vet ./... 2>&1 | tee /tmp/vet-output.txt; then
        log_success "✓ go vet passed"
    else
        log_error "✗ go vet found issues:"
        cat /tmp/vet-output.txt
    fi

    # Run go build
    log_info "Running go build..."
    if go build ./... 2>&1; then
        log_success "✓ go build passed"
    else
        log_error "✗ go build failed"
    fi

    # Check if staticcheck is available
    if command -v staticcheck &> /dev/null; then
        log_info "Running staticcheck..."
        if staticcheck ./... 2>&1 | tee /tmp/staticcheck-output.txt; then
            log_success "✓ staticcheck passed"
        else
            log_error "✗ staticcheck found issues:"
            cat /tmp/staticcheck-output.txt
        fi
    else
        log_warning "staticcheck not installed, skipping..."
    fi

    # Check if golangci-lint is available
    if command -v golangci-lint &> /dev/null; then
        log_info "Running golangci-lint..."
        if golangci-lint run ./... 2>&1; then
            log_success "✓ golangci-lint passed"
        else
            log_warning "golangci-lint found some issues"
        fi
    else
        log_warning "golangci-lint not installed, skipping..."
    fi

    # Check for large files
    log_info "Checking for large Go files (>500 lines)..."
    find "$PROJECT_ROOT" -name "*.go" -type f ! -path "*/proto/*" -exec sh -c '
        lines=$(wc -l < "$1")
        if [ "$lines" -gt 500 ]; then
            echo "$1: $lines lines"
        fi
    ' _ {} \; | sort -t: -k2 -rn > /tmp/large-files.txt

    if [ -s /tmp/large-files.txt ]; then
        log_warning "Large files found (>500 lines):"
        cat /tmp/large-files.txt
    else
        log_success "✓ No excessively large files found"
    fi

    # Count TODO/FIXME/HACK comments
    local todo_count=$(grep -r "TODO\|FIXME\|XXX\|HACK" --include="*.go" "$PROJECT_ROOT" | wc -l)
    log_info "Found $todo_count TODO/FIXME/HACK comments"

    log_success "Phase 4 quality checks completed!"
}

# Generate summary report
generate_report() {
    local report_file="$PROJECT_ROOT/CLEANUP_REPORT.md"

    log_info "Generating cleanup report..."

    cat > "$report_file" <<EOF
# Code Cleanup Report

**Generated:** $(date +%Y-%m-%d\ %H:%M:%S)
**Mode:** $([ "$DRY_RUN" = true ] && echo "DRY RUN" || echo "EXECUTED")

## Summary

- Phase 1: Quick Wins - $([ "$DRY_RUN" = true ] && echo "Simulated" || echo "Completed")
- Phase 2: TODO Extraction - $([ "$DRY_RUN" = true ] && echo "Simulated" || echo "Completed")
- Phase 3: Documentation Consolidation - $([ "$DRY_RUN" = true ] && echo "Simulated" || echo "Completed")
- Phase 4: Quality Checks - $([ "$DRY_RUN" = true ] && echo "Simulated" || echo "Completed")

## Files Created

- docs/refactor/CODE_CLEANLINESS_PLAN.md - Detailed refactoring plan
- docs/TODO_INVENTORY.md - Complete TODO inventory
- docs/README.md - Documentation index
- internal/templates/ - Template directory structure

## Next Steps

1. Review CODE_CLEANLINESS_PLAN.md for detailed refactoring strategy
2. Address high-priority TODOs in TODO_INVENTORY.md
3. Begin Phase 2 refactoring (large service files)
4. Update CHANGELOG.md with cleanup progress

## Statistics

- TODO Comments: $(grep -r "TODO\|FIXME\|XXX\|HACK" --include="*.go" "$PROJECT_ROOT" 2>/dev/null | wc -l || echo "N/A")
- Large Files (>500 lines): $(find "$PROJECT_ROOT" -name "*.go" -type f ! -path "*/proto/*" -exec sh -c 'wc -l "$1" | awk "{if (\$1 > 500) print \$1}"' _ {} \; 2>/dev/null | wc -l || echo "N/A")
- Documentation Files: $(find "$PROJECT_ROOT/docs" -name "*.md" -type f 2>/dev/null | wc -l || echo "N/A")

EOF

    log_success "Cleanup report generated at $report_file"
}

# Main execution
main() {
    log_info "Starting Code Cleanup Automation"
    log_info "Project Root: $PROJECT_ROOT"
    log_info "Dry Run: $DRY_RUN"
    log_info "Phase: $PHASE"
    echo ""

    case "$PHASE" in
        1)
            phase1_quick_wins
            ;;
        2)
            phase2_todo_extraction
            ;;
        3)
            phase3_docs_consolidation
            ;;
        4)
            phase4_quality_checks
            ;;
        all)
            phase1_quick_wins
            echo ""
            phase2_todo_extraction
            echo ""
            phase3_docs_consolidation
            echo ""
            phase4_quality_checks
            ;;
        *)
            log_error "Invalid phase: $PHASE"
            echo "Valid phases: 1, 2, 3, 4, all"
            exit 1
            ;;
    esac

    echo ""
    generate_report
    echo ""
    log_success "Cleanup automation completed!"

    if [ "$DRY_RUN" = true ]; then
        log_warning "This was a DRY RUN - no changes were made"
        log_info "Run without --dry-run to apply changes"
    fi
}

# Run main
main
