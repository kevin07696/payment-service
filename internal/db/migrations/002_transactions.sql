-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Transaction grouping: groups related transactions (auth -> capture -> refund)
    group_id UUID NOT NULL DEFAULT gen_random_uuid(),

    -- Multi-tenant: which agent/merchant
    agent_id VARCHAR(100) NOT NULL,

    -- Customer identification (NULL for guest transactions)
    customer_id VARCHAR(100),

    -- Transaction details
    amount NUMERIC(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    status VARCHAR(20) NOT NULL,  -- derived from AUTH_RESP: pending, completed, failed, refunded
    type VARCHAR(20) NOT NULL,    -- charge, refund, pre_note, auth, capture

    -- Payment method
    payment_method_type VARCHAR(20) NOT NULL,  -- credit_card, ach
    payment_method_id UUID REFERENCES customer_payment_methods(id) ON DELETE SET NULL,  -- Reference to saved payment method (NULL for guest)

    -- EPX Gateway response fields
    auth_guid VARCHAR(255),              -- EPX transaction token (BRIC format: "0V703LH1HDL006J74W1") - required for refunds/voids/captures, can be reused for recurring charges
    auth_resp VARCHAR(10),               -- EPX approval code ("00" = approved, "05" = declined, "12" = invalid) - maps to our status field
    auth_code VARCHAR(50),               -- Bank authorization code (e.g., "123456") - required for chargeback defense, NULL if declined
    auth_resp_text TEXT,                 -- Human-readable response message ("APPROVED", "INSUFFICIENT FUNDS") - display to users for decline reasons
    auth_card_type VARCHAR(20),          -- Card brand ("V" = Visa, "M" = Mastercard, "A" = Amex, "D" = Discover) - used for fees/reporting, NULL for ACH
    auth_avs VARCHAR(10),                -- Address verification ("Y" = match, "N" = no match, "U" = unavailable) - fraud prevention and risk scoring
    auth_cvv2 VARCHAR(10),               -- CVV verification ("M" = match, "N" = no match, "P" = not processed) - fraud prevention and risk scoring

    -- Idempotency and metadata
    idempotency_key VARCHAR(255) UNIQUE,  -- TRAN_NBR from merchant
    metadata JSONB DEFAULT '{}'::jsonb,   -- Store additional EPX fields if needed

    -- Timestamps
    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT transactions_amount_positive CHECK (amount >= 0),
    CONSTRAINT transactions_status_valid CHECK (status IN ('pending', 'completed', 'failed', 'refunded', 'voided')),
    CONSTRAINT transactions_type_valid CHECK (type IN ('charge', 'refund', 'pre_note', 'auth', 'capture'))
);

-- Indexes for performance
CREATE INDEX idx_transactions_group_id ON transactions(group_id);
CREATE INDEX idx_transactions_agent_id ON transactions(agent_id);
CREATE INDEX idx_transactions_agent_customer ON transactions(agent_id, customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_transactions_customer_id ON transactions(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_transactions_idempotency_key ON transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_transactions_auth_guid ON transactions(auth_guid) WHERE auth_guid IS NOT NULL;
CREATE INDEX idx_transactions_payment_method_id ON transactions(payment_method_id) WHERE payment_method_id IS NOT NULL;
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_deleted_at ON transactions(deleted_at) WHERE deleted_at IS NOT NULL;

-- Create subscriptions table
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(100) NOT NULL,
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
CREATE INDEX idx_subscriptions_agent_id ON subscriptions(agent_id);
CREATE INDEX idx_subscriptions_agent_customer ON subscriptions(agent_id, customer_id);
CREATE INDEX idx_subscriptions_next_billing_date ON subscriptions(next_billing_date) WHERE status = 'active';
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_gateway_subscription_id ON subscriptions(gateway_subscription_id) WHERE gateway_subscription_id IS NOT NULL;
CREATE INDEX idx_subscriptions_deleted_at ON subscriptions(deleted_at) WHERE deleted_at IS NOT NULL;

-- Create audit_logs table for PCI compliance
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(100) NOT NULL,
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
CREATE INDEX idx_audit_logs_agent_id ON audit_logs(agent_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_event_type ON audit_logs(event_type);

-- Apply update trigger to tables (function defined in 001_customer_payment_methods.sql)
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
