-- +goose Up
-- +goose StatementBegin

-- Create UNLOGGED table for high-speed distributed rate limiting
-- UNLOGGED = No WAL (Write-Ahead Logging) = 2-3x faster writes
-- Trade-off: Data lost on crash, but that's acceptable for rate limiting
CREATE UNLOGGED TABLE IF NOT EXISTS rate_limit_cache (
    bucket_key VARCHAR(255) PRIMARY KEY,
    tokens INTEGER NOT NULL,
    last_refill TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for cleanup queries
CREATE INDEX IF NOT EXISTS idx_rate_limit_cache_last_refill
ON rate_limit_cache(last_refill);

-- Migrate existing rate_limit_buckets data (if any)
INSERT INTO rate_limit_cache (bucket_key, tokens, last_refill)
SELECT bucket_key, tokens, last_refill::timestamptz
FROM rate_limit_buckets
ON CONFLICT (bucket_key) DO NOTHING;

-- Keep old table for backward compatibility during rollout
-- Can drop rate_limit_buckets in next migration after validation

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS rate_limit_cache;

-- +goose StatementEnd
