package platformservices

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	"github.com/movebigrocks/platform/pkg/extensionhost/shared/contracts"
)

// auditTenantScope establishes the database-local workspace setting required
// by PostgreSQL RLS before an append is attempted.
type auditTenantScope interface {
	contracts.TransactionRunner
	SetTenantContext(ctx context.Context, workspaceID string) error
}

// AuditService is the application boundary for immutable audit and security
// history. It intentionally exposes append operations only.
type AuditService struct {
	store       shared.AuditStore
	tenantScope auditTenantScope
	now         func() time.Time
}

func NewAuditService(store shared.AuditStore, tenantScope auditTenantScope) *AuditService {
	return &AuditService{
		store:       store,
		tenantScope: tenantScope,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (s *AuditService) LogActivity(ctx context.Context, req LogActivityRequest) error {
	if s == nil || s.store == nil || s.tenantScope == nil {
		return fmt.Errorf("audit service is not configured")
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace ID is required")
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		return fmt.Errorf("audit action is required")
	}
	outcome := strings.ToLower(strings.TrimSpace(req.Outcome))
	if outcome != "success" && outcome != "failure" {
		return fmt.Errorf("audit outcome must be success or failure")
	}

	auditLog := &platformdomain.AuditLog{
		WorkspaceID:  workspaceID,
		UserID:       strings.TrimSpace(req.ActorID),
		UserEmail:    strings.TrimSpace(req.ActorEmail),
		UserName:     strings.TrimSpace(req.ActorName),
		Action:       platformdomain.AuditAction(action),
		Resource:     strings.TrimSpace(req.ResourceType),
		ResourceID:   strings.TrimSpace(req.ResourceID),
		ResourceName: strings.TrimSpace(req.ResourceName),
		IPAddress:    strings.TrimSpace(req.IPAddress),
		UserAgent:    strings.TrimSpace(req.UserAgent),
		SessionID:    strings.TrimSpace(req.SessionID),
		RequestID:    strings.TrimSpace(req.RequestID),
		APIKeyID:     strings.TrimSpace(req.APIKeyID),
		Success:      outcome == "success",
		ErrorMessage: strings.TrimSpace(req.ErrorMessage),
		Metadata:     req.Details.Clone(),
		Tags:         append([]string(nil), req.Tags...),
		CreatedAt:    s.now(),
	}
	return s.appendInWorkspace(ctx, workspaceID, func(writeCtx context.Context) error {
		return s.store.CreateAuditLog(writeCtx, auditLog)
	})
}

func (s *AuditService) LogSecurityEvent(ctx context.Context, req LogSecurityEventRequest) error {
	if s == nil || s.store == nil || s.tenantScope == nil {
		return fmt.Errorf("audit service is not configured")
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return fmt.Errorf("workspace ID is required")
	}
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		return fmt.Errorf("security event type is required")
	}
	severity := platformdomain.SecuritySeverity(strings.ToLower(strings.TrimSpace(req.Severity)))
	if !validSecuritySeverity(severity) {
		return fmt.Errorf("invalid security event severity %q", req.Severity)
	}
	if strings.TrimSpace(req.Description) == "" {
		return fmt.Errorf("security event description is required")
	}
	if req.RiskScore < 0 || req.RiskScore > 100 {
		return fmt.Errorf("security event risk score must be between 0 and 100")
	}
	occurredAt := req.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = s.now()
	}

	event := &platformdomain.SecurityEvent{
		WorkspaceID:     workspaceID,
		Type:            platformdomain.SecurityEventType(eventType),
		Severity:        severity,
		Description:     strings.TrimSpace(req.Description),
		UserID:          strings.TrimSpace(req.ActorID),
		IPAddress:       strings.TrimSpace(req.IPAddress),
		UserAgent:       strings.TrimSpace(req.UserAgent),
		Resource:        strings.TrimSpace(req.ResourceType),
		ResourceID:      strings.TrimSpace(req.ResourceID),
		DetectionMethod: strings.TrimSpace(req.DetectionMethod),
		RiskScore:       req.RiskScore,
		Indicators:      append([]string(nil), req.Indicators...),
		AutoBlocked:     req.AutoBlocked,
		RequiresReview:  req.RequiresReview,
		Metadata:        req.Details.Clone(),
		OccurredAt:      occurredAt.UTC(),
		CreatedAt:       s.now(),
	}
	return s.appendInWorkspace(ctx, workspaceID, func(writeCtx context.Context) error {
		return s.store.CreateSecurityEvent(writeCtx, event)
	})
}

func (s *AuditService) appendInWorkspace(ctx context.Context, workspaceID string, appendRecord func(context.Context) error) error {
	return s.tenantScope.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.tenantScope.SetTenantContext(txCtx, workspaceID); err != nil {
			return fmt.Errorf("set audit tenant context: %w", err)
		}
		if err := appendRecord(txCtx); err != nil {
			return fmt.Errorf("append audit record: %w", err)
		}
		return nil
	})
}

func validSecuritySeverity(severity platformdomain.SecuritySeverity) bool {
	switch severity {
	case platformdomain.SecuritySeverityInfo,
		platformdomain.SecuritySeverityLow,
		platformdomain.SecuritySeverityMedium,
		platformdomain.SecuritySeverityHigh,
		platformdomain.SecuritySeverityCritical:
		return true
	default:
		return false
	}
}
