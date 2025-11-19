-- +goose Up
-- +goose StatementBegin

-- Add verification tracking fields for ACH payment methods
ALTER TABLE customer_payment_methods
    ADD COLUMN verification_status VARCHAR(20) DEFAULT 'pending',
    ADD COLUMN prenote_transaction_id UUID,
    ADD COLUMN verified_at TIMESTAMPTZ,
    ADD COLUMN verification_failure_reason TEXT,
    ADD COLUMN return_count INTEGER DEFAULT 0 NOT NULL,
    ADD COLUMN deactivation_reason VARCHAR(100),
    ADD COLUMN deactivated_at TIMESTAMPTZ;

-- Add check constraint for verification_status
ALTER TABLE customer_payment_methods
    ADD CONSTRAINT check_verification_status
    CHECK (verification_status IN ('pending', 'verified', 'failed'));

-- Add check constraint for return_count
ALTER TABLE customer_payment_methods
    ADD CONSTRAINT check_return_count
    CHECK (return_count >= 0);

-- Create index for pending verifications (for cron job)
CREATE INDEX idx_customer_payment_methods_pending_verification
    ON customer_payment_methods(verification_status, created_at)
    WHERE verification_status = 'pending' AND payment_type = 'ach';

-- Create index for prenote transaction lookups
CREATE INDEX idx_customer_payment_methods_prenote_transaction
    ON customer_payment_methods(prenote_transaction_id)
    WHERE prenote_transaction_id IS NOT NULL;

-- Update existing records to have proper verification_status
-- Credit cards are always considered verified (no pre-note required)
UPDATE customer_payment_methods
SET verification_status = 'verified',
    verified_at = created_at
WHERE payment_type = 'credit_card';

-- Existing ACH payment methods with is_verified=true should be marked as verified
UPDATE customer_payment_methods
SET verification_status = 'verified',
    verified_at = created_at
WHERE payment_type = 'ach' AND is_verified = true;

-- Existing ACH payment methods with is_verified=false should stay pending
UPDATE customer_payment_methods
SET verification_status = 'pending'
WHERE payment_type = 'ach' AND is_verified = false;

-- Add comment explaining verification flow
COMMENT ON COLUMN customer_payment_methods.verification_status IS
'ACH verification status: pending (pre-note sent, awaiting clearance), verified (pre-note cleared after 3 days), failed (return code received)';

COMMENT ON COLUMN customer_payment_methods.prenote_transaction_id IS
'Links to the pre-note (CKC0) transaction used for ACH verification';

COMMENT ON COLUMN customer_payment_methods.verified_at IS
'Timestamp when ACH verification completed (3 days after pre-note with no returns)';

COMMENT ON COLUMN customer_payment_methods.verification_failure_reason IS
'Reason for verification failure (e.g., "R03: No Account/Unable to Locate")';

COMMENT ON COLUMN customer_payment_methods.return_count IS
'Number of ACH returns received. Auto-deactivate after 2+ returns';

COMMENT ON COLUMN customer_payment_methods.deactivation_reason IS
'Reason for deactivation (e.g., "excessive_returns", "manual_deactivation")';

COMMENT ON COLUMN customer_payment_methods.deactivated_at IS
'Timestamp when payment method was deactivated';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove indexes
DROP INDEX IF EXISTS idx_customer_payment_methods_pending_verification;
DROP INDEX IF EXISTS idx_customer_payment_methods_prenote_transaction;

-- Remove constraints
ALTER TABLE customer_payment_methods DROP CONSTRAINT IF EXISTS check_verification_status;
ALTER TABLE customer_payment_methods DROP CONSTRAINT IF EXISTS check_return_count;

-- Remove columns
ALTER TABLE customer_payment_methods
    DROP COLUMN IF EXISTS verification_status,
    DROP COLUMN IF EXISTS prenote_transaction_id,
    DROP COLUMN IF EXISTS verified_at,
    DROP COLUMN IF EXISTS verification_failure_reason,
    DROP COLUMN IF EXISTS return_count,
    DROP COLUMN IF EXISTS deactivation_reason,
    DROP COLUMN IF EXISTS deactivated_at;

-- +goose StatementEnd
