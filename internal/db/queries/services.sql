-- name: CreateService :one
INSERT INTO services (
    id, service_id, service_name, public_key, public_key_fingerprint,
    environment, requests_per_second, burst_limit, is_active
) VALUES (
    sqlc.arg(id), sqlc.arg(service_id), sqlc.arg(service_name), sqlc.arg(public_key), sqlc.arg(public_key_fingerprint),
    sqlc.arg(environment), sqlc.arg(requests_per_second), sqlc.arg(burst_limit), sqlc.arg(is_active)
) RETURNING *;

-- name: UpsertService :one
INSERT INTO services (
    id, service_id, service_name, public_key, public_key_fingerprint,
    environment, requests_per_second, burst_limit, is_active
) VALUES (
    sqlc.arg(id), sqlc.arg(service_id), sqlc.arg(service_name), sqlc.arg(public_key), sqlc.arg(public_key_fingerprint),
    sqlc.arg(environment), sqlc.arg(requests_per_second), sqlc.arg(burst_limit), sqlc.arg(is_active)
) ON CONFLICT (service_id) DO UPDATE SET
    public_key = EXCLUDED.public_key,
    public_key_fingerprint = EXCLUDED.public_key_fingerprint,
    updated_at = NOW()
RETURNING *;

-- name: GetServiceByID :one
SELECT * FROM services
WHERE id = sqlc.arg(id);

-- name: GetServiceByServiceID :one
SELECT * FROM services
WHERE service_id = sqlc.arg(service_id) AND is_active = true;

-- name: ListServices :many
SELECT * FROM services
WHERE
    (sqlc.narg(environment)::varchar IS NULL OR environment = sqlc.narg(environment)) AND
    (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_val) OFFSET sqlc.arg(offset_val);

-- name: CountServices :one
SELECT COUNT(*) FROM services
WHERE
    (sqlc.narg(environment)::varchar IS NULL OR environment = sqlc.narg(environment)) AND
    (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active));

-- name: UpdateService :one
UPDATE services
SET
    service_name = sqlc.arg(service_name),
    public_key = sqlc.arg(public_key),
    public_key_fingerprint = sqlc.arg(public_key_fingerprint),
    requests_per_second = sqlc.arg(requests_per_second),
    burst_limit = sqlc.arg(burst_limit),
    is_active = sqlc.arg(is_active),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeactivateService :exec
UPDATE services
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: ActivateService :exec
UPDATE services
SET is_active = true, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: RotateServiceKey :one
UPDATE services
SET
    public_key = sqlc.arg(public_key),
    public_key_fingerprint = sqlc.arg(public_key_fingerprint),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListActiveServicePublicKeys :many
-- Get all active service public keys for JWT verification
SELECT service_id, public_key
FROM services
WHERE is_active = true;

-- name: GetServiceRateLimit :one
-- Get rate limit for a specific service
SELECT requests_per_second
FROM services
WHERE service_id = sqlc.arg(service_id) AND is_active = true;

-- name: HardDeleteService :exec
-- WARNING: For test cleanup only. Permanently deletes service record.
DELETE FROM services WHERE id = sqlc.arg(id);
