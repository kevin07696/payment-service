-- =====================================================
-- Migration: Convert customer_id from UUID to VARCHAR
-- =====================================================
--
-- This migration changes customer_id from UUID to VARCHAR(100)
-- to support external service identifiers (e.g., Stripe customer IDs,
-- WordPress user IDs, etc.) while maintaining consistency with the
-- chargebacks table which already uses VARCHAR(100).
--
-- Affected tables:
--   - customer_payment_methods
--   - transactions
--   - subscriptions
--
-- Note: chargebacks table already uses VARCHAR(100) for customer_id

-- ========================================
-- 1. Drop existing indexes
-- ========================================

DROP INDEX IF EXISTS idx_customer_payment_methods_customer_id;
DROP INDEX IF EXISTS idx_customer_payment_methods_merchant_customer;
DROP INDEX IF EXISTS idx_customer_payment_methods_is_default;
DROP INDEX IF EXISTS idx_transactions_customer_id;
DROP INDEX IF EXISTS idx_transactions_merchant_customer;
DROP INDEX IF EXISTS idx_subscriptions_merchant_customer;

-- ========================================
-- 2. Convert customer_id columns to VARCHAR(100)
-- ========================================

-- Convert customer_payment_methods.customer_id
ALTER TABLE customer_payment_methods
  ALTER COLUMN customer_id TYPE VARCHAR(100) USING customer_id::TEXT;

-- Convert transactions.customer_id
ALTER TABLE transactions
  ALTER COLUMN customer_id TYPE VARCHAR(100) USING customer_id::TEXT;

-- Convert subscriptions.customer_id
ALTER TABLE subscriptions
  ALTER COLUMN customer_id TYPE VARCHAR(100) USING customer_id::TEXT;

-- ========================================
-- 3. Recreate indexes with same definitions
-- ========================================

CREATE INDEX idx_customer_payment_methods_customer_id
  ON customer_payment_methods(customer_id);

CREATE INDEX idx_customer_payment_methods_merchant_customer
  ON customer_payment_methods(merchant_id, customer_id);

CREATE INDEX idx_customer_payment_methods_is_default
  ON customer_payment_methods(merchant_id, customer_id, is_default)
  WHERE is_default = true;

CREATE INDEX idx_transactions_customer_id
  ON transactions(customer_id) WHERE customer_id IS NOT NULL;

CREATE INDEX idx_transactions_merchant_customer
  ON transactions(merchant_id, customer_id) WHERE customer_id IS NOT NULL;

CREATE INDEX idx_subscriptions_merchant_customer
  ON subscriptions(merchant_id, customer_id);
