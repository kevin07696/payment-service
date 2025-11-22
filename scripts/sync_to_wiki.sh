#!/bin/bash
# sync_to_wiki.sh - Syncs documentation to GitHub wiki
# Usage: ./scripts/sync_to_wiki.sh

set -e

echo "📚 Syncing documentation to GitHub wiki..."

# Configuration
REPO_URL="https://github.com/kevin07696/payment-service.wiki.git"
WIKI_DIR="/tmp/payment-service-wiki-$$"
DOCS_INTEGRATION="docs/integration"
DOCS_DEVELOPMENT="docs/development"

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "❌ Error: Not in a git repository"
    exit 1
fi

# Get current git commit for reference
CURRENT_COMMIT=$(git rev-parse --short HEAD)
CURRENT_DATE=$(date '+%Y-%m-%d %H:%M:%S')

echo "Current commit: $CURRENT_COMMIT"
echo "Wiki will be cloned to: $WIKI_DIR"
echo ""

# Clone wiki repository
echo "📥 Cloning wiki repository..."
if ! git clone "$REPO_URL" "$WIKI_DIR" 2>/dev/null; then
    echo "⚠️  Warning: Could not clone wiki repository."
    echo "   Make sure the wiki has been initialized on GitHub."
    echo "   Go to: https://github.com/kevin07696/payment-service/wiki"
    echo "   Click 'Create the first page' to initialize the wiki."
    exit 1
fi

echo "✅ Wiki repository cloned"
echo ""

# Copy integration documentation
echo "📋 Copying integration documentation..."
if [ -d "$DOCS_INTEGRATION" ]; then
    for file in "$DOCS_INTEGRATION"/*.md; do
        [ -f "$file" ] || continue
        filename=$(basename "$file")
        # Skip generated files (will be regenerated)
        if [[ "$filename" == *"_GENERATED.md" ]]; then
            continue
        fi
        cp "$file" "$WIKI_DIR/$filename"
        echo "   Copied: $filename"
    done
    echo "✅ Integration docs copied"
else
    echo "⚠️  Integration docs directory not found: $DOCS_INTEGRATION"
fi

echo ""

# Copy development documentation
echo "📋 Copying development documentation..."
if [ -d "$DOCS_DEVELOPMENT" ]; then
    for file in "$DOCS_DEVELOPMENT"/*.md; do
        [ -f "$file" ] || continue
        filename=$(basename "$file")
        # Prefix development docs to avoid naming conflicts
        cp "$file" "$WIKI_DIR/Dev-$filename"
        echo "   Copied: Dev-$filename"
    done
    echo "✅ Development docs copied"
else
    echo "⚠️  Development docs directory not found: $DOCS_DEVELOPMENT"
fi

echo ""

# Copy README as Home page
echo "📋 Copying README.md as Home.md..."
if [ -f "README.md" ]; then
    cp README.md "$WIKI_DIR/Home.md"
    echo "✅ README copied as Home.md"
else
    echo "⚠️  README.md not found"
fi

echo ""

# Create wiki sidebar
echo "📋 Creating wiki sidebar..."
cat > "$WIKI_DIR/_Sidebar.md" <<EOF
# Payment Service Wiki

## 🚀 Getting Started
- [Home](Home)
- [Quick Start](Home#quick-start)

## 📖 Integration Guides
- [Admin CLI](ADMIN_CLI)
- [React Integration](REACT_INTEGRATION)
- [API Specifications](API_SPECS)
- [Browser Post](BROWSER_POST_REFERENCE)
- [Token Generation](TOKEN_GENERATION)

## 🛠️ Development
- [Authentication](Dev-AUTH)
- [Setup](Dev-SETUP)
- [Development Guide](Dev-DEVELOP)
- [Database](Dev-DATABASE)

## 📚 Reference
- [Style Guide](DOCUMENTATION_STYLE_GUIDE)
- [Changelog](https://github.com/kevin07696/payment-service/blob/main/CHANGELOG.md)
EOF

echo "✅ Sidebar created"
echo ""

# Create wiki footer
echo "📋 Creating wiki footer..."
cat > "$WIKI_DIR/_Footer.md" <<EOF
---
**Last synced:** $CURRENT_DATE | **Commit:** [\`$CURRENT_COMMIT\`](https://github.com/kevin07696/payment-service/commit/$CURRENT_COMMIT)

[📖 View on GitHub](https://github.com/kevin07696/payment-service) | [🐛 Report Issue](https://github.com/kevin07696/payment-service/issues)
EOF

echo "✅ Footer created"
echo ""

# Commit and push changes
echo "📤 Committing and pushing to wiki..."
cd "$WIKI_DIR"

git config user.name "Documentation Bot"
git config user.email "docs@payment-service"

git add .

if git diff --cached --quiet; then
    echo "ℹ️  No changes to commit"
else
    git commit -m "Sync documentation from main branch

Commit: $CURRENT_COMMIT
Date: $CURRENT_DATE

Auto-synced by scripts/sync_to_wiki.sh"

    if git push origin master; then
        echo "✅ Documentation pushed to wiki successfully!"
    else
        echo "❌ Failed to push to wiki"
        cd -
        rm -rf "$WIKI_DIR"
        exit 1
    fi
fi

# Cleanup
cd -
rm -rf "$WIKI_DIR"

echo ""
echo "=================================="
echo "✅ Wiki sync complete!"
echo "=================================="
echo ""
echo "View your wiki at:"
echo "https://github.com/kevin07696/payment-service/wiki"
echo ""
