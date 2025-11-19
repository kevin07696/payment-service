-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY,  -- Client-provided UUID via idempotency_key (REQUIRED for all operations)

    -- Transaction parent relationship: tracks transaction chains (AUTH -> CAPTURE -> REFUND)
    -- CAPTURE references AUTH, REFUND references SALE/CAPTURE, VOID references AUTH/SALE
    parent_transaction_id UUID REFERENCES transactions(id) ON DELETE RESTRICT,

    -- Multi-tenant: which merchant owns this transaction
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,

    -- Customer identification (NULL for guest transactions)
    customer_id UUID,

    -- Transaction details (amount in cents to avoid floating point issues)
    amount_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    type VARCHAR(20) NOT NULL,    -- SALE, AUTH, CAPTURE, REFUND, VOID, STORAGE, DEBIT

    -- Payment method
    payment_method_type VARCHAR(20) NOT NULL,  -- credit_card, ach
    payment_method_id UUID REFERENCES customer_payment_methods(id) ON DELETE SET NULL,  -- Reference to saved payment method (NULL for guest)

    -- Optional subscription reference (for recurring billing transactions)
    subscription_id UUID,  -- Links transaction to subscription (NULL for one-time payments) - FK added after subscriptions table created

    -- EPX Gateway response fields (queryable columns only)
    -- EPX TRAN_NBR: Deterministic 10-digit numeric ID derived from UUID (for EPX API calls)
    tran_nbr TEXT,

    -- EPX AUTH_GUID (BRIC) returned from gateway for this transaction
    -- IMPORTANT: Each transaction can have its own BRIC token
    -- - AUTH transaction: gets initial BRIC from EPX
    -- - CAPTURE: uses AUTH's BRIC as input, gets new BRIC as output
    -- - REFUND: uses CAPTURE's BRIC as input, gets new BRIC as output
    -- This allows querying by BRIC and supports EPX reconciliation
    auth_guid TEXT,

    auth_resp VARCHAR(10),               -- EPX approval code ("00" = approved, "05" = declined, "12" = invalid) - source of truth for status. NULL = pending/failed
    auth_code VARCHAR(50),               -- Bank authorization code (e.g., "123456") - required for chargeback defense, NULL if declined
    auth_card_type VARCHAR(20),          -- Card brand ("V" = Visa, "M" = Mastercard, "A" = Amex, "D" = Discover) - used for fees/reporting, NULL for ACH

    -- Status: Transaction outcome auto-generated from auth_resp
    -- EPX: "00" = approved, anything else = declined
    -- pending = not sent to EPX yet, failed = system error before reaching EPX
    status VARCHAR(20) GENERATED ALWAYS AS (
        CASE
            WHEN auth_resp IS NULL AND processed_at IS NULL THEN 'pending'
            WHEN auth_resp IS NULL AND processed_at IS NOT NULL THEN 'failed'
            WHEN auth_resp = '00' THEN 'approved'
            ELSE 'declined'
        END
    ) STORED,

    -- Timestamp when EPX responded (callback received)
    processed_at TIMESTAMPTZ,

    -- Metadata (auth_resp_text, auth_avs, auth_cvv2, card_last_4, card_holder_name, description, EPX raw response, integration-specific data)
    -- auth_resp_text: Human-readable response message ("APPROVED", "INSUFFICIENT FUNDS") - display only
    -- auth_avs: Address verification ("Y" = match, "N" = no match, "U" = unavailable) - fraud scoring, not queried
    -- auth_cvv2: CVV verification ("M" = match, "N" = no match, "P" = not processed) - fraud scoring, not queried
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Timestamps
    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT transactions_amount_cents_non_negative CHECK (amount_cents >= 0),
    CONSTRAINT transactions_type_valid CHECK (type IN ('SALE', 'AUTH', 'CAPTURE', 'REFUND', 'VOID', 'STORAGE', 'DEBIT')),
    -- Simple CHECK constraint as defense-in-depth (detailed validation in application layer)
    -- SALE, AUTH, STORAGE, DEBIT = standalone (no parent)
    -- CAPTURE, REFUND, VOID = must have parent
    CONSTRAINT transactions_parent_relationship CHECK (
        (type IN ('SALE', 'AUTH', 'STORAGE', 'DEBIT') AND parent_transaction_id IS NULL)
        OR
        (type IN ('CAPTURE', 'REFUND', 'VOID') AND parent_transaction_id IS NOT NULL)
    )
);

-- Indexes for performance
CREATE INDEX idx_transactions_parent_id ON transactions(parent_transaction_id) WHERE parent_transaction_id IS NOT NULL;
CREATE INDEX idx_transactions_merchant_id ON transactions(merchant_id);
CREATE INDEX idx_transactions_merchant_customer ON transactions(merchant_id, customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_transactions_customer_id ON transactions(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_transactions_tran_nbr ON transactions(tran_nbr) WHERE tran_nbr IS NOT NULL;
CREATE INDEX idx_transactions_auth_guid ON transactions(auth_guid) WHERE auth_guid IS NOT NULL;
CREATE INDEX idx_transactions_payment_method_id ON transactions(payment_method_id) WHERE payment_method_id IS NOT NULL;
CREATE INDEX idx_transactions_subscription_id ON transactions(subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_deleted_at ON transactions(deleted_at) WHERE deleted_at IS NOT NULL;

-- Comments explaining key design decisions
COMMENT ON COLUMN transactions.parent_transaction_id IS 'Foreign key to parent transaction. CAPTURE→AUTH, REFUND→SALE/CAPTURE, VOID→AUTH/SALE. NULL for standalone transactions (SALE, AUTH, STORAGE, DEBIT). Detailed validation in application layer.';
COMMENT ON COLUMN transactions.amount_cents IS 'Amount in cents (e.g., $10.50 = 1050). Using BIGINT avoids floating point precision issues.';
COMMENT ON COLUMN transactions.status IS 'Auto-generated from auth_resp: pending (not sent), failed (error), approved (00), declined (non-00).';
COMMENT ON COLUMN transactions.processed_at IS 'Timestamp when EPX responded (callback received). NULL if pending or failed before reaching EPX.';
COMMENT ON COLUMN transactions.tran_nbr IS 'EPX TRAN_NBR: Deterministic 10-digit numeric ID derived from transaction UUID via FNV-1a hash. Used for all EPX API calls.';
COMMENT ON COLUMN transactions.auth_guid IS 'EPX AUTH_GUID (BRIC) for this specific transaction. Each transaction can have its own BRIC. CAPTURE uses AUTH BRIC as input but gets new BRIC. REFUND uses CAPTURE BRIC.';

-- Create subscriptions table
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id UUID NOT NULL,
    amount_cents BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',

    -- Billing interval (e.g., 1 month, 2 weeks, 3 months)
    interval_value INTEGER NOT NULL DEFAULT 1,  -- 1, 2, 3, etc.
    interval_unit VARCHAR(10) NOT NULL DEFAULT 'month',  -- 'day', 'week', 'month', 'year'

    status VARCHAR(20) NOT NULL,  -- 'active', 'paused', 'cancelled', 'past_due'
    payment_method_id UUID NOT NULL REFERENCES customer_payment_methods(id) ON DELETE RESTRICT,  -- Cannot delete payment method with active subscriptions
    next_billing_date DATE NOT NULL,

    -- Failure handling
    failure_retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,

    -- Optional: EPX gateway subscription ID if EPX provides one
    gateway_subscription_id VARCHAR(255),

    metadata JSONB DEFAULT '{}'::jsonb,
    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cancelled_at TIMESTAMPTZ,

    CONSTRAINT subscriptions_amount_cents_positive CHECK (amount_cents > 0),
    CONSTRAINT subscriptions_retry_count_non_negative CHECK (failure_retry_count >= 0),
    CONSTRAINT subscriptions_interval_value_positive CHECK (interval_value > 0),
    CONSTRAINT subscriptions_interval_unit_valid CHECK (interval_unit IN ('day', 'week', 'month', 'year')),
    CONSTRAINT subscriptions_status_valid CHECK (status IN ('active', 'paused', 'cancelled', 'past_due'))
);

-- Indexes for subscriptions
CREATE INDEX idx_subscriptions_merchant_id ON subscriptions(merchant_id);
CREATE INDEX idx_subscriptions_merchant_customer ON subscriptions(merchant_id, customer_id);
CREATE INDEX idx_subscriptions_next_billing_date ON subscriptions(next_billing_date) WHERE status = 'active';
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_gateway_subscription_id ON subscriptions(gateway_subscription_id) WHERE gateway_subscription_id IS NOT NULL;
CREATE INDEX idx_subscriptions_deleted_at ON subscriptions(deleted_at) WHERE deleted_at IS NOT NULL;

-- Add foreign key constraint from transactions to subscriptions (now that subscriptions table exists)
ALTER TABLE transactions ADD CONSTRAINT transactions_subscription_id_fkey
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE SET NULL;

-- Apply update trigger to tables (function defined in 002_customer_payment_methods.sql)
CREATE TRIGGER update_transactions_updated_at BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_subscriptions_updated_at BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_subscriptions_updated_at ON subscriptions;
DROP TRIGGER IF EXISTS update_transactions_updated_at ON transactions;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS transactions;
-- +goose StatementEnd
