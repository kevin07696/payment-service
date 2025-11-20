-- +goose Up
-- Migration: 010_merchant_credentials.sql
-- Description: Add merchant_credentials table for API key authentication
-- This table was missing from 008_auth_tables.sql but is required by the auth middleware
-- Author: Authentication System
-- Date: 2025-11-20

-- Merchant API credentials table for API key/secret authentication
CREATE TABLE IF NOT EXISTS merchant_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,

    -- API credentials (stored as SHA-256 hashes with salt)
    api_key_hash VARCHAR(255) NOT NULL,
    api_secret_hash VARCHAR(255) NOT NULL,

    -- Metadata
    description TEXT, -- e.g., "Production API key", "Staging environment key"

    -- Status and lifecycle
    is_active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP, -- NULL = never expires
    last_used_at TIMESTAMP, -- Updated asynchronously by middleware

    -- Audit trail
    created_by UUID REFERENCES admins(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    -- Ensure unique credential pairs
    UNIQUE(api_key_hash, api_secret_hash)
);

-- Indexes for merchant_credentials lookups
CREATE INDEX IF NOT EXISTS idx_merchant_credentials_merchant
    ON merchant_credentials(merchant_id)
    WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_merchant_credentials_lookup
    ON merchant_credentials(api_key_hash, api_secret_hash)
    WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_merchant_credentials_expires
    ON merchant_credentials(expires_at)
    WHERE expires_at IS NOT NULL AND is_active = true;

-- Function to auto-update updated_at timestamp
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_merchant_credentials_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Trigger to auto-update updated_at on row updates
CREATE TRIGGER trigger_merchant_credentials_updated_at
    BEFORE UPDATE ON merchant_credentials
    FOR EACH ROW
    EXECUTE FUNCTION update_merchant_credentials_updated_at();

-- +goose Down
-- Drop merchant_credentials table and associated objects
DROP TRIGGER IF EXISTS trigger_merchant_credentials_updated_at ON merchant_credentials;
DROP FUNCTION IF EXISTS update_merchant_credentials_updated_at();
DROP TABLE IF EXISTS merchant_credentials;
