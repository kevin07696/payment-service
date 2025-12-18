-- +goose Up
-- Migration: 021_add_audit_logs_partitions_2025_remaining.sql
-- Description: Add missing partitions for audit_logs table for remainder of 2025
-- The original migration only created partitions for Jan-Mar 2025, causing failures
-- when trying to log audit events after March 2025.
-- Author: System
-- Date: 2025-11-22

-- Create monthly partitions for April-December 2025
CREATE TABLE IF NOT EXISTS audit_logs_2025_04 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-04-01') TO ('2025-05-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_05 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-05-01') TO ('2025-06-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_06 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_07 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_08 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-08-01') TO ('2025-09-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_09 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-09-01') TO ('2025-10-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_10 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-10-01') TO ('2025-11-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_11 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');
CREATE TABLE IF NOT EXISTS audit_logs_2025_12 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');

-- Add primary keys to each partition
ALTER TABLE audit_logs_2025_04 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_05 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_06 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_07 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_08 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_09 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_10 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_11 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2025_12 ADD PRIMARY KEY (id);

-- Add partitions for 2026 to prevent future issues
CREATE TABLE IF NOT EXISTS audit_logs_2026_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE IF NOT EXISTS audit_logs_2026_02 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE IF NOT EXISTS audit_logs_2026_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

ALTER TABLE audit_logs_2026_01 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2026_02 ADD PRIMARY KEY (id);
ALTER TABLE audit_logs_2026_03 ADD PRIMARY KEY (id);

-- +goose Down
-- Drop 2026 partitions
DROP TABLE IF EXISTS audit_logs_2026_03;
DROP TABLE IF EXISTS audit_logs_2026_02;
DROP TABLE IF EXISTS audit_logs_2026_01;

-- Drop 2025 partitions
DROP TABLE IF EXISTS audit_logs_2025_12;
DROP TABLE IF EXISTS audit_logs_2025_11;
DROP TABLE IF EXISTS audit_logs_2025_10;
DROP TABLE IF EXISTS audit_logs_2025_09;
DROP TABLE IF EXISTS audit_logs_2025_08;
DROP TABLE IF EXISTS audit_logs_2025_07;
DROP TABLE IF EXISTS audit_logs_2025_06;
DROP TABLE IF EXISTS audit_logs_2025_05;
DROP TABLE IF EXISTS audit_logs_2025_04;
