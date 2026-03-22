-- =====================================================
-- Move Big Rocks Schema: Infrastructure
-- Bounded Context: Core infrastructure, extensions, event infrastructure
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_infra;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- =====================================================
-- Outbox Events (for reliable event publishing)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_infra.outbox_events (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    stream VARCHAR(100) NOT NULL,
    aggregate_type VARCHAR(100),
    aggregate_id TEXT,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB NOT NULL,
    correlation_id TEXT,
    status VARCHAR(50) DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    last_error TEXT,
    next_retry TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_status ON core_infra.outbox_events(status);
CREATE INDEX IF NOT EXISTS idx_outbox_status_stream ON core_infra.outbox_events(status, stream);
CREATE INDEX IF NOT EXISTS idx_outbox_published ON core_infra.outbox_events(published_at);

-- =====================================================
-- Dead Letter Queue
-- =====================================================
CREATE TABLE IF NOT EXISTS core_infra.event_dlq (
    id BIGSERIAL PRIMARY KEY,
    stream VARCHAR(100) NOT NULL,
    event_type VARCHAR(100),
    event_data JSONB NOT NULL,
    failure_type VARCHAR(50) NOT NULL,
    error_msg TEXT,
    worker VARCHAR(100) NOT NULL,
    consumer_id VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retry_count INTEGER DEFAULT 0,
    processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_dlq_stream ON core_infra.event_dlq(stream);
CREATE INDEX IF NOT EXISTS idx_dlq_event_type ON core_infra.event_dlq(event_type);
CREATE INDEX IF NOT EXISTS idx_dlq_failure_type ON core_infra.event_dlq(failure_type);
CREATE INDEX IF NOT EXISTS idx_dlq_created_unprocessed ON core_infra.event_dlq(created_at) WHERE processed_at IS NULL;

-- =====================================================
-- Processed Events (for handler idempotency)
-- Uses composite key so each handler group independently tracks processed events
-- =====================================================
CREATE TABLE IF NOT EXISTS core_infra.processed_events (
    event_id VARCHAR(36) NOT NULL,
    handler_group VARCHAR(100) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, handler_group)
);

CREATE INDEX IF NOT EXISTS idx_processed_events_time ON core_infra.processed_events(processed_at);

-- =====================================================
-- Rate Limit Entries (for distributed rate limiting)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_infra.rate_limit_entries (
    key VARCHAR(255) PRIMARY KEY,
    count INTEGER NOT NULL DEFAULT 0,
    first_at TIMESTAMPTZ NOT NULL,
    last_at TIMESTAMPTZ NOT NULL,
    blocked BOOLEAN DEFAULT false,
    blocked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rate_limit_expires ON core_infra.rate_limit_entries(expires_at);

-- =====================================================
-- Files (generic file storage metadata)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_infra.files (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    filename VARCHAR(255),
    content_type VARCHAR(100),
    size BIGINT,
    storage_key VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
