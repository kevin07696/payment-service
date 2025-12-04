-- +goose Up
-- Migration: Add past_due tracking columns to subscriptions table
-- Purpose: Enable automatic cancellation of subscriptions after grace period expires

-- Add past_due_since: tracks when subscription entered past_due status
-- This is set when status changes from 'active' to 'past_due'
ALTER TABLE subscriptions
ADD COLUMN IF NOT EXISTS past_due_since TIMESTAMPTZ;

-- Add grace_period_days: configurable grace period before auto-cancellation
-- Default 30 days - can be overridden per subscription
ALTER TABLE subscriptions
ADD COLUMN IF NOT EXISTS grace_period_days INTEGER NOT NULL DEFAULT 30;

-- Add cancellation_reason: tracks why subscription was cancelled
-- Values: 'user_requested', 'payment_failed', 'grace_period_expired', 'admin'
ALTER TABLE subscriptions
ADD COLUMN IF NOT EXISTS cancellation_reason VARCHAR(50);

-- Create index for efficient past_due cleanup queries
-- Finds subscriptions where: status = 'past_due' AND past_due_since + grace_period_days < now()
CREATE INDEX IF NOT EXISTS idx_subscriptions_past_due_cleanup
ON subscriptions (status, past_due_since)
WHERE status = 'past_due' AND past_due_since IS NOT NULL;

-- Backfill: Set past_due_since for existing past_due subscriptions
-- Use updated_at as approximation since we don't have historical data
UPDATE subscriptions
SET past_due_since = updated_at
WHERE status = 'past_due' AND past_due_since IS NULL;

-- Add comment for documentation
COMMENT ON COLUMN subscriptions.past_due_since IS 'Timestamp when subscription entered past_due status. Used for grace period calculation.';
COMMENT ON COLUMN subscriptions.grace_period_days IS 'Number of days to wait before auto-cancelling a past_due subscription. Default 30.';
COMMENT ON COLUMN subscriptions.cancellation_reason IS 'Reason for cancellation: user_requested, payment_failed, grace_period_expired, admin';

-- +goose Down
-- Rollback: Remove past_due tracking columns

DROP INDEX IF EXISTS idx_subscriptions_past_due_cleanup;

ALTER TABLE subscriptions
DROP COLUMN IF EXISTS cancellation_reason;

ALTER TABLE subscriptions
DROP COLUMN IF EXISTS grace_period_days;

ALTER TABLE subscriptions
DROP COLUMN IF EXISTS past_due_since;
