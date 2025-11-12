-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(255) UNIQUE NOT NULL,

    -- EPX Credentials
    cust_nbr VARCHAR(50) NOT NULL,
    merch_nbr VARCHAR(50) NOT NULL,
    dba_nbr VARCHAR(50) NOT NULL,
    terminal_nbr VARCHAR(50) NOT NULL,

    -- Secret Manager integration
    mac_secret_path VARCHAR(500) NOT NULL,  -- Path to MAC secret in secret manager

    -- Environment and status
    environment VARCHAR(20) NOT NULL DEFAULT 'production',  -- 'production', 'staging', 'test'
    is_active BOOLEAN NOT NULL DEFAULT true,
    name VARCHAR(255) NOT NULL,

    -- Soft delete and timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Index for lookups
CREATE INDEX IF NOT EXISTS idx_merchants_slug ON merchants(slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchants_environment ON merchants(environment) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_merchants_is_active ON merchants(is_active) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_merchants_is_active;
DROP INDEX IF EXISTS idx_merchants_environment;
DROP INDEX IF EXISTS idx_merchants_slug;
DROP TABLE IF EXISTS merchants;
-- +goose StatementEnd
