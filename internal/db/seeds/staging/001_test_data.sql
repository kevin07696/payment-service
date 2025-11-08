-- Staging Environment Seed Data
-- This file contains test data for staging environment
-- Safe to run multiple times (uses ON CONFLICT DO NOTHING where possible)

BEGIN;

-- ============================================
-- CUSTOMERS (Test Users)
-- ============================================

INSERT INTO customers (id, email, first_name, last_name, phone, created_at, updated_at)
VALUES
    ('cust_test_001', 'john.doe@example.com', 'John', 'Doe', '+1234567890', NOW(), NOW()),
    ('cust_test_002', 'jane.smith@example.com', 'Jane', 'Smith', '+1234567891', NOW(), NOW()),
    ('cust_test_003', 'bob.wilson@example.com', 'Bob', 'Wilson', '+1234567892', NOW(), NOW()),
    ('cust_test_004', 'alice.brown@example.com', 'Alice', 'Brown', '+1234567893', NOW(), NOW()),
    ('cust_test_005', 'charlie.davis@example.com', 'Charlie', 'Davis', '+1234567894', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- PAYMENT METHODS (Saved Cards - BRIC Tokens)
-- ============================================

INSERT INTO payment_methods (id, customer_id, type, status, is_default, created_at, updated_at)
VALUES
    ('pm_test_001', 'cust_test_001', 'card', 'active', true, NOW(), NOW()),
    ('pm_test_002', 'cust_test_002', 'card', 'active', true, NOW(), NOW()),
    ('pm_test_003', 'cust_test_003', 'ach', 'active', true, NOW(), NOW()),
    ('pm_test_004', 'cust_test_004', 'card', 'active', false, NOW(), NOW()),
    ('pm_test_005', 'cust_test_004', 'card', 'active', true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- CARD DETAILS (Associated with Payment Methods)
-- ============================================

INSERT INTO cards (payment_method_id, bric_token, last_four, brand, exp_month, exp_year, cardholder_name, billing_address_line1, billing_address_city, billing_address_state, billing_address_postal_code, billing_address_country)
VALUES
    ('pm_test_001', 'BRIC_TEST_TOKEN_001', '1111', 'Visa', 12, 2026, 'John Doe', '123 Main St', 'San Francisco', 'CA', '94102', 'US'),
    ('pm_test_002', 'BRIC_TEST_TOKEN_002', '4242', 'Visa', 06, 2027, 'Jane Smith', '456 Oak Ave', 'New York', 'NY', '10001', 'US'),
    ('pm_test_004', 'BRIC_TEST_TOKEN_003', '5555', 'Mastercard', 03, 2025, 'Alice Brown', '789 Pine St', 'Austin', 'TX', '73301', 'US'),
    ('pm_test_005', 'BRIC_TEST_TOKEN_004', '8888', 'Amex', 09, 2028, 'Alice Brown', '789 Pine St', 'Austin', 'TX', '73301', 'US')
ON CONFLICT (payment_method_id) DO NOTHING;

-- ============================================
-- ACH DETAILS (Associated with Payment Methods)
-- ============================================

INSERT INTO ach_accounts (payment_method_id, account_type, account_number_last_four, routing_number, account_holder_name, bank_name)
VALUES
    ('pm_test_003', 'checking', '6789', '021000021', 'Bob Wilson', 'Chase Bank')
ON CONFLICT (payment_method_id) DO NOTHING;

-- ============================================
-- TRANSACTIONS (Payment History)
-- ============================================

INSERT INTO transactions (id, customer_id, payment_method_id, type, status, amount, currency, description, gateway_transaction_id, gateway_response_code, gateway_response_message, processed_at, created_at, updated_at)
VALUES
    -- Successful payments
    ('txn_test_001', 'cust_test_001', 'pm_test_001', 'sale', 'success', 5000, 'USD', 'Test payment - Monthly subscription', 'EPX_123456789', '00', 'Approved', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),
    ('txn_test_002', 'cust_test_002', 'pm_test_002', 'sale', 'success', 10000, 'USD', 'Test payment - One-time purchase', 'EPX_123456790', '00', 'Approved', NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days'),
    ('txn_test_003', 'cust_test_003', 'pm_test_003', 'sale', 'success', 15000, 'USD', 'Test payment - ACH transfer', 'EPX_123456791', '00', 'Approved', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days'),

    -- Failed payments
    ('txn_test_004', 'cust_test_004', 'pm_test_004', 'sale', 'failed', 7500, 'USD', 'Test payment - Insufficient funds', 'EPX_123456792', '51', 'Insufficient funds', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days'),
    ('txn_test_005', 'cust_test_005', 'pm_test_001', 'sale', 'failed', 2500, 'USD', 'Test payment - Declined', 'EPX_123456793', '05', 'Do not honor', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day'),

    -- Pending/Processing
    ('txn_test_006', 'cust_test_001', 'pm_test_001', 'auth', 'pending', 3000, 'USD', 'Test authorization - Pending capture', 'EPX_123456794', '00', 'Approved', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours'),

    -- Refunded
    ('txn_test_007', 'cust_test_002', 'pm_test_002', 'refund', 'success', 5000, 'USD', 'Refund for txn_test_001', 'EPX_123456795', '00', 'Approved', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour')
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- SUBSCRIPTIONS (Recurring Billing)
-- ============================================

INSERT INTO subscriptions (id, customer_id, payment_method_id, status, plan_name, amount, currency, interval, interval_count, start_date, next_billing_date, created_at, updated_at)
VALUES
    -- Active subscriptions
    ('sub_test_001', 'cust_test_001', 'pm_test_001', 'active', 'Pro Plan', 4999, 'USD', 'month', 1, NOW() - INTERVAL '30 days', NOW() + INTERVAL '1 day', NOW() - INTERVAL '30 days', NOW()),
    ('sub_test_002', 'cust_test_002', 'pm_test_002', 'active', 'Enterprise Plan', 9999, 'USD', 'month', 1, NOW() - INTERVAL '60 days', NOW() + INTERVAL '5 days', NOW() - INTERVAL '60 days', NOW()),
    ('sub_test_003', 'cust_test_003', 'pm_test_003', 'active', 'Basic Plan', 1999, 'USD', 'month', 1, NOW() - INTERVAL '15 days', NOW() + INTERVAL '15 days', NOW() - INTERVAL '15 days', NOW()),

    -- Cancelled subscription
    ('sub_test_004', 'cust_test_004', 'pm_test_004', 'cancelled', 'Pro Plan', 4999, 'USD', 'month', 1, NOW() - INTERVAL '90 days', NULL, NOW() - INTERVAL '90 days', NOW() - INTERVAL '10 days'),

    -- Paused subscription
    ('sub_test_005', 'cust_test_005', 'pm_test_001', 'paused', 'Pro Plan', 4999, 'USD', 'month', 1, NOW() - INTERVAL '120 days', NOW() + INTERVAL '30 days', NOW() - INTERVAL '120 days', NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- SUBSCRIPTION BILLING HISTORY
-- ============================================

INSERT INTO subscription_billings (id, subscription_id, transaction_id, amount, currency, status, billing_period_start, billing_period_end, attempted_at, succeeded_at, created_at)
VALUES
    ('sbill_test_001', 'sub_test_001', 'txn_test_001', 4999, 'USD', 'success', NOW() - INTERVAL '30 days', NOW(), NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),
    ('sbill_test_002', 'sub_test_002', 'txn_test_002', 9999, 'USD', 'success', NOW() - INTERVAL '30 days', NOW(), NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days', NOW() - INTERVAL '4 days'),
    ('sbill_test_003', 'sub_test_003', 'txn_test_003', 1999, 'USD', 'success', NOW() - INTERVAL '30 days', NOW(), NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days'),
    ('sbill_test_004', 'sub_test_004', 'txn_test_004', 4999, 'USD', 'failed', NOW() - INTERVAL '30 days', NOW(), NOW() - INTERVAL '2 days', NULL, NOW() - INTERVAL '2 days')
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- CHARGEBACKS (Dispute Data)
-- ============================================

INSERT INTO chargebacks (id, transaction_id, amount, currency, reason_code, reason_description, status, case_number, submitted_at, due_date, created_at, updated_at)
VALUES
    ('cb_test_001', 'txn_test_001', 5000, 'USD', '10.4', 'Fraudulent transaction', 'pending', 'CB-2024-001', NOW() - INTERVAL '7 days', NOW() + INTERVAL '23 days', NOW() - INTERVAL '7 days', NOW()),
    ('cb_test_002', 'txn_test_002', 10000, 'USD', '13.1', 'Not as described', 'won', 'CB-2024-002', NOW() - INTERVAL '30 days', NOW() - INTERVAL '5 days', NOW() - INTERVAL '30 days', NOW() - INTERVAL '5 days'),
    ('cb_test_003', 'txn_test_003', 15000, 'USD', '11.3', 'Authorization issue', 'lost', 'CB-2024-003', NOW() - INTERVAL '60 days', NOW() - INTERVAL '30 days', NOW() - INTERVAL '60 days', NOW() - INTERVAL '30 days')
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- WEBHOOKS (Outbound Notifications)
-- ============================================

INSERT INTO webhooks (id, event_type, endpoint_url, status, payload, response_code, response_body, attempted_at, succeeded_at, retry_count, created_at, updated_at)
VALUES
    ('wh_test_001', 'payment.success', 'https://example.com/webhooks/payment', 'success', '{"transaction_id": "txn_test_001", "amount": 5000}', 200, '{"received": true}', NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', 0, NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days'),
    ('wh_test_002', 'subscription.created', 'https://example.com/webhooks/subscription', 'success', '{"subscription_id": "sub_test_001"}', 200, '{"received": true}', NOW() - INTERVAL '30 days', NOW() - INTERVAL '30 days', 0, NOW() - INTERVAL '30 days', NOW() - INTERVAL '30 days'),
    ('wh_test_003', 'chargeback.created', 'https://example.com/webhooks/chargeback', 'failed', '{"chargeback_id": "cb_test_001"}', 500, 'Internal server error', NOW() - INTERVAL '7 days', NULL, 3, NOW() - INTERVAL '7 days', NOW() - INTERVAL '6 days')
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- AUDIT LOG (System Events)
-- ============================================

INSERT INTO audit_logs (id, entity_type, entity_id, action, user_id, ip_address, user_agent, changes, created_at)
VALUES
    ('audit_test_001', 'transaction', 'txn_test_001', 'created', 'system', '192.168.1.1', 'Mozilla/5.0', '{"status": "success"}', NOW() - INTERVAL '5 days'),
    ('audit_test_002', 'subscription', 'sub_test_004', 'cancelled', 'cust_test_004', '192.168.1.2', 'Mozilla/5.0', '{"reason": "customer_request"}', NOW() - INTERVAL '10 days'),
    ('audit_test_003', 'payment_method', 'pm_test_001', 'created', 'cust_test_001', '192.168.1.3', 'Mozilla/5.0', '{"type": "card"}', NOW() - INTERVAL '30 days')
ON CONFLICT (id) DO NOTHING;

-- ============================================
-- SUMMARY
-- ============================================

-- Print summary
DO $$
DECLARE
    customer_count INT;
    payment_method_count INT;
    transaction_count INT;
    subscription_count INT;
    chargeback_count INT;
BEGIN
    SELECT COUNT(*) INTO customer_count FROM customers WHERE id LIKE 'cust_test_%';
    SELECT COUNT(*) INTO payment_method_count FROM payment_methods WHERE id LIKE 'pm_test_%';
    SELECT COUNT(*) INTO transaction_count FROM transactions WHERE id LIKE 'txn_test_%';
    SELECT COUNT(*) INTO subscription_count FROM subscriptions WHERE id LIKE 'sub_test_%';
    SELECT COUNT(*) INTO chargeback_count FROM chargebacks WHERE id LIKE 'cb_test_%';

    RAISE NOTICE '';
    RAISE NOTICE '==================================================';
    RAISE NOTICE 'Seed Data Summary';
    RAISE NOTICE '==================================================';
    RAISE NOTICE 'Customers:        %', customer_count;
    RAISE NOTICE 'Payment Methods:  %', payment_method_count;
    RAISE NOTICE 'Transactions:     %', transaction_count;
    RAISE NOTICE 'Subscriptions:    %', subscription_count;
    RAISE NOTICE 'Chargebacks:      %', chargeback_count;
    RAISE NOTICE '==================================================';
    RAISE NOTICE '';
    RAISE NOTICE '✅ Seed data loaded successfully!';
    RAISE NOTICE '';
END $$;

COMMIT;
