-- +goose Up

-- =============================================================================
-- PRENOTE RETRY TRACKING MIGRATION
-- =============================================================================
-- This migration:
-- 1. Adds PRENOTE to valid transaction types
-- 2. Changes verification_status from pending/verified/failed to verified/unverified
-- 3. Adds prenote tracking columns (prenote_status, prenote_attempts, prenote_next_retry_at)
-- 4. Removes prenote_transaction_id column (prenotes now tracked via transactions.payment_method_id)
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Step 1: Add PRENOTE to transactions type constraint
-- -----------------------------------------------------------------------------
-- Drop and recreate the constraint to include PRENOTE
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_type_valid;
ALTER TABLE transactions ADD CONSTRAINT transactions_type_valid
    CHECK (type IN ('SALE', 'AUTH', 'CAPTURE', 'REFUND', 'VOID', 'STORAGE', 'DEBIT', 'PRENOTE'));

-- Update parent_relationship constraint to allow PRENOTE as standalone (no parent required)
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS parent_relationship;
ALTER TABLE transactions ADD CONSTRAINT parent_relationship CHECK (
    (type IN ('SALE', 'AUTH', 'STORAGE', 'PRENOTE') AND parent_transaction_id IS NULL)
    OR
    (type IN ('CAPTURE', 'REFUND', 'VOID', 'DEBIT') AND parent_transaction_id IS NOT NULL)
);

-- -----------------------------------------------------------------------------
-- Step 2: SKIPPED - verification_status changes handled by migration 027
-- -----------------------------------------------------------------------------
-- Migration 027 replaces verification_status with unified status column
-- (pending, active, failed, expired, revoked) so we skip this step

-- -----------------------------------------------------------------------------
-- Step 3: Add prenote tracking columns to customer_payment_methods
-- -----------------------------------------------------------------------------
ALTER TABLE customer_payment_methods
    ADD COLUMN IF NOT EXISTS prenote_status VARCHAR(20) DEFAULT 'not_required',
    ADD COLUMN IF NOT EXISTS prenote_attempts INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS prenote_next_retry_at TIMESTAMPTZ;

-- Constraint for valid prenote statuses
ALTER TABLE customer_payment_methods
    ADD CONSTRAINT check_prenote_status
    CHECK (prenote_status IN ('not_required', 'pending', 'sent', 'failed', 'max_retries'));

-- Index for prenote retry cron job (only ACH with pending/failed status)
CREATE INDEX IF NOT EXISTS idx_payment_methods_prenote_retry
    ON customer_payment_methods(prenote_status, prenote_next_retry_at)
    WHERE prenote_status IN ('pending', 'failed')
      AND payment_type = 'ach'
      AND deleted_at IS NULL;

-- -----------------------------------------------------------------------------
-- Step 4: Remove prenote_transaction_id column
-- -----------------------------------------------------------------------------
-- Prenotes are now tracked via transactions.payment_method_id FK (1:N relationship)
-- Drop indexes first
DROP INDEX IF EXISTS idx_customer_payment_methods_prenote_transaction;
DROP INDEX IF EXISTS idx_payment_methods_prenote_txn;

-- Drop the column
ALTER TABLE customer_payment_methods DROP COLUMN IF EXISTS prenote_transaction_id;

-- -----------------------------------------------------------------------------
-- Step 5: Migrate existing data
-- -----------------------------------------------------------------------------
-- Set prenote_status = 'sent' for ACH with successful prenote in transactions table
UPDATE customer_payment_methods cpm
SET prenote_status = 'sent'
WHERE cpm.payment_type = 'ach'
  AND cpm.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM transactions t
      WHERE t.payment_method_id = cpm.id
        AND t.type = 'PRENOTE'
        AND t.auth_resp = '00'
  );

-- Set prenote_status = 'pending' for ACH without any successful prenote
-- These will be picked up by the retry cron job
UPDATE customer_payment_methods cpm
SET prenote_status = 'pending',
    prenote_next_retry_at = NOW()
WHERE cpm.payment_type = 'ach'
  AND cpm.deleted_at IS NULL
  AND cpm.prenote_status = 'not_required'
  AND NOT EXISTS (
      SELECT 1 FROM transactions t
      WHERE t.payment_method_id = cpm.id
        AND t.type = 'PRENOTE'
        AND t.auth_resp = '00'
  );

-- Add comments for documentation
COMMENT ON COLUMN customer_payment_methods.prenote_status IS
    'Prenote status: not_required (credit cards), pending (needs send), sent (successful), failed (transient error, needs retry), max_retries (gave up after 5 attempts)';
COMMENT ON COLUMN customer_payment_methods.prenote_attempts IS
    'Number of prenote send attempts (max 5)';
COMMENT ON COLUMN customer_payment_methods.prenote_next_retry_at IS
    'Next scheduled retry time for failed prenotes (exponential backoff)';

-- +goose Down

-- Reverse Step 5: Nothing to reverse (data migration)

-- Reverse Step 4: Re-add prenote_transaction_id column
ALTER TABLE customer_payment_methods
    ADD COLUMN IF NOT EXISTS prenote_transaction_id UUID REFERENCES transactions(id);
CREATE INDEX IF NOT EXISTS idx_customer_payment_methods_prenote_transaction
    ON customer_payment_methods(prenote_transaction_id) WHERE prenote_transaction_id IS NOT NULL;

-- Reverse Step 3: Remove prenote tracking columns
DROP INDEX IF EXISTS idx_payment_methods_prenote_retry;
ALTER TABLE customer_payment_methods
    DROP CONSTRAINT IF EXISTS check_prenote_status,
    DROP COLUMN IF EXISTS prenote_status,
    DROP COLUMN IF EXISTS prenote_attempts,
    DROP COLUMN IF EXISTS prenote_next_retry_at;

-- Reverse Step 2: SKIPPED - handled by migration 027

-- Reverse Step 1: Remove PRENOTE from transaction types
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS parent_relationship;
ALTER TABLE transactions ADD CONSTRAINT parent_relationship CHECK (
    (type IN ('SALE', 'AUTH', 'STORAGE') AND parent_transaction_id IS NULL)
    OR
    (type IN ('CAPTURE', 'REFUND', 'VOID', 'DEBIT') AND parent_transaction_id IS NOT NULL)
);

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_type_valid;
ALTER TABLE transactions ADD CONSTRAINT transactions_type_valid
    CHECK (type IN ('SALE', 'AUTH', 'CAPTURE', 'REFUND', 'VOID', 'STORAGE', 'DEBIT'));
