-- +goose Up
-- Migration: 008_auth_tables.sql
-- Description: Clean separation - Services (auth) vs Merchants (business entities)
-- Architecture:
--   - services: ALL apps/clients (internal + external merchant apps) with JWT auth
--   - merchants: Pure business entity data + EPX gateway credentials
--   - service_merchants: Links services to merchants (many-to-many)
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

-- Update merchants table to add business fields
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'pending_activation';
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS tier VARCHAR(50) DEFAULT 'standard';
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES admins(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES admins(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS approved_at TIMESTAMP;

-- Services table: ALL apps/clients (internal microservices + merchant apps)
-- Examples:
--   - Internal: billing-service, subscription-service (merchant_id = NULL in service_merchants)
--   - External: "ACME Web App", "ACME Mobile App" (linked via service_merchants)
CREATE TABLE IF NOT EXISTS services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id VARCHAR(100) UNIQUE NOT NULL,  -- e.g., "acme-web-app", "billing-service"
    service_name VARCHAR(255) NOT NULL,       -- e.g., "ACME Corp Web Application"
    public_key TEXT NOT NULL,                 -- RSA public key for JWT verification
    public_key_fingerprint VARCHAR(64) NOT NULL,
    environment VARCHAR(50) NOT NULL,         -- staging, production

    -- Rate limit configuration (per service, not per merchant)
    requests_per_second INTEGER DEFAULT 100,
    burst_limit INTEGER DEFAULT 200,

    is_active BOOLEAN DEFAULT true,
    created_by UUID REFERENCES admins(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Service-to-merchant access control (many-to-many)
-- Links services to merchants with scoped permissions
CREATE TABLE IF NOT EXISTS service_merchants (
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    scopes TEXT[], -- ['payment:create', 'payment:read', 'subscription:manage', etc.]
    granted_by UUID REFERENCES admins(id),
    granted_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP,
    PRIMARY KEY (service_id, merchant_id)
);

-- Indexes for service_merchants
CREATE INDEX IF NOT EXISTS idx_service_merchants_service
    ON service_merchants(service_id);

CREATE INDEX IF NOT EXISTS idx_service_merchants_merchant
    ON service_merchants(merchant_id);

CREATE INDEX IF NOT EXISTS idx_service_merchants_expires
    ON service_merchants(expires_at)
    WHERE expires_at IS NOT NULL;

-- Merchant activation tokens (one-time use for onboarding)
CREATE TABLE IF NOT EXISTS merchant_activation_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID REFERENCES merchants(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Comprehensive audit log (partitioned by month)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID DEFAULT gen_random_uuid(),
    -- Actor
    actor_type VARCHAR(50), -- 'admin', 'service', 'system'
    actor_id VARCHAR(255),  -- service_id or admin_id
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
CREATE TABLE IF NOT EXISTS audit_logs_2025_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_02 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- Add primary key to each partition
ALTER TABLE audit_logs_2025_01 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_02 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_03 ADD PRIMARY KEY (id);

-- Rate limit tracking (per service)
CREATE TABLE IF NOT EXISTS rate_limit_buckets (
    bucket_key VARCHAR(255) PRIMARY KEY,  -- Format: "service_id:merchant_id" or "service_id"
    tokens INTEGER NOT NULL,
    last_refill TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- EPX IP whitelist (for callback security)
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
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs (actor_type, actor_id, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs (action, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs (entity_type, entity_id, performed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_ip ON audit_logs (ip_address, performed_at DESC) WHERE ip_address IS NOT NULL;

-- +goose Down
-- Drop all auth-related tables
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS jwt_blacklist;
DROP TABLE IF EXISTS epx_ip_whitelist;
DROP TABLE IF EXISTS rate_limit_buckets;
DROP TABLE IF EXISTS audit_logs_2025_03;
DROP TABLE IF EXISTS audit_logs_2025_02;
DROP TABLE IF EXISTS audit_logs_2025_01;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS merchant_activation_tokens;
DROP TABLE IF EXISTS service_merchants;
DROP TABLE IF EXISTS services;
ALTER TABLE merchants DROP COLUMN IF EXISTS status;
ALTER TABLE merchants DROP COLUMN IF EXISTS tier;
ALTER TABLE merchants DROP COLUMN IF EXISTS created_by;
ALTER TABLE merchants DROP COLUMN IF EXISTS approved_by;
ALTER TABLE merchants DROP COLUMN IF EXISTS approved_at;
DROP TABLE IF EXISTS admins;
