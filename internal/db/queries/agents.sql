-- name: CreateAgent :one
INSERT INTO agent_credentials (
    id, agent_id, cust_nbr, merch_nbr, dba_nbr, terminal_nbr,
    mac_secret_path, environment, is_active, agent_name
) VALUES (
    sqlc.arg(id), sqlc.arg(agent_id), sqlc.arg(cust_nbr), sqlc.arg(merch_nbr), sqlc.arg(dba_nbr), sqlc.arg(terminal_nbr),
    sqlc.arg(mac_secret_path), sqlc.arg(environment), sqlc.arg(is_active), sqlc.arg(agent_name)
) RETURNING *;

-- name: GetAgentByID :one
SELECT * FROM agent_credentials
WHERE id = sqlc.arg(id);

-- name: GetAgentByAgentID :one
SELECT * FROM agent_credentials
WHERE agent_id = sqlc.arg(agent_id);

-- name: ListAgents :many
SELECT * FROM agent_credentials
WHERE
    (sqlc.narg(environment)::varchar IS NULL OR environment = sqlc.narg(environment)) AND
    (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active))
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_val) OFFSET sqlc.arg(offset_val);

-- name: CountAgents :one
SELECT COUNT(*) FROM agent_credentials
WHERE
    (sqlc.narg(environment)::varchar IS NULL OR environment = sqlc.narg(environment)) AND
    (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active));

-- name: UpdateAgent :one
UPDATE agent_credentials
SET
    cust_nbr = sqlc.arg(cust_nbr),
    merch_nbr = sqlc.arg(merch_nbr),
    dba_nbr = sqlc.arg(dba_nbr),
    terminal_nbr = sqlc.arg(terminal_nbr),
    environment = sqlc.arg(environment),
    agent_name = sqlc.arg(agent_name),
    updated_at = CURRENT_TIMESTAMP
WHERE agent_id = sqlc.arg(agent_id)
RETURNING *;

-- name: UpdateAgentMACPath :exec
UPDATE agent_credentials
SET mac_secret_path = sqlc.arg(mac_secret_path), updated_at = CURRENT_TIMESTAMP
WHERE agent_id = sqlc.arg(agent_id);

-- name: DeactivateAgent :exec
UPDATE agent_credentials
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE agent_id = sqlc.arg(agent_id);

-- name: ActivateAgent :exec
UPDATE agent_credentials
SET is_active = true, updated_at = CURRENT_TIMESTAMP
WHERE agent_id = sqlc.arg(agent_id);

-- name: AgentExists :one
SELECT EXISTS(SELECT 1 FROM agent_credentials WHERE agent_id = sqlc.arg(agent_id));

-- name: ListActiveAgents :many
SELECT * FROM agent_credentials
WHERE is_active = true
ORDER BY created_at DESC;
