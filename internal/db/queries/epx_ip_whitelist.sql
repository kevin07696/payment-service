-- name: ListActiveIPWhitelist :many
-- Get all active IP addresses from EPX whitelist
SELECT ip_address
FROM epx_ip_whitelist
WHERE is_active = true;

-- name: AddIPToWhitelist :one
-- Add an IP address to the EPX whitelist
INSERT INTO epx_ip_whitelist (
    ip_address, description, added_by, is_active
) VALUES (
    sqlc.arg(ip_address), sqlc.narg(description), sqlc.narg(added_by), sqlc.arg(is_active)
) RETURNING *;

-- name: RemoveIPFromWhitelist :exec
-- Deactivate an IP address from the EPX whitelist
UPDATE epx_ip_whitelist
SET is_active = false
WHERE ip_address = sqlc.arg(ip_address);

-- name: GetIPWhitelistEntry :one
-- Get a specific IP whitelist entry
SELECT * FROM epx_ip_whitelist
WHERE ip_address = sqlc.arg(ip_address);
