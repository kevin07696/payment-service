#!/bin/bash
set -e

WIKI_DIR="../payment-service.wiki"
DOCS_DIR="./docs"

echo "🔄 Syncing documentation to GitHub Wiki..."

# Check if wiki directory exists
if [ ! -d "$WIKI_DIR" ]; then
    echo "❌ Wiki directory not found: $WIKI_DIR"
    echo ""
    echo "Clone the wiki first:"
    echo "  cd .."
    echo "  git clone https://github.com/kevin07696/payment-service.wiki.git"
    echo ""
    exit 1
fi

# Copy documentation files
echo "📋 Copying documentation files..."
cp "$DOCS_DIR/DATAFLOW.md" "$WIKI_DIR/"
cp "$DOCS_DIR/DEVELOP.md" "$WIKI_DIR/"
cp "$DOCS_DIR/API_SPECS.md" "$WIKI_DIR/API-Specs.md"  # Rename for wiki
cp "$DOCS_DIR/CICD.md" "$WIKI_DIR/"
cp "$DOCS_DIR/DATABASE.md" "$WIKI_DIR/"
cp "$DOCS_DIR/AUTH.md" "$WIKI_DIR/"

# Navigate to wiki directory
cd "$WIKI_DIR"

# Check for changes
if [ -z "$(git status --porcelain)" ]; then
    echo "✅ No changes to sync"
    exit 0
fi

# Show what changed
echo ""
echo "📝 Changes detected:"
git status --short

# Commit and push
echo ""
echo "📤 Pushing changes to wiki..."
git add .
git commit -m "docs: Sync documentation from main repo ($(date +%Y-%m-%d))"
git push origin master

echo ""
echo "✅ Documentation synced successfully!"
echo "🌐 View at: https://github.com/kevin07696/payment-service/wiki"