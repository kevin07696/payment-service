-- +goose Up
-- +goose StatementBegin
-- Initial migration placeholder
-- This creates the goose version tracking table automatically
-- Add your actual schema migrations here or in subsequent migration files

-- Example: Create a test table to verify migrations work
CREATE TABLE IF NOT EXISTS schema_info (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO schema_info (version) VALUES ('00001_init_schema')
ON CONFLICT (version) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS schema_info;
-- +goose StatementEnd
