-- name: CreateTransaction :one
INSERT INTO transactions (
    id, group_id, agent_id, customer_id,
    amount, currency, status, type, payment_method_type, payment_method_id,
    auth_guid, auth_resp, auth_code, auth_resp_text, auth_card_type, auth_avs, auth_cvv2,
    idempotency_key, metadata
) VALUES (
    sqlc.arg(id), sqlc.arg(group_id), sqlc.arg(agent_id), sqlc.narg(customer_id),
    sqlc.arg(amount), sqlc.arg(currency), sqlc.arg(status), sqlc.arg(type), sqlc.arg(payment_method_type), sqlc.narg(payment_method_id),
    sqlc.narg(auth_guid), sqlc.narg(auth_resp), sqlc.narg(auth_code), sqlc.narg(auth_resp_text), sqlc.narg(auth_card_type), sqlc.narg(auth_avs), sqlc.narg(auth_cvv2),
    sqlc.narg(idempotency_key), sqlc.arg(metadata)
) RETURNING *;

-- name: GetTransactionByID :one
SELECT * FROM transactions
WHERE id = sqlc.arg(id);

-- name: GetTransactionByIdempotencyKey :one
SELECT * FROM transactions
WHERE idempotency_key = sqlc.arg(idempotency_key);

-- name: GetTransactionsByGroupID :many
SELECT * FROM transactions
WHERE group_id = sqlc.arg(group_id)
ORDER BY created_at ASC;

-- name: ListTransactions :many
SELECT * FROM transactions
WHERE
    (sqlc.narg(agent_id)::varchar IS NULL OR agent_id = sqlc.narg(agent_id)) AND
    (sqlc.narg(customer_id)::varchar IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(group_id)::uuid IS NULL OR group_id = sqlc.narg(group_id)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)) AND
    (sqlc.narg(type)::varchar IS NULL OR type = sqlc.narg(type)) AND
    (sqlc.narg(payment_method_id)::uuid IS NULL OR payment_method_id = sqlc.narg(payment_method_id))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_val) OFFSET sqlc.arg(offset_val);

-- name: CountTransactions :one
SELECT COUNT(*) FROM transactions
WHERE
    (sqlc.narg(agent_id)::varchar IS NULL OR agent_id = sqlc.narg(agent_id)) AND
    (sqlc.narg(customer_id)::varchar IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(group_id)::uuid IS NULL OR group_id = sqlc.narg(group_id)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)) AND
    (sqlc.narg(type)::varchar IS NULL OR type = sqlc.narg(type)) AND
    (sqlc.narg(payment_method_id)::uuid IS NULL OR payment_method_id = sqlc.narg(payment_method_id));

-- name: UpdateTransaction :one
UPDATE transactions
SET
    status = sqlc.arg(status),
    auth_guid = sqlc.narg(auth_guid),
    auth_resp = sqlc.narg(auth_resp),
    auth_code = sqlc.narg(auth_code),
    auth_resp_text = sqlc.narg(auth_resp_text),
    auth_card_type = sqlc.narg(auth_card_type),
    auth_avs = sqlc.narg(auth_avs),
    auth_cvv2 = sqlc.narg(auth_cvv2),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateTransactionStatus :exec
UPDATE transactions
SET status = sqlc.arg(status), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);
