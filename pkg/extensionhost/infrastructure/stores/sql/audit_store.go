package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
)

// AuditStore persists append-only audit logs and security events.
type AuditStore struct {
	db *SqlxDB
}

func NewAuditStore(db *SqlxDB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) CreateAuditLog(ctx context.Context, auditLog *platformdomain.AuditLog) error {
	if auditLog == nil {
		return fmt.Errorf("audit log is nil")
	}

	oldValue, err := marshalAuditJSON(auditLog.OldValue, nil)
	if err != nil {
		return fmt.Errorf("marshal old value: %w", err)
	}
	newValue, err := marshalAuditJSON(auditLog.NewValue, nil)
	if err != nil {
		return fmt.Errorf("marshal new value: %w", err)
	}
	changes, err := marshalAuditJSON(auditLog.Changes, []platformdomain.FieldChange{})
	if err != nil {
		return fmt.Errorf("marshal changes: %w", err)
	}
	metadata, err := marshalAuditJSON(auditLog.Metadata, map[string]any{})
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	tags, err := marshalAuditJSON(auditLog.Tags, []string{})
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	normalizePersistedUUID(&auditLog.ID)
	if auditLog.CreatedAt.IsZero() {
		auditLog.CreatedAt = time.Now().UTC()
	}
	query := `INSERT INTO core_governance.audit_logs (
		id, workspace_id, user_id, user_email, user_name, action, resource,
		resource_id, resource_name, old_value, new_value, changes, ip_address,
		user_agent, session_id, request_id, api_key_id, success, error_message,
		metadata, tags, created_at
	) VALUES (
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?,
		?::jsonb, ?::jsonb, ?::jsonb, ?, ?, ?, ?, ?, ?, ?, ?::jsonb,
		?::jsonb, ?
	) RETURNING id, created_at`

	err = s.db.Get(ctx).QueryRowxContext(
		ctx,
		query,
		auditLog.ID,
		auditLog.WorkspaceID,
		nullableLegacyUUIDValue(auditLog.UserID),
		auditLog.UserEmail,
		auditLog.UserName,
		string(auditLog.Action),
		auditLog.Resource,
		auditLog.ResourceID,
		auditLog.ResourceName,
		oldValue,
		newValue,
		changes,
		auditLog.IPAddress,
		auditLog.UserAgent,
		nullableLegacyUUIDValue(auditLog.SessionID),
		auditLog.RequestID,
		auditLog.APIKeyID,
		auditLog.Success,
		auditLog.ErrorMessage,
		metadata,
		tags,
		auditLog.CreatedAt,
	).Scan(&auditLog.ID, &auditLog.CreatedAt)
	return TranslateSqlxError(err, "audit_logs")
}

func (s *AuditStore) CreateSecurityEvent(ctx context.Context, event *platformdomain.SecurityEvent) error {
	if event == nil {
		return fmt.Errorf("security event is nil")
	}

	indicators, err := marshalAuditJSON(event.Indicators, []string{})
	if err != nil {
		return fmt.Errorf("marshal indicators: %w", err)
	}
	metadata, err := marshalAuditJSON(event.Metadata, map[string]any{})
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	normalizePersistedUUID(&event.ID)
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	query := `INSERT INTO core_governance.security_events (
		id, workspace_id, type, severity, description, user_id, ip_address,
		user_agent, location, resource, resource_id, detection_method, risk_score,
		indicators, auto_blocked, requires_review, reviewed_by, reviewed_at,
		action_taken, metadata, occurred_at, created_at
	) VALUES (
		COALESCE(NULLIF(?, '')::uuid, uuidv7()), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		?::jsonb, ?, ?, ?, ?, ?, ?::jsonb, ?, ?
	) RETURNING id, occurred_at, created_at`

	err = s.db.Get(ctx).QueryRowxContext(
		ctx,
		query,
		event.ID,
		event.WorkspaceID,
		string(event.Type),
		string(event.Severity),
		event.Description,
		nullableLegacyUUIDValue(event.UserID),
		event.IPAddress,
		event.UserAgent,
		event.Location,
		event.Resource,
		event.ResourceID,
		event.DetectionMethod,
		event.RiskScore,
		indicators,
		event.AutoBlocked,
		event.RequiresReview,
		nullableLegacyUUIDValue(event.ReviewedBy),
		event.ReviewedAt,
		event.ActionTaken,
		metadata,
		event.OccurredAt,
		event.CreatedAt,
	).Scan(&event.ID, &event.OccurredAt, &event.CreatedAt)
	return TranslateSqlxError(err, "security_events")
}

func marshalAuditJSON(value any, fallback any) ([]byte, error) {
	if raw, ok := value.(json.RawMessage); ok {
		if len(raw) == 0 {
			return json.Marshal(fallback)
		}
		if !json.Valid(raw) {
			return nil, fmt.Errorf("invalid JSON")
		}
		return raw, nil
	}
	if value == nil {
		value = fallback
	}
	return json.Marshal(value)
}
