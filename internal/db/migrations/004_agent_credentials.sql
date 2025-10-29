-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS agent_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(255) UNIQUE NOT NULL,

    -- Secret Manager reference (path to MAC secret)
    -- Example: "payments/agents/agent1/mac"
    mac_secret_path TEXT NOT NULL UNIQUE,

    -- EPX routing identifiers (plaintext)
    cust_nbr VARCHAR(50) NOT NULL,
    merch_nbr VARCHAR(50) NOT NULL,
    dba_nbr VARCHAR(50) NOT NULL,
    terminal_nbr VARCHAR(50) NOT NULL,

    -- Environment determines which EPX URLs to use (from config)
    environment VARCHAR(20) NOT NULL DEFAULT 'production',

    -- Metadata
    agent_name VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    deleted_at TIMESTAMPTZ,  -- Soft delete timestamp
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT check_environment CHECK (environment IN ('test', 'production'))
);

-- Indexes for agent_credentials
CREATE INDEX idx_agent_credentials_agent_id ON agent_credentials(agent_id);
CREATE INDEX idx_agent_credentials_is_active ON agent_credentials(is_active) WHERE is_active = true;
CREATE INDEX idx_agent_credentials_deleted_at ON agent_credentials(deleted_at) WHERE deleted_at IS NOT NULL;

-- Trigger for updated_at
CREATE TRIGGER update_agent_credentials_updated_at
    BEFORE UPDATE ON agent_credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_agent_credentials_updated_at ON agent_credentials;
DROP TABLE IF EXISTS agent_credentials;
-- +goose StatementEnd
