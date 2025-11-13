-- name: CreatePaymentMethod :one
INSERT INTO customer_payment_methods (
    id, merchant_id, customer_id, payment_type,
    payment_token, last_four,
    card_brand, card_exp_month, card_exp_year,
    bank_name, account_type,
    is_default, is_active, is_verified
) VALUES (
    sqlc.arg(id), sqlc.arg(merchant_id), sqlc.arg(customer_id), sqlc.arg(payment_type),
    sqlc.arg(payment_token), sqlc.arg(last_four),
    sqlc.narg(card_brand), sqlc.narg(card_exp_month), sqlc.narg(card_exp_year),
    sqlc.narg(bank_name), sqlc.narg(account_type),
    sqlc.arg(is_default), sqlc.arg(is_active), sqlc.arg(is_verified)
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
    (sqlc.narg(merchant_id)::varchar IS NULL OR merchant_id = sqlc.narg(merchant_id)) AND
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
SET is_verified = true, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: DeactivatePaymentMethod :exec
UPDATE customer_payment_methods
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ActivatePaymentMethod :exec
UPDATE customer_payment_methods
SET is_active = true, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: DeletePaymentMethod :exec
UPDATE customer_payment_methods
SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
