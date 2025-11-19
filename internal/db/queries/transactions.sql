-- name: CreateTransaction :one
-- Transactions are append-only/immutable event logs
-- status is GENERATED column based on auth_resp, so we don't insert it
-- Uses ON CONFLICT DO NOTHING for idempotency: EPX callback retries return existing record unchanged
-- Modifications (VOID/REFUND) create NEW transaction records linked via parent_transaction_id
-- auth_guid stores EPX BRIC for this transaction (each transaction can have its own BRIC)
-- tran_nbr stores EPX TRAN_NBR (deterministic 10-digit numeric ID from UUID)
-- parent_transaction_id links to parent transaction (CAPTURE→AUTH, REFUND→SALE/CAPTURE, etc.)
INSERT INTO transactions (
    id, merchant_id, customer_id,
    amount_cents, currency, type, payment_method_type, payment_method_id, subscription_id,
    tran_nbr, auth_guid, auth_resp, auth_code, auth_card_type,
    metadata,
    parent_transaction_id, processed_at
) VALUES (
    sqlc.arg(id), sqlc.arg(merchant_id), sqlc.narg(customer_id),
    sqlc.arg(amount_cents), sqlc.arg(currency), sqlc.arg(type), sqlc.arg(payment_method_type), sqlc.narg(payment_method_id), sqlc.narg(subscription_id),
    sqlc.narg(tran_nbr), sqlc.narg(auth_guid), sqlc.narg(auth_resp), sqlc.narg(auth_code), sqlc.narg(auth_card_type),
    sqlc.arg(metadata),
    sqlc.narg(parent_transaction_id), sqlc.narg(processed_at)
)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: GetTransactionByID :one
SELECT * FROM transactions
WHERE id = sqlc.arg(id);

-- name: GetTransactionTree :many
-- Recursively fetches a transaction and all its descendants (children, grandchildren, etc.)
-- Use this to get the full transaction tree starting from any transaction
-- Example: GetTransactionTree(auth1) returns [auth1, void1, auth2, charge2, refund2]
-- Example: GetTransactionTree(auth2) returns [auth2, charge2, refund2]
WITH RECURSIVE transaction_tree AS (
    -- Base case: the requested transaction (root of the subtree)
    SELECT * FROM transactions WHERE transactions.id = sqlc.arg(parent_transaction_id)

    UNION ALL

    -- Recursive case: all children of transactions already in the tree
    SELECT t.*
    FROM transactions t
    INNER JOIN transaction_tree tt ON t.parent_transaction_id = tt.id
)
SELECT * FROM transaction_tree
ORDER BY created_at ASC;

-- name: ListTransactions :many
SELECT * FROM transactions
WHERE
    merchant_id = sqlc.arg(merchant_id) AND
    (sqlc.narg(customer_id)::uuid IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(subscription_id)::uuid IS NULL OR subscription_id = sqlc.narg(subscription_id)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)) AND
    (sqlc.narg(type)::varchar IS NULL OR type = sqlc.narg(type)) AND
    (sqlc.narg(payment_method_id)::uuid IS NULL OR payment_method_id = sqlc.narg(payment_method_id))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_val) OFFSET sqlc.arg(offset_val);

-- name: CountTransactions :one
SELECT COUNT(*) FROM transactions
WHERE
    merchant_id = sqlc.arg(merchant_id) AND
    (sqlc.narg(customer_id)::uuid IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(subscription_id)::uuid IS NULL OR subscription_id = sqlc.narg(subscription_id)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)) AND
    (sqlc.narg(type)::varchar IS NULL OR type = sqlc.narg(type)) AND
    (sqlc.narg(payment_method_id)::uuid IS NULL OR payment_method_id = sqlc.narg(payment_method_id));

-- UpdateTransaction removed: transactions are immutable/append-only
-- To modify a transaction (VOID/REFUND), create a NEW transaction record with parent_transaction_id

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
    auth_resp = COALESCE(sqlc.narg(auth_resp), auth_resp),
    auth_code = COALESCE(sqlc.narg(auth_code), auth_code),
    auth_card_type = COALESCE(sqlc.narg(auth_card_type), auth_card_type),
    processed_at = COALESCE(sqlc.narg(processed_at), processed_at),
    metadata = COALESCE(sqlc.arg(metadata), metadata),
    updated_at = CURRENT_TIMESTAMP
WHERE tran_nbr = sqlc.arg(tran_nbr)
RETURNING *;
