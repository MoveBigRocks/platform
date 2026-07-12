-- Add lease state so concurrent blue/green workers cannot execute the same
-- handler group for one event at the same time.
ALTER TABLE core_infra.outbox_events
    ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_outbox_publishing_claim
    ON core_infra.outbox_events(claimed_at)
    WHERE status = 'publishing';

ALTER TABLE core_infra.processed_events
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'processed',
    ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS claim_expires_at TIMESTAMPTZ;

ALTER TABLE core_infra.processed_events
    ALTER COLUMN processed_at DROP NOT NULL;

UPDATE core_infra.processed_events
SET status = 'processed'
WHERE status IS NULL OR status = '';

CREATE INDEX IF NOT EXISTS idx_processed_events_claim_expiry
    ON core_infra.processed_events(claim_expires_at)
    WHERE status = 'processing';
