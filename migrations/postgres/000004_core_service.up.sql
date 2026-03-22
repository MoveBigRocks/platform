-- =====================================================
-- Move Big Rocks Schema: Service
-- Bounded Context: Catalog, conversations, cases, communications, attachments
-- =====================================================

CREATE SCHEMA IF NOT EXISTS core_service;

-- =====================================================
-- Case Queues
-- Routing queues for operational work
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.case_queues (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_case_queues_workspace ON core_service.case_queues(workspace_id);
CREATE INDEX IF NOT EXISTS idx_case_queues_deleted ON core_service.case_queues(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_case_queues_ws_slug_unique
    ON core_service.case_queues(workspace_id, slug) WHERE deleted_at IS NULL;

-- =====================================================
-- Service Catalog Nodes
-- Semantic classifier for conversations, forms, and cases
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.service_catalog_nodes (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    parent_node_id UUID REFERENCES core_service.service_catalog_nodes(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    path_slug TEXT NOT NULL,
    title TEXT NOT NULL,
    description_markdown TEXT NOT NULL DEFAULT '',
    node_kind TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    visibility TEXT NOT NULL DEFAULT 'workspace',
    supported_channels TEXT[] NOT NULL DEFAULT '{}'::text[],
    default_case_category TEXT,
    default_queue_id UUID,
    default_priority TEXT,
    routing_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    entitlement_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    search_keywords TEXT[] NOT NULL DEFAULT '{}'::text[],
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(description_markdown, '')), 'B')
    ) STORED
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_service_catalog_nodes_workspace_path
    ON core_service.service_catalog_nodes(workspace_id, path_slug);
CREATE UNIQUE INDEX IF NOT EXISTS uq_service_catalog_nodes_workspace_parent_slug
    ON core_service.service_catalog_nodes(workspace_id, parent_node_id, slug);
CREATE INDEX IF NOT EXISTS idx_service_catalog_nodes_tree
    ON core_service.service_catalog_nodes(workspace_id, parent_node_id, display_order, id);
CREATE INDEX IF NOT EXISTS idx_service_catalog_nodes_visibility
    ON core_service.service_catalog_nodes(workspace_id, visibility, status, display_order, id);
CREATE INDEX IF NOT EXISTS idx_service_catalog_nodes_channels
    ON core_service.service_catalog_nodes USING GIN (supported_channels);
CREATE INDEX IF NOT EXISTS idx_service_catalog_nodes_search
    ON core_service.service_catalog_nodes USING GIN (search_vector);

-- =====================================================
-- Service Catalog Bindings
-- Links catalog nodes to knowledge, forms, routing, policy, and automation
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.service_catalog_bindings (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    catalog_node_id UUID NOT NULL REFERENCES core_service.service_catalog_nodes(id) ON DELETE CASCADE,
    target_kind TEXT NOT NULL,
    target_id UUID NOT NULL,
    binding_kind TEXT NOT NULL,
    confidence NUMERIC,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_service_catalog_bindings_target
    ON core_service.service_catalog_bindings(workspace_id, target_kind, target_id, binding_kind);
CREATE INDEX IF NOT EXISTS idx_service_catalog_bindings_catalog
    ON core_service.service_catalog_bindings(workspace_id, catalog_node_id, binding_kind, target_kind);

-- =====================================================
-- Conversation Sessions
-- Live supervised interactions that may escalate into work
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.conversation_sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    status TEXT NOT NULL,
    primary_contact_id UUID,
    primary_catalog_node_id UUID REFERENCES core_service.service_catalog_nodes(id) ON DELETE SET NULL,
    active_form_spec_id UUID,
    active_form_submission_id UUID,
    linked_case_id UUID,
    handling_team_id UUID REFERENCES core_platform.teams(id) ON DELETE SET NULL,
    assigned_operator_user_id UUID,
    delegated_runtime_connector_id UUID,
    title TEXT,
    language_code TEXT,
    source_ref TEXT,
    external_session_key TEXT,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversation_sessions_workspace_status_activity
    ON core_service.conversation_sessions(workspace_id, status, last_activity_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_workspace_catalog_status_activity
    ON core_service.conversation_sessions(workspace_id, primary_catalog_node_id, status, last_activity_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_workspace_contact_activity
    ON core_service.conversation_sessions(workspace_id, primary_contact_id, last_activity_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_sessions_linked_case
    ON core_service.conversation_sessions(linked_case_id)
    WHERE linked_case_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_conversation_sessions_external_session
    ON core_service.conversation_sessions(workspace_id, channel, external_session_key)
    WHERE external_session_key IS NOT NULL AND closed_at IS NULL;

-- =====================================================
-- Conversation Participants
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.conversation_participants (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    conversation_session_id UUID NOT NULL REFERENCES core_service.conversation_sessions(id) ON DELETE CASCADE,
    participant_kind TEXT NOT NULL,
    participant_ref TEXT NOT NULL,
    role_in_session TEXT NOT NULL,
    display_name TEXT,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversation_participants_session
    ON core_service.conversation_participants(conversation_session_id, joined_at, id);
CREATE INDEX IF NOT EXISTS idx_conversation_participants_lookup
    ON core_service.conversation_participants(workspace_id, participant_kind, participant_ref);

-- =====================================================
-- Conversation Messages
-- Append-only transcript events
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.conversation_messages (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    conversation_session_id UUID NOT NULL REFERENCES core_service.conversation_sessions(id) ON DELETE CASCADE,
    participant_id UUID REFERENCES core_service.conversation_participants(id) ON DELETE SET NULL,
    role TEXT NOT NULL,
    kind TEXT NOT NULL,
    visibility TEXT NOT NULL,
    content_text TEXT,
    content_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    search_vector TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('simple', coalesce(content_text, ''))
    ) STORED
);

CREATE INDEX IF NOT EXISTS idx_conversation_messages_session_created
    ON core_service.conversation_messages(conversation_session_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_session_visibility_created
    ON core_service.conversation_messages(conversation_session_id, visibility, created_at, id);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_workspace_created
    ON core_service.conversation_messages(workspace_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_conversation_messages_search
    ON core_service.conversation_messages USING GIN (search_vector);

-- =====================================================
-- Conversation Working State
-- Mutable classification, forms, and review state
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.conversation_working_state (
    conversation_session_id UUID PRIMARY KEY REFERENCES core_service.conversation_sessions(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    primary_catalog_node_id UUID REFERENCES core_service.service_catalog_nodes(id) ON DELETE SET NULL,
    suggested_catalog_nodes_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    classification_confidence NUMERIC,
    active_policy_profile_ref TEXT,
    active_form_spec_id UUID,
    active_form_submission_id UUID,
    collected_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    missing_fields_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    requires_operator_review BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversation_working_state_review
    ON core_service.conversation_working_state(workspace_id, requires_operator_review, updated_at DESC);

-- =====================================================
-- Conversation Outcomes
-- Material transitions produced by a session
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.conversation_outcomes (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    conversation_session_id UUID NOT NULL REFERENCES core_service.conversation_sessions(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    result_ref_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversation_outcomes_session
    ON core_service.conversation_outcomes(conversation_session_id, created_at, id);

-- =====================================================
-- Cases
-- Escalated operational work items
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.cases (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    human_id VARCHAR(36) NOT NULL,  -- Format: prefix-yymm-random (e.g., tp-2512-a3e9ef)
    subject VARCHAR(500),
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'open',
    priority VARCHAR(20) NOT NULL DEFAULT 'medium',
    channel VARCHAR(50),
    category VARCHAR(100),
    queue_id UUID,
    contact_id UUID,
    primary_catalog_node_id UUID REFERENCES core_service.service_catalog_nodes(id) ON DELETE SET NULL,
    originating_conversation_session_id UUID REFERENCES core_service.conversation_sessions(id) ON DELETE SET NULL,
    contact_email VARCHAR(255),
    contact_name VARCHAR(255),
    assigned_to_id UUID,
    team_id UUID,
    source VARCHAR(50),
    source_id VARCHAR(255),
    source_link TEXT,
    tags JSONB DEFAULT '[]',
    resolution VARCHAR(50),
    resolution_note TEXT,
    response_due_at TIMESTAMPTZ,
    resolution_due_at TIMESTAMPTZ,
    first_response_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    response_time_minutes INTEGER,
    resolution_time_minutes INTEGER,
    reopen_count INTEGER DEFAULT 0,
    message_count INTEGER DEFAULT 0,
    linked_issue_ids JSONB DEFAULT '[]',
    root_cause_issue_id UUID,
    issue_resolved BOOLEAN DEFAULT false,
    issue_resolved_at TIMESTAMPTZ,
    contact_notified BOOLEAN DEFAULT false,
    contact_notified_at TIMESTAMPTZ,
    notification_template VARCHAR(100),
    auto_created BOOLEAN DEFAULT false,
    is_system_case BOOLEAN DEFAULT false,
    custom_fields JSONB DEFAULT '{}',
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_cases_workspace ON core_service.cases(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_cases_human_id ON core_service.cases(human_id);
CREATE INDEX IF NOT EXISTS idx_cases_status ON core_service.cases(status);
CREATE INDEX IF NOT EXISTS idx_cases_priority ON core_service.cases(priority);
CREATE INDEX IF NOT EXISTS idx_cases_queue ON core_service.cases(queue_id);
CREATE INDEX IF NOT EXISTS idx_cases_contact ON core_service.cases(contact_id);
CREATE INDEX IF NOT EXISTS idx_cases_catalog_node ON core_service.cases(primary_catalog_node_id);
CREATE INDEX IF NOT EXISTS idx_cases_originating_conversation ON core_service.cases(originating_conversation_session_id);
CREATE INDEX IF NOT EXISTS idx_cases_assigned ON core_service.cases(assigned_to_id);
CREATE INDEX IF NOT EXISTS idx_cases_team ON core_service.cases(team_id);
CREATE INDEX IF NOT EXISTS idx_cases_deleted ON core_service.cases(deleted_at);

-- =====================================================
-- Queue Items
-- Concrete queue contents pointing to either cases or conversations
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.queue_items (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    queue_id UUID NOT NULL REFERENCES core_service.case_queues(id) ON DELETE CASCADE,
    item_kind TEXT NOT NULL,
    case_id UUID REFERENCES core_service.cases(id) ON DELETE CASCADE,
    conversation_session_id UUID REFERENCES core_service.conversation_sessions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT ck_queue_items_target CHECK (
        (item_kind = 'case' AND case_id IS NOT NULL AND conversation_session_id IS NULL) OR
        (item_kind = 'conversation_session' AND case_id IS NULL AND conversation_session_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_queue_items_workspace_queue_activity
    ON core_service.queue_items(workspace_id, queue_id, updated_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_queue_items_case
    ON core_service.queue_items(case_id)
    WHERE case_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_queue_items_conversation_session
    ON core_service.queue_items(conversation_session_id)
    WHERE conversation_session_id IS NOT NULL AND deleted_at IS NULL;

-- =====================================================
-- Communications (messages within cases)
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.communications (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    case_id UUID NOT NULL REFERENCES core_service.cases(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    type VARCHAR(50),  -- email, note, chat
    direction VARCHAR(20),  -- inbound, outbound
    subject VARCHAR(500),
    body TEXT,
    body_html TEXT,
    from_email VARCHAR(255),
    from_name VARCHAR(255),
    from_user_id UUID,
    to_emails JSONB DEFAULT '[]',
    cc_emails JSONB DEFAULT '[]',
    bcc_emails JSONB DEFAULT '[]',
    message_id VARCHAR(255),
    in_reply_to VARCHAR(255),
    email_references JSONB DEFAULT '[]',
    attachment_ids JSONB DEFAULT '[]',
    is_internal BOOLEAN DEFAULT false,
    is_read BOOLEAN DEFAULT false,
    is_spam BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_communications_case ON core_service.communications(case_id);
CREATE INDEX IF NOT EXISTS idx_communications_workspace ON core_service.communications(workspace_id);

-- =====================================================
-- Case Assignment History
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.case_assignment_history (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    case_id UUID NOT NULL REFERENCES core_service.cases(id) ON DELETE CASCADE,
    assignment_type VARCHAR(50),
    assigned_to_user_id UUID,
    assigned_to_team_id UUID,
    assigned_user_name VARCHAR(255),
    assigned_team_name VARCHAR(255),
    previous_user_id UUID,
    previous_team_id UUID,
    previous_user_name VARCHAR(255),
    previous_team_name VARCHAR(255),
    reason VARCHAR(100),
    status VARCHAR(50),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    duration INTEGER,
    assigned_by_id UUID,
    assigned_by_name VARCHAR(255),
    assigned_by_type VARCHAR(50),
    rule_id UUID,
    workflow_id UUID,
    priority INTEGER DEFAULT 0,
    is_urgent BOOLEAN DEFAULT false,
    sla_deadline TIMESTAMPTZ,
    workload_before INTEGER,
    workload_after INTEGER,
    required_skills JSONB DEFAULT '[]',
    matched_skills JSONB DEFAULT '[]',
    skill_match_score DOUBLE PRECISION DEFAULT 0,
    assignee_available BOOLEAN DEFAULT true,
    assignee_timezone VARCHAR(50),
    assignment_during_hours BOOLEAN DEFAULT true,
    response_time INTEGER,
    resolution_time INTEGER,
    customer_satisfaction DOUBLE PRECISION,
    was_escalated BOOLEAN DEFAULT false,
    escalated_at TIMESTAMPTZ,
    escalated_to_user_id UUID,
    escalated_to_team_id UUID,
    escalation_reason TEXT,
    was_transferred BOOLEAN DEFAULT false,
    transferred_at TIMESTAMPTZ,
    transferred_to_user_id UUID,
    transferred_to_team_id UUID,
    transfer_reason TEXT,
    was_accepted BOOLEAN DEFAULT false,
    accepted_by_id UUID,
    rejected_at TIMESTAMPTZ,
    rejection_reason TEXT,
    auto_assignment_config JSONB DEFAULT '{}',
    assignment_score DOUBLE PRECISION DEFAULT 0,
    alternative_candidates JSONB DEFAULT '[]',
    notification_sent BOOLEAN DEFAULT false,
    notification_sent_at TIMESTAMPTZ,
    notification_method VARCHAR(50),
    notification_viewed BOOLEAN DEFAULT false,
    notification_viewed_at TIMESTAMPTZ,
    case_status VARCHAR(50),
    case_priority VARCHAR(20),
    case_subject VARCHAR(500),
    case_created_at TIMESTAMPTZ,
    case_custom_fields JSONB DEFAULT '{}',
    comments TEXT,
    tags JSONB DEFAULT '[]',
    custom_fields JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_case_assign_hist_workspace ON core_service.case_assignment_history(workspace_id);
CREATE INDEX IF NOT EXISTS idx_case_assign_hist_case ON core_service.case_assignment_history(case_id);
CREATE INDEX IF NOT EXISTS idx_case_assign_hist_user ON core_service.case_assignment_history(assigned_to_user_id);
CREATE INDEX IF NOT EXISTS idx_case_assign_hist_team ON core_service.case_assignment_history(assigned_to_team_id);

-- =====================================================
-- Attachments
-- =====================================================
CREATE TABLE IF NOT EXISTS core_service.attachments (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    workspace_id UUID NOT NULL REFERENCES core_platform.workspaces(id) ON DELETE CASCADE,
    filename VARCHAR(255),
    original_name VARCHAR(255),
    content_type VARCHAR(100),
    size BIGINT,
    checksum VARCHAR(64),
    storage_key VARCHAR(500),
    storage_type VARCHAR(50),
    storage_bucket VARCHAR(255),
    case_id UUID,
    communication_id UUID,
    email_id UUID,
    conversation_session_id UUID,
    conversation_message_id UUID,
    form_submission_id UUID,
    is_scanned BOOLEAN DEFAULT false,
    scan_result VARCHAR(50),
    scanned_at TIMESTAMPTZ,
    scan_details JSONB DEFAULT '{}',
    is_public BOOLEAN DEFAULT false,
    allow_download BOOLEAN DEFAULT true,
    access_token VARCHAR(100),
    description TEXT,
    tags JSONB DEFAULT '[]',
    metadata JSONB DEFAULT '{}',
    uploaded_by_id UUID,
    uploaded_by_type VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_attachments_workspace ON core_service.attachments(workspace_id);
CREATE INDEX IF NOT EXISTS idx_attachments_case ON core_service.attachments(case_id);
CREATE INDEX IF NOT EXISTS idx_attachments_conversation_session ON core_service.attachments(conversation_session_id);
CREATE INDEX IF NOT EXISTS idx_attachments_form_submission ON core_service.attachments(form_submission_id);
CREATE INDEX IF NOT EXISTS idx_attachments_storage ON core_service.attachments(storage_key);
CREATE INDEX IF NOT EXISTS idx_attachments_deleted ON core_service.attachments(deleted_at);
