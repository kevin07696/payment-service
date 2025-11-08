-- Agent Credentials for Staging/Testing
-- Seeds the agent_credentials table with EPX sandbox merchant for testing
-- This follows the multi-tenant architecture where credentials are per-agent in the database

BEGIN;

-- ============================================
-- TEST AGENT (EPX Sandbox Merchant)
-- ============================================
-- This agent uses EPX sandbox credentials for integration testing
-- Credentials from .env.example (EPX documentation)

INSERT INTO agent_credentials (
    id,
    agent_id,
    mac_secret_path,
    cust_nbr,
    merch_nbr,
    dba_nbr,
    terminal_nbr,
    environment,
    agent_name,
    is_active,
    created_at,
    updated_at
) VALUES (
    'agent_epx_sandbox_001',
    'test-merchant-staging',
    'payments/agents/test-merchant-staging/mac',  -- Path for secret manager
    '9001',                                         -- EPX sandbox customer number
    '900300',                                       -- EPX sandbox merchant number
    '2',                                           -- EPX sandbox DBA number
    '77',                                          -- EPX sandbox terminal number
    'test',                                        -- Environment (test uses sandbox endpoints)
    'EPX Sandbox Test Merchant',
    true,
    NOW(),
    NOW()
) ON CONFLICT (id) DO UPDATE SET
    agent_id = EXCLUDED.agent_id,
    mac_secret_path = EXCLUDED.mac_secret_path,
    cust_nbr = EXCLUDED.cust_nbr,
    merch_nbr = EXCLUDED.merch_nbr,
    dba_nbr = EXCLUDED.dba_nbr,
    terminal_nbr = EXCLUDED.terminal_nbr,
    environment = EXCLUDED.environment,
    agent_name = EXCLUDED.agent_name,
    is_active = EXCLUDED.is_active,
    updated_at = NOW();

-- ============================================
-- SUMMARY
-- ============================================

DO $$
DECLARE
    agent_count INT;
BEGIN
    SELECT COUNT(*) INTO agent_count FROM agent_credentials WHERE is_active = true;

    RAISE NOTICE '';
    RAISE NOTICE '==================================================';
    RAISE NOTICE 'Agent Credentials Summary';
    RAISE NOTICE '==================================================';
    RAISE NOTICE 'Active Agents: %', agent_count;
    RAISE NOTICE '';
    RAISE NOTICE 'Test Agent Details:';
    RAISE NOTICE '  Agent ID:     test-merchant-staging';
    RAISE NOTICE '  Environment:  test (EPX sandbox)';
    RAISE NOTICE '  CUST_NBR:     9001';
    RAISE NOTICE '  MERCH_NBR:    900300';
    RAISE NOTICE '  DBA_NBR:      2';
    RAISE NOTICE '  TERMINAL_NBR: 77';
    RAISE NOTICE '';
    RAISE NOTICE 'MAC Secret Path: payments/agents/test-merchant-staging/mac';
    RAISE NOTICE '  (Actual MAC: 2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y - store in Secret Manager)';
    RAISE NOTICE '';
    RAISE NOTICE '✅ Agent credentials loaded successfully!';
    RAISE NOTICE '';
    RAISE NOTICE 'Note: This follows multi-tenant architecture where EPX credentials';
    RAISE NOTICE '      are stored per-agent in the database, not as environment variables.';
    RAISE NOTICE '';
END $$;

COMMIT;
