-- name: GrantServiceAccess :one
INSERT INTO service_merchants (
    service_id, merchant_id, scopes, granted_by, expires_at
) VALUES (
    sqlc.arg(service_id), sqlc.arg(merchant_id), sqlc.arg(scopes), sqlc.narg(granted_by), sqlc.narg(expires_at)
)
ON CONFLICT (service_id, merchant_id) DO UPDATE
SET
    scopes = EXCLUDED.scopes,
    granted_by = EXCLUDED.granted_by,
    granted_at = CURRENT_TIMESTAMP,
    expires_at = EXCLUDED.expires_at
RETURNING *;

-- name: GetServiceMerchantAccess :one
SELECT * FROM service_merchants
WHERE service_id = sqlc.arg(service_id) AND merchant_id = sqlc.arg(merchant_id);

-- name: ListServiceMerchants :many
-- Get all merchants a service has access to
SELECT sm.*, m.name as merchant_name, m.slug as merchant_slug
FROM service_merchants sm
JOIN merchants m ON sm.merchant_id = m.id
WHERE sm.service_id = sqlc.arg(service_id)
ORDER BY sm.granted_at DESC;

-- name: ListMerchantServices :many
-- Get all services that have access to a merchant
SELECT sm.*, s.service_name, s.environment, s.is_active
FROM service_merchants sm
JOIN services s ON sm.service_id = s.id
WHERE sm.merchant_id = sqlc.arg(merchant_id)
ORDER BY sm.granted_at DESC;

-- name: RevokeServiceAccess :exec
DELETE FROM service_merchants
WHERE service_id = sqlc.arg(service_id) AND merchant_id = sqlc.arg(merchant_id);

-- name: UpdateServiceScopes :one
UPDATE service_merchants
SET
    scopes = sqlc.arg(scopes),
    granted_at = CURRENT_TIMESTAMP
WHERE service_id = sqlc.arg(service_id) AND merchant_id = sqlc.arg(merchant_id)
RETURNING *;

-- name: CheckServiceHasScope :one
SELECT EXISTS(
    SELECT 1 FROM service_merchants
    WHERE service_id = sqlc.arg(service_id)
        AND merchant_id = sqlc.arg(merchant_id)
        AND sqlc.arg(scope)::text = ANY(scopes)
        AND (expires_at IS NULL OR expires_at > NOW())
) as has_scope;
