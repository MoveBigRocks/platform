package platformservices

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

type auditStoreSpy struct {
	auditLog      *platformdomain.AuditLog
	securityEvent *platformdomain.SecurityEvent
	err           error
}

func (s *auditStoreSpy) CreateAuditLog(_ context.Context, auditLog *platformdomain.AuditLog) error {
	s.auditLog = auditLog
	return s.err
}

func (s *auditStoreSpy) CreateSecurityEvent(_ context.Context, event *platformdomain.SecurityEvent) error {
	s.securityEvent = event
	return s.err
}

type auditTenantScopeSpy struct {
	workspaceID string
	setErr      error
	transaction bool
}

func (s *auditTenantScopeSpy) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	s.transaction = true
	return fn(ctx)
}

func (s *auditTenantScopeSpy) SetTenantContext(_ context.Context, workspaceID string) error {
	s.workspaceID = workspaceID
	return s.setErr
}

func TestAuditServiceLogActivityAppendsInTenantTransaction(t *testing.T) {
	store := &auditStoreSpy{}
	scope := &auditTenantScopeSpy{}
	service := NewAuditService(store, scope)
	fixedTime := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedTime }
	details := shareddomain.NewMetadata()
	details.SetString("reason", "verified_magic_link")

	err := service.LogActivity(context.Background(), LogActivityRequest{
		WorkspaceID:  "workspace-1",
		ActorID:      "user-1",
		ActorEmail:   "user@example.com",
		ActorName:    "User One",
		Action:       string(platformdomain.AuditActionLogin),
		ResourceType: "session",
		ResourceID:   "session-1",
		Details:      details,
		Outcome:      "success",
		IPAddress:    "192.0.2.1",
		RequestID:    "request-1",
		Tags:         []string{"authentication"},
	})

	require.NoError(t, err)
	assert.True(t, scope.transaction)
	assert.Equal(t, "workspace-1", scope.workspaceID)
	require.NotNil(t, store.auditLog)
	assert.Equal(t, platformdomain.AuditActionLogin, store.auditLog.Action)
	assert.Equal(t, "user@example.com", store.auditLog.UserEmail)
	assert.Equal(t, "session", store.auditLog.Resource)
	assert.Equal(t, "session-1", store.auditLog.ResourceID)
	assert.True(t, store.auditLog.Success)
	assert.Equal(t, fixedTime, store.auditLog.CreatedAt)
	assert.Equal(t, "verified_magic_link", store.auditLog.Metadata.GetString("reason"))

	// The service owns a clone, so caller mutation cannot rewrite the record.
	details.SetString("reason", "changed")
	assert.Equal(t, "verified_magic_link", store.auditLog.Metadata.GetString("reason"))
}

func TestAuditServiceLogSecurityEventAppendsTypedEvent(t *testing.T) {
	store := &auditStoreSpy{}
	scope := &auditTenantScopeSpy{}
	service := NewAuditService(store, scope)
	fixedTime := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedTime }

	err := service.LogSecurityEvent(context.Background(), LogSecurityEventRequest{
		WorkspaceID:     "workspace-1",
		ActorID:         "user-1",
		EventType:       string(platformdomain.SecurityEventTypeAuthenticationFailure),
		Severity:        "high",
		Description:     "Authentication failed",
		IPAddress:       "192.0.2.2",
		ResourceType:    "session",
		DetectionMethod: "authentication_flow",
		RiskScore:       70,
		Indicators:      []string{"expired_magic_link"},
		RequiresReview:  true,
	})

	require.NoError(t, err)
	require.NotNil(t, store.securityEvent)
	assert.Equal(t, platformdomain.SecurityEventTypeAuthenticationFailure, store.securityEvent.Type)
	assert.Equal(t, platformdomain.SecuritySeverityHigh, store.securityEvent.Severity)
	assert.Equal(t, 70, store.securityEvent.RiskScore)
	assert.Equal(t, []string{"expired_magic_link"}, store.securityEvent.Indicators)
	assert.Equal(t, fixedTime, store.securityEvent.OccurredAt)
	assert.Equal(t, fixedTime, store.securityEvent.CreatedAt)
}

func TestAuditServiceRejectsInvalidAndPropagatesAppendFailures(t *testing.T) {
	service := NewAuditService(&auditStoreSpy{}, &auditTenantScopeSpy{})

	err := service.LogActivity(context.Background(), LogActivityRequest{
		WorkspaceID: "workspace-1",
		Action:      "login",
		Outcome:     "unknown",
	})
	require.ErrorContains(t, err, "outcome")

	err = service.LogSecurityEvent(context.Background(), LogSecurityEventRequest{
		WorkspaceID: "workspace-1",
		EventType:   "authentication_failure",
		Severity:    "urgent",
		Description: "Authentication failed",
	})
	require.ErrorContains(t, err, "severity")

	storeErr := errors.New("database unavailable")
	service = NewAuditService(&auditStoreSpy{err: storeErr}, &auditTenantScopeSpy{})
	err = service.LogActivity(context.Background(), LogActivityRequest{
		WorkspaceID: "workspace-1",
		Action:      "login",
		Outcome:     "success",
	})
	require.ErrorIs(t, err, storeErr)
}
