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
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

// ErrCoreHostNotFound is returned when a core entity the extension asked for
// does not exist or is outside the extension's workspace. The handler maps it
// to HTTP 404.
var ErrCoreHostNotFound = errors.New("core entity not found")

// ExtensionCoreHostService backs the core-data host API. A first-party
// extension calls it (bearer host token) instead of embedding a copy of the
// core stores. It resolves the calling extension, enforces the extension's
// declared permission and workspace scope, and runs each core write inside the
// extension's workspace tenant context so row-level security applies exactly as
// it would for a user in that workspace.
type ExtensionCoreHostService struct {
	extensions      coreHostExtensionResolver
	cases           coreHostCaseService
	queueReader     coreHostQueueReader
	queueWriter     coreHostQueueWriter
	contacts        coreHostContactService
	attachments     coreHostAttachmentUploader
	attachmentStore coreHostAttachmentStore
	rules           coreHostRuleEngine
	artifacts       coreHostArtifactPublisher
	tenant          coreHostTenantRunner
}

// coreHostExtensionResolver loads the calling extension. It is an interface so
// the host service can be unit tested without the full extension service; the
// concrete *ExtensionService satisfies it.
type coreHostExtensionResolver interface {
	GetInstalledExtension(ctx context.Context, extensionID string) (*platformdomain.InstalledExtension, error)
}

// coreHostCaseService is the slice of the core case service the host API needs.
// It is an interface so this package does not hard-depend on the concrete
// service wiring and can be unit tested.
type coreHostCaseService interface {
	CreateCase(ctx context.Context, params serviceapp.CreateCaseParams) (*servicedomain.Case, error)
	GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*servicedomain.Case, error)
	UpdateCase(ctx context.Context, caseObj *servicedomain.Case) error
	HandoffCase(ctx context.Context, caseID string, params serviceapp.CaseHandoffParams) error
	MarkCaseResolved(ctx context.Context, caseID string, resolvedAt time.Time) error
}

// coreHostQueueReader reads queues; the core queue store satisfies it. Queue
// reads are store-level in the core, writes are service-level, so the host API
// takes both a reader and a writer.
type coreHostQueueReader interface {
	GetQueue(ctx context.Context, queueID string) (*servicedomain.Queue, error)
	GetQueueBySlug(ctx context.Context, workspaceID, slug string) (*servicedomain.Queue, error)
}

// coreHostQueueWriter creates queues; the core queue service satisfies it.
type coreHostQueueWriter interface {
	CreateQueue(ctx context.Context, params serviceapp.CreateQueueParams) (*servicedomain.Queue, error)
}

// coreHostContactService is the slice of the core contact service the host API
// needs.
type coreHostContactService interface {
	CreateContact(ctx context.Context, params CreateContactParams) (*platformdomain.Contact, error)
}

// CoreHostDeps wires the core services the host API sits in front of. Fields are
// interfaces so tests can substitute fakes; the container passes the concrete
// services.
type CoreHostDeps struct {
	Extensions      coreHostExtensionResolver
	Cases           coreHostCaseService
	QueueReader     coreHostQueueReader
	QueueWriter     coreHostQueueWriter
	Contacts        coreHostContactService
	Attachments     coreHostAttachmentUploader
	AttachmentStore coreHostAttachmentStore
	Rules           coreHostRuleEngine
	Artifacts       coreHostArtifactPublisher
	Tenant          coreHostTenantRunner
}

// coreHostTenantRunner runs a function inside one transaction and sets the
// workspace tenant context within it. set_config for the workspace is
// transaction-local, so the write must share the transaction that sets it.
type coreHostTenantRunner interface {
	WithTransaction(ctx context.Context, fn func(context.Context) error) error
	SetTenantContext(ctx context.Context, workspaceID string) error
	// GetHostOperationResult and PutHostOperationResult back the idempotency
	// ledger that makes a coarse operation safe to retry: it records the result
	// under the caller's key so a repeat returns the same ids.
	GetHostOperationResult(ctx context.Context, workspaceID, extensionID, operation, key string) ([]byte, bool, error)
	PutHostOperationResult(ctx context.Context, workspaceID, extensionID, operation, key string, result []byte) error
}

func NewExtensionCoreHostService(deps CoreHostDeps) *ExtensionCoreHostService {
	return &ExtensionCoreHostService{
		extensions:      deps.Extensions,
		cases:           deps.Cases,
		queueReader:     deps.QueueReader,
		queueWriter:     deps.QueueWriter,
		contacts:        deps.Contacts,
		attachments:     deps.Attachments,
		attachmentStore: deps.AttachmentStore,
		rules:           deps.Rules,
		artifacts:       deps.Artifacts,
		tenant:          deps.Tenant,
	}
}

// CreateCase creates a core case in the calling extension's workspace.
func (s *ExtensionCoreHostService) CreateCase(ctx context.Context, extensionID string, input runtimehost.CreateCaseInput) (*runtimehost.HostCase, error) {
	if s == nil || s.extensions == nil || s.cases == nil || s.tenant == nil {
		return nil, fmt.Errorf("core host services are not configured")
	}
	_, workspaceID, err := s.resolveExtensionForWorkspace(ctx, extensionID, "case:write", "")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Subject) == "" {
		return nil, fmt.Errorf("subject is required")
	}

	params := serviceapp.CreateCaseParams{
		WorkspaceID:  workspaceID,
		Subject:      strings.TrimSpace(input.Subject),
		Description:  input.Description,
		Priority:     servicedomain.CasePriority(strings.TrimSpace(input.Priority)),
		Channel:      servicedomain.CaseChannel(strings.TrimSpace(input.Channel)),
		Category:     strings.TrimSpace(input.Category),
		QueueID:      strings.TrimSpace(input.QueueID),
		ContactID:    strings.TrimSpace(input.ContactID),
		ContactName:  strings.TrimSpace(input.ContactName),
		ContactEmail: strings.TrimSpace(input.ContactEmail),
		TeamID:       strings.TrimSpace(input.TeamID),
		AssignedToID: strings.TrimSpace(input.AssignedToID),
		Tags:         input.Tags,
		CustomFields: customFieldsFromMap(input.CustomFields),
	}

	var created *servicedomain.Case
	err = s.tenant.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.tenant.SetTenantContext(txCtx, workspaceID); err != nil {
			return err
		}
		c, createErr := s.cases.CreateCase(txCtx, params)
		created = c
		return createErr
	})
	if err != nil {
		return nil, err
	}
	return hostCaseFromDomain(created), nil
}

// GetCase returns a case in the calling extension's workspace, or a not-found
// error the handler maps to HTTP 404.
func (s *ExtensionCoreHostService) GetCase(ctx context.Context, extensionID, caseID string) (*runtimehost.HostCase, error) {
	if s == nil || s.extensions == nil || s.cases == nil || s.tenant == nil {
		return nil, fmt.Errorf("core host services are not configured")
	}
	_, workspaceID, err := s.resolveExtensionForWorkspace(ctx, extensionID, "case:read", "")
	if err != nil {
		return nil, err
	}
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return nil, fmt.Errorf("case id is required")
	}

	var found *servicedomain.Case
	err = s.tenant.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.tenant.SetTenantContext(txCtx, workspaceID); err != nil {
			return err
		}
		c, getErr := s.cases.GetCaseInWorkspace(txCtx, workspaceID, caseID)
		found = c
		return getErr
	})
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			return nil, ErrCoreHostNotFound
		}
		return nil, err
	}
	return hostCaseFromDomain(found), nil
}

// resolveExtension loads the calling extension and enforces that it is active
// and declares the required permission. It does not bind a workspace; callers
// apply the scope rule for their operation through resolveExtensionForWorkspace
// (a single-workspace operation) or resolveExtensionForCrossWorkspace (an
// operation that spans workspaces).
func (s *ExtensionCoreHostService) resolveExtension(ctx context.Context, extensionID, permission string) (*platformdomain.InstalledExtension, error) {
	extension, err := s.extensions.GetInstalledExtension(ctx, strings.TrimSpace(extensionID))
	if err != nil {
		return nil, err
	}
	if extension == nil || extension.Status != platformdomain.ExtensionStatusActive {
		return nil, ErrExtensionHostForbidden
	}
	if !manifestHasPermission(extension.Manifest, permission) {
		return nil, ErrExtensionHostForbidden
	}
	return extension, nil
}

// resolveExtensionForWorkspace resolves the extension and the workspace a
// single-workspace operation must run in, applying the extension's scope:
//
//   - a workspace-scoped extension is pinned to its own installed workspace and
//     may not name another; passing a different target is refused.
//   - an instance-scoped extension is not bound to a workspace, so it must name
//     the target workspace it wants to act in. Holding the instance scope grant
//     is what authorizes it to act in any workspace.
//
// This is the one place the workspace an operation runs in is decided, so RLS
// and the cross-tenant boundary are enforced consistently for every op.
func (s *ExtensionCoreHostService) resolveExtensionForWorkspace(ctx context.Context, extensionID, permission, targetWorkspaceID string) (*platformdomain.InstalledExtension, string, error) {
	extension, err := s.resolveExtension(ctx, extensionID, permission)
	if err != nil {
		return nil, "", err
	}
	targetWorkspaceID = strings.TrimSpace(targetWorkspaceID)
	switch extension.Manifest.Scope {
	case platformdomain.ExtensionScopeWorkspace:
		if strings.TrimSpace(extension.WorkspaceID) == "" {
			return nil, "", ErrExtensionHostForbidden
		}
		if targetWorkspaceID != "" && targetWorkspaceID != extension.WorkspaceID {
			return nil, "", ErrExtensionHostForbidden
		}
		return extension, extension.WorkspaceID, nil
	case platformdomain.ExtensionScopeInstance:
		if targetWorkspaceID == "" {
			return nil, "", fmt.Errorf("workspaceId is required for an instance-scoped extension")
		}
		return extension, targetWorkspaceID, nil
	default:
		return nil, "", ErrExtensionHostForbidden
	}
}

// resolveExtensionForCrossWorkspace resolves the extension for an operation that
// inherently spans workspaces (for example listing every workspace). Only an
// instance-scoped extension holding the permission may perform it; a
// workspace-scoped extension is confined to its own workspace and is refused.
func (s *ExtensionCoreHostService) resolveExtensionForCrossWorkspace(ctx context.Context, extensionID, permission string) (*platformdomain.InstalledExtension, error) {
	extension, err := s.resolveExtension(ctx, extensionID, permission)
	if err != nil {
		return nil, err
	}
	if extension.Manifest.Scope != platformdomain.ExtensionScopeInstance {
		return nil, ErrExtensionHostForbidden
	}
	return extension, nil
}

func customFieldsFromMap(m map[string]any) shareddomain.TypedCustomFields {
	tcf := shareddomain.NewTypedCustomFields()
	for k, v := range m {
		tcf.SetAny(k, v)
	}
	return tcf
}

func hostCaseFromDomain(c *servicedomain.Case) *runtimehost.HostCase {
	if c == nil {
		return nil
	}
	return &runtimehost.HostCase{
		ID:           c.ID,
		HumanID:      c.HumanID,
		WorkspaceID:  c.WorkspaceID,
		Subject:      c.Subject,
		Description:  c.Description,
		Status:       string(c.Status),
		Priority:     string(c.Priority),
		Channel:      string(c.Channel),
		Category:     c.Category,
		QueueID:      c.QueueID,
		Tags:         c.Tags,
		ContactID:    c.ContactID,
		ContactEmail: c.ContactEmail,
		CustomFields: c.CustomFields.ToMap(),
	}
}
