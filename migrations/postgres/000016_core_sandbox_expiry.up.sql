-- +goose Up
-- Purpose: track sandbox expiry transitions and support expiry reaping queries
-- Bounded Context: vendor-operated sandbox lifecycle

ALTER TABLE core_platform.sandboxes
    ADD COLUMN IF NOT EXISTS expired_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sandboxes_expires_at ON core_platform.sandboxes(expires_at);
