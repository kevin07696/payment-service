#!/bin/bash
# validate_docs.sh - Validates documentation for common issues
# Usage: ./scripts/validate_docs.sh

set -e

echo "🔍 Validating documentation..."
echo ""

ERRORS=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print error
error() {
    echo -e "${RED}❌ $1${NC}"
    ERRORS=$((ERRORS + 1))
}

# Function to print warning
warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Function to print success
success() {
    echo -e "${GREEN}✅ $1${NC}"
}

# 1. Check for required headers in integration and development docs
echo "📝 Checking for required headers..."
for doc in docs/integration/*.md docs/development/*.md; do
    # Skip if file doesn't exist
    [ -f "$doc" ] || continue

    # Skip certain files
    basename=$(basename "$doc")
    if [[ "$basename" == "README.md" ]]; then
        continue
    fi

    # Check for required headers
    if ! grep -q "^\*\*Target Audience:\*\*" "$doc"; then
        error "Missing 'Target Audience:' header in $doc"
    fi
    if ! grep -q "^\*\*Topic:\*\*" "$doc"; then
        error "Missing 'Topic:' header in $doc"
    fi
    if ! grep -q "^\*\*Goal:\*\*" "$doc"; then
        error "Missing 'Goal:' header in $doc"
    fi
done

if [ $ERRORS -eq 0 ]; then
    success "All integration/development docs have required headers"
fi

# 2. Check for TODO/FIXME in documentation
echo ""
echo "🔍 Checking for TODOs and FIXMEs..."
TODO_COUNT=0
if grep -r "TODO\|FIXME" docs/integration docs/development 2>/dev/null; then
    TODO_COUNT=$(grep -r "TODO\|FIXME" docs/integration docs/development 2>/dev/null | wc -l)
    warn "Found $TODO_COUNT TODOs/FIXMEs in documentation (should be resolved before release)"
else
    success "No TODOs or FIXMEs found in documentation"
fi

# 3. Check for broken internal links
echo ""
echo "🔗 Checking for broken internal links..."
BROKEN_LINKS=0

for doc in docs/**/*.md; do
    [ -f "$doc" ] || continue

    # Extract markdown links [text](path)
    grep -o '\[.*\](.*\.md)' "$doc" 2>/dev/null | while read -r link; do
        # Extract path from link
        path=$(echo "$link" | sed 's/.*](\(.*\))/\1/')

        # Skip external links
        if [[ "$path" =~ ^http ]]; then
            continue
        fi

        # Resolve relative path
        doc_dir=$(dirname "$doc")
        full_path="$doc_dir/$path"

        # Normalize path
        full_path=$(realpath -m "$full_path" 2>/dev/null || echo "$full_path")

        # Check if file exists
        if [ ! -f "$full_path" ]; then
            error "Broken link in $doc: $path (resolved to $full_path)"
            BROKEN_LINKS=$((BROKEN_LINKS + 1))
        fi
    done
done

if [ $BROKEN_LINKS -eq 0 ]; then
    success "No broken internal links found"
fi

# 4. Check for dangerous commands in code blocks
echo ""
echo "⚠️  Checking for dangerous commands..."
DANGEROUS_COMMANDS=0

if grep -r "rm -rf /" docs/integration docs/development 2>/dev/null; then
    error "Found dangerous 'rm -rf /' command in documentation"
    DANGEROUS_COMMANDS=$((DANGEROUS_COMMANDS + 1))
fi

if [ $DANGEROUS_COMMANDS -eq 0 ]; then
    success "No dangerous commands found"
fi

# 5. Check for consistent code block syntax
echo ""
echo "📋 Checking code block syntax..."
UNCLOSED_BLOCKS=0

for doc in docs/**/*.md; do
    [ -f "$doc" ] || continue

    # Count opening and closing code blocks
    OPEN=$(grep -c '^```' "$doc" 2>/dev/null || echo "0")
    OPEN=$(echo "$OPEN" | tr -d '\n\r' | xargs)

    # Code blocks should come in pairs
    if [ "$OPEN" != "0" ] && [ $((OPEN % 2)) -ne 0 ]; then
        error "Unclosed code block in $doc (found $OPEN backtick lines)"
        UNCLOSED_BLOCKS=$((UNCLOSED_BLOCKS + 1))
    fi
done

if [ $UNCLOSED_BLOCKS -eq 0 ]; then
    success "All code blocks properly closed"
fi

# 6. Check for large files that might need splitting
echo ""
echo "📏 Checking for large documentation files..."
LARGE_FILES=0

for doc in docs/**/*.md; do
    [ -f "$doc" ] || continue

    # Get file size in lines
    LINES=$(wc -l < "$doc")

    if [ "$LINES" -gt 1000 ]; then
        warn "$doc is $LINES lines (consider splitting if over 1000)"
        LARGE_FILES=$((LARGE_FILES + 1))
    fi
done

if [ $LARGE_FILES -eq 0 ]; then
    success "No excessively large files found"
fi

# 7. Check for consistent header capitalization
echo ""
echo "🔤 Checking header capitalization..."
INCONSISTENT_HEADERS=0

for doc in docs/**/*.md; do
    [ -f "$doc" ] || continue

    # Check for headers starting with lowercase (except code references)
    if grep -E '^#+\s+[a-z]' "$doc" 2>/dev/null | grep -v '`' > /dev/null; then
        warn "$doc has headers starting with lowercase letters"
        INCONSISTENT_HEADERS=$((INCONSISTENT_HEADERS + 1))
    fi
done

if [ $INCONSISTENT_HEADERS -eq 0 ]; then
    success "Headers consistently capitalized"
fi

# Summary
echo ""
echo "=================================="
echo "Documentation Validation Summary"
echo "=================================="

if [ $ERRORS -eq 0 ]; then
    success "All critical checks passed!"
    echo ""
    if [ $TODO_COUNT -gt 0 ]; then
        warn "Note: $TODO_COUNT TODOs/FIXMEs found (non-critical)"
    fi
    if [ $LARGE_FILES -gt 0 ]; then
        warn "Note: $LARGE_FILES large files found (consider splitting)"
    fi
    exit 0
else
    error "Found $ERRORS critical issue(s)"
    echo ""
    echo "Please fix the errors above before committing documentation."
    exit 1
fi
