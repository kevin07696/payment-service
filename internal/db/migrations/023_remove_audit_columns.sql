-- +goose Up
-- Migration: 023_remove_audit_columns.sql
-- Description: Remove all audit trail columns (created_by, granted_by, etc.) to simplify CLI
-- Rationale: CLI no longer requires authentication - operates directly without admin accounts
-- Author: System Simplification
-- Date: 2025-11-24

-- Drop audit columns from services table
ALTER TABLE services DROP COLUMN IF EXISTS created_by;

-- Drop audit columns from merchants table
ALTER TABLE merchants DROP COLUMN IF EXISTS created_by;
ALTER TABLE merchants DROP COLUMN IF EXISTS approved_by;
ALTER TABLE merchants DROP COLUMN IF EXISTS approved_at;

-- Drop audit columns from service_merchants table
ALTER TABLE service_merchants DROP COLUMN IF EXISTS granted_by;

-- Drop audit columns from epx_ip_whitelist table
ALTER TABLE epx_ip_whitelist DROP COLUMN IF EXISTS added_by;

-- Drop audit columns from jwt_blacklist table
ALTER TABLE jwt_blacklist DROP COLUMN IF EXISTS blacklisted_by;

-- +goose Down
-- Restore audit columns (all nullable to allow migration back)

ALTER TABLE services ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES admins(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES admins(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES admins(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS approved_at TIMESTAMP;
ALTER TABLE service_merchants ADD COLUMN IF NOT EXISTS granted_by UUID REFERENCES admins(id);
ALTER TABLE epx_ip_whitelist ADD COLUMN IF NOT EXISTS added_by UUID REFERENCES admins(id);
ALTER TABLE jwt_blacklist ADD COLUMN IF NOT EXISTS blacklisted_by UUID REFERENCES admins(id);
