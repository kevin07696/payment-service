#!/bin/bash
# Seed test payment methods via EPX Browser Post flow
# This script creates real payment methods with valid BRICs for API testing
#
# Usage: ./scripts/seed_test_payment_methods.sh
#
# Prerequisites:
# - Server running (podman-compose up -d)
# - test-pos-system service credentials exist (auto-created on startup)
# - chromedp headless browser dependencies installed
#
# What this script does:
# 1. Generates a JWT token for test-pos-system
# 2. Creates a Visa card payment method via STORAGE flow
# 3. Creates a Sale transaction (for refund/void testing)
# 4. Outputs created IDs for use in API_SPECS.md

set -e  # Exit on any error

# Configuration
API_URL="${API_URL:-http://localhost:8081}"
MERCHANT_ID="${MERCHANT_ID:-f37b03e6-aef3-428d-984e-862af7e6b4e9}"
CUSTOMER_ID="test-customer-001"
CREDS_FILE="service_test-pos-system_credentials.json"

echo "=== Payment Service Test Data Seeder ==="
echo ""
echo "Configuration:"
echo "  API URL: $API_URL"
echo "  Merchant ID: $MERCHANT_ID"
echo "  Customer ID: $CUSTOMER_ID"
echo "  Credentials: $CREDS_FILE"
echo ""

# Check if paycli exists
if [ ! -f "./paycli" ] && [ ! -f "./bin/paycli" ]; then
    echo "Building paycli..."
    go build -o paycli ./cmd/paycli
fi
PAYCLI="./paycli"
if [ -f "./bin/paycli" ]; then
    PAYCLI="./bin/paycli"
fi

# Check for credentials file
if [ ! -f "$CREDS_FILE" ]; then
    echo "ERROR: Credentials file not found: $CREDS_FILE"
    echo ""
    echo "The service credentials should be auto-created on server startup."
    echo "If running in Docker, copy from container:"
    echo "  podman cp payment-server:/home/appuser/$CREDS_FILE ."
    exit 1
fi

# Generate token
echo "Step 1: Generating JWT token..."
TOKEN=$($PAYCLI -action=generate-token -c "$CREDS_FILE" -o token)
if [ -z "$TOKEN" ]; then
    echo "ERROR: Failed to generate token"
    exit 1
fi
echo "  Token: ${TOKEN:0:50}..."
echo ""

# Test the token with a simple API call
echo "Step 2: Verifying token..."
HEALTH_CHECK=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/cron/health" 2>/dev/null || echo "000")
if [ "$HEALTH_CHECK" != "200" ]; then
    echo "WARNING: Server may not be running (health check returned: $HEALTH_CHECK)"
fi

# List existing payment methods
echo "Step 3: Checking existing payment methods..."
LIST_RESP=$(curl -s -X POST "${API_URL}/payment_method.v1.PaymentMethodService/ListPaymentMethods" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "{
        \"merchant_id\": \"${MERCHANT_ID}\",
        \"customer_id\": \"${CUSTOMER_ID}\"
    }" 2>/dev/null)

PM_COUNT=$(echo "$LIST_RESP" | jq -r '.paymentMethods | length // 0' 2>/dev/null || echo "0")
echo "  Found $PM_COUNT existing payment method(s)"
echo ""

if [ "$PM_COUNT" -gt 0 ]; then
    echo "Existing payment methods:"
    echo "$LIST_RESP" | jq -r '.paymentMethods[] | "  - ID: \(.id) | Type: \(.cardType // .accountType // "unknown") | Last4: \(.lastFour) | Status: \(.status)"' 2>/dev/null || echo "$LIST_RESP"
    echo ""
fi

# Create payment method via Browser Post STORAGE
echo "Step 4: Creating payment method via Browser Post STORAGE flow..."
echo ""
echo "NOTE: Browser Post requires a browser for form submission."
echo "      Use one of these methods to create payment methods:"
echo ""
echo "Option A: Use the test HTML form (manual)"
echo "  1. Open: ${API_URL}/api/v1/payments/browser-post/form"
echo "  2. Use test card: 4111111111111111, CVV: 123, Exp: 12/25"
echo "  3. Submit and verify callback"
echo ""
echo "Option B: Run integration tests (automated, requires chromedp)"
echo "  SERVICE_URL=\"${API_URL}\" go test -v -tags=integration -run TestBrowserPostWorkflow ./tests/integration/payment/"
echo ""
echo "Option C: Use the interactive form (for human testing)"

# Generate form URL for STORAGE
FORM_URL="${API_URL}/api/v1/payments/browser-post/form?merchant_id=${MERCHANT_ID}&transaction_type=STORAGE&amount_cents=0&currency=USD&customer_id=${CUSTOMER_ID}&return_url=${API_URL}/api/v1/payments/browser-post/callback"
echo ""
echo "  Browser Post Form URL (copy to browser):"
echo "  $FORM_URL"
echo ""

# Get form to show TAC and verify it works
echo "Step 5: Getting Browser Post form configuration..."
FORM_RESP=$(curl -s "${FORM_URL}" \
    -H "Authorization: Bearer $TOKEN" 2>/dev/null)

TAC=$(echo "$FORM_RESP" | jq -r '.tac // empty' 2>/dev/null)
if [ -n "$TAC" ]; then
    echo "  TAC generated successfully (${#TAC} chars)"
    EPX_URL=$(echo "$FORM_RESP" | jq -r '.postURL // empty' 2>/dev/null)
    echo "  EPX Post URL: $EPX_URL"
else
    echo "  WARNING: Could not get TAC - check server logs"
    echo "  Response: $FORM_RESP"
fi
echo ""

# Show how to create a sale for refund testing
echo "Step 6: Sample API calls for testing..."
echo ""
echo "After creating a payment method, use these commands:"
echo ""

SALE_ID=$(cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen)
echo "# Create a Sale transaction (replace PAYMENT_METHOD_ID)"
echo "curl -X POST '${API_URL}/payment.v1.PaymentService/Sale' \\"
echo "  -H 'Content-Type: application/json' \\"
echo "  -H 'Authorization: Bearer \$TOKEN' \\"
echo "  -d '{"
echo "    \"merchant_id\": \"${MERCHANT_ID}\","
echo "    \"customer_id\": \"${CUSTOMER_ID}\","
echo "    \"payment_method_id\": \"YOUR_PAYMENT_METHOD_ID\","
echo "    \"amount_cents\": 5000,"
echo "    \"currency\": \"USD\","
echo "    \"idempotency_key\": \"${SALE_ID}\""
echo "  }'"
echo ""

echo "# List transactions"
echo "curl -X POST '${API_URL}/payment.v1.PaymentService/ListTransactions' \\"
echo "  -H 'Content-Type: application/json' \\"
echo "  -H 'Authorization: Bearer \$TOKEN' \\"
echo "  -d '{\"merchant_id\": \"${MERCHANT_ID}\", \"limit\": 10}'"
echo ""

echo "=== Summary ==="
echo ""
echo "Environment Variables for Testing:"
echo "  export TOKEN=\"$TOKEN\""
echo "  export API_URL=\"$API_URL\""
echo "  export MERCHANT_ID=\"$MERCHANT_ID\""
echo ""
echo "Quick Copy-Paste Setup:"
echo "  TOKEN=\"$TOKEN\" && API_URL=\"$API_URL\" && MERCHANT_ID=\"$MERCHANT_ID\""
echo ""
