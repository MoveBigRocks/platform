package platformservices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/platform/pkg/eventbus"
	shared "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
)

type coreHostEventPublisher interface {
	PublishEvent(ctx context.Context, stream eventbus.Stream, event eventbus.Event) error
}

func (s *ExtensionCoreHostService) LinkIssueToCase(ctx context.Context, extensionID, caseID string, input runtimehost.LinkIssueToCaseInput) error {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" || strings.TrimSpace(input.IssueID) == "" {
		return fmt.Errorf("case id and issue id are required")
	}
	err := s.runScopedInWorkspace(ctx, extensionID, "case:write", input.WorkspaceID, func(txCtx context.Context, _ string) error {
		return s.cases.LinkIssueToCase(txCtx, caseID, strings.TrimSpace(input.IssueID), strings.TrimSpace(input.ProjectID))
	})
	if errors.Is(err, shared.ErrNotFound) {
		return ErrCoreHostNotFound
	}
	return err
}

func (s *ExtensionCoreHostService) UnlinkIssueFromCase(ctx context.Context, extensionID, caseID string, input runtimehost.UnlinkIssueFromCaseInput) error {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" || strings.TrimSpace(input.IssueID) == "" {
		return fmt.Errorf("case id and issue id are required")
	}
	err := s.runScopedInWorkspace(ctx, extensionID, "case:write", input.WorkspaceID, func(txCtx context.Context, _ string) error {
		return s.cases.UnlinkIssueFromCase(txCtx, caseID, strings.TrimSpace(input.IssueID))
	})
	if errors.Is(err, shared.ErrNotFound) {
		return ErrCoreHostNotFound
	}
	return err
}

func (s *ExtensionCoreHostService) GetCaseByIssueAndContact(ctx context.Context, extensionID, workspaceID, issueID, contactID string) (*runtimehost.HostCase, error) {
	if s.caseStore == nil {
		return nil, fmt.Errorf("case lookup host service is not configured")
	}
	if strings.TrimSpace(issueID) == "" || strings.TrimSpace(contactID) == "" {
		return nil, fmt.Errorf("issue id and contact id are required")
	}
	var found *servicedomain.Case
	err := s.runScopedInWorkspace(ctx, extensionID, "case:read", workspaceID, func(txCtx context.Context, effectiveWorkspaceID string) error {
		caseObj, lookupErr := s.caseStore.GetCaseByIssueAndContact(txCtx, effectiveWorkspaceID, strings.TrimSpace(issueID), strings.TrimSpace(contactID))
		found = caseObj
		return lookupErr
	})
	if errors.Is(err, shared.ErrNotFound) {
		return nil, ErrCoreHostNotFound
	}
	if err != nil {
		return nil, err
	}
	return hostCaseFromDomain(found), nil
}

func (s *ExtensionCoreHostService) ListWorkspaces(ctx context.Context, extensionID string) ([]runtimehost.HostWorkspace, error) {
	if s == nil || s.workspaces == nil || s.tenant == nil {
		return nil, fmt.Errorf("workspace host service is not configured")
	}
	if _, err := s.resolveExtensionForCrossWorkspace(ctx, extensionID, "workspace:read"); err != nil {
		return nil, err
	}
	var workspaces []*platformdomain.Workspace
	err := s.tenant.WithAdminContext(ctx, func(adminCtx context.Context) error {
		var listErr error
		workspaces, listErr = s.workspaces.ListWorkspaces(adminCtx)
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return hostWorkspacesFromDomain(workspaces), nil
}

func (s *ExtensionCoreHostService) GetWorkspacesByIDs(ctx context.Context, extensionID string, ids []string) ([]runtimehost.HostWorkspace, error) {
	if s == nil || s.workspaces == nil || s.tenant == nil {
		return nil, fmt.Errorf("workspace host service is not configured")
	}
	if _, err := s.resolveExtensionForCrossWorkspace(ctx, extensionID, "workspace:read"); err != nil {
		return nil, err
	}
	var workspaces []*platformdomain.Workspace
	err := s.tenant.WithAdminContext(ctx, func(adminCtx context.Context) error {
		var listErr error
		workspaces, listErr = s.workspaces.GetWorkspacesByIDs(adminCtx, ids)
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return hostWorkspacesFromDomain(workspaces), nil
}

func (s *ExtensionCoreHostService) PublishEvent(ctx context.Context, extensionID string, input runtimehost.PublishEventInput) error {
	if s == nil || s.outbox == nil {
		return fmt.Errorf("event host service is not configured")
	}
	eventType := eventbus.EventTypeFromString(strings.TrimSpace(input.EventType))
	if eventType.IsUnknown() || eventType.IsZero() {
		return fmt.Errorf("registered eventType is required")
	}
	return s.runScopedInWorkspace(ctx, extensionID, "event:publish", input.WorkspaceID, func(txCtx context.Context, workspaceID string) error {
		data := make(map[string]any, len(input.Data)+1)
		for key, value := range input.Data {
			data[key] = value
		}
		if payloadWorkspaceID := publishedEventWorkspaceID(data); payloadWorkspaceID != "" && payloadWorkspaceID != workspaceID {
			return ErrExtensionHostForbidden
		}
		for _, key := range []string{"workspaceID", "workspaceId", "workspace_id"} {
			delete(data, key)
		}
		data["WorkspaceID"] = workspaceID
		event := &hostPublishedEvent{BaseEvent: eventbus.NewBaseEvent(eventType), data: data}
		return s.outbox.PublishEvent(txCtx, eventStream(eventType.String()), event)
	})
}

func publishedEventWorkspaceID(data map[string]any) string {
	for _, key := range []string{"WorkspaceID", "workspaceID", "workspaceId", "workspace_id"} {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func eventStream(eventType string) eventbus.Stream {
	switch {
	case strings.HasPrefix(eventType, "issue."):
		return eventbus.StreamIssueEvents
	case eventType == "issue_case.linked", eventType == "issue_case.unlinked", eventType == "case.created_for_contact", eventType == "cases.bulk_resolved":
		return eventbus.StreamCaseEvents
	default:
		return eventbus.StreamSystemEvents
	}
}

type hostPublishedEvent struct {
	eventbus.BaseEvent
	data map[string]any
}

func (e hostPublishedEvent) Validate() error {
	if strings.TrimSpace(e.EventID) == "" || e.EventType.IsZero() {
		return fmt.Errorf("event id and type are required")
	}
	return nil
}

func (e hostPublishedEvent) MarshalJSON() ([]byte, error) {
	data := make(map[string]any, len(e.data)+3)
	for key, value := range e.data {
		data[key] = value
	}
	data["event_id"] = e.EventID
	data["event_type"] = e.EventType
	data["timestamp"] = e.Timestamp
	return json.Marshal(data)
}

func hostWorkspacesFromDomain(workspaces []*platformdomain.Workspace) []runtimehost.HostWorkspace {
	result := make([]runtimehost.HostWorkspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		if workspace == nil {
			continue
		}
		result = append(result, runtimehost.HostWorkspace{
			ID: workspace.ID, Slug: workspace.Slug, Name: workspace.Name, ShortCode: workspace.ShortCode,
			Description: workspace.Description, LogoURL: workspace.LogoURL, PrimaryColor: workspace.PrimaryColor,
			AccentColor: workspace.AccentColor, IsActive: workspace.IsActive, IsSuspended: workspace.IsSuspended,
		})
	}
	return result
}
