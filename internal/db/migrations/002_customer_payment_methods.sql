-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS customer_payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Multi-tenant: which merchant + which customer
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id UUID NOT NULL,

    -- ✅ BRIC token from EPX (AUTH_GUID from STORAGE transaction)
    -- Example: "0V703LH1HDL006J74W1"
    bric TEXT NOT NULL,

    -- Payment method type
    payment_type VARCHAR(20) NOT NULL,

    -- ✅ Display metadata (last 4 only, NEVER full numbers)
    last_four VARCHAR(4) NOT NULL,

    -- ✅ Credit card metadata (for display/UI purposes)
    card_brand VARCHAR(20),         -- "visa", "mastercard", "amex", "discover"
    card_exp_month INTEGER,         -- 1-12 (optional, for expiration warnings)
    card_exp_year INTEGER,          -- 2025, 2026, etc.

    -- ✅ ACH metadata (user-provided labels for display)
    bank_name VARCHAR(255),         -- "Chase", "Bank of America", etc.
    account_type VARCHAR(20),       -- "checking" or "savings"

    -- Status tracking
    is_default BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    is_verified BOOLEAN DEFAULT false,  -- For ACH pre-note verification

    -- Timestamps
    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ,

    CONSTRAINT check_payment_type CHECK (payment_type IN ('credit_card', 'ach')),
    CONSTRAINT check_card_exp_month CHECK (card_exp_month IS NULL OR (card_exp_month >= 1 AND card_exp_month <= 12)),
    CONSTRAINT check_account_type CHECK (account_type IS NULL OR account_type IN ('checking', 'savings')),
    CONSTRAINT unique_bric UNIQUE (merchant_id, customer_id, bric)
);

-- Indexes for performance
CREATE INDEX idx_customer_payment_methods_merchant_customer ON customer_payment_methods(merchant_id, customer_id);
CREATE INDEX idx_customer_payment_methods_merchant_id ON customer_payment_methods(merchant_id);
CREATE INDEX idx_customer_payment_methods_customer_id ON customer_payment_methods(customer_id);
CREATE INDEX idx_customer_payment_methods_payment_type ON customer_payment_methods(payment_type);
CREATE INDEX idx_customer_payment_methods_is_default ON customer_payment_methods(merchant_id, customer_id, is_default) WHERE is_default = true;
CREATE INDEX idx_customer_payment_methods_is_active ON customer_payment_methods(is_active) WHERE is_active = true;
CREATE INDEX idx_customer_payment_methods_deleted_at ON customer_payment_methods(deleted_at) WHERE deleted_at IS NOT NULL;

-- Update timestamp trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger for updated_at
CREATE TRIGGER update_customer_payment_methods_updated_at
    BEFORE UPDATE ON customer_payment_methods
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_customer_payment_methods_updated_at ON customer_payment_methods;
DROP TABLE IF EXISTS customer_payment_methods;
DROP FUNCTION IF EXISTS update_updated_at_column();
-- +goose StatementEnd
