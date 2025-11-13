-- name: CreateTransaction :one
-- Transactions are append-only/immutable event logs
-- status is GENERATED column based on auth_resp, so we don't insert it
-- Uses ON CONFLICT DO NOTHING for idempotency: EPX callback retries return existing record unchanged
-- Modifications (VOID/REFUND) create NEW transaction records with same group_id
-- auth_guid stores EPX BRIC for this transaction (each transaction can have its own BRIC)
-- tran_nbr stores EPX TRAN_NBR (deterministic 10-digit numeric ID from UUID)
-- group_id is a logical grouping UUID (NOT a foreign key) - auto-generates if not provided
INSERT INTO transactions (
    id, merchant_id, customer_id,
    amount, currency, type, payment_method_type, payment_method_id, subscription_id,
    tran_nbr, auth_guid, auth_resp, auth_code, auth_card_type,
    metadata,
    group_id
) VALUES (
    sqlc.arg(id), sqlc.arg(merchant_id), sqlc.narg(customer_id),
    sqlc.arg(amount), sqlc.arg(currency), sqlc.arg(type), sqlc.arg(payment_method_type), sqlc.narg(payment_method_id), sqlc.narg(subscription_id),
    sqlc.narg(tran_nbr), sqlc.narg(auth_guid), sqlc.arg(auth_resp), sqlc.narg(auth_code), sqlc.narg(auth_card_type),
    sqlc.arg(metadata),
    COALESCE(sqlc.narg(group_id), gen_random_uuid())
)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: GetTransactionByID :one
SELECT * FROM transactions
WHERE id = sqlc.arg(id);

-- name: GetTransactionsByGroupID :many
SELECT * FROM transactions
WHERE group_id = sqlc.arg(group_id)
ORDER BY created_at ASC;

-- name: ListTransactions :many
SELECT * FROM transactions
WHERE
    (sqlc.narg(merchant_id)::uuid IS NULL OR merchant_id = sqlc.narg(merchant_id)) AND
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
    (sqlc.narg(merchant_id)::uuid IS NULL OR merchant_id = sqlc.narg(merchant_id)) AND
    (sqlc.narg(customer_id)::varchar IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(group_id)::uuid IS NULL OR group_id = sqlc.narg(group_id)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)) AND
    (sqlc.narg(type)::varchar IS NULL OR type = sqlc.narg(type)) AND
    (sqlc.narg(payment_method_id)::uuid IS NULL OR payment_method_id = sqlc.narg(payment_method_id));

-- UpdateTransaction removed: transactions are immutable/append-only
-- To modify a transaction (VOID/REFUND), create a NEW transaction record with same group_id

-- name: GetTransactionByTranNbr :one
SELECT * FROM transactions
WHERE tran_nbr = sqlc.arg(tran_nbr)
LIMIT 1;

-- name: UpdateTransactionFromEPXResponse :one
-- Updates transaction with EPX response data (for Browser Post callback)
-- Only updates EPX response fields, leaves core transaction data unchanged
UPDATE transactions SET
    customer_id = COALESCE(sqlc.narg(customer_id), customer_id),
    auth_guid = COALESCE(sqlc.narg(auth_guid), auth_guid),
    auth_resp = COALESCE(sqlc.arg(auth_resp), auth_resp),
    auth_code = COALESCE(sqlc.narg(auth_code), auth_code),
    auth_card_type = COALESCE(sqlc.narg(auth_card_type), auth_card_type),
    metadata = COALESCE(sqlc.arg(metadata), metadata),
    updated_at = CURRENT_TIMESTAMP
WHERE tran_nbr = sqlc.arg(tran_nbr)
RETURNING *;
