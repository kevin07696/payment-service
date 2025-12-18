#!/bin/bash
# Seed test chargebacks for integration testing
# This script creates test chargeback data linked to existing or new transactions
#
# SECURITY NOTES:
# - Uses parameterized queries via psql -v to prevent SQL injection
# - Validates all input variables before use
# - Uses constants matching testutil/constants.go for consistency

set -e  # Exit on any error
set -u  # Exit on undefined variables
set -o pipefail  # Exit on pipe failures

echo "🔧 Seeding test chargebacks..."

# Define constants (must match testutil/constants.go)
readonly TEST_MERCHANT_UUID="00000000-0000-0000-0000-000000000001"
readonly TEST_MERCHANT_SLUG="test-merchant-staging"
readonly TEST_CUSTOMER_ID="cust_test_001"

# Validate UUID format (basic validation)
if ! [[ "$TEST_MERCHANT_UUID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]]; then
    echo "ERROR: Invalid TEST_MERCHANT_UUID format" >&2
    exit 1
fi

# Validate slug format (alphanumeric and hyphens only)
if ! [[ "$TEST_MERCHANT_SLUG" =~ ^[a-z0-9-]+$ ]]; then
    echo "ERROR: Invalid TEST_MERCHANT_SLUG format" >&2
    exit 1
fi

# Validate customer ID format (alphanumeric, underscores, hyphens only)
if ! [[ "$TEST_CUSTOMER_ID" =~ ^[a-zA-Z0-9_-]+$ ]]; then
    echo "ERROR: Invalid TEST_CUSTOMER_ID format" >&2
    exit 1
fi

# Use psql with variable binding to prevent SQL injection
# Variables are passed via -v flag and referenced as :variable_name
podman exec -i payment-postgres psql -U postgres -d payment_service \
    -v merchant_uuid="$TEST_MERCHANT_UUID" \
    -v merchant_slug="$TEST_MERCHANT_SLUG" \
    -v customer_id="$TEST_CUSTOMER_ID" \
    <<'EOF'
DO $$
DECLARE
    test_txn_id UUID;
    test_merchant_uuid UUID := :'merchant_uuid'::uuid;
    test_merchant_slug VARCHAR(100) := :'merchant_slug';
    test_customer_id VARCHAR(100) := :'customer_id';
BEGIN
    -- Get an existing approved transaction or create one
    -- Note: transactions.merchant_id is UUID, chargebacks.merchant_id is VARCHAR
    -- status is generated: 'approved' when auth_resp = '00'
    SELECT id INTO test_txn_id
    FROM transactions
    WHERE merchant_id = test_merchant_uuid
    AND status = 'approved'
    LIMIT 1;

    IF test_txn_id IS NULL THEN
        -- Create a settled transaction to link chargebacks to
        -- Note: status is generated from auth_resp, so we set auth_resp='00' for approved
        INSERT INTO transactions (
            id, merchant_id, customer_id,
            amount_cents, currency, type, payment_method_type,
            tran_nbr, auth_guid, auth_resp, auth_code,
            processed_at, created_at, updated_at
        ) VALUES (
            gen_random_uuid(), test_merchant_uuid, gen_random_uuid(),
            10000, 'USD', 'SALE', 'credit_card',
            'TXN-' || substr(gen_random_uuid()::text, 1, 10), 'BRIC-' || gen_random_uuid()::text,
            '00', 'AUTH123',
            NOW(), NOW(), NOW()
        ) RETURNING id INTO test_txn_id;

        RAISE NOTICE '✅ Created test transaction: %', test_txn_id;
    ELSE
        RAISE NOTICE '✅ Using existing transaction: %', test_txn_id;
    END IF;

    -- Insert test chargebacks with various statuses
    -- Use fixed case numbers matching testutil/constants.go for idempotency
    INSERT INTO chargebacks (
        transaction_id, merchant_id, customer_id,
        case_number, dispute_date, chargeback_date,
        chargeback_amount, currency, reason_code, reason_description,
        status, raw_data
    ) VALUES
    -- NEW chargeback (just received)
    (
        test_txn_id, test_merchant_slug, test_customer_id,
        'CB-NEW-TEST',
        NOW() - INTERVAL '5 days', NOW() - INTERVAL '3 days',
        '50.00', 'USD', 'P22', 'Cardholder disputes quality of goods or services',
        'new', '{"status": "NEW", "source": "test_seed", "test": true}'::jsonb
    ),
    -- PENDING chargeback (under review)
    (
        test_txn_id, test_merchant_slug, test_customer_id,
        'CB-PENDING-TEST',
        NOW() - INTERVAL '10 days', NOW() - INTERVAL '7 days',
        '75.50', 'USD', 'F10', 'Fraudulent transaction - card absent environment',
        'pending', '{"status": "PENDING", "source": "test_seed", "test": true}'::jsonb
    ),
    -- RESPONDED chargeback (we submitted evidence)
    (
        test_txn_id, test_merchant_slug, test_customer_id,
        'CB-RESPONDED-TEST',
        NOW() - INTERVAL '20 days', NOW() - INTERVAL '15 days',
        '100.00', 'USD', 'C08', 'Goods/Services not received',
        'responded', '{"status": "RESPONDED", "source": "test_seed", "test": true}'::jsonb
    ),
    -- WON chargeback (merchant won the dispute)
    (
        test_txn_id, test_merchant_slug, test_customer_id,
        'CB-WON-TEST',
        NOW() - INTERVAL '30 days', NOW() - INTERVAL '25 days',
        '125.00', 'USD', 'P08', 'Credit not processed',
        'won', '{"status": "WON", "source": "test_seed", "test": true}'::jsonb
    ),
    -- LOST chargeback (merchant lost the dispute)
    (
        test_txn_id, test_merchant_slug, test_customer_id,
        'CB-LOST-TEST',
        NOW() - INTERVAL '35 days', NOW() - INTERVAL '30 days',
        '200.00', 'USD', 'F29', 'Card not present fraud',
        'lost', '{"status": "LOST", "source": "test_seed", "test": true}'::jsonb
    )
    ON CONFLICT (case_number) DO NOTHING;

    RAISE NOTICE '✅ Seeded test chargebacks for integration testing';
END $$;

-- Display created chargebacks
-- Use parameterized query to safely filter by merchant
SELECT
    '📋 Test Chargebacks Created:' as message;

SELECT
    id,
    case_number,
    chargeback_amount,
    status,
    reason_code,
    dispute_date::date
FROM chargebacks
WHERE merchant_id = :'merchant_slug'
ORDER BY created_at DESC
LIMIT 10;
EOF

echo ""
echo "✅ Test chargebacks seeded successfully!"
echo ""
echo "🧪 Run integration tests with:"
echo "   SERVICE_URL=\"http://localhost:8080\" go test -v -tags=integration ./tests/integration/chargeback/... -count=1"
echo ""
