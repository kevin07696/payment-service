#!/bin/bash
# install_git_hooks.sh - Install git hooks for documentation validation
# Usage: ./scripts/install_git_hooks.sh

set -e

echo "🔧 Installing git hooks..."

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "❌ Error: Not in a git repository"
    exit 1
fi

GIT_HOOKS_DIR=".git/hooks"

# Create hooks directory if it doesn't exist
mkdir -p "$GIT_HOOKS_DIR"

# 1. Pre-commit hook for documentation validation
echo "📝 Installing pre-commit hook..."
cat > "$GIT_HOOKS_DIR/pre-commit" <<'EOF'
#!/bin/bash
# Pre-commit hook: Validate documentation before commit

# Only run if documentation files are being committed
if git diff --cached --name-only | grep -qE '^docs/'; then
    echo "📚 Documentation files detected, running validation..."
    echo ""

    # Run validation script
    if [ -f "./scripts/validate_docs.sh" ]; then
        if ! ./scripts/validate_docs.sh; then
            echo ""
            echo "❌ Documentation validation failed!"
            echo ""
            echo "Options:"
            echo "  1. Fix the errors above"
            echo "  2. Skip validation: git commit --no-verify"
            echo ""
            exit 1
        fi
    else
        echo "⚠️  Warning: validate_docs.sh not found, skipping validation"
    fi

    echo ""
    echo "✅ Documentation validation passed!"
    echo ""
fi

# Continue with commit
exit 0
EOF

chmod +x "$GIT_HOOKS_DIR/pre-commit"
echo "✅ Pre-commit hook installed"

# 2. Pre-push hook for regenerating docs
echo "📝 Installing pre-push hook..."
cat > "$GIT_HOOKS_DIR/pre-push" <<'EOF'
#!/bin/bash
# Pre-push hook: Remind to regenerate documentation

# Check if proto files or migrations changed
if git diff --name-only @{u}..HEAD 2>/dev/null | grep -qE '(proto/.*\.proto|internal/db/migrations/.*\.sql)'; then
    echo ""
    echo "⚠️  Proto files or migrations changed!"
    echo ""
    echo "Did you regenerate documentation?"
    echo "  Run: make docs"
    echo ""
    echo "Continue push? (y/n)"
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        echo "Push cancelled. Run 'make docs' and try again."
        exit 1
    fi
fi

exit 0
EOF

chmod +x "$GIT_HOOKS_DIR/pre-push"
echo "✅ Pre-push hook installed"

# 3. Post-commit hook for auto-generation reminder
echo "📝 Installing post-commit hook..."
cat > "$GIT_HOOKS_DIR/post-commit" <<'EOF'
#!/bin/bash
# Post-commit hook: Remind about documentation sync

# Check if documentation was modified in this commit
if git diff-tree --no-commit-id --name-only -r HEAD | grep -qE '^docs/'; then
    echo ""
    echo "📚 Documentation was modified in this commit"
    echo ""
    echo "💡 Reminder: Documentation will auto-sync to wiki when pushed to main"
    echo "   Or manually sync: make docs-sync-wiki"
    echo ""
fi

exit 0
EOF

chmod +x "$GIT_HOOKS_DIR/post-commit"
echo "✅ Post-commit hook installed"

echo ""
echo "=================================="
echo "✅ Git hooks installed successfully!"
echo "=================================="
echo ""
echo "Installed hooks:"
echo "  • pre-commit   - Validates documentation before commit"
echo "  • pre-push     - Reminds to regenerate docs if proto/migrations changed"
echo "  • post-commit  - Reminds about wiki sync"
echo ""
echo "To bypass hooks (use sparingly):"
echo "  git commit --no-verify"
echo "  git push --no-verify"
echo ""
