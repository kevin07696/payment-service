-- name: CreateWebhookSubscription :one
INSERT INTO webhook_subscriptions (
    agent_id,
    event_type,
    webhook_url,
    secret,
    is_active
) VALUES (
    sqlc.arg(agent_id),
    sqlc.arg(event_type),
    sqlc.arg(webhook_url),
    sqlc.arg(secret),
    sqlc.arg(is_active)
) RETURNING *;

-- name: GetWebhookSubscription :one
SELECT * FROM webhook_subscriptions
WHERE id = sqlc.arg(id);

-- name: ListWebhookSubscriptions :many
SELECT * FROM webhook_subscriptions
WHERE agent_id = sqlc.arg(agent_id)
  AND (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active))
ORDER BY created_at DESC;

-- name: ListActiveWebhooksByEvent :many
SELECT * FROM webhook_subscriptions
WHERE agent_id = sqlc.arg(agent_id)
  AND event_type = sqlc.arg(event_type)
  AND is_active = true;

-- name: UpdateWebhookSubscription :one
UPDATE webhook_subscriptions
SET
    webhook_url = COALESCE(sqlc.narg(webhook_url), webhook_url),
    secret = COALESCE(sqlc.narg(secret), secret),
    is_active = COALESCE(sqlc.narg(is_active), is_active),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteWebhookSubscription :exec
DELETE FROM webhook_subscriptions
WHERE id = sqlc.arg(id) AND agent_id = sqlc.arg(agent_id);

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (
    subscription_id,
    event_type,
    payload,
    status,
    http_status_code,
    error_message,
    attempts,
    next_retry_at
) VALUES (
    sqlc.arg(subscription_id),
    sqlc.arg(event_type),
    sqlc.arg(payload),
    sqlc.arg(status),
    sqlc.narg(http_status_code),
    sqlc.narg(error_message),
    sqlc.arg(attempts),
    sqlc.narg(next_retry_at)
) RETURNING *;

-- name: UpdateWebhookDeliveryStatus :one
UPDATE webhook_deliveries
SET
    status = sqlc.arg(status),
    http_status_code = sqlc.narg(http_status_code),
    error_message = sqlc.narg(error_message),
    attempts = sqlc.arg(attempts),
    next_retry_at = sqlc.narg(next_retry_at),
    delivered_at = sqlc.narg(delivered_at)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListPendingWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE status = 'pending'
  AND next_retry_at <= CURRENT_TIMESTAMP
ORDER BY next_retry_at ASC
LIMIT sqlc.arg(limit_val);

-- name: GetWebhookDeliveryHistory :many
SELECT * FROM webhook_deliveries
WHERE subscription_id = sqlc.arg(subscription_id)
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_val)
OFFSET sqlc.arg(offset_val);
