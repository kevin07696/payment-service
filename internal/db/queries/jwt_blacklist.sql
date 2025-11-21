-- name: IsJWTBlacklisted :one
-- Check if a JWT token is blacklisted
SELECT EXISTS(
    SELECT 1 FROM jwt_blacklist
    WHERE jti = sqlc.arg(jti)
    AND expires_at > NOW()
) as is_blacklisted;

-- name: BlacklistJWT :exec
-- Add a JWT token to the blacklist
INSERT INTO jwt_blacklist (
    jti, service_id, merchant_id, expires_at, blacklisted_by, reason
) VALUES (
    sqlc.arg(jti), sqlc.narg(service_id), sqlc.narg(merchant_id),
    sqlc.arg(expires_at), sqlc.narg(blacklisted_by), sqlc.narg(reason)
);

-- name: CleanupExpiredBlacklist :exec
-- Remove expired entries from JWT blacklist
DELETE FROM jwt_blacklist
WHERE expires_at < NOW();
