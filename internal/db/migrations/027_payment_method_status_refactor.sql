-- +goose Up
-- Migration: Refactor payment method status fields
-- Replaces is_active, is_verified, deactivated_at, deactivation_reason, verification_status
-- with single status enum + status_reason + status_changed_at

-- Step 1: Add new columns
ALTER TABLE customer_payment_methods
ADD COLUMN IF NOT EXISTS status TEXT,
ADD COLUMN IF NOT EXISTS status_reason TEXT,
ADD COLUMN IF NOT EXISTS status_changed_at TIMESTAMPTZ;

-- Step 2: Migrate existing data
UPDATE customer_payment_methods SET
    status = CASE
        WHEN is_active = false THEN 'revoked'
        WHEN is_verified = false AND payment_type = 'ach' THEN 'pending'
        WHEN payment_type = 'card' AND card_exp_year IS NOT NULL
             AND make_date(card_exp_year, card_exp_month, 1) < CURRENT_DATE THEN 'expired'
        ELSE 'active'
    END,
    status_reason = CASE
        WHEN is_active = false THEN COALESCE(deactivation_reason, 'legacy_migration')
        WHEN verification_failure_reason IS NOT NULL THEN verification_failure_reason
        ELSE NULL
    END,
    status_changed_at = COALESCE(deactivated_at, verified_at, updated_at);

-- Step 3: Add CHECK constraint and make NOT NULL
ALTER TABLE customer_payment_methods
ADD CONSTRAINT check_payment_method_status
CHECK (status IN ('pending', 'active', 'failed', 'expired', 'revoked'));

ALTER TABLE customer_payment_methods
ALTER COLUMN status SET NOT NULL,
ALTER COLUMN status SET DEFAULT 'pending';

-- Step 4: Drop old columns (including deleted_at since we do hard delete now)
ALTER TABLE customer_payment_methods
DROP COLUMN IF EXISTS is_active,
DROP COLUMN IF EXISTS is_verified,
DROP COLUMN IF EXISTS deactivated_at,
DROP COLUMN IF EXISTS deactivation_reason,
DROP COLUMN IF EXISTS verification_status,
DROP COLUMN IF EXISTS deleted_at;

-- Step 5: Update FK constraints for deletion protection
-- Transactions: RESTRICT (can't delete PM if transactions exist)
ALTER TABLE transactions
DROP CONSTRAINT IF EXISTS transactions_payment_method_id_fkey;

ALTER TABLE transactions
ADD CONSTRAINT transactions_payment_method_id_fkey
    FOREIGN KEY (payment_method_id) REFERENCES customer_payment_methods(id)
    ON DELETE RESTRICT;

-- Subscriptions: SET NULL (nulls subscription's PM, triggers payment_method_required flow)
ALTER TABLE subscriptions
DROP CONSTRAINT IF EXISTS subscriptions_payment_method_id_fkey;

ALTER TABLE subscriptions
ADD CONSTRAINT subscriptions_payment_method_id_fkey
    FOREIGN KEY (payment_method_id) REFERENCES customer_payment_methods(id)
    ON DELETE SET NULL;

-- Step 6: Add index for cleanup cron queries
CREATE INDEX IF NOT EXISTS idx_payment_methods_status_changed
ON customer_payment_methods (status, status_changed_at)
WHERE status IN ('failed', 'expired', 'revoked');

-- Step 7: Add index for expired card detection
CREATE INDEX IF NOT EXISTS idx_payment_methods_card_expiration
ON customer_payment_methods (payment_type, card_exp_year, card_exp_month)
WHERE payment_type = 'card' AND status = 'active';
