-- +goose Up
-- +goose StatementBegin

-- Add order_id column to transactions table
-- Links transactions to merchant's external order/invoice system
-- Nullable: some transactions don't have orders (prenote, tokenization, direct API calls)
ALTER TABLE transactions ADD COLUMN order_id VARCHAR(255);

-- Index for efficient lookup by merchant + order
-- Partial index excludes NULL order_ids for better performance
CREATE INDEX idx_transactions_merchant_order
    ON transactions(merchant_id, order_id)
    WHERE order_id IS NOT NULL;

-- Composite index for merchant + customer + order lookups
CREATE INDEX idx_transactions_merchant_customer_order
    ON transactions(merchant_id, customer_id, order_id)
    WHERE order_id IS NOT NULL AND customer_id IS NOT NULL;

COMMENT ON COLUMN transactions.order_id IS 'Merchant external order/invoice ID. Nullable for transactions without orders (prenote, tokenization). Multiple transactions can share the same order_id (AUTH→CAPTURE→REFUND chain).';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_transactions_merchant_customer_order;
DROP INDEX IF EXISTS idx_transactions_merchant_order;
ALTER TABLE transactions DROP COLUMN IF EXISTS order_id;

-- +goose StatementEnd
