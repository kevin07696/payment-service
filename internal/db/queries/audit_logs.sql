-- name: CreateAuditLog :exec
INSERT INTO audit_logs (
    id, actor_type, actor_id, actor_name, action, entity_type, entity_id,
    changes, metadata, ip_address, user_agent, request_id, success, error_message
) VALUES (
    sqlc.arg(id), sqlc.narg(actor_type), sqlc.narg(actor_id), sqlc.narg(actor_name),
    sqlc.arg(action), sqlc.narg(entity_type), sqlc.narg(entity_id),
    sqlc.narg(changes), sqlc.narg(metadata), sqlc.narg(ip_address), sqlc.narg(user_agent),
    sqlc.narg(request_id), sqlc.arg(success), sqlc.narg(error_message)
);

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
WHERE
    (sqlc.narg(actor_type)::varchar IS NULL OR actor_type = sqlc.narg(actor_type)) AND
    (sqlc.narg(actor_id)::varchar IS NULL OR actor_id = sqlc.narg(actor_id)) AND
    (sqlc.narg(action)::varchar IS NULL OR action = sqlc.narg(action)) AND
    (sqlc.narg(entity_type)::varchar IS NULL OR entity_type = sqlc.narg(entity_type)) AND
    (sqlc.narg(entity_id)::varchar IS NULL OR entity_id = sqlc.narg(entity_id)) AND
    (sqlc.narg(start_date)::timestamp IS NULL OR performed_at >= sqlc.narg(start_date)) AND
    (sqlc.narg(end_date)::timestamp IS NULL OR performed_at <= sqlc.narg(end_date))
ORDER BY performed_at DESC
LIMIT sqlc.arg(limit_val) OFFSET sqlc.arg(offset_val);

-- name: GetAuditLogsByEntity :many
SELECT * FROM audit_logs
WHERE entity_type = sqlc.arg(entity_type) AND entity_id = sqlc.arg(entity_id)
ORDER BY performed_at DESC
LIMIT sqlc.arg(limit_val);

-- name: GetAuditLogsByActor :many
SELECT * FROM audit_logs
WHERE actor_type = sqlc.arg(actor_type) AND actor_id = sqlc.arg(actor_id)
ORDER BY performed_at DESC
LIMIT sqlc.arg(limit_val);

-- name: CountAuditLogs :one
SELECT COUNT(*) FROM audit_logs
WHERE
    (sqlc.narg(actor_type)::varchar IS NULL OR actor_type = sqlc.narg(actor_type)) AND
    (sqlc.narg(action)::varchar IS NULL OR action = sqlc.narg(action)) AND
    (sqlc.narg(start_date)::timestamp IS NULL OR performed_at >= sqlc.narg(start_date)) AND
    (sqlc.narg(end_date)::timestamp IS NULL OR performed_at <= sqlc.narg(end_date));
