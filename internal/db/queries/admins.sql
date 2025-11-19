-- name: CreateAdmin :one
INSERT INTO admins (
    id, email, password_hash, role, is_active
) VALUES (
    sqlc.arg(id), sqlc.arg(email), sqlc.arg(password_hash), sqlc.arg(role), sqlc.arg(is_active)
) RETURNING *;

-- name: GetAdminByID :one
SELECT * FROM admins
WHERE id = sqlc.arg(id);

-- name: GetAdminByEmail :one
SELECT * FROM admins
WHERE email = sqlc.arg(email);

-- name: ListAdmins :many
SELECT * FROM admins
WHERE
    (sqlc.narg(role)::varchar IS NULL OR role = sqlc.narg(role)) AND
    (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_val) OFFSET sqlc.arg(offset_val);

-- name: UpdateAdminPassword :exec
UPDATE admins
SET password_hash = sqlc.arg(password_hash), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: UpdateAdminRole :exec
UPDATE admins
SET role = sqlc.arg(role), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: DeactivateAdmin :exec
UPDATE admins
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: ActivateAdmin :exec
UPDATE admins
SET is_active = true, updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: CreateAdminSession :one
INSERT INTO admin_sessions (
    id, admin_id, token_hash, ip_address, user_agent, expires_at
) VALUES (
    sqlc.arg(id), sqlc.arg(admin_id), sqlc.arg(token_hash), sqlc.narg(ip_address), sqlc.narg(user_agent), sqlc.arg(expires_at)
) RETURNING *;

-- name: GetAdminSession :one
SELECT * FROM admin_sessions
WHERE id = sqlc.arg(id) AND expires_at > NOW();

-- name: DeleteAdminSession :exec
DELETE FROM admin_sessions
WHERE id = sqlc.arg(id);

-- name: DeleteExpiredAdminSessions :exec
DELETE FROM admin_sessions
WHERE expires_at < NOW();
