-- +goose Up
-- +goose StatementBegin
-- Refactoring for POS Option 2 architecture
-- Payment Service = Gateway Integration ONLY
-- Remove POS domain knowledge, add opaque references

-- Add external_reference_id for POS order linkage
ALTER TABLE transactions
  ADD COLUMN external_reference_id VARCHAR(100);

-- Add return_url for browser redirect after payment
ALTER TABLE transactions
  ADD COLUMN return_url TEXT;

-- Create index for POS queries by external reference
CREATE INDEX idx_transactions_external_ref
  ON transactions(external_reference_id)
  WHERE external_reference_id IS NOT NULL;

-- Migrate existing metadata to external_reference_id (if needed)
-- This extracts order_id from metadata JSONB to the new column
UPDATE transactions
SET external_reference_id = metadata->>'order_id'
WHERE metadata IS NOT NULL
  AND metadata->>'order_id' IS NOT NULL
  AND external_reference_id IS NULL;

COMMENT ON COLUMN transactions.external_reference_id IS 'Opaque reference to POS order/transaction (e.g., order-123)';
COMMENT ON COLUMN transactions.return_url IS 'URL to redirect browser after payment callback processing';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_transactions_external_ref;

ALTER TABLE transactions
  DROP COLUMN IF EXISTS external_reference_id,
  DROP COLUMN IF EXISTS return_url;

-- +goose StatementEnd
