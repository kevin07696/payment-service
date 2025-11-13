#!/bin/bash
set -e

echo "========================================================================"
echo "               🔑 BRIC Fixtures Status Check"
echo "========================================================================"
echo ""

# Check if we have any BRICs in the fixtures file
FIXTURES_FILE="tests/integration/fixtures/epx_brics.go"

if [ ! -f "$FIXTURES_FILE" ]; then
    echo "❌ Fixtures file not found: $FIXTURES_FILE"
    exit 1
fi

# Check if BRICs are still placeholders
if grep -q "REPLACE-WITH-REAL-BRIC-FROM-EPX-AFTER-RUNNING-MANUAL-TEST" "$FIXTURES_FILE"; then
    echo "⚠️  BRIC Fixtures are PLACEHOLDERS - Need real BRICs from EPX"
    echo ""
    echo "To get real BRICs:"
    echo "   1. Make sure payment-server is running:"
    echo "      podman-compose up -d"
    echo ""
    echo "   2. Start ngrok to expose localhost:"
    echo "      ngrok http 8081"
    echo ""
    echo "   3. Open the manual test page:"
    echo "      xdg-open tests/manual/get_real_bric.html"
    echo ""
    echo "   4. Follow the instructions in the browser"
    echo ""
    echo "   5. After getting BRIC, update: $FIXTURES_FILE"
    echo ""
    echo "See: tests/manual/README.md for detailed instructions"
    echo "========================================================================"
    exit 1
fi

# If we get here, we have real BRICs
echo "✅ BRIC Fixtures contain real BRICs (not placeholders)"
echo ""

# Check database for existing BRICs
echo "📊 Checking database for BRICs..."
podman exec payment-postgres psql -U postgres -d payment_service -t -c \
  "SELECT COUNT(*) FROM transaction_groups WHERE auth_guid IS NOT NULL;" 2>/dev/null | xargs echo "   Database BRICs:" || echo "   ⚠️  Could not connect to database"

echo ""
echo "========================================================================"
echo "✅ Ready to run integration tests with real BRICs!"
echo ""
echo "Run tests:"
echo "   export SERVICE_URL='http://localhost:8081'"
echo "   go test -v -tags=integration ./tests/integration/payment/... -run AuthCapture"
echo "========================================================================"
