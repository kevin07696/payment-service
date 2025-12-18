-- name: ConsumeRateLimitToken :one
-- Atomically consume a token from the rate limit bucket (UNLOGGED for speed)
-- Returns the number of tokens remaining after consumption
-- Uses UNLOGGED table for 2-3x faster writes (no WAL overhead)
INSERT INTO rate_limit_cache (bucket_key, tokens, last_refill)
VALUES (sqlc.arg(bucket_key), sqlc.arg(initial_tokens), NOW())
ON CONFLICT (bucket_key) DO UPDATE
SET tokens = GREATEST(rate_limit_cache.tokens - 1, 0),
    last_refill = NOW()
RETURNING tokens;

-- name: RefillRateLimitBucket :exec
-- Refill a rate limit bucket to maximum capacity
UPDATE rate_limit_cache
SET tokens = sqlc.arg(tokens),
    last_refill = NOW()
WHERE bucket_key = sqlc.arg(bucket_key);

-- name: GetRateLimitBucket :one
-- Get current state of a rate limit bucket
SELECT * FROM rate_limit_cache
WHERE bucket_key = sqlc.arg(bucket_key);

-- name: CleanupOldRateLimitBuckets :exec
-- Remove old rate limit bucket entries (runs every 5 minutes)
-- Keeps last hour of data for analytics
DELETE FROM rate_limit_cache
WHERE last_refill < NOW() - INTERVAL '1 hour';
