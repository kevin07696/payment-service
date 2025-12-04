-- +goose Up
-- Migration: Add billing retry backoff support to subscriptions
-- Purpose: Enable exponential backoff for transient billing failures (network issues)

-- Add next_billing_retry_at: scheduled time for next billing retry attempt
-- Only set when billing fails due to transient errors (network, timeout, gateway unavailable)
-- NULL means no backoff - subscription is eligible for billing on next cron run
ALTER TABLE subscriptions
ADD COLUMN IF NOT EXISTS next_billing_retry_at TIMESTAMPTZ;

-- Add last_billing_error: stores the last billing error for debugging
ALTER TABLE subscriptions
ADD COLUMN IF NOT EXISTS last_billing_error VARCHAR(500);

-- Add last_billing_error_at: when the last error occurred
ALTER TABLE subscriptions
ADD COLUMN IF NOT EXISTS last_billing_error_at TIMESTAMPTZ;

-- Create index for efficient billing retry queries
-- Finds subscriptions where: status = 'active' AND next_billing_retry_at <= now()
CREATE INDEX IF NOT EXISTS idx_subscriptions_billing_retry
ON subscriptions (status, next_billing_retry_at)
WHERE status = 'active' AND next_billing_retry_at IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN subscriptions.next_billing_retry_at IS 'Next scheduled billing retry time. Set only for transient errors (network issues). NULL means eligible for immediate retry.';
COMMENT ON COLUMN subscriptions.last_billing_error IS 'Last billing error message for debugging.';
COMMENT ON COLUMN subscriptions.last_billing_error_at IS 'Timestamp of last billing error.';

-- +goose Down
-- Rollback: Remove billing retry backoff columns

DROP INDEX IF EXISTS idx_subscriptions_billing_retry;

ALTER TABLE subscriptions
DROP COLUMN IF EXISTS last_billing_error_at;

ALTER TABLE subscriptions
DROP COLUMN IF EXISTS last_billing_error;

ALTER TABLE subscriptions
DROP COLUMN IF EXISTS next_billing_retry_at;
