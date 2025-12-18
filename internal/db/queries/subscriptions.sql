-- name: CreateSubscription :one
INSERT INTO subscriptions (
    id, merchant_id, customer_id, amount_cents, currency,
    interval_value, interval_unit, status,
    payment_method_id, next_billing_date,
    failure_retry_count, max_retries,
    gateway_subscription_id, metadata
) VALUES (
    sqlc.arg(id), sqlc.arg(merchant_id), sqlc.arg(customer_id), sqlc.arg(amount_cents), sqlc.arg(currency),
    sqlc.arg(interval_value), sqlc.arg(interval_unit), sqlc.arg(status),
    sqlc.arg(payment_method_id), sqlc.arg(next_billing_date),
    sqlc.arg(failure_retry_count), sqlc.arg(max_retries),
    sqlc.narg(gateway_subscription_id), sqlc.arg(metadata)
) RETURNING *;

-- name: GetSubscriptionByID :one
SELECT * FROM subscriptions
WHERE id = sqlc.arg(id);

-- name: GetSubscriptionByIDForUpdate :one
-- Locks the row for update to prevent race conditions during concurrent modifications
SELECT * FROM subscriptions
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: ListSubscriptionsByCustomer :many
SELECT * FROM subscriptions
WHERE merchant_id = sqlc.arg(merchant_id) AND customer_id = sqlc.arg(customer_id)
ORDER BY created_at DESC;

-- name: ListSubscriptions :many
SELECT * FROM subscriptions
WHERE
    (sqlc.narg(merchant_id)::uuid IS NULL OR merchant_id = sqlc.narg(merchant_id)) AND
    (sqlc.narg(customer_id)::varchar IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_val) OFFSET sqlc.arg(offset_val);

-- name: CountSubscriptions :one
SELECT COUNT(*) FROM subscriptions
WHERE
    (sqlc.narg(merchant_id)::uuid IS NULL OR merchant_id = sqlc.narg(merchant_id)) AND
    (sqlc.narg(customer_id)::varchar IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status));

-- name: ListDueSubscriptions :many
SELECT * FROM subscriptions
WHERE status = 'active' AND next_billing_date <= sqlc.arg(as_of_date)
ORDER BY next_billing_date ASC
LIMIT sqlc.arg(limit_val);

-- name: UpdateSubscription :one
UPDATE subscriptions
SET
    amount_cents = sqlc.arg(amount_cents),
    interval_value = sqlc.arg(interval_value),
    interval_unit = sqlc.arg(interval_unit),
    payment_method_id = sqlc.arg(payment_method_id),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateSubscriptionStatus :one
UPDATE subscriptions
SET status = sqlc.arg(status), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateNextBillingDate :exec
UPDATE subscriptions
SET next_billing_date = sqlc.arg(next_billing_date), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: IncrementSubscriptionRetryCount :exec
UPDATE subscriptions
SET failure_retry_count = failure_retry_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: ResetSubscriptionRetryCount :exec
UPDATE subscriptions
SET failure_retry_count = 0, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: CancelSubscription :one
UPDATE subscriptions
SET status = sqlc.arg(status), cancelled_at = sqlc.narg(canceled_at), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListSubscriptionsDueForBilling :many
-- Lists subscriptions due for billing, respecting retry backoff for transient errors.
-- A subscription is eligible if:
--   1. status = 'active' AND next_billing_date <= as_of_date
--   2. AND (no backoff scheduled OR backoff time has passed)
SELECT * FROM subscriptions
WHERE status = 'active'
  AND next_billing_date <= sqlc.arg(next_billing_date)
  AND (next_billing_retry_at IS NULL OR next_billing_retry_at <= CURRENT_TIMESTAMP)
ORDER BY next_billing_date ASC
LIMIT sqlc.arg(limit_val);

-- name: UpdateSubscriptionBilling :one
UPDATE subscriptions
SET
    next_billing_date = sqlc.arg(next_billing_date),
    failure_retry_count = sqlc.arg(failure_retry_count),
    status = sqlc.arg(status),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: IncrementSubscriptionFailureCount :one
-- Updates failure count and manages past_due_since timestamp:
-- - Sets past_due_since when transitioning TO past_due (fresh grace period)
-- - Clears past_due_since when leaving past_due status (e.g., successful retry)
UPDATE subscriptions
SET
    failure_retry_count = sqlc.arg(failure_retry_count),
    status = sqlc.arg(status),
    past_due_since = CASE
        -- Transitioning TO past_due: set fresh timestamp
        WHEN sqlc.arg(status) = 'past_due' AND status != 'past_due' THEN CURRENT_TIMESTAMP
        -- Leaving past_due status: clear the timestamp
        WHEN sqlc.arg(status) != 'past_due' AND status = 'past_due' THEN NULL
        -- Otherwise keep existing value
        ELSE past_due_since
    END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListExpiredPastDueSubscriptions :many
-- Lists subscriptions where grace period has expired and should be auto-cancelled
-- Uses make_interval for safer interval calculation
SELECT * FROM subscriptions
WHERE status = 'past_due'
  AND past_due_since IS NOT NULL
  AND deleted_at IS NULL
  AND past_due_since + make_interval(days => grace_period_days) < CURRENT_TIMESTAMP
ORDER BY past_due_since ASC
LIMIT sqlc.arg(limit_val);

-- name: CancelSubscriptionWithReason :one
-- Cancels subscription with reason tracking (idempotent - won't overwrite existing cancellation)
-- Returns the subscription even if already cancelled (for idempotency)
UPDATE subscriptions
SET
    status = 'cancelled',
    cancelled_at = COALESCE(cancelled_at, CURRENT_TIMESTAMP),
    cancellation_reason = COALESCE(cancellation_reason, sqlc.arg(cancellation_reason)),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateSubscriptionGracePeriod :one
-- Updates the grace period for a subscription
UPDATE subscriptions
SET
    grace_period_days = sqlc.arg(grace_period_days),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ReactivateSubscription :one
-- Reactivates a past_due subscription (clears past_due_since, resets status)
UPDATE subscriptions
SET
    status = 'active',
    past_due_since = NULL,
    failure_retry_count = 0,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND status = 'past_due'
RETURNING *;

-- name: GetSubscriptionStats :one
-- Gets subscription statistics for monitoring
SELECT
    COUNT(*) FILTER (WHERE status = 'active') as active_count,
    COUNT(*) FILTER (WHERE status = 'paused') as paused_count,
    COUNT(*) FILTER (WHERE status = 'past_due') as past_due_count,
    COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled_count,
    COUNT(*) FILTER (WHERE status = 'past_due' AND past_due_since + make_interval(days => grace_period_days) < CURRENT_TIMESTAMP) as expired_past_due_count
FROM subscriptions
WHERE (sqlc.narg(merchant_id)::uuid IS NULL OR merchant_id = sqlc.narg(merchant_id));

-- name: TryAdvisoryLock :one
-- Tries to acquire an advisory lock (non-blocking). Returns true if lock acquired.
-- Use for preventing concurrent cron job execution.
SELECT pg_try_advisory_lock(sqlc.arg(lock_id)::bigint) as acquired;

-- name: AdvisoryUnlock :one
-- Releases an advisory lock. Returns true if lock was held and released.
SELECT pg_advisory_unlock(sqlc.arg(lock_id)::bigint) as released;

-- name: SetBillingRetryBackoff :exec
-- Schedules next billing retry with backoff for transient errors (network issues).
-- Only use for transient errors - permanent failures should not set backoff.
UPDATE subscriptions
SET
    next_billing_retry_at = sqlc.arg(next_billing_retry_at),
    last_billing_error = sqlc.arg(last_billing_error),
    last_billing_error_at = CURRENT_TIMESTAMP,
    failure_retry_count = failure_retry_count + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: ClearBillingRetryBackoff :exec
-- Clears retry backoff after successful billing. Resets error tracking.
UPDATE subscriptions
SET
    next_billing_retry_at = NULL,
    last_billing_error = NULL,
    last_billing_error_at = NULL,
    failure_retry_count = 0,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: RecordBillingFailure :exec
-- Records a billing failure without backoff (for permanent errors like card declined).
-- Increments retry count but does not schedule backoff.
UPDATE subscriptions
SET
    last_billing_error = sqlc.arg(last_billing_error),
    last_billing_error_at = CURRENT_TIMESTAMP,
    failure_retry_count = failure_retry_count + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);
