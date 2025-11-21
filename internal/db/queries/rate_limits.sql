-- name: ConsumeRateLimitToken :one
-- Atomically consume a token from the rate limit bucket
-- Returns the number of tokens remaining after consumption
INSERT INTO rate_limit_buckets (bucket_key, tokens, last_refill)
VALUES (sqlc.arg(bucket_key), sqlc.arg(initial_tokens), NOW())
ON CONFLICT (bucket_key) DO UPDATE
SET tokens = GREATEST(rate_limit_buckets.tokens - 1, 0),
    last_refill = NOW()
RETURNING tokens;

-- name: RefillRateLimitBucket :exec
-- Refill a rate limit bucket to maximum capacity
UPDATE rate_limit_buckets
SET tokens = sqlc.arg(tokens),
    last_refill = NOW()
WHERE bucket_key = sqlc.arg(bucket_key);

-- name: GetRateLimitBucket :one
-- Get current state of a rate limit bucket
SELECT * FROM rate_limit_buckets
WHERE bucket_key = sqlc.arg(bucket_key);

-- name: CleanupOldRateLimitBuckets :exec
-- Remove old rate limit bucket entries
DELETE FROM rate_limit_buckets
WHERE last_refill < NOW() - INTERVAL '1 hour';
