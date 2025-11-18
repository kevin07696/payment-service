-- +goose Up
-- Migration: 008_auth_tables.sql
-- Description: Authentication and authorization tables for JWT and API key auth
-- Author: Authentication System
-- Date: 2025-11-18

-- Admin users table
CREATE TABLE IF NOT EXISTS admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'admin',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Update merchants table to add auth fields (if not exists)
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'pending_activation';
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS tier VARCHAR(50) DEFAULT 'standard';
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS requests_per_second INTEGER DEFAULT 100;
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS burst_limit INTEGER DEFAULT 200;
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES admins(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES admins(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS approved_at TIMESTAMP;

-- Registered services (POS, WordPress, etc.)
CREATE TABLE IF NOT EXISTS registered_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id VARCHAR(100) UNIQUE NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    public_key TEXT NOT NULL,
    public_key_fingerprint VARCHAR(64) NOT NULL,
    environment VARCHAR(50) NOT NULL, -- staging, production

    -- Rate limit configuration
    requests_per_second INTEGER DEFAULT 1000,
    burst_limit INTEGER DEFAULT 2000,

    is_active BOOLEAN DEFAULT true,
    created_by UUID REFERENCES admins(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Service-to-merchant access control
CREATE TABLE IF NOT EXISTS service_merchants (
    service_id UUID REFERENCES registered_services(id) ON DELETE CASCADE,
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    scopes TEXT[], -- ['payment:create', 'payment:read', etc.]
    granted_by UUID REFERENCES admins(id),
    granted_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    PRIMARY KEY (service_id, merchant_id)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_service_merchants_service
    ON service_merchants(service_id);

CREATE INDEX IF NOT EXISTS idx_service_merchants_merchant
    ON service_merchants(merchant_id);

CREATE INDEX IF NOT EXISTS idx_service_merchants_expires
    ON service_merchants(expires_at)
    WHERE expires_at IS NOT NULL;

-- Merchant self-managed credentials
CREATE TABLE IF NOT EXISTS merchant_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,

    -- Hashed credentials
    api_key_prefix VARCHAR(20) NOT NULL, -- First 10 chars for identification
    api_key_hash VARCHAR(255) NOT NULL,
    api_secret_hash VARCHAR(255) NOT NULL,

    description VARCHAR(255),
    environment VARCHAR(50) DEFAULT 'production', -- production, staging, test
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    is_active BOOLEAN DEFAULT true,

    created_at TIMESTAMP DEFAULT NOW(),
    created_by VARCHAR(100), -- 'initial_setup', 'merchant_portal', 'api_rotation'
    rotated_from UUID REFERENCES merchant_credentials(id)
);

-- Unique index on active credentials
CREATE UNIQUE INDEX IF NOT EXISTS idx_merchant_credentials_active
    ON merchant_credentials(api_key_hash)
    WHERE is_active = true;

-- Merchant activation tokens (one-time use)
CREATE TABLE IF NOT EXISTS merchant_activation_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Comprehensive audit log (partitioned by month)
CREATE TABLE IF NOT EXISTS audit_log (
    id UUID DEFAULT gen_random_uuid(),
    -- Actor
    actor_type VARCHAR(50), -- 'admin', 'merchant', 'service', 'system'
    actor_id VARCHAR(255),
    actor_name VARCHAR(255),

    -- Action
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50),
    entity_id VARCHAR(255),

    -- Details
    changes JSONB,
    metadata JSONB,

    -- Context
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(100),

    -- Result
    success BOOLEAN DEFAULT true,
    error_message TEXT,

    performed_at TIMESTAMP DEFAULT NOW()
) PARTITION BY RANGE (performed_at);

-- Create monthly partitions for audit log (next 3 months)
CREATE TABLE IF NOT EXISTS audit_log_2025_01 PARTITION OF audit_log
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE IF NOT EXISTS audit_log_2025_02 PARTITION OF audit_log
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
CREATE TABLE IF NOT EXISTS audit_log_2025_03 PARTITION OF audit_log
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- Add primary key to each partition
ALTER TABLE audit_log_2025_01 ADD PRIMARY KEY (id);
ALTER TABLE audit_log_2025_02 ADD PRIMARY KEY (id);
ALTER TABLE audit_log_2025_03 ADD PRIMARY KEY (id);

-- Rate limit tracking
CREATE TABLE IF NOT EXISTS rate_limit_buckets (
    bucket_key VARCHAR(255) PRIMARY KEY,
    tokens INTEGER NOT NULL,
    last_refill TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- EPX IP whitelist
CREATE TABLE IF NOT EXISTS epx_ip_whitelist (
    id SERIAL PRIMARY KEY,
    ip_address INET NOT NULL UNIQUE,
    description VARCHAR(255),
    added_by UUID REFERENCES admins(id),
    added_at TIMESTAMP DEFAULT NOW(),
    is_active BOOLEAN DEFAULT true
);

-- Insert default EPX IPs (update with real IPs in production)
INSERT INTO epx_ip_whitelist (ip_address, description) VALUES
    ('127.0.0.1', 'Local development'),
    ('::1', 'Local development IPv6')
ON CONFLICT (ip_address) DO NOTHING;

-- JWT token blacklist (for emergency revocation)
CREATE TABLE IF NOT EXISTS jwt_blacklist (
    jti VARCHAR(255) PRIMARY KEY, -- JWT ID
    service_id VARCHAR(100),
    merchant_id UUID,
    expires_at TIMESTAMP NOT NULL,
    blacklisted_at TIMESTAMP DEFAULT NOW(),
    blacklisted_by UUID REFERENCES admins(id),
    reason TEXT
);

-- Clean up expired blacklist entries periodically
CREATE INDEX IF NOT EXISTS idx_jwt_blacklist_expires ON jwt_blacklist(expires_at);

-- Session tracking for admin users
CREATE TABLE IF NOT EXISTS admin_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID REFERENCES admins(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for audit log queries
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log (actor_type, actor_id, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log (action, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_entity ON audit_log (entity_type, entity_id, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_ip ON audit_log (ip_address, performed_at DESC) WHERE ip_address IS NOT NULL;

-- Function to clean up expired data
-- Note: This function is commented out due to goose limitations with dollar-quoted strings
-- Create it manually after migration if needed
-- CREATE OR REPLACE FUNCTION cleanup_expired_auth_data()
-- RETURNS void
-- LANGUAGE plpgsql
-- AS $function$
-- BEGIN
--     -- Delete expired JWT blacklist entries
--     DELETE FROM jwt_blacklist WHERE expires_at < NOW() - INTERVAL '1 day';
--
--     -- Delete expired activation tokens
--     DELETE FROM merchant_activation_tokens
--     WHERE expires_at < NOW() - INTERVAL '7 days';
--
--     -- Delete old rate limit buckets
--     DELETE FROM rate_limit_buckets
--     WHERE last_refill < NOW() - INTERVAL '1 hour';
--
--     -- Delete expired admin sessions
--     DELETE FROM admin_sessions WHERE expires_at < NOW();
-- END;
-- $function$;

-- Create a scheduled job to clean up expired data (requires pg_cron extension)
-- Run this separately if pg_cron is available:
-- SELECT cron.schedule('cleanup-auth-data', '0 3 * * *', 'SELECT cleanup_expired_auth_data()');

-- Grant permissions (adjust as needed for your database user)
-- GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO payment_service_user;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO payment_service_user;

-- +goose Down
-- Drop all auth-related tables and functions
-- DROP FUNCTION IF EXISTS cleanup_expired_auth_data();
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS jwt_blacklist;
DROP TABLE IF EXISTS epx_ip_whitelist;
DROP TABLE IF EXISTS rate_limit_buckets;
DROP TABLE IF EXISTS audit_log_2025_03;
DROP TABLE IF EXISTS audit_log_2025_02;
DROP TABLE IF EXISTS audit_log_2025_01;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS merchant_activation_tokens;
DROP TABLE IF EXISTS merchant_credentials;
DROP TABLE IF EXISTS service_merchants;
DROP TABLE IF EXISTS registered_services;
ALTER TABLE merchants DROP COLUMN IF EXISTS status;
ALTER TABLE merchants DROP COLUMN IF EXISTS tier;
ALTER TABLE merchants DROP COLUMN IF EXISTS requests_per_second;
ALTER TABLE merchants DROP COLUMN IF EXISTS burst_limit;
ALTER TABLE merchants DROP COLUMN IF EXISTS created_by;
ALTER TABLE merchants DROP COLUMN IF EXISTS approved_by;
ALTER TABLE merchants DROP COLUMN IF EXISTS approved_at;
DROP TABLE IF EXISTS admins;