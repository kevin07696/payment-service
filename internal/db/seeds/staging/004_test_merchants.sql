-- Test Merchants for Integration Testing
-- Seeds the merchants table with test merchant for Browser Post and payment testing

BEGIN;

-- ============================================
-- TEST MERCHANT (EPX Sandbox)
-- ============================================
-- This merchant uses EPX sandbox credentials for integration testing
-- Fixed UUID allows integration tests to reference this merchant consistently

INSERT INTO merchants (
    id,
    slug,
    mac_secret_path,
    cust_nbr,
    merch_nbr,
    dba_nbr,
    terminal_nbr,
    environment,
    name,
    is_active,
    created_at,
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000001'::uuid,  -- Fixed UUID for testing
    'test-merchant-staging',
    'payments/merchants/test-merchant-staging/mac',  -- Path for secret manager
    '9001',                                          -- EPX sandbox customer number
    '900300',                                        -- EPX sandbox merchant number
    '2',                                            -- EPX sandbox DBA number
    '77',                                           -- EPX sandbox terminal number
    'test',                                         -- Environment (test uses sandbox endpoints)
    'EPX Sandbox Test Merchant',
    true,
    NOW(),
    NOW()
) ON CONFLICT (id) DO UPDATE SET
    slug = EXCLUDED.slug,
    mac_secret_path = EXCLUDED.mac_secret_path,
    cust_nbr = EXCLUDED.cust_nbr,
    merch_nbr = EXCLUDED.merch_nbr,
    dba_nbr = EXCLUDED.dba_nbr,
    terminal_nbr = EXCLUDED.terminal_nbr,
    environment = EXCLUDED.environment,
    name = EXCLUDED.name,
    is_active = EXCLUDED.is_active,
    updated_at = NOW();

-- ============================================
-- SUMMARY
-- ============================================

DO $$
DECLARE
    merchant_count INT;
BEGIN
    SELECT COUNT(*) INTO merchant_count FROM merchants WHERE is_active = true;

    RAISE NOTICE '';
    RAISE NOTICE '==================================================';
    RAISE NOTICE 'Test Merchants Summary';
    RAISE NOTICE '==================================================';
    RAISE NOTICE 'Active Merchants: %', merchant_count;
    RAISE NOTICE '';
    RAISE NOTICE 'Test Merchant Details:';
    RAISE NOTICE '  Merchant ID:  00000000-0000-0000-0000-000000000001';
    RAISE NOTICE '  Slug:         test-merchant-staging';
    RAISE NOTICE '  Environment:  test (EPX sandbox)';
    RAISE NOTICE '  CUST_NBR:     9001';
    RAISE NOTICE '  MERCH_NBR:    900300';
    RAISE NOTICE '  DBA_NBR:      2';
    RAISE NOTICE '  TERMINAL_NBR: 77';
    RAISE NOTICE '';
    RAISE NOTICE 'MAC Secret Path: payments/merchants/test-merchant-staging/mac';
    RAISE NOTICE '  (Actual MAC: 2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y - store in Secret Manager)';
    RAISE NOTICE '';
    RAISE NOTICE '✅ Test merchants loaded successfully!';
    RAISE NOTICE '';
    RAISE NOTICE 'Integration tests can use this merchant ID:';
    RAISE NOTICE '  merchantID := "00000000-0000-0000-0000-000000000001"';
    RAISE NOTICE '';
END $$;

COMMIT;