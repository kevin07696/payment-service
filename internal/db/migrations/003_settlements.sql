-- +goose Up
-- +goose StatementBegin
-- Settlement batches table
CREATE TABLE IF NOT EXISTS settlement_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id VARCHAR(255) NOT NULL,

    -- Settlement identification
    settlement_batch_id VARCHAR(255) NOT NULL UNIQUE, -- North's batch ID
    settlement_date DATE NOT NULL,
    deposit_date DATE,

    -- Financial summary
    total_sales NUMERIC(19, 4) NOT NULL DEFAULT 0,
    total_refunds NUMERIC(19, 4) NOT NULL DEFAULT 0,
    total_chargebacks NUMERIC(19, 4) NOT NULL DEFAULT 0,
    total_fees NUMERIC(19, 4) NOT NULL DEFAULT 0,
    net_amount NUMERIC(19, 4) NOT NULL, -- What actually deposited
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',

    -- Transaction counts
    sales_count INTEGER NOT NULL DEFAULT 0,
    refund_count INTEGER NOT NULL DEFAULT 0,
    chargeback_count INTEGER NOT NULL DEFAULT 0,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- "pending", "reconciled", "discrepancy", "completed"

    -- Reconciliation
    reconciled_at TIMESTAMPTZ,
    discrepancy_amount NUMERIC(19, 4),
    discrepancy_notes TEXT,

    -- Metadata
    raw_data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT settlement_batches_totals_non_negative CHECK (
        total_sales >= 0 AND
        total_refunds >= 0 AND
        total_chargebacks >= 0 AND
        total_fees >= 0
    )
);

-- Settlement transactions (detail)
CREATE TABLE IF NOT EXISTS settlement_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_batch_id UUID NOT NULL REFERENCES settlement_batches(id) ON DELETE CASCADE,
    transaction_id UUID REFERENCES transactions(id),

    -- Transaction details
    gateway_transaction_id VARCHAR(255),
    transaction_date TIMESTAMPTZ NOT NULL,
    settlement_date DATE NOT NULL,

    -- Amounts
    gross_amount NUMERIC(19, 4) NOT NULL,
    fee_amount NUMERIC(19, 4) NOT NULL DEFAULT 0,
    net_amount NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',

    -- Transaction info
    transaction_type VARCHAR(50), -- "SALE", "REFUND", "CHARGEBACK"
    card_brand VARCHAR(50),
    card_type VARCHAR(50), -- "CREDIT", "DEBIT"

    -- Interchange (if available)
    interchange_rate NUMERIC(6, 4),
    interchange_fee NUMERIC(19, 4),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for settlement_batches
CREATE INDEX idx_settlement_batches_merchant_id ON settlement_batches(merchant_id);
CREATE INDEX idx_settlement_batches_settlement_date ON settlement_batches(settlement_date DESC);
CREATE INDEX idx_settlement_batches_deposit_date ON settlement_batches(deposit_date DESC) WHERE deposit_date IS NOT NULL;
CREATE INDEX idx_settlement_batches_status ON settlement_batches(status);
CREATE INDEX idx_settlement_batches_batch_id ON settlement_batches(settlement_batch_id);

-- Indexes for settlement_transactions
CREATE INDEX idx_settlement_txns_batch_id ON settlement_transactions(settlement_batch_id);
CREATE INDEX idx_settlement_txns_transaction_id ON settlement_transactions(transaction_id) WHERE transaction_id IS NOT NULL;
CREATE INDEX idx_settlement_txns_gateway_txn_id ON settlement_transactions(gateway_transaction_id) WHERE gateway_transaction_id IS NOT NULL;
CREATE INDEX idx_settlement_txns_settlement_date ON settlement_transactions(settlement_date DESC);
CREATE INDEX idx_settlement_txns_transaction_date ON settlement_transactions(transaction_date DESC);

-- Trigger for updated_at on settlement_batches
CREATE TRIGGER update_settlement_batches_updated_at
    BEFORE UPDATE ON settlement_batches
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_settlement_batches_updated_at ON settlement_batches;
DROP TABLE IF EXISTS settlement_transactions;
DROP TABLE IF EXISTS settlement_batches;
-- +goose StatementEnd
