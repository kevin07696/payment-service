#!/bin/bash

# This script gets a real BRIC for ACME merchant using the automated browser test

echo "=========================================="
echo "Getting Real BRIC for ACME Corporation"
echo "=========================================="
echo ""
echo "Running automated browser test to get BRIC from EPX..."
echo "This will use headless Chrome to interact with EPX"
echo ""

# Run the test with ACME merchant ID
# Note: We're modifying the test merchant ID temporarily

# Save current test merchant
TEST_MERCHANT_ID="00000000-0000-0000-0000-000000000001"
ACME_MERCHANT_ID="1a20fff8-2cec-48e5-af49-87e501652913"

# Update test merchant in database to use ACME's credentials temporarily
PGPASSWORD=postgres psql -U postgres -h localhost -d payment_service <<SQL
-- Temporarily update test merchant to use ACME's MAC secret
UPDATE merchants
SET mac_secret_path = '/secrets/epx-sandbox-mac',
    status = 'active'
WHERE id = '$TEST_MERCHANT_ID';
SQL

echo "✅ Updated test merchant to use ACME's EPX configuration"
echo ""
echo "Running Browser Post integration test..."

# Run the test
SERVICE_URL="http://localhost:8081" go test -v -tags=integration \
  ./tests/integration/payment \
  -run TestIntegration_BrowserPost_SaleRefund_Workflow \
  -timeout 60s

# Extract the BRIC from the latest transaction
echo ""
echo "=========================================="
echo "Extracting BRIC from database..."

BRIC=$(PGPASSWORD=postgres psql -U postgres -h localhost -d payment_service -t -c \
  "SELECT auth_guid FROM transactions WHERE merchant_id = '$TEST_MERCHANT_ID' AND status = 'approved' ORDER BY created_at DESC LIMIT 1" | tr -d ' ')

if [ -n "$BRIC" ]; then
  echo "✅ Got real BRIC: $BRIC"
  echo ""
  echo "Now you can use this BRIC with ACME merchant:"
  echo "go run examples/test_real_bric.go -bric=$BRIC -amount=10.00"
else
  echo "❌ No approved BRIC found. Check test output above for errors."
fi

echo ""
echo "=========================================="
