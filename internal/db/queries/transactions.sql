-- name: CreateTransaction :exec
INSERT INTO transactions (
    id, merchant_id, customer_id, amount, currency, status, type,
    payment_method_type, payment_method_token, gateway_transaction_id,
    gateway_response_code, gateway_response_message, idempotency_key, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
);

-- name: GetTransactionByID :one
SELECT * FROM transactions
WHERE id = $1;

-- name: GetTransactionByIdempotencyKey :one
SELECT * FROM transactions
WHERE idempotency_key = $1;

-- name: UpdateTransactionStatus :exec
UPDATE transactions
SET status = $2,
    gateway_transaction_id = COALESCE($3, gateway_transaction_id),
    gateway_response_code = COALESCE($4, gateway_response_code),
    gateway_response_message = COALESCE($5, gateway_response_message)
WHERE id = $1;

-- name: ListTransactionsByMerchant :many
SELECT * FROM transactions
WHERE merchant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListTransactionsByCustomer :many
SELECT * FROM transactions
WHERE merchant_id = $1 AND customer_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
