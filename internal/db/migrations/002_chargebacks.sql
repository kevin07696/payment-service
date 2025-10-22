-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chargebacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    merchant_id VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,

    -- Chargeback details
    chargeback_id VARCHAR(255) UNIQUE, -- North's chargeback ID
    amount NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',

    -- Reason information
    reason_code VARCHAR(50) NOT NULL, -- e.g., "10.4", "13.1"
    reason_description TEXT,
    category VARCHAR(50), -- "fraud", "authorization", "processing_error", "consumer_dispute"

    -- Dates
    chargeback_date TIMESTAMPTZ NOT NULL, -- When chargeback was filed
    received_date TIMESTAMPTZ NOT NULL,   -- When we were notified
    respond_by_date TIMESTAMPTZ,          -- Deadline to respond
    response_submitted_at TIMESTAMPTZ,    -- When we submitted evidence
    resolved_at TIMESTAMPTZ,              -- When outcome was determined

    -- Status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- "pending", "responded", "won", "lost", "accepted"

    outcome VARCHAR(50), -- "reversed", "upheld", "partial"

    -- Evidence and notes
    evidence_files JSONB DEFAULT '[]'::jsonb, -- Array of file URLs/paths
    response_notes TEXT,
    internal_notes TEXT,

    -- Metadata
    raw_data JSONB, -- Store full webhook/notification payload
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT chargebacks_amount_positive CHECK (amount > 0)
);

-- Indexes for performance
CREATE INDEX idx_chargebacks_transaction_id ON chargebacks(transaction_id);
CREATE INDEX idx_chargebacks_merchant_id ON chargebacks(merchant_id);
CREATE INDEX idx_chargebacks_customer_id ON chargebacks(customer_id);
CREATE INDEX idx_chargebacks_status ON chargebacks(status);
CREATE INDEX idx_chargebacks_chargeback_id ON chargebacks(chargeback_id) WHERE chargeback_id IS NOT NULL;
CREATE INDEX idx_chargebacks_respond_by_date ON chargebacks(respond_by_date) WHERE status = 'pending';
CREATE INDEX idx_chargebacks_created_at ON chargebacks(created_at DESC);

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
