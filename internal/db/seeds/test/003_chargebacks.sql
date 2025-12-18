-- Seed chargebacks for integration testing
-- These represent disputes/chargebacks that would normally be synced from North API

-- Insert test chargebacks for test-merchant-staging
-- Note: Using dummy transaction IDs - these would normally reference actual transactions
INSERT INTO chargebacks (
    id,
    transaction_id,
    merchant_id,
    customer_id,
    case_number,
    dispute_date,
    chargeback_date,
    chargeback_amount,
    currency,
    reason_code,
    reason_description,
    status,
    respond_by_date,
    raw_data,
    created_at,
    updated_at
) VALUES
(
    '11111111-1111-1111-1111-111111111111',
    '00000000-0000-0000-0000-000000000001', -- Dummy transaction ID
    'test-merchant-staging',
    'test-customer-001',
    'CB-2025-001',
    '2025-11-01 10:00:00+00',
    '2025-11-05 10:00:00+00',
    '150.00',
    'USD',
    'F10',
    'Non-Receipt of Merchandise',
    'new',
    '2025-12-01',
    '{"source": "test_seed"}'::jsonb,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
),
(
    '22222222-2222-2222-2222-222222222222',
    '00000000-0000-0000-0000-000000000002', -- Dummy transaction ID
    'test-merchant-staging',
    'test-customer-002',
    'CB-2025-002',
    '2025-11-10 14:30:00+00',
    '2025-11-12 14:30:00+00',
    '75.50',
    'USD',
    'P22',
    'Not as Described',
    'pending',
    '2025-12-10',
    '{"source": "test_seed"}'::jsonb,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
),
(
    '33333333-3333-3333-3333-333333333333',
    '00000000-0000-0000-0000-000000000003', -- Dummy transaction ID
    'test-merchant-staging',
    'test-customer-001',
    'CB-2025-003',
    '2025-10-15 09:00:00+00',
    '2025-10-20 09:00:00+00',
    '200.00',
    'USD',
    'C08',
    'Credit Not Processed',
    'won',
    NULL, -- Already resolved
    '{"source": "test_seed"}'::jsonb,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
)
ON CONFLICT (case_number) DO NOTHING;
