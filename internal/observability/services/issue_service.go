package observabilityservices

import (
	"context"
	"time"

	apierrors "github.com/movebigrocks/platform/internal/infrastructure/errors"
	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
	observabilitydomain "github.com/movebigrocks/platform/internal/observability/domain"
	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	"github.com/movebigrocks/platform/internal/shared/authorization"
	"github.com/movebigrocks/platform/internal/shared/contracts"
	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

// IssueService handles all issue-related business logic
type IssueService struct {
	issueStore      shared.IssueStore
	projectStore    shared.ProjectStore
	errorEventStore shared.ErrorEventStore
	workspaceStore  shared.WorkspaceCRUD
	tx              contracts.TransactionRunner
	outbox          contracts.OutboxPublisher
	authorizer      *authorization.Authorizer // Defense-in-depth permission checking
	logger          *logger.Logger
}

// IssueServiceOption configures optional dependencies for IssueService.
type IssueServiceOption func(*IssueService)

// NewIssueService creates a new issue service
func NewIssueService(
	issueStore shared.IssueStore,
	projectStore shared.ProjectStore,
	errorEventStore shared.ErrorEventStore,
	workspaceStore shared.WorkspaceCRUD,
	outbox contracts.OutboxPublisher,
	opts ...IssueServiceOption,
) *IssueService {
	service := &IssueService{
		issueStore:      issueStore,
		projectStore:    projectStore,
		errorEventStore: errorEventStore,
		workspaceStore:  workspaceStore,
		outbox:          outbox,
		authorizer:      authorization.NewAuthorizer(),
		logger:          logger.New().WithField("service", "issue"),
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

// GetIssue retrieves an issue by ID
func (s *IssueService) GetIssue(ctx context.Context, issueID string) (*observabilitydomain.Issue, error) {
	if issueID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("issue_id", "required"))
	}

	issue, err := s.issueStore.GetIssue(ctx, issueID)
	if err != nil {
		return nil, apierrors.NotFoundError("issue", issueID)
	}
	return issue, nil
}

// GetIssueInWorkspace retrieves an issue only if it belongs to the specified workspace.
// Returns ErrNotFound if issue doesn't exist OR belongs to different workspace.
// This prevents information disclosure about entity existence across workspaces.
func (s *IssueService) GetIssueInWorkspace(ctx context.Context, workspaceID, issueID string) (*observabilitydomain.Issue, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if issueID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("issue_id", "required"))
	}

	issue, err := s.issueStore.GetIssueInWorkspace(ctx, workspaceID, issueID)
	if err != nil {
		return nil, apierrors.NotFoundError("issue", issueID)
	}

	// Layer 3 defense-in-depth (ADR-0003): verify store returned correct workspace
	// Guards against store implementation bugs - same error to prevent enumeration
	if issue.WorkspaceID != workspaceID {
		return nil, apierrors.NotFoundError("issue", issueID)
	}

	return issue, nil
}

// GetIssuesByIDs retrieves multiple issues by their IDs
func (s *IssueService) GetIssuesByIDs(ctx context.Context, issueIDs []string) ([]*observabilitydomain.Issue, error) {
	if len(issueIDs) == 0 {
		return []*observabilitydomain.Issue{}, nil
	}
	return s.issueStore.GetIssuesByIDs(ctx, issueIDs)
}

// UpdateIssue updates an existing issue
func (s *IssueService) UpdateIssue(ctx context.Context, issue *observabilitydomain.Issue) error {
	// Defense-in-depth: check permission at service layer (layer 3)
	if err := s.authorizer.RequirePermission(ctx, platformdomain.PermissionIssueWrite); err != nil {
		return apierrors.New(apierrors.ErrorTypeAuthorization, "issue:write permission required")
	}

	if issue.ID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("id", "required"))
	}

	if err := s.issueStore.UpdateIssue(ctx, issue); err != nil {
		return apierrors.DatabaseError("update issue", err)
	}
	return nil
}

// ListIssues lists issues with filters (workspace-scoped)
func (s *IssueService) ListIssues(ctx context.Context, filters shared.IssueFilters) ([]*observabilitydomain.Issue, int, error) {
	return s.issueStore.ListIssues(ctx, filters)
}

// ListWorkspaceIssues lists all issues for a workspace
func (s *IssueService) ListWorkspaceIssues(ctx context.Context, workspaceID string, limit int) ([]*observabilitydomain.Issue, int, error) {
	if workspaceID == "" {
		return nil, 0, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	return s.issueStore.ListIssues(ctx, shared.IssueFilters{
		WorkspaceID: workspaceID,
		Limit:       limit,
	})
}

// ListWorkspaceIssuesWithFilters lists workspace issues using the shared issue filters contract.
func (s *IssueService) ListWorkspaceIssuesWithFilters(ctx context.Context, workspaceID string, filters shared.IssueFilters) ([]*observabilitydomain.Issue, int, error) {
	if workspaceID == "" {
		return nil, 0, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	filters.WorkspaceID = workspaceID
	return s.issueStore.ListIssues(ctx, filters)
}

func (s *IssueService) CountOpenWorkspaceIssues(ctx context.Context, workspaceID string) (int, error) {
	if workspaceID == "" {
		return 0, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	issues, _, err := s.issueStore.ListIssues(ctx, shared.IssueFilters{
		WorkspaceID: workspaceID,
		Status:      string(observabilitydomain.IssueStatusUnresolved),
		Limit:       1000,
	})
	if err != nil {
		return 0, err
	}
	return len(issues), nil
}

func (s *IssueService) CountRecentIssues(ctx context.Context, since time.Time) (int64, error) {
	issues, _, err := s.issueStore.ListAllIssues(ctx, shared.IssueFilters{
		Limit: 10000,
	})
	if err != nil {
		return 0, err
	}
	var count int64
	for _, issue := range issues {
		if issue != nil && issue.FirstSeen.After(since) {
			count++
		}
	}
	return count, nil
}

// ListAllIssues lists all issues across workspaces (requires admin context)
func (s *IssueService) ListAllIssues(ctx context.Context, filters shared.IssueFilters) ([]*observabilitydomain.Issue, int, error) {
	return s.issueStore.ListAllIssues(ctx, filters)
}

// ListProjectIssues lists issues for a specific project
func (s *IssueService) ListProjectIssues(ctx context.Context, projectID string, filter shared.IssueFilter) ([]*observabilitydomain.Issue, error) {
	if projectID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("project_id", "required"))
	}
	return s.issueStore.ListProjectIssues(ctx, projectID, filter)
}

// ResolveIssue marks an issue as resolved
func (s *IssueService) ResolveIssue(ctx context.Context, issueID string, resolution string, resolvedBy string) error {
	// Defense-in-depth: check permission at service layer (layer 3)
	if err := s.authorizer.RequirePermission(ctx, platformdomain.PermissionIssueWrite); err != nil {
		return apierrors.New(apierrors.ErrorTypeAuthorization, "issue:write permission required")
	}

	if resolvedBy == "" {
		resolvedBy = "system"
	}
	if resolution == "" {
		resolution = "fixed"
	}

	issue, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return err
	}

	resolvedAt := time.Now()
	if err := issue.MarkResolved(resolvedAt, resolvedBy, resolution); err != nil {
		return apierrors.DatabaseError("resolve issue", err)
	}
	issue.SetResolutionNotes(resolution)

	updateFn := func(txCtx context.Context) error {
		if err := s.issueStore.UpdateIssue(txCtx, issue); err != nil {
			return apierrors.DatabaseError("resolve issue", err)
		}

		if err := s.publishIssueResolved(txCtx, issue); err != nil {
			return err
		}
		if err := s.publishCasesBulkResolved(txCtx, issue); err != nil {
			return err
		}
		return nil
	}

	if s.tx != nil && s.outbox != nil {
		return s.tx.WithTransaction(ctx, updateFn)
	}

	if err := s.issueStore.UpdateIssue(ctx, issue); err != nil {
		return apierrors.DatabaseError("resolve issue", err)
	}

	// Best effort: preserve existing behavior when running without transaction.
	if err := s.publishIssueResolved(ctx, issue); err != nil {
		s.logger.WithError(err).Warn("Failed to publish issue resolved event (non-transactional)", "issue_id", issue.ID)
	}
	if err := s.publishCasesBulkResolved(ctx, issue); err != nil {
		s.logger.WithError(err).Warn("Failed to publish cases bulk resolved event (non-transactional)", "issue_id", issue.ID)
	}
	return nil
}

// SetIssueStatus applies a validated status transition and persists the issue.
func (s *IssueService) SetIssueStatus(ctx context.Context, issueID, status, changedBy string) (*observabilitydomain.Issue, error) {
	if err := s.authorizer.RequirePermission(ctx, platformdomain.PermissionIssueWrite); err != nil {
		return nil, apierrors.New(apierrors.ErrorTypeAuthorization, "issue:write permission required")
	}

	issue, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}

	if changedBy == "" {
		changedBy = "system"
	}
	if err := issue.SetStatus(status, time.Now(), changedBy); err != nil {
		return nil, apierrors.Wrap(err, apierrors.ErrorTypeValidation, "issue status transition failed")
	}

	if err := s.issueStore.UpdateIssue(ctx, issue); err != nil {
		return nil, apierrors.DatabaseError("set issue status", err)
	}
	return issue, nil
}

// LinkIssueToCase records the case linkage on the issue aggregate.
func (s *IssueService) LinkIssueToCase(ctx context.Context, issueID, caseID string) (*observabilitydomain.Issue, error) {
	if err := s.authorizer.RequirePermission(ctx, platformdomain.PermissionIssueWrite); err != nil {
		return nil, apierrors.New(apierrors.ErrorTypeAuthorization, "issue:write permission required")
	}

	issue, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}

	issue.LinkCase(caseID)
	if err := s.issueStore.UpdateIssue(ctx, issue); err != nil {
		return nil, apierrors.DatabaseError("link issue to case", err)
	}
	return issue, nil
}

// UnlinkIssueFromCase removes a case linkage from the issue aggregate.
func (s *IssueService) UnlinkIssueFromCase(ctx context.Context, issueID, caseID string) (*observabilitydomain.Issue, error) {
	if err := s.authorizer.RequirePermission(ctx, platformdomain.PermissionIssueWrite); err != nil {
		return nil, apierrors.New(apierrors.ErrorTypeAuthorization, "issue:write permission required")
	}

	issue, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return nil, err
	}

	if caseID == "" {
		issue.ClearCaseLinks()
	} else {
		issue.UnlinkCase(caseID)
	}
	if err := s.issueStore.UpdateIssue(ctx, issue); err != nil {
		return nil, apierrors.DatabaseError("unlink issue from case", err)
	}
	return issue, nil
}

// IgnoreIssue marks an issue as ignored
func (s *IssueService) IgnoreIssue(ctx context.Context, issueID string) error {
	// Defense-in-depth: check permission at service layer (layer 3)
	if err := s.authorizer.RequirePermission(ctx, platformdomain.PermissionIssueWrite); err != nil {
		return apierrors.New(apierrors.ErrorTypeAuthorization, "issue:write permission required")
	}

	issue, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return err
	}

	issue.Status = observabilitydomain.IssueStatusIgnored

	if err := s.issueStore.UpdateIssue(ctx, issue); err != nil {
		return apierrors.DatabaseError("ignore issue", err)
	}
	return nil
}

// ReopenIssue reopens a resolved or ignored issue
func (s *IssueService) ReopenIssue(ctx context.Context, issueID string) error {
	// Defense-in-depth: check permission at service layer (layer 3)
	if err := s.authorizer.RequirePermission(ctx, platformdomain.PermissionIssueWrite); err != nil {
		return apierrors.New(apierrors.ErrorTypeAuthorization, "issue:write permission required")
	}

	issue, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return err
	}

	issue.Status = observabilitydomain.IssueStatusUnresolved
	issue.ResolvedAt = nil

	if err := s.issueStore.UpdateIssue(ctx, issue); err != nil {
		return apierrors.DatabaseError("reopen issue", err)
	}
	return nil
}

// GetIssueEvents retrieves error events for an issue
func (s *IssueService) GetIssueEvents(ctx context.Context, issueID string, limit int) ([]*observabilitydomain.ErrorEvent, error) {
	if issueID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("issue_id", "required"))
	}
	if limit <= 0 {
		limit = 50
	}
	return s.errorEventStore.GetIssueEvents(ctx, issueID, limit)
}

// GetIssueWithProject retrieves an issue with its project info
func (s *IssueService) GetIssueWithProject(ctx context.Context, issueID string) (*observabilitydomain.Issue, *observabilitydomain.Project, error) {
	issue, err := s.GetIssue(ctx, issueID)
	if err != nil {
		return nil, nil, err
	}

	project, err := s.projectStore.GetProject(ctx, issue.ProjectID)
	if err != nil {
		// Issue exists but project not found - return issue without project
		return issue, nil, nil //nolint:nilerr // Intentional: gracefully handle missing project
	}

	return issue, project, nil
}

// =============================================================================
// Event Handler Support Methods
// These methods are used by IssueEventHandler for processing issue events
// =============================================================================

// CreateOrUpdateIssueByFingerprint atomically creates or updates an issue by fingerprint.
// Returns the issue, whether it was created (vs updated), and any error.
func (s *IssueService) CreateOrUpdateIssueByFingerprint(ctx context.Context, issue *observabilitydomain.Issue) (*observabilitydomain.Issue, bool, error) {
	if issue.Fingerprint == "" {
		return nil, false, apierrors.NewValidationErrors(apierrors.NewValidationError("fingerprint", "required"))
	}
	return s.issueStore.CreateOrUpdateIssueByFingerprint(ctx, issue)
}

// AtomicUpdateIssueStats atomically updates issue statistics (event_count++, optionally user_count++)
// Returns the updated issue.
func (s *IssueService) AtomicUpdateIssueStats(ctx context.Context, workspaceID, issueID, newEventID string, lastSeen time.Time, hasNewUser bool) (*observabilitydomain.Issue, error) {
	if workspaceID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if issueID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("issue_id", "required"))
	}
	return s.issueStore.AtomicUpdateIssueStats(ctx, workspaceID, issueID, newEventID, lastSeen, hasNewUser)
}

// GetErrorEvent retrieves an error event by ID
func (s *IssueService) GetErrorEvent(ctx context.Context, eventID string) (*observabilitydomain.ErrorEvent, error) {
	if eventID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("event_id", "required"))
	}
	return s.errorEventStore.GetErrorEvent(ctx, eventID)
}

// publishIssueResolved publishes an IssueResolved event for downstream handlers.
// Best effort: if publishing fails, resolve operation succeeds and log warning.
func (s *IssueService) publishIssueResolved(ctx context.Context, issue *observabilitydomain.Issue) error {
	if s.outbox == nil {
		s.logger.Debug("Skipping issue resolved event publish - outbox not configured", "issue_id", issue.ID)
		return nil
	}

	if issue == nil {
		return nil
	}

	event := shareddomain.IssueResolved{
		BaseEvent:         eventbus.NewBaseEvent(eventbus.TypeIssueResolved),
		IssueID:           issue.ID,
		ProjectID:         issue.ProjectID,
		WorkspaceID:       issue.WorkspaceID,
		Resolution:        issue.Resolution,
		ResolutionNotes:   issue.ResolutionNotes,
		ResolvedBy:        issue.ResolvedBy,
		ResolvedInVersion: issue.ResolvedInVersion,
		ResolvedInCommit:  issue.ResolvedInCommit,
		ResolvedAt:        time.Now(),
		AffectedCaseCount: len(issue.RelatedCaseIDs),
		SystemCaseIDs:     issue.RelatedCaseIDs,
	}

	if issue.ResolvedAt != nil {
		event.ResolvedAt = *issue.ResolvedAt
	}

	if err := s.outbox.PublishEvent(ctx, eventbus.StreamIssueEvents, event); err != nil {
		s.logger.WithError(err).Warn("Failed to publish issue resolved event", "issue_id", issue.ID)
		return err
	}
	return nil
}

// publishCasesBulkResolved publishes related cases for automatic case resolution.
// Best effort: if publishing fails, issue resolution still succeeds.
func (s *IssueService) publishCasesBulkResolved(ctx context.Context, issue *observabilitydomain.Issue) error {
	if s.outbox == nil {
		s.logger.Debug("Skipping cases bulk resolved publish - outbox not configured", "issue_id", issue.ID)
		return nil
	}
	if issue == nil || len(issue.RelatedCaseIDs) == 0 {
		return nil
	}

	resolvedAt := time.Now()
	if issue.ResolvedAt != nil {
		resolvedAt = *issue.ResolvedAt
	}
	event := shareddomain.CasesBulkResolved{
		BaseEvent:   eventbus.NewBaseEvent(eventbus.TypeCasesBulkResolved),
		IssueID:     issue.ID,
		ProjectID:   issue.ProjectID,
		WorkspaceID: issue.WorkspaceID,
		CaseIDs:     append([]string(nil), issue.RelatedCaseIDs...),
		Resolution:  issue.Resolution,
		ResolvedAt:  resolvedAt,
	}

	if err := s.outbox.PublishEvent(ctx, eventbus.StreamCaseEvents, event); err != nil {
		s.logger.WithError(err).Warn("Failed to publish cases bulk resolved event", "issue_id", issue.ID)
		return err
	}
	return nil
}

// LinkEventToIssue updates an error event to associate it with an issue
func (s *IssueService) LinkEventToIssue(ctx context.Context, workspaceID, eventID, issueID string) error {
	if workspaceID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("workspace_id", "required"))
	}
	if eventID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("event_id", "required"))
	}
	if issueID == "" {
		return apierrors.NewValidationErrors(apierrors.NewValidationError("issue_id", "required"))
	}

	// Use atomic update instead of fetch-modify-create pattern
	// which would fail with duplicate primary key
	return s.errorEventStore.UpdateEventIssueID(ctx, workspaceID, eventID, issueID)
}

// GetProject retrieves a project by ID
func (s *IssueService) GetProject(ctx context.Context, projectID string) (*observabilitydomain.Project, error) {
	if projectID == "" {
		return nil, apierrors.NewValidationErrors(apierrors.NewValidationError("project_id", "required"))
	}
	return s.projectStore.GetProject(ctx, projectID)
}
