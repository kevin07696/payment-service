-- EPX Test Scenarios
-- Contains specific test data for EPX gateway integration testing
-- Based on EPX sandbox test card numbers and scenarios

BEGIN;

-- ============================================
-- EPX TEST CARDS (For Integration Testing)
-- ============================================
-- These use EPX's documented test card numbers
-- See: https://developer.north.com/testing

INSERT INTO customers (id, email, first_name, last_name, phone, created_at, updated_at)
VALUES
    ('cust_epx_approved', 'approved@epxtest.com', 'Approved', 'Test', '+1555000001', NOW(), NOW()),
    ('cust_epx_declined', 'declined@epxtest.com', 'Declined', 'Test', '+1555000002', NOW(), NOW()),
    ('cust_epx_insufficient', 'insufficient@epxtest.com', 'Insufficient', 'Funds', '+1555000003', NOW(), NOW()),
    ('cust_epx_expired', 'expired@epxtest.com', 'Expired', 'Card', '+1555000004', NOW(), NOW()),
    ('cust_epx_invalid', 'invalid@epxtest.com', 'Invalid', 'CVV', '+1555000005', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Payment methods with EPX test scenarios
INSERT INTO payment_methods (id, customer_id, type, status, is_default, created_at, updated_at)
VALUES
    ('pm_epx_approved', 'cust_epx_approved', 'card', 'active', true, NOW(), NOW()),
    ('pm_epx_declined', 'cust_epx_declined', 'card', 'active', true, NOW(), NOW()),
    ('pm_epx_insufficient', 'cust_epx_insufficient', 'card', 'active', true, NOW(), NOW()),
    ('pm_epx_expired', 'cust_epx_expired', 'card', 'inactive', true, NOW(), NOW()),
    ('pm_epx_invalid', 'cust_epx_invalid', 'card', 'active', true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Card details with EPX test numbers
INSERT INTO cards (payment_method_id, bric_token, last_four, brand, exp_month, exp_year, cardholder_name, billing_address_line1, billing_address_city, billing_address_state, billing_address_postal_code, billing_address_country)
VALUES
    -- 4111111111111111 - Approved
    ('pm_epx_approved', 'BRIC_EPX_APPROVED', '1111', 'Visa', 12, 2026, 'Approved Test', '100 Approved St', 'San Francisco', 'CA', '94102', 'US'),

    -- 4000000000000002 - Declined (generic)
    ('pm_epx_declined', 'BRIC_EPX_DECLINED', '0002', 'Visa', 12, 2026, 'Declined Test', '200 Declined Ave', 'New York', 'NY', '10001', 'US'),

    -- 4000000000009995 - Insufficient funds
    ('pm_epx_insufficient', 'BRIC_EPX_INSUFFICIENT', '9995', 'Visa', 12, 2026, 'Insufficient Test', '300 Poor St', 'Austin', 'TX', '73301', 'US'),

    -- Expired card
    ('pm_epx_expired', 'BRIC_EPX_EXPIRED', '0001', 'Visa', 01, 2020, 'Expired Test', '400 Old Card Ln', 'Seattle', 'WA', '98101', 'US'),

    -- Invalid CVV test
    ('pm_epx_invalid', 'BRIC_EPX_INVALID', '0003', 'Visa', 12, 2026, 'Invalid CVV Test', '500 Wrong CVV Rd', 'Chicago', 'IL', '60601', 'US')
ON CONFLICT (payment_method_id) DO NOTHING;

-- ============================================
-- EPX RESPONSE CODE EXAMPLES
-- ============================================
-- Historical transactions showing various EPX response codes

INSERT INTO transactions (id, customer_id, payment_method_id, type, status, amount, currency, description, gateway_transaction_id, gateway_response_code, gateway_response_message, processed_at, created_at, updated_at)
VALUES
    -- 00 - Approved
    ('txn_epx_00', 'cust_epx_approved', 'pm_epx_approved', 'sale', 'success', 5000, 'USD', 'EPX Test - Approved', 'EPX_APPROVED_001', '00', 'Approved', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day'),

    -- 05 - Do not honor
    ('txn_epx_05', 'cust_epx_declined', 'pm_epx_declined', 'sale', 'failed', 10000, 'USD', 'EPX Test - Do not honor', 'EPX_DECLINED_001', '05', 'Do not honor', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day'),

    -- 51 - Insufficient funds
    ('txn_epx_51', 'cust_epx_insufficient', 'pm_epx_insufficient', 'sale', 'failed', 15000, 'USD', 'EPX Test - Insufficient funds', 'EPX_INSUFFICIENT_001', '51', 'Insufficient funds', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day'),

    -- 54 - Expired card
    ('txn_epx_54', 'cust_epx_expired', 'pm_epx_expired', 'sale', 'failed', 7500, 'USD', 'EPX Test - Expired card', 'EPX_EXPIRED_001', '54', 'Expired card', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day'),

    -- 82 - Invalid CVV
    ('txn_epx_82', 'cust_epx_invalid', 'pm_epx_invalid', 'sale', 'failed', 2500, 'USD', 'EPX Test - Invalid CVV', 'EPX_INVALID_001', '82', 'CVV validation failed', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day'),

    -- 85 - Card OK (auth-only, not captured)
    ('txn_epx_85_auth', 'cust_epx_approved', 'pm_epx_approved', 'auth', 'success', 3000, 'USD', 'EPX Test - Authorization only', 'EPX_AUTH_001', '85', 'Card OK', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours'),

    -- 85 - Card OK (captured after auth)
    ('txn_epx_85_capture', 'cust_epx_approved', 'pm_epx_approved', 'capture', 'success', 3000, 'USD', 'EPX Test - Capture after auth', 'EPX_CAPTURE_001', '00', 'Approved', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour'),

    -- Refund
    ('txn_epx_refund', 'cust_epx_approved', 'pm_epx_approved', 'refund', 'success', 5000, 'USD', 'EPX Test - Refund', 'EPX_REFUND_001', '00', 'Approved', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes'),

    -- Void
    ('txn_epx_void', 'cust_epx_approved', 'pm_epx_approved', 'void', 'success', 0, 'USD', 'EPX Test - Void', 'EPX_VOID_001', '00', 'Approved', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '15 minutes')
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- BROWSER POST TEST DATA
-- ============================================
-- Simulates data from EPX Browser Post tokenization

INSERT INTO customers (id, email, first_name, last_name, phone, created_at, updated_at)
VALUES
    ('cust_browserpost', 'browserpost@epxtest.com', 'Browser', 'Post', '+1555000100', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO payment_methods (id, customer_id, type, status, is_default, created_at, updated_at)
VALUES
    ('pm_browserpost_001', 'cust_browserpost', 'card', 'active', true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- BRIC token from Browser Post
INSERT INTO cards (payment_method_id, bric_token, last_four, brand, exp_month, exp_year, cardholder_name, billing_address_line1, billing_address_city, billing_address_state, billing_address_postal_code, billing_address_country)
VALUES
    ('pm_browserpost_001', 'BRIC_BROWSER_POST_TOKEN_001', '1111', 'Visa', 12, 2026, 'Browser Post Test', '123 Browser St', 'San Francisco', 'CA', '94102', 'US')
ON CONFLICT (payment_method_id) DO NOTHING;

-- ============================================
-- RECURRING BILLING TEST SCENARIOS
-- ============================================

INSERT INTO subscriptions (id, customer_id, payment_method_id, status, plan_name, amount, currency, interval, interval_count, start_date, next_billing_date, created_at, updated_at)
VALUES
    -- Subscription that will bill successfully
    ('sub_epx_success', 'cust_epx_approved', 'pm_epx_approved', 'active', 'EPX Test - Success Plan', 2999, 'USD', 'month', 1, NOW() - INTERVAL '30 days', NOW() + INTERVAL '1 hour', NOW() - INTERVAL '30 days', NOW()),

    -- Subscription that will fail (insufficient funds)
    ('sub_epx_fail', 'cust_epx_insufficient', 'pm_epx_insufficient', 'active', 'EPX Test - Fail Plan', 4999, 'USD', 'month', 1, NOW() - INTERVAL '30 days', NOW() + INTERVAL '2 hours', NOW() - INTERVAL '30 days', NOW()),

    -- Subscription with expired card
    ('sub_epx_expired', 'cust_epx_expired', 'pm_epx_expired', 'past_due', 'EPX Test - Expired Plan', 1999, 'USD', 'month', 1, NOW() - INTERVAL '60 days', NOW() - INTERVAL '30 days', NOW() - INTERVAL '60 days', NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- SUMMARY
-- ============================================

DO $$
DECLARE
    epx_customer_count INT;
    epx_payment_method_count INT;
    epx_transaction_count INT;
    epx_subscription_count INT;
BEGIN
    SELECT COUNT(*) INTO epx_customer_count FROM customers WHERE id LIKE 'cust_epx_%' OR id = 'cust_browserpost';
    SELECT COUNT(*) INTO epx_payment_method_count FROM payment_methods WHERE id LIKE 'pm_epx_%' OR id LIKE 'pm_browserpost_%';
    SELECT COUNT(*) INTO epx_transaction_count FROM transactions WHERE id LIKE 'txn_epx_%';
    SELECT COUNT(*) INTO epx_subscription_count FROM subscriptions WHERE id LIKE 'sub_epx_%';

    RAISE NOTICE '';
    RAISE NOTICE '==================================================';
    RAISE NOTICE 'EPX Test Data Summary';
    RAISE NOTICE '==================================================';
    RAISE NOTICE 'EPX Test Customers:       %', epx_customer_count;
    RAISE NOTICE 'EPX Test Payment Methods: %', epx_payment_method_count;
    RAISE NOTICE 'EPX Test Transactions:    %', epx_transaction_count;
    RAISE NOTICE 'EPX Test Subscriptions:   %', epx_subscription_count;
    RAISE NOTICE '==================================================';
    RAISE NOTICE '';
    RAISE NOTICE 'Test Scenarios Available:';
    RAISE NOTICE '  - Approved transactions (00)';
    RAISE NOTICE '  - Declined transactions (05)';
    RAISE NOTICE '  - Insufficient funds (51)';
    RAISE NOTICE '  - Expired card (54)';
    RAISE NOTICE '  - Invalid CVV (82)';
    RAISE NOTICE '  - Auth/Capture flow (85)';
    RAISE NOTICE '  - Refunds and voids';
    RAISE NOTICE '  - Browser Post tokenization';
    RAISE NOTICE '  - Recurring billing scenarios';
    RAISE NOTICE '';
    RAISE NOTICE '✅ EPX test data loaded successfully!';
    RAISE NOTICE '';
END $$;

COMMIT;
