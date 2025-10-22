-- name: CreateSubscription :exec
INSERT INTO subscriptions (
    id, merchant_id, customer_id, amount, currency, frequency, status,
    payment_method_token, next_billing_date, max_retries, failure_option,
    gateway_subscription_id, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
);

-- name: GetSubscriptionByID :one
SELECT * FROM subscriptions
WHERE id = $1;

-- name: UpdateSubscription :exec
UPDATE subscriptions
SET amount = COALESCE(sqlc.narg(amount), amount),
    frequency = COALESCE(sqlc.narg(frequency), frequency),
    status = COALESCE(sqlc.narg(status), status),
    payment_method_token = COALESCE(sqlc.narg(payment_method_token), payment_method_token),
    next_billing_date = COALESCE(sqlc.narg(next_billing_date), next_billing_date),
    failure_retry_count = COALESCE(sqlc.narg(failure_retry_count), failure_retry_count),
    gateway_subscription_id = COALESCE(sqlc.narg(gateway_subscription_id), gateway_subscription_id),
    cancelled_at = sqlc.narg(cancelled_at)
WHERE id = $1;

-- name: ListSubscriptionsByCustomer :many
SELECT * FROM subscriptions
WHERE merchant_id = $1 AND customer_id = $2
ORDER BY created_at DESC;

-- name: ListActiveSubscriptionsDueForBilling :many
SELECT * FROM subscriptions
WHERE status = 'active'
  AND next_billing_date <= $1
ORDER BY next_billing_date ASC
LIMIT $2;
