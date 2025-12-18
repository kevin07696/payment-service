-- +goose Up
-- +goose StatementBegin

-- Rename agent_id to merchant_id in webhook_subscriptions table
-- This standardizes naming across the codebase (agent_id -> merchant_id)

ALTER TABLE webhook_subscriptions RENAME COLUMN agent_id TO merchant_id;

-- Update the unique constraint name for clarity
ALTER TABLE webhook_subscriptions
    DROP CONSTRAINT IF EXISTS unique_active_webhook;
ALTER TABLE webhook_subscriptions
    ADD CONSTRAINT unique_active_webhook UNIQUE (merchant_id, event_type, webhook_url);

-- Recreate index with new column name
DROP INDEX IF EXISTS idx_webhook_subscriptions_agent_event;
CREATE INDEX idx_webhook_subscriptions_merchant_event
    ON webhook_subscriptions(merchant_id, event_type);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Revert: rename merchant_id back to agent_id
ALTER TABLE webhook_subscriptions RENAME COLUMN merchant_id TO agent_id;

-- Restore original constraint
ALTER TABLE webhook_subscriptions
    DROP CONSTRAINT IF EXISTS unique_active_webhook;
ALTER TABLE webhook_subscriptions
    ADD CONSTRAINT unique_active_webhook UNIQUE (agent_id, event_type, webhook_url);

-- Restore original index
DROP INDEX IF EXISTS idx_webhook_subscriptions_merchant_event;
CREATE INDEX idx_webhook_subscriptions_agent_event
    ON webhook_subscriptions(agent_id, event_type);

-- +goose StatementEnd
