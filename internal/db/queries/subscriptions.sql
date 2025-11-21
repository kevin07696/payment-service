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
SELECT * FROM subscriptions
WHERE status = 'active' AND next_billing_date <= sqlc.arg(next_billing_date)
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
UPDATE subscriptions
SET
    failure_retry_count = sqlc.arg(failure_retry_count),
    status = sqlc.arg(status),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;
