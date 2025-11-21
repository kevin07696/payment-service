-- +goose NO TRANSACTION
-- +goose Up
-- Index on prenote_transaction_id for ACH return processing
-- Optimizes GetPaymentMethodByPreNoteTransaction query
-- Expected impact: 50-100ms → 2-5ms (-95% faster)
CREATE INDEX CONCURRENTLY idx_payment_methods_prenote_transaction
ON customer_payment_methods(prenote_transaction_id)
WHERE prenote_transaction_id IS NOT NULL
  AND deleted_at IS NULL;

COMMENT ON INDEX idx_payment_methods_prenote_transaction IS
  'Optimizes GetPaymentMethodByPreNoteTransaction for ACH return processing. Partial index excludes NULL values.';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_payment_methods_prenote_transaction;
