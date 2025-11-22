-- name: CreatePaymentMethod :one
INSERT INTO customer_payment_methods (
    id, merchant_id, customer_id, payment_type,
    bric, last_four,
    card_brand, card_exp_month, card_exp_year,
    bank_name, account_type,
    is_default, is_active, is_verified,
    verification_status, prenote_transaction_id
) VALUES (
    sqlc.arg(id), sqlc.arg(merchant_id), sqlc.arg(customer_id), sqlc.arg(payment_type),
    sqlc.arg(bric), sqlc.arg(last_four),
    sqlc.narg(card_brand), sqlc.narg(card_exp_month), sqlc.narg(card_exp_year),
    sqlc.narg(bank_name), sqlc.narg(account_type),
    sqlc.arg(is_default), sqlc.arg(is_active), sqlc.arg(is_verified),
    sqlc.arg(verification_status), sqlc.narg(prenote_transaction_id)
) RETURNING *;

-- name: GetPaymentMethodByID :one
SELECT * FROM customer_payment_methods
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListPaymentMethodsByCustomer :many
SELECT * FROM customer_payment_methods
WHERE merchant_id = sqlc.arg(merchant_id) AND customer_id = sqlc.arg(customer_id) AND deleted_at IS NULL
ORDER BY is_default DESC, created_at DESC;

-- name: ListPaymentMethods :many
SELECT * FROM customer_payment_methods
WHERE
    deleted_at IS NULL AND
    (sqlc.narg(merchant_id)::uuid IS NULL OR merchant_id = sqlc.narg(merchant_id)) AND
    (sqlc.narg(customer_id)::varchar IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(payment_type)::varchar IS NULL OR payment_type = sqlc.narg(payment_type)) AND
    (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active)) AND
    (sqlc.narg(is_default)::boolean IS NULL OR is_default = sqlc.narg(is_default))
ORDER BY is_default DESC, created_at DESC;

-- name: GetDefaultPaymentMethod :one
SELECT * FROM customer_payment_methods
WHERE merchant_id = sqlc.arg(merchant_id) AND customer_id = sqlc.arg(customer_id) AND is_default = true AND is_active = true AND deleted_at IS NULL
LIMIT 1;

-- name: SetPaymentMethodAsDefault :exec
-- First unset all defaults for this customer
UPDATE customer_payment_methods
SET is_default = false, updated_at = CURRENT_TIMESTAMP
WHERE merchant_id = sqlc.arg(merchant_id) AND customer_id = sqlc.arg(customer_id) AND deleted_at IS NULL;

-- name: MarkPaymentMethodAsDefault :exec
-- Then set the specified one as default
UPDATE customer_payment_methods
SET is_default = true, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: MarkPaymentMethodUsed :exec
UPDATE customer_payment_methods
SET last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: MarkPaymentMethodVerified :exec
UPDATE customer_payment_methods
SET is_verified = true,
    verification_status = 'verified',
    verified_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: DeactivatePaymentMethod :exec
UPDATE customer_payment_methods
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: DeactivatePaymentMethodWithReason :exec
UPDATE customer_payment_methods
SET is_active = false,
    deactivation_reason = sqlc.arg(deactivation_reason),
    deactivated_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ActivatePaymentMethod :exec
UPDATE customer_payment_methods
SET is_active = true, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: DeletePaymentMethod :exec
UPDATE customer_payment_methods
SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- ACH Verification Management Queries

-- name: GetPendingACHVerifications :many
-- Get ACH payment methods pending verification older than specified cutoff date
-- Used by cron job to mark accounts as verified after 3 days with no returns
SELECT * FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND verification_status = 'pending'
  AND created_at < sqlc.arg(cutoff_date)
  AND deleted_at IS NULL
ORDER BY created_at ASC
LIMIT sqlc.arg(limit_count);

-- name: UpdateVerificationStatus :exec
-- Update verification status and related fields
UPDATE customer_payment_methods
SET verification_status = sqlc.arg(verification_status),
    is_verified = CASE
        WHEN sqlc.arg(verification_status)::varchar = 'verified' THEN true
        ELSE false
    END,
    verified_at = CASE
        WHEN sqlc.arg(verification_status)::varchar = 'verified' THEN CURRENT_TIMESTAMP
        ELSE verified_at
    END,
    verification_failure_reason = sqlc.narg(verification_failure_reason),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: IncrementReturnCount :exec
-- Increment ACH return count and optionally deactivate if threshold reached
UPDATE customer_payment_methods
SET return_count = return_count + 1,
    is_active = CASE
        WHEN return_count + 1 >= sqlc.arg(deactivation_threshold) THEN false
        ELSE is_active
    END,
    deactivation_reason = CASE
        WHEN return_count + 1 >= sqlc.arg(deactivation_threshold) THEN 'excessive_returns'
        ELSE deactivation_reason
    END,
    deactivated_at = CASE
        WHEN return_count + 1 >= sqlc.arg(deactivation_threshold) THEN CURRENT_TIMESTAMP
        ELSE deactivated_at
    END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetPaymentMethodByPreNoteTransaction :one
-- Get payment method by pre-note transaction ID (for return code processing)
SELECT * FROM customer_payment_methods
WHERE prenote_transaction_id = sqlc.arg(prenote_transaction_id)
  AND deleted_at IS NULL
LIMIT 1;

-- name: MarkVerificationFailed :exec
-- Mark ACH verification as failed and deactivate payment method
UPDATE customer_payment_methods
SET verification_status = 'failed',
    is_verified = false,
    is_active = false,
    verification_failure_reason = sqlc.arg(failure_reason),
    deactivation_reason = 'verification_failed',
    deactivated_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- ACH Statistics Queries

-- name: CountTotalACH :one
-- Count total ACH payment methods (not deleted)
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach' AND deleted_at IS NULL;

-- name: CountPendingACH :one
-- Count ACH payment methods pending verification
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND verification_status = 'pending'
  AND deleted_at IS NULL;

-- name: CountVerifiedACH :one
-- Count verified ACH payment methods
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND verification_status = 'verified'
  AND deleted_at IS NULL;

-- name: CountFailedACH :one
-- Count failed ACH payment methods
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND verification_status = 'failed'
  AND deleted_at IS NULL;

-- name: CountEligibleACH :one
-- Count ACH payment methods eligible for verification (pending > cutoff date)
SELECT COUNT(*) FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND verification_status = 'pending'
  AND created_at <= sqlc.arg(cutoff_date)
  AND deleted_at IS NULL;

-- name: FindEligibleACHForVerification :many
-- Find ACH payment methods eligible for verification
SELECT id, merchant_id, customer_id, payment_type
FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND verification_status = 'pending'
  AND created_at <= sqlc.arg(cutoff_date)
  AND deleted_at IS NULL
ORDER BY created_at ASC
LIMIT sqlc.arg(batch_limit);

-- name: VerifyACHPaymentMethod :execresult
-- Mark an ACH payment method as verified and activate it
UPDATE customer_payment_methods
SET verification_status = 'verified',
    is_verified = true,
    is_active = true,
    verified_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND verification_status = 'pending'
  AND payment_type = 'ach'
  AND deleted_at IS NULL;
