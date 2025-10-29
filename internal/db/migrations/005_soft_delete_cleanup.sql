-- +goose Up
-- +goose StatementBegin

-- Note: pg_cron extension scheduling is optional and should be set up separately in production
-- For local development, this function can be called manually or via application cron

-- Create function to permanently delete soft-deleted records older than 90 days
CREATE OR REPLACE FUNCTION cleanup_soft_deleted_records()
RETURNS void AS $$
DECLARE
    deleted_count INTEGER;
    total_deleted INTEGER := 0;
BEGIN
    -- Transactions (older than 90 days)
    DELETE FROM transactions
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % transactions', deleted_count;

    -- Subscriptions (older than 90 days)
    DELETE FROM subscriptions
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % subscriptions', deleted_count;

    -- Chargebacks (older than 90 days)
    DELETE FROM chargebacks
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % chargebacks', deleted_count;

    -- Customer Payment Methods (older than 90 days)
    DELETE FROM customer_payment_methods
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % payment methods', deleted_count;

    -- Agent Credentials (older than 90 days)
    DELETE FROM agent_credentials
    WHERE deleted_at IS NOT NULL
      AND deleted_at < NOW() - INTERVAL '90 days';
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    total_deleted := total_deleted + deleted_count;
    RAISE NOTICE 'Permanently deleted % agent credentials', deleted_count;

    RAISE NOTICE 'Total permanently deleted records: %', total_deleted;
END;
$$ LANGUAGE plpgsql;

-- Note: For production with pg_cron extension, schedule the cleanup job with:
-- SELECT cron.schedule(
--     'cleanup-soft-deleted-records',
--     '0 2 * * *',
--     'SELECT cleanup_soft_deleted_records();'
-- );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Note: If pg_cron scheduling was set up, unschedule with:
-- SELECT cron.unschedule('cleanup-soft-deleted-records');

-- Drop the cleanup function
DROP FUNCTION IF EXISTS cleanup_soft_deleted_records();

-- +goose StatementEnd
