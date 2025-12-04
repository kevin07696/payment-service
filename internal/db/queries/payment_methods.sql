-- name: CreatePaymentMethod :one
INSERT INTO customer_payment_methods (
    id, merchant_id, customer_id, payment_type,
    bric, last_four,
    card_brand, card_exp_month, card_exp_year,
    bank_name, account_type,
    is_default, status, prenote_status
) VALUES (
    sqlc.arg(id), sqlc.arg(merchant_id), sqlc.arg(customer_id), sqlc.arg(payment_type),
    sqlc.arg(bric), sqlc.arg(last_four),
    sqlc.narg(card_brand), sqlc.narg(card_exp_month), sqlc.narg(card_exp_year),
    sqlc.narg(bank_name), sqlc.narg(account_type),
    sqlc.arg(is_default), sqlc.arg(status), sqlc.arg(prenote_status)
) RETURNING *;

-- name: GetPaymentMethodByID :one
SELECT * FROM customer_payment_methods
WHERE id = sqlc.arg(id);

-- name: ListPaymentMethodsByCustomer :many
SELECT * FROM customer_payment_methods
WHERE merchant_id = sqlc.arg(merchant_id) AND customer_id = sqlc.arg(customer_id)
ORDER BY is_default DESC, created_at DESC;

-- name: ListPaymentMethods :many
SELECT * FROM customer_payment_methods
WHERE
    (sqlc.narg(merchant_id)::uuid IS NULL OR merchant_id = sqlc.narg(merchant_id)) AND
    (sqlc.narg(customer_id)::varchar IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(payment_type)::varchar IS NULL OR payment_type = sqlc.narg(payment_type)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)) AND
    (sqlc.narg(is_default)::boolean IS NULL OR is_default = sqlc.narg(is_default))
ORDER BY is_default DESC, created_at DESC;

-- name: ListActivePaymentMethodsByCustomer :many
SELECT * FROM customer_payment_methods
WHERE merchant_id = sqlc.arg(merchant_id)
  AND customer_id = sqlc.arg(customer_id)
  AND status = 'active'
ORDER BY is_default DESC, created_at DESC;

-- name: GetDefaultPaymentMethod :one
SELECT * FROM customer_payment_methods
WHERE merchant_id = sqlc.arg(merchant_id)
  AND customer_id = sqlc.arg(customer_id)
  AND is_default = true
  AND status = 'active'
LIMIT 1;

-- name: SetPaymentMethodAsDefault :exec
-- First unset all defaults for this customer
UPDATE customer_payment_methods
SET is_default = false, updated_at = CURRENT_TIMESTAMP
WHERE merchant_id = sqlc.arg(merchant_id) AND customer_id = sqlc.arg(customer_id);

-- name: MarkPaymentMethodAsDefault :exec
-- Then set the specified one as default
UPDATE customer_payment_methods
SET is_default = true, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: MarkPaymentMethodUsed :exec
UPDATE customer_payment_methods
SET last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- =============================================================================
-- STATUS MANAGEMENT
-- =============================================================================

-- name: UpdatePaymentMethodStatus :exec
-- Update payment method status with reason and timestamp
UPDATE customer_payment_methods
SET status = sqlc.arg(status),
    status_reason = sqlc.arg(status_reason),
    status_changed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: ActivatePaymentMethod :exec
-- Mark payment method as active (verified and usable)
UPDATE customer_payment_methods
SET status = 'active',
    status_reason = sqlc.arg(status_reason),
    status_changed_at = CURRENT_TIMESTAMP,
    verified_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: RevokePaymentMethod :exec
-- Manually revoke a payment method
UPDATE customer_payment_methods
SET status = 'revoked',
    status_reason = sqlc.arg(status_reason),
    status_changed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: FailPaymentMethod :exec
-- Mark payment method as failed (verification failed)
UPDATE customer_payment_methods
SET status = 'failed',
    status_reason = sqlc.arg(status_reason),
    status_changed_at = CURRENT_TIMESTAMP,
    verification_failure_reason = sqlc.arg(status_reason),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: ExpirePaymentMethod :exec
-- Mark payment method as expired (card past expiration)
UPDATE customer_payment_methods
SET status = 'expired',
    status_reason = 'card_expired',
    status_changed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- =============================================================================
-- DELETION (Hard delete with FK protection)
-- =============================================================================

-- name: HardDeletePaymentMethod :exec
-- Hard delete payment method
-- Will fail with FK violation if transactions exist (ON DELETE RESTRICT)
-- Subscriptions will have payment_method_id set to NULL (ON DELETE SET NULL)
DELETE FROM customer_payment_methods WHERE id = sqlc.arg(id);

-- name: FindPaymentMethodsForHardDelete :many
-- Find terminal status payment methods older than 7 days for cleanup
SELECT * FROM customer_payment_methods
WHERE status IN ('failed', 'expired', 'revoked')
  AND status_changed_at < NOW() - INTERVAL '7 days'
LIMIT sqlc.arg(batch_limit);

-- =============================================================================
-- ACH VERIFICATION MANAGEMENT
-- =============================================================================

-- name: GetPendingACHVerifications :many
-- Get ACH payment methods pending verification older than specified cutoff date
-- Used by cron job to mark accounts as verified after 3 days with no returns
SELECT * FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND status = 'pending'
  AND prenote_status = 'sent'
  AND created_at < sqlc.arg(cutoff_date)
ORDER BY created_at ASC
LIMIT sqlc.arg(limit_count);

-- name: IncrementReturnCount :exec
-- Increment ACH return count and update status to failed if threshold reached
UPDATE customer_payment_methods
SET return_count = return_count + 1,
    status = CASE
        WHEN return_count + 1 >= sqlc.arg(deactivation_threshold) THEN 'failed'
        ELSE status
    END,
    status_reason = CASE
        WHEN return_count + 1 >= sqlc.arg(deactivation_threshold) THEN 'excessive_returns'
        ELSE status_reason
    END,
    status_changed_at = CASE
        WHEN return_count + 1 >= sqlc.arg(deactivation_threshold) THEN CURRENT_TIMESTAMP
        ELSE status_changed_at
    END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: GetPaymentMethodByPrenoteTransaction :one
-- Get payment method that has a prenote transaction (for return code processing)
SELECT cpm.* FROM customer_payment_methods cpm
WHERE EXISTS (
    SELECT 1 FROM transactions t
    WHERE t.payment_method_id = cpm.id
      AND t.type = 'PRENOTE'
      AND t.id = sqlc.arg(prenote_transaction_id)
)
LIMIT 1;

-- =============================================================================
-- ACH STATISTICS
-- =============================================================================

-- name: CountTotalACH :one
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach';

-- name: CountPendingACH :one
-- Count ACH payment methods pending verification
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach' AND status = 'pending';

-- name: CountActiveACH :one
-- Count active/verified ACH payment methods
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach' AND status = 'active';

-- name: CountFailedACH :one
-- Count failed ACH payment methods
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach' AND status = 'failed';

-- name: CountEligibleACH :one
-- Count ACH payment methods eligible for verification (pending with prenote sent, past cutoff date)
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND status = 'pending'
  AND prenote_status = 'sent'
  AND created_at <= sqlc.arg(cutoff_date);

-- name: FindEligibleACHForVerification :many
-- Find ACH payment methods eligible for verification
SELECT id, merchant_id, customer_id, payment_type, bric
FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND status = 'pending'
  AND prenote_status = 'sent'
  AND created_at <= sqlc.arg(cutoff_date)
ORDER BY created_at ASC
LIMIT sqlc.arg(batch_limit);

-- name: VerifyACHPaymentMethod :execresult
-- Mark an ACH payment method as verified/active
UPDATE customer_payment_methods
SET status = 'active',
    status_reason = 'ach_verified',
    status_changed_at = NOW(),
    verified_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND status = 'pending'
  AND prenote_status = 'sent'
  AND payment_type = 'ach';

-- =============================================================================
-- PRENOTE RETRY
-- =============================================================================

-- name: GetACHNeedingPrenoteRetry :many
-- Find ACH payment methods that need prenote retry
SELECT cpm.*, m.cust_nbr, m.merch_nbr, m.dba_nbr, m.terminal_nbr
FROM customer_payment_methods cpm
JOIN merchants m ON cpm.merchant_id = m.id
WHERE cpm.payment_type = 'ach'
  AND m.is_active = true
  AND cpm.prenote_status IN ('pending', 'failed')
  AND (cpm.prenote_next_retry_at IS NULL OR cpm.prenote_next_retry_at <= NOW())
  AND cpm.prenote_attempts < sqlc.arg(max_attempts)
ORDER BY cpm.prenote_next_retry_at ASC NULLS FIRST, cpm.created_at ASC
LIMIT sqlc.arg(batch_limit);

-- name: UpdatePrenoteStatusSuccess :exec
-- Mark prenote as successfully sent
UPDATE customer_payment_methods
SET prenote_status = 'sent',
    prenote_attempts = prenote_attempts + 1,
    prenote_next_retry_at = NULL,
    updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: UpdatePrenoteStatusFailed :exec
-- Mark prenote as failed with next retry time (for transient errors)
UPDATE customer_payment_methods
SET prenote_status = 'failed',
    prenote_attempts = prenote_attempts + 1,
    prenote_next_retry_at = sqlc.arg(next_retry_at),
    updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: UpdatePrenoteStatusMaxRetries :exec
-- Mark prenote as max retries exceeded (fail payment method)
UPDATE customer_payment_methods
SET prenote_status = 'max_retries',
    prenote_attempts = prenote_attempts + 1,
    prenote_next_retry_at = NULL,
    status = 'failed',
    status_reason = 'prenote_max_retries',
    status_changed_at = NOW(),
    verification_failure_reason = 'prenote_max_retries_exceeded',
    updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: UpdatePrenoteStatusPermanentFailure :exec
-- Mark prenote as permanently failed due to user/data error (no retry, fail PM)
UPDATE customer_payment_methods
SET prenote_status = 'max_retries',
    prenote_attempts = prenote_attempts + 1,
    prenote_next_retry_at = NULL,
    status = 'failed',
    status_reason = sqlc.arg(status_reason),
    status_changed_at = NOW(),
    verification_failure_reason = sqlc.arg(status_reason),
    updated_at = NOW()
WHERE id = sqlc.arg(id);

-- name: CountPrenotesByStatus :many
-- For monitoring: count payment methods by prenote status
SELECT prenote_status, COUNT(*) as count
FROM customer_payment_methods
WHERE payment_type = 'ach'
GROUP BY prenote_status;

-- name: GetPrenoteAttemptsForPaymentMethod :many
-- Get all prenote transaction attempts for a payment method
SELECT * FROM transactions
WHERE payment_method_id = sqlc.arg(payment_method_id)
  AND type = 'PRENOTE'
ORDER BY created_at DESC;

-- =============================================================================
-- CARD EXPIRATION
-- =============================================================================

-- name: FindExpiredCreditCards :many
-- Find active credit cards that are past expiration
SELECT * FROM customer_payment_methods
WHERE payment_type = 'card'
  AND status = 'active'
  AND card_exp_year IS NOT NULL
  AND card_exp_month IS NOT NULL
  AND make_date(card_exp_year, card_exp_month, 1) < CURRENT_DATE
LIMIT sqlc.arg(batch_limit);

-- name: CountByStatus :many
-- For monitoring: count payment methods by status
SELECT status, COUNT(*) as count
FROM customer_payment_methods
GROUP BY status;
