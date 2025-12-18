#!/bin/bash
# Generate ephemeral test service RSA keys for integration tests
#
# SECURITY: This script generates keys dynamically to avoid committing
# private keys to version control. Keys are stored in tests/fixtures/auth/
# which is in .gitignore.
#
# USAGE:
#   ./scripts/generate_test_keys.sh
#
# OUTPUT:
#   tests/fixtures/auth/test_services.json

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
readonly OUTPUT_DIR="$PROJECT_ROOT/tests/fixtures/auth"
readonly OUTPUT_FILE="$OUTPUT_DIR/test_services.json"

# Colors for output
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m' # No Color

echo "🔐 Generating ephemeral test service RSA keys..."

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Check if file already exists
if [ -f "$OUTPUT_FILE" ]; then
    echo -e "${YELLOW}⚠️  Test keys already exist at: $OUTPUT_FILE${NC}"
    read -p "Regenerate keys? This will invalidate existing tokens. (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "✅ Using existing test keys"
        exit 0
    fi
fi

# Build and run the Go key generator
echo "🔨 Building key generator..."
go build -o /tmp/generate_test_keys "$SCRIPT_DIR/generate_test_keys/generate_test_keys.go"

echo "🔑 Generating RSA key pairs..."
/tmp/generate_test_keys -output "$OUTPUT_FILE"

# Clean up temporary binary
rm -f /tmp/generate_test_keys

# Verify the file was created
if [ ! -f "$OUTPUT_FILE" ]; then
    echo "❌ ERROR: Failed to generate test keys"
    exit 1
fi

echo -e "${GREEN}✅ Test service keys generated successfully!${NC}"
echo "📁 Location: $OUTPUT_FILE"
echo ""

# Seed test services into database if DATABASE_URL is set
if [ -n "${DATABASE_URL:-}" ]; then
    echo "🔄 Seeding test services into database..."

    # Build seed program
    go build -o /tmp/seed_test_services "$SCRIPT_DIR/seed_test_services/seed_test_services.go"

    # Seed services
    /tmp/seed_test_services -input "$OUTPUT_FILE" -db "$DATABASE_URL"

    # Clean up
    rm -f /tmp/seed_test_services

    echo ""
elif [ -f /home/kevinlam/Documents/projects/payments/.env ]; then
    echo "⚠️  To automatically seed services into database, set DATABASE_URL environment variable"
    echo "    Example: export DATABASE_URL=\"postgres://postgres:postgres@localhost:5432/payment_service?sslmode=disable\""
    echo ""
    echo "    Or run manually:"
    echo "    go run scripts/seed_test_services/seed_test_services.go -input \"$OUTPUT_FILE\" -db \"\$DATABASE_URL\""
    echo ""
fi

echo "ℹ️  These keys are for TESTING ONLY and should never be committed to Git."
echo "ℹ️  The file is listed in .gitignore to prevent accidental commits."
