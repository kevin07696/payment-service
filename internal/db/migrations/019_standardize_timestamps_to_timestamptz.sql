-- +goose Up
-- +goose StatementBegin
-- Standardize all TIMESTAMP columns to TIMESTAMPTZ for timezone consistency
-- This fixes critical timezone handling issues across all tables
-- Assumes existing data is in UTC (safest assumption for database timestamps)

-- Fix merchants table
ALTER TABLE merchants
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC',
  ALTER COLUMN deleted_at TYPE TIMESTAMPTZ USING deleted_at AT TIME ZONE 'UTC',
  ALTER COLUMN approved_at TYPE TIMESTAMPTZ USING approved_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN merchants.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.updated_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.deleted_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchants.approved_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix services table (auth)
ALTER TABLE services
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN services.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN services.updated_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix service_merchants table (auth) - uses granted_at and expires_at, not created_at
ALTER TABLE service_merchants
  ALTER COLUMN granted_at TYPE TIMESTAMPTZ USING granted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN service_merchants.granted_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN service_merchants.expires_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix admins table (auth)
ALTER TABLE admins
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN admins.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN admins.updated_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix admin_sessions table
ALTER TABLE admin_sessions
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN admin_sessions.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN admin_sessions.expires_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Skip audit_logs table - it's partitioned by performed_at (partition key cannot be altered)
-- Partitioned tables require recreating the entire partition structure to change key column types
-- This is acceptable as audit_logs is for logging and timezone inconsistency is less critical
-- Future partitions should be created with TIMESTAMPTZ from the start

-- Fix jwt_blacklist table
ALTER TABLE jwt_blacklist
  ALTER COLUMN blacklisted_at TYPE TIMESTAMPTZ USING blacklisted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN jwt_blacklist.blacklisted_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN jwt_blacklist.expires_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix epx_ip_whitelist table
ALTER TABLE epx_ip_whitelist
  ALTER COLUMN added_at TYPE TIMESTAMPTZ USING added_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN epx_ip_whitelist.added_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix merchant_activation_tokens table
ALTER TABLE merchant_activation_tokens
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN used_at TYPE TIMESTAMPTZ USING used_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN merchant_activation_tokens.created_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchant_activation_tokens.expires_at IS 'Timezone-aware timestamp (stored as UTC)';
COMMENT ON COLUMN merchant_activation_tokens.used_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Fix rate_limit_buckets table
ALTER TABLE rate_limit_buckets
  ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC';

COMMENT ON COLUMN rate_limit_buckets.created_at IS 'Timezone-aware timestamp (stored as UTC)';

-- Verify all timestamp columns are now timezone-aware (except audit_logs partitions)
DO $$
DECLARE
    non_tz_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO non_tz_count
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND column_name LIKE '%_at'
      AND data_type = 'timestamp without time zone'
      AND table_name NOT LIKE 'audit_logs%'; -- Exclude audit_logs and its partitions

    IF non_tz_count > 0 THEN
        RAISE EXCEPTION 'Migration failed: % columns still using TIMESTAMP without timezone', non_tz_count;
    END IF;

    RAISE NOTICE 'SUCCESS: All timestamp columns are now timezone-aware (TIMESTAMPTZ), except audit_logs partitions';
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Revert to TIMESTAMP (not recommended, loses timezone information)
-- Only use this for rollback in case of issues

ALTER TABLE merchants
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
  ALTER COLUMN deleted_at TYPE TIMESTAMP USING deleted_at AT TIME ZONE 'UTC',
  ALTER COLUMN approved_at TYPE TIMESTAMP USING approved_at AT TIME ZONE 'UTC';

ALTER TABLE services
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE service_merchants
  ALTER COLUMN granted_at TYPE TIMESTAMP USING granted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE admins
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE admin_sessions
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC';

-- Skip audit_logs table (partitioned, cannot alter partition key)

ALTER TABLE jwt_blacklist
  ALTER COLUMN blacklisted_at TYPE TIMESTAMP USING blacklisted_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE epx_ip_whitelist
  ALTER COLUMN added_at TYPE TIMESTAMP USING added_at AT TIME ZONE 'UTC';

ALTER TABLE merchant_activation_tokens
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC',
  ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC',
  ALTER COLUMN used_at TYPE TIMESTAMP USING used_at AT TIME ZONE 'UTC';

ALTER TABLE rate_limit_buckets
  ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';
-- +goose StatementEnd
