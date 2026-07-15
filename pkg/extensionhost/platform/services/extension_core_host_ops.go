package platformservices

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	shared "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared"
	platformdomain "github.com/movebigrocks/platform/pkg/extensionhost/platform/domain"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
)

// runScoped runs fn inside the calling extension's workspace tenant context, in
// one transaction, after checking the extension is active, workspace-scoped, and
// holds permission. Every workspace-scoped host op funnels through here so the
// scope and RLS behavior are defined once.
func (s *ExtensionCoreHostService) runScoped(ctx context.Context, extensionID, permission string, fn func(txCtx context.Context, workspaceID string) error) error {
	return s.runScopedInWorkspace(ctx, extensionID, permission, "", fn)
}

// runScopedInWorkspace runs fn inside the resolved workspace's tenant context in
// one transaction, after applying the extension's scope for targetWorkspaceID. A
// workspace-scoped extension always runs in its own workspace; an instance-scoped
// extension runs in the target workspace it named. Every single-workspace host op
// funnels through here so the scope and RLS behavior are defined once.
func (s *ExtensionCoreHostService) runScopedInWorkspace(ctx context.Context, extensionID, permission, targetWorkspaceID string, fn func(txCtx context.Context, workspaceID string) error) error {
	if s == nil || s.extensions == nil || s.tenant == nil {
		return fmt.Errorf("core host services are not configured")
	}
	_, workspaceID, err := s.resolveExtensionForWorkspace(ctx, extensionID, permission, targetWorkspaceID)
	if err != nil {
		return err
	}
	return s.tenant.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.tenant.SetTenantContext(txCtx, workspaceID); err != nil {
			return err
		}
		return fn(txCtx, workspaceID)
	})
}

// GetQueue returns a queue by id in the extension's workspace.
func (s *ExtensionCoreHostService) GetQueue(ctx context.Context, extensionID, queueID string) (*runtimehost.HostQueue, error) {
	if s.queueReader == nil || s.queueWriter == nil {
		return nil, fmt.Errorf("queue host service is not configured")
	}
	var found *servicedomain.Queue
	err := s.runScoped(ctx, extensionID, "queue:read", func(txCtx context.Context, _ string) error {
		q, getErr := s.queueReader.GetQueue(txCtx, strings.TrimSpace(queueID))
		found = q
		return getErr
	})
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrCoreHostNotFound
		}
		return nil, err
	}
	return hostQueueFromDomain(found), nil
}

// GetQueueBySlug returns a queue by slug in the extension's workspace.
func (s *ExtensionCoreHostService) GetQueueBySlug(ctx context.Context, extensionID, slug string) (*runtimehost.HostQueue, error) {
	if s.queueReader == nil || s.queueWriter == nil {
		return nil, fmt.Errorf("queue host service is not configured")
	}
	var found *servicedomain.Queue
	err := s.runScoped(ctx, extensionID, "queue:read", func(txCtx context.Context, workspaceID string) error {
		q, getErr := s.queueReader.GetQueueBySlug(txCtx, workspaceID, strings.TrimSpace(slug))
		found = q
		return getErr
	})
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrCoreHostNotFound
		}
		return nil, err
	}
	return hostQueueFromDomain(found), nil
}

// CreateQueue creates a queue in the extension's workspace.
func (s *ExtensionCoreHostService) CreateQueue(ctx context.Context, extensionID string, input runtimehost.CreateQueueInput) (*runtimehost.HostQueue, error) {
	if s.queueReader == nil || s.queueWriter == nil {
		return nil, fmt.Errorf("queue host service is not configured")
	}
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Slug) == "" {
		return nil, fmt.Errorf("queue name and slug are required")
	}
	var created *servicedomain.Queue
	err := s.runScoped(ctx, extensionID, "queue:write", func(txCtx context.Context, workspaceID string) error {
		q, createErr := s.queueWriter.CreateQueue(txCtx, serviceapp.CreateQueueParams{
			WorkspaceID: workspaceID,
			Name:        strings.TrimSpace(input.Name),
			Slug:        strings.TrimSpace(input.Slug),
			Description: input.Description,
		})
		created = q
		return createErr
	})
	if err != nil {
		return nil, err
	}
	return hostQueueFromDomain(created), nil
}

// CreateContact creates a contact in the extension's workspace.
func (s *ExtensionCoreHostService) CreateContact(ctx context.Context, extensionID string, input runtimehost.CreateContactInput) (*runtimehost.HostContact, error) {
	if s.contacts == nil {
		return nil, fmt.Errorf("contact host service is not configured")
	}
	if strings.TrimSpace(input.Email) == "" {
		return nil, fmt.Errorf("contact email is required")
	}
	var created *platformdomain.Contact
	err := s.runScoped(ctx, extensionID, "contact:write", func(txCtx context.Context, workspaceID string) error {
		contact, createErr := s.contacts.CreateContact(txCtx, CreateContactParams{
			WorkspaceID: workspaceID,
			Email:       strings.TrimSpace(input.Email),
			Name:        strings.TrimSpace(input.Name),
			Phone:       strings.TrimSpace(input.Phone),
			Company:     strings.TrimSpace(input.Company),
			Source:      strings.TrimSpace(input.Source),
			Metadata:    input.Metadata,
		})
		created = contact
		return createErr
	})
	if err != nil {
		return nil, err
	}
	return &runtimehost.HostContact{
		ID:          created.ID,
		WorkspaceID: created.WorkspaceID,
		Email:       created.Email,
		Name:        created.Name,
		Phone:       created.Phone,
		Company:     created.Company,
	}, nil
}

// UpdateCase applies a targeted patch to a case in the extension's workspace.
// The read, patch, and write share one transaction so the update is atomic and
// only the requested fields change.
func (s *ExtensionCoreHostService) UpdateCase(ctx context.Context, extensionID, caseID string, patch runtimehost.CaseUpdateInput) (*runtimehost.HostCase, error) {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return nil, fmt.Errorf("case id is required")
	}
	var updated *servicedomain.Case
	err := s.runScopedInWorkspace(ctx, extensionID, "case:write", patch.WorkspaceID, func(txCtx context.Context, workspaceID string) error {
		current, getErr := s.cases.GetCaseInWorkspace(txCtx, workspaceID, caseID)
		if getErr != nil {
			return getErr
		}
		applyCasePatch(current, patch)
		if err := s.cases.UpdateCase(txCtx, current); err != nil {
			return err
		}
		updated = current
		return nil
	})
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrCoreHostNotFound
		}
		return nil, err
	}
	return hostCaseFromDomain(updated), nil
}

// HandoffCase moves a case to another queue in the extension's workspace.
func (s *ExtensionCoreHostService) HandoffCase(ctx context.Context, extensionID, caseID string, input runtimehost.HandoffCaseInput) error {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return fmt.Errorf("case id is required")
	}
	err := s.runScoped(ctx, extensionID, "case:write", func(txCtx context.Context, _ string) error {
		return s.cases.HandoffCase(txCtx, caseID, serviceapp.CaseHandoffParams{
			QueueID:          strings.TrimSpace(input.QueueID),
			TeamID:           strings.TrimSpace(input.TeamID),
			AssigneeID:       strings.TrimSpace(input.AssigneeID),
			Reason:           input.Reason,
			PerformedByID:    strings.TrimSpace(input.PerformedByID),
			PerformedByName:  input.PerformedByName,
			PerformedByType:  strings.TrimSpace(input.PerformedByType),
			OnBehalfOfUserID: strings.TrimSpace(input.OnBehalfOfUserID),
		})
	})
	if errors.Is(err, shared.ErrNotFound) {
		return ErrCoreHostNotFound
	}
	return err
}

// MarkCaseResolved marks a case resolved in the extension's workspace.
func (s *ExtensionCoreHostService) MarkCaseResolved(ctx context.Context, extensionID, targetWorkspaceID, caseID string, resolvedAt time.Time) error {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return fmt.Errorf("case id is required")
	}
	err := s.runScopedInWorkspace(ctx, extensionID, "case:write", targetWorkspaceID, func(txCtx context.Context, _ string) error {
		return s.cases.MarkCaseResolved(txCtx, caseID, resolvedAt)
	})
	if errors.Is(err, shared.ErrNotFound) {
		return ErrCoreHostNotFound
	}
	return err
}

// applyCasePatch mutates a case in place with the non-nil fields of a patch.
func applyCasePatch(c *servicedomain.Case, patch runtimehost.CaseUpdateInput) {
	if c == nil {
		return
	}
	if patch.Status != nil {
		c.Status = servicedomain.CaseStatus(strings.TrimSpace(*patch.Status))
	}
	if patch.Priority != nil {
		c.Priority = servicedomain.CasePriority(strings.TrimSpace(*patch.Priority))
	}
	if patch.QueueID != nil {
		c.QueueID = strings.TrimSpace(*patch.QueueID)
	}
	if patch.Category != nil {
		c.Category = strings.TrimSpace(*patch.Category)
	}
	if patch.Tags != nil {
		c.Tags = *patch.Tags
	}
	for k, v := range patch.CustomFields {
		c.CustomFields.SetAny(k, v)
	}
}

func hostQueueFromDomain(q *servicedomain.Queue) *runtimehost.HostQueue {
	if q == nil {
		return nil
	}
	return &runtimehost.HostQueue{
		ID:          q.ID,
		WorkspaceID: q.WorkspaceID,
		Slug:        q.Slug,
		Name:        q.Name,
		Description: q.Description,
	}
}
