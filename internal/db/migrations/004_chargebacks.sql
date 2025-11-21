-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chargebacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Link to specific transaction being disputed
    -- Can traverse to parent/child transactions via transactions.parent_transaction_id
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
    agent_id VARCHAR(100) NOT NULL,  -- Denormalized for querying (matches merchants.id)
    customer_id VARCHAR(100),  -- Denormalized for querying (NULL for guest)

    -- North API fields
    case_number VARCHAR(255) UNIQUE NOT NULL, -- North's caseNumber (unique ID)
    dispute_date TIMESTAMPTZ NOT NULL, -- North's disputeDate
    chargeback_date TIMESTAMPTZ NOT NULL, -- North's chargebackDate
    chargeback_amount VARCHAR(255) NOT NULL, -- Amount being disputed (stored as string to preserve precision)
    currency VARCHAR(3) NOT NULL DEFAULT 'USD', -- ISO currency code
    reason_code VARCHAR(50) NOT NULL, -- North's reasonCode (e.g., "P22", "F10")
    reason_description TEXT, -- North's reasonDescription

    -- Our status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'new', -- 'new', 'pending', 'responded', 'won', 'lost', 'accepted'
    respond_by_date DATE, -- Deadline to respond (calculated or from North)
    response_submitted_at TIMESTAMPTZ, -- When we submitted evidence
    resolved_at TIMESTAMPTZ, -- When outcome was determined

    -- Evidence and response (URLs to S3/blob storage)
    evidence_files TEXT[], -- Array of blob storage URLs: ["s3://bucket/receipt.pdf", "s3://bucket/tracking.jpg"]
    response_notes TEXT, -- Our written response to dispute
    internal_notes TEXT, -- Internal team notes

    -- Store full North API response
    raw_data JSONB NOT NULL, -- Full North disputes API response for this case

    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chargebacks_status_valid CHECK (status IN ('new', 'pending', 'responded', 'won', 'lost', 'accepted'))
);

-- Indexes for performance
CREATE INDEX idx_chargebacks_transaction_id ON chargebacks(transaction_id);
CREATE INDEX idx_chargebacks_agent_id ON chargebacks(agent_id);
CREATE INDEX idx_chargebacks_agent_customer ON chargebacks(agent_id, customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_chargebacks_customer_id ON chargebacks(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_chargebacks_status ON chargebacks(status);
CREATE INDEX idx_chargebacks_case_number ON chargebacks(case_number);
CREATE INDEX idx_chargebacks_respond_by_date ON chargebacks(respond_by_date) WHERE status = 'pending';
CREATE INDEX idx_chargebacks_created_at ON chargebacks(created_at DESC);
CREATE INDEX idx_chargebacks_deleted_at ON chargebacks(deleted_at) WHERE deleted_at IS NOT NULL;

-- Trigger for updated_at
CREATE TRIGGER update_chargebacks_updated_at
    BEFORE UPDATE ON chargebacks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_chargebacks_updated_at ON chargebacks;
DROP TABLE IF EXISTS chargebacks;
-- +goose StatementEnd
