-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY,  -- Client-provided UUID via idempotency_key (REQUIRED for all operations)

    -- Transaction grouping: groups related transactions (auth -> capture -> refund)
    -- This is NOT a foreign key - just a UUID for logical grouping
    -- Auto-generates if not provided, allowing first transaction to establish the group
    group_id UUID NOT NULL DEFAULT gen_random_uuid(),

    -- Multi-tenant: which merchant owns this transaction
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,

    -- Customer identification (NULL for guest transactions)
    customer_id VARCHAR(100),

    -- Transaction details
    amount NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    type VARCHAR(20) NOT NULL,    -- auth, sale, capture, refund, void, pre_note

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

    auth_resp VARCHAR(10) NOT NULL,      -- EPX approval code ("00" = approved, "05" = declined, "12" = invalid) - source of truth for status
    auth_code VARCHAR(50),               -- Bank authorization code (e.g., "123456") - required for chargeback defense, NULL if declined
    auth_card_type VARCHAR(20),          -- Card brand ("V" = Visa, "M" = Mastercard, "A" = Amex, "D" = Discover) - used for fees/reporting, NULL for ACH

    -- Status: Transaction outcome (approved/declined) generated from auth_resp
    -- EPX: "00" = approved, anything else = declined
    -- Note: This represents gateway approval status, NOT transaction lifecycle state
    -- Use 'type' column for transaction lifecycle (auth, capture, sale, refund, void)
    status VARCHAR(20) GENERATED ALWAYS AS (
        CASE
            WHEN auth_resp = '00' THEN 'approved'
            ELSE 'declined'
        END
    ) STORED,

    -- Metadata (auth_resp_text, auth_avs, auth_cvv2, card_last_4, card_holder_name, description, EPX raw response, integration-specific data)
    -- auth_resp_text: Human-readable response message ("APPROVED", "INSUFFICIENT FUNDS") - display only
    -- auth_avs: Address verification ("Y" = match, "N" = no match, "U" = unavailable) - fraud scoring, not queried
    -- auth_cvv2: CVV verification ("M" = match, "N" = no match, "P" = not processed) - fraud scoring, not queried
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Timestamps
    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT transactions_amount_positive CHECK (amount >= 0),
    CONSTRAINT transactions_type_valid CHECK (type IN ('auth', 'sale', 'capture', 'refund', 'void', 'pre_note'))
);

-- Indexes for performance
CREATE INDEX idx_transactions_group_id ON transactions(group_id);
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
COMMENT ON COLUMN transactions.group_id IS 'Logical grouping UUID for related transactions (auth -> capture -> refund). NOT a foreign key - just an index for grouping. Auto-generates if not provided.';
COMMENT ON COLUMN transactions.tran_nbr IS 'EPX TRAN_NBR: Deterministic 10-digit numeric ID derived from transaction UUID via FNV-1a hash. Used for all EPX API calls.';
COMMENT ON COLUMN transactions.auth_guid IS 'EPX AUTH_GUID (BRIC) for this specific transaction. Each transaction can have its own BRIC. CAPTURE uses AUTH BRIC as input but gets new BRIC. REFUND uses CAPTURE BRIC.';

-- Create subscriptions table
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id VARCHAR(100) NOT NULL,
    amount NUMERIC(19, 4) NOT NULL,
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

    CONSTRAINT subscriptions_amount_positive CHECK (amount > 0),
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

-- Create audit_logs table for PCI compliance
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    user_id VARCHAR(100),
    action VARCHAR(50) NOT NULL,
    before_state JSONB,
    after_state JSONB,
    metadata JSONB DEFAULT '{}'::jsonb,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for audit logs
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_merchant_id ON audit_logs(merchant_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_event_type ON audit_logs(event_type);

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
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS transactions;
-- +goose StatementEnd
