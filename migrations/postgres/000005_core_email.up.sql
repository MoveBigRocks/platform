-- =====================================================
-- Move Big Rocks Schema: Email
-- Bounded Context: Email templates, threads, analytics
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_service;

-- =====================================================
-- Email Templates
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.email_templates (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    key VARCHAR(100),
    name VARCHAR(255),
    subject VARCHAR(500),
    body_html TEXT,
    body_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_templates_workspace ON core_service.email_templates(workspace_id);
CREATE INDEX IF NOT EXISTS idx_email_templates_key ON core_service.email_templates(key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_templates_ws_key_unique
    ON core_service.email_templates(workspace_id, key);

-- =====================================================
-- Outbound Emails
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.outbound_emails (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    from_email VARCHAR(255),
    from_name VARCHAR(255),
    to_emails JSONB DEFAULT '[]',
    cc_emails JSONB DEFAULT '[]',
    bcc_emails JSONB DEFAULT '[]',
    reply_to_email VARCHAR(255),
    subject VARCHAR(500),
    html_content TEXT,
    text_content TEXT,
    template_id UUID,
    template_data JSONB DEFAULT '{}',
    provider VARCHAR(50),
    provider_settings JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'pending',
    scheduled_for TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    provider_message_id VARCHAR(255),
    provider_response TEXT,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    next_retry_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    open_count INTEGER DEFAULT 0,
    click_count INTEGER DEFAULT 0,
    last_click_at TIMESTAMPTZ,
    case_id UUID,
    contact_id UUID,
    communication_id UUID,
    user_id UUID,
    category VARCHAR(100),
    tags JSONB DEFAULT '[]',
    attachment_ids JSONB DEFAULT '[]',
    created_by_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbound_emails_workspace ON core_service.outbound_emails(workspace_id);
CREATE INDEX IF NOT EXISTS idx_outbound_emails_status ON core_service.outbound_emails(status);
CREATE INDEX IF NOT EXISTS idx_outbound_emails_provider_msg ON core_service.outbound_emails(provider_message_id);
CREATE INDEX IF NOT EXISTS idx_outbound_emails_case ON core_service.outbound_emails(case_id);
CREATE INDEX IF NOT EXISTS idx_outbound_emails_deleted ON core_service.outbound_emails(deleted_at);

-- =====================================================
-- Inbound Emails
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.inbound_emails (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    message_id VARCHAR(255) UNIQUE,
    in_reply_to VARCHAR(255),
    email_references JSONB DEFAULT '[]',
    from_email VARCHAR(255),
    from_name VARCHAR(255),
    to_emails JSONB DEFAULT '[]',
    cc_emails JSONB DEFAULT '[]',
    bcc_emails JSONB DEFAULT '[]',
    subject VARCHAR(500),
    html_content TEXT,
    text_content TEXT,
    processing_status VARCHAR(50) DEFAULT 'pending',
    processed_at TIMESTAMPTZ,
    processing_error TEXT,
    spam_score DOUBLE PRECISION DEFAULT 0,
    spam_reasons JSONB DEFAULT '[]',
    is_spam BOOLEAN DEFAULT false,
    case_id UUID,
    contact_id UUID,
    communication_id UUID,
    thread_id UUID,
    is_thread_start BOOLEAN DEFAULT false,
    previous_email_ids JSONB DEFAULT '[]',
    is_loop BOOLEAN DEFAULT false,
    loop_score DOUBLE PRECISION DEFAULT 0,
    is_bounce BOOLEAN DEFAULT false,
    bounce_type VARCHAR(50),
    original_message_id VARCHAR(255),
    is_auto_response BOOLEAN DEFAULT false,
    auto_response_type VARCHAR(50),
    is_read BOOLEAN DEFAULT false,
    tags JSONB DEFAULT '[]',
    attachment_ids JSONB DEFAULT '[]',
    attachment_count INTEGER DEFAULT 0,
    total_attachment_size BIGINT DEFAULT 0,
    raw_content TEXT,
    headers JSONB DEFAULT '{}',
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_inbound_emails_workspace ON core_service.inbound_emails(workspace_id);
CREATE INDEX IF NOT EXISTS idx_inbound_emails_message ON core_service.inbound_emails(message_id);
CREATE INDEX IF NOT EXISTS idx_inbound_emails_from ON core_service.inbound_emails(from_email);
CREATE INDEX IF NOT EXISTS idx_inbound_emails_status ON core_service.inbound_emails(processing_status);
CREATE INDEX IF NOT EXISTS idx_inbound_emails_thread ON core_service.inbound_emails(thread_id);

-- =====================================================
-- Email Threads
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.email_threads (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    thread_key VARCHAR(255),
    subject VARCHAR(500),
    type VARCHAR(50),
    status VARCHAR(50) DEFAULT 'active',
    priority VARCHAR(20) DEFAULT 'normal',
    participants JSONB DEFAULT '[]',
    case_id UUID,
    contact_ids JSONB DEFAULT '[]',
    email_count INTEGER DEFAULT 0,
    unread_count INTEGER DEFAULT 0,
    last_email_id UUID,
    first_email_id UUID,
    message_ids JSONB DEFAULT '[]',
    first_email_at TIMESTAMPTZ,
    last_email_at TIMESTAMPTZ,
    last_activity TIMESTAMPTZ,
    sentiment_score DOUBLE PRECISION DEFAULT 0,
    is_important BOOLEAN DEFAULT false,
    has_attachments BOOLEAN DEFAULT false,
    attachment_count INTEGER DEFAULT 0,
    parent_thread_id UUID,
    child_thread_ids JSONB DEFAULT '[]',
    merged_from_ids JSONB DEFAULT '[]',
    merged_into_id UUID,
    detected_by VARCHAR(100),
    detection_method VARCHAR(50),
    detection_score DOUBLE PRECISION DEFAULT 0,
    tags JSONB DEFAULT '[]',
    labels JSONB DEFAULT '[]',
    is_spam BOOLEAN DEFAULT false,
    spam_score DOUBLE PRECISION DEFAULT 0,
    is_quarantined BOOLEAN DEFAULT false,
    is_watched BOOLEAN DEFAULT false,
    is_muted BOOLEAN DEFAULT false,
    is_archived BOOLEAN DEFAULT false,
    notes TEXT,
    custom_fields JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_email_threads_workspace ON core_service.email_threads(workspace_id);
CREATE INDEX IF NOT EXISTS idx_email_threads_key ON core_service.email_threads(thread_key);
CREATE INDEX IF NOT EXISTS idx_email_threads_status ON core_service.email_threads(status);
CREATE INDEX IF NOT EXISTS idx_email_threads_case ON core_service.email_threads(case_id);
CREATE INDEX IF NOT EXISTS idx_email_threads_deleted ON core_service.email_threads(deleted_at);

-- =====================================================
-- Email Thread Links
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.email_thread_links (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    thread_id UUID NOT NULL,
    email_id UUID NOT NULL,
    email_type VARCHAR(20),
    position INTEGER DEFAULT 0,
    is_first BOOLEAN DEFAULT false,
    is_last BOOLEAN DEFAULT false,
    subject VARCHAR(500),
    from_email VARCHAR(255),
    from_name VARCHAR(255),
    to_emails JSONB DEFAULT '[]',
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_thread_links_workspace ON core_service.email_thread_links(workspace_id);
CREATE INDEX IF NOT EXISTS idx_thread_links_thread ON core_service.email_thread_links(thread_id);
CREATE INDEX IF NOT EXISTS idx_thread_links_email ON core_service.email_thread_links(email_id);

-- =====================================================
-- Thread Merges
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.thread_merges (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    source_thread_id UUID NOT NULL,
    target_thread_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    merged_by_id UUID,
    merged_by_name VARCHAR(255),
    merge_reason TEXT,
    emails_merged INTEGER DEFAULT 0,
    conflicts_found INTEGER DEFAULT 0,
    conflict_details JSONB DEFAULT '{}',
    merged_at TIMESTAMPTZ,
    reverted_at TIMESTAMPTZ,
    reverted_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_thread_merges_workspace ON core_service.thread_merges(workspace_id);
CREATE INDEX IF NOT EXISTS idx_thread_merges_source ON core_service.thread_merges(source_thread_id);
CREATE INDEX IF NOT EXISTS idx_thread_merges_target ON core_service.thread_merges(target_thread_id);

-- =====================================================
-- Thread Splits
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.thread_splits (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    source_thread_id UUID NOT NULL,
    new_thread_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    split_by_id UUID,
    split_by_name VARCHAR(255),
    split_reason TEXT,
    split_at_email_id UUID,
    emails_moved INTEGER DEFAULT 0,
    split_at TIMESTAMPTZ,
    reverted_at TIMESTAMPTZ,
    reverted_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_thread_splits_workspace ON core_service.thread_splits(workspace_id);
CREATE INDEX IF NOT EXISTS idx_thread_splits_source ON core_service.thread_splits(source_thread_id);
CREATE INDEX IF NOT EXISTS idx_thread_splits_new ON core_service.thread_splits(new_thread_id);

-- =====================================================
-- Thread Analytics
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.thread_analytics (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    thread_id UUID NOT NULL,
    date VARCHAR(10) NOT NULL,  -- YYYY-MM-DD
    message_count INTEGER DEFAULT 0,
    inbound_count INTEGER DEFAULT 0,
    outbound_count INTEGER DEFAULT 0,
    internal_count INTEGER DEFAULT 0,
    response_time_avg INTEGER DEFAULT 0,
    response_time_min INTEGER DEFAULT 0,
    response_time_max INTEGER DEFAULT 0,
    open_count INTEGER DEFAULT 0,
    click_count INTEGER DEFAULT 0,
    reply_count INTEGER DEFAULT 0,
    forward_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_thread_analytics_workspace ON core_service.thread_analytics(workspace_id);
CREATE INDEX IF NOT EXISTS idx_thread_analytics_thread ON core_service.thread_analytics(thread_id);
CREATE INDEX IF NOT EXISTS idx_thread_analytics_date ON core_service.thread_analytics(date);

-- =====================================================
-- Email Stats
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.email_stats (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    date VARCHAR(10) NOT NULL,  -- YYYY-MM-DD
    sent_count INTEGER DEFAULT 0,
    received_count INTEGER DEFAULT 0,
    bounced_count INTEGER DEFAULT 0,
    failed_count INTEGER DEFAULT 0,
    pending_count INTEGER DEFAULT 0,
    opened_count INTEGER DEFAULT 0,
    clicked_count INTEGER DEFAULT 0,
    replied_count INTEGER DEFAULT 0,
    threads_created INTEGER DEFAULT 0,
    threads_merged INTEGER DEFAULT 0,
    threads_split INTEGER DEFAULT 0,
    spam_count INTEGER DEFAULT 0,
    quarantine_count INTEGER DEFAULT 0,
    blacklist_hits INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, date)
);

CREATE INDEX IF NOT EXISTS idx_email_stats_workspace ON core_service.email_stats(workspace_id);

-- =====================================================
-- Email Blacklists
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.email_blacklists (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    email VARCHAR(255),
    domain VARCHAR(255),
    pattern TEXT,
    type VARCHAR(50),
    reason TEXT,
    is_active BOOLEAN DEFAULT true,
    block_inbound BOOLEAN DEFAULT true,
    block_outbound BOOLEAN DEFAULT false,
    expires_at TIMESTAMPTZ,
    block_count INTEGER DEFAULT 0,
    last_blocked_at TIMESTAMPTZ,
    created_by_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_blacklists_workspace ON core_service.email_blacklists(workspace_id);
CREATE INDEX IF NOT EXISTS idx_blacklists_email ON core_service.email_blacklists(email);
CREATE INDEX IF NOT EXISTS idx_blacklists_domain ON core_service.email_blacklists(domain);
CREATE INDEX IF NOT EXISTS idx_blacklists_active ON core_service.email_blacklists(is_active);
CREATE INDEX IF NOT EXISTS idx_blacklists_deleted ON core_service.email_blacklists(deleted_at);

-- =====================================================
-- Quarantined Messages
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.quarantined_messages (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    email_id UUID,
    email_type VARCHAR(20),
    message_id VARCHAR(255),
    reason VARCHAR(50),
    reason_detail TEXT,
    risk_score DOUBLE PRECISION DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending',
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID,
    approved_at TIMESTAMPTZ,
    approved_by UUID,
    rejected_at TIMESTAMPTZ,
    rejected_by UUID,
    raw_headers TEXT,
    raw_content TEXT,
    content_size BIGINT DEFAULT 0,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_quarantine_workspace ON core_service.quarantined_messages(workspace_id);
CREATE INDEX IF NOT EXISTS idx_quarantine_email ON core_service.quarantined_messages(email_id);
CREATE INDEX IF NOT EXISTS idx_quarantine_status ON core_service.quarantined_messages(status);
