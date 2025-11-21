-- name: CreateChargeback :one
INSERT INTO chargebacks (
    id, transaction_id, agent_id, customer_id,
    case_number, dispute_date, chargeback_date,
    chargeback_amount, currency, reason_code, reason_description,
    status, respond_by_date,
    evidence_files, response_notes, internal_notes,
    raw_data
) VALUES (
    sqlc.arg(id), sqlc.arg(transaction_id), sqlc.arg(agent_id), sqlc.narg(customer_id),
    sqlc.arg(case_number), sqlc.arg(dispute_date), sqlc.arg(chargeback_date),
    sqlc.arg(chargeback_amount), sqlc.arg(currency), sqlc.arg(reason_code), sqlc.narg(reason_description),
    sqlc.arg(status), sqlc.narg(respond_by_date),
    sqlc.arg(evidence_files), sqlc.narg(response_notes), sqlc.narg(internal_notes),
    sqlc.arg(raw_data)
) RETURNING *;

-- name: GetChargebackByID :one
SELECT * FROM chargebacks
WHERE id = sqlc.arg(id);

-- name: GetChargebackByCaseNumber :one
SELECT * FROM chargebacks
WHERE agent_id = sqlc.arg(agent_id) AND case_number = sqlc.arg(case_number);

-- name: GetChargebackByTransactionID :one
SELECT * FROM chargebacks
WHERE transaction_id = sqlc.arg(transaction_id);

-- name: ListChargebacks :many
SELECT * FROM chargebacks
WHERE
    (sqlc.narg(agent_id)::varchar IS NULL OR agent_id = sqlc.narg(agent_id)) AND
    (sqlc.narg(customer_id)::varchar IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(transaction_id)::uuid IS NULL OR transaction_id = sqlc.narg(transaction_id)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)) AND
    (sqlc.narg(dispute_date_from)::date IS NULL OR dispute_date >= sqlc.narg(dispute_date_from)) AND
    (sqlc.narg(dispute_date_to)::date IS NULL OR dispute_date <= sqlc.narg(dispute_date_to))
ORDER BY dispute_date DESC
LIMIT sqlc.arg(limit_val) OFFSET sqlc.arg(offset_val);

-- name: CountChargebacks :one
SELECT COUNT(*) FROM chargebacks
WHERE
    (sqlc.narg(agent_id)::varchar IS NULL OR agent_id = sqlc.narg(agent_id)) AND
    (sqlc.narg(customer_id)::varchar IS NULL OR customer_id = sqlc.narg(customer_id)) AND
    (sqlc.narg(transaction_id)::uuid IS NULL OR transaction_id = sqlc.narg(transaction_id)) AND
    (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)) AND
    (sqlc.narg(dispute_date_from)::date IS NULL OR dispute_date >= sqlc.narg(dispute_date_from)) AND
    (sqlc.narg(dispute_date_to)::date IS NULL OR dispute_date <= sqlc.narg(dispute_date_to));

-- name: UpdateChargeback :one
UPDATE chargebacks
SET
    status = sqlc.arg(status),
    response_submitted_at = sqlc.narg(response_submitted_at),
    resolved_at = sqlc.narg(resolved_at),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateChargebackStatus :one
UPDATE chargebacks
SET
    status = sqlc.arg(status),
    dispute_date = sqlc.arg(dispute_date),
    chargeback_date = sqlc.arg(chargeback_date),
    chargeback_amount = sqlc.arg(chargeback_amount),
    reason_code = sqlc.arg(reason_code),
    reason_description = sqlc.narg(reason_description),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: AddEvidenceFile :exec
UPDATE chargebacks
SET evidence_files = array_append(evidence_files, sqlc.arg(file_url)), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: UpdateChargebackResponse :exec
UPDATE chargebacks
SET
    response_notes = sqlc.arg(response_notes),
    response_submitted_at = CURRENT_TIMESTAMP,
    status = 'responded',
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: UpdateChargebackNotes :exec
UPDATE chargebacks
SET internal_notes = sqlc.arg(internal_notes), updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: MarkChargebackResolved :exec
UPDATE chargebacks
SET
    status = sqlc.arg(status),
    resolved_at = sqlc.arg(resolved_at),
    updated_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);
