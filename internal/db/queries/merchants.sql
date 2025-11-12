-- name: CreateMerchant :one
INSERT INTO merchants (
    id, slug, cust_nbr, merch_nbr, dba_nbr, terminal_nbr,
    mac_secret_path, environment, is_active, name
) VALUES (
    sqlc.arg(id), sqlc.arg(slug), sqlc.arg(cust_nbr), sqlc.arg(merch_nbr), sqlc.arg(dba_nbr), sqlc.arg(terminal_nbr),
    sqlc.arg(mac_secret_path), sqlc.arg(environment), sqlc.arg(is_active), sqlc.arg(name)
) RETURNING *;

-- name: GetMerchantByID :one
SELECT * FROM merchants
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetMerchantBySlug :one
SELECT * FROM merchants
WHERE slug = sqlc.arg(slug) AND deleted_at IS NULL;

-- name: MerchantExistsBySlug :one
SELECT EXISTS(SELECT 1 FROM merchants WHERE slug = sqlc.arg(slug) AND deleted_at IS NULL);

-- name: ListMerchants :many
SELECT * FROM merchants
WHERE
    deleted_at IS NULL AND
    (sqlc.narg(environment)::varchar IS NULL OR environment = sqlc.narg(environment)) AND
    (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_val) OFFSET sqlc.arg(offset_val);

-- name: CountMerchants :one
SELECT COUNT(*) FROM merchants
WHERE
    deleted_at IS NULL AND
    (sqlc.narg(environment)::varchar IS NULL OR environment = sqlc.narg(environment)) AND
    (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active));

-- name: UpdateMerchant :one
UPDATE merchants
SET
    cust_nbr = sqlc.arg(cust_nbr),
    merch_nbr = sqlc.arg(merch_nbr),
    dba_nbr = sqlc.arg(dba_nbr),
    terminal_nbr = sqlc.arg(terminal_nbr),
    environment = sqlc.arg(environment),
    name = sqlc.arg(name),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: UpdateMerchantMACPath :exec
UPDATE merchants
SET mac_secret_path = sqlc.arg(mac_secret_path), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: DeactivateMerchant :exec
UPDATE merchants
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ActivateMerchant :exec
UPDATE merchants
SET is_active = true, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: MerchantExists :one
SELECT EXISTS(SELECT 1 FROM merchants WHERE id = sqlc.arg(id) AND deleted_at IS NULL);

-- name: ListActiveMerchants :many
SELECT * FROM merchants
WHERE is_active = true AND deleted_at IS NULL
ORDER BY created_at DESC;
