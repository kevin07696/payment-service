#!/bin/bash
# Setup script for E2E tests
# This creates a test merchant/service and generates a JWT token

set -e

# Validate required environment variables - no hardcoded defaults
if [ -z "$SERVICE_URL" ]; then
    echo "❌ SERVICE_URL environment variable is required"
    echo "   Example: export SERVICE_URL='http://localhost:8081'"
    exit 1
fi

echo "=== E2E Test Environment Setup ==="
echo "Service URL: $SERVICE_URL"
echo ""

# Check if service is running
if ! curl -sf "$SERVICE_URL/cron/health" > /dev/null; then
    echo "❌ Service not running at $SERVICE_URL"
    echo "   Start with: podman-compose up -d"
    exit 1
fi
echo "✅ Service is healthy"

# For now, use the admin tool to create test data
# This requires the admin binary and database access

ADMIN_BIN="../../bin/admin"
if [ ! -f "$ADMIN_BIN" ]; then
    echo "Building admin tool..."
    (cd ../.. && go build -o bin/admin ./cmd/admin)
fi

# Generate test merchant and service
echo ""
echo "To run E2E tests, set these environment variables:"
echo ""
echo "  export TEST_MERCHANT_ID='<merchant-uuid>'"
echo "  export TEST_JWT_TOKEN='<jwt-token>'"
echo "  export SERVICE_URL='$SERVICE_URL'"
echo ""
echo "You can generate a JWT token using:"
echo "  ./bin/jwtgen -service-id=<service-id> -merchant-id=<merchant-id> -key=<private-key-file>"
echo ""
echo "Then run: npm test"
