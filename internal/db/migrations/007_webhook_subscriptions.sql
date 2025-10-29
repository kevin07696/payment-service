-- Migration: Add webhook subscriptions table
-- Purpose: Store merchant webhook URLs for chargeback notifications

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(100) NOT NULL,
    event_type VARCHAR(50) NOT NULL, -- 'chargeback.created', 'chargeback.updated', etc.
    webhook_url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL, -- Used to sign webhook payloads
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one active webhook per agent per event type
    CONSTRAINT unique_active_webhook UNIQUE (agent_id, event_type, webhook_url)
);

-- Index for fast lookups by agent and event type
CREATE INDEX idx_webhook_subscriptions_agent_event
ON webhook_subscriptions(agent_id, event_type)
WHERE is_active = true;

-- Table for webhook delivery logs (track success/failures)
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL, -- 'pending', 'success', 'failed'
    http_status_code INT,
    error_message TEXT,
    attempts INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT valid_status CHECK (status IN ('pending', 'success', 'failed'))
);

-- Index for retry queue
CREATE INDEX idx_webhook_deliveries_retry
ON webhook_deliveries(next_retry_at)
WHERE status = 'pending' AND next_retry_at IS NOT NULL;

-- Index for delivery history lookup
CREATE INDEX idx_webhook_deliveries_subscription
ON webhook_deliveries(subscription_id, created_at DESC);

COMMENT ON TABLE webhook_subscriptions IS 'Merchant webhook subscriptions for chargeback events';
COMMENT ON TABLE webhook_deliveries IS 'Webhook delivery log for tracking and retries';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_subscriptions;
-- +goose StatementEnd
